package api

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func availabilityTransition(at time.Time, from, to string) unifiedresources.ResourceChange {
	return unifiedresources.ResourceChange{
		Kind:       unifiedresources.ChangeStateTransition,
		ObservedAt: at,
		From:       from,
		To:         to,
	}
}

func TestComputeReportAvailability_NoTransitionsUsesCurrentState(t *testing.T) {
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)

	info := computeReportAvailability(nil, "online", start, end)
	if info == nil {
		t.Fatal("expected availability info")
	}
	if info.UptimePercent != 100 || info.ObservedPercent != 100 {
		t.Fatalf("uptime=%v observed=%v, want 100/100", info.UptimePercent, info.ObservedPercent)
	}
	if info.DownIncidents != 0 || info.TotalDowntime != 0 {
		t.Fatalf("expected no downtime, got %+v", info)
	}
}

func TestComputeReportAvailability_MidWindowOutage(t *testing.T) {
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(100 * time.Hour)
	changes := []unifiedresources.ResourceChange{
		// Store returns newest first; the computation must sort.
		availabilityTransition(start.Add(60*time.Hour), "offline", "online"),
		availabilityTransition(start.Add(50*time.Hour), "online", "offline"),
	}

	info := computeReportAvailability(changes, "online", start, end)
	if info == nil {
		t.Fatal("expected availability info")
	}
	if info.UptimePercent != 90 {
		t.Fatalf("uptime=%v, want 90 (10h down of 100h)", info.UptimePercent)
	}
	if info.ObservedPercent != 100 {
		t.Fatalf("observed=%v, want 100", info.ObservedPercent)
	}
	if info.DownIncidents != 1 {
		t.Fatalf("incidents=%d, want 1", info.DownIncidents)
	}
	if info.TotalDowntime != 10*time.Hour || info.LongestOutage != 10*time.Hour {
		t.Fatalf("downtime=%v longest=%v, want 10h/10h", info.TotalDowntime, info.LongestOutage)
	}
}

// A monitoring gap (resource absent from the registry, e.g. the monitor
// itself restarted) is unobserved time: excluded from the uptime math and
// surfaced via ObservedPercent, never counted as an outage.
func TestComputeReportAvailability_AbsenceIsUnobservedNotDowntime(t *testing.T) {
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(100 * time.Hour)
	changes := []unifiedresources.ResourceChange{
		availabilityTransition(start.Add(20*time.Hour), "online", "absent"),
		availabilityTransition(start.Add(40*time.Hour), "absent", "online"),
	}

	info := computeReportAvailability(changes, "online", start, end)
	if info == nil {
		t.Fatal("expected availability info")
	}
	if info.UptimePercent != 100 {
		t.Fatalf("uptime=%v, want 100 (gap is not downtime)", info.UptimePercent)
	}
	if info.ObservedPercent != 80 {
		t.Fatalf("observed=%v, want 80 (20h gap of 100h)", info.ObservedPercent)
	}
	if info.DownIncidents != 0 {
		t.Fatalf("incidents=%d, want 0", info.DownIncidents)
	}
}

func TestComputeReportAvailability_TwoOutagesTracksLongest(t *testing.T) {
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(100 * time.Hour)
	changes := []unifiedresources.ResourceChange{
		availabilityTransition(start.Add(10*time.Hour), "online", "offline"),
		availabilityTransition(start.Add(12*time.Hour), "offline", "online"),
		availabilityTransition(start.Add(50*time.Hour), "online", "offline"),
		availabilityTransition(start.Add(56*time.Hour), "offline", "online"),
	}

	info := computeReportAvailability(changes, "online", start, end)
	if info.DownIncidents != 2 {
		t.Fatalf("incidents=%d, want 2", info.DownIncidents)
	}
	if info.TotalDowntime != 8*time.Hour {
		t.Fatalf("downtime=%v, want 8h", info.TotalDowntime)
	}
	if info.LongestOutage != 6*time.Hour {
		t.Fatalf("longest=%v, want 6h", info.LongestOutage)
	}
	if info.UptimePercent != 92 {
		t.Fatalf("uptime=%v, want 92", info.UptimePercent)
	}
}

func TestComputeReportAvailability_OfflineThroughWindowEnd(t *testing.T) {
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(10 * time.Hour)
	changes := []unifiedresources.ResourceChange{
		availabilityTransition(start.Add(8*time.Hour), "online", "offline"),
	}

	info := computeReportAvailability(changes, "offline", start, end)
	if info.UptimePercent != 80 {
		t.Fatalf("uptime=%v, want 80", info.UptimePercent)
	}
	if info.TotalDowntime != 2*time.Hour || info.LongestOutage != 2*time.Hour {
		t.Fatalf("downtime=%v longest=%v, want 2h/2h (outage still open at window end)", info.TotalDowntime, info.LongestOutage)
	}
	if info.DownIncidents != 1 {
		t.Fatalf("incidents=%d, want 1", info.DownIncidents)
	}
}

func TestComputeReportAvailability_NeverObserved(t *testing.T) {
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(10 * time.Hour)

	info := computeReportAvailability(nil, "unknown", start, end)
	if info == nil {
		t.Fatal("expected availability info")
	}
	if info.Observed() {
		t.Fatalf("expected unobserved window, got %+v", info)
	}
	if info.UptimePercent != 0 {
		t.Fatalf("uptime=%v, want 0 when never observed", info.UptimePercent)
	}
}

// Warning is up: the resource is reachable and serving. A stability report
// that counted warnings as outages would invent downtime.
func TestComputeReportAvailability_WarningCountsAsUp(t *testing.T) {
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(10 * time.Hour)
	changes := []unifiedresources.ResourceChange{
		availabilityTransition(start.Add(2*time.Hour), "online", "warning"),
		availabilityTransition(start.Add(4*time.Hour), "warning", "online"),
	}

	info := computeReportAvailability(changes, "online", start, end)
	if info.UptimePercent != 100 || info.DownIncidents != 0 {
		t.Fatalf("uptime=%v incidents=%d, want 100/0", info.UptimePercent, info.DownIncidents)
	}
}

func TestComputeReportAvailability_IgnoresNonTransitionChanges(t *testing.T) {
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(10 * time.Hour)
	changes := []unifiedresources.ResourceChange{
		{Kind: unifiedresources.ChangeAlertFired, ObservedAt: start.Add(time.Hour), From: "online", To: "offline"},
	}

	info := computeReportAvailability(changes, "online", start, end)
	if info.UptimePercent != 100 || info.DownIncidents != 0 {
		t.Fatalf("non-transition changes must not affect availability, got %+v", info)
	}
}
