package monitoring

import (
	"context"
	"strconv"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/rs/zerolog/log"
)

const nodeMemoryCarryForwardMaxAge = 2 * time.Minute

func (m *Monitor) canCarryForwardNodeMemory(instance, node string, now time.Time) bool {
	if m == nil {
		return false
	}
	m.diagMu.RLock()
	snapshot, ok := m.nodeSnapshots[makeNodeSnapshotKey(instance, node)]
	m.diagMu.RUnlock()
	if !ok || !snapshot.Memory.HasKnownUsage() || snapshot.RetrievedAt.IsZero() {
		return false
	}
	age := now.Sub(snapshot.RetrievedAt)
	if age < 0 || age > nodeMemoryCarryForwardMaxAge {
		return false
	}
	switch CanonicalMemorySource(snapshot.MemorySource) {
	case "available-field", "derived-free-buffers-cached", "derived-total-minus-used",
		"rrd-memavailable", "rrd-memused":
		return true
	default:
		return false
	}
}

func (m *Monitor) pollPVENode(
	ctx context.Context,
	instanceName string,
	instanceCfg *config.PVEInstance,
	client PVEClientInterface,
	node proxmox.Node,
	connectionHealthStr string,
	prevNodeMemory map[string]models.Memory,
	prevInstanceNodes []models.Node,
) (models.Node, string, string, error) {
	nodeStart := time.Now()
	displayName := getNodeDisplayName(instanceCfg, node.Node)
	connectionHost, guestURL := resolveNodeConnectionInfo(instanceCfg, monitorDiscoveryConfig(m), node.Node)
	nodeID, effectiveStatus := m.determineNodeIDAndStatus(instanceName, instanceCfg, node)
	nodeFallbackFree := uint64(0)
	if node.MaxMem >= node.Mem {
		nodeFallbackFree = node.MaxMem - node.Mem
	}

	modelNode := models.Node{
		ID:                           nodeID,
		Name:                         node.Node,
		DisplayName:                  displayName,
		Instance:                     instanceName,
		Host:                         connectionHost,
		GuestURL:                     guestURL,
		Status:                       effectiveStatus,
		Type:                         "node",
		CPU:                          safeFloat(node.CPU), // Proxmox returns 0-1 ratio (e.g., 0.15 = 15%)
		Memory:                       models.UnavailableMemory(clampToInt64(node.MaxMem)),
		Uptime:                       int64(node.Uptime),
		LoadAverage:                  []float64{},
		LastSeen:                     time.Now(),
		ConnectionHealth:             connectionHealthStr, // Use the determined health status
		IsClusterMember:              instanceCfg.IsCluster,
		ClusterName:                  instanceCfg.ClusterName,
		TemperatureMonitoringEnabled: instanceCfg.TemperatureMonitoringEnabled,
	}
	var nodeDiskSource string
	modelNode.Disk, nodeDiskSource = m.resolveNodeDisk(instanceName, nodeID, node.Node, node, nil)

	nodeSnapshotRaw := NodeMemoryRaw{
		Total:               node.MaxMem,
		Used:                node.Mem,
		Free:                nodeFallbackFree,
		FallbackTotal:       node.MaxMem,
		FallbackUsed:        node.Mem,
		FallbackFree:        nodeFallbackFree,
		FallbackCalculated:  true,
		ProxmoxMemorySource: "nodes-endpoint",
	}
	nodeMemorySource := "unavailable"
	nodeFallbackReason := "cache-aware-memory-unavailable"

	// Debug logging for disk metrics - note that these values can fluctuate
	// due to thin provisioning and dynamic allocation
	if node.Disk > 0 && node.MaxDisk > 0 {
		log.Debug().
			Str("node", node.Node).
			Uint64("disk", node.Disk).
			Uint64("maxDisk", node.MaxDisk).
			Float64("diskUsage", safePercentage(float64(node.Disk), float64(node.MaxDisk))).
			Msg("Node disk metrics from /nodes endpoint")
	}

	// Track whether we successfully replaced memory metrics with detailed status data
	memoryUpdated := false

	// Get detailed node info if available (skip for offline nodes)
	if effectiveStatus == "online" {
		nodeInfo, nodeErr := client.GetNodeStatus(ctx, node.Node)
		if nodeErr != nil {
			nodeFallbackReason = "node-status-unavailable"
			// If we can't get node status, log but continue with data from /nodes endpoint
			if node.Disk > 0 && node.MaxDisk > 0 {
				log.Warn().
					Str("instance", instanceName).
					Str("node", node.Node).
					Err(nodeErr).
					Uint64("usingDisk", node.Disk).
					Uint64("usingMaxDisk", node.MaxDisk).
					Msg("Could not get node status - disk fallback retained; memory usage unavailable")
			} else {
				log.Warn().
					Str("instance", instanceName).
					Str("node", node.Node).
					Err(nodeErr).
					Uint64("disk", node.Disk).
					Uint64("maxDisk", node.MaxDisk).
					Msg("Could not get node status - memory usage unavailable")
			}
		} else if nodeInfo != nil {
			if nodeInfo.Memory != nil {
				nodeSnapshotRaw.Total = nodeInfo.Memory.Total
				nodeSnapshotRaw.Used = nodeInfo.Memory.Used
				nodeSnapshotRaw.Free = nodeInfo.Memory.Free
				nodeSnapshotRaw.Available = nodeInfo.Memory.Available
				nodeSnapshotRaw.Avail = nodeInfo.Memory.Avail
				nodeSnapshotRaw.Buffers = nodeInfo.Memory.Buffers
				nodeSnapshotRaw.Cached = nodeInfo.Memory.Cached
				nodeSnapshotRaw.Shared = nodeInfo.Memory.Shared
				nodeSnapshotRaw.EffectiveAvailable = nodeInfo.Memory.EffectiveAvailable()
				nodeSnapshotRaw.ProxmoxMemorySource = "node-status"
				nodeSnapshotRaw.FallbackCalculated = false
			}

			// Convert LoadAvg from interface{} to float64
			loadAvg := make([]float64, 0, len(nodeInfo.LoadAvg))
			for _, val := range nodeInfo.LoadAvg {
				switch v := val.(type) {
				case float64:
					loadAvg = append(loadAvg, v)
				case string:
					if f, err := strconv.ParseFloat(v, 64); err == nil {
						loadAvg = append(loadAvg, f)
					}
				}
			}
			modelNode.LoadAverage = loadAvg
			modelNode.KernelVersion = nodeInfo.KernelVersion
			modelNode.PVEVersion = nodeInfo.PVEVersion

			if resolvedDisk, diskSource := m.resolveNodeDisk(instanceName, nodeID, node.Node, node, nodeInfo); diskSource != "" {
				modelNode.Disk = resolvedDisk
				nodeDiskSource = diskSource
			} else {
				log.Warn().
					Str("node", node.Node).
					Bool("rootfsNil", nodeInfo.RootFS == nil).
					Uint64("nodeDisk", node.Disk).
					Uint64("nodeMaxDisk", node.MaxDisk).
					Msg("No valid disk metrics available for node")
			}

			// Update memory metrics to use Available field for more accurate usage
			if nodeInfo.Memory != nil && nodeInfo.Memory.Total > 0 {
				resolvedMemory, resolvedSource, resolvedFallback, resolvedRaw, ok := m.resolveNodeMemory(
					ctx,
					client,
					instanceName,
					node.Node,
					nodeInfo.Memory,
					nodeSnapshotRaw,
				)
				if ok {
					modelNode.Memory = resolvedMemory
					nodeMemorySource = resolvedSource
					nodeFallbackReason = resolvedFallback
					nodeSnapshotRaw = resolvedRaw
					memoryUpdated = resolvedMemory.HasKnownUsage()
				}
			}

			if nodeInfo.CPUInfo != nil {
				// Use MaxCPU from node data for logical CPU count (includes hyperthreading)
				// If MaxCPU is not available or 0, fall back to physical cores
				logicalCores := node.MaxCPU
				if logicalCores == 0 {
					logicalCores = nodeInfo.CPUInfo.Cores
				}

				mhzStr := nodeInfo.CPUInfo.GetMHzString()
				log.Debug().
					Str("node", node.Node).
					Str("model", nodeInfo.CPUInfo.Model).
					Int("cores", nodeInfo.CPUInfo.Cores).
					Int("logicalCores", logicalCores).
					Int("sockets", nodeInfo.CPUInfo.Sockets).
					Str("mhz", mhzStr).
					Msg("Node CPU info from Proxmox")
				modelNode.CPUInfo = models.CPUInfo{
					Model:   nodeInfo.CPUInfo.Model,
					Cores:   logicalCores, // Use logical cores for display
					Sockets: nodeInfo.CPUInfo.Sockets,
					MHz:     mhzStr,
				}
			}
		}
	}

	// If we couldn't update memory metrics using detailed status, preserve previous accurate values if available
	if !memoryUpdated &&
		effectiveStatus == "online" &&
		m.canCarryForwardNodeMemory(instanceName, node.Node, time.Now()) {
		if prevMem, exists := prevNodeMemory[modelNode.ID]; exists && prevMem.HasKnownUsage() {
			total := int64(node.MaxMem)
			if total == 0 {
				total = prevMem.Total
			}
			used := prevMem.Used
			if total > 0 && used > total {
				used = total
			}
			free := total - used
			if free < 0 {
				free = 0
			}

			preserved := prevMem
			preserved.Total = total
			preserved.Used = used
			preserved.Free = free
			preserved.Usage = safePercentage(float64(used), float64(total))

			modelNode.Memory = preserved
			log.Debug().
				Str("instance", instanceName).
				Str("node", node.Node).
				Msg("Preserving previous memory metrics - node status unavailable this cycle")

			nodeFallbackReason = "preserved-previous-snapshot"
			nodeMemorySource = "previous-snapshot"
			if nodeSnapshotRaw.ProxmoxMemorySource == "node-status" && nodeSnapshotRaw.Total == 0 {
				nodeSnapshotRaw.ProxmoxMemorySource = "previous-snapshot"
			}
		}
	}

	m.recordNodeSnapshot(instanceName, node.Node, NodeMemorySnapshot{
		RetrievedAt:    time.Now(),
		MemorySource:   nodeMemorySource,
		FallbackReason: nodeFallbackReason,
		Memory:         modelNode.Memory,
		Raw:            nodeSnapshotRaw,
	})

	m.collectNodeTemperatureData(ctx, instanceName, instanceCfg, node, &modelNode, prevInstanceNodes, effectiveStatus)
	m.applyNodePendingUpdates(ctx, instanceName, client, node, nodeID, effectiveStatus, &modelNode)
	m.recordNodePollMetrics(instanceName, node, &modelNode, nodeStart)

	return modelNode, effectiveStatus, nodeDiskSource, nil
}
