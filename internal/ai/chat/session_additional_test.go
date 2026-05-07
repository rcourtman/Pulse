package chat

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSessionStore_KnowledgeAndToolSets(t *testing.T) {
	store, err := NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}

	session, err := store.Create()
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	ka1 := store.GetKnowledgeAccumulator(session.ID)
	ka2 := store.NewKnowledgeAccumulatorForRun(session.ID)
	if ka1 == ka2 {
		t.Fatalf("expected new knowledge accumulator for run")
	}

	toolSet := map[string]bool{"pulse_query": true}
	store.SetToolSet(session.ID, toolSet)
	got := store.GetToolSet(session.ID)
	if got == nil || !got["pulse_query"] {
		t.Fatalf("expected tool set entry")
	}
	got["pulse_query"] = false
	if store.GetToolSet(session.ID)["pulse_query"] != true {
		t.Fatalf("expected tool set to be copied")
	}

	updated := store.AddToolSet(session.ID, map[string]bool{"pulse_metrics": true})
	if !updated["pulse_metrics"] {
		t.Fatalf("expected tool set to include additions")
	}
}

func TestSessionStore_ModelHandoffContextLifecycle(t *testing.T) {
	store, err := NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}

	session, err := store.Create()
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	initial, err := store.GetModelHandoffContext(session.ID)
	if err != nil {
		t.Fatalf("GetModelHandoffContext failed: %v", err)
	}
	if initial != "" {
		t.Fatalf("initial handoff context = %q, want empty", initial)
	}
	initialFindingID, err := store.GetModelHandoffFindingID(session.ID)
	if err != nil {
		t.Fatalf("GetModelHandoffFindingID failed: %v", err)
	}
	if initialFindingID != "" {
		t.Fatalf("initial handoff finding ID = %q, want empty", initialFindingID)
	}

	if err := store.SetModelHandoffFindingID(session.ID, " finding-123 "); err != nil {
		t.Fatalf("SetModelHandoffFindingID failed: %v", err)
	}
	gotFindingID, err := store.GetModelHandoffFindingID(session.ID)
	if err != nil {
		t.Fatalf("GetModelHandoffFindingID failed: %v", err)
	}
	if gotFindingID != "finding-123" {
		t.Fatalf("handoff finding ID = %q, want finding-123", gotFindingID)
	}

	handoffContext := "  [Finding Context]\nID: finding-123\nConclusion: CPU saturated after backup.  "
	if err := store.SetModelHandoffContext(session.ID, handoffContext); err != nil {
		t.Fatalf("SetModelHandoffContext failed: %v", err)
	}
	got, err := store.GetModelHandoffContext(session.ID)
	if err != nil {
		t.Fatalf("GetModelHandoffContext failed: %v", err)
	}
	if got != strings.TrimSpace(handoffContext) {
		t.Fatalf("handoff context = %q, want trimmed %q", got, strings.TrimSpace(handoffContext))
	}

	handoffResources := []HandoffResource{
		{ID: " vm-100 ", Name: " web-server ", Type: " vm ", Node: " pve-1 "},
		{ID: "vm-100", Name: "web-server", Type: "vm", Node: "pve-1"},
	}
	if err := store.SetModelHandoffResources(session.ID, handoffResources); err != nil {
		t.Fatalf("SetModelHandoffResources failed: %v", err)
	}
	gotResources, err := store.GetModelHandoffResources(session.ID)
	if err != nil {
		t.Fatalf("GetModelHandoffResources failed: %v", err)
	}
	if len(gotResources) != 1 {
		t.Fatalf("handoff resources = %#v, want one normalized resource", gotResources)
	}
	if gotResources[0] != (HandoffResource{ID: "vm-100", Name: "web-server", Type: "vm", Node: "pve-1"}) {
		t.Fatalf("handoff resource = %#v, want normalized VM", gotResources[0])
	}

	handoffActions := []HandoffAction{
		{
			FindingID:          " finding-123 ",
			RecordID:           " record-123 ",
			ApprovalID:         " approval-123 ",
			FixID:              " fix-123 ",
			Description:        " Restart the workload service ",
			RiskLevel:          " medium ",
			Destructive:        true,
			TargetHost:         " pve-1 ",
			TargetResourceID:   " vm-100 ",
			TargetResourceName: " web-server ",
			TargetResourceType: " vm ",
			TargetNode:         " pve-1 ",
		},
		{
			FindingID:          "finding-123",
			RecordID:           "record-123",
			ApprovalID:         "approval-123",
			FixID:              "fix-123",
			Description:        "Restart the workload service",
			RiskLevel:          "medium",
			Destructive:        true,
			TargetHost:         "pve-1",
			TargetResourceID:   "vm-100",
			TargetResourceName: "web-server",
			TargetResourceType: "vm",
			TargetNode:         "pve-1",
		},
	}
	if err := store.SetModelHandoffActions(session.ID, handoffActions); err != nil {
		t.Fatalf("SetModelHandoffActions failed: %v", err)
	}
	gotActions, err := store.GetModelHandoffActions(session.ID)
	if err != nil {
		t.Fatalf("GetModelHandoffActions failed: %v", err)
	}
	if len(gotActions) != 1 {
		t.Fatalf("handoff actions = %#v, want one normalized action", gotActions)
	}
	if gotActions[0] != (HandoffAction{
		FindingID:          "finding-123",
		RecordID:           "record-123",
		ApprovalID:         "approval-123",
		FixID:              "fix-123",
		Description:        "Restart the workload service",
		RiskLevel:          "medium",
		Destructive:        true,
		TargetHost:         "pve-1",
		TargetResourceID:   "vm-100",
		TargetResourceName: "web-server",
		TargetResourceType: "vm",
		TargetNode:         "pve-1",
	}) {
		t.Fatalf("handoff action = %#v, want normalized pending action", gotActions[0])
	}

	reloadedStore, err := NewSessionStore(filepath.Dir(store.dataDir))
	if err != nil {
		t.Fatalf("failed to reload session store: %v", err)
	}
	reloadedResources, err := reloadedStore.GetModelHandoffResources(session.ID)
	if err != nil {
		t.Fatalf("GetModelHandoffResources after reload failed: %v", err)
	}
	if len(reloadedResources) != 1 || reloadedResources[0].ID != "vm-100" {
		t.Fatalf("reloaded handoff resources = %#v, want persisted VM reference", reloadedResources)
	}
	reloadedFindingID, err := reloadedStore.GetModelHandoffFindingID(session.ID)
	if err != nil {
		t.Fatalf("GetModelHandoffFindingID after reload failed: %v", err)
	}
	if reloadedFindingID != "finding-123" {
		t.Fatalf("reloaded handoff finding ID = %q, want finding-123", reloadedFindingID)
	}
	reloadedActions, err := reloadedStore.GetModelHandoffActions(session.ID)
	if err != nil {
		t.Fatalf("GetModelHandoffActions after reload failed: %v", err)
	}
	if len(reloadedActions) != 1 || reloadedActions[0].ApprovalID != "approval-123" {
		t.Fatalf("reloaded handoff actions = %#v, want persisted approval reference", reloadedActions)
	}

	if err := store.SetModelHandoffResources(session.ID, nil); err != nil {
		t.Fatalf("SetModelHandoffResources clear failed: %v", err)
	}
	gotResources, err = store.GetModelHandoffResources(session.ID)
	if err != nil {
		t.Fatalf("GetModelHandoffResources after resource clear failed: %v", err)
	}
	if len(gotResources) != 0 {
		t.Fatalf("handoff resources after resource clear = %#v, want empty", gotResources)
	}
	got, err = store.GetModelHandoffContext(session.ID)
	if err != nil {
		t.Fatalf("GetModelHandoffContext after resource clear failed: %v", err)
	}
	if got != strings.TrimSpace(handoffContext) {
		t.Fatalf("handoff context after resource clear = %q, want retained context", got)
	}
	gotFindingID, err = store.GetModelHandoffFindingID(session.ID)
	if err != nil {
		t.Fatalf("GetModelHandoffFindingID after resource clear failed: %v", err)
	}
	if gotFindingID != "finding-123" {
		t.Fatalf("handoff finding ID after resource clear = %q, want retained finding reference", gotFindingID)
	}
	gotActions, err = store.GetModelHandoffActions(session.ID)
	if err != nil {
		t.Fatalf("GetModelHandoffActions after resource clear failed: %v", err)
	}
	if len(gotActions) != 1 {
		t.Fatalf("expected resource clear to retain handoff actions, got %#v", gotActions)
	}
	if err := store.SetModelHandoffActions(session.ID, nil); err != nil {
		t.Fatalf("SetModelHandoffActions clear failed: %v", err)
	}
	gotActions, err = store.GetModelHandoffActions(session.ID)
	if err != nil {
		t.Fatalf("GetModelHandoffActions after action clear failed: %v", err)
	}
	if len(gotActions) != 0 {
		t.Fatalf("handoff actions after action clear = %#v, want empty", gotActions)
	}
	got, err = store.GetModelHandoffContext(session.ID)
	if err != nil {
		t.Fatalf("GetModelHandoffContext after action clear failed: %v", err)
	}
	if got != strings.TrimSpace(handoffContext) {
		t.Fatalf("handoff context after action clear = %q, want retained context", got)
	}
	if err := store.SetModelHandoffResources(session.ID, handoffResources); err != nil {
		t.Fatalf("SetModelHandoffResources restore failed: %v", err)
	}
	if err := store.SetModelHandoffActions(session.ID, handoffActions); err != nil {
		t.Fatalf("SetModelHandoffActions restore failed: %v", err)
	}

	if err := store.AddMessage(session.ID, Message{Role: "user", Content: "What happened?"}); err != nil {
		t.Fatalf("AddMessage failed: %v", err)
	}
	messages, err := store.GetMessages(session.ID)
	if err != nil {
		t.Fatalf("GetMessages failed: %v", err)
	}
	if len(messages) != 1 || messages[0].Content != "What happened?" {
		t.Fatalf("stored messages = %#v, want clean user prompt only", messages)
	}

	store.ClearSessionState(session.ID, true)
	got, err = store.GetModelHandoffContext(session.ID)
	if err != nil {
		t.Fatalf("GetModelHandoffContext after keep-pinned clear failed: %v", err)
	}
	if got == "" {
		t.Fatalf("expected keep-pinned context clear to retain model handoff")
	}
	gotResources, err = store.GetModelHandoffResources(session.ID)
	if err != nil {
		t.Fatalf("GetModelHandoffResources after keep-pinned clear failed: %v", err)
	}
	if len(gotResources) != 1 {
		t.Fatalf("expected keep-pinned context clear to retain handoff resources, got %#v", gotResources)
	}
	gotActions, err = store.GetModelHandoffActions(session.ID)
	if err != nil {
		t.Fatalf("GetModelHandoffActions after keep-pinned clear failed: %v", err)
	}
	if len(gotActions) != 1 {
		t.Fatalf("expected keep-pinned context clear to retain handoff actions, got %#v", gotActions)
	}
	gotFindingID, err = store.GetModelHandoffFindingID(session.ID)
	if err != nil {
		t.Fatalf("GetModelHandoffFindingID after keep-pinned clear failed: %v", err)
	}
	if gotFindingID != "finding-123" {
		t.Fatalf("expected keep-pinned context clear to retain finding reference, got %q", gotFindingID)
	}

	store.ClearSessionState(session.ID, false)
	got, err = store.GetModelHandoffContext(session.ID)
	if err != nil {
		t.Fatalf("GetModelHandoffContext after full clear failed: %v", err)
	}
	if got != "" {
		t.Fatalf("handoff context after full clear = %q, want empty", got)
	}
	gotResources, err = store.GetModelHandoffResources(session.ID)
	if err != nil {
		t.Fatalf("GetModelHandoffResources after full clear failed: %v", err)
	}
	if len(gotResources) != 0 {
		t.Fatalf("handoff resources after full clear = %#v, want empty", gotResources)
	}
	gotActions, err = store.GetModelHandoffActions(session.ID)
	if err != nil {
		t.Fatalf("GetModelHandoffActions after full clear failed: %v", err)
	}
	if len(gotActions) != 0 {
		t.Fatalf("handoff actions after full clear = %#v, want empty", gotActions)
	}
	gotFindingID, err = store.GetModelHandoffFindingID(session.ID)
	if err != nil {
		t.Fatalf("GetModelHandoffFindingID after full clear failed: %v", err)
	}
	if gotFindingID != "" {
		t.Fatalf("handoff finding ID after full clear = %q, want empty", gotFindingID)
	}
}

