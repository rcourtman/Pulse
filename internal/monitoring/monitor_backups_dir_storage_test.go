package monitoring

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	pveapi "github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// dirStorageNamedPBSClient serves a dir-type storage whose NAME starts with
// "pbs-" alongside a real pbs-type storage. Only the latter may be treated as
// PBS-backed when a direct PBS connection exists (#1592).
type dirStorageNamedPBSClient struct {
	mockPVEClientExtra
}

func (c *dirStorageNamedPBSClient) GetStorage(ctx context.Context, node string) ([]pveapi.Storage, error) {
	return []pveapi.Storage{
		{Storage: "pbs-backup", Content: "backup", Type: "dir", Enabled: 1, Active: 1},
		{Storage: "pbs-real", Content: "backup", Type: "pbs", Enabled: 1, Active: 1},
	}, nil
}

func (c *dirStorageNamedPBSClient) GetStorageContent(ctx context.Context, node, storage string) ([]pveapi.StorageContent, error) {
	switch storage {
	case "pbs-backup":
		return []pveapi.StorageContent{{
			Volid:   "pbs-backup:backup/vzdump-qemu-106-2026_07_17-01_00_00.vma.zst",
			VMID:    106,
			Size:    2048,
			CTime:   time.Date(2026, 7, 17, 1, 0, 0, 0, time.UTC).Unix(),
			Content: "backup",
		}}, nil
	case "pbs-real":
		return []pveapi.StorageContent{{
			Volid:   "pbs-real:backup/vm/107/2026-07-17T01:00:00Z",
			VMID:    107,
			Size:    2048,
			CTime:   time.Date(2026, 7, 17, 1, 0, 0, 0, time.UTC).Unix(),
			Content: "backup",
			Format:  "pbs-vm",
		}}, nil
	}
	return nil, nil
}

func TestPollStorageBackups_KeepsDirStorageNamedLikePBS(t *testing.T) {
	m := &Monitor{
		state: models.NewState(),
		config: &config.Config{
			PBSInstances: []config.PBSInstance{{Name: "pbs-1", Host: "https://pbs.example:8007"}},
		},
	}

	client := &dirStorageNamedPBSClient{}
	nodes := []pveapi.Node{{Node: "node1", Status: "online"}, {Node: "node2", Status: "online"}}

	m.pollStorageBackupsWithNodes(context.Background(), "pve1", client, nodes, map[string]string{"node1": "online", "node2": "online"})

	backups := m.state.GetSnapshot().PVEBackups.StorageBackups
	if len(backups) != 2 {
		t.Fatalf("expected each independent dir-storage copy to survive, got %+v", backups)
	}
	if backups[0].ID == backups[1].ID || backups[0].StorageNode == backups[1].StorageNode {
		t.Fatalf("local disks named like PBS must have independent identities: %+v", backups)
	}
	for _, backup := range backups {
		if backup.Storage != "pbs-backup" || backup.VMID != 106 {
			t.Fatalf("expected only vzdump from dir storage named pbs-backup, got %+v", backup)
		}
	}
}

// pbsPartialFailureClient serves two pbs-type storages and can be told to
// fail one storage's content query, standing in for a partial poll failure.
type pbsPartialFailureClient struct {
	mockPVEClientExtra
	snapshotTime time.Time
	failStorage  string
}

func (c *pbsPartialFailureClient) GetStorage(ctx context.Context, node string) ([]pveapi.Storage, error) {
	return []pveapi.Storage{
		{Storage: "pbs-one", Content: "backup", Type: "pbs", Enabled: 1, Active: 1},
		{Storage: "pbs-two", Content: "backup", Type: "pbs", Enabled: 1, Active: 1},
	}, nil
}

func (c *pbsPartialFailureClient) GetStorageContent(ctx context.Context, node, storage string) ([]pveapi.StorageContent, error) {
	if storage == c.failStorage {
		return nil, fmt.Errorf("500 internal error")
	}
	vmid := 173
	if storage == "pbs-two" {
		vmid = 174
	}
	return []pveapi.StorageContent{{
		Volid:   fmt.Sprintf("%s:backup/vm/%d/2026-07-27T01:00:00Z", storage, vmid),
		VMID:    vmid,
		Size:    2048,
		CTime:   c.snapshotTime.Unix(),
		Content: "backup",
		Format:  "pbs-vm",
	}}, nil
}

