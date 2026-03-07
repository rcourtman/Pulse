package pulsecli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShowBootstrapTokenUsesStateExitOnMissingToken(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", dir)

	exitCode := 0
	exitFn := func(code int) { exitCode = code }
	runtime := &Runtime{Bootstrap: BootstrapRuntime{Exit: exitFn}}

	output := captureOutput(t, func() {
		ShowBootstrapToken(runtime)
	})

	if exitCode != 1 {
		t.Fatalf("exit code = %d, want 1", exitCode)
	}
	if !strings.Contains(output, "NO BOOTSTRAP TOKEN FOUND") {
		t.Fatalf("output = %q", output)
	}
}

func TestShowBootstrapTokenPrintsToken(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", dir)
	tokenPath := filepath.Join(dir, ".bootstrap_token")
	if err := os.WriteFile(tokenPath, []byte("test-token"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	output := captureOutput(t, func() {
		ShowBootstrapToken(&Runtime{})
	})

	if !strings.Contains(output, "test-token") {
		t.Fatalf("output = %q", output)
	}
}

func captureOutput(t *testing.T, fn func()) string {
	t.Helper()

	oldStdout := os.Stdout
	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	os.Stderr = w
	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	}()

	fn()

	_ = w.Close()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}
	return buf.String()
}
