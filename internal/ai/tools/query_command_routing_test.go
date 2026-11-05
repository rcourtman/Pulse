package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/stretchr/testify/mock"
)

func routingFixtureHost(id, hostname, displayName string) models.Host {
	return models.Host{ID: id, Hostname: hostname, DisplayName: displayName, MachineID: "machine-" + id, Status: "online", LastSeen: time.Now(), CommandsEnabled: true}
}

func routingFixtureDocker(id, hostname, displayName string) models.DockerHost {
	return models.DockerHost{ID: id, AgentID: id, Hostname: hostname, DisplayName: displayName, MachineID: "machine-" + id, Status: "online", LastSeen: time.Now()}
}

func routingFixtureNIC(name string, addresses ...string) []models.HostNetworkInterface {
	return []models.HostNetworkInterface{{Name: name, Addresses: addresses}}
}

// routingFixtureDockerOfAgent is a Docker host with no agent record merged into
// it, the shape a Docker-only agent leaves: its report names the agent, and the
// host's ID says nothing about it.
func routingFixtureDockerOfAgent(id, agentID, hostname string) models.DockerHost {
	host := routingFixtureDocker(id, hostname, "")
	host.AgentID = agentID
	return host
}

// routingFixtureDockerResourceID finds a Docker host by the ID it reported, since
// rows carry the canonical ID.
func routingFixtureDockerResourceID(t *testing.T, registry *unifiedresources.ResourceRegistry, sourceID string) string {
	t.Helper()
	for _, host := range registry.DockerHosts() {
		if host != nil && host.HostSourceID() == sourceID {
			return host.ID()
		}
	}
	t.Fatalf("no Docker host reported as %q", sourceID)
	return ""
}

func routingFixtureAgentResourceID(t *testing.T, registry *unifiedresources.ResourceRegistry, agentID string) string {
	t.Helper()
	for _, host := range registry.Hosts() {
		if host != nil && host.AgentID() == agentID {
			return host.ID()
		}
	}
	t.Fatalf("no record for agent %q", agentID)
	return ""
}

func routingFixtureDockerTopology(t *testing.T, executor *PulseToolExecutor) (map[string]any, []map[string]any) {
	t.Helper()
	response := sameNameQuery(t, executor, map[string]interface{}{"action": "topology", "include": "app-containers"})
	summary, ok := response["summary"].(map[string]any)
	if !ok {
		t.Fatalf("topology has no summary: %+v", response)
	}
	raw, ok := response["docker"].(map[string]any)["hosts"].([]any)
	if !ok {
		t.Fatalf("topology has no Docker hosts: %+v", response)
	}
	rows := make([]map[string]any, 0, len(raw))
	for _, row := range raw {
		rows = append(rows, row.(map[string]any))
	}
	return summary, rows
}

// A flag holds only if it agrees with where a command for that row would go.
func TestQueryHostFlagsFollowCommandRouting(t *testing.T) {
	dockerInContainer := func() models.StateSnapshot {
		return models.StateSnapshot{
			Nodes:       []models.Node{{ID: "lab-pve1", Instance: "lab", Name: "pve1", Status: "online"}},
			Containers:  []models.Container{{ID: "lab:pve1:200", Instance: "lab", Node: "pve1", VMID: 200, Name: "dock", Type: "lxc", Status: "running"}},
			DockerHosts: []models.DockerHost{routingFixtureDocker("dock-a", "dock", "")},
		}
	}
	dockerSharingContainerAddress := func() models.StateSnapshot {
		snapshot := dockerInContainer()
		snapshot.Containers[0].IPAddresses = []string{"10.0.0.5"}
		snapshot.DockerHosts[0].NetworkInterfaces = routingFixtureNIC("eth0", "10.0.0.5/24")
		return snapshot
	}
	for _, tc := range []struct {
		name     string
		snapshot models.StateSnapshot
		agents   []agentexec.ConnectedAgent
		hosts    map[string]string // agent record -> agent its commands reach
		docker   map[string]string // reported Docker host -> agent its commands reach
	}{
		{
			name: "two standalone hosts share a hostname",
			snapshot: models.StateSnapshot{Hosts: []models.Host{
				routingFixtureHost("agent-a", "box", ""),
				routingFixtureHost("agent-b", "box", ""),
			}},
			agents: []agentexec.ConnectedAgent{{AgentID: "agent-a", Hostname: "box"}},
			hosts:  map[string]string{"agent-a": "agent-a", "agent-b": ""},
		},
		{
			name:     "a display name another agent has as its hostname",
			snapshot: models.StateSnapshot{Hosts: []models.Host{routingFixtureHost("agent-a", "box", "shared")}},
			agents:   []agentexec.ConnectedAgent{{AgentID: "agent-x", Hostname: "shared"}},
			hosts:    map[string]string{"agent-a": ""},
		},
		{
			name:     "a host whose agent connects under a longer name",
			snapshot: models.StateSnapshot{Hosts: []models.Host{routingFixtureHost("agent-a", "box", "")}},
			agents:   []agentexec.ConnectedAgent{{AgentID: "agent-a", Hostname: "box.lan"}},
			hosts:    map[string]string{"agent-a": "agent-a"},
		},
		{
			name: "two Docker hosts share a hostname",
			snapshot: models.StateSnapshot{
				Hosts:       []models.Host{routingFixtureHost("agent-a", "dock", ""), routingFixtureHost("agent-b", "dock", "")},
				DockerHosts: []models.DockerHost{routingFixtureDocker("agent-a", "dock", ""), routingFixtureDocker("agent-b", "dock", "")},
			},
			agents: []agentexec.ConnectedAgent{{AgentID: "agent-a", Hostname: "dock"}},
			docker: map[string]string{"agent-a": "agent-a", "agent-b": ""},
		},
		{
			name: "a Docker host shown under another agent's hostname",
			snapshot: models.StateSnapshot{
				DockerHosts: []models.DockerHost{routingFixtureDocker("dock-a", "dock", "shared")},
			},
			agents: []agentexec.ConnectedAgent{{AgentID: "agent-x", Hostname: "shared"}},
			docker: map[string]string{"dock-a": ""},
		},
		{
			name: "two Docker hosts with no agent records share a hostname",
			snapshot: models.StateSnapshot{DockerHosts: []models.DockerHost{
				routingFixtureDockerOfAgent("dh-a", "agent-a", "dock"),
				routingFixtureDockerOfAgent("dh-b", "agent-b", "dock"),
			}},
			agents: []agentexec.ConnectedAgent{{AgentID: "agent-a", Hostname: "dock"}},
			docker: map[string]string{"dh-a": "agent-a", "dh-b": ""},
		},
		{
			name:     "a Docker host with no session, beside a machine with its hostname",
			snapshot: models.StateSnapshot{DockerHosts: []models.DockerHost{routingFixtureDockerOfAgent("dh-b", "agent-b", "dock")}},
			agents:   []agentexec.ConnectedAgent{{AgentID: "agent-a", Hostname: "dock"}},
			docker:   map[string]string{"dh-b": ""},
		},
		{
			name:     "a Docker host whose own agent connects under another name",
			snapshot: models.StateSnapshot{DockerHosts: []models.DockerHost{routingFixtureDockerOfAgent("dh-a", "agent-a", "dock")}},
			agents:   []agentexec.ConnectedAgent{{AgentID: "agent-a", Hostname: "renamed"}},
			docker:   map[string]string{"dh-a": "agent-a"},
		},
		{
			name:     "a Docker host named like a container, with only its own agent connected",
			snapshot: dockerInContainer(),
			agents:   []agentexec.ConnectedAgent{{AgentID: "dock-a", Hostname: "dock"}},
			docker:   map[string]string{"dock-a": "dock-a"},
		},
		{
			name:     "a Docker host named like a container, with only another machine of its name connected",
			snapshot: dockerInContainer(),
			agents:   []agentexec.ConnectedAgent{{AgentID: "other", Hostname: "dock"}},
			docker:   map[string]string{"dock-a": ""},
		},
		{
			name:     "a Docker host named like a container, with its node's agent and its own connected",
			snapshot: dockerInContainer(),
			agents:   []agentexec.ConnectedAgent{{AgentID: "dock-a", Hostname: "dock"}, {AgentID: "node-agent", Hostname: "pve1"}},
			docker:   map[string]string{"dock-a": "dock-a"},
		},
		{
			name:     "a Docker host sharing a container's address, with its node's agent and its own connected",
			snapshot: dockerSharingContainerAddress(),
			agents:   []agentexec.ConnectedAgent{{AgentID: "dock-a", Hostname: "dock"}, {AgentID: "node-agent", Hostname: "pve1"}},
			docker:   map[string]string{"dock-a": "dock-a"},
		},
		{
			name: "a host and Docker host named like a VM, with only their own agent connected",
			snapshot: models.StateSnapshot{
				Nodes:       []models.Node{{ID: "lab-pve1", Instance: "lab", Name: "pve1", Status: "online"}},
				VMs:         []models.VM{{ID: "lab:pve1:100", Instance: "lab", Node: "pve1", VMID: 100, Name: "dock", Type: "qemu", Status: "running"}},
				Hosts:       []models.Host{routingFixtureHost("dock-a", "dock", "")},
				DockerHosts: []models.DockerHost{routingFixtureDocker("dock-a", "dock", "")},
			},
			agents: []agentexec.ConnectedAgent{{AgentID: "dock-a", Hostname: "dock"}},
			hosts:  map[string]string{"dock-a": "dock-a"},
			docker: map[string]string{"dock-a": "dock-a"},
		},
		{
			name:     "a Docker host named like a container, with only its node's agent connected",
			snapshot: dockerInContainer(),
			agents:   []agentexec.ConnectedAgent{{AgentID: "node-agent", Hostname: "pve1"}},
			docker:   map[string]string{"dock-a": ""},
		},
		{
			name:     "a Docker host sharing a container's address, with only its node's agent connected",
			snapshot: dockerSharingContainerAddress(),
			agents:   []agentexec.ConnectedAgent{{AgentID: "node-agent", Hostname: "pve1"}},
			docker:   map[string]string{"dock-a": "node-agent"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			registry := sameNameRegistry(tc.snapshot)
			wantByID := make(map[string]string, len(tc.hosts)+len(tc.docker))
			wantDockerByID := make(map[string]string, len(tc.docker))
			for agentID, want := range tc.hosts {
				wantByID[routingFixtureAgentResourceID(t, registry, agentID)] = want
			}
			wantDockerConnected := 0
			for sourceID, want := range tc.docker {
				id := routingFixtureDockerResourceID(t, registry, sourceID)
				wantByID[id], wantDockerByID[id] = want, want
				if want != "" {
					wantDockerConnected++
				}
			}
			for variant, executor := range sameNameQueryExecutors(registry, tc.agents...) {
				t.Run(variant, func(t *testing.T) {
					checkRows := func(surface string, rows []map[string]any, want map[string]string) {
						t.Helper()
						seen := make(map[string]bool, len(want))
						for _, row := range rows {
							id, _ := row["id"].(string)
							agent, known := want[id]
							if !known {
								continue
							}
							seen[id] = true
							if row["command_agent_connected"] != (agent != "") {
								t.Fatalf("%s row for %s should report connected=%t: %+v", surface, id, agent != "", row)
							}
							if routed := executor.resolveTargetForCommandFull(id); routed.AgentID != agent {
								t.Fatalf("%s routed to %q, want %q", id, routed.AgentID, agent)
							}
						}
						if len(seen) != len(want) {
							t.Fatalf("%s lost a row: saw %v, want %v: %+v", surface, seen, want, rows)
						}
					}

					if variant == "canonical resources" {
						checkRows("systems", sameNameRows(t, sameNameQuery(t, executor, map[string]interface{}{"action": "list", "type": "systems"}), "systems"), wantByID)
						var matches []map[string]any
						for _, query := range []string{"box", "dock"} {
							matches = append(matches, sameNameRows(t, sameNameQuery(t, executor, map[string]interface{}{"action": "search", "query": query, "type": "agent"}), "matches")...)
						}
						checkRows("agent search", matches, wantByID)
					}
					if tc.docker == nil {
						return
					}
					listed := sameNameRows(t, sameNameQuery(t, executor, map[string]interface{}{"action": "list", "type": "docker-hosts"}), "docker_hosts")
					if len(listed) != len(wantDockerByID) {
						t.Fatalf("expected a row per Docker host: %+v", listed)
					}
					checkRows("docker-hosts", listed, wantDockerByID)

					summary, rows := routingFixtureDockerTopology(t, executor)
					if summary["docker_hosts_with_command_agents"] != float64(wantDockerConnected) {
						t.Fatalf("topology counted %v connected Docker hosts, want %d", summary["docker_hosts_with_command_agents"], wantDockerConnected)
					}
					if len(rows) != len(wantDockerByID) {
						t.Fatalf("expected a topology row per Docker host: %+v", rows)
					}
					for _, row := range rows {
						if row["can_execute"] != row["command_agent_connected"] {
							t.Fatalf("can_execute left the flag: %+v", row)
						}
					}
					if len(rows) > 1 {
						// Only the ID can say which host a repeated hostname means.
						checkRows("topology", rows, wantDockerByID)
						return
					}
					for id, agent := range wantDockerByID {
						if rows[0]["command_agent_connected"] != (agent != "") {
							t.Fatalf("topology row for %s should report connected=%t: %+v", id, agent != "", rows[0])
						}
						// The row shows no ID unless its hostname cannot select it, so
						// whichever it shows has to reach what its flag reports.
						reference, _ := rows[0]["id"].(string)
						if reference == "" {
							reference, _ = rows[0]["hostname"].(string)
						}
						if routed := executor.resolveTargetForCommandFull(reference); routed.AgentID != agent {
							t.Fatalf("topology row for %s shows %q, which routes to %+v, want %q: %+v", id, reference, routed, agent, rows[0])
						}
					}
				})
			}
		})
	}
}

