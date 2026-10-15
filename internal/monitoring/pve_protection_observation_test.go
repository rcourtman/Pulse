package monitoring

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
)

func TestBuildPVEProtectionProviderObservationMapsEvidenceQuality(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 4, 8, 0, 0, 0, time.UTC)
	tests := []struct {
		name            string
		stats           pveProtectionCollectionStats
		wantJob         recovery.Outcome
		wantHistory     recovery.ProtectionHistoryCompleteness
		wantPermissions operationaltrust.EvidencePermissions
		wantReason      string
	}{
		{
			name: "complete enumeration",
			stats: pveProtectionCollectionStats{
				NodeCount:             2,
				SuccessfulNodeQueries: 2,
				BackupStorageCount:    2,
				ContentSuccesses:      2,
			},
			wantJob:         recovery.OutcomeSuccess,
			wantHistory:     recovery.ProtectionHistoryComplete,
			wantPermissions: operationaltrust.EvidencePermissionsSufficient,
		},
		{
			name: "complete empty enumeration",
			stats: pveProtectionCollectionStats{
				NodeCount:             1,
				SuccessfulNodeQueries: 1,
			},
			wantJob:         recovery.OutcomeSuccess,
			wantHistory:     recovery.ProtectionHistoryComplete,
			wantPermissions: operationaltrust.EvidencePermissionsSufficient,
		},
		{
			name: "partial transient enumeration",
			stats: pveProtectionCollectionStats{
				NodeCount:             2,
				SuccessfulNodeQueries: 1,
				BackupStorageCount:    1,
				ContentSuccesses:      1,
			},
			wantJob:         recovery.OutcomeWarning,
			wantHistory:     recovery.ProtectionHistoryPartial,
			wantPermissions: operationaltrust.EvidencePermissionsUnknown,
			wantReason:      "pve_partial_enumeration",
		},
		{
			name: "partial provider access",
			stats: pveProtectionCollectionStats{
				NodeCount:              1,
				SuccessfulNodeQueries:  1,
				BackupStorageCount:     2,
				ContentSuccesses:       1,
				ContentFailures:        1,
				PermissionFailureCount: 1,
			},
			wantJob:         recovery.OutcomeWarning,
			wantHistory:     recovery.ProtectionHistoryPartial,
			wantPermissions: operationaltrust.EvidencePermissionsPartial,
			wantReason:      "pve_partial_provider_access",
		},
		{
			name: "provider timeout",
			stats: pveProtectionCollectionStats{
				NodeCount: 2,
			},
			wantJob:         recovery.OutcomeFailed,
			wantHistory:     recovery.ProtectionHistoryUnavailable,
			wantPermissions: operationaltrust.EvidencePermissionsUnknown,
			wantReason:      "pve_collection_unavailable",
		},
		{
			name: "provider denies every node",
			stats: pveProtectionCollectionStats{
				NodeCount:              2,
				PermissionFailureCount: 2,
			},
			wantJob:         recovery.OutcomeFailed,
			wantHistory:     recovery.ProtectionHistoryUnavailable,
			wantPermissions: operationaltrust.EvidencePermissionsDenied,
			wantReason:      "pve_provider_access_denied",
		},
		{
			name: "provider denies every backup storage",
			stats: pveProtectionCollectionStats{
				NodeCount:              1,
				SuccessfulNodeQueries:  1,
				BackupStorageCount:     2,
				ContentFailures:        2,
				PermissionFailureCount: 2,
			},
			wantJob:         recovery.OutcomeFailed,
			wantHistory:     recovery.ProtectionHistoryUnavailable,
			wantPermissions: operationaltrust.EvidencePermissionsDenied,
			wantReason:      "pve_provider_access_denied",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := buildPVEProtectionProviderObservation("pve-main", test.stats, now)
			if err != nil {
				t.Fatalf("buildPVEProtectionProviderObservation() error = %v", err)
			}
			if got.JobState != test.wantJob ||
				got.HistoryCompleteness != test.wantHistory ||
				got.Permissions != test.wantPermissions {
				t.Fatalf(
					"observation = job %q, history %q, permissions %q; want %q, %q, %q",
					got.JobState,
					got.HistoryCompleteness,
					got.Permissions,
					test.wantJob,
					test.wantHistory,
					test.wantPermissions,
				)
			}
			if test.wantReason == "" {
				if got.Evidence.Reason != nil {
					t.Fatalf("reason = %#v, want nil", got.Evidence.Reason)
				}
			} else if got.Evidence.Reason == nil || got.Evidence.Reason.Code != test.wantReason {
				t.Fatalf("reason = %#v, want %q", got.Evidence.Reason, test.wantReason)
			}
		})
	}
}

