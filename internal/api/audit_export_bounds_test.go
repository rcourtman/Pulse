package api

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/audit"
)

const auditExportTestMaxEvents = 10000

type auditExportBoundsLogger struct {
	*auditSelectionLogger
	ignoreLimit bool
}

func (l *auditExportBoundsLogger) Query(filter audit.QueryFilter) ([]audit.Event, error) {
	if l.ignoreLimit {
		l.queries++
		l.lastQuery = filter
		return l.selected(filter), nil
	}
	return l.auditSelectionLogger.Query(filter)
}

func auditExportBoundEvents(count int) []audit.Event {
	events := make([]audit.Event, count)
	for i := range events {
		events[i] = audit.Event{ID: fmt.Sprintf("synthetic-event-%d", i), EventType: "login", User: "synthetic-operator", Timestamp: time.Date(2026, 1, 1, 12, 0, i, 0, time.UTC), Success: true, Signature: "synthetic"}
	}
	return events
}

func auditExportBoundEndpoints() []struct {
	name, path string
	handle     http.HandlerFunc
} {
	h := NewAuditHandlers()
	return []struct {
		name, path string
		handle     http.HandlerFunc
	}{
		{"json", "/api/audit/export?format=json&verify=true", h.HandleExportAuditEvents},
		{"csv", "/api/audit/export?format=csv&verify=true", h.HandleExportAuditEvents},
		{"summary", "/api/audit/summary?verify=true", h.HandleAuditSummary},
	}
}

func TestAuditExportRejectsOversizedSelection(t *testing.T) {
	for _, endpoint := range auditExportBoundEndpoints() {
		for _, ignoreLimit := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/ignore-limit-%t", endpoint.name, ignoreLimit), func(t *testing.T) {
				logger := &auditExportBoundsLogger{auditSelectionLogger: newAuditSelectionLogger(), ignoreLimit: ignoreLimit}
				logger.events = auditExportBoundEvents(auditExportTestMaxEvents + 2)
				setAuditLogger(t, logger)
				rec := httptest.NewRecorder()
				endpoint.handle(rec, httptest.NewRequest(http.MethodGet, endpoint.path+"&limit=1&offset=999", nil))
				if rec.Code != http.StatusBadRequest {
					t.Fatalf("overflow status = %d, want 400", rec.Code)
				}
				var body APIError
				if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				if body.Code != "export_too_large" || !strings.Contains(body.Message, "10000") || !strings.Contains(body.Message, "Narrow") {
					t.Fatalf("missing bounded recovery error: %+v", body)
				}
				if strings.Contains(rec.Body.String(), "synthetic-event") || rec.Header().Get("Content-Disposition") != "" || rec.Header().Get("X-Event-Count") != "" {
					t.Fatal("overflow returned an event or download metadata")
				}
				if logger.queries != 1 || logger.counts != 0 || logger.verifications != 0 || logger.lastQuery.Limit != auditExportTestMaxEvents+1 || logger.lastQuery.Offset != 0 {
					t.Fatalf("query/count/verify/bounds = %d/%d/%d/%d/%d", logger.queries, logger.counts, logger.verifications, logger.lastQuery.Limit, logger.lastQuery.Offset)
				}
			})
		}
	}
}

