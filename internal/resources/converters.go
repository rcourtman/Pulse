package resources

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// FromNode converts a Proxmox Node to a unified Resource.
func FromNode(n models.Node) Resource {
	// Calculate primary temperature if available
	var temp *float64
	if n.Temperature != nil && n.Temperature.Available && n.Temperature.HasCPU {
		// Use CPU package temperature if available, otherwise average core temps
		if n.Temperature.CPUPackage > 0 {
			temp = &n.Temperature.CPUPackage
		} else if len(n.Temperature.Cores) > 0 {
			avg := 0.0
			for _, c := range n.Temperature.Cores {
				avg += c.Temp
			}
			avg /= float64(len(n.Temperature.Cores))
			temp = &avg
		}
	}

	// Build memory metric
	var memory *MetricValue
	if n.Memory.Total > 0 {
		memory = &MetricValue{
			Current: n.Memory.Usage,
			Total:   &n.Memory.Total,
			Used:    &n.Memory.Used,
			Free:    &n.Memory.Free,
		}
	}

	// Build disk metric
	var disk *MetricValue
	if n.Disk.Total > 0 {
		disk = &MetricValue{
			Current: n.Disk.Usage,
			Total:   &n.Disk.Total,
			Used:    &n.Disk.Used,
			Free:    &n.Disk.Free,
		}
	}

	// Build platform data
	platformData := NodePlatformData{
		Instance:         n.Instance,
		Host:             n.Host,
		GuestURL:         n.GuestURL,
		PVEVersion:       n.PVEVersion,
		KernelVersion:    n.KernelVersion,
		LoadAverage:      n.LoadAverage,
		IsClusterMember:  n.IsClusterMember,
		ClusterName:      n.ClusterName,
		ConnectionHealth: n.ConnectionHealth,
		CPUInfo: CPUInfo{
			Model:   n.CPUInfo.Model,
			Cores:   n.CPUInfo.Cores,
			Sockets: n.CPUInfo.Sockets,
		},
	}
	platformDataJSON, _ := json.Marshal(platformData)

	// Determine cluster ID
	clusterID := ""
	if n.IsClusterMember && n.ClusterName != "" {
		clusterID = fmt.Sprintf("pve-cluster/%s", n.ClusterName)
	}

	return Resource{
		ID:           n.ID,
		Type:         ResourceTypeNode,
		Name:         n.Name,
		DisplayName:  n.DisplayName,
		PlatformID:   n.Instance,
		PlatformType: PlatformProxmoxPVE,
		SourceType:   SourceAPI,
		ClusterID:    clusterID,
		Status:       mapNodeStatus(n.Status),
		CPU: &MetricValue{
			Current: n.CPU * 100, // Node CPU is 0-1, convert to percentage
		},
		Memory:       memory,
		Disk:         disk,
		Temperature:  temp,
		Uptime:       &n.Uptime,
		LastSeen:     n.LastSeen,
		PlatformData: platformDataJSON,
		Identity: &ResourceIdentity{
			Hostname: n.Name,
		},
		SchemaVersion: CurrentSchemaVersion,
	}
}

// FromVM converts a Proxmox VM to a unified Resource.
func FromVM(vm models.VM) Resource {
	// Build memory metric
	var memory *MetricValue
	if vm.Memory.Total > 0 {
		memory = &MetricValue{
			Current: vm.Memory.Usage,
			Total:   &vm.Memory.Total,
			Used:    &vm.Memory.Used,
			Free:    &vm.Memory.Free,
		}
	}

	// Build disk metric
	var disk *MetricValue
	if vm.Disk.Total > 0 {
		disk = &MetricValue{
			Current: vm.Disk.Usage,
			Total:   &vm.Disk.Total,
			Used:    &vm.Disk.Used,
			Free:    &vm.Disk.Free,
		}
	}

	// Build network metric
	network := &NetworkMetric{
		RXBytes: vm.NetworkIn,
		TXBytes: vm.NetworkOut,
	}

	// Build platform data
	var lastBackup *time.Time
	if !vm.LastBackup.IsZero() {
		lastBackup = &vm.LastBackup
	}
	platformData := VMPlatformData{
		VMID:         vm.VMID,
		Node:         vm.Node,
		Instance:     vm.Instance,
		CPUs:         vm.CPUs,
		Template:     vm.Template,
		Lock:         vm.Lock,
		AgentVersion: vm.AgentVersion,
		OSName:       vm.OSName,
		OSVersion:    vm.OSVersion,
		IPAddresses:  vm.IPAddresses,
		NetworkIn:    vm.NetworkIn,
		NetworkOut:   vm.NetworkOut,
		DiskRead:     vm.DiskRead,
		DiskWrite:    vm.DiskWrite,
		LastBackup:   lastBackup,
	}
	platformDataJSON, _ := json.Marshal(platformData)

	// Parent is the node - format matches Node.ID: instance-nodename
	parentID := fmt.Sprintf("%s-%s", vm.Instance, vm.Node)

	return Resource{
		ID:           vm.ID,
		Type:         ResourceTypeVM,
		Name:         vm.Name,
		PlatformID:   vm.Instance,
		PlatformType: PlatformProxmoxPVE,
		SourceType:   SourceAPI,
		ParentID:     parentID,
		Status:       mapGuestStatus(vm.Status),
		CPU: &MetricValue{
			Current: vm.CPU * 100, // VM CPU is 0-1, convert to percentage
		},
		Memory:       memory,
		Disk:         disk,
		Network:      network,
		Uptime:       &vm.Uptime,
		Tags:         vm.Tags,
		LastSeen:     vm.LastSeen,
		PlatformData: platformDataJSON,
		SchemaVersion: CurrentSchemaVersion,
	}
}

