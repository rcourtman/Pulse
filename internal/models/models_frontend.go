package models

import "encoding/json"

// Frontend-friendly type aliases with proper JSON tags
// These extend the base types with additional computed fields

// NodeFrontend represents a Node with frontend-friendly field names
type NodeFrontend struct {
	ID                           string       `json:"id"`
	Node                         string       `json:"node"` // Maps to Name
	Name                         string       `json:"name"`
	DisplayName                  string       `json:"displayName"`
	Instance                     string       `json:"instance"`
	Host                         string       `json:"host,omitempty"`
	GuestURL                     string       `json:"guestURL,omitempty"` // Optional guest-accessible URL (for navigation)
	Status                       string       `json:"status"`
	Type                         string       `json:"type"`
	CPU                          float64      `json:"cpu"`
	Memory                       *Memory      `json:"memory,omitempty"` // Full memory object with usage percentage
	Mem                          int64        `json:"mem"`              // Maps to Memory.Used (kept for backward compat)
	MaxMem                       int64        `json:"maxmem"`           // Maps to Memory.Total (kept for backward compat)
	Disk                         *Disk        `json:"disk,omitempty"`   // Full disk object with usage percentage
	MaxDisk                      int64        `json:"maxdisk"`          // Maps to Disk.Total (kept for backward compat)
	Uptime                       int64        `json:"uptime"`
	LoadAverage                  []float64    `json:"loadAverage"`
	KernelVersion                string       `json:"kernelVersion"`
	PVEVersion                   string       `json:"pveVersion"`
	CPUInfo                      CPUInfo      `json:"cpuInfo"`
	Temperature                  *Temperature `json:"temperature,omitempty"` // CPU/NVMe temperatures
	LastSeen                     int64        `json:"lastSeen"`              // Unix timestamp
	ConnectionHealth             string       `json:"connectionHealth"`
	IsClusterMember              bool         `json:"isClusterMember,omitempty"`
	ClusterName                  string       `json:"clusterName,omitempty"`
	TemperatureMonitoringEnabled *bool        `json:"temperatureMonitoringEnabled,omitempty"` // Per-node temperature monitoring override
	LinkedAgentID                string       `json:"linkedAgentId,omitempty"`                // ID of linked agent (for "Via agent" badge)
}

func (n NodeFrontend) NormalizeCollections() NodeFrontend {
	if n.LoadAverage == nil {
		n.LoadAverage = []float64{}
	}
	return n
}

// VMFrontend represents a VM with frontend-friendly field names
type VMFrontend struct {
	ID                string                  `json:"id"`
	VMID              int                     `json:"vmid"`
	Name              string                  `json:"name"`
	Node              string                  `json:"node"`
	Instance          string                  `json:"instance"`
	Status            string                  `json:"status"`
	Type              string                  `json:"type"`
	CPU               float64                 `json:"cpu"`
	CPUs              int                     `json:"cpus"`
	Memory            *Memory                 `json:"memory,omitempty"`           // Full memory object
	Mem               int64                   `json:"mem"`                        // Maps to Memory.Used
	MaxMem            int64                   `json:"maxmem"`                     // Maps to Memory.Total
	DiskObj           *Disk                   `json:"disk,omitempty"`             // Full disk object
	Disks             []Disk                  `json:"disks"`                      // Individual filesystem/disk usage
	DiskStatusReason  string                  `json:"diskStatusReason,omitempty"` // Why disk stats are unavailable
	OSName            string                  `json:"osName,omitempty"`
	OSVersion         string                  `json:"osVersion,omitempty"`
	AgentVersion      string                  `json:"agentVersion,omitempty"`
	NetworkInterfaces []GuestNetworkInterface `json:"networkInterfaces"`
	IPAddresses       []string                `json:"ipAddresses"`
	NetIn             int64                   `json:"networkIn"`  // Maps to NetworkIn (camelCase for frontend)
	NetOut            int64                   `json:"networkOut"` // Maps to NetworkOut (camelCase for frontend)
	DiskRead          int64                   `json:"diskRead"`   // Maps to DiskRead (camelCase for frontend)
	DiskWrite         int64                   `json:"diskWrite"`  // Maps to DiskWrite (camelCase for frontend)
	Uptime            int64                   `json:"uptime"`
	Template          bool                    `json:"template"`
	LastBackup        int64                   `json:"lastBackup,omitempty"` // Unix timestamp
	Tags              string                  `json:"tags,omitempty"`       // Joined string
	Lock              string                  `json:"lock,omitempty"`
	LastSeen          int64                   `json:"lastSeen"` // Unix timestamp
}

func (v VMFrontend) NormalizeCollections() VMFrontend {
	if v.Disks == nil {
		v.Disks = []Disk{}
	}
	if v.NetworkInterfaces == nil {
		v.NetworkInterfaces = []GuestNetworkInterface{}
	}
	if v.IPAddresses == nil {
		v.IPAddresses = []string{}
	}
	return v
}

