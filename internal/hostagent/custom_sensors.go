package hostagent

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	"gopkg.in/yaml.v3"
)

const (
	customSensorConfigVersion   = 1
	maxCustomSensorConfigBytes  = 64 * 1024
	maxCustomSensorOutputBytes  = 4 * 1024
	maxCustomSensorErrorBytes   = 256
	maxCustomSensors            = 32
	maxConcurrentCustomSensors  = 4
	defaultCustomSensorInterval = 5 * time.Minute
	defaultCustomSensorTimeout  = 3 * time.Second
	minCustomSensorInterval     = 10 * time.Second
	maxCustomSensorInterval     = 24 * time.Hour
	minCustomSensorTimeout      = 100 * time.Millisecond
	maxCustomSensorTimeout      = 10 * time.Second
)

var (
	customSensorIDPattern   = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)
	customSensorUnitPattern = regexp.MustCompile(`^[A-Za-z0-9%°/_(). -]{0,24}$`)
)

type customSensorFile struct {
	Version int                     `yaml:"version"`
	Sensors []customSensorFileEntry `yaml:"sensors"`
}

type customSensorFileEntry struct {
	ID            string   `yaml:"id"`
	Name          string   `yaml:"name"`
	Command       string   `yaml:"command"`
	Unit          string   `yaml:"unit,omitempty"`
	Interval      string   `yaml:"interval,omitempty"`
	Timeout       string   `yaml:"timeout,omitempty"`
	WarningAbove  *float64 `yaml:"warningAbove,omitempty"`
	CriticalAbove *float64 `yaml:"criticalAbove,omitempty"`
	WarningBelow  *float64 `yaml:"warningBelow,omitempty"`
	CriticalBelow *float64 `yaml:"criticalBelow,omitempty"`
	AlertOnError  *bool    `yaml:"alertOnError,omitempty"`
}

type customSensorDefinition struct {
	ID            string
	Name          string
	Command       string
	Unit          string
	Interval      time.Duration
	Timeout       time.Duration
	WarningAbove  *float64
	CriticalAbove *float64
	WarningBelow  *float64
	CriticalBelow *float64
	AlertOnError  bool
}

type customSensorExecution func(context.Context, string) (string, error)

type customSensorRuntimeState struct {
	reading agentshost.CustomSensorMetric
	nextRun time.Time
	running bool
}

type customSensorRuntime struct {
	mu          sync.Mutex
	definitions []customSensorDefinition
	states      map[string]customSensorRuntimeState
	execute     customSensorExecution
	now         func() time.Time
}

func loadCustomSensorDefinitions(path string) ([]customSensorDefinition, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, nil
	}
	if !filepath.IsAbs(path) {
		return nil, errors.New("custom sensor config path must be absolute")
	}
	if err := validateCustomSensorConfigFile(path); err != nil {
		return nil, err
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open custom sensor config: %w", err)
	}
	defer file.Close()

	decoder := yaml.NewDecoder(io.LimitReader(file, maxCustomSensorConfigBytes+1))
	decoder.KnownFields(true)
	var parsed customSensorFile
	if err := decoder.Decode(&parsed); err != nil {
		return nil, fmt.Errorf("decode custom sensor config: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, errors.New("custom sensor config must contain exactly one YAML document")
		}
		return nil, fmt.Errorf("decode custom sensor config trailer: %w", err)
	}
	if parsed.Version != customSensorConfigVersion {
		return nil, fmt.Errorf("custom sensor config version %d is unsupported; expected %d", parsed.Version, customSensorConfigVersion)
	}
	if len(parsed.Sensors) == 0 {
		return nil, errors.New("custom sensor config must define at least one sensor")
	}
	if len(parsed.Sensors) > maxCustomSensors {
		return nil, fmt.Errorf("custom sensor config has %d sensors; maximum is %d", len(parsed.Sensors), maxCustomSensors)
	}

	definitions := make([]customSensorDefinition, 0, len(parsed.Sensors))
	seen := make(map[string]struct{}, len(parsed.Sensors))
	for index, entry := range parsed.Sensors {
		definition, err := resolveCustomSensorDefinition(entry)
		if err != nil {
			return nil, fmt.Errorf("custom sensor %d: %w", index+1, err)
		}
		if _, exists := seen[definition.ID]; exists {
			return nil, fmt.Errorf("custom sensor %d: duplicate id %q", index+1, definition.ID)
		}
		seen[definition.ID] = struct{}{}
		definitions = append(definitions, definition)
	}
	return definitions, nil
}

