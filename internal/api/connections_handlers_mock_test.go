package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

func TestConnectionsHandleListIncludesMockAvailabilityTargets(t *testing.T) {
	previous := mock.IsMockEnabled()
	if err := mock.SetEnabled(true); err != nil {
		t.Fatalf("enable mock mode: %v", err)
	}
	t.Cleanup(func() { _ = mock.SetEnabled(previous) })

	handler := NewConnectionsHandlers(
		func(context.Context) *config.Config { return nil },
		func(context.Context) *config.ConfigPersistence { return nil },
		func(context.Context) *monitoring.Monitor { return nil },
	)

	req := httptest.NewRequest(http.MethodGet, "/api/connections", nil)
	rec := httptest.NewRecorder()
	handler.HandleList(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("HandleList status = %d, body=%s", rec.Code, rec.Body.String())
	}

	var response ConnectionsListResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode connections response: %v", err)
	}
	byID := make(map[string]Connection, len(response.Connections))
	for _, connection := range response.Connections {
		byID[connection.ID] = connection
	}

	mqtt, ok := byID["availability:mock-availability-mqtt-meter"]
	if !ok {
		t.Fatalf("expected mock MQTT availability connection, got %+v", response.Connections)
	}
	if mqtt.Type != ConnectionTypeAvailability || mqtt.State != ConnectionStateActive {
		t.Fatalf("unexpected MQTT connection state: %+v", mqtt)
	}

	esphome, ok := byID["availability:mock-availability-esphome-greenhouse"]
	if !ok {
		t.Fatalf("expected mock ESPHome availability connection, got %+v", response.Connections)
	}
	if esphome.Type != ConnectionTypeAvailability || esphome.State != ConnectionStateActive {
		t.Fatalf("unexpected ESPHome connection state: %+v", esphome)
	}

	door, ok := byID["availability:mock-availability-door-controller"]
	if !ok {
		t.Fatalf("expected mock door controller availability connection, got %+v", response.Connections)
	}
	if door.State != ConnectionStateUnreachable {
		t.Fatalf("door controller state = %q, want unreachable", door.State)
	}
	if door.LastError == nil || door.LastError.Category != "availability" {
		t.Fatalf("expected availability error metadata, got %+v", door.LastError)
	}
}

// TestConnectionsHandleListDropsRealSourcesInMockMode asserts the served
// /api/connections payload, not just the aggregator inputs: mock mode is a
// clean room, so no real configured source may reach the wire, by name or by
// address.
func TestConnectionsHandleListDropsRealSourcesInMockMode(t *testing.T) {
	previous := mock.IsMockEnabled()
	if err := mock.SetEnabled(true); err != nil {
		t.Fatalf("enable mock mode: %v", err)
	}
	t.Cleanup(func() { _ = mock.SetEnabled(previous) })

	cfg := &config.Config{
		PVEInstances: []config.PVEInstance{{Name: "minipc", Host: "https://minipc:8006"}},
		PBSInstances: []config.PBSInstance{{Name: "backup-vault", Host: "https://backup-vault:8007"}},
		PMGInstances: []config.PMGInstance{{Name: "mail-relay", Host: "https://mail-relay:8006"}},
	}

	handler := NewConnectionsHandlers(
		func(context.Context) *config.Config { return cfg },
		func(context.Context) *config.ConfigPersistence { return nil },
		func(context.Context) *monitoring.Monitor { return nil },
	)

	req := httptest.NewRequest(http.MethodGet, "/api/connections", nil)
	rec := httptest.NewRecorder()
	handler.HandleList(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("HandleList status = %d, body=%s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	for _, secret := range []string{"minipc", "backup-vault", "mail-relay"} {
		if strings.Contains(body, secret) {
			t.Fatalf("real source %q leaked into the mock connections payload: %s", secret, body)
		}
	}

	var response ConnectionsListResponse
	if err := json.NewDecoder(strings.NewReader(body)).Decode(&response); err != nil {
		t.Fatalf("decode connections response: %v", err)
	}
	for _, conn := range response.Connections {
		switch conn.Type {
		case ConnectionTypePVE, ConnectionTypePBS, ConnectionTypePMG:
			t.Fatalf("mock ledger must not carry configured platform rows, got %+v", conn)
		}
	}
	if len(response.Connections) == 0 {
		t.Fatal("mock ledger must still compose authored fixture rows")
	}
}

func TestConnectionsLedgerKeepsRealSourcesWhenRealPollingRetained(t *testing.T) {
	previous := mock.IsMockEnabled()
	if err := mock.SetEnabled(true); err != nil {
		t.Fatalf("enable mock mode: %v", err)
	}
	t.Cleanup(func() { _ = mock.SetEnabled(previous) })
	t.Setenv("PULSE_MOCK_KEEP_REAL_POLLING", "true")

	cfg := &config.Config{
		PVEInstances: []config.PVEInstance{{Name: "minipc", Host: "https://minipc:8006"}},
	}

	inputs := buildAggregatorInputsWithRuntimeSources(context.Background(), cfg, nil, nil, aggregatorRuntimeSources{})
	if len(inputs.pveInstances) != 1 {
		t.Fatalf("real polling opt-in must keep configured sources, got %+v", inputs.pveInstances)
	}
}
