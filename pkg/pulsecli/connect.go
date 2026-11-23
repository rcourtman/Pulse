package pulsecli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pulsecli/authstore"
	"github.com/spf13/cobra"
)

const (
	ConnectClientClaudeCode    = "claude-code"
	ConnectClientClaudeDesktop = "claude-desktop"
	ConnectClientCodex         = "codex"
	ConnectClientOpenCode      = "opencode"
)

// ConnectDeps allows deterministic testing of `pulse connect` without touching
// the user's real home or client configuration files. All fields are optional.
type ConnectDeps struct {
	Store      *authstore.Store
	HomeDir    func() (string, error)
	UserConfig func() (string, error)
	GOOS       string
	BinaryPath string
}

type connectOptions struct {
	Profile string
	DryRun  bool
	JSON    bool
}

type connectResult struct {
	Client  string   `json:"client"`
	Path    string   `json:"path"`
	Profile string   `json:"profile"`
	Command string   `json:"command"`
	Args    []string `json:"args"`
	DryRun  bool     `json:"dryRun"`
	Written bool     `json:"written"`
	Content string   `json:"content,omitempty"`
}

// NewConnectCommand constructs the `pulse connect <client>` command that writes
// a zero-plaintext-token MCP configuration into the client's user-local config
// file (never into a project-shared .mcp.json in the working directory).
func NewConnectCommand(deps *ConnectDeps) *cobra.Command {
	var opts connectOptions

	cmd := &cobra.Command{
		Use:   "connect <claude-code|claude-desktop|codex|opencode>",
		Short: "Configure an MCP client to use `pulse mcp` with zero plaintext tokens",
		Long: "Write Pulse's MCP server entry to the selected client's user-local configuration file " +
			"(never a project-shared .mcp.json). Credentials stay in your OS keychain or 0600 Pulse " +
			"credentials store. No API tokens are written into client config files.",
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{ConnectClientClaudeCode, ConnectClientClaudeDesktop, ConnectClientCodex, ConnectClientOpenCode},
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Profile = resolveFlagString(cmd, "profile", opts.Profile)
			opts.JSON = resolveFlagBool(cmd, "json", opts.JSON)
			return runConnect(cmd, deps, args[0], opts)
		},
	}

	cmd.Flags().StringVar(&opts.Profile, "profile", "", "Pulse profile name for `pulse mcp` (defaults to \"default\")")
	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "Print target path and resulting config without writing to disk")
	cmd.Flags().BoolVar(&opts.JSON, "json", false, "Output result as JSON")
	return cmd
}

func runConnect(cmd *cobra.Command, deps *ConnectDeps, rawClient string, opts connectOptions) error {
	client, err := normalizeConnectClient(rawClient)
	if err != nil {
		return err
	}
	profileName, err := authstore.NormalizeProfileName(opts.Profile)
	if err != nil {
		return err
	}

	configPath, err := resolveConnectConfigPath(deps, client)
	if err != nil {
		return err
	}

	command := agentcapabilities.DefaultPulseCLICommand
	if deps != nil && strings.TrimSpace(deps.BinaryPath) != "" {
		command = strings.TrimSpace(deps.BinaryPath)
	}
	mcpArgs := agentcapabilities.PulseCLIMCPArgs(profileName)

	existing, exists, existingMode, err := readExistingConfigFile(configPath)
	if err != nil {
		return err
	}

	var updated []byte
	switch client {
	case ConnectClientClaudeCode, ConnectClientClaudeDesktop:
		updated, err = mergeMCPServersJSON(existing, exists, command, mcpArgs)
	case ConnectClientOpenCode:
		updated, err = mergeOpenCodeJSON(existing, exists, command, mcpArgs)
	case ConnectClientCodex:
		updated = mergeCodexTOML(string(existing), command, profileName)
	default:
		return fmt.Errorf("unsupported client %q", client)
	}
	if err != nil {
		return fmt.Errorf("prepare %s config at %s: %w", client, configPath, err)
	}

	result := connectResult{
		Client:  client,
		Path:    configPath,
		Profile: profileName,
		Command: command,
		Args:    mcpArgs,
		DryRun:  opts.DryRun,
		Written: !opts.DryRun,
	}

	if opts.DryRun {
		result.Content = string(updated)
		if opts.JSON {
			return writeJSONIndent(cmd.OutOrStdout(), result)
		}
		out := cmd.OutOrStdout()
		fmt.Fprintf(out, "Dry run: would write %s config to %s\n\n%s", client, configPath, string(updated))
		return nil
	}

	mode := os.FileMode(0o600)
	if exists && existingMode != 0 {
		mode = existingMode.Perm()
	}
	if err := writeConfigFileAtomic(configPath, updated, mode); err != nil {
		return err
	}

	// Warn (non-blocking) if the target profile is not saved yet.
	if deps == nil || deps.Store != nil {
		if store, storeErr := resolveAuthStore(connectDepsStore(deps)); storeErr == nil {
			if _, _, profErr := store.GetProfile(profileName); errors.Is(profErr, authstore.ErrProfileNotFound) {
				if profileName == authstore.DefaultProfileName {
					fmt.Fprintf(cmd.ErrOrStderr(), "Note: no saved Pulse profile yet. Run \"pulse auth login <url>\" before starting %s.\n", client)
				} else {
					fmt.Fprintf(cmd.ErrOrStderr(), "Note: profile %q is not saved yet. Run \"pulse auth login <url> --profile %s\" before starting %s.\n", profileName, profileName, client)
				}
			}
		}
	}

	if opts.JSON {
		return writeJSONIndent(cmd.OutOrStdout(), result)
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Configured %s to use Pulse MCP (zero plaintext tokens).\n", client)
	fmt.Fprintf(out, "  Config path: %s\n", configPath)
	fmt.Fprintf(out, "  Command:     %s %s\n", command, strings.Join(mcpArgs, " "))
	fmt.Fprintf(out, "\nRestart %s to load the Pulse MCP server, then verify with: pulse doctor\n", clientDisplayName(client))
	return nil
}

func normalizeConnectClient(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case ConnectClientClaudeCode:
		return ConnectClientClaudeCode, nil
	case ConnectClientClaudeDesktop:
		return ConnectClientClaudeDesktop, nil
	case ConnectClientCodex:
		return ConnectClientCodex, nil
	case ConnectClientOpenCode:
		return ConnectClientOpenCode, nil
	default:
		return "", fmt.Errorf("unsupported client %q: choose one of %s, %s, %s, %s",
			raw,
			ConnectClientClaudeCode,
			ConnectClientClaudeDesktop,
			ConnectClientCodex,
			ConnectClientOpenCode,
		)
	}
}

