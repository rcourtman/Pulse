package ai

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
	recoverymodel "github.com/rcourtman/pulse-go-rewrite/internal/recovery/model"
)

const (
	DefaultAttentionPageSize = 50
	MaxAttentionPageSize     = 200
)

type AttentionFilter string

const (
	AttentionFilterActive       AttentionFilter = "active"
	AttentionFilterOpen         AttentionFilter = "open"
	AttentionFilterAcknowledged AttentionFilter = "acknowledged"
	AttentionFilterSuppressed   AttentionFilter = "suppressed"
	AttentionFilterUncertain    AttentionFilter = "stale_unknown"
	AttentionFilterResolved     AttentionFilter = "resolved"
	AttentionFilterAll          AttentionFilter = "all"
)

func (filter AttentionFilter) Valid() bool {
	switch filter {
	case AttentionFilterActive,
		AttentionFilterOpen,
		AttentionFilterAcknowledged,
		AttentionFilterSuppressed,
		AttentionFilterUncertain,
		AttentionFilterResolved,
		AttentionFilterAll:
		return true
	default:
		return false
	}
}

type AttentionVerificationState string

const (
	AttentionVerificationNotAvailable AttentionVerificationState = "not_available"
	AttentionVerificationPending      AttentionVerificationState = "pending"
	AttentionVerificationSucceeded    AttentionVerificationState = "succeeded"
	AttentionVerificationFailed       AttentionVerificationState = "failed"
	AttentionVerificationUnknown      AttentionVerificationState = "unknown"
)

type AttentionActionOffer struct {
	ActionID              string   `json:"actionId,omitempty"`
	TargetResourceID      string   `json:"targetResourceId"`
	Capability            string   `json:"capability"`
	Kind                  string   `json:"kind"`
	Label                 string   `json:"label"`
	Mode                  string   `json:"mode"`
	Risk                  string   `json:"risk"`
	Approval              string   `json:"approval"`
	Eligibility           string   `json:"eligibility"`
	Reasons               []string `json:"reasons"`
	EvidenceIDs           []string `json:"evidenceIds"`
	ExpectedPostcondition string   `json:"expectedPostcondition"`
	VerificationPolicy    string   `json:"verificationPolicy"`
	RequiresApproval      bool     `json:"requiresApproval"`
}

type AttentionResource struct {
	ResourceID string `json:"resourceId"`
}

// AttentionFlapping summarises an item whose lifecycle keeps moving between
// open and resolved. It uses the same window and threshold as finding
// flapping (findings_storm_throttler.go) so alerts and Patrol findings agree
// on the label. The full transition list stays in the detail timeline; this
// is the collapsed operator-facing summary.
type AttentionFlapping struct {
	TransitionCount   int       `json:"transitionCount"`
	WindowHours       int       `json:"windowHours"`
	FirstTransitionAt time.Time `json:"firstTransitionAt"`
	LastTransitionAt  time.Time `json:"lastTransitionAt"`
}

type AttentionItem struct {
	ID                   string                                `json:"id"`
	OperationalRecordID  string                                `json:"operationalRecordId"`
	SubjectResourceID    string                                `json:"subjectResourceId"`
	SubjectResourceName  string                                `json:"subjectResourceName"`
	SubjectResourceType  string                                `json:"subjectResourceType,omitempty"`
	Kind                 string                                `json:"kind"`
	Title                string                                `json:"title"`
	PlainLanguageSummary string                                `json:"plainLanguageSummary"`
	Severity             operationaltrust.OperationalSeverity  `json:"severity"`
	State                operationaltrust.OperationalState     `json:"state"`
	FirstObservedAt      time.Time                             `json:"firstObservedAt"`
	LastObservedAt       time.Time                             `json:"lastObservedAt"`
	EvidenceFreshness    operationaltrust.EvidenceFreshness    `json:"evidenceFreshness"`
	EvidenceCompleteness operationaltrust.EvidenceCompleteness `json:"evidenceCompleteness"`
	Impact               string                                `json:"impact,omitempty"`
	ProtectionPosture    *recoverymodel.ProtectionPosture      `json:"protectionPosture,omitempty"`
	RelatedResources     []AttentionResource                   `json:"relatedResources"`
	RecommendedNextStep  string                                `json:"recommendedNextStep,omitempty"`
	AvailableActions     []AttentionActionOffer                `json:"availableActions"`
	VerificationState    AttentionVerificationState            `json:"verificationState"`
	Flapping             *AttentionFlapping                    `json:"flapping,omitempty"`
}

