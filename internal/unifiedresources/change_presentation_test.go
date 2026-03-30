package unifiedresources

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"
)

func TestChangeKindLabel(t *testing.T) {
	cases := map[ChangeKind]string{
		ChangeStateTransition:     "State transition",
		ChangeActivity:            "Activity",
		ChangeRestart:             "Restart",
		ChangeConfigUpdate:        "Config update",
		ChangeAnomaly:             "Metric anomaly",
		ChangeRelationship:        "Relationship change",
		ChangeCapability:          "Capability change",
		ChangeAlertFired:          "Alert fired",
		ChangeAlertAcknowledged:   "Alert acknowledged",
		ChangeAlertUnacknowledged: "Alert unacknowledged",
		ChangeAlertResolved:       "Alert resolved",
		ChangeCommandExecuted:     "Command executed",
		ChangeRunbookExecuted:     "Runbook executed",
		ChangeKind("custom_kind"): "Custom kind",
		ChangeKind(""):            "Change",
	}

	for kind, want := range cases {
		if got := ChangeKindLabel(kind); got != want {
			t.Fatalf("ChangeKindLabel(%q) = %q, want %q", kind, got, want)
		}
	}
}

func TestDescribeChange(t *testing.T) {
	change := ResourceChange{
		Kind:             ChangeRelationship,
		From:             "  node-a  ",
		To:               "node-b",
		SourceType:       SourcePulseDiff,
		SourceAdapter:    AdapterOpsAgent,
		Actor:            " agent:ops-helper ",
		Reason:           "  config refresh  ",
		RelatedResources: []string{"", " related-1 ", "related-2"},
	}

	presentation := DescribeChange(change)
	if presentation.KindLabel != "Relationship change" {
		t.Fatalf("unexpected kind label: %q", presentation.KindLabel)
	}
	if presentation.From != "node-a" {
		t.Fatalf("unexpected from value: %q", presentation.From)
	}
	if presentation.To != "node-b" {
		t.Fatalf("unexpected to value: %q", presentation.To)
	}
	if presentation.SourceType != "pulse_diff" {
		t.Fatalf("unexpected source type value: %q", presentation.SourceType)
	}
	if presentation.SourceAdapter != "agent:ops-helper" {
		t.Fatalf("unexpected source adapter value: %q", presentation.SourceAdapter)
	}
	if presentation.Actor != "agent:ops-helper" {
		t.Fatalf("unexpected actor value: %q", presentation.Actor)
	}
	if presentation.Reason != "config refresh" {
		t.Fatalf("unexpected reason value: %q", presentation.Reason)
	}
	if len(presentation.RelatedResources) != 2 || presentation.RelatedResources[0] != "related-1" || presentation.RelatedResources[1] != "related-2" {
		t.Fatalf("unexpected related resources: %#v", presentation.RelatedResources)
	}
}

func TestResourceChangeSummaries(t *testing.T) {
	if got := resourceStateSummary(Resource{}); got != "unknown" {
		t.Fatalf("resourceStateSummary(empty) = %q, want unknown", got)
	}

	if got := resourceRestartSummary(Resource{Status: StatusWarning}); got != "warning" {
		t.Fatalf("resourceRestartSummary(status) = %q, want warning", got)
	}

	incidentSummary := resourceIncidentSummary(Resource{Incidents: []ResourceIncident{
		{Code: "alpha", Severity: storagehealth.RiskCritical},
		{Code: "beta", Summary: "needs attention"},
	}})
	if incidentSummary != "alpha[critical], beta:needs attention" {
		t.Fatalf("resourceIncidentSummary() = %q, want canonical labels", incidentSummary)
	}

	if got := resourceIncidentSummaryFromSlice(nil); got != "none" {
		t.Fatalf("resourceIncidentSummaryFromSlice(nil) = %q, want none", got)
	}

	if got := resourceIncidentLabel(ResourceIncident{}); got != "incident" {
		t.Fatalf("resourceIncidentLabel(empty) = %q, want incident", got)
	}

	if got := resourceConfigSummary(Resource{Type: "vm", Technology: "proxmox", Name: "vm-1", CustomURL: "https://example"}); got != "vm|proxmox|vm-1|https://example" {
		t.Fatalf("resourceConfigSummary() = %q, want canonical config summary", got)
	}
}