// ContainerFrontend represents a Container with frontend-friendly field names
type ContainerFrontend struct {
	ID       string `json:"id"`
	VMID     int    `json:"vmid"`
	Name     string `json:"name"`
	Node     string `json:"node"`
	Instance string `json:"instance"`
	Status   string `json:"status"`
	Type     string `json:"type"`
	// OCI container support (Proxmox VE 9.1+)
	IsOCI             bool                    `json:"isOci,omitempty"`      // True if this is an OCI container
	OSTemplate        string                  `json:"osTemplate,omitempty"` // Template or OCI image used (e.g., "oci:docker.io/library/alpine:latest")
	CPU               float64                 `json:"cpu"`
	CPUs              int                     `json:"cpus"`
	Memory            *Memory                 `json:"memory,omitempty"` // Full memory object
	Mem               int64                   `json:"mem"`              // Maps to Memory.Used
	MaxMem            int64                   `json:"maxmem"`           // Maps to Memory.Total
	DiskObj           *Disk                   `json:"disk,omitempty"`   // Full disk object
	Disks             []Disk                  `json:"disks"`            // Individual filesystem/disk usage
	NetworkInterfaces []GuestNetworkInterface `json:"networkInterfaces"`
	IPAddresses       []string                `json:"ipAddresses"`
	NetIn             int64                   `json:"networkIn"`  // Maps to NetworkIn (camelCase for frontend)
	NetOut            int64                   `json:"networkOut"` // Maps to NetworkOut (camelCase for frontend)
	DiskRead          int64                   `json:"diskRead"`   // Maps to DiskRead (camelCase for frontend)
	DiskWrite         int64                   `json:"diskWrite"`  // Maps to DiskWrite (camelCase for frontend)
	Uptime            int64                   `json:"uptime"`
	Template          bool                    `json:"template"`
	LastBackup        int64                   `json:"lastBackup,omitempty"` // Unix timestamp
	Tags              string                  `json:"tags,omitempty"`       // Joined string
	Lock              string                  `json:"lock,omitempty"`
	LastSeen          int64                   `json:"lastSeen"` // Unix timestamp
	OSName            string                  `json:"osName,omitempty"`
}

func (c ContainerFrontend) NormalizeCollections() ContainerFrontend {
	if c.Disks == nil {
		c.Disks = []Disk{}
	}
	if c.NetworkInterfaces == nil {
		c.NetworkInterfaces = []GuestNetworkInterface{}
	}
	if c.IPAddresses == nil {
		c.IPAddresses = []string{}
	}
	return c
}

// DockerHostFrontend represents a Docker host with frontend-friendly fields
type DockerHostFrontend struct {
	ID                string                     `json:"id"`
	AgentID           string                     `json:"agentId"`
	Hostname          string                     `json:"hostname"`
	DisplayName       string                     `json:"displayName"`
	CustomDisplayName string                     `json:"customDisplayName,omitempty"`
	MachineID         string                     `json:"machineId,omitempty"`
	OS                string                     `json:"os,omitempty"`
	KernelVersion     string                     `json:"kernelVersion,omitempty"`
	Architecture      string                     `json:"architecture,omitempty"`
	Runtime           string                     `json:"runtime"`
	RuntimeVersion    string                     `json:"runtimeVersion,omitempty"`
	DockerVersion     string                     `json:"dockerVersion,omitempty"`
	CPUs              int                        `json:"cpus"`
	TotalMemoryBytes  int64                      `json:"totalMemoryBytes"`
	UptimeSeconds     int64                      `json:"uptimeSeconds"`
	CPUUsagePercent   float64                    `json:"cpuUsagePercent"`
	LoadAverage       []float64                  `json:"loadAverage"`
	Memory            *Memory                    `json:"memory,omitempty"`
	Disks             []Disk                     `json:"disks"`
	NetworkInterfaces []HostNetworkInterface     `json:"networkInterfaces"`
	Status            string                     `json:"status"`
	LastSeen          int64                      `json:"lastSeen"`
	IntervalSeconds   int                        `json:"intervalSeconds"`
	AgentVersion      string                     `json:"agentVersion,omitempty"`
	Containers        []DockerContainerFrontend  `json:"containers"`
	Services          []DockerServiceFrontend    `json:"services"`
	Tasks             []DockerTaskFrontend       `json:"tasks"`
	Swarm             *DockerSwarmFrontend       `json:"swarm,omitempty"`
	TokenID           string                     `json:"tokenId,omitempty"`
	TokenName         string                     `json:"tokenName,omitempty"`
	TokenHint         string                     `json:"tokenHint,omitempty"`
	TokenLastUsedAt   *int64                     `json:"tokenLastUsedAt,omitempty"`
	PendingUninstall  bool                       `json:"pendingUninstall"`
	Command           *DockerHostCommandFrontend `json:"command,omitempty"`
}

func (h DockerHostFrontend) NormalizeCollections() DockerHostFrontend {
	if h.LoadAverage == nil {
		h.LoadAverage = []float64{}
	}
	if h.Disks == nil {
		h.Disks = []Disk{}
	}
	if h.NetworkInterfaces == nil {
		h.NetworkInterfaces = []HostNetworkInterface{}
	}
	if h.Containers == nil {
		h.Containers = []DockerContainerFrontend{}
	}
	if h.Services == nil {
		h.Services = []DockerServiceFrontend{}
	}
	if h.Tasks == nil {
		h.Tasks = []DockerTaskFrontend{}
	}
	for i := range h.Containers {
		h.Containers[i] = h.Containers[i].NormalizeCollections()
	}
	for i := range h.Services {
		h.Services[i] = h.Services[i].NormalizeCollections()
	}
	return h
}

// ConnectedInfrastructureSurfaceFrontend describes one reporting surface that
// Pulse associates with an infrastructure item in the Connected infrastructure
// settings view.
type ConnectedInfrastructureSurfaceFrontend struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	Label     string `json:"label"`
	Detail    string `json:"detail,omitempty"`
	ControlID string `json:"controlId,omitempty"`
	Action    string `json:"action,omitempty"`
	IDLabel   string `json:"idLabel,omitempty"`
	IDValue   string `json:"idValue,omitempty"`
}

