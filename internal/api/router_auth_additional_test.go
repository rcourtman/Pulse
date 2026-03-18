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
	"time"

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
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("admin:currentpassword")))
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
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("admin:currentpassword")))
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
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("admin:currentpassword")))
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
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("admin:currentpassword")))
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

// TestHandleChangePassword_InvalidatesSessionsDocker verifies the security-critical
// invariant that all existing sessions are invalidated after a successful password
// change. Without this, stolen session tokens remain valid even after the user
// changes their password.
func TestHandleChangePassword_InvalidatesSessionsDocker(t *testing.T) {
	resetSessionTracking()
	authLimiter.Reset("192.0.2.1")
	defer authLimiter.Reset("192.0.2.1")

	dir := t.TempDir()
	InitSessionStore(dir)
	InitCSRFStore(dir)
	t.Setenv("PULSE_DOCKER", "true")

	router := newAuthRouter(t)
	store := GetSessionStore()

	// Create two active sessions for the admin user.
	sessA := generateSessionToken()
	sessB := generateSessionToken()
	store.CreateSession(sessA, time.Hour, "browser", "10.0.0.1", "admin")
	store.CreateSession(sessB, time.Hour, "browser", "10.0.0.2", "admin")
	TrackUserSession("admin", sessA)
	TrackUserSession("admin", sessB)

	// Generate CSRF tokens tied to the sessions.
	csrfA := generateCSRFToken(sessA)
	csrfB := generateCSRFToken(sessB)

	// Sanity: sessions and CSRF tokens are valid before password change.
	if !ValidateSession(sessA) || !ValidateSession(sessB) {
		t.Fatal("sessions must be valid before password change")
	}
	if !GetCSRFStore().ValidateCSRFToken(sessA, csrfA) || !GetCSRFStore().ValidateCSRFToken(sessB, csrfB) {
		t.Fatal("CSRF tokens must be valid before password change")
	}

	// Execute password change.
	body := `{"currentPassword":"currentpassword","newPassword":"newpassword123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/change-password", strings.NewReader(body))
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("admin:currentpassword")))
	rec := httptest.NewRecorder()

	router.handleChangePassword(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("password change failed: status = %d, body = %s", rec.Code, rec.Body.String())
	}

	// Sessions must be invalidated.
	if ValidateSession(sessA) {
		t.Error("session A must be invalid after password change")
	}
	if ValidateSession(sessB) {
		t.Error("session B must be invalid after password change")
	}

	// CSRF tokens tied to those sessions must also be gone.
	if GetCSRFStore().ValidateCSRFToken(sessA, csrfA) {
		t.Error("CSRF token A must be invalid after password change")
	}
	if GetCSRFStore().ValidateCSRFToken(sessB, csrfB) {
		t.Error("CSRF token B must be invalid after password change")
	}

	// Session-to-user tracking must be cleared.
	if GetSessionUsername(sessA) != "" {
		t.Error("session A user tracking must be cleared after password change")
	}
	if GetSessionUsername(sessB) != "" {
		t.Error("session B user tracking must be cleared after password change")
	}
}

// TestHandleChangePassword_InvalidatesSessionsNonDocker verifies the same
// session invalidation invariant for the non-Docker (systemd/manual) code path.
func TestHandleChangePassword_InvalidatesSessionsNonDocker(t *testing.T) {
	resetSessionTracking()
	authLimiter.Reset("192.0.2.1")
	defer authLimiter.Reset("192.0.2.1")

	dir := t.TempDir()
	InitSessionStore(dir)
	InitCSRFStore(dir)
	t.Setenv("PULSE_DOCKER", "false")

	router := newAuthRouter(t)
	// Point ConfigPath to a writable temp dir so .env write succeeds.
	router.config.ConfigPath = dir
	store := GetSessionStore()

	// Create an active session.
	sess := generateSessionToken()
	store.CreateSession(sess, time.Hour, "browser", "10.0.0.1", "admin")
	TrackUserSession("admin", sess)

	csrf := generateCSRFToken(sess)

	// Sanity: session and CSRF token are valid before password change.
	if !ValidateSession(sess) {
		t.Fatal("session must be valid before password change")
	}
	if !GetCSRFStore().ValidateCSRFToken(sess, csrf) {
		t.Fatal("CSRF token must be valid before password change")
	}

	body := `{"currentPassword":"currentpassword","newPassword":"newpassword123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/change-password", strings.NewReader(body))
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("admin:currentpassword")))
	rec := httptest.NewRecorder()

	router.handleChangePassword(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("password change failed: status = %d, body = %s", rec.Code, rec.Body.String())
	}

	if ValidateSession(sess) {
		t.Error("session must be invalid after password change")
	}
	if GetCSRFStore().ValidateCSRFToken(sess, csrf) {
		t.Error("CSRF token must be invalid after password change")
	}
	if GetSessionUsername(sess) != "" {
		t.Error("session user tracking must be cleared after password change")
	}
}