func TestSessionStore_ListIncludesSafeHandoffSummary(t *testing.T) {
	store, err := NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}

	session, err := store.Create()
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	if err := store.SetModelHandoffFindingID(session.ID, " finding-123 "); err != nil {
		t.Fatalf("SetModelHandoffFindingID failed: %v", err)
	}
	if err := store.SetModelHandoffContext(session.ID, "[Finding Context]\nID: finding-123\nConclusion: CPU saturated after backup."); err != nil {
		t.Fatalf("SetModelHandoffContext failed: %v", err)
	}
	if err := store.SetModelHandoffResources(session.ID, []HandoffResource{{
		ID:   " vm-100 ",
		Name: " web-server ",
		Type: " vm ",
		Node: " pve-1 ",
	}}); err != nil {
		t.Fatalf("SetModelHandoffResources failed: %v", err)
	}
	if err := store.SetModelHandoffActions(session.ID, []HandoffAction{{
		FindingID:              " finding-123 ",
		ApprovalID:             " approval-123 ",
		ApprovalStatus:         " pending ",
		ActionID:               " action-123 ",
		ActionState:            " awaiting_approval ",
		ActionRequiresApproval: true,
		ActionPreflight:        "systemctl restart web-server",
		ActionResult:           "restart succeeded",
		Description:            "Restart the workload service",
		RiskLevel:              " medium ",
		TargetResourceID:       " vm-100 ",
		TargetResourceName:     " web-server ",
		TargetResourceType:     " vm ",
		TargetNode:             " pve-1 ",
	}}); err != nil {
		t.Fatalf("SetModelHandoffActions failed: %v", err)
	}

	sessions, err := store.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("sessions = %#v, want one session", sessions)
	}

	summary := sessions[0].HandoffSummary
	if summary == nil {
		t.Fatalf("expected handoff summary")
	}
	if summary.Kind != sessionHandoffKindPatrolFinding {
		t.Fatalf("handoff kind = %q, want %q", summary.Kind, sessionHandoffKindPatrolFinding)
	}
	if summary.FindingID != "finding-123" {
		t.Fatalf("finding ID = %q, want finding-123", summary.FindingID)
	}
	if !summary.HasModelContext {
		t.Fatalf("expected summary to report model context")
	}
	if summary.ResourceCount != 1 || summary.PrimaryResource == nil || summary.PrimaryResource.ID != "vm-100" || summary.PrimaryResource.Name != "web-server" {
		t.Fatalf("resource summary = %#v, want normalized primary resource", summary)
	}
	if summary.ActionCount != 1 || !summary.RequiresApproval {
		t.Fatalf("action summary = %#v, want one approval-required action", summary)
	}
	if summary.LastKnownApprovalStatus != "pending" || summary.LastKnownActionState != "awaiting_approval" || summary.LastKnownActionRisk != "medium" {
		t.Fatalf("action status summary = %#v, want normalized last-known status fields", summary)
	}

	retrieved, err := store.Get(session.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if retrieved.HandoffSummary == nil || retrieved.HandoffSummary.FindingID != "finding-123" {
		t.Fatalf("retrieved handoff summary = %#v, want same safe finding summary", retrieved.HandoffSummary)
	}

	payload, err := json.Marshal(sessions)
	if err != nil {
		t.Fatalf("Marshal sessions failed: %v", err)
	}
	publicJSON := string(payload)
	for _, forbidden := range []string{
		"CPU saturated",
		"systemctl restart",
		"restart succeeded",
		"Restart the workload service",
	} {
		if strings.Contains(publicJSON, forbidden) {
			t.Fatalf("public session JSON leaked %q: %s", forbidden, publicJSON)
		}
	}
}

