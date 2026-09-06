package monitoring

import (
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"os"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// An unmatched report is not an operator request to unlink. In particular,
// repairing old automatic associations must not erase a manual association.
func TestManualHostLinkSurvivesUnmatchedReportsAndProviderRefresh(t *testing.T) {
	monitor := issue1654Monitor()
	monitor.hostContinuityStore = config.NewHostContinuityStore(t.TempDir(), nil)
	report := issue1654Report(time.Now().UTC())
	report.Host.Hostname = "nas.example"
	report.Host.ReportIP = ""
	report.Network = nil
	host, err := monitor.ApplyHostReport(report, nil)
	if err != nil {
		t.Fatal(err)
	}
	node := models.Node{ID: "pve-node", Name: "pve", Instance: "cluster", Host: "https://pve.example:8006"}
	monitor.state.UpdateNodesForInstance("cluster", []models.Node{node})
	if err := monitor.LinkHostAgent(host.ID, node.ID); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		report.Timestamp = report.Timestamp.Add(time.Second)
		host, err = monitor.ApplyHostReport(report, nil)
		if err != nil {
			t.Fatal(err)
		}
		if host.LinkedNodeID != node.ID {
			t.Fatalf("manual host link after report %d = %q, want %q", i, host.LinkedNodeID, node.ID)
		}
		monitor.state.UpdateNodesForInstance("cluster", []models.Node{node})
		snapshot := monitor.state.GetSnapshot()
		if len(snapshot.Nodes) != 1 || snapshot.Nodes[0].LinkedAgentID != host.ID {
			t.Fatalf("provider refresh lost manual reverse link: %+v", snapshot.Nodes)
		}
		if got := monitor.state.GetHosts()[0].LinkedNodeID; got != node.ID {
			t.Fatalf("provider refresh host link = %q", got)
		}
	}
	if err := monitor.UnlinkHostAgent(host.ID); err != nil {
		t.Fatal(err)
	}
	if got := monitor.state.GetHosts()[0].LinkedNodeID; got != "" {
		t.Fatalf("explicit unlink retained host link %q", got)
	}
	if got := monitor.state.GetSnapshot().Nodes[0].LinkedAgentID; got != "" {
		t.Fatalf("explicit unlink retained node link %q", got)
	}
}

// A different provider ID is not permission to redirect an operator's intent.
// Keep the original selection dormant until that ID returns or explicit unlink.
func TestManualHostLinkDurableIntent(t *testing.T) {
	dir := t.TempDir()
	newMonitor := func() *Monitor {
		m := issue1654Monitor()
		m.hostContinuityStore = config.NewHostContinuityStore(dir, nil)
		return m
	}
	m := newMonitor()
	report := issue1654Report(time.Now().UTC())
	report.Host.Hostname, report.Host.ReportIP, report.Network = "nas.example", "", nil
	host, err := m.ApplyHostReport(report, nil)
	if err != nil {
		t.Fatal(err)
	}
	node := models.Node{ID: "selected", Name: "pve", Instance: "cluster"}
	m.state.UpdateNodesForInstance("cluster", []models.Node{node})
	if err := m.LinkHostAgent(host.ID, node.ID); err != nil {
		t.Fatal(err)
	}
	// Restart immediately, with no intervening report to accidentally save intent.
	m = newMonitor()
	for i := 0; i < 3; i++ {
		provider := node
		if i == 1 {
			provider.ID = "replacement"
		}
		m.state.UpdateNodesForInstance("cluster", []models.Node{provider})
		report.Timestamp = report.Timestamp.Add(time.Second)
		host, err = m.ApplyHostReport(report, nil)
		if err != nil {
			t.Fatal(err)
		}
		m.state.UpdateNodesForInstance("cluster", []models.Node{provider})
		if got := m.state.GetHosts()[0]; got.LinkedNodeID != node.ID || got.NodeLinkSource != "manual" {
			t.Fatalf("step %d lost intent: %+v", i, got)
		}
		want := host.ID
		if i == 1 {
			want = ""
		}
		if got := m.state.GetSnapshot().Nodes[0].LinkedAgentID; got != want {
			t.Fatalf("step %d reverse = %q, want %q", i, got, want)
		}
	}
	if err := m.UnlinkHostAgent(host.ID); err != nil {
		t.Fatal(err)
	}
	m = newMonitor()
	// Positive matching evidence must not undo an explicit unlink.
	report.Host.Hostname = node.Name
	for i := 0; i < 2; i++ {
		m.state.UpdateNodesForInstance("cluster", []models.Node{node})
		report.Timestamp = report.Timestamp.Add(time.Second)
		host, err = m.ApplyHostReport(report, nil)
		if err != nil {
			t.Fatal(err)
		}
		if host.LinkedNodeID != "" || host.NodeLinkSource != "unlinked" {
			t.Fatalf("explicit unlink undone: %+v", host)
		}
		if got := m.state.GetSnapshot().Nodes[0].LinkedAgentID; got != "" {
			t.Fatalf("reverse = %q", got)
		}
	}
}

