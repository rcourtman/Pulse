package alerts

import (
	"fmt"
	"math"
	"strings"
	"time"

	alertspecs "github.com/rcourtman/pulse-go-rewrite/internal/alerts/specs"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rs/zerolog/log"
)

// pmgQuarantineSnapshot stores quarantine counts at a point in time for growth detection
type pmgQuarantineSnapshot struct {
	Spam      int
	Virus     int
	Timestamp time.Time
}

// pmgMailMetricSample stores a single hourly mail count sample
type pmgMailMetricSample struct {
	SpamIn    float64
	SpamOut   float64
	VirusIn   float64
	VirusOut  float64
	Timestamp time.Time
}

// pmgAnomalyTracker tracks history for anomaly detection.
type pmgAnomalyTracker struct {
	Samples        []pmgMailMetricSample // Ring buffer (max 48 samples)
	LastSampleTime time.Time             // Timestamp of most recent sample
	SampleCount    int                   // Total samples collected (for warmup check)
}

func isPMGOffline(pmg models.PMGInstance) bool {
	status := strings.ToLower(strings.TrimSpace(pmg.Status))
	health := strings.ToLower(strings.TrimSpace(pmg.ConnectionHealth))
	return status == "offline" || health == "error" || health == "failed" || health == "unhealthy"
}

func (m *Manager) clearPMGMetricAlerts(pmgID string) {
	pmgID = strings.TrimSpace(pmgID)
	if pmgID == "" {
		return
	}

	offlineAlertID := canonicalConnectivityStateID(pmgID)

	m.mu.RLock()
	alertIDs := make([]string, 0)
	for storageKey, alert := range m.activeAlerts {
		alertID := storageKey
		if alertID == offlineAlertID {
			continue
		}
		if alert != nil && alert.ResourceID == pmgID {
			alertIDs = append(alertIDs, alertID)
		}
	}
	m.mu.RUnlock()

	for _, alertID := range alertIDs {
		m.clearAlert(alertID)
	}
}

// CheckPMG checks a Proxmox Mail Gateway instance against thresholds
func (m *Manager) CheckPMG(pmg models.PMGInstance) {
	m.mu.RLock()
	if !m.config.Enabled {
		m.mu.RUnlock()
		return
	}
	if m.config.DisableAllPMG {
		m.mu.RUnlock()
		// Clear any existing PMG alerts when all PMG alerts are disabled.
		m.mu.Lock()
		delete(m.offlineConfirmations, pmg.ID)
		m.mu.Unlock()
		m.clearPMGMetricAlerts(pmg.ID)
		m.clearAlert(canonicalConnectivityStateID(pmg.ID))
		return
	}

	// Check if there's an override for this PMG instance
	override, hasOverride := m.config.Overrides[pmg.ID]
	disablePMGOffline := m.config.DisableAllPMGOffline
	pmgDefaults := m.config.PMGDefaults
	m.mu.RUnlock()

	// Check override disable BEFORE offline detection to prevent spurious notifications
	if hasOverride && override.Disabled {
		m.mu.Lock()
		delete(m.offlineConfirmations, pmg.ID)
		m.mu.Unlock()
		m.clearPMGMetricAlerts(pmg.ID)
		m.clearAlert(canonicalConnectivityStateID(pmg.ID))
		return
	}

	pmgOffline := isPMGOffline(pmg)

	// Handle offline detection
	if disablePMGOffline {
		// Clear tracking and any existing offline alerts when globally disabled
		m.mu.Lock()
		delete(m.offlineConfirmations, pmg.ID)
		m.mu.Unlock()
		m.clearAlert(canonicalConnectivityStateID(pmg.ID))
	} else {
		// Check if PMG is offline (similar to PBS/nodes)
		if pmgOffline {
			m.checkPMGOffline(pmg)
		} else {
			// Clear any existing offline alert if PMG is back online
			m.clearPMGOfflineAlert(pmg)
		}
	}

	// When PMG is offline/unhealthy, clear stale metric alerts immediately.
	if pmgOffline {
		m.clearPMGMetricAlerts(pmg.ID)
		return
	}

	// Check queue depths across all nodes
	m.checkPMGQueueDepths(pmg, pmgDefaults)
	// Check oldest message age across all nodes
	m.checkPMGOldestMessage(pmg, pmgDefaults)
	// Check quarantine backlog and growth
	m.checkPMGQuarantineBacklog(pmg, pmgDefaults)
	// Check spam/virus rate anomalies
	m.checkPMGAnomalies(pmg, pmgDefaults)
	// Check per-node queue health
	m.checkPMGNodeQueues(pmg, pmgDefaults)
}

