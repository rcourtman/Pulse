package models

import (
	"strings"
	"time"
)

// ToFrontend converts a State to StateFrontend
func (s *State) ToFrontend() StateFrontend {
	return s.GetSnapshot().ToFrontend()
}

// ToFrontend converts a Node to NodeFrontend
func (n Node) ToFrontend() NodeFrontend {
	nf := NodeFrontend{
		ID:                           n.ID,
		Node:                         n.Name,
		Name:                         n.Name,
		DisplayName:                  n.DisplayName,
		Instance:                     n.Instance,
		Host:                         n.Host,
		Status:                       n.Status,
		Type:                         n.Type,
		CPU:                          n.CPU,
		Mem:                          n.Memory.Used,
		MaxMem:                       n.Memory.Total,
		MaxDisk:                      n.Disk.Total,
		Uptime:                       n.Uptime,
		LoadAverage:                  n.LoadAverage,
		KernelVersion:                n.KernelVersion,
		PVEVersion:                   n.PVEVersion,
		CPUInfo:                      n.CPUInfo,
		LastSeen:                     n.LastSeen.Unix() * 1000,
		ConnectionHealth:             n.ConnectionHealth,
		IsClusterMember:              n.IsClusterMember,
		ClusterName:                  n.ClusterName,
		TemperatureMonitoringEnabled: n.TemperatureMonitoringEnabled,
	}

	// Include full Memory object if it has data
	if n.Memory.Total > 0 {
		nf.Memory = &n.Memory
	}

	// Include full Disk object if it has data
	if n.Disk.Total > 0 {
		nf.Disk = &n.Disk
	}

	// Include temperature data if available
	if n.Temperature != nil && n.Temperature.Available {
		nf.Temperature = n.Temperature
	}

	if nf.DisplayName == "" {
		nf.DisplayName = nf.Name
	}

	return nf
}

// ToFrontend converts a VM to VMFrontend
func (v VM) ToFrontend() VMFrontend {
	vm := VMFrontend{
		ID:               v.ID,
		VMID:             v.VMID,
		Name:             v.Name,
		Node:             v.Node,
		Instance:         v.Instance,
		Status:           v.Status,
		Type:             v.Type,
		CPU:              v.CPU,
		CPUs:             v.CPUs,
		Mem:              v.Memory.Used,
		MaxMem:           v.Memory.Total,
		NetIn:            zeroIfNegative(v.NetworkIn),
		NetOut:           zeroIfNegative(v.NetworkOut),
		DiskRead:         zeroIfNegative(v.DiskRead),
		DiskWrite:        zeroIfNegative(v.DiskWrite),
		Uptime:           v.Uptime,
		Template:         v.Template,
		Lock:             v.Lock,
		LastSeen:         v.LastSeen.Unix() * 1000,
		DiskStatusReason: v.DiskStatusReason,
	}

	// Convert tags array to string
	if len(v.Tags) > 0 {
		vm.Tags = strings.Join(v.Tags, ",")
	}

	// Convert last backup time if not zero
	if !v.LastBackup.IsZero() {
		vm.LastBackup = v.LastBackup.Unix() * 1000
	}

	// Include full Memory object if it has data
	if v.Memory.Total > 0 {
		vm.Memory = &v.Memory
	}

	// Include full Disk object if it has data
	if v.Disk.Total > 0 {
		vm.DiskObj = &v.Disk
	}

	// Include individual disks array if available
	if len(v.Disks) > 0 {
		vm.Disks = v.Disks
	}

	if len(v.IPAddresses) > 0 {
		vm.IPAddresses = append([]string(nil), v.IPAddresses...)
	}

	if v.OSName != "" {
		vm.OSName = v.OSName
	}

	if v.OSVersion != "" {
		vm.OSVersion = v.OSVersion
	}

	if v.AgentVersion != "" {
		vm.AgentVersion = v.AgentVersion
	}

	if len(v.NetworkInterfaces) > 0 {
		vm.NetworkInterfaces = make([]GuestNetworkInterface, len(v.NetworkInterfaces))
		copy(vm.NetworkInterfaces, v.NetworkInterfaces)
	}

	return vm
}

