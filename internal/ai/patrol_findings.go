// patrol_findings.go manages the finding lifecycle: creation, resolution, dismissal,
// remediation plan generation, investigation triggering and verification,
// and the adapter types that bridge patrol findings to the investigation subsystem.
package ai

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/baseline"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/remediation"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/safety"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rs/zerolog/log"
)

// recordFinding stores a finding, syncs it to the unified store, and triggers follow-up actions.
func (p *PatrolService) recordFinding(f *Finding) bool {
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

		// Generate remediation plan for actionable findings
		// Skip internal error findings (not actionable by users)
		if !(stored.Key == "ai-patrol-error" || stored.ResourceID == "ai-service") {
			p.generateRemediationPlan(stored)
		}
	}

	// Keep unified store in sync even when findings transition to snoozed/dismissed/resolved.
	// The unified UI can filter by status; losing updates here makes the patrol loop look broken.
	if p.unifiedFindingCallback != nil {
		p.unifiedFindingCallback(stored)
	}

	// Trigger autonomous investigation if enabled and finding warrants it
	p.MaybeInvestigateFinding(stored)

	return isNew
}

func (p *PatrolService) setBlockedReason(reason string) {
	if reason == "" {
		return
	}
	p.mu.Lock()
	p.lastBlockedReason = reason
	p.lastBlockedAt = time.Now()
	p.mu.Unlock()
}

func (p *PatrolService) clearBlockedReason() {
	p.mu.Lock()
	p.lastBlockedReason = ""
	p.lastBlockedAt = time.Time{}
	p.mu.Unlock()
}

// generateRemediationPlan creates a remediation plan for a finding if appropriate.
// Only generates plans for critical/warning findings when a remediation engine is configured.
func (p *PatrolService) generateRemediationPlan(finding *Finding) {
	p.mu.RLock()
	engine := p.remediationEngine
	p.mu.RUnlock()

	if engine == nil {
		return
	}

	// Only generate plans for actionable findings
	if finding.Severity != FindingSeverityCritical && finding.Severity != FindingSeverityWarning {
		return
	}

	// Generate remediation steps based on finding category and resource type
	steps := p.generateRemediationSteps(finding)
	if len(steps) == 0 {
		return
	}

	// Determine risk level based on finding severity and category
	riskLevel := remediation.RiskLow
	if finding.Severity == FindingSeverityWarning {
		riskLevel = remediation.RiskMedium
	}
	if finding.Severity == FindingSeverityCritical {
		riskLevel = remediation.RiskHigh
	}
	// Reliability issues involving restarts/reboots are higher risk
	if finding.Category == FindingCategoryReliability {
		title := strings.ToLower(finding.Title)
		if strings.Contains(title, "restart") || strings.Contains(title, "reboot") || strings.Contains(title, "offline") {
			if riskLevel < remediation.RiskHigh {
				riskLevel = remediation.RiskHigh
			}
		} else if riskLevel < remediation.RiskMedium {
			riskLevel = remediation.RiskMedium
		}
	}

	// Create the remediation plan
	plan := &remediation.RemediationPlan{
		FindingID:   finding.ID,
		ResourceID:  finding.ResourceID,
		Title:       fmt.Sprintf("Fix: %s", finding.Title),
		Description: finding.Description,
		Category:    remediation.CategoryGuided, // All auto-generated plans require user approval
		RiskLevel:   riskLevel,
		Steps:       steps,
		Rationale:   finding.Recommendation,
	}

	// Add warnings based on risk level
	if riskLevel == remediation.RiskHigh {
		plan.Warnings = append(plan.Warnings, "High risk: This action may cause service disruption. Review carefully and consider scheduling during maintenance window.")
	} else if riskLevel == remediation.RiskMedium {
		plan.Warnings = append(plan.Warnings, "Review steps carefully before execution")
	}

	if err := engine.CreatePlan(plan); err != nil {
		log.Debug().
			Err(err).
			Str("finding_id", finding.ID).
			Str("resource", finding.ResourceName).
			Msg("AI Patrol: Failed to create remediation plan")
		return
	}

	log.Info().
		Str("plan_id", plan.ID).
		Str("finding_id", finding.ID).
		Str("resource", finding.ResourceName).
		Int("steps", len(steps)).
		Msg("AI Patrol: Remediation plan generated")
}

