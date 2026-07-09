package hostagent

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

const (
	nvidiaSMIQueryTimeout = 5 * time.Second
	bytesPerMiB           = 1024 * 1024
)

var nvidiaSMIStatsArgs = []string{
	"--query-gpu=index,name,temperature.gpu,utilization.gpu,memory.used,memory.total",
	"--format=csv,noheader,nounits",
}

func (a *Agent) collectNVIDIATemperatureSensors(ctx context.Context) agentshost.Sensors {
	var result agentshost.Sensors
	a.mergeNVIDIASMIStats(ctx, &result)
	if len(result.TemperatureCelsius) == 0 && len(result.GPU) == 0 {
		return agentshost.Sensors{}
	}
	return result
}

func (a *Agent) mergeNVIDIATemperatures(ctx context.Context, result *agentshost.Sensors) {
	a.mergeNVIDIASMIStats(ctx, result)
}

func (a *Agent) mergeNVIDIASMIStats(ctx context.Context, result *agentshost.Sensors) {
	gpus, err := a.queryNVIDIASMIStats(ctx)
	if err != nil {
		a.logger.Debug().Err(err).Msg("Failed to collect NVIDIA GPU stats")
		return
	}
	if len(gpus) == 0 {
		return
	}

	result.GPU = mergeNVIDIAGPUStats(result.GPU, gpus)
	for _, gpu := range gpus {
		if gpu.TemperatureCelsius == nil || *gpu.TemperatureCelsius <= 0 {
			continue
		}
		if result.TemperatureCelsius == nil {
			result.TemperatureCelsius = make(map[string]float64, len(gpus))
		}
		result.TemperatureCelsius["gpu_nvidia_"+gpu.ID] = *gpu.TemperatureCelsius
	}
}

func (a *Agent) queryNVIDIASMITemperatures(ctx context.Context) (map[string]float64, error) {
	gpus, err := a.queryNVIDIASMIStats(ctx)
	if err != nil {
		return nil, err
	}
	temps := make(map[string]float64)
	for _, gpu := range gpus {
		if gpu.TemperatureCelsius == nil || *gpu.TemperatureCelsius <= 0 {
			continue
		}
		temps["gpu_nvidia_"+gpu.ID] = *gpu.TemperatureCelsius
	}
	if len(temps) == 0 {
		return nil, nil
	}
	return temps, nil
}

func (a *Agent) queryNVIDIASMIStats(ctx context.Context) ([]agentshost.GPUSensor, error) {
	path, err := a.collector.LookPath("nvidia-smi")
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) || os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("locate nvidia-smi: %w", err)
	}

	cmdCtx, cancel := context.WithTimeout(ctx, nvidiaSMIQueryTimeout)
	defer cancel()

	output, err := a.collector.CommandCombinedOutput(cmdCtx, path, nvidiaSMIStatsArgs...)
	if err != nil {
		return nil, fmt.Errorf("execute nvidia-smi stats query: %w", err)
	}

	gpus, err := parseNVIDIASMIStats(output)
	if err != nil {
		return nil, fmt.Errorf("parse nvidia-smi stats output: %w", err)
	}
	return gpus, nil
}

func parseNVIDIASMITemperatures(output string) (map[string]float64, error) {
	gpus, err := parseNVIDIASMIStats(output)
	if err != nil {
		return nil, err
	}

	temps := make(map[string]float64)
	for _, gpu := range gpus {
		if gpu.TemperatureCelsius == nil || *gpu.TemperatureCelsius <= 0 {
			continue
		}
		temps["gpu_nvidia_"+gpu.ID] = *gpu.TemperatureCelsius
	}

	if len(temps) == 0 {
		return nil, nil
	}
	return temps, nil
}

