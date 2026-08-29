package alerts

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
)

// AlertQualitySnapshot is an identity-free aggregate of the alert lifecycle.
// It is safe to add across tenant managers. Canonical alert identity is used
// only inside CalculateAlertQualitySnapshot to fold local history occurrences.
type AlertQualitySnapshot struct {
	ActiveInfo                 int
	ActiveWarning              int
	ActiveCritical             int
	ActiveAgeUnder1h           int
	ActiveAge1h24h             int
	ActiveAge1d7d              int
	ActiveAge7dPlus            int
	Fired30d                   int
	FiredInfo30d               int
	FiredWarning30d            int
	FiredCritical30d           int
	Acknowledged30d            int
	Resolved30d                int
	ResolvedInfo30d            int
	ResolvedWarning30d         int
	ResolvedCritical30d        int
	ResolutionUnder15m30d      int
	Resolution15m1h30d         int
	Resolution1h24h30d         int
	Resolution1d7d30d          int
	Resolution7dPlus30d        int
	RepeatOccurrences30d       int
	SnoozedOccurrences30d      int
	ResolvedWhileSnoozed30d    int
	ManagerTenants             int
	DeliveryActiveTenants      int
	FlappingEnabledTenants     int
	IntentPolicyTenants        int
	EventHistoryHealthyTenants int
	ActiveStateHealthyTenants  int
	ActiveStateDegradedTenants int
}

// Add merges another tenant snapshot without retaining tenant identity.
func (snapshot *AlertQualitySnapshot) Add(other AlertQualitySnapshot) {
	snapshot.ActiveInfo += other.ActiveInfo
	snapshot.ActiveWarning += other.ActiveWarning
	snapshot.ActiveCritical += other.ActiveCritical
	snapshot.ActiveAgeUnder1h += other.ActiveAgeUnder1h
	snapshot.ActiveAge1h24h += other.ActiveAge1h24h
	snapshot.ActiveAge1d7d += other.ActiveAge1d7d
	snapshot.ActiveAge7dPlus += other.ActiveAge7dPlus
	snapshot.Fired30d += other.Fired30d
	snapshot.FiredInfo30d += other.FiredInfo30d
	snapshot.FiredWarning30d += other.FiredWarning30d
	snapshot.FiredCritical30d += other.FiredCritical30d
	snapshot.Acknowledged30d += other.Acknowledged30d
	snapshot.Resolved30d += other.Resolved30d
	snapshot.ResolvedInfo30d += other.ResolvedInfo30d
	snapshot.ResolvedWarning30d += other.ResolvedWarning30d
	snapshot.ResolvedCritical30d += other.ResolvedCritical30d
	snapshot.ResolutionUnder15m30d += other.ResolutionUnder15m30d
	snapshot.Resolution15m1h30d += other.Resolution15m1h30d
	snapshot.Resolution1h24h30d += other.Resolution1h24h30d
	snapshot.Resolution1d7d30d += other.Resolution1d7d30d
	snapshot.Resolution7dPlus30d += other.Resolution7dPlus30d
	snapshot.RepeatOccurrences30d += other.RepeatOccurrences30d
	snapshot.SnoozedOccurrences30d += other.SnoozedOccurrences30d
	snapshot.ResolvedWhileSnoozed30d += other.ResolvedWhileSnoozed30d
	snapshot.ManagerTenants += other.ManagerTenants
	snapshot.DeliveryActiveTenants += other.DeliveryActiveTenants
	snapshot.FlappingEnabledTenants += other.FlappingEnabledTenants
	snapshot.IntentPolicyTenants += other.IntentPolicyTenants
	snapshot.EventHistoryHealthyTenants += other.EventHistoryHealthyTenants
	snapshot.ActiveStateHealthyTenants += other.ActiveStateHealthyTenants
	snapshot.ActiveStateDegradedTenants += other.ActiveStateDegradedTenants
}