func TestSessionStore_ListIncludesSafePatrolRunHandoffSummary(t *testing.T) {
	store, err := NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}

	session, err := store.Create()
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	handoffContext := strings.Join([]string{
		"[Patrol Run Context]",
		"Source: Pulse Patrol run history",
		"Run ID: run-runtime-error",
		"Run Type: Scoped run",
		"Status: error",
		"Runtime Failure: Selected model does not support Patrol tools: No endpoints found that support the provided 'tool_choice' value.",
		"Patrol Analysis: Runtime failure prevented analysis.",
	}, "\n")
	if err := store.SetModelHandoffEnvelope(session.ID, "", handoffContext, []HandoffResource{{
		ID:   " vm-100 ",
		Type: " vm ",
	}}, nil, HandoffMetadata{
		Kind:           " patrol_run ",
		RunID:          " run-runtime-error ",
		RunType:        " Scoped run ",
		RunStatus:      " error ",
		RuntimeFailure: true,
	}); err != nil {
		t.Fatalf("SetModelHandoffEnvelope failed: %v", err)
	}

	sessions, err := store.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("sessions = %#v, want one session", sessions)
	}

	summary := sessions[0].HandoffSummary
	if summary == nil {
		t.Fatalf("expected handoff summary")
	}
	if summary.Kind != sessionHandoffKindPatrolRun {
		t.Fatalf("handoff kind = %q, want %q", summary.Kind, sessionHandoffKindPatrolRun)
	}
	if summary.FindingID != "" {
		t.Fatalf("finding ID = %q, want empty Patrol run summary", summary.FindingID)
	}
	if summary.RunID != "run-runtime-error" || summary.RunType != "Scoped run" || summary.RunStatus != "error" || !summary.RuntimeFailure {
		t.Fatalf("run summary = %#v, want safe Patrol run identity", summary)
	}
	if !summary.HasModelContext {
		t.Fatalf("expected summary to report model context")
	}
	if summary.ResourceCount != 1 || summary.PrimaryResource == nil || summary.PrimaryResource.ID != "vm-100" {
		t.Fatalf("resource summary = %#v, want normalized primary resource", summary)
	}
	if summary.ActionCount != 0 || summary.RequiresApproval {
		t.Fatalf("action summary = %#v, want review-only run context without action authority", summary)
	}

	payload, err := json.Marshal(sessions)
	if err != nil {
		t.Fatalf("Marshal sessions failed: %v", err)
	}
	publicJSON := string(payload)
	for _, forbidden := range []string{
		"Selected model",
		"No endpoints found",
		"tool_choice",
		"Runtime failure prevented analysis",
	} {
		if strings.Contains(publicJSON, forbidden) {
			t.Fatalf("public session JSON leaked %q: %s", forbidden, publicJSON)
		}
	}
}

