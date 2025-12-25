package dockeragent

import (
	"testing"
)

func TestMaskSensitiveEnvVars(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "empty input",
			input:    nil,
			expected: nil,
		},
		{
			name:     "no sensitive vars",
			input:    []string{"PATH=/usr/bin", "HOME=/root", "SHELL=/bin/bash"},
			expected: []string{"PATH=/usr/bin", "HOME=/root", "SHELL=/bin/bash"},
		},
		{
			name:     "password is masked",
			input:    []string{"DB_PASSWORD=secret123", "USER=admin"},
			expected: []string{"DB_PASSWORD=***", "USER=admin"},
		},
		{
			name:     "multiple sensitive vars",
			input:    []string{"API_KEY=abc123", "SECRET_TOKEN=xyz789", "DEBUG=true"},
			expected: []string{"API_KEY=***", "SECRET_TOKEN=***", "DEBUG=true"},
		},
		{
			name:     "case insensitive matching",
			input:    []string{"my_PASSWORD=pass", "MySecret=val", "TOKEN=tok"},
			expected: []string{"my_PASSWORD=***", "MySecret=***", "TOKEN=***"},
		},
		{
			name:     "empty value not masked",
			input:    []string{"PASSWORD=", "USER=admin"},
			expected: []string{"PASSWORD=", "USER=admin"},
		},
		{
			name:     "no equals sign preserved",
			input:    []string{"MALFORMED_VAR", "USER=admin"},
			expected: []string{"MALFORMED_VAR", "USER=admin"},
		},
		{
			name:     "auth keyword masked",
			input:    []string{"AUTH_TOKEN=abc", "AUTHORIZATION=bearer xyz"},
			expected: []string{"AUTH_TOKEN=***", "AUTHORIZATION=***"},
		},
		{
			name:     "database_url masked",
			input:    []string{"DATABASE_URL=postgres://user:pass@host/db"},
			expected: []string{"DATABASE_URL=***"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := maskSensitiveEnvVars(tt.input)

			if len(result) != len(tt.expected) {
				t.Errorf("length mismatch: got %d, want %d", len(result), len(tt.expected))
				return
			}

			for i, exp := range tt.expected {
				if result[i] != exp {
					t.Errorf("mismatch at index %d: got %q, want %q", i, result[i], exp)
				}
			}
		})
	}
}
