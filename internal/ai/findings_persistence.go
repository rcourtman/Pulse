// Package ai provides AI-powered infrastructure monitoring and investigation.
package ai

import (
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func findingsToRecords(findings map[string]*Finding) map[string]*config.AIFindingRecord {
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
	return records
}

func recordsToFindings(records map[string]*config.AIFindingRecord) map[string]*Finding {
	// Convert from AIFindingRecord to Finding
	findings := make(map[string]*Finding, len(records))
	for id, r := range records {
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
	return findings
}

func suppressionRulesToRecords(rules map[string]*SuppressionRule) map[string]*config.AISuppressionRuleRecord {
	records := make(map[string]*config.AISuppressionRuleRecord, len(rules))
	for id, r := range rules {
		if r == nil {
			continue
		}
		records[id] = &config.AISuppressionRuleRecord{
			ID:              r.ID,
			ResourceID:      r.ResourceID,
			ResourceName:    r.ResourceName,
			Category:        string(r.Category),
			Description:     r.Description,
			DismissedReason: r.DismissedReason,
			CreatedAt:       r.CreatedAt,
			CreatedFrom:     r.CreatedFrom,
			FindingID:       r.FindingID,
		}
	}
	return records
}

func recordsToSuppressionRules(records map[string]*config.AISuppressionRuleRecord) map[string]*SuppressionRule {
	rules := make(map[string]*SuppressionRule, len(records))
	for id, r := range records {
		if r == nil {
			continue
		}
		rules[id] = &SuppressionRule{
			ID:              r.ID,
			ResourceID:      r.ResourceID,
			ResourceName:    r.ResourceName,
			Category:        FindingCategory(r.Category),
			Description:     r.Description,
			DismissedReason: r.DismissedReason,
			CreatedAt:       r.CreatedAt,
			CreatedFrom:     r.CreatedFrom,
			FindingID:       r.FindingID,
		}
	}
	return rules
}

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
	records := findingsToRecords(findings)
	// Preserve existing suppression rules if present.
	return a.config.SaveAIFindingsWithSuppression(records, nil)
}

// LoadFindings loads findings from disk via ConfigPersistence
func (a *FindingsPersistenceAdapter) LoadFindings() (map[string]*Finding, error) {
	data, err := a.config.LoadAIFindings()
	if err != nil {
		return nil, err
	}
	return recordsToFindings(data.Findings), nil
}

// SaveFindingsAndSuppression saves findings and explicit suppression rules to disk.
func (a *FindingsPersistenceAdapter) SaveFindingsAndSuppression(findings map[string]*Finding, suppressionRules map[string]*SuppressionRule) error {
	records := findingsToRecords(findings)
	ruleRecords := suppressionRulesToRecords(suppressionRules)
	return a.config.SaveAIFindingsWithSuppression(records, ruleRecords)
}

// LoadFindingsAndSuppression loads findings and explicit suppression rules from disk.
func (a *FindingsPersistenceAdapter) LoadFindingsAndSuppression() (map[string]*Finding, map[string]*SuppressionRule, error) {
	data, err := a.config.LoadAIFindings()
	if err != nil {
		return nil, nil, err
	}
	return recordsToFindings(data.Findings), recordsToSuppressionRules(data.SuppressionRules), nil
}
