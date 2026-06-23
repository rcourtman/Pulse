package agentcapabilities

import (
	"fmt"
	"sort"
	"strings"
)

const (
	// PulseIntelligenceOverviewStartMarker and
	// PulseIntelligenceOverviewEndMarker delimit the manifest-derived public
	// Pulse Intelligence surface relationship in docs/AI.md.
	PulseIntelligenceOverviewStartMarker = "<!-- pulse-intelligence-overview:start -->"
	PulseIntelligenceOverviewEndMarker   = "<!-- pulse-intelligence-overview:end -->"

	// MCPReadmeSurfaceContractStartMarker and
	// MCPReadmeSurfaceContractEndMarker delimit the manifest-derived Pulse
	// Intelligence surface relationship in cmd/pulse-mcp/README.md.
	MCPReadmeSurfaceContractStartMarker = "<!-- pulse-mcp-surface-contract:start -->"
	MCPReadmeSurfaceContractEndMarker   = "<!-- pulse-mcp-surface-contract:end -->"

	// MCPReadmeScopeListStartMarker and MCPReadmeScopeListEndMarker delimit
	// the manifest-derived scope list in cmd/pulse-mcp/README.md.
	MCPReadmeScopeListStartMarker = "<!-- pulse-mcp-scope-list:start -->"
	MCPReadmeScopeListEndMarker   = "<!-- pulse-mcp-scope-list:end -->"

	// MCPReadmeClientConfigStartMarker and MCPReadmeClientConfigEndMarker
	// delimit the manifest-derived client setup examples in
	// cmd/pulse-mcp/README.md.
	MCPReadmeClientConfigStartMarker = "<!-- pulse-mcp-client-config:start -->"
	MCPReadmeClientConfigEndMarker   = "<!-- pulse-mcp-client-config:end -->"

	// MCPReadmeToolInventoryStartMarker and MCPReadmeToolInventoryEndMarker
	// delimit the manifest-derived request/response tool inventory in
	// cmd/pulse-mcp/README.md.
	MCPReadmeToolInventoryStartMarker = "<!-- pulse-mcp-tools:start -->"
	MCPReadmeToolInventoryEndMarker   = "<!-- pulse-mcp-tools:end -->"

	// MCPReadmePromptInventoryStartMarker and
	// MCPReadmePromptInventoryEndMarker delimit the manifest-derived workflow
	// prompt inventory in cmd/pulse-mcp/README.md.
	MCPReadmePromptInventoryStartMarker = "<!-- pulse-mcp-prompts:start -->"
	MCPReadmePromptInventoryEndMarker   = "<!-- pulse-mcp-prompts:end -->"

	// MCPReadmeErrorInventoryStartMarker and
	// MCPReadmeErrorInventoryEndMarker delimit the manifest-derived stable
	// error-code inventory in cmd/pulse-mcp/README.md.
	MCPReadmeErrorInventoryStartMarker = "<!-- pulse-mcp-errors:start -->"
	MCPReadmeErrorInventoryEndMarker   = "<!-- pulse-mcp-errors:end -->"
)

// RequiredScopeMarkdownList formats a manifest-owned scope set for
// operator-facing setup docs.
func RequiredScopeMarkdownList(scopes []string) string {
	scopes = NormalizeRequiredScopes(scopes)
	if len(scopes) == 0 {
		return "no scopes"
	}
	return markdownInlineCodeList(scopes)
}

// RequiredCapabilityScopeMarkdownList formats the capability-derived scope set
// for compatibility with older manifest payloads and tests.
func RequiredCapabilityScopeMarkdownList(capabilities []Capability) string {
	return RequiredScopeMarkdownList(RequiredCapabilityScopes(capabilities))
}

// ManifestRequiredScopeMarkdownList formats the manifest-owned requiredScopes
// summary for operator-facing setup docs. Older manifests fall back to the
// capability rows.
func ManifestRequiredScopeMarkdownList(manifest Manifest) string {
	if scopes := RequiredScopeMarkdownList(manifest.RequiredScopes); scopes != "no scopes" {
		return scopes
	}
	return RequiredCapabilityScopeMarkdownList(manifest.Capabilities)
}

