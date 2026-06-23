package agentcapabilities

import (
	"fmt"
	"strings"
)

// ToolGovernancePromptOptions controls how a surface projects a governed tool
// manifest into model-facing instructions.
type ToolGovernancePromptOptions struct {
	// OfferedToolNames is nil when every manifest tool should be described. A
	// non-nil empty slice explicitly means no tools are offered for this turn.
	OfferedToolNames []string

	// AdditionalToolGovernanceLines are pre-rendered surface-owned tool
	// governance lines for non-registry interaction tools. They are appended
	// after the shared manifest projection so native Assistant-only tools can
	// use the same prompt section without pretending to be manifest
	// capabilities.
	AdditionalToolGovernanceLines []string
}

// PulseIntelligenceOperatingInstructionOptions describes which affordances a
// Pulse Intelligence surface actually exposes. Shared operating instructions
// must not tell native Assistant models to use MCP-only resources or prompts,
// and external-agent adapters must not invent in-app affordances.
type PulseIntelligenceOperatingInstructionOptions struct {
	SurfaceID                    string
	SurfaceName                  string
	SupportsTools                bool
	SupportsResources            bool
	SupportsPrompts              bool
	SupportsCapabilityMetadata   bool
	SupportsInteractiveQuestions bool
	SurfaceContract              SurfaceContract
}

