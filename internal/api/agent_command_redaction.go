package api

import (
	"net/http"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func requestCanReadAgentCommandPayloads(r *http.Request) bool {
	token := getAPITokenRecordFromRequest(r)
	if token == nil {
		return true
	}
	return token.HasScope(config.ScopeAIExecute)
}

func redactAgentEventCommandsForRequest(event AgentEvent, r *http.Request) AgentEvent {
	if requestCanReadAgentCommandPayloads(r) {
		return event
	}
	switch payload := event.Payload.(type) {
	case AgentEventApprovalPendingPayload:
		event.Payload = redactAgentEventApprovalPendingCommand(payload)
	case *AgentEventApprovalPendingPayload:
		if payload != nil {
			redacted := redactAgentEventApprovalPendingCommand(*payload)
			event.Payload = &redacted
		}
	case AgentEventActionCompletedPayload:
		event.Payload = redactAgentEventActionCompletedCommand(payload)
	case *AgentEventActionCompletedPayload:
		if payload != nil {
			redacted := redactAgentEventActionCompletedCommand(*payload)
			event.Payload = &redacted
		}
	}
	return event
}

func redactAgentResourceContextCommandsForRequest(bundle *AgentResourceContext, r *http.Request) {
	if bundle == nil || requestCanReadAgentCommandPayloads(r) {
		return
	}
	for i := range bundle.PendingApprovals {
		bundle.PendingApprovals[i] = redactAgentResourceApprovalCommand(bundle.PendingApprovals[i])
	}
	for i := range bundle.RecentActions {
		bundle.RecentActions[i] = redactAgentResourceActionCommand(bundle.RecentActions[i])
	}
}

func redactAgentEventApprovalPendingCommand(payload AgentEventApprovalPendingPayload) AgentEventApprovalPendingPayload {
	if payload.Command != "" {
		payload.Command = ""
		payload.CommandRedacted = true
	}
	return payload
}

func redactAgentEventActionCompletedCommand(payload AgentEventActionCompletedPayload) AgentEventActionCompletedPayload {
	if payload.ActionResultV2 != nil {
		payload.ActionResultSummary = summarizeAgentActionResult(payload.ActionResultV2)
		payload.ActionResultV2 = nil
		payload.Success = payload.ActionResultSummary.Execution.Status == unified.ActionExecutionSucceeded
		payload.CommandRedacted = true
	}
	if payload.ErrorMessage != "" {
		payload.ErrorMessage = redactAgentActionError(payload.ErrorMessage)
		payload.CommandRedacted = true
	}
	if payload.Command != "" {
		payload.Command = ""
		payload.CommandRedacted = true
	}
	if payload.Verification != nil {
		verification := *payload.Verification
		redactAgentResourceVerificationCommand(&verification)
		payload.Verification = &verification
	}
	return payload
}

func redactAgentResourceApprovalCommand(summary AgentResourceApprovalSummary) AgentResourceApprovalSummary {
	if summary.Command != "" {
		summary.Command = ""
		summary.CommandRedacted = true
	}
	return summary
}

func redactAgentResourceActionCommand(summary AgentResourceActionSummary) AgentResourceActionSummary {
	if summary.ActionResultV2 != nil {
		summary.ActionResultSummary = summarizeAgentActionResult(summary.ActionResultV2)
		summary.ActionResultV2 = nil
		summary.Success = summary.ActionResultSummary.Execution.Status == unified.ActionExecutionSucceeded
		summary.CommandRedacted = true
	}
	if summary.ErrorMessage != "" {
		summary.ErrorMessage = redactAgentActionError(summary.ErrorMessage)
		summary.CommandRedacted = true
	}
	if summary.Command != "" {
		summary.Command = ""
		summary.CommandRedacted = true
	}
	if summary.Verification != nil {
		verification := *summary.Verification
		redactAgentResourceVerificationCommand(&verification)
		summary.Verification = &verification
	}
	return summary
}

func redactAgentResourceVerificationCommand(verification *AgentResourceActionVerification) {
	if verification == nil {
		return
	}
	if verification.Command != "" || verification.Note != "" {
		verification.Command = ""
		verification.Note = ""
		verification.CommandRedacted = true
	}
}

// Error text can echo an arbitrary command or provider output, even after
// known credential patterns have been scrubbed. Keep only the documented,
// closed refusal vocabulary, never the free-form suffix (or an unknown prefix).
func redactAgentActionError(message string) string {
	prefix, _, found := strings.Cut(message, ":")
	if found {
		switch prefix {
		case "resource_remediation_locked", "plan_drift", "action_plan_drift",
			"action_plan_expired", "action_dry_run_only", "action_not_approved",
			"action_already_executing", "action_execution_final",
			"action_execution_unavailable", "action_executor_unavailable":
			return prefix + ":"
		}
	}
	return "[redacted-action-detail]"
}
