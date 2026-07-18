package alerts

import (
	"testing"
	"time"
)

func TestNotificationActivationDoesNotSuppressDetectionOrActiveReadModel(t *testing.T) {
	for _, activationState := range []ActivationState{
		ActivationPending,
		ActivationSnoozed,
	} {
		t.Run(string(activationState), func(t *testing.T) {
			m := newTestManager(t)
			delivered := make(chan *Alert, 1)
			m.SetAlertCallback(func(alert *Alert) {
				delivered <- alert
			})

			m.mu.Lock()
			m.config.Enabled = true
			m.config.ActivationState = activationState
			m.config.TimeThresholds = map[string]int{}
			m.config.SuppressionWindow = 0
			m.config.MinimumDelta = 0
			m.mu.Unlock()

			m.checkMetric(
				"boundary-resource",
				"Boundary Resource",
				"node-1",
				"instance-1",
				"guest",
				"cpu",
				95,
				&HysteresisThreshold{Trigger: 80, Clear: 75},
				nil,
			)

			active := m.GetActiveAlerts()
			if len(active) != 1 {
				t.Fatalf(
					"activation state %q produced %d active alerts, want 1",
					activationState,
					len(active),
				)
			}
			if active[0].ResourceID != "boundary-resource" {
				t.Fatalf(
					"activation state %q exposed resource %q, want boundary-resource",
					activationState,
					active[0].ResourceID,
				)
			}

			select {
			case alert := <-delivered:
				t.Fatalf(
					"activation state %q delivered external notification for %q",
					activationState,
					alert.ID,
				)
			case <-time.After(50 * time.Millisecond):
			}
		})
	}
}
