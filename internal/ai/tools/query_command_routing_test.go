package tools

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func routingFixtureHost(id, hostname, displayName string) models.Host {
	return models.Host{ID: id, Hostname: hostname, DisplayName: displayName, MachineID: "machine-" + id, Status: "online", LastSeen: time.Now(), CommandsEnabled: true}
}

func routingFixtureDocker(id, hostname, displayName string) models.DockerHost {
	return models.DockerHost{ID: id, AgentID: id, Hostname: hostname, DisplayName: displayName, MachineID: "machine-" + id, Status: "online", LastSeen: time.Now()}
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
			name:     "a Docker host in a container, with only the agent inside it connected",
			snapshot: dockerInContainer(),
			agents:   []agentexec.ConnectedAgent{{AgentID: "dock-a", Hostname: "dock"}},
			docker:   map[string]string{"dock-a": ""},
		},
		{
			name:     "a Docker host in a container, with only its node's agent connected",
			snapshot: dockerInContainer(),
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
					if _, named := rows[0]["id"]; named {
						t.Fatalf("a unique hostname gained an ID: %+v", rows[0])
					}
					for id, agent := range wantDockerByID {
						if rows[0]["command_agent_connected"] != (agent != "") {
							t.Fatalf("topology row for %s should report connected=%t: %+v", id, agent != "", rows[0])
						}
					}
				})
			}
		})
	}
}

// A connected agent running inside a guest is the guest's own connection. A
// command for the guest row still goes through its node, so the two flags answer
// separately: node_command_agent_connected is where that command goes, and
// command_agent_connected is whether the agent's own row reaches the guest.
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
