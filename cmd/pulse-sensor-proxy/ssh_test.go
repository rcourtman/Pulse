package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func setupTempWrapper(t *testing.T) (scriptPath, thermalFile, binDir, baseDir string) {
	t.Helper()

	baseDir = t.TempDir()

	thermalDir := filepath.Join(baseDir, "sys", "class", "thermal", "thermal_zone0")
	if err := os.MkdirAll(thermalDir, 0o755); err != nil {
		t.Fatalf("failed to create thermal zone directory: %v", err)
	}
	thermalFile = filepath.Join(thermalDir, "temp")

	scriptContent := strings.ReplaceAll(tempWrapperScript, "/sys/class/thermal/thermal_zone0/temp", thermalFile)
	scriptPath = filepath.Join(baseDir, "temp-wrapper.sh")
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0o755); err != nil {
		t.Fatalf("failed to write wrapper script: %v", err)
	}

	binDir = filepath.Join(baseDir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("failed to create bin directory: %v", err)
	}

	linkCommand := func(name string) {
		target, err := exec.LookPath(name)
		if err != nil {
			t.Fatalf("required command %q not found on host: %v", name, err)
		}
		content := fmt.Sprintf("#!/bin/sh\nexec %s \"$@\"\n", target)
		if err := os.WriteFile(filepath.Join(binDir, name), []byte(content), 0o755); err != nil {
			t.Fatalf("failed to create shim for %s: %v", name, err)
		}
	}

	linkCommand("awk")
	linkCommand("cat")

	return scriptPath, thermalFile, binDir, baseDir
}

func runTempWrapper(t *testing.T, scriptPath, binDir string, extraEnv ...string) []byte {
	t.Helper()
	cmd := exec.Command("sh", scriptPath)
	env := []string{"PATH=" + binDir}
	env = append(env, extraEnv...)
	cmd.Env = env
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("temp wrapper failed: %v (output: %s)", err, strings.TrimSpace(string(output)))
	}
	return output
}

func TestTempWrapperFallbackWhenSensorsMissing(t *testing.T) {
	scriptPath, thermalFile, binDir, _ := setupTempWrapper(t)

	if err := os.WriteFile(thermalFile, []byte("51234\n"), 0o644); err != nil {
		t.Fatalf("failed to write thermal zone temperature: %v", err)
	}

	output := runTempWrapper(t, scriptPath, binDir)

	var data map[string]map[string]map[string]float64
	if err := json.Unmarshal(output, &data); err != nil {
		t.Fatalf("failed to parse wrapper output as JSON: %v (output: %s)", err, strings.TrimSpace(string(output)))
	}

	temp1, ok := data["rpitemp-virtual"]["temp1"]["temp1_input"]
	if !ok {
		t.Fatalf("expected rpitemp-virtual temp1 reading in output: %v", data)
	}
	if temp1 != 51.23 {
		t.Fatalf("expected converted temperature 51.23, got %.2f", temp1)
	}
}

func TestTempWrapperFallbackWhenSensorsEmpty(t *testing.T) {
	scriptPath, thermalFile, binDir, _ := setupTempWrapper(t)

	sensorsStub := filepath.Join(binDir, "sensors")
	content := "#!/bin/sh\nexit 0\n"
	if err := os.WriteFile(sensorsStub, []byte(content), 0o755); err != nil {
		t.Fatalf("failed to create sensors stub: %v", err)
	}

	if err := os.WriteFile(thermalFile, []byte("47890\n"), 0o644); err != nil {
		t.Fatalf("failed to write thermal zone temperature: %v", err)
	}

	output := runTempWrapper(t, scriptPath, binDir)

	var data map[string]map[string]map[string]float64
	if err := json.Unmarshal(output, &data); err != nil {
		t.Fatalf("failed to parse wrapper output as JSON: %v (output: %s)", err, strings.TrimSpace(string(output)))
	}

	temp1, ok := data["rpitemp-virtual"]["temp1"]["temp1_input"]
	if !ok {
		t.Fatalf("expected rpitemp-virtual temp1 reading in output: %v", data)
	}
	if temp1 != 47.89 {
		t.Fatalf("expected converted temperature 47.89, got %.2f", temp1)
	}
}

func TestTempWrapperPrefersSensorsOutput(t *testing.T) {
	scriptPath, thermalFile, binDir, _ := setupTempWrapper(t)

	jsonOutput := `{"cpu_thermal-virtual-0":{"temp1":{"temp1_input":42.5}}}`
	sensorsStub := filepath.Join(binDir, "sensors")
	content := fmt.Sprintf("#!/bin/sh\nprintf '%s'\n", jsonOutput)
	if err := os.WriteFile(sensorsStub, []byte(content), 0o755); err != nil {
		t.Fatalf("failed to create sensors stub: %v", err)
	}

	// Ensure thermal zone file exists but should be ignored
	if err := os.WriteFile(thermalFile, []byte("40000\n"), 0o644); err != nil {
		t.Fatalf("failed to write thermal zone temperature: %v", err)
	}

	output := runTempWrapper(t, scriptPath, binDir)

	trimmed := strings.TrimSpace(string(output))
	if trimmed != jsonOutput {
		t.Fatalf("expected wrapper to return sensors output %s, got %s", jsonOutput, trimmed)
	}
}

func TestReadAllWithLimit(t *testing.T) {
	reader := bytes.NewBufferString("abcdefg")
	data, exceeded, err := readAllWithLimit(reader, 4)
	if err != nil {
		t.Fatalf("readAllWithLimit returned error: %v", err)
	}
	if string(data) != "abcd" {
		t.Fatalf("expected truncated output 'abcd', got %q", string(data))
	}
	if !exceeded {
		t.Fatalf("expected exceeded flag when data exceeds limit")
	}

	reader2 := bytes.NewBufferString("xyz")
	data, exceeded, err = readAllWithLimit(reader2, 10)
	if err != nil {
		t.Fatalf("readAllWithLimit returned error: %v", err)
	}
	if string(data) != "xyz" {
		t.Fatalf("expected full output 'xyz', got %q", string(data))
	}
	if exceeded {
		t.Fatalf("did not expect exceeded flag for small output")
	}

	reader3 := bytes.NewBufferString("12345")
	data, exceeded, err = readAllWithLimit(reader3, 0)
	if err != nil {
		t.Fatalf("readAllWithLimit returned error: %v", err)
	}
	if string(data) != "12345" || exceeded {
		t.Fatalf("expected unlimited read to return full data without exceeding")
	}
}
