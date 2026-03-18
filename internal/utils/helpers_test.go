package utils

import "testing"

// --- IsHexString ---

func TestIsHexString(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"abcdef1234567890", true},
		{"ABCDEF", true},
		{"0", true},
		{"", false},
		{"xyz", false},
		{"abc def", false},
		{"12g4", false},
	}
	for _, tt := range tests {
		got := IsHexString(tt.input)
		if got != tt.expected {
			t.Errorf("IsHexString(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

// --- CompareVersions prerelease ---

func TestCompareVersions_Prerelease(t *testing.T) {
	tests := []struct {
		a, b     string
		expected int
	}{
		// Stable > prerelease
		{"4.33.1", "4.33.1-rc1", 1},
		{"4.33.1-rc1", "4.33.1", -1},
		// Numeric prerelease comparison
		{"4.33.1-rc2", "4.33.1-rc1", 1},
		// More prerelease identifiers = higher
		{"4.33.1-rc.1.1", "4.33.1-rc.1", 1},
		// Both prerelease, same
		{"4.33.1-rc1", "4.33.1-rc1", 0},
	}
	for _, tt := range tests {
		got := CompareVersions(tt.a, tt.b)
		if got != tt.expected {
			t.Errorf("CompareVersions(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.expected)
		}
	}
}

// --- CompareVersions build metadata ---

func TestCompareVersions_BuildMetadata(t *testing.T) {
	// Build metadata MUST be ignored per semver
	tests := []struct {
		a, b     string
		expected int
	}{
		{"4.36.2+dirty", "4.36.2", 0},
		{"4.36.2+git.14", "4.36.2+git.1", 0},
		{"4.36.2+build123", "4.36.2+build789", 0},
	}
	for _, tt := range tests {
		got := CompareVersions(tt.a, tt.b)
		if got != tt.expected {
			t.Errorf("CompareVersions(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.expected)
		}
	}
}

// --- parseVersionNumber ---

func TestParseVersionNumber(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"4", 4},
		{"33", 33},
		{"", 0},
		{"abc", 0},
		{"4abc", 4},
		{" 4 ", 4},
	}
	for _, tt := range tests {
		got := parseVersionNumber(tt.input)
		if got != tt.expected {
			t.Errorf("parseVersionNumber(%q) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}

// --- parseNumericIdentifier ---

func TestParseNumericIdentifier(t *testing.T) {
	tests := []struct {
		input    string
		expected int
		isNum    bool
	}{
		{"42", 42, true},
		{"0", 0, true},
		{"", 0, false},
		{"abc", 0, false},
		{"12abc", 0, false},
	}
	for _, tt := range tests {
		got, ok := parseNumericIdentifier(tt.input)
		if got != tt.expected || ok != tt.isNum {
			t.Errorf("parseNumericIdentifier(%q) = (%d, %v), want (%d, %v)", tt.input, got, ok, tt.expected, tt.isNum)
		}
	}
}
