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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupExecuteRouter creates a Router with an Ollama mock server and a valid
// ai:execute API token for route-level /api/ai/execute tests.
func setupExecuteRouter(t *testing.T, ollamaURL string) (*Router, string) {
	t.Helper()

	rawToken := "ai-execute-route-token-" + t.Name() + ".12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeAIExecute}, nil)
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

// mockOllamaForExecute returns an HTTP handler that mocks the Ollama API
// endpoints needed for /api/ai/execute: /api/chat, /api/version, /api/tags.
func mockOllamaForExecute() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
			_ = json.NewEncoder(w).Encode(map[string]any{"version": "0.3.0"})
		case "/api/tags":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"models": []map[string]any{{"name": "llama3"}},
			})
		default:
			http.NotFound(w, r)
		}
	})
}

// TestRouteExecute_Success verifies that POST /api/ai/execute dispatches
// through the full router middleware chain (RequireAdmin → RequireScope →
// HandleExecute) and returns a successful AI response from Ollama.
func TestRouteExecute_Success(t *testing.T) {
	t.Parallel()

	ollama := newIPv4HTTPServer(t, mockOllamaForExecute())
	defer ollama.Close()

	router, token := setupExecuteRouter(t, ollama.URL)

	body := `{"prompt":"What is the status of my server?"}`
	req := httptest.NewRequest(http.MethodPost, "/api/ai/execute", strings.NewReader(body))
	req.Header.Set("X-API-Token", token)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp AIExecuteResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "hello from ollama", resp.Content)
	assert.NotEmpty(t, resp.Model)
}

// TestRouteExecute_MethodNotAllowed verifies that non-POST methods through
// the full router chain return 405.
func TestRouteExecute_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	router, token := setupExecuteRouter(t, "http://192.0.2.1:11434")

	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/ai/execute", nil)
			req.Header.Set("X-API-Token", token)
			rec := httptest.NewRecorder()
			router.Handler().ServeHTTP(rec, req)

			assert.Equal(t, http.StatusMethodNotAllowed, rec.Code, "method %s should be rejected", method)
		})
	}
}

// TestRouteExecute_NoAuth verifies that unauthenticated requests to
// /api/ai/execute are rejected with 401.
func TestRouteExecute_NoAuth(t *testing.T) {
	t.Parallel()

	router, _ := setupExecuteRouter(t, "http://192.0.2.1:11434")

	body := `{"prompt":"hi"}`
	req := httptest.NewRequest(http.MethodPost, "/api/ai/execute", strings.NewReader(body))
	// No auth header
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code, "unauthenticated request should return 401")
	assert.Contains(t, rec.Body.String(), "Authentication required", "body should indicate authentication is needed")
}

// TestRouteExecute_WrongScope verifies that a token without the ai:execute
// scope is rejected with 403.
func TestRouteExecute_WrongScope(t *testing.T) {
	t.Parallel()

	rawToken := "ai-execute-wrong-scope-" + t.Name() + ".12345678"
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
	router.aiSettingsHandler.legacyConfig = cfg
	router.aiSettingsHandler.legacyPersistence = persistence
	svc := ai.NewService(persistence, nil)
	if err := svc.LoadConfig(); err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	router.aiSettingsHandler.legacyAIService = svc

	body := `{"prompt":"hi"}`
	req := httptest.NewRequest(http.MethodPost, "/api/ai/execute", strings.NewReader(body))
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code, "wrong scope should return 403")
	assert.Contains(t, rec.Body.String(), "missing_scope", "body should indicate missing scope")
}

// TestRouteExecute_EmptyPrompt verifies that an empty prompt is rejected
// with 400 Bad Request.
func TestRouteExecute_EmptyPrompt(t *testing.T) {
	t.Parallel()

	ollama := newIPv4HTTPServer(t, mockOllamaForExecute())
	defer ollama.Close()

	router, token := setupExecuteRouter(t, ollama.URL)

	body := `{"prompt":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/ai/execute", strings.NewReader(body))
	req.Header.Set("X-API-Token", token)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code, "empty prompt should return 400")
}

// TestRouteExecute_WhitespaceOnlyPrompt verifies that a whitespace-only
// prompt is rejected with 400 Bad Request.
func TestRouteExecute_WhitespaceOnlyPrompt(t *testing.T) {
	t.Parallel()

	ollama := newIPv4HTTPServer(t, mockOllamaForExecute())
	defer ollama.Close()

	router, token := setupExecuteRouter(t, ollama.URL)

	body := `{"prompt":"   "}`
	req := httptest.NewRequest(http.MethodPost, "/api/ai/execute", strings.NewReader(body))
	req.Header.Set("X-API-Token", token)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code, "whitespace-only prompt should return 400")
}

// TestRouteExecute_InvalidJSON verifies that a malformed JSON request body
// is rejected with 400 Bad Request.
func TestRouteExecute_InvalidJSON(t *testing.T) {
	t.Parallel()

	router, token := setupExecuteRouter(t, "http://192.0.2.1:11434")

	body := `{not json}`
	req := httptest.NewRequest(http.MethodPost, "/api/ai/execute", strings.NewReader(body))
	req.Header.Set("X-API-Token", token)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code, "invalid JSON should return 400")
}

// TestRouteExecute_AIDisabled verifies that when AI is not enabled,
// the endpoint returns 400 with a clear message.
func TestRouteExecute_AIDisabled(t *testing.T) {
	t.Parallel()

	rawToken := "ai-execute-disabled-" + t.Name() + ".12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeAIExecute}, nil)
	cfg := newTestConfigWithTokens(t, record)

	persistence := config.NewConfigPersistence(cfg.DataPath)
	// Save default AI config with Enabled = false (default)
	aiCfg := config.NewDefaultAIConfig()
	aiCfg.Enabled = false
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

	body := `{"prompt":"hi"}`
	req := httptest.NewRequest(http.MethodPost, "/api/ai/execute", strings.NewReader(body))
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code, "disabled AI should return 400")
	assert.Contains(t, rec.Body.String(), "not enabled", "body should indicate AI is not enabled")
}

// TestRouteExecute_WithConversationHistory verifies that the endpoint
// accepts and processes a request with conversation history.
func TestRouteExecute_WithConversationHistory(t *testing.T) {
	t.Parallel()

	ollama := newIPv4HTTPServer(t, mockOllamaForExecute())
	defer ollama.Close()

	router, token := setupExecuteRouter(t, ollama.URL)

	body := `{
		"prompt": "What was the last thing I asked?",
		"history": [
			{"role": "user", "content": "Hello"},
			{"role": "assistant", "content": "Hi there!"}
		]
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/ai/execute", strings.NewReader(body))
	req.Header.Set("X-API-Token", token)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp AIExecuteResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp.Content)
	assert.NotEmpty(t, resp.Model)
}

