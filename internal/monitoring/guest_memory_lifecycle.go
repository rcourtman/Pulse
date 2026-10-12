package monitoring

import (
	"context"
	"strings"
)

// Cancellation is not completion. The obsolete owner remains in the map until
// its call returns; neither a janitor nor a new lifecycle releases its fence.
// Guarded by rrdCacheMu, including publication in getVMAgentMemoryAvailability.
type guestMemoryFetch struct {
	cancel      context.CancelFunc
	invalidated bool
}

func (m *Monitor) invalidateGuestMemoryFetchLocked(key string) {
	if fetch := m.vmAgentMemInFlight[key]; fetch != nil {
		fetch.invalidated = true
		fetch.cancel()
	}
}

func (m *Monitor) clearGuestMemoryCache(instanceName, node string, vmid int) {
	if m == nil {
		return
	}
	key := guestMemoryCacheKey(instanceName, node, vmid)
	m.rrdCacheMu.Lock()
	m.invalidateGuestMemoryFetchLocked(key)
	delete(m.vmAgentMemCache, key)
	m.rrdCacheMu.Unlock()
}

// PVE node names do not contain '/'. Split the last two components, not an
// instance prefix: a connection named "site/other" is not owned by "site".
func guestMemoryCacheBelongsToInstance(key, instanceName string) bool {
	end := strings.LastIndexByte(key, '/')
	if end < 0 {
		return false
	}
	end = strings.LastIndexByte(key[:end], '/')
	return end >= 0 && key[:end] == instanceName
}

func (m *Monitor) retireGuestMemoryCache(instanceName string) {
	m.rrdCacheMu.Lock()
	defer m.rrdCacheMu.Unlock()
	for key := range m.vmAgentMemInFlight {
		if guestMemoryCacheBelongsToInstance(key, instanceName) {
			m.invalidateGuestMemoryFetchLocked(key)
		}
	}
	for key := range m.vmAgentMemCache {
		if guestMemoryCacheBelongsToInstance(key, instanceName) {
			delete(m.vmAgentMemCache, key)
		}
	}
}
