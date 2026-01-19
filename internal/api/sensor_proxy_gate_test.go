package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestSensorProxyGate(t *testing.T) {
	r := &Router{config: &config.Config{EnableSensorProxy: true}}
	if !r.isSensorProxyEnabled() {
		t.Fatal("expected sensor proxy enabled")
	}

	allowed := r.requireSensorProxyEnabled(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodGet, "/api/sensor", nil)
	resp := httptest.NewRecorder()
	allowed(resp, req)
	if resp.Code != http.StatusNoContent {
		t.Fatalf("unexpected status: %d", resp.Code)
	}

	r.config.EnableSensorProxy = false
	denied := r.requireSensorProxyEnabled(func(http.ResponseWriter, *http.Request) {
		t.Fatal("handler should not be called")
	})
	resp = httptest.NewRecorder()
	denied(resp, req)
	if resp.Code != http.StatusGone {
		t.Fatalf("unexpected status: %d", resp.Code)
	}
	if warning := resp.Header().Get("Warning"); warning == "" {
		t.Fatal("expected Warning header")
	}

	var apiErr APIError
	if err := json.Unmarshal(resp.Body.Bytes(), &apiErr); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if apiErr.Code != "sensor_proxy_disabled" {
		t.Fatalf("unexpected error code: %s", apiErr.Code)
	}
}
