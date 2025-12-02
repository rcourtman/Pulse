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
			name:      "with prerelease",
			input:     "4.24.0-rc.3",
			wantMajor: 4, wantMinor: 24, wantPatch: 0,
			wantPrerel: "rc.3",
		},
		{
			name:      "with build metadata",
			input:     "4.24.0+build.123",
			wantMajor: 4, wantMinor: 24, wantPatch: 0,
			wantBuild: "build.123",
		},
		{
			name:      "with prerelease and build",
			input:     "4.24.0-rc.3+build.123",
			wantMajor: 4, wantMinor: 24, wantPatch: 0,
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

func TestVersionString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		version  Version
		expected string
	}{
		{
			name:     "basic version",
			version:  Version{Major: 4, Minor: 24, Patch: 0},
			expected: "4.24.0",
		},
		{
			name:     "with prerelease",
			version:  Version{Major: 4, Minor: 24, Patch: 0, Prerelease: "rc.3"},
			expected: "4.24.0-rc.3",
		},
		{
			name:     "with build metadata",
			version:  Version{Major: 4, Minor: 24, Patch: 0, Build: "build.123"},
			expected: "4.24.0+build.123",
		},
		{
			name:     "with prerelease and build",
			version:  Version{Major: 4, Minor: 24, Patch: 0, Prerelease: "rc.3", Build: "build.123"},
			expected: "4.24.0-rc.3+build.123",
		},
		{
			name:     "zero version",
			version:  Version{Major: 0, Minor: 0, Patch: 0},
			expected: "0.0.0",
		},
		{
			name:     "large numbers",
			version:  Version{Major: 100, Minor: 200, Patch: 300},
			expected: "100.200.300",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.version.String(); got != tc.expected {
				t.Fatalf("Version.String() = %q, expected %q", got, tc.expected)
			}
		})
	}
}

func TestVersionCompare(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		v1       Version
		v2       Version
		expected int
	}{
		{
			name:     "equal versions",
			v1:       Version{Major: 4, Minor: 24, Patch: 0},
			v2:       Version{Major: 4, Minor: 24, Patch: 0},
			expected: 0,
		},
		{
			name:     "v1 major greater",
			v1:       Version{Major: 5, Minor: 0, Patch: 0},
			v2:       Version{Major: 4, Minor: 24, Patch: 0},
			expected: 1,
		},
		{
			name:     "v1 major less",
			v1:       Version{Major: 3, Minor: 99, Patch: 99},
			v2:       Version{Major: 4, Minor: 0, Patch: 0},
			expected: -1,
		},
		{
			name:     "v1 minor greater",
			v1:       Version{Major: 4, Minor: 25, Patch: 0},
			v2:       Version{Major: 4, Minor: 24, Patch: 0},
			expected: 1,
		},
		{
			name:     "v1 minor less",
			v1:       Version{Major: 4, Minor: 23, Patch: 99},
			v2:       Version{Major: 4, Minor: 24, Patch: 0},
			expected: -1,
		},
		{
			name:     "v1 patch greater",
			v1:       Version{Major: 4, Minor: 24, Patch: 1},
			v2:       Version{Major: 4, Minor: 24, Patch: 0},
			expected: 1,
		},
		{
			name:     "v1 patch less",
			v1:       Version{Major: 4, Minor: 24, Patch: 0},
			v2:       Version{Major: 4, Minor: 24, Patch: 1},
			expected: -1,
		},
		{
			name:     "release > prerelease",
			v1:       Version{Major: 4, Minor: 24, Patch: 0},
			v2:       Version{Major: 4, Minor: 24, Patch: 0, Prerelease: "rc.1"},
			expected: 1,
		},
		{
			name:     "prerelease < release",
			v1:       Version{Major: 4, Minor: 24, Patch: 0, Prerelease: "rc.1"},
			v2:       Version{Major: 4, Minor: 24, Patch: 0},
			expected: -1,
		},
		{
			name:     "RC comparison - rc.9 > rc.1",
			v1:       Version{Major: 4, Minor: 24, Patch: 0, Prerelease: "rc.9"},
			v2:       Version{Major: 4, Minor: 24, Patch: 0, Prerelease: "rc.1"},
			expected: 1,
		},
		{
			name:     "RC comparison - rc.1 < rc.9",
			v1:       Version{Major: 4, Minor: 24, Patch: 0, Prerelease: "rc.1"},
			v2:       Version{Major: 4, Minor: 24, Patch: 0, Prerelease: "rc.9"},
			expected: -1,
		},
		{
			name:     "RC comparison - rc9 format",
			v1:       Version{Major: 4, Minor: 24, Patch: 0, Prerelease: "rc9"},
			v2:       Version{Major: 4, Minor: 24, Patch: 0, Prerelease: "rc1"},
			expected: 1,
		},
		{
			name:     "equal prereleases",
			v1:       Version{Major: 4, Minor: 24, Patch: 0, Prerelease: "rc.3"},
			v2:       Version{Major: 4, Minor: 24, Patch: 0, Prerelease: "rc.3"},
			expected: 0,
		},
		{
			name:     "string prerelease comparison",
			v1:       Version{Major: 4, Minor: 24, Patch: 0, Prerelease: "beta"},
			v2:       Version{Major: 4, Minor: 24, Patch: 0, Prerelease: "alpha"},
			expected: 1, // "beta" > "alpha" lexically
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.v1.Compare(&tc.v2); got != tc.expected {
				t.Fatalf("(%v).Compare(%v) = %d, expected %d", tc.v1, tc.v2, got, tc.expected)
			}
		})
	}
}

