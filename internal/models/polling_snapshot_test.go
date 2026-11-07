package models

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"
)

func pollingStateFixture() *State {
	at := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	s := NewState()
	s.Nodes = []Node{{Name: "peer", Instance: "other"}, {Name: "same", Instance: "pve"}, {Name: "same", Instance: "other"}, {Name: ""}, {Name: " same "}, {Name: "SAME"}}
	s.Storage = []Storage{
		{ID: "same", Name: "first", Instance: "pve", Nodes: []string{"a"}, NodeIDs: []string{"pve-a"}, AliasIDs: []string{"legacy"}, LastSeen: at, Total: 1000, Used: 300, Free: 700, Usage: 30,
			ZFSPool: &ZFSPool{Name: "tank", Devices: []ZFSDevice{{Name: "disk"}}, Datasets: []ZFSDataset{{Name: "tank/data", Mountpoint: "/data"}}, ScanDetails: &ZFSScan{Function: "scrub", Percentage: 42, StartedAt: &at, EndedAt: &at}}},
		{ID: "same", Name: "other", Instance: "other"},
		{ID: "same", Name: "last", Instance: "pve", ZFSPool: &ZFSPool{Name: "empty"}},
		{ID: "", Name: "default", Instance: ""},
		{ID: "space", Instance: " pve"}, {ID: "case", Instance: "PVE"},
	}
	return s
}

func assertPollingSnapshotParity(t *testing.T, s *State) {
	t.Helper()
	snapshot := s.GetSnapshot()
	names := map[string]bool{}
	for _, node := range snapshot.Nodes {
		names[node.Name] = true
	}
	if got := s.GetNodeNames(); !reflect.DeepEqual(got, names) {
		t.Fatalf("node membership = %#v, want %#v", got, names)
	}
	for _, instance := range []string{"pve", "other", "", " pve", "PVE", "missing"} {
		want := []Storage{}
		for _, row := range snapshot.Storage {
			if row.Instance == instance {
				want = append(want, row)
			}
		}
		got := s.GetStorageForInstance(instance)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("storage for %q differs from full snapshot: got %+v, want %+v", instance, got, want)
		}
		a, err := json.Marshal(got)
		if err != nil {
			t.Fatal(err)
		}
		b, err := json.Marshal(want)
		if err != nil || string(a) != string(b) {
			t.Fatalf("storage JSON changed for %q", instance)
		}
	}
}

func TestPollingSnapshotsMatchFullStateAndDetach(t *testing.T) {
	for _, s := range []*State{{}, NewState(), pollingStateFixture()} {
		assertPollingSnapshotParity(t, s)
	}
	s := pollingStateFixture()
	before, err := json.Marshal(s.GetSnapshot())
	if err != nil {
		t.Fatal(err)
	}
	names := s.GetNodeNames()
	delete(names, "peer")
	names["reader"] = true
	// Both the complete snapshot and the narrow read must detach the nested
	// scan object and timestamps, as well as membership and physical-disk data.
	for _, rows := range [][]Storage{s.GetStorageForInstance("pve"), s.GetSnapshot().Storage} {
		rows[0].Nodes[0] = "reader"
		rows[0].NodeIDs[0] = "reader"
		rows[0].AliasIDs[0] = "reader"
		rows[0].ZFSPool.Devices[0].Name = "reader"
		rows[0].ZFSPool.Datasets[0].Name = "reader"
		rows[0].ZFSPool.ScanDetails.Function = "reader"
		*rows[0].ZFSPool.ScanDetails.StartedAt = time.Time{}
		*rows[0].ZFSPool.ScanDetails.EndedAt = time.Time{}
		rows[0] = Storage{}
	}
	after, err := json.Marshal(s.GetSnapshot())
	if err != nil || string(before) != string(after) {
		t.Fatal("polling read shares mutable source fields")
	}
	assertPollingSnapshotParity(t, s)
}

func TestPollingSnapshotsConcurrentSourceReplacement(t *testing.T) {
	s := pollingStateFixture()
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			id := fmt.Sprint(i)
			s.mu.Lock()
			s.Nodes = []Node{{Name: id}, {Name: "peer"}}
			row := cloneStorage(s.Storage[0])
			row.Name = id
			row.Nodes[0] = id
			row.ZFSPool.Devices[0].Name = id
			row.ZFSPool.ScanDetails.Function = id
			s.Storage = []Storage{row}
			s.mu.Unlock()
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			if names := s.GetNodeNames(); !names["peer"] {
				t.Error("lost peer membership")
				return
			}
			rows := s.GetStorageForInstance("pve")
			row := rows[0]
			// Initial fixture predates the writer's generation tags.
			if row.Name != "first" && (row.Nodes[0] != row.Name || row.ZFSPool.Devices[0].Name != row.Name || row.ZFSPool.ScanDetails.Function != row.Name) {
				t.Error("read mixed writer generations")
				return
			}
			row.Nodes[0] = "reader"
			row.ZFSPool.Devices[0].Name = "reader"
			row.ZFSPool.ScanDetails.Function = "reader"
		}
	}()
	wg.Wait()
}

var pollingSnapshotResult any

func TestPollingSnapshotAllocationDoesNotCopyUnrelatedInventory(t *testing.T) {
	small, large := pollingStateFixture(), pollingStateFixture()
	cluster := KubernetesCluster{ID: "unrelated"}
	for i := 0; i < 1000; i++ {
		cluster.ConfigMaps = append(cluster.ConfigMaps, KubernetesConfigMap{UID: fmt.Sprint(i), Labels: map[string]string{"fixture": "only"}, DataKeys: []string{"key"}})
		large.VMs = append(large.VMs, VM{ID: fmt.Sprint(i), Disks: []Disk{{Mountpoint: "/"}}})
	}
	large.KubernetesClusters = []KubernetesCluster{cluster}
	for _, read := range []struct {
		name string
		call func(*State) any
	}{
		{"node names", func(s *State) any { return s.GetNodeNames() }},
		{"storage", func(s *State) any { return s.GetStorageForInstance("pve") }},
	} {
		measure := func(s *State) float64 {
			return testing.AllocsPerRun(5, func() { pollingSnapshotResult = read.call(s) })
		}
		base, grown := measure(small), measure(large)
		t.Logf("%s allocations: small=%.0f unrelated=%.0f", read.name, base, grown)
		if grown > base+2 {
			t.Fatalf("%s copied unrelated inventory: %.0f -> %.0f", read.name, base, grown)
		}
	}
	full := func(s *State) float64 {
		return testing.AllocsPerRun(5, func() { pollingSnapshotResult = s.GetSnapshot() })
	}
	base, grown := full(small), full(large)
	t.Logf("full snapshot positive control: %.0f -> %.0f", base, grown)
	if grown <= base+1000 {
		t.Fatal("fixture did not expose full-fleet copying")
	}
}