// ToFrontend converts a Container to ContainerFrontend
func (c Container) ToFrontend() ContainerFrontend {
	ct := ContainerFrontend{
		ID:        c.ID,
		VMID:      c.VMID,
		Name:      c.Name,
		Node:      c.Node,
		Instance:  c.Instance,
		Status:    c.Status,
		Type:      c.Type,
		CPU:       c.CPU,
		CPUs:      c.CPUs,
		Mem:       c.Memory.Used,
		MaxMem:    c.Memory.Total,
		NetIn:     zeroIfNegative(c.NetworkIn),
		NetOut:    zeroIfNegative(c.NetworkOut),
		DiskRead:  zeroIfNegative(c.DiskRead),
		DiskWrite: zeroIfNegative(c.DiskWrite),
		Uptime:    c.Uptime,
		Template:  c.Template,
		Lock:      c.Lock,
		LastSeen:  c.LastSeen.Unix() * 1000,
	}

	// Convert tags array to string
	if len(c.Tags) > 0 {
		ct.Tags = strings.Join(c.Tags, ",")
	}

	// Convert last backup time if not zero
	if !c.LastBackup.IsZero() {
		ct.LastBackup = c.LastBackup.Unix() * 1000
	}

	// Include full Memory object if it has data
	if c.Memory.Total > 0 {
		ct.Memory = &c.Memory
	}

	// Include full Disk object if it has data
	if c.Disk.Total > 0 {
		ct.DiskObj = &c.Disk
	}

	// Include individual disks array if available
	if len(c.Disks) > 0 {
		ct.Disks = c.Disks
	}

	if len(c.IPAddresses) > 0 {
		ct.IPAddresses = append([]string(nil), c.IPAddresses...)
	}

	if len(c.NetworkInterfaces) > 0 {
		ct.NetworkInterfaces = make([]GuestNetworkInterface, len(c.NetworkInterfaces))
		copy(ct.NetworkInterfaces, c.NetworkInterfaces)
	}

	if c.OSName != "" {
		ct.OSName = c.OSName
	}

	return ct
}

// ToFrontend converts a DockerHost to DockerHostFrontend
func (d DockerHost) ToFrontend() DockerHostFrontend {
	h := DockerHostFrontend{
		ID:                d.ID,
		AgentID:           d.AgentID,
		Hostname:          d.Hostname,
		DisplayName:       d.DisplayName,
		CustomDisplayName: d.CustomDisplayName,
		MachineID:         d.MachineID,
		OS:                d.OS,
		KernelVersion:     d.KernelVersion,
		Architecture:      d.Architecture,
		Runtime:           d.Runtime,
		RuntimeVersion:    d.RuntimeVersion,
		DockerVersion:     d.DockerVersion,
		CPUs:              d.CPUs,
		TotalMemoryBytes:  d.TotalMemoryBytes,
		UptimeSeconds:     d.UptimeSeconds,
		CPUUsagePercent:   d.CPUUsage,
		Status:            d.Status,
		LastSeen:          d.LastSeen.Unix() * 1000,
		IntervalSeconds:   d.IntervalSeconds,
		AgentVersion:      d.AgentVersion,
		Containers:        make([]DockerContainerFrontend, len(d.Containers)),
	}

	if h.DisplayName == "" {
		h.DisplayName = h.Hostname
	}

	h.PendingUninstall = d.PendingUninstall

	if d.TokenID != "" {
		h.TokenID = d.TokenID
		h.TokenName = d.TokenName
		h.TokenHint = d.TokenHint
		if d.TokenLastUsedAt != nil && !d.TokenLastUsedAt.IsZero() {
			ts := d.TokenLastUsedAt.Unix() * 1000
			h.TokenLastUsedAt = &ts
		}
	}

	for i, ct := range d.Containers {
		h.Containers[i] = ct.ToFrontend()
	}

	if len(d.Services) > 0 {
		h.Services = make([]DockerServiceFrontend, len(d.Services))
		for i, svc := range d.Services {
			h.Services[i] = svc.ToFrontend()
		}
	}

	if len(d.Tasks) > 0 {
		h.Tasks = make([]DockerTaskFrontend, len(d.Tasks))
		for i, task := range d.Tasks {
			h.Tasks[i] = task.ToFrontend()
		}
	}

	if d.Swarm != nil {
		sw := d.Swarm.ToFrontend()
		h.Swarm = &sw
	}

	if len(d.LoadAverage) > 0 {
		h.LoadAverage = append([]float64(nil), d.LoadAverage...)
	}

	if (d.Memory != Memory{}) {
		mem := d.Memory
		h.Memory = &mem
	}

	if len(d.Disks) > 0 {
		h.Disks = append([]Disk(nil), d.Disks...)
	}

	if len(d.NetworkInterfaces) > 0 {
		h.NetworkInterfaces = make([]HostNetworkInterface, len(d.NetworkInterfaces))
		copy(h.NetworkInterfaces, d.NetworkInterfaces)
	}

	if d.Command != nil {
		h.Command = toDockerHostCommandFrontend(*d.Command)
	}

	return h
}

