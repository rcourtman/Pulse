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
