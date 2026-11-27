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

// Kernel TCP socket tables on Linux. On other platforms these paths do not
// exist and collectActivePeers returns nil.
const (
	procNetTCPPath  = "/proc/net/tcp"
	procNetTCP6Path = "/proc/net/tcp6"
)

// TCP socket states from <net/tcp_states.h>.
const (
	tcpStateEstablished = "01"
	tcpStateListen      = "0A"
)

// maxActivePeers caps the number of unique internal peer pairs reported per
// host snapshot so busy hosts cannot grow the report payload without bound.
const maxActivePeers = 128

// cgnatPrefix covers Tailscale, WireGuard overlays, and RFC 6598 shared
// address space (100.64.0.0/10).
var cgnatPrefix = netip.MustParsePrefix("100.64.0.0/10")

type activePeerKey struct {
	remoteIP string
	port     uint16
	inbound  bool
}

// collectActivePeers passively reads established internal TCP peers from the
// Linux kernel socket tables. Public internet, loopback, link-local,
// multicast, and same-address peers are discarded here before transmission.
func collectActivePeers() []agentshost.NetworkPeer {
	return readActivePeersFrom(procNetTCPPath, procNetTCP6Path)
}

func readActivePeersFrom(tcpPath, tcp6Path string) []agentshost.NetworkPeer {
	var tables []struct {
		raw  string
		ipv6 bool
	}
	if raw, err := os.ReadFile(tcpPath); err == nil {
		tables = append(tables, struct {
			raw  string
			ipv6 bool
		}{raw: string(raw), ipv6: false})
	}
	if raw, err := os.ReadFile(tcp6Path); err == nil {
		tables = append(tables, struct {
			raw  string
			ipv6 bool
		}{raw: string(raw), ipv6: true})
	}
	if len(tables) == 0 {
		return nil
	}

	// Pass 1: collect local listening TCP ports so established rows can tell
	// whether this host is the server (inbound) or client (outbound) without
	// guessing from port number ranges.
	listeningPorts := make(map[uint16]struct{})
	for _, table := range tables {
		collectListeningPorts(table.raw, table.ipv6, listeningPorts)
	}

	peersByKey := make(map[activePeerKey]agentshost.NetworkPeer)
	for _, table := range tables {
		parseProcNetTCPTable(table.raw, table.ipv6, listeningPorts, peersByKey)
	}
	if len(peersByKey) == 0 {
		return nil
	}

	peers := make([]agentshost.NetworkPeer, 0, len(peersByKey))
	for _, peer := range peersByKey {
		peers = append(peers, peer)
	}
	sort.Slice(peers, func(i, j int) bool {
		if peers[i].Inbound != peers[j].Inbound {
			return !peers[i].Inbound
		}
		if peers[i].RemoteIP != peers[j].RemoteIP {
			return peers[i].RemoteIP < peers[j].RemoteIP
		}
		if peers[i].RemotePort != peers[j].RemotePort {
			return peers[i].RemotePort < peers[j].RemotePort
		}
		if peers[i].LocalPort != peers[j].LocalPort {
			return peers[i].LocalPort < peers[j].LocalPort
		}
		return peers[i].LocalIP < peers[j].LocalIP
	})
	if len(peers) > maxActivePeers {
		peers = peers[:maxActivePeers]
	}
	return peers
}

