package agentcapabilities

import "encoding/json"

// AgentErrCode* constants are the snake_case values exposed by the
// agent-surface error envelope and advertised in manifest errorCodes.
const (
	AgentErrCodeResourceNotFound           = "resource_not_found"
	AgentErrCodeOperatorStateNotSet        = "operator_state_not_set"
	AgentErrCodeOperatorStateInvalid       = "operator_state_invalid"
	AgentErrCodeInvalidFindingRequest      = "invalid_finding_request"
	AgentErrCodeFindingNotFound            = "finding_not_found"
	AgentErrCodeFindingActionNotAllowed    = "finding_action_not_allowed"
	AgentErrCodePatrolUnavailable          = "patrol_unavailable"
	AgentErrCodeInvalidActionRequest       = "invalid_action_request"
	AgentErrCodeCapabilityNotFound         = "capability_not_found"
	AgentErrCodeActionExecutionUnavailable = "action_execution_unavailable"
	AgentErrCodeActionActorUnavailable     = "action_actor_unavailable"
	AgentErrCodeMissingID                  = "missing_id"
	AgentErrCodeInvalidID                  = "invalid_id"
	AgentErrCodeInvalidActionDecision      = "invalid_action_decision"
	AgentErrCodeActionNotFound             = "action_not_found"
	AgentErrCodeActionNotPending           = "action_not_pending"
	AgentErrCodeActionPlanExpired          = "action_plan_expired"
	AgentErrCodeActionApprovalForbidden    = "action_approval_forbidden"
	AgentErrCodeActionStepUpUnavailable    = "action_step_up_unavailable"
	AgentErrCodeActionDecisionConflict     = "action_decision_conflict"
	AgentErrCodeActionSeparationRequired   = "action_separation_required"
	AgentErrCodeActionReplanRequired       = "action_replan_required"
	AgentErrCodeInvalidActionExecution     = "invalid_action_execution"
	AgentErrCodeActionNotApproved          = "action_not_approved"
	AgentErrCodeActionAlreadyExecuting     = "action_already_executing"
	AgentErrCodeActionExecutionFinal       = "action_execution_final"
	AgentErrCodeActionExecutionForbidden   = "action_execution_forbidden"
	AgentErrCodeActionNotExecuting         = "action_not_executing"
	AgentErrCodeActionDryRunOnly           = "action_dry_run_only"
	AgentErrCodeActionPlanDrift            = "action_plan_drift"
	AgentErrCodeActionPlanIdentityMismatch = "action_plan_identity_mismatch"
	AgentErrCodeResourceRemediationLocked  = "resource_remediation_locked"
	AgentErrCodeMockModeEnabled            = "mock_mode_enabled"
	AgentErrCodeActionExecutorUnavailable  = "action_executor_unavailable"
	AgentErrCodeActionQueueUnavailable     = "action_queue_unavailable"
	AgentErrCodeActionQueueQueryFailed     = "action_queue_query_failed"
	AgentErrCodeActionQueueEncodeFailed    = "action_queue_encode_failed"
	AgentErrCodeInvalidActionListLimit     = "invalid_action_list_limit"
	AgentErrCodeInvalidActionListView      = "invalid_action_list_view"
	AgentErrCodeActionListFailed           = "action_list_failed"
	AgentErrCodeActionListEncodeFailed     = "action_list_encode_failed"
	AgentErrCodeActionDetailFailed         = "action_detail_failed"
	AgentErrCodeActionDetailEncodeFailed   = "action_detail_encode_failed"
	AgentErrCodeRawCommandRetired          = "raw_command_retired"
)

// ErrorEnvelope is the stable failure shape emitted by Pulse Intelligence
// agent-surface endpoints. Agents branch on Error, not on Message.
type ErrorEnvelope struct {
	Error   string            `json:"error"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

// NewErrorEnvelope builds an agent-stable error envelope while preserving the
// two-field JSON shape when there are no structured details.
func NewErrorEnvelope(code, message string, details map[string]string) ErrorEnvelope {
	if len(details) == 0 {
		details = nil
	}
	return ErrorEnvelope{
		Error:   code,
		Message: message,
		Details: details,
	}
}

// DecodeErrorEnvelope parses a response body as an agent-stable error envelope.
// The boolean is false when the body is not the shared shape.
func DecodeErrorEnvelope(body []byte) (ErrorEnvelope, bool) {
	var envelope ErrorEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return ErrorEnvelope{}, false
	}
	if envelope.Error == "" || envelope.Message == "" {
		return ErrorEnvelope{}, false
	}
	return envelope, true
}
