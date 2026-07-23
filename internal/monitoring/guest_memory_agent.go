package monitoring

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

const (
	vmAgentMemCacheTTL              = 60 * time.Second // Cache guest-agent /proc/meminfo reads.
	vmAgentMemRequestTTL            = 3 * time.Second  // Bound guest-agent file-read latency.
	vmAgentMemNegativeTTL           = 5 * time.Minute  // Back off on unsupported or failing guests.
	vmAgentMemNegativeKnownGuestTTL = 30 * time.Second // Retry sooner for guests we know are non-Windows.
	vmAgentMemCleanupMaxAge         = 2 * vmAgentMemNegativeTTL
)

type agentMemCacheEntry struct {
	available uint64
	info      proxmox.LinuxMemoryAvailability
	negative  bool
	fetchedAt time.Time
}

type guestAgentMemAvailableClient interface {
	GetVMMemAvailableFromAgent(ctx context.Context, node string, vmid int) (uint64, error)
}

type guestAgentMemoryAvailabilityClient interface {
	GetVMMemoryAvailabilityFromAgent(ctx context.Context, node string, vmid int) (proxmox.LinuxMemoryAvailability, error)
}

func guestMemoryCacheKey(instanceName, node string, vmid int) string {
	return fmt.Sprintf("%s/%s/%d", instanceName, node, vmid)
}

func (m *Monitor) getVMAgentMemAvailable(ctx context.Context, client PVEClientInterface, instanceName, node string, vmid int) (uint64, error) {
	info, err := m.getVMAgentMemoryAvailability(ctx, client, instanceName, node, vmid)
	return info.EffectiveAvailable, err
}

func (m *Monitor) getVMAgentMemoryAvailability(ctx context.Context, client PVEClientInterface, instanceName, node string, vmid int) (proxmox.LinuxMemoryAvailability, error) {
	if node == "" || vmid <= 0 {
		return proxmox.LinuxMemoryAvailability{}, fmt.Errorf("invalid arguments for guest agent meminfo lookup")
	}

	cacheKey := guestMemoryCacheKey(instanceName, node, vmid)
	now := time.Now()

	m.rrdCacheMu.RLock()
	if entry, ok := m.vmAgentMemCache[cacheKey]; ok {
		ttl := vmAgentMemCacheTTL
		if entry.negative {
			ttl = m.vmAgentMemNegativeCacheTTL(instanceName, node, vmid)
		}
		if now.Sub(entry.fetchedAt) < ttl {
			m.rrdCacheMu.RUnlock()
			if entry.negative {
				return proxmox.LinuxMemoryAvailability{}, fmt.Errorf("guest agent meminfo fallback unavailable")
			}
			if entry.info.Source != "" {
				return entry.info, nil
			}
			return proxmox.LinuxMemoryAvailability{
				Available:          entry.available,
				EffectiveAvailable: entry.available,
				Source:             "meminfo-available",
			}, nil
		}
	}
	m.rrdCacheMu.RUnlock()

	requestCtx, cancel := context.WithTimeout(ctx, vmAgentMemRequestTTL)
	defer cancel()

	var info proxmox.LinuxMemoryAvailability
	var err error
	if memClient, ok := client.(guestAgentMemoryAvailabilityClient); ok {
		info, err = memClient.GetVMMemoryAvailabilityFromAgent(requestCtx, node, vmid)
	} else if memClient, ok := client.(guestAgentMemAvailableClient); ok {
		var available uint64
		available, err = memClient.GetVMMemAvailableFromAgent(requestCtx, node, vmid)
		if available > 0 {
			info = proxmox.LinuxMemoryAvailability{
				Available:          available,
				EffectiveAvailable: available,
				Source:             "meminfo-available",
			}
		}
	} else {
		return proxmox.LinuxMemoryAvailability{}, fmt.Errorf("guest agent meminfo fallback unsupported")
	}

	m.rrdCacheMu.Lock()
	defer m.rrdCacheMu.Unlock()
	if m.vmAgentMemCache == nil {
		m.vmAgentMemCache = make(map[string]agentMemCacheEntry)
	}
	if err != nil || info.Source == "" {
		m.vmAgentMemCache[cacheKey] = agentMemCacheEntry{negative: true, fetchedAt: now}
		if err == nil {
			err = fmt.Errorf("guest agent meminfo fallback unavailable")
		}
		return proxmox.LinuxMemoryAvailability{}, err
	}

	m.vmAgentMemCache[cacheKey] = agentMemCacheEntry{
		available: info.EffectiveAvailable,
		info:      info,
		fetchedAt: now,
	}
	return info, nil
}

func (m *Monitor) vmAgentMemNegativeCacheTTL(instanceName, node string, vmid int) time.Duration {
	if m == nil {
		return vmAgentMemNegativeTTL
	}

	key := guestMetadataCacheKey(instanceName, node, vmid)
	m.guestMetadataMu.RLock()
	entry, ok := m.guestMetadataCache[key]
	m.guestMetadataMu.RUnlock()
	if !ok {
		return vmAgentMemNegativeTTL
	}

	osName := strings.ToLower(strings.TrimSpace(entry.osName))
	osVersion := strings.ToLower(strings.TrimSpace(entry.osVersion))
	if osName == "" && osVersion == "" {
		return vmAgentMemNegativeTTL
	}
	if strings.Contains(osName, "windows") || strings.Contains(osVersion, "windows") {
		return vmAgentMemNegativeTTL
	}
	return vmAgentMemNegativeKnownGuestTTL
}
