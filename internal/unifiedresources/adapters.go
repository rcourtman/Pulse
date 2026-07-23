package unifiedresources

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationreceipt"
	"github.com/rcourtman/pulse-go-rewrite/internal/platformsupport"
	"github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"
	"github.com/rcourtman/pulse-go-rewrite/pkg/diskinventory"
)

func resourceFromProxmoxNode(node models.Node, linkedHost *models.Host) (Resource, ResourceIdentity) {
	name := node.Name
	if node.DisplayName != "" {
		name = node.DisplayName
	}

	endpointHost := extractHostname(node.Host)
	identity := ResourceIdentity{
		Hostnames:   uniqueStrings([]string{node.Name}),
		ClusterName: node.ClusterName,
	}
	if endpointHost != "" {
		if parsed := net.ParseIP(endpointHost); parsed != nil {
			identity.IPAddresses = uniqueStrings([]string{parsed.String()})
		} else {
			identity.Hostnames = uniqueStrings(append(identity.Hostnames, endpointHost))
		}
	}

	if node.ClusterName != "" {
		identity.Hostnames = uniqueStrings(append(identity.Hostnames, node.ClusterName+":"+node.Name))
	}
	if linkedHost != nil {
		identity = mergeIdentity(identity, identityFromHost(*linkedHost))
	}

	linkedAgentID := strings.TrimSpace(node.LinkedAgentID)
	if linkedAgentID == "" && linkedHost != nil {
		linkedAgentID = strings.TrimSpace(linkedHost.ID)
	}

	proxmox := &ProxmoxData{
		SourceID:                     node.ID,
		NodeName:                     node.Name,
		ClusterName:                  node.ClusterName,
		IsClusterMember:              node.IsClusterMember,
		Instance:                     node.Instance,
		HostURL:                      node.Host,
		GuestURL:                     node.GuestURL,
		ConnectionHealth:             node.ConnectionHealth,
		Temperature:                  maxNodeTemp(node.Temperature),
		TemperatureDetails:           cloneTemperature(node.Temperature),
		PVEVersion:                   node.PVEVersion,
		KernelVersion:                node.KernelVersion,
		Uptime:                       node.Uptime,
		CPUInfo:                      &CPUInfo{Model: node.CPUInfo.Model, Cores: node.CPUInfo.Cores, Sockets: node.CPUInfo.Sockets},
		LoadAverage:                  append([]float64(nil), node.LoadAverage...),
		PendingUpdates:               node.PendingUpdates,
		TemperatureMonitoringEnabled: cloneBoolPtr(node.TemperatureMonitoringEnabled),
		PendingUpdatesCheckedAt:      zeroTimeToPtr(node.PendingUpdatesCheckedAt),
		MemoryCache:                  node.Memory.Cache,
		SwapUsed:                     node.Memory.SwapUsed,
		SwapTotal:                    node.Memory.SwapTotal,
		LinkedAgentID:                linkedAgentID,
	}

	metrics := metricsFromProxmoxNode(node)

	resource := Resource{
		Type:        ResourceTypeAgent,
		Technology:  "proxmox",
		Name:        name,
		Status:      statusFromString(node.Status),
		LastSeen:    node.LastSeen,
		UpdatedAt:   time.Now().UTC(),
		Metrics:     metrics,
		Uptime:      node.Uptime,
		Temperature: proxmox.Temperature,
		Proxmox:     proxmox,
		Tags:        nil,
	}

	return resource, identity
}

func identityFromHost(host models.Host) ResourceIdentity {
	ips, macs := collectInterfaceIDs(host.NetworkInterfaces)
	if host.ReportIP != "" {
		// --report-ip is the user naming the primary address on a multi-NIC
		// host, and consumers treat the first entry as primary, so it must
		// lead the list rather than trail the auto-detected ones (#829).
		ips = append([]string{host.ReportIP}, ips...)
	}

	return ResourceIdentity{
		MachineID:    strings.TrimSpace(host.MachineID),
		Hostnames:    uniqueStrings([]string{host.Hostname}),
		IPAddresses:  uniqueStrings(ips),
		MACAddresses: uniqueStrings(macs),
	}
}

// HostIngestRecord converts a host snapshot into the canonical source-native
// ingest shape used by monitored-system previews and continuity projections.
func HostIngestRecord(host models.Host) IngestRecord {
	resource, identity := resourceFromHost(host)
	return IngestRecord{
		SourceID: host.ID,
		Resource: resource,
		Identity: identity,
	}
}

func resourceFromHost(host models.Host) (Resource, ResourceIdentity) {
	name := host.Hostname
	if host.DisplayName != "" {
		name = host.DisplayName
	}

	identity := identityFromHost(host)
	hostProfile := agentHostProfileForHost(host)
	platform := agentRuntimePlatformForHost(host, hostProfile)

	agent := &AgentData{
		AgentID:           host.ID,
		AgentVersion:      host.AgentVersion,
		Hostname:          host.Hostname,
		MachineID:         host.MachineID,
		TokenID:           host.TokenID,
		TokenName:         host.TokenName,
		TokenHint:         host.TokenHint,
		TokenLastUsedAt:   host.TokenLastUsedAt,
		Platform:          platform,
		HostProfile:       hostProfile,
		OSName:            host.OSName,
		OSVersion:         host.OSVersion,
		KernelVersion:     host.KernelVersion,
		Architecture:      host.Architecture,
		CPUCount:          host.CPUCount,
		LoadAverage:       append([]float64(nil), host.LoadAverage...),
		UptimeSeconds:     host.UptimeSeconds,
		IntervalSeconds:   host.IntervalSeconds,
		Temperature:       maxCPUTemp(host.Sensors),
		NetworkInterfaces: convertInterfaces(host.NetworkInterfaces),
		Disks:             convertDisks(host.Disks),
		Memory: &AgentMemoryMeta{
			Total:     host.Memory.Total,
			Used:      host.Memory.Used,
			Free:      host.Memory.Free,
			Cache:     host.Memory.Cache,
			SwapUsed:  host.Memory.SwapUsed,
			SwapTotal: host.Memory.SwapTotal,
		},
		CommandsEnabled:         host.CommandsEnabled,
		OperationReceiptVersion: host.OperationReceiptVersion,
		ReportIP:                host.ReportIP,
		DiskExclude:             append([]string(nil), host.DiskExclude...),
		IsLegacy:                host.IsLegacy,
		NetInRate:               host.NetInRate,
		NetOutRate:              host.NetOutRate,
		DiskReadRate:            host.DiskReadRate,
		DiskWriteRate:           host.DiskWriteRate,
		LinkedNodeID:            host.LinkedNodeID,
		LinkedVMID:              host.LinkedVMID,
		LinkedContainerID:       host.LinkedContainerID,
	}
	if host.PackageUpdates != nil {
		packages := make([]AgentPackageUpdate, len(host.PackageUpdates.Packages))
		for i, pkg := range host.PackageUpdates.Packages {
			packages[i] = AgentPackageUpdate{
				Name:             pkg.Name,
				InstalledVersion: pkg.InstalledVersion,
				AvailableVersion: pkg.AvailableVersion,
			}
		}
		agent.PackageUpdates = &AgentPackageUpdateMeta{
			Supported:      host.PackageUpdates.Supported,
			Manager:        host.PackageUpdates.Manager,
			InventoryHash:  host.PackageUpdates.InventoryHash,
			PendingCount:   host.PackageUpdates.PendingCount,
			Packages:       packages,
			CheckedAt:      host.PackageUpdates.CheckedAt,
			ObservedAt:     host.PackageUpdates.ObservedAt,
			RebootRequired: host.PackageUpdates.RebootRequired,
			Error:          host.PackageUpdates.Error,
		}
	}
	if host.StorageCleanup != nil {
		agent.StorageCleanup = &AgentStorageCleanupMeta{
			Supported:        host.StorageCleanup.Supported,
			Provider:         host.StorageCleanup.Provider,
			Fingerprint:      host.StorageCleanup.Fingerprint,
			ReclaimableBytes: host.StorageCleanup.ReclaimableBytes,
			CheckedAt:        host.StorageCleanup.CheckedAt,
			ObservedAt:       host.StorageCleanup.ObservedAt,
			Error:            host.StorageCleanup.Error,
		}
	}

	// Surface agent staleness so a row that stays online via another source
	// (e.g. a Proxmox node still reachable over the PVE API) does not present a
	// dead agent's version as if it were current. The staleness evaluator marks
	// a host "offline" once it stops reporting; carry that onto the agent
	// payload along with the agent's own last report time.
	if !host.LastSeen.IsZero() {
		lastReport := host.LastSeen
		agent.LastReportAt = &lastReport
	}
	if strings.EqualFold(strings.TrimSpace(host.Status), "offline") {
		agent.Stale = true
	}

	storageAssessments := make([]storagehealth.Assessment, 0, len(host.RAID)+1)

	// Populate sensors
	if len(host.Sensors.TemperatureCelsius) > 0 || len(host.Sensors.FanRPM) > 0 || len(host.Sensors.PowerWatts) > 0 || len(host.Sensors.Additional) > 0 || len(host.Sensors.GPU) > 0 || host.Sensors.ThermalState != nil || len(host.Sensors.SMART) > 0 {
		sensorMeta := &HostSensorMeta{}
		if len(host.Sensors.TemperatureCelsius) > 0 {
			sensorMeta.TemperatureCelsius = make(map[string]float64, len(host.Sensors.TemperatureCelsius))
			for k, v := range host.Sensors.TemperatureCelsius {
				sensorMeta.TemperatureCelsius[k] = v
			}
		}
		if len(host.Sensors.FanRPM) > 0 {
			sensorMeta.FanRPM = make(map[string]float64, len(host.Sensors.FanRPM))
			for k, v := range host.Sensors.FanRPM {
				sensorMeta.FanRPM[k] = v
			}
		}
		if len(host.Sensors.PowerWatts) > 0 {
			sensorMeta.PowerWatts = make(map[string]float64, len(host.Sensors.PowerWatts))
			for k, v := range host.Sensors.PowerWatts {
				sensorMeta.PowerWatts[k] = v
			}
		}
		if len(host.Sensors.Additional) > 0 {
			sensorMeta.Additional = make(map[string]float64, len(host.Sensors.Additional))
			for k, v := range host.Sensors.Additional {
				sensorMeta.Additional[k] = v
			}
		}
		if len(host.Sensors.GPU) > 0 {
			sensorMeta.GPU = make([]HostGPUSensor, len(host.Sensors.GPU))
			for i, gpu := range host.Sensors.GPU {
				sensorMeta.GPU[i] = HostGPUSensor{
					ID:                 gpu.ID,
					Name:               gpu.Name,
					TemperatureCelsius: cloneFloat64Ptr(gpu.TemperatureCelsius),
					UtilizationPercent: cloneFloat64Ptr(gpu.UtilizationPercent),
					MemoryUsedBytes:    cloneInt64Ptr(gpu.MemoryUsedBytes),
					MemoryTotalBytes:   cloneInt64Ptr(gpu.MemoryTotalBytes),
				}
			}
		}
		if host.Sensors.ThermalState != nil {
			sensorMeta.ThermalState = hostThermalStateToMeta(host.Sensors.ThermalState)
		}
		if len(host.Sensors.SMART) > 0 {
			sensorMeta.SMART = make([]HostSMARTMeta, len(host.Sensors.SMART))
			for i, s := range host.Sensors.SMART {
				sensorMeta.SMART[i] = HostSMARTMeta{
					Device:      s.Device,
					Model:       s.Model,
					Serial:      s.Serial,
					WWN:         s.WWN,
					Type:        s.Type,
					Controller:  s.Controller,
					Target:      s.Target,
					SizeBytes:   s.SizeBytes,
					Temperature: s.Temperature,
					Health:      s.Health,
					Standby:     s.Standby,
					Pool:        s.Pool,
					IO:          physicalDiskIOToMeta(s.IO),
					Collection:  diskinventory.CloneStatus(s.Collection),
					Attributes:  cloneSMARTAttributes(s.Attributes),
				}
			}
		}
		agent.Sensors = sensorMeta
	}

	// Populate RAID
	if len(host.RAID) > 0 {
		raid := make([]HostRAIDMeta, len(host.RAID))
		for i, r := range host.RAID {
			devices := make([]HostRAIDDeviceMeta, len(r.Devices))
			for j, device := range r.Devices {
				devices[j] = HostRAIDDeviceMeta{
					Device: device.Device,
					State:  device.State,
					Slot:   device.Slot,
				}
			}
			assessment := storagehealth.AssessHostRAIDArray(r)
			raid[i] = HostRAIDMeta{
				Device:         r.Device,
				Name:           r.Name,
				Level:          r.Level,
				State:          r.State,
				TotalDevices:   r.TotalDevices,
				ActiveDevices:  r.ActiveDevices,
				WorkingDevices: r.WorkingDevices,
				FailedDevices:  r.FailedDevices,
				SpareDevices:   r.SpareDevices,
				UUID:           r.UUID,
				Devices:        devices,
				RebuildPercent: r.RebuildPercent,
				RebuildSpeed:   r.RebuildSpeed,
				Operation:      r.Operation,
				Risk:           storageRiskFromAssessment(assessment),
			}
			if !isInternalHostRAIDDevice(r.Device) {
				storageAssessments = append(storageAssessments, assessment)
			}
		}
		agent.RAID = raid
	}

	if host.Unraid != nil {
		disks := make([]HostUnraidDiskMeta, len(host.Unraid.Disks))
		for i, disk := range host.Unraid.Disks {
			sizeBytes := unraidDiskSizeBytes(host, disk)
			disks[i] = HostUnraidDiskMeta{
				Name:        disk.Name,
				Device:      disk.Device,
				Role:        disk.Role,
				Status:      disk.Status,
				RawStatus:   disk.RawStatus,
				Model:       disk.Model,
				Serial:      disk.Serial,
				Filesystem:  disk.Filesystem,
				Transport:   disk.Transport,
				SizeBytes:   sizeBytes,
				UsedBytes:   disk.UsedBytes,
				FreeBytes:   disk.FreeBytes,
				Temperature: disk.Temperature,
				SpunDown:    disk.SpunDown,
				ReadCount:   disk.ReadCount,
				WriteCount:  disk.WriteCount,
				ErrorCount:  disk.ErrorCount,
				Slot:        disk.Slot,
			}
		}
		assessment := storagehealth.AssessUnraidStorage(*host.Unraid)
		unraidRisk := storageRiskFromAssessment(assessment)
		_, protectionReduced, rebuildInProgress, protectionSummary, rebuildSummary := StorageRiskSemantics(unraidRisk)
		agent.Unraid = &HostUnraidMeta{
			ArrayStarted:      host.Unraid.ArrayStarted,
			ArrayState:        host.Unraid.ArrayState,
			SyncAction:        host.Unraid.SyncAction,
			SyncProgress:      host.Unraid.SyncProgress,
			SyncErrors:        host.Unraid.SyncErrors,
			NumProtected:      host.Unraid.NumProtected,
			NumDisabled:       host.Unraid.NumDisabled,
			NumInvalid:        host.Unraid.NumInvalid,
			NumMissing:        host.Unraid.NumMissing,
			Disks:             disks,
			Risk:              unraidRisk,
			RiskSummary:       StorageRiskSummary(unraidRisk),
			PostureSummary:    StorageRiskSummary(unraidRisk),
			ProtectionReduced: protectionReduced,
			ProtectionSummary: protectionSummary,
			RebuildInProgress: rebuildInProgress,
			RebuildSummary:    rebuildSummary,
		}
		storageAssessments = append(storageAssessments, assessment)
	}

	if len(storageAssessments) > 0 {
		agent.StorageRisk = storageRiskFromAssessment(storagehealth.SummarizeAssessments(storageAssessments...))
		agent.StorageRiskSummary = StorageRiskSummary(agent.StorageRisk)
		agent.StoragePostureSummary = agent.StorageRiskSummary
		_, agent.ProtectionReduced, agent.RebuildInProgress, agent.ProtectionSummary, agent.RebuildSummary = StorageRiskSemantics(agent.StorageRisk)
	}

	// Populate DiskIO
	if len(host.DiskIO) > 0 {
		diskIO := make([]HostDiskIOMeta, len(host.DiskIO))
		for i, d := range host.DiskIO {
			diskIO[i] = HostDiskIOMeta{
				Device:     d.Device,
				ReadBytes:  d.ReadBytes,
				WriteBytes: d.WriteBytes,
				ReadOps:    d.ReadOps,
				WriteOps:   d.WriteOps,
				IOTimeMs:   d.IOTime,
			}
		}
		agent.DiskIO = diskIO
	}

	// Populate Ceph
	if host.Ceph != nil {
		healthChecks := make(map[string]HostCephCheckMeta, len(host.Ceph.Health.Checks))
		for name, check := range host.Ceph.Health.Checks {
			healthChecks[name] = HostCephCheckMeta{
				Severity: check.Severity,
				Message:  check.Message,
				Detail:   append([]string(nil), check.Detail...),
			}
		}
		healthSummary := make([]HostCephHealthSummaryMeta, len(host.Ceph.Health.Summary))
		for i, summary := range host.Ceph.Health.Summary {
			healthSummary[i] = HostCephHealthSummaryMeta{
				Severity: summary.Severity,
				Message:  summary.Message,
			}
		}
		monitors := make([]HostCephMonitorMeta, len(host.Ceph.MonMap.Monitors))
		for i, mon := range host.Ceph.MonMap.Monitors {
			monitors[i] = HostCephMonitorMeta{
				Name:   mon.Name,
				Rank:   mon.Rank,
				Addr:   mon.Addr,
				Status: mon.Status,
			}
		}
		pools := make([]HostCephPoolMeta, len(host.Ceph.Pools))
		for i, pool := range host.Ceph.Pools {
			pools[i] = HostCephPoolMeta{
				ID:             pool.ID,
				Name:           pool.Name,
				BytesUsed:      pool.BytesUsed,
				BytesAvailable: pool.BytesAvailable,
				Objects:        pool.Objects,
				PercentUsed:    pool.PercentUsed,
			}
		}
		services := make([]HostCephServiceMeta, len(host.Ceph.Services))
		for i, service := range host.Ceph.Services {
			services[i] = HostCephServiceMeta{
				Type:    service.Type,
				Running: service.Running,
				Total:   service.Total,
				Daemons: append([]string(nil), service.Daemons...),
			}
		}
		agent.Ceph = &HostCephMeta{
			FSID: host.Ceph.FSID,
			Health: HostCephHealthMeta{
				Status:  host.Ceph.Health.Status,
				Checks:  healthChecks,
				Summary: healthSummary,
			},
			MonMap: HostCephMonitorMapMeta{
				Epoch:    host.Ceph.MonMap.Epoch,
				NumMons:  host.Ceph.MonMap.NumMons,
				Monitors: monitors,
			},
			MgrMap: HostCephManagerMapMeta{
				Available: host.Ceph.MgrMap.Available,
				NumMgrs:   host.Ceph.MgrMap.NumMgrs,
				ActiveMgr: host.Ceph.MgrMap.ActiveMgr,
				Standbys:  host.Ceph.MgrMap.Standbys,
			},
			OSDMap: HostCephOSDMapMeta{
				Epoch:   host.Ceph.OSDMap.Epoch,
				NumOSDs: host.Ceph.OSDMap.NumOSDs,
				NumUp:   host.Ceph.OSDMap.NumUp,
				NumIn:   host.Ceph.OSDMap.NumIn,
				NumDown: host.Ceph.OSDMap.NumDown,
				NumOut:  host.Ceph.OSDMap.NumOut,
			},
			PGMap: HostCephPGMapMeta{
				NumPGs:           host.Ceph.PGMap.NumPGs,
				BytesTotal:       host.Ceph.PGMap.BytesTotal,
				BytesUsed:        host.Ceph.PGMap.BytesUsed,
				BytesAvailable:   host.Ceph.PGMap.BytesAvailable,
				DataBytes:        host.Ceph.PGMap.DataBytes,
				UsagePercent:     host.Ceph.PGMap.UsagePercent,
				DegradedRatio:    host.Ceph.PGMap.DegradedRatio,
				MisplacedRatio:   host.Ceph.PGMap.MisplacedRatio,
				ReadBytesPerSec:  host.Ceph.PGMap.ReadBytesPerSec,
				WriteBytesPerSec: host.Ceph.PGMap.WriteBytesPerSec,
				ReadOpsPerSec:    host.Ceph.PGMap.ReadOpsPerSec,
				WriteOpsPerSec:   host.Ceph.PGMap.WriteOpsPerSec,
			},
			Pools:        pools,
			Services:     services,
			CollectedAt:  host.Ceph.CollectedAt,
			HealthStatus: host.Ceph.Health.Status,
			NumOSDs:      host.Ceph.OSDMap.NumOSDs,
			NumOSDsUp:    host.Ceph.OSDMap.NumUp,
			NumOSDsIn:    host.Ceph.OSDMap.NumIn,
			NumPGs:       host.Ceph.PGMap.NumPGs,
			UsagePercent: host.Ceph.PGMap.UsagePercent,
		}
	}

	metrics := metricsFromHost(host)

	resource := Resource{
		Type:         ResourceTypeAgent,
		Technology:   strings.TrimSpace(platform),
		Name:         name,
		Status:       storageStatus(statusFromString(host.Status), agent.StorageRisk),
		LastSeen:     host.LastSeen,
		UpdatedAt:    time.Now().UTC(),
		Metrics:      metrics,
		Uptime:       host.UptimeSeconds,
		Temperature:  agent.Temperature,
		Agent:        agent,
		Tags:         host.Tags,
		Capabilities: hostActionCapabilities(host),
	}

	return resource, identity
}

