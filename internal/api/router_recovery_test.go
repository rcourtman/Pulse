package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

func newRecoveryRouter(t *testing.T) *Router {
	t.Helper()
	t.Setenv("PULSE_TRUSTED_PROXY_CIDRS", "")
	resetTrustedProxyConfig()

	dir := t.TempDir()
	hashed, err := internalauth.HashPassword("Password!1")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	cfg := &config.Config{
		AuthUser:   "admin",
		AuthPass:   hashed,
		DataPath:   dir,
		ConfigPath: dir,
	}

	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	recoveryFile := filepath.Join(cfg.DataPath, ".auth_recovery")
	if err := os.WriteFile(recoveryFile, []byte("recovery enabled"), 0600); err != nil {
		t.Fatalf("write recovery file: %v", err)
	}

	return router
}

func TestAuthRecoveryAllowsDirectLoopback(t *testing.T) {
	router := newRecoveryRouter(t)

	router.mux.HandleFunc("/api/secure", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/secure", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d (%s)", http.StatusOK, rec.Code, rec.Body.String())
	}
	if rec.Header().Get("X-Auth-Recovery") != "true" {
		t.Fatalf("expected X-Auth-Recovery header to be set")
	}
}

func TestAuthRecoveryRejectsForwardedLoopback(t *testing.T) {
	router := newRecoveryRouter(t)

	router.mux.HandleFunc("/api/secure", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/secure", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "127.0.0.1")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d (%s)", http.StatusUnauthorized, rec.Code, rec.Body.String())
	}
}

func TestRecoveryEndpointDisableAuthRequiresLoopbackOrToken(t *testing.T) {
	router := newRecoveryRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/api/security/recovery", strings.NewReader(`{"action":"disable_auth"}`))
	req.RemoteAddr = "203.0.113.50:12345"
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d (%s)", http.StatusForbidden, rec.Code, rec.Body.String())
	}
}

func TestRecoveryEndpointDisableAuthAllowsLoopback(t *testing.T) {
	router := newRecoveryRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/api/security/recovery", strings.NewReader(`{"action":"disable_auth"}`))
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d (%s)", http.StatusOK, rec.Code, rec.Body.String())
	}
}

func TestRecoveryEndpointDisableAuthAllowsValidToken(t *testing.T) {
	router := newRecoveryRouter(t)
	InitRecoveryTokenStore(router.config.DataPath)
	token, err := GetRecoveryTokenStore().GenerateRecoveryToken(5 * time.Minute)
	if err != nil {
		t.Fatalf("generate recovery token: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/security/recovery", strings.NewReader(`{"action":"disable_auth"}`))
	req.RemoteAddr = "203.0.113.51:12345"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Recovery-Token", token)
	rec := httptest.NewRecorder()

	router.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d (%s)", http.StatusOK, rec.Code, rec.Body.String())
	}
}

func TestRecoveryEndpointEnableAuthRemovesFile(t *testing.T) {
	router := newRecoveryRouter(t)
	InitRecoveryTokenStore(router.config.DataPath)
	token, err := GetRecoveryTokenStore().GenerateRecoveryToken(5 * time.Minute)
	if err != nil {
		t.Fatalf("generate recovery token: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/security/recovery", strings.NewReader(`{"action":"enable_auth"}`))
	req.RemoteAddr = "203.0.113.52:12345"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Recovery-Token", token)
	rec := httptest.NewRecorder()

	router.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d (%s)", http.StatusOK, rec.Code, rec.Body.String())
	}

	recoveryFile := filepath.Join(router.config.DataPath, ".auth_recovery")
	if _, err := os.Stat(recoveryFile); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected recovery file to be removed, got err=%v", err)
	}
}

func TestRecoveryEndpointGenerateTokenRequiresLoopback(t *testing.T) {
	router := newRecoveryRouter(t)
	resetRecoveryStore()
	InitRecoveryTokenStore(router.config.DataPath)

	req := httptest.NewRequest(http.MethodPost, "/api/security/recovery", strings.NewReader(`{"action":"generate_token"}`))
	req.RemoteAddr = "203.0.113.60:12345"
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d (%s)", http.StatusForbidden, rec.Code, rec.Body.String())
	}
}

func TestRecoveryEndpointGenerateTokenRejectsRemoteToken(t *testing.T) {
	router := newRecoveryRouter(t)
	resetRecoveryStore()
	InitRecoveryTokenStore(router.config.DataPath)
	token, err := GetRecoveryTokenStore().GenerateRecoveryToken(5 * time.Minute)
	if err != nil {
		t.Fatalf("generate recovery token: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/security/recovery", strings.NewReader(`{"action":"generate_token"}`))
	req.RemoteAddr = "203.0.113.61:12345"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Recovery-Token", token)
	rec := httptest.NewRecorder()

	router.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d (%s)", http.StatusForbidden, rec.Code, rec.Body.String())
	}
}

func TestRecoveryEndpointGenerateTokenLoopbackSuccess(t *testing.T) {
	router := newRecoveryRouter(t)
	resetRecoveryStore()
	InitRecoveryTokenStore(router.config.DataPath)

	req := httptest.NewRequest(http.MethodPost, "/api/security/recovery", strings.NewReader(`{"action":"generate_token"}`))
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d (%s)", http.StatusOK, rec.Code, rec.Body.String())
	}

	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if ok, _ := payload["success"].(bool); !ok {
		t.Fatalf("expected success=true, got %#v", payload["success"])
	}
	if token, _ := payload["token"].(string); token == "" {
		t.Fatalf("expected token in response, got %#v", payload["token"])
	}
}

func TestRecoveryEndpointInvalidAction(t *testing.T) {
	router := newRecoveryRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/api/security/recovery", strings.NewReader(`{"action":"not-valid"}`))
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d (%s)", http.StatusOK, rec.Code, rec.Body.String())
	}

	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if ok, _ := payload["success"].(bool); ok {
		t.Fatalf("expected success=false, got %#v", payload["success"])
	}
	if msg, _ := payload["message"].(string); !strings.Contains(msg, "Invalid action") {
		t.Fatalf("unexpected message: %q", msg)
	}
}
