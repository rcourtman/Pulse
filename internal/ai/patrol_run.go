// patrol_run.go implements the PatrolService runtime: Start/Stop lifecycle,
// the main patrol loop, scoped patrol execution, alert auto-resolution,
// live streaming to UI subscribers, and run history tracking.
package ai

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/circuit"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rs/zerolog/log"
)

// Patrol run lifecycle constants.
const (
	initialPatrolStartDelay   = 30 * time.Second // Delay before first patrol after startup
	findingCleanupAge         = 24 * time.Hour   // Resolved findings older than this are purged
	scopedPatrolRetryBackoff1 = 5 * time.Second  // First retry backoff for dropped scoped patrols
	scopedPatrolRetryBackoff2 = 15 * time.Second // Second retry backoff for dropped scoped patrols
	scopedPatrolMaxRetries    = 2                // Maximum re-queue attempts for dropped scoped patrols
)

// Start begins the background patrol loop
func (p *PatrolService) Start(ctx context.Context) {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return
	}
	p.running = true
	p.stopCh = make(chan struct{})
	p.configChanged = make(chan struct{}, 1) // Buffered to allow non-blocking send
	p.mu.Unlock()

	log.Info().
		Dur("interval", p.config.GetInterval()).
		Msg("Starting AI Patrol Service")

	go p.patrolLoop(ctx)
}

// Stop stops the patrol service. It signals the patrol loop to exit, then
// waits up to 15 seconds for in-flight investigations to finish and
// force-saves findings/investigation state to disk.
func (p *PatrolService) Stop() {
	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		return
	}
	p.running = false
	close(p.stopCh)
	orchestrator := p.investigationOrchestrator
	findings := p.findings
	p.mu.Unlock()

	log.Info().Msg("stopping AI Patrol Service")

	// Give investigations 15 seconds to finish (leaves headroom within server's 30s budget)
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()

	// Signal orchestrator to cancel running investigations and persist state
	if orchestrator != nil {
		if err := orchestrator.Shutdown(shutdownCtx); err != nil {
			log.Warn().Err(err).Msg("AI Patrol: Investigation orchestrator shutdown returned error")
		}
	}

	// Wait for investigation goroutines tracked by PatrolService
	done := make(chan struct{})
	go func() {
		p.investigationWg.Wait()
		close(done)
	}()
	select {
	case <-done:
		// All investigation goroutines finished
	case <-shutdownCtx.Done():
		log.Warn().Msg("AI Patrol: Timed out waiting for investigation goroutines to finish")
	}

	// Force-save findings store
	if findings != nil {
		if err := findings.ForceSave(); err != nil {
			log.Error().Err(err).Msg("AI Patrol: Failed to force-save findings during shutdown")
		}
	}

	log.Info().Msg("AI Patrol Service stopped")
}

// patrolLoop is the main background loop
func (p *PatrolService) patrolLoop(ctx context.Context) {
	// Seed lastPatrol from persisted run history so the API can return
	// last_patrol_at immediately (before the first in-process patrol completes).
	if history := p.GetRunHistory(1); len(history) > 0 && !history[0].CompletedAt.IsZero() {
		p.mu.Lock()
		p.lastPatrol = history[0].CompletedAt
		p.mu.Unlock()
	}

	// Run initial patrol shortly after startup, but only if one hasn't run recently
	initialDelay := initialPatrolStartDelay
	initialTimer := time.NewTimer(initialDelay)
	defer initialTimer.Stop()

	select {
	case <-initialTimer.C:
		// Check if a patrol ran recently (within last hour) to avoid wasting tokens on restarts
		runHistory := p.GetRunHistory(1)

		skipInitial := false
		if len(runHistory) > 0 {
			lastRun := runHistory[0]
			timeSinceLastRun := time.Since(lastRun.CompletedAt)
			if timeSinceLastRun < 1*time.Hour {
				log.Info().
					Dur("time_since_last", timeSinceLastRun).
					Msg("AI Patrol: Skipping initial patrol - recent run exists")
				skipInitial = true
			}
		}

		if !skipInitial {
			p.runPatrolWithTrigger(ctx, TriggerReasonStartup, nil)
		}
	case <-p.stopCh:
		if !initialTimer.Stop() {
			select {
			case <-initialTimer.C:
			default:
			}
		}
		return
	case <-ctx.Done():
		if !initialTimer.Stop() {
			select {
			case <-initialTimer.C:
			default:
			}
		}
		return
	}

	p.mu.RLock()
	interval := p.config.GetInterval()
	configCh := p.configChanged
	p.mu.RUnlock()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	p.mu.Lock()
	p.nextScheduledAt = time.Now().Add(interval)
	p.mu.Unlock()

	for {
		select {
		case <-ticker.C:
			// Update next scheduled time before the run starts — time.Now() closely
			// matches the tick time here, and the ticker will fire again at roughly
			// this moment + interval regardless of how long the run takes.
			p.mu.Lock()
			p.nextScheduledAt = time.Now().Add(interval)
			p.mu.Unlock()
			p.runPatrolWithTrigger(ctx, TriggerReasonScheduled, nil)

		case alert := <-p.adHocTrigger:
			// Run immediate targeted patrol for this alert
			log.Info().Str("alert_id", alert.ID).Msg("patrol triggered by alert")
			p.runTargetedPatrol(ctx, alert)

		case <-configCh:
			// Config changed - reset ticker with new interval
			p.mu.RLock()
			newInterval := p.config.GetInterval()
			p.mu.RUnlock()

			if newInterval != interval {
				interval = newInterval
				ticker.Reset(interval)
				p.mu.Lock()
				p.nextScheduledAt = time.Now().Add(interval)
				p.mu.Unlock()
				log.Info().
					Dur("interval", interval).
					Msg("Patrol ticker reset to new interval")
			}

		case <-p.stopCh:
			return

		case <-ctx.Done():
			return
		}
	}
}

// runPatrol executes a scheduled patrol run
func (p *PatrolService) runPatrol(ctx context.Context) {
	p.runPatrolWithTrigger(ctx, TriggerReasonScheduled, nil)
}