const hostPackageUpdateHandler = "host.package_updates"

const hostStorageCleanupHandler = "host.storage_cleanup"

func hostActionCapabilities(host models.Host) []ResourceCapability {
	capabilities := hostPackageUpdateCapabilities(host)
	if capability, ok := hostStorageCleanupCapability(host); ok {
		capabilities = append(capabilities, capability)
	}
	return capabilities
}

func hostPackageUpdateCapabilities(host models.Host) []ResourceCapability {
	status := host.PackageUpdates
	now := time.Now().UTC()
	meta := agentPackageUpdateMetaFromModel(status)
	if !host.CommandsEnabled || host.OperationReceiptVersion != operationreceipt.ProtocolVersion || status == nil || !status.Supported || strings.TrimSpace(status.Manager) != "apt" || strings.TrimSpace(status.InventoryHash) == "" || status.PendingCount <= 0 || strings.TrimSpace(status.Error) != "" || !HostPackageUpdateTelemetryFresh(meta, now) {
		return nil
	}
	return []ResourceCapability{{
		Name:                 "install_os_updates",
		Type:                 CapabilityTypeCommon,
		Description:          "Refresh APT metadata and install standard OS package upgrades without removing packages or rebooting the host.",
		MinimumApprovalLevel: ApprovalAdmin,
		AutoAuthorization:    AutoAuthorizeElevated,
		Platform:             "linux",
		InternalHandler:      hostPackageUpdateHandler,
	}}
}

func hostStorageCleanupCapability(host models.Host) (ResourceCapability, bool) {
	status := host.StorageCleanup
	now := time.Now().UTC()
	meta := agentStorageCleanupMetaFromModel(status)
	if !host.CommandsEnabled || host.OperationReceiptVersion != operationreceipt.ProtocolVersion || status == nil || !status.Supported || strings.TrimSpace(status.Provider) != "apt-package-cache" || strings.TrimSpace(status.Fingerprint) == "" || status.ReclaimableBytes < HostStorageCleanupMinReclaimableBytes || strings.TrimSpace(status.Error) != "" || !HostStorageCleanupTelemetryFresh(meta, now) {
		return ResourceCapability{}, false
	}
	if _, ok := HostStorageCleanupPressureDisk(convertDisks(host.Disks)); !ok {
		return ResourceCapability{}, false
	}
	return ResourceCapability{
		Name:                 "clean_package_cache",
		Type:                 CapabilityTypeCommon,
		Description:          "Reclaim the agent-managed APT package cache without selecting paths, removing installed packages, or rebooting the host.",
		MinimumApprovalLevel: ApprovalAdmin,
		AutoAuthorization:    AutoAuthorizeLowRisk,
		Platform:             "linux",
		InternalHandler:      hostStorageCleanupHandler,
	}, true
}

func agentPackageUpdateMetaFromModel(status *models.HostPackageUpdateStatus) *AgentPackageUpdateMeta {
	if status == nil {
		return nil
	}
	return &AgentPackageUpdateMeta{CheckedAt: status.CheckedAt, ObservedAt: status.ObservedAt}
}

func agentStorageCleanupMetaFromModel(status *models.HostStorageCleanupStatus) *AgentStorageCleanupMeta {
	if status == nil {
		return nil
	}
	return &AgentStorageCleanupMeta{CheckedAt: status.CheckedAt, ObservedAt: status.ObservedAt}
}

func hostThermalStateToMeta(in *models.HostThermalState) *HostThermalState {
	if in == nil {
		return nil
	}
	return &HostThermalState{
		Source:                  in.Source,
		Pressure:                in.Pressure,
		ThermalWarningLevel:     cloneIntPtr(in.ThermalWarningLevel),
		PerformanceWarningLevel: cloneIntPtr(in.PerformanceWarningLevel),
		CPUPowerStatus:          cloneIntPtr(in.CPUPowerStatus),
		LimitsPercent:           cloneStringIntMap(in.LimitsPercent),
	}
}

func agentRuntimePlatformForHost(host models.Host, hostProfile string) string {
	return platformsupport.NormalizeRuntimePlatformForAgentHostProfile(hostProfile, host.Platform)
}

func agentHostProfileForHost(host models.Host) string {
	if host.Unraid != nil {
		return agentHostProfileIDFromIdentity("unraid")
	}
	return agentHostProfileIDFromIdentity(host.OSName, host.Platform)
}

func agentHostProfileIDFromIdentity(values ...string) string {
	profile, ok := platformsupport.AgentHostProfileForIdentity(values...)
	if !ok {
		return ""
	}
	return profile.ID
}

func resourceFromHostUnraidStorage(host models.Host) (Resource, ResourceIdentity) {
	name := host.Hostname
	if host.DisplayName != "" {
		name = host.DisplayName
	}
	name = strings.TrimSpace(name) + " Array"

	assessment := storagehealth.AssessUnraidStorage(*host.Unraid)
	protection := "none"
	switch {
	case host.Unraid.NumProtected >= 2:
		protection = "dual-parity"
	case host.Unraid.NumProtected == 1:
		protection = "single-parity"
	}

	path := unraidStoragePath(host)
	resource := Resource{
		Type:       ResourceTypeStorage,
		Technology: "unraid",
		Name:       name,
		Status:     storageStatus(statusFromString(host.Status), storageRiskFromAssessment(assessment)),
		LastSeen:   host.LastSeen,
		UpdatedAt:  time.Now().UTC(),
		Metrics:    metricsFromUnraidStorage(host),
		Storage: &StorageMeta{
			Type:         "unraid-array",
			Content:      "files",
			ContentTypes: []string{"files"},
			Shared:       false,
			Enabled:      true,
			Active:       true,
			IsCeph:       false,
			IsZFS:        false,
			Platform:     "unraid",
			Topology:     "array",
			Protection:   protection,
			Risk:         storageRiskFromAssessment(assessment),
			Path:         path,
			ArrayState:   host.Unraid.ArrayState,
			SyncAction:   host.Unraid.SyncAction,
			SyncProgress: host.Unraid.SyncProgress,
			NumProtected: host.Unraid.NumProtected,
			NumDisabled:  host.Unraid.NumDisabled,
			NumInvalid:   host.Unraid.NumInvalid,
			NumMissing:   host.Unraid.NumMissing,
		},
		Tags: uniqueStrings([]string{
			"unraid",
			"storage",
			"array",
			protection,
		}),
	}

	identity := ResourceIdentity{
		MachineID: hostUnraidStorageIdentity(host),
		Hostnames: uniqueStrings([]string{
			host.Hostname,
			host.Hostname + ":unraid-array",
		}),
	}

	return resource, identity
}

func resourceFromHostUnraidCacheStorage(host models.Host, disk models.HostUnraidDisk) (Resource, ResourceIdentity) {
	name := unraidCachePoolName(disk)
	if name == "" {
		name = "cache"
	}
	total, used, _, percent := unraidDiskCapacityForHost(host, disk)
	resource := Resource{
		Type:       ResourceTypeStorage,
		Technology: "unraid",
		Name:       name,
		Status:     statusFromString(firstNonEmpty(disk.Status, host.Status)),
		LastSeen:   host.LastSeen,
		UpdatedAt:  time.Now().UTC(),
		Metrics:    metricsFromUnraidDiskCapacity(total, used, percent),
		Storage: &StorageMeta{
			Type:         "unraid-cache-pool",
			Content:      "files",
			ContentTypes: []string{"files"},
			Shared:       false,
			Enabled:      true,
			Active:       strings.EqualFold(strings.TrimSpace(disk.Status), "online"),
			IsCeph:       false,
			IsZFS:        strings.EqualFold(strings.TrimSpace(disk.Filesystem), "zfs"),
			Platform:     "unraid",
			Topology:     "pool",
			Protection:   "none",
			Path:         "/mnt/" + name,
		},
		Tags: uniqueStrings([]string{
			"unraid",
			"storage",
			"cache",
			strings.TrimSpace(disk.Filesystem),
		}),
	}
	identity := ResourceIdentity{
		MachineID: hostUnraidCacheStorageIdentity(host, disk),
		Hostnames: uniqueStrings([]string{
			host.Hostname,
			host.Hostname + ":unraid-cache:" + name,
		}),
	}
	return resource, identity
}

func resourceFromHostUnraidPhysicalDisk(host models.Host, disk models.HostUnraidDisk) (Resource, ResourceIdentity) {
	model := strings.TrimSpace(disk.Model)
	name := firstNonEmpty(model, disk.Name, disk.Device, host.Hostname)
	health := unraidPhysicalDiskHealth(disk)
	assessment := assessUnraidPhysicalDisk(disk)
	sizeBytes := unraidDiskSizeBytes(host, disk)
	resource := Resource{
		Type:      ResourceTypePhysicalDisk,
		Name:      name,
		Status:    physicalDiskStatus(model, health, assessment),
		LastSeen:  host.LastSeen,
		UpdatedAt: time.Now().UTC(),
		PhysicalDisk: &PhysicalDiskMeta{
			DevPath:      strings.TrimSpace(disk.Device),
			Model:        model,
			Serial:       strings.TrimSpace(disk.Serial),
			DiskType:     unraidDiskDiskType(disk),
			SizeBytes:    sizeBytes,
			Health:       health,
			Wearout:      -1,
			Temperature:  disk.Temperature,
			Used:         unraidDiskMountPath(disk),
			StorageRole:  unraidDiskRole(&disk),
			StorageGroup: unraidDiskGroup(&disk),
			StorageState: strings.TrimSpace(disk.Status),
			SpunDown:     disk.SpunDown,
			ReadCount:    disk.ReadCount,
			WriteCount:   disk.WriteCount,
			ErrorCount:   disk.ErrorCount,
			Risk:         physicalDiskRiskFromAssessment(assessment),
		},
	}
	identity := ResourceIdentity{
		Hostnames: uniqueStrings([]string{host.Hostname}),
	}
	if serial := strings.TrimSpace(disk.Serial); diskinventory.IsUsableHardwareID(serial) {
		identity.MachineID = serial
	}
	return resource, identity
}

func resourceFromHostSMARTDisk(host models.Host, disk models.HostDiskSMART) (Resource, ResourceIdentity) {
	name := strings.TrimSpace(disk.Model)
	if name == "" {
		name = strings.TrimSpace(disk.Device)
	}
	if name == "" {
		name = strings.TrimSpace(host.Hostname)
	}

	var matchedDisk *models.Disk
	normalizedDevice := strings.ToLower(normalizePhysicalDiskDeviceToken(disk.Device))
	for i := range host.Disks {
		hostDevice := strings.ToLower(normalizePhysicalDiskDeviceToken(host.Disks[i].Device))
		if normalizedDevice == "" || hostDevice == "" {
			continue
		}
		if strings.EqualFold(hostDevice, normalizedDevice) {
			matchedDisk = &host.Disks[i]
			break
		}
	}

	// The agent reports the authoritative disk capacity (from /sys/block); the
	// filesystem-usage match is only a last-resort fallback for older agents that
	// predate the SizeBytes field. matchedDisk.Total is a filesystem size, not a
	// whole-disk size, so it is never preferred over a real capacity.
	sizeBytes := disk.SizeBytes
	used := ""
	if matchedDisk != nil {
		if sizeBytes <= 0 {
			sizeBytes = matchedDisk.Total
		}
		used = strings.TrimSpace(matchedDisk.Mountpoint)
	}
	unraidDisk := matchUnraidDisk(host.Unraid, disk)
	model := strings.TrimSpace(disk.Model)
	serial := strings.TrimSpace(disk.Serial)
	diskType := strings.TrimSpace(disk.Type)
	temperature := disk.Temperature
	health := strings.TrimSpace(disk.Health)
	if unraidDisk != nil {
		if model == "" {
			model = strings.TrimSpace(unraidDisk.Model)
		}
		if serial == "" {
			serial = strings.TrimSpace(unraidDisk.Serial)
		}
		if diskType == "" {
			diskType = unraidDiskDiskType(*unraidDisk)
		}
		if sizeBytes <= 0 {
			sizeBytes = unraidDiskSizeBytes(host, *unraidDisk)
		}
		if temperature <= 0 {
			temperature = unraidDisk.Temperature
		}
		if health == "" || strings.EqualFold(health, "UNKNOWN") {
			health = unraidPhysicalDiskHealth(*unraidDisk)
		}
	}
	assessment := storagehealth.AssessHostSMARTDisk(disk)
	if unraidDisk != nil {
		assessment = storagehealth.SummarizeAssessments(assessment, assessUnraidPhysicalDisk(*unraidDisk))
	}
	wearout := physicalDiskWearoutFromSMARTAttributes(disk.Attributes)

	storageGroup := unraidDiskGroup(unraidDisk)
	if storageGroup == "" {
		storageGroup = strings.TrimSpace(disk.Pool)
	}

	resource := Resource{
		Type:      ResourceTypePhysicalDisk,
		Name:      firstNonEmpty(model, name),
		Status:    physicalDiskStatus(model, health, assessment),
		LastSeen:  host.LastSeen,
		UpdatedAt: time.Now().UTC(),
		PhysicalDisk: &PhysicalDiskMeta{
			DevPath:      strings.TrimSpace(disk.Device),
			Model:        model,
			Serial:       serial,
			WWN:          strings.TrimSpace(disk.WWN),
			DiskType:     diskType,
			Controller:   strings.TrimSpace(disk.Controller),
			Target:       strings.TrimSpace(disk.Target),
			SizeBytes:    sizeBytes,
			Health:       health,
			Wearout:      wearout,
			Temperature:  temperature,
			Used:         used,
			StorageRole:  unraidDiskRole(unraidDisk),
			StorageGroup: storageGroup,
			StorageState: unraidDiskState(unraidDisk),
			SpunDown:     unraidDisk != nil && unraidDisk.SpunDown,
			ReadCount:    unraidDiskCounter(unraidDisk, "read"),
			WriteCount:   unraidDiskCounter(unraidDisk, "write"),
			ErrorCount:   unraidDiskCounter(unraidDisk, "error"),
			IO:           physicalDiskIOToMeta(disk.IO),
			Collection:   diskinventory.CloneStatus(disk.Collection),
			SMART:        convertSMARTAttributes(disk.Attributes),
			Risk:         physicalDiskRiskFromAssessment(assessment),
		},
	}

	identity := ResourceIdentity{
		Hostnames: uniqueStrings([]string{host.Hostname}),
	}
	if diskinventory.IsUsableHardwareID(serial) {
		identity.MachineID = serial
	} else if diskinventory.IsUsableHardwareID(disk.WWN) {
		identity.MachineID = strings.TrimSpace(disk.WWN)
	}

	return resource, identity
}

func physicalDiskWearoutFromSMARTAttributes(attrs *models.SMARTAttributes) int {
	if attrs == nil || attrs.PercentageUsed == nil {
		return -1
	}
	used := *attrs.PercentageUsed
	if used < 0 {
		used = 0
	}
	if used > 100 {
		used = 100
	}
	return 100 - used
}

func hostUnraidStorageIdentity(host models.Host) string {
	if machineID := strings.TrimSpace(host.MachineID); machineID != "" {
		return machineID + "/storage/unraid-array"
	}
	if hostname := strings.TrimSpace(host.Hostname); hostname != "" {
		return hostname + "/storage/unraid-array"
	}
	return ""
}

func hostUnraidStorageSourceID(host models.Host) string {
	hostID := strings.TrimSpace(host.ID)
	if hostID == "" {
		return ""
	}
	return hostID + "/storage:unraid-array"
}

func hostUnraidCacheStorageIdentity(host models.Host, disk models.HostUnraidDisk) string {
	pool := unraidCachePoolName(disk)
	if pool == "" {
		return ""
	}
	if machineID := strings.TrimSpace(host.MachineID); machineID != "" {
		return machineID + "/storage/unraid-cache/" + pool
	}
	if hostname := strings.TrimSpace(host.Hostname); hostname != "" {
		return hostname + "/storage/unraid-cache/" + pool
	}
	return ""
}

func hostUnraidCacheStorageSourceID(host models.Host, disk models.HostUnraidDisk) string {
	hostID := strings.TrimSpace(host.ID)
	pool := unraidCachePoolName(disk)
	if hostID == "" || pool == "" {
		return ""
	}
	return hostID + "/storage:unraid-cache:" + pool
}

