package chat

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/approval"
	"github.com/stretchr/testify/require"
)

func TestWaitForApprovalDecision_AllowsMatchingOrg(t *testing.T) {
	store, err := approval.NewStore(approval.StoreConfig{
		DataDir:            t.TempDir(),
		DisablePersistence: true,
	})
	require.NoError(t, err)

	req := &approval.ApprovalRequest{
		ID:      "approval-1",
		OrgID:   "org-a",
		Command: "uptime",
	}
	require.NoError(t, store.CreateApproval(req))
	_, err = store.Approve(req.ID, "tester")
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	decision, err := waitForApprovalDecision(ctx, store, req.ID, "org-a")
	require.NoError(t, err)
	require.Equal(t, approval.StatusApproved, decision.Status)
}

func TestWaitForApprovalDecision_RejectsCrossOrg(t *testing.T) {
	store, err := approval.NewStore(approval.StoreConfig{
		DataDir:            t.TempDir(),
		DisablePersistence: true,
	})
	require.NoError(t, err)

	req := &approval.ApprovalRequest{
		ID:      "approval-2",
		OrgID:   "org-a",
		Command: "uptime",
	}
	require.NoError(t, store.CreateApproval(req))
	_, err = store.Approve(req.ID, "tester")
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	start := time.Now()
	_, err = waitForApprovalDecision(ctx, store, req.ID, "org-b")
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "not found"))
	require.Less(t, time.Since(start), 700*time.Millisecond)
}