func parseNVIDIASMIStats(output string) ([]agentshost.GPUSensor, error) {
	output = strings.TrimSpace(output)
	if output == "" {
		return nil, nil
	}

	reader := csv.NewReader(strings.NewReader(output))
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true

	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	gpus := make([]agentshost.GPUSensor, 0, len(records))
	usedIDs := make(map[string]struct{}, len(records))
	for row, record := range records {
		if len(record) < 3 {
			continue
		}

		id := normalizeNVIDIASMIIndex(record[0], row)
		if _, exists := usedIDs[id]; exists {
			id = strconv.Itoa(row)
		}
		usedIDs[id] = struct{}{}

		gpu := agentshost.GPUSensor{
			ID:   id,
			Name: strings.TrimSpace(record[1]),
		}
		if temp, ok := parseNVIDIASMINumber(record[2], false); ok {
			gpu.TemperatureCelsius = &temp
		}
		if len(record) > 3 {
			if utilization, ok := parseNVIDIASMINumber(record[3], true); ok {
				gpu.UtilizationPercent = &utilization
			}
		}
		if len(record) > 4 {
			if memoryUsedMiB, ok := parseNVIDIASMINumber(record[4], true); ok {
				memoryUsedBytes := miBToBytes(memoryUsedMiB)
				gpu.MemoryUsedBytes = &memoryUsedBytes
			}
		}
		if len(record) > 5 {
			if memoryTotalMiB, ok := parseNVIDIASMINumber(record[5], false); ok {
				memoryTotalBytes := miBToBytes(memoryTotalMiB)
				gpu.MemoryTotalBytes = &memoryTotalBytes
			}
		}

		if gpu.TemperatureCelsius == nil && gpu.UtilizationPercent == nil && gpu.MemoryUsedBytes == nil && gpu.MemoryTotalBytes == nil {
			continue
		}
		gpus = append(gpus, gpu)
	}

	if len(gpus) == 0 {
		return nil, nil
	}
	return gpus, nil
}

func parseNVIDIASMINumber(value string, allowZero bool) (float64, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}

	lower := strings.ToLower(value)
	if lower == "n/a" || lower == "na" || strings.Contains(lower, "not supported") || strings.Contains(lower, "unknown") {
		return 0, false
	}

	fields := strings.Fields(value)
	if len(fields) > 0 {
		value = fields[0]
	}
	value = strings.TrimSuffix(value, "%")
	value = strings.TrimSuffix(strings.TrimSuffix(value, "C"), "c")
	value = strings.TrimSuffix(strings.TrimSuffix(value, "MiB"), "Mib")

	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, false
	}
	if parsed < 0 || (!allowZero && parsed == 0) {
		return 0, false
	}
	return parsed, true
}

func miBToBytes(value float64) int64 {
	return int64(math.Round(value * bytesPerMiB))
}

func mergeNVIDIAGPUStats(existing []agentshost.GPUSensor, incoming []agentshost.GPUSensor) []agentshost.GPUSensor {
	if len(existing) == 0 {
		return append([]agentshost.GPUSensor(nil), incoming...)
	}

	result := append([]agentshost.GPUSensor(nil), existing...)
	indexByID := make(map[string]int, len(result))
	for i, gpu := range result {
		if gpu.ID == "" {
			continue
		}
		indexByID[gpu.ID] = i
	}
	for _, gpu := range incoming {
		if gpu.ID != "" {
			if i, ok := indexByID[gpu.ID]; ok {
				result[i] = gpu
				continue
			}
			indexByID[gpu.ID] = len(result)
		}
		result = append(result, gpu)
	}
	return result
}

func normalizeNVIDIASMIIndex(value string, fallback int) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return strconv.Itoa(fallback)
	}

	var builder strings.Builder
	lastUnderscore := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if builder.Len() > 0 && !lastUnderscore {
			builder.WriteByte('_')
			lastUnderscore = true
		}
	}

	normalized := strings.Trim(builder.String(), "_")
	if normalized == "" {
		return strconv.Itoa(fallback)
	}
	return normalized
}
