package monitoring

import (
	"context"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/errors"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/rs/zerolog/log"
)

// convertPoolInfoToModel converts Proxmox ZFS pool info to our model
func convertPoolInfoToModel(poolInfo *proxmox.ZFSPoolInfo) *models.ZFSPool {
	if poolInfo == nil {
		return nil
	}

	// Use the converter from the proxmox package
	proxmoxPool := poolInfo.ConvertToModelZFSPool()
	if proxmoxPool == nil {
		return nil
	}

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

// pollVMsWithNodesOptimized polls VMs from all nodes in parallel using goroutines
func (m *Monitor) pollVMsWithNodesOptimized(ctx context.Context, instanceName string, client PVEClientInterface, nodes []proxmox.Node) {
	startTime := time.Now()

	// Channel to collect VM results from each node
	type nodeResult struct {
		node string
		vms  []models.VM
		err  error
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

	log.Info().
		Str("instance", instanceName).
		Int("totalNodes", len(nodes)).
		Int("onlineNodes", onlineNodes).
		Msg("Starting parallel VM polling")

	// Launch a goroutine for each online node
	for _, node := range nodes {
		// Skip offline nodes
		if node.Status != "online" {
			log.Debug().
				Str("node", node.Node).
				Str("status", node.Status).
				Msg("Skipping offline node for VM polling")
			continue
		}

		wg.Add(1)
		go func(n proxmox.Node) {
			defer wg.Done()

			nodeStart := time.Now()

			// Fetch VMs for this node
			vms, err := client.GetVMs(ctx, n.Node)
			if err != nil {
				monErr := errors.NewMonitorError(errors.ErrorTypeAPI, "get_vms", instanceName, err).WithNode(n.Node)
				log.Error().Err(monErr).Str("node", n.Node).Msg("Failed to get VMs")
				resultChan <- nodeResult{node: n.Node, err: err}
				return
			}

			var nodeVMs []models.VM

			// Process each VM
			for _, vm := range vms {
				// Skip templates
				if vm.Template == 1 {
					continue
				}

				// Parse tags
				var tags []string
				if vm.Tags != "" {
					tags = strings.Split(vm.Tags, ";")
				}

				// Create guest ID
				var guestID string
				if instanceName == n.Node {
					guestID = fmt.Sprintf("%s-%d", n.Node, vm.VMID)
				} else {
					guestID = fmt.Sprintf("%s-%s-%d", instanceName, n.Node, vm.VMID)
				}

				guestRaw := VMMemoryRaw{
					ListingMem:    vm.Mem,
					ListingMaxMem: vm.MaxMem,
					Agent:         vm.Agent,
				}
				memorySource := "listing-mem"

				// Initialize metrics from VM listing (may be 0 for disk I/O)
				diskReadBytes := int64(vm.DiskRead)
				diskWriteBytes := int64(vm.DiskWrite)
				networkInBytes := int64(vm.NetIn)
				networkOutBytes := int64(vm.NetOut)

				// Get memory info for running VMs (and agent status for disk)
				memUsed := uint64(0)
				memTotal := vm.MaxMem
				var vmStatus *proxmox.VMStatus
				var ipAddresses []string
				var networkInterfaces []models.GuestNetworkInterface
				var osName, osVersion string

				if vm.Status == "running" {
					// Try to get detailed VM status (but don't wait too long)
					statusCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
					if status, err := client.GetVMStatus(statusCtx, n.Node, vm.VMID); err == nil {
						vmStatus = status
						guestRaw.StatusMaxMem = status.MaxMem
						guestRaw.StatusMem = status.Mem
						guestRaw.StatusFreeMem = status.FreeMem
						guestRaw.Balloon = status.Balloon
						guestRaw.BalloonMin = status.BalloonMin
						guestRaw.Agent = status.Agent
						memAvailable := uint64(0)
						if status.MemInfo != nil {
							guestRaw.MemInfoUsed = status.MemInfo.Used
							guestRaw.MemInfoFree = status.MemInfo.Free
							guestRaw.MemInfoTotal = status.MemInfo.Total
							guestRaw.MemInfoAvailable = status.MemInfo.Available
							guestRaw.MemInfoBuffers = status.MemInfo.Buffers
							guestRaw.MemInfoCached = status.MemInfo.Cached
							guestRaw.MemInfoShared = status.MemInfo.Shared
							componentAvailable := status.MemInfo.Free
							if status.MemInfo.Buffers > 0 {
								if math.MaxUint64-componentAvailable < status.MemInfo.Buffers {
									componentAvailable = math.MaxUint64
								} else {
									componentAvailable += status.MemInfo.Buffers
								}
							}
							if status.MemInfo.Cached > 0 {
								if math.MaxUint64-componentAvailable < status.MemInfo.Cached {
									componentAvailable = math.MaxUint64
								} else {
									componentAvailable += status.MemInfo.Cached
								}
							}
							if status.MemInfo.Total > 0 && componentAvailable > status.MemInfo.Total {
								componentAvailable = status.MemInfo.Total
							}

							availableFromUsed := uint64(0)
							if status.MemInfo.Total > 0 && status.MemInfo.Used > 0 && status.MemInfo.Total >= status.MemInfo.Used {
								availableFromUsed = status.MemInfo.Total - status.MemInfo.Used
								guestRaw.MemInfoTotalMinusUsed = availableFromUsed
							}

							missingCacheMetrics := status.MemInfo.Available == 0 &&
								status.MemInfo.Buffers == 0 &&
								status.MemInfo.Cached == 0

							switch {
							case status.MemInfo.Available > 0:
								memAvailable = status.MemInfo.Available
								memorySource = "meminfo-available"
							case status.MemInfo.Free > 0 ||
								status.MemInfo.Buffers > 0 ||
								status.MemInfo.Cached > 0:
								memAvailable = status.MemInfo.Free +
									status.MemInfo.Buffers +
									status.MemInfo.Cached
								memorySource = "meminfo-derived"
							}

							if memAvailable == 0 && availableFromUsed > 0 && missingCacheMetrics {
								const vmTotalMinusUsedGapTolerance uint64 = 4 * 1024 * 1024
								if availableFromUsed > componentAvailable {
									gap := availableFromUsed - componentAvailable
									if componentAvailable == 0 || gap >= vmTotalMinusUsedGapTolerance {
										memAvailable = availableFromUsed
										memorySource = "meminfo-total-minus-used"
									}
								}
							}
						}
						if vmStatus.Balloon > 0 && vmStatus.Balloon < vmStatus.MaxMem {
							memTotal = vmStatus.Balloon
							guestRaw.DerivedFromBall = true
						}
						switch {
						case memAvailable > 0:
							if memAvailable > memTotal {
								memAvailable = memTotal
							}
							memUsed = memTotal - memAvailable
						case vmStatus.FreeMem > 0:
							memUsed = memTotal - vmStatus.FreeMem
							memorySource = "status-freemem"
						case vmStatus.Mem > 0:
							memUsed = vmStatus.Mem
							memorySource = "status-mem"
						default:
							memUsed = 0
							memorySource = "status-unavailable"
						}
						if memUsed > memTotal {
							memUsed = memTotal
						}
						// Use actual disk I/O values from detailed status
						diskReadBytes = int64(vmStatus.DiskRead)
						diskWriteBytes = int64(vmStatus.DiskWrite)
						networkInBytes = int64(vmStatus.NetIn)
						networkOutBytes = int64(vmStatus.NetOut)
					}
					cancel()
				}

				if vm.Status != "running" {
					memorySource = "powered-off"
				} else if vmStatus == nil {
					memorySource = "status-unavailable"
				}

				if vm.Status == "running" && vmStatus != nil {
					guestIPs, guestIfaces, guestOSName, guestOSVersion := m.fetchGuestAgentMetadata(ctx, client, instanceName, n.Node, vm.Name, vm.VMID, vmStatus)
					if len(guestIPs) > 0 {
						ipAddresses = guestIPs
					}
					if len(guestIfaces) > 0 {
						networkInterfaces = guestIfaces
					}
					if guestOSName != "" {
						osName = guestOSName
					}
					if guestOSVersion != "" {
						osVersion = guestOSVersion
					}
				}

				// Calculate I/O rates after we have the actual values
				sampleTime := time.Now()
				currentMetrics := IOMetrics{
					DiskRead:   diskReadBytes,
					DiskWrite:  diskWriteBytes,
					NetworkIn:  networkInBytes,
					NetworkOut: networkOutBytes,
					Timestamp:  sampleTime,
				}
				diskReadRate, diskWriteRate, netInRate, netOutRate := m.rateTracker.CalculateRates(guestID, currentMetrics)

				// Debug log disk I/O rates
				if diskReadRate > 0 || diskWriteRate > 0 {
					log.Debug().
						Str("vm", vm.Name).
						Int("vmid", vm.VMID).
						Float64("diskReadRate", diskReadRate).
						Float64("diskWriteRate", diskWriteRate).
						Int64("diskReadBytes", diskReadBytes).
						Int64("diskWriteBytes", diskWriteBytes).
						Msg("VM disk I/O rates calculated")
				}

				// Set CPU to 0 for non-running VMs
				cpuUsage := safeFloat(vm.CPU)
				if vm.Status != "running" {
					cpuUsage = 0
				}

				// Calculate disk usage - start with allocated disk size
				// NOTE: The Proxmox cluster/resources API always returns 0 for VM disk usage
				// We must query the guest agent to get actual disk usage
				diskUsed := uint64(vm.Disk)
				diskTotal := uint64(vm.MaxDisk)
				diskFree := diskTotal - diskUsed
				diskUsage := safePercentage(float64(diskUsed), float64(diskTotal))
				diskStatusReason := ""
				var individualDisks []models.Disk

				// For stopped VMs, we can't get guest agent data
				if vm.Status != "running" {
					// Show allocated disk size for stopped VMs
					if diskTotal > 0 {
						diskUsage = -1 // Indicates "allocated size only"
						diskStatusReason = "vm-stopped"
					}
				}

				// For running VMs, ALWAYS try to get filesystem info from guest agent
				// The cluster/resources endpoint always returns 0 for disk usage
				if vm.Status == "running" && vmStatus != nil && diskTotal > 0 {
					// Log the initial state
					log.Debug().
						Str("instance", instanceName).
						Str("vm", vm.Name).
						Int("vmid", vm.VMID).
						Int("agent", vmStatus.Agent).
						Uint64("diskUsed", diskUsed).
						Uint64("diskTotal", diskTotal).
						Msg("VM has 0 disk usage, checking guest agent")

					// Check if agent is enabled
					if vmStatus.Agent == 0 {
						diskStatusReason = "agent-disabled"
						log.Debug().
							Str("instance", instanceName).
							Str("vm", vm.Name).
							Msg("Guest agent disabled in VM config")
					} else if vmStatus.Agent > 0 || diskUsed == 0 {
						log.Debug().
							Str("instance", instanceName).
							Str("vm", vm.Name).
							Int("vmid", vm.VMID).
							Msg("Guest agent enabled, fetching filesystem info")

						statusCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
						if fsInfo, err := client.GetVMFSInfo(statusCtx, n.Node, vm.VMID); err != nil {
							// Handle errors
							errStr := err.Error()
							log.Warn().
								Str("instance", instanceName).
								Str("vm", vm.Name).
								Int("vmid", vm.VMID).
								Str("error", errStr).
								Msg("Failed to get VM filesystem info from guest agent")

							if strings.Contains(errStr, "QEMU guest agent is not running") {
								diskStatusReason = "agent-not-running"
								log.Info().
									Str("instance", instanceName).
									Str("vm", vm.Name).
									Int("vmid", vm.VMID).
									Msg("Guest agent enabled in VM config but not running inside guest OS. Install and start qemu-guest-agent in the VM")
							} else if strings.Contains(err.Error(), "timeout") {
								diskStatusReason = "agent-timeout"
							} else if strings.Contains(err.Error(), "permission denied") || strings.Contains(err.Error(), "not allowed") {
								diskStatusReason = "permission-denied"
							} else {
								diskStatusReason = "agent-error"
							}
						} else if len(fsInfo) == 0 {
							diskStatusReason = "no-filesystems"
							log.Warn().
								Str("instance", instanceName).
								Str("vm", vm.Name).
								Int("vmid", vm.VMID).
								Msg("Guest agent returned empty filesystem list")
						} else {
							log.Info().
								Str("instance", instanceName).
								Str("vm", vm.Name).
								Int("vmid", vm.VMID).
								Int("filesystems", len(fsInfo)).
								Msg("Got filesystem info from guest agent")
							// Aggregate disk usage from all filesystems
							// Fix for #425: Track seen devices to avoid counting duplicates
							var totalBytes, usedBytes uint64
							seenDevices := make(map[string]bool)

							for _, fs := range fsInfo {
								// Log each filesystem for debugging
								log.Debug().
									Str("vm", vm.Name).
									Str("mountpoint", fs.Mountpoint).
									Str("type", fs.Type).
									Str("disk", fs.Disk).
									Uint64("total", fs.TotalBytes).
									Uint64("used", fs.UsedBytes).
									Msg("Processing filesystem from guest agent")

								// Skip special filesystems and Windows System Reserved
								// For Windows, mountpoints are like "C:\\" or "D:\\" - don't skip those
								isWindowsDrive := len(fs.Mountpoint) >= 2 && fs.Mountpoint[1] == ':' && strings.Contains(fs.Mountpoint, "\\")

								if !isWindowsDrive {
									if reason, skip := readOnlyFilesystemReason(fs.Type, fs.TotalBytes, fs.UsedBytes); skip {
										log.Debug().
											Str("vm", vm.Name).
											Str("mountpoint", fs.Mountpoint).
											Str("type", fs.Type).
											Str("skipReason", reason).
											Uint64("total", fs.TotalBytes).
											Uint64("used", fs.UsedBytes).
											Msg("Skipping read-only filesystem from guest agent")
										continue
									}

									if fs.Type == "tmpfs" || fs.Type == "devtmpfs" ||
										strings.HasPrefix(fs.Mountpoint, "/dev") ||
										strings.HasPrefix(fs.Mountpoint, "/proc") ||
										strings.HasPrefix(fs.Mountpoint, "/sys") ||
										strings.HasPrefix(fs.Mountpoint, "/run") ||
										fs.Mountpoint == "/boot/efi" ||
										fs.Mountpoint == "System Reserved" ||
										strings.Contains(fs.Mountpoint, "System Reserved") ||
										strings.HasPrefix(fs.Mountpoint, "/snap") { // Skip snap mounts
										log.Debug().
											Str("vm", vm.Name).
											Str("mountpoint", fs.Mountpoint).
											Str("type", fs.Type).
											Msg("Skipping special filesystem")
										continue
									}
								}

								// Skip if we've already seen this device (duplicate mount point)
								if fs.Disk != "" && seenDevices[fs.Disk] {
									log.Debug().
										Str("vm", vm.Name).
										Str("mountpoint", fs.Mountpoint).
										Str("disk", fs.Disk).
										Msg("Skipping duplicate mount of same device")
									continue
								}

								// Only count real filesystems with valid data
								if fs.TotalBytes > 0 {
									// Mark this device as seen
									if fs.Disk != "" {
										seenDevices[fs.Disk] = true
									}

									totalBytes += fs.TotalBytes
									usedBytes += fs.UsedBytes
									individualDisks = append(individualDisks, models.Disk{
										Total:      int64(fs.TotalBytes),
										Used:       int64(fs.UsedBytes),
										Free:       int64(fs.TotalBytes - fs.UsedBytes),
										Usage:      safePercentage(float64(fs.UsedBytes), float64(fs.TotalBytes)),
										Mountpoint: fs.Mountpoint,
										Type:       fs.Type,
										Device:     fs.Disk,
									})
									log.Debug().
										Str("vm", vm.Name).
										Str("mountpoint", fs.Mountpoint).
										Str("disk", fs.Disk).
										Uint64("added_total", fs.TotalBytes).
										Uint64("added_used", fs.UsedBytes).
										Msg("Adding filesystem to total")
								} else {
									log.Debug().
										Str("vm", vm.Name).
										Str("mountpoint", fs.Mountpoint).
										Msg("Skipping filesystem with 0 total bytes")
								}
							}

							// If we got valid data from guest agent, use it
							if totalBytes > 0 {
								diskTotal = totalBytes
								diskUsed = usedBytes
								diskFree = totalBytes - usedBytes
								diskUsage = safePercentage(float64(usedBytes), float64(totalBytes))
								diskStatusReason = "" // Clear reason on success

								log.Info().
									Str("instance", instanceName).
									Str("vm", vm.Name).
									Int("vmid", vm.VMID).
									Uint64("totalBytes", totalBytes).
									Uint64("usedBytes", usedBytes).
									Float64("usage", diskUsage).
									Msg("✓ Successfully retrieved disk usage from guest agent")
							} else {
								// Only special filesystems found - show allocated disk size instead
								diskStatusReason = "special-filesystems-only"
								if diskTotal > 0 {
									diskUsage = -1 // Show as allocated size
								}
								log.Info().
									Str("instance", instanceName).
									Str("vm", vm.Name).
									Int("filesystems_found", len(fsInfo)).
									Msg("Guest agent provided filesystem info but no usable filesystems found (all were special mounts)")
							}
						}
						cancel()
					} else {
						// No vmStatus available or agent disabled - show allocated disk
						if diskTotal > 0 {
							diskUsage = -1 // Show as allocated size
							diskStatusReason = "no-agent"
						}
					}
				} else if vm.Status == "running" && diskTotal > 0 {
					// Running VM but no vmStatus - show allocated disk
					diskUsage = -1
					diskStatusReason = "no-status"
				}

				memTotalBytes := clampToInt64(memTotal)
				memUsedBytes := clampToInt64(memUsed)
				if memTotalBytes > 0 && memUsedBytes > memTotalBytes {
					memUsedBytes = memTotalBytes
				}
				memFreeBytes := memTotalBytes - memUsedBytes
				if memFreeBytes < 0 {
					memFreeBytes = 0
				}
				memory := models.Memory{
					Total: memTotalBytes,
					Used:  memUsedBytes,
					Free:  memFreeBytes,
					Usage: safePercentage(float64(memUsed), float64(memTotal)),
				}
				if guestRaw.Balloon > 0 {
					memory.Balloon = clampToInt64(guestRaw.Balloon)
				}

				// Create VM model
				modelVM := models.VM{
					ID:       guestID,
					VMID:     vm.VMID,
					Name:     vm.Name,
					Node:     n.Node,
					Instance: instanceName,
					Status:   vm.Status,
					Type:     "qemu",
					CPU:      cpuUsage,
					CPUs:     int(vm.CPUs),
					Memory:   memory,
					Disk: models.Disk{
						Total: int64(diskTotal),
						Used:  int64(diskUsed),
						Free:  int64(diskFree),
						Usage: diskUsage,
					},
					Disks:             individualDisks,
					DiskStatusReason:  diskStatusReason,
					NetworkIn:         maxInt64(0, int64(netInRate)),
					NetworkOut:        maxInt64(0, int64(netOutRate)),
					DiskRead:          maxInt64(0, int64(diskReadRate)),
					DiskWrite:         maxInt64(0, int64(diskWriteRate)),
					Uptime:            int64(vm.Uptime),
					Template:          vm.Template == 1,
					LastSeen:          sampleTime,
					Tags:              tags,
					IPAddresses:       ipAddresses,
					OSName:            osName,
					OSVersion:         osVersion,
					NetworkInterfaces: networkInterfaces,
				}

				// Zero out metrics for non-running VMs
				if vm.Status != "running" {
					modelVM.CPU = 0
					modelVM.Memory.Usage = 0
					modelVM.Disk.Usage = 0
					modelVM.NetworkIn = 0
					modelVM.NetworkOut = 0
					modelVM.DiskRead = 0
					modelVM.DiskWrite = 0
				}

				nodeVMs = append(nodeVMs, modelVM)

				m.recordGuestSnapshot(instanceName, modelVM.Type, n.Node, vm.VMID, GuestMemorySnapshot{
					Name:         vm.Name,
					Status:       vm.Status,
					RetrievedAt:  sampleTime,
					MemorySource: memorySource,
					Memory:       modelVM.Memory,
					Raw:          guestRaw,
				})

				// Check alerts
				m.alertManager.CheckGuest(modelVM, instanceName)
			}

			nodeDuration := time.Since(nodeStart)
			log.Debug().
				Str("node", n.Node).
				Int("vms", len(nodeVMs)).
				Dur("duration", nodeDuration).
				Msg("Node VM polling completed")

			resultChan <- nodeResult{node: n.Node, vms: nodeVMs}
		}(node)
	}

	// Close channel when all goroutines complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results from all nodes
	var allVMs []models.VM
	successfulNodes := 0
	failedNodes := 0

	for result := range resultChan {
		if result.err != nil {
			failedNodes++
		} else {
			successfulNodes++
			allVMs = append(allVMs, result.vms...)
		}
	}

	// Update state with all VMs
	m.state.UpdateVMsForInstance(instanceName, allVMs)

	duration := time.Since(startTime)
	log.Info().
		Str("instance", instanceName).
		Int("totalVMs", len(allVMs)).
		Int("successfulNodes", successfulNodes).
		Int("failedNodes", failedNodes).
		Dur("duration", duration).
		Msg("Parallel VM polling completed")
}

// pollContainersWithNodesOptimized polls containers from all nodes in parallel using goroutines
func (m *Monitor) pollContainersWithNodesOptimized(ctx context.Context, instanceName string, client PVEClientInterface, nodes []proxmox.Node) {
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
		if node.Status == "online" {
			onlineNodes++
		}
	}

	log.Info().
		Str("instance", instanceName).
		Int("totalNodes", len(nodes)).
		Int("onlineNodes", onlineNodes).
		Msg("Starting parallel container polling")

	// Launch a goroutine for each online node
	for _, node := range nodes {
		// Skip offline nodes
		if node.Status != "online" {
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

				// Create guest ID
				var guestID string
				if instanceName == n.Node {
					guestID = fmt.Sprintf("%s-%d", n.Node, container.VMID)
				} else {
					guestID = fmt.Sprintf("%s-%s-%d", instanceName, n.Node, container.VMID)
				}

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
					NetworkIn:  maxInt64(0, int64(netInRate)),
					NetworkOut: maxInt64(0, int64(netOutRate)),
					DiskRead:   maxInt64(0, int64(diskReadRate)),
					DiskWrite:  maxInt64(0, int64(diskWriteRate)),
					Uptime:     int64(container.Uptime),
					Template:   container.Template == 1,
					LastSeen:   time.Now(),
					Tags:       tags,
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

	// Update state with all containers
	m.state.UpdateContainersForInstance(instanceName, allContainers)

	duration := time.Since(startTime)
	log.Info().
		Str("instance", instanceName).
		Int("totalContainers", len(allContainers)).
		Int("successfulNodes", successfulNodes).
		Int("failedNodes", failedNodes).
		Dur("duration", duration).
		Msg("Parallel container polling completed")
}

// pollStorageWithNodesOptimized polls storage from all nodes in parallel using goroutines
func (m *Monitor) pollStorageWithNodesOptimized(ctx context.Context, instanceName string, client PVEClientInterface, nodes []proxmox.Node) {
	startTime := time.Now()

	// Get cluster storage configuration first (single call)
	clusterStorages, err := client.GetAllStorage(ctx)
	clusterStorageAvailable := err == nil
	if err != nil {
		log.Warn().Err(err).Str("instance", instanceName).Msg("Failed to get cluster storage config - will continue with node storage only")
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

	log.Info().
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
				// Handle timeout gracefully - unavailable storage (e.g., NFS mounts) can cause this
				if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "deadline exceeded") {
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

				// Determine if shared
				shared := hasClusterConfig && clusterConfig.Shared == 1

				// Create storage model
				modelStorage := models.Storage{
					ID:       storageID,
					Name:     storage.Storage,
					Node:     n.Node,
					Instance: instanceName,
					Type:     storage.Type,
					Status:   "available",
					Total:    int64(storage.Total),
					Used:     int64(storage.Used),
					Free:     int64(storage.Available),
					Usage:    safePercentage(float64(storage.Used), float64(storage.Total)),
					Content:  sortContent(storage.Content),
					Shared:   shared,
					Enabled:  true,
					Active:   true,
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

				// Override with cluster config if available
				if hasClusterConfig {
					modelStorage.Enabled = clusterConfig.Enabled == 1
					modelStorage.Active = clusterConfig.Active == 1
				}

				// Determine status based on active/enabled flags
				if storage.Active == 1 || modelStorage.Active {
					modelStorage.Status = "available"
				} else if modelStorage.Enabled {
					modelStorage.Status = "inactive"
				} else {
					modelStorage.Status = "disabled"
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

	// Check alerts for all storage devices
	for _, storage := range allStorage {
		m.alertManager.CheckStorage(storage)
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
	m.state.UpdateStorageForInstance(instanceName, allStorage)

	// Poll Ceph cluster data after refreshing storage information
	m.pollCephCluster(ctx, instanceName, client, cephDetected)

	duration := time.Since(startTime)

	// Warn if all nodes failed to get storage
	if successfulNodes == 0 && failedNodes > 0 {
		log.Error().
			Str("instance", instanceName).
			Int("failedNodes", failedNodes).
			Msg("All nodes failed to retrieve storage - check Proxmox API permissions for Datastore.Audit on all storage")
	} else {
		log.Info().
			Str("instance", instanceName).
			Int("totalStorage", len(allStorage)).
			Int("successfulNodes", successfulNodes).
			Int("failedNodes", failedNodes).
			Int("preservedStorage", preservedCount).
			Dur("duration", duration).
			Msg("Parallel storage polling completed")
	}
}
