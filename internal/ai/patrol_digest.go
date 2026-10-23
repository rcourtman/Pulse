package ai

import (
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/cost"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// The Patrol digest answers one customer question: "what did Patrol do for me
// this week?" It is a pure rollup over records Pulse already keeps (run
// history, the findings store, canonical action audits, and the usage cost
// store). It adds no telemetry and no new persistence; see
// docs/PATROL_WEEKLY_DIGEST.md for the source of each line and its limits.

const (
	// PatrolDigestDefaultDays is the digest window when the caller does not
	// ask for one.
	PatrolDigestDefaultDays = 7
	// PatrolDigestMaxDays bounds the window so the rollup stays a weekly
	// account rather than a history export.
	PatrolDigestMaxDays = 30

	// patrolDigestActionOriginSurface mirrors the broker-owned origin surface
	// stamped on actions Patrol proposes (internal/api patrolActionOriginSurface).
	patrolDigestActionOriginSurface = "patrol"
	patrolDigestUsageUseCase        = "patrol"
)

// PatrolDigestWindow describes the period the digest covers and whether the
// retained run history actually reaches back that far.
type PatrolDigestWindow struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
	Days  int       `json:"days"`
	// HistoryComplete is false when the bounded run history store had
	// already dropped runs from inside the window, in which case HistorySince
	// is the oldest retained run and the digest only speaks from that point.
	HistoryComplete bool       `json:"history_complete"`
	HistorySince    *time.Time `json:"history_since,omitempty"`
}

// PatrolDigestRuns summarises how often Patrol looked.
type PatrolDigestRuns struct {
	Total          int `json:"total"`
	Scheduled      int `json:"scheduled"`
	EventTriggered int `json:"event_triggered"`
	Manual         int `json:"manual"`
	Failed         int `json:"failed"`
	// Checks is the total number of resource checks across all runs;
	// ResourcesCovered is the largest single-run resource count, which is the
	// best available proxy for the size of the estate Patrol watches.
	Checks           int        `json:"checks"`
	ResourcesCovered int        `json:"resources_covered"`
	LastRunAt        *time.Time `json:"last_run_at,omitempty"`
}

// PatrolDigestSeverityCounts splits findings by severity.
type PatrolDigestSeverityCounts struct {
	Critical int `json:"critical"`
	Warning  int `json:"warning"`
	Watch    int `json:"watch"`
	Info     int `json:"info"`
}

// PatrolDigestFindings summarises what Patrol raised and what happened to it.
type PatrolDigestFindings struct {
	// New counts findings first raised in the window (from run records, so it
	// survives finding cleanup). OpenBySeverity covers only findings raised in
	// the window that are still open now.
	New            int                        `json:"new"`
	OpenBySeverity PatrolDigestSeverityCounts `json:"open_by_severity"`
	// Resolved is every resolution in the window; AutoResolved is the subset
	// Patrol cleared itself because the condition was no longer detected.
	Resolved     int `json:"resolved"`
	AutoResolved int `json:"auto_resolved"`
	Dismissed    int `json:"dismissed"`
	Suppressed   int `json:"suppressed"`
}

// PatrolDigestInvestigations summarises the investigation runs that finished
// in the window, one per run, by how each ended. A run whose fix later ran or
// was verified carries that later outcome.
type PatrolDigestInvestigations struct {
	Total     int            `json:"total"`
	ByOutcome map[string]int `json:"by_outcome"`
}

// PatrolDigestActions summarises the canonical action lifecycle for actions
// Patrol proposed. Pending is the current queue, not a window count, because
// it is the one number the reader can still act on.
type PatrolDigestActions struct {
	Proposed int `json:"proposed"`
	Approved int `json:"approved"`
	Rejected int `json:"rejected"`
	Executed int `json:"executed"`
	Verified int `json:"verified"`
	Failed   int `json:"failed"`
	Pending  int `json:"pending"`
}

// PatrolDigestAlerts counts the reader's own alerts that Patrol responded to.
type PatrolDigestAlerts struct {
	Reviewed int `json:"reviewed"`
}

// PatrolDigestSpend is the estimated model spend attributed to Patrol.
type PatrolDigestSpend struct {
	EstimatedUSD float64 `json:"estimated_usd"`
	// PricingKnown is false when at least one call used a model with no known
	// price; EstimatedUSD then covers only the priced calls.
	PricingKnown bool  `json:"pricing_known"`
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
	// Prompt-cache buckets, disjoint from InputTokens and priced at the
	// provider's cache rates when known.
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens,omitempty"`
	Calls                    int   `json:"calls"`
}

