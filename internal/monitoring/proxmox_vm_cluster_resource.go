package monitoring

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/logging"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/fsfilters"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type indexedClusterResource struct {
	order    int
	resource proxmox.ClusterResource
}

type efficientQEMUPollResult struct {
	order    int
	vm       models.VM
	alertVM  models.VM
	snapshot GuestMemorySnapshot
	ok       bool
}

func (m *Monitor) nextGuestAgentPollOffset(instanceName string, count int) int {
	if m == nil || count <= 1 {
		return 0
	}

	m.guestAgentPollOrderMu.Lock()
	defer m.guestAgentPollOrderMu.Unlock()

	if m.guestAgentPollCursor == nil {
		m.guestAgentPollCursor = make(map[string]int)
	}

	offset := m.guestAgentPollCursor[instanceName] % count
	m.guestAgentPollCursor[instanceName] = (offset + 1) % count

	return offset
}

func rotateIndexedClusterResources(resources []indexedClusterResource, offset int) []indexedClusterResource {
	count := len(resources)
	if count <= 1 {
		return append([]indexedClusterResource(nil), resources...)
	}

	offset %= count
	if offset < 0 {
		offset += count
	}
	if offset == 0 {
		return append([]indexedClusterResource(nil), resources...)
	}

	rotated := make([]indexedClusterResource, 0, count)
	rotated = append(rotated, resources[offset:]...)
	rotated = append(rotated, resources[:offset]...)
	return rotated
}

func (m *Monitor) efficientQEMUWorkerCount(total int) int {
	if total <= 0 {
		return 0
	}
	if m == nil || m.guestAgentWorkSlots == nil {
		return total
	}

	workers := cap(m.guestAgentWorkSlots)
	if workers <= 0 {
		return total
	}
	if workers > total {
		return total
	}
	return workers
}

