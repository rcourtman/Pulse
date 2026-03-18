package pulsecli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestGetMockEnvPathUsesDataDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", dir)

	if got := GetMockEnvPath(&MockDeps{}); got != filepath.Join(dir, ".env") {
		t.Fatalf("GetMockEnvPath() = %q", got)
	}
}

func TestGetMockEnvPathFallsBackToDefaultDir(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", "")

	defaultDir := t.TempDir()
	mock := &MockDeps{
		DefaultEnvDir: func() string { return defaultDir },
		Stat:          os.Stat,
	}
	if err := os.WriteFile(filepath.Join(defaultDir, ".env"), []byte("PULSE_MOCK_MODE=false\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if got := GetMockEnvPath(mock); got != filepath.Join(defaultDir, ".env") {
		t.Fatalf("GetMockEnvPath() = %q", got)
	}
}

func TestGetMockEnvPathFallsBackToDefaultDirWithoutFile(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", "")

	defaultDir := filepath.Join(t.TempDir(), "missing-dir")
	mock := &MockDeps{
		DefaultEnvDir: func() string { return defaultDir },
		Stat:          os.Stat,
	}

	if got := GetMockEnvPath(mock); got != filepath.Join(config.ResolveRuntimeDataDir(""), ".env") {
		t.Fatalf("GetMockEnvPath() = %q", got)
	}
}
