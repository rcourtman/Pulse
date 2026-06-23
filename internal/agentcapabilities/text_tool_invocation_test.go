package agentcapabilities

import (
	"strings"
	"testing"
)

func TestIsTextToolInvocation(t *testing.T) {
	if !IsTextToolInvocation("pulse_control_guest(guest_id='102')") {
		t.Fatalf("expected Pulse tool invocation to be detected")
	}
	if !IsTextToolInvocation("default_api:pulse_get_resource(id='1')") {
		t.Fatalf("expected default_api-prefixed tool invocation to be detected")
	}
	if IsTextToolInvocation("echo hello") {
		t.Fatalf("expected non-tool command to be false")
	}
}

func TestParseTextToolInvocation(t *testing.T) {
	params, err := ParseTextToolInvocation("default_api:pulse_control_guest(guest_id=\"102\", action='start')")
	if err != nil {
		t.Fatalf("ParseTextToolInvocation returned error: %v", err)
	}
	if params.Name != "pulse_control_guest" {
		t.Fatalf("tool name = %q, want pulse_control_guest", params.Name)
	}
	if params.Arguments["guest_id"] != "102" || params.Arguments["action"] != "start" {
		t.Fatalf("unexpected arguments: %#v", params.Arguments)
	}

	params, err = ParseTextToolInvocation(" pulse_run_command ()")
	if err != nil {
		t.Fatalf("ParseTextToolInvocation empty args returned error: %v", err)
	}
	if params.Name != "pulse_run_command" || len(params.Arguments) != 0 {
		t.Fatalf("expected empty args, got %+v", params)
	}

	if _, err = ParseTextToolInvocation("(target='vm:101')"); err == nil || !strings.Contains(err.Error(), "tool name is required") {
		t.Fatalf("expected tool name validation error, got %v", err)
	}
	if _, err = ParseTextToolInvocation("pulse_control_guest"); err == nil {
		t.Fatalf("expected error for missing opening parenthesis")
	}
	if _, err = ParseTextToolInvocation("pulse_control_guest("); err == nil {
		t.Fatalf("expected error for missing closing parenthesis")
	}
}

func TestParseTextToolInvocationPreservesQuotedArgumentBoundaries(t *testing.T) {
	params, err := ParseTextToolInvocation("pulse_run_command(action='start', guest_id=\"102\", note='hello, world', path=\"/tmp/a,b\", escaped=\"\\\"quote\\\"\")")
	if err != nil {
		t.Fatalf("ParseTextToolInvocation returned error: %v", err)
	}

	expected := map[string]string{
		"action":   "start",
		"guest_id": "102",
		"note":     "hello, world",
		"path":     "/tmp/a,b",
		"escaped":  "\\\"quote\\",
	}
	for key, want := range expected {
		if got := params.Arguments[key]; got != want {
			t.Fatalf("argument %s = %q, want %q", key, got, want)
		}
	}
}

func TestCurrentResourceReferenceHelpersOwnSharedToolArgumentVocabulary(t *testing.T) {
	for _, value := range []string{
		"current_resource",
		" CURRENT_RESOURCE ",
		"attached_resource",
		"selected_resource",
		"this_resource",
		"redacted by policy",
	} {
		if !IsCurrentResourceReference(value) {
			t.Fatalf("IsCurrentResourceReference(%q) = false, want true", value)
		}
	}
	for _, value := range []string{"", "resource-1", "redacted-by-policy"} {
		if IsCurrentResourceReference(value) {
			t.Fatalf("IsCurrentResourceReference(%q) = true, want false", value)
		}
	}

	input := map[string]any{
		"body": map[string]any{
			"targets": []any{"vm:101", map[string]any{"resource_id": "selected_resource"}},
		},
	}
	if !ToolInputContainsCurrentResourceReference(input) {
		t.Fatalf("expected nested current_resource alias to be detected")
	}
	if ToolInputContainsCurrentResourceReference(map[string]any{
		"body": []any{"vm:101", map[string]any{"resource_id": "resource-2"}},
	}) {
		t.Fatalf("non-placeholder arguments should not be detected as current_resource references")
	}
}

