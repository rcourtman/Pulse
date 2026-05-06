package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
