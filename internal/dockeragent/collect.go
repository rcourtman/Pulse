package dockeragent

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
)

// buildReport gathers all system and container metrics into a single report
func (a *Agent) buildReport(ctx context.Context) (agentsdocker.Report, error) {
	info, err := a.docker.Info(ctx)
	if err != nil {
		return agentsdocker.Report{}, fmt.Errorf("failed to query docker info: %w", err)
	}

	a.runtimeVer = info.ServerVersion
	if a.daemonHost == "" {
		a.daemonHost = a.docker.DaemonHost()
	}

	// Use current runtime as preference to avoid spurious switching.
	// This preserves user's --docker-runtime choice (stored in a.runtime at init).
	newRuntime := detectRuntime(info, a.daemonHost, a.runtime)
	if newRuntime != a.runtime {
		if a.runtime != "" {
			a.logger.Info().
				Str("runtime_previous", string(a.runtime)).
				Str("runtime_current", string(newRuntime)).
				Msg("Detected container runtime change")
		}
		a.runtime = newRuntime
		a.supportsSwarm = newRuntime == RuntimeDocker
		if newRuntime == RuntimePodman {
			if a.cfg.IncludeServices {
				a.logger.Warn().Msg("Podman runtime detected during report; disabling Swarm service collection")
			}
			if a.cfg.IncludeTasks {
				a.logger.Warn().Msg("Podman runtime detected during report; disabling Swarm task collection")
			}
			a.cfg.IncludeServices = false
			a.cfg.IncludeTasks = false
		}
		a.cfg.Runtime = string(newRuntime)
	}

	a.cpuCount = info.NCPU

	agentID := a.cfg.AgentID
	if agentID == "" {
		// In unified mode, use the EXACT same fallback chain as hostagent:
		// machineID -> hostname. Never use daemonID in unified mode because
		// hostagent doesn't use it, and using different IDs causes token
		// binding conflicts on the server (reported in #985, #986).
		if a.cfg.AgentType == "unified" {
			agentID = a.machineID
			if agentID == "" {
				agentID = a.hostName
			}
		} else {
			// Standalone mode: prefer daemonID for backward compatibility,
			// then fall back to machineID -> hostname.
			// Use cached daemon ID from init rather than info.ID from current call.
			// Podman can return different/empty IDs across calls, causing token
			// binding conflicts on the server.
			agentID = a.daemonID
			if agentID == "" {
				agentID = a.machineID
			}
			if agentID == "" {
				agentID = a.hostName
			}
		}
	}
	a.hostID = agentID

	hostName := a.hostName
	if hostName == "" {
		hostName = info.Name
	}

	uptime := readSystemUptime()

	metricsCtx, metricsCancel := context.WithTimeout(ctx, 10*time.Second)
	snapshot, err := hostmetricsCollect(metricsCtx, a.cfg.DiskExclude)
	metricsCancel()
	if err != nil {
		return agentsdocker.Report{}, fmt.Errorf("collect host metrics: %w", err)
	}

	collectContainers := a.cfg.IncludeContainers
	if !collectContainers && (a.cfg.IncludeServices || a.cfg.IncludeTasks) && !info.Swarm.ControlAvailable {
		collectContainers = true
	}

	var containers []agentsdocker.Container
	if collectContainers {
		var err error
		containers, err = a.collectContainers(ctx)
		if err != nil {
			return agentsdocker.Report{}, err
		}
	}

	services, tasks, swarmInfo := a.collectSwarmData(ctx, info, containers)

	// Use Docker's MemTotal, but fall back to gopsutil's reading if Docker returns 0.
	// This can happen in Docker-in-LXC setups where Docker daemon can't read host memory.
	totalMemory := info.MemTotal
	if totalMemory <= 0 && snapshot.Memory.TotalBytes > 0 {
		totalMemory = snapshot.Memory.TotalBytes
	}

	report := agentsdocker.Report{
		Agent: agentsdocker.AgentInfo{
			ID:              agentID,
			Version:         a.agentVersion,
			Type:            a.cfg.AgentType,
			IntervalSeconds: int(a.cfg.Interval / time.Second),
		},
		Host: agentsdocker.HostInfo{
			Hostname:         hostName,
			Name:             info.Name,
			MachineID:        a.machineID,
			OS:               info.OperatingSystem,
			Runtime:          string(a.runtime),
			RuntimeVersion:   a.runtimeVer,
			KernelVersion:    info.KernelVersion,
			Architecture:     info.Architecture,
			DockerVersion:    info.ServerVersion,
			TotalCPU:         info.NCPU,
			TotalMemoryBytes: totalMemory,
			UptimeSeconds:    uptime,
			CPUUsagePercent:  safeFloat(snapshot.CPUUsagePercent),
			LoadAverage:      append([]float64(nil), snapshot.LoadAverage...),
			Memory:           snapshot.Memory,
			Disks:            append([]agentsdocker.Disk(nil), snapshot.Disks...),
			Network:          append([]agentsdocker.NetworkInterface(nil), snapshot.Network...),
		},
		Timestamp: time.Now().UTC(),
	}

	if swarmInfo != nil {
		report.Host.Swarm = swarmInfo
	}

	if a.cfg.IncludeContainers {
		report.Containers = containers
	}
	if a.cfg.IncludeServices && len(services) > 0 {
		report.Services = services
	}
	if a.cfg.IncludeTasks && len(tasks) > 0 {
		report.Tasks = tasks
	}

	if report.Agent.IntervalSeconds <= 0 {
		report.Agent.IntervalSeconds = int(30 * time.Second / time.Second)
	}

	return report, nil
}

