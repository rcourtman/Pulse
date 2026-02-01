package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/approval"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestAISettingsHandler(cfg *config.Config, persistence *config.ConfigPersistence, agentServer *agentexec.Server) *AISettingsHandler {
	handler := NewAISettingsHandler(nil, nil, agentServer)
	handler.legacyConfig = cfg
	handler.legacyPersistence = persistence
	if persistence != nil {
		handler.legacyAIService = ai.NewService(persistence, agentServer)
		_ = handler.legacyAIService.LoadConfig()
	}
	return handler
}

func TestAISettingsHandler_GetAndUpdateSettings_RoundTrip(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)

	handler := newTestAISettingsHandler(cfg, persistence, nil)

	// GET should return defaults if no config has been saved yet.
	{
		req := httptest.NewRequest(http.MethodGet, "/api/settings/ai", nil)
		rec := httptest.NewRecorder()
		handler.HandleGetAISettings(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("GET status = %d, body=%s", rec.Code, rec.Body.String())
		}

		var resp AISettingsResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp.Enabled {
			t.Fatalf("expected default Enabled=false, got %+v", resp)
		}
	}

	// Update settings to enable AI via Ollama.
	{
		body, _ := json.Marshal(AISettingsUpdateRequest{
			Enabled:       ptr(true),
			Provider:      ptr("ollama"),
			Model:         ptr("ollama:llama3"),
			OllamaBaseURL: ptr("http://localhost:11434"),
		})
		req := httptest.NewRequest(http.MethodPut, "/api/settings/ai", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		handler.HandleUpdateAISettings(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("PUT status = %d, body=%s", rec.Code, rec.Body.String())
		}

		var resp AISettingsResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if !resp.Enabled || !resp.OllamaConfigured {
			t.Fatalf("expected enabled + ollama configured, got %+v", resp)
		}
		if resp.OllamaBaseURL != "http://localhost:11434" {
			t.Fatalf("unexpected ollama base url: %+v", resp)
		}
	}

	// GET again should reflect persisted updates.
	{
		req := httptest.NewRequest(http.MethodGet, "/api/settings/ai", nil)
		rec := httptest.NewRecorder()
		handler.HandleGetAISettings(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("GET status = %d, body=%s", rec.Code, rec.Body.String())
		}

		var resp AISettingsResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if !resp.Enabled || !resp.OllamaConfigured {
			t.Fatalf("expected enabled + ollama configured, got %+v", resp)
		}
	}
}

func TestAISettingsHandler_ListModels_Ollama(t *testing.T) {
	t.Parallel()

	ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"models": []map[string]any{
					{"name": "llama3:latest"},
					{"name": "tinyllama:latest"},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer ollama.Close()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)

	aiCfg := config.NewDefaultAIConfig()
	aiCfg.Enabled = true
	aiCfg.Model = "ollama:llama3"
	aiCfg.OllamaBaseURL = ollama.URL
	if err := persistence.SaveAIConfig(*aiCfg); err != nil {
		t.Fatalf("SaveAIConfig: %v", err)
	}

	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/models", nil)
	rec := httptest.NewRecorder()
	handler.HandleListModels(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Models []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"models"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("unexpected error: %s", resp.Error)
	}
	if len(resp.Models) != 2 {
		t.Fatalf("expected 2 models, got %+v", resp.Models)
	}
}

func TestAISettingsHandler_Execute_Ollama(t *testing.T) {
	t.Parallel()

	ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/chat":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"model":      "llama3",
				"created_at": time.Now().Format(time.RFC3339),
				"message": map[string]any{
					"role":    "assistant",
					"content": "hello from ollama",
				},
				"done":              true,
				"done_reason":       "stop",
				"prompt_eval_count": 3,
				"eval_count":        5,
			})
		case "/api/version":
			_ = json.NewEncoder(w).Encode(map[string]any{"version": "0.1.0"})
		case "/api/tags":
			_ = json.NewEncoder(w).Encode(map[string]any{"models": []map[string]any{{"name": "llama3"}}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer ollama.Close()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)

	aiCfg := config.NewDefaultAIConfig()
	aiCfg.Enabled = true
	aiCfg.Model = "ollama:llama3"
	aiCfg.OllamaBaseURL = ollama.URL
	if err := persistence.SaveAIConfig(*aiCfg); err != nil {
		t.Fatalf("SaveAIConfig: %v", err)
	}

	handler := newTestAISettingsHandler(cfg, persistence, nil)

	body, _ := json.Marshal(AIExecuteRequest{Prompt: "hi"})
	req := httptest.NewRequest(http.MethodPost, "/api/ai/execute", bytes.NewReader(body))
	req = req.WithContext(context.Background())
	rec := httptest.NewRecorder()
	handler.HandleExecute(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp AIExecuteResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Content != "hello from ollama" || resp.Model == "" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func ptr[T any](v T) *T { return &v }

func TestAISettingsHandler_TestConnection_Ollama(t *testing.T) {
	t.Parallel()

	ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/version" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"version": "0.1.0"})
	}))
	defer ollama.Close()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)

	aiCfg := config.NewDefaultAIConfig()
	aiCfg.Enabled = true
	aiCfg.Model = "ollama:llama3"
	aiCfg.OllamaBaseURL = ollama.URL
	if err := persistence.SaveAIConfig(*aiCfg); err != nil {
		t.Fatalf("SaveAIConfig: %v", err)
	}

	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/ai/test", nil)
	rec := httptest.NewRecorder()
	handler.HandleTestAIConnection(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success, got %+v", resp)
	}
}

