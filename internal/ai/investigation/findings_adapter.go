package investigation

import (
	"time"
)

// AIFinding represents the interface for an AI finding from the ai package
// This avoids importing the ai package directly
type AIFinding interface {
	GetID() string
	GetSeverity() string
	GetCategory() string
	GetResourceID() string
	GetResourceName() string
	GetResourceType() string
	GetTitle() string
	GetDescription() string
	GetRecommendation() string
	GetEvidence() string
	GetInvestigationSessionID() string
	GetInvestigationStatus() string
	GetInvestigationOutcome() string
	GetLastInvestigatedAt() *time.Time
	GetInvestigationAttempts() int
	SetInvestigationSessionID(string)
	SetInvestigationStatus(string)
	SetInvestigationOutcome(string)
	SetLastInvestigatedAt(*time.Time)
	SetInvestigationAttempts(int)
}

// AIFindingsStore represents the interface for the AI findings store
type AIFindingsStore interface {
	Get(id string) AIFinding
	UpdateInvestigation(id, sessionID, status, outcome string, lastInvestigatedAt *time.Time, attempts int) bool
}

// FindingsStoreAdapter adapts an AIFindingsStore to the FindingsStore interface
type FindingsStoreAdapter struct {
	store AIFindingsStore
}

// NewFindingsStoreAdapter creates a new findings store adapter
func NewFindingsStoreAdapter(store AIFindingsStore) *FindingsStoreAdapter {
	return &FindingsStoreAdapter{store: store}
}

// Get retrieves a finding by ID
func (a *FindingsStoreAdapter) Get(id string) *Finding {
	if a.store == nil {
		return nil
	}

	f := a.store.Get(id)
	if f == nil {
		return nil
	}

	return &Finding{
		ID:                     f.GetID(),
		Severity:               f.GetSeverity(),
		Category:               f.GetCategory(),
		ResourceID:             f.GetResourceID(),
		ResourceName:           f.GetResourceName(),
		ResourceType:           f.GetResourceType(),
		Title:                  f.GetTitle(),
		Description:            f.GetDescription(),
		Recommendation:         f.GetRecommendation(),
		Evidence:               f.GetEvidence(),
		InvestigationSessionID: f.GetInvestigationSessionID(),
		InvestigationStatus:    f.GetInvestigationStatus(),
		InvestigationOutcome:   f.GetInvestigationOutcome(),
		LastInvestigatedAt:     f.GetLastInvestigatedAt(),
		InvestigationAttempts:  f.GetInvestigationAttempts(),
	}
}

// Update updates a finding with investigation data
func (a *FindingsStoreAdapter) Update(f *Finding) bool {
	if a.store == nil || f == nil {
		return false
	}

	// Use the store's UpdateInvestigation method which modifies the actual finding
	// and triggers persistence
	return a.store.UpdateInvestigation(
		f.ID,
		f.InvestigationSessionID,
		f.InvestigationStatus,
		f.InvestigationOutcome,
		f.LastInvestigatedAt,
		f.InvestigationAttempts,
	)
}
