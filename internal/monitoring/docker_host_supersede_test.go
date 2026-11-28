package monitoring

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/api/agenttokens"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
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

// Re-enrolling a unified agent with a fresh install token updates the host
// record under the original ID while the Docker report forks to a token-scoped
// ID. The old agent can also emit one final in-flight report just after the
// replacement token is minted before the installer stops it. Neither the
// agent's own host record sharing the new token nor that brief overlap should
// leave a duplicate offline Docker host behind.
func TestApplyDockerReportSupersedesStaleHostWhenUnifiedHostRecordSharesNewToken(t *testing.T) {
	monitor := newTestMonitor(t)
	tokenCreatedAt := time.Now().UTC()

	oldToken := &config.APITokenRecord{ID: "token-old", CreatedAt: tokenCreatedAt.Add(-24 * time.Hour)}
	// Old agent sends one final in-flight report 36s after the replacement token
	// is minted (within one health window), then stops.
	oldReport := supersedeTestReport("agent-tower", "6.10.0-beta.2", tokenCreatedAt.Add(36*time.Second))
	oldReport.Host.Hostname = "Tower"
	oldReport.Host.MachineID = "machine-tower"
	oldHost, err := monitor.ApplyDockerReport(oldReport, oldToken)
	if err != nil {
		t.Fatalf("old report: %v", err)
	}

	// Unified agent re-enrolls with the new token: its host record already lives
	// under the canonical agent ID and the token's command-policy metadata points
	// at the same host.
	newToken := &config.APITokenRecord{
		ID:        "token-new",
		CreatedAt: tokenCreatedAt,
		Metadata: map[string]string{
			agenttokens.CommandPolicyAppliedAgentIDMetadataKey: "agent-tower",
		},
	}
	monitor.state.UpsertHost(models.Host{
		ID:        "agent-tower",
		Hostname:  "Tower",
		MachineID: "machine-tower",
		TokenID:   newToken.ID,
		Status:    "online",
		LastSeen:  tokenCreatedAt.Add(14 * time.Minute),
	})

	newReport := supersedeTestReport("agent-tower", "6.10.0-beta.2", tokenCreatedAt.Add(14*time.Minute))
	newReport.Host.Hostname = "Tower"
	newReport.Host.MachineID = "machine-tower"
	newHost, err := monitor.ApplyDockerReport(newReport, newToken)
	if err != nil {
		t.Fatalf("new report: %v", err)
	}
	if newHost.ID == oldHost.ID {
		t.Fatalf("expected forked Docker identity for new token, got %q", newHost.ID)
	}

	hosts := monitor.state.GetDockerHosts()
	if len(hosts) != 1 {
		ids := make([]string, 0, len(hosts))
		for _, h := range hosts {
			ids = append(ids, h.ID)
		}
		t.Fatalf("expected stale Docker host to be superseded despite unified host record sharing token, got %d hosts: %v", len(hosts), ids)
	}
	if hosts[0].ID != newHost.ID {
		t.Fatalf("expected surviving Docker host %q, got %q", newHost.ID, hosts[0].ID)
	}

	monitor.mu.RLock()
	_, oldTokenStillBound := monitor.dockerTokenBindings[oldToken.ID]
	monitor.mu.RUnlock()
	if oldTokenStillBound {
		t.Fatal("expected old Docker token binding to be released")
	}
}

// A token that already belongs to a different physical host must never
// supersede a stopped Docker host on another machine, whether that other host
// is still present in state or only referenced by token command-policy metadata.
func TestApplyDockerReportDoesNotSupersedeWhenTokenEnrolledOnDifferentHost(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(*Monitor, *config.APITokenRecord, time.Time)
	}{
		{
			name: "other host still in state",
			setup: func(m *Monitor, token *config.APITokenRecord, now time.Time) {
				m.state.UpsertHost(models.Host{
					ID:        "lab-host",
					Hostname:  "lab.local",
					MachineID: "machine-lab",
					TokenID:   token.ID,
					Status:    "online",
					LastSeen:  now,
				})
			},
		},
		{
			name: "other host recorded in command policy metadata",
			setup: func(_ *Monitor, token *config.APITokenRecord, _ time.Time) {
				token.Metadata = map[string]string{
					agenttokens.CommandPolicyAppliedAgentIDMetadataKey: "lab-host",
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			monitor := newTestMonitor(t)
			base := time.Now().UTC()

			oldToken := &config.APITokenRecord{ID: "token-old", CreatedAt: base.Add(-24 * time.Hour)}
			if _, err := monitor.ApplyDockerReport(supersedeTestReport("agent-old", "6.10.0-beta.2", base), oldToken); err != nil {
				t.Fatalf("old report: %v", err)
			}

			otherToken := &config.APITokenRecord{ID: "token-other", CreatedAt: base.Add(time.Hour)}
			tc.setup(monitor, otherToken, base.Add(time.Hour))

			if _, err := monitor.ApplyDockerReport(supersedeTestReport("agent-new", "6.10.0-beta.2", base.Add(time.Hour)), otherToken); err != nil {
				t.Fatalf("new report: %v", err)
			}

			if hosts := monitor.state.GetDockerHosts(); len(hosts) != 2 {
				t.Fatalf("expected stopped host to remain untouched when token belongs to another machine, got %d hosts", len(hosts))
			}
		})
	}
}

// Pre-existing duplicate Docker host generations created before this fix should
// still be cleaned up on the next report from the live Docker host.
func TestApplyDockerReportSupersedesPreexistingDuplicateOnSubsequentReport(t *testing.T) {
	monitor := newTestMonitor(t)
	createdAt := time.Now().UTC()

	monitor.state.UpsertDockerHost(models.DockerHost{
		ID:              "agent-tower",
		AgentID:         "agent-tower",
		Hostname:        "Tower",
		MachineID:       "machine-tower",
		TokenID:         "token-old",
		Status:          "offline",
		IntervalSeconds: 30,
		LastSeen:        createdAt.Add(30 * time.Second),
	})
	monitor.state.UpsertDockerHost(models.DockerHost{
		ID:              "agent-tower::token-token-new",
		AgentID:         "agent-tower",
		Hostname:        "Tower",
		MachineID:       "machine-tower",
		TokenID:         "token-new",
		Status:          "online",
		IntervalSeconds: 30,
		LastSeen:        createdAt.Add(5 * time.Minute),
	})

	newToken := &config.APITokenRecord{ID: "token-new", CreatedAt: createdAt}
	report := supersedeTestReport("agent-tower", "6.10.0-beta.2", createdAt.Add(6*time.Minute))
	report.Host.Hostname = "Tower"
	report.Host.MachineID = "machine-tower"
	if _, err := monitor.ApplyDockerReport(report, newToken); err != nil {
		t.Fatalf("subsequent report: %v", err)
	}

	hosts := monitor.state.GetDockerHosts()
	if len(hosts) != 1 || hosts[0].ID != "agent-tower::token-token-new" {
		t.Fatalf("expected preexisting duplicate to be superseded, got %+v", hosts)
	}
}