func TestAISettingsHandler_TestProvider_Ollama(t *testing.T) {
	t.Parallel()

	ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/version" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"version": "0.1.0"})
	}))
	defer ollama.Close()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)

	aiCfg := config.NewDefaultAIConfig()
	aiCfg.Enabled = true
	aiCfg.Model = "ollama:llama3"
	aiCfg.OllamaBaseURL = ollama.URL
	if err := persistence.SaveAIConfig(*aiCfg); err != nil {
		t.Fatalf("SaveAIConfig: %v", err)
	}

	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/ai/test/ollama", nil)
	rec := httptest.NewRecorder()
	handler.HandleTestProvider(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Success  bool   `json:"success"`
		Provider string `json:"provider"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Success || resp.Provider != "ollama" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

// ========================================
// HandleGetAICostSummary tests
// ========================================

func TestHandleGetAICostSummary_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/ai/cost/summary", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetAICostSummary(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleGetAICostSummary_NoAIService(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/cost/summary", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetAICostSummary(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp struct {
		Days        int    `json:"days"`
		PricingAsOf string `json:"pricing_as_of"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Days != 30 {
		t.Fatalf("expected Days=30 default, got %d", resp.Days)
	}
}

func TestHandleGetAICostSummary_CustomDays(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/cost/summary?days=7", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetAICostSummary(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp struct {
		Days int `json:"days"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Days != 7 {
		t.Fatalf("expected Days=7, got %d", resp.Days)
	}
}

func TestHandleGetAICostSummary_MaxDays(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	// Test that days > 365 is capped at 365
	req := httptest.NewRequest(http.MethodGet, "/api/ai/cost/summary?days=1000", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetAICostSummary(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp struct {
		Days int `json:"days"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Days != 365 {
		t.Fatalf("expected Days=365 (capped), got %d", resp.Days)
	}
}

// ========================================
// HandleResetAICostHistory tests
// ========================================

