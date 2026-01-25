// Package discovery provides AI-powered infrastructure discovery capabilities.
// It discovers services, versions, configurations, and CLI access methods
// for VMs, LXCs, Docker containers, Kubernetes pods, and hosts.
package aidiscovery

import (
	"fmt"
	"time"
)

// ResourceType identifies the type of infrastructure resource.
type ResourceType string

const (
	ResourceTypeVM        ResourceType = "vm"
	ResourceTypeLXC       ResourceType = "lxc"
	ResourceTypeDocker    ResourceType = "docker"
	ResourceTypeK8s       ResourceType = "k8s"
	ResourceTypeHost      ResourceType = "host"
	ResourceTypeDockerVM  ResourceType = "docker_vm"  // Docker on a VM
	ResourceTypeDockerLXC ResourceType = "docker_lxc" // Docker in an LXC
)

// FactCategory categorizes discovery facts.
type FactCategory string

const (
	FactCategoryVersion    FactCategory = "version"
	FactCategoryConfig     FactCategory = "config"
	FactCategoryService    FactCategory = "service"
	FactCategoryPort       FactCategory = "port"
	FactCategoryHardware   FactCategory = "hardware"
	FactCategoryNetwork    FactCategory = "network"
	FactCategoryStorage    FactCategory = "storage"
	FactCategoryDependency FactCategory = "dependency"
	FactCategorySecurity   FactCategory = "security"
)

// ServiceCategory categorizes the type of service discovered.
type ServiceCategory string

const (
	CategoryDatabase     ServiceCategory = "database"
	CategoryWebServer    ServiceCategory = "web_server"
	CategoryCache        ServiceCategory = "cache"
	CategoryMessageQueue ServiceCategory = "message_queue"
	CategoryMonitoring   ServiceCategory = "monitoring"
	CategoryBackup       ServiceCategory = "backup"
	CategoryNVR          ServiceCategory = "nvr"
	CategoryStorage      ServiceCategory = "storage"
	CategoryContainer    ServiceCategory = "container"
	CategoryVirtualizer  ServiceCategory = "virtualizer"
	CategoryNetwork      ServiceCategory = "network"
	CategorySecurity     ServiceCategory = "security"
	CategoryMedia        ServiceCategory = "media"
	CategoryHomeAuto     ServiceCategory = "home_automation"
	CategoryUnknown      ServiceCategory = "unknown"
)

// ResourceDiscovery is the main data model for discovered resource information.
type ResourceDiscovery struct {
	// Identity
	ID           string       `json:"id"`            // Unique ID: "lxc:minipc:101"
	ResourceType ResourceType `json:"resource_type"` // vm, lxc, docker, k8s, host
	ResourceID   string       `json:"resource_id"`   // 101, container-name, etc.
	HostID       string       `json:"host_id"`       // Proxmox node name or host agent ID
	Hostname     string       `json:"hostname"`      // Human-readable host name

	// AI-discovered info
	ServiceType    string          `json:"service_type"`    // frigate, postgres, pbs
	ServiceName    string          `json:"service_name"`    // Human-readable name
	ServiceVersion string          `json:"service_version"` // v0.13.2
	Category       ServiceCategory `json:"category"`        // nvr, database, backup
	CLIAccess      string          `json:"cli_access"`      // pct exec 101 -- ...

	// Deep discovery facts
	Facts       []DiscoveryFact `json:"facts"`
	ConfigPaths []string        `json:"config_paths"`
	DataPaths   []string        `json:"data_paths"`
	Ports       []PortInfo      `json:"ports"`

	// User-added (also encrypted)
	UserNotes   string            `json:"user_notes"`
	UserSecrets map[string]string `json:"user_secrets"` // tokens, creds

	// Metadata
	Confidence   float64   `json:"confidence"`    // 0-1 confidence score
	AIReasoning  string    `json:"ai_reasoning"`  // AI explanation
	DiscoveredAt time.Time `json:"discovered_at"` // First discovery
	UpdatedAt    time.Time `json:"updated_at"`    // Last update
	ScanDuration int64     `json:"scan_duration"` // Scan duration in ms

	// Raw data for debugging/re-analysis
	RawCommandOutput map[string]string `json:"raw_command_output,omitempty"`
}

// DiscoveryFact represents a single discovered fact about a resource.
type DiscoveryFact struct {
	Category     FactCategory `json:"category"`   // version, config, service, port
	Key          string       `json:"key"`        // e.g., "coral_tpu", "mqtt_broker"
	Value        string       `json:"value"`      // e.g., "/dev/apex_0", "mosquitto:1883"
	Source       string       `json:"source"`     // command that found this
	Confidence   float64      `json:"confidence"` // 0-1 confidence for this fact
	DiscoveredAt time.Time    `json:"discovered_at"`
}

// PortInfo represents information about a listening port.
type PortInfo struct {
	Port     int    `json:"port"`
	Protocol string `json:"protocol"` // tcp, udp
	Process  string `json:"process"`  // process name
	Address  string `json:"address"`  // bind address
}

