package auth

import "context"

// Authorizer defines the interface for making access control decisions.
type Authorizer interface {
	// Authorize checks if a subject (from context) can perform an action on a resource.
	// Returns true if allowed, false if denied, and an error if the check failed due to a system issue.
	Authorize(ctx context.Context, action string, resource string) (bool, error)
}

type contextKey string

const (
	contextKeyUser contextKey = "user"
)

// WithUser adds a username to the context
func WithUser(ctx context.Context, username string) context.Context {
	return context.WithValue(ctx, contextKeyUser, username)
}

// GetUser extracts the username from the context
func GetUser(ctx context.Context) string {
	if user, ok := ctx.Value(contextKeyUser).(string); ok {
		return user
	}
	return ""
}

// DefaultAuthorizer is a pass-through implementation that allows everything.
// Used in OSS version and when enterprise features are disabled.
type DefaultAuthorizer struct{}

func (d *DefaultAuthorizer) Authorize(ctx context.Context, action string, resource string) (bool, error) {
	return true, nil
}

var globalAuthorizer Authorizer = &DefaultAuthorizer{}

// SetAuthorizer sets the global authorizer instance.
// This is used by pulse-enterprise to register the real RBAC implementation.
func SetAuthorizer(auth Authorizer) {
	globalAuthorizer = auth
}

// AdminConfigurable is an optional interface for authorizers that can have an admin user set.
type AdminConfigurable interface {
	SetAdminUser(username string)
}

// SetAdminUser sets the admin user on the global authorizer if it supports it.
func SetAdminUser(username string) {
	if username == "" {
		return
	}
	if configurable, ok := globalAuthorizer.(AdminConfigurable); ok {
		configurable.SetAdminUser(username)
	}
}

// GetAuthorizer returns the global authorizer instance.
func GetAuthorizer() Authorizer {
	return globalAuthorizer
}
