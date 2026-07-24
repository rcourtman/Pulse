package monitoring

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// Refreshing an existing cluster's endpoints when the cluster re-IPs (#1493):
// the stored add-time addresses must be replaced with what the cluster
// reports now, and the failover client must be rebuilt so polling stops
// dialing the dead addresses.
func TestDetectClusterMembership_RefreshesStaleClusterEndpoints(t *testing.T) {
	originalDetect := detectMonitorPVECluster
	t.Cleanup(func() { detectMonitorPVECluster = originalDetect })

	detectMonitorPVECluster = func(clientConfig proxmox.ClientConfig, existingEndpoints []config.ClusterEndpoint) (bool, string, []config.ClusterEndpoint) {
		return true, "MyCluster", []config.ClusterEndpoint{
			// Same nodes, new subnet. 127.0.0.1:1 fails instantly (connection
			// refused) so the rebuilt client's health check doesn't block.
			{NodeID: "node/proxmox0", NodeName: "proxmox0", Host: "https://proxmox0:8006", IP: "127.0.0.1", Online: true, LastSeen: time.Now()},
			{NodeID: "node/proxmox1", NodeName: "proxmox1", Host: "https://proxmox1:8006", IP: "127.0.0.2", Online: true, LastSeen: time.Now()},
		}
	}

	sentinel := &stubPVEClient{}
	m := &Monitor{
		config: &config.Config{
			PVEInstances: []config.PVEInstance{
				{
					Name:        "Proxmox2",
					Host:        "https://127.0.0.1:1",
					IsCluster:   true,
					ClusterName: "MyCluster",
					ClusterEndpoints: []config.ClusterEndpoint{
						{NodeID: "node/proxmox0", NodeName: "proxmox0", Host: "https://proxmox0:8006", IP: "127.0.0.9"},
						{NodeID: "node/proxmox1", NodeName: "proxmox1", Host: "https://proxmox1:8006", IP: "127.0.0.10"},
					},
				},
			},
		},
		state:            models.NewState(),
		pveClients:       map[string]PVEClientInterface{"Proxmox2": sentinel},
		lastClusterCheck: make(map[string]time.Time),
	}
	instanceCfg := &m.config.PVEInstances[0]

	m.detectClusterMembership(context.Background(), "Proxmox2", instanceCfg, sentinel)

	got := m.config.PVEInstances[0].ClusterEndpoints
	if len(got) != 2 {
		t.Fatalf("expected 2 refreshed endpoints, got %d", len(got))
	}
	if got[0].IP != "127.0.0.1" || got[1].IP != "127.0.0.2" {
		t.Fatalf("expected refreshed IPs, got %q and %q", got[0].IP, got[1].IP)
	}
	if instanceCfg.ClusterEndpoints[0].IP != "127.0.0.1" {
		t.Fatalf("expected in-flight instance config to see refreshed IPs, got %q", instanceCfg.ClusterEndpoints[0].IP)
	}

	replacement, ok := m.pveClients["Proxmox2"]
	if !ok {
		t.Fatal("expected cluster client to remain registered")
	}
	if replacement == PVEClientInterface(sentinel) {
		t.Fatal("expected cluster client to be rebuilt after endpoint refresh")
	}
	if _, isCluster := replacement.(*proxmox.ClusterClient); !isCluster {
		t.Fatalf("expected rebuilt client to be a ClusterClient, got %T", replacement)
	}
}

