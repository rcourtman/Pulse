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
	approvalRecords := approvalRecordsForID(approvalID)

	record := unifiedresources.ActionAuditRecord{
		ID:        actionID,
		CreatedAt: now,
		UpdatedAt: now,
		State:     actionPreExecutionState(planFromApproval, approvalRecords),
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
	record.Approvals = approvalRecords

	record, err := e.recordActionExecutionStart(record, approvalID, requestedBy, fmt.Sprintf("dispatching command to agent %s", agentID), planFromApproval)
	if err != nil {
		return nil, err
	}

	payload.ApprovalID = strings.TrimSpace(approvalID)
	result, err := e.agentServer.ExecuteCommand(ctx, agentID, payload)
	finalMessage := "command completed"
	executionResult := &unifiedresources.ExecutionResult{Success: true}

	if err != nil {
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
			finalMessage = fmt.Sprintf("exit code %d", result.ExitCode)
			executionResult.Success = false
			executionResult.ErrorMessage = finalMessage
		}
	}

	e.recordActionExecutionResult(record, executionResult, requestedBy, finalMessage)

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
	approvalRecords := approvalRecordsForID(approvalID)

	record := unifiedresources.ActionAuditRecord{
		ID:        actionID,
		CreatedAt: now,
		UpdatedAt: now,
		State:     actionPreExecutionState(planFromApproval, approvalRecords),
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
	record.Approvals = approvalRecords

	record, err := e.recordActionExecutionStart(record, approvalID, requestedBy, "dispatching native resource action", planFromApproval)
	if err != nil {
		return &unifiedresources.ExecutionResult{Success: false, ErrorMessage: err.Error()}, err
	}

	result, err := execute(ctx)
	finalMessage := "native resource action completed"
	executionResult := &unifiedresources.ExecutionResult{Success: true}

	if err != nil {
		finalMessage = err.Error()
		executionResult.Success = false
		executionResult.ErrorMessage = err.Error()
	} else if result != nil {
		executionResult = result
		if !result.Success {
			finalMessage = strings.TrimSpace(result.ErrorMessage)
			if finalMessage == "" {
				finalMessage = "native resource action failed"
			}
		}
	}

	e.recordActionExecutionResult(record, executionResult, requestedBy, finalMessage)

	return executionResult, err
}

func actionPreExecutionState(planFromApproval bool, approvals []unifiedresources.ActionApprovalRecord) unifiedresources.ActionState {
	if planFromApproval || hasApprovedActionApproval(approvals) {
		return unifiedresources.ActionStateApproved
	}
	return unifiedresources.ActionStatePlanned
}

func hasApprovedActionApproval(approvals []unifiedresources.ActionApprovalRecord) bool {
	for _, approval := range approvals {
		if approval.Outcome == unifiedresources.OutcomeApproved {
			return true
		}
	}
	return false
}

func (e *PulseToolExecutor) recordActionExecutionStart(record unifiedresources.ActionAuditRecord, approvalID, actor, message string, planFromApproval bool) (unifiedresources.ActionAuditRecord, error) {
	now := time.Now().UTC()
	if strings.TrimSpace(actor) == "" {
		actor = approvalAuditActor
	}
	if planFromApproval {
		var err error
		record, err = e.ensureApprovalDecisionBeforeExecution(record, approvalID, actor, now)
		if err != nil {
			return record, err
		}
	} else if e != nil && e.actionAuditStore != nil {
		if err := e.actionAuditStore.RecordActionAudit(record); err != nil {
			log.Warn().
				Err(err).
				Str("action_id", record.ID).
				Str("resource_id", record.Request.ResourceID).
				Msg("failed to persist planned action audit")
			return record, err
		}
		event := unifiedresources.ActionLifecycleEvent{
			ActionID:  record.ID,
			Timestamp: now,
			State:     unifiedresources.ActionStatePlanned,
			Actor:     actor,
			Message:   strings.TrimSpace(record.Request.Reason),
		}
		if err := e.actionAuditStore.RecordActionLifecycleEvent(event); err != nil {
			log.Warn().
				Err(err).
				Str("action_id", record.ID).
				Str("state", string(unifiedresources.ActionStatePlanned)).
				Msg("failed to persist planned action lifecycle event")
			return record, err
		}
	}

	started, event, err := unifiedresources.BeginActionExecution(record, actor, now)
	if err != nil {
		log.Warn().
			Err(err).
			Str("action_id", record.ID).
			Str("state", string(record.State)).
			Msg("failed to normalize action execution start")
		return record, err
	}
	if strings.TrimSpace(message) != "" {
		event.Message = strings.TrimSpace(message)
	}
	if e == nil || e.actionAuditStore == nil {
		return started, nil
	}
	if err := e.actionAuditStore.RecordActionExecutionStart(started, event); err != nil {
		log.Warn().
			Err(err).
			Str("action_id", started.ID).
			Str("state", string(started.State)).
			Msg("failed to persist action execution start")
		return record, err
	}
	return started, nil
}

