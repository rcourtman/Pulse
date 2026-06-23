package agentcapabilities

import "strings"

const (
	DefaultMCPAdapterServerName     = "pulse"
	DefaultMCPAdapterCommand        = "pulse-mcp"
	DefaultMCPAdapterBaseURLFlag    = "--base-url"
	DefaultMCPAdapterDefaultBaseURL = "http://localhost:7655"
	DefaultMCPAdapterTokenEnv       = "PULSE_API_TOKEN"

	MCPAdapterConfigShapeOpenCodeMCP = "opencode_mcp"
	MCPAdapterConfigShapeMCPServers  = "mcp_servers"
	MCPAdapterConfigShapeCustom      = "custom"
)

// DefaultMCPAdapterContract returns the backwards-compatible Pulse MCP setup
// contract used when an older manifest payload lacks the mcpAdapter block.
func DefaultMCPAdapterContract() MCPAdapterContract {
	return MCPAdapterContract{
		ServerName:     DefaultMCPAdapterServerName,
		Command:        DefaultMCPAdapterCommand,
		BaseURLFlag:    DefaultMCPAdapterBaseURLFlag,
		DefaultBaseURL: DefaultMCPAdapterDefaultBaseURL,
		TokenEnv:       DefaultMCPAdapterTokenEnv,
		ConfigFamilies: defaultMCPAdapterConfigFamilies(),
	}
}

func defaultMCPAdapterConfigFamilies() []MCPAdapterConfigFamily {
	return []MCPAdapterConfigFamily{
		{
			ID:          "opencode",
			Label:       "OpenCode",
			Shape:       MCPAdapterConfigShapeOpenCodeMCP,
			Description: "Uses OpenCode's top-level mcp object.",
			FileHints: []string{
				"opencode.json",
				"opencode.jsonc",
				"~/.config/opencode/opencode.json",
			},
			ClientLabels: []string{"OpenCode"},
		},
		{
			ID:          "claude-style",
			Label:       "Claude-style clients",
			Shape:       MCPAdapterConfigShapeMCPServers,
			Description: "Uses the common mcpServers object supported by Claude Desktop and Claude Code.",
			FileHints: []string{
				"~/Library/Application Support/Claude/claude_desktop_config.json",
				".mcp.json",
			},
			ClientLabels: []string{"Claude Desktop", "Claude Code"},
		},
		{
			ID:           "custom-mcp",
			Label:        "custom MCP clients",
			Shape:        MCPAdapterConfigShapeCustom,
			Description:  "Keeps the Pulse MCP command, base URL flag, and token environment variable while adapting the outer client config shape.",
			ClientLabels: []string{"custom MCP clients"},
		},
	}
}

// NormalizeMCPAdapterContract fills compatibility defaults for older manifest
// payloads while preserving manifest-owned values when they are present.
func NormalizeMCPAdapterContract(contract MCPAdapterContract) MCPAdapterContract {
	if strings.TrimSpace(contract.ServerName) == "" {
		contract.ServerName = DefaultMCPAdapterServerName
	}
	if strings.TrimSpace(contract.Command) == "" {
		contract.Command = DefaultMCPAdapterCommand
	}
	if strings.TrimSpace(contract.BaseURLFlag) == "" {
		contract.BaseURLFlag = DefaultMCPAdapterBaseURLFlag
	}
	if strings.TrimSpace(contract.DefaultBaseURL) == "" {
		contract.DefaultBaseURL = DefaultMCPAdapterDefaultBaseURL
	}
	if strings.TrimSpace(contract.TokenEnv) == "" {
		contract.TokenEnv = DefaultMCPAdapterTokenEnv
	}

	contract.ConfigFamilies = NormalizeMCPAdapterConfigFamilies(contract.ConfigFamilies)
	return contract
}

// NormalizeMCPAdapterConfigFamilies detaches config-family descriptors, removes
// empty entries, and falls back to the default supported config families when an
// older manifest does not declare them.
func NormalizeMCPAdapterConfigFamilies(families []MCPAdapterConfigFamily) []MCPAdapterConfigFamily {
	if len(families) == 0 {
		return defaultMCPAdapterConfigFamilies()
	}

	normalized := make([]MCPAdapterConfigFamily, 0, len(families))
	for _, family := range families {
		id := strings.TrimSpace(family.ID)
		label := strings.TrimSpace(family.Label)
		shape := strings.TrimSpace(family.Shape)
		if id == "" && label == "" && shape == "" {
			continue
		}
		if id == "" {
			id = label
		}
		if label == "" {
			label = id
		}
		if shape == "" {
			shape = id
		}
		family.ID = id
		family.Label = label
		family.Shape = shape
		family.Description = strings.TrimSpace(family.Description)
		family.FileHints = trimmedStringSlice(family.FileHints)
		family.ClientLabels = trimmedStringSlice(family.ClientLabels)
		normalized = append(normalized, family)
	}
	if len(normalized) == 0 {
		return defaultMCPAdapterConfigFamilies()
	}
	return normalized
}

func trimmedStringSlice(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