func TestFormatResourceChangeSummary(t *testing.T) {
	cases := []struct {
		name   string
		change ResourceChange
		wants  []string
	}{
		{
			name: "recent",
			change: ResourceChange{
				Kind:             ChangeRestart,
				From:             "running",
				To:               "restarting",
				SourceType:       SourcePlatformEvent,
				SourceAdapter:    AdapterProxmox,
				Actor:            "agent:oncall-helper",
				Reason:           "Routine restart requested",
				RelatedResources: []string{"node-1"},
				ObservedAt:       time.Now().Add(-2 * time.Hour),
			},
			wants: []string{
				"**Restart**",
				"running → restarting",
				"platform_event/proxmox_adapter",
				"actor agent:oncall-helper",
				"Routine restart requested",
				"related: node-1",
				"2 hours ago",
			},
		},
		{
			name: "just now",
			change: ResourceChange{
				Kind:          ChangeConfigUpdate,
				From:          "old",
				To:            "new",
				SourceType:    SourcePulseDiff,
				SourceAdapter: AdapterOpsAgent,
				ObservedAt:    time.Now().Add(-30 * time.Second),
			},
			wants: []string{
				"just now",
			},
		},
		{
			name: "alert fired",
			change: ResourceChange{
				Kind:       ChangeAlertFired,
				SourceType: SourceHeuristic,
				Reason:     "CPU usage exceeded threshold",
				ObservedAt: time.Now().Add(-15 * time.Minute),
			},
			wants: []string{
				"**Alert fired**",
				"heuristic",
				"CPU usage exceeded threshold",
			},
		},
		{
			name: "provider activity",
			change: ResourceChange{
				Kind:          ChangeActivity,
				SourceType:    SourcePlatformEvent,
				SourceAdapter: AdapterVMware,
				Reason:        "Create snapshot (success)",
				ObservedAt:    time.Now().Add(-10 * time.Minute),
			},
			wants: []string{
				"**Activity**",
				"platform_event/vmware_adapter",
				"Create snapshot (success)",
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			summary := FormatResourceChangeSummary(tc.change)
			for _, want := range tc.wants {
				if !strings.Contains(summary, want) {
					t.Fatalf("expected summary %q to contain %q", summary, want)
				}
			}
			if strings.Contains(summary, "just now ago") {
				t.Fatalf("summary should not contain duplicated ago wording: %q", summary)
			}
		})
	}
}

func TestBuildPlatformActivityChange(t *testing.T) {
	occurredAt := time.Date(2026, 3, 30, 18, 12, 0, 0, time.UTC)
	change := BuildPlatformActivityChange(" vc-1:vm:vm-201 ", PlatformActivityChange{
		SourceAdapter: AdapterVMware,
		ActivityType:  "vmware_task",
		NativeID:      "task-201",
		Title:         "Create snapshot",
		State:         "success",
		Message:       "Snapshot completed successfully",
		Actor:         "vpxuser",
		OccurredAt:    occurredAt,
		RelatedResources: []string{
			" vc-1:host:host-101 ",
			"",
			"vc-1:storage:datastore-11",
		},
		Metadata: map[string]any{
			"vmwareConnectionId": "vc-1",
		},
	})
	if change == nil {
		t.Fatal("expected activity change")
	}
	if change.Kind != ChangeActivity {
		t.Fatalf("Kind = %q, want %q", change.Kind, ChangeActivity)
	}
	if change.SourceType != SourcePlatformEvent {
		t.Fatalf("SourceType = %q, want %q", change.SourceType, SourcePlatformEvent)
	}
	if change.SourceAdapter != AdapterVMware {
		t.Fatalf("SourceAdapter = %q, want %q", change.SourceAdapter, AdapterVMware)
	}
	if change.Reason != "Create snapshot (success)" {
		t.Fatalf("Reason = %q, want %q", change.Reason, "Create snapshot (success)")
	}
	if change.OccurredAt == nil || !change.OccurredAt.Equal(occurredAt) {
		t.Fatalf("OccurredAt = %v, want %v", change.OccurredAt, occurredAt)
	}
	if got := change.Metadata[MetadataActivityType]; got != "vmware_task" {
		t.Fatalf("activity_type = %#v, want vmware_task", got)
	}
	if got := change.Metadata[MetadataActivityNativeID]; got != "task-201" {
		t.Fatalf("activity_native_id = %#v, want task-201", got)
	}
	if got := change.Metadata[MetadataActivityTitle]; got != "Create snapshot" {
		t.Fatalf("activity_title = %#v, want Create snapshot", got)
	}
	if got := change.Metadata[MetadataActivityState]; got != "success" {
		t.Fatalf("activity_state = %#v, want success", got)
	}
	if got := change.Metadata[MetadataActivityMessage]; got != "Snapshot completed successfully" {
		t.Fatalf("activity_message = %#v, want Snapshot completed successfully", got)
	}
	if len(change.RelatedResources) != 2 || change.RelatedResources[0] != "vc-1:host:host-101" || change.RelatedResources[1] != "vc-1:storage:datastore-11" {
		t.Fatalf("RelatedResources = %#v, want canonical related IDs", change.RelatedResources)
	}

	repeat := BuildPlatformActivityChange("vc-1:vm:vm-201", PlatformActivityChange{
		SourceAdapter: AdapterVMware,
		ActivityType:  "vmware_task",
		NativeID:      "task-201",
		Title:         "Create snapshot",
		State:         "success",
		Message:       "Snapshot completed successfully",
		OccurredAt:    occurredAt,
	})
	if repeat == nil {
		t.Fatal("expected deterministic repeat activity change")
	}
	if repeat.ID != change.ID {
		t.Fatalf("repeat ID = %q, want %q", repeat.ID, change.ID)
	}
}

