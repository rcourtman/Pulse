package unifiedresources

import (
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

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
	out.DiscoveryReadiness = cloneResourceDiscoveryReadiness(in.DiscoveryReadiness)
	out.MetricsTarget = cloneMetricsTarget(in.MetricsTarget)
	out.PlatformScopes = cloneStringSlice(in.PlatformScopes)
	out.Sources = cloneDataSourceSlice(in.Sources)
	out.SourceStatus = cloneSourceStatusMap(in.SourceStatus)
	out.Identity = cloneResourceIdentity(in.Identity)
	out.Metrics = cloneResourceMetrics(in.Metrics)
	out.ParentID = cloneStringPtr(in.ParentID)
	out.parentBySource = cloneParentBySourceMap(in.parentBySource)
	out.Tags = cloneStringSlice(in.Tags)
	out.Incidents = cloneResourceIncidentSlice(in.Incidents)
	out.Proxmox = cloneProxmoxData(in.Proxmox)
	out.Storage = cloneStorageMeta(in.Storage)
	out.Agent = cloneAgentData(in.Agent)
	out.Docker = cloneDockerData(in.Docker)
	out.PBS = clonePBSData(in.PBS)
	out.PMG = clonePMGData(in.PMG)
	out.Kubernetes = cloneK8sData(in.Kubernetes)
	out.PhysicalDisk = clonePhysicalDiskMeta(in.PhysicalDisk)
	out.Ceph = cloneCephMeta(in.Ceph)
	out.TrueNAS = cloneTrueNASData(in.TrueNAS)
	out.VMware = cloneVMwareData(in.VMware)
	out.Availability = cloneAvailabilityData(in.Availability)
	out.FacetCounts = resourceFacetCounts(out)
	RefreshCanonicalMetadata(&out)
	return out
}

func resourceFacetCounts(resource Resource) ResourceFacetCounts {
	return ResourceFacetCounts{
		RecentChanges: len(resource.RecentChanges),
	}
}

func cloneParentBySourceMap(in map[DataSource]string) map[DataSource]string {
	if len(in) == 0 {
		if in == nil {
			return nil
		}
		return map[DataSource]string{}
	}
	out := make(map[DataSource]string, len(in))
	for source, parentID := range in {
		out[source] = parentID
	}
	return out
}

