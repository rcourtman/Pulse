package hostagent

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

func writeCustomSensorFixture(t *testing.T, configBody string) (string, string) {
	t.Helper()
	directory := t.TempDir()
	if runtime.GOOS != "windows" {
		if err := os.Chmod(directory, 0o700); err != nil {
			t.Fatalf("secure custom sensor fixture directory: %v", err)
		}
	}
	commandPath := filepath.Join(directory, "sensor-command")
	if runtime.GOOS == "windows" {
		commandPath += ".exe"
	}
	if err := os.WriteFile(commandPath, []byte("#!/bin/sh\nprintf '1\\n'\n"), 0o700); err != nil {
		t.Fatalf("write command fixture: %v", err)
	}
	configPath := filepath.Join(directory, "custom-sensors.yaml")
	configBody = strings.ReplaceAll(configBody, "__COMMAND__", commandPath)
	if err := os.WriteFile(configPath, []byte(configBody), 0o600); err != nil {
		t.Fatalf("write config fixture: %v", err)
	}
	return configPath, commandPath
}

func TestLoadCustomSensorDefinitionsValidatesAndNormalizes(t *testing.T) {
	configPath, commandPath := writeCustomSensorFixture(t, `version: 1
sensors:
  - id: pending_updates
    name: Pending updates
    command: __COMMAND__
    unit: count
    interval: 10m
    timeout: 2s
    warningAbove: 5
    criticalAbove: 20
`)

	definitions, err := loadCustomSensorDefinitions(configPath)
	if err != nil {
		t.Fatalf("loadCustomSensorDefinitions: %v", err)
	}
	if len(definitions) != 1 {
		t.Fatalf("definition count = %d, want 1", len(definitions))
	}
	definition := definitions[0]
	if definition.ID != "pending_updates" || definition.Name != "Pending updates" || definition.Command != commandPath {
		t.Fatalf("unexpected definition: %+v", definition)
	}
	if definition.Interval != 10*time.Minute || definition.Timeout != 2*time.Second {
		t.Fatalf("unexpected timing: interval=%s timeout=%s", definition.Interval, definition.Timeout)
	}
	if definition.WarningAbove == nil || *definition.WarningAbove != 5 || definition.CriticalAbove == nil || *definition.CriticalAbove != 20 {
		t.Fatalf("unexpected thresholds: %+v", definition)
	}
	if !definition.AlertOnError {
		t.Fatal("AlertOnError should default to true")
	}
}

func TestLoadCustomSensorDefinitionsSupportsGroupedHTTPKinds(t *testing.T) {
	configPath, _ := writeCustomSensorFixture(t, `version: 1
sensors:
  - id: dns_update
    name: Main DNS update
    group: Main server
    subgroup: Domain
    kind: timestamp
    url: https://metrics.example.test/dns
    interval: 1m
    timeout: 2s
    staleAfter: 10m
    warningAbove: 3600
    criticalAbove: 7200
`)

	definitions, err := loadCustomSensorDefinitions(configPath)
	if err != nil {
		t.Fatalf("loadCustomSensorDefinitions: %v", err)
	}
	if len(definitions) != 1 {
		t.Fatalf("definition count = %d, want 1", len(definitions))
	}
	definition := definitions[0]
	if definition.Command != "" || definition.URL != "https://metrics.example.test/dns" {
		t.Fatalf("unexpected HTTP source: %+v", definition)
	}
	if definition.Group != "Main server" || definition.Subgroup != "Domain" || definition.Kind != agentshost.CustomSensorKindTimestamp {
		t.Fatalf("unexpected grouped kind: %+v", definition)
	}
	if definition.StaleAfter != 10*time.Minute {
		t.Fatalf("staleAfter = %s, want 10m", definition.StaleAfter)
	}
}

