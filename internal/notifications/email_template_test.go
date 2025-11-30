package notifications

import (
	"testing"
	"time"
)

func TestTitleCase(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "single lowercase word",
			input:    "hello",
			expected: "Hello",
		},
		{
			name:     "single uppercase word",
			input:    "HELLO",
			expected: "Hello",
		},
		{
			name:     "single mixed case word",
			input:    "hElLo",
			expected: "Hello",
		},
		{
			name:     "multiple words lowercase",
			input:    "hello world",
			expected: "Hello World",
		},
		{
			name:     "multiple words uppercase",
			input:    "HELLO WORLD",
			expected: "Hello World",
		},
		{
			name:     "multiple words mixed case",
			input:    "hElLo WoRlD",
			expected: "Hello World",
		},
		{
			name:     "leading space",
			input:    " hello",
			expected: " Hello",
		},
		{
			name:     "trailing space",
			input:    "hello ",
			expected: "Hello ",
		},
		{
			name:     "multiple spaces between words",
			input:    "hello  world",
			expected: "Hello  World",
		},
		{
			name:     "tab separator",
			input:    "hello\tworld",
			expected: "Hello\tWorld",
		},
		{
			name:     "newline separator",
			input:    "hello\nworld",
			expected: "Hello\nWorld",
		},
		{
			name:     "single character",
			input:    "a",
			expected: "A",
		},
		{
			name:     "numbers and letters",
			input:    "test123 abc456",
			expected: "Test123 Abc456",
		},
		{
			name:     "hyphenated word stays joined",
			input:    "hello-world",
			expected: "Hello-world",
		},
		{
			name:     "underscore stays joined",
			input:    "hello_world",
			expected: "Hello_world",
		},
		{
			name:     "already title case",
			input:    "Hello World",
			expected: "Hello World",
		},
		{
			name:     "only spaces",
			input:    "   ",
			expected: "   ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := titleCase(tt.input)
			if result != tt.expected {
				t.Errorf("titleCase(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{
			name:     "zero duration",
			duration: 0,
			expected: "0 seconds",
		},
		{
			name:     "one second",
			duration: time.Second,
			expected: "1 seconds",
		},
		{
			name:     "30 seconds",
			duration: 30 * time.Second,
			expected: "30 seconds",
		},
		{
			name:     "59 seconds",
			duration: 59 * time.Second,
			expected: "59 seconds",
		},
		{
			name:     "one minute exactly",
			duration: time.Minute,
			expected: "1 minutes",
		},
		{
			name:     "90 seconds",
			duration: 90 * time.Second,
			expected: "1 minutes",
		},
		{
			name:     "30 minutes",
			duration: 30 * time.Minute,
			expected: "30 minutes",
		},
		{
			name:     "59 minutes",
			duration: 59 * time.Minute,
			expected: "59 minutes",
		},
		{
			name:     "one hour exactly",
			duration: time.Hour,
			expected: "1.0 hours",
		},
		{
			name:     "1.5 hours",
			duration: 90 * time.Minute,
			expected: "1.5 hours",
		},
		{
			name:     "12 hours",
			duration: 12 * time.Hour,
			expected: "12.0 hours",
		},
		{
			name:     "23 hours",
			duration: 23 * time.Hour,
			expected: "23.0 hours",
		},
		{
			name:     "one day exactly",
			duration: 24 * time.Hour,
			expected: "1.0 days",
		},
		{
			name:     "1.5 days",
			duration: 36 * time.Hour,
			expected: "1.5 days",
		},
		{
			name:     "7 days",
			duration: 7 * 24 * time.Hour,
			expected: "7.0 days",
		},
		{
			name:     "30 days",
			duration: 30 * 24 * time.Hour,
			expected: "30.0 days",
		},
		{
			name:     "sub-second durations truncate to 0 seconds",
			duration: 500 * time.Millisecond,
			expected: "0 seconds",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDuration(tt.duration)
			if result != tt.expected {
				t.Errorf("formatDuration(%v) = %q, want %q", tt.duration, result, tt.expected)
			}
		})
	}
}

func TestPluralize(t *testing.T) {
	tests := []struct {
		name     string
		count    int
		expected string
	}{
		{
			name:     "count zero",
			count:    0,
			expected: "s",
		},
		{
			name:     "count one",
			count:    1,
			expected: "",
		},
		{
			name:     "count two",
			count:    2,
			expected: "s",
		},
		{
			name:     "count ten",
			count:    10,
			expected: "s",
		},
		{
			name:     "count negative",
			count:    -1,
			expected: "s",
		},
		{
			name:     "large count",
			count:    1000000,
			expected: "s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pluralize(tt.count)
			if result != tt.expected {
				t.Errorf("pluralize(%d) = %q, want %q", tt.count, result, tt.expected)
			}
		})
	}
}

