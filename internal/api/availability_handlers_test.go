package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
)

func TestAvailabilityHandlersCRUDPersistsTargets(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	handler := NewAvailabilityHandlers(
		func(_ context.Context) *config.ConfigPersistence { return persistence },
		nil,
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
	if created.ConfigRevision != 1 {
		t.Fatalf("created config revision = %d, want 1", created.ConfigRevision)
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
	if loaded[0].ConfigRevision != 1 {
		t.Fatalf("non-execution edit revision = %d, want 1", loaded[0].ConfigRevision)
	}

	executionEdit := loaded[0]
	executionEdit.Address = "gateway-2.local"
	executionEdit.ConfigRevision = 99
	updateBody = availabilityRequestBody(t, executionEdit)
	updateReq = httptest.NewRequest(http.MethodPut, "/api/availability-targets/"+created.ID, updateBody)
	updateRec = httptest.NewRecorder()
	handler.HandleUpdate(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("execution HandleUpdate status = %d, body=%s", updateRec.Code, updateRec.Body.String())
	}
	loaded, err = persistence.LoadAvailabilityTargets()
	if err != nil {
		t.Fatalf("LoadAvailabilityTargets() after execution edit error = %v", err)
	}
	if loaded[0].ConfigRevision != 2 {
		t.Fatalf("server-authored execution edit revision = %d, want 2", loaded[0].ConfigRevision)
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

func TestAvailabilityHandlersCreateNormalizesPingAlias(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	handler := NewAvailabilityHandlers(
		func(_ context.Context) *config.ConfigPersistence { return persistence },
		nil,
		nil,
	)

	createBody := availabilityRequestBody(t, config.AvailabilityTarget{
		Name:             "Garage sensor",
		Address:          "https://garage-sensor.local/status",
		Protocol:         config.AvailabilityProbeProtocol("ping"),
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
	if created.Protocol != config.AvailabilityProbeICMP {
		t.Fatalf("created Protocol = %q, want %q", created.Protocol, config.AvailabilityProbeICMP)
	}
	if created.Address != "garage-sensor.local" {
		t.Fatalf("created Address = %q, want garage-sensor.local", created.Address)
	}

	loaded, err := persistence.LoadAvailabilityTargets()
	if err != nil {
		t.Fatalf("LoadAvailabilityTargets() error = %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("LoadAvailabilityTargets() length = %d, want 1", len(loaded))
	}
	if loaded[0].Protocol != config.AvailabilityProbeICMP {
		t.Fatalf("persisted Protocol = %q, want %q", loaded[0].Protocol, config.AvailabilityProbeICMP)
	}
}

func TestAvailabilityHandlersTestSavedTarget(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	persistence := config.NewConfigPersistence(t.TempDir())
	target := config.NormalizeAvailabilityTarget(config.AvailabilityTarget{
		ID:            "status-page",
		Name:          "Status page",
		Address:       server.URL,
		Protocol:      config.AvailabilityProbeHTTPS,
		Enabled:       true,
		TimeoutMillis: 1000,
	})
	if err := persistence.SaveAvailabilityTargets([]config.AvailabilityTarget{target}); err != nil {
		t.Fatalf("SaveAvailabilityTargets() error = %v", err)
	}

	handler := NewAvailabilityHandlers(
		func(_ context.Context) *config.ConfigPersistence { return persistence },
		nil,
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
	if response.Certificate == nil || response.Certificate.TrustStatus != "self-signed" {
		t.Fatalf("certificate = %+v, want self-signed test-server certificate", response.Certificate)
	}
}

func TestAvailabilityHandlersRedactAndPreserveHTTPContractSecrets(t *testing.T) {
	const password = "never-return-this-password"
	const headerValue = "never-return-this-header"
	const requestBody = `{"secret":"never-return-this-body"}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, gotPassword, ok := r.BasicAuth()
		if !ok || username != "pulse" || gotPassword != password {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.Header.Get("X-Contract-Key") != headerValue {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"healthy"}`))
	}))
	defer server.Close()

	persistence := config.NewConfigPersistence(t.TempDir())
	handler := NewAvailabilityHandlers(
		func(_ context.Context) *config.ConfigPersistence { return persistence },
		nil,
		nil,
	)
	passwordValue, headerSecret, bodyValue := password, headerValue, requestBody
	target := config.AvailabilityTarget{
		ID: "contract-target", Name: "Contract target", Address: server.URL,
		Protocol: config.AvailabilityProbeHTTP, Enabled: true, TimeoutMillis: 1000,
		HTTP: &config.AvailabilityHTTPConfig{
			Method:         config.AvailabilityHTTPMethodPOST,
			Headers:        []config.AvailabilityHTTPHeader{{ID: "contract-key", Name: "X-Contract-Key", Value: &headerSecret}},
			Authentication: config.AvailabilityHTTPAuthentication{Type: config.AvailabilityHTTPAuthBasic, Username: "pulse", Password: &passwordValue},
			Body:           &bodyValue, ExpectedStatusMin: 200, ExpectedStatusMax: 299,
			JSONPath: "status", JSONEquals: "healthy",
		},
	}
	createRec := httptest.NewRecorder()
	handler.HandleAdd(createRec, httptest.NewRequest(http.MethodPost, "/api/availability-targets", availabilityRequestBody(t, target)))
	if createRec.Code != http.StatusCreated {
		t.Fatalf("HandleAdd status = %d, body=%s", createRec.Code, createRec.Body.String())
	}
	for _, secret := range []string{password, headerValue, requestBody} {
		if strings.Contains(createRec.Body.String(), secret) {
			t.Fatalf("create response leaked secret %q: %s", secret, createRec.Body.String())
		}
	}

	var created availabilityTargetResponse
	if err := json.NewDecoder(createRec.Body).Decode(&created); err != nil {
		t.Fatalf("decode created response: %v", err)
	}
	if created.HTTPSecrets == nil || !created.HTTPSecrets.PasswordConfigured || !created.HTTPSecrets.BodyConfigured ||
		len(created.HTTPSecrets.Headers) != 1 || !created.HTTPSecrets.Headers[0].ValueConfigured {
		t.Fatalf("secret state = %+v, want configured flags", created.HTTPSecrets)
	}
	if created.HTTP == nil || created.HTTP.Authentication.Password != nil || created.HTTP.Body != nil || created.HTTP.Headers[0].Value != nil {
		t.Fatalf("redacted target still contains secret values: %+v", created.HTTP)
	}

	created.Name = "Renamed contract target"
	updateRec := httptest.NewRecorder()
	handler.HandleUpdate(updateRec, httptest.NewRequest(http.MethodPut, "/api/availability-targets/contract-target", availabilityRequestBody(t, created.AvailabilityTarget)))
	if updateRec.Code != http.StatusOK {
		t.Fatalf("HandleUpdate status = %d, body=%s", updateRec.Code, updateRec.Body.String())
	}
	loaded, err := persistence.LoadAvailabilityTargets()
	if err != nil || len(loaded) != 1 {
		t.Fatalf("LoadAvailabilityTargets() = %+v, %v", loaded, err)
	}
	stored := loaded[0]
	if stored.HTTP == nil || stored.HTTP.Authentication.Password == nil || *stored.HTTP.Authentication.Password != password ||
		stored.HTTP.Body == nil || *stored.HTTP.Body != requestBody || stored.HTTP.Headers[0].Value == nil || *stored.HTTP.Headers[0].Value != headerValue {
		t.Fatalf("stored contract did not preserve write-only secrets: %+v", stored.HTTP)
	}
	if stored.ConfigRevision != 1 {
		t.Fatalf("display-only edit revision = %d, want 1", stored.ConfigRevision)
	}

	testRec := httptest.NewRecorder()
	handler.HandleTestConnection(testRec, httptest.NewRequest(http.MethodPost, "/api/availability-targets/test", availabilityRequestBody(t, created.AvailabilityTarget)))
	if testRec.Code != http.StatusOK {
		t.Fatalf("HandleTestConnection status = %d, body=%s", testRec.Code, testRec.Body.String())
	}
	var tested availabilityTestResponse
	if err := json.NewDecoder(testRec.Body).Decode(&tested); err != nil {
		t.Fatalf("decode test response: %v", err)
	}
	if !tested.Success || tested.TransportOutcome != "reachable" || tested.Application == nil || tested.Application.Outcome != "passed" {
		t.Fatalf("test response = %+v, want reachable transport and passing application", tested)
	}
	for _, secret := range []string{password, headerValue, requestBody, "healthy"} {
		if strings.Contains(testRec.Body.String(), secret) {
			t.Fatalf("test response leaked request or response content %q: %s", secret, testRec.Body.String())
		}
	}

	changedOriginReached := false
	changedOrigin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		changedOriginReached = true
		w.WriteHeader(http.StatusOK)
	}))
	defer changedOrigin.Close()
	created.Address = changedOrigin.URL
	changedOriginUpdateRec := httptest.NewRecorder()
	handler.HandleUpdate(changedOriginUpdateRec, httptest.NewRequest(http.MethodPut, "/api/availability-targets/contract-target", availabilityRequestBody(t, created.AvailabilityTarget)))
	if changedOriginUpdateRec.Code != http.StatusBadRequest {
		t.Fatalf("changed-origin update status = %d, body=%s; want re-entry validation", changedOriginUpdateRec.Code, changedOriginUpdateRec.Body.String())
	}
	changedOriginTestRec := httptest.NewRecorder()
	handler.HandleTestConnection(changedOriginTestRec, httptest.NewRequest(http.MethodPost, "/api/availability-targets/test", availabilityRequestBody(t, created.AvailabilityTarget)))
	if changedOriginTestRec.Code != http.StatusBadRequest {
		t.Fatalf("changed-origin test status = %d, body=%s; want re-entry validation", changedOriginTestRec.Code, changedOriginTestRec.Body.String())
	}
	if changedOriginReached {
		t.Fatal("stored HTTP values were replayed to a changed origin")
	}

	created.Address = server.URL
	created.HTTP.JSONEquals = "ready"
	revisionRec := httptest.NewRecorder()
	handler.HandleUpdate(revisionRec, httptest.NewRequest(http.MethodPut, "/api/availability-targets/contract-target", availabilityRequestBody(t, created.AvailabilityTarget)))
	if revisionRec.Code != http.StatusOK {
		t.Fatalf("contract-edit HandleUpdate status = %d, body=%s", revisionRec.Code, revisionRec.Body.String())
	}
	loaded, err = persistence.LoadAvailabilityTargets()
	if err != nil || len(loaded) != 1 || loaded[0].ConfigRevision != 2 {
		t.Fatalf("contract edit revision = %+v, %v; want revision 2", loaded, err)
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
	if len(listed) < 5 {
		t.Fatalf("expected mock availability targets, got %+v", listed)
	}
	foundMQTT := false
	foundESPHome := false
	for _, target := range listed {
		switch target.ID {
		case "mock-availability-mqtt-meter":
			foundMQTT = true
			if target.Protocol != config.AvailabilityProbeTCP || target.Port != 1883 {
				t.Fatalf("unexpected MQTT target: %+v", target.AvailabilityTarget)
			}
			if target.Status == nil || !target.Status.Available {
				t.Fatalf("expected successful MQTT status, got %+v", target.Status)
			}
		case "mock-availability-esphome-greenhouse":
			foundESPHome = true
			if target.Name != "ESPHome greenhouse sensor" {
				t.Fatalf("unexpected ESPHome target name: %+v", target.AvailabilityTarget)
			}
			if target.Protocol != config.AvailabilityProbeTCP || target.Port != 6053 {
				t.Fatalf("unexpected ESPHome target: %+v", target.AvailabilityTarget)
			}
			if target.Status == nil || !target.Status.Available {
				t.Fatalf("expected successful ESPHome status, got %+v", target.Status)
			}
		}
	}
	if !foundMQTT {
		t.Fatalf("expected MQTT power meter target, got %+v", listed)
	}
	if !foundESPHome {
		t.Fatalf("expected ESPHome greenhouse sensor target, got %+v", listed)
	}
}

func TestAvailabilityHandlersTestSavedMockTargetUsesSyntheticStatus(t *testing.T) {
	previous := mock.IsMockEnabled()
	if err := mock.SetEnabled(true); err != nil {
		t.Fatalf("enable mock mode: %v", err)
	}
	t.Cleanup(func() { _ = mock.SetEnabled(previous) })

	handler := NewAvailabilityHandlers(nil, nil, nil)

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

func TestAvailabilityTargetLinkedResourceIDRoundTrip(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	handler := NewAvailabilityHandlers(
		func(_ context.Context) *config.ConfigPersistence { return persistence },
		nil,
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
		LinkedResourceID: "agent-test-123",
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
	if created.LinkedResourceID != "agent-test-123" {
		t.Fatalf("LinkedResourceID = %q, want %q", created.LinkedResourceID, "agent-test-123")
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/availability-targets/"+created.ID, nil)
	deleteRec := httptest.NewRecorder()
	handler.HandleDelete(deleteRec, deleteReq)

	createBodyNoID := availabilityRequestBody(t, config.AvailabilityTarget{
		Name:             "Energy monitor 2",
		Address:          "device2.local",
		Protocol:         config.AvailabilityProbeICMP,
		Enabled:          true,
		PollIntervalSecs: 30,
		TimeoutMillis:    1000,
		FailureThreshold: 2,
	})
	createReqNoID := httptest.NewRequest(http.MethodPost, "/api/availability-targets", createBodyNoID)
	createRecNoID := httptest.NewRecorder()
	handler.HandleAdd(createRecNoID, createReqNoID)
	if createRecNoID.Code != http.StatusCreated {
		t.Fatalf("HandleAdd status = %d, body=%s", createRecNoID.Code, createRecNoID.Body.String())
	}

	var createdNoID config.AvailabilityTarget
	if err := json.NewDecoder(createRecNoID.Body).Decode(&createdNoID); err != nil {
		t.Fatalf("decode created target: %v", err)
	}
	if createdNoID.LinkedResourceID != "" {
		t.Fatalf("LinkedResourceID = %q, want empty string", createdNoID.LinkedResourceID)
	}
}