// ConnectedInfrastructureItemFrontend is the canonical Connected
// infrastructure projection for the settings surface. It is derived from the
// unified resource model plus reporting-ignore state, so the frontend does not
// need to reconstruct machine rows from raw resource facets.
type ConnectedInfrastructureItemFrontend struct {
	ID                string                                   `json:"id"`
	Name              string                                   `json:"name"`
	DisplayName       string                                   `json:"displayName,omitempty"`
	Hostname          string                                   `json:"hostname,omitempty"`
	Status            string                                   `json:"status"`
	HealthStatus      string                                   `json:"healthStatus,omitempty"`
	LastSeen          int64                                    `json:"lastSeen,omitempty"`
	RemovedAt         int64                                    `json:"removedAt,omitempty"`
	Version           string                                   `json:"version,omitempty"`
	IsOutdatedBinary  bool                                     `json:"isOutdatedBinary,omitempty"`
	LinkedNodeID      string                                   `json:"linkedNodeId,omitempty"`
	CommandsEnabled   bool                                     `json:"commandsEnabled,omitempty"`
	ScopeAgentID      string                                   `json:"scopeAgentId,omitempty"`
	UpgradePlatform   string                                   `json:"upgradePlatform,omitempty"`
	UninstallAgentID  string                                   `json:"uninstallAgentId,omitempty"`
	UninstallHostname string                                   `json:"uninstallHostname,omitempty"`
	Surfaces          []ConnectedInfrastructureSurfaceFrontend `json:"surfaces"`
}

func (i ConnectedInfrastructureItemFrontend) NormalizeCollections() ConnectedInfrastructureItemFrontend {
	if i.Surfaces == nil {
		i.Surfaces = []ConnectedInfrastructureSurfaceFrontend{}
	}
	return i
}

// KubernetesClusterFrontend represents a Kubernetes cluster for the frontend.
type KubernetesClusterFrontend struct {
	ID                string `json:"id"`
	AgentID           string `json:"agentId"`
	Name              string `json:"name,omitempty"`
	DisplayName       string `json:"displayName,omitempty"`
	CustomDisplayName string `json:"customDisplayName,omitempty"`
	Server            string `json:"server,omitempty"`
	Context           string `json:"context,omitempty"`
	Version           string `json:"version,omitempty"`
	Status            string `json:"status"`
	LastSeen          int64  `json:"lastSeen"`
	IntervalSeconds   int    `json:"intervalSeconds"`
	AgentVersion      string `json:"agentVersion,omitempty"`
	TokenID           string `json:"tokenId,omitempty"`
	TokenName         string `json:"tokenName,omitempty"`
	TokenHint         string `json:"tokenHint,omitempty"`
	TokenLastUsedAt   *int64 `json:"tokenLastUsedAt,omitempty"`
	Hidden            bool   `json:"hidden"`
	PendingUninstall  bool   `json:"pendingUninstall"`

	Nodes       []KubernetesNodeFrontend       `json:"nodes"`
	Pods        []KubernetesPodFrontend        `json:"pods"`
	Deployments []KubernetesDeploymentFrontend `json:"deployments"`
}

func (c KubernetesClusterFrontend) NormalizeCollections() KubernetesClusterFrontend {
	if c.Nodes == nil {
		c.Nodes = []KubernetesNodeFrontend{}
	}
	if c.Pods == nil {
		c.Pods = []KubernetesPodFrontend{}
	}
	if c.Deployments == nil {
		c.Deployments = []KubernetesDeploymentFrontend{}
	}
	for i := range c.Nodes {
		c.Nodes[i] = c.Nodes[i].NormalizeCollections()
	}
	for i := range c.Pods {
		c.Pods[i] = c.Pods[i].NormalizeCollections()
	}
	for i := range c.Deployments {
		c.Deployments[i] = c.Deployments[i].NormalizeCollections()
	}
	return c
}

type KubernetesNodeFrontend struct {
	UID                     string   `json:"uid"`
	Name                    string   `json:"name"`
	Ready                   bool     `json:"ready"`
	Unschedulable           bool     `json:"unschedulable,omitempty"`
	KubeletVersion          string   `json:"kubeletVersion,omitempty"`
	ContainerRuntimeVersion string   `json:"containerRuntimeVersion,omitempty"`
	OSImage                 string   `json:"osImage,omitempty"`
	KernelVersion           string   `json:"kernelVersion,omitempty"`
	Architecture            string   `json:"architecture,omitempty"`
	CapacityCPU             int64    `json:"capacityCpuCores,omitempty"`
	CapacityMemoryBytes     int64    `json:"capacityMemoryBytes,omitempty"`
	CapacityPods            int64    `json:"capacityPods,omitempty"`
	AllocCPU                int64    `json:"allocatableCpuCores,omitempty"`
	AllocMemoryBytes        int64    `json:"allocatableMemoryBytes,omitempty"`
	AllocPods               int64    `json:"allocatablePods,omitempty"`
	UsageCPUMilliCores      int64    `json:"usageCpuMilliCores,omitempty"`
	UsageMemoryBytes        int64    `json:"usageMemoryBytes,omitempty"`
	UsageCPUPercent         float64  `json:"usageCpuPercent,omitempty"`
	UsageMemoryPercent      float64  `json:"usageMemoryPercent,omitempty"`
	Roles                   []string `json:"roles"`
}

func (n KubernetesNodeFrontend) NormalizeCollections() KubernetesNodeFrontend {
	if n.Roles == nil {
		n.Roles = []string{}
	}
	return n
}

