package monitoring

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/fsfilters"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/rs/zerolog/log"
)

type vmBuildState struct {
	memTotal          uint64
	memUsed           uint64
	memorySource      string
	guestRaw          VMMemoryRaw
	diskReadBytes     int64
	diskWriteBytes    int64
	networkInBytes    int64
	networkOutBytes   int64
	diskTotal         uint64
	diskUsed          uint64
	diskFree          uint64
	diskUsage         float64
	individualDisks   []models.Disk
	ipAddresses       []string
	networkInterfaces []models.GuestNetworkInterface
	osName            string
	osVersion         string
	agentVersion      string
	detailedStatus    *proxmox.VMStatus
}

func (m *Monitor) applyVMStatusDetails(
	ctx context.Context,
	instanceName string,
	res proxmox.ClusterResource,
	client PVEClientInterface,
	status *proxmox.VMStatus,
	state *vmBuildState,
) {
	if status == nil || state == nil {
		return
	}

	state.detailedStatus = status
	state.guestRaw.StatusMaxMem = status.MaxMem
	state.guestRaw.StatusMem = status.Mem
	state.guestRaw.StatusFreeMem = status.FreeMem
	state.guestRaw.Balloon = status.Balloon
	state.guestRaw.BalloonMin = status.BalloonMin
	state.guestRaw.Agent = status.Agent.Value

	memAvailable := uint64(0)
	if status.MemInfo != nil {
		state.guestRaw.MemInfoUsed = status.MemInfo.Used
		state.guestRaw.MemInfoFree = status.MemInfo.Free
		state.guestRaw.MemInfoTotal = status.MemInfo.Total
		state.guestRaw.MemInfoAvailable = status.MemInfo.Available
		state.guestRaw.MemInfoBuffers = status.MemInfo.Buffers
		state.guestRaw.MemInfoCached = status.MemInfo.Cached
		state.guestRaw.MemInfoShared = status.MemInfo.Shared

		switch {
		case status.MemInfo.Available > 0:
			memAvailable = status.MemInfo.Available
			state.memorySource = "meminfo-available"
		case status.MemInfo.Free > 0 || status.MemInfo.Buffers > 0 || status.MemInfo.Cached > 0:
			memAvailable = status.MemInfo.Free + status.MemInfo.Buffers + status.MemInfo.Cached
			state.memorySource = "meminfo-derived"
		}
	}

	// Use actual disk I/O values from detailed status
	state.diskReadBytes = int64(status.DiskRead)
	state.diskWriteBytes = int64(status.DiskWrite)
	state.networkInBytes = int64(status.NetIn)
	state.networkOutBytes = int64(status.NetOut)

	// Note: We intentionally do NOT override memTotal with balloon.
	// The balloon value is tracked separately in memory.balloon for
	// visualization purposes. Using balloon as total causes user
	// confusion (showing 1GB/1GB at 100% when VM is configured for 4GB)
	// and makes the frontend's balloon marker logic ineffective.
	// Refs: #1070
	if status.MaxMem > 0 {
		state.memTotal = status.MaxMem
	}

	switch {
	case memAvailable > 0:
		if memAvailable > state.memTotal {
			memAvailable = state.memTotal
		}
		state.memUsed = state.memTotal - memAvailable
	case status.Mem > 0:
		// Prefer Mem over FreeMem: Proxmox calculates Mem as
		// (total_mem - free_mem) using the balloon's guest-visible
		// total, which is correct even when ballooning is active.
		// FreeMem is relative to the balloon allocation (not MaxMem),
		// so subtracting it from MaxMem produces wildly inflated
		// usage when the balloon has reduced the VM's memory.
		// Refs: #1185
		state.memUsed = status.Mem
		state.memorySource = "status-mem"
	case status.FreeMem > 0 && state.memTotal >= status.FreeMem:
		state.memUsed = state.memTotal - status.FreeMem
		state.memorySource = "status-freemem"
	}
	if state.memUsed > state.memTotal {
		state.memUsed = state.memTotal
	}

	// Gather guest metadata from the agent when available
	guestIPs, guestIfaces, guestOSName, guestOSVersion, guestAgentVersion := m.fetchGuestAgentMetadata(ctx, client, instanceName, res.Node, res.Name, res.VMID, status)
	if len(guestIPs) > 0 {
		state.ipAddresses = guestIPs
	}
	if len(guestIfaces) > 0 {
		state.networkInterfaces = guestIfaces
	}
	if guestOSName != "" {
		state.osName = guestOSName
	}
	if guestOSVersion != "" {
		state.osVersion = guestOSVersion
	}
	if guestAgentVersion != "" {
		state.agentVersion = guestAgentVersion
	}

	// Always try to get filesystem info if agent is enabled
	// Prefer guest agent data over cluster/resources data for accuracy
	if status.Agent.Value > 0 {
		var fsDisks []models.Disk
		state.diskTotal, state.diskUsed, state.diskFree, state.diskUsage, fsDisks = m.updateVMDisksFromGuestAgentFSInfo(
			ctx,
			instanceName,
			res,
			client,
			state.diskTotal,
			state.diskUsed,
			state.diskUsage,
		)
		if len(fsDisks) > 0 {
			state.individualDisks = fsDisks
		}
	} else {
		// Agent disabled - show allocated disk size
		if state.diskTotal > 0 {
			state.diskUsage = -1 // Show as allocated size
		}
		log.Debug().
			Str("instance", instanceName).
			Str("vm", res.Name).
			Int("vmid", res.VMID).
			Int("agent", status.Agent.Value).
			Msg("VM does not have guest agent enabled in config")
	}
}

