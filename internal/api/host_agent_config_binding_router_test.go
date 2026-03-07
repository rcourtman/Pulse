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
	record := newTokenRecord(t, rawToken, []string{config.ScopeAgentConfigRead}, nil)
	record.ID = "token-1"

	cfg := newTestConfigWithTokens(t, record)

	state := models.NewState()
	state.UpsertHost(models.Host{ID: "host-1", TokenID: "token-1"})
	monitor := &monitoring.Monitor{}
	setUnexportedField(t, monitor, "state", state)

	router := NewRouter(cfg, monitor, nil, nil, nil, "1.0.0")

	for _, path := range []string{"/api/agents/agent/host-2/config", "/api/agents/host/host-2/config"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("X-API-Token", rawToken)
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 for bound host config on %s, got %d", path, rec.Code)
		}

		var resp struct {
			AgentID string `json:"agentId"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("decode response on %s: %v", path, err)
		}
		if resp.AgentID != "host-1" {
			t.Fatalf("expected agent id %q on %s, got %q", "host-1", path, resp.AgentID)
		}
	}
}
