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

func TestAlertManagerAdapter_NilManager(t *testing.T) {
	a := NewAlertManagerAdapter(nil)
	if got := a.GetActiveAlerts(); got != nil {
		t.Fatalf("GetActiveAlerts = %+v, want nil", got)
	}
	if got := a.GetRecentlyResolved(30); got != nil {
		t.Fatalf("GetRecentlyResolved = %+v, want nil", got)
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