func unraidCachePoolName(disk models.HostUnraidDisk) string {
	return strings.TrimSpace(firstNonEmpty(disk.Name, strings.TrimPrefix(disk.Device, "/dev/")))
}

func unraidStoragePath(host models.Host) string {
	for _, disk := range host.Disks {
		mount := strings.TrimSpace(disk.Mountpoint)
		switch mount {
		case "/mnt/user", "/mnt/user0":
			return mount
		}
	}
	return "/mnt/user"
}

func unraidDiskMountPath(disk models.HostUnraidDisk) string {
	name := strings.TrimSpace(disk.Name)
	switch unraidDiskRole(&disk) {
	case "data":
		if name != "" {
			return "/mnt/" + name
		}
	case "cache":
		if name != "" {
			return "/mnt/" + name
		}
	}
	return ""
}

func unraidStorageCapacity(host models.Host) (int64, int64, int64, float64) {
	for _, disk := range host.Disks {
		mount := strings.TrimSpace(disk.Mountpoint)
		if mount == "/mnt/user" || mount == "/mnt/user0" {
			return disk.Total, disk.Used, disk.Free, percentFromReportedPercent(disk.Usage)
		}
	}

	deviceUsage := make(map[string]models.Disk, len(host.Disks))
	for _, disk := range host.Disks {
		device := strings.TrimSpace(strings.TrimPrefix(strings.ToLower(disk.Device), "/dev/"))
		if device != "" {
			deviceUsage[device] = disk
		}
	}

	var total int64
	var used int64
	var free int64
	for _, disk := range host.Unraid.Disks {
		if unraidDiskRole(&disk) != "data" {
			continue
		}
		diskTotal, diskUsed, diskFree, _ := unraidDiskCapacityForHost(host, disk)
		if diskTotal > 0 {
			total += diskTotal
			used += diskUsed
			free += diskFree
			continue
		}
		device := strings.TrimSpace(strings.TrimPrefix(strings.ToLower(disk.Device), "/dev/"))
		if usage, ok := deviceUsage[device]; ok && usage.Total > 0 {
			total += usage.Total
			used += usage.Used
			free += usage.Free
		}
	}
	if total <= 0 {
		return 0, 0, 0, 0
	}
	return total, used, free, (float64(used) / float64(total)) * 100
}

func unraidDiskCapacity(disk models.HostUnraidDisk) (int64, int64, int64, float64) {
	used := disk.UsedBytes
	free := disk.FreeBytes
	total := used + free
	if total <= 0 {
		total = disk.SizeBytes
	}
	percent := float64(0)
	if total > 0 && used > 0 {
		percent = (float64(used) / float64(total)) * 100
	}
	return total, used, free, percent
}

func unraidDiskCapacityForHost(host models.Host, disk models.HostUnraidDisk) (int64, int64, int64, float64) {
	total, used, free, percent := unraidDiskCapacity(disk)
	if used <= 0 && free <= 0 {
		total = unraidDiskSizeBytes(host, disk)
	}
	if total > 0 && used > 0 {
		percent = (float64(used) / float64(total)) * 100
	}
	return total, used, free, percent
}

func unraidDiskSizeBytes(host models.Host, disk models.HostUnraidDisk) int64 {
	size := disk.SizeBytes
	if size <= 0 {
		return 0
	}
	if !shouldScaleLegacyUnraidKiBSize(host, disk, size) {
		return size
	}
	if size > maxInt64()/1024 {
		return size
	}
	return size * 1024
}

func shouldScaleLegacyUnraidKiBSize(host models.Host, disk models.HostUnraidDisk, size int64) bool {
	const maxPlausibleLegacyKiBValue = int64(100_000_000_000)
	if size <= 0 || size >= maxPlausibleLegacyKiBValue {
		return false
	}

	if total := disk.UsedBytes + disk.FreeBytes; legacyKiBMatchesTotal(total, size) {
		return true
	}

	role := unraidDiskRole(&disk)
	switch role {
	case "data", "parity":
		arrayTotal := unraidArrayMountTotal(host)
		if arrayTotal <= 0 || host.Unraid == nil {
			return false
		}
		var rawDataTotal int64
		for _, candidate := range host.Unraid.Disks {
			if unraidDiskRole(&candidate) != "data" || candidate.SizeBytes <= 0 {
				continue
			}
			rawDataTotal += candidate.SizeBytes
		}
		return legacyKiBMatchesTotal(arrayTotal, rawDataTotal)
	case "cache":
		return legacyKiBMatchesTotal(unraidDiskMountTotal(host, disk), size)
	default:
		return false
	}
}

func legacyKiBMatchesTotal(total int64, rawKiB int64) bool {
	if total <= 0 || rawKiB <= 0 || rawKiB > maxInt64()/1024 {
		return false
	}
	scaled := rawKiB * 1024
	lower := scaled - scaled/20
	upper := scaled + scaled/20
	return total >= lower && total <= upper
}

func unraidArrayMountTotal(host models.Host) int64 {
	for _, disk := range host.Disks {
		mount := strings.TrimSpace(disk.Mountpoint)
		if (mount == "/mnt/user" || mount == "/mnt/user0") && disk.Total > 0 {
			return disk.Total
		}
	}
	return 0
}

func unraidDiskMountTotal(host models.Host, disk models.HostUnraidDisk) int64 {
	mountPath := unraidDiskMountPath(disk)
	if mountPath == "" {
		return 0
	}
	for _, mounted := range host.Disks {
		if strings.TrimSpace(mounted.Mountpoint) == mountPath && mounted.Total > 0 {
			return mounted.Total
		}
	}
	return 0
}

func maxInt64() int64 {
	return int64(^uint64(0) >> 1)
}

func matchUnraidDisk(unraid *models.HostUnraidStorage, disk models.HostDiskSMART) *models.HostUnraidDisk {
	if unraid == nil || len(unraid.Disks) == 0 {
		return nil
	}

	normalizedDevice := strings.ToLower(normalizePhysicalDiskDeviceToken(disk.Device))
	normalizedSerial := strings.TrimSpace(strings.ToLower(disk.Serial))
	for i := range unraid.Disks {
		candidate := &unraid.Disks[i]
		if normalizedSerial != "" && strings.EqualFold(strings.TrimSpace(candidate.Serial), normalizedSerial) {
			return candidate
		}
		candidateDevice := strings.ToLower(normalizePhysicalDiskDeviceToken(candidate.Device))
		if normalizedDevice != "" && candidateDevice != "" && candidateDevice == normalizedDevice {
			return candidate
		}
	}
	return nil
}

func unraidDiskRole(disk *models.HostUnraidDisk) string {
	if disk == nil {
		return ""
	}
	role := strings.ToLower(strings.TrimSpace(disk.Role))
	if role != "" {
		return role
	}
	name := strings.ToLower(strings.TrimSpace(disk.Name))
	switch {
	case strings.HasPrefix(name, "parity"):
		return "parity"
	case strings.HasPrefix(name, "cache"):
		return "cache"
	case strings.HasPrefix(name, "disk"), strings.HasPrefix(name, "md"), disk.Slot > 0:
		return "data"
	default:
		return ""
	}
}

func unraidDiskGroup(disk *models.HostUnraidDisk) string {
	if disk == nil {
		return ""
	}
	role := unraidDiskRole(disk)
	switch role {
	case "parity", "data":
		return "unraid-array"
	case "cache":
		return unraidCachePoolName(*disk)
	default:
		return ""
	}
}

func unraidDiskState(disk *models.HostUnraidDisk) string {
	if disk == nil {
		return ""
	}
	return strings.TrimSpace(disk.Status)
}

func unraidDiskCounter(disk *models.HostUnraidDisk, counter string) int64 {
	if disk == nil {
		return 0
	}
	switch counter {
	case "read":
		return disk.ReadCount
	case "write":
		return disk.WriteCount
	case "error":
		return disk.ErrorCount
	default:
		return 0
	}
}

func unraidDiskDiskType(disk models.HostUnraidDisk) string {
	transport := strings.ToLower(strings.TrimSpace(disk.Transport))
	switch transport {
	case "ata":
		return "sata"
	case "nvme", "sata", "sas", "ssd", "hdd", "usb":
		return transport
	default:
		return transport
	}
}

func unraidPhysicalDiskHealth(disk models.HostUnraidDisk) string {
	switch strings.ToLower(strings.TrimSpace(disk.Status)) {
	case "online":
		return "PASSED"
	case "disabled", "invalid", "missing", "wrong", "error":
		return "FAILED"
	default:
		return "UNKNOWN"
	}
}

func assessUnraidPhysicalDisk(disk models.HostUnraidDisk) storagehealth.Assessment {
	assessment := storagehealth.AssessSample(storagehealth.Sample{
		Model:       disk.Model,
		Health:      unraidPhysicalDiskHealth(disk),
		Temperature: disk.Temperature,
		Wearout:     -1,
	})
	addReason := func(code string, severity storagehealth.RiskLevel, summary string) {
		if strings.TrimSpace(summary) == "" {
			return
		}
		assessment.Reasons = append(assessment.Reasons, storagehealth.Reason{
			Code:     code,
			Severity: severity,
			Summary:  summary,
		})
		if incidentSeverityRank(severity) > incidentSeverityRank(assessment.Level) {
			assessment.Level = severity
		}
	}
	label := firstNonEmpty(disk.Name, disk.Device, disk.Serial, "disk")
	status := strings.ToUpper(strings.TrimSpace(disk.Status))
	switch strings.ToLower(strings.TrimSpace(disk.Status)) {
	case "", "online":
	default:
		addReason("unraid_disk_state", storagehealth.RiskCritical, "Unraid disk "+label+" is "+status)
	}
	if disk.ErrorCount > 0 {
		addReason("unraid_disk_errors", storagehealth.RiskWarning, "Unraid disk "+label+" reports "+int64Label(disk.ErrorCount)+" error(s)")
	}
	return assessment
}

