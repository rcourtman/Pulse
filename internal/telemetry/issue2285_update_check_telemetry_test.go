package telemetry

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/updates"
)

func TestIssue2285UpdateCheckFieldsAreClosedAndContentFree(t *testing.T) {
	for _, tc := range []struct {
		name          string
		check         updates.UpdateCheckObservation
		wantChannel   string
		wantOutcome   string
		wantAvailable bool
	}{
		{"offered on stable", updates.UpdateCheckObservation{Channel: "stable", Outcome: updates.UpdateCheckOutcomeAvailable, Available: true}, "stable", "available", true},
		{"preview up to date", updates.UpdateCheckObservation{Channel: "RC", Outcome: " up_to_date ", Available: false}, "rc", "up_to_date", false},
		{"broken check never claims an offer", updates.UpdateCheckObservation{Channel: "stable", Outcome: updates.UpdateCheckOutcomeMetadataError, Available: true}, "stable", "metadata_error", false},
		{"never checked", updates.UpdateCheckObservation{}, "unknown", "not_checked", false},
		{"unknown values collapse", updates.UpdateCheckObservation{Channel: "nightly", Outcome: "failed to fetch https://example.invalid/releases"}, "unknown", "error", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var snap Snapshot
			ApplyUpdateCheckTelemetrySnapshot(&snap, tc.check)
			ping := BuildPingForSnapshot(snap)
			if ping.UpdateChannel != tc.wantChannel || ping.UpdateCheckOutcome != tc.wantOutcome || ping.UpdateAvailable != tc.wantAvailable {
				t.Fatalf("ping update discovery = (%q, %q, %v), want (%q, %q, %v)",
					ping.UpdateChannel, ping.UpdateCheckOutcome, ping.UpdateAvailable,
					tc.wantChannel, tc.wantOutcome, tc.wantAvailable)
			}
		})
	}

	// A snapshot that skipped the update wiring must still send closed values.
	ping := BuildPingForSnapshot(Snapshot{UpdateAvailable: true, UpdateCheckOutcome: "", UpdateChannel: ""})
	if ping.UpdateChannel != "unknown" || ping.UpdateCheckOutcome != "not_checked" || ping.UpdateAvailable {
		t.Fatalf("unwired snapshot ping = (%q, %q, %v), want (unknown, not_checked, false)",
			ping.UpdateChannel, ping.UpdateCheckOutcome, ping.UpdateAvailable)
	}
}

func TestIssue2285UpdateCheckFieldsSerializeWithStableNames(t *testing.T) {
	var snap Snapshot
	ApplyUpdateCheckTelemetrySnapshot(&snap, updates.UpdateCheckObservation{
		Channel:   "rc",
		Outcome:   updates.UpdateCheckOutcomeAvailable,
		Available: true,
		CheckedAt: time.Now(),
	})
	raw, err := json.Marshal(BuildPingForSnapshot(snap))
	if err != nil {
		t.Fatalf("marshal ping: %v", err)
	}
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatalf("unmarshal ping: %v", err)
	}
	if fields["update_channel"] != "rc" || fields["update_check_outcome"] != "available" || fields["update_available"] != true {
		t.Fatalf("serialized update discovery = (%v, %v, %v)", fields["update_channel"], fields["update_check_outcome"], fields["update_available"])
	}
}
