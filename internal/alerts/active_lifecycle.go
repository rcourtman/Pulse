package alerts

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// addRecentlyResolvedUnlocked records a resolved alert assuming the caller does not hold m.mu.
func (m *Manager) addRecentlyResolvedUnlocked(resolved *ResolvedAlert) {
	m.resolvedMutex.Lock()
	if resolved == nil || resolved.Alert == nil {
		m.resolvedMutex.Unlock()
		return
	}
	storageKey := activeAlertStorageKey(resolved.Alert, resolved.Alert.ID)
	m.recentlyResolved[storageKey] = resolved
	m.registerResolvedAliasUnlocked(storageKey, resolved)
	m.pruneRecentlyResolvedUnlocked(time.Now())
	m.resolvedMutex.Unlock()
}

func (m *Manager) pruneRecentlyResolvedUnlocked(now time.Time) {
	type candidate struct {
		key        string
		resolvedAt time.Time
	}

	cutoff := now.Add(-recentlyResolvedRetention)
	candidates := make([]candidate, 0, len(m.recentlyResolved))
	for key, resolved := range m.recentlyResolved {
		if resolved == nil || resolved.ResolvedTime.Before(cutoff) {
			m.removeResolvedAlertUnlocked(key)
			continue
		}
		candidates = append(candidates, candidate{key: key, resolvedAt: resolved.ResolvedTime})
	}

	overflow := len(m.recentlyResolved) - maxRecentlyResolvedAlerts
	if overflow <= 0 {
		return
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].resolvedAt.Before(candidates[j].resolvedAt)
	})

	for _, candidate := range candidates {
		if overflow <= 0 {
			return
		}
		if _, removed := m.removeResolvedAlertUnlocked(candidate.key); removed {
			overflow--
		}
	}
}

// addRecentlyResolvedWithPrimaryLock records a resolved alert while preserving the caller's
// ownership of m.mu. Callers must hold m.mu before invoking this helper.
func (m *Manager) addRecentlyResolvedWithPrimaryLock(resolved *ResolvedAlert) {
	m.mu.Unlock()
	m.addRecentlyResolvedUnlocked(resolved)
	m.mu.Lock()
}

// clearAlert removes an alert if it exists.
func (m *Manager) clearAlert(alertID string) {
	m.mu.Lock()
	alert, exists := m.getActiveAlertNoLock(alertID)
	if exists {
		m.removeActiveAlertNoLock(alertID)
	}
	m.mu.Unlock()

	if !exists {
		return
	}

	publicID := effectiveAlertID(alert, alertID)
	resolvedAlert := m.newResolvedAlert(alert, time.Now(), nil)

	m.addRecentlyResolvedUnlocked(resolvedAlert)

	m.safeCallResolvedAlertCallback(alert, publicID, false)

	log.Info().
		Str("alertID", publicID).
		Msg("Alert cleared")
}

// AcknowledgeAlert acknowledges an alert.
func (m *Manager) AcknowledgeAlert(alertID, user string) error {
	m.mu.Lock()

	key, exists := m.resolveActiveAlertKeyNoLock(alertID)
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("%w: %s", ErrAlertNotFound, alertID)
	}
	alert, ok := m.getActiveAlertNoLock(key)
	if !ok || alert == nil {
		m.mu.Unlock()
		return fmt.Errorf("%w: %s", ErrAlertNotFound, alertID)
	}

	alert.Acknowledged = true
	now := time.Now()
	alert.AckTime = &now
	alert.AckUser = user

	m.setActiveAlertNoLock(key, alert)
	m.setAckRecordNoLock(alert, alertID, ackRecord{
		acknowledged: true,
		user:         user,
		time:         now,
	})

	alertCopy := alert.Clone()
	m.mu.Unlock()

	log.Debug().
		Str("alertID", alertID).
		Str("user", user).
		Time("ackTime", now).
		Msg("Alert acknowledgment recorded")

	m.safeCallAcknowledgedCallback(alertCopy, user)
	return nil
}

