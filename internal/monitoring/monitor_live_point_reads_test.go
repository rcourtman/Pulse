package monitoring

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

var livePointReadTime = time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)

func livePointReadFixture(size int) (*Monitor, *agentshost.ProxmoxLXCInventory) {
	m := metadataMonitorFixture(size)
	m.state.Nodes = []models.Node{{ID: "node-id", Instance: "pve-a", Name: "node-a", LinkedAgentID: "agent-a"}}
	m.state.PVEBackups.BackupTasks = []models.BackupTask{{
		ID: "active", Instance: "pve-a", Node: "node-a", VMID: 100, Status: "running", ObservedAt: livePointReadTime,
	}}
	for i := 0; i < size; i++ {
		m.state.PVEBackups.StorageBackups = append(m.state.PVEBackups.StorageBackups,
			models.StorageBackup{ID: fmt.Sprint(i), Notes: "unrelated restore point"})
	}
	return m, &agentshost.ProxmoxLXCInventory{Status: agentshost.ProxmoxLXCCollectionComplete,
		Containers: []agentshost.ProxmoxLXCContainer{{VMID: 100, Name: "web", Disks: []agentshost.Disk{
			{Type: "rootfs", Mountpoint: "/", Device: "local:100/root", TotalBytes: 100, UsedBytes: 25, FreeBytes: 75},
			{Type: "mp0", Mountpoint: "/srv/data", Device: "tank:100/data", TotalBytes: 1000, UsedBytes: 500, FreeBytes: 500},
		}}}}
}

func TestLivePointReadsBackupPolicyBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name     string
		change   func(*models.BackupTask)
		instance string
		node     string
		vmid     int
		found    bool
	}{
		{"fresh", nil, "pve-a", "node-a", 100, true},
		{"trimmed-query", nil, " pve-a ", " node-a ", 100, true},
		{"wrong-instance", nil, "pve-b", "node-a", 100, false},
		{"wrong-node", nil, "pve-a", "node-b", 100, false},
		{"wrong-vmid", nil, "pve-a", "node-a", 101, false},
		{"invalid-vmid", nil, "pve-a", "node-a", 0, false},
		{"empty-query-hints", nil, "", "", 100, true},
		{"missing-task-node", func(task *models.BackupTask) { task.Node = "" }, "pve-a", "node-a", 100, true},
		{"exact-age-boundary", func(task *models.BackupTask) { task.ObservedAt = livePointReadTime.Add(-backupIntentEvidenceMaxAge) }, "pve-a", "node-a", 100, true},
		{"too-old", func(task *models.BackupTask) {
			task.ObservedAt = livePointReadTime.Add(-backupIntentEvidenceMaxAge - time.Nanosecond)
		}, "pve-a", "node-a", 100, false},
		{"unobserved", func(task *models.BackupTask) { task.ObservedAt = time.Time{} }, "pve-a", "node-a", 100, false},
		{"future-boundary", func(task *models.BackupTask) { task.ObservedAt = livePointReadTime.Add(time.Minute) }, "pve-a", "node-a", 100, true},
		{"too-far-future", func(task *models.BackupTask) { task.ObservedAt = livePointReadTime.Add(time.Minute + time.Nanosecond) }, "pve-a", "node-a", 100, false},
		{"end-time", func(task *models.BackupTask) { task.EndTime = livePointReadTime }, "pve-a", "node-a", 100, false},
		{"ok", func(task *models.BackupTask) { task.Status = " OK " }, "pve-a", "node-a", 100, false},
		{"stopped", func(task *models.BackupTask) { task.Status = "Stopped" }, "pve-a", "node-a", 100, false},
		{"error", func(task *models.BackupTask) { task.Status = "error" }, "pve-a", "node-a", 100, false},
		{"warning", func(task *models.BackupTask) { task.Status = "WARNING" }, "pve-a", "node-a", 100, false},
		// Preserve the existing resolver's non-terminal rule, not a new status policy.
		{"empty-status", func(task *models.BackupTask) { task.Status = "" }, "pve-a", "node-a", 100, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, _ := livePointReadFixture(0)
			if tc.change != nil {
				tc.change(&m.state.PVEBackups.BackupTasks[0])
			}
			got, found := m.resolveBackupIntentContext("ignored", tc.instance, tc.node, tc.vmid, livePointReadTime)
			want := alerts.BackupIntentContext{}
			if tc.found {
				want = alerts.BackupIntentContext{Active: true, ObservedAt: m.state.PVEBackups.BackupTasks[0].ObservedAt, Evidence: "pve_vzdump_task:active"}
			}
			if found != tc.found || !reflect.DeepEqual(got, want) {
				t.Fatalf("lookup = %+v, %t; want %+v, %t", got, found, want, tc.found)
			}
		})
	}
	m, _ := livePointReadFixture(0)
	m.state.PVEBackups.BackupTasks = append(m.state.PVEBackups.BackupTasks,
		models.BackupTask{ID: "later", VMID: 100, Instance: "pve-a", Status: "running", ObservedAt: livePointReadTime})
	got, found := m.resolveBackupIntentContext("", "pve-a", "node-a", 100, livePointReadTime)
	if !found || got.Evidence != "pve_vzdump_task:active" {
		t.Fatal("first matching task no longer wins in source order")
	}
	m.state.PVEBackups.BackupTasks[0].ObservedAt = time.Now()
	if _, found := m.resolveBackupIntentContext("", "pve-a", "node-a", 100, time.Time{}); !found {
		t.Fatal("zero caller clock no longer uses current time")
	}
}

