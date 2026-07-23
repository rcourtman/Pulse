package unifiedresources

import (
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
	"github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"
	"github.com/rcourtman/pulse-go-rewrite/pkg/diskinventory"
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

	DiscoveryTarget    *DiscoveryTarget            `json:"discoveryTarget,omitempty"`
	DiscoveryReadiness *ResourceDiscoveryReadiness `json:"discoveryReadiness,omitempty"`
	MetricsTarget      *MetricsTarget              `json:"metricsTarget,omitempty"`
	Canonical          *CanonicalIdentity          `json:"canonicalIdentity,omitempty"`
	// SupersededCanonicalIDs carries retired canonical IDs that are known to
	// identify this same resource. Providers declare them on IngestRecord when
	// an identity derivation changes; the registry retains them so alert
	// overrides and other operator-authored configuration can migrate onto the
	// current canonical ID instead of becoming orphaned.
	SupersededCanonicalIDs []string        `json:"-"`
	Policy                 *ResourcePolicy `json:"policy,omitempty"`
	AISafeSummary          string          `json:"aiSafeSummary,omitempty"`
	PlatformScopes         []string        `json:"platformScopes,omitempty"`

	Sources      []DataSource                `json:"sources"`
	SourceStatus map[DataSource]SourceStatus `json:"sourceStatus,omitempty"`

	Identity ResourceIdentity `json:"identity,omitempty"`
	Metrics  *ResourceMetrics `json:"metrics,omitempty"`

	// Surface-friendly projections of the nested source payloads that the
	// frontend infrastructure table reads directly. Adapters that wrap an
	// `AgentData` (Pulse-managed host) or `ProxmoxData` (Proxmox node)
	// project the runtime uptime and the max-sensor temperature here so
	// the canonical table renders real values instead of dashes for
	// agent-backed rows. Resources that have no native uptime/temperature
	// concept (e.g. k8s-deployment, docker-service) leave these unset.
	Uptime      int64    `json:"uptime,omitempty"`
	Temperature *float64 `json:"temperature,omitempty"`

	ParentID       *string `json:"parentId,omitempty"`
	ParentName     string  `json:"parentName,omitempty"`
	ChildCount     int     `json:"childCount,omitempty"`
	parentBySource map[DataSource]string

	Tags                  []string                  `json:"tags,omitempty"`
	CustomURL             string                    `json:"customUrl,omitempty"`
	Capabilities          []ResourceCapability      `json:"capabilities,omitempty"`
	ActionReadiness       []ResourceActionReadiness `json:"actionReadiness,omitempty"`
	Relationships         []ResourceRelationship    `json:"relationships,omitempty"`
	RecentChanges         []ResourceChange          `json:"recentChanges,omitempty"`
	FacetCounts           ResourceFacetCounts       `json:"facetCounts,omitempty"`
	Incidents             []ResourceIncident        `json:"incidents,omitempty"`
	IncidentCount         int                       `json:"incidentCount,omitempty"`
	IncidentCode          string                    `json:"incidentCode,omitempty"`
	IncidentSeverity      storagehealth.RiskLevel   `json:"incidentSeverity,omitempty"`
	IncidentSummary       string                    `json:"incidentSummary,omitempty"`
	IncidentCategory      string                    `json:"incidentCategory,omitempty"`
	IncidentLabel         string                    `json:"incidentLabel,omitempty"`
	IncidentPriority      int                       `json:"incidentPriority,omitempty"`
	IncidentImpactSummary string                    `json:"incidentImpactSummary,omitempty"`
	IncidentUrgency       string                    `json:"incidentUrgency,omitempty"`
	IncidentAction        string                    `json:"incidentAction,omitempty"`

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
	VMware       *VMwareData       `json:"vmware,omitempty"`
	Availability *AvailabilityData `json:"availability,omitempty"`
	// AvailabilityChecks is the canonical plural facet. Availability remains
	// the compatibility summary chosen from this set for existing consumers.
	AvailabilityChecks []AvailabilityData `json:"availabilityChecks,omitempty"`
}

// ResourceFacetCounts captures the total count of each resource facet that
// may be surfaced in row summaries or detail drawers.
type ResourceFacetCounts struct {
	RecentChanges              int                         `json:"recentChanges"`
	AvailabilityChecks         int                         `json:"availabilityChecks,omitempty"`
	RecentChangeKinds          map[ChangeKind]int          `json:"recentChangeKinds,omitempty"`
	RecentChangeSourceTypes    map[ChangeSourceType]int    `json:"recentChangeSourceTypes,omitempty"`
	RecentChangeSourceAdapters map[ChangeSourceAdapter]int `json:"recentChangeSourceAdapters,omitempty"`
}

// DiscoveryTarget describes the canonical discovery request coordinates
// for this unified resource.
type DiscoveryTarget struct {
	ResourceType string `json:"resourceType"`
	AgentID      string `json:"agentId"`
	ResourceID   string `json:"resourceId"`
	Hostname     string `json:"hostname,omitempty"`
}

// ResourceDiscoveryReadiness summarizes whether service discovery has useful,
// recent grounding for a resource. It is intentionally metadata-only: raw
// command output, environment variables, config files, and secrets do not
// belong in this projection.
type ResourceDiscoveryReadiness struct {
	State             ResourceDiscoveryReadinessState `json:"state"`
	Reason            string                          `json:"reason,omitempty"`
	Source            string                          `json:"source,omitempty"`
	ResourceType      string                          `json:"resourceType,omitempty"`
	TargetID          string                          `json:"targetId,omitempty"`
	ResourceID        string                          `json:"resourceId,omitempty"`
	DiscoveryID       string                          `json:"discoveryId,omitempty"`
	ObservedAt        *time.Time                      `json:"observedAt,omitempty"`
	GeneratedAt       time.Time                       `json:"generatedAt"`
	AgeSeconds        int64                           `json:"ageSeconds,omitempty"`
	StaleAfterSeconds int64                           `json:"staleAfterSeconds,omitempty"`
	FactCount         int                             `json:"factCount,omitempty"`
	ServiceName       string                          `json:"serviceName,omitempty"`
	ServiceCategory   string                          `json:"serviceCategory,omitempty"`
	Confidence        float64                         `json:"confidence,omitempty"`
}

// ResourceDiscoveryReadinessState is the canonical state vocabulary shared by
// API payloads, Assistant handoffs, and frontend badges.
type ResourceDiscoveryReadinessState string

const (
	ResourceDiscoveryReadinessFresh       ResourceDiscoveryReadinessState = "fresh"
	ResourceDiscoveryReadinessStale       ResourceDiscoveryReadinessState = "stale"
	ResourceDiscoveryReadinessMissing     ResourceDiscoveryReadinessState = "missing"
	ResourceDiscoveryReadinessRunning     ResourceDiscoveryReadinessState = "running"
	ResourceDiscoveryReadinessFailed      ResourceDiscoveryReadinessState = "failed"
	ResourceDiscoveryReadinessUnavailable ResourceDiscoveryReadinessState = "unavailable"
	ResourceDiscoveryReadinessUnsupported ResourceDiscoveryReadinessState = "unsupported"
)

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
	DisplayName   string   `json:"displayName,omitempty"`
	Hostname      string   `json:"hostname,omitempty"`
	PlatformID    string   `json:"platformId,omitempty"`
	PrimaryID     string   `json:"primaryId,omitempty"`
	Aliases       []string `json:"aliases,omitempty"`
	SupersededIDs []string `json:"supersededIds,omitempty"`
}

// ResourceType represents the kind of resource.
type ResourceType string

const (
	// ResourceTypeAgent is the canonical v6 infrastructure parent type.
	ResourceTypeAgent                 ResourceType = "agent"
	ResourceTypeVM                    ResourceType = "vm"
	ResourceTypeSystemContainer       ResourceType = "system-container"
	ResourceTypeAppContainer          ResourceType = "app-container"
	ResourceTypeDockerService         ResourceType = "docker-service"
	ResourceTypeDockerImage           ResourceType = "docker-image"
	ResourceTypeDockerVolume          ResourceType = "docker-volume"
	ResourceTypeDockerNetwork         ResourceType = "docker-network"
	ResourceTypeDockerTask            ResourceType = "docker-task"
	ResourceTypeDockerSwarmNode       ResourceType = "docker-swarm-node"
	ResourceTypeDockerSecret          ResourceType = "docker-secret"
	ResourceTypeDockerConfig          ResourceType = "docker-config"
	ResourceTypeK8sCluster            ResourceType = "k8s-cluster"
	ResourceTypeK8sNode               ResourceType = "k8s-node"
	ResourceTypePod                   ResourceType = "pod"
	ResourceTypeK8sDeployment         ResourceType = "k8s-deployment"
	ResourceTypeK8sReplicaSet         ResourceType = "k8s-replicaset"
	ResourceTypeK8sNamespace          ResourceType = "k8s-namespace"
	ResourceTypeK8sService            ResourceType = "k8s-service"
	ResourceTypeK8sStatefulSet        ResourceType = "k8s-statefulset"
	ResourceTypeK8sDaemonSet          ResourceType = "k8s-daemonset"
	ResourceTypeK8sJob                ResourceType = "k8s-job"
	ResourceTypeK8sCronJob            ResourceType = "k8s-cronjob"
	ResourceTypeK8sIngress            ResourceType = "k8s-ingress"
	ResourceTypeK8sEndpointSlice      ResourceType = "k8s-endpoint-slice"
	ResourceTypeK8sNetworkPolicy      ResourceType = "k8s-network-policy"
	ResourceTypeK8sPV                 ResourceType = "k8s-persistent-volume"
	ResourceTypeK8sPVC                ResourceType = "k8s-persistent-volume-claim"
	ResourceTypeK8sStorageClass       ResourceType = "k8s-storage-class"
	ResourceTypeK8sConfigMap          ResourceType = "k8s-configmap"
	ResourceTypeK8sSecret             ResourceType = "k8s-secret"
	ResourceTypeK8sServiceAccount     ResourceType = "k8s-serviceaccount"
	ResourceTypeK8sRole               ResourceType = "k8s-role"
	ResourceTypeK8sClusterRole        ResourceType = "k8s-cluster-role"
	ResourceTypeK8sRoleBinding        ResourceType = "k8s-role-binding"
	ResourceTypeK8sClusterRoleBinding ResourceType = "k8s-cluster-role-binding"
	ResourceTypeK8sResourceQuota      ResourceType = "k8s-resource-quota"
	ResourceTypeK8sLimitRange         ResourceType = "k8s-limit-range"
	ResourceTypeK8sPDB                ResourceType = "k8s-pod-disruption-budget"
	ResourceTypeK8sHPA                ResourceType = "k8s-horizontal-pod-autoscaler"
	ResourceTypeK8sEvent              ResourceType = "k8s-event"
	ResourceTypeStorage               ResourceType = "storage"
	ResourceTypeNetwork               ResourceType = "network"
	ResourceTypePBS                   ResourceType = "pbs"
	ResourceTypePMG                   ResourceType = "pmg"
	ResourceTypeCeph                  ResourceType = "ceph"
	ResourceTypePhysicalDisk          ResourceType = "physical_disk"
	ResourceTypeNetworkShare          ResourceType = "network-share"
	ResourceTypeNetworkEndpoint       ResourceType = "network-endpoint"
)

