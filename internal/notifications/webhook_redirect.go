package notifications

import (
	"errors"
	"net"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
)

const notificationRedirectOriginMessage = "notification redirects must stay on the configured origin; use the final destination URL"

var errNotificationRedirectOrigin = errors.New(notificationRedirectOriginMessage)

// Compare HTTP origins, not IP resolution, parent domains or just hostnames.
// Another port is another receiver even on the same host. URL paths and query
// strings may change, but HTTPS must never downgrade (nor HTTP upgrade silently).
func sameNotificationOrigin(initial, target *url.URL) bool {
	first, ok := notificationOrigin(initial)
	if !ok {
		return false
	}
	next, ok := notificationOrigin(target)
	return ok && first == next
}

func notificationOrigin(u *url.URL) (string, bool) {
	if u == nil || u.Opaque != "" || u.User != nil || u.Hostname() == "" {
		return "", false
	}
	scheme := strings.ToLower(u.Scheme)
	port := u.Port()
	switch scheme {
	case "http":
		if port == "" {
			port = "80"
		}
	case "https":
		if port == "" {
			port = "443"
		}
	default:
		return "", false
	}
	number, err := strconv.ParseUint(port, 10, 16)
	if err != nil || number == 0 {
		return "", false
	}
	host := u.Hostname()
	if address, err := netip.ParseAddr(host); err == nil {
		host = address.String() // Preserve case-sensitive IPv6 zone identity.
	} else {
		host = strings.ToLower(host)
	}
	return scheme + "://" + net.JoinHostPort(host, strconv.FormatUint(number, 10)), true
}
