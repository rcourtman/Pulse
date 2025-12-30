package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// createTestAIHandler creates a test AI handler with minimal setup
func createTestAIHandler(t *testing.T) *AISettingsHandler {
	t.Helper()
	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	return NewAISettingsHandler(cfg, persistence, nil)
}

// TestHandleGetPatterns tests the patterns endpoint
func TestHandleGetPatterns_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	handler := createTestAIHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/ai/intelligence/patterns", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetPatterns(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d for POST, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleGetPatterns_NoPatrolService(t *testing.T) {
	t.Parallel()
	handler := createTestAIHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/intelligence/patterns", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetPatterns(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	// Should return empty patterns array with message
	patterns, ok := resp["patterns"].([]interface{})
	if !ok {
		t.Fatal("expected patterns array in response")
	}
	if len(patterns) != 0 {
		t.Fatalf("expected empty patterns, got %d", len(patterns))
	}
	if resp["message"] != "AI is not enabled" {
		t.Fatalf("unexpected message: %v", resp["message"])
	}
}

func TestHandleGetPatterns_ResourceIDFilter(t *testing.T) {
	t.Parallel()
	handler := createTestAIHandler(t)

	// Test with resource_id query parameter - should still return gracefully
	req := httptest.NewRequest(http.MethodGet, "/api/ai/intelligence/patterns?resource_id=vm-100", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetPatterns(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	patterns, ok := resp["patterns"].([]interface{})
	if !ok {
		t.Fatal("expected patterns array in response")
	}
	if len(patterns) != 0 {
		t.Fatalf("expected empty patterns for non-initialized service, got %d", len(patterns))
	}
}

// TestHandleGetPredictions tests the predictions endpoint
func TestHandleGetPredictions_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	handler := createTestAIHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/ai/intelligence/predictions", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetPredictions(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d for POST, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleGetPredictions_NoPatrolService(t *testing.T) {
	t.Parallel()
	handler := createTestAIHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/intelligence/predictions", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetPredictions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	predictions, ok := resp["predictions"].([]interface{})
	if !ok {
		t.Fatal("expected predictions array in response")
	}
	if len(predictions) != 0 {
		t.Fatalf("expected empty predictions, got %d", len(predictions))
	}
	if resp["message"] != "AI is not enabled" {
		t.Fatalf("unexpected message: %v", resp["message"])
	}
}

func TestHandleGetPredictions_ResourceIDFilter(t *testing.T) {
	t.Parallel()
	handler := createTestAIHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/intelligence/predictions?resource_id=vm-200", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetPredictions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	// Verify response is valid JSON with expected structure
	if _, ok := resp["predictions"]; !ok {
		t.Fatal("expected predictions field in response")
	}
}

// TestHandleGetCorrelations tests the correlations endpoint
func TestHandleGetCorrelations_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	handler := createTestAIHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/ai/intelligence/correlations", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetCorrelations(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d for POST, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleGetCorrelations_NoPatrolService(t *testing.T) {
	t.Parallel()
	handler := createTestAIHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/intelligence/correlations", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetCorrelations(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	correlations, ok := resp["correlations"].([]interface{})
	if !ok {
		t.Fatal("expected correlations array in response")
	}
	if len(correlations) != 0 {
		t.Fatalf("expected empty correlations, got %d", len(correlations))
	}
	if resp["message"] != "AI is not enabled" {
		t.Fatalf("unexpected message: %v", resp["message"])
	}
}

func TestHandleGetCorrelations_ResourceIDFilter(t *testing.T) {
	t.Parallel()
	handler := createTestAIHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/intelligence/correlations?resource_id=storage-1", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetCorrelations(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if _, ok := resp["correlations"]; !ok {
		t.Fatal("expected correlations field in response")
	}
}

// TestHandleGetRecentChanges tests the changes endpoint
func TestHandleGetRecentChanges_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	handler := createTestAIHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/ai/intelligence/changes", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetRecentChanges(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d for POST, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleGetRecentChanges_NoPatrolService(t *testing.T) {
	t.Parallel()
	handler := createTestAIHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/intelligence/changes", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetRecentChanges(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	changes, ok := resp["changes"].([]interface{})
	if !ok {
		t.Fatal("expected changes array in response")
	}
	if len(changes) != 0 {
		t.Fatalf("expected empty changes, got %d", len(changes))
	}
	if resp["message"] != "AI is not enabled" {
		t.Fatalf("unexpected message: %v", resp["message"])
	}
}

// TestHandleGetRemediations tests the remediations endpoint
func TestHandleGetRemediations_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	handler := createTestAIHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/ai/intelligence/remediations", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetRemediations(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d for POST, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleGetRemediations_NoPatrolService(t *testing.T) {
	t.Parallel()
	handler := createTestAIHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/intelligence/remediations", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetRemediations(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	remediations, ok := resp["remediations"].([]interface{})
	if !ok {
		t.Fatal("expected remediations array in response")
	}
	if len(remediations) != 0 {
		t.Fatalf("expected empty remediations, got %d", len(remediations))
	}
	if resp["message"] != "AI is not enabled" {
		t.Fatalf("unexpected message: %v", resp["message"])
	}
}

// TestHandleGetBaselines tests the baselines endpoint
func TestHandleGetBaselines_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	handler := createTestAIHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/ai/intelligence/baselines", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetBaselines(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d for POST, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleGetBaselines_NoPatrolService(t *testing.T) {
	t.Parallel()
	handler := createTestAIHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/intelligence/baselines", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetBaselines(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	baselines, ok := resp["baselines"].([]interface{})
	if !ok {
		t.Fatal("expected baselines array in response")
	}
	if len(baselines) != 0 {
		t.Fatalf("expected empty baselines, got %d", len(baselines))
	}
	if resp["message"] != "AI is not enabled" {
		t.Fatalf("unexpected message: %v", resp["message"])
	}
}

func TestHandleGetBaselines_ResourceIDFilter(t *testing.T) {
	t.Parallel()
	handler := createTestAIHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/intelligence/baselines?resource_id=node-1", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetBaselines(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if _, ok := resp["baselines"]; !ok {
		t.Fatal("expected baselines field in response")
	}
}
