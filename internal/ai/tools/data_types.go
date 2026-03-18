package tools

import (
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// GovernedResourceMetadata carries canonical policy metadata for AI-facing
// tool payloads so downstream consumers can honor redaction/routing rules
// without rebuilding platform-specific inference.
type GovernedResourceMetadata struct {
	Policy        *unifiedresources.ResourcePolicy `json:"policy,omitempty"`
	AISafeSummary string                           `json:"ai_safe_summary,omitempty"`
}

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
	VMID       int
	Name       string
	Node       string
	Type       string // semantic type: "vm" or "system-container"
	Technology string // implementation: "qemu", "lxc", etc.
	Status     string
	Instance   string
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
	ProtectedGuests []string     `json:"protected_guests"`
	ConnectedAgents int          `json:"connected_agents"`
	Agents          []AgentInfo  `json:"agents"` // List of connected agents with hostnames
	Version         string       `json:"version"`
}

func EmptyCapabilitiesResponse() CapabilitiesResponse {
	return CapabilitiesResponse{}.NormalizeCollections()
}

func (r CapabilitiesResponse) NormalizeCollections() CapabilitiesResponse {
	if r.ProtectedGuests == nil {
		r.ProtectedGuests = []string{}
	}
	if r.Agents == nil {
		r.Agents = []AgentInfo{}
	}
	return r
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
	Nodes          []NodeSummary          `json:"nodes"`
	VMs            []VMSummary            `json:"vms"`
	Containers     []ContainerSummary     `json:"containers"`
	DockerHosts    []DockerHostSummary    `json:"docker_hosts"`
	K8sClusters    []K8sClusterSummary    `json:"k8s_clusters"`
	K8sNodes       []K8sNodeSummary       `json:"k8s_nodes"`
	K8sPods        []K8sPodSummary        `json:"k8s_pods"`
	K8sDeployments []K8sDeploymentSummary `json:"k8s_deployments"`
	Total          TotalCounts            `json:"total"`
	Pagination     *PaginationInfo        `json:"pagination,omitempty"`
}

func EmptyInfrastructureResponse() InfrastructureResponse {
	return InfrastructureResponse{}.NormalizeCollections()
}

func (r InfrastructureResponse) NormalizeCollections() InfrastructureResponse {
	if r.Nodes == nil {
		r.Nodes = []NodeSummary{}
	}
	if r.VMs == nil {
		r.VMs = []VMSummary{}
	}
	if r.Containers == nil {
		r.Containers = []ContainerSummary{}
	}
	if r.DockerHosts == nil {
		r.DockerHosts = []DockerHostSummary{}
	}
	if r.K8sClusters == nil {
		r.K8sClusters = []K8sClusterSummary{}
	}
	if r.K8sNodes == nil {
		r.K8sNodes = []K8sNodeSummary{}
	}
	if r.K8sPods == nil {
		r.K8sPods = []K8sPodSummary{}
	}
	if r.K8sDeployments == nil {
		r.K8sDeployments = []K8sDeploymentSummary{}
	}
	return r
}

// ResourceSearchResponse is returned by pulse_search_resources
type ResourceSearchResponse struct {
	Query      string          `json:"query"`
	Matches    []ResourceMatch `json:"matches"`
	Total      int             `json:"total"`
	Pagination *PaginationInfo `json:"pagination,omitempty"`
}

func EmptyResourceSearchResponse() ResourceSearchResponse {
	return ResourceSearchResponse{}.NormalizeCollections()
}

func (r ResourceSearchResponse) NormalizeCollections() ResourceSearchResponse {
	if r.Matches == nil {
		r.Matches = []ResourceMatch{}
	}
	return r
}

// ResourceMatch is a compact match result for pulse_search_resources
type ResourceMatch struct {
	GovernedResourceMetadata
	Type           string `json:"type"` // "node", "vm", "system-container", "app-container", "docker-host"
	ID             string `json:"id,omitempty"`
	Name           string `json:"name"`
	Status         string `json:"status,omitempty"`
	Node           string `json:"node,omitempty"`           // Hypervisor node this resource is on
	NodeHasAgent   bool   `json:"node_has_agent,omitempty"` // True if the node has a connected agent
	Host           string `json:"host,omitempty"`           // Docker host for docker containers
	VMID           int    `json:"vmid,omitempty"`
	Image          string `json:"image,omitempty"`
	AgentConnected bool   `json:"agent_connected,omitempty"` // True if this specific resource has a connected agent
}

// NodeSummary is a summarized node for list responses
type NodeSummary struct {
	GovernedResourceMetadata
	Name           string `json:"name"`
	Status         string `json:"status"`
	ID             string `json:"id,omitempty"`
	AgentConnected bool   `json:"agent_connected"` // True if an execution agent is connected for this node
}

// VMSummary is a summarized VM for list responses
type VMSummary struct {
	GovernedResourceMetadata
	VMID   int     `json:"vmid"`
	Name   string  `json:"name"`
	Status string  `json:"status"`
	Node   string  `json:"node"`
	CPU    float64 `json:"cpu_percent,omitempty"`
	Memory float64 `json:"memory_percent,omitempty"`
}

// ContainerSummary is a summarized LXC container for list responses
type ContainerSummary struct {
	GovernedResourceMetadata
	VMID   int     `json:"vmid"`
	Name   string  `json:"name"`
	Status string  `json:"status"`
	Node   string  `json:"node"`
	CPU    float64 `json:"cpu_percent,omitempty"`
	Memory float64 `json:"memory_percent,omitempty"`
}

