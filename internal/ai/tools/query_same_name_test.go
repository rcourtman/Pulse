package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// sameNameQueryExecutors covers both search paths: canonical resources from the
// provider, and read-state views when the provider has none.
func sameNameQueryExecutors(registry *unifiedresources.ResourceRegistry, agents ...agentexec.ConnectedAgent) map[string]*PulseToolExecutor {
	return map[string]*PulseToolExecutor{
		"canonical resources": NewPulseToolExecutor(ExecutorConfig{
			ReadState:               registry,
			UnifiedResourceProvider: &registryUnifiedQueryProvider{registry},
			AgentServer:             &mockAgentServer{agents: agents},
			ControlLevel:            ControlLevelControlled,
		}),
		"read-state views": sameNameExecutor(registry, agents...),
	}
}

// sameNameQuery reads the JSON the model receives.
func sameNameQuery(t *testing.T, executor *PulseToolExecutor, args map[string]interface{}) map[string]any {
	t.Helper()
	result, err := executor.executeQuery(context.Background(), args)
	if err != nil || result.IsError {
		t.Fatalf("query %v failed: %v %+v", args, err, result)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(result.Content[0].Text), &decoded); err != nil {
		t.Fatal(err)
	}
	return decoded
}

func sameNameRows(t *testing.T, response map[string]any, field string) []map[string]any {
	t.Helper()
	raw, ok := response[field].([]any)
	if !ok {
		t.Fatalf("%s is not a list: %+v", field, response)
	}
	rows := make([]map[string]any, 0, len(raw))
	for _, row := range raw {
		rows = append(rows, row.(map[string]any))
	}
	return rows
}

func sameNameListedNodes(t *testing.T, executor *PulseToolExecutor) []map[string]any {
	t.Helper()
	return sameNameRows(t, sameNameQuery(t, executor, map[string]interface{}{"action": "list", "type": "nodes"}), "nodes")
}

func sameNameSearchRow(t *testing.T, executor *PulseToolExecutor, query, resourceType string) map[string]any {
	t.Helper()
	rows := sameNameRows(t, sameNameQuery(t, executor, map[string]interface{}{"action": "search", "query": query, "type": resourceType}), "matches")
	if len(rows) != 1 {
		t.Fatalf("expected one %s row for %q: %+v", resourceType, query, rows)
	}
	return rows[0]
}

func TestQueryCommandFlagsFollowTheNodeNotItsName(t *testing.T) {
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
		{
			name:   "a linked agent that is offline while another shares the name",
			build:  func(s *models.StateSnapshot) { linkedTo(s, 0, "agent-a") },
			agents: []agentexec.ConnectedAgent{{AgentID: "other-machine", Hostname: "pmx1"}},
			want:   map[string]string{"site-a": "", "site-b": ""},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			snapshot := sameNameConnectionsSnapshot()
			tc.build(&snapshot)
			registry := sameNameRegistry(snapshot)
			wantByNodeID := make(map[string]string, len(tc.want))
			for instance, agent := range tc.want {
				wantByNodeID[sameNameNodeID(t, registry, instance)] = agent
			}
			for variant, executor := range sameNameQueryExecutors(registry, tc.agents...) {
				t.Run(variant, func(t *testing.T) {
					// A flag holds only if it agrees with where a command would go.
					checkNodeRow := func(surface string, row map[string]any) {
						t.Helper()
						id, _ := row["id"].(string)
						want, known := wantByNodeID[id]
						if !known {
							t.Fatalf("%s row names no node of its own: %+v", surface, row)
						}
						if row["command_agent_connected"] != (want != "") {
							t.Fatalf("%s row for %s should report connected=%t: %+v", surface, id, want != "", row)
						}
						if routed := executor.resolveTargetForCommandFull(id); routed.AgentID != want {
							t.Fatalf("%s routed to %q, want %q", id, routed.AgentID, want)
						}
					}

					listed := sameNameListedNodes(t, executor)
					if len(listed) != 2 {
						t.Fatalf("expected a row per node: %+v", listed)
					}
					for _, row := range listed {
						checkNodeRow("list", row)
						if row["instance"] != sameNameInstanceOf(t, registry, row["id"].(string)) {
							t.Fatalf("listed row hides its connection: %+v", row)
						}
					}

					matches := sameNameRows(t, sameNameQuery(t, executor, map[string]interface{}{"action": "search", "query": "pmx1", "type": "node"}), "matches")
					if len(matches) != 2 {
						t.Fatalf("expected a match per node: %+v", matches)
					}
					for _, row := range matches {
						checkNodeRow("search", row)
						if row["instance"] != sameNameInstanceOf(t, registry, row["id"].(string)) {
							t.Fatalf("matched row hides its connection: %+v", row)
						}
					}

					for _, guest := range []struct{ name, kind, instance string }{
						{"alpha", "vm", "site-a"},
						{"bravo", "vm", "site-b"},
						{"charlie", "system-container", "site-a"},
						{"delta", "system-container", "site-b"},
					} {
						row := sameNameSearchRow(t, executor, guest.name, guest.kind)
						want := tc.want[guest.instance]
						if row["node_command_agent_connected"] != (want != "") {
							t.Fatalf("%s should report its own node's connection, connected=%t: %+v", guest.name, want != "", row)
						}
						if row["command_agent_connected"] != false {
							t.Fatalf("%s took its node's connection as its own: %+v", guest.name, row)
						}
						if routed := executor.resolveTargetForCommandFull(row["id"].(string)); routed.AgentID != want {
							t.Fatalf("%s routed to %q, want %q", guest.name, routed.AgentID, want)
						}
					}

					if variant != "canonical resources" {
						return
					}
					systems := sameNameRows(t, sameNameQuery(t, executor, map[string]interface{}{"action": "list", "type": "systems"}), "systems")
					agents := sameNameRows(t, sameNameQuery(t, executor, map[string]interface{}{"action": "search", "query": "pmx1", "type": "agent"}), "matches")
					for surface, rows := range map[string][]map[string]any{"systems": systems, "agent search": agents} {
						seen := 0
						for _, row := range rows {
							if _, isNode := wantByNodeID[row["id"].(string)]; !isNode {
								// A standalone agent is its own machine.
								if row["id"] == nil || row["command_agent_connected"] != true {
									t.Fatalf("%s lost the standalone agent's connection: %+v", surface, row)
								}
								continue
							}
							seen++
							checkNodeRow(surface, row)
						}
						if seen != 2 {
							t.Fatalf("%s should hold both nodes: %+v", surface, rows)
						}
					}
				})
			}
		})
	}
}

