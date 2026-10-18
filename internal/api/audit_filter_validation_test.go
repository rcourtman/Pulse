package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/audit"
)

// Actual handlers/exporter, but no database, IdP, provider or production events.
type auditScopeLogger struct {
	testAuditLogger
	queries, counts, verifications int
}

func (l *auditScopeLogger) Query(f audit.QueryFilter) ([]audit.Event, error) {
	l.queries++
	return l.testAuditLogger.Query(f)
}
func (l *auditScopeLogger) Count(f audit.QueryFilter) (int, error) {
	l.counts++
	return l.testAuditLogger.Count(f)
}
func (l *auditScopeLogger) VerifySignature(e audit.Event) bool {
	l.verifications++
	return l.testAuditLogger.VerifySignature(e)
}

func auditScopeEndpoints() []struct {
	name   string
	handle http.HandlerFunc
} {
	h := NewAuditHandlers()
	return []struct {
		name   string
		handle http.HandlerFunc
	}{
		{"list", h.HandleListAuditEvents},
		{"export", h.HandleExportAuditEvents},
		{"summary", h.HandleAuditSummary},
	}
}

func TestAuditScopeRejectsMalformedTimes(t *testing.T) {
	cases := []struct{ name, query, code string }{
		{"invalid_start", "startTime=not-a-time", "invalid_start_time"},
		{"empty_start", "startTime=", "invalid_start_time"},
		{"invalid_end", "endTime=not-a-time", "invalid_end_time"},
		{"empty_end", "endTime=", "invalid_end_time"},
		{"missing_timezone", "startTime=2026-01-01T00:00:00", "invalid_start_time"},
		{"invalid_calendar", "endTime=2026-02-30T00:00:00Z", "invalid_end_time"},
		{"equal_bounds", "startTime=2026-01-01T00:00:00Z&endTime=2026-01-01T00:00:00Z", "invalid_time_range"},
		{"reversed_bounds", "startTime=2026-01-02T00:00:00Z&endTime=2026-01-01T00:00:00Z", "invalid_time_range"},
		{"equal_instants", "startTime=2026-01-01T01:00:00%2B01:00&endTime=2026-01-01T00:00:00Z", "invalid_time_range"},
	}
	for _, endpoint := range auditScopeEndpoints() {
		for _, tc := range cases {
			t.Run(endpoint.name+"/"+tc.name, func(t *testing.T) {
				logger := &auditScopeLogger{testAuditLogger: testAuditLogger{events: []audit.Event{{ID: "private-synthetic-event", Signature: "signature"}}}}
				setAuditLogger(t, logger)
				rec := httptest.NewRecorder()
				endpoint.handle(rec, httptest.NewRequest(http.MethodGet, "/api/audit?"+tc.query+"&verify=true", nil))
				var payload APIError
				if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
					t.Fatal(err)
				}
				if rec.Code != http.StatusBadRequest || payload.Code != tc.code {
					t.Errorf("status/code = %d/%q, want 400/%q", rec.Code, payload.Code, tc.code)
				}
				if logger.queries != 0 || logger.counts != 0 || logger.verifications != 0 {
					t.Errorf("rejected scope read events: query/count/verify = %d/%d/%d", logger.queries, logger.counts, logger.verifications)
				}
				if strings.Contains(rec.Body.String(), "private-synthetic-event") || rec.Header().Get("Content-Disposition") != "" || rec.Header().Get("X-Event-Count") != "" {
					t.Error("invalid scope produced export/event content")
				}
			})
		}
	}
}

func TestAuditScopeRejectsMalformedSuccess(t *testing.T) {
	for _, endpoint := range auditScopeEndpoints()[:2] { // Summary has no success filter.
		for _, value := range []string{"", "maybe", "TRUE", "0"} {
			name := value
			if name == "" {
				name = "empty"
			}
			t.Run(endpoint.name+"/"+name, func(t *testing.T) {
				logger := &auditScopeLogger{}
				setAuditLogger(t, logger)
				rec := httptest.NewRecorder()
				endpoint.handle(rec, httptest.NewRequest(http.MethodGet, "/api/audit?success="+value, nil))
				var payload APIError
				if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
					t.Fatal(err)
				}
				if rec.Code != http.StatusBadRequest || payload.Code != "invalid_success" || logger.queries != 0 || logger.counts != 0 {
					t.Fatalf("status/code/query/count = %d/%q/%d/%d", rec.Code, payload.Code, logger.queries, logger.counts)
				}
			})
		}
	}
}

