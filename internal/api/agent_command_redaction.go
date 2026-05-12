package api

import (
	"net/http"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
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
	if verification == nil || verification.Command == "" {
		return
	}
	verification.Command = ""
	verification.CommandRedacted = true
}