// ToFrontend converts a RemovedDockerHost to its frontend representation.
func (r RemovedDockerHost) ToFrontend() RemovedDockerHostFrontend {
	return RemovedDockerHostFrontend{
		ID:          r.ID,
		Hostname:    r.Hostname,
		DisplayName: r.DisplayName,
		RemovedAt:   r.RemovedAt.Unix() * 1000,
	}
}

// ToFrontend converts a Host to HostFrontend.
func (h Host) ToFrontend() HostFrontend {
	host := HostFrontend{
		ID:              h.ID,
		Hostname:        h.Hostname,
		DisplayName:     h.DisplayName,
		Platform:        h.Platform,
		OSName:          h.OSName,
		OSVersion:       h.OSVersion,
		KernelVersion:   h.KernelVersion,
		Architecture:    h.Architecture,
		CPUCount:        h.CPUCount,
		CPUUsage:        h.CPUUsage,
		Status:          h.Status,
		UptimeSeconds:   h.UptimeSeconds,
		IntervalSeconds: h.IntervalSeconds,
		AgentVersion:    h.AgentVersion,
		TokenID:         h.TokenID,
		TokenName:       h.TokenName,
		TokenHint:       h.TokenHint,
		Tags:            append([]string(nil), h.Tags...),
		LastSeen:        h.LastSeen.Unix() * 1000,
	}

	// Fall back to Hostname if DisplayName is empty
	if host.DisplayName == "" && h.Hostname != "" {
		host.DisplayName = h.Hostname
	}

	if len(h.LoadAverage) > 0 {
		host.LoadAverage = append([]float64(nil), h.LoadAverage...)
	}

	if (h.Memory != Memory{}) {
		mem := h.Memory
		host.Memory = &mem
	}

	if len(h.Disks) > 0 {
		host.Disks = append([]Disk(nil), h.Disks...)
	}

	if len(h.DiskIO) > 0 {
		host.DiskIO = append([]DiskIO(nil), h.DiskIO...)
	}

	if len(h.NetworkInterfaces) > 0 {
		host.NetworkInterfaces = make([]HostNetworkInterface, len(h.NetworkInterfaces))
		copy(host.NetworkInterfaces, h.NetworkInterfaces)
	}

	if s := hostSensorSummaryToFrontend(h.Sensors); s != nil {
		host.Sensors = s
	}

	if h.TokenLastUsedAt != nil && !h.TokenLastUsedAt.IsZero() {
		ts := h.TokenLastUsedAt.Unix() * 1000
		host.TokenLastUsedAt = &ts
	}

	return host
}

