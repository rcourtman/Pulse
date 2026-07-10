package agentcapabilities

import (
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
		InvocationClass{Kind: ToolCallKindWrite, Mutation: MutationNone})
	assertClass(PulseAlertsToolName, map[string]interface{}{"action": "resolve"},
		InvocationClass{Kind: ToolCallKindWrite, Mutation: MutationPulseState})
	assertClass(PulseControlToolName, nil,
		InvocationClass{Kind: ToolCallKindWrite, Mutation: MutationInfrastructure})
	assertClass(PulseFileEditToolName, map[string]interface{}{"action": "write"},
		InvocationClass{Kind: ToolCallKindWrite, Mutation: MutationInfrastructure})
	assertClass(PulseReadToolName, map[string]interface{}{"action": "exec"},
		InvocationClass{Kind: ToolCallKindRead, Mutation: MutationNone})
}
