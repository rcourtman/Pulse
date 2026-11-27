package monitoring

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	unifiedresources "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

type fakeDockerChecker struct{}

func (f *fakeDockerChecker) CheckDockerInContainer(ctx context.Context, container models.Container) (bool, error) {
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
	monitor := &Monitor{state: models.NewState(), hostContinuityStore: config.NewHostContinuityStore(t.TempDir(), nil)}

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

	endpoints, _, _ := monitor.buildClusterEndpointsForInit(config.PVEInstance{
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

func TestApplyStorageFallbackAndRecordNodeMetrics_PersistsTemperatureHistory(t *testing.T) {
	monitor := &Monitor{
		state:          models.NewState(),
		metricsHistory: NewMetricsHistory(16, time.Hour),
		alertManager:   alerts.NewManager(),
	}
	nodes := []models.Node{{
		ID:       "pve1-node1",
		Name:     "node1",
		Instance: "pve1",
		Status:   "online",
		CPU:      0.25,
		Temperature: &models.Temperature{
			Available:  true,
			CPUPackage: 61.5,
			CPUMax:     64,
		},
	}}

	monitor.applyStorageFallbackAndRecordNodeMetrics("pve1", nil, nodes, nil, nil)

	points := monitor.metricsHistory.GetNodeMetrics("pve1-node1", "temperature", time.Hour)
	if len(points) != 1 || points[0].Value != 64 {
		t.Fatalf("temperature history = %+v, want one point at 64C", points)
	}
}

// A node fed by a linked agent carries its package above CPUMax (CPUMax is
// built from cores there). Node history records the hottest reading, the same
// value the node's resource temperature and its agent's history carry.
func TestApplyStorageFallbackAndRecordNodeMetrics_RecordsPrimaryCPUTemperature(t *testing.T) {
	monitor := &Monitor{
		state:          models.NewState(),
		metricsHistory: NewMetricsHistory(16, time.Hour),
		alertManager:   alerts.NewManager(),
	}
	nodes := []models.Node{{
		ID:       "pve1-node2",
		Name:     "node2",
		Instance: "pve1",
		Status:   "online",
		Temperature: &models.Temperature{
			Available:  true,
			HasCPU:     true,
			CPUPackage: 61,
			CPUMax:     57,
			Cores:      []models.CoreTemp{{Core: 0, Temp: 55}, {Core: 1, Temp: 57}},
		},
	}}

	monitor.applyStorageFallbackAndRecordNodeMetrics("pve1", nil, nodes, nil, nil)

	points := monitor.metricsHistory.GetNodeMetrics("pve1-node2", "temperature", time.Hour)
	if len(points) != 1 || points[0].Value != 61 {
		t.Fatalf("temperature history = %+v, want one point at 61C", points)
	}
}

// Exercise the registry's real manual merge, followed by the next guest poll's
// fallback lookup. A healthy platform row must not revive an offline agent.
func TestCorrelatedGuestMemoryNextPoll(t *testing.T) {
	for _, tc := range []struct {
		name, status string
		age          time.Duration
		unavailable  bool
		want         uint64
	}{
		{name: "live", status: "online", want: 2800},
		{name: "offline", status: "offline", want: 8000},
		{name: "stale", status: "online", age: time.Hour, want: 8000},
		{name: "unavailable", status: "online", unavailable: true, want: 8000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Now()
			guestID := makeGuestID("pve-a", "node1", 111)
			store := unifiedresources.NewMemoryStore()
			if err := store.AddLink(unifiedresources.ResourceLink{ResourceA: "vm-test", ResourceB: "agent-test", PrimaryID: "agent-test"}); err != nil {
				t.Fatal(err)
			}
			registry := unifiedresources.NewRegistry(store)
			total, used := int64(8000), int64(8000)
			resources := []unifiedresources.Resource{
				{ID: "vm-test", Type: unifiedresources.ResourceTypeVM, Name: "firewall", Status: unifiedresources.StatusOnline, LastSeen: now,
					Sources: []unifiedresources.DataSource{unifiedresources.SourceProxmox},
					Proxmox: &unifiedresources.ProxmoxData{SourceID: guestID, Instance: "pve-a", NodeName: "node1", VMID: 111, RuntimeStatus: "running"},
					Metrics: &unifiedresources.ResourceMetrics{Memory: &unifiedresources.MetricValue{Total: &total, Used: &used, Percent: 100, Source: unifiedresources.SourceProxmox}}},
				{ID: "agent-test", Type: unifiedresources.ResourceTypeAgent, Name: "firewall-agent", Status: unifiedresources.ResourceStatus(tc.status), LastSeen: now.Add(-tc.age),
					Sources: []unifiedresources.DataSource{unifiedresources.SourceAgent},
					Agent:   &unifiedresources.AgentData{AgentID: "agent-test", Stale: tc.status == "offline", Memory: &unifiedresources.AgentMemoryMeta{Total: 8000, Used: 2800, Cache: 4200, Free: 1000, UsageUnavailable: tc.unavailable}}},
			}
			registry.IngestResources(resources)
			if len(registry.VMs()) != 1 || len(registry.Hosts()) != 0 {
				t.Fatal("initial registry did not merge linked guest")
			}
			// Rebuild from source evidence and the retained link store, not a
			// previously merged resource or the old in-memory matcher.
			registry = unifiedresources.NewRegistry(store)
			registry.IngestResources(resources)
			if len(registry.VMs()) != 1 || len(registry.Hosts()) != 0 {
				t.Fatalf("expected one merged VM and no standalone hosts")
			}

			// A simultaneous guest with the same node name and VMID must not
			// inherit this instance's agent memory.
			registry.IngestRecords(unifiedresources.SourceProxmox, []unifiedresources.IngestRecord{{
				SourceID: makeGuestID("pve-b", "node1", 111),
				Resource: unifiedresources.Resource{
					Type: unifiedresources.ResourceTypeVM, Name: "other-firewall",
					Status: unifiedresources.StatusOnline, LastSeen: now,
					Proxmox: &unifiedresources.ProxmoxData{
						SourceID: makeGuestID("pve-b", "node1", 111),
						Instance: "pve-b", NodeName: "node1", VMID: 111, RuntimeStatus: "running",
					},
				},
			}})
			if len(registry.VMs()) != 2 {
				t.Fatal("duplicate VMIDs from separate instances were not preserved")
			}
			mon := &Monitor{state: models.NewState(), rateTracker: NewRateTracker(), config: &config.Config{}, resourceStore: unifiedresources.NewMonitorAdapter(registry)}
			prev := mon.previousGuestContextForInstance("pve-a")
			raw := VMMemoryRaw{}
			_, got, source, _ := mon.resolveGuestStatusMemory(context.Background(), &stubPVEClient{}, "pve-a", "firewall", "node1", 111, guestID, &proxmox.VMStatus{MaxMem: 8000, Mem: 8100}, prev.hostAgentsByVMID, 8000, "", &raw)
			if got != tc.want {
				t.Fatalf("used=%d source=%s want=%d", got, source, tc.want)
			}
			if tc.want == 2800 && (source != "agent" || raw.HostAgentUsed != 2800 || raw.HostAgentTotal != 8000) {
				t.Fatalf("missing agent diagnostic evidence: %s %+v", source, raw)
			}
			if tc.want == 8000 && source != "status-mem" {
				t.Fatalf("unsafe fallback source %s", source)
			}
			vm, buildRaw, buildSource, _, _, ok := mon.buildVMFromClusterResource(context.Background(), "pve-a",
				proxmox.ClusterResource{Type: "qemu", Status: "running", Node: "node1", VMID: 111, Name: "firewall", MaxMem: 8000, Mem: 8100},
				&correlatedMemoryClient{}, guestID, prev.hostAgentsByVMID, nil)
			if !ok || vm.Memory.Used != int64(tc.want) || vm.Memory.Usage != float64(tc.want)/80 || buildSource != source || buildRaw.HostAgentUsed != raw.HostAgentUsed {
				t.Fatalf("builder disagrees with resolver: %+v source=%s raw=%+v", vm.Memory, buildSource, buildRaw)
			}
			if source == "agent" {
				if vm.Memory.Cache != 4200 || vm.Memory.Free != 1000 || !vm.Memory.Observation.ObservedAt.Equal(now) {
					t.Fatalf("VM lost the current agent partition or its time: %+v", vm.Memory)
				}
			} else if vm.Memory.Cache != 0 {
				t.Fatalf("provider fallback borrowed agent cache: %+v", vm.Memory)
			}
			projection := unifiedresources.NewRegistry(nil)
			projection.IngestSnapshot(models.StateSnapshot{VMs: []models.VM{vm}})
			if got := projection.VMs()[0].MemoryPercent(); got != vm.Memory.Usage {
				t.Fatalf("read-state card projection=%v want=%v", got, vm.Memory.Usage)
			}

			manager := alerts.NewManagerWithDataDir(t.TempDir())
			defer manager.Stop()
			cfg := manager.GetConfig()
			cfg.Enabled = true
			cfg.ActivationState = alerts.ActivationActive
			cfg.GuestDefaults = alerts.ThresholdConfig{Memory: &alerts.HysteresisThreshold{Trigger: 85, Clear: 80}}
			cfg.TimeThresholds = map[string]int{"guest": 0}
			manager.UpdateConfig(cfg)
			mon.alertManager = manager
			for i := 0; i < 2; i++ {
				if _, ok := mon.handleClusterVMResource(context.Background(), "pve-a", proxmox.ClusterResource{Type: "qemu", Status: "running", Node: "node1", VMID: 111, Name: "firewall", MaxMem: 8000, Mem: 8100}, guestID, &correlatedMemoryClient{}, nil, prev.hostAgentsByVMID); !ok {
					t.Fatal("poll handler skipped VM")
				}
			}
			snapshots := mon.GetDiagnosticSnapshots().Guests
			if len(snapshots) != 1 || snapshots[0].Memory.Usage != vm.Memory.Usage || snapshots[0].MemorySource != source {
				t.Fatalf("diagnostics disagree: %+v", snapshots)
			}
			active := manager.GetActiveAlerts()
			if tc.want == 2800 && len(active) != 0 {
				t.Fatalf("35 percent generated alerts: %+v", active)
			}
			if tc.want == 8000 && (len(active) != 1 || active[0].Value != 100) {
				t.Fatalf("100 percent did not generate matching alert: %+v", active)
			}
			other := mon.previousGuestContextForInstance("pve-b")
			if len(other.hostAgentsByVMID) != 0 {
				t.Fatal("correlated agent crossed instance boundary")
			}
			otherID := makeGuestID("pve-b", "node1", 111)
			_, otherUsed, otherSource, _ := mon.resolveGuestStatusMemory(context.Background(), &stubPVEClient{}, "pve-b", "other-firewall", "node1", 111, otherID, &proxmox.VMStatus{MaxMem: 8000, Mem: 8100}, other.hostAgentsByVMID, 8000, "", &VMMemoryRaw{})
			if otherUsed != 8000 || otherSource != "status-mem" {
				t.Fatalf("other instance inherited agent memory: used=%d source=%s", otherUsed, otherSource)
			}
		})
	}
}

// Issue #2148: a correlated agent merged into a system-container resource must
// surface through the previous-guest context so the LXC builder prefers it over
// the cache-inclusive cluster/resources fallback.
func TestCorrelatedContainerMemoryNextPoll(t *testing.T) {
	now := time.Now()
	const gib = int64(1024 * 1024 * 1024)
	guestID := makeGuestID("pve-a", "node1", 100)
	store := unifiedresources.NewMemoryStore()
	if err := store.AddLink(unifiedresources.ResourceLink{ResourceA: "ct-test", ResourceB: "agent-test", PrimaryID: "agent-test"}); err != nil {
		t.Fatal(err)
	}
	registry := unifiedresources.NewRegistry(store)
	total, used := int64(24)*gib, int64(11)*gib
	resources := []unifiedresources.Resource{
		{ID: "ct-test", Type: unifiedresources.ResourceTypeSystemContainer, Name: "npu-ct", Status: unifiedresources.StatusOnline, LastSeen: now,
			Sources: []unifiedresources.DataSource{unifiedresources.SourceProxmox},
			Proxmox: &unifiedresources.ProxmoxData{SourceID: guestID, Instance: "pve-a", NodeName: "node1", VMID: 100, RuntimeStatus: "running"},
			Metrics: &unifiedresources.ResourceMetrics{Memory: &unifiedresources.MetricValue{Total: &total, Used: &used, Percent: 45, Source: unifiedresources.SourceProxmox}}},
		{ID: "agent-test", Type: unifiedresources.ResourceTypeAgent, Name: "npu-ct-agent", Status: unifiedresources.StatusOnline, LastSeen: now,
			Sources: []unifiedresources.DataSource{unifiedresources.SourceAgent},
			Agent:   &unifiedresources.AgentData{AgentID: "agent-test", Memory: &unifiedresources.AgentMemoryMeta{Total: total, Used: used, Free: total - used}}},
	}
	registry.IngestResources(resources)
	if len(registry.Containers()) != 1 || len(registry.Hosts()) != 0 {
		t.Fatalf("expected one merged container and no standalone hosts, got %d/%d", len(registry.Containers()), len(registry.Hosts()))
	}
	mon := &Monitor{state: models.NewState(), rateTracker: NewRateTracker(), config: &config.Config{}, resourceStore: unifiedresources.NewMonitorAdapter(registry)}
	prev := mon.previousGuestContextForInstance("pve-a")
	if _, ok := prev.hostAgentsByVMID[guestID]; !ok {
		t.Fatalf("linked container agent missing from previous context: %+v", prev.hostAgentsByVMID)
	}
	container, _, source, _, ok := mon.buildContainerFromClusterResource(context.Background(), "pve-a",
		proxmox.ClusterResource{Type: "lxc", Status: "running", Node: "node1", VMID: 100, Name: "npu-ct", MaxMem: uint64(total), Mem: 3459743744},
		&stubPVEClient{}, map[int]bool{}, prev.hostAgentsByVMID)
	if !ok || source != "agent" || container.Memory.Used != used {
		t.Fatalf("container memory not agent-backed: ok=%v source=%s mem=%+v", ok, source, container.Memory)
	}
}

type correlatedMemoryClient struct{ stubPVEClient }

func (*correlatedMemoryClient) GetVMStatus(context.Context, string, int) (*proxmox.VMStatus, error) {
	return &proxmox.VMStatus{MaxMem: 8000, Mem: 8100}, nil
}

// Automatic VM correlation is a link hint, not a cross-type registry merge.
// Keep this separate from TestCorrelatedGuestMemoryNextPoll's retained links.
func TestAutomaticGuestMemoryLinkNextPoll(t *testing.T) {
	now := time.Now()
	guestID := makeGuestID("pve-a", "node1", 111)
	otherID := makeGuestID("pve-b", "node1", 111)
	snapshot := models.StateSnapshot{VMs: []models.VM{
		{ID: guestID, Instance: "pve-a", Node: "node1", VMID: 111, Name: "firewall", Status: "running", LastSeen: now},
		{ID: otherID, Instance: "pve-b", Node: "node1", VMID: 111, Name: "other-firewall", Status: "running", LastSeen: now},
	}}
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(snapshot)
	mon := &Monitor{state: models.NewState(), rateTracker: NewRateTracker(), resourceStore: unifiedresources.NewMonitorAdapter(registry)}
	node, linkedVM, ct := mon.findLinkedProxmoxEntityWithHints("firewall.example.test", "", nil)
	if node != "" || linkedVM != guestID || ct != "" {
		t.Fatalf("automatic link = %q/%q/%q", node, linkedVM, ct)
	}
	snapshot.Hosts = []models.Host{{ID: "guest-agent", Hostname: "firewall.example.test", LinkedVMID: linkedVM, Status: "online", LastSeen: now, Memory: models.Memory{Total: 8000, Used: 2800, Cache: 4200, Free: 1000, Usage: 35}}}
	// Reconstruct using source snapshots, without a manual link store.
	registry = unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(snapshot)
	mon.resourceStore = unifiedresources.NewMonitorAdapter(registry)
	if len(registry.VMs()) != 2 || len(registry.Hosts()) != 1 {
		t.Fatal("automatic hint unexpectedly merged the agent into a VM")
	}
	for _, tc := range []struct {
		instance, id string
		used         uint64
		source       string
	}{
		{"pve-a", guestID, 2800, "agent"}, {"pve-b", otherID, 8000, "status-mem"},
	} {
		prev := mon.previousGuestContextForInstance(tc.instance)
		_, used, source, _ := mon.resolveGuestStatusMemory(context.Background(), &stubPVEClient{}, tc.instance, "firewall", "node1", 111, tc.id, &proxmox.VMStatus{MaxMem: 8000, Mem: 8100}, prev.hostAgentsByVMID, 8000, "", &VMMemoryRaw{})
		if used != tc.used || source != tc.source {
			t.Fatalf("%s used=%d source=%s", tc.instance, used, source)
		}
		vm, _, builtSource, _, _, ok := mon.buildVMFromClusterResource(context.Background(), tc.instance,
			proxmox.ClusterResource{Type: "qemu", Status: "running", Node: "node1", VMID: 111, Name: "firewall", MaxMem: 8000, Mem: 8100},
			&correlatedMemoryClient{}, tc.id, prev.hostAgentsByVMID, nil)
		if !ok || vm.Memory.Used != int64(tc.used) || builtSource != tc.source {
			t.Fatalf("%s VM selected %+v via %s", tc.instance, vm.Memory, builtSource)
		}
		if tc.source == "agent" {
			if vm.Memory.Cache != 4200 || vm.Memory.Free != 1000 || !vm.Memory.Observation.ObservedAt.Equal(now) {
				t.Fatalf("automatic link flattened the agent partition or changed its time: %+v", vm.Memory)
			}
		} else if vm.Memory.Cache != 0 {
			t.Fatalf("%s borrowed another instance's cache: %+v", tc.instance, vm.Memory)
		}
	}
	// A matching name in another instance must prevent automatic selection.
	snapshot.VMs[1].Name = "firewall"
	registry = unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(snapshot)
	mon.resourceStore = unifiedresources.NewMonitorAdapter(registry)
	node, linkedVM, ct = mon.findLinkedProxmoxEntityWithHints("firewall.example.test", "", nil)
	if node != "" || linkedVM != "" || ct != "" {
		t.Fatalf("ambiguous hostname selected %q/%q/%q", node, linkedVM, ct)
	}
}

// shortenTenantShutdownBudgets makes the shutdown waits fast enough for tests
// while keeping deletion's budget longer than the plain one, as in production.
func shortenTenantShutdownBudgets(t *testing.T, plain, deletion time.Duration) {
	t.Helper()
	previousPlain, previousDeletion := tenantMonitorShutdownTimeout, tenantDeletionShutdownTimeout
	tenantMonitorShutdownTimeout, tenantDeletionShutdownTimeout = plain, deletion
	t.Cleanup(func() {
		tenantMonitorShutdownTimeout, tenantDeletionShutdownTimeout = previousPlain, previousDeletion
	})
}

// A tenant monitor that is still starting up or finishing a tick can take
// longer than the plain shutdown budget to see its cancelled context. In CI's
// mock mode, Start seeds mock metrics history synchronously before its select
// loop, so a just-used organization's loop took 8-10s to stop and deletion
// refused with tenant_shutdown_incomplete. Deletion waits on its own, longer
// budget instead.
func TestTenantDeletionWaitsForSlowMonitoringLoop(t *testing.T) {
	shortenTenantShutdownBudgets(t, 50*time.Millisecond, 2*time.Second)

	slowLoop := func(mtm *MultiTenantMonitor, orgID string) *Monitor {
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() {
			<-ctx.Done()
			// Exits well after the plain budget but inside deletion's.
			time.Sleep(300 * time.Millisecond)
			close(done)
		}()
		monitor := &Monitor{}
		mtm.monitors[orgID] = monitor
		mtm.tenantCancel[orgID] = cancel
		mtm.tenantDone[orgID] = done
		return monitor
	}

	mtm := NewMultiTenantMonitor(&config.Config{}, nil, nil)
	t.Cleanup(mtm.Stop)

	// RemoveTenant keeps the plain budget, gives up and retains the owner.
	retained := slowLoop(mtm, "removal")
	mtm.RemoveTenant("removal")
	assertTenantCleanupOwner(t, mtm, "removal", retained)

	slowLoop(mtm, "deletion")
	if err := mtm.BeginTenantDeletion("deletion"); err != nil {
		t.Fatalf("deletion refused a loop that stopped inside its budget: %v", err)
	}
	if _, ok := mtm.PeekMonitor("deletion"); ok {
		t.Fatal("completed deletion left the monitor registered")
	}
	mtm.FinishTenantDeletion("deletion")
}

// The poller reads the previous poll's nodes back through the unified read
// state. Those lookups key on the Proxmox source ID, so the projection must
// return it (not the registry's unified resource ID) and carry the stored
// temperature; otherwise the node's CPU low/record values reset to the
// current reading every poll and a failed collection never carries over.
func TestPreviousNodeProjectionFeedsPollerTemperatureTracking(t *testing.T) {
	instanceCfg := config.PVEInstance{Name: "pve1", Host: "https://10.0.0.1:8006"}
	adapter := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
	m := &Monitor{
		config:        &config.Config{TemperatureMonitoringEnabled: true, PVEInstances: []config.PVEInstance{instanceCfg}},
		state:         models.NewState(),
		resourceStore: adapter,
	}
	base := models.Node{ID: "pve1-node1", NodeIdentity: "pve1-node1", Name: "node1", Instance: "pve1", Status: "online", Type: "node"}

	// publish rebuilds the unified store from state the way the monitor does,
	// alongside the host agents' latest reports.
	var hosts []models.Host
	publish := func() {
		snapshot := m.state.GetSnapshot()
		snapshot.Hosts = hosts
		adapter.PopulateFromSnapshot(snapshot)
	}
	// poll delivers an agent report (none when agentReading is 0), runs one PVE
	// poll's temperature step against the previous poll's nodes, and publishes
	// the polled node.
	poll := func(agentReading float64) *models.Temperature {
		t.Helper()
		hosts = nil
		if agentReading > 0 {
			hosts = []models.Host{{
				ID: "agent-1", Hostname: "node1", Status: "online", LinkedNodeID: base.ID, IntervalSeconds: 30, LastSeen: time.Now(),
				Sensors: models.HostSensorSummary{TemperatureCelsius: map[string]float64{"cpu_package": agentReading}},
			}}
		}
		publish()
		prevNodes := m.snapshotPrevNodes("pve1")
		polled := base
		polled.LastSeen = time.Now()
		m.collectNodeTemperatureData(context.Background(), "pve1", &instanceCfg, proxmox.Node{Node: "node1"}, &polled, prevNodes, "online")
		m.state.UpdateNodesForInstance("pve1", []models.Node{polled})
		publish()
		return polled.Temperature
	}

	if temp := poll(50); temp == nil || temp.CPUMin != 50 || temp.CPUMaxRecord != 50 {
		t.Fatalf("first reading = %#v, want min and record initialised to 50", temp)
	}
	prevNodes := m.snapshotPrevNodes("pve1")
	if len(prevNodes) != 1 || prevNodes[0].ID != base.ID || prevNodes[0].Temperature == nil {
		t.Fatalf("previous nodes = %#v, want source ID %q with its temperature", prevNodes, base.ID)
	}

	if temp := poll(62); temp == nil || temp.CPUPackage != 62 || temp.CPUMin != 50 || temp.CPUMaxRecord != 62 {
		t.Fatalf("second reading = %#v, want current 62, low 50, record 62", temp)
	}
	third := poll(45)
	if third == nil || third.CPUPackage != 45 || third.CPUMin != 45 || third.CPUMaxRecord != 62 {
		t.Fatalf("third reading = %#v, want current 45, low 45, record 62", third)
	}

	// No agent and no SSH collector: the last reading carries over with its
	// original timestamp, inside the carry window.
	carried := poll(0)
	if carried == nil || carried.CPUPackage != 45 || !carried.LastUpdate.Equal(third.LastUpdate) {
		t.Fatalf("carried reading = %#v, want the previous 45 with its original timestamp", carried)
	}
}

// When a whole Proxmox instance stops answering, its last-known nodes are
// written back into state through the offline grace policy. That writeback
// must keep each node the same resource: the poller's source ID and the
// config-derived identity the registry derives a cluster member's canonical ID
// from. Past grace the node goes offline and stops presenting a temperature.
func TestPVEOutageWritebackKeepsNodeIdentity(t *testing.T) {
	for _, tc := range []struct {
		name           string
		instances      []config.PVEInstance
		wantID         string
		providerScoped bool
	}{
		{
			name:      "cluster member",
			instances: []config.PVEInstance{{Name: "pve-a", Host: "https://10.0.0.1:8006", IsCluster: true, ClusterName: "lab"}},
			wantID:    "lab-node1",
		},
		{
			name: "cluster name shared by two connections",
			instances: []config.PVEInstance{
				{Name: "pve-a", Host: "https://10.0.0.1:8006", IsCluster: true, ClusterName: "lab"},
				{Name: "pve-b", Host: "https://10.0.1.1:8006", IsCluster: true, ClusterName: "lab"},
			},
			wantID:         "pve-a-node1",
			providerScoped: true,
		},
		{
			name: "cluster named after another connection",
			instances: []config.PVEInstance{
				{Name: "pve-a", Host: "https://10.0.0.1:8006", IsCluster: true, ClusterName: "lab"},
				{Name: "lab", Host: "https://10.0.1.1:8006"},
			},
			wantID:         "pve-a-node1",
			providerScoped: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			adapter := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
			m := &Monitor{
				config:         &config.Config{PVEInstances: tc.instances},
				state:          models.NewState(),
				resourceStore:  adapter,
				nodeLastOnline: make(map[string]time.Time),
			}
			instanceCfg := m.getInstanceConfig("pve-a")
			if got := m.pveNodeID("pve-a", instanceCfg, "node1"); got != tc.wantID {
				t.Fatalf("pveNodeID = %q, want %q", got, tc.wantID)
			}
			now := time.Now()
			polled := m.placeholderNodeForInstance("pve-a", instanceCfg, "node1")
			polled.Status = "online"
			polled.ConnectionHealth = "healthy"
			polled.LastSeen = now
			polled.Temperature = &models.Temperature{CPUPackage: 55, Available: true, HasCPU: true, LastUpdate: now}
			if polled.ProviderScopedIdentity != tc.providerScoped {
				t.Fatalf("ProviderScopedIdentity = %v, want %v", polled.ProviderScopedIdentity, tc.providerScoped)
			}
			m.nodeLastOnline[polled.ID] = now
			m.state.UpdateNodesForInstance("pve-a", []models.Node{polled})
			adapter.PopulateFromSnapshot(m.state.GetSnapshot())
			unifiedID := adapter.Nodes()[0].ID()

			for cycle := 0; cycle < 3; cycle++ {
				m.markPVEInstanceNodesUnreachable("pve-a")
				adapter.PopulateFromSnapshot(m.state.GetSnapshot())
				stateNodes := m.state.GetSnapshot().Nodes
				if len(stateNodes) != 1 || stateNodes[0].ID != tc.wantID || stateNodes[0].ClusterName != "lab" ||
					stateNodes[0].ProviderScopedIdentity != tc.providerScoped {
					t.Fatalf("cycle %d: state nodes = %#v, want %q kept with its cluster identity", cycle, stateNodes, tc.wantID)
				}
				if stateNodes[0].Status != "online" || stateNodes[0].Temperature == nil {
					t.Fatalf("cycle %d: node within grace = %#v, want online with its recent temperature", cycle, stateNodes[0])
				}
				if nodes := adapter.Nodes(); len(nodes) != 1 || nodes[0].ID() != unifiedID {
					t.Fatalf("cycle %d: unified nodes = %v, want the single resource %q", cycle, nodes, unifiedID)
				}
			}

			// The process last saw the node online beyond the grace period.
			m.nodeLastOnline[polled.ID] = now.Add(-2 * nodeOfflineGracePeriod)
			polled.LastSeen = now.Add(-2 * nodeOfflineGracePeriod)
			m.state.UpdateNodesForInstance("pve-a", []models.Node{polled})
			adapter.PopulateFromSnapshot(m.state.GetSnapshot())
			m.markPVEInstanceNodesUnreachable("pve-a")
			adapter.PopulateFromSnapshot(m.state.GetSnapshot())
			expired := m.state.GetSnapshot().Nodes
			if len(expired) != 1 || expired[0].ID != tc.wantID || expired[0].Status != "offline" || expired[0].Temperature != nil {
				t.Fatalf("expired nodes = %#v, want %q offline without a temperature", expired, tc.wantID)
			}
			if nodes := adapter.Nodes(); len(nodes) != 1 || nodes[0].ID() != unifiedID {
				t.Fatalf("unified nodes after expiry = %v, want the single resource %q", nodes, unifiedID)
			}
		})
	}
}

// Within grace a preserved node keeps its temperature only while the reading
// is inside the same carry window a failed collection gets.
func TestPreserveOrExpireNodesBoundsGraceTemperature(t *testing.T) {
	m := &Monitor{config: &config.Config{}, state: models.NewState(), nodeLastOnline: make(map[string]time.Time)}
	now := time.Now()
	m.nodeLastOnline["pve-node1"] = now
	stale := now.Add(-m.nodeTemperatureCarryWindow() - time.Minute)
	preserved := m.preserveOrExpireNodes([]models.Node{{
		ID: "pve-node1", Name: "node1", Status: "online", LastSeen: now,
		Temperature: &models.Temperature{CPUPackage: 70, Available: true, LastUpdate: stale},
	}})
	if len(preserved) != 1 || preserved[0].Status != "online" || preserved[0].Temperature != nil {
		t.Fatalf("preserved = %#v, want online within grace without the stale temperature", preserved)
	}
}

// When detailed node status fails, the poller may carry its own last trusted
// memory reading for one cycle. On a node merged with a host agent, the read
// state holds the agent's memory, which is not the Proxmox value the snapshot
// vouches for, so the carry must use the snapshot and keep its used, cache
// and free split summing to the total.
func TestPollPVENodeCarriesOwnMemorySnapshot(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())
	const gib = uint64(1024 * 1024 * 1024)

	mon := newTestPVEMonitor("test")
	defer mon.alertManager.Stop()
	defer mon.notificationMgr.Stop()
	adapter := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
	mon.resourceStore = adapter

	client := &stubPVEClient{nodeStatus: &proxmox.NodeStatus{
		Memory: &proxmox.MemoryStatus{Total: 16 * gib, Used: 12 * gib, Free: 1 * gib, Available: 8 * gib},
	}}
	node := proxmox.Node{Node: "node1", Status: "online", MaxMem: 16 * gib, Mem: 15 * gib, MaxCPU: 8}

	first, _, _, err := mon.pollPVENode(context.Background(), "test", &mon.config.PVEInstances[0], client, node, "healthy", nil)
	if err != nil {
		t.Fatalf("first pollPVENode() error = %v", err)
	}
	if !first.Memory.HasKnownUsage() || first.Memory.Used != int64(8*gib) || first.Memory.Cache == 0 {
		t.Fatalf("first.Memory = %+v, want 8 GiB used with reclaimable cache split out", first.Memory)
	}
	mon.state.UpdateNodesForInstance("test", []models.Node{first})
	snapshot := mon.state.GetSnapshot()
	snapshot.Hosts = []models.Host{{
		ID: "agent-1", Hostname: "node1", Status: "online", LinkedNodeID: first.ID, IntervalSeconds: 30, LastSeen: time.Now(),
		Memory: models.Memory{Total: int64(16 * gib), Used: int64(3 * gib), Free: int64(13 * gib), Usage: 18.75},
	}}
	adapter.PopulateFromSnapshot(snapshot)

	client.nodeStatus = nil
	second, _, _, err := mon.pollPVENode(context.Background(), "test", &mon.config.PVEInstances[0], client, node, "healthy", mon.snapshotPrevNodes("test"))
	if err != nil {
		t.Fatalf("second pollPVENode() error = %v", err)
	}
	if second.Memory.Used != first.Memory.Used || second.Memory.Cache != first.Memory.Cache || second.Memory.Free != first.Memory.Free {
		t.Fatalf("second.Memory = %+v, want the poller's own reading %+v", second.Memory, first.Memory)
	}
	if second.Memory.Used+second.Memory.Cache+second.Memory.Free != second.Memory.Total {
		t.Fatalf("second.Memory = %+v, want used + cache + free = total", second.Memory)
	}
	if snap := mon.nodeSnapshots[makeNodeSnapshotKey("test", "node1")]; snap.MemorySource != "previous-snapshot" {
		t.Fatalf("snapshot.MemorySource = %q, want previous-snapshot", snap.MemorySource)
	}
}

