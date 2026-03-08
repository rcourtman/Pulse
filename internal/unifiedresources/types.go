package unifiedresources

import (
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"
)

// Resource represents a unified resource aggregated across multiple data sources.
type Resource struct {
	ID         string         `json:"id"`
	Type       ResourceType   `json:"type"`
	Technology string         `json:"technology,omitempty"`
	Name       string         `json:"name"`
	Status     ResourceStatus `json:"status"`
	LastSeen   time.Time      `json:"lastSeen"`
	UpdatedAt  time.Time      `json:"updatedAt,omitempty"`

	DiscoveryTarget *DiscoveryTarget   `json:"discoveryTarget,omitempty"`
	MetricsTarget   *MetricsTarget     `json:"metricsTarget,omitempty"`
	Canonical       *CanonicalIdentity `json:"canonicalIdentity,omitempty"`

	Sources      []DataSource                `json:"sources"`
	SourceStatus map[DataSource]SourceStatus `json:"sourceStatus,omitempty"`

	Identity ResourceIdentity `json:"identity,omitempty"`
	Metrics  *ResourceMetrics `json:"metrics,omitempty"`

	ParentID   *string `json:"parentId,omitempty"`
	ParentName string  `json:"parentName,omitempty"`
	ChildCount int     `json:"childCount,omitempty"`

	Tags      []string `json:"tags,omitempty"`
	CustomURL string   `json:"customUrl,omitempty"`

	// Source-specific payloads
	Proxmox      *ProxmoxData      `json:"proxmox,omitempty"`
	Storage      *StorageMeta      `json:"storage,omitempty"`
	Agent        *AgentData        `json:"agent,omitempty"`
	Docker       *DockerData       `json:"docker,omitempty"`
	PBS          *PBSData          `json:"pbs,omitempty"`
	PMG          *PMGData          `json:"pmg,omitempty"`
	Kubernetes   *K8sData          `json:"kubernetes,omitempty"`
	PhysicalDisk *PhysicalDiskMeta `json:"physicalDisk,omitempty"`
	Ceph         *CephMeta         `json:"ceph,omitempty"`
	TrueNAS      *TrueNASData      `json:"truenas,omitempty"`
}

// DiscoveryTarget describes the canonical discovery request coordinates
// for this unified resource.
type DiscoveryTarget struct {
	ResourceType string `json:"resourceType"`
	AgentID      string `json:"agentId"`
	ResourceID   string `json:"resourceId"`
	Hostname     string `json:"hostname,omitempty"`
}

// MetricsTarget describes the resource type and ID to use when querying
// the metrics history endpoint for this unified resource.
type MetricsTarget struct {
	ResourceType string `json:"resourceType"`
	ResourceID   string `json:"resourceId"`
}

// CanonicalIdentity describes the backend-chosen identity contract for a
// unified resource so frontend surfaces do not need to reconstruct labels and
// host hints from source-specific facets.
type CanonicalIdentity struct {
	DisplayName string   `json:"displayName,omitempty"`
	Hostname    string   `json:"hostname,omitempty"`
	PlatformID  string   `json:"platformId,omitempty"`
	PrimaryID   string   `json:"primaryId,omitempty"`
	Aliases     []string `json:"aliases,omitempty"`
}

// ResourceType represents the kind of resource.
type ResourceType string

const (
	// ResourceTypeAgent is the canonical v6 infrastructure parent type.
	ResourceTypeAgent           ResourceType = "agent"
	ResourceTypeVM              ResourceType = "vm"
	ResourceTypeSystemContainer ResourceType = "system-container"
	ResourceTypeAppContainer    ResourceType = "app-container"
	ResourceTypeDockerService   ResourceType = "docker-service"
	ResourceTypeK8sCluster      ResourceType = "k8s-cluster"
	ResourceTypeK8sNode         ResourceType = "k8s-node"
	ResourceTypePod             ResourceType = "pod"
	ResourceTypeK8sDeployment   ResourceType = "k8s-deployment"
	ResourceTypeStorage         ResourceType = "storage"
	ResourceTypePBS             ResourceType = "pbs"
	ResourceTypePMG             ResourceType = "pmg"
	ResourceTypeCeph            ResourceType = "ceph"
	ResourceTypePhysicalDisk    ResourceType = "physical_disk"
)

// CanonicalResourceType normalizes resource type spellings into the internal
// canonical enum value.
func CanonicalResourceType(rt ResourceType) ResourceType {
	normalized := ResourceType(strings.ToLower(strings.TrimSpace(string(rt))))
	return normalized
}

// ResourceStatus represents the high-level status of a resource.
type ResourceStatus string

const (
	StatusOnline  ResourceStatus = "online"
	StatusOffline ResourceStatus = "offline"
	StatusWarning ResourceStatus = "warning"
	StatusUnknown ResourceStatus = "unknown"
)

// DataSource represents a contributing data source.
type DataSource string

const (
	SourceProxmox DataSource = "proxmox"
	SourceAgent   DataSource = "agent"
	SourceDocker  DataSource = "docker"
	SourcePBS     DataSource = "pbs"
	SourcePMG     DataSource = "pmg"
	SourceK8s     DataSource = "kubernetes"
	SourceTrueNAS DataSource = "truenas"
)

// SourceStatus describes the freshness of data from a source.
type SourceStatus struct {
	Status   string    `json:"status"` // online, stale, offline
	LastSeen time.Time `json:"lastSeen"`
	Error    string    `json:"error,omitempty"`
}

