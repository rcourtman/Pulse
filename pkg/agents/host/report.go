package host

import "time"

// Report represents the payload sent by the pulse-host-agent.
type Report struct {
	Agent      AgentInfo          `json:"agent"`
	Host       HostInfo           `json:"host"`
	Metrics    Metrics            `json:"metrics"`
	Disks      []Disk             `json:"disks,omitempty"`
	Network    []NetworkInterface `json:"network,omitempty"`
	Sensors    Sensors            `json:"sensors,omitempty"`
	Tags       []string           `json:"tags,omitempty"`
	Timestamp  time.Time          `json:"timestamp"`
	SequenceID string             `json:"sequenceId,omitempty"`
}

// AgentInfo describes the reporting agent.
type AgentInfo struct {
	ID              string `json:"id"`
	Version         string `json:"version,omitempty"`
	IntervalSeconds int    `json:"intervalSeconds,omitempty"`
	Hostname        string `json:"hostname,omitempty"`
}

// HostInfo contains platform and identification details about the monitored host.
type HostInfo struct {
	ID            string    `json:"id,omitempty"`
	Hostname      string    `json:"hostname"`
	DisplayName   string    `json:"displayName,omitempty"`
	MachineID     string    `json:"machineId,omitempty"`
	Platform      string    `json:"platform,omitempty"`
	OSName        string    `json:"osName,omitempty"`
	OSVersion     string    `json:"osVersion,omitempty"`
	KernelVersion string    `json:"kernelVersion,omitempty"`
	Architecture  string    `json:"architecture,omitempty"`
	CPUModel      string    `json:"cpuModel,omitempty"`
	CPUCount      int       `json:"cpuCount,omitempty"`
	UptimeSeconds int64     `json:"uptimeSeconds,omitempty"`
	LoadAverage   []float64 `json:"loadAverage,omitempty"`
}

// Metrics encapsulates primary resource metrics for a host.
type Metrics struct {
	CPUUsagePercent float64      `json:"cpuUsagePercent,omitempty"`
	Memory          MemoryMetric `json:"memory,omitempty"`
}

// MemoryMetric captures memory usage statistics in bytes.
type MemoryMetric struct {
	TotalBytes int64   `json:"totalBytes,omitempty"`
	UsedBytes  int64   `json:"usedBytes,omitempty"`
	FreeBytes  int64   `json:"freeBytes,omitempty"`
	Usage      float64 `json:"usage,omitempty"`
	SwapTotal  int64   `json:"swapTotalBytes,omitempty"`
	SwapUsed   int64   `json:"swapUsedBytes,omitempty"`
}

// Disk represents disk utilisation metrics.
type Disk struct {
	Device     string  `json:"device,omitempty"`
	Mountpoint string  `json:"mountpoint,omitempty"`
	Filesystem string  `json:"filesystem,omitempty"`
	Type       string  `json:"type,omitempty"`
	TotalBytes int64   `json:"totalBytes,omitempty"`
	UsedBytes  int64   `json:"usedBytes,omitempty"`
	FreeBytes  int64   `json:"freeBytes,omitempty"`
	Usage      float64 `json:"usage,omitempty"`
}

// NetworkInterface summarises network adapter statistics.
type NetworkInterface struct {
	Name      string   `json:"name"`
	MAC       string   `json:"mac,omitempty"`
	Addresses []string `json:"addresses,omitempty"`
	RXBytes   uint64   `json:"rxBytes,omitempty"`
	TXBytes   uint64   `json:"txBytes,omitempty"`
	SpeedMbps *int64   `json:"speedMbps,omitempty"`
}

// Sensors captures optional sensor readings reported by the agent.
type Sensors struct {
	TemperatureCelsius map[string]float64 `json:"temperatureCelsius,omitempty"`
	FanRPM             map[string]float64 `json:"fanRpm,omitempty"`
	Additional         map[string]float64 `json:"additional,omitempty"`
}
