package dockeragent

import "time"

// Report represents a single heartbeat from the Docker agent to Pulse.
type Report struct {
	Agent      AgentInfo   `json:"agent"`
	Host       HostInfo    `json:"host"`
	Containers []Container `json:"containers"`
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
	Hostname         string `json:"hostname"`
	Name             string `json:"name,omitempty"`
	MachineID        string `json:"machineId,omitempty"`
	OS               string `json:"os,omitempty"`
	KernelVersion    string `json:"kernelVersion,omitempty"`
	Architecture     string `json:"architecture,omitempty"`
	DockerVersion    string `json:"dockerVersion,omitempty"`
	TotalCPU         int    `json:"totalCpu,omitempty"`
	TotalMemoryBytes int64  `json:"totalMemoryBytes,omitempty"`
	UptimeSeconds    int64  `json:"uptimeSeconds,omitempty"`
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