type AttentionItemDetail struct {
	Item              AttentionItem                          `json:"item"`
	OperationalRecord operationaltrust.OperationalRecord     `json:"operationalRecord"`
	Timeline          []operationaltrust.LifecycleTransition `json:"timeline"`
	Evidence          []operationaltrust.EvidenceEnvelope    `json:"evidence"`
}

type AttentionSummary struct {
	ActiveCount       int       `json:"activeCount"`
	OpenCount         int       `json:"openCount"`
	AcknowledgedCount int       `json:"acknowledgedCount"`
	SuppressedCount   int       `json:"suppressedCount"`
	UncertainCount    int       `json:"uncertainCount"`
	ResolvedCount     int       `json:"resolvedCount"`
	Calm              bool      `json:"calm"`
	CoverageState     string    `json:"coverageState"`
	EvaluatedAt       time.Time `json:"evaluatedAt"`
}

type AttentionProjection struct {
	Details []AttentionItemDetail `json:"details"`
	Summary AttentionSummary      `json:"summary"`
}

// ProjectAttentionItems is the single Patrol read-model projection over the
// canonical alert lifecycle. It does not inspect loose alert metadata to
// invent work and it never creates a second writable lifecycle.
func ProjectAttentionItems(
	active []alerts.Alert,
	history []alerts.Alert,
	postures map[string]recoverymodel.ProtectionPosture,
	now time.Time,
) AttentionProjection {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	now = now.UTC()

	records := make(map[string]alerts.Alert, len(active)+len(history))
	for _, alert := range history {
		addAttentionAlert(records, alert, false)
	}
	for _, alert := range active {
		addAttentionAlert(records, alert, true)
	}

	details := make([]AttentionItemDetail, 0, len(records))
	for _, alert := range records {
		detail, ok := projectAttentionAlert(alert, postures, now)
		if !ok {
			continue
		}
		details = append(details, detail)
	}
	sort.SliceStable(details, func(i, j int) bool {
		return attentionItemLess(details[i].Item, details[j].Item)
	})

	return AttentionProjection{
		Details: details,
		Summary: summarizeAttention(details, now),
	}
}

func addAttentionAlert(records map[string]alerts.Alert, alert alerts.Alert, active bool) {
	if alert.OperationalRecord == nil {
		return
	}
	if !active && alert.OperationalRecord.State != operationaltrust.OperationalResolved {
		return
	}
	recordID := strings.TrimSpace(alert.OperationalRecord.ID)
	if recordID == "" {
		return
	}
	existing, found := records[recordID]
	if !found || active || attentionAlertNewer(alert, existing) {
		records[recordID] = *alert.Clone()
	}
}

func attentionAlertNewer(candidate, existing alerts.Alert) bool {
	if candidate.OperationalRecord == nil {
		return false
	}
	if existing.OperationalRecord == nil {
		return true
	}
	if !candidate.OperationalRecord.StateChangedAt.Equal(existing.OperationalRecord.StateChangedAt) {
		return candidate.OperationalRecord.StateChangedAt.After(existing.OperationalRecord.StateChangedAt)
	}
	return candidate.OperationalRecord.LastObservedAt.After(existing.OperationalRecord.LastObservedAt)
}

func projectAttentionAlert(
	alert alerts.Alert,
	postures map[string]recoverymodel.ProtectionPosture,
	now time.Time,
) (AttentionItemDetail, bool) {
	if alert.OperationalRecord == nil {
		return AttentionItemDetail{}, false
	}
	record := alert.OperationalRecord.Clone()
	if err := record.Validate(); err != nil {
		return AttentionItemDetail{}, false
	}

	evidence := cloneAttentionEvidence(alert.Evidence)
	timeline := cloneAttentionTimeline(alert.Transitions)
	freshness, completeness := summarizeAttentionEvidence(record.State, evidence, now)
	resourceName := firstAttentionText(alert.ResourceName, alert.Instance, record.SubjectResourceID)
	title := attentionTitle(alert, resourceName)
	summary := firstAttentionText(alert.Message, record.ImpactSummary, title)

	related := make([]AttentionResource, 0, len(record.RelatedResourceIDs))
	for _, resourceID := range canonicalAttentionIDs(record.RelatedResourceIDs) {
		related = append(related, AttentionResource{ResourceID: resourceID})
	}

	var posture *recoverymodel.ProtectionPosture
	if candidate, found := postures[record.SubjectResourceID]; found {
		value := candidate.Clone()
		posture = &value
	}

	item := AttentionItem{
		ID:                   record.ID,
		OperationalRecordID:  record.ID,
		SubjectResourceID:    record.SubjectResourceID,
		SubjectResourceName:  resourceName,
		SubjectResourceType:  attentionResourceType(alert),
		Kind:                 strings.TrimSpace(alert.Type),
		Title:                title,
		PlainLanguageSummary: summary,
		Severity:             record.Severity,
		State:                record.State,
		FirstObservedAt:      record.FirstObservedAt,
		LastObservedAt:       record.LastObservedAt,
		EvidenceFreshness:    freshness,
		EvidenceCompleteness: completeness,
		Impact:               record.ImpactSummary,
		ProtectionPosture:    posture,
		RelatedResources:     related,
		RecommendedNextStep:  record.RecommendedNextStep,
		AvailableActions:     []AttentionActionOffer{},
		VerificationState:    AttentionVerificationNotAvailable,
		Flapping:             attentionFlapping(timeline, now),
	}
	return AttentionItemDetail{
		Item:              item,
		OperationalRecord: record,
		Timeline:          timeline,
		Evidence:          evidence,
	}, true
}

