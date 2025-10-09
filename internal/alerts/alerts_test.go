package alerts

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestAcknowledgePersistsThroughCheckMetric(t *testing.T) {
	m := NewManager()
	m.ClearActiveAlerts()
	cfg := m.GetConfig()
	cfg.TimeThreshold = 0
	cfg.TimeThresholds = map[string]int{}
	cfg.SuppressionWindow = 0
	cfg.MinimumDelta = 0
	m.UpdateConfig(cfg)

	threshold := &HysteresisThreshold{Trigger: 80, Clear: 70}
	m.checkMetric("res1", "Resource", "node1", "inst1", "guest", "usage", 90, threshold, nil)
	if _, exists := m.activeAlerts["res1-usage"]; !exists {
		t.Fatalf("expected alert to be created")
	}

	if err := m.AcknowledgeAlert("res1-usage", "tester"); err != nil {
		t.Fatalf("ack failed: %v", err)
	}

	if !m.activeAlerts["res1-usage"].Acknowledged {
		t.Fatalf("acknowledged flag not set")
	}

	alerts := m.GetActiveAlerts()
	if len(alerts) != 1 || !alerts[0].Acknowledged {
		t.Fatalf("GetActiveAlerts lost acknowledgement")
	}

	m.checkMetric("res1", "Resource", "node1", "inst1", "guest", "usage", 85, threshold, nil)
	if !m.activeAlerts["res1-usage"].Acknowledged {
		t.Fatalf("acknowledged flag lost after update")
	}
}

func TestHandleDockerHostRemovedClearsAlertsAndTracking(t *testing.T) {
	m := NewManager()
	host := models.DockerHost{ID: "host1", DisplayName: "Host One", Hostname: "host-one"}
	containerResourceID := "docker:host1/container1"
	containerAlertID := "docker-container-state-" + containerResourceID
	hostAlertID := "docker-host-offline-host1"

	m.mu.Lock()
	m.activeAlerts[hostAlertID] = &Alert{ID: hostAlertID, ResourceID: "docker:host1"}
	m.activeAlerts[containerAlertID] = &Alert{ID: containerAlertID, ResourceID: containerResourceID}
	m.dockerOfflineCount[host.ID] = 2
	m.dockerStateConfirm[containerResourceID] = 1
	m.dockerRestartTracking[containerResourceID] = &dockerRestartRecord{}
	m.dockerLastExitCode[containerResourceID] = 137
	m.mu.Unlock()

	m.HandleDockerHostRemoved(host)

	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, exists := m.activeAlerts[containerAlertID]; exists {
		t.Fatalf("expected container alerts to be cleared")
	}
	if _, exists := m.activeAlerts[hostAlertID]; exists {
		t.Fatalf("expected host offline alert to be cleared")
	}
	if _, exists := m.dockerOfflineCount[host.ID]; exists {
		t.Fatalf("expected offline tracking to be cleared")
	}
	if _, exists := m.dockerStateConfirm[containerResourceID]; exists {
		t.Fatalf("expected state confirmation to be cleared")
	}
	if _, exists := m.dockerRestartTracking[containerResourceID]; exists {
		t.Fatalf("expected restart tracking to be cleared")
	}
	if _, exists := m.dockerLastExitCode[containerResourceID]; exists {
		t.Fatalf("expected last exit code tracking to be cleared")
	}
}
