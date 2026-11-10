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

// The bridge hears an alert through the alert manager's firing subscription.
// It must hear the first firing, and the next one after the alert cleared,
// whether alert delivery is turned on or still pending, and however many
// polls the alert stays active for in between.
func TestAlertManagerToBridge_HearsEveryFiringWithDeliveryOnAndPending(t *testing.T) {
	const steadyPolls = 8 // more than the flapping threshold of 5

	for _, activation := range []alerts.ActivationState{alerts.ActivationActive, alerts.ActivationPending} {
		// A state alert: dispatched once when it fires.
		t.Run(string(activation)+"/container health", func(t *testing.T) {
			manager, _, recorder := newManagerBackedBridge(t, activation)

			manager.CheckDockerHost(dockerHostWithHealth("unhealthy"))
			first := recorder.expect(t, "alert_fired")
			for i := 0; i < steadyPolls; i++ {
				manager.CheckDockerHost(dockerHostWithHealth("unhealthy"))
			}
			recorder.expectNone(t)

			manager.CheckDockerHost(dockerHostWithHealth("healthy"))
			recorder.expect(t, "alert_cleared")

			manager.CheckDockerHost(dockerHostWithHealth("unhealthy"))
			if again := recorder.expect(t, "alert_fired"); again.AlertIdentifier != first.AlertIdentifier {
				t.Fatalf("the second firing came for %q, want %q", again.AlertIdentifier, first.AlertIdentifier)
			}
		})

		// A threshold alert: while delivery is pending it is dispatched again
		// on every poll, because nothing was delivered. Those repeats must
		// not be taken for flapping, which would cut the bridge off and stop
		// the alert from clearing.
		t.Run(string(activation)+"/storage usage", func(t *testing.T) {
			manager, _, recorder := newManagerBackedBridge(t, activation)
			cfg := manager.GetConfig()
			cfg.Enabled = true
			cfg.StorageDefault = alerts.HysteresisThreshold{Trigger: 85, Clear: 80}
			cfg.TimeThresholds = map[string]int{"storage": 0, "guest": 0, "node": 0, "agent": 0, "pbs": 0}
			cfg.MetricTimeThresholds = map[string]map[string]int{"all": {"usage": 0}, "storage": {"usage": 0}}
			manager.UpdateConfig(cfg)
			poll := func(usage float64) {
				manager.CheckStorage(models.Storage{
					ID: "pve1-local", Name: "local", Node: "pve1", Instance: "pve1", Status: "available",
					Total: 1000, Used: int64(usage * 10), Free: int64(1000 - usage*10), Usage: usage,
				})
			}

			poll(90)
			first := recorder.expect(t, "alert_fired")
			for i := 0; i < steadyPolls; i++ {
				poll(90)
			}
			recorder.expectNone(t)

			poll(50)
			recorder.expect(t, "alert_cleared")

			// A new firing needs a reading at least 2% away from the last one.
			poll(95)
			if again := recorder.expect(t, "alert_fired"); again.AlertIdentifier != first.AlertIdentifier {
				t.Fatalf("the second firing came for %q, want %q", again.AlertIdentifier, first.AlertIdentifier)
			}
		})
	}
}
