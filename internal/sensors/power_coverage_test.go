package sensors

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCollectPower_PrefersRAPLWhenAvailable(t *testing.T) {
	tmpDir := t.TempDir()
	originalRAPL := raplBasePath
	originalHwmon := hwmonBasePath
	raplBasePath = filepath.Join(tmpDir, "rapl")
	hwmonBasePath = filepath.Join(tmpDir, "hwmon")
	t.Cleanup(func() {
		raplBasePath = originalRAPL
		hwmonBasePath = originalHwmon
	})

	pkg0 := filepath.Join(raplBasePath, "intel-rapl:0")
	if err := os.MkdirAll(pkg0, 0755); err != nil {
		t.Fatalf("mkdir pkg0: %v", err)
	}
	if err := writeEnergyFile(filepath.Join(pkg0, "energy_uj"), "1000000"); err != nil {
		t.Fatalf("write package energy: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pkg0, "name"), []byte("package-0"), 0644); err != nil {
		t.Fatalf("write package name: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(hwmonBasePath, "hwmon0"), 0755); err != nil {
		t.Fatalf("mkdir hwmon: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hwmonBasePath, "hwmon0", "name"), []byte("amd_energy"), 0644); err != nil {
		t.Fatalf("write hwmon name: %v", err)
	}
	if err := writeEnergyFile(filepath.Join(hwmonBasePath, "hwmon0", "energy1_input"), "1000000"); err != nil {
		t.Fatalf("write amd energy: %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		time.Sleep(20 * time.Millisecond)
		if err := writeEnergyFile(filepath.Join(pkg0, "energy_uj"), "2000000"); err != nil {
			errCh <- err
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	data, err := CollectPower(ctx)
	if err != nil {
		t.Fatalf("CollectPower error: %v", err)
	}
	select {
	case err := <-errCh:
		t.Fatalf("update RAPL energy: %v", err)
	default:
	}
	if data.Source != "rapl" {
		t.Fatalf("expected rapl source, got %q", data.Source)
	}
	if data.PackageWatts <= 0 {
		t.Fatalf("expected package watts > 0, got %+v", data)
	}
}

func TestCollectRALP_ContextCanceledBeforeSecondSample(t *testing.T) {
	tmpDir := t.TempDir()
	original := raplBasePath
	raplBasePath = tmpDir
	t.Cleanup(func() {
		raplBasePath = original
	})

	pkg0 := filepath.Join(tmpDir, "intel-rapl:0")
	if err := os.MkdirAll(pkg0, 0755); err != nil {
		t.Fatalf("mkdir pkg0: %v", err)
	}
	if err := writeEnergyFile(filepath.Join(pkg0, "energy_uj"), "1000000"); err != nil {
		t.Fatalf("write package energy: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := collectRALP(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled error, got %v", err)
	}
}

func TestCollectAMDEnergy_LabelRoutingAndWraparound(t *testing.T) {
	tmpDir := t.TempDir()
	original := hwmonBasePath
	hwmonBasePath = tmpDir
	t.Cleanup(func() {
		hwmonBasePath = original
	})

	hwmon := filepath.Join(tmpDir, "hwmon0")
	if err := os.MkdirAll(hwmon, 0755); err != nil {
		t.Fatalf("mkdir hwmon: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hwmon, "name"), []byte("amd_energy"), 0644); err != nil {
		t.Fatalf("write hwmon name: %v", err)
	}

	if err := writeEnergyFile(filepath.Join(hwmon, "energy1_input"), "1000000"); err != nil {
		t.Fatalf("write core energy1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hwmon, "energy1_label"), []byte("core"), 0644); err != nil {
		t.Fatalf("write core label: %v", err)
	}

	if err := writeEnergyFile(filepath.Join(hwmon, "energy2_input"), "18446744073709551610"); err != nil {
		t.Fatalf("write misc energy1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hwmon, "energy2_label"), []byte("misc"), 0644); err != nil {
		t.Fatalf("write misc label: %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		time.Sleep(20 * time.Millisecond)
		if err := writeEnergyFile(filepath.Join(hwmon, "energy1_input"), "2000000"); err != nil {
			errCh <- err
			return
		}
		if err := writeEnergyFile(filepath.Join(hwmon, "energy2_input"), "100"); err != nil {
			errCh <- err
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	data, err := collectAMDEnergy(ctx)
	if err != nil {
		t.Fatalf("collectAMDEnergy error: %v", err)
	}
	select {
	case err := <-errCh:
		t.Fatalf("update AMD energy inputs: %v", err)
	default:
	}

	if !data.Available {
		t.Fatalf("expected available power data")
	}
	if data.Source != "amd_energy" {
		t.Fatalf("expected amd_energy source, got %q", data.Source)
	}
	if data.CoreWatts <= 0 {
		t.Fatalf("expected core watts > 0, got %+v", data)
	}
	if data.PackageWatts <= 0 {
		t.Fatalf("expected package watts > 0 from unlabeled fallback, got %+v", data)
	}
	if data.PackageWatts >= 1 {
		t.Fatalf("expected wraparound-derived package watts to stay small, got %f", data.PackageWatts)
	}
}

func TestFindAMDEnergyHwmon_DeterministicBranches(t *testing.T) {
	tmpDir := t.TempDir()
	original := hwmonBasePath
	hwmonBasePath = tmpDir
	t.Cleanup(func() {
		hwmonBasePath = original
	})

	if err := os.WriteFile(filepath.Join(tmpDir, "not-a-dir"), []byte("x"), 0644); err != nil {
		t.Fatalf("write file entry: %v", err)
	}

	missingNameDir := filepath.Join(tmpDir, "hwmon0")
	if err := os.MkdirAll(missingNameDir, 0755); err != nil {
		t.Fatalf("mkdir missingNameDir: %v", err)
	}

	otherDir := filepath.Join(tmpDir, "hwmon1")
	if err := os.MkdirAll(otherDir, 0755); err != nil {
		t.Fatalf("mkdir otherDir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(otherDir, "name"), []byte("acpitz"), 0644); err != nil {
		t.Fatalf("write other name: %v", err)
	}

	if _, err := findAMDEnergyHwmon(); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error before amd_energy exists, got %v", err)
	}

	targetDir := filepath.Join(tmpDir, "hwmon2")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatalf("mkdir targetDir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, "name"), []byte("amd_energy"), 0644); err != nil {
		t.Fatalf("write target name: %v", err)
	}

	got, err := findAMDEnergyHwmon()
	if err != nil {
		t.Fatalf("findAMDEnergyHwmon returned error: %v", err)
	}
	if got != targetDir {
		t.Fatalf("got path %q, want %q", got, targetDir)
	}
}
