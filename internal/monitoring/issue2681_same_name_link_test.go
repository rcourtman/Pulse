package monitoring

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

func issue2681VM(instance, address string) models.VM {
	vm := models.VM{
		ID: instance + ":pmx1:100", VMID: 100, Name: "docker1", Instance: instance, Node: "pmx1", Status: "running",
	}
	if address != "" {
		vm.IPAddresses = []string{address}
		vm.NetworkInterfaces = []models.GuestNetworkInterface{{Name: "eth0", Addresses: []string{address}}}
	}
	return vm
}

func issue2681AgentReport(timestamp time.Time, address string) agentshost.Report {
	report := issue1654Report(timestamp)
	report.Host.Hostname = "docker1"
	if address != "" {
		report.Network = []agentshost.NetworkInterface{{Name: "eth0", Addresses: []string{address + "/24"}}}
	}
	return report
}

// Adding an installation that reuses a guest name must not cost the guest
// already monitored the agent running inside it (#2681).
func TestApplyHostReportKeepsGuestLinkWhenAnotherInstallationReusesTheName(t *testing.T) {
	for _, tc := range []struct {
		name, homeAddress, hetznerAddress, reportAddress string
		wantLink                                         string
	}{
		{name: "nothing known about the guests' addresses", wantLink: "home:pmx1:100"},
		{name: "the report names no address", homeAddress: "192.0.2.10", hetznerAddress: "198.51.100.10", wantLink: "home:pmx1:100"},
		{name: "only the agent's guest has a known address", homeAddress: "192.0.2.10", reportAddress: "192.0.2.10", wantLink: "home:pmx1:100"},
		{name: "only the other guest has a known address", hetznerAddress: "198.51.100.10", reportAddress: "192.0.2.10", wantLink: "home:pmx1:100"},
		{name: "sites reuse a private range", homeAddress: "192.168.1.10", hetznerAddress: "192.168.1.10", reportAddress: "192.168.1.10", wantLink: "home:pmx1:100"},
		// A guest with no known addresses may hold this private one too.
		{name: "the other guest alone is known to hold a private address", hetznerAddress: "192.168.1.10", reportAddress: "192.168.1.10", wantLink: "home:pmx1:100"},
		{name: "the other guest alone is known to hold a shared-range address", hetznerAddress: "100.64.0.10", reportAddress: "100.64.0.10", wantLink: "home:pmx1:100"},
		{name: "the address belongs to the other guest", homeAddress: "192.0.2.10", hetznerAddress: "198.51.100.10", reportAddress: "198.51.100.10", wantLink: "hetzner:pmx1:100"},
		{name: "no guest holds the reported address", homeAddress: "192.0.2.10", hetznerAddress: "198.51.100.10", reportAddress: "203.0.113.10", wantLink: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			monitor := issue1654Monitor()
			now := time.Now().UTC()

			monitor.state.UpdateVMs([]models.VM{issue2681VM("home", tc.homeAddress)})
			host, err := monitor.ApplyHostReport(issue2681AgentReport(now, tc.reportAddress), nil)
			if err != nil {
				t.Fatal(err)
			}
			if host.LinkedVMID != "home:pmx1:100" {
				t.Fatalf("first report LinkedVMID = %q, want the only guest of that name", host.LinkedVMID)
			}

			monitor.state.UpdateVMs([]models.VM{
				issue2681VM("home", tc.homeAddress),
				issue2681VM("hetzner", tc.hetznerAddress),
			})
			host, err = monitor.ApplyHostReport(issue2681AgentReport(now.Add(time.Second), tc.reportAddress), nil)
			if err != nil {
				t.Fatal(err)
			}
			if host.LinkedVMID != tc.wantLink {
				t.Fatalf("LinkedVMID = %q, want %q", host.LinkedVMID, tc.wantLink)
			}
		})
	}
}

// An agent with no earlier link can still be placed by an address, and stays
// unplaced when nothing separates the guests.
func TestApplyHostReportLinksNewAgentToSameNamedGuestByAddress(t *testing.T) {
	for _, tc := range []struct {
		name, reportAddress, wantLink string
	}{
		{name: "address held by one guest", reportAddress: "198.51.100.10", wantLink: "hetzner:pmx1:100"},
		{name: "no address reported", wantLink: ""},
		{name: "address nobody holds", reportAddress: "203.0.113.10", wantLink: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			monitor := issue1654Monitor()
			monitor.state.UpdateVMs([]models.VM{
				issue2681VM("home", "192.0.2.10"),
				issue2681VM("hetzner", "198.51.100.10"),
			})
			host, err := monitor.ApplyHostReport(issue2681AgentReport(time.Now().UTC(), tc.reportAddress), nil)
			if err != nil {
				t.Fatal(err)
			}
			if host.LinkedVMID != tc.wantLink {
				t.Fatalf("LinkedVMID = %q, want %q", host.LinkedVMID, tc.wantLink)
			}
		})
	}
}

func TestApplyHostReportKeepsContainerLinkWhenAnotherInstallationReusesTheName(t *testing.T) {
	monitor := issue1654Monitor()
	now := time.Now().UTC()
	container := func(instance string) models.Container {
		return models.Container{
			ID: instance + ":pmx1:100", VMID: 100, Name: "docker1", Instance: instance, Node: "pmx1", Status: "running",
		}
	}

	monitor.state.UpdateContainers([]models.Container{container("home")})
	host, err := monitor.ApplyHostReport(issue2681AgentReport(now, ""), nil)
	if err != nil {
		t.Fatal(err)
	}
	if host.LinkedContainerID != "home:pmx1:100" {
		t.Fatalf("first report LinkedContainerID = %q", host.LinkedContainerID)
	}

	monitor.state.UpdateContainers([]models.Container{container("home"), container("hetzner")})
	host, err = monitor.ApplyHostReport(issue2681AgentReport(now.Add(time.Second), ""), nil)
	if err != nil {
		t.Fatal(err)
	}
	if host.LinkedContainerID != "home:pmx1:100" {
		t.Fatalf("LinkedContainerID = %q, want home:pmx1:100", host.LinkedContainerID)
	}
}

