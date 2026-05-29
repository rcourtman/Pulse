package monitoring

import (
	"context"
	"math"

	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/rs/zerolog/log"
)

const vmMemInfoGapTolerance uint64 = 16 * 1024 * 1024            // 16 MiB
const vmStatusMemoryMismatchTolerance uint64 = 128 * 1024 * 1024 // 128 MiB

type vmMemAvailableSelection struct {
	Available      uint64
	Source         string
	TotalMinusUsed uint64
}

type vmLowTrustUsedSelection struct {
	Used   uint64
	Source string
}

func effectiveVMLowTrustFreeMemTotal(memTotal uint64, status *proxmox.VMStatus) uint64 {
	if status == nil {
		return memTotal
	}
	if status.Balloon > 0 && status.Balloon <= memTotal && status.FreeMem <= status.Balloon {
		return status.Balloon
	}
	return memTotal
}

// selectVMAvailableFromMemInfo evaluates VM meminfo data from the QEMU guest
// agent and returns:
// - a primary cache-aware memAvailable value when meminfo is trustworthy
// - total-minus-used as a secondary fallback candidate
//
// We intentionally defer to downstream fallbacks (RRD/guest-agent file-read)
// when meminfo appears incomplete (e.g. tiny buffers/cached with a large gap
// to total-used), because those paths are more reliable for affected guests.
func selectVMAvailableFromMemInfo(memInfo *proxmox.VMMemInfo) vmMemAvailableSelection {
	if memInfo == nil {
		return vmMemAvailableSelection{}
	}

	selection := vmMemAvailableSelection{}
	if memInfo.Total > 0 && memInfo.Used > 0 && memInfo.Total >= memInfo.Used {
		selection.TotalMinusUsed = memInfo.Total - memInfo.Used
	}

	if memInfo.Available > 0 {
		selection.Available = memInfo.Available
		if memInfo.Total > 0 && selection.Available > memInfo.Total {
			selection.Available = memInfo.Total
		}
		selection.Source = "meminfo-available"
		return selection
	}

	// If the balloon doesn't report cache breakdown, avoid using Free alone.
	if memInfo.Buffers == 0 && memInfo.Cached == 0 {
		return selection
	}

	componentAvailable := saturatingAddUint64(memInfo.Free, memInfo.Buffers)
	componentAvailable = saturatingAddUint64(componentAvailable, memInfo.Cached)
	if memInfo.Total > 0 && componentAvailable > memInfo.Total {
		componentAvailable = memInfo.Total
	}
	if componentAvailable == 0 {
		return selection
	}

	// If total-used is materially larger than free+buffers+cached, the cache
	// breakdown appears incomplete. Defer to downstream fallbacks before using
	// total-used.
	if selection.TotalMinusUsed > componentAvailable {
		gap := selection.TotalMinusUsed - componentAvailable
		if gap >= vmMemInfoGapTolerance {
			return selection
		}
	}

	selection.Available = componentAvailable
	selection.Source = "meminfo-derived"
	return selection
}

func saturatingAddUint64(lhs, rhs uint64) uint64 {
	if math.MaxUint64-lhs < rhs {
		return math.MaxUint64
	}
	return lhs + rhs
}

// selectVMLowTrustUsedMemory chooses the least-wrong memory-used fallback when
// we have no cache-aware availability figure. Prefer status.mem in the general
// case because it is already relative to the guest-visible balloon size.
//
// However, when status.mem reports a fully saturated VM but status.freemem
// still shows meaningful headroom, treat the free-based calculation as the
// safer fallback. This avoids locking Windows guests onto false 100% samples
// when Proxmox emits a bogus full-used reading alongside a sane free value.
func selectVMLowTrustUsedMemory(memTotal uint64, status *proxmox.VMStatus) vmLowTrustUsedSelection {
	if status == nil {
		return vmLowTrustUsedSelection{}
	}

	freeMemTotal := effectiveVMLowTrustFreeMemTotal(memTotal, status)
	hasFreeFallback := status.FreeMem > 0 && freeMemTotal >= status.FreeMem
	freeDerivedUsed := uint64(0)
	if hasFreeFallback {
		freeDerivedUsed = freeMemTotal - status.FreeMem
	}

	if status.Mem > 0 {
		if hasFreeFallback && freeDerivedUsed < status.Mem {
			statusMemPlusFree := saturatingAddUint64(status.Mem, status.FreeMem)
			if status.Mem >= freeMemTotal && freeDerivedUsed < freeMemTotal {
				return vmLowTrustUsedSelection{Used: freeDerivedUsed, Source: "status-freemem"}
			}
			if statusMemPlusFree > freeMemTotal+vmStatusMemoryMismatchTolerance {
				return vmLowTrustUsedSelection{Used: freeDerivedUsed, Source: "status-freemem"}
			}
		}
		return vmLowTrustUsedSelection{Used: status.Mem, Source: "status-mem"}
	}

	if hasFreeFallback {
		return vmLowTrustUsedSelection{Used: freeDerivedUsed, Source: "status-freemem"}
	}

	return vmLowTrustUsedSelection{}
}

func (m *Monitor) tryVMAgentMemAvailable(
	ctx context.Context,
	client PVEClientInterface,
	instanceName string,
	node string,
	vmName string,
	vmid int,
	memTotal uint64,
	guestRaw *VMMemoryRaw,
) (uint64, bool) {
	agentAvailable, agentErr := m.getVMAgentMemAvailable(ctx, client, instanceName, node, vmid)
	if agentErr != nil || agentAvailable == 0 {
		return 0, false
	}

	if guestRaw != nil {
		guestRaw.GuestAgentMemAvailable = agentAvailable
		guestRaw.MemInfoAvailable = agentAvailable
	}

	log.Debug().
		Str("vm", vmName).
		Str("node", node).
		Int("vmid", vmid).
		Uint64("total", memTotal).
		Uint64("available", agentAvailable).
		Msg("QEMU memory: using guest agent /proc/meminfo (excludes reclaimable cache)")

	return agentAvailable, true
}
