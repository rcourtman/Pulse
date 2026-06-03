package unifiedresources

import "time"

const parentRelationshipDiscoverer = "resource_registry"

// RelationshipType defines the connection semantics between two resources.
type RelationshipType string

const (
	RelRunsOn     RelationshipType = "runs_on"     // e.g., container runs_on machine
	RelDependsOn  RelationshipType = "depends_on"  // e.g., app depends_on database
	RelMountedTo  RelationshipType = "mounted_to"  // e.g., volume mounted_to container
	RelExposedBy  RelationshipType = "exposed_by"  // e.g., container exposed_by ingress
	RelOwnedBy    RelationshipType = "owned_by"    // e.g., pod owned_by deployment
	RelAttachedTo RelationshipType = "attached_to" // e.g., container attached_to network
)

// ResourceRelationship represents a typed relationship edge between two unified resources.
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

// ResourceRelationshipsWithCanonicalParent returns explicit resource
// relationships plus a canonical relationship derived from ParentID when the
// resource has a parent but no equivalent typed edge.
func ResourceRelationshipsWithCanonicalParent(resource Resource) []ResourceRelationship {
	relationships := append([]ResourceRelationship(nil), resource.Relationships...)

	sourceID := CanonicalResourceID(resource.ID)
	parentID := ""
	if resource.ParentID != nil {
		parentID = CanonicalResourceID(*resource.ParentID)
	}
	if sourceID == "" || parentID == "" || sourceID == parentID {
		return relationships
	}

	relationshipType := parentRelationshipType(resource.Type)
	if relationshipType == "" {
		return relationships
	}

	for _, relationship := range relationships {
		if CanonicalResourceID(relationship.SourceID) == sourceID &&
			CanonicalResourceID(relationship.TargetID) == parentID &&
			relationship.Type == relationshipType {
			return relationships
		}
	}

	observedAt := resource.UpdatedAt
	if observedAt.IsZero() {
		observedAt = resource.LastSeen
	}

	return append(relationships, ResourceRelationship{
		SourceID:   sourceID,
		TargetID:   parentID,
		Type:       relationshipType,
		Confidence: 1,
		Active:     true,
		Discoverer: parentRelationshipDiscoverer,
		ObservedAt: observedAt,
		LastSeenAt: observedAt,
		Metadata: map[string]any{
			"source": "parentId",
		},
	})
}

func parentRelationshipType(resourceType ResourceType) RelationshipType {
	switch CanonicalResourceType(resourceType) {
	case ResourceTypeVM,
		ResourceTypeSystemContainer,
		ResourceTypeAppContainer,
		ResourceTypeDockerService,
		ResourceTypePBS,
		ResourceTypePMG,
		ResourceTypeCeph:
		return RelRunsOn
	case ResourceTypeStorage, ResourceTypePhysicalDisk, ResourceTypeNetworkShare:
		return RelMountedTo
	case ResourceTypeAgent, ResourceTypeK8sNode, ResourceTypePod, ResourceTypeK8sDeployment:
		return RelOwnedBy
	default:
		return RelOwnedBy
	}
}
