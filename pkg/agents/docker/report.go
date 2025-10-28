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
	IntervalSeconds int    `json:"intervalSeconds"`
}

// HostInfo contains metadata about the Docker host where the agent runs.
type HostInfo struct {
	Hostname         string             `json:"hostname"`
	Name             string             `json:"name,omitempty"`
	MachineID        string             `json:"machineId,omitempty"`
	OS               string             `json:"os,omitempty"`
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
	ID               string             `json:"id"`
	Name             string             `json:"name"`
	Image            string             `json:"image"`
	CreatedAt        time.Time          `json:"createdAt"`
	State            string             `json:"state"`
	Status           string             `json:"status"`
	Health           string             `json:"health,omitempty"`
	CPUPercent       float64            `json:"cpuPercent"`
	MemoryUsageBytes int64              `json:"memoryUsageBytes"`
	MemoryLimitBytes int64              `json:"memoryLimitBytes"`
	MemoryPercent    float64            `json:"memoryPercent"`
	UptimeSeconds    int64              `json:"uptimeSeconds"`
	RestartCount     int                `json:"restartCount"`
	ExitCode         int                `json:"exitCode"`
	StartedAt        *time.Time         `json:"startedAt,omitempty"`
	FinishedAt       *time.Time         `json:"finishedAt,omitempty"`
	Ports            []ContainerPort    `json:"ports,omitempty"`
	Labels           map[string]string  `json:"labels,omitempty"`
	Networks         []ContainerNetwork `json:"networks,omitempty"`
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
