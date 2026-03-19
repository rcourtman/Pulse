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

func TestFormatResourceGraphContext(t *testing.T) {
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

	ctx := FormatResourceGraphContext(resource, 1)
	if ctx == "" {
		t.Fatal("expected graph context")
	}
	if want := "### Resource Graph"; !contains(ctx, want) {
		t.Fatalf("expected %q in graph context, got %q", want, ctx)
	}
	if !contains(ctx, "Runs on") {
		t.Fatalf("expected canonical relationship label, got %q", ctx)
	}
	if !contains(ctx, "discoverer docker_adapter") {
		t.Fatalf("expected provenance in graph context, got %q", ctx)
	}
	if contains(ctx, "Depends on") {
		t.Fatalf("expected graph limit to truncate entries, got %q", ctx)
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
