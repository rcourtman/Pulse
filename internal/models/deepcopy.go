package models

import (
	"reflect"
	"time"
)

func cloneBoolPtr(src *bool) *bool {
	if src == nil {
		return nil
	}
	v := *src
	return &v
}

func cloneFloat64Ptr(src *float64) *float64 {
	if src == nil {
		return nil
	}
	v := *src
	return &v
}

func cloneIntPtr(src *int) *int {
	if src == nil {
		return nil
	}
	v := *src
	return &v
}

func cloneInt64Ptr(src *int64) *int64 {
	if src == nil {
		return nil
	}
	v := *src
	return &v
}

func cloneTimePtr(src *time.Time) *time.Time {
	if src == nil {
		return nil
	}
	v := *src
	return &v
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	dest := make(map[string]string, len(src))
	for k, v := range src {
		dest[k] = v
	}
	return dest
}

func clonePVETagStyles(src map[string]PVETagStyle) map[string]PVETagStyle {
	if len(src) == 0 {
		return nil
	}
	dest := make(map[string]PVETagStyle, len(src))
	for instance, style := range src {
		colors := cloneStringMap(style.Colors)
		if colors == nil {
			colors = map[string]string{}
		}
		dest[instance] = PVETagStyle{
			Colors:        colors,
			CaseSensitive: style.CaseSensitive,
		}
	}
	return dest
}

func cloneStringFloat64Map(src map[string]float64) map[string]float64 {
	if len(src) == 0 {
		return nil
	}
	dest := make(map[string]float64, len(src))
	for k, v := range src {
		dest[k] = v
	}
	return dest
}

func cloneStringIntMap(src map[string]int) map[string]int {
	if len(src) == 0 {
		return nil
	}
	dest := make(map[string]int, len(src))
	for k, v := range src {
		dest[k] = v
	}
	return dest
}

func cloneCoreTemps(src []CoreTemp) []CoreTemp {
	return append([]CoreTemp(nil), src...)
}

func cloneGPUTemps(src []GPUTemp) []GPUTemp {
	return append([]GPUTemp(nil), src...)
}

func cloneNVMeTemps(src []NVMeTemp) []NVMeTemp {
	return append([]NVMeTemp(nil), src...)
}

func cloneDiskTemps(src []DiskTemp) []DiskTemp {
	return append([]DiskTemp(nil), src...)
}

func cloneTemperature(src *Temperature) *Temperature {
	if src == nil {
		return nil
	}
	dest := *src
	dest.Cores = cloneCoreTemps(src.Cores)
	dest.GPU = cloneGPUTemps(src.GPU)
	dest.NVMe = cloneNVMeTemps(src.NVMe)
	dest.SMART = cloneDiskTemps(src.SMART)
	normalized := dest.NormalizeCollections()
	return &normalized
}

func cloneGuestNetworkInterfaces(src []GuestNetworkInterface) []GuestNetworkInterface {
	if len(src) == 0 {
		return nil
	}
	dest := make([]GuestNetworkInterface, len(src))
	for i, nic := range src {
		nicCopy := nic
		nicCopy.Addresses = append([]string(nil), nic.Addresses...)
		dest[i] = nicCopy.NormalizeCollections()
	}
	return dest
}

func cloneNode(src Node) Node {
	dest := src
	dest.LoadAverage = append([]float64(nil), src.LoadAverage...)
	dest.Temperature = cloneTemperature(src.Temperature)
	dest.TemperatureMonitoringEnabled = cloneBoolPtr(src.TemperatureMonitoringEnabled)
	return dest.NormalizeCollections()
}

func cloneNodes(src []Node) []Node {
	if len(src) == 0 {
		return nil
	}
	dest := make([]Node, len(src))
	for i, node := range src {
		dest[i] = cloneNode(node)
	}
	return dest
}

func cloneVM(src VM) VM {
	dest := src
	dest.Disks = append([]Disk(nil), src.Disks...)
	dest.IPAddresses = append([]string(nil), src.IPAddresses...)
	dest.Tags = append([]string(nil), src.Tags...)
	dest.NetworkInterfaces = cloneGuestNetworkInterfaces(src.NetworkInterfaces)
	return dest.NormalizeCollections()
}

func cloneVMs(src []VM) []VM {
	if len(src) == 0 {
		return nil
	}
	dest := make([]VM, len(src))
	for i, vm := range src {
		dest[i] = cloneVM(vm)
	}
	return dest
}

func cloneContainer(src Container) Container {
	dest := src
	dest.Disks = append([]Disk(nil), src.Disks...)
	dest.IPAddresses = append([]string(nil), src.IPAddresses...)
	dest.Tags = append([]string(nil), src.Tags...)
	dest.NetworkInterfaces = cloneGuestNetworkInterfaces(src.NetworkInterfaces)
	return dest.NormalizeCollections()
}

func cloneContainers(src []Container) []Container {
	if len(src) == 0 {
		return nil
	}
	dest := make([]Container, len(src))
	for i, container := range src {
		dest[i] = cloneContainer(container)
	}
	return dest
}

func cloneHostNetworkInterfaces(src []HostNetworkInterface) []HostNetworkInterface {
	if len(src) == 0 {
		return nil
	}
	dest := make([]HostNetworkInterface, len(src))
	for i, nic := range src {
		nicCopy := nic
		nicCopy.Addresses = append([]string(nil), nic.Addresses...)
		nicCopy.SpeedMbps = cloneInt64Ptr(nic.SpeedMbps)
		dest[i] = nicCopy.NormalizeCollections()
	}
	return dest
}