type KubernetesPodFrontend struct {
	UID                           string                           `json:"uid"`
	Name                          string                           `json:"name"`
	Namespace                     string                           `json:"namespace"`
	NodeName                      string                           `json:"nodeName,omitempty"`
	Phase                         string                           `json:"phase,omitempty"`
	Reason                        string                           `json:"reason,omitempty"`
	Message                       string                           `json:"message,omitempty"`
	QoSClass                      string                           `json:"qosClass,omitempty"`
	CreatedAt                     int64                            `json:"createdAt,omitempty"`
	StartTime                     *int64                           `json:"startTime,omitempty"`
	Restarts                      int                              `json:"restarts,omitempty"`
	UsageCPUMilliCores            int                              `json:"usageCpuMilliCores,omitempty"`
	UsageMemoryBytes              int64                            `json:"usageMemoryBytes,omitempty"`
	UsageCPUPercent               float64                          `json:"usageCpuPercent,omitempty"`
	UsageMemoryPercent            float64                          `json:"usageMemoryPercent,omitempty"`
	NetworkRxBytes                int64                            `json:"networkRxBytes,omitempty"`
	NetworkTxBytes                int64                            `json:"networkTxBytes,omitempty"`
	NetInRate                     float64                          `json:"netInRate,omitempty"`
	NetOutRate                    float64                          `json:"netOutRate,omitempty"`
	EphemeralStorageUsedBytes     int64                            `json:"ephemeralStorageUsedBytes,omitempty"`
	EphemeralStorageCapacityBytes int64                            `json:"ephemeralStorageCapacityBytes,omitempty"`
	DiskUsagePercent              float64                          `json:"diskUsagePercent,omitempty"`
	Labels                        map[string]string                `json:"labels"`
	OwnerKind                     string                           `json:"ownerKind,omitempty"`
	OwnerName                     string                           `json:"ownerName,omitempty"`
	Containers                    []KubernetesPodContainerFrontend `json:"containers"`
}

func (p KubernetesPodFrontend) NormalizeCollections() KubernetesPodFrontend {
	if p.Labels == nil {
		p.Labels = map[string]string{}
	}
	if p.Containers == nil {
		p.Containers = []KubernetesPodContainerFrontend{}
	}
	return p
}

type KubernetesPodContainerFrontend struct {
	Name         string `json:"name"`
	Image        string `json:"image,omitempty"`
	Ready        bool   `json:"ready"`
	RestartCount int32  `json:"restartCount,omitempty"`
	State        string `json:"state,omitempty"`
	Reason       string `json:"reason,omitempty"`
	Message      string `json:"message,omitempty"`
}

type KubernetesDeploymentFrontend struct {
	UID               string            `json:"uid"`
	Name              string            `json:"name"`
	Namespace         string            `json:"namespace"`
	DesiredReplicas   int32             `json:"desiredReplicas,omitempty"`
	UpdatedReplicas   int32             `json:"updatedReplicas,omitempty"`
	ReadyReplicas     int32             `json:"readyReplicas,omitempty"`
	AvailableReplicas int32             `json:"availableReplicas,omitempty"`
	Labels            map[string]string `json:"labels"`
}

func (d KubernetesDeploymentFrontend) NormalizeCollections() KubernetesDeploymentFrontend {
	if d.Labels == nil {
		d.Labels = map[string]string{}
	}
	return d
}

// DockerContainerFrontend represents a Docker container for the frontend
type DockerContainerFrontend struct {
	ID                  string                               `json:"id"`
	Name                string                               `json:"name"`
	Image               string                               `json:"image"`
	State               string                               `json:"state"`
	Status              string                               `json:"status"`
	Health              string                               `json:"health,omitempty"`
	CPUPercent          float64                              `json:"cpuPercent"`
	MemoryUsage         int64                                `json:"memoryUsageBytes"`
	MemoryLimit         int64                                `json:"memoryLimitBytes"`
	MemoryPercent       float64                              `json:"memoryPercent"`
	UptimeSeconds       int64                                `json:"uptimeSeconds"`
	RestartCount        int                                  `json:"restartCount"`
	ExitCode            int                                  `json:"exitCode"`
	CreatedAt           int64                                `json:"createdAt"`
	StartedAt           *int64                               `json:"startedAt,omitempty"`
	FinishedAt          *int64                               `json:"finishedAt,omitempty"`
	Ports               []DockerContainerPortFrontend        `json:"ports"`
	Labels              map[string]string                    `json:"labels"`
	Networks            []DockerContainerNetworkFrontend     `json:"networks"`
	WritableLayerBytes  int64                                `json:"writableLayerBytes,omitempty"`
	RootFilesystemBytes int64                                `json:"rootFilesystemBytes,omitempty"`
	BlockIO             *DockerContainerBlockIOFrontend      `json:"blockIo,omitempty"`
	Mounts              []DockerContainerMountFrontend       `json:"mounts"`
	Podman              *DockerPodmanContainerFrontend       `json:"podman,omitempty"`
	UpdateStatus        *DockerContainerUpdateStatusFrontend `json:"updateStatus,omitempty"`
}

func (c DockerContainerFrontend) NormalizeCollections() DockerContainerFrontend {
	if c.Ports == nil {
		c.Ports = []DockerContainerPortFrontend{}
	}
	if c.Labels == nil {
		c.Labels = map[string]string{}
	}
	if c.Networks == nil {
		c.Networks = []DockerContainerNetworkFrontend{}
	}
	if c.Mounts == nil {
		c.Mounts = []DockerContainerMountFrontend{}
	}
	return c
}

