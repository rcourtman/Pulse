package hostagent

import (
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func TestNormalisePlatform(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"darwin", "macos"},
		{"Darwin", "macos"},
		{"DARWIN", "macos"},
		{" darwin ", "macos"},
		{"linux", "linux"},
		{"Linux", "linux"},
		{"windows", "windows"},
		{"freebsd", "freebsd"},
		{"", ""},
		{"  ", ""},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := normalisePlatform(tc.input)
			if result != tc.expected {
				t.Errorf("normalisePlatform(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestIsLoopback(t *testing.T) {
	tests := []struct {
		name     string
		flags    []string
		expected bool
	}{
		{
			name:     "has loopback",
			flags:    []string{"up", "loopback", "running"},
			expected: true,
		},
		{
			name:     "has LOOPBACK uppercase",
			flags:    []string{"UP", "LOOPBACK", "RUNNING"},
			expected: true,
		},
		{
			name:     "has Loopback mixed case",
			flags:    []string{"up", "Loopback", "running"},
			expected: true,
		},
		{
			name:     "no loopback",
			flags:    []string{"up", "broadcast", "running"},
			expected: false,
		},
		{
			name:     "empty flags",
			flags:    []string{},
			expected: false,
		},
		{
			name:     "nil flags",
			flags:    nil,
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := isLoopback(tc.flags)
			if result != tc.expected {
				t.Errorf("isLoopback(%v) = %v, want %v", tc.flags, result, tc.expected)
			}
		})
	}
}

func TestNewAgent_MissingToken(t *testing.T) {
	cfg := Config{
		PulseURL: "http://localhost:7655",
		APIToken: "",
	}

	_, err := New(cfg)
	if err == nil {
		t.Error("New() should fail when API token is missing")
	}
}

func TestNewAgent_WhitespaceToken(t *testing.T) {
	cfg := Config{
		PulseURL: "http://localhost:7655",
		APIToken: "   ",
	}

	_, err := New(cfg)
	if err == nil {
		t.Error("New() should fail when API token is only whitespace")
	}
}

func TestNewAgent_DefaultInterval(t *testing.T) {
	// We can't easily test New() without mocking gohost.InfoWithContext
	// but we can verify the default interval constant
	if defaultInterval != 30*time.Second {
		t.Errorf("defaultInterval = %v, want %v", defaultInterval, 30*time.Second)
	}
}

func TestConfig_Defaults(t *testing.T) {
	cfg := Config{}

	// Verify zero values
	if cfg.Interval != 0 {
		t.Errorf("Config.Interval should be zero by default, got %v", cfg.Interval)
	}
	if cfg.InsecureSkipVerify {
		t.Error("Config.InsecureSkipVerify should be false by default")
	}
	if cfg.RunOnce {
		t.Error("Config.RunOnce should be false by default")
	}
	if cfg.LogLevel != zerolog.Level(0) {
		t.Errorf("Config.LogLevel should be 0 (TraceLevel) by default, got %v", cfg.LogLevel)
	}
}

func TestVersion(t *testing.T) {
	// Verify Version is set (build-time override or default)
	if Version == "" {
		t.Error("Version should not be empty")
	}
}
