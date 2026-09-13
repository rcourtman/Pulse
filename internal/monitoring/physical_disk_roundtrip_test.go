package monitoring

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// Exercise the real skipped-poll path, not just the ID helper: canonical views
// are converted back into source state and then re-ingested by the adapter.
func TestPhysicalDiskSkippedPollPreservesSourceIdentity(t *testing.T) {
	for _, device := range []string{"/dev/sdx", "sdx"} {
		t.Run(device, func(t *testing.T) {
			state := models.NewState()
			nodes := []models.Node{{ID: "pve-node1", Name: "node1", Instance: "pve"}, {ID: "pve-node2", Name: "node2", Instance: "pve"}}
			state.UpdateNodesForInstance("pve", nodes)
			var disks []models.PhysicalDisk
			for _, fixture := range []struct{ node, target string }{{"node1", ""}, {"node2", ""}, {"node1", "megaraid,0"}, {"node1", "megaraid,1"}} {
				disks = append(disks, models.PhysicalDisk{
					ID:       unifiedresources.ProxmoxPhysicalDiskSourceID("pve", fixture.node, device, "", fixture.target),
					Instance: "pve", Node: fixture.node, DevPath: device, Target: fixture.target,
					Model: "USB fixture", Size: 1024, Temperature: 37, Wearout: -1, LastChecked: time.Now(),
					ExpectedUpdateInterval: 5 * time.Minute,
				})
			}
			state.UpdatePhysicalDisks("pve", disks)
			adapter := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
			adapter.PopulateFromSnapshot(state.GetSnapshot())
			m := &Monitor{state: state, resourceStore: adapter, lastPhysicalDiskPoll: map[string]time.Time{"pve": time.Now()}}
			canonical := map[string]bool{}
			for _, view := range adapter.PhysicalDisks() {
				canonical[view.ID()] = true
			}
			if len(canonical) != len(disks) {
				t.Fatalf("initial disks = %d, want %d", len(canonical), len(disks))
			}
			for cycle := 0; cycle < 8; cycle++ {
				m.maybePollPhysicalDisksAsync(context.Background(), "pve", &config.PVEInstance{}, nil, nil, nil, nil)
				got := state.GetSnapshot().PhysicalDisks
				if len(got) != len(disks) {
					t.Fatalf("cycle %d: source count %d", cycle, len(got))
				}
				wantIDs := map[string]bool{}
				for _, disk := range disks {
					wantIDs[disk.ID] = true
				}
				for _, disk := range got {
					if !wantIDs[disk.ID] {
						t.Fatalf("cycle %d: canonical ID leaked into provider state: %q", cycle, disk.ID)
					}
					if disk.DevPath != device || disk.Temperature != 37 || disk.Size != 1024 || disk.ExpectedUpdateInterval != 5*time.Minute {
						t.Fatalf("cycle %d: metadata lost: %+v", cycle, disk)
					}
				}
				adapter.PopulateFromSnapshot(state.GetSnapshot())
				views := adapter.PhysicalDisks()
				if len(views) != len(disks) {
					t.Fatalf("cycle %d: view count = %d", cycle, len(views))
				}
				for _, view := range views {
					if !canonical[view.ID()] {
						t.Fatalf("cycle %d: canonical identity churn: %s", cycle, view.ID())
					}
				}
				// Retain the JSON-visible projection as well as the typed read-state checks.
				payload, err := json.Marshal(adapter.GetAll())
				if err != nil {
					t.Fatal(err)
				}
				var resources []unifiedresources.Resource
				if err := json.Unmarshal(payload, &resources); err != nil {
					t.Fatal(err)
				}
				count := 0
				for _, resource := range resources {
					if resource.Type != unifiedresources.ResourceTypePhysicalDisk {
						continue
					}
					count++
					if !canonical[resource.ID] || resource.Proxmox == nil || !wantIDs[resource.Proxmox.SourceID] || resource.PhysicalDisk == nil || resource.PhysicalDisk.Temperature != 37 {
						t.Fatalf("cycle %d: JSON disk identity/metadata lost: %+v", cycle, resource)
					}
				}
				if count != len(disks) {
					t.Fatalf("cycle %d: JSON disk count %d", cycle, count)
				}
			}
			// A confirmed full inventory removal must still remove source-owned disks.
			state.UpdatePhysicalDisks("pve", nil)
			adapter.PopulateFromSnapshot(state.GetSnapshot())
			if got := len(adapter.PhysicalDisks()); got != 0 {
				t.Fatalf("removed inventory retained %d disks", got)
			}
		})
	}
}

func TestPhysicalDiskReadbackSourceIDFallback(t *testing.T) {
	for _, resource := range []unifiedresources.Resource{
		{ID: "canonical"},
		{ID: "canonical", Proxmox: &unifiedresources.ProxmoxData{}},
		{ID: "canonical", Proxmox: &unifiedresources.ProxmoxData{SourceID: " native "}},
	} {
		view := unifiedresources.NewPhysicalDiskView(&resource)
		want := "canonical"
		if resource.Proxmox != nil && resource.Proxmox.SourceID != "" {
			want = "native"
		}
		if got := physicalDiskFromReadStateView(&view).ID; got != want {
			t.Fatalf("ID = %q, want %q", got, want)
		}
	}
	var view unifiedresources.PhysicalDiskView
	if view.SourceID() != "" {
		t.Fatal("nil resource has a source ID")
	}
}
