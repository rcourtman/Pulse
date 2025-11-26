package system

import (
	"testing"
)

func TestIsHexString(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"0123456789abcdef", true},
		{"ABCDEF", true},
		{"0123456789ABCDEF", true},
		{"abc123", true},
		{"", true}, // empty string has no non-hex chars
		{"xyz", false},
		{"123g", false},
		{"hello-world", false},
		{"12 34", false}, // space
		{"12\n34", false}, // newline
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := isHexString(tc.input)
			if result != tc.expected {
				t.Errorf("isHexString(%q) = %v, want %v", tc.input, result, tc.expected)
			}
		})
	}
}

func TestIsNumeric(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"123", true},
		{"0", true},
		{"9999999", true},
		{" 123 ", true}, // whitespace trimmed
		{"", false},
		{"   ", false},
		{"abc", false},
		{"12.3", false},
		{"-1", false},
		{"1a2", false},
		{"12 34", false},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := isNumeric(tc.input)
			if result != tc.expected {
				t.Errorf("isNumeric(%q) = %v, want %v", tc.input, result, tc.expected)
			}
		})
	}
}

func TestIsTruthy(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		// Truthy values
		{"1", true},
		{"true", true},
		{"TRUE", true},
		{"True", true},
		{"t", true},
		{"T", true},
		{"yes", true},
		{"YES", true},
		{"Yes", true},
		{"y", true},
		{"Y", true},
		{"on", true},
		{"ON", true},
		{"On", true},
		{" true ", true}, // with whitespace
		{"\ttrue\n", true},

		// Falsy values
		{"0", false},
		{"false", false},
		{"FALSE", false},
		{"no", false},
		{"n", false},
		{"off", false},
		{"", false},
		{"   ", false},
		{"random", false},
		{"2", false},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := isTruthy(tc.input)
			if result != tc.expected {
				t.Errorf("isTruthy(%q) = %v, want %v", tc.input, result, tc.expected)
			}
		})
	}
}

func TestContainerMarkers(t *testing.T) {
	// Verify all expected markers are present
	expectedMarkers := []string{
		"docker",
		"lxc",
		"containerd",
		"kubepods",
		"podman",
		"crio",
		"libpod",
		"lxcfs",
	}

	for _, expected := range expectedMarkers {
		found := false
		for _, marker := range containerMarkers {
			if marker == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected container marker %q not found", expected)
		}
	}
}

func TestInContainer(t *testing.T) {
	// This is an integration test that depends on the runtime environment
	// We can't mock the file system easily, but we can verify it doesn't panic
	result := InContainer()
	t.Logf("InContainer() = %v (depends on test environment)", result)
}

func TestDetectDockerContainerName(t *testing.T) {
	// This is an integration test that depends on the runtime environment
	result := DetectDockerContainerName()
	t.Logf("DetectDockerContainerName() = %q (depends on test environment)", result)
}

func TestDetectLXCCTID(t *testing.T) {
	// This is an integration test that depends on the runtime environment
	result := DetectLXCCTID()
	t.Logf("DetectLXCCTID() = %q (depends on test environment)", result)
}