func TestAuditScopePreservesFilters(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 125000000, time.UTC)
	end := start.Add(time.Hour)
	for _, endpoint := range auditScopeEndpoints() {
		for _, bounds := range []string{"none", "start_only", "end_only", "both"} {
			t.Run(endpoint.name+"/"+bounds, func(t *testing.T) {
				logger := &auditScopeLogger{testAuditLogger: testAuditLogger{events: []audit.Event{{ID: "event", Signature: "signed", Timestamp: start}}, verifyResult: true}}
				setAuditLogger(t, logger)
				q := url.Values{}
				if bounds != "none" {
					q.Set("event", "login")
					q.Set("user", "operator+test@example.invalid")
					if bounds != "end_only" {
						q.Set("startTime", start.In(time.FixedZone("test", 3600)).Format(time.RFC3339Nano))
					}
					if bounds != "start_only" {
						q.Set("endTime", end.Format(time.RFC3339Nano))
					}
				}
				q.Set("verify", "true")
				q.Set("limit", "1")
				q.Set("offset", "7")
				rec := httptest.NewRecorder()
				endpoint.handle(rec, httptest.NewRequest(http.MethodGet, "/api/audit?"+q.Encode(), nil))
				if rec.Code != http.StatusOK || logger.queries != 1 {
					t.Fatalf("status/query = %d/%d", rec.Code, logger.queries)
				}
				f := logger.lastQuery
				if f.EventType != q.Get("event") || f.User != q.Get("user") || f.Success != nil {
					t.Fatalf("scope = %+v", f)
				}
				if bounds == "none" || bounds == "end_only" {
					if f.StartTime != nil {
						t.Fatal("invented start bound")
					}
				} else if f.StartTime == nil || !f.StartTime.Equal(start) {
					t.Fatalf("start = %v", f.StartTime)
				}
				if bounds == "none" || bounds == "start_only" {
					if f.EndTime != nil {
						t.Fatal("invented end bound")
					}
				} else if f.EndTime == nil || !f.EndTime.Equal(end) {
					t.Fatalf("end = %v", f.EndTime)
				}
				if endpoint.name == "list" {
					if f.Limit != 1 || f.Offset != 7 || logger.counts != 1 {
						t.Fatalf("list paging = %+v, counts=%d", f, logger.counts)
					}
				} else if f.Limit != 0 || f.Offset != 0 || logger.verifications != 1 {
					t.Fatalf("export/summary = %+v, verifies=%d", f, logger.verifications)
				}
			})
		}
	}
}

func TestAuditScopePreservesSuccessAndFormats(t *testing.T) {
	for _, endpoint := range auditScopeEndpoints()[:2] {
		for _, value := range []string{"true", "false"} {
			for _, format := range []string{"json", "csv"} {
				t.Run(endpoint.name+"/"+value+"/"+format, func(t *testing.T) {
					logger := &auditScopeLogger{}
					setAuditLogger(t, logger)
					rec := httptest.NewRecorder()
					endpoint.handle(rec, httptest.NewRequest(http.MethodGet, "/api/audit?success="+value+"&format="+format, nil))
					if rec.Code != 200 || logger.lastQuery.Success == nil || *logger.lastQuery.Success != (value == "true") {
						t.Fatalf("status/filter = %d/%+v", rec.Code, logger.lastQuery)
					}
					if endpoint.name == "export" {
						want := "application/json; charset=utf-8"
						if format == "csv" {
							want = "text/csv; charset=utf-8"
						}
						if rec.Header().Get("Content-Type") != want || rec.Header().Get("X-Event-Count") != "0" {
							t.Fatalf("export headers = %v", rec.Header())
						}
					}
				})
			}
		}
	}
}

// The tenant manager uses its ordinary factory boundary, never a SQLite store.
type auditScopeFactory map[string]*auditScopeLogger

func (f auditScopeFactory) CreateLogger(path string) (audit.Logger, error) {
	org := filepath.Base(filepath.Dir(path))
	if logger := f[org]; logger != nil {
		return logger, nil
	}
	return nil, fmt.Errorf("unexpected synthetic org")
}

func TestAuditScopeTenantIsolation(t *testing.T) {
	for _, endpoint := range auditScopeEndpoints()[1:] {
		t.Run(endpoint.name, func(t *testing.T) {
			previous := GetTenantAuditManager()
			factory := auditScopeFactory{}
			for _, org := range []string{"org-a", "org-b"} {
				factory[org] = &auditScopeLogger{testAuditLogger: testAuditLogger{events: []audit.Event{{ID: org, User: org, Timestamp: time.Now()}}}}
			}
			manager := audit.NewTenantLoggerManager(t.TempDir(), factory)
			SetTenantAuditManager(manager)
			t.Cleanup(func() { manager.Close(); SetTenantAuditManager(previous) })
			for _, org := range []string{"org-a", "org-b"} {
				req := httptest.NewRequest(http.MethodGet, "/api/audit?event=login&startTime=2026-01-01T00:00:00Z", nil)
				req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, org))
				rec := httptest.NewRecorder()
				endpoint.handle(rec, req)
				if rec.Code != 200 || factory[org].queries != 1 || factory[org].lastQuery.StartTime == nil || !strings.Contains(rec.Body.String(), org) {
					t.Fatalf("tenant %s status/query/body = %d/%d/%s", org, rec.Code, factory[org].queries, rec.Body.String())
				}
				other := "org-a"
				if org == other {
					other = "org-b"
				}
				if strings.Contains(rec.Body.String(), other) {
					t.Fatal("read another tenant's events")
				}
			}
		})
	}
}

func TestAuditScopeMethodAndAvailabilityGates(t *testing.T) {
	for _, endpoint := range auditScopeEndpoints() {
		t.Run(endpoint.name, func(t *testing.T) {
			logger := &auditScopeLogger{}
			setAuditLogger(t, logger)
			rec := httptest.NewRecorder()
			endpoint.handle(rec, httptest.NewRequest(http.MethodPost, "/api/audit?startTime=bad", nil))
			if rec.Code != http.StatusMethodNotAllowed || logger.queries != 0 {
				t.Fatalf("method gate = %d/%d", rec.Code, logger.queries)
			}
			setAuditLogger(t, audit.NewConsoleLogger())
			rec = httptest.NewRecorder()
			endpoint.handle(rec, httptest.NewRequest(http.MethodGet, "/api/audit?startTime=bad", nil))
			want := http.StatusNotImplemented
			if endpoint.name == "list" {
				want = http.StatusServiceUnavailable
			}
			if rec.Code != want {
				t.Fatalf("availability gate = %d, want %d", rec.Code, want)
			}
		})
	}
}
