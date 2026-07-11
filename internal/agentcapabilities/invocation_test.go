package agentcapabilities

import (
	"strings"
	"testing"
)

func TestInvocationDescriptorClassifyFailsClosed(t *testing.T) {
	descriptor, ok := InvocationDescriptorFor(PulseKubernetesToolName)
	if !ok {
		t.Fatal("kubernetes descriptor missing")
	}

	cases := []struct {
		name string
		args map[string]interface{}
	}{
		{name: "missing discriminator", args: nil},
		{name: "malformed discriminator", args: map[string]interface{}{"type": 42}},
		{name: "unknown value", args: map[string]interface{}{"type": "drain_node"}},
		{name: "wrong discriminator key", args: map[string]interface{}{"action": "pods"}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			class := descriptor.Classify(tt.args)
			if class != FailClosedInvocationClass() {
				t.Fatalf("Classify(%#v) = %#v, want fail-closed write/infrastructure", tt.args, class)
			}
		})
	}

	if class := ClassifyRegisteredInvocation("no_such_tool", nil); class != FailClosedInvocationClass() {
		t.Fatalf("unknown tool classified %#v, want fail-closed", class)
	}
}

func TestInvocationDescriptorValidateRequiresExactEnumCoverage(t *testing.T) {
	descriptor := InvocationDescriptor{
		Discriminator: "action",
		Cases: map[string]InvocationClass{
			"list":    {Kind: ToolCallKindRead, Mutation: MutationNone},
			"restart": {Kind: ToolCallKindWrite, Mutation: MutationInfrastructure},
		},
	}
	if err := descriptor.Validate("demo", []string{"list", "restart"}); err != nil {
		t.Fatalf("exact coverage should validate: %v", err)
	}
	if err := descriptor.Validate("demo", []string{"list", "restart", "delete"}); err == nil {
		t.Fatal("missing case must fail validation")
	}
	if err := descriptor.Validate("demo", []string{"list"}); err == nil {
		t.Fatal("extra case must fail validation")
	}
	if err := descriptor.Validate("demo", nil); err == nil {
		t.Fatal("discriminator without schema enum must fail validation")
	}

	static := InvocationDescriptor{Static: &InvocationClass{Kind: ToolCallKindRead, Mutation: MutationNone}}
	if err := static.Validate("demo", nil); err != nil {
		t.Fatalf("static descriptor should validate without enum: %v", err)
	}
	both := static
	both.Discriminator = "action"
	if err := both.Validate("demo", nil); err == nil {
		t.Fatal("static plus discriminator must fail validation")
	}
	neither := InvocationDescriptor{}
	if err := neither.Validate("demo", nil); err == nil {
		t.Fatal("empty descriptor must fail validation")
	}
}

func TestCanonicalDescriptorsPinSafetyCriticalClassifications(t *testing.T) {
	assertClass := func(tool string, args map[string]interface{}, want InvocationClass) {
		t.Helper()
		if got := ClassifyRegisteredInvocation(tool, args); got != want {
			t.Fatalf("%s %v = %#v, want %#v", tool, args, got, want)
		}
	}
	assertClass(PulseKubernetesToolName, map[string]interface{}{"type": "scale"},
		InvocationClass{Kind: ToolCallKindWrite, Mutation: MutationInfrastructure})
	assertClass(PulseDockerToolName, map[string]interface{}{"action": "update"},
		InvocationClass{Kind: ToolCallKindWrite, Mutation: MutationInfrastructure})
	assertClass(PulseDockerToolName, map[string]interface{}{"action": "check_updates"},
		InvocationClass{Kind: ToolCallKindRead, Mutation: MutationNone})
	assertClass(PulseAlertsToolName, map[string]interface{}{"action": "resolve"},
		InvocationClass{Kind: ToolCallKindWrite, Mutation: MutationPulseState})
	assertClass(PulseControlToolName, nil,
		InvocationClass{Kind: ToolCallKindWrite, Mutation: MutationInfrastructure})
	assertClass(PulseFileEditToolName, map[string]interface{}{"action": "write"},
		InvocationClass{Kind: ToolCallKindWrite, Mutation: MutationInfrastructure})
	assertClass(PulseReadToolName, map[string]interface{}{"action": "exec"},
		InvocationClass{Kind: ToolCallKindRead, Mutation: MutationNone})
}

