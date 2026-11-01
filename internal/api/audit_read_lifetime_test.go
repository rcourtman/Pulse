package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/audit"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

// Only synthetic in-memory evidence: hold the real handlers' storage read,
// after actual Router authentication/permission/licence/scope admission.
type auditReadHold struct {
	armed            atomic.Bool
	entered, release chan struct{}
	once             sync.Once
}

func (h *auditReadHold) wait() {
	if h.armed.Load() {
		h.once.Do(func() { close(h.entered) })
		<-h.release
	}
}
func (h *auditReadHold) unblock() {
	select {
	case <-h.release:
	default:
		close(h.release)
	}
}

type heldAuditLogger struct {
	hold  *auditReadHold
	event audit.Event
}

func (l *heldAuditLogger) Record(audit.Event) error { return nil }
func (l *heldAuditLogger) Query(f audit.QueryFilter) ([]audit.Event, error) {
	l.hold.wait()
	if f.ID != "" && f.ID != l.event.ID {
		return []audit.Event{}, nil
	}
	return []audit.Event{l.event}, nil
}
func (l *heldAuditLogger) Count(audit.QueryFilter) (int, error) { return 1, nil }
func (l *heldAuditLogger) Close() error                         { return nil }
func (l *heldAuditLogger) GetWebhookURLs() []string             { return nil }
func (l *heldAuditLogger) UpdateWebhookURLs([]string) error     { return nil }
func (l *heldAuditLogger) VerifySignature(e audit.Event) bool {
	return e.Signature == l.event.Signature
}

type heldAuditResourceStore struct {
	unifiedresources.ResourceStore
	hold *auditReadHold
}

func (s *heldAuditResourceStore) GetActionAudits(id string, since time.Time, limit int) ([]unifiedresources.ActionAuditRecord, error) {
	s.hold.wait()
	return s.ResourceStore.GetActionAudits(id, since, limit)
}
func (s *heldAuditResourceStore) GetActionLifecycleEvents(id string, since time.Time, limit int) ([]unifiedresources.ActionLifecycleEvent, error) {
	s.hold.wait()
	return s.ResourceStore.GetActionLifecycleEvents(id, since, limit)
}
func (s *heldAuditResourceStore) GetExportAudits(since time.Time, limit int) ([]unifiedresources.ExportAuditRecord, error) {
	s.hold.wait()
	return s.ResourceStore.GetExportAudits(since, limit)
}

type auditReadCase struct{ name, path, marker string }

func auditReadCases() []auditReadCase {
	return []auditReadCase{
		{"list", "/api/audit", "private-audit-fixture"},
		{"list_slash", "/api/audit/", "private-audit-fixture"},
		{"verify", "/api/audit/private-audit-fixture/verify", "verified"},
		{"export_json", "/api/audit/export?format=json&verify=true", "private-audit-fixture"},
		{"export_csv", "/api/audit/export?format=csv&verify=true", "private-audit-fixture"},
		{"summary", "/api/audit/summary?verify=true", "config_change"},
		{"actions", "/api/audit/actions", "private-action-fixture"},
		{"lifecycle", "/api/audit/actions/private-action-fixture/events", "private-action-fixture"},
		{"exports", "/api/audit/exports", "private-export-fixture"},
	}
}

