package ai

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
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

func TestService_buildAlertContext_NoActiveWithResolved(t *testing.T) {
	now := time.Now()
	resolved := make([]ResolvedAlertInfo, 0, 6)
	for i := 0; i < 6; i++ {
		resolved = append(resolved, ResolvedAlertInfo{
			AlertInfo: AlertInfo{
				ID:           "r" + string(rune('a'+i)),
				Type:         "disk",
				Level:        "warning",
				ResourceID:   "storage:local",
				ResourceName: "local",
				Message:      "disk ok",
				Duration:     "10 mins",
			},
			ResolvedTime: now.Add(-time.Duration(i) * time.Minute),
			Duration:     "10 mins",
		})
	}

	s := &Service{}
	s.SetAlertProvider(&stubAlertProvider{
		active:   nil,
		resolved: resolved,
	})

	ctx := s.buildAlertContext()
	if !strings.Contains(ctx, "No Active Alerts") {
		t.Fatalf("expected no active alerts section, got: %s", ctx)
	}
	if !strings.Contains(ctx, "Recently Resolved") {
		t.Fatalf("expected recently resolved section, got: %s", ctx)
	}
	if !strings.Contains(ctx, "... and 1 more") {
		t.Fatalf("expected overflow line for resolved alerts, got: %s", ctx)
	}
}

// The assistant reads recently resolved alerts through the adapter, so a node
// alert handed to its Pulse agent must reach the prompt as a handover and not
// as "resolved", which the model would read as the node being healthy.
func TestService_buildAlertContext_HandoverIsNotARecovery(t *testing.T) {
	now := time.Now()
	summary := "Alert moved to pve1 (Host Agent). This is not a recovery: check the agent for the current reading."
	adapter := NewAlertManagerAdapter(&stubAlertManager{resolved: []models.ResolvedAlert{
		{
			Alert: models.Alert{
				ID:           "pve1-memory",
				Type:         "memory",
				Level:        "warning",
				ResourceID:   "pve1",
				ResourceName: "pve1",
				Message:      "Memory usage at 95%",
				StartTime:    now.Add(-time.Hour),
				Resolution:   &models.AlertResolution{Reason: "moved_to_agent", Summary: summary},
			},
			ResolvedTime: now.Add(-time.Minute),
		},
		{
			Alert: models.Alert{
				ID:           "pve2-cpu",
				Type:         "cpu",
				Level:        "warning",
				ResourceID:   "pve2",
				ResourceName: "pve2",
				Message:      "CPU usage at 90%",
				StartTime:    now.Add(-time.Hour),
			},
			ResolvedTime: now.Add(-time.Minute),
		},
	}})

	resolved := adapter.GetRecentlyResolved(30)
	if len(resolved) != 2 || resolved[0].Resolution != summary || resolved[1].Resolution != "" {
		t.Fatalf("GetRecentlyResolved = %+v, want only the handover to carry a resolution", resolved)
	}
	if history := adapter.GetAlertHistory("pve1", 5); len(history) != 1 || history[0].Resolution != summary {
		t.Fatalf("GetAlertHistory = %+v, want the handover resolution", history)
	}

	s := &Service{}
	s.SetAlertProvider(adapter)
	ctx := s.buildAlertContext()
	if !strings.Contains(ctx, "- **memory** on pve1: Memory usage at 95% (lasted ") ||
		!strings.Contains(ctx, ", closed 1 minute ago) "+summary) {
		t.Fatalf("expected the handover line, got: %s", ctx)
	}
	if !strings.Contains(ctx, "- **cpu** on pve2: CPU usage at 90% (lasted ") ||
		!strings.Contains(ctx, ", resolved 1 minute ago)") {
		t.Fatalf("expected the ordinary recovery line, got: %s", ctx)
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

func TestService_buildTargetAlertContext_Empty(t *testing.T) {
	s := &Service{}
	if got := s.buildTargetAlertContext("node:pve1"); got != "" {
		t.Fatalf("expected empty string with no provider, got: %q", got)
	}

	s.SetAlertProvider(&stubAlertProvider{
		active: []AlertInfo{
			{ID: "a1", Level: "critical", Type: "cpu", ResourceID: "node:pve1", ResourceName: "pve1"},
		},
	})
	if got := s.buildTargetAlertContext(""); got != "" {
		t.Fatalf("expected empty string for empty resource ID, got: %q", got)
	}
	if got := s.buildTargetAlertContext("node:missing"); got != "" {
		t.Fatalf("expected empty string for missing resource, got: %q", got)
	}
}

func TestFormatTimeAgo(t *testing.T) {
	now := time.Now()
	if got := formatTimeAgo(now.Add(-10 * time.Second)); got != "just now" {
		t.Fatalf("formatTimeAgo(<1m) = %q", got)
	}
	if got := formatTimeAgo(now.Add(-1 * time.Minute)); got != "1 minute" {
		t.Fatalf("formatTimeAgo(1m) = %q", got)
	}
	if got := formatTimeAgo(now.Add(-2 * time.Minute)); got != "2 minutes" {
		t.Fatalf("formatTimeAgo(2m) = %q", got)
	}
	if got := formatTimeAgo(now.Add(-1 * time.Hour)); got != "1 hour" {
		t.Fatalf("formatTimeAgo(1h) = %q", got)
	}
	if got := formatTimeAgo(now.Add(-2 * time.Hour)); got != "2 hours" {
		t.Fatalf("formatTimeAgo(2h) = %q", got)
	}
	if got := formatTimeAgo(now.Add(-24 * time.Hour)); got != "1 day" {
		t.Fatalf("formatTimeAgo(1d) = %q", got)
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
	if !strings.Contains(out, "Ask for operator approval before running any diagnostic command") {
		t.Fatalf("expected approval-bound diagnostic guidance, got: %s", out)
	}
	if strings.Contains(out, "execute diagnostic commands") {
		t.Fatalf("prompt must not instruct autonomous diagnostic command execution: %s", out)
	}
}

func TestAlertInvestigationRequestJSONCanonicalOutput(t *testing.T) {
	req := AlertInvestigationRequest{
		AlertIdentifier: "instance:node:100::metric/cpu",
		ResourceID:      "agent-1",
		ResourceName:    "node-1",
		ResourceType:    "node",
		AlertType:       "cpu",
		Level:           "warning",
		Message:         "high cpu",
	}

	raw, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload["alertIdentifier"] != "instance:node:100::metric/cpu" {
		t.Fatalf("expected canonical alertIdentifier, got %#v", payload["alertIdentifier"])
	}
	if _, ok := payload["alert_id"]; ok {
		t.Fatalf("did not expect legacy alert_id in canonical payload, got %#v", payload["alert_id"])
	}

	var decoded AlertInvestigationRequest
	if err := json.Unmarshal([]byte(`{
		"alertIdentifier":"instance:node:100::metric/cpu",
		"resource_id":"agent-1",
		"resource_name":"node-1",
		"resource_type":"node",
		"alert_type":"cpu",
		"level":"warning",
		"message":"high cpu"
	}`), &decoded); err != nil {
		t.Fatalf("unmarshal canonical request: %v", err)
	}
	if decoded.AlertIdentifier != "instance:node:100::metric/cpu" {
		t.Fatalf("expected canonical alertIdentifier to load, got %q", decoded.AlertIdentifier)
	}
}
