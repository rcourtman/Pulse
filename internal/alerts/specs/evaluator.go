package specs

import (
	"slices"
	"time"
)

type EvaluationTransitionKind string

const (
	EvaluationTransitionPending         EvaluationTransitionKind = "pending"
	EvaluationTransitionActivated       EvaluationTransitionKind = "activated"
	EvaluationTransitionRecovered       EvaluationTransitionKind = "recovered"
	EvaluationTransitionSeverityChanged EvaluationTransitionKind = "severity-changed"
	EvaluationTransitionSuppressed      EvaluationTransitionKind = "suppressed"
	EvaluationTransitionDisabled        EvaluationTransitionKind = "disabled"
	EvaluationTransitionReevaluated     EvaluationTransitionKind = "reevaluated"
)

type EvaluatorState struct {
	SpecID             string        `json:"specId"`
	SpecFingerprint    string        `json:"specFingerprint,omitempty"`
	State              AlertState    `json:"state"`
	Severity           AlertSeverity `json:"severity,omitempty"`
	Reason             string        `json:"reason,omitempty"`
	ConsecutiveMatches int           `json:"consecutiveMatches,omitempty"`
	FirstMatchedAt     time.Time     `json:"firstMatchedAt,omitempty"`
	ActiveSince        time.Time     `json:"activeSince,omitempty"`
	LastObservedAt     time.Time     `json:"lastObservedAt,omitempty"`
}

type EvaluationTransition struct {
	Kind             EvaluationTransitionKind `json:"kind"`
	SpecID           string                   `json:"specId"`
	ResourceID       string                   `json:"resourceId"`
	From             AlertState               `json:"from"`
	To               AlertState               `json:"to"`
	At               time.Time                `json:"at"`
	PreviousSeverity AlertSeverity            `json:"previousSeverity,omitempty"`
	Severity         AlertSeverity            `json:"severity,omitempty"`
	Reason           string                   `json:"reason,omitempty"`
	Evidence         AlertEvidence            `json:"evidence"`
}

type EvaluationResult struct {
	Previous   EvaluatorState        `json:"previous"`
	State      EvaluatorState        `json:"state"`
	Transition *EvaluationTransition `json:"transition,omitempty"`
}

// ValidateEvidence reports whether the evidence carries the payload the
// spec kind requires.
func ValidateEvidence(kind AlertSpecKind, evidence AlertEvidence) error {
	return evidence.validateForKind(kind)
}

// Match reports whether one observation matches a spec's trigger
// condition, with the severity and reason the match carries. Transition
// state ownership lives in internal/alerts/reducer.
func Match(spec ResourceAlertSpec, evidence AlertEvidence) (bool, AlertSeverity, string) {
	return matches(spec, evidence)
}

