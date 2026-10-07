package ai

import "github.com/rcourtman/pulse-go-rewrite/internal/alerts"

// diskTemperatureLimits is the alert disk temperature policy for one disk
// type, the same policy disk temperature alerts and the Physical Disks Health
// verdict judge heat by. A disk is hot at the trigger and stays hot until it
// cools to the clear value. A zero trigger means disk temperature alerting is
// off, so no reading counts as heat.
type diskTemperatureLimits struct {
	trigger float64
	clear   float64
}

// diskTemperatureLimitsFor resolves the policy from the user's alert
// configuration, or from the factory alert configuration when Patrol has no
// threshold provider.
func diskTemperatureLimitsFor(provider ThresholdProvider, diskType string) diskTemperatureLimits {
	var trigger, clear float64
	if provider != nil {
		trigger, clear = provider.GetDiskTemperatureThreshold(diskType)
	} else if threshold := alerts.DefaultDiskTemperatureThreshold(diskType); threshold != nil {
		trigger, clear = threshold.Trigger, threshold.Clear
	}
	if trigger <= 0 {
		return diskTemperatureLimits{}
	}
	if clear <= 0 || clear > trigger {
		clear = trigger
	}
	return diskTemperatureLimits{trigger: trigger, clear: clear}
}

// hot reports a reading at or above the alert trigger.
func (l diskTemperatureLimits) hot(temperature int) bool {
	return l.trigger > 0 && temperature > 0 && float64(temperature) >= l.trigger
}

// cooled reports a reading at or below the clear value, where a disk
// temperature alert recovers. With no band below the trigger, a reading at
// the trigger still fires, so only a reading under it has cooled.
func (l diskTemperatureLimits) cooled(temperature int) bool {
	if l.trigger <= 0 || temperature <= 0 {
		return true
	}
	if l.clear >= l.trigger {
		return float64(temperature) < l.trigger
	}
	return float64(temperature) <= l.clear
}