// UnacknowledgeAlert removes the acknowledged status from an alert.
func (m *Manager) UnacknowledgeAlert(alertID string) error {
	m.mu.Lock()

	key, exists := m.resolveActiveAlertKeyNoLock(alertID)
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("%w: %s", ErrAlertNotFound, alertID)
	}
	alert, ok := m.getActiveAlertNoLock(key)
	if !ok || alert == nil {
		m.mu.Unlock()
		return fmt.Errorf("%w: %s", ErrAlertNotFound, alertID)
	}

	alert.Acknowledged = false
	alert.AckTime = nil
	alert.AckUser = ""

	m.setActiveAlertNoLock(key, alert)
	m.deleteAckRecordNoLock(alert, alertID)

	alertCopy := alert.Clone()
	m.mu.Unlock()

	log.Info().
		Str("alertID", alertID).
		Msg("Alert unacknowledged")

	m.safeCallUnacknowledgedCallback(alertCopy, "")
	return nil
}

// preserveAlertState copies acknowledgement and escalation metadata from an existing alert
// into a freshly constructed alert before it replaces the existing entry in the map. This
// prevents UI state from regressing when alerts are rebuilt during polling.
func (m *Manager) preserveAlertState(alertID string, updated *Alert) {
	if updated == nil {
		return
	}
	backfillCanonicalIdentity(updated)

	if updated.NodeDisplayName == "" && updated.Node != "" {
		updated.NodeDisplayName = m.resolveNodeDisplayName(updated.Instance, updated.Node)
	}

	existing, exists := m.getActiveAlertNoLock(alertID)
	if exists && existing != nil {
		updated.StartTime = existing.StartTime
		if existing.LastNotified != nil {
			t := *existing.LastNotified
			updated.LastNotified = &t
		} else {
			updated.LastNotified = nil
		}
		updated.Acknowledged = existing.Acknowledged
		updated.AckUser = existing.AckUser
		if existing.AckTime != nil {
			t := *existing.AckTime
			updated.AckTime = &t
		} else {
			updated.AckTime = nil
		}
		updated.LastEscalation = existing.LastEscalation
		if len(existing.EscalationTimes) > 0 {
			updated.EscalationTimes = append([]time.Time(nil), existing.EscalationTimes...)
		} else {
			updated.EscalationTimes = nil
		}
		if existing.OperationalRecord != nil {
			value := existing.OperationalRecord.Clone()
			updated.OperationalRecord = &value
		}
		if existing.LatestTransition != nil {
			value := existing.LatestTransition.Clone()
			updated.LatestTransition = &value
		}
		for _, transition := range existing.Transitions {
			updated.Transitions = appendOperationalTransition(
				updated.Transitions,
				transition.Clone(),
			)
		}
		for _, envelope := range existing.Evidence {
			updated.Evidence = appendOperationalEvidence(updated.Evidence, envelope.Clone())
		}

		log.Debug().
			Str("alertID", alertID).
			Time("originalStartTime", existing.StartTime).
			Dur("currentDuration", time.Since(existing.StartTime)).
			Msg("Preserving alert state including StartTime")
		return
	}

	if m.historyManager != nil {
		previous := m.historyManager.LatestAlertForAlert(updated)
		if previous != nil &&
			previous.OperationalRecord != nil &&
			previous.OperationalRecord.ResolvedAt != nil &&
			previous.OperationalRecord.ResolvedAt.After(time.Now().Add(-recentlyResolvedRetention)) {
			if !previous.StartTime.IsZero() {
				updated.StartTime = previous.StartTime
			}
			mergeOperationalRecurrence(updated, previous, updated.LastSeen)
		}
	}

	if record, ok := m.getAckRecordNoLock(updated, alertID); ok && record.acknowledged {
		updated.Acknowledged = true
		updated.AckUser = record.user
		t := record.time
		updated.AckTime = &t
	}
}

func (m *Manager) removeActiveAlertNoLock(alertID string) {
	publicID := alertID
	var currentAlert *Alert
	key, exists := m.resolveActiveAlertKeyNoLock(alertID)
	if !exists {
		key, exists = m.resolveActiveAlertKeyByCanonicalStateNoLock(alertID)
	}
	if alert, ok := m.getActiveAlertNoLock(alertID); exists && ok && alert != nil {
		currentAlert = alert
		backfillCanonicalIdentity(alert)
		publicID = effectiveAlertID(alert, alertID)
		m.historyManager.UpdateAlertLastSeenForAlert(alert, alert.LastSeen)
		m.unregisterActiveAlertAliasNoLock(key, alert)
	}
	if exists {
		delete(m.offlineRecoveryConfirmations, key)
		delete(m.activeAlerts, key)
	}
	delete(m.offlineRecoveryConfirmations, alertID)

	// Preserve acknowledgement state so quick alert rebuilds keep user intent.
	if exists {
		m.markAckInactiveNoLock(currentAlert, publicID, time.Now())
	}
}

