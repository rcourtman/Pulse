package monitoring

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	unifiedresources "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

type fakeDockerChecker struct{}

func (f *fakeDockerChecker) CheckDockerInContainer(ctx context.Context, node string, vmid int) (bool, error) {
	return false, nil
}

func TestMonitorGetConfig(t *testing.T) {
	cfg := &config.Config{DataPath: "/tmp/pulse-test"}
	monitor := &Monitor{config: cfg}

	if got := monitor.GetConfig(); got != cfg {
		t.Fatalf("GetConfig = %v, want %v", got, cfg)
	}
}

func TestMonitorSetGetDockerChecker(t *testing.T) {
	monitor := &Monitor{}
	checker := &fakeDockerChecker{}

	monitor.SetDockerChecker(checker)
	if got := monitor.GetDockerChecker(); got != checker {
		t.Fatalf("GetDockerChecker = %v, want %v", got, checker)
	}

	monitor.SetDockerChecker(nil)
	if got := monitor.GetDockerChecker(); got != nil {
		t.Fatalf("GetDockerChecker = %v, want nil", got)
	}
}

func TestMonitorGetDockerHosts(t *testing.T) {
	monitor := &Monitor{state: models.NewState()}
	monitor.state.UpsertDockerHost(models.DockerHost{ID: "host-1", Hostname: "host-1"})

	hosts := monitor.GetDockerHosts()
	if len(hosts) != 1 {
		t.Fatalf("GetDockerHosts length = %d, want 1", len(hosts))
	}
	if hosts[0].ID != "host-1" {
		t.Fatalf("GetDockerHosts[0].ID = %q, want %q", hosts[0].ID, "host-1")
	}
}

func TestMonitorGetDockerHostsNilReceiver(t *testing.T) {
	var monitor *Monitor
	if got := monitor.GetDockerHosts(); got != nil {
		t.Fatalf("GetDockerHosts = %v, want nil", got)
	}
}

func TestMonitorLinkHostAgent(t *testing.T) {
	monitor := &Monitor{state: models.NewState()}

	if err := monitor.LinkHostAgent("", "node-1"); err == nil {
		t.Fatalf("expected error on empty host ID")
	}
	if err := monitor.LinkHostAgent("host-1", ""); err == nil {
		t.Fatalf("expected error on empty node ID")
	}

	monitor.state.UpsertHost(models.Host{ID: "host-1", Hostname: "host-1"})
	monitor.state.UpdateNodes([]models.Node{{ID: "node-1", Name: "node-1"}})

	if err := monitor.LinkHostAgent("host-1", "node-1"); err != nil {
		t.Fatalf("LinkHostAgent error: %v", err)
	}

	hosts := monitor.state.GetHosts()
	if len(hosts) != 1 || hosts[0].LinkedNodeID != "node-1" {
		t.Fatalf("LinkedNodeID = %q, want %q", hosts[0].LinkedNodeID, "node-1")
	}
	if len(monitor.state.Nodes) != 1 || monitor.state.Nodes[0].LinkedAgentID != "host-1" {
		t.Fatalf("LinkedAgentID = %q, want %q", monitor.state.Nodes[0].LinkedAgentID, "host-1")
	}
}

func TestMonitorInvalidateAgentProfileCache(t *testing.T) {
	monitor := &Monitor{
		agentProfileCache: &agentProfileCacheEntry{
			profiles: []models.AgentProfile{{ID: "profile-1"}},
			loadedAt: time.Now(),
		},
	}

	monitor.InvalidateAgentProfileCache()
	if monitor.agentProfileCache != nil {
		t.Fatalf("expected cache to be cleared")
	}
}

func TestMonitorMarkDockerHostPendingUninstall(t *testing.T) {
	monitor := &Monitor{state: models.NewState()}

	if _, err := monitor.MarkDockerHostPendingUninstall(""); err == nil {
		t.Fatalf("expected error on empty host ID")
	}
	if _, err := monitor.MarkDockerHostPendingUninstall("missing"); err == nil {
		t.Fatalf("expected error on missing host")
	}

	monitor.state.UpsertDockerHost(models.DockerHost{ID: "host-1", Hostname: "host-1"})
	host, err := monitor.MarkDockerHostPendingUninstall("host-1")
	if err != nil {
		t.Fatalf("MarkDockerHostPendingUninstall error: %v", err)
	}
	if !host.PendingUninstall {
		t.Fatalf("expected PendingUninstall to be true")
	}

	hosts := monitor.state.GetDockerHosts()
	if len(hosts) != 1 || !hosts[0].PendingUninstall {
		t.Fatalf("state PendingUninstall = %v, want true", hosts[0].PendingUninstall)
	}
}

