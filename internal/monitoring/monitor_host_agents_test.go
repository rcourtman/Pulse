package monitoring

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

func TestEvaluateHostAgentsTriggersOfflineAlert(t *testing.T) {
	t.Helper()

	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	hostID := "host-offline"
	monitor.state.UpsertHost(models.Host{
		ID:              hostID,
		Hostname:        "offline.local",
		DisplayName:     "Offline Host",
		Status:          "online",
		IntervalSeconds: 30,
		LastSeen:        time.Now().Add(-10 * time.Minute),
	})

	now := time.Now()
	for i := 0; i < 3; i++ {
		monitor.evaluateHostAgents(now.Add(time.Duration(i) * time.Second))
	}

	snapshot := monitor.state.GetSnapshot()
	statusUpdated := false
	for _, host := range snapshot.Hosts {
		if host.ID == hostID {
			statusUpdated = true
			if got := host.Status; got != "offline" {
				t.Fatalf("expected host status offline, got %q", got)
			}
		}
	}
	if !statusUpdated {
		t.Fatalf("host %q not found in state snapshot", hostID)
	}

	connKey := hostConnectionPrefix + hostID
	if healthy, ok := snapshot.ConnectionHealth[connKey]; !ok || healthy {
		t.Fatalf("expected connection health false, got %v (exists=%v)", healthy, ok)
	}

	alerts := monitor.alertManager.GetActiveAlerts()
	found := false
	for _, alert := range alerts {
		if alert.ID == "host-offline-"+hostID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected host offline alert to remain active")
	}
}

func TestEvaluateHostAgentsClearsAlertWhenHostReturns(t *testing.T) {
	t.Helper()

	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	hostID := "host-recover"
	monitor.state.UpsertHost(models.Host{
		ID:              hostID,
		Hostname:        "recover.local",
		DisplayName:     "Recover Host",
		Status:          "online",
		IntervalSeconds: 30,
		LastSeen:        time.Now().Add(-10 * time.Minute),
	})

	for i := 0; i < 3; i++ {
		monitor.evaluateHostAgents(time.Now().Add(time.Duration(i) * time.Second))
	}

	monitor.state.UpsertHost(models.Host{
		ID:              hostID,
		Hostname:        "recover.local",
		DisplayName:     "Recover Host",
		Status:          "online",
		IntervalSeconds: 30,
		LastSeen:        time.Now(),
	})

	monitor.evaluateHostAgents(time.Now())

	snapshot := monitor.state.GetSnapshot()
	connKey := hostConnectionPrefix + hostID
	if healthy, ok := snapshot.ConnectionHealth[connKey]; !ok || !healthy {
		t.Fatalf("expected connection health true after recovery, got %v (exists=%v)", healthy, ok)
	}

	for _, alert := range monitor.alertManager.GetActiveAlerts() {
		if alert.ID == "host-offline-"+hostID {
			t.Fatalf("offline alert still active after recovery")
		}
	}
}

func TestApplyHostReportRejectsTokenReuseAcrossAgents(t *testing.T) {
	t.Helper()

	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	now := time.Now().UTC()
	baseReport := agentshost.Report{
		Agent: agentshost.AgentInfo{
			ID:              "agent-one",
			Version:         "1.0.0",
			IntervalSeconds: 30,
		},
		Host: agentshost.HostInfo{
			ID:        "machine-one",
			Hostname:  "host-one",
			Platform:  "linux",
			OSName:    "debian",
			OSVersion: "12",
		},
		Timestamp: now,
		Metrics: agentshost.Metrics{
			CPUUsagePercent: 1.0,
		},
	}

	token := &config.APITokenRecord{ID: "token-one", Name: "Token One"}

	hostOne, err := monitor.ApplyHostReport(baseReport, token)
	if err != nil {
		t.Fatalf("ApplyHostReport hostOne: %v", err)
	}
	if hostOne.ID == "" {
		t.Fatalf("expected hostOne to have an identifier")
	}

	secondReport := baseReport
	secondReport.Agent.ID = "agent-two"
	secondReport.Host.ID = "machine-two"
	secondReport.Host.Hostname = "host-two"
	secondReport.Timestamp = now.Add(30 * time.Second)

	if _, err := monitor.ApplyHostReport(secondReport, token); err == nil {
		t.Fatalf("expected token reuse across agents to be rejected")
	}
}

