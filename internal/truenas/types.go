package truenas

import "time"

// FixtureSnapshot represents a deterministic TrueNAS API snapshot used for contract testing.
type FixtureSnapshot struct {
	CollectedAt      time.Time
	System           SystemInfo
	Pools            []Pool
	Datasets         []Dataset
	Disks            []Disk
	Alerts           []Alert
	Services         []Service
	Apps             []App
	VMs              []VirtualMachine
	Shares           []NetworkShare
	ZFSSnapshots     []ZFSSnapshot
	ReplicationTasks []ReplicationTask
}

// SystemInfo mirrors high-level TrueNAS system identity/status data.
type SystemInfo struct {
	Hostname             string
	Version              string
	Build                string
	UptimeSeconds        int64
	Healthy              bool
	MachineID            string
	CPUCount             int
	MemoryTotalBytes     int64
	MemoryAvailableBytes int64
	CPUPercent           float64
	NetInRate            float64
	NetOutRate           float64
	DiskReadRate         float64
	DiskWriteRate        float64
	TemperatureCelsius   map[string]float64
	IntervalSeconds      int
	CollectedAt          time.Time
}

// Pool mirrors the subset of TrueNAS pool fields needed for unified mapping.
type Pool struct {
	ID             string
	GUID           string
	Name           string
	Status         string
	StatusCode     string
	StatusDetail   string
	TotalBytes     int64
	UsedBytes      int64
	FreeBytes      int64
	ReadErrors     int64
	WriteErrors    int64
	ChecksumErrors int64
	Scan           *PoolScan
	VDevs          []PoolVDev
	// IsBoot distinguishes the boot pool collected from boot.get_state from
	// data pools. Identity remains connection-scoped at projection time.
	IsBoot bool
	// DiskMembers are the leaf disks of the pool's vdev topology with their
	// per-member ZFS state. pool.query attaches topology for data pools and
	// boot.get_state supplies the separate boot-pool topology. Together they
	// are the API sources for disk→pool membership and per-disk health:
	// disk.query carries neither a status nor a SMART field, and only reports
	// pool membership behind an extra option the REST bridge cannot pass
	// (#1573, #1609).
	DiskMembers []PoolDiskMember
}

// PoolDiskMember is a leaf disk in a pool's vdev topology.
type PoolDiskMember struct {
	// Disk is the whole-disk device name (da0, sda) middleware resolves for
	// every path-bearing leaf; Device is the partition/label device (da0p2).
	Disk   string
	Device string
	Path   string
	GUID   string
	Type   string
	Role   string
	// Status is the per-member ZFS state: ONLINE, DEGRADED, FAULTED,
	// OFFLINE, UNAVAIL, REMOVED (spares report AVAIL/INUSE).
	Status string
	// Missing is true only when the native topology reports an unavailable
	// datastore row or an unavail_disk vdev. Absence from disk.query alone is
	// never treated as evidence that a disk is missing.
	Missing        bool
	ReadErrors     int64
	WriteErrors    int64
	ChecksumErrors int64
	Message        string
}

// PoolVDev preserves the flattened native topology for mirrors, RAIDZ groups,
// spares, special/log/cache vdevs, and their leaf disks.
type PoolVDev struct {
	ID             string
	ParentID       string
	GUID           string
	Name           string
	Type           string
	Role           string
	Disk           string
	Device         string
	Path           string
	Status         string
	ReadErrors     int64
	WriteErrors    int64
	ChecksumErrors int64
	Missing        bool
	Message        string
}

// PoolScan is the native pool.query/boot.get_state scrub or resilver state.
type PoolScan struct {
	Function              string
	State                 string
	Percentage            float64
	Errors                int64
	BytesExamined         int64
	BytesToProcess        int64
	TotalSecondsRemaining int64
	StartedAt             *time.Time
	EndedAt               *time.Time
}

// DatasetReadOnlyReason describes why a dataset is read-only when the API
// reports readonly=on. An unspecified reason remains operator-actionable.
type DatasetReadOnlyReason string

const (
	DatasetReadOnlyUnspecified       DatasetReadOnlyReason = ""
	DatasetReadOnlyReplicationTarget DatasetReadOnlyReason = "replication-target"
)

// Dataset mirrors the subset of TrueNAS dataset fields needed for unified mapping.
type Dataset struct {
	ID             string
	Name           string
	Pool           string
	UsedBytes      int64
	AvailBytes     int64
	Mounted        bool
	Locked         bool
	ReadOnly       bool
	ReadOnlyReason DatasetReadOnlyReason
}

// Disk mirrors a TrueNAS disk listing entry.
type Disk struct {
	ID                   string
	Name                 string
	Pool                 string
	Status               string
	Health               string
	HealthStatusPresent  bool
	Model                string
	Serial               string
	SizeBytes            int64
	Temperature          int
	TemperatureAggregate DiskTemperatureAggregate
	Transport            string
	Rotational           bool
}

// DiskTemperatureAggregate stores recent aggregate disk-temperature history
// derived from the native TrueNAS disk.temperature_agg API.
type DiskTemperatureAggregate struct {
	WindowDays int
	MinCelsius float64
	AvgCelsius float64
	MaxCelsius float64
}

// TimeSeriesPoint stores one provider-native metric point before it is mapped
// onto the canonical monitoring/chart surface.
type TimeSeriesPoint struct {
	Timestamp time.Time
	Value     float64
}

// SystemMetricHistory stores provider-native TrueNAS system history before it
// is normalized onto the canonical monitoring guest-chart surface.
type SystemMetricHistory struct {
	CPUPercent           []TimeSeriesPoint
	MemoryPercent        []TimeSeriesPoint
	MemoryUsedBytes      []TimeSeriesPoint
	MemoryAvailableBytes []TimeSeriesPoint
	MemoryTotalBytes     []TimeSeriesPoint
	NetInRate            []TimeSeriesPoint
	NetOutRate           []TimeSeriesPoint
	DiskReadRate         []TimeSeriesPoint
	DiskWriteRate        []TimeSeriesPoint
}