// DockerContainerUpdateStatusFrontend tracks the image update status for a container.
type DockerContainerUpdateStatusFrontend struct {
	UpdateAvailable bool   `json:"updateAvailable"`
	CurrentDigest   string `json:"currentDigest,omitempty"`
	LatestDigest    string `json:"latestDigest,omitempty"`
	LastChecked     int64  `json:"lastChecked"`
	Error           string `json:"error,omitempty"` // e.g., "rate limited", "auth required"
}

// DockerContainerPortFrontend represents a container port mapping
type DockerContainerPortFrontend struct {
	PrivatePort int    `json:"privatePort"`
	PublicPort  int    `json:"publicPort,omitempty"`
	Protocol    string `json:"protocol"`
	IP          string `json:"ip,omitempty"`
}

// DockerContainerNetworkFrontend represents container network addresses
type DockerContainerNetworkFrontend struct {
	Name string `json:"name"`
	IPv4 string `json:"ipv4,omitempty"`
	IPv6 string `json:"ipv6,omitempty"`
}

// DockerContainerBlockIOFrontend exposes aggregate block IO counters.
type DockerContainerBlockIOFrontend struct {
	ReadBytes               uint64   `json:"readBytes,omitempty"`
	WriteBytes              uint64   `json:"writeBytes,omitempty"`
	ReadRateBytesPerSecond  *float64 `json:"readRateBytesPerSecond,omitempty"`
	WriteRateBytesPerSecond *float64 `json:"writeRateBytesPerSecond,omitempty"`
}

// DockerContainerMountFrontend represents a container mount for the UI.
type DockerContainerMountFrontend struct {
	Type        string `json:"type,omitempty"`
	Source      string `json:"source,omitempty"`
	Destination string `json:"destination,omitempty"`
	Mode        string `json:"mode,omitempty"`
	RW          bool   `json:"rw"`
	Propagation string `json:"propagation,omitempty"`
	Name        string `json:"name,omitempty"`
	Driver      string `json:"driver,omitempty"`
}

// DockerPodmanContainerFrontend exposes podman-specific metadata.
type DockerPodmanContainerFrontend struct {
	PodName           string `json:"podName,omitempty"`
	PodID             string `json:"podId,omitempty"`
	Infra             bool   `json:"infra,omitempty"`
	ComposeProject    string `json:"composeProject,omitempty"`
	ComposeService    string `json:"composeService,omitempty"`
	ComposeWorkdir    string `json:"composeWorkdir,omitempty"`
	ComposeConfigHash string `json:"composeConfigHash,omitempty"`
	AutoUpdatePolicy  string `json:"autoUpdatePolicy,omitempty"`
	AutoUpdateRestart string `json:"autoUpdateRestart,omitempty"`
	UserNamespace     string `json:"userNamespace,omitempty"`
}

// DockerServiceFrontend represents a Swarm service for the frontend.
type DockerServiceFrontend struct {
	ID             string                       `json:"id"`
	Name           string                       `json:"name"`
	Stack          string                       `json:"stack,omitempty"`
	Image          string                       `json:"image,omitempty"`
	Mode           string                       `json:"mode,omitempty"`
	DesiredTasks   int                          `json:"desiredTasks,omitempty"`
	RunningTasks   int                          `json:"runningTasks,omitempty"`
	CompletedTasks int                          `json:"completedTasks,omitempty"`
	Labels         map[string]string            `json:"labels"`
	EndpointPorts  []DockerServicePortFrontend  `json:"endpointPorts"`
	UpdateStatus   *DockerServiceUpdateFrontend `json:"updateStatus,omitempty"`
	CreatedAt      *int64                       `json:"createdAt,omitempty"`
	UpdatedAt      *int64                       `json:"updatedAt,omitempty"`
}

func (s DockerServiceFrontend) NormalizeCollections() DockerServiceFrontend {
	if s.Labels == nil {
		s.Labels = map[string]string{}
	}
	if s.EndpointPorts == nil {
		s.EndpointPorts = []DockerServicePortFrontend{}
	}
	return s
}

// DockerServicePortFrontend represents a published service port.
type DockerServicePortFrontend struct {
	Name          string `json:"name,omitempty"`
	Protocol      string `json:"protocol,omitempty"`
	TargetPort    uint32 `json:"targetPort,omitempty"`
	PublishedPort uint32 `json:"publishedPort,omitempty"`
	PublishMode   string `json:"publishMode,omitempty"`
}

// DockerServiceUpdateFrontend exposes service update status to the UI.
type DockerServiceUpdateFrontend struct {
	State       string `json:"state,omitempty"`
	Message     string `json:"message,omitempty"`
	CompletedAt *int64 `json:"completedAt,omitempty"`
}

// DockerTaskFrontend represents a Swarm task replica.
type DockerTaskFrontend struct {
	ID            string `json:"id"`
	ServiceID     string `json:"serviceId,omitempty"`
	ServiceName   string `json:"serviceName,omitempty"`
	Slot          int    `json:"slot,omitempty"`
	NodeID        string `json:"nodeId,omitempty"`
	NodeName      string `json:"nodeName,omitempty"`
	DesiredState  string `json:"desiredState,omitempty"`
	CurrentState  string `json:"currentState,omitempty"`
	Error         string `json:"error,omitempty"`
	Message       string `json:"message,omitempty"`
	ContainerID   string `json:"containerId,omitempty"`
	ContainerName string `json:"containerName,omitempty"`
	CreatedAt     *int64 `json:"createdAt,omitempty"`
	UpdatedAt     *int64 `json:"updatedAt,omitempty"`
	StartedAt     *int64 `json:"startedAt,omitempty"`
	CompletedAt   *int64 `json:"completedAt,omitempty"`
}