func sameNameInstanceOf(t *testing.T, registry *unifiedresources.ResourceRegistry, nodeID string) string {
	t.Helper()
	for _, node := range registry.Nodes() {
		if node != nil && node.ID() == nodeID {
			return node.Instance()
		}
	}
	t.Fatalf("no node %q", nodeID)
	return ""
}

func TestQueryGuestWithoutItsNodeNeverBorrowsANamesakesAgent(t *testing.T) {
	registry := sameNameRegistry(models.StateSnapshot{
		Nodes: []models.Node{{ID: "site-a-pmx1", Instance: "site-a", Name: "pmx1", Status: "online", LinkedAgentID: "agent-a"}},
		Hosts: []models.Host{{ID: "agent-a", Hostname: "pmx1", LinkedNodeID: "site-a-pmx1", CommandsEnabled: true}},
		VMs:   []models.VM{{ID: "site-b:pmx1:101", Instance: "site-b", Node: "pmx1", VMID: 101, Name: "bravo", Type: "qemu", Status: "running"}},
	})
	for variant, executor := range sameNameQueryExecutors(registry, agentexec.ConnectedAgent{AgentID: "agent-a", Hostname: "pmx1"}) {
		t.Run(variant, func(t *testing.T) {
			row := sameNameSearchRow(t, executor, "bravo", "vm")
			if row["node_command_agent_connected"] != false {
				t.Fatalf("a guest on site-b claimed site-a's agent: %+v", row)
			}
			if routed := executor.resolveTargetForCommandFull(row["id"].(string)); routed.AgentID != "" {
				t.Fatalf("routing disagrees: %+v", routed)
			}
		})
	}
}

func TestQueryGuestOnANodeNameWithoutANodeRecord(t *testing.T) {
	guest := func(instance string, vmid int) models.VM {
		return models.VM{ID: instance + ":pmx1", Instance: instance, Node: "pmx1", VMID: vmid, Name: instance + "-guest", Type: "qemu", Status: "running"}
	}
	agent := agentexec.ConnectedAgent{AgentID: "agent-x", Hostname: "pmx1"}
	for _, tc := range []struct {
		name string
		vms  []models.VM
		want string
	}{
		{"a name one connection uses still routes", []models.VM{guest("site-a", 101)}, "agent-x"},
		{"a name two connections use does not", []models.VM{guest("site-a", 101), guest("site-b", 102)}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			registry := sameNameRegistry(models.StateSnapshot{VMs: tc.vms})
			for variant, executor := range sameNameQueryExecutors(registry, agent) {
				t.Run(variant, func(t *testing.T) {
					row := sameNameSearchRow(t, executor, "site-a-guest", "vm")
					if row["node_command_agent_connected"] != (tc.want != "") {
						t.Fatalf("should report connected=%t: %+v", tc.want != "", row)
					}
					if routed := executor.resolveTargetForCommandFull(row["id"].(string)); routed.AgentID != tc.want {
						t.Fatalf("routed to %q, want %q", routed.AgentID, tc.want)
					}
				})
			}
		})
	}
}

