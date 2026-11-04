package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func sameNameConnectionsSnapshot() models.StateSnapshot {
	return models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "site-a-pmx1", Instance: "site-a", Name: "pmx1", Status: "online"},
			{ID: "site-b-pmx1", Instance: "site-b", Name: "pmx1", Status: "offline"},
		},
		VMs: []models.VM{
			{ID: "site-a:pmx1:101", Instance: "site-a", Node: "pmx1", VMID: 101, Name: "alpha", Type: "qemu", Status: "running"},
			{ID: "site-b:pmx1:101", Instance: "site-b", Node: "pmx1", VMID: 101, Name: "bravo", Type: "qemu", Status: "stopped"},
		},
		Containers: []models.Container{
			{ID: "site-a:pmx1:201", Instance: "site-a", Node: "pmx1", VMID: 201, Name: "charlie", Type: "lxc", Status: "running"},
			{ID: "site-b:pmx1:201", Instance: "site-b", Node: "pmx1", VMID: 201, Name: "delta", Type: "lxc", Status: "stopped"},
		},
	}
}

func sameNameRegistry(snapshot models.StateSnapshot) *unifiedresources.ResourceRegistry {
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(snapshot)
	return registry
}

func sameNameExecutor(registry *unifiedresources.ResourceRegistry, agents ...agentexec.ConnectedAgent) *PulseToolExecutor {
	return NewPulseToolExecutor(ExecutorConfig{
		ReadState:    registry,
		AgentServer:  &mockAgentServer{agents: agents},
		ControlLevel: ControlLevelControlled,
	})
}

// sameNameTopology reads the JSON the model receives, so these tests hold for
// any internal representation of a node's identity.
func sameNameTopology(t *testing.T, executor *PulseToolExecutor, args map[string]interface{}) (map[string]any, []map[string]any) {
	t.Helper()
	input := map[string]interface{}{"include": "proxmox"}
	for key, value := range args {
		input[key] = value
	}
	result, err := executor.executeGetTopology(context.Background(), input)
	if err != nil || result.IsError {
		t.Fatalf("topology failed: %v %+v", err, result)
	}
	var decoded struct {
		Summary map[string]any `json:"summary"`
		Proxmox struct {
			Nodes []map[string]any `json:"nodes"`
		} `json:"proxmox"`
	}
	if err := json.Unmarshal([]byte(result.Content[0].Text), &decoded); err != nil {
		t.Fatal(err)
	}
	return decoded.Summary, decoded.Proxmox.Nodes
}

func sameNameNodeID(t *testing.T, registry *unifiedresources.ResourceRegistry, instance string) string {
	t.Helper()
	for _, node := range registry.Nodes() {
		if node != nil && node.Instance() == instance {
			return node.ID()
		}
	}
	t.Fatalf("no node on connection %q", instance)
	return ""
}

func sameNameGuestNames(t *testing.T, node map[string]any, field string) []string {
	t.Helper()
	rows, ok := node[field].([]any)
	if !ok {
		t.Fatalf("%s is not a list: %+v", field, node)
	}
	names := []string{}
	for _, row := range rows {
		names = append(names, row.(map[string]any)["name"].(string))
	}
	return names
}