func TestFormatMetricValue(t *testing.T) {
	tests := []struct {
		name       string
		metricType string
		value      float64
		expected   string
	}{
		// CPU metrics
		{
			name:       "cpu percentage",
			metricType: "cpu",
			value:      85.5,
			expected:   "85.5%",
		},
		{
			name:       "CPU uppercase",
			metricType: "CPU",
			value:      95.3,
			expected:   "95.3%",
		},
		// Memory metrics
		{
			name:       "memory percentage",
			metricType: "memory",
			value:      72.8,
			expected:   "72.8%",
		},
		{
			name:       "Memory mixed case",
			metricType: "Memory",
			value:      50.0,
			expected:   "50.0%",
		},
		// Disk metrics
		{
			name:       "disk percentage",
			metricType: "disk",
			value:      90.1,
			expected:   "90.1%",
		},
		{
			name:       "usage percentage",
			metricType: "usage",
			value:      45.5,
			expected:   "45.5%",
		},
		// Disk I/O metrics
		{
			name:       "diskread rate",
			metricType: "diskread",
			value:      125.7,
			expected:   "125.7 MB/s",
		},
		{
			name:       "diskwrite rate",
			metricType: "diskwrite",
			value:      50.3,
			expected:   "50.3 MB/s",
		},
		{
			name:       "DiskRead uppercase",
			metricType: "DiskRead",
			value:      100.0,
			expected:   "100.0 MB/s",
		},
		// Network metrics
		{
			name:       "networkin rate",
			metricType: "networkin",
			value:      75.2,
			expected:   "75.2 MB/s",
		},
		{
			name:       "networkout rate",
			metricType: "networkout",
			value:      30.8,
			expected:   "30.8 MB/s",
		},
		// Temperature metrics
		{
			name:       "temperature celsius",
			metricType: "temperature",
			value:      65.5,
			expected:   "65.5°C",
		},
		{
			name:       "Temperature uppercase",
			metricType: "Temperature",
			value:      80.0,
			expected:   "80.0°C",
		},
		// Unknown metrics
		{
			name:       "unknown metric type",
			metricType: "custom",
			value:      123.456,
			expected:   "123.5",
		},
		{
			name:       "empty metric type",
			metricType: "",
			value:      99.9,
			expected:   "99.9",
		},
		// Edge cases
		{
			name:       "zero value",
			metricType: "cpu",
			value:      0.0,
			expected:   "0.0%",
		},
		{
			name:       "negative value",
			metricType: "temperature",
			value:      -10.5,
			expected:   "-10.5°C",
		},
		{
			name:       "large value",
			metricType: "diskread",
			value:      1000.0,
			expected:   "1000.0 MB/s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatMetricValue(tt.metricType, tt.value)
			if result != tt.expected {
				t.Errorf("formatMetricValue(%q, %v) = %q, want %q", tt.metricType, tt.value, result, tt.expected)
			}
		})
	}
}

func TestFormatMetricThreshold(t *testing.T) {
	tests := []struct {
		name       string
		metricType string
		threshold  float64
		expected   string
	}{
		// CPU metrics - note: threshold uses %.0f (no decimal places)
		{
			name:       "cpu threshold",
			metricType: "cpu",
			threshold:  80.0,
			expected:   "80%",
		},
		{
			name:       "CPU uppercase",
			metricType: "CPU",
			threshold:  90.0,
			expected:   "90%",
		},
		// Memory metrics
		{
			name:       "memory threshold",
			metricType: "memory",
			threshold:  75.0,
			expected:   "75%",
		},
		// Disk metrics
		{
			name:       "disk threshold",
			metricType: "disk",
			threshold:  85.0,
			expected:   "85%",
		},
		{
			name:       "usage threshold",
			metricType: "usage",
			threshold:  95.0,
			expected:   "95%",
		},
		// Disk I/O metrics
		{
			name:       "diskread threshold",
			metricType: "diskread",
			threshold:  100.0,
			expected:   "100 MB/s",
		},
		{
			name:       "diskwrite threshold",
			metricType: "diskwrite",
			threshold:  50.0,
			expected:   "50 MB/s",
		},
		// Network metrics
		{
			name:       "networkin threshold",
			metricType: "networkin",
			threshold:  200.0,
			expected:   "200 MB/s",
		},
		{
			name:       "networkout threshold",
			metricType: "networkout",
			threshold:  150.0,
			expected:   "150 MB/s",
		},
		// Temperature metrics
		{
			name:       "temperature threshold",
			metricType: "temperature",
			threshold:  80.0,
			expected:   "80°C",
		},
		// Unknown metrics
		{
			name:       "unknown metric type",
			metricType: "custom",
			threshold:  500.0,
			expected:   "500",
		},
		{
			name:       "empty metric type",
			metricType: "",
			threshold:  100.0,
			expected:   "100",
		},
		// Edge cases
		{
			name:       "zero threshold",
			metricType: "cpu",
			threshold:  0.0,
			expected:   "0%",
		},
		{
			name:       "decimal threshold rounds",
			metricType: "cpu",
			threshold:  85.7,
			expected:   "86%",
		},
		{
			name:       "negative threshold",
			metricType: "temperature",
			threshold:  -20.0,
			expected:   "-20°C",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatMetricThreshold(tt.metricType, tt.threshold)
			if result != tt.expected {
				t.Errorf("formatMetricThreshold(%q, %v) = %q, want %q", tt.metricType, tt.threshold, result, tt.expected)
			}
		})
	}
}