func (m *Monitor) buildVMFromClusterResource(
	ctx context.Context,
	instanceName string,
	res proxmox.ClusterResource,
	client PVEClientInterface,
) (models.VM, VMMemoryRaw, string, time.Time, bool) {
	// Skip templates if configured
	if res.Template == 1 {
		return models.VM{}, VMMemoryRaw{}, "", time.Time{}, false
	}

	guestID := makeGuestID(instanceName, res.Node, res.VMID)

	state := vmBuildState{
		memTotal:        res.MaxMem,
		memUsed:         res.Mem,
		memorySource:    "cluster-resources",
		guestRaw:        VMMemoryRaw{ListingMem: res.Mem, ListingMaxMem: res.MaxMem},
		diskReadBytes:   int64(res.DiskRead),
		diskWriteBytes:  int64(res.DiskWrite),
		networkInBytes:  int64(res.NetIn),
		networkOutBytes: int64(res.NetOut),
		diskTotal:       res.MaxDisk,
		diskUsed:        res.Disk,
	}
	state.diskFree = state.diskTotal - state.diskUsed
	state.diskUsage = safePercentage(float64(state.diskUsed), float64(state.diskTotal))

	// If VM shows 0 disk usage but has allocated disk, it's likely guest agent issue
	// Set to -1 to indicate "unknown" rather than showing misleading 0%
	if res.Type == "qemu" && state.diskUsed == 0 && state.diskTotal > 0 && res.Status == "running" {
		state.diskUsage = -1
	}

	// For running VMs, always try to get filesystem info from guest agent
	// The cluster/resources endpoint often returns 0 or incorrect values for disk usage
	// We should prefer guest agent data when available for accurate metrics
	if res.Status == "running" && res.Type == "qemu" {
		// First check if agent is enabled by getting VM status
		status, err := client.GetVMStatus(ctx, res.Node, res.VMID)
		if err != nil {
			log.Debug().
				Err(err).
				Str("instance", instanceName).
				Str("vm", res.Name).
				Int("vmid", res.VMID).
				Msg("Could not get VM status to check guest agent availability")
		} else if status != nil {
			m.applyVMStatusDetails(ctx, instanceName, res, client, status, &state)
		} else {
			// No vmStatus available - keep cluster/resources data
			log.Debug().
				Str("instance", instanceName).
				Str("vm", res.Name).
				Int("vmid", res.VMID).
				Msg("Could not get VM status, using cluster/resources disk data")
		}
	}

	if res.Status != "running" {
		state.memorySource = "powered-off"
		state.memUsed = 0
	}

	memFree := uint64(0)
	if state.memTotal >= state.memUsed {
		memFree = state.memTotal - state.memUsed
	}

	sampleTime := time.Now()
	currentMetrics := IOMetrics{
		DiskRead:   state.diskReadBytes,
		DiskWrite:  state.diskWriteBytes,
		NetworkIn:  state.networkInBytes,
		NetworkOut: state.networkOutBytes,
		Timestamp:  sampleTime,
	}
	diskReadRate, diskWriteRate, netInRate, netOutRate := m.rateTracker.CalculateRates(guestID, currentMetrics)

	memoryUsage := safePercentage(float64(state.memUsed), float64(state.memTotal))
	memory := models.Memory{
		Total: int64(state.memTotal),
		Used:  int64(state.memUsed),
		Free:  int64(memFree),
		Usage: memoryUsage,
	}
	if memory.Free < 0 {
		memory.Free = 0
	}
	if memory.Used > memory.Total {
		memory.Used = memory.Total
	}
	if state.detailedStatus != nil && state.detailedStatus.Balloon > 0 {
		memory.Balloon = int64(state.detailedStatus.Balloon)
	}

	vm := models.VM{
		ID:       guestID,
		VMID:     res.VMID,
		Name:     res.Name,
		Node:     res.Node,
		Instance: instanceName,
		Status:   res.Status,
		Type:     "qemu",
		CPU:      safeFloat(res.CPU),
		CPUs:     res.MaxCPU,
		Memory:   memory,
		Disk: models.Disk{
			Total: int64(state.diskTotal),
			Used:  int64(state.diskUsed),
			Free:  int64(state.diskFree),
			Usage: state.diskUsage,
		},
		Disks:             state.individualDisks,
		IPAddresses:       state.ipAddresses,
		OSName:            state.osName,
		OSVersion:         state.osVersion,
		AgentVersion:      state.agentVersion,
		NetworkInterfaces: state.networkInterfaces,
		NetworkIn:         max(0, int64(netInRate)),
		NetworkOut:        max(0, int64(netOutRate)),
		DiskRead:          max(0, int64(diskReadRate)),
		DiskWrite:         max(0, int64(diskWriteRate)),
		Uptime:            int64(res.Uptime),
		Template:          res.Template == 1,
		LastSeen:          sampleTime,
	}

	// Parse tags
	if res.Tags != "" {
		vm.Tags = strings.Split(res.Tags, ";")

		// Log if Pulse-specific tags are detected
		for _, tag := range vm.Tags {
			switch tag {
			case "pulse-no-alerts", "pulse-monitor-only", "pulse-relaxed":
				log.Info().
					Str("vm", vm.Name).
					Str("node", vm.Node).
					Str("tag", tag).
					Msg("Pulse control tag detected on VM")
			}
		}
	}

	return vm, state.guestRaw, state.memorySource, sampleTime, true
}

