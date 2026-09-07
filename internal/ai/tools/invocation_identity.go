package tools

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

type invocationScopeContextKey struct{}

// WithInvocationScope binds provider tool IDs to a trusted persisted user turn.
// The scope is transport context and is never accepted from model arguments.
func WithInvocationScope(ctx context.Context, sessionID, messageID string) context.Context {
	scope, _ := json.Marshal([]string{sessionID, messageID})
	return context.WithValue(ctx, invocationScopeContextKey{}, string(scope))
}

func actionRequestIDForInvocation(ctx context.Context) string {
	scope, _ := ctx.Value(invocationScopeContextKey{}).(string)
	invocation := InvocationIDFromContext(ctx)
	if scope == "" || invocation == "" {
		// Direct internal callers without a replay identity request a fresh plan.
		return uuid.NewString()
	}
	identity, _ := json.Marshal([]string{scope, invocation})
	return fmt.Sprintf("assistant:%x", sha256.Sum256(identity))
}
