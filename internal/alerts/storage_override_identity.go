package alerts

import (
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func storageThresholdLookupIDs(resourceID string, aliases ...string) []string {
	ids := make([]string, 0, len(aliases)+2)
	for _, id := range append([]string{resourceID}, aliases...) {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		ids = appendUniqueStorageLookupID(ids, id)
		if alias, ok := cephPoolStorageSourceAliasID(id); ok {
			ids = appendUniqueStorageLookupID(ids, alias)
		}
	}
	return ids
}

func storageAlertResourceIDs(storage models.Storage) []string {
	return storageThresholdLookupIDs(storage.ID, storage.AliasIDs...)
}

func appendUniqueStorageLookupID(ids []string, id string) []string {
	for _, existing := range ids {
		if existing == id {
			return ids
		}
	}
	return append(ids, id)
}

func cephPoolStorageSourceAliasID(id string) (string, bool) {
	const marker = "-ceph-pool-"
	idx := strings.Index(id, marker)
	if idx <= 0 {
		return "", false
	}
	instance := strings.TrimSpace(id[:idx])
	if instance == "" {
		return "", false
	}
	suffix := id[idx:]
	if strings.HasPrefix(instance, "agent:") {
		unprefixed := strings.TrimSpace(strings.TrimPrefix(instance, "agent:"))
		if unprefixed == "" {
			return "", false
		}
		return unprefixed + suffix, true
	}
	return "agent:" + instance + suffix, true
}

func (m *Manager) resolveStorageThresholdOverride(base ThresholdConfig, resourceID string, aliases []string) ThresholdConfig {
	for _, lookupID := range storageThresholdLookupIDs(resourceID, aliases...) {
		if override, exists := m.config.Overrides[lookupID]; exists {
			return m.applyThresholdOverride(base, override)
		}
	}
	return base
}

func (m *Manager) resolveStorageThresholdsNoLock(storage models.Storage) ThresholdConfig {
	return m.resolveStorageThresholdOverride(m.defaultThresholdsForResourceType("storage"), storage.ID, storage.AliasIDs)
}
