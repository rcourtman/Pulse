package cost

import (
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// UsageEvent represents a single AI provider call for cost/token tracking.
// It intentionally excludes prompt/response content for privacy.
type UsageEvent struct {
	Timestamp     time.Time `json:"timestamp"`
	Provider      string    `json:"provider"`
	RequestModel  string    `json:"request_model"`
	ResponseModel string    `json:"response_model,omitempty"`
	UseCase       string    `json:"use_case,omitempty"` // "chat" or "patrol"
	InputTokens   int       `json:"input_tokens,omitempty"`
	OutputTokens  int       `json:"output_tokens,omitempty"`
	TargetType    string    `json:"target_type,omitempty"`
	TargetID      string    `json:"target_id,omitempty"`
	FindingID     string    `json:"finding_id,omitempty"`
}

// Persistence defines the storage contract for usage history.
type Persistence interface {
	SaveUsageHistory(events []UsageEvent) error
	LoadUsageHistory() ([]UsageEvent, error)
}

// DefaultMaxDays is the default retention window for raw usage events.
const DefaultMaxDays = 365

// Store provides thread-safe usage tracking with optional persistence.
type Store struct {
	mu          sync.RWMutex
	events      []UsageEvent
	maxDays     int
	persistence Persistence

	// Debounced persistence to avoid frequent disk writes.
	saveTimer    *time.Timer
	savePending  bool
	saveDebounce time.Duration
}

// NewStore creates a new usage store.
func NewStore(maxDays int) *Store {
	if maxDays <= 0 {
		maxDays = DefaultMaxDays
	}
	return &Store{
		events:       make([]UsageEvent, 0),
		maxDays:      maxDays,
		saveDebounce: 5 * time.Second,
	}
}

// SetPersistence sets persistence and loads any existing history.
func (s *Store) SetPersistence(p Persistence) error {
	s.mu.Lock()
	s.persistence = p
	s.mu.Unlock()

	if p == nil {
		return nil
	}

	events, err := p.LoadUsageHistory()
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.events = events
	s.trimLocked(time.Now())
	s.mu.Unlock()
	return nil
}

// Record appends a usage event and schedules persistence.
func (s *Store) Record(event UsageEvent) {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	s.mu.Lock()
	s.events = append(s.events, event)
	s.trimLocked(time.Now())
	s.scheduleSaveLocked()
	s.mu.Unlock()
}

// Clear removes all retained usage events and persists the empty history.
func (s *Store) Clear() error {
	s.mu.Lock()
	s.events = s.events[:0]
	s.trimLocked(time.Now())
	s.scheduleSaveLocked()
	s.mu.Unlock()
	return s.Flush()
}

// ListEvents returns a copy of usage events within the requested time window, applying retention.
func (s *Store) ListEvents(days int) []UsageEvent {
	if days <= 0 {
		days = 30
	}
	now := time.Now()
	effectiveDays := days
	if s.maxDays > 0 && effectiveDays > s.maxDays {
		effectiveDays = s.maxDays
	}
	cutoff := now.AddDate(0, 0, -effectiveDays)

	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]UsageEvent, 0, len(s.events))
	for _, e := range s.events {
		if !e.Timestamp.Before(cutoff) {
			out = append(out, e)
		}
	}
	return out
}