func TestVersionIsNewerThan(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		v1       Version
		v2       Version
		expected bool
	}{
		{
			name:     "newer version",
			v1:       Version{Major: 5, Minor: 0, Patch: 0},
			v2:       Version{Major: 4, Minor: 24, Patch: 0},
			expected: true,
		},
		{
			name:     "older version",
			v1:       Version{Major: 4, Minor: 0, Patch: 0},
			v2:       Version{Major: 4, Minor: 24, Patch: 0},
			expected: false,
		},
		{
			name:     "equal versions",
			v1:       Version{Major: 4, Minor: 24, Patch: 0},
			v2:       Version{Major: 4, Minor: 24, Patch: 0},
			expected: false,
		},
		{
			name:     "release newer than prerelease",
			v1:       Version{Major: 4, Minor: 24, Patch: 0},
			v2:       Version{Major: 4, Minor: 24, Patch: 0, Prerelease: "rc.1"},
			expected: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.v1.IsNewerThan(&tc.v2); got != tc.expected {
				t.Fatalf("(%v).IsNewerThan(%v) = %v, expected %v", tc.v1, tc.v2, got, tc.expected)
			}
		})
	}
}

func TestVersionIsPrerelease(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		version  Version
		expected bool
	}{
		{
			name:     "release version",
			version:  Version{Major: 4, Minor: 24, Patch: 0},
			expected: false,
		},
		{
			name:     "prerelease version",
			version:  Version{Major: 4, Minor: 24, Patch: 0, Prerelease: "rc.1"},
			expected: true,
		},
		{
			name:     "alpha prerelease",
			version:  Version{Major: 1, Minor: 0, Patch: 0, Prerelease: "alpha"},
			expected: true,
		},
		{
			name:     "build metadata only is not prerelease",
			version:  Version{Major: 4, Minor: 24, Patch: 0, Build: "build.123"},
			expected: false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.version.IsPrerelease(); got != tc.expected {
				t.Fatalf("(%v).IsPrerelease() = %v, expected %v", tc.version, got, tc.expected)
			}
		})
	}
}

func TestCompareInts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		a        int
		b        int
		expected int
	}{
		{name: "a < b", a: 1, b: 2, expected: -1},
		{name: "a > b", a: 2, b: 1, expected: 1},
		{name: "a == b", a: 1, b: 1, expected: 0},
		{name: "negative numbers a < b", a: -2, b: -1, expected: -1},
		{name: "negative numbers a > b", a: -1, b: -2, expected: 1},
		{name: "zero comparisons", a: 0, b: 1, expected: -1},
		{name: "large numbers", a: 1000000, b: 1000001, expected: -1},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := compareInts(tc.a, tc.b); got != tc.expected {
				t.Fatalf("compareInts(%d, %d) = %d, expected %d", tc.a, tc.b, got, tc.expected)
			}
		})
	}
}

func TestExtractRCNumber(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		prerelease string
		expected   int
	}{
		{name: "rc.1", prerelease: "rc.1", expected: 1},
		{name: "rc.9", prerelease: "rc.9", expected: 9},
		{name: "rc1", prerelease: "rc1", expected: 1},
		{name: "rc9", prerelease: "rc9", expected: 9},
		{name: "RC.3 uppercase", prerelease: "RC.3", expected: 3},
		{name: "RC5 uppercase no dot", prerelease: "RC5", expected: 5},
		{name: "rc.10 double digit", prerelease: "rc.10", expected: 10},
		{name: "rc.123 triple digit", prerelease: "rc.123", expected: 123},
		{name: "alpha no RC", prerelease: "alpha", expected: -1},
		{name: "beta.1 no RC", prerelease: "beta.1", expected: -1},
		{name: "empty string", prerelease: "", expected: -1},
		{name: "just rc no number", prerelease: "rc", expected: -1},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := extractRCNumber(tc.prerelease); got != tc.expected {
				t.Fatalf("extractRCNumber(%q) = %d, expected %d", tc.prerelease, got, tc.expected)
			}
		})
	}
}

