package chat

import (
	"context"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	aitools "github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
)

func isPatrolInvestigationExecution(profile aitools.ExecutionProfile) bool {
	return profile == aitools.ProfilePatrolInvestigation
}

const investigationOutputLimitRecoverySystemPrompt = `You are Pulse Patrol completing an investigation after the previous final response exhausted its output budget. Do not call tools, repeat the investigation, or narrate your reasoning. Synthesize only the evidence already present in the conversation into the required five sections: Investigation Summary, Root Cause, Affected Resources, Recommendation, and Conclusion. Name causal and affected resources with their exact observed canonical name or ID. If the evidence does not establish root cause, say exactly what remains uncertain. Never invent evidence, actions, verification, or remediation.`

const investigationOutputLimitRecoveryAllowance = 4_096

func applyInvestigationOutputLimitRecoveryRequest(req *providers.ChatRequest, profile aitools.ExecutionProfile) bool {
	if req == nil || !isPatrolInvestigationExecution(profile) {
		return false
	}
	req.Tools = nil
	req.ToolChoice = nil
	req.System = investigationOutputLimitRecoverySystemPrompt
	req.MaxTokens = investigationOutputLimitRecoveryAllowance
	req.ReasoningEffort = providers.ReasoningEffortLow
	return true
}

func isInvestigationEvidenceTool(name string) bool {
	return agentcapabilities.IsPatrolInfrastructureEvidenceToolName(name)
}

// investigationTerminalTools are the tools that end an investigation: the
// typed action proposal and the typed conclusion. includePropose is false on
// a final turn that historically offered no proposal.
func investigationTerminalTools(available []providers.Tool, includePropose bool) []providers.Tool {
	var terminal []providers.Tool
	seen := make(map[string]bool, 2)
	for _, tool := range available {
		if seen[tool.Name] {
			continue
		}
		switch tool.Name {
		case agentcapabilities.PatrolProposeActionToolName:
			if !includePropose {
				continue
			}
		case agentcapabilities.PatrolConcludeInvestigationToolName:
		default:
			continue
		}
		seen[tool.Name] = true
		terminal = append(terminal, tool)
	}
	return terminal
}

const investigationFinalConclusionSystemNote = "\nThis is the investigation's final turn. Do not gather more evidence. Record your conclusion now with patrol_conclude_investigation, based only on the evidence already collected; you will then be asked for the final summary."

// Turn-time estimates for reserving a conclusion before the investigation
// deadline. Concluding takes up to two provider calls: the conclusion turn
// and the final summary.
const (
	investigationMinTurnEstimate      = 20 * time.Second
	investigationConclusionTimeMargin = 15 * time.Second
)

// investigationConclusionReserve is how much of the deadline must remain to
// start another evidence turn: that turn plus the conclusion and summary
// turns, each estimated as the slowest turn seen so far, plus a margin. It is
// capped at half the run's budget so short timeouts still gather evidence.
func investigationConclusionReserve(slowestTurn, budget time.Duration) time.Duration {
	reserve := 3*max(slowestTurn, investigationMinTurnEstimate) + investigationConclusionTimeMargin
	if budget > 0 && reserve > budget/2 {
		reserve = budget / 2
	}
	return reserve
}

// patrolSubscriptionInvestigationTurnTimeout is the complete-turn budget for an
// investigation on a subscription agent. That transport buffers the whole turn,
// so Patrol's stream idle timeout is the deadline for the entire turn. Watch
// keeps its short stall budget for latency; an investigation turn starts a CLI
// process, resends the conversation and reasons at high effort.
const patrolSubscriptionInvestigationTurnTimeout = 4 * time.Minute

// investigationStreamIdleTimeout returns the caller-owned provider stall
// budget for one turn. Only an investigation on a buffered subscription
// transport is widened; every other profile and provider keeps base.
func investigationStreamIdleTimeout(profile aitools.ExecutionProfile, providerName string, base time.Duration) time.Duration {
	if !isPatrolInvestigationExecution(profile) || base <= 0 || base >= patrolSubscriptionInvestigationTurnTimeout {
		return base
	}
	switch providers.SubscriptionAgent(providerName) {
	case providers.SubscriptionAgentCodex, providers.SubscriptionAgentClaude:
		return patrolSubscriptionInvestigationTurnTimeout
	default:
		return base
	}
}

// investigationSummaryTimeout bounds the final summary call. Interactive chat
// keeps the historical 20s bound; an investigation may use the time left
// before its deadline, because a slow provider's summary is its result.
func investigationSummaryTimeout(ctx context.Context, profile aitools.ExecutionProfile, base time.Duration) time.Duration {
	if !isPatrolInvestigationExecution(profile) {
		return base
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		return base
	}
	if remaining := time.Until(deadline); remaining > base {
		return remaining
	}
	return base
}