// attentionFlapping collapses open/resolved churn in the lifecycle timeline
// into one summary when it crosses the shared flapping threshold. Only
// transitions that open or resolve the record count; acknowledgement,
// suppression, and evidence refreshes are operator or detector bookkeeping,
// not the condition coming and going.
func attentionFlapping(
	timeline []operationaltrust.LifecycleTransition,
	now time.Time,
) *AttentionFlapping {
	transitions := make([]time.Time, 0, len(timeline))
	for _, transition := range timeline {
		if !attentionTransitionIsFlap(transition) {
			continue
		}
		transitions = append(transitions, transition.At)
	}
	summary := summarizeFlapTransitions(transitions, now)
	if summary == nil {
		return nil
	}
	return &AttentionFlapping{
		TransitionCount:   summary.TransitionCount,
		WindowHours:       summary.WindowHours,
		FirstTransitionAt: summary.FirstTransitionAt,
		LastTransitionAt:  summary.LastTransitionAt,
	}
}

func attentionTransitionIsFlap(transition operationaltrust.LifecycleTransition) bool {
	if transition.To == operationaltrust.OperationalResolved {
		return transition.From != operationaltrust.OperationalResolved
	}
	if transition.To == operationaltrust.OperationalOpen {
		return transition.From == operationaltrust.OperationalResolved
	}
	return false
}

func summarizeAttentionEvidence(
	state operationaltrust.OperationalState,
	evidence []operationaltrust.EvidenceEnvelope,
	now time.Time,
) (operationaltrust.EvidenceFreshness, operationaltrust.EvidenceCompleteness) {
	if state == operationaltrust.OperationalStale {
		return operationaltrust.EvidenceStale, worstAttentionCompleteness(evidence)
	}
	if state == operationaltrust.OperationalUnknown {
		return operationaltrust.EvidenceFreshnessUnknown, worstAttentionCompleteness(evidence)
	}
	if len(evidence) == 0 {
		return operationaltrust.EvidenceFreshnessUnknown, operationaltrust.EvidenceUnavailable
	}

	freshness := operationaltrust.EvidenceFresh
	completeness := operationaltrust.EvidenceComplete
	for _, envelope := range evidence {
		switch envelope.FreshnessAt(now) {
		case operationaltrust.EvidenceStale:
			freshness = operationaltrust.EvidenceStale
		case operationaltrust.EvidenceFreshnessUnknown:
			if freshness != operationaltrust.EvidenceStale {
				freshness = operationaltrust.EvidenceFreshnessUnknown
			}
		}
		switch envelope.Completeness {
		case operationaltrust.EvidenceUnavailable:
			completeness = operationaltrust.EvidenceUnavailable
		case operationaltrust.EvidencePartial:
			if completeness != operationaltrust.EvidenceUnavailable {
				completeness = operationaltrust.EvidencePartial
			}
		}
	}
	return freshness, completeness
}

func worstAttentionCompleteness(
	evidence []operationaltrust.EvidenceEnvelope,
) operationaltrust.EvidenceCompleteness {
	if len(evidence) == 0 {
		return operationaltrust.EvidenceUnavailable
	}
	completeness := operationaltrust.EvidenceComplete
	for _, envelope := range evidence {
		if envelope.Completeness == operationaltrust.EvidenceUnavailable {
			return operationaltrust.EvidenceUnavailable
		}
		if envelope.Completeness == operationaltrust.EvidencePartial {
			completeness = operationaltrust.EvidencePartial
		}
	}
	return completeness
}