func (a *Agent) collectContainers(ctx context.Context) ([]agentsdocker.Container, error) {
	options := containertypes.ListOptions{All: true}
	if len(a.stateFilters) > 0 {
		filterArgs := filters.NewArgs()
		for _, state := range a.stateFilters {
			filterArgs.Add("status", state)
		}
		options.Filters = filterArgs
	}

	list, err := a.docker.ContainerList(ctx, options)
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	containers := make([]agentsdocker.Container, 0, len(list))
	active := make(map[string]struct{}, len(list))
	for _, summary := range list {
		if len(a.allowedStates) > 0 {
			if _, ok := a.allowedStates[strings.ToLower(summary.State)]; !ok {
				continue
			}
		}

		// Skip backup containers created during updates - they're temporary
		if isBackupContainer(summary.Names) {
			continue
		}

		active[summary.ID] = struct{}{}

		container, err := a.collectContainer(ctx, summary)
		if err != nil {
			a.logger.Warn().Str("container", strings.Join(summary.Names, ",")).Err(err).Msg("Failed to collect container stats")
			continue
		}
		containers = append(containers, container)
	}
	a.pruneStaleCPUSamples(active)
	return containers, nil
}

func (a *Agent) pruneStaleCPUSamples(active map[string]struct{}) {
	a.cpuMu.Lock()
	defer a.cpuMu.Unlock()

	if len(a.prevContainerCPU) == 0 {
		return
	}

	for containerID := range a.prevContainerCPU {
		if _, ok := active[containerID]; !ok {
			delete(a.prevContainerCPU, containerID)
			// Reset stats failure counter when containers are removed,
			// though it's global per agent so not strictly necessary but good hygiene
		}
	}
}