// generateRemediationPlanFromInvestigation persists a remediation plan artifact when
// an investigation proposes a concrete fix command. This is intentionally separate
// from the "approval" execution pipeline; it's a durable summary users can act on
// later (often via Pulse Assistant).
func (p *PatrolService) generateRemediationPlanFromInvestigation(findingID string) {
	p.mu.RLock()
	engine := p.remediationEngine
	orchestrator := p.investigationOrchestrator
	p.mu.RUnlock()

	if engine == nil || orchestrator == nil || p.findings == nil || findingID == "" {
		return
	}

	finding := p.findings.Get(findingID)
	if finding == nil {
		return
	}

	inv := orchestrator.GetInvestigationByFinding(findingID)
	if inv == nil || inv.ProposedFix == nil || len(inv.ProposedFix.Commands) == 0 {
		return
	}
	fix := inv.ProposedFix

	targetHost := strings.TrimSpace(fix.TargetHost)
	if targetHost == "" {
		targetHost = "local"
	}

	// Map investigation risk strings into remediation risk levels.
	riskLevel := remediation.RiskMedium
	switch strings.ToLower(strings.TrimSpace(fix.RiskLevel)) {
	case "low":
		riskLevel = remediation.RiskLow
	case "medium":
		riskLevel = remediation.RiskMedium
	case "high":
		riskLevel = remediation.RiskHigh
	case "critical":
		riskLevel = remediation.RiskHigh
	}

	steps := make([]remediation.RemediationStep, 0, 2+len(fix.Commands))
	steps = append(steps, remediation.RemediationStep{
		Order:       1,
		Description: "Review the finding context and confirm the proposed fix is appropriate",
	})

	blockedCount := 0
	order := 2
	for _, raw := range fix.Commands {
		cmd := strings.TrimSpace(raw)
		if cmd == "" {
			continue
		}

		stepCommand := cmd
		stepDesc := fmt.Sprintf("Run the proposed fix on %s", targetHost)
		if safety.IsBlockedCommand(cmd) {
			// Don't store blocked commands in the remediation engine; keep the plan as an
			// artifact for users to review and apply manually (typically via Assistant).
			stepCommand = ""
			stepDesc = fmt.Sprintf("Blocked command proposed by investigation (review and apply manually): %s", cmd)
			blockedCount++
		}

		steps = append(steps, remediation.RemediationStep{
			Order:       order,
			Description: stepDesc,
			Command:     stepCommand,
			Target:      targetHost,
		})
		order++
	}

	steps = append(steps, remediation.RemediationStep{
		Order:       order,
		Description: "Verify the issue is resolved (re-check metrics/logs, confirm service health)",
	})

	description := strings.TrimSpace(fix.Rationale)
	if description == "" {
		description = strings.TrimSpace(inv.Summary)
	}
	if description == "" {
		description = finding.Description
	}

	plan := &remediation.RemediationPlan{
		FindingID:   finding.ID,
		ResourceID:  finding.ResourceID,
		Title:       fmt.Sprintf("Investigation Fix: %s", finding.Title),
		Description: description,
		Category:    remediation.CategoryGuided,
		RiskLevel:   riskLevel,
		Steps:       steps,
		Rationale:   fix.Description,
	}

	// Patrol findings are often reviewed hours/days later; keep investigation-derived
	// plans around longer than the default ephemeral remediation TTL.
	expires := time.Now().Add(7 * 24 * time.Hour)
	plan.ExpiresAt = &expires

	if blockedCount > 0 {
		plan.Warnings = append(plan.Warnings, "Investigation suggested one or more commands that are blocked by safety policy. Review carefully and apply manually (prefer Pulse Assistant).")
	}

	if err := engine.CreatePlan(plan); err != nil {
		// As a fallback, keep the plan as purely informational so it can still be
		// surfaced to the user without enabling remediation engine execution.
		for i := range plan.Steps {
			if plan.Steps[i].Command == "" {
				continue
			}
			plan.Steps[i].Description = fmt.Sprintf("%s: %s", plan.Steps[i].Description, plan.Steps[i].Command)
			plan.Steps[i].Command = ""
		}
		plan.RiskLevel = remediation.RiskMedium
		plan.Warnings = append(plan.Warnings, fmt.Sprintf("Failed to store command steps for automated remediation: %v", err))
		_ = engine.CreatePlan(plan)
	}
}

