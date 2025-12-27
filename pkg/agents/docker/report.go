package dockeragent

import (
	"time"

	hostagent "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

type MemoryMetric = hostagent.MemoryMetric
type Disk = hostagent.Disk
type NetworkInterface = hostagent.NetworkInterface

// Report represents a single heartbeat from the Docker agent to Pulse.
type Report struct {
	Agent      AgentInfo   `json:"agent"`
	Host       HostInfo    `json:"host"`
	Containers []Container `json:"containers"`
	Services   []Service   `json:"services,omitempty"`
	Tasks      []Task      `json:"tasks,omitempty"`
	Timestamp  time.Time   `json:"timestamp"`
}

// AgentInfo describes the reporting agent instance.
type AgentInfo struct {
	ID              string `json:"id"`
	Version         string `json:"version"`
	Type            string `json:"type,omitempty"` // "unified", "host", or "docker" - empty means legacy
	IntervalSeconds int    `json:"intervalSeconds"`
}

// HostInfo contains metadata about the Docker host where the agent runs.
type HostInfo struct {
	Hostname         string             `json:"hostname"`
	Name             string             `json:"name,omitempty"`
	MachineID        string             `json:"machineId,omitempty"`
	OS               string             `json:"os,omitempty"`
	Runtime          string             `json:"runtime,omitempty"`
	RuntimeVersion   string             `json:"runtimeVersion,omitempty"`
	KernelVersion    string             `json:"kernelVersion,omitempty"`
	Architecture     string             `json:"architecture,omitempty"`
	DockerVersion    string             `json:"dockerVersion,omitempty"`
	TotalCPU         int                `json:"totalCpu,omitempty"`
	TotalMemoryBytes int64              `json:"totalMemoryBytes,omitempty"`
	UptimeSeconds    int64              `json:"uptimeSeconds,omitempty"`
	Swarm            *SwarmInfo         `json:"swarm,omitempty"`
	CPUUsagePercent  float64            `json:"cpuUsagePercent,omitempty"`
	LoadAverage      []float64          `json:"loadAverage,omitempty"`
	Memory           MemoryMetric       `json:"memory,omitempty"`
	Disks            []Disk             `json:"disks,omitempty"`
	Network          []NetworkInterface `json:"network,omitempty"`
}

// Container captures the runtime state for a Docker container at report time.
type Container struct {
	ID                  string             `json:"id"`
	Name                string             `json:"name"`
	Image               string             `json:"image"`
	ImageDigest         string             `json:"imageDigest,omitempty"` // Current image digest for update detection
	CreatedAt           time.Time          `json:"createdAt"`
	State               string             `json:"state"`
	Status              string             `json:"status"`
	Health              string             `json:"health,omitempty"`
	CPUPercent          float64            `json:"cpuPercent"`
	MemoryUsageBytes    int64              `json:"memoryUsageBytes"`
	MemoryLimitBytes    int64              `json:"memoryLimitBytes"`
	MemoryPercent       float64            `json:"memoryPercent"`
	UptimeSeconds       int64              `json:"uptimeSeconds"`
	RestartCount        int                `json:"restartCount"`
	ExitCode            int                `json:"exitCode"`
	StartedAt           *time.Time         `json:"startedAt,omitempty"`
	FinishedAt          *time.Time         `json:"finishedAt,omitempty"`
	Ports               []ContainerPort    `json:"ports,omitempty"`
	Labels              map[string]string  `json:"labels,omitempty"`
	Env                 []string           `json:"env,omitempty"`
	Networks            []ContainerNetwork `json:"networks,omitempty"`
	WritableLayerBytes  int64              `json:"writableLayerBytes,omitempty"`
	RootFilesystemBytes int64              `json:"rootFilesystemBytes,omitempty"`
	BlockIO             *ContainerBlockIO  `json:"blockIo,omitempty"`
	Mounts              []ContainerMount   `json:"mounts,omitempty"`
	Podman              *PodmanContainer   `json:"podman,omitempty"`
	UpdateStatus        *UpdateStatus      `json:"updateStatus,omitempty"` // Image update detection status
}

// ContainerPort tracks an exposed container port mapping.
type ContainerPort struct {
	PrivatePort int    `json:"privatePort"`
	PublicPort  int    `json:"publicPort,omitempty"`
	Protocol    string `json:"protocol"`
	IP          string `json:"ip,omitempty"`
}

// ContainerNetwork summarises container network addresses by network name.
type ContainerNetwork struct {
	Name string `json:"name"`
	IPv4 string `json:"ipv4,omitempty"`
	IPv6 string `json:"ipv6,omitempty"`
}

// ContainerBlockIO summarises high-level block I/O metrics for a container.
type ContainerBlockIO struct {
	ReadBytes  uint64 `json:"readBytes,omitempty"`
	WriteBytes uint64 `json:"writeBytes,omitempty"`
}

// PodmanContainer carries metadata extracted from Podman-specific annotations.
type PodmanContainer struct {
	PodName           string `json:"podName,omitempty"`
	PodID             string `json:"podId,omitempty"`
	Infra             bool   `json:"infra,omitempty"`
	ComposeProject    string `json:"composeProject,omitempty"`
	ComposeService    string `json:"composeService,omitempty"`
	ComposeWorkdir    string `json:"composeWorkdir,omitempty"`
	ComposeConfig     string `json:"composeConfigHash,omitempty"`
	AutoUpdatePolicy  string `json:"autoUpdatePolicy,omitempty"`
	AutoUpdateRestart string `json:"autoUpdateRestart,omitempty"`
	UserNS            string `json:"userNamespace,omitempty"`
}

// ContainerMount describes a mount point exposed inside a container.
type ContainerMount struct {
	Type        string `json:"type,omitempty"`
	Source      string `json:"source,omitempty"`
	Destination string `json:"destination,omitempty"`
	Mode        string `json:"mode,omitempty"`
	RW          bool   `json:"rw"`
	Propagation string `json:"propagation,omitempty"`
	Name        string `json:"name,omitempty"`
	Driver      string `json:"driver,omitempty"`
}

// UpdateStatus tracks the image update status for a container.
type UpdateStatus struct {
	UpdateAvailable bool      `json:"updateAvailable"`
	CurrentDigest   string    `json:"currentDigest,omitempty"`
	LatestDigest    string    `json:"latestDigest,omitempty"`
	LastChecked     time.Time `json:"lastChecked"`
	Error           string    `json:"error,omitempty"` // e.g., "rate limited", "auth required"
}

// AgentKey returns the stable identifier for a reporting agent.
func (r Report) AgentKey() string {
	if r.Agent.ID != "" {
		return r.Agent.ID
	}
	if r.Host.MachineID != "" {
		return r.Host.MachineID
	}
	return r.Host.Hostname
}

// SwarmInfo captures metadata about the Docker Swarm state for the reporting node.
type SwarmInfo struct {
	NodeID           string `json:"nodeId,omitempty"`
	NodeRole         string `json:"nodeRole,omitempty"`
	LocalState       string `json:"localState,omitempty"`
	ControlAvailable bool   `json:"controlAvailable,omitempty"`
	ClusterID        string `json:"clusterId,omitempty"`
	ClusterName      string `json:"clusterName,omitempty"`
	Scope            string `json:"scope,omitempty"`
	Error            string `json:"error,omitempty"`
}

// Service summarises a Docker Swarm service and its aggregate status.
type Service struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Stack          string            `json:"stack,omitempty"`
	Image          string            `json:"image,omitempty"`
	Mode           string            `json:"mode,omitempty"`
	DesiredTasks   int               `json:"desiredTasks,omitempty"`
	RunningTasks   int               `json:"runningTasks,omitempty"`
	CompletedTasks int               `json:"completedTasks,omitempty"`
	Labels         map[string]string `json:"labels,omitempty"`
	EndpointPorts  []ServicePort     `json:"endpointPorts,omitempty"`
	UpdateStatus   *ServiceUpdate    `json:"updateStatus,omitempty"`
	CreatedAt      *time.Time        `json:"createdAt,omitempty"`
	UpdatedAt      *time.Time        `json:"updatedAt,omitempty"`
}

