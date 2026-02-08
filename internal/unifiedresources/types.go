package unifiedresources

import "time"

// Resource represents a unified resource aggregated across multiple data sources.
type Resource struct {
	ID        string         `json:"id"`
	Type      ResourceType   `json:"type"`
	Name      string         `json:"name"`
	Status    ResourceStatus `json:"status"`
	LastSeen  time.Time      `json:"lastSeen"`
	UpdatedAt time.Time      `json:"updatedAt,omitempty"`

	DiscoveryTarget *DiscoveryTarget `json:"discoveryTarget,omitempty"`

	Sources      []DataSource                `json:"sources"`
	SourceStatus map[DataSource]SourceStatus `json:"sourceStatus,omitempty"`

	Identity ResourceIdentity `json:"identity,omitempty"`
	Metrics  *ResourceMetrics `json:"metrics,omitempty"`

	ParentID   *string `json:"parentId,omitempty"`
	ParentName string  `json:"parentName,omitempty"`
	ChildCount int     `json:"childCount,omitempty"`

	Tags      []string `json:"tags,omitempty"`
	CustomURL string   `json:"customUrl,omitempty"`

	// Source-specific payloads
	Proxmox    *ProxmoxData `json:"proxmox,omitempty"`
	Agent      *AgentData   `json:"agent,omitempty"`
	Docker     *DockerData  `json:"docker,omitempty"`
	PBS        *PBSData     `json:"pbs,omitempty"`
	PMG        *PMGData     `json:"pmg,omitempty"`
	Kubernetes *K8sData     `json:"kubernetes,omitempty"`
}

// DiscoveryTarget describes the canonical discovery request coordinates
// for this unified resource.
type DiscoveryTarget struct {
	ResourceType string `json:"resourceType"`
	HostID       string `json:"hostId"`
	ResourceID   string `json:"resourceId"`
	Hostname     string `json:"hostname,omitempty"`
}

// ResourceType represents the kind of resource.
type ResourceType string

const (
	ResourceTypeHost          ResourceType = "host"
	ResourceTypeVM            ResourceType = "vm"
	ResourceTypeLXC           ResourceType = "lxc"
	ResourceTypeContainer     ResourceType = "container"
	ResourceTypeK8sCluster    ResourceType = "k8s-cluster"
	ResourceTypeK8sNode       ResourceType = "k8s-node"
	ResourceTypePod           ResourceType = "pod"
	ResourceTypeK8sDeployment ResourceType = "k8s-deployment"
	ResourceTypeStorage       ResourceType = "storage"
	ResourceTypePBS           ResourceType = "pbs"
	ResourceTypePMG           ResourceType = "pmg"
	ResourceTypeCeph          ResourceType = "ceph"
)

// ResourceStatus represents the high-level status of a resource.
type ResourceStatus string

const (
	StatusOnline  ResourceStatus = "online"
	StatusOffline ResourceStatus = "offline"
	StatusWarning ResourceStatus = "warning"
	StatusUnknown ResourceStatus = "unknown"
)

// DataSource represents a contributing data source.
type DataSource string

const (
	SourceProxmox DataSource = "proxmox"
	SourceAgent   DataSource = "agent"
	SourceDocker  DataSource = "docker"
	SourcePBS     DataSource = "pbs"
	SourcePMG     DataSource = "pmg"
	SourceK8s     DataSource = "kubernetes"
	SourceTrueNAS DataSource = "truenas"
)

// SourceStatus describes the freshness of data from a source.
type SourceStatus struct {
	Status   string    `json:"status"` // online, stale, offline
	LastSeen time.Time `json:"lastSeen"`
	Error    string    `json:"error,omitempty"`
}

