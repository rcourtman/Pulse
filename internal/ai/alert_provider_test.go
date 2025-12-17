package ai

import (
	"strings"
	"testing"
	"time"
)

type stubAlertProvider struct {
	active   []AlertInfo
	resolved []ResolvedAlertInfo
}

func (s *stubAlertProvider) GetActiveAlerts() []AlertInfo                        { return s.active }
func (s *stubAlertProvider) GetRecentlyResolved(minutes int) []ResolvedAlertInfo { return s.resolved }
func (s *stubAlertProvider) GetAlertsByResource(resourceID string) []AlertInfo {
	out := make([]AlertInfo, 0)
	for _, a := range s.active {
		if a.ResourceID == resourceID {
			out = append(out, a)
		}
	}
	return out
}
func (s *stubAlertProvider) GetAlertHistory(resourceID string, limit int) []ResolvedAlertInfo {
	out := make([]ResolvedAlertInfo, 0)
	for _, a := range s.resolved {
		if a.ResourceID == resourceID {
			out = append(out, a)
			if len(out) >= limit {
				break
			}
		}
	}
	return out
}

func TestService_buildAlertContext(t *testing.T) {
	now := time.Now()

	s := &Service{}
	s.SetAlertProvider(&stubAlertProvider{
		active: []AlertInfo{
			{
				ID:           "a1",
				Type:         "cpu",
				Level:        "critical",
				ResourceID:   "node:pve1",
				ResourceName: "pve1",
				ResourceType: "node",
				Node:         "pve1",
				Message:      "cpu high",
				Value:        95,
				Threshold:    80,
				StartTime:    now.Add(-5 * time.Minute),
				Duration:     "5 mins",
				Acknowledged: true,
			},
			{
				ID:           "a2",
				Type:         "memory",
				Level:        "warning",
				ResourceID:   "guest:100",
				ResourceName: "vm-100",
				ResourceType: "guest",
				Message:      "mem high",
				Value:        80,
				Threshold:    75,
				StartTime:    now.Add(-2 * time.Minute),
				Duration:     "2 mins",
			},
		},
		resolved: []ResolvedAlertInfo{
			{
				AlertInfo: AlertInfo{
					ID:           "r1",
					Type:         "disk",
					Level:        "warning",
					ResourceID:   "storage:local",
					ResourceName: "local",
					Message:      "disk ok",
					Duration:     "10 mins",
				},
				ResolvedTime: now.Add(-2 * time.Minute),
				Duration:     "10 mins",
			},
		},
	})

	ctx := s.buildAlertContext()
	if !strings.Contains(ctx, "## Alert Status") {
		t.Fatalf("expected alert status header, got: %s", ctx)
	}
	if !strings.Contains(ctx, "### Active Alerts") || !strings.Contains(ctx, "**Critical:**") || !strings.Contains(ctx, "**Warning:**") {
		t.Fatalf("expected active alert sections, got: %s", ctx)
	}
	if !strings.Contains(ctx, "[ACKNOWLEDGED]") || !strings.Contains(ctx, "on node pve1") {
		t.Fatalf("expected acknowledged/node formatting, got: %s", ctx)
	}
	if !strings.Contains(ctx, "### Recently Resolved") {
		t.Fatalf("expected recently resolved section, got: %s", ctx)
	}
}

func TestService_buildAlertContext_Empty(t *testing.T) {
	s := &Service{}
	if got := s.buildAlertContext(); got != "" {
		t.Fatalf("expected empty string, got: %q", got)
	}
	s.SetAlertProvider(&stubAlertProvider{})
	if got := s.buildAlertContext(); got != "" {
		t.Fatalf("expected empty string when no alerts, got: %q", got)
	}
}

func TestService_buildTargetAlertContext(t *testing.T) {
	s := &Service{}
	s.SetAlertProvider(&stubAlertProvider{
		active: []AlertInfo{
			{ID: "a1", Level: "critical", Type: "cpu", ResourceID: "node:pve1", ResourceName: "pve1", Duration: "1 min", Value: 90, Threshold: 80},
			{ID: "a2", Level: "warning", Type: "memory", ResourceID: "node:pve2", ResourceName: "pve2", Duration: "1 min", Value: 80, Threshold: 70},
		},
	})

	got := s.buildTargetAlertContext("node:pve1")
	if !strings.Contains(got, "Active Alerts for This Resource") || !strings.Contains(got, "pve1") {
		t.Fatalf("unexpected context: %s", got)
	}
	if strings.Contains(got, "pve2") {
		t.Fatalf("unexpected extra resource: %s", got)
	}
}

func TestFormatTimeAgo(t *testing.T) {
	now := time.Now()
	if got := formatTimeAgo(now.Add(-10 * time.Second)); got != "just now" {
		t.Fatalf("formatTimeAgo(<1m) = %q", got)
	}
	if got := formatTimeAgo(now.Add(-2 * time.Minute)); got != "2 minutes" {
		t.Fatalf("formatTimeAgo(2m) = %q", got)
	}
	if got := formatTimeAgo(now.Add(-2 * time.Hour)); got != "2 hours" {
		t.Fatalf("formatTimeAgo(2h) = %q", got)
	}
	if got := formatTimeAgo(now.Add(-48 * time.Hour)); got != "2 days" {
		t.Fatalf("formatTimeAgo(2d) = %q", got)
	}
}

func TestGenerateAlertInvestigationPrompt(t *testing.T) {
	out := GenerateAlertInvestigationPrompt(AlertInvestigationRequest{
		Level:        "critical",
		ResourceName: "pve1",
		ResourceType: "node",
		AlertType:    "cpu",
		Value:        95,
		Threshold:    80,
		Duration:     "5 mins",
		Node:         "pve1",
	})
	if !strings.Contains(out, "Investigate this CRITICAL alert") ||
		!strings.Contains(out, "**Resource:** pve1 (node)") ||
		!strings.Contains(out, "**Node:** pve1") {
		t.Fatalf("unexpected prompt: %s", out)
	}
}

