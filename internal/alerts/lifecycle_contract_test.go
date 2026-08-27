package alerts

// The lifecycle contract suite (docs/ALERT_ENGINE_EVOLUTION.md): end-to-end
// assertions over the alert manager's externally observable behavior, pinned
// as a release gate. Each contract is a regression class from the issue
// record:
//
//   - a config save that changes nothing emits nothing (#1682: config saves
//     dispatched resolved notifications for alerts that were still active)
//   - a config change that genuinely resolves an alert resolves it exactly
//     once, with no fire event and no core residue
//   - an acknowledged alert does not escalate (and resumes when unacked)
//   - an acknowledged alert's recovery notification is suppressed
//
// The grouped-rendering counterpart (#1683: N grouped alerts must render N
// lines on every surface) lives in
// internal/notifications/grouped_rendering_contract_test.go.

import (
	"sync"
	"testing"
	"time"
)

type contractCallbackCapture struct {
	mu       sync.Mutex
	fired    []string
	resolved []string
}

func (c *contractCallbackCapture) install(m *Manager) {
	m.SetAlertCallback(func(alert *Alert) {
		if alert == nil {
			return
		}
		c.mu.Lock()
		c.fired = append(c.fired, alert.ID)
		c.mu.Unlock()
	})
	m.SetResolvedCallback(func(alertID string) {
		c.mu.Lock()
		c.resolved = append(c.resolved, alertID)
		c.mu.Unlock()
	})
}

func (c *contractCallbackCapture) counts() (fired, resolved int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.fired), len(c.resolved)
}

func (c *contractCallbackCapture) reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.fired = c.fired[:0]
	c.resolved = c.resolved[:0]
}