// DockerSwarmFrontend summarises node-level swarm details.
type DockerSwarmFrontend struct {
	NodeID           string `json:"nodeId,omitempty"`
	NodeRole         string `json:"nodeRole,omitempty"`
	LocalState       string `json:"localState,omitempty"`
	ControlAvailable bool   `json:"controlAvailable,omitempty"`
	ClusterID        string `json:"clusterId,omitempty"`
	ClusterName      string `json:"clusterName,omitempty"`
	Scope            string `json:"scope,omitempty"`
	Error            string `json:"error,omitempty"`
}

// DockerHostCommandFrontend exposes docker host command state to the UI.
type DockerHostCommandFrontend struct {
	ID             string `json:"id"`
	Type           string `json:"type"`
	Status         string `json:"status"`
	Message        string `json:"message,omitempty"`
	CreatedAt      int64  `json:"createdAt"`
	UpdatedAt      int64  `json:"updatedAt"`
	DispatchedAt   *int64 `json:"dispatchedAt,omitempty"`
	AcknowledgedAt *int64 `json:"acknowledgedAt,omitempty"`
	CompletedAt    *int64 `json:"completedAt,omitempty"`
	FailedAt       *int64 `json:"failedAt,omitempty"`
	FailureReason  string `json:"failureReason,omitempty"`
	ExpiresAt      *int64 `json:"expiresAt,omitempty"`
}

// HostFrontend represents a generic infrastructure host exposed to the UI.
type HostFrontend struct {
	ID                string                     `json:"id"`
	Hostname          string                     `json:"hostname"`
	DisplayName       string                     `json:"displayName"`
	Platform          string                     `json:"platform,omitempty"`
	OSName            string                     `json:"osName,omitempty"`
	OSVersion         string                     `json:"osVersion,omitempty"`
	KernelVersion     string                     `json:"kernelVersion,omitempty"`
	Architecture      string                     `json:"architecture,omitempty"`
	CPUCount          int                        `json:"cpuCount,omitempty"`
	CPUUsage          float64                    `json:"cpuUsage,omitempty"`
	LoadAverage       []float64                  `json:"loadAverage"`
	Memory            *Memory                    `json:"memory,omitempty"`
	Disks             []Disk                     `json:"disks"`
	DiskIO            []DiskIO                   `json:"diskIO"`
	NetworkInterfaces []HostNetworkInterface     `json:"networkInterfaces"`
	Sensors           *HostSensorSummaryFrontend `json:"sensors,omitempty"`
	Status            string                     `json:"status"`
	UptimeSeconds     int64                      `json:"uptimeSeconds,omitempty"`
	LastSeen          int64                      `json:"lastSeen"`
	IntervalSeconds   int                        `json:"intervalSeconds,omitempty"`
	AgentVersion      string                     `json:"agentVersion,omitempty"`
	MachineID         string                     `json:"machineId,omitempty"`
	TokenID           string                     `json:"tokenId,omitempty"`
	TokenName         string                     `json:"tokenName,omitempty"`
	TokenHint         string                     `json:"tokenHint,omitempty"`
	TokenLastUsedAt   *int64                     `json:"tokenLastUsedAt,omitempty"`
	Tags              []string                   `json:"tags"`
	CommandsEnabled   bool                       `json:"commandsEnabled,omitempty"` // Whether AI command execution is enabled
	IsLegacy          bool                       `json:"isLegacy,omitempty"`        // True if using legacy agent protocol
	LinkedNodeID      string                     `json:"linkedNodeId,omitempty"`    // ID of linked PVE node (if running on a node)
}

func (h HostFrontend) NormalizeCollections() HostFrontend {
	if h.LoadAverage == nil {
		h.LoadAverage = []float64{}
	}
	if h.Disks == nil {
		h.Disks = []Disk{}
	}
	if h.DiskIO == nil {
		h.DiskIO = []DiskIO{}
	}
	if h.NetworkInterfaces == nil {
		h.NetworkInterfaces = []HostNetworkInterface{}
	}
	if h.Tags == nil {
		h.Tags = []string{}
	}
	if h.Sensors != nil {
		sensors := h.Sensors.NormalizeCollections()
		h.Sensors = &sensors
	}
	return h
}

// HostSensorSummaryFrontend mirrors HostSensorSummary with primitives for the frontend.
type HostSensorSummaryFrontend struct {
	TemperatureCelsius map[string]float64      `json:"temperatureCelsius"`
	FanRPM             map[string]float64      `json:"fanRpm"`
	Additional         map[string]float64      `json:"additional"`
	SMART              []HostDiskSMARTFrontend `json:"smart"` // S.M.A.R.T. disk data
}

func (s HostSensorSummaryFrontend) NormalizeCollections() HostSensorSummaryFrontend {
	if s.TemperatureCelsius == nil {
		s.TemperatureCelsius = map[string]float64{}
	}
	if s.FanRPM == nil {
		s.FanRPM = map[string]float64{}
	}
	if s.Additional == nil {
		s.Additional = map[string]float64{}
	}
	if s.SMART == nil {
		s.SMART = []HostDiskSMARTFrontend{}
	}
	return s
}