func (a *Agent) collectContainer(ctx context.Context, summary containertypes.Summary) (agentsdocker.Container, error) {
	const perContainerTimeout = 15 * time.Second

	containerCtx, cancel := context.WithTimeout(ctx, perContainerTimeout)
	defer cancel()

	requestSize := a.cfg.CollectDiskMetrics
	inspect, _, err := a.docker.ContainerInspectWithRaw(containerCtx, summary.ID, requestSize)
	if err != nil {
		return agentsdocker.Container{}, fmt.Errorf("inspect: %w", err)
	}

	var (
		cpuPercent float64
		memUsage   int64
		memLimit   int64
		memPercent float64
		blockIO    *agentsdocker.ContainerBlockIO
		networkRX  uint64
		networkTX  uint64
	)

	if inspect.State.Running || inspect.State.Paused {
		statsResp, err := a.docker.ContainerStatsOneShot(containerCtx, summary.ID)
		if err != nil {
			return agentsdocker.Container{}, fmt.Errorf("stats: %w", err)
		}
		defer statsResp.Body.Close()

		var stats containertypes.StatsResponse
		if err := json.NewDecoder(statsResp.Body).Decode(&stats); err != nil {
			return agentsdocker.Container{}, fmt.Errorf("decode stats: %w", err)
		}

		cpuPercent = a.calculateContainerCPUPercent(summary.ID, stats)
		memUsage, memLimit, memPercent = calculateMemoryUsage(stats)
		blockIO = summarizeBlockIO(stats)
		networkRX, networkTX = summarizeNetworkIO(stats)
	} else {
		a.cpuMu.Lock()
		delete(a.prevContainerCPU, summary.ID)
		a.cpuMu.Unlock()
	}

	createdAt := time.Unix(summary.Created, 0)

	startedAt := parseTime(inspect.State.StartedAt)
	finishedAt := parseTime(inspect.State.FinishedAt)

	uptimeSeconds := int64(0)
	if !startedAt.IsZero() && inspect.State.Running {
		uptimeSeconds = int64(time.Since(startedAt).Seconds())
		if uptimeSeconds < 0 {
			uptimeSeconds = 0
		}
	}

	health := ""
	if inspect.State.Health != nil {
		health = inspect.State.Health.Status
	}

	ports := make([]agentsdocker.ContainerPort, len(summary.Ports))
	for i, port := range summary.Ports {
		ports[i] = agentsdocker.ContainerPort{
			PrivatePort: int(port.PrivatePort),
			PublicPort:  int(port.PublicPort),
			Protocol:    port.Type,
			IP:          port.IP,
		}
	}

	labels := make(map[string]string, len(summary.Labels))
	for k, v := range summary.Labels {
		labels[k] = v
	}

	networks := make([]agentsdocker.ContainerNetwork, 0)
	if inspect.NetworkSettings != nil {
		for name, cfg := range inspect.NetworkSettings.Networks {
			networks = append(networks, agentsdocker.ContainerNetwork{
				Name: name,
				IPv4: cfg.IPAddress,
				IPv6: cfg.GlobalIPv6Address,
			})
		}
	}

	var startedPtr, finishedPtr *time.Time
	if !startedAt.IsZero() {
		started := startedAt
		startedPtr = &started
	}
	if !finishedAt.IsZero() && !inspect.State.Running {
		finished := finishedAt
		finishedPtr = &finished
	}

	var writableLayerBytes int64
	if inspect.SizeRw != nil {
		writableLayerBytes = *inspect.SizeRw
	}

	var rootFsBytes int64
	if inspect.SizeRootFs != nil {
		rootFsBytes = *inspect.SizeRootFs
	}

	var mounts []agentsdocker.ContainerMount
	if len(inspect.Mounts) > 0 {
		mounts = make([]agentsdocker.ContainerMount, 0, len(inspect.Mounts))
		for _, mount := range inspect.Mounts {
			mounts = append(mounts, agentsdocker.ContainerMount{
				Type:        string(mount.Type),
				Source:      mount.Source,
				Destination: mount.Destination,
				Mode:        mount.Mode,
				RW:          mount.RW,
				Propagation: string(mount.Propagation),
				Name:        mount.Name,
				Driver:      mount.Driver,
			})
		}
	}

	container := agentsdocker.Container{
		ID:                  summary.ID,
		Name:                trimLeadingSlash(summary.Names),
		Image:               summary.Image,
		ImageDigest:         summary.ImageID, // sha256:... digest of the image
		CreatedAt:           createdAt,
		State:               summary.State,
		Status:              summary.Status,
		Health:              health,
		CPUPercent:          cpuPercent,
		MemoryUsageBytes:    memUsage,
		MemoryLimitBytes:    memLimit,
		MemoryPercent:       memPercent,
		UptimeSeconds:       uptimeSeconds,
		RestartCount:        inspect.RestartCount,
		ExitCode:            inspect.State.ExitCode,
		StartedAt:           startedPtr,
		FinishedAt:          finishedPtr,
		Ports:               ports,
		Labels:              labels,
		Env:                 maskSensitiveEnvVars(inspect.Config.Env),
		Networks:            networks,
		NetworkRXBytes:      networkRX,
		NetworkTXBytes:      networkTX,
		WritableLayerBytes:  writableLayerBytes,
		RootFilesystemBytes: rootFsBytes,
		BlockIO:             blockIO,
		Mounts:              mounts,
	}

	if a.runtime == RuntimePodman {
		if meta := extractPodmanMetadata(labels); meta != nil {
			container.Podman = meta
		}
	}

	// Check for image updates if registry checker is enabled
	if a.registryChecker != nil && a.registryChecker.Enabled() {
		// Get the actual manifest digest (RepoDigest) from the image for accurate comparison.
		// The ImageID is a local content-addressable ID that differs from the registry manifest digest.
		// We also get the architecture details to correctly resolve manifest lists from the registry.
		digestForComparison, arch, os, variant := a.getImageRepoDigest(containerCtx, summary.ImageID, summary.Image)

		var imageToCheck string
		// Always prefer the image name from inspect config as it's the authoritative source
		// and avoids issues with short IDs or digests in summary.
		// HOWEVER, if the config image IS a digest (starts with sha256:), fall back to container.Image
		// which usually contains the human-readable tag/name.
		imageToCheck = container.Image
		if inspect.Config != nil && inspect.Config.Image != "" {
			if !strings.HasPrefix(inspect.Config.Image, "sha256:") {
				imageToCheck = inspect.Config.Image
			}
		}

		// Additional safety: if imageToCheck is still a SHA, we can't check it
		if strings.HasPrefix(imageToCheck, "sha256:") {
			container.UpdateStatus = &agentsdocker.UpdateStatus{
				UpdateAvailable: false,
				CurrentDigest:   digestForComparison,
				LastChecked:     time.Now(),
				Error:           "digest-pinned image",
			}
			// Skip to end of update check block - don't call registry
		} else {
			a.logger.Debug().
				Str("container", container.Name).
				Str("image", imageToCheck).
				Str("compareDigest", digestForComparison).
				Str("arch", arch).
				Str("os", os).
				Str("variant", variant).
				Msg("Checking update for container")

			result := a.registryChecker.CheckImageUpdate(ctx, imageToCheck, digestForComparison, arch, os, variant)
			if result != nil {
				container.UpdateStatus = &agentsdocker.UpdateStatus{
					UpdateAvailable: result.UpdateAvailable,
					CurrentDigest:   result.CurrentDigest,
					LatestDigest:    result.LatestDigest,
					LastChecked:     result.CheckedAt,
					Error:           result.Error,
				}
			}
		}
	}

	if requestSize {
		a.logger.Debug().
			Str("container", container.Name).
			Int64("writableLayerBytes", writableLayerBytes).
			Int64("rootFilesystemBytes", rootFsBytes).
			Int("mountCount", len(mounts)).
			Msg("Collected container disk metrics")
	}

	return container, nil
}