func int64Label(value int64) string {
	return strconv.FormatInt(value, 10)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func resourceFromDockerHost(host models.DockerHost) (Resource, ResourceIdentity) {
	name := host.Hostname
	if host.CustomDisplayName != "" {
		name = host.CustomDisplayName
	} else if host.DisplayName != "" {
		name = host.DisplayName
	}

	ips, macs := collectInterfaceIDs(host.NetworkInterfaces)

	identity := ResourceIdentity{
		MachineID:    host.MachineID,
		Hostnames:    uniqueStrings([]string{host.Hostname}),
		IPAddresses:  uniqueStrings(ips),
		MACAddresses: uniqueStrings(macs),
	}

	// If this Docker host is part of a Swarm, surface the Swarm cluster as the resource cluster.
	// This drives unified Infrastructure grouping and /api/resources?cluster filtering.
	if host.Swarm != nil {
		clusterName := strings.TrimSpace(host.Swarm.ClusterName)
		if clusterName == "" {
			clusterName = strings.TrimSpace(host.Swarm.ClusterID)
		}
		if clusterName != "" {
			identity.ClusterName = clusterName
			identity.Hostnames = uniqueStrings(append(identity.Hostnames, clusterName+":"+host.Hostname))
		}
	}

	updatesAvailableCount := 0
	var updatesLastCheckedAt time.Time
	for _, container := range host.Containers {
		if container.UpdateStatus == nil {
			continue
		}
		if container.UpdateStatus.UpdateAvailable {
			updatesAvailableCount++
		}
		if container.UpdateStatus.LastChecked.After(updatesLastCheckedAt) {
			updatesLastCheckedAt = container.UpdateStatus.LastChecked
		}
	}
	var updatesLastCheckedPtr *time.Time
	if !updatesLastCheckedAt.IsZero() {
		copied := updatesLastCheckedAt
		updatesLastCheckedPtr = &copied
	}

	docker := &DockerData{
		HostSourceID:          host.ID,
		AgentID:               host.AgentID,
		Hostname:              host.Hostname,
		DisplayName:           host.DisplayName,
		CustomDisplayName:     host.CustomDisplayName,
		MachineID:             host.MachineID,
		Temperature:           host.Temperature,
		Runtime:               host.Runtime,
		RuntimeVersion:        host.RuntimeVersion,
		DockerVersion:         host.DockerVersion,
		OS:                    host.OS,
		KernelVersion:         host.KernelVersion,
		Architecture:          host.Architecture,
		AgentVersion:          host.AgentVersion,
		CPUs:                  host.CPUs,
		TotalMemoryBytes:      host.TotalMemoryBytes,
		UptimeSeconds:         host.UptimeSeconds,
		LoadAverage:           append([]float64(nil), host.LoadAverage...),
		IntervalSeconds:       host.IntervalSeconds,
		NetInRate:             host.NetInRate,
		NetOutRate:            host.NetOutRate,
		DiskReadRate:          host.DiskReadRate,
		DiskWriteRate:         host.DiskWriteRate,
		ContainerCount:        len(host.Containers),
		ImageCount:            len(host.Images),
		VolumeCount:           len(host.Volumes),
		NetworkCount:          len(host.Networks),
		NodeCount:             len(host.Nodes),
		SecretCount:           len(host.Secrets),
		ConfigCount:           len(host.Configs),
		UpdatesAvailableCount: updatesAvailableCount,
		UpdatesLastCheckedAt:  updatesLastCheckedPtr,
		TokenID:               host.TokenID,
		TokenName:             host.TokenName,
		TokenHint:             host.TokenHint,
		TokenLastUsedAt:       host.TokenLastUsedAt,
		Hidden:                host.Hidden,
		PendingUninstall:      host.PendingUninstall,
		IsLegacy:              host.IsLegacy,
		Command:               cloneDockerHostCommandStatus(host.Command),
		Security:              cloneDockerHostSecurity(host.Security),
		IdentityConflict:      cloneDockerHostIdentityConflict(host.IdentityConflict),
		Swarm:                 convertSwarm(host.Swarm),
		NetworkInterfaces:     convertInterfaces(host.NetworkInterfaces),
		Disks:                 convertDisks(host.Disks),
		Containers:            append([]models.DockerContainer(nil), host.Containers...),
		Images:                append([]models.DockerImage(nil), host.Images...),
		Volumes:               append([]models.DockerVolume(nil), host.Volumes...),
		NetworksRaw:           append([]models.DockerNetwork(nil), host.Networks...),
		Services:              append([]models.DockerService(nil), host.Services...),
		Tasks:                 append([]models.DockerTask(nil), host.Tasks...),
		Nodes:                 append([]models.DockerNode(nil), host.Nodes...),
		Secrets:               append([]models.DockerSecret(nil), host.Secrets...),
		Configs:               append([]models.DockerConfig(nil), host.Configs...),
	}
	if host.StorageUsage != nil {
		docker.ImagesUsage = dockerStorageUsageMeta(host.StorageUsage.Images)
		docker.ContainersUsage = dockerStorageUsageMeta(host.StorageUsage.Containers)
		docker.VolumesUsage = dockerStorageUsageMeta(host.StorageUsage.Volumes)
		docker.BuildCacheUsage = dockerStorageUsageMeta(host.StorageUsage.BuildCache)
	}

	metrics := metricsFromDockerHost(host)

	resource := Resource{
		Type:        ResourceTypeAgent,
		Technology:  strings.TrimSpace(host.Runtime),
		Name:        name,
		Status:      statusFromString(host.Status),
		LastSeen:    host.LastSeen,
		UpdatedAt:   time.Now().UTC(),
		Metrics:     metrics,
		Uptime:      host.UptimeSeconds,
		Temperature: host.Temperature,
		Docker:      docker,
		Tags:        nil,
	}

	return resource, identity
}

func resourceFromPBSInstance(instance models.PBSInstance) (Resource, ResourceIdentity) {
	name := instance.Name
	if strings.TrimSpace(name) == "" {
		name = extractHostname(instance.Host)
	}

	assessments := make([]storagehealth.Assessment, 0, len(instance.Datastores))
	for _, datastore := range instance.Datastores {
		assessments = append(assessments, storagehealth.AssessPBSDatastore(datastore))
	}
	storageAssessment := storagehealth.SummarizeAssessments(assessments...)
	storageRisk := storageRiskFromAssessment(storageAssessment)
	incidents := incidentsFromAssessment("pulse", string(SourcePBS), "pbs-instance:"+name, storageAssessment, instance.LastSeen)
	status := statusFromPBSInstance(instance)
	status = storageStatus(status, storageRisk)
	status = incidentsStatus(status, incidents)

	resource := Resource{
		Type:      ResourceTypePBS,
		Name:      name,
		Status:    status,
		LastSeen:  instance.LastSeen,
		UpdatedAt: time.Now().UTC(),
		Metrics:   metricsFromPBSInstance(instance),
		Uptime:    instance.Uptime,
		CustomURL: instance.GuestURL,
		Incidents: incidents,
		PBS: &PBSData{
			InstanceID:             instance.ID,
			Hostname:               extractHostname(instance.Host),
			HostURL:                instance.Host,
			GuestURL:               instance.GuestURL,
			Version:                instance.Version,
			UptimeSeconds:          instance.Uptime,
			DatastoreCount:         len(instance.Datastores),
			DatastoreDetails:       clonePBSDatastores(instance.Datastores),
			BackupJobCount:         len(instance.BackupJobs),
			BackupJobs:             append([]models.PBSBackupJob(nil), instance.BackupJobs...),
			SyncJobCount:           len(instance.SyncJobs),
			SyncJobs:               append([]models.PBSSyncJob(nil), instance.SyncJobs...),
			VerifyJobCount:         len(instance.VerifyJobs),
			VerifyJobs:             append([]models.PBSVerifyJob(nil), instance.VerifyJobs...),
			PruneJobCount:          len(instance.PruneJobs),
			PruneJobs:              append([]models.PBSPruneJob(nil), instance.PruneJobs...),
			GarbageJobCount:        len(instance.GarbageJobs),
			GarbageJobs:            append([]models.PBSGarbageJob(nil), instance.GarbageJobs...),
			JobHealthEvidenceCount: len(instance.JobHealthEvidence),
			JobHealthEvidence:      append([]models.PBSJobHealthEvidence(nil), instance.JobHealthEvidence...),
			StorageRisk:            storageRisk,
			ConnectionHealth:       instance.ConnectionHealth,
		},
	}

	if len(instance.Datastores) > 0 {
		datastores := make([]PBSDatastoreMeta, len(instance.Datastores))
		for i, ds := range instance.Datastores {
			datastores[i] = PBSDatastoreMeta{
				Name:                ds.Name,
				Total:               ds.Total,
				Used:                ds.Used,
				Available:           ds.Free,
				UsagePercent:        ds.Usage,
				Status:              ds.Status,
				Error:               ds.Error,
				DeduplicationFactor: ds.DeduplicationFactor,
			}
		}
		resource.PBS.Datastores = datastores
	}

	identity := ResourceIdentity{
		Hostnames: uniqueStrings([]string{
			instance.Name,
			extractHostname(instance.Host),
		}),
	}
	return resource, identity
}

func resourceFromPBSDatastore(instance models.PBSInstance, datastore models.PBSDatastore) (Resource, ResourceIdentity) {
	name := strings.TrimSpace(datastore.Name)
	assessment := storagehealth.AssessPBSDatastore(datastore)
	risk := storageRiskFromAssessment(assessment)
	status := storageStatus(statusFromString(datastore.Status), risk)
	incidents := incidentsFromAssessment("pulse", string(SourcePBS), "pbs-datastore:"+name, assessment, instance.LastSeen)
	status = incidentsStatus(status, incidents)

	resource := Resource{
		Type:      ResourceTypeStorage,
		Name:      name,
		Status:    status,
		LastSeen:  instance.LastSeen,
		UpdatedAt: time.Now().UTC(),
		Metrics:   metricsFromPBSDatastore(datastore),
		Storage: &StorageMeta{
			Type:         "pbs-datastore",
			Platform:     "pbs",
			Topology:     "datastore",
			Protection:   "backup-repository",
			Content:      "backup",
			ContentTypes: []string{"backup"},
			Enabled:      true,
			Active:       true,
			Risk:         risk,
		},
		Incidents: incidents,
		Tags: []string{
			"pbs",
			"datastore",
			"backup",
		},
	}

	identity := ResourceIdentity{
		Hostnames: uniqueStrings([]string{
			name,
			instance.Name,
			extractHostname(instance.Host),
		}),
	}
	return resource, identity
}

func resourceFromPMGInstance(instance models.PMGInstance) (Resource, ResourceIdentity) {
	name := instance.Name
	if strings.TrimSpace(name) == "" {
		name = extractHostname(instance.Host)
	}
	uptime := maxPMGUptime(instance.Nodes)
	resource := Resource{
		Type:      ResourceTypePMG,
		Name:      name,
		Status:    statusFromPMGInstance(instance),
		LastSeen:  instance.LastSeen,
		UpdatedAt: time.Now().UTC(),
		Metrics:   metricsFromPMGInstance(instance),
		Uptime:    uptime,
		CustomURL: instance.GuestURL,
		PMG: &PMGData{
			InstanceID:       instance.ID,
			Hostname:         extractHostname(instance.Host),
			HostURL:          strings.TrimSpace(instance.Host),
			GuestURL:         strings.TrimSpace(instance.GuestURL),
			Version:          instance.Version,
			NodeCount:        len(instance.Nodes),
			UptimeSeconds:    uptime,
			ConnectionHealth: instance.ConnectionHealth,
			LastUpdated:      instance.LastUpdated,
		},
	}
	if instance.MailStats != nil {
		resource.PMG.MailCountTotal = instance.MailStats.CountTotal
		resource.PMG.SpamIn = instance.MailStats.SpamIn
		resource.PMG.VirusIn = instance.MailStats.VirusIn
	}
	queue := aggregatePMGQueue(instance.Nodes)
	resource.PMG.QueueActive = queue.Active
	resource.PMG.QueueDeferred = queue.Deferred
	resource.PMG.QueueHold = queue.Hold
	resource.PMG.QueueIncoming = queue.Incoming
	resource.PMG.QueueTotal = queue.Total

	// Populate per-node data
	if len(instance.Nodes) > 0 {
		nodes := make([]PMGNodeMeta, len(instance.Nodes))
		for i, n := range instance.Nodes {
			nodes[i] = PMGNodeMeta{
				Name:    n.Name,
				Status:  n.Status,
				Role:    n.Role,
				Uptime:  n.Uptime,
				LoadAvg: n.LoadAvg,
			}
			if n.QueueStatus != nil {
				nodes[i].QueueStatus = &PMGQueueMeta{
					Active:    n.QueueStatus.Active,
					Deferred:  n.QueueStatus.Deferred,
					Hold:      n.QueueStatus.Hold,
					Incoming:  n.QueueStatus.Incoming,
					Total:     n.QueueStatus.Total,
					OldestAge: n.QueueStatus.OldestAge,
					UpdatedAt: n.QueueStatus.UpdatedAt,
				}
			}
		}
		resource.PMG.Nodes = nodes
	}

	// Populate mail stats
	if instance.MailStats != nil {
		resource.PMG.MailStats = &PMGMailStatsMeta{
			Timeframe:            instance.MailStats.Timeframe,
			CountTotal:           instance.MailStats.CountTotal,
			CountIn:              instance.MailStats.CountIn,
			CountOut:             instance.MailStats.CountOut,
			SpamIn:               instance.MailStats.SpamIn,
			SpamOut:              instance.MailStats.SpamOut,
			VirusIn:              instance.MailStats.VirusIn,
			VirusOut:             instance.MailStats.VirusOut,
			BouncesIn:            instance.MailStats.BouncesIn,
			BouncesOut:           instance.MailStats.BouncesOut,
			BytesIn:              instance.MailStats.BytesIn,
			BytesOut:             instance.MailStats.BytesOut,
			GreylistCount:        instance.MailStats.GreylistCount,
			JunkIn:               instance.MailStats.JunkIn,
			RBLRejects:           instance.MailStats.RBLRejects,
			AverageProcessTimeMs: instance.MailStats.AverageProcessTimeMs,
			PregreetRejects:      instance.MailStats.PregreetRejects,
			UpdatedAt:            instance.MailStats.UpdatedAt,
		}
	}

	if len(instance.MailCount) > 0 {
		resource.PMG.MailCount = append([]models.PMGMailCountPoint(nil), instance.MailCount...)
	}

	// Populate quarantine
	if instance.Quarantine != nil {
		resource.PMG.Quarantine = &PMGQuarantineMeta{
			Spam:        instance.Quarantine.Spam,
			Virus:       instance.Quarantine.Virus,
			Attachment:  instance.Quarantine.Attachment,
			Blacklisted: instance.Quarantine.Blacklisted,
		}
	}

	// Populate spam distribution
	if len(instance.SpamDistribution) > 0 {
		buckets := make([]PMGSpamBucketMeta, len(instance.SpamDistribution))
		for i, b := range instance.SpamDistribution {
			buckets[i] = PMGSpamBucketMeta{
				Bucket: b.Score,
				Count:  b.Count,
			}
		}
		resource.PMG.SpamDistribution = buckets
	}

	// Populate relay domains
	if len(instance.RelayDomains) > 0 {
		relayDomains := make([]PMGRelayDomainMeta, len(instance.RelayDomains))
		for i, d := range instance.RelayDomains {
			relayDomains[i] = PMGRelayDomainMeta{
				Domain:  strings.TrimSpace(d.Domain),
				Comment: strings.TrimSpace(d.Comment),
			}
		}
		resource.PMG.RelayDomains = relayDomains
	}

	// Populate domain stats
	if len(instance.DomainStats) > 0 {
		stats := make([]PMGDomainStatMeta, len(instance.DomainStats))
		for i, s := range instance.DomainStats {
			stats[i] = PMGDomainStatMeta{
				Domain:     strings.TrimSpace(s.Domain),
				MailCount:  s.MailCount,
				SpamCount:  s.SpamCount,
				VirusCount: s.VirusCount,
				Bytes:      s.Bytes,
			}
		}
		resource.PMG.DomainStats = stats
		resource.PMG.DomainStatsAsOf = instance.DomainStatsAsOf
	}

	identity := ResourceIdentity{
		Hostnames: uniqueStrings([]string{
			instance.Name,
			extractHostname(instance.Host),
		}),
	}
	return resource, identity
}

func resourceFromVM(vm models.VM) (Resource, ResourceIdentity) {
	sourceID := proxmoxVMSourceID(vm)
	metrics := metricsFromVM(vm)
	proxmox := &ProxmoxData{
		SourceID:           sourceID,
		NodeName:           vm.Node,
		Pool:               vm.Pool,
		Instance:           vm.Instance,
		VMID:               vm.VMID,
		CPUs:               vm.CPUs,
		Uptime:             vm.Uptime,
		Template:           vm.Template,
		LastBackup:         vm.LastBackup,
		DiskStatusReason:   vm.DiskStatusReason,
		GuestAgentStatus:   vm.GuestAgentStatus,
		GuestAgentExpected: vm.GuestAgentExpected,
		OSName:             vm.OSName,
		OSVersion:          vm.OSVersion,
		AgentVersion:       vm.AgentVersion,
		NetworkInterfaces:  convertGuestInterfaces(vm.NetworkInterfaces),
		Disks:              convertDisks(vm.Disks),
		SwapUsed:           vm.Memory.SwapUsed,
		SwapTotal:          vm.Memory.SwapTotal,
		Balloon:            vm.Memory.Balloon,
		MemoryCache:        vm.Memory.Cache,
		Lock:               vm.Lock,
	}
	resource := Resource{
		Type:       ResourceTypeVM,
		Technology: "qemu",
		Name:       vm.Name,
		Status:     statusFromGuest(vm.Status),
		LastSeen:   vm.LastSeen,
		UpdatedAt:  time.Now().UTC(),
		Metrics:    metrics,
		Proxmox:    proxmox,
		Tags:       vm.Tags,
	}
	if incident, ok := vmGuestAgentIncident(vm); ok {
		resource.Incidents = append(resource.Incidents, incident)
		resource.Status = incidentsStatus(resource.Status, resource.Incidents)
	}
	resource.Capabilities = proxmoxGuestLifecycleCapabilities("vm", "qemu", vm.Status, vm.Template, vm.Lock)
	identity := ResourceIdentity{
		Hostnames:   uniqueStrings([]string{vm.Name}),
		IPAddresses: uniqueStrings(vm.IPAddresses),
	}
	return resource, identity
}

func vmGuestAgentIncident(vm models.VM) (ResourceIncident, bool) {
	if !vm.GuestAgentExpected || vm.GuestAgentStatus != "expected-unreachable" || !strings.EqualFold(strings.TrimSpace(vm.Status), "running") {
		return ResourceIncident{}, false
	}
	return ResourceIncident{
		Provider:  "proxmox",
		NativeID:  proxmoxVMSourceID(vm),
		Code:      "availability_unreachable",
		Severity:  storagehealth.RiskWarning,
		Source:    "qemu-guest-agent",
		Summary:   "QEMU guest agent stopped responding while the VM is still running",
		StartedAt: vm.LastSeen,
	}, true
}

func resourceFromContainer(ct models.Container) (Resource, ResourceIdentity) {
	sourceID := proxmoxContainerSourceID(ct)
	metrics := metricsFromContainer(ct)
	proxmox := &ProxmoxData{
		SourceID:          sourceID,
		NodeName:          ct.Node,
		Pool:              ct.Pool,
		Instance:          ct.Instance,
		VMID:              ct.VMID,
		ContainerType:     ct.Type,
		IsOCI:             ct.IsOCI,
		CPUs:              ct.CPUs,
		Uptime:            ct.Uptime,
		Template:          ct.Template,
		LastBackup:        ct.LastBackup,
		OSName:            ct.OSName,
		NetworkInterfaces: convertGuestInterfaces(ct.NetworkInterfaces),
		OSTemplate:        ct.OSTemplate,
		HasDocker:         ct.HasDocker,
		DockerCheckedAt:   timePtr(ct.DockerCheckedAt),
		Disks:             convertDisks(ct.Disks),
		SwapUsed:          ct.Memory.SwapUsed,
		SwapTotal:         ct.Memory.SwapTotal,
		Balloon:           ct.Memory.Balloon,
		MemoryCache:       ct.Memory.Cache,
		Lock:              ct.Lock,
	}
	resource := Resource{
		Type:       ResourceTypeSystemContainer,
		Technology: "lxc",
		Name:       ct.Name,
		Status:     statusFromGuest(ct.Status),
		LastSeen:   ct.LastSeen,
		UpdatedAt:  time.Now().UTC(),
		Metrics:    metrics,
		Proxmox:    proxmox,
		Tags:       ct.Tags,
	}
	resource.Capabilities = proxmoxGuestLifecycleCapabilities("ct", "lxc", ct.Status, ct.Template, ct.Lock)
	identity := ResourceIdentity{
		Hostnames:   uniqueStrings([]string{ct.Name}),
		IPAddresses: uniqueStrings(ct.IPAddresses),
	}
	return resource, identity
}

func proxmoxGuestLifecycleCapabilities(kind, platform, status string, template bool, lock string) []ResourceCapability {
	if template || strings.TrimSpace(lock) != "" {
		return nil
	}
	handler := ""
	subject := ""
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "vm", "qemu":
		handler = "proxmox.vm.lifecycle"
		subject = "VM"
	case "ct", "lxc", "container":
		handler = "proxmox.ct.lifecycle"
		subject = "LXC"
	default:
		return nil
	}

	status = strings.ToLower(strings.TrimSpace(status))
	operations := []string{}
	switch status {
	case "running", "online":
		operations = []string{"shutdown", "reboot", "stop"}
	case "stopped", "offline":
		operations = []string{"start"}
	default:
		return nil
	}

	capabilities := make([]ResourceCapability, 0, len(operations))
	for _, operation := range operations {
		capabilities = append(capabilities, ResourceCapability{
			Name:                 operation,
			Type:                 CapabilityTypeCommon,
			Description:          proxmoxGuestLifecycleDescription(subject, operation),
			MinimumApprovalLevel: ApprovalAdmin,
			Platform:             strings.TrimSpace(platform),
			InternalHandler:      handler,
		})
	}
	return capabilities
}

func proxmoxGuestLifecycleDescription(subject, operation string) string {
	switch operation {
	case "start":
		return "Start this Proxmox " + subject
	case "shutdown":
		return "Gracefully shut down this Proxmox " + subject
	case "reboot":
		return "Reboot this Proxmox " + subject
	case "stop":
		return "Hard stop this Proxmox " + subject
	default:
		return "Run Proxmox " + subject + " lifecycle action"
	}
}

func convertGuestInterfaces(in []models.GuestNetworkInterface) []NetworkInterface {
	if len(in) == 0 {
		return nil
	}

	out := make([]NetworkInterface, 0, len(in))
	for _, iface := range in {
		out = append(out, NetworkInterface{
			Name:      strings.TrimSpace(iface.Name),
			MAC:       strings.TrimSpace(iface.MAC),
			Addresses: uniqueStrings(iface.Addresses),
			RXBytes:   uint64(max(0, iface.RXBytes)),
			TXBytes:   uint64(max(0, iface.TXBytes)),
		})
	}
	return out
}

func timePtr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	copy := t
	return &copy
}

func resourceFromStorage(storage models.Storage) (Resource, ResourceIdentity) {
	name := strings.TrimSpace(storage.Name)
	if name == "" {
		name = strings.TrimSpace(storage.ID)
	}
	storageType := strings.ToLower(strings.TrimSpace(storage.Type))
	content := strings.TrimSpace(storage.Content)
	now := time.Now().UTC()

	zfsPoolState := ""
	var zfsReadErrors, zfsWriteErrors, zfsChecksumErrors int64
	var zfsPool *models.ZFSPool
	if storage.ZFSPool != nil {
		normalized := storage.ZFSPool.NormalizeCollections()
		zfsPool = &normalized
		zfsPoolState = strings.TrimSpace(storage.ZFSPool.State)
		zfsReadErrors = storage.ZFSPool.ReadErrors
		zfsWriteErrors = storage.ZFSPool.WriteErrors
		zfsChecksumErrors = storage.ZFSPool.ChecksumErrors
	}

	resource := Resource{
		Type:      ResourceTypeStorage,
		Name:      name,
		Status:    statusFromStorage(storage),
		LastSeen:  storage.LastSeen,
		UpdatedAt: now,
		Metrics:   metricsFromStorage(storage),
		Proxmox: &ProxmoxData{
			SourceID: storage.ID,
			NodeName: storage.Node,
			Instance: storage.Instance,
		},
		Storage: &StorageMeta{
			Type:              storageType,
			Content:           content,
			ContentTypes:      parseStorageContentTypes(content),
			Shared:            storage.Shared,
			Enabled:           storage.Enabled,
			Active:            storage.Active,
			IsCeph:            isCephStorageType(storageType),
			IsZFS:             isZFSStorageType(storageType) || storage.ZFSPool != nil,
			Nodes:             append([]string(nil), storage.Nodes...),
			Pool:              storage.Pool,
			Path:              storage.Path,
			ZFSPool:           zfsPool,
			ZFSPoolState:      zfsPoolState,
			ZFSReadErrors:     zfsReadErrors,
			ZFSWriteErrors:    zfsWriteErrors,
			ZFSChecksumErrors: zfsChecksumErrors,
		},
	}
	if storage.ZFSPool != nil {
		assessment := storagehealth.AssessZFSPool(*storage.ZFSPool)
		resource.Storage.Risk = storageRiskFromAssessment(assessment)
		resource.Status = storageStatus(resource.Status, resource.Storage.Risk)
	}

	identity := ResourceIdentity{
		Hostnames: uniqueStrings([]string{
			storage.Name,
			storage.Node,
		}),
	}
	return resource, identity
}

func resourceFromCephCluster(cluster models.CephCluster) (Resource, ResourceIdentity) {
	name := cluster.Name
	if name == "" {
		name = cluster.FSID
	}
	if name == "" {
		name = cluster.ID
	}

	cephMeta := &CephMeta{
		FSID:          cluster.FSID,
		HealthStatus:  cluster.Health,
		HealthMessage: cluster.HealthMessage,
		NumMons:       cluster.NumMons,
		NumMgrs:       cluster.NumMgrs,
		NumOSDs:       cluster.NumOSDs,
		NumOSDsUp:     cluster.NumOSDsUp,
		NumOSDsIn:     cluster.NumOSDsIn,
		NumPGs:        cluster.NumPGs,
		Pools:         convertCephPools(cluster.Pools),
		Services:      convertCephServices(cluster.Services),
	}

	resource := Resource{
		Type:      ResourceTypeCeph,
		Name:      name,
		Status:    statusFromCephHealth(cluster.Health),
		LastSeen:  cluster.LastUpdated,
		UpdatedAt: time.Now().UTC(),
		Metrics:   metricsFromCephCluster(cluster),
		Ceph:      cephMeta,
		Tags:      cephClusterTags(cluster),
	}

	identity := ResourceIdentity{}
	if cluster.FSID != "" {
		identity.MachineID = cluster.FSID
	}
	identity.Hostnames = uniqueStrings([]string{cluster.Name, cluster.Instance})

	return resource, identity
}

func convertCephPools(pools []models.CephPool) []CephPoolMeta {
	if len(pools) == 0 {
		return nil
	}
	out := make([]CephPoolMeta, 0, len(pools))
	for _, p := range pools {
		out = append(out, CephPoolMeta{
			Name:           p.Name,
			StoredBytes:    p.StoredBytes,
			AvailableBytes: p.AvailableBytes,
			Objects:        p.Objects,
			PercentUsed:    p.PercentUsed,
		})
	}
	return out
}

func convertCephServices(services []models.CephServiceStatus) []CephServiceMeta {
	if len(services) == 0 {
		return nil
	}
	out := make([]CephServiceMeta, 0, len(services))
	for _, s := range services {
		out = append(out, CephServiceMeta{
			Type:    s.Type,
			Running: s.Running,
			Total:   s.Total,
		})
	}
	return out
}

