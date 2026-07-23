package monitoring

import (
	"context"
	"math"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/rs/zerolog/log"
)

func (m *Monitor) resolveNodeMemory(
	ctx context.Context,
	client PVEClientInterface,
	instanceName string,
	nodeName string,
	memory *proxmox.MemoryStatus,
	raw NodeMemoryRaw,
) (models.Memory, string, string, NodeMemoryRaw, bool) {
	if memory == nil || memory.Total == 0 {
		return models.Memory{}, "", "", raw, false
	}

	raw.Total = memory.Total
	raw.Used = memory.Used
	raw.Free = memory.Free
	raw.Available = memory.Available
	raw.Avail = memory.Avail
	raw.Buffers = memory.Buffers
	raw.Cached = memory.Cached
	raw.Shared = memory.Shared
	raw.EffectiveAvailable = 0
	raw.ProxmoxMemorySource = "node-status"
	raw.FallbackCalculated = false

	componentAvailable := memory.Free
	if memory.Buffers > 0 {
		if math.MaxUint64-componentAvailable < memory.Buffers {
			componentAvailable = math.MaxUint64
		} else {
			componentAvailable += memory.Buffers
		}
	}
	if memory.Cached > 0 {
		if math.MaxUint64-componentAvailable < memory.Cached {
			componentAvailable = math.MaxUint64
		} else {
			componentAvailable += memory.Cached
		}
	}
	hasCacheComponents := memory.Buffers > 0 || memory.Cached > 0

	availableFromUsed := uint64(0)
	if memory.Total > 0 && memory.Used > 0 && memory.Total >= memory.Used {
		availableFromUsed = memory.Total - memory.Used
	}
	raw.TotalMinusUsed = availableFromUsed

	var rrdMetrics rrdMemCacheEntry
	haveRRDMetrics := false
	if memory.Available == 0 && memory.Avail == 0 && !hasCacheComponents {
		if metrics, err := m.getNodeRRDMetrics(ctx, client, instanceName, nodeName); err == nil {
			haveRRDMetrics = true
			rrdMetrics = metrics
		} else if err != nil {
			log.Debug().
				Err(err).
				Str("instance", instanceName).
				Str("node", nodeName).
				Msg("RRD memavailable fallback unavailable")
		}
	}

	const totalMinusUsedGapTolerance uint64 = 16 * 1024 * 1024
	var actualUsed uint64
	var effectiveAvailable uint64
	source := ""
	fallbackReason := ""

	switch {
	case memory.Available > 0 && memory.Available <= memory.Total:
		effectiveAvailable = memory.Available
		actualUsed = memory.Total - effectiveAvailable
		source = "available-field"
		log.Debug().
			Str("node", nodeName).
			Uint64("total", memory.Total).
			Uint64("available", effectiveAvailable).
			Msg("Node memory: using available field (excludes reclaimable cache)")

	case memory.Avail > 0 && memory.Avail <= memory.Total:
		effectiveAvailable = memory.Avail
		actualUsed = memory.Total - effectiveAvailable
		source = "available-field"
		log.Debug().
			Str("node", nodeName).
			Uint64("total", memory.Total).
			Uint64("available", effectiveAvailable).
			Msg("Node memory: using avail field (excludes reclaimable cache)")

	case hasCacheComponents && componentAvailable <= memory.Total:
		effectiveAvailable = componentAvailable
		actualUsed = memory.Total - effectiveAvailable
		source = "derived-free-buffers-cached"
		log.Debug().
			Str("node", nodeName).
			Uint64("total", memory.Total).
			Uint64("free", memory.Free).
			Uint64("buffers", memory.Buffers).
			Uint64("cached", memory.Cached).
			Uint64("effectiveAvailable", effectiveAvailable).
			Msg("Node memory: deriving availability from free+buffers+cached")

	case haveRRDMetrics && rrdMetrics.hasAvail && rrdMetrics.available <= memory.Total:
		effectiveAvailable = rrdMetrics.available
		actualUsed = memory.Total - effectiveAvailable
		source = "rrd-memavailable"
		fallbackReason = "rrd-memavailable"
		raw.FallbackCalculated = true
		raw.ProxmoxMemorySource = "rrd-memavailable"
		log.Debug().
			Str("node", nodeName).
			Uint64("total", memory.Total).
			Uint64("rrdAvailable", rrdMetrics.available).
			Msg("Node memory: using RRD memavailable fallback")

	case haveRRDMetrics && rrdMetrics.hasUsed && rrdMetrics.used <= memory.Total:
		actualUsed = rrdMetrics.used
		effectiveAvailable = memory.Total - actualUsed
		source = "rrd-memused"
		fallbackReason = "rrd-memused"
		raw.FallbackCalculated = true
		raw.ProxmoxMemorySource = "rrd-memused"
		log.Debug().
			Str("node", nodeName).
			Uint64("total", memory.Total).
			Uint64("rrdUsed", rrdMetrics.used).
			Msg("Node memory: using RRD memused fallback")

	case availableFromUsed > memory.Free &&
		availableFromUsed-memory.Free >= totalMinusUsedGapTolerance:
		effectiveAvailable = availableFromUsed
		actualUsed = memory.Used
		source = "derived-total-minus-used"
		fallbackReason = "node-status-total-minus-used"
		raw.FallbackCalculated = true
		raw.ProxmoxMemorySource = "node-status-total-minus-used"
		log.Debug().
			Str("node", nodeName).
			Uint64("total", memory.Total).
			Uint64("availableFromUsed", availableFromUsed).
			Uint64("reportedFree", memory.Free).
			Msg("Node memory: deriving availability from total-used cache gap")

	default:
		source = "unavailable"
		fallbackReason = "cache-aware-memory-unavailable"
		raw.FallbackCalculated = true
		raw.ProxmoxMemorySource = "cache-aware-unavailable"
		log.Warn().
			Str("instance", instanceName).
			Str("node", nodeName).
			Uint64("total", memory.Total).
			Uint64("used", memory.Used).
			Uint64("free", memory.Free).
			Msg("Node memory unavailable: no cache-aware source")
	}

	raw.EffectiveAvailable = effectiveAvailable
	if haveRRDMetrics {
		raw.RRDAvailable = rrdMetrics.available
		raw.RRDUsed = rrdMetrics.used
		raw.RRDTotal = rrdMetrics.total
	}

	if source == "unavailable" {
		return models.UnavailableMemory(clampToInt64(memory.Total)), source, fallbackReason, raw, true
	}

	free := int64(memory.Total - actualUsed)
	if free < 0 {
		free = 0
	}

	resolved := models.Memory{
		Total: int64(memory.Total),
		Used:  int64(actualUsed),
		Free:  free,
		Usage: safePercentage(float64(actualUsed), float64(memory.Total)),
	}
	// Free above is total-used, i.e. available. The node status reports its
	// truly-free pages directly, so split the reclaimable cache back out.
	splitReclaimableMemory(&resolved, memory.Free)

	return resolved, source, fallbackReason, raw, true
}
