package api

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestIsFallbackMemorySource(t *testing.T) {
	tests := []struct {
		source   string
		expected bool
	}{
		// Fallback sources
		{"", true},
		{"unknown", true},
		{"Unknown", true},
		{"UNKNOWN", true},
		{"nodes-endpoint", true},
		{"Nodes-Endpoint", true},
		{"node-status-used", true},
		{"previous-snapshot", true},

		// Non-fallback sources
		{"cgroup", false},
		{"qemu-agent", false},
		{"proxmox-api", false},
		{"rrddata", false},
		{"some-other-source", false},
	}

	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			result := isFallbackMemorySource(tt.source)
			if result != tt.expected {
				t.Errorf("isFallbackMemorySource(%q) = %v, want %v", tt.source, result, tt.expected)
			}
		})
	}
}

func TestCopyStringSlice(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "empty slice",
			input:    []string{},
			expected: []string{},
		},
		{
			name:     "nil slice",
			input:    nil,
			expected: []string{},
		},
		{
			name:     "single element",
			input:    []string{"one"},
			expected: []string{"one"},
		},
		{
			name:     "multiple elements",
			input:    []string{"one", "two", "three"},
			expected: []string{"one", "two", "three"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := copyStringSlice(tt.input)

			if len(result) != len(tt.expected) {
				t.Errorf("copyStringSlice() length = %d, want %d", len(result), len(tt.expected))
				return
			}

			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("copyStringSlice()[%d] = %q, want %q", i, v, tt.expected[i])
				}
			}

			// Verify it's a copy, not the same slice
			if len(tt.input) > 0 && len(result) > 0 {
				result[0] = "modified"
				if tt.input[0] == "modified" {
					t.Error("copyStringSlice() returned reference to original, not a copy")
				}
			}
		})
	}
}

