package server

import (
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/telemetry"
)

// applyLicensedFeatureConfigSnapshot adds content-free adoption counts for
// licensed features whose state lives in persisted config files.
//
// Counts only. Schedule names, delivery recipients, report scope, and profile
// names all stay on the install; the aggregate answers "how many are
// configured and how many ran", nothing about what they contain.
func applyLicensedFeatureConfigSnapshot(snap *telemetry.Snapshot, persistence *config.ConfigPersistence, now time.Time) {
	if snap == nil || persistence == nil {
		return
	}

	if store, err := persistence.LoadReportScheduleStore(); err == nil && store != nil {
		since := now.Add(-telemetry.PulseIntelligenceTelemetryWindow)
		for _, schedule := range store.Schedules {
			snap.ReportSchedules++
			if schedule.Enabled {
				snap.ReportSchedulesEnabled++
			}
			if schedule.LastRunAt != nil && schedule.LastRunAt.After(since) {
				snap.ReportSchedulesRun30d++
			}
		}
	}

	if profiles, err := persistence.LoadAgentProfiles(); err == nil {
		snap.AgentProfiles = len(profiles)
	}

	// Audit adoption is a read count, not store presence: the SQLite audit
	// logger is installed on every install for defense in depth, so only an
	// operator reaching a license-gated read surface distinguishes an install
	// that uses audit logging from one that merely has it.
	snap.AuditReads30d = persistence.CountAuditReadActivitySince(now.Add(-telemetry.PulseIntelligenceTelemetryWindow))
}
