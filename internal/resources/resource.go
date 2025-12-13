// Package resources provides a unified abstraction for all monitored entities
// across different platforms (Proxmox, Docker, Kubernetes, TrueNAS, etc.).
//
// The Resource type is the core abstraction that normalizes platform-specific
// types like Node, Host, DockerHost, VM, Container into a common data model.
// This enables:
//   - AI intelligence across all platforms
//   - Elimination of duplicate monitoring (one machine = one set of alerts)
//   - Extensibility for new platforms
//   - Foundation for unified views
package resources

import (
	"encoding/json"
	"time"
)

// Resource is the universal abstraction for any monitored entity.
// All platform-specific types (Node, VM, Container, DockerHost, etc.) can be
// converted to this common type for unified handling.
type Resource struct {
	// Identity
	ID          string       `json:"id"`          // Globally unique ID
	Type        ResourceType `json:"type"`        // vm, container, docker-container, pod, host, etc.
	Name        string       `json:"name"`        // Human-readable name
	DisplayName string       `json:"displayName"` // Custom display name (if set)

	// Platform/Source
	PlatformID   string       `json:"platformId"`   // Which platform instance (e.g., cluster URL)
	PlatformType PlatformType `json:"platformType"` // proxmox-pve, docker, kubernetes, etc.
	SourceType   SourceType   `json:"sourceType"`   // api, agent, hybrid

	// Hierarchy
	ParentID  string `json:"parentId,omitempty"`  // VM → Node, Pod → K8s Node
	ClusterID string `json:"clusterId,omitempty"` // Cluster membership

	// Universal Metrics (nullable - not all resources have all metrics)
	Status      ResourceStatus `json:"status"`                // online, offline, running, stopped, degraded
	CPU         *MetricValue   `json:"cpu,omitempty"`         // CPU usage percentage
	Memory      *MetricValue   `json:"memory,omitempty"`      // Memory usage
	Disk        *MetricValue   `json:"disk,omitempty"`        // Primary disk usage
	Network     *NetworkMetric `json:"network,omitempty"`     // Network I/O
	Temperature *float64       `json:"temperature,omitempty"` // Temperature in Celsius
	Uptime      *int64         `json:"uptime,omitempty"`      // Uptime in seconds

	// Universal Metadata
	Tags     []string          `json:"tags,omitempty"`
	Labels   map[string]string `json:"labels,omitempty"`
	LastSeen time.Time         `json:"lastSeen"`
	Alerts   []ResourceAlert   `json:"alerts,omitempty"`

	// Platform-Specific Data (discriminated by Type)
	// This preserves all the rich data while allowing common handling.
	// Use GetPlatformData() to unmarshal into the appropriate type.
	PlatformData json.RawMessage `json:"platformData,omitempty"`

	// Identity information for deduplication
	Identity *ResourceIdentity `json:"identity,omitempty"`

	// Schema version for future evolution
	SchemaVersion int `json:"schemaVersion"`
}

// ResourceType identifies the kind of monitored entity.
type ResourceType string

const (
	// Infrastructure - physical or virtual hosts that run workloads
	ResourceTypeNode       ResourceType = "node"        // Proxmox VE node
	ResourceTypeHost       ResourceType = "host"        // Standalone host (via host-agent)
	ResourceTypeDockerHost ResourceType = "docker-host" // Docker/Podman host
	ResourceTypeK8sCluster ResourceType = "k8s-cluster" // Kubernetes cluster
	ResourceTypeK8sNode    ResourceType = "k8s-node"    // Kubernetes node
	ResourceTypeTrueNAS    ResourceType = "truenas"     // TrueNAS system

	// Compute Workloads - individual running instances
	ResourceTypeVM              ResourceType = "vm"               // Proxmox VM
	ResourceTypeContainer       ResourceType = "container"        // LXC container
	ResourceTypeOCIContainer    ResourceType = "oci-container"    // OCI container (Proxmox VE 9.1+)
	ResourceTypeDockerContainer ResourceType = "docker-container" // Docker container
	ResourceTypePod             ResourceType = "pod"              // Kubernetes pod
	ResourceTypeJail            ResourceType = "jail"             // BSD jail / TrueNAS jail

	// Services - logical groupings of workloads
	ResourceTypeDockerService ResourceType = "docker-service" // Docker Swarm service
	ResourceTypeK8sDeployment ResourceType = "k8s-deployment" // Kubernetes deployment
	ResourceTypeK8sService    ResourceType = "k8s-service"    // Kubernetes service

	// Storage - storage resources
	ResourceTypeStorage   ResourceType = "storage"   // Generic storage
	ResourceTypeDatastore ResourceType = "datastore" // PBS datastore
	ResourceTypePool      ResourceType = "pool"      // ZFS/Ceph pool
	ResourceTypeDataset   ResourceType = "dataset"   // ZFS dataset

	// Backup Systems
	ResourceTypePBS ResourceType = "pbs" // Proxmox Backup Server
	ResourceTypePMG ResourceType = "pmg" // Proxmox Mail Gateway
)

// PlatformType identifies the source platform/system.
type PlatformType string

