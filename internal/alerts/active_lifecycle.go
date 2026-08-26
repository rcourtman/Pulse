package alerts

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/reducer"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
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

// consumeRecentlyResolvedForRefireWithPrimaryLock consumes a resolved alert
// that is still inside the refire window. The final return value reports that
// a prior resolved occurrence exists even when it is too old to reactivate, so
// callers can explicitly append the genuine new occurrence to history.
// The caller must hold m.mu.
//
// Lock order is always m.mu -> resolvedMutex. This helper performs only
// resolved-map access while resolvedMutex is held; history, dispatch, and
// notification work remain the caller's responsibility.
func (m *Manager) consumeRecentlyResolvedForRefireWithPrimaryLock(storageKey string, now time.Time) (time.Time, time.Time, bool, bool) {
	m.resolvedMutex.Lock()
	defer m.resolvedMutex.Unlock()

	resolved, ok := m.getResolvedAlertNoLock(storageKey)
	if !ok || resolved == nil || resolved.Alert == nil {
		return time.Time{}, time.Time{}, false, false
	}
	if !resolved.ResolvedTime.After(now.Add(-recentlyResolvedRetention)) {
		return time.Time{}, resolved.ResolvedTime, false, true
	}

	startTime := resolved.Alert.StartTime
	resolvedAt := resolved.ResolvedTime
	m.removeResolvedAlertUnlocked(storageKey)
	return startTime, resolvedAt, true, true
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
		m.mirrorForgetAlertNoLock(alert)
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
	if alert.Acknowledged {
		m.mu.Unlock()
		return nil
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
	m.mirrorAcknowledgeNoLock(alert, user, now)

	alertCopy := alert.Clone()
	m.mu.Unlock()
	m.saveActiveAlertsAsync("acknowledge")

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
	if !alert.Acknowledged {
		m.mu.Unlock()
		return nil
	}

	alert.Acknowledged = false
	alert.AckTime = nil
	alert.AckUser = ""

	m.setActiveAlertNoLock(key, alert)
	m.deleteAckRecordNoLock(alert, alertID)
	m.mirrorUnacknowledgeNoLock(alert)

	alertCopy := alert.Clone()
	m.mu.Unlock()
	m.saveActiveAlertsAsync("unacknowledge")

	log.Info().
		Str("alertID", alertID).
		Msg("Alert unacknowledged")

	m.safeCallUnacknowledgedCallback(alertCopy, "")
	return nil
}

// SuppressOperationalAlert moves one canonical operational record out of the
// default attention queue without changing the detector's underlying finding.
// Suppression is bounded by an optional expiry and remains inspectable.
func (m *Manager) SuppressOperationalAlert(
	alertID string,
	actor string,
	reason string,
	expiresAt *time.Time,
) error {
	actor = strings.TrimSpace(actor)
	reason = strings.TrimSpace(reason)
	if actor == "" {
		return fmt.Errorf("suppression actor is required")
	}
	if reason == "" {
		return fmt.Errorf("suppression reason is required")
	}

	now := time.Now().UTC()
	if expiresAt != nil {
		value := expiresAt.UTC()
		if !value.After(now) {
			return fmt.Errorf("suppression expiry must be in the future")
		}
		expiresAt = &value
	}

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
	ensureOperationalContract(alert, now)
	if alert.OperationalRecord == nil {
		m.mu.Unlock()
		return fmt.Errorf("operational record is unavailable: %s", alertID)
	}

	from := alert.OperationalRecord.State
	alert.OperationalRecord.State = operationaltrust.OperationalSuppressed
	alert.OperationalRecord.StateChangedAt = now
	alert.OperationalRecord.Suppression = &operationaltrust.Suppression{
		At:        now,
		By:        actor,
		Reason:    reason,
		ExpiresAt: expiresAt,
	}
	appendExplicitOperationalTransition(
		alert,
		from,
		operationaltrust.OperationalSuppressed,
		now,
		operationaltrust.TransitionSuppression,
		reason,
		alert.OperationalRecord.EvidenceIDs,
	)
	m.setActiveAlertNoLock(key, alert)
	m.mu.Unlock()
	m.saveActiveAlertsAsync("operational suppression")
	return nil
}

// UnsuppressOperationalAlert returns a suppressed record to the detector-owned
// active state. It does not resolve or erase the underlying alert.
func (m *Manager) UnsuppressOperationalAlert(alertID string) error {
	return m.updateExplicitOperationalState(
		alertID,
		operationaltrust.OperationalOpen,
		operationaltrust.TransitionSuppressionExpired,
		"suppression_removed",
		nil,
		func(record *operationaltrust.OperationalRecord) {
			record.Suppression = nil
		},
	)
}

// MarkOperationalCollectionStale preserves an open finding when its source is
// no longer current. Missing observations therefore cannot resolve it.
func (m *Manager) MarkOperationalCollectionStale(
	alertID string,
	evidence operationaltrust.EvidenceEnvelope,
	reason string,
) error {
	return m.updateExplicitOperationalState(
		alertID,
		operationaltrust.OperationalStale,
		operationaltrust.TransitionCollectionStale,
		reason,
		&evidence,
		nil,
	)
}

// MarkOperationalCollectionUnknown preserves an open finding when collection
// completeness, permission, or provider state cannot support a stronger claim.
func (m *Manager) MarkOperationalCollectionUnknown(
	alertID string,
	evidence operationaltrust.EvidenceEnvelope,
	reason string,
) error {
	return m.updateExplicitOperationalState(
		alertID,
		operationaltrust.OperationalUnknown,
		operationaltrust.TransitionCollectionUnknown,
		reason,
		&evidence,
		nil,
	)
}

// MarkOperationalResolving records fresh recovery evidence while verification
// is still pending. Resolution remains detector-owned and requires a later
// decisive recovery observation.
func (m *Manager) MarkOperationalResolving(
	alertID string,
	evidence operationaltrust.EvidenceEnvelope,
	reason string,
) error {
	return m.updateExplicitOperationalState(
		alertID,
		operationaltrust.OperationalResolving,
		operationaltrust.TransitionRecoveryEvidence,
		reason,
		&evidence,
		nil,
	)
}

// RestoreOperationalCollectionState reopens a stale or unknown record from a
// fresh detector observation while retaining its timeline and evidence.
func (m *Manager) RestoreOperationalCollectionState(
	alertID string,
	evidence operationaltrust.EvidenceEnvelope,
) error {
	return m.updateExplicitOperationalState(
		alertID,
		operationaltrust.OperationalOpen,
		operationaltrust.TransitionDetectorDecision,
		"collection_restored",
		&evidence,
		nil,
	)
}

func (m *Manager) updateExplicitOperationalState(
	alertID string,
	state operationaltrust.OperationalState,
	cause operationaltrust.TransitionCause,
	reason string,
	evidence *operationaltrust.EvidenceEnvelope,
	mutate func(*operationaltrust.OperationalRecord),
) error {
	now := time.Now().UTC()
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return fmt.Errorf("operational transition reason is required")
	}
	if evidence != nil {
		if err := evidence.Validate(); err != nil {
			return fmt.Errorf("operational transition evidence: %w", err)
		}
	}

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
	ensureOperationalContract(alert, now)
	if alert.OperationalRecord == nil {
		m.mu.Unlock()
		return fmt.Errorf("operational record is unavailable: %s", alertID)
	}
	from := alert.OperationalRecord.State
	if from == state {
		if evidence != nil {
			alert.Evidence = appendOperationalEvidence(alert.Evidence, evidence.Clone())
			alert.OperationalRecord.EvidenceIDs = operationalEvidenceIDs(alert.Evidence)
			if evidence.ObservedAt.After(alert.OperationalRecord.LastObservedAt) {
				alert.OperationalRecord.LastObservedAt = evidence.ObservedAt.UTC()
			}
			m.setActiveAlertNoLock(key, alert)
			m.mu.Unlock()
			operationaltrust.GetMetrics().ObserveEvidence(*evidence, now)
			m.saveActiveAlertsAsync("operational evidence refresh")
			return nil
		}
		m.mu.Unlock()
		return nil
	}

	evidenceIDs := append([]string(nil), alert.OperationalRecord.EvidenceIDs...)
	if evidence != nil {
		alert.Evidence = appendOperationalEvidence(alert.Evidence, evidence.Clone())
		evidenceIDs = []string{evidence.ID}
		alert.OperationalRecord.EvidenceIDs = operationalEvidenceIDs(alert.Evidence)
		operationaltrust.GetMetrics().ObserveEvidence(*evidence, now)
	}
	alert.OperationalRecord.State = state
	alert.OperationalRecord.StateChangedAt = now
	alert.OperationalRecord.ResolvedAt = nil
	if mutate != nil {
		mutate(alert.OperationalRecord)
	}
	appendExplicitOperationalTransition(alert, from, state, now, cause, reason, evidenceIDs)
	m.setActiveAlertNoLock(key, alert)
	m.mu.Unlock()
	m.saveActiveAlertsAsync("operational lifecycle transition")
	return nil
}

