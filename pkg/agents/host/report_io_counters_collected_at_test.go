package host

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"
)

// IOCountersCollectedAt is optional on the wire: an older agent's report
// decodes without it, a report without it emits nothing new, and a stamped
// report keeps it apart from Timestamp through the JSON that both delivery
// and the agent's persisted report buffer use.
func TestReportIOCountersCollectedAtIsOptionalOnTheWire(t *testing.T) {
	var older Report
	if err := json.Unmarshal([]byte(`{"agent":{"id":"agent-a"},"host":{"hostname":"host-a"},"timestamp":"2026-10-30T12:00:50Z","sequenceId":"v1:stream-a:2"}`), &older); err != nil {
		t.Fatalf("decode older report: %v", err)
	}
	if older.IOCountersCollectedAt != nil {
		t.Fatalf("older report decoded a counter read stamp: %v", older.IOCountersCollectedAt)
	}

	unstamped, err := json.Marshal(Report{Timestamp: time.Date(2026, 10, 30, 12, 0, 50, 0, time.UTC)})
	if err != nil {
		t.Fatalf("encode unstamped report: %v", err)
	}
	if bytes.Contains(unstamped, []byte("ioCountersCollectedAt")) {
		t.Fatalf("unstamped report emitted the optional field: %s", unstamped)
	}

	readAt := time.Date(2026, 10, 30, 12, 0, 30, 123456789, time.UTC)
	stamped := Report{Timestamp: readAt.Add(20 * time.Second), IOCountersCollectedAt: &readAt}
	raw, err := json.Marshal(stamped)
	if err != nil {
		t.Fatalf("encode stamped report: %v", err)
	}
	var decoded Report
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("decode stamped report: %v", err)
	}
	if decoded.IOCountersCollectedAt == nil || !decoded.IOCountersCollectedAt.Equal(readAt) {
		t.Fatalf("counter read stamp = %v, want %v", decoded.IOCountersCollectedAt, readAt)
	}
	if !decoded.Timestamp.Equal(stamped.Timestamp) {
		t.Fatalf("Timestamp = %v, want the report's own %v", decoded.Timestamp, stamped.Timestamp)
	}
}
