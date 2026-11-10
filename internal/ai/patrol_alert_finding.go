package ai

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rs/zerolog/log"
)

// alertFindingSource marks a finding Pulse opened from an alert because the
// run the alert started filed none.
const alertFindingSource = "alert"

// patrolScopeFiringAlert returns the alert a scoped run was started for, when
// it is still active. A cleared-alert check never has one.
func (p *PatrolService) patrolScopeFiringAlert(scope PatrolScope) *models.Alert {
	identifier := strings.TrimSpace(scope.AlertIdentifier)
	if p == nil || identifier == "" || scope.Reason == TriggerReasonAlertCleared || !p.hasPatrolRuntimeInputs() {
		return nil
	}
	for _, alert := range p.currentPatrolRuntimeState().ActiveAlerts {
		if alert.ID == identifier {
			found := alert
			return &found
		}
	}
	return nil
}

// patrolInvestigatesFindings reports whether Patrol is in a mode that
// investigates what it finds (Ask first or Autopilot) and has something to
// investigate with.
func (p *PatrolService) patrolInvestigatesFindings() bool {
	p.mu.RLock()
	orchestrator := p.investigationOrchestrator
	aiService := p.aiService
	p.mu.RUnlock()
	if orchestrator == nil || aiService == nil || aiService.GetConfig() == nil {
		return false
	}
	level := aiService.GetEffectivePatrolAutonomyLevel()
	return level != "" && level != "monitor"
}

// openFindingForUnansweredAlert opens a finding from the alert that started a
// run when the run left none of its own.
//
// The alert trigger exists so that an alert the operator's policy accepted is
// investigated. Investigations start from findings, so a model that describes
// the fault in prose and never files it leaves the alert with no
// investigation, and smaller models do that on every alert. Pulse files the
// finding itself then: the alert's resource, level and text, linked to the
// alert so it is folded under it and closes when it clears.
//
// It applies only to an alert that woke Patrol when it fired. A warning that
// only stayed active, a flapping alert, and a run a person or the schedule
// started keep the model's judgment, and Watch only gets nothing, because a
// finding nobody investigates would only repeat the alert.
//
// It returns the stored finding when one is open afterwards, and nil when it
// did nothing. The caller starts the investigation once the run is recorded.
func (p *PatrolService) openFindingForUnansweredAlert(scope PatrolScope, alert *models.Alert) *Finding {
	if p == nil || p.findings == nil || alert == nil {
		return nil
	}
	if scope.Reason != TriggerReasonAlertFired || scope.AlertSustained {
		return nil
	}
	if !p.patrolInvestigatesFindings() {
		return nil
	}

	state := p.currentPatrolRuntimeState()
	binding, _ := patrolAlertReferenceResolver(state).ResolveAlertResourceReference(alert.ResourceID)
	candidate := alertMirrorCandidate{ID: alert.ID, ResourceID: alert.ResourceID, Type: alert.Type}
	if !binding.Component {
		candidate.CanonicalResourceID = binding.ResourceID
	}
	resourceID := strings.TrimSpace(binding.ResourceID)
	if resourceID == "" {
		resourceID = strings.TrimSpace(alert.ResourceID)
	}
	if resourceID == "" {
		return nil
	}
	category := alertFindingCategory(alert.Type)
	// One finding per alert identifier: a host can have the same kind of alert
	// on two filesystems, and each repeat of an alert reopens its own finding.
	id := generateFindingID(resourceID, string(category), "alert:"+alert.ID)

	// Another finding already stands for this alert, for example one an
	// earlier run filed. It keeps the investigation it has, and a dismissed
	// one keeps the operator's answer.
	for _, existing := range p.findings.GetAll(nil) {
		if existing == nil || existing.ID == id || existing.ResolvedAt != nil || existing.Source == stormFindingSource {
			continue
		}
		if findingMirrorsAlert(existing, candidate) {
			return nil
		}
	}

	resourceType := string(binding.ResourceType)
	if resourceType == "" && len(scope.resolvedResourceTypes) > 0 {
		resourceType = scope.resolvedResourceTypes[0]
	}
	resourceName := strings.TrimSpace(alert.ResourceName)
	if resourceName == "" {
		resourceName = resourceID
	}
	severity := FindingSeverityWarning
	if strings.EqualFold(strings.TrimSpace(alert.Level), "critical") {
		severity = FindingSeverityCritical
	}
	title := strings.TrimSpace(alert.Message)
	if title == "" {
		title = fmt.Sprintf("%s alert on %s", alert.Type, resourceName)
	}
	evidence := fmt.Sprintf("Alert %s, level %s.", alert.Type, alert.Level)
	if alert.Value != 0 || alert.Threshold != 0 {
		// State alerts (offline, unhealthy) carry no reading.
		evidence += fmt.Sprintf(" Value %.1f, threshold %.1f.", alert.Value, alert.Threshold)
	}
	if !alert.StartTime.IsZero() {
		evidence += " Active since " + alert.StartTime.UTC().Format(time.RFC3339) + "."
	}

	finding := &Finding{
		ID:              id,
		Key:             normalizeFindingKey("alert-" + alert.Type),
		Severity:        severity,
		Category:        category,
		ResourceID:      resourceID,
		ResourceName:    resourceName,
		ResourceType:    resourceType,
		Title:           title,
		Description:     "Patrol looked at this alert and reported no finding of its own. Pulse opened this one from the alert so that it is investigated.",
		Evidence:        evidence,
		Source:          alertFindingSource,
		AlertIdentifier: alert.ID,
	}

	// The operator has already been told about the alert, so this finding is
	// stored without the notifications a new finding sends.
	p.findings.Add(finding)
	stored := p.findings.Get(id)
	if stored == nil {
		return nil
	}
	if p.unifiedFindingCallback != nil {
		p.unifiedFindingCallback(stored)
	}
	if !stored.IsActive() {
		// Dismissed or suppressed earlier. The operator's answer stands.
		return nil
	}
	log.Info().
		Str("finding_id", stored.ID).
		Str("alert_identifier", alert.ID).
		Str("resource", stored.ResourceName).
		Str("severity", string(stored.Severity)).
		Msg("AI Patrol: The run filed no finding for the alert that started it, opened one from the alert")
	return stored
}

