package tools

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNodeMatchesTargetID exercises nodeMatchesTargetID(nodeName, targetID string) bool
// across every branch: case-insensitive equality, composite instance-node suffix
// match, and the default fallthrough to false. Edge cases (empty inputs, case
// folding, dash-boundary semantics) are asserted against the actual current
// behavior of the function in tools_discovery.go.
func TestNodeMatchesTargetID(t *testing.T) {
	tests := []struct {
		name     string
		nodeName string
		targetID string
		want     bool
	}{
		// --- Branch 1: case-insensitive equality (strings.EqualFold) -> true ---
		{
			name:     "exact match lowercase",
			nodeName: "pve-node",
			targetID: "pve-node",
			want:     true,
		},
		{
			name:     "case-insensitive equality node upper target lower",
			nodeName: "PVE-NODE",
			targetID: "pve-node",
			want:     true,
		},
		{
			name:     "case-insensitive equality node lower target upper",
			nodeName: "pve-node",
			targetID: "PVE-NODE",
			want:     true,
		},
		{
			name:     "case-insensitive equality mixed case both sides",
			nodeName: "HoMeLaB-pVe-NoDe",
			targetID: "homelab-pve-node",
			want:     true,
		},
		{
			name:     "equality single character nodes",
			nodeName: "a",
			targetID: "A",
			want:     true,
		},
		// Both empty strings are EqualFold-equal, so this short-circuits to true.
		{
			name:     "both empty equality true",
			nodeName: "",
			targetID: "",
			want:     true,
		},

		// --- Branch 2: composite instance-node suffix match -> true ---
		// targetID ends with "-"+nodeName (case-insensitive).
		{
			name:     "composite instance node suffix match",
			nodeName: "pve-node",
			targetID: "homelab-pve-node",
			want:     true,
		},
		{
			name:     "composite suffix case-insensitive node upper",
			nodeName: "PVE-NODE",
			targetID: "homelab-pve-node",
			want:     true,
		},
		{
			name:     "composite suffix case-insensitive target upper",
			nodeName: "pve-node",
			targetID: "HOMELAB-PVE-NODE",
			want:     true,
		},
		{
			name:     "composite suffix with multi-segment instance prefix",
			nodeName: "node1",
			targetID: "cluster-a-instance-node1",
			want:     true,
		},
		// Note on actual behavior: suffix match requires only that targetID ends
		// with "-"+nodeName, so any nodeName that is itself the trailing dash
		// segment of targetID matches, even when nodeName is a generic word.
		{
			name:     "generic suffix segment matches",
			nodeName: "node",
			targetID: "homelab-pve-node",
			want:     true,
		},
		// Documented quirk: with nodeName empty, suffix becomes just "-", so any
		// targetID ending in "-" matches. See GLM_REPORT.md.
		{
			name:     "empty nodeName matches targetID ending in dash",
			nodeName: "",
			targetID: "abc-",
			want:     true,
		},

		// --- Branch 3: default fallthrough -> false ---
		{
			name:     "completely unrelated values",
			nodeName: "pve-node",
			targetID: "agent-1",
			want:     false,
		},
		{
			name:     "substring without dash boundary does not match",
			nodeName: "pve-node",
			targetID: "mypve-node",
			want:     false,
		},
		{
			name:     "nodeName appears earlier but not as dash suffix",
			nodeName: "pve-node",
			targetID: "pve-node-2",
			want:     false,
		},
		{
			name:     "nodeName is trailing but no dash prefix",
			nodeName: "node",
			targetID: "homelab-node-x",
			want:     false,
		},
		{
			name:     "empty nodeName non-dash targetID no match",
			nodeName: "",
			targetID: "pve-node",
			want:     false,
		},
		{
			name:     "empty targetID non-empty node no match",
			nodeName: "pve-node",
			targetID: "",
			want:     false,
		},
		{
			name:     "whitespace not trimmed equality miss",
			nodeName: "pve-node",
			targetID: "pve-node ",
			want:     false,
		},
		{
			name:     "whitespace not trimmed suffix miss",
			nodeName: "pve-node",
			targetID: "homelab-pve-node ",
			want:     false,
		},
		{
			name:     "single char nodeName not dash suffix of target",
			nodeName: "x",
			targetID: "x-foo",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nodeMatchesTargetID(tt.nodeName, tt.targetID)
			assert.Equal(t, tt.want, got,
				"nodeMatchesTargetID(%q, %q)", tt.nodeName, tt.targetID)
		})
	}
}
