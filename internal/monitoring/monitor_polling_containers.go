package monitoring

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring/errors"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/rs/zerolog/log"
)

func (m *Monitor) collectContainersWithNodes(ctx context.Context, instanceName string, clusterName string, isCluster bool, client PVEClientInterface, nodes []proxmox.Node, nodeEffectiveStatus map[string]string) []models.Container {
	startTime := time.Now()

	// Channel to collect container results from each node
	type nodeResult struct {
		node             string
		containers       []models.Container
		templateSubjects map[string]struct{}
		err              error
	}

	resultChan := make(chan nodeResult, len(nodes))
	var wg sync.WaitGroup

	// Count online nodes for logging
	onlineNodes := 0
	for _, node := range nodes {
		if nodeEffectiveStatus[node.Node] == "online" {
			onlineNodes++
		}
	}

	// Capture the previous guest context once per poll cycle so fallback behavior
	// is based on a consistent pre-poll snapshot.
	prevGuests := m.previousGuestContextForInstance(instanceName)
	prevContainerIsOCI := prevGuests.containerOCIByVMID

	log.Debug().
		Str("instance", instanceName).
		Int("totalNodes", len(nodes)).
		Int("onlineNodes", onlineNodes).
		Msg("Starting parallel container polling")

	// Launch a goroutine for each online node
	for _, node := range nodes {
		// Skip offline nodes
		if nodeEffectiveStatus[node.Node] != "online" {
			log.Debug().
				Str("node", node.Node).
				Str("status", node.Status).
				Msg("Skipping offline node for container polling")
			continue
		}

		wg.Add(1)
		go func(n proxmox.Node) {
			defer wg.Done()

			nodeStart := time.Now()

			// Fetch containers for this node
			containers, err := client.GetContainers(ctx, n.Node)
			if err != nil {
				monErr := errors.NewMonitorError(errors.ErrorTypeAPI, "get_containers", instanceName, err).WithNode(n.Node)
				log.Error().Err(monErr).Str("node", n.Node).Msg("failed to get containers")
				resultChan <- nodeResult{node: n.Node, err: err}
				return
			}

			vmIDs := make([]int, 0, len(containers))
			for _, ct := range containers {
				if ct.Template == 1 {
					continue
				}
				vmIDs = append(vmIDs, int(ct.VMID))
			}

			rootUsageOverrides := m.collectContainerRootUsage(ctx, client, n.Node, vmIDs)

			var nodeContainers []models.Container
			nodeTemplateSubjects := make(map[string]struct{})

			// Process each container
			for _, container := range containers {
				// Skip templates
				if container.Template == 1 {
					if key := pveBackupTemplateSubjectKey(instanceName, "lxc", n.Node, int(container.VMID)); key != "" {
						nodeTemplateSubjects[key] = struct{}{}
					}
					continue
				}

				// Parse tags
				var tags []string
				if container.Tags != "" {
					tags = strings.Split(container.Tags, ";")
				}

				// Generate canonical guest ID: instance:node:vmid
				guestID := makeGuestID(instanceName, n.Node, int(container.VMID))

				sampleTime := time.Now()
				counterObservedAt := observedAtOr(container.ObservedAt, sampleTime)
				currentMetrics := IOMetrics{
					DiskRead:     int64(container.DiskRead),
					DiskWrite:    int64(container.DiskWrite),
					NetworkIn:    int64(container.NetIn),
					NetworkOut:   int64(container.NetOut),
					Timestamp:    counterObservedAt,
					Presence:     pveCounterPresence(container.IOCounters),
					ObservedAt:   counterObservationTimes(counterObservedAt),
					SourceUptime: uint64(container.Uptime),
				}
				statusSnapshot := (*proxmox.Container)(nil)
				if container.Status == "running" {
					statusSnapshot = m.fetchContainerStatusSnapshot(
						ctx,
						client,
						instanceName,
						n.Node,
						container.Name,
						int(container.VMID),
					)
					currentMetrics = mergeContainerRuntimeCounters(currentMetrics, statusSnapshot)
				}
				diskReadRate, diskWriteRate, netInRate, netOutRate := m.rateTracker.CalculateRates(
					makeGuestRateKey(instanceName, "lxc", int(container.VMID)),
					currentMetrics,
				)
				diskReadValue, diskWriteValue, networkInValue, networkOutValue, rateValidity := guestRateValues(
					diskReadRate,
					diskWriteRate,
					netInRate,
					netOutRate,
				)

				// Set CPU to 0 for non-running containers
				cpuUsage := safeFloat(container.CPU)
				if container.Status != "running" {
					cpuUsage = 0
				}

				memTotal, memUsed, memorySource, guestRaw := m.calculateLXCMemory(ctx, instanceName, proxmox.ClusterResource{
					Type:   "lxc",
					Node:   n.Node,
					Name:   container.Name,
					Status: container.Status,
					VMID:   int(container.VMID),
					MaxMem: container.MaxMem,
					Mem:    container.Mem,
				}, client)
				memUsed, memorySource, _ = stabilizeGuestLowTrustMemory(
					m.previousGuestSnapshot(instanceName, "lxc", n.Node, int(container.VMID)),
					container.Status,
					memorySource,
					memTotal,
					memUsed,
					sampleTime,
					false,
				)

				memTotalBytes := clampToInt64(memTotal)
				memUsedBytes := clampToInt64(memUsed)
				if memTotalBytes > 0 && memUsedBytes > memTotalBytes {
					memUsedBytes = memTotalBytes
				}
				memFreeBytes := memTotalBytes - memUsedBytes
				if memFreeBytes < 0 {
					memFreeBytes = 0
				}
				memUsagePercent := safePercentage(float64(memUsedBytes), float64(memTotalBytes))
				memory := models.UnavailableMemory(memTotalBytes)
				if CanonicalMemorySource(memorySource) != "unavailable" {
					memory = models.Memory{
						Total: memTotalBytes,
						Used:  memUsedBytes,
						Free:  memFreeBytes,
						Usage: memUsagePercent,
					}
				}

				diskTotalBytes := clampToInt64(container.MaxDisk)
				diskUsedBytes := clampToInt64(container.Disk)
				if diskTotalBytes > 0 && diskUsedBytes > diskTotalBytes {
					diskUsedBytes = diskTotalBytes
				}
				diskFreeBytes := diskTotalBytes - diskUsedBytes
				if diskFreeBytes < 0 {
					diskFreeBytes = 0
				}
				diskUsagePercent := safePercentage(float64(diskUsedBytes), float64(diskTotalBytes))

				// Create container model
				modelContainer := models.Container{
					ID:       guestID,
					VMID:     int(container.VMID),
					Name:     container.Name,
					Node:     n.Node,
					Pool:     strings.TrimSpace(container.Pool),
					Instance: instanceName,
					Status:   container.Status,
					Type:     "lxc",
					CPU:      cpuUsage,
					CPUs:     int(container.CPUs),
					Memory:   memory,
					Disk: models.Disk{
						Total: diskTotalBytes,
						Used:  diskUsedBytes,
						Free:  diskFreeBytes,
						Usage: diskUsagePercent,
					},
					NetworkIn:      networkInValue,
					NetworkOut:     networkOutValue,
					DiskRead:       diskReadValue,
					DiskWrite:      diskWriteValue,
					IORateValidity: rateValidity,
					Uptime:         int64(container.Uptime),
					Template:       container.Template == 1,
					LastSeen:       sampleTime,
					Tags:           tags,
				}

				if prevContainerIsOCI[modelContainer.VMID] {
					modelContainer.IsOCI = true
					modelContainer.Type = "oci"
				}

				if override, ok := rootUsageOverrides[int(container.VMID)]; ok {
					overrideUsed := clampToInt64(override.Used)
					overrideTotal := clampToInt64(override.Total)

					if overrideUsed > 0 && (modelContainer.Disk.Used == 0 || overrideUsed < modelContainer.Disk.Used) {
						modelContainer.Disk.Used = overrideUsed
					}

					if overrideTotal > 0 {
						modelContainer.Disk.Total = overrideTotal
					}

					if modelContainer.Disk.Total > 0 && modelContainer.Disk.Used > modelContainer.Disk.Total {
						modelContainer.Disk.Used = modelContainer.Disk.Total
					}

					modelContainer.Disk.Free = modelContainer.Disk.Total - modelContainer.Disk.Used
					if modelContainer.Disk.Free < 0 {
						modelContainer.Disk.Free = 0
					}

					modelContainer.Disk.Usage = safePercentage(float64(modelContainer.Disk.Used), float64(modelContainer.Disk.Total))
				}

				m.enrichContainerMetadata(
					ctx,
					client,
					instanceName,
					n.Node,
					&modelContainer,
					statusSnapshot,
				)

				// Zero out metrics for non-running containers
				if container.Status != "running" {
					modelContainer.CPU = 0
					modelContainer.Memory.Usage = 0
					modelContainer.Disk.Usage = 0
					modelContainer.NetworkIn = 0
					modelContainer.NetworkOut = 0
					modelContainer.DiskRead = 0
					modelContainer.DiskWrite = 0
					modelContainer.IORateValidity = models.IORateValidity{
						Explicit:   true,
						DiskRead:   true,
						DiskWrite:  true,
						NetworkIn:  true,
						NetworkOut: true,
					}
				}

				// Trigger guest metadata migration if old format exists
				if m.guestMetadataStore != nil {
					m.guestMetadataStore.GetWithLegacyMigration(guestID, instanceName, n.Node, int(container.VMID))
				}

				m.recordGuestSnapshot(instanceName, modelContainer.Type, n.Node, int(container.VMID), GuestMemorySnapshot{
					Name:           modelContainer.Name,
					Status:         modelContainer.Status,
					RetrievedAt:    modelContainer.LastSeen,
					MemorySource:   memorySource,
					FallbackReason: guestMemoryFallbackReason(memorySource),
					Memory:         modelContainer.Memory,
					Raw:            guestRaw,
				})

				nodeContainers = append(nodeContainers, modelContainer)

				// Check alerts
				m.alertManager.CheckGuest(modelContainer, instanceName)
			}

			nodeDuration := time.Since(nodeStart)
			log.Debug().
				Str("node", n.Node).
				Int("containers", len(nodeContainers)).
				Dur("duration", nodeDuration).
				Msg("Node container polling completed")

			resultChan <- nodeResult{node: n.Node, containers: nodeContainers, templateSubjects: nodeTemplateSubjects}
		}(node)
	}

	// Close channel when all goroutines complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results from all nodes
	var allContainers []models.Container
	lxcTemplateSubjects := make(map[string]struct{})
	successfulNodes := 0
	failedNodes := 0
	failedNodeNames := make(map[string]struct{})

	for result := range resultChan {
		if result.err != nil {
			failedNodes++
			failedNodeNames[result.node] = struct{}{}
		} else {
			successfulNodes++
			allContainers = append(allContainers, result.containers...)
			for key := range result.templateSubjects {
				lxcTemplateSubjects[key] = struct{}{}
			}
		}
	}
	if failedNodes == 0 && successfulNodes > 0 {
		m.updatePVEBackupTemplateSubjectsForType(instanceName, "lxc", lxcTemplateSubjects)
	}

	preservedContainers := 0
	if len(failedNodeNames) > 0 {
		for _, container := range prevGuests.containers {
			if _, failed := failedNodeNames[container.Node]; failed {
				allContainers = append(allContainers, container)
				preservedContainers++
			}
		}
	}
	if preservedContainers > 0 {
		log.Warn().
			Str("instance", instanceName).
			Int("preservedContainers", preservedContainers).
			Int("failedNodes", failedNodes).
			Msg("Preserved prior containers for nodes whose enumeration failed")
	}

	// Check Docker presence for containers that need it (new, restarted, started)
	allContainers = m.CheckContainersForDocker(ctx, allContainers)
	m.CollectProxmoxGuestDockerInventory(ctx, allContainers)

	// Record guest metrics history for running containers (enables sparkline/trends view)
	if !shouldSkipNativeMockStateMetricWrites() {
		now := time.Now()
		for _, ct := range allContainers {
			if ct.Status != "running" {
				continue
			}
			diskRead, diskWrite, networkIn, networkOut := guestHistoryRates(
				ct.DiskRead,
				ct.DiskWrite,
				ct.NetworkIn,
				ct.NetworkOut,
				ct.IORateValidity,
			)
			m.recordGuestMetric(
				"container",
				ct.ID,
				unifiedresources.ProxmoxGuestCPUPercent(ct.CPU),
				historyMemoryUsage(ct.Memory),
				ct.Disk.Usage,
				diskRead,
				diskWrite,
				networkIn,
				networkOut,
				now,
			)
		}
	}

	duration := time.Since(startTime)
	log.Debug().
		Str("instance", instanceName).
		Int("totalContainers", len(allContainers)).
		Int("successfulNodes", successfulNodes).
		Int("failedNodes", failedNodes).
		Dur("duration", duration).
		Msg("Parallel container polling completed")

	return allContainers
}

// pollContainersWithNodes retains the focused single-kind polling entry point
// used by tests and maintenance callers. The production guest cycle uses
// collectContainersWithNodes and publishes both guest kinds atomically.
func (m *Monitor) pollContainersWithNodes(ctx context.Context, instanceName string, clusterName string, isCluster bool, client PVEClientInterface, nodes []proxmox.Node, nodeEffectiveStatus map[string]string) {
	containers := m.collectContainersWithNodes(ctx, instanceName, clusterName, isCluster, client, nodes, nodeEffectiveStatus)
	m.state.UpdateContainersForInstance(instanceName, containers)
}

// pollStorageWithNodes polls storage from all nodes in parallel using goroutines
