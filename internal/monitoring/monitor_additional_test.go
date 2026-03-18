package monitoring

import (
	"context"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	unifiedresources "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
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

	endpoint = config.ClusterEndpoint{}
	if got := clusterEndpointEffectiveURL(endpoint, true, false); got != "" {
		t.Fatalf("empty endpoint = %q, want empty", got)
	}
}
