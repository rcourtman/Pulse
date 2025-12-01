package monitoring

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestParseDurationEnv(t *testing.T) {
	const testKey = "TEST_DURATION_ENV"
	defaultVal := 30 * time.Second

	t.Run("empty env var returns default", func(t *testing.T) {
		t.Setenv(testKey, "")
		result := parseDurationEnv(testKey, defaultVal)
		if result != defaultVal {
			t.Errorf("expected %v, got %v", defaultVal, result)
		}
	})

	t.Run("unset env var returns default", func(t *testing.T) {
		// t.Setenv automatically cleans up, so not setting means unset
		result := parseDurationEnv("UNSET_DURATION_KEY_12345", defaultVal)
		if result != defaultVal {
			t.Errorf("expected %v, got %v", defaultVal, result)
		}
	})

	t.Run("valid duration seconds", func(t *testing.T) {
		t.Setenv(testKey, "1s")
		result := parseDurationEnv(testKey, defaultVal)
		expected := 1 * time.Second
		if result != expected {
			t.Errorf("expected %v, got %v", expected, result)
		}
	})

	t.Run("valid duration minutes", func(t *testing.T) {
		t.Setenv(testKey, "5m")
		result := parseDurationEnv(testKey, defaultVal)
		expected := 5 * time.Minute
		if result != expected {
			t.Errorf("expected %v, got %v", expected, result)
		}
	})

	t.Run("valid duration composite", func(t *testing.T) {
		t.Setenv(testKey, "2h30m")
		result := parseDurationEnv(testKey, defaultVal)
		expected := 2*time.Hour + 30*time.Minute
		if result != expected {
			t.Errorf("expected %v, got %v", expected, result)
		}
	})

	t.Run("valid duration milliseconds", func(t *testing.T) {
		t.Setenv(testKey, "500ms")
		result := parseDurationEnv(testKey, defaultVal)
		expected := 500 * time.Millisecond
		if result != expected {
			t.Errorf("expected %v, got %v", expected, result)
		}
	})

	t.Run("invalid duration returns default", func(t *testing.T) {
		t.Setenv(testKey, "invalid")
		result := parseDurationEnv(testKey, defaultVal)
		if result != defaultVal {
			t.Errorf("expected default %v, got %v", defaultVal, result)
		}
	})

	t.Run("numeric without unit returns default", func(t *testing.T) {
		t.Setenv(testKey, "100")
		result := parseDurationEnv(testKey, defaultVal)
		if result != defaultVal {
			t.Errorf("expected default %v, got %v", defaultVal, result)
		}
	})

	t.Run("negative duration parses correctly", func(t *testing.T) {
		t.Setenv(testKey, "-5s")
		result := parseDurationEnv(testKey, defaultVal)
		expected := -5 * time.Second
		if result != expected {
			t.Errorf("expected %v, got %v", expected, result)
		}
	})
}

func TestParseIntEnv(t *testing.T) {
	const testKey = "TEST_INT_ENV"
	defaultVal := 42

	t.Run("empty env var returns default", func(t *testing.T) {
		t.Setenv(testKey, "")
		result := parseIntEnv(testKey, defaultVal)
		if result != defaultVal {
			t.Errorf("expected %d, got %d", defaultVal, result)
		}
	})

	t.Run("unset env var returns default", func(t *testing.T) {
		result := parseIntEnv("UNSET_INT_KEY_12345", defaultVal)
		if result != defaultVal {
			t.Errorf("expected %d, got %d", defaultVal, result)
		}
	})

	t.Run("valid positive integer", func(t *testing.T) {
		t.Setenv(testKey, "100")
		result := parseIntEnv(testKey, defaultVal)
		if result != 100 {
			t.Errorf("expected 100, got %d", result)
		}
	})

	t.Run("valid zero", func(t *testing.T) {
		t.Setenv(testKey, "0")
		result := parseIntEnv(testKey, defaultVal)
		if result != 0 {
			t.Errorf("expected 0, got %d", result)
		}
	})

	t.Run("valid negative integer", func(t *testing.T) {
		t.Setenv(testKey, "-50")
		result := parseIntEnv(testKey, defaultVal)
		if result != -50 {
			t.Errorf("expected -50, got %d", result)
		}
	})

	t.Run("invalid string returns default", func(t *testing.T) {
		t.Setenv(testKey, "not-a-number")
		result := parseIntEnv(testKey, defaultVal)
		if result != defaultVal {
			t.Errorf("expected default %d, got %d", defaultVal, result)
		}
	})

	t.Run("float returns default", func(t *testing.T) {
		t.Setenv(testKey, "3.14")
		result := parseIntEnv(testKey, defaultVal)
		if result != defaultVal {
			t.Errorf("expected default %d, got %d", defaultVal, result)
		}
	})

	t.Run("whitespace returns default", func(t *testing.T) {
		t.Setenv(testKey, "  ")
		result := parseIntEnv(testKey, defaultVal)
		if result != defaultVal {
			t.Errorf("expected default %d, got %d", defaultVal, result)
		}
	})

	t.Run("number with trailing text returns default", func(t *testing.T) {
		t.Setenv(testKey, "100abc")
		result := parseIntEnv(testKey, defaultVal)
		if result != defaultVal {
			t.Errorf("expected default %d, got %d", defaultVal, result)
		}
	})
}