// GetSummary returns a rollup of usage over the last N days.
func (s *Store) GetSummary(days int) Summary {
	if days <= 0 {
		days = 30
	}

	now := time.Now()
	retentionDays := s.maxDays
	effectiveDays := days
	truncated := false
	if retentionDays > 0 && effectiveDays > retentionDays {
		effectiveDays = retentionDays
		truncated = true
	}

	cutoff := now.AddDate(0, 0, -effectiveDays)

	s.mu.RLock()
	events := make([]UsageEvent, 0, len(s.events))
	for _, e := range s.events {
		if !e.Timestamp.Before(cutoff) {
			events = append(events, e)
		}
	}
	s.mu.RUnlock()

	type pmKey struct {
		provider string
		model    string
	}

	pmTotals := make(map[pmKey]*ProviderModelSummary)
	dailyTotals := make(map[string]*DailySummary)

	var totalInput, totalOutput int64

	for _, e := range events {
		provider, model := ResolveProviderAndModel(e.Provider, e.RequestModel, e.ResponseModel)

		k := pmKey{provider: provider, model: model}
		pm := pmTotals[k]
		if pm == nil {
			pm = &ProviderModelSummary{Provider: provider, Model: model}
			pmTotals[k] = pm
		}
		pm.InputTokens += int64(e.InputTokens)
		pm.OutputTokens += int64(e.OutputTokens)

		totalInput += int64(e.InputTokens)
		totalOutput += int64(e.OutputTokens)

		usd, known, _ := EstimateUSD(provider, model, int64(e.InputTokens), int64(e.OutputTokens))
		if known {
			pm.EstimatedUSD += usd
			pm.PricingKnown = true
		}

		date := e.Timestamp.Format("2006-01-02")
		ds := dailyTotals[date]
		if ds == nil {
			ds = &DailySummary{Date: date}
			dailyTotals[date] = ds
		}
		ds.InputTokens += int64(e.InputTokens)
		ds.OutputTokens += int64(e.OutputTokens)
		if known {
			ds.EstimatedUSD += usd
		}
	}

	providerModels := make([]ProviderModelSummary, 0, len(pmTotals))
	for _, pm := range pmTotals {
		pm.TotalTokens = pm.InputTokens + pm.OutputTokens
		providerModels = append(providerModels, *pm)
	}
	sort.Slice(providerModels, func(i, j int) bool {
		if providerModels[i].Provider == providerModels[j].Provider {
			return providerModels[i].Model < providerModels[j].Model
		}
		return providerModels[i].Provider < providerModels[j].Provider
	})

	daily := make([]DailySummary, 0, len(dailyTotals))
	for _, ds := range dailyTotals {
		ds.TotalTokens = ds.InputTokens + ds.OutputTokens
		daily = append(daily, *ds)
	}
	sort.Slice(daily, func(i, j int) bool {
		return daily[i].Date < daily[j].Date
	})

	totals := ProviderModelSummary{
		Provider:     "all",
		InputTokens:  totalInput,
		OutputTokens: totalOutput,
		TotalTokens:  totalInput + totalOutput,
	}

	for _, pm := range providerModels {
		if pm.PricingKnown {
			totals.EstimatedUSD += pm.EstimatedUSD
			totals.PricingKnown = true
		}
	}

	return Summary{
		Days:           days,
		RetentionDays:  retentionDays,
		EffectiveDays:  effectiveDays,
		Truncated:      truncated,
		PricingAsOf:    PricingAsOf(),
		ProviderModels: providerModels,
		UseCases:       summarizeUseCases(events),
		Targets:        summarizeTargets(events),
		DailyTotals:    daily,
		Totals:         totals,
	}
}

// Flush immediately writes any pending changes to persistence.
func (s *Store) Flush() error {
	s.mu.Lock()
	if s.saveTimer != nil {
		s.saveTimer.Stop()
	}
	s.savePending = false
	events := make([]UsageEvent, len(s.events))
	copy(events, s.events)
	p := s.persistence
	s.mu.Unlock()

	if p != nil {
		return p.SaveUsageHistory(events)
	}
	return nil
}

func (s *Store) trimLocked(now time.Time) {
	if s.maxDays <= 0 {
		return
	}
	cutoff := now.AddDate(0, 0, -s.maxDays)
	filtered := s.events[:0]
	for _, e := range s.events {
		if !e.Timestamp.Before(cutoff) {
			filtered = append(filtered, e)
		}
	}
	s.events = filtered
}

func (s *Store) scheduleSaveLocked() {
	if s.persistence == nil {
		return
	}

	if s.saveTimer != nil {
		s.saveTimer.Stop()
	}

	s.savePending = true
	s.saveTimer = time.AfterFunc(s.saveDebounce, func() {
		s.mu.Lock()
		if !s.savePending {
			s.mu.Unlock()
			return
		}
		s.savePending = false
		events := make([]UsageEvent, len(s.events))
		copy(events, s.events)
		p := s.persistence
		s.mu.Unlock()

		if p != nil {
			if err := p.SaveUsageHistory(events); err != nil {
				log.Error().Err(err).Msg("Failed to save AI usage history")
			}
		}
	})
}

// ProviderModelSummary is a rollup for a provider/model pair.
type ProviderModelSummary struct {
	Provider     string  `json:"provider"`
	Model        string  `json:"model"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	TotalTokens  int64   `json:"total_tokens"`
	EstimatedUSD float64 `json:"estimated_usd,omitempty"`
	PricingKnown bool    `json:"pricing_known"`
}

// DailySummary is a rollup for a single day across all providers.
type DailySummary struct {
	Date         string  `json:"date"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	TotalTokens  int64   `json:"total_tokens"`
	EstimatedUSD float64 `json:"estimated_usd,omitempty"`
}

