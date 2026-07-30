package monitoring

import (
	"path"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

const (
	agentLXCFilesystemMinTTL        = 3 * time.Minute
	agentLXCFilesystemMaxTTL        = 15 * time.Minute
	agentLXCFilesystemMaxContainers = 128
	agentLXCFilesystemMaxDisks      = 257
	agentLXCFilesystemMaxNameBytes  = 256
	agentLXCFilesystemMaxLabelBytes = 512
)

type agentLXCFilesystemCacheEntry struct {
	agentID   string
	name      string
	disks     []models.Disk
	expiresAt time.Time
}

func agentLXCFilesystemCacheKey(instance, node string, vmid int) string {
	return strings.ToLower(strings.TrimSpace(instance)) + "\x00" +
		strings.ToLower(strings.TrimSpace(node)) + "\x00" +
		strconv.Itoa(vmid)
}

func agentLXCFilesystemTTL(intervalSeconds int) time.Duration {
	ttl := time.Duration(intervalSeconds) * 3 * time.Second
	if ttl < agentLXCFilesystemMinTTL {
		return agentLXCFilesystemMinTTL
	}
	if ttl > agentLXCFilesystemMaxTTL {
		return agentLXCFilesystemMaxTTL
	}
	return ttl
}

// applyAgentLXCFilesystems admits node-local pct df data only when the
// reporting host agent is already linked to exactly one current Proxmox node.
// Receipt time, rather than the agent clock, controls freshness.
func (m *Monitor) applyAgentLXCFilesystems(
	linkedNodeID string,
	agentID string,
	inventory *agentshost.ProxmoxLXCInventory,
	receivedAt time.Time,
	intervalSeconds int,
) {
	agentID = strings.TrimSpace(agentID)
	if m == nil || m.state == nil || inventory == nil ||
		strings.TrimSpace(linkedNodeID) == "" || agentID == "" {
		return
	}

	var linkedNodes []models.Node
	for _, node := range m.state.GetSnapshot().Nodes {
		if node.ID == linkedNodeID {
			linkedNodes = append(linkedNodes, node)
		}
	}
	if len(linkedNodes) != 1 {
		return
	}
	node := linkedNodes[0]
	if strings.TrimSpace(node.Instance) == "" || strings.TrimSpace(node.Name) == "" {
		return
	}

	if receivedAt.IsZero() {
		receivedAt = time.Now()
	}
	expiresAt := receivedAt.Add(agentLXCFilesystemTTL(intervalSeconds))
	limit := len(inventory.Containers)
	if limit > agentLXCFilesystemMaxContainers {
		limit = agentLXCFilesystemMaxContainers
	}

	updates := make(map[string]agentLXCFilesystemCacheEntry, limit)
	for _, container := range inventory.Containers[:limit] {
		name := strings.TrimSpace(container.Name)
		if container.VMID < 100 || container.VMID > 999999999 ||
			!safeAgentLXCFilesystemText(name, agentLXCFilesystemMaxNameBytes) {
			continue
		}
		disks := normalizeAgentLXCFilesystems(container.Disks)
		if len(disks) == 0 {
			continue
		}
		updates[agentLXCFilesystemCacheKey(node.Instance, node.Name, container.VMID)] =
			agentLXCFilesystemCacheEntry{
				agentID:   agentID,
				name:      name,
				disks:     disks,
				expiresAt: expiresAt,
			}
	}
	m.proxmoxLXCFilesystemsMu.Lock()
	defer m.proxmoxLXCFilesystemsMu.Unlock()
	if m.proxmoxLXCFilesystemsCache == nil {
		m.proxmoxLXCFilesystemsCache = make(map[string]agentLXCFilesystemCacheEntry)
	}
	for key, entry := range m.proxmoxLXCFilesystemsCache {
		if !receivedAt.Before(entry.expiresAt) {
			delete(m.proxmoxLXCFilesystemsCache, key)
		}
	}
	for key, entry := range updates {
		m.proxmoxLXCFilesystemsCache[key] = entry
	}
}

func normalizeAgentLXCFilesystems(disks []agentshost.Disk) []models.Disk {
	limit := len(disks)
	if limit > agentLXCFilesystemMaxDisks {
		limit = agentLXCFilesystemMaxDisks
	}

	result := make([]models.Disk, 0, limit)
	seenMountpoints := make(map[string]struct{}, limit)
	for _, disk := range disks[:limit] {
		diskType := strings.ToLower(strings.TrimSpace(disk.Type))
		mountpoint := strings.TrimSpace(disk.Mountpoint)
		device := strings.TrimSpace(disk.Device)
		if diskType != "rootfs" && !validAgentLXCMountKey(diskType) {
			continue
		}
		if !validAgentLXCMountpoint(mountpoint) ||
			!safeAgentLXCFilesystemText(device, agentLXCFilesystemMaxLabelBytes) ||
			disk.TotalBytes <= 0 ||
			disk.UsedBytes < 0 ||
			disk.FreeBytes < 0 {
			continue
		}
		mountpointKey := strings.ToLower(mountpoint)
		if _, exists := seenMountpoints[mountpointKey]; exists {
			continue
		}
		seenMountpoints[mountpointKey] = struct{}{}

		used := disk.UsedBytes
		if used > disk.TotalBytes {
			used = disk.TotalBytes
		}
		free := disk.FreeBytes
		if free > disk.TotalBytes {
			free = disk.TotalBytes
		}
		result = append(result, models.Disk{
			Total:      disk.TotalBytes,
			Used:       used,
			Free:       free,
			Usage:      safePercentage(float64(used), float64(disk.TotalBytes)),
			Mountpoint: mountpoint,
			Type:       diskType,
			Device:     device,
		})
	}

	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Mountpoint == "/" {
			return true
		}
		if result[j].Mountpoint == "/" {
			return false
		}
		return result[i].Mountpoint < result[j].Mountpoint
	})
	return result
}

