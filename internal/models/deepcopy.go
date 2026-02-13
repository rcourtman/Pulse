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
	return &dest
}

func cloneGuestNetworkInterfaces(src []GuestNetworkInterface) []GuestNetworkInterface {
	if len(src) == 0 {
		return nil
	}
	dest := make([]GuestNetworkInterface, len(src))
	for i, nic := range src {
		nicCopy := nic
		nicCopy.Addresses = append([]string(nil), nic.Addresses...)
		dest[i] = nicCopy
	}
	return dest
}

func cloneNode(src Node) Node {
	dest := src
	dest.LoadAverage = append([]float64(nil), src.LoadAverage...)
	dest.Temperature = cloneTemperature(src.Temperature)
	dest.TemperatureMonitoringEnabled = cloneBoolPtr(src.TemperatureMonitoringEnabled)
	return dest
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
	return dest
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
	return dest
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
		dest[i] = nicCopy
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
		SMART:              cloneHostDiskSMART(src.SMART),
	}
}

func cloneHostRAIDArrays(src []HostRAIDArray) []HostRAIDArray {
	if len(src) == 0 {
		return nil
	}
	dest := make([]HostRAIDArray, len(src))
	for i, arr := range src {
		arrCopy := arr
		arrCopy.Devices = append([]HostRAIDDevice(nil), arr.Devices...)
		dest[i] = arrCopy
	}
	return dest
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
	return dest
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
		dest.Services[i] = serviceCopy
	}
	if len(src.Services) == 0 {
		dest.Services = nil
	}
	return &dest
}

func cloneHost(src Host) Host {
	dest := src
	dest.LoadAverage = append([]float64(nil), src.LoadAverage...)
	dest.Disks = append([]Disk(nil), src.Disks...)
	dest.DiskIO = append([]DiskIO(nil), src.DiskIO...)
	dest.NetworkInterfaces = cloneHostNetworkInterfaces(src.NetworkInterfaces)
	dest.Sensors = cloneHostSensorSummary(src.Sensors)
	dest.RAID = cloneHostRAIDArrays(src.RAID)
	dest.Ceph = cloneHostCephCluster(src.Ceph)
	dest.TokenLastUsedAt = cloneTimePtr(src.TokenLastUsedAt)
	dest.Tags = append([]string(nil), src.Tags...)
	return dest
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
	return dest
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
		dest[i] = serviceCopy
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

func cloneDockerSwarmInfo(src *DockerSwarmInfo) *DockerSwarmInfo {
	if src == nil {
		return nil
	}
	dest := *src
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
	dest.Services = cloneDockerServices(src.Services)
	dest.Tasks = cloneDockerTasks(src.Tasks)
	dest.Swarm = cloneDockerSwarmInfo(src.Swarm)
	dest.Temperature = cloneFloat64Ptr(src.Temperature)
	dest.TokenLastUsedAt = cloneTimePtr(src.TokenLastUsedAt)
	dest.Command = cloneDockerHostCommandStatus(src.Command)
	return dest
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
		dest[i] = nodeCopy
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
		dest[i] = podCopy
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
		dest[i] = deploymentCopy
	}
	return dest
}

func cloneKubernetesCluster(src KubernetesCluster) KubernetesCluster {
	dest := src
	dest.Nodes = cloneKubernetesNodes(src.Nodes)
	dest.Pods = cloneKubernetesPods(src.Pods)
	dest.Deployments = cloneKubernetesDeployments(src.Deployments)
	dest.TokenLastUsedAt = cloneTimePtr(src.TokenLastUsedAt)
	return dest
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
	dest.ZFSPool = cloneZFSPool(src.ZFSPool)
	return dest
}

func cloneZFSPool(src *ZFSPool) *ZFSPool {
	if src == nil {
		return nil
	}
	dest := *src
	dest.Devices = append([]ZFSDevice(nil), src.Devices...)
	return &dest
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
	dest.Pools = append([]CephPool(nil), src.Pools...)
	dest.Services = append([]CephServiceStatus(nil), src.Services...)
	return dest
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
		dest[i] = datastoreCopy
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
	return dest
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
	return dest
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
	return dest
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

func clonePMGBackup(src PMGBackup) PMGBackup {
	return src
}

func clonePMGBackups(src []PMGBackup) []PMGBackup {
	return append([]PMGBackup(nil), src...)
}

func clonePVEBackups(src PVEBackups) PVEBackups {
	return PVEBackups{
		BackupTasks:    append([]BackupTask(nil), src.BackupTasks...),
		StorageBackups: append([]StorageBackup(nil), src.StorageBackups...),
		GuestSnapshots: append([]GuestSnapshot(nil), src.GuestSnapshots...),
	}
}

func clonePerformance(src Performance) Performance {
	return Performance{
		APICallDuration:  cloneStringFloat64Map(src.APICallDuration),
		LastPollDuration: src.LastPollDuration,
		PollingStartTime: src.PollingStartTime,
		TotalAPICalls:    src.TotalAPICalls,
		FailedAPICalls:   src.FailedAPICalls,
	}
}

func cloneAlert(src Alert) Alert {
	dest := src
	dest.AckTime = cloneTimePtr(src.AckTime)
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
	return dest
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
