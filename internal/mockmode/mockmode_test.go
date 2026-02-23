package mockmode

import (
	"os"
	"testing"
)

func TestIsEnabled(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected bool
	}{
		{
			name:     "empty returns false",
			envValue: "",
			expected: false,
		},
		{
			name:     "true returns true",
			envValue: "true",
			expected: true,
		},
		{
			name:     "TRUE uppercase returns false (case-sensitive)",
			envValue: "TRUE",
			expected: false,
		},
		{
			name:     "True mixed case returns false (case-sensitive)",
			envValue: "True",
			expected: false,
		},
		{
			name:     "false returns false",
			envValue: "false",
			expected: false,
		},
		{
			name:     "FALSE uppercase returns false",
			envValue: "FALSE",
			expected: false,
		},
		{
			name:     "1 returns false",
			envValue: "1",
			expected: false,
		},
		{
			name:     "0 returns false",
			envValue: "0",
			expected: false,
		},
		{
			name:     "yes returns false",
			envValue: "yes",
			expected: false,
		},
		{
			name:     "true with spaces returns true (trimmed)",
			envValue: "  true  ",
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.envValue == "" {
				os.Unsetenv("PULSE_MOCK_MODE")
			} else {
				t.Setenv("PULSE_MOCK_MODE", tc.envValue)
			}

			result := IsEnabled()
			if result != tc.expected {
				t.Errorf("IsEnabled() with PULSE_MOCK_MODE=%q = %v, want %v", tc.envValue, result, tc.expected)
			}
		})
	}
}
