package actionplanner

import (
	"encoding/json"
	"testing"

	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// These tests drive the partially-covered arms of validateParamType,
// validateParamValue, normalizeDataSources and normalizeChanges in planner.go.
// They assert on real returned values / errors / mutated state only.

func TestBranchcov0725amValidateParamType(t *testing.T) {
	spec := func(typ string) unified.CapabilityParam {
		return unified.CapabilityParam{Name: "mode", Type: typ}
	}
	steps := []struct {
		name    string
		value   any
		spec    unified.CapabilityParam
		wantErr bool
		wantMsg string
	}{
		// "" / "any" accept any value (including nil) and return nil.
		{"empty_type_accepts_string", "x", spec(""), false, ""},
		{"any_type_accepts_nil", nil, spec("any"), false, ""},

		// string
		{"string_accepts_string", "x", spec("string"), false, ""},
		{"string_rejects_int", 1, spec("string"), true, "parameter must be string"},
		{"string_rejects_bool", true, spec("string"), true, "parameter must be string"},

		// bool / boolean alias
		{"bool_accepts_bool", true, spec("bool"), false, ""},
		{"bool_rejects_string", "true", spec("bool"), true, "parameter must be bool"},
		{"boolean_alias_accepts_bool", false, spec("boolean"), false, ""},
		{"boolean_alias_rejects_int", 1, spec("boolean"), true, "parameter must be boolean"},

		// int / integer alias
		{"int_accepts_int", 5, spec("int"), false, ""},
		{"int_accepts_int64", int64(5), spec("int"), false, ""},
		{"int_rejects_float_fractional", 1.5, spec("int"), true, "parameter must be int"},
		{"int_rejects_string", "5", spec("int"), true, "parameter must be int"},
		{"integer_alias_accepts_int", 5, spec("integer"), false, ""},
		{"integer_alias_rejects_bool", true, spec("integer"), true, "parameter must be integer"},

		// number / float / float64 aliases
		{"number_accepts_float", 1.5, spec("number"), false, ""},
		{"number_accepts_int", 1, spec("number"), false, ""},
		{"number_rejects_string", "1.5", spec("number"), true, "parameter must be number"},
		{"float_alias_accepts_float", 1.5, spec("float"), false, ""},
		{"float64_alias_accepts_float", 1.5, spec("float64"), false, ""},
		{"float_alias_rejects_bool", true, spec("float"), true, "parameter must be float"},

		// object / map aliases
		{"object_accepts_map_string_any", map[string]any{"a": 1}, spec("object"), false, ""},
		{"object_accepts_map_int_string", map[int]string{1: "a"}, spec("object"), false, ""},
		{"object_rejects_string", "x", spec("object"), true, "parameter must be object"},
		{"map_alias_accepts_map_string_any", map[string]any{"a": 1}, spec("map"), false, ""},
		{"map_alias_rejects_slice", []int{1}, spec("map"), true, "parameter must be map"},

		// array / list aliases
		{"array_accepts_slice", []int{1}, spec("array"), false, ""},
		{"array_accepts_array", [3]int{1, 2, 3}, spec("array"), false, ""},
		{"array_rejects_map", map[string]any{"a": 1}, spec("array"), true, "parameter must be array"},
		{"list_alias_accepts_slice", []string{"a"}, spec("list"), false, ""},
		{"list_alias_rejects_string", "x", spec("list"), true, "parameter must be list"},

		// Case-insensitive + whitespace-trim normalization of the declared type
		// before the switch dispatches.
		{"uppercase_string_normalized_accepts", "x", spec("  STRING "), false, ""},
		{"mixedcase_integer_normalized_accepts", 1, spec("Integer"), false, ""},

		// Unknown type -> default arm.
		{"unknown_type_rejected", "x", spec("banana"), true, "capability declares unsupported parameter type banana"},
		{"unknown_type_uppercase_lowercased_in_message", "x", spec("  BANANA "), true, "capability declares unsupported parameter type banana"},
	}
	for _, step := range steps {
		t.Run(step.name, func(t *testing.T) {
			err := validateParamType(step.value, step.spec)
			if step.wantErr {
				if err == nil {
					t.Fatalf("validateParamType(%T, %q) = nil, want error", step.value, step.spec.Type)
				}
				ve, ok := AsValidationError(err)
				if !ok {
					t.Fatalf("validateParamType(%T, %q) error = %v, want *ValidationError", step.value, step.spec.Type, err)
				}
				if ve.Field != "params.mode" {
					t.Fatalf("Field = %q, want params.mode", ve.Field)
				}
				if step.wantMsg != "" && ve.Message != step.wantMsg {
					t.Fatalf("Message = %q, want %q", ve.Message, step.wantMsg)
				}
			} else if err != nil {
				t.Fatalf("validateParamType(%T, %q) = %v, want nil", step.value, step.spec.Type, err)
			}
		})
	}
}

func TestBranchcov0725amValidateParamValue(t *testing.T) {
	steps := []struct {
		name    string
		value   any
		spec    unified.CapabilityParam
		wantErr bool
		wantMsg string
	}{
		// validateParamType error propagates unchanged.
		{"type_error_propagated", 1, unified.CapabilityParam{Name: "mode", Type: "string"}, true, "parameter must be string"},

		// Enum arm: match via trimmed string.
		{"enum_string_match", "red", unified.CapabilityParam{Name: "color", Type: "string", Enum: []string{"red", "green"}}, false, ""},
		// Enum: candidate is whitespace-padded; match after TrimSpace.
		{"enum_matches_via_trimmed_candidate", "red", unified.CapabilityParam{Name: "color", Type: "any", Enum: []string{" red "}}, false, ""},
		// Enum: match via json.Number enumString().
		{"enum_json_number_match", json.Number("42"), unified.CapabilityParam{Name: "n", Type: "any", Enum: []string{"42"}}, false, ""},
		// Enum: match via fmt.Sprint default path (int value).
		{"enum_int_match", 42, unified.CapabilityParam{Name: "n", Type: "any", Enum: []string{"42"}}, false, ""},
		// Enum: no candidate matches.
		{"enum_no_match", "purple", unified.CapabilityParam{Name: "color", Type: "string", Enum: []string{"red", "green"}}, true, "parameter value is outside the allowed enum"},

		// Pattern arm.
		{"pattern_match", "abc", unified.CapabilityParam{Name: "slug", Type: "string", Pattern: "^[a-z]+$"}, false, ""},
		{"pattern_no_match", "ABC", unified.CapabilityParam{Name: "slug", Type: "string", Pattern: "^[a-z]+$"}, true, "parameter value does not match the required pattern"},
		// Pattern with a non-string value (type "any" passes type check first).
		{"pattern_requires_string_value", 42, unified.CapabilityParam{Name: "slug", Type: "any", Pattern: "^[0-9]+$"}, true, "pattern validation requires a string value"},
		// Pattern that fails to compile.
		{"pattern_invalid_regex", "abc", unified.CapabilityParam{Name: "slug", Type: "any", Pattern: "["}, true, "capability declares an invalid pattern"},

		// No enum, no pattern, type ok -> final nil return.
		{"plain_type_ok_no_enum_no_pattern", "hello", unified.CapabilityParam{Name: "greeting", Type: "string"}, false, ""},
	}
	for _, step := range steps {
		t.Run(step.name, func(t *testing.T) {
			err := validateParamValue(step.value, step.spec)
			if step.wantErr {
				if err == nil {
					t.Fatalf("validateParamValue(%T, %+v) = nil, want error", step.value, step.spec)
				}
				ve, ok := AsValidationError(err)
				if !ok {
					t.Fatalf("validateParamValue(%T, %+v) error = %v, want *ValidationError", step.value, step.spec, err)
				}
				if step.wantMsg != "" && ve.Message != step.wantMsg {
					t.Fatalf("Message = %q, want %q", ve.Message, step.wantMsg)
				}
			} else if err != nil {
				t.Fatalf("validateParamValue(%T, %+v) = %v, want nil", step.value, step.spec, err)
			}
		})
	}
}

func TestBranchcov0725amNormalizeDataSources(t *testing.T) {
	assertDataSources := func(t *testing.T, got, want []unified.DataSource) {
		t.Helper()
		if got == nil {
			t.Fatalf("normalizeDataSources returned nil, want non-nil slice")
		}
		if len(got) != len(want) {
			t.Fatalf("len = %d, want %d (got=%#v)", len(got), len(want), got)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("idx %d = %q, want %q (got=%#v)", i, got[i], want[i], got)
			}
		}
	}

	t.Run("nil_returns_nonnil_empty", func(t *testing.T) {
		got := normalizeDataSources(nil)
		assertDataSources(t, got, []unified.DataSource{})
	})

	t.Run("all_blank_and_whitespace_only_returns_nonnil_empty", func(t *testing.T) {
		got := normalizeDataSources([]unified.DataSource{"", "   ", "\t"})
		assertDataSources(t, got, []unified.DataSource{})
	})

	t.Run("blank_entries_dropped_keeping_valid", func(t *testing.T) {
		got := normalizeDataSources([]unified.DataSource{"", "   ", "\t", "proxmox"})
		assertDataSources(t, got, []unified.DataSource{"proxmox"})
	})

	t.Run("duplicates_after_case_and_trim_normalization_deduped_and_sorted", func(t *testing.T) {
		got := normalizeDataSources([]unified.DataSource{"Proxmox", "proxmox", " PROXMOX ", "docker"})
		assertDataSources(t, got, []unified.DataSource{"docker", "proxmox"})
	})

	t.Run("trim_and_lowercase_then_sorted", func(t *testing.T) {
		got := normalizeDataSources([]unified.DataSource{" Kubernetes ", "Docker", " proxmox"})
		assertDataSources(t, got, []unified.DataSource{"docker", "kubernetes", "proxmox"})
	})

	t.Run("already_sorted_unchanged", func(t *testing.T) {
		got := normalizeDataSources([]unified.DataSource{"agent", "docker"})
		assertDataSources(t, got, []unified.DataSource{"agent", "docker"})
	})
}