func resolveCustomSensorDefinition(entry customSensorFileEntry) (customSensorDefinition, error) {
	id := strings.TrimSpace(entry.ID)
	if !customSensorIDPattern.MatchString(id) {
		return customSensorDefinition{}, errors.New("id must start with a lowercase letter and contain only lowercase letters, digits, underscores, or hyphens (maximum 64 characters)")
	}
	name := strings.TrimSpace(entry.Name)
	if name == "" || len(name) > 100 || strings.IndexFunc(name, func(character rune) bool {
		return !unicode.IsPrint(character)
	}) >= 0 {
		return customSensorDefinition{}, errors.New("name must be 1-100 printable characters on one line")
	}
	unit := strings.TrimSpace(entry.Unit)
	if !customSensorUnitPattern.MatchString(unit) {
		return customSensorDefinition{}, errors.New("unit contains unsupported characters or exceeds 24 characters")
	}

	command := filepath.Clean(strings.TrimSpace(entry.Command))
	if !filepath.IsAbs(command) {
		return customSensorDefinition{}, errors.New("command must be an absolute path")
	}
	if err := validateCustomSensorCommandFile(command); err != nil {
		return customSensorDefinition{}, err
	}

	interval, err := parseCustomSensorDuration(entry.Interval, defaultCustomSensorInterval, minCustomSensorInterval, maxCustomSensorInterval, "interval")
	if err != nil {
		return customSensorDefinition{}, err
	}
	timeout, err := parseCustomSensorDuration(entry.Timeout, defaultCustomSensorTimeout, minCustomSensorTimeout, maxCustomSensorTimeout, "timeout")
	if err != nil {
		return customSensorDefinition{}, err
	}
	if timeout >= interval {
		return customSensorDefinition{}, errors.New("timeout must be shorter than interval")
	}
	for label, value := range map[string]*float64{
		"warningAbove":  entry.WarningAbove,
		"criticalAbove": entry.CriticalAbove,
		"warningBelow":  entry.WarningBelow,
		"criticalBelow": entry.CriticalBelow,
	} {
		if value != nil && (math.IsNaN(*value) || math.IsInf(*value, 0)) {
			return customSensorDefinition{}, fmt.Errorf("%s must be finite", label)
		}
	}
	if entry.WarningAbove != nil && entry.CriticalAbove != nil && *entry.WarningAbove > *entry.CriticalAbove {
		return customSensorDefinition{}, errors.New("warningAbove must be less than or equal to criticalAbove")
	}
	if entry.WarningBelow != nil && entry.CriticalBelow != nil && *entry.WarningBelow < *entry.CriticalBelow {
		return customSensorDefinition{}, errors.New("warningBelow must be greater than or equal to criticalBelow")
	}
	lowerBoundary := entry.WarningBelow
	if lowerBoundary == nil {
		lowerBoundary = entry.CriticalBelow
	}
	upperBoundary := entry.WarningAbove
	if upperBoundary == nil {
		upperBoundary = entry.CriticalAbove
	}
	if lowerBoundary != nil && upperBoundary != nil && *lowerBoundary >= *upperBoundary {
		return customSensorDefinition{}, errors.New("lower thresholds must be less than upper thresholds")
	}

	alertOnError := true
	if entry.AlertOnError != nil {
		alertOnError = *entry.AlertOnError
	}
	return customSensorDefinition{
		ID:            id,
		Name:          name,
		Command:       command,
		Unit:          unit,
		Interval:      interval,
		Timeout:       timeout,
		WarningAbove:  cloneFloat64(entry.WarningAbove),
		CriticalAbove: cloneFloat64(entry.CriticalAbove),
		WarningBelow:  cloneFloat64(entry.WarningBelow),
		CriticalBelow: cloneFloat64(entry.CriticalBelow),
		AlertOnError:  alertOnError,
	}, nil
}

func parseCustomSensorDuration(raw string, fallback, minimum, maximum time.Duration, label string) (time.Duration, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s is invalid: %w", label, err)
	}
	if parsed < minimum || parsed > maximum {
		return 0, fmt.Errorf("%s must be between %s and %s", label, minimum, maximum)
	}
	return parsed, nil
}

func validateCustomSensorConfigFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("stat custom sensor config: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return errors.New("custom sensor config must be a regular file, not a symlink")
	}
	if info.Size() > maxCustomSensorConfigBytes {
		return fmt.Errorf("custom sensor config exceeds %d bytes", maxCustomSensorConfigBytes)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return errors.New("custom sensor config permissions must not grant group or other access")
	}
	if err := validateCustomSensorFileOwner(info, "custom sensor config"); err != nil {
		return err
	}
	return validateCustomSensorParentDirectory(filepath.Dir(path), "custom sensor config")
}

func validateCustomSensorCommandFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("stat custom sensor command: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return errors.New("custom sensor command must be a regular file, not a symlink")
	}
	if runtime.GOOS != "windows" {
		if info.Mode().Perm()&0o111 == 0 {
			return errors.New("custom sensor command must be executable")
		}
		if info.Mode().Perm()&0o022 != 0 {
			return errors.New("custom sensor command must not be writable by group or others")
		}
	}
	if err := validateCustomSensorFileOwner(info, "custom sensor command"); err != nil {
		return err
	}
	return validateCustomSensorParentDirectory(filepath.Dir(path), "custom sensor command")
}

func validateCustomSensorParentDirectory(path, label string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("stat %s parent directory: %w", label, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s parent directory must not be a symlink", label)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s parent path is not a directory", label)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o022 != 0 {
		return fmt.Errorf("%s parent directory must not be writable by group or others", label)
	}
	return validateCustomSensorFileOwner(info, label+" parent directory")
}

func newCustomSensorRuntime(definitions []customSensorDefinition, execute customSensorExecution) *customSensorRuntime {
	if len(definitions) == 0 {
		return nil
	}
	if execute == nil {
		execute = executeCustomSensorCommand
	}
	return &customSensorRuntime{
		definitions: append([]customSensorDefinition(nil), definitions...),
		states:      make(map[string]customSensorRuntimeState, len(definitions)),
		execute:     execute,
		now:         time.Now,
	}
}

func (runtimeState *customSensorRuntime) collect(ctx context.Context) []agentshost.CustomSensorMetric {
	if runtimeState == nil {
		return nil
	}

	now := runtimeState.now()
	type dueSensor struct {
		definition customSensorDefinition
		previous   *agentshost.CustomSensorMetric
	}
	due := make([]dueSensor, 0, len(runtimeState.definitions))

	runtimeState.mu.Lock()
	for _, definition := range runtimeState.definitions {
		state := runtimeState.states[definition.ID]
		if !state.running && (state.nextRun.IsZero() || !now.Before(state.nextRun)) {
			state.running = true
			state.nextRun = now.Add(definition.Interval)
			runtimeState.states[definition.ID] = state
			var previous *agentshost.CustomSensorMetric
			if state.reading.ID != "" {
				cloned := cloneCustomSensorMetric(state.reading)
				previous = &cloned
			}
			due = append(due, dueSensor{definition: definition, previous: previous})
		}
	}
	runtimeState.mu.Unlock()

	type executionResult struct {
		definition customSensorDefinition
		reading    agentshost.CustomSensorMetric
	}
	results := make(chan executionResult, len(due))
	semaphore := make(chan struct{}, maxConcurrentCustomSensors)
	var wait sync.WaitGroup
	for _, item := range due {
		wait.Add(1)
		go func(definition customSensorDefinition, previous *agentshost.CustomSensorMetric) {
			defer wait.Done()
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				results <- executionResult{definition: definition, reading: customSensorErrorReading(definition, previous, now, ctx.Err())}
				return
			}
			results <- executionResult{
				definition: definition,
				reading:    runtimeState.executeOne(ctx, definition, previous, now),
			}
		}(item.definition, item.previous)
	}
	wait.Wait()
	close(results)

	runtimeState.mu.Lock()
	for result := range results {
		state := runtimeState.states[result.definition.ID]
		state.reading = result.reading
		state.running = false
		runtimeState.states[result.definition.ID] = state
	}
	readings := make([]agentshost.CustomSensorMetric, 0, len(runtimeState.definitions))
	for _, definition := range runtimeState.definitions {
		state := runtimeState.states[definition.ID]
		if state.reading.ID != "" {
			readings = append(readings, cloneCustomSensorMetric(state.reading))
		}
	}
	runtimeState.mu.Unlock()
	return readings
}