// HostDiskSMARTFrontend represents S.M.A.R.T. data for a disk from a host agent.
type HostDiskSMARTFrontend struct {
	Device      string           `json:"device"`            // Device name (e.g., sda)
	Model       string           `json:"model,omitempty"`   // Disk model
	Serial      string           `json:"serial,omitempty"`  // Serial number
	WWN         string           `json:"wwn,omitempty"`     // World Wide Name
	Type        string           `json:"type,omitempty"`    // Transport type: sata, sas, nvme
	Temperature int              `json:"temperature"`       // Temperature in Celsius
	Health      string           `json:"health,omitempty"`  // PASSED, FAILED, UNKNOWN
	Standby     bool             `json:"standby,omitempty"` // True if disk was in standby
	Attributes  *SMARTAttributes `json:"attributes,omitempty"`
}

// StorageFrontend represents Storage with frontend-friendly field names
type StorageFrontend struct {
	ID        string   `json:"id"`
	Storage   string   `json:"storage"` // Maps to Name
	Name      string   `json:"name"`
	Node      string   `json:"node"`
	Instance  string   `json:"instance"`
	Nodes     []string `json:"nodes"`
	NodeIDs   []string `json:"nodeIds"`
	NodeCount int      `json:"nodeCount,omitempty"`
	Type      string   `json:"type"`
	Status    string   `json:"status"`
	Total     int64    `json:"total"`
	Used      int64    `json:"used"`
	Avail     int64    `json:"avail"` // Maps to Free
	Free      int64    `json:"free"`
	Usage     float64  `json:"usage"`
	Content   string   `json:"content"`
	Shared    bool     `json:"shared"`
	Enabled   bool     `json:"enabled"`
	Active    bool     `json:"active"`
}

func (s StorageFrontend) NormalizeCollections() StorageFrontend {
	if s.Nodes == nil {
		s.Nodes = []string{}
	}
	if s.NodeIDs == nil {
		s.NodeIDs = []string{}
	}
	return s
}

// CephClusterFrontend represents a Ceph cluster with frontend-friendly field names
type CephClusterFrontend struct {
	ID             string              `json:"id"`
	Instance       string              `json:"instance"`
	Name           string              `json:"name"`
	FSID           string              `json:"fsid,omitempty"`
	Health         string              `json:"health"`
	HealthMessage  string              `json:"healthMessage,omitempty"`
	TotalBytes     int64               `json:"totalBytes"`
	UsedBytes      int64               `json:"usedBytes"`
	AvailableBytes int64               `json:"availableBytes"`
	UsagePercent   float64             `json:"usagePercent"`
	NumMons        int                 `json:"numMons"`
	NumMgrs        int                 `json:"numMgrs"`
	NumOSDs        int                 `json:"numOsds"`
	NumOSDsUp      int                 `json:"numOsdsUp"`
	NumOSDsIn      int                 `json:"numOsdsIn"`
	NumPGs         int                 `json:"numPGs"`
	Pools          []CephPool          `json:"pools"`
	Services       []CephServiceStatus `json:"services"`
	LastUpdated    int64               `json:"lastUpdated"`
}

func (c CephClusterFrontend) NormalizeCollections() CephClusterFrontend {
	if c.Pools == nil {
		c.Pools = []CephPool{}
	}
	if c.Services == nil {
		c.Services = []CephServiceStatus{}
	}
	return c
}

// ReplicationJobFrontend represents a replication job for the frontend.
type ReplicationJobFrontend struct {
	ID                      string   `json:"id"`
	Instance                string   `json:"instance"`
	JobID                   string   `json:"jobId"`
	JobNumber               int      `json:"jobNumber,omitempty"`
	Guest                   string   `json:"guest,omitempty"`
	GuestID                 int      `json:"guestId,omitempty"`
	GuestName               string   `json:"guestName,omitempty"`
	GuestType               string   `json:"guestType,omitempty"`
	GuestNode               string   `json:"guestNode,omitempty"`
	SourceNode              string   `json:"sourceNode,omitempty"`
	SourceStorage           string   `json:"sourceStorage,omitempty"`
	TargetNode              string   `json:"targetNode,omitempty"`
	TargetStorage           string   `json:"targetStorage,omitempty"`
	Schedule                string   `json:"schedule,omitempty"`
	Type                    string   `json:"type,omitempty"`
	Enabled                 bool     `json:"enabled"`
	State                   string   `json:"state,omitempty"`
	Status                  string   `json:"status,omitempty"`
	LastSyncStatus          string   `json:"lastSyncStatus,omitempty"`
	LastSyncTime            int64    `json:"lastSyncTime,omitempty"`
	LastSyncUnix            int64    `json:"lastSyncUnix,omitempty"`
	LastSyncDurationSeconds int      `json:"lastSyncDurationSeconds,omitempty"`
	LastSyncDurationHuman   string   `json:"lastSyncDurationHuman,omitempty"`
	NextSyncTime            int64    `json:"nextSyncTime,omitempty"`
	NextSyncUnix            int64    `json:"nextSyncUnix,omitempty"`
	DurationSeconds         int      `json:"durationSeconds,omitempty"`
	DurationHuman           string   `json:"durationHuman,omitempty"`
	FailCount               int      `json:"failCount,omitempty"`
	Error                   string   `json:"error,omitempty"`
	Comment                 string   `json:"comment,omitempty"`
	RemoveJob               string   `json:"removeJob,omitempty"`
	RateLimitMbps           *float64 `json:"rateLimitMbps,omitempty"`
	PolledAt                int64    `json:"polledAt,omitempty"`
}