// BuildToolGovernancePromptSection projects the shared tool-governance
// descriptors into the model-facing section used by Pulse Intelligence
// surfaces. Surface-specific pseudo-tools can be appended by the caller, but
// the generic mode/approval/summary wording stays here.
func BuildToolGovernancePromptSection(manifest []ToolGovernanceDescriptor, opts ToolGovernancePromptOptions) string {
	orderedOfferedNames, offeredFilter := normalizeOfferedToolNames(opts.OfferedToolNames)
	additionalLines := normalizeAdditionalToolGovernanceLines(opts.AdditionalToolGovernanceLines)
	additionalLineNames := additionalToolGovernanceLineNames(additionalLines)
	if offeredFilter != nil && len(offeredFilter) == 0 && len(additionalLines) == 0 {
		return strings.Join([]string{
			"## AVAILABLE TOOL GOVERNANCE",
			"No Pulse tools are offered for this turn. Answer directly from the user's message and safe conversation context.",
			"Do not claim to inspect infrastructure, run commands, or use Pulse data unless that evidence is already present in the conversation.",
		}, "\n")
	}

	var b strings.Builder
	b.WriteString("## AVAILABLE TOOL GOVERNANCE\n")
	b.WriteString("This manifest is generated from Pulse's governed tool registry. Use only tools that are actually offered by the provider for the current turn.\n")
	emitted := make(map[string]bool, len(manifest))
	for _, tool := range manifest {
		if offeredFilter != nil && !offeredFilter[tool.Name] {
			continue
		}
		if line := ToolGovernancePromptLine(tool); line != "" {
			b.WriteString("- ")
			b.WriteString(line)
			b.WriteByte('\n')
			emitted[tool.Name] = true
		}
	}

	if offeredFilter != nil {
		for _, name := range orderedOfferedNames {
			if emitted[name] || additionalLineNames[name] {
				continue
			}
			b.WriteString(fmt.Sprintf("- %s: mode=read; approval=%s (no approval required); Offered by Pulse for this turn.\n", name, ApprovalPolicyScopeOnly))
			emitted[name] = true
		}
	}
	for _, line := range additionalLines {
		if name := toolGovernanceLineName(line); name != "" && emitted[name] {
			continue
		}
		b.WriteString("- ")
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}

// BuildAssistantToolGovernancePromptSection projects native Assistant tool
// governance from the shared manifest plus the Assistant-only interactive
// surface tools that are actually offered for the turn.
func BuildAssistantToolGovernancePromptSection(manifest []ToolGovernanceDescriptor, offeredToolNames []string) string {
	filteredOfferedNames, includeQuestionTool := AssistantGovernanceOfferedToolNames(offeredToolNames)
	promptOptions := ToolGovernancePromptOptions{OfferedToolNames: filteredOfferedNames}
	if includeQuestionTool {
		promptOptions.AdditionalToolGovernanceLines = []string{
			PulseQuestionToolGovernancePromptLine(),
		}
	}
	return BuildToolGovernancePromptSection(manifest, promptOptions)
}

// BuildPulseIntelligenceOperatingInstructions returns the shared model-facing
// operating posture for a fully manifest-backed Pulse Intelligence surface.
// Surface-specific callers should prefer BuildPulseAssistantOperatingInstructions
// or BuildPulseMCPOperatingInstructions so the advertised affordances match the
// current surface.
func BuildPulseIntelligenceOperatingInstructions() string {
	return BuildPulseIntelligenceOperatingInstructionsForSurface(PulseIntelligenceOperatingInstructionOptions{
		SurfaceName:                "Pulse Intelligence",
		SupportsTools:              true,
		SupportsResources:          true,
		SupportsPrompts:            true,
		SupportsCapabilityMetadata: true,
	})
}

// BuildPulseAssistantOperatingInstructions returns the native in-app Assistant
// operating posture. Assistant exposes provider tools and in-app context, but
// not MCP resources/prompts, so the source-of-truth instruction must stay
// surface-accurate.
func BuildPulseAssistantOperatingInstructions() string {
	manifest := CanonicalManifest()
	surface, _ := ResolveSurfaceAffordanceContract(manifest.SurfaceContract, SurfaceIDPulseAssistant, "")
	affordances := surface.Affordances
	return BuildPulseIntelligenceOperatingInstructionsForSurface(PulseIntelligenceOperatingInstructionOptions{
		SurfaceName:                  surface.SurfaceLabel,
		SurfaceID:                    surface.SurfaceID,
		SupportsTools:                affordances.Tools,
		SupportsResources:            affordances.Resources,
		SupportsPrompts:              affordances.Prompts,
		SupportsCapabilityMetadata:   affordances.CapabilityMetadata,
		SupportsInteractiveQuestions: affordances.InteractiveQuestions,
		SurfaceContract:              manifest.SurfaceContract,
	})
}

// PulseMCPOperatingInstructionOptions describes the MCP affordances advertised
// during initialize for the current manifest.
type PulseMCPOperatingInstructionOptions struct {
	SurfaceID                  string
	SupportsTools              bool
	SupportsResources          bool
	SupportsPrompts            bool
	SupportsCapabilityMetadata bool
	SurfaceContract            SurfaceContract
}

// BuildPulseMCPOperatingInstructions returns the external-agent MCP operating
// posture with only the initialize-advertised affordances named.
func BuildPulseMCPOperatingInstructions(opts PulseMCPOperatingInstructionOptions) string {
	surfaceID := strings.TrimSpace(opts.SurfaceID)
	if surfaceID == "" {
		surfaceID = SurfaceIDPulseMCP
	}
	surface, _ := ResolveSurfaceAffordanceContract(opts.SurfaceContract, surfaceID, "")
	affordances := surface.Affordances
	return BuildPulseIntelligenceOperatingInstructionsForSurface(PulseIntelligenceOperatingInstructionOptions{
		SurfaceName:                surface.SurfaceLabel,
		SurfaceID:                  surface.SurfaceID,
		SupportsTools:              affordances.Tools && opts.SupportsTools,
		SupportsResources:          affordances.Resources && opts.SupportsResources,
		SupportsPrompts:            affordances.Prompts && opts.SupportsPrompts,
		SupportsCapabilityMetadata: affordances.CapabilityMetadata && opts.SupportsCapabilityMetadata,
		SurfaceContract:            opts.SurfaceContract,
	})
}

// BuildPulseIntelligenceOperatingInstructionsForSurface returns the shared
// model-facing operating posture for Pulse Intelligence surfaces. Native
// Assistant can add richer in-app context around it, and MCP can advertise it
// during initialize, but the governed-action baseline stays one shared
// projection.
func BuildPulseIntelligenceOperatingInstructionsForSurface(opts PulseIntelligenceOperatingInstructionOptions) string {
	surfaceName := strings.TrimSpace(opts.SurfaceName)
	if surfaceName == "" {
		surfaceName = "this surface"
	}
	sourceOfTruth := pulseIntelligenceAffordanceList(opts)
	lines := []string{
		"## PULSE INTELLIGENCE OPERATING MODEL",
		"- Treat Pulse Intelligence as a governed infrastructure-operations surface, not an unrestricted shell.",
		fmt.Sprintf("- This surface is %s. Use %s as the source of truth for what this surface can do.", surfaceName, sourceOfTruth),
	}
	lines = append(lines, pulseIntelligenceSurfaceContractLines(opts)...)
	lines = append(lines, []string{
		"- Prefer read-only context first: inspect fleet or resource context before recommending deeper investigation or a state-changing action.",
		"- Run write-capable tools only when the operator explicitly asks for a state change, and follow the plan, approval, and execute flow exposed by the manifest.",
		"- Treat approval-required, policy-blocked, and unavailable-tool results as safety boundaries. Explain the boundary and choose a safer read or clarification path instead of bypassing it.",
	}...)
	return strings.Join(lines, "\n")
}

func pulseIntelligenceSurfaceContractLines(opts PulseIntelligenceOperatingInstructionOptions) []string {
	contract := opts.SurfaceContract
	coreLabel := strings.TrimSpace(contract.Core.Label)
	if coreLabel == "" {
		return nil
	}

	currentSurface, ok := FindOperatorSurfaceContract(contract, opts.SurfaceID, opts.SurfaceName)
	if !ok {
		return nil
	}
	currentLabel := strings.TrimSpace(currentSurface.Label)
	if currentLabel == "" {
		return nil
	}

	currentRole := operatorSurfaceContractRole(currentSurface)
	otherSurfacePhrases := make([]string, 0, len(contract.OperatorSurfaces))
	for _, surface := range contract.OperatorSurfaces {
		if OperatorSurfaceContractMatches(surface, currentSurface.ID, currentSurface.Label) {
			continue
		}
		label := strings.TrimSpace(surface.Label)
		if label == "" {
			continue
		}
		otherSurfacePhrases = append(otherSurfacePhrases, fmt.Sprintf("%s is %s", label, operatorSurfaceContractRole(surface)))
	}

	surfaceLine := fmt.Sprintf("- Surface contract: %s is %s over %s.", currentLabel, currentRole, coreLabel)
	if len(otherSurfacePhrases) > 0 {
		surfaceLine = strings.TrimSuffix(surfaceLine, ".") + "; " + oxfordJoin(otherSurfacePhrases) + " on the same core."
	}

	lines := []string{surfaceLine}
	if affordances := SurfaceAffordanceLabels(surfaceAffordancesFromOptions(opts)); len(affordances) > 0 {
		lines = append(lines, fmt.Sprintf("- Surface affordances: %s exposes %s.", currentLabel, oxfordJoin(affordances)))
	}
	if proactiveLabel := strings.TrimSpace(contract.ProactiveEngine.Label); proactiveLabel != "" {
		lines = append(lines, fmt.Sprintf("- %s is the primary built-in operator on %s; treat its findings, investigations, governed actions, and run context as governed Pulse evidence.", proactiveLabel, coreLabel))
	}
	return lines
}

func surfaceAffordancesFromOptions(opts PulseIntelligenceOperatingInstructionOptions) SurfaceAffordanceContract {
	return SurfaceAffordanceContract{
		Tools:                opts.SupportsTools,
		Resources:            opts.SupportsResources,
		Prompts:              opts.SupportsPrompts,
		CapabilityMetadata:   opts.SupportsCapabilityMetadata,
		InteractiveQuestions: opts.SupportsInteractiveQuestions,
	}
}

func operatorSurfaceContractRole(surface OperatorSurfaceContract) string {
	switch {
	case surface.Native && surface.ExternalAdapter:
		return "the native Pulse surface and external-agent adapter"
	case surface.Native:
		return "the native Pulse surface"
	case surface.ExternalAdapter:
		return "the external-agent adapter"
	default:
		return "an operator surface"
	}
}

func pulseIntelligenceAffordanceList(opts PulseIntelligenceOperatingInstructionOptions) string {
	items := []string{}
	if opts.SupportsTools {
		items = append(items, "offered tools")
	}
	if opts.SupportsResources {
		items = append(items, "resources")
	}
	if opts.SupportsPrompts {
		items = append(items, "prompts")
	}
	if opts.SupportsCapabilityMetadata {
		items = append(items, "capability metadata")
	}
	if opts.SupportsInteractiveQuestions {
		items = append(items, "interactive questions")
	}
	if len(items) == 0 {
		return "the explicit surface contract"
	}
	return oxfordJoin(items)
}

func oxfordJoin(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	case 2:
		return items[0] + " and " + items[1]
	default:
		return strings.Join(items[:len(items)-1], ", ") + ", and " + items[len(items)-1]
	}
}