// runPatrolWithTrigger executes a patrol run with trigger context
func (p *PatrolService) runPatrolWithTrigger(ctx context.Context, trigger TriggerReason, scope *PatrolScope) {
	p.mu.RLock()
	cfg := p.config
	breaker := p.circuitBreaker
	p.mu.RUnlock()

	if !cfg.Enabled {
		return
	}

	if !p.tryStartRun("full") {
		return
	}
	defer p.endRun()

	// Check if circuit breaker allows LLM calls.
	llmAllowed := breaker == nil || breaker.Allow()
	if !llmAllowed {
		log.Warn().Msg("AI Patrol: Circuit breaker is open (LLM calls blocked)")
	}

	start := time.Now()
	runID := fmt.Sprintf("%d", start.UnixNano())
	patrolType := "patrol"
	GetPatrolMetrics().RecordRun(string(trigger), "full")

	log.Debug().Msg("AI Patrol: Starting patrol run")

	// Track run statistics
	var runStats struct {
		resourceCount     int
		nodesChecked      int
		guestsChecked     int
		dockerChecked     int
		storageChecked    int
		hostsChecked      int
		pbsChecked        int
		pmgChecked        int
		kubernetesChecked int
		newFindings       int
		existingFindings  int
		rejectedFindings  int
		triageFlags       int
		triageSkippedLLM  bool
		findingIDs        []string
		errors            int
		lastAIError       error             // Preserve original error for circuit breaker categorization
		aiAnalysis        *AIAnalysisResult // Stores the AI's analysis for the run record
	}
	var newFindings []*Finding

	// Get current state
	if p.stateProvider == nil {
		log.Warn().Msg("AI Patrol: No state provider available")
		return
	}

	state := p.stateProvider.GetState()

	// Helper to track findings
	// Note: Only warning+ severity findings count toward newFindings since watch/info are filtered from UI
	trackFinding := func(f *Finding) bool {
		isNew := p.recordFinding(f)
		if isNew {
			// Only count warning+ findings as "new" for user-facing stats
			if f.Severity == FindingSeverityWarning || f.Severity == FindingSeverityCritical {
				runStats.newFindings++
				newFindings = append(newFindings, f)
			}
		} else {
			runStats.existingFindings++
		}

		// Only track warning+ severity finding IDs in the run record
		if f.Severity == FindingSeverityWarning || f.Severity == FindingSeverityCritical {
			runStats.findingIDs = append(runStats.findingIDs, f.ID)
		}

		return isNew
	}

	// Count resources for statistics (respect analysis configuration)
	if cfg.AnalyzeNodes {
		runStats.nodesChecked = len(state.Nodes)
	}
	if cfg.AnalyzeGuests {
		runStats.guestsChecked = len(state.VMs) + len(state.Containers)
	}
	if cfg.AnalyzeDocker {
		runStats.dockerChecked = len(state.DockerHosts)
	}
	if cfg.AnalyzeStorage {
		// Prefer the canonical resource read surface.
		if p.readState != nil {
			runStats.storageChecked = len(p.readState.StoragePools())
		} else {
			runStats.storageChecked = 0
		}
	}
	if cfg.AnalyzePBS {
		runStats.pbsChecked = len(state.PBSInstances)
	}
	if cfg.AnalyzePMG {
		runStats.pmgChecked = len(state.PMGInstances)
	}
	if cfg.AnalyzeHosts {
		runStats.hostsChecked = len(state.Hosts)
	}
	if cfg.AnalyzeKubernetes {
		runStats.kubernetesChecked = len(state.KubernetesClusters)
	}
	runStats.resourceCount = runStats.nodesChecked + runStats.guestsChecked +
		runStats.dockerChecked + runStats.storageChecked + runStats.pbsChecked + runStats.pmgChecked + runStats.hostsChecked +
		runStats.kubernetesChecked

	// Determine if we can run LLM analysis (requires AI service + circuit breaker not open)
	aiServiceEnabled := p.aiService != nil && p.aiService.IsEnabled()
	canRunLLM := aiServiceEnabled && llmAllowed

	// Check if we can run LLM analysis (AI-only patrol)
	if !canRunLLM {
		reason := "AI not configured - set up a provider in Settings > Pulse Assistant"
		if !aiServiceEnabled {
			reason = "AI not configured - set up a provider in Settings > Pulse Assistant"
		} else if !llmAllowed {
			reason = "circuit breaker is open"
			GetPatrolMetrics().RecordCircuitBlock()
		}
		p.setBlockedReason(reason)
		log.Info().Str("reason", reason).Msg("AI Patrol: Skipping run - AI unavailable")
		return
	} else {
		p.clearBlockedReason()
		// Ensure stream state is clean for this run before the first streamed event.
		p.resetStreamForRun(runID)
		// Run agentic AI analysis — the LLM uses tools to investigate and reports findings
		aiResult, aiErr := p.runAIAnalysis(ctx, state, scope)
		if aiErr != nil {
			log.Warn().Err(aiErr).Msg("AI Patrol: LLM analysis failed")
			runStats.errors++
			runStats.lastAIError = aiErr

			// Create a finding to surface this error to the user
			errMsg := aiErr.Error()
			var title, description, recommendation string
			if strings.Contains(errMsg, "Insufficient Balance") || strings.Contains(errMsg, "402") {
				title = "Pulse Patrol: Insufficient API credits"
				description = "Pulse Patrol cannot analyze your infrastructure because your provider account has insufficient credits."
				recommendation = "Add credits to your provider account (DeepSeek, OpenAI, etc.) or switch to a different provider in Pulse Assistant settings."
			} else if strings.Contains(errMsg, "401") || strings.Contains(errMsg, "Unauthorized") {
				title = "Pulse Patrol: Invalid API key"
				description = "Pulse Patrol cannot analyze your infrastructure because the API key is invalid or expired."
				recommendation = "Check your API key in Pulse Assistant settings and verify it is correct."
			} else if strings.Contains(errMsg, "rate limit") || strings.Contains(errMsg, "429") {
				title = "Pulse Patrol: Rate limited"
				description = "Pulse Patrol is being rate limited by your provider. Analysis will be retried on the next patrol run."
				recommendation = "Wait for the rate limit to reset, or consider upgrading your API plan for higher limits."
			} else {
				title = "Pulse Patrol: Analysis failed"
				description = fmt.Sprintf("Pulse Patrol encountered an error while analyzing your infrastructure: %s", errMsg)
				recommendation = "Check your Pulse Assistant settings and API key. If the problem persists, check the logs for more details."
			}

			errorFinding := &Finding{
				ID:             generateFindingID("ai-service", "reliability", "ai-patrol-error"),
				Key:            "ai-patrol-error",
				Severity:       "warning",
				Category:       "reliability",
				ResourceID:     "ai-service",
				ResourceName:   "Pulse Patrol Service",
				ResourceType:   "service",
				Title:          title,
				Description:    description,
				Recommendation: recommendation,
				Evidence:       fmt.Sprintf("Error: %s", errMsg),
				DetectedAt:     time.Now(),
				LastSeenAt:     time.Now(),
			}
			trackFinding(errorFinding)
		} else if aiResult != nil {
			runStats.aiAnalysis = aiResult
			runStats.rejectedFindings = aiResult.RejectedFindings
			runStats.triageFlags = aiResult.TriageFlags
			runStats.triageSkippedLLM = aiResult.TriageSkippedLLM

			// Auto-resolve previous patrol error finding if this run succeeded
			errorFindingID := generateFindingID("ai-service", "reliability", "ai-patrol-error")
			if existing := p.findings.Get(errorFindingID); existing != nil && !existing.IsResolved() {
				p.findings.Resolve(errorFindingID, true) // auto-resolved
				if resolver := p.unifiedFindingResolver; resolver != nil {
					resolver(errorFindingID)
				}
				log.Info().Msg("AI Patrol: Auto-resolved previous patrol error finding after successful run")
			}

			// Findings are already recorded via patrol_report_finding tool calls.
			// Track stats from the collected findings.
			for _, f := range aiResult.Findings {
				if f.Severity == FindingSeverityWarning || f.Severity == FindingSeverityCritical {
					runStats.findingIDs = append(runStats.findingIDs, f.ID)
					// Check if this finding was new by looking at the store
					stored := p.findings.Get(f.ID)
					if stored != nil && stored.TimesRaised <= 1 {
						runStats.newFindings++
						newFindings = append(newFindings, f)
					} else {
						runStats.existingFindings++
					}
				}
			}
		}
	}

	// Count resolved findings: LLM-resolved (via tool) + auto-reconciled stale findings.
	var resolvedCount int
	if runStats.aiAnalysis != nil {
		resolvedCount = len(runStats.aiAnalysis.ResolvedIDs)

		// Auto-resolve stale findings: active findings that were presented to the LLM
		// in seed context but were neither re-reported nor explicitly resolved.
		// Only runs after successful full patrols (not scoped).
		autoResolved := p.reconcileStaleFindings(
			runStats.aiAnalysis.ReportedIDs,
			runStats.aiAnalysis.ResolvedIDs,
			runStats.aiAnalysis.SeededFindingIDs,
			runStats.errors > 0,
		)
		resolvedCount += autoResolved
		if autoResolved > 0 {
			log.Info().
				Int("auto_resolved", autoResolved).
				Msg("AI Patrol: Auto-resolved stale findings after full patrol")
		}
	}

	// Cleanup old resolved findings (always runs, doesn't require LLM)
	cleaned := p.findings.Cleanup(findingCleanupAge)
	if cleaned > 0 {
		log.Debug().Int("cleaned", cleaned).Msg("AI Patrol: Cleaned up old findings")
	}

	// Recover investigations stuck in "running" state (goroutine panicked or was killed)
	p.recoverStuckInvestigations()

	// Retry investigations that failed due to timeout (shorter cooldown than permanent failures)
	p.retryTimedOutInvestigations()

	// AI-based alert review: check active alerts against current state and auto-resolve fixed issues
	// Pass llmAllowed so it knows whether AI calls are allowed.
	alertsResolved := p.reviewAndResolveAlerts(ctx, state, llmAllowed)
	if alertsResolved > 0 {
		log.Info().Int("alerts_resolved", alertsResolved).Msg("AI Patrol: Auto-resolved alerts where issues are fixed")
	}

	duration := time.Since(start)
	completedAt := time.Now()

	// Build findings summary string
	summary := p.findings.GetSummary()
	var findingsSummaryStr string
	var status string
	// Only count critical and warning as active issues (watch/info are filtered from UI)
	totalActive := summary.Critical + summary.Warning
	if totalActive == 0 {
		findingsSummaryStr = "All healthy"
		status = "healthy"
	} else {
		parts := []string{}
		if summary.Critical > 0 {
			parts = append(parts, fmt.Sprintf("%d critical", summary.Critical))
		}
		if summary.Warning > 0 {
			parts = append(parts, fmt.Sprintf("%d warning", summary.Warning))
		}
		findingsSummaryStr = fmt.Sprintf("%s", joinParts(parts))
		if summary.Critical > 0 {
			status = "critical"
		} else {
			status = "issues_found"
		}
	}
	if runStats.errors > 0 {
		status = "error"
		// Don't claim "All healthy" if there were errors - the patrol didn't complete properly
		if findingsSummaryStr == "All healthy" {
			findingsSummaryStr = fmt.Sprintf("Analysis incomplete (%d errors)", runStats.errors)
		}
	}

	// Create run record
	runRecord := PatrolRunRecord{
		ID:                runID,
		StartedAt:         start,
		CompletedAt:       completedAt,
		Duration:          duration,
		DurationMs:        duration.Milliseconds(),
		Type:              patrolType,
		TriggerReason:     string(trigger),
		ResourcesChecked:  runStats.resourceCount,
		NodesChecked:      runStats.nodesChecked,
		GuestsChecked:     runStats.guestsChecked,
		DockerChecked:     runStats.dockerChecked,
		StorageChecked:    runStats.storageChecked,
		HostsChecked:      runStats.hostsChecked,
		PBSChecked:        runStats.pbsChecked,
		PMGChecked:        runStats.pmgChecked,
		KubernetesChecked: runStats.kubernetesChecked,
		NewFindings:       runStats.newFindings,
		ExistingFindings:  runStats.existingFindings,
		RejectedFindings:  runStats.rejectedFindings,
		ResolvedFindings:  resolvedCount,
		AutoFixCount:      0,
		FindingsSummary:   findingsSummaryStr,
		FindingIDs:        runStats.findingIDs,
		ErrorCount:        runStats.errors,
		Status:            status,
	}

	if scope != nil {
		runRecord.ScopeResourceIDs = scope.ResourceIDs
		runRecord.ScopeResourceTypes = scope.ResourceTypes
		runRecord.ScopeContext = scope.Context
		runRecord.AlertID = scope.AlertID
		runRecord.FindingID = scope.FindingID
	}

	// Add AI analysis details if available
	if runStats.aiAnalysis != nil {
		runRecord.AIAnalysis = runStats.aiAnalysis.Response
		runRecord.InputTokens = runStats.aiAnalysis.InputTokens
		runRecord.OutputTokens = runStats.aiAnalysis.OutputTokens
		runRecord.TriageFlags = runStats.triageFlags
		runRecord.TriageSkippedLLM = runStats.triageSkippedLLM
		toolCalls := runStats.aiAnalysis.ToolCalls
		if len(toolCalls) > MaxToolCallsPerRun {
			toolCalls = toolCalls[:MaxToolCallsPerRun]
		}
		runRecord.ToolCalls = toolCalls
		runRecord.ToolCallCount = len(runStats.aiAnalysis.ToolCalls)
		log.Debug().
			Int("response_length", len(runStats.aiAnalysis.Response)).
			Int("input_tokens", runStats.aiAnalysis.InputTokens).
			Int("output_tokens", runStats.aiAnalysis.OutputTokens).
			Int("tool_calls", runRecord.ToolCallCount).
			Msg("AI Patrol: Storing AI analysis in run record")
	} else {
		log.Debug().Msg("AI Patrol: No AI analysis to store (aiAnalysis is nil)")
	}

	p.mu.Lock()
	p.lastPatrol = completedAt
	p.lastDuration = duration
	p.resourcesChecked = runStats.resourceCount
	p.errorCount = runStats.errors
	p.mu.Unlock()

	// Record circuit breaker result only if we actually attempted LLM calls.
	// canRunLLM is true only when AI is enabled, licensed, AND breaker allowed.
	// Use error categorization so non-transient errors (auth failures, insufficient
	// credits) don't trip the breaker — those won't be fixed by waiting.
	if breaker != nil && canRunLLM {
		if runStats.errors > 0 {
			aiErr := runStats.lastAIError
			if aiErr == nil {
				aiErr = fmt.Errorf("patrol completed with %d errors", runStats.errors)
			}
			breaker.RecordFailureWithCategory(aiErr, circuit.CategorizeError(aiErr))
		} else {
			breaker.RecordSuccess()
		}
	}

	// Add to history store (handles persistence automatically)
	p.runHistoryStore.Add(runRecord)

	log.Info().
		Str("type", patrolType).
		Dur("duration", duration).
		Int("resources", runStats.resourceCount).
		Int("new_findings", runStats.newFindings).
		Int("resolved", resolvedCount).
		Int("critical", summary.Critical).
		Int("warning", summary.Warning).
		Int("watch", summary.Watch).
		Msg("AI Patrol: Completed patrol run")
}