// A container's command goes where its Docker host's would, so it cannot reach
// the other machine either.
func TestContainerCommandFollowsItsDockerHostsOwnAgent(t *testing.T) {
	withContainer := func(id, agentID string) models.DockerHost {
		host := routingFixtureDockerOfAgent(id, agentID, "dock")
		host.Containers = []models.DockerContainer{{ID: "ctr-" + id, Name: "web-" + id, State: "running", Status: "Up 1 hour"}}
		return host
	}
	registry := sameNameRegistry(models.StateSnapshot{DockerHosts: []models.DockerHost{
		withContainer("dh-a", "agent-a"),
		withContainer("dh-b", "agent-b"),
	}})
	agents := []agentexec.ConnectedAgent{{AgentID: "agent-a", Hostname: "dock"}}

	want := map[string]string{"web-dh-a": "agent-a", "web-dh-b": ""}
	seen := 0
	for _, container := range registry.DockerContainers() {
		agent, known := want[container.Name()]
		if !known {
			t.Fatalf("unexpected container %q", container.Name())
		}
		seen++
		routed := routeCommandTarget(registry, agents, container.ID())
		if routed.AgentID != agent || routed.ResolvedKind != "app-container" {
			t.Fatalf("%s routed to %q as %q, want %q", container.Name(), routed.AgentID, routed.ResolvedKind, agent)
		}
	}
	if seen != len(want) {
		t.Fatalf("saw %d containers, want %d", seen, len(want))
	}
}

// Pulse finds these hosts by probing a guest through its node, so their agent ID
// is synthetic and no session can hold it. The guest's node reaches them.
func TestLXCDiscoveredDockerHostRoutesThroughItsGuest(t *testing.T) {
	synthetic := "proxmox-lxc-docker:lab:pve1:200"
	registry := sameNameRegistry(models.StateSnapshot{
		Nodes:       []models.Node{{ID: "lab-pve1", Instance: "lab", Name: "pve1", Status: "online"}},
		Containers:  []models.Container{{ID: "lab:pve1:200", Instance: "lab", Node: "pve1", VMID: 200, Name: "dock", Type: "lxc", Status: "running"}},
		DockerHosts: []models.DockerHost{routingFixtureDockerOfAgent(synthetic, synthetic, "dock")},
	})
	id := routingFixtureDockerResourceID(t, registry, synthetic)

	routed := routeCommandTarget(registry, []agentexec.ConnectedAgent{{AgentID: "node-agent", Hostname: "pve1"}}, id)
	if routed.AgentID != "node-agent" || routed.Transport != "pct_exec" || routed.TargetType != "container" || routed.TargetID != "200" {
		t.Fatalf("expected the node's agent to reach the guest: %+v", routed)
	}
	routed = routeCommandTarget(registry, []agentexec.ConnectedAgent{{AgentID: "someone-else", Hostname: "dock"}}, id)
	if routed.AgentID != "" {
		t.Fatalf("a namesake's agent cannot stand in for the guest's node: %+v", routed)
	}
}

// An agent ID that no longer has a guest behind it names nothing a session could
// hold, so the host's hostname still cannot pick a machine.
func TestLXCDiscoveredDockerHostWithoutItsGuestRoutesNowhere(t *testing.T) {
	synthetic := "proxmox-lxc-docker:lab:pve1:200"
	registry := sameNameRegistry(models.StateSnapshot{DockerHosts: []models.DockerHost{routingFixtureDockerOfAgent(synthetic, synthetic, "dock")}})
	id := routingFixtureDockerResourceID(t, registry, synthetic)

	routed := routeCommandTarget(registry, []agentexec.ConnectedAgent{{AgentID: "someone-else", Hostname: "dock"}}, id)
	if routed.AgentID != "" {
		t.Fatalf("expected no route: %+v", routed)
	}
}

