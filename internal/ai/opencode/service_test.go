package opencode

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateSessionTitle(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "short message unchanged",
			input:    "How do I check disk space?",
			expected: "How do I check disk space?",
		},
		{
			name:     "long message truncated at word boundary",
			input:    "I need help understanding how to monitor CPU usage across all my Proxmox nodes and containers",
			expected: "I need help understanding how to monitor CPU...",
		},
		{
			name:     "message with newlines normalized",
			input:    "Hello\n\nHow are you?\nI need help",
			expected: "Hello How are you? I need help",
		},
		{
			name:     "message with multiple spaces collapsed",
			input:    "Hello    world   with   spaces",
			expected: "Hello world with spaces",
		},
		{
			name:     "very long word truncated without space",
			input:    "supercalifragilisticexpialidociouslylongwordthatgoesonforeverandever",
			expected: "supercalifragilisticexpialidociouslylongwordthatgo...",
		},
		{
			name:     "unicode characters counted correctly",
			input:    "こんにちは世界！これは日本語のテストです。長い文章を書いています。",
			expected: "こんにちは世界！これは日本語のテストです。長い文章を書いています。",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "exactly 50 characters",
			input:    "12345678901234567890123456789012345678901234567890",
			expected: "12345678901234567890123456789012345678901234567890",
		},
		{
			name:     "51 characters truncated",
			input:    "123456789012345678901234567890123456789012345678901",
			expected: "12345678901234567890123456789012345678901234567890...",
		},
		{
			name:     "leading/trailing whitespace trimmed",
			input:    "   Hello world   ",
			expected: "Hello world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateSessionTitle(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
