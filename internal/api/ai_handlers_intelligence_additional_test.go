package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestHandleGetIntelligence_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	handler := &AISettingsHandler{}

	req := httptest.NewRequest(http.MethodPost, "/api/ai/intelligence", nil)
	rr := httptest.NewRecorder()

	handler.HandleGetIntelligence(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandleGetIntelligence_NilAIService(t *testing.T) {
	t.Parallel()
	// handler with no defaultAIService — GetAIService returns nil
	handler := &AISettingsHandler{}

	req := httptest.NewRequest(http.MethodGet, "/api/ai/intelligence", nil)
	rr := httptest.NewRecorder()

	handler.HandleGetIntelligence(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rr.Code)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if payload["error"] != "Pulse Patrol service not available" {
		t.Fatalf("error = %v, want Pulse Patrol service not available", payload["error"])
	}
}

func TestHandleGetIntelligence_PatrolUnavailable(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	handler := &AISettingsHandler{
		defaultAIService: ai.NewService(persistence, nil),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/ai/intelligence", nil)
	rr := httptest.NewRecorder()

	handler.HandleGetIntelligence(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rr.Code)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if payload["error"] != "Pulse Patrol service not available" {
		t.Fatalf("error = %v, want Pulse Patrol service not available", payload["error"])
	}
}

func TestHandleGetIntelligence_ResourceIDTooLong(t *testing.T) {
	t.Parallel()
	svc := newEnabledAIService(t)
	handler := &AISettingsHandler{defaultAIService: svc}

	longID := strings.Repeat("x", 501)
	req := httptest.NewRequest(http.MethodGet, "/api/ai/intelligence?resource_id="+longID, nil)
	rr := httptest.NewRecorder()

	handler.HandleGetIntelligence(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleGetIntelligence_WithPatrolService(t *testing.T) {
	t.Parallel()
	svc := newEnabledAIService(t)
	handler := &AISettingsHandler{defaultAIService: svc}

	req := httptest.NewRequest(http.MethodGet, "/api/ai/intelligence", nil)
	rr := httptest.NewRecorder()

	handler.HandleGetIntelligence(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	// Should contain timestamp from GetSummary
	if _, ok := payload["timestamp"]; !ok {
		t.Fatalf("expected timestamp in intelligence summary, got %v", payload)
	}
}

func TestHandleGetIntelligence_WithResourceID(t *testing.T) {
	t.Parallel()
	svc := newEnabledAIService(t)
	handler := &AISettingsHandler{defaultAIService: svc}

	req := httptest.NewRequest(http.MethodGet, "/api/ai/intelligence?resource_id=vm-1", nil)
	rr := httptest.NewRecorder()

	handler.HandleGetIntelligence(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if payload["resource_id"] != "vm-1" {
		t.Fatalf("expected resource_id=vm-1 in response, got %v", payload["resource_id"])
	}
}

func TestHandleGetIntelligence_ResourceIDAtLimit(t *testing.T) {
	t.Parallel()
	svc := newEnabledAIService(t)
	handler := &AISettingsHandler{defaultAIService: svc}

	// Exactly 500 chars should be accepted
	exactID := strings.Repeat("a", 500)
	req := httptest.NewRequest(http.MethodGet, "/api/ai/intelligence?resource_id="+exactID, nil)
	rr := httptest.NewRecorder()

	handler.HandleGetIntelligence(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d for 500-char resource_id", rr.Code, http.StatusOK)
	}
}

func TestHandlePatrolStream_PatrolUnavailable(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	handler := &AISettingsHandler{
		defaultAIService: ai.NewService(persistence, nil),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/ai/patrol/stream", nil)
	rr := httptest.NewRecorder()

	handler.HandlePatrolStream(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rr.Code)
	}
}