// A connected agent running inside a guest is the guest's own connection. A
// command for the guest row still goes through its node, so the two flags answer
// separately: node_command_agent_connected is where that command goes, and
// command_agent_connected is whether the agent's own row reaches the guest. Only
// an agent folded into the guest also takes the row's command, while the node has
// no connected agent (TestCommandsReachAnAgentInsideAGuestOnlyByIdentity).
func TestQueryGuestOwnFlagFollowsItsAgentNotItsName(t *testing.T) {
	vm := func(instance, name string) models.VM {
		return models.VM{ID: instance + ":pve1:100", Instance: instance, Node: "pve1", VMID: 100, Name: name, Type: "qemu", Status: "running"}
	}
	node := func(instance string) models.Node {
		return models.Node{ID: instance + "-pve1", Instance: instance, Name: "pve1", Status: "online"}
	}
	linked := func(id, hostname, guestID string) models.Host {
		host := routingFixtureHost(id, hostname, "")
		host.LinkedVMID = guestID
		return host
	}
	webAgent := agentexec.ConnectedAgent{AgentID: "web-agent", Hostname: "web"}
	for _, tc := range []struct {
		name     string
		snapshot models.StateSnapshot
		agents   []agentexec.ConnectedAgent
		own      map[string]bool   // guest ID -> its own flag
		parent   map[string]string // guest ID -> agent a command for the guest goes through
	}{
		{
			name: "an agent linked to the guest",
			snapshot: models.StateSnapshot{
				Nodes: []models.Node{node("a")},
				VMs:   []models.VM{vm("a", "web")},
				Hosts: []models.Host{linked("web-agent", "web", "a:pve1:100")},
			},
			agents: []agentexec.ConnectedAgent{webAgent},
			own:    map[string]bool{"a:pve1:100": true},
		},
		{
			name: "an agent linked to one of two guests with one name",
			snapshot: models.StateSnapshot{
				Nodes: []models.Node{node("a"), node("b")},
				VMs:   []models.VM{vm("a", "web"), vm("b", "web")},
				Hosts: []models.Host{linked("web-agent", "web", "a:pve1:100")},
			},
			agents: []agentexec.ConnectedAgent{webAgent},
			own:    map[string]bool{"a:pve1:100": true, "b:pve1:100": false},
		},
		{
			name: "an unlinked agent named like two guests",
			snapshot: models.StateSnapshot{
				Nodes: []models.Node{node("a"), node("b")},
				VMs:   []models.VM{vm("a", "web"), vm("b", "web")},
				Hosts: []models.Host{routingFixtureHost("web-agent", "web", "")},
			},
			agents: []agentexec.ConnectedAgent{webAgent},
			own:    map[string]bool{"a:pve1:100": false, "b:pve1:100": false},
		},
		{
			name: "an agent linked to a different guest",
			snapshot: models.StateSnapshot{
				Nodes: []models.Node{node("a"), node("b")},
				VMs:   []models.VM{vm("a", "web"), vm("b", "other")},
				Hosts: []models.Host{linked("web-agent", "web", "b:pve1:100")},
			},
			agents: []agentexec.ConnectedAgent{webAgent},
			own:    map[string]bool{"a:pve1:100": false, "b:pve1:100": true},
		},
		{
			name: "a linked agent offline while another machine has the name",
			snapshot: models.StateSnapshot{
				Nodes: []models.Node{node("a")},
				VMs:   []models.VM{vm("a", "web")},
				Hosts: []models.Host{linked("web-agent", "web", "a:pve1:100")},
			},
			agents: []agentexec.ConnectedAgent{{AgentID: "other-machine", Hostname: "web"}},
			own:    map[string]bool{"a:pve1:100": false},
		},
		{
			name: "a guest named like its node, with the node's agent connected",
			snapshot: models.StateSnapshot{
				Nodes: []models.Node{node("a")},
				VMs:   []models.VM{vm("a", "pve1")},
			},
			agents: []agentexec.ConnectedAgent{{AgentID: "node-agent", Hostname: "pve1"}},
			own:    map[string]bool{"a:pve1:100": false},
			parent: map[string]string{"a:pve1:100": "node-agent"},
		},
		{
			name: "only the node's agent connected",
			snapshot: models.StateSnapshot{
				Nodes: []models.Node{node("a")},
				VMs:   []models.VM{vm("a", "web")},
			},
			agents: []agentexec.ConnectedAgent{{AgentID: "node-agent", Hostname: "pve1"}},
			own:    map[string]bool{"a:pve1:100": false},
			parent: map[string]string{"a:pve1:100": "node-agent"},
		},
		{
			name: "the only guest with the agent's name, before the agent is linked",
			snapshot: models.StateSnapshot{
				Nodes: []models.Node{node("a")},
				VMs:   []models.VM{vm("a", "web")},
				Hosts: []models.Host{routingFixtureHost("web-agent", "web", "")},
			},
			agents: []agentexec.ConnectedAgent{webAgent},
			own:    map[string]bool{"a:pve1:100": true},
		},
		{
			name: "the only guest with the agent's name, before the agent reports",
			snapshot: models.StateSnapshot{
				Nodes: []models.Node{node("a")},
				VMs:   []models.VM{vm("a", "web")},
			},
			agents: []agentexec.ConnectedAgent{webAgent},
			own:    map[string]bool{"a:pve1:100": true},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			registry := sameNameRegistry(tc.snapshot)
			idBySource := make(map[string]string, len(tc.own))
			for _, guest := range registry.VMs() {
				idBySource[guest.SourceID()] = guest.ID()
			}
			for variant, executor := range sameNameQueryExecutors(registry, tc.agents...) {
				t.Run(variant, func(t *testing.T) {
					rows := sameNameRows(t, sameNameQuery(t, executor, map[string]interface{}{"action": "search", "query": "e", "type": "vm"}), "matches")
					rows = append(rows, sameNameRows(t, sameNameQuery(t, executor, map[string]interface{}{"action": "search", "query": "pve1", "type": "vm"}), "matches")...)
					rowByID := make(map[string]map[string]any, len(rows))
					for _, row := range rows {
						rowByID[row["id"].(string)] = row
					}
					for source, want := range tc.own {
						id, known := idBySource[source]
						if !known {
							t.Fatalf("no guest %q", source)
						}
						row, found := rowByID[id]
						if !found {
							t.Fatalf("no search row for %q: %+v", source, rows)
						}
						if row["command_agent_connected"] != want {
							t.Fatalf("%s should report its own connection=%t: %+v", source, want, row)
						}
						// The row's own ID goes through the node, and a node's agent
						// is never the guest's own connection.
						wantParent := tc.parent[source]
						routed := executor.resolveTargetForCommandFull(id)
						if routed.AgentID != wantParent || row["node_command_agent_connected"] != (wantParent != "") {
							t.Fatalf("%s parent should be %q, routed %+v: %+v", source, wantParent, routed, row)
						}
					}
					// Where the flag is true, the agent's own row is what reaches it.
					for _, host := range registry.Hosts() {
						if host == nil || host.LinkedVMID() == "" {
							continue
						}
						_, connected := connectedAgentByID(tc.agents, host.AgentID())
						guestID := idBySource[host.LinkedVMID()]
						if rowByID[guestID]["command_agent_connected"] != connected {
							t.Fatalf("guest %s disagrees with its linked agent %s: %+v", guestID, host.AgentID(), rowByID[guestID])
						}
						if routed := executor.resolveTargetForCommandFull(host.ID()); (routed.AgentID == host.AgentID()) != connected {
							t.Fatalf("agent row %s routed to %+v", host.ID(), routed)
						}
					}
				})
			}
		})
	}
}

// Containers link through their own field, and share the guest rule.
func TestQueryContainerOwnFlagFollowsItsLink(t *testing.T) {
	host := routingFixtureHost("ct-agent", "db", "")
	host.LinkedContainerID = "a:pve1:200"
	registry := sameNameRegistry(models.StateSnapshot{
		Nodes: []models.Node{{ID: "a-pve1", Instance: "a", Name: "pve1", Status: "online"}, {ID: "b-pve1", Instance: "b", Name: "pve1", Status: "online"}},
		Containers: []models.Container{
			{ID: "a:pve1:200", Instance: "a", Node: "pve1", VMID: 200, Name: "db", Type: "lxc", Status: "running"},
			{ID: "b:pve1:200", Instance: "b", Node: "pve1", VMID: 200, Name: "db", Type: "lxc", Status: "running"},
		},
		Hosts: []models.Host{host},
	})
	want := map[string]bool{}
	for _, container := range registry.Containers() {
		want[container.ID()] = container.Instance() == "a"
	}
	for variant, executor := range sameNameQueryExecutors(registry, agentexec.ConnectedAgent{AgentID: "ct-agent", Hostname: "db"}) {
		t.Run(variant, func(t *testing.T) {
			rows := sameNameRows(t, sameNameQuery(t, executor, map[string]interface{}{"action": "search", "query": "db", "type": "system-container"}), "matches")
			if len(rows) != 2 {
				t.Fatalf("expected a row per container: %+v", rows)
			}
			for _, row := range rows {
				if row["command_agent_connected"] != want[row["id"].(string)] {
					t.Fatalf("%v should read %t: %+v", row["id"], want[row["id"].(string)], row)
				}
			}
		})
	}
}

// A manual link folds an agent's record into the guest, so no host row is left to
// link from and the agent's hostname need not be the guest's name.
func TestQueryGuestOwnFlagFollowsAFoldedDockerReport(t *testing.T) {
	snapshot := models.StateSnapshot{
		Nodes:       []models.Node{{ID: "a-pve1", Instance: "a", Name: "pve1", Status: "online"}},
		VMs:         []models.VM{{ID: "a:pve1:100", Instance: "a", Node: "pve1", VMID: 100, Name: "web", Type: "qemu", Status: "running"}},
		DockerHosts: []models.DockerHost{routingFixtureDocker("inside", "inner-host", "")},
	}
	registry := unifiedresources.NewRegistryWithManualLinks([]unifiedresources.ResourceLink{foldDockerInto("inside")(sameNameRegistry(snapshot))})
	registry.IngestSnapshot(snapshot)
	agents := []agentexec.ConnectedAgent{{AgentID: "inside", Hostname: "inner-host"}}
	for variant, executor := range sameNameQueryExecutors(registry, agents...) {
		t.Run(variant, func(t *testing.T) {
			row := sameNameSearchRow(t, executor, "web", "vm")
			if row["command_agent_connected"] != true || row["node_command_agent_connected"] != false {
				t.Fatalf("should report its own connection and no parent: %+v", row)
			}
		})
	}
}

func TestQueryGuestOwnFlagFollowsAnAgentFoldedIntoIt(t *testing.T) {
	snapshot := models.StateSnapshot{
		Nodes: []models.Node{{ID: "a-pve1", Instance: "a", Name: "pve1", Status: "online"}},
		VMs:   []models.VM{{ID: "a:pve1:100", Instance: "a", Node: "pve1", VMID: 100, Name: "web", Type: "qemu", Status: "running"}},
		Hosts: []models.Host{routingFixtureHost("inside", "inner-host", "")},
	}
	plain := sameNameRegistry(snapshot)
	guestID, agentRow := plain.VMs()[0].ID(), routingFixtureAgentResourceID(t, plain, "inside")
	registry := unifiedresources.NewRegistryWithManualLinks([]unifiedresources.ResourceLink{{ResourceA: guestID, ResourceB: agentRow, PrimaryID: guestID}})
	registry.IngestSnapshot(snapshot)
	if _, listed := registry.Get(agentRow); listed {
		t.Fatalf("the agent is still listed beside the guest it is linked into")
	}
	for _, tc := range []struct {
		name   string
		agents []agentexec.ConnectedAgent
		want   bool
	}{
		{"connected", []agentexec.ConnectedAgent{{AgentID: "inside", Hostname: "inner-host"}}, true},
		{"offline while another machine has the guest's name", []agentexec.ConnectedAgent{{AgentID: "other", Hostname: "web"}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for variant, executor := range sameNameQueryExecutors(registry, tc.agents...) {
				t.Run(variant, func(t *testing.T) {
					row := sameNameSearchRow(t, executor, "web", "vm")
					if row["command_agent_connected"] != tc.want || row["node_command_agent_connected"] != false {
						t.Fatalf("should report its own connection=%t and no parent: %+v", tc.want, row)
					}
				})
			}
		})
	}
}

