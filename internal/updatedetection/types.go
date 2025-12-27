// Package updatedetection provides unified update detection across all Pulse-managed
// infrastructure types: Docker containers, LXC, VMs, Proxmox hosts, and Kubernetes.
package updatedetection

import "time"

// UpdateType represents the category of update
type UpdateType string

const (
	UpdateTypeDockerImage     UpdateType = "docker_image"
	UpdateTypePackage         UpdateType = "package" // apt, yum, apk
	UpdateTypeProxmox         UpdateType = "proxmox" // pve/pbs specific
	UpdateTypeKubernetesImage UpdateType = "k8s_image"
	UpdateTypeHelmChart       UpdateType = "helm_chart" // future
)

// UpdateSeverity indicates how critical the update is
type UpdateSeverity string

const (
	SeverityUnknown  UpdateSeverity = "unknown"
	SeveritySecurity UpdateSeverity = "security"
	SeverityBugfix   UpdateSeverity = "bugfix"
	SeverityFeature  UpdateSeverity = "feature"
)

// UpdateInfo represents a single available update
type UpdateInfo struct {
	// Core identification
	ID           string `json:"id"`           // Unique ID for this update
	ResourceID   string `json:"resourceId"`   // Pulse resource ID (e.g., docker container ID)
	ResourceType string `json:"resourceType"` // "docker", "lxc", "vm", "node", "k8s_pod"
	ResourceName string `json:"resourceName"` // Human-readable name
	HostID       string `json:"hostId"`       // Which host/node owns this resource

	// Update specifics
	Type     UpdateType     `json:"type"`
	Severity UpdateSeverity `json:"severity,omitempty"`

	// Version/Image info
	CurrentVersion string `json:"currentVersion,omitempty"` // e.g., "1.2.3" or image:tag
	LatestVersion  string `json:"latestVersion,omitempty"`  // e.g., "1.2.4" or new digest
	CurrentDigest  string `json:"currentDigest,omitempty"`  // For container images
	LatestDigest   string `json:"latestDigest,omitempty"`

	// Package-specific (for apt/yum updates)
	PackageName   string `json:"packageName,omitempty"`
	PackageCount  int    `json:"packageCount,omitempty"`  // For summary: "15 packages"
	SecurityCount int    `json:"securityCount,omitempty"` // Security-only count

	// Timing
	FirstDetected time.Time `json:"firstDetected"`
	LastChecked   time.Time `json:"lastChecked"`

	// Additional metadata
	ChangelogURL string                 `json:"changelogUrl,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	Error        string                 `json:"error,omitempty"` // If check failed
}

// UpdateSummary provides aggregated update stats for a host
type UpdateSummary struct {
	HostID           string    `json:"hostId"`
	HostName         string    `json:"hostName"`
	TotalUpdates     int       `json:"totalUpdates"`
	SecurityUpdates  int       `json:"securityUpdates"`
	ContainerUpdates int       `json:"containerUpdates"`
	PackageUpdates   int       `json:"packageUpdates"`
	LastChecked      time.Time `json:"lastChecked"`
}

// ImageUpdateInfo contains the result of a container image update check
type ImageUpdateInfo struct {
	Image           string    `json:"image"`
	CurrentDigest   string    `json:"currentDigest"`
	LatestDigest    string    `json:"latestDigest"`
	UpdateAvailable bool      `json:"updateAvailable"`
	CheckedAt       time.Time `json:"checkedAt"`
	Error           string    `json:"error,omitempty"` // e.g., "rate limited", "auth required"
}

// ContainerUpdateStatus is included in container reports from the Docker agent
type ContainerUpdateStatus struct {
	UpdateAvailable bool      `json:"updateAvailable"`
	CurrentDigest   string    `json:"currentDigest,omitempty"`
	LatestDigest    string    `json:"latestDigest,omitempty"`
	LastChecked     time.Time `json:"lastChecked"`
	Error           string    `json:"error,omitempty"` // e.g., "rate limited", "auth required"
}
