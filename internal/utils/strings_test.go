package utils

import "testing"

// --- AddToken ---

func TestAddToken_Basic(t *testing.T) {
	tokens := make(map[string]struct{})
	AddToken(tokens, "Hello")
	if _, ok := tokens["hello"]; !ok {
		t.Error("token should be lowercased and added")
	}
}

func TestAddToken_Trimmed(t *testing.T) {
	tokens := make(map[string]struct{})
	AddToken(tokens, "  World  ")
	if _, ok := tokens["world"]; !ok {
		t.Error("token should be trimmed and lowercased")
	}
}

func TestAddToken_SkipsEmpty(t *testing.T) {
	tokens := make(map[string]struct{})
	AddToken(tokens, "")
	AddToken(tokens, "   ")
	if len(tokens) != 0 {
		t.Error("empty/whitespace values should be skipped")
	}
}

// --- LastSegment ---

func TestLastSegment(t *testing.T) {
	tests := []struct {
		value    string
		sep      byte
		expected string
	}{
		{"a/b/c", '/', "c"},
		{"a:b:c", ':', "c"},
		{"abc", '/', ""},
		{"abc/", '/', ""},
		{"", '/', ""},
	}
	for _, tt := range tests {
		got := LastSegment(tt.value, tt.sep)
		if got != tt.expected {
			t.Errorf("LastSegment(%q, %q) = %q, want %q", tt.value, tt.sep, got, tt.expected)
		}
	}
}

// --- TrailingDigits ---

func TestTrailingDigits(t *testing.T) {
	tests := []struct {
		input, expected string
	}{
		{"pve1", "1"},
		{"node123", "123"},
		{"abc", ""},
		{"", ""},
		{"123", "123"},
		{"node", ""},
	}
	for _, tt := range tests {
		got := TrailingDigits(tt.input)
		if got != tt.expected {
			t.Errorf("TrailingDigits(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
