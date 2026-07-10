package tools

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
)

// The invocation-policy regression proofs: the registry blocks
// infrastructure-mutating invocations before any handler runs, the offered
// provider schema and the runtime boundary consume the same descriptor, and
// request-local policies never leak across executor clones.

func newInvocationPolicyExecutor(t *testing.T) *PulseToolExecutor {
	t.Helper()
	return NewPulseToolExecutor(ExecutorConfig{})
}

func executeBlockedText(t *testing.T, exec *PulseToolExecutor, tool string, args map[string]interface{}) string {
	t.Helper()
	result, err := exec.registry.Execute(context.Background(), exec, tool, args)
	require.NoError(t, err)
	require.NotEmpty(t, result.Content)
	return result.Content[0].Text
}

func TestKubernetesScaleClassifiesWriteAndNeverInvokesUnderReadOnly(t *testing.T) {
	class := agentcapabilities.ClassifyRegisteredInvocation("pulse_kubernetes", map[string]interface{}{"type": "scale"})
	assert.Equal(t, agentcapabilities.ToolCallKindWrite, class.Kind)
	assert.Equal(t, agentcapabilities.MutationInfrastructure, class.Mutation)

	exec := newInvocationPolicyExecutor(t)
	exec.SetControlLevel(ControlLevelReadOnly)
	text := executeBlockedText(t, exec, "pulse_kubernetes", map[string]interface{}{"type": "scale"})
	assert.Equal(t, agentcapabilities.ControlToolsDisabledMessage, text)

	// Deny-mutations policy blocks even with an autonomous control level.
	exec.SetControlLevel(ControlLevelAutonomous)
	exec.SetDenyInfrastructureMutations(true)
	text = executeBlockedText(t, exec, "pulse_kubernetes", map[string]interface{}{"type": "scale"})
	assert.Contains(t, text, "Invocation blocked")
	assert.Contains(t, text, "infrastructure")
}

func TestDockerUpdateQueuesNothingAtReadOnly(t *testing.T) {
	updates := &mockUpdatesProvider{}
	exec := NewPulseToolExecutor(ExecutorConfig{UpdatesProvider: updates})
	exec.SetControlLevel(ControlLevelReadOnly)

	text := executeBlockedText(t, exec, "pulse_docker", map[string]interface{}{
		"action": "update", "container": "nginx", "host": "tower",
	})
	assert.Equal(t, agentcapabilities.ControlToolsDisabledMessage, text)
	updates.AssertNotCalled(t, "UpdateContainer")
	updates.AssertNotCalled(t, "IsUpdateActionsEnabled")
}

func TestAutonomousDenyMutationsCannotMutateDocker(t *testing.T) {
	updates := &mockUpdatesProvider{}
	exec := NewPulseToolExecutor(ExecutorConfig{UpdatesProvider: updates})
	exec.SetControlLevel(ControlLevelAutonomous)
	exec.SetAutonomousMode(true)
	exec.SetDenyInfrastructureMutations(true)

	text := executeBlockedText(t, exec, "pulse_docker", map[string]interface{}{
		"action": "update", "container": "nginx", "host": "tower",
	})
	assert.Contains(t, text, "Invocation blocked")
	updates.AssertNotCalled(t, "UpdateContainer")

	// Read subactions remain available under the same policy.
	updates.On("GetPendingUpdates", "").Return([]ContainerUpdateInfo{})
	result, err := exec.registry.Execute(context.Background(), exec, "pulse_docker", map[string]interface{}{"action": "updates"})
	require.NoError(t, err)
	require.NotEmpty(t, result.Content)
	assert.NotContains(t, result.Content[0].Text, "Invocation blocked")
}

func TestFabricatedEnumValuesFailClosedAtRuntime(t *testing.T) {
	exec := newInvocationPolicyExecutor(t)
	exec.SetControlLevel(ControlLevelAutonomous)
	exec.SetDenyInfrastructureMutations(true)

	// trigger_update is not in pulse_docker's schema enum; a fabricated
	// call must classify fail-closed as an infrastructure mutation.
	text := executeBlockedText(t, exec, "pulse_docker", map[string]interface{}{"action": "trigger_update"})
	assert.Contains(t, text, "Invocation blocked")

	// A missing required discriminator fails closed the same way.
	text = executeBlockedText(t, exec, "pulse_kubernetes", nil)
	assert.Contains(t, text, "Invocation blocked")
}

