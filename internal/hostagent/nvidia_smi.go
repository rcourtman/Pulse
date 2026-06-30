package hostagent

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

const nvidiaSMITemperatureTimeout = 5 * time.Second

var nvidiaSMITemperatureArgs = []string{
	"--query-gpu=index,name,temperature.gpu",
	"--format=csv,noheader,nounits",
}

func (a *Agent) collectNVIDIATemperatureSensors(ctx context.Context) agentshost.Sensors {
	var result agentshost.Sensors
	a.mergeNVIDIATemperatures(ctx, &result)
	if len(result.TemperatureCelsius) == 0 {
		return agentshost.Sensors{}
	}
	return result
}

func (a *Agent) mergeNVIDIATemperatures(ctx context.Context, result *agentshost.Sensors) {
	temps, err := a.queryNVIDIASMITemperatures(ctx)
	if err != nil {
		a.logger.Debug().Err(err).Msg("Failed to collect NVIDIA GPU temperatures")
		return
	}
	if len(temps) == 0 {
		return
	}

	if result.TemperatureCelsius == nil {
		result.TemperatureCelsius = make(map[string]float64, len(temps))
	}
	for key, temp := range temps {
		result.TemperatureCelsius[key] = temp
	}
}

func (a *Agent) queryNVIDIASMITemperatures(ctx context.Context) (map[string]float64, error) {
	path, err := a.collector.LookPath("nvidia-smi")
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) || os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("locate nvidia-smi: %w", err)
	}

	cmdCtx, cancel := context.WithTimeout(ctx, nvidiaSMITemperatureTimeout)
	defer cancel()

	output, err := a.collector.CommandCombinedOutput(cmdCtx, path, nvidiaSMITemperatureArgs...)
	if err != nil {
		return nil, fmt.Errorf("execute nvidia-smi temperature query: %w", err)
	}

	temps, err := parseNVIDIASMITemperatures(output)
	if err != nil {
		return nil, fmt.Errorf("parse nvidia-smi temperature output: %w", err)
	}
	return temps, nil
}

func parseNVIDIASMITemperatures(output string) (map[string]float64, error) {
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

	temps := make(map[string]float64)
	for row, record := range records {
		if len(record) < 3 {
			continue
		}

		temp, ok := parseNVIDIASMITemperatureValue(record[2])
		if !ok {
			continue
		}

		index := normalizeNVIDIASMIIndex(record[0], row)
		key := "gpu_nvidia_" + index
		if _, exists := temps[key]; exists {
			key = fmt.Sprintf("gpu_nvidia_%d", row)
		}
		temps[key] = temp
	}

	if len(temps) == 0 {
		return nil, nil
	}
	return temps, nil
}

func parseNVIDIASMITemperatureValue(value string) (float64, bool) {
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
	value = strings.TrimSuffix(strings.TrimSuffix(value, "C"), "c")

	temp, err := strconv.ParseFloat(value, 64)
	if err != nil || temp <= 0 {
		return 0, false
	}
	return temp, true
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