// checkPMGOffline creates an alert for offline PMG instances
func (m *Manager) checkPMGOffline(pmg models.PMGInstance) {
	m.mu.Lock()
	delete(m.offlineRecoveryConfirmations, canonicalConnectivityStateID(pmg.ID))
	m.mu.Unlock()

	m.mu.RLock()
	override, hasOverride := m.config.Overrides[pmg.ID]
	m.mu.RUnlock()
	disabled := hasOverride && (override.Disabled || override.DisableConnectivity)
	spec, err := buildCanonicalConnectivitySpec(pmg.ID, pmg.Name, unifiedresources.ResourceTypePMG, AlertLevelCritical, 3, disabled)
	if err != nil {
		log.Warn().
			Err(err).
			Str("pmg", pmg.Name).
			Str("pmgID", pmg.ID).
			Msg("Skipping invalid canonical PMG connectivity spec")
		return
	}

	_, _ = m.evaluateCanonicalLifecycleAlert(canonicalLifecycleAlertParams{
		Spec:         spec,
		Evidence:     alertspecs.AlertEvidence{ObservedAt: time.Now(), Connectivity: &alertspecs.ConnectivityEvidence{Signal: "status", Connected: false}},
		Tracking:     m.offlineConfirmations,
		TrackingKey:  pmg.ID,
		AlertID:      fmt.Sprintf("pmg-offline-%s", pmg.ID),
		AlertType:    "offline",
		ResourceID:   pmg.ID,
		ResourceName: pmg.Name,
		Node:         pmg.Host,
		Instance:     pmg.Name,
		Message:      fmt.Sprintf("PMG instance %s is offline", pmg.Name),
		Metadata: map[string]interface{}{
			"resourceType":     "pmg",
			"status":           pmg.Status,
			"connectionHealth": pmg.ConnectionHealth,
		},
		RateLimit:     true,
		DispatchAsync: true,
	})
}

// clearPMGOfflineAlert removes offline alert when PMG comes back online
func (m *Manager) clearPMGOfflineAlert(pmg models.PMGInstance) {
	m.clearResourceOfflineAlert(pmg.ID, pmg.Name, pmg.Host, "PMG", offlineRecoveryConfirmationsDefault)
}

// checkPMGQueueDepths checks PMG mail queue depths and creates alerts
// Evaluates all queue types (total, deferred, hold) independently
func (m *Manager) checkPMGQueueDepths(pmg models.PMGInstance, defaults PMGThresholdConfig) {
	// Aggregate queue totals across all nodes
	var totalQueue, totalDeferred, totalHold int

	for _, node := range pmg.Nodes {
		if node.QueueStatus != nil {
			totalQueue += node.QueueStatus.Total
			totalDeferred += node.QueueStatus.Deferred
			totalHold += node.QueueStatus.Hold
		}
	}

	m.checkPMGQueueDepth(pmg, defaults.QueueTotalWarning, defaults.QueueTotalCritical, totalQueue, "queue-total", "queue-depth",
		"PMG %s has %d total messages in queue (threshold: %d)", "total_queue")
	m.checkPMGQueueDepth(pmg, defaults.DeferredQueueWarn, defaults.DeferredQueueCritical, totalDeferred, "queue-deferred", "queue-deferred",
		"PMG %s has %d deferred messages (threshold: %d)", "deferred_queue")
	m.checkPMGQueueDepth(pmg, defaults.HoldQueueWarn, defaults.HoldQueueCritical, totalHold, "queue-hold", "queue-hold",
		"PMG %s has %d held messages (threshold: %d)", "hold_queue")
}

func thresholdForCanonicalSeverity(severity alertspecs.AlertSeverity, warningThreshold, criticalThreshold float64) float64 {
	switch severity {
	case alertspecs.AlertSeverityCritical:
		if criticalThreshold > 0 {
			return criticalThreshold
		}
		return warningThreshold
	case alertspecs.AlertSeverityWarning:
		if warningThreshold > 0 {
			return warningThreshold
		}
		return criticalThreshold
	default:
		return 0
	}
}

func quarantineAlertThreshold(metricType, reason string, previousCount int, defaults PMGThresholdConfig) float64 {
	switch metricType {
	case "spam":
		switch reason {
		case "change-threshold-current-critical":
			return float64(defaults.QuarantineSpamCritical)
		case "change-threshold-current-warning":
			return float64(defaults.QuarantineSpamWarn)
		case "change-threshold-growth-critical":
			return float64(previousCount + defaults.QuarantineGrowthCritMin)
		case "change-threshold-growth-warning":
			return float64(previousCount + defaults.QuarantineGrowthWarnMin)
		default:
			return 0
		}
	case "virus":
		switch reason {
		case "change-threshold-current-critical":
			return float64(defaults.QuarantineVirusCritical)
		case "change-threshold-current-warning":
			return float64(defaults.QuarantineVirusWarn)
		case "change-threshold-growth-critical":
			return float64(previousCount + defaults.QuarantineGrowthCritMin)
		case "change-threshold-growth-warning":
			return float64(previousCount + defaults.QuarantineGrowthWarnMin)
		default:
			return 0
		}
	default:
		return 0
	}
}

func quarantineAlertMessage(pmg models.PMGInstance, metricType string, current, previousCount int, reason string, defaults PMGThresholdConfig) string {
	switch reason {
	case "change-threshold-growth-critical", "change-threshold-growth-warning":
		growth := current - previousCount
		growthPct := 0.0
		if previousCount > 0 {
			growthPct = (float64(growth) / float64(previousCount)) * 100
		}
		if reason == "change-threshold-growth-critical" {
			return fmt.Sprintf("PMG %s %s quarantine growing rapidly: +%d messages (+%.1f%%) in 2 hours", pmg.Name, metricType, growth, growthPct)
		}
		return fmt.Sprintf("PMG %s %s quarantine growing: +%d messages (+%.1f%%) in 2 hours", pmg.Name, metricType, growth, growthPct)
	default:
		threshold := quarantineAlertThreshold(metricType, reason, previousCount, defaults)
		return fmt.Sprintf("PMG %s has %d %s messages in quarantine (threshold: %d)", pmg.Name, current, metricType, int(threshold))
	}
}