// DockerHostSummary is a summarized Docker host for list responses
type DockerHostSummary struct {
	GovernedResourceMetadata
	ID             string                   `json:"id"`
	Hostname       string                   `json:"hostname"`
	DisplayName    string                   `json:"display_name,omitempty"`
	ContainerCount int                      `json:"container_count"`
	AgentConnected bool                     `json:"agent_connected"` // True if an execution agent is connected for this host
	Containers     []DockerContainerSummary `json:"containers"`
}

func (s DockerHostSummary) NormalizeCollections() DockerHostSummary {
	if s.Containers == nil {
		s.Containers = []DockerContainerSummary{}
	}
	return s
}

// DockerContainerSummary is a summarized Docker container
type DockerContainerSummary struct {
	GovernedResourceMetadata
	ID     string `json:"id"`
	Name   string `json:"name"`
	State  string `json:"state"`
	Image  string `json:"image"`
	Health string `json:"health,omitempty"`
}

// K8sClusterSummary is a summarized Kubernetes cluster for list responses
type K8sClusterSummary struct {
	GovernedResourceMetadata
	ID              string `json:"id,omitempty"`
	Name            string `json:"name"`
	Status          string `json:"status"`
	NodeCount       int    `json:"node_count"`
	DeploymentCount int    `json:"deployment_count"`
	PodCount        int    `json:"pod_count"`
}

// K8sNodeSummary is a summarized Kubernetes node for list responses
type K8sNodeSummary struct {
	GovernedResourceMetadata
	Name    string   `json:"name"`
	Cluster string   `json:"cluster"`
	Status  string   `json:"status"`
	Ready   bool     `json:"ready"`
	Roles   []string `json:"roles"`
}

func (s K8sNodeSummary) NormalizeCollections() K8sNodeSummary {
	if s.Roles == nil {
		s.Roles = []string{}
	}
	return s
}

// K8sPodSummary is a summarized Kubernetes pod for list responses
type K8sPodSummary struct {
	GovernedResourceMetadata
	Name      string `json:"name"`
	Cluster   string `json:"cluster"`
	Namespace string `json:"namespace"`
	Status    string `json:"status"`
	Restarts  int    `json:"restarts,omitempty"`
	OwnerKind string `json:"owner_kind,omitempty"`
	OwnerName string `json:"owner_name,omitempty"`
}

// K8sDeploymentSummary is a summarized Kubernetes deployment for list responses
type K8sDeploymentSummary struct {
	GovernedResourceMetadata
	Name            string `json:"name"`
	Cluster         string `json:"cluster"`
	Namespace       string `json:"namespace"`
	Status          string `json:"status"`
	DesiredReplicas int32  `json:"desired_replicas,omitempty"`
	ReadyReplicas   int32  `json:"ready_replicas,omitempty"`
}

// TotalCounts for infrastructure response
type TotalCounts struct {
	Nodes          int `json:"nodes"`
	VMs            int `json:"vms"`
	Containers     int `json:"containers"`
	DockerHosts    int `json:"docker_hosts"`
	K8sClusters    int `json:"k8s_clusters"`
	K8sNodes       int `json:"k8s_nodes"`
	K8sPods        int `json:"k8s_pods"`
	K8sDeployments int `json:"k8s_deployments"`
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
	Proxmox    ProxmoxTopology    `json:"proxmox"`
	Docker     DockerTopology     `json:"docker"`
	Kubernetes KubernetesTopology `json:"kubernetes"`
	Summary    TopologySummary    `json:"summary"`
}

func EmptyTopologyResponse() TopologyResponse {
	return TopologyResponse{}.NormalizeCollections()
}

func (r TopologyResponse) NormalizeCollections() TopologyResponse {
	r.Proxmox = r.Proxmox.NormalizeCollections()
	r.Docker = r.Docker.NormalizeCollections()
	r.Kubernetes = r.Kubernetes.NormalizeCollections()
	return r
}

// ProxmoxTopology shows Proxmox nodes with their nested VMs and containers
type ProxmoxTopology struct {
	Nodes []ProxmoxNodeTopology `json:"nodes"`
}

func (t ProxmoxTopology) NormalizeCollections() ProxmoxTopology {
	if t.Nodes == nil {
		t.Nodes = []ProxmoxNodeTopology{}
	}
	for i := range t.Nodes {
		t.Nodes[i] = t.Nodes[i].NormalizeCollections()
	}
	return t
}

// ProxmoxNodeTopology represents a Proxmox node with its guests
type ProxmoxNodeTopology struct {
	GovernedResourceMetadata
	Name           string              `json:"name"`
	ID             string              `json:"id,omitempty"`
	Status         string              `json:"status"`
	AgentConnected bool                `json:"agent_connected"`
	CanExecute     bool                `json:"can_execute"` // True if commands can be executed on this node
	VMs            []TopologyVM        `json:"vms"`
	Containers     []TopologyContainer `json:"containers"`
	VMCount        int                 `json:"vm_count"`
	ContainerCount int                 `json:"container_count"`
}

func (t ProxmoxNodeTopology) NormalizeCollections() ProxmoxNodeTopology {
	if t.VMs == nil {
		t.VMs = []TopologyVM{}
	}
	for i := range t.VMs {
		t.VMs[i] = t.VMs[i].NormalizeCollections()
	}
	if t.Containers == nil {
		t.Containers = []TopologyContainer{}
	}
	for i := range t.Containers {
		t.Containers[i] = t.Containers[i].NormalizeCollections()
	}
	return t
}

