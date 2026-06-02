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
// hops. Untrusted peers cannot spoof audit or rate-limit identity with XFF.
func ClientIP(r *http.Request) string {
	if r == nil {
		return ""
	}

	remote := ExtractRemoteIP(r.RemoteAddr)
	if remote == "" {
		return ""
	}

	if IsTrustedProxyIP(remote) {
		if xff := rightMostUntrustedForwardedIP(r.Header.Get("X-Forwarded-For")); xff != "" {
			return xff
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

func rightMostUntrustedForwardedIP(header string) string {
	parts := strings.Split(header, ",")
	var leftMostValid string
	for i := len(parts) - 1; i >= 0; i-- {
		candidate := strings.TrimSpace(strings.Trim(parts[i], "[]"))
		if net.ParseIP(candidate) == nil {
			continue
		}
		leftMostValid = candidate
		if !IsTrustedProxyIP(candidate) {
			return candidate
		}
	}
	return leftMostValid
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
