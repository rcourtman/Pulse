package ai

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/chat"
	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
)

func TestBuildPatrolRunAssistantHandoffUsesBackendSafeRunContext(t *testing.T) {
	run := PatrolRunRecord{
		ID:                        "run-runtime-error",
		StartedAt:                 time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC),
		CompletedAt:               time.Date(2026, 5, 7, 12, 0, 3, 0, time.UTC),
		DurationMs:                3000,
		Type:                      "scoped",
		TriggerReason:             "alert_fired",
		EffectiveScopeResourceIDs: []string{"vm-100"},
		ScopeResourceTypes:        []string{"vm"},
		ObjectiveContext: &aicontracts.PatrolObjectiveContext{
			ObjectiveID:         "objective-availability",
			Revision:            2,
			Brief:               "Keep the service available",
			ObservedResourceIDs: []string{"vm-100"},
			ObservedAt:          time.Date(2026, 5, 7, 11, 59, 0, 0, time.UTC),
		},
		ResourcesChecked: 1,
		GuestsChecked:    1,
		FindingsSummary:  "Runtime failure prevented analysis.",
		FindingIDs:       []string{},
		ErrorCount:       1,
		ErrorSummary:     "Selected model does not support Patrol tools",
		ErrorDetail:      `API error: No endpoints found that support the provided tool_choice value. Authorization: Bearer sk-live-secret`,
		Status:           "error",
		ToolCallCount:    1,
		AIAnalysis:       `<｜DSML｜trace>provider trace</｜DSML｜trace>Visible runtime summary. {"api_key":"sk-json-secret"}`,
	}

	handoff := BuildPatrolRunAssistantHandoff(run)

	if handoff.Metadata != (chat.HandoffMetadata{
		Kind:           "patrol_run",
		RunID:          "run-runtime-error",
		RunType:        "Targeted check",
		RunStatus:      "error",
		RuntimeFailure: true,
	}) {
		t.Fatalf("metadata = %+v", handoff.Metadata)
	}
	if got, want := handoff.Resources, []chat.HandoffResource{{ID: "vm-100", Type: "vm"}}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("resources = %+v, want %+v", got, want)
	}

	for _, required := range []string{
		"[Patrol Run Context]",
		"Source: Pulse Patrol run history",
		"Run ID: run-runtime-error",
		"Run Type: Targeted check",
		"Trigger: Alert fired",
		`Triggering Objective: objective-availability revision 2; desired outcome "Keep the service available"; 1 resource(s) outside the local predicate; observed 2026-05-07T11:59:00Z`,
		"Runtime Failure: Selected model does not support Patrol tools",
		"no tool-capable endpoint",
		"Patrol Analysis: Visible runtime summary.",
		"Model Boundary:",
	} {
		if !strings.Contains(handoff.Context, required) {
			t.Fatalf("context missing %q in:\n%s", required, handoff.Context)
		}
	}
	if strings.Contains(handoff.Context, "provider trace") {
		t.Fatalf("context leaked provider trace: %s", handoff.Context)
	}
	if strings.Contains(handoff.Context, "sk-live-secret") || strings.Contains(handoff.Context, "sk-json-secret") {
		t.Fatalf("context leaked secret-shaped provider detail: %s", handoff.Context)
	}
	if strings.Contains(handoff.Context, "tool_choice") || strings.Contains(handoff.Context, "No endpoints found") {
		t.Fatalf("context leaked raw provider routing detail: %s", handoff.Context)
	}
}
