package monitoring

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

var backupProjectionTime = time.Date(2026, 10, 18, 1, 0, 0, 0, time.UTC)

// Hold the canonical guest generation and backup inputs fixed. Only unrelated
// estate metadata grows; constructing these fixtures is outside measurement.
func backupReadProjectionMonitor(unrelated int) *Monitor {
	m := snapshotProjectionMonitor(unrelated, 0)
	m.state.PVEBackups = models.PVEBackups{
		BackupTasks: []models.BackupTask{{ID: "running", Instance: "site", Node: "member", VMID: 100, Status: "running", StartTime: backupProjectionTime}},
		StorageBackups: []models.StorageBackup{
			{ID: "held", Instance: "site", Node: "member", StorageNode: "offline", Storage: "archive", VMID: 100, Type: "qemu", Time: backupProjectionTime.Add(-time.Hour), Size: 4000, Protected: true, Notes: "retained"},
			{ID: "foreign", Instance: "other", Node: "member", StorageNode: "offline", Storage: "archive", VMID: 100, Type: "qemu", Time: backupProjectionTime.Add(-2 * time.Hour), Size: 8000},
		},
	}
	for _, namespace := range []string{"", "team"} {
		for i := 0; i < pbsBackupSnapshotsPerGroupLimit+2; i++ {
			m.state.PBSBackups = append(m.state.PBSBackups, models.PBSBackup{
				ID: fmt.Sprintf("pbs-%s-%02d", namespace, i), Instance: "pbs", Datastore: "archive", Namespace: namespace, BackupType: "vm", VMID: "100",
				BackupTime: backupProjectionTime.Add(time.Duration(i) * time.Minute), Size: int64(1000 + i), Protected: true, Verified: i%2 == 0,
				InProgress: i%2 != 0, WriteActivityObserved: true, WriteActive: i%2 != 0,
				Files: []string{"qemu-server.conf.blob", "index.json.blob"}, VerificationRaw: map[string]interface{}{"state": "ok"}, Comment: "fixture", Owner: "fixture@pbs",
			})
		}
	}
	m.state.PBSBackups = append(m.state.PBSBackups, models.PBSBackup{ID: "other-pbs", Instance: "other", Datastore: "archive", BackupType: "vm", VMID: "100", BackupTime: backupProjectionTime})
	// Consumers must use the primary state, never this separately cached branch.
	m.state.Backups = models.Backups{PBS: []models.PBSBackup{{ID: "stale-aggregate"}}}
	return m
}

func pollOfflineBackupProjection(m *Monitor) {
	// An unavailable node is not successful emptiness. This ordinary branch
	// returns without rebuilding canonical state or making a provider request.
	m.pollStorageBackupsWithNodes(context.Background(), "site", &fakeSnapshotClient{},
		[]proxmox.Node{{Node: "offline"}}, map[string]string{"offline": "offline"})
	awaitBackupProjectionIngest(m)
}

func awaitBackupProjectionIngest(m *Monitor) {
	// There is no recovery manager in this fixture: the ordinary asynchronous
	// sink returns without storage I/O. Drain it so allocations from different
	// samples cannot overlap or choose different queue/coalescing paths.
	for {
		m.recoveryIngestMu.Lock()
		running := m.recoveryIngestRunning
		m.recoveryIngestMu.Unlock()
		if !running {
			return
		}
		runtime.Gosched()
	}
}

func TestBackupReadProjectionDoesNotScaleWithUnrelatedEstate(t *testing.T) {
	for _, reader := range []string{"pve-offline", "pbs-cache"} {
		t.Run(reader, func(t *testing.T) {
			small, large := backupReadProjectionMonitor(0), backupReadProjectionMonitor(1000)
			measure := func(m *Monitor) float64 {
				return testing.AllocsPerRun(10, func() {
					if reader == "pbs-cache" {
						m.buildPBSBackupCache("pbs")
					} else {
						pollOfflineBackupProjection(m)
					}
				})
			}
			base, grown := measure(small), measure(large)
			t.Logf("BACKUP_READ_PROJECTION reader=%s base_allocs=%.0f estate_allocs=%.0f", reader, base, grown)
			if grown > base+2 {
				t.Errorf("unconsumed estate increased %s allocations: %.0f -> %.0f", reader, base, grown)
			}
			if !reflect.DeepEqual(small.state.GetBackups(), large.state.GetBackups()) ||
				!reflect.DeepEqual(small.buildPBSBackupCache("pbs"), large.buildPBSBackupCache("pbs")) {
				t.Fatal("unrelated metadata changed retained or cached backup evidence")
			}
		})
	}
}