// Alert mirrors a TrueNAS alert listing entry.
type Alert struct {
	ID        string
	Level     string
	Message   string
	Source    string
	Dismissed bool
	Datetime  time.Time
}

// Service mirrors the service.query system service inventory returned by
// TrueNAS middleware.
type Service struct {
	ID      string
	Service string
	Enabled bool
	State   string
	PIDs    []int
}

// App mirrors the subset of TrueNAS application fields needed for unified
// workload mapping.
type App struct {
	ID                    string
	Name                  string
	State                 string
	Version               string
	HumanVersion          string
	CustomApp             bool
	UpgradeAvailable      bool
	ImageUpdatesAvailable bool
	Notes                 string
	ContainerCount        int
	UsedHostIPs           []string
	UsedPorts             []AppPort
	Containers            []AppContainer
	Volumes               []AppVolume
	Images                []string
	Networks              []AppNetwork
	Stats                 *AppStats
}

// AppStats contains live API-backed workload telemetry for one TrueNAS app.
type AppStats struct {
	CPUPercent      float64
	MemoryBytes     int64
	NetInRate       float64
	NetOutRate      float64
	BlockReadBytes  int64
	BlockWriteBytes int64
	DiskReadRate    float64
	DiskWriteRate   float64
	IntervalSeconds int
	CollectedAt     time.Time
	Interfaces      []AppInterfaceStats
}

// AppInterfaceStats stores per-interface throughput from TrueNAS app.stats.
type AppInterfaceStats struct {
	Name      string
	RxBytesPS float64
	TxBytesPS float64
}

// AppPort describes an app-level published port mapping.
type AppPort struct {
	ContainerPort int
	Protocol      string
	HostPorts     []AppHostPort
}

// AppHostPort describes a host-bound port used by a TrueNAS app.
type AppHostPort struct {
	HostPort int
	HostIP   string
}

// AppContainer describes one runtime container inside a TrueNAS app.
type AppContainer struct {
	ID           string
	ServiceName  string
	Image        string
	State        string
	PortConfig   []AppPort
	VolumeMounts []AppVolume
}

// AppLogLine stores one bounded log entry returned by the TrueNAS app log API.
type AppLogLine struct {
	Timestamp string
	Data      string
}

// AppLogResult captures a bounded log read for one TrueNAS app container.
type AppLogResult struct {
	Host      string
	App       App
	Container AppContainer
	Lines     []AppLogLine
	TailLines int
}

// AppConfigResult captures the current configuration/runtime shape of one
// TrueNAS application.
type AppConfigResult struct {
	Host string
	App  App
}

// AppVolume describes a bind or named volume mount exposed by a TrueNAS app.
type AppVolume struct {
	Source      string
	Destination string
	Mode        string
	Type        string
}

// AppNetwork describes a Docker network attached to a TrueNAS app.
type AppNetwork struct {
	ID     string
	Name   string
	Labels map[string]string
}

// VirtualMachine mirrors the subset of TrueNAS VM fields needed for unified
// workload mapping from vm.query.
type VirtualMachine struct {
	ID                    string
	Name                  string
	Description           string
	State                 string
	DomainState           string
	PID                   int
	VCPUs                 int
	Cores                 int
	Threads               int
	MemoryBytes           int64
	MinMemoryBytes        int64
	CPUMode               string
	CPUModel              string
	Bootloader            string
	Autostart             bool
	SuspendOnSnapshot     bool
	TrustedPlatformModule bool
	SecureBoot            bool
	Time                  string
	ArchType              string
	MachineType           string
	UUID                  string
	DisplayAvailable      bool
	DeviceCount           int
	DiskCount             int
	NICCount              int
	DisplayCount          int
	CDROMCount            int
	USBCount              int
	PCICount              int
}

// NetworkShare mirrors the subset of TrueNAS SMB/NFS sharing fields needed for
// canonical NAS share inventory.
type NetworkShare struct {
	ID                     string
	Name                   string
	Protocol               string
	Path                   string
	Dataset                string
	RelativePath           string
	Comment                string
	Enabled                bool
	ReadOnly               bool
	Browsable              bool
	Locked                 bool
	AccessBasedEnumeration bool
	AuditEnabled           bool
	ExposeSnapshots        bool
	Aliases                []string
	Hosts                  []string
	Networks               []string
	Security               []string
	MapRootUser            string
	MapRootGroup           string
	MapAllUser             string
	MapAllGroup            string
}

// ZFSSnapshot mirrors the subset of snapshot fields needed for recovery-point mapping.
type ZFSSnapshot struct {
	ID         string
	Dataset    string
	Name       string // snapshot name (without dataset prefix), best-effort
	FullName   string // dataset@snapshot, best-effort
	CreatedAt  *time.Time
	UsedBytes  *int64
	Referenced *int64
}

// ReplicationTask mirrors the subset of replication task fields needed for recovery-point mapping.
type ReplicationTask struct {
	ID             string
	Name           string
	SourceDatasets []string
	TargetDataset  string
	Direction      string
	Transport      string
	ReadOnlyMode   string
	// TargetHost is the configured SSH credential host for remote PUSH
	// replication. It is intentionally absent for unexpanded credentials.
	TargetHost string

	LastRun   *time.Time
	LastState string // SUCCESS / FAILED / RUNNING / etc, best-effort
	LastError string

	// Best-effort last snapshot name/identifier if exposed by API.
	LastSnapshot string
}
