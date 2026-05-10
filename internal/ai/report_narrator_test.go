package ai

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/reporting"
)

func TestParseReportNarratorResponse_Plain(t *testing.T) {
	raw := `{
  "health_status": "WARNING",
  "health_message": "Memory pressure",
  "executive_summary": "Memory ran hot all week.",
  "observations": [{"text": "Mem averaged 92%", "severity": "warning"}],
  "recommendations": ["Add RAM"],
  "period_comparison": "Up from 65% last week."
}`
	got, err := parseReportNarratorResponse(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.HealthStatus != "WARNING" {
		t.Errorf("HealthStatus = %q", got.HealthStatus)
	}
	if len(got.Observations) != 1 || got.Observations[0].Text != "Mem averaged 92%" {
		t.Errorf("Observations = %#v", got.Observations)
	}
	if got.PeriodComparison != "Up from 65% last week." {
		t.Errorf("PeriodComparison = %q", got.PeriodComparison)
	}
}

func TestParseReportNarratorResponse_StripsCodeFence(t *testing.T) {
	raw := "```json\n{\"health_status\":\"HEALTHY\",\"health_message\":\"ok\",\"observations\":[],\"recommendations\":[]}\n```"
	got, err := parseReportNarratorResponse(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.HealthStatus != "HEALTHY" {
		t.Errorf("HealthStatus = %q", got.HealthStatus)
	}
}

func TestParseReportNarratorResponse_StripsBareFence(t *testing.T) {
	raw := "```\n{\"health_status\":\"CRITICAL\",\"health_message\":\"x\",\"observations\":[],\"recommendations\":[]}\n```"
	got, err := parseReportNarratorResponse(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.HealthStatus != "CRITICAL" {
		t.Errorf("HealthStatus = %q", got.HealthStatus)
	}
}

func TestParseReportNarratorResponse_RejectsGarbage(t *testing.T) {
	if _, err := parseReportNarratorResponse("not json at all"); err == nil {
		t.Fatal("expected error on non-JSON input")
	}
}

func TestNormalizeBulletSeverity(t *testing.T) {
	cases := map[string]string{
		"critical": reporting.NarrativeSeverityCritical,
		"HIGH":     reporting.NarrativeSeverityCritical,
		"danger":   reporting.NarrativeSeverityCritical,
		"warning":  reporting.NarrativeSeverityWarning,
		"medium":   reporting.NarrativeSeverityWarning,
		"info":     reporting.NarrativeSeverityInfo,
		"ok":       reporting.NarrativeSeverityOK,
		"good":     reporting.NarrativeSeverityOK,
		"healthy":  reporting.NarrativeSeverityOK,
		"":         reporting.NarrativeSeverityInfo, // unknown defaults to info
		"banana":   reporting.NarrativeSeverityInfo,
	}
	for input, want := range cases {
		if got := normalizeBulletSeverity(input); got != want {
			t.Errorf("normalizeBulletSeverity(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestNormalizeReportHealthStatus(t *testing.T) {
	cases := map[string]string{
		"HEALTHY":  "HEALTHY",
		"healthy":  "HEALTHY",
		"WARNING":  "WARNING",
		"Critical": "CRITICAL",
		"":         "",
		"unknown":  "",
	}
	for input, want := range cases {
		if got := normalizeReportHealthStatus(input); got != want {
			t.Errorf("normalizeReportHealthStatus(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestBuildReportNarratorPayload_PopulatesPriorAndFindings(t *testing.T) {
	now := time.Now().UTC()
	in := reporting.NarrativeInput{
		Title:        "Node Report",
		ResourceType: "node",
		ResourceID:   "pve1",
		Period:       reporting.TimeRange{Start: now.Add(-time.Hour), End: now},
		PriorPeriod: &reporting.PriorPeriodInput{
			Period: reporting.TimeRange{Start: now.Add(-2 * time.Hour), End: now.Add(-time.Hour)},
			MetricStats: map[string]reporting.MetricStats{
				"cpu": {Avg: 50, Max: 60},
			},
		},
		Resource: &reporting.ResourceInfo{
			Name:        "pve1",
			DisplayName: "PVE 1",
			Status:      "online",
			Uptime:      3 * 86400,
			MemoryTotal: 16 * 1024 * 1024 * 1024,
			CPUCores:    8,
		},
		MetricStats: map[string]reporting.MetricStats{
			"cpu": {Avg: 70, Max: 95, Min: 30, Current: 80, Count: 12},
		},
		Alerts: []reporting.AlertInfo{
			{Type: "cpu", Level: "critical", Message: "spike"},
		},
		Findings: []reporting.FindingSummary{
			{Severity: "high", Title: "Patrol thing", Resolved: false},
		},
	}
	payload := buildReportNarratorPayload(in)
	if payload.Title != "Node Report" || payload.ResourceID != "pve1" {
		t.Fatalf("payload basics: %+v", payload)
	}
	if payload.Resource == nil || payload.Resource.UptimeDays != 3 {
		t.Errorf("Resource uptime: %#v", payload.Resource)
	}
	if payload.Resource.MemoryGB != 16 {
		t.Errorf("Resource memory: %v", payload.Resource.MemoryGB)
	}
	if payload.PriorPeriod == nil || payload.PriorPeriod.MetricStats["cpu"].Avg != 50 {
		t.Errorf("PriorPeriod: %#v", payload.PriorPeriod)
	}
	if len(payload.Alerts) != 1 || payload.Alerts[0].Resolved {
		t.Errorf("Alerts: %#v", payload.Alerts)
	}
	if len(payload.Findings) != 1 || payload.Findings[0].Title != "Patrol thing" {
		t.Errorf("Findings: %#v", payload.Findings)
	}
	if payload.MetricStats["cpu"].Max != 95 {
		t.Errorf("MetricStats: %#v", payload.MetricStats)
	}
}

func TestBuildReportNarratorPayload_OmitsEmptySections(t *testing.T) {
	now := time.Now().UTC()
	payload := buildReportNarratorPayload(reporting.NarrativeInput{
		Period: reporting.TimeRange{Start: now.Add(-time.Hour), End: now},
	})
	if payload.Resource != nil {
		t.Errorf("Resource should be nil: %#v", payload.Resource)
	}
	if payload.PriorPeriod != nil {
		t.Errorf("PriorPeriod should be nil: %#v", payload.PriorPeriod)
	}
	if len(payload.Alerts) != 0 || len(payload.Findings) != 0 || len(payload.Disks) != 0 || len(payload.Storage) != 0 {
		t.Errorf("expected empty collections, got %#v", payload)
	}
}

func TestBuildReportNarratorPayload_PeriodFormatting(t *testing.T) {
	start := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	payload := buildReportNarratorPayload(reporting.NarrativeInput{
		Period: reporting.TimeRange{Start: start, End: end},
	})
	if !strings.HasPrefix(payload.Period.Start, "2026-01-15") {
		t.Errorf("Period.Start = %q", payload.Period.Start)
	}
	if payload.Period.Hours != 24 {
		t.Errorf("Period.Hours = %d, want 24", payload.Period.Hours)
	}
}