func wireUnifiedDockerHostForMonitor(m *Monitor, host models.DockerHost) string {
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(models.StateSnapshot{
		DockerHosts: []models.DockerHost{host},
	})
	adapter := unifiedresources.NewMonitorAdapter(registry)
	m.resourceStore = adapter
	readState := unifiedresources.ReadState(adapter)
	return readState.DockerHosts()[0].ID()
}

func TestMonitorDockerRuntimeActionsAcceptUnifiedID(t *testing.T) {
	monitor := &Monitor{
		state:               models.NewState(),
		removedDockerHosts:  make(map[string]time.Time),
		dockerCommands:      make(map[string]*dockerHostCommand),
		dockerCommandIndex:  make(map[string]string),
		dockerMetadataStore: config.NewDockerMetadataStore(t.TempDir(), nil),
	}

	host := models.DockerHost{ID: "host-1", Hostname: "host-1", DisplayName: "Host 1", Status: "online"}
	monitor.state.UpsertDockerHost(host)
	unifiedID := wireUnifiedDockerHostForMonitor(monitor, host)

	got, found := monitor.GetDockerHost(unifiedID)
	if !found || got.ID != host.ID {
		t.Fatalf("GetDockerHost(%q) = (%+v, %v), want raw host id %q", unifiedID, got, found, host.ID)
	}

	updated, err := monitor.SetDockerHostCustomDisplayName(unifiedID, "Unified Name")
	if err != nil {
		t.Fatalf("SetDockerHostCustomDisplayName with unified id: %v", err)
	}
	if updated.CustomDisplayName != "Unified Name" {
		t.Fatalf("expected custom display name to update, got %q", updated.CustomDisplayName)
	}
	meta := monitor.dockerMetadataStore.GetHostMetadata(host.ID)
	if meta == nil || meta.CustomDisplayName != "Unified Name" {
		t.Fatalf("expected metadata keyed by raw host id, got %#v", meta)
	}

	hidden, err := monitor.HideDockerHost(unifiedID)
	if err != nil {
		t.Fatalf("HideDockerHost with unified id: %v", err)
	}
	if !hidden.Hidden {
		t.Fatal("expected hidden flag to be set")
	}

	visible, err := monitor.UnhideDockerHost(unifiedID)
	if err != nil {
		t.Fatalf("UnhideDockerHost with unified id: %v", err)
	}
	if visible.Hidden {
		t.Fatal("expected hidden flag to be cleared")
	}

	pending, err := monitor.MarkDockerHostPendingUninstall(unifiedID)
	if err != nil {
		t.Fatalf("MarkDockerHostPendingUninstall with unified id: %v", err)
	}
	if !pending.PendingUninstall {
		t.Fatal("expected pending uninstall flag to be set")
	}

	removed, err := monitor.RemoveDockerHost(unifiedID)
	if err != nil {
		t.Fatalf("RemoveDockerHost with unified id: %v", err)
	}
	if removed.ID != host.ID {
		t.Fatalf("expected removed host id %q, got %q", host.ID, removed.ID)
	}
	if hosts := monitor.state.GetDockerHosts(); len(hosts) != 0 {
		t.Fatalf("expected host to be removed from state, got %d hosts", len(hosts))
	}
	if _, exists := monitor.removedDockerHosts[host.ID]; !exists {
		t.Fatalf("expected raw host id %q to be blocklisted after removal", host.ID)
	}
}

func TestAllowDockerHostReenrollAcceptsUnifiedID(t *testing.T) {
	monitor := &Monitor{
		state:               models.NewState(),
		removedDockerHosts:  make(map[string]time.Time),
		dockerCommands:      make(map[string]*dockerHostCommand),
		dockerCommandIndex:  make(map[string]string),
		dockerMetadataStore: config.NewDockerMetadataStore(t.TempDir(), nil),
	}

	host := models.DockerHost{ID: "host-reenroll", Hostname: "host-reenroll", DisplayName: "Host Reenroll", Status: "online"}
	monitor.state.UpsertDockerHost(host)
	unifiedID := wireUnifiedDockerHostForMonitor(monitor, host)
	monitor.removedDockerHosts[host.ID] = time.Now()

	if err := monitor.AllowDockerHostReenroll(unifiedID); err != nil {
		t.Fatalf("AllowDockerHostReenroll with unified id: %v", err)
	}
	if _, exists := monitor.removedDockerHosts[host.ID]; exists {
		t.Fatalf("expected raw host id %q to be removed from blocklist", host.ID)
	}
}