// FromContainer converts a Proxmox LXC Container to a unified Resource.
func FromContainer(ct models.Container) Resource {
	// Build memory metric
	var memory *MetricValue
	if ct.Memory.Total > 0 {
		memory = &MetricValue{
			Current: ct.Memory.Usage,
			Total:   &ct.Memory.Total,
			Used:    &ct.Memory.Used,
			Free:    &ct.Memory.Free,
		}
	}

	// Build disk metric
	var disk *MetricValue
	if ct.Disk.Total > 0 {
		disk = &MetricValue{
			Current: ct.Disk.Usage,
			Total:   &ct.Disk.Total,
			Used:    &ct.Disk.Used,
			Free:    &ct.Disk.Free,
		}
	}

	// Build network metric
	network := &NetworkMetric{
		RXBytes: ct.NetworkIn,
		TXBytes: ct.NetworkOut,
	}

	// Build platform data
	var lastBackup *time.Time
	if !ct.LastBackup.IsZero() {
		lastBackup = &ct.LastBackup
	}
	platformData := ContainerPlatformData{
		VMID:        ct.VMID,
		Node:        ct.Node,
		Instance:    ct.Instance,
		Type:        ct.Type,
		CPUs:        ct.CPUs,
		Template:    ct.Template,
		Lock:        ct.Lock,
		OSName:      ct.OSName,
		IPAddresses: ct.IPAddresses,
		IsOCI:       ct.IsOCI || strings.EqualFold(strings.TrimSpace(ct.Type), "oci"),
		OSTemplate:  ct.OSTemplate,
		NetworkIn:   ct.NetworkIn,
		NetworkOut:  ct.NetworkOut,
		DiskRead:    ct.DiskRead,
		DiskWrite:   ct.DiskWrite,
		LastBackup:  lastBackup,
	}
	platformDataJSON, _ := json.Marshal(platformData)

	// Parent is the node - format matches Node.ID: instance-nodename
	parentID := fmt.Sprintf("%s-%s", ct.Instance, ct.Node)

	resourceType := ResourceTypeContainer
	if platformData.IsOCI {
		resourceType = ResourceTypeOCIContainer
	}

	return Resource{
		ID:           ct.ID,
		Type:         resourceType,
		Name:         ct.Name,
		PlatformID:   ct.Instance,
		PlatformType: PlatformProxmoxPVE,
		SourceType:   SourceAPI,
		ParentID:     parentID,
		Status:       mapGuestStatus(ct.Status),
		CPU: &MetricValue{
			Current: ct.CPU * 100, // Container CPU is 0-1, convert to percentage
		},
		Memory:       memory,
		Disk:         disk,
		Network:      network,
		Uptime:       &ct.Uptime,
		Tags:         ct.Tags,
		LastSeen:     ct.LastSeen,
		PlatformData: platformDataJSON,
		SchemaVersion: CurrentSchemaVersion,
	}
}