func newHeldAuditReadRouter(t *testing.T) (*Router, *config.Config, *httptest.Server, *auditReadHold) {
	t.Helper()
	cfg := newTestConfigWithTokens(t,
		newTokenRecord(t, logStreamToken, []string{config.ScopeAuditRead}, nil),
		newTokenRecord(t, logStreamControl, []string{config.ScopeWildcard}, nil))
	cfg.AuthUser = "operator"
	router := NewRouter(cfg, nil, nil, nil, nil, "test")
	cleanupTestRouter(t, router)
	setLicenseTierForHandlersForTests(t, router.licenseHandlers, "default", pkglicensing.TierEnterprise)
	hold := &auditReadHold{entered: make(chan struct{}), release: make(chan struct{})}
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	logger := &heldAuditLogger{hold: hold, event: audit.Event{ID: "private-audit-fixture", Timestamp: now, EventType: "config_change", User: "fixture-operator", IP: "192.0.2.10", Path: "/synthetic/private-setting", Success: true, Details: "synthetic private audit evidence", Signature: "synthetic-signature"}}
	priorManager := GetTenantAuditManager()
	SetTenantAuditManager(nil)
	t.Cleanup(func() { SetTenantAuditManager(priorManager) })
	setAuditLogger(t, logger)
	store := unifiedresources.NewMemoryStore()
	if err := store.RecordActionAudit(unifiedresources.ActionAuditRecord{ID: "private-action-fixture", CreatedAt: now, UpdatedAt: now, State: unifiedresources.ActionStateCompleted, Request: unifiedresources.ActionRequest{RequestID: "fixture-request", ResourceID: "vm:100", CapabilityName: "restart", RequestedBy: "fixture-operator"}}); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordActionLifecycleEvent(unifiedresources.ActionLifecycleEvent{ActionID: "private-action-fixture", Timestamp: now, State: unifiedresources.ActionStateCompleted}); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordExportAudit(unifiedresources.ExportAuditRecord{ID: "private-export-fixture", Timestamp: now, Actor: "fixture-operator", EnvelopeHash: "synthetic-hash", Decision: unifiedresources.ExportAllowed, Destination: "synthetic-destination"}); err != nil {
		t.Fatal(err)
	}
	handlers := NewAuditHandlers()
	handlers.SetResourceStoreProvider(func(string) (unifiedresources.ResourceStore, error) {
		return &heldAuditResourceStore{ResourceStore: store, hold: hold}, nil
	})
	// Re-register the production group with its existing provider seam, avoiding
	// a real database. Retain the full Router middleware and real DELETE/logout.
	original := router.mux
	router.mux = http.NewServeMux()
	router.registerOrgLicenseRoutesGroup(nil, nil, handlers)
	router.mux.Handle("/api/security/", original)
	router.mux.Handle("/api/logout", original)
	server := httptest.NewServer(router.Handler())
	t.Cleanup(func() { hold.unblock(); server.Close() })
	return router, cfg, server, hold
}

func auditReadRequest(t *testing.T, ctx context.Context, server *httptest.Server, path, raw, transport string) *http.Request {
	t.Helper()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	switch transport {
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+raw)
	case "session":
		req.AddCookie(&http.Cookie{Name: cookieNameSession, Value: raw})
	default:
		req.Header.Set("X-API-Token", raw)
	}
	return req
}

