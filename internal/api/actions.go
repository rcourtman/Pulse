package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/actionplanner"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

const maxActionPlanRequestBytes = 1 << 20
const maxActionDecisionRequestBytes = 64 << 10
const maxActionExecutionRequestBytes = 64 << 10

// ActionExecutor runs a previously planned and approved action through the
// API-owned execution contract.
type ActionExecutor interface {
	ExecuteAction(ctx context.Context, record unified.ActionAuditRecord) (*unified.ExecutionResult, error)
}

type actionDecisionRequest struct {
	Outcome unified.ApprovalOutcome `json:"outcome"`
	Reason  string                  `json:"reason,omitempty"`
}

type actionDecisionResponse struct {
	ActionID string                       `json:"actionId"`
	State    unified.ActionState          `json:"state"`
	Approval unified.ActionApprovalRecord `json:"approval"`
	Audit    unified.ActionAuditRecord    `json:"audit"`
}

type actionExecutionRequest struct {
	Reason string `json:"reason,omitempty"`
}

type actionExecutionResponse struct {
	ActionID string                    `json:"actionId"`
	State    unified.ActionState       `json:"state"`
	Result   *unified.ExecutionResult  `json:"result,omitempty"`
	Audit    unified.ActionAuditRecord `json:"audit"`
}

func (h *ResourceHandlers) HandlePlanAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req unified.ActionRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxActionPlanRequestBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeJSONErrorWithDetails(w, http.StatusBadRequest, "invalid_action_request", "Invalid action planning request", map[string]string{
			"body": "request body must be a valid ActionRequest JSON object",
		})
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeJSONErrorWithDetails(w, http.StatusBadRequest, "invalid_action_request", "Invalid action planning request", map[string]string{
			"body": "request body must contain one JSON object",
		})
		return
	}

	req.ResourceID = unified.CanonicalResourceID(req.ResourceID)
	if req.ResourceID == "" {
		writeJSONErrorWithDetails(w, http.StatusBadRequest, "invalid_action_request", "Invalid action planning request", map[string]string{
			"resourceId": "resource id is required",
		})
		return
	}

	orgID := GetOrgID(r.Context())
	registry, err := h.buildRegistry(orgID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "resource_registry_unavailable", sanitizeErrorForClient(err, "Resource registry unavailable"))
		return
	}

	resource, ok := registry.Get(req.ResourceID)
	if !ok || resource == nil {
		writeJSONErrorWithDetails(w, http.StatusNotFound, "resource_not_found", "Resource not found", map[string]string{
			"resourceId": req.ResourceID,
		})
		return
	}

	plan, err := (actionplanner.Planner{}).Plan(req, *resource)
	if err != nil {
		if validationErr, ok := actionplanner.AsValidationError(err); ok {
			details := map[string]string{}
			if validationErr.Field != "" {
				details[validationErr.Field] = validationErr.Message
			}
			writeJSONErrorWithDetails(w, http.StatusBadRequest, "invalid_action_request", "Invalid action planning request", details)
			return
		}
		if errors.Is(err, actionplanner.ErrCapabilityNotFound) {
			writeJSONErrorWithDetails(w, http.StatusNotFound, "capability_not_found", "Capability not found on resource", map[string]string{
				"resourceId":     req.ResourceID,
				"capabilityName": req.CapabilityName,
			})
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "action_plan_failed", sanitizeErrorForClient(err, "Action planning failed"))
		return
	}

	req = normalizeActionRequestForAudit(req)
	store, err := h.getStore(orgID)
	if err != nil {
		writeJSONError(w, http.StatusServiceUnavailable, "action_audit_unavailable", "Action audit history is not available")
		return
	}
	if err := persistActionPlanAudit(store, req, plan); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "action_audit_persist_failed", sanitizeErrorForClient(err, "Failed to persist action audit"))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(plan); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "action_plan_encode_failed", "Failed to encode action plan")
	}
}

func normalizeActionRequestForAudit(req unified.ActionRequest) unified.ActionRequest {
	req.RequestID = strings.TrimSpace(req.RequestID)
	req.ResourceID = unified.CanonicalResourceID(req.ResourceID)
	req.CapabilityName = strings.TrimSpace(req.CapabilityName)
	req.Reason = strings.TrimSpace(req.Reason)
	req.RequestedBy = strings.TrimSpace(req.RequestedBy)
	if req.Params == nil {
		req.Params = map[string]any{}
	}
	return req
}