func (m *Manager) checkPMGQueueDepth(pmg models.PMGInstance, warningThreshold, criticalThreshold, value int, alertIDSuffix, alertType, messageFormat, logField string) {
	if warningThreshold <= 0 && criticalThreshold <= 0 {
		return
	}

	alertID := fmt.Sprintf("%s-%s", pmg.ID, alertIDSuffix)
	spec, err := buildCanonicalSeverityThresholdSpec(alertID, pmg.ID, pmg.Name, unifiedresources.ResourceTypePMG, alertType, float64(warningThreshold), float64(criticalThreshold), false)
	if err != nil {
		log.Warn().
			Err(err).
			Str("pmg", pmg.Name).
			Str("alertID", alertID).
			Msg("Skipping invalid canonical PMG queue spec")
		return
	}

	result, _ := m.evaluateCanonicalStatefulAlert(canonicalStatefulAlertParams{
		Spec: spec,
		Evidence: alertspecs.AlertEvidence{
			ObservedAt: time.Now(),
			SeverityThreshold: &alertspecs.SeverityThresholdEvidence{
				Metric:    alertType,
				Direction: alertspecs.ThresholdDirectionAbove,
				Observed:  float64(value),
			},
		},
		AlertID:      alertID,
		AlertType:    alertType,
		ResourceID:   pmg.ID,
		ResourceName: pmg.Name,
		Node:         pmg.Host,
		Instance:     pmg.Name,
		MessageBuilder: func(result alertspecs.EvaluationResult) (string, float64, float64) {
			threshold := thresholdForCanonicalSeverity(result.State.Severity, float64(warningThreshold), float64(criticalThreshold))
			return fmt.Sprintf(messageFormat, pmg.Name, value, int(threshold)), float64(value), threshold
		},
		DispatchAsync: true,
	})

	if result.Transition != nil && result.Transition.Kind == alertspecs.EvaluationTransitionActivated {
		level, ok := alertLevelFromCanonicalSeverity(result.State.Severity)
		if !ok {
			level = AlertLevelWarning
		}
		log.Warn().
			Str("pmg", pmg.Name).
			Int(logField, value).
			Int("threshold", int(thresholdForCanonicalSeverity(result.State.Severity, float64(warningThreshold), float64(criticalThreshold)))).
			Str("level", string(level)).
			Msg(fmt.Sprintf("PMG %s alert triggered", alertType))
	}
}

// checkPMGOldestMessage checks oldest queued message age and creates alerts
func (m *Manager) checkPMGOldestMessage(pmg models.PMGInstance, defaults PMGThresholdConfig) {
	if defaults.OldestMessageWarnMins <= 0 && defaults.OldestMessageCritMins <= 0 {
		return
	}

	// Find the oldest message age across all nodes
	var oldestAge int64 // in seconds
	for _, node := range pmg.Nodes {
		if node.QueueStatus != nil && node.QueueStatus.OldestAge > oldestAge {
			oldestAge = node.QueueStatus.OldestAge
		}
	}

	if oldestAge == 0 {
		// No messages in queue, clear any existing alert
		alertID := fmt.Sprintf("%s-oldest-message", pmg.ID)
		m.clearAlert(buildCanonicalStateID(pmg.ID, alertID))
		return
	}

	alertID := fmt.Sprintf("%s-oldest-message", pmg.ID)
	oldestMinutes := oldestAge / 60
	spec, err := buildCanonicalSeverityThresholdSpec(alertID, pmg.ID, pmg.Name, unifiedresources.ResourceTypePMG, "message-age", float64(defaults.OldestMessageWarnMins), float64(defaults.OldestMessageCritMins), false)
	if err != nil {
		log.Warn().
			Err(err).
			Str("pmg", pmg.Name).
			Str("alertID", alertID).
			Msg("Skipping invalid canonical PMG oldest-message spec")
		return
	}

	result, _ := m.evaluateCanonicalStatefulAlert(canonicalStatefulAlertParams{
		Spec: spec,
		Evidence: alertspecs.AlertEvidence{
			ObservedAt: time.Now(),
			SeverityThreshold: &alertspecs.SeverityThresholdEvidence{
				Metric:    "message-age",
				Direction: alertspecs.ThresholdDirectionAbove,
				Observed:  float64(oldestMinutes),
			},
		},
		AlertID:      alertID,
		AlertType:    "message-age",
		ResourceID:   pmg.ID,
		ResourceName: pmg.Name,
		Node:         pmg.Host,
		Instance:     pmg.Name,
		MessageBuilder: func(result alertspecs.EvaluationResult) (string, float64, float64) {
			threshold := thresholdForCanonicalSeverity(result.State.Severity, float64(defaults.OldestMessageWarnMins), float64(defaults.OldestMessageCritMins))
			return fmt.Sprintf("PMG %s has messages queued for %d minutes (threshold: %d minutes)", pmg.Name, oldestMinutes, int64(threshold)), float64(oldestMinutes), threshold
		},
		DispatchAsync: true,
	})

	if result.Transition != nil && result.Transition.Kind == alertspecs.EvaluationTransitionActivated {
		level, ok := alertLevelFromCanonicalSeverity(result.State.Severity)
		if !ok {
			level = AlertLevelWarning
		}
		log.Warn().
			Str("pmg", pmg.Name).
			Int64("oldest_minutes", oldestMinutes).
			Int64("threshold", int64(thresholdForCanonicalSeverity(result.State.Severity, float64(defaults.OldestMessageWarnMins), float64(defaults.OldestMessageCritMins)))).
			Str("level", string(level)).
			Msg("PMG oldest message age alert triggered")
	}
}