// Authoritative cluster membership writes previous nodes back too. Read from
// the unified store, an offline member keeps its source ID and agent link but
// stops presenting a temperature, and a member missing from /nodes keeps a
// reading only inside the carry window.
func TestPVEMembershipReconcileKeepsPreviousNodeIdentity(t *testing.T) {
	cfg := &config.Config{PVEInstances: []config.PVEInstance{{
		Name: "cluster-api", Host: "https://pve-a:8006", IsCluster: true, ClusterName: "production",
	}}}
	m := newUnreachableTestMonitor(t, cfg)
	adapter := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
	m.resourceStore = adapter
	instanceCfg := &cfg.PVEInstances[0]
	now := time.Now()
	polled := func(name string, tempAge time.Duration) models.Node {
		node := m.placeholderNodeForInstance("cluster-api", instanceCfg, name)
		node.Status = "online"
		node.ConnectionHealth = "healthy"
		node.LastSeen = now
		node.Temperature = &models.Temperature{CPUPackage: 60, Available: true, HasCPU: true, LastUpdate: now.Add(-tempAge)}
		return node
	}
	offlineMember := polled("pve-a", 0)
	offlineMember.LinkedAgentID = "agent-pve-a"
	m.state.UpdateNodesForInstance("cluster-api", []models.Node{
		offlineMember, polled("pve-b", 0), polled("pve-c", m.nodeTemperatureCarryWindow()+time.Minute),
	})
	adapter.PopulateFromSnapshot(m.state.GetSnapshot())

	client := &membershipPVEClient{statuses: []proxmox.ClusterStatus{
		{Type: "cluster", Name: "production", Quorate: 1},
		{Type: "node", Name: "pve-a", Online: 0},
		{Type: "node", Name: "pve-b", Online: 1},
		{Type: "node", Name: "pve-c", Online: 1},
	}}
	got := pveNodeByName(m.reconcilePVENodeInventory(
		context.Background(), "cluster-api", instanceCfg, client, nil, m.snapshotPrevNodes("cluster-api"),
	))

	if node := got["pve-a"]; node.ID != "production-pve-a" || node.Status != "offline" || node.Temperature != nil || node.LinkedAgentID != "agent-pve-a" {
		t.Fatalf("offline member = %#v, want production-pve-a offline with its agent link and no temperature", node)
	}
	if node := got["pve-b"]; node.ID != "production-pve-b" || node.Status != "unknown" || node.Temperature == nil {
		t.Fatalf("unobserved member = %#v, want production-pve-b unknown with its recent temperature", node)
	}
	if node := got["pve-c"]; node.ID != "production-pve-c" || node.Status != "unknown" || node.Temperature != nil {
		t.Fatalf("unobserved member = %#v, want production-pve-c unknown without its stale temperature", node)
	}

	// Writing the reconciled inventory back keeps every member the same
	// unified resource.
	unifiedIDs := func() map[string]string {
		ids := make(map[string]string)
		for _, node := range adapter.Nodes() {
			ids[node.NodeName()] = node.ID()
		}
		return ids
	}
	before := unifiedIDs()
	m.state.UpdateNodesForInstance("cluster-api", sortedPVENodeInventory(got))
	adapter.PopulateFromSnapshot(m.state.GetSnapshot())
	if after := unifiedIDs(); len(after) != 3 || after["pve-a"] != before["pve-a"] || after["pve-b"] != before["pve-b"] || after["pve-c"] != before["pve-c"] {
		t.Fatalf("unified IDs after writeback = %v, want %v", after, before)
	}
}

