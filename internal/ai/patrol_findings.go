// patrol_findings.go manages the finding lifecycle: creation, resolution, dismissal,
// investigation-triggered action artifact capture and verification,
// and the adapter types that bridge patrol findings to the investigation subsystem.
package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/baseline"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/relay"
	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
	"github.com/rs/zerolog/log"
)

const (
	patrolRuntimeFindingKey = "ai-patrol-error"
	patrolRuntimeResourceID = "ai-service"
)

func patrolFindingUsesSyntheticRuntimeResource(f *Finding) bool {
	return f != nil && (f.Key == patrolRuntimeFindingKey || f.ResourceID == patrolRuntimeResourceID)
}

func patrolRuntimeFindingManualActionError(action string) error {
	return fmt.Errorf(
		"Patrol runtime findings cannot be %s manually; fix Patrol provider configuration in Pulse Intelligence settings and rerun Patrol",
		action,
	)
}

// recordFinding stores a finding, syncs it to the unified store, and triggers follow-up actions.
func (p *PatrolService) recordFinding(f *Finding) bool {
	return p.recordFindingWithInvestigation(f, true)
}

// recordFindingDuringPatrol records a model-reported finding without starting
// Pro investigation while the Watch run's finding adapter is still installed
// on the shared chat executor. The completed run dispatches investigation only
// after its durable run record has been written and the adapter has been
// cleared.
func (p *PatrolService) recordFindingDuringPatrol(f *Finding) bool {
	return p.recordFindingWithInvestigation(f, false)
}

func (p *PatrolService) recordFindingWithInvestigation(f *Finding, investigate bool) bool {
	if p == nil || p.findings == nil || f == nil {
		return false
	}

	isNew := p.findings.Add(f)
	stored := p.findings.Get(f.ID)
	if stored == nil {
		return false
	}

	if isNew {
		log.Info().
			Str("finding_id", stored.ID).
			Str("severity", string(stored.Severity)).
			Str("resource", stored.ResourceName).
			Str("title", stored.Title).
			Msg("AI Patrol: New finding")

		// Findings are evidence. Pulse does not manufacture remediation plans
		// from category/title heuristics; the selected model owns diagnosis and
		// remediation reasoning, with Pulse enforcing approval/verification gates
		// after the model proposes an action.

		// Send push notification for new critical/warning findings
		if stored.Severity == FindingSeverityCritical || stored.Severity == FindingSeverityWarning {
			p.mu.RLock()
			pushCb := p.pushNotifyCallback
			p.mu.RUnlock()
			if pushCb != nil {
				pushCb(relay.NewPatrolFindingNotification(
					stored.ID,
					string(stored.Severity),
					string(stored.Category),
					stored.Title,
				))
			}
		}
	}

	// Keep unified store in sync even when findings transition to snoozed/dismissed/resolved.
	// The unified UI can filter by status; losing updates here makes the patrol loop look broken.
	if p.unifiedFindingCallback != nil {
		p.unifiedFindingCallback(stored)
	}

	if investigate {
		// Trigger autonomous investigation if enabled and finding warrants it.
		p.MaybeInvestigateFinding(stored)
	}

	return isNew
}

// stampCapacityForecasts joins the deterministic capacity forecasts computed
// during a run onto matching active findings. This makes the trend/eta a
// first-class structured signal on the finding instead of relying on the
// model to optionally mention it in prose. When several metrics forecast the
// same resource, the most urgent (soonest to fill) wins. No-op without
// forecasts or a findings store.
func (p *PatrolService) stampCapacityForecasts(forecasts []seedForecast) {
	resources := 0
	for _, sf := range forecasts {
		if strings.TrimSpace(sf.resourceID) != "" {
			resources++
		}
	}
	log.Info().
		Int("forecasts", len(forecasts)).
		Int("resources", resources).
		Msg("AI Patrol: Capacity forecasts available for stamping")

	if p == nil || p.findings == nil || len(forecasts) == 0 {
		return
	}
	best := make(map[string]CapacityForecast, len(forecasts))
	for _, sf := range forecasts {
		rid := strings.TrimSpace(sf.resourceID)
		if rid == "" {
			continue
		}
		want := CapacityForecast{
			Metric:      sf.metric,
			CurrentPct:  sf.current,
			DailyChange: sf.dailyChange,
			DaysToFull:  sf.daysToFull,
		}
		if cur, ok := best[rid]; !ok || forecastMoreUrgent(want, cur) {
			best[rid] = want
		}
	}
	if len(best) == 0 {
		return
	}
	changed := p.findings.StampCapacityForecasts(best)
	log.Info().
		Int("forecasts", len(forecasts)).
		Int("resources", len(best)).
		Int("stamped", changed).
		Msg("AI Patrol: Stamped capacity forecasts onto findings")
}

// forecastMoreUrgent reports whether forecast a is more urgent than b: a
// filling (positive days-to-full) estimate beats a stable/declining one, and
// among filling estimates the soonest wins. Among non-filling estimates the
// faster-rising one wins as a tiebreaker.
func forecastMoreUrgent(a, b CapacityForecast) bool {
	aFilling := a.DaysToFull > 0
	bFilling := b.DaysToFull > 0
	if aFilling != bFilling {
		return aFilling
	}
	if aFilling {
		return a.DaysToFull < b.DaysToFull
	}
	return a.DailyChange > b.DailyChange
}

// RejectManualActionForRuntimeFinding fails closed when a manual lifecycle action targets
// a Patrol-owned runtime finding such as the synthetic ai-service provider/runtime error.
func (p *PatrolService) RejectManualActionForRuntimeFinding(findingID string, action string) error {
	if p == nil || p.findings == nil {
		return fmt.Errorf("finding not found: %s", findingID)
	}
	finding := p.findings.Get(findingID)
	if finding == nil {
		return fmt.Errorf("finding not found: %s", findingID)
	}
	if patrolFindingUsesSyntheticRuntimeResource(finding) {
		return patrolRuntimeFindingManualActionError(action)
	}
	return nil
}

func (p *PatrolService) setBlockedReason(reason string) {
	p.setBlockedReasonWithCause(reason, "")
}

func (p *PatrolService) setBlockedReasonWithCause(reason string, cause PatrolFailureCause) {
	if reason == "" {
		return
	}
	p.mu.Lock()
	p.lastBlockedReason = reason
	p.lastBlockedCause = cause
	p.lastBlockedAt = time.Now()
	p.mu.Unlock()
}

func (p *PatrolService) clearBlockedReason() {
	p.mu.Lock()
	p.lastBlockedReason = ""
	p.lastBlockedCause = ""
	p.lastBlockedAt = time.Time{}
	p.mu.Unlock()
}

// GetFindingsForResource returns active findings for a specific resource
func (p *PatrolService) GetFindingsForResource(resourceID string) []*Finding {
	findings := p.findings.GetByResource(resourceID)
	normalizeFindingResourceTypes(findings)
	return findings
}

// GetFindingsSummary returns a summary of all findings
func (p *PatrolService) GetFindingsSummary() FindingsSummary {
	return p.findings.GetSummary()
}

// GetFindingsTrustSummary returns a snapshot of how currently-tracked
// findings have resolved. See FindingsTrustSummary for snapshot semantics.
func (p *PatrolService) GetFindingsTrustSummary() FindingsTrustSummary {
	return p.findings.GetTrustSummary()
}

// ResolveFinding marks a finding as resolved with a resolution note
// This is called when the AI successfully fixes an issue
func (p *PatrolService) ResolveFinding(findingID string, resolutionNote string) error {
	if findingID == "" {
		return fmt.Errorf("finding ID is required")
	}

	// Get the finding first to update its resolution note
	finding := p.findings.Get(findingID)
	if finding == nil {
		return fmt.Errorf("finding not found: %s", findingID)
	}

	// Update the user note with the resolution
	finding.UserNote = resolutionNote

	// Mark as resolved (not auto-resolved since user/AI initiated it)
	if !p.findings.Resolve(findingID, false) {
		return fmt.Errorf("failed to resolve finding: %s", findingID)
	}

	p.mu.RLock()
	resolveUnified := p.unifiedFindingResolver
	p.mu.RUnlock()
	if resolveUnified != nil {
		resolveUnified(findingID)
	}

	log.Info().
		Str("finding_id", findingID).
		Str("resolution_note", resolutionNote).
		Msg("AI resolved finding")

	return nil
}

// DismissFinding dismisses a finding with a reason and note
// This is called when the AI determines the finding is not actually an issue
// For reasons "expected_behavior" or "not_an_issue", a suppression rule is automatically created
func (p *PatrolService) DismissFinding(findingID string, reason string, note string) error {
	if findingID == "" {
		return fmt.Errorf("finding ID is required")
	}

	// Validate reason
	validReasons := map[string]bool{"not_an_issue": true, "expected_behavior": true, "will_fix_later": true}
	if !validReasons[reason] {
		return fmt.Errorf("invalid reason: %s", reason)
	}

	// Check that the finding exists
	finding := p.findings.Get(findingID)
	if finding == nil {
		return fmt.Errorf("finding not found: %s", findingID)
	}
	if patrolFindingUsesSyntheticRuntimeResource(finding) {
		return patrolRuntimeFindingManualActionError("dismissed")
	}

	// Dismiss the finding:
	// - "not_an_issue" creates permanent suppression (true false positive)
	// - "expected_behavior" and "will_fix_later" just acknowledge (stays visible but marked)
	if !p.findings.Dismiss(findingID, reason, note) {
		return fmt.Errorf("failed to dismiss finding: %s", findingID)
	}

	log.Info().
		Str("finding_id", findingID).
		Str("reason", reason).
		Str("note", note).
		Bool("permanently_suppressed", reason == "not_an_issue").
		Msg("AI dismissed finding")

	return nil
}

