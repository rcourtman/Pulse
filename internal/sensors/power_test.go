package sensors

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCollectPower_NoRAPL(t *testing.T) {
	// On most CI systems, RAPL won't be available
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	data, err := CollectPower(ctx)

	// Either should fail (no RAPL) or succeed (has RAPL)
	if err != nil {
		// Expected on systems without RAPL
		t.Logf("Power collection unavailable (expected in CI): %v", err)
		return
	}

	// If we got here, RAPL is available
	if data == nil {
		t.Fatal("Expected non-nil data when no error returned")
	}

	if !data.Available {
		t.Error("Expected data.Available to be true")
	}

	if data.Source != "rapl" && data.Source != "amd_energy" {
		t.Errorf("Expected source 'rapl' or 'amd_energy', got '%s'", data.Source)
	}

	t.Logf("Power data: Package=%.2fW, Core=%.2fW, DRAM=%.2fW (source: %s)",
		data.PackageWatts, data.CoreWatts, data.DRAMWatts, data.Source)
}

func TestPowerData_StructInitialization(t *testing.T) {
	data := &PowerData{}

	if data.Available {
		t.Error("Expected Available to be false by default")
	}

	if data.PackageWatts != 0 {
		t.Error("Expected PackageWatts to be 0 by default")
	}

	if data.Source != "" {
		t.Error("Expected Source to be empty by default")
	}
}

func TestReadUint64File(t *testing.T) {
	// Create temp file with a value
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "energy_uj")

	// Test valid value
	if err := os.WriteFile(testFile, []byte("123456789\n"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	val, err := readUint64File(testFile)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if val != 123456789 {
		t.Errorf("Expected 123456789, got %d", val)
	}

	// Test with whitespace
	if err := os.WriteFile(testFile, []byte("  987654321  \n"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	val, err = readUint64File(testFile)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if val != 987654321 {
		t.Errorf("Expected 987654321, got %d", val)
	}

	// Test non-existent file
	_, err = readUint64File(filepath.Join(tmpDir, "nonexistent"))
	if err == nil {
		t.Error("Expected error for non-existent file")
	}

	// Test invalid content
	if err := os.WriteFile(testFile, []byte("not a number"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	_, err = readUint64File(testFile)
	if err == nil {
		t.Error("Expected error for invalid content")
	}
}

func TestReadStringFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "name")

	// Test valid value
	if err := os.WriteFile(testFile, []byte("package-0\n"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	val, err := readStringFile(testFile)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if val != "package-0" {
		t.Errorf("Expected 'package-0', got '%s'", val)
	}

	// Test with whitespace
	if err := os.WriteFile(testFile, []byte("  core  \n"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	val, err = readStringFile(testFile)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if val != "core" {
		t.Errorf("Expected 'core', got '%s'", val)
	}

	// Test non-existent file
	_, err = readStringFile(filepath.Join(tmpDir, "nonexistent"))
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestReadRAPLEnergy(t *testing.T) {
	// Create mock RAPL structure
	tmpDir := t.TempDir()
	pkg0 := filepath.Join(tmpDir, "intel-rapl:0")
	if err := os.MkdirAll(pkg0, 0755); err != nil {
		t.Fatalf("Failed to create mock RAPL dir: %v", err)
	}

	// Write energy and name files
	if err := os.WriteFile(filepath.Join(pkg0, "energy_uj"), []byte("1000000"), 0644); err != nil {
		t.Fatalf("Failed to write energy file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pkg0, "name"), []byte("package-0"), 0644); err != nil {
		t.Fatalf("Failed to write name file: %v", err)
	}

	// Create subdomain (core)
	core := filepath.Join(pkg0, "intel-rapl:0:0")
	if err := os.MkdirAll(core, 0755); err != nil {
		t.Fatalf("Failed to create mock core dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(core, "energy_uj"), []byte("500000"), 0644); err != nil {
		t.Fatalf("Failed to write core energy file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(core, "name"), []byte("core"), 0644); err != nil {
		t.Fatalf("Failed to write core name file: %v", err)
	}

	// Test reading
	result, err := readRAPLEnergy([]string{pkg0})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("Expected 2 readings, got %d", len(result))
	}

	if val, ok := result["package-0"]; !ok || val != 1000000 {
		t.Errorf("Expected package-0=1000000, got %v", result)
	}

	if val, ok := result["core"]; !ok || val != 500000 {
		t.Errorf("Expected core=500000, got %v", result)
	}
}

func TestReadRAPLEnergy_NoFiles(t *testing.T) {
	tmpDir := t.TempDir()
	pkg0 := filepath.Join(tmpDir, "intel-rapl:0")
	if err := os.MkdirAll(pkg0, 0755); err != nil {
		t.Fatalf("Failed to create mock RAPL dir: %v", err)
	}

	// No energy files
	_, err := readRAPLEnergy([]string{pkg0})
	if err == nil {
		t.Error("Expected error when no energy files exist")
	}
}

func TestFindAMDEnergyHwmon_NotFound(t *testing.T) {
	// This should fail on systems without amd_energy
	_, err := findAMDEnergyHwmon()
	if err == nil {
		// If it succeeds, that's fine - means we're on an AMD system
		t.Log("AMD energy hwmon found (running on AMD system)")
	}
}

func TestCollectPower_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := CollectPower(ctx)
	// Should fail quickly due to cancelled context or no power available
	if err == nil {
		t.Log("CollectPower succeeded despite cancelled context (power data was cached or instant)")
	}
}

func TestPowerCalculation(t *testing.T) {
	// Test that power calculation is correct
	// Power (W) = Energy delta (µJ) / 1e6 / time (s)

	// 1,000,000 µJ over 100ms = 10W
	deltaUJ := uint64(1000000)
	duration := 0.1 // 100ms
	expectedWatts := 10.0

	watts := float64(deltaUJ) / 1e6 / duration
	if watts != expectedWatts {
		t.Errorf("Expected %.2f W, got %.2f W", expectedWatts, watts)
	}

	// 5,000,000 µJ over 100ms = 50W
	deltaUJ = 5000000
	expectedWatts = 50.0
	watts = float64(deltaUJ) / 1e6 / duration
	if watts != expectedWatts {
		t.Errorf("Expected %.2f W, got %.2f W", expectedWatts, watts)
	}
}

func TestCounterWraparound(t *testing.T) {
	// Test normal (no wraparound) case
	energy1 := uint64(1000000)
	energy2 := uint64(2000000)

	var deltaUJ uint64
	if energy2 >= energy1 {
		deltaUJ = energy2 - energy1
	} else {
		deltaUJ = (^uint64(0) - energy1) + energy2 + 1
	}

	expectedDelta := uint64(1000000)
	if deltaUJ != expectedDelta {
		t.Errorf("Normal case: Expected delta %d, got %d", expectedDelta, deltaUJ)
	}

	// Test wraparound case (energy2 < energy1 means counter wrapped)
	energy1 = uint64(18446744073709551610) // Close to max uint64
	energy2 = uint64(100)                   // After wrap

	if energy2 >= energy1 {
		deltaUJ = energy2 - energy1
	} else {
		deltaUJ = (^uint64(0) - energy1) + energy2 + 1
	}

	// Max uint64 is 18446744073709551615
	// So delta should be: (max - 18446744073709551610) + 100 + 1 = 5 + 100 + 1 = 106
	expectedDelta = uint64(106)
	if deltaUJ != expectedDelta {
		t.Errorf("Wraparound case: Expected delta %d, got %d", expectedDelta, deltaUJ)
	}
}
