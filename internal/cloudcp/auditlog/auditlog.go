package auditlog

import (
	"net/http"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/proxytrust"
)

// ClientIP resolves the best-effort client IP for audit metadata.
func ClientIP(r *http.Request) string {
	return proxytrust.ClientIP(r)
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
