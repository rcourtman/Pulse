package alerts

import "time"

func canonicalAckKey(alert *Alert, fallback string) string {
	if alert == nil {
		return fallback
	}
	backfillCanonicalIdentity(alert)
	if alert.CanonicalState != "" {
		return alert.CanonicalState
	}
	return fallback
}

func (m *Manager) getAckRecordNoLock(alert *Alert, fallbackID string) (ackRecord, bool) {
	if canonicalKey := canonicalAckKey(alert, ""); canonicalKey != "" {
		if record, ok := m.ackStateByCanonical[canonicalKey]; ok {
			return record, true
		}
	}
	publicID := effectiveAlertID(alert, fallbackID)
	if publicID == "" {
		return ackRecord{}, false
	}
	record, ok := m.ackState[publicID]
	return record, ok
}

func (m *Manager) setAckRecordNoLock(alert *Alert, fallbackID string, record ackRecord) {
	publicID := effectiveAlertID(alert, fallbackID)
	if canonicalKey := canonicalAckKey(alert, ""); canonicalKey != "" {
		m.ackStateByCanonical[canonicalKey] = record
		if publicID != "" {
			delete(m.ackState, publicID)
		}
		return
	}
	if publicID != "" {
		m.ackState[publicID] = record
	}
}

func (m *Manager) deleteAckRecordNoLock(alert *Alert, fallbackID string) {
	publicID := effectiveAlertID(alert, fallbackID)
	if publicID != "" {
		delete(m.ackState, publicID)
	}
	if canonicalKey := canonicalAckKey(alert, ""); canonicalKey != "" {
		delete(m.ackStateByCanonical, canonicalKey)
	}
}

func (m *Manager) markAckInactiveNoLock(alert *Alert, fallbackID string, inactiveAt time.Time) {
	if canonicalKey := canonicalAckKey(alert, ""); canonicalKey != "" {
		if record, ok := m.ackStateByCanonical[canonicalKey]; ok {
			record.inactiveAt = inactiveAt
			m.ackStateByCanonical[canonicalKey] = record
		}
	}

	publicID := effectiveAlertID(alert, fallbackID)
	if publicID == "" {
		return
	}
	if record, ok := m.ackState[publicID]; ok {
		record.inactiveAt = inactiveAt
		m.ackState[publicID] = record
	}
}
