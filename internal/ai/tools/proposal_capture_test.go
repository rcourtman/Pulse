package tools

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func testProposalCatalog() ProposalCatalog {
	return func(ctx context.Context, resourceID string) ([]unified.ResourceCapability, error) {
		if resourceID != "vm:42" {
			return nil, nil
		}
		return []unified.ResourceCapability{
			{
				Name:                 "restart",
				MinimumApprovalLevel: unified.ApprovalAdmin,
				Params: []unified.CapabilityParam{
					{Name: "mode", Type: "string", Required: true, Enum: []string{"graceful", "force"}},
				},
			},
			{
				Name:                 "join_cluster",
				MinimumApprovalLevel: unified.ApprovalAdmin,
				Params: []unified.CapabilityParam{
					{Name: "join_token", Type: "string", Required: true, IsSensitive: true},
				},
			},
		}, nil
	}
}

func newInvestigationExecutor(t *testing.T, capture *ProposalCapture) *PulseToolExecutor {
	t.Helper()
	exec := NewPulseToolExecutor(ExecutorConfig{})
	exec.ApplyExecutionProfile(ProfilePatrolInvestigation)
	exec.SetProposalCapture(capture)
	return exec
}

func proposeArgs() map[string]interface{} {
	return map[string]interface{}{
		"resource_id":     "vm:42",
		"capability_name": "restart",
		"params":          map[string]interface{}{"mode": "graceful"},
		"reason":          "recover the stalled web tier",
	}
}

func executePropose(t *testing.T, exec *PulseToolExecutor, id string, args map[string]interface{}) CallToolResult {
	t.Helper()
	result, err := exec.ExecuteInvocation(context.Background(), ToolInvocation{
		ID:        id,
		Name:      agentcapabilities.PatrolProposeActionToolName,
		Arguments: args,
	})
	require.NoError(t, err)
	require.NotEmpty(t, result.Content)
	return result
}

// The essential proof: two concurrent valid proposal calls latch terminal
// ambiguity with a nil proposal, regardless of execution order.
func TestConcurrentValidProposalsLatchAmbiguityWithNoProposal(t *testing.T) {
	capture := NewProposalCapture(ProposalIdentity{ProposalID: "prop-1", FindingID: "f-1", InvestigationID: "inv-1"}, testProposalCatalog())
	exec := newInvestigationExecutor(t, capture)

	second := proposeArgs()
	second["reason"] = "an alternative remediation"

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); executePropose(t, exec, "call-a", proposeArgs()) }()
	go func() { defer wg.Done(); executePropose(t, exec, "call-b", second) }()
	wg.Wait()

	proposal, _, err := capture.Outcome()
	if !errors.Is(err, ErrProposalAmbiguous) {
		t.Fatalf("outcome error = %v, want ErrProposalAmbiguous", err)
	}
	if proposal != nil {
		t.Fatalf("ambiguous run must invalidate the captured proposal, got %#v", proposal)
	}
}

func TestProposalReplaySemanticsByInvocationID(t *testing.T) {
	capture := NewProposalCapture(ProposalIdentity{InvestigationID: "inv-1"}, testProposalCatalog())
	exec := newInvestigationExecutor(t, capture)

	executePropose(t, exec, "call-a", proposeArgs())
	// Same ID, same payload: idempotent replay.
	replay := executePropose(t, exec, "call-a", proposeArgs())
	assert.Contains(t, replay.Content[0].Text, "Proposal recorded")

	proposal, failed, err := capture.Outcome()
	require.NoError(t, err)
	require.NotNil(t, proposal)
	assert.Equal(t, 0, failed)
	assert.Equal(t, "call-a", proposal.InvocationID)
	assert.Equal(t, "inv-1", proposal.Identity.InvestigationID)

	// Same ID, different payload: terminal integrity error, capture
	// invalidated.
	mutated := proposeArgs()
	mutated["reason"] = "changed my mind"
	conflict := executePropose(t, exec, "call-a", mutated)
	assert.Contains(t, conflict.Content[0].Text, "integrity")

	proposal, _, err = capture.Outcome()
	if !errors.Is(err, ErrProposalIntegrity) {
		t.Fatalf("outcome error = %v, want ErrProposalIntegrity", err)
	}
	assert.Nil(t, proposal)
}

