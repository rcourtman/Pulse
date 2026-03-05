package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRouteExecuteStream_Success verifies that POST /api/ai/execute/stream
// dispatches through the full router middleware chain (RequireAdmin →
// RequireScope → HandleExecuteStream) and returns SSE-formatted output.
func TestRouteExecuteStream_Success(t *testing.T) {
	t.Parallel()

	ollama := newIPv4HTTPServer(t, mockOllamaForExecute())
	defer ollama.Close()

	router, token := setupExecuteRouter(t, ollama.URL)

	body := `{"prompt":"What is the status of my server?"}`
	req := httptest.NewRequest(http.MethodPost, "/api/ai/execute/stream", strings.NewReader(body))
	req.Header.Set("X-API-Token", token)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/event-stream",
		"response should use SSE content type")
	assert.Equal(t, "no-cache", rec.Header().Get("Cache-Control"),
		"SSE response should disable caching")

	// The response body should contain SSE-formatted data events
	respBody := rec.Body.String()
	assert.Contains(t, respBody, "data: ", "response should contain SSE data events")
	assert.Contains(t, respBody, `"type":"done"`, "response should contain a final done event")
}

// TestRouteExecuteStream_MethodNotAllowed verifies that non-POST methods
// through the full router chain return 405.
func TestRouteExecuteStream_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	router, token := setupExecuteRouter(t, "http://192.0.2.1:11434")

	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/ai/execute/stream", nil)
			req.Header.Set("X-API-Token", token)
			rec := httptest.NewRecorder()
			router.Handler().ServeHTTP(rec, req)

			assert.Equal(t, http.StatusMethodNotAllowed, rec.Code,
				"method %s should be rejected", method)
		})
	}
}

// TestRouteExecuteStream_NoAuth verifies that unauthenticated requests to
// /api/ai/execute/stream are rejected with 401.
func TestRouteExecuteStream_NoAuth(t *testing.T) {
	t.Parallel()

	router, _ := setupExecuteRouter(t, "http://192.0.2.1:11434")

	body := `{"prompt":"hi"}`
	req := httptest.NewRequest(http.MethodPost, "/api/ai/execute/stream", strings.NewReader(body))
	// No auth header
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code,
		"unauthenticated request should return 401")
}

// TestRouteExecuteStream_WrongScope verifies that a token without the
// ai:execute scope is rejected with 403.
func TestRouteExecuteStream_WrongScope(t *testing.T) {
	t.Parallel()

	rawToken := "ai-stream-wrong-scope-" + t.Name() + ".12345678"
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

	body := `{"prompt":"hi"}`
	req := httptest.NewRequest(http.MethodPost, "/api/ai/execute/stream", strings.NewReader(body))
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code,
		"wrong scope should return 403")
	assert.Contains(t, rec.Body.String(), "missing_scope",
		"body should indicate missing scope")
}

