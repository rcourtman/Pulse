package monitoring

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// Keep the ordinary typed read path, with an already populated canonical
// generation. Only irrelevant payload grows; snapshot inputs stay fixed.
type snapshotProjectionStore struct {
	resourceOnlyStore
	unifiedresources.ReadState
}

func snapshotProjectionMonitor(unrelated, detail int) *Monitor {
	resources := make([]unifiedresources.Resource, 0, 2)
	for i, kind := range []unifiedresources.ResourceType{unifiedresources.ResourceTypeVM, unifiedresources.ResourceTypeSystemContainer} {
		data := &unifiedresources.ProxmoxData{Instance: "site", NodeName: "member", VMID: 100 + i, SourceID: fmt.Sprintf("site:member:%d", 100+i)}
		for j := 0; j < detail; j++ {
			data.Disks = append(data.Disks, unifiedresources.DiskInfo{Device: fmt.Sprint(j), Mountpoint: "/", Total: 1000, Used: 200})
			data.NetworkInterfaces = append(data.NetworkInterfaces, unifiedresources.NetworkInterface{Name: fmt.Sprint(j), Addresses: []string{"192.0.2.1"}})
		}
		resources = append(resources, unifiedresources.Resource{ID: data.SourceID, Type: kind, Name: fmt.Sprintf("guest-%d", i), Tags: []string{"production"}, Proxmox: data})
	}
	readState := backupReadState(resources)
	// Warm the cached views outside allocation/benchmark measurements.
	_ = readState.VMs()
	_ = readState.Containers()
	m := &Monitor{state: models.NewState(), resourceStore: &snapshotProjectionStore{ReadState: readState}}
	cluster := models.KubernetesCluster{ID: "unrelated"}
	for i := 0; i < unrelated; i++ {
		cluster.ConfigMaps = append(cluster.ConfigMaps, models.KubernetesConfigMap{UID: fmt.Sprint(i), DataKeys: []string{"key"}, Labels: map[string]string{"app": "unrelated"}})
	}
	if unrelated > 0 {
		m.state.KubernetesClusters = []models.KubernetesCluster{cluster}
	}
	return m
}

func snapshotProjectionClient() *snapshotWorkerClient {
	return &snapshotWorkerClient{read: func(context.Context, string, int) ([]proxmox.Snapshot, error) {
		return []proxmox.Snapshot{{Name: "weekly", SnapTime: 1792028400, Description: "fixture", Parent: "previous"}}, nil
	}}
}

func TestGuestSnapshotProjectionDoesNotCopyUnconsumedPayload(t *testing.T) {
	for _, payload := range []string{"unrelated-estate", "guest-detail"} {
		t.Run(payload, func(t *testing.T) {
			small := snapshotProjectionMonitor(0, 0)
			var large *Monitor
			if payload == "unrelated-estate" {
				large = snapshotProjectionMonitor(1000, 0)
			} else {
				large = snapshotProjectionMonitor(0, 1000)
			}
			client := snapshotProjectionClient()
			measure := func(m *Monitor) float64 {
				return testing.AllocsPerRun(10, func() { m.pollGuestSnapshots(context.Background(), "site", client) })
			}
			base, grown := measure(small), measure(large)
			t.Logf("SNAPSHOT_PROJECTION payload=%s base_allocs=%.0f grown_allocs=%.0f", payload, base, grown)
			// Worker startup can vary by a few allocations. Thousands of
			// unconsumed fields must not add one copy each to this scan.
			if grown > base+8 {
				t.Errorf("unconsumed %s increased snapshot scan allocations: %.0f -> %.0f", payload, base, grown)
			}
			// Equal-time snapshots are emitted from a map with no tie-order
			// contract. Compare every field by identity, not map iteration.
			smallRows, largeRows := small.state.GetPVEBackups().GuestSnapshots, large.state.GetPVEBackups().GuestSnapshots
			sort.Slice(smallRows, func(i, j int) bool { return smallRows[i].ID < smallRows[j].ID })
			sort.Slice(largeRows, func(i, j int) bool { return largeRows[i].ID < largeRows[j].ID })
			if !reflect.DeepEqual(smallRows, largeRows) {
				t.Fatal("irrelevant payload changed the published snapshot inventory")
			}
		})
	}
}