// A Docker host's own agent, or an agent folded into a guest, takes a command when
// the guest's node has no connected agent. That agent is on the same machine as the
// target, so the command runs where it would have. A connected node agent keeps a
// folded guest's command, and no name can select the agent inside: automatic links
// come from names, so they do not count, and a Docker host placed in a guest by its
// name and an address does not inherit an agent folded into that guest.
//
// A Docker host's own agent goes before the node unless an identity puts the host in
// the guest. A matching name alone does not put it there at all: it can be another
// machine, so no command enters that guest, whichever agents are connected.
func TestCommandsReachAnAgentInsideAGuestOnlyByIdentity(t *testing.T) {
	node := models.Node{ID: "lab-pve1", Instance: "lab", Name: "pve1", Status: "online"}
	lxc := func(name string) models.Container {
		return models.Container{ID: "lab:pve1:200", Instance: "lab", Node: "pve1", VMID: 200, Name: name, Type: "lxc", Status: "running", IPAddresses: []string{"10.0.0.5"}}
	}
	vm := func(name string) models.VM {
		return models.VM{ID: "lab:pve1:100", Instance: "lab", Node: "pve1", VMID: 100, Name: name, Type: "qemu", Status: "running", IPAddresses: []string{"10.0.0.5"}}
	}
	// These hosts report the address their same-named guest has, so they show the
	// same machine. The elsewhere fixtures report another.
	dockerWithContainer := func(id string) models.DockerHost {
		host := routingFixtureDocker(id, "dock", "")
		host.Containers = []models.DockerContainer{{ID: "app-1", Name: "app", State: "running"}}
		host.NetworkInterfaces = routingFixtureNIC("eth0", "10.0.0.5/24")
		return host
	}
	dockerElsewhere := dockerWithContainer("dock-a")
	dockerElsewhere.NetworkInterfaces = routingFixtureNIC("eth0", "192.168.7.9/24")
	elsewhere := models.StateSnapshot{Nodes: []models.Node{node}, Containers: []models.Container{lxc("dock")}, Hosts: []models.Host{routingFixtureHost("dock-a", "dock", "")}, DockerHosts: []models.DockerHost{dockerElsewhere}}
	elsewhereFromVM := models.StateSnapshot{Nodes: []models.Node{node}, VMs: []models.VM{vm("dock")}, Hosts: []models.Host{routingFixtureHost("dock-a", "dock", "")}, DockerHosts: []models.DockerHost{dockerElsewhere}}
	unknownGuest := lxc("dock")
	unknownGuest.IPAddresses = nil
	unknownAddresses := models.StateSnapshot{Nodes: []models.Node{node}, Containers: []models.Container{unknownGuest}, DockerHosts: []models.DockerHost{dockerWithContainer("dock-a")}}
	bridgeOnly := dockerWithContainer("dock-a")
	bridgeOnly.NetworkInterfaces = routingFixtureNIC("docker0", "10.0.0.5/16")
	onlyABridgeShared := models.StateSnapshot{Nodes: []models.Node{node}, Containers: []models.Container{lxc("dock")}, DockerHosts: []models.DockerHost{bridgeOnly}}
	linkLocalGuest := lxc("dock")
	linkLocalGuest.IPAddresses = []string{"fe80::1"}
	linkLocalDocker := dockerWithContainer("dock-a")
	linkLocalDocker.NetworkInterfaces = routingFixtureNIC("eth0", "fe80::1/64")
	onlyLinkLocalShared := models.StateSnapshot{Nodes: []models.Node{node}, Containers: []models.Container{linkLocalGuest}, DockerHosts: []models.DockerHost{linkLocalDocker}}
	guestListsInterfaces := lxc("dock")
	guestListsInterfaces.IPAddresses = nil
	guestListsInterfaces.NetworkInterfaces = []models.GuestNetworkInterface{{Name: "eth0", Addresses: []string{"10.0.0.5"}}}
	sharedOnGuestInterface := models.StateSnapshot{Nodes: []models.Node{node}, Containers: []models.Container{guestListsInterfaces}, DockerHosts: []models.DockerHost{dockerWithContainer("dock-a")}}
	guestBridge := guestListsInterfaces
	guestBridge.NetworkInterfaces = []models.GuestNetworkInterface{{Name: "docker0", Addresses: []string{"10.0.0.5"}}}
	guestBridge.IPAddresses = []string{"10.0.0.5"}
	sharedOnGuestBridge := models.StateSnapshot{Nodes: []models.Node{node}, Containers: []models.Container{guestBridge}, DockerHosts: []models.DockerHost{dockerWithContainer("dock-a")}}
	moduleWithNoInterfaces := dockerWithContainer("dock-a")
	moduleWithNoInterfaces.NetworkInterfaces = nil
	recordWithAddress := routingFixtureHost("dock-a", "dock", "")
	recordWithAddress.NetworkInterfaces = routingFixtureNIC("eth0", "10.0.0.5/24")
	sharedOnAgentRecord := models.StateSnapshot{Nodes: []models.Node{node}, Containers: []models.Container{lxc("dock")}, Hosts: []models.Host{recordWithAddress}, DockerHosts: []models.DockerHost{moduleWithNoInterfaces}}
	recordLinkedByName := routingFixtureHost("dock-a", "dock", "")
	recordLinkedByName.LinkedContainerID = "lab:pve1:200"
	linkedByNameElsewhere := models.StateSnapshot{Nodes: []models.Node{node}, Containers: []models.Container{lxc("dock")}, Hosts: []models.Host{recordLinkedByName}, DockerHosts: []models.DockerHost{dockerElsewhere}}
	sharedAddressLinked := func(link func(*models.Host)) models.StateSnapshot {
		record := routingFixtureHost("dock-a", "dock", "")
		link(&record)
		return models.StateSnapshot{Nodes: []models.Node{node}, Containers: []models.Container{lxc("dock")}, Hosts: []models.Host{record}, DockerHosts: []models.DockerHost{dockerWithContainer("dock-a")}}
	}
	// One guest seen through two connections, which an operator's link merges.
	mergedGuest := func(fill func(*models.StateSnapshot)) models.StateSnapshot {
		second := lxc("dock")
		second.ID, second.Instance = "site:pve1:200", "site"
		snapshot := models.StateSnapshot{Nodes: []models.Node{node}, Containers: []models.Container{lxc("dock"), second}}
		fill(&snapshot)
		return snapshot
	}
	linkedToAKnownGuest := sharedAddressLinked(func(host *models.Host) { host.LinkedContainerID = "lab:pve1:201" })
	linkedToAKnownGuest.Containers = append(linkedToAKnownGuest.Containers, models.Container{ID: "lab:pve1:201", Instance: "lab", Node: "pve1", VMID: 201, Name: "web", Type: "lxc", Status: "running"})
	sharedAddressLinkedVM := func(link func(*models.Host)) models.StateSnapshot {
		snapshot := sharedAddressLinked(link)
		snapshot.Containers, snapshot.VMs = nil, []models.VM{vm("dock")}
		return snapshot
	}
	inLXC := models.StateSnapshot{Nodes: []models.Node{node}, Containers: []models.Container{lxc("dock")}, Hosts: []models.Host{routingFixtureHost("dock-a", "dock", "")}, DockerHosts: []models.DockerHost{dockerWithContainer("dock-a")}}
	inVM := models.StateSnapshot{Nodes: []models.Node{node}, VMs: []models.VM{vm("dock")}, Hosts: []models.Host{routingFixtureHost("dock-a", "dock", "")}, DockerHosts: []models.DockerHost{dockerWithContainer("dock-a")}}
	noHostRecord := models.StateSnapshot{Nodes: []models.Node{node}, Containers: []models.Container{lxc("dock")}, DockerHosts: []models.DockerHost{dockerWithContainer("dock-a")}}
	probedThroughNode := routingFixtureDocker("proxmox-lxc-docker:lab:pve1:200", "dock", "")
	probed := models.StateSnapshot{Nodes: []models.Node{node}, Containers: []models.Container{lxc("dock")}, DockerHosts: []models.DockerHost{probedThroughNode}}
	foldHost := routingFixtureHost("inside", "inner-host", "")
	foldedLXC := models.StateSnapshot{Nodes: []models.Node{node}, Containers: []models.Container{lxc("web")}, Hosts: []models.Host{foldHost}}
	foldedVM := models.StateSnapshot{Nodes: []models.Node{node}, VMs: []models.VM{vm("web")}, Hosts: []models.Host{foldHost}}
	foldedDocker := models.StateSnapshot{Nodes: []models.Node{node}, Containers: []models.Container{lxc("web")}, DockerHosts: []models.DockerHost{routingFixtureDocker("inside", "inner-host", "")}}
	sharingInside := routingFixtureDocker("inside", "inner-host", "")
	sharingInside.NetworkInterfaces = routingFixtureNIC("eth0", "10.0.0.5/24")
	foldedDockerSharingAddress := models.StateSnapshot{Nodes: []models.Node{node}, Containers: []models.Container{lxc("web")}, DockerHosts: []models.DockerHost{sharingInside}}
	linkedHost := routingFixtureHost("linked", "web", "")
	linkedHost.LinkedContainerID = "lab:pve1:200"
	autoLinked := models.StateSnapshot{Nodes: []models.Node{node}, Containers: []models.Container{lxc("web")}, Hosts: []models.Host{linkedHost}}
	mergedHost := routingFixtureHost("host-record", "dock", "")
	mergedDocker := dockerWithContainer("docker-module")
	mergedDocker.MachineID = mergedHost.MachineID
	merged := models.StateSnapshot{Nodes: []models.Node{node}, Containers: []models.Container{lxc("dock")}, Hosts: []models.Host{mergedHost}, DockerHosts: []models.DockerHost{mergedDocker}}
	namedAfterHostname := routingFixtureDocker("web", "web", "")
	namedAfterHostname.AgentID = "inside"
	namedAfterHostname.NetworkInterfaces = routingFixtureNIC("eth0", "10.0.0.5/24")
	sourceIsHostname := models.StateSnapshot{Nodes: []models.Node{node}, Containers: []models.Container{lxc("web")}, DockerHosts: []models.DockerHost{namedAfterHostname}}
	noIdentity := routingFixtureDocker("legacy", "web", "")
	noIdentity.AgentID = ""
	noIdentity.NetworkInterfaces = routingFixtureNIC("eth0", "10.0.0.5/24")
	sharedName := models.StateSnapshot{Nodes: []models.Node{node}, Containers: []models.Container{lxc("web")}, Hosts: []models.Host{foldHost}, DockerHosts: []models.DockerHost{noIdentity}}

	docker := func(t *testing.T, r *unifiedresources.ResourceRegistry) string {
		return r.DockerHosts()[0].ID()
	}
	app := func(t *testing.T, r *unifiedresources.ResourceRegistry) string {
		containers := r.DockerContainers()
		if len(containers) != 1 {
			t.Fatalf("expected one Docker container, got %d", len(containers))
		}
		return containers[0].ID()
	}
	guest := func(t *testing.T, r *unifiedresources.ResourceRegistry) string {
		if vms := r.VMs(); len(vms) == 1 {
			return vms[0].ID()
		}
		return r.Containers()[0].ID()
	}
	literal := func(reference string) func(*testing.T, *unifiedresources.ResourceRegistry) string {
		return func(*testing.T, *unifiedresources.ResourceRegistry) string { return reference }
	}
	agentRow := func(agentID string) func(*testing.T, *unifiedresources.ResourceRegistry) string {
		return func(t *testing.T, r *unifiedresources.ResourceRegistry) string {
			return routingFixtureAgentResourceID(t, r, agentID)
		}
	}

	nodeAgent := agentexec.ConnectedAgent{AgentID: "node-agent", Hostname: "pve1"}
	direct := func(agent, hostname, kind string) CommandRoutingResult {
		return CommandRoutingResult{AgentID: agent, AgentHostname: hostname, TargetType: "agent", Transport: "direct", ResolvedKind: kind, ResolvedNode: "pve1"}
	}
	throughNode := func(kind string) CommandRoutingResult {
		return CommandRoutingResult{AgentID: "node-agent", AgentHostname: "pve1", TargetType: "container", TargetID: "200", Transport: "pct_exec", ResolvedKind: kind, ResolvedNode: "pve1"}
	}
	nowhere := func(kind string) CommandRoutingResult {
		return CommandRoutingResult{TargetType: "container", TargetID: "200", Transport: "pct_exec", ResolvedKind: kind, ResolvedNode: "pve1"}
	}
	// A host a name does not place in a guest has no node to name.
	standalone := func(agent, hostname, kind string) CommandRoutingResult {
		return CommandRoutingResult{AgentID: agent, AgentHostname: hostname, TargetType: "agent", Transport: "direct", ResolvedKind: kind}
	}
	vmThroughNode := CommandRoutingResult{AgentID: "node-agent", AgentHostname: "pve1", TargetType: "vm", TargetID: "100", Transport: "qm_guest_exec", ResolvedKind: "docker-host", ResolvedNode: "pve1"}
	own := agentexec.ConnectedAgent{AgentID: "dock-a", Hostname: "dock"}
	namesake := agentexec.ConnectedAgent{AgentID: "other", Hostname: "dock"}
	folded := agentexec.ConnectedAgent{AgentID: "inside", Hostname: "inner-host"}

	for _, tc := range []struct {
		name     string
		snapshot models.StateSnapshot
		fold     func(*unifiedresources.ResourceRegistry) unifiedresources.ResourceLink
		extra    []unifiedresources.Resource
		target   func(*testing.T, *unifiedresources.ResourceRegistry) string
		agents   []agentexec.ConnectedAgent
		want     CommandRoutingResult
	}{
		{name: "Docker host in an LXC reaches its own agent", snapshot: inLXC, target: docker, agents: []agentexec.ConnectedAgent{own}, want: direct("dock-a", "dock", "docker-host")},
		{name: "its host row is the same resource", snapshot: inLXC, target: agentRow("dock-a"), agents: []agentexec.ConnectedAgent{own}, want: direct("dock-a", "dock", "docker-host")},
		{name: "its agent's ID reaches it too", snapshot: inLXC, target: literal("dock-a"), agents: []agentexec.ConnectedAgent{own}, want: direct("dock-a", "dock", "docker-host")},
		{name: "Docker host in a VM reaches its own agent", snapshot: inVM, target: docker, agents: []agentexec.ConnectedAgent{own}, want: direct("dock-a", "dock", "docker-host")},
		{name: "a container on it reaches the same agent", snapshot: inLXC, target: app, agents: []agentexec.ConnectedAgent{own}, want: direct("dock-a", "dock", "app-container")},
		{name: "a Docker report with no agent record names its agent", snapshot: noHostRecord, target: docker, agents: []agentexec.ConnectedAgent{own}, want: direct("dock-a", "dock", "docker-host")},

		{name: "a host identified only by its hostname keeps the agent its report names", snapshot: sourceIsHostname, target: docker, agents: []agentexec.ConnectedAgent{folded}, want: direct("inside", "inner-host", "docker-host")},
		{name: "an agent record speaks over the Docker report", snapshot: merged, target: docker, agents: []agentexec.ConnectedAgent{{AgentID: "host-record", Hostname: "dock"}}, want: direct("host-record", "dock", "docker-host")},
		{name: "an agent record speaks for a container on the host", snapshot: merged, target: app, agents: []agentexec.ConnectedAgent{{AgentID: "host-record", Hostname: "dock"}}, want: direct("host-record", "dock", "app-container")},

		{name: "its own agent goes before a connected node agent in an LXC", snapshot: inLXC, target: docker, agents: []agentexec.ConnectedAgent{own, nodeAgent}, want: direct("dock-a", "dock", "docker-host")},
		{name: "its own agent goes before a connected node agent in a VM", snapshot: inVM, target: docker, agents: []agentexec.ConnectedAgent{own, nodeAgent}, want: direct("dock-a", "dock", "docker-host")},
		{name: "its own agent goes before a connected node agent for a container", snapshot: inLXC, target: app, agents: []agentexec.ConnectedAgent{own, nodeAgent}, want: direct("dock-a", "dock", "app-container")},
		{name: "the node agent alone reaches a host that shares its guest's address", snapshot: inLXC, target: docker, agents: []agentexec.ConnectedAgent{nodeAgent}, want: throughNode("docker-host")},
		{name: "the node agent alone reaches that host in a VM", snapshot: inVM, target: docker, agents: []agentexec.ConnectedAgent{nodeAgent}, want: vmThroughNode},
		{name: "the node agent alone reaches a container on that host", snapshot: inLXC, target: app, agents: []agentexec.ConnectedAgent{nodeAgent}, want: throughNode("app-container")},
		{name: "an address on the guest's interface list shows the same machine", snapshot: sharedOnGuestInterface, target: docker, agents: []agentexec.ConnectedAgent{nodeAgent}, want: throughNode("docker-host")},
		{name: "an address only the agent record reports shows the same machine", snapshot: sharedOnAgentRecord, target: docker, agents: []agentexec.ConnectedAgent{nodeAgent}, want: throughNode("docker-host")},

		{name: "another machine with its name runs on its own agent, not in the LXC", snapshot: elsewhere, target: docker, agents: []agentexec.ConnectedAgent{own, nodeAgent}, want: standalone("dock-a", "dock", "docker-host")},
		{name: "another machine with its name runs on its own agent, not in the VM", snapshot: elsewhereFromVM, target: docker, agents: []agentexec.ConnectedAgent{own, nodeAgent}, want: standalone("dock-a", "dock", "docker-host")},
		{name: "a container on another machine with its name runs on that machine", snapshot: elsewhere, target: app, agents: []agentexec.ConnectedAgent{own, nodeAgent}, want: standalone("dock-a", "dock", "app-container")},
		{name: "the node cannot reach another machine with its name", snapshot: elsewhere, target: docker, agents: []agentexec.ConnectedAgent{nodeAgent}, want: standalone("", "", "docker-host")},
		{name: "the node cannot reach a container on another machine with its name", snapshot: elsewhere, target: app, agents: []agentexec.ConnectedAgent{nodeAgent}, want: standalone("", "", "app-container")},
		{name: "the node cannot reach another machine named like its VM", snapshot: elsewhereFromVM, target: docker, agents: []agentexec.ConnectedAgent{nodeAgent}, want: standalone("", "", "docker-host")},
		{name: "a namesake cannot stand in for another machine's agent", snapshot: elsewhere, target: docker, agents: []agentexec.ConnectedAgent{namesake}, want: standalone("", "", "docker-host")},
		{name: "a link formed from the name shows nothing", snapshot: linkedByNameElsewhere, target: docker, agents: []agentexec.ConnectedAgent{nodeAgent}, want: standalone("", "", "docker-host")},
		{name: "a link to another guest contradicts a shared address", snapshot: sharedAddressLinked(func(host *models.Host) { host.LinkedContainerID = "site:pve9:200" }), target: docker, agents: []agentexec.ConnectedAgent{nodeAgent}, want: standalone("", "", "docker-host")},
		{name: "a link to another VMID contradicts a shared address", snapshot: sharedAddressLinked(func(host *models.Host) { host.LinkedContainerID = "lab:pve1:201" }), target: docker, agents: []agentexec.ConnectedAgent{nodeAgent}, want: standalone("", "", "docker-host")},
		{name: "a link to another VM contradicts a shared address", snapshot: sharedAddressLinked(func(host *models.Host) { host.LinkedVMID = "lab:pve1:101" }), target: docker, agents: []agentexec.ConnectedAgent{nodeAgent}, want: standalone("", "", "docker-host")},
		{name: "a link to another guest Pulse knows contradicts a shared address", snapshot: linkedToAKnownGuest, target: docker, agents: []agentexec.ConnectedAgent{nodeAgent}, want: standalone("", "", "docker-host")},
		{name: "a link to a node contradicts a shared address", snapshot: sharedAddressLinked(func(host *models.Host) { host.LinkedNodeID = "lab-pve1" }), target: docker, agents: []agentexec.ConnectedAgent{nodeAgent}, want: standalone("", "", "docker-host")},
		{name: "a link that names nothing readable contradicts a shared address", snapshot: sharedAddressLinked(func(host *models.Host) { host.LinkedContainerID = "unreadable" }), target: docker, agents: []agentexec.ConnectedAgent{nodeAgent}, want: standalone("", "", "docker-host")},
		{name: "a link elsewhere leaves the host its own agent", snapshot: sharedAddressLinked(func(host *models.Host) { host.LinkedContainerID = "site:pve9:200" }), target: docker, agents: []agentexec.ConnectedAgent{own, nodeAgent}, want: standalone("dock-a", "dock", "docker-host")},
		{name: "a link to this guest leaves the shared address standing", snapshot: sharedAddressLinked(func(host *models.Host) { host.LinkedContainerID = "lab:pve1:200" }), target: docker, agents: []agentexec.ConnectedAgent{nodeAgent}, want: throughNode("docker-host")},
		{name: "a link to this guest from before it moved node leaves the address standing", snapshot: sharedAddressLinked(func(host *models.Host) { host.LinkedContainerID = "lab:pve9:200" }), target: docker, agents: []agentexec.ConnectedAgent{nodeAgent}, want: throughNode("docker-host")},
		{name: "a link to this VM leaves the shared address standing", snapshot: sharedAddressLinkedVM(func(host *models.Host) { host.LinkedVMID = "lab:pve1:100" }), target: docker, agents: []agentexec.ConnectedAgent{nodeAgent}, want: vmThroughNode},
		{name: "a link through a merged connection leaves the shared address standing", snapshot: mergedGuest(func(snapshot *models.StateSnapshot) {
			record := routingFixtureHost("dock-a", "dock", "")
			record.LinkedContainerID = "site:pve1:200"
			snapshot.Hosts = []models.Host{record}
			snapshot.DockerHosts = []models.DockerHost{dockerWithContainer("dock-a")}
		}), fold: foldGuestsTogether, target: docker, agents: []agentexec.ConnectedAgent{nodeAgent}, want: throughNode("docker-host")},
		{name: "a probe through a merged connection finds its guest", snapshot: mergedGuest(func(snapshot *models.StateSnapshot) {
			snapshot.DockerHosts = []models.DockerHost{routingFixtureDocker("proxmox-lxc-docker:site:pve1:200", "dock", "")}
		}), fold: foldGuestsTogether, target: docker, agents: []agentexec.ConnectedAgent{nodeAgent}, want: throughNode("docker-host")},
		{name: "a guest with no known address shows nothing to the node", snapshot: unknownAddresses, target: docker, agents: []agentexec.ConnectedAgent{nodeAgent}, want: standalone("", "", "docker-host")},
		{name: "a guest with no known address leaves the host its own agent", snapshot: unknownAddresses, target: docker, agents: []agentexec.ConnectedAgent{own, nodeAgent}, want: standalone("dock-a", "dock", "docker-host")},
		{name: "a Docker bridge address shows nothing", snapshot: onlyABridgeShared, target: docker, agents: []agentexec.ConnectedAgent{nodeAgent}, want: standalone("", "", "docker-host")},
		{name: "a bridge on the guest's interface list shows nothing", snapshot: sharedOnGuestBridge, target: docker, agents: []agentexec.ConnectedAgent{nodeAgent}, want: standalone("", "", "docker-host")},
		{name: "a link-local address shows nothing", snapshot: onlyLinkLocalShared, target: docker, agents: []agentexec.ConnectedAgent{nodeAgent}, want: standalone("", "", "docker-host")},

		{name: "nothing connected goes nowhere", snapshot: inLXC, target: docker, agents: nil, want: nowhere("docker-host")},
		{name: "a namesake cannot stand in for the Docker host's agent", snapshot: inLXC, target: docker, agents: []agentexec.ConnectedAgent{namesake}, want: nowhere("docker-host")},
		{name: "a namesake cannot stand in for the agent a Docker report names", snapshot: noHostRecord, target: docker, agents: []agentexec.ConnectedAgent{namesake}, want: nowhere("docker-host")},
		{name: "a namesake cannot reach a host probed through its node", snapshot: probed, target: docker, agents: []agentexec.ConnectedAgent{namesake}, want: nowhere("docker-host")},
		{name: "a host probed through its node keeps the node path", snapshot: probed, target: docker, agents: []agentexec.ConnectedAgent{nodeAgent}, want: throughNode("docker-host")},

		{name: "a folded agent takes its LXC's command", snapshot: foldedLXC, fold: foldInto("inside"), target: guest, agents: []agentexec.ConnectedAgent{folded}, want: direct("inside", "inner-host", "system-container")},
		{name: "a folded agent's own ID reaches it", snapshot: foldedLXC, fold: foldInto("inside"), target: literal("inside"), agents: []agentexec.ConnectedAgent{folded}, want: direct("inside", "inner-host", "system-container")},
		{name: "a folded agent takes its VM's command", snapshot: foldedVM, fold: foldInto("inside"), target: guest, agents: []agentexec.ConnectedAgent{folded}, want: direct("inside", "inner-host", "vm")},
		{name: "the agent a folded Docker report names takes its guest's command", snapshot: foldedDocker, fold: foldDockerInto("inside"), target: guest, agents: []agentexec.ConnectedAgent{folded}, want: direct("inside", "inner-host", "system-container")},
		{name: "a connected node agent keeps a folded guest's path", snapshot: foldedLXC, fold: foldInto("inside"), target: guest, agents: []agentexec.ConnectedAgent{folded, nodeAgent}, want: throughNode("system-container")},
		{name: "a namesake cannot stand in for a folded agent", snapshot: foldedLXC, fold: foldInto("inside"), target: guest, agents: []agentexec.ConnectedAgent{{AgentID: "other", Hostname: "web"}}, want: nowhere("system-container")},

		{name: "an automatic link does not take a guest's command", snapshot: autoLinked, target: guest, agents: []agentexec.ConnectedAgent{{AgentID: "linked", Hostname: "web"}}, want: nowhere("system-container")},
		{name: "a linked agent's own row still reaches it", snapshot: autoLinked, target: agentRow("linked"), agents: []agentexec.ConnectedAgent{{AgentID: "linked", Hostname: "web"}}, want: CommandRoutingResult{AgentID: "linked", AgentHostname: "web", TargetType: "agent", Transport: "direct", ResolvedKind: "agent"}},
		{name: "an unlinked namesake cannot take a guest's command", snapshot: models.StateSnapshot{Nodes: []models.Node{node}, Containers: []models.Container{lxc("web")}}, target: guest, agents: []agentexec.ConnectedAgent{{AgentID: "other", Hostname: "web"}}, want: nowhere("system-container")},
		{name: "a Docker host placed by name does not inherit the guest's folded agent", snapshot: sharedName, fold: foldInto("inside"), target: docker, agents: []agentexec.ConnectedAgent{folded}, want: nowhere("docker-host")},
		{
			name: "a container with no parent cannot borrow a folded agent through its hostname", snapshot: foldedLXC, fold: foldInto("inside"),
			extra: []unifiedresources.Resource{{
				ID: "parentless", Type: unifiedresources.ResourceTypeAppContainer, Name: "orphan", Sources: []unifiedresources.DataSource{unifiedresources.SourceDocker},
				Docker: &unifiedresources.DockerData{AgentID: "another-reporter", Hostname: "web", ContainerID: "orphan-1"},
			}},
			target: literal("parentless"), agents: []agentexec.ConnectedAgent{folded}, want: standalone("", "", "app-container"),
		},
		{
			name: "a container with no parent cannot borrow a folded report's address through its hostname", snapshot: foldedDockerSharingAddress, fold: foldDockerInto("inside"),
			extra: []unifiedresources.Resource{{
				ID: "parentless", Type: unifiedresources.ResourceTypeAppContainer, Name: "orphan", Sources: []unifiedresources.DataSource{unifiedresources.SourceDocker},
				Docker: &unifiedresources.DockerData{AgentID: "another-reporter", Hostname: "web", ContainerID: "orphan-1"},
			}},
			target: literal("parentless"), agents: []agentexec.ConnectedAgent{nodeAgent}, want: standalone("", "", "app-container"),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			registry := sameNameRegistry(tc.snapshot)
			if tc.fold != nil {
				link := tc.fold(registry)
				registry = unifiedresources.NewRegistryWithManualLinks([]unifiedresources.ResourceLink{link})
				registry.IngestSnapshot(tc.snapshot)
			}
			if tc.extra != nil {
				registry.IngestResources(tc.extra)
			}
			target := tc.target(t, registry)
			// The monitor's read state goes through the adapter, so prove both.
			for name, rs := range map[string]unifiedresources.ReadState{"registry": registry, "adapter": unifiedresources.NewMonitorAdapter(registry)} {
				if got := routeCommandTarget(rs, tc.agents, target); got != tc.want {
					t.Fatalf("%s through the %s routed to %+v, want %+v", target, name, got, tc.want)
				}
			}
		})
	}
}

