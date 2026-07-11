package host

import "time"

// Report represents the payload sent by the host module of pulse-agent.
type Report struct {
	Agent          AgentInfo            `json:"agent"`
	Host           HostInfo             `json:"host"`
	Metrics        Metrics              `json:"metrics"`
	Disks          []Disk               `json:"disks,omitempty"`
	DiskIO         []DiskIO             `json:"diskIO,omitempty"`
	Network        []NetworkInterface   `json:"network,omitempty"`
	Sensors        Sensors              `json:"sensors,omitempty"`
	RAID           []RAIDArray          `json:"raid,omitempty"`
	Unraid         *UnraidStorage       `json:"unraid,omitempty"`
	Ceph           *CephCluster         `json:"ceph,omitempty"`
	ClusterSensors []ClusterNodeSensors `json:"clusterSensors,omitempty"`
	Tags           []string             `json:"tags,omitempty"`
	Timestamp      time.Time            `json:"timestamp"`
	SequenceID     string               `json:"sequenceId,omitempty"`
}

// ClusterNodeSensors contains temperature sensor data collected from a Proxmox
// cluster sibling node via SSH. The agent on one node SSHes to peers using the
// cluster's pre-existing root SSH trust to run `sensors -j`.
type ClusterNodeSensors struct {
	NodeName    string  `json:"nodeName"`              // Proxmox cluster node name (lowercase)
	Sensors     Sensors `json:"sensors"`               // Temperature/fan/additional sensor data
	CollectedAt string  `json:"collectedAt,omitempty"` // RFC3339 timestamp of collection
}

// AgentInfo describes the reporting agent.
type AgentInfo struct {
	ID              string             `json:"id"`
	Version         string             `json:"version,omitempty"`
	Type            string             `json:"type,omitempty"` // "unified", "host", or "docker" - empty means legacy
	IntervalSeconds int                `json:"intervalSeconds,omitempty"`
	Hostname        string             `json:"hostname,omitempty"`
	UpdatedFrom     string             `json:"updatedFrom,omitempty"`     // Previous version if recently auto-updated
	CommandsEnabled bool               `json:"commandsEnabled,omitempty"` // Whether AI command execution is enabled
	DiskExclude     []string           `json:"diskExclude,omitempty"`     // Disk exclusion patterns from --disk-exclude flag
	AppliedConfig   *ConfigFingerprint `json:"appliedConfig,omitempty"`
	Update          *UpdateStatus      `json:"update,omitempty"`
	Modules         []ModuleStatus     `json:"modules,omitempty"`
}

