package monitoring

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

func TestParseVMConfigNetworks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input map[string]interface{}
		want  []containerNetworkDetails
	}{
		{name: "nil config", input: nil, want: nil},
		{name: "no devices", input: map[string]interface{}{"cores": 2, "netmask": "ignored"}, want: nil},
		{
			name:  "tagged device with firewall",
			input: map[string]interface{}{"net0": "virtio=bc:24:11:2e:5f:01,bridge=vmbr0,firewall=1,tag=30"},
			want: []containerNetworkDetails{
				{Name: "net0", MAC: "BC:24:11:2E:5F:01", Bridge: "vmbr0", VLAN: 30, Firewall: true},
			},
		},
		{
			name:  "untagged device without firewall",
			input: map[string]interface{}{"net0": "e1000=BC:24:11:00:00:02,bridge=vmbr1,link_down=1,queues=4"},
			want: []containerNetworkDetails{
				{Name: "net0", MAC: "BC:24:11:00:00:02", Bridge: "vmbr1"},
			},
		},
		{
			name: "devices keep their numeric order",
			input: map[string]interface{}{
				"net10": "virtio=BC:24:11:00:00:10,bridge=vmbr0,tag=40",
				"net2":  "virtio=BC:24:11:00:00:02,bridge=vmbr0,tag=20",
				"net0":  "virtio=BC:24:11:00:00:00,bridge=vmbr0",
			},
			want: []containerNetworkDetails{
				{Name: "net0", MAC: "BC:24:11:00:00:00", Bridge: "vmbr0"},
				{Name: "net2", MAC: "BC:24:11:00:00:02", Bridge: "vmbr0", VLAN: 20},
				{Name: "net10", MAC: "BC:24:11:00:00:10", Bridge: "vmbr0", VLAN: 40},
			},
		},
		{
			name:  "explicit macaddr and firewall off",
			input: map[string]interface{}{"net0": "model=virtio,macaddr=bc:24:11:aa:bb:cc,bridge=vnet10,firewall=0"},
			want: []containerNetworkDetails{
				{Name: "net0", MAC: "BC:24:11:AA:BB:CC", Bridge: "vnet10"},
			},
		},
		{
			name:  "a device with no bridge has no placement",
			input: map[string]interface{}{"net0": "virtio=BC:24:11:00:00:01"},
			want:  nil,
		},
		{
			name:  "a tag outside the 802.1Q range is untagged",
			input: map[string]interface{}{"net0": "virtio=BC:24:11:00:00:01,bridge=vmbr0,tag=4095"},
			want: []containerNetworkDetails{
				{Name: "net0", MAC: "BC:24:11:00:00:01", Bridge: "vmbr0"},
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := parseVMConfigNetworks(tc.input); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("parseVMConfigNetworks() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestParseProxmoxVLANTagAndFlagForms(t *testing.T) {
	t.Parallel()

	for input, want := range map[interface{}]int{"30": 30, " 7 ": 7, 20: 20, float64(40): 40, "0": 0, "abc": 0, 5000: 0, 2.5: 0} {
		if got := parseProxmoxVLANTag(input); got != want {
			t.Errorf("parseProxmoxVLANTag(%#v) = %d, want %d", input, got, want)
		}
	}
	if parseProxmoxVLANTag(nil) != 0 {
		t.Error("a missing tag must be untagged")
	}
	for input, want := range map[interface{}]bool{"1": true, "0": false, true: true, float64(1): true, float64(0): false, "yes": true, "": false} {
		if got := parseProxmoxFlag(input); got != want {
			t.Errorf("parseProxmoxFlag(%#v) = %v, want %v", input, got, want)
		}
	}
	if parseProxmoxFlag(nil) {
		t.Error("a missing flag must be off")
	}
}

func TestApplyVMNICConfig(t *testing.T) {
	t.Parallel()

	devices := []containerNetworkDetails{
		{Name: "net0", MAC: "BC:24:11:00:00:01", Bridge: "vmbr0", VLAN: 30, Firewall: true},
		{Name: "net1", MAC: "BC:24:11:00:00:02", Bridge: "vmbr1"},
	}

	t.Run("a reported interface takes the placement of the device with its address", func(t *testing.T) {
		reported := []models.GuestNetworkInterface{
			{Name: "ens18", MAC: "bc:24:11:00:00:01", Addresses: []string{"10.0.30.5"}},
			{Name: "ens18.5", MAC: "bc:24:11:00:00:01", Addresses: []string{"10.0.5.5"}},
			{Name: "docker0", MAC: "02:42:00:00:00:01", Addresses: []string{"172.17.0.1"}},
		}
		got := applyVMNICConfig(reported, devices)
		want := []models.GuestNetworkInterface{
			{Name: "ens18", MAC: "bc:24:11:00:00:01", Addresses: []string{"10.0.30.5"}, Bridge: "vmbr0", VLAN: 30, Firewall: true},
			{Name: "ens18.5", MAC: "bc:24:11:00:00:01", Addresses: []string{"10.0.5.5"}, Bridge: "vmbr0", VLAN: 30, Firewall: true},
			{Name: "docker0", MAC: "02:42:00:00:00:01", Addresses: []string{"172.17.0.1"}},
			{Name: "net1", MAC: "BC:24:11:00:00:02", Addresses: []string{}, Bridge: "vmbr1"},
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("applyVMNICConfig() = %+v, want %+v", got, want)
		}
		if reported[0].Bridge != "" {
			t.Fatal("the caller's interfaces must not be modified")
		}
	})

	t.Run("a guest that reports nothing is still placed", func(t *testing.T) {
		got := applyVMNICConfig(nil, devices)
		if len(got) != 2 || got[0].Name != "net0" || got[0].VLAN != 30 || got[1].Bridge != "vmbr1" {
			t.Fatalf("applyVMNICConfig(nil) = %+v", got)
		}
		for _, iface := range got {
			if !isPlacementOnlyInterface(iface) {
				t.Fatalf("%+v should be recognised as placement only", iface)
			}
		}
		if reported := guestReportedInterfaces(got); reported != nil {
			t.Fatalf("placement-only entries are not guest reports: %+v", reported)
		}
	})

	t.Run("applying again replaces an earlier placement", func(t *testing.T) {
		first := applyVMNICConfig([]models.GuestNetworkInterface{
			{Name: "ens18", MAC: "BC:24:11:00:00:01", Addresses: []string{"10.0.30.5"}},
		}, devices)
		moved := []containerNetworkDetails{{Name: "net0", MAC: "BC:24:11:00:00:01", Bridge: "vmbr0", VLAN: 40}}
		got := applyVMNICConfig(first, moved)
		want := []models.GuestNetworkInterface{
			{Name: "ens18", MAC: "BC:24:11:00:00:01", Addresses: []string{"10.0.30.5"}, Bridge: "vmbr0", VLAN: 40},
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("applyVMNICConfig() = %+v, want %+v", got, want)
		}
	})

	t.Run("no configuration leaves reports unplaced", func(t *testing.T) {
		got := applyVMNICConfig([]models.GuestNetworkInterface{{Name: "eth0", Addresses: []string{"10.0.0.2"}}}, nil)
		if len(got) != 1 || got[0].Bridge != "" {
			t.Fatalf("applyVMNICConfig() = %+v", got)
		}
		if applyVMNICConfig(nil, nil) != nil {
			t.Fatal("nothing reported and nothing configured must stay empty")
		}
	})
}

func TestGuestAgentEvidenceIgnoresPlacementOnlyInterfaces(t *testing.T) {
	t.Parallel()

	seen := time.Now().Add(-time.Minute)
	placed := applyVMNICConfig(nil, []containerNetworkDetails{{Name: "net0", MAC: "BC:24:11:00:00:01", Bridge: "vmbr0"}})
	vm := &models.VM{Type: "qemu", LastSeen: seen, NetworkInterfaces: placed}
	if got := guestAgentEvidenceTime(vm); !got.IsZero() {
		t.Fatalf("a device known only from configuration is not guest agent evidence, got %v", got)
	}
	vm.NetworkInterfaces = append(vm.NetworkInterfaces, models.GuestNetworkInterface{Name: "eth0", Addresses: []string{"10.0.0.2"}})
	if got := guestAgentEvidenceTime(vm); !got.Equal(seen) {
		t.Fatalf("a reported interface is evidence, got %v", got)
	}
}

func TestPreserveVMOptionalEnrichmentKeepsReportsOverPlacementOnlyRow(t *testing.T) {
	t.Parallel()

	previous := models.VM{NetworkInterfaces: []models.GuestNetworkInterface{
		{Name: "ens18", MAC: "BC:24:11:00:00:01", Addresses: []string{"10.0.30.5"}, Bridge: "vmbr0", VLAN: 30},
	}}
	current := models.VM{NetworkInterfaces: applyVMNICConfig(nil, []containerNetworkDetails{
		{Name: "net0", MAC: "BC:24:11:00:00:01", Bridge: "vmbr0", VLAN: 30},
	})}
	got := preserveVMOptionalEnrichment(current, &previous)
	if len(got.NetworkInterfaces) != 1 || got.NetworkInterfaces[0].Name != "ens18" {
		t.Fatalf("the guest's last report should survive a row built without guest detail: %+v", got.NetworkInterfaces)
	}

	reported := models.VM{NetworkInterfaces: []models.GuestNetworkInterface{{Name: "eth1", Addresses: []string{"10.0.0.9"}}}}
	got = preserveVMOptionalEnrichment(reported, &previous)
	if got.NetworkInterfaces[0].Name != "eth1" {
		t.Fatalf("a current report must not be replaced: %+v", got.NetworkInterfaces)
	}
}

type nicConfigClient struct {
	stubPVEClient
	config map[string]interface{}
	err    error
	calls  int
}

func (c *nicConfigClient) GetVMConfig(context.Context, string, int) (map[string]interface{}, error) {
	c.calls++
	return c.config, c.err
}

func TestRunningVMNICConfigReadsOnceThenReuses(t *testing.T) {
	t.Parallel()

	m := &Monitor{}
	// Past the start-up spread, as on any monitor that has been up a minute.
	m.vmNICConfig.startedAt = time.Now().Add(-2 * vmNICConfigStartupSpread)
	client := &nicConfigClient{config: map[string]interface{}{"net0": "virtio=BC:24:11:00:00:01,bridge=vmbr0,tag=30"}}

	for i := 0; i < 5; i++ {
		got := m.runningVMNICConfig(context.Background(), client, "pve", "node1", 100)
		if len(got) != 1 || got[0].VLAN != 30 {
			t.Fatalf("poll %d: runningVMNICConfig() = %+v", i, got)
		}
	}
	if client.calls != 1 {
		t.Fatalf("configuration reads = %d, want 1 across five polls", client.calls)
	}

	// Another guest has its own reading.
	m.runningVMNICConfig(context.Background(), client, "pve", "node1", 101)
	if client.calls != 2 {
		t.Fatalf("configuration reads = %d, want 2 after a second guest", client.calls)
	}
}

func TestRunningVMNICConfigRefreshesAfterTTL(t *testing.T) {
	t.Parallel()

	m := &Monitor{}
	key := guestMetadataCacheKey("pve", "node1", 100)
	stale := time.Now().Add(-(vmNICConfigTTL + vmNICConfigTTLJitter + time.Second))
	m.vmNICConfig.startedAt = stale
	m.vmNICConfig.store(key, []containerNetworkDetails{{Name: "net0", Bridge: "vmbr0", VLAN: 30}}, stale)
	client := &nicConfigClient{config: map[string]interface{}{"net0": "virtio=BC:24:11:00:00:01,bridge=vmbr0,tag=40"}}

	got := m.runningVMNICConfig(context.Background(), client, "pve", "node1", 100)
	if client.calls != 1 || len(got) != 1 || got[0].VLAN != 40 {
		t.Fatalf("an aged reading must be replaced: calls=%d got=%+v", client.calls, got)
	}
}

func TestRunningVMNICConfigKeepsLastReadingWhenReadFails(t *testing.T) {
	t.Parallel()

	m := &Monitor{}
	key := guestMetadataCacheKey("pve", "node1", 100)
	stale := time.Now().Add(-(vmNICConfigTTL + vmNICConfigTTLJitter + time.Second))
	m.vmNICConfig.startedAt = stale
	m.vmNICConfig.store(key, []containerNetworkDetails{{Name: "net0", Bridge: "vmbr0", VLAN: 30}}, stale)
	client := &nicConfigClient{err: errors.New("permission denied")}

	for i := 0; i < 3; i++ {
		got := m.runningVMNICConfig(context.Background(), client, "pve", "node1", 100)
		if len(got) != 1 || got[0].VLAN != 30 {
			t.Fatalf("poll %d: a failed read must keep the last reading, got %+v", i, got)
		}
	}
	if client.calls != 1 {
		t.Fatalf("a failed read must back off, got %d reads in three polls", client.calls)
	}
}

func TestRunningVMNICConfigSendsNothingOnClosedBudgetOrPlainClient(t *testing.T) {
	t.Parallel()

	m := &Monitor{}
	m.vmNICConfig.startedAt = time.Now().Add(-2 * vmNICConfigStartupSpread)
	client := &nicConfigClient{config: map[string]interface{}{"net0": "virtio=BC:24:11:00:00:01,bridge=vmbr0"}}

	closed, cancel := context.WithCancel(context.Background())
	cancel()
	if got := m.runningVMNICConfig(closed, client, "pve", "node1", 100); got != nil || client.calls != 0 {
		t.Fatalf("a closed enrichment budget must send nothing: got=%+v calls=%d", got, client.calls)
	}

	// A client without configuration reads is left alone.
	var plain PVEClientInterface = &stubPVEClient{}
	if got := m.runningVMNICConfig(context.Background(), plain, "pve", "node1", 100); got != nil {
		t.Fatalf("runningVMNICConfig() = %+v, want nil", got)
	}
}

func TestVMNICConfigFirstReadsAreSpreadAfterStart(t *testing.T) {
	t.Parallel()

	var cache vmNICConfigCache
	start := time.Now()
	dueAtStart := 0
	const guests = 200
	for vmid := 100; vmid < 100+guests; vmid++ {
		key := guestMetadataCacheKey("pve", "node1", vmid)
		if _, due := cache.due(key, start); due {
			dueAtStart++
		}
		if _, due := cache.due(key, start.Add(vmNICConfigStartupSpread)); !due {
			t.Fatalf("guest %d is still unread after the start-up spread", vmid)
		}
	}
	// A 10-second poll should see roughly a sixth of the guests, not all of them.
	if dueAtStart > guests/4 {
		t.Fatalf("%d of %d first reads fall on the first poll", dueAtStart, guests)
	}
}

func TestVMNICConfigCacheForgetsGuestsNoLongerPolled(t *testing.T) {
	t.Parallel()

	var cache vmNICConfigCache
	old := time.Now().Add(-2 * vmNICConfigRetention)
	cache.store("gone", []containerNetworkDetails{{Name: "net0", Bridge: "vmbr0"}}, old)
	cache.store("live", []containerNetworkDetails{{Name: "net0", Bridge: "vmbr0"}}, time.Now())
	if _, kept := cache.entries["gone"]; kept {
		t.Fatal("an entry nothing has asked about must be dropped")
	}
	if _, kept := cache.entries["live"]; !kept {
		t.Fatal("a current entry must be kept")
	}
}

func TestFetchStoppedVMConfigAnswersOnBootAndPlacementFromOneRead(t *testing.T) {
	t.Parallel()

	m := &Monitor{}
	client := &nicConfigClient{config: map[string]interface{}{
		"onboot": 1,
		"net0":   "virtio=BC:24:11:00:00:01,bridge=vmbr0,tag=20,firewall=1",
	}}
	onBoot, devices := m.fetchStoppedVMConfig(context.Background(), client, "pve", "node1", 100)
	if onBoot == nil || !*onBoot {
		t.Fatalf("onBoot = %v, want true", onBoot)
	}
	if len(devices) != 1 || devices[0].VLAN != 20 || !devices[0].Firewall {
		t.Fatalf("devices = %+v", devices)
	}
	if client.calls != 1 {
		t.Fatalf("configuration reads = %d, want 1", client.calls)
	}

	// The VM starts: its placement is already known, so nothing more is read.
	m.vmNICConfig.startedAt = time.Now().Add(-2 * vmNICConfigStartupSpread)
	if got := m.runningVMNICConfig(context.Background(), client, "pve", "node1", 100); len(got) != 1 || client.calls != 1 {
		t.Fatalf("a just-started VM must reuse the stopped read: got=%+v calls=%d", got, client.calls)
	}

	// A failed read keeps the placement and reports start-on-boot as unknown.
	client.err = errors.New("timeout")
	onBoot, devices = m.fetchStoppedVMConfig(context.Background(), client, "pve", "node1", 100)
	if onBoot != nil || len(devices) != 1 {
		t.Fatalf("after a failed read: onBoot=%v devices=%+v", onBoot, devices)
	}
}

func TestContainerNetworkPlacementFromStatusAndConfig(t *testing.T) {
	t.Parallel()

	details := parseContainerConfigNetworks(map[string]interface{}{
		"net0": "name=eth0,bridge=vmbr0,firewall=1,hwaddr=BC:24:11:00:00:01,ip=dhcp,tag=30,type=veth",
		"net1": "name=eth1,bridge=vmbr1,hwaddr=BC:24:11:00:00:02,ip=10.99.0.5/24",
	})
	if len(details) != 2 {
		t.Fatalf("details = %+v", details)
	}
	if details[0].Bridge != "vmbr0" || details[0].VLAN != 30 || !details[0].Firewall {
		t.Fatalf("net0 = %+v", details[0])
	}
	if details[1].Bridge != "vmbr1" || details[1].VLAN != 0 || details[1].Firewall {
		t.Fatalf("net1 = %+v", details[1])
	}

	// The live status named eth0 with an address; configuration adds where it
	// is plugged in, and a device the status did not list.
	interfaces := []models.GuestNetworkInterface{{Name: "eth0", MAC: "BC:24:11:00:00:01", Addresses: []string{"10.0.30.7"}}}
	for _, detail := range details {
		mergeContainerNetworkInterface(&interfaces, detail)
	}
	if len(interfaces) != 2 {
		t.Fatalf("interfaces = %+v", interfaces)
	}
	if got := interfaces[0]; got.Bridge != "vmbr0" || got.VLAN != 30 || !got.Firewall || !reflect.DeepEqual(got.Addresses, []string{"10.0.30.7"}) {
		t.Fatalf("eth0 = %+v", got)
	}
	if got := interfaces[1]; got.Name != "eth1" || got.Bridge != "vmbr1" {
		t.Fatalf("eth1 = %+v", got)
	}
}

func TestEnrichContainerMetadataCarriesBridgeAndVLAN(t *testing.T) {
	t.Parallel()

	m := &Monitor{}
	client := &guestConfigCoverageClient{containerConfig: map[string]interface{}{
		"net0": "name=eth0,bridge=vmbr0,firewall=1,hwaddr=BC:24:11:00:00:01,ip=dhcp,tag=30,type=veth",
	}}
	container := &models.Container{VMID: 200, Name: "ct", Status: "stopped"}
	m.enrichContainerMetadata(context.Background(), client, "pve", "node1", container)
	if client.containerCalls != 1 {
		t.Fatalf("configuration reads = %d, want the one the poll already makes", client.containerCalls)
	}
	if len(container.NetworkInterfaces) != 1 {
		t.Fatalf("interfaces = %+v", container.NetworkInterfaces)
	}
	if got := container.NetworkInterfaces[0]; got.Name != "eth0" || got.Bridge != "vmbr0" || got.VLAN != 30 || !got.Firewall {
		t.Fatalf("eth0 = %+v", got)
	}
}

func TestContainerStatusNetworkCarriesPlacement(t *testing.T) {
	t.Parallel()

	m := &Monitor{}
	client := &guestConfigCoverageClient{}
	container := &models.Container{VMID: 201, Name: "ct", Status: "running"}
	status := &proxmox.Container{Network: map[string]proxmox.ContainerNetworkConfig{
		"net0": {Name: "eth0", HWAddr: "BC:24:11:00:00:03", Bridge: "vmbr0", Tag: float64(40), Firewall: float64(1), IP: "10.0.40.9/24"},
	}}
	m.enrichContainerMetadata(context.Background(), client, "pve", "node1", container, status)
	if len(container.NetworkInterfaces) != 1 {
		t.Fatalf("interfaces = %+v", container.NetworkInterfaces)
	}
	if got := container.NetworkInterfaces[0]; got.Bridge != "vmbr0" || got.VLAN != 40 || !got.Firewall {
		t.Fatalf("eth0 = %+v", got)
	}
}
