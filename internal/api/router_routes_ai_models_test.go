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

// setupModelsRouter creates a Router with an Ollama mock server and a valid
// ai:chat API token for route-level /api/ai/models tests.
func setupModelsRouter(t *testing.T, ollamaURL string) (*Router, string) {
	t.Helper()

	rawToken := "ai-models-route-token-" + t.Name() + ".12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeAIChat}, nil)
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

// TestRouteListModels_Success verifies that GET /api/ai/models dispatches
// through the full router and returns a populated model list from a configured
// Ollama provider.
func TestRouteListModels_Success(t *testing.T) {
	t.Parallel()

	ollama := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"models": []map[string]any{
					{"name": "llama3:latest"},
					{"name": "tinyllama:latest"},
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer ollama.Close()

	router, token := setupModelsRouter(t, ollama.URL)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/models", nil)
	req.Header.Set("X-API-Token", token)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Models []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"models"`
		Error  string `json:"error"`
		Cached bool   `json:"cached"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Empty(t, resp.Error)
	assert.Len(t, resp.Models, 2)
	assert.Equal(t, "ollama:llama3:latest", resp.Models[0].ID)
	assert.Equal(t, "ollama:tinyllama:latest", resp.Models[1].ID)
}

// TestRouteListModels_MethodNotAllowed verifies that non-GET methods through
// the full router chain return 405.
func TestRouteListModels_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	router, token := setupModelsRouter(t, "http://192.0.2.1:11434")

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/ai/models", nil)
			req.Header.Set("X-API-Token", token)
			rec := httptest.NewRecorder()
			router.Handler().ServeHTTP(rec, req)

			assert.Equal(t, http.StatusMethodNotAllowed, rec.Code, "method %s should be rejected", method)
		})
	}
}

// TestRouteListModels_NoAuth verifies that unauthenticated requests to
// /api/ai/models are rejected.
func TestRouteListModels_NoAuth(t *testing.T) {
	t.Parallel()

	router, _ := setupModelsRouter(t, "http://192.0.2.1:11434")

	req := httptest.NewRequest(http.MethodGet, "/api/ai/models", nil)
	// No auth header
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code, "unauthenticated request should return 401")
}

// TestRouteListModels_WrongScope verifies that a token without the ai:chat
// scope is rejected.
func TestRouteListModels_WrongScope(t *testing.T) {
	t.Parallel()

	rawToken := "ai-models-wrong-scope-" + t.Name() + ".12345678"
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

	req := httptest.NewRequest(http.MethodGet, "/api/ai/models", nil)
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code, "wrong scope should return 403")
}

// TestRouteListModels_ProviderError verifies that when the backend provider
// is unreachable, the endpoint returns 200 with an empty model list rather
// than crashing. The service layer silently handles individual provider
// failures and returns what it can.
func TestRouteListModels_ProviderError(t *testing.T) {
	t.Parallel()

	// Server that always returns 500
	ollama := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ollama.Close()

	router, token := setupModelsRouter(t, ollama.URL)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/models", nil)
	req.Header.Set("X-API-Token", token)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Models []json.RawMessage `json:"models"`
		Error  string            `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	// The response should be valid JSON with an empty models array
	// and should not crash or return a 5xx status.
	assert.NotNil(t, resp.Models, "models field should be present")
	assert.Empty(t, resp.Models, "models should be empty when provider fails")
}

// TestRouteListModels_EmptyModelList verifies that when the provider returns
// no models, the endpoint returns an empty array (not null).
func TestRouteListModels_EmptyModelList(t *testing.T) {
	t.Parallel()

	ollama := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"models": []map[string]any{},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer ollama.Close()

	router, token := setupModelsRouter(t, ollama.URL)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/models", nil)
	req.Header.Set("X-API-Token", token)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Models []struct {
			ID string `json:"id"`
		} `json:"models"`
		Error string `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Empty(t, resp.Error)
	assert.NotNil(t, resp.Models, "models should be non-nil empty array")
	assert.Len(t, resp.Models, 0)
}

// TestRouteListModels_ResponseContentType verifies the response Content-Type
// is application/json on a successful response.
func TestRouteListModels_ResponseContentType(t *testing.T) {
	t.Parallel()

	ollama := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"models": []map[string]any{{"name": "llama3:latest"}},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer ollama.Close()

	router, token := setupModelsRouter(t, ollama.URL)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/models", nil)
	req.Header.Set("X-API-Token", token)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"),
		"response must have application/json Content-Type")
}

// TestRouteListModels_CachedField verifies that the cached field is present
// in the response and set to false on the first (uncached) call.
func TestRouteListModels_CachedField(t *testing.T) {
	t.Parallel()

	ollama := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"models": []map[string]any{{"name": "llama3:latest"}},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer ollama.Close()

	router, token := setupModelsRouter(t, ollama.URL)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/models", nil)
	req.Header.Set("X-API-Token", token)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	// Parse raw JSON to verify the "cached" key is present and boolean.
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &raw))
	cachedRaw, ok := raw["cached"]
	require.True(t, ok, "response must contain 'cached' field")
	var cached bool
	require.NoError(t, json.Unmarshal(cachedRaw, &cached))
	// First call should not be cached.
	assert.False(t, cached, "first request should not be cached")
}

// TestRouteListModels_MalformedProviderResponse verifies that when the
// provider returns invalid JSON, the endpoint does not crash and returns
// a valid JSON response with an empty model list. The service layer silently
// skips providers that fail (including JSON decode errors) and returns
// whatever models it could collect, so no error field is set.
func TestRouteListModels_MalformedProviderResponse(t *testing.T) {
	t.Parallel()

	ollama := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return garbage that is not valid JSON
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{not valid json`))
	}))
	defer ollama.Close()

	router, token := setupModelsRouter(t, ollama.URL)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/models", nil)
	req.Header.Set("X-API-Token", token)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	// Should not crash — should return 200 with empty models.
	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Models []json.RawMessage `json:"models"`
		Error  string            `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp),
		"response body must be valid JSON even when provider returns garbage")
	assert.NotNil(t, resp.Models, "models field should be present")
	assert.Empty(t, resp.Models, "models should be empty when provider returns invalid JSON")
	// The service layer silently skips failed providers and does not propagate
	// an error string unless all providers fail at the ListModelsWithCache level.
	assert.Empty(t, resp.Error, "no error field expected when provider is silently skipped")
}

// TestRouteListModels_HEADMethod verifies that HEAD /api/ai/models is
// rejected with 405 (the handler only accepts GET).
func TestRouteListModels_HEADMethod(t *testing.T) {
	t.Parallel()

	router, token := setupModelsRouter(t, "http://192.0.2.1:11434")

	req := httptest.NewRequest(http.MethodHead, "/api/ai/models", nil)
	req.Header.Set("X-API-Token", token)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code,
		"HEAD should be rejected with 405")
}
