package pulsecli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/bootstrap"
)

func TestBootstrapCLIReportsInvalidKeyWithoutReplacingSavedAccess(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", dir)
	t.Setenv("PULSE_LEGACY_KEY_PATH", filepath.Join(t.TempDir(), "missing-key"))
	const token = "synthetic-saved-setup-token"
	tokenPath, err := bootstrap.Persist(dir, token, time.Unix(1700000000, 0))
	if err != nil {
		t.Fatal(err)
	}
	state, err := os.ReadFile(tokenPath)
	if err != nil {
		t.Fatal(err)
	}
	keyPath := filepath.Join(dir, ".encryption.key")
	originalKey, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	invalidKey := []byte("synthetic-invalid-key-contents-must-not-appear-in-errors")
	if err := os.WriteFile(keyPath, invalidKey, 0600); err != nil {
		t.Fatal(err)
	}
	exitCode := 0
	output := captureOutput(t, func() {
		ShowBootstrapToken(&BootstrapDeps{Exit: func(code int) { exitCode = code }})
	})
	if exitCode != 1 || !strings.Contains(output, "Error reading bootstrap token:") || !strings.Contains(output, "refusing to replace") || !strings.Contains(output, "restore the original key") {
		t.Errorf("bootstrap CLI must identify refused key replacement and safe recovery: exit=%d", exitCode)
	}
	if strings.Contains(output, token) || strings.Contains(output, string(invalidKey)) || strings.Contains(output, "NO BOOTSTRAP TOKEN FOUND") {
		t.Error("corrupt-key output leaked fixture material or incorrectly reported missing setup state")
	}
	if got, err := os.ReadFile(keyPath); err != nil || !bytes.Equal(got, invalidKey) {
		t.Errorf("CLI replaced the invalid key: %v", err)
	}
	if got, err := os.ReadFile(tokenPath); err != nil || !bytes.Equal(got, state) {
		t.Errorf("CLI changed saved setup access: %v", err)
	}

	if err := os.WriteFile(keyPath, originalKey, 0600); err != nil {
		t.Fatal(err)
	}
	output = captureOutput(t, func() {
		ShowBootstrapToken(&BootstrapDeps{Exit: func(code int) { t.Errorf("recovered CLI unexpectedly exited %d", code) }})
	})
	if !strings.Contains(output, token) || !strings.Contains(output, "BOOTSTRAP TOKEN FOR FIRST-TIME SETUP") {
		t.Error("restoring the original key did not recover the original CLI token")
	}
	if got, err := os.ReadFile(tokenPath); err != nil || !bytes.Equal(got, state) {
		t.Errorf("recovered CLI changed saved setup access: %v", err)
	}
}
