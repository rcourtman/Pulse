package alerts

import (
	"fmt"
	"strings"

	alertspecs "github.com/rcourtman/pulse-go-rewrite/internal/alerts/specs"
)

const canonicalStateSeparator = "::"

func buildCanonicalStateID(resourceID, specID string) string {
	if resourceID == "" || specID == "" {
		return ""
	}
	return resourceID + canonicalStateSeparator + specID
}

func canonicalMetricSpecID(resourceID, metric string) string {
	if resourceID == "" || metric == "" {
		return ""
	}
	return "metric-threshold:" + metric
}

func canonicalMetricStateID(resourceID, metric string) string {
	return buildCanonicalStateID(resourceID, canonicalMetricSpecID(resourceID, metric))
}

func canonicalConnectivitySpecID(resourceID string) string {
	if resourceID == "" {
		return ""
	}
	return resourceID + "-connectivity"
}

func canonicalConnectivityStateID(resourceID string) string {
	return buildCanonicalStateID(resourceID, canonicalConnectivitySpecID(resourceID))
}

func canonicalPoweredStateSpecID(resourceID string) string {
	if resourceID == "" {
		return ""
	}
	return resourceID + "-powered-state"
}

func canonicalPoweredStateStateID(resourceID string) string {
	return buildCanonicalStateID(resourceID, canonicalPoweredStateSpecID(resourceID))
}

func canonicalDiscreteStateSpecID(resourceID, stateKey string) string {
	if resourceID == "" || stateKey == "" {
		return ""
	}
	return resourceID + "-" + stateKey
}

func canonicalDiscreteStateStateID(resourceID, stateKey string) string {
	return buildCanonicalStateID(resourceID, canonicalDiscreteStateSpecID(resourceID, stateKey))
}

func canonicalServiceGapSpecID(resourceID string) string {
	if resourceID == "" {
		return ""
	}
	return resourceID + "-service-gap"
}

func canonicalServiceGapStateID(resourceID string) string {
	return buildCanonicalStateID(resourceID, canonicalServiceGapSpecID(resourceID))
}

func backfillCanonicalIdentity(alert *Alert) {
	if alert == nil {
		return
	}
	if inferredResourceID := inferCanonicalResourceIDFromLegacyAlert(alert); inferredResourceID != "" {
		alert.ResourceID = inferredResourceID
	}
	if alert.CanonicalSpecID == "" && alert.Metadata != nil {
		if specID, ok := alert.Metadata["canonicalSpecID"].(string); ok {
			alert.CanonicalSpecID = specID
		}
	}
	if alert.CanonicalSpecID == "" {
		alert.CanonicalSpecID = inferCanonicalSpecIDFromLegacyAlert(alert)
	}
	if alert.CanonicalKind == "" && alert.Metadata != nil {
		if kind, ok := alert.Metadata["canonicalAlertKind"].(string); ok {
			alert.CanonicalKind = kind
		}
	}
	if alert.CanonicalKind == "" {
		alert.CanonicalKind = inferCanonicalKindFromLegacyAlert(alert)
	}
	if alert.CanonicalState == "" {
		alert.CanonicalState = buildCanonicalStateID(alert.ResourceID, alert.CanonicalSpecID)
	}
}

