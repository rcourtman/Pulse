package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/audit"
)

// Exercise the canonical handlers and exporter without a database or receiver.
func TestAuditSelectionRejectsDamagedQuery(t *testing.T) {
	queries := []string{
		"user=%zz", "event=%", "success=%zz", "startTime=%zz", "endTime=%zz",
		"limit=%zz", "offset=%zz", "format=%zz", "verify=%zz", "%zz=operator",
		"user=operator;event=login", "user=operator&user=%zz", "event=login&unknown=%zz",
	}
	for _, endpoint := range auditScopeEndpoints() {
		for _, query := range queries {
			t.Run(endpoint.name+"/"+query, func(t *testing.T) {
				assertAuditSelectionRejected(t, endpoint.handle, query, "invalid_query")
			})
		}
	}
}

func TestAuditSelectionRejectsRepeatedParameters(t *testing.T) {
	values := map[string]string{
		"event": "login", "user": "operator", "success": "true", "startTime": "2026-01-01T00:00:00Z",
		"endTime": "2026-01-02T00:00:00Z", "limit": "10", "offset": "0", "format": "json", "verify": "true",
	}
	for _, endpoint := range auditScopeEndpoints() {
		for _, key := range []string{"event", "user", "success", "startTime", "endTime", "limit", "offset", "format", "verify"} {
			for _, variant := range []string{"identical", "conflicting", "encoded_key"} {
				t.Run(endpoint.name+"/"+key+"/"+variant, func(t *testing.T) {
					first, second := key, url.QueryEscape(values[key])
					if variant == "conflicting" {
						second = "different"
					}
					if variant == "encoded_key" {
						first = fmt.Sprintf("%%%02x%s", key[0], key[1:])
					}
					query := first + "=" + url.QueryEscape(values[key]) + "&" + key + "=" + second
					assertAuditSelectionRejected(t, endpoint.handle, query, "ambiguous_query")
				})
			}
		}
	}
}

func assertAuditSelectionRejected(t *testing.T, handle http.HandlerFunc, query, code string) {
	t.Helper()
	logger := &auditScopeLogger{testAuditLogger: testAuditLogger{events: []audit.Event{{ID: "unselected-event", Signature: "synthetic"}}}}
	setAuditLogger(t, logger)
	rec := httptest.NewRecorder()
	handle(rec, httptest.NewRequest(http.MethodGet, "/api/audit?"+query, nil))
	var body APIError
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusBadRequest || body.Code != code {
		t.Errorf("status/code = %d/%q, want 400/%q", rec.Code, body.Code, code)
	}
	if logger.queries != 0 || logger.counts != 0 || logger.verifications != 0 {
		t.Errorf("query/count/verify = %d/%d/%d, want no event read", logger.queries, logger.counts, logger.verifications)
	}
	if strings.Contains(rec.Body.String(), "unselected-event") || rec.Header().Get("Content-Disposition") != "" || rec.Header().Get("X-Event-Count") != "" {
		t.Error("rejected selection returned event/export content")
	}
}

// A filter-backed, in-memory logger makes widening visible in the wire result.
type auditSelectionLogger struct {
	auditScopeLogger
	urls    []string
	updates int
}

func (l *auditSelectionLogger) selected(f audit.QueryFilter) []audit.Event {
	events := []audit.Event{}
	for _, e := range l.events {
		if f.EventType != "" && e.EventType != f.EventType || f.User != "" && e.User != f.User || f.Success != nil && e.Success != *f.Success || f.StartTime != nil && e.Timestamp.Before(*f.StartTime) || f.EndTime != nil && !e.Timestamp.Before(*f.EndTime) {
			continue
		}
		events = append(events, e)
	}
	return events
}
func (l *auditSelectionLogger) Query(f audit.QueryFilter) ([]audit.Event, error) {
	l.queries++
	l.lastQuery = f
	events := l.selected(f)
	if f.Offset >= len(events) {
		return []audit.Event{}, nil
	}
	events = events[f.Offset:]
	if f.Limit > 0 && len(events) > f.Limit {
		events = events[:f.Limit]
	}
	return events, nil
}
func (l *auditSelectionLogger) Count(f audit.QueryFilter) (int, error) {
	l.counts++
	l.lastCount = f
	return len(l.selected(f)), nil
}
func (l *auditSelectionLogger) GetWebhookURLs() []string { return append([]string{}, l.urls...) }
func (l *auditSelectionLogger) UpdateWebhookURLs(urls []string) error {
	l.updates++
	l.urls = append([]string{}, urls...)
	return l.updateErr
}

