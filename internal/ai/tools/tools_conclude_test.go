package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func executeConclude(t *testing.T, exec *PulseToolExecutor, id string, args map[string]interface{}) CallToolResult {
	t.Helper()
	result, err := exec.ExecuteInvocation(context.Background(), ToolInvocation{
		ID:        id,
		Name:      agentcapabilities.PatrolConcludeInvestigationToolName,
		Arguments: args,
	})
	require.NoError(t, err)
	require.NotEmpty(t, result.Content)
	return result
}

func TestParseConclusionOutcomeAcceptsOnlyTheClosedVocabulary(t *testing.T) {
	for input, want := range map[string]aicontracts.InvestigationOutcome{
		"resolved":        aicontracts.OutcomeResolved,
		" CANNOT_FIX ":    aicontracts.OutcomeCannotFix,
		"Needs_Attention": aicontracts.OutcomeNeedsAttention,
	} {
		got, ok := ParseConclusionOutcome(input)
		assert.True(t, ok, input)
		assert.Equal(t, want, got, input)
	}
	for _, input := range []string{"", "fix_verified", "timed_out", "unclassified", "resolved: yes"} {
		_, ok := ParseConclusionOutcome(input)
		assert.False(t, ok, "%q must not parse as a conclusion", input)
	}
}

func TestRecordConclusionKeepsTheFirstConclusion(t *testing.T) {
	capture := NewProposalCapture(ProposalIdentity{ProposalID: "p1", FindingID: "f1", InvestigationID: "i1"}, nil)
	assert.Nil(t, capture.Conclusion())

	recorded, err := capture.RecordConclusion("call-1", aicontracts.OutcomeResolved, "  usage is back under threshold  ")
	require.NoError(t, err)
	assert.Equal(t, aicontracts.OutcomeResolved, recorded.Outcome)
	assert.Equal(t, "usage is back under threshold", recorded.Reason)

	_, err = capture.RecordConclusion("call-2", aicontracts.OutcomeResolved, "replayed")
	require.NoError(t, err, "replaying the same outcome is accepted")

	_, err = capture.RecordConclusion("call-3", aicontracts.OutcomeNeedsAttention, "changed my mind")
	require.Error(t, err, "a different outcome must not replace the recorded conclusion")

	got := capture.Conclusion()
	require.NotNil(t, got)
	assert.Equal(t, aicontracts.OutcomeResolved, got.Outcome)
	assert.Equal(t, "usage is back under threshold", got.Reason)
	assert.Equal(t, "call-1", got.InvocationID)

	long := NewProposalCapture(ProposalIdentity{}, nil)
	recorded, err = long.RecordConclusion("call-1", aicontracts.OutcomeCannotFix, strings.Repeat("x", maxConclusionReasonRunes+50))
	require.NoError(t, err)
	assert.Len(t, []rune(recorded.Reason), maxConclusionReasonRunes)
}

func TestConcludeInvestigationIsInvestigationProfileOnly(t *testing.T) {
	args := map[string]interface{}{"outcome": "needs_attention", "reason": "replace the failing disk"}
	for _, profile := range []ExecutionProfile{ProfileInteractiveAssistant, ProfilePatrolDetection} {
		exec := NewPulseToolExecutor(ExecutorConfig{})
		exec.SetControlLevel(ControlLevelAutonomous)
		exec.ApplyExecutionProfile(profile)
		exec.SetProposalCapture(NewProposalCapture(ProposalIdentity{}, nil))

		result := executeConclude(t, exec, "call-x", args)
		assert.Contains(t, result.Content[0].Text, "Invocation blocked",
			"profile %d must reject the conclusion capture at the registry boundary", profile)
		for _, tool := range exec.registry.ListTools(exec.invocationPolicy()) {
			assert.NotEqual(t, agentcapabilities.PatrolConcludeInvestigationToolName, tool.Name,
				"profile %d must not offer the conclusion capture", profile)
		}
	}

	// Without the request-local capture the tool is not offered at all.
	bare := NewPulseToolExecutor(ExecutorConfig{})
	bare.ApplyExecutionProfile(ProfilePatrolInvestigation)
	for _, tool := range bare.ListTools() {
		assert.NotEqual(t, agentcapabilities.PatrolConcludeInvestigationToolName, tool.Name,
			"an investigation without a capture must not offer the conclusion tool")
	}

	capture := NewProposalCapture(ProposalIdentity{ProposalID: "p1", FindingID: "f1", InvestigationID: "i1"}, nil)
	exec := newInvestigationExecutor(t, capture)
	offered := false
	for _, tool := range exec.ListTools() {
		if tool.Name == agentcapabilities.PatrolConcludeInvestigationToolName {
			offered = true
		}
	}
	assert.True(t, offered, "investigation profile must offer patrol_conclude_investigation")
	assert.False(t, exec.InvestigationConcluded())

	result := executeConclude(t, exec, "call-a", args)
	assert.False(t, result.IsError, result.Content[0].Text)
	assert.Contains(t, result.Content[0].Text, "needs_attention")
	assert.True(t, exec.InvestigationConcluded())
	require.NotNil(t, capture.Conclusion())
	assert.Equal(t, aicontracts.OutcomeNeedsAttention, capture.Conclusion().Outcome)
	assert.Equal(t, "call-a", capture.Conclusion().InvocationID)
}

func TestConcludeInvestigationRefusesInvalidArguments(t *testing.T) {
	exec := newInvestigationExecutor(t, NewProposalCapture(ProposalIdentity{ProposalID: "p1", FindingID: "f1", InvestigationID: "i1"}, nil))

	for name, args := range map[string]map[string]interface{}{
		"unknown outcome": {"outcome": "fix_verified", "reason": "done"},
		"missing reason":  {"outcome": "resolved"},
		"blank reason":    {"outcome": "resolved", "reason": "   "},
	} {
		result := executeConclude(t, exec, "call-"+strings.ReplaceAll(name, " ", "-"), args)
		assert.True(t, result.IsError, "%s must be refused: %s", name, result.Content[0].Text)
	}
	assert.False(t, exec.InvestigationConcluded(), "refused calls must not record a conclusion")
}