func (e *PulseToolExecutor) ensureApprovalDecisionBeforeExecution(record unifiedresources.ActionAuditRecord, approvalID, actor string, now time.Time) (unifiedresources.ActionAuditRecord, error) {
	if e == nil || e.actionAuditStore == nil {
		return record, nil
	}

	current, ok, err := e.actionAuditStore.GetActionAudit(record.ID)
	if err != nil {
		log.Warn().Err(err).Str("action_id", record.ID).Msg("failed to query action audit before execution")
		return record, err
	}
	if !ok {
		if err := e.actionAuditStore.RecordActionAudit(record); err != nil {
			log.Warn().Err(err).Str("action_id", record.ID).Msg("failed to persist approved action audit before execution")
			return record, err
		}
		return record, nil
	}
	if current.State != unifiedresources.ActionStatePending {
		return current, nil
	}

	approvalRecord := actionApprovalRecordForExecution(approvalID, actor, now, unifiedresources.OutcomeApproved)
	updated, event, err := unifiedresources.ApplyActionDecision(current, approvalRecord, approvalRecord.Timestamp)
	if err != nil {
		log.Warn().Err(err).Str("action_id", record.ID).Msg("failed to normalize approval decision before execution")
		return current, err
	}
	if err := e.actionAuditStore.RecordActionDecision(updated, event); err != nil {
		log.Warn().Err(err).Str("action_id", record.ID).Msg("failed to persist approval decision before execution")
		return current, err
	}
	return updated, nil
}

func actionApprovalRecordForExecution(approvalID, actor string, now time.Time, fallbackOutcome unifiedresources.ApprovalOutcome) unifiedresources.ActionApprovalRecord {
	if fallbackOutcome == "" {
		fallbackOutcome = unifiedresources.OutcomeApproved
	}
	records := approvalRecordsForID(approvalID)
	if len(records) > 0 {
		record := records[len(records)-1]
		if record.Outcome == "" {
			record.Outcome = fallbackOutcome
		}
		if record.Timestamp.IsZero() {
			record.Timestamp = now
		}
		if strings.TrimSpace(record.Actor) == "" {
			record.Actor = actor
		}
		return record
	}
	return unifiedresources.ActionApprovalRecord{
		Actor:     actor,
		Method:    unifiedresources.MethodAPI,
		Timestamp: now,
		Outcome:   fallbackOutcome,
	}
}

func (e *PulseToolExecutor) recordActionExecutionResult(record unifiedresources.ActionAuditRecord, result *unifiedresources.ExecutionResult, actor, message string) unifiedresources.ActionAuditRecord {
	now := time.Now().UTC()
	if strings.TrimSpace(actor) == "" {
		actor = approvalAuditActor
	}
	completed, event, err := unifiedresources.CompleteActionExecution(record, result, actor, now)
	if err != nil {
		log.Warn().
			Err(err).
			Str("action_id", record.ID).
			Str("state", string(record.State)).
			Msg("failed to normalize action execution result")
		record.UpdatedAt = now
		record.Result = result
		if result != nil && result.Success {
			record.State = unifiedresources.ActionStateCompleted
		} else {
			record.State = unifiedresources.ActionStateFailed
		}
		e.recordActionAudit(record)
		e.recordActionLifecycle(record.ID, record.State, actor, message)
		return record
	}
	if strings.TrimSpace(message) != "" {
		event.Message = strings.TrimSpace(message)
	}
	if e == nil || e.actionAuditStore == nil {
		return completed
	}
	if err := e.actionAuditStore.RecordActionExecutionResult(completed, event); err != nil {
		log.Warn().
			Err(err).
			Str("action_id", completed.ID).
			Str("state", string(completed.State)).
			Msg("failed to persist action execution result")
	}
	return completed
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
	if e.recordApprovalDecisionAtomically(req.ID, record, actor) {
		return
	}
	e.recordActionAudit(record)
	e.recordActionLifecycle(req.Plan.ActionID, state, actor, message)
}

func (e *PulseToolExecutor) recordApprovalDecisionAtomically(approvalID string, record unifiedresources.ActionAuditRecord, actor string) bool {
	if e == nil || e.actionAuditStore == nil {
		return false
	}
	if record.State != unifiedresources.ActionStateApproved && record.State != unifiedresources.ActionStateRejected {
		return false
	}

	current, ok, err := e.actionAuditStore.GetActionAudit(record.ID)
	if err != nil {
		log.Warn().Err(err).Str("action_id", record.ID).Msg("failed to query action audit before approval decision")
		return false
	}
	if !ok {
		return false
	}
	if current.State != unifiedresources.ActionStatePending {
		if current.State != record.State {
			log.Warn().
				Str("action_id", record.ID).
				Str("current_state", string(current.State)).
				Str("decision_state", string(record.State)).
				Msg("ignoring approval decision for action audit that has already moved past pending")
		}
		return true
	}

	outcome := unifiedresources.OutcomeApproved
	if record.State == unifiedresources.ActionStateRejected {
		outcome = unifiedresources.OutcomeRejected
	}
	approvalRecord := actionApprovalRecordForExecution(approvalID, actor, time.Now().UTC(), outcome)
	approvalRecord.Outcome = outcome
	updated, event, err := unifiedresources.ApplyActionDecision(current, approvalRecord, approvalRecord.Timestamp)
	if err != nil {
		log.Warn().Err(err).Str("action_id", record.ID).Msg("failed to normalize approval decision")
		return false
	}
	if err := e.actionAuditStore.RecordActionDecision(updated, event); err != nil {
		log.Warn().Err(err).Str("action_id", record.ID).Msg("failed to persist approval decision")
		return false
	}
	return true
}