func TestModelContextHandoffSummaryInfersPatrolRunMetadataForExistingSessions(t *testing.T) {
	summary := modelContextHandoffSummary(&sessionModelContext{
		HandoffContext: strings.Join([]string{
			"[Patrol Run Context]",
			"Source: Pulse Patrol run history",
			"Run ID: run-existing",
			"Run Type: Scheduled run",
			"Status: completed",
			"Runtime Failure: Selected model failed during Patrol execution.",
		}, "\n"),
	})
	if summary == nil {
		t.Fatalf("summary is nil")
	}
	if summary.Kind != sessionHandoffKindPatrolRun {
		t.Fatalf("handoff kind = %q, want %q", summary.Kind, sessionHandoffKindPatrolRun)
	}
	if summary.RunID != "run-existing" || summary.RunType != "Scheduled run" || summary.RunStatus != "completed" || !summary.RuntimeFailure {
		t.Fatalf("run summary = %#v, want inferred Patrol run identity", summary)
	}
}

func TestModelContextHandoffSummaryRequiresApprovalOnlyForCurrentPendingApproval(t *testing.T) {
	tests := []struct {
		name             string
		action           HandoffAction
		requiresApproval bool
	}{
		{
			name: "pending approval",
			action: HandoffAction{
				ApprovalID:             "approval-pending",
				ApprovalStatus:         "pending",
				ActionState:            "pending_approval",
				ActionRequiresApproval: true,
			},
			requiresApproval: true,
		},
		{
			name: "legacy awaiting approval state",
			action: HandoffAction{
				ApprovalID:             "approval-awaiting",
				ActionState:            "awaiting_approval",
				ActionRequiresApproval: true,
			},
			requiresApproval: true,
		},
		{
			name: "approved completed action",
			action: HandoffAction{
				ApprovalID:             "approval-approved",
				ApprovalStatus:         "approved",
				ActionState:            "completed",
				ActionRequiresApproval: true,
			},
			requiresApproval: false,
		},
		{
			name: "denied historical action",
			action: HandoffAction{
				ApprovalID:             "approval-denied",
				ApprovalStatus:         "denied",
				ActionState:            "rejected",
				ActionRequiresApproval: true,
			},
			requiresApproval: false,
		},
		{
			name: "approval reference without current pending state",
			action: HandoffAction{
				ApprovalID:             "approval-unknown",
				ActionRequiresApproval: true,
			},
			requiresApproval: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary := modelContextHandoffSummary(&sessionModelContext{
				HandoffActions: []HandoffAction{tt.action},
			})
			if summary == nil {
				t.Fatalf("summary is nil")
			}
			if summary.RequiresApproval != tt.requiresApproval {
				t.Fatalf("requires approval = %v, want %v; summary=%#v", summary.RequiresApproval, tt.requiresApproval, summary)
			}
		})
	}
}

