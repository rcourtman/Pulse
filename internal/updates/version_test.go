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

func TestParseVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		input      string
		wantErr    bool
		wantMajor  int
		wantMinor  int
		wantPatch  int
		wantPrerel string
		wantBuild  string
	}{
		{
			name:      "basic version",
			input:     "4.24.0",
			wantMajor: 4, wantMinor: 24, wantPatch: 0,
		},
		{
			name:      "with v prefix",
			input:     "v4.24.0",
			wantMajor: 4, wantMinor: 24, wantPatch: 0,
		},
		{
			name:       "with prerelease",
			input:      "4.24.0-rc.3",
			wantMajor:  4, wantMinor: 24, wantPatch: 0,
			wantPrerel: "rc.3",
		},
		{
			name:      "with build metadata",
			input:     "4.24.0+build.123",
			wantMajor: 4, wantMinor: 24, wantPatch: 0,
			wantBuild: "build.123",
		},
		{
			name:       "with prerelease and build",
			input:      "4.24.0-rc.3+build.123",
			wantMajor:  4, wantMinor: 24, wantPatch: 0,
			wantPrerel: "rc.3",
			wantBuild:  "build.123",
		},
		{
			name:    "invalid format",
			input:   "not-a-version",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseVersion(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ParseVersion(%q) expected error, got nil", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseVersion(%q) unexpected error: %v", tc.input, err)
			}
			if got.Major != tc.wantMajor || got.Minor != tc.wantMinor || got.Patch != tc.wantPatch {
				t.Fatalf("ParseVersion(%q) = %d.%d.%d, want %d.%d.%d",
					tc.input, got.Major, got.Minor, got.Patch, tc.wantMajor, tc.wantMinor, tc.wantPatch)
			}
			if got.Prerelease != tc.wantPrerel {
				t.Fatalf("ParseVersion(%q).Prerelease = %q, want %q", tc.input, got.Prerelease, tc.wantPrerel)
			}
			if got.Build != tc.wantBuild {
				t.Fatalf("ParseVersion(%q).Build = %q, want %q", tc.input, got.Build, tc.wantBuild)
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