func TestIsPVEBackupPermissionError(t *testing.T) {
	t.Parallel()

	for _, err := range []error{
		errors.New("403 Forbidden"),
		errors.New("permission check failed"),
		errors.New("HTTP 401"),
	} {
		if !isPVEBackupPermissionError(err) {
			t.Fatalf("expected %q to be recognized as a permission failure", err)
		}
	}
	if isPVEBackupPermissionError(errors.New("connection timed out")) {
		t.Fatal("transient timeout must not be classified as a permission failure")
	}
}

// persistGuestIdentity runs inside the protection/backup read-state walk and
// must hand its write to the store rather than detaching its own goroutine.
// A detached write cannot be drained by Monitor.Stop, so it can land after
// shutdown and after the data directory has started being removed.
func TestPersistGuestIdentityQueuesADrainableWrite(t *testing.T) {
	dataPath := t.TempDir()
	store := config.NewGuestMetadataStore(dataPath, nil)

	persistGuestIdentity(store, "pve1:node1:100", "web-01", "qemu")

	if !store.WaitForPendingWrites(10 * time.Second) {
		t.Fatal("persistGuestIdentity write was not drainable; the call site detached its own goroutine")
	}
	if _, err := os.Stat(filepath.Join(dataPath, "guest_metadata.json")); err != nil {
		t.Fatalf("guest identity was not persisted after the drain: %v", err)
	}
	if meta := store.Get("pve1:node1:100"); meta == nil || meta.LastKnownName != "web-01" || meta.LastKnownType != "qemu" {
		t.Fatalf("guest identity not recorded: %#v", meta)
	}
}

// An unchanged identity must not queue a write at all, so a steady-state poll
// cycle does no disk work.
func TestPersistGuestIdentitySkipsUnchangedIdentities(t *testing.T) {
	dataPath := t.TempDir()
	store := config.NewGuestMetadataStore(dataPath, nil)

	persistGuestIdentity(store, "pve1:node1:101", "db-01", "lxc")
	if !store.WaitForPendingWrites(10 * time.Second) {
		t.Fatal("initial write did not drain")
	}
	firstWrite, err := os.Stat(filepath.Join(dataPath, "guest_metadata.json"))
	if err != nil {
		t.Fatalf("stat after first write: %v", err)
	}

	persistGuestIdentity(store, "pve1:node1:101", "db-01", "lxc")
	if !store.WaitForPendingWrites(10 * time.Second) {
		t.Fatal("second call did not drain")
	}
	secondWrite, err := os.Stat(filepath.Join(dataPath, "guest_metadata.json"))
	if err != nil {
		t.Fatalf("stat after second call: %v", err)
	}
	if !firstWrite.ModTime().Equal(secondWrite.ModTime()) {
		t.Error("unchanged identity triggered a redundant write")
	}
}

// A skipped/unreadable non-final log supplies no new guest observation. In
// particular, it cannot extend the retained running task's alert-intent age.
func TestPVELogRetryDoesNotRenewRetainedEvidence(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		t.Run(fmt.Sprint(cancelled), func(t *testing.T) {
			observed := time.Now().Add(-time.Hour).UTC()
			task := models.BackupTask{ID: "proof-job-100", Instance: "proof", Node: "pve1", VMID: 100, Status: "running", ObservedAt: observed}
			m := &Monitor{state: models.NewState(), vzdumpJobTaskCache: map[string]vzdumpJobTaskCacheEntry{"proof|job": {tasks: []models.BackupTask{task}}}}
			client := &unreadableJobLogClient{}
			ctx := context.Background()
			if cancelled {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			got := m.synthesizeVzdumpJobGuestTasks(ctx, "proof", client, []models.BackupTask{{ID: "parent", Node: "pve1"}}, map[string]string{"parent": "job"})
			if !reflect.DeepEqual(got, []models.BackupTask{task}) {
				t.Fatalf("retained log observation renewed or lost: %+v", got)
			}
		})
	}
}

type unreadableJobLogClient struct{ PVEClientInterface }

func (*unreadableJobLogClient) GetTaskLog(context.Context, string, string) ([]proxmox.TaskLogLine, error) {
	return nil, errors.New("fixture unavailable")
}

