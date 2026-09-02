package ai

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/cost"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// ErrCostBudgetExceeded is the sentinel behind every budget refusal so Patrol
// can tell "Pulse declined to spend" apart from "the provider failed". The
// two used to share one generic provider-error path, which is how a mispriced
// model tripped the circuit breaker and silently disabled Patrol (#1789).
var ErrCostBudgetExceeded = errors.New("cost budget exceeded")

// CostBudgetExceededError carries the numbers behind a budget refusal so the
// Patrol page can say what was spent against what limit.
type CostBudgetExceededError struct {
	SpentUSD  float64
	BudgetUSD float64
	Days      int
}

func (e *CostBudgetExceededError) Error() string {
	return fmt.Sprintf("Pulse Intelligence cost budget exceeded (%.2f/%.2f USD over %d days) - raise the 30-day budget, choose a cheaper model, or wait for the window to roll on",
		e.SpentUSD, e.BudgetUSD, e.Days)
}

func (e *CostBudgetExceededError) Is(target error) bool { return target == ErrCostBudgetExceeded }

// patrolBudgetBlockedReason is the operator-facing sentence shown on the
// Patrol page while runs are being skipped for budget.
func patrolBudgetBlockedReason(err error) string {
	var budgetErr *CostBudgetExceededError
	if errors.As(err, &budgetErr) && budgetErr.BudgetUSD > 0 {
		return fmt.Sprintf("Your 30-day AI cost budget is used up (about $%.2f spent of the $%.2f limit), so Patrol is skipping analysis. Raise the budget under Provider & Models, choose a cheaper model or a slower schedule, or wait for the 30-day window to roll on.",
			budgetErr.SpentUSD, budgetErr.BudgetUSD)
	}
	return "Your 30-day AI cost budget is used up, so Patrol is skipping analysis. Raise the budget under Provider & Models, choose a cheaper model or a slower schedule, or wait for the 30-day window to roll on."
}

// blockOnExhaustedBudget promotes a budget refusal from a log line into the
// Patrol runtime block state, so the Patrol page shows "Patrol paused" with
// the reason instead of a healthy schedule that quietly never runs.
func (p *PatrolService) blockOnExhaustedBudget(err error, failure patrolRuntimeFailure) {
	if p == nil || failure.Cause != PatrolFailureCauseBudgetExhausted {
		return
	}
	p.setBlockedReasonWithCause(patrolBudgetBlockedReason(err), PatrolFailureCauseBudgetExhausted)
}

const (
	// DefaultPatrolRunInputTokens and DefaultPatrolRunOutputTokens are the
	// per-run estimate used before an install has its own history. They are a
	// full Patrol run measured on a real Proxmox install and reported in
	// issue #1789 (104,528 input / 4,491 output tokens). Installs with many
	// resources send more; the projection switches to the install's own
	// median as soon as three priced full runs exist.
	DefaultPatrolRunInputTokens  int64 = 104_528
	DefaultPatrolRunOutputTokens int64 = 4_491

	// PatrolCostReferenceBudgetUSD is the reference 30-day budget the
	// schedule recommendation targets when the operator has not set one.
	// It is the figure support conversations and issue #1789 use, not a
	// product default: an unset budget means unlimited spend.
	PatrolCostReferenceBudgetUSD = 20.0

	// PatrolCostRecommendedBudgetShare is how much of the budget scheduled
	// Patrol runs may use before the schedule recommendation slows them
	// down. The other half is headroom for alert-triggered runs, Assistant
	// chat, and provider price drift.
	PatrolCostRecommendedBudgetShare = 0.5

	// PatrolCostDefaultIntervalMinutes mirrors the config default; the
	// recommendation never proposes a schedule more frequent than it.
	PatrolCostDefaultIntervalMinutes = 360

	patrolCostProjectionWindowDays     = 30
	patrolCostProjectionMinHistoryRuns = 3

	PatrolCostPerRunSourceHistory = "history"
	PatrolCostPerRunSourceDefault = "default"

	PatrolCostRecommendationKeep              = "keep"
	PatrolCostRecommendationFitsBudgetShare   = "fits_budget_share"
	PatrolCostRecommendationExceedsEvenDaily  = "exceeds_budget_share_even_daily"
	PatrolCostRecommendationNotBilledPerToken = "not_billed_per_token"
	PatrolCostRecommendationPricingUnknown    = "pricing_unknown"
)

