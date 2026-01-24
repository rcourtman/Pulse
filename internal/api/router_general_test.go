package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/stretchr/testify/assert"
)

func TestRouter_ConfigUpdates(t *testing.T) {
	// Initialize Router with minimal dependencies
	tmpDir := t.TempDir()
	cfg := &config.Config{
		BackendPort: 8080,
		DataPath:    tmpDir,
		ConfigPath:  tmpDir,
	}

	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	// Test SetConfig
	newCfg := &config.Config{
		BackendPort: 9090,
		DataPath:    tmpDir,
		ConfigPath:  tmpDir,
	}
	router.SetConfig(newCfg)

	// Check SetMonitor
	mon := &monitoring.Monitor{}
	router.SetMonitor(mon)
}

func TestRouter_HandlerWrapping(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   tmpDir,
		ConfigPath: tmpDir,
	}
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	handler := router.Handler()
	assert.NotNil(t, handler)

	// Verify it implements http.Handler
	_, ok := handler.(http.Handler)
	assert.True(t, ok)

	// Verify it handles a basic request
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
}

func TestRouter_GetTenantMonitor_Defaults(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		MultiTenantEnabled: false,
		DataPath:           tmpDir,
		ConfigPath:         tmpDir,
	}
	defaultMon := &monitoring.Monitor{}

	router := NewRouter(cfg, defaultMon, nil, nil, nil, "1.0.0")

	// With flag disabled, should always return default monitor
	ctx := context.Background()
	m := router.getTenantMonitor(ctx)
	assert.Equal(t, defaultMon, m)

	// Even with tenant context
	ctx = context.WithValue(ctx, OrgIDContextKey, "some-tenant")
	m = router.getTenantMonitor(ctx)
	assert.Equal(t, defaultMon, m)
}
