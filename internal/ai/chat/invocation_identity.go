package chat

import (
	"context"

	"github.com/google/uuid"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
)

func withRunInvocationIdentity(ctx context.Context, sessionID string, messages []Message) context.Context {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" && messages[i].ID != "" {
			return tools.WithInvocationScope(ctx, sessionID, messages[i].ID)
		}
	}
	// An unpersisted internal run still has one scope for all provider retries.
	return tools.WithInvocationScope(ctx, sessionID, uuid.NewString())
}