// evaluatePMGNodeQueueAlert runs one per-node PMG queue threshold check
// (total / deferred / hold share this shape). messageNoun is the operator
// phrasing ("total messages in queue", "deferred messages", "held messages")
// and specLabel feeds the invalid-spec log line. It returns true when the
// caller must skip the node's remaining checks — either the below-threshold
// clear or an invalid spec — preserving the historical per-node
// short-circuit semantics.
func (m *Manager) evaluatePMGNodeQueueAlert(pmg models.PMGInstance, nodeName, metric, specLabel, messageNoun string, observed, scaledWarn, scaledCrit, median int) bool {
	if scaledWarn <= 0 && scaledCrit <= 0 {
		return false
	}

	alertID := fmt.Sprintf("%s-%s-%s", pmg.ID, nodeName, metric)
	if (scaledCrit <= 0 || observed < scaledCrit) && (scaledWarn <= 0 || observed < scaledWarn) {
		m.clearAlert(buildCanonicalStateID(pmg.ID, alertID))
		return true
	}

	// Add outlier indicator to message if applicable
	isOutlier := isQueueOutlier(observed, median)
	outlierNote := ""
	if isOutlier {
		outlierNote = ", outlier"
	}

	spec, err := buildCanonicalSeverityThresholdSpec(alertID, pmg.ID, pmg.Name, unifiedresources.ResourceTypePMG, metric, float64(scaledWarn), float64(scaledCrit), false)
	if err != nil {
		log.Warn().Err(err).Str("pmg", pmg.Name).Str("alertID", alertID).Msg("Skipping invalid canonical PMG node " + specLabel + " spec")
		return true
	}
	_, _ = m.evaluateCanonicalStatefulAlert(canonicalStatefulAlertParams{
		Spec: spec,
		Evidence: alertspecs.AlertEvidence{
			ObservedAt: time.Now(),
			SeverityThreshold: &alertspecs.SeverityThresholdEvidence{
				Metric:    metric,
				Direction: alertspecs.ThresholdDirectionAbove,
				Observed:  float64(observed),
			},
		},
		AlertID:      alertID,
		AlertType:    metric,
		ResourceID:   pmg.ID,
		ResourceName: pmg.Name,
		Node:         nodeName,
		Instance:     pmg.Name,
		MessageBuilder: func(result alertspecs.EvaluationResult) (string, float64, float64) {
			currentThreshold := thresholdForCanonicalSeverity(result.State.Severity, float64(scaledWarn), float64(scaledCrit))
			return fmt.Sprintf("PMG node %s on %s has %d %s (threshold: %d%s)", nodeName, pmg.Name, observed, messageNoun, int(currentThreshold), outlierNote), float64(observed), currentThreshold
		},
		DispatchAsync: true,
	})
	return false
}

