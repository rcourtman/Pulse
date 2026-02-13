package alerts

import (
	"testing"
	"time"
)

func TestSetLicenseCheckerStoresChecker(t *testing.T) {
	m := newTestManager(t)

	m.SetLicenseChecker(func(feature string) bool {
		return feature == "updates"
	})

	m.mu.RLock()
	checker := m.hasProFeature
	m.mu.RUnlock()

	if checker == nil {
		t.Fatal("expected license checker to be stored")
	}
	if !checker("updates") {
		t.Fatal("expected checker to allow configured feature")
	}
	if checker("other") {
		t.Fatal("expected checker to reject other features")
	}
}

func TestAcknowledgeAlertInvokesCallback(t *testing.T) {
	m := newTestManager(t)
	alertID := "ack-callback-alert"

	m.mu.Lock()
	m.activeAlerts[alertID] = &Alert{ID: alertID, Type: "cpu", ResourceID: "guest-1"}
	m.mu.Unlock()

	done := make(chan struct{})
	var gotAlert *Alert
	var gotUser string
	m.SetAcknowledgedCallback(func(alert *Alert, user string) {
		gotAlert = alert
		gotUser = user
		close(done)
	})

	if err := m.AcknowledgeAlert(alertID, "alice"); err != nil {
		t.Fatalf("AcknowledgeAlert returned error: %v", err)
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("acknowledged callback not invoked")
	}

	if gotAlert == nil {
		t.Fatal("expected callback alert payload")
	}
	if gotUser != "alice" {
		t.Fatalf("expected callback user alice, got %q", gotUser)
	}
	if !gotAlert.Acknowledged || gotAlert.AckTime == nil || gotAlert.AckUser != "alice" {
		t.Fatalf("callback alert did not include acknowledged state: %+v", gotAlert)
	}

	m.mu.RLock()
	stored := m.activeAlerts[alertID]
	m.mu.RUnlock()
	if gotAlert == stored {
		t.Fatal("expected callback to receive a cloned alert")
	}
}

func TestUnacknowledgeAlertInvokesCallback(t *testing.T) {
	m := newTestManager(t)
	alertID := "unack-callback-alert"
	now := time.Now().Add(-time.Minute)

	m.mu.Lock()
	m.activeAlerts[alertID] = &Alert{
		ID:           alertID,
		Type:         "cpu",
		ResourceID:   "guest-1",
		Acknowledged: true,
		AckUser:      "alice",
		AckTime:      &now,
	}
	m.ackState[alertID] = ackRecord{acknowledged: true, user: "alice", time: now}
	m.mu.Unlock()

	done := make(chan struct{})
	var gotAlert *Alert
	var gotUser string
	m.SetUnacknowledgedCallback(func(alert *Alert, user string) {
		gotAlert = alert
		gotUser = user
		close(done)
	})

	if err := m.UnacknowledgeAlert(alertID); err != nil {
		t.Fatalf("UnacknowledgeAlert returned error: %v", err)
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("unacknowledged callback not invoked")
	}

	if gotAlert == nil {
		t.Fatal("expected callback alert payload")
	}
	if gotUser != "" {
		t.Fatalf("expected empty callback user, got %q", gotUser)
	}
	if gotAlert.Acknowledged || gotAlert.AckTime != nil || gotAlert.AckUser != "" {
		t.Fatalf("callback alert did not include unacknowledged state: %+v", gotAlert)
	}

	m.mu.RLock()
	stored := m.activeAlerts[alertID]
	m.mu.RUnlock()
	if gotAlert == stored {
		t.Fatal("expected callback to receive a cloned alert")
	}
}

func TestSafeCallAcknowledgedCallbackRecoversPanic(t *testing.T) {
	m := newTestManager(t)

	done := make(chan struct{})
	m.SetAcknowledgedCallback(func(alert *Alert, user string) {
		defer close(done)
		panic("ack callback panic")
	})

	m.safeCallAcknowledgedCallback(&Alert{ID: "ack-panic-alert"}, "alice")

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("panicing acknowledged callback was not invoked")
	}
}

func TestSafeCallUnacknowledgedCallbackRecoversPanic(t *testing.T) {
	m := newTestManager(t)

	done := make(chan struct{})
	m.SetUnacknowledgedCallback(func(alert *Alert, user string) {
		defer close(done)
		panic("unack callback panic")
	})

	m.safeCallUnacknowledgedCallback(&Alert{ID: "unack-panic-alert"}, "")

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("panicing unacknowledged callback was not invoked")
	}
}

func TestCheckMetricInvokesAICallbackWhenNotificationsSuppressed(t *testing.T) {
	m := newTestManager(t)

	aiDone := make(chan *Alert, 1)
	notifyDone := make(chan struct{}, 1)

	m.SetAlertForAICallback(func(alert *Alert) {
		aiDone <- alert
	})
	m.SetAlertCallback(func(alert *Alert) {
		notifyDone <- struct{}{}
	})

	m.mu.Lock()
	m.config.ActivationState = ActivationPending
	m.config.TimeThreshold = 0
	m.config.TimeThresholds = map[string]int{}
	m.config.SuppressionWindow = 0
	m.config.MinimumDelta = 0
	m.mu.Unlock()

	m.checkMetric("ai-resource", "AI Resource", "node-1", "node-1/qemu/100", "guest", "cpu", 95, &HysteresisThreshold{Trigger: 80, Clear: 75}, nil)

	var aiAlert *Alert
	select {
	case aiAlert = <-aiDone:
	case <-time.After(time.Second):
		t.Fatal("AI callback was not invoked")
	}

	select {
	case <-notifyDone:
		t.Fatal("expected regular notification callback to be suppressed while activation is pending")
	case <-time.After(100 * time.Millisecond):
	}

	m.mu.RLock()
	stored := m.activeAlerts["ai-resource-cpu"]
	m.mu.RUnlock()

	if stored == nil {
		t.Fatal("expected metric alert to be stored")
	}
	if aiAlert == stored {
		t.Fatal("expected AI callback to receive a cloned alert")
	}
	if aiAlert.ID != "ai-resource-cpu" {
		t.Fatalf("expected AI callback alert ID ai-resource-cpu, got %q", aiAlert.ID)
	}
}

