package vmware

import (
	"errors"
	"strings"
)

func classifyInventoryEnrichmentIssue(stage, entityType, entityID string, err error) (*InventoryEnrichmentIssue, bool) {
	if err == nil {
		return nil, false
	}
	var connectionErr *ConnectionError
	if !errors.As(err, &connectionErr) || connectionErr == nil {
		return nil, false
	}
	if !isNonFatalInventoryEnrichmentCategory(connectionErr.Category) {
		return nil, false
	}
	return &InventoryEnrichmentIssue{
		Stage:      strings.TrimSpace(stage),
		EntityType: strings.TrimSpace(entityType),
		EntityID:   strings.TrimSpace(entityID),
		Category:   strings.TrimSpace(connectionErr.Category),
		Message:    strings.TrimSpace(connectionErr.Message),
	}, true
}

func isNonFatalInventoryEnrichmentCategory(category string) bool {
	switch strings.ToLower(strings.TrimSpace(category)) {
	case "permission", "not_found", "unavailable", "endpoint":
		return true
	default:
		return false
	}
}