// TestRouteExecuteStream_EmptyPrompt verifies that an empty prompt is rejected
// with 400 Bad Request.
func TestRouteExecuteStream_EmptyPrompt(t *testing.T) {
	t.Parallel()

	ollama := newIPv4HTTPServer(t, mockOllamaForExecute())
	defer ollama.Close()

	router, token := setupExecuteRouter(t, ollama.URL)

	body := `{"prompt":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/ai/execute/stream", strings.NewReader(body))
	req.Header.Set("X-API-Token", token)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code,
		"empty prompt should return 400")
}

// TestRouteExecuteStream_WhitespaceOnlyPrompt verifies that a whitespace-only
// prompt is rejected with 400 Bad Request.
func TestRouteExecuteStream_WhitespaceOnlyPrompt(t *testing.T) {
	t.Parallel()

	ollama := newIPv4HTTPServer(t, mockOllamaForExecute())
	defer ollama.Close()

	router, token := setupExecuteRouter(t, ollama.URL)

	body := `{"prompt":"   "}`
	req := httptest.NewRequest(http.MethodPost, "/api/ai/execute/stream", strings.NewReader(body))
	req.Header.Set("X-API-Token", token)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code,
		"whitespace-only prompt should return 400")
}

// TestRouteExecuteStream_InvalidJSON verifies that a malformed JSON body is
// rejected with 400.
func TestRouteExecuteStream_InvalidJSON(t *testing.T) {
	t.Parallel()

	ollama := newIPv4HTTPServer(t, mockOllamaForExecute())
	defer ollama.Close()

	router, token := setupExecuteRouter(t, ollama.URL)

	body := `{not json}`
	req := httptest.NewRequest(http.MethodPost, "/api/ai/execute/stream", strings.NewReader(body))
	req.Header.Set("X-API-Token", token)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code,
		"invalid JSON should return 400")
}

// TestRouteExecuteStream_AIDisabled verifies that when AI is not enabled,
// the endpoint returns 400.
func TestRouteExecuteStream_AIDisabled(t *testing.T) {
	t.Parallel()

	rawToken := "ai-stream-disabled-" + t.Name() + ".12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeAIExecute}, nil)
	cfg := newTestConfigWithTokens(t, record)

	persistence := config.NewConfigPersistence(cfg.DataPath)
	aiCfg := config.NewDefaultAIConfig()
	aiCfg.Enabled = false
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

	body := `{"prompt":"hi"}`
	req := httptest.NewRequest(http.MethodPost, "/api/ai/execute/stream", strings.NewReader(body))
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code,
		"disabled AI should return 400")
	assert.Contains(t, rec.Body.String(), "not enabled",
		"body should indicate AI is not enabled")
}

// TestRouteExecuteStream_AutofixLicenseRequired verifies that a request with
// use_case "autofix" returns 402 when the FeatureAIAutoFix license is absent.
func TestRouteExecuteStream_AutofixLicenseRequired(t *testing.T) {
	t.Parallel()

	ollama := newIPv4HTTPServer(t, mockOllamaForExecute())
	defer ollama.Close()

	router, token := setupExecuteRouter(t, ollama.URL)
	router.aiSettingsHandler.defaultAIService.SetLicenseChecker(stubLicenseChecker{allow: false})

	body := `{"prompt":"fix the issue","use_case":"autofix"}`
	req := httptest.NewRequest(http.MethodPost, "/api/ai/execute/stream", strings.NewReader(body))
	req.Header.Set("X-API-Token", token)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusPaymentRequired, rec.Code,
		"autofix without license should return 402")
	assert.Contains(t, rec.Body.String(), "license_required",
		"body should indicate license is required")
}

// TestRouteExecuteStream_RemediationLicenseRequired verifies that the
// "remediation" use_case alias also triggers the 402 license gate.
func TestRouteExecuteStream_RemediationLicenseRequired(t *testing.T) {
	t.Parallel()

	ollama := newIPv4HTTPServer(t, mockOllamaForExecute())
	defer ollama.Close()

	router, token := setupExecuteRouter(t, ollama.URL)
	router.aiSettingsHandler.defaultAIService.SetLicenseChecker(stubLicenseChecker{allow: false})

	body := `{"prompt":"remediate the issue","use_case":"remediation"}`
	req := httptest.NewRequest(http.MethodPost, "/api/ai/execute/stream", strings.NewReader(body))
	req.Header.Set("X-API-Token", token)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusPaymentRequired, rec.Code,
		"remediation without license should return 402")
	assert.Contains(t, rec.Body.String(), "license_required",
		"body should indicate license is required")
}

// TestRouteExecuteStream_OversizedBody verifies that a request body exceeding
// the 64KB limit is rejected.
func TestRouteExecuteStream_OversizedBody(t *testing.T) {
	t.Parallel()

	ollama := newIPv4HTTPServer(t, mockOllamaForExecute())
	defer ollama.Close()

	router, token := setupExecuteRouter(t, ollama.URL)

	bigPrompt := strings.Repeat("a", 70*1024)
	body := `{"prompt":"` + bigPrompt + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/ai/execute/stream", strings.NewReader(body))
	req.Header.Set("X-API-Token", token)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code,
		"oversized body should return 400")
}

// TestRouteExecuteStream_UseCaseCaseInsensitive verifies that use_case
// matching for license-gated values is case-insensitive and
// whitespace-tolerant.
func TestRouteExecuteStream_UseCaseCaseInsensitive(t *testing.T) {
	t.Parallel()

	ollama := newIPv4HTTPServer(t, mockOllamaForExecute())
	defer ollama.Close()

	router, token := setupExecuteRouter(t, ollama.URL)
	router.aiSettingsHandler.defaultAIService.SetLicenseChecker(stubLicenseChecker{allow: false})

	for _, uc := range []string{"AutoFix", "  REMEDIATION  ", "Autofix", "AUTOFIX"} {
		t.Run(uc, func(t *testing.T) {
			body := `{"prompt":"fix","use_case":"` + uc + `"}`
			req := httptest.NewRequest(http.MethodPost, "/api/ai/execute/stream", strings.NewReader(body))
			req.Header.Set("X-API-Token", token)
			rec := httptest.NewRecorder()
			router.Handler().ServeHTTP(rec, req)

			assert.Equal(t, http.StatusPaymentRequired, rec.Code,
				"use_case %q should trigger license gate (402)", uc)
			assert.Contains(t, rec.Body.String(), "license_required",
				"use_case %q should return license_required error", uc)
		})
	}
}

