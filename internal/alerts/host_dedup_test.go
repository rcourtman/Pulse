package alerts

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// testRegisterHostAgentNodeLink applies a link the way CheckHost does, as the
// agent's newest report.
func testRegisterHostAgentNodeLink(m *Manager, link hostAgentNodeLink) {
	m.applyHostAgentNodeLink(link, m.beginHostAgentReport(link.agentID))
}

func TestHostAgentDeduplicatesNodeAlerts(t *testing.T) {
	agent := hostAgentNodeLink{agentID: "host-pi", nodeID: "node/pi", cpu: true, memory: true, disk: true}

	// Test 1: Without a linked host agent, node metrics ARE checked
	t.Run("node_metrics_checked_without_agent", func(t *testing.T) {
		m := NewManager()
		m.config.Enabled = true
		m.config.NodeDefaults.CPU = &HysteresisThreshold{Trigger: 80, Clear: 75}

		// Verify no host agent is linked
		if m.hasHostAgentForNode("node/pi") {
			t.Error("Expected no host agent for 'node/pi' initially")
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
		_, hasPending := m.core.Incident(node.ID, canonicalMetricSpecID(node.ID, "cpu"))
		m.mu.RUnlock()

		if !hasPending {
			t.Error("Expected pending alert for node CPU when no host agent registered")
		}
	})

	// Test 2: With a host agent linked to the node, node metrics are NOT checked
	t.Run("node_metrics_skipped_with_agent", func(t *testing.T) {
		m := NewManager()
		m.config.Enabled = true
		m.config.NodeDefaults.CPU = &HysteresisThreshold{Trigger: 80, Clear: 75}

		// Link a host agent to the node BEFORE checking the node
		testRegisterHostAgentNodeLink(m, agent)

		// Verify host agent IS linked
		if !m.hasHostAgentForNode("node/pi") {
			t.Error("Expected host agent for 'node/pi' to be linked")
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
		_, hasPending := m.core.Incident(node.ID, canonicalMetricSpecID(node.ID, "cpu"))
		m.mu.RUnlock()

		if hasPending {
			t.Error("Expected NO pending alert for node CPU when host agent is linked")
		}
	})

	// Test 3: After unregistering host agent, node metrics ARE checked again
	t.Run("node_metrics_resume_after_agent_unregistered", func(t *testing.T) {
		m := NewManager()
		m.config.Enabled = true
		m.config.NodeDefaults.CPU = &HysteresisThreshold{Trigger: 80, Clear: 75}

		// Register and then unregister
		testRegisterHostAgentNodeLink(m, agent)
		m.unregisterHostAgentNodeLink(agent.agentID)

		// Verify host agent is NOT linked
		if m.hasHostAgentForNode("node/pi") {
			t.Error("Expected host agent for 'node/pi' to be unregistered")
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
		_, hasPending := m.core.Incident(node.ID, canonicalMetricSpecID(node.ID, "cpu"))
		m.mu.RUnlock()

		if !hasPending {
			t.Error("Expected pending alert for node CPU after host agent unregistration")
		}
	})
}

// Deduplication follows the agent's node link, never a hostname match: two
// instances can both have a node named "pve", and an agent can report an FQDN.
func TestHostAgentDeduplicationFollowsNodeLink(t *testing.T) {
	m := NewManager()
	m.config.Enabled = true

	m.CheckHost(models.Host{ID: "host-a", Hostname: "pve.example.com", LinkedNodeID: "site-a-pve", CPUUsage: 10})
	if !m.hasHostAgentForNode("site-a-pve") {
		t.Error("Expected the linked node to dedup even though the agent hostname is an FQDN")
	}
	if m.hasHostAgentForNode("site-b-pve") {
		t.Error("Expected a same-named node in another instance to keep its own alerts")
	}

	m.CheckHost(models.Host{ID: "host-b", Hostname: "pve", CPUUsage: 10})
	if m.hasHostAgentForNode("site-b-pve") {
		t.Error("Expected an unlinked agent with a matching hostname not to suppress node alerts")
	}

	// An operator unlinking the agent hands the node's alerts back.
	m.CheckHost(models.Host{ID: "host-a", Hostname: "pve.example.com", CPUUsage: 10})
	if m.hasHostAgentForNode("site-a-pve") {
		t.Error("Expected an unlinked agent to stop deduplicating its former node")
	}
}

func TestCheckHostRegistersNodeLink(t *testing.T) {
	m := NewManager()
	m.config.Enabled = true

	host := models.Host{
		ID:           "host-test-123",
		Hostname:     "testhost",
		LinkedNodeID: "pve-testhost",
		CPUUsage:     50,
	}

	// Initially no node link registered
	if m.hasHostAgentForNode("pve-testhost") {
		t.Error("Expected no host agent registered initially")
	}

	// CheckHost should register the node link
	m.CheckHost(host)

	if !m.hasHostAgentForNode("pve-testhost") {
		t.Error("Expected host agent node link to be registered after CheckHost")
	}
}

func TestHandleHostOfflineUnregistersNodeLink(t *testing.T) {
	m := NewManager()
	m.config.Enabled = true

	host := models.Host{
		ID:           "host-test-456",
		Hostname:     "offlinehost",
		LinkedNodeID: "pve-offlinehost",
	}

	// Register the node link
	testRegisterHostAgentNodeLink(m, hostAgentNodeLink{agentID: host.ID, nodeID: host.LinkedNodeID, cpu: true})

	if !m.hasHostAgentForNode("pve-offlinehost") {
		t.Error("Expected host agent registered")
	}

	// HandleHostOffline should unregister the node link
	m.HandleHostOffline(host)

	if m.hasHostAgentForNode("pve-offlinehost") {
		t.Error("Expected host agent to be unregistered after HandleHostOffline")
	}
}
