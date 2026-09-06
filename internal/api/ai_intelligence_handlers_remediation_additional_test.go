package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/circuit"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/memory"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestHandleGetCircuitBreakerStatus(t *testing.T) {
	handler := &AISettingsHandler{}

	req := httptest.NewRequest(http.MethodGet, "/api/ai/circuit/status", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetCircuitBreakerStatus(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status OK, got %d", rec.Code)
	}

	breaker := circuit.NewBreaker("patrol", circuit.Config{
		FailureThreshold:  1,
		SuccessThreshold:  1,
		InitialBackoff:    time.Minute,
		MaxBackoff:        time.Minute,
		BackoffMultiplier: 1,
		HalfOpenTimeout:   time.Minute,
	})
	breaker.RecordFailure(context.Canceled)

	handler.SetCircuitBreaker(breaker)
	rec = httptest.NewRecorder()
	handler.HandleGetCircuitBreakerStatus(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status OK, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["state"] != "open" {
		t.Fatalf("expected state open, got %v", resp["state"])
	}
	if resp["can_patrol"].(bool) {
		t.Fatalf("expected can_patrol to be false when breaker is open")
	}
}

func setupIncidentHandler(t *testing.T) (*AISettingsHandler, *memory.IncidentStore) {
	t.Helper()
	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	svc := handler.GetAIService(context.Background())
	svc.SetStateProvider(&MockStateProvider{})
	patrol := svc.GetPatrolService()
	if patrol == nil {
		t.Fatalf("expected patrol service")
	}

	store := memory.NewIncidentStore(memory.IncidentStoreConfig{DataDir: ""})
	patrol.SetIncidentStore(store)

	alert := &alerts.Alert{
		ID:           "alert-1",
		Type:         "cpu",
		Level:        alerts.AlertLevelWarning,
		ResourceID:   "res-1",
		ResourceName: "node-1",
		StartTime:    time.Now(),
		LastSeen:     time.Now(),
	}
	store.RecordAlertFired(alert)

	return handler, store
}

func TestHandleGetRecentIncidents(t *testing.T) {
	handler, _ := setupIncidentHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/incidents?resource_id=res-1&limit=5", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetRecentIncidents(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	incidents := resp["incidents"].([]interface{})
	if len(incidents) != 1 {
		t.Fatalf("expected 1 incident, got %d", len(incidents))
	}
	if resp["active_count"] != nil || resp["active_count_status"] != "not_measured" {
		t.Fatalf("saved incident context must not imply a measured live count: %#v", resp)
	}
}

func TestHandleGetRecentIncidentsSummary(t *testing.T) {
	handler, _ := setupIncidentHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/incidents?limit=5", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetRecentIncidents(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["incident_summary"] == "" {
		t.Fatalf("expected incident_summary to be populated")
	}
}

func TestHandleGetIncidentData(t *testing.T) {
	handler, store := setupIncidentHandler(t)

	alert := &alerts.Alert{
		ID:           "alert-2",
		Type:         "disk",
		Level:        alerts.AlertLevelCritical,
		ResourceID:   "node/pve",
		ResourceName: "node/pve",
		StartTime:    time.Now(),
		LastSeen:     time.Now(),
	}
	store.RecordAlertFired(alert)

	escaped := url.PathEscape("node/pve")
	req := httptest.NewRequest(http.MethodGet, "/api/ai/incidents/"+escaped+"?limit=5", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetIncidentData(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["resource_id"] != "node/pve" {
		t.Fatalf("unexpected resource_id %v", resp["resource_id"])
	}
	incidents := resp["incidents"].([]interface{})
	if len(incidents) == 0 {
		t.Fatalf("expected incidents to be returned")
	}
	if resp["formatted_context"] == "" {
		t.Fatalf("expected formatted_context to be populated")
	}
}

func TestHandleGetRecentIncidentsCountIsNotMeasured(t *testing.T) {
	withMemory, _ := setupIncidentHandler(t)
	for _, tc := range []struct {
		name    string
		handler *AISettingsHandler
		query   string
	}{
		{"unavailable", &AISettingsHandler{}, ""},
		{"fleet context", withMemory, ""},
		{"resource context", withMemory, "?resource_id=res-1"},
		{"empty resource context", withMemory, "?resource_id=absent"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			tc.handler.HandleGetRecentIncidents(rec, httptest.NewRequest(http.MethodGet, "/api/ai/incidents"+tc.query, nil))
			var body map[string]interface{}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			count, present := body["active_count"]
			if !present || count != nil || body["active_count_status"] != "not_measured" {
				t.Fatalf("archive/context presence cannot establish live count: %#v", body)
			}
		})
	}
}
