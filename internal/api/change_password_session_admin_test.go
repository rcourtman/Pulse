package api

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"golang.org/x/crypto/bcrypt"
)

const changePasswordProbeCurrent = "correct-admin-password"

func newChangePasswordRouter(t *testing.T) (*Router, *config.Config) {
	t.Helper()
	dir := t.TempDir()
	hashed, err := bcrypt.GenerateFromPassword([]byte(changePasswordProbeCurrent), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	cfg := &config.Config{DataPath: dir, ConfigPath: dir, AuthUser: "admin", AuthPass: string(hashed)}
	return NewRouter(cfg, nil, nil, nil, nil, "1.0.0"), cfg
}

func changePasswordRequest(t *testing.T, current string) *http.Request {
	t.Helper()
	body := `{"currentPassword":"` + current + `","newPassword":"Str0ng-New-Passw0rd!x"}`
	req := httptest.NewRequest(http.MethodPost, "/api/security/change-password", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func attachChangePasswordSession(t *testing.T, req *http.Request, user string) {
	t.Helper()
	tok := "change-pw-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	GetSessionStore().CreateSession(tok, time.Hour, "browser", "127.0.0.1", user)
	req.AddCookie(&http.Cookie{Name: sessionCookieName(false), Value: tok})
	req.Header.Set("X-CSRF-Token", generateCSRFToken(tok))
}

// The proxy branch of handleChangePassword refuses non-admin proxy users. The
// session branch did not, so any authenticated non-admin session could submit
// guesses at the local admin password and read the answer from the 401, and
// could change it outright once it knew it.
func TestChangePasswordRefusesNonAdminSession(t *testing.T) {
	router, _ := newChangePasswordRouter(t)

	req := changePasswordRequest(t, changePasswordProbeCurrent)
	attachChangePasswordSession(t, req, "sso:outsider@example.com")
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("non-admin session change-password = %d, want 403 (body %s)", rec.Code, rec.Body.String())
	}
}

// The configured admin must still be able to change their own password.
func TestChangePasswordAllowsConfiguredAdminSession(t *testing.T) {
	router, _ := newChangePasswordRouter(t)

	req := changePasswordRequest(t, changePasswordProbeCurrent)
	attachChangePasswordSession(t, req, "admin")
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("configured admin change-password = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
}

// Basic Auth carries no session cookie, so the session gate must not touch it.
func TestChangePasswordBasicAuthPathUnaffected(t *testing.T) {
	router, _ := newChangePasswordRouter(t)

	req := changePasswordRequest(t, changePasswordProbeCurrent)
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString(
		[]byte("admin:"+changePasswordProbeCurrent)))
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	if rec.Code == http.StatusForbidden {
		t.Fatalf("Basic Auth change-password must not be refused by the session gate, got 403 (body %s)", rec.Body.String())
	}
}
