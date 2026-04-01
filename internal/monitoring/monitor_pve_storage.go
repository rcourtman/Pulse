package monitoring

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/rs/zerolog/log"
)

type storageFallback struct {
	mu     sync.Mutex
	byNode map[string]models.Disk
	done   chan struct{}
}

func (s *storageFallback) snapshot() map[string]models.Disk {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]models.Disk, len(s.byNode))
	for k, v := range s.byNode {
		out[k] = v
	}
	return out
}

func (m *Monitor) startStorageFallback(
	ctx context.Context,
	instanceName string,
	instanceCfg *config.PVEInstance,
	client PVEClientInterface,
	nodes []proxmox.Node,
	nodeEffectiveStatus map[string]string,
) *storageFallback {
	sf := &storageFallback{
		byNode: make(map[string]models.Disk),
		done:   make(chan struct{}),
	}

	if !instanceCfg.MonitorStorage {
		close(sf.done)
		return sf
	}

	go func() {
		defer close(sf.done)

		// Use a short timeout for storage fallback - it's an optimization, not critical
		storageFallbackTimeout := 10 * time.Second
		// Derive timeout from monitor lifecycle context so shutdown can cancel this work.
		parentCtx := m.runtimeCtx
		if parentCtx == nil {
			parentCtx = ctx
		}
		if parentCtx == nil {
			parentCtx = context.Background()
		}
		storageCtx, storageCancel := context.WithTimeout(parentCtx, storageFallbackTimeout)
		defer storageCancel()

		_, err := client.GetAllStorage(storageCtx)
		if err != nil {
			if storageCtx.Err() != nil {
				log.Debug().
					Str("instance", instanceName).
					Dur("timeout", storageFallbackTimeout).
					Msg("Storage fallback timed out - continuing without disk fallback data")
			}
			return
		}

		for _, node := range nodes {
			// Check if context was cancelled
			select {
			case <-storageCtx.Done():
				log.Debug().
					Str("instance", instanceName).
					Msg("Storage fallback cancelled - partial data collected")
				return
			default:
			}

			// Skip offline nodes to avoid 595 errors
			if nodeEffectiveStatus[node.Node] != "online" {
				continue
			}

			nodeStorages, err := client.GetStorage(storageCtx, node.Node)
			if err != nil {
				continue
			}

			bestRank := 0
			bestFound := false
			var bestStorage proxmox.Storage
			for _, storage := range nodeStorages {
				if reason, skip := readOnlyFilesystemReason(storage.Type, storage.Total, storage.Used); skip {
					log.Debug().
						Str("node", node.Node).
						Str("storage", storage.Storage).
						Str("type", storage.Type).
						Str("skipReason", reason).
						Uint64("total", storage.Total).
						Uint64("used", storage.Used).
						Msg("Skipping read-only storage while building disk fallback")
					continue
				}

				rank, ok := preferredNodeDiskFallbackRank(storage)
				if !ok {
					continue
				}
				if !bestFound || rank < bestRank || (rank == bestRank && storage.Total > bestStorage.Total) {
					bestRank = rank
					bestStorage = storage
					bestFound = true
				}
			}

			if bestFound {
				disk := models.Disk{
					Total: int64(bestStorage.Total),
					Used:  int64(bestStorage.Used),
					Free:  int64(bestStorage.Available),
					Usage: safePercentage(float64(bestStorage.Used), float64(bestStorage.Total)),
				}
				sf.mu.Lock()
				sf.byNode[node.Node] = disk
				sf.mu.Unlock()
				log.Debug().
					Str("node", node.Node).
					Str("storage", bestStorage.Storage).
					Str("type", bestStorage.Type).
					Int("rank", bestRank).
					Float64("usage", disk.Usage).
					Msg("Using preferred storage for disk metrics fallback")
			}
		}
	}()

	return sf
}

func (m *Monitor) awaitStorageFallback(instanceName string, sf *storageFallback, timeout time.Duration) map[string]models.Disk {
	if sf == nil {
		return nil
	}

	timer := time.NewTimer(timeout)
	defer func() {
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
	}()

	select {
	case <-sf.done:
		// Storage fallback completed normally
	case <-timer.C:
		log.Debug().
			Str("instance", instanceName).
			Msg("Storage fallback still running - proceeding without waiting (disk fallback may be unavailable)")
	}

	return sf.snapshot()
}

