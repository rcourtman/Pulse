package monitoring

import (
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/rs/zerolog/log"
)

// These fakes only read synthetic provider inventory; they perform no guest
// commands, backups, database operations or writes to an external service.
type backupPollingUnreadClient struct{ PVEClientInterface }

var errBackupPollingUnread = errors.New("synthetic inventory read unavailable")

func (backupPollingUnreadClient) GetVMSnapshots(context.Context, string, int) ([]proxmox.Snapshot, error) {
	return nil, errBackupPollingUnread
}

func (backupPollingUnreadClient) GetStorage(context.Context, string) ([]proxmox.Storage, error) {
	return nil, nil
}

func backupPollingReadFixture(unrelated int) *Monitor {
	at := time.Date(2026, 11, 7, 0, 0, 0, 0, time.UTC)
	state := models.NewState()
	state.PVEBackups = models.PVEBackups{
		BackupTasks: []models.BackupTask{{ID: "running", Instance: "site", Node: "node", VMID: 100, Status: "running", StartTime: at}},
		StorageBackups: []models.StorageBackup{
			{ID: "archive", Instance: "site", Storage: "local", StorageNode: "node", Node: "node", VMID: 100, Time: at, Protected: true},
			{ID: "peer-archive", Instance: "peer", Storage: "local", StorageNode: "node", Node: "node", VMID: 100, Time: at},
		},
		GuestSnapshots: []models.GuestSnapshot{
			{ID: "site-node-100-old", Instance: "site", Node: "node", VMID: 100, Type: "qemu", Name: "old", Time: at, Description: "accepted", SizeBytes: 4096},
			{ID: "peer-node-100-old", Instance: "peer", Node: "node", VMID: 100, Type: "qemu", Name: "peer-old", Time: at},
		},
	}
	state.PBSBackups = []models.PBSBackup{{ID: "pbs", Instance: "pbs", Datastore: "archive", Namespace: "team", BackupType: "vm", VMID: "100", BackupTime: at,
		Protected: true, InProgress: true, WriteActivityObserved: true, WriteActive: true, CarriedOver: true,
		Files: []string{"index.json.blob"}, VerificationRaw: map[string]interface{}{"state": "ok"}, Owner: "fixture@pbs"}}
	// The separately cached aggregate must never supply producer evidence.
	state.Backups = models.Backups{PVE: models.PVEBackups{StorageBackups: []models.StorageBackup{{ID: "stale-aggregate"}}}, PBS: []models.PBSBackup{{ID: "stale-aggregate"}}}
	state.VMs = []models.VM{{ID: "site-node-100", Instance: "site", Node: "node", VMID: 100}}
	cluster := models.KubernetesCluster{ID: "unrelated"}
	for i := 0; i < unrelated; i++ {
		id := fmt.Sprint(i)
		state.VMs = append(state.VMs, models.VM{ID: "unrelated-" + id, Instance: "unrelated", VMID: i + 1000, Disks: []models.Disk{{Mountpoint: "/"}}})
		cluster.ConfigMaps = append(cluster.ConfigMaps, models.KubernetesConfigMap{UID: id, Labels: map[string]string{"fixture": "only"}, DataKeys: []string{"key"}})
	}
	if unrelated > 0 {
		state.KubernetesClusters = []models.KubernetesCluster{cluster}
	}
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestResources([]unifiedresources.Resource{{ID: "canonical-site-100", Type: unifiedresources.ResourceTypeVM, Name: "guest", Status: unifiedresources.StatusOnline,
		Proxmox: &unifiedresources.ProxmoxData{Instance: "site", NodeName: "node", VMID: 100}}})
	return &Monitor{state: state, resourceStore: unifiedresources.NewMonitorAdapter(registry)}
}

var backupPollingReadResult any

func backupPollingReadCases() []struct {
	name string
	call func(*Monitor)
} {
	return []struct {
		name string
		call func(*Monitor)
	}{
		{"pbs-cache", func(m *Monitor) { backupPollingReadResult = m.buildPBSBackupCache("pbs") }},
		{"storage-unread", func(m *Monitor) {
			m.pollStorageBackupsWithNodes(context.Background(), "site", backupPollingUnreadClient{}, []proxmox.Node{{Node: "node"}}, map[string]string{"node": "offline"})
		}},
		{"snapshots-unread", func(m *Monitor) { m.pollGuestSnapshots(context.Background(), "site", backupPollingUnreadClient{}) }},
	}
}

