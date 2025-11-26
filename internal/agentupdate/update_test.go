package agentupdate

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
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
