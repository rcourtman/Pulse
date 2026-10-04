package monitoring

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	recoverymanager "github.com/rcourtman/pulse-go-rewrite/internal/recovery/manager"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func backupEvaluationMonitor(t *testing.T) (*Monitor, *config.MultiTenantPersistence) {
	t.Helper()
	mtp := config.NewMultiTenantPersistence(t.TempDir())
	manager := recoverymanager.New(mtp)
	am := alerts.NewManagerWithDataDir(t.TempDir())
	t.Cleanup(am.Stop)
	cfg := am.GetConfig()
	cfg.Enabled = true
	cfg.ActivationState = alerts.ActivationActive
	cfg.BackupDefaults.Enabled = true
	cfg.BackupDefaults.WarningDays = 1
	cfg.BackupDefaults.CriticalDays = 2
	am.UpdateConfig(cfg)
	m := &Monitor{
		state: models.NewState(), alertManager: am, recoveryManager: manager,
		resourceStore: backupReadStateResourceStore([]unifiedresources.Resource{
			{ID: "vm-100", Name: "database", Type: unifiedresources.ResourceTypeVM,
				Status:  unifiedresources.StatusOnline,
				Proxmox: &unifiedresources.ProxmoxData{Instance: "pve1", NodeName: "node1", VMID: 100}},
		}),
	}
	return m, mtp
}

func backupEvaluationPoint(at time.Time) recovery.RecoveryPoint {
	return recovery.RecoveryPoint{
		ID: "pve-backup:fixture-100", Provider: recovery.ProviderProxmoxPVE,
		Kind: recovery.KindBackup, Mode: recovery.ModeLocal, Outcome: recovery.OutcomeSuccess,
		CompletedAt: &at, SubjectResourceID: "vm-100",
		SubjectRef: &recovery.ExternalRef{Type: "proxmox-vm", ID: "pve1:node1:100", Name: "database"},
	}
}

func backupEvaluationAlerts(t *testing.T, m *Monitor) (self *alerts.Alert, ages int) {
	t.Helper()
	for _, a := range m.alertManager.GetActiveAlerts() {
		switch a.Type {
		case alerts.BackupEvaluationAlertType:
			copy := a
			self = &copy
		case "backup-age":
			ages++
		}
	}
	return
}

func TestBackupAlertEvaluationFailureAndFreshRecovery(t *testing.T) {
	m, mtp := backupEvaluationMonitor(t)
	store, err := m.recoveryManager.StoreForOrg("default")
	if err != nil {
		t.Fatal(err)
	}
	old := time.Now().UTC().Add(-4 * 24 * time.Hour)
	if err := store.UpsertPoints(context.Background(), []recovery.RecoveryPoint{backupEvaluationPoint(old)}); err != nil {
		t.Fatal(err)
	}
	m.checkBackupAlerts(context.Background())
	if self, ages := backupEvaluationAlerts(t, m); self != nil || ages != 1 {
		t.Fatalf("initial evaluation self=%+v age alerts=%d", self, ages)
	}
	// A read failure cannot be interpreted as an empty, healthy inventory.
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	m.checkBackupAlerts(context.Background())
	first, ages := backupEvaluationAlerts(t, m)
	if first == nil || ages != 1 {
		t.Fatalf("failure self=%+v age alerts=%d", first, ages)
	}
	if first.ID != alerts.SystemAlertID(alerts.BackupEvaluationAlertType) || first.ResourceID != "" || first.Level != alerts.AlertLevelWarning {
		t.Fatalf("incorrect system identity %+v", first)
	}
	if !strings.Contains(first.Message, "not evaluated") || strings.Contains(first.Message, "recovery.db") {
		t.Fatalf("unsafe or unclear message %q", first.Message)
	}
	for i := 0; i < 3; i++ {
		m.checkBackupAlerts(context.Background())
	}
	repeated, ages := backupEvaluationAlerts(t, m)
	if repeated == nil || ages != 1 || !repeated.StartTime.Equal(first.StartTime) || repeated.Metadata["systemAlertFingerprint"] != first.Metadata["systemAlertFingerprint"] {
		t.Fatal("repeated failure replaced the condition or lost existing age alerts")
	}
	// The operator-visible snapshot uses the live manager, not stale arrays.
	found := false
	for _, a := range m.GetState().ActiveAlerts {
		if a.ID == first.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("evaluation warning absent from served state")
	}
	// Reopening the same synthetic persisted data is recovery; success alone
	// clears only the self-alert while still detecting the stale backup.
	m.recoveryManager = recoverymanager.New(mtp)
	recovered, err := m.recoveryManager.StoreForOrg("default")
	if err != nil {
		t.Fatal(err)
	}
	defer recovered.Close()
	m.checkBackupAlerts(context.Background())
	if self, ages := backupEvaluationAlerts(t, m); self != nil || ages != 1 {
		t.Fatalf("read recovery self=%+v age alerts=%d", self, ages)
	}
	fresh := time.Now().UTC()
	if err := recovered.UpsertPoints(context.Background(), []recovery.RecoveryPoint{backupEvaluationPoint(fresh)}); err != nil {
		t.Fatal(err)
	}
	m.checkBackupAlerts(context.Background())
	if self, ages := backupEvaluationAlerts(t, m); self != nil || ages != 0 {
		t.Fatalf("fresh backup self=%+v age alerts=%d", self, ages)
	}
}

