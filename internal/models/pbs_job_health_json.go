package models

import (
	"encoding/json"
	"time"
)

// MarshalJSON keeps an unobserved optional timestamp absent on the wire.
// time.Time's own JSON marshaler otherwise defeats omitempty and invents a
// year-0001 last run/next run in consumers of partial PBS job evidence.
func (f PBSJobHealthFreshness) MarshalJSON() ([]byte, error) {
	var lastRun, nextRun *time.Time
	if !f.LastRunEndTime.IsZero() {
		lastRun = &f.LastRunEndTime
	}
	if !f.NextRun.IsZero() {
		nextRun = &f.NextRun
	}
	return json.Marshal(struct {
		ObservedAt     time.Time  `json:"observedAt"`
		LastRunEndTime *time.Time `json:"lastRunEndTime,omitempty"`
		NextRun        *time.Time `json:"nextRun,omitempty"`
		AgeSeconds     int64      `json:"ageSeconds,omitempty"`
		State          string     `json:"state,omitempty"`
	}{f.ObservedAt, lastRun, nextRun, f.AgeSeconds, f.State})
}