// MCPClientConfigMarkdown renders manifest-owned Pulse MCP setup examples. The
// surrounding README can explain installation, but command, flag, token-env,
// server-name, and config-family facts come from the discovery contract.
func MCPClientConfigMarkdown(adapter MCPAdapterContract) string {
	adapter = NormalizeMCPAdapterContract(adapter)
	families := adapter.ConfigFamilies

	lines := []string{
		fmt.Sprintf("Most MCP clients need the same manifest-owned runtime facts: server name `%s`, command `%s`, base URL flag `%s`, default URL `%s`, and token environment variable `%s`.", adapter.ServerName, adapter.Command, adapter.BaseURLFlag, adapter.DefaultBaseURL, adapter.TokenEnv),
	}
	if familyList := mcpAdapterConfigFamilyMarkdownList(families); familyList != "" {
		lines = append(lines, fmt.Sprintf("The generated examples below cover the currently declared config families: %s.", familyList))
	}
	lines = append(lines,
		"If your client uses a different outer config format, keep those runtime facts and adapt only the wrapper.",
		"",
	)

	if family := firstMCPAdapterConfigFamilyByShape(families, MCPAdapterConfigShapeOpenCodeMCP); family != nil {
		lines = append(lines, mcpOpenCodeConfigMarkdown(adapter, *family), "")
	}
	if family := firstMCPAdapterConfigFamilyByShape(families, MCPAdapterConfigShapeMCPServers); family != nil {
		lines = append(lines, mcpServersConfigMarkdown(adapter, *family), "")
	}
	if family := firstMCPAdapterConfigFamilyByShape(families, MCPAdapterConfigShapeCustom); family != nil {
		lines = append(lines, mcpCustomClientConfigMarkdown(adapter, *family), "")
	}

	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func mcpAdapterConfigFamilyMarkdownList(families []MCPAdapterConfigFamily) string {
	labels := make([]string, 0, len(families))
	for _, family := range families {
		if label := strings.TrimSpace(family.Label); label != "" {
			labels = append(labels, "`"+label+"`")
		}
	}
	return markdownInlineList(labels)
}

func firstMCPAdapterConfigFamilyByShape(families []MCPAdapterConfigFamily, shape string) *MCPAdapterConfigFamily {
	for i := range families {
		if families[i].Shape == shape {
			return &families[i]
		}
	}
	return nil
}

func mcpOpenCodeConfigMarkdown(adapter MCPAdapterContract, family MCPAdapterConfigFamily) string {
	fileHints := markdownInlineList(markdownCodeValues(family.FileHints))
	if fileHints == "" {
		fileHints = "`opencode.json` or `opencode.jsonc`"
	}
	description := strings.TrimSpace(family.Description)
	if description == "" {
		description = "Uses OpenCode's top-level `mcp` object."
	}
	return strings.Join([]string{
		fmt.Sprintf("#### %s", strings.TrimSpace(family.Label)),
		"",
		description,
		fmt.Sprintf("Add this to %s:", fileHints),
		"",
		"```json",
		fmt.Sprintf(`{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "%s": {
      "type": "local",
      "command": ["%s", "%s", "%s"],
      "enabled": true,
      "environment": {
        "%s": "your-token-here"
      }
    }
  }
}`, adapter.ServerName, adapter.Command, adapter.BaseURLFlag, adapter.DefaultBaseURL, adapter.TokenEnv),
		"```",
		"",
		fmt.Sprintf("Restart OpenCode or reload its config after saving. Pulse tools appear under the `%s` MCP server name.", adapter.ServerName),
	}, "\n")
}

func mcpServersConfigMarkdown(adapter MCPAdapterContract, family MCPAdapterConfigFamily) string {
	fileHints := markdownInlineList(markdownCodeValues(family.FileHints))
	if fileHints == "" {
		fileHints = "`mcpServers` client config"
	}
	description := strings.TrimSpace(family.Description)
	if description == "" {
		description = "Uses the common `mcpServers` object."
	}
	clientLabels := markdownInlineList(markdownCodeValues(family.ClientLabels))
	if clientLabels == "" {
		clientLabels = strings.TrimSpace(family.Label)
	}
	return strings.Join([]string{
		fmt.Sprintf("#### %s", strings.TrimSpace(family.Label)),
		"",
		description,
		fmt.Sprintf("Use this shape for %s in %s:", clientLabels, fileHints),
		"",
		"```json",
		fmt.Sprintf(`{
  "mcpServers": {
    "%s": {
      "command": "%s",
      "args": ["%s", "%s"],
      "env": {
        "%s": "your-token-here"
      }
    }
  }
}`, adapter.ServerName, adapter.Command, adapter.BaseURLFlag, adapter.DefaultBaseURL, adapter.TokenEnv),
		"```",
		"",
		fmt.Sprintf("Restart your client after saving the config. If the client cannot resolve `%s` from `PATH`, use the full binary path.", adapter.Command),
	}, "\n")
}

func mcpCustomClientConfigMarkdown(adapter MCPAdapterContract, family MCPAdapterConfigFamily) string {
	description := strings.TrimSpace(family.Description)
	if description == "" {
		description = "Keep the Pulse MCP runtime facts and adapt the outer config shape to the client."
	}
	return strings.Join([]string{
		fmt.Sprintf("#### %s", strings.TrimSpace(family.Label)),
		"",
		description,
		fmt.Sprintf("Use command `%s`, pass `%s %s`, set `%s` to the API token, and keep the server name `%s` when the client asks for one.", adapter.Command, adapter.BaseURLFlag, adapter.DefaultBaseURL, adapter.TokenEnv, adapter.ServerName),
	}, "\n")
}

func markdownCodeValues(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			out = append(out, "`"+trimmed+"`")
		}
	}
	return out
}

