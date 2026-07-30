package hostagent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
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
	maxCustomSensorLabelBytes   = 64
	maxCustomSensors            = 32
	maxConcurrentCustomSensors  = 4
	defaultCustomSensorInterval = 5 * time.Minute
	defaultCustomSensorTimeout  = 3 * time.Second
	minCustomSensorInterval     = 10 * time.Second
	maxCustomSensorInterval     = 24 * time.Hour
	minCustomSensorTimeout      = 100 * time.Millisecond
	maxCustomSensorTimeout      = 10 * time.Second
	maxCustomSensorFutureSkew   = 5 * time.Minute
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
	Group         string   `yaml:"group,omitempty"`
	Subgroup      string   `yaml:"subgroup,omitempty"`
	Kind          string   `yaml:"kind,omitempty"`
	Command       string   `yaml:"command,omitempty"`
	URL           string   `yaml:"url,omitempty"`
	Unit          string   `yaml:"unit,omitempty"`
	Interval      string   `yaml:"interval,omitempty"`
	Timeout       string   `yaml:"timeout,omitempty"`
	StaleAfter    string   `yaml:"staleAfter,omitempty"`
	WarningAbove  *float64 `yaml:"warningAbove,omitempty"`
	CriticalAbove *float64 `yaml:"criticalAbove,omitempty"`
	WarningBelow  *float64 `yaml:"warningBelow,omitempty"`
	CriticalBelow *float64 `yaml:"criticalBelow,omitempty"`
	AlertOnError  *bool    `yaml:"alertOnError,omitempty"`
}

type customSensorDefinition struct {
	ID            string
	Name          string
	Group         string
	Subgroup      string
	Kind          string
	Command       string
	URL           string
	Unit          string
	Interval      time.Duration
	Timeout       time.Duration
	StaleAfter    time.Duration
	WarningAbove  *float64
	CriticalAbove *float64
	WarningBelow  *float64
	CriticalBelow *float64
	AlertOnError  bool
}

type customSensorExecution func(context.Context, string) (string, error)
type customSensorHTTPFetch func(context.Context, string) (customSensorSourceReading, error)

type customSensorSourceReading struct {
	output     string
	observedAt time.Time
}

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
	fetchHTTP   customSensorHTTPFetch
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

	group, err := validateCustomSensorLabel(entry.Group, "group")
	if err != nil {
		return customSensorDefinition{}, err
	}
	subgroup, err := validateCustomSensorLabel(entry.Subgroup, "subgroup")
	if err != nil {
		return customSensorDefinition{}, err
	}
	kind := strings.ToLower(strings.TrimSpace(entry.Kind))
	if kind == "" {
		kind = agentshost.CustomSensorKindNumber
	}
	switch kind {
	case agentshost.CustomSensorKindNumber, agentshost.CustomSensorKindBoolean, agentshost.CustomSensorKindTimestamp:
	default:
		return customSensorDefinition{}, errors.New("kind must be number, boolean, or timestamp")
	}

	command := strings.TrimSpace(entry.Command)
	sourceURL := strings.TrimSpace(entry.URL)
	if (command == "") == (sourceURL == "") {
		return customSensorDefinition{}, errors.New("exactly one of command or url must be configured")
	}
	if command != "" {
		command = filepath.Clean(command)
		if !filepath.IsAbs(command) {
			return customSensorDefinition{}, errors.New("command must be an absolute path")
		}
		if err := validateCustomSensorCommandFile(command); err != nil {
			return customSensorDefinition{}, err
		}
	}
	if sourceURL != "" {
		normalizedURL, err := validateCustomSensorURL(sourceURL)
		if err != nil {
			return customSensorDefinition{}, err
		}
		sourceURL = normalizedURL
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
	staleAfter, err := parseOptionalCustomSensorDuration(entry.StaleAfter, minCustomSensorInterval, 30*24*time.Hour, "staleAfter")
	if err != nil {
		return customSensorDefinition{}, err
	}
	if staleAfter > 0 && sourceURL == "" {
		return customSensorDefinition{}, errors.New("staleAfter requires an HTTP url source with observedAt data")
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
		Group:         group,
		Subgroup:      subgroup,
		Kind:          kind,
		Command:       command,
		URL:           sourceURL,
		Unit:          unit,
		Interval:      interval,
		Timeout:       timeout,
		StaleAfter:    staleAfter,
		WarningAbove:  cloneFloat64(entry.WarningAbove),
		CriticalAbove: cloneFloat64(entry.CriticalAbove),
		WarningBelow:  cloneFloat64(entry.WarningBelow),
		CriticalBelow: cloneFloat64(entry.CriticalBelow),
		AlertOnError:  alertOnError,
	}, nil
}