// TopologyVM represents a VM in the topology
type TopologyVM struct {
	GovernedResourceMetadata
	VMID   int      `json:"vmid"`
	Name   string   `json:"name"`
	Status string   `json:"status"`
	CPU    float64  `json:"cpu_percent,omitempty"`
	Memory float64  `json:"memory_percent,omitempty"`
	OS     string   `json:"os,omitempty"`
	Tags   []string `json:"tags"`
}

func (t TopologyVM) NormalizeCollections() TopologyVM {
	if t.Tags == nil {
		t.Tags = []string{}
	}
	return t
}

// TopologyContainer represents a system container in the topology
type TopologyContainer struct {
	GovernedResourceMetadata
	VMID      int      `json:"vmid"`
	Name      string   `json:"name"`
	Status    string   `json:"status"`
	CPU       float64  `json:"cpu_percent,omitempty"`
	Memory    float64  `json:"memory_percent,omitempty"`
	OS        string   `json:"os,omitempty"`
	Tags      []string `json:"tags"`
	HasDocker bool     `json:"has_docker,omitempty"` // True if Docker is installed inside this container
}

func (t TopologyContainer) NormalizeCollections() TopologyContainer {
	if t.Tags == nil {
		t.Tags = []string{}
	}
	return t
}

// DockerTopology shows Docker hosts with their nested containers
type DockerTopology struct {
	Hosts []DockerHostTopology `json:"hosts"`
}

func (t DockerTopology) NormalizeCollections() DockerTopology {
	if t.Hosts == nil {
		t.Hosts = []DockerHostTopology{}
	}
	for i := range t.Hosts {
		t.Hosts[i] = t.Hosts[i].NormalizeCollections()
	}
	return t
}

// DockerHostTopology represents a Docker host with its containers
type DockerHostTopology struct {
	GovernedResourceMetadata
	Hostname       string                   `json:"hostname"`
	DisplayName    string                   `json:"display_name,omitempty"`
	AgentConnected bool                     `json:"agent_connected"`
	CanExecute     bool                     `json:"can_execute"` // True if commands can be executed on this host
	Containers     []DockerContainerSummary `json:"containers"`
	ContainerCount int                      `json:"container_count"`
	RunningCount   int                      `json:"running_count"`
}

func (t DockerHostTopology) NormalizeCollections() DockerHostTopology {
	if t.Containers == nil {
		t.Containers = []DockerContainerSummary{}
	}
	return t
}

// KubernetesTopology shows Kubernetes clusters with their nested resources.
type KubernetesTopology struct {
	Clusters []KubernetesClusterTopology `json:"clusters"`
}

func (t KubernetesTopology) NormalizeCollections() KubernetesTopology {
	if t.Clusters == nil {
		t.Clusters = []KubernetesClusterTopology{}
	}
	for i := range t.Clusters {
		t.Clusters[i] = t.Clusters[i].NormalizeCollections()
	}
	return t
}

// KubernetesClusterTopology represents a Kubernetes cluster and selected children.
type KubernetesClusterTopology struct {
	GovernedResourceMetadata
	Name            string                       `json:"name"`
	ID              string                       `json:"id,omitempty"`
	Status          string                       `json:"status"`
	Nodes           []KubernetesNodeTopology     `json:"nodes"`
	Deployments     []KubernetesDeploymentDetail `json:"deployments"`
	Pods            []KubernetesPodDetail        `json:"pods"`
	NodeCount       int                          `json:"node_count"`
	DeploymentCount int                          `json:"deployment_count"`
	PodCount        int                          `json:"pod_count"`
}

func (t KubernetesClusterTopology) NormalizeCollections() KubernetesClusterTopology {
	if t.Nodes == nil {
		t.Nodes = []KubernetesNodeTopology{}
	}
	for i := range t.Nodes {
		t.Nodes[i] = t.Nodes[i].NormalizeCollections()
	}
	if t.Deployments == nil {
		t.Deployments = []KubernetesDeploymentDetail{}
	}
	if t.Pods == nil {
		t.Pods = []KubernetesPodDetail{}
	}
	return t
}

// KubernetesNodeTopology represents a Kubernetes node in topology output.
type KubernetesNodeTopology struct {
	GovernedResourceMetadata
	Name   string   `json:"name"`
	Status string   `json:"status"`
	Ready  bool     `json:"ready"`
	Roles  []string `json:"roles"`
	CPU    float64  `json:"cpu_percent,omitempty"`
	Memory float64  `json:"memory_percent,omitempty"`
}

func (t KubernetesNodeTopology) NormalizeCollections() KubernetesNodeTopology {
	if t.Roles == nil {
		t.Roles = []string{}
	}
	return t
}

// KubernetesDeploymentDetail represents a deployment under a cluster.
type KubernetesDeploymentDetail struct {
	GovernedResourceMetadata
	Name            string `json:"name"`
	Namespace       string `json:"namespace"`
	Status          string `json:"status"`
	DesiredReplicas int32  `json:"desired_replicas,omitempty"`
	ReadyReplicas   int32  `json:"ready_replicas,omitempty"`
}

// KubernetesPodDetail represents a pod under a cluster.
type KubernetesPodDetail struct {
	GovernedResourceMetadata
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Status    string `json:"status"`
	Restarts  int    `json:"restarts,omitempty"`
	OwnerKind string `json:"owner_kind,omitempty"`
	OwnerName string `json:"owner_name,omitempty"`
}