// GetRunHistory returns the history of patrol runs
// If limit is > 0, returns at most that many records
func (p *PatrolService) GetRunHistory(limit int) []PatrolRunRecord {
	runs := filterPatrolRunRecordsForRuntimeEvidence(p.runHistoryStore.GetAll())
	return limitPatrolRunRecords(runs, limit)
}

// GetRunByID returns a single patrol run from history.
func (p *PatrolService) GetRunByID(id string) (PatrolRunRecord, bool) {
	if strings.TrimSpace(id) == "" {
		return PatrolRunRecord{}, false
	}
	run, ok := p.runHistoryStore.GetByID(id)
	if !ok {
		return PatrolRunRecord{}, false
	}
	if !IsDemoMode() && isDemoPatrolRunRecord(run) {
		return PatrolRunRecord{}, false
	}
	return run, true
}

// GetAllFindings returns all active findings sorted by severity
// Only returns critical and warning findings - watch/info are filtered out as noise
func (p *PatrolService) GetAllFindings() []*Finding {
	findings := p.findings.GetActive(FindingSeverityWarning)
	normalizeFindingResourceTypes(findings)

	// Sort by severity (critical first) then by time
	severityOrder := map[FindingSeverity]int{
		FindingSeverityCritical: 0,
		FindingSeverityWarning:  1,
		FindingSeverityWatch:    2,
		FindingSeverityInfo:     3,
	}

	sort.Slice(findings, func(i, j int) bool {
		if severityOrder[findings[i].Severity] != severityOrder[findings[j].Severity] {
			return severityOrder[findings[i].Severity] < severityOrder[findings[j].Severity]
		}
		return findings[i].DetectedAt.After(findings[j].DetectedAt)
	})

	return findings
}

// GetAllFindingsIncludingResolved returns active + resolved + dismissed +
// snoozed findings at warning severity or higher, sorted by severity then
// recency. Used by the Resolved tab in the Patrol UI so operators can audit
// the auto_resolved set credited in the trust strip — without this path
// the strip surfaced "8 auto-resolved" but the operator could not click
// through to see what those eight were.
func (p *PatrolService) GetAllFindingsIncludingResolved() []*Finding {
	all := p.findings.GetAll(nil)
	normalizeFindingResourceTypes(all)

	// Two orderings — they look similar but they're inverse:
	//   filterOrder is "low-to-high severity" (info=0, ..., critical=3) so
	//     `>= minOrder` keeps everything at or above the floor. This is the
	//     same convention FindingsStore.GetActive uses.
	//   sortOrder is "high-to-low priority" (critical=0, ..., info=3) so a
	//     `<` comparison surfaces critical first in the result slice.
	// Conflating the two earlier let watch-severity findings leak through
	// the warning floor in the test, because watch's "sort priority"
	// number was higher than warning's even though its severity is lower.
	filterOrder := map[FindingSeverity]int{
		FindingSeverityInfo:     0,
		FindingSeverityWatch:    1,
		FindingSeverityWarning:  2,
		FindingSeverityCritical: 3,
	}
	sortOrder := map[FindingSeverity]int{
		FindingSeverityCritical: 0,
		FindingSeverityWarning:  1,
		FindingSeverityWatch:    2,
		FindingSeverityInfo:     3,
	}
	minOrder := filterOrder[FindingSeverityWarning]

	filtered := make([]*Finding, 0, len(all))
	for _, f := range all {
		if filterOrder[f.Severity] >= minOrder {
			filtered = append(filtered, f)
		}
	}

	sort.Slice(filtered, func(i, j int) bool {
		// Active first, then resolved — operator typically wants to review
		// the live set before drilling into history.
		ai := filtered[i].IsActive()
		aj := filtered[j].IsActive()
		if ai != aj {
			return ai
		}
		if sortOrder[filtered[i].Severity] != sortOrder[filtered[j].Severity] {
			return sortOrder[filtered[i].Severity] < sortOrder[filtered[j].Severity]
		}
		// Most recent activity first within each bucket.
		return filtered[i].LastSeenAt.After(filtered[j].LastSeenAt)
	})

	return filtered
}

func normalizeFindingResourceTypes(findings []*Finding) {
	for _, f := range findings {
		if f == nil {
			continue
		}
		if strings.TrimSpace(f.ResourceType) == "" {
			f.ResourceType = inferFindingResourceType(f.ResourceID, f.ResourceName)
			continue
		}
		if normalized := canonicalFindingResourceType(f.ResourceType); normalized != "" {
			f.ResourceType = normalized
			continue
		}
		f.ResourceType = inferFindingResourceType(f.ResourceID, f.ResourceName)
	}
}

// GetFindingsHistory returns all findings including resolved ones for history display
// Optionally filter by startTime
func (p *PatrolService) GetFindingsHistory(startTime *time.Time) []*Finding {
	findings := p.findings.GetAll(startTime)
	normalizeFindingResourceTypes(findings)

	// Sort by detected time (newest first)
	sort.Slice(findings, func(i, j int) bool {
		return findings[i].DetectedAt.After(findings[j].DetectedAt)
	})

	return findings
}

// ForcePatrol triggers an immediate patrol run.
// Uses context.Background() since this runs async after the HTTP response.
func (p *PatrolService) ForcePatrol(ctx context.Context) {
	runCtx := context.Background()
	if ctx != nil {
		runCtx = context.WithoutCancel(ctx)
	}
	go p.runPatrolWithTrigger(runCtx, TriggerReasonManual, nil)
}

// chatServiceExecutorAccessor is satisfied by *chat.Service, allowing patrol to
// access the executor without adding GetExecutor to the ChatServiceProvider interface.
type chatServiceExecutorAccessor interface {
	GetExecutor() *tools.PulseToolExecutor
}

// patrolFindingCreatorAdapter implements tools.PatrolFindingCreator by wrapping
// the PatrolService's existing FindingsStore and recordFinding method.
type patrolFindingCreatorAdapter struct {
	patrol            *PatrolService
	snap              patrolRuntimeState
	findingsMu        sync.Mutex
	findings          []*Finding
	assessments       []PatrolFindingAssessment
	queriedFindingIDs []string
	resolvedIDs       []string
	rejectedCount     int
	checkedFindings   bool
}

func newPatrolFindingCreatorAdapterState(p *PatrolService, snap patrolRuntimeState) *patrolFindingCreatorAdapter {
	return &patrolFindingCreatorAdapter{
		patrol: p,
		snap:   snap,
	}
}

func (a *patrolFindingCreatorAdapter) CreateFinding(input tools.PatrolFindingInput) (string, bool, error) {
	// Map severity
	var sev FindingSeverity
	switch strings.ToLower(input.Severity) {
	case "critical":
		sev = FindingSeverityCritical
	case "warning":
		sev = FindingSeverityWarning
	case "watch":
		sev = FindingSeverityWatch
	default:
		sev = FindingSeverityInfo
	}

	// Map category
	var cat FindingCategory
	switch strings.ToLower(input.Category) {
	case "performance":
		cat = FindingCategoryPerformance
	case "capacity":
		cat = FindingCategoryCapacity
	case "reliability":
		cat = FindingCategoryReliability
	case "backup":
		cat = FindingCategoryBackup
	case "security":
		cat = FindingCategorySecurity
	default:
		cat = FindingCategoryGeneral
	}

	// Normalize key for stable dedup
	normalizedKey := normalizeFindingKey(input.Key)
	if normalizedKey == "" {
		normalizedKey = normalizeFindingKey(input.Title)
		if normalizedKey == "" {
			normalizedKey = "llm-finding"
		}
	}

	// Generate stable ID
	id := generateFindingID(input.ResourceID, string(cat), normalizedKey)

	finding := &Finding{
		ID:             id,
		Key:            normalizedKey,
		Severity:       sev,
		Category:       cat,
		ResourceID:     input.ResourceID,
		ResourceName:   input.ResourceName,
		ResourceType:   input.ResourceType,
		Title:          input.Title,
		Description:    input.Description,
		Impact:         input.Impact,
		Recommendation: input.Recommendation,
		Evidence:       input.Evidence,
		Source:         "ai-analysis",
	}

	// Inline validation: check if finding is actionable against current metrics
	if !a.isActionable(finding) {
		// Determine which metric caused rejection for logging and metrics
		rejectedMetric := "unknown"
		keyLower := strings.ToLower(finding.Key)
		titleLower := strings.ToLower(finding.Title)
		if strings.Contains(keyLower, "cpu") || strings.Contains(titleLower, "cpu") {
			rejectedMetric = "cpu"
		} else if strings.Contains(keyLower, "memory") || strings.Contains(keyLower, "mem") || strings.Contains(titleLower, "memory") {
			rejectedMetric = "memory"
		} else if strings.Contains(keyLower, "disk") || strings.Contains(keyLower, "storage") || strings.Contains(titleLower, "disk") {
			rejectedMetric = "disk"
		}
		a.findingsMu.Lock()
		a.rejectedCount++
		a.findingsMu.Unlock()
		GetPatrolMetrics().RecordFindingRejected(input.ResourceType, rejectedMetric)
		log.Info().
			Str("finding_id", id).
			Str("title", input.Title).
			Str("resource", input.ResourceName).
			Str("resource_type", input.ResourceType).
			Str("rejected_metric", rejectedMetric).
			Msg("AI Patrol: Finding rejected by threshold validation")

		// Broadcast rejection to stream consumers
		a.patrol.broadcast(PatrolStreamEvent{
			Type:    "finding_rejected",
			Content: fmt.Sprintf("Finding rejected: %s on %s (metric %s below threshold)", input.Title, input.ResourceName, rejectedMetric),
		})

		return id, false, fmt.Errorf("finding rejected: metrics do not support this finding (below actionable thresholds)")
	}

	// Record finding via PatrolService
	isNew := a.patrol.recordFindingDuringPatrol(finding)

	// Track for run stats
	a.trackCollectedFinding(finding)

	return id, isNew, nil
}