func TestFailedAttemptsWithoutSuccessAreATypedError(t *testing.T) {
	capture := NewProposalCapture(ProposalIdentity{}, testProposalCatalog())
	exec := newInvestigationExecutor(t, capture)

	bad := proposeArgs()
	bad["capability_name"] = "detonate"
	result := executePropose(t, exec, "call-a", bad)
	assert.Contains(t, result.Content[0].Text, "does not advertise")

	proposal, failed, err := capture.Outcome()
	if !errors.Is(err, ErrProposalAttemptsFailed) {
		t.Fatalf("outcome error = %v, want ErrProposalAttemptsFailed", err)
	}
	assert.Nil(t, proposal)
	assert.Equal(t, 1, failed)

	// A clean zero-proposal run stays a valid conclusion.
	clean := NewProposalCapture(ProposalIdentity{}, testProposalCatalog())
	proposal, failed, err = clean.Outcome()
	require.NoError(t, err)
	assert.Nil(t, proposal)
	assert.Equal(t, 0, failed)
}

func TestSensitiveProposalParamsRejectedWithoutEcho(t *testing.T) {
	capture := NewProposalCapture(ProposalIdentity{}, testProposalCatalog())
	exec := newInvestigationExecutor(t, capture)

	args := proposeArgs()
	args["capability_name"] = "join_cluster"
	args["params"] = map[string]interface{}{"join_token": "super-secret-token-value"}
	result := executePropose(t, exec, "call-a", args)
	text := result.Content[0].Text
	assert.Contains(t, text, "sensitive")
	assert.NotContains(t, text, "super-secret-token-value", "refusals must never echo parameter values")

	proposal, failed, err := capture.Outcome()
	if !errors.Is(err, ErrProposalAttemptsFailed) {
		t.Fatalf("outcome error = %v, want ErrProposalAttemptsFailed", err)
	}
	assert.Nil(t, proposal)
	assert.Equal(t, 1, failed)
}

func TestProposeActionIsInvestigationProfileOnly(t *testing.T) {
	for _, profile := range []ExecutionProfile{ProfileInteractiveAssistant, ProfilePatrolDetection} {
		exec := NewPulseToolExecutor(ExecutorConfig{})
		exec.SetControlLevel(ControlLevelAutonomous)
		exec.ApplyExecutionProfile(profile)
		// Even with a capture wired, the registry policy rejects the
		// fabricated call before the handler.
		exec.SetProposalCapture(NewProposalCapture(ProposalIdentity{}, testProposalCatalog()))

		result, err := exec.ExecuteInvocation(context.Background(), ToolInvocation{
			ID:        "call-x",
			Name:      agentcapabilities.PatrolProposeActionToolName,
			Arguments: proposeArgs(),
		})
		require.NoError(t, err)
		require.NotEmpty(t, result.Content)
		assert.Contains(t, result.Content[0].Text, "Invocation blocked",
			"profile %d must reject patrol_propose_action at the registry boundary", profile)

		// And it never appears in the projected manifest.
		for _, tool := range exec.registry.ListTools(exec.invocationPolicy()) {
			if tool.Name == agentcapabilities.PatrolProposeActionToolName {
				t.Fatalf("profile %d must not offer patrol_propose_action", profile)
			}
		}
	}

	// Under investigation the tool is both offered and executable.
	capture := NewProposalCapture(ProposalIdentity{}, testProposalCatalog())
	exec := newInvestigationExecutor(t, capture)
	offered := false
	for _, tool := range exec.registry.ListTools(exec.invocationPolicy()) {
		if tool.Name == agentcapabilities.PatrolProposeActionToolName {
			offered = true
		}
	}
	assert.True(t, offered, "investigation profile must offer patrol_propose_action")
	result := executePropose(t, exec, "call-a", proposeArgs())
	assert.Contains(t, result.Content[0].Text, "Proposal recorded")
}

