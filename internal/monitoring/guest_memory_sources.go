package monitoring

import (
	"context"
	"math"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/rs/zerolog/log"
)

const guestStatusMemoryMismatchTolerance uint64 = 128 * 1024 * 1024 // 128 MiB

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
	componentsConflict := memInfo.Total > 0 && componentAvailable > memInfo.Total

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
	case memInfo.Available > 0 && (memInfo.Total == 0 || memInfo.Available <= memInfo.Total):
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
		if missingCacheMetrics {
			return 0, ""
		}
		if componentsConflict {
			return 0, ""
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

func saturatingAddUint64(lhs, rhs uint64) uint64 {
	if math.MaxUint64-lhs < rhs {
		return math.MaxUint64
	}
	return lhs + rhs
}

func guestStatusFreeMem(status *proxmox.VMStatus) uint64 {
	if status == nil {
		return 0
	}
	if status.FreeMem > 0 {
		return status.FreeMem
	}
	if status.BalloonInfo != nil {
		return status.BalloonInfo.FreeMem
	}
	return 0
}

func effectiveGuestFreeMemTotal(memTotal uint64, status *proxmox.VMStatus) uint64 {
	if status == nil {
		return memTotal
	}

	freeMem := guestStatusFreeMem(status)
	if status.BalloonInfo != nil {
		if status.BalloonInfo.TotalMem > 0 && freeMem <= status.BalloonInfo.TotalMem {
			if memTotal > 0 && status.BalloonInfo.TotalMem > memTotal {
				return memTotal
			}
			return status.BalloonInfo.TotalMem
		}
		if status.BalloonInfo.Actual > 0 && freeMem <= status.BalloonInfo.Actual {
			if memTotal > 0 && status.BalloonInfo.Actual > memTotal {
				return memTotal
			}
			return status.BalloonInfo.Actual
		}
	}

	if status.Balloon > 0 && status.Balloon <= memTotal && freeMem <= status.Balloon {
		return status.Balloon
	}
	return memTotal
}

func selectGuestLowTrustUsedMemory(memTotal uint64, status *proxmox.VMStatus) (uint64, string) {
	if status == nil {
		return 0, ""
	}

	freeMemTotal := effectiveGuestFreeMemTotal(memTotal, status)
	freeMem := guestStatusFreeMem(status)
	hasFreeFallback := freeMem > 0 && freeMemTotal >= freeMem
	freeDerivedUsed := uint64(0)
	if hasFreeFallback {
		freeDerivedUsed = freeMemTotal - freeMem
	}

	if status.Mem > 0 {
		if hasFreeFallback && freeDerivedUsed < status.Mem {
			statusMemPlusFree := saturatingAddUint64(status.Mem, freeMem)
			if status.Mem >= freeMemTotal && freeDerivedUsed < freeMemTotal {
				return freeDerivedUsed, "status-freemem"
			}
			if statusMemPlusFree > freeMemTotal+guestStatusMemoryMismatchTolerance {
				return freeDerivedUsed, "status-freemem"
			}
		}
		return status.Mem, "status-mem"
	}

	if hasFreeFallback {
		return freeDerivedUsed, "status-freemem"
	}

	return 0, ""
}

func shouldPreferGuestAgentMemAvailable(status *proxmox.VMStatus, memTotal uint64) bool {
	if status == nil || !status.Agent.IsAvailable() || status.Mem == 0 {
		return false
	}
	if guestStatusFreeMem(status) > 0 {
		return false
	}

	effectiveTotal := memTotal
	if status.MaxMem > 0 {
		effectiveTotal = status.MaxMem
	}
	if effectiveTotal == 0 {
		return false
	}
	if status.Mem >= effectiveTotal {
		return true
	}

	return effectiveTotal-status.Mem <= guestStatusMemoryMismatchTolerance
}

func guestMemoryFallbackReason(source string) string {
	return MemorySourceFallbackReason(source)
}

func (m *Monitor) tryGuestAgentMemAvailable(
	ctx context.Context,
	client PVEClientInterface,
	instanceName string,
	guestName string,
	node string,
	vmid int,
	memTotal uint64,
	guestRaw *VMMemoryRaw,
) (uint64, string, bool) {
	availability, agentErr := m.getVMAgentMemoryAvailability(ctx, client, instanceName, node, vmid)
	if agentErr != nil || availability.Source == "" {
		return 0, "", false
	}
	agentAvailable := availability.EffectiveAvailable
	if guestRaw != nil {
		guestRaw.GuestAgentMemAvailable = agentAvailable
		guestRaw.GuestAgentMemFree = availability.Free
		guestRaw.GuestAgentMemBuffers = availability.Buffers
		guestRaw.GuestAgentMemCached = availability.Cached
		guestRaw.GuestAgentSReclaimable = availability.SReclaimable
		guestRaw.GuestAgentShmem = availability.Shmem
		guestRaw.GuestAgentDerived = availability.Source == "meminfo-derived"
	}
	if memTotal == 0 || agentAvailable > memTotal {
		return 0, "", false
	}
	source := "guest-agent-meminfo"
	if availability.Source == "meminfo-derived" {
		source = "guest-agent-meminfo-derived"
	}
	log.Debug().
		Str("vm", guestName).
		Str("node", node).
		Int("vmid", vmid).
		Uint64("total", memTotal).
		Uint64("available", agentAvailable).
		Str("source", source).
		Msg("QEMU memory: using guest agent /proc/meminfo fallback (excludes reclaimable cache)")
	return agentAvailable, source, true
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
		guestRaw.StatusFreeMem = guestStatusFreeMem(status)
		guestRaw.Balloon = status.Balloon
		guestRaw.BalloonMin = status.BalloonMin
		guestRaw.Agent = status.Agent.Value
		if status.BalloonInfo != nil {
			guestRaw.BalloonInfoFreeMem = status.BalloonInfo.FreeMem
			guestRaw.BalloonInfoTotalMem = status.BalloonInfo.TotalMem
			guestRaw.BalloonInfoActual = status.BalloonInfo.Actual
		}
	}
	if status.MaxMem > 0 {
		memTotal = status.MaxMem
	}

	memAvailable := uint64(0)
	hasMemAvailable := false
	derivedTotalMinusUsedAvailable := uint64(0)
	selectedUsed := uint64(0)
	hasSelectedUsed := false
	if status.MemInfo != nil {
		memAvailable, memorySource = deriveGuestMemInfoAvailable(status.MemInfo, guestRaw)
		hasMemAvailable = memorySource != ""
		if memAvailable > memTotal {
			memAvailable = 0
			hasMemAvailable = false
			memorySource = ""
		}
		if memorySource == "derived-total-minus-used" {
			derivedTotalMinusUsedAvailable = memAvailable
			memAvailable = 0
			hasMemAvailable = false
			memorySource = ""
		}
	}

	triedGuestAgentMemAvailable := false
	if !hasMemAvailable && shouldPreferGuestAgentMemAvailable(status, memTotal) {
		triedGuestAgentMemAvailable = true
		if agentAvailable, agentSource, ok := m.tryGuestAgentMemAvailable(ctx, client, instanceName, guestName, node, vmid, memTotal, guestRaw); ok {
			memAvailable = agentAvailable
			hasMemAvailable = true
			memorySource = agentSource
		}
	}

	if !hasMemAvailable {
		if rrdMemory, rrdErr := m.getVMRRDMemory(ctx, client, instanceName, node, vmid); rrdErr == nil {
			if guestRaw != nil {
				guestRaw.RRDMemAvailable = rrdMemory.available
				guestRaw.RRDMemUsed = rrdMemory.used
				guestRaw.RRDMaxMem = rrdMemory.total
			}
			switch {
			case rrdMemory.hasAvail && rrdMemory.available <= memTotal:
				memAvailable = rrdMemory.available
				hasMemAvailable = true
				memorySource = "rrd-memavailable"
				log.Debug().
					Str("vm", guestName).
					Str("node", node).
					Int("vmid", vmid).
					Uint64("total", memTotal).
					Uint64("available", memAvailable).
					Msg("QEMU memory: using RRD memavailable fallback")
			case rrdMemory.hasUsed && rrdMemory.used <= memTotal:
				selectedUsed = rrdMemory.used
				hasSelectedUsed = true
				memorySource = "rrd-memused"
				log.Debug().
					Str("vm", guestName).
					Str("node", node).
					Int("vmid", vmid).
					Uint64("total", memTotal).
					Uint64("used", rrdMemory.used).
					Msg("QEMU memory: using RRD memused fallback")
			}
		} else if rrdErr != nil {
			log.Debug().
				Err(rrdErr).
				Str("instance", instanceName).
				Str("vm", guestName).
				Int("vmid", vmid).
				Msg("RRD memory data unavailable for VM, using status/cluster resources values")
		}
	}

	if !hasMemAvailable && !hasSelectedUsed && status.Agent.IsAvailable() && !triedGuestAgentMemAvailable {
		if agentAvailable, agentSource, ok := m.tryGuestAgentMemAvailable(ctx, client, instanceName, guestName, node, vmid, memTotal, guestRaw); ok {
			memAvailable = agentAvailable
			hasMemAvailable = true
			memorySource = agentSource
		}
	}

	if !hasMemAvailable && !hasSelectedUsed && derivedTotalMinusUsedAvailable > 0 {
		memAvailable = derivedTotalMinusUsedAvailable
		hasMemAvailable = true
		memorySource = "derived-total-minus-used"
		log.Debug().
			Str("vm", guestName).
			Str("node", node).
			Int("vmid", vmid).
			Uint64("total", memTotal).
			Uint64("available", memAvailable).
			Uint64("availableFromUsed", derivedTotalMinusUsedAvailable).
			Msg("QEMU memory: deriving guest available from total-used gap after preferred fallbacks")
	}

	if !hasMemAvailable && !hasSelectedUsed {
		if agentHost, ok := vmIDToHostAgent[guestID]; ok &&
			agentHost.Memory.HasKnownUsage() &&
			agentHost.Memory.Used <= int64(memTotal) &&
			agentHost.Memory.Total >= agentHost.Memory.Used {
			agentAvailable := uint64(agentHost.Memory.Total - agentHost.Memory.Used)
			memAvailable = agentAvailable
			hasMemAvailable = true
			selectedUsed = uint64(agentHost.Memory.Used)
			hasSelectedUsed = true
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

	memUsed := uint64(0)
	switch {
	case hasSelectedUsed:
		memUsed = selectedUsed
	case hasMemAvailable:
		memUsed = memTotal - memAvailable
	default:
		if status.MemInfo != nil {
			memorySource = "unavailable"
		} else {
			memUsed, memorySource = selectGuestLowTrustUsedMemory(memTotal, status)
			if memorySource == "" {
				memorySource = "unavailable"
			}
		}
	}
	if memUsed > memTotal {
		memUsed = memTotal
	}

	return memTotal, memUsed, memorySource
}
