package sensors

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeScript(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0700); err != nil {
		t.Fatalf("write script: %v", err)
	}
}

func TestCollectLocalMissingSensors(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)

	if _, err := CollectLocal(context.Background()); err == nil {
		t.Fatal("expected error when sensors missing")
	}
}

func TestCollectLocalSensorsOutput(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "sensors", "#!/bin/sh\necho '{\"chip\":{\"temp\":{\"temp1_input\":42}}}'\n")
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	out, err := CollectLocal(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "{\"chip\":{\"temp\":{\"temp1_input\":42}}}" {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestCollectLocalSensorsOutputWithNonZeroExit(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "sensors", "#!/bin/sh\necho '{\"chip\":{\"temp\":{\"temp1_input\":42}}}'\nexit 1\n")
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	out, err := CollectLocal(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "{\"chip\":{\"temp\":{\"temp1_input\":42}}}" {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestCollectLocalFallbackToPiTemp(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "sensors", "#!/bin/sh\necho '{}'\n")
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	thermalPath := filepath.Join(dir, "thermal_zone0_temp")
	if err := os.WriteFile(thermalPath, []byte("42000\n"), 0600); err != nil {
		t.Fatalf("write thermal file: %v", err)
	}
	originalThermalPath := rpiThermalZonePath
	rpiThermalZonePath = thermalPath
	t.Cleanup(func() {
		rpiThermalZonePath = originalThermalPath
	})

	out, err := CollectLocal(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := `{"cpu_thermal-virtual-0":{"temp1":{"temp1_input":42.000}}}`
	if out != expected {
		t.Fatalf("unexpected fallback output: %s", out)
	}
}

func TestCollectLocalFallbackKeepsDegreeInput(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "sensors", "#!/bin/sh\necho '{}'\n")
	writeScript(t, dir, "cat", "#!/bin/sh\necho '42'\n")
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	thermalPath := filepath.Join(t.TempDir(), "thermal_zone0_temp")
	if err := os.WriteFile(thermalPath, []byte("42000"), 0644); err != nil {
		t.Fatalf("write thermal file: %v", err)
	}
	originalThermalPath := rpiThermalZoneTempPath
	rpiThermalZoneTempPath = thermalPath
	t.Cleanup(func() {
		rpiThermalZoneTempPath = originalThermalPath
	})

	out, err := CollectLocal(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := `{"cpu_thermal-virtual-0":{"temp1":{"temp1_input":42.000}}}`
	if out != expected {
		t.Fatalf("unexpected fallback output: %s", out)
	}
}

func TestCollectLocalAcceptsNonZeroWithOutput(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "sensors", "#!/bin/sh\necho '{\"chip\":{\"temp\":{\"temp1_input\":55}}}'\nexit 1\n")
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	out, err := CollectLocal(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "{\"chip\":{\"temp\":{\"temp1_input\":55}}}" {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestCollectLocalRejectsOversizedOutput(t *testing.T) {
	dir := t.TempDir()
	oversized := strings.Repeat("a", maxSensorsOutputSizeBytes+1)
	writeScript(t, dir, "sensors", "#!/bin/sh\ncat <<'EOF'\n"+oversized+"\nEOF\n")
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	_, err := CollectLocal(context.Background())
	if err == nil {
		t.Fatal("expected error for oversized sensors output")
	}
	if !strings.Contains(err.Error(), "exceeds size limit") {
		t.Fatalf("expected size-limit error, got: %v", err)
	}
}

func TestCollectLocalFallbackRejectsInvalidThermalValue(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "sensors", "#!/bin/sh\necho '{}'\n")
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	thermalPath := filepath.Join(dir, "thermal_zone0_temp")
	if err := os.WriteFile(thermalPath, []byte(`42"},"bad":{"temp1_input":1}`), 0600); err != nil {
		t.Fatalf("write thermal file: %v", err)
	}
	originalThermalPath := rpiThermalZonePath
	rpiThermalZonePath = thermalPath
	t.Cleanup(func() {
		rpiThermalZonePath = originalThermalPath
	})

	if _, err := CollectLocal(context.Background()); err == nil {
		t.Fatal("expected error for invalid fallback thermal value")
	}
}
