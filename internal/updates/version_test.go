package updates

import "testing"

func TestNormalizeVersionString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "release version stays the same",
			input:    "4.24.0-rc.3",
			expected: "4.24.0-rc.3",
		},
		{
			name:     "git describe converts to build metadata",
			input:    "4.24.0-rc.3-45-gABCDEF",
			expected: "4.24.0-rc.3+git.45.gabcdef",
		},
		{
			name:     "git describe dirty includes dirty flag",
			input:    "4.24.0-rc.3-45-gabc123-dirty",
			expected: "4.24.0-rc.3+git.45.gabc123.dirty",
		},
		{
			name:     "branch name falls back to prerelease",
			input:    "issue-551",
			expected: "0.0.0-issue-551",
		},
		{
			name:     "branch with slashes sanitized",
			input:    "feature/new-api",
			expected: "0.0.0-feature-new-api",
		},
		{
			name:     "empty input defaults to dev",
			input:    "",
			expected: "0.0.0-dev",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := normalizeVersionString(tc.input); got != tc.expected {
				t.Fatalf("normalizeVersionString(%q) = %q, expected %q", tc.input, got, tc.expected)
			}
		})
	}
}

func TestDetectChannelFromVersion(t *testing.T) {
	t.Parallel()

	if got := detectChannelFromVersion("4.24.0-rc.3"); got != "rc" {
		t.Fatalf("detectChannelFromVersion rc = %s, expected rc", got)
	}

	if got := detectChannelFromVersion("0.0.0-feature-x"); got != "stable" {
		t.Fatalf("detectChannelFromVersion stable = %s, expected stable", got)
	}
}