// When discovery reports the same node identities (only volatile fields like
// Online/LastSeen differ), nothing should be rewritten or rebuilt.
func TestDetectClusterMembership_NoRefreshWhenEndpointsUnchanged(t *testing.T) {
	originalDetect := detectMonitorPVECluster
	t.Cleanup(func() { detectMonitorPVECluster = originalDetect })

	detectMonitorPVECluster = func(clientConfig proxmox.ClientConfig, existingEndpoints []config.ClusterEndpoint) (bool, string, []config.ClusterEndpoint) {
		return true, "MyCluster", []config.ClusterEndpoint{
			{NodeID: "node/proxmox0", NodeName: "proxmox0", Host: "https://proxmox0:8006", IP: "127.0.0.9", Online: true, LastSeen: time.Now()},
		}
	}

	sentinel := &stubPVEClient{}
	m := &Monitor{
		config: &config.Config{
			PVEInstances: []config.PVEInstance{
				{
					Name:        "Proxmox2",
					Host:        "https://127.0.0.1:1",
					IsCluster:   true,
					ClusterName: "MyCluster",
					ClusterEndpoints: []config.ClusterEndpoint{
						{NodeID: "node/proxmox0", NodeName: "proxmox0", Host: "https://proxmox0:8006", IP: "127.0.0.9", Online: false},
					},
				},
			},
		},
		state:            models.NewState(),
		pveClients:       map[string]PVEClientInterface{"Proxmox2": sentinel},
		lastClusterCheck: make(map[string]time.Time),
	}

	m.detectClusterMembership(context.Background(), "Proxmox2", &m.config.PVEInstances[0], sentinel)

	if m.pveClients["Proxmox2"] != PVEClientInterface(sentinel) {
		t.Fatal("expected cluster client to be left alone when endpoints are unchanged")
	}
	if m.config.PVEInstances[0].ClusterEndpoints[0].Online {
		t.Fatal("expected stored endpoints to be untouched when identity is unchanged")
	}
}

func TestReconcilePVENodeInventoryRequiresRepeatedAuthoritativeRemoval(t *testing.T) {
	cfg := &config.Config{PVEInstances: []config.PVEInstance{{
		Name:        "cluster-api",
		Host:        "https://pve-a:8006",
		IsCluster:   true,
		ClusterName: "production",
		ClusterEndpoints: []config.ClusterEndpoint{
			{NodeName: "pve-a", Host: "https://pve-a:8006"},
			{NodeName: "pve-b", Host: "https://pve-b:8006"},
		},
	}}}
	m := newUnreachableTestMonitor(t, cfg)
	previous := []models.Node{
		{ID: "production-pve-a", Name: "pve-a", Instance: "cluster-api", Status: "online", LastSeen: time.Now()},
		{ID: "production-pve-b", Name: "pve-b", Instance: "cluster-api", Status: "offline"},
	}
	client := &membershipPVEClient{statuses: []proxmox.ClusterStatus{
		{Type: "cluster", Name: "production", Quorate: 1},
		{Type: "node", ID: "node/a", Name: "pve-a", Online: 1},
	}}

	first := m.reconcilePVENodeInventory(
		context.Background(),
		"cluster-api",
		&cfg.PVEInstances[0],
		client,
		previous[:1],
		previous,
	)
	if len(first) != 2 || len(cfg.PVEInstances[0].ClusterEndpoints) != 2 {
		t.Fatalf("first authoritative omission retired node: nodes=%+v endpoints=%+v", first, cfg.PVEInstances[0].ClusterEndpoints)
	}

	// A process restart resets the volatile confirmation counter while the
	// durable endpoint keeps the member visible.
	restarted := newUnreachableTestMonitor(t, cfg)
	afterRestart := restarted.reconcilePVENodeInventory(
		context.Background(),
		"cluster-api",
		&cfg.PVEInstances[0],
		client,
		first[:1],
		first,
	)
	if len(afterRestart) != 2 {
		t.Fatalf("restart converted one omission into removal: %+v", afterRestart)
	}

	// An uncertain read breaks the consecutive authoritative sequence.
	uncertain := restarted.reconcilePVENodeInventory(
		context.Background(),
		"cluster-api",
		&cfg.PVEInstances[0],
		&membershipPVEClient{statusErr: fmt.Errorf("temporary membership failure")},
		afterRestart[:1],
		afterRestart,
	)
	if len(uncertain) != 2 {
		t.Fatalf("uncertain read retired pending member: %+v", uncertain)
	}

	firstAfterUncertain := restarted.reconcilePVENodeInventory(
		context.Background(),
		"cluster-api",
		&cfg.PVEInstances[0],
		client,
		uncertain[:1],
		uncertain,
	)
	if len(firstAfterUncertain) != 2 {
		t.Fatalf("first absence after uncertainty retired member: %+v", firstAfterUncertain)
	}

	confirmed := restarted.reconcilePVENodeInventory(
		context.Background(),
		"cluster-api",
		&cfg.PVEInstances[0],
		client,
		firstAfterUncertain[:1],
		firstAfterUncertain,
	)
	if len(confirmed) != 1 || confirmed[0].Name != "pve-a" {
		t.Fatalf("confirmed removal = %+v, want only pve-a", confirmed)
	}
	if len(cfg.PVEInstances[0].ClusterEndpoints) != 1 || cfg.PVEInstances[0].ClusterEndpoints[0].NodeName != "pve-a" {
		t.Fatalf("confirmed removal did not reconcile durable endpoints: %+v", cfg.PVEInstances[0].ClusterEndpoints)
	}
}

