package agentcapabilities

import (
	"strings"
	"testing"
)

// TestSplitTrailingProviderToolNamePrefixBranchCoverage exercises the branch
// edges of SplitTrailingProviderToolNamePrefix that the sibling test does not
// reach: the empty-content early return and the no-alphanumeric-tail early
// return, alongside the HasPrefix-hold and pass-through arms for completeness.
func TestSplitTrailingProviderToolNamePrefixBranchCoverage(t *testing.T) {
	catalog := NewProviderToolNameCatalog([]string{
		"pulse_query",
		"pulse_read",
		"patrol_report_finding",
	})

	tests := []struct {
		name        string
		input       string
		wantVisible string
		wantHeld    string
	}{
		{
			name:        "empty content returns empty visible and held",
			input:       "",
			wantVisible: "",
			wantHeld:    "",
		},
		{
			name:        "trailing non-alnum char leaves nothing to hold",
			input:       "done.",
			wantVisible: "done.",
			wantHeld:    "",
		},
		{
			name:        "trailing whitespace leaves nothing to hold",
			input:       "done.\n",
			wantVisible: "done.\n",
			wantHeld:    "",
		},
		{
			name:        "fully non-alphanumeric content is returned whole",
			input:       "!!! ???",
			wantVisible: "!!! ???",
			wantHeld:    "",
		},
		{
			name:        "partial catalogued name at tail is held",
			input:       "Calling pulse_re",
			wantVisible: "Calling ",
			wantHeld:    "pulse_re",
		},
		{
			name:        "complete catalogued name at tail is held",
			input:       "Calling patrol_report_finding",
			wantVisible: "Calling ",
			wantHeld:    "patrol_report_finding",
		},
		{
			name:        "trailing alnum token that is no catalogued prefix passes through",
			input:       "done! now",
			wantVisible: "done! now",
			wantHeld:    "",
		},
		{
			name:        "whole content is an unknown alnum token and passes through",
			input:       "completely_unknown",
			wantVisible: "completely_unknown",
			wantHeld:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			visible, held := SplitTrailingProviderToolNamePrefix(tt.input, catalog)
			if visible != tt.wantVisible || held != tt.wantHeld {
				t.Fatalf("SplitTrailingProviderToolNamePrefix(%q) = (%q, %q), want (%q, %q)",
					tt.input, visible, held, tt.wantVisible, tt.wantHeld)
			}
		})
	}
}

// TestProviderJSONToolCallLeakIndexBranchCoverage drives the private JSON leak
// detector directly so every guard is reached independently of the public
// aggregator: empty content, regex no-match, a structurally-matching JSON tool
// call whose name is absent from the catalog, and a catalogued-name leak.
func TestProviderJSONToolCallLeakIndexBranchCoverage(t *testing.T) {
	catalog := NewProviderToolNameCatalog([]string{"pulse_query", "pulse_read"})

	tests := []struct {
		name      string
		input     string
		wantIndex int
	}{
		{
			name:      "empty content short-circuits to -1",
			input:     "",
			wantIndex: -1,
		},
		{
			name:      "plain prose produces no regex match",
			input:     "hello world, nothing structural here",
			wantIndex: -1,
		},
		{
			name:      "json without a name field produces no regex match",
			input:     `{"foo": "bar", "count": 3}`,
			wantIndex: -1,
		},
		{
			name:      "json tool-call shape with uncatalogued name is ignored",
			input:     `{"name": "frobnicate", "parameters": {}}`,
			wantIndex: -1,
		},
		{
			name:      "multiple json tool-call shapes all uncatalogued are ignored",
			input:     "{\"name\": \"alpha\"}\n{\"name\": \"beta\"}",
			wantIndex: -1,
		},
		{
			name:      "catalogued json tool call at content start reports offset zero",
			input:     `{"name":"pulse_query","parameters":{}}`,
			wantIndex: 0,
		},
		{
			name:      "catalogued json tool call after prose reports the newline offset",
			input:     "Prose.\n{\"name\":\"pulse_read\"}",
			wantIndex: strings.Index("Prose.\n{\"name\":\"pulse_read\"}", "\n"),
		},
		{
			name:      "catalogued json tool call inside a fenced block reports the fence-newline offset",
			input:     "Here is the call:\n```json\n{\"name\":\"pulse_query\"}",
			wantIndex: strings.Index("Here is the call:\n```json\n{\"name\":\"pulse_query\"}", "\n"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := providerJSONToolCallLeakIndex(tt.input, catalog); got != tt.wantIndex {
				t.Fatalf("providerJSONToolCallLeakIndex(%q) = %d, want %d", tt.input, got, tt.wantIndex)
			}
		})
	}
}

// TestProviderPlainFunctionToolCallLeakIndexBranchCoverage drives the private
// plain-function leak detector directly: empty content, regex no-match, a
// function-shaped call whose name is absent from the catalog, and catalogued
// leaks at both the content start and after a leading non-alnum separator.
func TestProviderPlainFunctionToolCallLeakIndexBranchCoverage(t *testing.T) {
	catalog := NewProviderToolNameCatalog([]string{"pulse_query", "pulse_read"})

	tests := []struct {
		name      string
		input     string
		wantIndex int
	}{
		{
			name:      "empty content short-circuits to -1",
			input:     "",
			wantIndex: -1,
		},
		{
			name:      "prose without a parenthesised call produces no regex match",
			input:     "the pulse_query tool is useful, no parens here",
			wantIndex: -1,
		},
		{
			name:      "function-shaped call with uncatalogued name is ignored",
			input:     `helper(target_host="current_resource")`,
			wantIndex: -1,
		},
		{
			name:      "multiple uncatalogued function calls are ignored",
			input:     "alpha(1)\nbeta(2)",
			wantIndex: -1,
		},
		{
			name:      "catalogued function call at content start reports name offset zero",
			input:     `pulse_query(action="get")`,
			wantIndex: 0,
		},
		{
			name:      "catalogued function call after a space reports the name offset",
			input:     "Run pulse_read(target=\"x\")",
			wantIndex: strings.Index("Run pulse_read(target=\"x\")", "pulse_read"),
		},
		{
			name:      "catalogued function call after a leading dot reports the name offset",
			input:     "step.pulse_query()",
			wantIndex: strings.Index("step.pulse_query()", "pulse_query"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := providerPlainFunctionToolCallLeakIndex(tt.input, catalog); got != tt.wantIndex {
				t.Fatalf("providerPlainFunctionToolCallLeakIndex(%q) = %d, want %d", tt.input, got, tt.wantIndex)
			}
		})
	}
}