// A node added later under the same name leaves the agent on its node, where
// neither node's endpoint or addresses say otherwise.
func TestApplyHostReportKeepsNodeLinkWhenAnotherInstallationReusesTheName(t *testing.T) {
	monitor := issue1654Monitor()
	now := time.Now().UTC()
	node := func(instance string) models.Node {
		return models.Node{
			ID: instance + "-pmx1", Name: "pmx1", Instance: instance, Status: "online",
			Host: "https://pmx1." + instance + ".example:8006",
		}
	}
	report := func(timestamp time.Time) agentshost.Report {
		report := issue1654Report(timestamp)
		report.Host.Hostname = "pmx1"
		return report
	}

	monitor.state.UpdateNodes([]models.Node{node("home")})
	host, err := monitor.ApplyHostReport(report(now), nil)
	if err != nil {
		t.Fatal(err)
	}
	if host.LinkedNodeID != "home-pmx1" {
		t.Fatalf("first report LinkedNodeID = %q", host.LinkedNodeID)
	}

	monitor.state.UpdateNodes([]models.Node{node("home"), node("hetzner")})
	host, err = monitor.ApplyHostReport(report(now.Add(time.Second)), nil)
	if err != nil {
		t.Fatal(err)
	}
	if host.LinkedNodeID != "home-pmx1" {
		t.Fatalf("LinkedNodeID = %q, want home-pmx1", host.LinkedNodeID)
	}
}

// The earlier link holds when the guest it names has moved within its cluster,
// though its source ID carries the node it was on.
func TestApplyHostReportKeepsGuestLinkAcrossMigration(t *testing.T) {
	monitor := issue1654Monitor()
	now := time.Now().UTC()

	monitor.state.UpdateVMs([]models.VM{issue2681VM("home", "")})
	host, err := monitor.ApplyHostReport(issue2681AgentReport(now, ""), nil)
	if err != nil {
		t.Fatal(err)
	}
	if host.LinkedVMID != "home:pmx1:100" {
		t.Fatalf("first report LinkedVMID = %q", host.LinkedVMID)
	}

	migrated := issue2681VM("home", "")
	migrated.ID, migrated.Node = "home:pmx2:100", "pmx2"
	monitor.state.UpdateVMs([]models.VM{migrated, issue2681VM("hetzner", "")})
	host, err = monitor.ApplyHostReport(issue2681AgentReport(now.Add(time.Second), ""), nil)
	if err != nil {
		t.Fatal(err)
	}
	if host.LinkedVMID != "home:pmx2:100" {
		t.Fatalf("LinkedVMID = %q, want the same guest on its new node", host.LinkedVMID)
	}
}

// A VM added elsewhere under a container's name does not take the agent from
// the container: VMs and containers are candidates alike.
func TestApplyHostReportKeepsContainerLinkWhenAVMReusesTheName(t *testing.T) {
	monitor := issue1654Monitor()
	now := time.Now().UTC()

	monitor.state.UpdateContainers([]models.Container{{
		ID: "home:pmx1:100", VMID: 100, Name: "docker1", Instance: "home", Node: "pmx1", Status: "running",
	}})
	host, err := monitor.ApplyHostReport(issue2681AgentReport(now, ""), nil)
	if err != nil {
		t.Fatal(err)
	}
	if host.LinkedContainerID != "home:pmx1:100" {
		t.Fatalf("first report LinkedContainerID = %q", host.LinkedContainerID)
	}

	monitor.state.UpdateVMs([]models.VM{issue2681VM("hetzner", "")})
	host, err = monitor.ApplyHostReport(issue2681AgentReport(now.Add(time.Second), ""), nil)
	if err != nil {
		t.Fatal(err)
	}
	if host.LinkedContainerID != "home:pmx1:100" || host.LinkedVMID != "" {
		t.Fatalf("links container=%q vm=%q, want the container only", host.LinkedContainerID, host.LinkedVMID)
	}
}

// A private address does not place a new agent while another same-named guest's
// addresses are unknown: that guest may hold it too.
func TestApplyHostReportDoesNotPlaceNewAgentOnPrivateAddressAlone(t *testing.T) {
	monitor := issue1654Monitor()
	monitor.state.UpdateVMs([]models.VM{
		issue2681VM("home", "192.168.1.10"),
		issue2681VM("hetzner", ""),
	})
	host, err := monitor.ApplyHostReport(issue2681AgentReport(time.Now().UTC(), "192.168.1.10"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if host.LinkedVMID != "" {
		t.Fatalf("LinkedVMID = %q, want no link", host.LinkedVMID)
	}
}

// The only guest with a name keeps its link whatever its known addresses say. An
// LXC's configured address stops being refreshed once it is set, so it can differ
// from the agent's report for as long as the guest runs.
func TestApplyHostReportKeepsOnlySameNamedGuestWithStaleAddresses(t *testing.T) {
	monitor := issue1654Monitor()
	monitor.state.UpdateVMs([]models.VM{issue2681VM("home", "192.0.2.10")})
	host, err := monitor.ApplyHostReport(issue2681AgentReport(time.Now().UTC(), "192.0.2.99"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if host.LinkedVMID != "home:pmx1:100" {
		t.Fatalf("LinkedVMID = %q, want the only guest of that name", host.LinkedVMID)
	}
}
