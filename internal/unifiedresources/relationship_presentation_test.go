package unifiedresources

import "testing"

func TestRelationshipTypeLabel(t *testing.T) {
	cases := map[RelationshipType]string{
		RelRunsOn:                       "Runs on",
		RelDependsOn:                    "Depends on",
		RelMountedTo:                    "Mounted to",
		RelExposedBy:                    "Exposed by",
		RelOwnedBy:                      "Owned by",
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
