package monitoring

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func metadataMonitorFixture(size int) *Monitor {
	state := models.NewState()
	stamp := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	rate := 64.0
	state.PVEBackups = models.PVEBackups{BackupTasks: []models.BackupTask{{ID: "task", VMID: 100, Status: "OK"}}}
	state.PBSBackups = []models.PBSBackup{{ID: "pbs", Files: []string{"index.json.blob"}, Protected: true, BackupTime: stamp}}
	state.PMGBackups = []models.PMGBackup{{ID: "pmg", BackupTime: stamp}}
	state.ReplicationJobs = []models.ReplicationJob{{ID: "job", LastSyncTime: &stamp, NextSyncTime: &stamp, RateLimitMbps: &rate}}
	state.ConnectionHealth = map[string]bool{"pve": true, "pbs": false}
	cluster := models.KubernetesCluster{ID: "cluster"}
	for i := 0; i < size; i++ {
		id := fmt.Sprint(i)
		state.VMs = append(state.VMs, models.VM{ID: id, Tags: []string{"production"}, Disks: []models.Disk{{Mountpoint: "/", Total: 100}}})
		cluster.ConfigMaps = append(cluster.ConfigMaps, models.KubernetesConfigMap{UID: id, DataKeys: []string{"key"}, Labels: map[string]string{"app": "test"}})
	}
	if size > 0 {
		state.KubernetesClusters = []models.KubernetesCluster{cluster}
	}
	return &Monitor{state: state}
}

var metadataReaders = []struct {
	name     string
	current  func(*Monitor) any
	previous func(*Monitor) any
}{
	{"backups", func(m *Monitor) any { return m.BackupsSnapshot() }, func(m *Monitor) any { return m.GetState().Backups }},
	{"pve", func(m *Monitor) any { return m.PVEBackupsSnapshot() }, func(m *Monitor) any { return m.GetState().PVEBackups }},
	{"pbs", func(m *Monitor) any { return m.PBSBackupsSnapshot() }, func(m *Monitor) any { return m.GetState().PBSBackups }},
	{"replication", func(m *Monitor) any { return m.ReplicationJobsSnapshot() }, func(m *Monitor) any { return m.GetState().ReplicationJobs }},
	{"health", func(m *Monitor) any { return m.ConnectionHealthSnapshot() }, func(m *Monitor) any { return m.GetState().ConnectionHealth }},
}

func TestMetadataMonitorProjectionParity(t *testing.T) {
	previous := mock.IsMockEnabled()
	defer mustSetMockEnabled(t, previous)
	for _, enabled := range []bool{false, true} {
		mustSetMockEnabled(t, enabled)
		for _, monitor := range []*Monitor{nil, {}, {state: models.NewState()}, metadataMonitorFixture(100)} {
			for _, reader := range metadataReaders {
				got, want := reader.current(monitor), reader.previous(monitor)
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("%s differs in mock=%t (nil=%t)", reader.name, enabled, monitor == nil)
				}
				gotJSON, err := json.Marshal(got)
				if err != nil {
					t.Fatal(err)
				}
				wantJSON, err := json.Marshal(want)
				if err != nil || string(gotJSON) != string(wantJSON) {
					t.Fatalf("%s changed serialized output", reader.name)
				}
			}
		}
	}
}

func TestMetadataMonitorProjectionConcurrentReset(t *testing.T) {
	previous := mock.IsMockEnabled()
	mustSetMockEnabled(t, false)
	defer mustSetMockEnabled(t, previous)
	m := metadataMonitorFixture(0)
	retained := m.BackupsSnapshot()
	var workers sync.WaitGroup
	workers.Add(2)
	go func() {
		defer workers.Done()
		for i := 0; i < 100; i++ {
			m.mu.Lock()
			m.resetStateLocked()
			m.mu.Unlock()
		}
	}()
	go func() {
		defer workers.Done()
		for i := 0; i < 100; i++ {
			for _, reader := range metadataReaders {
				_ = reader.current(m)
			}
		}
	}()
	workers.Wait()
	if len(retained.PBS) != 1 || retained.PBS[0].ID != "pbs" {
		t.Fatal("reset invalidated caller-owned backup data")
	}
	for _, reader := range metadataReaders {
		if !reflect.DeepEqual(reader.current(m), reader.previous(m)) {
			t.Fatalf("%s changed reset semantics", reader.name)
		}
	}
}

func TestMetadataMonitorProjectionAllocationScaling(t *testing.T) {
	previous := mock.IsMockEnabled()
	mustSetMockEnabled(t, false)
	defer mustSetMockEnabled(t, previous)
	small, large := metadataMonitorFixture(0), metadataMonitorFixture(1000)
	for _, reader := range metadataReaders {
		baseline := testing.AllocsPerRun(10, func() { metadataBenchmarkResult = reader.current(small) })
		grown := testing.AllocsPerRun(10, func() { metadataBenchmarkResult = reader.current(large) })
		if grown != baseline {
			t.Errorf("%s unrelated estate increased allocations: small=%.0f large=%.0f", reader.name, baseline, grown)
		}
		old := testing.AllocsPerRun(10, func() { metadataBenchmarkResult = reader.previous(large) })
		if grown >= old {
			t.Errorf("%s scoped=%.0f did not reduce full-estate=%.0f allocations", reader.name, grown, old)
		}
	}
}

var metadataBenchmarkResult any

// Fixed backup/job/health payloads; only unrelated guests and Kubernetes
// metadata grow. Compare the actual old/current monitor accessors, not a
// substitute micro-kernel, database path or reporter's running process.
func BenchmarkMetadataMonitorProjection(b *testing.B) {
	previous := mock.IsMockEnabled()
	mustSetMockEnabled(b, false)
	b.Cleanup(func() { mustSetMockEnabled(b, previous) })
	for _, size := range []int{0, 100, 1000} {
		m := metadataMonitorFixture(size)
		for _, reader := range metadataReaders {
			for _, method := range []struct {
				name string
				read func(*Monitor) any
			}{{"full", reader.previous}, {"scoped", reader.current}} {
				b.Run(fmt.Sprintf("%s/%d/%s", reader.name, size, method.name), func(b *testing.B) {
					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						metadataBenchmarkResult = method.read(m)
					}
				})
			}
		}
	}
}
