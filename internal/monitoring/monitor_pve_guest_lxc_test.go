package monitoring

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/rs/zerolog"
)

func TestGuestMemoryCarryForwardExpiryContract(t *testing.T) {
	t.Run("original-age", testGuestMemoryCarryForwardOriginalAge)
	t.Run("repeated-polls", testGuestMemoryCarryForwardRepeatedPollsDoNotExtendAge)
	t.Run("ordinary-deferral-recovery", testGuestMemoryCarryForwardOrdinaryDeferralAndRecovery)
}

type stubPVEClientLXCStatus struct {
	stubPVEClient

	containerStatus *proxmox.Container
	statusCalls     int
}

type issue2757ContainerClient struct {
	stubPVEClient
	status                   *proxmox.Container
	config                   map[string]interface{}
	interfaces               []proxmox.ContainerInterface
	statusCalls, configCalls int
	interfaceCalls           int
}

func (c *issue2757ContainerClient) GetContainerStatus(context.Context, string, int) (*proxmox.Container, error) {
	c.statusCalls++
	return c.status, nil
}

func (c *issue2757ContainerClient) GetContainerConfig(context.Context, string, int) (map[string]interface{}, error) {
	c.configCalls++
	return c.config, nil
}

func (c *issue2757ContainerClient) GetContainerInterfaces(context.Context, string, int) ([]proxmox.ContainerInterface, error) {
	c.interfaceCalls++
	return c.interfaces, nil
}

// These are source fixtures, not the reporter's inventory or native recovery.
// All addresses are synthetic; choosing one says nothing about its default route.
func TestIssue2757ContainerAddressSelection(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name           string
		status         *proxmox.Container
		config         map[string]interface{}
		interfaces     []proxmox.ContainerInterface
		stopped        bool
		want           []string
		wantIfaceCalls int
	}{
		{
			name: "status interface association beats flattened text order",
			status: &proxmox.Container{IP: "10.88.0.1 192.0.2.80", Network: map[string]proxmox.ContainerNetworkConfig{
				"net1": {Name: "podman0", IP: "10.88.0.1/16"},
				"net0": {Name: "eth0", IP: "192.0.2.80/24"},
			}},
			want: []string{"192.0.2.80", "10.88.0.1"},
		},
		{
			name: "stopped config-only guest needs no runtime query",
			config: map[string]interface{}{
				"net1": "name=podman0,ip=10.88.0.1/16",
				"net0": "name=eth0,ip=192.0.2.80/24",
			},
			stopped: true,
			want:    []string{"192.0.2.80", "10.88.0.1"},
		},
		{
			name:   "DHCP interfaces fallback keeps named addresses and filters loopback",
			config: map[string]interface{}{"net0": "name=eth0,ip=dhcp,ip6=auto"},
			interfaces: []proxmox.ContainerInterface{
				{Name: "podman0", Inet: "10.88.0.1/16"},
				{Name: "veth0", IPAddresses: []proxmox.ContainerInterfaceAddress{{Address: "10.89.0.1/16"}}},
				{Name: "eth0", IPAddresses: []proxmox.ContainerInterfaceAddress{{Address: "192.0.2.80/24"}, {Address: "2001:db8::80/64"}, {Address: "fe80::1/64"}}},
				{Name: "lo", Inet: "127.0.0.1/8 ::1/128"},
			},
			want:           []string{"192.0.2.80", "2001:db8::80", "10.88.0.1", "10.89.0.1"},
			wantIfaceCalls: 1,
		},
		{
			name: "IPv6 on preferred interface precedes secondary IPv4",
			status: &proxmox.Container{Network: map[string]proxmox.ContainerNetworkConfig{
				"podman0": {IP: "10.88.0.1"}, "eth0": {IP6: "2001:db8::80"},
			}},
			want: []string{"2001:db8::80", "10.88.0.1"},
		},
		{
			name: "secondary-only guest keeps useful addresses",
			status: &proxmox.Container{Network: map[string]proxmox.ContainerNetworkConfig{
				"podman0": {IP: "10.88.0.1"}, "docker0": {IP: "192.0.2.80"},
			}},
			want: []string{"192.0.2.80", "10.88.0.1"},
		},
		{
			name: "management bridge is not discarded or treated as a container bridge",
			status: &proxmox.Container{Network: map[string]proxmox.ContainerNetworkConfig{
				"podman0": {IP: "10.88.0.1"}, "br0": {IP: "192.0.2.80"},
			}},
			want: []string{"192.0.2.80", "10.88.0.1"},
		},
		{
			name: "unassociated status IP stays useful without inventing its interface",
			status: &proxmox.Container{IP: "192.0.2.80", Network: map[string]proxmox.ContainerNetworkConfig{
				"podman0": {IP: "10.88.0.1"},
			}},
			want: []string{"192.0.2.80", "10.88.0.1"},
		},
		{
			name:   "empty preferred interface does not hide the only address",
			config: map[string]interface{}{"net0": "name=eth0,ip=dhcp"},
			status: &proxmox.Container{Network: map[string]proxmox.ContainerNetworkConfig{
				"podman0": {IP: "10.88.0.1"},
			}},
			want: []string{"10.88.0.1"},
		},
		{
			name: "deduplicate globally but sort numerically only within each interface",
			status: &proxmox.Container{Network: map[string]proxmox.ContainerNetworkConfig{
				"eth0": {IP: "192.0.2.10 192.0.2.2", IP6: "2001:db8::10 2001:db8::2"},
				"eth1": {IP: "10.0.0.1"}, "podman0": {IP: "192.0.2.2 10.88.0.1"},
			}},
			want: []string{"192.0.2.2", "192.0.2.10", "2001:db8::2", "2001:db8::10", "10.0.0.1", "10.88.0.1"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client := &issue2757ContainerClient{status: tc.status, config: tc.config, interfaces: tc.interfaces}
			before, err := json.Marshal([]interface{}{client.status, client.config, client.interfaces})
			if err != nil {
				t.Fatal(err)
			}
			container := models.Container{ID: "site-a-node-a-2757", VMID: 2757, Name: "guest", Instance: "site-a", Node: "node-a", Status: "running", Type: "lxc", LastSeen: time.Now()}
			if tc.stopped {
				container.Status = "stopped"
			}
			monitor := &Monitor{}
			monitor.enrichContainerMetadata(context.Background(), client, "site-a", "node-a", &container)
			if !reflect.DeepEqual(container.IPAddresses, tc.want) {
				t.Fatalf("guest IP selection = %v, want %v", container.IPAddresses, tc.want)
			}
			wantStatusCalls := 1
			if tc.stopped {
				wantStatusCalls = 0
			}
			if client.configCalls != 1 || client.statusCalls != wantStatusCalls || client.interfaceCalls != tc.wantIfaceCalls {
				t.Fatalf("metadata query counts = status:%d config:%d interfaces:%d", client.statusCalls, client.configCalls, client.interfaceCalls)
			}
			after, err := json.Marshal([]interface{}{client.status, client.config, client.interfaces})
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("client changed while enriching")
			}
			monitor.state = models.NewState()
			monitor.state.UpdateContainers([]models.Container{container})
			assertIssue2757GuestWire(t, monitor, container.ID, tc.want, container.NetworkInterfaces)
		})
	}
}

