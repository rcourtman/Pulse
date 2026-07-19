package agentcapabilities

import (
	"testing"
)

// TestFirstStringPayloadValueAgentCapsBranchcov0719late covers the unexported
// firstStringPayloadValue helper across all of its branches: key present and a
// non-empty string (with surrounding whitespace that must be trimmed), key
// present but a non-string type, key present but an empty/whitespace-only
// string, key absent entirely, multiple keys where a later key is the first
// usable match, and the no-match empty return (including the degenerate
// no-keys case).
func TestFirstStringPayloadValueAgentCapsBranchcov0719late(t *testing.T) {
	cases := []struct {
		name    string
		payload map[string]any
		keys    []string
		want    string
	}{
		{
			name:    "key present as non-empty string returns trimmed value",
			payload: map[string]any{"command": "  reboot host  "},
			keys:    []string{"command"},
			want:    "reboot host",
		},
		{
			name:    "key present as non-empty string with no surrounding whitespace passes through",
			payload: map[string]any{"command": "reboot host"},
			keys:    []string{"command"},
			want:    "reboot host",
		},
		{
			name:    "key present but wrong type int is skipped",
			payload: map[string]any{"command": 42},
			keys:    []string{"command"},
			want:    "",
		},
		{
			name:    "key present but wrong type bool is skipped",
			payload: map[string]any{"command": true},
			keys:    []string{"command"},
			want:    "",
		},
		{
			name:    "key present but wrong type slice is skipped",
			payload: map[string]any{"command": []string{"a", "b"}},
			keys:    []string{"command"},
			want:    "",
		},
		{
			name:    "key present but empty string is skipped",
			payload: map[string]any{"command": ""},
			keys:    []string{"command"},
			want:    "",
		},
		{
			name:    "key present but whitespace-only string is skipped",
			payload: map[string]any{"command": "   "},
			keys:    []string{"command"},
			want:    "",
		},
		{
			name:    "key absent entirely returns empty",
			payload: map[string]any{"unrelated": "value"},
			keys:    []string{"command"},
			want:    "",
		},
		{
			name:    "nil payload returns empty",
			payload: nil,
			keys:    []string{"command"},
			want:    "",
		},
		{
			name:    "multiple keys where first key matches returns first",
			payload: map[string]any{"command": "first", "reason": "second"},
			keys:    []string{"command", "reason"},
			want:    "first",
		},
		{
			name:    "multiple keys where later key matches after wrong type returns later",
			payload: map[string]any{"command": 42, "reason": "real reason"},
			keys:    []string{"command", "reason"},
			want:    "real reason",
		},
		{
			name:    "multiple keys where later key matches after whitespace-only returns later",
			payload: map[string]any{"command": "   ", "reason": "real reason"},
			keys:    []string{"command", "reason"},
			want:    "real reason",
		},
		{
			name:    "multiple keys where later key matches after absent first returns later",
			payload: map[string]any{"reason": "real reason"},
			keys:    []string{"command", "reason"},
			want:    "real reason",
		},
		{
			name:    "multiple keys all wrong type returns empty",
			payload: map[string]any{"command": 1, "reason": true},
			keys:    []string{"command", "reason"},
			want:    "",
		},
		{
			name:    "multiple keys all whitespace-only returns empty",
			payload: map[string]any{"command": " ", "reason": "\t\n"},
			keys:    []string{"command", "reason"},
			want:    "",
		},
		{
			name:    "no keys provided returns empty",
			payload: map[string]any{"command": "value"},
			keys:    nil,
			want:    "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := firstStringPayloadValue(tc.payload, tc.keys...)
			if got != tc.want {
				t.Fatalf("firstStringPayloadValue(%v, %v) = %q, want %q", tc.payload, tc.keys, got, tc.want)
			}
		})
	}
}

