package chat

import (
	"fmt"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	aitools "github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rs/zerolog/log"
)

func isPatrolInvestigationExecution(profile aitools.ExecutionProfile) bool {
	return profile == aitools.ProfilePatrolInvestigation
}

const investigationProposalCompletedSystemPrompt = `

INVESTIGATION COMPLETION: A typed action proposal has already been accepted for this run. Do not call more tools. Produce the required investigation summary from the evidence already collected and state that the proposal is pending governed policy or operator handling; never claim that it executed.`

const investigationEvidenceBudgetExhaustedSystemPrompt = `

INVESTIGATION COMPLETION: The evidence-call budget is exhausted. No more evidence tools are available. Use the evidence already collected. If it supports a safe advertised remediation and no proposal has been submitted, you may call patrol_propose_action once; otherwise produce the required final summary and state any remaining uncertainty.`

func isInvestigationEvidenceTool(name string) bool {
	return strings.TrimSpace(name) != agentcapabilities.PatrolProposeActionToolName
}

func investigationTerminalTools(available []providers.Tool) []providers.Tool {
	for _, tool := range available {
		if tool.Name == agentcapabilities.PatrolProposeActionToolName {
			return []providers.Tool{tool}
		}
	}
	return nil
}

func investigationEvidenceCheckpoint(maxEvidenceCalls int) int {
	checkpoint := (maxEvidenceCalls + 1) / 2
	if checkpoint < 3 {
		return 3
	}
	return checkpoint
}

func maybeInjectInvestigationEvidenceCheckpoint(messages []providers.Message, used, remaining int) bool {
	return appendInvestigationBudgetMessage(messages, fmt.Sprintf(
		"[Patrol evidence checkpoint: %d evidence calls used, %d remain. Decide whether the evidence now supports all four completion questions: current symptom, most likely root cause or explicit uncertainty, affected scope, and a safe next action. If it does, conclude now; otherwise spend only targeted calls on a named evidence gap.]",
		used, remaining,
	), "checkpoint")
}

func maybeInjectInvestigationEvidenceBudgetWarning(messages []providers.Message, used, remaining int) bool {
	return appendInvestigationBudgetMessage(messages, fmt.Sprintf(
		"[Patrol evidence budget: %d evidence calls used, %d remain. Stop exploratory investigation. Use at most the remaining targeted calls, then either submit one supported typed proposal or conclude with the required summary and explicit uncertainty.]",
		used, remaining,
	), "warning")
}

func appendInvestigationBudgetMessage(messages []providers.Message, message, phase string) bool {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].ToolResult == nil || messages[i].ToolResult.IsError {
			continue
		}
		messages[i].ToolResult.Content += "\n\n" + message
		log.Info().
			Str("phase", phase).
			Int("message_index", i).
			Msg("[InvestigationEvidenceBudget] Injected evidence-completion guidance")
		return true
	}
	return false
}
