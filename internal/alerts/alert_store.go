package alerts

func (m *Manager) registerActiveAlertAliasNoLock(storageKey string, alert *Alert) {
	if m == nil || alert == nil || storageKey == "" {
		return
	}
	if m.activeAlertAlias == nil {
		m.activeAlertAlias = make(map[string]string)
	}
	backfillCanonicalIdentity(alert)
	if alert.CanonicalState != "" && alert.CanonicalState != storageKey {
		m.activeAlertAlias[alert.CanonicalState] = storageKey
	}
}

func (m *Manager) resolveActiveAlertKeyNoLock(id string) (string, bool) {
	if _, ok := m.activeAlerts[id]; ok {
		return id, true
	}
	key, ok := m.activeAlertAlias[id]
	if !ok {
		return "", false
	}
	if _, exists := m.activeAlerts[key]; !exists {
		delete(m.activeAlertAlias, id)
		return "", false
	}
	return key, true
}

func (m *Manager) getActiveAlertNoLock(id string) (*Alert, bool) {
	key, ok := m.resolveActiveAlertKeyNoLock(id)
	if !ok {
		return nil, false
	}
	alert := m.activeAlerts[key]
	if alert == nil {
		return nil, false
	}
	backfillCanonicalIdentity(alert)
	return alert, true
}

func (m *Manager) setActiveAlertNoLock(storageKey string, alert *Alert) {
	if storageKey == "" || alert == nil {
		return
	}
	backfillCanonicalIdentity(alert)
	m.activeAlerts[storageKey] = alert
	m.registerActiveAlertAliasNoLock(storageKey, alert)
}

func (m *Manager) unregisterActiveAlertAliasNoLock(storageKey string, alert *Alert) {
	if m == nil || storageKey == "" {
		return
	}
	if alert != nil {
		backfillCanonicalIdentity(alert)
		if alert.CanonicalState != "" && alert.CanonicalState != storageKey {
			delete(m.activeAlertAlias, alert.CanonicalState)
		}
	}
}

func (m *Manager) registerResolvedAliasUnlocked(storageKey string, resolved *ResolvedAlert) {
	if m == nil || resolved == nil || resolved.Alert == nil || storageKey == "" {
		return
	}
	if m.resolvedAlias == nil {
		m.resolvedAlias = make(map[string]string)
	}
	backfillCanonicalIdentity(resolved.Alert)
	if resolved.Alert.CanonicalState != "" && resolved.Alert.CanonicalState != storageKey {
		m.resolvedAlias[resolved.Alert.CanonicalState] = storageKey
	}
}

func (m *Manager) getResolvedAlertNoLock(id string) (*ResolvedAlert, bool) {
	if resolved, ok := m.recentlyResolved[id]; ok && resolved != nil {
		return resolved, true
	}
	key, ok := m.resolvedAlias[id]
	if !ok {
		return nil, false
	}
	resolved, exists := m.recentlyResolved[key]
	if !exists || resolved == nil {
		delete(m.resolvedAlias, id)
		return nil, false
	}
	return resolved, true
}
