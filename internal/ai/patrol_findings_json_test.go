package ai

import "testing"

// TestIsValidJSON exercises every branch of isValidJSON:
//   - the empty-after-trim arm (returns false),
//   - the fast-reject arm where the first rune is neither '{' nor '[' (returns false),
//   - the json.Unmarshal success arm (returns true),
//   - the json.Unmarshal failure arm for inputs that pass the prefix check but
//     are not well-formed JSON (returns false).
//
// It also pins the deliberate (but perhaps surprising) scalar-rejection
// behavior: top-level JSON scalars such as "123", "true", "null" and "\"s\""
// are syntactically valid JSON, yet isValidJSON returns false for them because
// the fast-reject only admits object/array roots. See GLM_REPORT.md.
func TestIsValidJSON(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		// --- empty-after-trim branch (trimmed == "") ---
		{
			name: "empty string yields false",
			in:   "",
			want: false,
		},
		{
			name: "whitespace only yields false",
			in:   "   ",
			want: false,
		},
		{
			name: "tab only yields false",
			in:   "\t",
			want: false,
		},
		{
			name: "newline only yields false",
			in:   "\n",
			want: false,
		},
		{
			name: "mixed whitespace only yields false",
			in:   " \t\n\r ",
			want: false,
		},

		// --- fast-reject branch: first rune is neither '{' nor '[' ---
		{
			name: "plain text rejected by prefix",
			in:   "hello world",
			want: false,
		},
		{
			name: "json scalar number rejected by prefix even though valid JSON",
			in:   "123",
			want: false,
		},
		{
			name: "json scalar true rejected by prefix even though valid JSON",
			in:   "true",
			want: false,
		},
		{
			name: "json scalar false rejected by prefix even though valid JSON",
			in:   "false",
			want: false,
		},
		{
			name: "json scalar null rejected by prefix even though valid JSON",
			in:   "null",
			want: false,
		},
		{
			name: "json scalar string rejected by prefix even though valid JSON",
			in:   `"a string"`,
			want: false,
		},
		{
			name: "xml-looking content rejected by prefix",
			in:   "<root>not json</root>",
			want: false,
		},
		{
			name: "leading paren rejected by prefix",
			in:   "(not json)",
			want: false,
		},
		{
			name: "whitespace then non-json prefix still rejected",
			in:   "   not-json-at-all",
			want: false,
		},

		// --- json.Unmarshal success branch (valid object/array roots) ---
		{
			name: "empty object is valid",
			in:   "{}",
			want: true,
		},
		{
			name: "empty array is valid",
			in:   "[]",
			want: true,
		},
		{
			name: "simple object is valid",
			in:   `{"key": "value"}`,
			want: true,
		},
		{
			name: "simple array is valid",
			in:   "[1, 2, 3]",
			want: true,
		},
		{
			name: "nested object is valid",
			in:   `{"a": {"b": {"c": [1, 2, {"d": true}]}}}`,
			want: true,
		},
		{
			name: "object with assorted scalar types is valid",
			in:   `{"n": 42, "s": "hi", "b": true, "z": null, "arr": [1, 2]}`,
			want: true,
		},
		{
			name: "unicode in object values is valid",
			in:   `{"msg": "héllo wörld 日本語"}`,
			want: true,
		},
		{
			name: "leading whitespace before object is trimmed and valid",
			in:   "   {\"k\": 1}",
			want: true,
		},
		{
			name: "trailing whitespace after object is trimmed and valid",
			in:   "{\"k\": 1}\n\n",
			want: true,
		},
		{
			name: "leading and trailing whitespace with newlines and tabs trimmed and valid",
			in:   "\n\t {\"k\": 1}\t\n",
			want: true,
		},
		{
			name: "leading whitespace before array is trimmed and valid",
			in:   "\t[1, 2]",
			want: true,
		},
		{
			name: "array of objects is valid",
			in:   `[{"id": 1}, {"id": 2}]`,
			want: true,
		},

		// --- json.Unmarshal failure branch: prefix ok but body malformed ---
		{
			name: "lone open brace fails unmarshal",
			in:   "{",
			want: false,
		},
		{
			name: "lone open bracket fails unmarshal",
			in:   "[",
			want: false,
		},
		{
			name: "object missing closing brace fails unmarshal",
			in:   `{"k": "v"`,
			want: false,
		},
		{
			name: "array missing closing bracket fails unmarshal",
			in:   `[1, 2, 3`,
			want: false,
		},
		{
			name: "object with trailing comma fails unmarshal",
			in:   `{"a": 1,}`,
			want: false,
		},
		{
			name: "unquoted key fails unmarshal",
			in:   `{key: "value"}`,
			want: false,
		},
		{
			name: "single quotes fail unmarshal",
			in:   `{'key': 'value'}`,
			want: false,
		},
		{
			name: "trailing garbage after valid object fails unmarshal",
			in:   `{"a": 1} garbage`,
			want: false,
		},
		{
			name: "leading whitespace then malformed object fails unmarshal",
			in:   "   {bad}",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidJSON(tt.in)
			if got != tt.want {
				t.Errorf("isValidJSON(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
