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

type alertBridgeSnapshotProvider struct {
	snapshot models.StateSnapshot
}

func (p alertBridgeSnapshotProvider) ReadSnapshot() models.StateSnapshot { return p.snapshot }

// A host filesystem alert reaches the bridge with the unified store's type
// for it, "agent-disk", which is not a Patrol resource type. The run request
// the wiring builds must still resolve to the host that owns the filesystem.
// On the Patrol fault set these runs ended "scope resolution failed".
func TestAlertBridgeWiring_HostFilesystemAlertRunRequestsResolve(t *testing.T) {
	const agentID = "66cd990e-2def-40c7-a9e4-c450a9e450de"
	diskAlert := models.Alert{
		ID:         "agent:" + agentID + "/disk:mnt-nvr::metric-threshold:disk",
		ResourceID: "agent:" + agentID + "/disk:mnt-nvr",
		Type:       "disk", Level: "warning", Value: 92.3, Threshold: 90,
		Metadata: map[string]interface{}{"mountpoint": "/mnt/nvr"},
	}
	hosts := []models.Host{{ID: agentID, Hostname: "docker01", MachineID: "machine-docker01"}}

	for _, tc := range []struct {
		reason   string
		snapshot models.StateSnapshot
	}{
		{"alert_fired", models.StateSnapshot{Hosts: hosts, ActiveAlerts: []models.Alert{diskAlert}}},
		{"alert_cleared", models.StateSnapshot{Hosts: hosts,
			RecentlyResolved: []models.ResolvedAlert{{Alert: diskAlert, ResolvedTime: time.Now()}}}},
	} {
		t.Run(tc.reason, func(t *testing.T) {
			patrol := ai.NewPatrolService(
				ai.NewService(config.NewConfigPersistence(t.TempDir()), nil),
				alertBridgeSnapshotProvider{snapshot: tc.snapshot},
			)
			var requested []ai.PatrolScope
			var cleared []string
			wiring := alertBridgePatrolWiring{
				runScoped: func(scope ai.PatrolScope) { requested = append(requested, scope) },
				alertCleared: func(alertIdentifier, alertResourceID, alertType string) int {
					cleared = append(cleared, alertIdentifier+"|"+alertResourceID+"|"+alertType)
					return 0
				},
			}

			wiring.trigger(unified.PatrolTriggerEvent{
				Reason:          tc.reason,
				ResourceID:      diskAlert.ResourceID,
				ResourceType:    "agent-disk",
				AlertType:       diskAlert.Type,
				AlertIdentifier: diskAlert.ID,
				AlertLevel:      diskAlert.Level,
				Value:           diskAlert.Value,
				Threshold:       diskAlert.Threshold,
			})

			if len(requested) != 1 {
				t.Fatalf("expected one run request, got %d", len(requested))
			}
			_, resolution := patrol.ResolvePatrolScope(requested[0])
			if len(resolution.UnmatchedResourceIDs)+len(resolution.AmbiguousResourceIDs) != 0 || len(resolution.EffectiveResourceIDs) == 0 {
				t.Fatalf("the run request does not resolve to the host: %+v", resolution)
			}

			wantCleared := 0
			if tc.reason == "alert_cleared" {
				wantCleared = 1
			}
			if len(cleared) != wantCleared {
				t.Fatalf("expected %d cleared-alert notices, got %v", wantCleared, cleared)
			}
			if wantCleared == 1 && cleared[0] != diskAlert.ID+"|"+diskAlert.ResourceID+"|disk" {
				t.Fatalf("cleared-alert notice lost the alert: %q", cleared[0])
			}
		})
	}
}

