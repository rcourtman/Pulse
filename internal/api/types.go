package api

import (
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/types"
)

// Common response types for API endpoints

// HealthResponse represents the health check response
type HealthResponse struct {
	Status                      string  `json:"status"`
	Timestamp                   int64   `json:"timestamp"`
	Uptime                      float64 `json:"uptime"`
	ProxyInstallScriptAvailable bool    `json:"proxyInstallScriptAvailable,omitempty"`
	DevModeSSH                  bool    `json:"devModeSSH,omitempty"` // DEV/TEST ONLY: SSH keys allowed in containers
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
	ContainerId     string `json:"containerId,omitempty"`
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
	GuestTypes     map[string]string           `json:"guestTypes"`     // Maps guest ID to type ("vm" or "container")
	Timestamp      int64                       `json:"timestamp"`
	Stats          ChartStats                  `json:"stats"`
}

// ChartStats represents chart statistics
type ChartStats struct {
	OldestDataTimestamp int64 `json:"oldestDataTimestamp"`
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
	Usage []types.MetricPoint `json:"usage"`
	Used  []types.MetricPoint `json:"used"`
	Total []types.MetricPoint `json:"total"`
	Avail []types.MetricPoint `json:"avail"`
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
