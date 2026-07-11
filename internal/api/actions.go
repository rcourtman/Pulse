package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/actionlifecycle"
	"github.com/rcourtman/pulse-go-rewrite/internal/actionplanner"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

const maxActionPlanRequestBytes = 1 << 20
const maxActionDecisionRequestBytes = 64 << 10
const maxActionExecutionRequestBytes = 64 << 10
const maxPendingActionAudits = 100

// ActionExecutor is the API-facing name for the canonical action lifecycle
// execution contract. The interface is owned by internal/actionlifecycle;
// the alias keeps existing executor implementations and wiring source-stable.
type ActionExecutor = actionlifecycle.Executor

// ActionAvailabilityChecker is the API-facing name for the canonical
// pre-plan readiness contract owned by internal/actionlifecycle.
type ActionAvailabilityChecker = actionlifecycle.AvailabilityChecker

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

type pendingActionsResponse struct {
	Actions []unified.ActionAuditRecord `json:"actions"`
	Count   int                         `json:"count"`
}

// ActionLifecycle returns the shared transport-independent action lifecycle
// service bound to this handler set's registry, store, executor, and
// completion publisher. The REST handlers below and any in-process broker
// (e.g. Patrol) must route through this one service; there is no other
// sanctioned path from a typed action request to execution.
func (h *ResourceHandlers) ActionLifecycle() *actionlifecycle.Service {
	return &actionlifecycle.Service{
		Registry: h.buildRegistry,
		Store: func(orgID string) (actionlifecycle.Store, error) {
			return h.getStore(orgID)
		},
		Executor:           h.actionExecutor,
		OnActionCompleted:  h.actionCompleted,
		OnActionTransition: h.actionTransition,
		PolicyAdmission:    h.policyAdmission,
		EmergencyStop:      h.actionEmergencyStop,
	}
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
		writeJSONErrorWithDetails(w, http.StatusBadRequest, agentcapabilities.AgentErrCodeInvalidActionRequest, "Invalid action planning request", map[string]string{
			"body": "request body must be a valid ActionRequest JSON object",
		})
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeJSONErrorWithDetails(w, http.StatusBadRequest, agentcapabilities.AgentErrCodeInvalidActionRequest, "Invalid action planning request", map[string]string{
			"body": "request body must contain one JSON object",
		})
		return
	}

	plan, err := h.ActionLifecycle().Plan(r.Context(), GetOrgID(r.Context()), req)
	if err != nil {
		writeActionPlanError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(plan); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "action_plan_encode_failed", "Failed to encode action plan")
	}
}

// HandleListPendingActions returns the canonical decision queue. It is not an
// audit-log endpoint and has no enterprise audit-log entitlement dependency;
// authorization is the same ai:execute scope required to decide an action.
func (h *ResourceHandlers) HandleListPendingActions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	store, err := h.getStore(GetOrgID(r.Context()))
	if err != nil || store == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "action_audit_unavailable", "Action decision queue is not available")
		return
	}
	reader, ok := store.(unified.PendingActionAuditReader)
	if !ok {
		writeJSONError(w, http.StatusServiceUnavailable, agentcapabilities.AgentErrCodeActionQueueUnavailable, "Action decision queue is not available")
		return
	}
	actions, err := reader.GetPendingActionAudits(maxPendingActionAudits)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, agentcapabilities.AgentErrCodeActionQueueQueryFailed, "Failed to query pending actions")
		return
	}
	if actions == nil {
		actions = []unified.ActionAuditRecord{}
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(pendingActionsResponse{Actions: actions, Count: len(actions)}); err != nil {
		writeJSONError(w, http.StatusInternalServerError, agentcapabilities.AgentErrCodeActionQueueEncodeFailed, "Failed to encode pending actions")
	}
}

