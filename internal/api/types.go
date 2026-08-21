package api

import "github.com/rcourtman/pulse-go-rewrite/internal/api/chartapi"

// Common response types for API endpoints

// HealthResponse represents the health check response
type HealthResponse struct {
	Status                      string          `json:"status"`
	Timestamp                   int64           `json:"timestamp"`
	Uptime                      float64         `json:"uptime"`
	ProxyInstallScriptAvailable bool            `json:"proxyInstallScriptAvailable,omitempty"`
	DevModeSSH                  bool            `json:"devModeSSH,omitempty"` // DEV/TEST ONLY: SSH keys allowed in containers
	Dependencies                map[string]bool `json:"dependencies"`
}

func EmptyHealthResponse() HealthResponse {
	return HealthResponse{}.NormalizeCollections()
}

func (r HealthResponse) NormalizeCollections() HealthResponse {
	if r.Dependencies == nil {
		r.Dependencies = map[string]bool{}
	}
	return r
}

// VersionResponse represents version information
type VersionResponse struct {
	Version                  string `json:"version"`
	BuildTime                string `json:"buildTime,omitempty"`
	Build                    string `json:"build,omitempty"`
	GoVersion                string `json:"goVersion,omitempty"`
	Runtime                  string `json:"runtime,omitempty"`
	Channel                  string `json:"channel,omitempty"`
	IsDocker                 bool   `json:"isDocker"`
	IsSourceBuild            bool   `json:"isSourceBuild"`
	IsDevelopment            bool   `json:"isDevelopment"`
	DeploymentType           string `json:"deploymentType,omitempty"`
	AgentUpdateTargetVersion string `json:"agentUpdateTargetVersion,omitempty"`
	UpdateAvailable          bool   `json:"updateAvailable"`
	LatestVersion            string `json:"latestVersion,omitempty"`
	Containerized            bool   `json:"containerized"`
	ContainerID              string `json:"containerId,omitempty"`
}

type ChartResponse = chartapi.ChartResponse
type InfrastructureChartsResponse = chartapi.InfrastructureChartsResponse
type WorkloadChartsResponse = chartapi.WorkloadChartsResponse
type WorkloadsSummaryMetricData = chartapi.WorkloadsSummaryMetricData
type WorkloadsGuestCounts = chartapi.WorkloadsGuestCounts
type WorkloadsSummaryContributor = chartapi.WorkloadsSummaryContributor
type WorkloadsSummaryContributors = chartapi.WorkloadsSummaryContributors
type WorkloadsSummaryBlastRadius = chartapi.WorkloadsSummaryBlastRadius
type WorkloadsSummaryBlastRadiusGroup = chartapi.WorkloadsSummaryBlastRadiusGroup
type WorkloadsSummaryChartsResponse = chartapi.WorkloadsSummaryChartsResponse
type StorageSummaryTrendResponse = chartapi.StorageSummaryTrendResponse
type ChartStats = chartapi.ChartStats
type ChartPointCounts = chartapi.ChartPointCounts
type VMChartData = chartapi.VMChartData
type NodeChartData = chartapi.NodeChartData
type StorageChartData = chartapi.StorageChartData
type StorageChartsResponse = chartapi.StorageChartsResponse
type StoragePoolChartData = chartapi.StoragePoolChartData
type StorageDiskChartData = chartapi.StorageDiskChartData
type MetricPoint = chartapi.MetricPoint

func EmptyChartResponse() ChartResponse { return chartapi.EmptyChartResponse() }
func EmptyInfrastructureChartsResponse() InfrastructureChartsResponse {
	return chartapi.EmptyInfrastructureChartsResponse()
}
func EmptyWorkloadChartsResponse() WorkloadChartsResponse {
	return chartapi.EmptyWorkloadChartsResponse()
}
func EmptyWorkloadsSummaryChartsResponse() WorkloadsSummaryChartsResponse {
	return chartapi.EmptyWorkloadsSummaryChartsResponse()
}
func EmptyStorageSummaryTrendResponse() StorageSummaryTrendResponse {
	return chartapi.EmptyStorageSummaryTrendResponse()
}
func EmptyStorageChartsResponse() StorageChartsResponse { return chartapi.EmptyStorageChartsResponse() }

// AgentVersionResponse represents Docker / Podman module version information.
type AgentVersionResponse struct {
	Version string `json:"version"`
}
