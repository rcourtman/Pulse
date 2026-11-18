package monitoring

import (
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

func routingTestReport(id string, routing *agentshost.NetworkRouting) agentshost.Report {
	return agentshost.Report{
		Agent:     agentshost.AgentInfo{ID: id, Version: "test", IntervalSeconds: 30},
		Host:      agentshost.HostInfo{ID: id, MachineID: "machine-" + id, Hostname: id, Platform: "linux"},
		Routing:   routing,
		Timestamp: time.Now().UTC(),
	}
}

// The gateway and resolvers a host agent read from the host reach the model,
// and nothing but addresses and an interface name can come in with them.
func TestApplyHostReportCarriesDefaultGatewayAndDNSServers(t *testing.T) {
	monitor := newTestMonitor(t)
	host, err := monitor.ApplyHostReport(routingTestReport("tower", &agentshost.NetworkRouting{
		DefaultGateways: []agentshost.NetworkGateway{
			{Address: " 192.168.0.1 ", Interface: "vmbr0"},
			{Address: "192.168.0.1", Interface: "vmbr0"},
			{Address: "not an address", Interface: "eth9"},
			{Address: "0.0.0.0", Interface: "eth8"},
			{Address: "10.10.1.1", Interface: "bad name\n"},
			{Address: "10.10.2.1", Interface: strings.Repeat("x", maxHostRouteInterfaceName+1)},
		},
		DNSServers: []string{"100.100.100.100", "FD7A:115C:A1E0::53", "100.100.100.100", "resolver.example.test", "::"},
	}), nil)
	if err != nil {
		t.Fatalf("apply report: %v", err)
	}

	wantGateways := []models.HostDefaultGateway{
		{Address: "192.168.0.1", Interface: "vmbr0"},
		{Address: "10.10.1.1"},
		{Address: "10.10.2.1"},
	}
	if !reflect.DeepEqual(host.DefaultGateways, wantGateways) {
		t.Fatalf("gateways = %+v, want %+v", host.DefaultGateways, wantGateways)
	}
	wantServers := []string{"100.100.100.100", "fd7a:115c:a1e0::53"}
	if !reflect.DeepEqual(host.DNSServers, wantServers) {
		t.Fatalf("DNS servers = %v, want %v", host.DNSServers, wantServers)
	}
}

// An agent older than the field, or one on a platform it cannot read the
// route table on, sends no block. That is unknown, not "no gateway".
func TestApplyHostReportLeavesRoutingUnknownWhenNotReported(t *testing.T) {
	monitor := newTestMonitor(t)
	host, err := monitor.ApplyHostReport(routingTestReport("mac-mini", nil), nil)
	if err != nil {
		t.Fatalf("apply report: %v", err)
	}
	if host.DefaultGateways != nil || host.DNSServers != nil {
		t.Fatalf("routing must stay nil when not reported: %+v %v", host.DefaultGateways, host.DNSServers)
	}
}

func TestHostRoutingFromReportBoundsBothLists(t *testing.T) {
	routing := &agentshost.NetworkRouting{}
	for i := 1; i <= 40; i++ {
		address := "10.0.0." + strconv.Itoa(i)
		routing.DefaultGateways = append(routing.DefaultGateways, agentshost.NetworkGateway{Address: address, Interface: "eth0"})
		routing.DNSServers = append(routing.DNSServers, address)
	}
	gateways, servers := hostRoutingFromReport(routing)
	if len(gateways) != maxHostDefaultGateways || len(servers) != maxHostDNSServers {
		t.Fatalf("kept %d gateways and %d servers, want %d and %d", len(gateways), len(servers), maxHostDefaultGateways, maxHostDNSServers)
	}
}