func writeActionPlanError(w http.ResponseWriter, err error) {
	var validationErr *actionplanner.ValidationError
	var notFound *actionlifecycle.ResourceNotFoundError
	var unavailable *actionlifecycle.AvailabilityRefusedError
	var persist *actionlifecycle.PersistError
	switch {
	case errors.As(err, &validationErr):
		details := map[string]string{}
		if validationErr.Field != "" {
			details[validationErr.Field] = validationErr.Message
		}
		writeJSONErrorWithDetails(w, http.StatusBadRequest, agentcapabilities.AgentErrCodeInvalidActionRequest, "Invalid action planning request", details)
	case errors.Is(err, actionplanner.ErrCapabilityNotFound):
		details := map[string]string{}
		var capabilityNotFound *actionlifecycle.CapabilityNotFoundError
		if errors.As(err, &capabilityNotFound) {
			details["resourceId"] = capabilityNotFound.ResourceID
			details["capabilityName"] = capabilityNotFound.CapabilityName
		}
		writeJSONErrorWithDetails(w, http.StatusNotFound, agentcapabilities.AgentErrCodeCapabilityNotFound, "Capability not found on resource", details)
	case errors.As(err, &notFound):
		writeJSONErrorWithDetails(w, http.StatusNotFound, agentcapabilities.AgentErrCodeResourceNotFound, "Resource not found", map[string]string{
			"resourceId": notFound.ResourceID,
		})
	case errors.As(err, &unavailable):
		writeJSONErrorWithDetails(w, http.StatusConflict, agentcapabilities.AgentErrCodeActionExecutionUnavailable, "Action execution is unavailable", map[string]string{
			"resourceId":     unavailable.ResourceID,
			"capabilityName": unavailable.CapabilityName,
			"reasonCode":     unavailable.Readiness.ReasonCode,
			"reason":         firstNonEmpty(unavailable.Readiness.Reason, "action execution is unavailable"),
		})
	case errors.Is(err, actionlifecycle.ErrRegistryUnavailable):
		writeJSONError(w, http.StatusInternalServerError, "resource_registry_unavailable", sanitizeErrorForClient(err, "Resource registry unavailable"))
	case errors.Is(err, actionlifecycle.ErrStoreUnavailable):
		writeJSONError(w, http.StatusServiceUnavailable, "action_audit_unavailable", "Action audit history is not available")
	case errors.As(err, &persist):
		writeJSONError(w, http.StatusInternalServerError, "action_audit_persist_failed", sanitizeErrorForClient(err, "Failed to persist action audit"))
	default:
		writeJSONError(w, http.StatusInternalServerError, "action_plan_failed", sanitizeErrorForClient(err, "Action planning failed"))
	}
}