// ToFrontend converts a DockerContainer to DockerContainerFrontend
func (c DockerContainer) ToFrontend() DockerContainerFrontend {
	container := DockerContainerFrontend{
		ID:                  c.ID,
		Name:                c.Name,
		Image:               c.Image,
		State:               c.State,
		Status:              c.Status,
		Health:              c.Health,
		CPUPercent:          c.CPUPercent,
		MemoryUsage:         c.MemoryUsage,
		MemoryLimit:         c.MemoryLimit,
		MemoryPercent:       c.MemoryPercent,
		UptimeSeconds:       c.UptimeSeconds,
		RestartCount:        c.RestartCount,
		ExitCode:            c.ExitCode,
		CreatedAt:           c.CreatedAt.Unix() * 1000,
		Labels:              c.Labels,
		WritableLayerBytes:  c.WritableLayerBytes,
		RootFilesystemBytes: c.RootFilesystemBytes,
	}

	if c.StartedAt != nil {
		ms := c.StartedAt.Unix() * 1000
		container.StartedAt = &ms
	}

	if c.FinishedAt != nil {
		ms := c.FinishedAt.Unix() * 1000
		container.FinishedAt = &ms
	}

	if len(c.Ports) > 0 {
		ports := make([]DockerContainerPortFrontend, len(c.Ports))
		for i, port := range c.Ports {
			ports[i] = DockerContainerPortFrontend(port)
		}
		container.Ports = ports
	}

	if len(c.Networks) > 0 {
		networks := make([]DockerContainerNetworkFrontend, len(c.Networks))
		for i, net := range c.Networks {
			networks[i] = DockerContainerNetworkFrontend(net)
		}
		container.Networks = networks
	}

	if c.BlockIO != nil {
		container.BlockIO = &DockerContainerBlockIOFrontend{
			ReadBytes:               c.BlockIO.ReadBytes,
			WriteBytes:              c.BlockIO.WriteBytes,
			ReadRateBytesPerSecond:  c.BlockIO.ReadRateBytesPerSecond,
			WriteRateBytesPerSecond: c.BlockIO.WriteRateBytesPerSecond,
		}
	}

	if len(c.Mounts) > 0 {
		mounts := make([]DockerContainerMountFrontend, len(c.Mounts))
		for i, mount := range c.Mounts {
			mounts[i] = DockerContainerMountFrontend(mount)
		}
		container.Mounts = mounts
	}

	if c.Podman != nil {
		container.Podman = &DockerPodmanContainerFrontend{
			PodName:           c.Podman.PodName,
			PodID:             c.Podman.PodID,
			Infra:             c.Podman.Infra,
			ComposeProject:    c.Podman.ComposeProject,
			ComposeService:    c.Podman.ComposeService,
			ComposeWorkdir:    c.Podman.ComposeWorkdir,
			ComposeConfigHash: c.Podman.ComposeConfigHash,
			AutoUpdatePolicy:  c.Podman.AutoUpdatePolicy,
			AutoUpdateRestart: c.Podman.AutoUpdateRestart,
			UserNamespace:     c.Podman.UserNamespace,
		}
	}

	return container
}

// ToFrontend converts a DockerService to DockerServiceFrontend.
func (s DockerService) ToFrontend() DockerServiceFrontend {
	service := DockerServiceFrontend{
		ID:             s.ID,
		Name:           s.Name,
		Stack:          s.Stack,
		Image:          s.Image,
		Mode:           s.Mode,
		DesiredTasks:   s.DesiredTasks,
		RunningTasks:   s.RunningTasks,
		CompletedTasks: s.CompletedTasks,
		Labels:         nil,
	}

	if len(s.Labels) > 0 {
		service.Labels = make(map[string]string, len(s.Labels))
		for k, v := range s.Labels {
			service.Labels[k] = v
		}
	}

	if len(s.EndpointPorts) > 0 {
		service.EndpointPorts = make([]DockerServicePortFrontend, len(s.EndpointPorts))
		for i, port := range s.EndpointPorts {
			service.EndpointPorts[i] = port.ToFrontend()
		}
	}

	if s.UpdateStatus != nil {
		update := s.UpdateStatus.ToFrontend()
		service.UpdateStatus = &update
	}

	if s.CreatedAt != nil && !s.CreatedAt.IsZero() {
		ts := s.CreatedAt.Unix() * 1000
		service.CreatedAt = &ts
	}
	if s.UpdatedAt != nil && !s.UpdatedAt.IsZero() {
		ts := s.UpdatedAt.Unix() * 1000
		service.UpdatedAt = &ts
	}

	return service
}

