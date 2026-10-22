package ai

import (
	"fmt"
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

func TestPatrolRunAssistantHandoffDoesNotCountScopeAliasesAsWorkloads(t *testing.T) {
	// Frozen source-ux8yc7zc producer semantics: one Docker host record, four
	// effective identities for container aliases and their transport context.
	ids := []string{"abc123def456", "agent-13d445d46d78a341", "app-container-c81d6ea4500a6178", "fixture-nas"}
	run := PatrolRunRecord{
		ID: "docker-scoped-error", Type: "scoped", Status: "error",
		ScopeResourceIDs:          []string{"docker:fixture-nas/abc123def456"},
		EffectiveScopeResourceIDs: ids, ResourcesChecked: 1, DockerChecked: 1,
		ErrorCount: 1, ErrorSummary: "Provider not ready", ErrorDetail: "chat service not available",
	}
	handoff := BuildPatrolRunAssistantHandoff(run)
	for _, required := range []string{"Checked 1 scoped resource", "Provider not ready", "chat service not available"} {
		if !strings.Contains(handoff.Context, required) {
			t.Fatalf("handoff missing %q: %s", required, handoff.Context)
		}
	}
	for _, invented := range []string{"of 4 scoped", "Scoped to 4 resources", "1 container"} {
		if strings.Contains(handoff.Context, invented) {
			t.Fatalf("handoff invents %q: %s", invented, handoff.Context)
		}
	}
	if len(handoff.Resources) != len(ids) {
		t.Fatalf("scope identities lost: %+v", handoff.Resources)
	}
	for i, id := range ids {
		if handoff.Resources[i].ID != id {
			t.Fatalf("scope identity changed: %+v", handoff.Resources)
		}
	}
	for _, n := range []int{0, 2} {
		run.ResourcesChecked = n
		want := fmt.Sprintf("Checked %d scoped resources", n)
		if got := patrolRunCoverageSummary(run); got != want {
			t.Fatalf("coverage = %q, want %q", got, want)
		}
	}
	for _, n := range []int{1, 2} {
		legacy := PatrolRunRecord{DockerChecked: n}
		want := fmt.Sprintf("%d Docker host", n)
		if n != 1 {
			want += "s"
		}
		if got := patrolRunCoverageSummary(legacy); got != want {
			t.Fatalf("legacy breakdown = %q, want %q", got, want)
		}
	}
}