func TestHostLinkProvenanceCleanupAcrossRestart(t *testing.T) {
	for _, source := range []string{"automatic", ""} {
		t.Run("source="+source, func(t *testing.T) {
			dir := t.TempDir()
			m := issue1654Monitor()
			m.hostContinuityStore = config.NewHostContinuityStore(dir, nil)
			report := issue1654Report(time.Now().UTC())
			report.Host.Hostname, report.Host.ReportIP, report.Network = "nas.example", "", nil
			host, err := m.ApplyHostReport(report, nil)
			if err != nil {
				t.Fatal(err)
			}
			node := models.Node{ID: "pve", Name: "pve", Instance: "cluster", LinkedAgentID: host.ID}
			host.LinkedNodeID, host.NodeLinkSource = node.ID, source
			m.state.UpsertHost(host)
			m.state.UpdateNodesForInstance("cluster", []models.Node{node})
			entry, _ := m.hostContinuityStore.Get(host.ID)
			entry.LinkedNodeID, entry.NodeLinkSource = node.ID, source
			if err := m.hostContinuityStore.Upsert(entry); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 2; i++ {
				if i == 1 {
					m = issue1654Monitor()
					m.hostContinuityStore = config.NewHostContinuityStore(dir, nil)
					m.state.UpdateNodesForInstance("cluster", []models.Node{node})
				}
				report.Timestamp = report.Timestamp.Add(time.Second)
				host, err = m.ApplyHostReport(report, nil)
				if err != nil {
					t.Fatal(err)
				}
				want := ""
				if source == "" {
					want = node.ID
				}
				if host.LinkedNodeID != want {
					t.Fatalf("step %d link = %q, want %q", i, host.LinkedNodeID, want)
				}
				reverse := ""
				if want != "" {
					reverse = host.ID
				}
				if got := m.state.GetSnapshot().Nodes[0].LinkedAgentID; got != reverse {
					t.Fatalf("reverse = %q, want %q", got, reverse)
				}
			}
		})
	}
}

