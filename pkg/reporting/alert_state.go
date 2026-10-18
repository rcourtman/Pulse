package reporting

import (
	"fmt"
	"strings"
)

// AlertResolutionMovedToAgent is the reason code of an alert that closed
// because a linked Pulse agent took its metric over
// (alerts.AlertResolutionMovedToAgent).
const AlertResolutionMovedToAgent = "moved_to_agent"

// Recovered reports whether the alert closed because its condition cleared.
// An open alert, or one that closed without recovering, is not recovered.
func (a AlertInfo) Recovered() bool {
	return a.ResolvedTime != nil && a.Resolution == nil
}

// ClosedWithoutRecovery reports whether the alert closed while its condition
// may still hold, such as a node alert handed to its Pulse agent.
func (a AlertInfo) ClosedWithoutRecovery() bool {
	return a.ResolvedTime != nil && a.Resolution != nil
}

// ResolutionOutcome is the short form of a close that was not a recovery,
// such as "moved to pve1 (Host Agent)". It is empty for an open alert and for
// an ordinary recovery.
func (a AlertInfo) ResolutionOutcome() string {
	if !a.ClosedWithoutRecovery() {
		return ""
	}
	if a.Resolution.Reason == AlertResolutionMovedToAgent {
		return "moved to " + a.successorLabel()
	}
	return "closed without recovering"
}

// ResolutionSummary is the alert engine's account of a close that was not a
// recovery, falling back to the short form when the engine supplied none. It
// is empty for an open alert and for an ordinary recovery.
func (a AlertInfo) ResolutionSummary() string {
	if !a.ClosedWithoutRecovery() {
		return ""
	}
	if summary := strings.TrimSpace(a.Resolution.Summary); summary != "" {
		return summary
	}
	return fmt.Sprintf("Alert %s. This is not a recovery.", a.ResolutionOutcome())
}

func (a AlertInfo) successorLabel() string {
	if a.Resolution != nil {
		if name := strings.TrimSpace(a.Resolution.SuccessorName); name != "" {
			return name
		}
	}
	return "its Pulse agent"
}

// unresolvedAlerts returns the alerts a report must treat as live: every open
// alert, and every alert that closed without recovering unless the report
// also shows its successor's own reading for that metric. That alert carries
// the condition's current state, so one condition is never counted twice and
// a handover never reads as a recovery.
func unresolvedAlerts(alerts []AlertInfo) []AlertInfo {
	return unresolvedAlertsAmong(alerts, alerts)
}

// unresolvedAlertsAmong is unresolvedAlerts with successors looked up in
// pool, so a fleet report matches a handover against every resource's alerts.
func unresolvedAlertsAmong(alerts, pool []AlertInfo) []AlertInfo {
	out := make([]AlertInfo, 0, len(alerts))
	for _, alert := range alerts {
		switch {
		case alert.ResolvedTime == nil:
			out = append(out, alert)
		case alert.ClosedWithoutRecovery() && successorAlertFor(alert, pool) == nil:
			out = append(out, alert)
		}
	}
	return out
}

// successorAlertFor returns the alert in pool that carries a moved alert's
// condition: one of the same type on its successor or one of the successor's
// children (an agent's filesystem alerts sit under "<agent>/disk:<mount>"),
// still open or recovered at or after the handover. A successor recovery from
// before the handover says nothing about the reading since, so it does not
// count.
func successorAlertFor(moved AlertInfo, pool []AlertInfo) *AlertInfo {
	if !moved.ClosedWithoutRecovery() {
		return nil
	}
	successor := strings.TrimSpace(moved.Resolution.SuccessorResourceID)
	if successor == "" {
		return nil
	}
	for i := range pool {
		candidate := &pool[i]
		if candidate.Type != moved.Type || candidate.ClosedWithoutRecovery() {
			continue
		}
		if candidate.ResolvedTime != nil && candidate.ResolvedTime.Before(*moved.ResolvedTime) {
			continue
		}
		resourceID := strings.TrimSpace(candidate.ResourceID)
		if resourceID == successor || strings.HasPrefix(resourceID, successor+"/") {
			return candidate
		}
	}
	return nil
}