// ResourceIdentity holds identifiers used for matching.
type ResourceIdentity struct {
	MachineID    string   `json:"machineId,omitempty"`
	DMIUUID      string   `json:"dmiUuid,omitempty"`
	Hostnames    []string `json:"hostnames,omitempty"`
	IPAddresses  []string `json:"ipAddresses,omitempty"`
	MACAddresses []string `json:"macAddresses,omitempty"`
	ClusterName  string   `json:"clusterName,omitempty"`
}

// MatchResult describes a potential identity match.
type MatchResult struct {
	ResourceA      string  `json:"resourceA"`
	ResourceB      string  `json:"resourceB"`
	Confidence     float64 `json:"confidence"`
	MatchReason    string  `json:"matchReason"`
	RequiresReview bool    `json:"requiresReview"`
}

// SourceTarget describes a source-specific mapping for a unified resource.
type SourceTarget struct {
	Source      DataSource `json:"source"`
	SourceID    string     `json:"sourceId"`
	CandidateID string     `json:"candidateId"`
}

// ResourceMetrics contains unified metrics derived from available sources.
type ResourceMetrics struct {
	CPU       *MetricValue `json:"cpu,omitempty"`
	Memory    *MetricValue `json:"memory,omitempty"`
	Disk      *MetricValue `json:"disk,omitempty"`
	NetIn     *MetricValue `json:"netIn,omitempty"`
	NetOut    *MetricValue `json:"netOut,omitempty"`
	DiskRead  *MetricValue `json:"diskRead,omitempty"`
	DiskWrite *MetricValue `json:"diskWrite,omitempty"`
}

// MetricValue represents a metric value, optionally with totals.
type MetricValue struct {
	Value   float64    `json:"value,omitempty"`
	Used    *int64     `json:"used,omitempty"`
	Total   *int64     `json:"total,omitempty"`
	Percent float64    `json:"percent,omitempty"`
	Unit    string     `json:"unit,omitempty"`
	Source  DataSource `json:"-"`
}

// ProxmoxData contains Proxmox-specific data for a resource.
type ProxmoxData struct {
	SourceID        string     `json:"sourceId,omitempty"` // raw model ID from source snapshot
	NodeName        string     `json:"nodeName,omitempty"`
	ClusterName     string     `json:"clusterName,omitempty"`
	IsClusterMember bool       `json:"isClusterMember,omitempty"`
	Instance        string     `json:"instance,omitempty"`
	HostURL         string     `json:"host,omitempty"`
	VMID            int        `json:"vmid,omitempty"`
	CPUs            int        `json:"cpus,omitempty"`
	Template        bool       `json:"template,omitempty"`
	Temperature     *float64   `json:"temperature,omitempty"` // Max node CPU temp in Celsius
	PVEVersion      string     `json:"pveVersion,omitempty"`
	KernelVersion   string     `json:"kernelVersion,omitempty"`
	Uptime          int64      `json:"uptime,omitempty"`
	LastBackup      time.Time  `json:"lastBackup,omitempty"`
	CPUInfo         *CPUInfo   `json:"cpuInfo,omitempty"`
	LoadAverage     []float64  `json:"loadAverage,omitempty"`
	PendingUpdates  int        `json:"pendingUpdates,omitempty"`
	Disks           []DiskInfo `json:"disks,omitempty"`
	SwapUsed        int64      `json:"swapUsed,omitempty"`
	SwapTotal       int64      `json:"swapTotal,omitempty"`
	Balloon         int64      `json:"balloon,omitempty"`
	Lock            string     `json:"lock,omitempty"` // Proxmox lock state (e.g. "backup", "migrate", "snapshot")
	// Internal link hint to a host agent resource.
	LinkedAgentID string `json:"-"`
}

// StorageMeta contains storage-specific metadata for storage resources.
type StorageMeta struct {
	Type          string                `json:"type,omitempty"`
	Content       string                `json:"content,omitempty"`
	ContentTypes  []string              `json:"contentTypes,omitempty"`
	Shared        bool                  `json:"shared"`
	IsCeph        bool                  `json:"isCeph"`
	IsZFS         bool                  `json:"isZfs"`
	Platform      string                `json:"platform,omitempty"`
	Topology      string                `json:"topology,omitempty"`
	Protection    string                `json:"protection,omitempty"`
	Risk          *StorageRisk          `json:"risk,omitempty"`
	ConsumerCount int                   `json:"consumerCount,omitempty"`
	ConsumerTypes []string              `json:"consumerTypes,omitempty"`
	TopConsumers  []StorageConsumerMeta `json:"topConsumers,omitempty"`

	// Accessibility metadata.
	Nodes []string `json:"nodes,omitempty"` // PVE nodes where this storage is accessible
	Path  string   `json:"path,omitempty"`  // local mount path on the node

	// ZFS metadata (when IsZFS is true and the source provides details).
	ZFSPoolState      string `json:"zfsPoolState,omitempty"`
	ZFSReadErrors     int64  `json:"zfsReadErrors,omitempty"`
	ZFSWriteErrors    int64  `json:"zfsWriteErrors,omitempty"`
	ZFSChecksumErrors int64  `json:"zfsChecksumErrors,omitempty"`

	// Unraid metadata (when Platform is "unraid").
	ArrayState   string  `json:"arrayState,omitempty"`
	SyncAction   string  `json:"syncAction,omitempty"`
	SyncProgress float64 `json:"syncProgress,omitempty"`
	NumProtected int     `json:"numProtected,omitempty"`
	NumDisabled  int     `json:"numDisabled,omitempty"`
	NumInvalid   int     `json:"numInvalid,omitempty"`
	NumMissing   int     `json:"numMissing,omitempty"`
}