// A host Pulse found by probing a guest through its node names that guest in its ID,
// so the ID places it. Its name says nothing either way: installations reuse guest
// names, and its hostname comes from inside the guest.
func TestDockerHostProbedThroughAGuestFollowsItsIDNotItsName(t *testing.T) {
	node := func(instance, name string) models.Node {
		return models.Node{ID: instance + "-" + name, Instance: instance, Name: name, Status: "online"}
	}
	lxc := func(instance, nodeName string, vmid int, name string) models.Container {
		return models.Container{ID: fmt.Sprintf("%s:%s:%d", instance, nodeName, vmid), Instance: instance, Node: nodeName, VMID: vmid, Name: name, Type: "lxc", Status: "running"}
	}
	probedHost := func(guestID, hostname string) models.DockerHost {
		id := "proxmox-lxc-docker:" + guestID
		return routingFixtureDockerOfAgent(id, id, hostname)
	}
	// An address shared with a guest of its name must not stand in for what the ID says.
	withAddress := func(guest models.Container) models.Container {
		guest.IPAddresses = []string{"10.0.0.5"}
		return guest
	}
	withNIC := func(host models.DockerHost) models.DockerHost {
		host.NetworkInterfaces = routingFixtureNIC("eth0", "10.0.0.5/24")
		return host
	}
	labAgent := agentexec.ConnectedAgent{AgentID: "lab-agent", Hostname: "pve1"}
	siteAgent := agentexec.ConnectedAgent{AgentID: "site-agent", Hostname: "pve2"}
	throughNode := func(agent, hostname, node string, vmid int) CommandRoutingResult {
		return CommandRoutingResult{AgentID: agent, AgentHostname: hostname, TargetType: "container", TargetID: fmt.Sprintf("%d", vmid), Transport: "pct_exec", ResolvedKind: "docker-host", ResolvedNode: node}
	}
	nowhere := CommandRoutingResult{TargetType: "agent", Transport: "direct", ResolvedKind: "docker-host"}

	for _, tc := range []struct {
		name     string
		snapshot models.StateSnapshot
		sourceID string
		want     CommandRoutingResult
	}{
		{
			name: "two connections reuse a guest's name, and each host keeps its own guest",
			snapshot: models.StateSnapshot{
				Nodes:       []models.Node{node("lab", "pve1"), node("site", "pve2")},
				Containers:  []models.Container{lxc("lab", "pve1", 200, "dock"), lxc("site", "pve2", 300, "dock")},
				DockerHosts: []models.DockerHost{probedHost("lab:pve1:200", "dock"), probedHost("site:pve2:300", "dock")},
			},
			sourceID: "proxmox-lxc-docker:site:pve2:300",
			want:     throughNode("site-agent", "pve2", "pve2", 300),
		},
		{
			name: "another guest has the hostname reported from inside this one",
			snapshot: models.StateSnapshot{
				Nodes:       []models.Node{node("lab", "pve1")},
				Containers:  []models.Container{lxc("lab", "pve1", 200, "dock"), lxc("lab", "pve1", 201, "web")},
				DockerHosts: []models.DockerHost{probedHost("lab:pve1:201", "dock")},
			},
			sourceID: "proxmox-lxc-docker:lab:pve1:201",
			want:     throughNode("lab-agent", "pve1", "pve1", 201),
		},
		{
			name: "the hostname inside the guest is not the guest's name",
			snapshot: models.StateSnapshot{
				Nodes:       []models.Node{node("lab", "pve1")},
				Containers:  []models.Container{lxc("lab", "pve1", 201, "web")},
				DockerHosts: []models.DockerHost{probedHost("lab:pve1:201", "ubuntu")},
			},
			sourceID: "proxmox-lxc-docker:lab:pve1:201",
			want:     throughNode("lab-agent", "pve1", "pve1", 201),
		},
		{
			name: "the guest has moved to another node",
			snapshot: models.StateSnapshot{
				Nodes:       []models.Node{node("lab", "pve1"), node("lab", "pve2")},
				Containers:  []models.Container{lxc("lab", "pve2", 201, "web")},
				DockerHosts: []models.DockerHost{probedHost("lab:pve1:201", "web")},
			},
			sourceID: "proxmox-lxc-docker:lab:pve1:201",
			want:     throughNode("site-agent", "pve2", "pve2", 201),
		},
		{
			name: "its guest has gone, and a guest with its name does not take it",
			snapshot: models.StateSnapshot{
				Nodes:       []models.Node{node("lab", "pve1")},
				Containers:  []models.Container{withAddress(lxc("lab", "pve1", 200, "dock"))},
				DockerHosts: []models.DockerHost{withNIC(probedHost("lab:pve1:999", "dock"))},
			},
			sourceID: "proxmox-lxc-docker:lab:pve1:999",
			want:     nowhere,
		},
		{
			// A probe report can match an existing row by hostname, so the row's ID
			// can name another guest than the report behind its inventory.
			name: "the agent ID behind its inventory beats a row ID naming another guest",
			snapshot: models.StateSnapshot{
				Nodes:       []models.Node{node("lab", "pve1"), node("site", "pve2")},
				Containers:  []models.Container{lxc("lab", "pve1", 200, "dock"), lxc("site", "pve2", 300, "dock")},
				DockerHosts: []models.DockerHost{routingFixtureDockerOfAgent("proxmox-lxc-docker:lab:pve1:200", "proxmox-lxc-docker:site:pve2:300", "dock")},
			},
			sourceID: "proxmox-lxc-docker:lab:pve1:200",
			want:     throughNode("site-agent", "pve2", "pve2", 300),
		},
		{
			name: "its VMID now belongs to a VM",
			snapshot: models.StateSnapshot{
				Nodes:       []models.Node{node("lab", "pve1")},
				VMs:         []models.VM{{ID: "lab:pve1:200", Instance: "lab", Node: "pve1", VMID: 200, Name: "dock", Type: "qemu", Status: "running"}},
				DockerHosts: []models.DockerHost{probedHost("lab:pve1:200", "dock")},
			},
			sourceID: "proxmox-lxc-docker:lab:pve1:200",
			want:     nowhere,
		},
		{
			name: "the same VMID on another connection is not its guest",
			snapshot: models.StateSnapshot{
				Nodes:       []models.Node{node("lab", "pve1"), node("site", "pve2")},
				Containers:  []models.Container{lxc("site", "pve2", 200, "dock")},
				DockerHosts: []models.DockerHost{probedHost("lab:pve1:200", "dock")},
			},
			sourceID: "proxmox-lxc-docker:lab:pve1:200",
			want:     nowhere,
		},
		{
			name: "an ID that names no guest does not fall back to the name",
			snapshot: models.StateSnapshot{
				Nodes:       []models.Node{node("lab", "pve1")},
				Containers:  []models.Container{withAddress(lxc("lab", "pve1", 200, "dock"))},
				DockerHosts: []models.DockerHost{withNIC(probedHost("lxc:200", "dock"))},
			},
			sourceID: "proxmox-lxc-docker:lxc:200",
			want:     nowhere,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			registry := sameNameRegistry(tc.snapshot)
			id := routingFixtureDockerResourceID(t, registry, tc.sourceID)
			for name, rs := range map[string]unifiedresources.ReadState{"registry": registry, "adapter": unifiedresources.NewMonitorAdapter(registry)} {
				if got := routeCommandTarget(rs, []agentexec.ConnectedAgent{labAgent, siteAgent}, id); got != tc.want {
					t.Fatalf("%s through the %s routed to %+v, want %+v", tc.sourceID, name, got, tc.want)
				}
			}
		})
	}
}

