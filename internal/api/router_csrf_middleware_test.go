package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

func newRouterWithSession(t *testing.T) (*Router, string) {
	t.Helper()
	t.Setenv("PULSE_TRUSTED_PROXY_CIDRS", "")
	resetTrustedProxyConfig()

	dir := t.TempDir()
	InitSessionStore(dir)
	InitCSRFStore(dir)

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

	router := &Router{
		mux:    http.NewServeMux(),
		config: cfg,
	}

	token := generateSessionToken()
	GetSessionStore().CreateSession(token, time.Hour, "test-agent", "127.0.0.1", "admin")

	return router, token
}

func TestRouterCSRFEnforcedForSessionRequests(t *testing.T) {
	router, sessionToken := newRouterWithSession(t)

	router.mux.HandleFunc("/api/secure", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/secure", nil)
	req.AddCookie(&http.Cookie{Name: "pulse_session", Value: sessionToken})
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d (%s)", http.StatusForbidden, rec.Code, rec.Body.String())
	}
}

func TestRouterCSRFAllowsValidToken(t *testing.T) {
	router, sessionToken := newRouterWithSession(t)

	router.mux.HandleFunc("/api/secure", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	csrfToken := generateCSRFToken(sessionToken)
	req := httptest.NewRequest(http.MethodPost, "/api/secure", nil)
	req.AddCookie(&http.Cookie{Name: "pulse_session", Value: sessionToken})
	req.Header.Set("X-CSRF-Token", csrfToken)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d (%s)", http.StatusOK, rec.Code, rec.Body.String())
	}
}

func TestRouterCSRFSpecialCaseSkips(t *testing.T) {
	router, sessionToken := newRouterWithSession(t)

	skipPaths := []string{
		"/api/login",
		"/api/security/apply-restart",
		"/api/security/quick-setup",
		"/api/security/validate-bootstrap-token",
		"/api/setup-script-url",
	}

	for _, path := range skipPaths {
		router.mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodPost, path, nil)
		req.AddCookie(&http.Cookie{Name: "pulse_session", Value: sessionToken})
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("path %s: expected status %d, got %d (%s)", path, http.StatusOK, rec.Code, rec.Body.String())
		}
	}
}

func TestRouterIssuesCSRFCookieForGetWithSession(t *testing.T) {
	router, sessionToken := newRouterWithSession(t)

	router.mux.HandleFunc("/api/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/ping", nil)
	req.AddCookie(&http.Cookie{Name: "pulse_session", Value: sessionToken})
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d (%s)", http.StatusOK, rec.Code, rec.Body.String())
	}

	var csrfCookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == "pulse_csrf" {
			csrfCookie = c
			break
		}
	}
	if csrfCookie == nil {
		t.Fatalf("expected pulse_csrf cookie to be set")
	}
	if csrfCookie.Value == "" {
		t.Fatalf("expected pulse_csrf cookie to have a value")
	}
}

func TestRouterDoesNotIssueCSRFCookieWithoutSession(t *testing.T) {
	router, _ := newRouterWithSession(t)

	router.mux.HandleFunc("/api/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/ping", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d (%s)", http.StatusUnauthorized, rec.Code, rec.Body.String())
	}

	for _, c := range rec.Result().Cookies() {
		if c.Name == "pulse_csrf" {
			t.Fatalf("did not expect pulse_csrf cookie to be set")
		}
	}
}
