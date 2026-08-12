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
	// The cluster resources API returns mem from cgroup which includes
	// cache/buffers (inflated). PVE guest RRD offers nothing better: it
	// carries only the same cache-inclusive mem/maxmem columns — the
	// cache-aware memused/memavailable columns exist only in node RRD — so
	// the listing value is the best evidence available for running
	// containers (issue #1634).
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
	memUsed, memorySource, _ = stabilizeGuestLowTrustMemory(
		m.previousGuestSnapshot(instanceName, "lxc", res.Node, res.VMID),
		res.Status,
		memorySource,
		memTotal,
		memUsed,
		sampleTime,
		false,
	)

	// Clamp memory and disk values to prevent >100% usage
	// (Proxmox can report used > total for LXC due to cgroup accounting,
	// shared pages, or thin-provisioned disk overcommit)
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
	diskUsed := res.Disk
	if diskUsed > res.MaxDisk && res.MaxDisk > 0 {
		diskUsed = res.MaxDisk
	}
	diskFree := int64(res.MaxDisk) - int64(diskUsed)
	if diskFree < 0 {
		diskFree = 0
	}

	lastSeen := time.Now()
	container := models.Container{
		ID:       guestID,
		VMID:     res.VMID,
		Name:     res.Name,
		Node:     res.Node,
		Pool:     strings.TrimSpace(res.Pool),
		Instance: instanceName,
		Status:   res.Status,
		Type:     "lxc",
		CPU:      safeFloat(res.CPU),
		CPUs:     res.MaxCPU,
		Memory:   memory,
		Disk: models.Disk{
			Total: int64(res.MaxDisk),
			Used:  int64(diskUsed),
			Free:  diskFree,
			Usage: safePercentage(float64(diskUsed), float64(res.MaxDisk)),
		},
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

	// For non-running containers, zero out resource usage metrics to prevent false alerts.
	// Proxmox may report stale or residual metrics for stopped containers.
	if container.Status != "running" {
		log.Debug().
			Str("container", container.Name).
			Str("status", container.Status).
			Float64("originalCpu", container.CPU).
			Float64("originalMemUsage", container.Memory.Usage).
			Msg("Non-running container detected - zeroing metrics")

		container.CPU = 0
		container.Memory.Usage = 0
		container.Disk.Usage = 0
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
