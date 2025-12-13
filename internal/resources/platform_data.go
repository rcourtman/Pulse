package resources

import "time"

// NodePlatformData contains Proxmox VE node-specific fields.
// Stored in Resource.PlatformData when Type is ResourceTypeNode.
type NodePlatformData struct {
	Instance      string    `json:"instance"`      // Proxmox instance URL
	Host          string    `json:"host"`          // Full host URL from config
	GuestURL      string    `json:"guestURL"`      // Optional guest-accessible URL
	PVEVersion    string    `json:"pveVersion"`    // Proxmox VE version
	KernelVersion string    `json:"kernelVersion"` // Linux kernel version
	CPUInfo       CPUInfo   `json:"cpuInfo"`       // CPU details
	LoadAverage   []float64 `json:"loadAverage"`   // 1, 5, 15 minute load averages

	// Cluster information
	IsClusterMember bool   `json:"isClusterMember"`
	ClusterName     string `json:"clusterName"`

	// Connection status
	ConnectionHealth string `json:"connectionHealth"`
}

// CPUInfo contains CPU hardware details.
type CPUInfo struct {
	Model   string `json:"model"`
	Cores   int    `json:"cores"`
	Sockets int    `json:"sockets"`
}

// VMPlatformData contains Proxmox VM-specific fields.
// Stored in Resource.PlatformData when Type is ResourceTypeVM.
type VMPlatformData struct {
	VMID         int      `json:"vmid"`
	Node         string   `json:"node"`     // Proxmox node hosting this VM
	Instance     string   `json:"instance"` // Proxmox instance URL
	CPUs         int      `json:"cpus"`     // Number of vCPUs
	Template     bool     `json:"template"`
	Lock         string   `json:"lock,omitempty"` // Lock status (backup, migrate, etc.)
	AgentVersion string   `json:"agentVersion,omitempty"`
	OSName       string   `json:"osName,omitempty"`
	OSVersion    string   `json:"osVersion,omitempty"`
	IPAddresses  []string `json:"ipAddresses,omitempty"`

	// I/O stats
	NetworkIn  int64 `json:"networkIn"`
	NetworkOut int64 `json:"networkOut"`
	DiskRead   int64 `json:"diskRead"`
	DiskWrite  int64 `json:"diskWrite"`

	// Backup info
	LastBackup *time.Time `json:"lastBackup,omitempty"`
}

// ContainerPlatformData contains Proxmox LXC container-specific fields.
// Stored in Resource.PlatformData when Type is ResourceTypeContainer or ResourceTypeOCIContainer.
type ContainerPlatformData struct {
	VMID        int      `json:"vmid"`
	Node        string   `json:"node"`           // Proxmox node hosting this container
	Instance    string   `json:"instance"`       // Proxmox instance URL
	Type        string   `json:"type,omitempty"` // lxc or oci
	CPUs        int      `json:"cpus"`           // Number of vCPUs
	Template    bool     `json:"template"`
	Lock        string   `json:"lock,omitempty"`
	OSName      string   `json:"osName,omitempty"`
	IPAddresses []string `json:"ipAddresses,omitempty"`
	// OCI container support (Proxmox VE 9.1+)
	IsOCI      bool   `json:"isOci,omitempty"`      // True if this is an OCI container
	OSTemplate string `json:"osTemplate,omitempty"` // Template or OCI image reference if available

	// I/O stats
	NetworkIn  int64 `json:"networkIn"`
	NetworkOut int64 `json:"networkOut"`
	DiskRead   int64 `json:"diskRead"`
	DiskWrite  int64 `json:"diskWrite"`

	// Backup info
	LastBackup *time.Time `json:"lastBackup,omitempty"`
}

// HostPlatformData contains host-agent specific fields.
// Stored in Resource.PlatformData when Type is ResourceTypeHost.
type HostPlatformData struct {
	Platform      string             `json:"platform,omitempty"`      // linux, windows, darwin
	OSName        string             `json:"osName,omitempty"`        // e.g., "Ubuntu 22.04"
	OSVersion     string             `json:"osVersion,omitempty"`     // OS version string
	KernelVersion string             `json:"kernelVersion,omitempty"` // Kernel version
	Architecture  string             `json:"architecture,omitempty"`  // amd64, arm64, etc.
	CPUCount      int                `json:"cpuCount,omitempty"`      // Number of CPUs
	LoadAverage   []float64          `json:"loadAverage,omitempty"`   // 1, 5, 15 minute loads
	AgentVersion  string             `json:"agentVersion,omitempty"`  // Pulse agent version
	IsLegacy      bool               `json:"isLegacy,omitempty"`      // Legacy agent indicator
	Sensors       HostSensorSummary  `json:"sensors,omitempty"`       // Temperature/fan sensors
	RAID          []HostRAIDArray    `json:"raid,omitempty"`          // RAID arrays
	DiskIO        []DiskIOStats      `json:"diskIO,omitempty"`        // Per-disk I/O stats
	Disks         []DiskInfo         `json:"disks,omitempty"`         // Disk usage info
	Interfaces    []NetworkInterface `json:"interfaces,omitempty"`    // Network interfaces

	// Token information
	TokenID         string     `json:"tokenId,omitempty"`
	TokenName       string     `json:"tokenName,omitempty"`
	TokenHint       string     `json:"tokenHint,omitempty"`
	TokenLastUsedAt *time.Time `json:"tokenLastUsedAt,omitempty"`
}