type StorageConsumerMeta struct {
	ResourceID   string       `json:"resourceId,omitempty"`
	ResourceType ResourceType `json:"resourceType"`
	Name         string       `json:"name"`
	DiskCount    int          `json:"diskCount,omitempty"`
}

// PhysicalDiskMeta contains physical disk-specific metadata.
type PhysicalDiskMeta struct {
	DevPath      string            `json:"devPath"`
	Model        string            `json:"model,omitempty"`
	Serial       string            `json:"serial,omitempty"`
	WWN          string            `json:"wwn,omitempty"`
	DiskType     string            `json:"diskType"` // nvme, sata, sas
	SizeBytes    int64             `json:"sizeBytes"`
	Health       string            `json:"health"`      // PASSED, FAILED, UNKNOWN
	Wearout      int               `json:"wearout"`     // 0-100, -1 unavailable
	Temperature  int               `json:"temperature"` // Celsius
	RPM          int               `json:"rpm"`
	Used         string            `json:"used,omitempty"`
	StorageRole  string            `json:"storageRole,omitempty"`
	StorageGroup string            `json:"storageGroup,omitempty"`
	StorageState string            `json:"storageState,omitempty"`
	SMART        *SMARTMeta        `json:"smart,omitempty"`
	Risk         *PhysicalDiskRisk `json:"risk,omitempty"`
}

type StorageRisk struct {
	Level   storagehealth.RiskLevel `json:"level"`
	Reasons []StorageRiskReason     `json:"reasons,omitempty"`
}

type StorageRiskReason struct {
	Code     string                  `json:"code"`
	Severity storagehealth.RiskLevel `json:"severity"`
	Summary  string                  `json:"summary"`
}

type PhysicalDiskRisk struct {
	Level   storagehealth.RiskLevel  `json:"level"`
	Reasons []PhysicalDiskRiskReason `json:"reasons,omitempty"`
}

type PhysicalDiskRiskReason struct {
	Code     string                  `json:"code"`
	Severity storagehealth.RiskLevel `json:"severity"`
	Summary  string                  `json:"summary"`
}

// CephMeta contains Ceph cluster-specific metadata.
type CephMeta struct {
	FSID          string            `json:"fsid,omitempty"`
	HealthStatus  string            `json:"healthStatus"`
	HealthMessage string            `json:"healthMessage,omitempty"`
	NumMons       int               `json:"numMons"`
	NumMgrs       int               `json:"numMgrs"`
	NumOSDs       int               `json:"numOsds"`
	NumOSDsUp     int               `json:"numOsdsUp"`
	NumOSDsIn     int               `json:"numOsdsIn"`
	NumPGs        int               `json:"numPGs"`
	Pools         []CephPoolMeta    `json:"pools,omitempty"`
	Services      []CephServiceMeta `json:"services,omitempty"`
}

// CephPoolMeta describes a Ceph storage pool.
type CephPoolMeta struct {
	Name           string  `json:"name"`
	StoredBytes    int64   `json:"storedBytes"`
	AvailableBytes int64   `json:"availableBytes"`
	Objects        int64   `json:"objects"`
	PercentUsed    float64 `json:"percentUsed"`
}

// CephServiceMeta describes a Ceph daemon service.
type CephServiceMeta struct {
	Type    string `json:"type"`
	Running int    `json:"running"`
	Total   int    `json:"total"`
}

// SMARTMeta contains SMART attribute data for a physical disk.
type SMARTMeta struct {
	PowerOnHours         int64 `json:"powerOnHours,omitempty"`
	PowerCycles          int64 `json:"powerCycles,omitempty"`
	ReallocatedSectors   int64 `json:"reallocatedSectors,omitempty"`
	PendingSectors       int64 `json:"pendingSectors,omitempty"`
	OfflineUncorrectable int64 `json:"offlineUncorrectable,omitempty"`
	UDMACRCErrors        int64 `json:"udmaCrcErrors,omitempty"`
	PercentageUsed       int   `json:"percentageUsed,omitempty"`
	AvailableSpare       int   `json:"availableSpare,omitempty"`
	MediaErrors          int64 `json:"mediaErrors,omitempty"`
	UnsafeShutdowns      int64 `json:"unsafeShutdowns,omitempty"`
}

// HostSensorMeta contains host sensor readings.
type HostSensorMeta struct {
	TemperatureCelsius map[string]float64 `json:"temperatureCelsius,omitempty"`
	FanRPM             map[string]float64 `json:"fanRpm,omitempty"`
	Additional         map[string]float64 `json:"additional,omitempty"`
	SMART              []HostSMARTMeta    `json:"smart,omitempty"`
}

// HostSMARTMeta describes a disk's SMART data.
type HostSMARTMeta struct {
	Device      string                  `json:"device"`
	Model       string                  `json:"model,omitempty"`
	Serial      string                  `json:"serial,omitempty"`
	WWN         string                  `json:"wwn,omitempty"`
	Type        string                  `json:"type,omitempty"`
	Temperature int                     `json:"temperature"`
	Health      string                  `json:"health"`
	Standby     bool                    `json:"standby,omitempty"`
	Attributes  *models.SMARTAttributes `json:"attributes,omitempty"`
}