// runScopedPatrol runs a patrol on a filtered subset of resources.
// This provides token-efficient analysis for event-driven patrols.
func (p *PatrolService) runScopedPatrol(ctx context.Context, scope PatrolScope) {
	p.mu.RLock()
	cfg := p.config
	breaker := p.circuitBreaker
	p.mu.RUnlock()

	if !cfg.Enabled {
		return
	}

	if !p.tryStartRun("scoped") {
		// Re-queue with backoff if retries remain
		if scope.RetryCount < scopedPatrolMaxRetries {
			scope.RetryCount++
			backoff := scopedPatrolRetryBackoff1
			if scope.RetryCount == scopedPatrolMaxRetries {
				backoff = scopedPatrolRetryBackoff2
			}
			scope.RetryAfter = time.Now().Add(backoff)
			if tm := p.GetTriggerManager(); tm != nil {
				tm.TriggerPatrol(scope)
				log.Info().
					Int("retry", scope.RetryCount).
					Dur("backoff", backoff).
					Strs("resources", scope.ResourceIDs).
					Msg("AI Patrol: Re-queued dropped scoped patrol with backoff")
			}
		} else {
			GetPatrolMetrics().RecordScopedDroppedFinal()
			log.Error().
				Strs("resources", scope.ResourceIDs).
				Str("reason", string(scope.Reason)).
				Msg("AI Patrol: Scoped patrol permanently dropped after 2 retries")
		}
		return
	}
	defer p.endRun()

	// Check if circuit breaker allows LLM calls.
	llmAllowed := breaker == nil || breaker.Allow()
	if !llmAllowed {
		log.Warn().Msg("AI Patrol: Circuit breaker is open for scoped patrol (LLM calls blocked)")
	}

	start := time.Now()
	runID := fmt.Sprintf("%d", start.UnixNano())
	GetPatrolMetrics().RecordRun(string(scope.Reason), "scoped")
	var runStats struct {
		resourceCount     int
		nodesChecked      int
		guestsChecked     int
		dockerChecked     int
		storageChecked    int
		hostsChecked      int
		pbsChecked        int
		pmgChecked        int
		kubernetesChecked int
		newFindings       int
		existingFindings  int
		rejectedFindings  int
		triageFlags       int
		triageSkippedLLM  bool
		findingIDs        []string
		errors            int
		aiAnalysis        *AIAnalysisResult
	}

	// Get current state
	if p.stateProvider == nil {
		log.Warn().Msg("AI Patrol: No state provider available for scoped patrol")
		return
	}

	fullState := p.stateProvider.GetState()

	// Filter state based on scope
	filteredState := p.filterStateByScope(fullState, scope)

	// Count filtered resources (respect analysis configuration)
	resourceCount := 0
	if cfg.AnalyzeNodes {
		resourceCount += len(filteredState.Nodes)
	}
	if cfg.AnalyzeGuests {
		resourceCount += len(filteredState.VMs) + len(filteredState.Containers)
	}
	if cfg.AnalyzeDocker {
		resourceCount += len(filteredState.DockerHosts)
	}
	if cfg.AnalyzeStorage {
		resourceCount += len(filteredState.Storage)
	}
	if cfg.AnalyzePBS {
		resourceCount += len(filteredState.PBSInstances)
	}
	if cfg.AnalyzeHosts {
		resourceCount += len(filteredState.Hosts)
	}
	if cfg.AnalyzeKubernetes {
		resourceCount += len(filteredState.KubernetesClusters)
	}
	if cfg.AnalyzePMG {
		resourceCount += len(filteredState.PMGInstances)
	}

	if resourceCount == 0 {
		log.Debug().
			Strs("requested_ids", scope.ResourceIDs).
			Strs("requested_types", scope.ResourceTypes).
			Msg("AI Patrol: No resources matched scope filter")
		return
	}

	log.Debug().
		Int("resource_count", resourceCount).
		Str("reason", string(scope.Reason)).
		Msg("AI Patrol: Running scoped analysis")

	// Track run statistics
	if cfg.AnalyzeNodes {
		runStats.nodesChecked = len(filteredState.Nodes)
	}
	if cfg.AnalyzeGuests {
		runStats.guestsChecked = len(filteredState.VMs) + len(filteredState.Containers)
	}
	if cfg.AnalyzeDocker {
		runStats.dockerChecked = len(filteredState.DockerHosts)
	}
	if cfg.AnalyzeStorage {
		runStats.storageChecked = len(filteredState.Storage)
	}
	if cfg.AnalyzePBS {
		runStats.pbsChecked = len(filteredState.PBSInstances)
	}
	if cfg.AnalyzeHosts {
		runStats.hostsChecked = len(filteredState.Hosts)
	}
	if cfg.AnalyzeKubernetes {
		runStats.kubernetesChecked = len(filteredState.KubernetesClusters)
	}
	if cfg.AnalyzePMG {
		runStats.pmgChecked = len(filteredState.PMGInstances)
	}
	runStats.resourceCount = resourceCount

	// Determine if we can run LLM analysis
	aiServiceEnabled := p.aiService != nil && p.aiService.IsEnabled()
	canRunLLM := aiServiceEnabled && llmAllowed

	if !canRunLLM {
		reason := "AI not configured - set up a provider in Settings > Pulse Assistant"
		if !aiServiceEnabled {
			reason = "AI not configured - set up a provider in Settings > Pulse Assistant"
		} else if !llmAllowed {
			reason = "circuit breaker is open"
			GetPatrolMetrics().RecordCircuitBlock()
		}
		p.setBlockedReason(reason)
		log.Info().Str("reason", reason).Msg("AI Patrol: Skipping scoped run - AI unavailable")
		return
	} else {
		p.clearBlockedReason()
		if !scope.NoStream {
			// Ensure stream state is clean for this run before the first streamed event.
			p.resetStreamForRun(runID)
		}
		// Run agentic AI analysis on filtered state with scope
		aiResult, aiErr := p.runAIAnalysis(ctx, filteredState, &scope)
		if aiErr != nil {
			log.Warn().Err(aiErr).Msg("AI Patrol (scoped): LLM analysis failed")
			runStats.errors++
		} else if aiResult != nil {
			runStats.aiAnalysis = aiResult
			runStats.rejectedFindings = aiResult.RejectedFindings
			runStats.triageFlags = aiResult.TriageFlags
			runStats.triageSkippedLLM = aiResult.TriageSkippedLLM
			// Findings are already recorded via patrol_report_finding tool calls.
			for _, f := range aiResult.Findings {
				if f.Severity == FindingSeverityWarning || f.Severity == FindingSeverityCritical {
					runStats.findingIDs = append(runStats.findingIDs, f.ID)
					stored := p.findings.Get(f.ID)
					if stored != nil && stored.TimesRaised <= 1 {
						runStats.newFindings++
					} else {
						runStats.existingFindings++
					}
				}
			}
		}
	}

	duration := time.Since(start)
	completedAt := time.Now()

	// Build findings summary string
	summary := p.findings.GetSummary()
	var findingsSummaryStr string
	var status string
	totalActive := summary.Critical + summary.Warning
	if totalActive == 0 {
		findingsSummaryStr = "All healthy"
		status = "healthy"
	} else {
		parts := []string{}
		if summary.Critical > 0 {
			parts = append(parts, fmt.Sprintf("%d critical", summary.Critical))
		}
		if summary.Warning > 0 {
			parts = append(parts, fmt.Sprintf("%d warning", summary.Warning))
		}
		findingsSummaryStr = fmt.Sprintf("%s", joinParts(parts))
		if summary.Critical > 0 {
			status = "critical"
		} else {
			status = "issues_found"
		}
	}
	if runStats.errors > 0 {
		status = "error"
		if findingsSummaryStr == "All healthy" {
			findingsSummaryStr = fmt.Sprintf("Analysis incomplete (%d errors)", runStats.errors)
		}
	}

	runRecord := PatrolRunRecord{
		ID:                 runID,
		StartedAt:          start,
		CompletedAt:        completedAt,
		Duration:           duration,
		DurationMs:         duration.Milliseconds(),
		Type:               "scoped",
		TriggerReason:      string(scope.Reason),
		ScopeResourceIDs:   scope.ResourceIDs,
		ScopeResourceTypes: scope.ResourceTypes,
		ScopeContext:       scope.Context,
		AlertID:            scope.AlertID,
		FindingID:          scope.FindingID,
		ResourcesChecked:   runStats.resourceCount,
		NodesChecked:       runStats.nodesChecked,
		GuestsChecked:      runStats.guestsChecked,
		DockerChecked:      runStats.dockerChecked,
		StorageChecked:     runStats.storageChecked,
		HostsChecked:       runStats.hostsChecked,
		PBSChecked:         runStats.pbsChecked,
		PMGChecked:         runStats.pmgChecked,
		KubernetesChecked:  runStats.kubernetesChecked,
		NewFindings:        runStats.newFindings,
		ExistingFindings:   runStats.existingFindings,
		RejectedFindings:   runStats.rejectedFindings,
		FindingsSummary:    findingsSummaryStr,
		FindingIDs:         runStats.findingIDs,
		ErrorCount:         runStats.errors,
		Status:             status,
	}

	if runStats.aiAnalysis != nil {
		runRecord.AIAnalysis = runStats.aiAnalysis.Response
		runRecord.InputTokens = runStats.aiAnalysis.InputTokens
		runRecord.OutputTokens = runStats.aiAnalysis.OutputTokens
		runRecord.TriageFlags = runStats.triageFlags
		runRecord.TriageSkippedLLM = runStats.triageSkippedLLM
		toolCalls := runStats.aiAnalysis.ToolCalls
		if len(toolCalls) > MaxToolCallsPerRun {
			toolCalls = toolCalls[:MaxToolCallsPerRun]
		}
		runRecord.ToolCalls = toolCalls
		runRecord.ToolCallCount = len(runStats.aiAnalysis.ToolCalls)
	}

	p.mu.Lock()
	p.lastPatrol = completedAt
	p.lastDuration = duration
	p.resourcesChecked = runStats.resourceCount
	p.errorCount = runStats.errors
	p.mu.Unlock()

	p.runHistoryStore.Add(runRecord)

	log.Info().
		Dur("duration", duration).
		Int("resources", resourceCount).
		Str("reason", string(scope.Reason)).
		Msg("AI Patrol: Scoped patrol complete")
}

