package monitoring

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
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
	available        uint64
	info             proxmox.LinuxMemoryAvailability
	negative         bool
	permissionDenied bool // Only an observed HTTP 403 overrides known-Linux retry cadence.
	fetchedAt        time.Time
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

// deferredVMGuestMemory retains a rendered observation only while the original
// successful guest read still supports it. The ordinary read TTL schedules a
// refresh; the cleanup age bounds last-known evidence during coordination.
// Neither a poll timestamp nor a previous-snapshot trust label renews that age.
func (m *Monitor) deferredVMGuestMemory(instanceName, node string, vmid int, total uint64, previous *GuestMemorySnapshot, now time.Time) (models.Memory, bool) {
	if previous == nil || previous.GuestType != "qemu" || previous.Status != "running" || previous.Instance != instanceName || previous.Node != node || previous.VMID != vmid || !previous.Memory.HasKnownUsage() || previous.Memory.Total <= 0 {
		return models.Memory{}, false
	}
	// The last guest sample can be smaller than its configured maximum. Keep
	// the tuple intact, but still reject retention across a capacity change.
	if guestMemoryConfiguredCapacity(previous) != total {
		return models.Memory{}, false
	}
	switch CanonicalMemorySource(previous.MemorySource) {
	case "guest-agent-meminfo", "guest-agent-meminfo-derived", "previous-snapshot":
	default:
		return models.Memory{}, false
	}
	m.rrdCacheMu.RLock()
	entry, ok := m.vmAgentMemCache[guestMemoryCacheKey(instanceName, node, vmid)]
	m.rrdCacheMu.RUnlock()
	if !ok || entry.negative || entry.fetchedAt.IsZero() || entry.fetchedAt.After(now) || now.Sub(entry.fetchedAt) > vmAgentMemCleanupMaxAge {
		return models.Memory{}, false
	}
	if entry.info.Source != "meminfo-available" && entry.info.Source != "meminfo-derived" {
		return models.Memory{}, false
	}
	sampleTotal := uint64(previous.Memory.Total)
	if (entry.info.Total > 0 && entry.info.Total != sampleTotal) || entry.info.EffectiveAvailable > sampleTotal || previous.Memory.Used != int64(sampleTotal-entry.info.EffectiveAvailable) {
		return models.Memory{}, false
	}
	expected := models.Memory{Free: int64(entry.info.EffectiveAvailable)}
	splitReclaimableMemory(&expected, entry.info.Free)
	if previous.Memory.Free != expected.Free || previous.Memory.Cache != expected.Cache {
		return models.Memory{}, false
	}
	return previous.Memory, true
}

func (m *Monitor) getVMAgentMemAvailable(ctx context.Context, client PVEClientInterface, instanceName, node string, vmid int) (uint64, error) {
	info, err := m.getVMAgentMemoryAvailability(ctx, client, instanceName, node, vmid)
	return info.EffectiveAvailable, err
}

func (m *Monitor) getVMAgentMemoryAvailability(ctx context.Context, client PVEClientInterface, instanceName, node string, vmid int) (proxmox.LinuxMemoryAvailability, error) {
	if node == "" || vmid <= 0 {
		return proxmox.LinuxMemoryAvailability{}, fmt.Errorf("invalid arguments for guest agent meminfo lookup")
	}
	if ctx.Err() != nil {
		return proxmox.LinuxMemoryAvailability{}, proxmox.ErrGuestAgentDeferred
	}

	cacheKey := guestMemoryCacheKey(instanceName, node, vmid)
	now := time.Now()
	if m.guestWindowsMeminfoUnsupported(instanceName, node, vmid, now) {
		// /proc/meminfo is Linux-specific. Do not knowingly queue a Windows
		// file-read that can pause the shared QGA channel on an uncertain reply.
		// This is neither a failed observation nor a coordination deferral;
		// independent status/balloon/linked-agent memory remains usable.
		return proxmox.LinuxMemoryAvailability{}, fmt.Errorf("guest agent Linux meminfo unavailable for Windows")
	}

	// Cache admission and read ownership share the invalidation lock. A miss
	// never grants permission to overlap another read, even after cancellation.
	m.rrdCacheMu.Lock()
	if m.vmAgentMemInFlight[cacheKey] != nil || ctx.Err() != nil {
		m.rrdCacheMu.Unlock()
		return proxmox.LinuxMemoryAvailability{}, proxmox.ErrGuestAgentDeferred
	}
	if entry, ok := m.vmAgentMemCache[cacheKey]; ok {
		ttl := vmAgentMemCacheTTL
		if entry.negative {
			ttl = vmAgentMemNegativeTTL
			if !entry.permissionDenied {
				ttl = m.vmAgentMemNegativeCacheTTL(instanceName, node, vmid)
			}
		}
		if now.Sub(entry.fetchedAt) < ttl {
			m.rrdCacheMu.Unlock()
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
	requestCtx, cancel := context.WithTimeout(ctx, vmAgentMemRequestTTL)
	fetch := &guestMemoryFetch{cancel: cancel}
	if m.vmAgentMemInFlight == nil {
		m.vmAgentMemInFlight = make(map[string]*guestMemoryFetch)
	}
	m.vmAgentMemInFlight[cacheKey] = fetch
	m.rrdCacheMu.Unlock()
	defer func() {
		cancel()
		m.rrdCacheMu.Lock()
		delete(m.vmAgentMemInFlight, cacheKey)
		m.rrdCacheMu.Unlock()
	}()
	if requestCtx.Err() != nil {
		return proxmox.LinuxMemoryAvailability{}, proxmox.ErrGuestAgentDeferred
	}

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
	if fetch.invalidated || requestCtx.Err() != nil || errors.Is(err, proxmox.ErrGuestAgentDeferred) {
		// A coordination pause is not a failed guest observation. Preserve the
		// last successful cache and its original timestamp for normal resumption.
		// Cancellation cannot establish completion of a consumed guest command,
		// or let its old reply/backoff repopulate a replacement lifecycle.
		if errors.Is(err, proxmox.ErrGuestAgentDeferred) && !fetch.invalidated && requestCtx.Err() == nil {
			return proxmox.LinuxMemoryAvailability{}, err
		}
		return proxmox.LinuxMemoryAvailability{}, proxmox.ErrGuestAgentDeferred
	}

	if m.vmAgentMemCache == nil {
		m.vmAgentMemCache = make(map[string]agentMemCacheEntry)
	}
	if err != nil || info.Source == "" {
		status, known := proxmox.APIErrorStatus(err)
		m.vmAgentMemCache[cacheKey] = agentMemCacheEntry{
			negative:         true,
			permissionDenied: known && status == http.StatusForbidden,
			fetchedAt:        now,
		}
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

func guestOSIsWindows(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	return name == "mswindows" || name == "windows" || name == "microsoft windows" ||
		strings.HasPrefix(name, "windows ") || strings.HasPrefix(name, "microsoft windows ")
}

func (m *Monitor) guestWindowsMeminfoUnsupported(instanceName, node string, vmid int, now time.Time) bool {
	key := guestMetadataCacheKey(instanceName, node, vmid)
	m.guestMetadataMu.RLock()
	entry := m.guestMetadataCache[key]
	m.guestMetadataMu.RUnlock()
	// Use the existing ten-minute guest evidence age, not the metadata cache's
	// last useful network/version receipt. Missing/future origins and copied
	// display strings cannot establish the current guest's OS.
	return entry.windowsGuest && !entry.osInfoObservedAt.IsZero() && !entry.osInfoObservedAt.After(now) &&
		now.Sub(entry.osInfoObservedAt) < vmAgentMemCleanupMaxAge
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
