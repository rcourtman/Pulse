package tools

import "testing"

func TestKnownToolNamesIncludesRegisteredTools(t *testing.T) {
	// Spot-check a representative subset of canonical names that should
	// always be registered. Drift here means registerTools() lost a tool
	// or KnownToolNames() stopped reading from the canonical registry.
	expected := []string{
		"pulse_query",
		"pulse_discovery",
		"pulse_read",
		"pulse_summarize",
		"pulse_control",
		"pulse_metrics",
		"pulse_storage",
		"pulse_docker",
		"pulse_kubernetes",
		"pulse_alerts",
		"pulse_file_edit",
		"pulse_knowledge",
		"pulse_pmg",
		"patrol_report_finding",
		"patrol_resolve_finding",
		"patrol_get_findings",
	}
	for _, name := range expected {
		if !IsKnownToolName(name) {
			t.Errorf("IsKnownToolName(%q) = false, want true", name)
		}
	}
}

func TestIsKnownToolNameRejectsUnknown(t *testing.T) {
	cases := []string{
		"",
		"frobnicate",
		"my-vm",
		"PULSE_QUERY", // case-sensitive
		"pulse_unknown_tool",
	}
	for _, name := range cases {
		if IsKnownToolName(name) {
			t.Errorf("IsKnownToolName(%q) = true, want false", name)
		}
	}
}
