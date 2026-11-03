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

func TestGetCurrentVersion_SourceBuildIdentity(t *testing.T) {
	oldBuildVersion := BuildVersion
	t.Cleanup(func() { BuildVersion = oldBuildVersion })
	t.Setenv("PATH", "")
	for _, tc := range []struct {
		name, version, build, channel string
		marker, source, development   bool
	}{
		{name: "stable", version: "6.4.1", build: "release", channel: "stable"},
		{name: "preview", version: "6.4.4-beta.1", build: "release", channel: "rc"},
		{name: "reporter", version: "6.4.1+test.1913.bd37ae18", build: "development", source: true, development: true},
		{name: "git metadata", version: "6.4.1+git.2.gabcdef", build: "development", source: true, development: true},
		{name: "dev base", version: "6.4.1-dev", build: "development", source: true, development: true},
		{name: "sentinel", version: "0.0.0-qual-build", build: "development", source: true, development: true},
		{name: "marker", version: "6.4.1", build: "source", marker: true, source: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			BuildVersion = tc.version
			if tc.marker {
				if err := os.WriteFile("BUILD_FROM_SOURCE", []byte("1"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			info, err := GetCurrentVersion()
			if err != nil {
				t.Fatal(err)
			}
			if info.Version != tc.version || info.Build != tc.build || info.Channel != tc.channel ||
				info.IsSourceBuild != tc.source || info.IsDevelopment != tc.development {
				t.Fatalf("unexpected build identity: %+v", info)
			}
		})
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
	if !info.IsSourceBuild || info.Channel != "" {
		t.Fatalf("development build must be source-only: %+v", info)
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

func TestIsProxmoxVEUpdateHelper(t *testing.T) {
	// Current community-scripts bootstrap, as written by write_update_entrypoint
	// in community-scripts/core. It names the app only through the export.
	communityBootstrap := func(name string) string {
		return `#!/usr/bin/env bash
# Community-Scripts update entrypoint (generated - do not edit by hand).
# Regenerated on install and on every successful update.
export SCRIPT_SLUG="` + name + `"
export UPDATE_SCRIPT_NAME="` + name + `"
export COMMUNITY_SCRIPTS_URL="https://raw.githubusercontent.com/community-scripts/ProxmoxVE/main"
export COMMUNITY_SCRIPTS_CORE_URL="https://raw.githubusercontent.com/community-scripts/core/main"
export COMMUNITY_SCRIPTS_WEBSITE_URL="https://community-scripts.org"
_cs_helper="$(curl -fsSL --connect-timeout 10 "${COMMUNITY_SCRIPTS_CORE_URL}/misc/update.sh" 2>/dev/null)"
if [[ -n "$_cs_helper" ]]; then
  bash -c "$_cs_helper"
else
  echo "Update helper unavailable - falling back to direct update."
  bash -c "$(curl -fsSL "${COMMUNITY_SCRIPTS_URL}/ct/${UPDATE_SCRIPT_NAME}.sh")"
fi
`
	}

	cases := []struct {
		name    string
		content string
		want    bool
	}{
		{"community-scripts bootstrap", communityBootstrap("pulse"), true},
		{"legacy community-scripts direct pull", `bash -c "$(curl -fsSL https://github.com/community-scripts/ProxmoxVE/raw/main/ct/pulse.sh)"`, true},
		{"pulse installer helper", "#!/usr/bin/env bash\n# Pulse update command\nINSTALLER_URL=\"https://github.com/rcourtman/Pulse/releases/latest/download/install.sh\"\n", true},
		{"bootstrap for another app", communityBootstrap("plex"), false},
		{"bootstrap for a pulse-prefixed app", communityBootstrap("pulsechain"), false},
		{"unrelated script", "#!/bin/sh\napt update\n", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isProxmoxVEUpdateHelper(tc.content); got != tc.want {
				t.Fatalf("isProxmoxVEUpdateHelper = %v, want %v", got, tc.want)
			}
		})
	}
}