// TestPollStorageBackups_Issue1639PreservesConfirmationsOnPartialFailure
// verifies that a storage whose content query fails keeps its previous PBS
// guest confirmations instead of having them evicted by the partial set.
// Losing them flips collision-VMID attribution from cycle to cycle.
func TestPollStorageBackups_Issue1639PreservesConfirmationsOnPartialFailure(t *testing.T) {
	snapshotTime := time.Date(2026, 7, 27, 1, 0, 0, 0, time.UTC)

	m := &Monitor{
		state: models.NewState(),
		config: &config.Config{
			PBSInstances: []config.PBSInstance{{Name: "pbs-1", Host: "https://pbs.example:8007"}},
		},
	}
	nodes := []pveapi.Node{{Node: "node-a", Status: "online"}}
	online := map[string]string{"node-a": "online"}

	// Healthy cycle: both storages list one snapshot each.
	m.pollStorageBackupsWithNodes(context.Background(), "cluster-a",
		&pbsPartialFailureClient{snapshotTime: snapshotTime}, nodes, online)

	healthy := m.state.PBSGuestConfirmationsForInstance("cluster-a")
	if len(healthy) != 2 {
		t.Fatalf("expected both storages to contribute confirmations, got %+v", healthy)
	}

	// Degraded cycle: pbs-one's content query fails. Its evidence must
	// survive, and pbs-two's must be refreshed as usual.
	m.pollStorageBackupsWithNodes(context.Background(), "cluster-a",
		&pbsPartialFailureClient{snapshotTime: snapshotTime, failStorage: "pbs-one"}, nodes, online)

	degraded := m.state.PBSGuestConfirmationsForInstance("cluster-a")
	if len(degraded) != 2 {
		t.Fatalf("partial failure evicted confirmation evidence, got %+v", degraded)
	}
	byStorage := map[string]models.PBSGuestConfirmation{}
	for _, confirmation := range degraded {
		byStorage[confirmation.Storage] = confirmation
	}
	if got, ok := byStorage["pbs-one"]; !ok || got.VMID != 173 {
		t.Fatalf("pbs-one confirmation not preserved across the failed query, got %+v", degraded)
	}
	if got, ok := byStorage["pbs-two"]; !ok || got.VMID != 174 {
		t.Fatalf("pbs-two confirmation missing after successful query, got %+v", degraded)
	}
}

// pbsCollisionStorageClient serves one pbs-type storage listing a single
// snapshot for VMID 173, standing in for cluster-a's own PBS storage view.
type pbsCollisionStorageClient struct {
	mockPVEClientExtra
	snapshotTime time.Time
}

func (c *pbsCollisionStorageClient) GetStorage(ctx context.Context, node string) ([]pveapi.Storage, error) {
	return []pveapi.Storage{
		{Storage: "pbs-store", Content: "backup", Type: "pbs", Enabled: 1, Active: 1},
	}, nil
}

func (c *pbsCollisionStorageClient) GetStorageContent(ctx context.Context, node, storage string) ([]pveapi.StorageContent, error) {
	return []pveapi.StorageContent{{
		Volid:   "pbs-store:backup/vm/173/2026-07-27T01:00:00Z",
		VMID:    173,
		Size:    2048,
		CTime:   c.snapshotTime.Unix(),
		Content: "backup",
		Format:  "pbs-vm",
	}}, nil
}

