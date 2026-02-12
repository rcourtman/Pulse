package monitoring

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/rs/zerolog/log"
)

func convertPoolInfoToModel(poolInfo *proxmox.ZFSPoolInfo) *models.ZFSPool {
	if poolInfo == nil {
		return nil
	}

	// Use the converter from the proxmox package
	proxmoxPool := poolInfo.ConvertToModelZFSPool()

	// Convert to our internal model
	modelPool := &models.ZFSPool{
		Name:           proxmoxPool.Name,
		State:          proxmoxPool.State,
		Status:         proxmoxPool.Status,
		Scan:           proxmoxPool.Scan,
		ReadErrors:     proxmoxPool.ReadErrors,
		WriteErrors:    proxmoxPool.WriteErrors,
		ChecksumErrors: proxmoxPool.ChecksumErrors,
		Devices:        make([]models.ZFSDevice, 0, len(proxmoxPool.Devices)),
	}

	// Convert devices
	for _, dev := range proxmoxPool.Devices {
		modelPool.Devices = append(modelPool.Devices, models.ZFSDevice{
			Name:           dev.Name,
			Type:           dev.Type,
			State:          dev.State,
			ReadErrors:     dev.ReadErrors,
			WriteErrors:    dev.WriteErrors,
			ChecksumErrors: dev.ChecksumErrors,
			Message:        dev.Message,
		})
	}

	return modelPool
}

