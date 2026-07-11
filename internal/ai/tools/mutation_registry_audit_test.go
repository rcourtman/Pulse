package tools

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/mutationregistry"
)

// This audit derives candidates from the actual constructed tool registry and
// each registered schema discriminator. A newly registered infrastructure
// mutation therefore fails unless LookupModelInvocation classifies it.
func TestRegisteredModelMutationSchemasResolveToClosedRegistry(t *testing.T) {
	executor := NewPulseToolExecutor(ExecutorConfig{})
	for name, registered := range executor.registry.tools {
		descriptor := registered.Invocation
		if descriptor == nil {
			t.Fatalf("registered tool %q has no invocation descriptor", name)
		}
		if descriptor.Static != nil {
			if descriptor.Static.Mutation == agentcapabilities.MutationInfrastructure {
				enumerated := false
				for _, discriminator := range []string{"type", "action"} {
					values := registered.Definition.InputSchema.Properties[discriminator].Enum
					for _, value := range values {
						enumerated = true
						assertModelCandidateRegistered(t, name, map[string]interface{}{discriminator: value})
					}
					if len(values) > 0 {
						break
					}
				}
				if !enumerated {
					assertModelCandidateRegistered(t, name, nil)
				}
			}
			continue
		}
		property := registered.Definition.InputSchema.Properties[descriptor.Discriminator]
		for _, value := range property.Enum {
			args := map[string]interface{}{descriptor.Discriminator: value}
			class := descriptor.Classify(args)
			if class.Mutation == agentcapabilities.MutationInfrastructure {
				assertModelCandidateRegistered(t, name, args)
			}
		}
	}
}

func TestRetiredMutationAliasesCannotShadowExtensions(t *testing.T) {
	for _, alias := range []string{agentcapabilities.LegacyAssistantRunCommandToolName, "pulse_run_command"} {
		t.Run(alias, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatalf("retired alias %q was accepted as an extension", alias)
				}
			}()
			NewToolRegistry().RegisterExtension(RegisteredTool{
				Definition: Tool{Name: alias},
				Invocation: StaticInvocation(agentcapabilities.ToolCallKindRead, agentcapabilities.MutationNone),
			})
		})
	}
}

func assertModelCandidateRegistered(t *testing.T, name string, args map[string]interface{}) {
	t.Helper()
	entry, ok := mutationregistry.LookupModelInvocation(name, args)
	if !ok {
		t.Fatalf("registered model mutation %s %v has no registry disposition", name, args)
	}
	if entry.Disposition != mutationregistry.DispositionLifecycle && entry.Disposition != mutationregistry.DispositionRetiredDenied {
		t.Fatalf("registered model mutation %s %v has invalid disposition %q", name, args, entry.Disposition)
	}
}