func matches(spec ResourceAlertSpec, evidence AlertEvidence) (bool, AlertSeverity, string) {
	switch spec.Kind {
	case AlertSpecKindSeverityThreshold:
		if evidence.SeverityThreshold == nil || spec.SeverityThreshold == nil {
			return false, "", ""
		}
		return matchesSeverityThreshold(*spec.SeverityThreshold, *evidence.SeverityThreshold)
	case AlertSpecKindChangeThreshold:
		if evidence.ChangeThreshold == nil || spec.ChangeThreshold == nil {
			return false, "", ""
		}
		return matchesChangeThreshold(*spec.ChangeThreshold, *evidence.ChangeThreshold)
	case AlertSpecKindBaselineAnomaly:
		if evidence.BaselineAnomaly == nil || spec.BaselineAnomaly == nil {
			return false, "", ""
		}
		return matchesBaselineAnomaly(*spec.BaselineAnomaly, *evidence.BaselineAnomaly)
	case AlertSpecKindHealthAssessment:
		if evidence.HealthAssessment == nil || spec.HealthAssessment == nil {
			return false, "", ""
		}
		return matchesHealthAssessment(*spec.HealthAssessment, *evidence.HealthAssessment)
	case AlertSpecKindPostureThreshold:
		if evidence.PostureThreshold == nil || spec.PostureThreshold == nil {
			return false, "", ""
		}
		return matchesPostureThreshold(*spec.PostureThreshold, *evidence.PostureThreshold)
	case AlertSpecKindConnectivity:
		return evidence.Connectivity != nil && !evidence.Connectivity.Connected, spec.Severity, "connectivity-lost"
	case AlertSpecKindPoweredState:
		if evidence.PoweredState == nil {
			return false, "", ""
		}
		return evidence.PoweredState.Observed != evidence.PoweredState.Expected, spec.Severity, "powered-state-mismatch"
	case AlertSpecKindDiscreteState:
		if evidence.DiscreteState == nil || spec.DiscreteState == nil {
			return false, "", ""
		}
		return slices.Contains(canonicalStringSet(spec.DiscreteState.TriggerStates), evidence.DiscreteState.Observed), spec.Severity, "discrete-state-match"
	case AlertSpecKindServiceGap:
		if evidence.ServiceGap == nil || spec.ServiceGap == nil {
			return false, "", ""
		}
		if evidence.ServiceGap.Desired > 0 {
			missing := evidence.ServiceGap.Desired - evidence.ServiceGap.Running
			if missing < 0 {
				missing = 0
			}
			percent := (float64(missing) / float64(evidence.ServiceGap.Desired)) * 100
			switch {
			case spec.ServiceGap.CriticalPercent > 0 && percent >= spec.ServiceGap.CriticalPercent:
				return true, AlertSeverityCritical, "service-gap-critical"
			case spec.ServiceGap.WarningPercent > 0 && percent >= spec.ServiceGap.WarningPercent:
				return true, AlertSeverityWarning, "service-gap-warning"
			default:
				return false, "", "service-gap-normal"
			}
		}
		return evidence.ServiceGap.MissingFor >= spec.ServiceGap.GapAfter && spec.ServiceGap.GapAfter > 0, spec.Severity, "service-gap-duration"
	case AlertSpecKindProviderIncident:
		if evidence.ProviderIncident == nil || spec.ProviderIncident == nil {
			return false, "", ""
		}
		if evidence.ProviderIncident.Provider != spec.ProviderIncident.Provider {
			return false, "", "provider-mismatch"
		}
		if len(spec.ProviderIncident.Codes) > 0 && !slices.Contains(spec.ProviderIncident.Codes, evidence.ProviderIncident.Code) {
			return false, "", "provider-code-mismatch"
		}
		if len(spec.ProviderIncident.NativeIDs) > 0 && !slices.Contains(spec.ProviderIncident.NativeIDs, evidence.ProviderIncident.NativeID) {
			return false, "", "provider-native-id-mismatch"
		}
		return true, spec.Severity, "provider-incident"
	case AlertSpecKindResourceIncidentRollup:
		if evidence.ResourceIncidentRollup == nil || spec.ResourceIncidentRollup == nil {
			return false, "", ""
		}
		return evidence.ResourceIncidentRollup.IncidentCount > 0 && evidence.ResourceIncidentRollup.Code == spec.ResourceIncidentRollup.Code, spec.Severity, "resource-incident-rollup"
	default:
		return false, "", ""
	}
}

func matchesSeverityThreshold(spec SeverityThresholdSpec, evidence SeverityThresholdEvidence) (bool, AlertSeverity, string) {
	if evidence.Direction != spec.Direction || evidence.Metric != spec.Metric {
		return false, "", ""
	}

	switch spec.Direction {
	case ThresholdDirectionAbove:
		switch {
		case spec.Critical > 0 && evidence.Observed >= spec.Critical:
			return true, AlertSeverityCritical, "severity-threshold-critical"
		case spec.Warning > 0 && evidence.Observed >= spec.Warning:
			return true, AlertSeverityWarning, "severity-threshold-warning"
		default:
			return false, "", "severity-threshold-normal"
		}
	case ThresholdDirectionBelow:
		switch {
		case spec.Critical > 0 && evidence.Observed <= spec.Critical:
			return true, AlertSeverityCritical, "severity-threshold-critical"
		case spec.Warning > 0 && evidence.Observed <= spec.Warning:
			return true, AlertSeverityWarning, "severity-threshold-warning"
		default:
			return false, "", "severity-threshold-normal"
		}
	default:
		return false, "", ""
	}
}

func matchesChangeThreshold(spec ChangeThresholdSpec, evidence ChangeThresholdEvidence) (bool, AlertSeverity, string) {
	if evidence.Metric != spec.Metric {
		return false, "", ""
	}

	if spec.CriticalCurrent > 0 && evidence.Observed >= spec.CriticalCurrent {
		return true, AlertSeverityCritical, "change-threshold-current-critical"
	}
	if spec.WarningCurrent > 0 && evidence.Observed >= spec.WarningCurrent {
		return true, AlertSeverityWarning, "change-threshold-current-warning"
	}

	if evidence.PreviousObserved == nil || *evidence.PreviousObserved <= 0 {
		return false, "", "change-threshold-normal"
	}

	delta := evidence.Observed - *evidence.PreviousObserved
	percent := (delta / *evidence.PreviousObserved) * 100

	if spec.CriticalDelta > 0 && delta >= spec.CriticalDelta {
		if spec.CriticalPercent <= 0 || percent >= spec.CriticalPercent {
			return true, AlertSeverityCritical, "change-threshold-growth-critical"
		}
	}
	if spec.WarningDelta > 0 && delta >= spec.WarningDelta {
		if spec.WarningPercent <= 0 || percent >= spec.WarningPercent {
			return true, AlertSeverityWarning, "change-threshold-growth-warning"
		}
	}

	return false, "", "change-threshold-normal"
}

