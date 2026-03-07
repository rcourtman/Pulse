package pulsecli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetMockEnvPathUsesDataDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", dir)

	if got := GetMockEnvPath(&MockDeps{}); got != filepath.Join(dir, "mock.env") {
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
	if err := os.WriteFile(filepath.Join(defaultDir, "mock.env"), []byte("PULSE_MOCK_MODE=false\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if got := GetMockEnvPath(mock); got != filepath.Join(defaultDir, "mock.env") {
		t.Fatalf("GetMockEnvPath() = %q", got)
	}
}