// checkPMGNodeQueues checks individual PMG node queue health
// Uses scaled thresholds (60% warn, 80% crit) and outlier detection
func (m *Manager) checkPMGNodeQueues(pmg models.PMGInstance, defaults PMGThresholdConfig) {
	if len(pmg.Nodes) == 0 {
		return
	}

	// Calculate median queue values across nodes for outlier detection
	nodeQueueTotals := make([]int, 0, len(pmg.Nodes))
	nodeQueueDeferred := make([]int, 0, len(pmg.Nodes))
	nodeQueueHold := make([]int, 0, len(pmg.Nodes))

	for _, node := range pmg.Nodes {
		if node.QueueStatus != nil {
			nodeQueueTotals = append(nodeQueueTotals, node.QueueStatus.Total)
			nodeQueueDeferred = append(nodeQueueDeferred, node.QueueStatus.Deferred)
			nodeQueueHold = append(nodeQueueHold, node.QueueStatus.Hold)
		}
	}

	medianTotal := calculateMedianInt(nodeQueueTotals)
	medianDeferred := calculateMedianInt(nodeQueueDeferred)
	medianHold := calculateMedianInt(nodeQueueHold)

	// Scaled thresholds: 60% for warning, 80% for critical (computed once, used for all nodes)
	scaledQueueWarn := scaleThreshold(defaults.QueueTotalWarning, 0.6)
	scaledQueueCrit := scaleThreshold(defaults.QueueTotalCritical, 0.8)
	scaledDeferredWarn := scaleThreshold(defaults.DeferredQueueWarn, 0.6)
	scaledDeferredCrit := scaleThreshold(defaults.DeferredQueueCritical, 0.8)
	scaledHoldWarn := scaleThreshold(defaults.HoldQueueWarn, 0.6)
	scaledHoldCrit := scaleThreshold(defaults.HoldQueueCritical, 0.8)
	scaledAgeWarn := scaleThreshold(defaults.OldestMessageWarnMins, 0.6)
	scaledAgeCrit := scaleThreshold(defaults.OldestMessageCritMins, 0.8)

	// Check each node
	for _, node := range pmg.Nodes {
		if node.QueueStatus == nil {
			continue
		}

		// Check total queue - always check thresholds
		if m.evaluatePMGNodeQueueAlert(pmg, node.Name, "queue-total", "queue", "total messages in queue", node.QueueStatus.Total, scaledQueueWarn, scaledQueueCrit, medianTotal) {
			continue
		}

		// Check deferred queue - always check thresholds
		if m.evaluatePMGNodeQueueAlert(pmg, node.Name, "queue-deferred", "deferred queue", "deferred messages", node.QueueStatus.Deferred, scaledDeferredWarn, scaledDeferredCrit, medianDeferred) {
			continue
		}

		// Check hold queue - always check thresholds
		if m.evaluatePMGNodeQueueAlert(pmg, node.Name, "queue-hold", "hold queue", "held messages", node.QueueStatus.Hold, scaledHoldWarn, scaledHoldCrit, medianHold) {
			continue
		}

		// Check oldest message age per node
		if scaledAgeWarn > 0 || scaledAgeCrit > 0 {
			oldestAge := node.QueueStatus.OldestAge
			if oldestAge > 0 {
				oldestMinutes := oldestAge / 60
				alertID := fmt.Sprintf("%s-%s-oldest-message", pmg.ID, node.Name)
				if (scaledAgeCrit <= 0 || oldestMinutes < int64(scaledAgeCrit)) && (scaledAgeWarn <= 0 || oldestMinutes < int64(scaledAgeWarn)) {
					m.clearAlert(buildCanonicalStateID(pmg.ID, alertID))
					continue
				}

				spec, err := buildCanonicalSeverityThresholdSpec(alertID, pmg.ID, pmg.Name, unifiedresources.ResourceTypePMG, "message-age", float64(scaledAgeWarn), float64(scaledAgeCrit), false)
				if err != nil {
					log.Warn().Err(err).Str("pmg", pmg.Name).Str("alertID", alertID).Msg("Skipping invalid canonical PMG node message-age spec")
					continue
				}
				_, _ = m.evaluateCanonicalStatefulAlert(canonicalStatefulAlertParams{
					Spec: spec,
					Evidence: alertspecs.AlertEvidence{
						ObservedAt: time.Now(),
						SeverityThreshold: &alertspecs.SeverityThresholdEvidence{
							Metric:    "message-age",
							Direction: alertspecs.ThresholdDirectionAbove,
							Observed:  float64(oldestMinutes),
						},
					},
					AlertID:      alertID,
					AlertType:    "message-age",
					ResourceID:   pmg.ID,
					ResourceName: pmg.Name,
					Node:         node.Name,
					Instance:     pmg.Name,
					MessageBuilder: func(result alertspecs.EvaluationResult) (string, float64, float64) {
						currentThreshold := thresholdForCanonicalSeverity(result.State.Severity, float64(scaledAgeWarn), float64(scaledAgeCrit))
						return fmt.Sprintf("PMG node %s on %s has messages queued for %d minutes (threshold: %d min, node-specific)", node.Name, pmg.Name, oldestMinutes, int(currentThreshold)), float64(oldestMinutes), currentThreshold
					},
					DispatchAsync: true,
				})
			}
		}
	}
}

// isQueueOutlier determines if a node's queue value is a significant outlier
// Returns true if value is >40% above the median across all nodes
func isQueueOutlier(value, median int) bool {
	if median == 0 {
		return value > 0
	}
	percentAboveMedian := float64(value-median) / float64(median) * 100
	return percentAboveMedian > 40
}

// scaleThreshold applies a scaling factor to a threshold and ensures minimum value of 1
// Uses ceiling to avoid truncation issues with small thresholds
func scaleThreshold(threshold int, scaleFactor float64) int {
	if threshold <= 0 {
		return 0
	}
	scaled := int(math.Ceil(float64(threshold) * scaleFactor))
	if scaled < 1 {
		return 1
	}
	return scaled
}

// calculateMedianInt calculates median of integer slice
func calculateMedianInt(values []int) int {
	if len(values) == 0 {
		return 0
	}

	// Copy and sort
	sorted := make([]int, len(values))
	copy(sorted, values)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	mid := len(sorted) / 2
	if len(sorted)%2 == 0 {
		return (sorted[mid-1] + sorted[mid]) / 2
	}
	return sorted[mid]
}