func (m *Monitor) enrichContainerWithAgentLXCFilesystems(
	instance string,
	node string,
	container *models.Container,
	now time.Time,
) {
	if m == nil || container == nil || !strings.EqualFold(strings.TrimSpace(container.Status), "running") {
		return
	}
	key := agentLXCFilesystemCacheKey(instance, node, container.VMID)

	m.proxmoxLXCFilesystemsMu.RLock()
	entry, exists := m.proxmoxLXCFilesystemsCache[key]
	m.proxmoxLXCFilesystemsMu.RUnlock()
	if !exists {
		return
	}
	if now.IsZero() {
		now = time.Now()
	}
	if !now.Before(entry.expiresAt) {
		m.proxmoxLXCFilesystemsMu.Lock()
		if current, ok := m.proxmoxLXCFilesystemsCache[key]; ok && !now.Before(current.expiresAt) {
			delete(m.proxmoxLXCFilesystemsCache, key)
		}
		m.proxmoxLXCFilesystemsMu.Unlock()
		return
	}
	if strings.TrimSpace(container.Name) != entry.name {
		return
	}

	container.Disks = append([]models.Disk(nil), entry.disks...)
	for _, disk := range container.Disks {
		if disk.Mountpoint == "/" {
			container.Disk = disk
			break
		}
	}
}

func (m *Monitor) clearAgentLXCFilesystems(agentID string) {
	if m == nil || strings.TrimSpace(agentID) == "" {
		return
	}
	m.proxmoxLXCFilesystemsMu.Lock()
	defer m.proxmoxLXCFilesystemsMu.Unlock()
	for key, entry := range m.proxmoxLXCFilesystemsCache {
		if entry.agentID == agentID {
			delete(m.proxmoxLXCFilesystemsCache, key)
		}
	}
}

func validAgentLXCMountKey(value string) bool {
	if !strings.HasPrefix(value, "mp") || len(value) < 3 || len(value) > 5 {
		return false
	}
	index, err := strconv.Atoi(value[2:])
	return err == nil && index >= 0 && index <= 255
}

func validAgentLXCMountpoint(value string) bool {
	return safeAgentLXCFilesystemText(value, agentLXCFilesystemMaxLabelBytes) &&
		strings.HasPrefix(value, "/") &&
		path.Clean(value) == value
}

func safeAgentLXCFilesystemText(value string, maxBytes int) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > maxBytes || !utf8.ValidString(value) {
		return false
	}
	for _, char := range value {
		if char < 0x20 || char == 0x7f {
			return false
		}
	}
	return true
}