// PulseIntelligenceOverviewMarkdown renders the public docs overview of the
// manifest-owned relationship between Pulse Intelligence Core, Patrol,
// Assistant, and Pulse MCP. The docs can add product detail around this block,
// but the surface labels and descriptions stay manifest-backed.
func PulseIntelligenceOverviewMarkdown(contract SurfaceContract) string {
	contract = CloneSurfaceContract(contract)
	coreLabel := strings.TrimSpace(contract.Core.Label)
	if coreLabel == "" {
		coreLabel = "Pulse Intelligence Core"
	}
	coreDescription := strings.TrimSpace(contract.Core.Description)
	if coreDescription == "" {
		coreDescription = "Canonical context, governed actions, safety gates, approval state, action audit, and verification."
	}

	surfaces := nonEmptyOperatorSurfaces(contract.OperatorSurfaces)
	lines := []string{
		fmt.Sprintf("Pulse Intelligence is built around a shared **%s**: %s", coreLabel, coreDescription),
		"",
	}

	if primaryLabel := strings.TrimSpace(contract.ProactiveEngine.Label); primaryLabel != "" {
		accessPathPhrase := pulseIntelligenceAccessPathPhrase(surfaces)
		lines = append(lines,
			fmt.Sprintf("That core is deliberately surfaced with %s as the primary built-in operator and %s as access paths over the same governed capabilities:", pulseIntelligenceShortSurfaceLabel(primaryLabel), accessPathPhrase),
			"",
			fmt.Sprintf("1. **%s**: %s", primaryLabel, primaryOperatorOverviewDescription(contract.ProactiveEngine, coreLabel)),
		)
		for i, surface := range surfaces {
			lines = append(lines, fmt.Sprintf("%d. **%s**: %s", i+2, strings.TrimSpace(surface.Label), operatorSurfaceOverviewDescription(surface, coreLabel)))
		}
	} else {
		lines = append(lines,
			fmt.Sprintf("That core is deliberately surfaced in %s supported operator-facing %s:", markdownCountWord(len(surfaces)), pluralize("surface", len(surfaces))),
			"",
		)
		if len(surfaces) == 0 {
			lines = append(lines, "- Supported operator-facing surfaces are declared by the agent capabilities manifest.")
		} else {
			for i, surface := range surfaces {
				lines = append(lines, fmt.Sprintf("%d. **%s**: %s", i+1, strings.TrimSpace(surface.Label), operatorSurfaceOverviewDescription(surface, coreLabel)))
			}
		}
	}
	return strings.Join(lines, "\n")
}

func pulseIntelligenceAccessPathPhrase(surfaces []OperatorSurfaceContract) string {
	labels := make([]string, 0, len(surfaces))
	for _, surface := range surfaces {
		label := pulseIntelligenceShortSurfaceLabel(surface.Label)
		if label != "" {
			labels = append(labels, label)
		}
	}
	if len(labels) == 0 {
		return "declared surfaces"
	}
	return strings.Join(labels, " plus ")
}

