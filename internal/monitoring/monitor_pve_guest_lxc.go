package monitoring

import (
	"context"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/rs/zerolog/log"
)

func (m *Monitor) calculateLXCMemory(
	res proxmox.ClusterResource,
) (uint64, uint64, string, VMMemoryRaw) {
	// The cluster resources mem for a container is its cgroup usage with page
	// cache already subtracted: PVE::CGroup::get_memory_stat reports
	// memory.current - memory.stat file (cgroup v2) or usage_in_bytes -
	// total_cache (v1), and pvestatd feeds that vmstatus value to the listing.
	// It is a cache-excluded reading against the container's own limit, unlike
	// a QEMU guest's listing mem. Guest RRD carries the same mem/maxmem pair,
	// so the listing value is the evidence for running containers (issue #1634).
	memTotal := res.MaxMem
	memUsed := uint64(0)
	memorySource := "powered-off"
	guestRaw := VMMemoryRaw{
		ListingMem:    res.Mem,
		ListingMaxMem: res.MaxMem,
	}

	if res.Status == "running" {
		memorySource = "unavailable"
		if res.Mem > 0 {
			memUsed = res.Mem
			memorySource = "cluster-resources"
		}
	}

	return memTotal, memUsed, memorySource, guestRaw
}

func (m *Monitor) buildContainerFromClusterResource(
	ctx context.Context,
	instanceName string,
	res proxmox.ClusterResource,
	client PVEClientInterface,
	prevContainerIsOCI map[int]bool,
	vmIDToHostAgent map[string]models.Host,
) (models.Container, VMMemoryRaw, string, time.Time, bool) {
	// Skip templates if configured
	if res.Template == 1 {
		return models.Container{}, VMMemoryRaw{}, "", time.Time{}, false
	}

	guestID := makeGuestID(instanceName, res.Node, res.VMID)

	sampleTime := time.Now()
	counterObservedAt := observedAtOr(res.ObservedAt, sampleTime)
	currentMetrics := IOMetrics{
		DiskRead:     int64(res.DiskRead),
		DiskWrite:    int64(res.DiskWrite),
		NetworkIn:    int64(res.NetIn),
		NetworkOut:   int64(res.NetOut),
		Timestamp:    counterObservedAt,
		Presence:     pveCounterPresence(res.IOCounters),
		ObservedAt:   counterObservationTimes(counterObservedAt),
		SourceUptime: res.Uptime,
	}
	statusSnapshot := (*proxmox.Container)(nil)
	if res.Status == "running" {
		statusSnapshot = m.fetchContainerStatusSnapshot(
			ctx,
			client,
			instanceName,
			res.Node,
			res.Name,
			res.VMID,
		)
		currentMetrics = mergeContainerRuntimeCounters(currentMetrics, statusSnapshot)
		// The container may have stopped after the listing was sampled. Use the
		// already-fetched complete status before selecting running-only readings
		// or enrichment; an older listing is not evidence it is still running.
		if statusSnapshot != nil && statusSnapshot.Status != "" {
			res.Status = statusSnapshot.Status
		}
	}
	diskReadRate, diskWriteRate, netInRate, netOutRate := m.rateTracker.CalculateRates(
		makeGuestRateKey(instanceName, "lxc", res.VMID),
		currentMetrics,
	)
	diskReadValue, diskWriteValue, networkInValue, networkOutValue, rateValidity := guestRateValues(
		diskReadRate,
		diskWriteRate,
		netInRate,
		netOutRate,
	)

	memTotal, memUsed, memorySource, guestRaw := m.calculateLXCMemory(res)
	agentHost, hasAgent := vmIDToHostAgent[guestID]
	memTotal, memUsed, memorySource = preferLinkedAgentLXCMemory(
		res.Status,
		memTotal,
		memUsed,
		memorySource,
		&guestRaw,
		agentHost,
		hasAgent,
	)
	previousMemory := m.previousGuestSnapshot(instanceName, "lxc", res.Node, res.VMID)
	memUsed, memorySource, _ = stabilizeGuestLowTrustMemory(
		previousMemory,
		res.Status,
		memorySource,
		memTotal,
		memUsed,
		sampleTime,
		false,
	)

	// Clamp memory values to prevent >100% usage
	// (Proxmox can report used > total for LXC due to cgroup accounting,
	// or shared pages)
	clampedMemUsed := memUsed
	if clampedMemUsed > memTotal && memTotal > 0 {
		clampedMemUsed = memTotal
	}
	memFree := int64(memTotal) - int64(clampedMemUsed)
	if memFree < 0 {
		memFree = 0
	}
	memory := models.UnavailableMemory(clampToInt64(memTotal))
	if CanonicalMemorySource(memorySource) != "unavailable" {
		memory = models.Memory{
			Total: int64(memTotal),
			Used:  int64(clampedMemUsed),
			Free:  memFree,
			Usage: safePercentage(float64(clampedMemUsed), float64(memTotal)),
		}
	}
	memory.Observation = m.guestMemoryObservation(instanceName, "lxc", res.Node, res.VMID, memory, guestMemoryObservationContext{
		source: memorySource, providerObservedAt: observedAtOr(res.ObservedAt, sampleTime),
		previous: previousMemory, linkedAgent: agentHost,
	}, time.Now())

	lastSeen := time.Now()
	container := models.Container{
		ID:             guestID,
		VMID:           res.VMID,
		Name:           res.Name,
		Node:           res.Node,
		Pool:           strings.TrimSpace(res.Pool),
		Instance:       instanceName,
		Status:         res.Status,
		Type:           "lxc",
		CPU:            guestCPUUsage(res.CPU),
		CPUs:           res.MaxCPU,
		Memory:         memory,
		Disk:           containerDiskUsage(res.MaxDisk, res.Disk, res.DiskUsage),
		NetworkIn:      networkInValue,
		NetworkOut:     networkOutValue,
		DiskRead:       diskReadValue,
		DiskWrite:      diskWriteValue,
		IORateValidity: rateValidity,
		Uptime:         int64(res.Uptime),
		Template:       res.Template == 1,
		LastSeen:       lastSeen,
	}

	if prevContainerIsOCI[container.VMID] {
		container.IsOCI = true
		container.Type = "oci"
	}

	// Parse tags
	if res.Tags != "" {
		container.Tags = strings.Split(res.Tags, ";")

		// Log if Pulse-specific tags are detected
		for _, tag := range container.Tags {
			switch tag {
			case "pulse-no-alerts", "pulse-monitor-only", "pulse-relaxed":
				log.Info().
					Str("container", container.Name).
					Str("node", container.Node).
					Str("tag", tag).
					Msg("Pulse control tag detected on container")
			}
		}
	}

	m.enrichContainerMetadata(ctx, client, instanceName, res.Node, &container, statusSnapshot)
	// The per-node fallback path applies node-local pct df data after metadata
	// enrichment (monitor_polling_containers.go); the efficient cluster/resources
	// path must do the same or installs served by it never surface the host
	// agent's per-mount usage and fall back to config-listed mounts with
	// unknown usage (#1477).
	m.enrichContainerWithAgentLXCFilesystems(instanceName, res.Node, &container, sampleTime)

	// Non-running containers have no current runtime CPU/memory/I/O usage.
	// Platform storage usage remains an independent observation; a stopped
	// container still occupies its filesystems. Alerting already excludes
	// metric alerts for non-running guests, so do not invent an empty disk.
	if container.Status != "running" {
		log.Debug().
			Str("container", container.Name).
			Str("status", container.Status).
			Float64("originalCpu", container.CPU).
			Float64("originalMemUsage", container.Memory.Usage).
			Msg("Non-running container detected - zeroing metrics")

		container.CPU = 0
		container.Memory.Usage = 0
		container.NetworkIn = 0
		container.NetworkOut = 0
		container.DiskRead = 0
		container.DiskWrite = 0
		container.IORateValidity = models.IORateValidity{
			Explicit:   true,
			DiskRead:   true,
			DiskWrite:  true,
			NetworkIn:  true,
			NetworkOut: true,
		}
	}

	return container, guestRaw, memorySource, sampleTime, true
}
