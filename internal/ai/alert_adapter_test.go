package ai

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

type stubAlertManager struct {
	active   []alerts.Alert
	resolved []models.ResolvedAlert
}

func (s *stubAlertManager) GetActiveAlerts() []alerts.Alert { return s.active }
func (s *stubAlertManager) GetRecentlyResolved() []models.ResolvedAlert {
	return s.resolved
}
func (s *stubAlertManager) ClearAlert(alertID string) bool { return true }

func TestAlertManagerAdapter_NilManager(t *testing.T) {
	a := NewAlertManagerAdapter(nil)
	if got := a.GetActiveAlerts(); got != nil {
		t.Fatalf("GetActiveAlerts = %+v, want nil", got)
	}
	if got := a.GetRecentlyResolved(30); got != nil {
		t.Fatalf("GetRecentlyResolved = %+v, want nil", got)
	}
	if got := a.GetAlertsByResource("resource"); got != nil {
		t.Fatalf("GetAlertsByResource = %+v, want nil", got)
	}
	if got := a.GetAlertHistory("resource", 10); got != nil {
		t.Fatalf("GetAlertHistory = %+v, want nil", got)
	}
}

func TestAlertManagerAdapter_ConvertsAndFilters(t *testing.T) {
	now := time.Now()
	active := []alerts.Alert{
		{
			ID:           "a1",
			Type:         "node_cpu",
			Level:        alerts.AlertLevelCritical,
			ResourceID:   "node:pve1",
			ResourceName: "pve1",
			Value:        95,
			Threshold:    80,
			StartTime:    now.Add(-2*time.Minute - 10*time.Second),
			Metadata:     map[string]any{"resourceType": "node"},
		},
		{
			ID:           "a2",
			Type:         "guest_memory",
			Level:        alerts.AlertLevelWarning,
			ResourceID:   "guest:100",
			ResourceName: "vm-100",
			Value:        80,
			Threshold:    75,
			StartTime:    now.Add(-30 * time.Second),
		},
	}
	resolved := []models.ResolvedAlert{
		{
			Alert: models.Alert{
				ID:           "r1",
				Type:         "storage_usage",
				Level:        "critical",
				ResourceID:   "storage:local",
				ResourceName: "local",
				StartTime:    now.Add(-10 * time.Minute),
			},
			ResolvedTime: now.Add(-2 * time.Minute),
		},
		{
			Alert: models.Alert{
				ID:           "r2",
				Type:         "host_offline",
				Level:        "warning",
				ResourceID:   "host:h1",
				ResourceName: "h1",
				StartTime:    now.Add(-2 * time.Hour),
			},
			ResolvedTime: now.Add(-2 * time.Hour),
		},
	}

	a := NewAlertManagerAdapter(&stubAlertManager{active: active, resolved: resolved})

	gotActive := a.GetActiveAlerts()
	if len(gotActive) != 2 {
		t.Fatalf("GetActiveAlerts = %d, want 2", len(gotActive))
	}
	if gotActive[0].ResourceType != "node" {
		t.Fatalf("ResourceType = %q, want node", gotActive[0].ResourceType)
	}
	if gotActive[1].Duration == "" {
		t.Fatalf("expected Duration to be populated")
	}

	gotByResource := a.GetAlertsByResource("node:pve1")
	if len(gotByResource) != 1 || gotByResource[0].ID != "a1" {
		t.Fatalf("GetAlertsByResource = %+v", gotByResource)
	}

	gotRecent := a.GetRecentlyResolved(30)
	if len(gotRecent) != 1 || gotRecent[0].ID != "r1" {
		t.Fatalf("GetRecentlyResolved = %+v", gotRecent)
	}

	gotHistory := a.GetAlertHistory("storage:local", 1)
	if len(gotHistory) != 1 || gotHistory[0].ID != "r1" {
		t.Fatalf("GetAlertHistory = %+v", gotHistory)
	}
}

func TestInferResourceType(t *testing.T) {
	tests := []struct {
		name      string
		alertType string
		metadata  map[string]interface{}
		expected  string
	}{
		{"node_offline", "node_offline", nil, "node"},
		{"node_cpu", "node_cpu", nil, "node"},
		{"node_memory", "node_memory", nil, "node"},
		{"node_temperature", "node_temperature", nil, "node"},
		{"storage_usage", "storage_usage", nil, "storage"},
		{"storage", "storage", nil, "storage"},
		{"docker_cpu", "docker_cpu", nil, "docker"},
		{"docker_memory", "docker_memory", nil, "docker"},
		{"docker_restart", "docker_restart", nil, "docker"},
		{"docker_offline", "docker_offline", nil, "docker"},
		{"host_cpu", "host_cpu", nil, "host"},
		{"host_memory", "host_memory", nil, "host"},
		{"host_offline", "host_offline", nil, "host"},
		{"host_disk", "host_disk", nil, "host"},
		{"pmg", "pmg", nil, "pmg"},
		{"pmg_queue", "pmg_queue", nil, "pmg"},
		{"pmg_quarantine", "pmg_quarantine", nil, "pmg"},
		{"backup", "backup", nil, "backup"},
		{"backup_missing", "backup_missing", nil, "backup"},
		{"snapshot", "snapshot", nil, "snapshot"},
		{"snapshot_age", "snapshot_age", nil, "snapshot"},
		{"unknown_type", "unknown_type", nil, "guest"},
		{"with_metadata", "unknown", map[string]interface{}{"resourceType": "custom"}, "custom"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := inferResourceType(tt.alertType, tt.metadata)
			if result != tt.expected {
				t.Errorf("inferResourceType(%q, %v) = %q, want %q", tt.alertType, tt.metadata, result, tt.expected)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{"less_than_minute", 30 * time.Second, "< 1 min"},
		{"one_minute", 1 * time.Minute, "1 min"},
		{"5_minutes", 5 * time.Minute, "5 mins"},
		{"59_minutes", 59 * time.Minute, "59 mins"},
		{"one_hour", 1 * time.Hour, "1 hour"},
		{"2_hours", 2 * time.Hour, "2 hours"},
		{"1h_30m", 90 * time.Minute, "1h 30m"},
		{"one_day", 24 * time.Hour, "1 day"},
		{"2_days", 48 * time.Hour, "2 days"},
		{"1d_12h", 36 * time.Hour, "1d 12h"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDuration(tt.duration)
			if result != tt.expected {
				t.Errorf("formatDuration(%v) = %q, want %q", tt.duration, result, tt.expected)
			}
		})
	}
}

func TestAlertConversions_Nil(t *testing.T) {
	if info := convertAlertFromManager(nil); info.ID != "" {
		t.Fatalf("expected empty AlertInfo from nil manager alert, got %+v", info)
	}
	if info := convertAlertFromModels(nil); info.ID != "" {
		t.Fatalf("expected empty AlertInfo from nil models alert, got %+v", info)
	}
}