// checkPMGQuarantineBacklog checks quarantine backlog and growth rates
func (m *Manager) checkPMGQuarantineBacklog(pmg models.PMGInstance, defaults PMGThresholdConfig) {
	if pmg.Quarantine == nil {
		m.clearAlert(buildCanonicalStateID(pmg.ID, fmt.Sprintf("%s-quarantine-spam", pmg.ID)))
		m.clearAlert(buildCanonicalStateID(pmg.ID, fmt.Sprintf("%s-quarantine-virus", pmg.ID)))
		return
	}

	now := time.Now()
	currentSpam := pmg.Quarantine.Spam
	currentVirus := pmg.Quarantine.Virus

	// Store current snapshot
	m.mu.Lock()
	snapshot := pmgQuarantineSnapshot{
		Spam:      currentSpam,
		Virus:     currentVirus,
		Timestamp: now,
	}

	// Get or create history for this PMG instance
	history := m.pmgQuarantineHistory[pmg.ID]
	history = append(history, snapshot)

	// Clean old snapshots (keep last 3 hours)
	cutoff := now.Add(-3 * time.Hour)
	validSnapshots := make([]pmgQuarantineSnapshot, 0, len(history))
	for _, snap := range history {
		if snap.Timestamp.After(cutoff) {
			validSnapshots = append(validSnapshots, snap)
		}
	}
	// Limit to max 48 samples to prevent unbounded growth
	const maxQuarantineSnapshots = 48
	if len(validSnapshots) > maxQuarantineSnapshots {
		validSnapshots = validSnapshots[len(validSnapshots)-maxQuarantineSnapshots:]
	}
	m.pmgQuarantineHistory[pmg.ID] = validSnapshots
	m.mu.Unlock()

	// Find snapshot from ~2 hours ago (within ±15 min tolerance)
	var twoHoursAgo *pmgQuarantineSnapshot
	targetTime := now.Add(-2 * time.Hour)
	minDiff := 15 * time.Minute

	for i := range validSnapshots {
		snap := &validSnapshots[i]
		diff := snap.Timestamp.Sub(targetTime)
		if diff < 0 {
			diff = -diff
		}
		if diff < minDiff {
			minDiff = diff
			twoHoursAgo = snap
		}
	}

	// Check spam quarantine
	m.checkQuarantineMetric(pmg, "spam", currentSpam, twoHoursAgo, defaults)

	// Check virus quarantine
	m.checkQuarantineMetric(pmg, "virus", currentVirus, twoHoursAgo, defaults)
}

// checkQuarantineMetric checks a single quarantine metric (spam or virus)
func (m *Manager) checkQuarantineMetric(pmg models.PMGInstance, metricType string, current int, twoHoursAgo *pmgQuarantineSnapshot, defaults PMGThresholdConfig) {
	alertID := fmt.Sprintf("%s-quarantine-%s", pmg.ID, metricType)

	var absoluteWarn, absoluteCrit int
	var previousCount int

	// Get thresholds and previous count based on metric type
	if metricType == "spam" {
		absoluteWarn = defaults.QuarantineSpamWarn
		absoluteCrit = defaults.QuarantineSpamCritical
		if twoHoursAgo != nil {
			previousCount = twoHoursAgo.Spam
		}
	} else { // virus
		absoluteWarn = defaults.QuarantineVirusWarn
		absoluteCrit = defaults.QuarantineVirusCritical
		if twoHoursAgo != nil {
			previousCount = twoHoursAgo.Virus
		}
	}

	if absoluteWarn <= 0 && absoluteCrit <= 0 &&
		defaults.QuarantineGrowthWarnMin <= 0 && defaults.QuarantineGrowthCritMin <= 0 {
		m.clearAlert(buildCanonicalStateID(pmg.ID, alertID))
		return
	}

	spec, err := buildCanonicalChangeThresholdSpec(
		alertID,
		pmg.ID,
		pmg.Name,
		unifiedresources.ResourceTypePMG,
		"quarantine-"+metricType,
		float64(absoluteWarn),
		float64(absoluteCrit),
		float64(defaults.QuarantineGrowthWarnMin),
		float64(defaults.QuarantineGrowthCritMin),
		float64(defaults.QuarantineGrowthWarnPct),
		float64(defaults.QuarantineGrowthCritPct),
		2*time.Hour,
		false,
	)
	if err != nil {
		log.Warn().
			Err(err).
			Str("pmg", pmg.Name).
			Str("alertID", alertID).
			Str("metricType", metricType).
			Msg("Skipping invalid canonical PMG quarantine spec")
		return
	}

	evidence := alertspecs.AlertEvidence{
		ObservedAt: time.Now(),
		ChangeThreshold: &alertspecs.ChangeThresholdEvidence{
			Metric:   "quarantine-" + metricType,
			Observed: float64(current),
		},
	}
	if previousCount > 0 {
		previous := float64(previousCount)
		evidence.ChangeThreshold.PreviousObserved = &previous
	}

	result, _ := m.evaluateCanonicalStatefulAlert(canonicalStatefulAlertParams{
		Spec:         spec,
		Evidence:     evidence,
		AlertID:      alertID,
		AlertType:    fmt.Sprintf("quarantine-%s", metricType),
		ResourceID:   pmg.ID,
		ResourceName: pmg.Name,
		Node:         pmg.Host,
		Instance:     pmg.Name,
		MessageBuilder: func(result alertspecs.EvaluationResult) (string, float64, float64) {
			reason := result.State.Reason
			threshold := quarantineAlertThreshold(metricType, reason, previousCount, defaults)
			return quarantineAlertMessage(pmg, metricType, current, previousCount, reason, defaults), float64(current), threshold
		},
		DispatchAsync: true,
	})

	if result.Transition != nil && result.Transition.Kind == alertspecs.EvaluationTransitionActivated {
		level, ok := alertLevelFromCanonicalSeverity(result.State.Severity)
		if !ok {
			level = AlertLevelWarning
		}
		log.Warn().
			Str("pmg", pmg.Name).
			Str("type", metricType).
			Int("current", current).
			Int("threshold", int(quarantineAlertThreshold(metricType, result.State.Reason, previousCount, defaults))).
			Str("level", string(level)).
			Msg("PMG quarantine backlog alert triggered")
	}
}

