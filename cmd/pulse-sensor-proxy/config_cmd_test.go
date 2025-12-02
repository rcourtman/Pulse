package main

import (
	"reflect"
	"testing"
)

func TestExtractNodesFromYAML(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		// Map format with allowed_nodes key
		{
			name: "map format with allowed_nodes key",
			input: `allowed_nodes:
  - node1
  - node2
  - node3`,
			expected: []string{"node1", "node2", "node3"},
		},
		{
			name: "map format with other keys",
			input: `metrics_address: 127.0.0.1:9127
allowed_nodes:
  - host1
  - host2
ssh_timeout: 30s`,
			expected: []string{"host1", "host2"},
		},
		{
			name: "map format empty allowed_nodes",
			input: `allowed_nodes: []
other_key: value`,
			expected: nil,
		},

		// List format (bare YAML list)
		{
			name: "list format bare",
			input: `- node1
- node2
- node3`,
			expected: []string{"node1", "node2", "node3"},
		},
		{
			name:     "list format single item",
			input:    `- single-node`,
			expected: []string{"single-node"},
		},

		// Empty and edge cases
		{
			name:     "empty input",
			input:    ``,
			expected: nil,
		},
		{
			name:     "only whitespace",
			input:    `   `,
			expected: nil,
		},
		{
			name: "empty strings filtered",
			input: `allowed_nodes:
  - node1
  - ""
  - node2`,
			expected: []string{"node1", "node2"},
		},
		{
			name: "null value in list",
			input: `allowed_nodes:
  - node1
  - ~
  - node2`,
			expected: []string{"node1", "node2"},
		},
		{
			name:     "invalid YAML",
			input:    `{{{invalid`,
			expected: nil,
		},

		// Mixed content
		{
			name: "map format with comments",
			input: `# This is a comment
allowed_nodes:
  - node1
  # inline comment
  - node2`,
			expected: []string{"node1", "node2"},
		},
		{
			name:     "map format allowed_nodes not a list",
			input:    `allowed_nodes: not-a-list`,
			expected: nil,
		},

		// Different YAML structures
		{
			name: "nested structure ignored",
			input: `config:
  allowed_nodes:
    - nested-node`,
			expected: nil, // only top-level allowed_nodes is recognized
		},
		{
			name:     "just null",
			input:    `~`,
			expected: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := extractNodesFromYAML([]byte(tc.input))

			if !reflect.DeepEqual(result, tc.expected) {
				t.Errorf("extractNodesFromYAML() = %v, want %v", result, tc.expected)
			}
		})
	}
}

func TestExtractNodesFromYAML_WithNumericValues(t *testing.T) {
	// YAML can have non-string values in lists
	input := `allowed_nodes:
  - node1
  - 12345
  - true
  - node2`

	result := extractNodesFromYAML([]byte(input))

	// Only string values should be extracted
	expected := []string{"node1", "node2"}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("extractNodesFromYAML() = %v, want %v", result, expected)
	}
}

func TestExtractNodesFromYAML_ListWithMixedTypes(t *testing.T) {
	// Bare list with mixed types
	input := `- host1
- 42
- host2
- null
- host3`

	result := extractNodesFromYAML([]byte(input))

	// Only non-empty strings should be extracted
	expected := []string{"host1", "host2", "host3"}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("extractNodesFromYAML() = %v, want %v", result, expected)
	}
}