func TestEnvBool(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected bool
	}{
		{name: "1 is true", value: "1", expected: true},
		{name: "true is true", value: "true", expected: true},
		{name: "TRUE is true", value: "TRUE", expected: true},
		{name: "True is true", value: "True", expected: true},
		{name: "yes is true", value: "yes", expected: true},
		{name: "YES is true", value: "YES", expected: true},
		{name: "on is true", value: "on", expected: true},
		{name: "ON is true", value: "ON", expected: true},
		{name: "0 is false", value: "0", expected: false},
		{name: "false is false", value: "false", expected: false},
		{name: "FALSE is false", value: "FALSE", expected: false},
		{name: "no is false", value: "no", expected: false},
		{name: "off is false", value: "off", expected: false},
		{name: "empty is false", value: "", expected: false},
		{name: "random string is false", value: "random", expected: false},
		{name: "whitespace true", value: "  true  ", expected: true},
		{name: "whitespace 1", value: "  1  ", expected: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			key := "TEST_ENV_BOOL_" + tc.name
			t.Setenv(key, tc.value)
			if got := envBool(key); got != tc.expected {
				t.Fatalf("envBool(%q) with value %q = %v, expected %v", key, tc.value, got, tc.expected)
			}
		})
	}
}

func TestSanitizePrereleaseIdentifier(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "simple branch", input: "feature-x", expected: "feature-x"},
		{name: "branch with slash", input: "feature/new-api", expected: "feature-new-api"},
		{name: "branch with multiple slashes", input: "user/feature/test", expected: "user-feature-test"},
		{name: "uppercase converts to lowercase", input: "FEATURE-X", expected: "feature-x"},
		{name: "mixed case", input: "Feature-X", expected: "feature-x"},
		{name: "numbers preserved", input: "issue-123", expected: "issue-123"},
		{name: "dots preserved", input: "v1.2.3", expected: "v1.2.3"},
		{name: "special chars replaced", input: "feat@test#123", expected: "feat-test-123"},
		{name: "consecutive separators collapsed", input: "feature//test", expected: "feature-test"},
		{name: "leading separator trimmed", input: "-feature", expected: "feature"},
		{name: "trailing separator trimmed", input: "feature-", expected: "feature"},
		{name: "empty string", input: "", expected: "dev"},
		{name: "whitespace only", input: "   ", expected: "dev"},
		{name: "only special chars", input: "@#$%", expected: "dev"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := sanitizePrereleaseIdentifier(tc.input); got != tc.expected {
				t.Fatalf("sanitizePrereleaseIdentifier(%q) = %q, expected %q", tc.input, got, tc.expected)
			}
		})
	}
}

func TestNormalizeGitDescribeVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
		ok       bool
	}{
		// Valid git describe outputs
		{
			name:     "basic git describe",
			input:    "4.24.0-45-gabcdef",
			expected: "4.24.0+git.45.gabcdef",
			ok:       true,
		},
		{
			name:     "with prerelease",
			input:    "4.24.0-rc.3-45-gABCDEF",
			expected: "4.24.0-rc.3+git.45.gabcdef",
			ok:       true,
		},
		{
			name:     "with dirty flag",
			input:    "4.24.0-rc.3-45-gabc123-dirty",
			expected: "4.24.0-rc.3+git.45.gabc123.dirty",
			ok:       true,
		},
		{
			name:     "uppercase hash normalized",
			input:    "4.24.0-1-gABCDEF",
			expected: "4.24.0+git.1.gabcdef",
			ok:       true,
		},
		{
			name:     "dirty without prerelease",
			input:    "4.24.0-10-g1234567-dirty",
			expected: "4.24.0+git.10.g1234567.dirty",
			ok:       true,
		},

		// Invalid inputs
		{
			name:     "plain version",
			input:    "4.24.0",
			expected: "",
			ok:       false,
		},
		{
			name:     "version with prerelease only",
			input:    "4.24.0-rc.1",
			expected: "",
			ok:       false,
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
			ok:       false,
		},
		{
			name:     "branch name",
			input:    "feature-branch",
			expected: "",
			ok:       false,
		},
		{
			name:     "invalid base version",
			input:    "invalid-45-gabcdef",
			expected: "",
			ok:       false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := normalizeGitDescribeVersion(tc.input)
			if ok != tc.ok {
				t.Fatalf("normalizeGitDescribeVersion(%q) ok = %v, expected %v", tc.input, ok, tc.ok)
			}
			if got != tc.expected {
				t.Fatalf("normalizeGitDescribeVersion(%q) = %q, expected %q", tc.input, got, tc.expected)
			}
		})
	}
}