func (runtimeState *customSensorRuntime) executeOne(parent context.Context, definition customSensorDefinition, previous *agentshost.CustomSensorMetric, observedAt time.Time) agentshost.CustomSensorMetric {
	if err := validateCustomSensorCommandFile(definition.Command); err != nil {
		return customSensorErrorReading(definition, previous, observedAt, errors.New("custom sensor command failed security validation"))
	}
	ctx, cancel := context.WithTimeout(parent, definition.Timeout)
	defer cancel()

	output, err := runtimeState.execute(ctx, definition.Command)
	if err != nil {
		return customSensorErrorReading(definition, previous, observedAt, err)
	}
	value, err := parseCustomSensorValue(output)
	if err != nil {
		return customSensorErrorReading(definition, previous, observedAt, err)
	}
	status := customSensorStatus(definition, value)
	return agentshost.CustomSensorMetric{
		ID:           definition.ID,
		Name:         definition.Name,
		Unit:         definition.Unit,
		Value:        &value,
		Status:       status,
		ObservedAt:   observedAt,
		AlertOnError: definition.AlertOnError,
	}
}

func customSensorErrorReading(definition customSensorDefinition, previous *agentshost.CustomSensorMetric, observedAt time.Time, err error) agentshost.CustomSensorMetric {
	reading := agentshost.CustomSensorMetric{
		ID:           definition.ID,
		Name:         definition.Name,
		Unit:         definition.Unit,
		Status:       agentshost.CustomSensorStatusError,
		ObservedAt:   observedAt,
		AlertOnError: definition.AlertOnError,
		Error:        boundedCustomSensorError(err),
	}
	if previous != nil && previous.Value != nil {
		value := *previous.Value
		reading.Value = &value
		reading.ObservedAt = previous.ObservedAt
		reading.Stale = true
	}
	return reading
}

func parseCustomSensorValue(output string) (float64, error) {
	output = strings.TrimSpace(output)
	if output == "" {
		return 0, errors.New("custom sensor returned empty output")
	}
	value, err := strconv.ParseFloat(output, 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, errors.New("custom sensor output must be exactly one finite number")
	}
	return value, nil
}

func customSensorStatus(definition customSensorDefinition, value float64) string {
	if definition.CriticalAbove != nil && value >= *definition.CriticalAbove {
		return agentshost.CustomSensorStatusCritical
	}
	if definition.CriticalBelow != nil && value <= *definition.CriticalBelow {
		return agentshost.CustomSensorStatusCritical
	}
	if definition.WarningAbove != nil && value >= *definition.WarningAbove {
		return agentshost.CustomSensorStatusWarning
	}
	if definition.WarningBelow != nil && value <= *definition.WarningBelow {
		return agentshost.CustomSensorStatusWarning
	}
	return agentshost.CustomSensorStatusOK
}

func executeCustomSensorCommand(ctx context.Context, path string) (string, error) {
	var stdout boundedSensorBuffer
	stdout.limit = maxCustomSensorOutputBytes

	command := exec.CommandContext(ctx, path)
	command.Stdout = &stdout
	command.WaitDelay = 500 * time.Millisecond
	err := command.Run()
	if stdout.truncated {
		return "", fmt.Errorf("custom sensor output exceeded %d bytes", maxCustomSensorOutputBytes)
	}
	if err != nil {
		if contextError := ctx.Err(); contextError != nil {
			return "", fmt.Errorf("custom sensor command: %w", contextError)
		}
		return "", fmt.Errorf("custom sensor command failed: %w", err)
	}
	return stdout.String(), nil
}

type boundedSensorBuffer struct {
	buffer    bytes.Buffer
	limit     int
	truncated bool
}

func (buffer *boundedSensorBuffer) Write(data []byte) (int, error) {
	originalLength := len(data)
	remaining := buffer.limit - buffer.buffer.Len()
	if remaining <= 0 {
		buffer.truncated = true
		return originalLength, nil
	}
	if len(data) > remaining {
		data = data[:remaining]
		buffer.truncated = true
	}
	_, _ = buffer.buffer.Write(data)
	return originalLength, nil
}

func (buffer *boundedSensorBuffer) String() string {
	return buffer.buffer.String()
}

func boundedCustomSensorError(err error) string {
	if err == nil {
		return ""
	}
	message := strings.TrimSpace(strings.ReplaceAll(err.Error(), "\x00", ""))
	if len(message) > maxCustomSensorErrorBytes {
		message = message[:maxCustomSensorErrorBytes]
	}
	return message
}

func cloneFloat64(value *float64) *float64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneCustomSensorMetric(metric agentshost.CustomSensorMetric) agentshost.CustomSensorMetric {
	metric.Value = cloneFloat64(metric.Value)
	return metric
}
