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
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

const changePasswordProbeCurrent = "correct-admin-password"

func newChangePasswordRouter(t *testing.T) (*Router, *config.Config) {
	t.Helper()
	dir := t.TempDir()
	hashed, err := internalauth.HashPassword(changePasswordProbeCurrent)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	cfg := &config.Config{DataPath: dir, ConfigPath: dir, AuthUser: "admin", AuthPass: hashed}
	router := newTestRouter(t, cfg, nil, nil, nil, nil, "1.0.0")
	cleanupTestRouter(t, router)
	return router, cfg
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

func newChangePasswordRouterWithToken(t *testing.T, rawToken string, scopes []string) (*Router, *config.Config) {
	t.Helper()
	dir := t.TempDir()
	hashed, err := internalauth.HashPassword(changePasswordProbeCurrent)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	record, err := config.NewAPITokenRecord(rawToken, "change-password-caller", scopes)
	if err != nil {
		t.Fatalf("NewAPITokenRecord: %v", err)
	}
	cfg := &config.Config{
		DataPath: dir, ConfigPath: dir,
		AuthUser: "admin", AuthPass: hashed,
		APITokens: []config.APITokenRecord{*record},
	}
	cfg.SortAPITokens()
	router := newTestRouter(t, cfg, nil, nil, nil, nil, "1.0.0")
	cleanupTestRouter(t, router)
	return router, cfg
}

// ensureAdminSession judges sessions and leaves a token's scope to the route,
// and this route asked for none: the report token of one monitored host could
// submit guesses at the admin password and replace it on a hit.
func TestChangePasswordRefusesTokenWithoutSettingsWrite(t *testing.T) {
	const rawToken = "change-password-agent-token.12345678"
	router, cfg := newChangePasswordRouterWithToken(t, rawToken, []string{config.ScopeAgentReport, config.ScopeAgentConfigRead})
	before := cfg.AuthPass

	for _, current := range []string{"wrong-guess", changePasswordProbeCurrent} {
		req := changePasswordRequest(t, current)
		req.Header.Set("X-API-Token", rawToken)
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("agent token change-password (%q) = %d, want 403 (body %s)", current, rec.Code, rec.Body.String())
		}
	}
	if cfg.AuthPass != before {
		t.Fatal("agent token changed the admin password")
	}
}

// Sign-in counts failures per account as well as per address. Password change
// tests the same password, so guesses spread over addresses must meet the same
// account lockout rather than five fresh attempts per address.
func TestChangePasswordCountsFailuresAgainstTheAccount(t *testing.T) {
	resetFailedLogins()
	t.Cleanup(resetFailedLogins)

	const rawToken = "change-password-settings-token.12345678"
	router, cfg := newChangePasswordRouterWithToken(t, rawToken, []string{config.ScopeSettingsWrite})
	before := cfg.AuthPass

	guessFrom := func(address, current string) *httptest.ResponseRecorder {
		req := changePasswordRequest(t, current)
		req.RemoteAddr = address + ":54321"
		req.Header.Set("X-API-Token", rawToken)
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		return rec
	}

	for i := 0; i < maxFailedAttempts; i++ {
		if rec := guessFrom("203.0.113."+strconv.Itoa(10+i), "wrong-guess"); rec.Code != http.StatusUnauthorized {
			t.Fatalf("wrong guess %d = %d, want 401 (body %s)", i+1, rec.Code, rec.Body.String())
		}
	}
	if _, _, locked := GetLockoutInfo(cfg.AuthUser); !locked {
		t.Fatalf("%d wrong current passwords from different addresses did not lock the account", maxFailedAttempts)
	}
	if rec := guessFrom("203.0.113.200", changePasswordProbeCurrent); rec.Code != http.StatusForbidden {
		t.Fatalf("change-password on a locked account = %d, want 403 (body %s)", rec.Code, rec.Body.String())
	}
	if cfg.AuthPass != before {
		t.Fatal("password changed while the account was locked")
	}
}

func TestChangePasswordFixtureOwnsRouterLifecycle(t *testing.T) {
	var router *Router
	workerDone := make(chan struct{})
	t.Run("fixture", func(t *testing.T) {
		router, _ = newChangePasswordRouter(t)
		router.startLifecycleWorker(func() {
			<-router.lifecycleCtx.Done()
			close(workerDone)
		})
	})
	// Ensure a failing regression does not itself leave workers running.
	cleanupTestRouter(t, router)
	select {
	case <-router.lifecycleCtx.Done():
	default:
		t.Fatal("password-change fixture left router lifecycle active")
	}
	select {
	case <-workerDone:
	default:
		t.Fatal("password-change fixture did not join its router worker")
	}
}
