package monitoring

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// An unmatched report is not an operator request to unlink. In particular,
// repairing old automatic associations must not erase a manual association.
func TestManualHostLinkSurvivesUnmatchedReportsAndProviderRefresh(t *testing.T) {
	monitor := issue1654Monitor()
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