// foldInto is the manual link an operator makes between an agent and its guest.
func foldInto(agentID string) func(*unifiedresources.ResourceRegistry) unifiedresources.ResourceLink {
	return func(registry *unifiedresources.ResourceRegistry) unifiedresources.ResourceLink {
		guestID := routingFixtureOnlyGuestID(registry)
		for _, host := range registry.Hosts() {
			if host != nil && host.AgentID() == agentID {
				return unifiedresources.ResourceLink{ResourceA: guestID, ResourceB: host.ID(), PrimaryID: guestID}
			}
		}
		return unifiedresources.ResourceLink{}
	}
}

// foldGuestsTogether merges the two views of one guest under the first connection.
func foldGuestsTogether(registry *unifiedresources.ResourceRegistry) unifiedresources.ResourceLink {
	var primary, second string
	for _, container := range registry.Containers() {
		if container.Instance() == "lab" {
			primary = container.ID()
		} else {
			second = container.ID()
		}
	}
	return unifiedresources.ResourceLink{ResourceA: primary, ResourceB: second, PrimaryID: primary}
}

func foldDockerInto(sourceID string) func(*unifiedresources.ResourceRegistry) unifiedresources.ResourceLink {
	return func(registry *unifiedresources.ResourceRegistry) unifiedresources.ResourceLink {
		guestID := routingFixtureOnlyGuestID(registry)
		for _, host := range registry.DockerHosts() {
			if host != nil && host.HostSourceID() == sourceID {
				return unifiedresources.ResourceLink{ResourceA: guestID, ResourceB: host.ID(), PrimaryID: guestID}
			}
		}
		return unifiedresources.ResourceLink{}
	}
}