type issue2757VMClient struct {
	emptyGuestMetadataClient
	interfaces []proxmox.VMNetworkInterface
	calls      int
}

func (c *issue2757VMClient) GetVMNetworkInterfaces(context.Context, string, int) ([]proxmox.VMNetworkInterface, error) {
	c.calls++
	return c.interfaces, nil
}

func TestIssue2757VMAddressSelectionAndCache(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		interfaces []proxmox.VMNetworkInterface
		want       []string
	}{
		{
			name: "prefer named interface even with lower secondary addresses",
			interfaces: []proxmox.VMNetworkInterface{
				{Name: "podman0", IPAddresses: []proxmox.VMIPAddress{{Address: "10.88.0.1"}}},
				{Name: "eth1", IPAddresses: []proxmox.VMIPAddress{{Address: "10.0.0.1"}}},
				{Name: "eth0", IPAddresses: []proxmox.VMIPAddress{{Address: "192.0.2.10"}, {Address: "192.0.2.2"}, {Address: "2001:db8::10"}, {Address: "2001:db8::2"}, {Address: "192.0.2.2"}, {Address: "fe80::1"}}},
			},
			want: []string{"192.0.2.2", "192.0.2.10", "2001:db8::2", "2001:db8::10", "10.0.0.1", "10.88.0.1"},
		},
		{
			name: "secondary-only VM keeps every useful address",
			interfaces: []proxmox.VMNetworkInterface{
				{Name: "podman0", IPAddresses: []proxmox.VMIPAddress{{Address: "10.88.0.1"}}},
				{Name: "docker0", IPAddresses: []proxmox.VMIPAddress{{Address: "192.0.2.80"}}},
			},
			want: []string{"192.0.2.80", "10.88.0.1"},
		},
		{
			name: "management bridge remains a usable first interface",
			interfaces: []proxmox.VMNetworkInterface{
				{Name: "podman0", IPAddresses: []proxmox.VMIPAddress{{Address: "10.88.0.1"}}},
				{Name: "br0", HardwareAddr: "02:00:00:00:00:01", IPAddresses: []proxmox.VMIPAddress{{Address: "192.0.2.80"}}, Statistics: map[string]interface{}{"rx-bytes": float64(123), "tx-bytes": float64(456)}},
			},
			want: []string{"192.0.2.80", "10.88.0.1"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for permutation := 0; permutation < 2; permutation++ {
				raw := append([]proxmox.VMNetworkInterface(nil), tc.interfaces...)
				if permutation == 1 {
					for i, j := 0, len(raw)-1; i < j; i, j = i+1, j-1 {
						raw[i], raw[j] = raw[j], raw[i]
					}
				}
				before, err := json.Marshal(raw)
				if err != nil {
					t.Fatal(err)
				}
				client := &issue2757VMClient{interfaces: raw}
				monitor := &Monitor{}
				status := &proxmox.VMStatus{Agent: proxmox.VMAgentField{Value: 1}}
				for poll := 0; poll < 2; poll++ {
					ips, ifaces, _, _, _, deferred := monitor.fetchGuestAgentMetadata(context.Background(), client, "site-a", "node-a", "guest", 2757, status, false)
					if deferred || !reflect.DeepEqual(ips, tc.want) {
						t.Fatalf("guest IP selection = %v, deferred:%v, want %v", ips, deferred, tc.want)
					}
					monitor.state = models.NewState()
					monitor.state.UpdateVMs([]models.VM{{ID: "site-a-node-a-2757", VMID: 2757, Name: "guest", Instance: "site-a", Node: "node-a", Status: "running", LastSeen: time.Now(), IPAddresses: ips, NetworkInterfaces: ifaces}})
					assertIssue2757GuestWire(t, monitor, "site-a-node-a-2757", tc.want, ifaces)
				}
				if client.calls != 1 {
					t.Fatalf("fresh cache issued %d network queries, want one", client.calls)
				}
				after, err := json.Marshal(raw)
				if err != nil || !bytes.Equal(before, after) {
					t.Fatal("guest-agent input was mutated")
				}
			}
		})
	}
}

