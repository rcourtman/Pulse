package monitoring

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
)

func supersedeTestReport(agentID, version string, at time.Time) agentsdocker.Report {
	return agentsdocker.Report{
		Agent: agentsdocker.AgentInfo{
			ID:              agentID,
			Version:         version,
			Type:            "unified",
			IntervalSeconds: 30,
		},
		Host: agentsdocker.HostInfo{
			Hostname:  "nuc",
			MachineID: "machine-nuc",
			TotalCPU:  4,
		},
		Containers: []agentsdocker.Container{
			{ID: "container-" + agentID, Name: "heimdall"},
		},
		Timestamp: at,
	}
}

// A wiped agent state dir regenerates the agent ID, and the fresh install
// command mints a fresh token, so identity resolution refuses to adopt the old
// record and creates a new one. The old record must be superseded rather than
// left behind with a stale agent version and stale containers (#1586, #1564).
func TestApplyDockerReportSupersedesStaleReenrolledHost(t *testing.T) {
	monitor := newTestMonitor(t)
	base := time.Now().UTC()

	oldToken := &config.APITokenRecord{ID: "token-old", CreatedAt: base.Add(-24 * time.Hour)}
	oldHost, err := monitor.ApplyDockerReport(supersedeTestReport("agent-old", "6.1.0-rc.2", base), oldToken)
	if err != nil {
		t.Fatalf("old report: %v", err)
	}

	// The new token is minted after the old record's last report: explicit
	// re-enroll intent, so the corpse is reaped.
	newToken := &config.APITokenRecord{ID: "token-new", CreatedAt: time.Now().UTC().Add(time.Hour)}
	newHost, err := monitor.ApplyDockerReport(supersedeTestReport("agent-new", "6.1.0-rc.3", base.Add(time.Hour)), newToken)
	if err != nil {
		t.Fatalf("new report: %v", err)
	}
	if newHost.ID == oldHost.ID {
		t.Fatalf("expected re-enrollment under a new token to mint a new identity, both got %q", newHost.ID)
	}

	hosts := monitor.state.GetDockerHosts()
	if len(hosts) != 1 {
		ids := make([]string, 0, len(hosts))
		for _, h := range hosts {
			ids = append(ids, h.ID+"@"+h.AgentVersion)
		}
		t.Fatalf("expected the stale record to be superseded, got %d hosts: %v", len(hosts), ids)
	}
	if hosts[0].ID != newHost.ID {
		t.Fatalf("expected the surviving record to be the new identity %q, got %q", newHost.ID, hosts[0].ID)
	}
	if hosts[0].AgentVersion != "v6.1.0-rc.3" {
		t.Fatalf("expected surviving record on the new agent version, got %q", hosts[0].AgentVersion)
	}

	monitor.mu.RLock()
	_, oldTokenStillBound := monitor.dockerTokenBindings["token-old"]
	_, oldStillBlocked := monitor.removedDockerHosts[oldHost.ID]
	monitor.mu.RUnlock()
	if oldTokenStillBound {
		t.Fatal("expected the orphaned token binding to be released")
	}
	if oldStillBlocked {
		t.Fatal("supersession must not set the deliberate-removal resurrection block")
	}
}

// A record that reported after the new token was minted is a live host (for
// example two genuinely different machines behind mismatched machine-id
// hygiene, or an old agent the user has not uninstalled yet). It must never be
// reaped by another agent's report.
func TestApplyDockerReportKeepsLiveHostDespiteMatchingIdentity(t *testing.T) {
	monitor := newTestMonitor(t)
	base := time.Now().UTC()

	oldToken := &config.APITokenRecord{ID: "token-old", CreatedAt: base.Add(-24 * time.Hour)}
	if _, err := monitor.ApplyDockerReport(supersedeTestReport("agent-old", "6.1.0-rc.2", base), oldToken); err != nil {
		t.Fatalf("old report: %v", err)
	}

	// Token minted before the existing record's last report: no proof the old
	// record is a corpse, so both stay.
	staleToken := &config.APITokenRecord{ID: "token-new", CreatedAt: base.Add(-time.Hour)}
	if _, err := monitor.ApplyDockerReport(supersedeTestReport("agent-new", "6.1.0-rc.3", base.Add(time.Minute)), staleToken); err != nil {
		t.Fatalf("new report: %v", err)
	}

	if hosts := monitor.state.GetDockerHosts(); len(hosts) != 2 {
		t.Fatalf("expected both records to survive without corpse proof, got %d hosts", len(hosts))
	}
}
