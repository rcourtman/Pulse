package monitoring

import (
	"testing"
	"time"
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
