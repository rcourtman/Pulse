package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

func TestDetectServiceName_Default(t *testing.T) {
	t.Setenv("PATH", "")

	if got := detectServiceName(); got != "pulse-backend" {
		t.Fatalf("expected pulse-backend, got %q", got)
	}
}

func TestResponseCaptureWrites(t *testing.T) {
	rec := httptest.NewRecorder()
	rc := &responseCapture{ResponseWriter: rec}

	rc.WriteHeader(http.StatusCreated)
	if !rc.wrote {
		t.Fatalf("expected wrote=true after WriteHeader")
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, rec.Code)
	}

	_, _ = rc.Write([]byte("ok"))
	if !rc.wrote {
		t.Fatalf("expected wrote=true after Write")
	}
	if rec.Body.String() != "ok" {
		t.Fatalf("expected body 'ok', got %q", rec.Body.String())
	}
}

func TestHandleRegenerateAPIToken_MissingEnvFile(t *testing.T) {
	dataDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   dataDir,
		ConfigPath: dataDir,
	}

	router := &Router{config: cfg}
	handler := http.HandlerFunc(router.HandleRegenerateAPIToken)

	authLimiter.Reset("198.51.100.9")

	req := httptest.NewRequest(http.MethodPost, "/api/security/regenerate-token", nil)
	req.RemoteAddr = "198.51.100.9:54321"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestHandleValidateAPIToken_InvalidJSON(t *testing.T) {
	router := &Router{config: &config.Config{}}
	handler := http.HandlerFunc(router.HandleValidateAPIToken)

	authLimiter.Reset("198.51.100.10")

	req := httptest.NewRequest(http.MethodPost, "/api/security/validate-token", strings.NewReader("not-json"))
	req.RemoteAddr = "198.51.100.10:54321"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestHandleValidateAPIToken_MissingToken(t *testing.T) {
	router := &Router{config: &config.Config{}}
	handler := http.HandlerFunc(router.HandleValidateAPIToken)

	authLimiter.Reset("198.51.100.11")

	req := httptest.NewRequest(http.MethodPost, "/api/security/validate-token", strings.NewReader(`{"token":""}`))
	req.RemoteAddr = "198.51.100.11:54321"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["message"] != "Token is required" {
		t.Fatalf("unexpected message: %#v", payload["message"])
	}
}

func TestHandleValidateAPIToken_NoTokensConfigured(t *testing.T) {
	router := &Router{config: &config.Config{}}
	handler := http.HandlerFunc(router.HandleValidateAPIToken)

	authLimiter.Reset("198.51.100.12")

	req := httptest.NewRequest(http.MethodPost, "/api/security/validate-token", strings.NewReader(`{"token":"abc"}`))
	req.RemoteAddr = "198.51.100.12:54321"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["message"] != "API token authentication is not configured" {
		t.Fatalf("unexpected message: %#v", payload["message"])
	}
}

func TestHandleValidateAPIToken_InvalidToken(t *testing.T) {
	hashed, err := internalauth.HashPassword("secret")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	record, err := config.NewAPITokenRecord("good-token", "token", nil)
	if err != nil {
		t.Fatalf("new token record: %v", err)
	}

	cfg := &config.Config{
		AuthUser:  "admin",
		AuthPass:  hashed,
		APITokens: []config.APITokenRecord{*record},
	}
	cfg.SortAPITokens()

	router := &Router{config: cfg}
	handler := http.HandlerFunc(router.HandleValidateAPIToken)

	authLimiter.Reset("198.51.100.13")

	req := httptest.NewRequest(http.MethodPost, "/api/security/validate-token", strings.NewReader(`{"token":"bad-token"}`))
	req.RemoteAddr = "198.51.100.13:54321"
	req.SetBasicAuth("admin", "secret")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["message"] != "Token is invalid" {
		t.Fatalf("unexpected message: %#v", payload["message"])
	}
}

func TestHandleValidateAPIToken_ValidToken(t *testing.T) {
	hashed, err := internalauth.HashPassword("secret")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	record, err := config.NewAPITokenRecord("good-token", "token", nil)
	if err != nil {
		t.Fatalf("new token record: %v", err)
	}

	cfg := &config.Config{
		AuthUser:  "admin",
		AuthPass:  hashed,
		APITokens: []config.APITokenRecord{*record},
	}
	cfg.SortAPITokens()

	router := &Router{config: cfg}
	handler := http.HandlerFunc(router.HandleValidateAPIToken)

	authLimiter.Reset("198.51.100.14")

	req := httptest.NewRequest(http.MethodPost, "/api/security/validate-token", strings.NewReader(`{"token":"good-token"}`))
	req.RemoteAddr = "198.51.100.14:54321"
	req.SetBasicAuth("admin", "secret")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["message"] != "Token is valid" {
		t.Fatalf("unexpected message: %#v", payload["message"])
	}
}