// FromHost converts a Host agent to a unified Resource.
func FromHost(h models.Host) Resource {
	// Build memory metric
	var memory *MetricValue
	if h.Memory.Total > 0 {
		memory = &MetricValue{
			Current: h.Memory.Usage,
			Total:   &h.Memory.Total,
			Used:    &h.Memory.Used,
			Free:    &h.Memory.Free,
		}
	}

	// Combine disk metrics from multiple disks
	var disk *MetricValue
	var totalDisk, usedDisk, freeDisk int64
	for _, d := range h.Disks {
		totalDisk += d.Total
		usedDisk += d.Used
		freeDisk += d.Free
	}
	if totalDisk > 0 {
		usage := float64(usedDisk) / float64(totalDisk) * 100
		disk = &MetricValue{
			Current: usage,
			Total:   &totalDisk,
			Used:    &usedDisk,
			Free:    &freeDisk,
		}
	}

	// Calculate network totals
	var rxTotal, txTotal int64
	for _, iface := range h.NetworkInterfaces {
		rxTotal += int64(iface.RXBytes)
		txTotal += int64(iface.TXBytes)
	}
	network := &NetworkMetric{
		RXBytes: rxTotal,
		TXBytes: txTotal,
	}

	// Get primary temperature
	var temp *float64
	if len(h.Sensors.TemperatureCelsius) > 0 {
		// Pick the first available temperature
		for _, t := range h.Sensors.TemperatureCelsius {
			temp = &t
			break
		}
	}

	// Build platform data
	disks := make([]DiskInfo, len(h.Disks))
	for i, d := range h.Disks {
		disks[i] = DiskInfo{
			Mountpoint: d.Mountpoint,
			Device:     d.Device,
			Type:       d.Type,
			Total:      d.Total,
			Used:       d.Used,
			Free:       d.Free,
			Usage:      d.Usage,
		}
	}

	interfaces := make([]NetworkInterface, len(h.NetworkInterfaces))
	for i, iface := range h.NetworkInterfaces {
		interfaces[i] = NetworkInterface{
			Name:      iface.Name,
			MAC:       iface.MAC,
			Addresses: iface.Addresses,
			RXBytes:   iface.RXBytes,
			TXBytes:   iface.TXBytes,
			SpeedMbps: iface.SpeedMbps,
		}
	}

	diskIO := make([]DiskIOStats, len(h.DiskIO))
	for i, d := range h.DiskIO {
		diskIO[i] = DiskIOStats{
			Device:     d.Device,
			ReadBytes:  d.ReadBytes,
			WriteBytes: d.WriteBytes,
			ReadOps:    d.ReadOps,
			WriteOps:   d.WriteOps,
			ReadTimeMs: d.ReadTime,
			WriteTimeMs: d.WriteTime,
			IOTimeMs:   d.IOTime,
		}
	}

	raid := make([]HostRAIDArray, len(h.RAID))
	for i, r := range h.RAID {
		devices := make([]HostRAIDDevice, len(r.Devices))
		for j, d := range r.Devices {
			devices[j] = HostRAIDDevice{
				Device: d.Device,
				State:  d.State,
				Slot:   d.Slot,
			}
		}
		raid[i] = HostRAIDArray{
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
		}
	}

	platformData := HostPlatformData{
		Platform:        h.Platform,
		OSName:          h.OSName,
		OSVersion:       h.OSVersion,
		KernelVersion:   h.KernelVersion,
		Architecture:    h.Architecture,
		CPUCount:        h.CPUCount,
		LoadAverage:     h.LoadAverage,
		AgentVersion:    h.AgentVersion,
		IsLegacy:        h.IsLegacy,
		Disks:           disks,
		Interfaces:      interfaces,
		DiskIO:          diskIO,
		RAID:            raid,
		TokenID:         h.TokenID,
		TokenName:       h.TokenName,
		TokenHint:       h.TokenHint,
		TokenLastUsedAt: h.TokenLastUsedAt,
		Sensors: HostSensorSummary{
			TemperatureCelsius: h.Sensors.TemperatureCelsius,
			FanRPM:             h.Sensors.FanRPM,
			Additional:         h.Sensors.Additional,
		},
	}
	platformDataJSON, _ := json.Marshal(platformData)

	// Collect IPs for identity
	var ips []string
	for _, iface := range h.NetworkInterfaces {
		ips = append(ips, iface.Addresses...)
	}

	return Resource{
		ID:           h.ID,
		Type:         ResourceTypeHost,
		Name:         h.Hostname,
		DisplayName:  h.DisplayName,
		PlatformID:   "host-agent", // No specific platform ID for host agents
		PlatformType: PlatformHostAgent,
		SourceType:   SourceAgent,
		Status:       mapHostStatus(h.Status),
		CPU: &MetricValue{
			Current: h.CPUUsage,
		},
		Memory:       memory,
		Disk:         disk,
		Network:      network,
		Temperature:  temp,
		Uptime:       &h.UptimeSeconds,
		Tags:         h.Tags,
		LastSeen:     h.LastSeen,
		PlatformData: platformDataJSON,
		Identity: &ResourceIdentity{
			Hostname: h.Hostname,
			IPs:      ips,
		},
		SchemaVersion: CurrentSchemaVersion,
	}
}