func routingFixtureOnlyGuestID(registry *unifiedresources.ResourceRegistry) string {
	if vms := registry.VMs(); len(vms) == 1 {
		return vms[0].ID()
	}
	return registry.Containers()[0].ID()
}

// The route's answer reaches a real read: the command runs on the agent itself, not
// wrapped for a guest through its node. With the node's agent connected as well, a
// Docker host on another machine is read there, not in the guest with its name.
func TestReadOnADockerHostInAGuestRunsOnItsOwnAgent(t *testing.T) {
	own := agentexec.ConnectedAgent{AgentID: "dock-a", Hostname: "dock"}
	for name, agents := range map[string][]agentexec.ConnectedAgent{
		"its own agent connected":        {own},
		"its node's agent connected too": {own, {AgentID: "node-agent", Hostname: "pve1"}},
	} {
		t.Run(name, func(t *testing.T) {
			registry := sameNameRegistry(models.StateSnapshot{
				Nodes:       []models.Node{{ID: "lab-pve1", Instance: "lab", Name: "pve1", Status: "online"}},
				Containers:  []models.Container{{ID: "lab:pve1:200", Instance: "lab", Node: "pve1", VMID: 200, Name: "dock", Type: "lxc", Status: "running"}},
				Hosts:       []models.Host{routingFixtureHost("dock-a", "dock", "")},
				DockerHosts: []models.DockerHost{routingFixtureDocker("dock-a", "dock", "")},
			})
			server := &mockAgentServer{agents: agents}
			server.On("GetConnectedAgents").Return(server.agents)
			server.On("ExecuteCommand", mock.Anything, "dock-a", mock.MatchedBy(func(payload agentexec.ExecuteCommandPayload) bool {
				return payload.TargetType == "agent" && payload.TargetID == "" && strings.Contains(payload.Command, "docker logs")
			})).Return(&agentexec.CommandResultPayload{ExitCode: 0, Stdout: "reached the Docker host's agent"}, nil)
			executor := NewPulseToolExecutor(ExecutorConfig{ReadState: registry, AgentServer: server, ControlLevel: ControlLevelReadOnly})
			result, err := executor.ExecuteTool(context.Background(), "pulse_read", map[string]interface{}{
				"action": "logs", "source": "docker", "container": "app", "since": "30m", "target_host": registry.DockerHosts()[0].ID(),
			})
			if err != nil || result.IsError || len(result.Content) == 0 || !strings.Contains(result.Content[0].Text, "reached the Docker host's agent") {
				t.Fatalf("the read did not reach the Docker host's own agent: %+v %v", result, err)
			}
			server.AssertExpectations(t)
		})
	}
}