func TestHostLinkIntentPersistenceFailure(t *testing.T) {
	dir := t.TempDir()
	m := issue1654Monitor()
	m.hostContinuityStore = config.NewHostContinuityStore(dir, nil)
	report := issue1654Report(time.Now().UTC())
	host, err := m.ApplyHostReport(report, nil)
	if err != nil {
		t.Fatal(err)
	}
	node := models.Node{ID: "selected", Name: "pve", Instance: "cluster"}
	m.state.UpdateNodesForInstance("cluster", []models.Node{node})
	// Replace the directory with a file after the store has loaded successfully.
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir, []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := m.LinkHostAgent(host.ID, node.ID); err == nil {
		t.Fatal("link acknowledged without persistence")
	}
	if got := m.state.GetHosts()[0].LinkedNodeID; got != "" {
		t.Fatalf("failed link changed state: %q", got)
	}
	entry, _ := m.hostContinuityStore.Get(host.ID)
	if entry.LinkedNodeID != "" {
		t.Fatal("failed link changed journal memory")
	}
	if err := os.Remove(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := m.LinkHostAgent(host.ID, node.ID); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir, []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := m.UnlinkHostAgent(host.ID); err == nil {
		t.Fatal("unlink acknowledged without persistence")
	}
	if got := m.state.GetHosts()[0].LinkedNodeID; got != node.ID {
		t.Fatalf("failed unlink changed state: %q", got)
	}
	if got := m.state.GetSnapshot().Nodes[0].LinkedAgentID; got != host.ID {
		t.Fatal("failed unlink changed reverse link")
	}
	entry, _ = m.hostContinuityStore.Get(host.ID)
	if entry.LinkedNodeID != node.ID || entry.NodeLinkSource != "manual" {
		t.Fatal("failed unlink changed journal memory")
	}
}

func TestManualHostLinkConcurrentReportsAndRefresh(t *testing.T) {
	m := issue1654Monitor()
	m.hostContinuityStore = config.NewHostContinuityStore(t.TempDir(), nil)
	report := issue1654Report(time.Now().UTC())
	report.Host.Hostname, report.Host.ReportIP, report.Network = "nas.example", "", nil
	host, err := m.ApplyHostReport(report, nil)
	if err != nil {
		t.Fatal(err)
	}
	node := models.Node{ID: "selected", Name: "pve", Instance: "cluster"}
	m.state.UpdateNodesForInstance("cluster", []models.Node{node})
	if err := m.LinkHostAgent(host.ID, node.ID); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		for i := 0; i < 20; i++ {
			report.Timestamp = report.Timestamp.Add(time.Second)
			if _, err := m.ApplyHostReport(report, nil); err != nil {
				done <- err
				return
			}
			m.state.UpdateNodesForInstance("cluster", []models.Node{node})
		}
		done <- nil
	}()
	for i := 0; i < 20; i++ {
		if err := m.UnlinkHostAgent(host.ID); err != nil {
			t.Error(err)
			break
		}
		if err := m.LinkHostAgent(host.ID, node.ID); err != nil {
			t.Error(err)
			break
		}
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	entry, _ := m.hostContinuityStore.Get(host.ID)
	live := m.state.GetHosts()[0]
	if live.NodeLinkSource != "manual" || live.LinkedNodeID != node.ID ||
		entry.NodeLinkSource != "manual" || entry.LinkedNodeID != node.ID ||
		m.state.GetSnapshot().Nodes[0].LinkedAgentID != host.ID {
		t.Fatal("concurrent reports or provider refresh overwrote committed intent")
	}
}

func TestManualHostLinkReservesNodeBeforeOwnerReconnects(t *testing.T) {
	dir := t.TempDir()
	m := issue1654Monitor()
	m.hostContinuityStore = config.NewHostContinuityStore(dir, nil)
	if err := m.hostContinuityStore.SetNodeLinkIntents([]config.HostContinuityEntry{{
		HostID: "dormant-owner", LinkedNodeID: "selected", NodeLinkSource: "manual",
	}}); err != nil {
		t.Fatal(err)
	}
	m.hostContinuityStore = config.NewHostContinuityStore(dir, nil)
	m.state.UpdateNodesForInstance("cluster", []models.Node{{ID: "selected", Name: "pve", Instance: "cluster"}})
	report := issue1654Report(time.Now().UTC())
	report.Host.Hostname = "pve"
	host, err := m.ApplyHostReport(report, nil)
	if err != nil {
		t.Fatal(err)
	}
	if host.LinkedNodeID != "" || m.state.GetSnapshot().Nodes[0].LinkedAgentID != "" {
		t.Fatal("automatic match stole operator reservation")
	}
}
