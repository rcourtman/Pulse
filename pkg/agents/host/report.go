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
	Ceph       *CephCluster       `json:"ceph,omitempty"`
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
	CommandsEnabled bool   `json:"commandsEnabled,omitempty"` // Whether AI command execution is enabled
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
	ReportIP      string    `json:"reportIp,omitempty"` // User-specified IP for multi-NIC systems
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
	PowerWatts         map[string]float64 `json:"powerWatts,omitempty"` // Power consumption (e.g., cpu_package, dram)
	Additional         map[string]float64 `json:"additional,omitempty"`
	SMART              []DiskSMART        `json:"smart,omitempty"` // S.M.A.R.T. disk data
}

// DiskSMART represents S.M.A.R.T. data for a single disk.
type DiskSMART struct {
	Device      string `json:"device"`            // Device path (e.g., sda)
	Model       string `json:"model,omitempty"`   // Disk model
	Serial      string `json:"serial,omitempty"`  // Serial number
	WWN         string `json:"wwn,omitempty"`     // World Wide Name
	Type        string `json:"type,omitempty"`    // Transport type: sata, sas, nvme
	Temperature int    `json:"temperature"`       // Temperature in Celsius
	Health      string `json:"health,omitempty"`  // PASSED, FAILED, UNKNOWN
	Standby     bool   `json:"standby,omitempty"` // True if disk was in standby
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

// CephCluster represents Ceph cluster status collected by the host agent.
type CephCluster struct {
	FSID        string           `json:"fsid"`
	Health      CephHealth       `json:"health"`
	MonMap      CephMonitorMap   `json:"monMap,omitempty"`
	MgrMap      CephManagerMap   `json:"mgrMap,omitempty"`
	OSDMap      CephOSDMap       `json:"osdMap"`
	PGMap       CephPGMap        `json:"pgMap"`
	Pools       []CephPool       `json:"pools,omitempty"`
	Services    []CephService    `json:"services,omitempty"`
	CollectedAt string           `json:"collectedAt"`
}

// CephHealth represents Ceph cluster health status.
type CephHealth struct {
	Status  string                 `json:"status"` // HEALTH_OK, HEALTH_WARN, HEALTH_ERR
	Checks  map[string]CephCheck   `json:"checks,omitempty"`
	Summary []CephHealthSummary    `json:"summary,omitempty"`
}

// CephCheck represents a health check detail.
type CephCheck struct {
	Severity string   `json:"severity"`
	Message  string   `json:"message,omitempty"`
	Detail   []string `json:"detail,omitempty"`
}

// CephHealthSummary represents a health summary message.
type CephHealthSummary struct {
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

// CephMonitorMap represents Ceph monitor information.
type CephMonitorMap struct {
	Epoch    int           `json:"epoch"`
	NumMons  int           `json:"numMons"`
	Monitors []CephMonitor `json:"monitors,omitempty"`
}

// CephMonitor represents a single Ceph monitor.
type CephMonitor struct {
	Name   string `json:"name"`
	Rank   int    `json:"rank"`
	Addr   string `json:"addr,omitempty"`
	Status string `json:"status,omitempty"`
}

// CephManagerMap represents Ceph manager information.
type CephManagerMap struct {
	Available bool   `json:"available"`
	NumMgrs   int    `json:"numMgrs"`
	ActiveMgr string `json:"activeMgr,omitempty"`
	Standbys  int    `json:"standbys"`
}

// CephOSDMap represents OSD status summary.
type CephOSDMap struct {
	Epoch   int `json:"epoch"`
	NumOSDs int `json:"numOsds"`
	NumUp   int `json:"numUp"`
	NumIn   int `json:"numIn"`
	NumDown int `json:"numDown,omitempty"`
	NumOut  int `json:"numOut,omitempty"`
}

// CephPGMap represents placement group statistics.
type CephPGMap struct {
	NumPGs           int     `json:"numPgs"`
	BytesTotal       uint64  `json:"bytesTotal"`
	BytesUsed        uint64  `json:"bytesUsed"`
	BytesAvailable   uint64  `json:"bytesAvailable"`
	DataBytes        uint64  `json:"dataBytes,omitempty"`
	UsagePercent     float64 `json:"usagePercent"`
	DegradedRatio    float64 `json:"degradedRatio,omitempty"`
	MisplacedRatio   float64 `json:"misplacedRatio,omitempty"`
	ReadBytesPerSec  uint64  `json:"readBytesPerSec,omitempty"`
	WriteBytesPerSec uint64  `json:"writeBytesPerSec,omitempty"`
	ReadOpsPerSec    uint64  `json:"readOpsPerSec,omitempty"`
	WriteOpsPerSec   uint64  `json:"writeOpsPerSec,omitempty"`
}

// CephPool represents a Ceph pool.
type CephPool struct {
	ID             int     `json:"id"`
	Name           string  `json:"name"`
	BytesUsed      uint64  `json:"bytesUsed"`
	BytesAvailable uint64  `json:"bytesAvailable"`
	Objects        uint64  `json:"objects"`
	PercentUsed    float64 `json:"percentUsed"`
}

// CephService represents a Ceph service summary.
type CephService struct {
	Type    string   `json:"type"` // mon, mgr, osd, mds, rgw
	Running int      `json:"running"`
	Total   int      `json:"total"`
	Daemons []string `json:"daemons,omitempty"`
}