func cloneSMARTAttributes(src *SMARTAttributes) *SMARTAttributes {
	if src == nil {
		return nil
	}
	dest := *src
	dest.PowerOnHours = cloneInt64Ptr(src.PowerOnHours)
	dest.PowerCycles = cloneInt64Ptr(src.PowerCycles)
	dest.ReallocatedSectors = cloneInt64Ptr(src.ReallocatedSectors)
	dest.PendingSectors = cloneInt64Ptr(src.PendingSectors)
	dest.OfflineUncorrectable = cloneInt64Ptr(src.OfflineUncorrectable)
	dest.UDMACRCErrors = cloneInt64Ptr(src.UDMACRCErrors)
	dest.PercentageUsed = cloneIntPtr(src.PercentageUsed)
	dest.AvailableSpare = cloneIntPtr(src.AvailableSpare)
	dest.MediaErrors = cloneInt64Ptr(src.MediaErrors)
	dest.UnsafeShutdowns = cloneInt64Ptr(src.UnsafeShutdowns)
	return &dest
}

func cloneHostDiskSMART(src []HostDiskSMART) []HostDiskSMART {
	if len(src) == 0 {
		return nil
	}
	dest := make([]HostDiskSMART, len(src))
	for i, disk := range src {
		diskCopy := disk
		diskCopy.Attributes = cloneSMARTAttributes(disk.Attributes)
		dest[i] = diskCopy
	}
	return dest
}

func cloneHostSensorSummary(src HostSensorSummary) HostSensorSummary {
	return HostSensorSummary{
		TemperatureCelsius: cloneStringFloat64Map(src.TemperatureCelsius),
		FanRPM:             cloneStringFloat64Map(src.FanRPM),
		Additional:         cloneStringFloat64Map(src.Additional),
		ThermalState:       cloneHostThermalState(src.ThermalState),
		SMART:              cloneHostDiskSMART(src.SMART),
	}.NormalizeCollections()
}

func cloneHostThermalState(src *HostThermalState) *HostThermalState {
	if src == nil {
		return nil
	}
	dest := *src
	dest.ThermalWarningLevel = cloneIntPtr(src.ThermalWarningLevel)
	dest.PerformanceWarningLevel = cloneIntPtr(src.PerformanceWarningLevel)
	dest.CPUPowerStatus = cloneIntPtr(src.CPUPowerStatus)
	dest.LimitsPercent = cloneStringIntMap(src.LimitsPercent)
	return &dest
}

func cloneHostRAIDArrays(src []HostRAIDArray) []HostRAIDArray {
	if len(src) == 0 {
		return nil
	}
	dest := make([]HostRAIDArray, len(src))
	for i, arr := range src {
		arrCopy := arr
		arrCopy.Devices = append([]HostRAIDDevice(nil), arr.Devices...)
		dest[i] = arrCopy.NormalizeCollections()
	}
	return dest
}

func cloneHostUnraidStorage(src *HostUnraidStorage) *HostUnraidStorage {
	if src == nil {
		return nil
	}
	dest := *src
	if len(src.Disks) > 0 {
		dest.Disks = make([]HostUnraidDisk, len(src.Disks))
		copy(dest.Disks, src.Disks)
	}
	normalized := dest.NormalizeCollections()
	return &normalized
}

func cloneHostCephHealth(src HostCephHealth) HostCephHealth {
	dest := HostCephHealth{
		Status:  src.Status,
		Checks:  make(map[string]HostCephCheck, len(src.Checks)),
		Summary: append([]HostCephHealthSummary(nil), src.Summary...),
	}
	for key, check := range src.Checks {
		checkCopy := check
		checkCopy.Detail = append([]string(nil), check.Detail...)
		dest.Checks[key] = checkCopy
	}
	if len(src.Checks) == 0 {
		dest.Checks = nil
	}
	return dest.NormalizeCollections()
}

func cloneHostCephCluster(src *HostCephCluster) *HostCephCluster {
	if src == nil {
		return nil
	}
	dest := *src
	dest.Health = cloneHostCephHealth(src.Health)
	dest.MonMap.Monitors = append([]HostCephMonitor(nil), src.MonMap.Monitors...)
	dest.Pools = append([]HostCephPool(nil), src.Pools...)
	dest.Services = make([]HostCephService, len(src.Services))
	for i, service := range src.Services {
		serviceCopy := service
		serviceCopy.Daemons = append([]string(nil), service.Daemons...)
		dest.Services[i] = serviceCopy.NormalizeCollections()
	}
	if len(src.Services) == 0 {
		dest.Services = nil
	}
	normalized := dest.NormalizeCollections()
	return &normalized
}

func cloneHost(src Host) Host {
	dest := src
	dest.LoadAverage = append([]float64(nil), src.LoadAverage...)
	dest.Disks = append([]Disk(nil), src.Disks...)
	dest.DiskIO = append([]DiskIO(nil), src.DiskIO...)
	dest.NetworkInterfaces = cloneHostNetworkInterfaces(src.NetworkInterfaces)
	dest.Sensors = cloneHostSensorSummary(src.Sensors)
	dest.RAID = cloneHostRAIDArrays(src.RAID)
	dest.Unraid = cloneHostUnraidStorage(src.Unraid)
	dest.Ceph = cloneHostCephCluster(src.Ceph)
	dest.TokenLastUsedAt = cloneTimePtr(src.TokenLastUsedAt)
	dest.Tags = append([]string(nil), src.Tags...)
	dest.DiskExclude = append([]string(nil), src.DiskExclude...)
	return dest.NormalizeCollections()
}

func cloneHosts(src []Host) []Host {
	if len(src) == 0 {
		return nil
	}
	dest := make([]Host, len(src))
	for i, host := range src {
		dest[i] = cloneHost(host)
	}
	return dest
}

func cloneDockerContainerBlockIO(src *DockerContainerBlockIO) *DockerContainerBlockIO {
	if src == nil {
		return nil
	}
	dest := *src
	dest.ReadRateBytesPerSecond = cloneFloat64Ptr(src.ReadRateBytesPerSecond)
	dest.WriteRateBytesPerSecond = cloneFloat64Ptr(src.WriteRateBytesPerSecond)
	return &dest
}

