package unified

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// newManagerBackedBridge wires a real alert manager through the production
// adapter into a started bridge, the way the router does.
func newManagerBackedBridge(t *testing.T, activation alerts.ActivationState) (*alerts.Manager, *AlertBridge, *triggerRecorder) {
	t.Helper()

	manager := alerts.NewManagerWithDataDir(t.TempDir())
	t.Cleanup(manager.Stop)
	cfg := manager.GetConfig()
	cfg.ActivationState = activation
	manager.UpdateConfig(cfg)
	// The monitor always installs its notification delivery callback.
	manager.SetAlertCallback(func(*alerts.Alert) {})

	bridge := NewAlertBridge(NewUnifiedStore(DefaultAlertToFindingConfig()), DefaultBridgeConfig())
	bridge.SetAlertProvider(NewAlertManagerAdapter(manager))
	recorder := newTriggerRecorder()
	bridge.SetPatrolTrigger(recorder.record)
	bridge.Start()
	t.Cleanup(bridge.Stop)

	return manager, bridge, recorder
}

func dockerHostWithHealth(health string) models.DockerHost {
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

// A container health alert keeps one identifier for the life of the
// container. Each time it goes unhealthy again Patrol must be woken again,
// not only the first time Pulse ever saw the alert.
func TestAlertManagerToBridge_RepeatHealthAlertWakesPatrolAgain(t *testing.T) {
	manager, _, recorder := newManagerBackedBridge(t, alerts.ActivationActive)

	manager.CheckDockerHost(dockerHostWithHealth("unhealthy"))
	first := recorder.expect(t, "alert_fired")
	if first.AlertLevel != "critical" || first.AlertType != "docker-container-health" {
		t.Fatalf("unexpected first trigger: %+v", first)
	}

	// Still unhealthy on the next poll: same episode, no second run.
	manager.CheckDockerHost(dockerHostWithHealth("unhealthy"))
	recorder.expectNone(t)

	manager.CheckDockerHost(dockerHostWithHealth("healthy"))
	recorder.expect(t, "alert_cleared")

	manager.CheckDockerHost(dockerHostWithHealth("unhealthy"))
	second := recorder.expect(t, "alert_fired")
	if second.AlertIdentifier != first.AlertIdentifier {
		t.Fatalf("expected the same alert identifier on the repeat, got %q then %q",
			first.AlertIdentifier, second.AlertIdentifier)
	}
}

// A warning from a real alert manager does not wake Patrol under the default
// minimum severity. Once it has stayed active for the delay, the sustained
// trigger asks Patrol to look, once.
func TestAlertManagerToBridge_WarningThatStaysActiveWakesPatrolOnce(t *testing.T) {
	manager, bridge, recorder := newManagerBackedBridge(t, alerts.ActivationActive)
	bridge.SetPatrolTriggerPolicy(func(_, alertLevel string) bool { return alertLevel == "critical" })
	bridge.SetSustainedTriggerPolicy(func(PatrolTriggerEvent, time.Time) SustainedTriggerDecision {
		return SustainedTriggerRun
	})

	// Any health state other than healthy, starting or unhealthy is a warning.
	manager.CheckDockerHost(dockerHostWithHealth("degraded"))
	recorder.expectNone(t)

	bridge.checkSustainedAlerts()
	recorder.expectNone(t)

	fired := time.Now()
	bridge.mu.Lock()
	bridge.now = func() time.Time { return fired.Add(sustainedAlertDelay + time.Minute) }
	bridge.mu.Unlock()
	bridge.checkSustainedAlerts()
	event := recorder.expect(t, "alert_fired")
	if !event.Sustained || event.AlertLevel != "warning" || event.AlertType != "docker-container-health" {
		t.Fatalf("unexpected sustained trigger: %+v", event)
	}

	bridge.checkSustainedAlerts()
	recorder.expectNone(t)
}
