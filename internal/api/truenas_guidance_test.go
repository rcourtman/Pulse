package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/truenas"
)

// Bind the help's distinctions to actual responses without contacting an
// appliance, changing a connection or running an ordinary poll.
func TestTrueNASGuidanceUnavailableIsLocalPersistence(t *testing.T) {
	setTrueNASFeatureForTest(t, true)
	setMockModeForTest(t, false)
	for _, tc := range []struct {
		name    string
		handler *TrueNASHandlers
	}{
		{"nil handler", nil},
		{"missing resolver", &TrueNASHandlers{}},
		{"unavailable request persistence", &TrueNASHandlers{
			getPersistence: func(context.Context) *config.ConfigPersistence { return nil },
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			clientCalls := 0
			if tc.handler != nil {
				tc.handler.newClient = func(truenas.ClientConfig) (trueNASClient, error) {
					clientCalls++
					t.Fatal("local persistence failure must not contact TrueNAS")
					return nil, nil
				}
			}
			for _, action := range []struct {
				name, method, path string
				handle             http.HandlerFunc
			}{
				{"list", http.MethodGet, "/api/truenas/connections", tc.handler.HandleList},
				{"saved test", http.MethodPost, "/api/truenas/connections/example/test", tc.handler.HandleTestSavedConnection},
			} {
				t.Run(action.name, func(t *testing.T) {
					rec := httptest.NewRecorder()
					action.handle(rec, httptest.NewRequest(action.method, action.path, nil))
					assertTrueNASGuidanceError(t, rec, http.StatusInternalServerError, "truenas_unavailable", "TrueNAS service unavailable")
				})
			}
			if clientCalls != 0 {
				t.Fatalf("unexpected appliance clients: %d", clientCalls)
			}
		})
	}
}

func TestTrueNASGuidanceUnavailableRouteIsLocalHandler(t *testing.T) {
	router := &Router{}
	handle := router.platformConnectionItemRoute("truenas_unavailable", "TrueNAS service unavailable", func() *platformConnectionItemHandlers { return nil })
	rec := httptest.NewRecorder()
	handle(rec, httptest.NewRequest(http.MethodPost, "/api/truenas/connections/example/test", nil))
	assertTrueNASGuidanceError(t, rec, http.StatusServiceUnavailable, "truenas_unavailable", "TrueNAS service unavailable")
}

func TestTrueNASGuidanceDisabledIsNotApplianceFailure(t *testing.T) {
	setTrueNASFeatureForTest(t, false)
	handler := &TrueNASHandlers{getPersistence: func(context.Context) *config.ConfigPersistence {
		t.Fatal("explicit opt-out must stop before storage or appliance access")
		return nil
	}}
	rec := httptest.NewRecorder()
	handler.HandleList(rec, httptest.NewRequest(http.MethodGet, "/api/truenas/connections", nil))
	assertTrueNASGuidanceError(t, rec, http.StatusNotFound, "truenas_disabled", "TrueNAS integration has been explicitly disabled")
}

func TestTrueNASGuidanceFailedProbeIsNotServiceUnavailable(t *testing.T) {
	setTrueNASFeatureForTest(t, true)
	clientCalls, probeCalls := 0, 0
	handler := &TrueNASHandlers{newClient: func(truenas.ClientConfig) (trueNASClient, error) {
		clientCalls++
		return &fakeTrueNASClient{testConnection: func(context.Context) error {
			probeCalls++
			return errors.New("synthetic probe failure")
		}}, nil
	}}
	body := marshalTrueNASRequest(t, map[string]any{
		"host": "https://nas.example.test", "apiKey": "synthetic-key", "username": "synthetic-reader",
	})
	rec := httptest.NewRecorder()
	handler.HandleTestConnection(rec, httptest.NewRequest(http.MethodPost, "/api/truenas/connections/test", bytes.NewReader(body)))
	assertTrueNASGuidanceError(t, rec, http.StatusBadRequest, "truenas_connection_failed", "Failed to connect to TrueNAS")
	if clientCalls != 1 || probeCalls != 1 {
		t.Fatalf("expected one synthetic test, got %d clients / %d probes", clientCalls, probeCalls)
	}
}

func assertTrueNASGuidanceError(t *testing.T, rec *httptest.ResponseRecorder, status int, code, message string) {
	t.Helper()
	var response APIError
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if rec.Code != status || response.StatusCode != status || response.Code != code || response.ErrorMessage != message {
		t.Fatalf("got HTTP %d / %+v; expected HTTP %d / %s / %s", rec.Code, response, status, code, message)
	}
	if rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("unexpected response type: %s", rec.Header().Get("Content-Type"))
	}
}