func TestProposalExposureProjectorRedactsParams(t *testing.T) {
	args := proposeArgs()
	redacted := agentcapabilities.RedactToolCallArgumentsForExposure(agentcapabilities.PatrolProposeActionToolName, args)
	assert.Equal(t, agentcapabilities.RedactedProposalParamsMarker, redacted["params"])
	assert.Equal(t, "vm:42", redacted["resource_id"])
	// The transient map used for provider continuation and validation is
	// untouched.
	if _, ok := args["params"].(map[string]interface{}); !ok {
		t.Fatal("projector must not mutate the original arguments")
	}
	if !strings.Contains(redacted["reason"].(string), "recover") {
		t.Fatal("non-parameter fields stay exposed")
	}
	// Other tools pass through unchanged.
	other := agentcapabilities.RedactToolCallArgumentsForExposure("pulse_query", args)
	if _, ok := other["params"].(map[string]interface{}); !ok {
		t.Fatal("non-proposal tools must not be redacted")
	}
}

func TestCapturedProposalIsImmuneToCallerMutation(t *testing.T) {
	capture := NewProposalCapture(ProposalIdentity{InvestigationID: "inv-1", EvidenceIDs: []string{"ev-1"}}, testProposalCatalog())
	args := proposeArgs()
	params := args["params"].(map[string]interface{})

	require.NoError(t, capture.Submit("call-a", "vm:42", "restart", "recover", params))

	// Mutating the caller's map after validation must not change the
	// actionable proposal (or its fingerprint identity).
	params["mode"] = "force"

	proposal, _, err := capture.Outcome()
	require.NoError(t, err)
	require.NotNil(t, proposal)
	assert.Equal(t, "graceful", proposal.Params["mode"])

	// Mutating the returned copy must not affect a later outcome read.
	proposal.Params["mode"] = "force"
	proposal.Identity.EvidenceIDs[0] = "tampered"
	again, _, err := capture.Outcome()
	require.NoError(t, err)
	assert.Equal(t, "graceful", again.Params["mode"])
	assert.Equal(t, "ev-1", again.Identity.EvidenceIDs[0])
}

func TestProposalCapabilityMatchingIsExactLikePlanning(t *testing.T) {
	capture := NewProposalCapture(ProposalIdentity{}, testProposalCatalog())
	exec := newInvestigationExecutor(t, capture)

	// The catalog advertises "restart"; a case-mismatched proposal must
	// fail exactly as planning would, so acceptance and planning never
	// drift on name resolution.
	args := proposeArgs()
	args["capability_name"] = "Restart"
	result := executePropose(t, exec, "call-a", args)
	assert.Contains(t, result.Content[0].Text, "does not advertise")

	_, failed, err := capture.Outcome()
	assert.Equal(t, 1, failed)
	if !errors.Is(err, ErrProposalAttemptsFailed) {
		t.Fatalf("outcome error = %v, want ErrProposalAttemptsFailed", err)
	}
}

func TestProposalValidationUsesCanonicalPlannerRules(t *testing.T) {
	capture := NewProposalCapture(ProposalIdentity{}, testProposalCatalog())
	exec := newInvestigationExecutor(t, capture)

	// Empty required value: planner treats whitespace as missing.
	args := proposeArgs()
	args["params"] = map[string]interface{}{"mode": "   "}
	result := executePropose(t, exec, "call-a", args)
	assert.Contains(t, result.Content[0].Text, "invalid for capability")

	// Wrong type for an enum string parameter.
	args = proposeArgs()
	args["params"] = map[string]interface{}{"mode": 42}
	result = executePropose(t, exec, "call-b", args)
	assert.Contains(t, result.Content[0].Text, "invalid for capability")

	_, failed, err := capture.Outcome()
	assert.Equal(t, 2, failed)
	if !errors.Is(err, ErrProposalAttemptsFailed) {
		t.Fatalf("outcome error = %v, want ErrProposalAttemptsFailed", err)
	}
}
