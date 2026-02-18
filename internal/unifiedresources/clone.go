package unifiedresources

func cloneResourcePtr(in *Resource) *Resource {
	if in == nil {
		return nil
	}
	out := cloneResource(in)
	return &out
}

func cloneResource(in *Resource) Resource {
	if in == nil {
		return Resource{}
	}

	out := *in
	out.DiscoveryTarget = cloneDiscoveryTarget(in.DiscoveryTarget)
	out.MetricsTarget = cloneMetricsTarget(in.MetricsTarget)
	out.Sources = cloneDataSourceSlice(in.Sources)
	out.SourceStatus = cloneSourceStatusMap(in.SourceStatus)
	out.Identity = cloneResourceIdentity(in.Identity)
	out.Metrics = cloneResourceMetrics(in.Metrics)
	out.ParentID = cloneStringPtr(in.ParentID)
	out.Tags = cloneStringSlice(in.Tags)
	out.Proxmox = cloneProxmoxData(in.Proxmox)
	out.Storage = cloneStorageMeta(in.Storage)
	out.Agent = cloneAgentData(in.Agent)
	out.Docker = cloneDockerData(in.Docker)
	out.PBS = clonePBSData(in.PBS)
	out.PMG = clonePMGData(in.PMG)
	out.Kubernetes = cloneK8sData(in.Kubernetes)
	out.PhysicalDisk = clonePhysicalDiskMeta(in.PhysicalDisk)
	out.Ceph = cloneCephMeta(in.Ceph)
	return out
}