func auditReadNativeWithdrawal(t *testing.T) {
	for _, tc := range auditReadCases() {
		for _, transport := range []string{"header", "bearer", "session"} {
			t.Run(tc.name+"/"+transport, func(t *testing.T) {
				router, cfg, server, hold := newHeldAuditReadRouter(t)
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				raw := logStreamToken
				if transport == "session" {
					raw = "synthetic-audit-session"
					GetSessionStore().CreateSession(raw, time.Hour, "fixture", "127.0.0.1", cfg.AuthUser)
					t.Cleanup(func() { GetSessionStore().DeleteSession(raw) })
				}
				hold.armed.Store(true)
				type result struct {
					response *http.Response
					err      error
				}
				done := make(chan result, 1)
				go func() {
					r, e := server.Client().Do(auditReadRequest(t, ctx, server, tc.path, raw, transport))
					done <- result{r, e}
				}()
				select {
				case <-hold.entered:
				case r := <-done:
					t.Fatalf("read not admitted: %v %v", r.response, r.err)
				case <-ctx.Done():
					t.Fatal("read did not reach storage hold")
				}
				if transport == "session" {
					logoutReadSession(t, server.URL, raw)
				} else {
					config.Mu.RLock()
					id := cfg.APITokens[0].ID
					config.Mu.RUnlock()
					deletion, _ := http.NewRequestWithContext(ctx, http.MethodDelete, server.URL+"/api/security/tokens/"+id, nil)
					deletion.Header.Set("X-API-Token", logStreamControl)
					response, err := server.Client().Do(deletion)
					if err != nil {
						t.Fatal(err)
					}
					response.Body.Close()
					persisted, err := router.persistence.LoadAPITokens()
					if response.StatusCode != http.StatusNoContent || err != nil || len(persisted) != 1 || persisted[0].ID == id {
						t.Fatal("DELETE did not persist withdrawal")
					}
				}
				fresh, err := server.Client().Do(auditReadRequest(t, ctx, server, tc.path, raw, transport))
				if err != nil {
					t.Fatal(err)
				}
				fresh.Body.Close()
				if fresh.StatusCode != http.StatusUnauthorized {
					t.Fatalf("fresh admission=%d", fresh.StatusCode)
				}
				hold.unblock()
				var completed result
				select {
				case completed = <-done:
				case <-ctx.Done():
					t.Fatal("held read did not finish")
				}
				if completed.err != nil {
					t.Fatal(completed.err)
				}
				body, err := io.ReadAll(completed.response.Body)
				completed.response.Body.Close()
				if err != nil {
					t.Fatal(err)
				}
				if completed.response.StatusCode != http.StatusUnauthorized || string(body) != "Authentication required\n" || completed.response.Header.Get("Content-Disposition") != "" || completed.response.Header.Get("X-Event-Count") != "" {
					t.Errorf("held audit=%d after persisted withdrawal (private evidence=%v), want generic 401 without attachment/count", completed.response.StatusCode, strings.Contains(string(body), tc.marker))
				}
				control, err := server.Client().Do(auditReadRequest(t, ctx, server, tc.path, logStreamControl, "header"))
				if err != nil {
					t.Fatal(err)
				}
				controlBody, err := io.ReadAll(control.Body)
				control.Body.Close()
				if err != nil || control.StatusCode != http.StatusOK || !strings.Contains(string(controlBody), tc.marker) {
					t.Fatalf("independent audit read lost: %d %s %v", control.StatusCode, controlBody, err)
				}
			})
		}
	}
}

func TestAuditReadNativeWithdrawal(t *testing.T) { auditReadNativeWithdrawal(t) }

func TestAuditReadPreservesAdmission(t *testing.T) {
	for _, gate := range []string{"scope", "licence", "session_admin"} {
		t.Run(gate, func(t *testing.T) {
			router, cfg, server, _ := newHeldAuditReadRouter(t)
			raw, transport, want := logStreamToken, "header", http.StatusForbidden
			switch gate {
			case "scope":
				config.Mu.Lock()
				cfg.APITokens[0].Scopes = []string{config.ScopeMonitoringRead}
				config.Mu.Unlock()
			case "licence":
				setLicenseTierForHandlersForTests(t, router.licenseHandlers, "default", pkglicensing.TierFree)
				want = http.StatusPaymentRequired
			case "session_admin":
				raw, transport = "synthetic-nonadmin-audit-session", "session"
				GetSessionStore().CreateSession(raw, time.Hour, "fixture", "127.0.0.1", "different-user")
				t.Cleanup(func() { GetSessionStore().DeleteSession(raw) })
			}
			for _, tc := range auditReadCases() {
				response, err := server.Client().Do(auditReadRequest(t, context.Background(), server, tc.path, raw, transport))
				if err != nil {
					t.Fatal(err)
				}
				body, err := io.ReadAll(response.Body)
				response.Body.Close()
				if err != nil || response.StatusCode != want || strings.Contains(string(body), tc.marker) || response.Header.Get("Content-Disposition") != "" {
					t.Errorf("%s gate=%d %s, want %d", tc.name, response.StatusCode, body, want)
				}
			}
		})
	}
}
