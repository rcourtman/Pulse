package monitoring

import (
	"context"
	"math"
	"strconv"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/rs/zerolog/log"
)

func (m *Monitor) pollPVENode(
	ctx context.Context,
	instanceName string,
	instanceCfg *config.PVEInstance,
	client PVEClientInterface,
	node proxmox.Node,
	connectionHealthStr string,
	prevNodeMemory map[string]models.Memory,
	prevInstanceNodes []models.Node,
) (models.Node, string, error) {
	nodeStart := time.Now()
	displayName := getNodeDisplayName(instanceCfg, node.Node)
	connectionHost, guestURL := resolveNodeConnectionInfo(instanceCfg, node.Node)
	nodeID, effectiveStatus := m.determineNodeIDAndStatus(instanceName, instanceCfg, node)

	modelNode := models.Node{
		ID:          nodeID,
		Name:        node.Node,
		DisplayName: displayName,
		Instance:    instanceName,
		Host:        connectionHost,
		GuestURL:    guestURL,
		Status:      effectiveStatus,
		Type:        "node",
		CPU:         safeFloat(node.CPU), // Proxmox returns 0-1 ratio (e.g., 0.15 = 15%)
		Memory: models.Memory{
			Total: int64(node.MaxMem),
			Used:  int64(node.Mem),
			Free:  int64(node.MaxMem - node.Mem),
			Usage: safePercentage(float64(node.Mem), float64(node.MaxMem)),
		},
		Disk: models.Disk{
			Total: int64(node.MaxDisk),
			Used:  int64(node.Disk),
			Free:  int64(node.MaxDisk - node.Disk),
			Usage: safePercentage(float64(node.Disk), float64(node.MaxDisk)),
		},
		Uptime:                       int64(node.Uptime),
		LoadAverage:                  []float64{},
		LastSeen:                     time.Now(),
		ConnectionHealth:             connectionHealthStr, // Use the determined health status
		IsClusterMember:              instanceCfg.IsCluster,
		ClusterName:                  instanceCfg.ClusterName,
		TemperatureMonitoringEnabled: instanceCfg.TemperatureMonitoringEnabled,
	}

	nodeSnapshotRaw := NodeMemoryRaw{
		Total:               node.MaxMem,
		Used:                node.Mem,
		Free:                node.MaxMem - node.Mem,
		FallbackTotal:       node.MaxMem,
		FallbackUsed:        node.Mem,
		FallbackFree:        node.MaxMem - node.Mem,
		FallbackCalculated:  true,
		ProxmoxMemorySource: "nodes-endpoint",
	}
	nodeMemorySource := "nodes-endpoint"
	var nodeFallbackReason string

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
					Msg("Could not get node status - using fallback metrics (memory will include cache/buffers)")
			} else {
				log.Warn().
					Str("instance", instanceName).
					Str("node", node.Node).
					Err(nodeErr).
					Uint64("disk", node.Disk).
					Uint64("maxDisk", node.MaxDisk).
					Msg("Could not get node status - no fallback metrics available (memory will include cache/buffers)")
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

			// Prefer rootfs data for more accurate disk metrics, but ensure we have valid fallback
			if nodeInfo.RootFS != nil && nodeInfo.RootFS.Total > 0 {
				modelNode.Disk = models.Disk{
					Total: int64(nodeInfo.RootFS.Total),
					Used:  int64(nodeInfo.RootFS.Used),
					Free:  int64(nodeInfo.RootFS.Free),
					Usage: safePercentage(float64(nodeInfo.RootFS.Used), float64(nodeInfo.RootFS.Total)),
				}
				log.Debug().
					Str("node", node.Node).
					Uint64("rootfsUsed", nodeInfo.RootFS.Used).
					Uint64("rootfsTotal", nodeInfo.RootFS.Total).
					Float64("rootfsUsage", modelNode.Disk.Usage).
					Msg("Using rootfs for disk metrics")
			} else if node.Disk > 0 && node.MaxDisk > 0 {
				// RootFS unavailable but we have valid disk data from /nodes endpoint
				// Keep the values we already set from the nodes list
				log.Debug().
					Str("node", node.Node).
					Bool("rootfsNil", nodeInfo.RootFS == nil).
					Uint64("fallbackDisk", node.Disk).
					Uint64("fallbackMaxDisk", node.MaxDisk).
					Msg("RootFS data unavailable - using /nodes endpoint disk metrics")
			} else {
				// Neither rootfs nor valid node disk data available
				log.Warn().
					Str("node", node.Node).
					Bool("rootfsNil", nodeInfo.RootFS == nil).
					Uint64("nodeDisk", node.Disk).
					Uint64("nodeMaxDisk", node.MaxDisk).
					Msg("No valid disk metrics available for node")
			}

			// Update memory metrics to use Available field for more accurate usage
			if nodeInfo.Memory != nil && nodeInfo.Memory.Total > 0 {
				var actualUsed uint64
				effectiveAvailable := nodeInfo.Memory.EffectiveAvailable()
				componentAvailable := nodeInfo.Memory.Free
				if nodeInfo.Memory.Buffers > 0 {
					if math.MaxUint64-componentAvailable < nodeInfo.Memory.Buffers {
						componentAvailable = math.MaxUint64
					} else {
						componentAvailable += nodeInfo.Memory.Buffers
					}
				}
				if nodeInfo.Memory.Cached > 0 {
					if math.MaxUint64-componentAvailable < nodeInfo.Memory.Cached {
						componentAvailable = math.MaxUint64
					} else {
						componentAvailable += nodeInfo.Memory.Cached
					}
				}
				if nodeInfo.Memory.Total > 0 && componentAvailable > nodeInfo.Memory.Total {
					componentAvailable = nodeInfo.Memory.Total
				}

				availableFromUsed := uint64(0)
				if nodeInfo.Memory.Total > 0 && nodeInfo.Memory.Used > 0 && nodeInfo.Memory.Total >= nodeInfo.Memory.Used {
					availableFromUsed = nodeInfo.Memory.Total - nodeInfo.Memory.Used
				}
				nodeSnapshotRaw.TotalMinusUsed = availableFromUsed

				missingCacheMetrics := nodeInfo.Memory.Available == 0 &&
					nodeInfo.Memory.Avail == 0 &&
					nodeInfo.Memory.Buffers == 0 &&
					nodeInfo.Memory.Cached == 0

				var rrdMetrics rrdMemCacheEntry
				haveRRDMetrics := false
				usedRRDAvailableFallback := false
				rrdMemUsedFallback := false

				if effectiveAvailable == 0 && missingCacheMetrics {
					if metrics, err := m.getNodeRRDMetrics(ctx, client, node.Node); err == nil {
						haveRRDMetrics = true
						rrdMetrics = metrics
						if metrics.available > 0 {
							effectiveAvailable = metrics.available
							usedRRDAvailableFallback = true
						}
						if metrics.used > 0 {
							rrdMemUsedFallback = true
						}
					} else if err != nil {
						log.Debug().
							Err(err).
							Str("instance", instanceName).
							Str("node", node.Node).
							Msg("RRD memavailable fallback unavailable")
					}
				}

				const totalMinusUsedGapTolerance uint64 = 16 * 1024 * 1024
				gapGreaterThanComponents := false
				if availableFromUsed > componentAvailable {
					gap := availableFromUsed - componentAvailable
					if componentAvailable == 0 || gap >= totalMinusUsedGapTolerance {
						gapGreaterThanComponents = true
					}
				}

				derivedFromTotalMinusUsed := !usedRRDAvailableFallback &&
					missingCacheMetrics &&
					availableFromUsed > 0 &&
					gapGreaterThanComponents &&
					effectiveAvailable == availableFromUsed

				switch {
				case effectiveAvailable > 0 && effectiveAvailable <= nodeInfo.Memory.Total:
					// Prefer available/avail fields or derived buffers+cache values when present.
					actualUsed = nodeInfo.Memory.Total - effectiveAvailable
					if actualUsed > nodeInfo.Memory.Total {
						actualUsed = nodeInfo.Memory.Total
					}

					logCtx := log.Debug().
						Str("node", node.Node).
						Uint64("total", nodeInfo.Memory.Total).
						Uint64("effectiveAvailable", effectiveAvailable).
						Uint64("actualUsed", actualUsed).
						Float64("usage", safePercentage(float64(actualUsed), float64(nodeInfo.Memory.Total)))
					if usedRRDAvailableFallback {
						if haveRRDMetrics && rrdMetrics.available > 0 {
							logCtx = logCtx.Uint64("rrdAvailable", rrdMetrics.available)
						}
						logCtx.Msg("node memory: using RRD memavailable fallback (excludes reclaimable cache)")
						nodeMemorySource = "rrd-memavailable"
						nodeFallbackReason = "rrd-memavailable"
						nodeSnapshotRaw.FallbackCalculated = true
						nodeSnapshotRaw.ProxmoxMemorySource = "rrd-memavailable"
					} else if nodeInfo.Memory.Available > 0 {
						logCtx.Msg("node memory: using available field (excludes reclaimable cache)")
						nodeMemorySource = "available-field"
					} else if nodeInfo.Memory.Avail > 0 {
						logCtx.Msg("node memory: using avail field (excludes reclaimable cache)")
						nodeMemorySource = "avail-field"
					} else if derivedFromTotalMinusUsed {
						logCtx.
							Uint64("availableFromUsed", availableFromUsed).
							Uint64("reportedFree", nodeInfo.Memory.Free).
							Msg("Node memory: derived available from total-used gap (cache fields missing)")
						nodeMemorySource = "derived-total-minus-used"
						if nodeFallbackReason == "" {
							nodeFallbackReason = "node-status-total-minus-used"
						}
						nodeSnapshotRaw.FallbackCalculated = true
						nodeSnapshotRaw.ProxmoxMemorySource = "node-status-total-minus-used"
					} else {
						logCtx.
							Uint64("free", nodeInfo.Memory.Free).
							Uint64("buffers", nodeInfo.Memory.Buffers).
							Uint64("cached", nodeInfo.Memory.Cached).
							Msg("Node memory: derived available from free+buffers+cached (excludes reclaimable cache)")
						nodeMemorySource = "derived-free-buffers-cached"
					}
				default:
					switch {
					case rrdMemUsedFallback && haveRRDMetrics && rrdMetrics.used > 0:
						actualUsed = rrdMetrics.used
						if actualUsed > nodeInfo.Memory.Total {
							actualUsed = nodeInfo.Memory.Total
						}
						log.Debug().
							Str("node", node.Node).
							Uint64("total", nodeInfo.Memory.Total).
							Uint64("rrdUsed", rrdMetrics.used).
							Msg("Node memory: using RRD memused fallback (excludes reclaimable cache)")
						nodeMemorySource = "rrd-memused"
						if nodeFallbackReason == "" {
							nodeFallbackReason = "rrd-memused"
						}
						nodeSnapshotRaw.FallbackCalculated = true
						nodeSnapshotRaw.ProxmoxMemorySource = "rrd-memused"
					default:
						// Fallback to traditional used memory if no cache-aware data is exposed
						actualUsed = nodeInfo.Memory.Used
						if actualUsed > nodeInfo.Memory.Total {
							actualUsed = nodeInfo.Memory.Total
						}
						log.Debug().
							Str("node", node.Node).
							Uint64("total", nodeInfo.Memory.Total).
							Uint64("used", actualUsed).
							Msg("Node memory: no cache-aware metrics - using traditional calculation (includes cache)")
						nodeMemorySource = "node-status-used"
					}
				}

				nodeSnapshotRaw.EffectiveAvailable = effectiveAvailable
				if haveRRDMetrics {
					nodeSnapshotRaw.RRDAvailable = rrdMetrics.available
					nodeSnapshotRaw.RRDUsed = rrdMetrics.used
					nodeSnapshotRaw.RRDTotal = rrdMetrics.total
				}

				free := int64(nodeInfo.Memory.Total - actualUsed)
				if free < 0 {
					free = 0
				}

				modelNode.Memory = models.Memory{
					Total: int64(nodeInfo.Memory.Total),
					Used:  int64(actualUsed),
					Free:  free,
					Usage: safePercentage(float64(actualUsed), float64(nodeInfo.Memory.Total)),
				}
				memoryUpdated = true
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
	if !memoryUpdated && effectiveStatus == "online" {
		if prevMem, exists := prevNodeMemory[modelNode.ID]; exists && prevMem.Total > 0 {
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

			if nodeFallbackReason == "" {
				nodeFallbackReason = "preserved-previous-snapshot"
			}
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

	return modelNode, effectiveStatus, nil
}