func newAuditSelectionLogger() *auditSelectionLogger {
	at := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	return &auditSelectionLogger{auditScopeLogger: auditScopeLogger{testAuditLogger: testAuditLogger{verifyResult: true, events: []audit.Event{
		{ID: "selected-event", EventType: "login", User: "operator+é@example.invalid", Timestamp: at, Success: true, Signature: "synthetic"},
		{ID: "other-user", EventType: "login", User: "another", Timestamp: at, Success: true},
		{ID: "other-event", EventType: "logout", User: "operator+é@example.invalid", Timestamp: at, Success: true},
		{ID: "outside-window", EventType: "login", User: "operator+é@example.invalid", Timestamp: at.Add(-48 * time.Hour), Success: true},
	}}}}
}

func TestAuditSelectionPreservesEncodedWireResults(t *testing.T) {
	for _, endpoint := range auditScopeEndpoints() {
		for _, format := range []string{"json", "csv"} {
			t.Run(endpoint.name+"/"+format, func(t *testing.T) {
				logger := newAuditSelectionLogger()
				setAuditLogger(t, logger)
				q := url.Values{"user": {"operator+é@example.invalid"}, "event": {"login"}, "startTime": {"2026-01-01T01:00:00+01:00"}, "endTime": {"2026-01-02T00:00:00Z"}, "verify": {"true"}, "format": {format}, "unknown": {"one", "two"}}
				rec := httptest.NewRecorder()
				endpoint.handle(rec, httptest.NewRequest(http.MethodGet, "/api/audit?"+q.Encode(), nil))
				if rec.Code != 200 || logger.queries != 1 || logger.lastQuery.User != q.Get("user") || logger.lastQuery.EventType != "login" {
					t.Fatalf("status/query/filter = %d/%d/%+v", rec.Code, logger.queries, logger.lastQuery)
				}
				for _, id := range []string{"other-user", "other-event", "outside-window"} {
					if strings.Contains(rec.Body.String(), id) {
						t.Fatalf("widened selection contains %s", id)
					}
				}
				if endpoint.name == "summary" {
					var summary audit.ExportSummary
					if err := json.Unmarshal(rec.Body.Bytes(), &summary); err != nil {
						t.Fatal(err)
					}
					if summary.TotalEvents != 1 {
						t.Fatalf("summary count = %d", summary.TotalEvents)
					}
				} else if !strings.Contains(rec.Body.String(), "selected-event") {
					t.Fatal("selected event missing")
				}
				if endpoint.name == "export" && rec.Header().Get("X-Event-Count") != "1" {
					t.Fatal("export count not scoped")
				}
			})
		}
	}
}

func TestAuditWebhookSelectionRequiresExplicitArray(t *testing.T) {
	for _, method := range []string{http.MethodPost, http.MethodPut} {
		for _, body := range []string{"null", "{}", `{"urls":null}`} {
			t.Run(method+"/"+body, func(t *testing.T) {
				logger := newAuditSelectionLogger()
				logger.urls = []string{"https://8.8.8.8/audit"}
				setAuditLogger(t, logger)
				rec := httptest.NewRecorder()
				NewAuditHandlers().HandleUpdateWebhooks(rec, httptest.NewRequest(method, "/api/admin/webhooks/audit", strings.NewReader(body)))
				var payload APIError
				if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
					t.Fatal(err)
				}
				if rec.Code != 400 || payload.Code != "invalid_request" || logger.updates != 0 || !reflect.DeepEqual(logger.urls, []string{"https://8.8.8.8/audit"}) {
					t.Fatalf("status/code/updates/urls = %d/%q/%d/%v", rec.Code, payload.Code, logger.updates, logger.urls)
				}
			})
		}
	}
}