// UseCaseSummary is a rollup for a use-case (e.g. "chat", "patrol").
type UseCaseSummary struct {
	UseCase      string  `json:"use_case"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	TotalTokens  int64   `json:"total_tokens"`
	EstimatedUSD float64 `json:"estimated_usd,omitempty"`
	PricingKnown bool    `json:"pricing_known"`
}

// TargetSummary is a rollup for a Pulse target (e.g. vm/container/node).
type TargetSummary struct {
	TargetType   string  `json:"target_type"`
	TargetID     string  `json:"target_id"`
	Calls        int64   `json:"calls"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	TotalTokens  int64   `json:"total_tokens"`
	EstimatedUSD float64 `json:"estimated_usd,omitempty"`
	PricingKnown bool    `json:"pricing_known"`
}

// Summary is returned by the cost summary API.
type Summary struct {
	Days          int  `json:"days"`
	RetentionDays int  `json:"retention_days"`
	EffectiveDays int  `json:"effective_days"`
	Truncated     bool `json:"truncated"`

	PricingAsOf string `json:"pricing_as_of,omitempty"`

	ProviderModels []ProviderModelSummary `json:"provider_models"`
	UseCases       []UseCaseSummary       `json:"use_cases"`
	Targets        []TargetSummary        `json:"targets"`
	DailyTotals    []DailySummary         `json:"daily_totals"`
	Totals         ProviderModelSummary   `json:"totals"`
}

func summarizeUseCases(events []UsageEvent) []UseCaseSummary {
	type totals struct {
		input  int64
		output int64
		usd    float64
		known  bool
	}

	perUseCase := make(map[string]*totals)
	for _, e := range events {
		useCase := strings.TrimSpace(strings.ToLower(e.UseCase))
		if useCase == "" {
			useCase = "unknown"
		}
		t := perUseCase[useCase]
		if t == nil {
			t = &totals{}
			perUseCase[useCase] = t
		}

		t.input += int64(e.InputTokens)
		t.output += int64(e.OutputTokens)

		provider, model := ResolveProviderAndModel(e.Provider, e.RequestModel, e.ResponseModel)

		usd, known, _ := EstimateUSD(provider, model, int64(e.InputTokens), int64(e.OutputTokens))
		if known {
			t.usd += usd
			t.known = true
		}
	}

	out := make([]UseCaseSummary, 0, len(perUseCase))
	for useCase, t := range perUseCase {
		out = append(out, UseCaseSummary{
			UseCase:      useCase,
			InputTokens:  t.input,
			OutputTokens: t.output,
			TotalTokens:  t.input + t.output,
			EstimatedUSD: t.usd,
			PricingKnown: t.known,
		})
	}

	order := map[string]int{
		"chat":    0,
		"patrol":  1,
		"unknown": 2,
	}
	sort.Slice(out, func(i, j int) bool {
		oi, okI := order[out[i].UseCase]
		oj, okJ := order[out[j].UseCase]
		if okI && okJ && oi != oj {
			return oi < oj
		}
		if okI != okJ {
			return okI
		}
		return out[i].UseCase < out[j].UseCase
	})
	return out
}

func summarizeTargets(events []UsageEvent) []TargetSummary {
	type key struct {
		targetType string
		targetID   string
	}
	type totals struct {
		calls  int64
		input  int64
		output int64
		usd    float64
		known  bool
	}

	perTarget := make(map[key]*totals)
	for _, e := range events {
		targetType := strings.TrimSpace(strings.ToLower(e.TargetType))
		targetID := strings.TrimSpace(e.TargetID)
		if targetType == "" || targetID == "" {
			continue
		}
		k := key{targetType: targetType, targetID: targetID}
		t := perTarget[k]
		if t == nil {
			t = &totals{}
			perTarget[k] = t
		}
		t.calls++
		t.input += int64(e.InputTokens)
		t.output += int64(e.OutputTokens)

		provider, model := ResolveProviderAndModel(e.Provider, e.RequestModel, e.ResponseModel)
		usd, known, _ := EstimateUSD(provider, model, int64(e.InputTokens), int64(e.OutputTokens))
		if known {
			t.usd += usd
			t.known = true
		}
	}

	out := make([]TargetSummary, 0, len(perTarget))
	for k, t := range perTarget {
		out = append(out, TargetSummary{
			TargetType:   k.targetType,
			TargetID:     k.targetID,
			Calls:        t.calls,
			InputTokens:  t.input,
			OutputTokens: t.output,
			TotalTokens:  t.input + t.output,
			EstimatedUSD: t.usd,
			PricingKnown: t.known,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].EstimatedUSD != out[j].EstimatedUSD {
			return out[i].EstimatedUSD > out[j].EstimatedUSD
		}
		if out[i].TotalTokens != out[j].TotalTokens {
			return out[i].TotalTokens > out[j].TotalTokens
		}
		if out[i].TargetType != out[j].TargetType {
			return out[i].TargetType < out[j].TargetType
		}
		return out[i].TargetID < out[j].TargetID
	})

	if len(out) > 20 {
		out = out[:20]
	}
	return out
}
