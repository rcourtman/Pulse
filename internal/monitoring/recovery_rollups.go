package monitoring

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	"github.com/rs/zerolog/log"
)

const (
	alertRollupsPageLimit = 500
	alertRollupsMaxPages  = 50
)

func (m *Monitor) listRecoveryRollupsForAlerts(ctx context.Context, kind recovery.Kind) ([]recovery.ProtectionRollup, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if mock.IsMockEnabled() {
		points := mock.CurrentFixtureGraph().RecoveryPoints()
		filtered := make([]recovery.RecoveryPoint, 0, len(points))
		for _, p := range points {
			if kind != "" && p.Kind != kind {
				continue
			}
			filtered = append(filtered, p)
		}
		return recovery.BuildRollupsFromPoints(filtered), nil
	}

	if m == nil || m.recoveryManager == nil {
		return nil, nil
	}
	orgID := m.GetOrgID()
	if orgID == "" {
		orgID = "default"
	}

	store, err := m.recoveryManager.StoreForOrg(orgID)
	if err != nil {
		return nil, err
	}

	opts := recovery.ListPointsOptions{
		Kind:  kind,
		Page:  1,
		Limit: alertRollupsPageLimit,
	}

	out := make([]recovery.ProtectionRollup, 0, 256)
	page := 1
	for page <= alertRollupsMaxPages {
		opts.Page = page
		rollups, total, err := store.ListRollups(ctx, opts)
		if err != nil {
			return nil, err
		}
		out = append(out, rollups...)

		if len(rollups) < opts.Limit {
			break
		}
		totalPages := 1
		if opts.Limit > 0 {
			totalPages = (total + opts.Limit - 1) / opts.Limit
		}
		if page >= totalPages {
			break
		}
		if page == alertRollupsMaxPages && page < totalPages {
			return out, fmt.Errorf("recovery rollups pagination exceeded max pages (%d)", alertRollupsMaxPages)
		}
		page++
	}

	return out, nil
}

func (m *Monitor) listBackupRollupsForAlerts(ctx context.Context) ([]recovery.ProtectionRollup, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return m.listRecoveryRollupsForAlerts(ctx, recovery.KindBackup)
}

// checkBackupAlerts is shared by PVE, PBS and mock polls. A failed/partial
// rollup read is not an empty inventory and cannot clear existing age alerts.
// Put the lost evaluation in the existing alert list, with one stable identity
// and no raw database/path/provider errors in its public message.
func (m *Monitor) checkBackupAlerts(ctx context.Context) {
	if m == nil || m.alertManager == nil || !m.backupAlertEvalMu.TryLock() {
		return
	}
	defer m.backupAlertEvalMu.Unlock()

	// Rollups and the guest lookup come from the fixture graph in mock mode;
	// an evaluation that read them must not land after a mode change.
	scope := m.mockModeFence.begin()
	cfg := m.alertManager.GetConfig()
	if !cfg.Enabled || !cfg.BackupDefaults.Enabled ||
		(cfg.BackupDefaults.WarningDays <= 0 && cfg.BackupDefaults.CriticalDays <= 0) {
		// Disabling evaluation is a deliberate policy change, not a read failure.
		m.alertManager.CheckBackupsWithInventory(nil, nil, nil, nil)
		m.alertManager.ClearSystemAlert(alerts.BackupEvaluationAlertType)
		return
	}
	if m.recoveryManager == nil && !mock.IsMockEnabled() {
		// Missing dependencies cannot stand in for affirmative read recovery.
		return
	}
	rollups, err := m.listBackupRollupsForAlerts(ctx)
	if err != nil {
		// Normal shutdown/cancellation is not a new evaluation outage. A read
		// deadline, including connection-pool waiting, still needs visibility.
		if !errors.Is(err, context.Canceled) {
			log.Warn().Err(err).Msg("Failed to list recovery rollups for backup alerts")
			scope.run(func() {
				m.alertManager.RaiseSystemAlert(alerts.SystemAlertInput{
					Type:        alerts.BackupEvaluationAlertType,
					Level:       alerts.AlertLevelWarning,
					Fingerprint: "recovery-rollups-unavailable",
					Message:     "Backup-age alerts were not evaluated because recovery data could not be read. Existing backup alerts have been kept; check Pulse's logs.",
				})
			})
		}
		return
	}
	guestsByKey, guestsByVMID := buildGuestLookupsFromReadState(m.currentModeReadState(), m.guestMetadataStore)
	scope.run(func() {
		m.alertManager.CheckBackupsWithInventory(rollups, guestsByKey, guestsByVMID, m.backupInventoryScopeForAlerts())
		// Only a complete read/evaluation (including a genuinely empty result)
		// resolves the outage. In-flight checks are coalesced, not queued, so an
		// older successful read cannot later erase a newer failure.
		m.alertManager.ClearSystemAlert(alerts.BackupEvaluationAlertType)
	})
}