func TestInvocationClassValidationRejectsOpenVocabulary(t *testing.T) {
	missingMutation := InvocationDescriptor{Static: &InvocationClass{Kind: ToolCallKindWrite}}
	if err := missingMutation.Validate("demo", nil); err == nil {
		t.Fatal("static class without a mutation target must fail validation")
	}
	unknownKind := InvocationDescriptor{Static: &InvocationClass{Kind: ToolCallKind(99), Mutation: MutationNone}}
	if err := unknownKind.Validate("demo", nil); err == nil {
		t.Fatal("static class with an unknown kind must fail validation")
	}
	badCase := InvocationDescriptor{
		Discriminator: "action",
		Cases: map[string]InvocationClass{
			"list": {Kind: ToolCallKindRead, Mutation: MutationTarget("estate")},
		},
	}
	if err := badCase.Validate("demo", []string{"list"}); err == nil {
		t.Fatal("case with an unknown mutation target must fail validation")
	}
}

func TestInvocationDescriptorForReturnsIsolatedCopies(t *testing.T) {
	first, ok := InvocationDescriptorFor(PulseDockerToolName)
	if !ok {
		t.Fatal("docker descriptor missing")
	}
	first.Cases["update"] = InvocationClass{Kind: ToolCallKindRead, Mutation: MutationNone}

	second, _ := InvocationDescriptorFor(PulseDockerToolName)
	if got := second.Cases["update"]; got.Mutation != MutationInfrastructure {
		t.Fatalf("mutating a returned descriptor leaked into the canonical table: %#v", got)
	}

	static, _ := InvocationDescriptorFor(PulseControlToolName)
	static.Static.Mutation = MutationNone
	refetched, _ := InvocationDescriptorFor(PulseControlToolName)
	if refetched.Static.Mutation != MutationInfrastructure {
		t.Fatalf("mutating a returned static class leaked into the canonical table: %#v", refetched.Static)
	}
}

func TestInvocationBlockedResultCoversAllMutationTargets(t *testing.T) {
	// The blocked-invocation result is the shared refusal for every
	// profile-denied mutation, not just infrastructure: pulse-state
	// blocks (e.g. Patrol detection's finding-tool allowlist) use it too.
	for _, target := range []MutationTarget{MutationPulseState, MutationInfrastructure} {
		result := NewInvocationBlockedToolResult("pulse_alerts", InvocationClass{Kind: ToolCallKindWrite, Mutation: target})
		text := ToolResultText(result)
		if text == "" {
			t.Fatal("blocked result must carry text")
		}
		for _, want := range []string{"Invocation blocked", "pulse_alerts", string(target), "propose a typed action"} {
			if !strings.Contains(text, want) {
				t.Fatalf("blocked result for %s missing %q: %s", target, want, text)
			}
		}
	}
}

func TestRestrictedExposureVocabulary(t *testing.T) {
	if !ToolHasRestrictedExposure(PatrolProposeActionToolName) {
		t.Fatal("patrol_propose_action must be exposure-restricted")
	}
	for _, name := range []string{PulseQueryToolName, PulseDockerToolName, PulseReadToolName} {
		if ToolHasRestrictedExposure(name) {
			t.Fatalf("%s must not be exposure-restricted", name)
		}
	}
	// Restricted exposure implies raw provider overrides are discarded;
	// the projected form is the only exposable one.
	projected := RedactToolCallArgumentsForExposure(PatrolProposeActionToolName, map[string]interface{}{
		"params": map[string]interface{}{"token": "secret"},
	})
	if projected["params"] != RedactedProposalParamsMarker {
		t.Fatalf("projector must redact params, got %#v", projected["params"])
	}
}

func TestLegacyAssistantAliasesUseClosedInvocationClassification(t *testing.T) {
	tests := map[string]MutationTarget{
		LegacyAssistantFetchURLToolName:       MutationNone,
		LegacyAssistantRunCommandToolName:     MutationInfrastructure,
		LegacyAssistantSetResourceURLToolName: MutationPulseState,
		ResolveFindingCapabilityName:          MutationPulseState,
		DismissFindingCapabilityName:          MutationPulseState,
		"unknown_alias":                       MutationInfrastructure,
	}
	for name, want := range tests {
		if got := ClassifyLegacyAssistantInvocation(name); got.Mutation != want {
			t.Fatalf("%s mutation = %q, want %q", name, got.Mutation, want)
		}
	}
}
