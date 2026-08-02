package monitoring

import (
	"testing"
	"time"

	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
)

func TestTrackDockerHostIdentityTranslatesConflict(t *testing.T) {
	monitor := newTestMonitor(t)
	base := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)

	// When the machine IDs behind one host identity diverge, the model's
	// MachineIDs field must carry them alongside the shared hostname.
	monitor.trackDockerHostIdentity("host-1", "shared-host", "machine-a", base)
	monitor.trackDockerHostIdentity("host-1", "shared-host", "machine-b", base.Add(30*time.Second))
	conflict := monitor.trackDockerHostIdentity("host-1", "shared-host", "machine-a", base.Add(60*time.Second))
	if conflict == nil {
		t.Fatal("expected conflict after machine ID revisit")
	}
	if len(conflict.Hostnames) != 1 || conflict.Hostnames[0] != "shared-host" {
		t.Fatalf("expected hostnames [shared-host], got %v", conflict.Hostnames)
	}
	if len(conflict.MachineIDs) != 2 || conflict.MachineIDs[0] != "machine-a" || conflict.MachineIDs[1] != "machine-b" {
		t.Fatalf("expected sorted machine IDs [machine-a machine-b], got %v", conflict.MachineIDs)
	}
	if conflict.FirstSeen.IsZero() || conflict.LastSeen.IsZero() {
		t.Fatalf("expected conflict timestamps, got %+v", conflict)
	}
}

func TestApplyDockerReportFlagsClonedMachineIDIdentity(t *testing.T) {
	monitor := newTestMonitor(t)

	report := func(hostname string, at time.Time) agentsdocker.Report {
		return agentsdocker.Report{
			Agent: agentsdocker.AgentInfo{
				ID:              "machine-duplicate",
				Version:         "6.0.0",
				Type:            "unified",
				IntervalSeconds: 30,
			},
			Host: agentsdocker.HostInfo{
				Hostname:  hostname,
				MachineID: "machine-duplicate",
				TotalCPU:  4,
			},
			Timestamp: at,
		}
	}

	base := time.Now().UTC()

	first, err := monitor.ApplyDockerReport(report("clone-a", base), nil)
	if err != nil {
		t.Fatalf("first report: %v", err)
	}
	if first.IdentityConflict != nil {
		t.Fatalf("first report should not conflict, got %+v", first.IdentityConflict)
	}

	second, err := monitor.ApplyDockerReport(report("clone-b", base.Add(30*time.Second)), nil)
	if err != nil {
		t.Fatalf("second report: %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("expected clones to collapse into one identity, got %q vs %q", first.ID, second.ID)
	}
	if second.IdentityConflict != nil {
		t.Fatalf("single hostname switch should not conflict yet, got %+v", second.IdentityConflict)
	}

	third, err := monitor.ApplyDockerReport(report("clone-a", base.Add(60*time.Second)), nil)
	if err != nil {
		t.Fatalf("third report: %v", err)
	}
	if third.IdentityConflict == nil {
		t.Fatal("expected identity conflict once the hostname flapped back")
	}
	if got := third.IdentityConflict.Hostnames; len(got) != 2 || got[0] != "clone-a" || got[1] != "clone-b" {
		t.Fatalf("expected conflict hostnames [clone-a clone-b], got %v", got)
	}

	hosts := monitor.state.GetDockerHosts()
	if len(hosts) != 1 {
		t.Fatalf("expected 1 collapsed host in state, got %d", len(hosts))
	}
	if hosts[0].IdentityConflict == nil {
		t.Fatal("expected identity conflict to be stored on the state host")
	}

	// A healthy host keeps reporting the same identity and never conflicts.
	steady, err := monitor.ApplyDockerReport(report("clone-a", base.Add(90*time.Second)), nil)
	if err != nil {
		t.Fatalf("steady report: %v", err)
	}
	if steady.IdentityConflict == nil {
		t.Fatal("conflict should linger while inside the detection window")
	}
}

func TestApplyDockerReportIdentityConflictClearedOnHostRemoval(t *testing.T) {
	monitor := newTestMonitor(t)

	base := time.Now().UTC()
	mkReport := func(hostname string, at time.Time) agentsdocker.Report {
		return agentsdocker.Report{
			Agent:     agentsdocker.AgentInfo{ID: "machine-dup", Type: "unified", Version: "6.0.0"},
			Host:      agentsdocker.HostInfo{Hostname: hostname, MachineID: "machine-dup"},
			Timestamp: at,
		}
	}

	host, err := monitor.ApplyDockerReport(mkReport("clone-a", base), nil)
	if err != nil {
		t.Fatalf("first report: %v", err)
	}
	if _, err = monitor.ApplyDockerReport(mkReport("clone-b", base.Add(time.Second)), nil); err != nil {
		t.Fatalf("second report: %v", err)
	}
	if _, err = monitor.ApplyDockerReport(mkReport("clone-a", base.Add(2*time.Second)), nil); err != nil {
		t.Fatalf("third report: %v", err)
	}

	monitor.mu.Lock()
	if _, ok := monitor.dockerIdentityFlaps[host.ID]; !ok {
		monitor.mu.Unlock()
		t.Fatal("expected flap tracker for host identity")
	}
	monitor.clearDockerHostIdentityTrackingLocked(host.ID)
	if _, ok := monitor.dockerIdentityFlaps[host.ID]; ok {
		monitor.mu.Unlock()
		t.Fatal("expected flap tracker to be cleared")
	}
	monitor.mu.Unlock()
}