// TestPollStorageBackups_Issue1639HarvestsPBSGuestConfirmations verifies
// that snapshots skipped in favor of the direct PBS connection still leave
// PVE-side attribution evidence: the cluster whose storage listed a
// snapshot gets it as LastBackup even though the VMID exists on another
// cluster too, and the other cluster's unconfirmed snapshot is not guessed.
func TestPollStorageBackups_Issue1639HarvestsPBSGuestConfirmations(t *testing.T) {
	timeA := time.Date(2026, 7, 27, 1, 0, 0, 0, time.UTC)
	timeB := time.Date(2026, 7, 27, 2, 0, 0, 0, time.UTC)

	state := models.NewState()
	state.UpdateVMs([]models.VM{
		{VMID: 173, Name: "web", Instance: "cluster-a", Node: "node-a", Status: "running"},
		{VMID: 173, Name: "web", Instance: "cluster-b", Node: "node-b", Status: "running"},
	})
	state.UpdatePBSBackups("pbs-1", []models.PBSBackup{
		{ID: "a-173", VMID: "173", BackupType: "vm", BackupTime: timeA, Instance: "pbs-1", Datastore: "backups", Owner: "shared@pbs!token"},
		{ID: "b-173", VMID: "173", BackupType: "vm", BackupTime: timeB, Instance: "pbs-1", Datastore: "backups", Owner: "shared@pbs!token"},
	})

	m := &Monitor{
		state: state,
		config: &config.Config{
			PBSInstances: []config.PBSInstance{{Name: "pbs-1", Host: "https://pbs.example:8007"}},
		},
	}

	client := &pbsCollisionStorageClient{snapshotTime: timeA}
	nodes := []pveapi.Node{{Node: "node-a", Status: "online"}}

	m.pollStorageBackupsWithNodes(context.Background(), "cluster-a", client, nodes, map[string]string{"node-a": "online"})

	snapshot := m.state.GetSnapshot()
	if len(snapshot.PVEBackups.StorageBackups) != 0 {
		t.Fatalf("pbs-type storage contents must stay out of StorageBackups, got %+v", snapshot.PVEBackups.StorageBackups)
	}
	for _, vm := range snapshot.VMs {
		switch vm.Instance {
		case "cluster-a":
			if !vm.LastBackup.Equal(timeA) {
				t.Errorf("cluster-a VM 173 LastBackup = %v, want confirmed snapshot %v", vm.LastBackup, timeA)
			}
		case "cluster-b":
			if !vm.LastBackup.IsZero() {
				t.Errorf("cluster-b VM 173 LastBackup = %v, want zero (its snapshot is unconfirmed)", vm.LastBackup)
			}
		}
	}
}

func TestPollPVEBackupsAsyncUsesRuntimeContextIndependentOfCycle(t *testing.T) {
	runtimeCtx, cancelRuntime := context.WithCancel(context.Background())
	cancelRuntime()
	m := &Monitor{
		state:             models.NewState(),
		runtimeCtx:        runtimeCtx,
		lastPVEBackupPoll: make(map[string]time.Time),
		config: &config.Config{
			EnableBackupPolling: true,
			BackupPollingCycles: 1,
		},
	}
	m.pollPVEBackupsAsync(
		"cluster-a",
		&config.PVEInstance{MonitorBackups: true},
		&mockPVEClientExtra{},
		nil,
		map[string]string{},
	)
	m.mu.RLock()
	scheduledAt := m.lastPVEBackupPoll["cluster-a"]
	m.mu.RUnlock()
	if scheduledAt.IsZero() {
		t.Fatal("backup polling was not scheduled on the monitor runtime context")
	}
}