func cloneDockerPodmanContainer(src *DockerPodmanContainer) *DockerPodmanContainer {
	if src == nil {
		return nil
	}
	dest := *src
	return &dest
}

func cloneDockerContainerUpdateStatus(src *DockerContainerUpdateStatus) *DockerContainerUpdateStatus {
	if src == nil {
		return nil
	}
	dest := *src
	return &dest
}

func cloneDockerContainer(src DockerContainer) DockerContainer {
	dest := src
	dest.StartedAt = cloneTimePtr(src.StartedAt)
	dest.FinishedAt = cloneTimePtr(src.FinishedAt)
	dest.Ports = append([]DockerContainerPort(nil), src.Ports...)
	dest.Labels = cloneStringMap(src.Labels)
	dest.Networks = append([]DockerContainerNetworkLink(nil), src.Networks...)
	dest.BlockIO = cloneDockerContainerBlockIO(src.BlockIO)
	dest.Mounts = append([]DockerContainerMount(nil), src.Mounts...)
	dest.Podman = cloneDockerPodmanContainer(src.Podman)
	dest.UpdateStatus = cloneDockerContainerUpdateStatus(src.UpdateStatus)
	return dest.NormalizeCollections()
}

func cloneDockerContainers(src []DockerContainer) []DockerContainer {
	if len(src) == 0 {
		return nil
	}
	dest := make([]DockerContainer, len(src))
	for i, container := range src {
		dest[i] = cloneDockerContainer(container)
	}
	return dest
}

func cloneDockerImages(src []DockerImage) []DockerImage {
	if len(src) == 0 {
		return nil
	}
	dest := make([]DockerImage, len(src))
	for i, image := range src {
		imageCopy := image
		imageCopy.RepoTags = append([]string(nil), image.RepoTags...)
		imageCopy.RepoDigests = append([]string(nil), image.RepoDigests...)
		imageCopy.Labels = cloneStringMap(image.Labels)
		dest[i] = imageCopy.NormalizeCollections()
	}
	return dest
}

func cloneDockerVolumes(src []DockerVolume) []DockerVolume {
	if len(src) == 0 {
		return nil
	}
	dest := make([]DockerVolume, len(src))
	for i, volume := range src {
		volumeCopy := volume
		volumeCopy.Labels = cloneStringMap(volume.Labels)
		volumeCopy.Options = cloneStringMap(volume.Options)
		dest[i] = volumeCopy.NormalizeCollections()
	}
	return dest
}

func cloneDockerNetworks(src []DockerNetwork) []DockerNetwork {
	if len(src) == 0 {
		return nil
	}
	dest := make([]DockerNetwork, len(src))
	for i, network := range src {
		networkCopy := network
		networkCopy.Subnets = append([]DockerNetworkSubnet(nil), network.Subnets...)
		networkCopy.Labels = cloneStringMap(network.Labels)
		networkCopy.Options = cloneStringMap(network.Options)
		dest[i] = networkCopy.NormalizeCollections()
	}
	return dest
}

func cloneDockerStorageUsage(src *DockerStorageUsage) *DockerStorageUsage {
	if src == nil {
		return nil
	}
	dest := *src
	return &dest
}

func cloneDockerServiceUpdate(src *DockerServiceUpdate) *DockerServiceUpdate {
	if src == nil {
		return nil
	}
	dest := *src
	dest.CompletedAt = cloneTimePtr(src.CompletedAt)
	return &dest
}

func cloneDockerServices(src []DockerService) []DockerService {
	if len(src) == 0 {
		return nil
	}
	dest := make([]DockerService, len(src))
	for i, service := range src {
		serviceCopy := service
		serviceCopy.UpdateStatus = cloneDockerServiceUpdate(service.UpdateStatus)
		serviceCopy.Labels = cloneStringMap(service.Labels)
		serviceCopy.EndpointPorts = append([]DockerServicePort(nil), service.EndpointPorts...)
		serviceCopy.CreatedAt = cloneTimePtr(service.CreatedAt)
		serviceCopy.UpdatedAt = cloneTimePtr(service.UpdatedAt)
		dest[i] = serviceCopy.NormalizeCollections()
	}
	return dest
}

func cloneDockerTasks(src []DockerTask) []DockerTask {
	if len(src) == 0 {
		return nil
	}
	dest := make([]DockerTask, len(src))
	for i, task := range src {
		taskCopy := task
		taskCopy.UpdatedAt = cloneTimePtr(task.UpdatedAt)
		taskCopy.StartedAt = cloneTimePtr(task.StartedAt)
		taskCopy.CompletedAt = cloneTimePtr(task.CompletedAt)
		dest[i] = taskCopy
	}
	return dest
}

func cloneDockerNodes(src []DockerNode) []DockerNode {
	if len(src) == 0 {
		return nil
	}
	dest := make([]DockerNode, len(src))
	for i, node := range src {
		nodeCopy := node
		nodeCopy.Labels = cloneStringMap(node.Labels)
		nodeCopy.EngineLabels = cloneStringMap(node.EngineLabels)
		nodeCopy.UpdatedAt = cloneTimePtr(node.UpdatedAt)
		dest[i] = nodeCopy.NormalizeCollections()
	}
	return dest
}

func cloneDockerSecrets(src []DockerSecret) []DockerSecret {
	if len(src) == 0 {
		return nil
	}
	dest := make([]DockerSecret, len(src))
	for i, secret := range src {
		secretCopy := secret
		secretCopy.Labels = cloneStringMap(secret.Labels)
		secretCopy.UpdatedAt = cloneTimePtr(secret.UpdatedAt)
		dest[i] = secretCopy.NormalizeCollections()
	}
	return dest
}