// HostRAIDDeviceMeta describes a device in a RAID array.
type HostRAIDDeviceMeta struct {
	Device string `json:"device"`
	State  string `json:"state"`
	Slot   int    `json:"slot"`
}

// HostRAIDMeta describes a RAID array.
type HostRAIDMeta struct {
	Device         string               `json:"device"`
	Name           string               `json:"name,omitempty"`
	Level          string               `json:"level"`
	State          string               `json:"state"`
	TotalDevices   int                  `json:"totalDevices"`
	ActiveDevices  int                  `json:"activeDevices"`
	WorkingDevices int                  `json:"workingDevices"`
	FailedDevices  int                  `json:"failedDevices"`
	SpareDevices   int                  `json:"spareDevices"`
	UUID           string               `json:"uuid,omitempty"`
	Devices        []HostRAIDDeviceMeta `json:"devices,omitempty"`
	RebuildPercent float64              `json:"rebuildPercent,omitempty"`
	RebuildSpeed   string               `json:"rebuildSpeed,omitempty"`
	Risk           *StorageRisk         `json:"risk,omitempty"`
}

// HostUnraidDiskMeta describes a disk's role and state inside an Unraid array.
type HostUnraidDiskMeta struct {
	Name       string `json:"name"`
	Device     string `json:"device,omitempty"`
	Role       string `json:"role,omitempty"`
	Status     string `json:"status,omitempty"`
	RawStatus  string `json:"rawStatus,omitempty"`
	Serial     string `json:"serial,omitempty"`
	Filesystem string `json:"filesystem,omitempty"`
	SizeBytes  int64  `json:"sizeBytes,omitempty"`
	Slot       int    `json:"slot,omitempty"`
}

// HostUnraidMeta describes Unraid array topology from a host agent.
type HostUnraidMeta struct {
	ArrayStarted bool                 `json:"arrayStarted"`
	ArrayState   string               `json:"arrayState,omitempty"`
	SyncAction   string               `json:"syncAction,omitempty"`
	SyncProgress float64              `json:"syncProgress,omitempty"`
	SyncErrors   int64                `json:"syncErrors,omitempty"`
	NumProtected int                  `json:"numProtected,omitempty"`
	NumDisabled  int                  `json:"numDisabled,omitempty"`
	NumInvalid   int                  `json:"numInvalid,omitempty"`
	NumMissing   int                  `json:"numMissing,omitempty"`
	Disks        []HostUnraidDiskMeta `json:"disks,omitempty"`
	Risk         *StorageRisk         `json:"risk,omitempty"`
}

// HostDiskIOMeta describes disk I/O counters.
type HostDiskIOMeta struct {
	Device     string `json:"device"`
	ReadBytes  uint64 `json:"readBytes"`
	WriteBytes uint64 `json:"writeBytes"`
	ReadOps    uint64 `json:"readOps"`
	WriteOps   uint64 `json:"writeOps"`
	IOTimeMs   uint64 `json:"ioTimeMs,omitempty"`
}

// HostCephCheckMeta represents a health check detail.
type HostCephCheckMeta struct {
	Severity string   `json:"severity"`
	Message  string   `json:"message,omitempty"`
	Detail   []string `json:"detail,omitempty"`
}