// getImageRepoDigest retrieves the RepoDigest for an image and its platform details.
// It returns the digest, architecture, OS, and variant.
func (a *Agent) getImageRepoDigest(ctx context.Context, imageID, imageName string) (string, string, string, string) {
	imageInspect, _, err := a.docker.ImageInspectWithRaw(ctx, imageID)
	if err != nil {
		a.logger.Debug().
			Err(err).
			Str("imageID", imageID).
			Str("imageName", imageName).
			Msg("Failed to inspect image for RepoDigest")
		return "", "", "", ""
	}

	arch := imageInspect.Architecture
	os := imageInspect.Os
	variant := imageInspect.Variant

	if len(imageInspect.RepoDigests) == 0 {
		// Locally built images won't have RepoDigests
		return "", arch, os, variant
	}

	// Try to find a RepoDigest that matches the image reference
	// RepoDigests format: "registry/repo@sha256:..."
	for _, repoDigest := range imageInspect.RepoDigests {
		// Extract just the digest part (after @)
		if idx := strings.LastIndex(repoDigest, "@"); idx >= 0 {
			repoRef := repoDigest[:idx]  // e.g., "docker.io/library/nginx"
			digest := repoDigest[idx+1:] // e.g., "sha256:abc..."

			// Check if this RepoDigest matches our image reference
			// Normalize both for comparison
			if matchesImageReference(imageName, repoRef) {
				return digest, arch, os, variant
			}
		}
	}

	// If no exact match, return the first RepoDigest's digest
	// This handles cases where the image was pulled with a different tag
	if idx := strings.LastIndex(imageInspect.RepoDigests[0], "@"); idx >= 0 {
		return imageInspect.RepoDigests[0][idx+1:], arch, os, variant
	}

	return "", arch, os, variant
}