// A temperature carried over from an earlier poll keeps a lapsed host agent's
// lease: a reading the agent supplied (stamped no later than its last report)
// is dropped once the agent stops reporting, even when its latest report had
// no sensors, while a reading taken after that report keeps the carry window.
// The same rule bounds what an instance outage preserves within node grace.
func TestNodeTemperatureCarryRespectsAgentLease(t *testing.T) {
	now := time.Now()
	silentSince := now.Add(-hostAgentHealthWindow(30) - time.Minute)
	instanceCfg := config.PVEInstance{Name: "pve1", Host: "https://10.0.0.1:8006"}
	base := models.Node{ID: "pve1-node1", NodeIdentity: "pve1-node1", Name: "node1", Instance: "pve1", Status: "online", Type: "node", LastSeen: now}

	for _, tc := range []struct {
		name         string
		readingAt    time.Time
		agentSensors bool
		wantCarried  bool
	}{
		{name: "reading from the silent agent", readingAt: silentSince, agentSensors: true},
		{name: "silent agent whose last report had no sensors", readingAt: silentSince.Add(-30 * time.Second)},
		{name: "SSH reading taken after the agent went silent", readingAt: now.Add(-time.Minute), agentSensors: true, wantCarried: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			adapter := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
			m := &Monitor{
				config:         &config.Config{TemperatureMonitoringEnabled: true, PVEInstances: []config.PVEInstance{instanceCfg}},
				state:          models.NewState(),
				resourceStore:  adapter,
				nodeLastOnline: map[string]time.Time{base.ID: now},
			}
			agent := models.Host{ID: "agent-1", Hostname: "node1", Status: "offline", LinkedNodeID: base.ID, IntervalSeconds: 30, LastSeen: silentSince}
			if tc.agentSensors {
				agent.Sensors = models.HostSensorSummary{TemperatureCelsius: map[string]float64{"cpu_package": 92}}
			}
			published := base
			published.Temperature = &models.Temperature{CPUPackage: 92, CPUMin: 92, CPUMaxRecord: 92, Available: true, HasCPU: true, LastUpdate: tc.readingAt}
			m.state.UpdateNodesForInstance("pve1", []models.Node{published})
			snapshot := m.state.GetSnapshot()
			snapshot.Hosts = []models.Host{agent}
			adapter.PopulateFromSnapshot(snapshot)

			polled := base
			m.collectNodeTemperatureData(context.Background(), "pve1", &instanceCfg, proxmox.Node{Node: "node1"}, &polled, m.snapshotPrevNodes("pve1"), "online")
			if carried := polled.Temperature != nil; carried != tc.wantCarried {
				t.Fatalf("failed collection carried = %v (%#v), want %v", carried, polled.Temperature, tc.wantCarried)
			}

			preserved := m.preserveOrExpireNodes(m.snapshotPrevNodes("pve1"))
			if len(preserved) != 1 || preserved[0].Status != "online" {
				t.Fatalf("preserved = %#v, want the node online within grace", preserved)
			}
			if kept := preserved[0].Temperature != nil; kept != tc.wantCarried {
				t.Fatalf("outage preservation kept temperature = %v, want %v", kept, tc.wantCarried)
			}
		})
	}
}

// A carried node reading keeps the node's CPU temperature steady through a
// failed collection, but never its disk temperatures: those also stamp the
// node's physical disks, which have their own freshness and standby state.
// Here a current agent whose only sensor is a SMART disk reports that disk in
// standby, so collection returns nothing; the disk must not be re-stamped with
// its pre-sleep reading through the carry.
func TestCarriedNodeTemperatureLeavesDiskReadingsOut(t *testing.T) {
	now := time.Now()
	instanceCfg := config.PVEInstance{Name: "pve1", Host: "https://10.0.0.1:8006"}
	base := models.Node{ID: "pve1-node1", NodeIdentity: "pve1-node1", Name: "node1", Instance: "pve1", Status: "online", Type: "node", LastSeen: now}
	disk := models.PhysicalDisk{ID: "pve1-node1-sda", Node: "node1", Instance: "pve1", DevPath: "/dev/sda", Serial: "WD-STANDBY", Type: "hdd"}

	for _, tc := range []struct {
		name     string
		previous models.Temperature
		wantCPU  bool
	}{
		{
			name: "CPU and disk reading",
			previous: models.Temperature{
				Available: true, HasCPU: true, CPUPackage: 50, HasSMART: true, HasNVMe: true,
				SMART: []models.DiskTemp{{Device: "/dev/sda", Serial: "WD-STANDBY", Temperature: 38}},
				NVMe:  []models.NVMeTemp{{Device: "nvme0", Temp: 44}},
			},
			wantCPU: true,
		},
		{
			name: "disk-only reading",
			previous: models.Temperature{
				Available: true, HasSMART: true,
				SMART: []models.DiskTemp{{Device: "/dev/sda", Serial: "WD-STANDBY", Temperature: 38}},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			adapter := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
			m := &Monitor{
				config:         &config.Config{TemperatureMonitoringEnabled: true, PVEInstances: []config.PVEInstance{instanceCfg}},
				state:          models.NewState(),
				resourceStore:  adapter,
				nodeLastOnline: map[string]time.Time{base.ID: now},
			}
			published := base
			previous := tc.previous
			previous.LastUpdate = now.Add(-time.Minute)
			published.Temperature = &previous
			m.state.UpdateNodesForInstance("pve1", []models.Node{published})
			snapshot := m.state.GetSnapshot()
			snapshot.Hosts = []models.Host{{
				ID: "agent-1", Hostname: "node1", Status: "online", LinkedNodeID: base.ID, IntervalSeconds: 30, LastSeen: now,
				Sensors: models.HostSensorSummary{SMART: []models.HostDiskSMART{{Device: "sda", Serial: "WD-STANDBY", Standby: true}}},
			}}
			adapter.PopulateFromSnapshot(snapshot)

			polled := base
			m.collectNodeTemperatureData(context.Background(), "pve1", &instanceCfg, proxmox.Node{Node: "node1"}, &polled, m.snapshotPrevNodes("pve1"), "online")
			if tc.wantCPU {
				if polled.Temperature == nil || polled.Temperature.CPUPackage != 50 || !polled.Temperature.LastUpdate.Equal(previous.LastUpdate) {
					t.Fatalf("carried temperature = %#v, want CPU 50 with its original timestamp", polled.Temperature)
				}
				if len(polled.Temperature.SMART) != 0 || len(polled.Temperature.NVMe) != 0 || polled.Temperature.HasSMART || polled.Temperature.HasNVMe {
					t.Fatalf("carried temperature kept disk readings: %#v", polled.Temperature)
				}
			} else if polled.Temperature != nil {
				t.Fatalf("carried temperature = %#v, want none when only disk readings were left", polled.Temperature)
			}
			if stamped := mergeNVMeTempsIntoDisks([]models.PhysicalDisk{disk}, []models.Node{polled}); stamped[0].Temperature != 0 {
				t.Fatalf("standby disk stamped with %d°C from a carried reading, want no reading", stamped[0].Temperature)
			}

			// Outage preservation follows the same rule.
			preserved := m.preserveOrExpireNodes(m.snapshotPrevNodes("pve1"))
			if len(preserved) != 1 {
				t.Fatalf("preserved = %#v, want one node", preserved)
			}
			if got := preserved[0].Temperature; (got != nil) != tc.wantCPU || (got != nil && (len(got.SMART) != 0 || len(got.NVMe) != 0)) {
				t.Fatalf("preserved temperature = %#v, want CPU only (%v)", got, tc.wantCPU)
			}
		})
	}
}

