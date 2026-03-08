package unifiedresources

import (
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"
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
		SourceID:        node.ID,
		NodeName:        node.Name,
		ClusterName:     node.ClusterName,
		IsClusterMember: node.IsClusterMember,
		Instance:        node.Instance,
		HostURL:         node.Host,
		Temperature:     maxNodeTemp(node.Temperature),
		PVEVersion:      node.PVEVersion,
		KernelVersion:   node.KernelVersion,
		Uptime:          node.Uptime,
		CPUInfo:         &CPUInfo{Model: node.CPUInfo.Model, Cores: node.CPUInfo.Cores, Sockets: node.CPUInfo.Sockets},
		LoadAverage:     append([]float64(nil), node.LoadAverage...),
		PendingUpdates:  node.PendingUpdates,
		LinkedAgentID:   linkedAgentID,
	}

	metrics := metricsFromProxmoxNode(node)

	resource := Resource{
		Type:       ResourceTypeAgent,
		Technology: "proxmox",
		Name:       name,
		Status:     statusFromString(node.Status),
		LastSeen:   node.LastSeen,
		UpdatedAt:  time.Now().UTC(),
		Metrics:    metrics,
		Proxmox:    proxmox,
		Tags:       nil,
	}

	return resource, identity
}

func identityFromHost(host models.Host) ResourceIdentity {
	ips, macs := collectInterfaceIDs(host.NetworkInterfaces)
	if host.ReportIP != "" {
		ips = append(ips, host.ReportIP)
	}

	return ResourceIdentity{
		MachineID:    strings.TrimSpace(host.MachineID),
		Hostnames:    uniqueStrings([]string{host.Hostname}),
		IPAddresses:  uniqueStrings(ips),
		MACAddresses: uniqueStrings(macs),
	}
}