func pulseIntelligenceShortSurfaceLabel(label string) string {
	label = strings.TrimSpace(label)
	if strings.HasPrefix(label, "Pulse ") {
		return strings.TrimSpace(strings.TrimPrefix(label, "Pulse "))
	}
	return label
}

func primaryOperatorOverviewDescription(component SurfaceContractComponent, coreLabel string) string {
	description := strings.TrimSpace(component.Description)
	if description == "" {
		return fmt.Sprintf("The first-party operations surface running on %s.", coreLabel)
	}
	return description
}

func nonEmptyOperatorSurfaces(surfaces []OperatorSurfaceContract) []OperatorSurfaceContract {
	out := make([]OperatorSurfaceContract, 0, len(surfaces))
	for _, surface := range surfaces {
		if strings.TrimSpace(surface.Label) == "" {
			continue
		}
		out = append(out, surface)
	}
	return out
}

func operatorSurfaceOverviewDescription(surface OperatorSurfaceContract, coreLabel string) string {
	description := strings.TrimSpace(surface.Description)
	if description == "" {
		description = fmt.Sprintf("%s over %s.", operatorSurfaceContractRole(surface), coreLabel)
	}
	return appendSurfaceAffordanceSentence(description, surface)
}

func markdownCountWord(count int) string {
	switch count {
	case 0:
		return "zero"
	case 1:
		return "one"
	case 2:
		return "two"
	case 3:
		return "three"
	case 4:
		return "four"
	case 5:
		return "five"
	default:
		return fmt.Sprintf("%d", count)
	}
}

func pluralize(word string, count int) string {
	if count == 1 {
		return word
	}
	return word + "s"
}

// MCPSurfaceContractMarkdown renders the manifest-owned relationship between
// Pulse Intelligence Core, Patrol, Assistant, and Pulse MCP for MCP onboarding
// docs. It intentionally uses the same surface contract as the runtime
// initialize instructions so external-agent docs cannot drift from the API
// manifest.
func MCPSurfaceContractMarkdown(contract SurfaceContract) string {
	core := contract.Core
	coreLabel := strings.TrimSpace(core.Label)
	if coreLabel == "" {
		coreLabel = "Pulse Intelligence Core"
	}
	coreDescription := strings.TrimSpace(core.Description)
	if coreDescription == "" {
		coreDescription = "The shared context, governed tool, safety gate, approval, audit, and verification substrate."
	}

	lines := []string{
		fmt.Sprintf("- **%s**: %s", coreLabel, coreDescription),
	}

	if proactiveLabel := strings.TrimSpace(contract.ProactiveEngine.Label); proactiveLabel != "" {
		description := strings.TrimSpace(contract.ProactiveEngine.Description)
		if description == "" {
			description = fmt.Sprintf("The primary built-in operator running on %s.", coreLabel)
		}
		lines = append(lines, fmt.Sprintf("- **%s**: %s", proactiveLabel, description))
	}

	surfaces := CloneSurfaceContract(contract).OperatorSurfaces
	for _, surface := range surfaces {
		label := strings.TrimSpace(surface.Label)
		if label == "" {
			continue
		}
		description := strings.TrimSpace(surface.Description)
		description = appendSurfaceAffordanceSentence(description, surface)
		role := operatorSurfaceContractRole(surface)
		switch {
		case description != "" && surfaceDescriptionStartsWithRole(description, role):
			lines = append(lines, fmt.Sprintf("- **%s**: %s", label, description))
		case description != "":
			lines = append(lines, fmt.Sprintf("- **%s** (%s): %s", label, role, description))
		default:
			lines = append(lines, fmt.Sprintf("- **%s**: %s over %s.", label, role, coreLabel))
		}
	}

	if len(lines) == 1 {
		lines = append(lines, "- **Pulse MCP**: The external-agent adapter over the same manifest-backed Pulse Intelligence capabilities.")
	}
	return strings.Join(lines, "\n")
}

