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
	LinkedHostAgentId            string       `json:"linkedHostAgentId,omitempty"`            // ID of linked host agent (for "Via agent" badge)
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
	Disks             []Disk                  `json:"disks,omitempty"`            // Individual filesystem/disk usage
	DiskStatusReason  string                  `json:"diskStatusReason,omitempty"` // Why disk stats are unavailable
	OSName            string                  `json:"osName,omitempty"`
	OSVersion         string                  `json:"osVersion,omitempty"`
	AgentVersion      string                  `json:"agentVersion,omitempty"`
	NetworkInterfaces []GuestNetworkInterface `json:"networkInterfaces,omitempty"`
	IPAddresses       []string                `json:"ipAddresses,omitempty"`
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
	Disks             []Disk                  `json:"disks,omitempty"`  // Individual filesystem/disk usage
	NetworkInterfaces []GuestNetworkInterface `json:"networkInterfaces,omitempty"`
	IPAddresses       []string                `json:"ipAddresses,omitempty"`
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
	LoadAverage       []float64                  `json:"loadAverage,omitempty"`
	Memory            *Memory                    `json:"memory,omitempty"`
	Disks             []Disk                     `json:"disks,omitempty"`
	NetworkInterfaces []HostNetworkInterface     `json:"networkInterfaces,omitempty"`
	Status            string                     `json:"status"`
	LastSeen          int64                      `json:"lastSeen"`
	IntervalSeconds   int                        `json:"intervalSeconds"`
	AgentVersion      string                     `json:"agentVersion,omitempty"`
	Containers        []DockerContainerFrontend  `json:"containers"`
	Services          []DockerServiceFrontend    `json:"services,omitempty"`
	Tasks             []DockerTaskFrontend       `json:"tasks,omitempty"`
	Swarm             *DockerSwarmFrontend       `json:"swarm,omitempty"`
	TokenID           string                     `json:"tokenId,omitempty"`
	TokenName         string                     `json:"tokenName,omitempty"`
	TokenHint         string                     `json:"tokenHint,omitempty"`
	TokenLastUsedAt   *int64                     `json:"tokenLastUsedAt,omitempty"`
	PendingUninstall  bool                       `json:"pendingUninstall"`
	Command           *DockerHostCommandFrontend `json:"command,omitempty"`
}

// RemovedDockerHostFrontend represents a blocked docker host entry for the frontend.
type RemovedDockerHostFrontend struct {
	ID          string `json:"id"`
	Hostname    string `json:"hostname,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
	RemovedAt   int64  `json:"removedAt"`
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

	Nodes       []KubernetesNodeFrontend       `json:"nodes,omitempty"`
	Pods        []KubernetesPodFrontend        `json:"pods,omitempty"`
	Deployments []KubernetesDeploymentFrontend `json:"deployments,omitempty"`
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
	Roles                   []string `json:"roles,omitempty"`
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
	Labels                        map[string]string                `json:"labels,omitempty"`
	OwnerKind                     string                           `json:"ownerKind,omitempty"`
	OwnerName                     string                           `json:"ownerName,omitempty"`
	Containers                    []KubernetesPodContainerFrontend `json:"containers,omitempty"`
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
	Labels            map[string]string `json:"labels,omitempty"`
}

// RemovedKubernetesClusterFrontend represents a blocked kubernetes cluster entry for the frontend.
type RemovedKubernetesClusterFrontend struct {
	ID          string `json:"id"`
	Name        string `json:"name,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
	RemovedAt   int64  `json:"removedAt"`
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
	Ports               []DockerContainerPortFrontend        `json:"ports,omitempty"`
	Labels              map[string]string                    `json:"labels,omitempty"`
	Networks            []DockerContainerNetworkFrontend     `json:"networks,omitempty"`
	WritableLayerBytes  int64                                `json:"writableLayerBytes,omitempty"`
	RootFilesystemBytes int64                                `json:"rootFilesystemBytes,omitempty"`
	BlockIO             *DockerContainerBlockIOFrontend      `json:"blockIo,omitempty"`
	Mounts              []DockerContainerMountFrontend       `json:"mounts,omitempty"`
	Podman              *DockerPodmanContainerFrontend       `json:"podman,omitempty"`
	UpdateStatus        *DockerContainerUpdateStatusFrontend `json:"updateStatus,omitempty"`
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
	Labels         map[string]string            `json:"labels,omitempty"`
	EndpointPorts  []DockerServicePortFrontend  `json:"endpointPorts,omitempty"`
	UpdateStatus   *DockerServiceUpdateFrontend `json:"updateStatus,omitempty"`
	CreatedAt      *int64                       `json:"createdAt,omitempty"`
	UpdatedAt      *int64                       `json:"updatedAt,omitempty"`
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
	LoadAverage       []float64                  `json:"loadAverage,omitempty"`
	Memory            *Memory                    `json:"memory,omitempty"`
	Disks             []Disk                     `json:"disks,omitempty"`
	DiskIO            []DiskIO                   `json:"diskIO,omitempty"`
	NetworkInterfaces []HostNetworkInterface     `json:"networkInterfaces,omitempty"`
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
	Tags              []string                   `json:"tags,omitempty"`
	CommandsEnabled   bool                       `json:"commandsEnabled,omitempty"` // Whether AI command execution is enabled
	IsLegacy          bool                       `json:"isLegacy,omitempty"`        // True if using legacy agent protocol
	LinkedNodeId      string                     `json:"linkedNodeId,omitempty"`    // ID of linked PVE node (if running on a node)
}