func TestEnsureClusterEndpointURL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"https://node.example:8006", "https://node.example:8006"},
		{"node.example", "https://node.example:8006"},
		{"node.example:9006", "https://node.example:9006"},
		{"  node.example  ", "https://node.example:8006"},
	}

	for _, tt := range tests {
		if got := ensureClusterEndpointURL(tt.input); got != tt.expected {
			t.Fatalf("ensureClusterEndpointURL(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestClusterEndpointEffectiveURL(t *testing.T) {
	endpoint := config.ClusterEndpoint{
		Host: "node.local",
		IP:   "10.0.0.1",
	}

	if got := clusterEndpointEffectiveURL(endpoint, true, false); got != "https://node.local:8006" {
		t.Fatalf("verifySSL host preference = %q, want %q", got, "https://node.local:8006")
	}

	endpoint.Host = ""
	if got := clusterEndpointEffectiveURL(endpoint, true, false); got != "https://10.0.0.1:8006" {
		t.Fatalf("verifySSL fallback to IP = %q, want %q", got, "https://10.0.0.1:8006")
	}

	endpoint.Host = "node.local"
	if got := clusterEndpointEffectiveURL(endpoint, false, false); got != "https://10.0.0.1:8006" {
		t.Fatalf("non-SSL IP preference = %q, want %q", got, "https://10.0.0.1:8006")
	}

	endpoint.IPOverride = "192.168.1.10"
	if got := clusterEndpointEffectiveURL(endpoint, false, false); got != "https://192.168.1.10:8006" {
		t.Fatalf("override IP preference = %q, want %q", got, "https://192.168.1.10:8006")
	}

	// #1665: an explicit override must win under verifySSL too, even without a
	// per-endpoint fingerprint. The hostname-for-TLS preference only applies to
	// auto-discovered addresses.
	if got := clusterEndpointEffectiveURL(endpoint, true, false); got != "https://192.168.1.10:8006" {
		t.Fatalf("override under verifySSL = %q, want %q", got, "https://192.168.1.10:8006")
	}

	// An override may be a hostname with a port, not just an IP.
	endpoint.IPOverride = "pve2.internal:9006"
	if got := clusterEndpointEffectiveURL(endpoint, true, false); got != "https://pve2.internal:9006" {
		t.Fatalf("hostname override = %q, want %q", got, "https://pve2.internal:9006")
	}

	// #1199: a cluster-level fingerprint (passed as hasFingerprint=true) must NOT
	// force IP routing for a member endpoint that has no per-endpoint fingerprint;
	// hostname routing must be preserved so TLS validation still works.
	endpoint = config.ClusterEndpoint{Host: "node.local", IP: "10.0.0.1"}
	if got := clusterEndpointEffectiveURL(endpoint, true, true); got != "https://node.local:8006" {
		t.Fatalf("cluster-level fingerprint must not force IP routing for a fingerprint-less endpoint, got %q", got)
	}

	// A per-endpoint fingerprint still allows IP routing under verifySSL.
	endpoint.Fingerprint = "endpoint-fp"
	if got := clusterEndpointEffectiveURL(endpoint, true, false); got != "https://10.0.0.1:8006" {
		t.Fatalf("per-endpoint fingerprint should allow IP routing, got %q", got)
	}

	endpoint = config.ClusterEndpoint{}
	if got := clusterEndpointEffectiveURL(endpoint, true, false); got != "" {
		t.Fatalf("empty endpoint = %q, want empty", got)
	}
}

func TestBuildClusterEndpointsForInit_RespectsDiscoveryPolicy(t *testing.T) {
	oldLookup := lookupIPFunc
	lookupIPFunc = func(host string) ([]net.IP, error) {
		switch host {
		case "allowed.local":
			return []net.IP{net.ParseIP("10.0.0.10")}, nil
		case "blocked.local":
			return []net.IP{net.ParseIP("192.168.1.10")}, nil
		default:
			return nil, nil
		}
	}
	t.Cleanup(func() {
		lookupIPFunc = oldLookup
	})

	monitor := &Monitor{
		config: &config.Config{
			Discovery: config.DiscoveryConfig{
				SubnetAllowlist: []string{"10.0.0.0/8"},
			},
		},
	}

	endpoints, _ := monitor.buildClusterEndpointsForInit(config.PVEInstance{
		Name:      "cluster-a",
		Host:      "https://main.local:8006",
		VerifySSL: true,
		ClusterEndpoints: []config.ClusterEndpoint{
			{NodeName: "node-a", Host: "allowed.local"},
			{NodeName: "node-b", Host: "blocked.local"},
		},
	})

	if len(endpoints) != 2 {
		t.Fatalf("expected configured authority plus allowed member failover, got %#v", endpoints)
	}
	if endpoints[0] != "https://main.local:8006" {
		t.Fatalf("expected configured authority first, got %#v", endpoints)
	}
	if endpoints[1] != "https://allowed.local:8006" {
		t.Fatalf("expected only the discovery-allowed member as failover, got %#v", endpoints)
	}
}

// TestGuestObservedInCycleFailsOpenWithoutEvidence pins the direction of the
// guard. Dropping a legitimate sample is worse than recording a fabricated one,
// so a guest with no LastSeen evidence is recorded rather than skipped.
func TestGuestObservedInCycleFailsOpenWithoutEvidence(t *testing.T) {
	cycleStart := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)

	for _, tc := range []struct {
		name     string
		lastSeen time.Time
		want     bool
	}{
		{name: "observed this cycle", lastSeen: cycleStart.Add(2 * time.Second), want: true},
		{name: "observed exactly at cycle start", lastSeen: cycleStart, want: true},
		{name: "carried forward from an earlier cycle", lastSeen: cycleStart.Add(-30 * time.Second), want: false},
		{name: "no evidence either way", lastSeen: time.Time{}, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := guestObservedInCycle(tc.lastSeen, cycleStart); got != tc.want {
				t.Fatalf("guestObservedInCycle(%v) = %v, want %v", tc.lastSeen, got, tc.want)
			}
		})
	}
}