// PatrolCostIntervalOptionsMinutes are the schedule presets the settings
// page offers; the projection prices each one so the operator can compare.
var PatrolCostIntervalOptionsMinutes = []int{60, 180, 360, 720, 1440}

// PatrolCostIntervalEstimate prices one schedule preset.
type PatrolCostIntervalEstimate struct {
	IntervalMinutes     int     `json:"interval_minutes"`
	ScheduledRunsPerDay float64 `json:"scheduled_runs_per_day"`
	Projected30dUSD     float64 `json:"projected_30d_usd"`
}

// PatrolCostProjection is the cost preview shown next to the Patrol model
// choice. Every figure is an estimate from Pulse's price table and a per-run
// token estimate; the assumptions travel with the numbers so the UI can show
// them.
type PatrolCostProjection struct {
	Provider   string `json:"provider"`
	Model      string `json:"model"`
	ModelRoute string `json:"model_route"`

	// BilledPerToken is true for providers that charge per token at a known
	// non-zero price. Local models and free routes are not billed per token.
	BilledPerToken   bool    `json:"billed_per_token"`
	PricingKnown     bool    `json:"pricing_known"`
	PricingAsOf      string  `json:"pricing_as_of,omitempty"`
	InputUSDPerMTok  float64 `json:"input_usd_per_mtok"`
	OutputUSDPerMTok float64 `json:"output_usd_per_mtok"`

	PerRunInputTokens  int64   `json:"per_run_input_tokens"`
	PerRunOutputTokens int64   `json:"per_run_output_tokens"`
	PerRunSource       string  `json:"per_run_source"`
	HistoryRunCount    int     `json:"history_run_count"`
	PerRunUSD          float64 `json:"per_run_usd"`

	IntervalMinutes          int                          `json:"interval_minutes"`
	ScheduledRunsPerDay      float64                      `json:"scheduled_runs_per_day"`
	TriggeredRunsPerDay      float64                      `json:"triggered_runs_per_day"`
	TriggeredPerRunUSD       float64                      `json:"triggered_per_run_usd"`
	ScheduledProjected30dUSD float64                      `json:"scheduled_projected_30d_usd"`
	Projected30dUSD          float64                      `json:"projected_30d_usd"`
	IntervalEstimates        []PatrolCostIntervalEstimate `json:"interval_estimates"`

	BudgetUSD30d      float64 `json:"budget_usd_30d"`
	Spend30dUSD       float64 `json:"spend_30d_usd"`
	Spend30dKnown     bool    `json:"spend_30d_known"`
	PatrolSpend30dUSD float64 `json:"patrol_spend_30d_usd"`
	BudgetReached     bool    `json:"budget_reached"`

	// RecommendedIntervalMinutes is the slowest-acceptable schedule the cost
	// model proposes for a per-token provider, never more frequent than the
	// default. Zero means "keep whatever is set".
	RecommendedIntervalMinutes int    `json:"recommended_interval_minutes"`
	RecommendationReason       string `json:"recommendation_reason"`
	// RecommendationTargetUSD is the 30-day spend the recommendation aimed
	// to stay under (the budget share of the configured or reference budget).
	RecommendationTargetUSD float64 `json:"recommendation_target_usd"`
}

// PatrolCostProjectionInput is everything the projection needs; it carries
// no live dependencies so it stays a pure function for tests.
type PatrolCostProjectionInput struct {
	Provider        string
	Model           string
	IntervalMinutes int
	BudgetUSD30d    float64
	Runs            []PatrolRunRecord
	Spend           cost.Summary
	Now             time.Time
}

func patrolRunIsFull(run PatrolRunRecord) bool {
	if len(run.ScopeResourceIDs) > 0 || len(run.ScopeResourceTypes) > 0 || len(run.EffectiveScopeResourceIDs) > 0 {
		return false
	}
	switch TriggerReason(strings.TrimSpace(run.TriggerReason)) {
	case "", TriggerReasonScheduled, TriggerReasonManual, TriggerReasonStartup, TriggerReasonConfigChanged:
		return true
	}
	return false
}