// HostSensorSummaryFrontend mirrors HostSensorSummary with primitives for the frontend.
type HostSensorSummaryFrontend struct {
	TemperatureCelsius map[string]float64      `json:"temperatureCelsius,omitempty"`
	FanRPM             map[string]float64      `json:"fanRpm,omitempty"`
	Additional         map[string]float64      `json:"additional,omitempty"`
	SMART              []HostDiskSMARTFrontend `json:"smart,omitempty"` // S.M.A.R.T. disk data
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
	Nodes     []string `json:"nodes,omitempty"`
	NodeIDs   []string `json:"nodeIds,omitempty"`
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
	Pools          []CephPool          `json:"pools,omitempty"`
	Services       []CephServiceStatus `json:"services,omitempty"`
	LastUpdated    int64               `json:"lastUpdated"`
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
	Nodes                        []NodeFrontend                     `json:"nodes"`
	VMs                          []VMFrontend                       `json:"vms"`
	Containers                   []ContainerFrontend                `json:"containers"`
	DockerHosts                  []DockerHostFrontend               `json:"dockerHosts"`
	RemovedDockerHosts           []RemovedDockerHostFrontend        `json:"removedDockerHosts"`
	KubernetesClusters           []KubernetesClusterFrontend        `json:"kubernetesClusters"`
	RemovedKubernetesClusters    []RemovedKubernetesClusterFrontend `json:"removedKubernetesClusters"`
	Hosts                        []HostFrontend                     `json:"hosts"`
	Storage                      []StorageFrontend                  `json:"storage"`
	CephClusters                 []CephClusterFrontend              `json:"cephClusters"`
	PhysicalDisks                []PhysicalDisk                     `json:"physicalDisks"`
	PBS                          []PBSInstance                      `json:"pbs"` // Keep as is
	PMG                          []PMGInstance                      `json:"pmg"`
	PBSBackups                   []PBSBackup                        `json:"pbsBackups"`
	PMGBackups                   []PMGBackup                        `json:"pmgBackups"`
	Backups                      Backups                            `json:"backups"`
	ReplicationJobs              []ReplicationJobFrontend           `json:"replicationJobs"`
	ActiveAlerts                 []Alert                            `json:"activeAlerts"`                 // Active alerts
	Metrics                      map[string]any                     `json:"metrics"`                      // Empty object for now
	PVEBackups                   PVEBackups                         `json:"pveBackups"`                   // Keep as is
	Performance                  map[string]any                     `json:"performance"`                  // Empty object for now
	ConnectionHealth             map[string]bool                    `json:"connectionHealth"`             // Keep as is
	Stats                        map[string]any                     `json:"stats"`                        // Empty object for now
	LastUpdate                   int64                              `json:"lastUpdate"`                   // Unix timestamp
	TemperatureMonitoringEnabled bool                               `json:"temperatureMonitoringEnabled"` // Global temperature monitoring setting
	// Unified resources - the new way to access all monitored entities
	Resources []ResourceFrontend `json:"resources,omitempty"`
}

// StripLegacyArrays removes legacy per-type arrays from the state payload.
// Use this when transitioning to unified Resources-only mode.
func (s *StateFrontend) StripLegacyArrays() {
	if s == nil {
		return
	}

	s.Nodes = nil
	s.VMs = nil
	s.Containers = nil
	s.DockerHosts = nil
	s.RemovedDockerHosts = nil
	s.Hosts = nil
	s.Storage = nil
	s.CephClusters = nil
	s.PhysicalDisks = nil
	// Keep PBS, PMG, and Backups fields while they are not fully migrated.
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
	Tags     []string                `json:"tags,omitempty"`
	Labels   map[string]string       `json:"labels,omitempty"`
	LastSeen int64                   `json:"lastSeen"` // Unix milliseconds
	Alerts   []ResourceAlertFrontend `json:"alerts,omitempty"`

	// Identity for deduplication
	Identity *ResourceIdentityFrontend `json:"identity,omitempty"`

	// Platform-specific data (JSON blob)
	PlatformData json.RawMessage `json:"platformData,omitempty"`
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
	IPs       []string `json:"ips,omitempty"`
}