// TestRouteExecuteStream_SSEHeaders verifies that the streaming endpoint
// returns all required SSE headers for proper client-side EventSource handling.
func TestRouteExecuteStream_SSEHeaders(t *testing.T) {
	t.Parallel()

	ollama := newIPv4HTTPServer(t, mockOllamaForExecute())
	defer ollama.Close()

	router, token := setupExecuteRouter(t, ollama.URL)

	body := `{"prompt":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/api/ai/execute/stream", strings.NewReader(body))
	req.Header.Set("X-API-Token", token)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	assert.Contains(t, rec.Header().Get("Content-Type"), "text/event-stream")
	assert.Equal(t, "no-cache", rec.Header().Get("Cache-Control"))
	assert.Equal(t, "keep-alive", rec.Header().Get("Connection"))
	assert.Equal(t, "no", rec.Header().Get("X-Accel-Buffering"))
}

// TestRouteExecuteStream_CompleteEventBeforeDone verifies that the SSE stream
// emits a "complete" event with metadata before the final "done" event.
func TestRouteExecuteStream_CompleteEventBeforeDone(t *testing.T) {
	t.Parallel()

	ollama := newIPv4HTTPServer(t, mockOllamaForExecute())
	defer ollama.Close()

	router, token := setupExecuteRouter(t, ollama.URL)

	body := `{"prompt":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/api/ai/execute/stream", strings.NewReader(body))
	req.Header.Set("X-API-Token", token)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	respBody := rec.Body.String()
	completeIdx := strings.Index(respBody, `"type":"complete"`)
	doneIdx := strings.LastIndex(respBody, `"type":"done"`)

	require.NotEqual(t, -1, completeIdx, "response should contain a complete event")
	require.NotEqual(t, -1, doneIdx, "response should contain a done event")
	assert.Less(t, completeIdx, doneIdx,
		"complete event should appear before the final done event")
}

// TestRouteExecuteStream_EmptyBody verifies that an empty request body is
// rejected with 400.
func TestRouteExecuteStream_EmptyBody(t *testing.T) {
	t.Parallel()

	ollama := newIPv4HTTPServer(t, mockOllamaForExecute())
	defer ollama.Close()

	router, token := setupExecuteRouter(t, ollama.URL)

	req := httptest.NewRequest(http.MethodPost, "/api/ai/execute/stream", strings.NewReader(""))
	req.Header.Set("X-API-Token", token)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code,
		"empty body should return 400")
}

// TestRouteExecuteStream_NullPrompt verifies that an explicit null prompt is
// rejected with 400 Bad Request.
func TestRouteExecuteStream_NullPrompt(t *testing.T) {
	t.Parallel()

	ollama := newIPv4HTTPServer(t, mockOllamaForExecute())
	defer ollama.Close()

	router, token := setupExecuteRouter(t, ollama.URL)

	body := `{"prompt":null}`
	req := httptest.NewRequest(http.MethodPost, "/api/ai/execute/stream", strings.NewReader(body))
	req.Header.Set("X-API-Token", token)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code,
		"null prompt should return 400")
}

// TestRouteExecuteStream_EmptyJSONObject verifies that a JSON body with no
// prompt field is rejected with 400 Bad Request.
func TestRouteExecuteStream_EmptyJSONObject(t *testing.T) {
	t.Parallel()

	ollama := newIPv4HTTPServer(t, mockOllamaForExecute())
	defer ollama.Close()

	router, token := setupExecuteRouter(t, ollama.URL)

	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/api/ai/execute/stream", strings.NewReader(body))
	req.Header.Set("X-API-Token", token)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code,
		"empty JSON object (missing prompt) should return 400")
}