// matchesImageReference checks if a RepoDigest repository matches an image reference.
// It handles Docker Hub's various naming conventions.
func matchesImageReference(imageName, repoRef string) bool {
	// Normalize image name by removing tag
	if idx := strings.LastIndex(imageName, ":"); idx >= 0 {
		// Make sure it's a tag, not a port (check if there's a / after it)
		if !strings.Contains(imageName[idx:], "/") {
			imageName = imageName[:idx]
		}
	}

	// Direct match
	if imageName == repoRef {
		return true
	}

	// Docker Hub library images: "nginx" == "docker.io/library/nginx"
	if repoRef == "docker.io/library/"+imageName {
		return true
	}

	// Docker Hub with namespace: "myuser/myapp" == "docker.io/myuser/myapp"
	if repoRef == "docker.io/"+imageName {
		return true
	}

	// Registry prefix matching (e.g., "ghcr.io/user/repo" matches "ghcr.io/user/repo")
	// Already handled by direct match above

	return false
}

func extractPodmanMetadata(labels map[string]string) *agentsdocker.PodmanContainer {
	if len(labels) == 0 {
		return nil
	}

	meta := &agentsdocker.PodmanContainer{}

	if v := strings.TrimSpace(labels["io.podman.annotations.pod.name"]); v != "" {
		meta.PodName = v
	}

	if v := strings.TrimSpace(labels["io.podman.annotations.pod.id"]); v != "" {
		meta.PodID = v
	}

	if v := strings.TrimSpace(labels["io.podman.annotations.pod.infra"]); v != "" {
		if parsed, err := strconv.ParseBool(v); err == nil {
			meta.Infra = parsed
		} else if strings.EqualFold(v, "yes") || strings.EqualFold(v, "true") {
			meta.Infra = true
		}
	}

	if v := strings.TrimSpace(labels["io.podman.compose.project"]); v != "" {
		meta.ComposeProject = v
	}

	if v := strings.TrimSpace(labels["io.podman.compose.service"]); v != "" {
		meta.ComposeService = v
	}

	if v := strings.TrimSpace(labels["io.podman.compose.working_dir"]); v != "" {
		meta.ComposeWorkdir = v
	}

	if v := strings.TrimSpace(labels["io.podman.compose.config-hash"]); v != "" {
		meta.ComposeConfig = v
	}

	if v := strings.TrimSpace(labels["io.containers.autoupdate"]); v != "" {
		meta.AutoUpdatePolicy = v
	}

	if v := strings.TrimSpace(labels["io.containers.autoupdate.restart"]); v != "" {
		meta.AutoUpdateRestart = v
	}

	if v := strings.TrimSpace(labels["io.podman.annotations.userns"]); v != "" {
		meta.UserNS = v
	} else if v := strings.TrimSpace(labels["io.containers.userns"]); v != "" {
		meta.UserNS = v
	}

	if meta.PodName == "" && meta.PodID == "" && meta.ComposeProject == "" && meta.AutoUpdatePolicy == "" && meta.UserNS == "" && !meta.Infra {
		return nil
	}

	return meta
}

