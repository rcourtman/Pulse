package updates

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeVersionTag(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"1.0.0", "v1.0.0"},
		{"v1.0.0", "v1.0.0"},
		{"2.3.4", "v2.3.4"},
		{"v2.3.4", "v2.3.4"},
		{" v1.2.3 ", "v1.2.3"},
		{" 1.2.3 ", "v1.2.3"},
		{"", ""},
		{"   ", ""},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := normalizeVersionTag(tc.input)
			if result != tc.expected {
				t.Errorf("normalizeVersionTag(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestNormalizeExecutableMode(t *testing.T) {
	tests := []struct {
		name     string
		input    os.FileMode
		expected os.FileMode
	}{
		{
			name:     "already executable",
			input:    0o755,
			expected: 0o755,
		},
		{
			name:     "not executable",
			input:    0o644,
			expected: 0o755,
		},
		{
			name:     "partial executable",
			input:    0o744,
			expected: 0o744, // already has execute bit
		},
		{
			name:     "read only",
			input:    0o444,
			expected: 0o755,
		},
		{
			name:     "no permissions",
			input:    0o000,
			expected: 0o755,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := normalizeExecutableMode(tc.input)
			if result != tc.expected {
				t.Errorf("normalizeExecutableMode(%o) = %o, want %o", tc.input, result, tc.expected)
			}
		})
	}
}

func TestHostAgentSearchPaths(t *testing.T) {
	// Save and restore env var
	originalEnv := os.Getenv("PULSE_BIN_DIR")
	defer func() {
		if originalEnv != "" {
			os.Setenv("PULSE_BIN_DIR", originalEnv)
		} else {
			os.Unsetenv("PULSE_BIN_DIR")
		}
	}()

	// Test with default (no env var)
	os.Unsetenv("PULSE_BIN_DIR")
	paths := HostAgentSearchPaths()

	if len(paths) == 0 {
		t.Error("HostAgentSearchPaths() returned empty slice")
	}
	if paths[0] != "/opt/pulse/bin" {
		t.Errorf("First path = %q, want /opt/pulse/bin", paths[0])
	}

	// Test with custom env var
	os.Setenv("PULSE_BIN_DIR", "/custom/path")
	paths = HostAgentSearchPaths()

	if paths[0] != "/custom/path" {
		t.Errorf("First path = %q, want /custom/path", paths[0])
	}

	// Verify deduplication
	os.Setenv("PULSE_BIN_DIR", ".")
	paths = HostAgentSearchPaths()

	// "." should only appear once
	dotCount := 0
	for _, p := range paths {
		if p == "." {
			dotCount++
		}
	}
	if dotCount > 1 {
		t.Errorf("'.' appears %d times, should only appear once", dotCount)
	}
}

func TestHostAgentBinary_Fields(t *testing.T) {
	binary := HostAgentBinary{
		Platform:  "linux",
		Arch:      "amd64",
		Filenames: []string{"pulse-host-agent-linux-amd64"},
	}

	if binary.Platform != "linux" {
		t.Errorf("Platform = %q, want linux", binary.Platform)
	}
	if binary.Arch != "amd64" {
		t.Errorf("Arch = %q, want amd64", binary.Arch)
	}
	if len(binary.Filenames) != 1 {
		t.Errorf("Filenames count = %d, want 1", len(binary.Filenames))
	}
}

func TestRequiredHostAgentBinaries(t *testing.T) {
	// Verify expected platforms are present
	expectedPlatforms := map[string]bool{
		"linux-amd64":   false,
		"linux-arm64":   false,
		"linux-armv7":   false,
		"linux-armv6":   false,
		"linux-386":     false,
		"darwin-amd64":  false,
		"darwin-arm64":  false,
		"windows-amd64": false,
		"windows-arm64": false,
		"windows-386":   false,
	}

	for _, binary := range requiredHostAgentBinaries {
		key := binary.Platform + "-" + binary.Arch
		if _, ok := expectedPlatforms[key]; ok {
			expectedPlatforms[key] = true
		}
	}

	for platform, found := range expectedPlatforms {
		if !found {
			t.Errorf("Expected platform %q not found in requiredHostAgentBinaries", platform)
		}
	}
}