func assertIssue2757GuestWire(t *testing.T, monitor *Monitor, nativeID string, want []string, interfaces []models.GuestNetworkInterface) {
	t.Helper()
	monitor.state.UpdateNodes([]models.Node{{ID: "site-a-node-a", Name: "node-a", Instance: "site-a", Status: "online", LastSeen: time.Now()}})
	monitor.resourceStore = unifiedresources.NewMonitorAdapter(nil)
	frontend := monitor.BuildBroadcastFrontendState()
	var listed *unifiedresources.Resource
	for _, resource := range monitor.GetUnifiedResources() {
		if resource.Proxmox != nil && resource.Proxmox.SourceID == nativeID {
			copy := resource
			listed = &copy
		}
	}
	if listed == nil || !reflect.DeepEqual(listed.Identity.IPAddresses, want) {
		t.Fatalf("canonical listing lost guest addresses: %+v, want %v", listed, want)
	}
	var projected *models.ResourceFrontend
	for _, resource := range frontend.Resources {
		if resource.ID == listed.ID {
			copy := resource
			projected = &copy
		}
	}
	if projected == nil || projected.Identity == nil || !reflect.DeepEqual(projected.Identity.IPs, want) {
		t.Fatalf("broadcast lost guest address order: %+v, want %v", projected, want)
	}
	var facet unifiedresources.ProxmoxData
	if err := json.Unmarshal(projected.Proxmox, &facet); err != nil {
		t.Fatal(err)
	}
	if facet.SourceID != nativeID || facet.Instance != "site-a" || facet.NodeName != "node-a" || facet.VMID != 2757 || listed.ParentID == nil || projected.ParentID != *listed.ParentID {
		t.Fatalf("broadcast changed guest ownership: %+v, listing parent:%v broadcast parent:%q", facet, listed.ParentID, projected.ParentID)
	}
	parentFound := false
	for _, resource := range monitor.GetUnifiedResources() {
		if resource.ID == *listed.ParentID && resource.Proxmox != nil && resource.Proxmox.SourceID == "site-a-node-a" {
			parentFound = true
		}
	}
	if !parentFound {
		t.Fatalf("guest parent does not name the supplying node: %s", *listed.ParentID)
	}
	if len(facet.NetworkInterfaces) != len(interfaces) || len(listed.Proxmox.NetworkInterfaces) != len(interfaces) {
		t.Fatalf("listing/broadcast lost named interfaces: %+v", facet.NetworkInterfaces)
	}
	for i, iface := range interfaces {
		// Empty address collections are omitted on the JSON wire and may be
		// normalised to [] in memory. Compare their values on both surfaces;
		// keep every name/address/traffic/owner assertion, not nil-vs-empty.
		for _, wire := range []unifiedresources.NetworkInterface{listed.Proxmox.NetworkInterfaces[i], facet.NetworkInterfaces[i]} {
			if wire.Name != iface.Name || wire.MAC != iface.MAC || !slices.Equal(wire.Addresses, iface.Addresses) || wire.RXBytes != uint64(max(0, iface.RXBytes)) || wire.TXBytes != uint64(max(0, iface.TXBytes)) {
				t.Fatalf("interface association or traffic changed: %+v, want %+v", wire, iface)
			}
		}
	}
}

func (s *stubPVEClientLXCStatus) GetContainerStatus(ctx context.Context, node string, vmid int) (*proxmox.Container, error) {
	s.statusCalls++
	return s.containerStatus, nil
}

func TestMergeContainerRuntimeCounters_PrefersNewerStatusCounters(t *testing.T) {
	t.Parallel()

	current := IOMetrics{
		DiskRead:     0,
		DiskWrite:    8,
		NetworkIn:    12,
		NetworkOut:   0,
		Timestamp:    time.Unix(0, 0),
		SourceUptime: 100,
	}

	merged := mergeContainerRuntimeCounters(current, &proxmox.Container{
		DiskRead:  128,
		DiskWrite: 4,
		NetIn:     10,
		NetOut:    256,
		Uptime:    1,
	})

	if merged.DiskRead != 128 {
		t.Fatalf("expected DiskRead to upgrade from status snapshot, got %d", merged.DiskRead)
	}
	if merged.DiskWrite != 4 {
		t.Fatalf("expected DiskWrite to follow the newer reset counter, got %d", merged.DiskWrite)
	}
	if merged.NetworkIn != 10 {
		t.Fatalf("expected NetworkIn to follow the newer reset counter, got %d", merged.NetworkIn)
	}
	if merged.NetworkOut != 256 {
		t.Fatalf("expected NetworkOut to upgrade from status snapshot, got %d", merged.NetworkOut)
	}
}

