package specs

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
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

func Evaluate(spec ResourceAlertSpec, previous EvaluatorState, evidence AlertEvidence) (EvaluationResult, error) {
	if err := spec.Validate(); err != nil {
		return EvaluationResult{}, err
	}
	if err := evidence.validateForKind(spec.Kind); err != nil {
		return EvaluationResult{}, err
	}

	fingerprint, err := specFingerprint(spec)
	if err != nil {
		return EvaluationResult{}, err
	}

	now := evidence.ObservedAt
	previous = coercePreviousState(previous, spec.ID, fingerprint)

	if spec.Disabled {
		return terminalEvaluation(spec, previous, evidence, now, fingerprint, AlertStateClear, EvaluationTransitionDisabled, "disabled"), nil
	}

	if spec.SuppressOnConnectivityLoss && evidence.ParentConnected != nil && !*evidence.ParentConnected {
		return terminalEvaluation(spec, previous, evidence, now, fingerprint, AlertStateSuppressed, EvaluationTransitionSuppressed, "connectivity-suppressed"), nil
	}

	switch spec.Kind {
	case AlertSpecKindMetricThreshold:
		return evaluateMetricThreshold(spec, previous, evidence, now, fingerprint), nil
	case AlertSpecKindSeverityThreshold,
		AlertSpecKindChangeThreshold,
		AlertSpecKindBaselineAnomaly,
		AlertSpecKindConnectivity,
		AlertSpecKindPoweredState,
		AlertSpecKindProviderIncident,
		AlertSpecKindResourceIncidentRollup,
		AlertSpecKindServiceGap,
		AlertSpecKindDiscreteState:
		return evaluateMatchSpec(spec, previous, evidence, now, fingerprint), nil
	default:
		return EvaluationResult{}, fmt.Errorf("unsupported spec kind %q", spec.Kind)
	}
}

func evaluateMetricThreshold(spec ResourceAlertSpec, previous EvaluatorState, evidence AlertEvidence, now time.Time, fingerprint string) EvaluationResult {
	result := EvaluationResult{Previous: previous}
	next := previous
	next.SpecID = spec.ID
	next.SpecFingerprint = fingerprint
	next.LastObservedAt = now

	metric := evidence.MetricThreshold
	if metric == nil {
		return result
	}

	triggered := metricTriggered(spec.MetricThreshold, metric.Observed)
	if triggered {
		next.State = AlertStateFiring
		next.Severity = spec.Severity
		next.Reason = "threshold-exceeded"
		next.ConsecutiveMatches = 1
		if next.ActiveSince.IsZero() {
			next.ActiveSince = now
		}
		if next.FirstMatchedAt.IsZero() {
			next.FirstMatchedAt = now
		}
		result.State = next
		if previous.State != AlertStateFiring {
			result.Transition = &EvaluationTransition{
				Kind:       EvaluationTransitionActivated,
				SpecID:     spec.ID,
				ResourceID: spec.ResourceID,
				From:       previous.State,
				To:         AlertStateFiring,
				At:         now,
				Severity:   spec.Severity,
				Reason:     "threshold-exceeded",
				Evidence:   evidence,
			}
		}
		return result
	}

	next.State = AlertStateClear
	next.Severity = ""
	next.Reason = ""
	next.ConsecutiveMatches = 0
	next.FirstMatchedAt = time.Time{}
	next.ActiveSince = time.Time{}
	result.State = next

	if previous.State != AlertStateFiring {
		return result
	}

	reason := "recovered"
	kind := EvaluationTransitionRecovered
	if previous.SpecFingerprint != "" && previous.SpecFingerprint != next.SpecFingerprint {
		reason = "reevaluated"
		kind = EvaluationTransitionReevaluated
	} else if metricStillLatched(spec.MetricThreshold, metric.Observed) {
		next = previous
		next.SpecID = spec.ID
		next.SpecFingerprint = previous.SpecFingerprint
		next.LastObservedAt = now
		result.State = next
		result.Transition = nil
		return result
	}

	result.Transition = &EvaluationTransition{
		Kind:             kind,
		SpecID:           spec.ID,
		ResourceID:       spec.ResourceID,
		From:             previous.State,
		To:               AlertStateClear,
		At:               now,
		PreviousSeverity: previous.Severity,
		Reason:           reason,
		Evidence:         evidence,
	}
	return result
}

