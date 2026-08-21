package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/api/configapi"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

func newTestConfigHandlers(t *testing.T, cfg *config.Config) *ConfigHandlers {
	t.Helper()
	if cfg == nil {
		cfg = &config.Config{}
	}
	if cfg.DataPath == "" {
		cfg.DataPath = t.TempDir()
	}
	handler := NewConfigHandlers(nil, nil, func() error { return nil }, nil, nil, func() {}, false)
	handler.SetPersistence(config.NewConfigPersistence(cfg.DataPath))
	monitor, _, _ := newTestMonitor(t)
	manager := alerts.NewManager()
	t.Cleanup(manager.Stop)
	setUnexportedField(t, monitor, "alertManager", manager)
	handler.SetMonitor(monitor)
	handler.SetConfig(cfg)
	return handler
}

func stubAutoRegisterNetworkDeps(t *testing.T) {
	t.Helper()
	restore := configapi.ConfigureAutoRegisterNetworkDependencies(
		func(proxmox.ClientConfig, string, []config.ClusterEndpoint) (bool, string, []config.ClusterEndpoint) {
			return false, "", nil
		},
		func(string) (string, error) { return "", nil },
	)
	t.Cleanup(restore)
}

func runAgentAutoRegister(t *testing.T, handler *ConfigHandlers, rawToken string, payload AutoRegisterRequest) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal auto-register payload: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", bytes.NewReader(body))
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	handler.HandleAutoRegister(rec, req)
	return rec
}

func truncate(value string, maxLen int) string {
	if maxLen <= 0 || len(value) <= maxLen {
		return value
	}
	return value[:maxLen]
}
