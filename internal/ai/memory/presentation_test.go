package memory

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestChangeTypeLabel(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		kind ChangeType
		want string
	}{
		{name: "created", kind: ChangeCreated, want: "Created"},
		{name: "deleted", kind: ChangeDeleted, want: "Deleted"},
		{name: "restart", kind: ChangeRestarted, want: "Restart"},
		{name: "fallback", kind: ChangeType("custom_type"), want: "Custom type"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := ChangeTypeLabel(tc.kind); got != tc.want {
				t.Fatalf("ChangeTypeLabel(%q) = %q, want %q", tc.kind, got, tc.want)
			}
		})
	}
}

func TestFormatRecentChangesContext(t *testing.T) {
	t.Parallel()

	now := time.Now()
	changes := []Change{
		{
			ResourceID:   "res-0",
			ResourceType: "node",
			ResourceName: "lb-01",
			ChangeType:   ChangeCreated,
			Description:  "came online",
			DetectedAt:   now.Add(-30 * time.Second),
		},
		{
			ResourceID:   "res-1",
			ResourceType: "vm",
			ResourceName: "web-01",
			ChangeType:   ChangeRestarted,
			Description:  "restarted after maintenance",
			DetectedAt:   now.Add(-90 * time.Minute),
		},
		{
			ResourceName: "cache-1",
			ResourceType: "container",
			ChangeType:   ChangeConfig,
			Description:  "adjusted memory limit",
			DetectedAt:   now.Add(-30 * time.Minute),
		},
	}

	got := FormatRecentChangesContext(changes, true, "###")
	for _, want := range []string{
		"### Recent Changes Across Infrastructure",
		"res-0 (node): **Created** came online (just now)",
		"res-1 (vm): **Restart** restarted after maintenance (1 hour ago)",
		"cache-1 (container): **Config update** adjusted memory limit (30 minutes ago)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("FormatRecentChangesContext output %q does not contain %q", got, want)
		}
	}
}

func TestChangeFromUnifiedResourceChange(t *testing.T) {
	t.Parallel()

	observedAt := time.Date(2026, time.March, 19, 12, 30, 0, 0, time.UTC)
	occurredAt := observedAt.Add(-5 * time.Minute)
	change := unifiedresources.ResourceChange{
		ID:               "chg-1",
		ObservedAt:       observedAt,
		OccurredAt:       &occurredAt,
		ResourceID:       "res-1",
		Kind:             unifiedresources.ChangeRestart,
		From:             " old-state ",
		To:               " new-state ",
		SourceType:       unifiedresources.SourcePulseDiff,
		SourceAdapter:    unifiedresources.AdapterOpsAgent,
		Actor:            " agent:ops-helper ",
		Reason:           " restarted after maintenance ",
		RelatedResources: []string{"", " related-1 "},
	}

	got := ChangeFromUnifiedResourceChange(change)
	if got.ID != "chg-1" {
		t.Fatalf("ChangeFromUnifiedResourceChange ID = %q, want chg-1", got.ID)
	}
	if got.ResourceID != "res-1" {
		t.Fatalf("ChangeFromUnifiedResourceChange ResourceID = %q, want res-1", got.ResourceID)
	}
	if got.ResourceName != "res-1" {
		t.Fatalf("ChangeFromUnifiedResourceChange ResourceName = %q, want res-1", got.ResourceName)
	}
	if got.ChangeType != ChangeType("Restart") {
		t.Fatalf("ChangeFromUnifiedResourceChange ChangeType = %q, want Restart", got.ChangeType)
	}
	if !got.DetectedAt.Equal(observedAt) {
		t.Fatalf("ChangeFromUnifiedResourceChange DetectedAt = %v, want %v", got.DetectedAt, observedAt)
	}
	if got.Description != unifiedresources.FormatResourceChangeSummary(change) {
		t.Fatalf("ChangeFromUnifiedResourceChange Description = %q, want canonical summary", got.Description)
	}
}

func TestResourceChangeFromMemoryChange(t *testing.T) {
	t.Parallel()

	detectedAt := time.Date(2026, time.March, 19, 13, 15, 0, 0, time.UTC)
	change := Change{
		ID:           "mem-1",
		ResourceID:   "res-2",
		ResourceName: "rel-2",
		ChangeType:   ChangeMigrated,
		DetectedAt:   detectedAt,
		Description:  "relocated",
	}

	got := ResourceChangeFromMemoryChange(change)
	if got.ID != "mem-1" {
		t.Fatalf("ResourceChangeFromMemoryChange ID = %q, want mem-1", got.ID)
	}
	if got.ObservedAt != detectedAt {
		t.Fatalf("ResourceChangeFromMemoryChange ObservedAt = %v, want %v", got.ObservedAt, detectedAt)
	}
	if got.ResourceID != "res-2" {
		t.Fatalf("ResourceChangeFromMemoryChange ResourceID = %q, want res-2", got.ResourceID)
	}
	if got.Kind != unifiedresources.ChangeRelationship {
		t.Fatalf("ResourceChangeFromMemoryChange Kind = %q, want %q", got.Kind, unifiedresources.ChangeRelationship)
	}
	if got.SourceType != unifiedresources.SourceHeuristic {
		t.Fatalf("ResourceChangeFromMemoryChange SourceType = %q, want %q", got.SourceType, unifiedresources.SourceHeuristic)
	}
	if got.Confidence != unifiedresources.ConfidenceMedium {
		t.Fatalf("ResourceChangeFromMemoryChange Confidence = %q, want %q", got.Confidence, unifiedresources.ConfidenceMedium)
	}
	if len(got.RelatedResources) != 1 || got.RelatedResources[0] != "rel-2" {
		t.Fatalf("ResourceChangeFromMemoryChange RelatedResources = %#v, want [rel-2]", got.RelatedResources)
	}
	if got.Reason != "relocated" {
		t.Fatalf("ResourceChangeFromMemoryChange Reason = %q, want relocated", got.Reason)
	}
	if got.Metadata["memoryChangeType"] != string(change.ChangeType) {
		t.Fatalf("ResourceChangeFromMemoryChange Metadata = %#v, want memoryChangeType=%q", got.Metadata, change.ChangeType)
	}
}