func cloneDiscoveryTarget(in *DiscoveryTarget) *DiscoveryTarget {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneMetricsTarget(in *MetricsTarget) *MetricsTarget {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneResourceIdentity(in ResourceIdentity) ResourceIdentity {
	out := in
	out.Hostnames = cloneStringSlice(in.Hostnames)
	out.IPAddresses = cloneStringSlice(in.IPAddresses)
	out.MACAddresses = cloneStringSlice(in.MACAddresses)
	return out
}

func cloneResourceMetrics(in *ResourceMetrics) *ResourceMetrics {
	if in == nil {
		return nil
	}
	out := *in
	out.CPU = cloneMetricValue(in.CPU)
	out.Memory = cloneMetricValue(in.Memory)
	out.Disk = cloneMetricValue(in.Disk)
	out.NetIn = cloneMetricValue(in.NetIn)
	out.NetOut = cloneMetricValue(in.NetOut)
	out.DiskRead = cloneMetricValue(in.DiskRead)
	out.DiskWrite = cloneMetricValue(in.DiskWrite)
	return &out
}

func cloneMetricValue(in *MetricValue) *MetricValue {
	if in == nil {
		return nil
	}
	out := *in
	out.Used = cloneInt64Ptr(in.Used)
	out.Total = cloneInt64Ptr(in.Total)
	return &out
}

func cloneProxmoxData(in *ProxmoxData) *ProxmoxData {
	if in == nil {
		return nil
	}
	out := *in
	out.Temperature = cloneFloat64Ptr(in.Temperature)
	out.CPUInfo = cloneCPUInfo(in.CPUInfo)
	out.LoadAverage = cloneFloat64Slice(in.LoadAverage)
	return &out
}

func cloneStorageMeta(in *StorageMeta) *StorageMeta {
	if in == nil {
		return nil
	}
	out := *in
	out.ContentTypes = cloneStringSlice(in.ContentTypes)
	return &out
}

func cloneAgentData(in *AgentData) *AgentData {
	if in == nil {
		return nil
	}
	out := *in
	out.Temperature = cloneFloat64Ptr(in.Temperature)
	out.NetworkInterfaces = cloneNetworkInterfaces(in.NetworkInterfaces)
	out.Disks = cloneDiskInfos(in.Disks)
	out.Sensors = cloneHostSensorMeta(in.Sensors)
	out.RAID = cloneHostRAIDMetaSlice(in.RAID)
	out.DiskIO = cloneHostDiskIOMetaSlice(in.DiskIO)
	out.Ceph = cloneHostCephMeta(in.Ceph)
	return &out
}

func cloneDockerData(in *DockerData) *DockerData {
	if in == nil {
		return nil
	}
	out := *in
	out.Temperature = cloneFloat64Ptr(in.Temperature)
	out.Ports = cloneDockerPortMetaSlice(in.Ports)
	out.Labels = cloneStringMap(in.Labels)
	out.Networks = cloneDockerNetworkMetaSlice(in.Networks)
	out.Mounts = cloneDockerMountMetaSlice(in.Mounts)
	out.UpdateStatus = cloneDockerUpdateStatusMeta(in.UpdateStatus)
	out.Swarm = cloneDockerSwarmInfo(in.Swarm)
	out.NetworkInterfaces = cloneNetworkInterfaces(in.NetworkInterfaces)
	out.Disks = cloneDiskInfos(in.Disks)
	return &out
}

func clonePBSData(in *PBSData) *PBSData {
	if in == nil {
		return nil
	}
	out := *in
	out.Datastores = clonePBSDatastoreMetaSlice(in.Datastores)
	return &out
}

func clonePMGData(in *PMGData) *PMGData {
	if in == nil {
		return nil
	}
	out := *in
	out.Nodes = clonePMGNodeMetaSlice(in.Nodes)
	out.MailStats = clonePMGMailStatsMeta(in.MailStats)
	out.Quarantine = clonePMGQuarantineMeta(in.Quarantine)
	out.SpamDistribution = clonePMGSpamBucketMetaSlice(in.SpamDistribution)
	out.RelayDomains = clonePMGRelayDomainMetaSlice(in.RelayDomains)
	out.DomainStats = clonePMGDomainStatMetaSlice(in.DomainStats)
	return &out
}

func clonePMGRelayDomainMetaSlice(in []PMGRelayDomainMeta) []PMGRelayDomainMeta {
	if in == nil {
		return nil
	}
	out := make([]PMGRelayDomainMeta, len(in))
	copy(out, in)
	return out
}

func clonePMGDomainStatMetaSlice(in []PMGDomainStatMeta) []PMGDomainStatMeta {
	if in == nil {
		return nil
	}
	out := make([]PMGDomainStatMeta, len(in))
	copy(out, in)
	return out
}

func cloneK8sData(in *K8sData) *K8sData {
	if in == nil {
		return nil
	}
	out := *in
	out.Roles = cloneStringSlice(in.Roles)
	out.Temperature = cloneFloat64Ptr(in.Temperature)
	out.Labels = cloneStringMap(in.Labels)
	out.MetricCapabilities = cloneKubernetesMetricCapabilities(in.MetricCapabilities)
	return &out
}

func clonePhysicalDiskMeta(in *PhysicalDiskMeta) *PhysicalDiskMeta {
	if in == nil {
		return nil
	}
	out := *in
	out.SMART = cloneSMARTMeta(in.SMART)
	return &out
}

func cloneCephMeta(in *CephMeta) *CephMeta {
	if in == nil {
		return nil
	}
	out := *in
	out.Pools = cloneCephPoolMetaSlice(in.Pools)
	out.Services = cloneCephServiceMetaSlice(in.Services)
	return &out
}

func cloneCPUInfo(in *CPUInfo) *CPUInfo {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneSMARTMeta(in *SMARTMeta) *SMARTMeta {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneHostSensorMeta(in *HostSensorMeta) *HostSensorMeta {
	if in == nil {
		return nil
	}
	out := *in
	out.TemperatureCelsius = cloneStringFloat64Map(in.TemperatureCelsius)
	out.FanRPM = cloneStringFloat64Map(in.FanRPM)
	out.Additional = cloneStringFloat64Map(in.Additional)
	out.SMART = cloneHostSMARTMetaSlice(in.SMART)
	return &out
}

func cloneHostCephMeta(in *HostCephMeta) *HostCephMeta {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneDockerUpdateStatusMeta(in *DockerUpdateStatusMeta) *DockerUpdateStatusMeta {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneDockerSwarmInfo(in *DockerSwarmInfo) *DockerSwarmInfo {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func clonePMGMailStatsMeta(in *PMGMailStatsMeta) *PMGMailStatsMeta {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func clonePMGQuarantineMeta(in *PMGQuarantineMeta) *PMGQuarantineMeta {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneNetworkInterfaces(in []NetworkInterface) []NetworkInterface {
	if in == nil {
		return nil
	}
	out := make([]NetworkInterface, len(in))
	for i := range in {
		out[i] = in[i]
		out[i].Addresses = cloneStringSlice(in[i].Addresses)
		out[i].SpeedMbps = cloneInt64Ptr(in[i].SpeedMbps)
	}
	return out
}

func cloneDiskInfos(in []DiskInfo) []DiskInfo {
	if in == nil {
		return nil
	}
	out := make([]DiskInfo, len(in))
	copy(out, in)
	return out
}

func cloneHostSMARTMetaSlice(in []HostSMARTMeta) []HostSMARTMeta {
	if in == nil {
		return nil
	}
	out := make([]HostSMARTMeta, len(in))
	copy(out, in)
	return out
}

func cloneHostRAIDMetaSlice(in []HostRAIDMeta) []HostRAIDMeta {
	if in == nil {
		return nil
	}
	out := make([]HostRAIDMeta, len(in))
	copy(out, in)
	return out
}

func cloneHostDiskIOMetaSlice(in []HostDiskIOMeta) []HostDiskIOMeta {
	if in == nil {
		return nil
	}
	out := make([]HostDiskIOMeta, len(in))
	copy(out, in)
	return out
}

func cloneDockerPortMetaSlice(in []DockerPortMeta) []DockerPortMeta {
	if in == nil {
		return nil
	}
	out := make([]DockerPortMeta, len(in))
	copy(out, in)
	return out
}

func cloneDockerNetworkMetaSlice(in []DockerNetworkMeta) []DockerNetworkMeta {
	if in == nil {
		return nil
	}
	out := make([]DockerNetworkMeta, len(in))
	copy(out, in)
	return out
}

func cloneDockerMountMetaSlice(in []DockerMountMeta) []DockerMountMeta {
	if in == nil {
		return nil
	}
	out := make([]DockerMountMeta, len(in))
	copy(out, in)
	return out
}

func clonePBSDatastoreMetaSlice(in []PBSDatastoreMeta) []PBSDatastoreMeta {
	if in == nil {
		return nil
	}
	out := make([]PBSDatastoreMeta, len(in))
	copy(out, in)
	return out
}

func clonePMGNodeMetaSlice(in []PMGNodeMeta) []PMGNodeMeta {
	if in == nil {
		return nil
	}
	out := make([]PMGNodeMeta, len(in))
	for i := range in {
		out[i] = in[i]
		out[i].QueueStatus = clonePMGQueueMeta(in[i].QueueStatus)
	}
	return out
}

func clonePMGQueueMeta(in *PMGQueueMeta) *PMGQueueMeta {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func clonePMGSpamBucketMetaSlice(in []PMGSpamBucketMeta) []PMGSpamBucketMeta {
	if in == nil {
		return nil
	}
	out := make([]PMGSpamBucketMeta, len(in))
	copy(out, in)
	return out
}

func cloneCephPoolMetaSlice(in []CephPoolMeta) []CephPoolMeta {
	if in == nil {
		return nil
	}
	out := make([]CephPoolMeta, len(in))
	copy(out, in)
	return out
}

func cloneCephServiceMetaSlice(in []CephServiceMeta) []CephServiceMeta {
	if in == nil {
		return nil
	}
	out := make([]CephServiceMeta, len(in))
	copy(out, in)
	return out
}

func cloneDataSourceSlice(in []DataSource) []DataSource {
	if in == nil {
		return nil
	}
	out := make([]DataSource, len(in))
	copy(out, in)
	return out
}

func cloneSourceStatusMap(in map[DataSource]SourceStatus) map[DataSource]SourceStatus {
	if in == nil {
		return nil
	}
	out := make(map[DataSource]SourceStatus, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneStringMap(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneStringFloat64Map(in map[string]float64) map[string]float64 {
	if in == nil {
		return nil
	}
	out := make(map[string]float64, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneStringSlice(in []string) []string {
	if in == nil {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}

func cloneFloat64Slice(in []float64) []float64 {
	if in == nil {
		return nil
	}
	out := make([]float64, len(in))
	copy(out, in)
	return out
}

func cloneStringPtr(in *string) *string {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneInt64Ptr(in *int64) *int64 {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneFloat64Ptr(in *float64) *float64 {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}