// ToFrontend converts a DockerServicePort to DockerServicePortFrontend.
func (p DockerServicePort) ToFrontend() DockerServicePortFrontend {
	return DockerServicePortFrontend(p)
}

// ToFrontend converts a DockerServiceUpdate to DockerServiceUpdateFrontend.
func (u DockerServiceUpdate) ToFrontend() DockerServiceUpdateFrontend {
	update := DockerServiceUpdateFrontend{
		State:   u.State,
		Message: u.Message,
	}
	if u.CompletedAt != nil && !u.CompletedAt.IsZero() {
		ts := u.CompletedAt.Unix() * 1000
		update.CompletedAt = &ts
	}
	return update
}

// ToFrontend converts a DockerTask to DockerTaskFrontend.
func (t DockerTask) ToFrontend() DockerTaskFrontend {
	task := DockerTaskFrontend{
		ID:            t.ID,
		ServiceID:     t.ServiceID,
		ServiceName:   t.ServiceName,
		Slot:          t.Slot,
		NodeID:        t.NodeID,
		NodeName:      t.NodeName,
		DesiredState:  t.DesiredState,
		CurrentState:  t.CurrentState,
		Error:         t.Error,
		Message:       t.Message,
		ContainerID:   t.ContainerID,
		ContainerName: t.ContainerName,
	}

	if !t.CreatedAt.IsZero() {
		ts := t.CreatedAt.Unix() * 1000
		task.CreatedAt = &ts
	}
	if t.UpdatedAt != nil && !t.UpdatedAt.IsZero() {
		ts := t.UpdatedAt.Unix() * 1000
		task.UpdatedAt = &ts
	}
	if t.StartedAt != nil && !t.StartedAt.IsZero() {
		ts := t.StartedAt.Unix() * 1000
		task.StartedAt = &ts
	}
	if t.CompletedAt != nil && !t.CompletedAt.IsZero() {
		ts := t.CompletedAt.Unix() * 1000
		task.CompletedAt = &ts
	}

	return task
}

// ToFrontend converts DockerSwarmInfo to DockerSwarmFrontend.
func (s DockerSwarmInfo) ToFrontend() DockerSwarmFrontend {
	return DockerSwarmFrontend(s)
}

func hostSensorSummaryToFrontend(src HostSensorSummary) *HostSensorSummaryFrontend {
	if len(src.TemperatureCelsius) == 0 && len(src.FanRPM) == 0 && len(src.Additional) == 0 {
		return nil
	}

	dest := &HostSensorSummaryFrontend{}
	if len(src.TemperatureCelsius) > 0 {
		dest.TemperatureCelsius = copyStringFloatMap(src.TemperatureCelsius)
	}
	if len(src.FanRPM) > 0 {
		dest.FanRPM = copyStringFloatMap(src.FanRPM)
	}
	if len(src.Additional) > 0 {
		dest.Additional = copyStringFloatMap(src.Additional)
	}
	return dest
}

func copyStringFloatMap(src map[string]float64) map[string]float64 {
	if len(src) == 0 {
		return nil
	}
	dest := make(map[string]float64, len(src))
	for k, v := range src {
		dest[k] = v
	}
	return dest
}

func toDockerHostCommandFrontend(cmd DockerHostCommandStatus) *DockerHostCommandFrontend {
	result := &DockerHostCommandFrontend{
		ID:        cmd.ID,
		Type:      cmd.Type,
		Status:    cmd.Status,
		Message:   cmd.Message,
		CreatedAt: cmd.CreatedAt.Unix() * 1000,
		UpdatedAt: cmd.UpdatedAt.Unix() * 1000,
	}

	if cmd.DispatchedAt != nil {
		ms := cmd.DispatchedAt.Unix() * 1000
		result.DispatchedAt = &ms
	}
	if cmd.AcknowledgedAt != nil {
		ms := cmd.AcknowledgedAt.Unix() * 1000
		result.AcknowledgedAt = &ms
	}
	if cmd.CompletedAt != nil {
		ms := cmd.CompletedAt.Unix() * 1000
		result.CompletedAt = &ms
	}
	if cmd.FailedAt != nil {
		ms := cmd.FailedAt.Unix() * 1000
		result.FailedAt = &ms
	}
	if cmd.FailureReason != "" {
		result.FailureReason = cmd.FailureReason
	}
	if cmd.ExpiresAt != nil {
		ms := cmd.ExpiresAt.Unix() * 1000
		result.ExpiresAt = &ms
	}

	return result
}

