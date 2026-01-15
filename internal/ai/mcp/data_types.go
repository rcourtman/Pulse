package mcp

import "time"

// MetricPoint represents a single metric data point
type MetricPoint struct {
	Timestamp time.Time `json:"timestamp"`
	CPU       float64   `json:"cpu"`
	Memory    float64   `json:"memory"`
	Disk      float64   `json:"disk,omitempty"`
}

// ResourceMetricsSummary summarizes metrics for a resource over a period
type ResourceMetricsSummary struct {
	ResourceID   string  `json:"resource_id"`
	ResourceName string  `json:"resource_name"`
	ResourceType string  `json:"resource_type"`
	AvgCPU       float64 `json:"avg_cpu"`
	MaxCPU       float64 `json:"max_cpu"`
	AvgMemory    float64 `json:"avg_memory"`
	MaxMemory    float64 `json:"max_memory"`
	AvgDisk      float64 `json:"avg_disk,omitempty"`
	MaxDisk      float64 `json:"max_disk,omitempty"`
	Trend        string  `json:"trend"` // "stable", "growing", "declining"
}

// MetricBaseline represents learned normal behavior for a metric
type MetricBaseline struct {
	Mean   float64 `json:"mean"`
	StdDev float64 `json:"std_dev"`
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
}

// Pattern represents a detected operational pattern
type Pattern struct {
	ResourceID   string    `json:"resource_id"`
	ResourceName string    `json:"resource_name"`
	PatternType  string    `json:"pattern_type"` // "recurring_spike", "gradual_growth", "weekly_cycle"
	Description  string    `json:"description"`
	Confidence   float64   `json:"confidence"`
	LastSeen     time.Time `json:"last_seen"`
}

// Prediction represents a predicted future issue
type Prediction struct {
	ResourceID     string    `json:"resource_id"`
	ResourceName   string    `json:"resource_name"`
	IssueType      string    `json:"issue_type"` // "disk_full", "memory_exhaustion", etc.
	PredictedTime  time.Time `json:"predicted_time"`
	Confidence     float64   `json:"confidence"`
	Recommendation string    `json:"recommendation"`
}

// ActiveAlert represents an active alert
type ActiveAlert struct {
	ID           string    `json:"id"`
	ResourceID   string    `json:"resource_id"`
	ResourceName string    `json:"resource_name"`
	Type         string    `json:"type"` // "cpu", "memory", "disk", "offline"
	Severity     string    `json:"severity"`
	Value        float64   `json:"value"`
	Threshold    float64   `json:"threshold"`
	StartTime    time.Time `json:"start_time"`
	Message      string    `json:"message"`
}

// Finding represents a patrol finding
type Finding struct {
	ID             string    `json:"id"`
	Key            string    `json:"key"`
	Severity       string    `json:"severity"`
	Category       string    `json:"category"`
	ResourceID     string    `json:"resource_id"`
	ResourceName   string    `json:"resource_name"`
	ResourceType   string    `json:"resource_type"`
	Title          string    `json:"title"`
	Description    string    `json:"description"`
	Recommendation string    `json:"recommendation"`
	Evidence       string    `json:"evidence"`
	DetectedAt     time.Time `json:"detected_at"`
	LastSeenAt     time.Time `json:"last_seen_at"`
	TimesRaised    int       `json:"times_raised"`
}

// GuestInfo represents resolved guest information
type GuestInfo struct {
	VMID     int
	Name     string
	Node     string
	Type     string // "vm" or "lxc"
	Status   string
	Instance string
}

// ========== JSON Response Types ==========

// CapabilitiesResponse is returned by pulse_get_capabilities
type CapabilitiesResponse struct {
	ControlLevel    string       `json:"control_level"`
	Features        FeatureFlags `json:"features"`
	ProtectedGuests []string     `json:"protected_guests,omitempty"`
	ConnectedAgents int          `json:"connected_agents"`
	Version         string       `json:"version"`
}

// FeatureFlags indicates which features are available
type FeatureFlags struct {
	MetricsHistory bool `json:"metrics_history"`
	Baselines      bool `json:"baselines"`
	Patterns       bool `json:"patterns"`
	Alerts         bool `json:"alerts"`
	Findings       bool `json:"findings"`
	Backups        bool `json:"backups"`
	Storage        bool `json:"storage"`
	DiskHealth     bool `json:"disk_health"`
	AgentProfiles  bool `json:"agent_profiles"`
	Control        bool `json:"control"`
}