// Two Proxmox connections can each have a node with the same name (one "px1"
// per site). The poller's temperature lookup must never hand one site's node the
// host agent, or the agent's cluster-sibling SSH readings, of the other.
func TestNodeTemperatureStaysWithinSameNamedNodesSite(t *testing.T) {
	instanceCfg := config.PVEInstance{Name: "siteB"}
	poll := func(m *Monitor, instance, nodeName string) *models.Temperature {
		m.config = &config.Config{TemperatureMonitoringEnabled: true}
		polled := readStateTestNode(instance, nodeName)
		m.collectNodeTemperatureData(context.Background(), instance, &instanceCfg, proxmox.Node{Node: nodeName}, &polled, nil, "online")
		return polled.Temperature
	}
	sites := func() []models.Node {
		return []models.Node{readStateTestNode("siteA", "px1"), readStateTestNode("siteA", "px2"), readStateTestNode("siteB", "px1")}
	}

	t.Run("agent linked to the other site's node", func(t *testing.T) {
		m := readStateMonitor(sites(), []models.Host{reportingTestHost("agent-a", "px1", "siteA-px1", 77)})
		if temp := poll(m, "siteA", "px1"); temp == nil || temp.CPUPackage != 77 {
			t.Fatalf("siteA px1 temperature = %#v, want its linked agent's 77", temp)
		}
		if temp := poll(m, "siteB", "px1"); temp != nil {
			t.Fatalf("siteB px1 took site A's agent reading: %#v", temp)
		}
	})

	t.Run("unlinked agent whose hostname several nodes share", func(t *testing.T) {
		m := readStateMonitor(sites(), []models.Host{reportingTestHost("agent-x", "px1", "", 66)})
		for _, instance := range []string{"siteA", "siteB"} {
			if temp := poll(m, instance, "px1"); temp != nil {
				t.Fatalf("%s px1 took an agent the linker could not place: %#v", instance, temp)
			}
		}
	})

	t.Run("unlinked agent whose hostname one node has", func(t *testing.T) {
		m := readStateMonitor(sites(), []models.Host{reportingTestHost("agent-y", "px2", "", 61)})
		if temp := poll(m, "siteA", "px2"); temp == nil || temp.CPUPackage != 61 {
			t.Fatalf("siteA px2 temperature = %#v, want the unambiguous agent's 61", temp)
		}
	})

	t.Run("two unlinked agents with the node's hostname", func(t *testing.T) {
		m := readStateMonitor(sites(), []models.Host{reportingTestHost("agent-y1", "px2", "", 61), reportingTestHost("agent-y2", "px2", "", 62)})
		if temp := poll(m, "siteA", "px2"); temp != nil {
			t.Fatalf("siteA px2 took one of two candidate agents: %#v", temp)
		}
	})

	t.Run("one cluster added through two connections", func(t *testing.T) {
		// The state folds both views of pve01 into one slot, held under whichever
		// view merged last, and the agent link follows the slot. Each view's poll
		// must still take the agent's reading.
		var instances []config.PVEInstance
		for _, name := range []string{"enacon-a", "enacon-b"} {
			instances = append(instances, config.PVEInstance{
				Name: name, IsCluster: true, ClusterName: "enacon",
				ClusterEndpoints: []config.ClusterEndpoint{{NodeName: "pve01", Host: "https://192.168.1.11:8006", Fingerprint: "AA:AA"}},
			})
		}
		adapter := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
		m := &Monitor{config: &config.Config{TemperatureMonitoringEnabled: true, PVEInstances: instances}, state: models.NewState(), resourceStore: adapter}
		view := func(instanceCfg *config.PVEInstance) models.Node {
			return models.Node{
				ID: instanceCfg.Name + "-pve01", NodeIdentity: instanceCfg.Name + "-pve01", Name: "pve01", Instance: instanceCfg.Name,
				ClusterName: "enacon", IsClusterMember: true, Host: "https://192.168.1.11:8006",
				TLSFingerprint: pveNodeTLSFingerprint(instanceCfg, "pve01"), Status: "online", LastSeen: time.Now(),
			}
		}
		m.state.UpdateNodesForInstance("enacon-a", []models.Node{view(&instances[0])})
		m.state.UpsertHost(reportingTestHost("agent-pve01", "pve01", "", 64))
		if err := m.state.LinkHostAgentToNode("agent-pve01", "enacon-a-pve01"); err != nil {
			t.Fatalf("LinkHostAgentToNode: %v", err)
		}
		adapter.PopulateFromSnapshot(m.state.GetSnapshot())

		for round := 0; round < 3; round++ {
			for i := range instances {
				polled := view(&instances[i])
				m.collectNodeTemperatureData(context.Background(), instances[i].Name, &instances[i], proxmox.Node{Node: "pve01"}, &polled, nil, "online")
				if polled.Temperature == nil || polled.Temperature.CPUPackage != 64 {
					t.Fatalf("round %d %s temperature = %#v, want the folded node's agent reading 64", round, instances[i].Name, polled.Temperature)
				}
				m.state.UpdateNodesForInstance(instances[i].Name, []models.Node{polled})
				adapter.PopulateFromSnapshot(m.state.GetSnapshot())
			}
		}
		if nodes := m.state.GetSnapshot().Nodes; len(nodes) != 1 {
			t.Fatalf("state holds %d nodes, want both views folded into one", len(nodes))
		}
	})

	t.Run("unclustered node whose address two sites' namesakes share", func(t *testing.T) {
		// Both site nodes would fold with the polled view by address alone; the
		// state declines an ambiguous alias, so the lookup must take neither.
		nodes := sites()
		for i := range nodes {
			nodes[i].ClusterName = map[string]string{"siteA": "a", "siteB": "b"}[nodes[i].Instance]
			nodes[i].Host = "https://10.0.0.5:8006"
		}
		m := readStateMonitor(nodes, []models.Host{reportingTestHost("agent-a", "px1", "siteA-px1", 77)})
		polled := readStateTestNode("siteC", "px1")
		polled.Host = "https://10.0.0.5:8006"
		m.config = &config.Config{TemperatureMonitoringEnabled: true}
		m.collectNodeTemperatureData(context.Background(), "siteC", &config.PVEInstance{Name: "siteC"}, proxmox.Node{Node: "px1"}, &polled, nil, "online")
		if polled.Temperature != nil {
			t.Fatalf("siteC px1 took a namesake's agent reading: %#v", polled.Temperature)
		}
	})

	t.Run("multi-homed host added through two standalone connections", func(t *testing.T) {
		// The agent reports both connection addresses, so the state folds the two
		// views into one slot through it. Each view's poll must take its reading.
		adapter := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
		m := &Monitor{config: &config.Config{TemperatureMonitoringEnabled: true}, state: models.NewState(), resourceStore: adapter}
		views := map[string]string{"lan-a": "https://10.0.0.5:8006", "lan-b": "https://10.0.1.5:8006"}
		view := func(instance string) models.Node {
			polled := readStateTestNode(instance, "minipc")
			polled.Host = views[instance]
			return polled
		}
		m.state.UpdateNodesForInstance("lan-a", []models.Node{view("lan-a")})
		agent := reportingTestHost("agent-minipc", "minipc", "", 64)
		agent.ReportIP = "10.0.0.5"
		agent.NetworkInterfaces = []models.HostNetworkInterface{{Name: "eth0", Addresses: []string{"10.0.0.5/24"}}, {Name: "eth1", Addresses: []string{"10.0.1.5/24"}}}
		m.state.UpsertHost(agent)
		if err := m.state.LinkHostAgentToNode("agent-minipc", "lan-a-minipc"); err != nil {
			t.Fatalf("LinkHostAgentToNode: %v", err)
		}
		adapter.PopulateFromSnapshot(m.state.GetSnapshot())

		for round := 0; round < 3; round++ {
			for _, instance := range []string{"lan-a", "lan-b"} {
				polled := view(instance)
				m.collectNodeTemperatureData(context.Background(), instance, &config.PVEInstance{Name: instance}, proxmox.Node{Node: "minipc"}, &polled, nil, "online")
				if polled.Temperature == nil || polled.Temperature.CPUPackage != 64 {
					t.Fatalf("round %d %s temperature = %#v, want the folded host's agent reading 64", round, instance, polled.Temperature)
				}
				m.state.UpdateNodesForInstance(instance, []models.Node{polled})
				adapter.PopulateFromSnapshot(m.state.GetSnapshot())
			}
		}
		if nodes := m.state.GetSnapshot().Nodes; len(nodes) != 1 {
			t.Fatalf("state holds %d nodes, want both views folded into one", len(nodes))
		}
	})

	t.Run("namesake's agent reporting the polled node's address", func(t *testing.T) {
		// Site A's px1 agent also reports 10.0.1.5, the address site B's px1
		// uses. The fingerprints contradict, so site B's node is another machine.
		siteA := readStateTestNode("siteA", "px1")
		siteA.Host = "https://10.0.0.5:8006"
		agent := reportingTestHost("agent-a", "px1", "siteA-px1", 77)
		agent.ReportIP = "10.0.0.5"
		agent.NetworkInterfaces = []models.HostNetworkInterface{{Name: "eth0", Addresses: []string{"10.0.0.5/24", "10.0.1.5/24"}}}
		m := readStateMonitor([]models.Node{siteA}, []models.Host{agent})
		m.config = &config.Config{TemperatureMonitoringEnabled: true, PVEInstances: []config.PVEInstance{
			{Name: "siteA", Host: "https://10.0.0.5:8006", Fingerprint: "AA:AA"},
			{Name: "siteB", Host: "https://10.0.1.5:8006", Fingerprint: "BB:BB"},
		}}
		polled := readStateTestNode("siteB", "px1")
		polled.Host = "https://10.0.1.5:8006"
		polled.TLSFingerprint = pveNodeTLSFingerprint(&m.config.PVEInstances[1], "px1")
		m.collectNodeTemperatureData(context.Background(), "siteB", &m.config.PVEInstances[1], proxmox.Node{Node: "px1"}, &polled, nil, "online")
		if polled.Temperature != nil {
			t.Fatalf("siteB px1 took site A's agent reading: %#v", polled.Temperature)
		}
	})

	t.Run("sibling reading across the two connections of one cluster", func(t *testing.T) {
		// A cluster added twice can keep px1 under one connection's view and the
		// reporting px2 under the other's; both connections list the same
		// members with the same fingerprints, so the reading is px1's.
		var instances []config.PVEInstance
		for _, name := range []string{"enacon-a", "enacon-b"} {
			instances = append(instances, config.PVEInstance{
				Name: name, IsCluster: true, ClusterName: "enacon",
				ClusterEndpoints: []config.ClusterEndpoint{{NodeName: "px1", Fingerprint: "11:11"}, {NodeName: "px2", Fingerprint: "22:22"}},
			})
		}
		member := func(instanceCfg *config.PVEInstance, name string) models.Node {
			node := readStateTestNode(instanceCfg.Name, name)
			node.ClusterName, node.IsClusterMember = "enacon", true
			node.TLSFingerprint = pveNodeTLSFingerprint(instanceCfg, name)
			return node
		}
		m := readStateMonitor(
			[]models.Node{member(&instances[0], "px1"), member(&instances[1], "px2")},
			[]models.Host{reportingTestHost("agent-px2", "px2", "enacon-b-px2", 50)},
		)
		m.applyClusterSensors("agent-px2", clusterSensorReport("px1", 88), time.Now())
		m.config = &config.Config{TemperatureMonitoringEnabled: true, PVEInstances: instances}
		polled := member(&instances[0], "px1")
		m.collectNodeTemperatureData(context.Background(), "enacon-a", &instances[0], proxmox.Node{Node: "px1"}, &polled, nil, "online")
		if polled.Temperature == nil || polled.Temperature.CPUPackage != 88 {
			t.Fatalf("px1 temperature = %#v, want the sibling reading 88", polled.Temperature)
		}
	})

	t.Run("cluster sibling reading from the other site", func(t *testing.T) {
		m := readStateMonitor(sites(), []models.Host{reportingTestHost("agent-a2", "px2", "siteA-px2", 50)})
		m.applyClusterSensors("agent-a2", clusterSensorReport("px1", 88), time.Now())
		if temp := poll(m, "siteA", "px1"); temp == nil || temp.CPUPackage != 88 {
			t.Fatalf("siteA px1 temperature = %#v, want its cluster sibling's 88", temp)
		}
		if temp := poll(m, "siteB", "px1"); temp != nil {
			t.Fatalf("siteB px1 took site A's cluster sibling reading: %#v", temp)
		}
	})
}