func (m *Manager) confirmOfflineRecoveryNoLock(alertID string, required int) (int, bool) {
	alertID = strings.TrimSpace(alertID)
	if alertID == "" {
		return 0, false
	}

	if required <= 1 {
		delete(m.offlineRecoveryConfirmations, alertID)
		return required, true
	}

	m.offlineRecoveryConfirmations[alertID]++
	confirmations := m.offlineRecoveryConfirmations[alertID]
	if confirmations < required {
		return confirmations, false
	}

	delete(m.offlineRecoveryConfirmations, alertID)
	return confirmations, true
}

// clearResourceOfflineAlert removes an offline alert when a poll-driven resource
// stays healthy for enough consecutive polls to confirm recovery.
func (m *Manager) clearResourceOfflineAlert(resourceID, resourceName, host, resourceKind string, requiredRecoveryCount int) {
	alertID := canonicalConnectivityStateID(resourceID)

	m.mu.Lock()
	defer m.mu.Unlock()

	if count, exists := m.offlineConfirmations[resourceID]; exists && count > 0 {
		log.Debug().
			Str(strings.ToLower(resourceKind), resourceName).
			Int("previousCount", count).
			Msg(resourceKind + " is online, resetting offline confirmation count")
		delete(m.offlineConfirmations, resourceID)
	}

	alert, exists := m.getActiveAlertNoLock(alertID)
	if !exists {
		delete(m.offlineRecoveryConfirmations, alertID)
		return
	}

	recoveryCount, confirmed := m.confirmOfflineRecoveryNoLock(alertID, requiredRecoveryCount)
	if !confirmed {
		log.Debug().
			Str(strings.ToLower(resourceKind), resourceName).
			Int("confirmations", recoveryCount).
			Int("required", requiredRecoveryCount).
			Msg(resourceKind + " appears back online, waiting for recovery confirmation")
		return
	}

	m.removeActiveAlertNoLock(alertID)

	resolvedAlert := m.newResolvedAlert(alert, time.Now(), nil)
	m.addRecentlyResolvedWithPrimaryLock(resolvedAlert)

	m.safeCallResolvedAlertCallback(alert, alertID, true)

	log.Info().
		Str(strings.ToLower(resourceKind), resourceName).
		Str("host", host).
		Dur("downtime", time.Since(alert.StartTime)).
		Msg(resourceKind + " instance is back online")
}

// ClearAlert removes an alert from active alerts while keeping it in history.
func (m *Manager) ClearAlert(alertID string) bool {
	m.mu.Lock()
	alert, exists := m.getActiveAlertNoLock(alertID)
	if !exists || alert == nil {
		m.mu.Unlock()
		return false
	}
	trackingKey := canonicalTrackingKeyForAlert(alert)

	m.clearAlertNoLock(alertID)
	delete(m.recentAlerts, alertID)
	delete(m.pendingAlerts, alertID)
	delete(m.suppressedUntil, alertID)
	delete(m.alertRateLimit, alertID)
	if trackingKey != "" && trackingKey != alertID {
		delete(m.recentAlerts, trackingKey)
		delete(m.pendingAlerts, trackingKey)
		delete(m.suppressedUntil, trackingKey)
		delete(m.alertRateLimit, trackingKey)
	}
	m.mu.Unlock()

	m.saveActiveAlertsAsync("manual-clear")
	return true
}

// clearAlertNoLock clears an alert without locking. Caller must hold m.mu.
func (m *Manager) clearAlertNoLock(alertID string) {
	alert, exists := m.getActiveAlertNoLock(alertID)
	if !exists {
		return
	}
	publicID := effectiveAlertID(alert, alertID)

	if recordAlertResolved != nil {
		recordAlertResolved(alert)
	}

	m.removeActiveAlertNoLock(alertID)
	resolvedAlert := m.newResolvedAlert(alert, time.Now(), nil)

	m.addRecentlyResolvedWithPrimaryLock(resolvedAlert)

	m.safeCallResolvedAlertCallback(alert, publicID, true)

	log.Info().
		Str("alertID", publicID).
		Msg("Alert cleared")
}

func (m *Manager) clearActiveAlertIfPresentNoLock(alertID string) bool {
	if _, exists := m.getActiveAlertNoLock(alertID); !exists {
		return false
	}
	m.clearAlertNoLock(alertID)
	return true
}