// HostSensorSummary captures sensor readings from a host.
type HostSensorSummary struct {
	TemperatureCelsius map[string]float64 `json:"temperatureCelsius,omitempty"`
	FanRPM             map[string]float64 `json:"fanRpm,omitempty"`
	Additional         map[string]float64 `json:"additional,omitempty"`
}

// HostRAIDArray represents an mdadm RAID array.
type HostRAIDArray struct {
	Device         string           `json:"device"`
	Name           string           `json:"name,omitempty"`
	Level          string           `json:"level"`
	State          string           `json:"state"`
	TotalDevices   int              `json:"totalDevices"`
	ActiveDevices  int              `json:"activeDevices"`
	WorkingDevices int              `json:"workingDevices"`
	FailedDevices  int              `json:"failedDevices"`
	SpareDevices   int              `json:"spareDevices"`
	UUID           string           `json:"uuid,omitempty"`
	Devices        []HostRAIDDevice `json:"devices"`
	RebuildPercent float64          `json:"rebuildPercent"`
	RebuildSpeed   string           `json:"rebuildSpeed,omitempty"`
}

// HostRAIDDevice represents a device in a RAID array.
type HostRAIDDevice struct {
	Device string `json:"device"`
	State  string `json:"state"`
	Slot   int    `json:"slot"`
}

// DiskIOStats captures I/O statistics for a disk device.
type DiskIOStats struct {
	Device      string `json:"device"`
	ReadBytes   uint64 `json:"readBytes,omitempty"`
	WriteBytes  uint64 `json:"writeBytes,omitempty"`
	ReadOps     uint64 `json:"readOps,omitempty"`
	WriteOps    uint64 `json:"writeOps,omitempty"`
	ReadTimeMs  uint64 `json:"readTimeMs,omitempty"`
	WriteTimeMs uint64 `json:"writeTimeMs,omitempty"`
	IOTimeMs    uint64 `json:"ioTimeMs,omitempty"`
}

// DiskInfo represents disk/partition usage.
type DiskInfo struct {
	Mountpoint string  `json:"mountpoint,omitempty"`
	Device     string  `json:"device,omitempty"`
	Type       string  `json:"type,omitempty"`
	Total      int64   `json:"total"`
	Used       int64   `json:"used"`
	Free       int64   `json:"free"`
	Usage      float64 `json:"usage"`
}

// NetworkInterface represents a network interface.
type NetworkInterface struct {
	Name      string   `json:"name"`
	MAC       string   `json:"mac,omitempty"`
	Addresses []string `json:"addresses,omitempty"`
	RXBytes   uint64   `json:"rxBytes,omitempty"`
	TXBytes   uint64   `json:"txBytes,omitempty"`
	SpeedMbps *int64   `json:"speedMbps,omitempty"`
}

// DockerHostPlatformData contains Docker host-specific fields.
// Stored in Resource.PlatformData when Type is ResourceTypeDockerHost.
type DockerHostPlatformData struct {
	AgentID           string             `json:"agentId"`
	MachineID         string             `json:"machineId,omitempty"`
	OS                string             `json:"os,omitempty"`
	KernelVersion     string             `json:"kernelVersion,omitempty"`
	Architecture      string             `json:"architecture,omitempty"`
	Runtime           string             `json:"runtime,omitempty"` // docker, podman
	RuntimeVersion    string             `json:"runtimeVersion,omitempty"`
	DockerVersion     string             `json:"dockerVersion,omitempty"`
	LoadAverage       []float64          `json:"loadAverage,omitempty"`
	AgentVersion      string             `json:"agentVersion,omitempty"`
	CPUs              int                `json:"cpus"`
	IsLegacy          bool               `json:"isLegacy,omitempty"`
	Disks             []DiskInfo         `json:"disks,omitempty"`
	Interfaces        []NetworkInterface `json:"interfaces,omitempty"`
	CustomDisplayName string             `json:"customDisplayName,omitempty"`
	Hidden            bool               `json:"hidden"`
	PendingUninstall  bool               `json:"pendingUninstall"`

	// Swarm information
	Swarm *DockerSwarmInfo `json:"swarm,omitempty"`

	// Token information
	TokenID         string     `json:"tokenId,omitempty"`
	TokenName       string     `json:"tokenName,omitempty"`
	TokenHint       string     `json:"tokenHint,omitempty"`
	TokenLastUsedAt *time.Time `json:"tokenLastUsedAt,omitempty"`
}

