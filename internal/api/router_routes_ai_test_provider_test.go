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

// setupTestProviderRouter creates a Router with an Ollama mock server and a valid
// settings:write API token for route-level /api/ai/test/{provider} tests.
func setupTestProviderRouter(t *testing.T, ollamaURL string) (*Router, string) {
	t.Helper()

	rawToken := "ai-test-provider-route-token-" + t.Name() + ".12345678"
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
	router.aiSettingsHandler.legacyConfig = cfg
	router.aiSettingsHandler.legacyPersistence = persistence
	svc := ai.NewService(persistence, nil)
	if err := svc.LoadConfig(); err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	router.aiSettingsHandler.legacyAIService = svc

	return router, rawToken
}

// TestRouteTestProvider_Success verifies that POST /api/ai/test/{provider}
// dispatches through the full router and returns a success response for a
// configured, reachable provider.
func TestRouteTestProvider_Success(t *testing.T) {
	t.Parallel()

	ollama := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/version" {
			_ = json.NewEncoder(w).Encode(map[string]any{"version": "0.3.0"})
			return
		}
		http.NotFound(w, r)
	}))
	defer ollama.Close()

	router, token := setupTestProviderRouter(t, ollama.URL)

	req := httptest.NewRequest(http.MethodPost, "/api/ai/test/ollama", nil)
	req.Header.Set("X-API-Token", token)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Success  bool   `json:"success"`
		Message  string `json:"message"`
		Provider string `json:"provider"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
	assert.Equal(t, "Connection successful", resp.Message)
	assert.Equal(t, "ollama", resp.Provider)
}

// TestRouteTestProvider_UnconfiguredProvider verifies that testing an
// unconfigured provider (e.g., openai when only ollama is set up) returns
// success=false with a clear "Provider not configured" message.
func TestRouteTestProvider_UnconfiguredProvider(t *testing.T) {
	t.Parallel()

	ollama := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer ollama.Close()

	router, token := setupTestProviderRouter(t, ollama.URL)

	req := httptest.NewRequest(http.MethodPost, "/api/ai/test/openai", nil)
	req.Header.Set("X-API-Token", token)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

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

// TestRouteTestProvider_ConnectionFailure verifies the route returns
// success=false when the provider is configured but the connection fails.
func TestRouteTestProvider_ConnectionFailure(t *testing.T) {
	t.Parallel()

	// Server that always returns 500
	ollama := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ollama.Close()

	router, token := setupTestProviderRouter(t, ollama.URL)

	req := httptest.NewRequest(http.MethodPost, "/api/ai/test/ollama", nil)
	req.Header.Set("X-API-Token", token)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

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

// TestRouteTestProvider_MethodNotAllowed verifies that GET (and other
// non-POST methods) through the full router chain return 405.
func TestRouteTestProvider_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	router, token := setupTestProviderRouter(t, "http://192.0.2.1:11434")

	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/ai/test/ollama", nil)
			req.Header.Set("X-API-Token", token)
			rec := httptest.NewRecorder()
			router.Handler().ServeHTTP(rec, req)

			assert.Equal(t, http.StatusMethodNotAllowed, rec.Code, "method %s should be rejected", method)
		})
	}
}

// TestRouteTestProvider_NoAuth verifies that unauthenticated requests to
// /api/ai/test/{provider} are rejected.
func TestRouteTestProvider_NoAuth(t *testing.T) {
	t.Parallel()

	router, _ := setupTestProviderRouter(t, "http://192.0.2.1:11434")

	req := httptest.NewRequest(http.MethodPost, "/api/ai/test/ollama", nil)
	// No auth header
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	// API-token mode returns 401 for unauthenticated requests
	assert.Equal(t, http.StatusUnauthorized, rec.Code, "unauthenticated request should return 401")
}

// TestRouteTestProvider_MultipleProviders verifies that the {provider} path
// variable correctly routes different provider names to the same handler
// and extracts the provider value from the URL.
func TestRouteTestProvider_MultipleProviders(t *testing.T) {
	t.Parallel()

	ollama := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer ollama.Close()

	router, token := setupTestProviderRouter(t, ollama.URL)

	// All providers except ollama are unconfigured, so they return "Provider not configured"
	// with the correct provider name extracted from the URL.
	for _, provider := range []string{"anthropic", "openai", "deepseek", "gemini", "openrouter"} {
		t.Run(provider, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/ai/test/"+provider, nil)
			req.Header.Set("X-API-Token", token)
			rec := httptest.NewRecorder()
			router.Handler().ServeHTTP(rec, req)

			require.Equal(t, http.StatusOK, rec.Code)

			var resp struct {
				Success  bool   `json:"success"`
				Message  string `json:"message"`
				Provider string `json:"provider"`
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
			assert.False(t, resp.Success)
			assert.Equal(t, "Provider not configured", resp.Message, "unconfigured provider should return clear message")
			assert.Equal(t, provider, resp.Provider, "provider name should be extracted from URL")
		})
	}
}

// TestRouteTestProvider_UnknownProvider verifies that testing an unknown/invalid
// provider name still returns a structured JSON response (not a crash), with
// success=false.
func TestRouteTestProvider_UnknownProvider(t *testing.T) {
	t.Parallel()

	ollama := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer ollama.Close()

	router, token := setupTestProviderRouter(t, ollama.URL)

	req := httptest.NewRequest(http.MethodPost, "/api/ai/test/nonexistent-provider", nil)
	req.Header.Set("X-API-Token", token)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Success  bool   `json:"success"`
		Message  string `json:"message"`
		Provider string `json:"provider"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.False(t, resp.Success)
	assert.Equal(t, "nonexistent-provider", resp.Provider)
}
