package unifiedresources

import (
	"encoding/json"
	"time"
)

// LegacyResource mirrors the legacy V1 resource shape used by websocket and AI consumers.
type LegacyResource struct {
	ID          string             `json:"id"`
	Type        LegacyResourceType `json:"type"`
	Name        string             `json:"name"`
	DisplayName string             `json:"displayName"`

	PlatformID   string             `json:"platformId"`
	PlatformType LegacyPlatformType `json:"platformType"`
	SourceType   LegacySourceType   `json:"sourceType"`

	ParentID  string `json:"parentId,omitempty"`
	ClusterID string `json:"clusterId,omitempty"`

	Status      LegacyResourceStatus `json:"status"`
	CPU         *LegacyMetricValue   `json:"cpu,omitempty"`
	Memory      *LegacyMetricValue   `json:"memory,omitempty"`
	Disk        *LegacyMetricValue   `json:"disk,omitempty"`
	Network     *LegacyNetworkMetric `json:"network,omitempty"`
	Temperature *float64             `json:"temperature,omitempty"`
	Uptime      *int64               `json:"uptime,omitempty"`

	Tags     []string          `json:"tags,omitempty"`
	Labels   map[string]string `json:"labels,omitempty"`
	LastSeen time.Time         `json:"lastSeen"`
	Alerts   []LegacyAlert     `json:"alerts,omitempty"`

	PlatformData  json.RawMessage `json:"platformData,omitempty"`
	Identity      *LegacyIdentity `json:"identity,omitempty"`
	SchemaVersion int             `json:"schemaVersion"`
}

type LegacyResourceType string

const (
	LegacyResourceTypeNode            LegacyResourceType = "node"
	LegacyResourceTypeHost            LegacyResourceType = "host"
	LegacyResourceTypeDockerHost      LegacyResourceType = "docker-host"
	LegacyResourceTypeK8sCluster      LegacyResourceType = "k8s-cluster"
	LegacyResourceTypeK8sNode         LegacyResourceType = "k8s-node"
	LegacyResourceTypeTrueNAS         LegacyResourceType = "truenas"
	LegacyResourceTypeVM              LegacyResourceType = "vm"
	LegacyResourceTypeContainer       LegacyResourceType = "container"
	LegacyResourceTypeOCIContainer    LegacyResourceType = "oci-container"
	LegacyResourceTypeDockerContainer LegacyResourceType = "docker-container"
	LegacyResourceTypePod             LegacyResourceType = "pod"
	LegacyResourceTypeJail            LegacyResourceType = "jail"
	LegacyResourceTypeDockerService   LegacyResourceType = "docker-service"
	LegacyResourceTypeK8sDeployment   LegacyResourceType = "k8s-deployment"
	LegacyResourceTypeK8sService      LegacyResourceType = "k8s-service"
	LegacyResourceTypeStorage         LegacyResourceType = "storage"
	LegacyResourceTypeDatastore       LegacyResourceType = "datastore"
	LegacyResourceTypePool            LegacyResourceType = "pool"
	LegacyResourceTypeDataset         LegacyResourceType = "dataset"
	LegacyResourceTypePBS             LegacyResourceType = "pbs"
	LegacyResourceTypePMG             LegacyResourceType = "pmg"
)

type LegacyPlatformType string

const (
	LegacyPlatformProxmoxPVE LegacyPlatformType = "proxmox-pve"
	LegacyPlatformProxmoxPBS LegacyPlatformType = "proxmox-pbs"
	LegacyPlatformProxmoxPMG LegacyPlatformType = "proxmox-pmg"
	LegacyPlatformDocker     LegacyPlatformType = "docker"
	LegacyPlatformKubernetes LegacyPlatformType = "kubernetes"
	LegacyPlatformTrueNAS    LegacyPlatformType = "truenas"
	LegacyPlatformHostAgent  LegacyPlatformType = "host-agent"
)

type LegacySourceType string

const (
	LegacySourceAPI    LegacySourceType = "api"
	LegacySourceAgent  LegacySourceType = "agent"
	LegacySourceHybrid LegacySourceType = "hybrid"
)