func resourceFromHost(host models.Host) (Resource, ResourceIdentity) {
	name := host.Hostname
	if host.DisplayName != "" {
		name = host.DisplayName
	}

	identity := identityFromHost(host)

	agent := &AgentData{
		AgentID:           host.ID,
		AgentVersion:      host.AgentVersion,
		Hostname:          host.Hostname,
		TokenID:           host.TokenID,
		TokenName:         host.TokenName,
		TokenHint:         host.TokenHint,
		TokenLastUsedAt:   host.TokenLastUsedAt,
		Platform:          host.Platform,
		OSName:            host.OSName,
		OSVersion:         host.OSVersion,
		KernelVersion:     host.KernelVersion,
		Architecture:      host.Architecture,
		UptimeSeconds:     host.UptimeSeconds,
		Temperature:       maxCPUTemp(host.Sensors),
		NetworkInterfaces: convertInterfaces(host.NetworkInterfaces),
		Disks:             convertDisks(host.Disks),
		Memory: &AgentMemoryMeta{
			Total:     host.Memory.Total,
			Used:      host.Memory.Used,
			Free:      host.Memory.Free,
			SwapUsed:  host.Memory.SwapUsed,
			SwapTotal: host.Memory.SwapTotal,
		},
		LinkedNodeID:      host.LinkedNodeID,
		LinkedVMID:        host.LinkedVMID,
		LinkedContainerID: host.LinkedContainerID,
	}
	storageAssessments := make([]storagehealth.Assessment, 0, len(host.RAID)+1)

	// Populate sensors
	if len(host.Sensors.TemperatureCelsius) > 0 || len(host.Sensors.FanRPM) > 0 || len(host.Sensors.Additional) > 0 || len(host.Sensors.SMART) > 0 {
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
		if len(host.Sensors.Additional) > 0 {
			sensorMeta.Additional = make(map[string]float64, len(host.Sensors.Additional))
			for k, v := range host.Sensors.Additional {
				sensorMeta.Additional[k] = v
			}
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
					Temperature: s.Temperature,
					Health:      s.Health,
					Standby:     s.Standby,
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
			disks[i] = HostUnraidDiskMeta{
				Name:       disk.Name,
				Device:     disk.Device,
				Role:       disk.Role,
				Status:     disk.Status,
				RawStatus:  disk.RawStatus,
				Serial:     disk.Serial,
				Filesystem: disk.Filesystem,
				SizeBytes:  disk.SizeBytes,
				Slot:       disk.Slot,
			}
		}
		assessment := storagehealth.AssessUnraidStorage(*host.Unraid)
		agent.Unraid = &HostUnraidMeta{
			ArrayStarted: host.Unraid.ArrayStarted,
			ArrayState:   host.Unraid.ArrayState,
			SyncAction:   host.Unraid.SyncAction,
			SyncProgress: host.Unraid.SyncProgress,
			SyncErrors:   host.Unraid.SyncErrors,
			NumProtected: host.Unraid.NumProtected,
			NumDisabled:  host.Unraid.NumDisabled,
			NumInvalid:   host.Unraid.NumInvalid,
			NumMissing:   host.Unraid.NumMissing,
			Disks:        disks,
			Risk:         storageRiskFromAssessment(assessment),
		}
		storageAssessments = append(storageAssessments, assessment)
	}

	if len(storageAssessments) > 0 {
		agent.StorageRisk = storageRiskFromAssessment(storagehealth.SummarizeAssessments(storageAssessments...))
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
		Type:       ResourceTypeAgent,
		Technology: strings.TrimSpace(host.Platform),
		Name:       name,
		Status:     storageStatus(statusFromString(host.Status), agent.StorageRisk),
		LastSeen:   host.LastSeen,
		UpdatedAt:  time.Now().UTC(),
		Metrics:    metrics,
		Agent:      agent,
		Tags:       host.Tags,
	}

	return resource, identity
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

func resourceFromHostSMARTDisk(host models.Host, disk models.HostDiskSMART) (Resource, ResourceIdentity) {
	name := strings.TrimSpace(disk.Model)
	if name == "" {
		name = strings.TrimSpace(disk.Device)
	}
	if name == "" {
		name = strings.TrimSpace(host.Hostname)
	}

	var matchedDisk *models.Disk
	normalizedDevice := strings.TrimSpace(strings.TrimPrefix(disk.Device, "/dev/"))
	for i := range host.Disks {
		hostDevice := strings.TrimSpace(strings.TrimPrefix(host.Disks[i].Device, "/dev/"))
		if normalizedDevice == "" || hostDevice == "" {
			continue
		}
		if strings.EqualFold(hostDevice, normalizedDevice) {
			matchedDisk = &host.Disks[i]
			break
		}
	}

	sizeBytes := int64(0)
	used := ""
	if matchedDisk != nil {
		sizeBytes = matchedDisk.Total
		used = strings.TrimSpace(matchedDisk.Mountpoint)
	}
	unraidDisk := matchUnraidDisk(host.Unraid, disk)
	assessment := storagehealth.AssessHostSMARTDisk(disk)

	resource := Resource{
		Type:      ResourceTypePhysicalDisk,
		Name:      name,
		Status:    physicalDiskStatus(disk.Model, disk.Health, assessment),
		LastSeen:  host.LastSeen,
		UpdatedAt: time.Now().UTC(),
		PhysicalDisk: &PhysicalDiskMeta{
			DevPath:      strings.TrimSpace(disk.Device),
			Model:        strings.TrimSpace(disk.Model),
			Serial:       strings.TrimSpace(disk.Serial),
			WWN:          strings.TrimSpace(disk.WWN),
			DiskType:     strings.TrimSpace(disk.Type),
			SizeBytes:    sizeBytes,
			Health:       strings.TrimSpace(disk.Health),
			Wearout:      -1,
			Temperature:  disk.Temperature,
			Used:         used,
			StorageRole:  unraidDiskRole(unraidDisk),
			StorageGroup: unraidDiskGroup(unraidDisk),
			StorageState: unraidDiskState(unraidDisk),
			SMART:        convertSMARTAttributes(disk.Attributes),
			Risk:         physicalDiskRiskFromAssessment(assessment),
		},
	}

	identity := ResourceIdentity{
		Hostnames: uniqueStrings([]string{host.Hostname}),
	}
	if disk.Serial != "" {
		identity.MachineID = strings.TrimSpace(disk.Serial)
	} else if disk.WWN != "" {
		identity.MachineID = strings.TrimSpace(disk.WWN)
	}

	return resource, identity
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

func unraidStorageCapacity(host models.Host) (int64, int64, int64, float64) {
	for _, disk := range host.Disks {
		mount := strings.TrimSpace(disk.Mountpoint)
		if mount == "/mnt/user" || mount == "/mnt/user0" {
			return disk.Total, disk.Used, disk.Free, percentFromUsage(disk.Usage)
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
		if strings.TrimSpace(disk.Role) != "data" {
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

func matchUnraidDisk(unraid *models.HostUnraidStorage, disk models.HostDiskSMART) *models.HostUnraidDisk {
	if unraid == nil || len(unraid.Disks) == 0 {
		return nil
	}

	normalizedDevice := strings.TrimSpace(strings.TrimPrefix(strings.ToLower(disk.Device), "/dev/"))
	normalizedSerial := strings.TrimSpace(strings.ToLower(disk.Serial))
	for i := range unraid.Disks {
		candidate := &unraid.Disks[i]
		if normalizedSerial != "" && strings.EqualFold(strings.TrimSpace(candidate.Serial), normalizedSerial) {
			return candidate
		}
		candidateDevice := strings.TrimSpace(strings.TrimPrefix(strings.ToLower(candidate.Device), "/dev/"))
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
	return strings.TrimSpace(disk.Role)
}

func unraidDiskGroup(disk *models.HostUnraidDisk) string {
	if disk == nil {
		return ""
	}
	role := strings.TrimSpace(disk.Role)
	switch role {
	case "parity", "data":
		return "unraid-array"
	case "cache":
		return "unraid-cache"
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
		MachineID:             host.MachineID,
		Temperature:           host.Temperature,
		Runtime:               host.Runtime,
		RuntimeVersion:        host.RuntimeVersion,
		DockerVersion:         host.DockerVersion,
		OS:                    host.OS,
		KernelVersion:         host.KernelVersion,
		Architecture:          host.Architecture,
		AgentVersion:          host.AgentVersion,
		UptimeSeconds:         host.UptimeSeconds,
		ContainerCount:        len(host.Containers),
		UpdatesAvailableCount: updatesAvailableCount,
		UpdatesLastCheckedAt:  updatesLastCheckedPtr,
		TokenID:               host.TokenID,
		TokenName:             host.TokenName,
		TokenHint:             host.TokenHint,
		TokenLastUsedAt:       host.TokenLastUsedAt,
		Hidden:                host.Hidden,
		PendingUninstall:      host.PendingUninstall,
		IsLegacy:              host.IsLegacy,
		Command:               host.Command,
		Swarm:                 convertSwarm(host.Swarm),
		NetworkInterfaces:     convertInterfaces(host.NetworkInterfaces),
		Disks:                 convertDisks(host.Disks),
		Services:              append([]models.DockerService(nil), host.Services...),
		Tasks:                 append([]models.DockerTask(nil), host.Tasks...),
	}

	metrics := metricsFromDockerHost(host)

	resource := Resource{
		Type:       ResourceTypeAgent,
		Technology: strings.TrimSpace(host.Runtime),
		Name:       name,
		Status:     statusFromString(host.Status),
		LastSeen:   host.LastSeen,
		UpdatedAt:  time.Now().UTC(),
		Metrics:    metrics,
		Docker:     docker,
		Tags:       nil,
	}

	return resource, identity
}

func resourceFromPBSInstance(instance models.PBSInstance) (Resource, ResourceIdentity) {
	name := instance.Name
	if strings.TrimSpace(name) == "" {
		name = extractHostname(instance.Host)
	}

	resource := Resource{
		Type:      ResourceTypePBS,
		Name:      name,
		Status:    statusFromPBSInstance(instance),
		LastSeen:  instance.LastSeen,
		UpdatedAt: time.Now().UTC(),
		Metrics:   metricsFromPBSInstance(instance),
		CustomURL: instance.GuestURL,
		PBS: &PBSData{
			InstanceID:       instance.ID,
			Hostname:         extractHostname(instance.Host),
			Version:          instance.Version,
			UptimeSeconds:    instance.Uptime,
			DatastoreCount:   len(instance.Datastores),
			BackupJobCount:   len(instance.BackupJobs),
			SyncJobCount:     len(instance.SyncJobs),
			VerifyJobCount:   len(instance.VerifyJobs),
			PruneJobCount:    len(instance.PruneJobs),
			GarbageJobCount:  len(instance.GarbageJobs),
			ConnectionHealth: instance.ConnectionHealth,
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
	status := statusFromString(datastore.Status)
	if strings.TrimSpace(datastore.Error) != "" && status == StatusOnline {
		status = StatusWarning
	}

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
		},
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
		CustomURL: instance.GuestURL,
		PMG: &PMGData{
			InstanceID:       instance.ID,
			Hostname:         extractHostname(instance.Host),
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
					Active:   n.QueueStatus.Active,
					Deferred: n.QueueStatus.Deferred,
					Hold:     n.QueueStatus.Hold,
					Incoming: n.QueueStatus.Incoming,
					Total:    n.QueueStatus.Total,
				}
			}
		}
		resource.PMG.Nodes = nodes
	}

	// Populate mail stats
	if instance.MailStats != nil {
		resource.PMG.MailStats = &PMGMailStatsMeta{
			Timeframe:            instance.MailStats.Timeframe,
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
			RBLRejects:           instance.MailStats.RBLRejects,
			AverageProcessTimeMs: instance.MailStats.AverageProcessTimeMs,
		}
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
	metrics := metricsFromVM(vm)
	proxmox := &ProxmoxData{
		SourceID:   vm.ID,
		NodeName:   vm.Node,
		Instance:   vm.Instance,
		VMID:       vm.VMID,
		CPUs:       vm.CPUs,
		Uptime:     vm.Uptime,
		Template:   vm.Template,
		LastBackup: vm.LastBackup,
		Disks:      convertDisks(vm.Disks),
		SwapUsed:   vm.Memory.SwapUsed,
		SwapTotal:  vm.Memory.SwapTotal,
		Balloon:    vm.Memory.Balloon,
		Lock:       vm.Lock,
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
	identity := ResourceIdentity{
		Hostnames:   uniqueStrings([]string{vm.Name}),
		IPAddresses: uniqueStrings(vm.IPAddresses),
	}
	return resource, identity
}

func resourceFromContainer(ct models.Container) (Resource, ResourceIdentity) {
	metrics := metricsFromContainer(ct)
	proxmox := &ProxmoxData{
		SourceID:   ct.ID,
		NodeName:   ct.Node,
		Instance:   ct.Instance,
		VMID:       ct.VMID,
		CPUs:       ct.CPUs,
		Uptime:     ct.Uptime,
		Template:   ct.Template,
		LastBackup: ct.LastBackup,
		Disks:      convertDisks(ct.Disks),
		SwapUsed:   ct.Memory.SwapUsed,
		SwapTotal:  ct.Memory.SwapTotal,
		Balloon:    ct.Memory.Balloon,
		Lock:       ct.Lock,
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
	identity := ResourceIdentity{
		Hostnames:   uniqueStrings([]string{ct.Name}),
		IPAddresses: uniqueStrings(ct.IPAddresses),
	}
	return resource, identity
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
	if storage.ZFSPool != nil {
		zfsPoolState = strings.TrimSpace(storage.ZFSPool.State)
		zfsReadErrors = storage.ZFSPool.ReadErrors
		zfsWriteErrors = storage.ZFSPool.WriteErrors
		zfsChecksumErrors = storage.ZFSPool.ChecksumErrors
	}

	resource := Resource{
		Type:      ResourceTypeStorage,
		Name:      name,
		Status:    statusFromStorage(storage),
		LastSeen:  now,
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
			IsCeph:            isCephStorageType(storageType),
			IsZFS:             isZFSStorageType(storageType) || storage.ZFSPool != nil,
			Nodes:             append([]string(nil), storage.Nodes...),
			Path:              storage.Path,
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
		DevPath:     disk.DevPath,
		Model:       disk.Model,
		Serial:      disk.Serial,
		WWN:         disk.WWN,
		DiskType:    disk.Type,
		SizeBytes:   disk.Size,
		Health:      disk.Health,
		Wearout:     disk.Wearout,
		Temperature: disk.Temperature,
		RPM:         disk.RPM,
		Used:        disk.Used,
		Risk:        physicalDiskRiskFromAssessment(assessment),
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
		Tags:         physicalDiskTags(disk),
	}

	identity := ResourceIdentity{
		Hostnames: uniqueStrings([]string{disk.Node}),
	}
	if disk.Serial != "" {
		identity.MachineID = disk.Serial
	} else if disk.WWN != "" {
		identity.MachineID = disk.WWN
	}

	return resource, identity
}

func convertSMARTAttributes(attrs *models.SMARTAttributes) *SMARTMeta {
	if attrs == nil {
		return nil
	}
	m := &SMARTMeta{}
	if attrs.PowerOnHours != nil {
		m.PowerOnHours = *attrs.PowerOnHours
	}
	if attrs.PowerCycles != nil {
		m.PowerCycles = *attrs.PowerCycles
	}
	if attrs.ReallocatedSectors != nil {
		m.ReallocatedSectors = *attrs.ReallocatedSectors
	}
	if attrs.PendingSectors != nil {
		m.PendingSectors = *attrs.PendingSectors
	}
	if attrs.OfflineUncorrectable != nil {
		m.OfflineUncorrectable = *attrs.OfflineUncorrectable
	}
	if attrs.UDMACRCErrors != nil {
		m.UDMACRCErrors = *attrs.UDMACRCErrors
	}
	if attrs.PercentageUsed != nil {
		m.PercentageUsed = *attrs.PercentageUsed
	}
	if attrs.AvailableSpare != nil {
		m.AvailableSpare = *attrs.AvailableSpare
	}
	if attrs.MediaErrors != nil {
		m.MediaErrors = *attrs.MediaErrors
	}
	if attrs.UnsafeShutdowns != nil {
		m.UnsafeShutdowns = *attrs.UnsafeShutdowns
	}
	return m
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
	metrics := metricsFromDockerContainer(ct)
	runtime := strings.TrimSpace(host.Runtime)
	if runtime == "" {
		if ct.Podman != nil {
			runtime = "podman"
		} else {
			runtime = "docker"
		}
	}
	docker := &DockerData{
		HostSourceID:   host.ID,
		ContainerID:    ct.ID,
		Image:          ct.Image,
		UptimeSeconds:  ct.UptimeSeconds,
		ContainerState: ct.State,
		Health:         ct.Health,
		RestartCount:   ct.RestartCount,
		ExitCode:       ct.ExitCode,
		Labels:         cloneLabelMap(ct.Labels),
		Runtime:        runtime,
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
		Type:       ResourceTypeAppContainer,
		Technology: runtime,
		Name:       ct.Name,
		Status:     statusFromDockerState(ct.State),
		LastSeen:   time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
		Metrics:    metrics,
	}
	resource.Docker = docker
	identity := ResourceIdentity{
		Hostnames: uniqueStrings([]string{ct.Name}),
	}
	return resource, identity
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
		Kubernetes: &K8sData{
			ClusterID:          cluster.ID,
			ClusterName:        clusterName,
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
			Labels:             labels,
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

func convertSwarm(info *models.DockerSwarmInfo) *DockerSwarmInfo {
	if info == nil {
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
	for _, iface := range interfaces {
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
	return ips, macs
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
