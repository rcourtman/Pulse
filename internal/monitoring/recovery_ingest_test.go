package monitoring

import (
	"context"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	recoverymanager "github.com/rcourtman/pulse-go-rewrite/internal/recovery/manager"
)

func recoveryIngestTestMonitor(t *testing.T) (*Monitor, *recoverymanager.Manager) {
	t.Helper()
	mtp := config.NewMultiTenantPersistence(t.TempDir())
	manager := recoverymanager.New(mtp)
	return &Monitor{recoveryManager: manager}, manager
}

func recoveryIngestTestPoint(id string, instance string, at time.Time) recovery.RecoveryPoint {
	return recovery.RecoveryPoint{
		ID:          id,
		Provider:    recovery.ProviderProxmoxPVE,
		Kind:        recovery.KindBackup,
		Mode:        recovery.ModeLocal,
		Outcome:     recovery.OutcomeSuccess,
		StartedAt:   &at,
		CompletedAt: &at,
		Details:     map[string]any{"instance": instance},
	}
}

func listRecoveryPointIDs(t *testing.T, manager *recoverymanager.Manager) map[string]bool {
	t.Helper()
	store, err := manager.StoreForOrg("default")
	if err != nil {
		t.Fatalf("StoreForOrg(default): %v", err)
	}
	points, _, err := store.ListPoints(context.Background(), recovery.ListPointsOptions{})
	if err != nil {
		t.Fatalf("ListPoints(): %v", err)
	}
	ids := make(map[string]bool, len(points))
	for _, p := range points {
		ids[p.ID] = true
	}
	return ids
}

// Regression test for the monitoring-layer wiring of #1580: a poll cycle's
// complete enumeration must delete in-scope points the source stopped
// reporting, so a backup deleted upstream cannot keep re-raising its alert.
func TestIngestRecoveryPointsBestEffortReconcilesScope(t *testing.T) {
	m, manager := recoveryIngestTestMonitor(t)
	store, err := manager.StoreForOrg("default")
	if err != nil {
		t.Fatalf("StoreForOrg(default): %v", err)
	}

	now := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	seed := []recovery.RecoveryPoint{
		recoveryIngestTestPoint("pve-backup:pve1-vzdump-100", "pve1", now),
		recoveryIngestTestPoint("pve-backup:pve1-vzdump-101", "pve1", now),
		// Same class, different instance: out of scope, must survive.
		recoveryIngestTestPoint("pve-backup:pve2-vzdump-100", "pve2", now),
	}
	if err := store.UpsertPoints(context.Background(), seed); err != nil {
		t.Fatalf("UpsertPoints(): %v", err)
	}

	// The next poll of pve1 enumerates only vzdump-100.
	m.ingestRecoveryPointsBestEffort(context.Background(), recoveryIngestBatch{
		points: []recovery.RecoveryPoint{
			recoveryIngestTestPoint("pve-backup:pve1-vzdump-100", "pve1", now),
		},
		reconcile: &recoveryReconcileScope{
			provider: string(recovery.ProviderProxmoxPVE),
			idPrefix: "pve-backup:",
			instance: "pve1",
		},
	})

	ids := listRecoveryPointIDs(t, manager)
	if ids["pve-backup:pve1-vzdump-101"] {
		t.Fatal("stale point pve-backup:pve1-vzdump-101 should have been reconciled away")
	}
	if !ids["pve-backup:pve1-vzdump-100"] {
		t.Fatal("still-enumerated point pve-backup:pve1-vzdump-100 must survive")
	}
	if !ids["pve-backup:pve2-vzdump-100"] {
		t.Fatal("other-instance point pve-backup:pve2-vzdump-100 must survive")
	}
}

// An empty enumeration is meaningful: the source reports no backups at all,
// so the whole scope clears (the delete-all-backups case from #1580).
func TestIngestRecoveryPointsBestEffortEmptyEnumerationClearsScope(t *testing.T) {
	m, manager := recoveryIngestTestMonitor(t)
	store, err := manager.StoreForOrg("default")
	if err != nil {
		t.Fatalf("StoreForOrg(default): %v", err)
	}

	now := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	if err := store.UpsertPoints(context.Background(), []recovery.RecoveryPoint{
		recoveryIngestTestPoint("pve-backup:pve1-vzdump-100", "pve1", now),
		recoveryIngestTestPoint("pve-backup:pve2-vzdump-100", "pve2", now),
	}); err != nil {
		t.Fatalf("UpsertPoints(): %v", err)
	}

	m.ingestRecoveryPointsBestEffort(context.Background(), recoveryIngestBatch{
		points: nil,
		reconcile: &recoveryReconcileScope{
			provider: string(recovery.ProviderProxmoxPVE),
			idPrefix: "pve-backup:",
			instance: "pve1",
		},
	})

	ids := listRecoveryPointIDs(t, manager)
	if ids["pve-backup:pve1-vzdump-100"] {
		t.Fatal("pve1 scope should be empty after an empty enumeration")
	}
	if !ids["pve-backup:pve2-vzdump-100"] {
		t.Fatal("other-instance point must survive an empty pve1 enumeration")
	}
}