func clientDisplayName(client string) string {
	switch client {
	case ConnectClientClaudeCode:
		return "Claude Code"
	case ConnectClientClaudeDesktop:
		return "Claude Desktop"
	case ConnectClientCodex:
		return "Codex"
	case ConnectClientOpenCode:
		return "OpenCode"
	default:
		return client
	}
}

func resolveConnectConfigPath(deps *ConnectDeps, client string) (string, error) {
	homeFn := os.UserHomeDir
	if deps != nil && deps.HomeDir != nil {
		homeFn = deps.HomeDir
	}
	userCfgFn := os.UserConfigDir
	if deps != nil && deps.UserConfig != nil {
		userCfgFn = deps.UserConfig
	}
	goos := runtime.GOOS
	if deps != nil && strings.TrimSpace(deps.GOOS) != "" {
		goos = strings.TrimSpace(deps.GOOS)
	}

	switch client {
	case ConnectClientClaudeCode:
		home, err := homeFn()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		return filepath.Join(home, ".claude.json"), nil

	case ConnectClientClaudeDesktop:
		switch goos {
		case "darwin":
			home, err := homeFn()
			if err != nil {
				return "", fmt.Errorf("resolve home directory: %w", err)
			}
			return filepath.Join(home, "Library", "Application Support", "Claude", "claude_desktop_config.json"), nil
		case "windows":
			cfgDir, err := userCfgFn()
			if err != nil {
				return "", fmt.Errorf("resolve Windows APPDATA directory: %w", err)
			}
			return filepath.Join(cfgDir, "Claude", "claude_desktop_config.json"), nil
		default:
			home, err := homeFn()
			if err != nil {
				return "", fmt.Errorf("resolve home directory: %w", err)
			}
			return filepath.Join(home, ".config", "Claude", "claude_desktop_config.json"), nil
		}

	case ConnectClientCodex:
		home, err := homeFn()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		return filepath.Join(home, ".codex", "config.toml"), nil

	case ConnectClientOpenCode:
		home, err := homeFn()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		return filepath.Join(home, ".config", "opencode", "opencode.json"), nil

	default:
		return "", fmt.Errorf("unsupported client %q", client)
	}
}

func readExistingConfigFile(path string) ([]byte, bool, os.FileMode, error) {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, 0, nil
		}
		return nil, false, 0, fmt.Errorf("stat %s: %w", path, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false, 0, fmt.Errorf("read %s: %w", path, err)
	}
	return data, true, info.Mode(), nil
}

func mergeMCPServersJSON(existing []byte, exists bool, command string, args []string) ([]byte, error) {
	root := map[string]any{}
	if exists && len(strings.TrimSpace(string(existing))) > 0 {
		if err := json.Unmarshal(existing, &root); err != nil {
			return nil, fmt.Errorf("existing JSON config is invalid: %w", err)
		}
		if root == nil {
			root = map[string]any{}
		}
	}

	var mcpServers map[string]any
	if rawServers, ok := root["mcpServers"]; ok && rawServers != nil {
		cast, ok := rawServers.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("existing \"mcpServers\" field is not a JSON object")
		}
		mcpServers = cast
	} else {
		mcpServers = map[string]any{}
	}

	mcpServers[agentcapabilities.DefaultMCPAdapterServerName] = map[string]any{
		"command": command,
		"args":    args,
	}
	root["mcpServers"] = mcpServers

	encoded, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode JSON config: %w", err)
	}
	return append(encoded, '\n'), nil
}