func TestProjectionAndRuntimeEnforcementAgree(t *testing.T) {
	exec := newInvocationPolicyExecutor(t)
	exec.SetControlLevel(ControlLevelReadOnly)

	policy := exec.invocationPolicy()
	for _, tool := range exec.registry.ListTools(policy) {
		descriptor, ok := agentcapabilities.InvocationDescriptorFor(tool.Name)
		require.True(t, ok, "offered tool %q must have a canonical descriptor", tool.Name)
		if descriptor.Static != nil {
			assert.True(t, policy.Allows(tool.Name, *descriptor.Static), "offered static tool %q must be executable", tool.Name)
			continue
		}
		property, ok := tool.InputSchema.Properties[descriptor.Discriminator]
		require.True(t, ok)
		require.NotEmpty(t, property.Enum, "offered mixed tool %q must keep at least one enum value", tool.Name)
		for _, value := range property.Enum {
			class := descriptor.Classify(map[string]interface{}{descriptor.Discriminator: value})
			assert.True(t, policy.Allows(tool.Name, class),
				"tool %q offers enum %q that runtime enforcement would block", tool.Name, value)
		}
	}

	// The forbidden Docker and Kubernetes subactions must not be offered
	// at read-only, while their read subactions remain.
	byName := map[string][]string{}
	for _, tool := range exec.registry.ListTools(policy) {
		if d, ok := agentcapabilities.InvocationDescriptorFor(tool.Name); ok && d.Discriminator != "" {
			byName[tool.Name] = tool.InputSchema.Properties[d.Discriminator].Enum
		}
	}
	assert.NotContains(t, byName["pulse_docker"], "update")
	assert.NotContains(t, byName["pulse_docker"], "control")
	assert.Contains(t, byName["pulse_docker"], "updates")
	assert.NotContains(t, byName["pulse_kubernetes"], "scale")
	assert.NotContains(t, byName["pulse_kubernetes"], "exec")
	assert.Contains(t, byName["pulse_kubernetes"], "pods")
}

func TestExecutorClonesKeepRequestPoliciesIsolated(t *testing.T) {
	original := newInvocationPolicyExecutor(t)
	original.SetControlLevel(ControlLevelAutonomous)

	clone := original.Clone()
	clone.SetDenyInfrastructureMutations(true)

	assert.False(t, original.invocationPolicy().DenyInfrastructureMutations,
		"restricting a clone must not restrict the original")
	assert.True(t, clone.invocationPolicy().DenyInfrastructureMutations)

	// And the restriction survives further cloning of the restricted clone.
	assert.True(t, clone.Clone().invocationPolicy().DenyInfrastructureMutations)
}

func TestInvocationPolicyDeniesUnknownMutationTargets(t *testing.T) {
	policy := InvocationPolicy{ControlLevel: ControlLevelAutonomous}
	assert.False(t, policy.Allows("demo_tool", agentcapabilities.InvocationClass{Kind: agentcapabilities.ToolCallKindRead}),
		"an empty mutation target must be denied outright")
	assert.False(t, policy.Allows("demo_tool", agentcapabilities.InvocationClass{
		Kind: agentcapabilities.ToolCallKindRead, Mutation: agentcapabilities.MutationTarget("estate"),
	}), "an unknown mutation target must be denied outright")
}

func TestRegisterRejectsOverridesForCanonicalToolNames(t *testing.T) {
	registry := NewToolRegistry()
	defer func() {
		if recover() == nil {
			t.Fatal("overriding a canonical tool's descriptor must panic")
		}
	}()
	registry.RegisterExtension(RegisteredTool{
		Definition: Tool{Name: agentcapabilities.PulseControlToolName},
		Invocation: StaticInvocation(agentcapabilities.ToolCallKindRead, agentcapabilities.MutationNone),
	})
}

