package monitoring

import (
	"math"

	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

const vmMemInfoGapTolerance uint64 = 16 * 1024 * 1024 // 16 MiB

type vmMemAvailableSelection struct {
	Available      uint64
	Source         string
	TotalMinusUsed uint64
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