func TestTopologyKeepsSameNamedNodesOnTheirOwnConnections(t *testing.T) {
	registry := sameNameRegistry(sameNameConnectionsSnapshot())
	_, nodes := sameNameTopology(t, sameNameExecutor(registry), nil)
	if len(nodes) != 2 {
		t.Fatalf("same-named nodes on two connections merged: %+v", nodes)
	}
	for i, want := range []struct{ instance, status, vm, container string }{
		{"site-a", "online", "alpha", "charlie"},
		{"site-b", "offline", "bravo", "delta"},
	} {
		node := nodes[i]
		if node["name"] != "pmx1" || node["instance"] != want.instance || node["status"] != want.status {
			t.Fatalf("node %d is not %s/pmx1 %s: %+v", i, want.instance, want.status, node)
		}
		// Assistant refuses a repeated name and asks for the canonical ID, so
		// the row must carry it.
		if node["id"] != sameNameNodeID(t, registry, want.instance) {
			t.Fatalf("node %d has no canonical ID: %+v", i, node)
		}
		if got := sameNameGuestNames(t, node, "vms"); len(got) != 1 || got[0] != want.vm || node["vm_count"] != float64(1) {
			t.Fatalf("%s holds VMs %v: %+v", want.instance, got, node)
		}
		if got := sameNameGuestNames(t, node, "containers"); len(got) != 1 || got[0] != want.container || node["container_count"] != float64(1) {
			t.Fatalf("%s holds containers %v: %+v", want.instance, got, node)
		}
		if summary, _ := node["ai_safe_summary"].(string); !strings.Contains(summary, "status "+want.status) {
			t.Fatalf("%s took another node's governed metadata: %+v", want.instance, node)
		}
	}
}

func TestTopologyCommandFlagsFollowTheNodeNotItsName(t *testing.T) {
	linkedTo := func(snapshot *models.StateSnapshot, nodeIndex int, agentID string) {
		snapshot.Nodes[nodeIndex].LinkedAgentID = agentID
		snapshot.Hosts = append(snapshot.Hosts, models.Host{ID: agentID, Hostname: "pmx1", LinkedNodeID: snapshot.Nodes[nodeIndex].ID, CommandsEnabled: true})
	}
	for _, tc := range []struct {
		name   string
		build  func(*models.StateSnapshot)
		agents []agentexec.ConnectedAgent
		want   map[string]string // connection -> agent that runs its commands
	}{
		{
			name:   "agent linked to one connection",
			build:  func(s *models.StateSnapshot) { linkedTo(s, 0, "agent-a") },
			agents: []agentexec.ConnectedAgent{{AgentID: "agent-a", Hostname: "pmx1"}},
			want:   map[string]string{"site-a": "agent-a", "site-b": ""},
		},
		{
			name: "a connected agent on each connection",
			build: func(s *models.StateSnapshot) {
				linkedTo(s, 0, "agent-a")
				linkedTo(s, 1, "agent-b")
			},
			agents: []agentexec.ConnectedAgent{{AgentID: "agent-a", Hostname: "pmx1"}, {AgentID: "agent-b", Hostname: "pmx1"}},
			want:   map[string]string{"site-a": "agent-a", "site-b": "agent-b"},
		},
		{
			name: "an unlinked agent that only shares the name",
			build: func(s *models.StateSnapshot) {
				s.Hosts = append(s.Hosts, models.Host{ID: "agent-x", Hostname: "pmx1", CommandsEnabled: true})
			},
			agents: []agentexec.ConnectedAgent{{AgentID: "agent-x", Hostname: "pmx1"}},
			want:   map[string]string{"site-a": "", "site-b": ""},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			snapshot := sameNameConnectionsSnapshot()
			tc.build(&snapshot)
			registry := sameNameRegistry(snapshot)
			executor := sameNameExecutor(registry, tc.agents...)
			summary, nodes := sameNameTopology(t, executor, nil)
			if len(nodes) != 2 {
				t.Fatalf("expected a row per connection: %+v", nodes)
			}
			connected := float64(0)
			for _, node := range nodes {
				instance, _ := node["instance"].(string)
				wantAgent, known := tc.want[instance]
				if !known {
					t.Fatalf("unexpected row: %+v", node)
				}
				hasAgent := wantAgent != ""
				if hasAgent {
					connected++
				}
				if node["command_agent_connected"] != hasAgent || node["can_execute"] != hasAgent {
					t.Fatalf("%s should report connected=%t: %+v", instance, hasAgent, node)
				}
				// The hint must agree with where a command would actually go.
				if routed := executor.resolveTargetForCommandFull(sameNameNodeID(t, registry, instance)); routed.AgentID != wantAgent {
					t.Fatalf("%s routed to %q, want %q", instance, routed.AgentID, wantAgent)
				}
			}
			if summary["nodes_with_command_agents"] != connected {
				t.Fatalf("summary counted the name, not the node: %+v", summary)
			}
		})
	}
}

