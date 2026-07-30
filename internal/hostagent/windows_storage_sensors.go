package hostagent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	"github.com/rcourtman/pulse-go-rewrite/pkg/diskinventory"
)

const (
	windowsStorageQueryTimeout   = 5 * time.Second
	windowsStorageMaxOutputBytes = 64 * 1024
	windowsStorageMaxDisks       = 128
	windowsStorageSource         = "windows-storage-reliability"
)

// The script is fixed agent code, not operator or server input. It asks the
// built-in Windows Storage module for only the bounded fields Pulse reports.
const windowsStorageTemperatureScript = `$ErrorActionPreference = 'Stop'
$rows = @(
  Get-PhysicalDisk -ErrorAction Stop |
    Select-Object -First 128 |
    ForEach-Object {
      $disk = $_
      $counter = $disk | Get-StorageReliabilityCounter -ErrorAction SilentlyContinue
      if ($null -ne $counter -and $null -ne $counter.Temperature) {
        $name = [string]$disk.FriendlyName
        if ($name.Length -gt 128) { $name = $name.Substring(0, 128) }
        $deviceId = [string]$disk.DeviceId
        if ($deviceId.Length -gt 32) { $deviceId = $deviceId.Substring(0, 32) }
        [pscustomobject]@{
          deviceId = $deviceId
          friendlyName = $name
          busType = [string]$disk.BusType
          mediaType = [string]$disk.MediaType
          sizeBytes = [uint64]$disk.Size
          temperature = [double]$counter.Temperature
        }
      }
    }
)
ConvertTo-Json -InputObject $rows -Compress`

type windowsStorageTemperatureReading struct {
	DeviceID     string   `json:"deviceId"`
	FriendlyName string   `json:"friendlyName"`
	BusType      string   `json:"busType"`
	MediaType    string   `json:"mediaType"`
	SizeBytes    *int64   `json:"sizeBytes"`
	Temperature  *float64 `json:"temperature"`
}

func (a *Agent) collectWindowsTemperatureSensors(ctx context.Context) agentshost.Sensors {
	result := a.collectWindowsStorageTemperatures(ctx)
	a.mergeLibreHardwareMonitorTemperatures(ctx, &result)
	a.mergeNVIDIATemperatures(ctx, &result)
	return result
}

func (a *Agent) collectWindowsStorageTemperatures(ctx context.Context) agentshost.Sensors {
	powerShellPath, err := a.resolveWindowsPowerShell()
	if err != nil {
		if !errors.Is(err, exec.ErrNotFound) && !os.IsNotExist(err) {
			a.logger.Debug().Err(err).Msg("Failed to locate Windows PowerShell for storage temperatures")
		}
		return agentshost.Sensors{}
	}

	queryCtx, cancel := context.WithTimeout(ctx, windowsStorageQueryTimeout)
	defer cancel()

	output, err := a.collector.CommandCombinedOutput(
		queryCtx,
		powerShellPath,
		"-NoLogo",
		"-NoProfile",
		"-NonInteractive",
		"-Command",
		windowsStorageTemperatureScript,
	)
	if err != nil {
		a.logger.Debug().Err(err).Msg("Failed to collect Windows storage reliability temperatures")
		return agentshost.Sensors{}
	}

	disks, err := parseWindowsStorageTemperatures(output)
	if err != nil {
		a.logger.Debug().Err(err).Msg("Failed to parse Windows storage reliability temperatures")
		return agentshost.Sensors{}
	}
	if len(disks) == 0 {
		return agentshost.Sensors{}
	}

	a.logger.Debug().
		Int("diskCount", len(disks)).
		Msg("Collected Windows storage reliability temperatures")
	return agentshost.Sensors{SMART: disks}
}

func (a *Agent) resolveWindowsPowerShell() (string, error) {
	var lastErr error
	for _, candidate := range []string{"powershell.exe", "powershell"} {
		path, err := a.collector.LookPath(candidate)
		if err == nil {
			return path, nil
		}
		lastErr = err
		if !errors.Is(err, exec.ErrNotFound) && !os.IsNotExist(err) {
			return "", fmt.Errorf("locate %s: %w", candidate, err)
		}
	}
	if lastErr == nil {
		lastErr = exec.ErrNotFound
	}
	return "", lastErr
}

func parseWindowsStorageTemperatures(output string) ([]agentshost.DiskSMART, error) {
	if len(output) > windowsStorageMaxOutputBytes {
		return nil, fmt.Errorf(
			"Windows storage reliability output exceeds %d bytes",
			windowsStorageMaxOutputBytes,
		)
	}

	output = strings.TrimSpace(strings.TrimPrefix(output, "\ufeff"))
	if output == "" || output == "null" {
		return nil, nil
	}

	var readings []windowsStorageTemperatureReading
	if err := json.Unmarshal([]byte(output), &readings); err != nil {
		var single windowsStorageTemperatureReading
		if singleErr := json.Unmarshal([]byte(output), &single); singleErr != nil {
			return nil, err
		}
		readings = []windowsStorageTemperatureReading{single}
	}
	if len(readings) > windowsStorageMaxDisks {
		readings = readings[:windowsStorageMaxDisks]
	}

	result := make([]agentshost.DiskSMART, 0, len(readings))
	seen := make(map[string]struct{}, len(readings))
	for index, reading := range readings {
		if reading.Temperature == nil ||
			math.IsNaN(*reading.Temperature) ||
			math.IsInf(*reading.Temperature, 0) ||
			*reading.Temperature <= 0 ||
			*reading.Temperature > 150 {
			continue
		}

		deviceID := normalizeWindowsStorageDeviceID(reading.DeviceID, index)
		device := "PhysicalDisk" + deviceID
		key := strings.ToLower(device)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}

		sizeBytes := int64(0)
		if reading.SizeBytes != nil && *reading.SizeBytes > 0 {
			sizeBytes = *reading.SizeBytes
		}
		result = append(result, agentshost.DiskSMART{
			Device:      device,
			Model:       truncateWindowsStorageLabel(reading.FriendlyName, 128),
			Type:        normalizeWindowsStorageType(reading.BusType, reading.MediaType),
			SizeBytes:   sizeBytes,
			Temperature: int(math.Round(*reading.Temperature)),
			Health:      "UNKNOWN",
			Collection: &diskinventory.CollectionStatus{
				Temperature: diskinventory.Available(windowsStorageSource),
			},
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Device < result[j].Device
	})
	if len(result) == 0 {
		return nil, nil
	}
	return result, nil
}

func normalizeWindowsStorageDeviceID(value string, fallback int) string {
	value = truncateWindowsStorageLabel(strings.TrimSpace(value), 32)
	var normalized strings.Builder
	normalized.Grow(len(value))
	for _, r := range value {
		if (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == '-' ||
			r == '_' ||
			r == '.' {
			normalized.WriteRune(r)
		} else {
			normalized.WriteByte('_')
		}
	}
	result := strings.Trim(normalized.String(), "_.-")
	if result == "" {
		return strconv.Itoa(fallback)
	}
	return result
}

func normalizeWindowsStorageType(busType, mediaType string) string {
	switch normalized := strings.ToLower(strings.TrimSpace(busType)); normalized {
	case "nvme", "sata", "sas", "usb":
		return normalized
	}
	switch normalized := strings.ToLower(strings.TrimSpace(mediaType)); normalized {
	case "ssd", "hdd", "scm":
		return normalized
	default:
		return ""
	}
}

func truncateWindowsStorageLabel(value string, maxRunes int) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes])
}
