package unifiedresources

import (
	"testing"
	"time"
)

func TestRelationshipTypeLabel(t *testing.T) {
	cases := map[RelationshipType]string{
		RelRunsOn:                       "Runs on",
		RelDependsOn:                    "Depends on",
		RelMountedTo:                    "Mounted to",
		RelExposedBy:                    "Exposed by",
		RelOwnedBy:                      "Owned by",
		RelAttachedTo:                   "Attached to",
		RelationshipType("custom_link"): "Custom link",
		RelationshipType(""):            "Related to",
	}

	for relationType, want := range cases {
		if got := RelationshipTypeLabel(relationType); got != want {
			t.Fatalf("RelationshipTypeLabel(%q) = %q, want %q", relationType, got, want)
		}
	}
}

func TestDescribeRelationship(t *testing.T) {
	presentation := DescribeRelationship(ResourceRelationship{
		SourceID:   "node-1",
		TargetID:   "vm-1",
		Type:       RelRunsOn,
		Confidence: 0.85,
		Active:     false,
		Discoverer: "docker_adapter",
		Metadata:   map[string]any{"region": "lab"},
	})

	if presentation.TypeLabel != "Runs on" {
		t.Fatalf("unexpected type label: %q", presentation.TypeLabel)
	}
	if presentation.Direction != "node-1 → vm-1" {
		t.Fatalf("unexpected direction: %q", presentation.Direction)
	}
	if presentation.Provenance != "docker_adapter" {
		t.Fatalf("unexpected provenance: %q", presentation.Provenance)
	}
	if presentation.StateLabel != "Historical" {
		t.Fatalf("unexpected state label: %q", presentation.StateLabel)
	}
	if presentation.Confidence != "85%" {
		t.Fatalf("unexpected confidence: %q", presentation.Confidence)
	}
	if !presentation.HasMetadata {
		t.Fatalf("expected metadata flag to be set")
	}
}

func TestResourceRelationshipSummary(t *testing.T) {
	summary := resourceRelationshipSummary([]ResourceRelationship{
		{
			SourceID: "node-1",
			TargetID: "vm-1",
			Type:     RelRunsOn,
		},
		{
			SourceID: "node-1",
			TargetID: "storage-1",
			Type:     RelDependsOn,
		},
	})

	if want := "node-1->storage-1[Depends on], node-1->vm-1[Runs on]"; summary != want {
		t.Fatalf("resourceRelationshipSummary() = %q, want %q", summary, want)
	}
}

func TestFormatResourceRelationshipContext(t *testing.T) {
	resource := &Resource{
		Relationships: []ResourceRelationship{
			{
				SourceID:   "node-1",
				TargetID:   "vm-1",
				Type:       RelRunsOn,
				Confidence: 0.85,
				Active:     true,
				Discoverer: "docker_adapter",
				Metadata:   map[string]any{"region": "lab"},
			},
			{
				SourceID:   "node-1",
				TargetID:   "storage-1",
				Type:       RelDependsOn,
				Confidence: 0.5,
				Active:     false,
			},
		},
	}

	ctx := FormatResourceRelationshipContext(resource, 1)
	if ctx == "" {
		t.Fatal("expected relationship context")
	}
	if want := "### Resource Relationships"; !contains(ctx, want) {
		t.Fatalf("expected %q in relationship context, got %q", want, ctx)
	}
	if !contains(ctx, "Runs on") {
		t.Fatalf("expected canonical relationship label, got %q", ctx)
	}
	if !contains(ctx, "discoverer docker_adapter") {
		t.Fatalf("expected provenance in relationship context, got %q", ctx)
	}
	if contains(ctx, "Depends on") {
		t.Fatalf("expected relationship limit to truncate entries, got %q", ctx)
	}
}

func TestResourceRelationshipsWithCanonicalParent(t *testing.T) {
	parentID := "k8s-cluster-1"
	now := time.Date(2026, 3, 18, 17, 0, 0, 0, time.UTC)
	resource := Resource{
		ID:       "agent-1",
		Type:     ResourceTypeAgent,
		ParentID: &parentID,
		LastSeen: now,
	}

	relationships := ResourceRelationshipsWithCanonicalParent(resource)
	if len(relationships) != 1 {
		t.Fatalf("expected one canonical parent relationship, got %#v", relationships)
	}
	relationship := relationships[0]
	if relationship.SourceID != "agent-1" || relationship.TargetID != parentID {
		t.Fatalf("unexpected relationship endpoints: %#v", relationship)
	}
	if relationship.Type != RelOwnedBy {
		t.Fatalf("relationship type = %q, want %q", relationship.Type, RelOwnedBy)
	}
	if relationship.Discoverer != parentRelationshipDiscoverer {
		t.Fatalf("discoverer = %q, want %q", relationship.Discoverer, parentRelationshipDiscoverer)
	}
	if relationship.Metadata["source"] != "parentId" {
		t.Fatalf("expected parentId provenance metadata, got %#v", relationship.Metadata)
	}

	relationships = ResourceRelationshipsWithCanonicalParent(Resource{
		ID:       "agent-1",
		Type:     ResourceTypeAgent,
		ParentID: &parentID,
		Relationships: []ResourceRelationship{
			{
				SourceID: "agent-1",
				TargetID: parentID,
				Type:     RelOwnedBy,
			},
		},
	})
	if len(relationships) != 1 {
		t.Fatalf("expected existing canonical parent relationship to be reused, got %#v", relationships)
	}
}

func TestNormalizeResourceRelationshipAddsStableAuditIdentity(t *testing.T) {
	relationship := ResourceRelationship{
		SourceID:   " network-endpoint:check-1 ",
		TargetID:   " app-container:web ",
		Type:       RelChecks,
		Confidence: 1,
		Discoverer: " availability_attachment ",
		EvidenceID: " evidence-1 ",
	}
	normalizeResourceRelationship(&relationship)
	firstID := relationship.ID

	if firstID == "" {
		t.Fatal("relationship ID was not derived")
	}
	if relationship.SourceID != "network-endpoint:check-1" ||
		relationship.TargetID != "app-container:web" {
		t.Fatalf("canonical endpoints = %q -> %q", relationship.SourceID, relationship.TargetID)
	}
	if relationship.EvidenceID != "evidence-1" {
		t.Fatalf("evidence ID = %q", relationship.EvidenceID)
	}

	relationship.ID = ""
	normalizeResourceRelationship(&relationship)
	if relationship.ID != firstID {
		t.Fatalf("relationship ID changed: got %q want %q", relationship.ID, firstID)
	}
}

func TestOperationalTrustRelationshipVocabularyIsTyped(t *testing.T) {
	for _, relationshipType := range []RelationshipType{
		RelRunsOn,
		RelHostedBy,
		RelDependsOn,
		RelStoresOn,
		RelProtectedBy,
		RelChecks,
		RelMemberOf,
		RelExposedBy,
	} {
		if relationshipType == "" {
			t.Fatal("relationship type must not be empty")
		}
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