func cloneDiscoveryTarget(in *DiscoveryTarget) *DiscoveryTarget {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneResourceDiscoveryReadiness(in *ResourceDiscoveryReadiness) *ResourceDiscoveryReadiness {
	if in == nil {
		return nil
	}
	out := *in
	out.ObservedAt = cloneTimePtr(in.ObservedAt)
	return &out
}

func cloneMetricsTarget(in *MetricsTarget) *MetricsTarget {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneTimePtr(in *time.Time) *time.Time {
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
	out.TemperatureDetails = cloneTemperature(in.TemperatureDetails)
	out.CPUInfo = cloneCPUInfo(in.CPUInfo)
	out.LoadAverage = cloneFloat64Slice(in.LoadAverage)
	out.NetworkInterfaces = cloneNetworkInterfaces(in.NetworkInterfaces)
	out.DockerCheckedAt = cloneTimePtr(in.DockerCheckedAt)
	out.TemperatureMonitoringEnabled = cloneBoolPtr(in.TemperatureMonitoringEnabled)
	out.PendingUpdatesCheckedAt = cloneTimePtr(in.PendingUpdatesCheckedAt)
	return &out
}

func cloneStorageMeta(in *StorageMeta) *StorageMeta {
	if in == nil {
		return nil
	}
	out := *in
	out.ContentTypes = cloneStringSlice(in.ContentTypes)
	out.Nodes = cloneStringSlice(in.Nodes)
	out.ConsumerTypes = cloneStringSlice(in.ConsumerTypes)
	out.TopConsumers = cloneStorageConsumerMetaSlice(in.TopConsumers)
	out.Risk = cloneStorageRisk(in.Risk)
	return &out
}

func cloneStorageConsumerMetaSlice(in []StorageConsumerMeta) []StorageConsumerMeta {
	if len(in) == 0 {
		return nil
	}
	out := make([]StorageConsumerMeta, len(in))
	copy(out, in)
	return out
}

func cloneResourceIncidentSlice(in []ResourceIncident) []ResourceIncident {
	if len(in) == 0 {
		return nil
	}
	out := make([]ResourceIncident, len(in))
	copy(out, in)
	return out
}

func cloneAgentData(in *AgentData) *AgentData {
	if in == nil {
		return nil
	}
	out := *in
	out.Temperature = cloneFloat64Ptr(in.Temperature)
	out.LoadAverage = cloneFloat64Slice(in.LoadAverage)
	out.NetworkInterfaces = cloneNetworkInterfaces(in.NetworkInterfaces)
	out.Disks = cloneDiskInfos(in.Disks)
	out.Sensors = cloneHostSensorMeta(in.Sensors)
	out.RAID = cloneHostRAIDMetaSlice(in.RAID)
	out.Unraid = cloneHostUnraidMeta(in.Unraid)
	out.DiskIO = cloneHostDiskIOMetaSlice(in.DiskIO)
	out.Ceph = cloneHostCephMeta(in.Ceph)
	out.StorageRisk = cloneStorageRisk(in.StorageRisk)
	out.PackageUpdates = cloneAgentPackageUpdateMeta(in.PackageUpdates)
	out.StorageCleanup = cloneAgentStorageCleanupMeta(in.StorageCleanup)
	out.DiskExclude = cloneStringSlice(in.DiskExclude)
	return &out
}

func cloneAgentPackageUpdateMeta(in *AgentPackageUpdateMeta) *AgentPackageUpdateMeta {
	if in == nil {
		return nil
	}
	out := *in
	out.Packages = append([]AgentPackageUpdate(nil), in.Packages...)
	return &out
}

func cloneAgentStorageCleanupMeta(in *AgentStorageCleanupMeta) *AgentStorageCleanupMeta {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneDockerData(in *DockerData) *DockerData {
	if in == nil {
		return nil
	}
	out := *in
	out.OOMKilled = cloneBoolPtr(in.OOMKilled)
	out.Temperature = cloneFloat64Ptr(in.Temperature)
	out.LoadAverage = cloneFloat64Slice(in.LoadAverage)
	out.TokenLastUsedAt = cloneTimePtr(in.TokenLastUsedAt)
	out.UpdatesLastCheckedAt = cloneTimePtr(in.UpdatesLastCheckedAt)
	out.ImagesUsage = cloneDockerStorageUsageMeta(in.ImagesUsage)
	out.ContainersUsage = cloneDockerStorageUsageMeta(in.ContainersUsage)
	out.VolumesUsage = cloneDockerStorageUsageMeta(in.VolumesUsage)
	out.BuildCacheUsage = cloneDockerStorageUsageMeta(in.BuildCacheUsage)
	out.Ports = cloneDockerPortMetaSlice(in.Ports)
	out.Labels = cloneStringMap(in.Labels)
	out.EngineLabels = cloneStringMap(in.EngineLabels)
	out.Networks = cloneDockerNetworkMetaSlice(in.Networks)
	out.Mounts = cloneDockerMountMetaSlice(in.Mounts)
	out.UpdateStatus = cloneDockerUpdateStatusMeta(in.UpdateStatus)
	out.BlockIO = cloneDockerContainerBlockIOMeta(in.BlockIO)
	out.Podman = cloneDockerPodmanContainerMeta(in.Podman)
	out.RepoTags = cloneStringSlice(in.RepoTags)
	out.RepoDigests = cloneStringSlice(in.RepoDigests)
	out.Options = cloneStringMap(in.Options)
	out.Subnets = cloneDockerNetworkSubnetMetaSlice(in.Subnets)
	out.StartedAt = cloneTimePtr(in.StartedAt)
	out.FinishedAt = cloneTimePtr(in.FinishedAt)
	out.CompletedAt = cloneTimePtr(in.CompletedAt)
	out.ObjectCreatedAt = cloneTimePtr(in.ObjectCreatedAt)
	out.ObjectUpdatedAt = cloneTimePtr(in.ObjectUpdatedAt)
	out.Command = cloneDockerHostCommandStatus(in.Command)
	out.Security = cloneDockerHostSecurity(in.Security)
	out.Swarm = cloneDockerSwarmInfo(in.Swarm)
	out.NetworkInterfaces = cloneNetworkInterfaces(in.NetworkInterfaces)
	out.Disks = cloneDiskInfos(in.Disks)
	out.Containers = append([]models.DockerContainer(nil), in.Containers...)
	out.Images = append([]models.DockerImage(nil), in.Images...)
	out.Volumes = append([]models.DockerVolume(nil), in.Volumes...)
	out.NetworksRaw = append([]models.DockerNetwork(nil), in.NetworksRaw...)
	out.Services = append([]models.DockerService(nil), in.Services...)
	out.Tasks = append([]models.DockerTask(nil), in.Tasks...)
	out.Nodes = append([]models.DockerNode(nil), in.Nodes...)
	out.Secrets = append([]models.DockerSecret(nil), in.Secrets...)
	out.Configs = append([]models.DockerConfig(nil), in.Configs...)
	return &out
}

func cloneDockerContainerBlockIOMeta(in *DockerContainerBlockIOMeta) *DockerContainerBlockIOMeta {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneDockerPodmanContainerMeta(in *DockerPodmanContainerMeta) *DockerPodmanContainerMeta {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func clonePBSData(in *PBSData) *PBSData {
	if in == nil {
		return nil
	}
	out := *in
	out.Datastores = clonePBSDatastoreMetaSlice(in.Datastores)
	out.DatastoreDetails = clonePBSDatastores(in.DatastoreDetails)
	out.StorageRisk = cloneStorageRisk(in.StorageRisk)
	out.AffectedDatastores = cloneStringSlice(in.AffectedDatastores)
	out.ProtectedWorkloadTypes = cloneStringSlice(in.ProtectedWorkloadTypes)
	out.ProtectedWorkloadNames = cloneStringSlice(in.ProtectedWorkloadNames)
	out.BackupJobs = append([]models.PBSBackupJob(nil), in.BackupJobs...)
	out.SyncJobs = append([]models.PBSSyncJob(nil), in.SyncJobs...)
	out.VerifyJobs = append([]models.PBSVerifyJob(nil), in.VerifyJobs...)
	out.PruneJobs = append([]models.PBSPruneJob(nil), in.PruneJobs...)
	out.GarbageJobs = append([]models.PBSGarbageJob(nil), in.GarbageJobs...)
	out.JobHealthEvidence = append([]models.PBSJobHealthEvidence(nil), in.JobHealthEvidence...)
	return &out
}

func clonePMGData(in *PMGData) *PMGData {
	if in == nil {
		return nil
	}
	out := *in
	out.Nodes = clonePMGNodeMetaSlice(in.Nodes)
	out.MailStats = clonePMGMailStatsMeta(in.MailStats)
	out.MailCount = append([]models.PMGMailCountPoint(nil), in.MailCount...)
	out.Quarantine = clonePMGQuarantineMeta(in.Quarantine)
	out.SpamDistribution = clonePMGSpamBucketMetaSlice(in.SpamDistribution)
	out.RelayDomains = clonePMGRelayDomainMetaSlice(in.RelayDomains)
	out.DomainStats = clonePMGDomainStatMetaSlice(in.DomainStats)
	return &out
}

func cloneAvailabilityData(in *AvailabilityData) *AvailabilityData {
	if in == nil {
		return nil
	}
	out := *in
	out.LastChecked = cloneTimePtr(in.LastChecked)
	out.LastSuccess = cloneTimePtr(in.LastSuccess)
	return &out
}

func cloneVMwareData(in *VMwareData) *VMwareData {
	if in == nil {
		return nil
	}
	out := *in
	out.DatastoreIDs = cloneStringSlice(in.DatastoreIDs)
	out.DatastoreNames = cloneStringSlice(in.DatastoreNames)
	out.ClusterHAEnabled = cloneBoolPtr(in.ClusterHAEnabled)
	out.ClusterDRSEnabled = cloneBoolPtr(in.ClusterDRSEnabled)
	out.DatastoreAccessible = cloneBoolPtr(in.DatastoreAccessible)
	out.MultipleHostAccess = cloneBoolPtr(in.MultipleHostAccess)
	out.NetworkHostIDs = cloneStringSlice(in.NetworkHostIDs)
	out.NetworkHostNames = cloneStringSlice(in.NetworkHostNames)
	out.NetworkVMIDs = cloneStringSlice(in.NetworkVMIDs)
	out.NetworkVMNames = cloneStringSlice(in.NetworkVMNames)
	out.GuestIPAddresses = cloneStringSlice(in.GuestIPAddresses)
	out.SnapshotTree = cloneVMwareSnapshotDataSlice(in.SnapshotTree)
	out.NetworkAdapters = cloneVMwareNetworkAdapterDataSlice(in.NetworkAdapters)
	out.VirtualDisks = cloneVMwareVirtualDiskDataSlice(in.VirtualDisks)
	out.Tools = cloneVMwareToolsData(in.Tools)
	out.Hardware = cloneVMwareVMHardwareData(in.Hardware)
	return &out
}

func cloneVMwareSnapshotDataSlice(in []VMwareSnapshotData) []VMwareSnapshotData {
	if in == nil {
		return nil
	}
	out := make([]VMwareSnapshotData, len(in))
	for i := range in {
		out[i] = in[i]
		out[i].CreatedAt = cloneTimePtr(in[i].CreatedAt)
		out[i].Children = cloneVMwareSnapshotDataSlice(in[i].Children)
	}
	return out
}

func cloneVMwareNetworkAdapterDataSlice(in []VMwareNetworkAdapterData) []VMwareNetworkAdapterData {
	if in == nil {
		return nil
	}
	out := make([]VMwareNetworkAdapterData, len(in))
	for i := range in {
		out[i] = in[i]
		out[i].PCISlotNumber = cloneInt64Ptr(in[i].PCISlotNumber)
	}
	return out
}

func cloneVMwareVirtualDiskDataSlice(in []VMwareVirtualDiskData) []VMwareVirtualDiskData {
	if in == nil {
		return nil
	}
	out := make([]VMwareVirtualDiskData, len(in))
	for i := range in {
		out[i] = in[i]
		out[i].IDEPrimary = cloneBoolPtr(in[i].IDEPrimary)
		out[i].IDEMaster = cloneBoolPtr(in[i].IDEMaster)
		out[i].SCSIBus = cloneInt64Ptr(in[i].SCSIBus)
		out[i].SCSIUnit = cloneInt64Ptr(in[i].SCSIUnit)
		out[i].SATABus = cloneInt64Ptr(in[i].SATABus)
		out[i].SATAUnit = cloneInt64Ptr(in[i].SATAUnit)
		out[i].NVMEBus = cloneInt64Ptr(in[i].NVMEBus)
		out[i].NVMEUnit = cloneInt64Ptr(in[i].NVMEUnit)
		out[i].CapacityBytes = cloneInt64Ptr(in[i].CapacityBytes)
	}
	return out
}

func cloneVMwareToolsData(in *VMwareToolsData) *VMwareToolsData {
	if in == nil {
		return nil
	}
	out := *in
	out.AutoUpdateSupported = cloneBoolPtr(in.AutoUpdateSupported)
	out.InstallAttemptCount = cloneInt64Ptr(in.InstallAttemptCount)
	out.VersionNumber = cloneInt64Ptr(in.VersionNumber)
	out.GuestRebootRequested = cloneBoolPtr(in.GuestRebootRequested)
	out.GuestRebootComponents = cloneStringSlice(in.GuestRebootComponents)
	return &out
}

func cloneVMwareVMHardwareData(in *VMwareVMHardwareData) *VMwareVMHardwareData {
	if in == nil {
		return nil
	}
	out := *in
	out.InstantCloneFrozen = cloneBoolPtr(in.InstantCloneFrozen)
	out.EFILegacyBoot = cloneBoolPtr(in.EFILegacyBoot)
	out.BootDelayMilliseconds = cloneInt64Ptr(in.BootDelayMilliseconds)
	out.BootRetry = cloneBoolPtr(in.BootRetry)
	out.BootRetryDelayMilliseconds = cloneInt64Ptr(in.BootRetryDelayMilliseconds)
	out.EnterSetupMode = cloneBoolPtr(in.EnterSetupMode)
	out.BootDevices = cloneVMwareBootDeviceDataSlice(in.BootDevices)
	out.CPUCoresPerSocket = cloneInt64Ptr(in.CPUCoresPerSocket)
	out.CPUHotAddEnabled = cloneBoolPtr(in.CPUHotAddEnabled)
	out.CPUHotRemoveEnabled = cloneBoolPtr(in.CPUHotRemoveEnabled)
	out.MemoryHotAddEnabled = cloneBoolPtr(in.MemoryHotAddEnabled)
	out.MemoryHotAddIncrementMiB = cloneInt64Ptr(in.MemoryHotAddIncrementMiB)
	out.MemoryHotAddLimitMiB = cloneInt64Ptr(in.MemoryHotAddLimitMiB)
	return &out
}

func cloneVMwareBootDeviceDataSlice(in []VMwareBootDeviceData) []VMwareBootDeviceData {
	if in == nil {
		return nil
	}
	out := make([]VMwareBootDeviceData, len(in))
	for i := range in {
		out[i] = in[i]
		out[i].Disks = cloneStringSlice(in[i].Disks)
	}
	return out
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
	out.ExternalIPs = cloneStringSlice(in.ExternalIPs)
	out.ServicePorts = cloneK8sServicePortSlice(in.ServicePorts)
	out.Selector = cloneStringMap(in.Selector)
	out.LastScheduleTime = cloneTimePtr(in.LastScheduleTime)
	out.LastSuccessfulTime = cloneTimePtr(in.LastSuccessfulTime)
	out.StartTime = cloneTimePtr(in.StartTime)
	out.CompletionTime = cloneTimePtr(in.CompletionTime)
	out.Hosts = cloneStringSlice(in.Hosts)
	out.Addresses = cloneStringSlice(in.Addresses)
	out.EndpointPorts = cloneK8sEndpointPortSlice(in.EndpointPorts)
	out.PolicyTypes = cloneStringSlice(in.PolicyTypes)
	out.AllowVolumeExpansion = cloneBoolPtr(in.AllowVolumeExpansion)
	out.ParameterKeys = cloneStringSlice(in.ParameterKeys)
	out.DataKeys = cloneStringSlice(in.DataKeys)
	out.BinaryDataKeys = cloneStringSlice(in.BinaryDataKeys)
	out.AutomountServiceAccountToken = cloneBoolPtr(in.AutomountServiceAccountToken)
	out.ImagePullSecrets = cloneStringSlice(in.ImagePullSecrets)
	out.Hard = cloneStringMap(in.Hard)
	out.Used = cloneStringMap(in.Used)
	out.LimitTypes = cloneStringSlice(in.LimitTypes)
	out.MetricTypes = cloneStringSlice(in.MetricTypes)
	out.AccessModes = cloneStringSlice(in.AccessModes)
	out.FirstSeen = cloneTimePtr(in.FirstSeen)
	out.EventTime = cloneTimePtr(in.EventTime)
	out.CreatedAt = cloneTimePtr(in.CreatedAt)
	out.MetricCapabilities = cloneKubernetesMetricCapabilities(in.MetricCapabilities)
	out.PodContainers = cloneK8sPodContainerSlice(in.PodContainers)
	return &out
}

func cloneK8sServicePortSlice(in []K8sServicePort) []K8sServicePort {
	if len(in) == 0 {
		return nil
	}
	out := make([]K8sServicePort, len(in))
	copy(out, in)
	return out
}

func cloneK8sEndpointPortSlice(in []K8sEndpointPort) []K8sEndpointPort {
	if len(in) == 0 {
		return nil
	}
	out := make([]K8sEndpointPort, len(in))
	copy(out, in)
	return out
}

func cloneK8sPodContainerSlice(in []K8sPodContainer) []K8sPodContainer {
	if len(in) == 0 {
		return nil
	}
	out := make([]K8sPodContainer, len(in))
	copy(out, in) // K8sPodContainer has no pointer/slice/map fields — shallow copy is sufficient
	return out
}

func clonePhysicalDiskMeta(in *PhysicalDiskMeta) *PhysicalDiskMeta {
	if in == nil {
		return nil
	}
	out := *in
	out.TemperatureAggregate = cloneTemperatureAggregateMeta(in.TemperatureAggregate)
	out.SMART = cloneSMARTMeta(in.SMART)
	out.Risk = clonePhysicalDiskRisk(in.Risk)
	return &out
}

func cloneTemperatureAggregateMeta(in *TemperatureAggregateMeta) *TemperatureAggregateMeta {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneHostUnraidMeta(in *HostUnraidMeta) *HostUnraidMeta {
	if in == nil {
		return nil
	}
	out := *in
	if len(in.Disks) > 0 {
		out.Disks = make([]HostUnraidDiskMeta, len(in.Disks))
		copy(out.Disks, in.Disks)
	}
	out.Risk = cloneStorageRisk(in.Risk)
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

func cloneTrueNASData(in *TrueNASData) *TrueNASData {
	if in == nil {
		return nil
	}
	out := *in
	out.StorageRisk = cloneStorageRisk(in.StorageRisk)
	out.App = cloneTrueNASApp(in.App)
	out.VM = cloneTrueNASVM(in.VM)
	out.Share = cloneTrueNASShare(in.Share)
	out.Services = cloneTrueNASServices(in.Services)
	return &out
}

func cloneTrueNASServices(in []TrueNASService) []TrueNASService {
	if len(in) == 0 {
		return nil
	}
	out := make([]TrueNASService, len(in))
	for i := range in {
		out[i] = in[i]
		out[i].PIDs = append([]int(nil), in[i].PIDs...)
	}
	return out
}

func cloneTrueNASApp(in *TrueNASApp) *TrueNASApp {
	if in == nil {
		return nil
	}
	out := *in
	out.UsedHostIPs = append([]string(nil), in.UsedHostIPs...)
	out.UsedPorts = cloneTrueNASAppPorts(in.UsedPorts)
	out.Containers = cloneTrueNASAppContainers(in.Containers)
	out.Volumes = append([]TrueNASAppVolume(nil), in.Volumes...)
	out.Images = append([]string(nil), in.Images...)
	out.Networks = cloneTrueNASAppNetworks(in.Networks)
	if in.Stats != nil {
		stats := *in.Stats
		out.Stats = &stats
	}
	return &out
}

func cloneTrueNASAppPorts(in []TrueNASAppPort) []TrueNASAppPort {
	if len(in) == 0 {
		return nil
	}
	out := make([]TrueNASAppPort, len(in))
	for i := range in {
		out[i] = in[i]
		out[i].HostPorts = append([]TrueNASAppHostPort(nil), in[i].HostPorts...)
	}
	return out
}

func cloneTrueNASAppContainers(in []TrueNASAppContainer) []TrueNASAppContainer {
	if len(in) == 0 {
		return nil
	}
	out := make([]TrueNASAppContainer, len(in))
	for i := range in {
		out[i] = in[i]
		out[i].PortConfig = cloneTrueNASAppPorts(in[i].PortConfig)
		out[i].VolumeMounts = append([]TrueNASAppVolume(nil), in[i].VolumeMounts...)
	}
	return out
}

func cloneTrueNASAppNetworks(in []TrueNASAppNetwork) []TrueNASAppNetwork {
	if len(in) == 0 {
		return nil
	}
	out := make([]TrueNASAppNetwork, len(in))
	for i := range in {
		out[i] = in[i]
		out[i].Labels = cloneStringMap(in[i].Labels)
	}
	return out
}

func cloneTrueNASVM(in *TrueNASVM) *TrueNASVM {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneTrueNASShare(in *TrueNASShare) *TrueNASShare {
	if in == nil {
		return nil
	}
	out := *in
	out.Aliases = append([]string(nil), in.Aliases...)
	out.Hosts = append([]string(nil), in.Hosts...)
	out.Networks = append([]string(nil), in.Networks...)
	out.Security = append([]string(nil), in.Security...)
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

func clonePhysicalDiskRisk(in *PhysicalDiskRisk) *PhysicalDiskRisk {
	if in == nil {
		return nil
	}
	out := *in
	out.Reasons = clonePhysicalDiskRiskReasonSlice(in.Reasons)
	return &out
}

func clonePhysicalDiskRiskReasonSlice(in []PhysicalDiskRiskReason) []PhysicalDiskRiskReason {
	if len(in) == 0 {
		return nil
	}
	out := make([]PhysicalDiskRiskReason, len(in))
	copy(out, in)
	return out
}

func cloneHostSensorMeta(in *HostSensorMeta) *HostSensorMeta {
	if in == nil {
		return nil
	}
	out := *in
	out.TemperatureCelsius = cloneStringFloat64Map(in.TemperatureCelsius)
	out.FanRPM = cloneStringFloat64Map(in.FanRPM)
	out.PowerWatts = cloneStringFloat64Map(in.PowerWatts)
	out.Additional = cloneStringFloat64Map(in.Additional)
	out.GPU = cloneHostGPUSensors(in.GPU)
	out.ThermalState = cloneHostThermalState(in.ThermalState)
	out.SMART = cloneHostSMARTMetaSlice(in.SMART)
	return &out
}

func cloneHostGPUSensors(in []HostGPUSensor) []HostGPUSensor {
	if len(in) == 0 {
		return nil
	}
	out := make([]HostGPUSensor, len(in))
	for i, gpu := range in {
		out[i] = HostGPUSensor{
			ID:                 gpu.ID,
			Name:               gpu.Name,
			TemperatureCelsius: cloneFloat64Ptr(gpu.TemperatureCelsius),
			UtilizationPercent: cloneFloat64Ptr(gpu.UtilizationPercent),
			MemoryUsedBytes:    cloneInt64Ptr(gpu.MemoryUsedBytes),
			MemoryTotalBytes:   cloneInt64Ptr(gpu.MemoryTotalBytes),
		}
	}
	return out
}

func cloneHostThermalState(in *HostThermalState) *HostThermalState {
	if in == nil {
		return nil
	}
	out := *in
	out.ThermalWarningLevel = cloneIntPtr(in.ThermalWarningLevel)
	out.PerformanceWarningLevel = cloneIntPtr(in.PerformanceWarningLevel)
	out.CPUPowerStatus = cloneIntPtr(in.CPUPowerStatus)
	out.LimitsPercent = cloneStringIntMap(in.LimitsPercent)
	return &out
}

func cloneHostCephMeta(in *HostCephMeta) *HostCephMeta {
	if in == nil {
		return nil
	}
	out := *in
	out.Health = cloneHostCephHealthMeta(in.Health)
	out.MonMap = cloneHostCephMonitorMapMeta(in.MonMap)
	out.Pools = cloneHostCephPoolMetaSlice(in.Pools)
	out.Services = cloneHostCephServiceMetaSlice(in.Services)
	return &out
}

func cloneDockerUpdateStatusMeta(in *DockerUpdateStatusMeta) *DockerUpdateStatusMeta {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneDockerStorageUsageMeta(in *DockerStorageUsageMeta) *DockerStorageUsageMeta {
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

func cloneDockerHostCommandStatus(in *models.DockerHostCommandStatus) *models.DockerHostCommandStatus {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneDockerHostSecurity(in *models.DockerHostSecurity) *models.DockerHostSecurity {
	if in == nil {
		return nil
	}
	out := *in
	out.AuthorizationPlugins = append([]string(nil), in.AuthorizationPlugins...)
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
	for i := range in {
		out[i] = in[i]
		out[i].Attributes = cloneSMARTAttributes(in[i].Attributes)
	}
	return out
}

func cloneSMARTAttributes(in *models.SMARTAttributes) *models.SMARTAttributes {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneHostRAIDMetaSlice(in []HostRAIDMeta) []HostRAIDMeta {
	if in == nil {
		return nil
	}
	out := make([]HostRAIDMeta, len(in))
	for i := range in {
		out[i] = in[i]
		out[i].Devices = cloneHostRAIDDeviceMetaSlice(in[i].Devices)
		out[i].Risk = cloneStorageRisk(in[i].Risk)
	}
	return out
}

func cloneStorageRisk(in *StorageRisk) *StorageRisk {
	if in == nil {
		return nil
	}
	out := *in
	out.Reasons = cloneStorageRiskReasonSlice(in.Reasons)
	return &out
}

func cloneStorageRiskReasonSlice(in []StorageRiskReason) []StorageRiskReason {
	if len(in) == 0 {
		return nil
	}
	out := make([]StorageRiskReason, len(in))
	copy(out, in)
	return out
}

func cloneHostRAIDDeviceMetaSlice(in []HostRAIDDeviceMeta) []HostRAIDDeviceMeta {
	if in == nil {
		return nil
	}
	out := make([]HostRAIDDeviceMeta, len(in))
	copy(out, in)
	return out
}

func cloneHostCephHealthMeta(in HostCephHealthMeta) HostCephHealthMeta {
	out := HostCephHealthMeta{
		Status:  in.Status,
		Checks:  make(map[string]HostCephCheckMeta, len(in.Checks)),
		Summary: make([]HostCephHealthSummaryMeta, len(in.Summary)),
	}
	for name, check := range in.Checks {
		out.Checks[name] = HostCephCheckMeta{
			Severity: check.Severity,
			Message:  check.Message,
			Detail:   append([]string(nil), check.Detail...),
		}
	}
	copy(out.Summary, in.Summary)
	return out
}

func cloneHostCephMonitorMapMeta(in HostCephMonitorMapMeta) HostCephMonitorMapMeta {
	out := in
	out.Monitors = cloneHostCephMonitorMetaSlice(in.Monitors)
	return out
}

func cloneHostCephMonitorMetaSlice(in []HostCephMonitorMeta) []HostCephMonitorMeta {
	if in == nil {
		return nil
	}
	out := make([]HostCephMonitorMeta, len(in))
	copy(out, in)
	return out
}

func cloneHostCephPoolMetaSlice(in []HostCephPoolMeta) []HostCephPoolMeta {
	if in == nil {
		return nil
	}
	out := make([]HostCephPoolMeta, len(in))
	copy(out, in)
	return out
}

func cloneHostCephServiceMetaSlice(in []HostCephServiceMeta) []HostCephServiceMeta {
	if in == nil {
		return nil
	}
	out := make([]HostCephServiceMeta, len(in))
	for i := range in {
		out[i] = in[i]
		out[i].Daemons = cloneStringSlice(in[i].Daemons)
	}
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

func cloneDockerNetworkSubnetMetaSlice(in []DockerNetworkSubnetMeta) []DockerNetworkSubnetMeta {
	if in == nil {
		return nil
	}
	out := make([]DockerNetworkSubnetMeta, len(in))
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

func clonePBSDatastores(in []models.PBSDatastore) []models.PBSDatastore {
	if in == nil {
		return nil
	}
	out := make([]models.PBSDatastore, len(in))
	for i := range in {
		out[i] = in[i]
		if len(in[i].Namespaces) > 0 {
			out[i].Namespaces = append([]models.PBSNamespace(nil), in[i].Namespaces...)
		}
	}
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

func cloneStringIntMap(in map[string]int) map[string]int {
	if in == nil {
		return nil
	}
	out := make(map[string]int, len(in))
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

func cloneIntPtr(in *int) *int {
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

func cloneBoolPtr(in *bool) *bool {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneTemperature(in *models.Temperature) *models.Temperature {
	if in == nil {
		return nil
	}
	out := *in
	out.Cores = append([]models.CoreTemp(nil), in.Cores...)
	out.GPU = append([]models.GPUTemp(nil), in.GPU...)
	out.NVMe = append([]models.NVMeTemp(nil), in.NVMe...)
	out.SMART = append([]models.DiskTemp(nil), in.SMART...)
	return &out
}

func zeroTimeToPtr(in time.Time) *time.Time {
	if in.IsZero() {
		return nil
	}
	out := in
	return &out
}