func (m *Monitor) pollEfficientQEMUResource(
	ctx context.Context,
	instanceName string,
	res proxmox.ClusterResource,
	client PVEClientInterface,
	vmIDToHostAgent map[string]models.Host,
	prevDiskByGuestID map[string]models.Disk,
	prevVMByGuestID map[string]models.VM,
) (models.VM, models.VM, GuestMemorySnapshot, bool) {
	guestID := makeGuestID(instanceName, res.Node, res.VMID)

	diskReadBytes := int64(res.DiskRead)
	diskWriteBytes := int64(res.DiskWrite)
	networkInBytes := int64(res.NetIn)
	networkOutBytes := int64(res.NetOut)
	var individualDisks []models.Disk
	diskFromAgent := false
	diskStatusReason := ""
	var ipAddresses []string
	var networkInterfaces []models.GuestNetworkInterface
	var osName, osVersion, agentVersion string
	var prevVM *models.VM
	if prev, ok := prevVMByGuestID[guestID]; ok {
		prevVM = &prev
	}
	guestAgentAvailable := false

	memTotal := res.MaxMem
	memUsed := res.Mem
	memorySource := "cluster-resources"
	guestRaw := VMMemoryRaw{
		ListingMem:    res.Mem,
		ListingMaxMem: res.MaxMem,
	}
	var detailedStatus *proxmox.VMStatus
	memAvailable := uint64(0)
	memRawFree := uint64(0)
	memInfoTotalMinusUsed := uint64(0)
	rrdUsed := uint64(0)

	diskUsed := res.Disk
	diskTotal := res.MaxDisk
	diskFree := diskTotal - diskUsed
	diskUsage := safePercentage(float64(diskUsed), float64(diskTotal))
	if diskUsed == 0 && diskTotal > 0 && res.Status == "running" {
		diskUsage = -1
		diskStatusReason = "no-data"
	}
	if res.Status != "running" && diskTotal > 0 {
		diskUsage = -1
		diskStatusReason = "vm-stopped"
	}

	if res.Status == "running" {
		statusCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		status, err := client.GetVMStatus(statusCtx, res.Node, res.VMID)
		cancel()
		if err != nil {
			log.Debug().
				Err(err).
				Str("instance", instanceName).
				Str("vm", res.Name).
				Int("vmid", res.VMID).
				Msg("Could not get VM status to check guest agent availability")
		} else if status != nil {
			detailedStatus = status
			guestRaw.StatusMaxMem = detailedStatus.MaxMem
			guestRaw.StatusMem = detailedStatus.Mem
			guestRaw.StatusFreeMem = detailedStatus.FreeMem
			guestRaw.Balloon = detailedStatus.Balloon
			guestRaw.BalloonMin = detailedStatus.BalloonMin
			guestRaw.Agent = detailedStatus.Agent.Value
			if detailedStatus.MemInfo != nil {
				guestRaw.MemInfoUsed = detailedStatus.MemInfo.Used
				guestRaw.MemInfoFree = detailedStatus.MemInfo.Free
				guestRaw.MemInfoTotal = detailedStatus.MemInfo.Total
				guestRaw.MemInfoAvailable = detailedStatus.MemInfo.Available
				guestRaw.MemInfoBuffers = detailedStatus.MemInfo.Buffers
				guestRaw.MemInfoCached = detailedStatus.MemInfo.Cached
				guestRaw.MemInfoShared = detailedStatus.MemInfo.Shared

				if detailedStatus.MemInfo.Free > 0 {
					memRawFree = detailedStatus.MemInfo.Free
				}

				selection := selectVMAvailableFromMemInfo(detailedStatus.MemInfo)
				memInfoTotalMinusUsed = selection.TotalMinusUsed
				guestRaw.MemInfoTotalMinusUsed = memInfoTotalMinusUsed
				if selection.Available > 0 {
					memAvailable = selection.Available
					memorySource = selection.Source
				}
			}

			diskReadBytes = int64(detailedStatus.DiskRead)
			diskWriteBytes = int64(detailedStatus.DiskWrite)
			networkInBytes = int64(detailedStatus.NetIn)
			networkOutBytes = int64(detailedStatus.NetOut)
			if detailedStatus.MaxMem > 0 {
				memTotal = detailedStatus.MaxMem
			}
		} else {
			log.Debug().
				Str("instance", instanceName).
				Str("vm", res.Name).
				Int("vmid", res.VMID).
				Msg("Could not get VM status, using cluster/resources disk data")
			if diskTotal > 0 {
				diskUsage = -1
				diskStatusReason = "no-status"
			}
		}
	}
	guestAgentAvailable = res.Status == "running" && shouldQueryGuestAgent(detailedStatus, prevVM, time.Now())

	if guestAgentAvailable && memAvailable == 0 {
		m.runGuestAgentVMWork(ctx, instanceName, res.Node, res.Name, res.VMID, func(agentCtx context.Context) {
			if agentAvailable, ok := m.tryVMAgentMemAvailable(agentCtx, client, instanceName, res.Node, res.Name, res.VMID, memTotal, &guestRaw); ok {
				memAvailable = agentAvailable
				memorySource = "guest-agent-meminfo"
			}
		})
	}

	if res.Status == "running" && memAvailable == 0 {
		if rrdEntry, rrdErr := m.getVMRRDMetrics(ctx, client, instanceName, res.Node, res.VMID); rrdErr == nil {
			if rrdEntry.total > 0 {
				memTotal = rrdEntry.total
			}
			if rrdEntry.available > 0 {
				memAvailable = rrdEntry.available
				memorySource = "rrd-memavailable"
				guestRaw.RRDAvailable = rrdEntry.available
				guestRaw.MemInfoAvailable = rrdEntry.available
			} else if rrdEntry.used > 0 {
				rrdUsed = rrdEntry.used
				memorySource = "rrd-memused"
				guestRaw.RRDUsed = rrdEntry.used
			}
		} else {
			log.Debug().
				Err(rrdErr).
				Str("instance", instanceName).
				Str("vm", res.Name).
				Int("vmid", res.VMID).
				Msg("RRD memory data unavailable for VM, using status/cluster resources values")
		}
	}

	if res.Status == "running" && memAvailable == 0 {
		if agentHost, ok := vmIDToHostAgent[guestID]; ok && agentHost.Memory.Total > 0 {
			agentAvailable := agentHost.Memory.Total - agentHost.Memory.Used
			if agentAvailable > 0 {
				memAvailable = uint64(agentAvailable)
				memorySource = "host-agent"
				guestRaw.HostAgentTotal = uint64(agentHost.Memory.Total)
				guestRaw.HostAgentUsed = uint64(agentHost.Memory.Used)
			}
		}
	}

	if guestAgentAvailable {
		m.runGuestAgentFSInfoWork(ctx, instanceName, res.Node, res.Name, res.VMID, func(agentCtx context.Context) {
			fsInfoRaw, err := m.retryGuestAgentCall(agentCtx, m.guestAgentFSInfoTimeout, m.guestAgentRetries, func(ctx context.Context) (interface{}, error) {
				return client.GetVMFSInfo(ctx, res.Node, res.VMID)
			})
			var fsInfo []proxmox.VMFileSystem
			if err == nil {
				if fs, ok := fsInfoRaw.([]proxmox.VMFileSystem); ok {
					fsInfo = fs
				}
			}
			if err != nil {
				diskStatusReason = classifyGuestAgentDiskStatusError(err)
				if diskTotal > 0 {
					diskUsage = -1
				}
				errMsg := err.Error()
				switch {
				case strings.Contains(errMsg, "500") || strings.Contains(errMsg, "QEMU guest agent is not running"):
					log.Info().Str("instance", instanceName).Str("vm", res.Name).Int("vmid", res.VMID).Msg("Guest agent enabled in VM config but not running inside guest OS. Install and start qemu-guest-agent in the VM")
				case strings.Contains(errMsg, "timeout") || strings.Contains(errMsg, "deadline exceeded"):
					log.Info().Str("instance", instanceName).Str("vm", res.Name).Int("vmid", res.VMID).Msg("Guest agent timeout - agent may be installed but not responding")
				case strings.Contains(errMsg, "403") || strings.Contains(errMsg, "401") || strings.Contains(errMsg, "authentication error"):
					log.Info().Str("instance", instanceName).Str("vm", res.Name).Int("vmid", res.VMID).Msg("VM disk monitoring permission denied. Check guest-agent permissions for this node")
				default:
					log.Debug().Err(err).Str("instance", instanceName).Str("vm", res.Name).Int("vmid", res.VMID).Msg("Failed to get filesystem info from guest agent")
				}
				return
			}
			if len(fsInfo) == 0 {
				diskStatusReason = "no-filesystems"
				if diskTotal > 0 {
					diskUsage = -1
				}
				log.Info().Str("instance", instanceName).Str("vm", res.Name).Int("vmid", res.VMID).Msg("Guest agent returned no filesystem info - agent may need restart or VM may have no mounted filesystems")
				return
			}

			var totalBytes, usedBytes uint64
			var skippedFS []string
			var includedFS []string
			seenFilesystems := make(map[string]bool)

			for _, fs := range fsInfo {
				shouldSkip, reasons := fsfilters.ShouldSkipFilesystem(fs.Type, fs.Mountpoint, fs.TotalBytes, fs.UsedBytes)
				if shouldSkip {
					skippedFS = append(skippedFS, fmt.Sprintf("%s(%s,%s)", fs.Mountpoint, fs.Type, strings.Join(reasons, ",")))
					continue
				}
				if reason, skip := readOnlyFilesystemReason(fs.Type, fs.TotalBytes, fs.UsedBytes); skip {
					skippedFS = append(skippedFS, fmt.Sprintf("%s(%s,%s)", fs.Mountpoint, fs.Type, reason))
					continue
				}
				if fs.TotalBytes <= 0 {
					if len(fs.Mountpoint) > 0 {
						skippedFS = append(skippedFS, fmt.Sprintf("%s(%s,0GB)", fs.Mountpoint, fs.Type))
					}
					continue
				}

				fsTypeLower := strings.ToLower(fs.Type)
				countThisFS := true
				if fs.Disk != "" {
					dedupeKey := fmt.Sprintf("%s:%d", fs.Disk, fs.TotalBytes)
					if seenFilesystems[dedupeKey] {
						countThisFS = false
					} else {
						seenFilesystems[dedupeKey] = true
					}
				} else if fsTypeLower == "btrfs" || fsTypeLower == "zfs" || strings.HasPrefix(fsTypeLower, "zfs") {
					dedupeKey := fmt.Sprintf("%s::%d", fsTypeLower, fs.TotalBytes)
					if seenFilesystems[dedupeKey] {
						countThisFS = false
					} else {
						seenFilesystems[dedupeKey] = true
					}
				}

				if countThisFS {
					totalBytes += fs.TotalBytes
					usedBytes += fs.UsedBytes
				}
				includedFS = append(includedFS, fmt.Sprintf("%s(%s,%.1fGB)", fs.Mountpoint, fs.Type, float64(fs.TotalBytes)/1073741824))
				individualDisks = append(individualDisks, models.Disk{
					Total:      int64(fs.TotalBytes),
					Used:       int64(fs.UsedBytes),
					Free:       int64(fs.TotalBytes - fs.UsedBytes),
					Usage:      safePercentage(float64(fs.UsedBytes), float64(fs.TotalBytes)),
					Mountpoint: fs.Mountpoint,
					Type:       fs.Type,
					Device:     fs.Disk,
				})
			}

			if len(skippedFS) > 0 {
				log.Debug().Str("instance", instanceName).Str("vm", res.Name).Strs("skipped", skippedFS).Msg("Skipped special filesystems")
			}
			if len(includedFS) > 0 {
				log.Debug().Str("instance", instanceName).Str("vm", res.Name).Int("vmid", res.VMID).Strs("included", includedFS).Msg("Filesystems included in disk calculation")
			}

			if totalBytes > 0 {
				diskTotal = totalBytes
				diskUsed = usedBytes
				diskFree = totalBytes - usedBytes
				diskUsage = safePercentage(float64(usedBytes), float64(totalBytes))
				diskFromAgent = true
				diskStatusReason = ""
			} else if diskTotal > 0 {
				diskUsage = -1
				diskStatusReason = "special-filesystems-only"
			}
		})
	} else if res.Status == "running" && diskTotal > 0 {
		diskUsage = -1
		if detailedStatus == nil {
			diskStatusReason = "no-status"
		} else {
			diskStatusReason = "agent-disabled"
		}
	}

	if res.Status == "running" {
		m.runGuestAgentVMWork(ctx, instanceName, res.Node, res.Name, res.VMID, func(agentCtx context.Context) {
			guestIPs, guestIfaces, guestOSName, guestOSVersion, guestAgentVersion := m.fetchGuestAgentMetadata(agentCtx, client, instanceName, res.Node, res.Name, res.VMID, detailedStatus, guestAgentAvailable)
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
			if guestAgentVersion != "" {
				agentVersion = guestAgentVersion
			}
		})
	}

	if res.Status == "running" {
		if hostDisk, hostDisks, ok := resolveGuestDiskFromLinkedHostAgent(guestID, vmIDToHostAgent); ok && hostDisk.Total > 0 {
			diskTotal = uint64(hostDisk.Total)
			diskUsed = uint64(hostDisk.Used)
			diskFree = uint64(hostDisk.Free)
			diskUsage = hostDisk.Usage
			individualDisks = hostDisks
			diskFromAgent = true
			diskStatusReason = ""
			log.Debug().
				Str("instance", instanceName).
				Str("vm", res.Name).
				Int("vmid", res.VMID).
				Float64("usage", hostDisk.Usage).
				Msg("QEMU disk: using linked Pulse host agent disk inventory")
		}
	}

	if res.Status == "running" && !diskFromAgent && shouldCarryForwardQEMUDisk(diskStatusReason) {
		if prev, ok := prevDiskByGuestID[guestID]; ok && prev.Usage > 0 && prev.Total > 0 && prev.Used >= 0 && prev.Used <= prev.Total {
			diskTotal = uint64(prev.Total)
			diskUsed = uint64(prev.Used)
			diskFree = diskTotal - diskUsed
			diskUsage = prev.Usage
			if prevVM, ok := prevVMByGuestID[guestID]; ok {
				individualDisks = cloneGuestDisks(prevVM.Disks)
			}
			diskStatusReason = "prev-" + diskStatusReason
			log.Debug().
				Str("instance", instanceName).
				Str("vm", res.Name).
				Int("vmid", res.VMID).
				Float64("prevUsage", prev.Usage).
				Msg("Guest agent disk query failed; carrying forward previous disk data")
		}
	}

	if res.Status == "running" && memAvailable == 0 && memInfoTotalMinusUsed > 0 {
		memAvailable = memInfoTotalMinusUsed
		memorySource = "meminfo-total-minus-used"
	}

	switch {
	case res.Status != "running":
		memorySource = "powered-off"
		memUsed = 0
	case memAvailable > 0:
		if memAvailable > memTotal {
			memAvailable = memTotal
		}
		memUsed = memTotal - memAvailable
	case rrdUsed > 0:
		memUsed = rrdUsed
		memorySource = "rrd-memused"
	case detailedStatus != nil:
		if selection := selectVMLowTrustUsedMemory(memTotal, detailedStatus); selection.Source != "" {
			memUsed = selection.Used
			memorySource = selection.Source
		} else {
			memorySource = "status-unavailable"
		}
	}
	if memUsed > memTotal {
		memUsed = memTotal
	}

	sampleTime := time.Now()
	var memory models.Memory
	snapshotNotes := []string(nil)
	guestAgentHealthy := guestAgentSignalsHealthy(detailedStatus, diskFromAgent, ipAddresses, networkInterfaces, osName, osVersion, agentVersion)
	if prev, ok := prevVMByGuestID[guestID]; ok {
		switch {
		case shouldCarryForwardHealthyGuestLowTrustVMMemory(prev, res.Status, memorySource, memTotal, memUsed, sampleTime, guestAgentHealthy):
			fallbackSource := memorySource
			memory = prev.Memory
			if detailedStatus != nil && detailedStatus.Balloon > 0 {
				memory.Balloon = int64(detailedStatus.Balloon)
			}
			memorySource = "previous-snapshot"
			snapshotNotes = append(snapshotNotes, "preserved-previous-memory-for-healthy-guest-low-trust-full-usage")
			log.Debug().
				Str("instance", instanceName).
				Str("vm", res.Name).
				Int("vmid", res.VMID).
				Str("previousSource", prev.MemorySource).
				Str("fallbackSource", fallbackSource).
				Float64("previousUsage", prev.Memory.Usage).
				Float64("fallbackUsage", safePercentage(float64(memUsed), float64(memTotal))).
				Msg("Preserving previous VM memory metrics for guest with healthy agent signals after low-trust full-usage fallback")
		case shouldCarryForwardPreviousVMMemory(prev, res.Status, memorySource, memTotal, memUsed, sampleTime):
			fallbackSource := memorySource
			memory = prev.Memory
			if detailedStatus != nil && detailedStatus.Balloon > 0 {
				memory.Balloon = int64(detailedStatus.Balloon)
			}
			memorySource = "previous-snapshot"
			snapshotNotes = append(snapshotNotes, "preserved-previous-memory-after-low-trust-fallback")
			log.Debug().
				Str("instance", instanceName).
				Str("vm", res.Name).
				Int("vmid", res.VMID).
				Str("previousSource", prev.MemorySource).
				Str("fallbackSource", fallbackSource).
				Float64("previousUsage", prev.Memory.Usage).
				Float64("fallbackUsage", safePercentage(float64(memUsed), float64(memTotal))).
				Msg("Preserving previous VM memory metrics after low-trust fallback")
		}
	}
	if memory.Total == 0 {
		memFree := uint64(0)
		if memTotal >= memUsed {
			memFree = memTotal - memUsed
		}
		memory = models.Memory{
			Total: int64(memTotal),
			Used:  int64(memUsed),
			Free:  int64(memFree),
			Usage: safePercentage(float64(memUsed), float64(memTotal)),
		}
		if memory.Free < 0 {
			memory.Free = 0
		}
		if memory.Used > memory.Total {
			memory.Used = memory.Total
		}
		if memRawFree > 0 && memFree > memRawFree {
			memory.Cache = int64(memFree - memRawFree)
			memory.Free = int64(memRawFree)
		}
		if detailedStatus != nil && detailedStatus.Balloon > 0 {
			memory.Balloon = int64(detailedStatus.Balloon)
		}
	}

	currentMetrics := IOMetrics{
		DiskRead:   diskReadBytes,
		DiskWrite:  diskWriteBytes,
		NetworkIn:  networkInBytes,
		NetworkOut: networkOutBytes,
		Timestamp:  sampleTime,
	}
	diskReadRate, diskWriteRate, netInRate, netOutRate := m.rateTracker.CalculateRates(guestID, currentMetrics)

	vm := models.VM{
		ID:           guestID,
		VMID:         res.VMID,
		Name:         res.Name,
		Node:         res.Node,
		Instance:     instanceName,
		Status:       res.Status,
		Type:         "qemu",
		CPU:          safeFloat(res.CPU),
		CPUs:         res.MaxCPU,
		Memory:       memory,
		MemorySource: memorySource,
		Disk: models.Disk{
			Total: int64(diskTotal),
			Used:  int64(diskUsed),
			Free:  int64(diskFree),
			Usage: diskUsage,
		},
		Disks:             individualDisks,
		DiskStatusReason:  diskStatusReason,
		IPAddresses:       ipAddresses,
		OSName:            osName,
		OSVersion:         osVersion,
		AgentVersion:      agentVersion,
		NetworkInterfaces: networkInterfaces,
		NetworkIn:         max(0, int64(netInRate)),
		NetworkOut:        max(0, int64(netOutRate)),
		DiskRead:          max(0, int64(diskReadRate)),
		DiskWrite:         max(0, int64(diskWriteRate)),
		Uptime:            int64(res.Uptime),
		Template:          res.Template == 1,
		LastSeen:          sampleTime,
	}

	if res.Tags != "" {
		vm.Tags = strings.Split(res.Tags, ";")
		for _, tag := range vm.Tags {
			switch tag {
			case "pulse-no-alerts", "pulse-monitor-only", "pulse-relaxed":
				log.Info().Str("vm", vm.Name).Str("node", vm.Node).Str("tag", tag).Msg("Pulse control tag detected on VM")
			}
		}
	}

	snapshot := GuestMemorySnapshot{
		Name:         vm.Name,
		Status:       vm.Status,
		RetrievedAt:  sampleTime,
		MemorySource: memorySource,
		Memory:       vm.Memory,
		Raw:          guestRaw,
		Notes:        snapshotNotes,
	}

	alertVM := vm
	if alertVM.Status != "running" {
		if logging.IsLevelEnabled(zerolog.DebugLevel) {
			log.Debug().
				Str("vm", alertVM.Name).
				Str("status", alertVM.Status).
				Float64("originalCpu", alertVM.CPU).
				Float64("originalMemUsage", alertVM.Memory.Usage).
				Msg("Non-running VM detected - zeroing metrics")
		}
		alertVM.CPU = 0
		alertVM.Memory.Usage = 0
		alertVM.Disk.Usage = 0
		alertVM.NetworkIn = 0
		alertVM.NetworkOut = 0
		alertVM.DiskRead = 0
		alertVM.DiskWrite = 0
	}

	return vm, alertVM, snapshot, true
}