// MakeResourceID creates a standardized resource ID.
func MakeResourceID(resourceType ResourceType, hostID, resourceID string) string {
	return fmt.Sprintf("%s:%s:%s", resourceType, hostID, resourceID)
}

// ParseResourceID parses a resource ID into its components.
func ParseResourceID(id string) (resourceType ResourceType, hostID, resourceID string, err error) {
	var parts [3]string
	count := 0
	start := 0
	for i, c := range id {
		if c == ':' {
			if count < 2 {
				parts[count] = id[start:i]
				count++
				start = i + 1
			}
		}
	}
	if count == 2 {
		parts[2] = id[start:]
		return ResourceType(parts[0]), parts[1], parts[2], nil
	}
	return "", "", "", fmt.Errorf("invalid resource ID format: %s", id)
}

// DiscoveryRequest represents a request to discover a resource.
type DiscoveryRequest struct {
	ResourceType ResourceType `json:"resource_type"`
	ResourceID   string       `json:"resource_id"`
	HostID       string       `json:"host_id"`
	Hostname     string       `json:"hostname"`
	Force        bool         `json:"force"` // Force re-scan even if recent
}

// DiscoveryStatus represents the status of a discovery scan.
type DiscoveryStatus string

const (
	DiscoveryStatusPending    DiscoveryStatus = "pending"
	DiscoveryStatusRunning    DiscoveryStatus = "running"
	DiscoveryStatusCompleted  DiscoveryStatus = "completed"
	DiscoveryStatusFailed     DiscoveryStatus = "failed"
	DiscoveryStatusNotStarted DiscoveryStatus = "not_started"
)

// DiscoveryProgress represents the progress of an ongoing discovery.
type DiscoveryProgress struct {
	ResourceID     string          `json:"resource_id"`
	Status         DiscoveryStatus `json:"status"`
	CurrentStep    string          `json:"current_step"`
	TotalSteps     int             `json:"total_steps"`
	CompletedSteps int             `json:"completed_steps"`
	StartedAt      time.Time       `json:"started_at"`
	Error          string          `json:"error,omitempty"`
}

// UpdateNotesRequest represents a request to update user notes.
type UpdateNotesRequest struct {
	UserNotes   string            `json:"user_notes"`
	UserSecrets map[string]string `json:"user_secrets,omitempty"`
}

// DiscoverySummary provides a summary of discoveries for listing.
type DiscoverySummary struct {
	ID             string          `json:"id"`
	ResourceType   ResourceType    `json:"resource_type"`
	ResourceID     string          `json:"resource_id"`
	HostID         string          `json:"host_id"`
	Hostname       string          `json:"hostname"`
	ServiceType    string          `json:"service_type"`
	ServiceName    string          `json:"service_name"`
	ServiceVersion string          `json:"service_version"`
	Category       ServiceCategory `json:"category"`
	Confidence     float64         `json:"confidence"`
	HasUserNotes   bool            `json:"has_user_notes"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

// ToSummary converts a full discovery to a summary.
func (d *ResourceDiscovery) ToSummary() DiscoverySummary {
	return DiscoverySummary{
		ID:             d.ID,
		ResourceType:   d.ResourceType,
		ResourceID:     d.ResourceID,
		HostID:         d.HostID,
		Hostname:       d.Hostname,
		ServiceType:    d.ServiceType,
		ServiceName:    d.ServiceName,
		ServiceVersion: d.ServiceVersion,
		Category:       d.Category,
		Confidence:     d.Confidence,
		HasUserNotes:   d.UserNotes != "",
		UpdatedAt:      d.UpdatedAt,
	}
}

// AIAnalysisRequest is sent to the AI for analysis.
type AIAnalysisRequest struct {
	ResourceType   ResourceType      `json:"resource_type"`
	ResourceID     string            `json:"resource_id"`
	HostID         string            `json:"host_id"`
	Hostname       string            `json:"hostname"`
	CommandOutputs map[string]string `json:"command_outputs"`
	ExistingFacts  []DiscoveryFact   `json:"existing_facts,omitempty"`
	Metadata       map[string]any    `json:"metadata,omitempty"` // Image, labels, etc.
}

// AIAnalysisResponse is returned by the AI.
type AIAnalysisResponse struct {
	ServiceType    string          `json:"service_type"`
	ServiceName    string          `json:"service_name"`
	ServiceVersion string          `json:"service_version"`
	Category       ServiceCategory `json:"category"`
	CLIAccess      string          `json:"cli_access"`
	Facts          []DiscoveryFact `json:"facts"`
	ConfigPaths    []string        `json:"config_paths"`
	DataPaths      []string        `json:"data_paths"`
	Ports          []PortInfo      `json:"ports"`
	Confidence     float64         `json:"confidence"`
	Reasoning      string          `json:"reasoning"`
}
