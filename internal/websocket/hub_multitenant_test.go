package websocket

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/pkg/audit"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

type fakeMultiTenantChecker struct {
	result MultiTenantCheckResult
}

func (f fakeMultiTenantChecker) CheckMultiTenant(ctx context.Context, orgID string) MultiTenantCheckResult {
	return f.result
}

type fakeOrgAuthChecker struct {
	called bool
	allow  bool
}

func (f *fakeOrgAuthChecker) CanAccessOrg(userID string, token interface{}, orgID string) bool {
	f.called = true
	return f.allow
}

type testAuditLogger struct {
	mu     sync.Mutex
	events []audit.Event
}

func (l *testAuditLogger) Log(event audit.Event) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, event)
	return nil
}

func (l *testAuditLogger) Query(filter audit.QueryFilter) ([]audit.Event, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]audit.Event, len(l.events))
	copy(out, l.events)
	return out, nil
}

func (l *testAuditLogger) Count(filter audit.QueryFilter) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.events), nil
}

func (l *testAuditLogger) GetWebhookURLs() []string {
	return nil
}

func (l *testAuditLogger) UpdateWebhookURLs(urls []string) error {
	return nil
}

func (l *testAuditLogger) Close() error {
	return nil
}

func setAuditLogger(t *testing.T, logger audit.Logger) {
	t.Helper()
	prev := audit.GetLogger()
	audit.SetLogger(logger)
	t.Cleanup(func() { audit.SetLogger(prev) })
}

func TestHandleWebSocket_MultiTenantDisabled(t *testing.T) {
	hub := NewHub(nil)
	hub.SetMultiTenantChecker(fakeMultiTenantChecker{
		result: MultiTenantCheckResult{
			Allowed:        false,
			FeatureEnabled: false,
			Licensed:       false,
			Reason:         "disabled",
		},
	})

	req := httptest.NewRequest(http.MethodGet, "http://example.com/ws", nil)
	req.Header.Set("X-Pulse-Org-ID", "tenant-a")
	rec := httptest.NewRecorder()

	hub.HandleWebSocket(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("expected status %d, got %d", http.StatusNotImplemented, rec.Code)
	}
}

func TestHandleWebSocket_MultiTenantUnlicensed(t *testing.T) {
	logger := &testAuditLogger{}
	setAuditLogger(t, logger)

	hub := NewHub(nil)
	hub.SetMultiTenantChecker(fakeMultiTenantChecker{
		result: MultiTenantCheckResult{
			Allowed:        false,
			FeatureEnabled: true,
			Licensed:       false,
			Reason:         "unlicensed",
		},
	})

	req := httptest.NewRequest(http.MethodGet, "http://example.com/ws", nil)
	req.Header.Set("X-Pulse-Org-ID", "tenant-a")
	req.RemoteAddr = "203.0.113.10:43120"
	rec := httptest.NewRecorder()

	hub.HandleWebSocket(rec, req)

	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("expected status %d, got %d", http.StatusPaymentRequired, rec.Code)
	}

	events, _ := logger.Query(audit.QueryFilter{})
	if len(events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(events))
	}
	if events[0].EventType != "websocket_multitenant_access_denied" {
		t.Fatalf("expected event websocket_multitenant_access_denied, got %q", events[0].EventType)
	}
	if events[0].IP != "203.0.113.10" {
		t.Fatalf("expected extracted peer IP, got %q", events[0].IP)
	}
	if events[0].Success {
		t.Fatalf("expected failed audit event for denied connection")
	}
	if !strings.Contains(events[0].Details, "org_id=tenant-a") {
		t.Fatalf("expected org ID in audit details, got %q", events[0].Details)
	}
}

func TestHandleWebSocket_OrgAuthorizationDenied(t *testing.T) {
	logger := &testAuditLogger{}
	setAuditLogger(t, logger)

	hub := NewHub(nil)
	authChecker := &fakeOrgAuthChecker{allow: false}
	hub.SetOrgAuthChecker(authChecker)
	hub.SetMultiTenantChecker(fakeMultiTenantChecker{
		result: MultiTenantCheckResult{
			Allowed:        true,
			FeatureEnabled: true,
			Licensed:       true,
			Reason:         "allowed",
		},
	})

	req := httptest.NewRequest(http.MethodGet, "http://example.com/ws", nil)
	req = req.WithContext(auth.WithUser(req.Context(), "alice"))
	req.Header.Set("X-Pulse-Org-ID", "tenant-a")
	req.RemoteAddr = "198.51.100.12:5001"
	rec := httptest.NewRecorder()

	hub.HandleWebSocket(rec, req)

	if !authChecker.called {
		t.Fatalf("expected org auth checker to be called")
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}

	events, _ := logger.Query(audit.QueryFilter{})
	if len(events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(events))
	}
	if events[0].EventType != "websocket_org_access_denied" {
		t.Fatalf("expected event websocket_org_access_denied, got %q", events[0].EventType)
	}
	if events[0].User != "alice" {
		t.Fatalf("expected user alice in audit event, got %q", events[0].User)
	}
	if events[0].IP != "198.51.100.12" {
		t.Fatalf("expected extracted peer IP, got %q", events[0].IP)
	}
	if events[0].Path != "/ws" {
		t.Fatalf("expected path /ws in audit event, got %q", events[0].Path)
	}
	if !strings.Contains(events[0].Details, "org_id=tenant-a") {
		t.Fatalf("expected org ID in audit details, got %q", events[0].Details)
	}
}

func TestHandleWebSocket_InvalidOrgIDRejected(t *testing.T) {
	hub := NewHub(nil)

	req := httptest.NewRequest(http.MethodGet, "http://example.com/ws", nil)
	req.Header.Set("X-Pulse-Org-ID", "../tenant-a")
	rec := httptest.NewRecorder()

	hub.HandleWebSocket(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestHandleWebSocket_OversizedOrgIDRejected(t *testing.T) {
	hub := NewHub(nil)

	req := httptest.NewRequest(http.MethodGet, "http://example.com/ws", nil)
	req.Header.Set("X-Pulse-Org-ID", strings.Repeat("a", maxWebSocketOrgIDLength+1))
	rec := httptest.NewRecorder()

	hub.HandleWebSocket(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}
