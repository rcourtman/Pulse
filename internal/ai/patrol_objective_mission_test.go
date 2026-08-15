package ai

import (
	"slices"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
)

func objectivePlanningScopeForTest() *PatrolScope {
	return &PatrolScope{
		Reason:      TriggerReasonObjectiveChanged,
		ResourceIDs: []string{"network-endpoint-jellyfin"},
		ObjectiveContext: &aicontracts.PatrolObjectiveContext{
			ObjectiveID: "objective-jellyfin",
			Revision:    1,
			Brief:       "Keep playback smooth",
		},
	}
}

func TestPatrolObjectivePlanningScopeHasOneMissionAndLeastAuthority(t *testing.T) {
	scope := objectivePlanningScopeForTest()
	if !isPatrolObjectivePlanningScope(scope) {
		t.Fatal("objective_changed scope was not classified as objective planning")
	}

	prompt := NewPatrolService(nil, nil).getPatrolSystemPromptForScope(scope)
	for _, required := range []string{"one mission", "patrol_propose_observer once", "proxy evidence", "untrusted data"} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("objective planning prompt missing %q: %s", required, prompt)
		}
	}
	for _, forbidden := range []string{"patrol_report_finding", "patrol_assess_finding", "Patrol Control Mode"} {
		if strings.Contains(prompt, forbidden) {
			t.Fatalf("objective planning prompt contains competing instruction %q: %s", forbidden, prompt)
		}
	}

	allowed := patrolAllowedToolNamesForScope(scope)
	if !slices.Contains(allowed, agentcapabilities.PatrolProposeObserverToolName) {
		t.Fatalf("objective planning tools = %v, observer proposal missing", allowed)
	}
	for _, forbidden := range []string{
		agentcapabilities.PatrolGetFindingsToolName,
		agentcapabilities.PatrolReportFindingToolName,
		agentcapabilities.PatrolAssessFindingToolName,
		agentcapabilities.PatrolResolveFindingToolName,
		agentcapabilities.PulseControlToolName,
		agentcapabilities.PulseRunCommandToolName,
	} {
		if slices.Contains(allowed, forbidden) {
			t.Fatalf("objective planning tools unexpectedly include %q: %v", forbidden, allowed)
		}
	}

	if got := patrolAllowedToolNamesForScope(&PatrolScope{Reason: TriggerReasonManual}); got != nil {
		t.Fatalf("ordinary Patrol tool manifest was restricted: %v", got)
	}
}

func TestPatrolObjectivePlanningSeedExcludesGlobalFindingLifecycle(t *testing.T) {
	patrol := NewPatrolService(nil, nil)
	patrol.findings.Add(&Finding{
		ID:           "runtime-provider-billing",
		Key:          patrolRuntimeFindingKey,
		Severity:     FindingSeverityWarning,
		Category:     FindingCategoryReliability,
		ResourceID:   patrolRuntimeResourceID,
		ResourceName: "Pulse Patrol Service",
		ResourceType: "service",
		Title:        "Provider billing issue",
	})

	sections, seeded := patrol.buildTriageSeedSectionsState(&TriageResult{}, patrolRuntimeState{}, objectivePlanningScopeForTest(), nil)
	if len(seeded) != 0 {
		t.Fatalf("objective planning seeded unrelated finding IDs: %v", seeded)
	}
	for _, section := range sections {
		if section.name == "findings" && strings.TrimSpace(section.content) != "" {
			t.Fatalf("objective planning seeded global finding lifecycle context: %s", section.content)
		}
	}
}