func TestPVERunningLogCacheKeepsActualSuccessfulReceipt(t *testing.T) {
	var failed atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api2/json/nodes":
			fmt.Fprint(w, `{"data":[{"node":"pve1","status":"online"}]}`)
		case r.URL.Path == "/api2/json/nodes/pve1/tasks":
			fmt.Fprint(w, `{"data":[{"upid":"running-job","node":"pve1","type":"vzdump","starttime":1792028280}]}`)
		case strings.HasSuffix(r.URL.Path, "/log"):
			if failed.Load() {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			fmt.Fprint(w, `{"data":[{"n":1,"t":"INFO: Starting Backup of VM 100 (qemu)"}]}`)
		default:
			t.Errorf("unexpected operation %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	client, err := proxmox.NewClient(proxmox.ClientConfig{Host: server.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture", Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	m := &Monitor{state: models.NewState()}
	m.pollBackupTasks(context.Background(), "proof", client)
	guestTask := func() models.BackupTask {
		tasks := m.state.GetSnapshot().PVEBackups.BackupTasks
		if len(tasks) != 2 {
			t.Fatalf("expected parent and guest tasks: %+v", tasks)
		}
		for _, task := range tasks {
			if task.VMID == 100 {
				return task
			}
		}
		t.Fatalf("guest100 task absent: %+v", tasks)
		return models.BackupTask{}
	}
	initial := guestTask()
	if initial.ObservedAt.IsZero() {
		t.Fatalf("initial running guest observation missing: %+v", initial)
	}
	failed.Store(true)
	m.pollBackupTasks(context.Background(), "proof", client)
	retained := guestTask()
	if !reflect.DeepEqual(retained, initial) {
		t.Fatalf("unreadable running log lost/renewed the successful receipt: %+v -> %+v", initial, retained)
	}
	failed.Store(false)
	m.pollBackupTasks(context.Background(), "proof", client)
	resumed := guestTask()
	if !resumed.ObservedAt.After(initial.ObservedAt) {
		t.Fatalf("ordinary successful running log did not resume: %+v", resumed)
	}
}

// Real HTTP snapshot readers, ordinary polling, recovery ingestion, alert
// evaluation and Monitor.GetState must agree. No QGA, guest mutation, backup,
// clock change, permission change or customer incident is exercised.
func TestSnapshotInventoryFailuresPreserveServedRecovery(t *testing.T) {
	m, recoveryManager := recoveryIngestTestMonitor(t)
	m.state = models.NewState()
	manager := alerts.NewManagerWithDataDir(t.TempDir())
	t.Cleanup(manager.Stop)
	cfg := manager.GetConfig()
	cfg.Enabled = true
	cfg.SnapshotDefaults = alerts.SnapshotAlertConfig{Enabled: true, WarningDays: 7, CriticalDays: 14}
	manager.UpdateConfig(cfg)
	m.alertManager = manager
	m.state.UpdateGuestsForInstance("pve1",
		[]models.VM{{ID: "pve1:member:100", Instance: "pve1", Node: "member", VMID: 100, Name: "database", Status: "running"}},
		[]models.Container{{ID: "pve1:member:200", Instance: "pve1", Node: "member", VMID: 200, Name: "worker", Type: "lxc", Status: "running"}})
	// Another connection and a backup of another class are outside this poll.
	birth := time.Now().UTC().Truncate(time.Second).Add(-10 * 24 * time.Hour)
	peer := models.GuestSnapshot{ID: "other-member-200-peer", Name: "peer", Instance: "other", Node: "member", VMID: 200, Type: "lxc", Time: birth}
	m.state.UpdateGuestSnapshotsForInstance("other", []models.GuestSnapshot{peer})
	store, err := recoveryManager.StoreForOrg("default")
	if err != nil {
		t.Fatal(err)
	}
	peerPoint := recoveryIngestTestPoint("pve-snapshot:other-member-200-peer", "other", birth)
	backupPoint := recoveryIngestTestPoint("pve-backup:pve1-independent", "pve1", birth)
	if err := store.UpsertPoints(context.Background(), []recovery.RecoveryPoint{peerPoint, backupPoint}); err != nil {
		t.Fatal(err)
	}
	type phase struct {
		name, body string
		status     int
		incomplete bool
	}
	phases := []phase{
		{"initial", fmt.Sprintf(`{"data":[{"name":"current"},{"name":"weekly","snaptime":%d,"description":"known worker snapshot"}]}`, birth.Unix()), 200, false},
		{"http-unavailable", `snapshot service not available`, 596, false},
		{"body-unavailable", `snapshot service not available`, 500, false},
		{"permission-denied", `fixture denied`, 403, false},
		{"null-inventory", `{"data":null}`, 200, false},
		{"duplicate-inventory", `{"data":[{"name":"weekly"}],"data":[]}`, 200, false},
		{"incomplete-empty-prefix", `{"data":[]}`, 200, true},
		{"recovered", fmt.Sprintf(`{"data":[{"name":"weekly","snaptime":%d,"description":"fresh worker snapshot"}]}`, time.Now().Add(-time.Hour).Unix()), 200, false},
		{"empty-worker", `{"data":[]}`, 200, false},
		{"empty-all", `{"data":[]}`, 200, false},
	}
	var current, reads, unexpected atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		index := int(current.Load())
		switch r.URL.Path {
		case "/api2/json/nodes/member/lxc/200/snapshot":
			reads.Add(1)
			p := phases[index]
			if p.incomplete {
				w.Header().Set("Content-Length", fmt.Sprint(len(p.body)+100))
			}
			w.WriteHeader(p.status)
			fmt.Fprint(w, p.body)
		case "/api2/json/nodes/member/qemu/100/snapshot":
			reads.Add(1)
			if phases[index].name == "empty-all" {
				fmt.Fprint(w, `{"data":[]}`)
			} else {
				fmt.Fprintf(w, `{"data":[{"name":"weekly","snaptime":%d,"description":"fresh VM inventory %d"}]}`, birth.Unix(), index)
			}
		case "/api2/json/nodes/member/storage":
			fmt.Fprint(w, `{"data":[]}`)
		default:
			unexpected.Add(1)
			http.Error(w, "unexpected fixture request", http.StatusNotFound)
		}
	}))
	defer server.Close()
	client, err := proxmox.NewClient(proxmox.ClientConfig{Host: server.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture"})
	if err != nil {
		t.Fatal(err)
	}
	waitForIngest := func() {
		t.Helper()
		deadline := time.Now().Add(10 * time.Second)
		for {
			m.recoveryIngestMu.Lock()
			pending := m.recoveryIngestRunning
			m.recoveryIngestMu.Unlock()
			if !pending {
				return
			}
			if time.Now().After(deadline) {
				t.Fatal("snapshot recovery ingest did not settle")
			}
			time.Sleep(time.Millisecond)
		}
	}
	t.Cleanup(waitForIngest)
	originalAlerts := make(map[string]string)
	for index, p := range phases {
		t.Run(p.name, func(t *testing.T) {
			current.Store(int32(index))
			m.pollGuestSnapshots(context.Background(), "pve1", client)
			waitForIngest()
			state := m.GetState()
			wantSnapshots, wantAlerts := 3, 2
			if p.name == "recovered" {
				wantAlerts = 1
			}
			if p.name == "empty-worker" {
				wantSnapshots, wantAlerts = 2, 1
			}
			if p.name == "empty-all" {
				wantSnapshots, wantAlerts = 1, 0
			}
			if len(state.PVEBackups.GuestSnapshots) != wantSnapshots || len(state.ActiveAlerts) != wantAlerts {
				t.Fatalf("unknown inventory changed snapshot/alert truth: snapshots=%+v alerts=%+v", state.PVEBackups.GuestSnapshots, state.ActiveAlerts)
			}
			for _, alert := range state.ActiveAlerts {
				if alert.Level != "warning" {
					t.Fatalf("unexpected alert level: %+v", alert)
				}
				if index == 0 {
					originalAlerts[alert.ResourceID] = alert.ID
				} else if originalAlerts[alert.ResourceID] != alert.ID {
					t.Fatalf("failed collection replaced alert identity: %+v", alert)
				}
			}
			foundPeer, foundWorker := false, false
			for _, snap := range state.PVEBackups.GuestSnapshots {
				if snap.Instance == "other" {
					foundPeer = reflect.DeepEqual(snap, peer)
				} else if snap.Type == "qemu" && snap.Description != fmt.Sprintf("fresh VM inventory %d", index) {
					t.Fatal("failed peer prevented independent VM refresh")
				} else if snap.Type == "lxc" {
					foundWorker = true
					if index < 7 && (!snap.Time.Equal(birth) || snap.Description != "known worker snapshot") {
						t.Fatalf("failed read renewed/replaced known snapshot: %+v", snap)
					}
				}
			}
			if !foundPeer || foundWorker != (index < 8) {
				t.Fatal("snapshot deletion escaped its successful guest/connection scope")
			}
			ids := listRecoveryPointIDs(t, recoveryManager)
			if len(ids) != wantSnapshots+1 || !ids[peerPoint.ID] || !ids[backupPoint.ID] || ids["pve-snapshot:pve1-member-200-weekly"] != (index < 8) {
				t.Fatalf("recovery inventory diverged from snapshots: %+v", ids)
			}
			if reads.Load() != int32(2*(index+1)) || unexpected.Load() != 0 {
				t.Fatalf("unexpected command or replay: reads=%d unexpected=%d", reads.Load(), unexpected.Load())
			}
			// Retain real served projections for the consuming surface, not
			// hand-written alert text or synthetic success in the browser.
			wire, err := json.Marshal(struct {
				Phase string               `json:"phase"`
				State models.StateSnapshot `json:"state"`
			}{p.name, state})
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("SNAPSHOT_INVENTORY_PROJECTION=%s", wire)
		})
	}
}