func TestHandleResetAICostHistory_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/cost/reset", nil)
	rec := httptest.NewRecorder()
	handler.HandleResetAICostHistory(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleResetAICostHistory_Success(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/ai/cost/reset", nil)
	rec := httptest.NewRecorder()
	handler.HandleResetAICostHistory(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp struct {
		Ok bool `json:"ok"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Ok {
		t.Fatalf("expected ok=true")
	}
}

// ========================================
// HandleExportAICostHistory tests
// ========================================

func TestHandleExportAICostHistory_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/ai/cost/export", nil)
	rec := httptest.NewRecorder()
	handler.HandleExportAICostHistory(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

// ========================================
// HandleGetSuppressionRules tests
// ========================================

func TestHandleGetSuppressionRules_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/ai/patrol/suppressions", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetSuppressionRules(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

// ========================================
// HandleAddSuppressionRule tests
// ========================================

func TestHandleAddSuppressionRule_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/patrol/suppressions", nil)
	rec := httptest.NewRecorder()
	handler.HandleAddSuppressionRule(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

// ========================================
// HandleDeleteSuppressionRule tests
// ========================================

func TestHandleDeleteSuppressionRule_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/patrol/suppressions/rule-123", nil)
	rec := httptest.NewRecorder()
	handler.HandleDeleteSuppressionRule(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

// ========================================
// HandleGetDismissedFindings tests
// ========================================

func TestHandleGetDismissedFindings_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/ai/patrol/dismissed", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetDismissedFindings(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

// ========================================
// HandleGetGuestKnowledge tests
// ========================================

func TestHandleGetGuestKnowledge_MissingGuestID(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/knowledge", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetGuestKnowledge(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

// ========================================
// HandleSaveGuestNote tests
// ========================================

func TestHandleSaveGuestNote_InvalidBody(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/ai/knowledge", bytes.NewReader([]byte(`{invalid json}`)))
	rec := httptest.NewRecorder()
	handler.HandleSaveGuestNote(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestHandleSaveGuestNote_MissingFields(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	body := []byte(`{"guest_id": "vm-100"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/ai/knowledge", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleSaveGuestNote(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

// ========================================
// HandleDeleteGuestNote tests
// ========================================

func TestHandleDeleteGuestNote_InvalidBody(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/ai/knowledge/delete", bytes.NewReader([]byte(`{invalid json}`)))
	rec := httptest.NewRecorder()
	handler.HandleDeleteGuestNote(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestHandleDeleteGuestNote_MissingFields(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	body := []byte(`{"guest_id": "vm-100"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/ai/knowledge/delete", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleDeleteGuestNote(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

// ========================================
// HandleClearGuestKnowledge tests
// ========================================

func TestHandleClearGuestKnowledge_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/knowledge/clear", nil)
	rec := httptest.NewRecorder()
	handler.HandleClearGuestKnowledge(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleClearGuestKnowledge_MissingGuestID(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	body := []byte(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/api/ai/knowledge/clear", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleClearGuestKnowledge(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

// ========================================
// HandleDebugContext tests
// ========================================

func TestHandleDebugContext_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/ai/debug/context", nil)
	rec := httptest.NewRecorder()
	handler.HandleDebugContext(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

// ========================================
// HandleGetConnectedAgents tests
// ========================================

func TestHandleGetConnectedAgents_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/ai/agents", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetConnectedAgents(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleGetConnectedAgents_NoAgentServer(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	// handler created with nil agentServer
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/agents", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetConnectedAgents(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp struct {
		Count int    `json:"count"`
		Note  string `json:"note"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Count != 0 {
		t.Fatalf("expected count=0, got %d", resp.Count)
	}
	if resp.Note == "" {
		t.Fatalf("expected note to be present")
	}
}

// ========================================
// HandleRunCommand tests
// ========================================

func TestHandleRunCommand_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/run-command", nil)
	rec := httptest.NewRecorder()
	handler.HandleRunCommand(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleRunCommand_InvalidBody(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/ai/run-command", bytes.NewReader([]byte(`{invalid json}`)))
	rec := httptest.NewRecorder()
	handler.HandleRunCommand(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

// ========================================
// HandleAnalyzeKubernetesCluster tests
// ========================================

func TestHandleAnalyzeKubernetesCluster_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/kubernetes/analyze", nil)
	rec := httptest.NewRecorder()
	handler.HandleAnalyzeKubernetesCluster(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleAnalyzeKubernetesCluster_InvalidBody(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/ai/kubernetes/analyze", bytes.NewReader([]byte(`{invalid json}`)))
	rec := httptest.NewRecorder()
	handler.HandleAnalyzeKubernetesCluster(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

// ========================================
// HandleInvestigateAlert tests
// ========================================

func TestHandleInvestigateAlert_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/investigate", nil)
	rec := httptest.NewRecorder()
	handler.HandleInvestigateAlert(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleInvestigateAlert_InvalidBody(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/ai/investigate", bytes.NewReader([]byte(`{invalid json}`)))
	rec := httptest.NewRecorder()
	handler.HandleInvestigateAlert(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestHandleInvestigateAlert_MissingAlertID(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	body := []byte(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/api/ai/investigate", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleInvestigateAlert(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

// ========================================
// AISettingsHandler setter method tests
// ========================================

func TestAISettingsHandler_SetConfig(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	// SetConfig with nil should be a no-op
	handler.SetConfig(nil)

	// SetConfig with new config should update the handler's config
	newCfg := &config.Config{DataPath: tmp}
	handler.SetConfig(newCfg)
	// No assertion needed - just verifying it doesn't panic
}

func TestAISettingsHandler_StopPatrol(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	// StopPatrol should be safe to call even when patrol is not running
	handler.StopPatrol()
	// No assertion needed - just verifying it doesn't panic
}

func TestAISettingsHandler_GetAlertTriggeredAnalyzer(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	// Should return the analyzer (may be nil if not initialized)
	analyzer := handler.GetAlertTriggeredAnalyzer(context.Background())
	// Just verify it doesn't panic and returns something
	_ = analyzer
}

func TestAISettingsHandler_StartPatrol(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	// Start patrol with a cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	handler.StartPatrol(ctx)

	// Give it a brief moment then stop
	time.Sleep(10 * time.Millisecond)
	cancel()
	handler.StopPatrol()
}

func TestAISettingsHandler_SetPatrolFindingsPersistence(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	// Set nil persistence should not panic
	err := handler.SetPatrolFindingsPersistence(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAISettingsHandler_SetPatrolRunHistoryPersistence(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	// Set nil persistence should not panic
	err := handler.SetPatrolRunHistoryPersistence(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAISettingsHandler_SetPatrolThresholdProvider(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	// Set nil threshold provider should not panic
	handler.SetPatrolThresholdProvider(nil)
}

func TestAISettingsHandler_SetMetricsHistoryProvider(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	// Set nil metrics provider should not panic
	handler.SetMetricsHistoryProvider(nil)
}

func TestAISettingsHandler_SetBaselineStore(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	// Set nil baseline store should not panic
	handler.SetBaselineStore(nil)
}

func TestAISettingsHandler_SetChangeDetector(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	// Set nil change detector should not panic
	handler.SetChangeDetector(nil)
}

func TestAISettingsHandler_SetRemediationLog(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	// Set nil remediation log should not panic
	handler.SetRemediationLog(nil)
}

func TestAISettingsHandler_SetPatternDetector(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	// Set nil pattern detector should not panic
	handler.SetPatternDetector(nil)
}

func TestAISettingsHandler_SetCorrelationDetector(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	// Set nil correlation detector should not panic
	handler.SetCorrelationDetector(nil)
}
func TestAISettingsHandler_Approvals(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	// Initialize approval store
	approvalStore, _ := approval.NewStore(approval.StoreConfig{
		DataDir:            tmp,
		DisablePersistence: true,
	})
	approval.SetStore(approvalStore)

	appID := "app-123"
	approvalStore.CreateApproval(&approval.ApprovalRequest{
		ID:      appID,
		Command: "ls -la",
		Status:  approval.StatusPending,
	})

	t.Run("HandleGetApproval", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/ai/approvals/"+appID, nil)
		rec := httptest.NewRecorder()
		handler.HandleGetApproval(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var resp approval.ApprovalRequest
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, appID, resp.ID)
	})

	t.Run("HandleApproveCommand", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/ai/approvals/"+appID+"/approve", nil)
		rec := httptest.NewRecorder()
		handler.HandleApproveCommand(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		app, _ := approvalStore.GetApproval(appID)
		assert.Equal(t, approval.StatusApproved, app.Status)
	})

	t.Run("HandleDenyCommand", func(t *testing.T) {
		appID2 := "app-456"
		approvalStore.CreateApproval(&approval.ApprovalRequest{
			ID:      appID2,
			Command: "rm -rf /",
			Status:  approval.StatusPending,
		})

		body, _ := json.Marshal(map[string]string{"reason": "too dangerous"})
		req := httptest.NewRequest(http.MethodPost, "/api/ai/approvals/"+appID2+"/deny", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		handler.HandleDenyCommand(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		app, _ := approvalStore.GetApproval(appID2)
		assert.Equal(t, approval.StatusDenied, app.Status)
		assert.Equal(t, "too dangerous", app.DenyReason)
	})
}
func TestAISettingsHandler_ChatSessions(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	t.Run("HandleSaveAndGetSession", func(t *testing.T) {
		sessionID := "sess-123"
		body, _ := json.Marshal(map[string]interface{}{
			"id":    sessionID,
			"title": "Test Session",
		})
		req := httptest.NewRequest(http.MethodPut, "/api/ai/chat/sessions/"+sessionID, bytes.NewReader(body))
		rec := httptest.NewRecorder()
		handler.HandleSaveAIChatSession(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		// GET session
		req = httptest.NewRequest(http.MethodGet, "/api/ai/chat/sessions/"+sessionID, nil)
		rec = httptest.NewRecorder()
		handler.HandleGetAIChatSession(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("HandleListSessions", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/ai/chat/sessions", nil)
		rec := httptest.NewRecorder()
		handler.HandleListAIChatSessions(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("HandleDeleteSession", func(t *testing.T) {
		sessionID := "sess-delete"
		body, _ := json.Marshal(map[string]interface{}{"id": sessionID})
		handler.HandleSaveAIChatSession(httptest.NewRecorder(), httptest.NewRequest(http.MethodPut, "/api/ai/chat/sessions/"+sessionID, bytes.NewReader(body)))

		req := httptest.NewRequest(http.MethodDelete, "/api/ai/chat/sessions/"+sessionID, nil)
		rec := httptest.NewRecorder()
		handler.HandleDeleteAIChatSession(rec, req)
		assert.Equal(t, http.StatusNoContent, rec.Code)
	})
}
