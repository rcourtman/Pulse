package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestHandleGetIntelligence_PatrolUnavailable(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	handler := &AISettingsHandler{
		legacyAIService: ai.NewService(persistence, nil),
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

func TestHandlePatrolStream_PatrolUnavailable(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	handler := &AISettingsHandler{
		legacyAIService: ai.NewService(persistence, nil),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/ai/patrol/stream", nil)
	rr := httptest.NewRecorder()

	handler.HandlePatrolStream(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rr.Code)
	}
}
