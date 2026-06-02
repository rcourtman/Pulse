package portal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
)

func writeSetupFactJSON(t *testing.T, dir string, leaf string, payload any) {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal %s: %v", leaf, err)
	}
	path := filepath.Join(dir, leaf)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestTenantDirWorkspaceSetupFactReaderCountsTenantFacts(t *testing.T) {
	tenantsDir := t.TempDir()
	orgDir := filepath.Join(tenantsDir, "ws_one", "orgs", "ws_one")
	if err := os.MkdirAll(orgDir, 0o755); err != nil {
		t.Fatal(err)
	}

	lastUsed := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)
	expiredAt := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	writeSetupFactJSON(t, orgDir, "api_tokens.json", []config.APITokenRecord{
		{
			ID:         "agent-used",
			Name:       "Agent",
			Hash:       "hash",
			CreatedAt:  lastUsed.Add(-time.Hour),
			LastUsedAt: &lastUsed,
			Scopes:     []string{config.ScopeAgentReport},
		},
		{
			ID:        "agent-generated-only",
			Name:      "Generated only",
			Hash:      "hash",
			CreatedAt: lastUsed.Add(-time.Hour),
			Scopes:    []string{config.ScopeAgentReport},
		},
		{
			ID:         "expired-agent",
			Name:       "Expired",
			Hash:       "hash",
			CreatedAt:  expiredAt.Add(-time.Hour),
			LastUsedAt: &expiredAt,
			ExpiresAt:  &expiredAt,
			Scopes:     []string{config.ScopeAgentReport},
		},
		{
			ID:         "settings-only",
			Name:       "Settings",
			Hash:       "hash",
			CreatedAt:  lastUsed.Add(-time.Hour),
			LastUsedAt: &lastUsed,
			Scopes:     []string{config.ScopeSettingsRead},
		},
	})
	writeSetupFactJSON(t, orgDir, "email.enc", notifications.EmailConfig{
		Enabled: true,
		To:      []string{"ops@example.com", "  ", "client@example.com"},
	})
	writeSetupFactJSON(t, orgDir, "apprise.enc", notifications.AppriseConfig{
		Enabled: true,
		Targets: []string{"discord://token"},
	})
	writeSetupFactJSON(t, orgDir, "webhooks.enc", []notifications.WebhookConfig{
		{Enabled: true, URL: "https://example.com/hook"},
		{Enabled: false, URL: "https://example.com/disabled"},
		{Enabled: true},
	})
	writeSetupFactJSON(t, orgDir, "report_schedules.json", []map[string]any{
		{"name": "Monthly", "enabled": true},
		{"name": "Disabled", "enabled": false},
	})

	facts := NewTenantDirWorkspaceSetupFactReader(tenantsDir).FactsForWorkspace("ws_one")
	if facts.AgentCount == nil || *facts.AgentCount != 1 {
		t.Fatalf("AgentCount = %v, want 1", facts.AgentCount)
	}
	if facts.AgentTokenCount == nil || *facts.AgentTokenCount != 2 {
		t.Fatalf("AgentTokenCount = %v, want 2", facts.AgentTokenCount)
	}
	if facts.UnusedAgentTokenCount == nil || *facts.UnusedAgentTokenCount != 1 {
		t.Fatalf("UnusedAgentTokenCount = %v, want 1", facts.UnusedAgentTokenCount)
	}
	if facts.LastAgentSeenAt == nil || !facts.LastAgentSeenAt.Equal(lastUsed) {
		t.Fatalf("LastAgentSeenAt = %v, want %v", facts.LastAgentSeenAt, lastUsed)
	}
	if facts.AlertRouteCount == nil || *facts.AlertRouteCount != 4 {
		t.Fatalf("AlertRouteCount = %v, want 4", facts.AlertRouteCount)
	}
	if facts.DisabledAlertRouteCount == nil || *facts.DisabledAlertRouteCount != 1 {
		t.Fatalf("DisabledAlertRouteCount = %v, want 1", facts.DisabledAlertRouteCount)
	}
	if facts.ReportScheduleCount == nil || *facts.ReportScheduleCount != 1 {
		t.Fatalf("ReportScheduleCount = %v, want 1", facts.ReportScheduleCount)
	}
	if facts.DisabledReportScheduleCount == nil || *facts.DisabledReportScheduleCount != 1 {
		t.Fatalf("DisabledReportScheduleCount = %v, want 1", facts.DisabledReportScheduleCount)
	}
}

func TestTenantDirWorkspaceSetupFactReaderMissingFactsAreZero(t *testing.T) {
	tenantsDir := t.TempDir()
	orgDir := filepath.Join(tenantsDir, "ws_empty", "orgs", "ws_empty")
	if err := os.MkdirAll(orgDir, 0o755); err != nil {
		t.Fatal(err)
	}

	facts := NewTenantDirWorkspaceSetupFactReader(tenantsDir).FactsForWorkspace("ws_empty")
	if facts.AgentCount == nil || *facts.AgentCount != 0 {
		t.Fatalf("AgentCount = %v, want 0", facts.AgentCount)
	}
	if facts.AgentTokenCount == nil || *facts.AgentTokenCount != 0 {
		t.Fatalf("AgentTokenCount = %v, want 0", facts.AgentTokenCount)
	}
	if facts.UnusedAgentTokenCount == nil || *facts.UnusedAgentTokenCount != 0 {
		t.Fatalf("UnusedAgentTokenCount = %v, want 0", facts.UnusedAgentTokenCount)
	}
	if facts.AlertRouteCount == nil || *facts.AlertRouteCount != 0 {
		t.Fatalf("AlertRouteCount = %v, want 0", facts.AlertRouteCount)
	}
	if facts.DisabledAlertRouteCount == nil || *facts.DisabledAlertRouteCount != 0 {
		t.Fatalf("DisabledAlertRouteCount = %v, want 0", facts.DisabledAlertRouteCount)
	}
	if facts.ReportScheduleCount == nil || *facts.ReportScheduleCount != 0 {
		t.Fatalf("ReportScheduleCount = %v, want 0", facts.ReportScheduleCount)
	}
	if facts.DisabledReportScheduleCount == nil || *facts.DisabledReportScheduleCount != 0 {
		t.Fatalf("DisabledReportScheduleCount = %v, want 0", facts.DisabledReportScheduleCount)
	}
}

func TestTenantDirWorkspaceSetupFactReaderRejectsUnsafeTenantID(t *testing.T) {
	tenantsDir := t.TempDir()
	facts := NewTenantDirWorkspaceSetupFactReader(tenantsDir).FactsForWorkspace("../ws")
	if facts.AgentCount != nil ||
		facts.AgentTokenCount != nil ||
		facts.UnusedAgentTokenCount != nil ||
		facts.AlertRouteCount != nil ||
		facts.DisabledAlertRouteCount != nil ||
		facts.ReportScheduleCount != nil ||
		facts.DisabledReportScheduleCount != nil {
		t.Fatalf("unsafe tenant facts = %+v, want empty facts", facts)
	}
}
