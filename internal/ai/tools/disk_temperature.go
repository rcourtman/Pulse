package tools

import (
	"fmt"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/pkg/diskinventory"
)

// DiskTemperature splits a physical disk temperature by its collection state
// for AI context. Collected is a reading the current observation took, and it
// is the only temperature that may be judged as heat. LastKnown is a value
// normalization kept without collecting it now (a disk in standby, a host
// agent past its reporting lease), with the collection state's reason in
// Reason. It is history, never evidence that the disk is hot now.
type DiskTemperature struct {
	Collected int
	LastKnown int
	Reason    string
}

// SplitDiskTemperature classifies a disk temperature with
// diskinventory.TemperatureCollected.
func SplitDiskTemperature(temperature int, collection *diskinventory.CollectionStatus) DiskTemperature {
	if diskinventory.TemperatureCollected(temperature, collection) {
		return DiskTemperature{Collected: temperature}
	}
	if temperature <= 0 {
		return DiskTemperature{}
	}
	split := DiskTemperature{LastKnown: temperature}
	if collection != nil {
		split.Reason = strings.TrimSpace(collection.Temperature.Reason)
	}
	return split
}

// Format renders the temperature with unit for AI text: a collected reading as
// the value ("41C"), a retained one as "last known 41C (disk is in standby)",
// and "" when the disk has no temperature.
func (t DiskTemperature) Format(unit string) string {
	switch {
	case t.Collected > 0:
		return fmt.Sprintf("%d%s", t.Collected, unit)
	case t.LastKnown > 0:
		text := fmt.Sprintf("last known %d%s", t.LastKnown, unit)
		if t.Reason != "" {
			text += " (" + t.Reason + ")"
		}
		return text
	default:
		return ""
	}
}