func (a *patrolFindingCreatorAdapter) findingInCurrentScope(finding *Finding) bool {
	if finding == nil {
		return false
	}
	scopedResources := patrolRuntimeKnownResources(a.snap)
	return len(scopedResources) == 0 || scopedResources[finding.ResourceID] || scopedResources[finding.ResourceName]
}

func (a *patrolFindingCreatorAdapter) trackCollectedFinding(finding *Finding) {
	if finding == nil {
		return
	}
	a.findingsMu.Lock()
	defer a.findingsMu.Unlock()
	for index, existing := range a.findings {
		if existing != nil && existing.ID == finding.ID {
			a.findings[index] = finding
			return
		}
	}
	a.findings = append(a.findings, finding)
}

// AssessFinding records the model's explicit terminal verdict for an existing
// finding in this run. Present refreshes the durable finding heartbeat and
// current evidence, resolved delegates to the existing fail-closed verifier,
// and uncertain remains a run-owned assessment without mutating the finding.
func (a *patrolFindingCreatorAdapter) AssessFinding(input tools.PatrolFindingAssessmentInput) error {
	finding := a.patrol.findings.Get(input.FindingID)
	if finding == nil || !finding.IsActive() {
		return fmt.Errorf("finding %s not found or no longer active", input.FindingID)
	}
	if !a.findingInCurrentScope(finding) {
		return fmt.Errorf("finding %s is outside the current patrol scope", input.FindingID)
	}

	verdict := strings.ToLower(strings.TrimSpace(input.Verdict))
	assessment := PatrolFindingAssessment{
		FindingID:  strings.TrimSpace(input.FindingID),
		Verdict:    verdict,
		Evidence:   strings.TrimSpace(input.Evidence),
		Reason:     strings.TrimSpace(input.Reason),
		AssessedAt: time.Now().UTC(),
	}
	a.findingsMu.Lock()
	for _, existing := range a.assessments {
		if existing.FindingID == assessment.FindingID {
			a.findingsMu.Unlock()
			return fmt.Errorf("finding %s already has verdict %s in this patrol run", assessment.FindingID, existing.Verdict)
		}
	}
	a.findingsMu.Unlock()

	switch verdict {
	case "present":
		refreshed := *finding
		refreshed.Evidence = assessment.Evidence
		if a.patrol.recordFinding(&refreshed) {
			return fmt.Errorf("finding %s unexpectedly became a new finding during assessment", assessment.FindingID)
		}
		if stored := a.patrol.findings.Get(assessment.FindingID); stored != nil {
			a.trackCollectedFinding(stored)
		}
	case "resolved":
		if err := a.ResolveFinding(assessment.FindingID, assessment.Reason+": "+assessment.Evidence); err != nil {
			return err
		}
	case "uncertain":
		// Intentionally no finding mutation. The run-owned assessment protects
		// this ID from absence-based stale reconciliation and keeps it active.
	default:
		return fmt.Errorf("invalid finding assessment verdict %q", verdict)
	}

	a.findingsMu.Lock()
	a.assessments = append(a.assessments, assessment)
	a.findingsMu.Unlock()
	log.Info().
		Str("finding_id", assessment.FindingID).
		Str("verdict", assessment.Verdict).
		Msg("AI Patrol: Existing finding assessed")
	return nil
}

// actionabilityThreshold returns the threshold below which a metric finding is rejected as noise.
// It reads user-configured PatrolThresholds (Watch level = lowest alarm tier) and falls back
// to hardcoded defaults (50/60/70) if the threshold is zero or unset.
// The resourceType parameter selects between node-level and guest-level thresholds where both exist.
func (a *patrolFindingCreatorAdapter) actionabilityThreshold(metric, resourceType string) float64 {
	a.patrol.mu.RLock()
	thresholds := a.patrol.thresholds
	a.patrol.mu.RUnlock()

	isNode := resourceType == "node"

	switch metric {
	case "cpu":
		// Only node-level CPU threshold exists; used for all resource types.
		if thresholds.NodeCPUWatch > 0 {
			return thresholds.NodeCPUWatch
		}
		return 50.0
	case "memory":
		if isNode {
			if thresholds.NodeMemWatch > 0 {
				return thresholds.NodeMemWatch
			}
		} else {
			if thresholds.GuestMemWatch > 0 {
				return thresholds.GuestMemWatch
			}
		}
		return 60.0
	case "disk":
		if thresholds.GuestDiskWatch > 0 {
			return thresholds.GuestDiskWatch
		}
		return 70.0
	case "storage":
		if thresholds.StorageWatch > 0 {
			return thresholds.StorageWatch
		}
		return 70.0
	default:
		return 50.0
	}
}

// isBaselineAnomaly checks if the given value is anomalously high compared to the learned
// baseline for this resource/metric. Returns true only for upward anomalies (rising above
// baseline), since dropping usage is not concerning. Returns false if baseline data is
// unavailable or insufficient.
func (a *patrolFindingCreatorAdapter) isBaselineAnomaly(resourceID, metric string, value float64) bool {
	a.patrol.mu.RLock()
	store := a.patrol.baselineStore
	a.patrol.mu.RUnlock()

	if store == nil {
		return false
	}

	severity, _, bl := store.CheckAnomaly(resourceID, metric, value)
	if severity == baseline.AnomalyNone || bl == nil {
		return false
	}

	// Only flag upward anomalies (value above baseline mean)
	return value > bl.Mean
}

// isActionable validates a finding against current metrics (inline version of the old
// validateAIFindings + isActionableFinding logic).
// Uses user-configured thresholds from PatrolThresholds and baseline anomaly detection
// as a second-chance check for findings below the threshold but statistically anomalous.
func (a *patrolFindingCreatorAdapter) isActionable(f *Finding) bool {
	resourceMetrics, hasInventory := a.actionabilityResourceMetrics()

	// Reject findings for resources that no longer exist in the current infrastructure.
	// Only enforce when we have state data (avoid rejecting during empty/error states).
	metrics, hasMetrics := resourceMetrics[f.ResourceID]
	if !hasMetrics {
		metrics, hasMetrics = resourceMetrics[f.ResourceName]
	}
	if !hasMetrics && hasInventory {
		// Resource not found — it may have been deleted. Reject the finding.
		return false
	}

	// Allow critical findings without metric threshold checks
	if f.Severity == FindingSeverityCritical {
		return true
	}
	// Allow backup and reliability findings without metric threshold checks
	if f.Category == FindingCategoryBackup || f.Category == FindingCategoryReliability {
		return true
	}

	if !hasMetrics {
		return true // empty state — benefit of doubt
	}

	key := strings.ToLower(f.Key)
	titleLower := strings.ToLower(f.Title)

	// CPU check
	if strings.Contains(key, "cpu") || strings.Contains(titleLower, "cpu") {
		if cpu, ok := metrics["cpu"]; ok && cpu < a.actionabilityThreshold("cpu", f.ResourceType) {
			// Below threshold — check if anomalous (statistically unusual spike)
			if a.isBaselineAnomaly(f.ResourceID, "cpu", cpu) {
				return true
			}
			return false
		}
	}
	// Memory check
	if strings.Contains(key, "memory") || strings.Contains(key, "mem") || strings.Contains(titleLower, "memory") {
		if mem, ok := metrics["memory"]; ok && mem < a.actionabilityThreshold("memory", f.ResourceType) {
			if a.isBaselineAnomaly(f.ResourceID, "memory", mem) {
				return true
			}
			return false
		}
	}
	// Disk/storage check
	if strings.Contains(key, "disk") || strings.Contains(key, "storage") || strings.Contains(titleLower, "disk") {
		if disk, ok := metrics["disk"]; ok && disk < a.actionabilityThreshold("disk", f.ResourceType) {
			if a.isBaselineAnomaly(f.ResourceID, "disk", disk) {
				return true
			}
			return false
		}
		if usage, ok := metrics["usage"]; ok && usage < a.actionabilityThreshold("storage", f.ResourceType) {
			if a.isBaselineAnomaly(f.ResourceID, "storage", usage) {
				return true
			}
			return false
		}
	}

	return true
}