func TestMergeRefreshedClusterEndpoints_InheritsMissingFields(t *testing.T) {
	reachable := true
	checked := time.Now().Add(-time.Minute)
	existing := []config.ClusterEndpoint{
		{
			NodeID:         "node/proxmox0",
			NodeName:       "proxmox0",
			Host:           "https://proxmox0:8006",
			IP:             "10.32.21.21",
			PulseReachable: &reachable,
			LastPulseCheck: &checked,
			PulseError:     "previous error",
		},
	}
	discovered := []config.ClusterEndpoint{
		// Cluster status omitted the IP for this node; the stored one must
		// survive the refresh.
		{NodeID: "", NodeName: "proxmox0", Host: "", IP: ""},
		{NodeID: "node/proxmox1", NodeName: "proxmox1", Host: "https://proxmox1:8006", IP: "10.32.20.22"},
	}

	merged := mergeRefreshedClusterEndpoints(existing, discovered, false)
	if len(merged) != 2 {
		t.Fatalf("expected 2 merged endpoints, got %d", len(merged))
	}
	if merged[0].IP != "10.32.21.21" || merged[0].Host != "https://proxmox0:8006" || merged[0].NodeID != "node/proxmox0" {
		t.Fatalf("expected omitted fields to be inherited, got %+v", merged[0])
	}
	if merged[0].PulseReachable != &reachable || merged[0].LastPulseCheck != &checked || merged[0].PulseError != "previous error" {
		t.Fatalf("expected Pulse reachability bookkeeping to be carried over, got %+v", merged[0])
	}
	if merged[1].IP != "10.32.20.22" {
		t.Fatalf("expected new node to keep its discovered IP, got %q", merged[1].IP)
	}
}

func TestMergeRefreshedClusterEndpoints_ReachabilityFollowsEffectiveTarget(t *testing.T) {
	reachable := true
	checked := time.Now().Add(-time.Minute)
	existing := []config.ClusterEndpoint{
		{
			NodeID:         "node/proxmox0",
			NodeName:       "proxmox0",
			Host:           "https://proxmox0:8006",
			IP:             "10.32.21.21",
			PulseReachable: &reachable,
			LastPulseCheck: &checked,
			PulseError:     "old address timed out",
		},
	}

	t.Run("network move resets stale evidence", func(t *testing.T) {
		discovered := []config.ClusterEndpoint{
			{NodeID: "node/proxmox0", NodeName: "proxmox0", Host: "https://proxmox0:8006", IP: "10.32.20.21"},
		}

		merged := mergeRefreshedClusterEndpoints(existing, discovered, false)
		if merged[0].PulseReachable != nil || merged[0].LastPulseCheck != nil || merged[0].PulseError != "" {
			t.Fatalf("expected reachability evidence to reset with the dial target, got %+v", merged[0])
		}
	})

	t.Run("stable override preserves useful evidence", func(t *testing.T) {
		withOverride := append([]config.ClusterEndpoint(nil), existing...)
		withOverride[0].IPOverride = "pulse-route.example.test"
		discovered := []config.ClusterEndpoint{
			{
				NodeID:      "node/proxmox0",
				NodeName:    "proxmox0",
				Host:        "https://proxmox0:8006",
				IP:          "10.32.20.21",
				IPOverride:  "pulse-route.example.test",
				Fingerprint: withOverride[0].Fingerprint,
			},
		}

		merged := mergeRefreshedClusterEndpoints(withOverride, discovered, false)
		if merged[0].PulseReachable != &reachable || merged[0].LastPulseCheck != &checked || merged[0].PulseError != "old address timed out" {
			t.Fatalf("expected evidence for the unchanged override target, got %+v", merged[0])
		}
	})
}

