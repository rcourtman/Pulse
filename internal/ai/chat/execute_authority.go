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
