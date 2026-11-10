package api

import (
	"context"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/unified"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// alertBridgeRig is the production path from a real alert manager to the
// scoped Patrol run request: alert manager, unified adapter, alert bridge,
// the router's trigger wiring and a real trigger queue.
type alertBridgeRig struct {
	manager  *alerts.Manager
	triggers *ai.TriggerManager
	runs     chan ai.PatrolScope
}

func newAlertBridgeRig(t *testing.T, activation alerts.ActivationState, aiCfg *config.AIConfig) *alertBridgeRig {
	t.Helper()

	manager := alerts.NewManagerWithDataDir(t.TempDir())
	t.Cleanup(manager.Stop)
	alertCfg := manager.GetConfig()
	alertCfg.ActivationState = activation
	manager.UpdateConfig(alertCfg)
	// The monitor always installs its notification delivery callback.
	manager.SetAlertCallback(func(*alerts.Alert) {})

	triggerCfg := ai.DefaultTriggerManagerConfig()
	triggerCfg.MinResourceInterval = time.Millisecond
	triggers := ai.NewTriggerManager(triggerCfg)
	runs := make(chan ai.PatrolScope, 16)
	triggers.SetOnTrigger(func(_ context.Context, scope ai.PatrolScope) { runs <- scope })
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	triggers.Start(ctx)
	t.Cleanup(triggers.Stop)

	bridge := unified.NewAlertBridge(unified.NewUnifiedStore(unified.DefaultAlertToFindingConfig()), unified.DefaultBridgeConfig())
	bridge.SetAlertProvider(unified.NewAlertManagerAdapter(manager))
	alertBridgePatrolWiring{
		aiConfig:       func() *config.AIConfig { return aiCfg },
		triggerManager: func() *ai.TriggerManager { return triggers },
		runScoped:      func(scope ai.PatrolScope) { t.Errorf("unexpected direct scoped run: %+v", scope) },
	}.attach(bridge)
	bridge.Start()
	t.Cleanup(bridge.Stop)

	return &alertBridgeRig{manager: manager, triggers: triggers, runs: runs}
}

func (rig *alertBridgeRig) checkContainerHealth(health string) {
	rig.manager.CheckDockerHost(models.DockerHost{
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
	})
}

// expectRun waits for the trigger queue to hand a scoped run to Patrol.
func (rig *alertBridgeRig) expectRun(t *testing.T, reason ai.TriggerReason) ai.PatrolScope {
	t.Helper()
	select {
	case scope := <-rig.runs:
		if scope.Reason != reason {
			t.Fatalf("expected a %s run, got %s", reason, scope.Reason)
		}
		return scope
	case <-time.After(5 * time.Second):
		t.Fatalf("expected a %s run, got none", reason)
	}
	return ai.PatrolScope{}
}

// expectQueued waits for a run request to sit in the trigger queue. The queue
// spaces runs at least ten seconds apart, so later requests are checked there.
func (rig *alertBridgeRig) expectQueued(t *testing.T, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if rig.triggers.GetPendingCount() == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("expected %d queued run requests, got %d", want, rig.triggers.GetPendingCount())
}

func TestAlertBridgeWiring_RepeatCriticalAlertRequestsARunEachEpisode(t *testing.T) {
	rig := newAlertBridgeRig(t, alerts.ActivationActive, config.NewDefaultAIConfig())

	rig.checkContainerHealth("unhealthy")
	first := rig.expectRun(t, ai.TriggerReasonAlertFired)
	if first.AlertIdentifier == "" || first.AlertContext == nil || first.AlertContext.Level != "critical" {
		t.Fatalf("run request lost the alert payload: %+v", first)
	}
	if first.AlertContext.AlertType != "docker-container-health" {
		t.Fatalf("expected the health alert type, got %q", first.AlertContext.AlertType)
	}

	// The fault is fixed and the alert clears.
	rig.checkContainerHealth("healthy")
	rig.expectQueued(t, 1) // alert_cleared

	// The same container goes unhealthy again. Its alert identifier has been
	// seen before, and Patrol must still be asked to look.
	rig.checkContainerHealth("unhealthy")
	rig.expectQueued(t, 2) // alert_cleared + the second alert_fired
}

func TestAlertBridgeWiring_AlertTriggersSwitchedOffRequestNoRun(t *testing.T) {
	aiCfg := config.NewDefaultAIConfig()
	aiCfg.PatrolAlertTriggersEnabled = false
	rig := newAlertBridgeRig(t, alerts.ActivationActive, aiCfg)

	rig.checkContainerHealth("unhealthy")
	select {
	case scope := <-rig.runs:
		t.Fatalf("alert triggers are off, yet a %s run was requested", scope.Reason)
	case <-time.After(1500 * time.Millisecond):
	}
	if pending := rig.triggers.GetPendingCount(); pending != 0 {
		t.Fatalf("expected an empty trigger queue, got %d", pending)
	}
}
