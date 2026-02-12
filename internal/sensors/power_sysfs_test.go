package sensors

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeEnergyFile(path string, value string) error {
	return os.WriteFile(path, []byte(value), 0644)
}

func TestCollectRALP_MockSysfs(t *testing.T) {
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
	if err := os.WriteFile(filepath.Join(pkg0, "name"), []byte("package-0"), 0644); err != nil {
		t.Fatalf("write name: %v", err)
	}

	core := filepath.Join(pkg0, "intel-rapl:0:0")
	if err := os.MkdirAll(core, 0755); err != nil {
		t.Fatalf("mkdir core: %v", err)
	}
	if err := writeEnergyFile(filepath.Join(core, "energy_uj"), "200000"); err != nil {
		t.Fatalf("write core energy: %v", err)
	}
	if err := os.WriteFile(filepath.Join(core, "name"), []byte("core"), 0644); err != nil {
		t.Fatalf("write core name: %v", err)
	}

	dram := filepath.Join(pkg0, "intel-rapl:0:1")
	if err := os.MkdirAll(dram, 0755); err != nil {
		t.Fatalf("mkdir dram: %v", err)
	}
	if err := writeEnergyFile(filepath.Join(dram, "energy_uj"), "300000"); err != nil {
		t.Fatalf("write dram energy: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dram, "name"), []byte("dram"), 0644); err != nil {
		t.Fatalf("write dram name: %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		time.Sleep(20 * time.Millisecond)
		if err := writeEnergyFile(filepath.Join(pkg0, "energy_uj"), "2000000"); err != nil {
			errCh <- err
			return
		}
		if err := writeEnergyFile(filepath.Join(core, "energy_uj"), "400000"); err != nil {
			errCh <- err
			return
		}
		if err := writeEnergyFile(filepath.Join(dram, "energy_uj"), "600000"); err != nil {
			errCh <- err
			return
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	data, err := collectRALP(ctx)
	if err != nil {
		t.Fatalf("collectRALP error: %v", err)
	}
	select {
	case err := <-errCh:
		t.Fatalf("update energy files: %v", err)
	default:
	}
	if !data.Available {
		t.Fatalf("expected data.Available true")
	}
	if data.Source != "rapl" {
		t.Fatalf("expected source rapl, got %q", data.Source)
	}
	if data.PackageWatts <= 0 || data.CoreWatts <= 0 || data.DRAMWatts <= 0 {
		t.Fatalf("expected non-zero watts, got %+v", data)
	}
}

func TestCollectAMDEnergy_MockSysfs(t *testing.T) {
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
		t.Fatalf("write energy input: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hwmon, "energy1_label"), []byte("socket"), 0644); err != nil {
		t.Fatalf("write energy label: %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		time.Sleep(20 * time.Millisecond)
		if err := writeEnergyFile(filepath.Join(hwmon, "energy1_input"), "2000000"); err != nil {
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
		t.Fatalf("update energy input: %v", err)
	default:
	}
	if !data.Available {
		t.Fatalf("expected data.Available true")
	}
	if data.Source != "amd_energy" {
		t.Fatalf("expected source amd_energy, got %q", data.Source)
	}
	if data.PackageWatts <= 0 {
		t.Fatalf("expected package watts > 0, got %+v", data)
	}
}

func TestCollectPower_FallbackToAMD(t *testing.T) {
	originalRAPL := raplBasePath
	originalHwmon := hwmonBasePath
	tmpDir := t.TempDir()
	raplBasePath = filepath.Join(tmpDir, "missing-rapl")
	hwmonBasePath = filepath.Join(tmpDir, "hwmon")
	t.Cleanup(func() {
		raplBasePath = originalRAPL
		hwmonBasePath = originalHwmon
	})

	if err := os.MkdirAll(hwmonBasePath, 0755); err != nil {
		t.Fatalf("mkdir hwmon base: %v", err)
	}
	hwmon := filepath.Join(hwmonBasePath, "hwmon0")
	if err := os.MkdirAll(hwmon, 0755); err != nil {
		t.Fatalf("mkdir hwmon: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hwmon, "name"), []byte("amd_energy"), 0644); err != nil {
		t.Fatalf("write hwmon name: %v", err)
	}
	if err := writeEnergyFile(filepath.Join(hwmon, "energy1_input"), "1000000"); err != nil {
		t.Fatalf("write energy input: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hwmon, "energy1_label"), []byte("package"), 0644); err != nil {
		t.Fatalf("write energy label: %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		time.Sleep(20 * time.Millisecond)
		if err := writeEnergyFile(filepath.Join(hwmon, "energy1_input"), "2000000"); err != nil {
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
		t.Fatalf("update energy input: %v", err)
	default:
	}
	if data.Source != "amd_energy" {
		t.Fatalf("expected amd_energy fallback, got %q", data.Source)
	}
}

func TestCollectRALP_CanceledDuringSampleWait(t *testing.T) {
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
	if err := os.WriteFile(filepath.Join(pkg0, "name"), []byte("package-0"), 0644); err != nil {
		t.Fatalf("write name: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	data, err := collectRALP(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled error, got %v", err)
	}
	if data != nil {
		t.Fatalf("expected nil data on cancellation, got %+v", data)
	}
	if elapsed := time.Since(start); elapsed >= sampleInterval {
		t.Fatalf("expected cancellation before sample interval elapsed, took %v", elapsed)
	}
}

func TestCollectAMDEnergy_CanceledDuringSampleWait(t *testing.T) {
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
		t.Fatalf("write energy input: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hwmon, "energy1_label"), []byte("socket"), 0644); err != nil {
		t.Fatalf("write energy label: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	data, err := collectAMDEnergy(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled error, got %v", err)
	}
	if data != nil {
		t.Fatalf("expected nil data on cancellation, got %+v", data)
	}
	if elapsed := time.Since(start); elapsed >= sampleInterval {
		t.Fatalf("expected cancellation before sample interval elapsed, took %v", elapsed)
	}
}
