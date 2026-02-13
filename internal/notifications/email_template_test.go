package notifications

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
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

func TestEmailTemplate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                string
		alerts              []*alerts.Alert
		isSingle            bool
		expectSingleSubject bool // subject contains single alert info vs "Multiple Alerts"
		subjectContains     string
	}{
		{
			name: "single alert with isSingle=true uses single template",
			alerts: []*alerts.Alert{
				{
					ID:           "alert-1",
					Level:        "critical",
					Type:         "cpu",
					ResourceName: "test-vm",
					Value:        95.5,
					Threshold:    90.0,
					StartTime:    time.Now(),
				},
			},
			isSingle:            true,
			expectSingleSubject: true,
			subjectContains:     "test-vm",
		},
		{
			name: "single alert with isSingle=false uses grouped template",
			alerts: []*alerts.Alert{
				{
					ID:           "alert-1",
					Level:        "warning",
					Type:         "memory",
					ResourceName: "test-vm",
					Value:        85.0,
					Threshold:    80.0,
					StartTime:    time.Now(),
				},
			},
			isSingle:            false,
			expectSingleSubject: false,
			subjectContains:     "1 Warning alert", // Grouped template uses "N Level alert(s)"
		},
		{
			name: "multiple alerts uses grouped template regardless of isSingle",
			alerts: []*alerts.Alert{
				{
					ID:           "alert-1",
					Level:        "critical",
					Type:         "cpu",
					ResourceName: "vm-1",
					Value:        95.5,
					Threshold:    90.0,
					StartTime:    time.Now(),
				},
				{
					ID:           "alert-2",
					Level:        "warning",
					Type:         "memory",
					ResourceName: "vm-2",
					Value:        85.0,
					Threshold:    80.0,
					StartTime:    time.Now(),
				},
			},
			isSingle:            true, // Even with isSingle=true, multiple alerts use grouped
			expectSingleSubject: false,
			subjectContains:     "Critical", // Subject shows level counts
		},
		{
			name: "warning level alert",
			alerts: []*alerts.Alert{
				{
					ID:           "alert-1",
					Level:        "warning",
					Type:         "disk",
					ResourceName: "storage-1",
					Value:        88.0,
					Threshold:    85.0,
					StartTime:    time.Now(),
				},
			},
			isSingle:            true,
			expectSingleSubject: true,
			subjectContains:     "Warning",
		},
		{
			name: "multiple critical alerts only uses grouped template",
			alerts: []*alerts.Alert{
				{
					ID:           "alert-1",
					Level:        "critical",
					Type:         "cpu",
					ResourceName: "vm-1",
					Value:        95.5,
					Threshold:    90.0,
					StartTime:    time.Now(),
				},
				{
					ID:           "alert-2",
					Level:        "critical",
					Type:         "memory",
					ResourceName: "vm-2",
					Value:        98.0,
					Threshold:    90.0,
					StartTime:    time.Now(),
				},
			},
			isSingle:            false,
			expectSingleSubject: false,
			subjectContains:     "2 Critical alerts",
		},
		{
			name: "io type alert formats as I/O",
			alerts: []*alerts.Alert{
				{
					ID:           "alert-io",
					Level:        "warning",
					Type:         "io",
					ResourceName: "storage-pool",
					Value:        150.0,
					Threshold:    100.0,
					StartTime:    time.Now(),
				},
			},
			isSingle:            true,
			expectSingleSubject: true,
			subjectContains:     "I/O",
		},
		{
			name: "custom type alert uses title case",
			alerts: []*alerts.Alert{
				{
					ID:           "alert-custom",
					Level:        "critical",
					Type:         "network_latency",
					ResourceName: "router-1",
					Value:        500.0,
					Threshold:    100.0,
					StartTime:    time.Now(),
				},
			},
			isSingle:            true,
			expectSingleSubject: true,
			subjectContains:     "router-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			subject, htmlBody, textBody := EmailTemplate(tt.alerts, tt.isSingle)

			// Check subject contains expected content
			if !strings.Contains(subject, tt.subjectContains) {
				t.Errorf("subject = %q, want to contain %q", subject, tt.subjectContains)
			}

			// Verify HTML body is not empty and contains basic structure
			if htmlBody == "" {
				t.Error("htmlBody is empty")
			}
			if !strings.Contains(htmlBody, "<!DOCTYPE html>") {
				t.Error("htmlBody missing DOCTYPE")
			}
			if !strings.Contains(htmlBody, "</html>") {
				t.Error("htmlBody missing closing html tag")
			}

			// Verify text body is not empty
			if textBody == "" {
				t.Error("textBody is empty")
			}

			// Check for single vs grouped template indicators
			if tt.expectSingleSubject {
				if strings.Contains(subject, "Multiple") || strings.Contains(subject, "Alerts]") {
					t.Errorf("expected single alert subject, got grouped: %q", subject)
				}
			}
		})
	}
}

func TestEmailTemplateEscapesHTMLContent(t *testing.T) {
	t.Parallel()

	alert := &alerts.Alert{
		ID:           "alert-html",
		Level:        "critical",
		Type:         "cpu<script>",
		ResourceName: `<img src=x onerror="alert(1)">`,
		ResourceID:   `vm-100"><script>alert(1)</script>`,
		Node:         `node-1"><script>alert(2)</script>`,
		Instance:     `https://pulse.example.com/?q=<script>alert(3)</script>`,
		Message:      `High CPU <script>alert("xss")</script>`,
		Value:        95.5,
		Threshold:    90.0,
		StartTime:    time.Now().Add(-5 * time.Minute),
	}

	_, htmlBody, textBody := EmailTemplate([]*alerts.Alert{alert}, true)

	if strings.Contains(htmlBody, "<script>") {
		t.Fatalf("html body should escape script tags, got %q", htmlBody)
	}
	if strings.Contains(htmlBody, `<img src=x onerror="alert(1)">`) {
		t.Fatalf("html body should escape html tags from resource name")
	}
	if !strings.Contains(htmlBody, "&lt;script&gt;alert(&#34;xss&#34;)&lt;/script&gt;") {
		t.Fatalf("expected escaped message content in html body, got %q", htmlBody)
	}
	if !strings.Contains(textBody, `High CPU <script>alert("xss")</script>`) {
		t.Fatalf("plain text body should preserve original message, got %q", textBody)
	}
}
