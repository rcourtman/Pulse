package monitoring

import (
	"math"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

const (
	vmMemoryCarryForwardMaxAge        = 2 * time.Minute
	vmMemoryCarryForwardMinUsageDelta = 5.0
	vmMemorySourceReliabilityLow      = 0
	vmMemorySourceReliabilityFallback = 1
	vmMemorySourceReliabilityTrusted  = 2
)

func vmMemorySourceReliability(source string) int {
	switch strings.TrimSpace(source) {
	case "meminfo-available", "meminfo-derived", "guest-agent-meminfo",
		"rrd-memavailable", "rrd-memused", "host-agent", "meminfo-total-minus-used":
		return vmMemorySourceReliabilityTrusted
	case "previous-snapshot":
		return vmMemorySourceReliabilityFallback
	case "", "cluster-resources", "listing-mem", "status-mem", "status-freemem", "status-unavailable":
		return vmMemorySourceReliabilityLow
	default:
		return vmMemorySourceReliabilityFallback
	}
}

func shouldCarryForwardPreviousVMMemory(prev models.VM, currentStatus, currentSource string, currentTotal, currentUsed uint64, now time.Time) bool {
	if currentStatus != "running" || prev.Type != "qemu" || prev.Status != "running" {
		return false
	}
	if prev.Memory.Total <= 0 || prev.Memory.Used < 0 {
		return false
	}
	if prev.LastSeen.IsZero() || now.Sub(prev.LastSeen) > vmMemoryCarryForwardMaxAge {
		return false
	}

	prevReliability := vmMemorySourceReliability(prev.MemorySource)
	currentReliability := vmMemorySourceReliability(currentSource)
	if prevReliability < vmMemorySourceReliabilityTrusted || currentReliability > vmMemorySourceReliabilityFallback {
		return false
	}
	if prevReliability <= currentReliability {
		return false
	}

	if currentTotal > 0 && prev.Memory.Total > 0 && prev.Memory.Total != int64(currentTotal) {
		return false
	}

	currentUsage := safePercentage(float64(currentUsed), float64(currentTotal))
	if prev.Memory.Usage > 0 && math.Abs(prev.Memory.Usage-currentUsage) < vmMemoryCarryForwardMinUsageDelta {
		return false
	}

	return true
}
