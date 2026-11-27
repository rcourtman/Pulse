package telemetry

import (
	"encoding/json"
	"strings"
	"testing"
)

// Schema v25 sends counts-only adoption totals for configured DNS and inbound
// push heartbeat availability checks. No tokens, schedules, hostnames, or
// answers leave the install.
func TestPingCarriesAvailabilityDNSAndPushCounts(t *testing.T) {
	if TelemetrySchemaVersion < 25 {
		t.Fatalf("TelemetrySchemaVersion = %d, want at least 25", TelemetrySchemaVersion)
	}
	ping := applySnapshot(Ping{InstallID: "test-id"}, func() Snapshot {
		return Snapshot{
			AvailabilityTargets:     10,
			AvailabilityDNSTargets:  2,
			AvailabilityPushTargets: 3,
		}
	})
	encoded, err := json.Marshal(ping)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{
		`"availability_dns_targets":2`,
		`"availability_push_targets":3`,
	} {
		if !strings.Contains(string(encoded), key) {
			t.Fatalf("ping JSON lacks %s: %s", key, encoded)
		}
	}
}

func TestPingReportsZeroAvailabilityDNSAndPushCountsWhenUnused(t *testing.T) {
	encoded, err := json.Marshal(applySnapshot(Ping{InstallID: "test-id"}, func() Snapshot { return Snapshot{} }))
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{
		`"availability_dns_targets":0`,
		`"availability_push_targets":0`,
	} {
		if !strings.Contains(string(encoded), key) {
			t.Fatalf("ping JSON lacks %s: %s", key, encoded)
		}
	}
}
