package monitoring

import (
	"net/netip"
	"sort"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

const maxHostActivePeers = 128

var hostPeerCGNATPrefix = netip.MustParsePrefix("100.64.0.0/10")

// hostActivePeersFromReport validates and sanitizes the passive internal TCP
// peers reported by a host agent. Even though the agent filters out public
// internet and loopback addresses before sending, the server enforces the same
// private-address invariant and payload bound on receipt.
func hostActivePeersFromReport(reported []agentshost.NetworkPeer) []models.HostNetworkPeer {
	if len(reported) == 0 {
		return nil
	}

	type peerKey struct {
		remoteIP string
		port     uint16
		inbound  bool
	}
	seen := make(map[peerKey]models.HostNetworkPeer, min(len(reported), maxHostActivePeers))
	for _, peer := range reported {
		remoteIP, ok := parseInternalHostPeerIP(peer.RemoteIP)
		if !ok {
			continue
		}
		localIP, _ := parseInternalHostPeerIP(peer.LocalIP)
		if localIP != "" && localIP == remoteIP {
			continue
		}

		var servicePort uint16
		var normalized models.HostNetworkPeer
		if peer.Inbound {
			if peer.LocalPort == 0 {
				continue
			}
			servicePort = peer.LocalPort
			normalized = models.HostNetworkPeer{
				LocalIP:   localIP,
				LocalPort: peer.LocalPort,
				RemoteIP:  remoteIP,
				Protocol:  "tcp",
				Inbound:   true,
			}
		} else {
			if peer.RemotePort == 0 {
				continue
			}
			servicePort = peer.RemotePort
			normalized = models.HostNetworkPeer{
				LocalIP:    localIP,
				RemoteIP:   remoteIP,
				RemotePort: peer.RemotePort,
				Protocol:   "tcp",
			}
		}

		key := peerKey{
			remoteIP: remoteIP,
			port:     servicePort,
			inbound:  peer.Inbound,
		}
		if existing, exists := seen[key]; !exists || (normalized.LocalIP != "" && (existing.LocalIP == "" || normalized.LocalIP < existing.LocalIP)) {
			seen[key] = normalized
		}
	}

	if len(seen) == 0 {
		return nil
	}

	out := make([]models.HostNetworkPeer, 0, len(seen))
	for _, peer := range seen {
		out = append(out, peer)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Inbound != out[j].Inbound {
			return !out[i].Inbound
		}
		if out[i].RemoteIP != out[j].RemoteIP {
			return out[i].RemoteIP < out[j].RemoteIP
		}
		if out[i].RemotePort != out[j].RemotePort {
			return out[i].RemotePort < out[j].RemotePort
		}
		if out[i].LocalPort != out[j].LocalPort {
			return out[i].LocalPort < out[j].LocalPort
		}
		return out[i].LocalIP < out[j].LocalIP
	})
	if len(out) > maxHostActivePeers {
		out = out[:maxHostActivePeers]
	}
	return out
}

func parseInternalHostPeerIP(raw string) (string, bool) {
	addr, err := netip.ParseAddr(strings.TrimSpace(raw))
	if err != nil {
		return "", false
	}
	addr = addr.Unmap()
	if !addr.IsValid() ||
		addr.IsUnspecified() ||
		addr.IsLoopback() ||
		addr.IsMulticast() ||
		addr.IsLinkLocalUnicast() ||
		addr.IsLinkLocalMulticast() ||
		addr.IsInterfaceLocalMulticast() {
		return "", false
	}
	if !addr.IsPrivate() && !hostPeerCGNATPrefix.Contains(addr) {
		return "", false
	}
	return addr.String(), true
}