func TestMergeContainerRuntimeCounters_OverridesOnlyPresentStatusFields(t *testing.T) {
	t.Parallel()

	listingObservedAt := time.Unix(10, 0)
	current := IOMetrics{
		DiskRead:   8,
		DiskWrite:  16,
		NetworkIn:  32,
		NetworkOut: 64,
		Timestamp:  listingObservedAt,
		ObservedAt: counterObservationTimes(listingObservedAt),
		Presence: models.IOCounterPresence{
			Explicit:   true,
			DiskRead:   true,
			DiskWrite:  true,
			NetworkIn:  true,
			NetworkOut: true,
		},
		SourceUptime: 100,
	}
	status := &proxmox.Container{
		DiskRead:  0,
		DiskWrite: 999,
		IOCounters: proxmox.IOCounterPresence{
			Explicit: true,
			DiskRead: true,
		},
		ObservedAt: time.Unix(20, 0),
		Uptime:     1,
	}

	merged := mergeContainerRuntimeCounters(current, status)
	if merged.DiskRead != 0 {
		t.Fatalf("explicit status zero was not authoritative: %d", merged.DiskRead)
	}
	if merged.DiskWrite != 16 || merged.NetworkIn != 32 || merged.NetworkOut != 64 {
		t.Fatalf("missing status fields overwrote listing counters: %+v", merged)
	}
	if !merged.Timestamp.Equal(status.ObservedAt) {
		t.Fatalf("timestamp = %v, want status receipt time %v", merged.Timestamp, status.ObservedAt)
	}
	if !merged.ObservedAt.DiskRead.Equal(status.ObservedAt) {
		t.Fatalf("disk-read receipt = %v, want status receipt %v", merged.ObservedAt.DiskRead, status.ObservedAt)
	}
	if !merged.ObservedAt.DiskWrite.Equal(listingObservedAt) ||
		!merged.ObservedAt.NetworkIn.Equal(listingObservedAt) ||
		!merged.ObservedAt.NetworkOut.Equal(listingObservedAt) {
		t.Fatalf("missing status fields lost listing receipt times: %+v", merged.ObservedAt)
	}
}

func TestIssue1613LXCStatusLagDoesNotEraseNewerListingDiskWrite(t *testing.T) {
	t.Parallel()

	monitor := &Monitor{rateTracker: NewRateTracker()}
	client := &stubPVEClientLXCStatus{
		containerStatus: &proxmox.Container{
			Status:    "running",
			DiskWrite: 0,
			Uptime:    600,
			IOCounters: proxmox.IOCounterPresence{
				Explicit:  true,
				DiskWrite: true,
			},
			ObservedAt: time.Unix(1_700_000_001, 0),
		},
	}
	resource := proxmox.ClusterResource{
		Type:       "lxc",
		Node:       "pve-a",
		Name:       "write-test",
		Status:     "running",
		VMID:       203,
		Uptime:     600,
		MaxMem:     4096,
		Mem:        2048,
		ObservedAt: time.Unix(1_700_000_000, 0),
		IOCounters: proxmox.IOCounterPresence{
			Explicit:  true,
			DiskWrite: true,
		},
	}

	if _, _, _, _, ok := monitor.buildContainerFromClusterResource(
		context.Background(), "cluster-a", resource, client, map[int]bool{}, nil,
	); !ok {
		t.Fatal("expected first LXC sample")
	}

	resource.DiskWrite = 90 * 1024 * 1024
	resource.ObservedAt = resource.ObservedAt.Add(120 * time.Second)
	client.containerStatus.ObservedAt = resource.ObservedAt.Add(time.Second)

	container, _, _, _, ok := monitor.buildContainerFromClusterResource(
		context.Background(), "cluster-a", resource, client, map[int]bool{}, nil,
	)
	if !ok {
		t.Fatal("expected second LXC sample")
	}
	if container.DiskWrite <= 0 {
		t.Fatalf("LXC disk write rate = %d, lagging status/current erased the listing counter", container.DiskWrite)
	}
	// The first authoritative zero came from status/current one second after
	// the listing, while the changed counter came from the next listing.
	const want = 90 * 1024 * 1024 / 119
	if container.DiskWrite != want {
		t.Fatalf("LXC disk write rate = %d B/s, want %d B/s", container.DiskWrite, want)
	}
}

