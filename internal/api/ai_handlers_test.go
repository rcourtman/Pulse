package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestAISettingsHandler_GetAndUpdateSettings_RoundTrip(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)

	handler := NewAISettingsHandler(cfg, persistence, nil)

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
			Enabled:      ptr(true),
			Provider:     ptr("ollama"),
			Model:        ptr("ollama:llama3"),
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

	handler := NewAISettingsHandler(cfg, persistence, nil)

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
				"done":             true,
				"done_reason":      "stop",
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

	handler := NewAISettingsHandler(cfg, persistence, nil)

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

	handler := NewAISettingsHandler(cfg, persistence, nil)

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

	handler := NewAISettingsHandler(cfg, persistence, nil)

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
