package ai

import (
	"context"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestShouldResolveAlert_DoesNotResolveUsageWithoutModel(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	now := time.Now()

	usageAlert := AlertInfo{
		ID:         "alert-usage",
		Type:       "usage",
		ResourceID: "storage-1",
		Value:      92,
		Threshold:  90,
		StartTime:  now.Add(-2 * time.Hour),
	}
	state := models.StateSnapshot{
		Storage: []models.Storage{{ID: "storage-1", Usage: 80}},
	}
	resolved, reason := ps.shouldResolveAlertState(context.Background(), usageAlert, patrolRuntimeStateForTest(ps, state), nil, "")
	if resolved || reason != "" {
		t.Fatalf("expected usage alert to remain unresolved without model review, got resolved=%v reason=%q", resolved, reason)
	}

	missingAlert := AlertInfo{
		ID:         "alert-missing",
		Type:       "usage",
		ResourceID: "storage-2",
		Value:      95,
		Threshold:  90,
		StartTime:  now.Add(-25 * time.Hour),
	}
	resolved, reason = ps.shouldResolveAlertState(context.Background(), missingAlert, patrolRuntimeStateForTest(ps, state), nil, "")
	if resolved || reason != "" {
		t.Fatalf("expected missing storage alert to remain unresolved without model review, got resolved=%v reason=%q", resolved, reason)
	}
}

func TestShouldResolveAlert_ReadStateDoesNotResolveWithoutModel(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	now := time.Now()
	alert := AlertInfo{
		ID:         "alert-usage-readstate",
		Type:       "usage",
		ResourceID: "storage-1",
		Value:      92,
		Threshold:  90,
		StartTime:  now.Add(-2 * time.Hour),
	}

	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(models.StateSnapshot{
		Storage: []models.Storage{{ID: "storage-1", Usage: 80}},
	})

	resolved, reason := ps.shouldResolveAlertState(context.Background(), alert, patrolRuntimeState{readState: registry}, nil, "")
	if resolved || reason != "" {
		t.Fatalf("expected readState alert to remain unresolved without model review, got resolved=%v reason=%q", resolved, reason)
	}
}

func TestShouldResolveAlert_CPUAndOfflineDoNotResolveWithoutModel(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	now := time.Now()

	cpuAlert := AlertInfo{
		ID:           "alert-cpu",
		Type:         "cpu",
		ResourceID:   "node-1",
		ResourceName: "node-1",
		ResourceType: "node",
		Value:        95,
		Threshold:    90,
		StartTime:    now.Add(-30 * time.Minute),
	}
	state := models.StateSnapshot{
		Nodes: []models.Node{{ID: "node-1", Name: "node-1", CPU: 0.10, Status: "online"}},
	}
	resolved, reason := ps.shouldResolveAlertState(context.Background(), cpuAlert, patrolRuntimeStateForTest(ps, state), nil, "")
	if resolved || reason != "" {
		t.Fatalf("expected cpu alert to remain unresolved without model review, got resolved=%v reason=%q", resolved, reason)
	}

	offlineAlert := AlertInfo{
		ID:           "alert-offline",
		Type:         "offline",
		ResourceID:   "vm-1",
		ResourceName: "vm-1",
		ResourceType: "vm",
		StartTime:    now.Add(-30 * time.Minute),
	}
	state.VMs = []models.VM{{ID: "vm-1", Name: "vm-1", Status: "running"}}
	resolved, reason = ps.shouldResolveAlertState(context.Background(), offlineAlert, patrolRuntimeStateForTest(ps, state), nil, "")
	if resolved || reason != "" {
		t.Fatalf("expected offline alert to remain unresolved without model review, got resolved=%v reason=%q", resolved, reason)
	}
}

func TestReviewAndResolveAlerts(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	resolver := &stubAlertResolver{}
	ps.alertResolver = resolver

	now := time.Now()
	resolver.alerts = []AlertInfo{
		{
			ID:         "recent",
			Type:       "usage",
			ResourceID: "storage-1",
			Value:      95,
			Threshold:  90,
			StartTime:  now.Add(-5 * time.Minute),
		},
		{
			ID:         "stale",
			Type:       "usage",
			ResourceID: "storage-1",
			Value:      95,
			Threshold:  90,
			StartTime:  now.Add(-30 * time.Minute),
		},
	}

	state := models.StateSnapshot{Storage: []models.Storage{{ID: "storage-1", Usage: 70}}}
	resolved := ps.reviewAndResolveAlertsState(context.Background(), patrolRuntimeStateForTest(ps, state), false, "")
	if resolved != 0 {
		t.Fatalf("expected no alert resolution without model review, got %d", resolved)
	}
	if len(resolver.clears) != 0 {
		t.Fatalf("expected no alerts to be resolved, got %v", resolver.clears)
	}
}
