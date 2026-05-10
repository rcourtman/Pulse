package ai

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/reporting"
)

func TestParseReportFleetResponse_StripsCodeFence(t *testing.T) {
	raw := "```json\n{\"health_status\":\"WARNING\",\"health_message\":\"x\",\"executive_summary\":\"y\",\"outliers\":[],\"patterns\":[],\"recommendations\":[]}\n```"
	got, err := parseReportFleetResponse(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.HealthStatus != "WARNING" {
		t.Errorf("HealthStatus = %q", got.HealthStatus)
	}
}

func TestParseReportFleetResponse_RejectsGarbage(t *testing.T) {
	if _, err := parseReportFleetResponse("not json"); err == nil {
		t.Fatal("expected error on non-JSON input")
	}
}

func TestBuildReportFleetPayload_PopulatesAggregateAndResources(t *testing.T) {
	now := time.Now().UTC()
	in := reporting.FleetNarrativeInput{
		Title:  "Fleet",
		Period: reporting.TimeRange{Start: now.Add(-time.Hour), End: now},
		Aggregate: reporting.FleetAggregate{
			ResourceCount:       2,
			TotalActiveAlerts:   3,
			TotalCriticalAlerts: 1,
			MaxCPUSeen:          95,
		},
		Resources: []reporting.FleetResourceSummary{
			{ResourceID: "a", ResourceName: "alpha", AvgMemory: 90, CriticalAlerts: 1},
			{ResourceID: "b", ResourceName: "beta", AvgCPU: 30},
		},
	}
	payload := buildReportFleetPayload(in)
	if payload.Aggregate.ResourceCount != 2 || payload.Aggregate.TotalCriticalAlerts != 1 {
		t.Errorf("Aggregate: %+v", payload.Aggregate)
	}
	if len(payload.Resources) != 2 {
		t.Fatalf("Resources length = %d", len(payload.Resources))
	}
	if payload.Resources[0].ResourceName != "alpha" || payload.Resources[0].AvgMemory != 90 {
		t.Errorf("Resources[0] = %+v", payload.Resources[0])
	}
	if !strings.HasPrefix(payload.Period.Start, now.Add(-time.Hour).UTC().Format("2006-01-02")) {
		t.Errorf("Period.Start = %q", payload.Period.Start)
	}
}

func TestNarrateFleet_RecordsCostEvent(t *testing.T) {
	tmp := t.TempDir()
	persistence := config.NewConfigPersistence(tmp)
	svc := NewService(persistence, nil)
	svc.cfg = &config.AIConfig{Enabled: true, Model: "anthropic:claude-test"}
	svc.provider = &mockProvider{
		chatFunc: func(_ context.Context, _ providers.ChatRequest) (*providers.ChatResponse, error) {
			return &providers.ChatResponse{
				Content: `{
                                  "health_status": "WARNING",
                                  "health_message": "Pressure",
                                  "executive_summary": "Memory creeping up.",
                                  "outliers": [{"resource_id":"a","resource_name":"alpha","reason":"Memory at 92%","severity":"warning"}],
                                  "patterns": [{"text":"3 of 8 resources show memory pressure","severity":"warning"}],
                                  "recommendations": ["Review memory across the fleet"],
                                  "period_comparison": ""
                                }`,
				Model:        "anthropic:claude-test",
				InputTokens:  500,
				OutputTokens: 200,
			}, nil
		},
	}

	in := reporting.FleetNarrativeInput{
		Title: "Weekly Fleet",
		Period: reporting.TimeRange{
			Start: time.Now().Add(-time.Hour),
			End:   time.Now(),
		},
		Aggregate: reporting.FleetAggregate{ResourceCount: 1},
		Resources: []reporting.FleetResourceSummary{
			{ResourceID: "a", ResourceName: "alpha", AvgMemory: 92},
		},
	}

	out, err := svc.NarrateFleet(context.Background(), in)
	if err != nil {
		t.Fatalf("NarrateFleet: %v", err)
	}
	if out.Source != reporting.NarrativeSourceAI {
		t.Errorf("Source = %q, want ai", out.Source)
	}
	if len(out.Outliers) != 1 || out.Outliers[0].ResourceName != "alpha" {
		t.Errorf("Outliers = %#v", out.Outliers)
	}

	events := svc.ListCostEvents(1)
	if len(events) != 1 {
		t.Fatalf("expected 1 cost event, got %d", len(events))
	}
	ev := events[0]
	if ev.UseCase != reportFleetNarratorUseCase {
		t.Errorf("UseCase = %q, want %q", ev.UseCase, reportFleetNarratorUseCase)
	}
	if ev.TargetType != "fleet" {
		t.Errorf("TargetType = %q, want fleet", ev.TargetType)
	}
	if ev.InputTokens != 500 || ev.OutputTokens != 200 {
		t.Errorf("tokens = (%d, %d)", ev.InputTokens, ev.OutputTokens)
	}
}

func TestNarrateFleet_FailsClosedOnEmptyContent(t *testing.T) {
	tmp := t.TempDir()
	persistence := config.NewConfigPersistence(tmp)
	svc := NewService(persistence, nil)
	svc.cfg = &config.AIConfig{Enabled: true, Model: "anthropic:claude-test"}
	svc.provider = &mockProvider{
		chatFunc: func(_ context.Context, _ providers.ChatRequest) (*providers.ChatResponse, error) {
			return &providers.ChatResponse{Content: "", Model: "anthropic:claude-test"}, nil
		},
	}
	if _, err := svc.NarrateFleet(context.Background(), reporting.FleetNarrativeInput{}); err == nil {
		t.Fatal("expected error on empty narrative")
	}
}