func TestSessionStore_ClearModelHandoffContext(t *testing.T) {
	store, err := NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}
	session, err := store.Create()
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	if err := store.SetModelHandoffFindingID(session.ID, "finding-123"); err != nil {
		t.Fatalf("SetModelHandoffFindingID failed: %v", err)
	}
	if err := store.SetModelHandoffContext(session.ID, "[Finding Context]\nID: finding-123"); err != nil {
		t.Fatalf("SetModelHandoffContext failed: %v", err)
	}
	if err := store.SetModelHandoffResources(session.ID, []HandoffResource{{
		ID:   "vm-100",
		Name: "web-server",
		Type: "vm",
	}}); err != nil {
		t.Fatalf("SetModelHandoffResources failed: %v", err)
	}
	if err := store.SetModelHandoffActions(session.ID, []HandoffAction{{
		FindingID:  "finding-123",
		ApprovalID: "approval-123",
		FixID:      "fix-123",
	}}); err != nil {
		t.Fatalf("SetModelHandoffActions failed: %v", err)
	}
	if err := store.AddMessage(session.ID, Message{Role: "user", Content: "What changed?"}); err != nil {
		t.Fatalf("AddMessage failed: %v", err)
	}

	if err := store.ClearModelHandoffContext(session.ID); err != nil {
		t.Fatalf("ClearModelHandoffContext failed: %v", err)
	}

	if got, err := store.GetModelHandoffFindingID(session.ID); err != nil {
		t.Fatalf("GetModelHandoffFindingID failed: %v", err)
	} else if got != "" {
		t.Fatalf("handoff finding ID after clear = %q, want empty", got)
	}
	if got, err := store.GetModelHandoffContext(session.ID); err != nil {
		t.Fatalf("GetModelHandoffContext failed: %v", err)
	} else if got != "" {
		t.Fatalf("handoff context after clear = %q, want empty", got)
	}
	if got, err := store.GetModelHandoffResources(session.ID); err != nil {
		t.Fatalf("GetModelHandoffResources failed: %v", err)
	} else if len(got) != 0 {
		t.Fatalf("handoff resources after clear = %#v, want empty", got)
	}
	if got, err := store.GetModelHandoffActions(session.ID); err != nil {
		t.Fatalf("GetModelHandoffActions failed: %v", err)
	} else if len(got) != 0 {
		t.Fatalf("handoff actions after clear = %#v, want empty", got)
	}

	messages, err := store.GetMessages(session.ID)
	if err != nil {
		t.Fatalf("GetMessages failed: %v", err)
	}
	if len(messages) != 1 || messages[0].Content != "What changed?" {
		t.Fatalf("messages after handoff clear = %#v, want user history retained", messages)
	}
}

