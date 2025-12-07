package alerts

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestHostAgentDeduplicatesNodeAlerts(t *testing.T) {
	// Test 1: Without a host agent registered, node metrics ARE checked
	t.Run("node_metrics_checked_without_host_agent", func(t *testing.T) {
		m := NewManager()
		m.config.Enabled = true
		m.config.NodeDefaults.CPU = &HysteresisThreshold{Trigger: 80, Clear: 75}

		// Verify no host agent is registered
		if m.hasHostAgentForNode("pi") {
			t.Error("Expected no host agent for 'pi' initially")
		}

		node := models.Node{
			ID:     "node/pi",
			Name:   "pi",
			Status: "online",
			CPU:    0.95, // 95% CPU
		}

		// This should attempt to check metrics (even if alert doesn't fire immediately due to time thresholds)
		m.CheckNode(node)

		// The key test: pendingAlerts should have an entry because metrics WERE checked
		m.mu.RLock()
		_, hasPending := m.pendingAlerts["node/pi-cpu"]
		m.mu.RUnlock()

		if !hasPending {
			t.Error("Expected pending alert for node CPU when no host agent registered")
		}
	})

	// Test 2: With host agent registered, node metrics are NOT checked
	t.Run("node_metrics_skipped_with_host_agent", func(t *testing.T) {
		m := NewManager()
		m.config.Enabled = true
		m.config.NodeDefaults.CPU = &HysteresisThreshold{Trigger: 80, Clear: 75}

		// Register a host agent with the same hostname BEFORE checking the node
		m.RegisterHostAgentHostname("pi")

		// Verify host agent IS registered
		if !m.hasHostAgentForNode("pi") {
			t.Error("Expected host agent for 'pi' to be registered")
		}

		node := models.Node{
			ID:     "node/pi",
			Name:   "pi",
			Status: "online",
			CPU:    0.95, // 95% CPU
		}

		m.CheckNode(node)

		// The key test: pendingAlerts should NOT have an entry because metrics were SKIPPED
		m.mu.RLock()
		_, hasPending := m.pendingAlerts["node/pi-cpu"]
		m.mu.RUnlock()

		if hasPending {
			t.Error("Expected NO pending alert for node CPU when host agent is registered")
		}
	})

	// Test 3: After unregistering host agent, node metrics ARE checked again
	t.Run("node_metrics_resume_after_host_agent_unregistered", func(t *testing.T) {
		m := NewManager()
		m.config.Enabled = true
		m.config.NodeDefaults.CPU = &HysteresisThreshold{Trigger: 80, Clear: 75}

		// Register and then unregister
		m.RegisterHostAgentHostname("pi")
		m.UnregisterHostAgentHostname("pi")

		// Verify host agent is NOT registered
		if m.hasHostAgentForNode("pi") {
			t.Error("Expected host agent for 'pi' to be unregistered")
		}

		node := models.Node{
			ID:     "node/pi",
			Name:   "pi",
			Status: "online",
			CPU:    0.95, // 95% CPU
		}

		m.CheckNode(node)

		// The key test: pendingAlerts should have an entry because metrics WERE checked
		m.mu.RLock()
		_, hasPending := m.pendingAlerts["node/pi-cpu"]
		m.mu.RUnlock()

		if !hasPending {
			t.Error("Expected pending alert for node CPU after host agent unregistration")
		}
	})
}

func TestHostAgentDeduplicationCaseInsensitive(t *testing.T) {
	m := NewManager()

	// Register with lowercase
	m.RegisterHostAgentHostname("myhost")

	// Check should match regardless of case
	if !m.hasHostAgentForNode("MYHOST") {
		t.Error("Expected hasHostAgentForNode to match uppercase hostname")
	}
	if !m.hasHostAgentForNode("MyHost") {
		t.Error("Expected hasHostAgentForNode to match mixed-case hostname")
	}
	if !m.hasHostAgentForNode("myhost") {
		t.Error("Expected hasHostAgentForNode to match lowercase hostname")
	}
}

func TestCheckHostRegistersHostname(t *testing.T) {
	m := NewManager()
	m.config.Enabled = true

	host := models.Host{
		ID:       "host-test-123",
		Hostname: "testhost",
		CPUUsage: 50,
	}

	// Initially no hostname registered
	if m.hasHostAgentForNode("testhost") {
		t.Error("Expected no host agent registered initially")
	}

	// CheckHost should register the hostname
	m.CheckHost(host)

	if !m.hasHostAgentForNode("testhost") {
		t.Error("Expected host agent hostname to be registered after CheckHost")
	}
}

func TestHandleHostOfflineUnregistersHostname(t *testing.T) {
	m := NewManager()
	m.config.Enabled = true

	host := models.Host{
		ID:       "host-test-456",
		Hostname: "offlinehost",
	}

	// Register the hostname
	m.RegisterHostAgentHostname(host.Hostname)

	if !m.hasHostAgentForNode("offlinehost") {
		t.Error("Expected host agent registered")
	}

	// HandleHostOffline should unregister the hostname
	m.HandleHostOffline(host)

	if m.hasHostAgentForNode("offlinehost") {
		t.Error("Expected host agent to be unregistered after HandleHostOffline")
	}
}
