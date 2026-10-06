package proxmox

import (
	"net"
	"net/netip"
	"strconv"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
)

// Coordinate equivalent HTTP origins without changing their request URLs or
// TLS/proxy policy. Literal spelling is not a distinct serial QGA channel:
// DNS case, default ports and IPv6 compression can differ between clients.
// Keep scheme, non-default port, zone and base path distinct. Do not resolve
// DNS aliases or infer that different origins address the same PVE cluster.
func guestAgentEndpointKey(endpoint string) (string, bool) {
	u, err := securityutil.NormalizeHTTPBaseURL(endpoint, "")
	if err != nil {
		return "", false
	}
	host := u.Hostname()
	if address, err := netip.ParseAddr(host); err == nil {
		host = address.String() // Preserve IPv6 zones, including their case.
	} else {
		host = strings.ToLower(host)
	}
	port := u.Port()
	if port != "" {
		number, err := strconv.ParseUint(port, 10, 16)
		if err != nil || number == 0 {
			return "", false
		}
		port = strconv.FormatUint(number, 10)
		if (u.Scheme == "http" && port == "80") || (u.Scheme == "https" && port == "443") {
			port = ""
		}
	}
	if port != "" {
		u.Host = net.JoinHostPort(host, port)
	} else if strings.Contains(host, ":") {
		u.Host = "[" + host + "]"
	} else {
		u.Host = host
	}
	return u.String(), true
}
