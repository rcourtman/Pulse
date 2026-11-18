package hostmetrics

import (
	"encoding/hex"
	"net/netip"
	"os"
	"sort"
	"strconv"
	"strings"

	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

// Where a host says how it reaches other networks and resolves names. The
// route table is Linux procfs; elsewhere it is absent and no gateway is
// reported. resolv.conf exists on every Unix the agent runs on.
const (
	procNetRoutePath      = "/proc/net/route"
	resolvConfPath        = "/etc/resolv.conf"
	resolvedUpstreamsPath = "/run/systemd/resolve/resolv.conf"
)

// The most of each the agent reports. A host has one or two of either; the
// bounds only stop a broken file from growing the report.
const (
	maxDefaultGateways = 8
	maxDNSServers      = 8
)

// Kernel route flags from <linux/route.h>.
const (
	routeFlagUp      = 0x1
	routeFlagGateway = 0x2
)

// collectRouting reads the host's IPv4 default gateways and its DNS servers.
// It returns nil when the host states neither, so a platform the agent cannot
// read sends nothing instead of an empty answer that looks like "no gateway".
func collectRouting() *agentshost.NetworkRouting {
	return readRoutingFrom(procNetRoutePath, resolvConfPath, resolvedUpstreamsPath)
}

func readRoutingFrom(routePath, resolvPath, upstreamsPath string) *agentshost.NetworkRouting {
	var routing agentshost.NetworkRouting
	if raw, err := os.ReadFile(routePath); err == nil {
		routing.DefaultGateways = parseProcNetRoute(string(raw))
	}
	if raw, err := os.ReadFile(resolvPath); err == nil {
		routing.DNSServers = parseResolvConf(string(raw))
	}
	// systemd-resolved points resolv.conf at its own stub on loopback and
	// keeps the servers it forwards to in a second file. The stub says
	// nothing about where names are resolved, so the forwarders are reported
	// when that file names any. A host that really runs its own resolver on
	// loopback has no such file and keeps the loopback address.
	if len(routing.DNSServers) > 0 && allLoopback(routing.DNSServers) {
		if raw, err := os.ReadFile(upstreamsPath); err == nil {
			if upstream := parseResolvConf(string(raw)); len(upstream) > 0 {
				routing.DNSServers = upstream
			}
		}
	}
	if len(routing.DefaultGateways) == 0 && len(routing.DNSServers) == 0 {
		return nil
	}
	return &routing
}

// parseProcNetRoute returns the IPv4 default routes that go through a gateway,
// lowest metric first. A default route with no gateway (a point-to-point
// link) names no next hop and is left out.
func parseProcNetRoute(table string) []agentshost.NetworkGateway {
	type route struct {
		gateway agentshost.NetworkGateway
		metric  int
	}
	var routes []route
	seen := make(map[agentshost.NetworkGateway]struct{})
	for index, line := range strings.Split(table, "\n") {
		fields := strings.Fields(line)
		// Iface Destination Gateway Flags RefCnt Use Metric Mask ...
		if index == 0 || len(fields) < 8 {
			continue
		}
		if fields[1] != "00000000" || fields[7] != "00000000" {
			continue
		}
		flags, err := strconv.ParseUint(fields[3], 16, 32)
		if err != nil || flags&routeFlagUp == 0 || flags&routeFlagGateway == 0 {
			continue
		}
		address, ok := procRouteAddress(fields[2])
		if !ok {
			continue
		}
		gateway := agentshost.NetworkGateway{Address: address, Interface: fields[0]}
		if _, dup := seen[gateway]; dup {
			continue
		}
		seen[gateway] = struct{}{}
		metric, _ := strconv.Atoi(fields[6])
		routes = append(routes, route{gateway: gateway, metric: metric})
	}
	sort.SliceStable(routes, func(i, j int) bool { return routes[i].metric < routes[j].metric })
	if len(routes) > maxDefaultGateways {
		routes = routes[:maxDefaultGateways]
	}
	gateways := make([]agentshost.NetworkGateway, 0, len(routes))
	for _, entry := range routes {
		gateways = append(gateways, entry.gateway)
	}
	if len(gateways) == 0 {
		return nil
	}
	return gateways
}

// procRouteAddress decodes one address column of /proc/net/route: eight hex
// digits holding the address in the kernel's byte order, which is the wire
// order read back to front.
func procRouteAddress(column string) (string, bool) {
	raw, err := hex.DecodeString(column)
	if err != nil || len(raw) != 4 {
		return "", false
	}
	address := netip.AddrFrom4([4]byte{raw[3], raw[2], raw[1], raw[0]})
	if address.IsUnspecified() {
		return "", false
	}
	return address.String(), true
}

// parseResolvConf returns the nameserver addresses of a resolv.conf in the
// order the file lists them, each once, in canonical form.
func parseResolvConf(conf string) []string {
	var servers []string
	seen := make(map[string]struct{})
	for _, line := range strings.Split(conf, "\n") {
		if cut := strings.IndexAny(line, "#;"); cut >= 0 {
			line = line[:cut]
		}
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != "nameserver" {
			continue
		}
		address, err := netip.ParseAddr(fields[1])
		if err != nil || address.IsUnspecified() {
			continue
		}
		server := address.String()
		if _, dup := seen[server]; dup {
			continue
		}
		seen[server] = struct{}{}
		servers = append(servers, server)
		if len(servers) == maxDNSServers {
			break
		}
	}
	return servers
}

func allLoopback(servers []string) bool {
	for _, server := range servers {
		address, err := netip.ParseAddr(server)
		if err != nil || !address.IsLoopback() {
			return false
		}
	}
	return true
}