// ToFrontend converts Storage to StorageFrontend
func (s Storage) ToFrontend() StorageFrontend {
	return StorageFrontend{
		ID:        s.ID,
		Storage:   s.Name,
		Name:      s.Name,
		Node:      s.Node,
		Instance:  s.Instance,
		Nodes:     s.Nodes,
		NodeIDs:   s.NodeIDs,
		NodeCount: s.NodeCount,
		Type:      s.Type,
		Status:    s.Status,
		Total:     s.Total,
		Used:      s.Used,
		Avail:     s.Free,
		Free:      s.Free,
		Usage:     s.Usage,
		Content:   s.Content,
		Shared:    s.Shared,
		Enabled:   s.Enabled,
		Active:    s.Active,
	}
}

// ToFrontend converts a CephCluster to CephClusterFrontend
func (c CephCluster) ToFrontend() CephClusterFrontend {
	frontend := CephClusterFrontend{
		ID:             c.ID,
		Instance:       c.Instance,
		Name:           c.Name,
		FSID:           c.FSID,
		Health:         c.Health,
		HealthMessage:  c.HealthMessage,
		TotalBytes:     c.TotalBytes,
		UsedBytes:      c.UsedBytes,
		AvailableBytes: c.AvailableBytes,
		UsagePercent:   c.UsagePercent,
		NumMons:        c.NumMons,
		NumMgrs:        c.NumMgrs,
		NumOSDs:        c.NumOSDs,
		NumOSDsUp:      c.NumOSDsUp,
		NumOSDsIn:      c.NumOSDsIn,
		NumPGs:         c.NumPGs,
		LastUpdated:    c.LastUpdated.Unix() * 1000,
	}

	if len(c.Pools) > 0 {
		frontend.Pools = append([]CephPool(nil), c.Pools...)
	}

	if len(c.Services) > 0 {
		frontend.Services = append([]CephServiceStatus(nil), c.Services...)
	}

	return frontend
}

// ToFrontend converts a replication job to a frontend representation.
func (r ReplicationJob) ToFrontend() ReplicationJobFrontend {
	frontend := ReplicationJobFrontend{
		ID:                      r.ID,
		Instance:                r.Instance,
		JobID:                   r.JobID,
		JobNumber:               r.JobNumber,
		Guest:                   r.Guest,
		GuestID:                 r.GuestID,
		GuestName:               r.GuestName,
		GuestType:               r.GuestType,
		GuestNode:               r.GuestNode,
		SourceNode:              r.SourceNode,
		SourceStorage:           r.SourceStorage,
		TargetNode:              r.TargetNode,
		TargetStorage:           r.TargetStorage,
		Schedule:                r.Schedule,
		Type:                    r.Type,
		Enabled:                 r.Enabled,
		State:                   r.State,
		Status:                  r.Status,
		LastSyncStatus:          r.LastSyncStatus,
		LastSyncUnix:            r.LastSyncUnix,
		LastSyncDurationSeconds: r.LastSyncDurationSeconds,
		LastSyncDurationHuman:   r.LastSyncDurationHuman,
		NextSyncUnix:            r.NextSyncUnix,
		DurationSeconds:         r.DurationSeconds,
		DurationHuman:           r.DurationHuman,
		FailCount:               r.FailCount,
		Error:                   r.Error,
		Comment:                 r.Comment,
		RemoveJob:               r.RemoveJob,
		RateLimitMbps:           r.RateLimitMbps,
	}

	if r.LastSyncTime != nil {
		frontend.LastSyncTime = r.LastSyncTime.UnixMilli()
	}

	if r.NextSyncTime != nil {
		frontend.NextSyncTime = r.NextSyncTime.UnixMilli()
	}

	polledAt := r.LastPolled
	if polledAt.IsZero() {
		polledAt = time.Now()
	}
	frontend.PolledAt = polledAt.UnixMilli()

	return frontend
}