type vmFSInfoSummary struct {
	totalBytes      uint64
	usedBytes       uint64
	individualDisks []models.Disk
	skippedFS       []string
	includedFS      []string
}

func (m *Monitor) fetchVMFSInfo(ctx context.Context, instanceName string, res proxmox.ClusterResource, client PVEClientInterface) ([]proxmox.VMFileSystem, bool) {
	// Use retry logic for guest agent calls to handle transient timeouts (refs #630)
	fsInfoRaw, err := m.retryGuestAgentCall(ctx, m.guestAgentFSInfoTimeout, m.guestAgentRetries, func(ctx context.Context) (interface{}, error) {
		return client.GetVMFSInfo(ctx, res.Node, res.VMID)
	})
	var fsInfo []proxmox.VMFileSystem
	if err == nil {
		if fs, ok := fsInfoRaw.([]proxmox.VMFileSystem); ok {
			fsInfo = fs
		}
	}
	if err != nil {
		// Log more helpful error messages based on the error type
		errMsg := err.Error()
		if strings.Contains(errMsg, "500") || strings.Contains(errMsg, "QEMU guest agent is not running") {
			log.Info().
				Str("instance", instanceName).
				Str("vm", res.Name).
				Int("vmid", res.VMID).
				Msg("Guest agent enabled in VM config but not running inside guest OS. Install and start qemu-guest-agent in the VM")
			log.Info().
				Str("instance", instanceName).
				Str("vm", res.Name).
				Msg("To verify: ssh into VM and run 'systemctl status qemu-guest-agent' or 'ps aux | grep qemu-ga'")
		} else if strings.Contains(errMsg, "timeout") {
			log.Info().
				Str("instance", instanceName).
				Str("vm", res.Name).
				Int("vmid", res.VMID).
				Msg("Guest agent timeout - agent may be installed but not responding")
		} else if strings.Contains(errMsg, "403") || strings.Contains(errMsg, "401") || strings.Contains(errMsg, "authentication error") {
			// Permission error - user/token lacks required permissions
			log.Info().
				Str("instance", instanceName).
				Str("vm", res.Name).
				Int("vmid", res.VMID).
				Msg("VM disk monitoring permission denied. Check permissions:")
			log.Info().
				Str("instance", instanceName).
				Str("vm", res.Name).
				Msg("• Proxmox 9: Ensure token/user has VM.GuestAgent.Audit privilege (Pulse setup adds this via PulseMonitor role)")
			log.Info().
				Str("instance", instanceName).
				Str("vm", res.Name).
				Msg("• Proxmox 8: Ensure token/user has VM.Monitor privilege (Pulse setup adds this via PulseMonitor role)")
			log.Info().
				Str("instance", instanceName).
				Str("vm", res.Name).
				Msg("• All versions: Sys.Audit is recommended for Ceph metrics and applied when available")
			log.Info().
				Str("instance", instanceName).
				Str("vm", res.Name).
				Msg("• Re-run Pulse setup script if node was added before v4.7")
			log.Info().
				Str("instance", instanceName).
				Str("vm", res.Name).
				Msg("• Verify guest agent is installed and running inside the VM")
		} else {
			log.Debug().
				Err(err).
				Str("instance", instanceName).
				Str("vm", res.Name).
				Int("vmid", res.VMID).
				Msg("Failed to get filesystem info from guest agent")
		}
		return nil, false
	}
	if len(fsInfo) == 0 {
		log.Info().
			Str("instance", instanceName).
			Str("vm", res.Name).
			Int("vmid", res.VMID).
			Msg("Guest agent returned no filesystem info - agent may need restart or VM may have no mounted filesystems")
		return nil, false
	}
	return fsInfo, true
}