func (a *patrolFindingCreatorAdapter) actionabilityResourceMetrics() (map[string]map[string]float64, bool) {
	return patrolActionabilityResourceMetrics(a.snap)
}

func (a *patrolFindingCreatorAdapter) ResolveFinding(findingID, reason string) error {
	finding := a.patrol.findings.Get(findingID)
	if finding == nil {
		return fmt.Errorf("finding %s not found or already resolved", findingID)
	}

	scopedResources := patrolRuntimeKnownResources(a.snap)
	if len(scopedResources) > 0 {
		if !scopedResources[finding.ResourceID] && !scopedResources[finding.ResourceName] {
			return fmt.Errorf("finding %s is outside the current patrol scope", findingID)
		}
	}

	// Event/persistent categories (backup, reliability, security, general)
	// must not be auto-resolved on absence — see the contract at
	// findings.go:CategorySupportsStaleAutoResolve. Before today's gate,
	// the LLM could call patrol_resolve_finding for a backup-failed finding
	// because its current investigation didn't surface a fresh failure
	// signal, and the next run's detector would re-detect the same
	// unresolved task. The "Backup failed" finding flapped 10x in a day
	// before this gate landed.
	//
	// For these categories, when a deterministic verifier exists for the
	// finding's key, run it before honoring the LLM's resolve. If the
	// verifier says the failure signal is still present, reject — the
	// underlying issue hasn't actually cleared. If the verifier confirms
	// the signal is gone, the LLM's resolve is grounded in real evidence.
	// Findings without a verifier fall through to the current trust-the-LLM
	// behavior to avoid regressing categories we haven't built verifiers
	// for yet.
	if !CategorySupportsStaleAutoResolve(finding.Category) && a.patrol.hasDeterministicVerifierForKey(finding.Key) {
		verifyCtx, verifyCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer verifyCancel()
		verified, verifyErr := a.patrol.VerifyFixResolved(verifyCtx, finding.ResourceID, finding.ResourceType, finding.Key, findingID)
		if verifyErr != nil {
			// Fail closed. Resolution of an event/persistent finding is
			// effectively permanent (next detection will register as a
			// regression, inflating counters and re-running the cycle the
			// rest of this branch's work spent commits closing). If the
			// deterministic verifier cannot confidently say the failure
			// signal is gone, we don't have grounds to honor the LLM's
			// judgment, even charitably — the LLM's "current investigation
			// didn't surface a fresh failure" is exactly the signal that
			// produced the bogus auto_resolved → re-detected cycles in
			// the first place. Surfacing the error back to the tool lets
			// the LLM choose to retry or escalate to the operator instead.
			log.Info().
				Err(verifyErr).
				Str("finding_id", findingID).
				Str("category", string(finding.Category)).
				Str("key", finding.Key).
				Msg("AI Patrol: rejected LLM resolve — deterministic verifier was inconclusive (fail closed)")
			return fmt.Errorf("cannot resolve %s: deterministic verification was inconclusive (%v). Confirm the underlying issue is actually fixed by running the verifier separately, or let the operator mark resolved", findingID, verifyErr)
		}
		if !verified {
			log.Info().
				Str("finding_id", findingID).
				Str("category", string(finding.Category)).
				Str("key", finding.Key).
				Msg("AI Patrol: rejected LLM resolve — deterministic verifier still detects the failure signal")
			return fmt.Errorf("cannot resolve %s: the failure signal is still present according to deterministic verification. Fix the underlying issue and re-run, do not resolve based on absence in the current investigation", findingID)
		}
	}

	resolved := a.patrol.findings.Resolve(findingID, true)
	if !resolved {
		return fmt.Errorf("finding %s not found or already resolved", findingID)
	}

	// Notify unified store
	a.patrol.mu.RLock()
	resolveUnified := a.patrol.unifiedFindingResolver
	a.patrol.mu.RUnlock()
	if resolveUnified != nil {
		resolveUnified(findingID)
	}

	a.findingsMu.Lock()
	a.resolvedIDs = append(a.resolvedIDs, findingID)
	a.findingsMu.Unlock()

	log.Info().
		Str("finding_id", findingID).
		Str("reason", reason).
		Msg("AI Patrol: Finding resolved via patrol tool")
	return nil
}

func (a *patrolFindingCreatorAdapter) GetActiveFindings(resourceID, minSeverity string) []tools.PatrolFindingInfo {
	a.findingsMu.Lock()
	a.checkedFindings = true
	a.findingsMu.Unlock()

	var minSev FindingSeverity
	switch strings.ToLower(minSeverity) {
	case "critical":
		minSev = FindingSeverityCritical
	case "warning":
		minSev = FindingSeverityWarning
	case "watch":
		minSev = FindingSeverityWatch
	default:
		minSev = FindingSeverityInfo
	}

	active := a.patrol.findings.GetActive(minSev)
	scopedResources := patrolRuntimeKnownResources(a.snap)
	var result []tools.PatrolFindingInfo
	for _, f := range active {
		if resourceID != "" && f.ResourceID != resourceID && f.ResourceName != resourceID {
			continue
		}
		if len(scopedResources) > 0 && !scopedResources[f.ResourceID] && !scopedResources[f.ResourceName] {
			continue
		}
		result = append(result, tools.PatrolFindingInfo{
			ID:           f.ID,
			Key:          f.Key,
			Severity:     string(f.Severity),
			Category:     string(f.Category),
			ResourceID:   f.ResourceID,
			ResourceName: f.ResourceName,
			ResourceType: f.ResourceType,
			Title:        f.Title,
			Description:  f.Description,
			DetectedAt:   f.DetectedAt.Format("2006-01-02 15:04"),
		})
		a.findingsMu.Lock()
		seen := false
		for _, findingID := range a.queriedFindingIDs {
			if findingID == f.ID {
				seen = true
				break
			}
		}
		if !seen {
			a.queriedFindingIDs = append(a.queriedFindingIDs, f.ID)
		}
		a.findingsMu.Unlock()
	}
	return result
}

func (a *patrolFindingCreatorAdapter) HasCheckedFindings() bool {
	a.findingsMu.Lock()
	defer a.findingsMu.Unlock()
	return a.checkedFindings
}

// getCollectedFindings returns all findings created during this patrol run.
func (a *patrolFindingCreatorAdapter) getCollectedFindings() []*Finding {
	a.findingsMu.Lock()
	defer a.findingsMu.Unlock()
	result := make([]*Finding, len(a.findings))
	copy(result, a.findings)
	return result
}

// getResolvedCount returns the number of findings resolved during this patrol run.
func (a *patrolFindingCreatorAdapter) getResolvedCount() int {
	a.findingsMu.Lock()
	defer a.findingsMu.Unlock()
	return len(a.resolvedIDs)
}

// getReportedFindingIDs returns the IDs of all findings created/re-reported this run.
func (a *patrolFindingCreatorAdapter) getReportedFindingIDs() []string {
	a.findingsMu.Lock()
	defer a.findingsMu.Unlock()
	ids := make([]string, len(a.findings))
	for i, f := range a.findings {
		ids[i] = f.ID
	}
	return ids
}

// getResolvedIDs returns the IDs of findings explicitly resolved by the LLM this run.
func (a *patrolFindingCreatorAdapter) getResolvedIDs() []string {
	a.findingsMu.Lock()
	defer a.findingsMu.Unlock()
	result := make([]string, len(a.resolvedIDs))
	copy(result, a.resolvedIDs)
	return result
}

func (a *patrolFindingCreatorAdapter) getAssessments() []PatrolFindingAssessment {
	a.findingsMu.Lock()
	defer a.findingsMu.Unlock()
	result := make([]PatrolFindingAssessment, len(a.assessments))
	copy(result, a.assessments)
	return result
}

func (a *patrolFindingCreatorAdapter) getAssessedFindingIDs() []string {
	a.findingsMu.Lock()
	defer a.findingsMu.Unlock()
	result := make([]string, 0, len(a.assessments))
	for _, assessment := range a.assessments {
		result = append(result, assessment.FindingID)
	}
	return result
}

func (a *patrolFindingCreatorAdapter) getQueriedFindingIDs() []string {
	a.findingsMu.Lock()
	defer a.findingsMu.Unlock()
	result := make([]string, len(a.queriedFindingIDs))
	copy(result, a.queriedFindingIDs)
	return result
}

// dispatchPatrolInvestigations advances model-reported findings from the
// completed Watch stage into Pro investigation. It must be called only after
// the Watch run record is durable and run-scoped executor state has unwound.
func (p *PatrolService) dispatchPatrolInvestigations(result *AIAnalysisResult) {
	if p == nil || p.findings == nil || result == nil {
		return
	}
	seen := make(map[string]bool, len(result.Findings))
	for _, candidate := range result.Findings {
		if candidate == nil || candidate.ID == "" || seen[candidate.ID] {
			continue
		}
		seen[candidate.ID] = true
		if stored := p.findings.Get(candidate.ID); stored != nil {
			p.MaybeInvestigateFinding(stored)
		}
	}
}