func (a *Agent) calculateContainerCPUPercent(containerID string, stats containertypes.StatsResponse) float64 {
	a.cpuMu.Lock()
	defer a.cpuMu.Unlock()

	current := cpuSample{
		totalUsage:  stats.CPUStats.CPUUsage.TotalUsage,
		systemUsage: stats.CPUStats.SystemUsage,
		onlineCPUs:  stats.CPUStats.OnlineCPUs,
		read:        stats.Read,
	}

	// Try to use PreCPUStats if available
	percent := calculateCPUPercent(stats, a.cpuCount)
	if percent > 0 {
		a.prevContainerCPU[containerID] = current
		a.logger.Debug().
			Str("container_id", containerID[:12]).
			Float64("cpu_percent", percent).
			Msg("CPU calculated from PreCPUStats")
		return percent
	}

	// PreCPUStats not available or invalid, use manual tracking
	a.preCPUStatsFailures++
	if a.preCPUStatsFailures == 10 {
		a.logger.Warn().
			Str("runtime", string(a.runtime)).
			Msg("PreCPUStats consistently unavailable from Docker API - using manual CPU tracking (this is normal for one-shot stats)")
	}
	prev, ok := a.prevContainerCPU[containerID]
	if !ok {
		// First time seeing this container - store current sample and return 0
		// On next collection cycle we'll have a previous sample to compare against
		a.prevContainerCPU[containerID] = current
		a.logger.Debug().
			Str("container_id", containerID[:12]).
			Uint64("total_usage", current.totalUsage).
			Uint64("system_usage", current.systemUsage).
			Msg("First CPU sample collected, no previous data for delta calculation")
		return 0
	}

	// We have a previous sample - update it after calculation
	a.prevContainerCPU[containerID] = current

	var totalDelta float64
	if current.totalUsage >= prev.totalUsage {
		totalDelta = float64(current.totalUsage - prev.totalUsage)
	} else {
		// Counter likely reset (container restart); fall back to current reading.
		totalDelta = float64(current.totalUsage)
	}

	if totalDelta <= 0 {
		return 0
	}

	onlineCPUs := current.onlineCPUs
	if onlineCPUs == 0 {
		onlineCPUs = prev.onlineCPUs
	}
	if onlineCPUs == 0 && a.cpuCount > 0 {
		onlineCPUs = uint32(a.cpuCount)
	}
	if onlineCPUs == 0 {
		return 0
	}

	var systemDelta float64
	if current.systemUsage >= prev.systemUsage {
		systemDelta = float64(current.systemUsage - prev.systemUsage)
	}
	// If systemUsage went backward (counter reset), leave systemDelta as 0
	// to fall through to time-based calculation below

	if systemDelta > 0 {
		cpuPercent := safeFloat((totalDelta / systemDelta) * float64(onlineCPUs) * 100.0)
		a.logger.Debug().
			Str("container_id", containerID[:12]).
			Float64("cpu_percent", cpuPercent).
			Float64("total_delta", totalDelta).
			Float64("system_delta", systemDelta).
			Uint32("online_cpus", onlineCPUs).
			Msg("CPU calculated from system delta")
		return cpuPercent
	}

	// Fall back to time-based calculation
	if !prev.read.IsZero() && !current.read.IsZero() {
		elapsed := current.read.Sub(prev.read).Seconds()
		if elapsed > 0 {
			denominator := elapsed * float64(onlineCPUs) * 1e9
			if denominator > 0 {
				cpuPercent := (totalDelta / denominator) * 100.0
				result := safeFloat(cpuPercent)
				a.logger.Debug().
					Str("container_id", containerID[:12]).
					Float64("cpu_percent", result).
					Float64("total_delta", totalDelta).
					Float64("elapsed_seconds", elapsed).
					Uint32("online_cpus", onlineCPUs).
					Msg("CPU calculated from time-based delta")
				return result
			}
		}
	}

	a.logger.Debug().
		Str("container_id", containerID[:12]).
		Float64("total_delta", totalDelta).
		Float64("system_delta", systemDelta).
		Bool("prev_read_zero", prev.read.IsZero()).
		Bool("current_read_zero", current.read.IsZero()).
		Msg("CPU calculation failed: no valid delta method available")
	return 0
}

func calculateCPUPercent(stats containertypes.StatsResponse, hostCPUs int) float64 {
	totalDelta := float64(stats.CPUStats.CPUUsage.TotalUsage - stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemUsage - stats.PreCPUStats.SystemUsage)

	if totalDelta <= 0 || systemDelta <= 0 {
		return 0
	}

	onlineCPUs := stats.CPUStats.OnlineCPUs
	if onlineCPUs == 0 {
		onlineCPUs = uint32(len(stats.CPUStats.CPUUsage.PercpuUsage))
	}
	if onlineCPUs == 0 && hostCPUs > 0 {
		onlineCPUs = uint32(hostCPUs)
	}

	if onlineCPUs == 0 {
		return 0
	}

	return safeFloat((totalDelta / systemDelta) * float64(onlineCPUs) * 100.0)
}

