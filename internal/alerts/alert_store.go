package alerts

func activeAlertStorageKey(alert *Alert, fallback string) string {
	if fallback == "" && alert == nil {
		return ""
	}
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

func effectiveAlertID(alert *Alert, fallback string) string {
	if alert != nil {
		backfillCanonicalIdentity(alert)
		if alert.LegacyID != "" {
			return alert.LegacyID
		}
		if alert.ID != "" {
			return alert.ID
		}
	}
	return fallback
}

func (m *Manager) registerActiveAlertAliasNoLock(storageKey string, alert *Alert) {
	if m == nil || alert == nil || storageKey == "" {
		return
	}
	if m.activeAlertAlias == nil {
		m.activeAlertAlias = make(map[string]string)
	}
	backfillCanonicalIdentity(alert)
	if alias := effectiveAlertID(alert, ""); alias != "" && alias != storageKey {
		m.activeAlertAlias[alias] = storageKey
	}
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

func (m *Manager) hasActiveAlertNoLock(id string) bool {
	_, ok := m.resolveActiveAlertKeyNoLock(id)
	return ok
}

func (m *Manager) setActiveAlertNoLock(storageKey string, alert *Alert) {
	if storageKey == "" || alert == nil {
		return
	}
	backfillCanonicalIdentity(alert)
	requestedKey := storageKey
	storageKey = activeAlertStorageKey(alert, storageKey)
	for _, staleKey := range []string{requestedKey, alert.ID, alert.CanonicalState} {
		if staleKey == "" || staleKey == storageKey {
			continue
		}
		if existing, ok := m.activeAlerts[staleKey]; ok {
			delete(m.activeAlerts, staleKey)
			m.unregisterActiveAlertAliasNoLock(staleKey, existing)
		}
	}
	m.activeAlerts[storageKey] = alert
	m.registerActiveAlertAliasNoLock(storageKey, alert)
}

func (m *Manager) unregisterActiveAlertAliasNoLock(storageKey string, alert *Alert) {
	if m == nil || storageKey == "" {
		return
	}
	if alert != nil {
		backfillCanonicalIdentity(alert)
		if alias := effectiveAlertID(alert, ""); alias != "" && alias != storageKey {
			delete(m.activeAlertAlias, alias)
		}
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
	if alias := effectiveAlertID(resolved.Alert, ""); alias != "" && alias != storageKey {
		m.resolvedAlias[alias] = storageKey
	}
	if resolved.Alert.CanonicalState != "" && resolved.Alert.CanonicalState != storageKey {
		m.resolvedAlias[resolved.Alert.CanonicalState] = storageKey
	}
}

func (m *Manager) unregisterResolvedAliasUnlocked(storageKey string, resolved *ResolvedAlert) {
	if m == nil || storageKey == "" {
		return
	}
	if resolved != nil && resolved.Alert != nil {
		backfillCanonicalIdentity(resolved.Alert)
		if alias := effectiveAlertID(resolved.Alert, ""); alias != "" && alias != storageKey {
			delete(m.resolvedAlias, alias)
		}
		if resolved.Alert.CanonicalState != "" && resolved.Alert.CanonicalState != storageKey {
			delete(m.resolvedAlias, resolved.Alert.CanonicalState)
		}
	}
}

func (m *Manager) removeResolvedAlertUnlocked(id string) (*ResolvedAlert, bool) {
	resolved, ok := m.getResolvedAlertNoLock(id)
	if !ok || resolved == nil {
		return nil, false
	}

	storageKey := id
	if resolved.Alert != nil {
		storageKey = activeAlertStorageKey(resolved.Alert, id)
	}
	if key, exists := m.resolvedAlias[id]; exists {
		storageKey = key
	}

	delete(m.recentlyResolved, storageKey)
	m.unregisterResolvedAliasUnlocked(storageKey, resolved)
	return resolved, true
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
