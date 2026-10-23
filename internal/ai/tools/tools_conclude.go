package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
)

// maxConclusionReasonRunes bounds the recorded reason. The run's summary
// carries the full narrative; the conclusion only needs its justification.
const maxConclusionReasonRunes = 2000

// CapturedConclusion is the one typed zero-action conclusion an investigation
// recorded. Outcome is aicontracts.OutcomeResolved, OutcomeCannotFix or
// OutcomeNeedsAttention.
type CapturedConclusion struct {
	Outcome      aicontracts.InvestigationOutcome
	Reason       string
	InvocationID string
}

// ParseConclusionOutcome maps a model-authored outcome onto the closed
// conclusion vocabulary. Anything else is refused, never defaulted.
func ParseConclusionOutcome(value string) (aicontracts.InvestigationOutcome, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(aicontracts.OutcomeResolved):
		return aicontracts.OutcomeResolved, true
	case string(aicontracts.OutcomeCannotFix):
		return aicontracts.OutcomeCannotFix, true
	case string(aicontracts.OutcomeNeedsAttention):
		return aicontracts.OutcomeNeedsAttention, true
	default:
		return "", false
	}
}

// RecordConclusion keeps the first conclusion for the run. Replaying the same
// outcome is accepted; a different outcome is refused so the recorded
// conclusion cannot flip after the model has acted on it.
func (c *ProposalCapture) RecordConclusion(invocationID string, outcome aicontracts.InvestigationOutcome, reason string) (*CapturedConclusion, error) {
	if c == nil {
		return nil, errors.New("investigation conclusions are not available in this run")
	}
	reason = strings.TrimSpace(reason)
	if runes := []rune(reason); len(runes) > maxConclusionReasonRunes {
		reason = string(runes[:maxConclusionReasonRunes])
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conclusion != nil {
		existing := *c.conclusion
		if existing.Outcome == outcome {
			return &existing, nil
		}
		return &existing, fmt.Errorf("this investigation already concluded %s; a different conclusion needs a new investigation", existing.Outcome)
	}
	c.conclusion = &CapturedConclusion{Outcome: outcome, Reason: reason, InvocationID: invocationID}
	recorded := *c.conclusion
	return &recorded, nil
}

// Conclusion returns a copy of the recorded conclusion, nil when none exists.
func (c *ProposalCapture) Conclusion() *CapturedConclusion {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conclusion == nil {
		return nil
	}
	recorded := *c.conclusion
	return &recorded
}

// InvestigationConcluded reports whether this run has recorded its
// conclusion. The agentic loop then asks only for the final summary.
func (e *PulseToolExecutor) InvestigationConcluded() bool {
	return e != nil && e.proposalCapture.Conclusion() != nil
}

// registerConcludeTool registers the investigation-only conclusion capture.
// The registry policy rejects it outside the Patrol investigation profile,
// and it is offered only when the request-local capture exists.
func (e *PulseToolExecutor) registerConcludeTool() {
	e.registry.registerBuiltin(RegisteredTool{
		Definition: Tool{
			Name: agentcapabilities.PatrolConcludeInvestigationToolName,
			Description: `Record this investigation's conclusion. Call it once, when the evidence supports a conclusion, then write the final summary. Choose exactly one outcome:
- resolved: the evidence shows the finding's condition no longer holds.
- cannot_fix: the problem is real, but no advertised capability can remediate it.
- needs_attention: a person must act or decide; the reason says what they should do.

If you proposed an action, its planning result is the investigation outcome and this call is optional. Side-effect-free: it records the conclusion for this run only and plans, approves or executes nothing.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"outcome": {
						Type:        "string",
						Description: "The conclusion this investigation reached",
						Enum:        []string{string(aicontracts.OutcomeResolved), string(aicontracts.OutcomeCannotFix), string(aicontracts.OutcomeNeedsAttention)},
					},
					"reason": {
						Type:        "string",
						Description: "The evidence for this conclusion and, for needs_attention, what the person should do",
					},
				},
				Required: []string{"outcome", "reason"},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeConcludeInvestigation(ctx, args)
		},
		Governance: ToolGovernance{
			ActionMode:      ToolActionRead,
			ApprovalPolicy:  ToolApprovalScopeOnly,
			ApprovalSummary: "records the investigation's typed conclusion without changing infrastructure or Pulse state",
			Summary:         "Records one typed zero-action conclusion during a Patrol investigation; the orchestrator persists the outcome.",
		},
	})
}

// executeConcludeInvestigation validates and records the run's conclusion.
func (e *PulseToolExecutor) executeConcludeInvestigation(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	capture := e.proposalCapture
	if capture == nil {
		return NewErrorResult(fmt.Errorf("investigation conclusions are not available in this run")), nil
	}
	outcome, ok := ParseConclusionOutcome(stringArg(args, "outcome"))
	if !ok {
		return NewErrorResult(fmt.Errorf("outcome must be one of resolved, cannot_fix or needs_attention")), nil
	}
	reason := strings.TrimSpace(stringArg(args, "reason"))
	if reason == "" {
		return NewErrorResult(fmt.Errorf("reason is required")), nil
	}
	recorded, err := capture.RecordConclusion(InvocationIDFromContext(ctx), outcome, reason)
	if err != nil {
		return NewErrorResult(err), nil
	}
	return NewJSONResult(map[string]interface{}{
		"recorded_outcome": string(recorded.Outcome),
		"next_step":        "Write the final summary sections now: Investigation Summary, Root Cause, Affected Resources and Recommendation. Do not call further tools.",
	}), nil
}