// InfrastructureResponse is returned by pulse_list_infrastructure
type InfrastructureResponse struct {
	Nodes       []NodeSummary       `json:"nodes,omitempty"`
	VMs         []VMSummary         `json:"vms,omitempty"`
	Containers  []ContainerSummary  `json:"containers,omitempty"`
	DockerHosts []DockerHostSummary `json:"docker_hosts,omitempty"`
	Total       TotalCounts         `json:"total"`
	Pagination  *PaginationInfo     `json:"pagination,omitempty"`
}

// NodeSummary is a summarized node for list responses
type NodeSummary struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	ID     string `json:"id,omitempty"`
}

// VMSummary is a summarized VM for list responses
type VMSummary struct {
	VMID   int     `json:"vmid"`
	Name   string  `json:"name"`
	Status string  `json:"status"`
	Node   string  `json:"node"`
	CPU    float64 `json:"cpu_percent,omitempty"`
	Memory float64 `json:"memory_percent,omitempty"`
}

// ContainerSummary is a summarized LXC container for list responses
type ContainerSummary struct {
	VMID   int     `json:"vmid"`
	Name   string  `json:"name"`
	Status string  `json:"status"`
	Node   string  `json:"node"`
	CPU    float64 `json:"cpu_percent,omitempty"`
	Memory float64 `json:"memory_percent,omitempty"`
}

// DockerHostSummary is a summarized Docker host for list responses
type DockerHostSummary struct {
	ID             string                   `json:"id"`
	Hostname       string                   `json:"hostname"`
	DisplayName    string                   `json:"display_name,omitempty"`
	ContainerCount int                      `json:"container_count"`
	Containers     []DockerContainerSummary `json:"containers,omitempty"`
}

// DockerContainerSummary is a summarized Docker container
type DockerContainerSummary struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	State  string `json:"state"`
	Image  string `json:"image"`
	Health string `json:"health,omitempty"`
}

// TotalCounts for infrastructure response
type TotalCounts struct {
	Nodes       int `json:"nodes"`
	VMs         int `json:"vms"`
	Containers  int `json:"containers"`
	DockerHosts int `json:"docker_hosts"`
}

