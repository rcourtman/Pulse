package monitoring

import (
	"net/netip"
	"strings"
	"unicode"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

// The most default gateways and DNS servers kept from one host report, and
// the longest interface name. A host has one or two of each; the bounds only
// stop a wrong or hostile report from growing the model.
const (
	maxHostDefaultGateways    = 8
	maxHostDNSServers         = 8
	maxHostRouteInterfaceName = 64
)

// hostRoutingFromReport keeps what a host agent read from the host's own
// route table and resolver configuration. Every address must parse as an IP
// address and is stored in canonical form, so nothing but addresses and an
// interface name can arrive through these fields. A report without the block
// (an older agent, or a platform the agent cannot read it on) yields nil for
// both, which means unknown and never "no gateway" or "no DNS".
func hostRoutingFromReport(routing *agentshost.NetworkRouting) ([]models.HostDefaultGateway, []string) {
	if routing == nil {
		return nil, nil
	}

	var gateways []models.HostDefaultGateway
	seenGateway := make(map[models.HostDefaultGateway]struct{})
	for _, reported := range routing.DefaultGateways {
		address, ok := canonicalRouteAddress(reported.Address)
		if !ok {
			continue
		}
		gateway := models.HostDefaultGateway{Address: address, Interface: routeInterfaceName(reported.Interface)}
		if _, dup := seenGateway[gateway]; dup {
			continue
		}
		seenGateway[gateway] = struct{}{}
		gateways = append(gateways, gateway)
		if len(gateways) == maxHostDefaultGateways {
			break
		}
	}

	var servers []string
	seenServer := make(map[string]struct{})
	for _, reported := range routing.DNSServers {
		address, ok := canonicalRouteAddress(reported)
		if !ok {
			continue
		}
		if _, dup := seenServer[address]; dup {
			continue
		}
		seenServer[address] = struct{}{}
		servers = append(servers, address)
		if len(servers) == maxHostDNSServers {
			break
		}
	}
	return gateways, servers
}

func canonicalRouteAddress(raw string) (string, bool) {
	address, err := netip.ParseAddr(strings.TrimSpace(raw))
	if err != nil || address.IsUnspecified() {
		return "", false
	}
	return address.String(), true
}

// routeInterfaceName keeps a plausible interface name and drops anything
// else. The gateway address stands without it.
func routeInterfaceName(raw string) string {
	name := strings.TrimSpace(raw)
	if name == "" || len(name) > maxHostRouteInterfaceName {
		return ""
	}
	if strings.IndexFunc(name, func(r rune) bool { return unicode.IsControl(r) || unicode.IsSpace(r) }) >= 0 {
		return ""
	}
	return name
}
