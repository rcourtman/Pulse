package ai

import (
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
)

// FindingsToolAdapter adapts FindingsStore to the native Assistant findings
// provider interface used by the shared Pulse Intelligence tool registry.
type FindingsToolAdapter struct {
	store *FindingsStore
}

// NewFindingsToolAdapter creates a new adapter for findings store.
func NewFindingsToolAdapter(store *FindingsStore) *FindingsToolAdapter {
	if store == nil {
		return nil
	}
	return &FindingsToolAdapter{store: store}
}

// GetActiveFindings implements tools.FindingsProvider
func (a *FindingsToolAdapter) GetActiveFindings() []tools.Finding {
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
func (a *FindingsToolAdapter) GetDismissedFindings() []tools.Finding {
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
