package ai

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestShouldResolveAlert_UsageAndMissing(t *testing.T) {
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
	resolved, reason := ps.shouldResolveAlertState(context.Background(), usageAlert, patrolRuntimeStateForTest(ps, state), nil)
	if !resolved || !strings.Contains(reason, "usage dropped") {
		t.Fatalf("expected usage alert to resolve, got resolved=%v reason=%q", resolved, reason)
	}

	missingAlert := AlertInfo{
		ID:         "alert-missing",
		Type:       "usage",
		ResourceID: "storage-2",
		Value:      95,
		Threshold:  90,
		StartTime:  now.Add(-25 * time.Hour),
	}
	resolved, reason = ps.shouldResolveAlertState(context.Background(), missingAlert, patrolRuntimeStateForTest(ps, state), nil)
	if !resolved || !strings.Contains(reason, "resource no longer present") {
		t.Fatalf("expected missing storage alert to resolve, got resolved=%v reason=%q", resolved, reason)
	}
}

func TestShouldResolveAlert_UsageUsesReadState(t *testing.T) {
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

	resolved, reason := ps.shouldResolveAlertState(context.Background(), alert, patrolRuntimeState{readState: registry}, nil)
	if !resolved || !strings.Contains(reason, "usage dropped") {
		t.Fatalf("expected usage alert to resolve from readState, got resolved=%v reason=%q", resolved, reason)
	}
}

func TestShouldResolveAlert_CPUAndOffline(t *testing.T) {
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
	resolved, reason := ps.shouldResolveAlertState(context.Background(), cpuAlert, patrolRuntimeStateForTest(ps, state), nil)
	if !resolved || !strings.Contains(reason, "cpu dropped") {
		t.Fatalf("expected cpu alert to resolve, got resolved=%v reason=%q", resolved, reason)
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
	resolved, reason = ps.shouldResolveAlertState(context.Background(), offlineAlert, patrolRuntimeStateForTest(ps, state), nil)
	if !resolved || !strings.Contains(reason, "resource is now online") {
		t.Fatalf("expected offline alert to resolve, got resolved=%v reason=%q", resolved, reason)
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
	resolved := ps.reviewAndResolveAlertsState(context.Background(), patrolRuntimeStateForTest(ps, state), false)
	if resolved != 1 {
		t.Fatalf("expected 1 alert resolved, got %d", resolved)
	}
	if len(resolver.clears) != 1 || resolver.clears[0] != "stale" {
		t.Fatalf("expected stale alert to be resolved, got %v", resolver.clears)
	}
}