func TestHostAgentBinaryExists(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test binary
	testBinary := filepath.Join(tmpDir, "pulse-host-agent-linux-amd64")
	if err := os.WriteFile(testBinary, []byte("test binary"), 0o755); err != nil {
		t.Fatalf("Failed to create test binary: %v", err)
	}

	// Test existing binary
	exists := hostAgentBinaryExists([]string{tmpDir}, []string{"pulse-host-agent-linux-amd64"})
	if !exists {
		t.Error("hostAgentBinaryExists should return true for existing binary")
	}

	// Test non-existing binary
	exists = hostAgentBinaryExists([]string{tmpDir}, []string{"pulse-host-agent-linux-arm64"})
	if exists {
		t.Error("hostAgentBinaryExists should return false for non-existing binary")
	}

	// Test with multiple search paths
	tmpDir2 := t.TempDir()
	exists = hostAgentBinaryExists([]string{tmpDir2, tmpDir}, []string{"pulse-host-agent-linux-amd64"})
	if !exists {
		t.Error("hostAgentBinaryExists should find binary in second search path")
	}

	// Test with directory (should not match)
	dirPath := filepath.Join(tmpDir, "pulse-host-agent-linux-386")
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	exists = hostAgentBinaryExists([]string{tmpDir}, []string{"pulse-host-agent-linux-386"})
	if exists {
		t.Error("hostAgentBinaryExists should return false for directories")
	}
}

func TestFindMissingHostAgentBinaries(t *testing.T) {
	tmpDir := t.TempDir()

	// With no binaries present, all should be missing
	missing := findMissingHostAgentBinaries([]string{tmpDir})
	if len(missing) != len(requiredHostAgentBinaries) {
		t.Errorf("Missing count = %d, want %d", len(missing), len(requiredHostAgentBinaries))
	}

	// Create one binary
	testBinary := filepath.Join(tmpDir, "pulse-host-agent-linux-amd64")
	if err := os.WriteFile(testBinary, []byte("test"), 0o755); err != nil {
		t.Fatalf("Failed to create test binary: %v", err)
	}

	missing = findMissingHostAgentBinaries([]string{tmpDir})
	if len(missing) != len(requiredHostAgentBinaries)-1 {
		t.Errorf("Missing count = %d, want %d", len(missing), len(requiredHostAgentBinaries)-1)
	}

	// Verify linux-amd64 is not in missing
	if _, ok := missing["linux-amd64"]; ok {
		t.Error("linux-amd64 should not be in missing after creating binary")
	}
}

func TestDownloadAndInstallHostAgentBinaries_DevVersion(t *testing.T) {
	tmpDir := t.TempDir()

	// Dev versions should fail
	err := DownloadAndInstallHostAgentBinaries("dev", tmpDir)
	if err == nil {
		t.Error("DownloadAndInstallHostAgentBinaries should fail for 'dev' version")
	}

	err = DownloadAndInstallHostAgentBinaries("vdev", tmpDir)
	if err == nil {
		t.Error("DownloadAndInstallHostAgentBinaries should fail for 'vdev' version")
	}

	err = DownloadAndInstallHostAgentBinaries("", tmpDir)
	if err == nil {
		t.Error("DownloadAndInstallHostAgentBinaries should fail for empty version")
	}
}

func TestWindowsBinaryFilenames(t *testing.T) {
	// Windows binaries should have both with and without .exe
	for _, binary := range requiredHostAgentBinaries {
		if binary.Platform != "windows" {
			continue
		}

		hasExe := false
		hasNoExe := false
		for _, filename := range binary.Filenames {
			if filepath.Ext(filename) == ".exe" {
				hasExe = true
			} else {
				hasNoExe = true
			}
		}

		if !hasExe || !hasNoExe {
			t.Errorf("Windows binary %s-%s should have both .exe and non-.exe filenames", binary.Platform, binary.Arch)
		}
	}
}

func TestNonWindowsBinaryFilenames(t *testing.T) {
	// Non-Windows binaries should not have .exe
	for _, binary := range requiredHostAgentBinaries {
		if binary.Platform == "windows" {
			continue
		}

		for _, filename := range binary.Filenames {
			if filepath.Ext(filename) == ".exe" {
				t.Errorf("Non-Windows binary %s-%s should not have .exe extension: %s",
					binary.Platform, binary.Arch, filename)
			}
		}
	}
}
