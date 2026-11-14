package proxytrust

import (
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
)

var (
	trustedProxyOnce  sync.Once
	trustedProxyCIDRs []*net.IPNet
)

// ClientIP resolves the client address only through explicitly trusted proxy
// hops. Untrusted peers cannot spoof audit or rate-limit identity with XFF,
// and a client behind a trusted proxy cannot either: the address comes from
// the right-most X-Forwarded-For hop that is not a trusted proxy, never from
// the client-controlled left of the list. X-Real-IP is used only when the
// trusted proxy sent no X-Forwarded-For entries.
func ClientIP(r *http.Request) string {
	if r == nil {
		return ""
	}

	remote := ExtractRemoteIP(r.RemoteAddr)
	if remote == "" {
		return ""
	}

	if IsTrustedProxyIP(remote) {
		if forwarded, present := forwardedClientIP(r.Header.Values("X-Forwarded-For")); present {
			if forwarded == "" {
				return remote
			}
			return forwarded
		}

		if realIP := strings.TrimSpace(strings.Trim(r.Header.Get("X-Real-IP"), "[]")); net.ParseIP(realIP) != nil {
			return realIP
		}
	}

	return remote
}

func ExtractRemoteIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return strings.Trim(remoteAddr, "[]")
	}
	return strings.Trim(host, "[]")
}

// forwardedClientIP reads the client address from X-Forwarded-For values sent
// by a trusted proxy. A proxy that appends keeps whatever the client sent on
// the left, so the client is the right-most hop that is not itself a trusted
// proxy; earlier entries are the client's own claims. Several header lines are
// read as one list, in order, because a proxy may add its hop as a new line
// after one the client sent.
//
// present is false when the header carries no entries at all, so the caller
// can fall back to X-Real-IP. An entry that is not an address stops the walk
// and returns "", because nothing to its left can be attributed; the caller
// then uses the proxy's own address. When every hop is trusted the left-most
// is returned.
func forwardedClientIP(values []string) (ip string, present bool) {
	var hops []string
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			if part = strings.TrimSpace(part); part != "" {
				hops = append(hops, part)
			}
		}
	}
	if len(hops) == 0 {
		return "", false
	}

	leftMost := ""
	for i := len(hops) - 1; i >= 0; i-- {
		hop := parseForwardedHop(hops[i])
		if hop == "" {
			return "", true
		}
		if !IsTrustedProxyIP(hop) {
			return hop, true
		}
		leftMost = hop
	}
	return leftMost, true
}

// parseForwardedHop accepts an X-Forwarded-For entry written as an address,
// a bracketed IPv6 address, or either with a port, and returns the address.
func parseForwardedHop(hop string) string {
	if ip := net.ParseIP(strings.Trim(hop, "[]")); ip != nil {
		return ip.String()
	}
	if host, _, err := net.SplitHostPort(hop); err == nil {
		if ip := net.ParseIP(host); ip != nil {
			return ip.String()
		}
	}
	return ""
}

func IsTrustedProxyIP(rawIP string) bool {
	ip := net.ParseIP(strings.Trim(rawIP, "[]"))
	if ip == nil {
		return false
	}

	trustedProxyOnce.Do(loadTrustedProxyCIDRs)
	if len(trustedProxyCIDRs) == 0 {
		return false
	}
	for _, network := range trustedProxyCIDRs {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

func loadTrustedProxyCIDRs() {
	raw := strings.TrimSpace(os.Getenv("CP_TRUSTED_PROXY_CIDRS"))
	if raw == "" {
		// Backward-compatible fallback to the shared setting used by the app server.
		raw = strings.TrimSpace(os.Getenv("PULSE_TRUSTED_PROXY_CIDRS"))
	}
	if raw == "" {
		return
	}

	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		if strings.Contains(entry, "/") {
			_, network, err := net.ParseCIDR(entry)
			if err != nil {
				continue
			}
			network.IP = network.IP.Mask(network.Mask)
			trustedProxyCIDRs = append(trustedProxyCIDRs, network)
			continue
		}

		ip := net.ParseIP(entry)
		if ip == nil {
			continue
		}
		bits := 32
		if ip.To4() == nil {
			bits = 128
		}
		mask := net.CIDRMask(bits, bits)
		trustedProxyCIDRs = append(trustedProxyCIDRs, &net.IPNet{
			IP:   ip.Mask(mask),
			Mask: mask,
		})
	}
}

func ResetForTesting() {
	trustedProxyOnce = sync.Once{}
	trustedProxyCIDRs = nil
}