func TestGuestWithoutItsNodeNeverBorrowsANamesakesAgent(t *testing.T) {
	registry := sameNameRegistry(models.StateSnapshot{
		Nodes: []models.Node{{ID: "site-a-pmx1", Instance: "site-a", Name: "pmx1", Status: "online", LinkedAgentID: "agent-a"}},
		Hosts: []models.Host{{ID: "agent-a", Hostname: "pmx1", LinkedNodeID: "site-a-pmx1", CommandsEnabled: true}},
		VMs:   []models.VM{{ID: "site-b:pmx1:101", Instance: "site-b", Node: "pmx1", VMID: 101, Name: "bravo", Type: "qemu", Status: "running"}},
	})
	executor := sameNameExecutor(registry, agentexec.ConnectedAgent{AgentID: "agent-a", Hostname: "pmx1"})

	summary, nodes := sameNameTopology(t, executor, nil)
	if len(nodes) != 2 || nodes[0]["instance"] != "site-a" || nodes[1]["instance"] != "site-b" {
		t.Fatalf("a guest moved onto another connection's node: %+v", nodes)
	}
	if got := sameNameGuestNames(t, nodes[0], "vms"); len(got) != 0 || nodes[0]["command_agent_connected"] != true {
		t.Fatalf("site-a holds %v: %+v", got, nodes[0])
	}
	if got := sameNameGuestNames(t, nodes[1], "vms"); len(got) != 1 || got[0] != "bravo" {
		t.Fatalf("site-b holds %v: %+v", got, nodes[1])
	}
	if nodes[1]["status"] != "unknown" || nodes[1]["command_agent_connected"] != false || nodes[1]["can_execute"] != false {
		t.Fatalf("a node missing from inventory cannot claim the namesake's agent: %+v", nodes[1])
	}
	if nodes[1]["ai_safe_summary"] != nil || nodes[1]["policy"] != nil {
		t.Fatalf("a node missing from inventory cannot take the namesake's metadata: %+v", nodes[1])
	}
	if summary["nodes_with_command_agents"] != float64(1) {
		t.Fatalf("unexpected connected count: %+v", summary)
	}

	guest := registry.VMs()[0]
	if routed := executor.resolveTargetForCommandFull(guest.ID()); routed.AgentID != "" || routed.Transport != "qm_guest_exec" {
		t.Fatalf("guest on site-b routed through site-a's agent: %+v", routed)
	}
}

func TestNodeNameWithoutANodeRoutesOnlyWhenOneConnectionUsesIt(t *testing.T) {
	guest := func(instance string, vmid int) models.VM {
		return models.VM{ID: instance + ":pmx1", Instance: instance, Node: "pmx1", VMID: vmid, Name: instance + "-guest", Type: "qemu", Status: "running"}
	}
	agent := agentexec.ConnectedAgent{AgentID: "agent-x", Hostname: "pmx1"}

	one := sameNameRegistry(models.StateSnapshot{VMs: []models.VM{guest("site-a", 101)}})
	oneExecutor := sameNameExecutor(one, agent)
	if routed := oneExecutor.resolveTargetForCommandFull(one.VMs()[0].ID()); routed.AgentID != "agent-x" {
		t.Fatalf("a unique node name should still route by hostname: %+v", routed)
	}
	if _, nodes := sameNameTopology(t, oneExecutor, nil); len(nodes) != 1 || nodes[0]["command_agent_connected"] != true {
		t.Fatalf("a unique node name should still report its agent: %+v", nodes)
	}

	two := sameNameRegistry(models.StateSnapshot{VMs: []models.VM{guest("site-a", 101), guest("site-b", 102)}})
	twoExecutor := sameNameExecutor(two, agent)
	for _, view := range two.VMs() {
		if routed := twoExecutor.resolveTargetForCommandFull(view.ID()); routed.AgentID != "" {
			t.Fatalf("%s routed by a name two connections use: %+v", view.Name(), routed)
		}
	}
	_, nodes := sameNameTopology(t, twoExecutor, nil)
	if len(nodes) != 2 {
		t.Fatalf("expected a row per connection: %+v", nodes)
	}
	for _, node := range nodes {
		if node["command_agent_connected"] != false || node["instance"] == nil || node["id"] != nil {
			t.Fatalf("a guest-only row names its connection, has no node ID and no agent: %+v", node)
		}
	}
}