func cloneDockerConfigs(src []DockerConfig) []DockerConfig {
	if len(src) == 0 {
		return nil
	}
	dest := make([]DockerConfig, len(src))
	for i, config := range src {
		configCopy := config
		configCopy.Labels = cloneStringMap(config.Labels)
		configCopy.UpdatedAt = cloneTimePtr(config.UpdatedAt)
		dest[i] = configCopy.NormalizeCollections()
	}
	return dest
}

func cloneDockerSwarmInfo(src *DockerSwarmInfo) *DockerSwarmInfo {
	if src == nil {
		return nil
	}
	dest := *src
	return &dest
}

func cloneDockerHostSecurity(src *DockerHostSecurity) *DockerHostSecurity {
	if src == nil {
		return nil
	}
	dest := *src
	dest.AuthorizationPlugins = append([]string(nil), src.AuthorizationPlugins...)
	return &dest
}

func cloneDockerHostCommandStatus(src *DockerHostCommandStatus) *DockerHostCommandStatus {
	if src == nil {
		return nil
	}
	dest := *src
	dest.DispatchedAt = cloneTimePtr(src.DispatchedAt)
	dest.AcknowledgedAt = cloneTimePtr(src.AcknowledgedAt)
	dest.CompletedAt = cloneTimePtr(src.CompletedAt)
	dest.FailedAt = cloneTimePtr(src.FailedAt)
	dest.ExpiresAt = cloneTimePtr(src.ExpiresAt)
	return &dest
}

func cloneDockerHost(src DockerHost) DockerHost {
	dest := src
	dest.LoadAverage = append([]float64(nil), src.LoadAverage...)
	dest.Disks = append([]Disk(nil), src.Disks...)
	dest.NetworkInterfaces = cloneHostNetworkInterfaces(src.NetworkInterfaces)
	dest.Containers = cloneDockerContainers(src.Containers)
	dest.Images = cloneDockerImages(src.Images)
	dest.Volumes = cloneDockerVolumes(src.Volumes)
	dest.Networks = cloneDockerNetworks(src.Networks)
	dest.Services = cloneDockerServices(src.Services)
	dest.Tasks = cloneDockerTasks(src.Tasks)
	dest.Nodes = cloneDockerNodes(src.Nodes)
	dest.Secrets = cloneDockerSecrets(src.Secrets)
	dest.Configs = cloneDockerConfigs(src.Configs)
	dest.StorageUsage = cloneDockerStorageUsage(src.StorageUsage)
	dest.Swarm = cloneDockerSwarmInfo(src.Swarm)
	dest.Security = cloneDockerHostSecurity(src.Security)
	dest.Temperature = cloneFloat64Ptr(src.Temperature)
	dest.TokenLastUsedAt = cloneTimePtr(src.TokenLastUsedAt)
	dest.Command = cloneDockerHostCommandStatus(src.Command)
	return dest.NormalizeCollections()
}

func cloneDockerHosts(src []DockerHost) []DockerHost {
	if len(src) == 0 {
		return nil
	}
	dest := make([]DockerHost, len(src))
	for i, host := range src {
		dest[i] = cloneDockerHost(host)
	}
	return dest
}

func cloneKubernetesNodes(src []KubernetesNode) []KubernetesNode {
	if len(src) == 0 {
		return nil
	}
	dest := make([]KubernetesNode, len(src))
	for i, node := range src {
		nodeCopy := node
		nodeCopy.Roles = append([]string(nil), node.Roles...)
		dest[i] = nodeCopy.NormalizeCollections()
	}
	return dest
}

func cloneKubernetesNamespaces(src []KubernetesNamespace) []KubernetesNamespace {
	if len(src) == 0 {
		return nil
	}
	dest := make([]KubernetesNamespace, len(src))
	for i, namespace := range src {
		namespaceCopy := namespace
		namespaceCopy.Labels = cloneStringMap(namespace.Labels)
		dest[i] = namespaceCopy.NormalizeCollections()
	}
	return dest
}

func cloneKubernetesPodContainers(src []KubernetesPodContainer) []KubernetesPodContainer {
	return append([]KubernetesPodContainer(nil), src...)
}

func cloneKubernetesPods(src []KubernetesPod) []KubernetesPod {
	if len(src) == 0 {
		return nil
	}
	dest := make([]KubernetesPod, len(src))
	for i, pod := range src {
		podCopy := pod
		podCopy.StartTime = cloneTimePtr(pod.StartTime)
		podCopy.Labels = cloneStringMap(pod.Labels)
		podCopy.Containers = cloneKubernetesPodContainers(pod.Containers)
		dest[i] = podCopy.NormalizeCollections()
	}
	return dest
}

func cloneKubernetesDeployments(src []KubernetesDeployment) []KubernetesDeployment {
	if len(src) == 0 {
		return nil
	}
	dest := make([]KubernetesDeployment, len(src))
	for i, deployment := range src {
		deploymentCopy := deployment
		deploymentCopy.Labels = cloneStringMap(deployment.Labels)
		dest[i] = deploymentCopy.NormalizeCollections()
	}
	return dest
}

func cloneKubernetesReplicaSets(src []KubernetesReplicaSet) []KubernetesReplicaSet {
	if len(src) == 0 {
		return nil
	}
	dest := make([]KubernetesReplicaSet, len(src))
	for i, replicaSet := range src {
		copy := replicaSet
		copy.Labels = cloneStringMap(replicaSet.Labels)
		dest[i] = copy.NormalizeCollections()
	}
	return dest
}

func cloneKubernetesStatefulSets(src []KubernetesStatefulSet) []KubernetesStatefulSet {
	if len(src) == 0 {
		return nil
	}
	dest := make([]KubernetesStatefulSet, len(src))
	for i, statefulSet := range src {
		copy := statefulSet
		copy.Labels = cloneStringMap(statefulSet.Labels)
		dest[i] = copy.NormalizeCollections()
	}
	return dest
}