func TestApprovalArgumentHelpers(t *testing.T) {
	args := WithApprovalArgument(nil, " approval-1 ")
	if got := ApprovalArgument(args); got != "approval-1" {
		t.Fatalf("ApprovalArgument = %q, want trimmed approval id", got)
	}
	if got := args[ApprovalArgumentKey]; got != "approval-1" {
		t.Fatalf("shared approval argument key stored %q", got)
	}

	empty := WithApprovalArgument(nil, " ")
	if empty != nil {
		t.Fatalf("empty approval id should not allocate args: %#v", empty)
	}
	if got := ApprovalArgument(map[string]any{ApprovalArgumentKey: 42}); got != "" {
		t.Fatalf("non-string approval argument = %q, want empty", got)
	}

	original := map[string]any{"resourceId": "vm:101"}
	withApproval := WithApprovalArgument(original, "approval-2")
	if _, mutated := original[ApprovalArgumentKey]; mutated {
		t.Fatalf("WithApprovalArgument mutated source args: %#v", original)
	}
	if got := ApprovalArgument(withApproval); got != "approval-2" {
		t.Fatalf("copied approval argument = %q, want approval-2", got)
	}
}

func TestCloneToolArgumentsDeepCopiesNestedValues(t *testing.T) {
	source := map[string]any{
		"body": map[string]any{
			"note": "maintenance",
			"tags": []any{"planned", "storage"},
		},
		"modes": []string{"dry-run", "apply"},
	}

	cloned := CloneToolArguments(source)
	clonedBody := cloned["body"].(map[string]any)
	clonedBody["note"] = "changed"
	clonedBody["tags"].([]any)[0] = "changed"
	cloned["modes"].([]string)[0] = "changed"

	sourceBody := source["body"].(map[string]any)
	if sourceBody["note"] != "maintenance" {
		t.Fatalf("nested map mutation leaked into source: %#v", sourceBody)
	}
	if sourceBody["tags"].([]any)[0] != "planned" {
		t.Fatalf("nested slice mutation leaked into source: %#v", sourceBody["tags"])
	}
	if source["modes"].([]string)[0] != "dry-run" {
		t.Fatalf("string slice mutation leaked into source: %#v", source["modes"])
	}
}

func TestPublicToolArgumentsDropsInternalMetadata(t *testing.T) {
	args := map[string]any{
		"resourceId":        "vm:101",
		"body":              map[string]any{"note": "maintenance"},
		ApprovalArgumentKey: "approval-1",
	}
	public := PublicToolArguments(args)
	if public["resourceId"] != "vm:101" {
		t.Fatalf("public arguments lost resource id: %#v", public)
	}
	if _, leaked := public[ApprovalArgumentKey]; leaked {
		t.Fatalf("public arguments leaked internal approval id: %#v", public)
	}
	public["resourceId"] = "vm:102"
	if args["resourceId"] != "vm:101" {
		t.Fatalf("PublicToolArguments returned aliased map: source=%#v public=%#v", args, public)
	}
	public["body"].(map[string]any)["note"] = "changed"
	if args["body"].(map[string]any)["note"] != "maintenance" {
		t.Fatalf("PublicToolArguments returned aliased nested body: source=%#v public=%#v", args, public)
	}
}

func TestSplitTextToolArguments(t *testing.T) {
	args := "action='start', guest_id=\"102\", note='hello, world', path=\"/tmp/a,b\", escaped=\"\\\"quote\\\"\""
	parts := splitTextToolArguments(args)
	expected := []string{
		"action='start'",
		"guest_id=\"102\"",
		"note='hello, world'",
		"path=\"/tmp/a,b\"",
		"escaped=\"\\\"quote\\\"\"",
	}
	if len(parts) != len(expected) {
		t.Fatalf("expected %d parts, got %d", len(expected), len(parts))
	}
	for i := range expected {
		if strings.TrimSpace(parts[i]) != expected[i] {
			t.Fatalf("part %d = %q, want %q", i, parts[i], expected[i])
		}
	}
}
