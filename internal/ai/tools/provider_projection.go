package tools

import "github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"

// AssistantProviderTools returns the provider-facing tool declarations for the
// native in-app Assistant surface. It is the executor-owned projection boundary
// that keeps runtime tool availability, registry governance descriptors, and
// Assistant-native interaction tools composed in one place.
func (e *PulseToolExecutor) AssistantProviderTools(opts agentcapabilities.AssistantProviderToolOptions) []agentcapabilities.ProviderTool {
	if e == nil {
		return agentcapabilities.ProjectPulseAssistantProviderTools(agentcapabilities.CanonicalManifest(), nil, nil, opts)
	}
	return agentcapabilities.ProjectPulseAssistantProviderTools(agentcapabilities.CanonicalManifest(), e.ListTools(), e.ListToolGovernance(), opts)
}

// AssistantSurfaceToolContract returns the native Assistant surface summary
// over the shared Pulse Intelligence Core. The executor owns the runtime
// registry and availability rules; agentcapabilities owns the surface projection
// contract shared with MCP and future external-agent adapters.
func (e *PulseToolExecutor) AssistantSurfaceToolContract(opts agentcapabilities.AssistantProviderToolOptions) agentcapabilities.SurfaceToolContract {
	contract, _ := agentcapabilities.ProjectPulseAssistantSurfaceToolContract(
		agentcapabilities.CanonicalManifest().SurfaceContract,
		e.AssistantProviderTools(opts),
	)
	return contract
}