// CanonicalResourceType normalizes resource type spellings into the internal
// canonical enum value.
func CanonicalResourceType(rt ResourceType) ResourceType {
	normalized := ResourceType(strings.ToLower(strings.TrimSpace(string(rt))))
	return normalized
}

// ContractResourceType maps an internal unified resource onto the canonical
// external transport type used by REST and websocket/state payloads.
func ContractResourceType(resource Resource) ResourceType {
	switch CanonicalResourceType(resource.Type) {
	case ResourceTypeAgent:
		if resource.Proxmox != nil || resource.Agent != nil || resource.TrueNAS != nil || resource.VMware != nil {
			return ResourceTypeAgent
		}
		if resource.Docker != nil {
			return ResourceType("docker-host")
		}
		return ResourceTypeAgent
	case ResourceTypeSystemContainer:
		return ResourceTypeSystemContainer
	case ResourceTypeAppContainer:
		return ResourceTypeAppContainer
	default:
		return CanonicalResourceType(resource.Type)
	}
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
	SourceProxmox      DataSource = "proxmox"
	SourceAgent        DataSource = "agent"
	SourceDocker       DataSource = "docker"
	SourcePBS          DataSource = "pbs"
	SourcePMG          DataSource = "pmg"
	SourceK8s          DataSource = "kubernetes"
	SourceTrueNAS      DataSource = "truenas"
	SourceVMware       DataSource = "vmware"
	SourceAvailability DataSource = "availability"
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
	SourceID                     string              `json:"sourceId,omitempty"` // raw model ID from source snapshot
	NodeName                     string              `json:"nodeName,omitempty"`
	Pool                         string              `json:"pool,omitempty"`
	ClusterName                  string              `json:"clusterName,omitempty"`
	IsClusterMember              bool                `json:"isClusterMember,omitempty"`
	Instance                     string              `json:"instance,omitempty"`
	HostURL                      string              `json:"host,omitempty"`
	GuestURL                     string              `json:"guestUrl,omitempty"`
	ConnectionHealth             string              `json:"connectionHealth,omitempty"`
	VMID                         int                 `json:"vmid,omitempty"`
	ContainerType                string              `json:"containerType,omitempty"`
	IsOCI                        bool                `json:"isOci,omitempty"`
	CPUs                         int                 `json:"cpus,omitempty"`
	Template                     bool                `json:"template,omitempty"`
	Temperature                  *float64            `json:"temperature,omitempty"` // Max node CPU temp in Celsius
	TemperatureDetails           *models.Temperature `json:"temperatureDetails,omitempty"`
	PVEVersion                   string              `json:"pveVersion,omitempty"`
	KernelVersion                string              `json:"kernelVersion,omitempty"`
	Uptime                       int64               `json:"uptime,omitempty"`
	LastBackup                   time.Time           `json:"lastBackup,omitempty"`
	DiskStatusReason             string              `json:"diskStatusReason,omitempty"`
	GuestAgentStatus             string              `json:"guestAgentStatus,omitempty"`
	GuestAgentExpected           bool                `json:"guestAgentExpected,omitempty"`
	OSName                       string              `json:"osName,omitempty"`
	OSVersion                    string              `json:"osVersion,omitempty"`
	AgentVersion                 string              `json:"agentVersion,omitempty"`
	NetworkInterfaces            []NetworkInterface  `json:"networkInterfaces,omitempty"`
	OSTemplate                   string              `json:"osTemplate,omitempty"`
	HasDocker                    bool                `json:"hasDocker,omitempty"`
	DockerCheckedAt              *time.Time          `json:"dockerCheckedAt,omitempty"`
	CPUInfo                      *CPUInfo            `json:"cpuInfo,omitempty"`
	LoadAverage                  []float64           `json:"loadAverage,omitempty"`
	PendingUpdates               int                 `json:"pendingUpdates,omitempty"`
	TemperatureMonitoringEnabled *bool               `json:"temperatureMonitoringEnabled,omitempty"`
	PendingUpdatesCheckedAt      *time.Time          `json:"pendingUpdatesCheckedAt,omitempty"`
	Disks                        []DiskInfo          `json:"disks,omitempty"`
	SwapUsed                     int64               `json:"swapUsed,omitempty"`
	SwapTotal                    int64               `json:"swapTotal,omitempty"`
	Balloon                      int64               `json:"balloon,omitempty"`
	// Reclaimable buff/cache split out of the memory metric's free bytes.
	MemoryCache int64          `json:"memoryCache,omitempty"`
	Memory      *models.Memory `json:"memory,omitempty"`
	Lock        string         `json:"lock,omitempty"` // Proxmox lock state (e.g. "backup", "migrate", "snapshot")
	// Internal link hint to a host agent resource.
	LinkedAgentID string `json:"-"`
}

