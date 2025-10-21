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
	LegacySSHDetected           bool    `json:"legacySSHDetected,omitempty"`
	RecommendProxyUpgrade       bool    `json:"recommendProxyUpgrade,omitempty"`
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

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
	Code    int    `json:"code,omitempty"`
}

// SuccessResponse represents a generic success response
type SuccessResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// StateResponse represents the full state response
type StateResponse struct {
	Nodes            []models.Node               `json:"nodes"`
	VMs              []models.VM                 `json:"vms"`
	Containers       []models.Container          `json:"containers"`
	DockerHosts      []models.DockerHostFrontend `json:"dockerHosts"`
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

// ConfigResponse represents configuration response
type ConfigResponse struct {
	Nodes    []NodeConfig   `json:"nodes"`
	Settings SettingsConfig `json:"settings"`
}

// NodeConfig represents a node configuration
type NodeConfig struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	Address     string            `json:"address"`
	Port        int               `json:"port,omitempty"`
	Username    string            `json:"username,omitempty"`
	HasPassword bool              `json:"hasPassword"`
	HasToken    bool              `json:"hasToken"`
	SkipTLS     bool              `json:"skipTLS,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// SettingsConfig represents application settings
type SettingsConfig struct {
	CheckInterval   int    `json:"checkInterval"`
	RetentionDays   int    `json:"retentionDays"`
	Theme           string `json:"theme,omitempty"`
	TimeZone        string `json:"timezone,omitempty"`
	NotificationsOn bool   `json:"notificationsOn"`
}

// NodeRequest represents a request to create/update a node
type NodeRequest struct {
	Name     string            `json:"name" validate:"required,min=1,max=100"`
	Type     string            `json:"type" validate:"required,oneof=proxmox pve pbs pmg"`
	Address  string            `json:"address" validate:"required,ip|hostname"`
	Port     int               `json:"port,omitempty" validate:"omitempty,min=1,max=65535"`
	Username string            `json:"username,omitempty"`
	Password string            `json:"password,omitempty"`
	Token    string            `json:"token,omitempty"`
	SkipTLS  bool              `json:"skipTLS,omitempty"`
	Tags     []string          `json:"tags,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// SettingsRequest represents a request to update settings
type SettingsRequest struct {
	CheckInterval   *int    `json:"checkInterval,omitempty" validate:"omitempty,min=10,max=3600"`
	RetentionDays   *int    `json:"retentionDays,omitempty" validate:"omitempty,min=1,max=365"`
	Theme           *string `json:"theme,omitempty" validate:"omitempty,oneof=light dark auto"`
	TimeZone        *string `json:"timezone,omitempty"`
	NotificationsOn *bool   `json:"notificationsOn,omitempty"`
}

// BackupResponse represents backup information
type BackupResponse struct {
	Backups []BackupInfo `json:"backups"`
	Total   int          `json:"total"`
}

// BackupInfo represents a single backup
type BackupInfo struct {
	ID      string    `json:"id"`
	VMID    string    `json:"vmid"`
	Name    string    `json:"name"`
	Type    string    `json:"type"`
	Size    int64     `json:"size"`
	Time    time.Time `json:"time"`
	Node    string    `json:"node"`
	Storage string    `json:"storage,omitempty"`
	Status  string    `json:"status"`
	Notes   string    `json:"notes,omitempty"`
}

// MetricsResponse represents metrics data
type MetricsResponse struct {
	Metrics map[string]MetricData `json:"metrics"`
	Period  string                `json:"period"`
}

// MetricData represents metric data points
type MetricData struct {
	Values     []float64   `json:"values"`
	Timestamps []time.Time `json:"timestamps"`
	Unit       string      `json:"unit,omitempty"`
}

// StorageResponse represents storage information
type StorageResponse struct {
	Storage []StorageInfo `json:"storage"`
	Total   StorageTotals `json:"totals"`
}

// StorageInfo represents storage details
type StorageInfo struct {
	ID         string  `json:"id"`
	Node       string  `json:"node"`
	Storage    string  `json:"storage"`
	Type       string  `json:"type"`
	Status     string  `json:"status"`
	Total      int64   `json:"total"`
	Used       int64   `json:"used"`
	Available  int64   `json:"available"`
	Percentage float64 `json:"percentage"`
}

// StorageTotals represents aggregate storage metrics
type StorageTotals struct {
	Total      int64   `json:"total"`
	Used       int64   `json:"used"`
	Available  int64   `json:"available"`
	Percentage float64 `json:"percentage"`
}

// ChartResponse represents chart data
type ChartResponse struct {
	ChartData   map[string]VMChartData      `json:"data"`
	NodeData    map[string]NodeChartData    `json:"nodeData"`
	StorageData map[string]StorageChartData `json:"storageData"`
	Timestamp   int64                       `json:"timestamp"`
	Stats       ChartStats                  `json:"stats"`
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

// ChartData represents data for a single chart
type ChartData struct {
	Labels   []string  `json:"labels"`
	Datasets []Dataset `json:"datasets"`
}

// Dataset represents a chart dataset
type Dataset struct {
	Label           string    `json:"label"`
	Data            []float64 `json:"data"`
	BackgroundColor string    `json:"backgroundColor,omitempty"`
	BorderColor     string    `json:"borderColor,omitempty"`
}

// DiagnosticsResponse represents system diagnostics
type DiagnosticsResponse struct {
	System      SystemInfo       `json:"system"`
	Connections []ConnectionInfo `json:"connections"`
	Errors      []ErrorInfo      `json:"errors"`
	Performance PerformanceInfo  `json:"performance"`
}

// SystemInfo represents system information
type SystemInfo struct {
	Hostname  string    `json:"hostname"`
	OS        string    `json:"os"`
	Arch      string    `json:"arch"`
	CPUCount  int       `json:"cpuCount"`
	Memory    int64     `json:"memory"`
	GoVersion string    `json:"goVersion"`
	Uptime    float64   `json:"uptime"`
	StartTime time.Time `json:"startTime"`
}

// ConnectionInfo represents connection status
type ConnectionInfo struct {
	Node     string        `json:"node"`
	Type     string        `json:"type"`
	Address  string        `json:"address"`
	Status   string        `json:"status"`
	Latency  time.Duration `json:"latency,omitempty"`
	LastSeen time.Time     `json:"lastSeen,omitempty"`
	Error    string        `json:"error,omitempty"`
}

// ErrorInfo represents error information
type ErrorInfo struct {
	Time    time.Time `json:"time"`
	Level   string    `json:"level"`
	Message string    `json:"message"`
	Source  string    `json:"source,omitempty"`
}

// PerformanceInfo represents performance metrics
type PerformanceInfo struct {
	CPUUsage    float64 `json:"cpuUsage"`
	MemoryUsage int64   `json:"memoryUsage"`
	Goroutines  int     `json:"goroutines"`
	RequestRate float64 `json:"requestRate"`
	ErrorRate   float64 `json:"errorRate"`
}

// SecurityStatusResponse represents security configuration status
type SecurityStatusResponse struct {
	Configured     bool   `json:"configured"`
	Method         string `json:"method"`
	RequiresSetup  bool   `json:"requiresSetup"`
	DeploymentType string `json:"deploymentType,omitempty"`
}

// ExportRequest represents a configuration export request
type ExportRequest struct {
	Passphrase         string `json:"passphrase" validate:"required,min=8"`
	IncludeCredentials bool   `json:"includeCredentials,omitempty"`
}

// ImportRequest represents a configuration import request
type ImportRequest struct {
	Data       string `json:"data" validate:"required"`
	Passphrase string `json:"passphrase" validate:"required"`
	Overwrite  bool   `json:"overwrite,omitempty"`
}

// NotificationTestRequest represents a notification test request
type NotificationTestRequest struct {
	Type    string                 `json:"type" validate:"required,oneof=webhook discord slack email"`
	Config  map[string]interface{} `json:"config" validate:"required"`
	Message string                 `json:"message,omitempty"`
}

// UpdateCheckResponse represents update availability
type UpdateCheckResponse struct {
	CurrentVersion  string    `json:"currentVersion"`
	LatestVersion   string    `json:"latestVersion"`
	UpdateAvailable bool      `json:"updateAvailable"`
	ReleaseNotes    string    `json:"releaseNotes,omitempty"`
	ReleaseDate     time.Time `json:"releaseDate,omitempty"`
	DownloadURL     string    `json:"downloadUrl,omitempty"`
}

// WebSocketMessage represents a WebSocket message
type WebSocketMessage struct {
	Type      string      `json:"type"`
	Data      interface{} `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
}

// LoginRequest represents a login request (if implemented)
type LoginRequest struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

// LoginResponse represents a login response
type LoginResponse struct {
	Success bool   `json:"success"`
	Token   string `json:"token,omitempty"`
	Message string `json:"message,omitempty"`
}

// TestConnectionResponse represents a connection test result
type TestConnectionResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Latency int64  `json:"latency,omitempty"`
	Details string `json:"details,omitempty"`
}

// NodeConnectionResponse represents node connection result
type NodeConnectionResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Nodes   int    `json:"nodes,omitempty"`
}

// DiscoveryResponse represents discovery results
type DiscoveryResponse struct {
	Servers   []DiscoveredServer `json:"servers"`
	Errors    []string           `json:"errors"`
	Cached    bool               `json:"cached"`
	UpdatedAt time.Time          `json:"updatedAt,omitempty"`
}

// DiscoveredServer represents a discovered server
type DiscoveredServer struct {
	IP      string `json:"ip"`
	Port    int    `json:"port"`
	Type    string `json:"type"`
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
	Status  string `json:"status,omitempty"`
}

// AutoRegisterResponse represents auto-registration response
type AutoRegisterResponse struct {
	Status    string `json:"status"`
	Message   string `json:"message"`
	TokenID   string `json:"tokenId,omitempty"`
	TokenName string `json:"tokenName,omitempty"`
}

// ConfigImportResponse represents import response
type ConfigImportResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// ConfigExportResponse represents export response
type ConfigExportResponse struct {
	Status string `json:"status"`
	Data   string `json:"data"`
}

// InstallScriptResponse represents install script response
type InstallScriptResponse struct {
	URL     string `json:"url"`
	Command string `json:"command"`
}

// AgentVersionResponse represents Docker agent version information
type AgentVersionResponse struct {
	Version string `json:"version"`
}
