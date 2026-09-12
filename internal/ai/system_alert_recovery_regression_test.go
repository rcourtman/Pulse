package ai

import (
	"context"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestPatrolDoesNotReviewSystemAlertsAsMissingResources(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	resolver := &stubAlertResolver{alerts: []AlertInfo{{
		ID:   alerts.SystemAlertID(alerts.NotificationDeliveryAlertType),
		Type: alerts.NotificationDeliveryAlertType, ResourceName: "Pulse",
		StartTime: time.Now().Add(-25 * time.Hour),
	}}}
	provider := &mockPatrolProvider{response: "ALERT 1: RESOLVE: Pulse VM is no longer found"}
	ps.alertResolver = resolver
	ps.aiService = &Service{provider: provider, cfg: &config.AIConfig{Enabled: true, PatrolModel: "mock:model"}}
	resolved := ps.reviewAndResolveAlertsState(context.Background(), patrolRuntimeStateForTest(ps, models.StateSnapshot{}), true, "")
	if resolved != 0 || len(resolver.clears) != 0 || provider.calls != 0 {
		t.Fatalf("system health reviewed as inventory: resolved=%d clears=%v model calls=%d", resolved, resolver.clears, provider.calls)
	}
}

func TestAlertAdapterRejectsSystemAlertResolution(t *testing.T) {
	a := NewAlertManagerAdapter(&stubAlertManager{})
	for _, kind := range []string{alerts.NotificationDeliveryAlertType, alerts.DeadManDeliveryAlertType, alerts.DeadManMonitoringStalledAlertType} {
		if a.ResolveAlert(alerts.SystemAlertID(kind)) {
			t.Errorf("AI adapter allowed resolution of system health %s", kind)
		}
	}
	if !a.ResolveAlert("ordinary-resource-alert") {
		t.Fatal("ordinary resource resolution should remain available")
	}
}
