// Package discovery provides AI-powered infrastructure discovery capabilities.
// It discovers services, versions, configurations, and CLI access methods
// for VMs, containers, Docker containers, Kubernetes pods, and hosts.
package servicediscovery

import (
	"fmt"
	"time"
)

// ResourceType identifies the type of infrastructure resource.
type ResourceType string

const (
	ResourceTypeVM                    ResourceType = "vm"
	ResourceTypeSystemContainer       ResourceType = "system-container"
	ResourceTypeDocker                ResourceType = "docker"
	ResourceTypeK8s                   ResourceType = "k8s"
	ResourceTypeHost                  ResourceType = "host"
	ResourceTypeDockerVM              ResourceType = "docker_vm"               // Docker on a VM
	ResourceTypeDockerSystemContainer ResourceType = "docker_system-container" // Docker in a system container
)

// FactCategory categorizes discovery facts.
type FactCategory string

const (
	FactCategoryVersion    FactCategory = "version"
	FactCategoryService    FactCategory = "service"
	FactCategoryHardware   FactCategory = "hardware"
	FactCategoryStorage    FactCategory = "storage"
	FactCategoryDependency FactCategory = "dependency"
	FactCategorySecurity   FactCategory = "security"
)

// ServiceCategory categorizes the type of service discovered.
type ServiceCategory string

const (
	CategoryDatabase    ServiceCategory = "database"
	CategoryWebServer   ServiceCategory = "web_server"
	CategoryCache       ServiceCategory = "cache"
	CategoryMonitoring  ServiceCategory = "monitoring"
	CategoryBackup      ServiceCategory = "backup"
	CategoryNVR         ServiceCategory = "nvr"
	CategoryStorage     ServiceCategory = "storage"
	CategoryVirtualizer ServiceCategory = "virtualizer"
	CategoryNetwork     ServiceCategory = "network"
	CategorySecurity    ServiceCategory = "security"
	CategoryMedia       ServiceCategory = "media"
	CategoryHomeAuto    ServiceCategory = "home_automation"
	CategoryUnknown     ServiceCategory = "unknown"
)