func FilterAttentionDetails(
	details []AttentionItemDetail,
	filter AttentionFilter,
) ([]AttentionItemDetail, error) {
	if filter == "" {
		filter = AttentionFilterActive
	}
	if !filter.Valid() {
		return nil, fmt.Errorf("invalid attention filter %q", filter)
	}
	filtered := make([]AttentionItemDetail, 0, len(details))
	for _, detail := range details {
		if attentionStateMatches(detail.Item.State, filter) {
			filtered = append(filtered, detail)
		}
	}
	return filtered, nil
}

func PaginateAttentionDetails(
	details []AttentionItemDetail,
	page int,
	limit int,
) ([]AttentionItemDetail, error) {
	if page < 1 {
		return nil, errors.New("attention page must be at least one")
	}
	if limit < 1 || limit > MaxAttentionPageSize {
		return nil, fmt.Errorf("attention limit must be between 1 and %d", MaxAttentionPageSize)
	}
	start := (page - 1) * limit
	if start >= len(details) {
		return []AttentionItemDetail{}, nil
	}
	end := start + limit
	if end > len(details) {
		end = len(details)
	}
	return append([]AttentionItemDetail(nil), details[start:end]...), nil
}

func attentionStateMatches(state operationaltrust.OperationalState, filter AttentionFilter) bool {
	switch filter {
	case AttentionFilterActive:
		return attentionStateCountsAsActive(state)
	case AttentionFilterOpen:
		return state == operationaltrust.OperationalOpen ||
			state == operationaltrust.OperationalObserving ||
			state == operationaltrust.OperationalResolving
	case AttentionFilterAcknowledged:
		return state == operationaltrust.OperationalAcknowledged
	case AttentionFilterSuppressed:
		return state == operationaltrust.OperationalSuppressed
	case AttentionFilterUncertain:
		return state == operationaltrust.OperationalStale ||
			state == operationaltrust.OperationalUnknown
	case AttentionFilterResolved:
		return state == operationaltrust.OperationalResolved
	case AttentionFilterAll:
		return true
	default:
		return false
	}
}

func attentionStateCountsAsActive(state operationaltrust.OperationalState) bool {
	switch state {
	case operationaltrust.OperationalObserving,
		operationaltrust.OperationalOpen,
		operationaltrust.OperationalResolving,
		operationaltrust.OperationalStale,
		operationaltrust.OperationalUnknown:
		return true
	default:
		return false
	}
}

func summarizeAttention(details []AttentionItemDetail, now time.Time) AttentionSummary {
	summary := AttentionSummary{
		CoverageState: "current",
		EvaluatedAt:   now,
	}
	for _, detail := range details {
		switch detail.Item.State {
		case operationaltrust.OperationalAcknowledged:
			summary.AcknowledgedCount++
		case operationaltrust.OperationalSuppressed:
			summary.SuppressedCount++
		case operationaltrust.OperationalStale, operationaltrust.OperationalUnknown:
			summary.UncertainCount++
			summary.ActiveCount++
		case operationaltrust.OperationalResolved:
			summary.ResolvedCount++
		default:
			if attentionStateCountsAsActive(detail.Item.State) {
				summary.ActiveCount++
				summary.OpenCount++
			}
		}
	}
	summary.Calm = summary.ActiveCount == 0
	return summary
}

func attentionItemLess(left, right AttentionItem) bool {
	if l, r := attentionSeverityRank(left.Severity), attentionSeverityRank(right.Severity); l != r {
		return l > r
	}
	if l, r := len(left.RelatedResources), len(right.RelatedResources); l != r {
		return l > r
	}
	if l, r := attentionProtectionRank(left.ProtectionPosture), attentionProtectionRank(right.ProtectionPosture); l != r {
		return l > r
	}
	if l, r := attentionFreshnessRank(left.EvidenceFreshness), attentionFreshnessRank(right.EvidenceFreshness); l != r {
		return l > r
	}
	if !left.FirstObservedAt.Equal(right.FirstObservedAt) {
		return left.FirstObservedAt.Before(right.FirstObservedAt)
	}
	return left.ID < right.ID
}

func attentionSeverityRank(severity operationaltrust.OperationalSeverity) int {
	switch severity {
	case operationaltrust.SeverityCritical:
		return 3
	case operationaltrust.SeverityWarning:
		return 2
	case operationaltrust.SeverityInfo:
		return 1
	default:
		return 0
	}
}

func attentionProtectionRank(posture *recoverymodel.ProtectionPosture) int {
	if posture == nil {
		return 2
	}
	switch posture.State {
	case recoverymodel.ProtectionStateAttention:
		return 4
	case recoverymodel.ProtectionStateUnprotected:
		return 3
	case recoverymodel.ProtectionStateUnknown:
		return 2
	default:
		return 0
	}
}