func TestBuildContainerFromClusterResource_UsesContainerStatusCountersForRates(t *testing.T) {
	t.Parallel()

	monitor := &Monitor{rateTracker: NewRateTracker()}
	client := &stubPVEClientLXCStatus{
		containerStatus: &proxmox.Container{
			Status:    "running",
			DiskRead:  4096,
			DiskWrite: 2048,
			NetIn:     1024,
			NetOut:    512,
		},
	}

	resource := proxmox.ClusterResource{
		Type:    "lxc",
		Node:    "pve-a",
		Name:    "cache-ct",
		Status:  "running",
		VMID:    202,
		MaxCPU:  2,
		MaxMem:  4096,
		Mem:     2048,
		MaxDisk: 32 * 1024 * 1024 * 1024,
		Disk:    8 * 1024 * 1024 * 1024,
	}

	if _, _, _, _, ok := monitor.buildContainerFromClusterResource(
		context.Background(),
		"cluster-a",
		resource,
		client,
		map[int]bool{},
		nil,
	); !ok {
		t.Fatal("expected first container sample to be built")
	}

	time.Sleep(20 * time.Millisecond)

	client.containerStatus = &proxmox.Container{
		Status:    "running",
		DiskRead:  8192,
		DiskWrite: 4096,
		NetIn:     2048,
		NetOut:    1024,
	}

	container, _, _, _, ok := monitor.buildContainerFromClusterResource(
		context.Background(),
		"cluster-a",
		resource,
		client,
		map[int]bool{},
		nil,
	)
	if !ok {
		t.Fatal("expected second container sample to be built")
	}
	if client.statusCalls < 2 {
		t.Fatalf("expected container status to be queried for running LXC samples, got %d calls", client.statusCalls)
	}
	if container.DiskRead <= 0 {
		t.Fatalf("expected DiskRead rate from container status counters, got %d", container.DiskRead)
	}
	if container.DiskWrite <= 0 {
		t.Fatalf("expected DiskWrite rate from container status counters, got %d", container.DiskWrite)
	}
	if container.NetworkIn <= 0 {
		t.Fatalf("expected NetworkIn rate from container status counters, got %d", container.NetworkIn)
	}
	if container.NetworkOut <= 0 {
		t.Fatalf("expected NetworkOut rate from container status counters, got %d", container.NetworkOut)
	}
}

type stubPVEClientLXCRRD struct {
	stubPVEClient

	lxcRRDCalls int
}

// GetLXCRRDData tracks lookups of the guest RRD endpoint. It is intentionally
// not part of PVEClientInterface anymore: guest rrddata carries only the
// cache-inclusive mem/maxmem columns (#1634), so the LXC memory path must not
// consult it.
func (s *stubPVEClientLXCRRD) GetLXCRRDData(ctx context.Context, node string, vmid int, timeframe, cf string, ds []string) ([]proxmox.GuestRRDPoint, error) {
	s.lxcRRDCalls++
	return nil, nil
}

// Issue #1634: real PVE guest RRD responses carry only mem/maxmem, never the
// cache-aware memused/memavailable columns, so running containers use the
// cluster-resources listing value instead of reporting memory as unavailable
// (rendered as 0%). The guest RRD lookup is gone entirely — it could never
// produce memory evidence.
func TestIssue1634LXCMemoryFallsBackToClusterResourcesOnRealRRDShape(t *testing.T) {
	t.Parallel()

	client := &stubPVEClientLXCRRD{}

	monitor := &Monitor{rateTracker: NewRateTracker()}
	resource := proxmox.ClusterResource{
		Type:   "lxc",
		Node:   "pve-a",
		Name:   "issue1634-ct",
		Status: "running",
		VMID:   108,
		MaxMem: 8 * 1024 * 1024 * 1024,
		Mem:    335716352,
	}

	container, _, memorySource, _, ok := monitor.buildContainerFromClusterResource(
		context.Background(),
		"cluster-a",
		resource,
		client,
		map[int]bool{},
		nil,
	)
	if !ok {
		t.Fatal("expected container sample to be built")
	}
	if CanonicalMemorySource(memorySource) != "cluster-resources" {
		t.Fatalf("memory source = %q, want cluster-resources fallback", memorySource)
	}
	if container.Memory.UsageUnavailable {
		t.Fatal("running LXC memory marked unavailable despite listing value")
	}
	if container.Memory.Used != int64(resource.Mem) {
		t.Fatalf("memory used = %d, want listing value %d", container.Memory.Used, resource.Mem)
	}
	if container.Memory.Usage <= 0 {
		t.Fatalf("memory usage = %f, want > 0", container.Memory.Usage)
	}
	if !container.Memory.HasKnownUsage() {
		t.Fatal("expected fallback memory to be usable for projections")
	}
	if client.lxcRRDCalls != 0 {
		t.Fatalf("expected no guest RRD lookups for LXC memory, got %d", client.lxcRRDCalls)
	}
}

func TestIssue1634LXCMemoryStaysUnavailableWithoutListingValue(t *testing.T) {
	t.Parallel()

	client := &stubPVEClientLXCRRD{}

	monitor := &Monitor{rateTracker: NewRateTracker()}
	resource := proxmox.ClusterResource{
		Type:   "lxc",
		Node:   "pve-a",
		Name:   "issue1634-no-mem",
		Status: "running",
		VMID:   110,
		MaxMem: 512 * 1024 * 1024,
	}

	container, _, memorySource, _, ok := monitor.buildContainerFromClusterResource(
		context.Background(),
		"cluster-a",
		resource,
		client,
		map[int]bool{},
		nil,
	)
	if !ok {
		t.Fatal("expected container sample to be built")
	}
	if CanonicalMemorySource(memorySource) != "unavailable" {
		t.Fatalf("memory source = %q, want unavailable when no evidence exists", memorySource)
	}
	if !container.Memory.UsageUnavailable {
		t.Fatal("expected memory to stay marked unavailable without any usage evidence")
	}
}

