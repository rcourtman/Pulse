package actionplanner

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestIsIntegerBranchCoverage0718(t *testing.T) {
	steps := []struct {
		name  string
		value any
		want  bool
	}{
		{"int", int(42), true},
		{"int8", int8(42), true},
		{"int16", int16(42), true},
		{"int32", int32(42), true},
		{"int64", int64(42), true},
		{"uint", uint(42), true},
		{"uint8", uint8(42), true},
		{"uint16", uint16(42), true},
		{"uint32", uint32(42), true},
		{"uint64", uint64(42), true},
		{"float32_whole", float32(42), true},
		{"float32_fractional", float32(42.5), false},
		{"float64_whole", float64(42), true},
		{"float64_negative_whole", float64(-7), true},
		{"float64_fractional", float64(42.5), false},
		{"json_number_whole", json.Number("42"), true},
		{"json_number_negative_whole", json.Number("-7"), true},
		{"json_number_fractional", json.Number("42.5"), false},
		{"json_number_overflow", json.Number("9999999999999999999999"), false},
		{"string", "42", false},
		{"bool_true", true, false},
		{"nil", nil, false},
		{"map_string_any", map[string]any{"a": 1}, false},
		{"slice_int", []int{1}, false},
		{"struct", struct{}{}, false},
	}
	for _, step := range steps {
		t.Run(step.name, func(t *testing.T) {
			got := isInteger(step.value)
			if got != step.want {
				t.Fatalf("isInteger(%T %v) = %v, want %v", step.value, step.value, got, step.want)
			}
		})
	}
}

func TestIsNumberBranchCoverage0718(t *testing.T) {
	steps := []struct {
		name  string
		value any
		want  bool
	}{
		{"int", int(42), true},
		{"int8", int8(42), true},
		{"int16", int16(42), true},
		{"int32", int32(42), true},
		{"int64", int64(42), true},
		{"uint", uint(42), true},
		{"uint8", uint8(42), true},
		{"uint16", uint16(42), true},
		{"uint32", uint32(42), true},
		{"uint64", uint64(42), true},
		{"float32", float32(42.5), true},
		{"float64", float64(42.5), true},
		{"json_number", json.Number("42.5"), true},
		{"string", "42", false},
		{"bool_true", true, false},
		{"nil", nil, false},
		{"map_string_any", map[string]any{"a": 1}, false},
		{"slice_int", []int{1}, false},
		{"struct", struct{}{}, false},
	}
	for _, step := range steps {
		t.Run(step.name, func(t *testing.T) {
			got := isNumber(step.value)
			if got != step.want {
				t.Fatalf("isNumber(%T %v) = %v, want %v", step.value, step.value, got, step.want)
			}
		})
	}
}

func TestIsMapBranchCoverage0718(t *testing.T) {
	steps := []struct {
		name  string
		value any
		want  bool
	}{
		{"map_string_any", map[string]any{"a": 1}, true},
		{"empty_map_string_any", map[string]any{}, true},
		{"map_int_string", map[int]string{1: "a"}, true},
		{"map_string_string", map[string]string{"a": "b"}, true},
		{"nil", nil, false},
		{"slice_int", []int{1}, false},
		{"array", [3]int{1, 2, 3}, false},
		{"string", "foo", false},
		{"int", 42, false},
		{"bool_true", true, false},
		{"struct", struct{}{}, false},
	}
	for _, step := range steps {
		t.Run(step.name, func(t *testing.T) {
			got := isMap(step.value)
			if got != step.want {
				t.Fatalf("isMap(%T %v) = %v, want %v", step.value, step.value, got, step.want)
			}
		})
	}
}

func TestIsSliceBranchCoverage0718(t *testing.T) {
	steps := []struct {
		name  string
		value any
		want  bool
	}{
		{"slice_int", []int{1, 2}, true},
		{"slice_string", []string{"a"}, true},
		{"empty_slice", []int{}, true},
		{"nil_slice", []int(nil), true},
		{"array", [3]int{1, 2, 3}, true},
		{"nil", nil, false},
		{"map_string_any", map[string]any{"a": 1}, false},
		{"string", "foo", false},
		{"int", 42, false},
		{"bool_true", true, false},
		{"struct", struct{}{}, false},
	}
	for _, step := range steps {
		t.Run(step.name, func(t *testing.T) {
			got := isSlice(step.value)
			if got != step.want {
				t.Fatalf("isSlice(%T %v) = %v, want %v", step.value, step.value, got, step.want)
			}
		})
	}
}

