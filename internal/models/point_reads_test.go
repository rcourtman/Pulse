package models

import (
	"encoding/json"
	"reflect"
	"sync"
	"testing"
)

func TestPointReadsMatchSnapshotAndDetach(t *testing.T) {
	enabled := true
	speed := int64(1000)
	s := metadataStateFixture()
	s.Nodes = []Node{{
		ID: "node-id", Instance: "pve", Name: "node-a",
		LoadAverage: []float64{1}, NativeNameAliases: []string{"node-a.example"},
		NetworkInterfaces:            []HostNetworkInterface{{Name: "eth0", Addresses: []string{"192.0.2.1"}, SpeedMbps: &speed}},
		TemperatureMonitoringEnabled: &enabled,
	}}
	before, err := json.Marshal(s.GetSnapshot())
	if err != nil {
		t.Fatal(err)
	}
	got, ok := s.GetUniqueNode("node-id")
	if !ok || !reflect.DeepEqual(got, s.GetSnapshot().Nodes[0]) {
		t.Fatal("unique node differs from its full-snapshot record")
	}
	tasks := s.GetBackupTasks()
	if !reflect.DeepEqual(tasks, s.GetSnapshot().PVEBackups.BackupTasks) {
		t.Fatal("task order or primary source changed")
	}
	got.LoadAverage[0] = 99
	got.NativeNameAliases[0] = "reader"
	got.NetworkInterfaces[0].Addresses[0] = "reader"
	*got.NetworkInterfaces[0].SpeedMbps = 0
	*got.TemperatureMonitoringEnabled = false
	tasks[0].ID = "reader"
	after, err := json.Marshal(s.GetSnapshot())
	if err != nil || string(before) != string(after) {
		t.Fatal("point reads share mutable source data")
	}
	for _, empty := range []*State{{}, NewState()} {
		if got := empty.GetBackupTasks(); !reflect.DeepEqual(got, empty.GetSnapshot().PVEBackups.BackupTasks) {
			t.Fatal("empty task collection changed")
		}
	}
}

func TestPointReadsUniqueNodeKeepsExactIdentityAndAmbiguity(t *testing.T) {
	for _, tc := range []struct {
		name  string
		nodes []Node
		id    string
		ok    bool
	}{
		{"missing", []Node{{ID: "other"}}, "node", false},
		{"unique", []Node{{ID: "other"}, {ID: "node", Name: "chosen"}}, "node", true},
		{"case-sensitive", []Node{{ID: "NODE"}}, "node", false},
		{"whitespace-sensitive", []Node{{ID: " node "}}, "node", false},
		{"different-duplicates", []Node{{ID: "node", Instance: "a"}, {ID: "node", Instance: "b"}}, "node", false},
		{"identical-duplicates", []Node{{ID: "node", Instance: "a"}, {ID: "node", Instance: "a"}}, "node", false},
		{"duplicates-after-other", []Node{{ID: "node"}, {ID: "other"}, {ID: "node"}}, "node", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := NewState()
			s.UpdateNodes(tc.nodes)
			got, ok := s.GetUniqueNode(tc.id)
			if ok != tc.ok || (!ok && !reflect.DeepEqual(got, Node{})) {
				t.Fatalf("lookup = %+v, %t; want success %t", got, ok, tc.ok)
			}
			var matches []Node
			for _, n := range s.GetSnapshot().Nodes {
				if n.ID == tc.id {
					matches = append(matches, n)
				}
			}
			if (len(matches) == 1) != ok || (ok && !reflect.DeepEqual(got, matches[0])) {
				t.Fatal("point read differs from previous exact-one snapshot rule")
			}
		})
	}
}

func TestPointReadsConcurrentSourceReplacement(t *testing.T) {
	s := metadataStateFixture()
	s.Nodes = []Node{{ID: "node", Instance: "pve", Name: "node-a"}}
	var workers sync.WaitGroup
	workers.Add(2)
	go func() {
		defer workers.Done()
		for i := 0; i < 100; i++ {
			s.UpdateNodes([]Node{{ID: "node", Instance: "pve", Name: "node-a"}})
			s.UpdateBackupTasksForInstance("pve", []BackupTask{{ID: "task", Instance: "pve", VMID: 100}})
		}
	}()
	go func() {
		defer workers.Done()
		for i := 0; i < 100; i++ {
			node, ok := s.GetUniqueNode("node")
			if !ok || node.Instance != "pve" {
				t.Error("unique node lost during locked replacement")
				return
			}
			tasks := s.GetBackupTasks()
			if len(tasks) != 1 || tasks[0].ID != "task" {
				t.Error("task read lost during locked replacement")
				return
			}
			tasks[0].ID = "reader"
		}
	}()
	workers.Wait()
}