// A command to a VMware guest never goes through an agent on its host, so a host
// name an agent happens to share must not read as a connected parent.
func TestQueryVMwareGuestClaimsNoAgentRoutingNeverUses(t *testing.T) {
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestResources([]unifiedresources.Resource{{
		ID:      "vm-esx-guest",
		Type:    unifiedresources.ResourceTypeVM,
		Name:    "esx-guest",
		Sources: []unifiedresources.DataSource{unifiedresources.SourceVMware},
		VMware:  &unifiedresources.VMwareData{RuntimeHostName: "esx1"},
	}})
	agent := agentexec.ConnectedAgent{AgentID: "agent-x", Hostname: "esx1"}
	executor := sameNameQueryExecutors(registry, agent)["canonical resources"]
	row := sameNameSearchRow(t, executor, "esx-guest", "vm")
	if row["node"] != "esx1" || row["node_command_agent_connected"] != false {
		t.Fatalf("host name read as a command connection: %+v", row)
	}
	if routed := executor.resolveTargetForCommandFull(row["id"].(string)); routed.AgentID != "" {
		t.Fatalf("routing disagrees: %+v", routed)
	}
}

// A shown name is not the name routing uses, so a display name must not read as
// "no agent" on a node commands reach.
func TestQueryDisplayNameDoesNotDisconnectTheNode(t *testing.T) {
	registry := sameNameRegistry(models.StateSnapshot{
		Nodes: []models.Node{{ID: "homelab-pve1", Instance: "homelab", Name: "pve1", DisplayName: "Rack One", Status: "online"}},
		VMs:   []models.VM{{ID: "homelab:pve1:100", Instance: "homelab", Node: "pve1", VMID: 100, Name: "web", Type: "qemu", Status: "running"}},
	})
	for variant, executor := range sameNameQueryExecutors(registry, agentexec.ConnectedAgent{AgentID: "agent-1", Hostname: "pve1"}) {
		t.Run(variant, func(t *testing.T) {
			listed := sameNameListedNodes(t, executor)
			if len(listed) != 1 || listed[0]["name"] != "Rack One" || listed[0]["command_agent_connected"] != true {
				t.Fatalf("display name disconnected the node: %+v", listed)
			}
			if row := sameNameSearchRow(t, executor, "web", "vm"); row["node_command_agent_connected"] != true {
				t.Fatalf("display name disconnected the guest's node: %+v", row)
			}
		})
	}
}

// An agent that shares a guest's name is that guest's own connection. It says
// nothing about the node, and the node's agent says nothing about the guest.
func TestQueryKeepsGuestAndParentConnectionsApart(t *testing.T) {
	snapshot := func() models.StateSnapshot {
		return models.StateSnapshot{
			Nodes: []models.Node{{ID: "homelab-pve1", Instance: "homelab", Name: "pve1", Status: "online"}},
			VMs:   []models.VM{{ID: "homelab:pve1:100", Instance: "homelab", Node: "pve1", VMID: 100, Name: "web", Type: "qemu", Status: "running"}},
		}
	}
	for _, tc := range []struct {
		name          string
		agent         agentexec.ConnectedAgent
		guest, parent bool
	}{
		{"agent on the node", agentexec.ConnectedAgent{AgentID: "node-agent", Hostname: "pve1"}, false, true},
		{"agent in the guest", agentexec.ConnectedAgent{AgentID: "guest-agent", Hostname: "web"}, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for variant, executor := range sameNameQueryExecutors(sameNameRegistry(snapshot()), tc.agent) {
				t.Run(variant, func(t *testing.T) {
					row := sameNameSearchRow(t, executor, "web", "vm")
					if row["command_agent_connected"] != tc.guest || row["node_command_agent_connected"] != tc.parent {
						t.Fatalf("guest and parent connections mixed: %+v", row)
					}
				})
			}
		})
	}
}