func medianInt64(values []int64) int64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]int64(nil), values...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	mid := len(sorted) / 2
	if len(sorted)%2 == 0 {
		return (sorted[mid-1] + sorted[mid]) / 2
	}
	return sorted[mid]
}

func roundUSD(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return math.Round(v*10000) / 10000
}

// ProjectPatrolCost estimates what Patrol will cost per 30 days on the given
// provider/model at the given schedule, using the install's own run history
// when it has enough and a documented default otherwise.
func ProjectPatrolCost(in PatrolCostProjectionInput) PatrolCostProjection {
	now := in.Now
	if now.IsZero() {
		now = time.Now()
	}
	provider := strings.ToLower(strings.TrimSpace(in.Provider))
	model := strings.TrimSpace(in.Model)
	out := PatrolCostProjection{
		Provider:        provider,
		Model:           model,
		IntervalMinutes: in.IntervalMinutes,
		BudgetUSD30d:    in.BudgetUSD30d,
	}
	if provider != "" && model != "" {
		out.ModelRoute = provider + ":" + model
	}
	if in.IntervalMinutes < 0 {
		out.IntervalMinutes = 0
	}

	// Per-run token estimate: the install's own median full run when it has
	// enough priced history, else the measured default.
	windowStart := now.AddDate(0, 0, -patrolCostProjectionWindowDays)
	oldest := now
	var fullInputs, fullOutputs, trigInputs, trigOutputs []int64
	triggeredCount := 0
	for _, run := range in.Runs {
		if run.InputTokens <= 0 || run.StartedAt.Before(windowStart) {
			continue
		}
		if run.StartedAt.Before(oldest) {
			oldest = run.StartedAt
		}
		if patrolRunIsFull(run) {
			fullInputs = append(fullInputs, int64(run.InputTokens))
			fullOutputs = append(fullOutputs, int64(run.OutputTokens))
			continue
		}
		triggeredCount++
		trigInputs = append(trigInputs, int64(run.InputTokens))
		trigOutputs = append(trigOutputs, int64(run.OutputTokens))
	}
	out.PerRunSource = PatrolCostPerRunSourceDefault
	out.PerRunInputTokens = DefaultPatrolRunInputTokens
	out.PerRunOutputTokens = DefaultPatrolRunOutputTokens
	if len(fullInputs) >= patrolCostProjectionMinHistoryRuns {
		out.PerRunSource = PatrolCostPerRunSourceHistory
		out.PerRunInputTokens = medianInt64(fullInputs)
		out.PerRunOutputTokens = medianInt64(fullOutputs)
		out.HistoryRunCount = len(fullInputs)
	}

	// Observed window for rates: at least one day, at most the projection
	// window, so a fresh install does not extrapolate a burst.
	observedDays := now.Sub(oldest).Hours() / 24
	if observedDays < 1 {
		observedDays = 1
	}
	if observedDays > patrolCostProjectionWindowDays {
		observedDays = patrolCostProjectionWindowDays
	}
	if triggeredCount > 0 {
		out.TriggeredRunsPerDay = float64(triggeredCount) / observedDays
	}

	if out.IntervalMinutes > 0 {
		out.ScheduledRunsPerDay = 1440 / float64(out.IntervalMinutes)
	}

	// Pricing.
	_, known, price := cost.EstimateUSD(provider, model, out.PerRunInputTokens, out.PerRunOutputTokens)
	out.PricingKnown = known
	if known {
		out.PricingAsOf = price.AsOf
		out.InputUSDPerMTok = price.InputUSDPerMTok
		out.OutputUSDPerMTok = price.OutputUSDPerMTok
		out.BilledPerToken = price.InputUSDPerMTok > 0 || price.OutputUSDPerMTok > 0
	}
	perRun := func(inputTokens, outputTokens int64) float64 {
		if !known {
			return 0
		}
		usd, _, _ := cost.EstimateUSD(provider, model, inputTokens, outputTokens)
		return usd
	}
	out.PerRunUSD = roundUSD(perRun(out.PerRunInputTokens, out.PerRunOutputTokens))
	if triggeredCount > 0 {
		out.TriggeredPerRunUSD = roundUSD(perRun(medianInt64(trigInputs), medianInt64(trigOutputs)))
	}
	scheduled30d := func(intervalMinutes int) float64 {
		if intervalMinutes <= 0 {
			return 0
		}
		return (1440 / float64(intervalMinutes)) * out.PerRunUSD * patrolCostProjectionWindowDays
	}
	out.ScheduledProjected30dUSD = roundUSD(scheduled30d(out.IntervalMinutes))
	out.Projected30dUSD = roundUSD(out.ScheduledProjected30dUSD + out.TriggeredRunsPerDay*out.TriggeredPerRunUSD*patrolCostProjectionWindowDays)
	for _, option := range PatrolCostIntervalOptionsMinutes {
		out.IntervalEstimates = append(out.IntervalEstimates, PatrolCostIntervalEstimate{
			IntervalMinutes:     option,
			ScheduledRunsPerDay: 1440 / float64(option),
			Projected30dUSD:     roundUSD(scheduled30d(option)),
		})
	}

	// Spend against budget, on the same basis enforceBudget uses.
	out.Spend30dKnown = in.Spend.Totals.PricingKnown
	out.Spend30dUSD = roundUSD(in.Spend.Totals.EstimatedUSD)
	for _, useCase := range in.Spend.UseCases {
		if strings.EqualFold(useCase.UseCase, "patrol") && useCase.PricingKnown {
			out.PatrolSpend30dUSD = roundUSD(useCase.EstimatedUSD)
		}
	}
	out.BudgetReached = in.BudgetUSD30d > 0 && out.Spend30dKnown && in.Spend.Totals.EstimatedUSD >= in.BudgetUSD30d

	// Schedule recommendation for per-token providers.
	switch {
	case !known:
		out.RecommendationReason = PatrolCostRecommendationPricingUnknown
	case !out.BilledPerToken:
		out.RecommendationReason = PatrolCostRecommendationNotBilledPerToken
	default:
		referenceBudget := in.BudgetUSD30d
		if referenceBudget <= 0 {
			referenceBudget = PatrolCostReferenceBudgetUSD
		}
		target := referenceBudget * PatrolCostRecommendedBudgetShare
		out.RecommendationTargetUSD = roundUSD(target)
		recommended := 0
		for _, option := range PatrolCostIntervalOptionsMinutes {
			if option < PatrolCostDefaultIntervalMinutes {
				continue
			}
			if scheduled30d(option) <= target {
				recommended = option
				break
			}
		}
		if recommended == 0 {
			out.RecommendedIntervalMinutes = PatrolCostIntervalOptionsMinutes[len(PatrolCostIntervalOptionsMinutes)-1]
			out.RecommendationReason = PatrolCostRecommendationExceedsEvenDaily
		} else {
			out.RecommendedIntervalMinutes = recommended
			out.RecommendationReason = PatrolCostRecommendationFitsBudgetShare
		}
	}
	return out
}

// ProjectPatrolCostForConfig resolves the effective Patrol model, interval,
// and budget from config, overriding model/interval when the caller is
// previewing a pending selection.
func ProjectPatrolCostForConfig(cfg *config.AIConfig, modelRoute string, intervalMinutes int, runs []PatrolRunRecord, spend cost.Summary, now time.Time) PatrolCostProjection {
	route := strings.TrimSpace(modelRoute)
	interval := intervalMinutes
	var budget float64
	if cfg != nil {
		if route == "" {
			route = cfg.GetPatrolModel()
		}
		if interval < 0 {
			interval = cfg.PatrolIntervalMinutes
		}
		budget = cfg.CostBudgetUSD30d
	}
	if interval < 0 {
		interval = PatrolCostDefaultIntervalMinutes
	}
	provider, model := "", ""
	if route != "" {
		provider, model = config.ParseModelString(route)
	}
	return ProjectPatrolCost(PatrolCostProjectionInput{
		Provider:        provider,
		Model:           model,
		IntervalMinutes: interval,
		BudgetUSD30d:    budget,
		Runs:            runs,
		Spend:           spend,
		Now:             now,
	})
}
