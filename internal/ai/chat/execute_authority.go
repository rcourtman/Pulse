package chat

import "context"

type executeAuthorityContextKey struct{}

// WithExecuteAuthority carries trusted transport authority into read-only
// projection endpoints. The value is server-authored and cannot originate in
// a model/provider payload.
func WithExecuteAuthority(ctx context.Context, allowed bool) context.Context {
	return context.WithValue(ctx, executeAuthorityContextKey{}, allowed)
}

func executeAuthorityFromContext(ctx context.Context) bool {
	allowed, _ := ctx.Value(executeAuthorityContextKey{}).(bool)
	return allowed
}

type hostCommandAuthorityContextKey struct{}

// WithHostCommandAuthority carries the caller's canonical action-execute
// capability into read-only projection endpoints, so a listed tool manifest
// matches what a chat turn for the same caller would offer. Server-authored.
func WithHostCommandAuthority(ctx context.Context, allowed bool) context.Context {
	return context.WithValue(ctx, hostCommandAuthorityContextKey{}, allowed)
}

// HostCommandAuthorityFromContext reports the host-command authority bound by
// WithHostCommandAuthority. Absent means false.
func HostCommandAuthorityFromContext(ctx context.Context) bool {
	allowed, _ := ctx.Value(hostCommandAuthorityContextKey{}).(bool)
	return allowed
}
