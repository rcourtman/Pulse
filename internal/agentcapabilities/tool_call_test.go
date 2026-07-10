package agentcapabilities

import "testing"

func TestToolCallKindString(t *testing.T) {
	tests := []struct {
		kind ToolCallKind
		want string
	}{
		{ToolCallKindResolve, "resolve"},
		{ToolCallKindRead, "read"},
		{ToolCallKindWrite, "write"},
		{ToolCallKindUserInput, "user_input"},
		{ToolCallKind(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.kind.String(); got != tt.want {
			t.Fatalf("ToolCallKind(%d).String() = %q, want %q", tt.kind, got, tt.want)
		}
	}
}

func TestPulseIntelligenceToolNameConstants(t *testing.T) {
	tests := map[string]string{
		"PulseQueryToolName":                 PulseQueryToolName,
		"PulseDiscoveryToolName":             PulseDiscoveryToolName,
		"PulseMetricsToolName":               PulseMetricsToolName,
		"PulseStorageToolName":               PulseStorageToolName,
		"PulseDockerToolName":                PulseDockerToolName,
		"PulseKubernetesToolName":            PulseKubernetesToolName,
		"PulseAlertsToolName":                PulseAlertsToolName,
		"PulseReadToolName":                  PulseReadToolName,
		"PulseControlToolName":               PulseControlToolName,
		"PulseFileEditToolName":              PulseFileEditToolName,
		"PulseKnowledgeToolName":             PulseKnowledgeToolName,
		"PulsePMGToolName":                   PulsePMGToolName,
		"PulseSummarizeToolName":             PulseSummarizeToolName,
		"PulseRunCommandToolName":            PulseRunCommandToolName,
		"PulseControlGuestToolName":          PulseControlGuestToolName,
		"PulseControlDockerToolName":         PulseControlDockerToolName,
		"PulseSearchResourcesToolName":       PulseSearchResourcesToolName,
		"PulseGetResourceToolName":           PulseGetResourceToolName,
		"PulseGetTopologyToolName":           PulseGetTopologyToolName,
		"PulseListInfrastructureToolName":    PulseListInfrastructureToolName,
		"PulseGetConnectionHealthToolName":   PulseGetConnectionHealthToolName,
		"PulseGetDockerLogsToolName":         PulseGetDockerLogsToolName,
		"PulseGetPerformanceMetricsToolName": PulseGetPerformanceMetricsToolName,
		"PulseGetTemperaturesToolName":       PulseGetTemperaturesToolName,
		"PulseGetBaselinesToolName":          PulseGetBaselinesToolName,
		"PulseGetPatternsToolName":           PulseGetPatternsToolName,
		"PatrolGetFindingsToolName":          PatrolGetFindingsToolName,
		"PatrolReportFindingToolName":        PatrolReportFindingToolName,
		"PatrolResolveFindingToolName":       PatrolResolveFindingToolName,
	}
	want := map[string]string{
		"PulseQueryToolName":                 "pulse_query",
		"PulseDiscoveryToolName":             "pulse_discovery",
		"PulseMetricsToolName":               "pulse_metrics",
		"PulseStorageToolName":               "pulse_storage",
		"PulseDockerToolName":                "pulse_docker",
		"PulseKubernetesToolName":            "pulse_kubernetes",
		"PulseAlertsToolName":                "pulse_alerts",
		"PulseReadToolName":                  "pulse_read",
		"PulseControlToolName":               "pulse_control",
		"PulseFileEditToolName":              "pulse_file_edit",
		"PulseKnowledgeToolName":             "pulse_knowledge",
		"PulsePMGToolName":                   "pulse_pmg",
		"PulseSummarizeToolName":             "pulse_summarize",
		"PulseRunCommandToolName":            "pulse_run_command",
		"PulseControlGuestToolName":          "pulse_control_guest",
		"PulseControlDockerToolName":         "pulse_control_docker",
		"PulseSearchResourcesToolName":       "pulse_search_resources",
		"PulseGetResourceToolName":           "pulse_get_resource",
		"PulseGetTopologyToolName":           "pulse_get_topology",
		"PulseListInfrastructureToolName":    "pulse_list_infrastructure",
		"PulseGetConnectionHealthToolName":   "pulse_get_connection_health",
		"PulseGetDockerLogsToolName":         "pulse_get_docker_logs",
		"PulseGetPerformanceMetricsToolName": "pulse_get_performance_metrics",
		"PulseGetTemperaturesToolName":       "pulse_get_temperatures",
		"PulseGetBaselinesToolName":          "pulse_get_baselines",
		"PulseGetPatternsToolName":           "pulse_get_patterns",
		"PatrolGetFindingsToolName":          "patrol_get_findings",
		"PatrolReportFindingToolName":        "patrol_report_finding",
		"PatrolResolveFindingToolName":       "patrol_resolve_finding",
	}

	for name, got := range tests {
		if got != want[name] {
			t.Fatalf("%s = %q, want %q", name, got, want[name])
		}
	}
}

func TestClassifyToolCallUsesSharedSafetyClassification(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		args     map[string]interface{}
		want     ToolCallKind
	}{
		{name: "native question", toolName: PulseQuestionToolName, want: ToolCallKindUserInput},
		{name: "query resolves", toolName: "pulse_query", want: ToolCallKindResolve},
		{name: "discovery get resolves", toolName: "pulse_discovery", args: map[string]interface{}{"action": "get"}, want: ToolCallKindResolve},
		{name: "discovery run resolves", toolName: "pulse_discovery", args: map[string]interface{}{"action": "run"}, want: ToolCallKindResolve},
		{name: "discovery missing action fails closed", toolName: "pulse_discovery", want: ToolCallKindWrite},
		{name: "metrics reads", toolName: "pulse_metrics", want: ToolCallKindRead},
		{name: "summarize reads", toolName: PulseSummarizeToolName, want: ToolCallKindRead},
		{name: "alert list reads", toolName: "pulse_alerts", args: map[string]interface{}{"action": "list"}, want: ToolCallKindRead},
		{name: "alert resolve writes", toolName: "pulse_alerts", args: map[string]interface{}{"action": "resolve"}, want: ToolCallKindWrite},
		{name: "read exec remains read", toolName: "pulse_read", args: map[string]interface{}{"action": "exec"}, want: ToolCallKindRead},
		{name: "control writes", toolName: "pulse_control", args: map[string]interface{}{"type": "command"}, want: ToolCallKindWrite},
		{name: "docker services reads", toolName: "pulse_docker", args: map[string]interface{}{"action": "services"}, want: ToolCallKindRead},
		{name: "docker update writes", toolName: "pulse_docker", args: map[string]interface{}{"action": "update"}, want: ToolCallKindWrite},
		// Kubernetes's real discriminator is `type`; the retired
		// hard-coded classifier read `action` and therefore classified
		// scale/restart/delete_pod/exec as read.
		{name: "kubernetes pods reads", toolName: "pulse_kubernetes", args: map[string]interface{}{"type": "pods"}, want: ToolCallKindRead},
		{name: "kubernetes scale writes", toolName: "pulse_kubernetes", args: map[string]interface{}{"type": "scale"}, want: ToolCallKindWrite},
		{name: "kubernetes exec writes", toolName: "pulse_kubernetes", args: map[string]interface{}{"type": "exec"}, want: ToolCallKindWrite},
		{name: "kubernetes wrong discriminator fails closed", toolName: "pulse_kubernetes", args: map[string]interface{}{"action": "pods"}, want: ToolCallKindWrite},
		// pulse_file_edit is write-only; file reads route via pulse_read.
		{name: "file read fails closed", toolName: "pulse_file_edit", args: map[string]interface{}{"action": "read"}, want: ToolCallKindWrite},
		{name: "file append writes", toolName: "pulse_file_edit", args: map[string]interface{}{"action": "append"}, want: ToolCallKindWrite},
		{name: "knowledge recall reads", toolName: "pulse_knowledge", args: map[string]interface{}{"action": "recall"}, want: ToolCallKindRead},
		{name: "knowledge remember writes", toolName: "pulse_knowledge", args: map[string]interface{}{"action": "remember"}, want: ToolCallKindWrite},
		{name: "legacy command writes", toolName: LegacyAssistantRunCommandToolName, want: ToolCallKindWrite},
		{name: "legacy fetch url reads", toolName: LegacyAssistantFetchURLToolName, want: ToolCallKindRead},
		{name: "legacy set url writes", toolName: LegacyAssistantSetResourceURLToolName, want: ToolCallKindWrite},
		{name: "patrol findings read", toolName: "patrol_get_findings", want: ToolCallKindRead},
		{name: "patrol report writes", toolName: "patrol_report_finding", want: ToolCallKindWrite},
		{name: "unknown defaults write", toolName: "future_tool", want: ToolCallKindWrite},
		{name: "generic action read", toolName: "future_tool", args: map[string]interface{}{"action": "inspect"}, want: ToolCallKindRead},
		{name: "generic operation write", toolName: "future_tool", args: map[string]interface{}{"operation": "delete"}, want: ToolCallKindWrite},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClassifyToolCall(tt.toolName, tt.args); got != tt.want {
				t.Fatalf("ClassifyToolCall(%q, %#v) = %s, want %s", tt.toolName, tt.args, got, tt.want)
			}
		})
	}
}
