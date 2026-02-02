package ai

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/correlation"
)

func TestSeedFormattingHelpers(t *testing.T) {
	t.Run("formatBytes", func(t *testing.T) {
		cases := []struct {
			value int64
			want  string
		}{
			{value: 512, want: "512 B"},
			{value: 2 * 1024, want: "2 KB"},
			{value: 5 * 1024 * 1024, want: "5 MB"},
			{value: 3 * 1024 * 1024 * 1024, want: "3.0 GB"},
			{value: 2 * 1024 * 1024 * 1024 * 1024, want: "2.0 TB"},
		}
		for _, c := range cases {
			if got := seedFormatBytes(c.value); got != c.want {
				t.Fatalf("seedFormatBytes(%d) = %q, want %q", c.value, got, c.want)
			}
		}
	})

	t.Run("formatDuration", func(t *testing.T) {
		cases := []struct {
			value time.Duration
			want  string
		}{
			{value: 30 * time.Second, want: "30s"},
			{value: 5 * time.Minute, want: "5m"},
			{value: 3 * time.Hour, want: "3h"},
			{value: 48 * time.Hour, want: "2d"},
		}
		for _, c := range cases {
			if got := seedFormatDuration(c.value); got != c.want {
				t.Fatalf("seedFormatDuration(%s) = %q, want %q", c.value, got, c.want)
			}
		}
	})

	t.Run("formatTimeAgo", func(t *testing.T) {
		now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
		cases := []struct {
			value time.Time
			want  string
		}{
			{value: now.Add(-30 * time.Second), want: "just now"},
			{value: now.Add(-10 * time.Minute), want: "10m ago"},
			{value: now.Add(-2 * time.Hour), want: "2h ago"},
			{value: now.Add(-24 * time.Hour), want: "1d ago"},
			{value: now.Add(-72 * time.Hour), want: "3d ago"},
		}
		for _, c := range cases {
			if got := seedFormatTimeAgo(now, c.value); got != c.want {
				t.Fatalf("seedFormatTimeAgo(%s) = %q, want %q", c.value, got, c.want)
			}
		}
	})
}

func TestSeedIsInScope(t *testing.T) {
	if !seedIsInScope(nil, "anything") {
		t.Fatal("seedIsInScope should return true for nil scope")
	}

	scoped := map[string]bool{"res-1": true}
	if seedIsInScope(scoped, "res-2") {
		t.Fatal("seedIsInScope should return false for missing resource")
	}
	if !seedIsInScope(scoped, "res-1") {
		t.Fatal("seedIsInScope should return true for scoped resource")
	}
}

func TestPatrolService_BuildScopedSet_WithCorrelation(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	cfg := correlation.DefaultConfig()
	cfg.MinOccurrences = 1
	cfg.CorrelationWindow = 10 * time.Minute

	detector := correlation.NewDetector(cfg)
	now := time.Now()
	detector.RecordEvent(correlation.Event{
		ResourceID: "vm:node1:101",
		EventType:  correlation.EventType("alert"),
		Timestamp:  now.Add(-2 * time.Minute),
	})
	detector.RecordEvent(correlation.Event{
		ResourceID: "vm:node1:102",
		EventType:  correlation.EventType("alert"),
		Timestamp:  now.Add(-1 * time.Minute),
	})

	ps.correlationDetector = detector

	scope := &PatrolScope{ResourceIDs: []string{"vm:node1:101"}}
	scopedSet := ps.buildScopedSet(scope)
	if scopedSet == nil {
		t.Fatal("expected non-nil scoped set")
	}
	if !scopedSet["vm:node1:101"] || !scopedSet["vm:node1:102"] {
		t.Fatalf("expected correlated resources to be included, got: %+v", scopedSet)
	}
}

func TestPatrolService_SeedPreviousRun(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	ps.runHistoryStore = NewPatrolRunHistoryStore(10)

	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	ps.runHistoryStore.Add(PatrolRunRecord{
		ID:               "run-1",
		StartedAt:        now.Add(-2 * time.Hour),
		Duration:         30 * time.Minute,
		Status:           "issues_found",
		NewFindings:      1,
		ExistingFindings: 2,
		ResolvedFindings: 1,
		RejectedFindings: 0,
		FindingsSummary:  "CPU warning on node1",
	})

	summary := ps.seedPreviousRun(now)
	if summary == "" {
		t.Fatal("expected previous run summary to be non-empty")
	}
	expectedParts := []string{
		"# Previous Patrol Run",
		"Status: issues_found",
		"duration: 30m",
		"Summary: CPU warning on node1",
		"Trigger: scheduled",
	}
	for _, part := range expectedParts {
		if !strings.Contains(summary, part) {
			t.Fatalf("expected summary to contain %q, got:\n%s", part, summary)
		}
	}
}
