package unified

import (
	"testing"
	"time"
)

type stubAlertProvider struct {
	alerts     []AlertAdapter
	alertCb    func(AlertAdapter)
	resolvedCb func(string)
}

func (s *stubAlertProvider) GetActiveAlerts() []AlertAdapter {
	return s.alerts
}

func (s *stubAlertProvider) GetAlert(alertID string) AlertAdapter {
	for _, alert := range s.alerts {
		if alert.GetAlertID() == alertID {
			return alert
		}
	}
	return nil
}

func (s *stubAlertProvider) SetAlertCallback(cb func(AlertAdapter)) {
	s.alertCb = cb
}

func (s *stubAlertProvider) SetResolvedCallback(cb func(alertID string)) {
	s.resolvedCb = cb
}

func TestAlertBridge_StartStopAndSync(t *testing.T) {
	store := NewUnifiedStore(DefaultAlertToFindingConfig())
	bridge := NewAlertBridge(store, DefaultBridgeConfig())

	alert := &SimpleAlertAdapter{
		AlertID:      "alert-1",
		AlertType:    "cpu",
		AlertLevel:   "warning",
		ResourceID:   "vm-1",
		ResourceName: "web",
		Value:        90,
		Threshold:    80,
		StartTime:    time.Now(),
		LastSeen:     time.Now(),
	}
	provider := &stubAlertProvider{alerts: []AlertAdapter{alert}}
	bridge.SetAlertProvider(provider)

	bridge.Start()
	stats := bridge.Stats()
	if !stats.Running {
		t.Fatalf("expected bridge running")
	}
	if store.GetByAlert("alert-1") == nil {
		t.Fatalf("expected alert to be synced into store")
	}

	bridge.Stop()
	stats = bridge.Stats()
	if stats.Running {
		t.Fatalf("expected bridge stopped")
	}
}

func TestAlertBridge_HandleNewAlertAndEnhance(t *testing.T) {
	store := NewUnifiedStore(DefaultAlertToFindingConfig())
	bridge := NewAlertBridge(store, BridgeConfig{
		AutoEnhance:        true,
		EnhanceDelay:       10 * time.Millisecond,
		TriggerPatrolOnNew: true,
	})
	bridge.running = true

	patrolCh := make(chan string, 1)
	bridge.SetPatrolTrigger(func(resourceID, reason string) {
		patrolCh <- reason
	})

	enhanceCh := make(chan string, 1)
	bridge.SetAIEnhancement(func(findingID string) error {
		enhanceCh <- findingID
		return nil
	})

	alert := &SimpleAlertAdapter{
		AlertID:      "alert-1",
		AlertType:    "cpu",
		AlertLevel:   "warning",
		ResourceID:   "vm-1",
		ResourceName: "web",
		Value:        90,
		Threshold:    80,
		StartTime:    time.Now(),
		LastSeen:     time.Now(),
	}

	bridge.handleNewAlert(alert)

	select {
	case reason := <-patrolCh:
		if reason != "alert_fired" {
			t.Fatalf("expected patrol reason alert_fired, got %s", reason)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("expected patrol trigger")
	}

	select {
	case <-enhanceCh:
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("expected enhancement callback")
	}
}

func TestAlertBridge_HandleAlertResolved(t *testing.T) {
	store := NewUnifiedStore(DefaultAlertToFindingConfig())
	bridge := NewAlertBridge(store, BridgeConfig{
		TriggerPatrolOnClear: true,
	})
	bridge.running = true

	patrolCh := make(chan string, 1)
	bridge.SetPatrolTrigger(func(resourceID, reason string) {
		patrolCh <- reason
	})

	alert := &SimpleAlertAdapter{
		AlertID:      "alert-1",
		AlertType:    "cpu",
		AlertLevel:   "warning",
		ResourceID:   "vm-1",
		ResourceName: "web",
		Value:        90,
		Threshold:    80,
		StartTime:    time.Now(),
		LastSeen:     time.Now(),
	}
	finding, _ := store.AddFromAlert(alert)
	bridge.pendingEnhancements[finding.ID] = time.AfterFunc(time.Second, func() {})

	bridge.handleAlertResolved(alert.AlertID)

	select {
	case reason := <-patrolCh:
		if reason != "alert_cleared" {
			t.Fatalf("expected patrol reason alert_cleared, got %s", reason)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("expected patrol trigger")
	}

	if _, ok := bridge.pendingEnhancements[finding.ID]; ok {
		t.Fatalf("expected enhancement to be canceled")
	}
}

func TestAlertBridge_ScheduleEnhancementInactiveFinding(t *testing.T) {
	store := NewUnifiedStore(DefaultAlertToFindingConfig())
	bridge := NewAlertBridge(store, DefaultBridgeConfig())

	alert := &SimpleAlertAdapter{
		AlertID:      "alert-1",
		AlertType:    "cpu",
		AlertLevel:   "warning",
		ResourceID:   "vm-1",
		ResourceName: "web",
		Value:        90,
		Threshold:    80,
		StartTime:    time.Now(),
		LastSeen:     time.Now(),
	}
	finding, _ := store.AddFromAlert(alert)
	store.Resolve(finding.ID)

	enhanceCh := make(chan string, 1)
	bridge.scheduleEnhancement(finding.ID, 10*time.Millisecond, func(id string) error {
		enhanceCh <- id
		return nil
	})

	select {
	case <-enhanceCh:
		t.Fatalf("did not expect enhancement on inactive finding")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestAlertBridge_StatsAndStore(t *testing.T) {
	store := NewUnifiedStore(DefaultAlertToFindingConfig())
	bridge := NewAlertBridge(store, DefaultBridgeConfig())
	bridge.running = true
	bridge.pendingEnhancements["f1"] = time.AfterFunc(time.Second, func() {})

	stats := bridge.Stats()
	if !stats.Running {
		t.Fatalf("expected running stats")
	}
	if stats.PendingEnhancements != 1 {
		t.Fatalf("expected pending enhancements to be 1")
	}
	if bridge.GetUnifiedStore() != store {
		t.Fatalf("expected store to be returned")
	}
}
