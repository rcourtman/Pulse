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
	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockAlertAnalyzerForTest satisfies aicontracts.AlertAnalyzer for test assertions.
type mockAlertAnalyzerForTest struct{ enabled bool }

func (m *mockAlertAnalyzerForTest) OnAlertFired(aicontracts.AlertPayload) {}
func (m *mockAlertAnalyzerForTest) SetEnabled(e bool)                     { m.enabled = e }
func (m *mockAlertAnalyzerForTest) IsEnabled() bool                       { return m.enabled }
func (m *mockAlertAnalyzerForTest) Start()                                {}
func (m *mockAlertAnalyzerForTest) Stop()                                 {}

func newTestAISettingsHandler(cfg *config.Config, persistence *config.ConfigPersistence, agentServer *agentexec.Server) *AISettingsHandler {
	handler := NewAISettingsHandler(nil, nil, agentServer)
	handler.defaultConfig = cfg
	handler.defaultPersistence = persistence
	if persistence != nil {
		handler.defaultAIService = ai.NewService(persistence, agentServer)
		_ = handler.defaultAIService.LoadConfig()
	}
	return handler
}

func saveEnabledTestAIConfig(t *testing.T, persistence *config.ConfigPersistence) {
	t.Helper()

	aiCfg := config.NewDefaultAIConfig()
	aiCfg.Enabled = true
	aiCfg.Model = "ollama:llama3"
	aiCfg.OllamaBaseURL = "http://127.0.0.1:11434"
	require.NoError(t, persistence.SaveAIConfig(*aiCfg))
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

func TestAISettingsHandler_UpdateSettings_OpenRouterKeySetAndClear(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	// Set OpenRouter key.
	{
		body, _ := json.Marshal(AISettingsUpdateRequest{
			OpenRouterAPIKey: ptr("  sk-or-test-key  "),
			Model:            ptr("openrouter:openai/gpt-4o-mini"),
		})
		req := httptest.NewRequest(http.MethodPut, "/api/settings/ai/update", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		handler.HandleUpdateAISettings(rec, req)

		require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())

		var resp AISettingsResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		assert.True(t, resp.OpenRouterConfigured)
		assert.Contains(t, resp.ConfiguredProviders, config.AIProviderOpenRouter)

		saved, err := persistence.LoadAIConfig()
		require.NoError(t, err)
		require.NotNil(t, saved)
		assert.Equal(t, "sk-or-test-key", saved.OpenRouterAPIKey)
	}

	// Clear OpenRouter key.
	{
		body, _ := json.Marshal(AISettingsUpdateRequest{
			ClearOpenRouterKey: ptr(true),
		})
		req := httptest.NewRequest(http.MethodPut, "/api/settings/ai/update", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		handler.HandleUpdateAISettings(rec, req)

		require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())

		var resp AISettingsResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		assert.False(t, resp.OpenRouterConfigured)
		assert.NotContains(t, resp.ConfiguredProviders, config.AIProviderOpenRouter)

		saved, err := persistence.LoadAIConfig()
		require.NoError(t, err)
		require.NotNil(t, saved)
		assert.Empty(t, saved.OpenRouterAPIKey)
	}
}

func TestAISettingsHandler_ListModels_Ollama(t *testing.T) {
	t.Parallel()

	ollama := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	ollama := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

func TestHandleExecute_RejectsLegacyHostTargetType(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	saveEnabledTestAIConfig(t, persistence)

	handler := newTestAISettingsHandler(cfg, persistence, nil)

	body := []byte(`{"prompt":"hi","target_type":"host","target_id":"agent-1"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/ai/execute", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleExecute(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "Invalid target_type")
}

func TestHandleExecuteStream_RejectsLegacyHostTargetType(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	saveEnabledTestAIConfig(t, persistence)

	handler := newTestAISettingsHandler(cfg, persistence, nil)

	body := []byte(`{"prompt":"hi","target_type":"host","target_id":"agent-1"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/ai/execute/stream", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleExecuteStream(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "Invalid target_type")
}

func TestNormalizeAIExecuteTargetType_StrictCanonicalV6(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "system-container", want: "system-container"},
		{in: "SYSTEM-CONTAINER", want: "system-container"},
		{in: "container", want: "container"},
		{in: "system_container", want: "system_container"},
	}

	for _, tt := range tests {
		got := normalizeAIExecuteTargetType(tt.in)
		if got != tt.want {
			t.Fatalf("normalizeAIExecuteTargetType(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestNormalizeAndValidateAIExecuteTargetType_StrictCanonicalV6(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr string
	}{
		{name: "empty allowed", in: "", want: ""},
		{name: "agent", in: "agent", want: "agent"},
		{name: "system container canonical", in: "SYSTEM-CONTAINER", want: "system-container"},
		{name: "vm", in: "vm", want: "vm"},
		{name: "legacy host rejected", in: "host", wantErr: "invalid target_type"},
		{name: "legacy container rejected", in: "container", wantErr: "invalid target_type"},
		{name: "legacy underscore container rejected", in: "system_container", wantErr: "invalid target_type"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeAndValidateAIExecuteTargetType(tt.in)
			if tt.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestNormalizeInvestigateAlertTargetType_StrictCanonicalV6(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr string
	}{
		{name: "vm", in: "vm", want: "vm"},
		{name: "system container", in: "system-container", want: "system-container"},
		{name: "oci container maps to system-container", in: "oci-container", want: "system-container"},
		{name: "app container maps to agent", in: "app-container", want: "agent"},
		{name: "pod maps to agent", in: "pod", want: "agent"},
		{name: "k8s cluster maps to agent", in: "k8s-cluster", want: "agent"},
		{name: "node maps to agent", in: "node", want: "agent"},
		{name: "legacy host rejected", in: "host", wantErr: "unsupported resource_type"},
		{name: "legacy guest rejected", in: "guest", wantErr: "unsupported resource_type"},
		{name: "legacy docker rejected", in: "docker", wantErr: "unsupported resource_type"},
		{name: "legacy container rejected", in: "container", wantErr: "unsupported resource_type"},
		{name: "legacy k8s alias rejected", in: "k8s", wantErr: "unsupported resource_type"},
		{name: "missing type", in: "", wantErr: "resource_type is required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeInvestigateAlertTargetType(tt.in)
			if tt.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func ptr[T any](v T) *T { return &v }

func TestAISettingsHandler_TestConnection_Ollama(t *testing.T) {
	t.Parallel()

	ollama := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	ollama := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
// HandleTestAIConnection edge-case tests
// ========================================

func TestAISettingsHandler_TestConnection_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/test", nil)
	rec := httptest.NewRecorder()
	handler.HandleTestAIConnection(rec, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

func TestAISettingsHandler_TestConnection_Failure(t *testing.T) {
	t.Parallel()

	// Ollama server that always returns 500 for /api/version
	ollama := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
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

	// Should still return 200 with success=false (not an HTTP error)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.False(t, resp.Success)
	assert.Equal(t, "Connection test failed", resp.Message)
}

func TestAISettingsHandler_TestConnection_NoConfig(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)

	// Don't save any AI config — service will have no configured provider
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/ai/test", nil)
	rec := httptest.NewRecorder()
	handler.HandleTestAIConnection(rec, req)

	// Should return 200 with success=false (connection test fails gracefully)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.False(t, resp.Success)
	assert.Equal(t, "Connection test failed", resp.Message)
}

// ========================================
// HandleTestProvider edge-case tests
// ========================================

func TestAISettingsHandler_TestProvider_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/test/ollama", nil)
	rec := httptest.NewRecorder()
	handler.HandleTestProvider(rec, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

func TestAISettingsHandler_TestProvider_NotConfigured(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)

	// Configure Ollama only — so "openai" is unconfigured
	aiCfg := config.NewDefaultAIConfig()
	aiCfg.Enabled = true
	aiCfg.Model = "ollama:llama3"
	aiCfg.OllamaBaseURL = "http://192.0.2.1:11434" // TEST-NET, non-routable
	if err := persistence.SaveAIConfig(*aiCfg); err != nil {
		t.Fatalf("SaveAIConfig: %v", err)
	}

	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/ai/test/openai", nil)
	rec := httptest.NewRecorder()
	handler.HandleTestProvider(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Success  bool   `json:"success"`
		Message  string `json:"message"`
		Provider string `json:"provider"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.False(t, resp.Success)
	assert.Equal(t, "Provider not configured", resp.Message)
	assert.Equal(t, "openai", resp.Provider)
}

func TestAISettingsHandler_TestProvider_NoAIConfig(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)

	// No AI config file saved — LoadConfig returns a default config (not nil),
	// so the handler falls through to the HasProvider check which returns false
	// for the default (empty) config.
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/ai/test/ollama", nil)
	rec := httptest.NewRecorder()
	handler.HandleTestProvider(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Success  bool   `json:"success"`
		Message  string `json:"message"`
		Provider string `json:"provider"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.False(t, resp.Success)
	assert.Equal(t, "Provider not configured", resp.Message)
	assert.Equal(t, "ollama", resp.Provider)
}

func TestAISettingsHandler_TestProvider_ConnectionFailure(t *testing.T) {
	t.Parallel()

	// Server that returns 500 for all requests
	ollama := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
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

	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Success  bool   `json:"success"`
		Message  string `json:"message"`
		Provider string `json:"provider"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.False(t, resp.Success)
	assert.Equal(t, "Connection test failed", resp.Message)
	assert.Equal(t, "ollama", resp.Provider)
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

func TestHandleGetAICostSummary_NegativeDays(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/cost/summary?days=-5", nil)
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
	if resp.Days != 30 {
		t.Fatalf("expected Days=30 (default for negative), got %d", resp.Days)
	}
}

func TestHandleGetAICostSummary_ZeroDays(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/cost/summary?days=0", nil)
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
	if resp.Days != 30 {
		t.Fatalf("expected Days=30 (default for zero), got %d", resp.Days)
	}
}

func TestHandleGetAICostSummary_NonNumericDays(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/cost/summary?days=abc", nil)
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
	if resp.Days != 30 {
		t.Fatalf("expected Days=30 (default for non-numeric), got %d", resp.Days)
	}
}

func TestHandleGetAICostSummary_ResponseShape(t *testing.T) {
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

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	requiredFields := []string{"days", "pricing_as_of", "provider_models", "use_cases", "targets", "daily_totals", "totals"}
	for _, field := range requiredFields {
		if _, ok := resp[field]; !ok {
			t.Errorf("missing required field %q in response", field)
		}
	}

	// Verify all array fields are arrays (not null)
	arrayFields := []string{"provider_models", "use_cases", "targets", "daily_totals"}
	for _, field := range arrayFields {
		v, ok := resp[field]
		if !ok {
			t.Fatalf("missing %s", field)
		}
		if _, ok := v.([]interface{}); !ok {
			t.Fatalf("expected %s to be array, got %T", field, v)
		}
	}
}

func TestHandleGetAICostSummary_NoService_InvalidDays(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{DataPath: t.TempDir()}
	// Pass nil persistence so defaultAIService is nil — exercises the no-service stub path
	handler := newTestAISettingsHandler(cfg, nil, nil)

	tests := []struct {
		name  string
		query string
		want  int
	}{
		{"negative", "?days=-5", 30},
		{"zero", "?days=0", 30},
		{"non-numeric", "?days=abc", 30},
		{"over-max", "?days=1000", 365},
		{"valid", "?days=7", 7},
		{"default", "", 30},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/ai/cost/summary"+tt.query, nil)
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
			if resp.Days != tt.want {
				t.Fatalf("expected Days=%d, got %d", tt.want, resp.Days)
			}
		})
	}
}

func TestHandleGetAICostSummary_NoService_ResponseShape(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{DataPath: t.TempDir()}
	// nil persistence → nil defaultAIService → exercises no-service stub
	handler := newTestAISettingsHandler(cfg, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/cost/summary", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetAICostSummary(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp struct {
		Days           int                    `json:"days"`
		RetentionDays  int                    `json:"retention_days"`
		EffectiveDays  int                    `json:"effective_days"`
		Truncated      bool                   `json:"truncated"`
		PricingAsOf    string                 `json:"pricing_as_of"`
		ProviderModels []interface{}          `json:"provider_models"`
		UseCases       []interface{}          `json:"use_cases"`
		Targets        []interface{}          `json:"targets"`
		DailyTotals    []interface{}          `json:"daily_totals"`
		Totals         map[string]interface{} `json:"totals"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.Days != 30 {
		t.Errorf("expected Days=30, got %d", resp.Days)
	}
	if resp.RetentionDays != 365 {
		t.Errorf("expected RetentionDays=365, got %d", resp.RetentionDays)
	}
	if resp.EffectiveDays != 30 {
		t.Errorf("expected EffectiveDays=30, got %d", resp.EffectiveDays)
	}
	if resp.Truncated {
		t.Errorf("expected Truncated=false, got true")
	}

	// All array fields must be non-null arrays in the stub response
	if resp.ProviderModels == nil {
		t.Error("provider_models is null, expected empty array")
	}
	if resp.UseCases == nil {
		t.Error("use_cases is null, expected empty array")
	}
	if resp.Targets == nil {
		t.Error("targets is null, expected empty array")
	}
	if resp.DailyTotals == nil {
		t.Error("daily_totals is null, expected empty array")
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
// HandleGetGuestKnowledge / HandleSaveGuestNote / HandleDeleteGuestNote / HandleClearGuestKnowledge
// tests have been consolidated into ai_handlers_knowledge_test.go
// ========================================

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

func TestNormalizeRunCommandApprovalTarget_CanonicalizesAgentTarget(t *testing.T) {
	tests := []struct {
		name    string
		req     AIRunCommandRequest
		wantTyp string
		wantID  string
		wantErr string
	}{
		{
			name: "host target type rejected",
			req: AIRunCommandRequest{
				TargetType: "host",
				TargetID:   "Node-1",
			},
			wantErr: "unsupported target_type",
		},
		{
			name: "run_on_host forces agent target",
			req: AIRunCommandRequest{
				TargetType: "vm",
				RunOnHost:  true,
				TargetHost: "PVE-A",
			},
			wantTyp: "agent",
			wantID:  "pve-a",
		},
		{
			name: "system-container target preserved",
			req: AIRunCommandRequest{
				TargetType: "system-container",
				TargetID:   "201",
			},
			wantTyp: "system-container",
			wantID:  "201",
		},
		{
			name: "legacy container alias rejected",
			req: AIRunCommandRequest{
				TargetType: "container",
				TargetID:   "201",
			},
			wantErr: "unsupported target_type",
		},
		{
			name: "missing target host when run_on_host",
			req: AIRunCommandRequest{
				RunOnHost: true,
			},
			wantErr: "target_host is required when run_on_host is true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotID, err := normalizeRunCommandApprovalTarget(tt.req)
			if tt.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantTyp, gotType)
			require.Equal(t, tt.wantID, gotID)
		})
	}
}

func TestHandleRunCommand_RequiresApprovalID(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	body := []byte(`{"command":"uptime","target_type":"vm","target_id":"vm-101","run_on_host":false}`)
	req := httptest.NewRequest(http.MethodPost, "/api/ai/run-command", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleRunCommand(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestHandleRunCommand_ConsumesApproval(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, agentexec.NewServer(nil))

	store, err := approval.NewStore(approval.StoreConfig{
		DataDir:            tmp,
		DisablePersistence: true,
	})
	require.NoError(t, err)
	approval.SetStore(store)
	defer approval.SetStore(nil)

	appReq := &approval.ApprovalRequest{
		ID:         "approval-1",
		Command:    "uptime",
		TargetType: "vm",
		TargetID:   "vm-101",
	}
	require.NoError(t, store.CreateApproval(appReq))
	_, err = store.Approve(appReq.ID, "tester")
	require.NoError(t, err)

	body := []byte(`{"approval_id":"approval-1","command":"uptime","target_type":"vm","target_id":"vm-101","run_on_host":false}`)
	req := httptest.NewRequest(http.MethodPost, "/api/ai/run-command", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleRunCommand(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())

	stored, found := store.GetApproval(appReq.ID)
	require.True(t, found)
	require.True(t, stored.Consumed)
}

func TestHandleRunCommand_RejectsCommandMismatch(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, agentexec.NewServer(nil))

	store, err := approval.NewStore(approval.StoreConfig{
		DataDir:            tmp,
		DisablePersistence: true,
	})
	require.NoError(t, err)
	approval.SetStore(store)
	defer approval.SetStore(nil)

	appReq := &approval.ApprovalRequest{
		ID:         "approval-2",
		Command:    "uptime",
		TargetType: "vm",
		TargetID:   "vm-101",
	}
	require.NoError(t, store.CreateApproval(appReq))
	_, err = store.Approve(appReq.ID, "tester")
	require.NoError(t, err)

	body := []byte(`{"approval_id":"approval-2","command":"whoami","target_type":"vm","target_id":"vm-101","run_on_host":false}`)
	req := httptest.NewRequest(http.MethodPost, "/api/ai/run-command", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleRunCommand(rec, req)

	require.Equal(t, http.StatusConflict, rec.Code)

	stored, found := store.GetApproval(appReq.ID)
	require.True(t, found)
	require.False(t, stored.Consumed)
}

func TestHandleRunCommand_RejectsUnsupportedTargetType(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, agentexec.NewServer(nil))

	store, err := approval.NewStore(approval.StoreConfig{
		DataDir:            tmp,
		DisablePersistence: true,
	})
	require.NoError(t, err)
	approval.SetStore(store)
	defer approval.SetStore(nil)

	appReq := &approval.ApprovalRequest{
		ID:         "approval-docker-1",
		Command:    "docker restart web",
		TargetType: "docker",
		TargetID:   "host-1:web",
	}
	require.NoError(t, store.CreateApproval(appReq))
	_, err = store.Approve(appReq.ID, "tester")
	require.NoError(t, err)

	body := []byte(`{"approval_id":"approval-docker-1","command":"docker restart web","target_type":"docker","target_id":"host-1:web","run_on_host":false}`)
	req := httptest.NewRequest(http.MethodPost, "/api/ai/run-command", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleRunCommand(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "unsupported target_type")

	stored, found := store.GetApproval(appReq.ID)
	require.True(t, found)
	require.False(t, stored.Consumed)
}

func TestHandleRunCommand_RejectsCrossOrgApproval(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, agentexec.NewServer(nil))

	store, err := approval.NewStore(approval.StoreConfig{
		DataDir:            tmp,
		DisablePersistence: true,
	})
	require.NoError(t, err)
	prevStore := approval.GetStore()
	t.Cleanup(func() { approval.SetStore(prevStore) })
	approval.SetStore(store)

	appReq := &approval.ApprovalRequest{
		ID:         "approval-org-a",
		OrgID:      "org-a",
		Command:    "uptime",
		TargetType: "vm",
		TargetID:   "vm-101",
	}
	require.NoError(t, store.CreateApproval(appReq))
	_, err = store.Approve(appReq.ID, "tester")
	require.NoError(t, err)

	body := []byte(`{"approval_id":"approval-org-a","command":"uptime","target_type":"vm","target_id":"vm-101","run_on_host":false}`)
	req := httptest.NewRequest(http.MethodPost, "/api/ai/run-command", bytes.NewReader(body))
	ctx := context.WithValue(req.Context(), OrgIDContextKey, "org-b")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler.HandleRunCommand(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
	stored, found := store.GetApproval(appReq.ID)
	require.True(t, found)
	require.False(t, stored.Consumed)
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

func TestHandleInvestigateAlert_RejectsLegacyHostResourceType(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	saveEnabledTestAIConfig(t, persistence)

	handler := newTestAISettingsHandler(cfg, persistence, nil)

	body := []byte(`{"alertIdentifier":"alert-1","resource_id":"agent-1","resource_name":"node-1","resource_type":"host","alert_type":"cpu","level":"warning","message":"high cpu"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/ai/investigate", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleInvestigateAlert(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), `unsupported resource_type "host"`)
	require.NotContains(t, rec.Header().Get("Content-Type"), "text/event-stream")
}

func TestHandleInvestigateAlert_AcceptsCanonicalAlertIdentifier(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	saveEnabledTestAIConfig(t, persistence)

	handler := newTestAISettingsHandler(cfg, persistence, nil)

	body := []byte(`{"alertIdentifier":"instance:node:100::metric/cpu","resource_id":"agent-1","resource_name":"node-1","resource_type":"host","alert_type":"cpu","level":"warning","message":"high cpu"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/ai/investigate", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleInvestigateAlert(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), `unsupported resource_type "host"`)
	require.NotContains(t, rec.Header().Get("Content-Type"), "text/event-stream")
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
	require.Same(t, cfg, handler.defaultConfig)

	// SetConfig with new config should update the handler's config
	newCfg := &config.Config{DataPath: tmp}
	handler.SetConfig(newCfg)
	require.Same(t, newCfg, handler.defaultConfig)
}

func TestAISettingsHandler_GetAlertTriggeredAnalyzer(t *testing.T) {
	// Not parallel: mutates package-global createAlertAnalyzerFunc
	mock := &mockAlertAnalyzerForTest{}
	SetCreateAlertAnalyzer(func(_ aicontracts.AlertAnalyzerDeps) aicontracts.AlertAnalyzer {
		return mock
	})
	t.Cleanup(func() { SetCreateAlertAnalyzer(nil) })

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)
	handler.defaultAIService.SetAlertAnalyzerFactory(getCreateAlertAnalyzer())

	handler.SetStateProvider(&MockStateProvider{})

	analyzer := handler.GetAlertTriggeredAnalyzer(context.Background())
	require.NotNil(t, analyzer)

	again := handler.GetAlertTriggeredAnalyzer(context.Background())
	require.Same(t, mock, again)
}

func TestAISettingsHandler_StartPatrol(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	aiCfg := config.NewDefaultAIConfig()
	aiCfg.Enabled = true
	aiCfg.OllamaBaseURL = "http://localhost:11434"
	aiCfg.Model = "ollama:llama3"
	aiCfg.PatrolEnabled = true
	require.NoError(t, persistence.SaveAIConfig(*aiCfg))
	require.NoError(t, handler.defaultAIService.LoadConfig())

	handler.SetStateProvider(&MockStateProvider{})

	// Start patrol with a cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	handler.StartPatrol(ctx)

	patrol := handler.defaultAIService.GetPatrolService()
	require.NotNil(t, patrol)
	status := patrol.GetStatus()
	require.True(t, status.Enabled)

	cancel()
	assert.NotPanics(t, func() { handler.StopPatrol() })

	after := patrol.GetStatus()
	require.False(t, after.Running)
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

func TestAISettingsHandler_Approvals(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	// Initialize approval store
	approvalStore, _ := approval.NewStore(approval.StoreConfig{
		DataDir:            tmp,
		DisablePersistence: true,
	})
	prevStore := approval.GetStore()
	t.Cleanup(func() { approval.SetStore(prevStore) })
	approval.SetStore(approvalStore)

	appID := "app-123"
	_ = approvalStore.CreateApproval(&approval.ApprovalRequest{
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
		_ = approvalStore.CreateApproval(&approval.ApprovalRequest{
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

func TestAISettingsHandler_Approvals_RejectCrossOrgAccess(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	store, err := approval.NewStore(approval.StoreConfig{
		DataDir:            tmp,
		DisablePersistence: true,
	})
	require.NoError(t, err)

	prevStore := approval.GetStore()
	t.Cleanup(func() { approval.SetStore(prevStore) })
	approval.SetStore(store)

	withOrg := func(req *http.Request, orgID string) *http.Request {
		ctx := context.WithValue(req.Context(), OrgIDContextKey, orgID)
		return req.WithContext(ctx)
	}

	require.NoError(t, store.CreateApproval(&approval.ApprovalRequest{
		ID:      "cross-org-get",
		OrgID:   "org-a",
		Command: "ls -la",
	}))

	getReq := withOrg(httptest.NewRequest(http.MethodGet, "/api/ai/approvals/cross-org-get", nil), "org-b")
	getRec := httptest.NewRecorder()
	handler.HandleGetApproval(getRec, getReq)
	require.Equal(t, http.StatusNotFound, getRec.Code)

	require.NoError(t, store.CreateApproval(&approval.ApprovalRequest{
		ID:      "cross-org-approve",
		OrgID:   "org-a",
		Command: "uptime",
	}))

	approveReq := withOrg(httptest.NewRequest(http.MethodPost, "/api/ai/approvals/cross-org-approve/approve", nil), "org-b")
	approveRec := httptest.NewRecorder()
	handler.HandleApproveCommand(approveRec, approveReq)
	require.Equal(t, http.StatusNotFound, approveRec.Code)
	approveApp, ok := store.GetApproval("cross-org-approve")
	require.True(t, ok)
	require.Equal(t, approval.StatusPending, approveApp.Status)

	require.NoError(t, store.CreateApproval(&approval.ApprovalRequest{
		ID:      "cross-org-deny",
		OrgID:   "org-a",
		Command: "rm -rf /tmp",
	}))

	denyReq := withOrg(
		httptest.NewRequest(http.MethodPost, "/api/ai/approvals/cross-org-deny/deny", bytes.NewReader([]byte(`{"reason":"no"}`))),
		"org-b",
	)
	denyRec := httptest.NewRecorder()
	handler.HandleDenyCommand(denyRec, denyReq)
	require.Equal(t, http.StatusNotFound, denyRec.Code)
	denyApp, ok := store.GetApproval("cross-org-deny")
	require.True(t, ok)
	require.Equal(t, approval.StatusPending, denyApp.Status)
}