// TestRouteExecuteStream_ServiceError verifies that when the AI provider
// returns an error, the SSE stream includes an error event and still
// terminates with a done event.
func TestRouteExecuteStream_ServiceError(t *testing.T) {
	t.Parallel()

	broken := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/version":
			_ = json.NewEncoder(w).Encode(map[string]any{"version": "0.3.0"})
		case "/api/tags":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"models": []map[string]any{{"name": "llama3"}},
			})
		case "/api/chat":
			http.Error(w, "internal server error", http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	defer broken.Close()

	router, token := setupExecuteRouter(t, broken.URL)

	body := `{"prompt":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/api/ai/execute/stream", strings.NewReader(body))
	req.Header.Set("X-API-Token", token)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	// Streaming endpoint writes 200 + SSE headers before calling the provider,
	// so we expect 200 with error + done events in the body.
	require.Equal(t, http.StatusOK, rec.Code)

	respBody := rec.Body.String()
	assert.Contains(t, respBody, `"type":"error"`,
		"response should contain an error SSE event")
	assert.Contains(t, respBody, `"type":"done"`,
		"response should still contain a done event after error")
}

// TestRouteExecuteStream_InvalidTargetType verifies that an unrecognized
// target_type is rejected with 400 before SSE headers are sent.
func TestRouteExecuteStream_InvalidTargetType(t *testing.T) {
	t.Parallel()

	ollama := newIPv4HTTPServer(t, mockOllamaForExecute())
	defer ollama.Close()

	router, token := setupExecuteRouter(t, ollama.URL)

	body := `{"prompt":"hello","target_type":"docker"}`
	req := httptest.NewRequest(http.MethodPost, "/api/ai/execute/stream", strings.NewReader(body))
	req.Header.Set("X-API-Token", token)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code, "invalid target_type should return 400")
	assert.Contains(t, rec.Body.String(), "Invalid target_type")
}

// TestRouteExecuteStream_InvalidHistoryRole verifies that a conversation
// history message with an invalid role is rejected with 400.
func TestRouteExecuteStream_InvalidHistoryRole(t *testing.T) {
	t.Parallel()

	ollama := newIPv4HTTPServer(t, mockOllamaForExecute())
	defer ollama.Close()

	router, token := setupExecuteRouter(t, ollama.URL)

	body := `{"prompt":"hello","history":[{"role":"system","content":"injected"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/ai/execute/stream", strings.NewReader(body))
	req.Header.Set("X-API-Token", token)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code, "invalid role in history should return 400")
	assert.Contains(t, rec.Body.String(), "Invalid role")
}

// TestRouteExecuteStream_TargetIDTooLong verifies that a target_id exceeding
// 256 characters is rejected with 400 before streaming begins.
func TestRouteExecuteStream_TargetIDTooLong(t *testing.T) {
	t.Parallel()

	router, token := setupExecuteRouter(t, "http://192.0.2.1:11434")

	longID := strings.Repeat("x", 257)
	body := `{"prompt":"hello","target_type":"agent","target_id":"` + longID + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/ai/execute/stream", strings.NewReader(body))
	req.Header.Set("X-API-Token", token)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code, "oversized target_id should return 400")
	assert.Contains(t, rec.Body.String(), "target_id exceeds maximum length")
}

// TestRouteExecuteStream_ValidTargetTypes verifies that all recognized
// target_type values are accepted and produce a streaming response.
func TestRouteExecuteStream_ValidTargetTypes(t *testing.T) {
	t.Parallel()

	ollama := newIPv4HTTPServer(t, mockOllamaForExecute())
	defer ollama.Close()

	for _, tt := range []string{"agent", "system-container", "vm", "node"} {
		t.Run(tt, func(t *testing.T) {
			router, token := setupExecuteRouter(t, ollama.URL)

			body := `{"prompt":"check it","target_type":"` + tt + `","target_id":"test-1"}`
			req := httptest.NewRequest(http.MethodPost, "/api/ai/execute/stream", strings.NewReader(body))
			req.Header.Set("X-API-Token", token)
			rec := httptest.NewRecorder()
			router.Handler().ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code, "target_type %q should be accepted", tt)
			assert.Contains(t, rec.Header().Get("Content-Type"), "text/event-stream")

			respBody := rec.Body.String()
			assert.Contains(t, respBody, `"type":"complete"`,
				"target_type %q should produce a complete event", tt)
			assert.NotContains(t, respBody, `"type":"error"`,
				"target_type %q should not produce an error event", tt)
		})
	}
}

