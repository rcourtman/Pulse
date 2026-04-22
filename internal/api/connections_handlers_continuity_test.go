package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

func TestConnectionsHandleListIncludesContinuityBackedHostAgents(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{DataPath: dir}
	now := time.Now().UTC()

	seedMonitor, err := monitoring.New(cfg)
	if err != nil {
		t.Fatalf("monitoring.New seed monitor: %v", err)
	}
	t.Cleanup(seedMonitor.Stop)

	_, err = seedMonitor.ApplyHostReport(agentshost.Report{
		Agent: agentshost.AgentInfo{
			ID:              "agent-1",
			Version:         "6.0.0-rc.1",
			IntervalSeconds: 30,
		},
		Host: agentshost.HostInfo{
			ID:        "machine-1",
			MachineID: "machine-1",
			Hostname:  "host-1.local",
			Platform:  "linux",
		},
		Timestamp: now,
	}, &config.APITokenRecord{ID: "token-1", Name: "Token One"})
	if err != nil {
		t.Fatalf("ApplyHostReport seed continuity: %v", err)
	}

	reloadedMonitor, err := monitoring.New(cfg)
	if err != nil {
		t.Fatalf("monitoring.New reloaded monitor: %v", err)
	}
	t.Cleanup(reloadedMonitor.Stop)

	handler := NewConnectionsHandlers(
		func(context.Context) *config.Config { return cfg },
		func(context.Context) *config.ConfigPersistence { return nil },
		func(context.Context) *monitoring.Monitor { return reloadedMonitor },
	)

	req := httptest.NewRequest(http.MethodGet, "/api/connections", nil)
	rec := httptest.NewRecorder()
	handler.HandleList(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp ConnectionsListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode connections response: %v", err)
	}
	if len(resp.Connections) != 1 {
		t.Fatalf("expected 1 continuity-backed connection, got %d", len(resp.Connections))
	}

	conn := resp.Connections[0]
	if conn.ID != "agent:machine-1" {
		t.Fatalf("connection id = %q, want %q", conn.ID, "agent:machine-1")
	}
	if conn.Type != ConnectionTypeAgent {
		t.Fatalf("connection type = %q, want %q", conn.Type, ConnectionTypeAgent)
	}
	if conn.Name != "host-1.local" || conn.Address != "host-1.local" {
		t.Fatalf("unexpected continuity-backed connection identity: name=%q address=%q", conn.Name, conn.Address)
	}
	if conn.State != ConnectionStateActive {
		t.Fatalf("connection state = %q, want %q", conn.State, ConnectionStateActive)
	}
	if conn.Source != ConnectionSourceAgent {
		t.Fatalf("connection source = %q, want %q", conn.Source, ConnectionSourceAgent)
	}
	if conn.LastSeen == nil || conn.LastSeen.IsZero() {
		t.Fatalf("expected continuity-backed connection to carry lastSeen, got %#v", conn.LastSeen)
	}
}