func TestIssue1477ConfigOnlyMountsSurviveIntoContainerDisks(t *testing.T) {
	t.Parallel()

	// Stock PVE reports no per-mount usage through the LXC status API, so the
	// only record of an mpX mount is the container config. The API path must
	// still surface it: capacity from size=, usage unknown (-1), and the
	// aggregate-seeded rootfs row (live usage) must survive the merge.
	metadata := parseContainerMountMetadata(map[string]interface{}{
		"rootfs": "local-lvm:vm-106-disk-0,size=59G",
		"mp0":    "tank:subvol-106-disk-1,mp=/srv/archive,size=20G",
	})

	discovered := convertContainerDiskInfo(nil, metadata)
	if len(discovered) != 2 {
		t.Fatalf("discovered disks = %+v, want rootfs + mp0", discovered)
	}

	seededRootfs := models.Disk{
		Total:      63350767616,
		Used:       23530764893,
		Free:       39820002723,
		Usage:      37.14,
		Mountpoint: "/",
		Type:       "rootfs",
	}
	merged := mergeContainerDisksPreservingExisting([]models.Disk{seededRootfs}, discovered)
	if len(merged) != 2 {
		t.Fatalf("merged disks = %+v, want rootfs + mp0", merged)
	}

	var rootfs, archive *models.Disk
	for i := range merged {
		switch merged[i].Mountpoint {
		case "/":
			rootfs = &merged[i]
		case "/srv/archive":
			archive = &merged[i]
		}
	}
	if rootfs == nil || rootfs.Used != seededRootfs.Used || rootfs.Usage != seededRootfs.Usage {
		t.Fatalf("live rootfs row must win the merge, got %+v", rootfs)
	}
	if archive == nil {
		t.Fatal("config-only mount dropped by the merge")
	}
	if archive.Usage != -1 {
		t.Fatalf("config-only mount usage = %f, want the -1 unknown sentinel", archive.Usage)
	}
	if archive.Total != int64(20)*1024*1024*1024 {
		t.Fatalf("config-only mount total = %d, want 20 GiB from size=", archive.Total)
	}
	if archive.Device != "tank:subvol-106-disk-1" || archive.Type != "mp0" {
		t.Fatalf("config-only mount must keep device and mp key, got %+v", archive)
	}
}

// The efficient cluster/resources path must apply the host agent's node-local
// pct df data the same way the per-node fallback path does. Before the #1477
// fix it skipped that enrichment entirely, so cluster-served installs never
// surfaced per-mount usage regardless of a healthy linked agent.
func TestBuildContainerFromClusterResource_AppliesAgentLXCFilesystems(t *testing.T) {
	monitor := newTestMonitor(t)
	monitor.state.UpdateNodesForInstance("cluster-a", []models.Node{{
		ID:       "cluster-a-pve-a",
		Name:     "pve-a",
		Instance: "cluster-a",
		Status:   "online",
	}})

	now := time.Now()
	monitor.applyAgentLXCFilesystems("cluster-a-pve-a", "agent-1", &agentshost.ProxmoxLXCInventory{
		Containers: []agentshost.ProxmoxLXCContainer{{
			VMID: 202,
			Name: "cache-ct",
			Disks: []agentshost.Disk{
				{Device: "local:202/vm-202-disk-0.raw", Mountpoint: "/", Type: "rootfs", TotalBytes: 32 << 30, UsedBytes: 8 << 30, FreeBytes: 24 << 30, Usage: 25},
				{Device: "/mnt/tank/cache", Mountpoint: "/data", Type: "mp0", TotalBytes: 100 << 30, UsedBytes: 50 << 30, FreeBytes: 50 << 30, Usage: 50},
			},
		}},
		CollectedAt: now.UTC(),
	}, now, 30)

	client := &stubPVEClientLXCStatus{
		containerStatus: &proxmox.Container{Status: "running"},
	}
	resource := proxmox.ClusterResource{
		Type:    "lxc",
		Node:    "pve-a",
		Name:    "cache-ct",
		Status:  "running",
		VMID:    202,
		MaxCPU:  2,
		MaxMem:  4096,
		Mem:     2048,
		MaxDisk: 32 << 30,
		Disk:    8 << 30,
	}

	container, _, _, _, ok := monitor.buildContainerFromClusterResource(
		context.Background(),
		"cluster-a",
		resource,
		client,
		map[int]bool{},
		nil,
	)
	if !ok {
		t.Fatal("expected container to be built")
	}
	if len(container.Disks) != 2 {
		t.Fatalf("expected 2 disks from agent pct df enrichment, got %d: %+v", len(container.Disks), container.Disks)
	}
	var dataMount *models.Disk
	for i := range container.Disks {
		if container.Disks[i].Mountpoint == "/data" {
			dataMount = &container.Disks[i]
		}
	}
	if dataMount == nil || dataMount.Total != 100<<30 || dataMount.Used != 50<<30 {
		t.Fatalf("expected /data mount with real usage from agent data, got %+v", container.Disks)
	}
}

