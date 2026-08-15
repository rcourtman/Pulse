package chat

import (
	"fmt"

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

const investigationEvidenceStartRepairSystemPrompt = `

INVESTIGATION GROUNDING: Pulse cannot accept a completed investigation until at least one structured call to an advertised evidence tool succeeds. Choose the most relevant available tool and arguments from the exact finding and resource identity already supplied. Call the tool now through the structured tool interface. Do not narrate, simulate, or place a hypothetical tool call in prose. A failed or policy-blocked call is not evidence.`

const investigationEvidenceBudgetExhaustedSystemPrompt = `

INVESTIGATION COMPLETION: The evidence-call budget is exhausted. No more evidence tools are available. Use the evidence already collected. If it supports a safe advertised remediation and no proposal has been submitted, you must call patrol_propose_action once before the final summary; never leave that remediation only as prose. Otherwise produce the required final summary and state any remaining uncertainty.`

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

func investigationEvidenceTools(available []providers.Tool) []providers.Tool {
	filtered := make([]providers.Tool, 0, len(available))
	for _, tool := range available {
		if isInvestigationEvidenceTool(tool.Name) {
			filtered = append(filtered, tool)
		}
	}
	return filtered
}

func applyInvestigationEvidenceStartRepairRequest(req *providers.ChatRequest, profile aitools.ExecutionProfile, available []providers.Tool) bool {
	if req == nil || !isPatrolInvestigationExecution(profile) {
		return false
	}
	evidenceTools := investigationEvidenceTools(available)
	if len(evidenceTools) == 0 {
		return false
	}
	req.Tools = evidenceTools
	req.ToolChoice = &providers.ToolChoice{Type: providers.ToolChoiceRequired}
	req.System += investigationEvidenceStartRepairSystemPrompt
	return true
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
		"[Patrol evidence checkpoint: %d evidence calls used, %d remain. Decide whether the evidence now supports all four completion questions: current symptom, most likely root cause or explicit uncertainty, affected scope, and a safe next action. If it does, conclude now; when the safe next action is an advertised remediation, submit it through patrol_propose_action before the final summary instead of leaving it only as prose. Otherwise spend only targeted calls on a named evidence gap.]",
		used, remaining,
	), "checkpoint")
}

func maybeInjectInvestigationEvidenceBudgetWarning(messages []providers.Message, used, remaining int) bool {
	return appendInvestigationBudgetMessage(messages, fmt.Sprintf(
		"[Patrol evidence budget: %d evidence calls used, %d remain. Stop exploratory investigation. Use at most the remaining targeted calls. If the evidence supports a safe advertised remediation, submit one typed proposal before the final summary; never leave it only as prose. Otherwise conclude with the required summary and explicit uncertainty.]",
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
