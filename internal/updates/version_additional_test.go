package updates

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetCurrentVersion_UsesVersionFile(t *testing.T) {
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldwd)
	}()

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	t.Setenv("PATH", "")
	t.Setenv("PULSE_MOCK_MODE", "")
	t.Setenv("PULSE_ALLOW_DOCKER_UPDATES", "")

	versionPath := filepath.Join(tmpDir, "VERSION")
	if err := os.WriteFile(versionPath, []byte("1.2.3"), 0644); err != nil {
		t.Fatalf("write VERSION: %v", err)
	}

	info, err := GetCurrentVersion()
	if err != nil {
		t.Fatalf("GetCurrentVersion error: %v", err)
	}
	if info.Version != "1.2.3" {
		t.Fatalf("Version = %q, want 1.2.3", info.Version)
	}
	if info.Build != "release" {
		t.Fatalf("Build = %q, want release", info.Build)
	}
	if info.IsDevelopment {
		t.Fatalf("IsDevelopment = true, want false")
	}
	if info.Channel != "stable" {
		t.Fatalf("Channel = %q, want stable", info.Channel)
	}
}

func TestGetDeploymentType_Mock(t *testing.T) {
	t.Setenv("PULSE_MOCK_MODE", "true")
	if got := GetDeploymentType(); got != "mock" {
		t.Fatalf("GetDeploymentType = %q, want mock", got)
	}
}

func TestGetDeploymentType_Manual(t *testing.T) {
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldwd)
	}()

	oldArgs := os.Args
	defer func() {
		os.Args = oldArgs
	}()

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	t.Setenv("PULSE_MOCK_MODE", "")
	t.Setenv("PULSE_ALLOW_DOCKER_UPDATES", "")
	t.Setenv("PATH", "")

	os.Args = []string{"pulse"}

	if got := GetDeploymentType(); got != "manual" {
		t.Fatalf("GetDeploymentType = %q, want manual", got)
	}
}
