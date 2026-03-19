package unifiedresources

import (
	"strings"
	"testing"
	"time"
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

func TestFormatResourceChangeSummary(t *testing.T) {
	change := ResourceChange{
		Kind:             ChangeRestart,
		From:             "running",
		To:               "restarting",
		SourceType:       SourcePlatformEvent,
		SourceAdapter:    AdapterProxmox,
		Actor:            "agent:oncall-helper",
		Reason:           "Routine restart requested",
		RelatedResources: []string{"node-1"},
		ObservedAt:       time.Now().Add(-2 * time.Hour),
	}

	summary := FormatResourceChangeSummary(change)
	for _, want := range []string{
		"**Restart**",
		"running → restarting",
		"platform_event/proxmox_adapter",
		"actor agent:oncall-helper",
		"Routine restart requested",
		"related: node-1",
		"2 hours ago",
	} {
		if !strings.Contains(summary, want) {
			t.Fatalf("expected summary %q to contain %q", summary, want)
		}
	}
}