func cephClusterTags(cluster models.CephCluster) []string {
	var tags []string
	tags = append(tags, "ceph")
	if cluster.Health != "" {
		tags = append(tags, strings.ToLower(cluster.Health))
	}
	if cluster.Instance != "" {
		tags = append(tags, cluster.Instance)
	}
	return uniqueStrings(tags)
}

func resourceFromPhysicalDisk(disk models.PhysicalDisk) (Resource, ResourceIdentity) {
	name := disk.Model
	if name == "" {
		name = disk.DevPath
	}
	assessment := storagehealth.AssessPhysicalDisk(disk)

	pdMeta := &PhysicalDiskMeta{
		DevPath:      disk.DevPath,
		Model:        disk.Model,
		Vendor:       disk.Vendor,
		Serial:       disk.Serial,
		WWN:          disk.WWN,
		DiskType:     disk.Type,
		Controller:   disk.Controller,
		Target:       disk.Target,
		SizeBytes:    disk.Size,
		Health:       disk.Health,
		Wearout:      disk.Wearout,
		Temperature:  disk.Temperature,
		RPM:          disk.RPM,
		Used:         disk.Used,
		StorageGroup: disk.StorageGroup,
		IO:           physicalDiskIOToMeta(disk.IO),
		Collection:   diskinventory.CloneStatus(disk.Collection),
		Risk:         physicalDiskRiskFromAssessment(assessment),
	}

	if disk.SmartAttributes != nil {
		pdMeta.SMART = convertSMARTAttributes(disk.SmartAttributes)
	}

	resource := Resource{
		Type:         ResourceTypePhysicalDisk,
		Name:         name,
		Status:       physicalDiskStatus(disk.Model, disk.Health, assessment),
		LastSeen:     disk.LastChecked,
		UpdatedAt:    time.Now().UTC(),
		Metrics:      metricsFromPhysicalDisk(disk),
		PhysicalDisk: pdMeta,
		Proxmox: &ProxmoxData{
			SourceID: disk.ID,
			NodeName: disk.Node,
			Instance: disk.Instance,
		},
		Tags: physicalDiskTags(disk),
	}

	identity := ResourceIdentity{
		Hostnames: uniqueStrings([]string{disk.Node}),
	}
	if diskinventory.IsUsableHardwareID(disk.Serial) {
		identity.MachineID = disk.Serial
	} else if diskinventory.IsUsableHardwareID(disk.WWN) {
		identity.MachineID = disk.WWN
	}

	return resource, identity
}

func physicalDiskIOToMeta(in *models.DiskIO) *PhysicalDiskIOMeta {
	if in == nil {
		return nil
	}
	return &PhysicalDiskIOMeta{
		Device:      strings.TrimSpace(in.Device),
		ReadBytes:   in.ReadBytes,
		WriteBytes:  in.WriteBytes,
		ReadOps:     in.ReadOps,
		WriteOps:    in.WriteOps,
		ReadTimeMs:  in.ReadTime,
		WriteTimeMs: in.WriteTime,
		IOTimeMs:    in.IOTime,
	}
}

func convertSMARTAttributes(attrs *models.SMARTAttributes) *SMARTMeta {
	if attrs == nil {
		return nil
	}
	return &SMARTMeta{
		PowerOnHours:         cloneInt64Ptr(attrs.PowerOnHours),
		PowerCycles:          cloneInt64Ptr(attrs.PowerCycles),
		ReallocatedSectors:   cloneInt64Ptr(attrs.ReallocatedSectors),
		PendingSectors:       cloneInt64Ptr(attrs.PendingSectors),
		OfflineUncorrectable: cloneInt64Ptr(attrs.OfflineUncorrectable),
		UDMACRCErrors:        cloneInt64Ptr(attrs.UDMACRCErrors),
		PercentageUsed:       cloneIntPtr(attrs.PercentageUsed),
		AvailableSpare:       cloneIntPtr(attrs.AvailableSpare),
		MediaErrors:          cloneInt64Ptr(attrs.MediaErrors),
		UnsafeShutdowns:      cloneInt64Ptr(attrs.UnsafeShutdowns),
	}
}

func physicalDiskTags(disk models.PhysicalDisk) []string {
	var tags []string
	if disk.Type != "" {
		tags = append(tags, disk.Type)
	}
	if disk.Health != "" {
		tags = append(tags, strings.ToLower(disk.Health))
	}
	if disk.Node != "" {
		tags = append(tags, disk.Node)
	}
	return uniqueStrings(tags)
}

