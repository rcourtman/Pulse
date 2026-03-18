package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
	// Since monitor.ReadSnapshot() checks IsMockEnabled(), it will return mock data.
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

		var payload map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &payload)
		assert.NoError(t, err)

		// Verify v6 contract: legacy per-type arrays are omitted from /api/state.
		for _, key := range []string{"nodes", "vms", "containers", "dockerHosts", "hosts", "storage"} {
			if _, ok := payload[key]; ok {
				t.Fatalf("expected %q to be omitted from /api/state payload", key)
			}
		}

		var state models.StateFrontend
		err = json.Unmarshal(w.Body.Bytes(), &state)
		assert.NoError(t, err)
		assert.Greater(t, state.LastUpdate, int64(0))
		assert.NotNil(t, state.RecentlyResolved)
		assert.NotNil(t, state.Metrics)
		assert.NotNil(t, state.ConnectionHealth)

		metricsPayload, ok := payload["metrics"].([]any)
		if !ok {
			t.Fatalf("expected metrics payload to be an array, got %T", payload["metrics"])
		}
		if metricsPayload == nil {
			t.Fatal("expected metrics payload array to be non-nil")
		}

		if _, ok := payload["performance"].(map[string]any); !ok {
			t.Fatalf("expected performance payload to be an object, got %T", payload["performance"])
		}
		if _, ok := payload["stats"].(map[string]any); !ok {
			t.Fatalf("expected stats payload to be an object, got %T", payload["stats"])
		}
		if _, ok := payload["resources"].([]any); !ok {
			t.Fatalf("expected resources payload to be an array, got %T", payload["resources"])
		}
		if _, ok := payload["connectedInfrastructure"].([]any); !ok {
			t.Fatalf("expected connectedInfrastructure payload to be an array, got %T", payload["connectedInfrastructure"])
		}
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