func validateCustomSensorLabel(raw, label string) (string, error) {
	value := strings.TrimSpace(raw)
	if len(value) > maxCustomSensorLabelBytes || strings.IndexFunc(value, func(character rune) bool {
		return character == '\n' || character == '\r' || !unicode.IsPrint(character)
	}) >= 0 {
		return "", fmt.Errorf("%s must be at most %d printable characters on one line", label, maxCustomSensorLabelBytes)
	}
	return value, nil
}

func validateCustomSensorURL(raw string) (string, error) {
	parsed, err := url.ParseRequestURI(raw)
	if err != nil {
		return "", errors.New("url must be a valid absolute HTTP(S) URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("url scheme must be http or https")
	}
	if parsed.Hostname() == "" || parsed.User != nil || parsed.Fragment != "" {
		return "", errors.New("url must include a host and must not include credentials or a fragment")
	}
	return parsed.String(), nil
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

func parseOptionalCustomSensorDuration(raw string, minimum, maximum time.Duration, label string) (time.Duration, error) {
	if strings.TrimSpace(raw) == "" {
		return 0, nil
	}
	return parseCustomSensorDuration(raw, 0, minimum, maximum, label)
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
		fetchHTTP:   fetchCustomSensorHTTP,
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
	ctx, cancel := context.WithTimeout(parent, definition.Timeout)
	defer cancel()

	sourceReading := customSensorSourceReading{observedAt: observedAt}
	var err error
	if definition.Command != "" {
		if err := validateCustomSensorCommandFile(definition.Command); err != nil {
			return customSensorErrorReading(definition, previous, observedAt, errors.New("custom sensor command failed security validation"))
		}
		sourceReading.output, err = runtimeState.execute(ctx, definition.Command)
	} else {
		sourceReading, err = runtimeState.fetchHTTP(ctx, definition.URL)
		if sourceReading.observedAt.IsZero() {
			sourceReading.observedAt = observedAt
		}
	}
	if err != nil {
		return customSensorErrorReading(definition, previous, observedAt, err)
	}
	if sourceReading.observedAt.After(observedAt.Add(maxCustomSensorFutureSkew)) {
		return customSensorErrorReading(definition, previous, observedAt, errors.New("custom sensor source observation time is in the future"))
	}
	value, eventAt, err := parseCustomSensorValueForKind(sourceReading.output, definition.Kind, observedAt)
	if err != nil {
		return customSensorErrorReading(definition, previous, observedAt, err)
	}
	if definition.StaleAfter > 0 && observedAt.Sub(sourceReading.observedAt) > definition.StaleAfter {
		reading := customSensorMetric(definition, &value, agentshost.CustomSensorStatusError, sourceReading.observedAt)
		reading.EventAt = eventAt
		reading.Stale = true
		reading.Error = fmt.Sprintf("source data is stale; last observed at %s", sourceReading.observedAt.UTC().Format(time.RFC3339))
		return reading
	}
	status := customSensorStatus(definition, value)
	reading := customSensorMetric(definition, &value, status, sourceReading.observedAt)
	reading.EventAt = eventAt
	return reading
}

func customSensorMetric(definition customSensorDefinition, value *float64, status string, observedAt time.Time) agentshost.CustomSensorMetric {
	return agentshost.CustomSensorMetric{
		ID:           definition.ID,
		Name:         definition.Name,
		Group:        definition.Group,
		Subgroup:     definition.Subgroup,
		Kind:         definition.Kind,
		Unit:         definition.Unit,
		Value:        value,
		Status:       status,
		ObservedAt:   observedAt,
		AlertOnError: definition.AlertOnError,
	}
}

func customSensorErrorReading(definition customSensorDefinition, previous *agentshost.CustomSensorMetric, observedAt time.Time, err error) agentshost.CustomSensorMetric {
	reading := customSensorMetric(definition, nil, agentshost.CustomSensorStatusError, observedAt)
	reading.Error = boundedCustomSensorError(err)
	if previous != nil && previous.Value != nil {
		value := *previous.Value
		reading.Value = &value
		reading.ObservedAt = previous.ObservedAt
		reading.EventAt = cloneTime(previous.EventAt)
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

func parseCustomSensorValueForKind(output, kind string, now time.Time) (float64, *time.Time, error) {
	switch kind {
	case agentshost.CustomSensorKindBoolean:
		switch strings.ToLower(strings.TrimSpace(output)) {
		case "1", "true", "yes", "on", "up", "online", "healthy":
			return 1, nil, nil
		case "0", "false", "no", "off", "down", "offline", "unhealthy":
			return 0, nil, nil
		default:
			return 0, nil, errors.New("boolean custom sensor output must be true/false, 1/0, yes/no, on/off, up/down, or online/offline")
		}
	case agentshost.CustomSensorKindTimestamp:
		eventAt, err := parseCustomSensorTimestamp(output)
		if err != nil {
			return 0, nil, err
		}
		if eventAt.After(now.Add(maxCustomSensorFutureSkew)) {
			return 0, nil, errors.New("timestamp custom sensor event time is in the future")
		}
		age := now.Sub(eventAt).Seconds()
		if age < 0 {
			age = 0
		}
		return age, &eventAt, nil
	default:
		value, err := parseCustomSensorValue(output)
		return value, nil, err
	}
}

func parseCustomSensorTimestamp(output string) (time.Time, error) {
	raw := strings.TrimSpace(output)
	if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
		return parsed.UTC(), nil
	}
	seconds, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return time.Time{}, errors.New("timestamp custom sensor output must be RFC3339 or Unix seconds")
	}
	return time.Unix(seconds, 0).UTC(), nil
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

type customSensorHTTPResponse struct {
	Value      any    `json:"value"`
	ObservedAt string `json:"observedAt,omitempty"`
}

func fetchCustomSensorHTTP(ctx context.Context, sourceURL string) (customSensorSourceReading, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return customSensorSourceReading{}, errors.New("create custom sensor HTTP request")
	}
	request.Header.Set("Accept", "application/json, text/plain")
	client := &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	response, err := client.Do(request)
	if err != nil {
		if contextError := ctx.Err(); contextError != nil {
			return customSensorSourceReading{}, fmt.Errorf("custom sensor HTTP request: %w", contextError)
		}
		return customSensorSourceReading{}, errors.New("custom sensor HTTP request failed")
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return customSensorSourceReading{}, fmt.Errorf("custom sensor HTTP endpoint returned status %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxCustomSensorOutputBytes+1))
	if err != nil {
		return customSensorSourceReading{}, errors.New("read custom sensor HTTP response")
	}
	if len(body) > maxCustomSensorOutputBytes {
		return customSensorSourceReading{}, fmt.Errorf("custom sensor HTTP response exceeds %d bytes", maxCustomSensorOutputBytes)
	}
	raw := strings.TrimSpace(string(body))
	reading := customSensorSourceReading{output: raw, observedAt: time.Now().UTC()}
	if !strings.HasPrefix(raw, "{") {
		return reading, nil
	}
	var payload customSensorHTTPResponse
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return customSensorSourceReading{}, errors.New("custom sensor HTTP response must be a scalar or JSON object with value")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return customSensorSourceReading{}, errors.New("custom sensor HTTP response must contain exactly one JSON value")
	}
	switch value := payload.Value.(type) {
	case json.Number:
		reading.output = value.String()
	case string:
		reading.output = value
	case bool:
		reading.output = strconv.FormatBool(value)
	default:
		return customSensorSourceReading{}, errors.New("custom sensor HTTP JSON value must be a number, string, or boolean")
	}
	if strings.TrimSpace(payload.ObservedAt) != "" {
		parsed, err := time.Parse(time.RFC3339, payload.ObservedAt)
		if err != nil {
			return customSensorSourceReading{}, errors.New("custom sensor HTTP observedAt must be RFC3339")
		}
		reading.observedAt = parsed.UTC()
	}
	return reading, nil
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
	metric.EventAt = cloneTime(metric.EventAt)
	return metric
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