// calculateTrimmedBaseline computes a robust baseline from historical samples
// using trimmed mean with median fallback for statistical robustness
func calculateTrimmedBaseline(samples []float64) (baseline float64, trustworthy bool) {
	sampleCount := len(samples)

	// Need at least 12 samples for trustworthy baseline (warmup period)
	if sampleCount < 12 {
		return 0, false
	}

	// For full 24-sample baseline, use trimmed mean
	if sampleCount >= 24 {
		// Create a copy for sorting
		sorted := make([]float64, len(samples))
		copy(sorted, samples)

		// Sort samples
		for i := 0; i < len(sorted); i++ {
			for j := i + 1; j < len(sorted); j++ {
				if sorted[i] > sorted[j] {
					sorted[i], sorted[j] = sorted[j], sorted[i]
				}
			}
		}

		// Calculate median
		var median float64
		mid := len(sorted) / 2
		if len(sorted)%2 == 0 {
			median = (sorted[mid-1] + sorted[mid]) / 2
		} else {
			median = sorted[mid]
		}

		// Calculate trimmed mean: drop top and bottom 2, average remaining 20
		if len(sorted) >= 24 {
			trimmed := sorted[2 : len(sorted)-2]
			sum := 0.0
			for _, val := range trimmed {
				sum += val
			}
			trimmedMean := sum / float64(len(trimmed))

			// Fallback rule: if trimmed mean differs from median by >40%, use median
			diff := trimmedMean - median
			if diff < 0 {
				diff = -diff
			}
			percentDiff := (diff / median) * 100

			if percentDiff > 40 {
				return median, true
			}
			return trimmedMean, true
		}
	}

	// For 12-23 samples, use simple mean (not enough for trimming)
	sum := 0.0
	for _, val := range samples {
		sum += val
	}
	return sum / float64(len(samples)), true
}

func anomalyAlertMessage(pmg models.PMGInstance, metricName string, current, baseline float64) string {
	ratio := 0.0
	if baseline > 0 {
		ratio = current / baseline
	}
	return fmt.Sprintf("PMG %s anomaly detected: %s is %.1f messages/hour (%.1fx baseline of %.1f)", pmg.Name, metricName, current, ratio, baseline)
}

// checkPMGAnomalies detects spam/virus rate anomalies using trimmed baseline
func (m *Manager) checkPMGAnomalies(pmg models.PMGInstance, _ PMGThresholdConfig) {
	// Need mail count data
	if len(pmg.MailCount) == 0 {
		return
	}

	// Get the latest hourly sample (most recent)
	latest := pmg.MailCount[len(pmg.MailCount)-1]
	now := time.Now()

	// Get or create anomaly tracker for this PMG instance
	m.mu.Lock()
	tracker := m.pmgAnomalyTrackers[pmg.ID]
	if tracker == nil {
		tracker = &pmgAnomalyTracker{
			Samples: make([]pmgMailMetricSample, 0, 48),
		}
		m.pmgAnomalyTrackers[pmg.ID] = tracker
	}

	// Create sample from latest mail count
	sample := pmgMailMetricSample{
		SpamIn:    latest.SpamIn,
		SpamOut:   latest.SpamOut,
		VirusIn:   latest.VirusIn,
		VirusOut:  latest.VirusOut,
		Timestamp: latest.Timestamp,
	}

	// Check for duplicate timestamp (already processed this sample)
	if !tracker.LastSampleTime.IsZero() && !sample.Timestamp.After(tracker.LastSampleTime) {
		m.mu.Unlock()
		return
	}

	// Check for timestamp gaps (>90 min indicates data discontinuity)
	if !tracker.LastSampleTime.IsZero() {
		gap := sample.Timestamp.Sub(tracker.LastSampleTime)
		if gap > 90*time.Minute {
			// Discard old samples - data gap detected
			log.Debug().
				Str("pmg", pmg.Name).
				Dur("gap", gap).
				Msg("PMG mail count data gap detected, resetting anomaly history")
			tracker.Samples = make([]pmgMailMetricSample, 0, 48)
			tracker.SampleCount = 0
		}
	}

	// Add sample to ring buffer
	tracker.Samples = append(tracker.Samples, sample)
	tracker.SampleCount++
	tracker.LastSampleTime = sample.Timestamp

	// Maintain ring buffer size (keep last 48)
	if len(tracker.Samples) > 48 {
		tracker.Samples = tracker.Samples[len(tracker.Samples)-48:]
	}

	sampleCount := len(tracker.Samples)
	m.mu.Unlock()

	// Need at least 12 samples for baseline warmup
	if sampleCount < 12 {
		log.Debug().
			Str("pmg", pmg.Name).
			Int("samples", sampleCount).
			Msg("PMG anomaly detection warming up (need 12 samples)")
		return
	}

	// Calculate baselines and check each metric
	metrics := []struct {
		name      string
		current   float64
		extractor func(pmgMailMetricSample) float64
	}{
		{"spamIn", sample.SpamIn, func(s pmgMailMetricSample) float64 { return s.SpamIn }},
		{"spamOut", sample.SpamOut, func(s pmgMailMetricSample) float64 { return s.SpamOut }},
		{"virusIn", sample.VirusIn, func(s pmgMailMetricSample) float64 { return s.VirusIn }},
		{"virusOut", sample.VirusOut, func(s pmgMailMetricSample) float64 { return s.VirusOut }},
	}

	for _, metric := range metrics {
		m.checkAnomalyMetric(pmg, tracker, metric.name, metric.current, metric.extractor, now)
	}
}

