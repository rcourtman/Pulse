package monitoring

import (
	"math"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

const (
	guestMemoryCarryForwardMaxAge        = 2 * time.Minute
	guestMemoryHealthyGuestMaxAge        = 10 * time.Minute
	guestMemoryCarryForwardMinUsageDelta = 5.0
	guestMemoryReliabilityLow            = 0
	guestMemoryReliabilityFallback       = 1
	guestMemoryReliabilityTrusted        = 2
)

func guestMemorySourceReliability(source string) int {
	switch CanonicalMemorySource(source) {
	case "available-field", "derived-free-buffers-cached",
		"guest-agent-meminfo", "rrd-memavailable", "rrd-memused", "agent":
		return guestMemoryReliabilityTrusted
	case "derived-total-minus-used", "previous-snapshot":
		return guestMemoryReliabilityFallback
	case "unknown", "cluster-resources", "status-mem", "status-freemem", "status-unavailable":
		return guestMemoryReliabilityLow
	default:
		return guestMemoryReliabilityFallback
	}
}

func (m *Monitor) previousGuestSnapshot(instance, guestType, node string, vmid int) *GuestMemorySnapshot {
	if m == nil {
		return nil
	}

	key := makeGuestSnapshotKey(instance, guestType, node, vmid)

	m.diagMu.RLock()
	snapshot, ok := m.guestSnapshots[key]
	m.diagMu.RUnlock()
	if !ok {
		return nil
	}

	normalized := normalizeGuestMemorySnapshot(snapshot)
	return &normalized
}

func guestAgentSignalsHealthy(
	detailedStatus *proxmox.VMStatus,
	diskFromAgent bool,
	ipAddresses []string,
	networkInterfaces []models.GuestNetworkInterface,
	osName, osVersion, agentVersion string,
) bool {
	if detailedStatus != nil && detailedStatus.Agent.Value > 0 {
		return true
	}
	return diskFromAgent ||
		len(ipAddresses) > 0 ||
		len(networkInterfaces) > 0 ||
		osName != "" ||
		osVersion != "" ||
		agentVersion != ""
}

func stabilizeGuestLowTrustMemory(
	prev *GuestMemorySnapshot,
	currentStatus string,
	currentSource string,
	currentTotal uint64,
	currentUsed uint64,
	now time.Time,
	guestAgentHealthy bool,
) (uint64, string, []string) {
	if shouldCarryForwardPreviousGuestMemory(prev, currentStatus, currentSource, currentTotal, currentUsed, now) {
		return uint64(prev.Memory.Used), "previous-snapshot", []string{"preserved-previous-memory-after-repeated-low-trust-pattern"}
	}
	if shouldCarryForwardHealthyGuestLowTrustMemory(prev, currentStatus, currentSource, currentTotal, currentUsed, now, guestAgentHealthy) {
		return uint64(prev.Memory.Used), "previous-snapshot", []string{"preserved-previous-memory-for-healthy-guest-low-trust-full-usage"}
	}
	return currentUsed, currentSource, nil
}

func shouldCarryForwardPreviousGuestMemory(prev *GuestMemorySnapshot, currentStatus, currentSource string, currentTotal, currentUsed uint64, now time.Time) bool {
	if prev == nil || currentStatus != "running" || prev.Status != "running" {
		return false
	}
	if prev.Memory.Total <= 0 || prev.Memory.Used < 0 {
		return false
	}
	if prev.RetrievedAt.IsZero() || now.Sub(prev.RetrievedAt) > guestMemoryCarryForwardMaxAge {
		return false
	}

	prevReliability := guestMemorySourceReliability(prev.MemorySource)
	currentReliability := guestMemorySourceReliability(currentSource)
	if prevReliability < guestMemoryReliabilityTrusted || currentReliability > guestMemoryReliabilityFallback {
		return false
	}
	if prevReliability <= currentReliability {
		return false
	}
	if currentTotal > 0 && prev.Memory.Total > 0 && prev.Memory.Total != int64(currentTotal) {
		return false
	}

	currentUsage := safePercentage(float64(currentUsed), float64(currentTotal))
	if prev.Memory.Usage > 0 && math.Abs(prev.Memory.Usage-currentUsage) < guestMemoryCarryForwardMinUsageDelta {
		return false
	}

	return true
}

func shouldCarryForwardHealthyGuestLowTrustMemory(prev *GuestMemorySnapshot, currentStatus, currentSource string, currentTotal, currentUsed uint64, now time.Time, guestAgentHealthy bool) bool {
	if prev == nil || !guestAgentHealthy || currentStatus != "running" || prev.Status != "running" {
		return false
	}
	if prev.Memory.Total <= 0 || prev.Memory.Used < 0 {
		return false
	}
	if prev.RetrievedAt.IsZero() || now.Sub(prev.RetrievedAt) > guestMemoryHealthyGuestMaxAge {
		return false
	}
	if currentTotal == 0 || prev.Memory.Total != int64(currentTotal) {
		return false
	}
	if guestMemorySourceReliability(currentSource) != guestMemoryReliabilityLow {
		return false
	}

	currentUsage := safePercentage(float64(currentUsed), float64(currentTotal))
	if currentUsage < 99 {
		return false
	}
	if prev.Memory.Usage >= 90 || math.Abs(prev.Memory.Usage-currentUsage) < guestMemoryCarryForwardMinUsageDelta {
		return false
	}

	prevReliability := guestMemorySourceReliability(prev.MemorySource)
	if CanonicalMemorySource(prev.MemorySource) != "previous-snapshot" && prevReliability < guestMemoryReliabilityTrusted {
		return false
	}

	return true
}
