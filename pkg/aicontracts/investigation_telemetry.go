package aicontracts

// InvestigationHandoffReason is content-free attribution, not permission to
// act. The model may describe its decision with the first four values; core
// records the remaining values from its own capability and action facts.
type InvestigationHandoffReason string

const (
	HandoffHumanDecision           InvestigationHandoffReason = "human_decision"
	HandoffMissingEvidence         InvestigationHandoffReason = "missing_evidence"
	HandoffUnsupportedRemediation  InvestigationHandoffReason = "unsupported_remediation"
	HandoffUnspecified             InvestigationHandoffReason = "unspecified"
	HandoffCommandsDisabled        InvestigationHandoffReason = "commands_disabled"
	HandoffGuestActionsUnavailable InvestigationHandoffReason = "guest_actions_unavailable"
	HandoffActionExpired           InvestigationHandoffReason = "action_expired"
	HandoffPlanningFailed          InvestigationHandoffReason = "planning_failed"
)

// Valid validates a closed wire/storage vocabulary; customer text is never a
// fallback value. Attribution of a model decision remains model-reported.
func (r InvestigationHandoffReason) Valid() bool {
	switch r {
	case HandoffHumanDecision, HandoffMissingEvidence, HandoffUnsupportedRemediation,
		HandoffUnspecified, HandoffCommandsDisabled, HandoffGuestActionsUnavailable,
		HandoffActionExpired, HandoffPlanningFailed:
		return true
	default:
		return false
	}
}
