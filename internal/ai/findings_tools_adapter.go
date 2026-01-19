package ai

import (
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
)

// FindingsMCPAdapter adapts FindingsStore to MCP FindingsProvider interface
type FindingsMCPAdapter struct {
	store *FindingsStore
}

// NewFindingsMCPAdapter creates a new adapter for findings store
func NewFindingsMCPAdapter(store *FindingsStore) *FindingsMCPAdapter {
	if store == nil {
		return nil
	}
	return &FindingsMCPAdapter{store: store}
}

// GetActiveFindings implements tools.FindingsProvider
func (a *FindingsMCPAdapter) GetActiveFindings() []tools.Finding {
	if a.store == nil {
		return nil
	}

	// Get all active findings (empty severity means all)
	internal := a.store.GetActive("")
	result := make([]tools.Finding, 0, len(internal))

	for _, f := range internal {
		result = append(result, tools.Finding{
			ID:             f.ID,
			Key:            f.Key,
			Severity:       string(f.Severity),
			Category:       string(f.Category),
			ResourceID:     f.ResourceID,
			ResourceName:   f.ResourceName,
			ResourceType:   f.ResourceType,
			Title:          f.Title,
			Description:    f.Description,
			Recommendation: f.Recommendation,
			Evidence:       f.Evidence,
			DetectedAt:     f.DetectedAt,
			LastSeenAt:     f.LastSeenAt,
		})
	}

	return result
}

// GetDismissedFindings implements tools.FindingsProvider
func (a *FindingsMCPAdapter) GetDismissedFindings() []tools.Finding {
	if a.store == nil {
		return nil
	}

	internal := a.store.GetDismissedFindings()
	result := make([]tools.Finding, 0, len(internal))

	for _, f := range internal {
		result = append(result, tools.Finding{
			ID:             f.ID,
			Key:            f.Key,
			Severity:       string(f.Severity),
			Category:       string(f.Category),
			ResourceID:     f.ResourceID,
			ResourceName:   f.ResourceName,
			ResourceType:   f.ResourceType,
			Title:          f.Title,
			Description:    f.Description,
			Recommendation: f.Recommendation,
			Evidence:       f.Evidence,
			DetectedAt:     f.DetectedAt,
			LastSeenAt:     f.LastSeenAt,
		})
	}

	return result
}
