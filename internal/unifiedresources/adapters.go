package unifiedresources

import (
	"net/url"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func resourceFromProxmoxNode(node models.Node) (Resource, ResourceIdentity) {
	name := node.Name
	if node.DisplayName != "" {
		name = node.DisplayName
	}

	identity := ResourceIdentity{
		Hostnames:   uniqueStrings([]string{node.Name, extractHostname(node.Host)}),
		ClusterName: node.ClusterName,
	}

	if node.ClusterName != "" {
		identity.Hostnames = uniqueStrings(append(identity.Hostnames, node.ClusterName+":"+node.Name))
	}

	proxmox := &ProxmoxData{
		NodeName:          node.Name,
		ClusterName:       node.ClusterName,
		Temperature:       maxNodeTemp(node.Temperature),
		PVEVersion:        node.PVEVersion,
		KernelVersion:     node.KernelVersion,
		Uptime:            node.Uptime,
		CPUInfo:           &CPUInfo{Model: node.CPUInfo.Model, Cores: node.CPUInfo.Cores, Sockets: node.CPUInfo.Sockets},
		LoadAverage:       append([]float64(nil), node.LoadAverage...),
		PendingUpdates:    node.PendingUpdates,
		LinkedHostAgentID: node.LinkedHostAgentID,
	}

	metrics := metricsFromProxmoxNode(node)

	resource := Resource{
		Type:      ResourceTypeHost,
		Name:      name,
		Status:    statusFromNode(node.Status),
		LastSeen:  node.LastSeen,
		UpdatedAt: time.Now().UTC(),
		Metrics:   metrics,
		Proxmox:   proxmox,
		Tags:      nil,
	}

	return resource, identity
}

func resourceFromHost(host models.Host) (Resource, ResourceIdentity) {
	name := host.Hostname
	if host.DisplayName != "" {
		name = host.DisplayName
	}

	ips, macs := collectInterfaceIDs(host.NetworkInterfaces)
	if host.ReportIP != "" {
		ips = append(ips, host.ReportIP)
	}

	identity := ResourceIdentity{
		MachineID:    strings.TrimSpace(host.MachineID),
		Hostnames:    uniqueStrings([]string{host.Hostname}),
		IPAddresses:  uniqueStrings(ips),
		MACAddresses: uniqueStrings(macs),
	}

	agent := &AgentData{
		AgentID:           host.ID,
		AgentVersion:      host.AgentVersion,
		Hostname:          host.Hostname,
		Platform:          host.Platform,
		OSName:            host.OSName,
		OSVersion:         host.OSVersion,
		KernelVersion:     host.KernelVersion,
		Architecture:      host.Architecture,
		UptimeSeconds:     host.UptimeSeconds,
		Temperature:       maxCPUTemp(host.Sensors),
		NetworkInterfaces: convertInterfaces(host.NetworkInterfaces),
		Disks:             convertDisks(host.Disks),
		LinkedNodeID:      host.LinkedNodeID,
		LinkedVMID:        host.LinkedVMID,
		LinkedContainerID: host.LinkedContainerID,
	}

	metrics := metricsFromHost(host)

	resource := Resource{
		Type:      ResourceTypeHost,
		Name:      name,
		Status:    statusFromHost(host.Status),
		LastSeen:  host.LastSeen,
		UpdatedAt: time.Now().UTC(),
		Metrics:   metrics,
		Agent:     agent,
		Tags:      host.Tags,
	}

	return resource, identity
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

	docker := &DockerData{
		Hostname:          host.Hostname,
		Temperature:       host.Temperature,
		Runtime:           host.Runtime,
		RuntimeVersion:    host.RuntimeVersion,
		DockerVersion:     host.DockerVersion,
		OS:                host.OS,
		KernelVersion:     host.KernelVersion,
		Architecture:      host.Architecture,
		AgentVersion:      host.AgentVersion,
		Swarm:             convertSwarm(host.Swarm),
		NetworkInterfaces: convertInterfaces(host.NetworkInterfaces),
		Disks:             convertDisks(host.Disks),
	}

	metrics := metricsFromDockerHost(host)

	resource := Resource{
		Type:      ResourceTypeHost,
		Name:      name,
		Status:    statusFromHost(host.Status),
		LastSeen:  host.LastSeen,
		UpdatedAt: time.Now().UTC(),
		Metrics:   metrics,
		Docker:    docker,
		Tags:      nil,
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

	identity := ResourceIdentity{
		Hostnames: uniqueStrings([]string{
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
		NodeName:   vm.Node,
		Instance:   vm.Instance,
		VMID:       vm.VMID,
		CPUs:       vm.CPUs,
		Uptime:     vm.Uptime,
		Template:   vm.Template,
		LastBackup: vm.LastBackup,
	}
	resource := Resource{
		Type:      ResourceTypeVM,
		Name:      vm.Name,
		Status:    statusFromGuest(vm.Status),
		LastSeen:  vm.LastSeen,
		UpdatedAt: time.Now().UTC(),
		Metrics:   metrics,
		Proxmox:   proxmox,
		Tags:      vm.Tags,
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
		NodeName:   ct.Node,
		Instance:   ct.Instance,
		VMID:       ct.VMID,
		CPUs:       ct.CPUs,
		Uptime:     ct.Uptime,
		Template:   ct.Template,
		LastBackup: ct.LastBackup,
	}
	resource := Resource{
		Type:      ResourceTypeLXC,
		Name:      ct.Name,
		Status:    statusFromGuest(ct.Status),
		LastSeen:  ct.LastSeen,
		UpdatedAt: time.Now().UTC(),
		Metrics:   metrics,
		Proxmox:   proxmox,
		Tags:      ct.Tags,
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
			ZFSPoolState:      zfsPoolState,
			ZFSReadErrors:     zfsReadErrors,
			ZFSWriteErrors:    zfsWriteErrors,
			ZFSChecksumErrors: zfsChecksumErrors,
		},
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
	}

	if disk.SmartAttributes != nil {
		pdMeta.SMART = convertSMARTAttributes(disk.SmartAttributes)
	}

	resource := Resource{
		Type:         ResourceTypePhysicalDisk,
		Name:         name,
		Status:       statusFromPhysicalDisk(disk.Health),
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

func resourceFromDockerContainer(ct models.DockerContainer) (Resource, ResourceIdentity) {
	metrics := metricsFromDockerContainer(ct)
	resource := Resource{
		Type:      ResourceTypeContainer,
		Name:      ct.Name,
		Status:    statusFromDockerState(ct.State),
		LastSeen:  time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
		Metrics:   metrics,
		Docker:    &DockerData{ContainerID: ct.ID, Image: ct.Image, UptimeSeconds: ct.UptimeSeconds},
	}
	identity := ResourceIdentity{
		Hostnames: uniqueStrings([]string{ct.Name}),
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
			AgentID:            cluster.AgentID,
			Context:            cluster.Context,
			Server:             cluster.Server,
			Version:            cluster.Version,
			PendingUninstall:   cluster.PendingUninstall,
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
		Type:      ResourceTypePod,
		Name:      pod.Name,
		Status:    statusFromKubernetesPod(pod),
		LastSeen:  cluster.LastSeen,
		UpdatedAt: now,
		Metrics:   metrics,
		Kubernetes: &K8sData{
			ClusterID:   cluster.ID,
			ClusterName: clusterName,
			AgentID:     cluster.AgentID,
			Context:     cluster.Context,
			Server:      cluster.Server,
			Version:     cluster.Version,
			Namespace:   pod.Namespace,
			PodUID:      pod.UID,
			NodeName:    pod.NodeName,
			PodPhase:    pod.Phase,
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