// PaginationInfo describes pagination state
type PaginationInfo struct {
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// ResourceResponse is returned by pulse_get_resource
type ResourceResponse struct {
	Type            string            `json:"type"` // "vm", "container", "docker"
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	Status          string            `json:"status"`
	Node            string            `json:"node,omitempty"`
	Host            string            `json:"host,omitempty"`
	CPU             ResourceCPU       `json:"cpu"`
	Memory          ResourceMemory    `json:"memory"`
	Disk            *ResourceDisk     `json:"disk,omitempty"`
	OS              string            `json:"os,omitempty"`
	Tags            []string          `json:"tags,omitempty"`
	Networks        []NetworkInfo     `json:"networks,omitempty"`
	Ports           []PortInfo        `json:"ports,omitempty"`
	Mounts          []MountInfo       `json:"mounts,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
	LastBackup      *time.Time        `json:"last_backup,omitempty"`
	Image           string            `json:"image,omitempty"`
	Health          string            `json:"health,omitempty"`
	RestartCount    int               `json:"restart_count,omitempty"`
	UpdateAvailable bool              `json:"update_available,omitempty"`
}

// ResourceCPU describes CPU usage
type ResourceCPU struct {
	Percent float64 `json:"percent"`
	Cores   int     `json:"cores"`
}

// ResourceMemory describes memory usage
type ResourceMemory struct {
	Percent float64 `json:"percent"`
	UsedGB  float64 `json:"used_gb"`
	TotalGB float64 `json:"total_gb"`
}

// ResourceDisk describes disk usage
type ResourceDisk struct {
	Percent float64 `json:"percent"`
	UsedGB  float64 `json:"used_gb"`
	TotalGB float64 `json:"total_gb"`
}

// NetworkInfo describes a network interface
type NetworkInfo struct {
	Name      string   `json:"name"`
	Addresses []string `json:"addresses"`
}

// PortInfo describes a port mapping
type PortInfo struct {
	Private  int    `json:"private"`
	Public   int    `json:"public,omitempty"`
	Protocol string `json:"protocol"`
	IP       string `json:"ip,omitempty"`
}

// MountInfo describes a volume mount
type MountInfo struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
	ReadWrite   bool   `json:"rw"`
}

// URLFetchResponse is returned by pulse_get_url_content
type URLFetchResponse struct {
	URL        string            `json:"url"`
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
	Error      string            `json:"error,omitempty"`
}

// AlertsResponse is returned by pulse_list_alerts
type AlertsResponse struct {
	Alerts     []ActiveAlert   `json:"alerts"`
	Count      int             `json:"count"`
	Pagination *PaginationInfo `json:"pagination,omitempty"`
}

// FindingsResponse is returned by pulse_list_findings
type FindingsResponse struct {
	Active     []Finding       `json:"active"`
	Dismissed  []Finding       `json:"dismissed,omitempty"`
	Counts     FindingCounts   `json:"counts"`
	Pagination *PaginationInfo `json:"pagination,omitempty"`
}

// FindingCounts for findings response
type FindingCounts struct {
	Active    int `json:"active"`
	Dismissed int `json:"dismissed"`
}

// MetricsResponse is returned by pulse_get_metrics
type MetricsResponse struct {
	ResourceID string                            `json:"resource_id,omitempty"`
	Period     string                            `json:"period"`
	Points     []MetricPoint                     `json:"points,omitempty"`
	Summary    map[string]ResourceMetricsSummary `json:"summary,omitempty"`
}

// BaselinesResponse is returned by pulse_get_baselines
type BaselinesResponse struct {
	ResourceID string                                `json:"resource_id,omitempty"`
	Baselines  map[string]map[string]*MetricBaseline `json:"baselines"` // resourceID -> metric -> baseline
}

// PatternsResponse is returned by pulse_get_patterns
type PatternsResponse struct {
	Patterns    []Pattern    `json:"patterns"`
	Predictions []Prediction `json:"predictions"`
}

// BackupsResponse is returned by pulse_list_backups
type BackupsResponse struct {
	PBS         []PBSBackupSummary  `json:"pbs,omitempty"`
	PVE         []PVEBackupSummary  `json:"pve,omitempty"`
	PBSServers  []PBSServerSummary  `json:"pbs_servers,omitempty"`
	RecentTasks []BackupTaskSummary `json:"recent_tasks,omitempty"`
	Pagination  *PaginationInfo     `json:"pagination,omitempty"`
}

// PBSBackupSummary is a summarized PBS backup
type PBSBackupSummary struct {
	VMID       string    `json:"vmid"`
	BackupType string    `json:"backup_type"`
	BackupTime time.Time `json:"backup_time"`
	Instance   string    `json:"instance"`
	Datastore  string    `json:"datastore"`
	SizeGB     float64   `json:"size_gb"`
	Verified   bool      `json:"verified"`
	Protected  bool      `json:"protected"`
}

// PVEBackupSummary is a summarized PVE backup
type PVEBackupSummary struct {
	VMID       int       `json:"vmid"`
	BackupTime time.Time `json:"backup_time"`
	SizeGB     float64   `json:"size_gb"`
	Storage    string    `json:"storage"`
}

// PBSServerSummary is a summarized PBS server
type PBSServerSummary struct {
	Name       string             `json:"name"`
	Host       string             `json:"host"`
	Status     string             `json:"status"`
	Datastores []DatastoreSummary `json:"datastores"`
}

// DatastoreSummary is a summarized datastore
type DatastoreSummary struct {
	Name         string  `json:"name"`
	UsagePercent float64 `json:"usage_percent"`
	FreeGB       float64 `json:"free_gb"`
}

// BackupTaskSummary is a summarized backup task
type BackupTaskSummary struct {
	VMID      int       `json:"vmid"`
	Node      string    `json:"node"`
	Status    string    `json:"status"`
	StartTime time.Time `json:"start_time"`
}

// StorageResponse is returned by pulse_list_storage
type StorageResponse struct {
	Pools        []StoragePoolSummary `json:"pools,omitempty"`
	CephClusters []CephClusterSummary `json:"ceph_clusters,omitempty"`
	Pagination   *PaginationInfo      `json:"pagination,omitempty"`
}

// StoragePoolSummary is a summarized storage pool
type StoragePoolSummary struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Type         string          `json:"type"`
	Status       string          `json:"status"`
	UsagePercent float64         `json:"usage_percent"`
	UsedGB       float64         `json:"used_gb"`
	TotalGB      float64         `json:"total_gb"`
	FreeGB       float64         `json:"free_gb"`
	Content      string          `json:"content"`
	Shared       bool            `json:"shared"`
	ZFS          *ZFSPoolSummary `json:"zfs,omitempty"`
}

// ZFSPoolSummary is a summarized ZFS pool
type ZFSPoolSummary struct {
	Name           string `json:"name"`
	State          string `json:"state"`
	ReadErrors     int64  `json:"read_errors"`
	WriteErrors    int64  `json:"write_errors"`
	ChecksumErrors int64  `json:"checksum_errors"`
	Scan           string `json:"scan,omitempty"`
}

// CephClusterSummary is a summarized Ceph cluster
type CephClusterSummary struct {
	Name          string  `json:"name"`
	Health        string  `json:"health"`
	HealthMessage string  `json:"health_message,omitempty"`
	UsagePercent  float64 `json:"usage_percent"`
	UsedTB        float64 `json:"used_tb"`
	TotalTB       float64 `json:"total_tb"`
	NumOSDs       int     `json:"num_osds"`
	NumOSDsUp     int     `json:"num_osds_up"`
	NumOSDsIn     int     `json:"num_osds_in"`
	NumMons       int     `json:"num_mons"`
	NumMgrs       int     `json:"num_mgrs"`
}

// DiskHealthResponse is returned by pulse_get_disk_health
type DiskHealthResponse struct {
	Hosts []HostDiskHealth `json:"hosts"`
}

// HostDiskHealth is disk health for a single host
type HostDiskHealth struct {
	Hostname string             `json:"hostname"`
	SMART    []SMARTDiskSummary `json:"smart,omitempty"`
	RAID     []RAIDArraySummary `json:"raid,omitempty"`
	Ceph     *CephStatusSummary `json:"ceph,omitempty"`
}

// SMARTDiskSummary is a summarized SMART disk
type SMARTDiskSummary struct {
	Device      string `json:"device"`
	Model       string `json:"model"`
	Health      string `json:"health"`
	Temperature int    `json:"temperature,omitempty"`
}

// RAIDArraySummary is a summarized RAID array
type RAIDArraySummary struct {
	Device         string  `json:"device"`
	Level          string  `json:"level"`
	State          string  `json:"state"`
	ActiveDevices  int     `json:"active_devices"`
	WorkingDevices int     `json:"working_devices"`
	FailedDevices  int     `json:"failed_devices"`
	SpareDevices   int     `json:"spare_devices"`
	RebuildPercent float64 `json:"rebuild_percent,omitempty"`
}

// CephStatusSummary is a summarized Ceph status from agent
type CephStatusSummary struct {
	Health       string  `json:"health"`
	NumOSDs      int     `json:"num_osds"`
	NumOSDsUp    int     `json:"num_osds_up"`
	NumOSDsIn    int     `json:"num_osds_in"`
	NumPGs       int     `json:"num_pgs"`
	UsagePercent float64 `json:"usage_percent"`
}

// AgentScopeResponse is returned by pulse_get_agent_scope
type AgentScopeResponse struct {
	AgentID         string                 `json:"agent_id"`
	AgentLabel      string                 `json:"agent_label"`
	ProfileID       string                 `json:"profile_id,omitempty"`
	ProfileName     string                 `json:"profile_name,omitempty"`
	ProfileVersion  int                    `json:"profile_version,omitempty"`
	Settings        map[string]interface{} `json:"settings,omitempty"`
	ObservedModules []string               `json:"observed_modules,omitempty"`
	CommandsEnabled *bool                  `json:"commands_enabled,omitempty"`
}

// CommandResponse is returned by control tools
type CommandResponse struct {
	Success  bool   `json:"success"`
	Output   string `json:"output,omitempty"`
	ExitCode int    `json:"exit_code,omitempty"`
	Error    string `json:"error,omitempty"`
}

// ControlActionResponse is returned by pulse_control_guest and pulse_control_docker
type ControlActionResponse struct {
	Success    bool   `json:"success"`
	Action     string `json:"action"`
	Target     string `json:"target"`
	TargetType string `json:"target_type"` // "vm", "lxc", "docker"
	Output     string `json:"output,omitempty"`
	Error      string `json:"error,omitempty"`
}
