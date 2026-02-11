package agentupdate

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func TestDetermineArch(t *testing.T) {
	// Test that determineArch returns a non-empty string on common platforms
	result := determineArch()

	// On known platforms, should return os-arch format
	switch runtime.GOOS {
	case "linux", "darwin", "windows":
		if result == "" {
			t.Errorf("determineArch() returned empty string on %s/%s", runtime.GOOS, runtime.GOARCH)
		}

		// Should contain the OS
		if len(result) < len(runtime.GOOS) {
			t.Errorf("determineArch() = %q, expected to start with %s", result, runtime.GOOS)
		}

		// Should be in format "os-arch"
		expectedPrefix := runtime.GOOS + "-"
		if result[:len(expectedPrefix)] != expectedPrefix {
			t.Errorf("determineArch() = %q, expected to start with %q", result, expectedPrefix)
		}
	}
}

func TestDetermineArch_Format(t *testing.T) {
	result := determineArch()
	if result == "" {
		t.Skip("determineArch returned empty string on this platform")
	}

	// Result should contain a dash separating OS and arch
	found := false
	for i := 0; i < len(result); i++ {
		if result[i] == '-' {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("determineArch() = %q, expected format os-arch with dash separator", result)
	}
}

func TestUnraidPersistentPath(t *testing.T) {
	tests := []struct {
		agentName string
		expected  string
	}{
		{
			agentName: "pulse-agent",
			expected:  "/boot/config/plugins/pulse-agent/pulse-agent",
		},
		{
			agentName: "pulse-docker-agent",
			expected:  "/boot/config/plugins/pulse-docker-agent/pulse-docker-agent",
		},
		{
			agentName: "pulse-host-agent",
			expected:  "/boot/config/plugins/pulse-host-agent/pulse-host-agent",
		},
		{
			agentName: "custom-agent",
			expected:  "/boot/config/plugins/custom-agent/custom-agent",
		},
		{
			agentName: "",
			expected:  "/boot/config/plugins//",
		},
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

func TestVerifyBinaryMagic_ELF(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("ELF verification test only runs on Linux")
	}

	// Create a temp file with ELF magic bytes
	tmpDir := t.TempDir()
	elfPath := filepath.Join(tmpDir, "test_elf")

	// ELF magic: 0x7f 'E' 'L' 'F' followed by some data
	elfData := []byte{0x7f, 'E', 'L', 'F', 0x02, 0x01, 0x01, 0x00}
	if err := os.WriteFile(elfPath, elfData, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	err := verifyBinaryMagic(elfPath)
	if err != nil {
		t.Errorf("verifyBinaryMagic() error = %v for valid ELF", err)
	}
}

func TestVerifyBinaryMagic_InvalidELF(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("ELF verification test only runs on Linux")
	}

	tmpDir := t.TempDir()
	invalidPath := filepath.Join(tmpDir, "invalid")

	// Write invalid magic bytes
	invalidData := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	if err := os.WriteFile(invalidPath, invalidData, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	err := verifyBinaryMagic(invalidPath)
	if err == nil {
		t.Error("verifyBinaryMagic() expected error for invalid binary")
	}
}

func TestVerifyBinaryMagic_MachO64(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Mach-O verification test only runs on macOS")
	}

	tmpDir := t.TempDir()
	machoPath := filepath.Join(tmpDir, "test_macho")

	// Mach-O 64-bit magic (little-endian): 0xcf 0xfa 0xed 0xfe
	machoData := []byte{0xcf, 0xfa, 0xed, 0xfe, 0x07, 0x00, 0x00, 0x01}
	if err := os.WriteFile(machoPath, machoData, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	err := verifyBinaryMagic(machoPath)
	if err != nil {
		t.Errorf("verifyBinaryMagic() error = %v for valid Mach-O", err)
	}
}

func TestVerifyBinaryMagic_MachO32(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Mach-O verification test only runs on macOS")
	}

	tmpDir := t.TempDir()
	machoPath := filepath.Join(tmpDir, "test_macho32")

	// Mach-O 32-bit magic (little-endian): 0xce 0xfa 0xed 0xfe
	machoData := []byte{0xce, 0xfa, 0xed, 0xfe, 0x07, 0x00, 0x00, 0x01}
	if err := os.WriteFile(machoPath, machoData, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	err := verifyBinaryMagic(machoPath)
	if err != nil {
		t.Errorf("verifyBinaryMagic() error = %v for valid Mach-O 32-bit", err)
	}
}

func TestVerifyBinaryMagic_MachOUniversal(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Mach-O verification test only runs on macOS")
	}

	tmpDir := t.TempDir()
	machoPath := filepath.Join(tmpDir, "test_macho_fat")

	// Mach-O universal/fat binary magic: 0xca 0xfe 0xba 0xbe
	machoData := []byte{0xca, 0xfe, 0xba, 0xbe, 0x00, 0x00, 0x00, 0x02}
	if err := os.WriteFile(machoPath, machoData, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	err := verifyBinaryMagic(machoPath)
	if err != nil {
		t.Errorf("verifyBinaryMagic() error = %v for valid Mach-O universal", err)
	}
}

func TestVerifyBinaryMagic_InvalidMachO(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Mach-O verification test only runs on macOS")
	}

	tmpDir := t.TempDir()
	invalidPath := filepath.Join(tmpDir, "invalid")

	// Write invalid magic bytes
	invalidData := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	if err := os.WriteFile(invalidPath, invalidData, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	err := verifyBinaryMagic(invalidPath)
	if err == nil {
		t.Error("verifyBinaryMagic() expected error for invalid Mach-O binary")
	}
}

func TestVerifyBinaryMagic_PE(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("PE verification test only runs on Windows")
	}

	tmpDir := t.TempDir()
	pePath := filepath.Join(tmpDir, "test_pe.exe")

	// PE magic: 'M' 'Z'
	peData := []byte{'M', 'Z', 0x90, 0x00, 0x03, 0x00, 0x00, 0x00}
	if err := os.WriteFile(pePath, peData, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	err := verifyBinaryMagic(pePath)
	if err != nil {
		t.Errorf("verifyBinaryMagic() error = %v for valid PE", err)
	}
}

func TestVerifyBinaryMagic_InvalidPE(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("PE verification test only runs on Windows")
	}

	tmpDir := t.TempDir()
	invalidPath := filepath.Join(tmpDir, "invalid.exe")

	// Write invalid magic bytes (not MZ)
	invalidData := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	if err := os.WriteFile(invalidPath, invalidData, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	err := verifyBinaryMagic(invalidPath)
	if err == nil {
		t.Error("verifyBinaryMagic() expected error for invalid PE binary")
	}
}

func TestVerifyBinaryMagic_NonexistentFile(t *testing.T) {
	err := verifyBinaryMagic("/nonexistent/path/to/binary")
	if err == nil {
		t.Error("verifyBinaryMagic() expected error for nonexistent file")
	}
}

func TestVerifyBinaryMagic_TooShort(t *testing.T) {
	tmpDir := t.TempDir()
	shortPath := filepath.Join(tmpDir, "short")

	// Write only 2 bytes (less than 4 required for magic)
	shortData := []byte{0x7f, 'E'}
	if err := os.WriteFile(shortPath, shortData, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	err := verifyBinaryMagic(shortPath)
	if err == nil {
		t.Error("verifyBinaryMagic() expected error for file too short to read magic")
	}
}

func TestVerifyBinaryMagic_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	emptyPath := filepath.Join(tmpDir, "empty")

	// Write empty file
	if err := os.WriteFile(emptyPath, []byte{}, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	err := verifyBinaryMagic(emptyPath)
	if err == nil {
		t.Error("verifyBinaryMagic() expected error for empty file")
	}
}

func TestVerifyBinaryMagic_TextFile(t *testing.T) {
	// Skip on unknown platforms where verification is skipped
	switch runtime.GOOS {
	case "linux", "darwin", "windows":
		// continue with test
	default:
		t.Skip("Platform verification skipped on unknown OS")
	}

	tmpDir := t.TempDir()
	textPath := filepath.Join(tmpDir, "script.sh")

	// Write a shell script (not a binary)
	textData := []byte("#!/bin/bash\necho hello\n")
	if err := os.WriteFile(textPath, textData, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	err := verifyBinaryMagic(textPath)
	if err == nil {
		t.Error("verifyBinaryMagic() expected error for text file")
	}
}

func TestIsUnraid(t *testing.T) {
	// isUnraid checks for /etc/unraid-version
	// On non-Unraid systems, this should return false
	result := isUnraid()

	// Check if /etc/unraid-version exists
	_, err := os.Stat("/etc/unraid-version")
	expected := err == nil

	if result != expected {
		t.Errorf("isUnraid() = %v, want %v", result, expected)
	}
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
		t.Error("CheckInterval should be zero by default")
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

func TestNew_DefaultCheckInterval(t *testing.T) {
	cfg := Config{
		PulseURL:       "https://pulse.example.com",
		APIToken:       "test-token",
		AgentName:      "pulse-agent",
		CurrentVersion: "1.0.0",
		// CheckInterval not set
	}

	updater := New(cfg)

	// Should have defaulted to 1 hour
	if updater.cfg.CheckInterval != 1*time.Hour {
		t.Errorf("CheckInterval = %v, want 1h", updater.cfg.CheckInterval)
	}
}

func TestNew_CustomCheckInterval(t *testing.T) {
	cfg := Config{
		PulseURL:       "https://pulse.example.com",
		APIToken:       "test-token",
		AgentName:      "pulse-agent",
		CurrentVersion: "1.0.0",
		CheckInterval:  30 * time.Minute,
	}

	updater := New(cfg)

	// Should preserve custom interval
	if updater.cfg.CheckInterval != 30*time.Minute {
		t.Errorf("CheckInterval = %v, want 30m", updater.cfg.CheckInterval)
	}
}

func TestNew_NegativeCheckIntervalUsesDefault(t *testing.T) {
	cfg := Config{
		PulseURL:       "https://pulse.example.com",
		APIToken:       "test-token",
		AgentName:      "pulse-agent",
		CurrentVersion: "1.0.0",
		CheckInterval:  -30 * time.Second,
	}

	updater := New(cfg)

	if updater.cfg.CheckInterval != 1*time.Hour {
		t.Errorf("CheckInterval = %v, want 1h default", updater.cfg.CheckInterval)
	}
}

func TestNew_WithLogger(t *testing.T) {
	logger := zerolog.Nop()
	cfg := Config{
		PulseURL:       "https://pulse.example.com",
		APIToken:       "test-token",
		AgentName:      "pulse-agent",
		CurrentVersion: "1.0.0",
		Logger:         &logger,
	}

	updater := New(cfg)

	// Should not panic and should create valid updater
	if updater == nil {
		t.Error("New() returned nil")
	}
}

func TestNew_NilLogger(t *testing.T) {
	cfg := Config{
		PulseURL:       "https://pulse.example.com",
		APIToken:       "test-token",
		AgentName:      "pulse-agent",
		CurrentVersion: "1.0.0",
		Logger:         nil,
	}

	updater := New(cfg)

	// Should not panic and should create valid updater with nop logger
	if updater == nil {
		t.Error("New() returned nil")
	}
}

func TestNew_InsecureSkipVerify(t *testing.T) {
	cfg := Config{
		PulseURL:           "https://pulse.example.com",
		APIToken:           "test-token",
		AgentName:          "pulse-agent",
		CurrentVersion:     "1.0.0",
		InsecureSkipVerify: true,
	}

	updater := New(cfg)

	if !updater.cfg.InsecureSkipVerify {
		t.Error("InsecureSkipVerify should be true")
	}
}

func TestNew_ClientNotNil(t *testing.T) {
	cfg := Config{
		PulseURL:       "https://pulse.example.com",
		APIToken:       "test-token",
		AgentName:      "pulse-agent",
		CurrentVersion: "1.0.0",
	}

	updater := New(cfg)

	if updater.client == nil {
		t.Error("client should not be nil")
	}
}

func TestConstants(t *testing.T) {
	// Verify maxBinarySize is reasonable (100 MB)
	if maxBinarySize != 100*1024*1024 {
		t.Errorf("maxBinarySize = %d, want %d", maxBinarySize, 100*1024*1024)
	}

	// Verify downloadTimeout is reasonable (5 minutes)
	if downloadTimeout != 5*time.Minute {
		t.Errorf("downloadTimeout = %v, want 5m", downloadTimeout)
	}
}