// SuccessorAlertListed reports whether alerts shows the reading of the
// resource this alert moved to: an alert of the same type on the successor,
// open or recovered since the handover. The report then counts the condition
// once, through that alert. It is false for an open alert or a recovery.
func (a AlertInfo) SuccessorAlertListed(alerts []AlertInfo) bool {
	return successorAlertFor(a, alerts) != nil
}

// fleetAlerts is every alert listed across a fleet report's resources.
func (d *MultiReportData) fleetAlerts() []AlertInfo {
	if d == nil {
		return nil
	}
	var pool []AlertInfo
	for _, rd := range d.Resources {
		if rd != nil {
			pool = append(pool, rd.Alerts...)
		}
	}
	return pool
}

// alertHealth is the health verdict a report's alerts support. The PDF health
// card and the heuristic narrator both read it, so they never disagree.
type alertHealth struct {
	Status   string // HEALTHY, WARNING or CRITICAL
	Message  string
	Critical int
	Warning  int
	// Unresolved is every alert the report treats as live (unresolvedAlerts).
	Unresolved []AlertInfo
}

func assessAlertHealth(alerts []AlertInfo) alertHealth {
	health := alertHealth{Unresolved: unresolvedAlerts(alerts)}
	open := 0
	var moved []AlertInfo
	for _, alert := range health.Unresolved {
		if alert.Level == "critical" {
			health.Critical++
		} else {
			health.Warning++
		}
		if alert.ClosedWithoutRecovery() {
			moved = append(moved, alert)
		} else {
			open++
		}
	}

	switch {
	case health.Critical > 0:
		health.Status = "CRITICAL"
	case health.Warning > 0:
		health.Status = "WARNING"
	default:
		health.Status = "HEALTHY"
	}

	switch {
	case open == 0 && len(moved) > 0:
		health.Message = movedAlertsHealthMessage(moved)
	case health.Critical == 1:
		health.Message = "1 critical issue requires immediate attention"
	case health.Critical > 1:
		health.Message = fmt.Sprintf("%d critical issues require immediate attention", health.Critical)
	case health.Warning == 1:
		health.Message = "1 warning detected - review recommended"
	case health.Warning > 1:
		health.Message = fmt.Sprintf("%d warnings detected - review recommended", health.Warning)
	default:
		health.Message = "All systems operating normally"
	}
	return health
}

// movedAlertsHealthMessage says where alerts that closed without recovering
// went, so the report points at the resource that now holds the reading
// instead of calling this one healthy.
func movedAlertsHealthMessage(moved []AlertInfo) string {
	first := moved[0]
	for _, alert := range moved[1:] {
		if alert.ResolutionOutcome() != first.ResolutionOutcome() {
			return fmt.Sprintf("%d alerts closed without recovering - review recommended", len(moved))
		}
	}
	if first.Resolution.Reason != AlertResolutionMovedToAgent {
		if len(moved) == 1 {
			return "1 alert closed without recovering - review recommended"
		}
		return fmt.Sprintf("%d alerts closed without recovering - review recommended", len(moved))
	}
	if len(moved) == 1 {
		return fmt.Sprintf("Alert %s - check the agent for the current reading", first.ResolutionOutcome())
	}
	return fmt.Sprintf("%d alerts %s - check the agent for the current reading", len(moved), first.ResolutionOutcome())
}

// movedAlertSuccessors names, once each and in report order, the Pulse agents
// that unresolved alerts moved to.
func movedAlertSuccessors(unresolved []AlertInfo) []string {
	var successors []string
	seen := make(map[string]struct{})
	for _, alert := range unresolved {
		if !alert.ClosedWithoutRecovery() || alert.Resolution.Reason != AlertResolutionMovedToAgent {
			continue
		}
		successor := alert.successorLabel()
		if _, ok := seen[successor]; ok {
			continue
		}
		seen[successor] = struct{}{}
		successors = append(successors, successor)
	}
	return successors
}