func TestLoadCustomSensorDefinitionsRejectsUnsafeAndAmbiguousConfiguration(t *testing.T) {
	t.Run("unknown field", func(t *testing.T) {
		configPath, _ := writeCustomSensorFixture(t, `version: 1
sensors:
  - id: metric
    name: Metric
    command: __COMMAND__
    shell: true
`)
		if _, err := loadCustomSensorDefinitions(configPath); err == nil || !strings.Contains(err.Error(), "field shell not found") {
			t.Fatalf("expected unknown field rejection, got %v", err)
		}
	})

	t.Run("relative command", func(t *testing.T) {
		configPath, _ := writeCustomSensorFixture(t, `version: 1
sensors:
  - id: metric
    name: Metric
    command: ./sensor
`)
		if _, err := loadCustomSensorDefinitions(configPath); err == nil || !strings.Contains(err.Error(), "absolute path") {
			t.Fatalf("expected relative command rejection, got %v", err)
		}
	})

	t.Run("ambiguous source", func(t *testing.T) {
		configPath, _ := writeCustomSensorFixture(t, `version: 1
sensors:
  - id: metric
    name: Metric
    command: __COMMAND__
    url: https://metrics.example.test/value
`)
		if _, err := loadCustomSensorDefinitions(configPath); err == nil || !strings.Contains(err.Error(), "exactly one") {
			t.Fatalf("expected ambiguous source rejection, got %v", err)
		}
	})

	t.Run("unsafe url", func(t *testing.T) {
		configPath, _ := writeCustomSensorFixture(t, `version: 1
sensors:
  - id: metric
    name: Metric
    url: file:///etc/passwd
`)
		if _, err := loadCustomSensorDefinitions(configPath); err == nil || !strings.Contains(err.Error(), "scheme") {
			t.Fatalf("expected URL scheme rejection, got %v", err)
		}
	})

	t.Run("control character in name", func(t *testing.T) {
		_, err := resolveCustomSensorDefinition(customSensorFileEntry{
			ID:      "metric",
			Name:    "unsafe\x01name",
			Command: commandPathForTest(t),
		})
		if err == nil || !strings.Contains(err.Error(), "printable") {
			t.Fatalf("expected control character rejection, got %v", err)
		}
	})

	t.Run("threshold ordering", func(t *testing.T) {
		configPath, _ := writeCustomSensorFixture(t, `version: 1
sensors:
  - id: metric
    name: Metric
    command: __COMMAND__
    warningAbove: 20
    criticalAbove: 10
`)
		if _, err := loadCustomSensorDefinitions(configPath); err == nil || !strings.Contains(err.Error(), "warningAbove") {
			t.Fatalf("expected threshold ordering rejection, got %v", err)
		}
	})

	t.Run("overlapping lower and upper thresholds", func(t *testing.T) {
		configPath, _ := writeCustomSensorFixture(t, `version: 1
sensors:
  - id: metric
    name: Metric
    command: __COMMAND__
    warningBelow: 20
    warningAbove: 10
`)
		if _, err := loadCustomSensorDefinitions(configPath); err == nil || !strings.Contains(err.Error(), "lower thresholds") {
			t.Fatalf("expected overlapping threshold rejection, got %v", err)
		}
	})

	if runtime.GOOS != "windows" {
		t.Run("loose config permissions", func(t *testing.T) {
			configPath, _ := writeCustomSensorFixture(t, `version: 1
sensors:
  - id: metric
    name: Metric
    command: __COMMAND__
`)
			if err := os.Chmod(configPath, 0o644); err != nil {
				t.Fatalf("chmod config: %v", err)
			}
			if _, err := loadCustomSensorDefinitions(configPath); err == nil || !strings.Contains(err.Error(), "permissions") {
				t.Fatalf("expected loose permission rejection, got %v", err)
			}
		})

		t.Run("symlinked config parent", func(t *testing.T) {
			_, commandPath := writeCustomSensorFixture(t, `version: 1
sensors:
  - id: metric
    name: Metric
    command: __COMMAND__
`)
			root := t.TempDir()
			realDirectory := filepath.Join(root, "real")
			if err := os.Mkdir(realDirectory, 0o700); err != nil {
				t.Fatalf("mkdir real config parent: %v", err)
			}
			linkDirectory := filepath.Join(root, "linked")
			if err := os.Symlink(realDirectory, linkDirectory); err != nil {
				t.Fatalf("symlink config parent: %v", err)
			}
			configPath := filepath.Join(linkDirectory, "custom-sensors.yaml")
			configBody := "version: 1\nsensors:\n  - id: metric\n    name: Metric\n    command: " + commandPath + "\n"
			if err := os.WriteFile(configPath, []byte(configBody), 0o600); err != nil {
				t.Fatalf("write config: %v", err)
			}
			if _, err := loadCustomSensorDefinitions(configPath); err == nil || !strings.Contains(err.Error(), "must not be a symlink") {
				t.Fatalf("expected symlinked parent rejection, got %v", err)
			}
		})
	}
}

