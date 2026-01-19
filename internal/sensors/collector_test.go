package sensors

import (
	"context"
	"os"
	"path/filepath"
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

func TestCollectLocalFallbackToPiTemp(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "sensors", "#!/bin/sh\necho '{}'\n")
	writeScript(t, dir, "cat", "#!/bin/sh\necho '42000'\n")
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	out, err := CollectLocal(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := `{"cpu_thermal-virtual-0":{"temp1":{"temp1_input":42000}}}`
	if out != expected {
		t.Fatalf("unexpected fallback output: %s", out)
	}
}