// findingKeyAliases maps directional synonyms the LLM plausibly assigns onto
// the canonical verifier vocabulary (the verifyFixDeterministically switch),
// so deduplication and deterministic verification meet on one key. Only
// unambiguous aliases belong here — node-offline is NOT guest-unreachable
// and pbs-job-failed is NOT backup-failed; mapping those would point the
// verifier at the wrong resource model.
var findingKeyAliases = map[string]string{
	"high-cpu":    "cpu-high",
	"high-memory": "memory-high",
	"high-disk":   "disk-high",
}

func normalizeFindingKey(key string) string {
	if key == "" {
		return ""
	}
	key = strings.TrimSpace(strings.ToLower(key))
	if key == "" {
		return ""
	}
	key = strings.ReplaceAll(key, "_", "-")
	key = strings.ReplaceAll(key, " ", "-")
	var b strings.Builder
	for _, r := range key {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		}
	}
	normalized := strings.Trim(b.String(), "-")
	if canonical, ok := findingKeyAliases[normalized]; ok {
		return canonical
	}
	return normalized
}

// recoverStuckInvestigations detects findings stuck in "running" state for longer than
// the investigation timeout and resets them to "failed/timed_out" so they can be retried.
// This handles the case where an investigation goroutine panics or is killed without
// properly updating the finding status.
func (p *PatrolService) recoverStuckInvestigations() {
	if p.findings == nil {
		return
	}
	const stuckThreshold = 15 * time.Minute // investigation timeout is 10min; allow 5min grace
	active := p.findings.GetActive(FindingSeverityWarning)
	recovered := 0
	for _, f := range active {
		if f.InvestigationStatus != string(InvestigationStatusRunning) {
			continue
		}
		if f.LastInvestigatedAt == nil {
			continue
		}
		if time.Since(*f.LastInvestigatedAt) < stuckThreshold {
			continue
		}
		// This finding has been "running" for too long — reset it
		p.findings.UpdateInvestigation(
			f.ID,
			f.InvestigationSessionID,
			string(InvestigationStatusFailed),
			string(InvestigationOutcomeTimedOut),
			f.LastInvestigatedAt,
			f.InvestigationAttempts,
		)
		recovered++
		log.Warn().
			Str("finding_id", f.ID).
			Str("resource", f.ResourceName).
			Time("last_investigated", *f.LastInvestigatedAt).
			Msg("AI Patrol: Recovered stuck investigation (exceeded timeout)")
	}
	if recovered > 0 {
		log.Info().Int("recovered", recovered).
			Msg("AI Patrol: Recovered stuck investigations")
	}
}

// retryTimedOutInvestigations re-triggers investigation for findings that failed due to timeout.
// Called at the end of each patrol run to give timed-out investigations another chance
// without waiting for the full 1-hour cooldown.
func (p *PatrolService) retryTimedOutInvestigations() {
	if p.findings == nil {
		return
	}
	active := p.findings.GetActive(FindingSeverityWarning)
	retried := 0
	for _, f := range active {
		if f.InvestigationStatus != string(InvestigationStatusFailed) {
			continue
		}
		if f.InvestigationOutcome != string(InvestigationOutcomeTimedOut) {
			continue
		}
		p.MaybeInvestigateFinding(f)
		retried++
	}
	if retried > 0 {
		log.Info().Int("retried", retried).
			Msg("AI Patrol: Retried timed-out investigations")
	}
}

// MaybeInvestigateFinding checks if a finding should be investigated and triggers investigation if so
// This is called both during scheduled patrol runs and when alert-triggered findings are created
func (p *PatrolService) MaybeInvestigateFinding(f *Finding) {
	p.mu.RLock()
	orchestrator := p.investigationOrchestrator
	aiService := p.aiService
	p.mu.RUnlock()

	// No orchestrator configured
	if orchestrator == nil {
		return
	}

	// Get autonomy level from AI config
	if aiService == nil {
		return
	}
	if aiService.GetConfig() == nil {
		return
	}
	autonomyLevel := aiService.GetEffectivePatrolAutonomyLevel()

	// Check if finding should be investigated
	if !f.ShouldInvestigate(autonomyLevel) {
		return
	}

	// Check if we can start another investigation (concurrency limit)
	if !orchestrator.CanStartInvestigation() {
		log.Debug().
			Str("finding_id", f.ID).
			Msg("Cannot start investigation: max concurrent investigations reached")
		return
	}

	// Convert Finding to shared finding type for the investigation orchestrator
	invFinding := f.ToCoreFinding()

	// Attach the operator-set state for the finding's resource so the
	// orchestrator's reasoning can incorporate the operator's
	// commitments (intentionally offline, never auto-remediate, active
	// maintenance window). Without this, Patrol can propose fixes that
	// contradict the operator's intent — the action broker refuses
	// them downstream (slice 33's `resource_remediation_locked:`), but
	// the proposal shouldn't have happened in the first place. The
	// projection is also the data path "Pulse uses the privileged
	// context" that defines the product's differentiation.
	if p.findings != nil {
		now := time.Now()
		if projection, ok := p.findings.OperatorStateProjectionFor(f.ResourceID, now); ok {
			ctx := &aicontracts.FindingOperatorContext{
				IntentionallyOffline: projection.IntentionallyOffline,
				NeverAutoRemediate:   projection.NeverAutoRemediate,
			}
			if window := projection.MaintenanceWindow; window != nil {
				ctx.MaintenanceWindowActive = !now.Before(window.StartAt) && now.Before(window.EndAt)
				start := window.StartAt
				end := window.EndAt
				ctx.MaintenanceStartAt = &start
				ctx.MaintenanceEndAt = &end
				ctx.MaintenanceReason = window.Reason
			}
			// Attach only when something meaningful is set —
			// otherwise leave the field nil so the orchestrator can
			// branch on absence rather than on zero values.
			if ctx.IntentionallyOffline ||
				ctx.NeverAutoRemediate ||
				ctx.MaintenanceStartAt != nil ||
				ctx.MaintenanceWindowActive {
				invFinding.OperatorContext = ctx
			}
		}
	}

	// Trigger investigation in background with a timeout to prevent indefinite runs.
	// Track with WaitGroup so graceful shutdown can wait for completion.
	p.investigationWg.Add(1)
	go func() {
		defer p.investigationWg.Done()

		// Re-read autonomy level at execution time to avoid using a stale value
		// captured before the goroutine was scheduled.
		currentCfg := aiService.GetConfig()
		if currentCfg == nil {
			log.Warn().Str("finding_id", f.ID).Msg("AI config unavailable at investigation start, aborting")
			return
		}
		currentAutonomy := aiService.GetEffectivePatrolAutonomyLevel()

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		if err := orchestrator.InvestigateFinding(ctx, invFinding, currentAutonomy); err != nil {
			log.Error().
				Err(err).
				Str("finding_id", f.ID).
				Msg("Failed to start investigation")
			return
		}

		// The orchestrator updates the patrol findings store; sync the latest state to the unified store.
		// This makes fix verification and resolution visible as an actual closed loop in the UI.
		var pushUnified UnifiedFindingCallback
		var resolveUnified func(string)
		var pushCb PushNotifyCallback
		p.mu.RLock()
		pushUnified = p.unifiedFindingCallback
		resolveUnified = p.unifiedFindingResolver
		pushCb = p.pushNotifyCallback
		p.mu.RUnlock()
		if latest := p.findings.Get(f.ID); latest != nil {
			var latestInvestigation *InvestigationSession
			if orchestrator != nil {
				latestInvestigation = orchestrator.GetInvestigationByFinding(latest.ID)
			}
			if record := BuildFindingInvestigationRecord(latest, latestInvestigation); record != nil {
				// When a remediation plan exists for this finding, lift its
				// per-step rollback strings into record.Rollback so the
				// operator-facing investigation surface answers
				// "what's the undo for the proposed fix?" at the record root
				// rather than only in nested per-step payload.
				if engine := p.remediationEngine; engine != nil {
					if plan := engine.GetPlanForFinding(latest.ID); plan != nil {
						record.Rollback = AggregatePlanRollbackSteps(plan)
					}
				}
				if p.findings.UpdateInvestigationRecord(latest.ID, record) {
					if refreshed := p.findings.Get(latest.ID); refreshed != nil {
						latest = refreshed
					} else {
						latest.InvestigationRecord = record
					}
				}
			}
			if pushUnified != nil {
				pushUnified(latest)
			}
			if latest.ResolvedAt != nil && resolveUnified != nil {
				resolveUnified(latest.ID)
			}

			// Send push notifications for investigation outcomes
			if pushCb != nil {
				switch latest.InvestigationOutcome {
				case string(InvestigationOutcomeFixQueued):
					if latestInvestigation != nil && latestInvestigation.Action != nil && latestInvestigation.Action.State == "pending_approval" {
						pushCb(relay.NewActionDecisionNotification(
							latestInvestigation.Action.ActionID,
							latest.Title,
						))
					}
				case string(InvestigationOutcomeFixExecuted), string(InvestigationOutcomeFixVerified):
					pushCb(relay.NewFixCompletedNotification(latest.ID, latest.Title, true))
				case string(InvestigationOutcomeFixFailed), string(InvestigationOutcomeFixVerificationFailed):
					pushCb(relay.NewFixCompletedNotification(latest.ID, latest.Title, false))
				}
			}
		}

		// Typed remediation now flows exclusively through the action
		// proposal channel: the investigation's ActionReference points at
		// the canonical action audit, so no command-shaped remediation
		// plan artifact is generated from investigation prose.
	}()

	log.Info().
		Str("finding_id", f.ID).
		Str("severity", string(f.Severity)).
		Str("resource", f.ResourceName).
		Str("autonomy_level", autonomyLevel).
		Msg("Triggered autonomous investigation for finding")
}

