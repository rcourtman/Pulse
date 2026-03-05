package monitoring

import (
	"context"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
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
	if len(monitor.state.Nodes) != 1 || monitor.state.Nodes[0].LinkedHostAgentID != "host-1" {
		t.Fatalf("LinkedHostAgentID = %q, want %q", monitor.state.Nodes[0].LinkedHostAgentID, "host-1")
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

	if got := clusterEndpointEffectiveURL(endpoint, true, ""); got != "https://node.local:8006" {
		t.Fatalf("verifySSL host preference = %q, want %q", got, "https://node.local:8006")
	}

	endpoint.Host = ""
	if got := clusterEndpointEffectiveURL(endpoint, true, ""); got != "https://10.0.0.1:8006" {
		t.Fatalf("verifySSL fallback to IP = %q, want %q", got, "https://10.0.0.1:8006")
	}

	endpoint.Host = "node.local"
	if got := clusterEndpointEffectiveURL(endpoint, false, ""); got != "https://10.0.0.1:8006" {
		t.Fatalf("non-SSL IP preference = %q, want %q", got, "https://10.0.0.1:8006")
	}

	endpoint.IPOverride = "192.168.1.10"
	if got := clusterEndpointEffectiveURL(endpoint, false, ""); got != "https://192.168.1.10:8006" {
		t.Fatalf("override IP preference = %q, want %q", got, "https://192.168.1.10:8006")
	}

	endpoint.Fingerprint = "ep-fingerprint"
	if got := clusterEndpointEffectiveURL(endpoint, true, ""); got != "https://192.168.1.10:8006" {
		t.Fatalf("per-endpoint fingerprint should allow IP override, got %q", got)
	}

	endpoint = config.ClusterEndpoint{}
	if got := clusterEndpointEffectiveURL(endpoint, true, ""); got != "" {
		t.Fatalf("empty endpoint = %q, want empty", got)
	}
}

func TestBuildClusterClientEndpoints_PrefersOverrideWhenEndpointFingerprintPresent(t *testing.T) {
	pve := config.PVEInstance{
		Name:        "cluster-a",
		Host:        "https://cluster-a.local:8006",
		VerifySSL:   true,
		IsCluster:   true,
		ClusterName: "cluster-a",
		ClusterEndpoints: []config.ClusterEndpoint{
			{
				NodeName:    "node1",
				Host:        "https://node1.local:8006",
				IP:          "10.15.5.11",
				IPOverride:  "10.15.2.11",
				Fingerprint: "node1-fp",
			},
		},
	}

	endpoints, fingerprints := buildClusterClientEndpoints(pve)

	if len(endpoints) != 2 {
		t.Fatalf("expected endpoint plus main host fallback, got %d", len(endpoints))
	}
	if endpoints[0] != "https://10.15.2.11:8006" {
		t.Fatalf("expected endpoint override URL first, got %q", endpoints[0])
	}
	if fingerprints["https://10.15.2.11:8006"] != "node1-fp" {
		t.Fatalf("expected fingerprint to follow effective endpoint URL, got %q", fingerprints["https://10.15.2.11:8006"])
	}
}
