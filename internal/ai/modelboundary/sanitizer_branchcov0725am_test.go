package modelboundary

import (
	"reflect"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// branchcov0725amByTypeOnlyProvider implements only
// UnifiedResourceProvider.GetByType and intentionally NOT allUnifiedResourceProvider.GetAll,
// forcing resourcePolicySanitizerResources down its per-type aggregation loop instead of the
// GetAll fast-path. The slice returned for each type is fully controlled so the dedup branch
// (same Type+ID emitted twice) can be exercised deterministically.
type branchcov0725amByTypeOnlyProvider struct {
	byType map[unifiedresources.ResourceType][]unifiedresources.Resource
}

func (p branchcov0725amByTypeOnlyProvider) GetByType(t unifiedresources.ResourceType) []unifiedresources.Resource {
	return append([]unifiedresources.Resource(nil), p.byType[t]...)
}

// TestBranchcov0725amSortStringsByLengthDesc drives both arms of the sort comparator:
// the equal-length tie branch (alphabetical ascending) and the length branch (descending by
// length), plus the empty / single / already-sorted degenerate inputs.
func TestBranchcov0725amSortStringsByLengthDesc(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{
			name: "nil slice unchanged",
			in:   nil,
			want: nil,
		},
		{
			name: "empty non-nil slice unchanged",
			in:   []string{},
			want: []string{},
		},
		{
			name: "single element unchanged",
			in:   []string{"solo"},
			want: []string{"solo"},
		},
		{
			name: "ties broken alphabetically ascending",
			in:   []string{"cat", "bat", "ant"},
			want: []string{"ant", "bat", "cat"},
		},
		{
			name: "distinct lengths sorted descending by length",
			in:   []string{"a", "ccc", "bb"},
			want: []string{"ccc", "bb", "a"},
		},
		{
			name: "mixed distinct lengths and ties",
			in:   []string{"z", "bbb", "aaa", "dddd"},
			want: []string{"dddd", "aaa", "bbb", "z"},
		},
		{
			name: "already sorted input is stable",
			in:   []string{"dddd", "aaa", "bbb", "z"},
			want: []string{"dddd", "aaa", "bbb", "z"},
		},
		{
			name: "fully reverse input is sorted",
			in:   []string{"z", "bbb", "aaa", "dddd"},
			want: []string{"dddd", "aaa", "bbb", "z"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.in
			sortStringsByLengthDesc(got)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("sortStringsByLengthDesc(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

// TestBranchcov0725amResourcePolicySanitizerResources drives the per-type aggregation loop of
// resourcePolicySanitizerResources (the branch taken when the provider does NOT implement
// GetAll), including the Type+ID dedup continue and the final RefreshCanonicalMetadataSlice +
// resourcesWithPolicy return.
func TestBranchcov0725amResourcePolicySanitizerResources(t *testing.T) {
	t.Run("by_type_only_provider_dedups_and_refreshes_policy", func(t *testing.T) {
		vm := unifiedresources.Resource{
			ID:   "vm-100",
			Type: unifiedresources.ResourceTypeVM,
			Name: "vm1",
		}
		// Duplicated VM entry with identical Type+ID: the second must be dropped by the
		// `seen` map (dedup continue branch).
		vmDup := unifiedresources.Resource{
			ID:   "vm-100",
			Type: unifiedresources.ResourceTypeVM,
			Name: "vm1-duplicate-name",
		}
		agent := unifiedresources.Resource{
			ID:   "agent/neo",
			Type: unifiedresources.ResourceTypeAgent,
			Name: "neo",
		}
		provider := branchcov0725amByTypeOnlyProvider{
			byType: map[unifiedresources.ResourceType][]unifiedresources.Resource{
				unifiedresources.ResourceTypeVM:    {vm, vmDup},
				unifiedresources.ResourceTypeAgent: {agent},
			},
		}

		got := resourcePolicySanitizerResources(provider)

		if len(got) != 2 {
			t.Fatalf("expected 2 resources after dedup, got %d: %+v", len(got), got)
		}

		// ResourceTypeAgent is iterated before ResourceTypeVM, so the agent is collected first.
		if got[0].ID != "agent/neo" || got[0].Type != unifiedresources.ResourceTypeAgent {
			t.Fatalf("first resource = %+v, want agent/neo", got[0])
		}
		if got[1].ID != "vm-100" || got[1].Type != unifiedresources.ResourceTypeVM {
			t.Fatalf("second resource = %+v, want vm-100", got[1])
		}
		// The duplicate's name must not have survived dedup.
		if got[1].Name == "vm1-duplicate-name" {
			t.Fatalf("dedup did not drop the duplicate vm-100 entry: %+v", got[1])
		}
		// RefreshCanonicalMetadataSlice must have populated a non-nil Policy on each kept
		// resource (resourcesWithPolicy otherwise filters Policy==nil entries).
		for i, r := range got {
			if r.Policy == nil {
				t.Fatalf("resource %d (%s) has nil Policy after refresh", i, r.ID)
			}
		}
	})

	t.Run("by_type_only_empty_provider_returns_empty_non_nil", func(t *testing.T) {
		got := resourcePolicySanitizerResources(branchcov0725amByTypeOnlyProvider{})
		// resourcesWithPolicy allocates a non-nil empty slice even for no input.
		if got == nil {
			t.Fatalf("expected non-nil empty slice, got nil")
		}
		if len(got) != 0 {
			t.Fatalf("expected 0 resources, got %d: %+v", len(got), got)
		}
	})
}

// TestBranchcov0725amSanitizePromptSecretValue drives every type arm of sanitizePromptSecretValue
// that existing tests leave uncovered: the []string sensitive path, the full map[string]string
// arm (sensitive and non-sensitive sub-arms under both parent-sensitive and parent-non-sensitive),
// and the default (passthrough) arm. The string/sensitive path is also exercised at the
// RedactSensitiveValue length boundary: below minimum (empty), at the boundary (whitespace-only),
// and well above (real content -> [REDACTED]).
func TestBranchcov0725amSanitizePromptSecretValue(t *testing.T) {
	t.Run("string_sensitive_length_boundaries", func(t *testing.T) {
		cases := []struct {
			name  string
			value string
			want  string
		}{
			{name: "below_minimum_empty_returns_empty", value: "", want: ""},
			{name: "boundary_whitespace_only_preserved", value: "   ", want: "   "},
			{name: "well_above_real_value_redacted", value: "real-secret", want: "[REDACTED]"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				got := sanitizePromptSecretValue("password", tc.value, true)
				if got != tc.want {
					t.Fatalf("sanitizePromptSecretValue(string,%q,sensitive) = %#v, want %q", tc.value, got, tc.want)
				}
			})
		}
	})

	t.Run("string_slice_sensitive_redacts_each_element", func(t *testing.T) {
		got := sanitizePromptSecretValue("password", []string{"", "real-secret"}, true)
		want := []string{"", "[REDACTED]"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %#v, want %#v", got, want)
		}
	})

	t.Run("string_slice_non_sensitive_uses_text_path", func(t *testing.T) {
		// Under the non-sensitive arm, an OpenAI-shaped token is pattern-redacted but a plain
		// value is left intact — distinguishing the text path from the always-redacted
		// sensitive path.
		got := sanitizePromptSecretValue("tags", []string{"sk-aaaaaaaaaaaaaaaa", "plain"}, false)
		want := []string{"[REDACTED_PROVIDER_KEY]", "plain"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %#v, want %#v", got, want)
		}
	})

	t.Run("map_string_string_parent_sensitive_distinguishes_carrier_keys", func(t *testing.T) {
		// parent sensitiveValue=true: a value-carrier key ("value") and a sensitive key
		// ("password") both route to the sensitive-value redactor, while a non-carrier key
		// ("note") routes to the plain-text redactor.
		got := sanitizePromptSecretValue(
			"",
			map[string]string{"value": "carrier-secret", "password": "cred", "note": "keep-me"},
			true,
		).(map[string]string)
		if got["value"] != "[REDACTED]" {
			t.Fatalf("carrier key 'value' = %q, want [REDACTED]", got["value"])
		}
		if got["password"] != "[REDACTED]" {
			t.Fatalf("sensitive key 'password' = %q, want [REDACTED]", got["password"])
		}
		if got["note"] != "keep-me" {
			t.Fatalf("non-carrier key 'note' = %q, want keep-me", got["note"])
		}
	})

	t.Run("map_string_string_parent_non_sensitive_skips_carrier_term", func(t *testing.T) {
		// parent sensitiveValue=false: the `sensitiveValue && carrier` term is false, so a
		// carrier key ("value") is NOT treated as sensitive and flows through the text path.
		got := sanitizePromptSecretValue(
			"",
			map[string]string{"value": "keep-this", "password": "cred"},
			false,
		).(map[string]string)
		if got["value"] != "keep-this" {
			t.Fatalf("carrier key 'value' under non-sensitive parent = %q, want keep-this", got["value"])
		}
		if got["password"] != "[REDACTED]" {
			t.Fatalf("sensitive key 'password' = %q, want [REDACTED]", got["password"])
		}
	})

	t.Run("default_arm_passes_through_non_string_scalars", func(t *testing.T) {
		for _, v := range []interface{}{42, true, 3.14, nil} {
			got := sanitizePromptSecretValue("count", v, true)
			if !reflect.DeepEqual(got, v) {
				t.Fatalf("default passthrough for %#v = %#v, want identity", v, got)
			}
		}
	})

	t.Run("interface_slice_recurses_into_default_element", func(t *testing.T) {
		// A non-string element inside []interface{} recurses through sanitizePromptSecretValue
		// into the default (passthrough) arm while leaving string elements text-redacted.
		got := sanitizePromptSecretValue("items", []interface{}{"plain", 7}, false).([]interface{})
		if got[0] != "plain" {
			t.Fatalf("string element = %#v, want plain", got[0])
		}
		if got[1] != 7 {
			t.Fatalf("int element = %#v, want 7 (passthrough)", got[1])
		}
	})
}
