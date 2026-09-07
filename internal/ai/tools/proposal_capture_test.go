package tools

import (
	"context"
	"errors"
	"fmt"
	"github.com/rcourtman/pulse-go-rewrite/internal/actionplanner"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"reflect"
	"sync"
	"testing"
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
		"resource_id":        "vm:42",
		"causal_resource_id": "vm:42",
		"capability_name":    "restart",
		"params":             map[string]interface{}{"mode": "graceful"},
		"reason":             "recover the stalled web tier",
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

func TestProposeActionIsInvestigationProfileOnly(t *testing.T) {
	for _, profile := range []ExecutionProfile{ProfileInteractiveAssistant, ProfilePatrolDetection} {
		exec := NewPulseToolExecutor(ExecutorConfig{})
		exec.SetControlLevel(ControlLevelAutonomous)
		exec.ApplyExecutionProfile(profile)
		// Even with a capture wired, the registry policy rejects the
		// fabricated call before the handler.
		exec.SetProposalCapture(NewProposalCapture(ProposalIdentity{}, testProposalCatalog()))

		for _, invocation := range []ToolInvocation{
			{ID: "call-x", Name: agentcapabilities.PatrolProposeActionToolName, Arguments: proposeArgs()},
			{ID: "call-y", Name: agentcapabilities.PatrolActionCapabilitiesToolName, Arguments: map[string]interface{}{"resource_id": "vm:42"}},
		} {
			result, err := exec.ExecuteInvocation(context.Background(), invocation)
			require.NoError(t, err)
			require.NotEmpty(t, result.Content)
			assert.Contains(t, result.Content[0].Text, "Invocation blocked",
				"profile %d must reject %s at the registry boundary", profile, invocation.Name)
		}

		// And it never appears in the projected manifest.
		for _, tool := range exec.registry.ListTools(exec.invocationPolicy()) {
			if isPatrolInvestigationOnlyTool(tool.Name) {
				t.Fatalf("profile %d must not offer %s", profile, tool.Name)
			}
		}
	}

	// Under investigation the tool is both offered and executable.
	capture := newPlanningCapture()
	exec := newInvestigationExecutor(t, capture)
	offered := false
	for _, tool := range exec.registry.ListTools(exec.invocationPolicy()) {
		if tool.Name == agentcapabilities.PatrolProposeActionToolName {
			offered = true
		}
	}
	assert.True(t, offered, "investigation profile must offer patrol_propose_action")
	result := executePropose(t, exec, "call-a", proposeArgs())
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "act_existing")
}