// PatrolDigest is the wire payload for GET /api/ai/patrol/digest.
type PatrolDigest struct {
	GeneratedAt    time.Time                  `json:"generated_at"`
	Window         PatrolDigestWindow         `json:"window"`
	Mode           string                     `json:"mode"`
	Runs           PatrolDigestRuns           `json:"runs"`
	Findings       PatrolDigestFindings       `json:"findings"`
	Investigations PatrolDigestInvestigations `json:"investigations"`
	Actions        PatrolDigestActions        `json:"actions"`
	Alerts         PatrolDigestAlerts         `json:"alerts"`
	Spend          PatrolDigestSpend          `json:"spend"`
}

// PatrolDigestInput carries the raw records the digest is built from. Every
// slice may be nil; the digest then reports zero for that line.
type PatrolDigestInput struct {
	Now  time.Time
	Days int
	Mode string
	// Runs is the full retained run history, newest first or in any order.
	Runs []PatrolRunRecord
	// RunHistoryCapacity is the store's retention cap; when Runs is at
	// capacity and the oldest run falls inside the window the digest marks the
	// window as incomplete. Zero disables the check.
	RunHistoryCapacity int
	Findings           []*Finding
	Actions            []unifiedresources.ActionAuditRecord
	Usage              []cost.UsageEvent
}

// NormalizePatrolDigestDays clamps a requested window into the supported range.
func NormalizePatrolDigestDays(days int) int {
	if days <= 0 {
		return PatrolDigestDefaultDays
	}
	if days > PatrolDigestMaxDays {
		return PatrolDigestMaxDays
	}
	return days
}

// BuildPatrolDigest computes the digest for the window ending at input.Now.
func BuildPatrolDigest(in PatrolDigestInput) PatrolDigest {
	now := in.Now
	if now.IsZero() {
		now = time.Now()
	}
	now = now.UTC()
	days := NormalizePatrolDigestDays(in.Days)
	start := now.Add(-time.Duration(days) * 24 * time.Hour)
	inWindow := func(t time.Time) bool {
		return !t.IsZero() && !t.Before(start) && !t.After(now)
	}

	digest := PatrolDigest{
		GeneratedAt: now,
		Window: PatrolDigestWindow{
			Start:           start,
			End:             now,
			Days:            days,
			HistoryComplete: true,
		},
		Mode: normalizePatrolDigestMode(in.Mode),
		Investigations: PatrolDigestInvestigations{
			ByOutcome: map[string]int{},
		},
	}

	digest.Runs, digest.Alerts.Reviewed, digest.Findings.New, digest.Findings.AutoResolved = summarizeDigestRuns(in.Runs, inWindow)
	digest.Window.HistoryComplete, digest.Window.HistorySince = digestHistoryCoverage(in.Runs, in.RunHistoryCapacity, start)
	summarizeDigestFindings(&digest, in.Findings, inWindow)
	digest.Actions = summarizeDigestActions(in.Actions, inWindow)
	digest.Spend = summarizeDigestSpend(in.Usage, inWindow)
	return digest
}

func normalizePatrolDigestMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case config.PatrolAutonomyApproval:
		return config.PatrolAutonomyApproval
	case config.PatrolAutonomyAssisted:
		return config.PatrolAutonomyAssisted
	case config.PatrolAutonomyFull:
		return config.PatrolAutonomyFull
	default:
		return config.PatrolAutonomyMonitor
	}
}

func digestRunTime(run PatrolRunRecord) time.Time {
	if !run.StartedAt.IsZero() {
		return run.StartedAt
	}
	return run.CompletedAt
}

func isAlertTriggeredRun(reason string) bool {
	switch TriggerReason(strings.TrimSpace(reason)) {
	case TriggerReasonAlertFired, TriggerReasonAlertCleared, TriggerReasonAlertFlapping:
		return true
	}
	return false
}

