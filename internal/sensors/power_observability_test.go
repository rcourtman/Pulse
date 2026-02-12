package sensors

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func TestCollectPower_LogsRAPLFailureBeforeAMDFallback(t *testing.T) {
	originalRAPL := raplBasePath
	originalHwmon := hwmonBasePath
	tmpDir := t.TempDir()
	raplBasePath = filepath.Join(tmpDir, "missing-rapl")
	hwmonBasePath = filepath.Join(tmpDir, "hwmon")
	t.Cleanup(func() {
		raplBasePath = originalRAPL
		hwmonBasePath = originalHwmon
	})

	if err := os.MkdirAll(hwmonBasePath, 0o755); err != nil {
		t.Fatalf("mkdir hwmon base: %v", err)
	}
	hwmon := filepath.Join(hwmonBasePath, "hwmon0")
	if err := os.MkdirAll(hwmon, 0o755); err != nil {
		t.Fatalf("mkdir hwmon: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hwmon, "name"), []byte("amd_energy"), 0o644); err != nil {
		t.Fatalf("write hwmon name: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hwmon, "energy1_input"), []byte("1000000"), 0o644); err != nil {
		t.Fatalf("write energy input: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hwmon, "energy1_label"), []byte("package"), 0o644); err != nil {
		t.Fatalf("write energy label: %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		time.Sleep(20 * time.Millisecond)
		if err := os.WriteFile(filepath.Join(hwmon, "energy1_input"), []byte("2000000"), 0o644); err != nil {
			errCh <- err
		}
	}()

	logOutput := captureSensorsPowerLogs(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	data, err := CollectPower(ctx)
	if err != nil {
		t.Fatalf("CollectPower error: %v", err)
	}
	select {
	case err := <-errCh:
		t.Fatalf("update energy input: %v", err)
	default:
	}
	if data == nil || data.Source != "amd_energy" {
		t.Fatalf("expected amd_energy fallback, got %+v", data)
	}

	for _, expected := range []string{
		`"component":"sensors_power"`,
		`"action":"collect_rapl_failed"`,
		`"message":"Failed to collect Intel RAPL power data"`,
	} {
		if !strings.Contains(logOutput.String(), expected) {
			t.Fatalf("expected log output to include %s, got %q", expected, logOutput.String())
		}
	}
}

func TestReadAMDEnergy_LogsSkippedEnergyReadErrors(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "energy1_input"), []byte("invalid"), 0o644); err != nil {
		t.Fatalf("write energy1_input: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "energy2_input"), []byte("2000"), 0o644); err != nil {
		t.Fatalf("write energy2_input: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "energy2_label"), []byte("package"), 0o644); err != nil {
		t.Fatalf("write energy2_label: %v", err)
	}

	logOutput := captureSensorsPowerLogs(t)
	result, err := readAMDEnergy(tmpDir)
	if err != nil {
		t.Fatalf("readAMDEnergy returned error: %v", err)
	}
	if result["package"] != 2000 {
		t.Fatalf("result = %#v, expected package reading", result)
	}

	for _, expected := range []string{
		`"component":"sensors_power"`,
		`"action":"read_amd_energy_failed"`,
		`"message":"Failed to read AMD energy counter"`,
		`energy1_input`,
	} {
		if !strings.Contains(logOutput.String(), expected) {
			t.Fatalf("expected log output to include %s, got %q", expected, logOutput.String())
		}
	}
}

func captureSensorsPowerLogs(t *testing.T) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer
	origLogger := log.Logger
	log.Logger = zerolog.New(&buf).Level(zerolog.DebugLevel).With().Timestamp().Logger()
	t.Cleanup(func() {
		log.Logger = origLogger
	})

	return &buf
}