func TestSortedCanonicalResourceIDsBranchCoverage0718(t *testing.T) {
	t.Run("nil_input_returns_nonnil_empty", func(t *testing.T) {
		got := sortedCanonicalResourceIDs(nil)
		if got == nil {
			t.Fatalf("sortedCanonicalResourceIDs(nil) = nil, want non-nil empty slice")
		}
		if len(got) != 0 {
			t.Fatalf("sortedCanonicalResourceIDs(nil) = %#v, want empty", got)
		}
	})
	t.Run("empty_input_returns_nonnil_empty", func(t *testing.T) {
		got := sortedCanonicalResourceIDs([]string{})
		if got == nil {
			t.Fatalf("sortedCanonicalResourceIDs([]) = nil, want non-nil empty slice")
		}
		if len(got) != 0 {
			t.Fatalf("sortedCanonicalResourceIDs([]) = %#v, want empty", got)
		}
	})
	t.Run("all_blank_input_returns_nonnil_empty", func(t *testing.T) {
		got := sortedCanonicalResourceIDs([]string{"", "   ", "\t"})
		if got == nil || len(got) != 0 {
			t.Fatalf("sortedCanonicalResourceIDs(%#v) = %#v, want non-nil empty slice", []string{"", "   ", "\t"}, got)
		}
	})
	t.Run("sorts_dedupes_and_trims", func(t *testing.T) {
		input := []string{" vm:3 ", "vm:1", "vm:2", "vm:1", "", "   ", "vm:3"}
		got := sortedCanonicalResourceIDs(input)
		want := []string{"vm:1", "vm:2", "vm:3"}
		if len(got) != len(want) {
			t.Fatalf("len = %d, want %d (got=%#v)", len(got), len(want), got)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("idx %d = %q, want %q (got=%#v)", i, got[i], want[i], got)
			}
		}
	})
	t.Run("already_sorted_unchanged", func(t *testing.T) {
		got := sortedCanonicalResourceIDs([]string{"vm:1", "vm:2", "vm:3"})
		want := []string{"vm:1", "vm:2", "vm:3"}
		if len(got) != len(want) {
			t.Fatalf("len = %d, want %d", len(got), len(want))
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("idx %d = %q, want %q", i, got[i], want[i])
			}
		}
	})
	t.Run("reverse_order_sorted", func(t *testing.T) {
		got := sortedCanonicalResourceIDs([]string{"vm:3", "vm:2", "vm:1"})
		want := []string{"vm:1", "vm:2", "vm:3"}
		if len(got) != len(want) {
			t.Fatalf("len = %d, want %d", len(got), len(want))
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("idx %d = %q, want %q", i, got[i], want[i])
			}
		}
	})
}