func TestQuerySingleInstallationOutputIsUnchanged(t *testing.T) {
	snapshot := models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "homelab-pve2", Instance: "homelab", Name: "pve2", Status: "online"},
			{ID: "homelab-pve1", Instance: "homelab", Name: "pve1", Status: "online"},
		},
		VMs:        []models.VM{{ID: "homelab:pve1:100", Instance: "homelab", Node: "pve1", VMID: 100, Name: "web", Type: "qemu", Status: "running"}},
		Containers: []models.Container{{ID: "homelab:pve2:200", Instance: "homelab", Node: "pve2", VMID: 200, Name: "db", Type: "lxc", Status: "stopped"}},
	}
	strip := func(rows []map[string]any) []map[string]any {
		for _, row := range rows {
			delete(row, "policy")
			delete(row, "ai_safe_summary")
			delete(row, "id")
		}
		return rows
	}
	marshal := func(value any) string {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	for variant, executor := range sameNameQueryExecutors(sameNameRegistry(snapshot), agentexec.ConnectedAgent{AgentID: "agent-1", Hostname: "pve1"}) {
		t.Run(variant, func(t *testing.T) {
			listed := sameNameListedNodes(t, executor)
			for _, row := range listed {
				if _, exists := row["id"]; exists {
					t.Fatalf("a unique node name gained an ID: %+v", row)
				}
			}
			wantListed := `[` +
				`{"command_agent_connected":true,"name":"pve1","status":"online"},` +
				`{"command_agent_connected":false,"name":"pve2","status":"online"}` +
				`]`
			if got := marshal(strip(listed)); got != wantListed {
				t.Fatalf("listed nodes changed:\n got %s\nwant %s", got, wantListed)
			}
			node := sameNameSearchRow(t, executor, "pve1", "node")
			if _, exists := node["id"]; exists {
				t.Fatalf("a unique node name gained an ID: %+v", node)
			}
			wantNode := `[{"command_agent_connected":true,"name":"pve1","status":"online","type":"node"}]`
			if got := marshal(strip([]map[string]any{node})); got != wantNode {
				t.Fatalf("node match changed:\n got %s\nwant %s", got, wantNode)
			}
			for _, guest := range []struct {
				name, kind string
				parent     bool
			}{{"web", "vm", true}, {"db", "system-container", false}} {
				row := sameNameSearchRow(t, executor, guest.name, guest.kind)
				if row["node_command_agent_connected"] != guest.parent || row["command_agent_connected"] != false {
					t.Fatalf("%s row changed: %+v", guest.name, row)
				}
			}
		})
	}

	// Several connections whose node names differ have nothing to disambiguate.
	snapshot.Nodes[0].Instance = "office"
	snapshot.Containers[0].Instance = "office"
	for _, row := range sameNameListedNodes(t, sameNameExecutor(sameNameRegistry(snapshot))) {
		if row["id"] != nil || row["instance"] != nil {
			t.Fatalf("distinct names gained connection noise: %+v", row)
		}
	}
}

func TestQueryOmitsUnobservedCommandConnections(t *testing.T) {
	registry := sameNameRegistry(commandEvidenceSnapshot())
	node := registry.Nodes()[0]
	guest := registry.VMs()[0]
	resource := unifiedresources.Resource{ID: node.ID(), Type: unifiedresources.ResourceTypeAgent, Name: node.Name(), Proxmox: &unifiedresources.ProxmoxData{NodeName: node.NodeName()}}
	standalone := unifiedresources.Resource{ID: "standalone", Type: unifiedresources.ResourceTypeAgent, Name: "standalone", Agent: &unifiedresources.AgentData{Hostname: "standalone"}}

	unobserved := newQueryCommandConnections(registry, nil)
	for label, flag := range map[string]*bool{
		"node":             unobserved.node(node.ID()),
		"guest parent":     unobserved.guestParentNode(guest.ID()),
		"node resource":    unobserved.resource(resource),
		"non-node":         unobserved.resource(standalone),
		"system summary":   canonicalSystemSummaryFromResource(resource, unobserved).CommandAgentConnected,
		"standalone agent": canonicalSystemSummaryFromResource(standalone, unobserved).CommandAgentConnected,
	} {
		if flag != nil {
			t.Fatalf("unobserved %s became a value: %t", label, *flag)
		}
	}

	observed := newQueryCommandConnections(registry, []agentexec.ConnectedAgent{})
	for label, flag := range map[string]*bool{
		"node":          observed.node(node.ID()),
		"guest parent":  observed.guestParentNode(guest.ID()),
		"node resource": observed.resource(resource),
		"non-node":      observed.resource(standalone),
	} {
		if flag == nil || *flag {
			t.Fatalf("observed %s with no agents should read false: %v", label, flag)
		}
	}
}
