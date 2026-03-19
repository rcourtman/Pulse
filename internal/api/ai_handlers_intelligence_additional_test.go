package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	ur "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
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
	canonicalStore := ur.NewMemoryStore()
	if err := canonicalStore.RecordChange(ur.ResourceChange{
		ID:         "change-summary",
		ObservedAt: time.Now().Add(-20 * time.Minute),
		ResourceID: "vm-summary",
		Kind:       ur.ChangeRestart,
		SourceType: ur.SourcePlatformEvent,
		Reason:     "guest restarted",
	}); err != nil {
		t.Fatalf("record canonical change: %v", err)
	}
	setUnexportedField(t, svc, "resourceExportStore", canonicalStore)
	setUnexportedField(t, svc.GetPatrolService(), "aiService", svc)
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
	recentChanges, ok := payload["recent_changes"].([]interface{})
	if !ok {
		t.Fatalf("expected recent_changes array in summary response, got %T", payload["recent_changes"])
	}
	if len(recentChanges) != 1 {
		t.Fatalf("expected 1 recent change in summary, got %d", len(recentChanges))
	}
}

func TestHandleGetIntelligence_WithResourceID(t *testing.T) {
	t.Parallel()
	svc := newEnabledAIService(t)
	canonicalStore := ur.NewMemoryStore()
	if err := canonicalStore.RecordChange(ur.ResourceChange{
		ID:         "change-api",
		ObservedAt: time.Now().Add(-15 * time.Minute),
		ResourceID: "vm-1",
		Kind:       ur.ChangeConfigUpdate,
		SourceType: ur.SourcePulseDiff,
		Reason:     "configuration drift reconciled",
	}); err != nil {
		t.Fatalf("record canonical change: %v", err)
	}
	setUnexportedField(t, svc, "resourceExportStore", canonicalStore)
	setUnexportedField(t, svc.GetPatrolService(), "aiService", svc)
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
	recentChanges, ok := payload["recent_changes"].([]interface{})
	if !ok {
		t.Fatalf("expected recent_changes array in response, got %T", payload["recent_changes"])
	}
	if len(recentChanges) != 1 {
		t.Fatalf("expected 1 recent change, got %d", len(recentChanges))
	}
}

func TestHandleGetRecentChanges_NoSource(t *testing.T) {
	t.Parallel()
	handler := &AISettingsHandler{defaultAIService: newEnabledAIService(t)}
	req := httptest.NewRequest(http.MethodGet, "/api/ai/intelligence/changes", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetRecentChanges(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["message"] != "Recent changes not initialized" {
		t.Fatalf("unexpected message: %#v", payload["message"])
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
