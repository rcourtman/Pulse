package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/approval"
	unifiedresources "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rs/zerolog/log"
)

const approvalAuditActor = "pulse_assistant"

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
	approvalReq := approvalRequestForID(approvalID)
	planFromApproval := approvalReq != nil && approvalReq.Plan != nil
	if planFromApproval {
		if approvedActionID := strings.TrimSpace(approvalReq.Plan.ActionID); approvedActionID != "" {
			actionID = approvedActionID
		}
		if approvedRequestID := strings.TrimSpace(approvalReq.Plan.RequestID); approvedRequestID != "" {
			requestCorrelationID = approvedRequestID
		}
	}
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
		Preflight:       actionAuditPreflight(resourceID, reason, now),
	}
	if planFromApproval {
		plan = mergeApprovedActionPlan(*approvalReq.Plan, plan)
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
	record.Approvals = approvalRecordsForID(approvalID)

	if !planFromApproval {
		e.recordActionLifecycle(actionID, unifiedresources.ActionStatePlanned, requestedBy, reason)
	}
	e.recordActionLifecycle(actionID, unifiedresources.ActionStateExecuting, requestedBy, fmt.Sprintf("dispatching command to agent %s", agentID))
	e.recordActionAudit(record)

	payload.ApprovalID = strings.TrimSpace(approvalID)
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

func (e *PulseToolExecutor) executeNativeActionWithAudit(
	ctx context.Context,
	capabilityName string,
	resourceID string,
	approvalID string,
	requiresApproval bool,
	params map[string]any,
	requestedBy string,
	reason string,
	execute func(context.Context) (*unifiedresources.ExecutionResult, error),
) (*unifiedresources.ExecutionResult, error) {
	actionID := uuid.NewString()
	requestCorrelationID := strings.TrimSpace(approvalID)
	if requestCorrelationID == "" {
		requestCorrelationID = actionID
	}

	now := time.Now().UTC()
	approvalReq := approvalRequestForID(approvalID)
	planFromApproval := approvalReq != nil && approvalReq.Plan != nil
	if planFromApproval {
		if approvedActionID := strings.TrimSpace(approvalReq.Plan.ActionID); approvedActionID != "" {
			actionID = approvedActionID
		}
		if approvedRequestID := strings.TrimSpace(approvalReq.Plan.RequestID); approvedRequestID != "" {
			requestCorrelationID = approvedRequestID
		}
	}
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
		PlanHash:        actionPlanHashForParams(actionID, requestCorrelationID, capabilityName, resourceID, params, reason),
		Message:         reason,
		Preflight:       actionAuditPreflight(resourceID, reason, now),
	}
	if planFromApproval {
		plan = mergeApprovedActionPlan(*approvalReq.Plan, plan)
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
			Params:         cloneActionParams(params),
			Reason:         reason,
			RequestedBy:    requestedBy,
		},
		Plan: plan,
	}
	record.Approvals = approvalRecordsForID(approvalID)

	if !planFromApproval {
		e.recordActionLifecycle(actionID, unifiedresources.ActionStatePlanned, requestedBy, reason)
	}
	e.recordActionLifecycle(actionID, unifiedresources.ActionStateExecuting, requestedBy, "dispatching native resource action")
	e.recordActionAudit(record)

	result, err := execute(ctx)
	finalState := unifiedresources.ActionStateCompleted
	finalMessage := "native resource action completed"
	executionResult := &unifiedresources.ExecutionResult{Success: true}

	if err != nil {
		finalState = unifiedresources.ActionStateFailed
		finalMessage = err.Error()
		executionResult.Success = false
		executionResult.ErrorMessage = err.Error()
	} else if result != nil {
		executionResult = result
		if !result.Success {
			finalState = unifiedresources.ActionStateFailed
			finalMessage = strings.TrimSpace(result.ErrorMessage)
			if finalMessage == "" {
				finalMessage = "native resource action failed"
			}
		}
	}

	record.State = finalState
	record.UpdatedAt = time.Now().UTC()
	record.Result = executionResult
	e.recordActionAudit(record)
	e.recordActionLifecycle(actionID, finalState, requestedBy, finalMessage)

	return executionResult, err
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

