package agentcapabilities

import (
	"strings"
	"testing"
)

// TestSubstitutePathParametersTable exercises every branch of
// SubstitutePathParameters: the no-placeholder passthrough, the happy
// substitution path (single, multiple, and repeated placeholders), the
// missing-argument error path, the non-string-argument error path, and the
// combined missing + non-string collection path (whose order is deterministic
// because ReplaceAllStringFunc walks placeholder matches left-to-right rather
// than iterating the args map). It also pins regex edge cases (placeholder
// names that the {a-zA-Z][a-zA-Z0-9]*} grammar rejects are left literal) and
// the end-to-end percent-escaping behavior surfaced through
// EscapePathSegmentParameter.
func TestSubstitutePathParametersTable(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		args      map[string]any
		wantPath  string
		wantErr   bool
		errSubstr string
	}{
		{
			name:     "no placeholders returns path unchanged",
			path:     "/api/agent/patrol-control/status",
			args:     map[string]any{"resourceId": "ignored"},
			wantPath: "/api/agent/patrol-control/status",
		},
		{
			name:     "empty path returns empty string with no error",
			path:     "",
			args:     nil,
			wantPath: "",
		},
		{
			name:     "nil args with no placeholders returns path unchanged",
			path:     "/static/route",
			args:     nil,
			wantPath: "/static/route",
		},
		{
			name:      "nil args map with placeholder reported missing",
			path:      "/api/resources/{resourceId}",
			args:      nil,
			wantErr:   true,
			errSubstr: "missing path argument(s): resourceId",
		},
		{
			name:      "empty args map with placeholder reported missing",
			path:      "/api/resources/{resourceId}/operator-state",
			args:      map[string]any{},
			wantErr:   true,
			errSubstr: "missing path argument(s): resourceId",
		},
		{
			name:     "single placeholder substituted and escaped",
			path:     "/api/resources/{resourceId}/operator-state",
			args:     map[string]any{"resourceId": "vm:101"},
			wantPath: "/api/resources/vm%3A101/operator-state",
		},
		{
			name:     "multiple placeholders substituted in match order",
			path:     "/api/config/nodes/{nodeId}/test/{resourceId}",
			args:     map[string]any{"nodeId": "pve/lab node", "resourceId": "vm:101"},
			wantPath: "/api/config/nodes/pve%2Flab%20node/test/vm%3A101",
		},
		{
			name:     "repeated placeholder substituted at every occurrence",
			path:     "/{a}/and/{a}",
			args:     map[string]any{"a": "x:y"},
			wantPath: "/x%3Ay/and/x%3Ay",
		},
		{
			name:      "integer arg reported as not a string",
			path:      "/api/actions/{actionId}/decision",
			args:      map[string]any{"actionId": 42},
			wantErr:   true,
			errSubstr: "actionId (not a string)",
		},
		{
			name:      "bool arg reported as not a string",
			path:      "/api/actions/{actionId}/decision",
			args:      map[string]any{"actionId": true},
			wantErr:   true,
			errSubstr: "actionId (not a string)",
		},
		{
			name:      "map arg reported as not a string",
			path:      "/api/actions/{actionId}/decision",
			args:      map[string]any{"actionId": map[string]any{"nested": 1}},
			wantErr:   true,
			errSubstr: "actionId (not a string)",
		},
		{
			name:      "slice arg reported as not a string",
			path:      "/api/actions/{actionId}/decision",
			args:      map[string]any{"actionId": []string{"a", "b"}},
			wantErr:   true,
			errSubstr: "actionId (not a string)",
		},
		{
			name:      "explicit nil arg value reported as not a string",
			path:      "/api/actions/{actionId}/decision",
			args:      map[string]any{"actionId": nil},
			wantErr:   true,
			errSubstr: "actionId (not a string)",
		},
		{
			name:      "mixed missing and non-string collected in placeholder order",
			path:      "/{first}/{second}/{third}",
			args:      map[string]any{"second": 42},
			wantErr:   true,
			errSubstr: "missing path argument(s): first, second (not a string), third",
		},
		{
			name:     "empty string value is a valid segment",
			path:     "/api/{a}/end",
			args:     map[string]any{"a": ""},
			wantPath: "/api//end",
		},
		{
			name:     "extra unknown string args are ignored",
			path:     "/api/{a}",
			args:     map[string]any{"a": "x", "extra": "y"},
			wantPath: "/api/x",
		},
		{
			name:     "extra non-string arg unreferenced by path is ignored",
			path:     "/api/{a}",
			args:     map[string]any{"a": "x", "ignored": 42},
			wantPath: "/api/x",
		},
		{
			name:     "placeholder name with leading underscore is not matched stays literal",
			path:     "/api/{_name}",
			args:     map[string]any{"_name": "x"},
			wantPath: "/api/{_name}",
		},
		{
			name:     "placeholder name containing dash is not matched stays literal",
			path:     "/api/{action-id}",
			args:     map[string]any{"action-id": "x"},
			wantPath: "/api/{action-id}",
		},
		{
			name:     "placeholder name containing underscore is not matched stays literal",
			path:     "/api/{a_b}",
			args:     map[string]any{"a_b": "x"},
			wantPath: "/api/{a_b}",
		},
		{
			name:     "placeholder name with trailing digits is matched",
			path:     "/api/{node1}",
			args:     map[string]any{"node1": "a/b"},
			wantPath: "/api/a%2Fb",
		},
		{
			name:     "value of only unreserved bytes stays literal",
			path:     "/{a}",
			args:     map[string]any{"a": "Aa0-._~"},
			wantPath: "/Aa0-._~",
		},
		{
			name:     "multibyte value escapes each UTF-8 byte as uppercase hex",
			path:     "/{a}",
			args:     map[string]any{"a": "é"},
			wantPath: "/%C3%A9",
		},
		{
			name:     "control byte tab escaped as percent hex",
			path:     "/{a}",
			args:     map[string]any{"a": "a\tb"},
			wantPath: "/a%09b",
		},
		{
			name:     "slash space and colon all escaped in a single value",
			path:     "/{a}",
			args:     map[string]any{"a": "a b/c:d"},
			wantPath: "/a%20b%2Fc%3Ad",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotPath, err := SubstitutePathParameters(tc.path, tc.args)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("SubstitutePathParameters err = nil, want non-nil; path=%q args=%#v got=%q", tc.path, tc.args, gotPath)
				}
				if tc.errSubstr != "" && !strings.Contains(err.Error(), tc.errSubstr) {
					t.Fatalf("SubstitutePathParameters err = %q, want substring %q", err.Error(), tc.errSubstr)
				}
				if gotPath != "" {
					t.Fatalf("SubstitutePathParameters path = %q on error, want empty string", gotPath)
				}
				return
			}
			if err != nil {
				t.Fatalf("SubstitutePathParameters err = %v, want nil; path=%q args=%#v", err, tc.path, tc.args)
			}
			if gotPath != tc.wantPath {
				t.Fatalf("SubstitutePathParameters path = %q, want %q", gotPath, tc.wantPath)
			}
		})
	}
}

// TestSubstitutePathParametersErrorIsNonTyped confirms the error returned by
// SubstitutePathParameters is a plain fmt.Errorf-wrapped string (not a typed
// sentinel like CapabilityLookupError), pinning the error-shape contract that
// adapters rely on for path-argument validation.
func TestSubstitutePathParametersErrorIsNonTyped(t *testing.T) {
	_, err := SubstitutePathParameters("/{missing}", map[string]any{})
	if err == nil {
		t.Fatal("SubstitutePathParameters err = nil, want non-nil for missing argument")
	}
	if _, ok := err.(CapabilityLookupError); ok {
		t.Fatalf("SubstitutePathParameters must not return CapabilityLookupError, got %T: %v", err, err)
	}
	if !strings.HasPrefix(err.Error(), "missing path argument(s):") {
		t.Fatalf("SubstitutePathParameters err = %q, want 'missing path argument(s):' prefix", err.Error())
	}
}