func cloneKubernetesDaemonSets(src []KubernetesDaemonSet) []KubernetesDaemonSet {
	if len(src) == 0 {
		return nil
	}
	dest := make([]KubernetesDaemonSet, len(src))
	for i, daemonSet := range src {
		copy := daemonSet
		copy.Labels = cloneStringMap(daemonSet.Labels)
		dest[i] = copy.NormalizeCollections()
	}
	return dest
}

func cloneKubernetesServices(src []KubernetesService) []KubernetesService {
	if len(src) == 0 {
		return nil
	}
	dest := make([]KubernetesService, len(src))
	for i, service := range src {
		copy := service
		copy.ExternalIPs = append([]string(nil), service.ExternalIPs...)
		copy.Ports = append([]KubernetesServicePort(nil), service.Ports...)
		copy.Selector = cloneStringMap(service.Selector)
		copy.Labels = cloneStringMap(service.Labels)
		dest[i] = copy.NormalizeCollections()
	}
	return dest
}

func cloneKubernetesJobs(src []KubernetesJob) []KubernetesJob {
	if len(src) == 0 {
		return nil
	}
	dest := make([]KubernetesJob, len(src))
	for i, job := range src {
		copy := job
		copy.StartTime = cloneTimePtr(job.StartTime)
		copy.CompletionTime = cloneTimePtr(job.CompletionTime)
		copy.Labels = cloneStringMap(job.Labels)
		dest[i] = copy.NormalizeCollections()
	}
	return dest
}

func cloneKubernetesCronJobs(src []KubernetesCronJob) []KubernetesCronJob {
	if len(src) == 0 {
		return nil
	}
	dest := make([]KubernetesCronJob, len(src))
	for i, cronJob := range src {
		copy := cronJob
		copy.LastScheduleTime = cloneTimePtr(cronJob.LastScheduleTime)
		copy.LastSuccessfulTime = cloneTimePtr(cronJob.LastSuccessfulTime)
		copy.Labels = cloneStringMap(cronJob.Labels)
		dest[i] = copy.NormalizeCollections()
	}
	return dest
}

func cloneKubernetesIngresses(src []KubernetesIngress) []KubernetesIngress {
	if len(src) == 0 {
		return nil
	}
	dest := make([]KubernetesIngress, len(src))
	for i, ingress := range src {
		copy := ingress
		copy.Hosts = append([]string(nil), ingress.Hosts...)
		copy.Addresses = append([]string(nil), ingress.Addresses...)
		copy.Labels = cloneStringMap(ingress.Labels)
		dest[i] = copy.NormalizeCollections()
	}
	return dest
}

func cloneKubernetesEndpointSlices(src []KubernetesEndpointSlice) []KubernetesEndpointSlice {
	if len(src) == 0 {
		return nil
	}
	dest := make([]KubernetesEndpointSlice, len(src))
	for i, endpointSlice := range src {
		copy := endpointSlice
		copy.Ports = append([]KubernetesEndpointPort(nil), endpointSlice.Ports...)
		copy.Labels = cloneStringMap(endpointSlice.Labels)
		dest[i] = copy.NormalizeCollections()
	}
	return dest
}

func cloneKubernetesNetworkPolicies(src []KubernetesNetworkPolicy) []KubernetesNetworkPolicy {
	if len(src) == 0 {
		return nil
	}
	dest := make([]KubernetesNetworkPolicy, len(src))
	for i, policy := range src {
		copy := policy
		copy.PolicyTypes = append([]string(nil), policy.PolicyTypes...)
		copy.Labels = cloneStringMap(policy.Labels)
		dest[i] = copy.NormalizeCollections()
	}
	return dest
}

func cloneKubernetesPersistentVolumes(src []KubernetesPersistentVolume) []KubernetesPersistentVolume {
	if len(src) == 0 {
		return nil
	}
	dest := make([]KubernetesPersistentVolume, len(src))
	for i, volume := range src {
		copy := volume
		copy.AccessModes = append([]string(nil), volume.AccessModes...)
		copy.Labels = cloneStringMap(volume.Labels)
		dest[i] = copy.NormalizeCollections()
	}
	return dest
}

func cloneKubernetesPersistentVolumeClaims(src []KubernetesPersistentVolumeClaim) []KubernetesPersistentVolumeClaim {
	if len(src) == 0 {
		return nil
	}
	dest := make([]KubernetesPersistentVolumeClaim, len(src))
	for i, claim := range src {
		copy := claim
		copy.AccessModes = append([]string(nil), claim.AccessModes...)
		copy.Labels = cloneStringMap(claim.Labels)
		dest[i] = copy.NormalizeCollections()
	}
	return dest
}

func cloneKubernetesStorageClasses(src []KubernetesStorageClass) []KubernetesStorageClass {
	if len(src) == 0 {
		return nil
	}
	dest := make([]KubernetesStorageClass, len(src))
	for i, class := range src {
		copy := class
		copy.AllowVolumeExpansion = cloneBoolPtr(class.AllowVolumeExpansion)
		copy.ParameterKeys = append([]string(nil), class.ParameterKeys...)
		copy.Labels = cloneStringMap(class.Labels)
		dest[i] = copy.NormalizeCollections()
	}
	return dest
}

func cloneKubernetesConfigMaps(src []KubernetesConfigMap) []KubernetesConfigMap {
	if len(src) == 0 {
		return nil
	}
	dest := make([]KubernetesConfigMap, len(src))
	for i, configMap := range src {
		copy := configMap
		copy.DataKeys = append([]string(nil), configMap.DataKeys...)
		copy.BinaryDataKeys = append([]string(nil), configMap.BinaryDataKeys...)
		copy.Labels = cloneStringMap(configMap.Labels)
		dest[i] = copy.NormalizeCollections()
	}
	return dest
}