// generateRemediationSteps creates appropriate steps based on finding type
func (p *PatrolService) generateRemediationSteps(finding *Finding) []remediation.RemediationStep {
	var steps []remediation.RemediationStep

	switch finding.Category {
	case FindingCategoryPerformance:
		steps = p.generatePerformanceSteps(finding)
	case FindingCategoryCapacity:
		steps = p.generateCapacitySteps(finding)
	case FindingCategoryReliability:
		steps = p.generateAvailabilitySteps(finding)
	case FindingCategoryBackup:
		steps = p.generateBackupSteps(finding)
	case FindingCategorySecurity:
		steps = p.generateSecuritySteps(finding)
	case FindingCategoryGeneral:
		steps = p.generateConfigurationSteps(finding)
	default:
		// Generic investigation steps for unknown categories
		steps = []remediation.RemediationStep{
			{Order: 1, Description: "Investigate the issue by reviewing current resource state"},
			{Order: 2, Description: "Review recent changes that may have caused this issue"},
			{Order: 3, Description: "Take appropriate corrective action based on findings"},
		}
	}

	return steps
}

// generatePerformanceSteps creates steps for performance issues
func (p *PatrolService) generatePerformanceSteps(finding *Finding) []remediation.RemediationStep {
	title := strings.ToLower(finding.Title)

	if strings.Contains(title, "cpu") {
		return []remediation.RemediationStep{
			{Order: 1, Description: "Identify processes consuming excessive CPU", Target: finding.ResourceID},
			{Order: 2, Description: "Check if resource needs more CPU cores allocated"},
			{Order: 3, Description: "Consider migrating to a less loaded host if VM/container"},
			{Order: 4, Description: "Optimize or restart resource-hungry applications"},
		}
	}

	if strings.Contains(title, "memory") || strings.Contains(title, "ram") {
		return []remediation.RemediationStep{
			{Order: 1, Description: "Identify processes consuming excessive memory", Target: finding.ResourceID},
			{Order: 2, Description: "Check for memory leaks in running applications"},
			{Order: 3, Description: "Consider increasing allocated memory"},
			{Order: 4, Description: "Restart affected services to reclaim memory"},
		}
	}

	if strings.Contains(title, "io") || strings.Contains(title, "disk") {
		return []remediation.RemediationStep{
			{Order: 1, Description: "Identify processes causing high disk I/O", Target: finding.ResourceID},
			{Order: 2, Description: "Check for runaway log files or heavy writes"},
			{Order: 3, Description: "Consider migrating to faster storage"},
		}
	}

	// Generic performance steps
	return []remediation.RemediationStep{
		{Order: 1, Description: "Review current resource utilization metrics", Target: finding.ResourceID},
		{Order: 2, Description: "Identify performance bottlenecks"},
		{Order: 3, Description: "Optimize resource allocation or application configuration"},
	}
}

// generateCapacitySteps creates steps for capacity issues
func (p *PatrolService) generateCapacitySteps(finding *Finding) []remediation.RemediationStep {
	title := strings.ToLower(finding.Title)

	if strings.Contains(title, "disk") || strings.Contains(title, "storage") {
		return []remediation.RemediationStep{
			{Order: 1, Description: "Identify largest files and directories consuming space", Target: finding.ResourceID},
			{Order: 2, Description: "Clean up temporary files, logs, and caches"},
			{Order: 3, Description: "Remove unused packages and old kernels"},
			{Order: 4, Description: "Consider expanding disk or adding additional storage"},
		}
	}

	if strings.Contains(title, "memory") {
		return []remediation.RemediationStep{
			{Order: 1, Description: "Review memory allocation across workloads", Target: finding.ResourceID},
			{Order: 2, Description: "Reduce memory allocation on over-provisioned VMs"},
			{Order: 3, Description: "Add more physical memory to the host"},
		}
	}

	// Generic capacity steps
	return []remediation.RemediationStep{
		{Order: 1, Description: "Review current capacity utilization", Target: finding.ResourceID},
		{Order: 2, Description: "Identify growth trends and plan for expansion"},
		{Order: 3, Description: "Clean up unused resources to free capacity"},
	}
}