func TestPlanWithRequirementBranchCoverage0718(t *testing.T) {
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	resource := unified.Resource{
		ID:     "vm:42",
		Type:   unified.ResourceTypeVM,
		Name:   "web-42",
		Status: unified.StatusOnline,
		Capabilities: []unified.ResourceCapability{{
			Name:                 "restart",
			Type:                 unified.CapabilityTypeCommon,
			Description:          "Restart the VM",
			MinimumApprovalLevel: unified.ApprovalAdmin,
		}},
	}
	actor := unified.ActionActor{
		SubjectID:    "agent:oncall-helper",
		Kind:         unified.ActionActorService,
		CredentialID: "service:test",
		OrgID:        "default",
	}

	t.Run("happy_path_floor_requirement_deterministic", func(t *testing.T) {
		req := unified.ActionRequest{
			RequestID:      "agent-run-req-floor",
			ResourceID:     "vm:42",
			CapabilityName: "restart",
			Reason:         "recover",
			Actor:          actor,
		}
		planner := Planner{Now: func() time.Time { return now }}
		plan, err := planner.PlanWithRequirement(req, resource, unified.ApprovalRequirement{})
		if err != nil {
			t.Fatalf("PlanWithRequirement() error = %v", err)
		}
		if plan.ActionID == "" {
			t.Fatal("ActionID is empty")
		}
		if plan.PlanHash == "" {
			t.Fatal("PlanHash is empty")
		}
		if !plan.Allowed {
			t.Fatal("Allowed = false, want true")
		}
		if !plan.RequiresApproval {
			t.Fatal("RequiresApproval = false, want true for ApprovalAdmin floor")
		}
		if plan.ApprovalPolicy != unified.ApprovalAdmin {
			t.Fatalf("ApprovalPolicy = %q, want %q", plan.ApprovalPolicy, unified.ApprovalAdmin)
		}
		if plan.Preflight == nil {
			t.Fatal("Preflight is nil")
		}
		if plan.Preflight.Target != "vm:42" {
			t.Fatalf("Preflight.Target = %q, want vm:42", plan.Preflight.Target)
		}
		if !plan.PlannedAt.Equal(now) {
			t.Fatalf("PlannedAt = %s, want %s", plan.PlannedAt, now)
		}
		if !plan.ExpiresAt.Equal(now.Add(DefaultPlanTTL)) {
			t.Fatalf("ExpiresAt = %s, want %s", plan.ExpiresAt, now.Add(DefaultPlanTTL))
		}
		again, err := planner.PlanWithRequirement(req, resource, unified.ApprovalRequirement{})
		if err != nil {
			t.Fatalf("second call error = %v", err)
		}
		if again.ActionID != plan.ActionID || again.PlanHash != plan.PlanHash {
			t.Fatalf("plan not deterministic: first=(%q,%q) second=(%q,%q)",
				plan.ActionID, plan.PlanHash, again.ActionID, again.PlanHash)
		}
	})

	t.Run("happy_path_explicit_requirement_version_used", func(t *testing.T) {
		req := unified.ActionRequest{
			RequestID:      "agent-run-req-explicit",
			ResourceID:     "vm:42",
			CapabilityName: "restart",
			Reason:         "recover",
			Actor:          actor,
		}
		requested := unified.ApprovalRequirement{
			Version: 1,
			Floor:   unified.ApprovalAdmin,
		}
		planner := Planner{Now: func() time.Time { return now }}
		plan, err := planner.PlanWithRequirement(req, resource, requested)
		if err != nil {
			t.Fatalf("PlanWithRequirement() error = %v", err)
		}
		if plan.ApprovalRequirement.Version != requested.Version {
			t.Fatalf("plan requirement Version = %d, want %d", plan.ApprovalRequirement.Version, requested.Version)
		}
		if plan.ActionID == "" || plan.PlanHash == "" {
			t.Fatalf("ActionID/PlanHash not populated: %#v", plan)
		}
	})

	t.Run("missing_request_id_returns_validation_error", func(t *testing.T) {
		req := unified.ActionRequest{
			ResourceID:     "vm:42",
			CapabilityName: "restart",
			Reason:         "recover",
			Actor:          actor,
		}
		planner := Planner{Now: func() time.Time { return now }}
		_, err := planner.PlanWithRequirement(req, resource, unified.ApprovalRequirement{})
		validationErr, ok := AsValidationError(err)
		if !ok {
			t.Fatalf("PlanWithRequirement() error = %v, want validation error", err)
		}
		if validationErr.Field != "requestId" {
			t.Fatalf("validation field = %q, want requestId", validationErr.Field)
		}
	})

	t.Run("resource_id_mismatch_returns_validation_error", func(t *testing.T) {
		req := unified.ActionRequest{
			RequestID:      "agent-run-req-mismatch",
			ResourceID:     "vm:99",
			CapabilityName: "restart",
			Reason:         "recover",
			Actor:          actor,
		}
		planner := Planner{Now: func() time.Time { return now }}
		_, err := planner.PlanWithRequirement(req, resource, unified.ApprovalRequirement{})
		validationErr, ok := AsValidationError(err)
		if !ok {
			t.Fatalf("error = %v, want validation error", err)
		}
		if validationErr.Field != "resourceId" {
			t.Fatalf("validation field = %q, want resourceId", validationErr.Field)
		}
		if !strings.Contains(validationErr.Message, "does not match") {
			t.Fatalf("validation message = %q, want substring 'does not match'", validationErr.Message)
		}
	})

	t.Run("missing_capability_returns_capability_not_found", func(t *testing.T) {
		req := unified.ActionRequest{
			RequestID:      "agent-run-req-no-cap",
			ResourceID:     "vm:42",
			CapabilityName: "restart",
			Reason:         "recover",
			Actor:          actor,
		}
		emptyResource := unified.Resource{ID: "vm:42", Type: unified.ResourceTypeVM}
		planner := Planner{Now: func() time.Time { return now }}
		_, err := planner.PlanWithRequirement(req, emptyResource, unified.ApprovalRequirement{})
		if !errors.Is(err, ErrCapabilityNotFound) {
			t.Fatalf("error = %v, want ErrCapabilityNotFound", err)
		}
	})

	t.Run("resource_without_canonical_id_returns_validation_error", func(t *testing.T) {
		req := unified.ActionRequest{
			RequestID:      "agent-run-req-no-canonical",
			ResourceID:     "vm:42",
			CapabilityName: "restart",
			Reason:         "recover",
			Actor:          actor,
		}
		blankResource := unified.Resource{Type: unified.ResourceTypeVM}
		planner := Planner{Now: func() time.Time { return now }}
		_, err := planner.PlanWithRequirement(req, blankResource, unified.ApprovalRequirement{})
		validationErr, ok := AsValidationError(err)
		if !ok {
			t.Fatalf("error = %v, want validation error", err)
		}
		if validationErr.Field != "resourceId" {
			t.Fatalf("validation field = %q, want resourceId", validationErr.Field)
		}
		if !strings.Contains(validationErr.Message, "no canonical id") {
			t.Fatalf("validation message = %q, want substring 'no canonical id'", validationErr.Message)
		}
	})
}
