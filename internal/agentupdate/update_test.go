package agentupdate

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
)

func TestDetermineArch(t *testing.T) {
	arch := determineArch()

	// Should return a non-empty string on known platforms
	if runtime.GOOS == "linux" || runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		if arch == "" {
			t.Error("determineArch() returned empty string on known platform")
		}

		// Should be in format "os-arch"
		expected := runtime.GOOS + "-" + runtime.GOARCH
		// Handle arm normalization
		if runtime.GOARCH == "arm" {
			expected = runtime.GOOS + "-armv7"
		}
		if arch != expected {
			t.Logf("determineArch() = %q (expected %q, but platform-specific normalization may apply)", arch, expected)
		}
	}
}

func TestVerifyBinaryMagic_ELF(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("ELF test only runs on Linux")
	}

	// Create a temp file with valid ELF magic
	tmpDir := t.TempDir()
	validELF := filepath.Join(tmpDir, "valid-elf")
	// ELF magic: 0x7f 'E' 'L' 'F' followed by some padding
	err := os.WriteFile(validELF, []byte{0x7f, 'E', 'L', 'F', 0, 0, 0, 0}, 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	if err := verifyBinaryMagic(validELF); err != nil {
		t.Errorf("verifyBinaryMagic() rejected valid ELF: %v", err)
	}

	// Create invalid file
	invalidFile := filepath.Join(tmpDir, "invalid")
	err = os.WriteFile(invalidFile, []byte("not an elf binary"), 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	if err := verifyBinaryMagic(invalidFile); err == nil {
		t.Error("verifyBinaryMagic() accepted invalid file")
	}
}

func TestVerifyBinaryMagic_PE(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("PE test only runs on Windows")
	}

	tmpDir := t.TempDir()

	// Create a temp file with valid PE magic (MZ)
	validPE := filepath.Join(tmpDir, "valid.exe")
	err := os.WriteFile(validPE, []byte{'M', 'Z', 0, 0, 0, 0, 0, 0}, 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	if err := verifyBinaryMagic(validPE); err != nil {
		t.Errorf("verifyBinaryMagic() rejected valid PE: %v", err)
	}

	// Create invalid file
	invalidFile := filepath.Join(tmpDir, "invalid.exe")
	err = os.WriteFile(invalidFile, []byte("not a pe binary!"), 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	if err := verifyBinaryMagic(invalidFile); err == nil {
		t.Error("verifyBinaryMagic() accepted invalid file")
	}
}

func TestVerifyBinaryMagic_MachO(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Mach-O test only runs on macOS")
	}

	tmpDir := t.TempDir()

	// Create a temp file with valid Mach-O 64-bit magic (little-endian)
	validMachO := filepath.Join(tmpDir, "valid-macho")
	err := os.WriteFile(validMachO, []byte{0xcf, 0xfa, 0xed, 0xfe, 0, 0, 0, 0}, 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	if err := verifyBinaryMagic(validMachO); err != nil {
		t.Errorf("verifyBinaryMagic() rejected valid Mach-O: %v", err)
	}

	// Test universal binary magic
	universalMachO := filepath.Join(tmpDir, "universal-macho")
	err = os.WriteFile(universalMachO, []byte{0xca, 0xfe, 0xba, 0xbe, 0, 0, 0, 0}, 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	if err := verifyBinaryMagic(universalMachO); err != nil {
		t.Errorf("verifyBinaryMagic() rejected valid universal Mach-O: %v", err)
	}

	// Create invalid file
	invalidFile := filepath.Join(tmpDir, "invalid")
	err = os.WriteFile(invalidFile, []byte("not a macho binary"), 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	if err := verifyBinaryMagic(invalidFile); err == nil {
		t.Error("verifyBinaryMagic() accepted invalid file")
	}
}

func TestVerifyBinaryMagic_TooShort(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file that's too short to have magic bytes
	shortFile := filepath.Join(tmpDir, "short")
	err := os.WriteFile(shortFile, []byte{0x7f}, 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	if err := verifyBinaryMagic(shortFile); err == nil {
		t.Error("verifyBinaryMagic() accepted file that's too short")
	}
}

func TestVerifyBinaryMagic_NonExistent(t *testing.T) {
	if err := verifyBinaryMagic("/nonexistent/path/to/file"); err == nil {
		t.Error("verifyBinaryMagic() accepted non-existent file")
	}
}

func TestVerifyBinaryMagic_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()

	emptyFile := filepath.Join(tmpDir, "empty")
	err := os.WriteFile(emptyFile, []byte{}, 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	if err := verifyBinaryMagic(emptyFile); err == nil {
		t.Error("verifyBinaryMagic() accepted empty file")
	}
}

func TestUnraidPersistentPath(t *testing.T) {
	tests := []struct {
		agentName string
		expected  string
	}{
		{"pulse-agent", "/boot/config/plugins/pulse-agent/pulse-agent"},
		{"pulse-docker-agent", "/boot/config/plugins/pulse-docker-agent/pulse-docker-agent"},
		{"custom-agent", "/boot/config/plugins/custom-agent/custom-agent"},
	}

	for _, tc := range tests {
		t.Run(tc.agentName, func(t *testing.T) {
			result := unraidPersistentPath(tc.agentName)
			if result != tc.expected {
				t.Errorf("unraidPersistentPath(%q) = %q, want %q", tc.agentName, result, tc.expected)
			}
		})
	}
}

func TestNewUpdater_Defaults(t *testing.T) {
	cfg := Config{
		PulseURL:       "https://pulse.example.com",
		APIToken:       "test-token",
		AgentName:      "pulse-agent",
		CurrentVersion: "1.0.0",
	}

	updater := New(cfg)

	if updater == nil {
		t.Fatal("New() returned nil")
	}

	// Check default check interval is applied
	if updater.cfg.CheckInterval != 1*60*60*1000000000 { // 1 hour in nanoseconds
		t.Errorf("default CheckInterval not applied, got %v", updater.cfg.CheckInterval)
	}

	if updater.client == nil {
		t.Error("http client is nil")
	}
}

func TestNewUpdater_CustomInterval(t *testing.T) {
	cfg := Config{
		PulseURL:       "https://pulse.example.com",
		APIToken:       "test-token",
		AgentName:      "pulse-agent",
		CurrentVersion: "1.0.0",
		CheckInterval:  30 * 60 * 1000000000, // 30 minutes in nanoseconds
	}

	updater := New(cfg)

	if updater.cfg.CheckInterval != cfg.CheckInterval {
		t.Errorf("custom CheckInterval not preserved, got %v", updater.cfg.CheckInterval)
	}
}

func TestIsUnraid(t *testing.T) {
	// This test verifies the function doesn't panic
	// Actual result depends on the system
	result := isUnraid()
	t.Logf("isUnraid() = %v (depends on test environment)", result)
}

func TestConfig_Defaults(t *testing.T) {
	cfg := Config{}

	// Verify zero values
	if cfg.PulseURL != "" {
		t.Error("PulseURL should be empty by default")
	}
	if cfg.APIToken != "" {
		t.Error("APIToken should be empty by default")
	}
	if cfg.AgentName != "" {
		t.Error("AgentName should be empty by default")
	}
	if cfg.CurrentVersion != "" {
		t.Error("CurrentVersion should be empty by default")
	}
	if cfg.CheckInterval != 0 {
		t.Error("CheckInterval should be 0 by default")
	}
	if cfg.InsecureSkipVerify {
		t.Error("InsecureSkipVerify should be false by default")
	}
	if cfg.Logger != nil {
		t.Error("Logger should be nil by default")
	}
	if cfg.Disabled {
		t.Error("Disabled should be false by default")
	}
}

func TestNewUpdater_InsecureSkipVerify(t *testing.T) {
	cfg := Config{
		PulseURL:           "https://pulse.example.com",
		APIToken:           "test-token",
		AgentName:          "pulse-agent",
		CurrentVersion:     "1.0.0",
		InsecureSkipVerify: true,
	}

	updater := New(cfg)

	if updater == nil {
		t.Fatal("New() returned nil")
	}

	if !updater.cfg.InsecureSkipVerify {
		t.Error("InsecureSkipVerify should be true")
	}
}

func TestNewUpdater_Disabled(t *testing.T) {
	cfg := Config{
		PulseURL:       "https://pulse.example.com",
		APIToken:       "test-token",
		AgentName:      "pulse-agent",
		CurrentVersion: "1.0.0",
		Disabled:       true,
	}

	updater := New(cfg)

	if updater == nil {
		t.Fatal("New() returned nil")
	}

	if !updater.cfg.Disabled {
		t.Error("Disabled should be true")
	}
}

func TestDetermineArch_KnownPlatforms(t *testing.T) {
	arch := determineArch()

	// On known platforms, should return os-arch format
	switch runtime.GOOS {
	case "linux", "darwin", "windows":
		if arch == "" {
			t.Error("determineArch() should return non-empty string on known platforms")
		}
		// Should contain hyphen separating OS and arch
		if len(arch) < 3 || arch[0] == '-' || arch[len(arch)-1] == '-' {
			t.Errorf("determineArch() = %q, invalid format", arch)
		}
	}
}

func TestVerifyBinaryMagic_DirectoryFails(t *testing.T) {
	tmpDir := t.TempDir()

	// Trying to verify a directory should fail
	err := verifyBinaryMagic(tmpDir)
	if err == nil {
		t.Error("verifyBinaryMagic() should fail for directories")
	}
}

func TestConstants(t *testing.T) {
	// Verify constants are sensible
	if maxBinarySize <= 0 {
		t.Error("maxBinarySize should be positive")
	}
	if maxBinarySize < 1024*1024 {
		t.Error("maxBinarySize should be at least 1MB")
	}

	if downloadTimeout <= 0 {
		t.Error("downloadTimeout should be positive")
	}
}

func TestSymlinkResolution(t *testing.T) {
	// Test that filepath.EvalSymlinks works as expected for our use case
	tmpDir := t.TempDir()

	// Create a real file
	realFile := filepath.Join(tmpDir, "real", "binary")
	if err := os.MkdirAll(filepath.Dir(realFile), 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	if err := os.WriteFile(realFile, []byte("test"), 0755); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	// Create a symlink to it
	linkFile := filepath.Join(tmpDir, "link", "binary")
	if err := os.MkdirAll(filepath.Dir(linkFile), 0755); err != nil {
		t.Fatalf("failed to create link dir: %v", err)
	}
	if err := os.Symlink(realFile, linkFile); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	// EvalSymlinks should resolve to the real path
	resolved, err := filepath.EvalSymlinks(linkFile)
	if err != nil {
		t.Fatalf("EvalSymlinks failed: %v", err)
	}

	// Normalize realFile in case tmpDir itself contains symlinks (e.g. /var -> /private/var on macOS)
	realFileResolved, err := filepath.EvalSymlinks(realFile)
	if err != nil {
		t.Fatalf("EvalSymlinks on realFile failed: %v", err)
	}

	if resolved != realFileResolved {
		t.Errorf("EvalSymlinks(%q) = %q, want %q", linkFile, resolved, realFileResolved)
	}

	// Verify temp file in same dir as resolved path allows rename
	targetDir := filepath.Dir(resolved)
	tmpFile, err := os.CreateTemp(targetDir, "test-*.tmp")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	// Rename should work since both files are on the same filesystem
	newPath := filepath.Join(targetDir, "renamed")
	if err := os.Rename(tmpPath, newPath); err != nil {
		t.Errorf("Rename failed (same filesystem): %v", err)
	}
	os.Remove(newPath)
}

func TestNormalizeVersion(t *testing.T) {
	if got := utils.NormalizeVersion("v4.33.1"); got != "4.33.1" {
		t.Errorf("NormalizeVersion(%q) = %q, want %q", "v4.33.1", got, "4.33.1")
	}
	if got := utils.NormalizeVersion("4.33.1"); got != "4.33.1" {
		t.Errorf("NormalizeVersion(%q) = %q, want %q", "4.33.1", got, "4.33.1")
	}
}
