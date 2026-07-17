package ai

import (
	"context"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestAlertStillFiring_MetricAtOrAboveThresholdSkipsModelReview(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		Storage: []models.Storage{{ID: "storage-1", Usage: 95}},
	}
	alert := AlertInfo{
		ID:         "alert-usage",
		Type:       "usage",
		ResourceID: "storage-1",
		Value:      92,
		Threshold:  90,
	}
	if !ps.alertStillFiringState(alert, patrolRuntimeStateForTest(ps, state)) {
		t.Fatalf("expected usage alert still above threshold to be treated as still firing")
	}
}

func TestAlertStillFiring_UncertainCasesGoToModel(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		Storage: []models.Storage{{ID: "storage-1", Usage: 80}},
		Nodes:   []models.Node{{ID: "node-1", Name: "node-1", CPU: 0.10, Status: "online"}},
	}
	snap := patrolRuntimeStateForTest(ps, state)

	cases := []struct {
		name  string
		alert AlertInfo
	}{
		{"metric below threshold", AlertInfo{Type: "usage", ResourceID: "storage-1", Threshold: 90}},
		{"resource missing", AlertInfo{Type: "usage", ResourceID: "storage-gone", Threshold: 90}},
		{"zero threshold", AlertInfo{Type: "cpu", ResourceID: "node-1", ResourceType: "node", Threshold: 0}},
		{"unknown alert type", AlertInfo{Type: "disk-health", ResourceID: "node-1", ResourceType: "node", Threshold: 1}},
		{"offline resource missing", AlertInfo{Type: "offline", ResourceID: "vm-gone", ResourceType: "vm"}},
		{"offline unhandled resource type", AlertInfo{Type: "offline", ResourceID: "storage-1", ResourceType: "storage"}},
	}
	for _, tc := range cases {
		if ps.alertStillFiringState(tc.alert, snap) {
			t.Errorf("%s: expected alert to go to model review, but gate claimed still firing", tc.name)
		}
	}
}

func TestAlertStillFiring_OfflineResourceStillDownSkipsModelReview(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		VMs: []models.VM{{ID: "vm-1", Name: "vm-1", Status: "stopped"}},
	}
	snap := patrolRuntimeStateForTest(ps, state)

	alert := AlertInfo{
		ID:           "alert-offline",
		Type:         "offline",
		ResourceID:   "vm-1",
		ResourceName: "vm-1",
		ResourceType: "vm",
	}
	if !ps.alertStillFiringState(alert, snap) {
		t.Fatalf("expected offline alert for a still-stopped VM to be treated as still firing")
	}

	state.VMs[0].Status = "running"
	snap = patrolRuntimeStateForTest(ps, state)
	if ps.alertStillFiringState(alert, snap) {
		t.Fatalf("expected offline alert for a running VM to go to model review")
	}
}

func TestAlertStillFiring_ReadState(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(models.StateSnapshot{
		Storage: []models.Storage{{ID: "storage-1", Usage: 95, Used: 95, Total: 100}},
	})
	snap := patrolRuntimeState{readState: registry}

	alert := AlertInfo{
		ID:         "alert-usage-readstate",
		Type:       "usage",
		ResourceID: "storage-1",
		Value:      92,
		Threshold:  90,
	}
	if !ps.alertStillFiringState(alert, snap) {
		t.Fatalf("expected readState usage alert above threshold to be treated as still firing")
	}
}

// Without a model, nothing resolves — Pulse must not use local heuristics to
// decide an issue has cleared, even when the metric has dropped below its
// threshold.
func TestReviewAndResolveAlerts_NoModelResolvesNothing(t *testing.T) {
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

	// Usage has dropped below threshold — only a model review may resolve.
	state := models.StateSnapshot{Storage: []models.Storage{{ID: "storage-1", Usage: 70}}}
	resolved := ps.reviewAndResolveAlertsState(context.Background(), patrolRuntimeStateForTest(ps, state), false, "")
	if resolved != 0 {
		t.Fatalf("expected no alert resolution without model review, got %d", resolved)
	}
	if len(resolver.clears) != 0 {
		t.Fatalf("expected no alerts to be resolved, got %v", resolver.clears)
	}
}

func TestReviewAndResolveAlerts_StillFiringMakesNoModelCalls(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	resolver := &stubAlertResolver{}
	ps.alertResolver = resolver
	provider := &mockPatrolProvider{response: "ALERT 1: RESOLVE: should never be asked"}
	ps.aiService = &Service{
		provider: provider,
		cfg:      &config.AIConfig{Enabled: true, PatrolModel: "mock:model"},
	}

	now := time.Now()
	resolver.alerts = []AlertInfo{
		{
			ID:         "still-firing",
			Type:       "usage",
			ResourceID: "storage-1",
			Value:      95,
			Threshold:  90,
			StartTime:  now.Add(-2 * time.Hour),
		},
	}

	// Usage still above threshold — the review could only answer KEEP.
	state := models.StateSnapshot{Storage: []models.Storage{{ID: "storage-1", Usage: 95}}}
	resolved := ps.reviewAndResolveAlertsState(context.Background(), patrolRuntimeStateForTest(ps, state), true, "")
	if resolved != 0 {
		t.Fatalf("expected no resolutions, got %d", resolved)
	}
	if provider.calls != 0 {
		t.Fatalf("expected no model calls for a still-firing alert, got %d", provider.calls)
	}
	if len(resolver.clears) != 0 {
		t.Fatalf("expected no alerts to be resolved, got %v", resolver.clears)
	}
}

func TestReviewAndResolveAlerts_BatchedModelReviewResolves(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	resolver := &stubAlertResolver{}
	ps.alertResolver = resolver
	provider := &mockPatrolProvider{response: "ALERT 1: RESOLVE: usage back to normal\nALERT 2: KEEP: still verifying"}
	ps.aiService = &Service{
		provider: provider,
		cfg:      &config.AIConfig{Enabled: true, PatrolModel: "mock:model"},
	}

	now := time.Now()
	resolver.alerts = []AlertInfo{
		{
			ID:         "cleared",
			Type:       "usage",
			ResourceID: "storage-1",
			Value:      95,
			Threshold:  90,
			StartTime:  now.Add(-2 * time.Hour),
		},
		{
			ID:           "ambiguous",
			Type:         "offline",
			ResourceID:   "vm-1",
			ResourceName: "vm-1",
			ResourceType: "vm",
			StartTime:    now.Add(-2 * time.Hour),
		},
		{
			ID:         "too-recent",
			Type:       "usage",
			ResourceID: "storage-1",
			Value:      95,
			Threshold:  90,
			StartTime:  now.Add(-5 * time.Minute),
		},
	}

	state := models.StateSnapshot{
		Storage: []models.Storage{{ID: "storage-1", Usage: 70}},
		VMs:     []models.VM{{ID: "vm-1", Name: "vm-1", Status: "running"}},
	}
	resolved := ps.reviewAndResolveAlertsState(context.Background(), patrolRuntimeStateForTest(ps, state), true, "")
	if provider.calls != 1 {
		t.Fatalf("expected one batched model call for two review candidates, got %d", provider.calls)
	}
	if resolved != 1 {
		t.Fatalf("expected exactly one resolution, got %d", resolved)
	}
	if len(resolver.clears) != 1 || resolver.clears[0] != "cleared" {
		t.Fatalf("expected only the model-resolved alert to clear, got %v", resolver.clears)
	}
}
