package unifiedresources

import "time"

// RelationshipType defines the connection semantics between two resources.
type RelationshipType string

const (
	RelRunsOn    RelationshipType = "runs_on"    // e.g., container runs_on machine
	RelDependsOn RelationshipType = "depends_on" // e.g., app depends_on database
	RelMountedTo RelationshipType = "mounted_to" // e.g., volume mounted_to container
	RelExposedBy RelationshipType = "exposed_by" // e.g., container exposed_by ingress
	RelOwnedBy   RelationshipType = "owned_by"   // e.g., pod owned_by deployment
)

// ResourceRelationship represents a typed graph edge between two unified resources.
type ResourceRelationship struct {
	SourceID   string           `json:"sourceId"`
	TargetID   string           `json:"targetId"`
	Type       RelationshipType `json:"type"`
	Confidence float64          `json:"confidence"` // 1.0 for defined relationships, lower for heuristics

	// Provenance and time boundaries
	Active     bool           `json:"active"`     // False means historical edge
	Discoverer string         `json:"discoverer"` // e.g. "docker_adapter" or "pulse_inference_engine"
	ObservedAt time.Time      `json:"observedAt"`
	LastSeenAt time.Time      `json:"lastSeenAt"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}