// checkAnomalyMetric checks a single spam/virus metric for anomalies
func (m *Manager) checkAnomalyMetric(pmg models.PMGInstance, tracker *pmgAnomalyTracker, metricName string, current float64, extractor func(pmgMailMetricSample) float64, now time.Time) {
	// Extract historical values for this metric (excluding current sample)
	m.mu.RLock()
	samples := tracker.Samples
	m.mu.RUnlock()

	if len(samples) < 2 {
		return
	}

	// Get previous 24 samples (or all available if less than 25 total)
	startIdx := 0
	if len(samples) > 25 {
		startIdx = len(samples) - 25
	}
	historicalSamples := samples[startIdx : len(samples)-1] // Exclude current (last) sample

	// Extract metric values
	values := make([]float64, 0, len(historicalSamples))
	for _, s := range historicalSamples {
		values = append(values, extractor(s))
	}

	// Calculate baseline
	baseline, trustworthy := calculateTrimmedBaseline(values)
	if !trustworthy {
		return
	}

	alertID := fmt.Sprintf("%s-anomaly-%s", pmg.ID, metricName)
	pendingKey := fmt.Sprintf("pmg-anomaly-%s-%s", pmg.ID, metricName)
	spec, err := buildCanonicalBaselineAnomalySpec(alertID, pmg.ID, pmg.Name, unifiedresources.ResourceTypePMG, metricName, 2, false)
	if err != nil {
		log.Warn().
			Err(err).
			Str("pmg", pmg.Name).
			Str("metric", metricName).
			Str("alertID", alertID).
			Msg("Skipping invalid canonical PMG anomaly spec")
		return
	}

	result, _ := m.evaluateCanonicalStatefulAlert(canonicalStatefulAlertParams{
		Spec:            spec,
		Evidence:        alertspecs.AlertEvidence{ObservedAt: now, BaselineAnomaly: &alertspecs.BaselineAnomalyEvidence{Metric: metricName, Observed: current, Baseline: baseline}},
		PendingTracking: m.pendingAlerts,
		PendingKey:      pendingKey,
		AlertID:         alertID,
		AlertType:       fmt.Sprintf("anomaly-%s", metricName),
		ResourceID:      pmg.ID,
		ResourceName:    pmg.Name,
		Node:            pmg.Host,
		Instance:        pmg.Name,
		MessageBuilder: func(_ alertspecs.EvaluationResult) (string, float64, float64) {
			return anomalyAlertMessage(pmg, metricName, current, baseline), current, baseline
		},
		DispatchAsync: true,
	})

	if result.State.State == alertspecs.AlertStatePending {
		log.Debug().
			Str("pmg", pmg.Name).
			Str("metric", metricName).
			Float64("current", current).
			Float64("baseline", baseline).
			Msg("PMG anomaly pending confirmation (first sample)")
		return
	}

	if result.Transition != nil && result.Transition.Kind == alertspecs.EvaluationTransitionActivated {
		pendingSince := result.Previous.FirstMatchedAt
		if pendingSince.IsZero() {
			pendingSince = now
		}

		level, ok := alertLevelFromCanonicalSeverity(result.State.Severity)
		if !ok {
			level = AlertLevelWarning
		}
		ratio := 0.0
		effectiveBaseline := baseline
		if effectiveBaseline == 0 && current > 0 {
			effectiveBaseline = 1
		}
		if effectiveBaseline > 0 {
			ratio = current / effectiveBaseline
		}
		log.Debug().
			Str("pmg", pmg.Name).
			Str("metric", metricName).
			Float64("current", current).
			Float64("baseline", baseline).
			Dur("pending", now.Sub(pendingSince)).
			Msg("PMG anomaly confirmed (second sample)")

		log.Warn().
			Str("pmg", pmg.Name).
			Str("metric", metricName).
			Float64("current", current).
			Float64("baseline", baseline).
			Float64("ratio", ratio).
			Str("level", string(level)).
			Msg("PMG anomaly alert triggered")
	}
}
