package api

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

// A full monitor reload (a node is added or registers itself, the polling
// interval changes) stops the monitor and builds a new one with a new alert
// manager. Patrol is started once and bound its alert bridge to the alert
// manager of that moment, so after a reload the bridge kept listening to a
// manager that had stopped, and no alert woke Patrol again until a restart.
func TestPatrolAlertBridgeFollowsTheAlertManagerAfterAMonitorReload(t *testing.T) {
	newMonitor := func() (*monitoring.Monitor, func()) {
		t.Helper()
		monitor, err := monitoring.New(&config.Config{DataPath: t.TempDir()})
		if err != nil {
			t.Fatal(err)
		}
		stopped := false
		stop := func() {
			if !stopped {
				stopped = true
				monitor.Stop()
			}
		}
		t.Cleanup(stop)
		return monitor, stop
	}
	unhealthyContainer := func(manager *alerts.Manager, container string) {
		manager.CheckDockerHost(models.DockerHost{
			ID: "docker01", DisplayName: "docker01", Hostname: "docker01.lan",
			Containers: []models.DockerContainer{{ID: container + "-1", Name: container, State: "running", Status: "Up 10 minutes", Health: "unhealthy"}},
		})
	}

	first, stopFirst := newMonitor()
	router := newTestRouter(t, &config.Config{DataPath: t.TempDir()}, first, nil, nil, func() error { return nil }, "test")
	aiService := ai.NewService(config.NewConfigPersistence(t.TempDir()), nil)

	// What starting Patrol builds for the organisation.
	bridge := router.startUnifiedAlertBridge("default", aiService, first.GetAlertManager())
	if bridge == nil {
		t.Fatal("no alert bridge was started")
	}
	t.Cleanup(bridge.Stop)
	store := router.aiSettingsHandler.GetUnifiedStoreForOrg("default")
	heard := func(alertType string) bool {
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			for _, finding := range store.GetAll() {
				if finding.AlertType == alertType && finding.ResourceName != "" {
					return true
				}
			}
			time.Sleep(20 * time.Millisecond)
		}
		return false
	}
	countFor := func(resourceName string) int {
		count := 0
		for _, finding := range store.GetAll() {
			if finding.ResourceName == resourceName {
				count++
			}
		}
		return count
	}

	unhealthyContainer(first.GetAlertManager(), "nextcloud")
	if !heard("docker-container-health") || countFor("nextcloud") != 1 {
		t.Fatalf("before the reload the bridge did not hear the alert: %d findings", countFor("nextcloud"))
	}

	// The reload: the old monitor stops and the router is handed a new one,
	// through the same initializer the multi-tenant monitor calls for every
	// monitor it builds.
	stopFirst()
	second, _ := newMonitor()
	router.configureMonitorDependencies(second)

	unhealthyContainer(second.GetAlertManager(), "gitea")
	deadline := time.Now().Add(2 * time.Second)
	for countFor("gitea") == 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if countFor("gitea") != 1 {
		t.Fatalf("after the reload the bridge heard %d alerts from the new alert manager, want 1", countFor("gitea"))
	}

	// Handing the router the same monitor again must not subscribe twice.
	router.configureMonitorDependencies(second)
	unhealthyContainer(second.GetAlertManager(), "paperless")
	deadline = time.Now().Add(2 * time.Second)
	for countFor("paperless") == 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond)
	if countFor("paperless") != 1 {
		t.Fatalf("the bridge recorded %d findings for one alert, want 1", countFor("paperless"))
	}
}