// filterStateByScope filters a StateSnapshot to only include resources matching the scope.
func (p *PatrolService) filterStateByScope(state models.StateSnapshot, scope PatrolScope) models.StateSnapshot {
	// Build lookup sets for efficient matching
	resourceIDSet := make(map[string]bool)
	for _, id := range scope.ResourceIDs {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" {
			continue
		}
		resourceIDSet[trimmed] = true
	}

	typeSet := make(map[string]bool)
	addScopeType := func(t string) {
		trimmed := strings.TrimSpace(strings.ToLower(t))
		if trimmed == "" {
			return
		}
		typeSet[trimmed] = true
		switch trimmed {
		case "docker", "docker_host", "docker_container":
			typeSet["docker"] = true
			typeSet["docker_host"] = true
			typeSet["docker_container"] = true
		case "k8s", "kubernetes", "kubernetes_cluster":
			typeSet["k8s"] = true
			typeSet["kubernetes"] = true
			typeSet["kubernetes_cluster"] = true
		case "lxc", "container":
			typeSet["lxc"] = true
			typeSet["container"] = true
		case "vm", "qemu":
			typeSet["vm"] = true
			typeSet["qemu"] = true
		case "host", "host_raid", "host_sensor":
			typeSet["host"] = true
			typeSet["host_raid"] = true
			typeSet["host_sensor"] = true
		case "pbs", "pbs_datastore", "pbs_job":
			typeSet["pbs"] = true
			typeSet["pbs_datastore"] = true
			typeSet["pbs_job"] = true
		}
	}
	for _, t := range scope.ResourceTypes {
		addScopeType(t)
	}

	hasIDs := len(resourceIDSet) > 0
	hasTypes := len(typeSet) > 0

	matchesType := func(candidates ...string) bool {
		if !hasTypes {
			return true
		}
		for _, candidate := range candidates {
			if candidate == "" {
				continue
			}
			if typeSet[strings.ToLower(candidate)] {
				return true
			}
		}
		return false
	}

	matchesID := func(candidates ...string) bool {
		if !hasIDs {
			return true
		}
		for _, candidate := range candidates {
			if candidate == "" {
				continue
			}
			if resourceIDSet[candidate] {
				return true
			}
		}
		return false
	}

	filtered := models.StateSnapshot{
		LastUpdate:       state.LastUpdate,
		ConnectionHealth: state.ConnectionHealth,
		Stats:            state.Stats,
		ActiveAlerts:     state.ActiveAlerts,
		RecentlyResolved: state.RecentlyResolved,
	}

	// Filter each resource type
	for _, n := range state.Nodes {
		if matchesType("node") && matchesID(n.ID, n.Name) {
			filtered.Nodes = append(filtered.Nodes, n)
		}
	}
	for _, vm := range state.VMs {
		if matchesType("vm", "qemu") && matchesID(vm.ID, vm.Name) {
			filtered.VMs = append(filtered.VMs, vm)
		}
	}
	for _, c := range state.Containers {
		if matchesType("container", "lxc") && matchesID(c.ID, c.Name) {
			filtered.Containers = append(filtered.Containers, c)
		}
	}
	for _, d := range state.DockerHosts {
		if !matchesType("docker", "docker_host", "docker_container") {
			continue
		}

		hostName := d.CustomDisplayName
		if hostName == "" {
			hostName = d.DisplayName
		}
		if hostName == "" {
			hostName = d.Hostname
		}

		hostMatches := matchesID(d.ID, hostName, d.Hostname, d.DisplayName, d.CustomDisplayName)
		if !hasIDs {
			filtered.DockerHosts = append(filtered.DockerHosts, d)
			continue
		}

		var matchedContainers []models.DockerContainer
		for _, c := range d.Containers {
			if matchesID(c.ID, c.Name) {
				matchedContainers = append(matchedContainers, c)
			}
		}

		if hostMatches {
			filtered.DockerHosts = append(filtered.DockerHosts, d)
			continue
		}
		if len(matchedContainers) > 0 {
			hostCopy := d
			hostCopy.Containers = matchedContainers
			filtered.DockerHosts = append(filtered.DockerHosts, hostCopy)
		}
	}
	for _, s := range state.Storage {
		if matchesType("storage") && matchesID(s.ID, s.Name) {
			filtered.Storage = append(filtered.Storage, s)
		}
	}
	for _, pbs := range state.PBSInstances {
		if !matchesType("pbs", "pbs_datastore", "pbs_job") {
			continue
		}

		pbsName := pbs.Name
		if pbsName == "" {
			pbsName = pbs.Host
		}
		pbsMatches := matchesID(pbs.ID, pbs.Name, pbsName, pbs.Host)
		if !hasIDs {
			filtered.PBSInstances = append(filtered.PBSInstances, pbs)
			continue
		}
		if !pbsMatches {
			for _, ds := range pbs.Datastores {
				if matchesID(pbs.ID+":"+ds.Name, ds.Name) {
					pbsMatches = true
					break
				}
			}
		}
		if !pbsMatches {
			for _, job := range pbs.BackupJobs {
				if matchesID(pbs.ID+":job:"+job.ID, job.ID) {
					pbsMatches = true
					break
				}
			}
		}
		if !pbsMatches {
			for _, job := range pbs.VerifyJobs {
				if matchesID(pbs.ID+":verify:"+job.ID, job.ID) {
					pbsMatches = true
					break
				}
			}
		}
		if pbsMatches {
			filtered.PBSInstances = append(filtered.PBSInstances, pbs)
		}
	}
	for _, h := range state.Hosts {
		if matchesType("host", "host_raid", "host_sensor") && matchesID(h.ID, h.DisplayName, h.Hostname) {
			filtered.Hosts = append(filtered.Hosts, h)
		}
	}
	for _, k := range state.KubernetesClusters {
		clusterName := k.CustomDisplayName
		if clusterName == "" {
			clusterName = k.DisplayName
		}
		if clusterName == "" {
			clusterName = k.Name
		}
		if matchesType("kubernetes", "k8s", "kubernetes_cluster") && matchesID(k.ID, clusterName) {
			filtered.KubernetesClusters = append(filtered.KubernetesClusters, k)
		}
	}

	return filtered
}