// TopologySummary provides aggregate counts and status
type TopologySummary struct {
	TotalNodes            int `json:"total_nodes"`
	TotalVMs              int `json:"total_vms"`
	TotalSystemContainers int `json:"total_system_containers"`
	TotalDockerHosts      int `json:"total_docker_hosts"`
	TotalDockerContainers int `json:"total_docker_containers"`
	TotalK8sClusters      int `json:"total_k8s_clusters"`
	TotalK8sNodes         int `json:"total_k8s_nodes"`
	TotalK8sDeployments   int `json:"total_k8s_deployments"`
	TotalK8sPods          int `json:"total_k8s_pods"`
	NodesWithAgents       int `json:"nodes_with_agents"`
	DockerHostsWithAgents int `json:"docker_hosts_with_agents"`
	RunningVMs            int `json:"running_vms"`
	RunningContainers     int `json:"running_containers"`
	RunningDocker         int `json:"running_docker"`
	RunningK8sPods        int `json:"running_k8s_pods"`
}

// ResourceResponse is returned by pulse_get_resource
type ResourceResponse struct {
	GovernedResourceMetadata
	Type            string            `json:"type"` // "vm", "system-container", "app-container"
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	Status          string            `json:"status"`
	Node            string            `json:"node,omitempty"`
	Host            string            `json:"host,omitempty"`
	CPU             ResourceCPU       `json:"cpu"`
	Memory          ResourceMemory    `json:"memory"`
	Disk            *ResourceDisk     `json:"disk,omitempty"`
	OS              string            `json:"os,omitempty"`
	Tags            []string          `json:"tags"`
	Networks        []NetworkInfo     `json:"networks"`
	Ports           []PortInfo        `json:"ports"`
	Mounts          []MountInfo       `json:"mounts"`
	Labels          map[string]string `json:"labels"`
	LastBackup      *time.Time        `json:"last_backup,omitempty"`
	Image           string            `json:"image,omitempty"`
	Health          string            `json:"health,omitempty"`
	RestartCount    int               `json:"restart_count,omitempty"`
	UpdateAvailable bool              `json:"update_available,omitempty"`
}

func EmptyResourceResponse() ResourceResponse {
	return ResourceResponse{}.NormalizeCollections()
}

func (r ResourceResponse) NormalizeCollections() ResourceResponse {
	if r.Tags == nil {
		r.Tags = []string{}
	}
	if r.Networks == nil {
		r.Networks = []NetworkInfo{}
	}
	if r.Ports == nil {
		r.Ports = []PortInfo{}
	}
	if r.Mounts == nil {
		r.Mounts = []MountInfo{}
	}
	if r.Labels == nil {
		r.Labels = map[string]string{}
	}
	for i := range r.Networks {
		r.Networks[i] = r.Networks[i].NormalizeCollections()
	}
	return r
}

// GuestConfigResponse is returned by pulse_get_guest_config.
type GuestConfigResponse struct {
	GovernedResourceMetadata
	GuestType string             `json:"guest_type"`
	VMID      int                `json:"vmid"`
	Name      string             `json:"name,omitempty"`
	Node      string             `json:"node,omitempty"`
	Instance  string             `json:"instance,omitempty"`
	Hostname  string             `json:"hostname,omitempty"`
	OSType    string             `json:"os_type,omitempty"`
	Onboot    *bool              `json:"onboot,omitempty"`
	RootFS    string             `json:"rootfs,omitempty"`
	Mounts    []GuestMountConfig `json:"mounts"`
	Disks     []GuestDiskConfig  `json:"disks"`
	Raw       map[string]string  `json:"raw"`
}

func EmptyGuestConfigResponse() GuestConfigResponse {
	return GuestConfigResponse{}.NormalizeCollections()
}