// The same through the path a running server takes. Patrol is started the way
// the server starts it, the monitor is reloaded the way adding a node reloads
// it (ReloadableMonitor.Reload, then the router is handed the new monitors),
// and the new alert manager's fired, resolved and flapping callbacks must all
// reach Patrol.
func TestPatrolHearsAlertsAfterARealMonitorReload(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())
	cfg, err := config.LoadWithoutLoggingInit()
	if err != nil {
		t.Fatal(err)
	}
	persistence := config.NewMultiTenantPersistence(cfg.DataPath)
	defaultPersistence, err := persistence.GetPersistence("default")
	if err != nil {
		t.Fatal(err)
	}
	aiCfg := config.NewDefaultAIConfig()
	aiCfg.Enabled = true
	aiCfg.Model = "ollama:llama3"
	aiCfg.PatrolModel = "ollama:llama3"
	aiCfg.OllamaBaseURL = "http://127.0.0.1:9"
	if err := defaultPersistence.SaveAIConfig(*aiCfg); err != nil {
		t.Fatal(err)
	}

	reloadable, err := monitoring.NewReloadableMonitor(cfg, persistence, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	reloadable.Start(ctx)
	t.Cleanup(func() {
		reloadable.Stop()
		cancel()
	})

	// What pkg/server does at start and again after every reload.
	var router *Router
	handRouterTheMonitors := func() {
		router.SetMonitor(reloadable.GetMonitor())
		router.SetMultiTenantMonitor(reloadable.GetMultiTenantMonitor())
		if current := reloadable.GetConfig(); current != nil {
			router.SetConfig(current)
		}
		router.ReloadSystemSettings()
	}
	reload := func() error {
		if err := reloadable.Reload(); err != nil {
			return err
		}
		handRouterTheMonitors()
		return nil
	}
	router = newTestRouter(t, cfg, reloadable.GetMonitor(), reloadable.GetMultiTenantMonitor(), nil, reload, "test")
	t.Cleanup(router.ShutdownAIIntelligence)
	handRouterTheMonitors()
	router.StartPatrol(ctx)

	store := router.aiSettingsHandler.GetUnifiedStoreForOrg("default")
	if store == nil || router.aiSettingsHandler.GetAlertBridgeForOrg("default") == nil {
		t.Fatal("starting Patrol built no alert bridge")
	}
	patrol := router.aiSettingsHandler.GetAIService(ctx).GetPatrolService()
	if patrol == nil {
		t.Fatal("starting Patrol built no Patrol service")
	}

	container := func(manager *alerts.Manager, name, health string) {
		manager.CheckDockerHost(models.DockerHost{
			ID: "docker01", DisplayName: "docker01", Hostname: "docker01.lan",
			Containers: []models.DockerContainer{{ID: name + "-1", Name: name, State: "running", Status: "Up 10 minutes", Health: health}},
		})
	}
	// findingFor waits for the bridge's finding for a container to be open or resolved.
	findingFor := func(name string, resolved bool) bool {
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			for _, finding := range store.GetAll() {
				if finding.ResourceName == name && (finding.ResolvedAt != nil) == resolved {
					return true
				}
			}
			time.Sleep(20 * time.Millisecond)
		}
		return false
	}
	flappingFinding := func() bool {
		for _, finding := range patrol.GetFindings().GetAll(nil) {
			if strings.HasPrefix(finding.ID, "alert-flapping:") {
				return true
			}
		}
		return false
	}

	before := reloadable.GetMonitor().GetAlertManager()
	container(before, "nextcloud", "unhealthy")
	if !findingFor("nextcloud", false) {
		t.Fatal("before the reload the bridge did not hear a fired alert")
	}

	if err := reload(); err != nil {
		t.Fatal(err)
	}
	after := reloadable.GetMonitor().GetAlertManager()
	if after == before {
		t.Fatal("the reload kept the alert manager, so this test proves nothing")
	}

	// Fired.
	container(after, "gitea", "unhealthy")
	if !findingFor("gitea", false) {
		t.Error("after the reload the bridge did not hear a fired alert from the new alert manager")
	}
	// Resolved.
	container(after, "gitea", "healthy")
	if !findingFor("gitea", true) {
		t.Error("after the reload the bridge did not hear the alert resolve")
	}
	// Flapping.
	for round := 0; round < 12 && !flappingFinding(); round++ {
		container(after, "paperless", "unhealthy")
		container(after, "paperless", "healthy")
		time.Sleep(50 * time.Millisecond)
	}
	if !flappingFinding() {
		t.Error("after the reload a flapping alert on the new alert manager did not reach Patrol")
	}
}
