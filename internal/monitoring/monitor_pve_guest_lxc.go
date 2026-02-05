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
	ctx context.Context,
	instanceName string,
	res proxmox.ClusterResource,
	client PVEClientInterface,
) (uint64, uint64, string, VMMemoryRaw) {
	// Calculate cache-aware memory for LXC containers
	// The cluster resources API returns mem from cgroup which includes cache/buffers (inflated).
	// Try to get more accurate memory metrics from RRD data.
	memTotal := res.MaxMem
	memUsed := res.Mem
	memorySource := "cluster-resources"
	guestRaw := VMMemoryRaw{
		ListingMem:    res.Mem,
		ListingMaxMem: res.MaxMem,
	}

	// For running containers, try to get RRD data for cache-aware memory calculation
	if res.Status == "running" {
		rrdCtx, rrdCancel := context.WithTimeout(ctx, 5*time.Second)
		rrdPoints, err := client.GetLXCRRDData(rrdCtx, res.Node, res.VMID, "hour", "AVERAGE", []string{"memavailable", "memused", "maxmem"})
		rrdCancel()

		if err == nil && len(rrdPoints) > 0 {
			// Use the most recent RRD point
			point := rrdPoints[len(rrdPoints)-1]

			if point.MaxMem != nil && *point.MaxMem > 0 {
				guestRaw.StatusMaxMem = uint64(*point.MaxMem)
			}

			// Prefer memavailable-based calculation (excludes cache/buffers)
			if point.MemAvailable != nil && *point.MemAvailable > 0 {
				memAvailable := uint64(*point.MemAvailable)
				if memAvailable <= memTotal {
					memUsed = memTotal - memAvailable
					memorySource = "rrd-memavailable"
					guestRaw.MemInfoAvailable = memAvailable
					log.Debug().
						Str("container", res.Name).
						Str("node", res.Node).
						Uint64("total", memTotal).
						Uint64("available", memAvailable).
						Uint64("used", memUsed).
						Float64("usage", safePercentage(float64(memUsed), float64(memTotal))).
						Msg("LXC memory: using RRD memavailable (excludes reclaimable cache)")
				}
			} else if point.MemUsed != nil && *point.MemUsed > 0 {
				// Fall back to memused from RRD if available
				memUsed = uint64(*point.MemUsed)
				if memUsed <= memTotal {
					memorySource = "rrd-memused"
					guestRaw.MemInfoUsed = memUsed
					log.Debug().
						Str("container", res.Name).
						Str("node", res.Node).
						Uint64("total", memTotal).
						Uint64("used", memUsed).
						Float64("usage", safePercentage(float64(memUsed), float64(memTotal))).
						Msg("LXC memory: using RRD memused (excludes reclaimable cache)")
				}
			}
		} else if err != nil {
			log.Debug().
				Err(err).
				Str("instance", instanceName).
				Str("container", res.Name).
				Int("vmid", res.VMID).
				Msg("RRD memory data unavailable for LXC, using cluster resources value")
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

	// Calculate I/O rates for container
	sampleTime := time.Now()
	currentMetrics := IOMetrics{
		DiskRead:   int64(res.DiskRead),
		DiskWrite:  int64(res.DiskWrite),
		NetworkIn:  int64(res.NetIn),
		NetworkOut: int64(res.NetOut),
		Timestamp:  sampleTime,
	}
	diskReadRate, diskWriteRate, netInRate, netOutRate := m.rateTracker.CalculateRates(guestID, currentMetrics)

	memTotal, memUsed, memorySource, guestRaw := m.calculateLXCMemory(ctx, instanceName, res, client)

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
		Instance: instanceName,
		Status:   res.Status,
		Type:     "lxc",
		CPU:      safeFloat(res.CPU),
		CPUs:     res.MaxCPU,
		Memory: models.Memory{
			Total: int64(memTotal),
			Used:  int64(clampedMemUsed),
			Free:  memFree,
			Usage: safePercentage(float64(clampedMemUsed), float64(memTotal)),
		},
		Disk: models.Disk{
			Total: int64(res.MaxDisk),
			Used:  int64(diskUsed),
			Free:  diskFree,
			Usage: safePercentage(float64(diskUsed), float64(res.MaxDisk)),
		},
		NetworkIn:  max(0, int64(netInRate)),
		NetworkOut: max(0, int64(netOutRate)),
		DiskRead:   max(0, int64(diskReadRate)),
		DiskWrite:  max(0, int64(diskWriteRate)),
		Uptime:     int64(res.Uptime),
		Template:   res.Template == 1,
		LastSeen:   lastSeen,
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

	m.enrichContainerMetadata(ctx, client, instanceName, res.Node, &container)

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
	}

	return container, guestRaw, memorySource, sampleTime, true
}
