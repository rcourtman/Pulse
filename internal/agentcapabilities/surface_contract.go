package agentcapabilities

import "strings"

const (
	SurfaceIDPulseAssistant = "pulse_assistant"
	SurfaceIDPulseMCP       = "pulse_mcp"

	SurfaceToolSourceAssistantRegistry  = "assistant_registry"
	SurfaceToolSourceCapabilityManifest = "capability_manifest"
)

// SurfaceToolContract describes the tool vocabulary projected for one
// operator-facing Pulse Intelligence surface. Assistant tools come from the
// native registry/provider-tool projection, while external-adapter tools come
// from the canonical manifest capability projection; keeping the source
// explicit stops supported surfaces from being treated as competing duplicate
// implementations.
type SurfaceToolContract struct {
	SurfaceID         string                    `json:"surfaceId"`
	SurfaceLabel      string                    `json:"surfaceLabel,omitempty"`
	ToolSource        string                    `json:"toolSource"`
	ToolNames         []string                  `json:"toolNames"`
	RegistryToolNames []string                  `json:"registryToolNames,omitempty"`
	CapabilityNames   []string                  `json:"capabilityNames,omitempty"`
	NativeToolNames   []string                  `json:"nativeToolNames,omitempty"`
	Affordances       SurfaceAffordanceContract `json:"affordances,omitempty"`
}

// ResolvedSurfaceAffordanceContract is the normalized surface identity and
// affordance set that prompts, adapters, docs, and UI projections should use
// when they need one surface's supported Pulse Intelligence affordances.
type ResolvedSurfaceAffordanceContract struct {
	SurfaceID    string
	SurfaceLabel string
	Affordances  SurfaceAffordanceContract
}

// NormalizeCollections detaches slice fields and preserves a stable empty
// toolNames collection for JSON surfaces.
func (c SurfaceToolContract) NormalizeCollections() SurfaceToolContract {
	c.ToolNames = append([]string(nil), c.ToolNames...)
	if c.ToolNames == nil {
		c.ToolNames = []string{}
	}
	c.RegistryToolNames = append([]string(nil), c.RegistryToolNames...)
	c.CapabilityNames = append([]string(nil), c.CapabilityNames...)
	c.NativeToolNames = append([]string(nil), c.NativeToolNames...)
	return c
}

// CloneSurfaceToolContracts returns detached surface tool summaries in their
// original projection order.
func CloneSurfaceToolContracts(contracts []SurfaceToolContract) []SurfaceToolContract {
	cloned := make([]SurfaceToolContract, 0, len(contracts))
	for _, contract := range contracts {
		cloned = append(cloned, contract.NormalizeCollections())
	}
	return cloned
}

// DefaultSurfaceLabelForID returns the compatibility label for known Pulse
// Intelligence surfaces when older manifest payloads omit a surfaceContract
// block.
func DefaultSurfaceLabelForID(surfaceID string) string {
	switch strings.TrimSpace(surfaceID) {
	case SurfaceIDPulseAssistant:
		return "Pulse Assistant"
	case SurfaceIDPulseMCP:
		return "Pulse MCP"
	default:
		return ""
	}
}

// DefaultSurfaceAffordancesForID returns compatibility defaults for known
// Pulse Intelligence surfaces when older manifest payloads do not declare the
// affordance block.
func DefaultSurfaceAffordancesForID(surfaceID string) SurfaceAffordanceContract {
	switch strings.TrimSpace(surfaceID) {
	case SurfaceIDPulseAssistant:
		return SurfaceAffordanceContract{
			Tools:                true,
			InteractiveQuestions: true,
		}
	case SurfaceIDPulseMCP:
		return SurfaceAffordanceContract{
			Tools:              true,
			Resources:          true,
			Prompts:            true,
			CapabilityMetadata: true,
		}
	default:
		return SurfaceAffordanceContract{}
	}
}

// SurfaceAffordancesDeclared reports whether any affordance was explicitly
// enabled. It intentionally treats an all-false struct as missing/legacy for
// the known current surfaces, because older JSON payloads had no affordance
// field at all.
func SurfaceAffordancesDeclared(affordances SurfaceAffordanceContract) bool {
	return affordances.Tools ||
		affordances.Resources ||
		affordances.Prompts ||
		affordances.CapabilityMetadata ||
		affordances.InteractiveQuestions
}

