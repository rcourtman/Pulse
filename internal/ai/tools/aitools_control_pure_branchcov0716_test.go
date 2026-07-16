package tools

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
)

// This file raises branch coverage for two pure helpers in tools_control.go:
//   - resolvedResourceControlIdentity (nil resource guard, provider-uid path,
//     alias-scan loop with whitespace skipping, fallback trimming)
//   - formatAvailableAgentHosts (no-agents path, hostname/agentID fallback in
//     collectAgentHostnames, maxItems truncation and "(+N more)" suffix)
//
// Both helpers are pure (no I/O, no executor state) and return exact strings,
// so every case asserts the concrete return value rather than "no panic".
//
// The in-package test mock `mockResource` (defined in strict_resolution_test.go)
// implements ResolvedResourceInfo and is reused here to drive the non-nil arm.

// TestBranchCovResolvedResourceControlIdentity covers every return path of
// resolvedResourceControlIdentity: the nil-resource guard, the providerUID win
// (including whitespace trimming), the alias scan that skips whitespace-only
// aliases and returns the first non-empty one, and the final fallback path
// (no providerUID and no usable alias) including fallback trimming.
func TestBranchCovResolvedResourceControlIdentity(t *testing.T) {
	tests := []struct {
		name     string
		resource ResolvedResourceInfo
		fallback string
		want     string
	}{
		// --- nil resource: returns trimmed fallback. ---
		{name: "nil_resource_trims_fallback", resource: nil, fallback: "  nginx-01  ", want: "nginx-01"},
		{name: "nil_resource_empty_fallback", resource: nil, fallback: "", want: ""},
		{name: "nil_resource_whitespace_only_fallback", resource: nil, fallback: "   ", want: ""},

		// --- providerUID path: non-empty (after trim) wins over aliases/fallback. ---
		{
			name:     "providerUID_wins_over_aliases",
			resource: &mockResource{providerUID: "ctr-abc", aliases: []string{"nginx", "zzz"}},
			fallback: "fallback",
			want:     "ctr-abc",
		},
		{name: "providerUID_is_trimmed", resource: &mockResource{providerUID: "  ctr-abc  "}, fallback: "fallback", want: "ctr-abc"},
		// Whitespace-only providerUID is treated as empty -> falls through to aliases.
		{
			name:     "whitespace_only_providerUID_falls_through",
			resource: &mockResource{providerUID: "   ", aliases: []string{"nginx"}},
			fallback: "fallback",
			want:     "nginx",
		},

		// --- alias-scan path: returns the first non-empty (after trim) alias. ---
		{name: "first_alias_returned", resource: &mockResource{aliases: []string{"nginx", "abc"}}, fallback: "fallback", want: "nginx"},
		{name: "alias_is_trimmed", resource: &mockResource{aliases: []string{"  nginx-02  "}}, fallback: "fallback", want: "nginx-02"},
		// Leading whitespace-only alias is skipped, next usable alias is returned.
		{name: "skip_whitespace_only_alias", resource: &mockResource{aliases: []string{"   ", "nginx"}}, fallback: "fallback", want: "nginx"},
		{name: "skip_multiple_whitespace_aliases", resource: &mockResource{aliases: []string{"", "  ", "\t", "real-alias"}}, fallback: "fallback", want: "real-alias"},

		// --- fallback path: no providerUID and no usable alias. ---
		{name: "empty_aliases_falls_back", resource: &mockResource{aliases: []string{}}, fallback: "host-01", want: "host-01"},
		{name: "nil_aliases_falls_back", resource: &mockResource{aliases: nil}, fallback: "host-01", want: "host-01"},
		{name: "all_whitespace_aliases_falls_back", resource: &mockResource{aliases: []string{"  ", "\t"}}, fallback: "host-01", want: "host-01"},
		{name: "fallback_is_trimmed_when_used", resource: &mockResource{}, fallback: "  host-01  ", want: "host-01"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := resolvedResourceControlIdentity(tc.resource, tc.fallback)
			if got != tc.want {
				t.Fatalf("resolvedResourceControlIdentity(%v, %q) = %q, want %q",
					tc.resource, tc.fallback, got, tc.want)
			}
		})
	}
}