func calculateMemoryUsage(stats containertypes.StatsResponse) (usage int64, limit int64, percent float64) {
	usage = int64(stats.MemoryStats.Usage)

	// Subtract reclaimable cache from usage to match `docker stats` behavior.
	// Docker subtracts cache/file to show "actual" memory usage rather than
	// memory.current which includes reclaimable filesystem cache.
	//
	// cgroup v1: "cache" stat contains the reclaimable cache
	// cgroup v2: "cache" doesn't exist, use "inactive_file" (preferred) or "file"
	var cacheBytes uint64
	if cache, ok := stats.MemoryStats.Stats["cache"]; ok {
		// cgroup v1
		cacheBytes = cache
	} else if inactiveFile, ok := stats.MemoryStats.Stats["inactive_file"]; ok {
		// cgroup v2: inactive_file is the reclaimable portion of file cache
		// This matches what docker CLI does internally
		cacheBytes = inactiveFile
	}

	if cacheBytes > 0 && int64(cacheBytes) < usage {
		usage -= int64(cacheBytes)
	}

	limit = int64(stats.MemoryStats.Limit)
	if limit > 0 {
		percent = (float64(usage) / float64(limit)) * 100.0
	}

	return usage, limit, safeFloat(percent)
}

func safeFloat(val float64) float64 {
	if math.IsNaN(val) || math.IsInf(val, 0) {
		return 0
	}
	return val
}

func parseTime(value string) time.Time {
	if value == "" || value == "0001-01-01T00:00:00Z" {
		return time.Time{}
	}
	if strings.Contains(value, ".") {
		if t, err := time.Parse(time.RFC3339Nano, value); err == nil {
			return t
		}
	} else {
		if t, err := time.Parse(time.RFC3339, value); err == nil {
			return t
		}
	}
	return time.Time{}
}

func trimLeadingSlash(names []string) string {
	if len(names) == 0 {
		return ""
	}
	name := names[0]
	return strings.TrimPrefix(name, "/")
}

func randomDuration(max time.Duration) time.Duration {
	if max <= 0 {
		return 0
	}

	n, err := randIntFn(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0
	}

	return time.Duration(n.Int64())
}

func summarizeBlockIO(stats containertypes.StatsResponse) *agentsdocker.ContainerBlockIO {
	// BlkioStats structure varies by cgroup version
	// Cgroup v1: IoServiceBytesRecursive []BlkioStatEntry
	// Cgroup v2: IoServiceBytesRecursive is empty? No, Docker maps it?
	// Docker API guarantees IoServiceBytesRecursive is populated?
	// It seems to try to handle both.

	if len(stats.BlkioStats.IoServiceBytesRecursive) == 0 {
		return nil
	}

	var readBytes, writeBytes uint64

	for _, entry := range stats.BlkioStats.IoServiceBytesRecursive {
		op := strings.ToLower(entry.Op)
		switch op {
		case "read":
			readBytes += entry.Value
		case "write":
			writeBytes += entry.Value
		}
	}

	if readBytes == 0 && writeBytes == 0 {
		return nil
	}

	return &agentsdocker.ContainerBlockIO{
		ReadBytes:  readBytes,
		WriteBytes: writeBytes,
	}
}

func summarizeNetworkIO(stats containertypes.StatsResponse) (uint64, uint64) {
	if len(stats.Networks) == 0 {
		return 0, 0
	}

	var rxBytes uint64
	var txBytes uint64
	for _, network := range stats.Networks {
		rxBytes += network.RxBytes
		txBytes += network.TxBytes
	}

	return rxBytes, txBytes
}

// sensitiveEnvPatterns are substrings that, when found in an env var name (case-insensitive),
// indicate the value should be masked for security.
var sensitiveEnvPatterns = []string{
	"password", "passwd", "secret", "key", "token", "credential", "auth",
	"api_key", "apikey", "private", "access_token", "refresh_token",
	"database_url", "connection_string", "encryption",
}

// maskSensitiveEnvVars returns a copy of the environment variables with sensitive values masked.
// Environment variables whose names contain sensitive keywords will have their values replaced with "***".
func maskSensitiveEnvVars(envVars []string) []string {
	if len(envVars) == 0 {
		return nil
	}

	result := make([]string, 0, len(envVars))
	for _, env := range envVars {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			result = append(result, env)
			continue
		}

		name := parts[0]
		value := parts[1]

		// Check if the environment variable name contains a sensitive pattern
		lowerName := strings.ToLower(name)
		isSensitive := false
		for _, pattern := range sensitiveEnvPatterns {
			if strings.Contains(lowerName, pattern) {
				isSensitive = true
				break
			}
		}

		if isSensitive && value != "" {
			result = append(result, name+"=***")
		} else {
			result = append(result, env)
		}
	}

	return result
}