func TestLivePointReadsNilAndMockSemantics(t *testing.T) {
	previous := mock.IsMockEnabled()
	defer mustSetMockEnabled(t, previous)
	for _, enabled := range []bool{false, true} {
		mustSetMockEnabled(t, enabled)
		for _, m := range []*Monitor{nil, {}, {state: models.NewState()}} {
			if got, found := m.resolveBackupIntentContext("", "pve-a", "node-a", 100, livePointReadTime); found || got.Active {
				t.Fatal("absent live state invented backup evidence")
			}
			m.applyAgentLXCFilesystems("node-id", "agent-a", &agentshost.ProxmoxLXCInventory{}, livePointReadTime, 30)
		}
		// These internal live-state queries have never substituted mock fixtures.
		m, inventory := livePointReadFixture(0)
		m.applyAgentLXCFilesystems("node-id", "agent-a", inventory, livePointReadTime, 30)
		if len(m.proxmoxLXCFilesystemsCache) != 1 {
			t.Fatal("live admission unexpectedly read a mock inventory")
		}
		if _, found := m.resolveBackupIntentContext("", "pve-a", "node-a", 100, livePointReadTime); !found {
			t.Fatal("live task lookup unexpectedly read a mock inventory")
		}
	}
}

func TestLivePointReadsConcurrentMonitorStateReplacement(t *testing.T) {
	m, inventory := livePointReadFixture(0)
	other, _ := livePointReadFixture(0)
	states := []*models.State{m.state, other.state, nil}
	var workers sync.WaitGroup
	workers.Add(2)
	go func() {
		defer workers.Done()
		for i := 0; i < 100; i++ {
			m.mu.Lock()
			m.state = states[i%len(states)]
			m.mu.Unlock()
		}
	}()
	go func() {
		defer workers.Done()
		for i := 0; i < 100; i++ {
			m.applyAgentLXCFilesystems("node-id", "agent-a", inventory, livePointReadTime, 30)
			m.resolveBackupIntentContext("", "pve-a", "node-a", 100, livePointReadTime)
		}
	}()
	workers.Wait()
}