func matchesBaselineAnomaly(spec BaselineAnomalySpec, evidence BaselineAnomalyEvidence) (bool, AlertSeverity, string) {
	if evidence.Metric != spec.Metric {
		return false, "", ""
	}

	baseline := evidence.Baseline
	if baseline == 0 && evidence.Observed > 0 {
		baseline = 1
	}

	if baseline < spec.QuietBaseline {
		delta := evidence.Observed - baseline
		switch {
		case delta >= spec.QuietCriticalDelta:
			return true, AlertSeverityCritical, "baseline-anomaly-quiet-critical"
		case delta >= spec.QuietWarningDelta:
			return true, AlertSeverityWarning, "baseline-anomaly-quiet-warning"
		default:
			return false, "", "baseline-anomaly-normal"
		}
	}

	if baseline <= 0 {
		return false, "", "baseline-anomaly-normal"
	}

	ratio := evidence.Observed / baseline
	delta := evidence.Observed - baseline
	switch {
	case spec.CriticalRatio > 0 && ratio >= spec.CriticalRatio && delta >= spec.CriticalDelta:
		return true, AlertSeverityCritical, "baseline-anomaly-critical"
	case spec.WarningRatio > 0 && ratio >= spec.WarningRatio && delta >= spec.WarningDelta:
		return true, AlertSeverityWarning, "baseline-anomaly-warning"
	default:
		return false, "", "baseline-anomaly-normal"
	}
}

func matchesHealthAssessment(spec HealthAssessmentSpec, evidence HealthAssessmentEvidence) (bool, AlertSeverity, string) {
	if evidence.Signal != spec.Signal {
		return false, "", "health-assessment-signal-mismatch"
	}
	if len(evidence.Codes) == 0 || evidence.Severity == "" {
		return false, "", "health-assessment-normal"
	}
	if len(spec.Codes) == 0 {
		return true, evidence.Severity, "health-assessment-match"
	}

	expected := canonicalStringSet(spec.Codes)
	observed := canonicalStringSet(evidence.Codes)
	for _, code := range observed {
		if slices.Contains(expected, code) {
			return true, evidence.Severity, "health-assessment-match"
		}
	}

	return false, "", "health-assessment-normal"
}

func matchesPostureThreshold(spec PostureThresholdSpec, evidence PostureThresholdEvidence) (bool, AlertSeverity, string) {
	ageCritical := false
	ageWarning := false
	sizeCritical := false
	sizeWarning := false

	if spec.AgeMetric != "" {
		if evidence.AgeMetric != spec.AgeMetric {
			return false, "", "posture-threshold-age-metric-mismatch"
		}
		ageCritical = spec.CriticalAge > 0 && evidence.AgeValue >= spec.CriticalAge
		ageWarning = !ageCritical && spec.WarningAge > 0 && evidence.AgeValue >= spec.WarningAge
	}

	if spec.SizeMetric != "" {
		if evidence.SizeMetric != spec.SizeMetric {
			return false, "", "posture-threshold-size-metric-mismatch"
		}
		if evidence.SizeValue == nil {
			return false, "", "posture-threshold-size-missing"
		}
		sizeCritical = spec.CriticalSize > 0 && *evidence.SizeValue >= spec.CriticalSize
		sizeWarning = !sizeCritical && spec.WarningSize > 0 && *evidence.SizeValue >= spec.WarningSize
	}

	switch {
	case ageCritical && sizeCritical:
		return true, AlertSeverityCritical, "posture-threshold-critical"
	case ageCritical:
		return true, AlertSeverityCritical, "posture-threshold-age-critical"
	case sizeCritical:
		return true, AlertSeverityCritical, "posture-threshold-size-critical"
	case ageWarning && sizeWarning:
		return true, AlertSeverityWarning, "posture-threshold-warning"
	case ageWarning:
		return true, AlertSeverityWarning, "posture-threshold-age-warning"
	case sizeWarning:
		return true, AlertSeverityWarning, "posture-threshold-size-warning"
	default:
		return false, "", "posture-threshold-normal"
	}
}
