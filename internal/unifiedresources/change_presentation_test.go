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

func TestBuildIncidentTimelineChanges(t *testing.T) {
	alertChange := BuildAlertTimelineChange("vm-1", ChangeAlertResolved, time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC), "ops-user", AlertTimelineChange{
		AlertIdentifier: "alert-1",
		AlertType:       "cpu",
		AlertLevel:      "critical",
		AlertMessage:    "CPU normalized",
		AlertValue:      91.4,
		AlertThreshold:  80,
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
	if got := alertChange.Metadata[metadataAlertIdentifier]; got != "alert-1" {
		t.Fatalf("alert_identifier = %#v, want alert-1", got)
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
	if got := commandChange.Metadata[metadataOutputExcerpt].(string); len(got) <= resourceChangeOutputExcerptLimit {
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
	if got := runbookChange.Metadata[metadataRunbookID]; got != "rb-1" {
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