func TestGetInstanceConfig(t *testing.T) {
	t.Run("nil Monitor returns nil", func(t *testing.T) {
		var m *Monitor
		result := m.getInstanceConfig("any")
		if result != nil {
			t.Errorf("expected nil, got %+v", result)
		}
	})

	t.Run("nil config returns nil", func(t *testing.T) {
		m := &Monitor{config: nil}
		result := m.getInstanceConfig("any")
		if result != nil {
			t.Errorf("expected nil, got %+v", result)
		}
	})

	t.Run("empty PVEInstances slice returns nil", func(t *testing.T) {
		m := &Monitor{
			config: &config.Config{
				PVEInstances: []config.PVEInstance{},
			},
		}
		result := m.getInstanceConfig("any")
		if result != nil {
			t.Errorf("expected nil, got %+v", result)
		}
	})

	t.Run("instance found by exact name match", func(t *testing.T) {
		m := &Monitor{
			config: &config.Config{
				PVEInstances: []config.PVEInstance{
					{Name: "pve1", Host: "192.168.1.1"},
				},
			},
		}
		result := m.getInstanceConfig("pve1")
		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if result.Name != "pve1" {
			t.Errorf("expected Name 'pve1', got '%s'", result.Name)
		}
		if result.Host != "192.168.1.1" {
			t.Errorf("expected Host '192.168.1.1', got '%s'", result.Host)
		}
	})

	t.Run("instance found by case-insensitive match", func(t *testing.T) {
		m := &Monitor{
			config: &config.Config{
				PVEInstances: []config.PVEInstance{
					{Name: "pve1", Host: "192.168.1.1"},
				},
			},
		}
		result := m.getInstanceConfig("PVE1")
		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if result.Name != "pve1" {
			t.Errorf("expected Name 'pve1', got '%s'", result.Name)
		}
	})

	t.Run("instance not found returns nil", func(t *testing.T) {
		m := &Monitor{
			config: &config.Config{
				PVEInstances: []config.PVEInstance{
					{Name: "pve1", Host: "192.168.1.1"},
				},
			},
		}
		result := m.getInstanceConfig("nonexistent")
		if result != nil {
			t.Errorf("expected nil, got %+v", result)
		}
	})

	t.Run("multiple instances finds correct one", func(t *testing.T) {
		m := &Monitor{
			config: &config.Config{
				PVEInstances: []config.PVEInstance{
					{Name: "pve1", Host: "192.168.1.1"},
					{Name: "pve2", Host: "192.168.1.2"},
					{Name: "pve3", Host: "192.168.1.3"},
				},
			},
		}
		result := m.getInstanceConfig("pve2")
		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if result.Name != "pve2" {
			t.Errorf("expected Name 'pve2', got '%s'", result.Name)
		}
		if result.Host != "192.168.1.2" {
			t.Errorf("expected Host '192.168.1.2', got '%s'", result.Host)
		}
	})
}

