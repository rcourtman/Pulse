package api

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

func newAuthRouter(t *testing.T) *Router {
	t.Helper()
	hashed, err := auth.HashPassword("currentpassword")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	return &Router{
		config: &config.Config{
			AuthUser:   "admin",
			AuthPass:   hashed,
			ConfigPath: t.TempDir(),
		},
	}
}

func TestHandleChangePassword_InvalidJSON(t *testing.T) {
	router := newAuthRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/api/change-password", strings.NewReader("{"))
	rec := httptest.NewRecorder()

	router.handleChangePassword(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["code"] != "invalid_request" {
		t.Fatalf("expected invalid_request, got %#v", payload["code"])
	}
}

func TestHandleChangePassword_InvalidPassword(t *testing.T) {
	router := newAuthRouter(t)
	body := `{"currentPassword":"currentpassword","newPassword":"short"}`
	req := httptest.NewRequest(http.MethodPost, "/api/change-password", strings.NewReader(body))
	rec := httptest.NewRecorder()

	router.handleChangePassword(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["code"] != "invalid_password" {
		t.Fatalf("expected invalid_password, got %#v", payload["code"])
	}
}

func TestHandleChangePassword_MissingCurrent(t *testing.T) {
	router := newAuthRouter(t)
	body := `{"currentPassword":"","newPassword":"newpassword123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/change-password", strings.NewReader(body))
	rec := httptest.NewRecorder()

	router.handleChangePassword(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["code"] != "unauthorized" {
		t.Fatalf("expected unauthorized, got %#v", payload["code"])
	}
}

func TestHandleChangePassword_SuccessDocker(t *testing.T) {
	router := newAuthRouter(t)
	t.Setenv("PULSE_DOCKER", "true")

	body := `{"currentPassword":"currentpassword","newPassword":"newpassword123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/change-password", strings.NewReader(body))
	rec := httptest.NewRecorder()

	router.handleChangePassword(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if ok, _ := payload["success"].(bool); !ok {
		t.Fatalf("expected success=true, got %#v", payload["success"])
	}
	envPath := filepath.Join(router.config.ConfigPath, ".env")
	if _, err := os.Stat(envPath); err != nil {
		t.Fatalf("expected .env to be written, got error: %v", err)
	}
}

func TestHandleResetLockout_InvalidJSON(t *testing.T) {
	router := newAuthRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/api/reset-lockout", strings.NewReader("{"))
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("admin:currentpassword")))
	rec := httptest.NewRecorder()

	router.handleResetLockout(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleResetLockout_MissingIdentifier(t *testing.T) {
	router := newAuthRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/api/reset-lockout", strings.NewReader(`{"identifier":""}`))
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("admin:currentpassword")))
	rec := httptest.NewRecorder()

	router.handleResetLockout(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleResetLockout_Success(t *testing.T) {
	router := newAuthRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/api/reset-lockout", strings.NewReader(`{"identifier":"user1"}`))
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("admin:currentpassword")))
	rec := httptest.NewRecorder()

	router.handleResetLockout(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if ok, _ := payload["success"].(bool); !ok {
		t.Fatalf("expected success=true, got %#v", payload["success"])
	}
}