func (h *ResourceHandlers) HandleDecideAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	actionID := strings.TrimSpace(r.PathValue("id"))
	if actionID == "" {
		writeJSONError(w, http.StatusBadRequest, agentcapabilities.AgentErrCodeMissingID, "Missing action ID")
		return
	}
	if !validAuditEventID.MatchString(actionID) || len(actionID) > 128 {
		writeJSONError(w, http.StatusBadRequest, agentcapabilities.AgentErrCodeInvalidID, "Invalid action ID format")
		return
	}

	var decision actionDecisionRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxActionDecisionRequestBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decision); err != nil {
		writeJSONErrorWithDetails(w, http.StatusBadRequest, agentcapabilities.AgentErrCodeInvalidActionDecision, "Invalid action decision request", map[string]string{
			"body": "request body must be a valid action decision JSON object",
		})
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeJSONErrorWithDetails(w, http.StatusBadRequest, agentcapabilities.AgentErrCodeInvalidActionDecision, "Invalid action decision request", map[string]string{
			"body": "request body must contain one JSON object",
		})
		return
	}
	decision.Outcome = unified.ApprovalOutcome(strings.TrimSpace(string(decision.Outcome)))
	decision.Reason = strings.TrimSpace(decision.Reason)
	if decision.Outcome != unified.OutcomeApproved && decision.Outcome != unified.OutcomeRejected {
		writeJSONErrorWithDetails(w, http.StatusBadRequest, agentcapabilities.AgentErrCodeInvalidActionDecision, "Invalid action decision request", map[string]string{
			"outcome": "outcome must be approved or rejected",
		})
		return
	}

	approval := unified.ActionApprovalRecord{
		Actor:   actionDecisionActor(h, r),
		Method:  unified.MethodAPI,
		Outcome: decision.Outcome,
		Reason:  decision.Reason,
	}
	updated, err := h.ActionLifecycle().Decide(r.Context(), GetOrgID(r.Context()), actionID, approval)
	if err != nil {
		writeActionLifecycleReadError(w, err, func() {
			var persist *actionlifecycle.PersistError
			if errors.As(err, &persist) {
				writeJSONError(w, http.StatusInternalServerError, "action_decision_persist_failed", sanitizeErrorForClient(err, "Failed to persist action decision"))
				return
			}
			writeActionDecisionApplyError(w, err)
		})
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
		writeJSONError(w, http.StatusBadRequest, agentcapabilities.AgentErrCodeMissingID, "Missing action ID")
		return
	}
	if !validAuditEventID.MatchString(actionID) || len(actionID) > 128 {
		writeJSONError(w, http.StatusBadRequest, agentcapabilities.AgentErrCodeInvalidID, "Invalid action ID format")
		return
	}

	var execution actionExecutionRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxActionExecutionRequestBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&execution); err != nil {
		if !errors.Is(err, io.EOF) {
			writeJSONErrorWithDetails(w, http.StatusBadRequest, agentcapabilities.AgentErrCodeInvalidActionExecution, "Invalid action execution request", map[string]string{
				"body": "request body must be a valid action execution JSON object",
			})
			return
		}
	} else if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeJSONErrorWithDetails(w, http.StatusBadRequest, agentcapabilities.AgentErrCodeInvalidActionExecution, "Invalid action execution request", map[string]string{
			"body": "request body must contain one JSON object",
		})
		return
	}
	execution.Reason = strings.TrimSpace(execution.Reason)

	completed, err := h.ActionLifecycle().Execute(r.Context(), GetOrgID(r.Context()), actionID, actionDecisionActor(h, r), execution.Reason)
	if err != nil {
		writeActionLifecycleReadError(w, err, func() {
			writeActionExecuteError(w, err)
		})
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

// writeActionLifecycleReadError handles the store/query/not-found failures
// shared by the decision and execution endpoints, delegating anything else
// to the endpoint-specific fallback.
func writeActionLifecycleReadError(w http.ResponseWriter, err error, fallback func()) {
	var notFound *actionlifecycle.ActionNotFoundError
	var query *actionlifecycle.QueryError
	switch {
	case errors.Is(err, actionlifecycle.ErrStoreUnavailable):
		writeJSONError(w, http.StatusServiceUnavailable, "action_audit_unavailable", "Action audit history is not available")
	case errors.As(err, &query):
		writeJSONError(w, http.StatusInternalServerError, "action_audit_query_failed", "Failed to query action audit")
	case errors.As(err, &notFound):
		writeJSONErrorWithDetails(w, http.StatusNotFound, agentcapabilities.AgentErrCodeActionNotFound, "Action not found", map[string]string{
			"actionId": notFound.ActionID,
		})
	default:
		fallback()
	}
}

func writeActionExecuteError(w http.ResponseWriter, err error) {
	var persist *actionlifecycle.PersistError
	var freshness *actionlifecycle.FreshnessCheckError
	var policy *actionlifecycle.PolicyCheckError
	switch {
	case errors.Is(err, actionlifecycle.ErrExecutorUnavailable):
		writeJSONError(w, http.StatusNotImplemented, agentcapabilities.AgentErrCodeActionExecutorUnavailable, "No action executor is configured for this API instance")
	case errors.As(err, &persist):
		writeActionExecutionPersistError(w, err)
	case errors.As(err, &freshness):
		writeJSONError(w, http.StatusInternalServerError, "action_plan_validation_failed", sanitizeErrorForClient(err, "Failed to validate action plan freshness"))
	case errors.As(err, &policy):
		writeJSONError(w, http.StatusInternalServerError, "action_policy_validation_failed", sanitizeErrorForClient(err, "Failed to validate action policy"))
	default:
		writeActionExecutionApplyError(w, err)
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
		writeJSONErrorWithDetails(w, http.StatusBadRequest, agentcapabilities.AgentErrCodeInvalidActionDecision, "Invalid action decision request", map[string]string{
			"outcome": "outcome must be approved or rejected",
		})
	case errors.Is(err, unified.ErrActionNotPending):
		writeJSONError(w, http.StatusConflict, agentcapabilities.AgentErrCodeActionNotPending, "Action is not pending approval")
	case errors.Is(err, unified.ErrActionPlanExpired):
		writeJSONError(w, http.StatusConflict, agentcapabilities.AgentErrCodeActionPlanExpired, "Action plan has expired")
	default:
		writeJSONError(w, http.StatusInternalServerError, "action_decision_failed", sanitizeErrorForClient(err, "Action decision failed"))
	}
}

func writeActionExecutionApplyError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, unified.ErrActionNotApproved):
		writeJSONError(w, http.StatusConflict, agentcapabilities.AgentErrCodeActionNotApproved, "Action is not approved for execution")
	case errors.Is(err, unified.ErrActionAlreadyExecuting):
		writeJSONError(w, http.StatusConflict, agentcapabilities.AgentErrCodeActionAlreadyExecuting, "Action is already executing")
	case errors.Is(err, unified.ErrActionExecutionFinal):
		writeJSONError(w, http.StatusConflict, agentcapabilities.AgentErrCodeActionExecutionFinal, "Action execution is already final")
	case errors.Is(err, unified.ErrActionNotExecuting):
		writeJSONError(w, http.StatusConflict, "action_not_executing", "Action is not executing")
	case errors.Is(err, unified.ErrActionPlanExpired):
		writeJSONError(w, http.StatusConflict, agentcapabilities.AgentErrCodeActionPlanExpired, "Action plan has expired")
	case errors.Is(err, unified.ErrActionDryRunOnly):
		writeJSONError(w, http.StatusConflict, agentcapabilities.AgentErrCodeActionDryRunOnly, "Action plan is dry-run only and cannot be executed")
	case errors.Is(err, unified.ErrActionPlanDrift):
		writeJSONError(w, http.StatusConflict, agentcapabilities.AgentErrCodeActionPlanDrift, "Action plan no longer matches the current resource contract; re-plan before executing")
	case errors.Is(err, unified.ErrResourceRemediationLocked):
		writeJSONError(w, http.StatusConflict, agentcapabilities.AgentErrCodeResourceRemediationLocked, "Resource is operator-locked against automated remediation")
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
		errors.Is(err, unified.ErrActionDryRunOnly),
		errors.Is(err, unified.ErrResourceRemediationLocked):
		writeActionExecutionApplyError(w, err)
	default:
		writeJSONError(w, http.StatusInternalServerError, "action_execution_persist_failed", sanitizeErrorForClient(err, "Failed to persist action execution"))
	}
}