// FromDockerHost converts a DockerHost to a unified Resource.
func FromDockerHost(dh models.DockerHost) Resource {
	// Build memory metric
	var memory *MetricValue
	if dh.Memory.Total > 0 {
		memory = &MetricValue{
			Current: dh.Memory.Usage,
			Total:   &dh.Memory.Total,
			Used:    &dh.Memory.Used,
			Free:    &dh.Memory.Free,
		}
	}

	// Combine disk metrics
	var disk *MetricValue
	var totalDisk, usedDisk, freeDisk int64
	for _, d := range dh.Disks {
		totalDisk += d.Total
		usedDisk += d.Used
		freeDisk += d.Free
	}
	if totalDisk > 0 {
		usage := float64(usedDisk) / float64(totalDisk) * 100
		disk = &MetricValue{
			Current: usage,
			Total:   &totalDisk,
			Used:    &usedDisk,
			Free:    &freeDisk,
		}
	}

	// Calculate network totals
	var rxTotal, txTotal int64
	for _, iface := range dh.NetworkInterfaces {
		rxTotal += int64(iface.RXBytes)
		txTotal += int64(iface.TXBytes)
	}
	network := &NetworkMetric{
		RXBytes: rxTotal,
		TXBytes: txTotal,
	}

	// Build platform data
	disks := make([]DiskInfo, len(dh.Disks))
	for i, d := range dh.Disks {
		disks[i] = DiskInfo{
			Mountpoint: d.Mountpoint,
			Device:     d.Device,
			Type:       d.Type,
			Total:      d.Total,
			Used:       d.Used,
			Free:       d.Free,
			Usage:      d.Usage,
		}
	}

	interfaces := make([]NetworkInterface, len(dh.NetworkInterfaces))
	for i, iface := range dh.NetworkInterfaces {
		interfaces[i] = NetworkInterface{
			Name:      iface.Name,
			MAC:       iface.MAC,
			Addresses: iface.Addresses,
			RXBytes:   iface.RXBytes,
			TXBytes:   iface.TXBytes,
			SpeedMbps: iface.SpeedMbps,
		}
	}

	var swarm *DockerSwarmInfo
	if dh.Swarm != nil {
		swarm = &DockerSwarmInfo{
			NodeID:           dh.Swarm.NodeID,
			NodeRole:         dh.Swarm.NodeRole,
			LocalState:       dh.Swarm.LocalState,
			ControlAvailable: dh.Swarm.ControlAvailable,
			ClusterID:        dh.Swarm.ClusterID,
			ClusterName:      dh.Swarm.ClusterName,
			Scope:            dh.Swarm.Scope,
			Error:            dh.Swarm.Error,
		}
	}

	platformData := DockerHostPlatformData{
		AgentID:           dh.AgentID,
		MachineID:         dh.MachineID,
		OS:                dh.OS,
		KernelVersion:     dh.KernelVersion,
		Architecture:      dh.Architecture,
		Runtime:           dh.Runtime,
		RuntimeVersion:    dh.RuntimeVersion,
		DockerVersion:     dh.DockerVersion,
		LoadAverage:       dh.LoadAverage,
		AgentVersion:      dh.AgentVersion,
		CPUs:              dh.CPUs,
		IsLegacy:          dh.IsLegacy,
		Disks:             disks,
		Interfaces:        interfaces,
		CustomDisplayName: dh.CustomDisplayName,
		Hidden:            dh.Hidden,
		PendingUninstall:  dh.PendingUninstall,
		Swarm:             swarm,
		TokenID:           dh.TokenID,
		TokenName:         dh.TokenName,
		TokenHint:         dh.TokenHint,
		TokenLastUsedAt:   dh.TokenLastUsedAt,
	}
	platformDataJSON, _ := json.Marshal(platformData)

	// Collect IPs for identity
	var ips []string
	for _, iface := range dh.NetworkInterfaces {
		ips = append(ips, iface.Addresses...)
	}

	// Determine display name
	displayName := dh.DisplayName
	if dh.CustomDisplayName != "" {
		displayName = dh.CustomDisplayName
	}

	// Determine cluster ID from swarm
	clusterID := ""
	if dh.Swarm != nil && dh.Swarm.ClusterID != "" {
		clusterID = fmt.Sprintf("docker-swarm/%s", dh.Swarm.ClusterID)
	}

	return Resource{
		ID:           dh.ID,
		Type:         ResourceTypeDockerHost,
		Name:         dh.Hostname,
		DisplayName:  displayName,
		PlatformID:   dh.AgentID,
		PlatformType: PlatformDocker,
		SourceType:   SourceAgent,
		ClusterID:    clusterID,
		Status:       mapDockerHostStatus(dh.Status),
		CPU: &MetricValue{
			Current: dh.CPUUsage,
		},
		Memory:       memory,
		Disk:         disk,
		Network:      network,
		Uptime:       &dh.UptimeSeconds,
		LastSeen:     dh.LastSeen,
		PlatformData: platformDataJSON,
		Identity: &ResourceIdentity{
			Hostname:  dh.Hostname,
			MachineID: dh.MachineID,
			IPs:       ips,
		},
		SchemaVersion: CurrentSchemaVersion,
	}
}