// ResourceIdentity holds identifiers used for matching.
type ResourceIdentity struct {
	MachineID    string   `json:"machineId,omitempty"`
	DMIUUID      string   `json:"dmiUuid,omitempty"`
	Hostnames    []string `json:"hostnames,omitempty"`
	IPAddresses  []string `json:"ipAddresses,omitempty"`
	MACAddresses []string `json:"macAddresses,omitempty"`
	ClusterName  string   `json:"clusterName,omitempty"`
}

// MatchResult describes a potential identity match.
type MatchResult struct {
	ResourceA      string  `json:"resourceA"`
	ResourceB      string  `json:"resourceB"`
	Confidence     float64 `json:"confidence"`
	MatchReason    string  `json:"matchReason"`
	RequiresReview bool    `json:"requiresReview"`
}

// SourceTarget describes a source-specific mapping for a unified resource.
type SourceTarget struct {
	Source      DataSource `json:"source"`
	SourceID    string     `json:"sourceId"`
	CandidateID string     `json:"candidateId"`
}

// ResourceMetrics contains unified metrics derived from available sources.
type ResourceMetrics struct {
	CPU       *MetricValue `json:"cpu,omitempty"`
	Memory    *MetricValue `json:"memory,omitempty"`
	Disk      *MetricValue `json:"disk,omitempty"`
	NetIn     *MetricValue `json:"netIn,omitempty"`
	NetOut    *MetricValue `json:"netOut,omitempty"`
	DiskRead  *MetricValue `json:"diskRead,omitempty"`
	DiskWrite *MetricValue `json:"diskWrite,omitempty"`
}

// MetricValue represents a metric value, optionally with totals.
type MetricValue struct {
	Value   float64    `json:"value,omitempty"`
	Used    *int64     `json:"used,omitempty"`
	Total   *int64     `json:"total,omitempty"`
	Percent float64    `json:"percent,omitempty"`
	Unit    string     `json:"unit,omitempty"`
	Source  DataSource `json:"-"`
}

// ProxmoxData contains Proxmox-specific data for a resource.
type ProxmoxData struct {
	NodeName      string    `json:"nodeName,omitempty"`
	ClusterName   string    `json:"clusterName,omitempty"`
	Instance      string    `json:"instance,omitempty"`
	VMID          int       `json:"vmid,omitempty"`
	CPUs          int       `json:"cpus,omitempty"`
	Template      bool      `json:"template,omitempty"`
	Temperature   *float64  `json:"temperature,omitempty"` // Max node CPU temp in Celsius
	PVEVersion    string    `json:"pveVersion,omitempty"`
	KernelVersion string    `json:"kernelVersion,omitempty"`
	Uptime        int64     `json:"uptime,omitempty"`
	LastBackup    time.Time `json:"lastBackup,omitempty"`
	CPUInfo       *CPUInfo  `json:"cpuInfo,omitempty"`
	// Internal link hint to a host agent resource.
	LinkedHostAgentID string `json:"-"`
}

// AgentData contains host agent-specific data.
type AgentData struct {
	AgentID           string             `json:"agentId,omitempty"`
	AgentVersion      string             `json:"agentVersion,omitempty"`
	Hostname          string             `json:"hostname,omitempty"`
	Platform          string             `json:"platform,omitempty"`
	OSName            string             `json:"osName,omitempty"`
	OSVersion         string             `json:"osVersion,omitempty"`
	KernelVersion     string             `json:"kernelVersion,omitempty"`
	Architecture      string             `json:"architecture,omitempty"`
	UptimeSeconds     int64              `json:"uptimeSeconds,omitempty"`
	Temperature       *float64           `json:"temperature,omitempty"` // Max CPU temp in Celsius
	NetworkInterfaces []NetworkInterface `json:"networkInterfaces,omitempty"`
	Disks             []DiskInfo         `json:"disks,omitempty"`
	// Internal link hints to proxmox resources.
	LinkedNodeID      string `json:"-"`
	LinkedVMID        string `json:"-"`
	LinkedContainerID string `json:"-"`
}

