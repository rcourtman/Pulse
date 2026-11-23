package pulsecli

import (
	"bytes"
	"context"
	"io"
	"reflect"
	"strings"
	"testing"
)

func TestNormalizeClientArgsPulseMCPShim(t *testing.T) {
	tests := []struct {
		name  string
		argv0 string
		args  []string
		want  []string
	}{
		{
			name:  "regular pulse binary leaves args unchanged",
			argv0: "/usr/local/bin/pulse",
			args:  []string{"fleet", "--json"},
			want:  []string{"fleet", "--json"},
		},
		{
			name:  "pulse-mcp symlink dispatches to mcp",
			argv0: "/usr/local/bin/pulse-mcp",
			args:  []string{"--emit-notifications"},
			want:  []string{"mcp", "--emit-notifications"},
		},
		{
			name:  "windows pulse-mcp.exe dispatches to mcp",
			argv0: "pulse-mcp.exe",
			args:  []string{"--base-url", "https://pulse.example"},
			want:  []string{"mcp", "--base-url", "https://pulse.example"},
		},
		{
			name:  "release-named pulse-mcp binary dispatches to mcp",
			argv0: "pulse-mcp-darwin-arm64",
			args:  nil,
			want:  []string{"mcp"},
		},
		{
			name:  "already starting with mcp is not doubled",
			argv0: "pulse-mcp",
			args:  []string{"mcp", "--emit-notifications"},
			want:  []string{"mcp", "--emit-notifications"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeClientArgs(tt.argv0, tt.args)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("NormalizeClientArgs(%q, %v) = %v, want %v", tt.argv0, tt.args, got, tt.want)
			}
		})
	}
}

func TestNewClientRootCommandBarePrintsHelpAndRegistersAllCommands(t *testing.T) {
	cmd := NewClientRootCommand(CommandSpec{
		Use:             "pulse",
		Version:         "6.8.0-test",
		VersionTemplate: "Pulse CLI {{.Version}}\n",
		VersionPrinter:  func(w io.Writer) { _, _ = io.WriteString(w, "Pulse CLI 6.8.0-test\n") },
	}, ClientDeps{})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{})

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("bare client root command returned error: %v", err)
	}

	help := out.String()
	for _, want := range []string{
		"Usage:",
		"auth",
		"connect",
		"doctor",
		"mcp",
		"fleet",
		"resource",
		"changes",
		"metrics",
		"events",
		"findings",
		"operator-state",
		"actions",
		"call",
		"version",
		"--api-url",
		"--profile",
		"--json",
	} {
		if !strings.Contains(help, want) {
			t.Errorf("bare help output missing %q:\n%s", want, help)
		}
	}
}

func TestNewClientRootCommandVersion(t *testing.T) {
	cmd := NewClientRootCommand(CommandSpec{
		Use:             "pulse",
		Version:         "6.8.0-test",
		VersionTemplate: "Pulse CLI {{.Version}}\n",
		VersionPrinter:  func(w io.Writer) { _, _ = io.WriteString(w, "Pulse CLI 6.8.0-test\n") },
	}, ClientDeps{})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"version"})

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("version subcommand failed: %v", err)
	}
	if !strings.Contains(out.String(), "Pulse CLI 6.8.0-test") {
		t.Fatalf("unexpected version output: %q", out.String())
	}
}