// ResourceDiscovery is the main data model for discovered resource information.
type ResourceDiscovery struct {
	// Identity
	ID           string       `json:"id"`            // Unique ID: "system-container:minipc:101"
	ResourceType ResourceType `json:"resource_type"` // vm, system-container, docker, k8s, host
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
	Facts        []DiscoveryFact   `json:"facts"`
	ConfigPaths  []string          `json:"config_paths"`
	DataPaths    []string          `json:"data_paths"`
	LogPaths     []string          `json:"log_paths"`
	Ports        []PortInfo        `json:"ports"`
	DockerMounts []DockerBindMount `json:"docker_mounts,omitempty"` // Docker container bind mounts (source->dest)

	// User-added (also encrypted)
	UserNotes   string            `json:"user_notes"`
	UserSecrets map[string]string `json:"user_secrets"` // tokens, creds

	// Metadata
	Confidence   float64   `json:"confidence"`    // 0-1 confidence score
	AIReasoning  string    `json:"ai_reasoning"`  // AI explanation
	DiscoveredAt time.Time `json:"discovered_at"` // First discovery
	UpdatedAt    time.Time `json:"updated_at"`    // Last update
	ScanDuration int64     `json:"scan_duration"` // Scan duration in ms

	// Fingerprint tracking for just-in-time discovery
	Fingerprint              string    `json:"fingerprint,omitempty"`                // Hash when discovery was done
	FingerprintedAt          time.Time `json:"fingerprinted_at,omitempty"`           // When fingerprint was captured
	FingerprintSchemaVersion int       `json:"fingerprint_schema_version,omitempty"` // Schema version when fingerprint was captured
	CLIAccessVersion         int       `json:"cli_access_version,omitempty"`         // Version of CLI access pattern format

	// Raw data for debugging/re-analysis
	RawCommandOutput map[string]string `json:"raw_command_output,omitempty"`

	// Auto-suggested web interface URL based on service type and discovered ports
	SuggestedURL             string `json:"suggested_url,omitempty"`
	SuggestedURLSourceCode   string `json:"suggested_url_source_code,omitempty"`
	SuggestedURLSourceDetail string `json:"suggested_url_source_detail,omitempty"`
	SuggestedURLDiagnostic   string `json:"suggested_url_diagnostic,omitempty"`
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

// DockerBindMount represents a Docker bind mount with source and destination paths.
// This is critical for knowing where to actually edit files - the source path on the
// host filesystem, not the destination path inside the container.
type DockerBindMount struct {
	ContainerName string `json:"container_name"`      // Docker container name
	Source        string `json:"source"`              // Host path (where to actually write files)
	Destination   string `json:"destination"`         // Container path (what the service sees)
	Type          string `json:"type,omitempty"`      // Mount type: bind, volume, tmpfs
	ReadOnly      bool   `json:"read_only,omitempty"` // Whether mount is read-only
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
	DiscoveryStatusRunning   DiscoveryStatus = "running"
	DiscoveryStatusCompleted DiscoveryStatus = "completed"
)

// DiscoveryProgress represents the progress of an ongoing discovery.
type DiscoveryProgress struct {
	ResourceID      string          `json:"resource_id"`
	Status          DiscoveryStatus `json:"status"`
	CurrentStep     string          `json:"current_step"`
	CurrentCommand  string          `json:"current_command,omitempty"`
	TotalSteps      int             `json:"total_steps"`
	CompletedSteps  int             `json:"completed_steps"`
	ElapsedMs       int64           `json:"elapsed_ms,omitempty"`
	PercentComplete float64         `json:"percent_complete,omitempty"`
	StartedAt       time.Time       `json:"started_at"`
	Error           string          `json:"error,omitempty"`
}

// AIProviderInfo describes the AI provider being used for discovery analysis.
type AIProviderInfo struct {
	Provider string `json:"provider"` // e.g., "anthropic", "openai", "ollama"
	Model    string `json:"model"`    // e.g., "claude-haiku-4-5", "gpt-4o"
	IsLocal  bool   `json:"is_local"` // true for ollama (local models)
	Label    string `json:"label"`    // Human-readable label, e.g., "Local (Ollama)" or "Cloud (Anthropic)"
}

// DiscoveryInfo provides metadata about the discovery system configuration.
type DiscoveryInfo struct {
	AIProvider        *AIProviderInfo    `json:"ai_provider,omitempty"`        // Current AI provider info
	Commands          []DiscoveryCommand `json:"commands,omitempty"`           // Commands that will be run
	CommandCategories []string           `json:"command_categories,omitempty"` // Unique categories of commands
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
	Fingerprint    string          `json:"fingerprint,omitempty"` // Current fingerprint
	NeedsDiscovery bool            `json:"needs_discovery"`       // True if fingerprint changed
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
		Fingerprint:    d.Fingerprint,
		NeedsDiscovery: false, // Will be set by caller if fingerprint changed
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

// ServiceStatus is a typed snapshot of service runtime status.
type ServiceStatus struct {
	Running             bool
	LastRun             time.Time
	Interval            string
	CacheSize           int
	AIAnalyzerSet       bool
	ScannerSet          bool
	StoreSet            bool
	DeepScanTimeout     string
	AIAnalysisTimeout   string
	MaxDiscoveryAge     string
	FingerprintCount    int
	LastFingerprintScan time.Time
}

// ToMap converts the status snapshot to a legacy map representation.
func (s ServiceStatus) ToMap() map[string]any {
	return map[string]any{
		"running":               s.Running,
		"last_run":              s.LastRun,
		"interval":              s.Interval,
		"cache_size":            s.CacheSize,
		"ai_analyzer_set":       s.AIAnalyzerSet,
		"scanner_set":           s.ScannerSet,
		"store_set":             s.StoreSet,
		"deep_scan_timeout":     s.DeepScanTimeout,
		"ai_analysis_timeout":   s.AIAnalysisTimeout,
		"max_discovery_age":     s.MaxDiscoveryAge,
		"fingerprint_count":     s.FingerprintCount,
		"last_fingerprint_scan": s.LastFingerprintScan,
	}
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
	LogPaths       []string        `json:"log_paths"`
	Ports          []PortInfo      `json:"ports"`
	Confidence     float64         `json:"confidence"`
	Reasoning      string          `json:"reasoning"`
}

// ContainerFingerprint captures the key metadata that indicates a container changed.
// This is used for just-in-time discovery - only running discovery when something
// actually changed rather than on a fixed timer.
// FingerprintSchemaVersion is incremented when the fingerprint algorithm changes.
// This prevents mass rediscovery when we add new fields to the fingerprint hash.
// Old fingerprints with different schema versions are treated as "schema changed"
// rather than "container changed", allowing for more controlled migration.
const FingerprintSchemaVersion = 3 // v3: Removed IP addresses (DHCP churn caused false positives)

// CLIAccessVersion is incremented when the CLI access pattern format changes.
// When a discovery has an older version, its CLIAccess field is regenerated
// to use the new instructional format.
const CLIAccessVersion = 2 // v2: Changed from shell commands to pulse_control instructions

type ContainerFingerprint struct {
	ResourceID    string    `json:"resource_id"`
	HostID        string    `json:"host_id"`
	Hash          string    `json:"hash"`           // SHA256 of metadata (truncated to 16 chars)
	SchemaVersion int       `json:"schema_version"` // Version of fingerprint algorithm
	GeneratedAt   time.Time `json:"generated_at"`

	// Components that went into the hash (for debugging)
	ImageID    string   `json:"image_id,omitempty"`
	ImageName  string   `json:"image_name,omitempty"`
	Ports      []string `json:"ports,omitempty"`
	MountPaths []string `json:"mount_paths,omitempty"`
	EnvKeys    []string `json:"env_keys,omitempty"`   // Keys only, not values (security)
	CreatedAt  string   `json:"created_at,omitempty"` // Container creation time
}

// IsSchemaOutdated returns true if this fingerprint was created with an older schema.
func (fp *ContainerFingerprint) IsSchemaOutdated() bool {
	return fp.SchemaVersion < FingerprintSchemaVersion
}
