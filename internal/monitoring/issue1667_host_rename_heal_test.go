package monitoring

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

func issue1667Report(hostname string, at time.Time) agentshost.Report {
	return agentshost.Report{
		Agent: agentshost.AgentInfo{
			ID:              "mac-02d9aee1c3a1",
			Type:            "unified",
			Version:         "6.1.2",
			IntervalSeconds: 30,
		},
		Host: agentshost.HostInfo{
			ID:        "mac-02d9aee1c3a1",
			Hostname:  hostname,
			Platform:  "linux",
			MachineID: "machine-issue1667",
		},
		Timestamp: at,
	}
}

func issue1667AgeHost(t *testing.T, m *Monitor, hostID string, lastSeen time.Time) {
	t.Helper()
	for _, h := range m.state.GetHosts() {
		if h.ID == hostID {
			h.LastSeen = lastSeen
			m.state.UpsertHost(h)
			return
		}
	}
	t.Fatalf("host %q not found in state", hostID)
}

// Issue #1667: a hostname change on the same machine and token (e.g. the Home
// Assistant add-on gaining agent_hostname after first boot) forked the host
// module onto a suffixed identity while workload modules kept the base agent
// ID, permanently splitting one machine into two agent identities. Once the
// pre-rename record has clearly stopped reporting, the fork must heal back to
// the base identity.
func TestApplyHostReportRenameHealsForkedIdentity(t *testing.T) {
	monitor := newTestMonitor(t)
	monitor.hostTokenBindings = make(map[string]string)
	token := &config.APITokenRecord{ID: "token-issue1667", Name: "HA"}

	now := time.Now().UTC()
	original, err := monitor.ApplyHostReport(issue1667Report("3ec0269c-pulse-docker-agent", now), token)
	if err != nil {
		t.Fatalf("first report: %v", err)
	}

	// Rename while the old record is still fresh: this is indistinguishable
	// from a cloned VM sharing a machine ID (#1584), so it must still fork.
	forked, err := monitor.ApplyHostReport(issue1667Report("homeassistant", now.Add(30*time.Second)), token)
	if err != nil {
		t.Fatalf("renamed report: %v", err)
	}
	if forked.ID == original.ID {
		t.Fatalf("expected fork while the old hostname record is fresh, both got %q", forked.ID)
	}

	// The machine only reports under the new hostname, so the pre-rename
	// record goes quiet. Once it is stale the fork is proven to be a rename.
	issue1667AgeHost(t, monitor, original.ID, now.Add(-24*time.Hour))

	healed, err := monitor.ApplyHostReport(issue1667Report("homeassistant", now.Add(60*time.Second)), token)
	if err != nil {
		t.Fatalf("healing report: %v", err)
	}
	if healed.ID != original.ID {
		t.Fatalf("expected heal back to base identity %q, got %q", original.ID, healed.ID)
	}

	hosts := monitor.state.GetHosts()
	if len(hosts) != 1 {
		t.Fatalf("expected exactly one host after heal, got %d", len(hosts))
	}
	if hosts[0].ID != original.ID || hosts[0].Hostname != "homeassistant" {
		t.Fatalf("expected base identity with new hostname, got ID %q hostname %q", hosts[0].ID, hosts[0].Hostname)
	}
}

// A rename discovered only after the old record already went stale (agent was
// stopped, hostname changed, agent restarted later) must adopt the base
// identity directly instead of forking at all.
func TestApplyHostReportRenameAdoptsBaseWhenOldRecordStale(t *testing.T) {
	monitor := newTestMonitor(t)
	monitor.hostTokenBindings = make(map[string]string)
	token := &config.APITokenRecord{ID: "token-issue1667b", Name: "HA"}

	now := time.Now().UTC()
	original, err := monitor.ApplyHostReport(issue1667Report("old-name", now), token)
	if err != nil {
		t.Fatalf("first report: %v", err)
	}

	issue1667AgeHost(t, monitor, original.ID, now.Add(-24*time.Hour))

	renamed, err := monitor.ApplyHostReport(issue1667Report("new-name", now.Add(30*time.Second)), token)
	if err != nil {
		t.Fatalf("renamed report: %v", err)
	}
	if renamed.ID != original.ID {
		t.Fatalf("expected stale rename to keep base identity %q, got %q", original.ID, renamed.ID)
	}

	hosts := monitor.state.GetHosts()
	if len(hosts) != 1 {
		t.Fatalf("expected exactly one host after stale rename, got %d", len(hosts))
	}
	if hosts[0].Hostname != "new-name" {
		t.Fatalf("expected hostname to follow the rename, got %q", hosts[0].Hostname)
	}
}

// Two live machines flapping on one machine ID (cloned VMs, #1584) must keep
// their forked identities: the heal only fires when the colliding record has
// stopped reporting.
func TestApplyHostReportCloneKeepsForkedIdentity(t *testing.T) {
	monitor := newTestMonitor(t)
	monitor.hostTokenBindings = make(map[string]string)
	token := &config.APITokenRecord{ID: "token-issue1667c", Name: "HA"}

	now := time.Now().UTC()
	cloneA, err := monitor.ApplyHostReport(issue1667Report("clone-a", now), token)
	if err != nil {
		t.Fatalf("clone-a report: %v", err)
	}
	cloneB, err := monitor.ApplyHostReport(issue1667Report("clone-b", now.Add(15*time.Second)), token)
	if err != nil {
		t.Fatalf("clone-b report: %v", err)
	}
	if cloneB.ID == cloneA.ID {
		t.Fatalf("expected clones to fork, both got %q", cloneB.ID)
	}

	// Both keep reporting: no heal, identities stay split.
	againA, err := monitor.ApplyHostReport(issue1667Report("clone-a", now.Add(30*time.Second)), token)
	if err != nil {
		t.Fatalf("clone-a second report: %v", err)
	}
	againB, err := monitor.ApplyHostReport(issue1667Report("clone-b", now.Add(45*time.Second)), token)
	if err != nil {
		t.Fatalf("clone-b second report: %v", err)
	}
	if againA.ID != cloneA.ID || againB.ID != cloneB.ID {
		t.Fatalf("expected stable forked identities, got %q/%q then %q/%q", cloneA.ID, cloneB.ID, againA.ID, againB.ID)
	}
	if len(monitor.state.GetHosts()) != 2 {
		t.Fatalf("expected two host records for live clones, got %d", len(monitor.state.GetHosts()))
	}
}
