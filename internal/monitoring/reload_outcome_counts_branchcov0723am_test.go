package monitoring

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBranchcov0723Am_AccumulateAlertOutcomeCounts drives every branch of
// accumulateAlertOutcomeCounts: the nil-counts guard, the empty-history path
// (which must not perturb pre-existing counts), and every alert classification
// the loop distinguishes, including the precise cutoff boundary semantics of
// `!AckTime.Before(cutoff)`.
func TestBranchcov0723Am_AccumulateAlertOutcomeCounts(t *testing.T) {
	cutoff := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	t.Run("nil_counts_is_noop", func(t *testing.T) {
		// The guard at the top of the function is the only thing that prevents
		// a nil-pointer dereference once the loop begins mutating fields, so it
		// has no observable side effect beyond returning safely. Pass a rich
		// history so the guard is provably the gate.
		richHistory := []alerts.Alert{
			{AckTime: &cutoff, OperationalRecord: &operationaltrust.OperationalRecord{State: operationaltrust.OperationalResolved}},
		}
		assert.NotPanics(t, func() {
			accumulateAlertOutcomeCounts(nil, richHistory, cutoff)
		})
	})

	t.Run("empty_history_preserves_existing_counts", func(t *testing.T) {
		// Pre-existing totals (resource counts and prior alert totals) must be
		// preserved untouched when there is no history to fold in, and the three
		// alert-outcome fields must not be incremented.
		counts := InstallSnapshotCounts{
			PVENodes:              3,
			AlertsFired30d:        7,
			AlertsAcknowledged30d: 2,
			AlertsResolved30d:     1,
		}
		accumulateAlertOutcomeCounts(&counts, nil, cutoff)
		assert.Equal(t, InstallSnapshotCounts{
			PVENodes:              3,
			AlertsFired30d:        7,
			AlertsAcknowledged30d: 2,
			AlertsResolved30d:     1,
		}, counts)

		// Same behaviour with an explicit empty (non-nil) slice.
		counts2 := InstallSnapshotCounts{
			PVENodes:              3,
			AlertsFired30d:        7,
			AlertsAcknowledged30d: 2,
			AlertsResolved30d:     1,
		}
		accumulateAlertOutcomeCounts(&counts2, []alerts.Alert{}, cutoff)
		assert.Equal(t, counts, counts2)
	})

	t.Run("classifications_and_cutoff_boundary", func(t *testing.T) {
		// AckTime exactly at the cutoff must be counted (Before is strict), an
		// AckTime strictly before must not, and each OperationalRecord
		// classification must increment only the resolved field.
		atCutoff := cutoff
		beforeCutoff := cutoff.Add(-time.Second)
		afterCutoff := cutoff.Add(time.Second)

		counts := InstallSnapshotCounts{}
		accumulateAlertOutcomeCounts(&counts, []alerts.Alert{
			// 1) Acknowledged strictly before cutoff: fired, NOT acked, no OR.
			{AckTime: &beforeCutoff},
			// 2) Acknowledged exactly at cutoff: fired, acked.
			{AckTime: &atCutoff},
			// 3) Acknowledged after cutoff: fired, acked.
			{AckTime: &afterCutoff},
			// 4) Nil AckTime: fired, NOT acked, nil OR.
			{},
			// 5) Resolved operational record, acknowledged: fired, acked, resolved.
			{
				AckTime:           &afterCutoff,
				OperationalRecord: &operationaltrust.OperationalRecord{State: operationaltrust.OperationalResolved},
			},
			// 6) Non-resolved operational record, nil ack: fired, NOT acked, NOT resolved.
			{
				OperationalRecord: &operationaltrust.OperationalRecord{State: operationaltrust.OperationalOpen},
			},
		}, cutoff)

		assert.Equal(t, 6, counts.AlertsFired30d, "every history entry is fired")
		assert.Equal(t, 3, counts.AlertsAcknowledged30d, "acked: at-cutoff, after-cutoff, resolved+after")
		assert.Equal(t, 1, counts.AlertsResolved30d, "only the resolved operational record counts")
	})
}