func appendExplicitOperationalTransition(
	alert *Alert,
	from operationaltrust.OperationalState,
	to operationaltrust.OperationalState,
	at time.Time,
	cause operationaltrust.TransitionCause,
	reason string,
	evidenceIDs []string,
) {
	if alert == nil || alert.OperationalRecord == nil || from == to {
		return
	}
	id, err := operationaltrust.NewTransitionID(
		alert.OperationalRecord.ID,
		from,
		to,
		at,
		cause,
		alert.OperationalRecord.CauseKey,
		evidenceIDs,
	)
	if err != nil {
		return
	}
	transition := operationaltrust.LifecycleTransition{
		ID:                  id,
		OperationalRecordID: alert.OperationalRecord.ID,
		From:                from,
		To:                  to,
		At:                  at,
		Cause:               cause,
		CauseKey:            alert.OperationalRecord.CauseKey,
		EvidenceIDs:         append([]string(nil), evidenceIDs...),
		Reason:              strings.TrimSpace(reason),
	}
	alert.LatestTransition = &transition
	alert.Transitions = appendOperationalTransition(alert.Transitions, transition.Clone())
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
// stays healthy for enough consecutive polls to confirm recovery. The
// reducer core owns the recovery gate.
func (m *Manager) clearResourceOfflineAlert(resourceID, resourceName, host, resourceKind string, requiredRecoveryCount int) {
	alertID := canonicalConnectivityStateID(resourceID)

	m.mu.Lock()
	defer m.mu.Unlock()
	defer m.shadowObserveRecoveryNoLock(resourceID, canonicalConnectivitySpecID(resourceID), alertID, requiredRecoveryCount)

	m.resolveDiscreteRecoveryNoLock(resourceID, canonicalConnectivitySpecID(resourceID), alertID, requiredRecoveryCount, resourceKind, resourceName, host)
}

// resolveDiscreteRecoveryNoLock feeds one healthy observation for a
// poll-driven discrete condition to the reducer core and resolves the
// active alert once the core's recovery gate confirms. Caller holds m.mu.
func (m *Manager) resolveDiscreteRecoveryNoLock(resourceID, specKey, alertID string, requiredRecoveryCount int, resourceKind, resourceName, host string) {
	// An existing alert the core does not know about means it was firing:
	// adopt it before the healthy observation, exactly as the old engine
	// treated any active alert as previously firing.
	if adopted, adoptedExists := m.getActiveAlertNoLock(alertID); adoptedExists && adopted != nil {
		if _, known := m.core.Incident(resourceID, specKey); !known {
			ackAt := time.Time{}
			if adopted.AckTime != nil {
				ackAt = *adopted.AckTime
			}
			m.core.SeedFiringIncident(resourceID, specKey, shadowSeverityForLevel(adopted.Level), adopted.StartTime, adopted.Acknowledged, adopted.AckUser, ackAt)
		}
	}

	events := m.core.ApplyDiscrete(reducer.DiscreteSignal{
		ResourceID: resourceID,
		Key:        specKey,
		Matched:    false,
		ObservedAt: time.Now(),
	}, reducer.DiscreteRule{RecoveryConfirmations: requiredRecoveryCount})

	// Legacy-map mirrors (read-only during Phase 2; deleted in Phase 3):
	// a healthy poll clears any confirmation-run mirror and reflects the
	// core's recovery count.
	delete(m.offlineConfirmations, resourceID)
	if incident, tracked := m.core.Incident(resourceID, specKey); tracked && incident.RecoveryCount > 0 {
		m.offlineRecoveryConfirmations[alertID] = incident.RecoveryCount
	} else {
		delete(m.offlineRecoveryConfirmations, alertID)
	}

	alert, exists := m.getActiveAlertNoLock(alertID)
	if !exists {
		return
	}

	resolved := false
	for _, event := range events {
		if event.Type == reducer.EventResolved {
			resolved = true
			break
		}
	}
	if !resolved {
		// Self-heal in the authoritative direction: an alert with no core
		// incident resolves immediately rather than sticking.
		if _, hasIncident := m.core.Incident(resourceID, specKey); hasIncident {
			log.Debug().
				Str(strings.ToLower(resourceKind), resourceName).
				Int("required", requiredRecoveryCount).
				Msg(resourceKind + " appears back online, waiting for recovery confirmation")
			return
		}
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

	m.mirrorForgetAlertNoLock(alert)
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