// PublishFindingLifecycleUpdate projects a reconciled action outcome to the
// unified finding owner and, for terminal execution outcomes, to mobile push.
// It is called only after the finding store changed, so duplicate action
// callbacks and read-time hydration do not emit duplicate notifications.
func (p *PatrolService) PublishFindingLifecycleUpdate(findingID string) {
	if p == nil || p.findings == nil {
		return
	}
	finding := p.findings.Get(findingID)
	if finding == nil {
		return
	}
	p.mu.RLock()
	pushUnified := p.unifiedFindingCallback
	resolveUnified := p.unifiedFindingResolver
	pushNotify := p.pushNotifyCallback
	p.mu.RUnlock()
	if pushUnified != nil {
		pushUnified(finding)
	}
	if finding.ResolvedAt != nil && resolveUnified != nil {
		resolveUnified(finding.ID)
	}
	if pushNotify == nil {
		return
	}
	switch InvestigationOutcome(finding.InvestigationOutcome) {
	case InvestigationOutcomeFixVerified:
		pushNotify(relay.NewActionOutcomeNotification(finding.ID, finding.Title, "verified"))
	case InvestigationOutcomeFixVerificationFailed:
		pushNotify(relay.NewActionOutcomeNotification(finding.ID, finding.Title, "failed"))
	case InvestigationOutcomeFixVerificationUnknown:
		pushNotify(relay.NewActionOutcomeNotification(finding.ID, finding.Title, "unverified"))
	case InvestigationOutcomeFixFailed:
		pushNotify(relay.NewActionOutcomeNotification(finding.ID, finding.Title, "execution_failed"))
	}
}

// VerifyFixResolved runs a lightweight scoped patrol to check if the issue
// identified by the given finding has been resolved after a fix was executed.
// It bypasses tryStartRun (the patrol mutex) because verification runs inline
// within the investigation goroutine.
func (p *PatrolService) VerifyFixResolved(ctx context.Context, resourceID, resourceType, findingKey, findingID string) (bool, error) {
	if p == nil || !p.hasPatrolRuntimeInputs() {
		return false, fmt.Errorf("%w: no patrol runtime state available", aicontracts.ErrVerificationUnknown)
	}
	if ctx == nil {
		ctx = context.Background()
	}

	startTime := time.Now()

	// Prefer canonical finding details from store when available.
	var finding *Finding
	if p.findings != nil && findingID != "" {
		finding = p.findings.Get(findingID)
	}
	if finding != nil {
		if resourceID == "" {
			resourceID = finding.ResourceID
		}
		if resourceType == "" {
			resourceType = finding.ResourceType
		}
		if findingKey == "" {
			findingKey = finding.Key
		}
	}

	log.Info().
		Str("finding_id", findingID).
		Str("resource_id", resourceID).
		Str("key", findingKey).
		Msg("Running deterministic verification to confirm fix")

	verified, verifyErr := p.verifyFixDeterministically(ctx, finding, resourceID, resourceType, findingKey, findingID)

	endTime := time.Now()
	duration := endTime.Sub(startTime)

	// Persist a verification run record for debugging and user transparency.
	status := "healthy"
	summary := "Verification: issue resolved"
	if verifyErr != nil {
		status = "error"
		summary = fmt.Sprintf("Verification inconclusive: %v", verifyErr)
	} else if !verified {
		status = "issues_found"
		summary = "Verification: issue still present"
	}

	verifyRecord := PatrolRunRecord{
		ID:                        fmt.Sprintf("%d", startTime.UnixNano()),
		StartedAt:                 startTime,
		CompletedAt:               endTime,
		Duration:                  duration,
		DurationMs:                duration.Milliseconds(),
		Type:                      "verification",
		TriggerReason:             string(TriggerReasonVerification),
		ScopeResourceIDs:          []string{resourceID},
		EffectiveScopeResourceIDs: []string{resourceID},
		ScopeResourceTypes:        []string{resourceType},
		ScopeContext:              fmt.Sprintf("Verifying fix for finding: %s", findingID),
		FindingID:                 findingID,
		ResourcesChecked:          1,
		NewFindings:               0,
		FindingsSummary:           summary,
		Status:                    status,
	}
	if strings.TrimSpace(resourceID) == "" {
		verifyRecord.ScopeResourceIDs = nil
		verifyRecord.EffectiveScopeResourceIDs = nil
		verifyRecord.ResourcesChecked = 0
	}
	if strings.TrimSpace(resourceType) == "" {
		verifyRecord.ScopeResourceTypes = nil
	}
	if verifyErr != nil {
		verifyRecord.ErrorCount = 1
	}
	if p.runHistoryStore != nil {
		p.runHistoryStore.Add(verifyRecord)
	}

	p.mu.Lock()
	p.lastActivity = endTime
	p.lastDuration = duration
	p.resourcesChecked = verifyRecord.ResourcesChecked
	p.errorCount = verifyRecord.ErrorCount
	p.mu.Unlock()

	return verified, verifyErr
}

func (p *PatrolService) verifyFixDeterministically(
	ctx context.Context,
	finding *Finding,
	resourceID, resourceType, findingKey, findingID string,
) (bool, error) {
	key := normalizeFindingKey(findingKey)
	if key == "" {
		return false, fmt.Errorf("%w: missing finding key", aicontracts.ErrVerificationUnknown)
	}

	// State-only verifiers (no tools required).
	fullState := p.currentPatrolRuntimeState()
	if resolved, handled, err := verifyAPTWorkflowFinding(fullState, key, resourceID, time.Now()); handled {
		return resolved, err
	}
	switch key {
	case "backup-stale":
		ok, err := verifyBackupFreshState(fullState, resourceID)
		if err != nil {
			return false, err
		}
		return ok, nil
	case "cpu-high", "memory-high", "disk-high":
		ok, err := verifyMetricRecoveredState(fullState, p.thresholds, key, resourceID, resourceType)
		if err != nil {
			return false, err
		}
		return ok, nil
	case "guest-unreachable":
		ok, err := p.verifyGuestReachabilityState(ctx, fullState, resourceID)
		if err != nil {
			return false, err
		}
		return ok, nil
	}

	// Tool-based verifiers (deterministic tool calls + deterministic signal parsing).
	executor, execErr := p.getExecutorForVerification()
	if execErr != nil {
		return false, execErr
	}

	p.mu.RLock()
	sigThresholds := SignalThresholdsFromPatrol(p.thresholds)
	p.mu.RUnlock()

	switch key {
	case "smart-failure":
		node := strings.TrimSpace(resourceID)
		device := ""
		if finding != nil {
			device = strings.TrimSpace(finding.ResourceName)
		}
		return p.verifyBySignals(ctx, executor, sigThresholds, key, node, device)
	case "backup-failed":
		guestID := strings.TrimSpace(resourceID)
		return p.verifyBySignals(ctx, executor, sigThresholds, key, guestID, "")
	default:
		return false, fmt.Errorf("%w: no deterministic verifier for key=%q (finding_id=%s)", aicontracts.ErrVerificationUnknown, key, findingID)
	}
}

// hasDeterministicVerifierForKey reports whether a deterministic
// verifier exists for the given finding key. It is the single source of
// truth consulted by both the LLM-resolve gate (ResolveFinding) and the
// verified stale-finding reconcile pass (reconcileStaleFindings), and it
// must stay aligned with the dispatch switch in verifyFixDeterministically.
// It previously listed only smart-failure and backup-failed despite the
// dispatch handling seven keys, which made the resolve gate silently skip
// verification that existed (trust-the-LLM fallback) for backup-stale and
// guest-unreachable findings.
func (p *PatrolService) hasDeterministicVerifierForKey(key string) bool {
	switch normalizeFindingKey(key) {
	case "backup-stale", "cpu-high", "memory-high", "disk-high",
		"guest-unreachable", "smart-failure", "backup-failed":
		return true
	default:
		return false
	}
}

func (p *PatrolService) getExecutorForVerification() (*tools.PulseToolExecutor, error) {
	if p == nil || p.aiService == nil {
		return nil, fmt.Errorf("%w: AI service unavailable", aicontracts.ErrVerificationUnknown)
	}
	cs := p.aiService.GetChatService()
	if cs == nil {
		return nil, fmt.Errorf("%w: chat service unavailable", aicontracts.ErrVerificationUnknown)
	}
	executorAccessor, ok := cs.(chatServiceExecutorAccessor)
	if !ok {
		return nil, fmt.Errorf("%w: chat service does not expose tool executor", aicontracts.ErrVerificationUnknown)
	}
	exec := executorAccessor.GetExecutor()
	if exec == nil {
		return nil, fmt.Errorf("%w: tool executor unavailable", aicontracts.ErrVerificationUnknown)
	}
	return exec, nil
}