// TestBranchcov0723Am_AccumulateInstallOutcomeCounts covers the guard arms and
// every arm reachable with a zero-value or minimally-wired Monitor. The
// notification-manager arm (and its error arm) are intentionally skipped: they
// require a sqlite-backed NotificationManager with seeded audit rows to assert
// meaningfully, which is fully-wired-live-Monitor territory.
func TestBranchcov0723Am_AccumulateInstallOutcomeCounts(t *testing.T) {
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)

	// sentryCounts is a baseline that must survive any no-op accumulation call
	// so a regression that spuriously mutates counts is caught.
	sentryCounts := func() InstallSnapshotCounts {
		return InstallSnapshotCounts{
			PVENodes:               4,
			ActiveAlerts:           9,
			AlertsFired30d:         5,
			NotificationAttempts7d: 11,
		}
	}

	t.Run("nil_counts_is_noop", func(t *testing.T) {
		// Guard fires before any manager access; only observable effect is safe
		// return. Use a Monitor with a real alert manager so the guard is
		// provably the only gate in front of the nil pointer.
		alertMgr := alerts.NewManagerWithDataDir(t.TempDir())
		defer alertMgr.Stop()
		mon := &Monitor{alertManager: alertMgr}
		assert.NotPanics(t, func() {
			accumulateInstallOutcomeCounts(nil, mon, now)
		})
	})

	t.Run("nil_monitor_leaves_counts_untouched", func(t *testing.T) {
		counts := sentryCounts()
		accumulateInstallOutcomeCounts(&counts, nil, now)
		assert.Equal(t, sentryCounts(), counts)
	})

	t.Run("zero_monitor_zero_now_uses_now_recompute", func(t *testing.T) {
		// now.IsZero() true-arm: the function recomputes now internally. With a
		// zero-value Monitor both managers are nil, so no counts accrue.
		counts := sentryCounts()
		assert.NotPanics(t, func() {
			accumulateInstallOutcomeCounts(&counts, &Monitor{}, time.Time{})
		})
		assert.Equal(t, sentryCounts(), counts)
	})

	t.Run("zero_monitor_real_now_skips_nil_managers", func(t *testing.T) {
		// now.IsZero() false-arm plus the nil alertManager and nil
		// notificationManager branches: nothing is accumulated.
		counts := sentryCounts()
		mon := &Monitor{}
		require.Nil(t, mon.GetAlertManager())
		require.Nil(t, mon.GetNotificationManager())
		accumulateInstallOutcomeCounts(&counts, mon, now)
		assert.Equal(t, sentryCounts(), counts)
	})

	t.Run("alert_manager_with_empty_history_increments_nothing", func(t *testing.T) {
		// Covers the alertManager != nil true-arm. A freshly-constructed alert
		// manager has no history, so GetAlertHistorySince returns nothing and
		// no alert-outcome counts accrue. (Deeper history-driven counting is
		// exercised directly on accumulateAlertOutcomeCounts above; history
		// cannot be injected into an alerts.Manager from outside the alerts
		// package without driving the full evaluation pipeline.)
		alertMgr := alerts.NewManagerWithDataDir(t.TempDir())
		defer alertMgr.Stop()

		mon := &Monitor{alertManager: alertMgr}
		require.NotNil(t, mon.GetAlertManager())

		counts := sentryCounts()
		accumulateInstallOutcomeCounts(&counts, mon, now)
		assert.Equal(t, sentryCounts(), counts, "empty history must not change any count")
	})
}