func TestBranchcov0725amNormalizeChanges(t *testing.T) {
	assertNormalized := func(t *testing.T, got []normalizedChange, want []normalizedChange) {
		t.Helper()
		if got == nil {
			t.Fatalf("normalizeChanges returned nil, want non-nil slice")
		}
		if len(got) != len(want) {
			t.Fatalf("len = %d, want %d (got=%#v)", len(got), len(want), got)
		}
		for i := range want {
			if got[i].ID != want[i].ID ||
				got[i].ResourceID != want[i].ResourceID ||
				got[i].Kind != want[i].Kind ||
				got[i].From != want[i].From ||
				got[i].To != want[i].To ||
				got[i].SourceType != want[i].SourceType ||
				got[i].SourceAdapter != want[i].SourceAdapter ||
				got[i].Confidence != want[i].Confidence {
				t.Fatalf("idx %d scalar fields = %+v, want %+v", i, got[i], want[i])
			}
			if len(got[i].RelatedResources) != len(want[i].RelatedResources) {
				t.Fatalf("idx %d RelatedResources len = %d, want %d", i, len(got[i].RelatedResources), len(want[i].RelatedResources))
			}
			for j := range want[i].RelatedResources {
				if got[i].RelatedResources[j] != want[i].RelatedResources[j] {
					t.Fatalf("idx %d RelatedResources[%d] = %q, want %q", i, j, got[i].RelatedResources[j], want[i].RelatedResources[j])
				}
			}
		}
	}

	t.Run("nil_returns_nonnil_empty", func(t *testing.T) {
		got := normalizeChanges(nil)
		assertNormalized(t, got, []normalizedChange{})
	})

	t.Run("single_change_trims_and_canonicalizes_fields", func(t *testing.T) {
		change := unified.ResourceChange{
			ID:               "  c1  ",
			ResourceID:       " vm:1 ",
			Kind:             unified.ChangeStateTransition,
			From:             " running ",
			To:               " stopped ",
			SourceType:       unified.SourcePlatformEvent,
			SourceAdapter:    unified.AdapterProxmox,
			Confidence:       unified.ConfidenceHigh,
			RelatedResources: []string{" vm:3 ", "vm:1", "vm:1"},
		}
		got := normalizeChanges([]unified.ResourceChange{change})
		want := []normalizedChange{{
			ID:               "c1",
			ResourceID:       "vm:1",
			Kind:             unified.ChangeStateTransition,
			From:             "running",
			To:               "stopped",
			SourceType:       unified.SourcePlatformEvent,
			SourceAdapter:    unified.AdapterProxmox,
			Confidence:       unified.ConfidenceHigh,
			RelatedResources: []string{"vm:1", "vm:3"},
		}}
		assertNormalized(t, got, want)
	})

	t.Run("sorted_by_resource_id", func(t *testing.T) {
		changes := []unified.ResourceChange{
			{ID: "a", ResourceID: "vm:9", Kind: unified.ChangeActivity},
			{ID: "a", ResourceID: "vm:1", Kind: unified.ChangeActivity},
		}
		got := normalizeChanges(changes)
		if got[0].ResourceID != "vm:1" || got[1].ResourceID != "vm:9" {
			t.Fatalf("order = [%s, %s], want vm:1 then vm:9", got[0].ResourceID, got[1].ResourceID)
		}
	})

	t.Run("same_resource_id_sorted_by_change_id", func(t *testing.T) {
		changes := []unified.ResourceChange{
			{ID: "c9", ResourceID: "vm:1", Kind: unified.ChangeActivity},
			{ID: "c1", ResourceID: "vm:1", Kind: unified.ChangeActivity},
		}
		got := normalizeChanges(changes)
		if got[0].ID != "c1" || got[1].ID != "c9" {
			t.Fatalf("order = [%s, %s], want c1 then c9", got[0].ID, got[1].ID)
		}
	})

	t.Run("same_resource_and_change_id_sorted_by_kind", func(t *testing.T) {
		changes := []unified.ResourceChange{
			{ID: "c1", ResourceID: "vm:1", Kind: unified.ChangeRestart},
			{ID: "c1", ResourceID: "vm:1", Kind: unified.ChangeActivity},
		}
		got := normalizeChanges(changes)
		if got[0].Kind != unified.ChangeActivity || got[1].Kind != unified.ChangeRestart {
			t.Fatalf("order = [%s, %s], want activity then restart", got[0].Kind, got[1].Kind)
		}
	})

	t.Run("blank_related_resources_dropped", func(t *testing.T) {
		change := unified.ResourceChange{
			ID:               "c1",
			ResourceID:       "vm:1",
			Kind:             unified.ChangeActivity,
			RelatedResources: []string{"", "   ", "vm:2"},
		}
		got := normalizeChanges([]unified.ResourceChange{change})
		if len(got[0].RelatedResources) != 1 || got[0].RelatedResources[0] != "vm:2" {
			t.Fatalf("RelatedResources = %#v, want [vm:2]", got[0].RelatedResources)
		}
	})
}