// waitFor blocks until the capture holds at least the wanted counts.
// Callback fan-out runs on goroutines spawned inside the dispatch, so
// assertions that expect events must wait; assertions that expect silence
// use settle.
func (c *contractCallbackCapture) waitFor(t *testing.T, wantFired, wantResolved int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		fired, resolved := c.counts()
		if fired >= wantFired && resolved >= wantResolved {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	fired, resolved := c.counts()
	t.Fatalf("timed out waiting for callbacks: fired=%d/%d resolved=%d/%d", fired, wantFired, resolved, wantResolved)
}

// settle gives in-flight callback goroutines time to land before a
// zero-events assertion.
func contractSettle() {
	time.Sleep(150 * time.Millisecond)
}

// contractRaiseGuestCPUAlert drives a real metric evaluation to a firing
// alert whose threshold comes from the config's guest defaults, so config
// changes re-evaluate it the same way production does.
func contractRaiseGuestCPUAlert(t *testing.T, m *Manager, resourceID string, value float64) *Alert {
	t.Helper()
	cfg := m.GetConfig()
	threshold := cfg.GuestDefaults.CPU
	if threshold == nil {
		t.Fatal("test config must define a guest CPU threshold")
	}
	m.checkMetric(resourceID, "Contract VM "+resourceID, "node1", "inst1", "guest", "cpu", value, threshold, nil)
	for _, alert := range m.GetActiveAlerts() {
		if alert.ResourceID == resourceID {
			alertCopy := alert
			return &alertCopy
		}
	}
	t.Fatalf("expected firing CPU alert for %s at value %v", resourceID, value)
	return nil
}

func contractTestConfig(m *Manager) AlertConfig {
	cfg := m.GetConfig()
	cfg.Enabled = true
	cfg.ActivationState = ActivationActive
	cfg.GuestDefaults.CPU = &HysteresisThreshold{Trigger: 80, Clear: 75}
	cfg.TimeThresholds = map[string]int{"guest": 0}
	cfg.MetricTimeThresholds = nil
	cfg.MinimumDelta = 0
	return cfg
}

func TestContractConfigSaveChangingNothingEmitsNothing(t *testing.T) {
	m := newTestManager(t)
	m.UpdateConfig(contractTestConfig(m))

	capture := &contractCallbackCapture{}
	capture.install(m)

	alert := contractRaiseGuestCPUAlert(t, m, "contract-vm-1", 95)
	capture.waitFor(t, 1, 0)
	startTime := alert.StartTime
	capture.reset()

	// The contract: an unchanged config saved back emits no lifecycle
	// events and disturbs no active alert.
	m.UpdateConfig(m.GetConfig())
	contractSettle()

	fired, resolved := capture.counts()
	if fired != 0 || resolved != 0 {
		t.Fatalf("no-op config save emitted lifecycle events: fired=%d resolved=%d", fired, resolved)
	}
	m.mu.RLock()
	after, ok := testCanonicalAlertLookupNoLock(m.activeAlerts, canonicalMetricStateID("contract-vm-1", "cpu"))
	m.mu.RUnlock()
	if !ok || after == nil {
		t.Fatal("no-op config save removed an active alert")
	}
	if !after.StartTime.Equal(startTime) {
		t.Fatalf("no-op config save disturbed the alert's start time: %v -> %v", startTime, after.StartTime)
	}
}

func TestContractConfigChangeResolvesExactlyOnce(t *testing.T) {
	m := newTestManager(t)
	m.UpdateConfig(contractTestConfig(m))

	capture := &contractCallbackCapture{}
	capture.install(m)

	contractRaiseGuestCPUAlert(t, m, "contract-vm-2", 85)
	capture.waitFor(t, 1, 0)
	capture.reset()

	// Raising the trigger above the alert's value must resolve it exactly
	// once — one resolved event, zero fired events — and leave no incident
	// behind in the reducer core.
	cfg := m.GetConfig()
	cfg.GuestDefaults.CPU = &HysteresisThreshold{Trigger: 90, Clear: 85}
	m.UpdateConfig(cfg)
	contractSettle()

	fired, resolved := capture.counts()
	if fired != 0 {
		t.Fatalf("threshold raise emitted %d fire events, want 0", fired)
	}
	if resolved != 1 {
		t.Fatalf("threshold raise emitted %d resolve events, want exactly 1", resolved)
	}
	m.mu.RLock()
	_, stillActive := testCanonicalAlertLookupNoLock(m.activeAlerts, canonicalMetricStateID("contract-vm-2", "cpu"))
	coreResidue := testCoreHasIncident(m, "contract-vm-2", canonicalMetricSpecID("contract-vm-2", "cpu"))
	m.mu.RUnlock()
	if stillActive {
		t.Fatal("alert remained active after the threshold moved above its value")
	}
	if coreResidue {
		t.Fatal("reducer core still tracks the incident after config-change resolution")
	}

	// Saving the same config again must not re-resolve or re-fire.
	capture.reset()
	m.UpdateConfig(m.GetConfig())
	contractSettle()
	fired, resolved = capture.counts()
	if fired != 0 || resolved != 0 {
		t.Fatalf("follow-up no-op save emitted events: fired=%d resolved=%d", fired, resolved)
	}
}

func TestContractAcknowledgeStopsEscalation(t *testing.T) {
	m := newTestManager(t)
	cfg := contractTestConfig(m)
	cfg.Schedule.Escalation = EscalationConfig{
		Enabled: true,
		Levels:  []EscalationLevel{{After: 5, Notify: "all"}},
	}
	m.UpdateConfig(cfg)

	capture := &contractCallbackCapture{}
	capture.install(m)

	var escalateMu sync.Mutex
	escalations := 0
	m.SetEscalateCallback(func(alert *Alert, level int) {
		escalateMu.Lock()
		escalations++
		escalateMu.Unlock()
	})

	alert := contractRaiseGuestCPUAlert(t, m, "contract-vm-3", 95)
	alertID := alert.ID
	m.mu.Lock()
	if active, ok := testCanonicalAlertLookupNoLock(m.activeAlerts, canonicalMetricStateID("contract-vm-3", "cpu")); ok && active != nil {
		active.StartTime = time.Now().Add(-10 * time.Minute)
	}
	m.mu.Unlock()

	if err := m.AcknowledgeAlert(alertID, "contract-user"); err != nil {
		t.Fatalf("acknowledge failed: %v", err)
	}
	m.checkEscalations()
	contractSettle()
	escalateMu.Lock()
	afterAck := escalations
	escalateMu.Unlock()
	if afterAck != 0 {
		t.Fatalf("acknowledged alert escalated %d times, want 0", afterAck)
	}

	if err := m.UnacknowledgeAlert(alertID); err != nil {
		t.Fatalf("unacknowledge failed: %v", err)
	}
	m.checkEscalations()
	deadline := time.Now().Add(2 * time.Second)
	for {
		escalateMu.Lock()
		afterUnack := escalations
		escalateMu.Unlock()
		if afterUnack == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("unacknowledged overdue alert escalated %d times, want exactly 1", afterUnack)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestContractAcknowledgedRecoveryIsSuppressed(t *testing.T) {
	m := newTestManager(t)
	m.UpdateConfig(contractTestConfig(m))

	capture := &contractCallbackCapture{}
	capture.install(m)

	exported := contractRaiseGuestCPUAlert(t, m, "contract-vm-4", 95)
	capture.waitFor(t, 1, 0)

	m.mu.RLock()
	alert, _ := testCanonicalAlertLookupNoLock(m.activeAlerts, canonicalMetricStateID("contract-vm-4", "cpu"))
	m.mu.RUnlock()
	if alert == nil {
		t.Fatalf("active alert missing for exported id %s", exported.ID)
	}
	if alert.LastNotified == nil {
		t.Fatal("dispatched firing did not record LastNotified on the active alert")
	}
	if m.ShouldSuppressResolvedNotification(alert) {
		t.Fatal("a delivered, unacknowledged alert's recovery must not be suppressed")
	}
	if err := m.AcknowledgeAlert(exported.ID, "contract-user"); err != nil {
		t.Fatalf("acknowledge failed: %v", err)
	}
	m.mu.RLock()
	acked, _ := testCanonicalAlertLookupNoLock(m.activeAlerts, canonicalMetricStateID("contract-vm-4", "cpu"))
	m.mu.RUnlock()
	if acked == nil || !acked.Acknowledged {
		t.Fatal("acknowledgement did not stick")
	}
	if !m.ShouldSuppressResolvedNotification(acked) {
		t.Fatal("acknowledged alert's recovery notification must be suppressed")
	}
}
