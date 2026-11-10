package alerts

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func firingCallbackTestHost(health string) models.DockerHost {
	return models.DockerHost{
		ID:          "docker01",
		DisplayName: "docker01",
		Hostname:    "docker01.lan",
		Containers: []models.DockerContainer{{
			ID:     "nextcloud-1",
			Name:   "nextcloud",
			State:  "running",
			Status: "Up 10 minutes",
			Health: health,
		}},
	}
}

// firingCallbackProbe records what reached notification delivery and what
// reached a firing subscriber.
type firingCallbackProbe struct {
	delivered chan string
	firing    chan string
}

func newFiringCallbackProbe(m *Manager) *firingCallbackProbe {
	probe := &firingCallbackProbe{
		delivered: make(chan string, 16),
		firing:    make(chan string, 16),
	}
	m.SetAlertCallback(func(alert *Alert) { probe.delivered <- alert.ID })
	m.SubscribeAlertFiringCallback(func(alert *Alert) { probe.firing <- alert.ID })
	return probe
}

func expectAlertCallback(t *testing.T, name string, ch <-chan string) string {
	t.Helper()
	select {
	case id := <-ch:
		return id
	case <-time.After(2 * time.Second):
		t.Fatalf("expected the %s callback to hear the alert", name)
	}
	return ""
}

func expectNoAlertCallback(t *testing.T, name string, ch <-chan string) {
	t.Helper()
	select {
	case id := <-ch:
		t.Fatalf("expected the %s callback to stay quiet, got alert %s", name, id)
	case <-time.After(150 * time.Millisecond):
	}
}

// A fresh install has alert delivery pending review. Its alerts still fire,
// and anything that follows the alert rather than the notification must hear
// them.
func TestAlertFiringCallbackHearsAlertsBeforeDeliveryIsTurnedOn(t *testing.T) {
	m := newTestManager(t)
	if state := m.GetConfig().ActivationState; state != ActivationPending {
		t.Fatalf("expected a fresh manager to start with delivery pending review, got %q", state)
	}
	probe := newFiringCallbackProbe(m)

	m.CheckDockerHost(firingCallbackTestHost("unhealthy"))

	expectAlertCallback(t, "firing", probe.firing)
	expectNoAlertCallback(t, "delivery", probe.delivered)
}

func TestAlertFiringCallbackHearsAlertsOnceWhenDeliveryIsOn(t *testing.T) {
	m := newTestManager(t)
	cfg := m.GetConfig()
	cfg.ActivationState = ActivationActive
	m.UpdateConfig(cfg)
	probe := newFiringCallbackProbe(m)

	m.CheckDockerHost(firingCallbackTestHost("unhealthy"))

	expectAlertCallback(t, "firing", probe.firing)
	expectAlertCallback(t, "delivery", probe.delivered)
	expectNoAlertCallback(t, "firing", probe.firing)
	expectNoAlertCallback(t, "delivery", probe.delivered)
}

// Acknowledging or snoozing an alert tells Pulse the operator has it in hand.
// That holds the alert back from firing subscribers exactly as it holds back
// the notification, whether or not delivery is turned on.
func TestAlertFiringCallbackSkipsAcknowledgedAndSnoozedAlerts(t *testing.T) {
	for _, activation := range []ActivationState{ActivationPending, ActivationActive} {
		t.Run(string(activation), func(t *testing.T) {
			m := newTestManager(t)
			cfg := m.GetConfig()
			cfg.ActivationState = activation
			m.UpdateConfig(cfg)
			probe := newFiringCallbackProbe(m)

			m.CheckDockerHost(firingCallbackTestHost("unhealthy"))
			alertID := expectAlertCallback(t, "firing", probe.firing)

			if err := m.AcknowledgeAlert(alertID, "tester"); err != nil {
				t.Fatalf("acknowledge: %v", err)
			}
			m.NotifyExistingAlert(alertID)
			expectNoAlertCallback(t, "firing", probe.firing)

			if err := m.UnacknowledgeAlert(alertID); err != nil {
				t.Fatalf("unacknowledge: %v", err)
			}
			if err := m.SnoozeAlert(alertID, "tester", time.Now().Add(time.Hour)); err != nil {
				t.Fatalf("snooze: %v", err)
			}
			// Drain anything the unacknowledge re-dispatched before the snooze.
			for len(probe.firing) > 0 {
				<-probe.firing
			}
			m.NotifyExistingAlert(alertID)
			expectNoAlertCallback(t, "firing", probe.firing)

			if err := m.UnsnoozeAlert(alertID, "tester"); err != nil {
				t.Fatalf("unsnooze: %v", err)
			}
			for len(probe.firing) > 0 {
				<-probe.firing
			}
			m.NotifyExistingAlert(alertID)
			expectAlertCallback(t, "firing", probe.firing)
		})
	}
}