// TestMCPManifestPromptProjectionSupportedAgentCapsBranchcov0719late covers both arms
// of MCPManifestPromptProjectionSupported and its unexported helper
// mcpManifestPromptProjectionSupported: a manifest whose Pulse workflow prompts
// include at least one prompt with a non-empty trimmed Name returns true, while
// a manifest with no workflow prompts or only blank-named prompts returns false.
func TestMCPManifestPromptProjectionSupportedAgentCapsBranchcov0719late(t *testing.T) {
	t.Run("exported returns true when a workflow prompt has a non-empty name", func(t *testing.T) {
		manifest := Manifest{
			WorkflowPrompts: []PulseWorkflowPrompt{
				{Name: "triage_fleet", Description: "Triage the fleet"},
			},
		}
		if !MCPManifestPromptProjectionSupported(manifest) {
			t.Fatalf("MCPManifestPromptProjectionSupported = false, want true for manifest with a named workflow prompt")
		}
	})

	t.Run("exported returns true when only one of several prompts has a non-empty name", func(t *testing.T) {
		manifest := Manifest{
			WorkflowPrompts: []PulseWorkflowPrompt{
				{Name: "   "},
				{Name: ""},
				{Name: "review_finding"},
			},
		}
		if !MCPManifestPromptProjectionSupported(manifest) {
			t.Fatalf("MCPManifestPromptProjectionSupported = false, want true when any prompt has a non-blank name")
		}
	})

	t.Run("exported returns false when no workflow prompts are declared", func(t *testing.T) {
		manifest := Manifest{}
		if MCPManifestPromptProjectionSupported(manifest) {
			t.Fatalf("MCPManifestPromptProjectionSupported = true, want false for manifest without workflow prompts")
		}
	})

	t.Run("exported returns false when workflow prompts only have blank names", func(t *testing.T) {
		manifest := Manifest{
			WorkflowPrompts: []PulseWorkflowPrompt{
				{Name: "   "},
				{Name: ""},
			},
		}
		if MCPManifestPromptProjectionSupported(manifest) {
			t.Fatalf("MCPManifestPromptProjectionSupported = true, want false when only blank-named prompts exist")
		}
	})

	t.Run("exported returns true for a name that needs trimming", func(t *testing.T) {
		manifest := Manifest{
			WorkflowPrompts: []PulseWorkflowPrompt{
				{Name: "  investigate_resource  "},
			},
		}
		if !MCPManifestPromptProjectionSupported(manifest) {
			t.Fatalf("MCPManifestPromptProjectionSupported = false, want true for a name that becomes non-empty after trimming")
		}
	})

	t.Run("unexported helper returns true when a workflow prompt has a non-empty name", func(t *testing.T) {
		manifest := Manifest{
			WorkflowPrompts: []PulseWorkflowPrompt{
				{Name: "operations_loop"},
			},
		}
		if !mcpManifestPromptProjectionSupported(manifest) {
			t.Fatalf("mcpManifestPromptProjectionSupported = false, want true")
		}
	})

	t.Run("unexported helper returns false when no prompts are declared", func(t *testing.T) {
		manifest := Manifest{}
		if mcpManifestPromptProjectionSupported(manifest) {
			t.Fatalf("mcpManifestPromptProjectionSupported = true, want false for empty manifest")
		}
	})

	t.Run("unexported helper returns false when only blank-named prompts exist", func(t *testing.T) {
		manifest := Manifest{
			WorkflowPrompts: []PulseWorkflowPrompt{
				{Name: "  "},
			},
		}
		if mcpManifestPromptProjectionSupported(manifest) {
			t.Fatalf("mcpManifestPromptProjectionSupported = true, want false for blank-named prompts")
		}
	})
}

// TestJSONRPCErrorErrorAgentCapsBranchcov0719late covers JSONRPCError.Error() across its
// two formatting branches: the populated-message arm (returns the message
// verbatim) and the empty-message / zero-state arm (returns the code-formatted
// fallback). Representative codes from the JSON-RPC error vocabulary are used
// alongside the fully zero value.
func TestJSONRPCErrorErrorAgentCapsBranchcov0719late(t *testing.T) {
	cases := []struct {
		name string
		err  JSONRPCError
		want string
	}{
		{
			name: "representative populated message and code returns the message verbatim",
			err:  JSONRPCError{Code: JSONRPCErrorMethodNotFound, Message: "method not found: tools/call"},
			want: "method not found: tools/call",
		},
		{
			name: "parse error code with populated message returns the message verbatim",
			err:  JSONRPCError{Code: JSONRPCErrorParse, Message: "malformed JSON-RPC request: unexpected EOF"},
			want: "malformed JSON-RPC request: unexpected EOF",
		},
		{
			name: "empty message with representative code returns code-formatted fallback",
			err:  JSONRPCError{Code: JSONRPCErrorInternal, Message: ""},
			want: "json-rpc error -32603",
		},
		{
			name: "zero value returns code-formatted fallback with code zero",
			err:  JSONRPCError{},
			want: "json-rpc error 0",
		},
		{
			name: "populated message ignores the code entirely",
			err:  JSONRPCError{Code: JSONRPCErrorInternal, Message: "tools/call handler unavailable"},
			want: "tools/call handler unavailable",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.err.Error()
			if got != tc.want {
				t.Fatalf("JSONRPCError{Code: %d, Message: %q}.Error() = %q, want %q", tc.err.Code, tc.err.Message, got, tc.want)
			}
		})
	}
}
