package chat

import (
	"context"
	"fmt"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/approval"
)

// AnswerQuestion provides an answer to a pending question.
func (a *AgenticLoop) AnswerQuestion(questionID string, answers []QuestionAnswer) error {
	a.mu.Lock()
	ch, exists := a.pendingQs[questionID]
	a.mu.Unlock()

	if !exists {
		return fmt.Errorf("no pending question with ID: %s", questionID)
	}

	// Non-blocking send
	select {
	case ch <- answers:
		return nil
	default:
		return fmt.Errorf("question already answered: %s", questionID)
	}
}

// waitForApprovalDecision polls for an approval decision.
func waitForApprovalDecision(ctx context.Context, store *approval.Store, approvalID string) (*approval.ApprovalRequest, error) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			req, ok := store.GetApproval(approvalID)
			if !ok {
				return nil, fmt.Errorf("approval request not found: %s", approvalID)
			}
			if req.Status != approval.StatusPending {
				return req, nil
			}
		}
	}
}
