package monitoring

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
)

func TestNotificationDeliveryAlertMessageNamesTheOutcome(t *testing.T) {
	cases := []struct {
		name   string
		health notifications.DeliveryHealth
		want   []string
	}{
		{
			name: "failed only",
			health: notifications.DeliveryHealth{
				Status: notifications.DeliveryDegraded,
				Failed: 4,
			},
			want: []string{"4 failed deliveries", "not reaching their destinations"},
		},
		{
			name: "dead lettered only",
			health: notifications.DeliveryHealth{
				Status:     notifications.DeliveryDegraded,
				DeadLetter: 1,
			},
			want: []string{"1 dead-lettered delivery", "gave up after repeated failures"},
		},
		{
			name: "both",
			health: notifications.DeliveryHealth{
				Status:     notifications.DeliveryDegraded,
				Failed:     2,
				DeadLetter: 3,
			},
			want: []string{"2 failed deliveries", "3 dead-lettered deliveries"},
		},
		{
			name:   "unavailable",
			health: notifications.DeliveryHealth{Status: notifications.DeliveryUnavailable},
			want:   []string{"cannot read the notification queue"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			message := notificationDeliveryAlertMessage(tc.health)
			for _, want := range tc.want {
				if !strings.Contains(message, want) {
					t.Errorf("message %q does not contain %q", message, want)
				}
			}
			// The operator reading this has not received a notification about
			// it, so the message must say where to go.
			if tc.health.Status == notifications.DeliveryDegraded &&
				!strings.Contains(message, "Alerts, Notifications") {
				t.Errorf("message %q does not point at the destinations surface", message)
			}
		})
	}
}

func TestNotificationDeliveryAlertMessageSingularises(t *testing.T) {
	message := notificationDeliveryAlertMessage(notifications.DeliveryHealth{
		Status: notifications.DeliveryDegraded,
		Failed: 1,
	})

	if !strings.Contains(message, "1 failed delivery ") {
		t.Errorf("expected a singular delivery in %q", message)
	}
}

func TestDeliveryHealthFingerprintIgnoresCountDrift(t *testing.T) {
	base := notifications.ClassifyQueueHealth(map[string]int{
		string(notifications.QueueStatusDLQ): 11,
	})
	drifted := notifications.ClassifyQueueHealth(map[string]int{
		string(notifications.QueueStatusDLQ): 14,
	})
	if deliveryHealthFingerprint(base) != deliveryHealthFingerprint(drifted) {
		t.Error("expected count drift within one failure class to keep the fingerprint stable")
	}

	withFailed := notifications.ClassifyQueueHealth(map[string]int{
		string(notifications.QueueStatusDLQ):    14,
		string(notifications.QueueStatusFailed): 1,
	})
	if deliveryHealthFingerprint(base) == deliveryHealthFingerprint(withFailed) {
		t.Error("expected a new failure class to change the fingerprint")
	}

	unavailable := notifications.UnavailableDeliveryHealth()
	if deliveryHealthFingerprint(base) == deliveryHealthFingerprint(unavailable) {
		t.Error("expected an unavailable queue to change the fingerprint")
	}
}

func TestEvaluateNotificationDeliveryThrottles(t *testing.T) {
	// The poll ticker runs on the polling cadence, which can be seconds, and
	// reading queue health costs a SQLite query. A nil notification manager
	// means the evaluation returns early, but the throttle stamp must still be
	// taken so the interval is honoured.
	m := &Monitor{}

	start := time.Now()
	m.evaluateNotificationDelivery(start)
	firstStamp := m.lastDeliveryHealthCheck
	if firstStamp.IsZero() {
		t.Fatal("expected the first evaluation to record a check time")
	}

	m.evaluateNotificationDelivery(start.Add(notificationDeliveryCheckInterval / 2))
	if !m.lastDeliveryHealthCheck.Equal(firstStamp) {
		t.Error("expected an evaluation inside the interval to be skipped")
	}

	due := start.Add(notificationDeliveryCheckInterval + time.Second)
	m.evaluateNotificationDelivery(due)
	if !m.lastDeliveryHealthCheck.Equal(due) {
		t.Error("expected an evaluation past the interval to run")
	}
}

func TestEvaluateNotificationDeliveryIsSafeWithoutAMonitor(t *testing.T) {
	var m *Monitor
	m.evaluateNotificationDelivery(time.Now())
}