func commandPathForTest(t *testing.T) string {
	t.Helper()
	_, commandPath := writeCustomSensorFixture(t, `version: 1
sensors:
  - id: metric
    name: Metric
    command: __COMMAND__
`)
	return commandPath
}

func TestCustomSensorRuntimeCachesAndReportsThresholdAndErrorState(t *testing.T) {
	_, commandPath := writeCustomSensorFixture(t, `version: 1
sensors:
  - id: queue_depth
    name: Queue depth
    command: __COMMAND__
`)
	warning := 5.0
	critical := 20.0
	definition := customSensorDefinition{
		ID:            "queue_depth",
		Name:          "Queue depth",
		Command:       commandPath,
		Unit:          "count",
		Interval:      time.Minute,
		Timeout:       time.Second,
		WarningAbove:  &warning,
		CriticalAbove: &critical,
		AlertOnError:  true,
	}

	var calls atomic.Int32
	execute := func(context.Context, string) (string, error) {
		switch calls.Add(1) {
		case 1:
			return "7\n", nil
		default:
			return "", errors.New("probe failed with secret detail that remains bounded")
		}
	}
	current := time.Date(2026, 7, 30, 19, 0, 0, 0, time.UTC)
	customRuntime := newCustomSensorRuntime([]customSensorDefinition{definition}, execute)
	customRuntime.now = func() time.Time { return current }

	first := customRuntime.collect(context.Background())
	if len(first) != 1 || first[0].Value == nil || *first[0].Value != 7 {
		t.Fatalf("unexpected first reading: %+v", first)
	}
	if first[0].Status != agentshost.CustomSensorStatusWarning || first[0].Stale {
		t.Fatalf("unexpected first status: %+v", first[0])
	}

	cached := customRuntime.collect(context.Background())
	if calls.Load() != 1 || len(cached) != 1 || cached[0].Value == nil || *cached[0].Value != 7 {
		t.Fatalf("cached read should not execute again: calls=%d reading=%+v", calls.Load(), cached)
	}

	current = current.Add(time.Minute)
	failed := customRuntime.collect(context.Background())
	if calls.Load() != 2 || len(failed) != 1 {
		t.Fatalf("expected second execution: calls=%d reading=%+v", calls.Load(), failed)
	}
	if failed[0].Status != agentshost.CustomSensorStatusError || !failed[0].Stale || failed[0].Value == nil || *failed[0].Value != 7 {
		t.Fatalf("expected stale last-good value with error state: %+v", failed[0])
	}
	if !failed[0].ObservedAt.Equal(first[0].ObservedAt) {
		t.Fatalf("stale reading observedAt = %s, want last-good time %s", failed[0].ObservedAt, first[0].ObservedAt)
	}
	if !strings.Contains(failed[0].Error, "probe failed") {
		t.Fatalf("expected bounded execution error, got %q", failed[0].Error)
	}
}

func TestParseCustomSensorValueAndStatus(t *testing.T) {
	for _, invalid := range []string{"", "NaN", "+Inf", "1 2", "healthy"} {
		if _, err := parseCustomSensorValue(invalid); err == nil {
			t.Fatalf("parseCustomSensorValue(%q) unexpectedly succeeded", invalid)
		}
	}
	if value, err := parseCustomSensorValue(" 0.125\n"); err != nil || value != 0.125 {
		t.Fatalf("parseCustomSensorValue valid result = %v, %v", value, err)
	}

	warningBelow := 10.0
	criticalBelow := 5.0
	definition := customSensorDefinition{WarningBelow: &warningBelow, CriticalBelow: &criticalBelow}
	if got := customSensorStatus(definition, 4); got != agentshost.CustomSensorStatusCritical {
		t.Fatalf("critical-below status = %q", got)
	}
	if got := customSensorStatus(definition, 8); got != agentshost.CustomSensorStatusWarning {
		t.Fatalf("warning-below status = %q", got)
	}
	if got := customSensorStatus(definition, 11); got != agentshost.CustomSensorStatusOK {
		t.Fatalf("healthy status = %q", got)
	}
}