// TestRouteExecute_WithTargetContext verifies that the endpoint accepts
// and processes a request with target type and ID context.
func TestRouteExecute_WithTargetContext(t *testing.T) {
	t.Parallel()

	ollama := newIPv4HTTPServer(t, mockOllamaForExecute())
	defer ollama.Close()

	router, token := setupExecuteRouter(t, ollama.URL)

	body := `{
		"prompt": "Check CPU usage",
		"target_type": "node",
		"target_id": "pve1",
		"context": {"cpu": 85.5, "memory": 60.2}
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/ai/execute", strings.NewReader(body))
	req.Header.Set("X-API-Token", token)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp AIExecuteResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp.Content)
}

// TestRouteExecute_DefaultUseCaseIsChat verifies that when no use_case is
// specified, the endpoint defaults to "chat" and succeeds.
func TestRouteExecute_DefaultUseCaseIsChat(t *testing.T) {
	t.Parallel()

	ollama := newIPv4HTTPServer(t, mockOllamaForExecute())
	defer ollama.Close()

	router, token := setupExecuteRouter(t, ollama.URL)

	// No use_case field — should default to "chat"
	body := `{"prompt":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/api/ai/execute", strings.NewReader(body))
	req.Header.Set("X-API-Token", token)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp AIExecuteResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp.Content)
}

// TestRouteExecute_AutofixLicenseRequired verifies that a request with
// use_case "autofix" returns 402 when the FeatureAIAutoFix license is not
// available.
func TestRouteExecute_AutofixLicenseRequired(t *testing.T) {
	t.Parallel()

	ollama := newIPv4HTTPServer(t, mockOllamaForExecute())
	defer ollama.Close()

	router, token := setupExecuteRouter(t, ollama.URL)
	// Inject a license checker that denies all features
	router.aiSettingsHandler.legacyAIService.SetLicenseChecker(stubLicenseChecker{allow: false})

	body := `{"prompt":"fix the issue","use_case":"autofix"}`
	req := httptest.NewRequest(http.MethodPost, "/api/ai/execute", strings.NewReader(body))
	req.Header.Set("X-API-Token", token)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusPaymentRequired, rec.Code, "autofix without license should return 402")
	assert.Contains(t, rec.Body.String(), "license_required", "body should indicate license is required")
}

// TestRouteExecute_RemediationLicenseRequired verifies that the "remediation"
// use_case alias also triggers the 402 license gate.
func TestRouteExecute_RemediationLicenseRequired(t *testing.T) {
	t.Parallel()

	ollama := newIPv4HTTPServer(t, mockOllamaForExecute())
	defer ollama.Close()

	router, token := setupExecuteRouter(t, ollama.URL)
	router.aiSettingsHandler.legacyAIService.SetLicenseChecker(stubLicenseChecker{allow: false})

	body := `{"prompt":"remediate the issue","use_case":"remediation"}`
	req := httptest.NewRequest(http.MethodPost, "/api/ai/execute", strings.NewReader(body))
	req.Header.Set("X-API-Token", token)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusPaymentRequired, rec.Code, "remediation without license should return 402")
	assert.Contains(t, rec.Body.String(), "license_required", "body should indicate license is required")
}

// TestRouteExecute_OversizedBody verifies that a request body exceeding the
// 64KB limit is rejected.
func TestRouteExecute_OversizedBody(t *testing.T) {
	t.Parallel()

	ollama := newIPv4HTTPServer(t, mockOllamaForExecute())
	defer ollama.Close()

	router, token := setupExecuteRouter(t, ollama.URL)

	// Create a body that exceeds 64KB
	bigPrompt := strings.Repeat("a", 70*1024)
	body := `{"prompt":"` + bigPrompt + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/ai/execute", strings.NewReader(body))
	req.Header.Set("X-API-Token", token)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code, "oversized body should return 400")
}