// RecordApprovalDecision updates the unified action audit for an approval that
// reached a terminal or pre-execution decision state.
func (e *PulseToolExecutor) RecordApprovalDecision(approvalID string, state unifiedresources.ActionState, actor, message string) {
	req := approvalRequestForID(approvalID)
	if e == nil || req == nil || req.Plan == nil {
		return
	}
	if strings.TrimSpace(actor) == "" {
		actor = approvalDecisionActor(req, approvalAuditActor)
	}
	if strings.TrimSpace(message) == "" {
		message = string(state)
	}
	record := actionAuditRecordFromApproval(req, state, actor)
	record.Approvals = approvalRecordsForID(req.ID)
	e.recordActionAudit(record)
	e.recordActionLifecycle(req.Plan.ActionID, state, actor, message)
}

func (e *PulseToolExecutor) recordPendingApprovalAction(req *approval.ApprovalRequest) {
	if e == nil || req == nil || req.Plan == nil {
		return
	}
	record := actionAuditRecordFromApproval(req, unifiedresources.ActionStatePending, approvalAuditActor)
	e.recordActionAudit(record)
	e.recordActionLifecycle(req.Plan.ActionID, unifiedresources.ActionStatePlanned, approvalAuditActor, strings.TrimSpace(req.Context))
	e.recordActionLifecycle(req.Plan.ActionID, unifiedresources.ActionStatePending, approvalAuditActor, "waiting for approval")
}

func mergeApprovedActionPlan(approved unifiedresources.ActionPlan, fallback unifiedresources.ActionPlan) unifiedresources.ActionPlan {
	if strings.TrimSpace(approved.ActionID) == "" {
		approved.ActionID = fallback.ActionID
	}
	if strings.TrimSpace(approved.RequestID) == "" {
		approved.RequestID = fallback.RequestID
	}
	if approved.PlannedAt.IsZero() {
		approved.PlannedAt = fallback.PlannedAt
	}
	if approved.ExpiresAt.IsZero() {
		approved.ExpiresAt = fallback.ExpiresAt
	}
	if strings.TrimSpace(approved.PlanHash) == "" {
		approved.PlanHash = fallback.PlanHash
	}
	if strings.TrimSpace(approved.Message) == "" {
		approved.Message = fallback.Message
	}
	approved.Allowed = true
	approved.RequiresApproval = approved.RequiresApproval || fallback.RequiresApproval
	if approved.ApprovalPolicy == "" {
		approved.ApprovalPolicy = fallback.ApprovalPolicy
	}
	return approved
}

func actionAuditRecordFromApproval(req *approval.ApprovalRequest, state unifiedresources.ActionState, actor string) unifiedresources.ActionAuditRecord {
	now := time.Now().UTC()
	plan := *req.Plan
	createdAt := plan.PlannedAt
	if createdAt.IsZero() {
		createdAt = now
	}
	requestID := strings.TrimSpace(plan.RequestID)
	if requestID == "" {
		requestID = strings.TrimSpace(req.ID)
	}
	params := map[string]any{
		"command":    req.Command,
		"targetType": req.TargetType,
		"targetId":   req.TargetID,
		"targetName": req.TargetName,
		"approvalId": req.ID,
	}
	if orgID := strings.TrimSpace(req.OrgID); orgID != "" {
		params["orgId"] = orgID
	}
	return unifiedresources.ActionAuditRecord{
		ID:        plan.ActionID,
		CreatedAt: createdAt,
		UpdatedAt: now,
		State:     state,
		Request: unifiedresources.ActionRequest{
			RequestID:      requestID,
			ResourceID:     approvalAuditResourceID(req.TargetType, req.TargetID, req.TargetName),
			CapabilityName: approvalCapabilityForTargetType(req.TargetType),
			Params:         params,
			Reason:         strings.TrimSpace(req.Context),
			RequestedBy:    actor,
		},
		Plan: plan,
	}
}

func approvalRequestForID(approvalID string) *approval.ApprovalRequest {
	approvalID = strings.TrimSpace(approvalID)
	if approvalID == "" {
		return nil
	}
	store := approval.GetStore()
	if store == nil {
		return nil
	}
	req, ok := store.GetApproval(approvalID)
	if !ok || req == nil {
		return nil
	}
	return req
}

