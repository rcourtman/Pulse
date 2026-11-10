package alerts

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// steadyAlertRig drives the real manager through its storage evaluation, the
// way a poll does, and counts what reaches each kind of subscriber.
type steadyAlertRig struct {
	t         *testing.T
	m         *Manager
	delivered atomic.Int32
	fired     atomic.Int32
	flapping  atomic.Int32
}

func newSteadyAlertRig(t *testing.T, activation ActivationState, reminderMinutes int) *steadyAlertRig {
	t.Helper()
	rig := &steadyAlertRig{t: t, m: NewManagerWithDataDir(t.TempDir())}
	t.Cleanup(rig.m.Stop)

	cfg := rig.m.GetConfig()
	cfg.Enabled = true
	cfg.StorageDefault = HysteresisThreshold{Trigger: 85, Clear: 80}
	cfg.FlappingEnabled = true
	cfg.FlappingThreshold = 5
	cfg.FlappingWindowSeconds = 300
	cfg.FlappingCooldownMinutes = 15
	cfg.Schedule.Cooldown = reminderMinutes
	cfg.Schedule.MaxAlertsHour = 10
	rig.m.UpdateConfig(cfg)
	rig.m.mu.Lock()
	rig.m.config.TimeThresholds = map[string]int{}
	rig.m.config.ActivationState = activation
	// By default a new firing within 2% of the last one's value is held back
	// before it is an alert at all. That is a different rule, and it would
	// keep the alert below from firing five times.
	rig.m.config.MinimumDelta = 0
	rig.m.mu.Unlock()

	rig.m.SetAlertCallback(func(*Alert) { rig.delivered.Add(1) })
	rig.m.SubscribeAlertFiringCallback(func(*Alert) { rig.fired.Add(1) })
	rig.m.SetFlappingDetectedCallback(func(*Alert, string) { rig.flapping.Add(1) })
	return rig
}

func (r *steadyAlertRig) poll(usage float64) {
	r.t.Helper()
	r.m.CheckStorage(models.Storage{
		ID: "pve1-local", Name: "local", Node: "pve1", Instance: "pve1", Status: "available",
		Total: 1000, Used: int64(usage * 10), Free: int64(1000 - usage*10), Usage: usage,
	})
	// Callbacks run on their own goroutines. Let them land before the next
	// poll so counts are read in order.
	time.Sleep(20 * time.Millisecond)
}

func (r *steadyAlertRig) activeAlertID() string {
	r.t.Helper()
	active := r.m.GetActiveAlerts()
	if len(active) != 1 {
		r.t.Fatalf("active alerts = %d, want the one storage alert", len(active))
	}
	return active[0].ID
}

func (r *steadyAlertRig) diagnosis() AlertDeliveryDiagnosis {
	r.t.Helper()
	diagnosis, found := r.m.DiagnoseAlertDelivery(r.activeAlertID())
	if !found {
		r.t.Fatal("no delivery diagnosis for the active alert")
	}
	return diagnosis
}

// ageLastNotification moves the alert's last notification into the past, as if
// that many minutes had gone by between two polls.
func (r *steadyAlertRig) ageLastNotification(minutes int) {
	r.t.Helper()
	id := r.activeAlertID()
	r.m.mu.Lock()
	defer r.m.mu.Unlock()
	alert, ok := r.m.getActiveAlertNoLock(id)
	if !ok || alert == nil {
		r.t.Fatalf("alert %s is not active", id)
	}
	if alert.LastNotified == nil {
		// The last attempt was refused. The next poll tries again by itself.
		return
	}
	aged := alert.LastNotified.Add(-time.Duration(minutes) * time.Minute)
	alert.LastNotified = &aged
}