// AlertQualityTelemetrySnapshot returns the current manager's aggregate
// quality snapshot for a closed 30-day history window.
func (m *Manager) AlertQualityTelemetrySnapshot(now time.Time) AlertQualitySnapshot {
	if m == nil {
		return AlertQualitySnapshot{}
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	cutoff := now.Add(-30 * 24 * time.Hour)
	snapshot := CalculateAlertQualitySnapshot(
		m.GetAlertHistorySince(cutoff, 0),
		m.GetActiveAlerts(),
		cutoff,
		now,
	)
	snapshot.ManagerTenants = 1

	config := m.GetConfig()
	if config.Enabled && config.ActivationState == ActivationActive {
		snapshot.DeliveryActiveTenants = 1
	}
	if config.FlappingEnabled {
		snapshot.FlappingEnabledTenants = 1
	}
	intent := m.GetIntentPolicies()
	if intent.Revision > 0 || intent.UpdatedAt != nil || len(intent.Defaults) > 0 || len(intent.ResourceTypes) > 0 || len(intent.Resources) > 0 {
		snapshot.IntentPolicyTenants = 1
	}
	if m.eventLogStore() != nil && m.eventHistoryAuthoritative.Load() {
		snapshot.EventHistoryHealthyTenants = 1
	}
	if m.eventLogStore() != nil && m.activeStateAuthoritative.Load() {
		snapshot.ActiveStateHealthyTenants = 1
	}
	if m.activeStateDegraded() {
		snapshot.ActiveStateDegradedTenants = 1
	}
	return snapshot
}

type alertQualityOccurrence struct {
	alert        Alert
	acknowledged bool
	resolved     bool
}

// CalculateAlertQualitySnapshot folds local lifecycle rows into aggregate
// counts. The input may contain repeated snapshots for one occurrence.
func CalculateAlertQualitySnapshot(history, active []Alert, cutoff, now time.Time) AlertQualitySnapshot {
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	if cutoff.IsZero() {
		cutoff = now.Add(-30 * 24 * time.Hour)
	} else {
		cutoff = cutoff.UTC()
	}

	var snapshot AlertQualitySnapshot
	for _, alert := range active {
		addActiveAlertQuality(&snapshot, alert, now)
	}

	occurrences := make(map[string]alertQualityOccurrence, len(history))
	for index, alert := range history {
		identity := localAlertIdentity(alert)
		occurredAt := alert.StartTime
		if occurredAt.IsZero() && alert.OperationalRecord != nil {
			occurredAt = alert.OperationalRecord.FirstObservedAt
		}
		key := fmt.Sprintf("%s:%d", identity, occurredAt.UnixNano())
		if identity == "" {
			key = fmt.Sprintf("anonymous:%d", index)
		}
		current := occurrences[key]
		current.alert = alert
		if alert.AckTime != nil && !alert.AckTime.Before(cutoff) {
			current.acknowledged = true
		}
		if alert.OperationalRecord != nil && alert.OperationalRecord.State == operationaltrust.OperationalResolved {
			current.resolved = true
		}
		occurrences[key] = current
	}

	identities := make(map[string]int, len(occurrences))
	for _, occurrence := range occurrences {
		alert := occurrence.alert
		snapshot.Fired30d++
		addSeverityCount(alert.Level, &snapshot.FiredInfo30d, &snapshot.FiredWarning30d, &snapshot.FiredCritical30d)
		if occurrence.acknowledged {
			snapshot.Acknowledged30d++
		}
		identity := localAlertIdentity(alert)
		if identity != "" {
			identities[identity]++
		}
		if snoozed, resolvedWhileSnoozed := snoozeOutcome(alert); snoozed {
			snapshot.SnoozedOccurrences30d++
			if occurrence.resolved && resolvedWhileSnoozed {
				snapshot.ResolvedWhileSnoozed30d++
			}
		}
		if !occurrence.resolved {
			continue
		}
		snapshot.Resolved30d++
		addSeverityCount(alert.Level, &snapshot.ResolvedInfo30d, &snapshot.ResolvedWarning30d, &snapshot.ResolvedCritical30d)
		addResolutionDuration(&snapshot, alert)
	}
	for _, count := range identities {
		if count > 1 {
			snapshot.RepeatOccurrences30d += count - 1
		}
	}
	return snapshot
}

func localAlertIdentity(alert Alert) string {
	if identity := strings.TrimSpace(alert.CanonicalState); identity != "" {
		return identity
	}
	return strings.TrimSpace(alert.ID)
}

func addSeverityCount(level AlertLevel, info, warning, critical *int) {
	switch NormalizeAlertLevel(level) {
	case AlertLevelInfo:
		*info++
	case AlertLevelCritical:
		*critical++
	default:
		*warning++
	}
}

func addActiveAlertQuality(snapshot *AlertQualitySnapshot, alert Alert, now time.Time) {
	addSeverityCount(alert.Level, &snapshot.ActiveInfo, &snapshot.ActiveWarning, &snapshot.ActiveCritical)
	startedAt := alert.StartTime
	if startedAt.IsZero() && alert.OperationalRecord != nil {
		startedAt = alert.OperationalRecord.FirstObservedAt
	}
	age := now.Sub(startedAt)
	if startedAt.IsZero() || age < 0 {
		age = 0
	}
	switch {
	case age < time.Hour:
		snapshot.ActiveAgeUnder1h++
	case age < 24*time.Hour:
		snapshot.ActiveAge1h24h++
	case age < 7*24*time.Hour:
		snapshot.ActiveAge1d7d++
	default:
		snapshot.ActiveAge7dPlus++
	}
}

func addResolutionDuration(snapshot *AlertQualitySnapshot, alert Alert) {
	if alert.OperationalRecord == nil || alert.OperationalRecord.ResolvedAt == nil {
		return
	}
	startedAt := alert.StartTime
	if startedAt.IsZero() {
		startedAt = alert.OperationalRecord.FirstObservedAt
	}
	duration := alert.OperationalRecord.ResolvedAt.Sub(startedAt)
	if startedAt.IsZero() || duration < 0 {
		duration = 0
	}
	switch {
	case duration < 15*time.Minute:
		snapshot.ResolutionUnder15m30d++
	case duration < time.Hour:
		snapshot.Resolution15m1h30d++
	case duration < 24*time.Hour:
		snapshot.Resolution1h24h30d++
	case duration < 7*24*time.Hour:
		snapshot.Resolution1d7d30d++
	default:
		snapshot.Resolution7dPlus30d++
	}
}

func snoozeOutcome(alert Alert) (snoozed, resolvedWhileSnoozed bool) {
	transitions := append([]operationaltrust.LifecycleTransition(nil), alert.Transitions...)
	sort.SliceStable(transitions, func(i, j int) bool { return transitions[i].At.Before(transitions[j].At) })
	active := false
	for _, transition := range transitions {
		switch transition.Cause {
		case operationaltrust.TransitionSuppression:
			snoozed = true
			active = true
		case operationaltrust.TransitionSuppressionExpired:
			active = false
		case operationaltrust.TransitionRecoveryEvidence:
			resolvedWhileSnoozed = active
		}
	}
	return snoozed, resolvedWhileSnoozed
}
