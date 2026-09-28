package sensors

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCollectLocal_EmptyFallbackReturnsError(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "sensors", "#!/bin/sh\necho '{}'\n")
	writeScript(t, dir, "cat", "#!/bin/sh\necho ''\n")
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	useTestSysfsRoots(t)

	_, err := CollectLocal(context.Background())
	if err == nil {
		t.Fatalf("expected error when sensors and Pi fallback are empty")
	}
	if !strings.Contains(err.Error(), "empty output") {
		t.Fatalf("expected empty output error, got %v", err)
	}
}

func TestCollectLocal_MissingSensorsUsesIdentifiedArmadaThermalZone(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	thermal, _ := useTestSysfsRoots(t)
	writeSysfsSensor(t, thermal, "thermal_zone0", "type", "gpu-thermal", "70000")
	writeSysfsSensor(t, thermal, "thermal_zone1", "type", "armada_thermal", "57000")

	out, err := CollectLocal(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := Parse(out)
	if err != nil {
		t.Fatal(err)
	}
	if !parsed.Available || parsed.CPUPackage != 57 || parsed.CPUMax != 57 {
		t.Fatalf("armada sysfs reading not mapped to CPU: %+v", parsed)
	}
}

func TestCollectLocal_SysfsRejectsUnidentifiedAndBadReadings(t *testing.T) {
	for _, tc := range []struct {
		name, chip, value string
	}{
		{"unidentified", "gpu-thermal", "57000"},
		{"empty", "armada_thermal", ""},
		{"degrees-not-millidegrees", "armada_thermal", "57"},
		{"negative", "armada_thermal", "-57000"},
		{"too-hot", "armada_thermal", "150000"},
		{"malformed", "armada_thermal", "57000C"},
		{"non-finite", "armada_thermal", "NaN"},
		{"oversized", "armada_thermal", strings.Repeat("5", maxThermalFileReadBytes+1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("PATH", t.TempDir())
			thermal, _ := useTestSysfsRoots(t)
			writeSysfsSensor(t, thermal, "thermal_zone0", "type", tc.chip, tc.value)
			if out, err := CollectLocal(context.Background()); err == nil {
				t.Fatalf("unexpected CPU temperature %s for %s", out, tc.name)
			}
		})
	}
}

func TestCollectLocal_SysfsContinuesPastBadSource(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	thermal, hwmon := useTestSysfsRoots(t)
	writeSysfsSensor(t, thermal, "thermal_zone0", "type", "armada_thermal", "invalid")
	writeSysfsSensor(t, hwmon, "hwmon3", "name", "armada_thermal", "57500")

	out, err := CollectLocal(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `57.500`) {
		t.Fatalf("expected hwmon fallback, got %s", out)
	}
}

func TestCollectLocal_SensorsJSONGainsMissingSysfsCPU(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "sensors", "#!/bin/sh\necho '{\"nvme-pci-0100\":{\"Composite\":{\"temp1_input\":39}}}'\n")
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	thermal, _ := useTestSysfsRoots(t)
	writeSysfsSensor(t, thermal, "thermal_zone0", "type", "armada_thermal", "57000")
	out, err := CollectLocal(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := Parse(out)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.CPUPackage != 57 || parsed.NVMe["nvme0"] != 39 {
		t.Fatalf("lm-sensors data and sysfs CPU were not combined: %+v", parsed)
	}
}

func TestCollectLocal_ExistingCPUPrecedesSysfs(t *testing.T) {
	dir := t.TempDir()
	const original = `{"coretemp-isa-0000":{"Package id 0":{"temp1_input":42}}}`
	writeScript(t, dir, "sensors", "#!/bin/sh\necho '"+original+"'\n")
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	thermal, _ := useTestSysfsRoots(t)
	writeSysfsSensor(t, thermal, "thermal_zone0", "type", "armada_thermal", "57000")
	out, err := CollectLocal(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if out != original {
		t.Fatalf("existing CPU reading was changed: %s", out)
	}
}

func TestCollectLocal_InvalidSysfsDoesNotDiscardOtherSensors(t *testing.T) {
	dir := t.TempDir()
	const original = `{"nvme-pci-0100":{"Composite":{"temp1_input":39}}}`
	writeScript(t, dir, "sensors", "#!/bin/sh\necho '"+original+"'\n")
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	thermal, _ := useTestSysfsRoots(t)
	writeSysfsSensor(t, thermal, "thermal_zone0", "type", "armada_thermal", "bad")
	out, err := CollectLocal(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if out != original {
		t.Fatalf("valid lm-sensors data was discarded: %s", out)
	}
}

func TestCollectLocal_SysfsNameReadBounded(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	thermal, _ := useTestSysfsRoots(t)
	writeSysfsSensor(t, thermal, "thermal_zone0", "type", "armada_thermal", "57000")
	if err := os.WriteFile(filepath.Join(thermal, "thermal_zone0", "type"), []byte(strings.Repeat("a", maxThermalFileReadBytes+1)), 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := CollectLocal(context.Background()); err == nil {
		t.Fatalf("oversized type unexpectedly accepted: %s", out)
	}
}

func TestCollectLocal_ContextCanceled(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "sensors", "#!/bin/sh\necho '{\"chip\":{\"temp\":{\"temp1_input\":42}}}'\n")
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := CollectLocal(ctx)
	if err == nil {
		t.Fatalf("expected command execution error for canceled context")
	}
	if !strings.Contains(err.Error(), "failed to execute sensors") {
		t.Fatalf("expected command execution error, got %v", err)
	}
}
