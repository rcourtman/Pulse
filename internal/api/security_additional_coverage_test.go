package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/audit"
)

type auditCaptureLogger struct {
	mu     sync.Mutex
	events []audit.Event
}

func (l *auditCaptureLogger) Log(event audit.Event) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, event)
	return nil
}

func (l *auditCaptureLogger) Query(filter audit.QueryFilter) ([]audit.Event, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	events := make([]audit.Event, len(l.events))
	copy(events, l.events)
	return events, nil
}

func (l *auditCaptureLogger) Count(filter audit.QueryFilter) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.events), nil
}

func (l *auditCaptureLogger) GetWebhookURLs() []string {
	return nil
}

func (l *auditCaptureLogger) UpdateWebhookURLs(urls []string) error {
	return nil
}

func (l *auditCaptureLogger) Close() error {
	return nil
}

type auditErrorLogger struct{}

func (l *auditErrorLogger) Log(event audit.Event) error { return errors.New("log failed") }
func (l *auditErrorLogger) Query(filter audit.QueryFilter) ([]audit.Event, error) {
	return nil, errors.New("query failed")
}
func (l *auditErrorLogger) Count(filter audit.QueryFilter) (int, error) {
	return 0, errors.New("count failed")
}
func (l *auditErrorLogger) GetWebhookURLs() []string              { return nil }
func (l *auditErrorLogger) UpdateWebhookURLs(urls []string) error { return errors.New("update failed") }
func (l *auditErrorLogger) Close() error                          { return nil }

type errorLoggerFactory struct{}

func (f *errorLoggerFactory) CreateLogger(dbPath string) (audit.Logger, error) {
	return &auditErrorLogger{}, nil
}

func TestLogAuditEventForTenantFallsBackWhenManagerNil(t *testing.T) {
	capture := &auditCaptureLogger{}
	audit.SetLogger(capture)
	SetTenantAuditManager(nil)

	LogAuditEventForTenant("org-1", "event", "user", "1.2.3.4", "/path", false, "details")

	if count, _ := capture.Count(audit.QueryFilter{}); count != 1 {
		t.Fatalf("expected 1 audit event, got %d", count)
	}
}

func TestLogAuditEventForTenantFallsBackOnError(t *testing.T) {
	capture := &auditCaptureLogger{}
	audit.SetLogger(capture)

	manager := audit.NewTenantLoggerManager(t.TempDir(), &errorLoggerFactory{})
	SetTenantAuditManager(manager)

	LogAuditEventForTenant("org-2", "event", "user", "1.2.3.4", "/path", true, "details")

	if count, _ := capture.Count(audit.QueryFilter{}); count != 1 {
		t.Fatalf("expected fallback audit event, got %d", count)
	}

	SetTenantAuditManager(nil)
}

func TestInvalidateUserSessionsRemovesSessionsAndCSRF(t *testing.T) {
	resetSessionTracking()

	dir := t.TempDir()
	InitSessionStore(dir)
	InitCSRFStore(dir)

	store := GetSessionStore()

	sessionA := generateSessionToken()
	sessionB := generateSessionToken()
	store.CreateSession(sessionA, time.Hour, "agent", "127.0.0.1", "alice")
	store.CreateSession(sessionB, time.Hour, "agent", "127.0.0.1", "alice")

	TrackUserSession("alice", sessionA)
	TrackUserSession("alice", sessionB)

	tokenA := generateCSRFToken(sessionA)
	tokenB := generateCSRFToken(sessionB)
	if !GetCSRFStore().ValidateCSRFToken(sessionA, tokenA) || !GetCSRFStore().ValidateCSRFToken(sessionB, tokenB) {
		t.Fatalf("expected CSRF tokens to be valid before invalidation")
	}

	InvalidateUserSessions("alice")

	if store.GetSession(sessionA) != nil || store.GetSession(sessionB) != nil {
		t.Fatalf("expected sessions to be removed from store")
	}
	if GetCSRFStore().ValidateCSRFToken(sessionA, tokenA) || GetCSRFStore().ValidateCSRFToken(sessionB, tokenB) {
		t.Fatalf("expected CSRF tokens to be deleted")
	}
	if GetSessionUsername(sessionA) != "" || GetSessionUsername(sessionB) != "" {
		t.Fatalf("expected session tracking to be cleared")
	}
}

func TestUntrackUserSessionRemovesOnlyTarget(t *testing.T) {
	resetSessionTracking()

	TrackUserSession("bob", "sess-1")
	TrackUserSession("bob", "sess-2")

	UntrackUserSession("bob", "sess-1")

	if GetSessionUsername("sess-1") != "" {
		t.Fatalf("expected session sess-1 to be removed")
	}
	if GetSessionUsername("sess-2") != "bob" {
		t.Fatalf("expected sess-2 to remain tracked")
	}
}

func TestGetSessionUsernameFallsBackToSessionStore(t *testing.T) {
	resetSessionTracking()

	dir := t.TempDir()
	InitSessionStore(dir)

	store := GetSessionStore()
	sessionToken := generateSessionToken()
	store.CreateSession(sessionToken, time.Hour, "agent", "127.0.0.1", "carol")

	if got := GetSessionUsername(sessionToken); got != "carol" {
		t.Fatalf("expected fallback username 'carol', got %q", got)
	}
	if got := GetSessionUsername(sessionToken); got != "carol" {
		t.Fatalf("expected cached username 'carol', got %q", got)
	}
}

func TestLoadTrustedProxyCIDRsSkipsInvalidEntries(t *testing.T) {
	t.Setenv("PULSE_TRUSTED_PROXY_CIDRS", "not-a-cidr,300.0.0.1,10.0.0.0/8")
	resetTrustedProxyConfig()

	if !isTrustedProxyIP("10.1.2.3") {
		t.Fatalf("expected trusted proxy IP in valid CIDR to be accepted")
	}
}

func TestCheckCSRFWithEmptySessionValueDoesNotIssueToken(t *testing.T) {
	dir := t.TempDir()
	InitCSRFStore(dir)

	req := httptest.NewRequest(http.MethodPost, "/api/test", nil)
	req.AddCookie(&http.Cookie{
		Name:  "pulse_session",
		Value: "",
	})
	rr := httptest.NewRecorder()

	if CheckCSRF(rr, req) {
		t.Fatalf("expected CSRF check to fail with empty session value")
	}
	if rr.Header().Get("X-CSRF-Token") != "" {
		t.Fatalf("expected no CSRF token to be issued for empty session value")
	}
}