// NormalizeSurfaceAffordances fills compatibility defaults for known surfaces
// while preserving explicit manifest-owned affordances when present.
func NormalizeSurfaceAffordances(surface OperatorSurfaceContract) SurfaceAffordanceContract {
	if SurfaceAffordancesDeclared(surface.Affordances) {
		return surface.Affordances
	}
	return DefaultSurfaceAffordancesForID(surface.ID)
}

// ResolveSurfaceAffordanceContract resolves one operator surface from the
// manifest surface contract and applies compatibility defaults for known first-
// party surfaces. The returned bool reports whether the surface was declared in
// the provided contract; callers can still use the returned compatibility
// defaults when false.
func ResolveSurfaceAffordanceContract(contract SurfaceContract, surfaceID, surfaceLabelFallback string) (ResolvedSurfaceAffordanceContract, bool) {
	surfaceID = strings.TrimSpace(surfaceID)
	if surfaceID == "" {
		return ResolvedSurfaceAffordanceContract{}, false
	}

	lookupLabel := strings.TrimSpace(surfaceLabelFallback)
	if lookupLabel == "" {
		lookupLabel = DefaultSurfaceLabelForID(surfaceID)
	}
	label := lookupLabel
	if label == "" {
		label = surfaceID
	}

	if surface, ok := FindOperatorSurfaceContract(contract, surfaceID, lookupLabel); ok {
		resolvedID := strings.TrimSpace(surface.ID)
		if resolvedID == "" {
			resolvedID = surfaceID
		}
		resolvedLabel := strings.TrimSpace(surface.Label)
		if resolvedLabel == "" {
			resolvedLabel = label
		}
		return ResolvedSurfaceAffordanceContract{
			SurfaceID:    resolvedID,
			SurfaceLabel: resolvedLabel,
			Affordances:  NormalizeSurfaceAffordances(surface),
		}, true
	}

	return ResolvedSurfaceAffordanceContract{
		SurfaceID:    surfaceID,
		SurfaceLabel: label,
		Affordances:  DefaultSurfaceAffordancesForID(surfaceID),
	}, false
}

// ManifestSurfaceAffordances resolves the affordance contract for one manifest
// surface. Known first-party surfaces retain compatibility defaults for older
// manifests that predate the surfaceContract block.
func ManifestSurfaceAffordances(manifest Manifest, surfaceID string) (SurfaceAffordanceContract, bool) {
	resolution, ok := ResolveSurfaceAffordanceContract(manifest.SurfaceContract, surfaceID, "")
	if SurfaceAffordancesDeclared(resolution.Affordances) {
		return resolution.Affordances, ok
	}
	return SurfaceAffordanceContract{}, false
}

// FindOperatorSurfaceContract resolves one manifest-owned operator surface by
// id or label.
func FindOperatorSurfaceContract(contract SurfaceContract, surfaceID, surfaceName string) (OperatorSurfaceContract, bool) {
	surfaceID = strings.TrimSpace(surfaceID)
	surfaceName = strings.TrimSpace(surfaceName)
	for _, surface := range contract.OperatorSurfaces {
		if OperatorSurfaceContractMatches(surface, surfaceID, surfaceName) {
			return surface, true
		}
	}
	return OperatorSurfaceContract{}, false
}

// OperatorSurfaceContractMatches reports whether a surface matches by stable id
// or display label.
func OperatorSurfaceContractMatches(surface OperatorSurfaceContract, surfaceID, surfaceName string) bool {
	if surfaceID != "" && strings.EqualFold(strings.TrimSpace(surface.ID), surfaceID) {
		return true
	}
	if surfaceName != "" && strings.EqualFold(strings.TrimSpace(surface.Label), surfaceName) {
		return true
	}
	return false
}

