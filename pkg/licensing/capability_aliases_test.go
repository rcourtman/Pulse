package licensing

import (
	"testing"
	"time"
)

func TestResolveAlias(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected string
	}{
		{
			name:     "non_aliased_key_returns_unchanged",
			key:      "some_feature",
			expected: "some_feature",
		},
		{
			name:     "empty_key_returns_unchanged",
			key:      "",
			expected: "",
		},
	}

	originalAliases := LegacyAliases
	LegacyAliases = map[string]string{
		"old_feature": "new_feature",
	}
	defer func() { LegacyAliases = originalAliases }()

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveAlias(tt.key)
			if got != tt.expected {
				t.Fatalf("ResolveAlias(%q) = %q, want %q", tt.key, got, tt.expected)
			}
		})
	}
}

func TestResolveAlias_WithAliases(t *testing.T) {
	originalAliases := LegacyAliases
	LegacyAliases = map[string]string{
		"old_feature":   "new_feature",
		"legacy_api":    "v2_api",
		"deprecated_ai": "ai_v2",
	}
	defer func() { LegacyAliases = originalAliases }()

	tests := []struct {
		name     string
		key      string
		expected string
	}{
		{
			name:     "resolves_aliased_key",
			key:      "old_feature",
			expected: "new_feature",
		},
		{
			name:     "resolves_another_aliased_key",
			key:      "legacy_api",
			expected: "v2_api",
		},
		{
			name:     "non_aliased_key_unchanged",
			key:      "new_feature",
			expected: "new_feature",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveAlias(tt.key)
			if got != tt.expected {
				t.Fatalf("ResolveAlias(%q) = %q, want %q", tt.key, got, tt.expected)
			}
		})
	}
}

func TestIsDeprecated(t *testing.T) {
	tests := []struct {
		name              string
		key               string
		expectDeprecated  bool
		expectReplacement string
	}{
		{
			name:             "non_deprecated_key_returns_false",
			key:              "some_capability",
			expectDeprecated: false,
		},
		{
			name:             "empty_key_returns_false",
			key:              "",
			expectDeprecated: false,
		},
	}

	originalDeprecated := DeprecatedCapabilities
	DeprecatedCapabilities = map[string]DeprecatedCapability{
		"old_capability": {
			ReplacementKey: "new_capability",
			SunsetAt:       time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		},
	}
	defer func() { DeprecatedCapabilities = originalDeprecated }()

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			dep, got := IsDeprecated(tt.key)
			if got != tt.expectDeprecated {
				t.Fatalf("IsDeprecated(%q) = %v, want %v", tt.key, got, tt.expectDeprecated)
			}
			if tt.expectDeprecated && dep.ReplacementKey != tt.expectReplacement {
				t.Fatalf("IsDeprecated(%q).ReplacementKey = %q, want %q", tt.key, dep.ReplacementKey, tt.expectReplacement)
			}
		})
	}
}

func TestIsDeprecated_WithDeprecations(t *testing.T) {
	originalDeprecated := DeprecatedCapabilities
	DeprecatedCapabilities = map[string]DeprecatedCapability{
		"old_capability": {
			ReplacementKey: "new_capability",
			SunsetAt:       time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		},
		"legacy_feature": {
			ReplacementKey: "modern_feature",
			SunsetAt:       time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
		},
	}
	defer func() { DeprecatedCapabilities = originalDeprecated }()

	tests := []struct {
		name              string
		key               string
		expectDeprecated  bool
		expectReplacement string
		expectSunset      time.Time
	}{
		{
			name:              "finds_deprecated_capability",
			key:               "old_capability",
			expectDeprecated:  true,
			expectReplacement: "new_capability",
			expectSunset:      time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:              "finds_another_deprecated_capability",
			key:               "legacy_feature",
			expectDeprecated:  true,
			expectReplacement: "modern_feature",
			expectSunset:      time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:             "non_deprecated_key_not_found",
			key:              "new_capability",
			expectDeprecated: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			dep, got := IsDeprecated(tt.key)
			if got != tt.expectDeprecated {
				t.Fatalf("IsDeprecated(%q) = %v, want %v", tt.key, got, tt.expectDeprecated)
			}
			if tt.expectDeprecated {
				if dep.ReplacementKey != tt.expectReplacement {
					t.Fatalf("ReplacementKey = %q, want %q", dep.ReplacementKey, tt.expectReplacement)
				}
				if !dep.SunsetAt.Equal(tt.expectSunset) {
					t.Fatalf("SunsetAt = %v, want %v", dep.SunsetAt, tt.expectSunset)
				}
			}
		})
	}
}
