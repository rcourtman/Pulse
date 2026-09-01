package monitoring

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
)

func sharedTokenSiteReport(machineID string, at time.Time) agentsdocker.Report {
	return agentsdocker.Report{
		Agent: agentsdocker.AgentInfo{
			// The unified agent's Docker module reports the machine ID as its
			// agent ID (the hostagent fallback chain, #985/#986).
			ID:              machineID,
			Version:         "6.4.2",
			Type:            "unified",
			IntervalSeconds: 30,
		},
		Host: agentsdocker.HostInfo{
			Hostname:  "docker01",
			MachineID: machineID,
		},
		Timestamp: at,
	}
}

// Docker analog of #1753: two live machines reuse one short hostname and one
// shared install token. The hostname+token fallback may fold the second
// machine's first report (indistinguishable from a recreated container until
// the first machine reports again), but the identities must not keep
// flip-flopping: the machine that minted the record reclaims it, and the other
// machine converges to the documented unique-token rejection instead of
// silently overwriting the record every cycle.
func TestApplyDockerReportSharedTokenSameHostnameDoesNotFlipFlop(t *testing.T) {
	monitor := newTestMonitor(t)
	token := &config.APITokenRecord{ID: "shared-install-token", Name: "Shared install"}
	base := time.Now().UTC()

	siteA, err := monitor.ApplyDockerReport(sharedTokenSiteReport("machine-id-site-a", base), token)
	if err != nil {
		t.Fatalf("site A first report: %v", err)
	}

	// Site B's first report is the one ambiguous cycle and may fold into
	// site A's record.
	if _, err := monitor.ApplyDockerReport(sharedTokenSiteReport("machine-id-site-b", base.Add(time.Second)), token); err != nil {
		t.Logf("site B first report rejected immediately: %v", err)
	}

	// Site A's next report proves two live machines are alternating: it must
	// reclaim its own record, not mint a new identity.
	reclaimed, err := monitor.ApplyDockerReport(sharedTokenSiteReport("machine-id-site-a", base.Add(2*time.Second)), token)
	if err != nil {
		t.Fatalf("site A reclaim report: %v", err)
	}
	if reclaimed.ID != siteA.ID {
		t.Fatalf("site A lost its record: acknowledged %q, want %q", reclaimed.ID, siteA.ID)
	}
	if reclaimed.MachineID != "machine-id-site-a" {
		t.Fatalf("site A record carries machine ID %q after reclaim", reclaimed.MachineID)
	}

	// From here on, site B must be steered to a unique token, and site A must
	// stay stable - the silent alternation is what collapsed the reporter's
	// estate and let a removal revoke the shared credential.
	for cycle := 0; cycle < 3; cycle++ {
		at := base.Add(time.Duration(3+cycle*2) * time.Second)
		_, err := monitor.ApplyDockerReport(sharedTokenSiteReport("machine-id-site-b", at), token)
		if err == nil {
			t.Fatalf("cycle %d: site B silently adopted an identity instead of the unique-token rejection", cycle)
		}
		if !strings.Contains(err.Error(), "unique API token") {
			t.Fatalf("cycle %d: site B rejection lacks the unique-token guidance: %v", cycle, err)
		}

		steady, err := monitor.ApplyDockerReport(sharedTokenSiteReport("machine-id-site-a", at.Add(time.Second)), token)
		if err != nil {
			t.Fatalf("cycle %d: site A report rejected: %v", cycle, err)
		}
		if steady.ID != siteA.ID || steady.MachineID != "machine-id-site-a" {
			t.Fatalf("cycle %d: site A flapped to ID %q machine %q", cycle, steady.ID, steady.MachineID)
		}
	}

	hosts := monitor.state.GetDockerHosts()
	if len(hosts) != 1 {
		ids := make([]string, 0, len(hosts))
		for _, h := range hosts {
			ids = append(ids, h.ID+"@"+h.MachineID)
		}
		t.Fatalf("expected exactly site A's record to survive, got %d: %v", len(hosts), ids)
	}
	if hosts[0].ID != siteA.ID || hosts[0].MachineID != "machine-id-site-a" {
		t.Fatalf("surviving record is %q@%q, want %q@machine-id-site-a", hosts[0].ID, hosts[0].MachineID, siteA.ID)
	}
}
