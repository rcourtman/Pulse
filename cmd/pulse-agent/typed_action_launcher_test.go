package main

import (
	"os"
	"path/filepath"
	"testing"
)

// systemd starts the collector binary as a typed action launcher inside a
// transient unit. The binary has to take that path before it parses anything
// as an agent, and the launcher it runs is the collector's narrow one.
func TestCollectorBinaryServesTheTypedActionLauncherAndOnlyAsItsFirstArgument(t *testing.T) {
	for _, args := range [][]string{
		{"pulse-agent"},
		{"pulse-agent", "--enable-commands"},
		{"pulse-agent", "status"},
		{"pulse-agent", "--", "__pulse_typed_action_launcher"},
	} {
		if code, launcher := typedActionLauncherExit(args); launcher || code != 0 {
			t.Fatalf("%v was treated as a launcher start (exit %d)", args, code)
		}
	}

	// A stop is in the runner's catalog and not in the collector's. The
	// collector's launcher refuses it and writes no result.
	resultPath := filepath.Join(t.TempDir(), "result.json")
	code, launcher := typedActionLauncherExit([]string{
		"pulse-agent", typedActionLauncherCommand,
		"--catalog", "proxmox", "--result-file", resultPath, "--", "pct", "stop", "112",
	})
	if !launcher || code == 0 {
		t.Fatalf("stop through the collector's launcher: launcher=%v exit=%d, want a refusal", launcher, code)
	}
	if _, err := os.Lstat(resultPath); !os.IsNotExist(err) {
		t.Fatalf("a refused launcher left a result file: %v", err)
	}
}
