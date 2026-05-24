package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"github.com/stretchr/testify/assert"
)

func TestRouter_HandleState_MockIsolation(t *testing.T) {
	// Enable mock mode to control GetState output
	t.Setenv("PULSE_MOCK_MODE", "true")
	mock.SetEnabled(true)
	defer mock.SetEnabled(false)

	dataPath := t.TempDir()
	hp, _ := auth.HashPassword("password")
	cfg := &config.Config{
		DataPath:           dataPath,
		MultiTenantEnabled: true,
		AuthUser:           "admin",
		AuthPass:           hp,
	}

	// Initialize persistent stores to avoid permission issues with /etc/pulse
	InitSessionStore(dataPath)
	InitCSRFStore(dataPath)

	// Create a router with a real monitor
	// Since monitor.GetState() checks IsMockEnabled(), it will return mock data.
	monitor, _ := monitoring.New(cfg)
	router := &Router{
		config:      cfg,
		monitor:     monitor,
		persistence: config.NewConfigPersistence(dataPath),
	}

	t.Run("basic state access", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/state", nil)
		// Set credentials for CheckAuth (Basic Auth)
		req.SetBasicAuth("admin", "password")

		w := httptest.NewRecorder()
		router.handleState(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var state models.StateFrontend
		err := json.Unmarshal(w.Body.Bytes(), &state)
		assert.NoError(t, err)

		// Verify we got the mock state
		assert.NotEmpty(t, state.Nodes)
		assert.Greater(t, state.LastUpdate, int64(0))
	})

	t.Run("invalid method", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/state", nil)
		w := httptest.NewRecorder()
		router.handleState(w, req)
		assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	})

	t.Run("unauthorized", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/state", nil)
		w := httptest.NewRecorder()
		router.handleState(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestRouter_HandleStateSummary(t *testing.T) {
	dataPath := t.TempDir()
	hp, _ := auth.HashPassword("password")
	cfg := &config.Config{
		DataPath: dataPath,
		AuthUser: "admin",
		AuthPass: hp,
	}
	monitor, state, _ := newTestMonitor(t)
	lastUpdate := time.Date(2026, 5, 24, 10, 11, 12, 0, time.UTC)
	state.Nodes = []models.Node{{ID: "node-1"}, {ID: "node-2"}}
	state.VMs = []models.VM{{ID: "vm-1"}}
	state.Containers = []models.Container{{ID: "ct-1"}, {ID: "ct-2"}, {ID: "ct-3"}}
	state.ActiveAlerts = []models.Alert{{ID: "alert-1"}}
	state.DockerHosts = []models.DockerHost{
		{
			ID:            "docker-1",
			Hostname:      "docker.local",
			DisplayName:   "Docker Host",
			UptimeSeconds: 3600,
			CPUUsage:      12.5,
			Containers: []models.DockerContainer{
				{ID: "container-1"},
				{ID: "container-2"},
			},
		},
	}
	state.LastUpdate = lastUpdate

	router := &Router{config: cfg, monitor: monitor}
	req := httptest.NewRequest(http.MethodGet, "/api/state/summary", nil)
	req.SetBasicAuth("admin", "password")
	rec := httptest.NewRecorder()

	router.handleStateSummary(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var summary stateSummaryResponse
	err := json.Unmarshal(rec.Body.Bytes(), &summary)
	assert.NoError(t, err)
	assert.Equal(t, 0, summary.ActiveAlerts)
	assert.Equal(t, 2, summary.Nodes)
	assert.Equal(t, 1, summary.VMs)
	assert.Equal(t, 3, summary.Containers)
	assert.Equal(t, lastUpdate, summary.LastUpdate)
	if assert.Len(t, summary.DockerHosts, 1) {
		assert.Equal(t, "Docker Host", summary.DockerHosts[0].Name)
		assert.Equal(t, 2, summary.DockerHosts[0].Containers)
		assert.Equal(t, int64(3600), summary.DockerHosts[0].UptimeSeconds)
		assert.Equal(t, 12.5, summary.DockerHosts[0].CPUUsagePercent)
	}
}

func TestBuildStateSummaryCountsSnapshotResources(t *testing.T) {
	lastUpdate := time.Date(2026, 5, 24, 10, 11, 12, 0, time.UTC)
	summary := buildStateSummary(models.StateSnapshot{
		ActiveAlerts: []models.Alert{{ID: "alert-1"}},
		Nodes:        []models.Node{{ID: "node-1"}, {ID: "node-2"}},
		VMs:          []models.VM{{ID: "vm-1"}},
		Containers:   []models.Container{{ID: "ct-1"}, {ID: "ct-2"}, {ID: "ct-3"}},
		DockerHosts: []models.DockerHost{{
			ID:                "docker-1",
			Hostname:          "docker.local",
			DisplayName:       "Docker Host",
			CustomDisplayName: "Custom Docker Host",
			UptimeSeconds:     3600,
			CPUUsage:          12.5,
			Containers:        []models.DockerContainer{{ID: "container-1"}, {ID: "container-2"}},
		}},
		LastUpdate: lastUpdate,
	})

	assert.Equal(t, 1, summary.ActiveAlerts)
	assert.Equal(t, 2, summary.Nodes)
	assert.Equal(t, 1, summary.VMs)
	assert.Equal(t, 3, summary.Containers)
	assert.Equal(t, lastUpdate, summary.LastUpdate)
	if assert.Len(t, summary.DockerHosts, 1) {
		assert.Equal(t, "Custom Docker Host", summary.DockerHosts[0].Name)
		assert.Equal(t, 2, summary.DockerHosts[0].Containers)
		assert.Equal(t, int64(3600), summary.DockerHosts[0].UptimeSeconds)
		assert.Equal(t, 12.5, summary.DockerHosts[0].CPUUsagePercent)
	}
}