func inferCanonicalResourceIDFromLegacyAlert(alert *Alert) string {
	if alert == nil {
		return ""
	}

	id := alert.ID
	if strings.Contains(id, canonicalStateSeparator) {
		parts := strings.SplitN(id, canonicalStateSeparator, 2)
		if len(parts) == 2 {
			return parts[0]
		}
	}
	switch {
	case strings.HasPrefix(id, "host-offline-"):
		return hostResourceID(strings.TrimPrefix(id, "host-offline-"))
	case strings.HasPrefix(id, "docker-host-offline-"):
		return "docker:" + strings.TrimPrefix(id, "docker-host-offline-")
	case strings.HasPrefix(id, "node-offline-"):
		return strings.TrimPrefix(id, "node-offline-")
	case strings.HasPrefix(id, "pbs-offline-"):
		return strings.TrimPrefix(id, "pbs-offline-")
	case strings.HasPrefix(id, "pmg-offline-"):
		return strings.TrimPrefix(id, "pmg-offline-")
	case strings.HasPrefix(id, "storage-offline-"):
		return strings.TrimPrefix(id, "storage-offline-")
	case strings.HasPrefix(id, "guest-powered-off-"):
		return strings.TrimPrefix(id, "guest-powered-off-")
	case strings.HasPrefix(id, "docker-container-state-"):
		return strings.TrimPrefix(id, "docker-container-state-")
	case strings.HasPrefix(id, "docker-container-health-"):
		return strings.TrimPrefix(id, "docker-container-health-")
	case strings.HasPrefix(id, "docker-container-restart-loop-"):
		return strings.TrimPrefix(id, "docker-container-restart-loop-")
	case strings.HasPrefix(id, "docker-container-oom-"):
		return strings.TrimPrefix(id, "docker-container-oom-")
	case strings.HasPrefix(id, "docker-container-memory-limit-"):
		return strings.TrimPrefix(id, "docker-container-memory-limit-")
	case strings.HasPrefix(id, "docker-container-update-"):
		return strings.TrimPrefix(id, "docker-container-update-")
	case strings.HasPrefix(id, "docker-service-health-"):
		return strings.TrimPrefix(id, "docker-service-health-")
	case strings.HasPrefix(id, "disk-health-"):
		if alert.Instance != "" && alert.Node != "" {
			prefix := fmt.Sprintf("disk-health-%s-%s-", alert.Instance, alert.Node)
			if strings.HasPrefix(id, prefix) {
				return proxmoxDiskCanonicalResourceID(alert.Instance, alert.Node, strings.TrimPrefix(id, prefix))
			}
		}
		parts := strings.SplitN(strings.TrimPrefix(id, "disk-health-"), "-", 3)
		if len(parts) == 3 {
			return proxmoxDiskCanonicalResourceID(parts[0], parts[1], parts[2])
		}
	case strings.HasPrefix(id, "disk-wearout-"):
		if alert.Instance != "" && alert.Node != "" {
			prefix := fmt.Sprintf("disk-wearout-%s-%s-", alert.Instance, alert.Node)
			if strings.HasPrefix(id, prefix) {
				return proxmoxDiskCanonicalResourceID(alert.Instance, alert.Node, strings.TrimPrefix(id, prefix))
			}
		}
		parts := strings.SplitN(strings.TrimPrefix(id, "disk-wearout-"), "-", 3)
		if len(parts) == 3 {
			return proxmoxDiskCanonicalResourceID(parts[0], parts[1], parts[2])
		}
	}

	switch alert.Type {
	case "cpu", "memory", "disk", "temperature", "usage":
		suffix := "-" + alert.Type
		if strings.HasSuffix(id, suffix) {
			return strings.TrimSuffix(id, suffix)
		}
	}

	return ""
}

func inferCanonicalSpecIDFromLegacyAlert(alert *Alert) string {
	if alert == nil {
		return ""
	}
	id := alert.ID
	if strings.Contains(id, canonicalStateSeparator) {
		parts := strings.SplitN(id, canonicalStateSeparator, 2)
		if len(parts) == 2 {
			return parts[1]
		}
	}
	resourceID := inferCanonicalResourceIDFromLegacyAlert(alert)
	switch {
	case strings.HasPrefix(id, "host-offline-"),
		strings.HasPrefix(id, "docker-host-offline-"),
		strings.HasPrefix(id, "node-offline-"),
		strings.HasPrefix(id, "pbs-offline-"),
		strings.HasPrefix(id, "pmg-offline-"),
		strings.HasPrefix(id, "storage-offline-"):
		return canonicalConnectivitySpecID(resourceID)
	case strings.HasPrefix(id, "guest-powered-off-"):
		return canonicalPoweredStateSpecID(resourceID)
	case strings.HasPrefix(id, "docker-container-state-"):
		return canonicalDiscreteStateSpecID(resourceID, "runtime-state")
	case strings.HasPrefix(id, "docker-service-health-"):
		return canonicalServiceGapSpecID(resourceID)
	case strings.HasPrefix(id, "docker-container-health-"):
		return resourceID + "-health"
	case strings.HasPrefix(id, "docker-container-restart-loop-"):
		return resourceID + "-restart-loop"
	case strings.HasPrefix(id, "docker-container-oom-"):
		return resourceID + "-oom-kill"
	case strings.HasPrefix(id, "docker-container-memory-limit-"):
		return resourceID + "-memory-limit"
	case strings.HasPrefix(id, "docker-container-update-"):
		return resourceID + "-image-update"
	case strings.HasPrefix(id, "disk-health-"):
		return resourceID + "-health"
	case strings.HasPrefix(id, "disk-wearout-"):
		return resourceID + "-wearout"
	}

	switch alert.Type {
	case "cpu", "memory", "disk", "temperature", "usage":
		return canonicalMetricSpecID(resourceID, alert.Type)
	}

	return ""
}