// A storage that crosses its threshold once and stays there has changed state
// once. Flapping is an alert that keeps firing and clearing, so a steady alert
// must never be declared flapping, however often Pulse tries to deliver it.
func TestSteadyAlertIsNotDeclaredFlapping(t *testing.T) {
	const polls = 12

	t.Run("delivery turned on", func(t *testing.T) {
		rig := newSteadyAlertRig(t, ActivationActive, 5)
		for i := 0; i < polls; i++ {
			rig.poll(90)
		}
		diagnosis := rig.diagnosis()
		if rig.flapping.Load() != 0 || diagnosis.FlappingActive || diagnosis.FlappingHistoryInWindow != 1 {
			t.Errorf("steady alert, delivery on: flapping callbacks=%d active=%v state changes counted=%d, want 0, false, 1",
				rig.flapping.Load(), diagnosis.FlappingActive, diagnosis.FlappingHistoryInWindow)
		}
		if rig.delivered.Load() != 1 {
			t.Errorf("steady alert, delivery on: %d notifications, want 1", rig.delivered.Load())
		}
	})

	t.Run("delivery not turned on yet", func(t *testing.T) {
		rig := newSteadyAlertRig(t, ActivationPending, 5)
		for i := 0; i < polls; i++ {
			rig.poll(90)
		}
		diagnosis := rig.diagnosis()
		if rig.flapping.Load() != 0 || diagnosis.FlappingActive || diagnosis.FlappingHistoryInWindow != 1 {
			t.Errorf("steady alert, delivery pending: flapping callbacks=%d active=%v state changes counted=%d, want 0, false, 1",
				rig.flapping.Load(), diagnosis.FlappingActive, diagnosis.FlappingHistoryInWindow)
		}
		if rig.delivered.Load() != 0 {
			t.Errorf("delivery pending: %d notifications sent, want 0", rig.delivered.Load())
		}
		if rig.fired.Load() == 0 {
			t.Error("delivery pending: firing subscribers never heard the alert")
		}

		// The operator turns delivery on. The alert that is already active
		// must reach them now, not after a flapping hold.
		rig.m.mu.Lock()
		rig.m.config.ActivationState = ActivationActive
		rig.m.mu.Unlock()
		rig.m.NotifyExistingAlert(rig.activeAlertID())
		time.Sleep(20 * time.Millisecond)
		if rig.delivered.Load() != 1 {
			t.Errorf("after delivery was turned on: %d notifications for the active alert, want 1", rig.delivered.Load())
		}
	})

	// A flapping hold also stops the alert's own evaluation, so an alert held
	// by mistake stayed on screen after the reading had dropped.
	t.Run("delivery not turned on yet, then the reading drops", func(t *testing.T) {
		rig := newSteadyAlertRig(t, ActivationPending, 5)
		for i := 0; i < polls; i++ {
			rig.poll(90)
		}
		rig.poll(50)
		if active := rig.m.GetActiveAlerts(); len(active) != 0 {
			t.Errorf("the storage is at 50%% and its alert is still active at %.0f%%", active[0].Value)
		}
	})

	t.Run("delivery turned on with one-minute reminders", func(t *testing.T) {
		rig := newSteadyAlertRig(t, ActivationActive, 1)
		const reminders = 6
		rig.poll(90)
		for i := 0; i < reminders; i++ {
			rig.ageLastNotification(1)
			rig.poll(90)
		}
		diagnosis := rig.diagnosis()
		if rig.flapping.Load() != 0 || diagnosis.FlappingActive {
			t.Errorf("reminders for a steady alert were counted as flapping: callbacks=%d active=%v state changes counted=%d",
				rig.flapping.Load(), diagnosis.FlappingActive, diagnosis.FlappingHistoryInWindow)
		}
		if got := rig.delivered.Load(); got != 1+reminders {
			t.Errorf("first notification plus %d reminders: %d delivered, want %d", reminders, got, 1+reminders)
		}
	})
}

// An alert that keeps firing and clearing is still caught: every new firing is
// a state change, and the configured number of them pauses notifications.
func TestAlertThatKeepsFiringAndClearingIsStillFlapping(t *testing.T) {
	for _, activation := range []ActivationState{ActivationActive, ActivationPending} {
		t.Run(string(activation), func(t *testing.T) {
			rig := newSteadyAlertRig(t, activation, 5)
			for firing := 1; firing <= 5; firing++ {
				rig.poll(90)
				if got := len(rig.m.GetActiveAlerts()); got != 1 {
					t.Fatalf("firing %d: %d active alerts, want 1", firing, got)
				}
				if firing < 5 {
					if rig.flapping.Load() != 0 {
						t.Fatalf("flapping declared after %d firings, want it at the fifth", firing)
					}
					rig.poll(50)
					if got := len(rig.m.GetActiveAlerts()); got != 0 {
						t.Fatalf("firing %d: the alert did not clear at 50%%", firing)
					}
				}
			}
			diagnosis := rig.diagnosis()
			if rig.flapping.Load() != 1 || !diagnosis.FlappingActive {
				t.Fatalf("five firings in the window: flapping callbacks=%d active=%v, want 1, true",
					rig.flapping.Load(), diagnosis.FlappingActive)
			}
			// With delivery off that is the reason the diagnosis gives first.
			if activation == ActivationActive && diagnosis.Reason != AlertDeliveryReasonFlapping {
				t.Errorf("delivery diagnosis reason = %q, want %q", diagnosis.Reason, AlertDeliveryReasonFlapping)
			}
			if activation == ActivationActive && rig.delivered.Load() != 4 {
				t.Errorf("%d notifications delivered, want 4 before the pause", rig.delivered.Load())
			}
		})
	}
}

// firesAgain stands for the alert clearing and firing again, which is what
// flapping detection counts. The manager builds a new Alert for a new firing,
// so tests that drive dispatchAlert with one Alert use this between
// dispatches to say that each one is a new firing and not a reminder.
func firesAgain(alert *Alert) *Alert {
	alert.flappingCounted = false
	return alert
}
