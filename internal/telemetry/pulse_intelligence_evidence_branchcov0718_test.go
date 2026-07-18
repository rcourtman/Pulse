package telemetry

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// fixedCutoff is the `since` boundary used across these tests; events strictly
// before it must be filtered out, events at-or-after it must be projected.
var fixedCutoff = time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

func TestPulseIntelligenceAIUsageEvidenceFromHistory(t *testing.T) {
	// Anchor timestamps relative to the cutoff.
	before := fixedCutoff.Add(-24 * time.Hour) // excluded by cutoff
	atCutoff := fixedCutoff                    // included (not Before(since) and not zero)
	after := fixedCutoff.Add(24 * time.Hour)   // included

	tests := []struct {
		name    string
		history *config.AIUsageHistoryData
		want    PulseIntelligenceAIUsageEvidence
	}{
		{
			name:    "nil history returns zero evidence",
			history: nil,
			want:    PulseIntelligenceAIUsageEvidence{},
		},
		{
			name:    "empty events returns zero evidence",
			history: &config.AIUsageHistoryData{},
			want:    PulseIntelligenceAIUsageEvidence{},
		},
		{
			name: "event before cutoff is excluded",
			history: &config.AIUsageHistoryData{
				Events: []config.AIUsageEventRecord{
					{Timestamp: before, UseCase: "chat"},
				},
			},
			want: PulseIntelligenceAIUsageEvidence{},
		},
		{
			name: "zero timestamp event is excluded",
			history: &config.AIUsageHistoryData{
				Events: []config.AIUsageEventRecord{
					{Timestamp: time.Time{}, UseCase: "chat"},
				},
			},
			want: PulseIntelligenceAIUsageEvidence{},
		},
		{
			name: "event at cutoff is included (Before is strict)",
			history: &config.AIUsageHistoryData{
				Events: []config.AIUsageEventRecord{
					{Timestamp: atCutoff, UseCase: "patrol"},
				},
			},
			want: PulseIntelligenceAIUsageEvidence{PatrolAICalls: 1},
		},
		{
			name: "plain chat after cutoff increments AssistantAICalls only",
			history: &config.AIUsageHistoryData{
				Events: []config.AIUsageEventRecord{
					{Timestamp: after, UseCase: "chat"},
				},
			},
			want: PulseIntelligenceAIUsageEvidence{AssistantAICalls: 1},
		},
		{
			name: "chat with governed context scopes increments context counter",
			history: &config.AIUsageHistoryData{
				Events: []config.AIUsageEventRecord{
					{Timestamp: after, UseCase: "chat", ContextScope: "fleet"},
				},
			},
			want: PulseIntelligenceAIUsageEvidence{AssistantAICalls: 1, AssistantContextAICalls: 1},
		},
		{
			name: "chat with TargetType/TargetID/FindingID all trigger context detection",
			history: &config.AIUsageHistoryData{
				Events: []config.AIUsageEventRecord{
					{Timestamp: after, UseCase: "chat", TargetType: "vm"},
					{Timestamp: after, UseCase: "chat", TargetID: "node-1"},
					{Timestamp: after, UseCase: "chat", FindingID: "F-42"},
				},
			},
			want: PulseIntelligenceAIUsageEvidence{AssistantAICalls: 3, AssistantContextAICalls: 3},
		},
		{
			name: "whitespace-only context fields do not count as context",
			history: &config.AIUsageHistoryData{
				Events: []config.AIUsageEventRecord{
					{Timestamp: after, UseCase: "chat", ContextScope: "   "},
				},
			},
			want: PulseIntelligenceAIUsageEvidence{AssistantAICalls: 1},
		},
		{
			name: "chat ToolCallCount is summed across calls",
			history: &config.AIUsageHistoryData{
				Events: []config.AIUsageEventRecord{
					{Timestamp: after, UseCase: "chat", ToolCallCount: 3},
					{Timestamp: after, UseCase: "chat", ToolCallCount: 2},
					{Timestamp: after, UseCase: "chat"}, // zero ToolCallCount does not add
				},
			},
			want: PulseIntelligenceAIUsageEvidence{AssistantAICalls: 3, AssistantToolCalls: 5},
		},
		{
			name: "patrol use case increments PatrolAICalls only",
			history: &config.AIUsageHistoryData{
				Events: []config.AIUsageEventRecord{
					{Timestamp: after, UseCase: "patrol", ToolCallCount: 9, ContextScope: "fleet"},
				},
			},
			want: PulseIntelligenceAIUsageEvidence{PatrolAICalls: 1},
		},
		{
			name: "use case is case-insensitive and trimmed",
			history: &config.AIUsageHistoryData{
				Events: []config.AIUsageEventRecord{
					{Timestamp: after, UseCase: "  CHAT  "},
					{Timestamp: after, UseCase: "PaTrOl"},
				},
			},
			want: PulseIntelligenceAIUsageEvidence{AssistantAICalls: 1, PatrolAICalls: 1},
		},
		{
			name: "unknown use case is ignored",
			history: &config.AIUsageHistoryData{
				Events: []config.AIUsageEventRecord{
					{Timestamp: after, UseCase: "summarize"},
				},
			},
			want: PulseIntelligenceAIUsageEvidence{},
		},
		{
			name: "mixed inclusion: before-cutoff filtered, after-cutoff projected",
			history: &config.AIUsageHistoryData{
				Events: []config.AIUsageEventRecord{
					{Timestamp: before, UseCase: "chat", ContextScope: "fleet", ToolCallCount: 100},
					{Timestamp: after, UseCase: "chat", ContextScope: "host", ToolCallCount: 2},
					{Timestamp: after, UseCase: "patrol"},
				},
			},
			want: PulseIntelligenceAIUsageEvidence{
				AssistantAICalls:        1,
				AssistantContextAICalls: 1,
				AssistantToolCalls:      2,
				PatrolAICalls:           1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PulseIntelligenceAIUsageEvidenceFromHistory(tt.history, fixedCutoff)
			if got != tt.want {
				t.Fatalf("PulseIntelligenceAIUsageEvidenceFromHistory() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestPulseIntelligenceExternalAgentActivitySurface(t *testing.T) {
	tests := []struct {
		name    string
		surface string
		want    bool
	}{
		{"agent_api", config.ExternalAgentActivitySurfaceAgentAPI, true},
		{"pulse_mcp", config.ExternalAgentActivitySurfacePulseMCP, true},
		{"whitespace wrapped agent_api", "  " + config.ExternalAgentActivitySurfaceAgentAPI + "  ", true},
		{"unknown surface", "webhook", false},
		{"empty surface", "", false},
		{"whitespace only", "   ", false},
		{"workflow prompt agent_api constant shares the same surface value", config.WorkflowPromptActivitySurfaceAgentAPI, true}, // value is "agent_api"
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PulseIntelligenceExternalAgentActivitySurface(tt.surface); got != tt.want {
				t.Fatalf("PulseIntelligenceExternalAgentActivitySurface(%q) = %v, want %v", tt.surface, got, tt.want)
			}
		})
	}
}

func TestPulseIntelligenceExternalAgentEvidence_CollaborationActive(t *testing.T) {
	tests := []struct {
		name string
		e    PulseIntelligenceExternalAgentEvidence
		want bool
	}{
		{name: "zero evidence inactive", want: false},
		{name: "Used flag active", e: PulseIntelligenceExternalAgentEvidence{Used: true}, want: true},
		{name: "MCPAdapterUsed active", e: PulseIntelligenceExternalAgentEvidence{MCPAdapterUsed: true}, want: true},
		{name: "ContextRequests active", e: PulseIntelligenceExternalAgentEvidence{ContextRequests: 1}, want: true},
		{name: "EventStreamRequests active", e: PulseIntelligenceExternalAgentEvidence{EventStreamRequests: 1}, want: true},
		{name: "ProvisioningRequests active", e: PulseIntelligenceExternalAgentEvidence{ProvisioningRequests: 1}, want: true},
		{name: "OperatorStateRequests active", e: PulseIntelligenceExternalAgentEvidence{OperatorStateRequests: 1}, want: true},
		{name: "FindingRequests active", e: PulseIntelligenceExternalAgentEvidence{FindingRequests: 1}, want: true},
		{name: "ActionRequests active", e: PulseIntelligenceExternalAgentEvidence{ActionRequests: 1}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.e.CollaborationActive(); got != tt.want {
				t.Fatalf("CollaborationActive() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPulseIntelligenceExternalAgentEvidence_CollaborationCount(t *testing.T) {
	tests := []struct {
		name string
		e    PulseIntelligenceExternalAgentEvidence
		want int
	}{
		{name: "zero evidence count is zero", want: 0},
		{name: "request counts sum to total", e: PulseIntelligenceExternalAgentEvidence{
			ContextRequests: 2, EventStreamRequests: 3, ProvisioningRequests: 1,
			OperatorStateRequests: 1, FindingRequests: 1, ActionRequests: 2,
		}, want: 10},
		{name: "Used only collapses to coarse count 1", e: PulseIntelligenceExternalAgentEvidence{Used: true}, want: 1},
		{name: "MCPAdapterUsed only collapses to coarse count 1", e: PulseIntelligenceExternalAgentEvidence{MCPAdapterUsed: true}, want: 1},
		{name: "request counts take precedence over coarse Used flag", e: PulseIntelligenceExternalAgentEvidence{
			Used: true, ContextRequests: 4,
		}, want: 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.e.CollaborationCount(); got != tt.want {
				t.Fatalf("CollaborationCount() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestPulseIntelligenceExternalAgentEvidence_ApplyActivity(t *testing.T) {
	tests := []struct {
		name     string
		activity string
		want     PulseIntelligenceExternalAgentEvidence
	}{
		{"resource_context bumps ContextRequests", config.ExternalAgentActivityResourceContext, PulseIntelligenceExternalAgentEvidence{ContextRequests: 1}},
		{"fleet_context bumps ContextRequests", config.ExternalAgentActivityFleetContext, PulseIntelligenceExternalAgentEvidence{ContextRequests: 1}},
		{"event_stream bumps EventStreamRequests", config.ExternalAgentActivityEventStream, PulseIntelligenceExternalAgentEvidence{EventStreamRequests: 1}},
		{"provisioning bumps ProvisioningRequests", config.ExternalAgentActivityProvisioning, PulseIntelligenceExternalAgentEvidence{ProvisioningRequests: 1}},
		{"operator_state bumps OperatorStateRequests", config.ExternalAgentActivityOperatorState, PulseIntelligenceExternalAgentEvidence{OperatorStateRequests: 1}},
		{"finding_list bumps FindingRequests", config.ExternalAgentActivityFindingList, PulseIntelligenceExternalAgentEvidence{FindingRequests: 1}},
		{"finding_decision bumps FindingRequests", config.ExternalAgentActivityFindingDecision, PulseIntelligenceExternalAgentEvidence{FindingRequests: 1}},
		{"action_plan bumps ActionRequests", config.ExternalAgentActivityActionPlan, PulseIntelligenceExternalAgentEvidence{ActionRequests: 1}},
		{"action_decision bumps ActionRequests", config.ExternalAgentActivityActionDecision, PulseIntelligenceExternalAgentEvidence{ActionRequests: 1}},
		{"action_execute bumps ActionRequests", config.ExternalAgentActivityActionExecute, PulseIntelligenceExternalAgentEvidence{ActionRequests: 1}},
		{"whitespace wrapped activity still matches", "  " + config.ExternalAgentActivityEventStream + "  ", PulseIntelligenceExternalAgentEvidence{EventStreamRequests: 1}},
		{"unknown activity is a no-op", "nope", PulseIntelligenceExternalAgentEvidence{}},
		{"empty activity is a no-op", "", PulseIntelligenceExternalAgentEvidence{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var e PulseIntelligenceExternalAgentEvidence
			e.ApplyActivity(tt.activity)
			if e != tt.want {
				t.Fatalf("ApplyActivity(%q) -> %+v, want %+v", tt.activity, e, tt.want)
			}
		})
	}

	t.Run("nil receiver does not panic", func(t *testing.T) {
		var e *PulseIntelligenceExternalAgentEvidence
		// Must not panic; the guard is the only behaviour observable here.
		e.ApplyActivity(config.ExternalAgentActivityEventStream)
	})

	t.Run("repeated apply accumulates across buckets", func(t *testing.T) {
		var e PulseIntelligenceExternalAgentEvidence
		e.ApplyActivity(config.ExternalAgentActivityResourceContext)
		e.ApplyActivity(config.ExternalAgentActivityFleetContext)
		e.ApplyActivity(config.ExternalAgentActivityActionPlan)
		e.ApplyActivity(config.ExternalAgentActivityActionExecute)
		want := PulseIntelligenceExternalAgentEvidence{ContextRequests: 2, ActionRequests: 2}
		if e != want {
			t.Fatalf("accumulated evidence = %+v, want %+v", e, want)
		}
	})
}

func TestPulseIntelligenceExternalAgentEvidenceFromHistory(t *testing.T) {
	before := fixedCutoff.Add(-24 * time.Hour)
	atCutoff := fixedCutoff
	after := fixedCutoff.Add(24 * time.Hour)

	tests := []struct {
		name    string
		history *config.ExternalAgentActivityHistoryData
		want    PulseIntelligenceExternalAgentEvidence
	}{
		{
			name:    "nil history returns zero evidence",
			history: nil,
			want:    PulseIntelligenceExternalAgentEvidence{},
		},
		{
			name:    "empty events returns zero evidence",
			history: &config.ExternalAgentActivityHistoryData{},
			want:    PulseIntelligenceExternalAgentEvidence{},
		},
		{
			name: "event before cutoff is excluded",
			history: &config.ExternalAgentActivityHistoryData{
				Events: []config.ExternalAgentActivityRecord{
					{Timestamp: before, Surface: config.ExternalAgentActivitySurfaceAgentAPI, Activity: config.ExternalAgentActivityEventStream},
				},
			},
			want: PulseIntelligenceExternalAgentEvidence{},
		},
		{
			name: "zero timestamp event is excluded",
			history: &config.ExternalAgentActivityHistoryData{
				Events: []config.ExternalAgentActivityRecord{
					{Timestamp: time.Time{}, Surface: config.ExternalAgentActivitySurfaceAgentAPI, Activity: config.ExternalAgentActivityEventStream},
				},
			},
			want: PulseIntelligenceExternalAgentEvidence{},
		},
		{
			name: "unknown surface is filtered (Used stays false)",
			history: &config.ExternalAgentActivityHistoryData{
				Events: []config.ExternalAgentActivityRecord{
					{Timestamp: after, Surface: "webhook", Activity: config.ExternalAgentActivityEventStream},
				},
			},
			want: PulseIntelligenceExternalAgentEvidence{},
		},
		{
			name: "agent_api surface marks Used but not MCPAdapterUsed",
			history: &config.ExternalAgentActivityHistoryData{
				Events: []config.ExternalAgentActivityRecord{
					{Timestamp: after, Surface: config.ExternalAgentActivitySurfaceAgentAPI, Activity: config.ExternalAgentActivityEventStream},
				},
			},
			want: PulseIntelligenceExternalAgentEvidence{Used: true, EventStreamRequests: 1},
		},
		{
			name: "pulse_mcp surface marks Used and MCPAdapterUsed",
			history: &config.ExternalAgentActivityHistoryData{
				Events: []config.ExternalAgentActivityRecord{
					{Timestamp: after, Surface: config.ExternalAgentActivitySurfacePulseMCP, Activity: config.ExternalAgentActivityResourceContext},
				},
			},
			want: PulseIntelligenceExternalAgentEvidence{Used: true, MCPAdapterUsed: true, ContextRequests: 1},
		},
		{
			name: "at-cutoff event is included (Before is strict)",
			history: &config.ExternalAgentActivityHistoryData{
				Events: []config.ExternalAgentActivityRecord{
					{Timestamp: atCutoff, Surface: config.ExternalAgentActivitySurfaceAgentAPI, Activity: config.ExternalAgentActivityActionPlan},
				},
			},
			want: PulseIntelligenceExternalAgentEvidence{Used: true, ActionRequests: 1},
		},
		{
			name: "mixed events: before excluded, unknown surface filtered, multiple buckets accumulate",
			history: &config.ExternalAgentActivityHistoryData{
				Events: []config.ExternalAgentActivityRecord{
					{Timestamp: before, Surface: config.ExternalAgentActivitySurfacePulseMCP, Activity: config.ExternalAgentActivityResourceContext},
					{Timestamp: after, Surface: "intranet", Activity: config.ExternalAgentActivityEventStream},
					{Timestamp: after, Surface: config.ExternalAgentActivitySurfaceAgentAPI, Activity: config.ExternalAgentActivityFindingDecision},
					{Timestamp: after, Surface: config.ExternalAgentActivitySurfacePulseMCP, Activity: config.ExternalAgentActivityActionExecute},
					{Timestamp: after, Surface: config.ExternalAgentActivitySurfacePulseMCP, Activity: config.ExternalAgentActivityOperatorState},
				},
			},
			want: PulseIntelligenceExternalAgentEvidence{
				Used:                  true,
				MCPAdapterUsed:        true,
				FindingRequests:       1,
				ActionRequests:        1,
				OperatorStateRequests: 1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PulseIntelligenceExternalAgentEvidenceFromHistory(tt.history, fixedCutoff)
			if got != tt.want {
				t.Fatalf("PulseIntelligenceExternalAgentEvidenceFromHistory() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
