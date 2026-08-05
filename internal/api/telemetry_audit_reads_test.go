package api

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func newAuditReadActivityRouter(t *testing.T) (*Router, *config.ConfigPersistence) {
	t.Helper()
	persistence := config.NewConfigPersistence(t.TempDir())
	return &Router{persistence: persistence}, persistence
}

func countRecordedAuditReads(t *testing.T, persistence *config.ConfigPersistence) int {
	t.Helper()
	history, err := persistence.LoadAuditReadActivityHistory()
	if err != nil {
		t.Fatalf("LoadAuditReadActivityHistory: %v", err)
	}
	if history == nil {
		return 0
	}
	return len(history.Events)
}

func TestWithAuditReadActivity_RecordsAndStillServes(t *testing.T) {
	router, persistence := newAuditReadActivityRouter(t)

	served := false
	handler := router.withAuditReadActivity(config.AuditReadActivityExport, func(w http.ResponseWriter, req *http.Request) {
		served = true
		w.WriteHeader(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodGet, "/api/audit/export", nil))

	if !served {
		t.Fatal("wrapped handler was not invoked")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := countRecordedAuditReads(t, persistence); got != 1 {
		t.Fatalf("recorded audit reads = %d, want 1", got)
	}
}

// The recorded marker must carry the activity class and nothing that could
// describe what was read.
func TestWithAuditReadActivity_RecordIsContentFree(t *testing.T) {
	router, persistence := newAuditReadActivityRouter(t)

	handler := router.withAuditReadActivity(config.AuditReadActivityList, func(w http.ResponseWriter, req *http.Request) {})
	req := httptest.NewRequest(http.MethodGet, "/api/audit?user=alice&event=login&start=2026-01-01", nil)
	handler(httptest.NewRecorder(), req)

	history, err := persistence.LoadAuditReadActivityHistory()
	if err != nil {
		t.Fatalf("LoadAuditReadActivityHistory: %v", err)
	}
	if len(history.Events) != 1 {
		t.Fatalf("events = %d, want 1", len(history.Events))
	}
	record := history.Events[0]
	if record.Activity != config.AuditReadActivityList {
		t.Fatalf("activity = %q, want %q", record.Activity, config.AuditReadActivityList)
	}
	if record.Timestamp.IsZero() {
		t.Fatal("record must carry a timestamp")
	}
	// The record type has exactly two fields; a future field carrying query
	// filters or actors would have to be added deliberately and would fail the
	// privacy disclosure guard.
	if got := recordFieldCount(record); got != 2 {
		t.Fatalf("AuditReadActivityRecord has %d fields, want 2 (timestamp, activity)", got)
	}
}

// Unknown activity classes are dropped rather than stored, so a future caller
// cannot widen what this history means by passing a new string.
func TestRecordAuditReadActivity_RejectsUnknownActivity(t *testing.T) {
	router, persistence := newAuditReadActivityRouter(t)

	handler := router.withAuditReadActivity("peeked-at-everything", func(w http.ResponseWriter, req *http.Request) {})
	handler(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/audit", nil))

	if got := countRecordedAuditReads(t, persistence); got != 0 {
		t.Fatalf("recorded audit reads = %d for an unknown activity class, want 0", got)
	}
}

// A router with no persistence must not panic: the audit routes are reachable
// before every dependency is guaranteed to be wired.
func TestWithAuditReadActivity_NilPersistenceIsSafe(t *testing.T) {
	served := false
	handler := (&Router{}).withAuditReadActivity(config.AuditReadActivityList, func(w http.ResponseWriter, req *http.Request) {
		served = true
	})
	handler(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/audit", nil))
	if !served {
		t.Fatal("handler must still run when activity cannot be recorded")
	}
}

func recordFieldCount(record config.AuditReadActivityRecord) int {
	return reflect.TypeOf(record).NumField()
}