func persistActionPlanAudit(store unified.ResourceStore, req unified.ActionRequest, plan unified.ActionPlan) error {
	state := plannedActionState(plan)
	record := unified.ActionAuditRecord{
		ID:        plan.ActionID,
		CreatedAt: plan.PlannedAt,
		UpdatedAt: plan.PlannedAt,
		State:     state,
		Request:   req,
		Plan:      plan,
	}
	if err := store.RecordActionAudit(record); err != nil {
		return err
	}

	existingEvents, err := store.GetActionLifecycleEvents(plan.ActionID, time.Time{}, 100)
	if err != nil {
		return err
	}
	seenStates := map[unified.ActionState]bool{}
	for _, event := range existingEvents {
		seenStates[event.State] = true
	}

	if !seenStates[unified.ActionStatePlanned] {
		if err := store.RecordActionLifecycleEvent(unified.ActionLifecycleEvent{
			ActionID:  plan.ActionID,
			Timestamp: plan.PlannedAt,
			State:     unified.ActionStatePlanned,
			Actor:     req.RequestedBy,
			Message:   "Action plan created.",
		}); err != nil {
			return err
		}
	}
	if state != unified.ActionStatePlanned && !seenStates[state] {
		if err := store.RecordActionLifecycleEvent(unified.ActionLifecycleEvent{
			ActionID:  plan.ActionID,
			Timestamp: plan.PlannedAt,
			State:     state,
			Actor:     req.RequestedBy,
			Message:   "Action is waiting for approval before execution.",
		}); err != nil {
			return err
		}
	}
	return nil
}

func plannedActionState(plan unified.ActionPlan) unified.ActionState {
	if plan.RequiresApproval {
		return unified.ActionStatePending
	}
	return unified.ActionStatePlanned
}

func (h *ResourceHandlers) HandleDecideAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	actionID := strings.TrimSpace(r.PathValue("id"))
	if actionID == "" {
		writeJSONError(w, http.StatusBadRequest, "missing_id", "Missing action ID")
		return
	}
	if !validAuditEventID.MatchString(actionID) || len(actionID) > 128 {
		writeJSONError(w, http.StatusBadRequest, "invalid_id", "Invalid action ID format")
		return
	}

	var decision actionDecisionRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxActionDecisionRequestBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decision); err != nil {
		writeJSONErrorWithDetails(w, http.StatusBadRequest, "invalid_action_decision", "Invalid action decision request", map[string]string{
			"body": "request body must be a valid action decision JSON object",
		})
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeJSONErrorWithDetails(w, http.StatusBadRequest, "invalid_action_decision", "Invalid action decision request", map[string]string{
			"body": "request body must contain one JSON object",
		})
		return
	}
	decision.Outcome = unified.ApprovalOutcome(strings.TrimSpace(string(decision.Outcome)))
	decision.Reason = strings.TrimSpace(decision.Reason)
	if decision.Outcome != unified.OutcomeApproved && decision.Outcome != unified.OutcomeRejected {
		writeJSONErrorWithDetails(w, http.StatusBadRequest, "invalid_action_decision", "Invalid action decision request", map[string]string{
			"outcome": "outcome must be approved or rejected",
		})
		return
	}

	orgID := GetOrgID(r.Context())
	store, err := h.getStore(orgID)
	if err != nil {
		writeJSONError(w, http.StatusServiceUnavailable, "action_audit_unavailable", "Action audit history is not available")
		return
	}
	record, ok, err := store.GetActionAudit(actionID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "action_audit_query_failed", "Failed to query action audit")
		return
	}
	if !ok {
		writeJSONErrorWithDetails(w, http.StatusNotFound, "action_not_found", "Action not found", map[string]string{
			"actionId": actionID,
		})
		return
	}

	actor := actionDecisionActor(h, r)
	now := time.Now().UTC()
	approval := unified.ActionApprovalRecord{
		Actor:     actor,
		Method:    unified.MethodAPI,
		Timestamp: now,
		Outcome:   decision.Outcome,
		Reason:    decision.Reason,
	}
	updated, event, err := unified.ApplyActionDecision(record, approval, now)
	if err != nil {
		writeActionDecisionApplyError(w, err)
		return
	}
	if err := store.RecordActionDecision(updated, event); err != nil {
		if errors.Is(err, unified.ErrActionNotPending) {
			writeActionDecisionApplyError(w, err)
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "action_decision_persist_failed", sanitizeErrorForClient(err, "Failed to persist action decision"))
		return
	}

	responseApproval := approval
	if len(updated.Approvals) > 0 {
		responseApproval = updated.Approvals[len(updated.Approvals)-1]
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(actionDecisionResponse{
		ActionID: updated.ID,
		State:    updated.State,
		Approval: responseApproval,
		Audit:    updated,
	}); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "action_decision_encode_failed", "Failed to encode action decision")
	}
}

