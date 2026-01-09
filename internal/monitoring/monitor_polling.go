package monitoring

import (
	"context"
	stderrors "errors"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/errors"
	"github.com/rcourtman/pulse-go-rewrite/internal/logging"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func (m *Monitor) describeInstancesForScheduler() []InstanceDescriptor {
	total := len(m.pveClients) + len(m.pbsClients) + len(m.pmgClients)
	if total == 0 {
		return nil
	}

	descriptors := make([]InstanceDescriptor, 0, total)

	if len(m.pveClients) > 0 {
		names := make([]string, 0, len(m.pveClients))
		for name := range m.pveClients {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			desc := InstanceDescriptor{
				Name: name,
				Type: InstanceTypePVE,
			}
			if m.scheduler != nil {
				if last, ok := m.scheduler.LastScheduled(InstanceTypePVE, name); ok {
					desc.LastScheduled = last.NextRun
					desc.LastInterval = last.Interval
				}
			}
			if m.stalenessTracker != nil {
				if snap, ok := m.stalenessTracker.snapshot(InstanceTypePVE, name); ok {
					desc.LastSuccess = snap.LastSuccess
					desc.LastFailure = snap.LastError
					desc.Metadata = map[string]any{"changeHash": snap.ChangeHash}
				}
			}
			descriptors = append(descriptors, desc)
		}
	}

	if len(m.pbsClients) > 0 {
		names := make([]string, 0, len(m.pbsClients))
		for name := range m.pbsClients {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			desc := InstanceDescriptor{
				Name: name,
				Type: InstanceTypePBS,
			}
			if m.scheduler != nil {
				if last, ok := m.scheduler.LastScheduled(InstanceTypePBS, name); ok {
					desc.LastScheduled = last.NextRun
					desc.LastInterval = last.Interval
				}
			}
			if m.stalenessTracker != nil {
				if snap, ok := m.stalenessTracker.snapshot(InstanceTypePBS, name); ok {
					desc.LastSuccess = snap.LastSuccess
					desc.LastFailure = snap.LastError
					desc.Metadata = map[string]any{"changeHash": snap.ChangeHash}
				}
			}
			descriptors = append(descriptors, desc)
		}
	}

	if len(m.pmgClients) > 0 {
		names := make([]string, 0, len(m.pmgClients))
		for name := range m.pmgClients {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			desc := InstanceDescriptor{
				Name: name,
				Type: InstanceTypePMG,
			}
			if m.scheduler != nil {
				if last, ok := m.scheduler.LastScheduled(InstanceTypePMG, name); ok {
					desc.LastScheduled = last.NextRun
					desc.LastInterval = last.Interval
				}
			}
			if m.stalenessTracker != nil {
				if snap, ok := m.stalenessTracker.snapshot(InstanceTypePMG, name); ok {
					desc.LastSuccess = snap.LastSuccess
					desc.LastFailure = snap.LastError
					desc.Metadata = map[string]any{"changeHash": snap.ChangeHash}
				}
			}
			descriptors = append(descriptors, desc)
		}
	}

	return descriptors
}

func (m *Monitor) buildScheduledTasks(now time.Time) []ScheduledTask {
	descriptors := m.describeInstancesForScheduler()
	if len(descriptors) == 0 {
		return nil
	}

	queueDepth := 0
	if m.taskQueue != nil {
		queueDepth = m.taskQueue.Size()
	}

	if m.scheduler == nil {
		tasks := make([]ScheduledTask, 0, len(descriptors))
		for _, desc := range descriptors {
			interval := m.baseIntervalForInstanceType(desc.Type)
			if interval <= 0 {
				interval = DefaultSchedulerConfig().BaseInterval
			}
			tasks = append(tasks, ScheduledTask{
				InstanceName: desc.Name,
				InstanceType: desc.Type,
				NextRun:      now,
				Interval:     interval,
			})
		}
		return tasks
	}

	return m.scheduler.BuildPlan(now, descriptors, queueDepth)
}

// convertPoolInfoToModel converts Proxmox ZFS pool info to our model
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
					Total:    int64(storage.Total),
					Used:     int64(storage.Used),
					Free:     int64(storage.Available),
					Usage:    safePercentage(float64(storage.Used), float64(storage.Total)),
					Content:  sortContent(storage.Content),
					Shared:   shared,
					Enabled:  storage.Enabled == 1,
					Active:   storage.Active == 1,
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
	if !pve.DisableCeph {
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
		return nil, err
	}

	return directClient.GetStorage(ctx, nodeName)
}

