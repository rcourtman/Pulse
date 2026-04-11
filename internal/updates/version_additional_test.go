package updates

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/mockruntime"
)

func setMockRuntime(t *testing.T, enabled bool) {
	t.Helper()

	original := mockruntime.IsEnabled()
	mockruntime.SetEnabled(enabled)
	t.Cleanup(func() {
		mockruntime.SetEnabled(original)
	})
}

func TestGetCurrentVersion_UsesBuildVersion(t *testing.T) {
	setMockRuntime(t, false)

	oldBuildVersion := BuildVersion
	BuildVersion = "6.0.0-rc.1"
	defer func() {
		BuildVersion = oldBuildVersion
	}()

	t.Setenv("PATH", "")
	t.Setenv("PULSE_MOCK_MODE", "")
	t.Setenv("PULSE_ALLOW_DOCKER_UPDATES", "")

	info, err := GetCurrentVersion()
	if err != nil {
		t.Fatalf("GetCurrentVersion error: %v", err)
	}
	if info.Version != "6.0.0-rc.1" {
		t.Fatalf("Version = %q, want 6.0.0-rc.1", info.Version)
	}
	if info.Build != "release" {
		t.Fatalf("Build = %q, want release", info.Build)
	}
	if info.IsDevelopment {
		t.Fatalf("IsDevelopment = true, want false")
	}
	if info.Channel != "rc" {
		t.Fatalf("Channel = %q, want rc", info.Channel)
	}
}

func TestGetCurrentVersion_UsesVersionFile(t *testing.T) {
	setMockRuntime(t, false)

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

func TestGetCurrentVersion_UsesVersionFileAsDevelopmentBase(t *testing.T) {
	setMockRuntime(t, false)

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldwd)
	}()

	oldBuildVersion := BuildVersion
	BuildVersion = ""
	defer func() {
		BuildVersion = oldBuildVersion
	}()

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	gitPath := filepath.Join(tmpDir, "git")
	gitScript := "#!/bin/sh\nprintf '%s\\n' 'v6.0.0-rc.1-45-gABCDEF'\n"
	if err := os.WriteFile(gitPath, []byte(gitScript), 0755); err != nil {
		t.Fatalf("write fake git: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "VERSION"), []byte("6.0.0-dev"), 0644); err != nil {
		t.Fatalf("write VERSION: %v", err)
	}

	t.Setenv("PATH", tmpDir)
	t.Setenv("PULSE_MOCK_MODE", "")
	t.Setenv("PULSE_ALLOW_DOCKER_UPDATES", "")

	info, err := GetCurrentVersion()
	if err != nil {
		t.Fatalf("GetCurrentVersion error: %v", err)
	}
	if info.Version != "6.0.0-dev+git.45.gabcdef" {
		t.Fatalf("Version = %q, want 6.0.0-dev+git.45.gabcdef", info.Version)
	}
	if info.Build != "development" {
		t.Fatalf("Build = %q, want development", info.Build)
	}
	if !info.IsDevelopment {
		t.Fatalf("IsDevelopment = false, want true")
	}
	if info.Channel != "stable" {
		t.Fatalf("Channel = %q, want stable", info.Channel)
	}
}

func TestGetDeploymentType_Mock(t *testing.T) {
	setMockRuntime(t, true)
	t.Setenv("PULSE_MOCK_MODE", "true")
	if got := GetDeploymentType(); got != "mock" {
		t.Fatalf("GetDeploymentType = %q, want mock", got)
	}
}

func TestGetDeploymentType_Manual(t *testing.T) {
	setMockRuntime(t, false)

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