func TestAuditExportBoundedCompleteResults(t *testing.T) {
	for _, endpoint := range auditExportBoundEndpoints() {
		for _, count := range []int{0, 2, auditExportTestMaxEvents} {
			t.Run(fmt.Sprintf("%s/%d", endpoint.name, count), func(t *testing.T) {
				logger := newAuditSelectionLogger()
				logger.events = auditExportBoundEvents(count)
				setAuditLogger(t, logger)
				rec := httptest.NewRecorder()
				endpoint.handle(rec, httptest.NewRequest(http.MethodGet, endpoint.path+"&limit=1&offset=999", nil))
				if rec.Code != http.StatusOK {
					t.Fatalf("status = %d, want 200", rec.Code)
				}
				if endpoint.name == "summary" {
					var summary audit.ExportSummary
					if err := json.Unmarshal(rec.Body.Bytes(), &summary); err != nil {
						t.Fatal(err)
					}
					if summary.TotalEvents != count || summary.SuccessCount != count || summary.EventsByType["login"] != count || summary.EventsByUser["synthetic-operator"] != count {
						t.Fatalf("incomplete summary: %+v", summary)
					}
				} else {
					if rec.Header().Get("X-Event-Count") != strconv.Itoa(count) || !strings.HasPrefix(rec.Header().Get("Content-Disposition"), "attachment; filename=audit-log-") {
						t.Fatal("incorrect attachment metadata")
					}
					if endpoint.name == "json" {
						var body struct {
							EventCount int                 `json:"event_count"`
							Events     []audit.ExportEvent `json:"events"`
						}
						if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
							t.Fatal(err)
						}
						if body.EventCount != count || len(body.Events) != count || body.Events == nil {
							t.Fatal("incomplete JSON export")
						}
						for i, event := range body.Events {
							if event.ID != logger.events[i].ID || event.SignatureValid == nil || !*event.SignatureValid {
								t.Fatalf("incorrect JSON row %d or signature verdict", i)
							}
						}
					} else {
						rows, err := csv.NewReader(rec.Body).ReadAll()
						if err != nil {
							t.Fatal(err)
						}
						if len(rows) != count+1 || len(rows[0]) != 10 {
							t.Fatal("incomplete CSV export")
						}
						for i, row := range rows[1:] {
							if row[0] != logger.events[i].ID || row[9] != "true" {
								t.Fatalf("incorrect CSV row %d or signature verdict", i)
							}
						}
					}
				}
				if logger.queries != 1 || logger.counts != 0 || logger.verifications != count || logger.lastQuery.Limit != auditExportTestMaxEvents+1 || logger.lastQuery.Offset != 0 {
					t.Fatalf("query/count/verify/bounds = %d/%d/%d/%d/%d", logger.queries, logger.counts, logger.verifications, logger.lastQuery.Limit, logger.lastQuery.Offset)
				}
			})
		}
	}
}

func TestAuditExportOverflowCanNarrowSelection(t *testing.T) {
	for _, endpoint := range auditExportBoundEndpoints() {
		t.Run(endpoint.name, func(t *testing.T) {
			logger := newAuditSelectionLogger()
			logger.events = auditExportBoundEvents(auditExportTestMaxEvents + 1)
			for i := 2; i < len(logger.events); i++ {
				logger.events[i].EventType = "config_change"
			}
			setAuditLogger(t, logger)
			wide := httptest.NewRecorder()
			endpoint.handle(wide, httptest.NewRequest(http.MethodGet, endpoint.path, nil))
			if wide.Code != http.StatusBadRequest {
				t.Fatalf("wide status = %d, want 400", wide.Code)
			}
			narrow := httptest.NewRecorder()
			endpoint.handle(narrow, httptest.NewRequest(http.MethodGet, endpoint.path+"&event=login&user=synthetic-operator&startTime=2026-01-01T00:00:00Z&endTime=2026-01-02T00:00:00Z", nil))
			if narrow.Code != http.StatusOK {
				t.Fatalf("narrow status = %d, want 200", narrow.Code)
			}
			if logger.lastQuery.EventType != "login" || logger.lastQuery.User != "synthetic-operator" || logger.lastQuery.StartTime == nil || logger.lastQuery.EndTime == nil || logger.counts != 0 || logger.verifications != 2 {
				t.Fatal("narrowed selection lost filters or verified an unselected row")
			}
			if endpoint.name == "summary" {
				var summary audit.ExportSummary
				if err := json.Unmarshal(narrow.Body.Bytes(), &summary); err != nil {
					t.Fatal(err)
				}
				if summary.TotalEvents != 2 {
					t.Fatalf("narrow count = %d, want 2", summary.TotalEvents)
				}
			} else if narrow.Header().Get("X-Event-Count") != "2" {
				t.Fatal("narrowed export did not contain exactly two events")
			}
		})
	}
}
