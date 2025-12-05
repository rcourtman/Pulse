package host

import "time"

// Report represents the payload sent by the pulse-host-agent.
type Report struct {
	Agent      AgentInfo          `json:"agent"`
	Host       HostInfo           `json:"host"`
	Metrics    Metrics            `json:"metrics"`
	Disks      []Disk             `json:"disks,omitempty"`
	DiskIO     []DiskIO           `json:"diskIO,omitempty"`
	Network    []NetworkInterface `json:"network,omitempty"`
	Sensors    Sensors            `json:"sensors,omitempty"`
	RAID       []RAIDArray        `json:"raid,omitempty"`
	Tags       []string           `json:"tags,omitempty"`
	Timestamp  time.Time          `json:"timestamp"`
	SequenceID string             `json:"sequenceId,omitempty"`
}

// AgentInfo describes the reporting agent.
type AgentInfo struct {
	ID              string `json:"id"`
	Version         string `json:"version,omitempty"`
	Type            string `json:"type,omitempty"` // "unified", "host", or "docker" - empty means legacy
	IntervalSeconds int    `json:"intervalSeconds,omitempty"`
	Hostname        string `json:"hostname,omitempty"`
	UpdatedFrom     string `json:"updatedFrom,omitempty"` // Previous version if recently auto-updated
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

// DiskIO represents disk I/O statistics for a block device.
// These are cumulative counters since boot - the backend calculates rates.
type DiskIO struct {
	Device     string `json:"device"`               // e.g., "nvme0n1", "sda"
	ReadBytes  uint64 `json:"readBytes,omitempty"`  // Total bytes read
	WriteBytes uint64 `json:"writeBytes,omitempty"` // Total bytes written
	ReadOps    uint64 `json:"readOps,omitempty"`    // Total read operations
	WriteOps   uint64 `json:"writeOps,omitempty"`   // Total write operations
	ReadTime   uint64 `json:"readTimeMs,omitempty"` // Total time spent reading (ms)
	WriteTime  uint64 `json:"writeTimeMs,omitempty"`// Total time spent writing (ms)
	IOTime     uint64 `json:"ioTimeMs,omitempty"`   // Total time spent doing I/O (ms)
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

// RAIDArray represents an mdadm RAID array.
type RAIDArray struct {
	Device         string       `json:"device"`                 // e.g., /dev/md0
	Name           string       `json:"name,omitempty"`         // Array name if set
	Level          string       `json:"level"`                  // RAID level: raid0, raid1, raid5, raid6, raid10
	State          string       `json:"state"`                  // clean, active, degraded, recovering, resyncing, etc.
	TotalDevices   int          `json:"totalDevices"`           // Total number of devices in array
	ActiveDevices  int          `json:"activeDevices"`          // Number of active devices
	WorkingDevices int          `json:"workingDevices"`         // Number of working devices
	FailedDevices  int          `json:"failedDevices"`          // Number of failed devices
	SpareDevices   int          `json:"spareDevices"`           // Number of spare devices
	UUID           string       `json:"uuid,omitempty"`         // Array UUID
	Devices        []RAIDDevice `json:"devices"`                // Individual devices in array
	RebuildPercent float64      `json:"rebuildPercent"`         // Rebuild/resync progress (0-100)
	RebuildSpeed   string       `json:"rebuildSpeed,omitempty"` // Rebuild speed (e.g., "50000K/sec")
}

// RAIDDevice represents a single device in a RAID array.
type RAIDDevice struct {
	Device string `json:"device"` // e.g., /dev/sda1
	State  string `json:"state"`  // active, spare, faulty, removed
	Slot   int    `json:"slot"`   // Position in array (-1 if not applicable)
}
