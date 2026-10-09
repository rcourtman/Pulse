package alerts

import (
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"
)

// Match temperature's existing departure confirmation without sharing its
// pending-run state or resource IDs. One failed probe can omit a disk.
const hostSMARTRiskAbsenceConfirmations = hostDiskTemperatureAbsenceConfirmations

func isHostSMARTRiskAlertType(alertType string) bool {
	return alertType == "disk-health" || alertType == "disk-wearout"
}

func hostSMARTRiskThresholds(thresholds ThresholdConfig) (storagehealth.SMARTThresholds, int64) {
	return storagehealth.SMARTThresholds{
		HealthFailure:        intValue(thresholds.SMARTHealthFailure) > 0,
		ReallocatedSectors:   int64Value(thresholds.SMARTReallocated),
		PendingSectors:       int64Value(thresholds.SMARTPending),
		OfflineUncorrectable: int64Value(thresholds.SMARTUncorrectable),
		MediaErrors:          int64Value(thresholds.SMARTMediaErrors),
		LifeWarning:          intValue(thresholds.SMARTLifeWarning),
		LifeCritical:         intValue(thresholds.SMARTLifeCritical),
		AvailableSpareWarn:   intValue(thresholds.SMARTSpareWarning),
		AvailableSpareCrit:   intValue(thresholds.SMARTSpareCritical),
	}, int64Value(thresholds.SMARTCRCErrorDelta)
}

// Missing risk metadata is not evidence that the rules which raised an old
// alert have been disabled. Unknown codes are conservatively enabled too.
func hostSMARTRiskRulesOff(alert *Alert, thresholds storagehealth.SMARTThresholds, crcMinimumDelta int64) bool {
	if alert == nil {
		return false
	}
	codes := hostSMARTRiskCodes(alert.Metadata["riskCodes"])
	if len(codes) == 0 {
		return false
	}
	for _, code := range codes {
		if hostSMARTRiskRuleEnabled(code, thresholds, crcMinimumDelta) {
			return false
		}
	}
	return true
}

// Caller holds m.mu. Keep counts only for resources still holding risk alerts.
// A disabled global policy breaks the observation sequence, not the alert.
func (m *Manager) pruneHostSMARTDiskAbsencesNoLock() {
	active := make(map[string]struct{})
	if m.config.Enabled {
		for _, alert := range m.activeAlerts {
			if alert != nil && isHostSMARTRiskAlertType(alert.Type) {
				active[alert.ResourceID] = struct{}{}
			}
		}
	}
	for resourceID := range m.hostSMARTRiskAbsences {
		if _, exists := active[resourceID]; !exists {
			delete(m.hostSMARTRiskAbsences, resourceID)
		}
	}
}

func (m *Manager) resetHostSMARTDiskAbsencesNoLock(hostID string) {
	prefix := hostResourceID(hostID) + "/disk:"
	for resourceID := range m.hostSMARTRiskAbsences {
		if strings.HasPrefix(resourceID, prefix) {
			delete(m.hostSMARTRiskAbsences, resourceID)
		}
	}
}

func (m *Manager) resetHostSMARTDiskAbsences(hostID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.resetHostSMARTDiskAbsencesNoLock(hostID)
}

func (m *Manager) clearHostSMARTDiskAlerts(hostID string) {
	prefix := hostResourceID(hostID) + "/disk:"
	m.mu.Lock()
	defer m.mu.Unlock()
	m.resetHostSMARTDiskAbsencesNoLock(hostID)
	for storageKey, alert := range m.activeAlerts {
		if alert != nil && isHostSMARTRiskAlertType(alert.Type) && strings.HasPrefix(alert.ResourceID, prefix) {
			m.clearAlertNoLock(storageKey)
		}
	}
}

// An empty report holds risk alerts and breaks departure confirmation. A
// listed disk (including standby or unknown fields) restarts its count; the
// ordinary evidence evaluator decides whether its risk actually recovered.
// Only three consecutive non-empty omissions release departed disk alerts.
// Explicit rule disablement is authoritative even without a disk reading.
func (m *Manager) cleanupHostSMARTDiskAlerts(hostID string, seen map[string]struct{}, thresholds ThresholdConfig) {
	prefix := hostResourceID(hostID) + "/disk:"
	smartThresholds, crcMinimumDelta := hostSMARTRiskThresholds(thresholds)
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.hostSMARTRiskAbsences == nil {
		m.hostSMARTRiskAbsences = make(map[string]int)
	}
	absent := make(map[string][]string)
	for storageKey, alert := range m.activeAlerts {
		if alert == nil || !isHostSMARTRiskAlertType(alert.Type) || !strings.HasPrefix(alert.ResourceID, prefix) {
			continue
		}
		if hostSMARTRiskRulesOff(alert, smartThresholds, crcMinimumDelta) {
			m.clearAlertNoLock(storageKey)
			continue
		}
		if _, exists := seen[alert.ResourceID]; !exists {
			absent[alert.ResourceID] = append(absent[alert.ResourceID], storageKey)
		}
	}
	for resourceID := range m.hostSMARTRiskAbsences {
		if !strings.HasPrefix(resourceID, prefix) {
			continue
		}
		if _, missing := absent[resourceID]; !missing || seen == nil {
			delete(m.hostSMARTRiskAbsences, resourceID)
		}
	}
	if seen == nil {
		return
	}
	for resourceID, storageKeys := range absent {
		m.hostSMARTRiskAbsences[resourceID]++
		if m.hostSMARTRiskAbsences[resourceID] < hostSMARTRiskAbsenceConfirmations {
			continue
		}
		delete(m.hostSMARTRiskAbsences, resourceID)
		for _, storageKey := range storageKeys {
			m.clearAlertNoLock(storageKey)
		}
	}
}
