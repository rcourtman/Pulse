package alerts

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestDisableAllNodesOfflinePreventsOfflineAlert(t *testing.T) {
	manager := NewManager()

	// Reset state to avoid interference from persisted alerts.
	manager.mu.Lock()
	manager.activeAlerts = make(map[string]*Alert)
	manager.nodeOfflineCount = make(map[string]int)
	manager.mu.Unlock()

	config := manager.GetConfig()
	config.DisableAllNodesOffline = true
	manager.UpdateConfig(config)

	node := models.Node{
		ID:               "node-1",
		Name:             "node-1",
		Status:           "offline",
		ConnectionHealth: "error",
	}

	manager.CheckNode(node)

	manager.mu.RLock()
	_, alertExists := manager.activeAlerts["node-offline-node-1"]
	_, counterExists := manager.nodeOfflineCount["node-1"]
	manager.mu.RUnlock()

	if alertExists {
		t.Fatalf("expected no node offline alert when DisableAllNodesOffline is true")
	}
	if counterExists {
		t.Fatalf("expected node offline counter to be cleared when DisableAllNodesOffline is true")
	}
}

func TestUpdateConfigClearsExistingNodeOfflineAlerts(t *testing.T) {
	manager := NewManager()

	manager.mu.Lock()
	manager.activeAlerts = make(map[string]*Alert)
	manager.nodeOfflineCount = make(map[string]int)
	manager.activeAlerts["node-offline-node-1"] = &Alert{
		ID:           "node-offline-node-1",
		Type:         "connectivity",
		ResourceID:   "node-1",
		ResourceName: "node-1",
		Node:         "node-1",
		StartTime:    time.Now().Add(-5 * time.Minute),
		LastSeen:     time.Now(),
	}
	manager.nodeOfflineCount["node-1"] = 3
	manager.mu.Unlock()

	config := manager.GetConfig()
	config.DisableAllNodesOffline = true
	manager.UpdateConfig(config)

	// Allow asynchronous resolution callbacks, if any, to run.
	time.Sleep(50 * time.Millisecond)

	manager.mu.RLock()
	_, alertExists := manager.activeAlerts["node-offline-node-1"]
	_, counterExists := manager.nodeOfflineCount["node-1"]
	manager.mu.RUnlock()

	if alertExists {
		t.Fatalf("expected node offline alert to be cleared when DisableAllNodesOffline is enabled")
	}
	if counterExists {
		t.Fatalf("expected node offline counter to be reset when DisableAllNodesOffline is enabled")
	}
}

func TestUpdateConfigClearsDockerContainerAlertsWhenDisabled(t *testing.T) {
	manager := NewManager()

	containerResourceID := "docker:host-1/container-1"
	containerAlertIDs := []string{
		"docker-container-state-" + containerResourceID,
		"docker-container-health-" + containerResourceID,
		"docker-container-restart-loop-" + containerResourceID,
		"docker-container-oom-" + containerResourceID,
		"docker-container-memory-limit-" + containerResourceID,
	}

	manager.mu.Lock()
	for _, id := range containerAlertIDs {
		manager.activeAlerts[id] = &Alert{ID: id, ResourceID: containerResourceID}
	}
	manager.dockerStateConfirm[containerResourceID] = 2
	manager.dockerRestartTracking[containerResourceID] = &dockerRestartRecord{}
	manager.dockerLastExitCode[containerResourceID] = 137
	manager.mu.Unlock()

	config := manager.GetConfig()
	config.DisableAllDockerContainers = true
	manager.UpdateConfig(config)

	time.Sleep(10 * time.Millisecond)

	manager.mu.RLock()
	defer manager.mu.RUnlock()
	for _, id := range containerAlertIDs {
		if _, exists := manager.activeAlerts[id]; exists {
			t.Fatalf("expected docker container alert %s to be cleared when DisableAllDockerContainers is enabled", id)
		}
	}
	if len(manager.dockerStateConfirm) != 0 {
		t.Fatalf("expected dockerStateConfirm map to be cleared when DisableAllDockerContainers is enabled")
	}
	if len(manager.dockerRestartTracking) != 0 {
		t.Fatalf("expected dockerRestartTracking map to be cleared when DisableAllDockerContainers is enabled")
	}
	if len(manager.dockerLastExitCode) != 0 {
		t.Fatalf("expected dockerLastExitCode map to be cleared when DisableAllDockerContainers is enabled")
	}
}
