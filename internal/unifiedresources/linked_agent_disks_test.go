package unifiedresources

import (
	"reflect"
	"testing"
	"time"
)

func TestLinkedAgentDiskInventorySourceBoundaries(t *testing.T) {
	now := time.Now()
	seen := now.Add(-5 * time.Second)
	for _, tc := range []struct {
		name, status          string
		stale, missing, empty bool
		seen                  time.Time
		want                  bool
	}{
		{name: "current", status: "online", seen: seen, want: true},
		{name: "offline", status: "offline", seen: seen},
		{name: "expired", status: "stale", seen: seen},
		{name: "agent-stale", status: "online", seen: seen, stale: true},
		{name: "missing-source", seen: seen, missing: true},
		{name: "undated", status: "online"},
		{name: "future", status: "online", seen: now.Add(time.Hour)},
		{name: "empty-inventory", status: "online", seen: seen, empty: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &Resource{LastSeen: now, Agent: &AgentData{Stale: tc.stale, Disks: []DiskInfo{{Total: 1000, Used: 620, Free: 380, Usage: 62, Mountpoint: `C:\`, Device: "C:", Filesystem: "ntfs"}}}, SourceStatus: map[DataSource]SourceStatus{SourceAgent: {Status: tc.status, LastSeen: tc.seen}}}
			if tc.missing {
				delete(r.SourceStatus, SourceAgent)
			}
			if tc.empty {
				r.Agent.Disks = nil
			}
			for _, read := range []func() ([]DiskInfo, time.Time, bool){NewVMView(r).LinkedAgentDisks, NewContainerView(r).LinkedAgentDisks, NewHostView(r).CurrentAgentDisks} {
				disks, stamp, ok := read()
				if ok != tc.want {
					t.Fatalf("inventory eligibility=%v want %v", ok, tc.want)
				}
				if !ok {
					if disks != nil || !stamp.IsZero() {
						t.Fatal("unavailable source leaked current inventory")
					}
					continue
				}
				if !stamp.Equal(seen) || !reflect.DeepEqual(disks, r.Agent.Disks) {
					t.Fatalf("lost source identity/time: %v %+v", stamp, disks)
				}
				disks[0].Used = 999
				if r.Agent.Disks[0].Used != 620 {
					t.Fatal("view aliases owned inventory")
				}
			}
			// Last-known presentation remains available even when selection is denied.
			if !reflect.DeepEqual(NewHostView(r).Disks(), r.Agent.Disks) {
				t.Fatal("selection erased retained inventory")
			}
		})
	}
	for _, read := range []func() ([]DiskInfo, time.Time, bool){(VMView{}).LinkedAgentDisks, (ContainerView{}).LinkedAgentDisks, (HostView{}).CurrentAgentDisks} {
		if disks, stamp, ok := read(); ok || disks != nil || !stamp.IsZero() {
			t.Fatal("nil view invents inventory")
		}
	}
}