func TestBackupPollingUnreadReadsKeepPrimaryBackupEvidence(t *testing.T) {
	m := backupPollingReadFixture(10)
	before := m.state.GetPVEBackups()
	byID := func(rows []models.GuestSnapshot) map[string]models.GuestSnapshot {
		result := make(map[string]models.GuestSnapshot, len(rows))
		for _, row := range rows {
			result[row.ID] = row
		}
		return result
	}
	for _, c := range backupPollingReadCases()[1:] {
		c.call(m)
		after := m.state.GetPVEBackups()
		// Publication preserves per-instance rows but moves the replaced
		// instance after its peers. Compare evidence, not that incidental order.
		if !reflect.DeepEqual(after.BackupTasks, before.BackupTasks) ||
			!reflect.DeepEqual(after.StorageBackups, before.StorageBackups) ||
			!reflect.DeepEqual(byID(after.GuestSnapshots), byID(before.GuestSnapshots)) {
			t.Fatalf("%s changed primary evidence or another instance: got %+v, want %+v", c.name, after, before)
		}
	}
}

func TestBackupPollingPBSCacheKeepsIdentityAndDetachedEvidence(t *testing.T) {
	m := backupPollingReadFixture(10)
	original := m.state.PBSBackups[0]
	for _, change := range []struct{ instance, store, namespace, kind string }{
		{"peer", "archive", "team", "vm"},
		{"pbs", "other", "team", "vm"},
		{"pbs", "archive", "other", "vm"},
		{"pbs", "archive", "team", "ct"},
	} {
		row := original
		row.ID = fmt.Sprint(change)
		row.Instance, row.Datastore, row.Namespace, row.BackupType = change.instance, change.store, change.namespace, change.kind
		m.state.PBSBackups = append(m.state.PBSBackups, row)
	}
	before := m.state.GetSnapshot().PBSBackups
	cache := m.buildPBSBackupCache("pbs")
	if len(cache) != 4 {
		t.Fatalf("instance/namespace/type/datastore grouping changed: %+v", cache)
	}
	for key, group := range cache {
		if len(group.snapshots) != 1 || !group.latest.Equal(original.BackupTime) {
			t.Fatalf("unexpected group %v: %+v", key, group)
		}
		row := group.snapshots[0]
		if row.Instance != "pbs" || row.Datastore != key.datastore || row.Namespace != key.namespace || row.BackupType != key.backupType || row.VMID != key.backupID ||
			!row.Protected || !row.InProgress || !row.WriteActivityObserved || !row.WriteActive || !row.CarriedOver || row.Owner != original.Owner {
			t.Fatalf("group %v lost source identity or evidence: %+v", key, row)
		}
		row.Files[0] = "reader-only"
		row.VerificationRaw.(map[string]interface{})["state"] = "reader-only"
		group.snapshots[0] = models.PBSBackup{}
	}
	if after := m.state.GetSnapshot().PBSBackups; !reflect.DeepEqual(after, before) {
		t.Fatal("cache caller mutated primary backup evidence")
	}
}

func TestBackupPollingReadsDoNotCopyUnrelatedInventory(t *testing.T) {
	for _, c := range backupPollingReadCases() {
		t.Run(c.name, func(t *testing.T) {
			small, large := backupPollingReadFixture(0), backupPollingReadFixture(1000)
			measure := func(m *Monitor) float64 { return testing.AllocsPerRun(5, func() { c.call(m) }) }
			base, grown := measure(small), measure(large)
			t.Logf("allocations per real callback: small=%.0f unrelated=%.0f", base, grown)
			if grown > base+4 {
				t.Fatalf("backup callback copied unrelated fleet inventory: %.0f -> %.0f", base, grown)
			}
		})
	}
	small, large := backupPollingReadFixture(0), backupPollingReadFixture(1000)
	measure := func(m *Monitor) float64 {
		return testing.AllocsPerRun(5, func() { backupPollingReadResult = m.state.GetSnapshot() })
	}
	base, grown := measure(small), measure(large)
	t.Logf("full snapshot positive control: %.0f -> %.0f", base, grown)
	if grown <= base+1000 {
		t.Fatal("fixture does not expose whole-fleet copying")
	}
}

// Fixed callback measurements keep provider operations, retained backup data
// and registry inputs the same while growing only unrelated fleet metadata.
func BenchmarkBackupPollingReadCost(b *testing.B) {
	previous := log.Logger
	log.Logger = log.Output(io.Discard)
	b.Cleanup(func() { log.Logger = previous })
	for _, c := range backupPollingReadCases() {
		for _, size := range []int{0, 1000} {
			b.Run(fmt.Sprintf("%s/unrelated-%d", c.name, size), func(b *testing.B) {
				m := backupPollingReadFixture(size)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					c.call(m)
				}
			})
		}
	}
}