func (p *PatrolService) verifyBySignals(
	ctx context.Context,
	executor *tools.PulseToolExecutor,
	thresholds SignalThresholds,
	findingKey string,
	resourceID string,
	resourceName string,
) (bool, error) {
	if executor == nil {
		return false, fmt.Errorf("%w: tool executor unavailable", aicontracts.ErrVerificationUnknown)
	}

	var toolName string
	args := map[string]interface{}{}
	switch findingKey {
	case "smart-failure":
		toolName = "pulse_storage"
		args = map[string]interface{}{"type": "disk_health"}
		if strings.TrimSpace(resourceID) != "" {
			args["node"] = resourceID
		}
	case "backup-failed":
		toolName = "pulse_storage"
		args = map[string]interface{}{"type": "backup_tasks"}
		if strings.TrimSpace(resourceID) != "" {
			args["guest_id"] = resourceID
		}
	default:
		return false, fmt.Errorf("%w: unhandled signal verifier key=%q", aicontracts.ErrVerificationUnknown, findingKey)
	}

	tc, err := executeToolCall(ctx, executor, toolName, args)
	if err != nil {
		return false, err
	}

	signals := DetectSignals([]ToolCallRecord{tc}, thresholds)
	persisting := false
	for _, s := range signals {
		switch findingKey {
		case "smart-failure":
			if s.SignalType == SignalSMARTFailure {
				if resourceName == "" || strings.TrimSpace(strings.ToLower(s.ResourceName)) == strings.TrimSpace(strings.ToLower(resourceName)) {
					persisting = true
				}
			}
		case "backup-failed":
			if s.SignalType == SignalBackupFailed && (resourceID == "" || s.ResourceID == resourceID) {
				persisting = true
			}
		}
	}
	if persisting {
		return false, nil
	}
	return true, nil
}

func executeToolCall(ctx context.Context, executor *tools.PulseToolExecutor, toolName string, args map[string]interface{}) (ToolCallRecord, error) {
	if executor == nil {
		return ToolCallRecord{}, fmt.Errorf("%w: tool executor unavailable", aicontracts.ErrVerificationUnknown)
	}
	if toolName == "" {
		return ToolCallRecord{}, fmt.Errorf("%w: missing tool name", aicontracts.ErrVerificationUnknown)
	}
	if args == nil {
		args = map[string]interface{}{}
	}
	inputBytes, _ := json.Marshal(args)
	inputStr := string(inputBytes)
	start := time.Now().UnixMilli()

	result, execErr := executor.ExecuteTool(ctx, toolName, args)
	output := ""
	success := false
	var interpreted agentcapabilities.ToolResultInterpretation
	if execErr != nil {
		output = execErr.Error()
	} else {
		interpreted = agentcapabilities.InterpretToolResult(result)
		output = interpreted.Text
		success = !interpreted.IsError && !interpreted.ApprovalRequired && !interpreted.PolicyBlocked
	}
	end := time.Now().UnixMilli()

	if execErr != nil {
		return ToolCallRecord{}, fmt.Errorf("%w: tool execution failed (%s): %v", aicontracts.ErrVerificationUnknown, toolName, execErr)
	}
	if interpreted.IsError {
		return ToolCallRecord{}, fmt.Errorf("%w: tool returned error (%s): %s", aicontracts.ErrVerificationUnknown, toolName, output)
	}
	if interpreted.ApprovalRequired {
		return ToolCallRecord{}, fmt.Errorf("%w: tool requires approval (%s): %s", aicontracts.ErrVerificationUnknown, toolName, output)
	}
	if interpreted.PolicyBlocked {
		return ToolCallRecord{}, fmt.Errorf("%w: tool blocked by security policy (%s): %s", aicontracts.ErrVerificationUnknown, toolName, output)
	}
	// Most verification probes rely on parsing structured JSON outputs. If we receive
	// non-JSON text, treat verification as inconclusive rather than "resolved".
	if toolName == "pulse_storage" || toolName == "pulse_metrics" || toolName == "pulse_alerts" {
		if !isValidJSON(output) {
			return ToolCallRecord{}, fmt.Errorf("%w: tool returned non-JSON output (%s)", aicontracts.ErrVerificationUnknown, toolName)
		}
	}

	return ToolCallRecord{
		ID:        fmt.Sprintf("verify-%d", time.Now().UnixNano()),
		ToolName:  toolName,
		Input:     truncateString(inputStr, MaxToolInputSize),
		Output:    output,
		Success:   success,
		StartTime: start,
		EndTime:   end,
		Duration:  end - start,
	}, nil
}

func isValidJSON(s string) bool {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return false
	}
	if trimmed[0] != '{' && trimmed[0] != '[' {
		return false
	}
	var v interface{}
	return json.Unmarshal([]byte(trimmed), &v) == nil
}

func verifyBackupFreshState(snap patrolRuntimeState, guestID string) (bool, error) {
	vmID := strings.TrimSpace(guestID)
	if vmID == "" {
		return false, fmt.Errorf("%w: missing guest id", aicontracts.ErrVerificationUnknown)
	}

	now := time.Now()
	details, ok := patrolLookupGuestRuntimeDetails(snap, vmID)
	if !ok || details.lastBackup.IsZero() {
		// If the guest cannot be found, verification can't be concluded deterministically.
		return false, fmt.Errorf("%w: guest not found for backup verification (%s)", aicontracts.ErrVerificationUnknown, vmID)
	}

	if now.Sub(details.lastBackup) <= 48*time.Hour {
		return true, nil
	}
	return false, nil
}

func verifyMetricRecoveredState(snap patrolRuntimeState, thresholds PatrolThresholds, key, resourceID, resourceType string) (bool, error) {
	rid := strings.TrimSpace(resourceID)
	if rid == "" {
		return false, fmt.Errorf("%w: missing resource id", aicontracts.ErrVerificationUnknown)
	}

	// Use a small margin to avoid flapping around exact thresholds.
	const margin = 0.95
	metrics, ok := patrolLookupResourceMetricsForType(snap, rid, resourceType)
	if ok {
		switch key {
		case "cpu-high":
			if value, exists := metrics["cpu"]; exists {
				return value < thresholds.NodeCPUWarning*margin, nil
			}
		case "memory-high":
			value, exists := metrics["memory"]
			if !exists {
				break
			}
			if resourceType == "node" {
				return value < thresholds.NodeMemWarning*margin, nil
			}
			return value < thresholds.GuestMemWarning*margin, nil
		case "disk-high":
			if resourceType == "physical_disk" {
				if disk, exists := patrolLookupPhysicalDiskVerificationState(snap, rid); exists {
					if disk.health != "" && !strings.EqualFold(disk.health, "PASSED") && !strings.EqualFold(disk.health, "UNKNOWN") && !strings.EqualFold(disk.health, "OK") {
						return false, nil
					}
					if disk.wearout >= 0 && disk.wearout < 20 {
						return false, nil
					}
					if disk.temperature > 55 {
						return false, nil
					}
					return true, nil
				}
				break
			}
			if resourceType == "storage" {
				if value, exists := metrics["usage"]; exists {
					return value < thresholds.StorageWarning*margin, nil
				}
				break
			}
			if value, exists := metrics["disk"]; exists {
				return value < thresholds.GuestDiskWarn*margin, nil
			}
		}
	}

	// If we can't locate the resource, verification is inconclusive.
	return false, fmt.Errorf("%w: resource not found for metric verification (%s)", aicontracts.ErrVerificationUnknown, rid)
}

type patrolPhysicalDiskVerification struct {
	health      string
	wearout     int
	temperature int
}

type patrolPhysicalDiskVisitor func(identifiers []string, verification patrolPhysicalDiskVerification) bool

func patrolLookupPhysicalDiskVerificationState(snap patrolRuntimeState, resourceID string) (patrolPhysicalDiskVerification, bool) {
	return patrolLookupPhysicalDiskVerificationWithVisitor(resourceID, func(visit patrolPhysicalDiskVisitor) bool {
		return patrolVisitPhysicalDiskVerification(snap, visit)
	})
}

func (p *PatrolService) verifyGuestReachabilityState(ctx context.Context, snap patrolRuntimeState, guestID string) (bool, error) {
	p.mu.RLock()
	prober := p.guestProber
	p.mu.RUnlock()
	if prober == nil {
		return false, fmt.Errorf("%w: guest prober not configured", aicontracts.ErrVerificationUnknown)
	}

	vmID := strings.TrimSpace(guestID)
	if vmID == "" {
		return false, fmt.Errorf("%w: missing guest id", aicontracts.ErrVerificationUnknown)
	}

	details, ok := patrolLookupGuestRuntimeDetails(snap, vmID)
	if !ok || details.node == "" || details.ip == "" {
		return false, fmt.Errorf("%w: missing node/ip for guest reachability verification (guest=%s)", aicontracts.ErrVerificationUnknown, vmID)
	}

	agentID, ok := prober.GetAgentForHost(details.node)
	if !ok || strings.TrimSpace(agentID) == "" {
		return false, fmt.Errorf("%w: no agent available for host %s", aicontracts.ErrVerificationUnknown, details.node)
	}

	results, err := prober.PingGuests(ctx, agentID, []string{details.ip})
	if err != nil {
		return false, fmt.Errorf("%w: reachability probe failed: %v", aicontracts.ErrVerificationUnknown, err)
	}
	if res, ok := results[details.ip]; ok {
		if res.Reachable {
			return true, nil
		}
		return false, nil
	}
	return false, fmt.Errorf("%w: missing ping result for %s", aicontracts.ErrVerificationUnknown, details.ip)
}

