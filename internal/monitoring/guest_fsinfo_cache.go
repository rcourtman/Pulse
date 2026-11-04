package monitoring

import (
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// vmFSInfoCacheTTL sits just under a minute so 10-second polls read about every
// 60 seconds despite timer jitter, and polls of a minute or longer don't fall to
// every other poll unless the guest takes several seconds to answer.
const vmFSInfoCacheTTL = 55 * time.Second

// vmFSInfoCacheEntry holds a completed get-fsinfo reply. Failed and deferred
// reads are never stored, so they stay retried and retained as they were.
type vmFSInfoCacheEntry struct {
	filesystems []proxmox.VMFileSystem
	fetchedAt   time.Time
	uptime      uint64
}

// vmFSInfoRead says when the reply a poll used was read, and whether that was
// an earlier poll's read. The time stays with the read, so reuse cannot extend
// how long the reading may be retained or renew the evidence that lets Pulse
// query the guest without a status reply.
type vmFSInfoRead struct {
	observedAt time.Time
	reused     bool
}

// Falling uptime means the guest restarted between polls, which no stopped
// listing or agent status reported, so the reply belongs to an earlier boot. Each
// lookup records the uptime it saw: a read can fail after a restart, and a young
// reply must not become usable again as the new boot's uptime climbs past it.
func (m *Monitor) cachedVMFSInfo(instanceName, node string, vmid int, uptime uint64, now time.Time) (vmFSInfoCacheEntry, bool) {
	if m.guestAgentFSInfoCacheTTL <= 0 {
		return vmFSInfoCacheEntry{}, false
	}
	key := guestMemoryCacheKey(instanceName, node, vmid)
	m.rrdCacheMu.Lock()
	defer m.rrdCacheMu.Unlock()
	entry, ok := m.vmFSInfoCache[key]
	if !ok {
		return vmFSInfoCacheEntry{}, false
	}
	if uptime < entry.uptime {
		delete(m.vmFSInfoCache, key)
		return vmFSInfoCacheEntry{}, false
	}
	entry.uptime = uptime
	m.vmFSInfoCache[key] = entry
	if entry.fetchedAt.After(now) || now.Sub(entry.fetchedAt) >= m.guestAgentFSInfoCacheTTL {
		return vmFSInfoCacheEntry{}, false
	}
	return entry, true
}

func (m *Monitor) storeVMFSInfo(instanceName, node string, vmid int, uptime uint64, filesystems []proxmox.VMFileSystem, fetchedAt time.Time) {
	if m.guestAgentFSInfoCacheTTL <= 0 {
		return
	}
	m.rrdCacheMu.Lock()
	defer m.rrdCacheMu.Unlock()
	if m.vmFSInfoCache == nil {
		m.vmFSInfoCache = make(map[string]vmFSInfoCacheEntry)
	}
	m.vmFSInfoCache[guestMemoryCacheKey(instanceName, node, vmid)] = vmFSInfoCacheEntry{filesystems: filesystems, fetchedAt: fetchedAt, uptime: uptime}
}

func (m *Monitor) clearVMFSInfoCache(instanceName, node string, vmid int) {
	m.rrdCacheMu.Lock()
	defer m.rrdCacheMu.Unlock()
	delete(m.vmFSInfoCache, guestMemoryCacheKey(instanceName, node, vmid))
}