// GetStatus returns the current patrol status
func (p *PatrolService) GetStatus() PatrolStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()

	interval := p.config.GetInterval()
	intervalMs := int64(interval / time.Millisecond)

	// "Running" means an analysis is currently in progress, not just the service loop
	analysisInProgress := p.runInProgress

	status := PatrolStatus{
		Running:          analysisInProgress,
		Enabled:          p.config.Enabled,
		LastDuration:     p.lastDuration,
		ResourcesChecked: p.resourcesChecked,
		FindingsCount:    len(p.findings.GetActive(FindingSeverityInfo)),
		ErrorCount:       p.errorCount,
		IntervalMs:       intervalMs,
		BlockedReason:    p.lastBlockedReason,
	}

	if !p.lastPatrol.IsZero() {
		status.LastPatrolAt = &p.lastPatrol
	}
	if !p.lastBlockedAt.IsZero() {
		status.BlockedAt = &p.lastBlockedAt
	}

	// Use the tracked next scheduled time (accounts for ticker resets on interval changes)
	if p.config.Enabled && interval > 0 && !p.nextScheduledAt.IsZero() {
		next := p.nextScheduledAt
		status.NextPatrolAt = &next
	}

	summary := p.findings.GetSummary()
	status.Healthy = summary.IsHealthy()

	return status
}

// SubscribeToStream returns a channel that will receive streaming patrol events
func (p *PatrolService) SubscribeToStream() chan PatrolStreamEvent {
	return p.SubscribeToStreamFrom(0)
}