// DockerSwarmInfo captures Docker Swarm membership details.
type DockerSwarmInfo struct {
	NodeID           string `json:"nodeId,omitempty"`
	NodeRole         string `json:"nodeRole,omitempty"`
	LocalState       string `json:"localState,omitempty"`
	ControlAvailable bool   `json:"controlAvailable,omitempty"`
	ClusterID        string `json:"clusterId,omitempty"`
	ClusterName      string `json:"clusterName,omitempty"`
	Scope            string `json:"scope,omitempty"`
	Error            string `json:"error,omitempty"`
}

// DockerContainerPlatformData contains Docker container-specific fields.
// Stored in Resource.PlatformData when Type is ResourceTypeDockerContainer.
type DockerContainerPlatformData struct {
	HostID       string             `json:"hostId"`   // Parent Docker host ID
	HostName     string             `json:"hostName"` // Parent Docker host name
	Image        string             `json:"image"`    // Container image
	State        string             `json:"state"`    // created, running, paused, restarting, exited, dead
	Status       string             `json:"status"`   // Human-readable status
	Health       string             `json:"health"`   // healthy, unhealthy, starting, none
	RestartCount int                `json:"restartCount"`
	ExitCode     int                `json:"exitCode"`
	CreatedAt    time.Time          `json:"createdAt"`
	StartedAt    *time.Time         `json:"startedAt,omitempty"`
	FinishedAt   *time.Time         `json:"finishedAt,omitempty"`
	Labels       map[string]string  `json:"labels,omitempty"`
	Ports        []ContainerPort    `json:"ports,omitempty"`
	Networks     []ContainerNetwork `json:"networks,omitempty"`

	// Podman-specific
	Podman *PodmanContainerInfo `json:"podman,omitempty"`
}

// KubernetesClusterPlatformData contains Kubernetes cluster-specific fields.
// Stored in Resource.PlatformData when Type is ResourceTypeK8sCluster.
type KubernetesClusterPlatformData struct {
	AgentID           string `json:"agentId"`
	Server            string `json:"server,omitempty"`
	Context           string `json:"context,omitempty"`
	Version           string `json:"version,omitempty"`
	CustomDisplayName string `json:"customDisplayName,omitempty"`
	Hidden            bool   `json:"hidden"`
	PendingUninstall  bool   `json:"pendingUninstall"`

	NodeCount       int `json:"nodeCount,omitempty"`
	PodCount        int `json:"podCount,omitempty"`
	DeploymentCount int `json:"deploymentCount,omitempty"`

	// Token information
	TokenID         string     `json:"tokenId,omitempty"`
	TokenName       string     `json:"tokenName,omitempty"`
	TokenHint       string     `json:"tokenHint,omitempty"`
	TokenLastUsedAt *time.Time `json:"tokenLastUsedAt,omitempty"`
}

// KubernetesNodePlatformData contains Kubernetes node-specific fields.
// Stored in Resource.PlatformData when Type is ResourceTypeK8sNode.
type KubernetesNodePlatformData struct {
	ClusterID               string   `json:"clusterId"`
	Ready                   bool     `json:"ready"`
	Unschedulable           bool     `json:"unschedulable,omitempty"`
	KubeletVersion          string   `json:"kubeletVersion,omitempty"`
	ContainerRuntimeVersion string   `json:"containerRuntimeVersion,omitempty"`
	OSImage                 string   `json:"osImage,omitempty"`
	KernelVersion           string   `json:"kernelVersion,omitempty"`
	Architecture            string   `json:"architecture,omitempty"`
	CapacityCPUCores        int64    `json:"capacityCpuCores,omitempty"`
	CapacityMemoryBytes     int64    `json:"capacityMemoryBytes,omitempty"`
	CapacityPods            int64    `json:"capacityPods,omitempty"`
	AllocatableCPUCores     int64    `json:"allocatableCpuCores,omitempty"`
	AllocatableMemoryBytes  int64    `json:"allocatableMemoryBytes,omitempty"`
	AllocatablePods         int64    `json:"allocatablePods,omitempty"`
	Roles                   []string `json:"roles,omitempty"`
}

