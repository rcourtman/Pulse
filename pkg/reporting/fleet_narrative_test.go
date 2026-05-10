package reporting

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestHeuristicFleetNarrator_HealthyFleet(t *testing.T) {
	in := FleetNarrativeInput{
		Aggregate: FleetAggregate{ResourceCount: 3},
		Resources: []FleetResourceSummary{
			{ResourceID: "a", AvgCPU: 30, MaxCPU: 50, AvgMemory: 40},
			{ResourceID: "b", AvgCPU: 35, MaxCPU: 55, AvgMemory: 50},
			{ResourceID: "c", AvgCPU: 25, MaxCPU: 45, AvgMemory: 60},
		},
	}
	out, err := HeuristicFleetNarrator{}.NarrateFleet(context.Background(), in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.HealthStatus != "HEALTHY" {
		t.Errorf("HealthStatus = %q, want HEALTHY", out.HealthStatus)
	}
	if out.Source != NarrativeSourceHeuristic {
		t.Errorf("Source = %q", out.Source)
	}
	if len(out.Outliers) != 0 {
		t.Errorf("expected no outliers, got %d", len(out.Outliers))
	}
}

func TestHeuristicFleetNarrator_PicksCriticalAlertsAsOutliers(t *testing.T) {
	in := FleetNarrativeInput{
		Aggregate: FleetAggregate{
			ResourceCount:       3,
			TotalActiveAlerts:   2,
			TotalCriticalAlerts: 2,
		},
		Resources: []FleetResourceSummary{
			{ResourceID: "a", ResourceName: "alpha", CriticalAlerts: 1, ActiveAlerts: 1},
			{ResourceID: "b", ResourceName: "beta", CriticalAlerts: 1, ActiveAlerts: 1},
			{ResourceID: "c", ResourceName: "gamma"},
		},
	}
	out, _ := HeuristicFleetNarrator{}.NarrateFleet(context.Background(), in)
	if out.HealthStatus != "CRITICAL" {
		t.Errorf("HealthStatus = %q, want CRITICAL", out.HealthStatus)
	}
	if len(out.Outliers) < 2 {
		t.Fatalf("expected at least 2 outliers, got %d", len(out.Outliers))
	}
	names := []string{out.Outliers[0].ResourceName, out.Outliers[1].ResourceName}
	if !contains(names, "alpha") || !contains(names, "beta") {
		t.Errorf("expected alpha and beta outliers, got %v", names)
	}
	if !sliceContainsSubstring(out.Recommendations, "critical alerts across the fleet") {
		t.Errorf("expected fleet-scoped critical recommendation, got %v", out.Recommendations)
	}
}

func TestHeuristicFleetNarrator_PatternCountsAreFractional(t *testing.T) {
	in := FleetNarrativeInput{
		Aggregate: FleetAggregate{ResourceCount: 10},
		Resources: append(
			make([]FleetResourceSummary, 0, 10),
			func() []FleetResourceSummary {
				out := make([]FleetResourceSummary, 10)
				for i := range out {
					out[i] = FleetResourceSummary{ResourceID: "r", ResourceName: "r"}
				}
				// 6 of 10 hot on memory -> critical pattern
				for i := 0; i < 6; i++ {
					out[i].AvgMemory = 90
				}
				// 3 of 10 hot on CPU -> warning pattern
				for i := 0; i < 3; i++ {
					out[i].MaxCPU = 95
				}
				return out
			}()...,
		),
	}
	out, _ := HeuristicFleetNarrator{}.NarrateFleet(context.Background(), in)
	var memBullet, cpuBullet *NarrativeBullet
	for i := range out.Patterns {
		if strings.Contains(out.Patterns[i].Text, "memory") {
			memBullet = &out.Patterns[i]
		}
		if strings.Contains(out.Patterns[i].Text, "CPU") {
			cpuBullet = &out.Patterns[i]
		}
	}
	if memBullet == nil || memBullet.Severity != NarrativeSeverityCritical {
		t.Errorf("expected critical memory pattern, got %#v", memBullet)
	}
	if cpuBullet == nil || cpuBullet.Severity != NarrativeSeverityWarning {
		t.Errorf("expected warning cpu pattern, got %#v", cpuBullet)
	}
}

func TestNarrateFleet_FallsBackToHeuristicOnError(t *testing.T) {
	stub := &stubFleetNarrator{err: errors.New("boom")}
	out := narrateFleet(context.Background(), stub, FleetNarrativeInput{
		Aggregate: FleetAggregate{ResourceCount: 1},
		Resources: []FleetResourceSummary{{ResourceID: "x", AvgMemory: 95}},
	})
	if out.Source != NarrativeSourceHeuristic {
		t.Fatalf("Source = %q, want heuristic", out.Source)
	}
	if len(out.Outliers) == 0 {
		t.Fatal("expected heuristic outliers on AI failure")
	}
}

func TestNarrateFleet_UsesAINarrativeOnSuccess(t *testing.T) {
	stub := &stubFleetNarrator{out: FleetNarrative{
		HealthStatus:     "WARNING",
		HealthMessage:    "Pressure",
		ExecutiveSummary: "Memory creeping up across half the fleet.",
		Outliers: []FleetOutlier{
			{ResourceID: "a", ResourceName: "alpha", Reason: "Memory at 92%", Severity: NarrativeSeverityWarning},
		},
		Recommendations: []string{"Add RAM"},
	}}
	out := narrateFleet(context.Background(), stub, FleetNarrativeInput{})
	if out.Source != NarrativeSourceAI {
		t.Fatalf("Source = %q, want ai", out.Source)
	}
	if len(out.Outliers) != 1 || out.Outliers[0].ResourceName != "alpha" {
		t.Errorf("Outliers = %#v", out.Outliers)
	}
}

func TestBuildFleetNarrativeInput_AggregatesAlertsAndDisks(t *testing.T) {
	now := time.Now()
	resolved := now.Add(-30 * time.Minute)
	multi := &MultiReportData{
		Title: "Fleet",
		Start: now.Add(-time.Hour),
		End:   now,
		Resources: []*ReportData{
			{
				ResourceID:   "a",
				ResourceType: "node",
				Resource:     &ResourceInfo{Name: "alpha", Status: "online"},
				Summary: MetricSummary{ByMetric: map[string]MetricStats{
					"cpu":    {Avg: 50, Max: 92},
					"memory": {Avg: 88, Max: 90},
				}},
				Alerts: []AlertInfo{
					{Level: "critical"},
					{Level: "warning", ResolvedTime: &resolved},
				},
				Disks: []DiskInfo{
					{Device: "sda", Health: "FAILED"},
					{Device: "sdb", WearLevel: 20}, // wear level <= 30 counts unhealthy
				},
				Storage: []StorageInfo{{Name: "tank", UsagePerc: 95}},
			},
			{
				ResourceID:   "b",
				ResourceType: "node",
				Resource:     &ResourceInfo{Name: "beta", Status: "online"},
				Summary: MetricSummary{ByMetric: map[string]MetricStats{
					"cpu": {Avg: 30, Max: 50},
				}},
			},
		},
	}
	in := buildFleetNarrativeInput(multi)
	if in.Aggregate.ResourceCount != 2 {
		t.Errorf("ResourceCount = %d", in.Aggregate.ResourceCount)
	}
	if in.Aggregate.TotalCriticalAlerts != 1 {
		t.Errorf("TotalCriticalAlerts = %d", in.Aggregate.TotalCriticalAlerts)
	}
	if in.Aggregate.TotalActiveAlerts != 1 {
		t.Errorf("TotalActiveAlerts = %d", in.Aggregate.TotalActiveAlerts)
	}
	if in.Aggregate.TotalResolvedAlerts != 1 {
		t.Errorf("TotalResolvedAlerts = %d", in.Aggregate.TotalResolvedAlerts)
	}
	if len(in.Resources) != 2 {
		t.Fatalf("Resources = %d", len(in.Resources))
	}
	if in.Resources[0].UnhealthyDisks != 2 {
		t.Errorf("UnhealthyDisks = %d", in.Resources[0].UnhealthyDisks)
	}
	if in.Resources[0].StoragePoolsHigh != 1 {
		t.Errorf("StoragePoolsHigh = %d", in.Resources[0].StoragePoolsHigh)
	}
	if in.Aggregate.MaxCPUSeen != 92 {
		t.Errorf("MaxCPUSeen = %v", in.Aggregate.MaxCPUSeen)
	}
}

type stubFleetNarrator struct {
	out FleetNarrative
	err error
}

func (s *stubFleetNarrator) NarrateFleet(_ context.Context, _ FleetNarrativeInput) (FleetNarrative, error) {
	return s.out, s.err
}

func contains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}
