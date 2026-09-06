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
