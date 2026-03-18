package api

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

// ChartResponse represents chart data
type ChartResponse struct {
	ChartData      map[string]VMChartData      `json:"data"`
	NodeData       map[string]NodeChartData    `json:"nodeData"`
	StorageData    map[string]StorageChartData `json:"storageData"`
	DockerData     map[string]VMChartData      `json:"dockerData"`     // Docker container metrics (keyed by container ID)
	DockerHostData map[string]VMChartData      `json:"dockerHostData"` // Docker host metrics (keyed by host ID)
	AgentData      map[string]VMChartData      `json:"agentData"`      // Unified agent metrics (keyed by agent ID)
	GuestTypes     map[string]string           `json:"guestTypes"`     // Maps guest ID to type ("vm", "system-container", "k8s")
	Timestamp      int64                       `json:"timestamp"`
	Stats          ChartStats                  `json:"stats"`
}

func EmptyChartResponse() ChartResponse {
	return ChartResponse{}.NormalizeCollections()
}

func (r ChartResponse) NormalizeCollections() ChartResponse {
	if r.ChartData == nil {
		r.ChartData = map[string]VMChartData{}
	}
	if r.NodeData == nil {
		r.NodeData = map[string]NodeChartData{}
	}
	if r.StorageData == nil {
		r.StorageData = map[string]StorageChartData{}
	}
	if r.DockerData == nil {
		r.DockerData = map[string]VMChartData{}
	}
	if r.DockerHostData == nil {
		r.DockerHostData = map[string]VMChartData{}
	}
	if r.AgentData == nil {
		r.AgentData = map[string]VMChartData{}
	}
	if r.GuestTypes == nil {
		r.GuestTypes = map[string]string{}
	}
	r.Stats = r.Stats.NormalizeCollections()
	return r
}

// InfrastructureChartsResponse is a lightweight variant of ChartResponse used by
// infra-only clients (Infrastructure summary sparklines, prewarm caches).
// It avoids the heavy guest/storage chart payload and associated compute.
type InfrastructureChartsResponse struct {
	NodeData       map[string]NodeChartData `json:"nodeData"`
	DockerHostData map[string]VMChartData   `json:"dockerHostData"` // Docker host metrics (keyed by host ID)
	AgentData      map[string]VMChartData   `json:"agentData"`      // Unified agent metrics (keyed by agent ID)
	Timestamp      int64                    `json:"timestamp"`
	Stats          ChartStats               `json:"stats"`
}

func EmptyInfrastructureChartsResponse() InfrastructureChartsResponse {
	return InfrastructureChartsResponse{}.NormalizeCollections()
}

func (r InfrastructureChartsResponse) NormalizeCollections() InfrastructureChartsResponse {
	if r.NodeData == nil {
		r.NodeData = map[string]NodeChartData{}
	}
	if r.DockerHostData == nil {
		r.DockerHostData = map[string]VMChartData{}
	}
	if r.AgentData == nil {
		r.AgentData = map[string]VMChartData{}
	}
	r.Stats = r.Stats.NormalizeCollections()
	return r
}

// WorkloadChartsResponse is a lightweight chart payload used by Workloads
// summary sparklines. It intentionally excludes infrastructure/storage series.
type WorkloadChartsResponse struct {
	ChartData  map[string]VMChartData `json:"data"`       // Workload metrics keyed by workload ID
	DockerData map[string]VMChartData `json:"dockerData"` // Docker container metrics keyed by container ID
	GuestTypes map[string]string      `json:"guestTypes"` // Maps guest ID to type ("vm", "system-container", "k8s")
	Timestamp  int64                  `json:"timestamp"`  // Unix timestamp in milliseconds
	Stats      ChartStats             `json:"stats"`      // Includes pointCounts + source hints
}

func EmptyWorkloadChartsResponse() WorkloadChartsResponse {
	return WorkloadChartsResponse{}.NormalizeCollections()
}

func (r WorkloadChartsResponse) NormalizeCollections() WorkloadChartsResponse {
	if r.ChartData == nil {
		r.ChartData = map[string]VMChartData{}
	}
	if r.DockerData == nil {
		r.DockerData = map[string]VMChartData{}
	}
	if r.GuestTypes == nil {
		r.GuestTypes = map[string]string{}
	}
	r.Stats = r.Stats.NormalizeCollections()
	return r
}

// WorkloadsSummaryMetricData captures aggregate workload trend lines for a
// single metric (median and p95 across workloads).
type WorkloadsSummaryMetricData struct {
	P50 []MetricPoint `json:"p50"`
	P95 []MetricPoint `json:"p95"`
}