// TestRecordGuestMetricsSkipsGracePeriodGuestsButKeepsObservedOnes is the
// regression guard for the fabricated-zero defect. ac0fb263c made preserved
// guests carry their real runtime status ("running") instead of the stringified
// aggregate status ("online"), so they started passing recordGuestMetrics'
// status filter. The carried-forward projection has no counters, so every cycle
// a node spent in its grace period wrote CPU, disk and network zeroes into the
// persistent store, showing a collapse to zero rather than a gap.
//
// Both directions matter: the observed guest must still be recorded, because
// silently losing real samples would be a worse regression than the one being
// fixed.
func TestRecordGuestMetricsSkipsGracePeriodGuestsButKeepsObservedOnes(t *testing.T) {
	history := NewMetricsHistory(32, time.Hour)
	monitor := &Monitor{metricsHistory: history}

	cycleStart := time.Now().UTC()
	observedAt := cycleStart.Add(time.Second)
	carriedForward := cycleStart.Add(-2 * time.Minute)

	monitor.recordGuestMetrics(
		[]models.VM{
			{ID: "vm-observed", Status: "running", CPU: 0.42, CPUs: 4, LastSeen: observedAt},
			{ID: "vm-preserved", Status: "running", CPU: 0, CPUs: 4, LastSeen: carriedForward},
		},
		[]models.Container{
			{ID: "ct-observed", Status: "running", Type: "lxc", CPU: 0.25, CPUs: 1, LastSeen: observedAt},
			{ID: "ct-preserved", Status: "running", Type: "lxc", CPU: 0, CPUs: 1, LastSeen: carriedForward},
		},
		cycleStart,
	)

	for _, id := range []string{"vm-observed", "ct-observed"} {
		if got := history.GetGuestMetrics(id, "cpu", time.Hour); len(got) == 0 {
			t.Fatalf("%s: observed guest lost its sample entirely", id)
		}
	}

	for _, id := range []string{"vm-preserved", "ct-preserved"} {
		if got := history.GetGuestMetrics(id, "cpu", time.Hour); len(got) != 0 {
			t.Fatalf("%s: grace-period guest recorded %d fabricated sample(s): %+v", id, len(got), got)
		}
	}
}

