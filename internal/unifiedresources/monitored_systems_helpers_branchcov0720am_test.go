package unifiedresources

import (
	"strings"
	"testing"
	"time"
)

// Branch-coverage tests for two helpers in monitored_systems.go that are
// otherwise at 0% coverage:
//
//   - monitoredSystemHasReasonStatus: a slice-scanning predicate with
//     nil/empty/no-match/match arms.
//   - monitoredSystemStatusReportedAtSuffix: a formatter with a zero-time
//     arm (empty result) and a populated arm that emits a UTC RFC3339
//     timestamp.
//
// Assertions are behavioral: the predicate's classification outcome across
// its distinct arms, and the formatter's empty-vs-populated contract plus
// UTC normalization (the suffix for a given instant must be identical
// regardless of the input's timezone offset).

// --- monitoredSystemHasReasonStatus ---

func TestBranchcov0720am_HasReasonStatus(t *testing.T) {
	cases := []struct {
		name    string
		reasons []MonitoredSystemStatusReason
		status  string
		want    bool
	}{
		{
			name:    "nil slice returns false",
			reasons: nil,
			status:  "offline",
			want:    false,
		},
		{
			name:    "empty slice returns false",
			reasons: []MonitoredSystemStatusReason{},
			status:  "offline",
			want:    false,
		},
		{
			name: "no matching status returns false after full scan",
			reasons: []MonitoredSystemStatusReason{
				{Status: "online"},
				{Status: "warning"},
				{Status: "stale"},
			},
			status: "offline",
			want:   false,
		},
		{
			name: "single matching entry returns true",
			reasons: []MonitoredSystemStatusReason{
				{Status: "offline"},
			},
			status: "offline",
			want:   true,
		},
		{
			name: "match at first position returns true",
			reasons: []MonitoredSystemStatusReason{
				{Status: "offline"},
				{Status: "online"},
				{Status: "stale"},
			},
			status: "offline",
			want:   true,
		},
		{
			name: "match at middle position returns true",
			reasons: []MonitoredSystemStatusReason{
				{Status: "online"},
				{Status: "stale"},
				{Status: "warning"},
			},
			status: "stale",
			want:   true,
		},
		{
			name: "match at last position returns true",
			reasons: []MonitoredSystemStatusReason{
				{Status: "online"},
				{Status: "warning"},
				{Status: "offline"},
			},
			status: "offline",
			want:   true,
		},
		{
			name: "empty queried status matches entry with unset Status field",
			reasons: []MonitoredSystemStatusReason{
				{Status: ""},
				{Status: "online"},
			},
			status: "",
			want:   true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := monitoredSystemHasReasonStatus(tc.reasons, tc.status)
			if got != tc.want {
				t.Fatalf("monitoredSystemHasReasonStatus(_, %q) = %v, want %v", tc.status, got, tc.want)
			}
		})
	}
}

// The no-match path scans every element; assert it leaves the input slice
// untouched (the predicate must be side-effect free).
func TestBranchcov0720am_HasReasonStatus_NoMatchPathLeavesInputUntouched(t *testing.T) {
	reasons := []MonitoredSystemStatusReason{
		{Status: "online"},
		{Status: "warning"},
		{Status: "stale"},
	}
	snapshot := make([]MonitoredSystemStatusReason, len(reasons))
	copy(snapshot, reasons)

	if monitoredSystemHasReasonStatus(reasons, "offline") {
		t.Fatal("expected no-match for status 'offline'")
	}
	if len(reasons) != len(snapshot) {
		t.Fatalf("input length changed: got %d, want %d", len(reasons), len(snapshot))
	}
	for i := range reasons {
		if reasons[i] != snapshot[i] {
			t.Errorf("input[%d] mutated by no-match scan: got %+v, want %+v", i, reasons[i], snapshot[i])
		}
	}
}

// --- monitoredSystemStatusReportedAtSuffix ---

func TestBranchcov0720am_StatusReportedAtSuffix(t *testing.T) {
	// Two representations of the SAME instant, in different time zones.
	// The formatter is documented to emit UTC, so equivalent instants must
	// yield byte-identical suffixes regardless of input zone offset.
	utcTime := time.Date(2024, 3, 15, 10, 30, 0, 0, time.UTC)
	offsetZone := time.FixedZone("OFFSET", -5*3600)
	offsetEquivalent := utcTime.In(offsetZone)

	t.Run("zero time yields empty suffix", func(t *testing.T) {
		if got := monitoredSystemStatusReportedAtSuffix(time.Time{}); got != "" {
			t.Fatalf("zero-time suffix: got %q, want empty", got)
		}
	})

	t.Run("populated UTC time yields non-empty suffix embedding UTC RFC3339", func(t *testing.T) {
		got := monitoredSystemStatusReportedAtSuffix(utcTime)
		if got == "" {
			t.Fatal("populated-time suffix: got empty, want non-empty")
		}
		wantEmbedded := utcTime.UTC().Format(time.RFC3339)
		if !strings.Contains(got, wantEmbedded) {
			t.Errorf("suffix %q does not embed expected UTC RFC3339 form %q", got, wantEmbedded)
		}
	})

	t.Run("populated offset-zone time yields non-empty suffix embedding UTC RFC3339", func(t *testing.T) {
		got := monitoredSystemStatusReportedAtSuffix(offsetEquivalent)
		if got == "" {
			t.Fatal("populated-time suffix: got empty, want non-empty")
		}
		wantEmbedded := offsetEquivalent.UTC().Format(time.RFC3339)
		if !strings.Contains(got, wantEmbedded) {
			t.Errorf("suffix %q does not embed expected UTC RFC3339 form %q", got, wantEmbedded)
		}
		// Negative assertion: the formatter must NOT have used the input's
		// local-zone representation (which would carry a "-05:00" offset
		// rather than the "Z" UTC marker).
		localForm := offsetEquivalent.Format(time.RFC3339)
		if localForm != wantEmbedded && strings.Contains(got, localForm) {
			t.Errorf("suffix %q embeds local-zone form %q; UTC normalization broken", got, localForm)
		}
	})

	t.Run("equal instants in different zones produce identical suffixes", func(t *testing.T) {
		a := monitoredSystemStatusReportedAtSuffix(utcTime)
		b := monitoredSystemStatusReportedAtSuffix(offsetEquivalent)
		if a != b {
			t.Errorf("suffixes differ for equal instants: UTC=%q offset=%q", a, b)
		}
	})
}
