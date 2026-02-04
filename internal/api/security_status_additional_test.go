package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecurityStatusIgnoresInvalidTokenHeader(t *testing.T) {
	rawToken := "status-valid-token-123.12345678"
	record := newTokenRecord(t, rawToken, nil, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/security/status", nil)
	req.Header.Set("X-API-Token", "invalid-token")
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for security status, got %d", rec.Code)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if hint, ok := payload["apiTokenHint"].(string); ok && hint != "" {
		t.Fatalf("expected apiTokenHint to be empty for invalid token, got %q", hint)
	}
	if _, ok := payload["tokenScopes"]; ok {
		t.Fatalf("expected tokenScopes to be omitted for invalid token")
	}
}