func (m WorkloadsSummaryMetricData) NormalizeCollections() WorkloadsSummaryMetricData {
	if m.P50 == nil {
		m.P50 = []MetricPoint{}
	}
	if m.P95 == nil {
		m.P95 = []MetricPoint{}
	}
	return m
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

func (c WorkloadsSummaryContributors) NormalizeCollections() WorkloadsSummaryContributors {
	if c.CPU == nil {
		c.CPU = []WorkloadsSummaryContributor{}
	}
	if c.Memory == nil {
		c.Memory = []WorkloadsSummaryContributor{}
	}
	if c.Disk == nil {
		c.Disk = []WorkloadsSummaryContributor{}
	}
	if c.Network == nil {
		c.Network = []WorkloadsSummaryContributor{}
	}
	return c
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

func EmptyWorkloadsSummaryChartsResponse() WorkloadsSummaryChartsResponse {
	return WorkloadsSummaryChartsResponse{}.NormalizeCollections()
}

func (r WorkloadsSummaryChartsResponse) NormalizeCollections() WorkloadsSummaryChartsResponse {
	r.CPU = r.CPU.NormalizeCollections()
	r.Memory = r.Memory.NormalizeCollections()
	r.Disk = r.Disk.NormalizeCollections()
	r.Network = r.Network.NormalizeCollections()
	r.TopContributors = r.TopContributors.NormalizeCollections()
	r.Stats = r.Stats.NormalizeCollections()
	return r
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

func (s ChartStats) NormalizeCollections() ChartStats {
	return s
}

// ChartPointCounts summarizes how many points were returned in /api/charts.
type ChartPointCounts struct {
	Total            int `json:"total,omitempty"`
	Guests           int `json:"guests,omitempty"`
	Nodes            int `json:"nodes,omitempty"`
	Storage          int `json:"storage,omitempty"`
	DockerContainers int `json:"dockerContainers,omitempty"`
	DockerHosts      int `json:"dockerHosts,omitempty"`
	Agents           int `json:"agents,omitempty"`
}

// VMChartData represents chart data for a VM
type VMChartData map[string][]MetricPoint

// NodeChartData represents chart data for a node
type NodeChartData map[string][]MetricPoint

// StorageChartData represents chart data for storage
type StorageChartData map[string][]MetricPoint

// StorageChartsResponse represents storage charts API response.
// It includes both pool-level capacity metrics and physical disk metrics.
type StorageChartsResponse struct {
	Pools map[string]StoragePoolChartData `json:"pools"`
	Disks map[string]StorageDiskChartData `json:"disks"`
	Stats ChartStats                      `json:"stats"`
}

func EmptyStorageChartsResponse() StorageChartsResponse {
	return StorageChartsResponse{}.NormalizeCollections()
}

func (r StorageChartsResponse) NormalizeCollections() StorageChartsResponse {
	if r.Pools == nil {
		r.Pools = map[string]StoragePoolChartData{}
	}
	if r.Disks == nil {
		r.Disks = map[string]StorageDiskChartData{}
	}
	for key, pool := range r.Pools {
		r.Pools[key] = pool.NormalizeCollections()
	}
	for key, disk := range r.Disks {
		r.Disks[key] = disk.NormalizeCollections()
	}
	r.Stats = r.Stats.NormalizeCollections()
	return r
}

// StoragePoolChartData holds per-pool capacity time-series.
type StoragePoolChartData struct {
	Name  string        `json:"name"`
	Usage []MetricPoint `json:"usage"`
	Used  []MetricPoint `json:"used"`
	Avail []MetricPoint `json:"avail"`
}

func (d StoragePoolChartData) NormalizeCollections() StoragePoolChartData {
	if d.Usage == nil {
		d.Usage = []MetricPoint{}
	}
	if d.Used == nil {
		d.Used = []MetricPoint{}
	}
	if d.Avail == nil {
		d.Avail = []MetricPoint{}
	}
	return d
}

// StorageDiskChartData holds per-disk temperature time-series.
type StorageDiskChartData struct {
	Name        string        `json:"name"`
	Node        string        `json:"node"`
	Temperature []MetricPoint `json:"temperature"`
}

func (d StorageDiskChartData) NormalizeCollections() StorageDiskChartData {
	if d.Temperature == nil {
		d.Temperature = []MetricPoint{}
	}
	return d
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