// A task running on node-b cannot be writing a file on node-a's independent
// local disk, even when both paths and VMIDs are identical. Shared views and
// missing task origins remain conservative: a peer may be writing that file.
func TestPVEStorageRunningArchiveOrigin(t *testing.T) {
	for _, tc := range []struct {
		name, storageType, taskNode string
		shared, otherInstance       bool
		ended, failed, oldArchive   bool
		earlierPeer                 bool
		want                        map[string]bool
	}{
		{name: "writer-a", taskNode: "node-a", want: map[string]bool{"node-a": true, "node-b": false}},
		{name: "writer-b", taskNode: "node-b", want: map[string]bool{"node-a": false, "node-b": true}},
		{name: "writer-elsewhere", taskNode: "node-c", want: map[string]bool{"node-a": false, "node-b": false}},
		{name: "separate-node-windows", taskNode: "node-a", earlierPeer: true, want: map[string]bool{"node-a": false, "node-b": true}},
		{name: "unknown-writer", want: map[string]bool{"node-a": true, "node-b": true}},
		{name: "shared-flag", shared: true, taskNode: "node-b", want: map[string]bool{"": true}},
		{name: "nfs-peer-view", storageType: "nfs", taskNode: "node-b", want: map[string]bool{"": true}},
		{name: "cifs-peer-view", storageType: "cifs", taskNode: "node-b", want: map[string]bool{"": true}},
		{name: "pbs-peer-view", storageType: "pbs", taskNode: "node-b", want: map[string]bool{"": true}},
		{name: "other-connection", taskNode: "node-a", otherInstance: true, want: map[string]bool{"node-a": false, "node-b": false}},
		{name: "ended-task", taskNode: "node-a", ended: true, want: map[string]bool{"node-a": false, "node-b": false}},
		{name: "failed-task", taskNode: "node-a", failed: true, want: map[string]bool{"node-a": false, "node-b": false}},
		{name: "archive-before-window", taskNode: "node-a", oldArchive: true, want: map[string]bool{"node-a": false, "node-b": false}},
	} {
		for _, reverse := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/reverse=%t", tc.name, reverse), func(t *testing.T) {
				f, client := localArchiveIdentityFixture(t, "qemu")
				now := time.Now().UTC().Truncate(time.Second)
				task := models.BackupTask{ID: "running", Instance: "cluster", Node: tc.taskNode, VMID: 101,
					Type: "vzdump", Status: "running", StartTime: now.Add(-time.Minute), ObservedAt: now}
				// Inside the existing clock-skew window, including a file just
				// older than the task. The change must not widen that window.
				f.client.at = task.StartTime.Add(-time.Minute)
				if tc.oldArchive {
					f.client.at = task.StartTime.Add(-runningVzdumpClockSkew - time.Second)
				}
				if tc.otherInstance {
					task.Instance = "other"
				}
				if tc.ended {
					task.EndTime = now
				}
				if tc.failed {
					task.Status = "error"
				}
				client.shared = tc.shared
				if tc.storageType != "" {
					client.storageType = tc.storageType
				}
				if reverse {
					f.nodes[0], f.nodes[1] = f.nodes[1], f.nodes[0]
				}
				tasks := []models.BackupTask{task}
				if tc.earlierPeer {
					peer := task
					peer.ID, peer.Node, peer.StartTime = "peer", "node-b", now.Add(-time.Hour)
					tasks = append(tasks, peer)
					f.client.at = now.Add(-10 * time.Minute)
				}
				f.m.state.UpdateBackupTasksForInstance(task.Instance, tasks)
				f.m.pollStorageBackupsWithNodes(context.Background(), "cluster", client, f.nodes, f.online)
				f.wait(t)
				rows := f.m.state.GetSnapshot().PVEBackups.StorageBackups
				if len(rows) != len(tc.want) {
					t.Fatalf("archive count=%d want=%d", len(rows), len(tc.want))
				}
				for _, row := range rows {
					want, ok := tc.want[row.StorageNode]
					if !ok || row.InProgress != want {
						t.Errorf("disk=%q partial=%t want=%t (known=%t)", row.StorageNode, row.InProgress, want, ok)
					}
				}
			})
		}
	}
}

