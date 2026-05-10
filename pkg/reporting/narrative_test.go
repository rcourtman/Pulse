package reporting

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestHeuristicNarrator_HealthStatus(t *testing.T) {
	cases := []struct {
		name           string
		alerts         []AlertInfo
		expectedStatus string
	}{
		{"clean", nil, "HEALTHY"},
		{"warning_only", []AlertInfo{{Level: "warning"}}, "WARNING"},
		{"critical_dominates", []AlertInfo{{Level: "warning"}, {Level: "critical"}}, "CRITICAL"},
		{"resolved_does_not_count", []AlertInfo{{Level: "critical", ResolvedTime: ptrTimeNow()}}, "HEALTHY"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := HeuristicNarrator{}.Narrate(context.Background(), NarrativeInput{Alerts: tc.alerts})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if out.HealthStatus != tc.expectedStatus {
				t.Fatalf("HealthStatus = %q, want %q", out.HealthStatus, tc.expectedStatus)
			}
			if out.Source != NarrativeSourceHeuristic {
				t.Fatalf("Source = %q, want %q", out.Source, NarrativeSourceHeuristic)
			}
		})
	}
}

func TestHeuristicNarrator_RecommendationsCoverage(t *testing.T) {
	in := NarrativeInput{
		MetricStats: map[string]MetricStats{
			"cpu":    {Avg: 40, Max: 95},
			"memory": {Avg: 90, Max: 95},
			"disk":   {Avg: 90, Max: 92},
		},
		Storage: []StorageInfo{{Name: "local", UsagePerc: 95}},
		Disks:   []DiskInfo{{Device: "sda", WearLevel: 5, Health: "FAILED"}},
		Alerts:  []AlertInfo{{Level: "critical"}},
		Resource: &ResourceInfo{
			Uptime: 100 * 86400,
		},
	}
	out, _ := HeuristicNarrator{}.Narrate(context.Background(), in)
	wants := []string{
		"Replace disk sda",
		"SMART health check failed",
		"critical alerts",
		"adding memory",
		"CPU-intensive",
		"Clean up disk space",
		"Expand storage pool 'local'",
		"Schedule maintenance window",
	}
	for _, want := range wants {
		if !sliceContainsSubstring(out.Recommendations, want) {
			t.Errorf("Recommendations missing %q\nGot: %#v", want, out.Recommendations)
		}
	}
}

type stubNarrator struct {
	out  Narrative
	err  error
	seen NarrativeInput
}

func (s *stubNarrator) Narrate(_ context.Context, in NarrativeInput) (Narrative, error) {
	s.seen = in
	return s.out, s.err
}

func TestNarrate_FallsBackToHeuristicOnError(t *testing.T) {
	stub := &stubNarrator{err: errors.New("provider failed")}
	out := narrate(context.Background(), stub, NarrativeInput{
		MetricStats: map[string]MetricStats{"cpu": {Avg: 50, Max: 60}},
	})
	if out.Source != NarrativeSourceHeuristic {
		t.Fatalf("Source = %q, want heuristic", out.Source)
	}
	if len(out.Observations) == 0 {
		t.Fatal("expected heuristic observations when AI errors")
	}
}

func TestNarrate_UsesAINarrativeOnSuccess(t *testing.T) {
	stub := &stubNarrator{out: Narrative{
		HealthStatus:     "HEALTHY",
		HealthMessage:    "All clear",
		ExecutiveSummary: "Quiet week.",
		Observations:     []NarrativeBullet{{Text: "From AI", Severity: NarrativeSeverityOK}},
		Recommendations:  []string{"Keep monitoring"},
		PeriodComparison: "No change vs prior week.",
	}}
	out := narrate(context.Background(), stub, NarrativeInput{})
	if out.Source != NarrativeSourceAI {
		t.Fatalf("Source = %q, want ai", out.Source)
	}
	if out.ExecutiveSummary != "Quiet week." {
		t.Fatalf("ExecutiveSummary = %q", out.ExecutiveSummary)
	}
	if len(out.Observations) != 1 || out.Observations[0].Text != "From AI" {
		t.Fatalf("Observations = %#v", out.Observations)
	}
}

func TestNarrate_NilNarratorUsesHeuristic(t *testing.T) {
	out := narrate(context.Background(), nil, NarrativeInput{
		MetricStats: map[string]MetricStats{"cpu": {Avg: 95, Max: 99}},
	})
	if out.Source != NarrativeSourceHeuristic {
		t.Fatalf("Source = %q, want heuristic", out.Source)
	}
}

func ptrTimeNow() *time.Time {
	now := time.Now()
	return &now
}

func sliceContainsSubstring(haystack []string, needle string) bool {
	for _, h := range haystack {
		if strings.Contains(h, needle) {
			return true
		}
	}
	return false
}
