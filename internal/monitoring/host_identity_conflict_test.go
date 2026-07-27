package monitoring

import (
	"testing"
	"time"

	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

func TestHostIdentityFlapTrackerDetectsAlternatingHostnames(t *testing.T) {
	tracker := newHostIdentityFlapTracker()
	base := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	interval := 30 * time.Second

	if conflict := tracker.observe("clone-a", "", base); conflict != nil {
		t.Fatalf("first report should not conflict, got %+v", conflict)
	}
	// First switch could be a legitimate rename, so it must not warn yet.
	if conflict := tracker.observe("clone-b", "", base.Add(interval)); conflict != nil {
		t.Fatalf("single hostname switch should not conflict, got %+v", conflict)
	}
	// The revisit of clone-a proves two machines are alternating.
	conflict := tracker.observe("clone-a", "", base.Add(2*interval))
	if conflict == nil {
		t.Fatal("expected conflict after hostname revisit")
	}
	if len(conflict.Hostnames) != 2 || conflict.Hostnames[0] != "clone-a" || conflict.Hostnames[1] != "clone-b" {
		t.Fatalf("expected sorted hostnames [clone-a clone-b], got %v", conflict.Hostnames)
	}
	if len(conflict.ReportIPs) != 0 {
		t.Fatalf("absent report IPs should not be listed as conflicting, got %v", conflict.ReportIPs)
	}
	if conflict.FirstSeen.IsZero() || conflict.LastSeen.IsZero() {
		t.Fatalf("expected conflict timestamps, got %+v", conflict)
	}

	// Continued flapping keeps the conflict alive and refreshes LastSeen.
	later := tracker.observe("clone-b", "", base.Add(3*interval))
	if later == nil {
		t.Fatal("expected conflict to persist while flapping continues")
	}
	if !later.LastSeen.After(conflict.LastSeen) {
		t.Fatalf("expected LastSeen to advance, got %v then %v", conflict.LastSeen, later.LastSeen)
	}
	if !later.FirstSeen.Equal(conflict.FirstSeen) {
		t.Fatalf("expected FirstSeen to be stable, got %v then %v", conflict.FirstSeen, later.FirstSeen)
	}
}

func TestHostIdentityFlapTrackerDetectsAlternatingReportIPsBehindOneHostname(t *testing.T) {
	// MSP template deployments reuse hostnames across sites (pve01 at two
	// customers), so the alternating report IP is the only field that betrays
	// the clone.
	tracker := newHostIdentityFlapTracker()
	base := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)

	tracker.observe("pve01", "192.168.1.10", base)
	if conflict := tracker.observe("pve01", "10.0.0.10", base.Add(30*time.Second)); conflict != nil {
		t.Fatalf("single report IP switch should not conflict, got %+v", conflict)
	}
	conflict := tracker.observe("pve01", "192.168.1.10", base.Add(60*time.Second))
	if conflict == nil {
		t.Fatal("expected conflict after report IP revisit")
	}
	if len(conflict.Hostnames) != 1 || conflict.Hostnames[0] != "pve01" {
		t.Fatalf("expected shared hostname [pve01], got %v", conflict.Hostnames)
	}
	if len(conflict.ReportIPs) != 2 || conflict.ReportIPs[0] != "10.0.0.10" || conflict.ReportIPs[1] != "192.168.1.10" {
		t.Fatalf("expected sorted report IPs [10.0.0.10 192.168.1.10], got %v", conflict.ReportIPs)
	}
}

func TestHostIdentityFlapTrackerIgnoresSingleRename(t *testing.T) {
	tracker := newHostIdentityFlapTracker()
	base := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)

	if conflict := tracker.observe("old-name", "", base); conflict != nil {
		t.Fatalf("unexpected conflict: %+v", conflict)
	}
	for i := 1; i <= 5; i++ {
		if conflict := tracker.observe("new-name", "", base.Add(time.Duration(i)*30*time.Second)); conflict != nil {
			t.Fatalf("rename should never conflict, got %+v on report %d", conflict, i)
		}
	}
}

func TestHostIdentityFlapTrackerConflictExpiresAfterWindow(t *testing.T) {
	tracker := newHostIdentityFlapTracker()
	base := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)

	tracker.observe("clone-a", "", base)
	tracker.observe("clone-b", "", base.Add(30*time.Second))
	if conflict := tracker.observe("clone-a", "", base.Add(60*time.Second)); conflict == nil {
		t.Fatal("expected conflict after revisit")
	}

	// One clone goes away; steady reports from the survivor eventually clear it.
	steady := base.Add(60 * time.Second).Add(hostIdentityConflictWindow + time.Minute)
	if conflict := tracker.observe("clone-a", "", steady); conflict != nil {
		t.Fatalf("expected conflict to expire after quiet window, got %+v", conflict)
	}
}