// ToolGovernancePromptLine returns the canonical one-line model-facing
// governance description for a single tool.
func ToolGovernancePromptLine(tool ToolGovernanceDescriptor) string {
	name := strings.TrimSpace(tool.Name)
	if name == "" {
		return ""
	}
	description := ToolGovernancePromptDescription(tool)
	if description == "" {
		return ""
	}
	return fmt.Sprintf("%s: %s", name, description)
}

// ToolGovernancePromptDescription returns the canonical model-facing
// mode/approval/summary fragment for one governed tool. Prompt sections, native
// provider tool declarations, and future agent surfaces use this helper instead
// of each carrying slightly different governance prose.
func ToolGovernancePromptDescription(tool ToolGovernanceDescriptor) string {
	mode := tool.ActionMode
	if mode == "" {
		mode = ActionModeRead
	}
	policy := strings.TrimSpace(string(tool.ApprovalPolicy))
	if policy == "" {
		policy = string(ApprovalPolicyScopeOnly)
	}
	approvalSummary := strings.TrimSpace(tool.ApprovalSummary)
	approval := policy
	if approvalSummary != "" {
		approval = fmt.Sprintf("%s (%s)", policy, approvalSummary)
	}
	summary := strings.TrimSpace(tool.Summary)
	if summary == "" {
		summary = firstToolGovernancePromptLine(tool.Description)
	}
	if summary != "" {
		return fmt.Sprintf("mode=%s; approval=%s; %s", mode, approval, summary)
	}
	return fmt.Sprintf("mode=%s; approval=%s", mode, approval)
}