func TestSessionStore_ResolvedContextLifecycle(t *testing.T) {
	store, err := NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}

	session, err := store.Create()
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	res := &ResolvedResource{
		ResourceID:     "vm:node1:101",
		ResourceType:   "vm",
		Name:           "alpha",
		TargetHost:     "alpha",
		AllowedActions: []string{"start"},
	}
	store.AddResolvedResource(session.ID, res.Name, res)

	if _, err := store.ValidateResourceForAction(session.ID, res.ResourceID, "start"); err != nil {
		t.Fatalf("expected action to be allowed: %v", err)
	}
	if _, err := store.ValidateResourceForAction(session.ID, res.ResourceID, "stop"); err == nil {
		t.Fatalf("expected action to be blocked")
	}

	store.ClearResolvedContext(session.ID)
	if _, err := store.ValidateResourceForAction(session.ID, res.ResourceID, "start"); err == nil {
		t.Fatalf("expected resource to be unresolved after clear")
	}
}

func TestSessionStore_ClearSessionState(t *testing.T) {
	store, err := NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}

	session, err := store.Create()
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Set up context, FSM, and toolset
	res := &ResolvedResource{ResourceID: "node:node1", Name: "node1", ResourceType: "node"}
	store.AddResolvedResource(session.ID, res.Name, res)
	ctx := store.GetResolvedContext(session.ID)
	ctx.PinResource(res.ResourceID)

	fsm := store.GetSessionFSM(session.ID)
	fsm.State = StateVerifying
	store.SetToolSet(session.ID, map[string]bool{"pulse_query": true})
	store.GetKnowledgeAccumulator(session.ID)

	store.ClearSessionState(session.ID, true)
	if !store.GetResolvedContext(session.ID).HasAnyResources() {
		t.Fatalf("expected pinned resources to remain")
	}
	if fsm.State != StateReading {
		t.Fatalf("expected FSM to keep progress when pinned resources remain")
	}
	if store.GetToolSet(session.ID) == nil {
		t.Fatalf("expected toolset to remain when keepPinned=true")
	}

	store.ClearSessionState(session.ID, false)
	if store.GetToolSet(session.ID) != nil {
		t.Fatalf("expected toolset to be cleared when keepPinned=false")
	}
}