func TestRemoveHostAgentUnbindsToken(t *testing.T) {
	t.Helper()

	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	hostID := "host-to-remove"
	tokenID := "token-remove"
	monitor.state.UpsertHost(models.Host{
		ID:       hostID,
		Hostname: "remove.me",
		TokenID:  tokenID,
	})
	monitor.hostTokenBindings[tokenID] = "agent-remove"

	if _, err := monitor.RemoveHostAgent(hostID); err != nil {
		t.Fatalf("RemoveHostAgent: %v", err)
	}

	if _, exists := monitor.hostTokenBindings[tokenID]; exists {
		t.Fatalf("expected token binding to be cleared after host removal")
	}
}

func TestEvaluateHostAgentsEmptyHostsList(t *testing.T) {
	monitor := &Monitor{
		state:        models.NewState(),
		alertManager: alerts.NewManager(),
		config:       &config.Config{},
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	// No hosts in state - should complete without error or state changes
	monitor.evaluateHostAgents(time.Now())

	snapshot := monitor.state.GetSnapshot()
	if len(snapshot.Hosts) != 0 {
		t.Errorf("expected 0 hosts, got %d", len(snapshot.Hosts))
	}
	if len(snapshot.ConnectionHealth) != 0 {
		t.Errorf("expected 0 connection health entries, got %d", len(snapshot.ConnectionHealth))
	}
}

func TestEvaluateHostAgentsZeroIntervalUsesDefault(t *testing.T) {
	monitor := &Monitor{
		state:        models.NewState(),
		alertManager: alerts.NewManager(),
		config:       &config.Config{},
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	hostID := "host-zero-interval"
	// IntervalSeconds = 0, LastSeen = now, should use default interval (30s)
	// Default window = 30s * 4 = 120s, but minimum is 30s, so window = 30s
	// With LastSeen = now, the host should be healthy
	monitor.state.UpsertHost(models.Host{
		ID:              hostID,
		Hostname:        "zero-interval.local",
		Status:          "unknown",
		IntervalSeconds: 0, // Zero interval - should use default
		LastSeen:        time.Now(),
	})

	monitor.evaluateHostAgents(time.Now())

	snapshot := monitor.state.GetSnapshot()
	connKey := hostConnectionPrefix + hostID
	if healthy, ok := snapshot.ConnectionHealth[connKey]; !ok || !healthy {
		t.Fatalf("expected connection health true for zero-interval host with recent LastSeen, got %v (exists=%v)", healthy, ok)
	}

	for _, host := range snapshot.Hosts {
		if host.ID == hostID && host.Status != "online" {
			t.Errorf("expected host status online, got %q", host.Status)
		}
	}
}

func TestEvaluateHostAgentsNegativeIntervalUsesDefault(t *testing.T) {
	monitor := &Monitor{
		state:        models.NewState(),
		alertManager: alerts.NewManager(),
		config:       &config.Config{},
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	hostID := "host-negative-interval"
	monitor.state.UpsertHost(models.Host{
		ID:              hostID,
		Hostname:        "negative-interval.local",
		Status:          "unknown",
		IntervalSeconds: -10, // Negative interval - should use default
		LastSeen:        time.Now(),
	})

	monitor.evaluateHostAgents(time.Now())

	snapshot := monitor.state.GetSnapshot()
	connKey := hostConnectionPrefix + hostID
	if healthy, ok := snapshot.ConnectionHealth[connKey]; !ok || !healthy {
		t.Fatalf("expected connection health true for negative-interval host with recent LastSeen, got %v (exists=%v)", healthy, ok)
	}
}

func TestEvaluateHostAgentsWindowClampedToMinimum(t *testing.T) {
	monitor := &Monitor{
		state:        models.NewState(),
		alertManager: alerts.NewManager(),
		config:       &config.Config{},
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	hostID := "host-min-window"
	// IntervalSeconds = 1, so window = 1s * 4 = 4s, but minimum is 30s
	// Host last seen 25s ago should still be healthy (within 30s minimum window)
	now := time.Now()
	monitor.state.UpsertHost(models.Host{
		ID:              hostID,
		Hostname:        "min-window.local",
		Status:          "unknown",
		IntervalSeconds: 1, // Very small interval
		LastSeen:        now.Add(-25 * time.Second),
	})

	monitor.evaluateHostAgents(now)

	snapshot := monitor.state.GetSnapshot()
	connKey := hostConnectionPrefix + hostID
	if healthy, ok := snapshot.ConnectionHealth[connKey]; !ok || !healthy {
		t.Fatalf("expected connection health true (window clamped to minimum 30s), got %v (exists=%v)", healthy, ok)
	}

	for _, host := range snapshot.Hosts {
		if host.ID == hostID && host.Status != "online" {
			t.Errorf("expected host status online, got %q", host.Status)
		}
	}
}

func TestEvaluateHostAgentsWindowClampedToMaximum(t *testing.T) {
	monitor := &Monitor{
		state:        models.NewState(),
		alertManager: alerts.NewManager(),
		config:       &config.Config{},
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	hostID := "host-max-window"
	// IntervalSeconds = 300 (5 min), so window = 300s * 4 = 1200s (20 min)
	// But maximum is 10 min = 600s
	// Host last seen 11 minutes ago should be unhealthy (outside 10 min max window)
	now := time.Now()
	monitor.state.UpsertHost(models.Host{
		ID:              hostID,
		Hostname:        "max-window.local",
		Status:          "online",
		IntervalSeconds: 300, // 5 minute interval
		LastSeen:        now.Add(-11 * time.Minute),
	})

	monitor.evaluateHostAgents(now)

	snapshot := monitor.state.GetSnapshot()
	connKey := hostConnectionPrefix + hostID
	if healthy, ok := snapshot.ConnectionHealth[connKey]; !ok || healthy {
		t.Fatalf("expected connection health false (window clamped to maximum 10m), got %v (exists=%v)", healthy, ok)
	}

	for _, host := range snapshot.Hosts {
		if host.ID == hostID && host.Status != "offline" {
			t.Errorf("expected host status offline, got %q", host.Status)
		}
	}
}

func TestEvaluateHostAgentsRecentLastSeenIsHealthy(t *testing.T) {
	monitor := &Monitor{
		state:        models.NewState(),
		alertManager: alerts.NewManager(),
		config:       &config.Config{},
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	hostID := "host-recent"
	now := time.Now()
	// IntervalSeconds = 30, window = 30s * 4 = 120s (clamped to min 30s is not needed)
	// LastSeen = 10s ago, should be healthy
	monitor.state.UpsertHost(models.Host{
		ID:              hostID,
		Hostname:        "recent.local",
		Status:          "unknown",
		IntervalSeconds: 30,
		LastSeen:        now.Add(-10 * time.Second),
	})

	monitor.evaluateHostAgents(now)

	snapshot := monitor.state.GetSnapshot()
	connKey := hostConnectionPrefix + hostID
	if healthy, ok := snapshot.ConnectionHealth[connKey]; !ok || !healthy {
		t.Fatalf("expected connection health true for recent LastSeen, got %v (exists=%v)", healthy, ok)
	}

	for _, host := range snapshot.Hosts {
		if host.ID == hostID && host.Status != "online" {
			t.Errorf("expected host status online, got %q", host.Status)
		}
	}
}

func TestEvaluateHostAgentsZeroLastSeenIsUnhealthy(t *testing.T) {
	monitor := &Monitor{
		state:        models.NewState(),
		alertManager: alerts.NewManager(),
		config:       &config.Config{},
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	hostID := "host-zero-lastseen"
	monitor.state.UpsertHost(models.Host{
		ID:              hostID,
		Hostname:        "zero-lastseen.local",
		Status:          "online",
		IntervalSeconds: 30,
		LastSeen:        time.Time{}, // Zero time
	})

	monitor.evaluateHostAgents(time.Now())

	snapshot := monitor.state.GetSnapshot()
	connKey := hostConnectionPrefix + hostID
	if healthy, ok := snapshot.ConnectionHealth[connKey]; !ok || healthy {
		t.Fatalf("expected connection health false for zero LastSeen, got %v (exists=%v)", healthy, ok)
	}

	for _, host := range snapshot.Hosts {
		if host.ID == hostID && host.Status != "offline" {
			t.Errorf("expected host status offline for zero LastSeen, got %q", host.Status)
		}
	}
}

func TestEvaluateHostAgentsOldLastSeenIsUnhealthy(t *testing.T) {
	monitor := &Monitor{
		state:        models.NewState(),
		alertManager: alerts.NewManager(),
		config:       &config.Config{},
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	hostID := "host-old-lastseen"
	now := time.Now()
	// IntervalSeconds = 30, window = 30s * 4 = 120s
	// LastSeen = 5 minutes ago, should be unhealthy
	monitor.state.UpsertHost(models.Host{
		ID:              hostID,
		Hostname:        "old-lastseen.local",
		Status:          "online",
		IntervalSeconds: 30,
		LastSeen:        now.Add(-5 * time.Minute),
	})

	monitor.evaluateHostAgents(now)

	snapshot := monitor.state.GetSnapshot()
	connKey := hostConnectionPrefix + hostID
	if healthy, ok := snapshot.ConnectionHealth[connKey]; !ok || healthy {
		t.Fatalf("expected connection health false for old LastSeen, got %v (exists=%v)", healthy, ok)
	}

	for _, host := range snapshot.Hosts {
		if host.ID == hostID && host.Status != "offline" {
			t.Errorf("expected host status offline for old LastSeen, got %q", host.Status)
		}
	}
}

func TestEvaluateHostAgentsNilAlertManagerOnline(t *testing.T) {
	monitor := &Monitor{
		state:        models.NewState(),
		alertManager: nil, // No alert manager
		config:       &config.Config{},
	}

	hostID := "host-nil-am-online"
	monitor.state.UpsertHost(models.Host{
		ID:              hostID,
		Hostname:        "nil-am-online.local",
		Status:          "unknown",
		IntervalSeconds: 30,
		LastSeen:        time.Now(),
	})

	// Should not panic with nil alertManager
	monitor.evaluateHostAgents(time.Now())

	snapshot := monitor.state.GetSnapshot()
	connKey := hostConnectionPrefix + hostID
	if healthy, ok := snapshot.ConnectionHealth[connKey]; !ok || !healthy {
		t.Fatalf("expected connection health true, got %v (exists=%v)", healthy, ok)
	}

	for _, host := range snapshot.Hosts {
		if host.ID == hostID && host.Status != "online" {
			t.Errorf("expected host status online, got %q", host.Status)
		}
	}
}

func TestEvaluateHostAgentsNilAlertManagerOffline(t *testing.T) {
	monitor := &Monitor{
		state:        models.NewState(),
		alertManager: nil, // No alert manager
		config:       &config.Config{},
	}

	hostID := "host-nil-am-offline"
	monitor.state.UpsertHost(models.Host{
		ID:              hostID,
		Hostname:        "nil-am-offline.local",
		Status:          "online",
		IntervalSeconds: 30,
		LastSeen:        time.Time{}, // Zero time - unhealthy
	})

	// Should not panic with nil alertManager
	monitor.evaluateHostAgents(time.Now())

	snapshot := monitor.state.GetSnapshot()
	connKey := hostConnectionPrefix + hostID
	if healthy, ok := snapshot.ConnectionHealth[connKey]; !ok || healthy {
		t.Fatalf("expected connection health false, got %v (exists=%v)", healthy, ok)
	}

	for _, host := range snapshot.Hosts {
		if host.ID == hostID && host.Status != "offline" {
			t.Errorf("expected host status offline, got %q", host.Status)
		}
	}
}