func appendSurfaceAffordanceSentence(description string, surface OperatorSurfaceContract) string {
	description = strings.TrimSpace(description)
	affordances := SurfaceAffordanceLabels(NormalizeSurfaceAffordances(surface))
	if len(affordances) == 0 {
		return description
	}
	affordanceSentence := fmt.Sprintf("Affordances: %s.", oxfordJoin(affordances))
	if description == "" {
		return affordanceSentence
	}
	return strings.TrimRight(description, ".") + ". " + affordanceSentence
}

func surfaceDescriptionStartsWithRole(description, role string) bool {
	description = strings.ToLower(strings.TrimSpace(description))
	role = strings.ToLower(strings.TrimSpace(role))
	return role != "" && strings.HasPrefix(description, role)
}

// MCPToolCapabilityInventoryMarkdown renders the Pulse MCP request/response
// tool inventory from the manifest-owned Pulse MCP surface contract. Streaming
// capabilities such as subscribe_events, and raw manifest capabilities omitted
// from the Pulse MCP surface, are intentionally excluded because MCP exposes
// only the shared surface-tool allowlist through tools/list and tools/call.
func MCPToolCapabilityInventoryMarkdown(manifest Manifest) string {
	return mcpToolCapabilityInventoryMarkdown(ManifestSurfaceToolCapabilities(manifest, SurfaceIDPulseMCP))
}

func mcpToolCapabilityInventoryMarkdown(capabilities []Capability) string {
	categories := CloneCapabilityCategories(canonicalManifest.Categories)
	groups := make(map[string][]Capability)
	for _, cap := range capabilities {
		if !IsRequestResponseCapability(cap) {
			continue
		}
		category := strings.TrimSpace(cap.Category)
		if category == "" {
			category = "uncategorized"
		}
		groups[category] = append(groups[category], CloneCapability(cap))
	}
	if len(groups) == 0 {
		return "No request/response MCP tools are currently advertised by the manifest."
	}

	orderedCategories := orderedMCPCapabilityCategories(groups, categories)
	sections := make([]string, 0, len(orderedCategories))
	for _, category := range orderedCategories {
		var b strings.Builder
		fmt.Fprintf(&b, "**%s:**\n\n", mcpCapabilityCategoryHeading(category, categories))
		for _, cap := range groups[category] {
			description := strings.TrimSpace(cap.Description)
			if description == "" {
				description = "Manifest-backed Pulse capability."
			}
			fmt.Fprintf(&b, "- `%s` (%s): %s\n", cap.Name, mcpCapabilityMetadataMarkdown(cap), description)
		}
		sections = append(sections, strings.TrimRight(b.String(), "\n"))
	}
	return strings.Join(sections, "\n\n")
}

// MCPPromptInventoryMarkdown renders the MCP workflow prompt inventory from the
// same manifest-owned surface affordance gate and catalogue projection used by
// prompts/list.
func MCPPromptInventoryMarkdown(manifest Manifest) string {
	if !MCPManifestSurfacePromptProjectionSupported(manifest, SurfaceIDPulseMCP) {
		return "No MCP workflow prompts are currently advertised by the manifest."
	}

	prompts := ProjectMCPWorkflowPrompts(ManifestPulseWorkflowPrompts(manifest))
	if len(prompts) == 0 {
		return "No MCP workflow prompts are currently advertised by the manifest."
	}

	lines := make([]string, 0, len(prompts))
	for _, prompt := range prompts {
		description := strings.TrimSpace(prompt.Description)
		if description == "" {
			description = "Manifest-backed Pulse workflow prompt."
		}
		metadata := make([]string, 0, 2)
		if label := strings.TrimSpace(prompt.Title); label != "" {
			metadata = append(metadata, label)
		}
		arguments := mcpPromptArgumentsMarkdown(prompt.Arguments)
		if arguments != "" {
			metadata = append(metadata, arguments)
		}
		if len(metadata) > 0 {
			lines = append(lines, fmt.Sprintf("- `%s` (%s): %s", prompt.Name, strings.Join(metadata, "; "), description))
			continue
		}
		lines = append(lines, fmt.Sprintf("- `%s`: %s", prompt.Name, description))
	}
	return strings.Join(lines, "\n")
}