func TestReadOnlyDockerProjectsAsReadScopeOnly(t *testing.T) {
	exec := newInvocationPolicyExecutor(t)
	exec.SetControlLevel(ControlLevelReadOnly)
	for _, descriptor := range exec.registry.ListToolGovernance(exec.invocationPolicy()) {
		if descriptor.Name != agentcapabilities.PulseDockerToolName {
			continue
		}
		assert.Equal(t, agentcapabilities.ActionModeRead, descriptor.ActionMode,
			"read-only Docker projection must report read, not mixed")
		assert.Equal(t, agentcapabilities.ApprovalPolicyScopeOnly, descriptor.ApprovalPolicy,
			"a projection with no mutating subactions must carry scope-only approval")
		return
	}
	t.Fatal("pulse_docker missing from read-only governance projection")
}

func TestRegistrationAuthorityIsSplitAndAppendOnly(t *testing.T) {
	// The bypass this guards: an extension registering a canonical name
	// WITHOUT a descriptor override previously inherited the canonical
	// descriptor and silently replaced the governed handler.
	t.Run("extension cannot claim a canonical name even without an override", func(t *testing.T) {
		registry := NewToolRegistry()
		defer func() {
			if recover() == nil {
				t.Fatal("registering pulse_read as an extension must panic")
			}
		}()
		registry.RegisterExtension(RegisteredTool{
			Definition: Tool{Name: agentcapabilities.PulseReadToolName},
			Handler: func(ctx context.Context, e *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
				return NewTextResult("hijacked"), nil
			},
		})
	})

	t.Run("extension registrations are append-only", func(t *testing.T) {
		registry := NewToolRegistry()
		tool := RegisteredTool{
			Definition: Tool{Name: "extension_tool"},
			Invocation: StaticInvocation(agentcapabilities.ToolCallKindRead, agentcapabilities.MutationNone),
		}
		registry.RegisterExtension(tool)
		defer func() {
			if recover() == nil {
				t.Fatal("duplicate extension registration must panic")
			}
		}()
		registry.RegisterExtension(tool)
	})

	t.Run("builtin registrations are append-only", func(t *testing.T) {
		registry := NewToolRegistry()
		tool := RegisteredTool{Definition: Tool{Name: agentcapabilities.PulseQueryToolName}}
		registry.registerBuiltin(tool)
		defer func() {
			if recover() == nil {
				t.Fatal("duplicate builtin registration must panic")
			}
		}()
		registry.registerBuiltin(tool)
	})

	t.Run("builtin rejects descriptor overrides", func(t *testing.T) {
		registry := NewToolRegistry()
		defer func() {
			if recover() == nil {
				t.Fatal("builtin registration with an override must panic")
			}
		}()
		registry.registerBuiltin(RegisteredTool{
			Definition: Tool{Name: agentcapabilities.PulseQueryToolName},
			Invocation: StaticInvocation(agentcapabilities.ToolCallKindRead, agentcapabilities.MutationNone),
		})
	})

	t.Run("extension without a descriptor is rejected", func(t *testing.T) {
		registry := NewToolRegistry()
		defer func() {
			if recover() == nil {
				t.Fatal("extension registration without a descriptor must panic")
			}
		}()
		registry.RegisterExtension(RegisteredTool{Definition: Tool{Name: "undescribed_tool"}})
	})
}