type patrolGuestRuntimeDetails struct {
	lastBackup time.Time
	node       string
	ip         string
}

type patrolGuestRuntimeDetailsVisitor func(identifiers []string, details patrolGuestRuntimeDetails) bool

type patrolMetricVisitor func(identifiers []string, metrics map[string]float64) bool

func patrolActionabilityResourceMetrics(snap patrolRuntimeState) (map[string]map[string]float64, bool) {
	resourceMetrics := make(map[string]map[string]float64)
	hasInventory := patrolVisitMetrics(snap, func(identifiers []string, metrics map[string]float64) bool {
		patrolRegisterResourceMetrics(resourceMetrics, metrics, identifiers...)
		return true
	})
	return patrolAugmentActionabilityMetricsWithPhysicalDisks(resourceMetrics, snap), hasInventory
}

func patrolAugmentActionabilityMetricsWithPhysicalDisks(dest map[string]map[string]float64, snap patrolRuntimeState) map[string]map[string]float64 {
	if dest == nil {
		dest = make(map[string]map[string]float64)
	}
	for _, disk := range patrolPhysicalDiskRows(snap, nil) {
		patrolRegisterResourceMetrics(dest, map[string]float64{}, disk.id, disk.name, disk.devPath, disk.model)
	}
	return dest
}

func patrolLookupGuestRuntimeDetails(snap patrolRuntimeState, guestID string) (patrolGuestRuntimeDetails, bool) {
	return patrolLookupGuestRuntimeDetailsWithVisitor(guestID, func(visit patrolGuestRuntimeDetailsVisitor) bool {
		return patrolVisitGuestRuntimeDetails(snap, visit)
	})
}

func patrolLookupResourceMetrics(snap patrolRuntimeState, resourceID string) (map[string]float64, bool) {
	return patrolLookupMetricsWithVisitor(resourceID, func(visit patrolMetricVisitor) bool {
		return patrolVisitMetrics(snap, visit)
	})
}

func patrolLookupResourceMetricsForType(snap patrolRuntimeState, resourceID, resourceType string) (map[string]float64, bool) {
	switch strings.ToLower(strings.TrimSpace(resourceType)) {
	case "node", "agent":
		return patrolLookupMetricsWithVisitor(resourceID, func(visit patrolMetricVisitor) bool {
			return patrolVisitNodeMetrics(snap, visit)
		})
	case "vm":
		return patrolLookupMetricsWithVisitor(resourceID, func(visit patrolMetricVisitor) bool {
			return patrolVisitGuestMetrics(snap, "VM", visit)
		})
	case "container", "system-container":
		return patrolLookupMetricsWithVisitor(resourceID, func(visit patrolMetricVisitor) bool {
			return patrolVisitGuestMetrics(snap, "Container", visit)
		})
	case "storage":
		return patrolLookupMetricsWithVisitor(resourceID, func(visit patrolMetricVisitor) bool {
			return patrolVisitStorageMetrics(snap, visit)
		})
	case "physical_disk":
		if metrics, ok := patrolLookupPhysicalDiskMetricsState(snap, resourceID); ok {
			return metrics, true
		}
		return nil, false
	default:
		return patrolLookupResourceMetrics(snap, resourceID)
	}
}

func patrolLookupPhysicalDiskMetricsState(snap patrolRuntimeState, resourceID string) (map[string]float64, bool) {
	if _, ok := patrolLookupPhysicalDiskVerificationState(snap, resourceID); ok {
		return map[string]float64{}, true
	}
	return nil, false
}

func patrolLookupPhysicalDiskVerificationWithVisitor(resourceID string, walk func(patrolPhysicalDiskVisitor) bool) (patrolPhysicalDiskVerification, bool) {
	found := false
	var result patrolPhysicalDiskVerification
	walk(func(identifiers []string, verification patrolPhysicalDiskVerification) bool {
		for _, identifier := range identifiers {
			if strings.TrimSpace(identifier) != strings.TrimSpace(resourceID) {
				continue
			}
			result = verification
			found = true
			return false
		}
		return true
	})
	return result, found
}

func patrolVisitPhysicalDiskVerification(snap patrolRuntimeState, visit patrolPhysicalDiskVisitor) bool {
	rows := patrolPhysicalDiskRows(snap, nil)
	for _, disk := range rows {
		if !visit([]string{disk.id, disk.name, disk.devPath, disk.model}, patrolPhysicalDiskVerification{
			health:      strings.TrimSpace(disk.health),
			wearout:     disk.wearout,
			temperature: disk.temperature,
		}) {
			return true
		}
	}
	return len(rows) > 0
}

func patrolLookupGuestRuntimeDetailsWithVisitor(guestID string, walk func(patrolGuestRuntimeDetailsVisitor) bool) (patrolGuestRuntimeDetails, bool) {
	found := false
	var result patrolGuestRuntimeDetails
	walk(func(identifiers []string, details patrolGuestRuntimeDetails) bool {
		for _, identifier := range identifiers {
			if strings.TrimSpace(identifier) != strings.TrimSpace(guestID) {
				continue
			}
			result = details
			found = true
			return false
		}
		return true
	})
	return result, found
}

func patrolVisitGuestRuntimeDetails(snap patrolRuntimeState, visit patrolGuestRuntimeDetailsVisitor) bool {
	rows := patrolGuestInventoryRows(snap, nil, nil)
	for _, guest := range rows {
		identifiers := []string{guest.id, guest.name}
		if guest.vmid > 0 {
			identifiers = append(identifiers, fmt.Sprintf("%d", guest.vmid))
		}
		if !visit(identifiers, patrolGuestRuntimeDetails{
			lastBackup: guest.lastBackup,
			node:       guest.node,
			ip:         guest.ip,
		}) {
			return true
		}
	}
	return len(rows) > 0
}

func patrolLookupMetricsWithVisitor(resourceID string, walk func(patrolMetricVisitor) bool) (map[string]float64, bool) {
	found := false
	var result map[string]float64
	walk(func(identifiers []string, metrics map[string]float64) bool {
		for _, identifier := range identifiers {
			if strings.TrimSpace(identifier) != strings.TrimSpace(resourceID) {
				continue
			}
			result = metrics
			found = true
			return false
		}
		return true
	})
	return result, found
}

func patrolVisitMetrics(snap patrolRuntimeState, visit patrolMetricVisitor) bool {
	hasInventory := false
	for _, walk := range []func(patrolRuntimeState, patrolMetricVisitor) bool{
		patrolVisitNodeMetrics,
		func(s patrolRuntimeState, v patrolMetricVisitor) bool { return patrolVisitGuestMetrics(s, "VM", v) },
		func(s patrolRuntimeState, v patrolMetricVisitor) bool {
			return patrolVisitGuestMetrics(s, "Container", v)
		},
		patrolVisitStorageMetrics,
	} {
		if walk(snap, visit) {
			hasInventory = true
		}
	}
	return hasInventory
}

func patrolVisitNodeMetrics(snap patrolRuntimeState, visit patrolMetricVisitor) bool {
	rows := patrolNodeInventoryRows(snap, nil)
	for _, node := range rows {
		metrics := map[string]float64{"cpu": node.cpu}
		if node.mem > 0 {
			metrics["memory"] = node.mem
		}
		if !visit([]string{node.id, node.name}, metrics) {
			return true
		}
	}
	return len(rows) > 0
}

func patrolVisitGuestMetrics(snap patrolRuntimeState, guestType string, visit patrolMetricVisitor) bool {
	rows := patrolGuestInventoryRows(snap, nil, nil)
	count := 0
	for _, guest := range rows {
		if guest.gType != guestType {
			continue
		}
		count++
		if !visit([]string{guest.id, guest.name}, map[string]float64{
			"cpu":    guest.cpu,
			"memory": guest.mem,
			"disk":   guest.disk,
		}) {
			return true
		}
	}
	return count > 0
}

func patrolVisitStorageMetrics(snap patrolRuntimeState, visit patrolMetricVisitor) bool {
	rows := patrolStoragePoolRows(snap, nil)
	for _, storage := range rows {
		if !visit([]string{storage.id, storage.name}, map[string]float64{"usage": storage.usage}) {
			return true
		}
	}
	return len(rows) > 0
}

func patrolRegisterResourceMetrics(dest map[string]map[string]float64, metrics map[string]float64, identifiers ...string) {
	for _, identifier := range identifiers {
		identifier = strings.TrimSpace(identifier)
		if identifier == "" {
			continue
		}
		dest[identifier] = metrics
	}
}

func patrolFirstIP(ips []string) string {
	if len(ips) == 0 {
		return ""
	}
	return ips[0]
}