func TestNormalizeHostForComparison(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// Empty/whitespace
		{"", ""},
		{"   ", ""},

		// Basic hostnames
		{"example.com", "example.com"},
		{"Example.COM", "example.com"},
		{"  example.com  ", "example.com"},

		// With protocol (lowercase only - uppercase protocols not stripped)
		{"https://example.com", "example.com"},
		{"http://example.com", "example.com"},
		// Note: uppercase protocols are not stripped, result is lowercase of input
		{"HTTPS://Example.COM", "https"},

		// With port
		{"example.com:8006", "example.com"},
		{"https://example.com:8006", "example.com"},
		{"192.168.1.1:8006", "192.168.1.1"},

		// With path
		{"example.com/api/v1", "example.com"},
		{"https://example.com/api/v1", "example.com"},
		{"https://example.com:8006/api/v1", "example.com"},

		// IP addresses
		{"192.168.1.1", "192.168.1.1"},
		{"https://192.168.1.1:8006", "192.168.1.1"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeHostForComparison(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeHostForComparison(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNormalizeVersionLabel(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// Empty/whitespace
		{"", ""},
		{"   ", ""},

		// Already has v prefix
		{"v1.0.0", "v1.0.0"},
		{"v4.35.0", "v4.35.0"},
		{"  v1.0.0  ", "v1.0.0"},

		// Needs v prefix
		{"1.0.0", "v1.0.0"},
		{"4.35.0", "v4.35.0"},
		{"  4.35.0  ", "v4.35.0"},

		// Non-numeric prefix (no v added)
		{"dev", "dev"},
		{"alpha-1.0", "alpha-1.0"},
		{"beta", "beta"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeVersionLabel(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeVersionLabel(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		slice    []string
		str      string
		expected bool
	}{
		{"empty slice", []string{}, "test", false},
		{"nil slice", nil, "test", false},
		{"found at start", []string{"test", "foo", "bar"}, "test", true},
		{"found at end", []string{"foo", "bar", "test"}, "test", true},
		{"found in middle", []string{"foo", "test", "bar"}, "test", true},
		{"not found", []string{"foo", "bar", "baz"}, "test", false},
		{"case sensitive", []string{"Test", "TEST"}, "test", false},
		{"empty string search", []string{"foo", "", "bar"}, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := contains(tt.slice, tt.str)
			if result != tt.expected {
				t.Errorf("contains(%v, %q) = %v, want %v", tt.slice, tt.str, result, tt.expected)
			}
		})
	}
}

func TestContainsFold(t *testing.T) {
	tests := []struct {
		name      string
		slice     []string
		candidate string
		expected  bool
	}{
		{"empty slice", []string{}, "test", false},
		{"nil slice", nil, "test", false},
		{"empty candidate", []string{"foo", "bar"}, "", false},
		{"whitespace candidate", []string{"foo", "bar"}, "   ", false},
		{"exact match", []string{"test", "foo"}, "test", true},
		{"case insensitive match", []string{"Test", "foo"}, "test", true},
		{"uppercase match", []string{"test", "foo"}, "TEST", true},
		{"mixed case match", []string{"TeSt", "foo"}, "tEsT", true},
		{"with whitespace in slice", []string{"  test  ", "foo"}, "test", true},
		{"with whitespace in candidate", []string{"test", "foo"}, "  test  ", true},
		{"not found", []string{"foo", "bar"}, "test", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsFold(tt.slice, tt.candidate)
			if result != tt.expected {
				t.Errorf("containsFold(%v, %q) = %v, want %v", tt.slice, tt.candidate, result, tt.expected)
			}
		})
	}
}

func TestInterfaceToStringSlice(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected []string
	}{
		{
			name:     "nil",
			input:    nil,
			expected: nil,
		},
		{
			name:     "string slice",
			input:    []string{"one", "two", "three"},
			expected: []string{"one", "two", "three"},
		},
		{
			name:     "empty string slice",
			input:    []string{},
			expected: []string{},
		},
		{
			name:     "interface slice with strings",
			input:    []interface{}{"one", "two", "three"},
			expected: []string{"one", "two", "three"},
		},
		{
			name:     "empty interface slice",
			input:    []interface{}{},
			expected: []string{},
		},
		{
			name:     "interface slice with mixed types",
			input:    []interface{}{"one", 2, "three", true},
			expected: []string{"one", "three"},
		},
		{
			name:     "interface slice with only non-strings",
			input:    []interface{}{1, 2, 3},
			expected: []string{},
		},
		{
			name:     "unsupported type",
			input:    "not a slice",
			expected: nil,
		},
		{
			name:     "int slice (unsupported)",
			input:    []int{1, 2, 3},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := interfaceToStringSlice(tt.input)

			if tt.expected == nil {
				if result != nil {
					t.Errorf("interfaceToStringSlice() = %v, want nil", result)
				}
				return
			}

			if len(result) != len(tt.expected) {
				t.Errorf("interfaceToStringSlice() length = %d, want %d", len(result), len(tt.expected))
				return
			}

			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("interfaceToStringSlice()[%d] = %q, want %q", i, v, tt.expected[i])
				}
			}
		})
	}
}

