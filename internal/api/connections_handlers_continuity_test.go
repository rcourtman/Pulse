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

func TestConnectionsLedgerSeparatesTelemetryHealthFromCommandAdmission(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{DataPath: dir}
	now := time.Now().UTC()
	monitor, err := monitoring.New(cfg)
	if err != nil {
		t.Fatalf("monitoring.New: %v", err)
	}
	t.Cleanup(monitor.Stop)

	for _, report := range []agentshost.Report{
		{
			Agent:     agentshost.AgentInfo{ID: "agent-enabled", Version: "6.1.1", IntervalSeconds: 30, CommandsEnabled: true},
			Host:      agentshost.HostInfo{ID: "machine-enabled", MachineID: "machine-enabled", Hostname: "docker-host", Platform: "linux"},
			Timestamp: now,
		},
		{
			Agent:     agentshost.AgentInfo{ID: "agent-disabled", Version: "6.1.1", IntervalSeconds: 30, CommandsEnabled: false},
			Host:      agentshost.HostInfo{ID: "machine-disabled", MachineID: "machine-disabled", Hostname: "policy-disabled", Platform: "linux"},
			Timestamp: now,
		},
	} {
		tokenID := "token-enabled"
		if report.Agent.ID == "agent-disabled" {
			tokenID = "token-disabled"
		}
		if _, err := monitor.ApplyHostReport(report, &config.APITokenRecord{ID: tokenID, Name: tokenID}); err != nil {
			t.Fatalf("ApplyHostReport(%s): %v", report.Agent.ID, err)
		}
	}

	handler := NewConnectionsHandlers(
		func(context.Context) *config.Config { return cfg },
		func(context.Context) *config.ConfigPersistence { return nil },
		func(context.Context) *monitoring.Monitor { return monitor },
	)
	handler.SetAgentCommandSessionProvider(func(_, tokenID, _, _ string) bool {
		return tokenID == "token-connected"
	})

	req := httptest.NewRequest(http.MethodGet, "/api/connections", nil)
	rec := httptest.NewRecorder()
	handler.HandleList(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var response ConnectionsListResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode: %v", err)
	}
	byID := make(map[string]Connection, len(response.Connections))
	for _, connection := range response.Connections {
		byID[connection.ID] = connection
	}

	enabled := byID["agent:machine-enabled"]
	if enabled.State != ConnectionStateActive || enabled.Fleet.AdapterHealth != fleetStateHealthy {
		t.Fatalf("telemetry health changed when command channel was absent: %+v", enabled.Fleet)
	}
	if enabled.Fleet.RemoteControl != fleetStateDisconnected ||
		enabled.Fleet.CommandPolicy == nil ||
		enabled.Fleet.CommandPolicy.Status != fleetStateBlocked {
		t.Fatalf("enabled report without admitted socket must be disconnected/blocked: %+v", enabled.Fleet)
	}

	disabled := byID["agent:machine-disabled"]
	if disabled.Fleet.RemoteControl != fleetStateDisabled ||
		disabled.Fleet.CommandPolicy == nil ||
		disabled.Fleet.CommandPolicy.Status != fleetStateDisabled {
		t.Fatalf("disabled command policy must not be misreported as a transport failure: %+v", disabled.Fleet)
	}
}

func TestConnectionsHandleListUsesTrueNASPollerRuntimeSummary(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	connection := config.TrueNASInstance{
		ID:                 "tn1",
		Name:               "TrueNAS",
		Host:               "truenas.lan",
		APIKey:             "secret",
		UseHTTPS:           true,
		Enabled:            true,
		PollIntervalSecs:   60,
		MonitorDatasets:    true,
		MonitorPools:       true,
		MonitorReplication: true,
	}
	if err := persistence.SaveTrueNASConfig([]config.TrueNASInstance{connection}); err != nil {
		t.Fatalf("SaveTrueNASConfig: %v", err)
	}

	poller := monitoring.NewTrueNASPoller(nil, time.Minute, nil)
	successAt := time.Now().UTC().Add(-30 * time.Second)
	poller.RecordConnectionTestSuccess("default", connection.ID, connection, successAt)

	handler := NewConnectionsHandlers(
		func(context.Context) *config.Config { return nil },
		func(context.Context) *config.ConfigPersistence { return persistence },
		func(context.Context) *monitoring.Monitor { return nil },
	)
	handler.SetPlatformPollers(
		func(context.Context) *monitoring.TrueNASPoller { return poller },
		nil,
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

	var tn *Connection
	for i := range resp.Connections {
		if resp.Connections[i].ID == "truenas:tn1" {
			tn = &resp.Connections[i]
			break
		}
	}
	if tn == nil {
		t.Fatalf("expected TrueNAS connection, got %+v", resp.Connections)
	}
	if tn.State != ConnectionStateActive {
		t.Fatalf("state = %q, want active", tn.State)
	}
	if tn.LastSeen == nil || !tn.LastSeen.Equal(successAt) {
		t.Fatalf("lastSeen = %+v, want %s", tn.LastSeen, successAt)
	}
	if tn.Fleet.CredentialStatus != fleetStateVerified ||
		tn.Fleet.CredentialHealth == nil ||
		tn.Fleet.CredentialHealth.Status != fleetStateVerified {
		t.Fatalf("credential governance = %+v", tn.Fleet)
	}
}