func TestGuestSnapshotProjectionPreservesAlertIdentityAndScope(t *testing.T) {
	m := snapshotProjectionMonitor(20, 20)
	manager := alerts.NewManagerWithDataDir(t.TempDir())
	defer manager.Stop()
	cfg := manager.GetConfig()
	cfg.Enabled = true
	cfg.SnapshotDefaults = alerts.SnapshotAlertConfig{Enabled: true, WarningDays: 1, CriticalDays: 2}
	cfg.GuestTagBlacklist = []string{"maintenance"}
	manager.UpdateConfig(cfg)
	m.alertManager = manager
	// Names, tags and template state must come from the canonical view,
	// not an unrelated legacy row with the same VMID.
	resources := []unifiedresources.Resource{}
	for i, kind := range []unifiedresources.ResourceType{unifiedresources.ResourceTypeVM, unifiedresources.ResourceTypeSystemContainer} {
		for j, label := range []string{"visible", "maintenance", "template", "other-connection"} {
			instance := "site"
			if j == 3 {
				instance = "other"
			}
			vmid := 100 + i*10 + j
			resources = append(resources, unifiedresources.Resource{ID: fmt.Sprintf("canonical-%d", vmid), Type: kind, Name: fmt.Sprintf("%s-%d", label, vmid), Tags: []string{label}, Proxmox: &unifiedresources.ProxmoxData{Instance: instance, NodeName: "member", VMID: vmid, Template: j == 2, SourceID: fmt.Sprintf("%s:member:%d", instance, vmid)}})
		}
	}
	m.resourceStore = &snapshotProjectionStore{ReadState: backupReadState(resources)}
	m.state.UpdateVMsForInstance("site", []models.VM{{ID: "legacy", VMID: 100, Instance: "site", Node: "wrong-node", Name: "wrong-name"}})
	foreign := models.GuestSnapshot{ID: "foreign", Instance: "other", Node: "member", VMID: 100, Type: "qemu", Name: "keep", Time: time.Unix(1792028400, 0)}
	m.state.UpdateGuestSnapshotsForInstance("other", []models.GuestSnapshot{foreign})
	created := time.Now().UTC().Truncate(time.Second).Add(-3 * 24 * time.Hour)
	var mu sync.Mutex
	calls := make(map[int]string)
	client := &snapshotWorkerClient{read: func(_ context.Context, kind string, vmid int) ([]proxmox.Snapshot, error) {
		mu.Lock()
		calls[vmid] = kind
		mu.Unlock()
		return []proxmox.Snapshot{{Name: "weekly", SnapTime: created.Unix(), Description: "fixture", Parent: "previous"}}, nil
	}}
	m.pollGuestSnapshots(context.Background(), "site", client)
	if want := (map[int]string{100: "qemu", 101: "qemu", 110: "lxc", 111: "lxc"}); !reflect.DeepEqual(calls, want) {
		t.Fatalf("snapshot request scope/type changed: got=%v want=%v", calls, want)
	}
	got := m.state.GetPVEBackups().GuestSnapshots
	if len(got) != 5 {
		t.Fatalf("published %d snapshots, want four observed and one independent", len(got))
	}
	for _, row := range got {
		if row.Instance == "other" {
			if row != foreign {
				t.Error("snapshot scan changed the independent connection")
			}
			continue
		}
		want := models.GuestSnapshot{ID: fmt.Sprintf("site-member-%d-weekly", row.VMID), Instance: "site", Node: "member", VMID: row.VMID, Type: calls[row.VMID], Name: "weekly", Time: time.Unix(created.Unix(), 0), Description: "fixture", Parent: "previous", VMState: calls[row.VMID] == "qemu"}
		if row != want {
			t.Errorf("snapshot fields changed: got=%+v want=%+v", row, want)
		}
	}
	active := manager.GetActiveAlerts()
	if len(active) != 2 {
		t.Fatalf("snapshot lookup lost tag suppression: got %d alerts, want two", len(active))
	}
	for _, alert := range active {
		if alert.ResourceID != "site:member:100" && alert.ResourceID != "site:member:110" {
			t.Errorf("snapshot alert has unexpected identity: %+v", alert)
		}
		if alert.ResourceName != "visible-100 snapshot 'weekly'" && alert.ResourceName != "visible-110 snapshot 'weekly'" {
			t.Errorf("snapshot alert lost its canonical name: %q", alert.ResourceName)
		}
	}
}

// Fixed real collector, fixed snapshot result. Compare the same samples before
// and after the repair; this measures no database, remote guest or installed RSS.
func BenchmarkGuestSnapshotProjection(b *testing.B) {
	for _, fixture := range []struct {
		name              string
		unrelated, detail int
	}{{"small", 0, 0}, {"unrelated=1000", 1000, 0}, {"guest-detail=1000", 0, 1000}} {
		b.Run(fixture.name, func(b *testing.B) {
			m, client := snapshotProjectionMonitor(fixture.unrelated, fixture.detail), snapshotProjectionClient()
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				m.pollGuestSnapshots(context.Background(), "site", client)
			}
			b.StopTimer()
			if len(m.state.GetPVEBackups().GuestSnapshots) != 2 {
				b.Fatal("snapshot inventory did not reach state")
			}
		})
	}
}
