package agentcapabilities

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPulseCLIMCPArgs(t *testing.T) {
	cases := []struct {
		profile string
		want    []string
	}{
		{profile: "", want: []string{"mcp"}},
		{profile: "default", want: []string{"mcp"}},
		{profile: "  default  ", want: []string{"mcp"}},
		{profile: "staging", want: []string{"mcp", "--profile", "staging"}},
	}
	for _, tc := range cases {
		got := PulseCLIMCPArgs(tc.profile)
		if len(got) != len(tc.want) {
			t.Fatalf("PulseCLIMCPArgs(%q) = %v, want %v", tc.profile, got, tc.want)
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Fatalf("PulseCLIMCPArgs(%q)[%d] = %q, want %q", tc.profile, i, got[i], tc.want[i])
			}
		}
	}
}

func TestPulseCLIMCPAdapterContractIncludesCodexWithoutBreakingDefaultContract(t *testing.T) {
	legacy := DefaultMCPAdapterContract()
	if legacy.Command != DefaultMCPAdapterCommand {
		t.Fatalf("DefaultMCPAdapterContract().Command = %q, want %q", legacy.Command, DefaultMCPAdapterCommand)
	}
	if len(legacy.ConfigFamilies) != 3 {
		t.Fatalf("DefaultMCPAdapterContract().ConfigFamilies len = %d, want 3", len(legacy.ConfigFamilies))
	}

	cliContract := PulseCLIMCPAdapterContract()
	if cliContract.Command != DefaultPulseCLICommand {
		t.Fatalf("PulseCLIMCPAdapterContract().Command = %q, want %q", cliContract.Command, DefaultPulseCLICommand)
	}
	if len(cliContract.ConfigFamilies) != 4 {
		t.Fatalf("PulseCLIMCPAdapterContract().ConfigFamilies len = %d, want 4", len(cliContract.ConfigFamilies))
	}
	foundCodex := false
	for _, family := range cliContract.ConfigFamilies {
		if family.Shape == MCPAdapterConfigShapeCodexTOML {
			foundCodex = true
			if family.ID != "codex" || !containsString(family.FileHints, "~/.codex/config.toml") {
				t.Fatalf("unexpected Codex config family: %+v", family)
			}
		}
	}
	if !foundCodex {
		t.Fatalf("PulseCLIMCPAdapterContract() missing Codex TOML config family")
	}
}

func TestFormatPulseCLISnippetsEmbedZeroPlaintextTokens(t *testing.T) {
	mcpServers, err := FormatPulseCLIMCPServersSnippet("", "", "")
	if err != nil {
		t.Fatalf("FormatPulseCLIMCPServersSnippet() error = %v", err)
	}
	var parsedServers map[string]any
	if err := json.Unmarshal([]byte(mcpServers), &parsedServers); err != nil {
		t.Fatalf("mcpServers snippet invalid JSON: %v\n%s", err, mcpServers)
	}
	if strings.Contains(mcpServers, "PULSE_API_TOKEN") || strings.Contains(mcpServers, `"env"`) {
		t.Fatalf("mcpServers snippet must not embed token env:\n%s", mcpServers)
	}
	if !strings.Contains(mcpServers, `"command": "pulse"`) || !strings.Contains(mcpServers, `"mcp"`) {
		t.Fatalf("mcpServers snippet missing pulse mcp command:\n%s", mcpServers)
	}

	openCode, err := FormatPulseCLIOpenCodeMCPSnippet("pulse", "pulse", "prod")
	if err != nil {
		t.Fatalf("FormatPulseCLIOpenCodeMCPSnippet() error = %v", err)
	}
	var parsedOpenCode map[string]any
	if err := json.Unmarshal([]byte(openCode), &parsedOpenCode); err != nil {
		t.Fatalf("OpenCode snippet invalid JSON: %v\n%s", err, openCode)
	}
	if strings.Contains(openCode, "PULSE_API_TOKEN") || strings.Contains(openCode, `"environment"`) {
		t.Fatalf("OpenCode snippet must not embed token env:\n%s", openCode)
	}
	if !strings.Contains(openCode, `"--profile"`) || !strings.Contains(openCode, `"prod"`) {
		t.Fatalf("OpenCode snippet missing --profile prod:\n%s", openCode)
	}

	codexDefault := FormatPulseCLICodexTOMLSnippet("", "", "")
	wantCodexDefault := "[mcp_servers.pulse]\ncommand = \"pulse\"\nargs = [\"mcp\"]\n"
	if codexDefault != wantCodexDefault {
		t.Fatalf("FormatPulseCLICodexTOMLSnippet(default) =\n%s\nwant:\n%s", codexDefault, wantCodexDefault)
	}

	codexStaging := FormatPulseCLICodexTOMLSnippet("pulse", "/usr/local/bin/pulse", "staging")
	wantCodexStaging := "[mcp_servers.pulse]\ncommand = \"/usr/local/bin/pulse\"\nargs = [\"mcp\", \"--profile\", \"staging\"]\n"
	if codexStaging != wantCodexStaging {
		t.Fatalf("FormatPulseCLICodexTOMLSnippet(staging) =\n%s\nwant:\n%s", codexStaging, wantCodexStaging)
	}
}