func parseStorageContentTypes(content string) []string {
	if strings.TrimSpace(content) == "" {
		return nil
	}

	parts := strings.Split(content, ",")
	seen := make(map[string]struct{}, len(parts))
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.ToLower(strings.TrimSpace(part))
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func isCephStorageType(storageType string) bool {
	switch strings.ToLower(strings.TrimSpace(storageType)) {
	case "rbd", "cephfs", "ceph":
		return true
	default:
		return false
	}
}

func isZFSStorageType(storageType string) bool {
	normalized := strings.ToLower(strings.TrimSpace(storageType))
	switch normalized {
	case "zfspool", "zfs", "local-zfs":
		return true
	default:
		return strings.Contains(normalized, "zfs")
	}
}

func resourceFromDockerContainer(ct models.DockerContainer, host models.DockerHost) (Resource, ResourceIdentity) {
	metrics := metricsFromDockerContainer(ct, host.CPUs)
	now := time.Now().UTC()
	runtime := strings.TrimSpace(host.Runtime)
	if runtime == "" {
		if ct.Podman != nil {
			runtime = "podman"
		} else {
			runtime = "docker"
		}
	}
	docker := &DockerData{
		HostSourceID:       host.ID,
		AgentID:            strings.TrimSpace(host.AgentID),
		ContainerID:        ct.ID,
		Hostname:           host.Hostname,
		Image:              ct.Image,
		ImageID:            ct.ImageDigest,
		UptimeSeconds:      ct.UptimeSeconds,
		ContainerState:     ct.State,
		Health:             ct.Health,
		RestartCount:       ct.RestartCount,
		ExitCode:           ct.ExitCode,
		OOMKilled:          cloneBoolPtr(ct.OOMKilled),
		CPURawPercent:      ct.CPUPercent,
		CPUCapacityPercent: models.DockerContainerCPUCapacityPercent(ct, host.CPUs),
		CPUCapacityCores:   host.CPUs,
		Labels:             cloneLabelMap(ct.Labels),
		Runtime:            runtime,
		RuntimeVersion:     host.RuntimeVersion,
		DockerVersion:      host.DockerVersion,
		Security:           cloneDockerHostSecurity(host.Security),
	}
	if !ct.CreatedAt.IsZero() {
		docker.CreatedAt = ct.CreatedAt.UTC().Format(time.RFC3339)
	}
	if ct.StartedAt != nil && !ct.StartedAt.IsZero() {
		startedAt := ct.StartedAt.UTC()
		docker.StartedAt = &startedAt
	}
	if ct.FinishedAt != nil && !ct.FinishedAt.IsZero() {
		finishedAt := ct.FinishedAt.UTC()
		docker.FinishedAt = &finishedAt
	}
	if ct.BlockIO != nil && (ct.BlockIO.ReadBytes > 0 || ct.BlockIO.WriteBytes > 0) {
		docker.BlockIO = &DockerContainerBlockIOMeta{
			ReadBytes:  ct.BlockIO.ReadBytes,
			WriteBytes: ct.BlockIO.WriteBytes,
		}
	}
	if ct.Podman != nil {
		docker.Podman = &DockerPodmanContainerMeta{
			PodName:          strings.TrimSpace(ct.Podman.PodName),
			PodID:            strings.TrimSpace(ct.Podman.PodID),
			Infra:            ct.Podman.Infra,
			ComposeProject:   strings.TrimSpace(ct.Podman.ComposeProject),
			ComposeService:   strings.TrimSpace(ct.Podman.ComposeService),
			AutoUpdatePolicy: strings.TrimSpace(ct.Podman.AutoUpdatePolicy),
			UserNamespace:    strings.TrimSpace(ct.Podman.UserNamespace),
		}
	}
	if len(ct.Ports) > 0 {
		docker.Ports = make([]DockerPortMeta, len(ct.Ports))
		for i, p := range ct.Ports {
			docker.Ports[i] = DockerPortMeta{
				PrivatePort: p.PrivatePort,
				PublicPort:  p.PublicPort,
				Protocol:    p.Protocol,
				IP:          p.IP,
			}
		}
	}
	if len(ct.Networks) > 0 {
		docker.Networks = make([]DockerNetworkMeta, len(ct.Networks))
		for i, n := range ct.Networks {
			docker.Networks[i] = DockerNetworkMeta{
				Name: n.Name,
				IPv4: n.IPv4,
				IPv6: n.IPv6,
			}
		}
	}
	if len(ct.Mounts) > 0 {
		docker.Mounts = make([]DockerMountMeta, len(ct.Mounts))
		for i, m := range ct.Mounts {
			docker.Mounts[i] = DockerMountMeta{
				Type:        m.Type,
				Source:      m.Source,
				Destination: m.Destination,
				Mode:        m.Mode,
				RW:          m.RW,
			}
		}
	}
	if ct.UpdateStatus != nil {
		docker.UpdateStatus = &DockerUpdateStatusMeta{
			UpdateAvailable: ct.UpdateStatus.UpdateAvailable,
			CurrentDigest:   ct.UpdateStatus.CurrentDigest,
			LatestDigest:    ct.UpdateStatus.LatestDigest,
			LastChecked:     ct.UpdateStatus.LastChecked,
			Error:           ct.UpdateStatus.Error,
		}
	}
	resource := Resource{
		Type:         ResourceTypeAppContainer,
		Technology:   runtime,
		Name:         ct.Name,
		Status:       statusFromDockerState(ct.State),
		LastSeen:     host.LastSeen,
		UpdatedAt:    now,
		Metrics:      metrics,
		Capabilities: dockerContainerLifecycleCapabilities(ct, host, runtime, now),
	}
	resource.Docker = docker
	identity := ResourceIdentity{
		Hostnames: uniqueStrings([]string{ct.Name}),
	}
	return resource, identity
}

func dockerContainerLifecycleCapabilities(ct models.DockerContainer, host models.DockerHost, runtime string, now time.Time) []ResourceCapability {
	runtime = normalizeDockerLifecycleRuntime(runtime, ct.Podman != nil)
	if runtime == "" {
		return nil
	}
	if strings.TrimSpace(host.AgentID) == "" {
		return nil
	}
	if host.Security != nil && host.Security.MutatingCommandsBlocked {
		return nil
	}
	if host.LastSeen.IsZero() {
		return nil
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	if threshold := defaultStaleThresholds[SourceDocker]; threshold > 0 && now.Sub(host.LastSeen.UTC()) > threshold {
		return nil
	}

	var capabilities []ResourceCapability
	switch strings.ToLower(strings.TrimSpace(ct.State)) {
	case "running":
		capabilities = dockerContainerLifecycleCapabilitySet(runtime, "stop", "restart")
	case "created", "exited", "dead", "stopped":
		capabilities = dockerContainerLifecycleCapabilitySet(runtime, "start")
	default:
		return nil
	}
	if ct.UpdateStatus != nil && ct.UpdateStatus.UpdateAvailable && strings.TrimSpace(ct.UpdateStatus.CurrentDigest) != "" {
		capabilities = append(capabilities, dockerContainerUpdateCapability(runtime))
	}
	return capabilities
}

func dockerContainerUpdateCapability(runtime string) ResourceCapability {
	displayRuntime := "Docker"
	if runtime == "podman" {
		displayRuntime = "Podman"
	}
	return ResourceCapability{
		Name:                 "update",
		Type:                 CapabilityTypeCommon,
		Description:          fmt.Sprintf("Update this %s container to its latest image through its reporting Pulse agent, with automatic backup and rollback.", displayRuntime),
		MinimumApprovalLevel: ApprovalAdmin,
		AutoAuthorization:    AutoAuthorizeLowRisk,
		SupportsRollback:     true,
		Platform:             runtime,
		InternalHandler:      "docker.container.update",
	}
}

func normalizeDockerLifecycleRuntime(runtime string, podman bool) string {
	switch strings.ToLower(strings.TrimSpace(runtime)) {
	case "docker":
		return "docker"
	case "podman":
		return "podman"
	case "":
		if podman {
			return "podman"
		}
		return "docker"
	default:
		return ""
	}
}

func dockerContainerLifecycleCapabilitySet(runtime string, names ...string) []ResourceCapability {
	displayRuntime := "Docker"
	if runtime == "podman" {
		displayRuntime = "Podman"
	}

	capabilities := make([]ResourceCapability, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		autoAuthorization := AutoAuthorizeNever
		if name == "restart" {
			autoAuthorization = AutoAuthorizeLowRisk
		}
		capabilities = append(capabilities, ResourceCapability{
			Name:                 name,
			Type:                 CapabilityTypeCommon,
			Description:          fmt.Sprintf("%s this %s container through its reporting Pulse agent.", titleAction(name), displayRuntime),
			MinimumApprovalLevel: ApprovalAdmin,
			AutoAuthorization:    autoAuthorization,
			Platform:             runtime,
			InternalHandler:      "docker.container.lifecycle",
		})
	}
	return capabilities
}

func titleAction(action string) string {
	action = strings.TrimSpace(action)
	if action == "" {
		return "Run"
	}
	return strings.ToUpper(action[:1]) + action[1:]
}

func resourceFromDockerService(service models.DockerService, host models.DockerHost) (Resource, ResourceIdentity) {
	now := time.Now().UTC()

	clusterName := ""
	if host.Swarm != nil {
		clusterName = strings.TrimSpace(host.Swarm.ClusterName)
		if clusterName == "" {
			clusterName = strings.TrimSpace(host.Swarm.ClusterID)
		}
	}

	docker := &DockerData{
		HostSourceID:   host.ID,
		Hostname:       host.Hostname,
		ServiceID:      service.ID,
		Stack:          strings.TrimSpace(service.Stack),
		Image:          strings.TrimSpace(service.Image),
		Mode:           strings.TrimSpace(service.Mode),
		DesiredTasks:   service.DesiredTasks,
		RunningTasks:   service.RunningTasks,
		CompletedTasks: service.CompletedTasks,
		Labels:         cloneLabelMap(service.Labels),
		Swarm:          convertSwarm(host.Swarm),
	}

	if service.UpdateStatus != nil {
		docker.ServiceUpdate = &DockerServiceUpdateMeta{
			State:       strings.TrimSpace(service.UpdateStatus.State),
			Message:     strings.TrimSpace(service.UpdateStatus.Message),
			CompletedAt: service.UpdateStatus.CompletedAt,
		}
	}

	if len(service.EndpointPorts) > 0 {
		ports := make([]DockerServicePortMeta, 0, len(service.EndpointPorts))
		for _, port := range service.EndpointPorts {
			ports = append(ports, DockerServicePortMeta{
				Name:          strings.TrimSpace(port.Name),
				Protocol:      strings.TrimSpace(port.Protocol),
				TargetPort:    port.TargetPort,
				PublishedPort: port.PublishedPort,
				PublishMode:   strings.TrimSpace(port.PublishMode),
			})
		}
		docker.EndpointPorts = ports
	}

	resource := Resource{
		Type:       ResourceTypeDockerService,
		Technology: "docker",
		Name:       strings.TrimSpace(service.Name),
		Status:     statusFromDockerService(service),
		LastSeen:   host.LastSeen,
		UpdatedAt:  now,
		Docker:     docker,
		Tags:       labelsToTags(docker.Labels),
	}

	identity := ResourceIdentity{
		Hostnames: uniqueStrings([]string{
			strings.TrimSpace(service.Name),
		}),
		ClusterName: clusterName,
	}
	if docker.Stack != "" {
		identity.Hostnames = uniqueStrings(append(identity.Hostnames, docker.Stack+":"+strings.TrimSpace(service.Name)))
	}

	return resource, identity
}

func resourceFromDockerImage(image models.DockerImage, host models.DockerHost) (Resource, ResourceIdentity) {
	name := firstNonEmpty(firstDockerImageReference(image), shortDigest(image.ID), image.ID)
	labels := cloneLabelMap(image.Labels)
	docker := &DockerData{
		HostSourceID:    host.ID,
		Hostname:        host.Hostname,
		ImageID:         strings.TrimSpace(image.ID),
		Image:           name,
		RepoTags:        append([]string(nil), image.RepoTags...),
		RepoDigests:     append([]string(nil), image.RepoDigests...),
		SizeBytes:       image.SizeBytes,
		SharedSizeBytes: image.SharedSizeBytes,
		ImageContainers: image.Containers,
		Runtime:         strings.TrimSpace(host.Runtime),
		RuntimeVersion:  strings.TrimSpace(host.RuntimeVersion),
		DockerVersion:   strings.TrimSpace(host.DockerVersion),
		Labels:          labels,
	}
	resource := Resource{
		Type:       ResourceTypeDockerImage,
		Technology: firstNonEmpty(strings.TrimSpace(host.Runtime), "docker"),
		Name:       name,
		Status:     StatusOnline,
		LastSeen:   host.LastSeen,
		UpdatedAt:  time.Now().UTC(),
		Docker:     docker,
		Tags:       labelsToTags(labels),
	}
	identity := ResourceIdentity{
		Hostnames: uniqueStrings([]string{name, host.Hostname + ":" + name}),
	}
	return resource, identity
}

func resourceFromDockerVolume(volume models.DockerVolume, host models.DockerHost) (Resource, ResourceIdentity) {
	name := strings.TrimSpace(volume.Name)
	labels := cloneLabelMap(volume.Labels)
	docker := &DockerData{
		HostSourceID: host.ID,
		Hostname:     host.Hostname,
		VolumeName:   name,
		Driver:       strings.TrimSpace(volume.Driver),
		Mountpoint:   strings.TrimSpace(volume.Mountpoint),
		Scope:        strings.TrimSpace(volume.Scope),
		CreatedAt:    strings.TrimSpace(volume.CreatedAt),
		SizeBytes:    volume.SizeBytes,
		RefCount:     volume.RefCount,
		Runtime:      strings.TrimSpace(host.Runtime),
		Labels:       labels,
		Options:      cloneLabelMap(volume.Options),
	}
	resource := Resource{
		Type:       ResourceTypeDockerVolume,
		Technology: firstNonEmpty(strings.TrimSpace(host.Runtime), "docker"),
		Name:       name,
		Status:     StatusOnline,
		LastSeen:   host.LastSeen,
		UpdatedAt:  time.Now().UTC(),
		Docker:     docker,
		Tags:       labelsToTags(labels),
	}
	identity := ResourceIdentity{
		Hostnames: uniqueStrings([]string{name, host.Hostname + ":" + name}),
	}
	return resource, identity
}

func resourceFromDockerNetwork(network models.DockerNetwork, host models.DockerHost) (Resource, ResourceIdentity) {
	name := strings.TrimSpace(network.Name)
	labels := cloneLabelMap(network.Labels)
	subnets := make([]DockerNetworkSubnetMeta, 0, len(network.Subnets))
	for _, subnet := range network.Subnets {
		subnets = append(subnets, DockerNetworkSubnetMeta{
			Subnet:  strings.TrimSpace(subnet.Subnet),
			Gateway: strings.TrimSpace(subnet.Gateway),
		})
	}
	docker := &DockerData{
		HostSourceID: host.ID,
		Hostname:     host.Hostname,
		NetworkID:    strings.TrimSpace(network.ID),
		Driver:       strings.TrimSpace(network.Driver),
		Scope:        strings.TrimSpace(network.Scope),
		EnableIPv4:   network.EnableIPv4,
		EnableIPv6:   network.EnableIPv6,
		Internal:     network.Internal,
		Attachable:   network.Attachable,
		Ingress:      network.Ingress,
		ConfigOnly:   network.ConfigOnly,
		Runtime:      strings.TrimSpace(host.Runtime),
		Labels:       labels,
		Options:      cloneLabelMap(network.Options),
		Subnets:      subnets,
	}
	resource := Resource{
		Type:       ResourceTypeDockerNetwork,
		Technology: firstNonEmpty(strings.TrimSpace(host.Runtime), "docker"),
		Name:       name,
		Status:     StatusOnline,
		LastSeen:   host.LastSeen,
		UpdatedAt:  time.Now().UTC(),
		Docker:     docker,
		Tags:       labelsToTags(labels),
	}
	identity := ResourceIdentity{
		Hostnames: uniqueStrings([]string{name, host.Hostname + ":" + name}),
	}
	return resource, identity
}

func resourceFromDockerTask(task models.DockerTask, host models.DockerHost) (Resource, ResourceIdentity) {
	name := firstNonEmpty(strings.TrimSpace(task.ServiceName), strings.TrimSpace(task.ContainerName), shortDigest(task.ID), task.ID)
	if task.Slot > 0 && task.ServiceName != "" {
		name = fmt.Sprintf("%s.%d", task.ServiceName, task.Slot)
	}
	docker := &DockerData{
		HostSourceID:   host.ID,
		Hostname:       host.Hostname,
		TaskID:         strings.TrimSpace(task.ID),
		ServiceID:      strings.TrimSpace(task.ServiceID),
		ServiceName:    strings.TrimSpace(task.ServiceName),
		Slot:           task.Slot,
		NodeID:         strings.TrimSpace(task.NodeID),
		NodeName:       strings.TrimSpace(task.NodeName),
		DesiredState:   strings.TrimSpace(task.DesiredState),
		CurrentState:   strings.TrimSpace(task.CurrentState),
		Error:          strings.TrimSpace(task.Error),
		Message:        strings.TrimSpace(task.Message),
		ContainerID:    strings.TrimSpace(task.ContainerID),
		ContainerState: strings.TrimSpace(task.CurrentState),
		StartedAt:      task.StartedAt,
		CompletedAt:    task.CompletedAt,
		Swarm:          convertSwarm(host.Swarm),
	}
	resource := Resource{
		Type:       ResourceTypeDockerTask,
		Technology: "docker",
		Name:       name,
		Status:     statusFromDockerTask(task),
		LastSeen:   host.LastSeen,
		UpdatedAt:  time.Now().UTC(),
		Docker:     docker,
	}
	identity := ResourceIdentity{
		Hostnames:   uniqueStrings([]string{name, strings.TrimSpace(task.ID)}),
		ClusterName: dockerSwarmClusterKeyFromMeta(docker.Swarm),
	}
	return resource, identity
}

func resourceFromDockerSwarmNode(node models.DockerNode, host models.DockerHost) (Resource, ResourceIdentity) {
	name := firstNonEmpty(strings.TrimSpace(node.Hostname), shortDigest(node.ID), node.ID)
	clusterName := dockerSwarmClusterKeyFromMeta(convertSwarm(host.Swarm))
	labels := cloneLabelMap(node.Labels)
	docker := &DockerData{
		HostSourceID:        host.ID,
		Hostname:            strings.TrimSpace(host.Hostname),
		NodeID:              strings.TrimSpace(node.ID),
		NodeName:            strings.TrimSpace(node.Hostname),
		NodeRole:            strings.TrimSpace(node.Role),
		Availability:        strings.TrimSpace(node.Availability),
		CurrentState:        strings.TrimSpace(node.State),
		Message:             strings.TrimSpace(node.Message),
		Address:             strings.TrimSpace(node.Address),
		ManagerReachability: strings.TrimSpace(node.ManagerReachability),
		ManagerAddress:      strings.TrimSpace(node.ManagerAddress),
		Leader:              node.Leader,
		EngineVersion:       strings.TrimSpace(node.EngineVersion),
		RuntimeVersion:      strings.TrimSpace(node.EngineVersion),
		OS:                  strings.TrimSpace(node.OS),
		Architecture:        strings.TrimSpace(node.Architecture),
		NanoCPUs:            node.NanoCPUs,
		MemoryBytes:         node.MemoryBytes,
		Runtime:             "docker",
		Labels:              labels,
		EngineLabels:        cloneLabelMap(node.EngineLabels),
		Swarm:               convertSwarm(host.Swarm),
	}
	resource := Resource{
		Type:       ResourceTypeDockerSwarmNode,
		Technology: "docker",
		Name:       name,
		Status:     statusFromDockerSwarmNode(node),
		LastSeen:   host.LastSeen,
		UpdatedAt:  time.Now().UTC(),
		Docker:     docker,
		Tags:       labelsToTags(labels),
	}
	identity := ResourceIdentity{
		Hostnames: uniqueStrings([]string{name, strings.TrimSpace(node.ID)}),
	}
	if clusterName != "" {
		identity.ClusterName = clusterName
		identity.Hostnames = uniqueStrings(append(identity.Hostnames, clusterName+":"+name))
	}
	return resource, identity
}

func resourceFromDockerSecret(secret models.DockerSecret, host models.DockerHost) (Resource, ResourceIdentity) {
	name := firstNonEmpty(strings.TrimSpace(secret.Name), shortDigest(secret.ID), secret.ID)
	clusterName := dockerSwarmClusterKeyFromMeta(convertSwarm(host.Swarm))
	labels := cloneLabelMap(secret.Labels)
	docker := &DockerData{
		HostSourceID:     host.ID,
		Hostname:         strings.TrimSpace(host.Hostname),
		SecretID:         strings.TrimSpace(secret.ID),
		SecretName:       strings.TrimSpace(secret.Name),
		Driver:           strings.TrimSpace(secret.DriverName),
		TemplatingDriver: strings.TrimSpace(secret.TemplatingDriver),
		ObjectCreatedAt:  timePtr(secret.CreatedAt),
		ObjectUpdatedAt:  secret.UpdatedAt,
		Runtime:          "docker",
		Labels:           labels,
		Swarm:            convertSwarm(host.Swarm),
	}
	resource := Resource{
		Type:       ResourceTypeDockerSecret,
		Technology: "docker",
		Name:       name,
		Status:     StatusOnline,
		LastSeen:   host.LastSeen,
		UpdatedAt:  time.Now().UTC(),
		Docker:     docker,
		Tags:       labelsToTags(labels),
	}
	identity := ResourceIdentity{
		Hostnames: uniqueStrings([]string{name, strings.TrimSpace(secret.ID)}),
	}
	if clusterName != "" {
		identity.ClusterName = clusterName
		identity.Hostnames = uniqueStrings(append(identity.Hostnames, clusterName+":"+name))
	}
	return resource, identity
}

func resourceFromDockerConfig(config models.DockerConfig, host models.DockerHost) (Resource, ResourceIdentity) {
	name := firstNonEmpty(strings.TrimSpace(config.Name), shortDigest(config.ID), config.ID)
	clusterName := dockerSwarmClusterKeyFromMeta(convertSwarm(host.Swarm))
	labels := cloneLabelMap(config.Labels)
	docker := &DockerData{
		HostSourceID:     host.ID,
		Hostname:         strings.TrimSpace(host.Hostname),
		ConfigID:         strings.TrimSpace(config.ID),
		ConfigName:       strings.TrimSpace(config.Name),
		TemplatingDriver: strings.TrimSpace(config.TemplatingDriver),
		ObjectCreatedAt:  timePtr(config.CreatedAt),
		ObjectUpdatedAt:  config.UpdatedAt,
		Runtime:          "docker",
		Labels:           labels,
		Swarm:            convertSwarm(host.Swarm),
	}
	resource := Resource{
		Type:       ResourceTypeDockerConfig,
		Technology: "docker",
		Name:       name,
		Status:     StatusOnline,
		LastSeen:   host.LastSeen,
		UpdatedAt:  time.Now().UTC(),
		Docker:     docker,
		Tags:       labelsToTags(labels),
	}
	identity := ResourceIdentity{
		Hostnames: uniqueStrings([]string{name, strings.TrimSpace(config.ID)}),
	}
	if clusterName != "" {
		identity.ClusterName = clusterName
		identity.Hostnames = uniqueStrings(append(identity.Hostnames, clusterName+":"+name))
	}
	return resource, identity
}

func firstDockerImageReference(image models.DockerImage) string {
	for _, candidate := range image.RepoTags {
		candidate = strings.TrimSpace(candidate)
		if candidate != "" && candidate != "<none>:<none>" {
			return candidate
		}
	}
	for _, candidate := range image.RepoDigests {
		if value := strings.TrimSpace(candidate); value != "" {
			return value
		}
	}
	return ""
}

func shortDigest(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "sha256:")
	if len(value) > 12 {
		return value[:12]
	}
	return value
}

func dockerStorageUsageMeta(bucket models.DockerStorageUsageBucket) *DockerStorageUsageMeta {
	if bucket.TotalCount == 0 && bucket.ActiveCount == 0 && bucket.TotalSizeBytes == 0 && bucket.ReclaimableBytes == 0 {
		return nil
	}
	return &DockerStorageUsageMeta{
		TotalCount:       bucket.TotalCount,
		ActiveCount:      bucket.ActiveCount,
		TotalSizeBytes:   bucket.TotalSizeBytes,
		ReclaimableBytes: bucket.ReclaimableBytes,
	}
}

func resourceFromKubernetesCluster(cluster models.KubernetesCluster, linkedHosts []*models.Host, capabilities *K8sMetricCapabilities) (Resource, ResourceIdentity) {
	clusterName := kubernetesClusterDisplayName(cluster)
	metrics := metricsFromKubernetesCluster(cluster, linkedHosts)
	resource := Resource{
		Type:      ResourceTypeK8sCluster,
		Name:      clusterName,
		Status:    statusFromKubernetesCluster(cluster),
		LastSeen:  cluster.LastSeen,
		UpdatedAt: time.Now().UTC(),
		Metrics:   metrics,
		Kubernetes: &K8sData{
			ClusterID:          cluster.ID,
			ClusterName:        clusterName,
			SourceName:         cluster.Name,
			SourceStatus:       cluster.Status,
			AgentID:            cluster.AgentID,
			Context:            cluster.Context,
			Server:             cluster.Server,
			Version:            cluster.Version,
			PendingUninstall:   cluster.PendingUninstall,
			AgentVersion:       cluster.AgentVersion,
			IntervalSeconds:    cluster.IntervalSeconds,
			MetricCapabilities: cloneKubernetesMetricCapabilities(capabilities),
		},
		Tags: nil,
	}
	identity := ResourceIdentity{
		Hostnames: uniqueStrings([]string{
			cluster.Name,
			cluster.DisplayName,
			cluster.CustomDisplayName,
			cluster.Context,
			extractHostname(cluster.Server),
		}),
		ClusterName: clusterName,
	}
	return resource, identity
}

func resourceFromKubernetesNode(cluster models.KubernetesCluster, node models.KubernetesNode, linkedHost *models.Host, capabilities *K8sMetricCapabilities) (Resource, ResourceIdentity) {
	clusterName := kubernetesClusterDisplayName(cluster)
	metrics := metricsFromKubernetesNode(cluster, node, linkedHost)
	uptimeSeconds := int64(0)
	var temperature *float64
	if linkedHost != nil {
		uptimeSeconds = linkedHost.UptimeSeconds
		temperature = maxCPUTemp(linkedHost.Sensors)
	}
	resource := Resource{
		Type:      ResourceTypeK8sNode,
		Name:      node.Name,
		Status:    statusFromKubernetesNode(node),
		LastSeen:  cluster.LastSeen,
		UpdatedAt: time.Now().UTC(),
		Metrics:   metrics,
		Kubernetes: &K8sData{
			ClusterID:               cluster.ID,
			ClusterName:             clusterName,
			AgentID:                 cluster.AgentID,
			AgentVersion:            cluster.AgentVersion,
			Context:                 cluster.Context,
			Server:                  cluster.Server,
			Version:                 cluster.Version,
			NodeUID:                 node.UID,
			NodeName:                node.Name,
			Ready:                   node.Ready,
			Unschedulable:           node.Unschedulable,
			Roles:                   append([]string(nil), node.Roles...),
			KubeletVersion:          node.KubeletVersion,
			ContainerRuntimeVersion: node.ContainerRuntimeVersion,
			OSImage:                 node.OSImage,
			KernelVersion:           node.KernelVersion,
			Architecture:            node.Architecture,
			CapacityCPU:             node.CapacityCPU,
			CapacityMemoryBytes:     node.CapacityMemoryBytes,
			CapacityPods:            node.CapacityPods,
			AllocCPU:                node.AllocCPU,
			AllocMemoryBytes:        node.AllocMemoryBytes,
			AllocPods:               node.AllocPods,
			UptimeSeconds:           uptimeSeconds,
			Temperature:             temperature,
			MetricCapabilities:      cloneKubernetesMetricCapabilities(capabilities),
		},
		Tags: append([]string(nil), node.Roles...),
	}
	identity := ResourceIdentity{
		Hostnames: uniqueStrings([]string{
			node.Name,
			clusterName + ":" + node.Name,
		}),
		ClusterName: clusterName,
	}
	return resource, identity
}

func resourceFromKubernetesPod(cluster models.KubernetesCluster, pod models.KubernetesPod, capabilities *K8sMetricCapabilities) (Resource, ResourceIdentity) {
	clusterName := kubernetesClusterDisplayName(cluster)
	labels := cloneLabelMap(pod.Labels)
	primaryImage := ""
	now := time.Now().UTC()
	if len(pod.Containers) > 0 {
		primaryImage = pod.Containers[0].Image
	}
	metrics := metricsFromKubernetesPod(cluster, pod)
	resource := Resource{
		Type:       ResourceTypePod,
		Technology: "kubernetes",
		Name:       pod.Name,
		Status:     statusFromKubernetesPod(pod),
		LastSeen:   cluster.LastSeen,
		UpdatedAt:  now,
		Metrics:    metrics,
		Kubernetes: &K8sData{
			ClusterID:     cluster.ID,
			ClusterName:   clusterName,
			ResourceUID:   pod.UID,
			ResourceKind:  "Pod",
			AgentID:       cluster.AgentID,
			Context:       cluster.Context,
			Server:        cluster.Server,
			Version:       cluster.Version,
			Namespace:     pod.Namespace,
			PodUID:        pod.UID,
			NodeName:      pod.NodeName,
			PodPhase:      pod.Phase,
			PodReason:     pod.Reason,
			PodMessage:    pod.Message,
			PodContainers: cloneK8sPodContainers(pod.Containers),
			QoSClass:      strings.TrimSpace(pod.QoSClass),
			UptimeSeconds: func() int64 {
				if pod.StartTime != nil {
					start := pod.StartTime.UTC()
					if !start.IsZero() && !start.After(now) {
						return int64(now.Sub(start).Seconds())
					}
				}
				created := pod.CreatedAt.UTC()
				if !created.IsZero() && !created.After(now) {
					return int64(now.Sub(created).Seconds())
				}
				return 0
			}(),
			Restarts:           pod.Restarts,
			OwnerKind:          pod.OwnerKind,
			OwnerName:          pod.OwnerName,
			Image:              primaryImage,
			Labels:             labels,
			CreatedAt:          zeroTimeToPtr(pod.CreatedAt),
			MetricCapabilities: cloneKubernetesMetricCapabilities(capabilities),
		},
		Tags: labelsToTags(labels),
	}
	identity := ResourceIdentity{
		Hostnames: uniqueStrings([]string{
			pod.Name,
			pod.Namespace + "/" + pod.Name,
		}),
		ClusterName: clusterName,
	}
	return resource, identity
}

func resourceFromKubernetesDeployment(cluster models.KubernetesCluster, deployment models.KubernetesDeployment, capabilities *K8sMetricCapabilities) (Resource, ResourceIdentity) {
	clusterName := kubernetesClusterDisplayName(cluster)
	labels := cloneLabelMap(deployment.Labels)
	resource := Resource{
		Type:      ResourceTypeK8sDeployment,
		Name:      deployment.Name,
		Status:    statusFromKubernetesDeployment(deployment),
		LastSeen:  cluster.LastSeen,
		UpdatedAt: time.Now().UTC(),
		Metrics:   metricsFromKubernetesDeployment(cluster, deployment),
		Kubernetes: &K8sData{
			ClusterID:          cluster.ID,
			ClusterName:        clusterName,
			ResourceUID:        deployment.UID,
			ResourceKind:       "Deployment",
			AgentID:            cluster.AgentID,
			Context:            cluster.Context,
			Server:             cluster.Server,
			Version:            cluster.Version,
			Namespace:          deployment.Namespace,
			DeploymentUID:      deployment.UID,
			DesiredReplicas:    deployment.DesiredReplicas,
			UpdatedReplicas:    deployment.UpdatedReplicas,
			ReadyReplicas:      deployment.ReadyReplicas,
			AvailableReplicas:  deployment.AvailableReplicas,
			ObservedGeneration: deployment.ObservedGeneration,
			Labels:             labels,
			CreatedAt:          zeroTimeToPtr(deployment.CreatedAt),
			MetricCapabilities: cloneKubernetesMetricCapabilities(capabilities),
		},
		Tags: labelsToTags(labels),
	}
	identity := ResourceIdentity{
		Hostnames: uniqueStrings([]string{
			deployment.Name,
			deployment.Namespace + "/" + deployment.Name,
		}),
		ClusterName: clusterName,
	}
	return resource, identity
}

func resourceFromKubernetesReplicaSet(cluster models.KubernetesCluster, replicaSet models.KubernetesReplicaSet, capabilities *K8sMetricCapabilities) (Resource, ResourceIdentity) {
	clusterName := kubernetesClusterDisplayName(cluster)
	labels := cloneLabelMap(replicaSet.Labels)
	data := baseKubernetesData(cluster, clusterName, "ReplicaSet", capabilities)
	data.ReplicaSetUID = replicaSet.UID
	data.ResourceUID = replicaSet.UID
	data.Namespace = replicaSet.Namespace
	data.DesiredReplicas = replicaSet.DesiredReplicas
	data.ReadyReplicas = replicaSet.ReadyReplicas
	data.AvailableReplicas = replicaSet.AvailableReplicas
	data.FullyLabeledReplicas = replicaSet.FullyLabeledReplicas
	data.ObservedGeneration = replicaSet.ObservedGeneration
	data.OwnerKind = replicaSet.OwnerKind
	data.OwnerName = replicaSet.OwnerName
	data.Labels = labels
	return namespacedKubernetesResource(cluster, clusterName, replicaSet.Namespace, replicaSet.Name, ResourceTypeK8sReplicaSet, statusFromKubernetesReplicaSet(replicaSet), data, labels)
}

// namespacedKubernetesResource assembles the Resource scaffold and namespaced
// identity shared by kubernetes adapters once the kind-specific K8sData fields
// are populated.
func namespacedKubernetesResource(cluster models.KubernetesCluster, clusterName, namespace, name string, resourceType ResourceType, status ResourceStatus, data K8sData, labels map[string]string) (Resource, ResourceIdentity) {
	resource := Resource{
		Type:       resourceType,
		Technology: "kubernetes",
		Name:       name,
		Status:     status,
		LastSeen:   cluster.LastSeen,
		UpdatedAt:  time.Now().UTC(),
		Kubernetes: &data,
		Tags:       labelsToTags(labels),
	}
	return resource, namespacedKubernetesIdentity(clusterName, namespace, name)
}

func baseKubernetesData(cluster models.KubernetesCluster, clusterName, resourceKind string, capabilities *K8sMetricCapabilities) K8sData {
	return K8sData{
		ClusterID:          cluster.ID,
		ClusterName:        clusterName,
		ResourceKind:       resourceKind,
		AgentID:            cluster.AgentID,
		Context:            cluster.Context,
		Server:             cluster.Server,
		Version:            cluster.Version,
		MetricCapabilities: cloneKubernetesMetricCapabilities(capabilities),
	}
}

func resourceFromKubernetesNamespace(cluster models.KubernetesCluster, namespace models.KubernetesNamespace, capabilities *K8sMetricCapabilities) (Resource, ResourceIdentity) {
	clusterName := kubernetesClusterDisplayName(cluster)
	labels := cloneLabelMap(namespace.Labels)
	data := baseKubernetesData(cluster, clusterName, "Namespace", capabilities)
	data.NamespaceUID = namespace.UID
	data.ResourceUID = namespace.UID
	data.Namespace = namespace.Name
	data.Phase = namespace.Phase
	data.Labels = labels
	data.CreatedAt = zeroTimeToPtr(namespace.CreatedAt)
	resource := Resource{
		Type:       ResourceTypeK8sNamespace,
		Technology: "kubernetes",
		Name:       namespace.Name,
		Status:     statusFromKubernetesPhase(namespace.Phase),
		LastSeen:   cluster.LastSeen,
		UpdatedAt:  time.Now().UTC(),
		Kubernetes: &data,
		Tags:       labelsToTags(labels),
	}
	identity := ResourceIdentity{
		Hostnames:   uniqueStrings([]string{namespace.Name, clusterName + ":" + namespace.Name}),
		ClusterName: clusterName,
	}
	return resource, identity
}

func resourceFromKubernetesService(cluster models.KubernetesCluster, service models.KubernetesService, capabilities *K8sMetricCapabilities) (Resource, ResourceIdentity) {
	clusterName := kubernetesClusterDisplayName(cluster)
	labels := cloneLabelMap(service.Labels)
	data := baseKubernetesData(cluster, clusterName, "Service", capabilities)
	data.ServiceUID = service.UID
	data.ResourceUID = service.UID
	data.Namespace = service.Namespace
	data.ServiceType = service.ServiceType
	data.ClusterIP = service.ClusterIP
	data.ExternalIPs = append([]string(nil), service.ExternalIPs...)
	data.ServicePorts = k8sServicePorts(service.Ports)
	data.Selector = cloneLabelMap(service.Selector)
	data.Labels = labels
	data.CreatedAt = zeroTimeToPtr(service.CreatedAt)
	return namespacedKubernetesResource(cluster, clusterName, service.Namespace, service.Name, ResourceTypeK8sService, StatusOnline, data, labels)
}

func resourceFromKubernetesStatefulSet(cluster models.KubernetesCluster, statefulSet models.KubernetesStatefulSet, capabilities *K8sMetricCapabilities) (Resource, ResourceIdentity) {
	clusterName := kubernetesClusterDisplayName(cluster)
	labels := cloneLabelMap(statefulSet.Labels)
	data := baseKubernetesData(cluster, clusterName, "StatefulSet", capabilities)
	data.StatefulSetUID = statefulSet.UID
	data.ResourceUID = statefulSet.UID
	data.Namespace = statefulSet.Namespace
	data.DesiredReplicas = statefulSet.DesiredReplicas
	data.ReadyReplicas = statefulSet.ReadyReplicas
	data.CurrentReplicas = statefulSet.CurrentReplicas
	data.UpdatedReplicas = statefulSet.UpdatedReplicas
	data.AvailableReplicas = statefulSet.AvailableReplicas
	data.ServiceName = statefulSet.ServiceName
	data.Labels = labels
	return namespacedKubernetesResource(cluster, clusterName, statefulSet.Namespace, statefulSet.Name, ResourceTypeK8sStatefulSet, statusFromKubernetesStatefulSet(statefulSet), data, labels)
}

func resourceFromKubernetesDaemonSet(cluster models.KubernetesCluster, daemonSet models.KubernetesDaemonSet, capabilities *K8sMetricCapabilities) (Resource, ResourceIdentity) {
	clusterName := kubernetesClusterDisplayName(cluster)
	labels := cloneLabelMap(daemonSet.Labels)
	data := baseKubernetesData(cluster, clusterName, "DaemonSet", capabilities)
	data.DaemonSetUID = daemonSet.UID
	data.ResourceUID = daemonSet.UID
	data.Namespace = daemonSet.Namespace
	data.DesiredNumberScheduled = daemonSet.DesiredNumberScheduled
	data.CurrentNumberScheduled = daemonSet.CurrentNumberScheduled
	data.NumberReady = daemonSet.NumberReady
	data.UpdatedReplicas = daemonSet.UpdatedNumberScheduled
	data.NumberAvailable = daemonSet.NumberAvailable
	data.NumberUnavailable = daemonSet.NumberUnavailable
	data.NumberMisscheduled = daemonSet.NumberMisscheduled
	data.Labels = labels
	return namespacedKubernetesResource(cluster, clusterName, daemonSet.Namespace, daemonSet.Name, ResourceTypeK8sDaemonSet, statusFromKubernetesDaemonSet(daemonSet), data, labels)
}

func resourceFromKubernetesJob(cluster models.KubernetesCluster, job models.KubernetesJob, capabilities *K8sMetricCapabilities) (Resource, ResourceIdentity) {
	clusterName := kubernetesClusterDisplayName(cluster)
	labels := cloneLabelMap(job.Labels)
	data := baseKubernetesData(cluster, clusterName, "Job", capabilities)
	data.JobUID = job.UID
	data.ResourceUID = job.UID
	data.Namespace = job.Namespace
	data.DesiredReplicas = job.DesiredCompletions
	data.Succeeded = job.Succeeded
	data.Failed = job.Failed
	data.Active = job.Active
	data.StartTime = job.StartTime
	data.CompletionTime = job.CompletionTime
	data.Labels = labels
	return namespacedKubernetesResource(cluster, clusterName, job.Namespace, job.Name, ResourceTypeK8sJob, statusFromKubernetesJob(job), data, labels)
}

func resourceFromKubernetesCronJob(cluster models.KubernetesCluster, cronJob models.KubernetesCronJob, capabilities *K8sMetricCapabilities) (Resource, ResourceIdentity) {
	clusterName := kubernetesClusterDisplayName(cluster)
	labels := cloneLabelMap(cronJob.Labels)
	data := baseKubernetesData(cluster, clusterName, "CronJob", capabilities)
	data.CronJobUID = cronJob.UID
	data.ResourceUID = cronJob.UID
	data.Namespace = cronJob.Namespace
	data.Schedule = cronJob.Schedule
	data.Suspend = cronJob.Suspend
	data.Active = int32(cronJob.Active)
	data.LastScheduleTime = cronJob.LastScheduleTime
	data.LastSuccessfulTime = cronJob.LastSuccessfulTime
	data.Labels = labels
	return namespacedKubernetesResource(cluster, clusterName, cronJob.Namespace, cronJob.Name, ResourceTypeK8sCronJob, statusFromKubernetesCronJob(cronJob), data, labels)
}

func resourceFromKubernetesIngress(cluster models.KubernetesCluster, ingress models.KubernetesIngress, capabilities *K8sMetricCapabilities) (Resource, ResourceIdentity) {
	clusterName := kubernetesClusterDisplayName(cluster)
	labels := cloneLabelMap(ingress.Labels)
	data := baseKubernetesData(cluster, clusterName, "Ingress", capabilities)
	data.IngressUID = ingress.UID
	data.ResourceUID = ingress.UID
	data.Namespace = ingress.Namespace
	data.ClassName = ingress.ClassName
	data.Hosts = append([]string(nil), ingress.Hosts...)
	data.Addresses = append([]string(nil), ingress.Addresses...)
	data.Labels = labels
	data.CreatedAt = zeroTimeToPtr(ingress.CreatedAt)
	return namespacedKubernetesResource(cluster, clusterName, ingress.Namespace, ingress.Name, ResourceTypeK8sIngress, StatusOnline, data, labels)
}

func resourceFromKubernetesEndpointSlice(cluster models.KubernetesCluster, slice models.KubernetesEndpointSlice, capabilities *K8sMetricCapabilities) (Resource, ResourceIdentity) {
	clusterName := kubernetesClusterDisplayName(cluster)
	labels := cloneLabelMap(slice.Labels)
	ports := make([]K8sEndpointPort, 0, len(slice.Ports))
	for _, port := range slice.Ports {
		ports = append(ports, K8sEndpointPort{
			Name:        port.Name,
			Protocol:    port.Protocol,
			Port:        port.Port,
			AppProtocol: port.AppProtocol,
		})
	}
	data := baseKubernetesData(cluster, clusterName, "EndpointSlice", capabilities)
	data.EndpointSliceUID = slice.UID
	data.ResourceUID = slice.UID
	data.Namespace = slice.Namespace
	data.AddressType = slice.AddressType
	data.ServiceName = slice.ServiceName
	data.EndpointPorts = ports
	data.EndpointCount = slice.EndpointCount
	data.ReadyEndpointCount = slice.ReadyEndpointCount
	data.Labels = labels
	data.CreatedAt = zeroTimeToPtr(slice.CreatedAt)
	return namespacedKubernetesResource(cluster, clusterName, slice.Namespace, slice.Name, ResourceTypeK8sEndpointSlice, statusFromKubernetesEndpointSlice(slice), data, labels)
}

func resourceFromKubernetesNetworkPolicy(cluster models.KubernetesCluster, policy models.KubernetesNetworkPolicy, capabilities *K8sMetricCapabilities) (Resource, ResourceIdentity) {
	clusterName := kubernetesClusterDisplayName(cluster)
	labels := cloneLabelMap(policy.Labels)
	data := baseKubernetesData(cluster, clusterName, "NetworkPolicy", capabilities)
	data.NetworkPolicyUID = policy.UID
	data.ResourceUID = policy.UID
	data.Namespace = policy.Namespace
	data.PolicyTypes = append([]string(nil), policy.PolicyTypes...)
	data.IngressRuleCount = policy.IngressRuleCount
	data.EgressRuleCount = policy.EgressRuleCount
	data.Labels = labels
	data.CreatedAt = zeroTimeToPtr(policy.CreatedAt)
	return namespacedKubernetesResource(cluster, clusterName, policy.Namespace, policy.Name, ResourceTypeK8sNetworkPolicy, StatusOnline, data, labels)
}

func resourceFromKubernetesPersistentVolume(cluster models.KubernetesCluster, volume models.KubernetesPersistentVolume, capabilities *K8sMetricCapabilities) (Resource, ResourceIdentity) {
	clusterName := kubernetesClusterDisplayName(cluster)
	labels := cloneLabelMap(volume.Labels)
	data := baseKubernetesData(cluster, clusterName, "PersistentVolume", capabilities)
	data.PersistentVolumeUID = volume.UID
	data.ResourceUID = volume.UID
	data.Phase = volume.Phase
	data.StorageClass = volume.StorageClass
	data.CapacityBytes = volume.CapacityBytes
	data.AccessModes = append([]string(nil), volume.AccessModes...)
	data.ReclaimPolicy = volume.ReclaimPolicy
	data.ClaimNamespace = volume.ClaimNamespace
	data.ClaimName = volume.ClaimName
	data.Labels = labels
	data.CreatedAt = zeroTimeToPtr(volume.CreatedAt)
	resource := Resource{
		Type:       ResourceTypeK8sPV,
		Technology: "kubernetes",
		Name:       volume.Name,
		Status:     statusFromKubernetesPhase(volume.Phase),
		LastSeen:   cluster.LastSeen,
		UpdatedAt:  time.Now().UTC(),
		Kubernetes: &data,
		Tags:       labelsToTags(labels),
	}
	identity := ResourceIdentity{
		Hostnames:   uniqueStrings([]string{volume.Name, clusterName + ":" + volume.Name}),
		ClusterName: clusterName,
	}
	return resource, identity
}

func resourceFromKubernetesPersistentVolumeClaim(cluster models.KubernetesCluster, claim models.KubernetesPersistentVolumeClaim, capabilities *K8sMetricCapabilities) (Resource, ResourceIdentity) {
	clusterName := kubernetesClusterDisplayName(cluster)
	labels := cloneLabelMap(claim.Labels)
	data := baseKubernetesData(cluster, clusterName, "PersistentVolumeClaim", capabilities)
	data.PersistentVolumeClaimUID = claim.UID
	data.ResourceUID = claim.UID
	data.Namespace = claim.Namespace
	data.Phase = claim.Phase
	data.StorageClass = claim.StorageClass
	data.RequestedBytes = claim.RequestedBytes
	data.CapacityBytes = claim.CapacityBytes
	data.AccessModes = append([]string(nil), claim.AccessModes...)
	data.VolumeName = claim.VolumeName
	data.Labels = labels
	data.CreatedAt = zeroTimeToPtr(claim.CreatedAt)
	return namespacedKubernetesResource(cluster, clusterName, claim.Namespace, claim.Name, ResourceTypeK8sPVC, statusFromKubernetesPhase(claim.Phase), data, labels)
}

func resourceFromKubernetesStorageClass(cluster models.KubernetesCluster, class models.KubernetesStorageClass, capabilities *K8sMetricCapabilities) (Resource, ResourceIdentity) {
	clusterName := kubernetesClusterDisplayName(cluster)
	labels := cloneLabelMap(class.Labels)
	data := baseKubernetesData(cluster, clusterName, "StorageClass", capabilities)
	data.StorageClassUID = class.UID
	data.ResourceUID = class.UID
	data.StorageClass = class.Name
	data.Provisioner = class.Provisioner
	data.ReclaimPolicy = class.ReclaimPolicy
	data.VolumeBindingMode = class.VolumeBindingMode
	data.AllowVolumeExpansion = cloneBoolPtr(class.AllowVolumeExpansion)
	data.ParameterKeys = append([]string(nil), class.ParameterKeys...)
	data.Labels = labels
	data.CreatedAt = zeroTimeToPtr(class.CreatedAt)
	resource := Resource{
		Type:       ResourceTypeK8sStorageClass,
		Technology: "kubernetes",
		Name:       class.Name,
		Status:     StatusOnline,
		LastSeen:   cluster.LastSeen,
		UpdatedAt:  time.Now().UTC(),
		Kubernetes: &data,
		Tags:       labelsToTags(labels),
	}
	identity := ResourceIdentity{
		Hostnames:   uniqueStrings([]string{class.Name, clusterName + ":" + class.Name}),
		ClusterName: clusterName,
	}
	return resource, identity
}

func resourceFromKubernetesConfigMap(cluster models.KubernetesCluster, configMap models.KubernetesConfigMap, capabilities *K8sMetricCapabilities) (Resource, ResourceIdentity) {
	clusterName := kubernetesClusterDisplayName(cluster)
	labels := cloneLabelMap(configMap.Labels)
	data := baseKubernetesData(cluster, clusterName, "ConfigMap", capabilities)
	data.ConfigMapUID = configMap.UID
	data.ResourceUID = configMap.UID
	data.Namespace = configMap.Namespace
	data.DataKeys = append([]string(nil), configMap.DataKeys...)
	data.BinaryDataKeys = append([]string(nil), configMap.BinaryDataKeys...)
	data.Immutable = configMap.Immutable
	data.MetadataOnly = configMap.MetadataOnly
	data.Labels = labels
	data.CreatedAt = zeroTimeToPtr(configMap.CreatedAt)
	return namespacedKubernetesResource(cluster, clusterName, configMap.Namespace, configMap.Name, ResourceTypeK8sConfigMap, StatusOnline, data, labels)
}

func resourceFromKubernetesSecret(cluster models.KubernetesCluster, secret models.KubernetesSecret, capabilities *K8sMetricCapabilities) (Resource, ResourceIdentity) {
	clusterName := kubernetesClusterDisplayName(cluster)
	labels := cloneLabelMap(secret.Labels)
	data := baseKubernetesData(cluster, clusterName, "Secret", capabilities)
	data.SecretUID = secret.UID
	data.ResourceUID = secret.UID
	data.Namespace = secret.Namespace
	data.SecretType = secret.Type
	data.DataKeys = append([]string(nil), secret.DataKeys...)
	data.Immutable = secret.Immutable
	data.MetadataOnly = secret.MetadataOnly
	data.Labels = labels
	data.CreatedAt = zeroTimeToPtr(secret.CreatedAt)
	return namespacedKubernetesResource(cluster, clusterName, secret.Namespace, secret.Name, ResourceTypeK8sSecret, StatusOnline, data, labels)
}

func resourceFromKubernetesServiceAccount(cluster models.KubernetesCluster, account models.KubernetesServiceAccount, capabilities *K8sMetricCapabilities) (Resource, ResourceIdentity) {
	clusterName := kubernetesClusterDisplayName(cluster)
	labels := cloneLabelMap(account.Labels)
	data := baseKubernetesData(cluster, clusterName, "ServiceAccount", capabilities)
	data.ServiceAccountUID = account.UID
	data.ResourceUID = account.UID
	data.Namespace = account.Namespace
	data.AutomountServiceAccountToken = cloneBoolPtr(account.AutomountServiceAccountToken)
	data.SecretCount = account.SecretCount
	data.ImagePullSecrets = append([]string(nil), account.ImagePullSecrets...)
	data.Labels = labels
	data.CreatedAt = zeroTimeToPtr(account.CreatedAt)
	return namespacedKubernetesResource(cluster, clusterName, account.Namespace, account.Name, ResourceTypeK8sServiceAccount, StatusOnline, data, labels)
}

func resourceFromKubernetesRole(cluster models.KubernetesCluster, role models.KubernetesRole, capabilities *K8sMetricCapabilities) (Resource, ResourceIdentity) {
	clusterName := kubernetesClusterDisplayName(cluster)
	labels := cloneLabelMap(role.Labels)
	data := baseKubernetesData(cluster, clusterName, "Role", capabilities)
	data.RoleUID = role.UID
	data.ResourceUID = role.UID
	data.Namespace = role.Namespace
	data.RuleCount = role.RuleCount
	data.Labels = labels
	data.CreatedAt = zeroTimeToPtr(role.CreatedAt)
	return namespacedKubernetesResource(cluster, clusterName, role.Namespace, role.Name, ResourceTypeK8sRole, StatusOnline, data, labels)
}

func resourceFromKubernetesClusterRole(cluster models.KubernetesCluster, role models.KubernetesClusterRole, capabilities *K8sMetricCapabilities) (Resource, ResourceIdentity) {
	clusterName := kubernetesClusterDisplayName(cluster)
	labels := cloneLabelMap(role.Labels)
	data := baseKubernetesData(cluster, clusterName, "ClusterRole", capabilities)
	data.ClusterRoleUID = role.UID
	data.ResourceUID = role.UID
	data.RuleCount = role.RuleCount
	data.AggregationLabels = cloneStringMap(role.AggregationLabels)
	data.Labels = labels
	data.CreatedAt = zeroTimeToPtr(role.CreatedAt)
	resource := Resource{
		Type:       ResourceTypeK8sClusterRole,
		Technology: "kubernetes",
		Name:       role.Name,
		Status:     StatusOnline,
		LastSeen:   cluster.LastSeen,
		UpdatedAt:  time.Now().UTC(),
		Kubernetes: &data,
		Tags:       labelsToTags(labels),
	}
	identity := ResourceIdentity{
		Hostnames:   uniqueStrings([]string{role.Name, clusterName + ":" + role.Name}),
		ClusterName: clusterName,
	}
	return resource, identity
}

func resourceFromKubernetesRoleBinding(cluster models.KubernetesCluster, binding models.KubernetesRoleBinding, capabilities *K8sMetricCapabilities) (Resource, ResourceIdentity) {
	clusterName := kubernetesClusterDisplayName(cluster)
	labels := cloneLabelMap(binding.Labels)
	data := baseKubernetesData(cluster, clusterName, "RoleBinding", capabilities)
	data.RoleBindingUID = binding.UID
	data.ResourceUID = binding.UID
	data.Namespace = binding.Namespace
	data.RoleKind = binding.RoleKind
	data.RoleName = binding.RoleName
	data.SubjectCount = binding.SubjectCount
	data.SubjectKinds = append([]string(nil), binding.SubjectKinds...)
	data.Labels = labels
	data.CreatedAt = zeroTimeToPtr(binding.CreatedAt)
	return namespacedKubernetesResource(cluster, clusterName, binding.Namespace, binding.Name, ResourceTypeK8sRoleBinding, StatusOnline, data, labels)
}

func resourceFromKubernetesClusterRoleBinding(cluster models.KubernetesCluster, binding models.KubernetesClusterRoleBinding, capabilities *K8sMetricCapabilities) (Resource, ResourceIdentity) {
	clusterName := kubernetesClusterDisplayName(cluster)
	labels := cloneLabelMap(binding.Labels)
	data := baseKubernetesData(cluster, clusterName, "ClusterRoleBinding", capabilities)
	data.ClusterRoleBindingUID = binding.UID
	data.ResourceUID = binding.UID
	data.RoleKind = binding.RoleKind
	data.RoleName = binding.RoleName
	data.SubjectCount = binding.SubjectCount
	data.SubjectKinds = append([]string(nil), binding.SubjectKinds...)
	data.Labels = labels
	data.CreatedAt = zeroTimeToPtr(binding.CreatedAt)
	resource := Resource{
		Type:       ResourceTypeK8sClusterRoleBinding,
		Technology: "kubernetes",
		Name:       binding.Name,
		Status:     StatusOnline,
		LastSeen:   cluster.LastSeen,
		UpdatedAt:  time.Now().UTC(),
		Kubernetes: &data,
		Tags:       labelsToTags(labels),
	}
	identity := ResourceIdentity{
		Hostnames:   uniqueStrings([]string{binding.Name, clusterName + ":" + binding.Name}),
		ClusterName: clusterName,
	}
	return resource, identity
}

func resourceFromKubernetesResourceQuota(cluster models.KubernetesCluster, quota models.KubernetesResourceQuota, capabilities *K8sMetricCapabilities) (Resource, ResourceIdentity) {
	clusterName := kubernetesClusterDisplayName(cluster)
	labels := cloneLabelMap(quota.Labels)
	data := baseKubernetesData(cluster, clusterName, "ResourceQuota", capabilities)
	data.ResourceQuotaUID = quota.UID
	data.ResourceUID = quota.UID
	data.Namespace = quota.Namespace
	data.Hard = cloneStringMap(quota.Hard)
	data.Used = cloneStringMap(quota.Used)
	data.Labels = labels
	data.CreatedAt = zeroTimeToPtr(quota.CreatedAt)
	return namespacedKubernetesResource(cluster, clusterName, quota.Namespace, quota.Name, ResourceTypeK8sResourceQuota, StatusOnline, data, labels)
}

func resourceFromKubernetesLimitRange(cluster models.KubernetesCluster, limitRange models.KubernetesLimitRange, capabilities *K8sMetricCapabilities) (Resource, ResourceIdentity) {
	clusterName := kubernetesClusterDisplayName(cluster)
	labels := cloneLabelMap(limitRange.Labels)
	data := baseKubernetesData(cluster, clusterName, "LimitRange", capabilities)
	data.LimitRangeUID = limitRange.UID
	data.ResourceUID = limitRange.UID
	data.Namespace = limitRange.Namespace
	data.LimitTypes = append([]string(nil), limitRange.LimitTypes...)
	data.Labels = labels
	data.CreatedAt = zeroTimeToPtr(limitRange.CreatedAt)
	return namespacedKubernetesResource(cluster, clusterName, limitRange.Namespace, limitRange.Name, ResourceTypeK8sLimitRange, StatusOnline, data, labels)
}

func resourceFromKubernetesPodDisruptionBudget(cluster models.KubernetesCluster, budget models.KubernetesPodDisruptionBudget, capabilities *K8sMetricCapabilities) (Resource, ResourceIdentity) {
	clusterName := kubernetesClusterDisplayName(cluster)
	labels := cloneLabelMap(budget.Labels)
	data := baseKubernetesData(cluster, clusterName, "PodDisruptionBudget", capabilities)
	data.PodDisruptionBudgetUID = budget.UID
	data.ResourceUID = budget.UID
	data.Namespace = budget.Namespace
	data.MinAvailable = budget.MinAvailable
	data.MaxUnavailable = budget.MaxUnavailable
	data.DesiredHealthy = budget.DesiredHealthy
	data.CurrentHealthy = budget.CurrentHealthy
	data.DisruptionsAllowed = budget.DisruptionsAllowed
	data.ExpectedPods = budget.ExpectedPods
	data.Labels = labels
	data.CreatedAt = zeroTimeToPtr(budget.CreatedAt)
	return namespacedKubernetesResource(cluster, clusterName, budget.Namespace, budget.Name, ResourceTypeK8sPDB, statusFromKubernetesPodDisruptionBudget(budget), data, labels)
}

func resourceFromKubernetesHorizontalPodAutoscaler(cluster models.KubernetesCluster, autoscaler models.KubernetesHorizontalPodAutoscaler, capabilities *K8sMetricCapabilities) (Resource, ResourceIdentity) {
	clusterName := kubernetesClusterDisplayName(cluster)
	labels := cloneLabelMap(autoscaler.Labels)
	data := baseKubernetesData(cluster, clusterName, "HorizontalPodAutoscaler", capabilities)
	data.HorizontalPodAutoscalerUID = autoscaler.UID
	data.ResourceUID = autoscaler.UID
	data.Namespace = autoscaler.Namespace
	data.TargetKind = autoscaler.TargetKind
	data.TargetName = autoscaler.TargetName
	data.MinReplicas = autoscaler.MinReplicas
	data.MaxReplicas = autoscaler.MaxReplicas
	data.CurrentReplicas = autoscaler.CurrentReplicas
	data.DesiredReplicas = autoscaler.DesiredReplicas
	data.MetricTypes = append([]string(nil), autoscaler.MetricTypes...)
	data.Labels = labels
	data.CreatedAt = zeroTimeToPtr(autoscaler.CreatedAt)
	return namespacedKubernetesResource(cluster, clusterName, autoscaler.Namespace, autoscaler.Name, ResourceTypeK8sHPA, statusFromKubernetesHorizontalPodAutoscaler(autoscaler), data, labels)
}

func resourceFromKubernetesEvent(cluster models.KubernetesCluster, event models.KubernetesEvent, capabilities *K8sMetricCapabilities) (Resource, ResourceIdentity) {
	clusterName := kubernetesClusterDisplayName(cluster)
	data := baseKubernetesData(cluster, clusterName, "Event", capabilities)
	data.EventUID = event.UID
	data.ResourceUID = event.UID
	data.Namespace = event.Namespace
	data.EventType = event.EventType
	data.Reason = event.Reason
	data.Message = event.Message
	data.InvolvedKind = event.InvolvedKind
	data.InvolvedName = event.InvolvedName
	data.Count = event.Count
	data.FirstSeen = event.FirstSeen
	data.EventTime = event.EventTime
	if event.LastSeen != nil {
		data.CreatedAt = event.LastSeen
	}
	resource := Resource{
		Type:       ResourceTypeK8sEvent,
		Technology: "kubernetes",
		Name:       firstNonEmpty(event.Name, event.Reason, event.InvolvedName),
		Status:     statusFromKubernetesEvent(event),
		LastSeen:   cluster.LastSeen,
		UpdatedAt:  time.Now().UTC(),
		Kubernetes: &data,
	}
	return resource, namespacedKubernetesIdentity(clusterName, event.Namespace, resource.Name)
}

func k8sServicePorts(in []models.KubernetesServicePort) []K8sServicePort {
	if len(in) == 0 {
		return nil
	}
	out := make([]K8sServicePort, 0, len(in))
	for _, port := range in {
		out = append(out, K8sServicePort{
			Name:       strings.TrimSpace(port.Name),
			Protocol:   strings.TrimSpace(port.Protocol),
			Port:       port.Port,
			TargetPort: strings.TrimSpace(port.TargetPort),
			NodePort:   port.NodePort,
		})
	}
	return out
}

func namespacedKubernetesIdentity(clusterName, namespace, name string) ResourceIdentity {
	namespace = strings.TrimSpace(namespace)
	name = strings.TrimSpace(name)
	values := []string{name}
	if namespace != "" && name != "" {
		values = append(values, namespace+"/"+name)
	}
	if clusterName != "" && name != "" {
		values = append(values, clusterName+":"+name)
	}
	return ResourceIdentity{
		Hostnames:   uniqueStrings(values),
		ClusterName: clusterName,
	}
}

func kubernetesClusterDisplayName(cluster models.KubernetesCluster) string {
	if v := strings.TrimSpace(cluster.CustomDisplayName); v != "" {
		return v
	}
	if v := strings.TrimSpace(cluster.DisplayName); v != "" {
		return v
	}
	if v := strings.TrimSpace(cluster.Name); v != "" {
		return v
	}
	return strings.TrimSpace(cluster.ID)
}

func cloneK8sPodContainers(in []models.KubernetesPodContainer) []K8sPodContainer {
	if len(in) == 0 {
		return nil
	}
	out := make([]K8sPodContainer, len(in))
	for i, c := range in {
		out[i] = K8sPodContainer{
			Name:         c.Name,
			Image:        c.Image,
			Ready:        c.Ready,
			RestartCount: c.RestartCount,
			State:        c.State,
			Reason:       c.Reason,
			Message:      c.Message,
		}
	}
	return out
}

func cloneLabelMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		if strings.TrimSpace(k) == "" {
			continue
		}
		out[k] = v
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func labelsToTags(labels map[string]string) []string {
	if len(labels) == 0 {
		return nil
	}
	out := make([]string, 0, len(labels))
	for k, v := range labels {
		key := strings.TrimSpace(k)
		if key == "" {
			continue
		}
		if strings.TrimSpace(v) == "" {
			out = append(out, key)
			continue
		}
		out = append(out, key+":"+v)
	}
	return uniqueStrings(out)
}

func convertInterfaces(interfaces []models.HostNetworkInterface) []NetworkInterface {
	out := make([]NetworkInterface, 0, len(interfaces))
	for _, iface := range interfaces {
		out = append(out, NetworkInterface{
			Name:      iface.Name,
			MAC:       iface.MAC,
			Addresses: iface.Addresses,
			RXBytes:   iface.RXBytes,
			TXBytes:   iface.TXBytes,
			SpeedMbps: iface.SpeedMbps,
		})
	}
	return out
}

func convertDisks(disks []models.Disk) []DiskInfo {
	out := make([]DiskInfo, 0, len(disks))
	for _, disk := range disks {
		out = append(out, DiskInfo{
			Device:     disk.Device,
			Mountpoint: disk.Mountpoint,
			Filesystem: disk.Type,
			Total:      disk.Total,
			Used:       disk.Used,
			Free:       disk.Free,
			Usage:      disk.Usage,
		})
	}
	return out
}

func hasReportableDockerSwarmInfo(info *models.DockerSwarmInfo) bool {
	if info == nil {
		return false
	}
	state := strings.ToLower(strings.TrimSpace(info.LocalState))
	if state == "inactive" {
		return info.ControlAvailable ||
			strings.TrimSpace(info.ClusterID) != "" ||
			strings.TrimSpace(info.ClusterName) != "" ||
			strings.TrimSpace(info.Error) != ""
	}

	return strings.TrimSpace(info.NodeID) != "" ||
		state != "" ||
		info.ControlAvailable ||
		strings.TrimSpace(info.ClusterID) != "" ||
		strings.TrimSpace(info.ClusterName) != "" ||
		strings.TrimSpace(info.Error) != ""
}

func convertSwarm(info *models.DockerSwarmInfo) *DockerSwarmInfo {
	if !hasReportableDockerSwarmInfo(info) {
		return nil
	}
	return &DockerSwarmInfo{
		NodeID:           info.NodeID,
		NodeRole:         info.NodeRole,
		LocalState:       info.LocalState,
		ControlAvailable: info.ControlAvailable,
		ClusterID:        info.ClusterID,
		ClusterName:      info.ClusterName,
		Scope:            info.Scope,
		Error:            info.Error,
	}
}

func collectInterfaceIDs(interfaces []models.HostNetworkInterface) ([]string, []string) {
	var ips []string
	var macs []string
	// Agents report interfaces sorted by name, which places docker0/br-* bridges
	// ahead of eth*/en*, and consumers treat the first IP as the host's primary
	// address. Collect physical-looking interfaces first so bridge and overlay
	// addresses never lead the list. Mirrors hostagent.isLikelyVirtualInterfaceName.
	for pass := 0; pass < 2; pass++ {
		for _, iface := range interfaces {
			if (pass == 0) == isLikelyVirtualInterfaceName(iface.Name) {
				continue
			}
			if iface.MAC != "" {
				macs = append(macs, iface.MAC)
			}
			for _, addr := range iface.Addresses {
				ip := addr
				if strings.Contains(ip, "/") {
					ip = strings.Split(ip, "/")[0]
				}
				ips = append(ips, ip)
			}
		}
	}
	return ips, macs
}

func isLikelyVirtualInterfaceName(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	switch {
	case name == "" || name == "lo":
		return true
	case strings.HasPrefix(name, "docker"):
		return true
	case strings.HasPrefix(name, "veth"):
		return true
	case strings.HasPrefix(name, "br-"):
		return true
	case strings.HasPrefix(name, "cni"):
		return true
	case strings.HasPrefix(name, "flannel"):
		return true
	case strings.HasPrefix(name, "virbr"):
		return true
	case strings.HasPrefix(name, "zt"):
		return true
	default:
		return false
	}
}

func extractHostname(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err == nil && parsed.Host != "" {
		host := parsed.Host
		if strings.Contains(host, ":") {
			host = strings.Split(host, ":")[0]
		}
		return host
	}

	if strings.Contains(raw, "/") {
		raw = strings.Split(raw, "/")[0]
	}
	if strings.Contains(raw, ":") {
		raw = strings.Split(raw, ":")[0]
	}
	return raw
}

// maxCPUTemp returns the highest CPU temperature from host sensor readings.
// It looks for cpu_package first, then falls back to max of any cpu_core_* key.
func maxCPUTemp(sensors models.HostSensorSummary) *float64 {
	temps := sensors.TemperatureCelsius
	if len(temps) == 0 {
		return nil
	}
	// Prefer cpu_package if available.
	if v, ok := temps["cpu_package"]; ok {
		return &v
	}
	// Fall back to max of any cpu-related key.
	var best float64
	found := false
	for k, v := range temps {
		if strings.HasPrefix(k, "cpu") {
			if !found || v > best {
				best = v
				found = true
			}
		}
	}
	if found {
		return &best
	}
	return nil
}

// maxNodeTemp returns the best CPU temperature from a proxmox node temperature snapshot.
func maxNodeTemp(temperature *models.Temperature) *float64 {
	if temperature == nil || !temperature.Available {
		return nil
	}

	if temperature.CPUMax > 0 {
		v := temperature.CPUMax
		return &v
	}
	if temperature.CPUPackage > 0 {
		v := temperature.CPUPackage
		return &v
	}

	var best float64
	found := false
	for _, core := range temperature.Cores {
		if !found || core.Temp > best {
			best = core.Temp
			found = true
		}
	}
	if found {
		return &best
	}
	return nil
}

func maxPMGUptime(nodes []models.PMGNodeStatus) int64 {
	var best int64
	for _, node := range nodes {
		if node.Uptime > best {
			best = node.Uptime
		}
	}
	return best
}

func aggregatePMGQueue(nodes []models.PMGNodeStatus) models.PMGQueueStatus {
	var out models.PMGQueueStatus
	for _, node := range nodes {
		if node.QueueStatus == nil {
			continue
		}
		out.Active += node.QueueStatus.Active
		out.Deferred += node.QueueStatus.Deferred
		out.Hold += node.QueueStatus.Hold
		out.Incoming += node.QueueStatus.Incoming
		out.Total += node.QueueStatus.Total
		if node.QueueStatus.OldestAge > out.OldestAge {
			out.OldestAge = node.QueueStatus.OldestAge
		}
		if node.QueueStatus.UpdatedAt.After(out.UpdatedAt) {
			out.UpdatedAt = node.QueueStatus.UpdatedAt
		}
	}
	return out
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
