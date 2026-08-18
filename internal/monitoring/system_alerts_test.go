package monitoring

import (
	"strings"
	"testing"
	"time"

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