func (m *Monitor) summarizeVMFSInfo(instanceName string, res proxmox.ClusterResource, fsInfo []proxmox.VMFileSystem) vmFSInfoSummary {
	summary := vmFSInfoSummary{}

	// Track seen filesystems to dedupe btrfs/zfs subvolumes that share the same pool.
	// These filesystems mount multiple subvolumes from one storage pool, each reporting
	// the same TotalBytes. Without deduplication, we'd sum 11 × 77GB = 851GB instead of 77GB.
	// Key: "fstype:device:totalBytes" or "fstype::totalBytes" if device unknown.
	seenFilesystems := make(map[string]bool)

	// Log all filesystems received for debugging
	log.Debug().
		Str("instance", instanceName).
		Str("vm", res.Name).
		Int("vmid", res.VMID).
		Int("filesystem_count", len(fsInfo)).
		Msg("Processing filesystems from guest agent")

	for _, fs := range fsInfo {
		// Skip special filesystems and mounts
		shouldSkip, reasons := fsfilters.ShouldSkipFilesystem(fs.Type, fs.Mountpoint, fs.TotalBytes, fs.UsedBytes)
		if shouldSkip {
			// Check if any reason is read-only for detailed logging
			for _, r := range reasons {
				if strings.HasPrefix(r, "read-only-") {
					log.Debug().
						Str("instance", instanceName).
						Str("vm", res.Name).
						Int("vmid", res.VMID).
						Str("mountpoint", fs.Mountpoint).
						Str("type", fs.Type).
						Float64("total_gb", float64(fs.TotalBytes)/1073741824).
						Float64("used_gb", float64(fs.UsedBytes)/1073741824).
						Msg("Skipping read-only filesystem from disk aggregation")
					break
				}
			}
			summary.skippedFS = append(summary.skippedFS, fmt.Sprintf("%s(%s,%s)",
				fs.Mountpoint, fs.Type, strings.Join(reasons, ",")))
			continue
		}

		// Only count real filesystems with valid data
		// Some filesystems report 0 bytes (like unformatted or system partitions)
		if fs.TotalBytes > 0 {
			// Deduplication for COW filesystems (btrfs, zfs) that mount multiple
			// subvolumes from the same pool. Each subvolume reports identical TotalBytes
			// because they share the underlying storage pool.
			// Key format: "fstype:device:totalBytes" - if multiple mounts have the same
			// key, they're subvolumes of the same pool and should only be counted once.
			fsTypeLower := strings.ToLower(fs.Type)
			needsDedupe := fsTypeLower == "btrfs" || fsTypeLower == "zfs" ||
				strings.HasPrefix(fsTypeLower, "zfs")

			countThisFS := true
			if needsDedupe {
				// Use device if available, otherwise fall back to just type+size
				dedupeKey := fmt.Sprintf("%s:%s:%d", fsTypeLower, fs.Disk, fs.TotalBytes)
				if seenFilesystems[dedupeKey] {
					// Already counted this pool - skip adding to totals but still add to
					// individual disks for display purposes
					countThisFS = false
					log.Debug().
						Str("instance", instanceName).
						Str("vm", res.Name).
						Int("vmid", res.VMID).
						Str("mountpoint", fs.Mountpoint).
						Str("type", fs.Type).
						Str("device", fs.Disk).
						Uint64("total", fs.TotalBytes).
						Str("dedupe_key", dedupeKey).
						Msg("Skipping duplicate btrfs/zfs subvolume in total calculation")
				} else {
					seenFilesystems[dedupeKey] = true
				}
			}

			if countThisFS {
				summary.totalBytes += fs.TotalBytes
				summary.usedBytes += fs.UsedBytes
			}
			summary.includedFS = append(summary.includedFS, fmt.Sprintf("%s(%s,%.1fGB)",
				fs.Mountpoint, fs.Type, float64(fs.TotalBytes)/1073741824))

			// Add to individual disks array (always include for display)
			summary.individualDisks = append(summary.individualDisks, models.Disk{
				Total:      int64(fs.TotalBytes),
				Used:       int64(fs.UsedBytes),
				Free:       int64(fs.TotalBytes - fs.UsedBytes),
				Usage:      safePercentage(float64(fs.UsedBytes), float64(fs.TotalBytes)),
				Mountpoint: fs.Mountpoint,
				Type:       fs.Type,
				Device:     fs.Disk,
			})

			log.Debug().
				Str("instance", instanceName).
				Str("vm", res.Name).
				Int("vmid", res.VMID).
				Str("mountpoint", fs.Mountpoint).
				Str("type", fs.Type).
				Uint64("total", fs.TotalBytes).
				Uint64("used", fs.UsedBytes).
				Float64("total_gb", float64(fs.TotalBytes)/1073741824).
				Float64("used_gb", float64(fs.UsedBytes)/1073741824).
				Bool("counted_in_total", countThisFS).
				Msg("Including filesystem in disk usage calculation")
		} else if fs.TotalBytes == 0 && len(fs.Mountpoint) > 0 {
			summary.skippedFS = append(summary.skippedFS, fmt.Sprintf("%s(%s,0GB)", fs.Mountpoint, fs.Type))
			log.Debug().
				Str("instance", instanceName).
				Str("vm", res.Name).
				Int("vmid", res.VMID).
				Str("mountpoint", fs.Mountpoint).
				Str("type", fs.Type).
				Msg("Skipping filesystem with zero total bytes")
		}
	}

	if len(summary.skippedFS) > 0 {
		log.Debug().
			Str("instance", instanceName).
			Str("vm", res.Name).
			Strs("skipped", summary.skippedFS).
			Msg("Skipped special filesystems")
	}

	if len(summary.includedFS) > 0 {
		log.Debug().
			Str("instance", instanceName).
			Str("vm", res.Name).
			Int("vmid", res.VMID).
			Strs("included", summary.includedFS).
			Msg("Filesystems included in disk calculation")
	}

	return summary
}

