package server

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/telemetry"
)

func TestApplyLicensedFeatureConfigSnapshot_CountsScheduledReportingAndProfiles(t *testing.T) {
	now := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	recent := now.Add(-48 * time.Hour)
	old := now.Add(-(telemetry.PulseIntelligenceTelemetryWindow + time.Hour))

	persistence := config.NewConfigPersistence(t.TempDir())
	if err := persistence.SaveReportScheduleStore(config.ReportScheduleStore{
		Schedules: []config.ReportSchedule{
			{ID: "a", Name: "weekly-capacity", Enabled: true, LastRunAt: &recent},
			{ID: "b", Name: "monthly-uptime", Enabled: true, LastRunAt: &old},
			{ID: "c", Name: "paused", Enabled: false, LastRunAt: &recent},
			{ID: "d", Name: "never-run", Enabled: true},
		},
	}); err != nil {
		t.Fatalf("SaveReportScheduleStore: %v", err)
	}
	if err := persistence.SaveAgentProfiles([]models.AgentProfile{
		{ID: "profile-1", Name: "edge"},
		{ID: "profile-2", Name: "core"},
	}); err != nil {
		t.Fatalf("SaveAgentProfiles: %v", err)
	}

	var snap telemetry.Snapshot
	applyLicensedFeatureConfigSnapshot(&snap, persistence, now)

	if snap.ReportSchedules != 4 {
		t.Fatalf("report schedules = %d, want 4", snap.ReportSchedules)
	}
	if snap.ReportSchedulesEnabled != 3 {
		t.Fatalf("enabled report schedules = %d, want 3", snap.ReportSchedulesEnabled)
	}
	// Only the two schedules whose last run falls inside the window count, and
	// a disabled schedule that still ran recently is one of them.
	if snap.ReportSchedulesRun30d != 2 {
		t.Fatalf("report schedules run in window = %d, want 2", snap.ReportSchedulesRun30d)
	}
	if snap.AgentProfiles != 2 {
		t.Fatalf("agent profiles = %d, want 2", snap.AgentProfiles)
	}
}

func TestApplyLicensedFeatureConfigSnapshot_LeavesCountsZeroWhenNothingConfigured(t *testing.T) {
	var snap telemetry.Snapshot
	applyLicensedFeatureConfigSnapshot(&snap, config.NewConfigPersistence(t.TempDir()), time.Now().UTC())

	if snap.ReportSchedules != 0 || snap.ReportSchedulesEnabled != 0 || snap.ReportSchedulesRun30d != 0 || snap.AgentProfiles != 0 {
		t.Fatalf("unconfigured install must report zeroes: %#v", snap)
	}
}

// A nil persistence must not panic: the telemetry snapshot runs on a timer and
// a failed persistence init cannot be allowed to take the process down.
func TestApplyLicensedFeatureConfigSnapshot_ToleratesNilInputs(t *testing.T) {
	var snap telemetry.Snapshot
	applyLicensedFeatureConfigSnapshot(&snap, nil, time.Now().UTC())
	applyLicensedFeatureConfigSnapshot(nil, config.NewConfigPersistence(t.TempDir()), time.Now().UTC())
}

func TestApplyLicensedFeatureConfigSnapshotIncludesSuccessfulExtendedHistoryReads(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	now := time.Now().UTC()
	if err := persistence.RecordSuccessfulExtendedHistoryRead(now); err != nil {
		t.Fatal(err)
	}
	var snap telemetry.Snapshot
	applyLicensedFeatureConfigSnapshot(&snap, persistence, now)
	if snap.SuccessfulHistoryReadsOver7d30d != 1 {
		t.Fatalf("extended history reads = %d, want 1", snap.SuccessfulHistoryReadsOver7d30d)
	}
}

func TestApplyLicensedFeatureConfigSnapshotReportingOutcomes(t *testing.T) {
	p := config.NewConfigPersistence(t.TempDir())
	now := time.Now().UTC()
	for _, event := range []string{"generated", "saved", "email_accepted", "delivery_failed", "delivery_unconfirmed"} {
		if err := p.RecordReportingActivity(config.ReportingActivityEvent{At: now, Kind: config.ReportScheduleKindBackupHealth, Event: event, DataState: "partial"}); err != nil {
			t.Fatal(err)
		}
	}
	var snap telemetry.Snapshot
	applyLicensedFeatureConfigSnapshot(&snap, p, now)
	if !snap.ReportingTelemetryAvailable || snap.ReportingObservationDays != 1 || snap.ReportingGeneratedBackupHealth30d != 1 || snap.ReportingBackupHealthActiveDays30d != 1 || snap.ReportingSaved30d != 1 || snap.ReportingEmailAccepted30d != 1 || snap.ReportingDeliveryFailed30d != 1 || snap.ReportingDeliveryUnconfirmed30d != 1 || snap.ReportingDataIncomplete30d != 1 {
		t.Fatalf("reporting snapshot: %+v", snap)
	}
}