// HostCephHealthSummaryMeta represents a health summary message.
type HostCephHealthSummaryMeta struct {
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

// HostCephHealthMeta represents Ceph cluster health status.
type HostCephHealthMeta struct {
	Status  string                       `json:"status"`
	Checks  map[string]HostCephCheckMeta `json:"checks,omitempty"`
	Summary []HostCephHealthSummaryMeta  `json:"summary,omitempty"`
}

// HostCephMonitorMeta represents a single Ceph monitor.
type HostCephMonitorMeta struct {
	Name   string `json:"name"`
	Rank   int    `json:"rank"`
	Addr   string `json:"addr,omitempty"`
	Status string `json:"status,omitempty"`
}

// HostCephMonitorMapMeta represents Ceph monitor information.
type HostCephMonitorMapMeta struct {
	Epoch    int                   `json:"epoch"`
	NumMons  int                   `json:"numMons"`
	Monitors []HostCephMonitorMeta `json:"monitors,omitempty"`
}

// HostCephManagerMapMeta represents Ceph manager information.
type HostCephManagerMapMeta struct {
	Available bool   `json:"available"`
	NumMgrs   int    `json:"numMgrs"`
	ActiveMgr string `json:"activeMgr,omitempty"`
	Standbys  int    `json:"standbys"`
}

// HostCephOSDMapMeta represents OSD status summary.
type HostCephOSDMapMeta struct {
	Epoch   int `json:"epoch"`
	NumOSDs int `json:"numOsds"`
	NumUp   int `json:"numUp"`
	NumIn   int `json:"numIn"`
	NumDown int `json:"numDown,omitempty"`
	NumOut  int `json:"numOut,omitempty"`
}

// HostCephPGMapMeta represents placement group statistics.
type HostCephPGMapMeta struct {
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

// HostCephPoolMeta represents a Ceph pool.
type HostCephPoolMeta struct {
	ID             int     `json:"id"`
	Name           string  `json:"name"`
	BytesUsed      uint64  `json:"bytesUsed"`
	BytesAvailable uint64  `json:"bytesAvailable"`
	Objects        uint64  `json:"objects"`
	PercentUsed    float64 `json:"percentUsed"`
}

// HostCephServiceMeta represents a Ceph service summary.
type HostCephServiceMeta struct {
	Type    string   `json:"type"`
	Running int      `json:"running"`
	Total   int      `json:"total"`
	Daemons []string `json:"daemons,omitempty"`
}

// HostCephMeta describes host-level Ceph cluster data.
type HostCephMeta struct {
	FSID         string                 `json:"fsid"`
	Health       HostCephHealthMeta     `json:"health"`
	MonMap       HostCephMonitorMapMeta `json:"monMap,omitempty"`
	MgrMap       HostCephManagerMapMeta `json:"mgrMap,omitempty"`
	OSDMap       HostCephOSDMapMeta     `json:"osdMap"`
	PGMap        HostCephPGMapMeta      `json:"pgMap"`
	Pools        []HostCephPoolMeta     `json:"pools,omitempty"`
	Services     []HostCephServiceMeta  `json:"services,omitempty"`
	CollectedAt  time.Time              `json:"collectedAt,omitempty"`
	HealthStatus string                 `json:"healthStatus"`
	NumOSDs      int                    `json:"numOsds"`
	NumOSDsUp    int                    `json:"numOsdsUp"`
	NumOSDsIn    int                    `json:"numOsdsIn"`
	NumPGs       int                    `json:"numPGs"`
	UsagePercent float64                `json:"usagePercent"`
}

// AgentMemoryMeta describes agent-reported memory including swap.
type AgentMemoryMeta struct {
	Total     int64 `json:"total,omitempty"`
	Used      int64 `json:"used,omitempty"`
	Free      int64 `json:"free,omitempty"`
	SwapUsed  int64 `json:"swapUsed,omitempty"`
	SwapTotal int64 `json:"swapTotal,omitempty"`
}

// AgentData contains host agent-specific data.
type AgentData struct {
	AgentID           string             `json:"agentId,omitempty"`
	AgentVersion      string             `json:"agentVersion,omitempty"`
	Hostname          string             `json:"hostname,omitempty"`
	TokenID           string             `json:"tokenId,omitempty"`
	TokenName         string             `json:"tokenName,omitempty"`
	TokenHint         string             `json:"tokenHint,omitempty"`
	TokenLastUsedAt   *time.Time         `json:"tokenLastUsedAt,omitempty"`
	Platform          string             `json:"platform,omitempty"`
	OSName            string             `json:"osName,omitempty"`
	OSVersion         string             `json:"osVersion,omitempty"`
	KernelVersion     string             `json:"kernelVersion,omitempty"`
	Architecture      string             `json:"architecture,omitempty"`
	UptimeSeconds     int64              `json:"uptimeSeconds,omitempty"`
	Temperature       *float64           `json:"temperature,omitempty"` // Max CPU temp in Celsius
	NetworkInterfaces []NetworkInterface `json:"networkInterfaces,omitempty"`
	Disks             []DiskInfo         `json:"disks,omitempty"`
	Memory            *AgentMemoryMeta   `json:"memory,omitempty"`
	Sensors           *HostSensorMeta    `json:"sensors,omitempty"`
	RAID              []HostRAIDMeta     `json:"raid,omitempty"`
	Unraid            *HostUnraidMeta    `json:"unraid,omitempty"`
	DiskIO            []HostDiskIOMeta   `json:"diskIo,omitempty"`
	Ceph              *HostCephMeta      `json:"ceph,omitempty"`
	StorageRisk       *StorageRisk       `json:"storageRisk,omitempty"`
	// Internal link hints to proxmox resources.
	LinkedNodeID      string `json:"-"`
	LinkedVMID        string `json:"-"`
	LinkedContainerID string `json:"-"`
}

// DockerPortMeta describes a container port mapping.
type DockerPortMeta struct {
	PrivatePort int    `json:"privatePort"`
	PublicPort  int    `json:"publicPort,omitempty"`
	Protocol    string `json:"protocol"`
	IP          string `json:"ip,omitempty"`
}

// DockerNetworkMeta describes a container network attachment.
type DockerNetworkMeta struct {
	Name string `json:"name"`
	IPv4 string `json:"ipv4,omitempty"`
	IPv6 string `json:"ipv6,omitempty"`
}

// DockerMountMeta describes a container volume mount.
type DockerMountMeta struct {
	Type        string `json:"type,omitempty"`
	Source      string `json:"source,omitempty"`
	Destination string `json:"destination,omitempty"`
	Mode        string `json:"mode,omitempty"`
	RW          bool   `json:"rw"`
}

// DockerUpdateStatusMeta describes container image update status.
type DockerUpdateStatusMeta struct {
	UpdateAvailable bool      `json:"updateAvailable"`
	CurrentDigest   string    `json:"currentDigest,omitempty"`
	LatestDigest    string    `json:"latestDigest,omitempty"`
	LastChecked     time.Time `json:"lastChecked,omitempty"`
	Error           string    `json:"error,omitempty"`
}

// DockerServiceUpdateMeta captures service update progress.
type DockerServiceUpdateMeta struct {
	State       string     `json:"state,omitempty"`
	Message     string     `json:"message,omitempty"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
}

// DockerServicePortMeta describes a published service port.
type DockerServicePortMeta struct {
	Name          string `json:"name,omitempty"`
	Protocol      string `json:"protocol,omitempty"`
	TargetPort    uint32 `json:"targetPort,omitempty"`
	PublishedPort uint32 `json:"publishedPort,omitempty"`
	PublishMode   string `json:"publishMode,omitempty"`
}

// DockerData contains Docker host- and container-specific data.
type DockerData struct {
	HostSourceID   string   `json:"hostSourceId,omitempty"` // raw model ID for the docker host
	AgentID        string   `json:"agentId,omitempty"`
	ContainerID    string   `json:"containerId,omitempty"`
	Hostname       string   `json:"hostname,omitempty"`
	MachineID      string   `json:"machineId,omitempty"`
	Image          string   `json:"image,omitempty"`
	Temperature    *float64 `json:"temperature,omitempty"`
	Runtime        string   `json:"runtime,omitempty"`
	RuntimeVersion string   `json:"runtimeVersion,omitempty"`
	DockerVersion  string   `json:"dockerVersion,omitempty"`
	OS             string   `json:"os,omitempty"`
	KernelVersion  string   `json:"kernelVersion,omitempty"`
	Architecture   string   `json:"architecture,omitempty"`
	AgentVersion   string   `json:"agentVersion,omitempty"`
	UptimeSeconds  int64    `json:"uptimeSeconds,omitempty"`

	// Host-level summary fields (populated when Resource.Type == ResourceTypeAgent and Docker != nil)
	ContainerCount        int                             `json:"containerCount,omitempty"`
	UpdatesAvailableCount int                             `json:"updatesAvailableCount,omitempty"`
	UpdatesLastCheckedAt  *time.Time                      `json:"updatesLastCheckedAt,omitempty"`
	TokenID               string                          `json:"tokenId,omitempty"`
	TokenName             string                          `json:"tokenName,omitempty"`
	TokenHint             string                          `json:"tokenHint,omitempty"`
	TokenLastUsedAt       *time.Time                      `json:"tokenLastUsedAt,omitempty"`
	Hidden                bool                            `json:"hidden,omitempty"`
	PendingUninstall      bool                            `json:"pendingUninstall,omitempty"`
	IsLegacy              bool                            `json:"isLegacy,omitempty"`
	Command               *models.DockerHostCommandStatus `json:"command,omitempty"`

	// Container-specific fields (populated when Resource.Type == ResourceTypeAppContainer)
	ContainerState string                  `json:"containerState,omitempty"`
	Health         string                  `json:"health,omitempty"`
	RestartCount   int                     `json:"restartCount,omitempty"`
	ExitCode       int                     `json:"exitCode,omitempty"`
	Ports          []DockerPortMeta        `json:"ports,omitempty"`
	Labels         map[string]string       `json:"labels,omitempty"`
	Networks       []DockerNetworkMeta     `json:"networks,omitempty"`
	Mounts         []DockerMountMeta       `json:"mounts,omitempty"`
	UpdateStatus   *DockerUpdateStatusMeta `json:"updateStatus,omitempty"`

	// Service-specific fields (populated when Resource.Type == ResourceTypeDockerService)
	ServiceID      string                   `json:"serviceId,omitempty"`
	Stack          string                   `json:"stack,omitempty"`
	Mode           string                   `json:"mode,omitempty"`
	DesiredTasks   int                      `json:"desiredTasks,omitempty"`
	RunningTasks   int                      `json:"runningTasks,omitempty"`
	CompletedTasks int                      `json:"completedTasks,omitempty"`
	ServiceUpdate  *DockerServiceUpdateMeta `json:"serviceUpdate,omitempty"`
	EndpointPorts  []DockerServicePortMeta  `json:"endpointPorts,omitempty"`

	Swarm             *DockerSwarmInfo   `json:"swarm,omitempty"`
	NetworkInterfaces []NetworkInterface `json:"networkInterfaces,omitempty"`
	Disks             []DiskInfo         `json:"disks,omitempty"`

	// These hold raw data for tools access
	Services []models.DockerService `json:"-"`
	Tasks    []models.DockerTask    `json:"-"`
}

// PBSData contains Proxmox Backup Server data.
//
// NOTE: Some tools need per-datastore details; those are exposed via Datastores.
type PBSData struct {
	InstanceID       string             `json:"instanceId,omitempty"`
	Hostname         string             `json:"hostname,omitempty"`
	Version          string             `json:"version,omitempty"`
	UptimeSeconds    int64              `json:"uptimeSeconds,omitempty"`
	DatastoreCount   int                `json:"datastoreCount,omitempty"`
	Datastores       []PBSDatastoreMeta `json:"datastores,omitempty"`
	BackupJobCount   int                `json:"backupJobCount,omitempty"`
	SyncJobCount     int                `json:"syncJobCount,omitempty"`
	VerifyJobCount   int                `json:"verifyJobCount,omitempty"`
	PruneJobCount    int                `json:"pruneJobCount,omitempty"`
	GarbageJobCount  int                `json:"garbageJobCount,omitempty"`
	ConnectionHealth string             `json:"connectionHealth,omitempty"`
}

// PBSDatastoreMeta describes a single PBS datastore.
type PBSDatastoreMeta struct {
	Name                string  `json:"name"`
	Total               int64   `json:"total"`
	Used                int64   `json:"used"`
	Available           int64   `json:"available"`
	UsagePercent        float64 `json:"usagePercent"`
	Status              string  `json:"status"`
	Error               string  `json:"error,omitempty"`
	DeduplicationFactor float64 `json:"deduplicationFactor,omitempty"`
}

// PMGNodeMeta describes a PMG cluster node.
type PMGNodeMeta struct {
	Name        string        `json:"name"`
	Status      string        `json:"status"`
	Role        string        `json:"role,omitempty"`
	Uptime      int64         `json:"uptime,omitempty"`
	LoadAvg     string        `json:"loadAvg,omitempty"`
	QueueStatus *PMGQueueMeta `json:"queueStatus,omitempty"`
}

// PMGQueueMeta describes a PMG node's postfix queue status.
type PMGQueueMeta struct {
	Active   int `json:"active"`
	Deferred int `json:"deferred"`
	Hold     int `json:"hold"`
	Incoming int `json:"incoming"`
	Total    int `json:"total"`
}

// PMGMailStatsMeta describes PMG mail statistics.
type PMGMailStatsMeta struct {
	Timeframe            string  `json:"timeframe"`
	CountIn              float64 `json:"countIn"`
	CountOut             float64 `json:"countOut"`
	SpamIn               float64 `json:"spamIn"`
	SpamOut              float64 `json:"spamOut"`
	VirusIn              float64 `json:"virusIn"`
	VirusOut             float64 `json:"virusOut"`
	BouncesIn            float64 `json:"bouncesIn"`
	BouncesOut           float64 `json:"bouncesOut"`
	BytesIn              float64 `json:"bytesIn"`
	BytesOut             float64 `json:"bytesOut"`
	GreylistCount        float64 `json:"greylistCount"`
	RBLRejects           float64 `json:"rblRejects"`
	AverageProcessTimeMs float64 `json:"averageProcessTimeMs"`
}

// PMGQuarantineMeta describes PMG quarantine totals.
type PMGQuarantineMeta struct {
	Spam        int `json:"spam"`
	Virus       int `json:"virus"`
	Attachment  int `json:"attachment"`
	Blacklisted int `json:"blacklisted"`
}

// PMGSpamBucketMeta describes a spam score distribution bucket.
type PMGSpamBucketMeta struct {
	Bucket string  `json:"bucket"`
	Count  float64 `json:"count"`
}

// PMGRelayDomainMeta represents a relay domain configured in Proxmox Mail Gateway.
type PMGRelayDomainMeta struct {
	Domain  string `json:"domain"`
	Comment string `json:"comment,omitempty"`
}

// PMGDomainStatMeta describes mail statistics for a domain over a fixed time window
// (currently: the last 24 hours at poll time).
type PMGDomainStatMeta struct {
	Domain     string  `json:"domain"`
	MailCount  float64 `json:"mailCount"`
	SpamCount  float64 `json:"spamCount"`
	VirusCount float64 `json:"virusCount"`
	Bytes      float64 `json:"bytes,omitempty"`
}

// PMGData contains Proxmox Mail Gateway data.
type PMGData struct {
	InstanceID       string    `json:"instanceId,omitempty"`
	Hostname         string    `json:"hostname,omitempty"`
	Version          string    `json:"version,omitempty"`
	NodeCount        int       `json:"nodeCount,omitempty"`
	UptimeSeconds    int64     `json:"uptimeSeconds,omitempty"`
	QueueActive      int       `json:"queueActive,omitempty"`
	QueueDeferred    int       `json:"queueDeferred,omitempty"`
	QueueHold        int       `json:"queueHold,omitempty"`
	QueueIncoming    int       `json:"queueIncoming,omitempty"`
	QueueTotal       int       `json:"queueTotal,omitempty"`
	MailCountTotal   float64   `json:"mailCountTotal,omitempty"`
	SpamIn           float64   `json:"spamIn,omitempty"`
	VirusIn          float64   `json:"virusIn,omitempty"`
	ConnectionHealth string    `json:"connectionHealth,omitempty"`
	LastUpdated      time.Time `json:"lastUpdated,omitempty"`

	Nodes            []PMGNodeMeta        `json:"nodes,omitempty"`
	MailStats        *PMGMailStatsMeta    `json:"mailStats,omitempty"`
	Quarantine       *PMGQuarantineMeta   `json:"quarantine,omitempty"`
	SpamDistribution []PMGSpamBucketMeta  `json:"spamDistribution,omitempty"`
	RelayDomains     []PMGRelayDomainMeta `json:"relayDomains,omitempty"`
	DomainStats      []PMGDomainStatMeta  `json:"domainStats,omitempty"`
	DomainStatsAsOf  time.Time            `json:"domainStatsAsOf,omitempty"`
}

// TrueNASData contains TrueNAS-specific metadata for system host resources.
type TrueNASData struct {
	Hostname      string `json:"hostname,omitempty"`
	Version       string `json:"version,omitempty"`
	UptimeSeconds int64  `json:"uptimeSeconds,omitempty"`
}

// K8sMetricCapabilities describes which Kubernetes metric families are available
// for this cluster right now based on active collection paths.
type K8sMetricCapabilities struct {
	NodeCPUMemory    bool `json:"nodeCpuMemory"`
	NodeTelemetry    bool `json:"nodeTelemetry"`
	PodCPUMemory     bool `json:"podCpuMemory"`
	PodNetwork       bool `json:"podNetwork"`
	PodEphemeralDisk bool `json:"podEphemeralDisk"`
	PodDiskIO        bool `json:"podDiskIo"`
}

// K8sData contains Kubernetes data.
type K8sData struct {
	ClusterID               string                 `json:"clusterId,omitempty"`
	ClusterName             string                 `json:"clusterName,omitempty"`
	SourceName              string                 `json:"sourceName,omitempty"`   // raw model Name before display-name resolution
	SourceStatus            string                 `json:"sourceStatus,omitempty"` // raw model Status before normalization
	AgentID                 string                 `json:"agentId,omitempty"`
	Context                 string                 `json:"context,omitempty"`
	Server                  string                 `json:"server,omitempty"`
	Version                 string                 `json:"version,omitempty"`
	PendingUninstall        bool                   `json:"pendingUninstall,omitempty"`
	AgentVersion            string                 `json:"agentVersion,omitempty"`    // cluster: k8s agent version
	IntervalSeconds         int                    `json:"intervalSeconds,omitempty"` // cluster: telemetry interval
	NodeUID                 string                 `json:"nodeUid,omitempty"`
	NodeName                string                 `json:"nodeName,omitempty"`
	Ready                   bool                   `json:"ready,omitempty"`
	Unschedulable           bool                   `json:"unschedulable,omitempty"`
	Roles                   []string               `json:"roles,omitempty"`
	KubeletVersion          string                 `json:"kubeletVersion,omitempty"`
	ContainerRuntimeVersion string                 `json:"containerRuntimeVersion,omitempty"`
	OSImage                 string                 `json:"osImage,omitempty"`
	KernelVersion           string                 `json:"kernelVersion,omitempty"`
	Architecture            string                 `json:"architecture,omitempty"`
	CapacityCPU             int64                  `json:"capacityCpuCores,omitempty"`
	CapacityMemoryBytes     int64                  `json:"capacityMemoryBytes,omitempty"`
	CapacityPods            int64                  `json:"capacityPods,omitempty"`
	AllocCPU                int64                  `json:"allocatableCpuCores,omitempty"`
	AllocMemoryBytes        int64                  `json:"allocatableMemoryBytes,omitempty"`
	AllocPods               int64                  `json:"allocatablePods,omitempty"`
	Namespace               string                 `json:"namespace,omitempty"`
	PodUID                  string                 `json:"podUid,omitempty"`
	PodPhase                string                 `json:"podPhase,omitempty"`
	PodReason               string                 `json:"podReason,omitempty"`     // pod: status reason
	PodMessage              string                 `json:"podMessage,omitempty"`    // pod: status message
	PodContainers           []K8sPodContainer      `json:"podContainers,omitempty"` // pod: sub-containers
	UptimeSeconds           int64                  `json:"uptimeSeconds,omitempty"`
	Temperature             *float64               `json:"temperature,omitempty"`
	Restarts                int                    `json:"restarts,omitempty"`
	OwnerKind               string                 `json:"ownerKind,omitempty"`
	OwnerName               string                 `json:"ownerName,omitempty"`
	Image                   string                 `json:"image,omitempty"`
	Labels                  map[string]string      `json:"labels,omitempty"`
	DeploymentUID           string                 `json:"deploymentUid,omitempty"`
	DesiredReplicas         int32                  `json:"desiredReplicas,omitempty"`
	UpdatedReplicas         int32                  `json:"updatedReplicas,omitempty"`
	ReadyReplicas           int32                  `json:"readyReplicas,omitempty"`
	AvailableReplicas       int32                  `json:"availableReplicas,omitempty"`
	MetricCapabilities      *K8sMetricCapabilities `json:"metricCapabilities,omitempty"`
}

// K8sPodContainer describes a container within a Kubernetes pod.
type K8sPodContainer struct {
	Name         string `json:"name"`
	Image        string `json:"image,omitempty"`
	Ready        bool   `json:"ready"`
	RestartCount int32  `json:"restartCount,omitempty"`
	State        string `json:"state,omitempty"`
	Reason       string `json:"reason,omitempty"`
	Message      string `json:"message,omitempty"`
}

// CPUInfo describes CPU characteristics.
type CPUInfo struct {
	Model   string `json:"model,omitempty"`
	Cores   int    `json:"cores,omitempty"`
	Sockets int    `json:"sockets,omitempty"`
}

// NetworkInterface describes a network interface.
type NetworkInterface struct {
	Name      string   `json:"name"`
	MAC       string   `json:"mac,omitempty"`
	Addresses []string `json:"addresses,omitempty"`
	RXBytes   uint64   `json:"rxBytes,omitempty"`
	TXBytes   uint64   `json:"txBytes,omitempty"`
	SpeedMbps *int64   `json:"speedMbps,omitempty"`
	Status    string   `json:"status,omitempty"`
}

// DiskInfo describes disk usage.
type DiskInfo struct {
	Device     string  `json:"device,omitempty"`
	Mountpoint string  `json:"mountpoint,omitempty"`
	Filesystem string  `json:"filesystem,omitempty"`
	Total      int64   `json:"total,omitempty"`
	Used       int64   `json:"used,omitempty"`
	Free       int64   `json:"free,omitempty"`
	Usage      float64 `json:"usage,omitempty"`
}

// DockerSwarmInfo captures Docker Swarm details.
type DockerSwarmInfo struct {
	NodeID           string `json:"nodeId,omitempty"`
	NodeRole         string `json:"nodeRole,omitempty"`
	LocalState       string `json:"localState,omitempty"`
	ControlAvailable bool   `json:"controlAvailable,omitempty"`
	ClusterID        string `json:"clusterId,omitempty"`
	ClusterName      string `json:"clusterName,omitempty"`
	Scope            string `json:"scope,omitempty"`
	Error            string `json:"error,omitempty"`
}

// ResourceStats contains aggregated stats for a set of resources.
type ResourceStats struct {
	Total    int                    `json:"total"`
	ByType   map[ResourceType]int   `json:"byType"`
	ByStatus map[ResourceStatus]int `json:"byStatus"`
	BySource map[DataSource]int     `json:"bySource"`
}