// ModuleStatus describes whether an enabled Unified Agent module initialized
// and is actively running. Errors are deliberately limited to the latest
// operator-facing initialization failure.
type ModuleStatus struct {
	Name      string    `json:"name"`
	Enabled   bool      `json:"enabled"`
	State     string    `json:"state"`
	LastError string    `json:"lastError,omitempty"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// ConfigFingerprint identifies the effective managed configuration applied by
// the running agent without exposing any configuration values or secrets.
type ConfigFingerprint struct {
	Version string `json:"version"`
	Hash    string `json:"hash"`
}

// UpdateStatus is the agent-authored state of its self-update loop. Server-side
// version comparison remains authoritative for whether an update is available;
// this payload explains whether the agent is actually checking and why its last
// attempt succeeded, failed, or is disabled.
type UpdateStatus struct {
	State            string     `json:"state"`
	AutoUpdate       bool       `json:"autoUpdate"`
	UpdatedFrom      string     `json:"updatedFrom,omitempty"`
	AvailableVersion string     `json:"availableVersion,omitempty"`
	LastCheckedAt    *time.Time `json:"lastCheckedAt,omitempty"`
	LastAttemptAt    *time.Time `json:"lastAttemptAt,omitempty"`
	LastSuccessAt    *time.Time `json:"lastSuccessAt,omitempty"`
	LastError        string     `json:"lastError,omitempty"`
}

// HostInfo contains platform and identification details about the monitored host.
type HostInfo struct {
	ID             string                `json:"id,omitempty"`
	Hostname       string                `json:"hostname"`
	DisplayName    string                `json:"displayName,omitempty"`
	MachineID      string                `json:"machineId,omitempty"`
	Platform       string                `json:"platform,omitempty"`
	OSName         string                `json:"osName,omitempty"`
	OSVersion      string                `json:"osVersion,omitempty"`
	KernelVersion  string                `json:"kernelVersion,omitempty"`
	Architecture   string                `json:"architecture,omitempty"`
	CPUModel       string                `json:"cpuModel,omitempty"`
	CPUCount       int                   `json:"cpuCount,omitempty"`
	UptimeSeconds  int64                 `json:"uptimeSeconds,omitempty"`
	LoadAverage    []float64             `json:"loadAverage,omitempty"`
	ReportIP       string                `json:"reportIp,omitempty"` // User-specified IP for multi-NIC systems
	PackageUpdates *PackageUpdateStatus  `json:"packageUpdates,omitempty"`
	StorageCleanup *StorageCleanupStatus `json:"storageCleanup,omitempty"`
}

// PackageUpdateStatus is the agent-authored, read-only package posture for
// the host OS. It reports only package identifiers and versions; execution is
// a separate typed agent operation and never accepts model-authored commands.
type PackageUpdateStatus struct {
	Supported      bool            `json:"supported"`
	Manager        string          `json:"manager,omitempty"`
	InventoryHash  string          `json:"inventoryHash,omitempty"`
	PendingCount   int             `json:"pendingCount"`
	Packages       []PackageUpdate `json:"packages,omitempty"`
	CheckedAt      time.Time       `json:"checkedAt,omitempty"`
	RebootRequired bool            `json:"rebootRequired,omitempty"`
	Error          string          `json:"error,omitempty"`
}

// PackageUpdate identifies one package visible in the package manager's
// bounded upgrade simulation. Values are informational and never executed.
type PackageUpdate struct {
	Name             string `json:"name"`
	InstalledVersion string `json:"installedVersion,omitempty"`
	AvailableVersion string `json:"availableVersion,omitempty"`
}

// StorageCleanupStatus is the agent-authored read-only posture for the
// bounded package-cache cleanup provider. Paths and cache entry names never
// leave the agent; the fingerprint binds a later typed cleanup request.
type StorageCleanupStatus struct {
	Supported        bool      `json:"supported"`
	Provider         string    `json:"provider,omitempty"`
	Fingerprint      string    `json:"fingerprint,omitempty"`
	ReclaimableBytes int64     `json:"reclaimableBytes"`
	CheckedAt        time.Time `json:"checkedAt,omitempty"`
	Error            string    `json:"error,omitempty"`
}

// Metrics encapsulates primary resource metrics for a host.
type Metrics struct {
	CPUUsagePercent float64      `json:"cpuUsagePercent,omitempty"`
	Memory          MemoryMetric `json:"memory,omitempty"`
}

// MemoryMetric captures memory usage statistics in bytes.
type MemoryMetric struct {
	TotalBytes int64 `json:"totalBytes,omitempty"`
	UsedBytes  int64 `json:"usedBytes,omitempty"`
	FreeBytes  int64 `json:"freeBytes,omitempty"`
	// CacheBytes is the reclaimable page cache (Available - Free);
	// used + cache + free ≈ total.
	CacheBytes int64   `json:"cacheBytes,omitempty"`
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
	Device     string `json:"device"`                // e.g., "nvme0n1", "sda"
	ReadBytes  uint64 `json:"readBytes,omitempty"`   // Total bytes read
	WriteBytes uint64 `json:"writeBytes,omitempty"`  // Total bytes written
	ReadOps    uint64 `json:"readOps,omitempty"`     // Total read operations
	WriteOps   uint64 `json:"writeOps,omitempty"`    // Total write operations
	ReadTime   uint64 `json:"readTimeMs,omitempty"`  // Total time spent reading (ms)
	WriteTime  uint64 `json:"writeTimeMs,omitempty"` // Total time spent writing (ms)
	IOTime     uint64 `json:"ioTimeMs,omitempty"`    // Total time spent doing I/O (ms)
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
	GPU                []GPUSensor        `json:"gpu,omitempty"`
	ThermalState       *ThermalState      `json:"thermalState,omitempty"`
	SMART              []DiskSMART        `json:"smart,omitempty"` // S.M.A.R.T. disk data
}

// GPUSensor captures direct GPU telemetry reported by a local host agent.
type GPUSensor struct {
	ID                 string   `json:"id,omitempty"`
	Name               string   `json:"name,omitempty"`
	TemperatureCelsius *float64 `json:"temperatureCelsius,omitempty"`
	UtilizationPercent *float64 `json:"utilizationPercent,omitempty"`
	MemoryUsedBytes    *int64   `json:"memoryUsedBytes,omitempty"`
	MemoryTotalBytes   *int64   `json:"memoryTotalBytes,omitempty"`
}

// ThermalState captures OS-level thermal pressure that is not a direct
// Celsius temperature reading. macOS exposes this kind of signal more
// reliably than raw sensor temperatures.
type ThermalState struct {
	Source                  string         `json:"source,omitempty"`
	Pressure                string         `json:"pressure,omitempty"` // nominal, constrained, or unknown
	ThermalWarningLevel     *int           `json:"thermalWarningLevel,omitempty"`
	PerformanceWarningLevel *int           `json:"performanceWarningLevel,omitempty"`
	CPUPowerStatus          *int           `json:"cpuPowerStatus,omitempty"`
	LimitsPercent           map[string]int `json:"limitsPercent,omitempty"`
}

const (
	ThermalPressureNominal     = "nominal"
	ThermalPressureConstrained = "constrained"
	ThermalPressureUnknown     = "unknown"
)

// DiskSMART represents S.M.A.R.T. data for a single disk.
type DiskSMART struct {
	Device      string           `json:"device"`              // Block device name (e.g., sda, nvme0n1)
	Model       string           `json:"model,omitempty"`     // Disk model
	Serial      string           `json:"serial,omitempty"`    // Serial number
	WWN         string           `json:"wwn,omitempty"`       // World Wide Name
	Type        string           `json:"type,omitempty"`      // Transport type: sata, sas, nvme
	SizeBytes   int64            `json:"sizeBytes,omitempty"` // Capacity in bytes (0 when unknown)
	Temperature int              `json:"temperature"`         // Temperature in Celsius
	Health      string           `json:"health,omitempty"`    // PASSED, FAILED, UNKNOWN
	Standby     bool             `json:"standby,omitempty"`   // True if disk was in standby
	Pool        string           `json:"pool,omitempty"`      // ZFS pool this disk belongs to (empty if not a ZFS member)
	Attributes  *SMARTAttributes `json:"attributes,omitempty"`
}

// SMARTAttributes holds normalized SMART attributes for both SATA and NVMe disks.
type SMARTAttributes struct {
	PowerOnHours         *int64 `json:"powerOnHours,omitempty"`
	PowerCycles          *int64 `json:"powerCycles,omitempty"`
	ReallocatedSectors   *int64 `json:"reallocatedSectors,omitempty"`
	PendingSectors       *int64 `json:"pendingSectors,omitempty"`
	OfflineUncorrectable *int64 `json:"offlineUncorrectable,omitempty"`
	UDMACRCErrors        *int64 `json:"udmaCrcErrors,omitempty"`
	PercentageUsed       *int   `json:"percentageUsed,omitempty"`
	AvailableSpare       *int   `json:"availableSpare,omitempty"`
	MediaErrors          *int64 `json:"mediaErrors,omitempty"`
	UnsafeShutdowns      *int64 `json:"unsafeShutdowns,omitempty"`
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
	// Operation reports the in-progress sync action from /proc/mdstat:
	// "recovery", "resync", "check", or "reshape". Empty when idle.
	Operation string `json:"operation,omitempty"`
}

// RAIDDevice represents a single device in a RAID array.
type RAIDDevice struct {
	Device string `json:"device"` // e.g., /dev/sda1
	State  string `json:"state"`  // active, spare, faulty, removed
	Slot   int    `json:"slot"`   // Position in array (-1 if not applicable)
}

// UnraidStorage represents best-effort Unraid array topology collected locally.
type UnraidStorage struct {
	ArrayStarted bool         `json:"arrayStarted"`
	ArrayState   string       `json:"arrayState,omitempty"`
	SyncAction   string       `json:"syncAction,omitempty"`
	SyncProgress float64      `json:"syncProgress,omitempty"`
	SyncErrors   int64        `json:"syncErrors,omitempty"`
	NumProtected int          `json:"numProtected,omitempty"`
	NumDisabled  int          `json:"numDisabled,omitempty"`
	NumInvalid   int          `json:"numInvalid,omitempty"`
	NumMissing   int          `json:"numMissing,omitempty"`
	Disks        []UnraidDisk `json:"disks,omitempty"`
}

// UnraidDisk represents a disk's role and state inside an Unraid array.
type UnraidDisk struct {
	Name        string `json:"name"`
	Device      string `json:"device,omitempty"`
	Role        string `json:"role,omitempty"`
	Status      string `json:"status,omitempty"`
	RawStatus   string `json:"rawStatus,omitempty"`
	Model       string `json:"model,omitempty"`
	Serial      string `json:"serial,omitempty"`
	Filesystem  string `json:"filesystem,omitempty"`
	Transport   string `json:"transport,omitempty"`
	SizeBytes   int64  `json:"sizeBytes,omitempty"`
	UsedBytes   int64  `json:"usedBytes,omitempty"`
	FreeBytes   int64  `json:"freeBytes,omitempty"`
	Temperature int    `json:"temperature,omitempty"`
	SpunDown    bool   `json:"spunDown,omitempty"`
	ReadCount   int64  `json:"readCount,omitempty"`
	WriteCount  int64  `json:"writeCount,omitempty"`
	ErrorCount  int64  `json:"errorCount,omitempty"`
	Slot        int    `json:"slot,omitempty"`
}

// CephCluster represents Ceph cluster status collected by the host agent.
type CephCluster struct {
	FSID        string         `json:"fsid"`
	Health      CephHealth     `json:"health"`
	MonMap      CephMonitorMap `json:"monMap,omitempty"`
	MgrMap      CephManagerMap `json:"mgrMap,omitempty"`
	OSDMap      CephOSDMap     `json:"osdMap"`
	PGMap       CephPGMap      `json:"pgMap"`
	Pools       []CephPool     `json:"pools,omitempty"`
	Services    []CephService  `json:"services,omitempty"`
	CollectedAt string         `json:"collectedAt"`
}

// CephHealth represents Ceph cluster health status.
type CephHealth struct {
	Status  string               `json:"status"` // HEALTH_OK, HEALTH_WARN, HEALTH_ERR
	Checks  map[string]CephCheck `json:"checks,omitempty"`
	Summary []CephHealthSummary  `json:"summary,omitempty"`
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