// SubscribeToStreamFrom subscribes a client to patrol streaming events and optionally replays
// events with Seq > lastSeq (best-effort). This allows SSE clients to resume after disconnects
// using the Last-Event-ID header.
func (p *PatrolService) SubscribeToStreamFrom(lastSeq int64) chan PatrolStreamEvent {
	ch := make(chan PatrolStreamEvent, 100) // Buffered to prevent blocking
	sub := &streamSubscriber{ch: ch}
	replayedCount := 0
	snapshotReasons := make([]string, 0, 2)
	snapshotReasonSeen := make(map[string]struct{}, 2)

	p.streamMu.Lock()
	p.streamSubscribers[ch] = sub

	trySendSnapshot := func(reason string) bool {
		if _, seen := snapshotReasonSeen[reason]; seen {
			return true
		}
		snap := p.makeSnapshotLocked(reason)
		select {
		case ch <- snap:
			snapshotReasonSeen[reason] = struct{}{}
			snapshotReasons = append(snapshotReasons, reason)
			return true
		default:
			return false
		}
	}

	bufferStart, bufferEnd := p.streamBufferWindowLocked()
	// If the client is behind the buffered window, proactively emit a snapshot that
	// advertises truncation. (We may still replay what we have.)
	if lastSeq > 0 && bufferStart > 0 && lastSeq < bufferStart && p.streamPhase != "idle" {
		trySendSnapshot("buffer_rotated")
	}

	// Best-effort replay / snapshot:
	// - If client provides lastSeq, replay newer buffered events (Seq > lastSeq).
	// - If lastSeq is stale/ahead (e.g. from a different run), send a snapshot so UI can resync.
	// - If no lastSeq, send a snapshot (late-joiner).
	if lastSeq > 0 && len(p.streamEvents) > 0 {
		events := p.streamEventsSinceLocked(lastSeq)
	replayLoop:
		for _, ev := range events {
			select {
			case ch <- ev:
				replayedCount++
			default:
				// If subscriber can't catch up, stop replaying and let it receive live events.
				break replayLoop
			}
		}
	}
	if replayedCount == 0 && len(snapshotReasons) == 0 && lastSeq > 0 && p.streamPhase != "idle" {
		// lastSeq is likely stale (ahead of this run) or we're missing buffered events.
		// Provide a snapshot to allow the UI to resync.
		reason := "stale_last_event_id"
		if bufferEnd > 0 && lastSeq > bufferEnd {
			reason = "stale_last_event_id"
		} else if bufferStart > 0 && lastSeq < bufferStart {
			reason = "buffer_rotated"
		}
		trySendSnapshot(reason)
	}
	if lastSeq == 0 && p.streamPhase != "idle" {
		trySendSnapshot("late_joiner")
	}
	p.streamMu.Unlock()

	metrics := GetPatrolMetrics()
	if replayedCount > 0 {
		metrics.RecordStreamReplay(replayedCount)
		log.Debug().Int64("last_seq", lastSeq).Int("replayed_events", replayedCount).Msg("patrol stream replayed buffered events")
	}
	for _, reason := range snapshotReasons {
		metrics.RecordStreamSnapshot(reason)
		log.Debug().Int64("last_seq", lastSeq).Str("resync_reason", reason).Msg("patrol stream sent synthetic snapshot")
	}
	if lastSeq > 0 && replayedCount == 0 && len(snapshotReasons) == 0 {
		metrics.RecordStreamMiss()
		log.Debug().Int64("last_seq", lastSeq).Msg("patrol stream resume had no replay or snapshot")
	}

	return ch
}

// UnsubscribeFromStream removes a subscriber
func (p *PatrolService) UnsubscribeFromStream(ch chan PatrolStreamEvent) {
	p.streamMu.Lock()
	sub, exists := p.streamSubscribers[ch]
	delete(p.streamSubscribers, ch)
	p.streamMu.Unlock()

	// Use atomic CAS to ensure exactly one goroutine closes the channel,
	// even if broadcast and unsubscribe race.
	if exists && sub.closed.CompareAndSwap(false, true) {
		close(ch)
	}
}

// broadcast sends an event to all subscribers
// Subscribers with full channels are automatically removed to prevent memory leaks
func (p *PatrolService) broadcast(event PatrolStreamEvent) {
	p.streamMu.Lock()
	defer p.streamMu.Unlock()

	// Track a couple pieces of best-effort state for snapshots/resync.
	switch event.Type {
	case "tool_start":
		if event.ToolName != "" {
			p.streamCurrentTool = event.ToolName
		}
	case "tool_end":
		p.streamCurrentTool = ""
	}

	// Bound payload sizes so streaming and replay buffers can't balloon due to a single tool
	// output or oversized content chunk.
	event = truncateStreamEvent(event)

	// Decorate once so every subscriber sees identical meta.
	event = p.decorateStreamEventLocked(event)
	p.appendStreamEventLocked(event)

	var staleChannels []chan PatrolStreamEvent
	dropReasons := make(map[chan PatrolStreamEvent]string)
	for ch, sub := range p.streamSubscribers {
		if sub == nil || sub.closed.Load() {
			staleChannels = append(staleChannels, ch)
			dropReasons[ch] = "closed"
			continue
		}
		select {
		case ch <- event:
			// Successfully sent
			sub.fullCount = 0
		default:
			// Channel full. Tolerate bursts, but disconnect subscribers that are
			// consistently unable to receive events (likely dead/slow clients).
			sub.fullCount++
			if sub.fullCount >= 25 {
				staleChannels = append(staleChannels, ch)
				dropReasons[ch] = "backpressure"
			}
		}
	}

	// Clean up stale subscribers using atomic CAS for safe close
	for _, ch := range staleChannels {
		sub := p.streamSubscribers[ch]
		delete(p.streamSubscribers, ch)
		reason := dropReasons[ch]
		GetPatrolMetrics().RecordStreamSubscriberDrop(reason)
		log.Debug().Str("reason", reason).Msg("patrol stream subscriber dropped")
		if sub != nil && sub.closed.CompareAndSwap(false, true) {
			close(ch)
		}
	}
}

// resetStreamForRun resets stream state for a new run so late-joiners don't see stale output.
// This should only be called for runs that will actually stream events (NoStream=false).
func (p *PatrolService) resetStreamForRun(runID string) {
	p.streamMu.Lock()
	p.streamRunID = runID
	p.streamSeq = 0
	p.streamPhase = "idle"
	p.streamCurrentTool = ""
	p.currentOutput.Reset()
	p.streamEvents = nil
	p.streamMu.Unlock()
}

func (p *PatrolService) decorateStreamEventLocked(event PatrolStreamEvent) PatrolStreamEvent {
	if event.RunID == "" {
		event.RunID = p.streamRunID
	}
	if event.Seq == 0 {
		p.streamSeq++
		event.Seq = p.streamSeq
	}
	if event.TsMs == 0 {
		event.TsMs = time.Now().UnixMilli()
	}
	return event
}

const patrolStreamReplayBufferSize = 200
const patrolStreamMaxEventFieldBytes = 8 * 1024

func truncateStreamEvent(event PatrolStreamEvent) PatrolStreamEvent {
	event.Content = truncateStreamField(event.Content, patrolStreamMaxEventFieldBytes)
	event.ToolInput = truncateStreamField(event.ToolInput, patrolStreamMaxEventFieldBytes)
	event.ToolRawInput = truncateStreamField(event.ToolRawInput, patrolStreamMaxEventFieldBytes)
	event.ToolOutput = truncateStreamField(event.ToolOutput, patrolStreamMaxEventFieldBytes)
	return event
}

func truncateStreamField(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	const suffix = "...[truncated]"
	if max <= len(suffix) {
		return s[:max]
	}
	return s[:max-len(suffix)] + suffix
}

func (p *PatrolService) appendStreamEventLocked(event PatrolStreamEvent) {
	// Keep a bounded buffer for Last-Event-ID replay (best-effort).
	p.streamEvents = append(p.streamEvents, event)
	if len(p.streamEvents) > patrolStreamReplayBufferSize {
		p.streamEvents = p.streamEvents[len(p.streamEvents)-patrolStreamReplayBufferSize:]
	}
}

func (p *PatrolService) streamEventsSinceLocked(lastSeq int64) []PatrolStreamEvent {
	// Seq is monotonic within a run; we reset buffer on new run.
	for i := len(p.streamEvents) - 1; i >= 0; i-- {
		if p.streamEvents[i].Seq <= lastSeq {
			// Return events after i
			out := make([]PatrolStreamEvent, len(p.streamEvents)-(i+1))
			copy(out, p.streamEvents[i+1:])
			return out
		}
	}
	// All buffered events are newer
	out := make([]PatrolStreamEvent, len(p.streamEvents))
	copy(out, p.streamEvents)
	return out
}

func (p *PatrolService) streamBufferWindowLocked() (start, end int64) {
	if len(p.streamEvents) == 0 {
		return 0, 0
	}
	return p.streamEvents[0].Seq, p.streamEvents[len(p.streamEvents)-1].Seq
}

