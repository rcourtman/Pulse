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

	if activity, err := persistence.ReportingActivitySnapshot(now); err == nil {
		snap.ReportingGeneratedResources30d = activity.Counts.Generated[0]
		snap.ReportingGeneratedVMInventory30d = activity.Counts.Generated[1]
		snap.ReportingGeneratedBackupHealth30d = activity.Counts.Generated[2]
		snap.ReportingGeneratedPatrolDigest30d = activity.Counts.Generated[3]
		snap.ReportingSuccessfulResourcesPeriods30d = activity.Counts.Successful[0]
		snap.ReportingSuccessfulVMInventoryPeriods30d = activity.Counts.Successful[1]
		snap.ReportingSuccessfulBackupHealthPeriods30d = activity.Counts.Successful[2]
		snap.ReportingSuccessfulPatrolDigestPeriods30d = activity.Counts.Successful[3]
		snap.ReportingRepeatResourcesPeriods30d = activity.Counts.Repeated[0]
		snap.ReportingRepeatVMInventoryPeriods30d = activity.Counts.Repeated[1]
		snap.ReportingRepeatBackupHealthPeriods30d = activity.Counts.Repeated[2]
		snap.ReportingRepeatPatrolDigestPeriods30d = activity.Counts.Repeated[3]
		snap.ReportingSaved30d = activity.Counts.Saved
		snap.ReportingEmailAccepted30d = activity.Counts.EmailAccepted
		snap.ReportingDeliveryFailed30d = activity.Counts.DeliveryFailed
		snap.ReportingDeliveryUnconfirmed30d = activity.Counts.DeliveryUnconfirmed
		snap.ReportingDataComplete30d = activity.Counts.DataComplete
		snap.ReportingDataIncomplete30d = activity.Counts.DataIncomplete
		snap.ReportingDataUnavailable30d = activity.Counts.DataUnavailable
		snap.ReportingTelemetryAvailable = activity.Available
		snap.ReportingObservationDays = activity.ObservationDays
		snap.ReportingResourcesActiveDays30d = activity.ActiveDays[0]
		snap.ReportingVMInventoryActiveDays30d = activity.ActiveDays[1]
		snap.ReportingBackupHealthActiveDays30d = activity.ActiveDays[2]
		snap.ReportingPatrolDigestActiveDays30d = activity.ActiveDays[3]
	}

	if profiles, err := persistence.LoadAgentProfiles(); err == nil {
		snap.AgentProfiles = len(profiles)
	}

	snap.SuccessfulHistoryReadsOver7d30d = persistence.CountSuccessfulExtendedHistoryReadsSince(now.Add(-telemetry.PulseIntelligenceTelemetryWindow), now)

	// Audit adoption is a read count, not store presence: the SQLite audit
	// logger is installed on every install for defense in depth, so only an
	// operator reaching a license-gated read surface distinguishes an install
	// that uses audit logging from one that merely has it.
	snap.AuditReads30d = persistence.CountAuditReadActivitySince(now.Add(-telemetry.PulseIntelligenceTelemetryWindow))
}