// Issue1645: PVE's per-node endpoint GET /nodes/{node}/storage returns every
// storage in the datacenter config, including ones the node is not allowed to
// use. Those come back enabled:0/active:0 and used to be ingested as "disabled"
// rows, which the UI rendered as an offline storage on every node.
func TestIssue1645StorageRestrictedToOtherNodesIsNotListed(t *testing.T) {
	monitor := &Monitor{
		state: models.NewState(),
		config: &config.Config{
			PVEInstances: []config.PVEInstance{
				{
					Name:        "inst1",
					IsCluster:   true,
					ClusterName: "cluster-a",
				},
			},
		},
	}

	client := &fakeStorageClient{
		allStorage: []proxmox.Storage{
			// Restricted to node1 only.
			{Storage: "node1-only", Type: "dir", Content: "images", Nodes: "node1", Enabled: 1, Active: 1, Total: 100, Used: 10, Available: 90},
			// No node restriction at all.
			{Storage: "everywhere", Type: "dir", Content: "images", Enabled: 1, Active: 1, Total: 200, Used: 20, Available: 180},
			// No node restriction, but disabled across the whole datacenter.
			{Storage: "off-everywhere", Type: "dir", Content: "images", Enabled: 0, Active: 0, Total: 300, Used: 30, Available: 270},
		},
		storageByNode: map[string][]proxmox.Storage{
			"node1": {
				{Storage: "node1-only", Type: "dir", Content: "images", Enabled: 1, Active: 1, Total: 100, Used: 10, Available: 90},
				{Storage: "everywhere", Type: "dir", Content: "images", Enabled: 1, Active: 1, Total: 200, Used: 20, Available: 180},
				{Storage: "off-everywhere", Type: "dir", Content: "images", Enabled: 0, Active: 0, Total: 300, Used: 30, Available: 270},
			},
			"node2": {
				// PVE still lists the restricted storage here, flagged unusable.
				{Storage: "node1-only", Type: "dir", Content: "images", Enabled: 0, Active: 0, Total: 100, Used: 10, Available: 90},
				{Storage: "everywhere", Type: "dir", Content: "images", Enabled: 1, Active: 1, Total: 200, Used: 25, Available: 175},
				{Storage: "off-everywhere", Type: "dir", Content: "images", Enabled: 0, Active: 0, Total: 300, Used: 30, Available: 270},
			},
		},
	}

	nodes := []proxmox.Node{
		{Node: "node1", Status: "online"},
		{Node: "node2", Status: "online"},
	}

	monitor.pollStorageWithNodes(context.Background(), "inst1", client, nodes)

	byName := map[string][]models.Storage{}
	for _, storage := range monitor.state.GetSnapshot().Storage {
		byName[storage.Name] = append(byName[storage.Name], storage)
	}

	// (a) The restricted storage must only exist on the node it is assigned to.
	restricted := byName["node1-only"]
	if len(restricted) != 1 {
		t.Fatalf("expected node-restricted storage on exactly one node, got %+v", restricted)
	}
	if restricted[0].Node != "node1" {
		t.Fatalf("expected node-restricted storage on node1, got %+v", restricted[0])
	}

	// (b) An unrestricted storage still shows up on every node.
	unrestricted := byName["everywhere"]
	if len(unrestricted) != 2 {
		t.Fatalf("expected unrestricted storage on both nodes, got %+v", unrestricted)
	}
	seenNodes := map[string]bool{}
	for _, storage := range unrestricted {
		seenNodes[storage.Node] = true
	}
	if !seenNodes["node1"] || !seenNodes["node2"] {
		t.Fatalf("expected unrestricted storage on node1 and node2, got %+v", unrestricted)
	}

	// (c) A globally disabled storage with no restriction is still reported, as disabled.
	disabled := byName["off-everywhere"]
	if len(disabled) != 2 {
		t.Fatalf("expected globally disabled storage to remain visible on both nodes, got %+v", disabled)
	}
	for _, storage := range disabled {
		if storage.Status != "disabled" || storage.Enabled {
			t.Fatalf("expected globally disabled storage to report disabled, got %+v", storage)
		}
	}
}