// The registry gives a guest with no connection a canonical parent, and routing
// follows that parent. Topology must report the same node, once.
func TestGuestWithoutAConnectionStaysOnItsCanonicalParent(t *testing.T) {
	guest := models.VM{ID: "pmx1:101", Node: "pmx1", VMID: 101, Name: "loose", Type: "qemu", Status: "running"}
	for _, tc := range []struct {
		name string
		rows int
	}{{"site-a-only", 1}, {"both-connections", 2}} {
		t.Run(tc.name, func(t *testing.T) {
			snapshot := sameNameConnectionsSnapshot()
			snapshot.Nodes = snapshot.Nodes[:tc.rows]
			snapshot.VMs, snapshot.Containers = []models.VM{guest}, nil
			registry := sameNameRegistry(snapshot)
			parent := registry.VMs()[0].ParentID()
			if parent == "" {
				t.Fatal("fixture no longer has a canonical parent")
			}
			_, nodes := sameNameTopology(t, sameNameExecutor(registry), nil)
			if len(nodes) != tc.rows {
				t.Fatalf("a guest with no connection added a row: %+v", nodes)
			}
			holders := 0
			for _, node := range nodes {
				if len(sameNameGuestNames(t, node, "vms")) == 0 {
					continue
				}
				holders++
				if tc.rows > 1 && node["id"] != parent {
					t.Fatalf("guest shown on %v, its parent is %s", node["id"], parent)
				}
			}
			if holders != 1 {
				t.Fatalf("guest shown on %d rows: %+v", holders, nodes)
			}
		})
	}
}

func TestTopologyCapKeepsTheConnectionOfARepeatedName(t *testing.T) {
	registry := sameNameRegistry(sameNameConnectionsSnapshot())
	_, nodes := sameNameTopology(t, sameNameExecutor(registry), map[string]interface{}{"max_proxmox_nodes": 1})
	if len(nodes) != 1 {
		t.Fatalf("cap ignored: %+v", nodes)
	}
	instance, _ := nodes[0]["instance"].(string)
	if instance == "" || nodes[0]["id"] != sameNameNodeID(t, registry, instance) {
		t.Fatalf("capped row hides that its name repeats: %+v", nodes[0])
	}
	if got := sameNameGuestNames(t, nodes[0], "vms"); len(got) != 1 {
		t.Fatalf("capped row took the other connection's guests: %v", got)
	}
}

// A display name is what the row shows. Guests name the Proxmox node, so keying
// on the display name split one machine into an empty row and an "unknown" one.
func TestTopologyDisplayNameDoesNotSplitTheNode(t *testing.T) {
	registry := sameNameRegistry(models.StateSnapshot{
		Nodes: []models.Node{{ID: "homelab-pve1", Instance: "homelab", Name: "pve1", DisplayName: "Rack One", Status: "online"}},
		VMs:   []models.VM{{ID: "homelab:pve1:100", Instance: "homelab", Node: "pve1", VMID: 100, Name: "web", Type: "qemu", Status: "running"}},
	})
	executor := sameNameExecutor(registry, agentexec.ConnectedAgent{AgentID: "agent-1", Hostname: "pve1"})
	_, nodes := sameNameTopology(t, executor, nil)
	if len(nodes) != 1 || nodes[0]["name"] != "Rack One" || nodes[0]["status"] != "online" || nodes[0]["command_agent_connected"] != true {
		t.Fatalf("display name split or disconnected the node: %+v", nodes)
	}
	if got := sameNameGuestNames(t, nodes[0], "vms"); len(got) != 1 {
		t.Fatalf("guest lost from its node: %+v", nodes[0])
	}
}

