package monitoring

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// Keep the ordinary adapter's VM/container views while supplying immutable
// host views. This also admits nil and unknown-status inputs that a registry
// would normally discard or normalize before the poller sees them.
type previousHostProjectionStore struct {
	*unifiedresources.MonitorAdapter
	hosts []*unifiedresources.HostView
}

func (s *previousHostProjectionStore) Hosts() []*unifiedresources.HostView { return s.hosts }

func previousHostProjectionFixture(hosts, disks int) (*Monitor, *previousHostProjectionStore) {
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(models.StateSnapshot{
		VMs:        []models.VM{{ID: "site-a:node:105", Instance: "site-a", Node: "node", VMID: 105, Name: "vm", Type: "qemu", Status: "running"}},
		Containers: []models.Container{{ID: "site-a:node:106", Instance: "site-a", Node: "node", VMID: 106, Name: "ct", Status: "running"}},
	})
	store := &previousHostProjectionStore{MonitorAdapter: unifiedresources.NewMonitorAdapter(registry)}
	for i := 0; i < hosts; i++ {
		store.hosts = append(store.hosts, previousHostProjectionView(fmt.Sprint(i), unifiedresources.StatusOnline, "", "", disks))
	}
	m := &Monitor{state: models.NewState(), resourceStore: store}
	// The cost being measured is repeated previous-state assembly, not the
	// one-time registry ingestion/cache build, which this repair does not alter.
	_ = m.previousGuestContextForInstance("site-a")
	return m, store
}

func previousHostProjectionView(id string, status unifiedresources.ResourceStatus, vm, ct string, disks int) *unifiedresources.HostView {
	seen := time.Now().Add(-time.Second)
	r := &unifiedresources.Resource{
		ID: "host-" + id, Name: "host-" + id, Type: unifiedresources.ResourceTypeAgent,
		Status: status, LastSeen: seen,
		SourceStatus: map[unifiedresources.DataSource]unifiedresources.SourceStatus{
			unifiedresources.SourceAgent: {Status: "online", LastSeen: seen},
		},
		Agent: &unifiedresources.AgentData{AgentID: "agent-" + id, LinkedVMID: vm, LinkedContainerID: ct},
	}
	for i := 0; i < disks; i++ {
		r.Agent.Disks = append(r.Agent.Disks, unifiedresources.DiskInfo{Mountpoint: fmt.Sprintf("/volume/%d", i), Total: 1000, Used: 300, Free: 700, Usage: 30})
	}
	v := unifiedresources.NewHostView(r)
	return &v
}

func TestPreviousGuestContextHostProjectionParity(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status unifiedresources.ResourceStatus
		vm, ct string
	}{
		{name: "unlinked-online", status: unifiedresources.StatusOnline},
		{name: "whitespace-only", status: unifiedresources.StatusOnline, vm: " \t", ct: "\n"},
		{name: "linked-offline", status: unifiedresources.StatusOffline, vm: "site-a:node:105"},
		{name: "linked-unknown", vm: "site-a:node:105"},
		{name: "linked-warning", status: "warning", ct: "site-a:node:106"},
		{name: "linked-vm", status: unifiedresources.StatusOnline, vm: " site-a:node:105 "},
		{name: "linked-container", status: unifiedresources.StatusOnline, ct: " site-a:node:106 "},
		{name: "both-links", status: unifiedresources.StatusOnline, vm: "site-a:node:105", ct: "site-a:node:106"},
		{name: "other-instance", status: unifiedresources.StatusOnline, vm: "site-b:node:105"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, store := previousHostProjectionFixture(0, 0)
			// VM/container assembly stays on the ordinary producer. Add the old
			// host-loop result as an independent selection/ownership reference.
			want := m.previousGuestContextForInstance("site-a")
			view := previousHostProjectionView(tc.name, tc.status, tc.vm, tc.ct, 3)
			store.hosts = []*unifiedresources.HostView{nil, new(unifiedresources.HostView), view}
			legacyHost := previousHostFromView(view)
			if legacyHost.Status == "online" {
				if legacyHost.LinkedVMID != "" {
					want.hostAgentsByVMID[legacyHost.LinkedVMID] = legacyHost
				}
				if legacyHost.LinkedContainerID != "" {
					want.hostAgentsByVMID[legacyHost.LinkedContainerID] = legacyHost
				}
			}
			before, err := json.Marshal(view.Disks())
			if err != nil {
				t.Fatal(err)
			}
			got := m.previousGuestContextForInstance("site-a")
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("host prefilter changed previous-state output: got %+v, want %+v", got.hostAgentsByVMID, want.hostAgentsByVMID)
			}
			for key, host := range got.hostAgentsByVMID {
				if len(host.Disks) == 0 {
					t.Fatalf("linked host %q lost current disk evidence", key)
				}
				host.Disks[0].Used = 999
			}
			after, err := json.Marshal(view.Disks())
			if err != nil || string(before) != string(after) {
				t.Fatal("previous-state output aliases the host's disk evidence")
			}
		})
	}
}

func TestPreviousGuestContextUnlinkedHostAllocationBound(t *testing.T) {
	small, _ := previousHostProjectionFixture(0, 0)
	large, store := previousHostProjectionFixture(128, 32)
	if len(store.Hosts()) != 128 {
		t.Fatal("large fixture lost its unrelated hosts")
	}
	want := small.previousGuestContextForInstance("site-a")
	got := large.previousGuestContextForInstance("site-a")
	if !reflect.DeepEqual(got, want) {
		t.Fatal("unrelated hosts changed the guest carry-over input")
	}
	smallAllocs := testing.AllocsPerRun(10, func() { _ = small.previousGuestContextForInstance("site-a") })
	largeAllocs := testing.AllocsPerRun(10, func() { _ = large.previousGuestContextForInstance("site-a") })
	t.Logf("previous guest context allocations: no unrelated hosts %.0f, 128 unrelated hosts/32 disks each %.0f", smallAllocs, largeAllocs)
	if largeAllocs > smallAllocs+4 {
		t.Fatalf("guest carry-over allocated for discarded host disks: %.0f -> %.0f", smallAllocs, largeAllocs)
	}
}

func TestPreviousGuestContextLinkedHostWithdrawalAndRecovery(t *testing.T) {
	m, store := previousHostProjectionFixture(0, 0)
	view := previousHostProjectionView("linked", unifiedresources.StatusOnline, "site-a:node:105", "", 2)
	for _, hosts := range [][]*unifiedresources.HostView{{view}, nil, {view}} {
		store.hosts = hosts
		got := m.previousGuestContextForInstance("site-a")
		if len(got.hostAgentsByVMID) != len(hosts) {
			t.Fatal("host removal or ordinary return was not reflected in the poller's next input")
		}
		if len(hosts) > 0 {
			host := got.hostAgentsByVMID["site-a:node:105"]
			if !host.LastSeen.Equal(view.LastSeen()) || len(host.Disks) != 2 {
				t.Fatal("linked evidence changed timestamp or lost disks")
			}
		}
	}
}

func BenchmarkPreviousGuestContextHostProjection(b *testing.B) {
	for _, tc := range []struct{ hosts, disks int }{{0, 0}, {34, 16}, {200, 64}} {
		b.Run(fmt.Sprintf("hosts-%d/disks-%d", tc.hosts, tc.disks), func(b *testing.B) {
			m, _ := previousHostProjectionFixture(tc.hosts, tc.disks)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = m.previousGuestContextForInstance("site-a")
			}
		})
	}
}