const (
	PlatformProxmoxPVE PlatformType = "proxmox-pve"
	PlatformProxmoxPBS PlatformType = "proxmox-pbs"
	PlatformProxmoxPMG PlatformType = "proxmox-pmg"
	PlatformDocker     PlatformType = "docker"
	PlatformKubernetes PlatformType = "kubernetes"
	PlatformTrueNAS    PlatformType = "truenas"
	PlatformHostAgent  PlatformType = "host-agent"
)

// SourceType indicates how data is collected for this resource.
type SourceType string

const (
	SourceAPI    SourceType = "api"    // Data from polling an API
	SourceAgent  SourceType = "agent"  // Data pushed from agent
	SourceHybrid SourceType = "hybrid" // Both sources, agent preferred
)

// ResourceStatus represents the operational state of a resource.
type ResourceStatus string

const (
	StatusOnline   ResourceStatus = "online"
	StatusOffline  ResourceStatus = "offline"
	StatusRunning  ResourceStatus = "running"
	StatusStopped  ResourceStatus = "stopped"
	StatusDegraded ResourceStatus = "degraded"
	StatusPaused   ResourceStatus = "paused"
	StatusUnknown  ResourceStatus = "unknown"
)

// MetricValue represents a metric with current value and optional limits.
type MetricValue struct {
	Current float64 `json:"current"`         // Current value (percentage for CPU, bytes for memory/disk)
	Total   *int64  `json:"total,omitempty"` // Total capacity (bytes) - nil for percentages like CPU
	Used    *int64  `json:"used,omitempty"`  // Used amount (bytes) - nil for percentages
	Free    *int64  `json:"free,omitempty"`  // Free amount (bytes) - nil for percentages
}

// NetworkMetric captures network I/O.
type NetworkMetric struct {
	RXBytes int64 `json:"rxBytes"` // Total bytes received
	TXBytes int64 `json:"txBytes"` // Total bytes transmitted
}

// ResourceAlert represents an alert associated with a resource.
type ResourceAlert struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`  // cpu, memory, disk, temperature, etc.
	Level     string    `json:"level"` // warning, critical
	Message   string    `json:"message"`
	Value     float64   `json:"value"`
	Threshold float64   `json:"threshold"`
	StartTime time.Time `json:"startTime"`
}

// ResourceIdentity contains information used for deduplication.
// When multiple sources report on the same physical machine, we use
// these fields to identify and merge them.
type ResourceIdentity struct {
	Hostname  string   `json:"hostname,omitempty"`  // Primary identifier
	MachineID string   `json:"machineId,omitempty"` // /etc/machine-id or equivalent
	IPs       []string `json:"ips,omitempty"`       // Network addresses
}

// CurrentSchemaVersion is the current version of the Resource schema.
const CurrentSchemaVersion = 1

// GetPlatformData unmarshals the PlatformData into the provided type.
// Example: var nodeData NodePlatformData; r.GetPlatformData(&nodeData)
func (r *Resource) GetPlatformData(v interface{}) error {
	if r.PlatformData == nil {
		return nil
	}
	return json.Unmarshal(r.PlatformData, v)
}

// SetPlatformData marshals the provided value into PlatformData.
func (r *Resource) SetPlatformData(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	r.PlatformData = data
	return nil
}

// IsInfrastructure returns true if this resource is an infrastructure host
// (node, host, docker-host) rather than a workload (vm, container).
func (r *Resource) IsInfrastructure() bool {
	switch r.Type {
	case ResourceTypeNode, ResourceTypeHost, ResourceTypeDockerHost, ResourceTypeK8sCluster, ResourceTypeK8sNode, ResourceTypeTrueNAS:
		return true
	default:
		return false
	}
}

// IsWorkload returns true if this resource is a workload (vm, container, pod)
// rather than infrastructure.
func (r *Resource) IsWorkload() bool {
	switch r.Type {
	case ResourceTypeVM, ResourceTypeContainer, ResourceTypeOCIContainer, ResourceTypeDockerContainer, ResourceTypePod, ResourceTypeJail:
		return true
	default:
		return false
	}
}

// EffectiveDisplayName returns DisplayName if set, otherwise Name.
func (r *Resource) EffectiveDisplayName() string {
	if r.DisplayName != "" {
		return r.DisplayName
	}
	return r.Name
}

// CPUPercent returns the CPU usage as a percentage, or 0 if not available.
func (r *Resource) CPUPercent() float64 {
	if r.CPU == nil {
		return 0
	}
	return r.CPU.Current
}

// MemoryPercent returns the memory usage as a percentage, or 0 if not available.
func (r *Resource) MemoryPercent() float64 {
	if r.Memory == nil {
		return 0
	}
	// If we have used/total, calculate percentage
	if r.Memory.Total != nil && *r.Memory.Total > 0 && r.Memory.Used != nil {
		return float64(*r.Memory.Used) / float64(*r.Memory.Total) * 100
	}
	// Otherwise, Current is the percentage
	return r.Memory.Current
}

// DiskPercent returns the disk usage as a percentage, or 0 if not available.
func (r *Resource) DiskPercent() float64 {
	if r.Disk == nil {
		return 0
	}
	if r.Disk.Total != nil && *r.Disk.Total > 0 && r.Disk.Used != nil {
		return float64(*r.Disk.Used) / float64(*r.Disk.Total) * 100
	}
	return r.Disk.Current
}