// Exercise the actual scheduler result, admission and health entry points.
// No provider request, backup operation or database is involved in this control.
func TestMonitorPersistentFailureBackoffAndRecovery(t *testing.T) {
	m := &Monitor{
		config:          &config.Config{},
		circuitBreakers: make(map[string]*circuitBreaker),
		pollStatusMap:   make(map[string]*pollStatus),
		failureCounts:   make(map[string]int),
		lastOutcome:     make(map[string]taskOutcome),
	}
	failed := ScheduledTask{InstanceType: InstanceTypePVE, InstanceName: "offline"}
	healthy := ScheduledTask{InstanceType: InstanceTypePBS, InstanceName: "healthy"}
	for failure := 1; failure <= 130; failure++ {
		m.recordTaskResult(failed.InstanceType, failed.InstanceName, errors.New("connection timeout"))
		if failure < 3 {
			continue
		}
		cb := m.ensureBreaker(schedulerKey(failed.InstanceType, failed.InstanceName))
		state, count, retryAt := cb.State()
		if state != "open" || count != failure || !retryAt.After(time.Now()) || m.allowExecution(failed) {
			t.Fatalf("failure %d lost scheduler containment: state=%s count=%d retry=%v", failure, state, count, retryAt)
		}
		if !m.allowExecution(healthy) {
			t.Fatal("one failing connection blocked another instance")
		}
		health := m.SchedulerHealth()
		if len(health.Breakers) != 1 || health.Breakers[0].Failures != failure || !health.Breakers[0].RetryAt.Equal(retryAt) || health.Breakers[0].State != "open" {
			t.Fatalf("served scheduler health lost the owned retry fence: %+v", health.Breakers)
		}
		// Simulate reaching the scheduled retry without sleeping or polling a host.
		if !cb.allow(retryAt) {
			t.Fatal("due probe was not admitted")
		}
	}
	m.recordTaskResult(failed.InstanceType, failed.InstanceName, nil)
	if !m.allowExecution(failed) || len(m.SchedulerHealth().Breakers) != 0 {
		t.Fatal("successful poll did not restore scheduler admission and health")
	}
	status := m.pollStatusMap[schedulerKey(failed.InstanceType, failed.InstanceName)]
	if status.ConsecutiveFailures != 0 || !status.LastErrorAt.IsZero() || status.LastErrorMessage != "" || status.LastErrorCategory != "" || status.LastSuccess.IsZero() {
		t.Fatalf("successful poll retained an outstanding failure: %+v", status)
	}
	for i := 0; i < 3; i++ {
		m.recordTaskResult(failed.InstanceType, failed.InstanceName, errors.New("connection timeout"))
	}
	cb := m.ensureBreaker(schedulerKey(failed.InstanceType, failed.InstanceName))
	if cb.retryInterval != 40*time.Second || m.allowExecution(failed) {
		t.Fatal("new failure episode did not restart the original backoff")
	}
}

// A downstream read failure after a successful connection is a scheduler
// success that keeps the read error outstanding in one update (#2960).
func TestRecordTaskOutcomeReadErrorSchedulesAsSuccess(t *testing.T) {
	m := &Monitor{
		config:          &config.Config{},
		circuitBreakers: make(map[string]*circuitBreaker),
		pollStatusMap:   make(map[string]*pollStatus),
		failureCounts:   make(map[string]int),
		lastOutcome:     make(map[string]taskOutcome),
	}
	task := ScheduledTask{InstanceType: InstanceTypePMG, InstanceName: "gateway"}
	key := schedulerKey(task.InstanceType, task.InstanceName)
	for i := 0; i < 3; i++ {
		m.recordTaskResult(task.InstanceType, task.InstanceName, errors.New("connection timeout"))
	}
	if m.allowExecution(task) {
		t.Fatal("connection failures did not open the breaker")
	}

	m.recordTaskOutcome(task.InstanceType, task.InstanceName, nil, errors.New("pmg_queue_status: decode failed"))
	if outcome := m.lastOutcome[key]; !outcome.success || outcome.err != nil || m.failureCounts[key] != 0 {
		t.Fatalf("read error scheduled as a failure: outcome=%+v failures=%d", outcome, m.failureCounts[key])
	}
	if !m.allowExecution(task) || len(m.SchedulerHealth().Breakers) != 0 {
		t.Fatal("read error kept the breaker open")
	}
	status := m.pollStatusMap[key]
	if status.ConsecutiveFailures != 0 || status.LastSuccess.IsZero() || !status.FirstFailureAt.IsZero() ||
		status.LastErrorCategory != PollErrorCategoryPartial || !strings.Contains(status.LastErrorMessage, "pmg_queue_status") ||
		!status.LastErrorAt.Equal(status.LastSuccess) {
		t.Fatalf("read error not kept as the outstanding poll error: %+v", status)
	}

	m.recordTaskOutcome(task.InstanceType, task.InstanceName, nil, nil)
	if status.LastErrorMessage != "" || status.LastErrorCategory != "" || !status.LastErrorAt.IsZero() {
		t.Fatalf("complete poll left the read error outstanding: %+v", status)
	}
}

// monitorLockProbeError records whether the monitor lock was held while the
// error was rendered.
type monitorLockProbeError struct {
	m    *Monitor
	held *bool
}

func (e monitorLockProbeError) Error() string {
	if e.m.mu.TryLock() {
		e.m.mu.Unlock()
	} else {
		*e.held = true
	}
	return "probe failed"
}

// An error's text is built from a monitored system's replies and can be large.
// Rendering it must not hold the lock every poll and state read waits on.
func TestRecordTaskOutcomeRendersErrorsOutsideMonitorLock(t *testing.T) {
	for _, kind := range []string{"poll", "read"} {
		t.Run(kind, func(t *testing.T) {
			m := &Monitor{
				config:          &config.Config{},
				circuitBreakers: make(map[string]*circuitBreaker),
				pollStatusMap:   make(map[string]*pollStatus),
				failureCounts:   make(map[string]int),
				lastOutcome:     make(map[string]taskOutcome),
			}
			var held bool
			var pollErr, readErr error
			if kind == "poll" {
				pollErr = monitorLockProbeError{m: m, held: &held}
			} else {
				readErr = monitorLockProbeError{m: m, held: &held}
			}
			m.recordTaskOutcome(InstanceTypePMG, "gateway", pollErr, readErr)
			if held {
				t.Fatalf("%s error rendered while holding the monitor lock", kind)
			}
			if status := m.pollStatusMap[schedulerKey(InstanceTypePMG, "gateway")]; status.LastErrorMessage != "probe failed" {
				t.Fatalf("%s error not recorded: %+v", kind, status)
			}
		})
	}
}

func TestGuestConfigPlacement(t *testing.T) {
	for _, kind := range []string{"vm", "container", "lxc"} {
		for _, reverse := range []bool{false, true} {
			for _, tc := range []struct {
				name, instance, node, wantInstance, wantNode, failure string
			}{
				{name: "instance constraint", instance: "second", wantInstance: "second", wantNode: "node-b"},
				{name: "node constraint", node: "node-b", wantInstance: "second", wantNode: "node-b"},
				{name: "bare VMID ambiguous", failure: "ambiguous"},
				{name: "unknown instance", instance: "absent", failure: "unable to resolve"},
				{name: "unknown node", node: "absent", failure: "unable to resolve"},
			} {
				t.Run(kind+"/"+tc.name+"/reverse="+map[bool]string{true: "yes", false: "no"}[reverse], func(t *testing.T) {
					state := models.NewState()
					vms := []models.VM{{ID: "a", VMID: 105, Instance: "first", Node: "node-a"}, {ID: "b", VMID: 105, Instance: "second", Node: "node-b"}}
					if reverse {
						vms[0], vms[1] = vms[1], vms[0]
					}
					if kind == "vm" {
						state.VMs = vms
					} else {
						for _, vm := range vms {
							state.Containers = append(state.Containers, models.Container{ID: vm.ID, VMID: vm.VMID, Instance: vm.Instance, Node: vm.Node})
						}
					}
					first := &guestConfigCoverageClient{vmConfig: map[string]interface{}{"origin": "first"}, containerConfig: map[string]interface{}{"origin": "first"}}
					second := &guestConfigCoverageClient{vmConfig: map[string]interface{}{"origin": "second"}, containerConfig: map[string]interface{}{"origin": "second"}}
					m := &Monitor{state: state, pveClients: map[string]PVEClientInterface{"first": first, "second": second}}
					got, err := m.GetGuestConfig(context.Background(), kind, tc.instance, tc.node, 105)
					if tc.failure != "" {
						if err == nil || !strings.Contains(err.Error(), tc.failure) || got != nil || first.vmCalls+first.containerCalls+second.vmCalls+second.containerCalls != 0 {
							t.Fatalf("unresolved identity reached a provider: result=%v err=%v", got, err)
						}
						return
					}
					if err != nil || got["origin"] != tc.wantInstance || second.lastNode != tc.wantNode || second.lastVMID != 105 || second.vmCalls+second.containerCalls != 1 || first.vmCalls+first.containerCalls != 0 {
						t.Fatalf("placement constraint was not honoured: result=%v err=%v target=%s/%d", got, err, second.lastNode, second.lastVMID)
					}
				})
			}
		}
	}
}

func TestGuestConfigPlacementCurrentInventory(t *testing.T) {
	for _, kind := range []string{"vm", "container"} {
		for _, empty := range []bool{false, true} {
			t.Run(kind+"/empty="+map[bool]string{true: "yes", false: "no"}[empty], func(t *testing.T) {
				legacy := models.NewState()
				legacy.VMs = []models.VM{{ID: "stale-vm", VMID: 105, Instance: "old", Node: "old-node"}}
				legacy.Containers = []models.Container{{ID: "stale-ct", VMID: 105, Instance: "old", Node: "old-node"}}
				registry := unifiedresources.NewRegistry(nil)
				if !empty {
					registry.IngestSnapshot(models.StateSnapshot{
						VMs:        []models.VM{{ID: "current-vm", VMID: 105, Instance: "current", Node: "current-node"}},
						Containers: []models.Container{{ID: "current-ct", VMID: 105, Instance: "current", Node: "current-node"}},
					})
				}
				old, current := &guestConfigCoverageClient{}, &guestConfigCoverageClient{}
				m := &Monitor{state: legacy, resourceStore: unifiedresources.NewMonitorAdapter(registry), pveClients: map[string]PVEClientInterface{"old": old, "current": current}}
				_, err := m.GetGuestConfig(context.Background(), kind, "", "", 105)
				if empty {
					if err == nil || current.vmCalls+current.containerCalls != 0 {
						t.Fatal("empty current inventory fell back to stale placement")
					}
				} else if err != nil || current.lastNode != "current-node" || current.vmCalls+current.containerCalls != 1 {
					t.Fatalf("current inventory did not own placement: %v", err)
				}
				if old.vmCalls+old.containerCalls != 0 {
					t.Fatal("stale connection was read")
				}
			})
		}
	}
}

func TestGuestConfigPlacementAdverseAndDirect(t *testing.T) {
	for _, kind := range []string{"vm", "container"} {
		t.Run(kind, func(t *testing.T) {
			failure := errors.New("provider read failed")
			client := &guestConfigCoverageClient{vmErr: failure, containerErr: failure}
			m := &Monitor{pveClients: map[string]PVEClientInterface{"chosen": client}}
			_, err := m.GetGuestConfig(context.Background(), kind, "chosen", "chosen-node", 105)
			if !errors.Is(err, failure) || client.lastNode != "chosen-node" || client.vmCalls+client.containerCalls != 1 {
				t.Fatal("explicit placement or original provider failure was changed")
			}
			state := models.NewState()
			state.VMs = []models.VM{{ID: "one", VMID: 105, Instance: "chosen", Node: "same-node"}, {ID: "two", VMID: 105, Instance: "other", Node: "same-node"}}
			state.Containers = []models.Container{{ID: "one", VMID: 105, Instance: "chosen", Node: "same-node"}, {ID: "two", VMID: 105, Instance: "other", Node: "same-node"}}
			m.state = state
			_, err = m.GetGuestConfig(context.Background(), kind, "", "same-node", 105)
			if err == nil || !strings.Contains(err.Error(), "ambiguous") || client.vmCalls+client.containerCalls != 1 {
				t.Fatal("two installations sharing a node name were guessed")
			}
			state.VMs, state.Containers = []models.VM{{VMID: 105, Instance: "chosen"}}, []models.Container{{VMID: 105, Instance: "chosen"}}
			_, err = m.GetGuestConfig(context.Background(), kind, "chosen", "", 105)
			if err == nil || client.vmCalls+client.containerCalls != 1 {
				t.Fatal("incomplete placement reached a provider")
			}
		})
	}
}

type concurrentGuestConfigClient struct {
	stubPVEClient
	calls atomic.Int32
}

func (c *concurrentGuestConfigClient) GetVMConfig(_ context.Context, node string, vmid int) (map[string]interface{}, error) {
	c.calls.Add(1)
	return map[string]interface{}{"node": node, "vmid": vmid}, nil
}