type LegacyResourceStatus string

const (
	LegacyStatusOnline   LegacyResourceStatus = "online"
	LegacyStatusOffline  LegacyResourceStatus = "offline"
	LegacyStatusRunning  LegacyResourceStatus = "running"
	LegacyStatusStopped  LegacyResourceStatus = "stopped"
	LegacyStatusDegraded LegacyResourceStatus = "degraded"
	LegacyStatusPaused   LegacyResourceStatus = "paused"
	LegacyStatusUnknown  LegacyResourceStatus = "unknown"
)

type LegacyMetricValue struct {
	Current float64 `json:"current"`
	Total   *int64  `json:"total,omitempty"`
	Used    *int64  `json:"used,omitempty"`
	Free    *int64  `json:"free,omitempty"`
}

type LegacyNetworkMetric struct {
	RXBytes int64 `json:"rxBytes"`
	TXBytes int64 `json:"txBytes"`
}

type LegacyAlert struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	Value     float64   `json:"value"`
	Threshold float64   `json:"threshold"`
	StartTime time.Time `json:"startTime"`
}

type LegacyIdentity struct {
	Hostname  string   `json:"hostname,omitempty"`
	MachineID string   `json:"machineId,omitempty"`
	IPs       []string `json:"ips,omitempty"`
}

const LegacySchemaVersion = 1

func (r *LegacyResource) EffectiveDisplayName() string {
	if r.DisplayName != "" {
		return r.DisplayName
	}
	return r.Name
}

func (r *LegacyResource) IsInfrastructure() bool {
	switch r.Type {
	case LegacyResourceTypeNode, LegacyResourceTypeHost, LegacyResourceTypeDockerHost, LegacyResourceTypeK8sCluster, LegacyResourceTypeK8sNode, LegacyResourceTypeTrueNAS:
		return true
	default:
		return false
	}
}

func (r *LegacyResource) IsWorkload() bool {
	switch r.Type {
	case LegacyResourceTypeVM, LegacyResourceTypeContainer, LegacyResourceTypeOCIContainer, LegacyResourceTypeDockerContainer, LegacyResourceTypePod, LegacyResourceTypeJail:
		return true
	default:
		return false
	}
}

func (r *LegacyResource) CPUPercent() float64 {
	if r.CPU == nil {
		return 0
	}
	return r.CPU.Current
}

func (r *LegacyResource) MemoryPercent() float64 {
	if r.Memory == nil {
		return 0
	}
	if r.Memory.Total != nil && *r.Memory.Total > 0 && r.Memory.Used != nil {
		return float64(*r.Memory.Used) / float64(*r.Memory.Total) * 100
	}
	return r.Memory.Current
}

func (r *LegacyResource) DiskPercent() float64 {
	if r.Disk == nil {
		return 0
	}
	if r.Disk.Total != nil && *r.Disk.Total > 0 && r.Disk.Used != nil {
		return float64(*r.Disk.Used) / float64(*r.Disk.Total) * 100
	}
	return r.Disk.Current
}

type LegacyStoreStats struct {
	TotalResources      int                          `json:"totalResources"`
	SuppressedResources int                          `json:"suppressedResources"`
	ByType              map[LegacyResourceType]int   `json:"byType"`
	ByPlatform          map[LegacyPlatformType]int   `json:"byPlatform"`
	ByStatus            map[LegacyResourceStatus]int `json:"byStatus"`
	WithAlerts          int                          `json:"withAlerts"`
	LastUpdated         string                       `json:"lastUpdated"`
}

type LegacyResourceSummary struct {
	TotalResources int
	Healthy        int
	Degraded       int
	Offline        int
	WithAlerts     int
	ByType         map[LegacyResourceType]LegacyTypeSummary
	ByPlatform     map[LegacyPlatformType]LegacyPlatformSummary
}

type LegacyTypeSummary struct {
	Count              int
	TotalCPUPercent    float64
	TotalMemoryPercent float64
	AvgCPUPercent      float64
	AvgMemoryPercent   float64
}

type LegacyPlatformSummary struct {
	Count int
}