// Issue1645: the shared-storage aggregation derives its node list from the
// per-node rows, so honouring the datacenter restriction must also stop
// non-member nodes from being credited with the storage.
func TestIssue1645SharedStorageNodeListExcludesRestrictedNodes(t *testing.T) {
	monitor := &Monitor{
		state: models.NewState(),
		config: &config.Config{
			PVEInstances: []config.PVEInstance{
				{
					Name:        "inst1",
					IsCluster:   true,
					ClusterName: "cluster-a",
				},
			},
		},
	}

	sharedRow := proxmox.Storage{
		Storage:   "shared-nfs",
		Type:      "nfs",
		Content:   "images,backup",
		Shared:    1,
		Enabled:   1,
		Active:    1,
		Total:     1000,
		Used:      400,
		Available: 600,
	}
	unusableRow := sharedRow
	unusableRow.Enabled = 0
	unusableRow.Active = 0

	clusterRow := sharedRow
	clusterRow.Nodes = "node1,node2"

	client := &fakeStorageClient{
		allStorage: []proxmox.Storage{clusterRow},
		storageByNode: map[string][]proxmox.Storage{
			"node1": {sharedRow},
			"node2": {sharedRow},
			// node3 is not in the restriction but PVE lists the storage anyway.
			"node3": {unusableRow},
		},
	}

	nodes := []proxmox.Node{
		{Node: "node1", Status: "online"},
		{Node: "node2", Status: "online"},
		{Node: "node3", Status: "online"},
	}

	monitor.pollStorageWithNodes(context.Background(), "inst1", client, nodes)

	var shared *models.Storage
	for _, storage := range monitor.state.GetSnapshot().Storage {
		if storage.Name == "shared-nfs" {
			storageCopy := storage
			shared = &storageCopy
			break
		}
	}
	if shared == nil {
		t.Fatalf("expected shared storage in state, got %+v", monitor.state.GetSnapshot().Storage)
	}
	if shared.NodeCount != 2 {
		t.Fatalf("expected shared storage node count of 2, got %+v", *shared)
	}
	for _, nodeName := range shared.Nodes {
		if nodeName == "node3" {
			t.Fatalf("node3 is excluded by the storage node restriction but was listed: %+v", *shared)
		}
	}
	for _, nodeID := range shared.NodeIDs {
		if nodeID == "cluster-a-node3" {
			t.Fatalf("node3 is excluded by the storage node restriction but was listed: %+v", *shared)
		}
	}
}

func TestIssue1645ClusterStorageRestrictedToOtherNodes(t *testing.T) {
	cases := []struct {
		name        string
		restriction string
		node        string
		want        bool
	}{
		{name: "no restriction", restriction: "", node: "node1", want: false},
		{name: "member", restriction: "node1,node2", node: "node2", want: false},
		{name: "non member", restriction: "node1,node2", node: "node3", want: true},
		{name: "case insensitive member", restriction: "Node1", node: "node1", want: false},
		{name: "spaced restriction", restriction: " node1 ; node2 ", node: "node2", want: false},
		{name: "empty node name", restriction: "node1", node: "", want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := clusterStorageRestrictedToOtherNodes(tc.restriction, tc.node); got != tc.want {
				t.Fatalf("clusterStorageRestrictedToOtherNodes(%q, %q) = %v, want %v", tc.restriction, tc.node, got, tc.want)
			}
		})
	}
}

// A node-local ZFS pool must never be attached to inherently shared/remote
// storage types. On a single-pool node the sole-pool fallback previously
// attached the pool to every storage in the datacenter config, so one failing
// device raised a duplicate ZFS alert per NFS/PBS storage (#1731).
func TestMatchZFSPoolForStorageSkipsInherentlySharedTypes(t *testing.T) {
	rpool := &models.ZFSPool{Name: "rpool"}
	singlePool := map[string]*models.ZFSPool{"rpool": rpool}

	sharedStorages := []models.Storage{
		{Name: "NFS_Qnap_Proxmox_Backup", Type: "nfs", Path: "/mnt/pve/NFS_Qnap_Proxmox_Backup"},
		{Name: "PBS_01_QNAP", Type: "pbs"},
		{Name: "smb_share", Type: "cifs", Path: "/mnt/pve/smb_share"},
		{Name: "ceph_pool", Type: "rbd", Pool: "rpool"},
		// A name collision with the pool must not override the type gate.
		{Name: "rpool", Type: "nfs", Path: "/mnt/pve/rpool"},
	}
	for _, storage := range sharedStorages {
		if got := matchZFSPoolForStorage(storage, singlePool); got != nil {
			t.Fatalf("expected no pool for shared storage %q (type %s), got %q", storage.Name, storage.Type, got.Name)
		}
	}
}