// ServicePort describes an exposed service endpoint.
type ServicePort struct {
	Name          string `json:"name,omitempty"`
	Protocol      string `json:"protocol,omitempty"`
	TargetPort    uint32 `json:"targetPort,omitempty"`
	PublishedPort uint32 `json:"publishedPort,omitempty"`
	PublishMode   string `json:"publishMode,omitempty"`
}

// ServiceUpdate captures the current rolling update status for a service.
type ServiceUpdate struct {
	State       string     `json:"state,omitempty"`
	Message     string     `json:"message,omitempty"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
}

// Task summarises an individual Docker Swarm task (replica).
type Task struct {
	ID            string     `json:"id"`
	ServiceID     string     `json:"serviceId,omitempty"`
	ServiceName   string     `json:"serviceName,omitempty"`
	Slot          int        `json:"slot,omitempty"`
	NodeID        string     `json:"nodeId,omitempty"`
	NodeName      string     `json:"nodeName,omitempty"`
	DesiredState  string     `json:"desiredState,omitempty"`
	CurrentState  string     `json:"currentState,omitempty"`
	Error         string     `json:"error,omitempty"`
	Message       string     `json:"message,omitempty"`
	ContainerID   string     `json:"containerId,omitempty"`
	ContainerName string     `json:"containerName,omitempty"`
	CreatedAt     time.Time  `json:"createdAt,omitempty"`
	UpdatedAt     *time.Time `json:"updatedAt,omitempty"`
	StartedAt     *time.Time `json:"startedAt,omitempty"`
	CompletedAt   *time.Time `json:"completedAt,omitempty"`
}