func (h *ResourceHandlers) HandleExecuteAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	actionID := strings.TrimSpace(r.PathValue("id"))
	if actionID == "" {
		writeJSONError(w, http.StatusBadRequest, "missing_id", "Missing action ID")
		return
	}
	if !validAuditEventID.MatchString(actionID) || len(actionID) > 128 {
		writeJSONError(w, http.StatusBadRequest, "invalid_id", "Invalid action ID format")
		return
	}

	var execution actionExecutionRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxActionExecutionRequestBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&execution); err != nil {
		if !errors.Is(err, io.EOF) {
			writeJSONErrorWithDetails(w, http.StatusBadRequest, "invalid_action_execution", "Invalid action execution request", map[string]string{
				"body": "request body must be a valid action execution JSON object",
			})
			return
		}
	} else if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeJSONErrorWithDetails(w, http.StatusBadRequest, "invalid_action_execution", "Invalid action execution request", map[string]string{
			"body": "request body must contain one JSON object",
		})
		return
	}
	execution.Reason = strings.TrimSpace(execution.Reason)

	orgID := GetOrgID(r.Context())
	store, err := h.getStore(orgID)
	if err != nil {
		writeJSONError(w, http.StatusServiceUnavailable, "action_audit_unavailable", "Action audit history is not available")
		return
	}
	record, ok, err := store.GetActionAudit(actionID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "action_audit_query_failed", "Failed to query action audit")
		return
	}
	if !ok {
		writeJSONErrorWithDetails(w, http.StatusNotFound, "action_not_found", "Action not found", map[string]string{
			"actionId": actionID,
		})
		return
	}

	actor := actionDecisionActor(h, r)
	started, startEvent, err := unified.BeginActionExecution(record, actor, time.Now().UTC())
	if err != nil {
		writeActionExecutionApplyError(w, err)
		return
	}
	if execution.Reason != "" {
		startEvent.Message = "Action execution started: " + execution.Reason
	}
	if h.actionExecutor == nil {
		writeJSONError(w, http.StatusNotImplemented, "action_executor_unavailable", "No action executor is configured for this API instance")
		return
	}
	if err := store.RecordActionExecutionStart(started, startEvent); err != nil {
		writeActionExecutionPersistError(w, err)
		return
	}

	result, execErr := h.actionExecutor.ExecuteAction(r.Context(), started)
	if execErr != nil {
		result = &unified.ExecutionResult{Success: false, ErrorMessage: execErr.Error()}
	}
	completed, doneEvent, err := unified.CompleteActionExecution(started, result, actor, time.Now().UTC())
	if err != nil {
		writeActionExecutionApplyError(w, err)
		return
	}
	if err := store.RecordActionExecutionResult(completed, doneEvent); err != nil {
		writeActionExecutionPersistError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(actionExecutionResponse{
		ActionID: completed.ID,
		State:    completed.State,
		Result:   completed.Result,
		Audit:    completed,
	}); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "action_execution_encode_failed", "Failed to encode action execution")
	}
}

func actionDecisionActor(h *ResourceHandlers, r *http.Request) string {
	if h != nil {
		if actor := strings.TrimSpace(getAuthUsername(h.cfg, r)); actor != "" {
			return actor
		}
	}
	if actor := strings.TrimSpace(getUserID(r)); actor != "" {
		return actor
	}
	return "api:authenticated"
}

func writeActionDecisionApplyError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, unified.ErrInvalidApprovalOutcome):
		writeJSONErrorWithDetails(w, http.StatusBadRequest, "invalid_action_decision", "Invalid action decision request", map[string]string{
			"outcome": "outcome must be approved or rejected",
		})
	case errors.Is(err, unified.ErrActionNotPending):
		writeJSONError(w, http.StatusConflict, "action_not_pending", "Action is not pending approval")
	case errors.Is(err, unified.ErrActionPlanExpired):
		writeJSONError(w, http.StatusConflict, "action_plan_expired", "Action plan has expired")
	default:
		writeJSONError(w, http.StatusInternalServerError, "action_decision_failed", sanitizeErrorForClient(err, "Action decision failed"))
	}
}

func writeActionExecutionApplyError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, unified.ErrActionNotApproved):
		writeJSONError(w, http.StatusConflict, "action_not_approved", "Action is not approved for execution")
	case errors.Is(err, unified.ErrActionAlreadyExecuting):
		writeJSONError(w, http.StatusConflict, "action_already_executing", "Action is already executing")
	case errors.Is(err, unified.ErrActionExecutionFinal):
		writeJSONError(w, http.StatusConflict, "action_execution_final", "Action execution is already final")
	case errors.Is(err, unified.ErrActionNotExecuting):
		writeJSONError(w, http.StatusConflict, "action_not_executing", "Action is not executing")
	case errors.Is(err, unified.ErrActionPlanExpired):
		writeJSONError(w, http.StatusConflict, "action_plan_expired", "Action plan has expired")
	case errors.Is(err, unified.ErrActionDryRunOnly):
		writeJSONError(w, http.StatusConflict, "action_dry_run_only", "Action plan is dry-run only and cannot be executed")
	default:
		writeJSONError(w, http.StatusInternalServerError, "action_execution_failed", sanitizeErrorForClient(err, "Action execution failed"))
	}
}

func writeActionExecutionPersistError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, unified.ErrActionNotApproved),
		errors.Is(err, unified.ErrActionAlreadyExecuting),
		errors.Is(err, unified.ErrActionExecutionFinal),
		errors.Is(err, unified.ErrActionNotExecuting),
		errors.Is(err, unified.ErrActionPlanExpired),
		errors.Is(err, unified.ErrActionDryRunOnly):
		writeActionExecutionApplyError(w, err)
	default:
		writeJSONError(w, http.StatusInternalServerError, "action_execution_persist_failed", sanitizeErrorForClient(err, "Failed to persist action execution"))
	}
}
