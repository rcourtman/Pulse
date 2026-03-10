package alerts

import alertspecs "github.com/rcourtman/pulse-go-rewrite/internal/alerts/specs"

const canonicalStateSeparator = "::"

func buildCanonicalStateID(resourceID, specID string) string {
	if resourceID == "" || specID == "" {
		return ""
	}
	return resourceID + canonicalStateSeparator + specID
}

func backfillCanonicalIdentity(alert *Alert) {
	if alert == nil {
		return
	}
	if alert.LegacyID == "" && alert.Metadata != nil {
		if legacyID, ok := alert.Metadata["legacyAlertID"].(string); ok {
			alert.LegacyID = legacyID
		}
	}
	if alert.CanonicalSpecID == "" && alert.Metadata != nil {
		if specID, ok := alert.Metadata["canonicalSpecID"].(string); ok {
			alert.CanonicalSpecID = specID
		}
	}
	if alert.CanonicalKind == "" && alert.Metadata != nil {
		if kind, ok := alert.Metadata["canonicalAlertKind"].(string); ok {
			alert.CanonicalKind = kind
		}
	}
	if alert.CanonicalState == "" {
		alert.CanonicalState = buildCanonicalStateID(alert.ResourceID, alert.CanonicalSpecID)
	}
	if alert.CanonicalState != "" && alert.LegacyID == "" && alert.ID != "" && alert.ID != alert.CanonicalState {
		alert.LegacyID = alert.ID
		if alert.Metadata == nil {
			alert.Metadata = make(map[string]interface{}, 1)
		}
		alert.Metadata["legacyAlertID"] = alert.LegacyID
	}
}

func applyCanonicalIdentity(alert *Alert, specID, kind string) {
	if alert == nil {
		return
	}
	alert.CanonicalSpecID = specID
	alert.CanonicalKind = kind
	alert.CanonicalState = buildCanonicalStateID(alert.ResourceID, specID)
	if alert.Metadata == nil {
		alert.Metadata = make(map[string]interface{}, 3)
	}
	alert.Metadata["canonicalSpecID"] = specID
	alert.Metadata["canonicalAlertKind"] = kind
	if alert.ID != "" && alert.CanonicalState != "" && alert.ID != alert.CanonicalState {
		alert.LegacyID = alert.ID
		alert.Metadata["legacyAlertID"] = alert.ID
	}
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
	legacyID := effectiveAlertID(clone, "")
	if legacyID == publicID {
		legacyID = ""
	}
	clone.ID = publicID
	clone.LegacyID = legacyID
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