func TestBuildIncidentTimelineChanges(t *testing.T) {
	alertChange := BuildAlertTimelineChange("vm-1", ChangeAlertResolved, time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC), "ops-user", AlertTimelineChange{
		AlertIdentifier: "alert-1",
		AlertType:       "cpu",
		AlertLevel:      "critical",
		AlertMessage:    "CPU normalized",
		AlertValue:      91.4,
		AlertThreshold:  80,
		AlertMetadata: map[string]any{
			"incidentCategory": "health",
			"canonicalSpecID":  "alertspec:provider-incident:test",
		},
	})
	if alertChange == nil {
		t.Fatal("expected alert change")
	}
	if alertChange.Kind != ChangeAlertResolved {
		t.Fatalf("Kind = %q, want %q", alertChange.Kind, ChangeAlertResolved)
	}
	if alertChange.SourceType != SourceHeuristic {
		t.Fatalf("SourceType = %q, want %q", alertChange.SourceType, SourceHeuristic)
	}
	if got := alertChange.Metadata[MetadataAlertIdentifier]; got != "alert-1" {
		t.Fatalf("alert_identifier = %#v, want alert-1", got)
	}
	if got := alertChange.Metadata["incidentCategory"]; got != "health" {
		t.Fatalf("incidentCategory = %#v, want health", got)
	}
	if got := alertChange.Metadata["canonicalSpecID"]; got != "alertspec:provider-incident:test" {
		t.Fatalf("canonicalSpecID = %#v, want alertspec:provider-incident:test", got)
	}

	commandChange := BuildCommandExecutionChange("vm-1", "alert-1", "agent:pulse-assistant", "systemctl restart nginx", true, strings.Repeat("x", 700), map[string]any{
		"resource_type": "vm",
	})
	if commandChange == nil {
		t.Fatal("expected command change")
	}
	if commandChange.Kind != ChangeCommandExecuted {
		t.Fatalf("Kind = %q, want %q", commandChange.Kind, ChangeCommandExecuted)
	}
	if commandChange.SourceType != SourceAgentAction {
		t.Fatalf("SourceType = %q, want %q", commandChange.SourceType, SourceAgentAction)
	}
	if got := commandChange.Metadata[MetadataOutputExcerpt].(string); len(got) <= resourceChangeOutputExcerptLimit {
		t.Fatalf("expected truncated output excerpt to include ellipsis, got length %d", len(got))
	}

	runbookChange := BuildRunbookExecutionChange("vm-1", "alert-1", "agent:pulse-patrol", "rb-1", "Restart service", "resolved", true, "Recovered", nil)
	if runbookChange == nil {
		t.Fatal("expected runbook change")
	}
	if runbookChange.Kind != ChangeRunbookExecuted {
		t.Fatalf("Kind = %q, want %q", runbookChange.Kind, ChangeRunbookExecuted)
	}
	if runbookChange.SourceType != SourceAgentAction {
		t.Fatalf("SourceType = %q, want %q", runbookChange.SourceType, SourceAgentAction)
	}
	if got := runbookChange.Metadata[MetadataRunbookID]; got != "rb-1" {
		t.Fatalf("runbook_id = %#v, want rb-1", got)
	}
}

func TestFormatResourceRecentChangesContext(t *testing.T) {
	changes := []ResourceChange{
		{
			Kind:          ChangeRestart,
			ResourceID:    "vm-1",
			SourceType:    SourcePlatformEvent,
			SourceAdapter: AdapterProxmox,
			Reason:        "maintenance",
			ObservedAt:    time.Now().Add(-time.Hour),
		},
	}

	ctx := FormatResourceRecentChangesContext(changes, true, "###")
	for _, want := range []string{
		"### Recent Changes Across Infrastructure",
		"vm-1: **Restart**",
		"platform_event/proxmox_adapter",
	} {
		if !strings.Contains(ctx, want) {
			t.Fatalf("expected recent changes context %q to contain %q", ctx, want)
		}
	}
}
