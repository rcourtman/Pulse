package alerts

import (
	"strings"
	"testing"
)

func testNewCanonicalAlert(resourceID, specID, kind, alertType string) (string, *Alert) {
	alert := &Alert{
		ID:         buildCanonicalStateID(resourceID, specID),
		Type:       alertType,
		ResourceID: resourceID,
	}
	applyCanonicalIdentity(alert, specID, kind)
	return alert.CanonicalState, alert
}

func testAlertEquivalentIDs(alert *Alert) []string {
	if alert == nil {
		return nil
	}

	ids := make([]string, 0, 8)
	addID := func(id string) {
		if id == "" {
			return
		}
		for _, existing := range ids {
			if existing == id {
				return
			}
		}
		ids = append(ids, id)
	}

	addID(alert.ID)
	addID(alert.CanonicalSpecID)
	if alert.CanonicalSpecID != "" && alert.ResourceID != "" {
		addID(buildCanonicalStateID(alert.ResourceID, alert.CanonicalSpecID))
	}
	if strings.HasSuffix(alert.ID, canonicalStateSeparator+alert.CanonicalSpecID) {
		addID(alert.CanonicalSpecID)
	}

	switch {
	case alert.Type == "snapshot-age" && strings.Contains(alert.CanonicalSpecID, "/snapshot:"):
		parts := strings.SplitN(alert.CanonicalSpecID, "/snapshot:", 2)
		if len(parts) == 2 {
			addID("snapshot-age-" + parts[1])
		}
	case alert.Type == "backup-age" && strings.HasSuffix(alert.CanonicalSpecID, "-backup-age"):
		subjectID := strings.TrimSuffix(alert.CanonicalSpecID, "-backup-age")
		addID("backup-age-" + sanitizeAlertKey(subjectID))
	case alert.Type == "powered-off" || strings.HasSuffix(alert.CanonicalSpecID, "-powered-state"):
		addID("guest-powered-off-" + alert.ResourceID)
	case strings.HasSuffix(alert.CanonicalSpecID, "-connectivity"):
		switch {
		case strings.HasPrefix(alert.ResourceID, "agent:"):
			addID("host-offline-" + strings.TrimPrefix(alert.ResourceID, "agent:"))
		case strings.HasPrefix(alert.ResourceID, "docker:"):
			addID("docker-host-offline-" + strings.TrimPrefix(alert.ResourceID, "docker:"))
		default:
			resourceType, _ := alert.Metadata["resourceType"].(string)
			switch resourceType {
			case "node":
				addID("node-offline-" + alert.ResourceID)
			case "pbs":
				addID("pbs-offline-" + alert.ResourceID)
			case "storage":
				addID("storage-offline-" + alert.ResourceID)
			case "pmg":
				addID("pmg-offline-" + alert.ResourceID)
			}
		}
	}

	if strings.HasSuffix(alert.CanonicalSpecID, "-runtime-state") {
		addID("docker-container-state-" + alert.ResourceID)
	}
	if strings.HasSuffix(alert.CanonicalSpecID, "-service-gap") || strings.HasSuffix(alert.CanonicalSpecID, "-update-state") {
		addID("docker-service-health-" + alert.ResourceID)
	}
	if alert.Type == "docker-container-health" && alert.ResourceID != "" {
		addID("docker-container-health-" + alert.ResourceID)
	}
	if alert.Type == "docker-container-restart-loop" && alert.ResourceID != "" {
		addID("docker-container-restart-loop-" + alert.ResourceID)
	}
	if alert.Type == "docker-container-oom-kill" && alert.ResourceID != "" {
		addID("docker-container-oom-" + alert.ResourceID)
	}
	if alert.Type == "docker-container-memory-limit" && alert.ResourceID != "" {
		addID("docker-container-memory-limit-" + alert.ResourceID)
	}
	if strings.HasSuffix(alert.CanonicalSpecID, "-image-update") {
		addID("docker-container-update-" + alert.ResourceID)
	}
	if (alert.Type == "disk-health" || alert.Type == "disk-wearout") && alert.Instance != "" && alert.Node != "" {
		if devPath, _ := alert.Metadata["disk_path"].(string); devPath != "" {
			addID(alert.Type + "-" + alert.Instance + "-" + alert.Node + "-" + devPath)
		}
	}

	if provider, ok := alert.Metadata["incidentProvider"].(string); ok {
		if nativeID, ok := alert.Metadata["incidentNativeID"].(string); ok {
			if code, ok := alert.Metadata["incidentCode"].(string); ok {
				addID(unifiedIncidentAlertPrefix + sanitizeAlertKey(strings.Join([]string{alert.ResourceID, provider, nativeID, code}, "-")))
			}
		}
	}

	return ids
}

func testCanonicalAlertLookupNoLock(alerts map[string]*Alert, alertID string) (*Alert, bool) {
	if alert, exists := alerts[alertID]; exists && alert != nil {
		return alert, true
	}
	if strings.Contains(alertID, canonicalStateSeparator) {
		return nil, false
	}
	for _, alert := range alerts {
		if alert == nil {
			continue
		}
		for _, equivalentID := range testAlertEquivalentIDs(alert) {
			if equivalentID == alertID {
				return alert, true
			}
		}
	}
	return nil, false
}

func testLookupActiveAlert(t testing.TB, m *Manager, alertID string) (*Alert, bool) {
	t.Helper()

	m.mu.RLock()
	defer m.mu.RUnlock()

	if alert, exists := m.getActiveAlertNoLock(alertID); exists {
		return alert, true
	}
	return testCanonicalAlertLookupNoLock(m.activeAlerts, alertID)
}

func testRequireActiveAlert(t testing.TB, m *Manager, alertID string) *Alert {
	t.Helper()

	alert, exists := testLookupActiveAlert(t, m, alertID)
	if !exists || alert == nil {
		t.Fatalf("expected active alert %q", alertID)
	}
	return alert
}

func testHasActiveAlert(t testing.TB, m *Manager, alertID string) bool {
	t.Helper()

	_, exists := testLookupActiveAlert(t, m, alertID)
	return exists
}

func testLookupResolvedAlert(t testing.TB, m *Manager, alertID string) (*ResolvedAlert, bool) {
	t.Helper()

	m.resolvedMutex.RLock()
	defer m.resolvedMutex.RUnlock()

	if resolved, exists := m.getResolvedAlertNoLock(alertID); exists {
		return resolved, true
	}
	if strings.Contains(alertID, canonicalStateSeparator) {
		return nil, false
	}
	for _, resolved := range m.recentlyResolved {
		if resolved == nil || resolved.Alert == nil {
			continue
		}
		for _, equivalentID := range testAlertEquivalentIDs(resolved.Alert) {
			if equivalentID == alertID {
				return resolved, true
			}
		}
	}
	return nil, false
}