// generateAvailabilitySteps creates steps for availability issues
func (p *PatrolService) generateAvailabilitySteps(finding *Finding) []remediation.RemediationStep {
	title := strings.ToLower(finding.Title)

	if strings.Contains(title, "offline") || strings.Contains(title, "down") {
		return []remediation.RemediationStep{
			{Order: 1, Description: "Verify network connectivity to the resource", Target: finding.ResourceID},
			{Order: 2, Description: "Check host status if this is a VM/container"},
			{Order: 3, Description: "Review system logs for crash or shutdown reasons"},
			{Order: 4, Description: "Attempt to start or restart the resource"},
		}
	}

	if strings.Contains(title, "restart") || strings.Contains(title, "reboot") {
		return []remediation.RemediationStep{
			{Order: 1, Description: "Review system logs for cause of restarts", Target: finding.ResourceID},
			{Order: 2, Description: "Check for OOM kills or kernel panics"},
			{Order: 3, Description: "Investigate application crashes"},
			{Order: 4, Description: "Consider enabling watchdog or health checks"},
		}
	}

	// Generic availability steps
	return []remediation.RemediationStep{
		{Order: 1, Description: "Verify resource health and connectivity", Target: finding.ResourceID},
		{Order: 2, Description: "Review recent events and logs"},
		{Order: 3, Description: "Take corrective action to restore availability"},
	}
}

// generateBackupSteps creates steps for backup-related issues
func (p *PatrolService) generateBackupSteps(finding *Finding) []remediation.RemediationStep {
	title := strings.ToLower(finding.Title)

	if strings.Contains(title, "missing") || strings.Contains(title, "no backup") {
		return []remediation.RemediationStep{
			{Order: 1, Description: "Verify backup job configuration exists", Target: finding.ResourceID},
			{Order: 2, Description: "Check backup storage availability and capacity"},
			{Order: 3, Description: "Create or enable backup schedule"},
			{Order: 4, Description: "Run initial backup job"},
		}
	}

	if strings.Contains(title, "failed") || strings.Contains(title, "error") {
		return []remediation.RemediationStep{
			{Order: 1, Description: "Review backup job logs for error details", Target: finding.ResourceID},
			{Order: 2, Description: "Check backup storage connectivity and space"},
			{Order: 3, Description: "Verify backup credentials and permissions"},
			{Order: 4, Description: "Retry backup job after fixing issues"},
		}
	}

	if strings.Contains(title, "old") || strings.Contains(title, "stale") || strings.Contains(title, "outdated") {
		return []remediation.RemediationStep{
			{Order: 1, Description: "Check why scheduled backups are not running", Target: finding.ResourceID},
			{Order: 2, Description: "Review backup retention policy"},
			{Order: 3, Description: "Trigger a new backup immediately"},
		}
	}

	// Generic backup steps
	return []remediation.RemediationStep{
		{Order: 1, Description: "Review backup configuration and schedule", Target: finding.ResourceID},
		{Order: 2, Description: "Verify backup storage health"},
		{Order: 3, Description: "Ensure backup jobs are running successfully"},
	}
}

// generateConfigurationSteps creates steps for configuration issues
func (p *PatrolService) generateConfigurationSteps(finding *Finding) []remediation.RemediationStep {
	return []remediation.RemediationStep{
		{Order: 1, Description: "Review current configuration settings", Target: finding.ResourceID},
		{Order: 2, Description: "Compare against recommended best practices"},
		{Order: 3, Description: "Apply configuration changes as needed"},
		{Order: 4, Description: "Verify changes don't impact dependent services"},
	}
}