// TestRouteExecuteStream_EmptyTargetTypeAllowed verifies that both a missing
// and an explicit empty target_type are accepted since the field is optional.
func TestRouteExecuteStream_EmptyTargetTypeAllowed(t *testing.T) {
	t.Parallel()

	ollama := newIPv4HTTPServer(t, mockOllamaForExecute())
	defer ollama.Close()

	for _, tc := range []struct {
		name string
		body string
	}{
		{"missing", `{"prompt":"hello"}`},
		{"explicit_empty", `{"prompt":"hello","target_type":""}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router, token := setupExecuteRouter(t, ollama.URL)

			req := httptest.NewRequest(http.MethodPost, "/api/ai/execute/stream", strings.NewReader(tc.body))
			req.Header.Set("X-API-Token", token)
			rec := httptest.NewRecorder()
			router.Handler().ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code, "empty target_type should be allowed")
			assert.Contains(t, rec.Header().Get("Content-Type"), "text/event-stream")

			respBody := rec.Body.String()
			assert.Contains(t, respBody, `"type":"complete"`,
				"response should contain a complete event")
			assert.NotContains(t, respBody, `"type":"error"`,
				"response should not contain an error event")
		})
	}
}

// TestRouteExecuteStream_WithConversationHistory verifies that the streaming
// endpoint accepts and processes a request with conversation history.
func TestRouteExecuteStream_WithConversationHistory(t *testing.T) {
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
	req := httptest.NewRequest(http.MethodPost, "/api/ai/execute/stream", strings.NewReader(body))
	req.Header.Set("X-API-Token", token)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/event-stream")

	respBody := rec.Body.String()
	assert.Contains(t, respBody, `"type":"complete"`,
		"response should contain a complete event")
	assert.Contains(t, respBody, `"type":"done"`,
		"response should contain a done event")
	assert.NotContains(t, respBody, `"type":"error"`,
		"response should not contain an error event")
}

// TestRouteExecuteStream_WithTargetContext verifies that the streaming endpoint
// accepts and processes a request with target type, ID, and context.
func TestRouteExecuteStream_WithTargetContext(t *testing.T) {
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
	req := httptest.NewRequest(http.MethodPost, "/api/ai/execute/stream", strings.NewReader(body))
	req.Header.Set("X-API-Token", token)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/event-stream")

	respBody := rec.Body.String()
	assert.Contains(t, respBody, `"type":"complete"`,
		"response should contain a complete event")
	assert.Contains(t, respBody, `"type":"done"`,
		"response should contain a done event")
	assert.NotContains(t, respBody, `"type":"error"`,
		"response should not contain an error event")
}

// TestRouteExecuteStream_DefaultUseCaseIsChat verifies that when no use_case
// is specified, the endpoint defaults to "chat" and succeeds.
func TestRouteExecuteStream_DefaultUseCaseIsChat(t *testing.T) {
	t.Parallel()

	ollama := newIPv4HTTPServer(t, mockOllamaForExecute())
	defer ollama.Close()

	router, token := setupExecuteRouter(t, ollama.URL)

	body := `{"prompt":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/api/ai/execute/stream", strings.NewReader(body))
	req.Header.Set("X-API-Token", token)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/event-stream")
	assert.Contains(t, rec.Body.String(), `"type":"complete"`,
		"response should contain a complete event")
}

// TestRouteExecuteStream_FindingIDPassedThrough verifies that a request with
// a finding_id field is accepted and completes successfully.
func TestRouteExecuteStream_FindingIDPassedThrough(t *testing.T) {
	t.Parallel()

	ollama := newIPv4HTTPServer(t, mockOllamaForExecute())
	defer ollama.Close()

	router, token := setupExecuteRouter(t, ollama.URL)

	body := `{"prompt":"investigate","finding_id":"finding-abc-123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/ai/execute/stream", strings.NewReader(body))
	req.Header.Set("X-API-Token", token)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/event-stream")
	assert.Contains(t, rec.Body.String(), `"type":"complete"`,
		"response should contain a complete event")
}

// TestRouteExecuteStream_ModelFieldPassedThrough verifies that a request with
// a model field is accepted and completes successfully in streaming mode.
func TestRouteExecuteStream_ModelFieldPassedThrough(t *testing.T) {
	t.Parallel()

	ollama := newIPv4HTTPServer(t, mockOllamaForExecute())
	defer ollama.Close()

	router, token := setupExecuteRouter(t, ollama.URL)

	body := `{"prompt":"hello","model":"ollama:llama3"}`
	req := httptest.NewRequest(http.MethodPost, "/api/ai/execute/stream", strings.NewReader(body))
	req.Header.Set("X-API-Token", token)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/event-stream")
	assert.Contains(t, rec.Body.String(), `"type":"complete"`,
		"response should contain a complete event")
}