func (r GuestConfigResponse) NormalizeCollections() GuestConfigResponse {
	if r.Mounts == nil {
		r.Mounts = []GuestMountConfig{}
	}
	if r.Disks == nil {
		r.Disks = []GuestDiskConfig{}
	}
	if r.Raw == nil {
		r.Raw = map[string]string{}
	}
	return r
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

func (n NetworkInfo) NormalizeCollections() NetworkInfo {
	if n.Addresses == nil {
		n.Addresses = []string{}
	}
	return n
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

func EmptyURLFetchResponse() URLFetchResponse {
	return URLFetchResponse{}.NormalizeCollections()
}

func (r URLFetchResponse) NormalizeCollections() URLFetchResponse {
	if r.Headers == nil {
		r.Headers = map[string]string{}
	}
	return r
}

// AlertsResponse is returned by pulse_list_alerts
type AlertsResponse struct {
	Alerts     []ActiveAlert   `json:"alerts"`
	Count      int             `json:"count"`
	Pagination *PaginationInfo `json:"pagination,omitempty"`
}

func EmptyAlertsResponse() AlertsResponse {
	return AlertsResponse{}.NormalizeCollections()
}

func (r AlertsResponse) NormalizeCollections() AlertsResponse {
	if r.Alerts == nil {
		r.Alerts = []ActiveAlert{}
	}
	return r
}

// FindingsResponse is returned by pulse_list_findings
type FindingsResponse struct {
	Active     []Finding       `json:"active"`
	Dismissed  []Finding       `json:"dismissed"`
	Counts     FindingCounts   `json:"counts"`
	Pagination *PaginationInfo `json:"pagination,omitempty"`
}

func EmptyFindingsResponse() FindingsResponse {
	return FindingsResponse{}.NormalizeCollections()
}

func (r FindingsResponse) NormalizeCollections() FindingsResponse {
	if r.Active == nil {
		r.Active = []Finding{}
	}
	if r.Dismissed == nil {
		r.Dismissed = []Finding{}
	}
	return r
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
	Points        []MetricPoint                     `json:"points"`
	Summary       map[string]ResourceMetricsSummary `json:"summary"`
	Pagination    *PaginationInfo                   `json:"pagination,omitempty"`
	Downsampled   bool                              `json:"downsampled,omitempty"`
	OriginalCount int                               `json:"original_count,omitempty"`
}

func EmptyMetricsResponse() MetricsResponse {
	return MetricsResponse{}.NormalizeCollections()
}

func (r MetricsResponse) NormalizeCollections() MetricsResponse {
	if r.Points == nil {
		r.Points = []MetricPoint{}
	}
	if r.Summary == nil {
		r.Summary = map[string]ResourceMetricsSummary{}
	}
	return r
}

// BaselinesResponse is returned by pulse_get_baselines
type BaselinesResponse struct {
	ResourceID string                                `json:"resource_id,omitempty"`
	Baselines  map[string]map[string]*MetricBaseline `json:"baselines"` // resourceID -> metric -> baseline
	Pagination *PaginationInfo                       `json:"pagination,omitempty"`
}

func EmptyBaselinesResponse() BaselinesResponse {
	return BaselinesResponse{}.NormalizeCollections()
}

func (r BaselinesResponse) NormalizeCollections() BaselinesResponse {
	if r.Baselines == nil {
		r.Baselines = map[string]map[string]*MetricBaseline{}
	}
	return r
}

// PatternsResponse is returned by pulse_get_patterns
type PatternsResponse struct {
	Patterns    []Pattern    `json:"patterns"`
	Predictions []Prediction `json:"predictions"`
}

func EmptyPatternsResponse() PatternsResponse {
	return PatternsResponse{}.NormalizeCollections()
}

func (r PatternsResponse) NormalizeCollections() PatternsResponse {
	if r.Patterns == nil {
		r.Patterns = []Pattern{}
	}
	if r.Predictions == nil {
		r.Predictions = []Prediction{}
	}
	return r
}

// BackupsResponse is returned by pulse_list_backups
type BackupsResponse struct {
	PBS         []PBSBackupSummary  `json:"pbs"`
	PVE         []PVEBackupSummary  `json:"pve"`
	PBSServers  []PBSServerSummary  `json:"pbs_servers"`
	RecentTasks []BackupTaskSummary `json:"recent_tasks"`
	Pagination  *PaginationInfo     `json:"pagination,omitempty"`
}

func EmptyBackupsResponse() BackupsResponse {
	return BackupsResponse{}.NormalizeCollections()
}

func (r BackupsResponse) NormalizeCollections() BackupsResponse {
	if r.PBS == nil {
		r.PBS = []PBSBackupSummary{}
	}
	if r.PVE == nil {
		r.PVE = []PVEBackupSummary{}
	}
	if r.PBSServers == nil {
		r.PBSServers = []PBSServerSummary{}
	}
	for i := range r.PBSServers {
		r.PBSServers[i] = r.PBSServers[i].NormalizeCollections()
	}
	if r.RecentTasks == nil {
		r.RecentTasks = []BackupTaskSummary{}
	}
	return r
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

func (s PBSServerSummary) NormalizeCollections() PBSServerSummary {
	if s.Datastores == nil {
		s.Datastores = []DatastoreSummary{}
	}
	return s
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
	Pools        []StoragePoolSummary `json:"pools"`
	CephClusters []CephClusterSummary `json:"ceph_clusters"`
	Pagination   *PaginationInfo      `json:"pagination,omitempty"`
}

func EmptyStorageResponse() StorageResponse {
	return StorageResponse{}.NormalizeCollections()
}

func (r StorageResponse) NormalizeCollections() StorageResponse {
	if r.Pools == nil {
		r.Pools = []StoragePoolSummary{}
	}
	for i := range r.Pools {
		r.Pools[i] = r.Pools[i].NormalizeCollections()
	}
	if r.CephClusters == nil {
		r.CephClusters = []CephClusterSummary{}
	}
	return r
}

// StoragePoolSummary is a summarized storage pool
type StoragePoolSummary struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Node         string          `json:"node,omitempty"`
	Instance     string          `json:"instance,omitempty"`
	Nodes        []string        `json:"nodes"`
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

func (s StoragePoolSummary) NormalizeCollections() StoragePoolSummary {
	if s.Nodes == nil {
		s.Nodes = []string{}
	}
	return s
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

func EmptyDiskHealthResponse() DiskHealthResponse {
	return DiskHealthResponse{}.NormalizeCollections()
}

func (r DiskHealthResponse) NormalizeCollections() DiskHealthResponse {
	if r.Hosts == nil {
		r.Hosts = []HostDiskHealth{}
	}
	for i := range r.Hosts {
		r.Hosts[i] = r.Hosts[i].NormalizeCollections()
	}
	return r
}

// HostDiskHealth is disk health for a single host
type HostDiskHealth struct {
	Hostname string             `json:"hostname"`
	SMART    []SMARTDiskSummary `json:"smart"`
	RAID     []RAIDArraySummary `json:"raid"`
	Ceph     *CephStatusSummary `json:"ceph,omitempty"`
}

func (h HostDiskHealth) NormalizeCollections() HostDiskHealth {
	if h.SMART == nil {
		h.SMART = []SMARTDiskSummary{}
	}
	if h.RAID == nil {
		h.RAID = []RAIDArraySummary{}
	}
	return h
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
	Settings        map[string]interface{} `json:"settings"`
	ObservedModules []string               `json:"observed_modules"`
	CommandsEnabled *bool                  `json:"commands_enabled,omitempty"`
}

func EmptyAgentScopeResponse() AgentScopeResponse {
	return AgentScopeResponse{}.NormalizeCollections()
}

func (r AgentScopeResponse) NormalizeCollections() AgentScopeResponse {
	if r.Settings == nil {
		r.Settings = map[string]interface{}{}
	}
	if r.ObservedModules == nil {
		r.ObservedModules = []string{}
	}
	return r
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
	TargetID        string `json:"target_id"`
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
	Updates  []ContainerUpdateInfo `json:"updates"`
	Total    int                   `json:"total"`
	TargetID string                `json:"target_id,omitempty"`
}

func EmptyDockerUpdatesResponse() DockerUpdatesResponse {
	return DockerUpdatesResponse{}.NormalizeCollections()
}

func (r DockerUpdatesResponse) NormalizeCollections() DockerUpdatesResponse {
	if r.Updates == nil {
		r.Updates = []ContainerUpdateInfo{}
	}
	return r
}

// DockerCheckUpdatesResponse is returned by pulse_check_docker_updates
type DockerCheckUpdatesResponse struct {
	Success   bool                `json:"success"`
	TargetID  string              `json:"target_id"`
	HostName  string              `json:"host_name"`
	CommandID string              `json:"command_id"`
	Message   string              `json:"message"`
	Command   DockerCommandStatus `json:"command"`
}

// DockerUpdateContainerResponse is returned by pulse_update_docker_container
type DockerUpdateContainerResponse struct {
	Success       bool                `json:"success"`
	TargetID      string              `json:"target_id"`
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

func EmptyKubernetesClustersResponse() KubernetesClustersResponse {
	return KubernetesClustersResponse{}.NormalizeCollections()
}

func (r KubernetesClustersResponse) NormalizeCollections() KubernetesClustersResponse {
	if r.Clusters == nil {
		r.Clusters = []KubernetesClusterSummary{}
	}
	return r
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

func EmptyKubernetesNodesResponse() KubernetesNodesResponse {
	return KubernetesNodesResponse{}.NormalizeCollections()
}

func (r KubernetesNodesResponse) NormalizeCollections() KubernetesNodesResponse {
	if r.Nodes == nil {
		r.Nodes = []KubernetesNodeSummary{}
	}
	for i := range r.Nodes {
		r.Nodes[i] = r.Nodes[i].NormalizeCollections()
	}
	return r
}

// KubernetesNodeSummary summarizes a Kubernetes node
type KubernetesNodeSummary struct {
	UID                     string   `json:"uid"`
	Name                    string   `json:"name"`
	Ready                   bool     `json:"ready"`
	Unschedulable           bool     `json:"unschedulable,omitempty"`
	Roles                   []string `json:"roles"`
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

func (s KubernetesNodeSummary) NormalizeCollections() KubernetesNodeSummary {
	if s.Roles == nil {
		s.Roles = []string{}
	}
	return s
}

// KubernetesPodsResponse is returned by pulse_get_kubernetes_pods
type KubernetesPodsResponse struct {
	Cluster  string                 `json:"cluster"`
	Pods     []KubernetesPodSummary `json:"pods"`
	Total    int                    `json:"total"`
	Filtered int                    `json:"filtered,omitempty"`
}

func EmptyKubernetesPodsResponse() KubernetesPodsResponse {
	return KubernetesPodsResponse{}.NormalizeCollections()
}

func (r KubernetesPodsResponse) NormalizeCollections() KubernetesPodsResponse {
	if r.Pods == nil {
		r.Pods = []KubernetesPodSummary{}
	}
	for i := range r.Pods {
		r.Pods[i] = r.Pods[i].NormalizeCollections()
	}
	return r
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
	Containers []KubernetesPodContainerSummary `json:"containers"`
}

func (s KubernetesPodSummary) NormalizeCollections() KubernetesPodSummary {
	if s.Containers == nil {
		s.Containers = []KubernetesPodContainerSummary{}
	}
	return s
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

func EmptyKubernetesDeploymentsResponse() KubernetesDeploymentsResponse {
	return KubernetesDeploymentsResponse{}.NormalizeCollections()
}

func (r KubernetesDeploymentsResponse) NormalizeCollections() KubernetesDeploymentsResponse {
	if r.Deployments == nil {
		r.Deployments = []KubernetesDeploymentSummary{}
	}
	return r
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

func EmptyPMGStatusResponse() PMGStatusResponse {
	return PMGStatusResponse{}.NormalizeCollections()
}

func (r PMGStatusResponse) NormalizeCollections() PMGStatusResponse {
	if r.Instances == nil {
		r.Instances = []PMGInstanceSummary{}
	}
	for i := range r.Instances {
		r.Instances[i] = r.Instances[i].NormalizeCollections()
	}
	return r
}

// PMGInstanceSummary summarizes a PMG instance
type PMGInstanceSummary struct {
	ID      string           `json:"id"`
	Name    string           `json:"name"`
	Host    string           `json:"host"`
	Status  string           `json:"status"`
	Version string           `json:"version,omitempty"`
	Nodes   []PMGNodeSummary `json:"nodes"`
}

func (s PMGInstanceSummary) NormalizeCollections() PMGInstanceSummary {
	if s.Nodes == nil {
		s.Nodes = []PMGNodeSummary{}
	}
	return s
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

func EmptyMailQueuesResponse() MailQueuesResponse {
	return MailQueuesResponse{}.NormalizeCollections()
}

func (r MailQueuesResponse) NormalizeCollections() MailQueuesResponse {
	if r.Queues == nil {
		r.Queues = []PMGQueueSummary{}
	}
	return r
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
	Distribution []PMGSpamBucketSummary `json:"spam_distribution"`
}

func EmptySpamStatsResponse() SpamStatsResponse {
	return SpamStatsResponse{}.NormalizeCollections()
}

func (r SpamStatsResponse) NormalizeCollections() SpamStatsResponse {
	if r.Distribution == nil {
		r.Distribution = []PMGSpamBucketSummary{}
	}
	return r
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

func EmptySnapshotsResponse() SnapshotsResponse {
	return SnapshotsResponse{}.NormalizeCollections()
}

func (r SnapshotsResponse) NormalizeCollections() SnapshotsResponse {
	if r.Snapshots == nil {
		r.Snapshots = []SnapshotSummary{}
	}
	return r
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

func EmptyPBSJobsResponse() PBSJobsResponse {
	return PBSJobsResponse{}.NormalizeCollections()
}

func (r PBSJobsResponse) NormalizeCollections() PBSJobsResponse {
	if r.Jobs == nil {
		r.Jobs = []PBSJobSummary{}
	}
	return r
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

func EmptyBackupTasksListResponse() BackupTasksListResponse {
	return BackupTasksListResponse{}.NormalizeCollections()
}

func (r BackupTasksListResponse) NormalizeCollections() BackupTasksListResponse {
	if r.Tasks == nil {
		r.Tasks = []BackupTaskDetail{}
	}
	return r
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

func EmptyNetworkStatsResponse() NetworkStatsResponse {
	return NetworkStatsResponse{}.NormalizeCollections()
}

func (r NetworkStatsResponse) NormalizeCollections() NetworkStatsResponse {
	if r.Hosts == nil {
		r.Hosts = []HostNetworkStatsSummary{}
	}
	for i := range r.Hosts {
		r.Hosts[i] = r.Hosts[i].NormalizeCollections()
	}
	return r
}

// HostNetworkStatsSummary summarizes network stats for a host
type HostNetworkStatsSummary struct {
	Hostname   string                    `json:"hostname"`
	Interfaces []NetworkInterfaceSummary `json:"interfaces"`
}

func (s HostNetworkStatsSummary) NormalizeCollections() HostNetworkStatsSummary {
	if s.Interfaces == nil {
		s.Interfaces = []NetworkInterfaceSummary{}
	}
	for i := range s.Interfaces {
		s.Interfaces[i] = s.Interfaces[i].NormalizeCollections()
	}
	return s
}

// NetworkInterfaceSummary summarizes a network interface
type NetworkInterfaceSummary struct {
	Name      string   `json:"name"`
	MAC       string   `json:"mac,omitempty"`
	Addresses []string `json:"addresses"`
	RXBytes   uint64   `json:"rx_bytes"`
	TXBytes   uint64   `json:"tx_bytes"`
	SpeedMbps *int64   `json:"speed_mbps,omitempty"`
}

func (s NetworkInterfaceSummary) NormalizeCollections() NetworkInterfaceSummary {
	if s.Addresses == nil {
		s.Addresses = []string{}
	}
	return s
}

// DiskIOStatsResponse is returned by pulse_get_diskio_stats
type DiskIOStatsResponse struct {
	Hosts []HostDiskIOStatsSummary `json:"hosts"`
	Total int                      `json:"total"`
}

func EmptyDiskIOStatsResponse() DiskIOStatsResponse {
	return DiskIOStatsResponse{}.NormalizeCollections()
}

func (r DiskIOStatsResponse) NormalizeCollections() DiskIOStatsResponse {
	if r.Hosts == nil {
		r.Hosts = []HostDiskIOStatsSummary{}
	}
	for i := range r.Hosts {
		r.Hosts[i] = r.Hosts[i].NormalizeCollections()
	}
	return r
}

// HostDiskIOStatsSummary summarizes disk I/O for a host
type HostDiskIOStatsSummary struct {
	Hostname string                `json:"hostname"`
	Devices  []DiskIODeviceSummary `json:"devices"`
}

func (s HostDiskIOStatsSummary) NormalizeCollections() HostDiskIOStatsSummary {
	if s.Devices == nil {
		s.Devices = []DiskIODeviceSummary{}
	}
	return s
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

func EmptyClusterStatusResponse() ClusterStatusResponse {
	return ClusterStatusResponse{}.NormalizeCollections()
}

func (r ClusterStatusResponse) NormalizeCollections() ClusterStatusResponse {
	if r.Clusters == nil {
		r.Clusters = []PVEClusterStatus{}
	}
	for i := range r.Clusters {
		r.Clusters[i] = r.Clusters[i].NormalizeCollections()
	}
	return r
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

func (s PVEClusterStatus) NormalizeCollections() PVEClusterStatus {
	if s.Nodes == nil {
		s.Nodes = []PVEClusterNodeStatus{}
	}
	return s
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

func EmptyDockerServicesResponse() DockerServicesResponse {
	return DockerServicesResponse{}.NormalizeCollections()
}

func (r DockerServicesResponse) NormalizeCollections() DockerServicesResponse {
	if r.Services == nil {
		r.Services = []DockerServiceSummary{}
	}
	return r
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

func EmptyDockerTasksResponse() DockerTasksResponse {
	return DockerTasksResponse{}.NormalizeCollections()
}

func (r DockerTasksResponse) NormalizeCollections() DockerTasksResponse {
	if r.Tasks == nil {
		r.Tasks = []DockerTaskSummary{}
	}
	return r
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

func EmptyRecentTasksResponse() RecentTasksResponse {
	return RecentTasksResponse{}.NormalizeCollections()
}

func (r RecentTasksResponse) NormalizeCollections() RecentTasksResponse {
	if r.Tasks == nil {
		r.Tasks = []ProxmoxTaskSummary{}
	}
	return r
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

func EmptyPhysicalDisksResponse() PhysicalDisksResponse {
	return PhysicalDisksResponse{}.NormalizeCollections()
}

func (r PhysicalDisksResponse) NormalizeCollections() PhysicalDisksResponse {
	if r.Disks == nil {
		r.Disks = []PhysicalDiskSummary{}
	}
	return r
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

func EmptyHostRAIDStatusResponse() HostRAIDStatusResponse {
	return HostRAIDStatusResponse{}.NormalizeCollections()
}

func (r HostRAIDStatusResponse) NormalizeCollections() HostRAIDStatusResponse {
	if r.Hosts == nil {
		r.Hosts = []HostRAIDSummary{}
	}
	for i := range r.Hosts {
		r.Hosts[i] = r.Hosts[i].NormalizeCollections()
	}
	return r
}

// HostRAIDSummary summarizes RAID arrays for a host
type HostRAIDSummary struct {
	Hostname string                 `json:"hostname"`
	TargetID string                 `json:"target_id"`
	Arrays   []HostRAIDArraySummary `json:"arrays"`
}

func (s HostRAIDSummary) NormalizeCollections() HostRAIDSummary {
	if s.Arrays == nil {
		s.Arrays = []HostRAIDArraySummary{}
	}
	for i := range s.Arrays {
		s.Arrays[i] = s.Arrays[i].NormalizeCollections()
	}
	return s
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
	Devices        []HostRAIDDeviceSummary `json:"devices"`
}

func (s HostRAIDArraySummary) NormalizeCollections() HostRAIDArraySummary {
	if s.Devices == nil {
		s.Devices = []HostRAIDDeviceSummary{}
	}
	return s
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

func EmptyHostCephDetailsResponse() HostCephDetailsResponse {
	return HostCephDetailsResponse{}.NormalizeCollections()
}

func (r HostCephDetailsResponse) NormalizeCollections() HostCephDetailsResponse {
	if r.Hosts == nil {
		r.Hosts = []HostCephSummary{}
	}
	for i := range r.Hosts {
		r.Hosts[i] = r.Hosts[i].NormalizeCollections()
	}
	return r
}

// HostCephSummary summarizes host-collected Ceph cluster details
type HostCephSummary struct {
	Hostname    string                `json:"hostname"`
	TargetID    string                `json:"target_id"`
	FSID        string                `json:"fsid"`
	Health      HostCephHealthSummary `json:"health"`
	MonMap      *HostCephMonSummary   `json:"mon_map,omitempty"`
	MgrMap      *HostCephMgrSummary   `json:"mgr_map,omitempty"`
	OSDMap      HostCephOSDSummary    `json:"osd_map"`
	PGMap       HostCephPGSummary     `json:"pg_map"`
	Pools       []HostCephPoolSummary `json:"pools"`
	CollectedAt time.Time             `json:"collected_at"`
}

func (s HostCephSummary) NormalizeCollections() HostCephSummary {
	s.Health = s.Health.NormalizeCollections()
	if s.MonMap != nil {
		mon := s.MonMap.NormalizeCollections()
		s.MonMap = &mon
	}
	if s.Pools == nil {
		s.Pools = []HostCephPoolSummary{}
	}
	return s
}

// HostCephHealthSummary summarizes Ceph health
type HostCephHealthSummary struct {
	Status   string                  `json:"status"` // HEALTH_OK, HEALTH_WARN, HEALTH_ERR
	Messages []HostCephHealthMessage `json:"messages"`
}

func (s HostCephHealthSummary) NormalizeCollections() HostCephHealthSummary {
	if s.Messages == nil {
		s.Messages = []HostCephHealthMessage{}
	}
	return s
}

// HostCephHealthMessage represents a health check message
type HostCephHealthMessage struct {
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

// HostCephMonSummary summarizes Ceph monitors
type HostCephMonSummary struct {
	NumMons  int                      `json:"num_mons"`
	Monitors []HostCephMonitorSummary `json:"monitors"`
}

func (s HostCephMonSummary) NormalizeCollections() HostCephMonSummary {
	if s.Monitors == nil {
		s.Monitors = []HostCephMonitorSummary{}
	}
	return s
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

func EmptyResourceDisksResponse() ResourceDisksResponse {
	return ResourceDisksResponse{}.NormalizeCollections()
}

func (r ResourceDisksResponse) NormalizeCollections() ResourceDisksResponse {
	if r.Resources == nil {
		r.Resources = []ResourceDisksSummary{}
	}
	for i := range r.Resources {
		r.Resources[i] = r.Resources[i].NormalizeCollections()
	}
	return r
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

func (s ResourceDisksSummary) NormalizeCollections() ResourceDisksSummary {
	if s.Disks == nil {
		s.Disks = []ResourceDiskInfo{}
	}
	return s
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

func EmptyConnectionHealthResponse() ConnectionHealthResponse {
	return ConnectionHealthResponse{}.NormalizeCollections()
}

func (r ConnectionHealthResponse) NormalizeCollections() ConnectionHealthResponse {
	if r.Connections == nil {
		r.Connections = []ConnectionStatus{}
	}
	return r
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

func EmptyResolvedAlertsResponse() ResolvedAlertsResponse {
	return ResolvedAlertsResponse{}.NormalizeCollections()
}

func (r ResolvedAlertsResponse) NormalizeCollections() ResolvedAlertsResponse {
	if r.Alerts == nil {
		r.Alerts = []ResolvedAlertSummary{}
	}
	return r
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
