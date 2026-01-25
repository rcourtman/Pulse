package ai

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

type stubAlertResolver struct {
	alerts []AlertInfo
	clears []string
}

func (s *stubAlertResolver) GetActiveAlerts() []AlertInfo {
	return s.alerts
}

func (s *stubAlertResolver) ResolveAlert(alertID string) bool {
	s.clears = append(s.clears, alertID)
	return true
}

type stubAlertManagerClear struct {
	cleared []string
}

func (s *stubAlertManagerClear) GetActiveAlerts() []alerts.Alert {
	return nil
}

func (s *stubAlertManagerClear) GetRecentlyResolved() []models.ResolvedAlert {
	return nil
}

func (s *stubAlertManagerClear) ClearAlert(alertID string) bool {
	s.cleared = append(s.cleared, alertID)
	return true
}

func TestSetAlertResolverAndResolve(t *testing.T) {
	resolver := &stubAlertResolver{}
	service := &Service{
		patrolService: &PatrolService{},
	}

	service.SetAlertResolver(resolver)

	if service.patrolService.GetAlertResolver() != resolver {
		t.Fatalf("expected resolver to be set on patrol service")
	}

	manager := &stubAlertManagerClear{}
	adapter := NewAlertManagerAdapter(manager)
	if !adapter.ResolveAlert("alert-1") {
		t.Fatalf("expected ResolveAlert to return true")
	}
	if len(manager.cleared) != 1 || manager.cleared[0] != "alert-1" {
		t.Fatalf("expected alert-1 to be cleared, got %v", manager.cleared)
	}

	adapter = NewAlertManagerAdapter(nil)
	if adapter.ResolveAlert("alert-2") {
		t.Fatalf("expected ResolveAlert to return false when manager nil")
	}
}
