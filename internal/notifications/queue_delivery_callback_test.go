package notifications

import (
	"errors"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
)

// A health callback can resolve an incident and acquire its delivery gate.
// Every send-failure transition must release that gate and finish its audit
// before calling the owner, just like policy cancellation already does.
func TestQueuedFailureCallbackSeesCommittedAuditAndReleasedDeliveryGate(t *testing.T) {
	for _, tc := range []struct {
		name     string
		err      error
		attempts int
		max      int
		status   NotificationQueueStatus
		failures int
	}{
		{"configuration", errors.New("invalid configuration: synthetic blocked destination"), 0, 3, QueueStatusDLQ, 1},
		{"transient", errors.New("connection refused: synthetic unavailable destination"), 0, 3, QueueStatusPending, 0},
		{"exhausted", errors.New("connection refused: synthetic unavailable destination"), 2, 3, QueueStatusDLQ, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Now()
			q := newQuietReplayQueue(t, t.TempDir(), &now)
			n := &QueuedNotification{
				ID: "callback-order", Type: "webhook", Status: QueueStatusPending,
				Config: []byte(`{}`), Attempts: tc.attempts, MaxAttempts: tc.max,
				Alerts: []*alerts.Alert{{ID: "incident"}},
				Links: []operationaltrust.NotificationLink{{DestinationID: "ops", OperationalRecordID: "incident",
					TransitionID: "firing", LifecycleState: operationaltrust.OperationalOpen, CauseKey: "cpu"}},
			}
			if err := q.Enqueue(n); err != nil {
				t.Fatal(err)
			}
			q.SetProcessor(func(*QueuedNotification) error { return tc.err })
			callbacks := 0
			q.SetDeliveryHealthChangedCallback(func() {
				callbacks++
				stats, err := q.GetTelemetryStats(time.Time{})
				if err != nil {
					t.Error(err)
					return
				}
				if stats.Attempts != 1 || stats.Deliveries != 0 || stats.Failures != tc.failures {
					t.Errorf("callback saw incomplete audit: %+v", stats)
				}
				if tc.name == "configuration" && stats.FailureClasses.Configuration != 1 ||
					tc.name == "exhausted" && stats.FailureClasses.Connectivity != 1 {
					t.Errorf("callback lost the terminal failure class: %+v", stats)
				}
				q.deliveryGateMu.Lock()
				gate := q.deliveryGates["incident"]
				q.deliveryGateMu.Unlock()
				if gate != nil {
					// Fail rather than hang when this regression is deliberately
					// restored. A callback must be free to take the writer gate.
					if !gate.mu.TryLock() {
						t.Error("callback retained the firing alert delivery gate")
						return
					}
					gate.mu.Unlock()
				}
				release := q.acquireAlertDeliveryGates([]string{"incident"}, true)
				defer release()
				rows, err := q.GetQueueStats()
				if err != nil || rows[string(tc.status)] != 1 {
					t.Errorf("callback did not see committed %s: %v, %v", tc.status, rows, err)
				}
				links, err := q.getNotificationLinks(n.ID)
				if err != nil || len(links) != 1 {
					t.Errorf("callback lost operational links: %v, %v", links, err)
					return
				}
				wantState := operationaltrust.NotificationDeadLetter
				if tc.status == QueueStatusPending {
					wantState = operationaltrust.NotificationRetrying
				}
				if links[0].DeliveryState != wantState || links[0].AttemptedAt == nil {
					t.Errorf("callback saw incomplete attempt linkage: %+v", links[0])
				}
			})
			q.processNotification(n)
			if callbacks != 1 {
				t.Fatalf("health callbacks=%d, want 1", callbacks)
			}
			if n.Status != tc.status || n.Attempts != tc.attempts+1 {
				t.Fatalf("status=%s attempts=%d, want %s/%d", n.Status, n.Attempts, tc.status, tc.attempts+1)
			}
		})
	}
}