func TestMatchZFSPoolForStorageKeepsLocalMatches(t *testing.T) {
	rpool := &models.ZFSPool{Name: "rpool"}
	tank := &models.ZFSPool{Name: "tank"}
	singlePool := map[string]*models.ZFSPool{"rpool": rpool}
	multiPool := map[string]*models.ZFSPool{"rpool": rpool, "tank": tank}

	cases := []struct {
		name    string
		storage models.Storage
		pools   map[string]*models.ZFSPool
		want    *models.ZFSPool
	}{
		{
			name:    "zfspool storage matches by pool dataset prefix",
			storage: models.Storage{Name: "vm_storage0_01", Type: "zfspool", Pool: "tank/data"},
			pools:   multiPool,
			want:    tank,
		},
		{
			name:    "zfspool storage matches by pool name",
			storage: models.Storage{Name: "local-zfs", Type: "zfspool", Pool: "rpool"},
			pools:   multiPool,
			want:    rpool,
		},
		{
			name:    "dir storage on single-pool node keeps sole-pool fallback",
			storage: models.Storage{Name: "local", Type: "dir", Path: "/var/lib/vz"},
			pools:   singlePool,
			want:    rpool,
		},
		{
			name:    "dir storage on multi-pool node stays unmatched",
			storage: models.Storage{Name: "backup_dir", Type: "dir", Path: "/mnt/backup"},
			pools:   multiPool,
			want:    nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := matchZFSPoolForStorage(tc.storage, tc.pools); got != tc.want {
				t.Fatalf("matchZFSPoolForStorage(%q) = %v, want %v", tc.storage.Name, got, tc.want)
			}
		})
	}
}

// A per-resource grace override saved through the intent policy UI is keyed by
// the unified registry resource ID. For a pulse-agent merged with its Proxmox
// node the alert evaluator references the host as "agent:{hostID}", so the
// override only applies when that reference resolves back to the same merged
// resource the UI saved against (#1497).
func TestMergedProxmoxHostAgentGraceOverrideHoldsCPUAlert(t *testing.T) {
	host := models.Host{
		ID:              "host-uuid-1",
		Hostname:        "proxmox2",
		MachineID:       "machine-abc",
		LinkedNodeID:    "node/proxmox2",
		Platform:        "linux",
		Status:          "online",
		CPUUsage:        95,
		CPUCount:        8,
		IntervalSeconds: 30,
		LastSeen:        time.Now(),
	}
	node := models.Node{
		ID:            "node/proxmox2",
		Name:          "proxmox2",
		Instance:      "pve",
		Status:        "online",
		LinkedAgentID: "host-uuid-1",
	}

	newAdapter := func() (*unifiedresources.MonitorAdapter, unifiedresources.Resource) {
		adapter := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
		adapter.PopulateFromSnapshot(models.StateSnapshot{
			Nodes: []models.Node{node},
			Hosts: []models.Host{host},
		})
		for _, resource := range adapter.GetAll() {
			if resource.Agent != nil && resource.Proxmox != nil {
				return adapter, resource
			}
		}
		t.Fatal("expected node+agent snapshot to merge into one resource")
		return nil, unifiedresources.Resource{}
	}

	newManager := func(adapter *unifiedresources.MonitorAdapter) *alerts.Manager {
		manager := alerts.NewManagerWithDataDir(t.TempDir())
		t.Cleanup(manager.Stop)
		cfg := manager.GetConfig()
		// Normalization restores absent keys to the 5s default, but an
		// explicit zero survives, and zero legacy delay makes the assertion
		// discriminating: no override means the alert fires on first check.
		cfg.TimeThresholds = map[string]int{"agent": 0}
		cfg.MetricTimeThresholds = nil
		manager.UpdateConfig(cfg)
		monitor := &Monitor{alertManager: manager}
		monitor.installOperatorIntentResolver(adapter)
		return manager
	}

	hostCPUAlert := func(manager *alerts.Manager) *alerts.Alert {
		for _, alert := range manager.GetActiveAlerts() {
			if alert.ResourceID == "agent:host-uuid-1" && alert.Type == "cpu" {
				found := alert
				return &found
			}
		}
		return nil
	}

	// Control: without an override the CPU alert fires on the first check,
	// proving the fixture actually triggers.
	controlAdapter, _ := newAdapter()
	control := newManager(controlAdapter)
	control.CheckHost(host)
	if hostCPUAlert(control) == nil {
		t.Fatal("control manager without override should raise the CPU alert immediately")
	}

	adapter, merged := newAdapter()
	manager := newManager(adapter)
	grace := 600
	document := alerts.NewAlertIntentPolicyDocument()
	document.Resources = map[string]map[string]alerts.AlertIntentRule{
		merged.ID: {"metric.cpu": {GraceSeconds: &grace}},
	}
	if err := manager.LoadIntentPolicies(document); err != nil {
		t.Fatalf("LoadIntentPolicies: %v", err)
	}

	manager.CheckHost(host)
	if alert := hostCPUAlert(manager); alert != nil {
		t.Fatalf("CPU alert fired immediately despite 600s per-resource grace override on %q: %+v", merged.ID, alert)
	}
}