func TestSessionStore_ResetFSMAndCleanupContext(t *testing.T) {
	store, err := NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}

	session, err := store.Create()
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	fsm := store.GetSessionFSM(session.ID)
	fsm.State = StateVerifying
	store.ResetSessionFSM(session.ID, true)
	if fsm.State != StateReading {
		t.Fatalf("expected ResetSessionFSM keep progress to move to READING")
	}

	fsm.State = StateVerifying
	store.ResetSessionFSM(session.ID, false)
	if fsm.State != StateResolving {
		t.Fatalf("expected ResetSessionFSM full reset to move to RESOLVING")
	}

	store.AddResolvedResource(session.ID, "node1", &ResolvedResource{ResourceID: "node:node1", Name: "node1"})
	store.cleanupResolvedContext(session.ID)
	if store.GetResolvedContext(session.ID).HasAnyResources() {
		t.Fatalf("expected cleanupResolvedContext to remove resources")
	}
}

func TestSessionStore_RejectsInvalidSessionIDs(t *testing.T) {
	root := t.TempDir()
	store, err := NewSessionStore(root)
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}

	type op func(string) error
	ops := []struct {
		name string
		run  op
	}{
		{
			name: "get",
			run: func(id string) error {
				_, err := store.Get(id)
				return err
			},
		},
		{
			name: "delete",
			run: func(id string) error {
				return store.Delete(id)
			},
		},
		{
			name: "add_message",
			run: func(id string) error {
				return store.AddMessage(id, Message{Role: "user", Content: "test"})
			},
		},
		{
			name: "ensure_session",
			run: func(id string) error {
				_, err := store.EnsureSession(id)
				return err
			},
		},
	}

	badIDs := []string{
		"../escape",
		"..\\escape",
		"nested/session",
		"contains space",
		"evil.json",
		strings.Repeat("a", maxSessionIDLength+1),
	}

	for _, tc := range ops {
		for _, id := range badIDs {
			err := tc.run(id)
			if err == nil {
				t.Fatalf("%s should reject invalid id %q", tc.name, id)
			}
			if !strings.Contains(err.Error(), "invalid session id") {
				t.Fatalf("%s returned unexpected error for id %q: %v", tc.name, id, err)
			}
		}
	}

	// Ensure traversal-style IDs never created files outside ai_sessions.
	if _, err := os.Stat(filepath.Join(root, "escape.json")); !os.IsNotExist(err) {
		t.Fatalf("expected no escaped session file to be created, stat err=%v", err)
	}
}