func TestSingleInstallationTopologyOutputIsUnchanged(t *testing.T) {
	snapshot := models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "homelab-pve2", Instance: "homelab", Name: "pve2", Status: "online"},
			{ID: "homelab-pve1", Instance: "homelab", Name: "pve1", Status: "online"},
		},
		VMs:        []models.VM{{ID: "homelab:pve1:100", Instance: "homelab", Node: "pve1", VMID: 100, Name: "web", Type: "qemu", Status: "running"}},
		Containers: []models.Container{{ID: "homelab:pve2:200", Instance: "homelab", Node: "pve2", VMID: 200, Name: "db", Type: "lxc", Status: "stopped"}},
	}
	want := `[` +
		`{"can_execute":true,"command_agent_connected":true,"container_count":0,"containers":[],"name":"pve1","status":"online","vm_count":1,` +
		`"vms":[{"memory_evidence":{"available":false,"pressure_known":false,"state":"unavailable"},"memory_percent":null,"name":"web","status":"online","tags":[],"vmid":100}]},` +
		`{"can_execute":false,"command_agent_connected":false,"container_count":1,` +
		`"containers":[{"memory_evidence":{"available":false,"pressure_known":false,"state":"unavailable"},"memory_percent":null,"name":"db","status":"offline","tags":[],"vmid":200}],` +
		`"name":"pve2","status":"online","vm_count":0,"vms":[]}` +
		`]`
	_, nodes := sameNameTopology(t, sameNameExecutor(sameNameRegistry(snapshot), agentexec.ConnectedAgent{AgentID: "agent-1", Hostname: "pve1"}), nil)
	for _, node := range nodes {
		delete(node, "policy")
		delete(node, "ai_safe_summary")
		for _, field := range []string{"vms", "containers"} {
			for _, guest := range node[field].([]any) {
				delete(guest.(map[string]any), "policy")
				delete(guest.(map[string]any), "ai_safe_summary")
			}
		}
	}
	got, err := json.Marshal(nodes)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("single-installation topology changed:\n got %s\nwant %s", got, want)
	}

	// Several connections whose node names differ have nothing to disambiguate.
	snapshot.Nodes[0].Instance = "office"
	snapshot.Containers[0].Instance = "office"
	_, nodes = sameNameTopology(t, sameNameExecutor(sameNameRegistry(snapshot)), nil)
	if len(nodes) != 2 || nodes[0]["instance"] != nil || nodes[0]["id"] != nil || nodes[1]["instance"] != nil || nodes[1]["id"] != nil {
		t.Fatalf("distinct names gained connection noise: %+v", nodes)
	}
}

func TestLinkedAgentOfflineNeverFallsBackToTheHostname(t *testing.T) {
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestResources([]unifiedresources.Resource{
		{ID: "node-a", Type: unifiedresources.ResourceTypeAgent, Name: "pmx1", Sources: []unifiedresources.DataSource{unifiedresources.SourceProxmox}, Proxmox: &unifiedresources.ProxmoxData{Instance: "site-a", NodeName: "pmx1", SourceID: "site-a:pmx1"}},
		{ID: "host-a", Type: unifiedresources.ResourceTypeAgent, Name: "rack-host", Sources: []unifiedresources.DataSource{unifiedresources.SourceAgent}, Agent: &unifiedresources.AgentData{AgentID: "linked", Hostname: "rack-host", LinkedNodeID: "node-a", CommandsEnabled: true}},
	})
	executor := sameNameExecutor(registry, agentexec.ConnectedAgent{AgentID: "other-machine", Hostname: "pmx1"})
	if routed := executor.resolveTargetForCommandFull("node-a"); routed.AgentID != "" {
		t.Fatalf("offline linked agent replaced by a hostname match: %+v", routed)
	}
	if _, nodes := sameNameTopology(t, executor, nil); len(nodes) != 1 || nodes[0]["command_agent_connected"] != false {
		t.Fatalf("topology must agree with routing: %+v", nodes)
	}
}