// DockerData contains Docker host-specific data.
type DockerData struct {
	ContainerID       string             `json:"containerId,omitempty"`
	Hostname          string             `json:"hostname,omitempty"`
	Image             string             `json:"image,omitempty"`
	Temperature       *float64           `json:"temperature,omitempty"`
	Runtime           string             `json:"runtime,omitempty"`
	RuntimeVersion    string             `json:"runtimeVersion,omitempty"`
	DockerVersion     string             `json:"dockerVersion,omitempty"`
	OS                string             `json:"os,omitempty"`
	KernelVersion     string             `json:"kernelVersion,omitempty"`
	Architecture      string             `json:"architecture,omitempty"`
	AgentVersion      string             `json:"agentVersion,omitempty"`
	UptimeSeconds     int64              `json:"uptimeSeconds,omitempty"`
	Swarm             *DockerSwarmInfo   `json:"swarm,omitempty"`
	NetworkInterfaces []NetworkInterface `json:"networkInterfaces,omitempty"`
	Disks             []DiskInfo         `json:"disks,omitempty"`
}

// PBSData contains Proxmox Backup Server data.
type PBSData struct {
	InstanceID       string `json:"instanceId,omitempty"`
	Hostname         string `json:"hostname,omitempty"`
	Version          string `json:"version,omitempty"`
	UptimeSeconds    int64  `json:"uptimeSeconds,omitempty"`
	DatastoreCount   int    `json:"datastoreCount,omitempty"`
	BackupJobCount   int    `json:"backupJobCount,omitempty"`
	SyncJobCount     int    `json:"syncJobCount,omitempty"`
	VerifyJobCount   int    `json:"verifyJobCount,omitempty"`
	PruneJobCount    int    `json:"pruneJobCount,omitempty"`
	GarbageJobCount  int    `json:"garbageJobCount,omitempty"`
	ConnectionHealth string `json:"connectionHealth,omitempty"`
}

// PMGData contains Proxmox Mail Gateway data.
type PMGData struct {
	InstanceID       string    `json:"instanceId,omitempty"`
	Hostname         string    `json:"hostname,omitempty"`
	Version          string    `json:"version,omitempty"`
	NodeCount        int       `json:"nodeCount,omitempty"`
	UptimeSeconds    int64     `json:"uptimeSeconds,omitempty"`
	QueueActive      int       `json:"queueActive,omitempty"`
	QueueDeferred    int       `json:"queueDeferred,omitempty"`
	QueueHold        int       `json:"queueHold,omitempty"`
	QueueIncoming    int       `json:"queueIncoming,omitempty"`
	QueueTotal       int       `json:"queueTotal,omitempty"`
	MailCountTotal   float64   `json:"mailCountTotal,omitempty"`
	SpamIn           float64   `json:"spamIn,omitempty"`
	VirusIn          float64   `json:"virusIn,omitempty"`
	ConnectionHealth string    `json:"connectionHealth,omitempty"`
	LastUpdated      time.Time `json:"lastUpdated,omitempty"`
}

// K8sMetricCapabilities describes which Kubernetes metric families are available
// for this cluster right now based on active collection paths.
type K8sMetricCapabilities struct {
	NodeCPUMemory    bool `json:"nodeCpuMemory"`
	NodeTelemetry    bool `json:"nodeTelemetry"`
	PodCPUMemory     bool `json:"podCpuMemory"`
	PodNetwork       bool `json:"podNetwork"`
	PodEphemeralDisk bool `json:"podEphemeralDisk"`
	PodDiskIO        bool `json:"podDiskIo"`
}

