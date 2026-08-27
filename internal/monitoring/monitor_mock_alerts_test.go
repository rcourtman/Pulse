package monitoring

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestCheckMockDockerHostAlertsUsesHostLifecycleForOfflineFixtures(t *testing.T) {
	manager := alerts.NewManagerWithDataDir(t.TempDir())
	t.Cleanup(manager.Stop)
	config := manager.GetConfig()
	config.Enabled = true
	config.ActivationState = alerts.ActivationPending
	config.TimeThresholds = map[string]int{}
	config.DockerDefaults.StateDisableConnectivity = false
	manager.UpdateConfig(config)

	host := models.DockerHost{
		ID:          "branch-edge",
		Hostname:    "branch-edge-01",
		DisplayName: "Branch Edge",
		Status:      "offline",
		Containers: []models.DockerContainer{
			{ID: "portal-1", Name: "branch-portal", State: "exited", Status: "Exited (137)"},
			{ID: "sync-1", Name: "branch-syncthing", State: "exited", Status: "Exited (137)"},
			{ID: "vpn-1", Name: "branch-vpn", State: "exited", Status: "Exited (137)"},
		},
	}

	// Prove the fixture would create the child storm if it were incorrectly
	// treated as fresh online telemetry.
	manager.CheckDockerHost(host)
	manager.CheckDockerHost(host)
	if got := manager.GetActiveAlerts(); len(got) != len(host.Containers) {
		t.Fatalf("fresh container evaluation created %d alerts, want %d", len(got), len(host.Containers))
	}

	monitor := &Monitor{alertManager: manager}
	monitor.checkMockDockerHostAlerts(host)
	monitor.checkMockDockerHostAlerts(host)
	monitor.checkMockDockerHostAlerts(host)

	active := manager.GetActiveAlerts()
	if len(active) != 1 {
		t.Fatalf("offline mock host produced %d active alerts, want one host incident: %+v", len(active), active)
	}
	if active[0].Type != "docker-host-offline" || active[0].ResourceID != "docker:branch-edge" {
		t.Fatalf("offline mock host alert = %+v, want canonical host connectivity incident", active[0])
	}
}
