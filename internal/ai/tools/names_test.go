package tools

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
)

func TestKnownToolNamesIncludesRegisteredTools(t *testing.T) {
	// Spot-check a representative subset of canonical names that should
	// always be registered. Drift here means registerTools() lost a tool
	// or IsKnownToolName() stopped reading from the canonical registry-backed
	// shared provider-tool name catalog.
	expected := []string{
		agentcapabilities.PulseQueryToolName,
		agentcapabilities.PulseDiscoveryToolName,
		agentcapabilities.PulseReadToolName,
		agentcapabilities.PulseSummarizeToolName,
		agentcapabilities.PulseControlToolName,
		agentcapabilities.PulseMetricsToolName,
		agentcapabilities.PulseStorageToolName,
		agentcapabilities.PulseDockerToolName,
		agentcapabilities.PulseKubernetesToolName,
		agentcapabilities.PulseAlertsToolName,
		agentcapabilities.PulseKnowledgeToolName,
		agentcapabilities.PulsePMGToolName,
		agentcapabilities.PulseQuestionToolName,
		agentcapabilities.PatrolReportFindingToolName,
		agentcapabilities.PatrolResolveFindingToolName,
		agentcapabilities.PatrolGetFindingsToolName,
		agentcapabilities.PatrolAssessFindingToolName,
	}
	for _, name := range expected {
		if !IsKnownToolName(name) {
			t.Errorf("IsKnownToolName(%q) = false, want true", name)
		}
	}
}

func TestRetiredFileMutationToolIsNotKnown(t *testing.T) {
	if IsKnownToolName(agentcapabilities.PulseFileEditToolName) {
		t.Fatal("retired file mutation tool must not remain in the provider catalog")
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

func TestIsKnownToolNamePrefix(t *testing.T) {
	cases := []string{
		"p",
		"pulse_",
		"pulse_q",
		"pulse_re",
		"pulse_read",
		"patrol_",
		"patrol_report",
	}
	for _, prefix := range cases {
		if !IsKnownToolNamePrefix(prefix) {
			t.Errorf("IsKnownToolNamePrefix(%q) = false, want true", prefix)
		}
	}
}

func TestIsKnownToolNamePrefixRejectsUnknown(t *testing.T) {
	cases := []string{
		"",
		"x",
		"Pulse_",
		"helper",
		"pulse_unknown_tool",
	}
	for _, prefix := range cases {
		if IsKnownToolNamePrefix(prefix) {
			t.Errorf("IsKnownToolNamePrefix(%q) = true, want false", prefix)
		}
	}
}