// Hold the older snapshot between read and apply while a newer reconciliation
// tries to enter. Exercise both stale-clear and stale-raise failure modes
// without a database, queue workers, or notification destinations.
func TestProjectNotificationDeliveryHealthOrdersSnapshots(t *testing.T) {
	healthy := notifications.ClassifyQueueHealth(map[string]int{})
	failed := notifications.ClassifyQueueHealth(map[string]int{string(notifications.QueueStatusDLQ): 1})
	for _, tc := range []struct {
		name         string
		old, current notifications.DeliveryHealth
	}{
		{"new_failure_survives_old_clear", healthy, failed},
		{"dismissal_survives_old_failure", failed, healthy},
	} {
		t.Run(tc.name, func(t *testing.T) {
			manager := alerts.NewManagerWithDataDir(t.TempDir())
			t.Cleanup(manager.Stop)
			m := &Monitor{}
			read := make(chan struct{})
			release := make(chan struct{})
			oldDone := make(chan struct{})
			go func() {
				defer close(oldDone)
				m.projectNotificationDeliveryHealth(manager, func() notifications.DeliveryHealth {
					close(read)
					<-release
					return tc.old
				})
			}()
			<-read
			// This assertion is independent of scheduling: the snapshot must
			// already be protected before reading, not just when applying it.
			if m.deliveryHealthProjectionMu.TryLock() {
				m.deliveryHealthProjectionMu.Unlock()
				t.Error("health read is not protected by the projection lock")
			}
			newRead := make(chan struct{})
			newDone := make(chan struct{})
			go func() {
				defer close(newDone)
				m.projectNotificationDeliveryHealth(manager, func() notifications.DeliveryHealth {
					close(newRead)
					return tc.current
				})
			}()
			select {
			case <-newRead:
				// Ensure the newer state applies before releasing the stale
				// snapshot when checking the unprotected implementation.
				<-newDone
				t.Error("new health read overtook an unfinished projection")
			case <-time.After(25 * time.Millisecond):
			}
			close(release)
			<-oldDone
			<-newDone
			active := false
			for _, alert := range manager.GetActiveAlerts() {
				if alert.Type == alerts.NotificationDeliveryAlertType {
					active = true
				}
			}
			if active == tc.current.Healthy {
				t.Errorf("delivery warning active = %v, latest health healthy = %v", active, tc.current.Healthy)
			}
		})
	}
}

// Exercise the real monitor-to-alert-manager boundary, not just a fingerprint
// helper or hand-built SystemAlertInput. No queue, database or destination is
// needed: health snapshots are the input to this projection.
func TestProjectNotificationDeliveryHealthLifecycle(t *testing.T) {
	manager := alerts.NewManagerWithDataDir(t.TempDir())
	t.Cleanup(manager.Stop)
	config := manager.GetConfig()
	config.ActivationState = alerts.ActivationActive
	manager.UpdateConfig(config)
	manager.SetAlertCallback(func(*alerts.Alert) {})
	m := &Monitor{}

	healthy := notifications.ClassifyQueueHealth(nil)
	failed := notifications.ClassifyQueueHealth(map[string]int{string(notifications.QueueStatusFailed): 1})
	drifted := notifications.ClassifyQueueHealth(map[string]int{string(notifications.QueueStatusFailed): 4})
	mixed := notifications.ClassifyQueueHealth(map[string]int{
		string(notifications.QueueStatusFailed): 4,
		string(notifications.QueueStatusDLQ):    2,
	})
	var previous *alerts.Alert
	var lastIncidentStart time.Time
	for _, step := range []struct {
		name   string
		health notifications.DeliveryHealth
		notify bool
	}{
		{"initially healthy", healthy, false},
		{"first failure", failed, true},
		{"unchanged failure", failed, false},
		{"count drift", drifted, false},
		{"additional failure class", mixed, true},
		{"recovery", healthy, false},
		{"repeated recovery", healthy, false},
		{"same cause recurs", mixed, true},
	} {
		if !t.Run(step.name, func(t *testing.T) {
			m.projectNotificationDeliveryHealth(manager, func() notifications.DeliveryHealth { return step.health })
			active := manager.GetActiveAlerts()
			if step.health.Healthy {
				if len(active) != 0 {
					t.Fatalf("healthy projection left %d active alerts", len(active))
				}
				previous = nil
				return
			}
			if len(active) != 1 {
				t.Fatalf("active alerts = %d, want exactly one", len(active))
			}
			current := active[0]
			if current.ID != alerts.SystemAlertID(alerts.NotificationDeliveryAlertType) || current.ResourceID != "" || current.Level != alerts.AlertLevelWarning {
				t.Fatalf("unexpected delivery warning identity or level: %s, %s, %s", current.ID, current.ResourceID, current.Level)
			}
			if current.Message != notificationDeliveryAlertMessage(step.health) ||
				current.Metadata["failedDeliveries"] != step.health.Failed ||
				current.Metadata["deadLetterCount"] != step.health.DeadLetter ||
				current.Metadata["deliveryStatus"] != string(step.health.Status) {
				t.Fatal("warning presentation did not refresh to the current health")
			}
			// Dispatch stamps LastNotified under the manager lock before any
			// asynchronous callback. This tests silence without sleep-based
			// assertions about a callback that may not have run yet.
			if current.LastNotified == nil {
				t.Fatal("active delivery warning was never dispatched")
			}
			if previous != nil {
				if !current.StartTime.Equal(previous.StartTime) {
					t.Fatal("standing warning was replaced instead of refreshed")
				}
				notified := !current.LastNotified.Equal(*previous.LastNotified)
				if notified != step.notify {
					t.Fatalf("dispatch timestamp changed = %v, want %v", notified, step.notify)
				}
			} else if !current.StartTime.After(lastIncidentStart) {
				t.Fatal("recurrence did not start a new incident")
			}
			lastIncidentStart = current.StartTime
			previous = &current
		}) {
			return // Later steps depend on the preceding lifecycle state.
		}
	}
}

