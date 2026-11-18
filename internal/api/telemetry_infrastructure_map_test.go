package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/telemetry"
)

func TestHandleInfrastructureMapActivityRecordsOnlyCount(t *testing.T) {
	dir := t.TempDir()
	persistence := config.NewConfigPersistence(dir)
	router := &Router{persistence: persistence}
	req := httptest.NewRequest(http.MethodPost, "/api/usage/infrastructure-map", strings.NewReader(`{"activity":"export_names_hidden"}`))
	rec := httptest.NewRecorder()

	router.HandleInfrastructureMapActivity(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d body=%q, want 204", rec.Code, rec.Body.String())
	}
	tally, err := persistence.LoadInfrastructureMapActivityTally()
	if err != nil {
		t.Fatalf("LoadInfrastructureMapActivityTally = %v", err)
	}
	if got := tally.CountSince(config.InfrastructureMapActivityExportNamesHidden, time.Now().UTC().AddDate(0, 0, -30)); got != 1 {
		t.Fatalf("names-hidden exports = %d, want 1", got)
	}
}

// The map draws names, addresses and networks. None of them may reach the
// tally, whether sent as the activity value or beside it.
func TestHandleInfrastructureMapActivityRejectsContentAndUnknownFields(t *testing.T) {
	for _, body := range []string{
		`{"activity":"pve-node-1"}`,
		`{"activity":"192.168.10.0/24"}`,
		`{"activity":"export","view":"networks"}`,
		`{"activity":"export","names":["nas","router"]}`,
		`{"activity":"open"}{"activity":"export"}`,
		`{}`,
	} {
		dir := t.TempDir()
		router := &Router{persistence: config.NewConfigPersistence(dir)}
		req := httptest.NewRequest(http.MethodPost, "/api/usage/infrastructure-map", strings.NewReader(body))
		rec := httptest.NewRecorder()
		router.HandleInfrastructureMapActivity(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body %q status = %d, want 400", body, rec.Code)
		}
		if _, err := os.Stat(filepath.Join(dir, "infrastructure_map_activity_tally.json")); !os.IsNotExist(err) {
			t.Fatalf("body %q wrote the tally file", body)
		}
	}
}

func TestHandleInfrastructureMapActivityRefusesOtherMethods(t *testing.T) {
	router := &Router{persistence: config.NewConfigPersistence(t.TempDir())}
	req := httptest.NewRequest(http.MethodGet, "/api/usage/infrastructure-map", nil)
	rec := httptest.NewRecorder()
	router.HandleInfrastructureMapActivity(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET status = %d, want 405", rec.Code)
	}
}

func TestApplyInfrastructureMapTelemetrySnapshotAggregatesRollingCounts(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	now := time.Now().UTC()
	for _, activity := range []string{
		config.InfrastructureMapActivityOpen,
		config.InfrastructureMapActivityOpen,
		config.InfrastructureMapActivityOpen,
		config.InfrastructureMapActivityExport,
		config.InfrastructureMapActivityExport,
		config.InfrastructureMapActivityExportServices,
		config.InfrastructureMapActivityExportServices,
		config.InfrastructureMapActivityExportConnections,
		config.InfrastructureMapActivityExportHosts,
		config.InfrastructureMapActivityExportNamesHidden,
	} {
		if err := persistence.RecordInfrastructureMapActivity(activity, now); err != nil {
			t.Fatalf("record %q = %v", activity, err)
		}
	}
	// Outside the 30-day window: retained on disk for a day, not reported.
	if err := persistence.RecordInfrastructureMapActivity(config.InfrastructureMapActivityOpen, now.AddDate(0, 0, -31)); err != nil {
		t.Fatalf("record old open = %v", err)
	}

	snapshot := telemetry.Snapshot{}
	(&Router{persistence: persistence}).ApplyInfrastructureMapTelemetrySnapshot(&snapshot, now)
	if snapshot.InfrastructureMapOpenSessions30d != 3 ||
		snapshot.InfrastructureMapExportSessions30d != 2 ||
		snapshot.InfrastructureMapExportServicesSessions30d != 2 ||
		snapshot.InfrastructureMapExportConnectionsSessions30d != 1 ||
		snapshot.InfrastructureMapExportHostsSessions30d != 1 ||
		snapshot.InfrastructureMapExportNamesHiddenSessions30d != 1 {
		t.Fatalf("unexpected infrastructure map snapshot: open=%d export=%d services=%d connections=%d hosts=%d hidden=%d",
			snapshot.InfrastructureMapOpenSessions30d,
			snapshot.InfrastructureMapExportSessions30d,
			snapshot.InfrastructureMapExportServicesSessions30d,
			snapshot.InfrastructureMapExportConnectionsSessions30d,
			snapshot.InfrastructureMapExportHostsSessions30d,
			snapshot.InfrastructureMapExportNamesHiddenSessions30d)
	}
}