func evaluateMatchSpec(spec ResourceAlertSpec, previous EvaluatorState, evidence AlertEvidence, now time.Time, fingerprint string) EvaluationResult {
	result := EvaluationResult{Previous: previous}
	next := previous
	next.SpecID = spec.ID
	next.SpecFingerprint = fingerprint
	next.LastObservedAt = now

	match, severity, reason := matches(spec, evidence)
	if !match {
		next.State = AlertStateClear
		next.Severity = ""
		next.Reason = ""
		next.ConsecutiveMatches = 0
		next.FirstMatchedAt = time.Time{}
		next.ActiveSince = time.Time{}
		result.State = next
		if previous.State == AlertStateFiring || previous.State == AlertStatePending || previous.State == AlertStateSuppressed {
			result.Transition = &EvaluationTransition{
				Kind:             EvaluationTransitionRecovered,
				SpecID:           spec.ID,
				ResourceID:       spec.ResourceID,
				From:             previous.State,
				To:               AlertStateClear,
				At:               now,
				PreviousSeverity: previous.Severity,
				Reason:           reason,
				Evidence:         evidence,
			}
		}
		return result
	}

	required := defaultConfirmations(spec)
	if previous.State == AlertStateFiring {
		next.State = AlertStateFiring
		next.Severity = severity
		next.ConsecutiveMatches = required
		if next.ActiveSince.IsZero() {
			next.ActiveSince = now
		}
		if next.FirstMatchedAt.IsZero() {
			next.FirstMatchedAt = now
		}
		result.State = next
		if previous.Severity != "" && previous.Severity != severity {
			result.Transition = &EvaluationTransition{
				Kind:             EvaluationTransitionSeverityChanged,
				SpecID:           spec.ID,
				ResourceID:       spec.ResourceID,
				From:             AlertStateFiring,
				To:               AlertStateFiring,
				At:               now,
				PreviousSeverity: previous.Severity,
				Severity:         severity,
				Reason:           reason,
				Evidence:         evidence,
			}
		}
		return result
	}

	count := 1
	if previous.State == AlertStatePending && previous.FirstMatchedAt.IsZero() == false {
		count = previous.ConsecutiveMatches + 1
	}
	next.ConsecutiveMatches = count
	if next.FirstMatchedAt.IsZero() || previous.State != AlertStatePending {
		next.FirstMatchedAt = now
	}
	next.Severity = severity
	next.Reason = reason

	if count < required {
		next.State = AlertStatePending
		result.State = next
		if previous.State != AlertStatePending {
			result.Transition = &EvaluationTransition{
				Kind:       EvaluationTransitionPending,
				SpecID:     spec.ID,
				ResourceID: spec.ResourceID,
				From:       previous.State,
				To:         AlertStatePending,
				At:         now,
				Severity:   severity,
				Reason:     reason,
				Evidence:   evidence,
			}
		}
		return result
	}

	next.State = AlertStateFiring
	next.ConsecutiveMatches = required
	next.ActiveSince = now
	next.Reason = reason
	result.State = next
	result.Transition = &EvaluationTransition{
		Kind:       EvaluationTransitionActivated,
		SpecID:     spec.ID,
		ResourceID: spec.ResourceID,
		From:       previous.State,
		To:         AlertStateFiring,
		At:         now,
		Severity:   severity,
		Reason:     reason,
		Evidence:   evidence,
	}
	return result
}

func terminalEvaluation(spec ResourceAlertSpec, previous EvaluatorState, evidence AlertEvidence, now time.Time, fingerprint string, state AlertState, kind EvaluationTransitionKind, reason string) EvaluationResult {
	next := previous
	next.SpecID = spec.ID
	next.SpecFingerprint = fingerprint
	next.LastObservedAt = now
	next.State = state
	next.ConsecutiveMatches = 0
	next.FirstMatchedAt = time.Time{}
	if state != AlertStateFiring {
		next.ActiveSince = time.Time{}
		next.Severity = ""
		next.Reason = ""
	}

	result := EvaluationResult{
		Previous: previous,
		State:    next,
	}
	if previous.State == state {
		return result
	}
	result.Transition = &EvaluationTransition{
		Kind:             kind,
		SpecID:           spec.ID,
		ResourceID:       spec.ResourceID,
		From:             previous.State,
		To:               state,
		At:               now,
		PreviousSeverity: previous.Severity,
		Reason:           reason,
		Evidence:         evidence,
	}
	return result
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

func metricTriggered(spec *MetricThresholdSpec, observed float64) bool {
	if spec == nil {
		return false
	}
	switch spec.Direction {
	case ThresholdDirectionAbove:
		return observed >= spec.Trigger
	case ThresholdDirectionBelow:
		return observed <= spec.Trigger
	default:
		return false
	}
}

func metricStillLatched(spec *MetricThresholdSpec, observed float64) bool {
	if spec == nil {
		return false
	}
	if spec.Recovery == nil {
		return false
	}
	switch spec.Direction {
	case ThresholdDirectionAbove:
		return observed > *spec.Recovery
	case ThresholdDirectionBelow:
		return observed < *spec.Recovery
	default:
		return false
	}
}

func defaultConfirmations(spec ResourceAlertSpec) int {
	if spec.ConfirmationsRequired > 0 {
		return spec.ConfirmationsRequired
	}
	switch spec.Kind {
	case AlertSpecKindConnectivity:
		return 3
	case AlertSpecKindPoweredState:
		return 2
	default:
		return 1
	}
}

func coercePreviousState(previous EvaluatorState, specID, fingerprint string) EvaluatorState {
	if previous.SpecID == "" {
		previous.SpecID = specID
	}
	if previous.SpecFingerprint == "" {
		previous.SpecFingerprint = fingerprint
	}
	if previous.State == "" {
		previous.State = AlertStateClear
	}
	if previous.SpecID != specID {
		previous.SpecID = specID
	}
	return previous
}

func specFingerprint(spec ResourceAlertSpec) (string, error) {
	payload, err := json.Marshal(spec)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}
