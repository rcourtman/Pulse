package ai

import (
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/cost"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// CostPersistenceAdapter bridges ConfigPersistence to cost.Persistence.
type CostPersistenceAdapter struct {
	config *config.ConfigPersistence
}

// NewCostPersistenceAdapter creates a new adapter.
func NewCostPersistenceAdapter(cfg *config.ConfigPersistence) *CostPersistenceAdapter {
	return &CostPersistenceAdapter{config: cfg}
}

// SaveUsageHistory saves usage events to disk via ConfigPersistence.
func (a *CostPersistenceAdapter) SaveUsageHistory(events []cost.UsageEvent) error {
	records := make([]config.AIUsageEventRecord, len(events))
	for i, e := range events {
		records[i] = config.AIUsageEventRecord{
			Timestamp:     e.Timestamp,
			Provider:      e.Provider,
			RequestModel:  e.RequestModel,
			ResponseModel: e.ResponseModel,
			UseCase:       e.UseCase,
			InputTokens:   e.InputTokens,
			OutputTokens:  e.OutputTokens,
			TargetType:    e.TargetType,
			TargetID:      e.TargetID,
			FindingID:     e.FindingID,
		}
	}
	return a.config.SaveAIUsageHistory(records)
}

// LoadUsageHistory loads usage events from disk via ConfigPersistence.
func (a *CostPersistenceAdapter) LoadUsageHistory() ([]cost.UsageEvent, error) {
	data, err := a.config.LoadAIUsageHistory()
	if err != nil {
		return nil, err
	}

	events := make([]cost.UsageEvent, len(data.Events))
	for i, r := range data.Events {
		events[i] = cost.UsageEvent{
			Timestamp:     r.Timestamp,
			Provider:      r.Provider,
			RequestModel:  r.RequestModel,
			ResponseModel: r.ResponseModel,
			UseCase:       r.UseCase,
			InputTokens:   r.InputTokens,
			OutputTokens:  r.OutputTokens,
			TargetType:    r.TargetType,
			TargetID:      r.TargetID,
			FindingID:     r.FindingID,
		}
	}
	return events, nil
}