func TestBackupAlertEvaluationUnavailableIsNotRecovery(t *testing.T) {
	m, _ := backupEvaluationMonitor(t)
	m.recoveryManager = recoverymanager.New(nil)
	m.checkBackupAlerts(context.Background())
	self, _ := backupEvaluationAlerts(t, m)
	if self == nil {
		t.Fatal("missing persistence did not raise an evaluation warning")
	}
	m.recoveryManager = nil
	m.checkBackupAlerts(context.Background())
	if current, _ := backupEvaluationAlerts(t, m); current == nil {
		t.Fatal("missing manager falsely cleared unavailable evaluation")
	}
	cfg := m.alertManager.GetConfig()
	cfg.BackupDefaults.Enabled = false
	m.alertManager.UpdateConfig(cfg)
	m.checkBackupAlerts(context.Background())
	if current, _ := backupEvaluationAlerts(t, m); current != nil {
		t.Fatal("disabled backup checks retain an irrelevant evaluation warning")
	}
}

func TestBackupAlertEvaluationCancellationAndDeadline(t *testing.T) {
	m, _ := backupEvaluationMonitor(t)
	store, err := m.recoveryManager.StoreForOrg("default")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	m.checkBackupAlerts(cancelled)
	if self, _ := backupEvaluationAlerts(t, m); self != nil {
		t.Fatal("normal cancellation generated an outage")
	}
	deadline, stop := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer stop()
	m.checkBackupAlerts(deadline)
	if self, _ := backupEvaluationAlerts(t, m); self == nil {
		t.Fatal("deadline skip stayed invisible")
	}
	// Another normal cancellation is not positive evaluation evidence.
	m.checkBackupAlerts(cancelled)
	if self, _ := backupEvaluationAlerts(t, m); self == nil {
		t.Fatal("cancellation falsely cleared an outage")
	}
	m.checkBackupAlerts(nil)
	if self, _ := backupEvaluationAlerts(t, m); self != nil {
		t.Fatal("complete empty read did not recover the evaluator")
	}
}

func TestBackupAlertEvaluationCoalescesInFlightReads(t *testing.T) {
	m, _ := backupEvaluationMonitor(t)
	m.recoveryManager = recoverymanager.New(nil)
	m.backupAlertEvalMu.Lock()
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); m.checkBackupAlerts(context.Background()) }()
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("overlapping global reads queued behind in-flight evaluation")
	}
	m.backupAlertEvalMu.Unlock()
	if self, _ := backupEvaluationAlerts(t, m); self != nil {
		t.Fatal("coalesced read published failure without evaluating")
	}
	m.checkBackupAlerts(context.Background())
	if self, _ := backupEvaluationAlerts(t, m); self == nil {
		t.Fatal("subsequent global check did not run")
	}
}
