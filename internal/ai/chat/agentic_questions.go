package chat

import (
	"context"
	"fmt"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/approval"
)

// waitForApprovalDecision polls for an approval decision.
func waitForApprovalDecision(ctx context.Context, store *approval.Store, approvalID string, orgID string) (*approval.ApprovalRequest, error) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			req, ok := store.GetApproval(approvalID)
			if !ok || !approval.BelongsToOrg(req, orgID) {
				return nil, fmt.Errorf("approval request not found: %s", approvalID)
			}
			if req.Status != approval.StatusPending {
				return req, nil
			}
		}
	}
}
