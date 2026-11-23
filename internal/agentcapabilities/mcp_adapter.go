package agentcapabilities

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

const (
	DefaultMCPAdapterServerName     = "pulse"
	DefaultMCPAdapterCommand        = "pulse-mcp"
	DefaultMCPAdapterBaseURLFlag    = "--base-url"
	DefaultMCPAdapterDefaultBaseURL = "http://localhost:7655"
	DefaultMCPAdapterTokenEnv       = "PULSE_API_TOKEN"

	DefaultPulseCLICommand       = "pulse"
	DefaultPulseCLIMCPSubcommand = "mcp"

	MCPAdapterConfigShapeOpenCodeMCP = "opencode_mcp"
	MCPAdapterConfigShapeMCPServers  = "mcp_servers"
	MCPAdapterConfigShapeCodexTOML   = "codex_toml"
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

// CodexMCPAdapterConfigFamily returns the client configuration descriptor for
// Codex's user-local TOML config (~/.codex/config.toml).
func CodexMCPAdapterConfigFamily() MCPAdapterConfigFamily {
	return MCPAdapterConfigFamily{
		ID:          "codex",
		Label:       "Codex",
		Shape:       MCPAdapterConfigShapeCodexTOML,
		Description: "Uses Codex's [mcp_servers.<name>] TOML table in ~/.codex/config.toml.",
		FileHints: []string{
			"~/.codex/config.toml",
		},
		ClientLabels: []string{"Codex"},
	}
}

// PulseCLIMCPArgs returns the argv list for running the built-in MCP stdio
// server through the unified pulse CLI (`pulse mcp`, plus `--profile <name>`
// when a non-default profile is requested).
func PulseCLIMCPArgs(profile string) []string {
	profile = strings.TrimSpace(profile)
	if profile == "" || profile == "default" {
		return []string{DefaultPulseCLIMCPSubcommand}
	}
	return []string{DefaultPulseCLIMCPSubcommand, "--profile", profile}
}

// PulseCLIMCPAdapterContract returns the unified `pulse` CLI MCP adapter
// contract, including Codex TOML support alongside OpenCode, Claude-style, and
// custom client families. Existing manifest callers continue to use
// DefaultMCPAdapterContract() for backwards-compatible pulse-mcp expectations.
func PulseCLIMCPAdapterContract() MCPAdapterContract {
	base := defaultMCPAdapterConfigFamilies()
	families := make([]MCPAdapterConfigFamily, 0, len(base)+1)
	if len(base) >= 2 {
		families = append(families, base[:2]...)
		families = append(families, CodexMCPAdapterConfigFamily())
		families = append(families, base[2:]...)
	} else {
		families = append(families, base...)
		families = append(families, CodexMCPAdapterConfigFamily())
	}
	return MCPAdapterContract{
		ServerName:     DefaultMCPAdapterServerName,
		Command:        DefaultPulseCLICommand,
		BaseURLFlag:    "--api-url",
		DefaultBaseURL: DefaultMCPAdapterDefaultBaseURL,
		TokenEnv:       DefaultMCPAdapterTokenEnv,
		ConfigFamilies: families,
	}
}

// FormatPulseCLIMCPServersSnippet renders a zero-plaintext-token mcpServers
// JSON snippet for Claude Code / Claude Desktop using `pulse mcp`.
func FormatPulseCLIMCPServersSnippet(serverName, command, profile string) (string, error) {
	serverName, command = normalizePulseCLISnippetInputs(serverName, command)
	payload := map[string]any{
		"mcpServers": map[string]any{
			serverName: map[string]any{
				"command": command,
				"args":    PulseCLIMCPArgs(profile),
			},
		},
	}
	encoded, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode mcpServers snippet: %w", err)
	}
	return string(encoded) + "\n", nil
}

// FormatPulseCLIOpenCodeMCPSnippet renders a zero-plaintext-token OpenCode JSON
// snippet using `pulse mcp`.
func FormatPulseCLIOpenCodeMCPSnippet(serverName, command, profile string) (string, error) {
	serverName, command = normalizePulseCLISnippetInputs(serverName, command)
	cmdSlice := append([]string{command}, PulseCLIMCPArgs(profile)...)
	payload := map[string]any{
		"$schema": "https://opencode.ai/config.json",
		"mcp": map[string]any{
			serverName: map[string]any{
				"type":    "local",
				"command": cmdSlice,
				"enabled": true,
			},
		},
	}
	encoded, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode OpenCode MCP snippet: %w", err)
	}
	return string(encoded) + "\n", nil
}

// FormatPulseCLICodexTOMLSnippet renders a zero-plaintext-token Codex TOML
// snippet for ~/.codex/config.toml using `pulse mcp`.
func FormatPulseCLICodexTOMLSnippet(serverName, command, profile string) string {
	serverName, command = normalizePulseCLISnippetInputs(serverName, command)
	args := PulseCLIMCPArgs(profile)
	quotedArgs := make([]string, len(args))
	for i, arg := range args {
		quotedArgs[i] = strconv.Quote(arg)
	}
	headerKey := formatTOMLKey(serverName)
	return fmt.Sprintf("[mcp_servers.%s]\ncommand = %s\nargs = [%s]\n",
		headerKey,
		strconv.Quote(command),
		strings.Join(quotedArgs, ", "),
	)
}

func normalizePulseCLISnippetInputs(serverName, command string) (string, string) {
	serverName = strings.TrimSpace(serverName)
	if serverName == "" {
		serverName = DefaultMCPAdapterServerName
	}
	command = strings.TrimSpace(command)
	if command == "" {
		command = DefaultPulseCLICommand
	}
	return serverName, command
}

func formatTOMLKey(key string) string {
	for i := 0; i < len(key); i++ {
		ch := key[i]
		isBare := (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '_'
		if !isBare {
			return strconv.Quote(key)
		}
	}
	if key == "" {
		return strconv.Quote(DefaultMCPAdapterServerName)
	}
	return key
}
