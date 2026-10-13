package tools

import (
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"testing"
)

func TestCanonicalCommandTargetKeepsProviderScopedParent(t *testing.T) {
	rr := unifiedresources.NewRegistry(nil)
	records := []unifiedresources.Resource{}
	for _, instance := range []string{"cluster-a", "cluster-b"} {
		nodeID := "node-" + instance
		guestID := unifiedresources.ProxmoxGuestCanonicalID(unifiedresources.ResourceTypeVM, instance, 101)
		records = append(records,
			unifiedresources.Resource{ID: nodeID, Type: unifiedresources.ResourceTypeAgent, Name: "node1", Sources: []unifiedresources.DataSource{unifiedresources.SourceProxmox, unifiedresources.SourceAgent}, Proxmox: &unifiedresources.ProxmoxData{Instance: instance, NodeName: "node1", SourceID: instance + ":node1"}, Agent: &unifiedresources.AgentData{AgentID: "native-" + instance, Hostname: "node1", CommandsEnabled: true}},
			unifiedresources.Resource{ID: guestID, Type: unifiedresources.ResourceTypeVM, Name: "duplicate", Sources: []unifiedresources.DataSource{unifiedresources.SourceProxmox}, ParentID: &nodeID, Proxmox: &unifiedresources.ProxmoxData{Instance: instance, NodeName: "node1", VMID: 101, SourceID: instance + ":node1:101"}},
		)
	}
	rr.IngestResources(records)
	if len(rr.VMs()) != 2 || len(rr.Nodes()) != 2 {
		t.Fatalf("fixture conflated provider scopes: VMs=%d nodes=%d", len(rr.VMs()), len(rr.Nodes()))
	}
	server := &mockAgentServer{agents: []agentexec.ConnectedAgent{{AgentID: "native-cluster-a", Hostname: "node1"}, {AgentID: "native-cluster-b", Hostname: "node1"}}}
	executor := NewPulseToolExecutor(ExecutorConfig{ReadState: rr, AgentServer: server})
	for _, instance := range []string{"cluster-a", "cluster-b"} {
		id := unifiedresources.ProxmoxGuestCanonicalID(unifiedresources.ResourceTypeVM, instance, 101)
		result := executor.resolveTargetForCommandFull(id)
		if result.AgentID != "native-"+instance || result.TargetID != "101" || result.Transport != "qm_guest_exec" {
			t.Fatalf("canonical guest routed through another provider: %s %+v", instance, result)
		}
	}
	if id := executor.findAgentForNode("node1"); id != "" {
		t.Fatalf("ambiguous legacy node name selected %s", id)
	}
}

func TestAmbiguousResourceNameCannotFallBackToConnectedHostname(t *testing.T) {
	rr := unifiedresources.NewRegistry(nil)
	rr.IngestSnapshot(models.StateSnapshot{VMs: []models.VM{
		{ID: "cluster-a:node1:101", Instance: "cluster-a", Node: "node1", VMID: 101, Name: "duplicate", Type: "qemu"},
		{ID: "cluster-b:node1:101", Instance: "cluster-b", Node: "node1", VMID: 101, Name: "duplicate", Type: "qemu"},
	}})
	executor := NewPulseToolExecutor(ExecutorConfig{ReadState: rr, AgentServer: &mockAgentServer{agents: []agentexec.ConnectedAgent{{AgentID: "other", Hostname: "duplicate"}}}})
	result := executor.resolveTargetForCommandFull("duplicate")
	if result.AgentID != "" || result.ResolvedKind != "ambiguous" {
		t.Fatalf("known ambiguity escaped topology ownership: %+v", result)
	}
}

func TestKnownAgentIDCannotRouteToAnImpostorHostname(t *testing.T) {
	rr := unifiedresources.NewRegistry(nil)
	rr.IngestSnapshot(models.StateSnapshot{Hosts: []models.Host{{ID: "native-target", Hostname: "target", CommandsEnabled: true}}})
	executor := NewPulseToolExecutor(ExecutorConfig{ReadState: rr, AgentServer: &mockAgentServer{agents: []agentexec.ConnectedAgent{{AgentID: "different-agent", Hostname: "target"}}}})
	result := executor.resolveTargetForCommandFull(rr.Hosts()[0].ID())
	if result.AgentID != "" || result.ResolvedKind != "agent" {
		t.Fatalf("a known agent ID was rebound by hostname: %+v", result)
	}
}
