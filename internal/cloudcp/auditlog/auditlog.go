package auditlog

import (
	"context"
	"net/http"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/proxytrust"
)

// ClientIP resolves the best-effort client IP for audit metadata.
func ClientIP(r *http.Request) string {
	return proxytrust.ClientIP(r)
}

const (
	// ActorAdminKey is the audit actor for admin-key authenticated requests.
	ActorAdminKey = "admin_key"
	// ActorAnonymous is the audit actor for requests nothing authenticated.
	ActorAnonymous = "anonymous"
)

// actorKey is the context key for the authenticated audit actor.
type actorKey struct{}

// WithActor returns a copy of ctx carrying the actor that authentication
// middleware verified for the request.
func WithActor(ctx context.Context, actorID string) context.Context {
	return context.WithValue(ctx, actorKey{}, strings.TrimSpace(actorID))
}

// ActorID returns the actor that authentication middleware verified for the
// request, or ActorAnonymous when nothing authenticated it. Request headers
// are client-controlled and are never consulted.
func ActorID(r *http.Request) string {
	if r == nil {
		return ActorAnonymous
	}
	if v, ok := r.Context().Value(actorKey{}).(string); ok && v != "" {
		return v
	}
	return ActorAnonymous
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