func (e *PulseToolExecutor) recordPendingApprovalAction(req *approval.ApprovalRequest) {
	if e == nil {
		return
	}
	RecordPendingApprovalAction(e.actionAuditStore, req)
}

// AttachApprovalActionPlan normalizes an approval request onto the shared
// action-governance model before it is persisted by the approval store.
func AttachApprovalActionPlan(req *approval.ApprovalRequest, now time.Time) {
	if req == nil {
		return
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	req.ID = strings.TrimSpace(req.ID)
	if req.ID == "" {
		req.ID = uuid.NewString()
	}
	capabilityName := approvalCapabilityForTargetType(req.TargetType)
	resourceID := approvalAuditResourceID(req.TargetType, req.TargetID, req.TargetName)
	if req.Plan == nil {
		actionID := uuid.NewString()
		req.Plan = &unifiedresources.ActionPlan{
			ActionID:             actionID,
			RequestID:            req.ID,
			Allowed:              true,
			RequiresApproval:     true,
			ApprovalPolicy:       unifiedresources.ApprovalAdmin,
			PredictedBlastRadius: approvalBlastRadius(req.TargetType, req.Command),
			RollbackAvailable:    approvalRollbackAvailable(req.TargetType, req.Command),
			Message:              strings.TrimSpace(req.Context),
			PlannedAt:            now,
			PlanHash:             approvalPlanHash(actionID, req.ID, capabilityName, resourceID, req.Command, req.TargetType, req.TargetID, req.Context),
		}
	}
	if strings.TrimSpace(req.Plan.ActionID) == "" {
		req.Plan.ActionID = uuid.NewString()
	}
	if strings.TrimSpace(req.Plan.RequestID) == "" {
		req.Plan.RequestID = req.ID
	}
	if req.Plan.PlannedAt.IsZero() {
		req.Plan.PlannedAt = now
	}
	if strings.TrimSpace(req.Plan.Message) == "" {
		req.Plan.Message = strings.TrimSpace(req.Context)
	}
	req.Plan.Allowed = true
	req.Plan.RequiresApproval = true
	if req.Plan.ApprovalPolicy == "" {
		req.Plan.ApprovalPolicy = unifiedresources.ApprovalAdmin
	}
	if strings.TrimSpace(req.Plan.PlanHash) == "" {
		req.Plan.PlanHash = approvalPlanHash(req.Plan.ActionID, req.Plan.RequestID, capabilityName, resourceID, req.Command, req.TargetType, req.TargetID, req.Context)
	}
	req.ContextConfidence = approvalContextConfidence(req)
	req.Preflight = approvalPreflight(req)
	req.Plan.Preflight = req.Preflight
}

// RecordPendingApprovalAction persists the planned and pending audit state for
// a newly-created governed approval request.
func RecordPendingApprovalAction(store unifiedresources.ResourceStore, req *approval.ApprovalRequest) {
	if store == nil || req == nil || req.Plan == nil {
		return
	}
	record := actionAuditRecordFromApproval(req, unifiedresources.ActionStatePending, approvalAuditActor)
	if err := store.RecordActionAudit(record); err != nil {
		log.Warn().Err(err).Str("action_id", record.ID).Msg("failed to persist pending approval action")
		return
	}
	recordApprovalLifecycle(store, req.Plan.ActionID, unifiedresources.ActionStatePlanned, strings.TrimSpace(req.Context))
	recordApprovalLifecycle(store, req.Plan.ActionID, unifiedresources.ActionStatePending, "waiting for approval")
}

func recordApprovalLifecycle(store unifiedresources.ResourceStore, actionID string, state unifiedresources.ActionState, message string) {
	event := unifiedresources.ActionLifecycleEvent{
		ActionID:  strings.TrimSpace(actionID),
		State:     state,
		Timestamp: time.Now().UTC(),
		Actor:     approvalAuditActor,
		Message:   strings.TrimSpace(message),
	}
	if err := store.RecordActionLifecycleEvent(event); err != nil {
		log.Warn().Err(err).Str("action_id", actionID).Str("state", string(state)).Msg("failed to persist pending approval lifecycle event")
	}
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