// KubernetesPodPlatformData contains Kubernetes pod-specific fields.
// Stored in Resource.PlatformData when Type is ResourceTypePod.
type KubernetesPodPlatformData struct {
	ClusterID string `json:"clusterId"`
	Namespace string `json:"namespace"`
	NodeName  string `json:"nodeName,omitempty"`
	Phase     string `json:"phase,omitempty"`
	Reason    string `json:"reason,omitempty"`
	Message   string `json:"message,omitempty"`
	QoSClass  string `json:"qosClass,omitempty"`
	Restarts  int    `json:"restarts,omitempty"`

	OwnerKind string `json:"ownerKind,omitempty"`
	OwnerName string `json:"ownerName,omitempty"`

	Containers []KubernetesPodContainerInfo `json:"containers,omitempty"`
}

// KubernetesPodContainerInfo captures per-container pod status.
type KubernetesPodContainerInfo struct {
	Name         string `json:"name"`
	Image        string `json:"image,omitempty"`
	Ready        bool   `json:"ready"`
	RestartCount int32  `json:"restartCount,omitempty"`
	State        string `json:"state,omitempty"`
	Reason       string `json:"reason,omitempty"`
	Message      string `json:"message,omitempty"`
}

// KubernetesDeploymentPlatformData contains Kubernetes deployment-specific fields.
// Stored in Resource.PlatformData when Type is ResourceTypeK8sDeployment.
type KubernetesDeploymentPlatformData struct {
	ClusterID         string `json:"clusterId"`
	Namespace         string `json:"namespace"`
	DesiredReplicas   int32  `json:"desiredReplicas,omitempty"`
	UpdatedReplicas   int32  `json:"updatedReplicas,omitempty"`
	ReadyReplicas     int32  `json:"readyReplicas,omitempty"`
	AvailableReplicas int32  `json:"availableReplicas,omitempty"`
}

// ContainerPort describes a port mapping.
type ContainerPort struct {
	PrivatePort int    `json:"privatePort"`
	PublicPort  int    `json:"publicPort,omitempty"`
	Protocol    string `json:"protocol"`
	IP          string `json:"ip,omitempty"`
}

// ContainerNetwork describes a container's network attachment.
type ContainerNetwork struct {
	Name string `json:"name"`
	IPv4 string `json:"ipv4,omitempty"`
	IPv6 string `json:"ipv6,omitempty"`
}

// PodmanContainerInfo captures Podman-specific container info.
type PodmanContainerInfo struct {
	PodName        string `json:"podName,omitempty"`
	PodID          string `json:"podId,omitempty"`
	Infra          bool   `json:"infra,omitempty"`
	ComposeProject string `json:"composeProject,omitempty"`
	ComposeService string `json:"composeService,omitempty"`
}

// PBSPlatformData contains PBS-specific fields.
// Stored in Resource.PlatformData when Type is ResourceTypePBS.
type PBSPlatformData struct {
	Host             string `json:"host"`
	Version          string `json:"version"`
	ConnectionHealth string `json:"connectionHealth"`
	MemoryUsed       int64  `json:"memoryUsed"`
	MemoryTotal      int64  `json:"memoryTotal"`
	NumDatastores    int    `json:"numDatastores"`
}

// DatastorePlatformData contains PBS datastore-specific fields.
// Stored in Resource.PlatformData when Type is ResourceTypeDatastore.
type DatastorePlatformData struct {
	PBSInstanceID       string  `json:"pbsInstanceId"`
	PBSInstanceName     string  `json:"pbsInstanceName"`
	Content             string  `json:"content,omitempty"`
	Error               string  `json:"error,omitempty"`
	DeduplicationFactor float64 `json:"deduplicationFactor,omitempty"`
}

// StoragePlatformData contains Proxmox storage-specific fields.
// Stored in Resource.PlatformData when Type is ResourceTypeStorage.
type StoragePlatformData struct {
	Instance string   `json:"instance"`
	Node     string   `json:"node"`            // Primary node
	Nodes    []string `json:"nodes,omitempty"` // All nodes (for shared storage)
	Type     string   `json:"type"`            // zfspool, lvmthin, cephfs, etc.
	Content  string   `json:"content"`
	Shared   bool     `json:"shared"`
	Enabled  bool     `json:"enabled"`
	Active   bool     `json:"active"`
}
