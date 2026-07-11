package api

import (
	"context"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/actionlifecycle"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func actionDispatchTestContext(t *testing.T, actionID string) context.Context {
	t.Helper()
	attempt, err := unified.NewActionDispatchAttempt(actionID, time.Now())
	if err != nil {
		t.Fatalf("NewActionDispatchAttempt: %v", err)
	}
	return actionlifecycle.ContextWithCommittedDispatchAttempt(context.Background(), attempt)
}