func approvalDecisionActor(req *approval.ApprovalRequest, fallback string) string {
	if req != nil {
		if actor := strings.TrimSpace(req.DecidedBy); actor != "" {
			return actor
		}
	}
	if actor := strings.TrimSpace(fallback); actor != "" {
		return actor
	}
	return approvalAuditActor
}

func approvalCapabilityForTargetType(targetType string) string {
	switch strings.ToLower(strings.TrimSpace(targetType)) {
	case "docker":
		return "pulse_docker"
	case "file":
		return "pulse_file_edit"
	case "kubernetes":
		return "pulse_kubernetes"
	default:
		return "pulse_control"
	}
}

func approvalAuditResourceID(targetType, targetID, targetName string) string {
	targetType = strings.TrimSpace(targetType)
	targetID = strings.TrimSpace(targetID)
	if targetID == "" {
		targetID = strings.TrimSpace(targetName)
	}
	if targetID == "" {
		return strings.TrimSpace(targetType)
	}
	if targetType == "" || strings.Contains(targetID, ":") {
		return targetID
	}
	return targetType + ":" + targetID
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

func actionPlanHashForParams(actionID, requestID, capabilityName, resourceID string, params map[string]any, reason string) string {
	encoded, _ := json.Marshal(cloneActionParams(params))
	sum := sha256.Sum256([]byte(strings.Join([]string{
		actionID,
		requestID,
		capabilityName,
		strings.TrimSpace(resourceID),
		string(encoded),
		reason,
	}, "|")))
	return hex.EncodeToString(sum[:])
}

func approvalPlanHash(actionID, requestID, capabilityName, resourceID, command, targetType, targetID, context string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{
		actionID,
		requestID,
		capabilityName,
		strings.TrimSpace(resourceID),
		command,
		strings.TrimSpace(targetType),
		strings.TrimSpace(targetID),
		strings.TrimSpace(context),
	}, "|")))
	return hex.EncodeToString(sum[:])
}

func cloneActionParams(params map[string]any) map[string]any {
	if len(params) == 0 {
		return nil
	}
	cloned := make(map[string]any, len(params))
	for key, value := range params {
		cloned[key] = value
	}
	return cloned
}

func actionAuditPreflight(resourceID, reason string, generatedAt time.Time) *unifiedresources.ActionPreflight {
	return unifiedresources.NormalizeActionPreflight(&unifiedresources.ActionPreflight{
		Target:            strings.TrimSpace(resourceID),
		IntendedChange:    strings.TrimSpace(reason),
		DryRunAvailable:   false,
		DryRunSummary:     "No provider-supported dry run is available for this action.",
		SafetyChecks:      []string{"Approval and execution are scoped to the resolved resource."},
		VerificationSteps: []string{"Read back the target state after execution."},
		GeneratedAt:       generatedAt,
	}, unifiedresources.ActionRequest{ResourceID: resourceID, Reason: reason}, unifiedresources.ActionPlan{PlannedAt: generatedAt, Message: reason})
}

func approvalRecordsForID(approvalID string) []unifiedresources.ActionApprovalRecord {
	approvalID = strings.TrimSpace(approvalID)
	if approvalID == "" {
		return nil
	}
	store := approval.GetStore()
	if store == nil {
		return nil
	}
	req, ok := store.GetApproval(approvalID)
	if !ok || req == nil {
		return nil
	}

	record := unifiedresources.ActionApprovalRecord{
		Actor:     strings.TrimSpace(req.DecidedBy),
		Method:    unifiedresources.MethodAPI,
		Timestamp: approvalTimestamp(req),
		Outcome:   unifiedresources.OutcomeApproved,
		Reason:    strings.TrimSpace(req.Context),
	}
	if req.Status == approval.StatusDenied {
		record.Outcome = unifiedresources.OutcomeRejected
	}
	return []unifiedresources.ActionApprovalRecord{record}
}

func approvalTimestamp(req *approval.ApprovalRequest) time.Time {
	if req == nil || req.DecidedAt == nil {
		return time.Now().UTC()
	}
	return req.DecidedAt.UTC()
}
