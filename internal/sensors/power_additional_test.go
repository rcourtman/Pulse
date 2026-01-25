package sensors

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadAMDEnergy(t *testing.T) {
	tmpDir := t.TempDir()

	energy1 := filepath.Join(tmpDir, "energy1_input")
	if err := os.WriteFile(energy1, []byte("1000"), 0644); err != nil {
		t.Fatalf("write energy1_input: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "energy1_label"), []byte("package"), 0644); err != nil {
		t.Fatalf("write energy1_label: %v", err)
	}

	energy2 := filepath.Join(tmpDir, "energy2_input")
	if err := os.WriteFile(energy2, []byte("2000"), 0644); err != nil {
		t.Fatalf("write energy2_input: %v", err)
	}

	energy3 := filepath.Join(tmpDir, "energy3_input")
	if err := os.WriteFile(energy3, []byte("not-a-number"), 0644); err != nil {
		t.Fatalf("write energy3_input: %v", err)
	}

	result, err := readAMDEnergy(tmpDir)
	if err != nil {
		t.Fatalf("readAMDEnergy returned error: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("result = %#v, want 2 readings", result)
	}
	if result["package"] != 1000 {
		t.Fatalf("package = %d, want 1000", result["package"])
	}
	if result["energy2_input"] != 2000 {
		t.Fatalf("energy2_input = %d, want 2000", result["energy2_input"])
	}
	if _, ok := result["energy3_input"]; ok {
		t.Fatalf("energy3_input should be skipped due to parse error")
	}
}

func TestReadAMDEnergy_NoFiles(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := readAMDEnergy(tmpDir)
	if err == nil {
		t.Fatalf("expected error for missing energy files")
	}
}

func TestReadAMDEnergy_NoReadings(t *testing.T) {
	tmpDir := t.TempDir()
	energy1 := filepath.Join(tmpDir, "energy1_input")
	if err := os.WriteFile(energy1, []byte("invalid"), 0644); err != nil {
		t.Fatalf("write energy1_input: %v", err)
	}

	_, err := readAMDEnergy(tmpDir)
	if err == nil {
		t.Fatalf("expected error for unreadable energy values")
	}
}