func (p *PatrolService) makeSnapshotLocked(reason string) PatrolStreamEvent {
	start, end := p.streamBufferWindowLocked()
	phase := p.streamPhase
	if phase == "idle" {
		phase = ""
	}
	tr := p.currentOutput.Truncated()
	var trPtr *bool
	if tr {
		trPtr = &tr
	}
	// Snapshot is synthetic and should not advance seq; use the most recent real event seq
	// so clients can resume from a meaningful Last-Event-ID.
	seq := end
	return PatrolStreamEvent{
		Type:             "snapshot",
		RunID:            p.streamRunID,
		Seq:              seq,
		TsMs:             time.Now().UnixMilli(),
		ResyncReason:     reason,
		BufferStart:      start,
		BufferEnd:        end,
		ContentTruncated: trPtr,
		Phase:            phase,
		Content:          p.currentOutput.String(),
		ToolName:         p.streamCurrentTool,
	}
}

// appendStreamContent adds content to the current output and broadcasts it
func (p *PatrolService) appendStreamContent(content string) {
	p.streamMu.Lock()
	p.currentOutput.WriteString(content)
	p.streamMu.Unlock()

	p.broadcast(PatrolStreamEvent{
		Type:    "content",
		Content: content,
	})
}

// setStreamPhase updates the current phase and broadcasts it to all subscribers.
// The frontend only updates its phase display when it receives a 'phase' event,
// so we must broadcast phase changes to keep the UI in sync.
func (p *PatrolService) setStreamPhase(phase string) {
	p.streamMu.Lock()
	oldPhase := p.streamPhase
	p.streamPhase = phase
	p.streamMu.Unlock()

	// Broadcast phase change (except for idle which just clears state)
	// This ensures late joiners and continuous watchers see the current phase
	if phase != "idle" && phase != oldPhase {
		p.broadcast(PatrolStreamEvent{
			Type:  "phase",
			Phase: phase,
		})
	}
}

// GetCurrentStreamOutput returns the current buffered output (for late joiners)
func (p *PatrolService) GetCurrentStreamOutput() (string, string) {
	p.streamMu.RLock()
	defer p.streamMu.RUnlock()
	return p.currentOutput.String(), p.streamPhase
}

// reviewAndResolveAlerts uses AI to review active alerts and resolve those where the issue is fixed.
// This is the core of autonomous alert management - the AI looks at each alert, checks current state,
// and determines if the underlying issue has been resolved.
func (p *PatrolService) reviewAndResolveAlerts(ctx context.Context, state models.StateSnapshot, llmAllowed bool) int {
	p.mu.RLock()
	resolver := p.alertResolver
	aiService := p.aiService
	p.mu.RUnlock()

	if resolver == nil {
		return 0
	}

	activeAlerts := resolver.GetActiveAlerts()
	if len(activeAlerts) == 0 {
		return 0
	}

	// Only review alerts that have been active for at least 10 minutes
	// This avoids thrashing on transient alerts
	minAge := 10 * time.Minute
	var alertsToReview []AlertInfo
	for _, alert := range activeAlerts {
		if time.Since(alert.StartTime) >= minAge {
			alertsToReview = append(alertsToReview, alert)
		}
	}

	if len(alertsToReview) == 0 {
		return 0
	}

	log.Info().
		Int("total_active", len(activeAlerts)).
		Int("to_review", len(alertsToReview)).
		Msg("AI Patrol: Reviewing alerts for auto-resolution")

	resolvedCount := 0

	// Pass nil for aiService if LLM is not allowed (use heuristic checks only).
	aiSvc := aiService
	if !llmAllowed {
		aiSvc = nil
	}

	for _, alert := range alertsToReview {
		shouldResolve, reason := p.shouldResolveAlert(ctx, alert, state, aiSvc)
		if shouldResolve {
			if resolver.ResolveAlert(alert.ID) {
				resolvedCount++
				log.Info().
					Str("alertID", alert.ID).
					Str("resource", alert.ResourceName).
					Str("reason", reason).
					Dur("age", time.Since(alert.StartTime)).
					Msg("AI Patrol: Auto-resolved alert - issue no longer detected")
			}
		}
	}

	if resolvedCount > 0 {
		log.Info().
			Int("resolved", resolvedCount).
			Msg("AI Patrol: Completed alert review")
	}

	return resolvedCount
}

// shouldResolveAlert determines if an alert should be auto-resolved based on current state.
// Returns (shouldResolve, reason)
func (p *PatrolService) shouldResolveAlert(ctx context.Context, alert AlertInfo, state models.StateSnapshot, aiService *Service) (bool, string) {
	// First, try smart heuristic checks based on alert type
	switch alert.Type {
	case "usage": // Storage usage alert
		// Find the storage in current state
		for _, storage := range state.Storage {
			if storage.ID == alert.ResourceID {
				// If current usage is below the threshold (with some margin), resolve
				if storage.Usage < alert.Threshold*0.95 { // 5% margin below threshold
					return true, fmt.Sprintf("storage usage dropped from %.1f%% to %.1f%% (threshold: %.1f%%)",
						alert.Value, storage.Usage, alert.Threshold)
				}
				// Still high, don't resolve
				return false, ""
			}
		}
		// Storage not found in current state - might have been removed
		// Resolve after 24 hours if resource is gone
		if time.Since(alert.StartTime) > 24*time.Hour {
			return true, "resource no longer present in infrastructure"
		}

	case "cpu", "memory": // Resource utilization alerts
		// Check if this is a node, VM, container, or docker container
		currentValue := p.getCurrentMetricValue(alert, state)
		if currentValue >= 0 && currentValue < alert.Threshold*0.95 {
			return true, fmt.Sprintf("%s dropped from %.1f%% to %.1f%% (threshold: %.1f%%)",
				alert.Type, alert.Value, currentValue, alert.Threshold)
		}

	case "offline", "stopped", "docker-offline":
		// Check if the resource is now online
		if p.isResourceOnline(alert, state) {
			return true, "resource is now online/running"
		}
	}

	// For complex cases or when heuristics don't apply, use AI judgment if available
	if aiService != nil && aiService.IsEnabled() {
		return p.askAIAboutAlert(ctx, alert, state, aiService)
	}

	return false, ""
}

// getCurrentMetricValue gets the current value of the metric that triggered the alert
func (p *PatrolService) getCurrentMetricValue(alert AlertInfo, state models.StateSnapshot) float64 {
	switch alert.ResourceType {
	case "node":
		for _, node := range state.Nodes {
			if node.ID == alert.ResourceID || node.Name == alert.ResourceName {
				if alert.Type == "cpu" {
					return node.CPU * 100
				} else if alert.Type == "memory" {
					return node.Memory.Usage
				}
			}
		}
	case "guest", "vm":
		for _, vm := range state.VMs {
			if vm.ID == alert.ResourceID || vm.Name == alert.ResourceName {
				if alert.Type == "cpu" {
					return vm.CPU * 100
				} else if alert.Type == "memory" {
					return vm.Memory.Usage
				}
			}
		}
	case "container":
		for _, ct := range state.Containers {
			if ct.ID == alert.ResourceID || ct.Name == alert.ResourceName {
				if alert.Type == "cpu" {
					return ct.CPU * 100
				} else if alert.Type == "memory" {
					return ct.Memory.Usage
				}
			}
		}
	case "docker":
		for _, host := range state.DockerHosts {
			for _, container := range host.Containers {
				if container.ID == alert.ResourceID || container.Name == alert.ResourceName {
					if alert.Type == "cpu" {
						return container.CPUPercent
					} else if alert.Type == "memory" {
						return container.MemoryPercent
					}
				}
			}
		}
	case "Storage":
		for _, storage := range state.Storage {
			if storage.ID == alert.ResourceID || storage.Name == alert.ResourceName {
				return storage.Usage
			}
		}
	}
	return -1 // Not found
}

// isResourceOnline checks if a resource that triggered an offline alert is now online
func (p *PatrolService) isResourceOnline(alert AlertInfo, state models.StateSnapshot) bool {
	switch alert.ResourceType {
	case "node":
		for _, node := range state.Nodes {
			if (node.ID == alert.ResourceID || node.Name == alert.ResourceName) && node.Status == "online" {
				return true
			}
		}
	case "guest", "vm":
		for _, vm := range state.VMs {
			if (vm.ID == alert.ResourceID || vm.Name == alert.ResourceName) && vm.Status == "running" {
				return true
			}
		}
	case "container":
		for _, ct := range state.Containers {
			if (ct.ID == alert.ResourceID || ct.Name == alert.ResourceName) && ct.Status == "running" {
				return true
			}
		}
	case "docker":
		for _, host := range state.DockerHosts {
			for _, container := range host.Containers {
				if (container.ID == alert.ResourceID || container.Name == alert.ResourceName) && container.State == "running" {
					return true
				}
			}
		}
	}
	return false
}

