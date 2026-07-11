package api

import (
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/approval"
)

// verifyAndConsumeCommandAuthorization is the final server-owned authority
// boundary for approval-gated arbitrary agent commands. It validates the
// tenant and action identity before atomically consuming the exact
// command/target approval. Grant signing and WebSocket dispatch happen only
// after this function succeeds.
func verifyAndConsumeCommandAuthorization(req agentexec.CommandAuthorizationRequest) error {
	store := approval.GetStore()
	if store == nil {
		return fmt.Errorf("approval store is unavailable")
	}
	approvalReq, found := store.GetApproval(strings.TrimSpace(req.ApprovalID))
	if !found || approvalReq == nil {
		return fmt.Errorf("approval not found")
	}
	if !approval.BelongsToOrg(approvalReq, req.OrgID) {
		return fmt.Errorf("approval belongs to another org")
	}
	if approvalReq.Plan == nil || strings.TrimSpace(approvalReq.Plan.ActionID) == "" ||
		strings.TrimSpace(approvalReq.Plan.ActionID) != strings.TrimSpace(req.ActionID) {
		return fmt.Errorf("approval action does not match")
	}
	if approvalReq.Status != approval.StatusApproved {
		return fmt.Errorf("approval is not approved (status: %s)", approvalReq.Status)
	}
	if approvalReq.Consumed {
		return fmt.Errorf("approval has already been consumed")
	}
	if time.Now().After(approvalReq.ExpiresAt) {
		return fmt.Errorf("approval has expired")
	}
	if _, err := store.ConsumeApproval(req.ApprovalID, req.Command, req.TargetType, req.TargetID); err != nil {
		return err
	}
	return nil
}