// PulseQuestionToolName is the native Assistant-only structured clarification
// tool. It is not a manifest capability or MCP tool; it is governed prompt
// metadata for interactive in-app turns that can wait for user input.
const PulseQuestionToolName = "pulse_question"

// PulseQuestionToolGovernancePromptLine returns the shared governance line for
// the Assistant's interactive clarification tool.
func PulseQuestionToolGovernancePromptLine() string {
	return PulseQuestionToolName + ": mode=interactive; approval=user answer required; asks the user for missing information using a structured prompt in interactive chat only."
}

// AssistantGovernanceOfferedToolNames splits Assistant-native interaction
// tools out of the manifest-tool filter. This keeps pulse_question governed as
// an interactive surface tool without pretending it is a manifest capability.
func AssistantGovernanceOfferedToolNames(orderedOfferedNames []string) ([]string, bool) {
	if orderedOfferedNames == nil {
		return nil, true
	}
	filtered := make([]string, 0, len(orderedOfferedNames))
	includeQuestionTool := false
	seen := map[string]bool{}
	for _, name := range orderedOfferedNames {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		if trimmed == PulseQuestionToolName {
			includeQuestionTool = true
			continue
		}
		filtered = append(filtered, trimmed)
	}
	return filtered, includeQuestionTool
}

func normalizeOfferedToolNames(names []string) ([]string, map[string]bool) {
	if names == nil {
		return nil, nil
	}
	filter := make(map[string]bool, len(names))
	ordered := make([]string, 0, len(names))
	for _, name := range names {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" || filter[trimmed] {
			continue
		}
		filter[trimmed] = true
		ordered = append(ordered, trimmed)
	}
	return ordered, filter
}

func normalizeAdditionalToolGovernanceLines(lines []string) []string {
	if len(lines) == 0 {
		return nil
	}
	normalized := make([]string, 0, len(lines))
	seen := map[string]bool{}
	for _, line := range lines {
		trimmed := strings.Join(strings.Fields(strings.TrimSpace(line)), " ")
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		normalized = append(normalized, trimmed)
	}
	return normalized
}

func additionalToolGovernanceLineNames(lines []string) map[string]bool {
	names := make(map[string]bool, len(lines))
	for _, line := range lines {
		if name := toolGovernanceLineName(line); name != "" {
			names[name] = true
		}
	}
	return names
}

func toolGovernanceLineName(line string) string {
	name, _, ok := strings.Cut(line, ":")
	if !ok {
		return ""
	}
	return strings.TrimSpace(name)
}

func firstToolGovernancePromptLine(description string) string {
	description = strings.TrimSpace(description)
	if description == "" {
		return ""
	}
	if idx := strings.Index(description, "\n"); idx >= 0 {
		description = strings.TrimSpace(description[:idx])
	}
	return strings.Join(strings.Fields(description), " ")
}