// MCPErrorCodeInventoryMarkdown renders the manifest-advertised
// capability-specific stable error codes used by Pulse MCP docs. It follows
// the same manifest-owned Pulse MCP surface contract as tools/list so omitted
// raw manifest capabilities do not appear as documented MCP tools.
func MCPErrorCodeInventoryMarkdown(manifest Manifest) string {
	lines := make([]string, 0)
	for _, cap := range ManifestSurfaceToolCapabilities(manifest, SurfaceIDPulseMCP) {
		name := strings.TrimSpace(cap.Name)
		errorCodes := mcpCapabilityErrorCodes(cap)
		if name == "" || len(errorCodes) == 0 {
			continue
		}
		lines = append(lines, fmt.Sprintf("- `%s`: %s", name, markdownInlineCodeList(errorCodes)))
	}
	if len(lines) == 0 {
		return "No capability-specific stable error codes are currently advertised by the manifest."
	}
	return strings.Join(lines, "\n")
}

func orderedMCPCapabilityCategories(groups map[string][]Capability, categories []CapabilityCategory) []string {
	known := make(map[string]bool, len(categories))
	ordered := make([]string, 0, len(groups))
	for _, category := range categories {
		id := strings.TrimSpace(category.ID)
		if id == "" {
			continue
		}
		known[id] = true
		if len(groups[id]) > 0 {
			ordered = append(ordered, id)
		}
	}

	unknown := make([]string, 0)
	for category := range groups {
		if known[category] {
			continue
		}
		unknown = append(unknown, category)
	}
	sort.Strings(unknown)
	return append(ordered, unknown...)
}

func mcpCapabilityCategoryHeading(category string, categories []CapabilityCategory) string {
	for _, descriptor := range categories {
		if strings.TrimSpace(descriptor.ID) == category {
			if label := strings.TrimSpace(descriptor.Label); label != "" {
				return label
			}
			return category
		}
	}
	if category == "uncategorized" {
		return "Uncategorized"
	}
	return category
}

func mcpCapabilityMetadataMarkdown(cap Capability) string {
	governance := NormalizeCapabilityGovernance(cap)
	metadata := []string{}
	if title := CapabilityTitle(cap); title != "" {
		metadata = append(metadata, title)
	}
	method := strings.TrimSpace(cap.Method)
	path := strings.TrimSpace(cap.Path)
	switch {
	case method != "" && path != "":
		metadata = append(metadata, markdownInlineCode(method+" "+path))
	case path != "":
		metadata = append(metadata, markdownInlineCode(path))
	case method != "":
		metadata = append(metadata, markdownInlineCode(method))
	}
	if scope := strings.TrimSpace(cap.Scope); scope != "" {
		metadata = append(metadata, "scope "+markdownInlineCode(scope))
	}
	metadata = append(metadata, "mode "+markdownInlineCode(string(governance.ActionMode)))
	metadata = append(metadata, "approval "+markdownInlineCode(string(governance.ApprovalPolicy)))
	return strings.Join(metadata, ", ")
}

func mcpPromptArgumentsMarkdown(args []MCPPromptArgument) string {
	if len(args) == 0 {
		return ""
	}
	parts := make([]string, 0, len(args))
	for _, arg := range args {
		name := strings.TrimSpace(arg.Name)
		if name == "" {
			continue
		}
		if arg.Required {
			parts = append(parts, "required argument "+markdownInlineCode(name))
			continue
		}
		parts = append(parts, "optional argument "+markdownInlineCode(name))
	}
	return strings.Join(parts, ", ")
}

func mcpCapabilityErrorCodes(cap Capability) []string {
	seen := make(map[string]bool, len(cap.ErrorCodes))
	errorCodes := make([]string, 0, len(cap.ErrorCodes))
	for _, code := range cap.ErrorCodes {
		code = strings.TrimSpace(code)
		if code == "" || seen[code] {
			continue
		}
		seen[code] = true
		errorCodes = append(errorCodes, code)
	}
	return errorCodes
}

func markdownInlineCodeList(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		quoted = append(quoted, markdownInlineCode(value))
	}
	if len(quoted) == 0 {
		return "none"
	}
	return oxfordJoin(quoted)
}

func markdownInlineList(values []string) string {
	trimmed := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			trimmed = append(trimmed, value)
		}
	}
	return oxfordJoin(trimmed)
}

func markdownInlineCode(value string) string {
	return "`" + strings.ReplaceAll(value, "`", "'") + "`"
}
