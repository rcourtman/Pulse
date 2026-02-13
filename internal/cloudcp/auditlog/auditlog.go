package auditlog

import (
	"net"
	"net/http"
	"strings"
)

// ClientIP resolves the best-effort client IP for audit metadata.
func ClientIP(r *http.Request) string {
	if r == nil {
		return ""
	}

	if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
		if i := strings.IndexByte(xff, ','); i >= 0 {
			return strings.TrimSpace(xff[:i])
		}
		return xff
	}

	if rip := strings.TrimSpace(r.Header.Get("X-Real-IP")); rip != "" {
		return strings.Trim(rip, "[]")
	}

	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err != nil {
		return strings.TrimSpace(r.RemoteAddr)
	}
	return strings.TrimSpace(host)
}

// ActorID returns the request actor identifier from common headers.
func ActorID(r *http.Request) string {
	if r == nil {
		return ""
	}

	for _, header := range []string{"X-Actor-ID", "X-Actor-Id", "X-User-ID", "X-User-Id"} {
		if v := strings.TrimSpace(r.Header.Get(header)); v != "" {
			return v
		}
	}
	return ""
}

// RequestPath returns a stable request path for audit metadata.
func RequestPath(r *http.Request) string {
	if r == nil || r.URL == nil {
		return ""
	}
	if p := strings.TrimSpace(r.URL.Path); p != "" {
		return p
	}
	return "/"
}