func TestCustomSensorKindsAndHTTPFreshness(t *testing.T) {
	now := time.Date(2026, 7, 30, 20, 0, 0, 0, time.UTC)
	booleanValue, eventAt, err := parseCustomSensorValueForKind("offline", agentshost.CustomSensorKindBoolean, now)
	if err != nil || booleanValue != 0 || eventAt != nil {
		t.Fatalf("boolean parse = %v, %v, %v", booleanValue, eventAt, err)
	}
	age, eventAt, err := parseCustomSensorValueForKind("2026-07-30T18:30:00Z", agentshost.CustomSensorKindTimestamp, now)
	if err != nil || age != 90*time.Minute.Seconds() || eventAt == nil || !eventAt.Equal(now.Add(-90*time.Minute)) {
		t.Fatalf("timestamp parse = %v, %v, %v", age, eventAt, err)
	}

	criticalBelow := 0.5
	definition := customSensorDefinition{
		ID:            "service",
		Name:          "Service online",
		Group:         "Main server",
		Subgroup:      "Services",
		Kind:          agentshost.CustomSensorKindBoolean,
		URL:           "https://metrics.example.test/service",
		Interval:      time.Minute,
		Timeout:       time.Second,
		StaleAfter:    5 * time.Minute,
		CriticalBelow: &criticalBelow,
		AlertOnError:  true,
	}
	customRuntime := newCustomSensorRuntime([]customSensorDefinition{definition}, nil)
	customRuntime.now = func() time.Time { return now }
	customRuntime.fetchHTTP = func(context.Context, string) (customSensorSourceReading, error) {
		return customSensorSourceReading{output: "true", observedAt: now.Add(-10 * time.Minute)}, nil
	}
	readings := customRuntime.collect(context.Background())
	if len(readings) != 1 || readings[0].Status != agentshost.CustomSensorStatusError || !readings[0].Stale {
		t.Fatalf("expected stale HTTP reading: %+v", readings)
	}
	if readings[0].Group != "Main server" || readings[0].Subgroup != "Services" || readings[0].Value == nil || *readings[0].Value != 1 {
		t.Fatalf("HTTP reading metadata/value not preserved: %+v", readings[0])
	}
}

func TestFetchCustomSensorHTTPAcceptsBoundedScalarAndJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/json" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"value":false,"observedAt":"2026-07-30T19:00:00Z"}`))
			return
		}
		if request.URL.Path == "/redirect" {
			http.Redirect(w, request, "https://example.test/secret", http.StatusFound)
			return
		}
		if request.URL.Path == "/large" {
			_, _ = w.Write([]byte(strings.Repeat("1", maxCustomSensorOutputBytes+1)))
			return
		}
		_, _ = w.Write([]byte("42.5\n"))
	}))
	defer server.Close()

	scalar, err := fetchCustomSensorHTTP(context.Background(), server.URL+"/scalar")
	if err != nil || scalar.output != "42.5" {
		t.Fatalf("scalar HTTP reading = %+v, %v", scalar, err)
	}
	jsonReading, err := fetchCustomSensorHTTP(context.Background(), server.URL+"/json")
	if err != nil || jsonReading.output != "false" || !jsonReading.observedAt.Equal(time.Date(2026, 7, 30, 19, 0, 0, 0, time.UTC)) {
		t.Fatalf("JSON HTTP reading = %+v, %v", jsonReading, err)
	}
	if _, err := fetchCustomSensorHTTP(context.Background(), server.URL+"/redirect"); err == nil || !strings.Contains(err.Error(), "status 302") {
		t.Fatalf("expected redirect rejection, got %v", err)
	}
	if _, err := fetchCustomSensorHTTP(context.Background(), server.URL+"/large"); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("expected oversized response rejection, got %v", err)
	}
}

func TestExecuteCustomSensorCommandDoesNotExposeStderr(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture is POSIX-only")
	}
	commandPath := filepath.Join(t.TempDir(), "failing-sensor")
	if err := os.WriteFile(commandPath, []byte("#!/bin/sh\nprintf 'database-password' >&2\nexit 1\n"), 0o700); err != nil {
		t.Fatalf("write command: %v", err)
	}

	_, err := executeCustomSensorCommand(context.Background(), commandPath)
	if err == nil {
		t.Fatal("expected command failure")
	}
	if strings.Contains(err.Error(), "database-password") {
		t.Fatalf("stderr escaped the local execution boundary: %q", err)
	}
}
