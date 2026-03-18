package monitoring

import (
	"context"
	"math"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/rs/zerolog/log"
)

// deriveGuestMemInfoAvailable normalizes guest meminfo-based availability
// selection so the efficient VM builder and the node-by-node VM polling path
// do not drift apart on degraded Proxmox guest-agent payloads.
func deriveGuestMemInfoAvailable(memInfo *proxmox.VMMemInfo, guestRaw *VMMemoryRaw) (uint64, string) {
	if memInfo == nil {
		return 0, ""
	}

	if guestRaw != nil {
		guestRaw.MemInfoUsed = memInfo.Used
		guestRaw.MemInfoFree = memInfo.Free
		guestRaw.MemInfoTotal = memInfo.Total
		guestRaw.MemInfoAvailable = memInfo.Available
		guestRaw.MemInfoBuffers = memInfo.Buffers
		guestRaw.MemInfoCached = memInfo.Cached
		guestRaw.MemInfoShared = memInfo.Shared
	}

	componentAvailable := memInfo.Free
	if memInfo.Buffers > 0 {
		if math.MaxUint64-componentAvailable < memInfo.Buffers {
			componentAvailable = math.MaxUint64
		} else {
			componentAvailable += memInfo.Buffers
		}
	}
	if memInfo.Cached > 0 {
		if math.MaxUint64-componentAvailable < memInfo.Cached {
			componentAvailable = math.MaxUint64
		} else {
			componentAvailable += memInfo.Cached
		}
	}
	if memInfo.Total > 0 && componentAvailable > memInfo.Total {
		componentAvailable = memInfo.Total
	}

	availableFromUsed := uint64(0)
	if memInfo.Total > 0 && memInfo.Used > 0 && memInfo.Total >= memInfo.Used {
		availableFromUsed = memInfo.Total - memInfo.Used
		if guestRaw != nil {
			guestRaw.MemInfoTotalMinusUsed = availableFromUsed
		}
	}

	missingCacheMetrics := memInfo.Available == 0 &&
		memInfo.Buffers == 0 &&
		memInfo.Cached == 0

	switch {
	case memInfo.Available > 0:
		return memInfo.Available, "available-field"
	case memInfo.Free > 0 || memInfo.Buffers > 0 || memInfo.Cached > 0:
		if availableFromUsed > 0 && missingCacheMetrics {
			const vmTotalMinusUsedGapTolerance uint64 = 4 * 1024 * 1024
			if availableFromUsed > componentAvailable {
				gap := availableFromUsed - componentAvailable
				if componentAvailable == 0 || gap >= vmTotalMinusUsedGapTolerance {
					return availableFromUsed, "derived-total-minus-used"
				}
			}
		}
		return componentAvailable, "derived-free-buffers-cached"
	default:
		if availableFromUsed > 0 && missingCacheMetrics {
			const vmTotalMinusUsedGapTolerance uint64 = 4 * 1024 * 1024
			if availableFromUsed > componentAvailable {
				gap := availableFromUsed - componentAvailable
				if componentAvailable == 0 || gap >= vmTotalMinusUsedGapTolerance {
					return availableFromUsed, "derived-total-minus-used"
				}
			}
		}
		return 0, ""
	}
}

func guestMemoryFallbackReason(source string) string {
	return MemorySourceFallbackReason(source)
}

func (m *Monitor) resolveGuestStatusMemory(
	ctx context.Context,
	client PVEClientInterface,
	instanceName string,
	guestName string,
	node string,
	vmid int,
	guestID string,
	status *proxmox.VMStatus,
	vmIDToHostAgent map[string]models.Host,
	memTotal uint64,
	memorySource string,
	guestRaw *VMMemoryRaw,
) (uint64, uint64, string) {
	if status == nil {
		return memTotal, 0, memorySource
	}

	if guestRaw != nil {
		guestRaw.StatusMaxMem = status.MaxMem
		guestRaw.StatusMem = status.Mem
		guestRaw.StatusFreeMem = status.FreeMem
		guestRaw.Balloon = status.Balloon
		guestRaw.BalloonMin = status.BalloonMin
		guestRaw.Agent = status.Agent.Value
	}

	memAvailable := uint64(0)
	if status.MemInfo != nil {
		memAvailable, memorySource = deriveGuestMemInfoAvailable(status.MemInfo, guestRaw)
		if memAvailable > 0 && memorySource == "derived-total-minus-used" {
			log.Debug().
				Str("vm", guestName).
				Str("node", node).
				Int("vmid", vmid).
				Uint64("total", memTotal).
				Uint64("available", memAvailable).
				Uint64("availableFromUsed", guestRaw.MemInfoTotalMinusUsed).
				Msg("QEMU memory: deriving guest available from total-used gap when cache fields are missing")
		}
	}

	if memAvailable == 0 {
		if rrdAvailable, rrdErr := m.getVMRRDMetrics(ctx, client, node, vmid); rrdErr == nil && rrdAvailable > 0 {
			memAvailable = rrdAvailable
			memorySource = "rrd-memavailable"
			if guestRaw != nil {
				guestRaw.MemInfoAvailable = memAvailable
			}
			log.Debug().
				Str("vm", guestName).
				Str("node", node).
				Int("vmid", vmid).
				Uint64("total", memTotal).
				Uint64("available", memAvailable).
				Msg("QEMU memory: using RRD memavailable fallback (excludes reclaimable cache)")
		} else if rrdErr != nil {
			log.Debug().
				Err(rrdErr).
				Str("instance", instanceName).
				Str("vm", guestName).
				Int("vmid", vmid).
				Msg("RRD memory data unavailable for VM, using status/cluster resources values")
		}
	}

	if memAvailable == 0 {
		if agentHost, ok := vmIDToHostAgent[guestID]; ok &&
			agentHost.Memory.Total > 0 &&
			agentHost.Memory.Used >= 0 &&
			agentHost.Memory.Total >= agentHost.Memory.Used {
			agentAvailable := uint64(agentHost.Memory.Total - agentHost.Memory.Used)
			if agentAvailable > 0 {
				memAvailable = agentAvailable
				memorySource = "agent"
				if guestRaw != nil {
					guestRaw.HostAgentTotal = uint64(agentHost.Memory.Total)
					guestRaw.HostAgentUsed = uint64(agentHost.Memory.Used)
				}
				log.Debug().
					Str("vm", guestName).
					Str("node", node).
					Int("vmid", vmid).
					Uint64("total", memTotal).
					Uint64("available", memAvailable).
					Int64("agentTotal", agentHost.Memory.Total).
					Int64("agentUsed", agentHost.Memory.Used).
					Msg("QEMU memory: using linked Pulse host agent memory (excludes page cache)")
			}
		}
	}

	if status.MaxMem > 0 {
		memTotal = status.MaxMem
	}

	memUsed := uint64(0)
	switch {
	case memAvailable > 0:
		if memAvailable > memTotal {
			memAvailable = memTotal
		}
		memUsed = memTotal - memAvailable
	case status.Mem > 0:
		memUsed = status.Mem
		memorySource = "status-mem"
	case status.FreeMem > 0 && memTotal >= status.FreeMem:
		memUsed = memTotal - status.FreeMem
		memorySource = "status-freemem"
	default:
		memorySource = "status-unavailable"
	}
	if memUsed > memTotal {
		memUsed = memTotal
	}

	return memTotal, memUsed, memorySource
}