// alertFindingCategory places an alert-opened finding in the category a
// finding about the same condition would have.
func alertFindingCategory(alertType string) FindingCategory {
	switch alertConditionClass(alertType) {
	case mirrorConditionDown, mirrorConditionRestartLoop, mirrorConditionContainerHealth:
		return FindingCategoryReliability
	case mirrorConditionDiskCapacity:
		return FindingCategoryCapacity
	case mirrorConditionMemory, mirrorConditionCPU, mirrorConditionTemperature:
		return FindingCategoryPerformance
	case mirrorConditionBackupAge, mirrorConditionSnapshotAge:
		return FindingCategoryBackup
	default:
		return FindingCategoryGeneral
	}
}

// patrolAlertSummaryName names an alert for a run summary line.
func patrolAlertSummaryName(alert *models.Alert) string {
	if alert == nil {
		return ""
	}
	name := strings.TrimSpace(alert.ResourceName)
	if name == "" {
		name = strings.TrimSpace(alert.ResourceID)
	}
	level := strings.ToLower(strings.TrimSpace(alert.Level))
	if level == "" {
		return fmt.Sprintf("alert on %s", name)
	}
	return fmt.Sprintf("%s alert on %s", level, name)
}

// patrolScopeActiveAlerts returns the alerts active right now on the resources
// a scoped run was asked about, most severe first. Alerts on resources that are
// in the run only as context, such as a container's host, are left out. A
// cleared-alert check and a run without named resources have none.
func (p *PatrolService) patrolScopeActiveAlerts(scope PatrolScope) []models.Alert {
	if p == nil || scope.Reason == TriggerReasonAlertCleared || len(scope.ResourceIDs) == 0 || !p.hasPatrolRuntimeInputs() {
		return nil
	}
	asked := make(map[string]bool, len(scope.ResourceIDs))
	for _, id := range scope.ResourceIDs {
		if id = strings.TrimSpace(id); id != "" {
			asked[id] = true
		}
	}
	state := p.currentPatrolRuntimeState()
	var active []models.Alert
	for _, alert := range state.ActiveAlerts {
		if patrolAlertBelongsToScope(state, alert.ResourceID, asked) {
			active = append(active, alert)
		}
	}
	sort.SliceStable(active, func(i, j int) bool {
		iCritical := strings.EqualFold(active[i].Level, "critical")
		jCritical := strings.EqualFold(active[j].Level, "critical")
		if iCritical != jCritical {
			return iCritical
		}
		return active[i].StartTime.Before(active[j].StartTime)
	})
	return active
}

// patrolAlertsSummaryName names the most severe of several alerts for a run
// summary line and says how many more there are.
func patrolAlertsSummaryName(alerts []models.Alert) string {
	if len(alerts) == 0 {
		return ""
	}
	name := patrolAlertSummaryName(&alerts[0])
	if more := len(alerts) - 1; more > 0 {
		name += fmt.Sprintf(" (and %d more)", more)
	}
	return name
}
