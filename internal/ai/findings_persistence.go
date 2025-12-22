// Package ai provides AI-powered infrastructure monitoring and investigation.
package ai

import (
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// FindingsPersistenceAdapter bridges ConfigPersistence to FindingsPersistence interface
type FindingsPersistenceAdapter struct {
	config *config.ConfigPersistence
}

// NewFindingsPersistenceAdapter creates a new adapter
func NewFindingsPersistenceAdapter(cfg *config.ConfigPersistence) *FindingsPersistenceAdapter {
	return &FindingsPersistenceAdapter{config: cfg}
}

// SaveFindings saves findings to disk via ConfigPersistence
func (a *FindingsPersistenceAdapter) SaveFindings(findings map[string]*Finding) error {
	// Convert from Finding to AIFindingRecord
	records := make(map[string]*config.AIFindingRecord, len(findings))
	for id, f := range findings {
		records[id] = &config.AIFindingRecord{
			ID:              f.ID,
			Key:             f.Key,
			Severity:        string(f.Severity),
			Category:        string(f.Category),
			ResourceID:      f.ResourceID,
			ResourceName:    f.ResourceName,
			ResourceType:    f.ResourceType,
			Node:            f.Node,
			Title:           f.Title,
			Description:     f.Description,
			Recommendation:  f.Recommendation,
			Evidence:        f.Evidence,
			Source:          f.Source,
			DetectedAt:      f.DetectedAt,
			LastSeenAt:      f.LastSeenAt,
			ResolvedAt:      f.ResolvedAt,
			AutoResolved:    f.AutoResolved,
			AcknowledgedAt:  f.AcknowledgedAt,
			SnoozedUntil:    f.SnoozedUntil,
			AlertID:         f.AlertID,
			DismissedReason: f.DismissedReason,
			UserNote:        f.UserNote,
			TimesRaised:     f.TimesRaised,
			Suppressed:      f.Suppressed,
		}
	}
	return a.config.SaveAIFindings(records)
}

// LoadFindings loads findings from disk via ConfigPersistence
func (a *FindingsPersistenceAdapter) LoadFindings() (map[string]*Finding, error) {
	data, err := a.config.LoadAIFindings()
	if err != nil {
		return nil, err
	}

	// Convert from AIFindingRecord to Finding
	findings := make(map[string]*Finding, len(data.Findings))
	for id, r := range data.Findings {
		findings[id] = &Finding{
			ID:              r.ID,
			Key:             r.Key,
			Severity:        FindingSeverity(r.Severity),
			Category:        FindingCategory(r.Category),
			ResourceID:      r.ResourceID,
			ResourceName:    r.ResourceName,
			ResourceType:    r.ResourceType,
			Node:            r.Node,
			Title:           r.Title,
			Description:     r.Description,
			Recommendation:  r.Recommendation,
			Evidence:        r.Evidence,
			Source:          r.Source,
			DetectedAt:      r.DetectedAt,
			LastSeenAt:      r.LastSeenAt,
			ResolvedAt:      r.ResolvedAt,
			AutoResolved:    r.AutoResolved,
			AcknowledgedAt:  r.AcknowledgedAt,
			SnoozedUntil:    r.SnoozedUntil,
			AlertID:         r.AlertID,
			DismissedReason: r.DismissedReason,
			UserNote:        r.UserNote,
			TimesRaised:     r.TimesRaised,
			Suppressed:      r.Suppressed,
		}
	}
	return findings, nil
}