func collectListeningPorts(table string, ipv6 bool, out map[uint16]struct{}) {
	for index, line := range strings.Split(table, "\n") {
		if index == 0 {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 || !strings.EqualFold(fields[3], tcpStateListen) {
			continue
		}
		_, localPort, ok := parseProcSocketEndpoint(fields[1], ipv6)
		if !ok || localPort == 0 {
			continue
		}
		out[localPort] = struct{}{}
	}
}

func parseProcNetTCPTable(
	table string,
	ipv6 bool,
	listeningPorts map[uint16]struct{},
	out map[activePeerKey]agentshost.NetworkPeer,
) {
	for index, line := range strings.Split(table, "\n") {
		if index == 0 {
			continue
		}
		fields := strings.Fields(line)
		// sl local_address rem_address st ...
		if len(fields) < 4 || !strings.EqualFold(fields[3], tcpStateEstablished) {
			continue
		}
		localAddr, localPort, okLocal := parseProcSocketEndpoint(fields[1], ipv6)
		remoteAddr, remotePort, okRemote := parseProcSocketEndpoint(fields[2], ipv6)
		if !okLocal || !okRemote {
			continue
		}
		localAddr = localAddr.Unmap()
		remoteAddr = remoteAddr.Unmap()
		if !isInternalPeerAddr(remoteAddr) {
			continue
		}
		// Skip connections from a host to one of its own addresses.
		if localAddr == remoteAddr {
			continue
		}

		_, localIsListening := listeningPorts[localPort]
		if localIsListening {
			if localPort == 0 {
				continue
			}
			key := activePeerKey{
				remoteIP: remoteAddr.String(),
				port:     localPort,
				inbound:  true,
			}
			localIP := ""
			if isInternalPeerAddr(localAddr) {
				localIP = localAddr.String()
			}
			if existing, exists := out[key]; !exists || (localIP != "" && (existing.LocalIP == "" || localIP < existing.LocalIP)) {
				out[key] = agentshost.NetworkPeer{
					LocalIP:   localIP,
					LocalPort: localPort,
					RemoteIP:  remoteAddr.String(),
					Protocol:  "tcp",
					Inbound:   true,
				}
			}
			continue
		}

		if remotePort == 0 {
			continue
		}
		key := activePeerKey{
			remoteIP: remoteAddr.String(),
			port:     remotePort,
			inbound:  false,
		}
		localIP := ""
		if isInternalPeerAddr(localAddr) {
			localIP = localAddr.String()
		}
		if existing, exists := out[key]; !exists || (localIP != "" && (existing.LocalIP == "" || localIP < existing.LocalIP)) {
			out[key] = agentshost.NetworkPeer{
				LocalIP:    localIP,
				RemoteIP:   remoteAddr.String(),
				RemotePort: remotePort,
				Protocol:   "tcp",
			}
		}
	}
}

func parseProcSocketEndpoint(field string, ipv6 bool) (netip.Addr, uint16, bool) {
	addrHex, portHex, ok := strings.Cut(field, ":")
	if !ok {
		return netip.Addr{}, 0, false
	}
	portVal, err := strconv.ParseUint(portHex, 16, 16)
	if err != nil {
		return netip.Addr{}, 0, false
	}
	if !ipv6 {
		addr, ok := parseProcIPv4Hex(addrHex)
		return addr, uint16(portVal), ok
	}
	addr, ok := parseProcIPv6Hex(addrHex)
	return addr, uint16(portVal), ok
}

func parseProcIPv4Hex(addrHex string) (netip.Addr, bool) {
	if len(addrHex) != 8 {
		return netip.Addr{}, false
	}
	raw, err := hex.DecodeString(addrHex)
	if err != nil || len(raw) != 4 {
		return netip.Addr{}, false
	}
	return netip.AddrFrom4([4]byte{raw[3], raw[2], raw[1], raw[0]}), true
}

// parseProcIPv6Hex decodes a 32-hex-digit /proc/net/tcp6 address, stored as
// four little-endian 32-bit words on little-endian Linux hosts.
func parseProcIPv6Hex(addrHex string) (netip.Addr, bool) {
	if len(addrHex) != 32 {
		return netip.Addr{}, false
	}
	raw, err := hex.DecodeString(addrHex)
	if err != nil || len(raw) != 16 {
		return netip.Addr{}, false
	}
	var out [16]byte
	for word := 0; word < 4; word++ {
		base := word * 4
		out[base+0] = raw[base+3]
		out[base+1] = raw[base+2]
		out[base+2] = raw[base+1]
		out[base+3] = raw[base+0]
	}
	return netip.AddrFrom16(out), true
}

// isInternalPeerAddr returns true only for private RFC 1918, IPv6 ULA
// (fc00::/7), and RFC 6598 CGNAT / Tailscale overlay (100.64.0.0/10) unicast
// addresses. Public internet, loopback, unspecified, link-local, and multicast
// addresses always return false.
func isInternalPeerAddr(addr netip.Addr) bool {
	if !addr.IsValid() {
		return false
	}
	addr = addr.Unmap()
	if addr.IsUnspecified() ||
		addr.IsLoopback() ||
		addr.IsMulticast() ||
		addr.IsLinkLocalUnicast() ||
		addr.IsLinkLocalMulticast() ||
		addr.IsInterfaceLocalMulticast() {
		return false
	}
	return addr.IsPrivate() || cgnatPrefix.Contains(addr)
}