func TestBackupReadProjectionPreservesDetachedPBSCache(t *testing.T) {
	m := backupReadProjectionMonitor(1000)
	before := m.state.GetPBSBackups()
	cache := m.buildPBSBackupCache("pbs")
	if len(cache) != 2 {
		t.Fatalf("cache groups = %d, want two independent namespaces", len(cache))
	}
	for key, entry := range cache {
		var want []models.PBSBackup
		for _, backup := range before {
			if backup.Instance == "pbs" && backup.Namespace == key.namespace {
				want = append(want, backup)
			}
		}
		sortPBSBackupsByLatest(want)
		want = want[:pbsBackupSnapshotsPerGroupLimit]
		if key.datastore != "archive" || key.backupType != "vm" || key.backupID != "100" ||
			!entry.latest.Equal(want[0].BackupTime) || !reflect.DeepEqual(entry.snapshots, want) {
			t.Fatal("cache changed primary fields, namespace identity, newest-first order or retention limit")
		}
		entry.snapshots[0].Files[0] = "reader-only"
		entry.snapshots[0].VerificationRaw.(map[string]interface{})["state"] = "reader-only"
		entry.snapshots[0].Owner = "reader-only"
	}
	if !reflect.DeepEqual(before, m.state.GetPBSBackups()) {
		t.Fatal("cache mutation reached primary backup evidence")
	}
	for _, state := range []*models.State{models.NewState(), {}} {
		if got := (&Monitor{state: state}).buildPBSBackupCache("pbs"); len(got) != 0 {
			t.Fatal("empty primary backup state did not produce an empty cache")
		}
	}
}

type backupReadProjectionClient struct {
	fakeSnapshotClient
	calls []string
}

func (c *backupReadProjectionClient) GetStorage(_ context.Context, node string) ([]proxmox.Storage, error) {
	c.calls = append(c.calls, "storage/"+node)
	return []proxmox.Storage{{Storage: "live", Content: "backup", Type: "dir", Enabled: 1, Active: 1}}, nil
}

func (c *backupReadProjectionClient) GetStorageContent(_ context.Context, node, storage string) ([]proxmox.StorageContent, error) {
	c.calls = append(c.calls, "content/"+node+"/"+storage)
	return []proxmox.StorageContent{
		{Volid: "live:backup/vzdump-qemu-100-current.vma.zst", VMID: 100, Content: "backup", CTime: backupProjectionTime.Unix(), Size: 2000, Format: "vma"},
		{Volid: "live:backup/vzdump-qemu-100-previous.vma.zst", VMID: 100, Content: "backup", CTime: backupProjectionTime.Add(-time.Hour).Unix(), Size: 3000, Format: "vma"},
	}, nil
}

func TestBackupReadProjectionKeepsLiveTaskAndHeldArchiveEvidence(t *testing.T) {
	var baseline []models.StorageBackup
	for _, unrelated := range []int{0, 1000} {
		m := backupReadProjectionMonitor(unrelated)
		before := m.state.GetPVEBackups()
		client := &backupReadProjectionClient{}
		m.pollStorageBackupsWithNodes(context.Background(), "site", client,
			[]proxmox.Node{{Node: "member"}, {Node: "offline"}}, map[string]string{"member": "online", "offline": "offline"})
		awaitBackupProjectionIngest(m)
		if !reflect.DeepEqual(client.calls, []string{"storage/member", "content/member/live"}) {
			t.Fatalf("request set changed: %v", client.calls)
		}
		got := m.state.GetPVEBackups()
		if !reflect.DeepEqual(got.BackupTasks, before.BackupTasks) || len(got.StorageBackups) != 4 {
			t.Fatal("storage poll altered tasks or lost a held/independent archive")
		}
		byID := make(map[string]models.StorageBackup)
		for _, backup := range got.StorageBackups {
			byID[backup.ID] = backup
		}
		for _, held := range before.StorageBackups {
			if !reflect.DeepEqual(byID[held.ID], held) {
				t.Fatalf("held archive %s changed", held.ID)
			}
		}
		scope := pveBackupStorageScope{storage: "live", node: "member"}
		currentID := pveStorageBackupID("site", scope, "live:backup/vzdump-qemu-100-current.vma.zst")
		previousID := pveStorageBackupID("site", scope, "live:backup/vzdump-qemu-100-previous.vma.zst")
		current, previous := byID[currentID], byID[previousID]
		if !current.InProgress || previous.InProgress || current.Size != 2000 || previous.Size != 3000 || current.Node != "member" {
			t.Fatal("current task's partial archive became completed backup evidence, or an older archive changed")
		}
		sort.Slice(got.StorageBackups, func(i, j int) bool { return got.StorageBackups[i].ID < got.StorageBackups[j].ID })
		if unrelated == 0 {
			baseline = got.StorageBackups
		} else if !reflect.DeepEqual(baseline, got.StorageBackups) {
			t.Fatal("unrelated estate changed published storage-backup fields")
		}
	}
}

var backupProjectionCacheSink map[pbsBackupGroupKey]cachedPBSGroup

func BenchmarkBackupReadProjection(b *testing.B) {
	for _, reader := range []string{"pve-offline", "pbs-cache"} {
		for _, unrelated := range []int{0, 1000} {
			b.Run(fmt.Sprintf("%s/estate=%d", reader, unrelated), func(b *testing.B) {
				m := backupReadProjectionMonitor(unrelated)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if reader == "pbs-cache" {
						backupProjectionCacheSink = m.buildPBSBackupCache("pbs")
					} else {
						pollOfflineBackupProjection(m)
					}
				}
			})
		}
	}
}