func TestApplyHostReportFlagsClonedMachineIDIdentity(t *testing.T) {
	monitor := newTestMonitor(t)

	report := func(hostname, reportIP string, at time.Time) agentshost.Report {
		return agentshost.Report{
			Agent: agentshost.AgentInfo{
				ID:              "machine-duplicate",
				Version:         "6.0.0",
				Type:            "unified",
				IntervalSeconds: 30,
			},
			Host: agentshost.HostInfo{
				ID:       "machine-duplicate",
				Hostname: hostname,
				Platform: "linux",
				ReportIP: reportIP,
			},
			Timestamp: at,
		}
	}

	base := time.Now().UTC()

	// Two physical pve01 machines at different sites share a cloned
	// /etc/machine-id, so their reports collapse into one host row whose
	// report IP alternates every cycle.
	first, err := monitor.ApplyHostReport(report("pve01", "192.168.1.10", base), nil)
	if err != nil {
		t.Fatalf("first report: %v", err)
	}
	if first.IdentityConflict != nil {
		t.Fatalf("first report should not conflict, got %+v", first.IdentityConflict)
	}

	second, err := monitor.ApplyHostReport(report("pve01", "10.0.0.10", base.Add(30*time.Second)), nil)
	if err != nil {
		t.Fatalf("second report: %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("expected clones to collapse into one identity, got %q vs %q", first.ID, second.ID)
	}
	if second.IdentityConflict != nil {
		t.Fatalf("single report IP switch should not conflict yet, got %+v", second.IdentityConflict)
	}

	third, err := monitor.ApplyHostReport(report("pve01", "192.168.1.10", base.Add(60*time.Second)), nil)
	if err != nil {
		t.Fatalf("third report: %v", err)
	}
	if third.IdentityConflict == nil {
		t.Fatal("expected identity conflict once the report IP flapped back")
	}
	if got := third.IdentityConflict.ReportIPs; len(got) != 2 || got[0] != "10.0.0.10" || got[1] != "192.168.1.10" {
		t.Fatalf("expected conflict report IPs [10.0.0.10 192.168.1.10], got %v", got)
	}

	hosts := monitor.state.GetHosts()
	if len(hosts) != 1 {
		t.Fatalf("expected 1 collapsed host in state, got %d", len(hosts))
	}
	if hosts[0].IdentityConflict == nil {
		t.Fatal("expected identity conflict to be stored on the state host")
	}

	// The conflict lingers while inside the detection window so the UI keeps
	// warning even between flaps.
	steady, err := monitor.ApplyHostReport(report("pve01", "192.168.1.10", base.Add(90*time.Second)), nil)
	if err != nil {
		t.Fatalf("steady report: %v", err)
	}
	if steady.IdentityConflict == nil {
		t.Fatal("conflict should linger while inside the detection window")
	}
}

func TestApplyHostReportOneTimeHostnameRenameDoesNotConflict(t *testing.T) {
	monitor := newTestMonitor(t)

	report := func(hostname string, at time.Time) agentshost.Report {
		return agentshost.Report{
			Agent: agentshost.AgentInfo{ID: "renamed-machine", Type: "unified", Version: "6.0.0", IntervalSeconds: 30},
			Host: agentshost.HostInfo{
				ID:       "renamed-machine",
				Hostname: hostname,
				Platform: "linux",
			},
			Timestamp: at,
		}
	}

	base := time.Now().UTC()
	if _, err := monitor.ApplyHostReport(report("old-name", base), nil); err != nil {
		t.Fatalf("first report: %v", err)
	}
	for i := 1; i <= 3; i++ {
		host, err := monitor.ApplyHostReport(report("new-name", base.Add(time.Duration(i)*30*time.Second)), nil)
		if err != nil {
			t.Fatalf("report %d: %v", i, err)
		}
		if host.IdentityConflict != nil {
			t.Fatalf("one-time rename should never conflict, got %+v on report %d", host.IdentityConflict, i)
		}
	}
}

func TestApplyHostReportIdentityConflictClearedOnHostRemoval(t *testing.T) {
	monitor := newTestMonitor(t)

	base := time.Now().UTC()
	mkReport := func(hostname string, at time.Time) agentshost.Report {
		return agentshost.Report{
			Agent:     agentshost.AgentInfo{ID: "machine-dup", Type: "unified", Version: "6.0.0"},
			Host:      agentshost.HostInfo{ID: "machine-dup", Hostname: hostname, Platform: "linux"},
			Timestamp: at,
		}
	}

	host, err := monitor.ApplyHostReport(mkReport("clone-a", base), nil)
	if err != nil {
		t.Fatalf("first report: %v", err)
	}
	if _, err = monitor.ApplyHostReport(mkReport("clone-b", base.Add(time.Second)), nil); err != nil {
		t.Fatalf("second report: %v", err)
	}
	if _, err = monitor.ApplyHostReport(mkReport("clone-a", base.Add(2*time.Second)), nil); err != nil {
		t.Fatalf("third report: %v", err)
	}

	monitor.mu.Lock()
	if _, ok := monitor.hostIdentityFlaps[host.ID]; !ok {
		monitor.mu.Unlock()
		t.Fatal("expected flap tracker for host identity")
	}
	monitor.mu.Unlock()

	if _, err := monitor.RemoveHostAgent(host.ID); err != nil {
		t.Fatalf("RemoveHostAgent: %v", err)
	}

	monitor.mu.Lock()
	if _, ok := monitor.hostIdentityFlaps[host.ID]; ok {
		monitor.mu.Unlock()
		t.Fatal("expected flap tracker to be cleared on host removal")
	}
	monitor.mu.Unlock()
}
