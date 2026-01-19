package updates

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupInstallShCurlStub(t *testing.T, content string) string {
	t.Helper()
	sum := sha256.Sum256([]byte(content))
	checksum := hex.EncodeToString(sum[:])

	dir := t.TempDir()
	curl := filepath.Join(dir, "curl")
	script := strings.Join([]string{
		"#!/bin/sh",
		`out=""`,
		`url=""`,
		`while [ "$#" -gt 0 ]; do`,
		`  if [ "$1" = "-o" ]; then`,
		`    out="$2"`,
		`    shift 2`,
		`    continue`,
		`  fi`,
		`  url="$1"`,
		`  shift`,
		`done`,
		`if echo "$url" | grep -q ".sha256$"; then`,
		`  echo "` + checksum + `  install.sh" > "$out"`,
		`else`,
		`  printf '%s' "` + content + `" > "$out"`,
		`fi`,
		``,
	}, "\n")
	if err := os.WriteFile(curl, []byte(script), 0755); err != nil {
		t.Fatalf("write curl stub: %v", err)
	}

	return dir
}

func setupBashStub(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bash := filepath.Join(dir, "bash")
	script := strings.Join([]string{
		"#!/bin/sh",
		"cat >/dev/null",
		"echo \"Backup: /tmp/backup\"",
		"echo \"Installing\"",
		"echo \"Success\"",
		"exit 0",
		"",
	}, "\n")
	if err := os.WriteFile(bash, []byte(script), 0755); err != nil {
		t.Fatalf("write bash stub: %v", err)
	}
	return dir
}

func TestInstallShAdapterExecuteSuccess(t *testing.T) {
	scriptContent := "echo ok"
	curlDir := setupInstallShCurlStub(t, scriptContent)
	bashDir := setupBashStub(t)

	t.Setenv("PATH", strings.Join([]string{curlDir, bashDir, os.Getenv("PATH")}, string(os.PathListSeparator)))

	adapter := &InstallShAdapter{
		installScriptURL: "http://example/install.sh",
		logDir:           t.TempDir(),
	}

	var updates []UpdateProgress
	progress := func(p UpdateProgress) {
		updates = append(updates, p)
	}

	err := adapter.Execute(context.Background(), UpdateRequest{Version: "v1.2.3"}, progress)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if len(updates) == 0 {
		t.Fatal("expected progress updates")
	}
	foundCompleted := false
	for _, progress := range updates {
		if progress.Stage == "completed" && progress.IsComplete {
			foundCompleted = true
			break
		}
	}
	if !foundCompleted {
		t.Fatalf("expected completed progress, got %+v", updates)
	}
}

func TestInstallShAdapterExecuteInvalidVersion(t *testing.T) {
	scriptContent := "echo ok"
	curlDir := setupInstallShCurlStub(t, scriptContent)
	t.Setenv("PATH", curlDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	adapter := &InstallShAdapter{
		installScriptURL: "http://example/install.sh",
		logDir:           t.TempDir(),
	}

	err := adapter.Execute(context.Background(), UpdateRequest{Version: "bad version"}, func(UpdateProgress) {})
	if err == nil {
		t.Fatal("expected error for invalid version")
	}
	if !strings.Contains(err.Error(), "invalid version format") {
		t.Fatalf("unexpected error: %v", err)
	}
}
