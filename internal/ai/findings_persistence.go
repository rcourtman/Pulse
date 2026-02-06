// Package ai provides AI-powered infrastructure monitoring and investigation.
package ai

import (
	"time"

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
		lifecycle := make([]struct {
			At       time.Time         `json:"at"`
			Type     string            `json:"type"`
			Message  string            `json:"message,omitempty"`
			From     string            `json:"from,omitempty"`
			To       string            `json:"to,omitempty"`
			Metadata map[string]string `json:"metadata,omitempty"`
		}, 0, len(f.Lifecycle))
		for _, e := range f.Lifecycle {
			lifecycle = append(lifecycle, struct {
				At       time.Time         `json:"at"`
				Type     string            `json:"type"`
				Message  string            `json:"message,omitempty"`
				From     string            `json:"from,omitempty"`
				To       string            `json:"to,omitempty"`
				Metadata map[string]string `json:"metadata,omitempty"`
			}{
				At:       e.At,
				Type:     e.Type,
				Message:  e.Message,
				From:     e.From,
				To:       e.To,
				Metadata: e.Metadata,
			})
		}
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
			ResolveReason:   f.ResolveReason,
			AcknowledgedAt:  f.AcknowledgedAt,
			SnoozedUntil:    f.SnoozedUntil,
			AlertID:         f.AlertID,
			DismissedReason: f.DismissedReason,
			UserNote:        f.UserNote,
			TimesRaised:     f.TimesRaised,
			Suppressed:      f.Suppressed,
			// Investigation fields
			InvestigationSessionID: f.InvestigationSessionID,
			InvestigationStatus:    f.InvestigationStatus,
			InvestigationOutcome:   f.InvestigationOutcome,
			LastInvestigatedAt:     f.LastInvestigatedAt,
			InvestigationAttempts:  f.InvestigationAttempts,
			LoopState:              f.LoopState,
			Lifecycle:              lifecycle,
			RegressionCount:        f.RegressionCount,
			LastRegressionAt:       f.LastRegressionAt,
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
		lifecycle := make([]FindingLifecycleEvent, 0, len(r.Lifecycle))
		for _, e := range r.Lifecycle {
			lifecycle = append(lifecycle, FindingLifecycleEvent{
				At:       e.At,
				Type:     e.Type,
				Message:  e.Message,
				From:     e.From,
				To:       e.To,
				Metadata: e.Metadata,
			})
		}
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
			ResolveReason:   r.ResolveReason,
			AcknowledgedAt:  r.AcknowledgedAt,
			SnoozedUntil:    r.SnoozedUntil,
			AlertID:         r.AlertID,
			DismissedReason: r.DismissedReason,
			UserNote:        r.UserNote,
			TimesRaised:     r.TimesRaised,
			Suppressed:      r.Suppressed,
			// Investigation fields
			InvestigationSessionID: r.InvestigationSessionID,
			InvestigationStatus:    r.InvestigationStatus,
			InvestigationOutcome:   r.InvestigationOutcome,
			LastInvestigatedAt:     r.LastInvestigatedAt,
			InvestigationAttempts:  r.InvestigationAttempts,
			LoopState:              r.LoopState,
			Lifecycle:              lifecycle,
			RegressionCount:        r.RegressionCount,
			LastRegressionAt:       r.LastRegressionAt,
		}
	}
	return findings, nil
}
