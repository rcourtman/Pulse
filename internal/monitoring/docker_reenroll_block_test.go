package monitoring

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
)

func reenrollBlockTestReport(agentID string, at time.Time) agentsdocker.Report {
	return agentsdocker.Report{
		Agent: agentsdocker.AgentInfo{
			ID:              agentID,
			Version:         "6.4.1",
			Type:            "unified",
			IntervalSeconds: 30,
		},
		Host: agentsdocker.HostInfo{
			Hostname:  "alpine-docker",
			MachineID: "machine-alpine-docker",
			TotalCPU:  4,
		},
		Containers: []agentsdocker.Container{
			{ID: "container-" + agentID, Name: "portainer"},
		},
		Timestamp: at,
	}
}

// Removing a Docker host blocks its reports, but a token minted after the
// removal is explicit re-enroll intent and must clear the block — the same
// rule host agents follow (#1581). Before this rule the re-enrolled host was
// rejected until a server restart happened to wipe the in-memory block, which
// read as "Docker tab missing until reboot" (#1728). The block must also
// survive a restart via the persisted store, so the restart itself never
// becomes the thing that lifts it.
func TestDockerRemovalBlockClearsOnFreshTokenReenroll(t *testing.T) {
	monitor := newTestMonitor(t)
	base := time.Now().UTC().Add(-2 * time.Hour)

	oldToken := &config.APITokenRecord{ID: "token-old", CreatedAt: base}
	host, err := monitor.ApplyDockerReport(reenrollBlockTestReport("agent-alpine", base.Add(time.Minute)), oldToken)
	if err != nil {
		t.Fatalf("initial report: %v", err)
	}

	if _, err := monitor.RemoveDockerHost(host.ID); err != nil {
		t.Fatalf("remove docker host: %v", err)
	}

	// The still-running old agent keeps presenting its pre-removal token and
	// stays blocked.
	if _, err := monitor.ApplyDockerReport(reenrollBlockTestReport("agent-alpine", base.Add(2*time.Minute)), oldToken); err == nil {
		t.Fatal("expected pre-removal token report to stay blocked")
	}

	// Simulate a server restart: the in-memory map resets while the persisted
	// entry remains. The block must hold.
	monitor.mu.Lock()
	monitor.removedDockerHosts = make(map[string]time.Time)
	monitor.mu.Unlock()
	if _, err := monitor.ApplyDockerReport(reenrollBlockTestReport("agent-alpine", base.Add(3*time.Minute)), oldToken); err == nil {
		t.Fatal("expected persisted removal block to survive restart")
	}

	// A token minted after removal clears the block and the report lands.
	freshToken := &config.APITokenRecord{ID: "token-new", CreatedAt: time.Now().UTC()}
	reenrolled, err := monitor.ApplyDockerReport(reenrollBlockTestReport("agent-alpine", base.Add(4*time.Minute)), freshToken)
	if err != nil {
		t.Fatalf("fresh-token re-enroll rejected: %v", err)
	}

	hosts := monitor.state.GetDockerHosts()
	if len(hosts) != 1 || hosts[0].ID != reenrolled.ID {
		ids := make([]string, 0, len(hosts))
		for _, h := range hosts {
			ids = append(ids, h.ID)
		}
		t.Fatalf("expected exactly the re-enrolled host in state, got %v", ids)
	}

	monitor.mu.RLock()
	_, stillBlockedInMemory := monitor.removedDockerHosts[host.ID]
	monitor.mu.RUnlock()
	if stillBlockedInMemory {
		t.Fatal("expected in-memory removal block to be cleared by fresh-token re-enroll")
	}
	for _, entry := range monitor.state.GetRemovedDockerHosts() {
		if entry.ID == host.ID {
			t.Fatal("expected persisted removal block to be cleared by fresh-token re-enroll")
		}
	}
}
