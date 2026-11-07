package monitoring

import (
	"encoding/json"
	"math/rand"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// The reference copies the pre-index scans. Keep it independent of index keys,
// candidate selection and sameAgentIdentity, including source-order precedence.
type scanningAgentFleetIdentity struct{ state models.StateSnapshot }

func scanningAgentFleetSameIdentity(subject agentFleetSubject, id, agentID, hostname, tokenID string) bool {
	return (subject.agentID != "" && (subject.agentID == agentID || subject.agentID == id)) ||
		(subject.tokenID != "" && tokenID != "" && subject.tokenID == tokenID) ||
		(subject.hostname != "" && strings.EqualFold(subject.hostname, hostname))
}

func (scan scanningAgentFleetIdentity) hostPeer(subject agentFleetSubject) (models.Host, bool) {
	for _, host := range scan.state.Hosts {
		if host.ID != subject.id && scanningAgentFleetSameIdentity(subject, host.ID, host.ID, host.Hostname, host.TokenID) {
			return host, true
		}
	}
	return models.Host{}, false
}

func (scan scanningAgentFleetIdentity) dockerPeer(subject agentFleetSubject) (models.DockerHost, bool) {
	for _, host := range scan.state.DockerHosts {
		if !host.Hidden && host.ID != subject.id && scanningAgentFleetSameIdentity(subject, host.ID, host.AgentID, host.Hostname, host.TokenID) {
			return host, true
		}
	}
	return models.DockerHost{}, false
}

func (scan scanningAgentFleetIdentity) duplicateHostPeer(subject agentFleetSubject) (models.Host, bool) {
	if subject.host == nil {
		return models.Host{}, false
	}
	machineID := strings.TrimSpace(subject.host.MachineID)
	hostname := strings.TrimSpace(subject.host.Hostname)
	if machineID == "" || hostname == "" {
		return models.Host{}, false
	}
	for _, peer := range scan.state.Hosts {
		if strings.TrimSpace(peer.ID) != strings.TrimSpace(subject.host.ID) &&
			strings.TrimSpace(peer.MachineID) == machineID &&
			unifiedresources.HostnamesEquivalent(peer.Hostname, hostname) {
			return peer, true
		}
	}
	return models.Host{}, false
}

func assertAgentFleetPeerParity(t *testing.T, state models.StateSnapshot, subjects []agentFleetSubject) {
	t.Helper()
	index, scan := newAgentFleetIdentityIndex(state), scanningAgentFleetIdentity{state}
	for pos, subject := range subjects {
		gotHost, gotOK := index.hostPeer(subject)
		wantHost, wantOK := scan.hostPeer(subject)
		if gotOK != wantOK || !reflect.DeepEqual(gotHost, wantHost) {
			t.Fatalf("subject %d host peer differs: got %+v/%v want %+v/%v", pos, gotHost, gotOK, wantHost, wantOK)
		}
		gotDocker, gotOK := index.dockerPeer(subject)
		wantDocker, wantOK := scan.dockerPeer(subject)
		if gotOK != wantOK || !reflect.DeepEqual(gotDocker, wantDocker) {
			t.Fatalf("subject %d Docker peer differs: got %+v/%v want %+v/%v", pos, gotDocker, gotOK, wantDocker, wantOK)
		}
		gotHost, gotOK = index.duplicateHostPeer(subject)
		wantHost, wantOK = scan.duplicateHostPeer(subject)
		if gotOK != wantOK || !reflect.DeepEqual(gotHost, wantHost) {
			t.Fatalf("subject %d duplicate peer differs: got %+v/%v want %+v/%v", pos, gotHost, gotOK, wantHost, wantOK)
		}
	}
}

func TestAgentFleetIdentityIndexPreservesScanSemantics(t *testing.T) {
	state := models.StateSnapshot{
		Hosts: []models.Host{
			{ID: "self", Hostname: "NODE", TokenID: "token", MachineID: "machine"},
			{ID: "name-first", Hostname: "node", MachineID: "machine"},
			{ID: "token-later", Hostname: "other", TokenID: "token"},
			{ID: "agent-last", Hostname: "elsewhere"},
			{ID: "Unicode", Hostname: "ſKΣ"},
			{ID: "spaced", Hostname: " node "},
			{ID: "fqdn-one", Hostname: "node.one", MachineID: " machine "},
			{ID: "fqdn-two", Hostname: "node.two", MachineID: "machine"},
			{ID: "ip", Hostname: "192.0.2.1", MachineID: "machine"},
			{ID: "invalid", Hostname: string([]byte{0xff, 'X'})},
			{ID: "self", Hostname: "NODE", TokenID: "token", MachineID: "machine"},
			{ID: "", Hostname: "", TokenID: "", MachineID: ""},
		},
		DockerHosts: []models.DockerHost{
			{ID: "hidden", Hostname: "node", TokenID: "token", Hidden: true},
			{ID: "self", Hostname: "node", TokenID: "token"},
			{ID: "name-first", Hostname: "node"},
			{ID: "token-later", TokenID: "token"},
			{ID: "id-later", AgentID: "agent-last"},
			{ID: "Unicode", Hostname: "ſKΣ"},
			{ID: "fqdn-one", Hostname: "node.one"},
			{ID: "invalid", Hostname: string([]byte{0xfe, 'x'})},
			{ID: "", Hostname: ""},
		},
	}
	index := newAgentFleetIdentityIndex(state)
	// An early weaker signal beats a later agent-ID match, and self/hidden
	// rows cannot win. This is NOT an opportunity to reinterpret identity.
	subject := agentFleetSubject{id: "self", agentID: "agent-last", tokenID: "token", hostname: "NODE"}
	if got, ok := index.hostPeer(subject); !ok || got.ID != "name-first" {
		t.Fatalf("host source order changed: %+v/%v", got, ok)
	}
	if got, ok := index.dockerPeer(subject); !ok || got.ID != "name-first" {
		t.Fatalf("Docker source order or exclusion changed: %+v/%v", got, ok)
	}
	if got, ok := index.dockerPeer(agentFleetSubject{hostname: "sKς"}); !ok || got.ID != "Unicode" {
		t.Fatalf("Unicode EqualFold match was lost: %+v/%v", got, ok)
	}
	subjects := buildAgentFleetSubjects(state)
	for _, value := range []string{"", "node", "NODE", " node ", "node.", "node.one", "sKς", "ſKΣ", string([]byte{0xff, 'x'}), "absent"} {
		subjects = append(subjects, agentFleetSubject{id: value, agentID: value, tokenID: value, hostname: value, host: &models.Host{ID: value, Hostname: value, MachineID: " machine "}})
	}
	assertAgentFleetPeerParity(t, state, subjects)
}

func TestAgentFleetIdentityIndexRandomizedScanParity(t *testing.T) {
	random := rand.New(rand.NewSource(20261107))
	values := []string{"", "a", "A", "a.", "a.one", "a.two", " a ", "b", "s", "ſ", "K", "K", "Σ", "ς", "192.0.2.1", "m"}
	pick := func() string { return values[random.Intn(len(values))] }
	for trial := 0; trial < 100; trial++ {
		state := models.StateSnapshot{}
		for row := 0; row < 25; row++ {
			state.Hosts = append(state.Hosts, models.Host{ID: pick(), Hostname: pick(), TokenID: pick(), MachineID: pick()})
			state.DockerHosts = append(state.DockerHosts, models.DockerHost{ID: pick(), AgentID: pick(), Hostname: pick(), TokenID: pick(), Hidden: random.Intn(3) == 0})
		}
		subjects := buildAgentFleetSubjects(state)
		for row := 0; row < 25; row++ {
			host := &models.Host{ID: pick(), MachineID: pick(), Hostname: pick()}
			subjects = append(subjects, agentFleetSubject{id: pick(), agentID: pick(), hostname: pick(), tokenID: pick(), host: host})
		}
		assertAgentFleetPeerParity(t, state, subjects)
	}
}

func TestAgentFleetHostnameFoldKeyCoversSimpleFold(t *testing.T) {
	for r := rune(0); r <= unicode.MaxRune; r++ {
		folded := unicode.SimpleFold(r)
		if folded == r {
			continue
		}
		if agentFleetHostnameFoldKey(string(r)) != agentFleetHostnameFoldKey(string(folded)) {
			t.Fatalf("EqualFold-equivalent runes get different buckets: %U / %U", r, folded)
		}
	}
}

// This repeats the containing producer's old orchestration with scanning
// lookups, checking the entire wire payload, not just a selected reason code.
func scanningAgentFleetDiagnostics(m *Monitor) AgentFleetDiagnostics {
	state := m.GetState()
	profiles, assignments, deployments := m.agentFleetProfileState()
	profileByID, assignmentByAgent := mapProfilesByID(profiles), mapAssignmentsByAgent(assignments)
	deploymentByAgentProfile := mapDeploymentsByAgentProfile(deployments)
	tokens := m.agentFleetTokenInventory(agentFleetBenchmarkNow)
	scan := scanningAgentFleetIdentity{state}
	out := AgentFleetDiagnostics{SchemaVersion: AgentFleetDiagnosticsSchemaVersion, GeneratedAt: agentFleetBenchmarkNow.UnixMilli(), ServerVersion: "6.7.0", AgentUpdateTargetVersion: "6.7.0"}
	for _, subject := range buildAgentFleetSubjects(state) {
		out.Agents = append(out.Agents, diagnoseAgentFleetSubject(subject, state, "6.7.0", agentFleetBenchmarkNow, tokens, profileByID, assignmentByAgent, deploymentByAgentProfile, scan))
	}
	sort.Slice(out.Agents, func(i, j int) bool {
		if out.Agents[i].Status != out.Agents[j].Status {
			return agentFleetStatusRank(out.Agents[i].Status) > agentFleetStatusRank(out.Agents[j].Status)
		}
		return strings.ToLower(out.Agents[i].Name) < strings.ToLower(out.Agents[j].Name)
	})
	out.Summary.Total = len(out.Agents)
	for _, agent := range out.Agents {
		switch agent.Status {
		case AgentFleetStatusCritical:
			out.Summary.Critical++
		case AgentFleetStatusWarning:
			out.Summary.Warning++
		case AgentFleetStatusRemoved:
			out.Summary.Removed++
		default:
			out.Summary.Healthy++
		}
	}
	return out
}

func TestAgentFleetIdentityIndexPreservesCompleteDiagnostics(t *testing.T) {
	for _, scenario := range []string{"host-only", "mixed-split", "profile-and-duplicate"} {
		t.Run(scenario, func(t *testing.T) {
			m := agentFleetBenchmarkMonitor(20, scenario)
			if scenario == "profile-and-duplicate" {
				m.persistence = config.NewConfigPersistence(t.TempDir())
				m.state.Hosts[1].MachineID, m.state.Hosts[1].Hostname = m.state.Hosts[0].MachineID, m.state.Hosts[0].Hostname
				m.state.Hosts[0].AgentVersion = "6.6.0" // Generic upgrade must remain blocked for duplicate installs.
				m.state.Hosts[0].TokenID = "missing"    // Authentication repair must remain blocked too.
				m.state.Hosts[2].TokenID = "missing"
				m.state.Hosts[3].LastSeen = agentFleetBenchmarkNow.Add(-20 * time.Minute)
				m.state.DockerHosts = []models.DockerHost{{ID: "split", Hostname: m.state.Hosts[0].Hostname, AgentVersion: "6.7.0", LastSeen: agentFleetBenchmarkNow, Status: "online"}}
				m.state.RemovedHostAgents = []models.RemovedHostAgent{{ID: "removed", Hostname: "removed", RemovedAt: agentFleetBenchmarkNow}}
				assignments := []models.AgentProfileAssignment{}
				for _, host := range m.state.Hosts {
					assignments = append(assignments, models.AgentProfileAssignment{AgentID: host.ID, ProfileID: "p", ProfileVersion: 2})
				}
				saveAgentFleetProfileState(t, m.persistence, []models.AgentProfile{{ID: "p", Version: 2, Config: models.AgentConfigMap{"enable_docker": true, "enable_kubernetes": true, "enable_proxmox": true}}}, assignments, nil)
			}
			before := m.state.GetSnapshot()
			got := m.GetAgentFleetDiagnosticsForTarget("6.7.0", "6.7.0", agentFleetBenchmarkNow)
			want := scanningAgentFleetDiagnostics(m)
			gotJSON, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			wantJSON, err := json.Marshal(want)
			if err != nil {
				t.Fatal(err)
			}
			if string(gotJSON) != string(wantJSON) {
				t.Fatalf("complete diagnostics changed:\ngot %s\nwant %s", gotJSON, wantJSON)
			}
			if !reflect.DeepEqual(before, m.state.GetSnapshot()) {
				t.Fatal("read-only diagnostics changed monitoring state")
			}
			if scenario == "profile-and-duplicate" {
				agent := requireAgentDiagnostic(t, got, "agent-"+m.state.Hosts[0].ID)
				requireReasonCode(t, agent, AgentFleetReasonDuplicateInstallation)
				requireReasonCode(t, agent, "docker_profile_identity_split")
				for _, action := range agent.RepairActions {
					if action.Supported {
						t.Fatalf("duplicate installation gained a supported repair: %+v", action)
					}
				}
			}
			t.Logf("identical complete wire payload: %d bytes, %d diagnostic rows", len(gotJSON), len(got.Agents))
		})
	}
}

func TestAgentFleetIdentityIndexBelongsToOneSnapshot(t *testing.T) {
	m := agentFleetBenchmarkMonitor(20, "mixed-split")
	oldState := m.state.GetSnapshot()
	index := newAgentFleetIdentityIndex(oldState)
	subject := buildAgentFleetSubjects(oldState)[0]
	peer, ok := index.dockerPeer(subject)
	if !ok {
		t.Fatal("fixture has no peer")
	}
	// A subsequent report/removal must only change the next request. There is
	// no cached identity or stale index shared by these independent snapshots.
	m.state.ClearAllDockerHosts()
	if got, ok := index.dockerPeer(subject); !ok || got.ID != peer.ID {
		t.Fatal("an already captured index changed with live state")
	}
	if _, ok := newAgentFleetIdentityIndex(m.state.GetSnapshot()).dockerPeer(subject); ok {
		t.Fatal("a new request kept a removed peer")
	}
	var wg sync.WaitGroup
	for reader := 0; reader < 8; reader++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				if got, ok := index.dockerPeer(subject); !ok || got.ID != peer.ID {
					t.Errorf("concurrent reader changed captured peer")
				}
			}
		}()
	}
	wg.Wait()
}

