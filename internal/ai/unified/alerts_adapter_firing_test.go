package unified

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// On a fresh install alert delivery is pending review: alerts fire and show in
// Pulse, and no notification is sent. Patrol must still be woken, because the
// trigger follows the alert and not the notification switch.
func TestAlertManagerToBridge_AlertWakesPatrolBeforeDeliveryIsTurnedOn(t *testing.T) {
	manager := alerts.NewManagerWithDataDir(t.TempDir())
	t.Cleanup(manager.Stop)
	if state := manager.GetConfig().ActivationState; state != alerts.ActivationPending {
		t.Fatalf("expected a fresh manager to start with delivery pending review, got %q", state)
	}
	// The monitor always installs its notification delivery callback.
	delivered := make(chan string, 4)
	manager.SetAlertCallback(func(alert *alerts.Alert) { delivered <- alert.ID })

	bridge := NewAlertBridge(NewUnifiedStore(DefaultAlertToFindingConfig()), DefaultBridgeConfig())
	bridge.SetAlertProvider(NewAlertManagerAdapter(manager))
	triggers := make(chan PatrolTriggerEvent, 4)
	bridge.SetPatrolTrigger(func(event PatrolTriggerEvent) { triggers <- event })
	bridge.Start()
	t.Cleanup(bridge.Stop)

	manager.CheckDockerHost(models.DockerHost{
		ID:          "docker01",
		DisplayName: "docker01",
		Hostname:    "docker01.lan",
		Containers: []models.DockerContainer{{
			ID:     "nextcloud-1",
			Name:   "nextcloud",
			State:  "running",
			Status: "Up 10 minutes",
			Health: "unhealthy",
		}},
	})

	select {
	case event := <-triggers:
		if event.Reason != "alert_fired" || event.AlertLevel != "critical" || event.AlertType != "docker-container-health" {
			t.Fatalf("unexpected trigger: %+v", event)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("a critical alert fired with delivery pending review and Patrol got no trigger")
	}

	select {
	case id := <-delivered:
		t.Fatalf("alert %s was delivered as a notification while delivery is pending review", id)
	case <-time.After(150 * time.Millisecond):
	}
}