func summarizeDigestRuns(runs []PatrolRunRecord, inWindow func(time.Time) bool) (PatrolDigestRuns, int, int, int) {
	var summary PatrolDigestRuns
	alerts := map[string]struct{}{}
	alertRunsWithoutID := 0
	newFindings := 0
	autoResolved := 0
	for _, run := range runs {
		at := digestRunTime(run)
		if !inWindow(at) {
			continue
		}
		summary.Total++
		switch reason := TriggerReason(strings.TrimSpace(run.TriggerReason)); {
		case reason == TriggerReasonManual:
			summary.Manual++
		case reason == "" || reason == TriggerReasonScheduled:
			summary.Scheduled++
		default:
			summary.EventTriggered++
		}
		if strings.EqualFold(strings.TrimSpace(run.Status), "error") {
			summary.Failed++
		}
		summary.Checks += run.ResourcesChecked
		if run.ResourcesChecked > summary.ResourcesCovered {
			summary.ResourcesCovered = run.ResourcesChecked
		}
		completed := run.CompletedAt
		if completed.IsZero() {
			completed = at
		}
		if summary.LastRunAt == nil || completed.After(*summary.LastRunAt) {
			last := completed.UTC()
			summary.LastRunAt = &last
		}
		if isAlertTriggeredRun(run.TriggerReason) {
			if id := strings.TrimSpace(run.AlertIdentifier); id != "" {
				alerts[id] = struct{}{}
			} else {
				alertRunsWithoutID++
			}
		}
		newFindings += run.NewFindings
		autoResolved += run.ResolvedFindings
	}
	return summary, len(alerts) + alertRunsWithoutID, newFindings, autoResolved
}

func digestHistoryCoverage(runs []PatrolRunRecord, capacity int, start time.Time) (bool, *time.Time) {
	if capacity <= 0 || len(runs) < capacity {
		return true, nil
	}
	var oldest time.Time
	for _, run := range runs {
		at := digestRunTime(run)
		if at.IsZero() {
			continue
		}
		if oldest.IsZero() || at.Before(oldest) {
			oldest = at
		}
	}
	if oldest.IsZero() || !oldest.After(start) {
		return true, nil
	}
	since := oldest.UTC()
	return false, &since
}

func summarizeDigestFindings(digest *PatrolDigest, findings []*Finding, inWindow func(time.Time) bool) {
	manualResolved := 0
	for _, f := range findings {
		if f == nil {
			continue
		}
		if inWindow(f.DetectedAt) && f.ResolvedAt == nil && strings.TrimSpace(f.DismissedReason) == "" {
			switch f.Severity {
			case FindingSeverityCritical:
				digest.Findings.OpenBySeverity.Critical++
			case FindingSeverityWarning:
				digest.Findings.OpenBySeverity.Warning++
			case FindingSeverityWatch:
				digest.Findings.OpenBySeverity.Watch++
			default:
				digest.Findings.OpenBySeverity.Info++
			}
		}
		if f.ResolvedAt != nil && inWindow(*f.ResolvedAt) && !f.AutoResolved {
			manualResolved++
		}
		for _, event := range f.Lifecycle {
			if !inWindow(event.At) {
				continue
			}
			switch event.Type {
			case "dismissed":
				digest.Findings.Dismissed++
			case "suppressed":
				digest.Findings.Suppressed++
			}
		}
		for _, outcome := range digestInvestigationRuns(f.Lifecycle, inWindow) {
			digest.Investigations.Total++
			digest.Investigations.ByOutcome[outcome]++
		}
	}
	digest.Findings.Resolved = digest.Findings.AutoResolved + manualResolved
}

// digestInvestigationRuns returns how each investigation run recorded in a
// finding's lifecycle ended, for the runs that finished inside the window.
// Each run counts once, in the window it finished. The lifecycle keeps a
// finding's latest 50 rows, so a run whose rows it has already dropped is not
// counted.
//
// A run's start and end are "investigation_updated" rows that move the status
// into and out of running; a regression clears the status of a run still in
// progress, so its end can also start from an empty status. A run whose own
// fix is verified while it is open logs "verification_passed" at that
// verification and no closing row, unless stuck-run recovery closes it or its
// own end records a different outcome. The digest counts such a run at the
// verification, keeps the verified result, and treats a closing row that
// follows as the same run. A run whose end the store never records stays open
// here until the next run starts, and a verification of its fix in that time
// finishes it. After a run finishes, its fix can still move through the action
// lifecycle ("investigation_outcome", then "verification_passed" once
// verified); those rows relabel the latest finished run rather than counting
// as another investigation. Rows carry no session ID, so the walker relies on
// their order: a late end written by an older run reads as the end of
// whichever run is open.
func digestInvestigationRuns(lifecycle []FindingLifecycleEvent, inWindow func(time.Time) bool) []string {
	running := string(InvestigationStatusRunning)
	var finished []*string
	var latest *string
	open, closedByVerification := false, false
	finish := func(outcome string, at time.Time) {
		latest = &outcome
		if inWindow(at) {
			finished = append(finished, latest)
		}
	}
	for _, event := range lifecycle {
		switch event.Type {
		case "investigation_updated":
			prev := strings.TrimSpace(event.Metadata["prev_status"])
			next := strings.TrimSpace(event.Metadata["next_status"])
			switch {
			case next == running:
				open, closedByVerification = true, false
			case next != "" && (prev == running || open):
				if !closedByVerification {
					finish(digestRunOutcome(event.Metadata["next_outcome"], next), event.At)
				}
				open, closedByVerification = false, false
			}
		case "investigation_outcome":
			// While a run is open its closing row carries the result.
			if outcome := strings.TrimSpace(event.Metadata["next_outcome"]); !open && latest != nil && outcome != "" {
				*latest = strings.ToLower(outcome)
			}
		case "verification_passed":
			if open {
				open, closedByVerification = false, true
				finish(string(InvestigationOutcomeFixVerified), event.At)
			} else if latest != nil {
				*latest = string(InvestigationOutcomeFixVerified)
			}
		}
	}
	outcomes := make([]string, len(finished))
	for i, outcome := range finished {
		outcomes[i] = *outcome
	}
	return outcomes
}