// Drive the actual poller, live guest dates, asynchronous recovery ingestion
// and canonical posture through a running task, read failure and ordinary
// completion. No guest, backup operation, schema or data repair is involved.
func TestPVEStorageRunningArchiveOriginLifecycle(t *testing.T) {
	for _, kind := range []string{"qemu", "lxc"} {
		t.Run(kind, func(t *testing.T) {
			f, client := localArchiveIdentityFixture(t, kind)
			now := time.Now().UTC().Truncate(time.Second)
			f.client.at = now.Add(-time.Minute)
			task := models.BackupTask{ID: "running", Instance: "cluster", Node: "node-b", VMID: 101,
				Type: "vzdump", Status: "running", StartTime: now.Add(-10 * time.Minute), ObservedAt: now}
			frames := map[string]any{}
			var idA, idB string
			for _, phase := range []string{"running", "partial-read", "completed"} {
				if phase == "partial-read" {
					f.client.contentErr = fmt.Errorf("503 unavailable")
				}
				if phase == "completed" {
					f.client.contentErr = nil
					task.Status, task.EndTime = "OK", now
				}
				f.m.state.UpdateBackupTasksForInstance("cluster", []models.BackupTask{task})
				f.m.pollStorageBackupsWithNodes(context.Background(), "cluster", client, f.nodes, f.online)
				f.wait(t)
				snap, points, postures := f.m.state.GetSnapshot(), f.points(t), f.postures(t)
				if len(snap.PVEBackups.StorageBackups) != 2 || len(points) != 2 {
					t.Fatal("independent archive lost from live inventory or recovery")
				}
				for _, row := range snap.PVEBackups.StorageBackups {
					wantPartial := row.StorageNode == "node-b" && phase != "completed"
					point := points["pve-backup:"+row.ID]
					if row.InProgress != wantPartial || point.SubjectResourceID != f.ids[0] || row.Node != "node-b" ||
						row.Type != kind || !row.Time.Equal(f.client.at) || row.VMID != 101 || row.Storage != "local" {
						t.Fatalf("phase=%s wrong archive facts: row=%+v point=%+v", phase, row, point)
					}
					if wantPartial {
						if point.Outcome != recovery.OutcomeRunning || point.CompletedAt != nil {
							t.Fatal("the writer's partial output claimed completion")
						}
					} else if point.Outcome != recovery.OutcomeSuccess || point.CompletedAt == nil || !point.CompletedAt.Equal(row.Time) {
						t.Fatal("the independent completed archive lost its completion")
					}
					if row.StorageNode == "node-a" {
						if idA != "" && idA != row.ID {
							t.Fatal("task state renamed independent archive")
						}
						idA = row.ID
					} else {
						if idB != "" && idB != row.ID {
							t.Fatal("task state renamed writer archive")
						}
						idB = row.ID
					}
				}
				last, running := time.Time{}, false
				if kind == "qemu" {
					last, running = snap.VMs[0].LastBackup, snap.VMs[0].BackupInProgress
				} else {
					last, running = snap.Containers[0].LastBackup, snap.Containers[0].BackupInProgress
				}
				if !last.Equal(f.client.at) || running != (phase != "completed") {
					t.Fatalf("phase=%s guest completion/running=%s/%t", phase, last, running)
				}
				posture := postures[0]
				if posture.LastSuccessfulPointAt == nil || !posture.LastSuccessfulPointAt.Equal(f.client.at) ||
					(phase != "partial-read" && posture.State != recovery.ProtectionStateProtected) ||
					(phase == "partial-read" && posture.State == recovery.ProtectionStateProtected) {
					t.Fatalf("phase=%s wrong scoped protection: %+v", phase, posture)
				}
				frames[phase] = map[string]any{"pve": map[string]any{"data": snap.PVEBackups}, "postures": map[string]any{"data": postures, "policy": recovery.DefaultProtectionPosturePolicy.Payload()}, "vms": snap.VMs, "containers": snap.Containers}
			}
			if os.Getenv("PULSE_RUNNING_ARCHIVE_FRAMES") == "1" {
				hashes := map[string]string{}
				for _, path := range []string{"monitor_backups.go", "../models/models.go", "../recovery/mapper/proxmox/mapper.go"} {
					b, err := os.ReadFile(path)
					if err != nil {
						t.Fatal(err)
					}
					hashes[path] = fmt.Sprintf("%x", sha256.Sum256(b))
				}
				b, err := json.Marshal(map[string]any{"kind": kind, "runtime_sha256": hashes, "resource_ids": f.ids, "frames": frames})
				if err != nil {
					t.Fatal(err)
				}
				t.Log("RUNNING_ARCHIVE_FRAMES=" + string(b))
			}
		})
	}
}