// SurfaceAffordanceLabels formats enabled affordances with stable,
// operator-facing labels.
func SurfaceAffordanceLabels(affordances SurfaceAffordanceContract) []string {
	labels := []string{}
	if affordances.Tools {
		labels = append(labels, "tools")
	}
	if affordances.Resources {
		labels = append(labels, "resources")
	}
	if affordances.Prompts {
		labels = append(labels, "prompts")
	}
	if affordances.CapabilityMetadata {
		labels = append(labels, "capability metadata")
	}
	if affordances.InteractiveQuestions {
		labels = append(labels, "interactive questions")
	}
	return labels
}

// ProjectPulseAssistantSurfaceToolContract projects the native Assistant
// provider-tool list into the shared surface summary. Registry-backed tools and
// Assistant-native interaction tools are separated deliberately: pulse_question
// is a native in-app affordance, not an MCP capability.
func ProjectPulseAssistantSurfaceToolContract(contract SurfaceContract, providerTools []ProviderTool) (SurfaceToolContract, bool) {
	surface, ok := ResolveSurfaceAffordanceContract(contract, SurfaceIDPulseAssistant, "")

	toolNames := ProviderToolNames(providerTools)
	if !surface.Affordances.Tools {
		toolNames = []string{}
	}
	nativeCatalog := NewProviderToolNameCatalog(AssistantNativeProviderToolNames())
	registryNames := make([]string, 0, len(toolNames))
	nativeNames := make([]string, 0, len(toolNames))
	offeredNames := make([]string, 0, len(toolNames))
	for _, name := range toolNames {
		if nativeCatalog.Has(name) {
			if !surface.Affordances.InteractiveQuestions {
				continue
			}
			nativeNames = append(nativeNames, name)
			offeredNames = append(offeredNames, name)
			continue
		}
		registryNames = append(registryNames, name)
		offeredNames = append(offeredNames, name)
	}

	return SurfaceToolContract{
		SurfaceID:         surface.SurfaceID,
		SurfaceLabel:      surface.SurfaceLabel,
		ToolSource:        SurfaceToolSourceAssistantRegistry,
		ToolNames:         offeredNames,
		RegistryToolNames: registryNames,
		NativeToolNames:   nativeNames,
		Affordances:       surface.Affordances,
	}.NormalizeCollections(), ok
}

// ProjectManifestSurfaceToolContract normalizes a manifest-published
// request/response tool allowlist into the shared surface summary for a
// manifest-declared external adapter. Streaming capabilities remain represented
// by resources, prompts, or notifications, not request/response tool entries.
func ProjectManifestSurfaceToolContract(manifest Manifest, surfaceID string) (SurfaceToolContract, bool) {
	surfaceID = strings.TrimSpace(surfaceID)
	if surfaceID == "" {
		return SurfaceToolContract{}, false
	}
	surface, ok := FindOperatorSurfaceContract(manifest.SurfaceContract, surfaceID, "")
	if !ok || !surface.ExternalAdapter {
		return SurfaceToolContract{}, false
	}
	if contract, ok := FindManifestSurfaceToolContract(manifest, surface.ID); ok {
		return normalizeManifestSurfaceToolContract(manifest, surface, contract), true
	}
	return SurfaceToolContract{}, false
}

func normalizeManifestSurfaceToolContract(manifest Manifest, surface OperatorSurfaceContract, contract SurfaceToolContract) SurfaceToolContract {
	resolvedSurface, _ := ResolveSurfaceAffordanceContract(manifest.SurfaceContract, surface.ID, surface.Label)
	contract = contract.NormalizeCollections()
	if strings.TrimSpace(contract.SurfaceID) == "" {
		contract.SurfaceID = resolvedSurface.SurfaceID
	}
	if strings.TrimSpace(contract.SurfaceLabel) == "" {
		contract.SurfaceLabel = resolvedSurface.SurfaceLabel
	}
	if strings.TrimSpace(contract.ToolSource) == "" {
		contract.ToolSource = SurfaceToolSourceCapabilityManifest
	}
	if SurfaceAffordancesDeclared(contract.Affordances) {
		contract.Affordances = IntersectSurfaceAffordances(resolvedSurface.Affordances, contract.Affordances)
	} else {
		contract.Affordances = resolvedSurface.Affordances
	}
	if len(contract.ToolNames) == 0 && len(contract.CapabilityNames) > 0 {
		contract.ToolNames = append([]string(nil), contract.CapabilityNames...)
	}
	if contract.Affordances.Tools {
		toolNames := requestResponseCapabilityNamesForNames(manifest.Capabilities, contract.ToolNames)
		contract.ToolNames = toolNames
		contract.CapabilityNames = append([]string(nil), toolNames...)
	} else {
		contract.ToolNames = []string{}
		contract.CapabilityNames = []string{}
	}
	contract.RegistryToolNames = nil
	contract.NativeToolNames = nil
	return contract.NormalizeCollections()
}