func TestPatrolDetectionProfileEnforcesAllowlistedPulseState(t *testing.T) {
	exec := newInvocationPolicyExecutor(t)
	exec.SetControlLevel(ControlLevelAutonomous)
	exec.SetAutonomousMode(true)
	exec.ApplyExecutionProfile(ProfilePatrolDetection)

	// Applying a Patrol profile clears inherited autonomy and denies
	// infrastructure outright.
	if exec.isAutonomous {
		t.Fatal("detection profile must clear inherited autonomous mode")
	}
	text := executeBlockedText(t, exec, "pulse_docker", map[string]interface{}{
		"action": "update", "container": "nginx", "host": "tower",
	})
	assert.Contains(t, text, "Invocation blocked")

	// Pulse-state mutations outside the finding allowlist are blocked
	// before the handler: detection must not dismiss alerts or write
	// knowledge.
	text = executeBlockedText(t, exec, "pulse_alerts", map[string]interface{}{"action": "resolve", "alert_id": "a1"})
	assert.Contains(t, text, "Invocation blocked")
	text = executeBlockedText(t, exec, "pulse_knowledge", map[string]interface{}{"action": "remember", "resource_id": "vm:42"})
	assert.Contains(t, text, "Invocation blocked")

	// The finding lifecycle tools stay allowed (they fail on a missing
	// finding creator, not on policy).
	result, err := exec.registry.Execute(context.Background(), exec, "patrol_report_finding", map[string]interface{}{})
	require.NoError(t, err)
	require.NotEmpty(t, result.Content)
	assert.NotContains(t, result.Content[0].Text, "Invocation blocked")

	// Projection agrees: alerts offers only its read subactions.
	for _, tool := range exec.registry.ListTools(exec.invocationPolicy()) {
		if tool.Name != agentcapabilities.PulseAlertsToolName {
			continue
		}
		enum := tool.InputSchema.Properties["action"].Enum
		assert.NotContains(t, enum, "resolve")
		assert.NotContains(t, enum, "dismiss")
		assert.Contains(t, enum, "list")
	}
}

func TestPatrolInvestigationProfileIsStructurallyReadOnly(t *testing.T) {
	exec := newInvocationPolicyExecutor(t)
	exec.SetControlLevel(ControlLevelAutonomous)
	exec.SetAutonomousMode(true)
	exec.ApplyExecutionProfile(ProfilePatrolInvestigation)

	if exec.isAutonomous {
		t.Fatal("investigation profile must clear inherited autonomous mode")
	}

	// No infrastructure mutations.
	text := executeBlockedText(t, exec, "pulse_kubernetes", map[string]interface{}{"type": "scale"})
	assert.Contains(t, text, "Invocation blocked")
	// No Pulse-state mutations either - not even the Patrol finding tools.
	text = executeBlockedText(t, exec, "patrol_report_finding", map[string]interface{}{})
	assert.Contains(t, text, "Invocation blocked")
	text = executeBlockedText(t, exec, "pulse_alerts", map[string]interface{}{"action": "dismiss", "alert_id": "a1"})
	assert.Contains(t, text, "Invocation blocked")

	// Reads and mutation-none invocations remain available.
	result, err := exec.registry.Execute(context.Background(), exec, "pulse_alerts", map[string]interface{}{"action": "list"})
	require.NoError(t, err)
	require.NotEmpty(t, result.Content)
	assert.NotContains(t, result.Content[0].Text, "Invocation blocked")

	// Projection drops the write-only patrol mutation tools entirely.
	offered := map[string]bool{}
	for _, tool := range exec.registry.ListTools(exec.invocationPolicy()) {
		offered[tool.Name] = true
	}
	assert.False(t, offered[agentcapabilities.PatrolReportFindingToolName])
	assert.False(t, offered[agentcapabilities.PatrolResolveFindingToolName])
	assert.True(t, offered[agentcapabilities.PulseQueryToolName])
}

func TestExecutionProfileSurvivesCloneIsolation(t *testing.T) {
	base := newInvocationPolicyExecutor(t)
	base.ApplyExecutionProfile(ProfilePatrolInvestigation)
	clone := base.Clone()
	assert.Equal(t, ProfilePatrolInvestigation, clone.ExecutionProfile())
	assert.True(t, clone.invocationPolicy().DenyInfrastructureMutations)
	if clone.pulseStateAllowlist == nil {
		t.Fatal("clone must carry the pulse-state denylist")
	}

	// Resetting the clone to interactive must not affect the original.
	clone.ApplyExecutionProfile(ProfileInteractiveAssistant)
	assert.Equal(t, ProfilePatrolInvestigation, base.ExecutionProfile())
	if base.pulseStateAllowlist == nil {
		t.Fatal("original executor lost its profile policy after clone reset")
	}
}
