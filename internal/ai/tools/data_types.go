package tools

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
// AgentInfo represents a connected execution agent
type AgentInfo struct {
	Hostname    string `json:"hostname"`
	Version     string `json:"version,omitempty"`
	Platform    string `json:"platform,omitempty"`
	ConnectedAt string `json:"connected_at,omitempty"`
}

type CapabilitiesResponse struct {
	ControlLevel    string       `json:"control_level"`
	Features        FeatureFlags `json:"features"`
	ProtectedGuests []string     `json:"protected_guests,omitempty"`
	ConnectedAgents int          `json:"connected_agents"`
	Agents          []AgentInfo  `json:"agents,omitempty"` // List of connected agents with hostnames
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

// ResourceSearchResponse is returned by pulse_search_resources
type ResourceSearchResponse struct {
	Query      string          `json:"query"`
	Matches    []ResourceMatch `json:"matches"`
	Total      int             `json:"total"`
	Pagination *PaginationInfo `json:"pagination,omitempty"`
}

// ResourceMatch is a compact match result for pulse_search_resources
type ResourceMatch struct {
	Type           string `json:"type"` // "node", "vm", "container", "docker", "docker_host"
	ID             string `json:"id,omitempty"`
	Name           string `json:"name"`
	Status         string `json:"status,omitempty"`
	Node           string `json:"node,omitempty"`           // Proxmox node this resource is on
	NodeHasAgent   bool   `json:"node_has_agent,omitempty"` // True if the Proxmox node has a connected agent
	Host           string `json:"host,omitempty"`           // Docker host for docker containers
	VMID           int    `json:"vmid,omitempty"`
	Image          string `json:"image,omitempty"`
	AgentConnected bool   `json:"agent_connected,omitempty"` // True if this specific resource has a connected agent
}

// NodeSummary is a summarized node for list responses
type NodeSummary struct {
	Name           string `json:"name"`
	Status         string `json:"status"`
	ID             string `json:"id,omitempty"`
	AgentConnected bool   `json:"agent_connected"` // True if an execution agent is connected for this node
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
	AgentConnected bool                     `json:"agent_connected"` // True if an execution agent is connected for this host
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

// ========== Topology Response Types (Hierarchical View) ==========

// TopologyResponse provides a fully hierarchical view of infrastructure
// This is the recommended tool for understanding infrastructure relationships
type TopologyResponse struct {
	Proxmox ProxmoxTopology `json:"proxmox"`
	Docker  DockerTopology  `json:"docker"`
	Summary TopologySummary `json:"summary"`
}

// ProxmoxTopology shows Proxmox nodes with their nested VMs and containers
type ProxmoxTopology struct {
	Nodes []ProxmoxNodeTopology `json:"nodes"`
}

// ProxmoxNodeTopology represents a Proxmox node with its guests
type ProxmoxNodeTopology struct {
	Name           string        `json:"name"`
	ID             string        `json:"id,omitempty"`
	Status         string        `json:"status"`
	AgentConnected bool          `json:"agent_connected"`
	CanExecute     bool          `json:"can_execute"` // True if commands can be executed on this node
	VMs            []TopologyVM  `json:"vms,omitempty"`
	Containers     []TopologyLXC `json:"containers,omitempty"`
	VMCount        int           `json:"vm_count"`
	ContainerCount int           `json:"container_count"`
}

// TopologyVM represents a VM in the topology
type TopologyVM struct {
	VMID   int      `json:"vmid"`
	Name   string   `json:"name"`
	Status string   `json:"status"`
	CPU    float64  `json:"cpu_percent,omitempty"`
	Memory float64  `json:"memory_percent,omitempty"`
	OS     string   `json:"os,omitempty"`
	Tags   []string `json:"tags,omitempty"`
}

// TopologyLXC represents an LXC container in the topology
type TopologyLXC struct {
	VMID      int      `json:"vmid"`
	Name      string   `json:"name"`
	Status    string   `json:"status"`
	CPU       float64  `json:"cpu_percent,omitempty"`
	Memory    float64  `json:"memory_percent,omitempty"`
	OS        string   `json:"os,omitempty"`
	Tags      []string `json:"tags,omitempty"`
	HasDocker bool     `json:"has_docker,omitempty"` // True if Docker is installed inside this container
}

// DockerTopology shows Docker hosts with their nested containers
type DockerTopology struct {
	Hosts []DockerHostTopology `json:"hosts"`
}

// DockerHostTopology represents a Docker host with its containers
type DockerHostTopology struct {
	Hostname       string                   `json:"hostname"`
	DisplayName    string                   `json:"display_name,omitempty"`
	AgentConnected bool                     `json:"agent_connected"`
	CanExecute     bool                     `json:"can_execute"` // True if commands can be executed on this host
	Containers     []DockerContainerSummary `json:"containers,omitempty"`
	ContainerCount int                      `json:"container_count"`
	RunningCount   int                      `json:"running_count"`
}

// TopologySummary provides aggregate counts and status
type TopologySummary struct {
	TotalNodes            int `json:"total_nodes"`
	TotalVMs              int `json:"total_vms"`
	TotalLXCContainers    int `json:"total_lxc_containers"`
	TotalDockerHosts      int `json:"total_docker_hosts"`
	TotalDockerContainers int `json:"total_docker_containers"`
	NodesWithAgents       int `json:"nodes_with_agents"`
	DockerHostsWithAgents int `json:"docker_hosts_with_agents"`
	RunningVMs            int `json:"running_vms"`
	RunningLXC            int `json:"running_lxc"`
	RunningDocker         int `json:"running_docker"`
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

// GuestConfigResponse is returned by pulse_get_guest_config.
type GuestConfigResponse struct {
	GuestType string             `json:"guest_type"`
	VMID      int                `json:"vmid"`
	Name      string             `json:"name,omitempty"`
	Node      string             `json:"node,omitempty"`
	Instance  string             `json:"instance,omitempty"`
	Hostname  string             `json:"hostname,omitempty"`
	OSType    string             `json:"os_type,omitempty"`
	Onboot    *bool              `json:"onboot,omitempty"`
	RootFS    string             `json:"rootfs,omitempty"`
	Mounts    []GuestMountConfig `json:"mounts,omitempty"`
	Disks     []GuestDiskConfig  `json:"disks,omitempty"`
	Raw       map[string]string  `json:"raw,omitempty"`
}

// GuestMountConfig summarizes a container mount.
type GuestMountConfig struct {
	Key        string `json:"key"`
	Source     string `json:"source"`
	Mountpoint string `json:"mountpoint,omitempty"`
}

// GuestDiskConfig summarizes a VM disk definition.
type GuestDiskConfig struct {
	Key   string `json:"key"`
	Value string `json:"value"`
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
	ResourceID    string                            `json:"resource_id,omitempty"`
	Period        string                            `json:"period"`
	Points        []MetricPoint                     `json:"points,omitempty"`
	Summary       map[string]ResourceMetricsSummary `json:"summary,omitempty"`
	Pagination    *PaginationInfo                   `json:"pagination,omitempty"`
	Downsampled   bool                              `json:"downsampled,omitempty"`
	OriginalCount int                               `json:"original_count,omitempty"`
}

// BaselinesResponse is returned by pulse_get_baselines
type BaselinesResponse struct {
	ResourceID string                                `json:"resource_id,omitempty"`
	Baselines  map[string]map[string]*MetricBaseline `json:"baselines"` // resourceID -> metric -> baseline
	Pagination *PaginationInfo                       `json:"pagination,omitempty"`
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
	Node         string          `json:"node,omitempty"`
	Instance     string          `json:"instance,omitempty"`
	Nodes        []string        `json:"nodes,omitempty"`
	Type         string          `json:"type"`
	Status       string          `json:"status"`
	Enabled      bool            `json:"enabled"`
	Active       bool            `json:"active"`
	Path         string          `json:"path,omitempty"`
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

// ========== Docker Updates Types ==========

// ContainerUpdateInfo represents a container with an available update
type ContainerUpdateInfo struct {
	HostID          string `json:"host_id"`
	HostName        string `json:"host_name"`
	ContainerID     string `json:"container_id"`
	ContainerName   string `json:"container_name"`
	Image           string `json:"image"`
	CurrentDigest   string `json:"current_digest,omitempty"`
	LatestDigest    string `json:"latest_digest,omitempty"`
	UpdateAvailable bool   `json:"update_available"`
	LastChecked     int64  `json:"last_checked,omitempty"`
	Error           string `json:"error,omitempty"`
}

// DockerCommandStatus represents the status of a queued Docker command
type DockerCommandStatus struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

// DockerUpdatesResponse is returned by pulse_list_docker_updates
type DockerUpdatesResponse struct {
	Updates []ContainerUpdateInfo `json:"updates"`
	Total   int                   `json:"total"`
	HostID  string                `json:"host_id,omitempty"`
}

// DockerCheckUpdatesResponse is returned by pulse_check_docker_updates
type DockerCheckUpdatesResponse struct {
	Success   bool                `json:"success"`
	HostID    string              `json:"host_id"`
	HostName  string              `json:"host_name"`
	CommandID string              `json:"command_id"`
	Message   string              `json:"message"`
	Command   DockerCommandStatus `json:"command"`
}

// DockerUpdateContainerResponse is returned by pulse_update_docker_container
type DockerUpdateContainerResponse struct {
	Success       bool                `json:"success"`
	HostID        string              `json:"host_id"`
	ContainerID   string              `json:"container_id"`
	ContainerName string              `json:"container_name"`
	CommandID     string              `json:"command_id"`
	Message       string              `json:"message"`
	Command       DockerCommandStatus `json:"command"`
}

// ========== Kubernetes Types ==========

// KubernetesClustersResponse is returned by pulse_get_kubernetes_clusters
type KubernetesClustersResponse struct {
	Clusters []KubernetesClusterSummary `json:"clusters"`
	Total    int                        `json:"total"`
}

// KubernetesClusterSummary summarizes a Kubernetes cluster
type KubernetesClusterSummary struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	DisplayName     string `json:"display_name,omitempty"`
	Server          string `json:"server,omitempty"`
	Version         string `json:"version,omitempty"`
	Status          string `json:"status"`
	NodeCount       int    `json:"node_count"`
	PodCount        int    `json:"pod_count"`
	DeploymentCount int    `json:"deployment_count"`
	ReadyNodes      int    `json:"ready_nodes"`
}

// KubernetesNodesResponse is returned by pulse_get_kubernetes_nodes
type KubernetesNodesResponse struct {
	Cluster string                  `json:"cluster"`
	Nodes   []KubernetesNodeSummary `json:"nodes"`
	Total   int                     `json:"total"`
}

// KubernetesNodeSummary summarizes a Kubernetes node
type KubernetesNodeSummary struct {
	UID                     string   `json:"uid"`
	Name                    string   `json:"name"`
	Ready                   bool     `json:"ready"`
	Unschedulable           bool     `json:"unschedulable,omitempty"`
	Roles                   []string `json:"roles,omitempty"`
	KubeletVersion          string   `json:"kubelet_version,omitempty"`
	ContainerRuntimeVersion string   `json:"container_runtime_version,omitempty"`
	OSImage                 string   `json:"os_image,omitempty"`
	Architecture            string   `json:"architecture,omitempty"`
	CapacityCPU             int64    `json:"capacity_cpu_cores,omitempty"`
	CapacityMemoryBytes     int64    `json:"capacity_memory_bytes,omitempty"`
	CapacityPods            int64    `json:"capacity_pods,omitempty"`
	AllocatableCPU          int64    `json:"allocatable_cpu_cores,omitempty"`
	AllocatableMemoryBytes  int64    `json:"allocatable_memory_bytes,omitempty"`
	AllocatablePods         int64    `json:"allocatable_pods,omitempty"`
}

// KubernetesPodsResponse is returned by pulse_get_kubernetes_pods
type KubernetesPodsResponse struct {
	Cluster  string                 `json:"cluster"`
	Pods     []KubernetesPodSummary `json:"pods"`
	Total    int                    `json:"total"`
	Filtered int                    `json:"filtered,omitempty"`
}

// KubernetesPodSummary summarizes a Kubernetes pod
type KubernetesPodSummary struct {
	UID        string                          `json:"uid"`
	Name       string                          `json:"name"`
	Namespace  string                          `json:"namespace"`
	NodeName   string                          `json:"node_name,omitempty"`
	Phase      string                          `json:"phase,omitempty"`
	Reason     string                          `json:"reason,omitempty"`
	Restarts   int                             `json:"restarts,omitempty"`
	QoSClass   string                          `json:"qos_class,omitempty"`
	OwnerKind  string                          `json:"owner_kind,omitempty"`
	OwnerName  string                          `json:"owner_name,omitempty"`
	Containers []KubernetesPodContainerSummary `json:"containers,omitempty"`
}

// KubernetesPodContainerSummary summarizes a container in a pod
type KubernetesPodContainerSummary struct {
	Name         string `json:"name"`
	Ready        bool   `json:"ready"`
	State        string `json:"state,omitempty"`
	RestartCount int32  `json:"restart_count,omitempty"`
	Reason       string `json:"reason,omitempty"`
}

// KubernetesDeploymentsResponse is returned by pulse_get_kubernetes_deployments
type KubernetesDeploymentsResponse struct {
	Cluster     string                        `json:"cluster"`
	Deployments []KubernetesDeploymentSummary `json:"deployments"`
	Total       int                           `json:"total"`
	Filtered    int                           `json:"filtered,omitempty"`
}

// KubernetesDeploymentSummary summarizes a Kubernetes deployment
type KubernetesDeploymentSummary struct {
	UID               string `json:"uid"`
	Name              string `json:"name"`
	Namespace         string `json:"namespace"`
	DesiredReplicas   int32  `json:"desired_replicas"`
	ReadyReplicas     int32  `json:"ready_replicas"`
	AvailableReplicas int32  `json:"available_replicas"`
	UpdatedReplicas   int32  `json:"updated_replicas"`
}

// ========== PMG (Mail Gateway) Types ==========

// PMGStatusResponse is returned by pulse_get_pmg_status
type PMGStatusResponse struct {
	Instances []PMGInstanceSummary `json:"instances"`
	Total     int                  `json:"total"`
}

// PMGInstanceSummary summarizes a PMG instance
type PMGInstanceSummary struct {
	ID      string           `json:"id"`
	Name    string           `json:"name"`
	Host    string           `json:"host"`
	Status  string           `json:"status"`
	Version string           `json:"version,omitempty"`
	Nodes   []PMGNodeSummary `json:"nodes,omitempty"`
}

// PMGNodeSummary summarizes a PMG cluster node
type PMGNodeSummary struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Role    string `json:"role,omitempty"`
	Uptime  int64  `json:"uptime_seconds,omitempty"`
	LoadAvg string `json:"load_avg,omitempty"`
}

// MailStatsResponse is returned by pulse_get_mail_stats
type MailStatsResponse struct {
	Instance string              `json:"instance,omitempty"`
	Stats    PMGMailStatsSummary `json:"stats"`
}

// PMGMailStatsSummary summarizes mail statistics
type PMGMailStatsSummary struct {
	Timeframe            string  `json:"timeframe,omitempty"`
	TotalIn              float64 `json:"total_in"`
	TotalOut             float64 `json:"total_out"`
	SpamIn               float64 `json:"spam_in"`
	SpamOut              float64 `json:"spam_out"`
	VirusIn              float64 `json:"virus_in"`
	VirusOut             float64 `json:"virus_out"`
	BouncesIn            float64 `json:"bounces_in"`
	BouncesOut           float64 `json:"bounces_out"`
	BytesIn              float64 `json:"bytes_in,omitempty"`
	BytesOut             float64 `json:"bytes_out,omitempty"`
	GreylistCount        float64 `json:"greylist_count,omitempty"`
	RBLRejects           float64 `json:"rbl_rejects,omitempty"`
	AverageProcessTimeMs float64 `json:"avg_process_time_ms,omitempty"`
}

// MailQueuesResponse is returned by pulse_get_mail_queues
type MailQueuesResponse struct {
	Instance string            `json:"instance,omitempty"`
	Queues   []PMGQueueSummary `json:"queues"`
}

// PMGQueueSummary summarizes mail queue status for a node
type PMGQueueSummary struct {
	Node             string `json:"node"`
	Active           int    `json:"active"`
	Deferred         int    `json:"deferred"`
	Hold             int    `json:"hold"`
	Incoming         int    `json:"incoming"`
	Total            int    `json:"total"`
	OldestAgeSeconds int64  `json:"oldest_age_seconds"`
}

// SpamStatsResponse is returned by pulse_get_spam_stats
type SpamStatsResponse struct {
	Instance     string                 `json:"instance,omitempty"`
	Quarantine   PMGQuarantineSummary   `json:"quarantine"`
	Distribution []PMGSpamBucketSummary `json:"spam_distribution,omitempty"`
}

// PMGQuarantineSummary summarizes quarantine counts
type PMGQuarantineSummary struct {
	Spam        int `json:"spam"`
	Virus       int `json:"virus"`
	Attachment  int `json:"attachment"`
	Blacklisted int `json:"blacklisted"`
	Total       int `json:"total"`
}

// PMGSpamBucketSummary summarizes spam score distribution
type PMGSpamBucketSummary struct {
	Score string  `json:"score"`
	Count float64 `json:"count"`
}

// ========== Snapshots & Backup Types ==========

// SnapshotsResponse is returned by pulse_list_snapshots
type SnapshotsResponse struct {
	Snapshots []SnapshotSummary `json:"snapshots"`
	Total     int               `json:"total"`
	Filtered  int               `json:"filtered,omitempty"`
}

// SnapshotSummary summarizes a VM/container snapshot
type SnapshotSummary struct {
	ID           string    `json:"id"`
	VMID         int       `json:"vmid"`
	VMName       string    `json:"vm_name,omitempty"`
	Type         string    `json:"type"` // "vm" or "lxc"
	Node         string    `json:"node"`
	Instance     string    `json:"instance,omitempty"`
	SnapshotName string    `json:"snapshot_name"`
	Description  string    `json:"description,omitempty"`
	Time         time.Time `json:"time"`
	VMState      bool      `json:"vm_state"`
	SizeBytes    int64     `json:"size_bytes,omitempty"`
}

// PBSJobsResponse is returned by pulse_list_pbs_jobs
type PBSJobsResponse struct {
	Instance string          `json:"instance,omitempty"`
	Jobs     []PBSJobSummary `json:"jobs"`
	Total    int             `json:"total"`
}

// PBSJobSummary summarizes a PBS job (backup, sync, verify, prune, garbage)
type PBSJobSummary struct {
	ID      string    `json:"id"`
	Type    string    `json:"type"` // "backup", "sync", "verify", "prune", "garbage"
	Store   string    `json:"store"`
	Status  string    `json:"status"`
	LastRun time.Time `json:"last_run,omitempty"`
	NextRun time.Time `json:"next_run,omitempty"`
	Error   string    `json:"error,omitempty"`
	// Additional fields for specific job types
	VMID         string `json:"vmid,omitempty"`          // For backup jobs
	Remote       string `json:"remote,omitempty"`        // For sync jobs
	RemovedBytes int64  `json:"removed_bytes,omitempty"` // For garbage jobs
}

// BackupTasksListResponse is returned by pulse_list_backup_tasks
type BackupTasksListResponse struct {
	Tasks    []BackupTaskDetail `json:"tasks"`
	Total    int                `json:"total"`
	Filtered int                `json:"filtered,omitempty"`
}

// BackupTaskDetail provides detailed backup task information
type BackupTaskDetail struct {
	ID        string    `json:"id"`
	VMID      int       `json:"vmid"`
	VMName    string    `json:"vm_name,omitempty"`
	Node      string    `json:"node"`
	Instance  string    `json:"instance,omitempty"`
	Type      string    `json:"type"` // "vm" or "lxc"
	Status    string    `json:"status"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time,omitempty"`
	SizeBytes int64     `json:"size_bytes,omitempty"`
	Error     string    `json:"error,omitempty"`
}

// ========== Host Diagnostics Types ==========

// NetworkStatsResponse is returned by pulse_get_network_stats
type NetworkStatsResponse struct {
	Hosts []HostNetworkStatsSummary `json:"hosts"`
	Total int                       `json:"total"`
}

// HostNetworkStatsSummary summarizes network stats for a host
type HostNetworkStatsSummary struct {
	Hostname   string                    `json:"hostname"`
	Interfaces []NetworkInterfaceSummary `json:"interfaces"`
}

// NetworkInterfaceSummary summarizes a network interface
type NetworkInterfaceSummary struct {
	Name      string   `json:"name"`
	MAC       string   `json:"mac,omitempty"`
	Addresses []string `json:"addresses,omitempty"`
	RXBytes   uint64   `json:"rx_bytes"`
	TXBytes   uint64   `json:"tx_bytes"`
	SpeedMbps *int64   `json:"speed_mbps,omitempty"`
}

// DiskIOStatsResponse is returned by pulse_get_diskio_stats
type DiskIOStatsResponse struct {
	Hosts []HostDiskIOStatsSummary `json:"hosts"`
	Total int                      `json:"total"`
}

// HostDiskIOStatsSummary summarizes disk I/O for a host
type HostDiskIOStatsSummary struct {
	Hostname string                `json:"hostname"`
	Devices  []DiskIODeviceSummary `json:"devices"`
}

// DiskIODeviceSummary summarizes disk I/O for a device
type DiskIODeviceSummary struct {
	Device     string `json:"device"`
	ReadBytes  uint64 `json:"read_bytes"`
	WriteBytes uint64 `json:"write_bytes"`
	ReadOps    uint64 `json:"read_ops"`
	WriteOps   uint64 `json:"write_ops"`
	IOTimeMs   uint64 `json:"io_time_ms,omitempty"`
}

// ClusterStatusResponse is returned by pulse_get_cluster_status
type ClusterStatusResponse struct {
	Clusters []PVEClusterStatus `json:"clusters"`
}

// PVEClusterStatus summarizes Proxmox cluster status
type PVEClusterStatus struct {
	Instance    string                 `json:"instance"`
	ClusterName string                 `json:"cluster_name,omitempty"`
	QuorumOK    bool                   `json:"quorum_ok"`
	TotalNodes  int                    `json:"total_nodes"`
	OnlineNodes int                    `json:"online_nodes"`
	Nodes       []PVEClusterNodeStatus `json:"nodes"`
}

// PVEClusterNodeStatus summarizes a node's cluster membership
type PVEClusterNodeStatus struct {
	Name            string `json:"name"`
	Status          string `json:"status"`
	IsClusterMember bool   `json:"is_cluster_member"`
	ClusterName     string `json:"cluster_name,omitempty"`
}

// ========== Docker Swarm Types ==========

// SwarmStatusResponse is returned by pulse_get_swarm_status
type SwarmStatusResponse struct {
	Host   string             `json:"host"`
	Status DockerSwarmSummary `json:"status"`
}

// DockerSwarmSummary summarizes Docker Swarm status
type DockerSwarmSummary struct {
	NodeID           string `json:"node_id,omitempty"`
	NodeRole         string `json:"node_role,omitempty"`
	LocalState       string `json:"local_state,omitempty"`
	ControlAvailable bool   `json:"control_available"`
	ClusterID        string `json:"cluster_id,omitempty"`
	ClusterName      string `json:"cluster_name,omitempty"`
	Error            string `json:"error,omitempty"`
}

// DockerServicesResponse is returned by pulse_list_docker_services
type DockerServicesResponse struct {
	Host     string                 `json:"host"`
	Services []DockerServiceSummary `json:"services"`
	Total    int                    `json:"total"`
	Filtered int                    `json:"filtered,omitempty"`
}

// DockerServiceSummary summarizes a Docker Swarm service
type DockerServiceSummary struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Stack        string `json:"stack,omitempty"`
	Image        string `json:"image,omitempty"`
	Mode         string `json:"mode,omitempty"`
	DesiredTasks int    `json:"desired_tasks"`
	RunningTasks int    `json:"running_tasks"`
	UpdateStatus string `json:"update_status,omitempty"`
}

// DockerTasksResponse is returned by pulse_list_docker_tasks
type DockerTasksResponse struct {
	Host    string              `json:"host"`
	Service string              `json:"service,omitempty"`
	Tasks   []DockerTaskSummary `json:"tasks"`
	Total   int                 `json:"total"`
}

// DockerTaskSummary summarizes a Docker Swarm task
type DockerTaskSummary struct {
	ID           string     `json:"id"`
	ServiceName  string     `json:"service_name,omitempty"`
	NodeName     string     `json:"node_name,omitempty"`
	DesiredState string     `json:"desired_state,omitempty"`
	CurrentState string     `json:"current_state,omitempty"`
	Error        string     `json:"error,omitempty"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
}

// ========== Recent Tasks Types ==========

// RecentTasksResponse is returned by pulse_list_recent_tasks
type RecentTasksResponse struct {
	Tasks    []ProxmoxTaskSummary `json:"tasks"`
	Total    int                  `json:"total"`
	Filtered int                  `json:"filtered,omitempty"`
}

// ProxmoxTaskSummary summarizes a Proxmox task
type ProxmoxTaskSummary struct {
	ID          string    `json:"id"`
	Node        string    `json:"node"`
	Instance    string    `json:"instance,omitempty"`
	Type        string    `json:"type"`
	Status      string    `json:"status"`
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time,omitempty"`
	VMID        int       `json:"vmid,omitempty"`
	Description string    `json:"description,omitempty"`
}

// ========== Physical Disk Types ==========

// PhysicalDisksResponse is returned by pulse_list_physical_disks
type PhysicalDisksResponse struct {
	Disks    []PhysicalDiskSummary `json:"disks"`
	Total    int                   `json:"total"`
	Filtered int                   `json:"filtered,omitempty"`
}

// PhysicalDiskSummary summarizes a physical disk with SMART health info
type PhysicalDiskSummary struct {
	ID          string    `json:"id"`
	Node        string    `json:"node"`
	Instance    string    `json:"instance"`
	DevPath     string    `json:"dev_path"`
	Model       string    `json:"model,omitempty"`
	Serial      string    `json:"serial,omitempty"`
	WWN         string    `json:"wwn,omitempty"`
	Type        string    `json:"type"` // nvme, sata, sas
	SizeBytes   int64     `json:"size_bytes"`
	Health      string    `json:"health"`                // PASSED, FAILED, UNKNOWN
	Wearout     *int      `json:"wearout,omitempty"`     // SSD wear percentage (0-100), nil when unavailable
	Temperature *int      `json:"temperature,omitempty"` // Celsius, nil when unavailable
	RPM         *int      `json:"rpm,omitempty"`         // 0 for SSDs, nil when unavailable
	Used        string    `json:"used,omitempty"`
	LastChecked time.Time `json:"last_checked,omitempty"`
}

// ========== Host RAID Types ==========

// HostRAIDStatusResponse is returned by pulse_get_host_raid_status
type HostRAIDStatusResponse struct {
	Hosts []HostRAIDSummary `json:"hosts"`
	Total int               `json:"total"`
}

// HostRAIDSummary summarizes RAID arrays for a host
type HostRAIDSummary struct {
	Hostname string                 `json:"hostname"`
	HostID   string                 `json:"host_id"`
	Arrays   []HostRAIDArraySummary `json:"arrays"`
}

// HostRAIDArraySummary summarizes a RAID array
type HostRAIDArraySummary struct {
	Device         string                  `json:"device"`
	Name           string                  `json:"name,omitempty"`
	Level          string                  `json:"level"` // raid0, raid1, raid5, etc.
	State          string                  `json:"state"` // clean, degraded, rebuilding
	TotalDevices   int                     `json:"total_devices"`
	ActiveDevices  int                     `json:"active_devices"`
	WorkingDevices int                     `json:"working_devices"`
	FailedDevices  int                     `json:"failed_devices"`
	SpareDevices   int                     `json:"spare_devices"`
	UUID           string                  `json:"uuid,omitempty"`
	RebuildPercent float64                 `json:"rebuild_percent,omitempty"`
	RebuildSpeed   string                  `json:"rebuild_speed,omitempty"`
	Devices        []HostRAIDDeviceSummary `json:"devices,omitempty"`
}

// HostRAIDDeviceSummary summarizes a device in a RAID array
type HostRAIDDeviceSummary struct {
	Device string `json:"device"`
	State  string `json:"state"`
	Slot   int    `json:"slot"`
}

// ========== Host Ceph Details Types ==========

// HostCephDetailsResponse is returned by pulse_get_host_ceph_details
type HostCephDetailsResponse struct {
	Hosts []HostCephSummary `json:"hosts"`
	Total int               `json:"total"`
}

// HostCephSummary summarizes host-collected Ceph cluster details
type HostCephSummary struct {
	Hostname    string                `json:"hostname"`
	HostID      string                `json:"host_id"`
	FSID        string                `json:"fsid"`
	Health      HostCephHealthSummary `json:"health"`
	MonMap      *HostCephMonSummary   `json:"mon_map,omitempty"`
	MgrMap      *HostCephMgrSummary   `json:"mgr_map,omitempty"`
	OSDMap      HostCephOSDSummary    `json:"osd_map"`
	PGMap       HostCephPGSummary     `json:"pg_map"`
	Pools       []HostCephPoolSummary `json:"pools,omitempty"`
	CollectedAt time.Time             `json:"collected_at"`
}

// HostCephHealthSummary summarizes Ceph health
type HostCephHealthSummary struct {
	Status   string                  `json:"status"` // HEALTH_OK, HEALTH_WARN, HEALTH_ERR
	Messages []HostCephHealthMessage `json:"messages,omitempty"`
}

// HostCephHealthMessage represents a health check message
type HostCephHealthMessage struct {
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

// HostCephMonSummary summarizes Ceph monitors
type HostCephMonSummary struct {
	NumMons  int                      `json:"num_mons"`
	Monitors []HostCephMonitorSummary `json:"monitors,omitempty"`
}

// HostCephMonitorSummary summarizes a single monitor
type HostCephMonitorSummary struct {
	Name   string `json:"name"`
	Rank   int    `json:"rank"`
	Addr   string `json:"addr,omitempty"`
	Status string `json:"status,omitempty"`
}

// HostCephMgrSummary summarizes Ceph managers
type HostCephMgrSummary struct {
	Available bool   `json:"available"`
	NumMgrs   int    `json:"num_mgrs"`
	ActiveMgr string `json:"active_mgr,omitempty"`
	Standbys  int    `json:"standbys"`
}

// HostCephOSDSummary summarizes OSD status
type HostCephOSDSummary struct {
	NumOSDs int `json:"num_osds"`
	NumUp   int `json:"num_up"`
	NumIn   int `json:"num_in"`
	NumDown int `json:"num_down,omitempty"`
	NumOut  int `json:"num_out,omitempty"`
}

// HostCephPGSummary summarizes placement group stats
type HostCephPGSummary struct {
	NumPGs           int     `json:"num_pgs"`
	BytesTotal       uint64  `json:"bytes_total"`
	BytesUsed        uint64  `json:"bytes_used"`
	BytesAvailable   uint64  `json:"bytes_available"`
	UsagePercent     float64 `json:"usage_percent"`
	DegradedRatio    float64 `json:"degraded_ratio,omitempty"`
	MisplacedRatio   float64 `json:"misplaced_ratio,omitempty"`
	ReadBytesPerSec  uint64  `json:"read_bytes_per_sec,omitempty"`
	WriteBytesPerSec uint64  `json:"write_bytes_per_sec,omitempty"`
	ReadOpsPerSec    uint64  `json:"read_ops_per_sec,omitempty"`
	WriteOpsPerSec   uint64  `json:"write_ops_per_sec,omitempty"`
}

// HostCephPoolSummary summarizes a Ceph pool
type HostCephPoolSummary struct {
	ID             int     `json:"id"`
	Name           string  `json:"name"`
	BytesUsed      uint64  `json:"bytes_used"`
	BytesAvailable uint64  `json:"bytes_available,omitempty"`
	Objects        uint64  `json:"objects"`
	PercentUsed    float64 `json:"percent_used"`
}

// ========== Resource Disks Types ==========

// ResourceDisksResponse is returned by pulse_get_resource_disks
type ResourceDisksResponse struct {
	Resources []ResourceDisksSummary `json:"resources"`
	Total     int                    `json:"total"`
}

// ResourceDisksSummary summarizes disk info for a VM or container
type ResourceDisksSummary struct {
	ID       string             `json:"id"`
	VMID     int                `json:"vmid"`
	Name     string             `json:"name"`
	Type     string             `json:"type"` // "vm" or "lxc"
	Node     string             `json:"node"`
	Instance string             `json:"instance,omitempty"`
	Disks    []ResourceDiskInfo `json:"disks"`
}

// ResourceDiskInfo represents a disk attached to a VM or container
type ResourceDiskInfo struct {
	Device     string  `json:"device,omitempty"`
	Mountpoint string  `json:"mountpoint,omitempty"`
	Type       string  `json:"type,omitempty"`
	TotalBytes int64   `json:"total_bytes"`
	UsedBytes  int64   `json:"used_bytes"`
	FreeBytes  int64   `json:"free_bytes"`
	Usage      float64 `json:"usage_percent"`
}

// ========== Connection Health Types ==========

// ConnectionHealthResponse is returned by pulse_get_connection_health
type ConnectionHealthResponse struct {
	Connections  []ConnectionStatus `json:"connections"`
	Total        int                `json:"total"`
	Connected    int                `json:"connected"`
	Disconnected int                `json:"disconnected"`
}

// ConnectionStatus represents the health of a connection to an instance
type ConnectionStatus struct {
	InstanceID string `json:"instance_id"`
	Connected  bool   `json:"connected"`
}

// ========== Resolved Alerts Types ==========

// ResolvedAlertsResponse is returned by pulse_list_resolved_alerts
type ResolvedAlertsResponse struct {
	Alerts []ResolvedAlertSummary `json:"alerts"`
	Total  int                    `json:"total"`
}

// ResolvedAlertSummary summarizes a recently resolved alert
type ResolvedAlertSummary struct {
	ID           string    `json:"id"`
	Type         string    `json:"type"`
	Level        string    `json:"level"`
	ResourceID   string    `json:"resource_id"`
	ResourceName string    `json:"resource_name"`
	Node         string    `json:"node,omitempty"`
	Instance     string    `json:"instance,omitempty"`
	Message      string    `json:"message"`
	Value        float64   `json:"value,omitempty"`
	Threshold    float64   `json:"threshold,omitempty"`
	StartTime    time.Time `json:"start_time"`
	ResolvedTime time.Time `json:"resolved_time"`
}