func TestGuestConfigPlacementConcurrentSnapshot(t *testing.T) {
	state := models.NewState()
	state.UpdateVMsForInstance("chosen", []models.VM{{ID: "vm", VMID: 105, Instance: "chosen", Node: "a"}})
	client := &concurrentGuestConfigClient{}
	m := &Monitor{state: state, pveClients: map[string]PVEClientInterface{"chosen": client}}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			state.UpdateVMsForInstance("chosen", []models.VM{{ID: "vm", VMID: 105, Instance: "chosen", Node: "b"}})
			state.UpdateVMsForInstance("chosen", []models.VM{{ID: "vm", VMID: 105, Instance: "chosen", Node: "a"}})
		}
	}()
	for i := 0; i < 100; i++ {
		got, err := m.GetGuestConfig(context.Background(), "vm", "chosen", "", 105)
		if err != nil || got["vmid"] != 105 || got["node"] != "a" && got["node"] != "b" {
			t.Errorf("invalid concurrent snapshot: %v %v", got, err)
		}
	}
	wg.Wait()
	if client.calls.Load() != 100 {
		t.Fatal("concurrent reads were replayed or lost")
	}
}

// Slow reporters share one temperature lease through admission and collection.
func temperatureCadenceFixture(t *testing.T, interval int, age time.Duration, cluster bool) (*Monitor, models.Node, *mockCommandRunner, time.Time) {
	t.Helper()
	now := time.Now()
	reportTime := now.Add(-age)
	nodes := []models.Node{readStateTestNode("siteA", "px1"), readStateTestNode("siteA", "px2"), readStateTestNode("siteB", "px1")}
	target := nodes[0]
	target.Type = "node"
	target.Host = "px1.invalid"
	target.PVEVersion = "pve-manager/9.0.1"
	target.Memory = models.Memory{Total: 8 << 30, Used: 2 << 30, Free: 6 << 30, Usage: 25}
	reporterNode, cpu := "siteA-px1", 62.0
	if cluster {
		reporterNode, cpu = "siteA-px2", 0
	}
	reporter := reportingTestHost("agent-reporter", "reporter", reporterNode, cpu)
	reporter.IntervalSeconds, reporter.LastSeen = interval, reportTime
	m := readStateMonitor(nodes, []models.Host{reporter})
	m.config = &config.Config{TemperatureMonitoringEnabled: true}
	if cluster {
		m.applyClusterSensors(reporter.ID, clusterSensorReport("px1", 62), reportTime)
	}
	key := t.TempDir() + "/fixture-key"
	if err := os.WriteFile(key, []byte("fixture-only, never used for authentication"), 0600); err != nil {
		t.Fatal(err)
	}
	runner := &mockCommandRunner{outputs: map[string]string{"sensors -j": `{"coretemp-isa-0000":{"Package id 0":{"temp1_input":45}}}`}}
	m.tempCollector = &TemperatureCollector{sshKeyPath: key, sshUser: "pulse-monitor", sshPort: 22, runner: runner}
	return m, target, runner, reportTime
}

func collectTemperatureCadenceFixture(m *Monitor, target models.Node) models.Node {
	m.collectNodeTemperatureData(context.Background(), target.Instance, &config.PVEInstance{Name: target.Instance}, proxmox.Node{Node: target.Name}, &target, nil, "online")
	return target
}

func TestNodeTemperatureUsesReporterCadence(t *testing.T) {
	for _, cluster := range []bool{false, true} {
		source := "direct"
		if cluster {
			source = "sibling"
		}
		for _, tc := range []struct {
			name      string
			interval  int
			age       time.Duration
			wantAgent bool
		}{
			{"default-fresh", 30, time.Minute, true},
			{"120-second-cadence", 120, 140 * time.Second, true},
			{"180-second-cadence", 180, 160 * time.Second, true},
			{"long-cadence", 300, 6 * time.Minute, true},
			{"short-cadence-expired", 30, 4 * time.Minute, false},
			{"capped-cadence-expired", 3600, 11 * time.Minute, false},
		} {
			t.Run(source+"/"+tc.name, func(t *testing.T) {
				m, target, runner, reportTime := temperatureCadenceFixture(t, tc.interval, tc.age, cluster)
				got := collectTemperatureCadenceFixture(m, target)
				if got.Temperature == nil || !got.Temperature.Available {
					t.Fatalf("no usable result: %#v", got.Temperature)
				}
				if tc.wantAgent {
					if runner.callCount != 0 {
						t.Errorf("valid report triggered %d unnecessary SSH commands", runner.callCount)
					}
					if got.Temperature.CPUPackage != 62 || !got.Temperature.LastUpdate.Equal(reportTime) {
						t.Errorf("report changed or restamped: %#v, report %s", got.Temperature, reportTime)
					}
				} else {
					if runner.callCount != 1 || got.Temperature.CPUPackage != 45 {
						t.Errorf("expired report must allow one ordinary fallback: calls=%d temp=%#v", runner.callCount, got.Temperature)
					}
				}
			})
		}
	}
}

func TestClusterTemperatureDoesNotRenewOldReadingWithFreshReporter(t *testing.T) {
	m, target, _, _ := temperatureCadenceFixture(t, 30, time.Minute, true)
	for key, entry := range m.clusterSensorsCache {
		entry.updatedAt = time.Now().Add(-4 * time.Minute)
		m.clusterSensorsCache[key] = entry
	}
	if got := m.getHostAgentTemperatureForNode(target); got != nil {
		t.Fatalf("fresh reporter renewed stale sensor reading: %#v", got)
	}
}

func TestClusterTemperatureDoesNotBorrowMergedNodeLiveness(t *testing.T) {
	m, target, _, _ := temperatureCadenceFixture(t, 30, 4*time.Minute, true)
	// The PVE view of px2 is current, but the reporting agent is not.
	m.applyClusterSensors("agent-reporter", clusterSensorReport("px1", 95), time.Now())
	if got := m.getHostAgentTemperatureForNode(target); got != nil {
		t.Fatalf("stale reporter borrowed its live PVE node: %#v", got)
	}
}

func TestClusterTemperatureCadenceKeepsConnectionAndReporterIdentity(t *testing.T) {
	m, target, _, _ := temperatureCadenceFixture(t, 300, 6*time.Minute, true)
	if got := m.getHostAgentTemperatureForNode(target); got == nil || got.CPUPackage != 62 {
		t.Fatalf("valid slow sibling lost: %#v", got)
	}
	if got := m.getHostAgentTemperatureForNode(readStateTestNode("siteB", "px1")); got != nil {
		t.Fatalf("namesake site borrowed reading: %#v", got)
	}
	m.applyClusterSensors("missing-agent", clusterSensorReport("px1", 99), time.Now())
	if got := m.getHostAgentTemperatureForNode(target); got == nil || got.CPUPackage != 62 {
		t.Fatalf("newer orphan displaced valid reporter: %#v", got)
	}
	// An unrelated report must not evict the still-live six-minute sibling row.
	m.applyClusterSensors("unrelated-agent", nil, time.Now())
	if got := m.getHostAgentTemperatureForNode(target); got == nil || got.CPUPackage != 62 {
		t.Fatalf("unrelated report evicted valid slow sibling: %#v", got)
	}
}

type pollWorkFixturePVE struct {
	stubPVEClient
	getNodes func(context.Context) ([]proxmox.Node, error)
}

func (c *pollWorkFixturePVE) GetNodes(ctx context.Context) ([]proxmox.Node, error) {
	return c.getNodes(ctx)
}

func TestSchedulerHealthDistinguishesOverdueQueuedPolls(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		m := &Monitor{taskQueue: NewTaskQueue(), deadLetterQueue: NewTaskQueue()}
		start := time.Now()
		first := ScheduledTask{InstanceType: InstanceTypePVE, InstanceName: "first", NextRun: start.Add(10 * time.Second)}
		m.taskQueue.upsertPlan(first, 0)
		m.taskQueue.upsertPlan(ScheduledTask{InstanceType: InstanceTypePBS, InstanceName: "future", NextRun: start.Add(5 * time.Minute)}, 0)
		m.deadLetterQueue.Upsert(ScheduledTask{InstanceType: InstanceTypePVE, InstanceName: "parked", NextRun: start.Add(-time.Hour)})

		check := func(phase string, depth, dueSoon int, oldest float64) {
			t.Helper()
			// Assert the wire shape as well as the meaning: a measured zero
			// must not be omitted or look like an older, unmeasured response.
			snapshot := m.SchedulerHealth()
			wire, err := json.Marshal(snapshot.Queue)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("%s: %s", phase, wire)
			var queue struct {
				Depth                int      `json:"depth"`
				DueWithinSeconds     int      `json:"dueWithinSeconds"`
				OldestOverdueSeconds *float64 `json:"oldestOverdueSeconds"`
			}
			if err := json.Unmarshal(wire, &queue); err != nil {
				t.Fatal(err)
			}
			if queue.Depth != depth || queue.DueWithinSeconds != dueSoon {
				t.Errorf("%s: queue depth/due soon = %d/%d, want %d/%d", phase, queue.Depth, queue.DueWithinSeconds, depth, dueSoon)
			}
			if queue.OldestOverdueSeconds == nil || *queue.OldestOverdueSeconds != oldest {
				t.Errorf("%s: want oldest overdue %g seconds in %s", phase, oldest, wire)
			}
		}

		check("due soon, not late", 2, 1, 0)
		time.Sleep(100 * time.Second)
		check("unclaimed, same counts but 90 seconds late", 2, 1, 90)

		newer := ScheduledTask{InstanceType: InstanceTypePBS, InstanceName: "newer", NextRun: time.Now().Add(-2 * time.Second)}
		m.taskQueue.upsertPlan(newer, 0)
		check("two overdue, use the oldest", 3, 2, 90)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		claimed, ok := m.taskQueue.WaitNext(ctx)
		if !ok || claimed != first {
			t.Fatalf("expected the oldest scheduled task, got %+v (claimed = %v)", claimed, ok)
		}
		check("older task has been claimed", 2, 1, 2)

		retry := first
		retry.NextRun = time.Now().Add(5 * time.Second)
		m.taskQueue.completeClaim(claimed, retry, pollAfterBackoff)
		m.taskQueue.Remove(newer.InstanceType, newer.InstanceName)
		check("only future work including the retry", 2, 1, 0)
		time.Sleep(5 * time.Second)
		check("exactly at the retry deadline", 2, 1, 0)
		time.Sleep(500 * time.Millisecond)
		m.taskQueue.Upsert(ScheduledTask{InstanceType: InstanceTypePBS, InstanceName: "immediate-without-timestamp"})
		check("untimestamped work cannot inflate the retry's measured age", 3, 2, 0.5)
	})
}

func TestSchedulerQueuedPollOverdueFollowsActiveQueues(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		registry := prometheus.NewPedanticRegistry()
		metrics := newPollMetrics(registry)
		left := &Monitor{pollMetrics: metrics, taskQueue: NewTaskQueue()}
		right := &Monitor{pollMetrics: metrics, taskQueue: NewTaskQueue(), deadLetterQueue: NewTaskQueue()}
		origin := time.Now()
		left.taskQueue.upsertPlan(ScheduledTask{InstanceType: InstanceTypePBS, InstanceName: "store", NextRun: origin.Add(8 * time.Second)}, 0)
		right.taskQueue.upsertPlan(ScheduledTask{InstanceType: InstanceTypePBS, InstanceName: "store", NextRun: origin.Add(10 * time.Second)}, 0)
		right.deadLetterQueue.Upsert(ScheduledTask{InstanceType: InstanceTypePBS, InstanceName: "parked", NextRun: origin.Add(-time.Hour)})

		handler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
		scrape := func(phase, age, dueSoon string) {
			t.Helper()
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("%s: HTTP %d: %s", phase, rec.Code, rec.Body)
			}
			body := rec.Body.String()
			for _, sample := range []struct{ name, value string }{
				{"pulse_scheduler_queued_poll_overdue_seconds", age},
				{"pulse_scheduler_queue_due_soon", dueSoon},
			} {
				got := "<absent>"
				for line := range strings.SplitSeq(body, "\n") {
					if value, ok := strings.CutPrefix(line, sample.name+" "); ok {
						got = value
						break
					}
				}
				t.Logf("%s: %s %s", phase, sample.name, got)
				if got != sample.value {
					t.Errorf("%s: %s = %s, want %s", phase, sample.name, got, sample.value)
				}
			}
		}
		scrape("before workers start", "0", "0")

		limiter := newPollTaskWorkerLimiter(1)
		if !limiter.acquire(context.Background()) {
			t.Fatal("could not reserve the only worker slot")
		}
		defer limiter.release()
		start := func(m *Monitor) func() {
			// The dispatcher takes this claim, then blocks on the only slot.
			// Its other work will stay in the heap without any result writes.
			m.taskQueue.Upsert(ScheduledTask{InstanceType: InstanceTypePVE, InstanceName: "held", NextRun: origin.Add(-time.Minute)})
			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan struct{})
			go func() {
				defer close(done)
				m.dispatchTaskWorkers(ctx, limiter)
			}()
			return func() {
				cancel()
				<-done
			}
		}
		stopLeft, stopRight := start(left), start(right)
		defer stopLeft()
		defer stopRight()
		synctest.Wait()
		left.taskQueue.Upsert(ScheduledTask{InstanceType: InstanceTypePMG, InstanceName: "no scheduled time"})

		scrape("due soon but not late", "0", "3")
		time.Sleep(8 * time.Second)
		scrape("exactly due", "0", "3")
		time.Sleep(7500 * time.Millisecond)
		scrape("same count, use the oldest across both queues", "7.5", "3")

		stopLeft()
		if got := left.taskQueue.Snapshot().OldestOverdueSeconds; got != 7.5 {
			t.Fatalf("retired queue should still retain the older work, got age %v", got)
		}
		scrape("older queue stopped", "5.5", "1")
		claimCtx, cancelClaim := context.WithTimeout(context.Background(), time.Second)
		claimed, ok := right.taskQueue.WaitNext(claimCtx)
		cancelClaim()
		if !ok || claimed.InstanceType != InstanceTypePBS || claimed.InstanceName != "store" {
			t.Fatalf("fixture could not claim the remaining overdue poll: %+v (%v)", claimed, ok)
		}
		scrape("remaining overdue poll claimed", "0", "0")
		retry := claimed
		retry.NextRun = time.Now().Add(5 * time.Second)
		right.taskQueue.completeClaim(claimed, retry, pollAfterBackoff)
		scrape("retry queued for the future", "0", "1")
		time.Sleep(5500 * time.Millisecond)
		scrape("retry overdue, still unclaimed", "0.5", "1")

		stopRight()
		if got := right.taskQueue.Snapshot().OldestOverdueSeconds; got != 0.5 {
			t.Fatalf("last queue should still retain the retry, got age %v", got)
		}
		scrape("last queue stopped", "0", "0")
	})
}