// Issue #2148: a Proxmox LXC with a correlated, online Pulse agent must report
// the agent's own memory sample instead of the cache-inclusive
// cluster/resources fallback, which can badly under-report shared-memory
// workloads (mirrors the #1962 VM behaviour).
func TestIssue2148LXCPrefersLinkedAgentMemoryOverClusterResources(t *testing.T) {
	t.Parallel()

	const gib = 1024 * 1024 * 1024

	client := &stubPVEClientLXCRRD{}
	monitor := &Monitor{rateTracker: NewRateTracker()}
	resource := proxmox.ClusterResource{
		Type:   "lxc",
		Node:   "pve-a",
		Name:   "npu-ct",
		Status: "running",
		VMID:   100,
		MaxMem: 24 * gib,
		Mem:    3459743744,
	}
	guestID := makeGuestID("cluster-a", "pve-a", 100)
	agentHost := models.Host{
		LinkedVMID: guestID,
		Status:     "online",
		Memory: models.Memory{
			Total: 24 * gib,
			Used:  11025571200,
			Free:  24*gib - 11025571200,
		},
	}

	container, _, memorySource, _, ok := monitor.buildContainerFromClusterResource(
		context.Background(),
		"cluster-a",
		resource,
		client,
		map[int]bool{},
		map[string]models.Host{guestID: agentHost},
	)
	if !ok {
		t.Fatal("expected container sample to be built")
	}
	if memorySource != "agent" {
		t.Fatalf("memory source = %q, want agent", memorySource)
	}
	if container.Memory.Used != agentHost.Memory.Used {
		t.Fatalf("memory used = %d, want linked agent value %d", container.Memory.Used, agentHost.Memory.Used)
	}
	if container.Memory.UsageUnavailable || !container.Memory.HasKnownUsage() {
		t.Fatalf("expected usable agent-backed memory, got %+v", container.Memory)
	}
}

// An agent inside a container without lxcfs sees the host's /proc/meminfo, so a
// sample whose total does not match the container's configured limit must not
// replace the provider reading.
func TestIssue2148LXCRejectsAgentMemoryWithMismatchedTotal(t *testing.T) {
	t.Parallel()

	const gib = 1024 * 1024 * 1024

	client := &stubPVEClientLXCRRD{}
	monitor := &Monitor{rateTracker: NewRateTracker()}
	resource := proxmox.ClusterResource{
		Type:   "lxc",
		Node:   "pve-a",
		Name:   "host-memory-leak-ct",
		Status: "running",
		VMID:   101,
		MaxMem: 24 * gib,
		Mem:    3459743744,
	}
	guestID := makeGuestID("cluster-a", "pve-a", 101)
	agentHost := models.Host{
		LinkedVMID: guestID,
		Status:     "online",
		Memory: models.Memory{
			Total: 128 * gib,
			Used:  40 * gib,
			Free:  88 * gib,
		},
	}

	container, _, memorySource, _, ok := monitor.buildContainerFromClusterResource(
		context.Background(),
		"cluster-a",
		resource,
		client,
		map[int]bool{},
		map[string]models.Host{guestID: agentHost},
	)
	if !ok {
		t.Fatal("expected container sample to be built")
	}
	if CanonicalMemorySource(memorySource) != "cluster-resources" {
		t.Fatalf("memory source = %q, want cluster-resources fallback", memorySource)
	}
	if container.Memory.Used != int64(resource.Mem) {
		t.Fatalf("memory used = %d, want provider value %d", container.Memory.Used, resource.Mem)
	}
}

func TestGuestFilesystemStatusDoesNotDiagnoseFromErrorText(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{nil, ""},
		{errors.New("API error 500: internal server error"), "agent-error"},
		{errors.New("API error 500: unsupported command: guest-get-fsinfo"), "agent-error"},
		{errors.New("API error 500: QEMU guest agent is not running"), "agent-error"},
		{errors.New("request for VM 500 failed"), "agent-error"},
		{errors.New("API error 400: upstream API error 403: permission denied"), "agent-error"},
		{errors.New("QEMU guest agent is not running"), "agent-not-running"},
		{context.DeadlineExceeded, "agent-timeout"},
		{fmt.Errorf("wrapped: %w", context.DeadlineExceeded), "agent-timeout"},
		{errors.New("guest agent request timeout"), "agent-timeout"},
		{errors.New("guest agent request: context deadline exceeded"), "agent-timeout"},
	}
	for _, tc := range cases {
		if got := classifyGuestAgentDiskStatusError(tc.err); got != tc.want {
			t.Errorf("%v: reason = %q, want %q", tc.err, got, tc.want)
		}
	}
}

