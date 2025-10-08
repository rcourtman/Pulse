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