// The same benchmark compiles against the candidate and exact former entry
// point sources. Fixed requested data; only unrelated estate/artifacts grow.
func BenchmarkLivePointReads(b *testing.B) {
	for _, size := range []int{0, 100, 1000} {
		m, inventory := livePointReadFixture(size)
		b.Run(fmt.Sprintf("agent-filesystems/%d", size), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				m.applyAgentLXCFilesystems("node-id", "agent-a", inventory, livePointReadTime, 30)
			}
		})
		b.Run(fmt.Sprintf("backup-intent/%d", size), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				livePointReadContext, livePointReadFound = m.resolveBackupIntentContext("", "pve-a", "node-a", 100, livePointReadTime)
			}
		})
	}
}

var livePointReadContext alerts.BackupIntentContext
var livePointReadFound bool

func TestLivePointReadsAllocationScaling(t *testing.T) {
	small, inventory := livePointReadFixture(0)
	large, _ := livePointReadFixture(1000)
	for _, read := range []struct {
		name string
		call func(*Monitor)
	}{
		{"agent-filesystems", func(m *Monitor) { m.applyAgentLXCFilesystems("node-id", "agent-a", inventory, livePointReadTime, 30) }},
		{"backup-intent", func(m *Monitor) {
			livePointReadContext, livePointReadFound = m.resolveBackupIntentContext("", "pve-a", "node-a", 100, livePointReadTime)
		}},
	} {
		t.Run(read.name, func(t *testing.T) {
			baseline := testing.AllocsPerRun(10, func() { read.call(small) })
			grown := testing.AllocsPerRun(10, func() { read.call(large) })
			if grown != baseline {
				t.Fatalf("unrelated estate increased allocations: %.0f -> %.0f", baseline, grown)
			}
		})
	}
}

// Export actual admission -> enrichment -> frontend-projection bytes, plus the
// alert resolver result, for an exact baseline/candidate comparison. No browser,
// installed estate, native helper or reporter incident is represented by this.
func TestLivePointReadsProducerParityExport(t *testing.T) {
	for _, scenario := range []string{"complete", "partial", "expired", "ambiguous", "missing", "renamed"} {
		m, inventory := livePointReadFixture(0)
		if scenario == "ambiguous" {
			m.state.Nodes = append(m.state.Nodes, models.Node{ID: "node-id", Instance: "other", Name: "other"})
		} else if scenario == "missing" {
			m.state.Nodes = nil
		} else if scenario == "partial" {
			inventory.Status = agentshost.ProxmoxLXCCollectionPartial
			inventory.OmittedVMIDs = []int{101}
		}
		m.applyAgentLXCFilesystems("node-id", "agent-a", inventory, livePointReadTime, 30)
		guest := models.Container{ID: "pve-a:node-a:100", VMID: 100, Instance: "pve-a", Node: "node-a", Name: "web", Status: "running", Disk: models.Disk{Total: 100, Used: 10, Free: 90, Mountpoint: "/"}}
		readAt := livePointReadTime.Add(time.Minute)
		if scenario == "expired" {
			readAt = livePointReadTime.Add(agentLXCFilesystemMinTTL)
		} else if scenario == "renamed" {
			guest.Name = "renamed"
		}
		m.enrichContainerWithAgentLXCFilesystems("pve-a", "node-a", &guest, readAt)
		wantDisks := 0
		if scenario == "complete" || scenario == "partial" {
			wantDisks = 2
		}
		if len(guest.Disks) != wantDisks || (wantDisks == 2 && guest.Disk.Used != 25) || (wantDisks == 0 && guest.Disk.Used != 10) {
			t.Fatalf("%s changed admission/enrichment: %+v", scenario, guest)
		}
		m.state.UpdateContainers([]models.Container{guest})
		m.state.LastUpdate = livePointReadTime
		backup, found := m.resolveBackupIntentContext("", "pve-a", "node-a", 100, livePointReadTime)
		frame := struct {
			Scenario string                     `json:"scenario"`
			State    models.StateFrontend       `json:"state"`
			Backup   alerts.BackupIntentContext `json:"backup"`
			Found    bool                       `json:"found"`
		}{scenario, buildFrontendStateFromSnapshot(m.state.GetSnapshot()), backup, found}
		data, err := json.Marshal(frame)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Printf("POINT_READ_PRODUCER=%s\n", data)
	}
}