func TestPreferredDockerHostName(t *testing.T) {
	tests := []struct {
		name     string
		host     models.DockerHost
		expected string
	}{
		{
			name: "display name preferred",
			host: models.DockerHost{
				ID:          "id-123",
				DisplayName: "My Docker Host",
				Hostname:    "docker-host.local",
				AgentID:     "agent-456",
			},
			expected: "My Docker Host",
		},
		{
			name: "hostname when no display name",
			host: models.DockerHost{
				ID:          "id-123",
				DisplayName: "",
				Hostname:    "docker-host.local",
				AgentID:     "agent-456",
			},
			expected: "docker-host.local",
		},
		{
			name: "agent ID when no display name or hostname",
			host: models.DockerHost{
				ID:          "id-123",
				DisplayName: "",
				Hostname:    "",
				AgentID:     "agent-456",
			},
			expected: "agent-456",
		},
		{
			name: "ID as fallback",
			host: models.DockerHost{
				ID:          "id-123",
				DisplayName: "",
				Hostname:    "",
				AgentID:     "",
			},
			expected: "id-123",
		},
		{
			name: "whitespace-only display name ignored",
			host: models.DockerHost{
				ID:          "id-123",
				DisplayName: "   ",
				Hostname:    "docker-host.local",
				AgentID:     "agent-456",
			},
			expected: "docker-host.local",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := preferredDockerHostName(tt.host)
			if result != tt.expected {
				t.Errorf("preferredDockerHostName() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestFormatTimeMaybe(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		expected string
	}{
		{
			name:     "zero time",
			input:    time.Time{},
			expected: "",
		},
		{
			name:     "specific time",
			input:    time.Date(2025, 11, 30, 12, 30, 45, 0, time.UTC),
			expected: "2025-11-30T12:30:45Z",
		},
		{
			name:     "non-UTC time converted to UTC",
			input:    time.Date(2025, 11, 30, 12, 30, 45, 0, time.FixedZone("EST", -5*60*60)),
			expected: "2025-11-30T17:30:45Z",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatTimeMaybe(tt.input)
			if result != tt.expected {
				t.Errorf("formatTimeMaybe() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestHasLegacyThresholds(t *testing.T) {
	ptrFloat := func(v float64) *float64 { return &v }

	tests := []struct {
		name     string
		config   alerts.ThresholdConfig
		expected bool
	}{
		{
			name:     "empty config",
			config:   alerts.ThresholdConfig{},
			expected: false,
		},
		{
			name: "modern thresholds only",
			config: alerts.ThresholdConfig{
				CPU:    &alerts.HysteresisThreshold{Trigger: 80, Clear: 70},
				Memory: &alerts.HysteresisThreshold{Trigger: 85, Clear: 75},
			},
			expected: false,
		},
		{
			name: "CPU legacy set",
			config: alerts.ThresholdConfig{
				CPULegacy: ptrFloat(80.0),
			},
			expected: true,
		},
		{
			name: "Memory legacy set",
			config: alerts.ThresholdConfig{
				MemoryLegacy: ptrFloat(85.0),
			},
			expected: true,
		},
		{
			name: "Disk legacy set",
			config: alerts.ThresholdConfig{
				DiskLegacy: ptrFloat(90.0),
			},
			expected: true,
		},
		{
			name: "DiskRead legacy set",
			config: alerts.ThresholdConfig{
				DiskReadLegacy: ptrFloat(50.0),
			},
			expected: true,
		},
		{
			name: "DiskWrite legacy set",
			config: alerts.ThresholdConfig{
				DiskWriteLegacy: ptrFloat(60.0),
			},
			expected: true,
		},
		{
			name: "NetworkIn legacy set",
			config: alerts.ThresholdConfig{
				NetworkInLegacy: ptrFloat(70.0),
			},
			expected: true,
		},
		{
			name: "NetworkOut legacy set",
			config: alerts.ThresholdConfig{
				NetworkOutLegacy: ptrFloat(75.0),
			},
			expected: true,
		},
		{
			name: "multiple legacy fields set",
			config: alerts.ThresholdConfig{
				CPULegacy:    ptrFloat(80.0),
				MemoryLegacy: ptrFloat(85.0),
				DiskLegacy:   ptrFloat(90.0),
			},
			expected: true,
		},
		{
			name: "mixed modern and legacy",
			config: alerts.ThresholdConfig{
				CPU:          &alerts.HysteresisThreshold{Trigger: 80, Clear: 70},
				MemoryLegacy: ptrFloat(85.0),
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasLegacyThresholds(tt.config)
			if result != tt.expected {
				t.Errorf("hasLegacyThresholds() = %v, want %v", result, tt.expected)
			}
		})
	}
}