func attentionFreshnessRank(freshness operationaltrust.EvidenceFreshness) int {
	switch freshness {
	case operationaltrust.EvidenceFresh:
		return 2
	case operationaltrust.EvidenceStale:
		return 1
	default:
		return 0
	}
}

func attentionTitle(alert alerts.Alert, resourceName string) string {
	alertType := strings.TrimSpace(alert.Type)
	if alertType == "" {
		return "Issue on " + resourceName
	}
	if incidentTitle := attentionIncidentTitle(alertType, alert.Message); incidentTitle != "" {
		return incidentTitle + " on " + resourceName
	}
	switch strings.ToLower(alertType) {
	case "usage":
		if strings.EqualFold(attentionResourceType(alert), "storage") {
			return "Storage usage on " + resourceName
		}
		return "Resource usage on " + resourceName
	case "cpu":
		return "CPU usage on " + resourceName
	case "disk":
		return "Disk usage on " + resourceName
	case "memory":
		return "Memory usage on " + resourceName
	}
	words := strings.Fields(strings.NewReplacer("-", " ", "_", " ").Replace(alertType))
	for i := range words {
		word := strings.ToLower(words[i])
		switch word {
		case "api", "cpu", "dns", "ip", "vm", "zfs":
			words[i] = strings.ToUpper(word)
		case "io":
			words[i] = "I/O"
		default:
			words[i] = word
			if i == 0 {
				words[i] = strings.ToUpper(word[:1]) + word[1:]
			}
		}
	}
	return strings.Join(words, " ") + " on " + resourceName
}

func attentionIncidentTitle(alertType, message string) string {
	normalizedType := strings.NewReplacer("-", "_", " ", "_").Replace(
		strings.ToLower(strings.TrimSpace(alertType)),
	)
	if normalizedType != "resource_incident" && normalizedType != "storage_incident" {
		return ""
	}
	if strings.Contains(strings.ToLower(message), "pg degraded") {
		return "Ceph placement groups degraded"
	}

	const vmwareAlarmMarker = " has VMware alarm "
	if markerIndex := strings.Index(message, vmwareAlarmMarker); markerIndex >= 0 {
		issue := strings.TrimSpace(message[markerIndex+len(vmwareAlarmMarker):])
		for _, suffix := range []string{" (", ". Affects "} {
			if suffixIndex := strings.Index(issue, suffix); suffixIndex >= 0 {
				issue = issue[:suffixIndex]
			}
		}
		if issue = strings.TrimSpace(strings.TrimSuffix(issue, ".")); issue != "" {
			return issue
		}
	}
	if strings.Contains(message, " has VMware overall status ") {
		if normalizedType == "storage_incident" {
			return "VMware datastore health"
		}
		return "VMware host health"
	}
	if normalizedType == "storage_incident" {
		return "Storage issue"
	}
	return "Infrastructure issue"
}

func attentionResourceType(alert alerts.Alert) string {
	if alert.Metadata != nil {
		if value, ok := alert.Metadata["resourceType"].(string); ok {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstAttentionText(values ...string) string {
	for _, value := range values {
		if normalized := strings.TrimSpace(value); normalized != "" {
			return normalized
		}
	}
	return ""
}

func canonicalAttentionIDs(values []string) []string {
	unique := make(map[string]struct{}, len(values))
	for _, value := range values {
		if normalized := strings.TrimSpace(value); normalized != "" {
			unique[normalized] = struct{}{}
		}
	}
	result := make([]string, 0, len(unique))
	for value := range unique {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func cloneAttentionEvidence(
	values []operationaltrust.EvidenceEnvelope,
) []operationaltrust.EvidenceEnvelope {
	result := make([]operationaltrust.EvidenceEnvelope, len(values))
	for i := range values {
		result[i] = values[i].Clone()
	}
	sort.Slice(result, func(i, j int) bool {
		if !result[i].ObservedAt.Equal(result[j].ObservedAt) {
			return result[i].ObservedAt.Before(result[j].ObservedAt)
		}
		return result[i].ID < result[j].ID
	})
	return result
}

func cloneAttentionTimeline(
	values []operationaltrust.LifecycleTransition,
) []operationaltrust.LifecycleTransition {
	result := make([]operationaltrust.LifecycleTransition, len(values))
	for i := range values {
		result[i] = values[i].Clone()
	}
	sort.Slice(result, func(i, j int) bool {
		if !result[i].At.Equal(result[j].At) {
			return result[i].At.Before(result[j].At)
		}
		return result[i].ID < result[j].ID
	})
	return result
}