// digestRunOutcome names how a run ended. A run that failed before reaching
// any outcome is "failed", the same bucket telemetry uses for that state.
func digestRunOutcome(outcome, status string) string {
	if outcome = strings.ToLower(strings.TrimSpace(outcome)); outcome != "" {
		return outcome
	}
	if strings.EqualFold(strings.TrimSpace(status), string(InvestigationStatusFailed)) {
		return "failed"
	}
	return "unknown"
}

func summarizeDigestActions(records []unifiedresources.ActionAuditRecord, inWindow func(time.Time) bool) PatrolDigestActions {
	var summary PatrolDigestActions
	for _, record := range records {
		if record.Origin == nil || strings.TrimSpace(record.Origin.Surface) != patrolDigestActionOriginSurface {
			continue
		}
		if record.State == unifiedresources.ActionStatePending {
			summary.Pending++
		}
		if inWindow(record.CreatedAt) {
			summary.Proposed++
		}
		for _, approval := range record.Approvals {
			if !inWindow(approval.Timestamp) {
				continue
			}
			switch approval.Outcome {
			case unifiedresources.OutcomeApproved:
				summary.Approved++
			case unifiedresources.OutcomeRejected:
				summary.Rejected++
			}
		}
		if !inWindow(record.UpdatedAt) {
			continue
		}
		truth := unifiedresources.CanonicalActionResultV2(record)
		switch truth.Execution.Status {
		case unifiedresources.ActionExecutionSucceeded:
			summary.Executed++
			if truth.Verification.Status == unifiedresources.ActionVerificationConfirmed {
				summary.Verified++
			}
		case unifiedresources.ActionExecutionFailed:
			summary.Executed++
			summary.Failed++
		}
	}
	return summary
}

func summarizeDigestSpend(events []cost.UsageEvent, inWindow func(time.Time) bool) PatrolDigestSpend {
	summary := PatrolDigestSpend{PricingKnown: true}
	for _, event := range events {
		if !inWindow(event.Timestamp) || !strings.EqualFold(strings.TrimSpace(event.UseCase), patrolDigestUsageUseCase) {
			continue
		}
		summary.Calls++
		summary.InputTokens += int64(event.InputTokens)
		summary.OutputTokens += int64(event.OutputTokens)
		summary.CacheCreationInputTokens += int64(event.CacheCreationInputTokens)
		summary.CacheReadInputTokens += int64(event.CacheReadInputTokens)
		provider, model := cost.ResolveProviderAndModel(event.Provider, event.RequestModel, event.ResponseModel)
		usd, known, _ := cost.EstimateUsageUSD(provider, model, event.TokenUsage())
		if !known {
			summary.PricingKnown = false
			continue
		}
		summary.EstimatedUSD += usd
	}
	return summary
}

// PatrolDigestOutcomeOrder returns the by-outcome keys in a stable, most
// actionable-first order so presentation layers do not have to know the
// vocabulary.
func PatrolDigestOutcomeOrder(byOutcome map[string]int) []string {
	priority := map[string]int{
		"needs_attention":          0,
		"fix_failed":               1,
		"fix_verification_failed":  2,
		"cannot_fix":               3,
		"timed_out":                4,
		"failed":                   5,
		"unclassified":             6,
		"fix_queued":               7,
		"fix_executed":             8,
		"fix_verification_unknown": 9,
		"fix_verified":             10,
		"resolved":                 11,
		"fix_rejected":             12,
		"interrupted":              13,
	}
	keys := make([]string, 0, len(byOutcome))
	for key := range byOutcome {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		pi, okI := priority[keys[i]]
		pj, okJ := priority[keys[j]]
		if okI != okJ {
			return okI
		}
		if pi != pj {
			return pi < pj
		}
		return keys[i] < keys[j]
	})
	return keys
}