func TestSchedulerQueueScrapeRetiresCanceledSchedulerWithBlockedWorker(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		reg := prometheus.NewPedanticRegistry()
		pm := newPollMetrics(reg)
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		peerCtx, cancelPeer := context.WithCancel(t.Context())
		defer cancelPeer()

		entered := make(chan struct{})
		release := make(chan struct{})
		defer func() {
			if release != nil {
				close(release)
			}
		}()
		gate := release
		runtime := &Monitor{
			pollMetrics: pm, taskQueue: NewTaskQueue(), pollTimeout: 5 * time.Second,
			pollProviders: map[InstanceType]PollProvider{
				InstanceTypePVE: testPollProvider{
					providerType: InstanceTypePVE,
					buildPollTask: func(string) (PollTask, error) {
						return PollTask{Run: func(context.Context) {
							close(entered)
							<-gate // The read ignores cancellation until it returns.
						}}, nil
					},
				},
			},
		}
		runtime.executor = newRealExecutor(runtime)
		peer := &Monitor{pollMetrics: pm, taskQueue: NewTaskQueue()}
		now := time.Now()
		runtime.taskQueue.Upsert(ScheduledTask{InstanceType: InstanceTypePVE, InstanceName: "running", NextRun: now})
		runtime.taskQueue.Upsert(ScheduledTask{InstanceType: InstanceTypePBS, InstanceName: "behind-running", NextRun: now.Add(time.Second)})
		peer.taskQueue.Upsert(ScheduledTask{InstanceType: InstanceTypePMG, InstanceName: "peer", NextRun: now.Add(time.Minute)})

		stopped := make(chan struct{})
		go func() {
			defer close(stopped)
			runtime.taskWorker(ctx, 0)
		}()
		go peer.taskWorker(peerCtx, 0)
		<-entered
		synctest.Wait()

		handler := promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
		scrape := func(phase string, want ...string) {
			t.Helper()
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("%s: scrape returned HTTP %d", phase, rec.Code)
			}
			for _, line := range want {
				if !strings.Contains(rec.Body.String(), "\n"+line+"\n") {
					t.Errorf("%s: missing exact sample %q", phase, line)
				} else {
					t.Logf("%s: %s", phase, line)
				}
			}
		}
		time.Sleep(8 * time.Second)
		scrape("before cancellation",
			"pulse_monitor_poll_queue_depth 2", "pulse_scheduler_queue_due_soon 1",
			"pulse_scheduler_queued_poll_overdue_seconds 7",
			`pulse_scheduler_queue_depth{instance_type="pbs"} 1`,
			`pulse_scheduler_queue_depth{instance_type="pmg"} 1`,
			`pulse_scheduler_claimed_poll_overdue_seconds{stage="running"} 3`)

		cancel()
		synctest.Wait()
		select {
		case <-stopped:
			t.Fatal("fixture worker returned before the provider was released")
		default:
		}
		if got := runtime.taskQueue.Size(); got != 1 {
			t.Fatalf("fixture must still hold the abandoned queued poll; depth = %d", got)
		}
		scrape("cancelled, worker still blocked",
			"pulse_monitor_poll_queue_depth 1", "pulse_scheduler_queue_due_soon 0",
			"pulse_scheduler_queued_poll_overdue_seconds 0",
			`pulse_scheduler_queue_depth{instance_type="pbs"} 0`,
			`pulse_scheduler_queue_depth{instance_type="pmg"} 1`,
			`pulse_scheduler_claimed_poll_overdue_seconds{stage="running"} 3`)

		close(release)
		release = nil
		<-stopped
		scrape("cancelled worker returned",
			"pulse_monitor_poll_queue_depth 1",
			`pulse_scheduler_claimed_poll_overdue_seconds{stage="running"} 0`)
		cancelPeer()
		synctest.Wait()
		scrape("all stopped",
			"pulse_monitor_poll_queue_depth 0",
			`pulse_scheduler_queue_depth{instance_type="pmg"} 0`)
	})
}

func TestScheduledPollWorkProgressInExporter(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const (
			waiting      = "fixture-held"
			sibling      = "fixture-sibling"
			down         = "fixture-down"
			longInterval = 10 * time.Minute
		)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		gate := make(chan struct{})
		released := false
		defer func() {
			if !released {
				close(gate)
			}
		}()
		origin := time.Now()
		var starts []time.Duration
		physicalDiskMonitoring := false
		okRead := func(context.Context) ([]proxmox.Node, error) { return []proxmox.Node{}, nil }
		registry := prometheus.NewRegistry()
		m := &Monitor{
			config: &config.Config{PVEInstances: []config.PVEInstance{
				{Name: waiting, MonitorPhysicalDisks: &physicalDiskMonitoring},
				{Name: sibling, MonitorPhysicalDisks: &physicalDiskMonitoring},
				{Name: down, MonitorPhysicalDisks: &physicalDiskMonitoring},
			}},
			state: models.NewState(), alertManager: &alerts.Manager{}, runtimeCtx: ctx,
			taskQueue: NewTaskQueue(), pollMetrics: newPollMetrics(registry), pollTimeout: defaultTaskTimeout,
			pollStatusMap: make(map[string]*pollStatus), lastOutcome: make(map[string]taskOutcome),
			failureCounts: make(map[string]int), circuitBreakers: make(map[string]*circuitBreaker),
			maxRetryAttempts: 5, backoffCfg: backoffConfig{Initial: 5 * time.Second},
			lastClusterCheck: map[string]time.Time{waiting: origin, sibling: origin, down: origin},
			pveClients: map[string]PVEClientInterface{
				waiting: &pollWorkFixturePVE{getNodes: func(ctx context.Context) ([]proxmox.Node, error) {
					starts = append(starts, time.Since(origin))
					if len(starts) == 1 {
						<-gate
						return nil, ctx.Err()
					}
					return okRead(ctx)
				}},
				sibling: &pollWorkFixturePVE{getNodes: okRead},
				down: &pollWorkFixturePVE{getNodes: func(context.Context) ([]proxmox.Node, error) {
					return nil, errors.New("fixture target unavailable")
				}},
			},
		}
		m.executor = newRealExecutor(m)
		limiter := newPollTaskWorkerLimiter(2)
		if !limiter.acquire(ctx) || !limiter.acquire(ctx) {
			t.Fatal("could not reserve fixture slots")
		}
		handler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
		queue := func(name string, due time.Time) {
			m.taskQueue.Upsert(ScheduledTask{InstanceType: InstanceTypePVE, InstanceName: name, NextRun: due, Interval: longInterval})
		}
		observe := func(label string) string {
			t.Helper()
			synctest.Wait()
			// Refresh the legacy heap-derived gauge; stale publication is not needed for the gap.
			m.updateQueueDepthMetric()
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("scrape status %d", rec.Code)
			}
			body := rec.Body.String()
			t.Logf("%s at %s: heap=%+v provider starts=%v", label, time.Since(origin), m.taskQueue.Snapshot(), starts)
			for _, line := range strings.Split(body, "\n") {
				if strings.HasPrefix(line, "pulse_scheduler_claimed_poll_overdue_seconds{") ||
					strings.HasPrefix(line, "pulse_scheduler_queue_due_soon ") ||
					strings.HasPrefix(line, "pulse_monitor_poll_inflight{instance_type=\"pve\"}") ||
					strings.HasPrefix(line, "pulse_scheduler_queue_wait_seconds_count{instance_type=\"pve\"}") ||
					(strings.HasPrefix(line, "pulse_monitor_poll_total{") && strings.Contains(line, "instance=\"fixture-")) {
					t.Log(line)
				}
			}
			return body
		}
		want := func(scrape string, lines ...string) {
			t.Helper()
			for _, line := range lines {
				if !strings.Contains(scrape, "\n"+line+"\n") {
					t.Errorf("missing sample %q", line)
				}
			}
		}
		wantOverdue := func(scrape, waiting, running string) {
			t.Helper()
			want(scrape,
				`pulse_scheduler_claimed_poll_overdue_seconds{stage="waiting_for_slot"} `+waiting,
				`pulse_scheduler_claimed_poll_overdue_seconds{stage="running"} `+running,
			)
		}
		go m.dispatchTaskWorkers(ctx, limiter)
		idle := observe("idle")
		wantOverdue(idle, "0", "0")
		want(idle, "pulse_scheduler_queue_due_soon 0")
		queue(waiting, origin)
		t.Log("fixture-held due at 0s")
		time.Sleep(defaultTaskTimeout + 5*time.Second)
		held := observe("waiting for process slot")
		wantOverdue(held, "5", "0")
		want(held, "pulse_scheduler_queue_due_soon 0")
		if m.taskQueue.Size() != 0 || len(starts) != 0 ||
			strings.Contains(held, "pulse_scheduler_queue_wait_seconds_count{instance_type=\"pve\"}") ||
			strings.Contains(held, "pulse_monitor_poll_inflight{instance_type=\"pve\"}") {
			t.Fatal("slot-waiting task should be absent from the heap, admission histogram and provider gauge")
		}
		limiter.release()
		admitted := observe("provider entered")
		wantOverdue(admitted, "0", "0")
		want(admitted, "pulse_monitor_poll_inflight{instance_type=\"pve\"} 1")
		if len(starts) != 1 || starts[0] != defaultTaskTimeout+5*time.Second {
			t.Fatalf("provider starts = %v", starts)
		}
		// Queue both controls in advance so they become eligible together.
		controlsDue := time.Now().Add(defaultTaskTimeout + 5*time.Second)
		queue(sibling, controlsDue)
		queue(down, controlsDue)
		limiter.release()
		time.Sleep(time.Until(controlsDue))
		blocked := observe("provider over budget; sibling success and down target returned")
		wantOverdue(blocked, "0", "5")
		want(blocked,
			"pulse_scheduler_queue_due_soon 1",
			"pulse_monitor_poll_inflight{instance_type=\"pve\"} 1",
			"pulse_monitor_poll_total{instance=\"fixture-sibling\",instance_type=\"pve\",result=\"success\"} 1",
			"pulse_monitor_poll_total{instance=\"fixture-down\",instance_type=\"pve\",result=\"error\"} 1",
		)
		if strings.Contains(blocked, "pulse_monitor_poll_total{instance=\"fixture-held\",") {
			t.Fatal("still-blocked provider reported a terminal result")
		}
		if retry, ok := m.taskQueue.Get(InstanceTypePVE, down); !ok || retry.NextRun.Sub(time.Now()) != 5*time.Second {
			t.Fatalf("returned failure did not enter 5s backoff: %+v present=%v", retry, ok)
		}
		close(gate)
		released = true
		returned := observe("gate released; expired read recorded and backed off")
		wantOverdue(returned, "0", "0")
		want(returned,
			"pulse_monitor_poll_inflight{instance_type=\"pve\"} 0",
			"pulse_monitor_poll_total{instance=\"fixture-held\",instance_type=\"pve\",result=\"error\"} 1",
		)
		if retry, ok := m.taskQueue.Get(InstanceTypePVE, waiting); !ok || retry.NextRun.Sub(time.Now()) != 5*time.Second {
			t.Fatalf("released read did not enter 5s backoff: %+v present=%v", retry, ok)
		}
		time.Sleep(5 * time.Second)
		recovered := observe("same held target next ordinary read completed")
		wantOverdue(recovered, "0", "0")
		want(recovered, "pulse_monitor_poll_total{instance=\"fixture-held\",instance_type=\"pve\",result=\"success\"} 1")
		if len(starts) != 2 || starts[1] != 2*defaultTaskTimeout+15*time.Second {
			t.Fatalf("provider starts = %v", starts)
		}
	})
}

func TestScheduledPollProgressWhileMonitorIsLocked(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		reg := prometheus.NewRegistry()
		release := make(chan struct{})
		defer func() {
			if release != nil {
				close(release)
			}
		}()

		entered := make(chan struct{})
		gate := release
		m := &Monitor{
			taskQueue:   NewTaskQueue(),
			pollMetrics: newPollMetrics(reg),
			pollProviders: map[InstanceType]PollProvider{
				InstanceTypePVE: pollProviderAdapter{
					instanceType: InstanceTypePVE,
					buildPollTask: func(m *Monitor, _ string) (PollTask, error) {
						m.mu.Lock()
						defer m.mu.Unlock()
						close(entered)
						<-gate
						return PollTask{}, errors.New("fixture client no longer configured")
					},
				},
			},
		}
		m.executor = newRealExecutor(m)
		m.taskQueue.Upsert(ScheduledTask{
			InstanceType: InstanceTypePVE, InstanceName: "fixture-preparing", NextRun: time.Now(), Interval: time.Hour,
		})
		go m.taskWorker(ctx, 0)
		<-entered
		time.Sleep(defaultTaskTimeout + time.Second)
		synctest.Wait()
		// The normal worker has not entered the provider and the monitor lock is
		// deliberately unavailable. The actual exporter must still answer.
		assertClaimedPollOverdue(t, reg, 0, 1)
		close(release)
		release = nil
		synctest.Wait()
		assertClaimedPollOverdue(t, reg, 0, 0)
	})
}

func TestScheduledPollProgressClearsOnSlotCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		reg := prometheus.NewRegistry()
		m := &Monitor{taskQueue: NewTaskQueue(), pollMetrics: newPollMetrics(reg)}
		task := ScheduledTask{InstanceType: InstanceTypePVE, InstanceName: "fixture-cancel", NextRun: time.Now()}
		m.taskQueue.Upsert(task)
		limiter := newPollTaskWorkerLimiter(1)
		if !limiter.acquire(ctx) {
			t.Fatal("could not reserve fixture slot")
		}
		defer limiter.release()
		done := make(chan struct{})
		go func() {
			defer close(done)
			m.dispatchTaskWorkers(ctx, limiter)
		}()
		time.Sleep(defaultTaskTimeout + time.Second)
		synctest.Wait()
		assertClaimedPollOverdue(t, reg, 1, 0)
		m.taskQueue.upsertPlan(task, 0)
		if m.taskQueue.Size() != 0 {
			t.Fatal("a poll awaiting a process slot must keep owning the target")
		}
		cancel()
		<-done
		assertClaimedPollOverdue(t, reg, 0, 0)
		m.taskQueue.upsertPlan(task, 0)
		if m.taskQueue.Size() != 1 {
			t.Fatal("slot cancellation did not release the claim")
		}
	})
}

func TestScheduledPollDoesNotTakeSiblingSlot(t *testing.T) {
	for _, tc := range []struct {
		name             string
		adaptive, shared bool
	}{
		{name: "fixed-local"},
		{name: "fixed-process", shared: true},
		{name: "adaptive-local", adaptive: true},
		{name: "adaptive-process", adaptive: true, shared: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				const (
					slow     = "fixture-slow"
					peer     = "fixture-peer"
					cadence  = 10 * time.Second
					budget   = 30 * time.Second
					backoff  = 5 * time.Second
					fixtTick = 250 * time.Millisecond // Observe after the queue's empty timer.
				)
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				release := make(chan struct{})
				unblock := sync.OnceFunc(func() { close(release) })
				defer unblock()
				start := time.Now()
				var slowStarts atomic.Int32
				physicalDisks := false
				m := &Monitor{
					config: &config.Config{PVEPollingInterval: cadence, PVEInstances: []config.PVEInstance{
						{Name: slow, MonitorPhysicalDisks: &physicalDisks},
						{Name: peer, MonitorPhysicalDisks: &physicalDisks},
					}},
					state: models.NewState(), alertManager: &alerts.Manager{}, runtimeCtx: ctx,
					metricsHistory: NewMetricsHistory(16, time.Hour),
					taskQueue:      NewTaskQueue(), pollTimeout: budget, startTime: start,
					pollStatusMap: make(map[string]*pollStatus), lastOutcome: make(map[string]taskOutcome),
					failureCounts: make(map[string]int), circuitBreakers: make(map[string]*circuitBreaker),
					maxRetryAttempts: 5, backoffCfg: backoffConfig{Initial: backoff},
					lastClusterCheck: map[string]time.Time{slow: start, peer: start},
					nodeLastOnline:   make(map[string]time.Time),
					nodeRRDMemCache:  make(map[string]rrdMemCacheEntry),
					pveClients: map[string]PVEClientInterface{
						slow: &pollWorkFixturePVE{getNodes: func(ctx context.Context) ([]proxmox.Node, error) {
							// Both the original request and an erroneous second request
							// hold their worker until the test releases the source.
							slowStarts.Add(1)
							select {
							case <-release:
								return []proxmox.Node{}, nil
							default:
								<-release
								return nil, ctx.Err()
							}
						}},
					},
				}
				retryDelay := backoff
				if tc.adaptive {
					m.config.AdaptivePollingEnabled = true
					m.config.AdaptivePollingMaxInterval = cadence
					retryDelay = 4 * time.Second // Existing tight-cadence cap.
					m.scheduler = NewAdaptiveScheduler(SchedulerConfig{BaseInterval: cadence, MinInterval: cadence, MaxInterval: cadence}, nil, nil, nil)
				}
				m.executor = newRealExecutor(m)
				if tc.shared {
					go m.dispatchTaskWorkers(ctx, newPollTaskWorkerLimiter(2))
				} else {
					go m.taskWorker(ctx, 0)
					go m.taskWorker(ctx, 1)
				}

				m.poll(ctx, nil)
				time.Sleep(fixtTick)
				synctest.Wait()
				if n := slowStarts.Load(); n != 1 {
					t.Fatalf("first ordinary poll did not hold exactly one worker: %d", n)
				}

				// A later planning tick sees the slow connection again. Only
				// after the duplicate could enter the spare slot do we add a
				// healthy peer as the next inventory change.
				time.Sleep(cadence)
				m.poll(ctx, nil)
				time.Sleep(fixtTick)
				synctest.Wait()
				m.mu.Lock()
				m.pveClients[peer] = &stubPVEClient{nodes: []proxmox.Node{
					{Node: peer, Status: "online", CPU: 0.42, Uptime: 36000},
				}}
				m.mu.Unlock()
				m.poll(ctx, nil)
				time.Sleep(fixtTick)
				synctest.Wait()

				nodes := m.state.GetSnapshot().Nodes
				t.Logf("while slow poll is held: source calls=%d, peer rows=%d", slowStarts.Load(), len(nodes))
				if slowStarts.Load() != 1 || len(nodes) != 1 || nodes[0].Name != peer || nodes[0].CPU != 0.42 {
					t.Fatalf("the slow connection must use one worker while the peer publishes; calls=%d, nodes=%+v", slowStarts.Load(), nodes)
				}
				// A node can reach the snapshot before the rest of its poll runs.
				// Require the ordinary history write as well.
				history := m.metricsHistory.GetNodeMetrics(nodes[0].ID, "cpu", time.Minute)
				if len(history) != 1 || history[0].Value != 42 {
					t.Fatalf("healthy peer did not complete the CPU history write: %+v", history)
				}

				time.Sleep(budget)
				synctest.Wait()
				m.poll(ctx, nil)
				time.Sleep(fixtTick)
				synctest.Wait()
				if slowStarts.Load() != 1 {
					t.Fatal("expiry released a claim before the original provider call returned")
				}

				unblock()
				synctest.Wait()
				retry, ok := m.taskQueue.Get(InstanceTypePVE, slow)
				if !ok || retry.NextRun.Sub(time.Now()) != retryDelay {
					t.Fatalf("returned timeout must enter its normal retry delay, got %+v, queued=%v", retry, ok)
				}
				time.Sleep(retryDelay)
				synctest.Wait()
				if n := slowStarts.Load(); n != 2 || m.pollStatusMap[schedulerKey(InstanceTypePVE, slow)].LastSuccess.IsZero() {
					t.Fatalf("same connection did not resume its next scheduled poll, starts=%d", n)
				}
			})
		})
	}
}

func TestScheduledPollClaimPreservesExplicitEdits(t *testing.T) {
	for _, action := range []string{"replace", "remove", "remove-then-replace", "replace-then-remove", "normal", "abandon"} {
		t.Run(action, func(t *testing.T) {
			q := NewTaskQueue()
			target := ScheduledTask{InstanceType: InstanceTypeAvailability, InstanceName: "fixture-target", NextRun: time.Now()}
			q.Upsert(target)
			claimed, ok := q.WaitNext(t.Context())
			if !ok || claimed != target {
				t.Fatalf("initial claim = %+v, %v", claimed, ok)
			}
			next, edit, lastEdit := target, target, target
			next.Interval = time.Minute
			edit.Interval = 20 * time.Second
			lastEdit.Interval = 30 * time.Second
			lastEdit.NextRun = time.Now().Add(lastEdit.Interval)
			switch action {
			case "replace", "replace-then-remove":
				q.Upsert(edit)
				q.Upsert(lastEdit)
			case "remove-then-replace":
				q.Remove(target.InstanceType, target.InstanceName)
				q.Upsert(lastEdit)
			}
			if action == "remove" || action == "replace-then-remove" {
				q.Remove(target.InstanceType, target.InstanceName)
			}
			q.upsertPlan(target, 0)
			if got, available := q.Get(target.InstanceType, target.InstanceName); available || q.Size() != 0 {
				t.Fatalf("claimed target is not queued, got %+v, %v, size %d", got, available, q.Size())
			}
			// Identity is not a global name lock: another provider with the
			// same name can still dispatch while the first claim is held.
			namesake := target
			namesake.InstanceType = InstanceTypePVE
			q.upsertPlan(namesake, 0)
			sibling, ok := q.WaitNext(t.Context())
			if !ok || sibling != namesake {
				t.Fatalf("namesake claim = %+v, %v", sibling, ok)
			}
			q.completeClaim(sibling, ScheduledTask{}, noPollReschedule)

			reschedule := pollOnCadence
			if action == "abandon" {
				reschedule = noPollReschedule
			}
			q.completeClaim(claimed, next, reschedule)
			got, queued := q.Get(target.InstanceType, target.InstanceName)
			switch action {
			case "replace", "remove-then-replace":
				q.upsertPlan(target, 0)
				got, queued = q.Get(target.InstanceType, target.InstanceName)
				if !queued || got != lastEdit {
					t.Fatalf("worker follow-up or periodic plan replaced the explicit edit: %+v, %v", got, queued)
				}
			case "remove", "replace-then-remove", "abandon":
				if queued {
					t.Fatalf("completion resurrected a discarded schedule: %+v", got)
				}
			case "normal":
				if !queued || got != next {
					t.Fatalf("ordinary successor = %+v, %v", got, queued)
				}
			}
		})
	}
}

func withScheduledPollPanicFixture(t *testing.T, processLimit bool, observe func(*testing.T, *Monitor)) {
	t.Helper()
	synctest.Test(t, func(t *testing.T) {
		const (
			peer    = "fixture-peer"
			fault   = "fixture-task-preparation"
			cadence = 45 * time.Second
		)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		origin := time.Now()
		registry := prometheus.NewRegistry()
		physicalDisks := false
		peerReads, faultCalls := 0, 0
		m := &Monitor{
			config: &config.Config{PVEPollingInterval: cadence, PVEInstances: []config.PVEInstance{{
				Name: peer, Host: "https://fixture-peer.invalid:8006", MonitorPhysicalDisks: &physicalDisks,
			}}},
			state: models.NewState(), alertManager: &alerts.Manager{}, runtimeCtx: ctx,
			metricsHistory: NewMetricsHistory(16, time.Hour),
			taskQueue:      NewTaskQueue(), pollMetrics: newPollMetrics(registry),
			pollStatusMap: make(map[string]*pollStatus),
			lastOutcome:   make(map[string]taskOutcome), failureCounts: make(map[string]int),
			lastClusterCheck: map[string]time.Time{peer: origin},
			nodeLastOnline:   make(map[string]time.Time),
			nodeRRDMemCache:  make(map[string]rrdMemCacheEntry),
			pveClients: map[string]PVEClientInterface{
				peer: &pollWorkFixturePVE{getNodes: func(context.Context) ([]proxmox.Node, error) {
					peerReads++
					cpu := 0.24
					if peerReads > 1 {
						cpu = 0.42
					}
					return []proxmox.Node{{Node: peer, Status: "online", CPU: cpu, Uptime: 36000}}, nil
				}},
			},
			pollProviders: map[InstanceType]PollProvider{
				InstanceTypePBS: pollProviderAdapter{
					instanceType: InstanceTypePBS,
					buildPollTask: func(*Monitor, string) (PollTask, error) {
						faultCalls++
						panic("fixture panic during poll preparation")
					},
				},
			},
		}
		m.executor = newRealExecutor(m)
		m.taskQueue.Upsert(ScheduledTask{InstanceType: InstanceTypePVE, InstanceName: peer, NextRun: origin, Interval: cadence})
		failed := ScheduledTask{InstanceType: InstanceTypePBS, InstanceName: fault, NextRun: origin.Add(time.Second), Interval: cadence}
		m.taskQueue.Upsert(failed)
		if processLimit {
			go m.dispatchTaskWorkers(ctx, newPollTaskWorkerLimiter(1))
		} else {
			go m.taskWorker(ctx, 0)
		}

		synctest.Wait()
		if next, ok := m.taskQueue.Get(InstanceTypePVE, peer); peerReads != 1 || !ok || next.NextRun != origin.Add(cadence) {
			t.Fatalf("expected initial peer read and ordinary successor at %v: reads=%d queued=%v next=%+v", origin.Add(cadence), peerReads, ok, next)
		}
		time.Sleep(time.Second)
		synctest.Wait()
		if faultCalls != 1 {
			t.Fatalf("fault did not run exactly once: %d", faultCalls)
		}
		if _, ok := m.taskQueue.Get(failed.InstanceType, failed.InstanceName); ok {
			t.Fatal("panicking attempt should not immediately reschedule itself")
		}
		if _, ok := m.lastOutcome[schedulerKey(failed.InstanceType, failed.InstanceName)]; ok {
			t.Fatal("poll preparation panic should not fabricate a provider outcome")
		}

		replanned := failed
		replanned.NextRun = origin.Add(time.Hour)
		m.taskQueue.upsertPlan(replanned, 0)
		if got, ok := m.taskQueue.Get(failed.InstanceType, failed.InstanceName); !ok || got != replanned {
			t.Fatalf("panicking attempt left its claim behind: queued=%v task=%+v", ok, got)
		}
		time.Sleep(time.Until(origin.Add(cadence)))
		synctest.Wait()
		assertClaimedPollOverdue(t, registry, 0, 0)
		observe(t, m)
	})
}

func TestScheduledPollTaskPanicDoesNotStopWorker(t *testing.T) {
	for _, tc := range []struct {
		name         string
		processLimit bool
	}{
		{name: "default-single-worker"},
		{name: "process-limit-one", processLimit: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			withScheduledPollPanicFixture(t, tc.processLimit, func(t *testing.T, m *Monitor) {
				nodes := m.state.GetSnapshot().Nodes
				if len(nodes) != 1 || nodes[0].Name != "fixture-peer" {
					t.Fatalf("expected only the fixture peer, got %d nodes", len(nodes))
				}
				if nodes[0].CPU != 0.42 {
					t.Fatalf("the scheduled peer did not publish new CPU data after the fault: got %.0f%%, want 42%%", nodes[0].CPU*100)
				}
				history := m.GetNodeMetrics(nodes[0].ID, "cpu", time.Minute)
				if len(history) != 2 || history[0].Value != 24 || history[1].Value != 42 {
					t.Fatalf("the peer must complete both ordinary History writes: %+v", history)
				}
			})
		})
	}
}
