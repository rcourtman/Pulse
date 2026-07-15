package qualification

import (
	"strings"
	"testing"
)

// TestCanonicalToolInputBranches exercises every branch of canonicalToolInput,
// focusing on the previously uncovered error and early-return paths: the empty
// input short circuit ("{}"), the raw invalid-JSON decode failure, the
// multiple-JSON-values rejection, and the trailing-data rejection. A canonical
// happy path is included to pin the success contract.
func TestCanonicalToolInputBranches(t *testing.T) {
	for _, tc := range []struct {
		name       string
		input      string
		wantOut    string
		wantErr    bool
		errSubstr  string
		notSubstrs []string
	}{
		{
			name:    "empty input returns empty object",
			input:   "",
			wantOut: "{}",
		},
		{
			name:    "whitespace only collapses to empty object",
			input:   "   \t\n  ",
			wantOut: "{}",
		},
		{
			name:       "valid single object canonicalizes key order and spacing",
			input:      `{ "b": 2, "a": 1 }`,
			wantOut:    `{"a":1,"b":2}`,
			notSubstrs: []string{"multiple JSON values", "trailing JSON data"},
		},
		{
			name:       "invalid json returns decoder error",
			input:      `{"description":"cut...`,
			wantErr:    true,
			notSubstrs: []string{"multiple JSON values", "trailing JSON data"},
		},
		{
			name:      "multiple json values are rejected",
			input:     `{}{}`,
			wantErr:   true,
			errSubstr: "multiple JSON values",
		},
		{
			name:      "trailing garbage after value is rejected",
			input:     `1 garbage`,
			wantErr:   true,
			errSubstr: "trailing JSON data",
		},
		{
			name:      "trailing garbage after object is rejected",
			input:     `{} x`,
			wantErr:   true,
			errSubstr: "trailing JSON data",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := canonicalToolInput(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("canonicalToolInput(%q) error = nil, want non-nil", tc.input)
				}
				if got != "" {
					t.Fatalf("canonicalToolInput(%q) output = %q, want empty string on error", tc.input, got)
				}
				if tc.errSubstr != "" && !strings.Contains(err.Error(), tc.errSubstr) {
					t.Fatalf("canonicalToolInput(%q) error = %q, want substring %q", tc.input, err.Error(), tc.errSubstr)
				}
				for _, bad := range tc.notSubstrs {
					if strings.Contains(err.Error(), bad) {
						t.Fatalf("canonicalToolInput(%q) error = %q, must not mention %q", tc.input, err.Error(), bad)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("canonicalToolInput(%q) error = %v, want nil", tc.input, err)
			}
			if got != tc.wantOut {
				t.Fatalf("canonicalToolInput(%q) = %q, want %q", tc.input, got, tc.wantOut)
			}
		})
	}
}