// K8sData contains Kubernetes data.
type K8sData struct {
	ClusterID               string                 `json:"clusterId,omitempty"`
	ClusterName             string                 `json:"clusterName,omitempty"`
	AgentID                 string                 `json:"agentId,omitempty"`
	Context                 string                 `json:"context,omitempty"`
	Server                  string                 `json:"server,omitempty"`
	Version                 string                 `json:"version,omitempty"`
	PendingUninstall        bool                   `json:"pendingUninstall,omitempty"`
	NodeUID                 string                 `json:"nodeUid,omitempty"`
	NodeName                string                 `json:"nodeName,omitempty"`
	Ready                   bool                   `json:"ready,omitempty"`
	Unschedulable           bool                   `json:"unschedulable,omitempty"`
	Roles                   []string               `json:"roles,omitempty"`
	KubeletVersion          string                 `json:"kubeletVersion,omitempty"`
	ContainerRuntimeVersion string                 `json:"containerRuntimeVersion,omitempty"`
	OSImage                 string                 `json:"osImage,omitempty"`
	KernelVersion           string                 `json:"kernelVersion,omitempty"`
	Architecture            string                 `json:"architecture,omitempty"`
	CapacityCPU             int64                  `json:"capacityCpuCores,omitempty"`
	CapacityMemoryBytes     int64                  `json:"capacityMemoryBytes,omitempty"`
	CapacityPods            int64                  `json:"capacityPods,omitempty"`
	AllocCPU                int64                  `json:"allocatableCpuCores,omitempty"`
	AllocMemoryBytes        int64                  `json:"allocatableMemoryBytes,omitempty"`
	AllocPods               int64                  `json:"allocatablePods,omitempty"`
	Namespace               string                 `json:"namespace,omitempty"`
	PodUID                  string                 `json:"podUid,omitempty"`
	PodPhase                string                 `json:"podPhase,omitempty"`
	UptimeSeconds           int64                  `json:"uptimeSeconds,omitempty"`
	Temperature             *float64               `json:"temperature,omitempty"`
	Restarts                int                    `json:"restarts,omitempty"`
	OwnerKind               string                 `json:"ownerKind,omitempty"`
	OwnerName               string                 `json:"ownerName,omitempty"`
	Image                   string                 `json:"image,omitempty"`
	Labels                  map[string]string      `json:"labels,omitempty"`
	DeploymentUID           string                 `json:"deploymentUid,omitempty"`
	DesiredReplicas         int32                  `json:"desiredReplicas,omitempty"`
	UpdatedReplicas         int32                  `json:"updatedReplicas,omitempty"`
	ReadyReplicas           int32                  `json:"readyReplicas,omitempty"`
	AvailableReplicas       int32                  `json:"availableReplicas,omitempty"`
	MetricCapabilities      *K8sMetricCapabilities `json:"metricCapabilities,omitempty"`
}

// CPUInfo describes CPU characteristics.
type CPUInfo struct {
	Model   string `json:"model,omitempty"`
	Cores   int    `json:"cores,omitempty"`
	Sockets int    `json:"sockets,omitempty"`
}

// NetworkInterface describes a network interface.
type NetworkInterface struct {
	Name      string   `json:"name"`
	MAC       string   `json:"mac,omitempty"`
	Addresses []string `json:"addresses,omitempty"`
	SpeedMbps *int64   `json:"speedMbps,omitempty"`
	Status    string   `json:"status,omitempty"`
}

// DiskInfo describes disk usage.
type DiskInfo struct {
	Device     string `json:"device,omitempty"`
	Mountpoint string `json:"mountpoint,omitempty"`
	Filesystem string `json:"filesystem,omitempty"`
	Total      int64  `json:"total,omitempty"`
	Used       int64  `json:"used,omitempty"`
	Free       int64  `json:"free,omitempty"`
}

// DockerSwarmInfo captures Docker Swarm details.
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

// ResourceStats contains aggregated stats for a set of resources.
type ResourceStats struct {
	Total    int                    `json:"total"`
	ByType   map[ResourceType]int   `json:"byType"`
	ByStatus map[ResourceStatus]int `json:"byStatus"`
	BySource map[DataSource]int     `json:"bySource"`
}