// These synthetic responses exercise the production command guard, filesystem
// collector, unavailable sentinel and emitted operator guidance together. They
// perform no native QGA, backup, guest activation or recovery action.
func TestGuestFilesystemFailureGuidanceUsesObservedEvidence(t *testing.T) {
	cases := []struct {
		name, body, reason, message string
		status                      int
		logged                      bool
	}{
		{"stopped", `{"message":"QEMU guest agent is not running"}`, "agent-not-running", "Proxmox reports the guest agent is not running", 500, true},
		{"unsupported", "unsupported command: guest-get-fsinfo", "agent-error", "Guest filesystem query failed", 500, true},
		{"forbidden", "provider-private-detail", "permission-denied", "Guest filesystem query was not authorised", 403, true},
		{"unauthorised-quotes-stopped", "API error 500: QEMU guest agent is not running", "permission-denied", "Guest filesystem query was not authorised", 401, true},
		{"bad-request-quotes-permission", "API error 403: permission denied", "agent-error", "Guest filesystem query failed", 400, true},
		{"empty", `{"data":{"result":[]}}`, "no-filesystems", "Guest agent returned no filesystem readings", 200, true},
		{"malformed", `{"data":`, "agent-completion-unverified", "", 200, false},
		{"server-uncertain", "provider-private-detail", "agent-completion-unverified", "", 500, false},
		{"gateway-quotes-stopped", "QEMU guest agent is not running", "agent-completion-unverified", "", 502, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/status/current") {
					fmt.Fprint(w, `{"data":{"status":"running","cpu":0.25,"diskread":0,"diskwrite":null,"netin":42}}`)
					return
				}
				if strings.HasSuffix(r.URL.Path, "/config") {
					fmt.Fprint(w, `{"data":{}}`)
					return
				}
				calls.Add(1)
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			client, err := proxmox.NewClient(proxmox.ClientConfig{Host: server.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture", Timeout: time.Second})
			if err != nil {
				t.Fatal(err)
			}
			var output bytes.Buffer
			logger := zerolog.New(&output).With().Str("receipt_scope", "fixture").Logger()
			ctx := logger.WithContext(context.Background())
			m := &Monitor{guestAgentFSInfoTimeout: time.Second, guestAgentRetries: 2}
			res := proxmox.ClusterResource{Node: "node", VMID: 105, Name: "fixture-vm", Type: "qemu", Status: "running", MaxDisk: 1000}
			total, used, free, usage, disks, fromAgent, reason := m.updateVMDisksFromGuestAgentFSInfo(ctx, "fixture-instance", res, client, 1000, 0, 0)
			if reason != tc.reason || fromAgent || usage != -1 || total != 1000 || used != 0 || free != 1000 || disks != nil {
				t.Errorf("unavailable reading = %d/%d/%d/%v/%v/%t/%q", total, used, free, usage, disks, fromAgent, reason)
			}
			beforeStatus := time.Now()
			status, err := client.GetVMStatus(context.Background(), res.Node, res.VMID)
			afterStatus := time.Now()
			if err != nil {
				t.Fatal(err)
			}
			presence := status.IOCounters.Effective()
			if status.CPU != 0.25 || status.DiskRead != 0 || status.NetIn != 42 || !presence.DiskRead || !presence.NetworkIn || presence.DiskWrite || presence.NetworkOut || status.ObservedAt.Before(beforeStatus) || status.ObservedAt.After(afterStatus) {
				t.Errorf("filesystem failure changed independent live counters/presence/receipt: %+v", status)
			}
			if calls.Load() != 1 {
				t.Errorf("wire guest commands = %d, want one even with retries configured", calls.Load())
			}
			if guestAgentDiskDeferred(tc.reason) {
				diagnostic, err := proxmox.NewClient(proxmox.ClientConfig{Host: server.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture", Timeout: time.Second})
				if err != nil {
					t.Fatal(err)
				}
				_, err = diagnostic.GetVMAgentInfo(context.Background(), res.Node, res.VMID)
				if proxmox.GuestAgentDeferredReason(err) != "agent-cooldown" || calls.Load() != 1 {
					t.Errorf("filesystem uncertainty let a fresh diagnostic queue another command: %v calls=%d", err, calls.Load())
				}
			}
			guidance := 0
			for _, line := range bytes.Split(bytes.TrimSpace(output.Bytes()), []byte("\n")) {
				if len(line) == 0 {
					continue
				}
				var event map[string]interface{}
				if json.Unmarshal(line, &event) != nil {
					t.Fatalf("invalid structured log: %s", line)
				}
				if event["vmid"] != float64(res.VMID) {
					continue // Request-layer status logs are separate, not guidance.
				}
				guidance++
				message, _ := event["message"].(string)
				if !tc.logged || event["receipt_scope"] != "fixture" || event["level"] != "info" || event["instance"] != "fixture-instance" || event["vm"] != "fixture-vm" || event["reason"] != tc.reason || !strings.HasPrefix(message, tc.message) {
					t.Errorf("observed-evidence guidance lost: %v", event)
				}
				if !strings.Contains(message, "guest-agent and backup settings") {
					t.Errorf("backup-settings precaution missing: %s", message)
				}
				if tc.reason == "permission-denied" && (!strings.Contains(message, "this VM") || !strings.Contains(message, "Do not broaden shared roles")) {
					t.Errorf("credential advice is not scoped: %s", message)
				}
				for _, unsafe := range []string{"Install and start", "restart", "systemctl", "ps aux", "Re-run", "PulseMonitor", "VM.Monitor", "Sys.Audit", "provider-private-detail"} {
					if strings.Contains(message, unsafe) {
						t.Errorf("unsafe/unobserved advice %q: %s", unsafe, message)
					}
				}
			}
			wantGuidance := 0
			if tc.logged {
				wantGuidance = 1
			}
			if guidance != wantGuidance {
				t.Errorf("guidance records = %d, want %d", guidance, wantGuidance)
			}
		})
	}
}