// IntersectSurfaceAffordances applies a surface-local narrowing contract without
// allowing that local contract to re-enable an affordance disabled by the
// operator surface.
func IntersectSurfaceAffordances(allowed SurfaceAffordanceContract, requested SurfaceAffordanceContract) SurfaceAffordanceContract {
	return SurfaceAffordanceContract{
		Tools:                allowed.Tools && requested.Tools,
		Resources:            allowed.Resources && requested.Resources,
		Prompts:              allowed.Prompts && requested.Prompts,
		CapabilityMetadata:   allowed.CapabilityMetadata && requested.CapabilityMetadata,
		InteractiveQuestions: allowed.InteractiveQuestions && requested.InteractiveQuestions,
	}
}

// ResolveManifestSurfaceToolContract returns the normalized request/response
// surface tool contract for a manifest-backed external-agent surface. Missing
// published surface tool contracts fail closed rather than inferring tools from
// raw manifest capabilities.
func ResolveManifestSurfaceToolContract(manifest Manifest, surfaceID string) (SurfaceToolContract, bool) {
	surfaceID = strings.TrimSpace(surfaceID)
	if surfaceID == "" {
		return SurfaceToolContract{}, false
	}
	return ProjectManifestSurfaceToolContract(manifest, surfaceID)
}

// ProjectManifestSurfaceToolContracts returns manifest-backed external-adapter
// surface tool summaries in manifest operator-surface order. Native Assistant
// tool availability is intentionally omitted because it is runtime-specific.
func ProjectManifestSurfaceToolContracts(manifest Manifest) []SurfaceToolContract {
	contracts := make([]SurfaceToolContract, 0, len(manifest.SurfaceContract.OperatorSurfaces))
	for _, surface := range manifest.SurfaceContract.OperatorSurfaces {
		if !surface.ExternalAdapter || strings.TrimSpace(surface.ID) == "" {
			continue
		}
		if contract, ok := ProjectManifestSurfaceToolContract(manifest, surface.ID); ok {
			contracts = append(contracts, contract)
		}
	}
	return contracts
}

// ProjectPulseIntelligenceSurfaceToolContracts returns the supported
// operator-facing surface tool summaries in manifest order. The function is
// intentionally a projection over the two canonical sources rather than a new
// tool registry.
func ProjectPulseIntelligenceSurfaceToolContracts(manifest Manifest, assistantTools []ProviderTool) []SurfaceToolContract {
	contracts := make([]SurfaceToolContract, 0, len(manifest.SurfaceContract.OperatorSurfaces))
	for _, surface := range manifest.SurfaceContract.OperatorSurfaces {
		switch strings.TrimSpace(surface.ID) {
		case SurfaceIDPulseAssistant:
			contract, _ := ProjectPulseAssistantSurfaceToolContract(manifest.SurfaceContract, assistantTools)
			contracts = append(contracts, contract)
		default:
			if surface.ExternalAdapter && strings.TrimSpace(surface.ID) != "" {
				if contract, ok := ProjectManifestSurfaceToolContract(manifest, surface.ID); ok {
					contracts = append(contracts, contract)
				}
			}
		}
	}
	return contracts
}

func requestResponseCapabilityNamesForNames(capabilities []Capability, names []string) []string {
	result := make([]string, 0, len(names))
	seen := map[string]struct{}{}
	for _, rawName := range names {
		name := strings.TrimSpace(rawName)
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		if _, err := ResolveRequestResponseCapability(capabilities, name); err != nil {
			continue
		}
		result = append(result, name)
	}
	return result
}
