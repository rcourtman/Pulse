package sensors

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// PowerData contains power consumption readings from the system.
type PowerData struct {
	// PackageWatts is the CPU package power consumption in watts.
	// This is the total power for the CPU socket (all cores + uncore).
	PackageWatts float64

	// CoreWatts is the CPU cores-only power consumption in watts (if available).
	CoreWatts float64

	// DRAMWatts is the DRAM power consumption in watts (if available).
	// Note: Not all platforms support DRAM power measurement.
	DRAMWatts float64

	// Available indicates whether any power data was successfully collected.
	Available bool

	// Source indicates the method used: "rapl", "amd_energy", or empty.
	Source string
}

// raplBasePath is the base path for Intel RAPL (Running Average Power Limit) readings.
// RAPL provides energy counters that we sample to calculate power.
var raplBasePath = "/sys/class/powercap/intel-rapl"

// sampleInterval is the time between energy counter readings.
// Shorter intervals are less accurate; longer intervals add latency.
const sampleInterval = 100 * time.Millisecond

// CollectPower reads power consumption data from the system.
// Supports Intel RAPL and AMD energy driver.
// Returns nil if no power data is available.
func CollectPower(ctx context.Context) (*PowerData, error) {
	ctx = normalizeCollectionContext(ctx)

	// Try Intel RAPL first (most common on Intel)
	if data, err := collectRALP(ctx); err == nil && data.Available {
		return data, nil
	}

	// Try AMD energy driver (for AMD Ryzen/EPYC)
	if data, err := collectAMDEnergy(ctx); err == nil && data.Available {
		return data, nil
	}

	// TODO: Add IPMI support for server BMCs

	return nil, fmt.Errorf("no power monitoring available")
}

// collectRALP reads power data from Intel RAPL sysfs interface.
// RAPL provides energy counters in microjoules that we sample twice
// to calculate instantaneous power in watts.
func collectRALP(ctx context.Context) (*PowerData, error) {
	ctx = normalizeCollectionContext(ctx)

	// Check if RAPL is available
	if _, err := os.Stat(raplBasePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("RAPL not available: %w", err)
	}

	data := &PowerData{Source: "rapl"}

	// Find all RAPL domains (packages)
	// Typically: intel-rapl:0 (package), intel-rapl:0:0 (core), intel-rapl:0:1 (uncore), etc.
	packages, err := filepath.Glob(filepath.Join(raplBasePath, "intel-rapl:*"))
	if err != nil || len(packages) == 0 {
		return nil, fmt.Errorf("no RAPL packages found")
	}

	// Sample energy counters
	sample1, err := readRAPLEnergy(packages)
	if err != nil {
		return nil, fmt.Errorf("first RAPL sample failed: %w", err)
	}

	// Wait for sample interval
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(sampleInterval):
	}

	// Second sample
	sample2, err := readRAPLEnergy(packages)
	if err != nil {
		return nil, fmt.Errorf("second RAPL sample failed: %w", err)
	}

	// Calculate power from energy delta
	// Power (W) = Energy delta (J) / Time delta (s)
	duration := sampleInterval.Seconds()

	for domain, energy1 := range sample1 {
		energy2, ok := sample2[domain]
		if !ok {
			continue
		}

		// Handle counter wraparound (energy counters are typically 32-bit)
		var deltaUJ uint64
		if energy2 >= energy1 {
			deltaUJ = energy2 - energy1
		} else {
			// Counter wrapped around
			deltaUJ = (^uint64(0) - energy1) + energy2 + 1
		}

		// Convert microjoules to watts
		watts := float64(deltaUJ) / 1e6 / duration

		// Categorize by domain name
		domainLower := strings.ToLower(domain)
		switch {
		case strings.Contains(domainLower, "package") || strings.HasSuffix(domain, ":0"):
			// Package-level (total CPU socket power)
			data.PackageWatts += watts
		case strings.Contains(domainLower, "core"):
			data.CoreWatts += watts
		case strings.Contains(domainLower, "dram"):
			data.DRAMWatts += watts
		}

		data.Available = true
	}

	if data.Available {
		log.Debug().
			Float64("packageWatts", data.PackageWatts).
			Float64("coreWatts", data.CoreWatts).
			Float64("dramWatts", data.DRAMWatts).
			Msg("Collected RAPL power data")
	}

	return data, nil
}

