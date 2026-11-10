package api

import (
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