// zeroIfNegative returns 0 for negative values (used for I/O metrics)
func zeroIfNegative(val int64) int64 {
	if val < 0 {
		return 0
	}
	return val
}

// ResourceToFrontend converts a resources.Resource to ResourceFrontend.
// This function is in models package to avoid circular imports.
// It takes individual fields rather than the whole Resource to avoid importing resources package.
type ResourceConvertInput struct {
	ID           string
	Type         string
	Name         string
	DisplayName  string
	PlatformID   string
	PlatformType string
	SourceType   string
	ParentID     string
	ClusterID    string
	Status       string
	CPU          *ResourceMetricInput
	Memory       *ResourceMetricInput
	Disk         *ResourceMetricInput
	NetworkRX    int64
	NetworkTX    int64
	HasNetwork   bool
	Temperature  *float64
	Uptime       *int64
	Tags         []string
	Labels       map[string]string
	LastSeenUnix int64
	Alerts       []ResourceAlertInput
	Identity     *ResourceIdentityInput
	PlatformData map[string]any
}

// ResourceMetricInput represents a metric value for resource conversion.
type ResourceMetricInput struct {
	Current float64
	Total   *int64
	Used    *int64
	Free    *int64
}

type ResourceAlertInput struct {
	ID            string
	Type          string
	Level         string
	Message       string
	Value         float64
	Threshold     float64
	StartTimeUnix int64
}

type ResourceIdentityInput struct {
	Hostname  string
	MachineID string
	IPs       []string
}

// ConvertResourceToFrontend converts input to ResourceFrontend.
func ConvertResourceToFrontend(input ResourceConvertInput) ResourceFrontend {
	rf := ResourceFrontend{
		ID:           input.ID,
		Type:         input.Type,
		Name:         input.Name,
		DisplayName:  input.DisplayName,
		PlatformID:   input.PlatformID,
		PlatformType: input.PlatformType,
		SourceType:   input.SourceType,
		ParentID:     input.ParentID,
		ClusterID:    input.ClusterID,
		Status:       input.Status,
		Temperature:  input.Temperature,
		Uptime:       input.Uptime,
		Tags:         input.Tags,
		Labels:       input.Labels,
		LastSeen:     input.LastSeenUnix,
		PlatformData: input.PlatformData,
	}

	// Convert metrics
	if input.CPU != nil {
		rf.CPU = &ResourceMetricFrontend{
			Current: input.CPU.Current,
			Total:   input.CPU.Total,
			Used:    input.CPU.Used,
			Free:    input.CPU.Free,
		}
	}

	if input.Memory != nil {
		rf.Memory = &ResourceMetricFrontend{
			Current: input.Memory.Current,
			Total:   input.Memory.Total,
			Used:    input.Memory.Used,
			Free:    input.Memory.Free,
		}
	}

	if input.Disk != nil {
		rf.Disk = &ResourceMetricFrontend{
			Current: input.Disk.Current,
			Total:   input.Disk.Total,
			Used:    input.Disk.Used,
			Free:    input.Disk.Free,
		}
	}

	if input.HasNetwork {
		rf.Network = &ResourceNetworkFrontend{
			RXBytes: input.NetworkRX,
			TXBytes: input.NetworkTX,
		}
	}

	// Convert alerts
	if len(input.Alerts) > 0 {
		rf.Alerts = make([]ResourceAlertFrontend, len(input.Alerts))
		for i, a := range input.Alerts {
			rf.Alerts[i] = ResourceAlertFrontend{
				ID:        a.ID,
				Type:      a.Type,
				Level:     a.Level,
				Message:   a.Message,
				Value:     a.Value,
				Threshold: a.Threshold,
				StartTime: a.StartTimeUnix,
			}
		}
	}

	// Convert identity
	if input.Identity != nil {
		rf.Identity = &ResourceIdentityFrontend{
			Hostname:  input.Identity.Hostname,
			MachineID: input.Identity.MachineID,
			IPs:       input.Identity.IPs,
		}
	}

	return rf
}