func TestNodeThresholdOverrideStoredUnderRegistryIDApplies(t *testing.T) {
	node := models.Node{
		ID:          "mock-cluster-pve1",
		Name:        "pve1",
		Instance:    "mock-cluster",
		Status:      "online",
		Type:        "node",
		CPU:         0.10,
		Memory:      models.Memory{Total: 16 << 30, Used: 10 << 30, Free: 6 << 30, Usage: 60},
		LoadAverage: []float64{},
	}

	newAdapter := func() (*unifiedresources.MonitorAdapter, string) {
		adapter := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
		adapter.PopulateFromSnapshot(models.StateSnapshot{Nodes: []models.Node{node}})
		for _, resource := range adapter.GetAll() {
			if resource.Proxmox != nil {
				return adapter, resource.ID
			}
		}
		t.Fatal("expected node snapshot to produce a registry resource with a Proxmox facet")
		return nil, ""
	}

	overrideFor := func(registryID string) map[string]alerts.ThresholdConfig {
		return map[string]alerts.ThresholdConfig{
			registryID: {Memory: &alerts.HysteresisThreshold{Trigger: 50, Clear: 45}},
		}
	}

	newManager := func(overrides map[string]alerts.ThresholdConfig) *alerts.Manager {
		manager := alerts.NewManagerWithDataDir(t.TempDir())
		t.Cleanup(manager.Stop)
		cfg := manager.GetConfig()
		cfg.TimeThresholds = map[string]int{"node": 0}
		cfg.MetricTimeThresholds = nil
		cfg.Overrides = overrides
		manager.UpdateConfig(cfg)
		return manager
	}

	memoryAlert := func(manager *alerts.Manager) *alerts.Alert {
		for _, alert := range manager.GetActiveAlerts() {
			if alert.ResourceID == node.ID && alert.Type == "memory" {
				found := alert
				return &found
			}
		}
		return nil
	}

	// Control 1: without the override, the 60% sample stays below the node
	// default, so the fixture cannot fire on defaults alone.
	controlAdapter, _ := newAdapter()
	control := newManager(nil)
	monitorControl := &Monitor{alertManager: control}
	monitorControl.installOperatorIntentResolver(controlAdapter)
	control.CheckNode(node)
	if alert := memoryAlert(control); alert != nil {
		t.Fatalf("control without override fired a memory alert: %+v", alert)
	}

	// Control 2: the override stored under the registry ID with no registry
	// resolver installed never resolves, which is the reported divergence
	// (#1738): the UI shows Custom while the engine evaluates defaults.
	_, registryID := newAdapter()
	unresolved := newManager(overrideFor(registryID))
	unresolved.CheckNode(node)
	if alert := memoryAlert(unresolved); alert != nil {
		t.Fatalf("manager without registry resolver applied a registry-keyed override: %+v", alert)
	}

	adapter, registryID2 := newAdapter()
	manager := newManager(overrideFor(registryID2))
	monitor := &Monitor{alertManager: manager}
	monitor.installOperatorIntentResolver(adapter)
	manager.CheckNode(node)
	alert := memoryAlert(manager)
	if alert == nil {
		t.Fatalf("memory alert did not fire despite 50%% override stored under registry ID %q", registryID2)
	}
	if alert.Threshold != 50 {
		t.Fatalf("alert threshold = %v, want 50", alert.Threshold)
	}
}