// A failed upsert must not reconcile: deleting on top of a failed batch could
// drop points the source still reports.
func TestIngestRecoveryPointsBestEffortSkipsReconcileWhenUpsertFails(t *testing.T) {
	m, manager := recoveryIngestTestMonitor(t)
	store, err := manager.StoreForOrg("default")
	if err != nil {
		t.Fatalf("StoreForOrg(default): %v", err)
	}

	now := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	if err := store.UpsertPoints(context.Background(), []recovery.RecoveryPoint{
		recoveryIngestTestPoint("pve-backup:pve1-vzdump-100", "pve1", now),
		recoveryIngestTestPoint("pve-backup:pve1-vzdump-101", "pve1", now),
	}); err != nil {
		t.Fatalf("UpsertPoints(): %v", err)
	}

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	m.ingestRecoveryPointsBestEffort(cancelled, recoveryIngestBatch{
		points: []recovery.RecoveryPoint{
			recoveryIngestTestPoint("pve-backup:pve1-vzdump-100", "pve1", now),
		},
		reconcile: &recoveryReconcileScope{
			provider: string(recovery.ProviderProxmoxPVE),
			idPrefix: "pve-backup:",
			instance: "pve1",
		},
	})

	ids := listRecoveryPointIDs(t, manager)
	if !ids["pve-backup:pve1-vzdump-101"] {
		t.Fatal("reconcile must be skipped when the upsert fails; vzdump-101 was deleted")
	}
}

func TestIngestRecoveryPointsBestEffortPersistsProviderObservationAndPosture(t *testing.T) {
	t.Parallel()

	m, manager := recoveryIngestTestMonitor(t)
	now := time.Now().UTC().Truncate(time.Millisecond)
	point := recoveryIngestTestPoint("pve-backup:pve1-vzdump-100", "pve1", now)
	point.SubjectResourceID = "resource:vm-100"
	observation, err := recovery.NewProtectionProviderObservation(
		recovery.ProviderProxmoxPVE,
		"pve-backup-enumeration",
		"pve1",
		recovery.OutcomeSuccess,
		recovery.ProtectionHistoryComplete,
		operationaltrust.EvidencePermissionsSufficient,
		false,
		now,
		now,
		nil,
	)
	if err != nil {
		t.Fatalf("NewProtectionProviderObservation() error = %v", err)
	}

	m.ingestRecoveryPointsBestEffort(context.Background(), recoveryIngestBatch{
		points: []recovery.RecoveryPoint{point},
		observations: []recovery.ProtectionProviderObservation{
			observation,
		},
	})

	store, err := manager.StoreForOrg("default")
	if err != nil {
		t.Fatalf("StoreForOrg(default): %v", err)
	}
	postures, _, err := store.ListProtectionPostures(
		context.Background(),
		recovery.ProtectionPostureQuery{
			SubjectResourceIDs: []string{"resource:vm-100"},
		},
	)
	if err != nil {
		t.Fatalf("ListProtectionPostures() error = %v", err)
	}
	if len(postures) != 1 ||
		postures[0].State != recovery.ProtectionStateProtected {
		t.Fatalf("postures = %#v, want one protected posture", postures)
	}
}

func TestIngestRecoveryPointsBestEffortPersistsProviderObservationBeforePointFailure(t *testing.T) {
	t.Parallel()

	m, manager := recoveryIngestTestMonitor(t)
	store, err := manager.StoreForOrg("default")
	if err != nil {
		t.Fatalf("StoreForOrg(default): %v", err)
	}
	now := time.Now().UTC().Truncate(time.Millisecond)
	existing := recoveryIngestTestPoint("pve-backup:pve1-vzdump-100", "pve1", now)
	existing.SubjectResourceID = "resource:vm-100"
	if err := store.UpsertPoints(context.Background(), []recovery.RecoveryPoint{existing}); err != nil {
		t.Fatalf("UpsertPoints(existing): %v", err)
	}
	observation, err := recovery.NewProtectionProviderObservation(
		recovery.ProviderProxmoxPVE,
		"pve-backup-enumeration",
		"pve1",
		recovery.OutcomeSuccess,
		recovery.ProtectionHistoryComplete,
		operationaltrust.EvidencePermissionsSufficient,
		false,
		now,
		now,
		nil,
	)
	if err != nil {
		t.Fatalf("NewProtectionProviderObservation() error = %v", err)
	}

	invalidPoint := existing
	invalidPoint.ID = ""
	m.ingestRecoveryPointsBestEffort(context.Background(), recoveryIngestBatch{
		points:       []recovery.RecoveryPoint{invalidPoint},
		observations: []recovery.ProtectionProviderObservation{observation},
	})

	postures, _, err := store.ListProtectionPostures(
		context.Background(),
		recovery.ProtectionPostureQuery{
			SubjectResourceIDs: []string{"resource:vm-100"},
		},
	)
	if err != nil {
		t.Fatalf("ListProtectionPostures() error = %v", err)
	}
	if len(postures) != 1 ||
		postures[0].State != recovery.ProtectionStateProtected {
		t.Fatalf(
			"postures = %#v, want provider observation to survive point failure",
			postures,
		)
	}
}
