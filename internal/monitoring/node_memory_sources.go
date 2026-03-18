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
	raw.EffectiveAvailable = memory.EffectiveAvailable()
	raw.ProxmoxMemorySource = "node-status"
	raw.FallbackCalculated = false

	effectiveAvailable := memory.EffectiveAvailable()
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
	if memory.Total > 0 && componentAvailable > memory.Total {
		componentAvailable = memory.Total
	}

	availableFromUsed := uint64(0)
	if memory.Total > 0 && memory.Used > 0 && memory.Total >= memory.Used {
		availableFromUsed = memory.Total - memory.Used
	}
	raw.TotalMinusUsed = availableFromUsed

	missingCacheMetrics := memory.Available == 0 &&
		memory.Avail == 0 &&
		memory.Buffers == 0 &&
		memory.Cached == 0

	var rrdMetrics rrdMemCacheEntry
	haveRRDMetrics := false
	usedRRDAvailableFallback := false
	rrdMemUsedFallback := false

	if effectiveAvailable == 0 && missingCacheMetrics {
		if metrics, err := m.getNodeRRDMetrics(ctx, client, nodeName); err == nil {
			haveRRDMetrics = true
			rrdMetrics = metrics
			if metrics.available > 0 {
				effectiveAvailable = metrics.available
				usedRRDAvailableFallback = true
			}
			if metrics.used > 0 {
				rrdMemUsedFallback = true
			}
		} else if err != nil {
			log.Debug().
				Err(err).
				Str("instance", instanceName).
				Str("node", nodeName).
				Msg("RRD memavailable fallback unavailable")
		}
	}

	const totalMinusUsedGapTolerance uint64 = 16 * 1024 * 1024
	gapGreaterThanComponents := false
	if availableFromUsed > componentAvailable {
		gap := availableFromUsed - componentAvailable
		if componentAvailable == 0 || gap >= totalMinusUsedGapTolerance {
			gapGreaterThanComponents = true
		}
	}

	derivedFromTotalMinusUsed := !usedRRDAvailableFallback &&
		missingCacheMetrics &&
		availableFromUsed > 0 &&
		gapGreaterThanComponents &&
		effectiveAvailable == availableFromUsed

	var actualUsed uint64
	source := ""
	fallbackReason := ""

	switch {
	case effectiveAvailable > 0 && effectiveAvailable <= memory.Total:
		actualUsed = memory.Total - effectiveAvailable
		if actualUsed > memory.Total {
			actualUsed = memory.Total
		}

		logCtx := log.Debug().
			Str("node", nodeName).
			Uint64("total", memory.Total).
			Uint64("effectiveAvailable", effectiveAvailable).
			Uint64("actualUsed", actualUsed).
			Float64("usage", safePercentage(float64(actualUsed), float64(memory.Total)))

		if usedRRDAvailableFallback {
			if haveRRDMetrics && rrdMetrics.available > 0 {
				logCtx = logCtx.Uint64("rrdAvailable", rrdMetrics.available)
			}
			logCtx.Msg("node memory: using RRD memavailable fallback (excludes reclaimable cache)")
			source = "rrd-memavailable"
			fallbackReason = "rrd-memavailable"
			raw.FallbackCalculated = true
			raw.ProxmoxMemorySource = "rrd-memavailable"
		} else if memory.Available > 0 {
			logCtx.Msg("node memory: using available field (excludes reclaimable cache)")
			source = "available-field"
		} else if memory.Avail > 0 {
			logCtx.Msg("node memory: using avail field (excludes reclaimable cache)")
			source = "available-field"
		} else if derivedFromTotalMinusUsed {
			logCtx.
				Uint64("availableFromUsed", availableFromUsed).
				Uint64("reportedFree", memory.Free).
				Msg("Node memory: derived available from total-used gap (cache fields missing)")
			source = "derived-total-minus-used"
			fallbackReason = "node-status-total-minus-used"
			raw.FallbackCalculated = true
			raw.ProxmoxMemorySource = "node-status-total-minus-used"
		} else {
			logCtx.
				Uint64("free", memory.Free).
				Uint64("buffers", memory.Buffers).
				Uint64("cached", memory.Cached).
				Msg("Node memory: derived available from free+buffers+cached (excludes reclaimable cache)")
			source = "derived-free-buffers-cached"
		}
	default:
		switch {
		case rrdMemUsedFallback && haveRRDMetrics && rrdMetrics.used > 0:
			actualUsed = rrdMetrics.used
			if actualUsed > memory.Total {
				actualUsed = memory.Total
			}
			log.Debug().
				Str("node", nodeName).
				Uint64("total", memory.Total).
				Uint64("rrdUsed", rrdMetrics.used).
				Msg("Node memory: using RRD memused fallback (excludes reclaimable cache)")
			source = "rrd-memused"
			fallbackReason = "rrd-memused"
			raw.FallbackCalculated = true
			raw.ProxmoxMemorySource = "rrd-memused"
		default:
			actualUsed = memory.Used
			if actualUsed > memory.Total {
				actualUsed = memory.Total
			}
			log.Debug().
				Str("node", nodeName).
				Uint64("total", memory.Total).
				Uint64("used", actualUsed).
				Msg("Node memory: no cache-aware metrics - using traditional calculation (includes cache)")
			source = "node-status-used"
		}
	}

	raw.EffectiveAvailable = effectiveAvailable
	if haveRRDMetrics {
		raw.RRDAvailable = rrdMetrics.available
		raw.RRDUsed = rrdMetrics.used
		raw.RRDTotal = rrdMetrics.total
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

	return resolved, source, fallbackReason, raw, true
}
