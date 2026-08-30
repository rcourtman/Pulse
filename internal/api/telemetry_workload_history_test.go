package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/telemetry"
)

func TestHandleWorkloadHistoryActivityRecordsOnlyCount(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	router := &Router{persistence: persistence}
	req := httptest.NewRequest(http.MethodPost, "/api/usage/workload-history", strings.NewReader(`{"activity":"preview"}`))
	rec := httptest.NewRecorder()

	router.HandleWorkloadHistoryActivity(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d body=%q, want 204", rec.Code, rec.Body.String())
	}
	tally, err := persistence.LoadWorkloadHistoryActivityTally()
	if err != nil {
		t.Fatalf("LoadWorkloadHistoryActivityTally = %v", err)
	}
	if got := tally.PreviewsSince(time.Now().UTC().AddDate(0, 0, -30)); got != 1 {
		t.Fatalf("previews = %d, want 1", got)
	}
}

func TestHandleWorkloadHistoryActivityRejectsContentAndUnknownFields(t *testing.T) {
	for _, body := range []string{
		`{"activity":"guest-101"}`,
		`{"activity":"preview","guest_id":"101"}`,
		`{"activity":"preview"}{"activity":"scrub"}`,
	} {
		router := &Router{persistence: config.NewConfigPersistence(t.TempDir())}
		req := httptest.NewRequest(http.MethodPost, "/api/usage/workload-history", strings.NewReader(body))
		rec := httptest.NewRecorder()
		router.HandleWorkloadHistoryActivity(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body %q status = %d, want 400", body, rec.Code)
		}
	}
}

func TestApplyWorkloadHistoryTelemetrySnapshotAggregatesRollingCounts(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	now := time.Now().UTC()
	for _, activity := range []string{
		config.WorkloadHistoryActivityPreview,
		config.WorkloadHistoryActivityPreview,
		config.WorkloadHistoryActivityScrub,
		config.WorkloadHistoryActivityRangeChange,
		config.WorkloadHistoryActivityDetailsSelected,
	} {
		if err := persistence.RecordWorkloadHistoryActivity(activity, now); err != nil {
			t.Fatalf("record %q = %v", activity, err)
		}
	}

	snapshot := telemetry.Snapshot{}
	(&Router{persistence: persistence}).ApplyWorkloadHistoryTelemetrySnapshot(&snapshot, now)
	if snapshot.WorkloadHistoryPreviewSessions30d != 2 ||
		snapshot.WorkloadHistoryScrubSessions30d != 1 ||
		snapshot.WorkloadHistoryRangeChangeSessions30d != 1 ||
		snapshot.WorkloadHistoryDetailsSelectionSessions30d != 1 {
		t.Fatalf("unexpected workload history snapshot: %+v", snapshot)
	}
}
