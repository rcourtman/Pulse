package monitoring

import (
	"context"
	"fmt"
	"time"
)

const (
	vmAgentMemCacheTTL      = 60 * time.Second // Cache guest-agent /proc/meminfo reads.
	vmAgentMemRequestTTL    = 3 * time.Second  // Bound guest-agent file-read latency.
	vmAgentMemNegativeTTL   = 5 * time.Minute  // Back off on unsupported or failing guests.
	vmAgentMemCleanupMaxAge = 2 * vmAgentMemNegativeTTL
)

type agentMemCacheEntry struct {
	available uint64
	negative  bool
	fetchedAt time.Time
}

type guestAgentMemAvailableClient interface {
	GetVMMemAvailableFromAgent(ctx context.Context, node string, vmid int) (uint64, error)
}

func guestMemoryCacheKey(instanceName, node string, vmid int) string {
	return fmt.Sprintf("%s/%s/%d", instanceName, node, vmid)
}

func (m *Monitor) getVMAgentMemAvailable(ctx context.Context, client PVEClientInterface, instanceName, node string, vmid int) (uint64, error) {
	memClient, ok := client.(guestAgentMemAvailableClient)
	if !ok {
		return 0, fmt.Errorf("guest agent meminfo fallback unsupported")
	}
	if node == "" || vmid <= 0 {
		return 0, fmt.Errorf("invalid arguments for guest agent meminfo lookup")
	}

	cacheKey := guestMemoryCacheKey(instanceName, node, vmid)
	now := time.Now()

	m.rrdCacheMu.RLock()
	if entry, ok := m.vmAgentMemCache[cacheKey]; ok {
		ttl := vmAgentMemCacheTTL
		if entry.negative {
			ttl = vmAgentMemNegativeTTL
		}
		if now.Sub(entry.fetchedAt) < ttl {
			m.rrdCacheMu.RUnlock()
			if entry.negative {
				return 0, fmt.Errorf("guest agent meminfo fallback unavailable")
			}
			return entry.available, nil
		}
	}
	m.rrdCacheMu.RUnlock()

	requestCtx, cancel := context.WithTimeout(ctx, vmAgentMemRequestTTL)
	defer cancel()

	available, err := memClient.GetVMMemAvailableFromAgent(requestCtx, node, vmid)
	m.rrdCacheMu.Lock()
	defer m.rrdCacheMu.Unlock()
	if m.vmAgentMemCache == nil {
		m.vmAgentMemCache = make(map[string]agentMemCacheEntry)
	}
	if err != nil || available == 0 {
		m.vmAgentMemCache[cacheKey] = agentMemCacheEntry{negative: true, fetchedAt: now}
		if err == nil {
			err = fmt.Errorf("guest agent meminfo fallback unavailable")
		}
		return 0, err
	}

	m.vmAgentMemCache[cacheKey] = agentMemCacheEntry{available: available, fetchedAt: now}
	return available, nil
}