func mergeOpenCodeJSON(existing []byte, exists bool, command string, args []string) ([]byte, error) {
	root := map[string]any{}
	if exists && len(strings.TrimSpace(string(existing))) > 0 {
		if err := json.Unmarshal(existing, &root); err != nil {
			return nil, fmt.Errorf("existing JSON config is invalid: %w", err)
		}
		if root == nil {
			root = map[string]any{}
		}
	}
	if _, hasSchema := root["$schema"]; !hasSchema {
		root["$schema"] = "https://opencode.ai/config.json"
	}

	var mcpMap map[string]any
	if rawMCP, ok := root["mcp"]; ok && rawMCP != nil {
		cast, ok := rawMCP.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("existing \"mcp\" field is not a JSON object")
		}
		mcpMap = cast
	} else {
		mcpMap = map[string]any{}
	}

	cmdList := make([]string, 0, 1+len(args))
	cmdList = append(cmdList, command)
	cmdList = append(cmdList, args...)

	mcpMap[agentcapabilities.DefaultMCPAdapterServerName] = map[string]any{
		"type":    "local",
		"command": cmdList,
		"enabled": true,
	}
	root["mcp"] = mcpMap

	encoded, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode JSON config: %w", err)
	}
	return append(encoded, '\n'), nil
}

func mergeCodexTOML(existing, command, profile string) []byte {
	snippet := strings.TrimRight(
		agentcapabilities.FormatPulseCLICodexTOMLSnippet(agentcapabilities.DefaultMCPAdapterServerName, command, profile),
		"\n",
	)
	if strings.TrimSpace(existing) == "" {
		return []byte(snippet + "\n")
	}

	normalized := strings.ReplaceAll(existing, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	// Remove trailing empty element produced by final newline so we don't duplicate newlines.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	out := make([]string, 0, len(lines)+4)
	inPulseSection := false
	replaced := false

	for _, line := range lines {
		if tableName, isHeader := parseTOMLTableHeader(line); isHeader {
			if isPulseCodexTable(tableName) {
				if !replaced {
					out = append(out, strings.Split(snippet, "\n")...)
					replaced = true
				}
				inPulseSection = true
				continue
			}
			if inPulseSection {
				// Leave one blank separator line before the next table header.
				if len(out) > 0 && strings.TrimSpace(out[len(out)-1]) != "" {
					out = append(out, "")
				}
				inPulseSection = false
			}
		}
		if inPulseSection {
			continue
		}
		out = append(out, line)
	}

	if !replaced {
		for len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
			out = out[:len(out)-1]
		}
		if len(out) > 0 {
			out = append(out, "")
		}
		out = append(out, strings.Split(snippet, "\n")...)
	}

	return []byte(strings.Join(out, "\n") + "\n")
}

func parseTOMLTableHeader(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "[") {
		return "", false
	}
	isArrayTable := strings.HasPrefix(trimmed, "[[")
	var inner string
	if isArrayTable {
		end := strings.Index(trimmed[2:], "]]")
		if end < 0 {
			return "", false
		}
		rest := strings.TrimSpace(trimmed[2+end+2:])
		if rest != "" && !strings.HasPrefix(rest, "#") {
			return "", false
		}
		inner = trimmed[2 : 2+end]
	} else {
		end := strings.Index(trimmed[1:], "]")
		if end < 0 {
			return "", false
		}
		rest := strings.TrimSpace(trimmed[1+end+1:])
		if rest != "" && !strings.HasPrefix(rest, "#") {
			return "", false
		}
		inner = trimmed[1 : 1+end]
	}
	parts := strings.Split(inner, ".")
	clean := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		p = strings.Trim(p, `"'`)
		if p != "" {
			clean = append(clean, p)
		}
	}
	if len(clean) == 0 {
		return "", false
	}
	return strings.Join(clean, "."), true
}

func isPulseCodexTable(tableName string) bool {
	return tableName == "mcp_servers.pulse" || strings.HasPrefix(tableName, "mcp_servers.pulse.")
}

func connectDepsStore(deps *ConnectDeps) *authstore.Store {
	if deps != nil {
		return deps.Store
	}
	return nil
}

func writeConfigFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config directory %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".pulse-connect-*")
	if err != nil {
		return fmt.Errorf("create temp config file in %s: %w", dir, err)
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temp config file %s: %w", tmpPath, err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp config file %s: %w", tmpPath, err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temp config file %s: %w", tmpPath, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp config file %s: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("replace config file %s: %w", path, err)
	}
	cleanup = false
	return nil
}