// FromDockerContainer converts a DockerContainer to a unified Resource.
func FromDockerContainer(dc models.DockerContainer, hostID, hostName string) Resource {
	// Build memory metric
	memTotal := dc.MemoryLimit
	memUsed := dc.MemoryUsage
	var memory *MetricValue
	if memTotal > 0 {
		memory = &MetricValue{
			Current: dc.MemoryPercent,
			Total:   &memTotal,
			Used:    &memUsed,
		}
	}

	// Build platform data
	ports := make([]ContainerPort, len(dc.Ports))
	for i, p := range dc.Ports {
		ports[i] = ContainerPort{
			PrivatePort: p.PrivatePort,
			PublicPort:  p.PublicPort,
			Protocol:    p.Protocol,
			IP:          p.IP,
		}
	}

	networks := make([]ContainerNetwork, len(dc.Networks))
	for i, n := range dc.Networks {
		networks[i] = ContainerNetwork{
			Name: n.Name,
			IPv4: n.IPv4,
			IPv6: n.IPv6,
		}
	}

	var podman *PodmanContainerInfo
	if dc.Podman != nil {
		podman = &PodmanContainerInfo{
			PodName:        dc.Podman.PodName,
			PodID:          dc.Podman.PodID,
			Infra:          dc.Podman.Infra,
			ComposeProject: dc.Podman.ComposeProject,
			ComposeService: dc.Podman.ComposeService,
		}
	}

	platformData := DockerContainerPlatformData{
		HostID:       hostID,
		HostName:     hostName,
		Image:        dc.Image,
		State:        dc.State,
		Status:       dc.Status,
		Health:       dc.Health,
		RestartCount: dc.RestartCount,
		ExitCode:     dc.ExitCode,
		CreatedAt:    dc.CreatedAt,
		StartedAt:    dc.StartedAt,
		FinishedAt:   dc.FinishedAt,
		Labels:       dc.Labels,
		Ports:        ports,
		Networks:     networks,
		Podman:       podman,
	}
	platformDataJSON, _ := json.Marshal(platformData)

	// Create unique ID combining host and container
	resourceID := fmt.Sprintf("%s/%s", hostID, dc.ID)

	return Resource{
		ID:           resourceID,
		Type:         ResourceTypeDockerContainer,
		Name:         dc.Name,
		PlatformID:   hostID,
		PlatformType: PlatformDocker,
		SourceType:   SourceAgent,
		ParentID:     hostID,
		Status:       mapDockerContainerStatus(dc.State),
		CPU: &MetricValue{
			Current: dc.CPUPercent,
		},
		Memory:       memory,
		Uptime:       &dc.UptimeSeconds,
		Labels:       dc.Labels,
		LastSeen:     time.Now(), // Containers don't have their own LastSeen
		PlatformData: platformDataJSON,
		SchemaVersion: CurrentSchemaVersion,
	}
}