// TestBranchcov0723Am_AggregateInstallSnapshotCounts covers the uncovered arms
// of AggregateInstallSnapshotCounts that the existing
// TestReloadableMonitorAggregateInstallSnapshotCountsIncludesProvisionedTenants
// does not reach: nil mtMonitor, nil persistence (default-only path), the
// ListOrganizations error fallback, and the GetMonitor-skip arm.
func TestBranchcov0723Am_AggregateInstallSnapshotCounts(t *testing.T) {
	t.Run("nil_mtmonitor_returns_empty", func(t *testing.T) {
		cfg := &config.Config{DataPath: t.TempDir()}
		rm, err := NewReloadableMonitor(cfg, config.NewMultiTenantPersistence(cfg.DataPath), nil)
		require.NoError(t, err)
		rm.mtMonitor = nil

		assert.Equal(t, InstallSnapshotCounts{}, rm.AggregateInstallSnapshotCounts())
	})

	t.Run("nil_persistence_aggregates_default_only", func(t *testing.T) {
		// persistence == nil keeps orgIDs as ["default"]. Pre-seed the default
		// monitor so GetMonitor returns it without touching persistence.
		cfg := &config.Config{DataPath: t.TempDir()}
		rm, err := NewReloadableMonitor(cfg, nil, nil)
		require.NoError(t, err)

		mtm := rm.GetMultiTenantMonitor()
		require.NotNil(t, mtm)
		mtm.monitors["default"] = testTelemetryMonitor(
			[]models.Node{{ID: "n1", Name: "n1", Instance: "pve-1"}},
			[]models.VM{{ID: "vm1", VMID: 1, Name: "vm1", Instance: "pve-1"}},
			nil, nil, nil, nil, nil, 2,
		)

		counts := rm.AggregateInstallSnapshotCounts()
		assert.Equal(t, 1, counts.PVENodes, "default org aggregated on nil-persistence path")
		assert.Equal(t, 1, counts.VMs)
		assert.Equal(t, 2, counts.ActiveAlerts)
	})

	t.Run("list_organizations_error_falls_back_to_default", func(t *testing.T) {
		// Make the orgs entry a regular file so ReadDir fails with a
		// non-IsNotExist error, triggering the ListOrganizations error branch
		// and the fallback to orgIDs=["default"].
		baseDir := t.TempDir()
		cfg := &config.Config{DataPath: baseDir}
		persistence := config.NewMultiTenantPersistence(baseDir)

		rm, err := NewReloadableMonitor(cfg, persistence, nil)
		require.NoError(t, err)

		orgsPath := filepath.Join(persistence.BaseDataDir(), "orgs")
		// The orgs entry may not exist yet at construction time; only remove if
		// present, then replace it with a regular file so ReadDir fails.
		if err := os.Remove(orgsPath); err != nil && !os.IsNotExist(err) {
			require.NoError(t, err)
		}
		require.NoError(t, os.WriteFile(orgsPath, []byte("not a directory"), 0o644))

		mtm := rm.GetMultiTenantMonitor()
		require.NotNil(t, mtm)
		mtm.monitors["default"] = testTelemetryMonitor(
			[]models.Node{{ID: "n1", Name: "n1", Instance: "pve-1"}},
			nil, nil, nil, nil, nil, nil, 0,
		)

		counts := rm.AggregateInstallSnapshotCounts()
		assert.Equal(t, 1, counts.PVENodes, "default aggregated despite tenant listing failure")
	})

	t.Run("deleting_org_is_skipped", func(t *testing.T) {
		// Provision a second org so persistence lists it, then mark it as
		// being-deleted so GetMonitor errors and it is skipped while default is
		// still aggregated.
		baseDir := t.TempDir()
		cfg := &config.Config{DataPath: baseDir}
		persistence := config.NewMultiTenantPersistence(baseDir)
		_, err := persistence.GetPersistence("ghost")
		require.NoError(t, err)

		rm, err := NewReloadableMonitor(cfg, persistence, nil)
		require.NoError(t, err)

		mtm := rm.GetMultiTenantMonitor()
		require.NotNil(t, mtm)
		mtm.monitors["default"] = testTelemetryMonitor(
			[]models.Node{{ID: "n1", Name: "n1", Instance: "pve-1"}},
			nil, nil, nil, nil, nil, nil, 0,
		)
		mtm.tenantDeleting["ghost"] = struct{}{}

		counts := rm.AggregateInstallSnapshotCounts()
		assert.Equal(t, 1, counts.PVENodes, "default aggregated, deleting org contributed nothing")
	})
}