// readRAPLEnergy reads energy counters from all RAPL domains.
// Returns a map of domain name -> energy in microjoules.
func readRAPLEnergy(packages []string) (map[string]uint64, error) {
	result := make(map[string]uint64)

	for _, pkgPath := range packages {
		// Read the package energy
		energyPath := filepath.Join(pkgPath, "energy_uj")
		if energy, err := readUint64File(energyPath); err == nil {
			name := filepath.Base(pkgPath)
			// Also read the domain name if available
			namePath := filepath.Join(pkgPath, "name")
			if domainName, err := readStringFile(namePath); err == nil {
				name = domainName
			}
			result[name] = energy
		}

		// Also read subdomain energy (core, uncore, dram)
		subdomains, _ := filepath.Glob(filepath.Join(pkgPath, "intel-rapl:*"))
		for _, subPath := range subdomains {
			energyPath := filepath.Join(subPath, "energy_uj")
			if energy, err := readUint64File(energyPath); err == nil {
				name := filepath.Base(subPath)
				// Read subdomain name
				namePath := filepath.Join(subPath, "name")
				if domainName, err := readStringFile(namePath); err == nil {
					name = domainName
				}
				result[name] = energy
			}
		}
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no RAPL energy readings available")
	}

	return result, nil
}

// readUint64File reads a file containing a single uint64 value.
func readUint64File(path string) (uint64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
}

// readStringFile reads a file containing a single string value.
func readStringFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// hwmonBasePath is the base path for hwmon devices (used by AMD energy driver).
var hwmonBasePath = "/sys/class/hwmon"

// collectAMDEnergy reads power data from AMD energy driver via hwmon.
// The amd_energy module exposes energy counters similar to Intel RAPL.
func collectAMDEnergy(ctx context.Context) (*PowerData, error) {
	ctx = normalizeCollectionContext(ctx)

	// Find hwmon device with amd_energy driver
	hwmonPath, err := findAMDEnergyHwmon()
	if err != nil {
		return nil, err
	}

	data := &PowerData{Source: "amd_energy"}

	// Sample energy counters
	sample1, err := readAMDEnergy(hwmonPath)
	if err != nil {
		return nil, fmt.Errorf("first AMD energy sample failed: %w", err)
	}

	// Wait for sample interval
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(sampleInterval):
	}

	// Second sample
	sample2, err := readAMDEnergy(hwmonPath)
	if err != nil {
		return nil, fmt.Errorf("second AMD energy sample failed: %w", err)
	}

	// Calculate power from energy delta
	duration := sampleInterval.Seconds()

	for label, energy1 := range sample1 {
		energy2, ok := sample2[label]
		if !ok {
			continue
		}

		// Handle counter wraparound
		var deltaUJ uint64
		if energy2 >= energy1 {
			deltaUJ = energy2 - energy1
		} else {
			deltaUJ = (^uint64(0) - energy1) + energy2 + 1
		}

		// Convert microjoules to watts
		watts := float64(deltaUJ) / 1e6 / duration

		// Categorize by label
		labelLower := strings.ToLower(label)
		switch {
		case strings.Contains(labelLower, "socket") || strings.Contains(labelLower, "package"):
			data.PackageWatts += watts
		case strings.Contains(labelLower, "core"):
			data.CoreWatts += watts
		default:
			// Default to package power for unlabeled readings
			if data.PackageWatts == 0 {
				data.PackageWatts = watts
			}
		}

		data.Available = true
	}

	if data.Available {
		log.Debug().
			Float64("packageWatts", data.PackageWatts).
			Float64("coreWatts", data.CoreWatts).
			Msg("Collected AMD energy power data")
	}

	return data, nil
}

// findAMDEnergyHwmon finds the hwmon device path for amd_energy driver.
func findAMDEnergyHwmon() (string, error) {
	entries, err := os.ReadDir(hwmonBasePath)
	if err != nil {
		return "", fmt.Errorf("cannot read hwmon: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		hwmonDir := filepath.Join(hwmonBasePath, entry.Name())
		namePath := filepath.Join(hwmonDir, "name")

		name, err := readStringFile(namePath)
		if err != nil {
			continue
		}

		if name == "amd_energy" {
			return hwmonDir, nil
		}
	}

	return "", fmt.Errorf("amd_energy hwmon device not found")
}

// readAMDEnergy reads energy counters from AMD energy hwmon device.
// Returns a map of label -> energy in microjoules.
func readAMDEnergy(hwmonPath string) (map[string]uint64, error) {
	result := make(map[string]uint64)

	// AMD energy exposes energy*_input files (in microjoules)
	energyFiles, err := filepath.Glob(filepath.Join(hwmonPath, "energy*_input"))
	if err != nil || len(energyFiles) == 0 {
		return nil, fmt.Errorf("no AMD energy files found")
	}

	for _, energyPath := range energyFiles {
		energy, err := readUint64File(energyPath)
		if err != nil {
			continue
		}

		// Try to get the label for this energy reading
		// energy1_input -> energy1_label
		labelPath := strings.Replace(energyPath, "_input", "_label", 1)
		label, err := readStringFile(labelPath)
		if err != nil {
			// Use filename as fallback
			label = filepath.Base(energyPath)
		}

		result[label] = energy
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no AMD energy readings available")
	}

	return result, nil
}
