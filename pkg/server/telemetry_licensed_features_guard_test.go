package server

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/api"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/telemetry"
	"github.com/rcourtman/pulse-go-rewrite/pkg/audit"
)

// buildUnusedInstallPing produces the outbound payload for an install that has
// configured none of the licensed features, running every production snapshot
// path the real server runs.
//
// It deliberately installs a real SQLite audit logger, because that is what
// pkg/server does unconditionally on every install. Pinning a console logger
// here would make the guard pass while the shipped payload lied.
func buildUnusedInstallPing(t *testing.T) map[string]any {
	t.Helper()

	dataDir := t.TempDir()

	sqliteLogger, err := audit.NewSQLiteLogger(audit.SQLiteLoggerConfig{DataDir: dataDir})
	if err != nil {
		t.Fatalf("NewSQLiteLogger: %v", err)
	}
	previous := audit.GetLogger()
	audit.SetLogger(sqliteLogger)
	t.Cleanup(func() {
		audit.SetLogger(previous)
		if err := sqliteLogger.Close(); err != nil {
			t.Logf("close audit logger: %v", err)
		}
	})

	// An unused install still records audit events for defense in depth, so
	// write one. A field that counts these rather than counting operator reads
	// will be caught here.
	if err := audit.GetLogger().Record(audit.Event{
		Timestamp: time.Now().UTC(),
		EventType: "login",
		Success:   true,
	}); err != nil {
		t.Fatalf("record baseline audit event: %v", err)
	}

	now := time.Now().UTC()
	var snap telemetry.Snapshot
	applyLicensedFeatureConfigSnapshot(&snap, config.NewConfigPersistence(dataDir), now)
	(&api.Router{}).ApplyLicensedFeatureTelemetrySnapshot(&snap, now)

	ping := telemetry.BuildPingForSnapshot(snap)
	raw, err := json.Marshal(ping)
	if err != nil {
		t.Fatalf("marshal ping: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal ping: %v", err)
	}
	return decoded
}

// TestLicensedFeatureAdoptionFieldsDiscriminate is the guard that schema v6
// needed and did not have.
//
// Every field that exists to measure licensed-feature adoption must read its
// zero value on an install that uses none of those features. A field that reads
// the same whether or not the feature is used measures nothing, and is worse
// than no field because it looks like data in the fleet aggregate.
//
// audit_logging_persistent and audit_events_30d both failed this property in
// production before anyone noticed: the audit store is installed on every
// install, so the boolean was true and the count was in the tens of thousands
// on unlicensed community installs.
func TestLicensedFeatureAdoptionFieldsDiscriminate(t *testing.T) {
	payload := buildUnusedInstallPing(t)

	for _, field := range telemetry.LicensedFeatureAdoptionFields {
		value, ok := payload[field]
		if !ok {
			t.Errorf("licensed-feature field %q is missing from the outbound payload", field)
			continue
		}
		switch v := value.(type) {
		case bool:
			if v {
				t.Errorf("licensed-feature field %q is true on an install that uses no licensed feature; it cannot discriminate adoption", field)
			}
		case float64:
			if v != 0 {
				t.Errorf("licensed-feature field %q is %v on an install that uses no licensed feature; it cannot discriminate adoption", field, v)
			}
		default:
			t.Errorf("licensed-feature field %q has unsupported type %T; adoption fields must be counts or coarse booleans", field, value)
		}
	}
}

// The complement: once a feature is actually used, its field must move off
// zero. A field that is always zero is as useless as one that is always set,
// which is what pulse_intelligence_patrol_autofixes_30d was for its whole life.
func TestLicensedFeatureAdoptionFieldsMoveWhenFeaturesAreUsed(t *testing.T) {
	dataDir := t.TempDir()
	persistence := config.NewConfigPersistence(dataDir)
	now := time.Now().UTC()
	recent := now.Add(-24 * time.Hour)

	if err := persistence.SaveReportScheduleStore(config.ReportScheduleStore{
		Schedules: []config.ReportSchedule{
			{ID: "s1", Name: "weekly", Enabled: true, LastRunAt: &recent},
		},
	}); err != nil {
		t.Fatalf("SaveReportScheduleStore: %v", err)
	}
	if err := persistence.RecordAuditReadActivity(config.AuditReadActivityRecord{
		Timestamp: recent,
		Activity:  config.AuditReadActivityList,
	}); err != nil {
		t.Fatalf("RecordAuditReadActivity: %v", err)
	}

	var snap telemetry.Snapshot
	applyLicensedFeatureConfigSnapshot(&snap, persistence, now)

	if snap.AuditReads30d != 1 {
		t.Errorf("audit_reads_30d = %d after one gated read, want 1", snap.AuditReads30d)
	}
	if snap.ReportSchedules != 1 || snap.ReportSchedulesEnabled != 1 || snap.ReportSchedulesRun30d != 1 {
		t.Errorf("report schedule fields did not move: %#v", snap)
	}
}

// An audit read that fell outside the telemetry window must not be counted, or
// the field would drift into "has ever used" rather than "is using".
func TestAuditReadsOutsideWindowAreNotCounted(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	now := time.Now().UTC()
	stale := now.Add(-(telemetry.PulseIntelligenceTelemetryWindow + 48*time.Hour))

	if err := persistence.RecordAuditReadActivity(config.AuditReadActivityRecord{
		Timestamp: stale,
		Activity:  config.AuditReadActivityExport,
	}); err != nil {
		t.Fatalf("RecordAuditReadActivity: %v", err)
	}

	var snap telemetry.Snapshot
	applyLicensedFeatureConfigSnapshot(&snap, persistence, now)

	if snap.AuditReads30d != 0 {
		t.Errorf("audit_reads_30d = %d for a read outside the window, want 0", snap.AuditReads30d)
	}
}