// pollVMsWithNodes polls VMs from all nodes in parallel using goroutines
// When the instance is part of a cluster, the cluster name is used for guest IDs to prevent duplicates
// when multiple cluster nodes are configured as separate PVE instances.
func (m *Monitor) pollStorageWithNodes(ctx context.Context, instanceName string, client PVEClientInterface, nodes []proxmox.Node) {
	startTime := time.Now()

	instanceCfg := m.getInstanceConfig(instanceName)

	// Determine the storage instance name - use cluster name for clustered setups
	// This must match what is set in each storage item's Instance field
	storageInstanceName := instanceName
	if instanceCfg != nil && instanceCfg.IsCluster && instanceCfg.ClusterName != "" {
		storageInstanceName = instanceCfg.ClusterName
	}

	// Get cluster storage configuration first (single call)
	clusterStorages, err := client.GetAllStorage(ctx)
	clusterStorageAvailable := err == nil
	if err != nil {
		// Provide detailed context about cluster health issues
		if strings.Contains(err.Error(), "no healthy nodes available") {
			log.Warn().
				Err(err).
				Str("instance", instanceName).
				Msg("Cluster health check shows no healthy endpoints - continuing with direct node storage polling. Check network connectivity and API accessibility from Pulse to each cluster node.")
		} else {
			log.Warn().
				Err(err).
				Str("instance", instanceName).
				Msg("Failed to get cluster storage config - will continue with node storage only")
		}
	}

	// Create a map for quick lookup of cluster storage config
	clusterStorageMap := make(map[string]proxmox.Storage)
	cephDetected := false
	if clusterStorageAvailable {
		for _, cs := range clusterStorages {
			clusterStorageMap[cs.Storage] = cs
			if !cephDetected && isCephStorageType(cs.Type) {
				cephDetected = true
			}
		}
	}

	// Channel to collect storage results from each node
	type nodeResult struct {
		node    string
		storage []models.Storage
		err     error
	}

	resultChan := make(chan nodeResult, len(nodes))
	var wg sync.WaitGroup

	// Count online nodes for logging
	onlineNodes := 0
	for _, node := range nodes {
		if node.Status == "online" {
			onlineNodes++
		}
	}

	log.Debug().
		Str("instance", instanceName).
		Int("totalNodes", len(nodes)).
		Int("onlineNodes", onlineNodes).
		Msg("Starting parallel storage polling")

	// Get existing storage from state to preserve data for offline nodes
	currentState := m.state.GetSnapshot()
	existingStorageMap := make(map[string]models.Storage)
	for _, storage := range currentState.Storage {
		if storage.Instance == instanceName {
			existingStorageMap[storage.ID] = storage
		}
	}

	// Track which nodes we successfully polled
	polledNodes := make(map[string]bool)

	// Launch a goroutine for each online node
	for _, node := range nodes {
		// Skip offline nodes but preserve their existing storage data
		if node.Status != "online" {
			log.Debug().
				Str("node", node.Node).
				Str("status", node.Status).
				Msg("Skipping offline node for storage polling - preserving existing data")
			continue
		}

		wg.Add(1)
		go func(n proxmox.Node) {
			defer wg.Done()

			nodeStart := time.Now()

			// Fetch storage for this node
			nodeStorage, err := client.GetStorage(ctx, n.Node)
			if err != nil {
				if shouldAttemptFallback(err) {
					if fallbackStorage, ferr := m.fetchNodeStorageFallback(ctx, instanceCfg, n.Node); ferr == nil {
						log.Warn().
							Str("instance", instanceName).
							Str("node", n.Node).
							Err(err).
							Msg("Primary storage query failed; using direct node fallback")
						nodeStorage = fallbackStorage
						err = nil
					} else {
						log.Warn().
							Str("instance", instanceName).
							Str("node", n.Node).
							Err(ferr).
							Msg("Storage fallback to direct node query failed")
					}
				}
			}
			if err != nil {
				// Handle timeout gracefully - unavailable storage (e.g., NFS mounts) can cause this
				errStr := err.Error()
				if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded") {
					log.Warn().
						Str("node", n.Node).
						Str("instance", instanceName).
						Msg("Storage query timed out - likely due to unavailable storage mounts. Preserving existing storage data for this node.")
					// Send an error result so the node is marked as failed and preservation logic works
					resultChan <- nodeResult{node: n.Node, err: err}
					return
				}
				// For other errors, log as error
				log.Error().
					Err(err).
					Str("node", n.Node).
					Str("instance", instanceName).
					Msg("Failed to get node storage - check API permissions")
				resultChan <- nodeResult{node: n.Node, err: err}
				return
			}

			var nodeStorageList []models.Storage

			// Get ZFS pool status for this node if any storage is ZFS
			// This is now production-ready with proper API integration
			var zfsPoolMap = make(map[string]*models.ZFSPool)
			enableZFSMonitoring := os.Getenv("PULSE_DISABLE_ZFS_MONITORING") != "true" // Enabled by default

			if enableZFSMonitoring {
				hasZFSStorage := false
				for _, storage := range nodeStorage {
					if storage.Type == "zfspool" || storage.Type == "zfs" || storage.Type == "local-zfs" {
						hasZFSStorage = true
						break
					}
				}

				if hasZFSStorage {
					if poolInfos, err := client.GetZFSPoolsWithDetails(ctx, n.Node); err == nil {
						log.Debug().
							Str("node", n.Node).
							Int("pools", len(poolInfos)).
							Msg("Successfully fetched ZFS pool details")

						// Convert to our model format
						for _, poolInfo := range poolInfos {
							modelPool := convertPoolInfoToModel(&poolInfo)
							if modelPool != nil {
								zfsPoolMap[poolInfo.Name] = modelPool
							}
						}
					} else {
						// Log but don't fail - ZFS monitoring is optional
						log.Debug().
							Err(err).
							Str("node", n.Node).
							Str("instance", instanceName).
							Msg("Could not get ZFS pool status (may require additional permissions)")
					}
				}
			}

			// Process each storage
			for _, storage := range nodeStorage {
				if reason, skip := readOnlyFilesystemReason(storage.Type, storage.Total, storage.Used); skip {
					log.Debug().
						Str("node", n.Node).
						Str("storage", storage.Storage).
						Str("type", storage.Type).
						Str("skipReason", reason).
						Uint64("total", storage.Total).
						Uint64("used", storage.Used).
						Msg("Skipping read-only storage mount")
					continue
				}

				// Create storage ID
				var storageID string
				if instanceName == n.Node {
					storageID = fmt.Sprintf("%s-%s", n.Node, storage.Storage)
				} else {
					storageID = fmt.Sprintf("%s-%s-%s", instanceName, n.Node, storage.Storage)
				}

				// Get cluster config for this storage
				clusterConfig, hasClusterConfig := clusterStorageMap[storage.Storage]

				// Determine if shared - check multiple sources:
				// 1. Per-node API returns shared flag directly
				// 2. Cluster config API also has shared flag (if available)
				// 3. Some storage types are inherently cluster-wide even if flags aren't set
				shared := storage.Shared == 1 ||
					(hasClusterConfig && clusterConfig.Shared == 1) ||
					isInherentlySharedStorageType(storage.Type)

				// Create storage model
				// Initialize Enabled/Active from per-node API response
				// Use storageInstanceName (cluster name when clustered) to match node ID format
				modelStorage := models.Storage{
					ID:       storageID,
					Name:     storage.Storage,
					Node:     n.Node,
					Instance: storageInstanceName,
					Type:     storage.Type,
					Status:   "available",
					Path:     storage.Path,
					Total:    int64(storage.Total),
					Used:     int64(storage.Used),
					Free:     int64(storage.Available),
					Usage:    safePercentage(float64(storage.Used), float64(storage.Total)),
					Content:  sortContent(storage.Content),
					Shared:   shared,
					Enabled:  storage.Enabled == 1,
					Active:   storage.Active == 1,
				}

				if hasClusterConfig {
					if nodes := parseClusterStorageNodes(clusterConfig.Nodes); len(nodes) > 0 {
						modelStorage.Nodes = nodes
					}
					if modelStorage.Path == "" && clusterConfig.Path != "" {
						modelStorage.Path = clusterConfig.Path
					}
				}

				// If this is ZFS storage, attach pool status information
				if storage.Type == "zfspool" || storage.Type == "zfs" || storage.Type == "local-zfs" {
					// Try to match by storage name or by common ZFS pool names
					poolName := storage.Storage

					// Common mappings
					if poolName == "local-zfs" {
						poolName = "rpool/data" // Common default
					}

					// Look for exact match first
					if pool, found := zfsPoolMap[poolName]; found {
						modelStorage.ZFSPool = pool
					} else {
						// Try partial matches for common patterns
						for name, pool := range zfsPoolMap {
							if name == "rpool" && strings.Contains(storage.Storage, "rpool") {
								modelStorage.ZFSPool = pool
								break
							} else if name == "data" && strings.Contains(storage.Storage, "data") {
								modelStorage.ZFSPool = pool
								break
							}
						}
					}
				}

				// Override with cluster config if available, but only when the
				// cluster metadata explicitly carries those flags. Some storage
				// types (notably PBS) omit enabled/active, and forcing them to 0
				// would make us skip backup polling even though the node reports
				// the storage as available.
				if hasClusterConfig {
					// Cluster metadata is inconsistent across storage types; PBS storages often omit
					// enabled/active entirely (decode as zero). To avoid marking usable storages as
					// disabled, only override when the cluster explicitly sets the flag to 1.
					if clusterConfig.Enabled == 1 {
						modelStorage.Enabled = true
					}
					if clusterConfig.Active == 1 {
						modelStorage.Active = true
					}
				}

				// Determine status based on enabled/active flags
				// Priority: disabled storage always shows as "disabled", regardless of active state
				if !modelStorage.Enabled {
					modelStorage.Status = "disabled"
				} else if modelStorage.Active {
					modelStorage.Status = "available"
				} else {
					modelStorage.Status = "inactive"
				}

				nodeStorageList = append(nodeStorageList, modelStorage)
			}

			nodeDuration := time.Since(nodeStart)
			log.Debug().
				Str("node", n.Node).
				Int("storage", len(nodeStorageList)).
				Dur("duration", nodeDuration).
				Msg("Node storage polling completed")

			// If we got empty storage but have existing storage for this node, don't mark as successfully polled
			// This allows preservation logic to keep the existing storage
			if len(nodeStorageList) == 0 {
				// Check if we have existing storage for this node
				hasExisting := false
				for _, existing := range existingStorageMap {
					if existing.Node == n.Node {
						hasExisting = true
						break
					}
				}
				if hasExisting {
					log.Warn().
						Str("node", n.Node).
						Str("instance", instanceName).
						Msg("Node returned empty storage but has existing storage - preserving existing data")
					// Don't send result, allowing preservation logic to work
					return
				}
			}

			resultChan <- nodeResult{node: n.Node, storage: nodeStorageList}
		}(node)
	}

	// Close channel when all goroutines complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results from all nodes
	var allStorage []models.Storage
	type sharedStorageAggregation struct {
		storage models.Storage
		nodes   map[string]struct{}
		nodeIDs map[string]struct{}
	}
	sharedStorageMap := make(map[string]*sharedStorageAggregation) // Map to keep shared storage entries with node affiliations

	toSortedSlice := func(set map[string]struct{}) []string {
		slice := make([]string, 0, len(set))
		for value := range set {
			slice = append(slice, value)
		}
		sort.Strings(slice)
		return slice
	}
	successfulNodes := 0
	failedNodes := 0

	for result := range resultChan {
		if result.err != nil {
			failedNodes++
		} else {
			successfulNodes++
			polledNodes[result.node] = true // Mark this node as successfully polled
			for _, storage := range result.storage {
				if storage.Shared {
					// For shared storage, aggregate by storage name so we can retain the reporting nodes
					key := storage.Name
					nodeIdentifier := fmt.Sprintf("%s-%s", storage.Instance, storage.Node)

					if entry, exists := sharedStorageMap[key]; exists {
						entry.nodes[storage.Node] = struct{}{}
						entry.nodeIDs[nodeIdentifier] = struct{}{}

						// Prefer the entry with the most up-to-date utilization data
						if storage.Used > entry.storage.Used || (storage.Total > entry.storage.Total && storage.Used == entry.storage.Used) {
							entry.storage.Total = storage.Total
							entry.storage.Used = storage.Used
							entry.storage.Free = storage.Free
							entry.storage.Usage = storage.Usage
							entry.storage.ZFSPool = storage.ZFSPool
							entry.storage.Status = storage.Status
							entry.storage.Enabled = storage.Enabled
							entry.storage.Active = storage.Active
							entry.storage.Content = storage.Content
							entry.storage.Type = storage.Type
						}
					} else {
						sharedStorageMap[key] = &sharedStorageAggregation{
							storage: storage,
							nodes:   map[string]struct{}{storage.Node: {}},
							nodeIDs: map[string]struct{}{nodeIdentifier: {}},
						}
					}
				} else {
					// Non-shared storage goes directly to results
					allStorage = append(allStorage, storage)
				}
			}
		}
	}

	// Add deduplicated shared storage to results
	for _, entry := range sharedStorageMap {
		entry.storage.Node = "cluster"
		entry.storage.Nodes = toSortedSlice(entry.nodes)
		entry.storage.NodeIDs = toSortedSlice(entry.nodeIDs)
		entry.storage.NodeCount = len(entry.storage.Nodes)
		// Fix for #1049: Use a consistent ID for shared storage that doesn't
		// include the node name, preventing duplicates when different nodes
		// report the same shared storage across polling cycles.
		entry.storage.ID = fmt.Sprintf("%s-cluster-%s", entry.storage.Instance, entry.storage.Name)
		allStorage = append(allStorage, entry.storage)
	}

	// Preserve existing storage data for nodes that weren't polled (offline or error)
	preservedCount := 0
	for _, existingStorage := range existingStorageMap {
		// Only preserve if we didn't poll this node
		if !polledNodes[existingStorage.Node] && existingStorage.Node != "cluster" {
			allStorage = append(allStorage, existingStorage)
			preservedCount++
			log.Debug().
				Str("node", existingStorage.Node).
				Str("storage", existingStorage.Name).
				Msg("Preserving existing storage data for unpolled node")
		}
	}

	// Record metrics and check alerts for all storage devices
	for _, storage := range allStorage {
		if m.metricsHistory != nil {
			timestamp := time.Now()
			m.metricsHistory.AddStorageMetric(storage.ID, "usage", storage.Usage, timestamp)
			m.metricsHistory.AddStorageMetric(storage.ID, "used", float64(storage.Used), timestamp)
			m.metricsHistory.AddStorageMetric(storage.ID, "total", float64(storage.Total), timestamp)
			m.metricsHistory.AddStorageMetric(storage.ID, "avail", float64(storage.Free), timestamp)

			// Also write to persistent store for enterprise reporting
			if m.metricsStore != nil {
				m.metricsStore.Write("storage", storage.ID, "usage", storage.Usage, timestamp)
				m.metricsStore.Write("storage", storage.ID, "used", float64(storage.Used), timestamp)
				m.metricsStore.Write("storage", storage.ID, "total", float64(storage.Total), timestamp)
				m.metricsStore.Write("storage", storage.ID, "avail", float64(storage.Free), timestamp)
			}
		}

		if m.alertManager != nil {
			m.alertManager.CheckStorage(storage)
		}
	}

	if !cephDetected {
		for _, storage := range allStorage {
			if isCephStorageType(storage.Type) {
				cephDetected = true
				break
			}
		}
	}

	// Update state with all storage
	m.state.UpdateStorageForInstance(storageInstanceName, allStorage)

	// Poll Ceph cluster data after refreshing storage information
	if instanceCfg == nil || !instanceCfg.DisableCeph {
		m.pollCephCluster(ctx, instanceName, client, cephDetected)
	}

	duration := time.Since(startTime)

	// Warn if all nodes failed to get storage
	if successfulNodes == 0 && failedNodes > 0 {
		log.Error().
			Str("instance", instanceName).
			Int("failedNodes", failedNodes).
			Msg("All nodes failed to retrieve storage - check Proxmox API permissions for Datastore.Audit on all storage")
	} else {
		log.Debug().
			Str("instance", instanceName).
			Int("totalStorage", len(allStorage)).
			Int("successfulNodes", successfulNodes).
			Int("failedNodes", failedNodes).
			Int("preservedStorage", preservedCount).
			Dur("duration", duration).
			Msg("Parallel storage polling completed")
	}
}