func cloneKubernetesSecrets(src []KubernetesSecret) []KubernetesSecret {
	if len(src) == 0 {
		return nil
	}
	dest := make([]KubernetesSecret, len(src))
	for i, secret := range src {
		copy := secret
		copy.DataKeys = append([]string(nil), secret.DataKeys...)
		copy.Labels = cloneStringMap(secret.Labels)
		dest[i] = copy.NormalizeCollections()
	}
	return dest
}

func cloneKubernetesServiceAccounts(src []KubernetesServiceAccount) []KubernetesServiceAccount {
	if len(src) == 0 {
		return nil
	}
	dest := make([]KubernetesServiceAccount, len(src))
	for i, account := range src {
		copy := account
		copy.AutomountServiceAccountToken = cloneBoolPtr(account.AutomountServiceAccountToken)
		copy.ImagePullSecrets = append([]string(nil), account.ImagePullSecrets...)
		copy.Labels = cloneStringMap(account.Labels)
		dest[i] = copy.NormalizeCollections()
	}
	return dest
}

func cloneKubernetesResourceQuotas(src []KubernetesResourceQuota) []KubernetesResourceQuota {
	if len(src) == 0 {
		return nil
	}
	dest := make([]KubernetesResourceQuota, len(src))
	for i, quota := range src {
		copy := quota
		copy.Hard = cloneStringMap(quota.Hard)
		copy.Used = cloneStringMap(quota.Used)
		copy.Labels = cloneStringMap(quota.Labels)
		dest[i] = copy.NormalizeCollections()
	}
	return dest
}

func cloneKubernetesLimitRanges(src []KubernetesLimitRange) []KubernetesLimitRange {
	if len(src) == 0 {
		return nil
	}
	dest := make([]KubernetesLimitRange, len(src))
	for i, limitRange := range src {
		copy := limitRange
		copy.LimitTypes = append([]string(nil), limitRange.LimitTypes...)
		copy.Labels = cloneStringMap(limitRange.Labels)
		dest[i] = copy.NormalizeCollections()
	}
	return dest
}

func cloneKubernetesPodDisruptionBudgets(src []KubernetesPodDisruptionBudget) []KubernetesPodDisruptionBudget {
	if len(src) == 0 {
		return nil
	}
	dest := make([]KubernetesPodDisruptionBudget, len(src))
	for i, budget := range src {
		copy := budget
		copy.Labels = cloneStringMap(budget.Labels)
		dest[i] = copy.NormalizeCollections()
	}
	return dest
}

func cloneKubernetesHorizontalPodAutoscalers(src []KubernetesHorizontalPodAutoscaler) []KubernetesHorizontalPodAutoscaler {
	if len(src) == 0 {
		return nil
	}
	dest := make([]KubernetesHorizontalPodAutoscaler, len(src))
	for i, autoscaler := range src {
		copy := autoscaler
		copy.MetricTypes = append([]string(nil), autoscaler.MetricTypes...)
		copy.Labels = cloneStringMap(autoscaler.Labels)
		dest[i] = copy.NormalizeCollections()
	}
	return dest
}

func cloneKubernetesEvents(src []KubernetesEvent) []KubernetesEvent {
	if len(src) == 0 {
		return nil
	}
	dest := make([]KubernetesEvent, len(src))
	for i, event := range src {
		copy := event
		copy.FirstSeen = cloneTimePtr(event.FirstSeen)
		copy.LastSeen = cloneTimePtr(event.LastSeen)
		copy.EventTime = cloneTimePtr(event.EventTime)
		dest[i] = copy
	}
	return dest
}

func cloneKubernetesCluster(src KubernetesCluster) KubernetesCluster {
	dest := src
	dest.Nodes = cloneKubernetesNodes(src.Nodes)
	dest.Namespaces = cloneKubernetesNamespaces(src.Namespaces)
	dest.Pods = cloneKubernetesPods(src.Pods)
	dest.Deployments = cloneKubernetesDeployments(src.Deployments)
	dest.ReplicaSets = cloneKubernetesReplicaSets(src.ReplicaSets)
	dest.StatefulSets = cloneKubernetesStatefulSets(src.StatefulSets)
	dest.DaemonSets = cloneKubernetesDaemonSets(src.DaemonSets)
	dest.Services = cloneKubernetesServices(src.Services)
	dest.Jobs = cloneKubernetesJobs(src.Jobs)
	dest.CronJobs = cloneKubernetesCronJobs(src.CronJobs)
	dest.Ingresses = cloneKubernetesIngresses(src.Ingresses)
	dest.EndpointSlices = cloneKubernetesEndpointSlices(src.EndpointSlices)
	dest.NetworkPolicies = cloneKubernetesNetworkPolicies(src.NetworkPolicies)
	dest.PersistentVolumes = cloneKubernetesPersistentVolumes(src.PersistentVolumes)
	dest.PersistentVolumeClaims = cloneKubernetesPersistentVolumeClaims(src.PersistentVolumeClaims)
	dest.StorageClasses = cloneKubernetesStorageClasses(src.StorageClasses)
	dest.ConfigMaps = cloneKubernetesConfigMaps(src.ConfigMaps)
	dest.Secrets = cloneKubernetesSecrets(src.Secrets)
	dest.ServiceAccounts = cloneKubernetesServiceAccounts(src.ServiceAccounts)
	dest.ResourceQuotas = cloneKubernetesResourceQuotas(src.ResourceQuotas)
	dest.LimitRanges = cloneKubernetesLimitRanges(src.LimitRanges)
	dest.PodDisruptionBudgets = cloneKubernetesPodDisruptionBudgets(src.PodDisruptionBudgets)
	dest.HorizontalPodAutoscalers = cloneKubernetesHorizontalPodAutoscalers(src.HorizontalPodAutoscalers)
	dest.Events = cloneKubernetesEvents(src.Events)
	// RBAC slices: NormalizeCollections() mutates these via index
	// assignment (c.Roles[i] = c.Roles[i].NormalizeCollections()), so they
	// must not be aliased to the source slice or concurrent clones will
	// race on the shared underlying array.
	dest.Roles = append([]KubernetesRole(nil), src.Roles...)
	dest.ClusterRoles = append([]KubernetesClusterRole(nil), src.ClusterRoles...)
	dest.RoleBindings = append([]KubernetesRoleBinding(nil), src.RoleBindings...)
	dest.ClusterRoleBindings = append([]KubernetesClusterRoleBinding(nil), src.ClusterRoleBindings...)
	dest.TokenLastUsedAt = cloneTimePtr(src.TokenLastUsedAt)
	return dest.NormalizeCollections()
}

