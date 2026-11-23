package pulsecli

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConnectClaudeCodePreservesExistingConfigAndNeverWritesProjectMCPJSON(t *testing.T) {
	home := t.TempDir()
	workDir := t.TempDir()

	// Seed existing ~/.claude.json with unrelated keys and another MCP server.
	claudePath := filepath.Join(home, ".claude.json")
	initial := `{
  "theme": "dark",
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem"]
    }
  }
}
`
	if err := os.WriteFile(claudePath, []byte(initial), 0o600); err != nil {
		t.Fatalf("WriteFile(.claude.json) error = %v", err)
	}

	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(workDir); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	var stdout, stderr bytes.Buffer
	cmd := NewConnectCommand(&ConnectDeps{
		HomeDir: func() (string, error) { return home, nil },
	})
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"claude-code"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("connect claude-code error = %v", err)
	}

	// Never write a project-shared .mcp.json in cwd.
	if _, err := os.Stat(filepath.Join(workDir, ".mcp.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("connect must never create project-shared .mcp.json, stat err = %v", err)
	}

	if !strings.Contains(stdout.String(), claudePath) {
		t.Fatalf("output must print exact config path %q:\n%s", claudePath, stdout.String())
	}

	raw, err := os.ReadFile(claudePath)
	if err != nil {
		t.Fatalf("ReadFile(.claude.json) error = %v", err)
	}
	if strings.Contains(string(raw), "PULSE_API_TOKEN") || strings.Contains(string(raw), `"env"`) {
		t.Fatalf(".claude.json must embed zero plaintext tokens:\n%s", string(raw))
	}

	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("invalid JSON written to .claude.json: %v", err)
	}
	if parsed["theme"] != "dark" {
		t.Fatalf("existing top-level key lost: %+v", parsed)
	}
	servers := parsed["mcpServers"].(map[string]any)
	if _, ok := servers["filesystem"]; !ok {
		t.Fatalf("existing mcpServers.filesystem entry lost: %+v", servers)
	}
	pulseEntry := servers["pulse"].(map[string]any)
	if pulseEntry["command"] != "pulse" {
		t.Fatalf("mcpServers.pulse.command = %v, want pulse", pulseEntry["command"])
	}
	args := pulseEntry["args"].([]any)
	if len(args) != 1 || args[0] != "mcp" {
		t.Fatalf("mcpServers.pulse.args = %v, want [mcp]", args)
	}
}

func TestConnectClaudeDesktopOSPathsAndProfileArg(t *testing.T) {
	cases := []struct {
		goos     string
		wantTail string
	}{
		{
			goos:     "darwin",
			wantTail: filepath.Join("Library", "Application Support", "Claude", "claude_desktop_config.json"),
		},
		{
			goos:     "linux",
			wantTail: filepath.Join(".config", "Claude", "claude_desktop_config.json"),
		},
		{
			goos:     "windows",
			wantTail: filepath.Join("appdata", "Claude", "claude_desktop_config.json"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.goos, func(t *testing.T) {
			home := t.TempDir()
			appData := filepath.Join(home, "appdata")
			var stdout bytes.Buffer
			cmd := NewConnectCommand(&ConnectDeps{
				HomeDir:    func() (string, error) { return home, nil },
				UserConfig: func() (string, error) { return appData, nil },
				GOOS:       tc.goos,
			})
			cmd.SetOut(&stdout)
			cmd.SetErr(&bytes.Buffer{})
			cmd.SetArgs([]string{"claude-desktop", "--profile", "prod"})

			if err := cmd.Execute(); err != nil {
				t.Fatalf("connect claude-desktop (%s) error = %v", tc.goos, err)
			}

			target := filepath.Join(home, tc.wantTail)
			raw, err := os.ReadFile(target)
			if err != nil {
				t.Fatalf("expected file at %s, err = %v", target, err)
			}
			var parsed map[string]any
			if err := json.Unmarshal(raw, &parsed); err != nil {
				t.Fatalf("invalid JSON at %s: %v", target, err)
			}
			servers := parsed["mcpServers"].(map[string]any)
			pulseEntry := servers["pulse"].(map[string]any)
			args := pulseEntry["args"].([]any)
			if len(args) != 3 || args[0] != "mcp" || args[1] != "--profile" || args[2] != "prod" {
				t.Fatalf("args = %v, want [mcp --profile prod]", args)
			}
		})
	}
}

func TestConnectCodexTOMLCreateAndMergePreservesOtherSections(t *testing.T) {
	home := t.TempDir()
	codexPath := filepath.Join(home, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(codexPath), 0o700); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}

	existing := `model = "o3"

[mcp_servers.github]
command = "npx"
args = ["-y", "@modelcontextprotocol/server-github"]

[mcp_servers.pulse]
command = "pulse-mcp"
args = ["--base-url", "http://localhost:7655"]

[mcp_servers.pulse.env]
PULSE_API_TOKEN = "leaked-old-token"

[features]
web_search = true
`
	if err := os.WriteFile(codexPath, []byte(existing), 0o600); err != nil {
		t.Fatalf("WriteFile(config.toml) error = %v", err)
	}

	var stdout bytes.Buffer
	cmd := NewConnectCommand(&ConnectDeps{
		HomeDir: func() (string, error) { return home, nil },
	})
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"codex", "--profile", "homelab"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("connect codex error = %v", err)
	}

	raw, err := os.ReadFile(codexPath)
	if err != nil {
		t.Fatalf("ReadFile(config.toml) error = %v", err)
	}
	got := string(raw)

	for _, mustHave := range []string{
		`model = "o3"`,
		`[mcp_servers.github]`,
		`command = "npx"`,
		`[mcp_servers.pulse]`,
		`command = "pulse"`,
		`args = ["mcp", "--profile", "homelab"]`,
		`[features]`,
		`web_search = true`,
	} {
		if !strings.Contains(got, mustHave) {
			t.Fatalf("merged config.toml missing %q:\n%s", mustHave, got)
		}
	}
	if strings.Contains(got, "leaked-old-token") || strings.Contains(got, "PULSE_API_TOKEN") || strings.Contains(got, "[mcp_servers.pulse.env]") {
		t.Fatalf("merged config.toml must strip legacy plaintext token env:\n%s", got)
	}
}