func shouldAttemptFallback(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline exceeded") || strings.Contains(msg, "context canceled")
}

func (m *Monitor) fetchNodeStorageFallback(ctx context.Context, instanceCfg *config.PVEInstance, nodeName string) ([]proxmox.Storage, error) {
	if m == nil || instanceCfg == nil || !instanceCfg.IsCluster || len(instanceCfg.ClusterEndpoints) == 0 {
		return nil, fmt.Errorf("fallback unavailable")
	}

	var target string
	hasFingerprint := strings.TrimSpace(instanceCfg.Fingerprint) != ""
	for _, ep := range instanceCfg.ClusterEndpoints {
		if !strings.EqualFold(ep.NodeName, nodeName) {
			continue
		}
		target = clusterEndpointEffectiveURL(ep, instanceCfg.VerifySSL, hasFingerprint)
		if target != "" {
			break
		}
	}
	if strings.TrimSpace(target) == "" {
		return nil, fmt.Errorf("fallback endpoint missing for node %s", nodeName)
	}

	cfg := proxmox.ClientConfig{
		Host:        target,
		VerifySSL:   instanceCfg.VerifySSL,
		Fingerprint: instanceCfg.Fingerprint,
		Timeout:     m.pollTimeout,
	}
	if instanceCfg.TokenName != "" && instanceCfg.TokenValue != "" {
		cfg.TokenName = instanceCfg.TokenName
		cfg.TokenValue = instanceCfg.TokenValue
	} else {
		cfg.User = instanceCfg.User
		cfg.Password = instanceCfg.Password
	}

	directClient, err := proxmox.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("create fallback proxmox client for node %s: %w", nodeName, err)
	}

	return directClient.GetStorage(ctx, nodeName)
}

// pollPVENode polls a single PVE node and returns the result
func parseClusterStorageNodes(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '\t' || r == '\n'
	})
	if len(parts) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(parts))
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if _, exists := seen[part]; exists {
			continue
		}
		seen[part] = struct{}{}
		result = append(result, part)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