// The sustained trigger is for installs where Patrol investigates findings.
// Watch only and free installs get no runs from it, and it follows the alert
// trigger switch and the alert-type allowlist. The minimum severity setting is
// not consulted: this rule exists for the warnings below it.
func TestAlertBridgeWiring_SustainedAlertPolicy(t *testing.T) {
	since := time.Now().Add(-time.Hour)
	event := unified.PatrolTriggerEvent{Reason: "alert_fired", AlertType: "disk", AlertLevel: "warning", AlertIdentifier: "disk-alert"}

	for _, tc := range []struct {
		name         string
		mutate       func(cfg *config.AIConfig)
		investigates bool
		lookedAt     bool
		want         unified.SustainedTriggerDecision
	}{
		{name: "paid mode, critical-only floor, warning alert", investigates: true, want: unified.SustainedTriggerRun},
		{name: "watch only or free", investigates: false, want: unified.SustainedTriggerWait},
		{name: "alert triggers switched off", investigates: true,
			mutate: func(cfg *config.AIConfig) { cfg.PatrolAlertTriggersEnabled = false }, want: unified.SustainedTriggerWait},
		{name: "alert type not on the allowlist", investigates: true,
			mutate: func(cfg *config.AIConfig) { cfg.PatrolAlertTriggerTypes = []string{"cpu"} }, want: unified.SustainedTriggerWait},
		{name: "alert type on the allowlist", investigates: true,
			mutate: func(cfg *config.AIConfig) { cfg.PatrolAlertTriggerTypes = []string{"disk"} }, want: unified.SustainedTriggerRun},
		{name: "a run already looked at this episode", investigates: true, lookedAt: true, want: unified.SustainedTriggerSkip},
	} {
		t.Run(tc.name, func(t *testing.T) {
			aiCfg := config.NewDefaultAIConfig()
			if tc.mutate != nil {
				tc.mutate(aiCfg)
			}
			wiring := alertBridgePatrolWiring{
				aiConfig:     func() *config.AIConfig { return aiCfg },
				investigates: func() bool { return tc.investigates },
				lookedAtSince: func(alertIdentifier string, got time.Time) bool {
					if alertIdentifier != "disk-alert" || !got.Equal(since) {
						t.Errorf("run history asked about %q since %v", alertIdentifier, got)
					}
					return tc.lookedAt
				},
			}
			if got := wiring.acceptsSustainedAlert(event, since); got != tc.want {
				t.Fatalf("decision = %v, want %v", got, tc.want)
			}
		})
	}

	// Wiring that cannot tell Patrol's mode never runs the sustained trigger.
	if got := (alertBridgePatrolWiring{}).acceptsSustainedAlert(event, since); got != unified.SustainedTriggerWait {
		t.Fatalf("unconfigured wiring decided %v", got)
	}
}

func TestAlertBridgeWiring_SustainedAlertRequestsAMarkedLowerPriorityRun(t *testing.T) {
	var requested []ai.PatrolScope
	wiring := alertBridgePatrolWiring{runScoped: func(scope ai.PatrolScope) { requested = append(requested, scope) }}

	base := unified.PatrolTriggerEvent{
		Reason: "alert_fired", ResourceID: "agent:host-1/disk:data", ResourceType: "agent-disk",
		AlertType: "disk", AlertLevel: "warning", AlertIdentifier: "disk-alert", Value: 92, Threshold: 90,
	}
	sustained := base
	sustained.Sustained = true
	sustained.ActiveSince = time.Now().Add(-11 * time.Minute)
	wiring.trigger(base)
	wiring.trigger(sustained)

	if len(requested) != 2 {
		t.Fatalf("expected two run requests, got %d", len(requested))
	}
	fresh, stayed := requested[0], requested[1]
	if fresh.AlertSustained || !stayed.AlertSustained {
		t.Fatalf("only the sustained request must be marked: fresh=%v stayed=%v", fresh.AlertSustained, stayed.AlertSustained)
	}
	if stayed.Reason != ai.TriggerReasonAlertFired || stayed.AlertIdentifier != "disk-alert" {
		t.Fatalf("sustained request lost its alert: %+v", stayed)
	}
	if stayed.Priority >= fresh.Priority {
		t.Fatalf("a sustained alert must queue behind a freshly fired one: %d vs %d", stayed.Priority, fresh.Priority)
	}
}