func (m *Monitor) updateVMDisksFromGuestAgentFSInfo(
	ctx context.Context,
	instanceName string,
	res proxmox.ClusterResource,
	client PVEClientInterface,
	diskTotal uint64,
	diskUsed uint64,
	diskUsage float64,
) (uint64, uint64, uint64, float64, []models.Disk) {
	log.Debug().
		Str("instance", instanceName).
		Str("vm", res.Name).
		Int("vmid", res.VMID).
		Msg("Guest agent enabled, querying filesystem info for accurate disk usage")

	fsInfo, ok := m.fetchVMFSInfo(ctx, instanceName, res, client)
	if !ok {
		return diskTotal, diskUsed, diskTotal - diskUsed, diskUsage, nil
	}

	log.Debug().
		Str("instance", instanceName).
		Str("vm", res.Name).
		Int("filesystems", len(fsInfo)).
		Msg("Got filesystem info from guest agent")

	summary := m.summarizeVMFSInfo(instanceName, res, fsInfo)

	// If we got valid data from guest agent, use it
	if summary.totalBytes > 0 {
		// Sanity check: if the reported disk is way larger than allocated disk,
		// we might be getting host disk info somehow
		allocatedDiskGB := float64(res.MaxDisk) / 1073741824
		reportedDiskGB := float64(summary.totalBytes) / 1073741824

		// If reported disk is more than 2x the allocated disk, log a warning
		// This could indicate we're getting host disk or network shares
		if allocatedDiskGB > 0 && reportedDiskGB > allocatedDiskGB*2 {
			log.Warn().
				Str("instance", instanceName).
				Str("vm", res.Name).
				Int("vmid", res.VMID).
				Float64("allocated_gb", allocatedDiskGB).
				Float64("reported_gb", reportedDiskGB).
				Float64("ratio", reportedDiskGB/allocatedDiskGB).
				Strs("filesystems", summary.includedFS).
				Msg("VM reports disk usage significantly larger than allocated disk - possible issue with filesystem detection")
		}

		diskTotal = summary.totalBytes
		diskUsed = summary.usedBytes
		diskUsage = safePercentage(float64(summary.usedBytes), float64(summary.totalBytes))

		log.Debug().
			Str("instance", instanceName).
			Str("vm", res.Name).
			Int("vmid", res.VMID).
			Uint64("totalBytes", summary.totalBytes).
			Uint64("usedBytes", summary.usedBytes).
			Float64("total_gb", float64(summary.totalBytes)/1073741824).
			Float64("used_gb", float64(summary.usedBytes)/1073741824).
			Float64("allocated_gb", allocatedDiskGB).
			Float64("usage", diskUsage).
			Uint64("old_disk", res.Disk).
			Uint64("old_maxdisk", res.MaxDisk).
			Msg("Using guest agent data for accurate disk usage (replacing cluster/resources data)")
		return diskTotal, diskUsed, diskTotal - diskUsed, diskUsage, summary.individualDisks
	}

	// Only special filesystems found - show allocated disk size instead
	if diskTotal > 0 {
		diskUsage = -1 // Show as allocated size
	}
	log.Info().
		Str("instance", instanceName).
		Str("vm", res.Name).
		Int("filesystems_found", len(fsInfo)).
		Msg("Guest agent provided filesystem info but no usable filesystems found (all were special mounts)")

	return diskTotal, diskUsed, diskTotal - diskUsed, diskUsage, nil
}