func cloneKubernetesClusters(src []KubernetesCluster) []KubernetesCluster {
	if len(src) == 0 {
		return nil
	}
	dest := make([]KubernetesCluster, len(src))
	for i, cluster := range src {
		dest[i] = cloneKubernetesCluster(cluster)
	}
	return dest
}

func cloneStorage(src Storage) Storage {
	dest := src
	dest.Nodes = append([]string(nil), src.Nodes...)
	dest.NodeIDs = append([]string(nil), src.NodeIDs...)
	dest.AliasIDs = append([]string(nil), src.AliasIDs...)
	dest.ZFSPool = cloneZFSPool(src.ZFSPool)
	return dest.NormalizeCollections()
}

func cloneZFSPool(src *ZFSPool) *ZFSPool {
	if src == nil {
		return nil
	}
	dest := *src
	dest.Devices = append([]ZFSDevice(nil), src.Devices...)
	normalized := dest.NormalizeCollections()
	return &normalized
}

func cloneStorages(src []Storage) []Storage {
	if len(src) == 0 {
		return nil
	}
	dest := make([]Storage, len(src))
	for i, storage := range src {
		dest[i] = cloneStorage(storage)
	}
	return dest
}

func cloneCephCluster(src CephCluster) CephCluster {
	dest := src
	dest.InstanceAliases = append([]string(nil), src.InstanceAliases...)
	dest.Pools = append([]CephPool(nil), src.Pools...)
	dest.Services = append([]CephServiceStatus(nil), src.Services...)
	return dest.NormalizeCollections()
}

func cloneCephClusters(src []CephCluster) []CephCluster {
	if len(src) == 0 {
		return nil
	}
	dest := make([]CephCluster, len(src))
	for i, cluster := range src {
		dest[i] = cloneCephCluster(cluster)
	}
	return dest
}

func clonePhysicalDisk(src PhysicalDisk) PhysicalDisk {
	dest := src
	dest.SmartAttributes = cloneSMARTAttributes(src.SmartAttributes)
	return dest
}

func clonePhysicalDisks(src []PhysicalDisk) []PhysicalDisk {
	if len(src) == 0 {
		return nil
	}
	dest := make([]PhysicalDisk, len(src))
	for i, disk := range src {
		dest[i] = clonePhysicalDisk(disk)
	}
	return dest
}

func clonePBSDatastores(src []PBSDatastore) []PBSDatastore {
	if len(src) == 0 {
		return nil
	}
	dest := make([]PBSDatastore, len(src))
	for i, datastore := range src {
		datastoreCopy := datastore
		datastoreCopy.Namespaces = append([]PBSNamespace(nil), datastore.Namespaces...)
		dest[i] = datastoreCopy.NormalizeCollections()
	}
	return dest
}

func clonePBSInstance(src PBSInstance) PBSInstance {
	dest := src
	dest.Datastores = clonePBSDatastores(src.Datastores)
	dest.BackupJobs = append([]PBSBackupJob(nil), src.BackupJobs...)
	dest.SyncJobs = append([]PBSSyncJob(nil), src.SyncJobs...)
	dest.VerifyJobs = append([]PBSVerifyJob(nil), src.VerifyJobs...)
	dest.PruneJobs = append([]PBSPruneJob(nil), src.PruneJobs...)
	dest.GarbageJobs = append([]PBSGarbageJob(nil), src.GarbageJobs...)
	dest.JobHealthEvidence = append([]PBSJobHealthEvidence(nil), src.JobHealthEvidence...)
	return dest.NormalizeCollections()
}

func clonePBSInstances(src []PBSInstance) []PBSInstance {
	if len(src) == 0 {
		return nil
	}
	dest := make([]PBSInstance, len(src))
	for i, instance := range src {
		dest[i] = clonePBSInstance(instance)
	}
	return dest
}

func clonePMGNodeStatus(src []PMGNodeStatus) []PMGNodeStatus {
	if len(src) == 0 {
		return nil
	}
	dest := make([]PMGNodeStatus, len(src))
	for i, node := range src {
		nodeCopy := node
		if node.QueueStatus != nil {
			queueCopy := *node.QueueStatus
			nodeCopy.QueueStatus = &queueCopy
		}
		dest[i] = nodeCopy
	}
	return dest
}

func clonePMGMailStats(src *PMGMailStats) *PMGMailStats {
	if src == nil {
		return nil
	}
	dest := *src
	return &dest
}

func clonePMGQuarantine(src *PMGQuarantineTotals) *PMGQuarantineTotals {
	if src == nil {
		return nil
	}
	dest := *src
	return &dest
}

func clonePMGInstance(src PMGInstance) PMGInstance {
	dest := src
	dest.Nodes = clonePMGNodeStatus(src.Nodes)
	dest.MailStats = clonePMGMailStats(src.MailStats)
	dest.MailCount = append([]PMGMailCountPoint(nil), src.MailCount...)
	dest.SpamDistribution = append([]PMGSpamBucket(nil), src.SpamDistribution...)
	dest.Quarantine = clonePMGQuarantine(src.Quarantine)
	dest.RelayDomains = append([]PMGRelayDomain(nil), src.RelayDomains...)
	dest.DomainStats = append([]PMGDomainStat(nil), src.DomainStats...)
	return dest.NormalizeCollections()
}

