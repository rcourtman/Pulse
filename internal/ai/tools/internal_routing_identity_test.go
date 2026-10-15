package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/stretchr/testify/mock"
)

func TestDockerLogReadRoutesToHostWithCollectedChildren(t *testing.T) {
	rr := unifiedresources.NewRegistry(nil)
	rr.IngestSnapshot(models.StateSnapshot{
		Hosts: []models.Host{{ID: "native-host", Hostname: "unattended-lab", MachineID: "machine-1", CommandsEnabled: true}},
		DockerHosts: []models.DockerHost{{
			ID: "native-host", AgentID: "native-host", Hostname: "unattended-lab", MachineID: "machine-1",
			Containers: []models.DockerContainer{{ID: "container-1", Name: "target", State: "running"}, {ID: "container-2", Name: "control", State: "running"}},
			Images:     []models.DockerImage{{ID: "image-1", RepoTags: []string{"example:latest"}}},
			Networks:   []models.DockerNetwork{{ID: "network-1", Name: "bridge"}},
		}},
	})
	if len(rr.Hosts()) != 1 || len(rr.DockerHosts()) != 1 || rr.Hosts()[0].ID() != rr.DockerHosts()[0].ID() || len(rr.DockerContainers()) != 2 {
		t.Fatalf("fixture lost host or collected children")
	}
	server := &mockAgentServer{agents: []agentexec.ConnectedAgent{{AgentID: "native-host", Hostname: "unattended-lab"}}}
	server.On("GetConnectedAgents").Return(server.agents)
	server.On("ExecuteCommand", mock.Anything, "native-host", mock.MatchedBy(func(payload agentexec.ExecuteCommandPayload) bool {
		return payload.TargetType == "agent" && payload.TargetID == "" && strings.Contains(payload.Command, "docker logs") && strings.Contains(payload.Command, "target")
	})).Return(&agentexec.CommandResultPayload{ExitCode: 0, Stdout: "worker session stalled"}, nil)
	executor := NewPulseToolExecutor(ExecutorConfig{ReadState: rr, AgentServer: server, ControlLevel: ControlLevelReadOnly})
	containerID := ""
	for _, container := range rr.DockerContainers() {
		if container.Name() == "target" {
			containerID = container.ID()
		}
	}
	for _, reference := range []string{"unattended-lab", "native-host", "agent:native-host", rr.Hosts()[0].ID(), containerID} {
		t.Run(reference, func(t *testing.T) {
			result, err := executor.ExecuteTool(context.Background(), "pulse_read", map[string]interface{}{
				"action": "logs", "source": "docker", "container": "target", "since": "30m", "target_host": reference,
			})
			if err != nil || result.IsError || len(result.Content) == 0 || !strings.Contains(result.Content[0].Text, "worker session stalled") {
				t.Fatalf("registered Docker log read did not reach its connected parent: result=%+v err=%v", result, err)
			}
		})
	}
	server.AssertExpectations(t)
	// A real child/host name collision is still ambiguous. It must not use
	// the connected host merely because unrelated inherited aliases are gone.
	rr.IngestResources([]unifiedresources.Resource{{
		ID: "colliding-child", Type: unifiedresources.ResourceTypeAppContainer, Name: "unattended-lab",
		Docker:   &unifiedresources.DockerData{HostSourceID: "native-host", ContainerID: "container-3", Hostname: "unattended-lab"},
		Identity: unifiedresources.ResourceIdentity{Hostnames: []string{"unattended-lab"}},
	}})
	if result := executor.resolveTargetForCommandFull("unattended-lab"); result.AgentID != "" || result.ResolvedKind != "ambiguous" {
		t.Fatalf("a genuine child/host collision selected a command host: %+v", result)
	}
}

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
	read, err := executor.ExecuteTool(context.Background(), "pulse_read", map[string]interface{}{
		"action": "logs", "source": "docker", "container": "target", "target_host": "duplicate",
	})
	if err != nil || !read.IsError || len(read.Content) != 1 {
		t.Fatalf("ambiguous read must fail before dispatch: %+v %v", read, err)
	}
	var response ToolResponse
	if err := json.Unmarshal([]byte(read.Content[0].Text), &response); err != nil || response.Error == nil || response.Error.Code != ErrCodeStrictResolution {
		t.Fatalf("ambiguity was misreported as an absent connection: %+v %v", response, err)
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