func TestBuildClusterEndpoints_PreservesConfiguredAuthority(t *testing.T) {
	monitor := &Monitor{config: &config.Config{}}
	instance := config.PVEInstance{
		Name:      "remote-cluster",
		Host:      "https://management.example.test:8006",
		VerifySSL: false,
		ClusterEndpoints: []config.ClusterEndpoint{
			{NodeName: "pve-a", IP: "10.15.5.11", Fingerprint: "AA:11"},
			{NodeName: "pve-b", IP: "10.15.5.12", Fingerprint: "BB:22"},
			// Discovery may report the configured route as a member too; it
			// remains one endpoint and keeps its member certificate evidence.
			{NodeName: "pve-c", Host: "https://management.example.test:8006", Fingerprint: "CC:33"},
		},
	}

	want := []string{
		"https://management.example.test:8006",
		"https://10.15.5.11:8006",
		"https://10.15.5.12:8006",
	}

	for _, build := range []struct {
		name string
		fn   func(config.PVEInstance) ([]string, map[string]string)
	}{
		{name: "initialization", fn: monitor.buildClusterEndpointsForInit},
		{name: "reconnect", fn: monitor.buildClusterEndpointsForReconnect},
	} {
		t.Run(build.name, func(t *testing.T) {
			endpoints, fingerprints := build.fn(instance)
			if len(endpoints) != len(want) {
				t.Fatalf("expected %d ordered endpoints, got %v", len(want), endpoints)
			}
			for i := range want {
				if endpoints[i] != want[i] {
					t.Fatalf("endpoint %d: expected %q, got %q (all: %v)", i, want[i], endpoints[i], endpoints)
				}
			}
			if fingerprints["https://10.15.5.11:8006"] != "AA:11" ||
				fingerprints["https://10.15.5.12:8006"] != "BB:22" ||
				fingerprints["https://management.example.test:8006"] != "CC:33" {
				t.Fatalf("expected per-member fingerprints to survive deduplication, got %+v", fingerprints)
			}
		})
	}
}

func TestClusterEndpointIdentityChanged(t *testing.T) {
	base := []config.ClusterEndpoint{
		{NodeID: "node/a", NodeName: "a", Host: "https://a:8006", IP: "10.0.0.1"},
		{NodeID: "node/b", NodeName: "b", Host: "https://b:8006", IP: "10.0.0.2"},
	}

	volatileOnly := []config.ClusterEndpoint{
		{NodeID: "node/a", NodeName: "a", Host: "https://a:8006", IP: "10.0.0.1", Online: true, LastSeen: time.Now()},
		{NodeID: "node/b", NodeName: "b", Host: "https://b:8006", IP: "10.0.0.2", Online: false},
	}
	if clusterEndpointIdentityChanged(base, volatileOnly) {
		t.Fatal("volatile-only differences must not count as identity changes")
	}

	reIPed := []config.ClusterEndpoint{
		{NodeID: "node/a", NodeName: "a", Host: "https://a:8006", IP: "10.32.20.1"},
		{NodeID: "node/b", NodeName: "b", Host: "https://b:8006", IP: "10.0.0.2"},
	}
	if !clusterEndpointIdentityChanged(base, reIPed) {
		t.Fatal("an IP change must count as an identity change")
	}

	nodeAdded := append(append([]config.ClusterEndpoint{}, base...), config.ClusterEndpoint{NodeID: "node/c", NodeName: "c", Host: "https://c:8006", IP: "10.0.0.3"})
	if !clusterEndpointIdentityChanged(base, nodeAdded) {
		t.Fatal("an added node must count as an identity change")
	}
}
