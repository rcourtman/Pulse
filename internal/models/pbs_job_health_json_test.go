package models

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestPBSJobHealthFreshnessJSONDoesNotInventRunTimes(t *testing.T) {
	observed := time.Date(2026, 10, 16, 20, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name             string
		lastRun, nextRun time.Time
		state            string
	}{
		{name: "partial", state: "unknown"},
		{name: "observed", lastRun: observed.Add(-time.Hour), state: "observed"},
		{name: "scheduled", nextRun: observed.Add(time.Hour), state: "scheduled"},
		{name: "observed-and-scheduled", lastRun: observed.Add(-time.Hour), nextRun: observed.Add(time.Hour), state: "observed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			freshness := PBSJobHealthFreshness{ObservedAt: observed, LastRunEndTime: tc.lastRun, NextRun: tc.nextRun, State: tc.state, AgeSeconds: 3600}
			wire, err := json.Marshal(PBSJobHealthEvidence{ID: "fixture", Freshness: freshness})
			if err != nil {
				t.Fatal(err)
			}
			var envelope struct {
				Freshness map[string]json.RawMessage `json:"freshness"`
			}
			if err := json.Unmarshal(wire, &envelope); err != nil {
				t.Fatal(err)
			}
			for key, known := range map[string]bool{"lastRunEndTime": !tc.lastRun.IsZero(), "nextRun": !tc.nextRun.IsZero()} {
				if _, present := envelope.Freshness[key]; present != known {
					t.Errorf("%s presence=%t want=%t: %s", key, present, known, wire)
				}
			}
			if strings.Contains(string(wire), "0001-") {
				t.Error("unobserved timestamp became a year-0001 observation")
			}
			var restored PBSJobHealthEvidence
			if err := json.Unmarshal(wire, &restored); err != nil {
				t.Fatal(err)
			}
			got := restored.Freshness
			if !got.ObservedAt.Equal(observed) || !got.LastRunEndTime.Equal(tc.lastRun) || !got.NextRun.Equal(tc.nextRun) || got.State != tc.state || got.AgeSeconds != 3600 {
				t.Error("known observation/state failed JSON round trip")
			}
		})
	}
}