// FromPBSInstance converts a PBS instance to a unified Resource.
func FromPBSInstance(pbs models.PBSInstance) Resource {
	// Build memory metric
	var memory *MetricValue
	if pbs.MemoryTotal > 0 {
		memory = &MetricValue{
			Current: pbs.Memory,
			Total:   &pbs.MemoryTotal,
			Used:    &pbs.MemoryUsed,
		}
	}

	platformData := PBSPlatformData{
		Host:             pbs.Host,
		Version:          pbs.Version,
		ConnectionHealth: pbs.ConnectionHealth,
		MemoryUsed:       pbs.MemoryUsed,
		MemoryTotal:      pbs.MemoryTotal,
		NumDatastores:    len(pbs.Datastores),
	}
	platformDataJSON, _ := json.Marshal(platformData)

	return Resource{
		ID:           pbs.ID,
		Type:         ResourceTypePBS,
		Name:         pbs.Name,
		PlatformID:   pbs.Host,
		PlatformType: PlatformProxmoxPBS,
		SourceType:   SourceAPI,
		Status:       mapPBSStatus(pbs.Status, pbs.ConnectionHealth),
		CPU: &MetricValue{
			Current: pbs.CPU,
		},
		Memory:        memory,
		Uptime:        &pbs.Uptime,
		LastSeen:      pbs.LastSeen,
		PlatformData:  platformDataJSON,
		SchemaVersion: CurrentSchemaVersion,
	}
}

// FromStorage converts a Proxmox Storage to a unified Resource.
func FromStorage(s models.Storage) Resource {
	var disk *MetricValue
	if s.Total > 0 {
		disk = &MetricValue{
			Current: s.Usage,
			Total:   &s.Total,
			Used:    &s.Used,
			Free:    &s.Free,
		}
	}

	platformData := StoragePlatformData{
		Instance: s.Instance,
		Node:     s.Node,
		Nodes:    s.Nodes,
		Type:     s.Type,
		Content:  s.Content,
		Shared:   s.Shared,
		Enabled:  s.Enabled,
		Active:   s.Active,
	}
	platformDataJSON, _ := json.Marshal(platformData)

	status := StatusOnline
	if !s.Active {
		status = StatusOffline
	} else if !s.Enabled {
		status = StatusStopped
	}

	return Resource{
		ID:            s.ID,
		Type:          ResourceTypeStorage,
		Name:          s.Name,
		PlatformID:    s.Instance,
		PlatformType:  PlatformProxmoxPVE,
		SourceType:    SourceAPI,
		ParentID:      fmt.Sprintf("%s-%s", s.Instance, s.Node), // Format matches Node.ID: instance-nodename
		Status:        status,
		Disk:          disk,
		LastSeen:      time.Now(), // Storage doesn't have LastSeen
		PlatformData:  platformDataJSON,
		SchemaVersion: CurrentSchemaVersion,
	}
}

// Status mapping helpers

func mapNodeStatus(status string) ResourceStatus {
	switch strings.ToLower(status) {
	case "online":
		return StatusOnline
	case "offline":
		return StatusOffline
	default:
		return StatusUnknown
	}
}

func mapGuestStatus(status string) ResourceStatus {
	switch strings.ToLower(status) {
	case "running":
		return StatusRunning
	case "stopped":
		return StatusStopped
	case "paused":
		return StatusPaused
	default:
		return StatusUnknown
	}
}

func mapHostStatus(status string) ResourceStatus {
	switch strings.ToLower(status) {
	case "online":
		return StatusOnline
	case "offline":
		return StatusOffline
	case "degraded":
		return StatusDegraded
	default:
		return StatusUnknown
	}
}

func mapDockerHostStatus(status string) ResourceStatus {
	switch strings.ToLower(status) {
	case "online":
		return StatusOnline
	case "offline":
		return StatusOffline
	default:
		return StatusUnknown
	}
}

func mapDockerContainerStatus(state string) ResourceStatus {
	switch strings.ToLower(state) {
	case "running":
		return StatusRunning
	case "exited", "dead":
		return StatusStopped
	case "paused":
		return StatusPaused
	case "restarting", "created":
		return StatusUnknown
	default:
		return StatusUnknown
	}
}

func mapPBSStatus(status, connectionHealth string) ResourceStatus {
	if connectionHealth != "healthy" {
		return StatusDegraded
	}
	switch strings.ToLower(status) {
	case "online":
		return StatusOnline
	case "offline":
		return StatusOffline
	default:
		return StatusUnknown
	}
}