// askAIAboutAlert uses the AI to determine if an alert should be resolved
func (p *PatrolService) askAIAboutAlert(ctx context.Context, alert AlertInfo, state models.StateSnapshot, aiService *Service) (bool, string) {
	// Build a focused prompt for the AI
	prompt := fmt.Sprintf(`Review this alert and determine if it should be auto-resolved based on current state.

ALERT:
- ID: %s
- Type: %s
- Resource: %s (%s)
- Message: %s
- Value when triggered: %.1f
- Threshold: %.1f
- Active for: %s

CURRENT STATE OF THIS RESOURCE:
%s

Should this alert be RESOLVED because the underlying issue is fixed?
Respond with ONLY one of:
- RESOLVE: <brief reason>
- KEEP: <brief reason>`,
		alert.ID, alert.Type, alert.ResourceName, alert.ResourceType,
		alert.Message, alert.Value, alert.Threshold, alert.Duration,
		p.getResourceCurrentState(alert, state))

	// Use a quick, low-cost AI call
	response, err := aiService.QuickAnalysis(ctx, prompt)
	if err != nil {
		log.Debug().Err(err).Str("alertID", alert.ID).Msg("AI Patrol: Failed to get AI judgment on alert")
		return false, ""
	}

	response = strings.TrimSpace(response)
	if strings.HasPrefix(strings.ToUpper(response), "RESOLVE:") {
		reason := strings.TrimSpace(strings.TrimPrefix(response, "RESOLVE:"))
		if reason == "" {
			reason = strings.TrimSpace(strings.TrimPrefix(strings.ToUpper(response), "RESOLVE:"))
		}
		return true, "Patrol: " + reason
	}

	return false, ""
}

// getResourceCurrentState returns a description of the resource's current state
func (p *PatrolService) getResourceCurrentState(alert AlertInfo, state models.StateSnapshot) string {
	switch alert.ResourceType {
	case "Storage":
		for _, storage := range state.Storage {
			if storage.ID == alert.ResourceID || storage.Name == alert.ResourceName {
				return fmt.Sprintf("Storage '%s': %.1f%% used, status: %s", storage.Name, storage.Usage, storage.Status)
			}
		}
		return "Storage not found in current state (may have been removed)"
	case "node":
		for _, node := range state.Nodes {
			if node.ID == alert.ResourceID || node.Name == alert.ResourceName {
				return fmt.Sprintf("Node '%s': CPU %.1f%%, Memory %.1f%%, Status: %s",
					node.Name, node.CPU, node.Memory.Usage, node.Status)
			}
		}
		return "Node not found in current state"
	case "guest", "vm":
		for _, vm := range state.VMs {
			if vm.ID == alert.ResourceID || vm.Name == alert.ResourceName {
				return fmt.Sprintf("VM '%s': CPU %.1f%%, Memory %.1f%%, Status: %s",
					vm.Name, vm.CPU, vm.Memory.Usage, vm.Status)
			}
		}
		return "VM not found in current state"
	case "container":
		for _, ct := range state.Containers {
			if ct.ID == alert.ResourceID || ct.Name == alert.ResourceName {
				return fmt.Sprintf("Container '%s': CPU %.1f%%, Memory %.1f%%, Status: %s",
					ct.Name, ct.CPU, ct.Memory.Usage, ct.Status)
			}
		}
		return "Container not found in current state"
	case "docker":
		for _, host := range state.DockerHosts {
			for _, container := range host.Containers {
				if container.ID == alert.ResourceID || container.Name == alert.ResourceName {
					return fmt.Sprintf("Docker container '%s': CPU %.1f%%, Memory %.1f%%, State: %s",
						container.Name, container.CPUPercent, container.MemoryPercent, container.State)
				}
			}
		}
		return "Docker container not found in current state"
	default:
		return "Resource state unknown"
	}
}

// TriggerPatrolForAlert triggers an immediate patrol for a specific alert
func (p *PatrolService) TriggerPatrolForAlert(alert *alerts.Alert) {
	if alert == nil {
		return
	}

	p.mu.RLock()
	triggerManager := p.triggerManager
	eventTriggersEnabled := p.eventTriggersEnabled
	p.mu.RUnlock()

	// Gate: skip if event-driven triggers are disabled
	if !eventTriggersEnabled {
		log.Debug().Str("alert_id", alert.ID).Msg("alert-triggered patrol skipped: event triggers disabled")
		return
	}

	resourceType := inferResourceType(alert.Type, alert.Metadata)

	if triggerManager != nil {
		scope := AlertTriggeredPatrolScope(alert.ID, alert.ResourceID, resourceType, alert.Type)
		if triggerManager.TriggerPatrol(scope) {
			log.Debug().Str("alert_id", alert.ID).Msg("queued alert-triggered patrol via trigger manager")
		} else {
			log.Warn().Str("alert_id", alert.ID).Msg("alert-triggered patrol rejected by trigger manager")
		}
		return
	}

	// Non-blocking send
	select {
	case p.adHocTrigger <- alert:
		log.Debug().Str("alert_id", alert.ID).Msg("queued ad-hoc patrol trigger")
	default:
		log.Warn().Str("alert_id", alert.ID).Msg("patrol trigger queue full, dropping trigger")
	}
}

func (p *PatrolService) tryStartRun(kind string) bool {
	p.mu.Lock()
	if p.runInProgress {
		// Detect stuck runs: if the current run has been going for >20 minutes,
		// force-clear the flag so a new run can proceed.
		if !p.runStartedAt.IsZero() && time.Since(p.runStartedAt) > 20*time.Minute {
			log.Warn().
				Str("kind", kind).
				Time("started_at", p.runStartedAt).
				Dur("elapsed", time.Since(p.runStartedAt)).
				Msg("AI Patrol: Previous run appears stuck (>20min), force-clearing runInProgress")
			p.runInProgress = false
			// Fall through to start new run
		} else {
			p.mu.Unlock()
			if kind == "scoped" {
				GetPatrolMetrics().RecordScopedDropped()
				log.Warn().Str("kind", kind).Msg("AI Patrol: Run already in progress, dropping scoped patrol")
			} else {
				log.Debug().Str("kind", kind).Msg("AI Patrol: Run already in progress, skipping")
			}
			return false
		}
	}
	p.runInProgress = true
	p.runStartedAt = time.Now()
	p.mu.Unlock()
	return true
}

func (p *PatrolService) endRun() {
	p.mu.Lock()
	p.runInProgress = false
	orch := p.investigationOrchestrator
	p.mu.Unlock()

	// Periodic investigation store maintenance after each run
	if maintainer, ok := orch.(InvestigationStoreMaintainer); ok {
		maintainer.CleanupInvestigationStore(24*time.Hour, 1000)
	}
}

// runTargetedPatrol executes a focused patrol for a specific alert
func (p *PatrolService) runTargetedPatrol(ctx context.Context, alert *alerts.Alert) {
	log.Info().
		Str("alert_id", alert.ID).
		Str("resource_id", alert.ResourceID).
		Msg("Running targeted AI patrol for alert")

	resourceType := inferResourceType(alert.Type, alert.Metadata)
	scope := AlertTriggeredPatrolScope(alert.ID, alert.ResourceID, resourceType, alert.Type)
	p.TriggerScopedPatrol(ctx, scope)
}

// joinParts joins string parts with commas and "and" for the last element
func joinParts(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return parts[0]
	}
	if len(parts) == 2 {
		return parts[0] + " and " + parts[1]
	}
	return strings.Join(parts[:len(parts)-1], ", ") + ", and " + parts[len(parts)-1]
}

// generateFindingID creates a stable ID for a finding based on resource, category, and issue.
// All three components are included to ensure distinct issues on the same resource remain separate.
func generateFindingID(resourceID, category, issue string) string {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%s", resourceID, category, issue)))
	return fmt.Sprintf("%x", hash[:8])
}
