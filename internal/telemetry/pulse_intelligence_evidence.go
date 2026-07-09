package telemetry

import (
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// PulseIntelligenceAIUsageEvidence is the content-free subset of AI usage
// history used to prove Pulse Intelligence loop collaboration.
type PulseIntelligenceAIUsageEvidence struct {
	AssistantAICalls        int
	AssistantContextAICalls int
	AssistantToolCalls      int
	PatrolAICalls           int
}

// PulseIntelligenceAIUsageEvidenceFromHistory projects AI usage history into
// the same content-free Pulse Intelligence evidence used by telemetry and the
// agent operations-loop status endpoint.
func PulseIntelligenceAIUsageEvidenceFromHistory(history *config.AIUsageHistoryData, since time.Time) PulseIntelligenceAIUsageEvidence {
	var evidence PulseIntelligenceAIUsageEvidence
	if history == nil {
		return evidence
	}
	for _, event := range history.Events {
		if event.Timestamp.IsZero() || event.Timestamp.Before(since) {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(event.UseCase)) {
		case "chat":
			evidence.AssistantAICalls++
			if PulseIntelligenceAssistantContextEvent(event) {
				evidence.AssistantContextAICalls++
			}
			if event.ToolCallCount > 0 {
				evidence.AssistantToolCalls += event.ToolCallCount
			}
		case "patrol":
			evidence.PatrolAICalls++
		}
	}
	return evidence
}

// PulseIntelligenceAssistantContextEvent reports whether a chat usage event
// carried governed context. It deliberately ignores prompt text and responses.
func PulseIntelligenceAssistantContextEvent(event config.AIUsageEventRecord) bool {
	return strings.TrimSpace(event.ContextScope) != "" ||
		strings.TrimSpace(event.TargetType) != "" ||
		strings.TrimSpace(event.TargetID) != "" ||
		strings.TrimSpace(event.FindingID) != ""
}

// PulseIntelligenceExternalAgentEvidence is the content-free subset of
// authenticated agent/MCP route activity used to prove external collaboration.
type PulseIntelligenceExternalAgentEvidence struct {
	Used                  bool
	MCPAdapterUsed        bool
	ContextRequests       int
	EventStreamRequests   int
	ProvisioningRequests  int
	OperatorStateRequests int
	FindingRequests       int
	ActionRequests        int
}

// CollaborationActive reports whether an external agent or MCP adapter used a
// manifest-published Pulse Intelligence capability in the evidence window.
func (e PulseIntelligenceExternalAgentEvidence) CollaborationActive() bool {
	return e.Used ||
		e.MCPAdapterUsed ||
		e.ContextRequests > 0 ||
		e.EventStreamRequests > 0 ||
		e.ProvisioningRequests > 0 ||
		e.OperatorStateRequests > 0 ||
		e.FindingRequests > 0 ||
		e.ActionRequests > 0
}

// CollaborationCount returns a coarse activity count for step-level rollups.
func (e PulseIntelligenceExternalAgentEvidence) CollaborationCount() int {
	count := e.ContextRequests +
		e.EventStreamRequests +
		e.ProvisioningRequests +
		e.OperatorStateRequests +
		e.FindingRequests +
		e.ActionRequests
	if count > 0 {
		return count
	}
	if e.Used || e.MCPAdapterUsed {
		return 1
	}
	return 0
}

// PulseIntelligenceExternalAgentEvidenceFromHistory projects external-agent
// activity history into content-free collaboration evidence.
func PulseIntelligenceExternalAgentEvidenceFromHistory(history *config.ExternalAgentActivityHistoryData, since time.Time) PulseIntelligenceExternalAgentEvidence {
	var evidence PulseIntelligenceExternalAgentEvidence
	if history == nil {
		return evidence
	}
	for _, event := range history.Events {
		if event.Timestamp.IsZero() || event.Timestamp.Before(since) {
			continue
		}
		if !PulseIntelligenceExternalAgentActivitySurface(event.Surface) {
			continue
		}
		evidence.Used = true
		if strings.TrimSpace(event.Surface) == config.ExternalAgentActivitySurfacePulseMCP {
			evidence.MCPAdapterUsed = true
		}
		evidence.ApplyActivity(event.Activity)
	}
	return evidence
}

// PulseIntelligenceExternalAgentActivitySurface reports whether a persisted
// activity surface is part of the governed external-agent contract.
func PulseIntelligenceExternalAgentActivitySurface(surface string) bool {
	switch strings.TrimSpace(surface) {
	case config.ExternalAgentActivitySurfaceAgentAPI, config.ExternalAgentActivitySurfacePulseMCP:
		return true
	default:
		return false
	}
}

// ApplyActivity increments the matching content-free external-agent activity
// bucket.
func (e *PulseIntelligenceExternalAgentEvidence) ApplyActivity(activity string) {
	if e == nil {
		return
	}
	switch strings.TrimSpace(activity) {
	case config.ExternalAgentActivityResourceContext, config.ExternalAgentActivityFleetContext:
		e.ContextRequests++
	case config.ExternalAgentActivityEventStream:
		e.EventStreamRequests++
	case config.ExternalAgentActivityProvisioning:
		e.ProvisioningRequests++
	case config.ExternalAgentActivityOperatorState:
		e.OperatorStateRequests++
	case config.ExternalAgentActivityFindingList, config.ExternalAgentActivityFindingDecision:
		e.FindingRequests++
	case config.ExternalAgentActivityActionPlan, config.ExternalAgentActivityActionDecision, config.ExternalAgentActivityActionExecute:
		e.ActionRequests++
	}
}