// TestBranchCovFormatAvailableAgentHosts covers every return path of
// formatAvailableAgentHosts (and its helper collectAgentHostnames): the
// no-agents path (nil, empty, and agents that all collapse to empty names),
// the hostname-used path, the AgentID fallback when Hostname is blank, the
// dropping of agents whose Hostname and AgentID are both blank, name trimming,
// and the truncation/"(+N more)" suffix at the maxItems=6 boundary.
func TestBranchCovFormatAvailableAgentHosts(t *testing.T) {
	// hostnames builds a slice of n agents with distinct, non-empty Hostnames.
	hostnames := func(n int) []agentexec.ConnectedAgent {
		out := make([]agentexec.ConnectedAgent, n)
		for i := 0; i < n; i++ {
			out[i] = agentexec.ConnectedAgent{Hostname: string(rune('a' + i))}
		}
		return out
	}

	tests := []struct {
		name   string
		agents []agentexec.ConnectedAgent
		want   string
	}{
		// --- no-agents path. ---
		{name: "nil_agents", agents: nil, want: "No agents are currently connected."},
		{name: "empty_agents", agents: []agentexec.ConnectedAgent{}, want: "No agents are currently connected."},
		// Agents exist but every one collapses to an empty name -> still "No agents".
		{
			name:   "all_agents_blank",
			agents: []agentexec.ConnectedAgent{{Hostname: "  ", AgentID: "  "}, {}},
			want:   "No agents are currently connected.",
		},

		// --- hostname path (single agent). ---
		{name: "single_hostname", agents: []agentexec.ConnectedAgent{{Hostname: "host-1"}}, want: "Available targets: host-1"},

		// --- AgentID fallback when Hostname is blank/whitespace. ---
		{
			name:   "agentID_used_when_hostname_blank",
			agents: []agentexec.ConnectedAgent{{AgentID: "agent-9"}},
			want:   "Available targets: agent-9",
		},
		{
			name:   "agentID_used_when_hostname_whitespace",
			agents: []agentexec.ConnectedAgent{{Hostname: "   ", AgentID: "agent-9"}},
			want:   "Available targets: agent-9",
		},
		// agentID is also trimmed.
		{
			name:   "agentID_is_trimmed",
			agents: []agentexec.ConnectedAgent{{AgentID: "  agent-9  "}},
			want:   "Available targets: agent-9",
		},

		// --- Hostname is preferred over AgentID. ---
		{
			name:   "hostname_preferred_over_agentID",
			agents: []agentexec.ConnectedAgent{{Hostname: "host-1", AgentID: "agent-1"}},
			want:   "Available targets: host-1",
		},

		// --- blank agent is dropped; remaining agents are listed in order. ---
		{
			name: "blank_agent_dropped_others_kept",
			agents: []agentexec.ConnectedAgent{
				{Hostname: "host-1"},
				{Hostname: "  ", AgentID: "  "}, // dropped
				{AgentID: "agent-3"},
			},
			want: "Available targets: host-1, agent-3",
		},

		// --- maxItems boundary (maxItems == 6 inside the function). ---
		// Exactly 6 hostnames: full list, no "(+N more)" suffix.
		{
			name:   "exactly_max_no_truncation",
			agents: hostnames(6),
			want:   "Available targets: a, b, c, d, e, f",
		},
		// 7 hostnames: list truncated to first 6, suffix "(+1 more)".
		{
			name:   "one_over_max_truncated",
			agents: hostnames(7),
			want:   "Available targets: a, b, c, d, e, f (+1 more)",
		},
		// 9 hostnames: suffix counts the overflow beyond maxItems.
		{
			name:   "several_over_max_truncated",
			agents: hostnames(9),
			want:   "Available targets: a, b, c, d, e, f (+3 more)",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := formatAvailableAgentHosts(tc.agents)
			if got != tc.want {
				t.Fatalf("formatAvailableAgentHosts(%v) = %q, want %q", tc.agents, got, tc.want)
			}
		})
	}
}