// The callback stands in for the separately race-tested canonical lifecycle.
// It accepts one immutable request and refuses conflicting reuse of its ID.
func newPlanningCapture() *ProposalCapture {
	c := NewProposalCapture(ProposalIdentity{ProposalID: "p1", FindingID: "f1", InvestigationID: "i1"}, testProposalCatalog())
	var first *CapturedProposal
	c.SetPlanner(func(ctx context.Context, p CapturedProposal) (unified.ActionAuditRecord, error) {
		if err := validateProposalAgainstCatalog(ctx, c.catalog, p.ResourceID, p.CapabilityName, p.Params); err != nil {
			return unified.ActionAuditRecord{}, err
		}
		if first != nil && (p.Reason != first.Reason || !reflect.DeepEqual(p.Params, first.Params)) {
			return unified.ActionAuditRecord{}, unified.ErrActionIdentityConflict
		}
		if first == nil {
			copy := p
			first = &copy
		}
		return unified.ActionAuditRecord{ID: "act_existing", State: unified.ActionStatePending}, nil
	})
	return c
}
func TestPlanningConflictRetainsAcceptedAction(t *testing.T) {
	c := newPlanningCapture()
	exec := newInvestigationExecutor(t, c)
	require.False(t, executePropose(t, exec, "call-a", proposeArgs()).IsError)
	changed := proposeArgs()
	changed["reason"] = "different intent"
	require.True(t, executePropose(t, exec, "call-b", changed).IsError)
	got, err := c.Outcome()
	require.NoError(t, err)
	require.Equal(t, "act_existing", got.Action.ID)
	require.Equal(t, proposeArgs()["reason"], got.Reason)
	require.False(t, executePropose(t, exec, "call-c", proposeArgs()).IsError)
}
func TestConcurrentPlanningConflictDoesNotErasePersistedAction(t *testing.T) {
	c := newPlanningCapture()
	exec := newInvestigationExecutor(t, c)
	changed := proposeArgs()
	changed["reason"] = "alternative intent"
	var wg sync.WaitGroup
	for i, args := range []map[string]interface{}{proposeArgs(), changed} {
		wg.Add(1)
		go func(i int, args map[string]interface{}) {
			defer wg.Done()
			executePropose(t, exec, []string{"a", "b"}[i], args)
		}(i, args)
	}
	wg.Wait()
	got, err := c.Outcome()
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, "act_existing", got.Action.ID)
}
func TestPlanningRefusalDoesNotDetermineDiagnosis(t *testing.T) {
	c := newPlanningCapture()
	exec := newInvestigationExecutor(t, c)
	args := proposeArgs()
	args["capability_name"] = "unavailable"
	require.True(t, executePropose(t, exec, "a", args).IsError)
	got, err := c.Outcome()
	require.NoError(t, err)
	require.Nil(t, got)
	require.False(t, executePropose(t, exec, "b", proposeArgs()).IsError)
}
func TestPlanningKeepsCompletedEvidenceSnapshotAndCopiesResult(t *testing.T) {
	c := newPlanningCapture()
	exec := newInvestigationExecutor(t, c)
	c.RecordEvidence("completed-read")
	require.False(t, executePropose(t, exec, "a", proposeArgs()).IsError)
	c.RecordEvidence("later-read")
	require.False(t, executePropose(t, exec, "b", proposeArgs()).IsError)
	got, err := c.Outcome()
	require.NoError(t, err)
	require.Equal(t, []string{"completed-read"}, got.Identity.EvidenceIDs)
	got.Identity.EvidenceIDs[0] = "modified"
	got.Action.ID = "modified"
	got.Params["mode"] = "modified"
	again, err := c.Outcome()
	require.NoError(t, err)
	require.Equal(t, "act_existing", again.Action.ID)
	require.Equal(t, "graceful", again.Params["mode"])
}
func TestKnownActionSurvivesPlanningReadFailure(t *testing.T) {
	c := NewProposalCapture(ProposalIdentity{}, nil)
	c.SetPlanner(func(context.Context, CapturedProposal) (unified.ActionAuditRecord, error) {
		return unified.ActionAuditRecord{ID: "act_known"}, errors.New("audit read unavailable")
	})
	result := executePropose(t, newInvestigationExecutor(t, c), "a", proposeArgs())
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "act_known")
	got, err := c.Outcome()
	require.NoError(t, err)
	require.Equal(t, "act_known", got.Action.ID)
}
func TestPlanningRequiresCorePlanner(t *testing.T) {
	c := NewProposalCapture(ProposalIdentity{}, testProposalCatalog())
	result := executePropose(t, newInvestigationExecutor(t, c), "a", proposeArgs())
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "planning is unavailable")
}

// validateProposalAgainstCatalog checks the proposal against the
// resource's advertised capability contract. Error messages never echo
// parameter values: proposal params exist only transiently for provider
// continuation and validation.
func validateProposalAgainstCatalog(ctx context.Context, catalog ProposalCatalog, resourceID, capabilityName string, params map[string]interface{}) error {
	if catalog == nil {
		return errors.New("no capability catalog is wired for proposal validation")
	}
	capabilities, err := catalog(ctx, resourceID)
	if err != nil {
		return fmt.Errorf("capability catalog lookup failed for resource %q", resourceID)
	}
	// Exact-name resolution and full parameter validation are the
	// planner's canonical implementations, so proposal acceptance and
	// planning can never drift on matching, types, enums, patterns,
	// required presence, or malformed capability schemas.
	capability, found := actionplanner.FindCapability(capabilities, capabilityName)
	if !found {
		return fmt.Errorf("resource %q does not advertise capability %q", resourceID, capabilityName)
	}
	// Proposal-specific ratchet on top of planning: investigations must
	// never carry sensitive values (operators supply those at approval
	// time on the canonical surface).
	for _, param := range capability.Params {
		if !param.IsSensitive {
			continue
		}
		if value, ok := params[param.Name]; ok && value != nil {
			return fmt.Errorf("parameter %q is sensitive and must be supplied by an operator on the canonical approval surface, never by an investigation", param.Name)
		}
	}
	if err := actionplanner.ValidateParams(params, capability.Params); err != nil {
		return fmt.Errorf("proposal parameters are invalid for capability %q: %s", capabilityName, err.Error())
	}
	return nil
}
