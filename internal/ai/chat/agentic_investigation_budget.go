package chat

import (
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

func investigationTerminalTools(available []providers.Tool) []providers.Tool {
	for _, tool := range available {
		if tool.Name == agentcapabilities.PatrolProposeActionToolName {
			return []providers.Tool{tool}
		}
	}
	return nil
}
