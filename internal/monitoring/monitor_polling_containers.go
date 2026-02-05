package monitoring

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/errors"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/rs/zerolog/log"
)

func (m *Monitor) pollContainersWithNodes(ctx context.Context, instanceName string, clusterName string, isCluster bool, client PVEClientInterface, nodes []proxmox.Node, nodeEffectiveStatus map[string]string) {
	startTime := time.Now()

	// Channel to collect container results from each node
	type nodeResult struct {
		node       string
		containers []models.Container
		err        error
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

	// Seed OCI classification from previous state so we never "downgrade" to LXC
	// if container config fetching intermittently fails (permissions or transient API errors).
	prevState := m.GetState()
	prevContainerIsOCI := make(map[int]bool)
	for _, ct := range prevState.Containers {
		if ct.Instance != instanceName {
			continue
		}
		if ct.VMID <= 0 {
			continue
		}
		if ct.Type == "oci" || ct.IsOCI {
			prevContainerIsOCI[ct.VMID] = true
		}
	}

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
				log.Error().Err(monErr).Str("node", n.Node).Msg("Failed to get containers")
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

			// Process each container
			for _, container := range containers {
				// Skip templates
				if container.Template == 1 {
					continue
				}

				// Parse tags
				var tags []string
				if container.Tags != "" {
					tags = strings.Split(container.Tags, ";")
				}

				// Generate canonical guest ID: instance:node:vmid
				guestID := makeGuestID(instanceName, n.Node, int(container.VMID))

				// Calculate I/O rates
				currentMetrics := IOMetrics{
					DiskRead:   int64(container.DiskRead),
					DiskWrite:  int64(container.DiskWrite),
					NetworkIn:  int64(container.NetIn),
					NetworkOut: int64(container.NetOut),
					Timestamp:  time.Now(),
				}
				diskReadRate, diskWriteRate, netInRate, netOutRate := m.rateTracker.CalculateRates(guestID, currentMetrics)

				// Set CPU to 0 for non-running containers
				cpuUsage := safeFloat(container.CPU)
				if container.Status != "running" {
					cpuUsage = 0
				}

				memTotalBytes := clampToInt64(container.MaxMem)
				memUsedBytes := clampToInt64(container.Mem)
				if memTotalBytes > 0 && memUsedBytes > memTotalBytes {
					memUsedBytes = memTotalBytes
				}
				memFreeBytes := memTotalBytes - memUsedBytes
				if memFreeBytes < 0 {
					memFreeBytes = 0
				}
				memUsagePercent := safePercentage(float64(memUsedBytes), float64(memTotalBytes))

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
					Instance: instanceName,
					Status:   container.Status,
					Type:     "lxc",
					CPU:      cpuUsage,
					CPUs:     int(container.CPUs),
					Memory: models.Memory{
						Total: memTotalBytes,
						Used:  memUsedBytes,
						Free:  memFreeBytes,
						Usage: memUsagePercent,
					},
					Disk: models.Disk{
						Total: diskTotalBytes,
						Used:  diskUsedBytes,
						Free:  diskFreeBytes,
						Usage: diskUsagePercent,
					},
					NetworkIn:  max(0, int64(netInRate)),
					NetworkOut: max(0, int64(netOutRate)),
					DiskRead:   max(0, int64(diskReadRate)),
					DiskWrite:  max(0, int64(diskWriteRate)),
					Uptime:     int64(container.Uptime),
					Template:   container.Template == 1,
					LastSeen:   time.Now(),
					Tags:       tags,
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

				m.enrichContainerMetadata(ctx, client, instanceName, n.Node, &modelContainer)

				// Zero out metrics for non-running containers
				if container.Status != "running" {
					modelContainer.CPU = 0
					modelContainer.Memory.Usage = 0
					modelContainer.Disk.Usage = 0
					modelContainer.NetworkIn = 0
					modelContainer.NetworkOut = 0
					modelContainer.DiskRead = 0
					modelContainer.DiskWrite = 0
				}

				// Trigger guest metadata migration if old format exists
				if m.guestMetadataStore != nil {
					m.guestMetadataStore.GetWithLegacyMigration(guestID, instanceName, n.Node, int(container.VMID))
				}

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

			resultChan <- nodeResult{node: n.Node, containers: nodeContainers}
		}(node)
	}

	// Close channel when all goroutines complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results from all nodes
	var allContainers []models.Container
	successfulNodes := 0
	failedNodes := 0

	for result := range resultChan {
		if result.err != nil {
			failedNodes++
		} else {
			successfulNodes++
			allContainers = append(allContainers, result.containers...)
		}
	}

	// If we got ZERO containers but had containers before (likely cluster health issue),
	// preserve previous containers instead of clearing them
	if len(allContainers) == 0 && len(nodes) > 0 {
		prevState := m.GetState()
		prevContainerCount := 0
		for _, container := range prevState.Containers {
			if container.Instance == instanceName {
				allContainers = append(allContainers, container)
				prevContainerCount++
			}
		}
		if prevContainerCount > 0 {
			log.Warn().
				Str("instance", instanceName).
				Int("prevContainers", prevContainerCount).
				Int("successfulNodes", successfulNodes).
				Int("totalNodes", len(nodes)).
				Msg("Traditional polling returned zero containers but had containers before - preserving previous containers")
		}
	}

	// Check Docker presence for containers that need it (new, restarted, started)
	allContainers = m.CheckContainersForDocker(ctx, allContainers)

	// Update state with all containers
	m.state.UpdateContainersForInstance(instanceName, allContainers)

	// Record guest metrics history for running containers (enables sparkline/trends view)
	now := time.Now()
	for _, ct := range allContainers {
		if ct.Status == "running" {
			m.metricsHistory.AddGuestMetric(ct.ID, "cpu", ct.CPU*100, now)
			m.metricsHistory.AddGuestMetric(ct.ID, "memory", ct.Memory.Usage, now)
			if ct.Disk.Usage >= 0 {
				m.metricsHistory.AddGuestMetric(ct.ID, "disk", ct.Disk.Usage, now)
			}
			// Also write to persistent store
			if m.metricsStore != nil {
				m.metricsStore.Write("container", ct.ID, "cpu", ct.CPU*100, now)
				m.metricsStore.Write("container", ct.ID, "memory", ct.Memory.Usage, now)
				if ct.Disk.Usage >= 0 {
					m.metricsStore.Write("container", ct.ID, "disk", ct.Disk.Usage, now)
				}
			}
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
}

// pollStorageWithNodes polls storage from all nodes in parallel using goroutines