func TestBaseIntervalForInstanceType(t *testing.T) {
	defaultInterval := DefaultSchedulerConfig().BaseInterval

	t.Run("nil Monitor returns default", func(t *testing.T) {
		var m *Monitor
		result := m.baseIntervalForInstanceType(InstanceTypePVE)
		if result != defaultInterval {
			t.Errorf("expected %v, got %v", defaultInterval, result)
		}
	})

	t.Run("nil config returns default", func(t *testing.T) {
		m := &Monitor{config: nil}
		result := m.baseIntervalForInstanceType(InstanceTypePVE)
		if result != defaultInterval {
			t.Errorf("expected %v, got %v", defaultInterval, result)
		}
	})

	t.Run("InstanceTypePVE returns effectivePVEPollingInterval result", func(t *testing.T) {
		m := &Monitor{
			config: &config.Config{
				PVEPollingInterval: 30 * time.Second,
			},
		}
		result := m.baseIntervalForInstanceType(InstanceTypePVE)
		expected := 30 * time.Second
		if result != expected {
			t.Errorf("expected %v, got %v", expected, result)
		}
	})

	t.Run("InstanceTypePBS returns clamped PBS interval", func(t *testing.T) {
		m := &Monitor{
			config: &config.Config{
				PBSPollingInterval: 45 * time.Second,
			},
		}
		result := m.baseIntervalForInstanceType(InstanceTypePBS)
		expected := 45 * time.Second
		if result != expected {
			t.Errorf("expected %v, got %v", expected, result)
		}
	})

	t.Run("InstanceTypePBS with interval < 10s gets clamped to 10s", func(t *testing.T) {
		m := &Monitor{
			config: &config.Config{
				PBSPollingInterval: 5 * time.Second,
			},
		}
		result := m.baseIntervalForInstanceType(InstanceTypePBS)
		expected := 10 * time.Second
		if result != expected {
			t.Errorf("expected %v, got %v", expected, result)
		}
	})

	t.Run("InstanceTypePBS with interval > 1h gets clamped to 1h", func(t *testing.T) {
		m := &Monitor{
			config: &config.Config{
				PBSPollingInterval: 2 * time.Hour,
			},
		}
		result := m.baseIntervalForInstanceType(InstanceTypePBS)
		expected := time.Hour
		if result != expected {
			t.Errorf("expected %v, got %v", expected, result)
		}
	})

	t.Run("InstanceTypePMG returns clamped PMG interval", func(t *testing.T) {
		m := &Monitor{
			config: &config.Config{
				PMGPollingInterval: 2 * time.Minute,
			},
		}
		result := m.baseIntervalForInstanceType(InstanceTypePMG)
		expected := 2 * time.Minute
		if result != expected {
			t.Errorf("expected %v, got %v", expected, result)
		}
	})

	t.Run("unknown instance type with positive AdaptivePollingBaseInterval", func(t *testing.T) {
		m := &Monitor{
			config: &config.Config{
				AdaptivePollingBaseInterval: 20 * time.Second,
			},
		}
		result := m.baseIntervalForInstanceType(InstanceType("unknown"))
		expected := 20 * time.Second
		if result != expected {
			t.Errorf("expected %v, got %v", expected, result)
		}
	})

	t.Run("unknown instance type with zero AdaptivePollingBaseInterval uses default", func(t *testing.T) {
		m := &Monitor{
			config: &config.Config{
				AdaptivePollingBaseInterval: 0,
			},
		}
		result := m.baseIntervalForInstanceType(InstanceType("unknown"))
		if result != defaultInterval {
			t.Errorf("expected %v, got %v", defaultInterval, result)
		}
	})

	t.Run("unknown instance type with negative AdaptivePollingBaseInterval uses default", func(t *testing.T) {
		m := &Monitor{
			config: &config.Config{
				AdaptivePollingBaseInterval: -5 * time.Second,
			},
		}
		result := m.baseIntervalForInstanceType(InstanceType("unknown"))
		if result != defaultInterval {
			t.Errorf("expected %v, got %v", defaultInterval, result)
		}
	})
}
