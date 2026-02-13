package api

import (
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// Common response types for API endpoints

// HealthResponse represents the health check response
type HealthResponse struct {
	Status                      string          `json:"status"`
	Timestamp                   int64           `json:"timestamp"`
	Uptime                      float64         `json:"uptime"`
	ProxyInstallScriptAvailable bool            `json:"proxyInstallScriptAvailable,omitempty"`
	DevModeSSH                  bool            `json:"devModeSSH,omitempty"` // DEV/TEST ONLY: SSH keys allowed in containers
	Dependencies                map[string]bool `json:"dependencies,omitempty"`
}

// VersionResponse represents version information
type VersionResponse struct {
	Version         string `json:"version"`
	BuildTime       string `json:"buildTime,omitempty"`
	Build           string `json:"build,omitempty"`
	GoVersion       string `json:"goVersion,omitempty"`
	Runtime         string `json:"runtime,omitempty"`
	Channel         string `json:"channel,omitempty"`
	IsDocker        bool   `json:"isDocker"`
	IsSourceBuild   bool   `json:"isSourceBuild"`
	IsDevelopment   bool   `json:"isDevelopment"`
	DeploymentType  string `json:"deploymentType,omitempty"`
	UpdateAvailable bool   `json:"updateAvailable"`
	LatestVersion   string `json:"latestVersion,omitempty"`
	Containerized   bool   `json:"containerized"`
	ContainerID     string `json:"containerId,omitempty"`
}

// StateResponse represents the full state response
type StateResponse struct {
	Nodes            []models.Node               `json:"nodes"`
	VMs              []models.VM                 `json:"vms"`
	Containers       []models.Container          `json:"containers"`
	DockerHosts      []models.DockerHostFrontend `json:"dockerHosts"`
	Hosts            []models.HostFrontend       `json:"hosts"`
	Storage          []models.Storage            `json:"storage"`
	CephClusters     []models.CephCluster        `json:"cephClusters"`
	PBSInstances     []models.PBSInstance        `json:"pbs"`
	PMGInstances     []models.PMGInstance        `json:"pmg"`
	PBSBackups       []models.PBSBackup          `json:"pbsBackups"`
	PMGBackups       []models.PMGBackup          `json:"pmgBackups"`
	Backups          models.Backups              `json:"backups"`
	Metrics          []models.Metric             `json:"metrics"`
	PVEBackups       models.PVEBackups           `json:"pveBackups"`
	Performance      models.Performance          `json:"performance"`
	ConnectionHealth map[string]bool             `json:"connectionHealth"`
	Stats            models.Stats                `json:"stats"`
	ActiveAlerts     []models.Alert              `json:"activeAlerts"`
	RecentlyResolved []models.ResolvedAlert      `json:"recentlyResolved"`
	LastUpdate       time.Time                   `json:"lastUpdate"`
}

// ChartResponse represents chart data
type ChartResponse struct {
	ChartData      map[string]VMChartData      `json:"data"`
	NodeData       map[string]NodeChartData    `json:"nodeData"`
	StorageData    map[string]StorageChartData `json:"storageData"`
	DockerData     map[string]VMChartData      `json:"dockerData"`     // Docker container metrics (keyed by container ID)
	DockerHostData map[string]VMChartData      `json:"dockerHostData"` // Docker host metrics (keyed by host ID)
	HostData       map[string]VMChartData      `json:"hostData"`       // Unified host agent metrics (keyed by host ID)
	GuestTypes     map[string]string           `json:"guestTypes"`     // Maps guest ID to type ("vm", "container", "k8s")
	Timestamp      int64                       `json:"timestamp"`
	Stats          ChartStats                  `json:"stats"`
}

// InfrastructureChartsResponse is a lightweight variant of ChartResponse used by
// infra-only clients (Infrastructure summary sparklines, prewarm caches).
// It avoids the heavy guest/storage chart payload and associated compute.
type InfrastructureChartsResponse struct {
	NodeData       map[string]NodeChartData `json:"nodeData"`
	DockerHostData map[string]VMChartData   `json:"dockerHostData,omitempty"` // Docker host metrics (keyed by host ID)
	HostData       map[string]VMChartData   `json:"hostData,omitempty"`       // Unified host agent metrics (keyed by host ID)
	Timestamp      int64                    `json:"timestamp"`
	Stats          ChartStats               `json:"stats"`
}

// WorkloadChartsResponse is a lightweight chart payload used by Workloads
// summary sparklines. It intentionally excludes infrastructure/storage series.
type WorkloadChartsResponse struct {
	ChartData  map[string]VMChartData `json:"data"`       // Workload metrics keyed by workload ID
	DockerData map[string]VMChartData `json:"dockerData"` // Docker container metrics keyed by container ID
	GuestTypes map[string]string      `json:"guestTypes"` // Maps guest ID to type ("vm", "container", "k8s")
	Timestamp  int64                  `json:"timestamp"`  // Unix timestamp in milliseconds
	Stats      ChartStats             `json:"stats"`      // Includes pointCounts + source hints
}

// WorkloadsSummaryMetricData captures aggregate workload trend lines for a
// single metric (median and p95 across workloads).
type WorkloadsSummaryMetricData struct {
	P50 []MetricPoint `json:"p50"`
	P95 []MetricPoint `json:"p95"`
}

// WorkloadsGuestCounts captures workload counts used by the workloads summary
// cards for quick context.
type WorkloadsGuestCounts struct {
	Total   int `json:"total"`
	Running int `json:"running"`
	Stopped int `json:"stopped"`
}

// WorkloadsSummaryContributor identifies a high-impact workload for a metric.
type WorkloadsSummaryContributor struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

// WorkloadsSummaryContributors groups top contributors by metric.
type WorkloadsSummaryContributors struct {
	CPU     []WorkloadsSummaryContributor `json:"cpu"`
	Memory  []WorkloadsSummaryContributor `json:"memory"`
	Disk    []WorkloadsSummaryContributor `json:"disk"`
	Network []WorkloadsSummaryContributor `json:"network"`
}

// WorkloadsSummaryBlastRadius describes how concentrated each metric pressure is.
type WorkloadsSummaryBlastRadius struct {
	Scope           string  `json:"scope"` // idle, concentrated, mixed, distributed
	Top3Share       float64 `json:"top3Share"`
	ActiveWorkloads int     `json:"activeWorkloads"`
}

// WorkloadsSummaryBlastRadiusGroup groups blast-radius insights by metric.
type WorkloadsSummaryBlastRadiusGroup struct {
	CPU     WorkloadsSummaryBlastRadius `json:"cpu"`
	Memory  WorkloadsSummaryBlastRadius `json:"memory"`
	Disk    WorkloadsSummaryBlastRadius `json:"disk"`
	Network WorkloadsSummaryBlastRadius `json:"network"`
}

// WorkloadsSummaryChartsResponse is a compact response for workloads top-card
// sparklines. It avoids returning per-workload time series.
type WorkloadsSummaryChartsResponse struct {
	CPU             WorkloadsSummaryMetricData       `json:"cpu"`
	Memory          WorkloadsSummaryMetricData       `json:"memory"`
	Disk            WorkloadsSummaryMetricData       `json:"disk"`
	Network         WorkloadsSummaryMetricData       `json:"network"`
	GuestCounts     WorkloadsGuestCounts             `json:"guestCounts"`
	TopContributors WorkloadsSummaryContributors     `json:"topContributors"`
	BlastRadius     WorkloadsSummaryBlastRadiusGroup `json:"blastRadius"`
	Timestamp       int64                            `json:"timestamp"`
	Stats           ChartStats                       `json:"stats"`
}

// ChartStats represents chart statistics
type ChartStats struct {
	OldestDataTimestamp   int64            `json:"oldestDataTimestamp"`
	Range                 string           `json:"range,omitempty"`
	RangeSeconds          int64            `json:"rangeSeconds,omitempty"`
	MetricsStoreEnabled   bool             `json:"metricsStoreEnabled"`
	PrimarySourceHint     string           `json:"primarySourceHint,omitempty"`
	InMemoryThresholdSecs int64            `json:"inMemoryThresholdSecs,omitempty"`
	PointCounts           ChartPointCounts `json:"pointCounts,omitempty"`
}

// ChartPointCounts summarizes how many points were returned in /api/charts.
type ChartPointCounts struct {
	Total            int `json:"total,omitempty"`
	Guests           int `json:"guests,omitempty"`
	Nodes            int `json:"nodes,omitempty"`
	Storage          int `json:"storage,omitempty"`
	DockerContainers int `json:"dockerContainers,omitempty"`
	DockerHosts      int `json:"dockerHosts,omitempty"`
	Hosts            int `json:"hosts,omitempty"`
}

// VMChartData represents chart data for a VM
type VMChartData map[string][]MetricPoint

// NodeChartData represents chart data for a node
type NodeChartData map[string][]MetricPoint

// StorageChartData represents chart data for storage
type StorageChartData map[string][]MetricPoint

// StorageChartsResponse represents storage charts API response
type StorageChartsResponse map[string]StorageMetrics

// StorageMetrics represents storage metrics data
type StorageMetrics struct {
	Usage []models.MetricPoint `json:"usage"`
	Used  []models.MetricPoint `json:"used"`
	Total []models.MetricPoint `json:"total"`
	Avail []models.MetricPoint `json:"avail"`
}

// MetricPoint represents a single metric data point
type MetricPoint struct {
	Timestamp int64   `json:"timestamp"`
	Value     float64 `json:"value"`
}

// AgentVersionResponse represents Docker agent version information
type AgentVersionResponse struct {
	Version string `json:"version"`
}