// Inactive delivery must not hide the system warning: it is the fallback
// visibility path when notifications cannot reach their destination.
func TestProjectNotificationDeliveryHealthRespectsActivation(t *testing.T) {
	for _, state := range []alerts.ActivationState{alerts.ActivationPending, alerts.ActivationSnoozed} {
		t.Run(string(state), func(t *testing.T) {
			manager := alerts.NewManagerWithDataDir(t.TempDir())
			t.Cleanup(manager.Stop)
			config := manager.GetConfig()
			config.ActivationState = state
			manager.UpdateConfig(config)
			// A callback is required to exercise policy rather than the
			// no-destination early return. No external destination is used.
			manager.SetAlertCallback(func(*alerts.Alert) {})
			m := &Monitor{}
			for _, counts := range []map[string]int{
				{string(notifications.QueueStatusFailed): 1},
				{string(notifications.QueueStatusFailed): 4},
				{string(notifications.QueueStatusDLQ): 2},
				nil,
				{string(notifications.QueueStatusFailed): 1},
			} {
				health := notifications.ClassifyQueueHealth(counts)
				m.projectNotificationDeliveryHealth(manager, func() notifications.DeliveryHealth { return health })
				active := manager.GetActiveAlerts()
				if health.Healthy {
					if len(active) != 0 {
						t.Fatal("recovery left a suppressed warning active")
					}
					continue
				}
				if len(active) != 1 || active[0].ID != alerts.SystemAlertID(alerts.NotificationDeliveryAlertType) {
					t.Fatal("inactive delivery hid or duplicated the system warning")
				}
				if active[0].Message != notificationDeliveryAlertMessage(health) {
					t.Fatal("suppressed warning did not refresh its presentation")
				}
				// LastNotified is stamped synchronously before an async
				// callback, so this does not depend on goroutine scheduling.
				if active[0].LastNotified != nil {
					t.Fatal("inactive delivery dispatched a system warning")
				}
			}
		})
	}
}

// Changing activation must suppress new dispatches without losing the standing
// warning, and must not suppress a fresh incident after delivery is re-enabled.
func TestProjectNotificationDeliveryHealthActivationTransitions(t *testing.T) {
	manager := alerts.NewManagerWithDataDir(t.TempDir())
	t.Cleanup(manager.Stop)
	manager.SetAlertCallback(func(*alerts.Alert) {})
	m := &Monitor{}
	project := func(counts map[string]int) []alerts.Alert {
		health := notifications.ClassifyQueueHealth(counts)
		m.projectNotificationDeliveryHealth(manager, func() notifications.DeliveryHealth { return health })
		return manager.GetActiveAlerts()
	}
	activate := func(state alerts.ActivationState) {
		config := manager.GetConfig()
		config.ActivationState = state
		manager.UpdateConfig(config)
	}

	activate(alerts.ActivationActive)
	first := project(map[string]int{string(notifications.QueueStatusFailed): 1})
	if len(first) != 1 || first[0].LastNotified == nil {
		t.Fatal("initial active failure was not dispatched")
	}
	activate(alerts.ActivationSnoozed)
	changed := project(map[string]int{string(notifications.QueueStatusDLQ): 2})
	if len(changed) != 1 || changed[0].LastNotified == nil || !changed[0].LastNotified.Equal(*first[0].LastNotified) {
		t.Fatal("snoozed diagnosis change hid the warning or dispatched it again")
	}
	if !changed[0].StartTime.Equal(first[0].StartTime) || changed[0].Metadata["deadLetterCount"] != 2 {
		t.Fatal("snoozed diagnosis did not refresh the standing incident")
	}
	if active := project(nil); len(active) != 0 {
		t.Fatal("recovery while snoozed left the warning active")
	}
	activate(alerts.ActivationActive)
	recurred := project(map[string]int{string(notifications.QueueStatusDLQ): 2})
	if len(recurred) != 1 || recurred[0].LastNotified == nil || !recurred[0].LastNotified.After(*first[0].LastNotified) {
		t.Fatal("re-enabled delivery did not dispatch the recurring failure")
	}
	if !recurred[0].StartTime.After(first[0].StartTime) {
		t.Fatal("recurrence reused the previous incident")
	}
}
