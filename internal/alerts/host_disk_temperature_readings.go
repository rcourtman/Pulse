package alerts

import "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"

// clearHostDiskTemperatureResources resolves the disk temperature alerts
// raised under the given resource IDs and drops their pending threshold runs.
// CheckHost calls it for disks its host still lists but the agent's
// --disk-exclude patterns took out of monitoring, so no reading will resolve
// them.
func (m *Manager) clearHostDiskTemperatureResources(resourceIDs map[string]struct{}) {
	if len(resourceIDs) == 0 {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	var storageKeys []string
	for storageKey, alert := range m.activeAlerts {
		if alert == nil {
			continue
		}
		if _, ok := resourceIDs[alert.ResourceID]; ok {
			storageKeys = append(storageKeys, storageKey)
		}
	}
	for resourceID := range resourceIDs {
		m.core.DropPendingForResource(resourceID)
	}
	for _, storageKey := range storageKeys {
		m.clearAlertNoLock(storageKey)
	}
}

// judgedHostDiskTemperature is a disk temperature reading with the threshold
// its disk type resolves to under the host's policy.
type judgedHostDiskTemperature struct {
	reading   unifiedresources.HostDiskTemperatureReading
	threshold *HysteresisThreshold
}

// standing ranks the reading against its own threshold: 2 at or above the
// trigger, 1 inside the recovery band, where an open alert holds, 0 below it,
// and -1 when the threshold is off. margin is how far the reading sits above
// the trigger.
func (j judgedHostDiskTemperature) standing() (rank int, margin float64) {
	threshold := j.threshold
	if threshold == nil || threshold.Trigger <= 0 {
		return -1, 0
	}
	temperature := float64(j.reading.Temperature)
	margin = temperature - threshold.Trigger
	switch {
	case temperature >= threshold.Trigger:
		return 2, margin
	case threshold.Clear > 0 && threshold.Clear < threshold.Trigger && temperature > threshold.Clear:
		return 1, margin
	default:
		return 0, margin
	}
}

// outranks reports whether j should be judged instead of other for their
// shared alert. A tie keeps the reading listed first.
func (j judgedHostDiskTemperature) outranks(other judgedHostDiskTemperature) bool {
	rank, margin := j.standing()
	otherRank, otherMargin := other.standing()
	if rank != otherRank {
		return rank > otherRank
	}
	return margin > otherMargin
}
