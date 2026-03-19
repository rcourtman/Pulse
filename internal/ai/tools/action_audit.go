package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/approval"
	unifiedresources "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rs/zerolog/log"
)

func (e *PulseToolExecutor) executeCommandWithAudit(
	ctx context.Context,
	capabilityName string,
	resourceID string,
	approvalID string,
	requiresApproval bool,
	agentID string,
	payload agentexec.ExecuteCommandPayload,
	requestedBy string,
	reason string,
) (*agentexec.CommandResultPayload, error) {
	if e.agentServer == nil {
		return nil, fmt.Errorf("no agent server available")
	}

	actionID := uuid.NewString()
	requestCorrelationID := strings.TrimSpace(approvalID)
	if requestCorrelationID == "" {
		requestCorrelationID = actionID
	}

	now := time.Now().UTC()
	plan := unifiedresources.ActionPlan{
		ActionID:         actionID,
		RequestID:        requestCorrelationID,
		Allowed:          true,
		RequiresApproval: requiresApproval,
		ApprovalPolicy: func() unifiedresources.ActionApprovalLevel {
			if requiresApproval {
				return unifiedresources.ApprovalAdmin
			}
			return unifiedresources.ApprovalNone
		}(),
		PlannedAt:       now,
		ExpiresAt:       now.Add(5 * time.Minute),
		ResourceVersion: "",
		PolicyVersion:   "",
		PlanHash:        actionPlanHash(actionID, requestCorrelationID, capabilityName, resourceID, payload, reason),
		Message:         reason,
	}

	record := unifiedresources.ActionAuditRecord{
		ID:        actionID,
		CreatedAt: now,
		UpdatedAt: now,
		State:     unifiedresources.ActionStateExecuting,
		Request: unifiedresources.ActionRequest{
			RequestID:      requestCorrelationID,
			ResourceID:     strings.TrimSpace(resourceID),
			CapabilityName: capabilityName,
			Params: map[string]any{
				"command":     payload.Command,
				"targetType":  payload.TargetType,
				"targetId":    payload.TargetID,
				"agentId":     agentID,
				"approvalId":  approvalID,
				"requestedBy": requestedBy,
			},
			Reason:      reason,
			RequestedBy: requestedBy,
		},
		Plan: plan,
	}

	if approvalID != "" {
		if store := approval.GetStore(); store != nil {
			if req, ok := store.GetApproval(approvalID); ok && req != nil {
				record.Approvals = append(record.Approvals, unifiedresources.ActionApprovalRecord{
					Actor:     strings.TrimSpace(req.DecidedBy),
					Method:    unifiedresources.MethodAPI,
					Timestamp: approvalTimestamp(req),
					Outcome:   unifiedresources.OutcomeApproved,
					Reason:    strings.TrimSpace(req.Context),
				})
				if req.Status == approval.StatusDenied {
					record.Approvals[len(record.Approvals)-1].Outcome = unifiedresources.OutcomeRejected
				}
			}
		}
	}

	e.recordActionLifecycle(actionID, unifiedresources.ActionStatePlanned, requestedBy, reason)
	e.recordActionLifecycle(actionID, unifiedresources.ActionStateExecuting, requestedBy, fmt.Sprintf("dispatching command to agent %s", agentID))
	e.recordActionAudit(record)

	result, err := e.agentServer.ExecuteCommand(ctx, agentID, payload)
	finalState := unifiedresources.ActionStateCompleted
	finalMessage := "command completed"
	executionResult := &unifiedresources.ExecutionResult{Success: true}

	if err != nil {
		finalState = unifiedresources.ActionStateFailed
		finalMessage = err.Error()
		executionResult.Success = false
		executionResult.ErrorMessage = err.Error()
	} else {
		output := result.Stdout
		if strings.TrimSpace(result.Stderr) != "" {
			if output != "" {
				output += "\n"
			}
			output += result.Stderr
		}
		executionResult.Output = output
		if result.ExitCode != 0 {
			finalState = unifiedresources.ActionStateFailed
			finalMessage = fmt.Sprintf("exit code %d", result.ExitCode)
			executionResult.Success = false
			executionResult.ErrorMessage = finalMessage
		}
	}

	record.State = finalState
	record.UpdatedAt = time.Now().UTC()
	record.Result = executionResult
	e.recordActionAudit(record)
	e.recordActionLifecycle(actionID, finalState, requestedBy, finalMessage)

	return result, err
}

func (e *PulseToolExecutor) recordActionAudit(record unifiedresources.ActionAuditRecord) {
	if e == nil || e.actionAuditStore == nil {
		return
	}
	if err := e.actionAuditStore.RecordActionAudit(record); err != nil {
		log.Warn().
			Err(err).
			Str("action_id", record.ID).
			Str("resource_id", record.Request.ResourceID).
			Msg("failed to persist action audit")
	}
}

func (e *PulseToolExecutor) recordActionLifecycle(actionID string, state unifiedresources.ActionState, actor, message string) {
	if e == nil || e.actionAuditStore == nil || strings.TrimSpace(actionID) == "" {
		return
	}
	event := unifiedresources.ActionLifecycleEvent{
		ActionID:  actionID,
		Timestamp: time.Now().UTC(),
		State:     state,
		Actor:     actor,
		Message:   message,
	}
	if err := e.actionAuditStore.RecordActionLifecycleEvent(event); err != nil {
		log.Warn().
			Err(err).
			Str("action_id", actionID).
			Str("state", string(state)).
			Msg("failed to persist action lifecycle event")
	}
}

func actionPlanHash(actionID, requestID, capabilityName, resourceID string, payload agentexec.ExecuteCommandPayload, reason string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{
		actionID,
		requestID,
		capabilityName,
		strings.TrimSpace(resourceID),
		payload.Command,
		payload.TargetType,
		payload.TargetID,
		reason,
	}, "|")))
	return hex.EncodeToString(sum[:])
}

func approvalTimestamp(req *approval.ApprovalRequest) time.Time {
	if req == nil || req.DecidedAt == nil {
		return time.Now().UTC()
	}
	return req.DecidedAt.UTC()
}
