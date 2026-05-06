package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
)

func TestAvailabilityHandlersCRUDPersistsTargets(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	handler := NewAvailabilityHandlers(
		func(_ context.Context) *config.ConfigPersistence { return persistence },
		nil,
	)

	createBody := availabilityRequestBody(t, config.AvailabilityTarget{
		Name:             "Energy monitor",
		Address:          "device.local",
		Protocol:         config.AvailabilityProbeICMP,
		Enabled:          true,
		PollIntervalSecs: 30,
		TimeoutMillis:    1000,
		FailureThreshold: 2,
	})
	createReq := httptest.NewRequest(http.MethodPost, "/api/availability-targets", createBody)
	createRec := httptest.NewRecorder()
	handler.HandleAdd(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("HandleAdd status = %d, body=%s", createRec.Code, createRec.Body.String())
	}

	var created config.AvailabilityTarget
	if err := json.NewDecoder(createRec.Body).Decode(&created); err != nil {
		t.Fatalf("decode created target: %v", err)
	}
	if created.ID == "" {
		t.Fatal("created ID is empty")
	}

	updated := created
	updated.Enabled = false
	updateBody := availabilityRequestBody(t, updated)
	updateReq := httptest.NewRequest(http.MethodPut, "/api/availability-targets/"+created.ID, updateBody)
	updateRec := httptest.NewRecorder()
	handler.HandleUpdate(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("HandleUpdate status = %d, body=%s", updateRec.Code, updateRec.Body.String())
	}

	loaded, err := persistence.LoadAvailabilityTargets()
	if err != nil {
		t.Fatalf("LoadAvailabilityTargets() error = %v", err)
	}
	if len(loaded) != 1 || loaded[0].Enabled {
		t.Fatalf("loaded targets = %+v, want one paused target", loaded)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/availability-targets", nil)
	listRec := httptest.NewRecorder()
	handler.HandleList(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("HandleList status = %d, body=%s", listRec.Code, listRec.Body.String())
	}

	var listed []availabilityTargetResponse
	if err := json.NewDecoder(listRec.Body).Decode(&listed); err != nil {
		t.Fatalf("decode listed targets: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != created.ID {
		t.Fatalf("listed targets = %+v, want created target", listed)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/availability-targets/"+created.ID, nil)
	deleteRec := httptest.NewRecorder()
	handler.HandleDelete(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("HandleDelete status = %d, body=%s", deleteRec.Code, deleteRec.Body.String())
	}
	loaded, err = persistence.LoadAvailabilityTargets()
	if err != nil {
		t.Fatalf("LoadAvailabilityTargets() after delete error = %v", err)
	}
	if len(loaded) != 0 {
		t.Fatalf("loaded targets after delete = %+v, want none", loaded)
	}
}

func TestAvailabilityHandlersTestSavedTarget(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	persistence := config.NewConfigPersistence(t.TempDir())
	target := config.NormalizeAvailabilityTarget(config.AvailabilityTarget{
		ID:            "status-page",
		Name:          "Status page",
		Address:       server.URL,
		Protocol:      config.AvailabilityProbeHTTP,
		Enabled:       true,
		TimeoutMillis: 1000,
	})
	if err := persistence.SaveAvailabilityTargets([]config.AvailabilityTarget{target}); err != nil {
		t.Fatalf("SaveAvailabilityTargets() error = %v", err)
	}

	handler := NewAvailabilityHandlers(
		func(_ context.Context) *config.ConfigPersistence { return persistence },
		nil,
	)

	req := httptest.NewRequest(http.MethodPost, "/api/availability-targets/status-page/test", nil)
	rec := httptest.NewRecorder()
	handler.HandleTestSavedConnection(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("HandleTestSavedConnection status = %d, body=%s", rec.Code, rec.Body.String())
	}

	var response availabilityTestResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode test response: %v", err)
	}
	if !response.Success {
		t.Fatalf("response = %+v, want success", response)
	}
}

func TestAvailabilityHandlersListReturnsMockTargetsInMockMode(t *testing.T) {
	previous := mock.IsMockEnabled()
	if err := mock.SetEnabled(true); err != nil {
		t.Fatalf("enable mock mode: %v", err)
	}
	t.Cleanup(func() { _ = mock.SetEnabled(previous) })

	handler := NewAvailabilityHandlers(
		func(context.Context) *config.ConfigPersistence {
			t.Fatal("mock availability list should not load persistence")
			return nil
		},
		nil,
	)

	req := httptest.NewRequest(http.MethodGet, "/api/availability-targets", nil)
	rec := httptest.NewRecorder()
	handler.HandleList(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("HandleList status = %d, body=%s", rec.Code, rec.Body.String())
	}

	var listed []availabilityTargetResponse
	if err := json.NewDecoder(rec.Body).Decode(&listed); err != nil {
		t.Fatalf("decode listed targets: %v", err)
	}
	if len(listed) < 4 {
		t.Fatalf("expected mock availability targets, got %+v", listed)
	}
	foundMQTT := false
	for _, target := range listed {
		if target.ID != "mock-availability-mqtt-meter" {
			continue
		}
		foundMQTT = true
		if target.Protocol != config.AvailabilityProbeTCP || target.Port != 1883 {
			t.Fatalf("unexpected MQTT target: %+v", target.AvailabilityTarget)
		}
		if target.Status == nil || !target.Status.Available {
			t.Fatalf("expected successful MQTT status, got %+v", target.Status)
		}
	}
	if !foundMQTT {
		t.Fatalf("expected MQTT power meter target, got %+v", listed)
	}
}

func TestAvailabilityHandlersTestSavedMockTargetUsesSyntheticStatus(t *testing.T) {
	previous := mock.IsMockEnabled()
	if err := mock.SetEnabled(true); err != nil {
		t.Fatalf("enable mock mode: %v", err)
	}
	t.Cleanup(func() { _ = mock.SetEnabled(previous) })

	handler := NewAvailabilityHandlers(nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/availability-targets/mock-availability-door-controller/test", nil)
	rec := httptest.NewRecorder()
	handler.HandleTestSavedConnection(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("HandleTestSavedConnection status = %d, body=%s", rec.Code, rec.Body.String())
	}

	var response availabilityTestResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode test response: %v", err)
	}
	if response.Success {
		t.Fatalf("expected synthetic failure for offline mock target, got %+v", response)
	}
	if response.Error != "icmp probe timed out" {
		t.Fatalf("unexpected mock test error: %+v", response)
	}
}

func availabilityRequestBody(t *testing.T, target config.AvailabilityTarget) *bytes.Reader {
	t.Helper()
	payload, err := json.Marshal(target)
	if err != nil {
		t.Fatalf("marshal target: %v", err)
	}
	return bytes.NewReader(payload)
}
