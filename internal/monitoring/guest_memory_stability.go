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
		"guest-agent-meminfo", "guest-agent-meminfo-derived", "agent":
		return guestMemoryReliabilityTrusted
	case "derived-total-minus-used", "previous-snapshot":
		return guestMemoryReliabilityFallback
	case "unknown", "unavailable", "cluster-resources", "status-mem", "status-freemem", "status-unavailable":
		return guestMemoryReliabilityLow
	default:
		return guestMemoryReliabilityFallback
	}
}

// Configured capacity fences a VM resize; it is not the denominator of an
// in-guest reading. Older snapshots and containers keep their existing total.
func guestMemoryConfiguredCapacity(previous *GuestMemorySnapshot) uint64 {
	if previous == nil || previous.Memory.Total <= 0 {
		return 0
	}
	if previous.GuestType == "qemu" {
		if previous.Raw.StatusMaxMem > 0 {
			return previous.Raw.StatusMaxMem
		}
		if previous.Raw.ListingMaxMem > 0 {
			return previous.Raw.ListingMaxMem
		}
	}
	return uint64(previous.Memory.Total)
}

func guestMemoryCapacityMatches(previous *GuestMemorySnapshot, currentTotal uint64) bool {
	return previous != nil && previous.Memory.Total > 0 &&
		(uint64(previous.Memory.Total) == currentTotal || guestMemoryConfiguredCapacity(previous) == currentTotal)
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
	if detailedStatus != nil && detailedStatus.Agent.IsAvailable() {
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
	if !guestMemorySnapshotWithinAge(prev, now, guestMemoryCarryForwardMaxAge) {
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
	if currentTotal > 0 && !guestMemoryCapacityMatches(prev, currentTotal) {
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
	if !guestMemorySnapshotWithinAge(prev, now, guestMemoryHealthyGuestMaxAge) {
		return false
	}
	if currentTotal == 0 || !guestMemoryCapacityMatches(prev, currentTotal) {
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

// The diagnostic snapshot is refreshed on every poll, including when it only
// carries an old reading forward. It cannot extend the lifetime of that reading.
// Legacy direct readings can use their receipt time once; a legacy retained
// value has lost its original age and cannot borrow the latest poll's time.
func guestMemorySnapshotWithinAge(prev *GuestMemorySnapshot, now time.Time, maxAge time.Duration) bool {
	observation := prev.Memory.Observation
	observedAt := observation.ObservedAt
	if observation.State != "" || observation.Source != "" || !observedAt.IsZero() {
		if observation.State != "current" && observation.State != "last-known" {
			return false
		}
	} else {
		if CanonicalMemorySource(prev.MemorySource) == "previous-snapshot" {
			return false
		}
		observedAt = prev.RetrievedAt
	}
	return !observedAt.IsZero() && !observedAt.After(now) && now.Sub(observedAt) <= maxAge
}