func TestConnectOpenCodePreservesKeysAndDryRunDoesNotWrite(t *testing.T) {
	home := t.TempDir()
	openCodePath := filepath.Join(home, ".config", "opencode", "opencode.json")

	// Dry run must NOT create the file.
	var dryOut bytes.Buffer
	dryCmd := NewConnectCommand(&ConnectDeps{
		HomeDir: func() (string, error) { return home, nil },
	})
	dryCmd.SetOut(&dryOut)
	dryCmd.SetErr(&bytes.Buffer{})
	dryCmd.SetArgs([]string{"opencode", "--dry-run"})
	if err := dryCmd.Execute(); err != nil {
		t.Fatalf("connect opencode --dry-run error = %v", err)
	}
	if _, err := os.Stat(openCodePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("--dry-run must not write config file, stat err = %v", err)
	}
	if !strings.Contains(dryOut.String(), openCodePath) || !strings.Contains(dryOut.String(), `"pulse"`) {
		t.Fatalf("dry-run output missing path or snippet:\n%s", dryOut.String())
	}

	// Seed existing opencode.json and run real write.
	if err := os.MkdirAll(filepath.Dir(openCodePath), 0o700); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}
	if err := os.WriteFile(openCodePath, []byte(`{"theme":"nord","mcp":{"other":{"type":"local","command":["other"]}}}`), 0o600); err != nil {
		t.Fatalf("WriteFile(opencode.json) error = %v", err)
	}

	var out bytes.Buffer
	cmd := NewConnectCommand(&ConnectDeps{
		HomeDir: func() (string, error) { return home, nil },
	})
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"opencode"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("connect opencode error = %v", err)
	}

	raw, err := os.ReadFile(openCodePath)
	if err != nil {
		t.Fatalf("ReadFile(opencode.json) error = %v", err)
	}
	if strings.Contains(string(raw), "PULSE_API_TOKEN") || strings.Contains(string(raw), `"environment"`) {
		t.Fatalf("opencode.json must embed zero plaintext tokens:\n%s", string(raw))
	}
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("invalid JSON in opencode.json: %v", err)
	}
	if parsed["theme"] != "nord" {
		t.Fatalf("existing opencode.json key lost: %+v", parsed)
	}
	mcpMap := parsed["mcp"].(map[string]any)
	if _, ok := mcpMap["other"]; !ok {
		t.Fatalf("existing mcp.other entry lost: %+v", mcpMap)
	}
	pulseCfg := mcpMap["pulse"].(map[string]any)
	if pulseCfg["type"] != "local" || pulseCfg["enabled"] != true {
		t.Fatalf("unexpected mcp.pulse config: %+v", pulseCfg)
	}
	cmdSlice := pulseCfg["command"].([]any)
	if len(cmdSlice) != 2 || cmdSlice[0] != "pulse" || cmdSlice[1] != "mcp" {
		t.Fatalf("mcp.pulse.command = %v, want [pulse mcp]", cmdSlice)
	}
}

func TestConnectRejectsUnsupportedClient(t *testing.T) {
	cmd := NewConnectCommand(&ConnectDeps{
		HomeDir: func() (string, error) { return t.TempDir(), nil },
	})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{"unknown-client"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "unsupported client") {
		t.Fatalf("expected unsupported client error, got %v", err)
	}
}
