package api

import (
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestSnapshotLocalAuthCredentialsLockedDoesNotReenterConfigLock(t *testing.T) {
	cfg := &config.Config{AuthUser: "admin", AuthPass: "hash"}

	// Hold the same outer read lock as checkAuth, then queue a writer exactly as
	// API-token validation does. Go's RWMutex blocks any new reader behind that
	// writer, so a recursive RLock in the snapshot would never return.
	config.Mu.RLock()
	outerLocked := true
	defer func() {
		if outerLocked {
			config.Mu.RUnlock()
		}
	}()

	writerAttempting := make(chan struct{})
	writerAcquired := make(chan struct{})
	writerDone := make(chan struct{})
	go func() {
		close(writerAttempting)
		config.Mu.Lock()
		close(writerAcquired)
		config.Mu.Unlock()
		close(writerDone)
	}()
	<-writerAttempting
	runtime.Gosched()

	snapshotDone := make(chan [2]string, 1)
	go func() {
		user, pass := snapshotLocalAuthCredentialsLocked(cfg)
		snapshotDone <- [2]string{user, pass}
	}()

	select {
	case got := <-snapshotDone:
		if got != [2]string{"admin", "hash"} {
			t.Fatalf("credential snapshot = %#v, want admin/hash", got)
		}
	case <-writerAcquired:
		t.Fatal("writer acquired config.Mu while the outer read lock was held")
	case <-time.After(250 * time.Millisecond):
		config.Mu.RUnlock()
		outerLocked = false
		<-writerDone
		t.Fatal("credential snapshot blocked behind a pending config writer; recursive RLock deadlock regressed")
	}

	config.Mu.RUnlock()
	outerLocked = false
	select {
	case <-writerDone:
	case <-time.After(time.Second):
		t.Fatal("queued config writer did not complete after releasing outer read lock")
	}
}

func TestCheckAuth_APIOnlyModeRequiresToken(t *testing.T) {
	record, err := config.NewAPITokenRecord("token-required-123.12345678", "api", []string{config.ScopeMonitoringRead})
	if err != nil {
		t.Fatalf("NewAPITokenRecord: %v", err)
	}
	cfg := &config.Config{
		APITokens: []config.APITokenRecord{*record},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	rr := httptest.NewRecorder()

	if CheckAuth(cfg, rr, req) {
		t.Fatalf("expected CheckAuth to fail without token in API-only mode")
	}
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "API token required") {
		t.Fatalf("expected API token required message, got %q", rr.Body.String())
	}
	if rr.Header().Get("WWW-Authenticate") == "" {
		t.Fatalf("expected WWW-Authenticate header to be set")
	}
}

func TestCheckAuth_APIOnlyModeRejectsInvalidToken(t *testing.T) {
	record, err := config.NewAPITokenRecord("token-valid-123.12345678", "api", []string{config.ScopeMonitoringRead})
	if err != nil {
		t.Fatalf("NewAPITokenRecord: %v", err)
	}
	cfg := &config.Config{
		APITokens: []config.APITokenRecord{*record},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("X-API-Token", "token-invalid-123.12345678")
	rr := httptest.NewRecorder()

	if CheckAuth(cfg, rr, req) {
		t.Fatalf("expected CheckAuth to fail with invalid token")
	}
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestCheckAuth_APIOnlyModeAcceptsQueryToken(t *testing.T) {
	rawToken := "token-query-123.12345678"
	record, err := config.NewAPITokenRecord(rawToken, "api", []string{config.ScopeMonitoringRead})
	if err != nil {
		t.Fatalf("NewAPITokenRecord: %v", err)
	}
	cfg := &config.Config{
		APITokens: []config.APITokenRecord{*record},
	}

	// Query-string tokens are rejected on regular HTTP to prevent URL-based leakage.
	req := httptest.NewRequest(http.MethodGet, "/api/test?token="+rawToken, nil)
	rr := httptest.NewRecorder()

	if CheckAuth(cfg, rr, req) {
		t.Fatalf("expected CheckAuth to reject query token on regular HTTP request")
	}

	// Query-string tokens are accepted on WebSocket upgrade requests.
	wsReq := httptest.NewRequest(http.MethodGet, "/api/test?token="+rawToken, nil)
	wsReq.Header.Set("Upgrade", "websocket")
	wsReq.Header.Set("Connection", "Upgrade")
	wsRR := httptest.NewRecorder()

	if !CheckAuth(cfg, wsRR, wsReq) {
		t.Fatalf("expected CheckAuth to succeed with query token on WebSocket upgrade")
	}
	if wsRR.Header().Get("X-Auth-Method") != "api_token" {
		t.Fatalf("expected X-Auth-Method api_token, got %q", wsRR.Header().Get("X-Auth-Method"))
	}
}

func TestCheckAuth_AcceptsBearerToken(t *testing.T) {
	rawToken := "token-bearer-123.12345678"
	record, err := config.NewAPITokenRecord(rawToken, "api", []string{config.ScopeMonitoringRead})
	if err != nil {
		t.Fatalf("NewAPITokenRecord: %v", err)
	}
	cfg := &config.Config{
		APITokens: []config.APITokenRecord{*record},
		AuthUser:  "admin",
		AuthPass:  "$2a$10$dummy",
	}

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Authorization", "Bearer "+rawToken)
	rr := httptest.NewRecorder()

	if !CheckAuth(cfg, rr, req) {
		t.Fatalf("expected CheckAuth to succeed with bearer token")
	}
	if rr.Header().Get("X-Auth-Method") != "api_token" {
		t.Fatalf("expected X-Auth-Method api_token, got %q", rr.Header().Get("X-Auth-Method"))
	}
}

func TestCheckAuth_InvalidBearerDoesNotFallBackToSession(t *testing.T) {
	record, err := config.NewAPITokenRecord("token-valid-123.12345678", "api", []string{config.ScopeMonitoringRead})
	if err != nil {
		t.Fatalf("NewAPITokenRecord: %v", err)
	}
	cfg := &config.Config{
		APITokens: []config.APITokenRecord{*record},
		AuthUser:  "admin",
		AuthPass:  "$2a$10$dummy",
	}

	sessionToken := generateSessionToken()
	GetSessionStore().CreateSession(sessionToken, time.Hour, "agent", "127.0.0.1", "alice")

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Authorization", "Bearer token-invalid-123.12345678")
	req.AddCookie(&http.Cookie{Name: "pulse_session", Value: sessionToken})
	rr := httptest.NewRecorder()

	if CheckAuth(cfg, rr, req) {
		t.Fatalf("expected CheckAuth to reject invalid bearer token even with a valid session")
	}
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Invalid API token") {
		t.Fatalf("expected invalid token response, got %q", rr.Body.String())
	}
	if rr.Header().Get("X-Auth-Method") == "session" {
		t.Fatalf("did not expect session auth fallback for invalid bearer token")
	}
}

func TestCheckAuth_NilConfigFailsClosed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	rr := httptest.NewRecorder()

	if CheckAuth(nil, rr, req) {
		t.Fatalf("expected CheckAuth to fail when config is nil")
	}
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Authentication unavailable") {
		t.Fatalf("expected authentication unavailable message, got %q", rr.Body.String())
	}
}
