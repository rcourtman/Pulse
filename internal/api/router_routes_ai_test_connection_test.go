package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestConnectionRouter creates a Router with an Ollama mock server and a
// valid settings:write API token for route-level /api/ai/test tests.
func setupTestConnectionRouter(t *testing.T, ollamaURL string) (*Router, string) {
	t.Helper()

	rawToken := "ai-test-conn-route-token-" + t.Name() + ".12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsWrite}, nil)
	cfg := newTestConfigWithTokens(t, record)

	persistence := config.NewConfigPersistence(cfg.DataPath)
	aiCfg := config.NewDefaultAIConfig()
	aiCfg.Enabled = true
	aiCfg.Model = "ollama:llama3"
	aiCfg.OllamaBaseURL = ollamaURL
	if err := persistence.SaveAIConfig(*aiCfg); err != nil {
		t.Fatalf("SaveAIConfig: %v", err)
	}

	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")
	router.aiSettingsHandler.defaultConfig = cfg
	router.aiSettingsHandler.defaultPersistence = persistence
	svc := ai.NewService(persistence, nil)
	if err := svc.LoadConfig(); err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	router.aiSettingsHandler.defaultAIService = svc

	return router, rawToken
}

// TestRouteTestConnection_Success verifies that POST /api/ai/test dispatches
// through the full router and returns a success response with the configured
// model name.
func TestRouteTestConnection_Success(t *testing.T) {
	t.Parallel()

	ollama := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/version" {
			_ = json.NewEncoder(w).Encode(map[string]any{"version": "0.3.0"})
			return
		}
		http.NotFound(w, r)
	}))
	defer ollama.Close()

	router, token := setupTestConnectionRouter(t, ollama.URL)

	req := httptest.NewRequest(http.MethodPost, "/api/ai/test", nil)
	req.Header.Set("X-API-Token", token)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
		Model   string `json:"model"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
	assert.Equal(t, "Connection successful", resp.Message)
	assert.Equal(t, "ollama:llama3", resp.Model)
}

// TestRouteTestConnection_ConnectionFailure verifies the route returns
// success=false when the provider is configured but unreachable.
func TestRouteTestConnection_ConnectionFailure(t *testing.T) {
	t.Parallel()

	// Server that always returns 500
	ollama := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ollama.Close()

	router, token := setupTestConnectionRouter(t, ollama.URL)

	req := httptest.NewRequest(http.MethodPost, "/api/ai/test", nil)
	req.Header.Set("X-API-Token", token)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.False(t, resp.Success)
	assert.Equal(t, "Connection test failed", resp.Message)
}

// TestRouteTestConnection_MethodNotAllowed verifies that non-POST methods
// through the full router chain return 405.
func TestRouteTestConnection_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	router, token := setupTestConnectionRouter(t, "http://192.0.2.1:11434")

	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/ai/test", nil)
			req.Header.Set("X-API-Token", token)
			rec := httptest.NewRecorder()
			router.Handler().ServeHTTP(rec, req)

			assert.Equal(t, http.StatusMethodNotAllowed, rec.Code, "method %s should be rejected", method)
		})
	}
}

// TestRouteTestConnection_NoAuth verifies that unauthenticated requests to
// /api/ai/test are rejected with 401.
func TestRouteTestConnection_NoAuth(t *testing.T) {
	t.Parallel()

	router, _ := setupTestConnectionRouter(t, "http://192.0.2.1:11434")

	req := httptest.NewRequest(http.MethodPost, "/api/ai/test", nil)
	// No auth header
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code, "unauthenticated request should return 401")
}

// TestRouteTestConnection_WrongScope verifies that a token without the
// settings:write scope is rejected with 403.
func TestRouteTestConnection_WrongScope(t *testing.T) {
	t.Parallel()

	rawToken := "ai-test-conn-wrong-scope-" + t.Name() + ".12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsRead}, nil)
	cfg := newTestConfigWithTokens(t, record)

	persistence := config.NewConfigPersistence(cfg.DataPath)
	aiCfg := config.NewDefaultAIConfig()
	aiCfg.Enabled = true
	aiCfg.Model = "ollama:llama3"
	aiCfg.OllamaBaseURL = "http://192.0.2.1:11434"
	if err := persistence.SaveAIConfig(*aiCfg); err != nil {
		t.Fatalf("SaveAIConfig: %v", err)
	}

	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")
	router.aiSettingsHandler.defaultConfig = cfg
	router.aiSettingsHandler.defaultPersistence = persistence
	svc := ai.NewService(persistence, nil)
	if err := svc.LoadConfig(); err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	router.aiSettingsHandler.defaultAIService = svc

	req := httptest.NewRequest(http.MethodPost, "/api/ai/test", nil)
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code, "wrong scope should return 403")
}

// TestRouteTestConnection_NoConfig verifies that when no AI provider is
// configured, the endpoint returns success=false gracefully rather than
// crashing.
func TestRouteTestConnection_NoConfig(t *testing.T) {
	t.Parallel()

	rawToken := "ai-test-conn-no-config-" + t.Name() + ".12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsWrite}, nil)
	cfg := newTestConfigWithTokens(t, record)

	persistence := config.NewConfigPersistence(cfg.DataPath)
	// Don't save any AI config — service will have no configured provider

	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")
	router.aiSettingsHandler.defaultConfig = cfg
	router.aiSettingsHandler.defaultPersistence = persistence
	svc := ai.NewService(persistence, nil)
	if err := svc.LoadConfig(); err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	router.aiSettingsHandler.defaultAIService = svc

	req := httptest.NewRequest(http.MethodPost, "/api/ai/test", nil)
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.False(t, resp.Success)
	assert.Equal(t, "Connection test failed", resp.Message)
}