func TestOnAlertHistoryRegistersCallback(t *testing.T) {
	m := newTestManager(t)

	done := make(chan string, 1)
	m.OnAlertHistory(func(alert Alert) {
		done <- alert.ID
	})

	m.historyManager.AddAlert(Alert{ID: "history-wrapper-callback"})

	select {
	case got := <-done:
		if got != "history-wrapper-callback" {
			t.Fatalf("expected callback ID history-wrapper-callback, got %q", got)
		}
	case <-time.After(time.Second):
		t.Fatal("history callback was not invoked")
	}
}

func TestMigrateActivationState(t *testing.T) {
	t.Run("existing install with active alerts migrates to active", func(t *testing.T) {
		m := &Manager{
			activeAlerts: map[string]*Alert{
				"existing-alert": {ID: "existing-alert"},
			},
		}
		cfg := AlertConfig{Overrides: map[string]ThresholdConfig{}}

		m.migrateActivationState(&cfg)

		if cfg.ActivationState != ActivationActive {
			t.Fatalf("expected activation state %q, got %q", ActivationActive, cfg.ActivationState)
		}
		if cfg.ActivationTime == nil {
			t.Fatal("expected activation time to be set during migration")
		}
	})

	t.Run("existing install with overrides migrates to active", func(t *testing.T) {
		m := &Manager{}
		cfg := AlertConfig{
			Overrides: map[string]ThresholdConfig{
				"qemu-100": {},
			},
		}

		m.migrateActivationState(&cfg)

		if cfg.ActivationState != ActivationActive {
			t.Fatalf("expected activation state %q, got %q", ActivationActive, cfg.ActivationState)
		}
		if cfg.ActivationTime == nil {
			t.Fatal("expected activation time to be set during migration")
		}
	})

	t.Run("new install defaults to pending", func(t *testing.T) {
		m := &Manager{}
		cfg := AlertConfig{}

		m.migrateActivationState(&cfg)

		if cfg.ActivationState != ActivationPending {
			t.Fatalf("expected activation state %q, got %q", ActivationPending, cfg.ActivationState)
		}
		if cfg.ActivationTime != nil {
			t.Fatal("expected activation time to remain nil for pending state")
		}
	})

	t.Run("pre-set state is preserved", func(t *testing.T) {
		m := &Manager{
			activeAlerts: map[string]*Alert{
				"existing-alert": {ID: "existing-alert"},
			},
		}
		now := time.Now().Add(-time.Hour)
		cfg := AlertConfig{
			ActivationState: ActivationSnoozed,
			ActivationTime:  &now,
		}

		m.migrateActivationState(&cfg)

		if cfg.ActivationState != ActivationSnoozed {
			t.Fatalf("expected pre-set activation state to be preserved, got %q", cfg.ActivationState)
		}
		if cfg.ActivationTime == nil || !cfg.ActivationTime.Equal(now) {
			t.Fatal("expected pre-set activation time to be preserved")
		}
	})
}

func TestValidateQuietHoursTimezone(t *testing.T) {
	t.Run("invalid timezone disables quiet hours", func(t *testing.T) {
		cfg := AlertConfig{
			Schedule: ScheduleConfig{
				QuietHours: QuietHours{
					Enabled:  true,
					Timezone: "Invalid/Timezone",
				},
			},
		}

		validateQuietHoursTimezone(&cfg)

		if cfg.Schedule.QuietHours.Enabled {
			t.Fatal("expected quiet hours to be disabled for invalid timezone")
		}
	})

	t.Run("valid timezone keeps quiet hours enabled", func(t *testing.T) {
		cfg := AlertConfig{
			Schedule: ScheduleConfig{
				QuietHours: QuietHours{
					Enabled:  true,
					Timezone: "America/New_York",
				},
			},
		}

		validateQuietHoursTimezone(&cfg)

		if !cfg.Schedule.QuietHours.Enabled {
			t.Fatal("expected quiet hours to remain enabled for valid timezone")
		}
	})

	t.Run("empty timezone leaves quiet hours enabled", func(t *testing.T) {
		cfg := AlertConfig{
			Schedule: ScheduleConfig{
				QuietHours: QuietHours{
					Enabled:  true,
					Timezone: "",
				},
			},
		}

		validateQuietHoursTimezone(&cfg)

		if !cfg.Schedule.QuietHours.Enabled {
			t.Fatal("expected quiet hours to remain enabled when timezone is empty")
		}
	})

	t.Run("disabled quiet hours stay disabled", func(t *testing.T) {
		cfg := AlertConfig{
			Schedule: ScheduleConfig{
				QuietHours: QuietHours{
					Enabled:  false,
					Timezone: "Invalid/Timezone",
				},
			},
		}

		validateQuietHoursTimezone(&cfg)

		if cfg.Schedule.QuietHours.Enabled {
			t.Fatal("expected quiet hours to remain disabled")
		}
	})
}