func (m *Monitor) applyStorageFallbackAndRecordNodeMetrics(
	instanceName string,
	client PVEClientInterface,
	modelNodes []models.Node,
	nodeDiskSources map[string]string,
	storageFallback map[string]models.Disk,
) []models.Node {
	for i := range modelNodes {
		currentDiskSource := nodeDiskSources[modelNodes[i].Name]
		if disk, exists := storageFallback[modelNodes[i].Name]; exists &&
			(modelNodes[i].Disk.Total == 0 || currentDiskSource == "" || currentDiskSource == "nodes-endpoint") {
			modelNodes[i].Disk = disk
			log.Debug().
				Str("node", modelNodes[i].Name).
				Str("previousSource", currentDiskSource).
				Float64("usage", disk.Usage).
				Msg("Applied storage fallback for disk metrics")
		}

		if modelNodes[i].Status == "online" {
			// Record node metrics history only for online nodes
			now := time.Now()
			var nodeNetMetrics *rrdMemCacheEntry
			if client != nil {
				if rrdMetrics, err := m.getNodeRRDMetrics(context.Background(), client, modelNodes[i].Name); err == nil {
					nodeNetMetrics = &rrdMetrics
				}
			}

			m.metricsHistory.AddNodeMetric(modelNodes[i].ID, "cpu", modelNodes[i].CPU*100, now)
			m.metricsHistory.AddNodeMetric(modelNodes[i].ID, "memory", modelNodes[i].Memory.Usage, now)
			m.metricsHistory.AddNodeMetric(modelNodes[i].ID, "disk", modelNodes[i].Disk.Usage, now)
			if nodeNetMetrics != nil {
				if nodeNetMetrics.hasNetIn {
					m.metricsHistory.AddNodeMetric(modelNodes[i].ID, "netin", nodeNetMetrics.netIn, now)
				}
				if nodeNetMetrics.hasNetOut {
					m.metricsHistory.AddNodeMetric(modelNodes[i].ID, "netout", nodeNetMetrics.netOut, now)
				}
			}

			// Also write to persistent store
			if m.metricsStore != nil {
				m.metricsStore.Write("node", modelNodes[i].ID, "cpu", modelNodes[i].CPU*100, now)
				m.metricsStore.Write("node", modelNodes[i].ID, "memory", modelNodes[i].Memory.Usage, now)
				m.metricsStore.Write("node", modelNodes[i].ID, "disk", modelNodes[i].Disk.Usage, now)
				if nodeNetMetrics != nil {
					if nodeNetMetrics.hasNetIn {
						m.metricsStore.Write("node", modelNodes[i].ID, "netin", nodeNetMetrics.netIn, now)
					}
					if nodeNetMetrics.hasNetOut {
						m.metricsStore.Write("node", modelNodes[i].ID, "netout", nodeNetMetrics.netOut, now)
					}
				}
			}
		}

		// Check thresholds for alerts
		m.alertManager.CheckNode(modelNodes[i])
	}

	// Update state again with corrected disk metrics
	m.state.UpdateNodesForInstance(instanceName, modelNodes)

	// Clean up alerts for nodes that no longer exist
	// Get all nodes from the global state (includes all instances)
	existingNodes := make(map[string]bool)
	allState := m.state.GetSnapshot()
	for _, node := range allState.Nodes {
		existingNodes[node.Name] = true
	}
	m.alertManager.CleanupAlertsForNodes(existingNodes)

	return modelNodes
}

func (m *Monitor) pollStorageAsync(
	ctx context.Context,
	instanceName string,
	instanceCfg *config.PVEInstance,
	client PVEClientInterface,
	nodes []proxmox.Node,
) error {
	// Poll storage in background if enabled - storage APIs can be slow (NFS mounts, etc.)
	// so we run this asynchronously to prevent it from causing task timeouts.
	// This is similar to how backup polling runs in the background.
	if !instanceCfg.MonitorStorage {
		return nil
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		go func(inst string, pveClient PVEClientInterface, nodeList []proxmox.Node) {
			defer recoverFromPanic(fmt.Sprintf("pollStorageWithNodes-%s", inst))

			// Use a generous timeout for storage polling - it's not blocking the main task
			storageTimeout := 60 * time.Second
			// Use monitor lifecycle context so shutdown can interrupt detached async polling.
			parentCtx := m.runtimeCtx
			if parentCtx == nil {
				parentCtx = ctx
			}
			if parentCtx == nil {
				parentCtx = context.Background()
			}
			storageCtx, storageCancel := context.WithTimeout(parentCtx, storageTimeout)
			defer storageCancel()

			m.pollStorageWithNodes(storageCtx, inst, pveClient, nodeList)
		}(instanceName, client, nodes)
	}

	return nil
}
