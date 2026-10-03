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

func useTestSysfsRoots(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	thermal := filepath.Join(root, "thermal")
	hwmon := filepath.Join(root, "hwmon")
	for _, dir := range []string{thermal, hwmon} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	previousThermal, previousHwmon := thermalZoneRoot, hwmonRoot
	thermalZoneRoot, hwmonRoot = thermal, hwmon
	t.Cleanup(func() { thermalZoneRoot, hwmonRoot = previousThermal, previousHwmon })
	return thermal, hwmon
}

func writeSysfsSensor(t *testing.T, root, dir, label, name, value string) {
	t.Helper()
	path := filepath.Join(root, dir)
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	for file, content := range map[string]string{label: name, "temp": value} {
		if label == "name" && file == "temp" {
			file = "temp1_input"
		}
		if err := os.WriteFile(filepath.Join(path, file), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCollectLocalMissingSensors(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	useTestSysfsRoots(t)

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
	thermal, _ := useTestSysfsRoots(t)
	writeSysfsSensor(t, thermal, "thermal_zone0", "type", "cpu-thermal\n", "42000\n")

	out, err := CollectLocal(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := `{"cpu_thermal-virtual-0":{"temp1":{"temp1_input":42.000}}}`
	if out != expected {
		t.Fatalf("unexpected fallback output: %s", out)
	}
}

func TestCollectLocalFallbackToArmadaHwmon(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "sensors", "#!/bin/sh\necho '{}'\n")
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	_, hwmon := useTestSysfsRoots(t)
	writeSysfsSensor(t, hwmon, "hwmon0", "name", "armada_thermal\n", "42000\n")

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
	thermal, _ := useTestSysfsRoots(t)
	writeSysfsSensor(t, thermal, "thermal_zone0", "type", "cpu-thermal", `42"},"bad":{"temp1_input":1}`)

	if _, err := CollectLocal(context.Background()); err == nil {
		t.Fatal("expected error for invalid fallback thermal value")
	}
}