func TestAuditWebhookSelectionExplicitClearRestoreAndValidation(t *testing.T) {
	for _, method := range []string{http.MethodPost, http.MethodPut} {
		t.Run(method, func(t *testing.T) {
			logger := newAuditSelectionLogger()
			logger.urls = []string{"https://8.8.8.8/audit"}
			setAuditLogger(t, logger)
			for _, tc := range []struct {
				body            string
				status, updates int
				urls            []string
			}{
				{`{"urls":["http://127.0.0.1/private"]}`, 400, 0, []string{"https://8.8.8.8/audit"}},
				{`{"urls":[]}`, 204, 1, []string{}},
				{`{"urls":["https://8.8.8.8/audit"]}`, 204, 2, []string{"https://8.8.8.8/audit"}},
				{`{"urls":["https://8.8.8.8/changed","http://127.0.0.1/private"]}`, 400, 2, []string{"https://8.8.8.8/audit"}},
			} {
				rec := httptest.NewRecorder()
				NewAuditHandlers().HandleUpdateWebhooks(rec, httptest.NewRequest(method, "/api/admin/webhooks/audit", strings.NewReader(tc.body)))
				if rec.Code != tc.status || logger.updates != tc.updates || !reflect.DeepEqual(logger.urls, tc.urls) {
					t.Fatalf("status/updates/urls = %d/%d/%v", rec.Code, logger.updates, logger.urls)
				}
			}
			rec := httptest.NewRecorder()
			NewAuditHandlers().HandleUpdateWebhooks(rec, httptest.NewRequest(http.MethodGet, "/api/admin/webhooks/audit", strings.NewReader(`{"urls":[]}`)))
			if rec.Code != 405 || logger.updates != 2 {
				t.Fatal("safe method mutated selection")
			}
		})
	}
}

// Tenant selection still comes only from the authenticated request context.
type auditSelectionFactory map[string]*auditSelectionLogger

func (f auditSelectionFactory) CreateLogger(path string) (audit.Logger, error) {
	for org, l := range f {
		if strings.HasSuffix(path, "/"+org+"/audit.db") {
			return l, nil
		}
	}
	return nil, context.Canceled
}

func TestAuditSelectionTenantIsolation(t *testing.T) {
	previous := GetTenantAuditManager()
	factory := auditSelectionFactory{"org-a": newAuditSelectionLogger(), "org-b": newAuditSelectionLogger()}
	for org, l := range factory {
		l.urls = []string{"https://8.8.8.8/" + org}
		l.events[0].ID = org
	}
	manager := audit.NewTenantLoggerManager(t.TempDir(), factory)
	SetTenantAuditManager(manager)
	t.Cleanup(func() { manager.Close(); SetTenantAuditManager(previous) })
	for _, org := range []string{"org-a", "org-b"} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/audit?user=operator%2B%C3%A9%40example.invalid&event=login&startTime=2026-01-01T00:00:00Z", nil)
		req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, org))
		NewAuditHandlers().HandleListAuditEvents(rec, req)
		other := "org-a"
		if org == other {
			other = "org-b"
		}
		if rec.Code != 200 || !strings.Contains(rec.Body.String(), org) || strings.Contains(rec.Body.String(), other) {
			t.Fatalf("tenant list: %d/%s", rec.Code, rec.Body.String())
		}
		req = httptest.NewRequest(http.MethodPut, "/api/admin/webhooks/audit", strings.NewReader(`{"urls":null}`))
		req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, org))
		rec = httptest.NewRecorder()
		NewAuditHandlers().HandleUpdateWebhooks(rec, req)
		if rec.Code != 400 || factory[org].updates != 0 || factory[other].updates != 0 {
			t.Fatal("absent selection mutated tenant receivers")
		}
		req = httptest.NewRequest(http.MethodPut, "/api/admin/webhooks/audit", strings.NewReader(`{"urls":[]}`))
		req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, org))
		rec = httptest.NewRecorder()
		before := factory[other].updates
		NewAuditHandlers().HandleUpdateWebhooks(rec, req)
		if rec.Code != 204 || len(factory[org].urls) != 0 || factory[other].updates != before {
			t.Fatal("explicit clear crossed tenant boundary")
		}
	}
}