func inferCanonicalKindFromLegacyAlert(alert *Alert) string {
	if alert == nil {
		return ""
	}
	id := alert.ID
	if strings.Contains(id, canonicalStateSeparator) {
		specID := inferCanonicalSpecIDFromLegacyAlert(alert)
		switch {
		case strings.HasSuffix(specID, "-connectivity"):
			return string(alertspecs.AlertSpecKindConnectivity)
		case strings.HasSuffix(specID, "-powered-state"):
			return string(alertspecs.AlertSpecKindPoweredState)
		case strings.HasSuffix(specID, "-runtime-state"), strings.HasSuffix(specID, "-update-state"):
			return string(alertspecs.AlertSpecKindDiscreteState)
		case strings.HasSuffix(specID, "-service-gap"):
			return string(alertspecs.AlertSpecKindServiceGap)
		case strings.HasSuffix(specID, "-health"):
			return string(alertspecs.AlertSpecKindHealthAssessment)
		case strings.HasSuffix(specID, "-restart-loop"), strings.HasSuffix(specID, "-oom-kill"), strings.HasSuffix(specID, "-memory-limit"), strings.HasSuffix(specID, "-image-update"), strings.HasSuffix(specID, "-wearout"):
			return string(alertspecs.AlertSpecKindSeverityThreshold)
		case strings.Contains(specID, "/snapshot:"), strings.HasSuffix(specID, "-backup-age"):
			return string(alertspecs.AlertSpecKindPostureThreshold)
		case strings.HasPrefix(specID, "metric-threshold:") || strings.Contains(specID, "-cpu") || strings.Contains(specID, "-memory") || strings.Contains(specID, "-disk") || strings.Contains(specID, "-temperature") || strings.Contains(specID, "-usage"):
			return string(alertspecs.AlertSpecKindMetricThreshold)
		}
	}
	switch {
	case strings.HasPrefix(id, "host-offline-"),
		strings.HasPrefix(id, "docker-host-offline-"),
		strings.HasPrefix(id, "node-offline-"),
		strings.HasPrefix(id, "pbs-offline-"),
		strings.HasPrefix(id, "pmg-offline-"),
		strings.HasPrefix(id, "storage-offline-"):
		return string(alertspecs.AlertSpecKindConnectivity)
	case strings.HasPrefix(id, "guest-powered-off-"):
		return string(alertspecs.AlertSpecKindPoweredState)
	case strings.HasPrefix(id, "docker-container-state-"):
		return string(alertspecs.AlertSpecKindDiscreteState)
	case strings.HasPrefix(id, "docker-service-health-"):
		return string(alertspecs.AlertSpecKindServiceGap)
	case strings.HasPrefix(id, "docker-container-health-"),
		strings.HasPrefix(id, "disk-health-"):
		return string(alertspecs.AlertSpecKindHealthAssessment)
	case strings.HasPrefix(id, "docker-container-restart-loop-"),
		strings.HasPrefix(id, "docker-container-oom-"),
		strings.HasPrefix(id, "docker-container-memory-limit-"),
		strings.HasPrefix(id, "docker-container-update-"),
		strings.HasPrefix(id, "disk-wearout-"):
		return string(alertspecs.AlertSpecKindSeverityThreshold)
	}

	switch alert.Type {
	case "cpu", "memory", "disk", "temperature", "usage":
		return string(alertspecs.AlertSpecKindMetricThreshold)
	}

	return ""
}

func applyCanonicalIdentity(alert *Alert, specID, kind string) {
	if alert == nil {
		return
	}
	alert.CanonicalSpecID = specID
	alert.CanonicalKind = kind
	alert.CanonicalState = buildCanonicalStateID(alert.ResourceID, specID)
	if alert.Metadata == nil {
		alert.Metadata = make(map[string]interface{}, 2)
	}
	alert.Metadata["canonicalSpecID"] = specID
	alert.Metadata["canonicalAlertKind"] = kind
}

func exportedAlertID(alert *Alert, fallback string) string {
	if alert != nil {
		backfillCanonicalIdentity(alert)
		if alert.CanonicalState != "" {
			return alert.CanonicalState
		}
		if alert.ID != "" {
			return alert.ID
		}
	}
	return fallback
}

func cloneAlertForOutput(alert *Alert) *Alert {
	if alert == nil {
		return nil
	}
	clone := alert.Clone()
	backfillCanonicalIdentity(clone)
	publicID := exportedAlertID(clone, clone.ID)
	clone.ID = publicID
	return clone
}

func canonicalizeAlertHistoryForOutput(history []Alert) []Alert {
	if len(history) == 0 {
		return history
	}
	exported := make([]Alert, 0, len(history))
	for _, alert := range history {
		exportedAlert := cloneAlertForOutput(&alert)
		if exportedAlert == nil {
			continue
		}
		exported = append(exported, *exportedAlert)
	}
	return exported
}

func canonicalTrackingKeyForSpec(spec alertspecs.ResourceAlertSpec, fallback string) string {
	if key := buildCanonicalStateID(spec.ResourceID, spec.ID); key != "" {
		return key
	}
	return fallback
}

func canonicalTrackingKeyForAlert(alert *Alert) string {
	if alert == nil {
		return ""
	}
	backfillCanonicalIdentity(alert)
	if alert.CanonicalState != "" {
		return alert.CanonicalState
	}
	return alert.ID
}

func canonicalTrackingKeyOrFallback(alert *Alert, fallback string) string {
	if key := canonicalTrackingKeyForAlert(alert); key != "" {
		return key
	}
	return fallback
}

func (m *Manager) hasActiveAlertTrackingKeyNoLock(trackingKey string) bool {
	if trackingKey == "" {
		return false
	}
	if _, exists := m.activeAlerts[trackingKey]; exists {
		return true
	}
	for _, alert := range m.activeAlerts {
		if alert == nil {
			continue
		}
		if canonicalTrackingKeyForAlert(alert) == trackingKey {
			return true
		}
	}
	return false
}