// StorageMeta contains storage-specific metadata for storage resources.
type StorageMeta struct {
	Type                  string                `json:"type,omitempty"`
	Content               string                `json:"content,omitempty"`
	ContentTypes          []string              `json:"contentTypes,omitempty"`
	Shared                bool                  `json:"shared"`
	Enabled               bool                  `json:"enabled"`
	Active                bool                  `json:"active"`
	IsCeph                bool                  `json:"isCeph"`
	IsZFS                 bool                  `json:"isZfs"`
	Platform              string                `json:"platform,omitempty"`
	Topology              string                `json:"topology,omitempty"`
	Protection            string                `json:"protection,omitempty"`
	Risk                  *StorageRisk          `json:"risk,omitempty"`
	RiskSummary           string                `json:"riskSummary,omitempty"`
	ConsumerCount         int                   `json:"consumerCount,omitempty"`
	ConsumerTypes         []string              `json:"consumerTypes,omitempty"`
	TopConsumers          []StorageConsumerMeta `json:"topConsumers,omitempty"`
	ConsumerImpactSummary string                `json:"consumerImpactSummary,omitempty"`
	PostureSummary        string                `json:"postureSummary,omitempty"`
	ProtectionReduced     bool                  `json:"protectionReduced,omitempty"`
	ProtectionSummary     string                `json:"protectionSummary,omitempty"`
	RebuildInProgress     bool                  `json:"rebuildInProgress,omitempty"`
	RebuildSummary        string                `json:"rebuildSummary,omitempty"`

	// Accessibility metadata.
	Nodes []string `json:"nodes,omitempty"` // PVE nodes where this storage is accessible
	Pool  string   `json:"pool,omitempty"`  // Proxmox backing pool/dataset (for example rpool/data)
	Path  string   `json:"path,omitempty"`  // local mount path on the node

	// ZFS metadata (when IsZFS is true and the source provides details).
	// ZFSPool carries the full pool report (scan status, per-device states
	// and error counts); the scalar fields below are the flattened summary
	// kept for consumers that only need pool-level state.
	ZFSPool           *models.ZFSPool `json:"zfsPool,omitempty"`
	ZFSPoolState      string          `json:"zfsPoolState,omitempty"`
	ZFSReadErrors     int64           `json:"zfsReadErrors,omitempty"`
	ZFSWriteErrors    int64           `json:"zfsWriteErrors,omitempty"`
	ZFSChecksumErrors int64           `json:"zfsChecksumErrors,omitempty"`

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

type ResourceIncident struct {
	Provider  string                  `json:"provider,omitempty"`
	NativeID  string                  `json:"nativeId,omitempty"`
	Code      string                  `json:"code"`
	Severity  storagehealth.RiskLevel `json:"severity"`
	Source    string                  `json:"source,omitempty"`
	Summary   string                  `json:"summary"`
	StartedAt time.Time               `json:"startedAt,omitempty"`
}

// PhysicalDiskMeta contains physical disk-specific metadata.
type PhysicalDiskMeta struct {
	DevPath              string                          `json:"devPath"`
	Model                string                          `json:"model,omitempty"`
	Vendor               string                          `json:"vendor,omitempty"`
	Serial               string                          `json:"serial,omitempty"`
	WWN                  string                          `json:"wwn,omitempty"`
	DiskType             string                          `json:"diskType"` // nvme, sata, sas
	Controller           string                          `json:"controller,omitempty"`
	Target               string                          `json:"target,omitempty"`
	SizeBytes            int64                           `json:"sizeBytes"`
	Health               string                          `json:"health"`      // PASSED, FAILED, UNKNOWN
	Wearout              int                             `json:"wearout"`     // 0-100, -1 unavailable
	Temperature          int                             `json:"temperature"` // Celsius
	TemperatureAggregate *TemperatureAggregateMeta       `json:"temperatureAggregate,omitempty"`
	RPM                  int                             `json:"rpm"`
	Used                 string                          `json:"used,omitempty"`
	StorageRole          string                          `json:"storageRole,omitempty"`
	StorageGroup         string                          `json:"storageGroup,omitempty"`
	StorageState         string                          `json:"storageState,omitempty"`
	SpunDown             bool                            `json:"spunDown,omitempty"`
	ReadCount            int64                           `json:"readCount,omitempty"`
	WriteCount           int64                           `json:"writeCount,omitempty"`
	ErrorCount           int64                           `json:"errorCount,omitempty"`
	IO                   *PhysicalDiskIOMeta             `json:"io,omitempty"`
	Collection           *diskinventory.CollectionStatus `json:"collection,omitempty"`
	SMART                *SMARTMeta                      `json:"smart,omitempty"`
	Risk                 *PhysicalDiskRisk               `json:"risk,omitempty"`
}

// PhysicalDiskIOMeta preserves the cumulative kernel counters attributed to a
// physical disk. Rates are derived separately by the monitoring layer.
type PhysicalDiskIOMeta struct {
	Device      string `json:"device,omitempty"`
	ReadBytes   uint64 `json:"readBytes,omitempty"`
	WriteBytes  uint64 `json:"writeBytes,omitempty"`
	ReadOps     uint64 `json:"readOps,omitempty"`
	WriteOps    uint64 `json:"writeOps,omitempty"`
	ReadTimeMs  uint64 `json:"readTimeMs,omitempty"`
	WriteTimeMs uint64 `json:"writeTimeMs,omitempty"`
	IOTimeMs    uint64 `json:"ioTimeMs,omitempty"`
}

// TemperatureAggregateMeta stores recent aggregate temperature history for a
// resource sensor where the provider can supply min/avg/max readings.
type TemperatureAggregateMeta struct {
	WindowDays int     `json:"windowDays,omitempty"`
	MinCelsius float64 `json:"minCelsius,omitempty"`
	AvgCelsius float64 `json:"avgCelsius,omitempty"`
	MaxCelsius float64 `json:"maxCelsius,omitempty"`
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

// HostSensorMeta contains host sensor readings.
type HostSensorMeta struct {
	TemperatureCelsius map[string]float64 `json:"temperatureCelsius,omitempty"`
	FanRPM             map[string]float64 `json:"fanRpm,omitempty"`
	PowerWatts         map[string]float64 `json:"powerWatts,omitempty"`
	Additional         map[string]float64 `json:"additional,omitempty"`
	GPU                []HostGPUSensor    `json:"gpu,omitempty"`
	ThermalState       *HostThermalState  `json:"thermalState,omitempty"`
	SMART              []HostSMARTMeta    `json:"smart,omitempty"`
}

type HostGPUSensor struct {
	ID                 string   `json:"id,omitempty"`
	Name               string   `json:"name,omitempty"`
	TemperatureCelsius *float64 `json:"temperatureCelsius,omitempty"`
	UtilizationPercent *float64 `json:"utilizationPercent,omitempty"`
	MemoryUsedBytes    *int64   `json:"memoryUsedBytes,omitempty"`
	MemoryTotalBytes   *int64   `json:"memoryTotalBytes,omitempty"`
}

type HostThermalState struct {
	Source                  string         `json:"source,omitempty"`
	Pressure                string         `json:"pressure,omitempty"`
	ThermalWarningLevel     *int           `json:"thermalWarningLevel,omitempty"`
	PerformanceWarningLevel *int           `json:"performanceWarningLevel,omitempty"`
	CPUPowerStatus          *int           `json:"cpuPowerStatus,omitempty"`
	LimitsPercent           map[string]int `json:"limitsPercent,omitempty"`
}

// HostSMARTMeta describes a disk's SMART data.
type HostSMARTMeta struct {
	Device      string                          `json:"device"`
	Model       string                          `json:"model,omitempty"`
	Serial      string                          `json:"serial,omitempty"`
	WWN         string                          `json:"wwn,omitempty"`
	Type        string                          `json:"type,omitempty"`
	Controller  string                          `json:"controller,omitempty"`
	Target      string                          `json:"target,omitempty"`
	SizeBytes   int64                           `json:"sizeBytes,omitempty"`
	Temperature int                             `json:"temperature"`
	Health      string                          `json:"health"`
	Standby     bool                            `json:"standby,omitempty"`
	Pool        string                          `json:"pool,omitempty"`
	IO          *PhysicalDiskIOMeta             `json:"io,omitempty"`
	Collection  *diskinventory.CollectionStatus `json:"collection,omitempty"`
	Attributes  *models.SMARTAttributes         `json:"attributes,omitempty"`
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
	Operation      string               `json:"operation,omitempty"`
	Risk           *StorageRisk         `json:"risk,omitempty"`
}

// HostUnraidDiskMeta describes a disk's role and state inside an Unraid array.
type HostUnraidDiskMeta struct {
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

// HostUnraidMeta describes Unraid array topology from a host agent.
type HostUnraidMeta struct {
	ArrayStarted      bool                 `json:"arrayStarted"`
	ArrayState        string               `json:"arrayState,omitempty"`
	SyncAction        string               `json:"syncAction,omitempty"`
	SyncProgress      float64              `json:"syncProgress,omitempty"`
	SyncErrors        int64                `json:"syncErrors,omitempty"`
	NumProtected      int                  `json:"numProtected,omitempty"`
	NumDisabled       int                  `json:"numDisabled,omitempty"`
	NumInvalid        int                  `json:"numInvalid,omitempty"`
	NumMissing        int                  `json:"numMissing,omitempty"`
	Disks             []HostUnraidDiskMeta `json:"disks,omitempty"`
	Risk              *StorageRisk         `json:"risk,omitempty"`
	RiskSummary       string               `json:"riskSummary,omitempty"`
	PostureSummary    string               `json:"postureSummary,omitempty"`
	ProtectionReduced bool                 `json:"protectionReduced,omitempty"`
	ProtectionSummary string               `json:"protectionSummary,omitempty"`
	RebuildInProgress bool                 `json:"rebuildInProgress,omitempty"`
	RebuildSummary    string               `json:"rebuildSummary,omitempty"`
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
	Total int64 `json:"total,omitempty"`
	Used  int64 `json:"used,omitempty"`
	Free  int64 `json:"free,omitempty"`
	// Cache is the reclaimable page cache; used + cache + free ≈ total.
	Cache            int64 `json:"cache,omitempty"`
	UsageUnavailable bool  `json:"usageUnavailable,omitempty"`
	SwapUsed         int64 `json:"swapUsed,omitempty"`
	SwapTotal        int64 `json:"swapTotal,omitempty"`
}

const HostPackageUpdateFreshness = 45 * time.Minute

const (
	HostStorageCleanupFreshness           = 45 * time.Minute
	HostStoragePressureThreshold          = 90.0
	HostStorageCleanupMinReclaimableBytes = int64(64 * 1024 * 1024)
)

type AgentPackageUpdateMeta struct {
	Supported      bool                 `json:"supported"`
	Manager        string               `json:"manager,omitempty"`
	InventoryHash  string               `json:"inventoryHash,omitempty"`
	PendingCount   int                  `json:"pendingCount"`
	Packages       []AgentPackageUpdate `json:"packages,omitempty"`
	CheckedAt      time.Time            `json:"checkedAt,omitempty"`
	ObservedAt     time.Time            `json:"observedAt,omitempty"`
	RebootRequired bool                 `json:"rebootRequired,omitempty"`
	Error          string               `json:"error,omitempty"`
}

type AgentPackageUpdate struct {
	Name             string `json:"name"`
	InstalledVersion string `json:"installedVersion,omitempty"`
	AvailableVersion string `json:"availableVersion,omitempty"`
}

type AgentStorageCleanupMeta struct {
	Supported        bool      `json:"supported"`
	Provider         string    `json:"provider,omitempty"`
	Fingerprint      string    `json:"fingerprint,omitempty"`
	ReclaimableBytes int64     `json:"reclaimableBytes"`
	CheckedAt        time.Time `json:"checkedAt,omitempty"`
	ObservedAt       time.Time `json:"observedAt,omitempty"`
	Error            string    `json:"error,omitempty"`
}

// AgentData contains host agent-specific data.
type AgentData struct {
	AgentID      string `json:"agentId,omitempty"`
	AgentVersion string `json:"agentVersion,omitempty"`
	// Stale is set when the agent has stopped reporting (its host was marked
	// offline by the staleness evaluator) even though the row itself may stay
	// online via another source such as the Proxmox API poll. It lets the UI
	// present the agent and its version as not-reporting instead of a
	// healthy-looking stale value.
	Stale bool `json:"stale,omitempty"`
	// LastReportAt is the agent's own last successful report time. On a
	// multi-source row (for example a Proxmox node also polled over the PVE
	// API) this differs from the row's LastSeen, which reflects the freshest
	// source rather than the agent.
	LastReportAt            *time.Time               `json:"lastReportAt,omitempty"`
	Hostname                string                   `json:"hostname,omitempty"`
	MachineID               string                   `json:"machineId,omitempty"`
	TokenID                 string                   `json:"tokenId,omitempty"`
	TokenName               string                   `json:"tokenName,omitempty"`
	TokenHint               string                   `json:"tokenHint,omitempty"`
	TokenLastUsedAt         *time.Time               `json:"tokenLastUsedAt,omitempty"`
	Platform                string                   `json:"platform,omitempty"`
	HostProfile             string                   `json:"hostProfile,omitempty"`
	OSName                  string                   `json:"osName,omitempty"`
	OSVersion               string                   `json:"osVersion,omitempty"`
	KernelVersion           string                   `json:"kernelVersion,omitempty"`
	Architecture            string                   `json:"architecture,omitempty"`
	CPUCount                int                      `json:"cpuCount,omitempty"`
	LoadAverage             []float64                `json:"loadAverage,omitempty"`
	UptimeSeconds           int64                    `json:"uptimeSeconds,omitempty"`
	IntervalSeconds         int                      `json:"intervalSeconds,omitempty"`
	Temperature             *float64                 `json:"temperature,omitempty"` // Max CPU temp in Celsius
	NetworkInterfaces       []NetworkInterface       `json:"networkInterfaces,omitempty"`
	Disks                   []DiskInfo               `json:"disks,omitempty"`
	Memory                  *AgentMemoryMeta         `json:"memory,omitempty"`
	Sensors                 *HostSensorMeta          `json:"sensors,omitempty"`
	RAID                    []HostRAIDMeta           `json:"raid,omitempty"`
	Unraid                  *HostUnraidMeta          `json:"unraid,omitempty"`
	DiskIO                  []HostDiskIOMeta         `json:"diskIo,omitempty"`
	Ceph                    *HostCephMeta            `json:"ceph,omitempty"`
	StorageRisk             *StorageRisk             `json:"storageRisk,omitempty"`
	StorageRiskSummary      string                   `json:"storageRiskSummary,omitempty"`
	StoragePostureSummary   string                   `json:"storagePostureSummary,omitempty"`
	ProtectionReduced       bool                     `json:"protectionReduced,omitempty"`
	ProtectionSummary       string                   `json:"protectionSummary,omitempty"`
	RebuildInProgress       bool                     `json:"rebuildInProgress,omitempty"`
	RebuildSummary          string                   `json:"rebuildSummary,omitempty"`
	CommandsEnabled         bool                     `json:"commandsEnabled,omitempty"`
	OperationReceiptVersion int                      `json:"-"`
	PackageUpdates          *AgentPackageUpdateMeta  `json:"packageUpdates,omitempty"`
	StorageCleanup          *AgentStorageCleanupMeta `json:"storageCleanup,omitempty"`
	ReportIP                string                   `json:"reportIp,omitempty"`
	DiskExclude             []string                 `json:"diskExclude,omitempty"`
	IsLegacy                bool                     `json:"isLegacy,omitempty"`
	NetInRate               float64                  `json:"netInRate,omitempty"`
	NetOutRate              float64                  `json:"netOutRate,omitempty"`
	DiskReadRate            float64                  `json:"diskReadRate,omitempty"`
	DiskWriteRate           float64                  `json:"diskWriteRate,omitempty"`
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

// DockerNetworkSubnetMeta describes an IPAM subnet on a Docker network.
type DockerNetworkSubnetMeta struct {
	Subnet  string `json:"subnet,omitempty"`
	Gateway string `json:"gateway,omitempty"`
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

// DockerStorageUsageMeta captures one /system/df resource bucket.
type DockerStorageUsageMeta struct {
	TotalCount       int64 `json:"totalCount,omitempty"`
	ActiveCount      int64 `json:"activeCount,omitempty"`
	TotalSizeBytes   int64 `json:"totalSizeBytes,omitempty"`
	ReclaimableBytes int64 `json:"reclaimableBytes,omitempty"`
}

// DockerContainerBlockIOMeta captures cumulative container block IO totals.
type DockerContainerBlockIOMeta struct {
	ReadBytes  uint64 `json:"readBytes,omitempty"`
	WriteBytes uint64 `json:"writeBytes,omitempty"`
}

// DockerPodmanContainerMeta captures Podman-specific container metadata.
type DockerPodmanContainerMeta struct {
	PodName          string `json:"podName,omitempty"`
	PodID            string `json:"podId,omitempty"`
	Infra            bool   `json:"infra"`
	ComposeProject   string `json:"composeProject,omitempty"`
	ComposeService   string `json:"composeService,omitempty"`
	AutoUpdatePolicy string `json:"autoUpdatePolicy,omitempty"`
	UserNamespace    string `json:"userNamespace,omitempty"`
}

// DockerData contains Docker host- and container-specific data.
type DockerData struct {
	HostSourceID      string           `json:"hostSourceId,omitempty"` // raw model ID for the docker host
	AgentID           string           `json:"agentId,omitempty"`
	ContainerID       string           `json:"containerId,omitempty"`
	Hostname          string           `json:"hostname,omitempty"`
	DisplayName       string           `json:"displayName,omitempty"`
	CustomDisplayName string           `json:"customDisplayName,omitempty"`
	MachineID         string           `json:"machineId,omitempty"`
	Image             string           `json:"image,omitempty"`
	Temperature       *float64         `json:"temperature,omitempty"`
	Runtime           string           `json:"runtime,omitempty"`
	RuntimeVersion    string           `json:"runtimeVersion,omitempty"`
	DockerVersion     string           `json:"dockerVersion,omitempty"`
	OS                string           `json:"os,omitempty"`
	KernelVersion     string           `json:"kernelVersion,omitempty"`
	Architecture      string           `json:"architecture,omitempty"`
	AgentVersion      string           `json:"agentVersion,omitempty"`
	CPUs              int              `json:"cpus,omitempty"`
	TotalMemoryBytes  int64            `json:"totalMemoryBytes,omitempty"`
	Memory            *AgentMemoryMeta `json:"memory,omitempty"`
	UptimeSeconds     int64            `json:"uptimeSeconds,omitempty"`
	LoadAverage       []float64        `json:"loadAverage,omitempty"`
	IntervalSeconds   int              `json:"intervalSeconds,omitempty"`
	NetInRate         float64          `json:"netInRate,omitempty"`
	NetOutRate        float64          `json:"netOutRate,omitempty"`
	DiskReadRate      float64          `json:"diskReadRate,omitempty"`
	DiskWriteRate     float64          `json:"diskWriteRate,omitempty"`

	// Host-level summary fields (populated when Resource.Type == ResourceTypeAgent and Docker != nil)
	ContainerCount        int                                `json:"containerCount,omitempty"`
	ImageCount            int                                `json:"imageCount,omitempty"`
	VolumeCount           int                                `json:"volumeCount,omitempty"`
	NetworkCount          int                                `json:"networkCount,omitempty"`
	NodeCount             int                                `json:"nodeCount,omitempty"`
	SecretCount           int                                `json:"secretCount,omitempty"`
	ConfigCount           int                                `json:"configCount,omitempty"`
	UpdatesAvailableCount int                                `json:"updatesAvailableCount,omitempty"`
	UpdatesLastCheckedAt  *time.Time                         `json:"updatesLastCheckedAt,omitempty"`
	ImagesUsage           *DockerStorageUsageMeta            `json:"imagesUsage,omitempty"`
	ContainersUsage       *DockerStorageUsageMeta            `json:"containersUsage,omitempty"`
	VolumesUsage          *DockerStorageUsageMeta            `json:"volumesUsage,omitempty"`
	BuildCacheUsage       *DockerStorageUsageMeta            `json:"buildCacheUsage,omitempty"`
	TokenID               string                             `json:"tokenId,omitempty"`
	TokenName             string                             `json:"tokenName,omitempty"`
	TokenHint             string                             `json:"tokenHint,omitempty"`
	TokenLastUsedAt       *time.Time                         `json:"tokenLastUsedAt,omitempty"`
	Hidden                bool                               `json:"hidden,omitempty"`
	PendingUninstall      bool                               `json:"pendingUninstall,omitempty"`
	IsLegacy              bool                               `json:"isLegacy,omitempty"`
	Command               *models.DockerHostCommandStatus    `json:"command,omitempty"`
	Security              *models.DockerHostSecurity         `json:"security,omitempty"`
	IdentityConflict      *models.DockerHostIdentityConflict `json:"identityConflict,omitempty"`

	// Container-specific fields (populated when Resource.Type == ResourceTypeAppContainer)
	ContainerState     string                      `json:"containerState,omitempty"`
	Health             string                      `json:"health,omitempty"`
	RestartCount       int                         `json:"restartCount,omitempty"`
	ExitCode           int                         `json:"exitCode,omitempty"`
	OOMKilled          *bool                       `json:"oomKilled,omitempty"`
	CPURawPercent      float64                     `json:"cpuRawPercent,omitempty"`
	CPUCapacityPercent float64                     `json:"cpuCapacityPercent,omitempty"`
	CPUCapacityCores   int                         `json:"cpuCapacityCores,omitempty"`
	StartedAt          *time.Time                  `json:"startedAt,omitempty"`
	FinishedAt         *time.Time                  `json:"finishedAt,omitempty"`
	BlockIO            *DockerContainerBlockIOMeta `json:"blockIo,omitempty"`
	Podman             *DockerPodmanContainerMeta  `json:"podman,omitempty"`
	Ports              []DockerPortMeta            `json:"ports,omitempty"`
	Labels             map[string]string           `json:"labels,omitempty"`
	Networks           []DockerNetworkMeta         `json:"networks,omitempty"`
	Mounts             []DockerMountMeta           `json:"mounts,omitempty"`
	UpdateStatus       *DockerUpdateStatusMeta     `json:"updateStatus,omitempty"`

	// Service-specific fields (populated when Resource.Type == ResourceTypeDockerService)
	ServiceID      string                   `json:"serviceId,omitempty"`
	ServiceName    string                   `json:"serviceName,omitempty"`
	Stack          string                   `json:"stack,omitempty"`
	Mode           string                   `json:"mode,omitempty"`
	DesiredTasks   int                      `json:"desiredTasks,omitempty"`
	RunningTasks   int                      `json:"runningTasks,omitempty"`
	CompletedTasks int                      `json:"completedTasks,omitempty"`
	ServiceUpdate  *DockerServiceUpdateMeta `json:"serviceUpdate,omitempty"`
	EndpointPorts  []DockerServicePortMeta  `json:"endpointPorts,omitempty"`

	// Image-specific fields (populated when Resource.Type == ResourceTypeDockerImage)
	ImageID         string   `json:"imageId,omitempty"`
	RepoTags        []string `json:"repoTags,omitempty"`
	RepoDigests     []string `json:"repoDigests,omitempty"`
	SizeBytes       int64    `json:"sizeBytes,omitempty"`
	SharedSizeBytes int64    `json:"sharedSizeBytes,omitempty"`
	ImageContainers int64    `json:"imageContainers,omitempty"`

	// Volume-specific fields (populated when Resource.Type == ResourceTypeDockerVolume)
	VolumeName string            `json:"volumeName,omitempty"`
	Driver     string            `json:"driver,omitempty"`
	Mountpoint string            `json:"mountpoint,omitempty"`
	Scope      string            `json:"scope,omitempty"`
	CreatedAt  string            `json:"createdAt,omitempty"`
	RefCount   int64             `json:"refCount,omitempty"`
	Options    map[string]string `json:"options,omitempty"`

	// Network-specific fields (populated when Resource.Type == ResourceTypeDockerNetwork)
	NetworkID  string                    `json:"networkId,omitempty"`
	EnableIPv4 bool                      `json:"enableIpv4,omitempty"`
	EnableIPv6 bool                      `json:"enableIpv6,omitempty"`
	Internal   bool                      `json:"internal,omitempty"`
	Attachable bool                      `json:"attachable,omitempty"`
	Ingress    bool                      `json:"ingress,omitempty"`
	ConfigOnly bool                      `json:"configOnly,omitempty"`
	Subnets    []DockerNetworkSubnetMeta `json:"subnets,omitempty"`

	// Task-specific fields (populated when Resource.Type == ResourceTypeDockerTask)
	TaskID       string     `json:"taskId,omitempty"`
	NodeID       string     `json:"nodeId,omitempty"`
	NodeName     string     `json:"nodeName,omitempty"`
	Slot         int        `json:"slot,omitempty"`
	DesiredState string     `json:"desiredState,omitempty"`
	CurrentState string     `json:"currentState,omitempty"`
	Error        string     `json:"error,omitempty"`
	Message      string     `json:"message,omitempty"`
	CompletedAt  *time.Time `json:"completedAt,omitempty"`

	// Swarm-node-specific fields (populated when Resource.Type == ResourceTypeDockerSwarmNode)
	NodeRole            string            `json:"nodeRole,omitempty"`
	Availability        string            `json:"availability,omitempty"`
	Address             string            `json:"address,omitempty"`
	ManagerReachability string            `json:"managerReachability,omitempty"`
	ManagerAddress      string            `json:"managerAddress,omitempty"`
	Leader              bool              `json:"leader,omitempty"`
	EngineVersion       string            `json:"engineVersion,omitempty"`
	NanoCPUs            int64             `json:"nanoCpus,omitempty"`
	MemoryBytes         int64             `json:"memoryBytes,omitempty"`
	EngineLabels        map[string]string `json:"engineLabels,omitempty"`

	// Swarm secret/config fields (populated when Resource.Type == ResourceTypeDockerSecret or ResourceTypeDockerConfig)
	SecretID         string     `json:"secretId,omitempty"`
	SecretName       string     `json:"secretName,omitempty"`
	ConfigID         string     `json:"configId,omitempty"`
	ConfigName       string     `json:"configName,omitempty"`
	TemplatingDriver string     `json:"templatingDriver,omitempty"`
	ObjectCreatedAt  *time.Time `json:"objectCreatedAt,omitempty"`
	ObjectUpdatedAt  *time.Time `json:"objectUpdatedAt,omitempty"`

	Swarm             *DockerSwarmInfo   `json:"swarm,omitempty"`
	NetworkInterfaces []NetworkInterface `json:"networkInterfaces,omitempty"`
	Disks             []DiskInfo         `json:"disks,omitempty"`

	// These hold raw data for tools access
	Containers  []models.DockerContainer `json:"-"`
	Images      []models.DockerImage     `json:"-"`
	Volumes     []models.DockerVolume    `json:"-"`
	NetworksRaw []models.DockerNetwork   `json:"-"`
	Services    []models.DockerService   `json:"-"`
	Tasks       []models.DockerTask      `json:"-"`
	Nodes       []models.DockerNode      `json:"-"`
	Secrets     []models.DockerSecret    `json:"-"`
	Configs     []models.DockerConfig    `json:"-"`
}

// PBSData contains Proxmox Backup Server data.
//
// NOTE: Some tools need per-datastore details; those are exposed via Datastores.
type PBSData struct {
	InstanceID               string                        `json:"instanceId,omitempty"`
	Hostname                 string                        `json:"hostname,omitempty"`
	HostURL                  string                        `json:"hostUrl,omitempty"`
	GuestURL                 string                        `json:"guestUrl,omitempty"`
	Version                  string                        `json:"version,omitempty"`
	UptimeSeconds            int64                         `json:"uptimeSeconds,omitempty"`
	DatastoreCount           int                           `json:"datastoreCount,omitempty"`
	Datastores               []PBSDatastoreMeta            `json:"datastores,omitempty"`
	DatastoreDetails         []models.PBSDatastore         `json:"datastoreDetails,omitempty"`
	StorageRisk              *StorageRisk                  `json:"storageRisk,omitempty"`
	AffectedDatastoreCount   int                           `json:"affectedDatastoreCount,omitempty"`
	AffectedDatastores       []string                      `json:"affectedDatastores,omitempty"`
	AffectedDatastoreSummary string                        `json:"affectedDatastoreSummary,omitempty"`
	ProtectedWorkloadCount   int                           `json:"protectedWorkloadCount,omitempty"`
	ProtectedWorkloadTypes   []string                      `json:"protectedWorkloadTypes,omitempty"`
	ProtectedWorkloadNames   []string                      `json:"protectedWorkloadNames,omitempty"`
	ProtectedWorkloadSummary string                        `json:"protectedWorkloadSummary,omitempty"`
	PostureSummary           string                        `json:"postureSummary,omitempty"`
	BackupJobCount           int                           `json:"backupJobCount,omitempty"`
	BackupJobs               []models.PBSBackupJob         `json:"backupJobs,omitempty"`
	SyncJobCount             int                           `json:"syncJobCount,omitempty"`
	SyncJobs                 []models.PBSSyncJob           `json:"syncJobs,omitempty"`
	VerifyJobCount           int                           `json:"verifyJobCount,omitempty"`
	VerifyJobs               []models.PBSVerifyJob         `json:"verifyJobs,omitempty"`
	PruneJobCount            int                           `json:"pruneJobCount,omitempty"`
	PruneJobs                []models.PBSPruneJob          `json:"pruneJobs,omitempty"`
	GarbageJobCount          int                           `json:"garbageJobCount,omitempty"`
	GarbageJobs              []models.PBSGarbageJob        `json:"garbageJobs,omitempty"`
	JobHealthEvidenceCount   int                           `json:"jobHealthEvidenceCount,omitempty"`
	JobHealthEvidence        []models.PBSJobHealthEvidence `json:"jobHealthEvidence,omitempty"`
	ConnectionHealth         string                        `json:"connectionHealth,omitempty"`
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
	Active    int       `json:"active"`
	Deferred  int       `json:"deferred"`
	Hold      int       `json:"hold"`
	Incoming  int       `json:"incoming"`
	Total     int       `json:"total"`
	OldestAge int64     `json:"oldestAge,omitempty"`
	UpdatedAt time.Time `json:"updatedAt,omitempty"`
}

// PMGMailStatsMeta describes PMG mail statistics.
type PMGMailStatsMeta struct {
	Timeframe            string    `json:"timeframe"`
	CountTotal           float64   `json:"countTotal"`
	CountIn              float64   `json:"countIn"`
	CountOut             float64   `json:"countOut"`
	SpamIn               float64   `json:"spamIn"`
	SpamOut              float64   `json:"spamOut"`
	VirusIn              float64   `json:"virusIn"`
	VirusOut             float64   `json:"virusOut"`
	BouncesIn            float64   `json:"bouncesIn"`
	BouncesOut           float64   `json:"bouncesOut"`
	BytesIn              float64   `json:"bytesIn"`
	BytesOut             float64   `json:"bytesOut"`
	GreylistCount        float64   `json:"greylistCount"`
	JunkIn               float64   `json:"junkIn"`
	RBLRejects           float64   `json:"rblRejects"`
	AverageProcessTimeMs float64   `json:"averageProcessTimeMs"`
	PregreetRejects      float64   `json:"pregreetRejects"`
	UpdatedAt            time.Time `json:"updatedAt,omitempty"`
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
	HostURL          string    `json:"hostUrl,omitempty"`
	GuestURL         string    `json:"guestUrl,omitempty"`
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

	Nodes            []PMGNodeMeta              `json:"nodes,omitempty"`
	MailStats        *PMGMailStatsMeta          `json:"mailStats,omitempty"`
	MailCount        []models.PMGMailCountPoint `json:"mailCount,omitempty"`
	Quarantine       *PMGQuarantineMeta         `json:"quarantine,omitempty"`
	SpamDistribution []PMGSpamBucketMeta        `json:"spamDistribution,omitempty"`
	RelayDomains     []PMGRelayDomainMeta       `json:"relayDomains,omitempty"`
	DomainStats      []PMGDomainStatMeta        `json:"domainStats,omitempty"`
	DomainStatsAsOf  time.Time                  `json:"domainStatsAsOf,omitempty"`
}

// VMwareData contains VMware vSphere metadata for canonical agent, vm,
// storage, and network resources projected from one vCenter connection.
type VMwareData struct {
	ConnectionID        string                     `json:"connectionId,omitempty"`
	ConnectionName      string                     `json:"connectionName,omitempty"`
	VCenterHost         string                     `json:"vcenterHost,omitempty"`
	ManagedObjectID     string                     `json:"managedObjectId,omitempty"`
	EntityType          string                     `json:"entityType,omitempty"`
	HostUUID            string                     `json:"hostUuid,omitempty"`
	DatacenterID        string                     `json:"datacenterId,omitempty"`
	DatacenterName      string                     `json:"datacenterName,omitempty"`
	ComputeResourceID   string                     `json:"computeResourceId,omitempty"`
	ComputeResourceName string                     `json:"computeResourceName,omitempty"`
	ClusterID           string                     `json:"clusterId,omitempty"`
	ClusterName         string                     `json:"clusterName,omitempty"`
	ClusterHAEnabled    *bool                      `json:"clusterHaEnabled,omitempty"`
	ClusterDRSEnabled   *bool                      `json:"clusterDrsEnabled,omitempty"`
	FolderID            string                     `json:"folderId,omitempty"`
	FolderName          string                     `json:"folderName,omitempty"`
	ResourcePoolID      string                     `json:"resourcePoolId,omitempty"`
	ResourcePoolName    string                     `json:"resourcePoolName,omitempty"`
	RuntimeHostID       string                     `json:"runtimeHostId,omitempty"`
	RuntimeHostName     string                     `json:"runtimeHostName,omitempty"`
	ConnectionState     string                     `json:"connectionState,omitempty"`
	PowerState          string                     `json:"powerState,omitempty"`
	OverallStatus       string                     `json:"overallStatus,omitempty"`
	CPUCount            int                        `json:"cpuCount,omitempty"`
	MemorySizeMiB       int64                      `json:"memorySizeMib,omitempty"`
	DatastoreType       string                     `json:"datastoreType,omitempty"`
	DatastoreIDs        []string                   `json:"datastoreIds,omitempty"`
	DatastoreNames      []string                   `json:"datastoreNames,omitempty"`
	DatastoreURL        string                     `json:"datastoreUrl,omitempty"`
	DatastoreAccessible *bool                      `json:"datastoreAccessible,omitempty"`
	MultipleHostAccess  *bool                      `json:"multipleHostAccess,omitempty"`
	MaintenanceMode     string                     `json:"maintenanceMode,omitempty"`
	NetworkType         string                     `json:"networkType,omitempty"`
	NetworkHostIDs      []string                   `json:"networkHostIds,omitempty"`
	NetworkHostNames    []string                   `json:"networkHostNames,omitempty"`
	NetworkVMIDs        []string                   `json:"networkVmIds,omitempty"`
	NetworkVMNames      []string                   `json:"networkVmNames,omitempty"`
	InstanceUUID        string                     `json:"instanceUuid,omitempty"`
	BIOSUUID            string                     `json:"biosUuid,omitempty"`
	GuestOSFamily       string                     `json:"guestOsFamily,omitempty"`
	GuestHostname       string                     `json:"guestHostname,omitempty"`
	GuestIPAddresses    []string                   `json:"guestIpAddresses,omitempty"`
	ActiveAlarmCount    int                        `json:"activeAlarmCount,omitempty"`
	ActiveAlarmSummary  string                     `json:"activeAlarmSummary,omitempty"`
	RecentTaskCount     int                        `json:"recentTaskCount,omitempty"`
	RecentTaskSummary   string                     `json:"recentTaskSummary,omitempty"`
	SnapshotCount       int                        `json:"snapshotCount,omitempty"`
	CurrentSnapshotID   string                     `json:"currentSnapshotId,omitempty"`
	SnapshotTree        []VMwareSnapshotData       `json:"snapshotTree,omitempty"`
	NetworkAdapters     []VMwareNetworkAdapterData `json:"networkAdapters,omitempty"`
	VirtualDisks        []VMwareVirtualDiskData    `json:"virtualDisks,omitempty"`
	Tools               *VMwareToolsData           `json:"tools,omitempty"`
	Hardware            *VMwareVMHardwareData      `json:"hardware,omitempty"`
}

// VMwareSnapshotData contains one node in the vSphere VM snapshot tree. It is
// read-side workload context only, not a Pulse recovery artifact.
type VMwareSnapshotData struct {
	Snapshot        string               `json:"snapshot,omitempty"`
	Name            string               `json:"name,omitempty"`
	Description     string               `json:"description,omitempty"`
	ID              int                  `json:"id,omitempty"`
	CreatedAt       *time.Time           `json:"createdAt,omitempty"`
	State           string               `json:"state,omitempty"`
	Quiesced        bool                 `json:"quiesced"`
	ReplaySupported bool                 `json:"replaySupported,omitempty"`
	Current         bool                 `json:"current,omitempty"`
	Children        []VMwareSnapshotData `json:"children,omitempty"`
}

// VMwareNetworkAdapterData contains vCenter VM hardware Ethernet adapter facts
// as read-only workload context, not a Pulse network-control model.
type VMwareNetworkAdapterData struct {
	NIC                   string `json:"nic,omitempty"`
	Label                 string `json:"label,omitempty"`
	Type                  string `json:"type,omitempty"`
	MACType               string `json:"macType,omitempty"`
	MACAddress            string `json:"macAddress,omitempty"`
	PCISlotNumber         *int64 `json:"pciSlotNumber,omitempty"`
	BackingType           string `json:"backingType,omitempty"`
	NetworkID             string `json:"networkId,omitempty"`
	NetworkName           string `json:"networkName,omitempty"`
	DistributedSwitchUUID string `json:"distributedSwitchUuid,omitempty"`
	DistributedPort       string `json:"distributedPort,omitempty"`
	OpaqueNetworkType     string `json:"opaqueNetworkType,omitempty"`
	OpaqueNetworkID       string `json:"opaqueNetworkId,omitempty"`
	HostDevice            string `json:"hostDevice,omitempty"`
	State                 string `json:"state,omitempty"`
	StartConnected        bool   `json:"startConnected"`
	AllowGuestControl     bool   `json:"allowGuestControl"`
	WakeOnLANEnabled      bool   `json:"wakeOnLanEnabled"`
	UPTCompatibility      bool   `json:"uptCompatibilityEnabled,omitempty"`
	UPTV2Compatibility    bool   `json:"uptV2CompatibilityEnabled,omitempty"`
}

// VMwareVirtualDiskData contains vCenter VM hardware disk facts as read-only
// workload context, not a Pulse storage/recovery model.
type VMwareVirtualDiskData struct {
	Disk          string `json:"disk,omitempty"`
	Label         string `json:"label,omitempty"`
	Type          string `json:"type,omitempty"`
	IDEPrimary    *bool  `json:"idePrimary,omitempty"`
	IDEMaster     *bool  `json:"ideMaster,omitempty"`
	SCSIBus       *int64 `json:"scsiBus,omitempty"`
	SCSIUnit      *int64 `json:"scsiUnit,omitempty"`
	SATABus       *int64 `json:"sataBus,omitempty"`
	SATAUnit      *int64 `json:"sataUnit,omitempty"`
	NVMEBus       *int64 `json:"nvmeBus,omitempty"`
	NVMEUnit      *int64 `json:"nvmeUnit,omitempty"`
	BackingType   string `json:"backingType,omitempty"`
	VMDKFile      string `json:"vmdkFile,omitempty"`
	DatastoreName string `json:"datastoreName,omitempty"`
	CapacityBytes *int64 `json:"capacityBytes,omitempty"`
}

// VMwareToolsData contains VMware Tools runtime facts as read-only VM context.
type VMwareToolsData struct {
	AutoUpdateSupported    *bool    `json:"autoUpdateSupported,omitempty"`
	InstallAttemptCount    *int64   `json:"installAttemptCount,omitempty"`
	ErrorMessage           string   `json:"errorMessage,omitempty"`
	VersionNumber          *int64   `json:"versionNumber,omitempty"`
	Version                string   `json:"version,omitempty"`
	UpgradePolicy          string   `json:"upgradePolicy,omitempty"`
	VersionStatus          string   `json:"versionStatus,omitempty"`
	InstallType            string   `json:"installType,omitempty"`
	RunState               string   `json:"runState,omitempty"`
	GuestRebootRequested   *bool    `json:"guestRebootRequested,omitempty"`
	GuestRebootComponents  []string `json:"guestRebootComponents,omitempty"`
	GuestRebootRequestTime string   `json:"guestRebootRequestTime,omitempty"`
}

// VMwareBootDeviceData contains one vCenter VM boot-device entry as read-only
// virtual hardware context.
type VMwareBootDeviceData struct {
	Type  string   `json:"type,omitempty"`
	NIC   string   `json:"nic,omitempty"`
	Disks []string `json:"disks,omitempty"`
}

// VMwareVMHardwareData contains vCenter VM virtual hardware, CPU, memory, and
// boot configuration facts as read-only VM context.
type VMwareVMHardwareData struct {
	GuestOS                    string                 `json:"guestOs,omitempty"`
	InstantCloneFrozen         *bool                  `json:"instantCloneFrozen,omitempty"`
	Version                    string                 `json:"version,omitempty"`
	UpgradePolicy              string                 `json:"upgradePolicy,omitempty"`
	UpgradeVersion             string                 `json:"upgradeVersion,omitempty"`
	UpgradeStatus              string                 `json:"upgradeStatus,omitempty"`
	UpgradeErrorMessage        string                 `json:"upgradeErrorMessage,omitempty"`
	BootType                   string                 `json:"bootType,omitempty"`
	EFILegacyBoot              *bool                  `json:"efiLegacyBoot,omitempty"`
	BootNetworkProtocol        string                 `json:"bootNetworkProtocol,omitempty"`
	BootDelayMilliseconds      *int64                 `json:"bootDelayMilliseconds,omitempty"`
	BootRetry                  *bool                  `json:"bootRetry,omitempty"`
	BootRetryDelayMilliseconds *int64                 `json:"bootRetryDelayMilliseconds,omitempty"`
	EnterSetupMode             *bool                  `json:"enterSetupMode,omitempty"`
	BootDevices                []VMwareBootDeviceData `json:"bootDevices,omitempty"`
	CPUCoresPerSocket          *int64                 `json:"cpuCoresPerSocket,omitempty"`
	CPUHotAddEnabled           *bool                  `json:"cpuHotAddEnabled,omitempty"`
	CPUHotRemoveEnabled        *bool                  `json:"cpuHotRemoveEnabled,omitempty"`
	MemoryHotAddEnabled        *bool                  `json:"memoryHotAddEnabled,omitempty"`
	MemoryHotAddIncrementMiB   *int64                 `json:"memoryHotAddIncrementMib,omitempty"`
	MemoryHotAddLimitMiB       *int64                 `json:"memoryHotAddLimitMib,omitempty"`
}

// TrueNASData contains TrueNAS-specific metadata for TrueNAS resources.
type TrueNASData struct {
	Hostname              string           `json:"hostname,omitempty"`
	Version               string           `json:"version,omitempty"`
	UptimeSeconds         int64            `json:"uptimeSeconds,omitempty"`
	StorageRisk           *StorageRisk     `json:"storageRisk,omitempty"`
	StorageRiskSummary    string           `json:"storageRiskSummary,omitempty"`
	StoragePostureSummary string           `json:"storagePostureSummary,omitempty"`
	ProtectionReduced     bool             `json:"protectionReduced,omitempty"`
	ProtectionSummary     string           `json:"protectionSummary,omitempty"`
	RebuildInProgress     bool             `json:"rebuildInProgress,omitempty"`
	RebuildSummary        string           `json:"rebuildSummary,omitempty"`
	App                   *TrueNASApp      `json:"app,omitempty"`
	VM                    *TrueNASVM       `json:"vm,omitempty"`
	Share                 *TrueNASShare    `json:"share,omitempty"`
	Services              []TrueNASService `json:"services,omitempty"`
}

// TrueNASService contains service.query data for one native TrueNAS system
// service.
type TrueNASService struct {
	ID      string `json:"id,omitempty"`
	Service string `json:"service,omitempty"`
	Enabled bool   `json:"enabled"`
	State   string `json:"state,omitempty"`
	PIDs    []int  `json:"pids,omitempty"`
}

// TrueNASApp contains app.query / active_workloads data for one TrueNAS app.
type TrueNASApp struct {
	ID                    string                `json:"id,omitempty"`
	Name                  string                `json:"name,omitempty"`
	State                 string                `json:"state,omitempty"`
	Version               string                `json:"version,omitempty"`
	HumanVersion          string                `json:"humanVersion,omitempty"`
	CustomApp             bool                  `json:"customApp,omitempty"`
	UpgradeAvailable      bool                  `json:"upgradeAvailable,omitempty"`
	ImageUpdatesAvailable bool                  `json:"imageUpdatesAvailable,omitempty"`
	Notes                 string                `json:"notes,omitempty"`
	ContainerCount        int                   `json:"containerCount,omitempty"`
	UsedHostIPs           []string              `json:"usedHostIps,omitempty"`
	UsedPorts             []TrueNASAppPort      `json:"usedPorts,omitempty"`
	Containers            []TrueNASAppContainer `json:"containers,omitempty"`
	Volumes               []TrueNASAppVolume    `json:"volumes,omitempty"`
	Images                []string              `json:"images,omitempty"`
	Networks              []TrueNASAppNetwork   `json:"networks,omitempty"`
	Stats                 *TrueNASAppStats      `json:"stats,omitempty"`
}

// TrueNASAppStats contains app.stats telemetry metadata that is not already
// normalized into top-level Resource metrics.
type TrueNASAppStats struct {
	IntervalSeconds int       `json:"intervalSeconds,omitempty"`
	CollectedAt     time.Time `json:"collectedAt,omitempty"`
}

// TrueNASAppPort describes a TrueNAS app published port entry.
type TrueNASAppPort struct {
	ContainerPort int                  `json:"containerPort,omitempty"`
	Protocol      string               `json:"protocol,omitempty"`
	HostPorts     []TrueNASAppHostPort `json:"hostPorts,omitempty"`
}

// TrueNASAppHostPort describes a host binding for a TrueNAS app port.
type TrueNASAppHostPort struct {
	HostPort int    `json:"hostPort,omitempty"`
	HostIP   string `json:"hostIp,omitempty"`
}

// TrueNASAppContainer describes a runtime container inside a TrueNAS app.
type TrueNASAppContainer struct {
	ID           string             `json:"id,omitempty"`
	ServiceName  string             `json:"serviceName,omitempty"`
	Image        string             `json:"image,omitempty"`
	State        string             `json:"state,omitempty"`
	PortConfig   []TrueNASAppPort   `json:"portConfig,omitempty"`
	VolumeMounts []TrueNASAppVolume `json:"volumeMounts,omitempty"`
}

// TrueNASAppVolume describes a TrueNAS app volume or bind mount.
type TrueNASAppVolume struct {
	Source      string `json:"source,omitempty"`
	Destination string `json:"destination,omitempty"`
	Mode        string `json:"mode,omitempty"`
	Type        string `json:"type,omitempty"`
}

// TrueNASAppNetwork describes a network attached to a TrueNAS app workload.
type TrueNASAppNetwork struct {
	ID     string            `json:"id,omitempty"`
	Name   string            `json:"name,omitempty"`
	Labels map[string]string `json:"labels,omitempty"`
}

// TrueNASVM contains vm.query data for one TrueNAS virtual machine.
type TrueNASVM struct {
	ID                    string `json:"id,omitempty"`
	Name                  string `json:"name,omitempty"`
	Description           string `json:"description,omitempty"`
	State                 string `json:"state,omitempty"`
	DomainState           string `json:"domainState,omitempty"`
	PID                   int    `json:"pid,omitempty"`
	VCPUs                 int    `json:"vcpus,omitempty"`
	Cores                 int    `json:"cores,omitempty"`
	Threads               int    `json:"threads,omitempty"`
	MemoryBytes           int64  `json:"memoryBytes,omitempty"`
	MinMemoryBytes        int64  `json:"minMemoryBytes,omitempty"`
	CPUMode               string `json:"cpuMode,omitempty"`
	CPUModel              string `json:"cpuModel,omitempty"`
	Bootloader            string `json:"bootloader,omitempty"`
	Autostart             bool   `json:"autostart,omitempty"`
	SuspendOnSnapshot     bool   `json:"suspendOnSnapshot,omitempty"`
	TrustedPlatformModule bool   `json:"trustedPlatformModule,omitempty"`
	SecureBoot            bool   `json:"secureBoot,omitempty"`
	Time                  string `json:"time,omitempty"`
	ArchType              string `json:"archType,omitempty"`
	MachineType           string `json:"machineType,omitempty"`
	UUID                  string `json:"uuid,omitempty"`
	DisplayAvailable      bool   `json:"displayAvailable,omitempty"`
	DeviceCount           int    `json:"deviceCount,omitempty"`
	DiskCount             int    `json:"diskCount,omitempty"`
	NICCount              int    `json:"nicCount,omitempty"`
	DisplayCount          int    `json:"displayCount,omitempty"`
	CDROMCount            int    `json:"cdromCount,omitempty"`
	USBCount              int    `json:"usbCount,omitempty"`
	PCICount              int    `json:"pciCount,omitempty"`
}

// TrueNASShare contains sharing.smb.query / sharing.nfs.query data for one
// native TrueNAS network share.
type TrueNASShare struct {
	ID                     string   `json:"id,omitempty"`
	Name                   string   `json:"name,omitempty"`
	Protocol               string   `json:"protocol,omitempty"`
	Path                   string   `json:"path,omitempty"`
	Dataset                string   `json:"dataset,omitempty"`
	RelativePath           string   `json:"relativePath,omitempty"`
	Comment                string   `json:"comment,omitempty"`
	Enabled                bool     `json:"enabled"`
	ReadOnly               bool     `json:"readOnly"`
	Browsable              bool     `json:"browsable,omitempty"`
	Locked                 bool     `json:"locked,omitempty"`
	AccessBasedEnumeration bool     `json:"accessBasedEnumeration,omitempty"`
	AuditEnabled           bool     `json:"auditEnabled,omitempty"`
	ExposeSnapshots        bool     `json:"exposeSnapshots,omitempty"`
	Aliases                []string `json:"aliases,omitempty"`
	Hosts                  []string `json:"hosts,omitempty"`
	Networks               []string `json:"networks,omitempty"`
	Security               []string `json:"security,omitempty"`
	MapRootUser            string   `json:"mapRootUser,omitempty"`
	MapRootGroup           string   `json:"mapRootGroup,omitempty"`
	MapAllUser             string   `json:"mapAllUser,omitempty"`
	MapAllGroup            string   `json:"mapAllGroup,omitempty"`
}

// AvailabilityCorrelationState records whether an agentless check has a
// trustworthy canonical owner. It is deliberately separate from probe health:
// a reachable endpoint can still have ambiguous identity.
type AvailabilityCorrelationState string

const (
	AvailabilityCorrelationAttached   AvailabilityCorrelationState = "attached"
	AvailabilityCorrelationStandalone AvailabilityCorrelationState = "standalone"
	AvailabilityCorrelationAmbiguous  AvailabilityCorrelationState = "ambiguous"
	AvailabilityCorrelationUnresolved AvailabilityCorrelationState = "unresolved"
)

// AvailabilityData contains agentless endpoint probe metadata for a resource.
type AvailabilityData struct {
	TargetID              string                             `json:"targetId,omitempty"`
	LinkedResourceID      string                             `json:"linkedResourceId,omitempty"`
	Name                  string                             `json:"name,omitempty"`
	TargetKind            string                             `json:"targetKind,omitempty"`
	Address               string                             `json:"address,omitempty"`
	Protocol              string                             `json:"protocol,omitempty"`
	ProbeOutcome          string                             `json:"probeOutcome,omitempty"`
	UDPMode               string                             `json:"udpMode,omitempty"`
	Port                  int                                `json:"port,omitempty"`
	Path                  string                             `json:"path,omitempty"`
	Enabled               bool                               `json:"enabled"`
	Available             bool                               `json:"available"`
	LastChecked           *time.Time                         `json:"lastChecked,omitempty"`
	LastSuccess           *time.Time                         `json:"lastSuccess,omitempty"`
	LatencyMillis         int64                              `json:"latencyMillis,omitempty"`
	ConsecutiveFailures   int                                `json:"consecutiveFailures,omitempty"`
	LastError             string                             `json:"lastError,omitempty"`
	FailureThreshold      int                                `json:"failureThreshold,omitempty"`
	PollIntervalSeconds   int                                `json:"pollIntervalSeconds,omitempty"`
	TimeoutMillis         int                                `json:"timeoutMillis,omitempty"`
	CorrelationState      AvailabilityCorrelationState       `json:"correlationState,omitempty"`
	CorrelationRule       string                             `json:"correlationRule,omitempty"`
	CorrelationReason     string                             `json:"correlationReason,omitempty"`
	CorrelationCandidates int                                `json:"correlationCandidates,omitempty"`
	Evidence              *operationaltrust.EvidenceEnvelope `json:"evidence,omitempty"`
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
	ClusterID                    string                 `json:"clusterId,omitempty"`
	ClusterName                  string                 `json:"clusterName,omitempty"`
	ResourceUID                  string                 `json:"resourceUid,omitempty"`
	ResourceKind                 string                 `json:"resourceKind,omitempty"`
	SourceName                   string                 `json:"sourceName,omitempty"`   // raw model Name before display-name resolution
	SourceStatus                 string                 `json:"sourceStatus,omitempty"` // raw model Status before normalization
	AgentID                      string                 `json:"agentId,omitempty"`
	Context                      string                 `json:"context,omitempty"`
	Server                       string                 `json:"server,omitempty"`
	Version                      string                 `json:"version,omitempty"`
	PendingUninstall             bool                   `json:"pendingUninstall,omitempty"`
	AgentVersion                 string                 `json:"agentVersion,omitempty"`    // cluster: k8s agent version
	IntervalSeconds              int                    `json:"intervalSeconds,omitempty"` // cluster: telemetry interval
	NodeUID                      string                 `json:"nodeUid,omitempty"`
	NodeName                     string                 `json:"nodeName,omitempty"`
	Ready                        bool                   `json:"ready,omitempty"`
	Unschedulable                bool                   `json:"unschedulable,omitempty"`
	Roles                        []string               `json:"roles,omitempty"`
	KubeletVersion               string                 `json:"kubeletVersion,omitempty"`
	ContainerRuntimeVersion      string                 `json:"containerRuntimeVersion,omitempty"`
	OSImage                      string                 `json:"osImage,omitempty"`
	KernelVersion                string                 `json:"kernelVersion,omitempty"`
	Architecture                 string                 `json:"architecture,omitempty"`
	CapacityCPU                  int64                  `json:"capacityCpuCores,omitempty"`
	CapacityMemoryBytes          int64                  `json:"capacityMemoryBytes,omitempty"`
	CapacityPods                 int64                  `json:"capacityPods,omitempty"`
	AllocCPU                     int64                  `json:"allocatableCpuCores,omitempty"`
	AllocMemoryBytes             int64                  `json:"allocatableMemoryBytes,omitempty"`
	AllocPods                    int64                  `json:"allocatablePods,omitempty"`
	Namespace                    string                 `json:"namespace,omitempty"`
	PodUID                       string                 `json:"podUid,omitempty"`
	PodPhase                     string                 `json:"podPhase,omitempty"`
	PodReason                    string                 `json:"podReason,omitempty"`     // pod: status reason
	PodMessage                   string                 `json:"podMessage,omitempty"`    // pod: status message
	PodContainers                []K8sPodContainer      `json:"podContainers,omitempty"` // pod: sub-containers
	QoSClass                     string                 `json:"qosClass,omitempty"`      // pod: Guaranteed / Burstable / BestEffort
	UptimeSeconds                int64                  `json:"uptimeSeconds,omitempty"`
	Temperature                  *float64               `json:"temperature,omitempty"`
	Restarts                     int                    `json:"restarts,omitempty"`
	OwnerKind                    string                 `json:"ownerKind,omitempty"`
	OwnerName                    string                 `json:"ownerName,omitempty"`
	Image                        string                 `json:"image,omitempty"`
	Labels                       map[string]string      `json:"labels,omitempty"`
	DeploymentUID                string                 `json:"deploymentUid,omitempty"`
	ReplicaSetUID                string                 `json:"replicaSetUid,omitempty"`
	StatefulSetUID               string                 `json:"statefulSetUid,omitempty"`
	DaemonSetUID                 string                 `json:"daemonSetUid,omitempty"`
	ServiceUID                   string                 `json:"serviceUid,omitempty"`
	JobUID                       string                 `json:"jobUid,omitempty"`
	CronJobUID                   string                 `json:"cronJobUid,omitempty"`
	IngressUID                   string                 `json:"ingressUid,omitempty"`
	EndpointSliceUID             string                 `json:"endpointSliceUid,omitempty"`
	NetworkPolicyUID             string                 `json:"networkPolicyUid,omitempty"`
	PersistentVolumeUID          string                 `json:"persistentVolumeUid,omitempty"`
	PersistentVolumeClaimUID     string                 `json:"persistentVolumeClaimUid,omitempty"`
	StorageClassUID              string                 `json:"storageClassUid,omitempty"`
	ConfigMapUID                 string                 `json:"configMapUid,omitempty"`
	SecretUID                    string                 `json:"secretUid,omitempty"`
	ServiceAccountUID            string                 `json:"serviceAccountUid,omitempty"`
	RoleUID                      string                 `json:"roleUid,omitempty"`
	ClusterRoleUID               string                 `json:"clusterRoleUid,omitempty"`
	RoleBindingUID               string                 `json:"roleBindingUid,omitempty"`
	ClusterRoleBindingUID        string                 `json:"clusterRoleBindingUid,omitempty"`
	RoleKind                     string                 `json:"roleKind,omitempty"`
	RoleName                     string                 `json:"roleName,omitempty"`
	RuleCount                    int                    `json:"ruleCount,omitempty"`
	SubjectCount                 int                    `json:"subjectCount,omitempty"`
	SubjectKinds                 []string               `json:"subjectKinds,omitempty"`
	AggregationLabels            map[string]string      `json:"aggregationLabels,omitempty"`
	ResourceQuotaUID             string                 `json:"resourceQuotaUid,omitempty"`
	LimitRangeUID                string                 `json:"limitRangeUid,omitempty"`
	PodDisruptionBudgetUID       string                 `json:"podDisruptionBudgetUid,omitempty"`
	HorizontalPodAutoscalerUID   string                 `json:"horizontalPodAutoscalerUid,omitempty"`
	EventUID                     string                 `json:"eventUid,omitempty"`
	NamespaceUID                 string                 `json:"namespaceUid,omitempty"`
	DesiredReplicas              int32                  `json:"desiredReplicas,omitempty"`
	UpdatedReplicas              int32                  `json:"updatedReplicas,omitempty"`
	ReadyReplicas                int32                  `json:"readyReplicas,omitempty"`
	AvailableReplicas            int32                  `json:"availableReplicas,omitempty"`
	CurrentReplicas              int32                  `json:"currentReplicas,omitempty"`
	FullyLabeledReplicas         int32                  `json:"fullyLabeledReplicas,omitempty"`
	ObservedGeneration           int64                  `json:"observedGeneration,omitempty"`
	DesiredNumberScheduled       int32                  `json:"desiredNumberScheduled,omitempty"`
	CurrentNumberScheduled       int32                  `json:"currentNumberScheduled,omitempty"`
	NumberReady                  int32                  `json:"numberReady,omitempty"`
	NumberAvailable              int32                  `json:"numberAvailable,omitempty"`
	NumberUnavailable            int32                  `json:"numberUnavailable,omitempty"`
	NumberMisscheduled           int32                  `json:"numberMisscheduled,omitempty"`
	ServiceName                  string                 `json:"serviceName,omitempty"`
	ServiceType                  string                 `json:"serviceType,omitempty"`
	ClusterIP                    string                 `json:"clusterIp,omitempty"`
	ExternalIPs                  []string               `json:"externalIps,omitempty"`
	ServicePorts                 []K8sServicePort       `json:"servicePorts,omitempty"`
	Selector                     map[string]string      `json:"selector,omitempty"`
	Succeeded                    int32                  `json:"succeeded,omitempty"`
	Failed                       int32                  `json:"failed,omitempty"`
	Active                       int32                  `json:"active,omitempty"`
	Schedule                     string                 `json:"schedule,omitempty"`
	Suspend                      bool                   `json:"suspend,omitempty"`
	LastScheduleTime             *time.Time             `json:"lastScheduleTime,omitempty"`
	LastSuccessfulTime           *time.Time             `json:"lastSuccessfulTime,omitempty"`
	StartTime                    *time.Time             `json:"startTime,omitempty"`
	CompletionTime               *time.Time             `json:"completionTime,omitempty"`
	ClassName                    string                 `json:"className,omitempty"`
	Hosts                        []string               `json:"hosts,omitempty"`
	Addresses                    []string               `json:"addresses,omitempty"`
	AddressType                  string                 `json:"addressType,omitempty"`
	EndpointCount                int                    `json:"endpointCount,omitempty"`
	ReadyEndpointCount           int                    `json:"readyEndpointCount,omitempty"`
	EndpointPorts                []K8sEndpointPort      `json:"endpointPorts,omitempty"`
	PolicyTypes                  []string               `json:"policyTypes,omitempty"`
	IngressRuleCount             int                    `json:"ingressRuleCount,omitempty"`
	EgressRuleCount              int                    `json:"egressRuleCount,omitempty"`
	Phase                        string                 `json:"phase,omitempty"`
	StorageClass                 string                 `json:"storageClass,omitempty"`
	Provisioner                  string                 `json:"provisioner,omitempty"`
	VolumeBindingMode            string                 `json:"volumeBindingMode,omitempty"`
	AllowVolumeExpansion         *bool                  `json:"allowVolumeExpansion,omitempty"`
	ParameterKeys                []string               `json:"parameterKeys,omitempty"`
	DataKeys                     []string               `json:"dataKeys,omitempty"`
	BinaryDataKeys               []string               `json:"binaryDataKeys,omitempty"`
	Immutable                    bool                   `json:"immutable,omitempty"`
	MetadataOnly                 bool                   `json:"metadataOnly,omitempty"`
	SecretType                   string                 `json:"secretType,omitempty"`
	AutomountServiceAccountToken *bool                  `json:"automountServiceAccountToken,omitempty"`
	SecretCount                  int                    `json:"secretCount,omitempty"`
	ImagePullSecrets             []string               `json:"imagePullSecrets,omitempty"`
	Hard                         map[string]string      `json:"hard,omitempty"`
	Used                         map[string]string      `json:"used,omitempty"`
	LimitTypes                   []string               `json:"limitTypes,omitempty"`
	MinAvailable                 string                 `json:"minAvailable,omitempty"`
	MaxUnavailable               string                 `json:"maxUnavailable,omitempty"`
	DesiredHealthy               int32                  `json:"desiredHealthy,omitempty"`
	CurrentHealthy               int32                  `json:"currentHealthy,omitempty"`
	DisruptionsAllowed           int32                  `json:"disruptionsAllowed,omitempty"`
	ExpectedPods                 int32                  `json:"expectedPods,omitempty"`
	TargetKind                   string                 `json:"targetKind,omitempty"`
	TargetName                   string                 `json:"targetName,omitempty"`
	MinReplicas                  int32                  `json:"minReplicas,omitempty"`
	MaxReplicas                  int32                  `json:"maxReplicas,omitempty"`
	MetricTypes                  []string               `json:"metricTypes,omitempty"`
	CapacityBytes                int64                  `json:"capacityBytes,omitempty"`
	RequestedBytes               int64                  `json:"requestedBytes,omitempty"`
	AccessModes                  []string               `json:"accessModes,omitempty"`
	ReclaimPolicy                string                 `json:"reclaimPolicy,omitempty"`
	ClaimNamespace               string                 `json:"claimNamespace,omitempty"`
	ClaimName                    string                 `json:"claimName,omitempty"`
	VolumeName                   string                 `json:"volumeName,omitempty"`
	EventType                    string                 `json:"eventType,omitempty"`
	Reason                       string                 `json:"reason,omitempty"`
	Message                      string                 `json:"message,omitempty"`
	InvolvedKind                 string                 `json:"involvedKind,omitempty"`
	InvolvedName                 string                 `json:"involvedName,omitempty"`
	Count                        int32                  `json:"count,omitempty"`
	FirstSeen                    *time.Time             `json:"firstSeen,omitempty"`
	EventTime                    *time.Time             `json:"eventTime,omitempty"`
	CreatedAt                    *time.Time             `json:"createdAt,omitempty"`
	MetricCapabilities           *K8sMetricCapabilities `json:"metricCapabilities,omitempty"`
}

// K8sServicePort describes a Kubernetes service port.
type K8sServicePort struct {
	Name       string `json:"name,omitempty"`
	Protocol   string `json:"protocol,omitempty"`
	Port       int32  `json:"port,omitempty"`
	TargetPort string `json:"targetPort,omitempty"`
	NodePort   int32  `json:"nodePort,omitempty"`
}

// K8sEndpointPort describes a Kubernetes EndpointSlice port.
type K8sEndpointPort struct {
	Name        string `json:"name,omitempty"`
	Protocol    string `json:"protocol,omitempty"`
	Port        int32  `json:"port,omitempty"`
	AppProtocol string `json:"appProtocol,omitempty"`
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
	Total         int                           `json:"total"`
	ByType        map[ResourceType]int          `json:"byType"`
	ByStatus      map[ResourceStatus]int        `json:"byStatus"`
	BySource      map[DataSource]int            `json:"bySource"`
	PolicyPosture *ResourcePolicyPostureSummary `json:"policyPosture,omitempty"`
}