func clonePMGInstances(src []PMGInstance) []PMGInstance {
	if len(src) == 0 {
		return nil
	}
	dest := make([]PMGInstance, len(src))
	for i, instance := range src {
		dest[i] = clonePMGInstance(instance)
	}
	return dest
}

func clonePBSBackup(src PBSBackup) PBSBackup {
	dest := src
	dest.Files = append([]string(nil), src.Files...)
	// Deep-copy VerificationRaw to avoid shared map references
	if m, ok := src.VerificationRaw.(map[string]interface{}); ok {
		cp := make(map[string]interface{}, len(m))
		for k, v := range m {
			cp[k] = v
		}
		dest.VerificationRaw = cp
	}
	// string and nil cases are immutable - shallow copy is fine
	return dest.NormalizeCollections()
}

func clonePBSBackups(src []PBSBackup) []PBSBackup {
	if len(src) == 0 {
		return nil
	}
	dest := make([]PBSBackup, len(src))
	for i, backup := range src {
		dest[i] = clonePBSBackup(backup)
	}
	return dest
}

func clonePMGBackups(src []PMGBackup) []PMGBackup {
	return append([]PMGBackup(nil), src...)
}

func clonePVEBackups(src PVEBackups) PVEBackups {
	return PVEBackups{
		BackupTasks:    append([]BackupTask(nil), src.BackupTasks...),
		StorageBackups: append([]StorageBackup(nil), src.StorageBackups...),
		GuestSnapshots: append([]GuestSnapshot(nil), src.GuestSnapshots...),
	}.NormalizeCollections()
}

func clonePerformance(src Performance) Performance {
	return Performance{
		APICallDuration:  cloneStringFloat64Map(src.APICallDuration),
		LastPollDuration: src.LastPollDuration,
		PollingStartTime: src.PollingStartTime,
		TotalAPICalls:    src.TotalAPICalls,
		FailedAPICalls:   src.FailedAPICalls,
	}.NormalizeCollections()
}

func cloneAlert(src Alert) Alert {
	dest := src
	dest.AckTime = cloneTimePtr(src.AckTime)
	if src.Metadata != nil {
		dest.Metadata = make(map[string]interface{}, len(src.Metadata))
		for k, v := range src.Metadata {
			dest.Metadata[k] = v
		}
	}
	return dest
}

func cloneAlerts(src []Alert) []Alert {
	if len(src) == 0 {
		return nil
	}
	dest := make([]Alert, len(src))
	for i, alert := range src {
		dest[i] = cloneAlert(alert)
	}
	return dest
}

func cloneResolvedAlerts(src []ResolvedAlert) []ResolvedAlert {
	if len(src) == 0 {
		return nil
	}
	dest := make([]ResolvedAlert, len(src))
	for i, alert := range src {
		alertCopy := alert
		alertCopy.Alert = cloneAlert(alert.Alert)
		dest[i] = alertCopy
	}
	return dest
}

func cloneReplicationJob(src ReplicationJob) ReplicationJob {
	dest := src
	dest.LastSyncTime = cloneTimePtr(src.LastSyncTime)
	dest.NextSyncTime = cloneTimePtr(src.NextSyncTime)
	dest.RateLimitMbps = cloneFloat64Ptr(src.RateLimitMbps)
	return dest
}

func cloneReplicationJobs(src []ReplicationJob) []ReplicationJob {
	if len(src) == 0 {
		return nil
	}
	dest := make([]ReplicationJob, len(src))
	for i, job := range src {
		dest[i] = cloneReplicationJob(job)
	}
	return dest
}

func cloneMetricValues(src map[string]interface{}) map[string]interface{} {
	if len(src) == 0 {
		return nil
	}
	dest := make(map[string]interface{}, len(src))
	for key, value := range src {
		dest[key] = cloneDynamicValue(value)
	}
	return dest
}

func cloneDynamicValue(value interface{}) interface{} {
	if value == nil {
		return nil
	}

	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Map:
		if v.IsNil() {
			return value
		}
		cloned := reflect.MakeMapWithSize(v.Type(), v.Len())
		iter := v.MapRange()
		for iter.Next() {
			clonedValue := cloneDynamicValue(iter.Value().Interface())
			cloned.SetMapIndex(iter.Key(), valueForType(v.Type().Elem(), clonedValue))
		}
		return cloned.Interface()
	case reflect.Slice:
		if v.IsNil() {
			return value
		}
		cloned := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
		for i := 0; i < v.Len(); i++ {
			clonedValue := cloneDynamicValue(v.Index(i).Interface())
			cloned.Index(i).Set(valueForType(v.Type().Elem(), clonedValue))
		}
		return cloned.Interface()
	case reflect.Pointer:
		if v.IsNil() {
			return value
		}
		clonedElem := cloneDynamicValue(v.Elem().Interface())
		ptr := reflect.New(v.Type().Elem())
		ptr.Elem().Set(valueForType(v.Type().Elem(), clonedElem))
		return ptr.Interface()
	default:
		return value
	}
}

func valueForType(targetType reflect.Type, value interface{}) reflect.Value {
	if value == nil {
		return reflect.Zero(targetType)
	}
	result := reflect.ValueOf(value)
	if result.Type().AssignableTo(targetType) {
		return result
	}
	if result.Type().ConvertibleTo(targetType) {
		return result.Convert(targetType)
	}
	return reflect.Zero(targetType)
}

func cloneMetric(src Metric) Metric {
	dest := src
	dest.Values = cloneMetricValues(src.Values)
	return dest.NormalizeCollections()
}

func cloneMetrics(src []Metric) []Metric {
	if len(src) == 0 {
		return nil
	}
	dest := make([]Metric, len(src))
	for i, metric := range src {
		dest[i] = cloneMetric(metric)
	}
	return dest
}
