package alerts

import (
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/reducer"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// metricStatusInput is what the canonical metric evaluator compared on one
// observation: the value after any rolling-average window, the exact rule
// handed to the reducer, and the incident the reducer holds afterwards.
type metricStatusInput struct {
	value                float64
	triggered            bool
	trigger              float64
	clear                float64
	recoveryDelaySeconds int
	unit                 string
	window               metricWindowObservation
	incident             reducer.Incident
	observedAt           time.Time
}

// metricStatusUnit is the unit of an evaluated metric value. CPU and memory
// share the "%" of usage even though their legacy messages omit a unit.
func metricStatusUnit(metricType string) string {
	switch metricType {
	case "temperature", "disk_temperature", "diskTemperature":
		return "°C"
	case "diskRead", "diskWrite", "networkIn", "networkOut":
		return "MB/s"
	default:
		return "%"
	}
}

// buildMetricAlertStatus derives the live status of an open threshold alert
// from the evaluator's own inputs, so surfaces never re-derive the phase or
// the recovery timer from thresholds and a resource reading of their own.
func buildMetricAlertStatus(input metricStatusInput) *models.MetricAlertStatus {
	// The reducer treats a non-positive clear as "clear at the trigger".
	recovery := input.clear
	if recovery <= 0 {
		recovery = input.trigger
	}
	status := &models.MetricAlertStatus{
		Phase:                models.MetricAlertPhaseBreaching,
		Value:                input.value,
		Unit:                 input.unit,
		ObservedAt:           input.observedAt,
		Trigger:              input.trigger,
		Recovery:             recovery,
		RecoveryDelaySeconds: input.recoveryDelaySeconds,
	}
	if input.window.WindowSeconds > 0 {
		raw := input.window.CurrentValue
		status.RawValue = &raw
		status.EvaluationWindowSeconds = input.window.WindowSeconds
	}
	if input.triggered {
		return status
	}
	if input.value > recovery || input.incident.RecoverySince.IsZero() {
		status.Phase = models.MetricAlertPhaseLatched
		return status
	}
	status.Phase = models.MetricAlertPhaseRecovering
	startedAt := input.incident.RecoverySince
	status.RecoveryStartedAt = &startedAt
	elapsed := input.observedAt.Sub(input.incident.RecoverySince)
	if input.incident.RecoveryTicksSupplied {
		elapsed = input.incident.RecoveryElapsed
	}
	if elapsed > 0 {
		status.RecoveryElapsedSeconds = int(elapsed / time.Second)
	}
	return status
}
