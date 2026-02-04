package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

func TestHostAgentConfigUsesTokenBindingInRouter(t *testing.T) {
	rawToken := "host-config-binding-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeHostConfigRead}, nil)
	record.ID = "token-1"

	cfg := newTestConfigWithTokens(t, record)

	state := models.NewState()
	state.UpsertHost(models.Host{ID: "host-1", TokenID: "token-1"})
	monitor := &monitoring.Monitor{}
	setUnexportedField(t, monitor, "state", state)

	router := NewRouter(cfg, monitor, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/agents/host/host-2/config", nil)
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for bound host config, got %d", rec.Code)
	}

	var resp struct {
		HostID string `json:"hostId"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.HostID != "host-1" {
		t.Fatalf("expected host id %q, got %q", "host-1", resp.HostID)
	}
}
