package actionlifecycle

import (
	"context"

	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

type dispatchAttemptContextKey struct{}

func withDispatchAttempt(ctx context.Context, attempt unified.ActionDispatchAttempt) context.Context {
	return context.WithValue(ctx, dispatchAttemptContextKey{}, attempt)
}

// ContextWithCommittedDispatchAttempt is exposed for executor conformance
// tests and transport adapters. Production callers must obtain attempt from a
// successful lifecycle-store admission before using it.
func ContextWithCommittedDispatchAttempt(ctx context.Context, attempt unified.ActionDispatchAttempt) context.Context {
	return withDispatchAttempt(ctx, attempt)
}

// DispatchAttemptFromContext returns the committed transport authority for an
// executor call. Executors must use its ID as the transport request identity.
func DispatchAttemptFromContext(ctx context.Context) (unified.ActionDispatchAttempt, bool) {
	attempt, ok := ctx.Value(dispatchAttemptContextKey{}).(unified.ActionDispatchAttempt)
	return attempt, ok
}
