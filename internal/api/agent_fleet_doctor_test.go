package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

func TestHandleAgentFleetDiagnosticsReturnsFleetPayload(t *testing.T) {
	monitor, state, _ := newTestMonitor(t)
	now := time.Now().UTC()
	state.UpsertHost(models.Host{
		ID:              "agent-1",
		Hostname:        "node-1",
		DisplayName:     "Node One",
		Platform:        "linux",
		Status:          "online",
		LastSeen:        now.Add(-30 * time.Second),
		IntervalSeconds: 30,
		AgentVersion:    "6.0.0",
	})

	router := &Router{config: &config.Config{}, monitor: monitor}
	req := httptest.NewRequest(http.MethodGet, "/api/agents/diagnostics", nil)
	rec := httptest.NewRecorder()

	router.handleAgentFleetDiagnostics(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}

	var payload monitoring.AgentFleetDiagnostics
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode diagnostics: %v", err)
	}
	if payload.Agents == nil {
		t.Fatal("expected agents to be a non-nil array")
	}
	if payload.Summary.Total != 1 || len(payload.Agents) != 1 {
		t.Fatalf("expected one agent diagnostic, summary=%+v agents=%+v", payload.Summary, payload.Agents)
	}
	if payload.SchemaVersion != monitoring.AgentFleetDiagnosticsSchemaVersion {
		t.Fatalf("schema version = %d, want %d", payload.SchemaVersion, monitoring.AgentFleetDiagnosticsSchemaVersion)
	}
	if payload.Agents[0].Name != "Node One" {
		t.Fatalf("agent name = %q, want Node One", payload.Agents[0].Name)
	}
	if payload.Agents[0].ConnectionID != "agent:agent-1" || payload.Agents[0].Platform != "linux" {
		t.Fatalf("agent identity = %+v", payload.Agents[0])
	}
	if payload.AgentUpdateTargetVersion != currentAgentTargetVersion() {
		t.Fatalf("agent update target = %q, want %q", payload.AgentUpdateTargetVersion, currentAgentTargetVersion())
	}
}