// An identity hostname can name the machine a guest runs on, and clones can share
// one, so only the guest's own name stands in for a link.
func TestQueryGuestOwnFlagReadsOnlyTheGuestsOwnName(t *testing.T) {
	guest := func(id, name string, hostnames []string) unifiedresources.Resource {
		return unifiedresources.Resource{ID: id, Type: unifiedresources.ResourceTypeVM, Name: name, Identity: unifiedresources.ResourceIdentity{Hostnames: hostnames}}
	}
	onTrueNAS := guest("truenas-guest", "vm-a", []string{"tank", "vm-a"})
	onTrueNAS.Sources = []unifiedresources.DataSource{unifiedresources.SourceTrueNAS}
	onTrueNAS.TrueNAS = &unifiedresources.TrueNASData{Hostname: "tank"}
	cloneOne := guest("clone-one", "vm-b", []string{"vm-b", "web"})
	cloneTwo := guest("clone-two", "vm-c", []string{"vm-c", "web"})
	named := guest("named", "db", []string{"db"})
	for _, resource := range []*unifiedresources.Resource{&cloneOne, &cloneTwo, &named} {
		resource.Sources = []unifiedresources.DataSource{unifiedresources.SourceVMware}
		resource.VMware = &unifiedresources.VMwareData{RuntimeHostName: "esx1", GuestHostname: "web"}
	}
	named.VMware.GuestHostname = "db"
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestResources([]unifiedresources.Resource{onTrueNAS, cloneOne, cloneTwo, named})
	executor := sameNameQueryExecutors(registry,
		agentexec.ConnectedAgent{AgentID: "tank-agent", Hostname: "tank"},
		agentexec.ConnectedAgent{AgentID: "web-agent", Hostname: "web"},
		agentexec.ConnectedAgent{AgentID: "db-agent", Hostname: "db"},
	)["canonical resources"]
	for query, want := range map[string]bool{"vm-a": false, "vm-b": false, "vm-c": false, "db": true} {
		row := sameNameSearchRow(t, executor, query, "vm")
		if row["command_agent_connected"] != want {
			t.Fatalf("%s should report its own connection=%t: %+v", query, want, row)
		}
	}
}

// One installation keeps the JSON it had wherever the flag already agreed with
// routing, including the topology row, which has no ID unless a hostname repeats.
func TestQueryHostOutputForOneInstallationIsUnchanged(t *testing.T) {
	dockerRow := func(t *testing.T, row map[string]any) string {
		t.Helper()
		delete(row, "policy")
		delete(row, "ai_safe_summary")
		raw, err := json.Marshal(row)
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	registry := sameNameRegistry(models.StateSnapshot{
		Hosts: []models.Host{routingFixtureHost("agent-a", "box", ""), routingFixtureHost("agent-d", "dock", ""), routingFixtureHost("agent-off", "quiet", "")},
		DockerHosts: []models.DockerHost{
			routingFixtureDocker("agent-d", "dock", ""),
			routingFixtureDocker("agent-off", "quiet", ""),
		},
	})
	agents := []agentexec.ConnectedAgent{{AgentID: "agent-a", Hostname: "box"}, {AgentID: "agent-d", Hostname: "dock"}}
	for variant, executor := range sameNameQueryExecutors(registry, agents...) {
		t.Run(variant, func(t *testing.T) {
			summary, rows := routingFixtureDockerTopology(t, executor)
			if summary["docker_hosts_with_command_agents"] != float64(1) {
				t.Fatalf("topology count changed: %+v", summary)
			}
			want := []string{
				`{"can_execute":true,"command_agent_connected":true,"container_count":0,"containers":[],"containers_truncated":false,"display_name":"dock","hostname":"dock","returned_container_count":0,"running_count":0}`,
				`{"can_execute":false,"command_agent_connected":false,"container_count":0,"containers":[],"containers_truncated":false,"display_name":"quiet","hostname":"quiet","returned_container_count":0,"running_count":0}`,
			}
			if len(rows) != len(want) {
				t.Fatalf("topology rows changed: %+v", rows)
			}
			for i := range rows {
				if got := dockerRow(t, rows[i]); got != want[i] {
					t.Fatalf("topology row changed:\n got %s\nwant %s", got, want[i])
				}
			}
			listed := sameNameRows(t, sameNameQuery(t, executor, map[string]interface{}{"action": "list", "type": "docker-hosts"}), "docker_hosts")
			flags := make(map[string]any, len(listed))
			for _, row := range listed {
				flags[row["hostname"].(string)] = row["command_agent_connected"]
			}
			if flags["dock"] != true || flags["quiet"] != false {
				t.Fatalf("listed flags changed: %+v", listed)
			}
			if variant != "canonical resources" {
				return
			}
			systems := make(map[string]any)
			for _, row := range sameNameRows(t, sameNameQuery(t, executor, map[string]interface{}{"action": "list", "type": "systems"}), "systems") {
				systems[row["name"].(string)] = row["command_agent_connected"]
			}
			if systems["box"] != true || systems["dock"] != true || systems["quiet"] != false {
				t.Fatalf("system flags changed: %+v", systems)
			}
		})
	}
}

func TestQueryOmitsUnobservedHostAndGuestConnections(t *testing.T) {
	registry := sameNameRegistry(models.StateSnapshot{
		Nodes:       []models.Node{{ID: "a-pve1", Instance: "a", Name: "pve1", Status: "online"}},
		VMs:         []models.VM{{ID: "a:pve1:100", Instance: "a", Node: "pve1", VMID: 100, Name: "web", Type: "qemu", Status: "running"}},
		Hosts:       []models.Host{routingFixtureHost("agent-a", "box", "")},
		DockerHosts: []models.DockerHost{routingFixtureDocker("dock-a", "dock", "")},
	})
	guest := registry.VMs()[0]
	hostID := routingFixtureAgentResourceID(t, registry, "agent-a")
	dockerID := routingFixtureDockerResourceID(t, registry, "dock-a")

	unobserved := newQueryCommandConnections(registry, nil)
	for label, flag := range map[string]*bool{
		"host":   unobserved.target(hostID),
		"docker": unobserved.target(dockerID),
		"guest":  unobserved.guest(guest.ID(), guest.Name()),
	} {
		if flag != nil {
			t.Fatalf("unobserved %s became a value: %t", label, *flag)
		}
	}
	observed := newQueryCommandConnections(registry, []agentexec.ConnectedAgent{})
	for label, flag := range map[string]*bool{
		"host":   observed.target(hostID),
		"docker": observed.target(dockerID),
		"guest":  observed.guest(guest.ID(), guest.Name()),
	} {
		if flag == nil || *flag {
			t.Fatalf("observed %s with no agents should read false: %v", label, flag)
		}
	}
}

// Routing and the flags share one answer, so an empty reference must not read as
// "the only connected agent", which is what routing does with no target at all.
func TestQueryRowWithoutAnIDClaimsNoConnection(t *testing.T) {
	registry := sameNameRegistry(models.StateSnapshot{})
	connections := newQueryCommandConnections(registry, []agentexec.ConnectedAgent{{AgentID: "only", Hostname: "only"}})
	if flag := connections.target("  "); flag == nil || *flag {
		t.Fatalf("a row with no ID borrowed the only connected agent: %v", flag)
	}
	executor := sameNameExecutor(registry, agentexec.ConnectedAgent{AgentID: "only", Hostname: "only"})
	if routed := executor.resolveTargetForCommandFull(""); routed.AgentID != "only" {
		t.Fatalf("no target should still pick the only agent: %+v", routed)
	}
}