// pollPVENode polls a single PVE node and returns the result
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
	connectionHost := instanceCfg.Host
	guestURL := instanceCfg.GuestURL
	if instanceCfg.IsCluster && len(instanceCfg.ClusterEndpoints) > 0 {
		hasFingerprint := instanceCfg.Fingerprint != ""
		for _, ep := range instanceCfg.ClusterEndpoints {
			if strings.EqualFold(ep.NodeName, node.Node) {
				if effective := clusterEndpointEffectiveURL(ep, instanceCfg.VerifySSL, hasFingerprint); effective != "" {
					connectionHost = effective
				}
				if ep.GuestURL != "" {
					guestURL = ep.GuestURL
				}
				break
			}
		}
	}

	// Apply grace period for node status to prevent flapping
	// For clustered nodes, use clusterName-nodeName as the ID to deduplicate
	// when the same cluster is registered via multiple entry points
	// (e.g., agent installed with --enable-proxmox on multiple cluster nodes)
	var nodeID string
	if instanceCfg.IsCluster && instanceCfg.ClusterName != "" {
		nodeID = instanceCfg.ClusterName + "-" + node.Node
	} else {
		nodeID = instanceName + "-" + node.Node
	}
	effectiveStatus := node.Status
	now := time.Now()

	m.mu.Lock()
	if strings.ToLower(node.Status) == "online" {
		// Node is online - update last-online timestamp
		m.nodeLastOnline[nodeID] = now
	} else {
		// Node is reported as offline - check grace period
		lastOnline, exists := m.nodeLastOnline[nodeID]
		if exists && now.Sub(lastOnline) < nodeOfflineGracePeriod {
			// Still within grace period - preserve online status
			effectiveStatus = "online"
			log.Debug().
				Str("instance", instanceName).
				Str("node", node.Node).
				Dur("timeSinceOnline", now.Sub(lastOnline)).
				Dur("gracePeriod", nodeOfflineGracePeriod).
				Msg("Node offline but within grace period - preserving online status")
		} else {
			// Grace period expired or never seen online - mark as offline
			if exists {
				log.Info().
					Str("instance", instanceName).
					Str("node", node.Node).
					Dur("timeSinceOnline", now.Sub(lastOnline)).
					Msg("Node offline and grace period expired - marking as offline")
			}
		}
	}
	m.mu.Unlock()

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
						logCtx.Msg("Node memory: using RRD memavailable fallback (excludes reclaimable cache)")
						nodeMemorySource = "rrd-memavailable"
						nodeFallbackReason = "rrd-memavailable"
						nodeSnapshotRaw.FallbackCalculated = true
						nodeSnapshotRaw.ProxmoxMemorySource = "rrd-memavailable"
					} else if nodeInfo.Memory.Available > 0 {
						logCtx.Msg("Node memory: using available field (excludes reclaimable cache)")
						nodeMemorySource = "available-field"
					} else if nodeInfo.Memory.Avail > 0 {
						logCtx.Msg("Node memory: using avail field (excludes reclaimable cache)")
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

	// Collect temperature data via SSH (non-blocking, best effort)
	// Only attempt for online nodes when temperature monitoring is enabled
	// Check per-node setting first, fall back to global setting
	tempMonitoringEnabled := m.config.TemperatureMonitoringEnabled
	if instanceCfg.TemperatureMonitoringEnabled != nil {
		tempMonitoringEnabled = *instanceCfg.TemperatureMonitoringEnabled
	}
	if effectiveStatus == "online" && tempMonitoringEnabled {
		// First, check if there's a matching host agent with temperature data.
		// Host agent temperatures are preferred because they don't require SSH access.
		// Use getHostAgentTemperatureByID with the unique node ID to correctly handle
		// duplicate hostname scenarios (e.g., two "px1" nodes on different IPs).
		hostAgentTemp := m.getHostAgentTemperatureByID(modelNode.ID, node.Node)
		if hostAgentTemp != nil {
			log.Debug().
				Str("node", node.Node).
				Float64("cpuPackage", hostAgentTemp.CPUPackage).
				Float64("cpuMax", hostAgentTemp.CPUMax).
				Int("nvmeCount", len(hostAgentTemp.NVMe)).
				Msg("Using temperature data from host agent")
		}

		// If no host agent temp or we need additional data (SMART), try SSH/proxy collection
		var proxyTemp *models.Temperature
		var err error
		if m.tempCollector != nil {
			// Temperature collection is best-effort - use a short timeout to avoid blocking node polling
			// Use context.Background() so the timeout is truly independent of the parent polling context
			// If SSH is slow or unresponsive, we'll preserve previous temperature data
			tempCtx, tempCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer tempCancel()

			// Determine SSH hostname to use (most robust approach):
			// Prefer the resolved host for this node, with cluster overrides when available.
			sshHost := modelNode.Host
			foundNodeEndpoint := false

			if modelNode.IsClusterMember && instanceCfg.IsCluster {
				// Try to find specific endpoint configuration for this node
				if len(instanceCfg.ClusterEndpoints) > 0 {
					hasFingerprint := instanceCfg.Fingerprint != ""
					for _, ep := range instanceCfg.ClusterEndpoints {
						if strings.EqualFold(ep.NodeName, node.Node) {
							if effective := clusterEndpointEffectiveURL(ep, instanceCfg.VerifySSL, hasFingerprint); effective != "" {
								sshHost = effective
								foundNodeEndpoint = true
							}
							break
						}
					}
				}

				// If no specific endpoint found, fall back to node name
				if !foundNodeEndpoint {
					sshHost = node.Node
					log.Debug().
						Str("node", node.Node).
						Str("instance", instanceCfg.Name).
						Msg("Node endpoint not found in cluster metadata - falling back to node name for temperature collection")
				}
			}

			if strings.TrimSpace(sshHost) == "" {
				sshHost = node.Node
			}

			// Skip SSH/proxy collection if we already have host agent data and no proxy is configured
			// (proxy might provide additional SMART data that host agent doesn't have)
			skipProxyCollection := hostAgentTemp != nil &&
				strings.TrimSpace(instanceCfg.TemperatureProxyURL) == "" &&
				!m.HasSocketTemperatureProxy()

			if !skipProxyCollection {
				// Use HTTP proxy if configured for this instance, otherwise fall back to socket/SSH
				proxyTemp, err = m.tempCollector.CollectTemperatureWithProxy(tempCtx, sshHost, node.Node, instanceCfg.TemperatureProxyURL, instanceCfg.TemperatureProxyToken)
				if err != nil && hostAgentTemp == nil {
					log.Debug().
						Str("node", node.Node).
						Str("sshHost", sshHost).
						Bool("isCluster", modelNode.IsClusterMember).
						Int("endpointCount", len(instanceCfg.ClusterEndpoints)).
						Msg("Temperature collection failed - check SSH access")
				}
			}

			// Debug: log proxy temp details before merge
			if proxyTemp != nil {
				log.Debug().
					Str("node", node.Node).
					Bool("proxyTempAvailable", proxyTemp.Available).
					Bool("proxyHasSMART", proxyTemp.HasSMART).
					Int("proxySMARTCount", len(proxyTemp.SMART)).
					Bool("proxyHasNVMe", proxyTemp.HasNVMe).
					Int("proxyNVMeCount", len(proxyTemp.NVMe)).
					Msg("Proxy temperature data before merge")
			} else {
				log.Debug().
					Str("node", node.Node).
					Msg("Proxy temperature data is nil")
			}
		}

		// Merge host agent and proxy temperatures
		temp := mergeTemperatureData(hostAgentTemp, proxyTemp)

		if temp != nil && temp.Available {
			// Get the current CPU temperature (prefer package, fall back to max)
			currentTemp := temp.CPUPackage
			if currentTemp == 0 && temp.CPUMax > 0 {
				currentTemp = temp.CPUMax
			}

			// Find previous temperature data for this node to preserve min/max
			var prevTemp *models.Temperature
			for _, prevNode := range prevInstanceNodes {
				if prevNode.ID == modelNode.ID && prevNode.Temperature != nil {
					prevTemp = prevNode.Temperature
					break
				}
			}

			// Initialize or update min/max tracking
			if prevTemp != nil && prevTemp.CPUMin > 0 {
				// Preserve existing min/max and update if necessary
				temp.CPUMin = prevTemp.CPUMin
				temp.CPUMaxRecord = prevTemp.CPUMaxRecord
				temp.MinRecorded = prevTemp.MinRecorded
				temp.MaxRecorded = prevTemp.MaxRecorded

				// Update min if current is lower
				if currentTemp > 0 && currentTemp < temp.CPUMin {
					temp.CPUMin = currentTemp
					temp.MinRecorded = time.Now()
				}

				// Update max if current is higher
				if currentTemp > temp.CPUMaxRecord {
					temp.CPUMaxRecord = currentTemp
					temp.MaxRecorded = time.Now()
				}
			} else if currentTemp > 0 {
				// First reading - initialize min/max to current value
				temp.CPUMin = currentTemp
				temp.CPUMaxRecord = currentTemp
				temp.MinRecorded = time.Now()
				temp.MaxRecorded = time.Now()
			}

			modelNode.Temperature = temp

			// Determine source for logging
			tempSource := "proxy/ssh"
			if hostAgentTemp != nil && proxyTemp == nil {
				tempSource = "host-agent"
			} else if hostAgentTemp != nil && proxyTemp != nil {
				tempSource = "host-agent+proxy"
			}

			log.Debug().
				Str("node", node.Node).
				Str("source", tempSource).
				Float64("cpuPackage", temp.CPUPackage).
				Float64("cpuMax", temp.CPUMax).
				Float64("cpuMin", temp.CPUMin).
				Float64("cpuMaxRecord", temp.CPUMaxRecord).
				Int("nvmeCount", len(temp.NVMe)).
				Msg("Collected temperature data")
		} else {
			// Temperature data returned but not available (temp != nil && !temp.Available)
			// OR no temperature data from any source - preserve previous temperature if available
			// This prevents the temperature column from flickering when collection temporarily fails
			var prevTemp *models.Temperature
			for _, prevNode := range prevInstanceNodes {
				if prevNode.ID == modelNode.ID && prevNode.Temperature != nil && prevNode.Temperature.Available {
					prevTemp = prevNode.Temperature
					break
				}
			}

			if prevTemp != nil {
				// Clone the previous temperature to avoid modifying historical data
				preserved := *prevTemp
				preserved.LastUpdate = prevTemp.LastUpdate // Keep original update time to indicate staleness
				modelNode.Temperature = &preserved
				log.Debug().
					Str("node", node.Node).
					Bool("isCluster", modelNode.IsClusterMember).
					Float64("cpuPackage", preserved.CPUPackage).
					Time("lastUpdate", preserved.LastUpdate).
					Msg("Preserved previous temperature data (current collection failed or unavailable)")
			} else {
				log.Debug().
					Str("node", node.Node).
					Bool("isCluster", modelNode.IsClusterMember).
					Msg("No temperature data available (collection failed, no previous data to preserve)")
			}
		}
	}

	if m.pollMetrics != nil {
		nodeNameLabel := strings.TrimSpace(node.Node)
		if nodeNameLabel == "" {
			nodeNameLabel = strings.TrimSpace(modelNode.DisplayName)
		}
		if nodeNameLabel == "" {
			nodeNameLabel = "unknown-node"
		}

		success := true
		nodeErrReason := ""
		health := strings.ToLower(strings.TrimSpace(modelNode.ConnectionHealth))
		if health != "" && health != "healthy" {
			success = false
			nodeErrReason = fmt.Sprintf("connection health %s", health)
		}

		status := strings.ToLower(strings.TrimSpace(modelNode.Status))
		if success && status != "" && status != "online" {
			success = false
			nodeErrReason = fmt.Sprintf("status %s", status)
		}

		var nodeErr error
		if !success {
			if nodeErrReason == "" {
				nodeErrReason = "unknown node error"
			}
			nodeErr = stderrors.New(nodeErrReason)
		}

		m.pollMetrics.RecordNodeResult(NodePollResult{
			InstanceName: instanceName,
			InstanceType: "pve",
			NodeName:     nodeNameLabel,
			Success:      success,
			Error:        nodeErr,
			StartTime:    nodeStart,
			EndTime:      time.Now(),
		})
	}

	return modelNode, effectiveStatus, nil
}