// StateFrontend represents the state with frontend-friendly field names
type StateFrontend struct {
	ActiveAlerts                 []Alert         `json:"activeAlerts"`                 // Active alerts
	RecentlyResolved             []ResolvedAlert `json:"recentlyResolved"`             // Recently resolved alerts
	Metrics                      []Metric        `json:"metrics"`                      // Time-series metrics
	Performance                  Performance     `json:"performance"`                  // Polling/runtime performance
	ConnectionHealth             map[string]bool `json:"connectionHealth"`             // Keep as is
	Stats                        Stats           `json:"stats"`                        // Runtime statistics
	LastUpdate                   int64           `json:"lastUpdate"`                   // Unix timestamp
	TemperatureMonitoringEnabled bool            `json:"temperatureMonitoringEnabled"` // Global temperature monitoring setting
	// Unified resources - the new way to access all monitored entities
	Resources               []ResourceFrontend                    `json:"resources"`
	ConnectedInfrastructure []ConnectedInfrastructureItemFrontend `json:"connectedInfrastructure"`
}

// EmptyStateFrontend returns a canonical empty frontend state with stable
// collection/object semantics for API and websocket consumers.
func EmptyStateFrontend() StateFrontend {
	return StateFrontend{}.NormalizeCollections()
}

// NormalizeCollections ensures StateFrontend preserves stable empty collection
// semantics instead of leaking nil slices or maps to clients.
func (s StateFrontend) NormalizeCollections() StateFrontend {
	if s.ActiveAlerts == nil {
		s.ActiveAlerts = []Alert{}
	}
	if s.RecentlyResolved == nil {
		s.RecentlyResolved = []ResolvedAlert{}
	}
	if s.Metrics == nil {
		s.Metrics = []Metric{}
	}
	if s.ConnectionHealth == nil {
		s.ConnectionHealth = map[string]bool{}
	}
	if s.Performance.APICallDuration == nil {
		s.Performance.APICallDuration = map[string]float64{}
	}
	if s.Resources == nil {
		s.Resources = []ResourceFrontend{}
	}
	if s.ConnectedInfrastructure == nil {
		s.ConnectedInfrastructure = []ConnectedInfrastructureItemFrontend{}
	}
	for i := range s.ConnectedInfrastructure {
		s.ConnectedInfrastructure[i] = s.ConnectedInfrastructure[i].NormalizeCollections()
	}
	for i := range s.Resources {
		s.Resources[i] = s.Resources[i].NormalizeCollections()
	}
	return s
}

// ResourceFrontend is the frontend representation of a unified Resource.
// This mirrors resources.Resource but with time.Time converted to Unix milliseconds.
type ResourceFrontend struct {
	// Identity
	ID          string `json:"id"`
	Type        string `json:"type"`
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`

	// Platform/Source
	PlatformID   string `json:"platformId"`
	PlatformType string `json:"platformType"`
	SourceType   string `json:"sourceType"`

	// Hierarchy
	ParentID  string `json:"parentId,omitempty"`
	ClusterID string `json:"clusterId,omitempty"`

	// Universal Metrics
	Status      string                   `json:"status"`
	CPU         *ResourceMetricFrontend  `json:"cpu,omitempty"`
	Memory      *ResourceMetricFrontend  `json:"memory,omitempty"`
	Disk        *ResourceMetricFrontend  `json:"disk,omitempty"`
	Network     *ResourceNetworkFrontend `json:"network,omitempty"`
	Temperature *float64                 `json:"temperature,omitempty"`
	Uptime      *int64                   `json:"uptime,omitempty"`

	// Metadata
	Tags     []string                `json:"tags"`
	Labels   map[string]string       `json:"labels"`
	LastSeen int64                   `json:"lastSeen"` // Unix milliseconds
	Alerts   []ResourceAlertFrontend `json:"alerts"`

	// Identity for deduplication
	Identity *ResourceIdentityFrontend `json:"identity,omitempty"`

	// Platform-specific data (JSON blob)
	PlatformData json.RawMessage `json:"platformData,omitempty"`
}

func (r ResourceFrontend) NormalizeCollections() ResourceFrontend {
	if r.Tags == nil {
		r.Tags = []string{}
	}
	if r.Labels == nil {
		r.Labels = map[string]string{}
	}
	if r.Alerts == nil {
		r.Alerts = []ResourceAlertFrontend{}
	}
	if r.Identity != nil {
		identity := r.Identity.NormalizeCollections()
		r.Identity = &identity
	}
	return r
}

// ResourceMetricFrontend represents a metric value for the frontend.
type ResourceMetricFrontend struct {
	Current float64 `json:"current"`
	Total   *int64  `json:"total,omitempty"`
	Used    *int64  `json:"used,omitempty"`
	Free    *int64  `json:"free,omitempty"`
}

// ResourceNetworkFrontend represents network metrics for the frontend.
type ResourceNetworkFrontend struct {
	RXBytes int64 `json:"rxBytes"`
	TXBytes int64 `json:"txBytes"`
}

// ResourceAlertFrontend represents an alert on a resource.
type ResourceAlertFrontend struct {
	ID        string  `json:"id"`
	Type      string  `json:"type"`
	Level     string  `json:"level"`
	Message   string  `json:"message"`
	Value     float64 `json:"value"`
	Threshold float64 `json:"threshold"`
	StartTime int64   `json:"startTime"` // Unix milliseconds
}

// ResourceIdentityFrontend contains identity info for deduplication.
type ResourceIdentityFrontend struct {
	Hostname  string   `json:"hostname,omitempty"`
	MachineID string   `json:"machineId,omitempty"`
	IPs       []string `json:"ips"`
}

func (i ResourceIdentityFrontend) NormalizeCollections() ResourceIdentityFrontend {
	if i.IPs == nil {
		i.IPs = []string{}
	}
	return i
}
