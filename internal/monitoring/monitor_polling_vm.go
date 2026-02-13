package monitoring

import (
	"context"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/logging"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring/errors"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func (m *Monitor) pollVMsWithNodes(ctx context.Context, instanceName string, clusterName string, isCluster bool, client PVEClientInterface, nodes []proxmox.Node, nodeEffectiveStatus map[string]string) {
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
		if nodeEffectiveStatus[node.Node] == "online" {
			onlineNodes++
		}
	}

	log.Debug().
		Str("instance", instanceName).
		Int("totalNodes", len(nodes)).
		Int("onlineNodes", onlineNodes).
		Msg("Starting parallel VM polling")

	// Launch a goroutine for each online node
	for _, node := range nodes {
		// Skip offline nodes
		if nodeEffectiveStatus[node.Node] != "online" {
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
				log.Error().Err(monErr).Str("node", n.Node).Msg("Failed to get VMs; deferring node poll until next cycle")
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

				// Generate canonical guest ID: instance:node:vmid
				guestID := makeGuestID(instanceName, n.Node, vm.VMID)

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
				var osName, osVersion, guestAgentVersion string

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
						guestRaw.Agent = status.Agent.Value
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
						// Note: We intentionally do NOT override memTotal with balloon.
						// The balloon value is tracked separately in memory.balloon for
						// visualization purposes. Using balloon as total causes user
						// confusion (showing 1GB/1GB at 100% when VM is configured for 4GB)
						// and makes the frontend's balloon marker logic ineffective.
						// Refs: #1070
						switch {
						case memAvailable > 0:
							if memAvailable > memTotal {
								memAvailable = memTotal
							}
							memUsed = memTotal - memAvailable
						case vmStatus.Mem > 0:
							// Prefer Mem over FreeMem: Proxmox calculates Mem as
							// (total_mem - free_mem) using the balloon's guest-visible
							// total, which is correct even when ballooning is active.
							// FreeMem is relative to the balloon allocation (not MaxMem),
							// so subtracting it from MaxMem produces wildly inflated
							// usage when the balloon has reduced the VM's memory.
							// Refs: #1185
							memUsed = vmStatus.Mem
							memorySource = "status-mem"
						case vmStatus.FreeMem > 0:
							memUsed = memTotal - vmStatus.FreeMem
							memorySource = "status-freemem"
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
					guestIPs, guestIfaces, guestOSName, guestOSVersion, agentVersion := m.fetchGuestAgentMetadata(ctx, client, instanceName, n.Node, vm.Name, vm.VMID, vmStatus)
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
					if agentVersion != "" {
						guestAgentVersion = agentVersion
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
				diskUsed := vm.Disk
				diskTotal := vm.MaxDisk
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
					if logging.IsLevelEnabled(zerolog.DebugLevel) {
						log.Debug().
							Str("instance", instanceName).
							Str("vm", vm.Name).
							Int("vmid", vm.VMID).
							Int("agent", vmStatus.Agent.Value).
							Uint64("diskUsed", diskUsed).
							Uint64("diskTotal", diskTotal).
							Msg("VM has 0 disk usage, checking guest agent")
					}

					// Check if agent is enabled
					if vmStatus.Agent.Value == 0 {
						diskStatusReason = "agent-disabled"
						if logging.IsLevelEnabled(zerolog.DebugLevel) {
							log.Debug().
								Str("instance", instanceName).
								Str("vm", vm.Name).
								Msg("Guest agent disabled in VM config")
						}
					} else if vmStatus.Agent.Value > 0 || diskUsed == 0 {
						if logging.IsLevelEnabled(zerolog.DebugLevel) {
							log.Debug().
								Str("instance", instanceName).
								Str("vm", vm.Name).
								Int("vmid", vm.VMID).
								Msg("Guest agent enabled, fetching filesystem info")
						}

						// Filesystem info with configurable timeout and retry (refs #592)
						fsInfoRaw, err := m.retryGuestAgentCall(ctx, m.guestAgentFSInfoTimeout, m.guestAgentRetries, func(ctx context.Context) (interface{}, error) {
							return client.GetVMFSInfo(ctx, n.Node, vm.VMID)
						})
						var fsInfo []proxmox.VMFileSystem
						if err == nil {
							if fs, ok := fsInfoRaw.([]proxmox.VMFileSystem); ok {
								fsInfo = fs
							}
						}
						if err != nil {
							// Handle errors
							errStr := err.Error()
							errStrLower := strings.ToLower(errStr)
							log.Warn().
								Str("instance", instanceName).
								Str("vm", vm.Name).
								Int("vmid", vm.VMID).
								Str("error", errStr).
								Msg("Failed to get VM filesystem info from guest agent")

							// Classify the error type for better user messaging
							// Order matters: check most specific patterns first
							if strings.Contains(errStr, "QEMU guest agent is not running") {
								diskStatusReason = "agent-not-running"
								log.Info().
									Str("instance", instanceName).
									Str("vm", vm.Name).
									Int("vmid", vm.VMID).
									Msg("Guest agent enabled in VM config but not running inside guest OS. Install and start qemu-guest-agent in the VM")
							} else if strings.Contains(errStr, "timeout") {
								diskStatusReason = "agent-timeout"
							} else if strings.Contains(errStr, "500") && (strings.Contains(errStr, "not running") || strings.Contains(errStr, "not available")) {
								// Proxmox API error 500 with "not running"/"not available" indicates guest agent issue, not permissions
								// This commonly happens when guest agent is not installed or not running
								diskStatusReason = "agent-not-running"
								log.Info().
									Str("instance", instanceName).
									Str("vm", vm.Name).
									Int("vmid", vm.VMID).
									Msg("Guest agent communication failed (API error 500). Install and start qemu-guest-agent in the VM")
							} else if (strings.Contains(errStr, "403") || strings.Contains(errStr, "401")) &&
								(strings.Contains(errStrLower, "permission") || strings.Contains(errStrLower, "forbidden") || strings.Contains(errStrLower, "not allowed")) {
								// Only treat as permission-denied if we get explicit auth/permission error codes (401/403)
								// This distinguishes actual permission issues from guest agent unavailability
								diskStatusReason = "permission-denied"
								log.Warn().
									Str("instance", instanceName).
									Str("vm", vm.Name).
									Int("vmid", vm.VMID).
									Msg("Permission denied accessing guest agent. Verify Pulse user has VM.Monitor (PVE 8) or VM.Audit+VM.GuestAgent.Audit (PVE 9) permissions")
							} else if strings.Contains(errStr, "500") {
								// Generic 500 error without clear indicators - likely agent unavailable
								// Refs #596: Proxmox returns 500 errors when guest agent isn't installed/running
								diskStatusReason = "agent-not-running"
								log.Info().
									Str("instance", instanceName).
									Str("vm", vm.Name).
									Int("vmid", vm.VMID).
									Msg("Failed to communicate with guest agent (API error 500). This usually means qemu-guest-agent is not installed or not running in the VM")
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
					CPUs:     vm.CPUs,
					Memory:   memory,
					Disk: models.Disk{
						Total: int64(diskTotal),
						Used:  int64(diskUsed),
						Free:  int64(diskFree),
						Usage: diskUsage,
					},
					Disks:             individualDisks,
					DiskStatusReason:  diskStatusReason,
					NetworkIn:         max(0, int64(netInRate)),
					NetworkOut:        max(0, int64(netOutRate)),
					DiskRead:          max(0, int64(diskReadRate)),
					DiskWrite:         max(0, int64(diskWriteRate)),
					Uptime:            int64(vm.Uptime),
					Template:          vm.Template == 1,
					LastSeen:          sampleTime,
					Tags:              tags,
					IPAddresses:       ipAddresses,
					OSName:            osName,
					OSVersion:         osVersion,
					AgentVersion:      guestAgentVersion,
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

				// Trigger guest metadata migration if old format exists
				if m.guestMetadataStore != nil {
					m.guestMetadataStore.GetWithLegacyMigration(guestID, instanceName, n.Node, vm.VMID)
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

	// If we got ZERO VMs but had VMs before (likely cluster health issue),
	// preserve previous VMs instead of clearing them
	if len(allVMs) == 0 && len(nodes) > 0 {
		prevState := m.GetState()
		prevVMCount := 0
		for _, vm := range prevState.VMs {
			if vm.Instance == instanceName {
				allVMs = append(allVMs, vm)
				prevVMCount++
			}
		}
		if prevVMCount > 0 {
			log.Warn().
				Str("instance", instanceName).
				Int("prevVMs", prevVMCount).
				Int("successfulNodes", successfulNodes).
				Int("totalNodes", len(nodes)).
				Msg("Traditional polling returned zero VMs but had VMs before - preserving previous VMs")
		}
	}

	// Update state with all VMs
	m.state.UpdateVMsForInstance(instanceName, allVMs)

	// Record guest metrics history for running VMs (enables sparkline/trends view)
	now := time.Now()
	for _, vm := range allVMs {
		if vm.Status == "running" {
			m.metricsHistory.AddGuestMetric(vm.ID, "cpu", vm.CPU*100, now)
			m.metricsHistory.AddGuestMetric(vm.ID, "memory", vm.Memory.Usage, now)
			if vm.Disk.Usage >= 0 {
				m.metricsHistory.AddGuestMetric(vm.ID, "disk", vm.Disk.Usage, now)
			}
			// Also write to persistent store
			if m.metricsStore != nil {
				m.metricsStore.Write("vm", vm.ID, "cpu", vm.CPU*100, now)
				m.metricsStore.Write("vm", vm.ID, "memory", vm.Memory.Usage, now)
				if vm.Disk.Usage >= 0 {
					m.metricsStore.Write("vm", vm.ID, "disk", vm.Disk.Usage, now)
				}
			}
		}
	}

	duration := time.Since(startTime)
	log.Debug().
		Str("instance", instanceName).
		Int("totalVMs", len(allVMs)).
		Int("successfulNodes", successfulNodes).
		Int("failedNodes", failedNodes).
		Dur("duration", duration).
		Msg("Parallel VM polling completed")
}

// pollContainersWithNodes polls containers from all nodes in parallel using goroutines
// When the instance is part of a cluster, the cluster name is used for guest IDs to prevent duplicates
// when multiple cluster nodes are configured as separate PVE instances.
