package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/approval"
	unifiedresources "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rs/zerolog/log"
)

const approvalAuditActor = approval.RequesterPulseAssistant

// ErrRemediationLockStateUnknown is returned when the broker cannot
// determine whether the operator has set NeverAutoRemediate on the
// target resource — no audit store is wired, or the operator-state
// lookup failed. Autonomous (non-human-approved) dispatches must fail
// CLOSED on this error: a broker that cannot read the operator's
// "never touch this" flag must not assume it is unset. Human-approved
// dispatches may proceed, since the operator has explicitly signed
// off on the specific plan.
var ErrRemediationLockStateUnknown = errors.New("remediation lock state unknown; operator approval required")

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

	// Plan-drift check: the operator approved a specific command + target +
	// reason combination, and the broker must refuse to run a different one
	// even when the approval ID resolves. Compares the approved hash against
	// a freshly-recomputed approval-equivalent hash from the actual payload.
	// Persists a refused audit record so operators can see drift caught in
	// the audit history rather than only in WARN logs.
	if planFromApproval {
		if driftErr := validateApprovedCommandPlanHash(
			plan.PlanHash,
			plan.ActionID,
			plan.RequestID,
			capabilityName,
			resourceID,
			payload.Command,
			payload.TargetType,
			payload.TargetID,
			reason,
		); driftErr != nil {
			log.Warn().
				Str("action_id", plan.ActionID).
				Str("approval_id", approvalID).
				Str("capability", capabilityName).
				Err(driftErr).
				Msg("Refusing action execution: payload does not match approved plan hash")
			now := time.Now().UTC()
			record.State = unifiedresources.ActionStateFailed
			record.UpdatedAt = now
			record.Result = &unifiedresources.ExecutionResult{
				Success:      false,
				ErrorMessage: fmt.Sprintf("plan_drift: %s", driftErr.Error()),
			}
			if err := persistFailedActionAudit(e.actionAuditStore, record, requestedBy, "plan drift refused"); err != nil {
				log.Warn().Err(err).Str("action_id", record.ID).Msg("failed to persist plan-drift refusal")
			}
			e.publishActionCompleted(record)
			return nil, driftErr
		}
	}

	// Operator-set NeverAutoRemediate refusal: when the operator has
	// flagged the target resource as never-auto-remediate, the broker
	// must refuse the dispatch even with a valid approval and matching
	// plan hash. The operator's per-resource intent outranks the
	// per-action approval — this is the safety mechanism for "do not
	// touch this resource even if you think you should." When the lock
	// state cannot be determined (no store, or lookup error), only a
	// dispatch backed by an approved human decision may proceed;
	// autonomous dispatches fail closed. Persists a Failed audit record
	// with `resource_remediation_locked:` or
	// `remediation_lock_state_unknown:` prefix so the audit history
	// shows every refused dispatch.
	if refusal := e.checkRemediationLockForDispatch(plan.ActionID, resourceID, capabilityName, hasApprovedActionApproval(approvalRecords)); refusal != nil {
		e.refuseDispatchForRemediationLock(record, requestedBy, refusal)
		return nil, refusal
	}

	record, err := e.recordActionExecutionStart(record, approvalID, requestedBy, fmt.Sprintf("dispatching command to agent %s", agentID), planFromApproval)
	if err != nil {
		return nil, err
	}

	payload.ApprovalID = strings.TrimSpace(approvalID)
	if approvalReq != nil && approvalReq.Plan != nil {
		payload.BindCommandAuthorization(e.orgID, approvalReq.Plan.ActionID)
	}
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

	// Read-after-write verification: if execution succeeded AND the command
	// class has a derivable verification check, run it via the same agent
	// path and record the outcome on the audit record. Verification is
	// best-effort: if no check is derivable, or the check fails to run, we
	// record that fact rather than fabricating a verified=true.
	if executionResult.Success {
		if vCmd, ok := VerificationCommandForCommand(payload.TargetType, payload.Command); ok {
			vRanAt := time.Now().UTC()
			vResult, vErr := e.agentServer.ExecuteCommand(ctx, agentID, agentexec.ExecuteCommandPayload{
				Command:    vCmd,
				TargetType: payload.TargetType,
				TargetID:   payload.TargetID,
			})
			verification := &unifiedresources.ActionVerificationResult{
				Ran:     true,
				Command: vCmd,
				RanAt:   vRanAt,
			}
			if vErr != nil {
				verification.Success = false
				verification.Note = fmt.Sprintf("verification check failed to dispatch: %s", vErr.Error())
			} else {
				output := strings.TrimSpace(vResult.Stdout)
				if vResult.Stderr != "" {
					if output != "" {
						output += "\n"
					}
					output += strings.TrimSpace(vResult.Stderr)
				}
				verification.Output = output
				verification.Success = vResult.ExitCode == 0
				if !verification.Success {
					verification.Note = fmt.Sprintf("verification check exit code %d", vResult.ExitCode)
				}
			}
			executionResult.Verification = verification
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

	// Same operator-lock gate as the agent-command dispatch path: native
	// provider dispatches (e.g. TrueNAS app start/stop/restart) are write
	// actions on the resource, and the operator's NeverAutoRemediate flag
	// must gate them identically — including failing closed for
	// autonomous dispatches when the lock state cannot be determined.
	if refusal := e.checkRemediationLockForDispatch(plan.ActionID, resourceID, capabilityName, hasApprovedActionApproval(approvalRecords)); refusal != nil {
		return e.refuseDispatchForRemediationLock(record, requestedBy, refusal), refusal
	}

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
		approvalRecord := record
		record, err = e.ensureApprovalDecisionBeforeExecution(record, approvalID, actor, now)
		if err != nil {
			if unifiedresources.IsPermanentActionExecutionRefusal(err) {
				return e.recordActionExecutionRefusal(actionExecutionRefusalRecord(record, approvalRecord), err, actor, now)
			}
			return record, err
		}
	} else if e != nil && e.actionAuditStore != nil {
		plannedEvent := unifiedresources.ActionLifecycleEvent{
			ActionID: record.ID, Timestamp: now, State: unifiedresources.ActionStatePlanned,
			Actor: actor, Message: strings.TrimSpace(record.Request.Reason),
		}
		current, _, err := e.actionAuditStore.CreateActionAudit(record, []unifiedresources.ActionLifecycleEvent{plannedEvent})
		if err != nil {
			log.Warn().
				Err(err).
				Str("action_id", record.ID).
				Str("resource_id", record.Request.ResourceID).
				Msg("failed to persist planned action audit")
			return record, err
		}
		record = current
	}

	started, event, err := unifiedresources.BeginActionExecution(record, actor, now)
	if err != nil {
		log.Warn().
			Err(err).
			Str("action_id", record.ID).
			Str("state", string(record.State)).
			Msg("failed to normalize action execution start")
		if unifiedresources.IsPermanentActionExecutionRefusal(err) {
			return e.recordActionExecutionRefusal(record, err, actor, now)
		}
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

func (e *PulseToolExecutor) recordActionExecutionRefusal(record unifiedresources.ActionAuditRecord, reason error, actor string, now time.Time) (unifiedresources.ActionAuditRecord, error) {
	failed, event, err := unifiedresources.RefuseActionExecution(record, reason, actor, now)
	if err != nil {
		return record, err
	}
	if e == nil || e.actionAuditStore == nil {
		e.publishActionCompleted(failed)
		return failed, reason
	}
	_, found, queryErr := e.actionAuditStore.GetActionAudit(failed.ID)
	if queryErr != nil {
		return record, queryErr
	}
	var persistErr error
	if found {
		persistErr = e.actionAuditStore.RecordActionExecutionRefusal(failed, event)
	} else {
		_, _, persistErr = e.actionAuditStore.CreateActionAudit(failed, []unifiedresources.ActionLifecycleEvent{event})
	}
	if persistErr != nil {
		log.Warn().
			Err(persistErr).
			Str("action_id", failed.ID).
			Str("state", string(failed.State)).
			Msg("failed to persist action execution refusal")
		return record, persistErr
	}
	e.publishActionCompleted(failed)
	return failed, reason
}

func actionExecutionRefusalRecord(record, fallback unifiedresources.ActionAuditRecord) unifiedresources.ActionAuditRecord {
	if len(record.Approvals) == 0 && len(fallback.Approvals) > 0 {
		record.Approvals = append([]unifiedresources.ActionApprovalRecord(nil), fallback.Approvals...)
	}
	if record.Request.Params == nil && len(fallback.Request.Params) > 0 {
		record.Request.Params = cloneActionParams(fallback.Request.Params)
	}
	return record
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
		decisionEvent := unifiedresources.ActionLifecycleEvent{ActionID: record.ID, Timestamp: now, State: record.State, Actor: actor, Message: "Action approval was recorded before execution."}
		current, _, err := e.actionAuditStore.CreateActionAudit(record, []unifiedresources.ActionLifecycleEvent{decisionEvent})
		if err != nil {
			log.Warn().Err(err).Str("action_id", record.ID).Msg("failed to persist approved action audit before execution")
			return record, err
		}
		return current, nil
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
		log.Warn().Str("action_id", record.ID).Msg("refusing to persist a synthetic terminal action result after transition normalization failed")
		return record
	}
	if strings.TrimSpace(message) != "" {
		event.Message = strings.TrimSpace(message)
	}
	if e == nil || e.actionAuditStore == nil {
		e.publishActionCompleted(completed)
		return completed
	}
	if err := e.actionAuditStore.RecordActionExecutionResult(completed, event); err != nil {
		log.Warn().
			Err(err).
			Str("action_id", completed.ID).
			Str("state", string(completed.State)).
			Msg("failed to persist action execution result")
	}
	e.publishActionCompleted(completed)
	return completed
}

// publishActionCompleted dispatches the executor's post-completion
// callback, if installed. Fire-and-forget on its own goroutine: the
// callback runs after the audit record has already been persisted,
// so a panic or stall on the consumer side must not back up the
// dispatch hot path. Snapshots the record by value so the consumer
// reads a stable view independent of any subsequent record
// mutations on the originating goroutine.
func (e *PulseToolExecutor) publishActionCompleted(record unifiedresources.ActionAuditRecord) {
	if e == nil {
		return
	}
	cb := e.onActionCompleted
	if cb == nil {
		return
	}
	if record.State != unifiedresources.ActionStateCompleted && record.State != unifiedresources.ActionStateFailed {
		// Defensive — only Completed/Failed are terminal states the
		// agent SSE stream surfaces. Any other state would confuse
		// agents that branch on "this action is done."
		return
	}
	go cb(record)
}

// RecordApprovalDecision updates the unified action audit for an approval that
// reached a terminal or pre-execution decision state.
func (e *PulseToolExecutor) RecordApprovalDecision(approvalID string, state unifiedresources.ActionState, actor, message string) {
	var store unifiedresources.ResourceStore
	if e != nil {
		store = e.actionAuditStore
	}
	RecordApprovalDecision(store, approvalID, state, actor, message)
}

// RecordApprovalDecision updates the unified action audit for an approval that
// reached a terminal or pre-execution decision state.
func RecordApprovalDecision(store unifiedresources.ResourceStore, approvalID string, state unifiedresources.ActionState, actor, message string) {
	req := approvalRequestForID(approvalID)
	if req == nil || req.Plan == nil {
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
	if recordApprovalDecisionAtomically(store, req.ID, record, actor) {
		return
	}
	event := unifiedresources.ActionLifecycleEvent{ActionID: req.Plan.ActionID, Timestamp: time.Now().UTC(), State: state, Actor: actor, Message: message}
	if _, _, err := store.CreateActionAudit(record, []unifiedresources.ActionLifecycleEvent{event}); err != nil {
		log.Warn().Err(err).Str("action_id", record.ID).Msg("failed to create action audit for approval decision")
	}
}

func (e *PulseToolExecutor) recordApprovalDecisionAtomically(approvalID string, record unifiedresources.ActionAuditRecord, actor string) bool {
	if e == nil {
		return false
	}
	return recordApprovalDecisionAtomically(e.actionAuditStore, approvalID, record, actor)
}

func recordApprovalDecisionAtomically(store unifiedresources.ResourceStore, approvalID string, record unifiedresources.ActionAuditRecord, actor string) bool {
	if store == nil {
		return false
	}
	if record.State != unifiedresources.ActionStateApproved && record.State != unifiedresources.ActionStateRejected {
		return false
	}

	current, ok, err := store.GetActionAudit(record.ID)
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
	if err := store.RecordActionDecision(updated, event); err != nil {
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
	actor := approval.RequesterForRequest(req)
	record := actionAuditRecordFromApproval(req, unifiedresources.ActionStatePending, actor)
	events := []unifiedresources.ActionLifecycleEvent{
		{ActionID: req.Plan.ActionID, State: unifiedresources.ActionStatePlanned, Timestamp: record.CreatedAt, Actor: actor, Message: strings.TrimSpace(req.Context)},
		{ActionID: req.Plan.ActionID, State: unifiedresources.ActionStatePending, Timestamp: record.CreatedAt, Actor: actor, Message: "waiting for approval"},
	}
	if _, _, err := store.CreateActionAudit(record, events); err != nil {
		log.Warn().Err(err).Str("action_id", record.ID).Msg("failed to persist pending approval action")
	}
}

func recordApprovalLifecycle(store unifiedresources.ResourceStore, actionID string, state unifiedresources.ActionState, actor, message string) {
	event := unifiedresources.ActionLifecycleEvent{
		ActionID:  strings.TrimSpace(actionID),
		State:     state,
		Timestamp: time.Now().UTC(),
		Actor:     strings.TrimSpace(actor),
		Message:   strings.TrimSpace(message),
	}
	if event.Actor == "" {
		event.Actor = approvalAuditActor
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
		return agentcapabilities.PulseDockerToolName
	case "file":
		return agentcapabilities.PulseFileEditToolName
	case "kubernetes":
		return agentcapabilities.PulseKubernetesToolName
	default:
		return agentcapabilities.PulseControlToolName
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

// validateApprovedCommandPlanHash recomputes the approval-equivalent plan
// hash from the actual payload presented at execute time and compares it to
// the hash recorded at approval time. If approvedHash is empty (older
// approval records, or contract paths that did not author one), validation
// is skipped and existing behavior is preserved. If the hashes differ, the
// broker must refuse execution: the operator approved a specific command +
// target + reason combination, and a different one is now being requested.
//
// The hash function used here matches `approvalPlanHash` (the function
// invoked at approval-creation time) so direct comparison is meaningful.
// `actionPlanHash` shape differs slightly (no field-level whitespace
// trimming) and would produce false-positive drift on legitimate payloads.
func validateApprovedCommandPlanHash(
	approvedHash string,
	actionID, requestID, capabilityName, resourceID string,
	command, targetType, targetID, reason string,
) error {
	approvedHash = strings.TrimSpace(approvedHash)
	if approvedHash == "" {
		return nil
	}
	fresh := approvalPlanHash(actionID, requestID, capabilityName, resourceID, command, targetType, targetID, reason)
	if fresh == approvedHash {
		return nil
	}
	return fmt.Errorf("%w (approved %s, payload %s)", unifiedresources.ErrActionPlanDrift, approvedHash, fresh)
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

// isResourceRemediationLocked returns whether the operator has set
// NeverAutoRemediate=true on the resource the dispatch targets.
// Returns (false, nil) when the resource has no operator state, or
// when only IntentionallyOffline (without NeverAutoRemediate) is set.
// When the lock state cannot be determined — no audit store is wired,
// or the store lookup fails — it returns an error wrapping
// ErrRemediationLockStateUnknown so the caller can
// distinguish "not locked" from "unknown". Caller policy lives in
// checkRemediationLockForDispatch: autonomous (non-human-approved)
// dispatches fail closed on unknown state; human-approved dispatches
// keep the historical fail-open posture.
func (e *PulseToolExecutor) isResourceRemediationLocked(resourceID string) (bool, error) {
	canonical := strings.TrimSpace(resourceID)
	if canonical == "" {
		return false, nil
	}
	if e == nil || e.actionAuditStore == nil {
		return false, fmt.Errorf("%w: no action audit store is wired", ErrRemediationLockStateUnknown)
	}
	state, found, err := e.actionAuditStore.GetResourceOperatorState(canonical)
	if err != nil {
		return false, fmt.Errorf("%w: operator-state lookup failed: %s", ErrRemediationLockStateUnknown, err.Error())
	}
	if !found {
		return false, nil
	}
	return state.NeverAutoRemediate, nil
}

// checkRemediationLockForDispatch applies the operator NeverAutoRemediate
// policy at the dispatch decision point. Returns nil when the dispatch may
// proceed, ErrResourceRemediationLocked when the operator has locked the
// resource, or an ErrRemediationLockStateUnknown-wrapped error when the
// lock state cannot be determined and the dispatch carries no approved
// human decision. Autonomous dispatches fail CLOSED on unknown lock state:
// a broker that cannot read the operator's "never touch this" flag must
// not assume it is unset. Human-approved dispatches keep the historical
// fail-open posture — the operator explicitly signed off on this exact
// plan, so a degraded operator-state lookup does not override their
// decision — but the degraded lookup is still logged.
func (e *PulseToolExecutor) checkRemediationLockForDispatch(actionID, resourceID, capabilityName string, humanApproved bool) error {
	locked, lockErr := e.isResourceRemediationLocked(resourceID)
	if lockErr != nil {
		if humanApproved {
			log.Warn().
				Str("action_id", actionID).
				Str("resource_id", resourceID).
				Str("capability", capabilityName).
				Err(lockErr).
				Msg("Remediation lock state unknown; allowing human-approved dispatch (fail-open) but logging")
			return nil
		}
		log.Warn().
			Str("action_id", actionID).
			Str("resource_id", resourceID).
			Str("capability", capabilityName).
			Err(lockErr).
			Msg("Refusing autonomous action execution: remediation lock state unknown (fail-closed); operator approval required")
		return lockErr
	}
	if locked {
		log.Warn().
			Str("action_id", actionID).
			Str("resource_id", resourceID).
			Str("capability", capabilityName).
			Msg("Refusing action execution: resource is operator-locked against automated remediation")
		return unifiedresources.ErrResourceRemediationLocked
	}
	return nil
}

// refuseDispatchForRemediationLock persists a Failed audit record for a
// dispatch refused by the remediation-lock gate and returns the
// ExecutionResult describing the refusal. The ErrorMessage carries a
// stable prefix (`resource_remediation_locked:` or
// `remediation_lock_state_unknown:`) so audit-UI filters and alert rules
// can branch on the token.
func (e *PulseToolExecutor) refuseDispatchForRemediationLock(record unifiedresources.ActionAuditRecord, requestedBy string, refusal error) *unifiedresources.ExecutionResult {
	now := time.Now().UTC()
	record.State = unifiedresources.ActionStateFailed
	record.UpdatedAt = now
	result := &unifiedresources.ExecutionResult{
		Success:      false,
		ErrorMessage: remediationLockRefusalMessage(refusal),
	}
	record.Result = result
	if err := persistFailedActionAudit(e.actionAuditStore, record, requestedBy, remediationLockLifecycleMessage(refusal)); err != nil {
		log.Warn().Err(err).Str("action_id", record.ID).Msg("failed to persist remediation-lock refusal")
	}
	e.publishActionCompleted(record)
	return result
}

func persistFailedActionAudit(store unifiedresources.ResourceStore, record unifiedresources.ActionAuditRecord, actor, message string) error {
	if store == nil {
		return nil
	}
	event := unifiedresources.ActionLifecycleEvent{ActionID: record.ID, Timestamp: record.UpdatedAt, State: unifiedresources.ActionStateFailed, Actor: actor, Message: message}
	current, found, err := store.GetActionAudit(record.ID)
	if err != nil {
		return err
	}
	if found {
		current.State = unifiedresources.ActionStateFailed
		current.UpdatedAt = record.UpdatedAt
		current.Result = record.Result
		current.Verification = record.Verification
		current.VerificationOutcome = record.VerificationOutcome
		return store.RecordActionExecutionRefusal(current, event)
	}
	_, _, err = store.CreateActionAudit(record, []unifiedresources.ActionLifecycleEvent{event})
	return err
}

func remediationLockRefusalMessage(refusal error) string {
	if errors.Is(refusal, ErrRemediationLockStateUnknown) {
		return fmt.Sprintf("remediation_lock_state_unknown: %s", refusal.Error())
	}
	return fmt.Sprintf("resource_remediation_locked: %s", refusal.Error())
}

func remediationLockLifecycleMessage(refusal error) string {
	if errors.Is(refusal, ErrRemediationLockStateUnknown) {
		return "remediation lock state unknown refused"
	}
	return "resource remediation lock refused"
}