func TestAgentFleetIdentityIndexAvoidsUnrelatedCandidates(t *testing.T) {
	for _, size := range []int{10, 100, 1000} {
		state := agentFleetBenchmarkMonitor(size, "mixed-split").state.GetSnapshot()
		index := newAgentFleetIdentityIndex(state)
		visits := 0
		for i, host := range state.Hosts {
			subject := agentFleetSubject{id: host.ID, agentID: host.ID, hostname: host.Hostname}
			pos := index.dockerPeers.first(subject, func(int) bool { visits++; return true })
			if pos != i {
				t.Fatalf("lost source position: got %d want %d", pos, i)
			}
			if len(index.hostsByMachine[host.MachineID]) != 1 {
				t.Fatal("duplicate lookup is not confined to its machine ID")
			}
		}
		if visits != size {
			t.Fatalf("index visits unrelated candidates: %d for %d subjects", visits, size)
		}
		t.Logf("%d mixed hosts: %d split candidate predicates (scan requires %d), one duplicate candidate per host", size, visits, size*(size+1)/2)
	}
}

// Compile-time assurance that both the indexed producer and the independent
// scan oracle satisfy the same diagnostic seam.
var _ agentFleetIdentityReader = scanningAgentFleetIdentity{}
var _ agentFleetIdentityReader = (*agentFleetIdentityIndex)(nil)
