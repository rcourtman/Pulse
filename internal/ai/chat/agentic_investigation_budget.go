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

type investigationProposalBasis struct {
	TargetResourceID string
	CausalResourceID string
	CapabilityName   string
	Reason           string
}

func investigationProposalBasisFromToolCall(call providers.ToolCall) *investigationProposalBasis {
	if call.Name != agentcapabilities.PatrolProposeActionToolName {
		return nil
	}
	stringInput := func(key string) string {
		value, _ := call.Input[key].(string)
		return strings.TrimSpace(value)
	}
	basis := &investigationProposalBasis{
		TargetResourceID: stringInput("resource_id"),
		CausalResourceID: stringInput("causal_resource_id"),
		CapabilityName:   stringInput("capability_name"),
		Reason:           stringInput("reason"),
	}
	if basis.TargetResourceID == "" || basis.CausalResourceID == "" || basis.CapabilityName == "" || basis.Reason == "" {
		return nil
	}
	return basis
}

func investigationProposalCompletionSystemPrompt(basis *investigationProposalBasis) string {
	prompt := `

INVESTIGATION COMPLETION: A typed action proposal has already been accepted for this run. Do not call more tools. Produce the required investigation summary from the evidence already collected and state that the proposal is pending governed policy or operator handling; never claim that it executed.`
	if basis == nil {
		return prompt
	}
	return prompt + fmt.Sprintf(`

ACCEPTED PROPOSAL EVIDENCE CHECKPOINT: The governed proposal record identifies causal resource %q, action target %q, capability %q, and this exact recorded basis: %q. Preserve the causal resource identity and recorded basis in the Root Cause section, distinguish the action target in Affected Resources, and do not contradict or downgrade facts already used to justify the accepted proposal.`, basis.CausalResourceID, basis.TargetResourceID, basis.CapabilityName, basis.Reason)
}

// groundInvestigationConclusionInProposal makes the typed proposal record the
// terminal source of truth for facts the model already committed as its action
// rationale. Provider prose remains free-form, but it cannot omit the exact
// causal identity or rationale after Patrol has accepted them structurally.
func groundInvestigationConclusionInProposal(content string, basis *investigationProposalBasis) (string, string) {
	if basis == nil || strings.TrimSpace(content) == "" {
		return content, ""
	}
	reason := strings.Join(strings.Fields(basis.Reason), " ")
	checkpoint := fmt.Sprintf("Proposal evidence checkpoint: causal resource `%s`; recorded basis: %s", basis.CausalResourceID, reason)
	rootCause := markdownInvestigationSection(content, "Root Cause")
	if strings.Contains(strings.ToLower(rootCause), strings.ToLower(basis.CausalResourceID)) &&
		strings.Contains(strings.ToLower(rootCause), strings.ToLower(reason)) {
		return content, ""
	}
	grounded, ok := appendToMarkdownInvestigationSection(content, "Root Cause", checkpoint)
	if !ok {
		separator := "\n\n"
		if strings.TrimSpace(content) == "" {
			separator = ""
		}
		addition := separator + "### Root Cause\n" + checkpoint
		return content + addition, addition
	}
	return grounded, "\n\n" + checkpoint
}

func markdownInvestigationSection(content, heading string) string {
	lines := strings.Split(content, "\n")
	start := -1
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if start < 0 {
			if strings.EqualFold(trimmed, "### "+heading) {
				start = index + 1
			}
			continue
		}
		if strings.HasPrefix(trimmed, "### ") {
			return strings.Join(lines[start:index], "\n")
		}
	}
	if start >= 0 {
		return strings.Join(lines[start:], "\n")
	}
	return ""
}

func appendToMarkdownInvestigationSection(content, heading, addition string) (string, bool) {
	lines := strings.Split(content, "\n")
	start := -1
	insertAt := len(lines)
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if start < 0 {
			if strings.EqualFold(trimmed, "### "+heading) {
				start = index + 1
			}
			continue
		}
		if strings.HasPrefix(trimmed, "### ") {
			insertAt = index
			break
		}
	}
	if start < 0 {
		return content, false
	}
	prefix := append([]string(nil), lines[:insertAt]...)
	for len(prefix) > 0 && strings.TrimSpace(prefix[len(prefix)-1]) == "" {
		prefix = prefix[:len(prefix)-1]
	}
	prefix = append(prefix, "", addition, "")
	prefix = append(prefix, lines[insertAt:]...)
	return strings.Join(prefix, "\n"), true
}

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

func isSuccessfulInvestigationEvidenceResult(name, content string, isError bool) bool {
	content = strings.TrimSpace(content)
	if !isInvestigationEvidenceTool(name) || isError || content == "" {
		return false
	}
	if agentcapabilities.HasPolicyBlockedToolMarker(content) || agentcapabilities.HasApprovalRequiredToolMarker(content) {
		return false
	}
	if code, ok := agentcapabilities.ToolResultErrorCode(content); ok {
		switch code {
		case agentcapabilities.ErrCodePolicyBlocked, agentcapabilities.ErrCodeApprovalRequired:
			return false
		}
	}
	return true
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
		"[Patrol evidence checkpoint: %d evidence calls used, %d remain. Decide whether the evidence now supports all four completion questions: current symptom, most likely root cause or explicit uncertainty, affected scope, and a safe next action. Before concluding that root cause is unknown or proposing remediation on the symptom resource, use available canonical query, discovery, or topology evidence to test at least one plausible causal peer or dependency whenever a cross-resource cause remains plausible. Empty logs or a blocked deep-read path do not close that evidence gap. If the four questions are supported, conclude now; when the safe next action is an advertised remediation, submit it through patrol_propose_action before the final summary instead of leaving it only as prose. Otherwise spend only targeted calls on a named evidence gap.]",
		used, remaining,
	), "checkpoint")
}

func maybeInjectInvestigationEvidenceBudgetWarning(messages []providers.Message, used, remaining int) bool {
	return appendInvestigationBudgetMessage(messages, fmt.Sprintf(
		"[Patrol evidence budget: %d evidence calls used, %d remain. Stop exploratory investigation. Use at most the remaining targeted calls. If a cross-resource cause is still plausible and no causal peer or dependency has been tested with canonical query, discovery, or topology evidence, make that the remaining evidence priority; empty logs or a blocked deep-read path are not a root-cause conclusion. If the evidence supports a safe advertised remediation, submit one typed proposal before the final summary; never leave it only as prose. Otherwise conclude with the required summary and explicit uncertainty.]",
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
