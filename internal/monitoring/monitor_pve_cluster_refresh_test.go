package monitoring

import (
	"context"
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

	merged := mergeRefreshedClusterEndpoints(existing, discovered)
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