// generateSecuritySteps creates steps for security issues
func (p *PatrolService) generateSecuritySteps(finding *Finding) []remediation.RemediationStep {
	return []remediation.RemediationStep{
		{Order: 1, Description: "Assess the security impact and urgency", Target: finding.ResourceID},
		{Order: 2, Description: "Review access logs for suspicious activity"},
		{Order: 3, Description: "Apply security patches or configuration fixes"},
		{Order: 4, Description: "Verify remediation and update security policies"},
	}
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
	if limit <= 0 {
		return p.runHistoryStore.GetAll()
	}
	return p.runHistoryStore.GetRecent(limit)
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

func normalizeFindingResourceTypes(findings []*Finding) {
	for _, f := range findings {
		if f == nil || f.ResourceType != "" {
			continue
		}
		f.ResourceType = inferFindingResourceType(f.ResourceID, f.ResourceName)
	}
}

// GetFindingsHistory returns all findings including resolved ones for history display
// Optionally filter by startTime
func (p *PatrolService) GetFindingsHistory(startTime *time.Time) []*Finding {
	findings := p.findings.GetAll(startTime)

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
	patrol          *PatrolService
	state           models.StateSnapshot
	findingsMu      sync.Mutex
	findings        []*Finding
	resolvedIDs     []string
	rejectedCount   int
	checkedFindings bool
}

func newPatrolFindingCreatorAdapter(p *PatrolService, state models.StateSnapshot) *patrolFindingCreatorAdapter {
	return &patrolFindingCreatorAdapter{
		patrol: p,
		state:  state,
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
	isNew := a.patrol.recordFinding(finding)

	// Track for run stats
	a.findingsMu.Lock()
	a.findings = append(a.findings, finding)
	a.findingsMu.Unlock()

	return id, isNew, nil
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
	// Build resource metrics lookup from current state
	resourceMetrics := make(map[string]map[string]float64)
	for _, n := range a.state.Nodes {
		m := map[string]float64{"cpu": n.CPU * 100}
		if n.Memory.Total > 0 {
			m["memory"] = float64(n.Memory.Used) / float64(n.Memory.Total) * 100
		}
		resourceMetrics[n.ID] = m
		resourceMetrics[n.Name] = m
	}
	for _, vm := range a.state.VMs {
		m := map[string]float64{"cpu": vm.CPU * 100, "memory": vm.Memory.Usage, "disk": vm.Disk.Usage}
		resourceMetrics[vm.ID] = m
		resourceMetrics[vm.Name] = m
	}
	for _, ct := range a.state.Containers {
		m := map[string]float64{"cpu": ct.CPU * 100, "memory": ct.Memory.Usage, "disk": ct.Disk.Usage}
		resourceMetrics[ct.ID] = m
		resourceMetrics[ct.Name] = m
	}
	for _, s := range a.state.Storage {
		m := map[string]float64{}
		if s.Total > 0 {
			m["usage"] = float64(s.Used) / float64(s.Total) * 100
		}
		resourceMetrics[s.ID] = m
		resourceMetrics[s.Name] = m
	}

	// Reject findings for resources that no longer exist in the current infrastructure.
	// Only enforce when we have state data (avoid rejecting during empty/error states).
	metrics, hasMetrics := resourceMetrics[f.ResourceID]
	if !hasMetrics {
		metrics, hasMetrics = resourceMetrics[f.ResourceName]
	}
	stateHasResources := len(a.state.Nodes) > 0 || len(a.state.VMs) > 0 || len(a.state.Containers) > 0 || len(a.state.Storage) > 0
	if !hasMetrics && stateHasResources {
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

func (a *patrolFindingCreatorAdapter) ResolveFinding(findingID, reason string) error {
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
	var result []tools.PatrolFindingInfo
	for _, f := range active {
		if resourceID != "" && f.ResourceID != resourceID && f.ResourceName != resourceID {
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
	return strings.Trim(b.String(), "-")
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
	cfg := aiService.GetConfig()
	if cfg == nil {
		return
	}
	autonomyLevel := cfg.GetPatrolAutonomyLevel()

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
		currentAutonomy := currentCfg.GetPatrolAutonomyLevel()

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
		p.mu.RLock()
		pushUnified = p.unifiedFindingCallback
		resolveUnified = p.unifiedFindingResolver
		p.mu.RUnlock()
		if latest := p.findings.Get(f.ID); latest != nil {
			if pushUnified != nil {
				pushUnified(latest)
			}
			if latest.ResolvedAt != nil && resolveUnified != nil {
				resolveUnified(latest.ID)
			}
		}

		// Investigation finished successfully. If it produced a proposed fix, persist a
		// remediation plan artifact so the user can review and execute later.
		p.generateRemediationPlanFromInvestigation(f.ID)
	}()

	log.Info().
		Str("finding_id", f.ID).
		Str("severity", string(f.Severity)).
		Str("resource", f.ResourceName).
		Str("autonomy_level", autonomyLevel).
		Msg("Triggered autonomous investigation for finding")
}

// VerifyFixResolved runs a lightweight scoped patrol to check if the issue
// identified by the given finding has been resolved after a fix was executed.
// It bypasses tryStartRun (the patrol mutex) because verification runs inline
// within the investigation goroutine.
func (p *PatrolService) VerifyFixResolved(ctx context.Context, resourceID, resourceType, findingKey, findingID string) (bool, error) {
	if p.stateProvider == nil {
		return false, fmt.Errorf("no state provider available for verification")
	}
	if p.aiService == nil {
		return false, fmt.Errorf("AI service not available for verification")
	}

	// Check circuit breaker before making LLM calls
	if p.circuitBreaker != nil && !p.circuitBreaker.Allow() {
		return false, fmt.Errorf("circuit breaker open, skipping verification")
	}

	log.Info().
		Str("finding_id", findingID).
		Str("resource_id", resourceID).
		Msg("Running verification patrol to confirm fix")

	scope := PatrolScope{
		ResourceIDs:   []string{resourceID},
		ResourceTypes: []string{resourceType},
		Depth:         PatrolDepthQuick,
		Reason:        TriggerReasonVerification,
		Context:       fmt.Sprintf("Verifying fix for finding: %s", findingID),
		FindingID:     findingID,
		NoStream:      true,
	}

	startTime := time.Now()

	fullState := p.stateProvider.GetState()
	filteredState := p.filterStateByScope(fullState, scope)

	result, err := p.runAIAnalysis(ctx, filteredState, &scope)
	if err != nil {
		if p.circuitBreaker != nil {
			p.circuitBreaker.RecordFailure(err)
		}
		return false, fmt.Errorf("verification patrol failed: %w", err)
	}
	if result == nil {
		return false, fmt.Errorf("verification patrol returned no result")
	}
	if p.circuitBreaker != nil {
		p.circuitBreaker.RecordSuccess()
	}

	// Record verification run in patrol history
	endTime := time.Now()
	duration := endTime.Sub(startTime)
	status := "completed"
	findingsSummary := "Verification: no issues found"
	if len(result.Findings) > 0 {
		findingsSummary = fmt.Sprintf("Verification: %d issue(s) still present", len(result.Findings))
		status = "issues_found"
	}
	verifyRecord := PatrolRunRecord{
		ID:                 fmt.Sprintf("%d", startTime.UnixNano()),
		StartedAt:          startTime,
		CompletedAt:        endTime,
		Duration:           duration,
		DurationMs:         duration.Milliseconds(),
		Type:               "verification",
		TriggerReason:      string(TriggerReasonVerification),
		ScopeResourceIDs:   scope.ResourceIDs,
		ScopeResourceTypes: scope.ResourceTypes,
		ScopeContext:       scope.Context,
		FindingID:          findingID,
		NewFindings:        len(result.Findings),
		FindingsSummary:    findingsSummary,
		Status:             status,
		InputTokens:        result.InputTokens,
		OutputTokens:       result.OutputTokens,
	}
	if p.runHistoryStore != nil {
		p.runHistoryStore.Add(verifyRecord)
	}

	// Check if the original finding was re-detected
	for _, f := range result.Findings {
		if (findingKey != "" && f.Key == findingKey) || f.ResourceID == resourceID {
			log.Info().
				Str("finding_id", findingID).
				Str("re_detected_key", f.Key).
				Msg("Verification patrol re-detected the issue")
			return false, nil // Issue still present
		}
	}

	log.Info().
		Str("finding_id", findingID).
		Int("findings_count", len(result.Findings)).
		Msg("Verification patrol found no matching issues - fix confirmed")
	return true, nil // No matching finding = issue resolved
}
