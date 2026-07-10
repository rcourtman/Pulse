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
			assert.True(t, policy.Allows(*descriptor.Static), "offered static tool %q must be executable", tool.Name)
			continue
		}
		property, ok := tool.InputSchema.Properties[descriptor.Discriminator]
		require.True(t, ok)
		require.NotEmpty(t, property.Enum, "offered mixed tool %q must keep at least one enum value", tool.Name)
		for _, value := range property.Enum {
			class := descriptor.Classify(map[string]interface{}{descriptor.Discriminator: value})
			assert.True(t, policy.Allows(class),
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
