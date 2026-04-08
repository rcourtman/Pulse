// Package hypervisor defines the common abstraction layer for hypervisors and cloud providers.
// All provider implementations map their native types to these unified types so that the
// monitoring engine, WebSocket broadcast, and frontend can treat resources uniformly.
package hypervisor

import "time"

// ProviderType identifies the hypervisor or cloud platform.
type ProviderType string

const (
	ProviderProxmox ProviderType = "proxmox"
	ProviderVMware  ProviderType = "vmware"
	ProviderLibvirt ProviderType = "libvirt"
	ProviderNutanix ProviderType = "nutanix"
	ProviderAWS     ProviderType = "aws"
	ProviderAzure   ProviderType = "azure"
	ProviderGCP     ProviderType = "gcp"
	ProviderHyperV  ProviderType = "hyperv"
)

// ConsoleType identifies a remote console protocol.
type ConsoleType string

const (
	ConsoleVNC    ConsoleType = "vnc"
	ConsoleSPICE  ConsoleType = "spice"
	ConsoleSSH    ConsoleType = "ssh"
	ConsoleSerial ConsoleType = "serial"
	ConsoleRDP    ConsoleType = "rdp"
)

// ResourceUsage tracks usage vs total for a measurable resource (memory, disk, etc.).
type ResourceUsage struct {
	Used  uint64  `json:"used"`
	Total uint64  `json:"total"`
	Free  uint64  `json:"free"`
	Pct   float64 `json:"pct"` // Percentage used (0-100)
}

// CPUInfo describes processor hardware.
type CPUInfo struct {
	Model   string `json:"model,omitempty"`
	Sockets int    `json:"sockets,omitempty"`
	Cores   int    `json:"cores,omitempty"`
	Threads int    `json:"threads,omitempty"`
}

// Node represents a physical host or cluster node.
type Node struct {
	ID               string        `json:"id"`
	Name             string        `json:"name"`
	DisplayName      string        `json:"displayName,omitempty"`
	ProviderType     ProviderType  `json:"providerType"`
	ProviderID       string        `json:"providerId"`
	ProviderName     string        `json:"providerName,omitempty"`
	Status           string        `json:"status"` // "online", "offline", "maintenance"
	CPU              float64       `json:"cpu"`     // Usage fraction 0.0-1.0
	Memory           ResourceUsage `json:"memory"`
	Disk             ResourceUsage `json:"disk"`
	Uptime           int64         `json:"uptime"` // Seconds
	KernelVersion    string        `json:"kernelVersion,omitempty"`
	HypervisorVersion string      `json:"hypervisorVersion,omitempty"`
	CPUInfo          CPUInfo       `json:"cpuInfo,omitempty"`
	LoadAverage      []float64     `json:"loadAverage,omitempty"`
	LastSeen         time.Time     `json:"lastSeen"`
}

// VMPowerState represents the power state of a virtual machine.
type VMPowerState string

const (
	VMRunning    VMPowerState = "running"
	VMStopped    VMPowerState = "stopped"
	VMPaused     VMPowerState = "paused"
	VMSuspended  VMPowerState = "suspended"
	VMMigrating  VMPowerState = "migrating"
	VMUnknown    VMPowerState = "unknown"
)

// VM represents a virtual machine on any hypervisor.
type VM struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	NodeID        string        `json:"nodeId"`
	NodeName      string        `json:"nodeName,omitempty"`
	ProviderType  ProviderType  `json:"providerType"`
	ProviderID    string        `json:"providerId"`
	ProviderName  string        `json:"providerName,omitempty"`
	Status        string        `json:"status"`
	PowerState    VMPowerState  `json:"powerState"`
	CPU           float64       `json:"cpu"`
	CPUs          int           `json:"cpus"`
	Memory        ResourceUsage `json:"memory"`
	Disk          ResourceUsage `json:"disk"`
	IPAddresses   []string      `json:"ipAddresses,omitempty"`
	OSName        string        `json:"osName,omitempty"`
	OSVersion     string        `json:"osVersion,omitempty"`
	GuestToolsOK  bool          `json:"guestToolsOk,omitempty"`
	NetworkIn     int64         `json:"networkIn"`
	NetworkOut    int64         `json:"networkOut"`
	DiskRead      int64         `json:"diskRead"`
	DiskWrite     int64         `json:"diskWrite"`
	Uptime        int64         `json:"uptime"`
	Template      bool          `json:"template"`
	Tags          []string      `json:"tags,omitempty"`
	LastSeen      time.Time     `json:"lastSeen"`
	// ConsoleTypes lists the console protocols available for this VM.
	ConsoleTypes  []ConsoleType `json:"consoleTypes,omitempty"`
}

// Container represents a container (LXC, Docker, OCI, etc.) on any platform.
type Container struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	NodeID       string        `json:"nodeId"`
	NodeName     string        `json:"nodeName,omitempty"`
	ProviderType ProviderType  `json:"providerType"`
	ProviderID   string        `json:"providerId"`
	ProviderName string        `json:"providerName,omitempty"`
	Status       string        `json:"status"`
	CPU          float64       `json:"cpu"`
	CPUs         int           `json:"cpus"`
	Memory       ResourceUsage `json:"memory"`
	Disk         ResourceUsage `json:"disk"`
	NetworkIn    int64         `json:"networkIn"`
	NetworkOut   int64         `json:"networkOut"`
	Uptime       int64         `json:"uptime"`
	Image        string        `json:"image,omitempty"`
	Tags         []string      `json:"tags,omitempty"`
	LastSeen     time.Time     `json:"lastSeen"`
}

// Storage represents a storage resource (datastore, pool, volume, etc.).
type Storage struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	NodeID       string        `json:"nodeId,omitempty"`
	ProviderType ProviderType  `json:"providerType"`
	ProviderID   string        `json:"providerId"`
	Type         string        `json:"type"` // "nfs", "iscsi", "local", "ceph", "vmfs", "ebs", etc.
	Status       string        `json:"status"`
	Usage        ResourceUsage `json:"usage"`
	Shared       bool          `json:"shared"`
}

// Snapshot represents a VM snapshot.
type Snapshot struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Created     time.Time `json:"created"`
	Parent      string    `json:"parent,omitempty"`
	Current     bool      `json:"current"`
	SizeBytes   int64     `json:"sizeBytes,omitempty"`
}

// BackupJob represents a configured backup job.
type BackupJob struct {
	ID           string    `json:"id"`
	Name         string    `json:"name,omitempty"`
	Schedule     string    `json:"schedule,omitempty"`
	TargetVMs    []string  `json:"targetVMs,omitempty"`
	LastRun      time.Time `json:"lastRun,omitempty"`
	LastStatus   string    `json:"lastStatus,omitempty"`
	NextRun      time.Time `json:"nextRun,omitempty"`
	ProviderType ProviderType `json:"providerType"`
	ProviderID   string    `json:"providerId"`
}

// Backup represents a single backup artifact.
type Backup struct {
	ID           string    `json:"id"`
	VMID         string    `json:"vmId"`
	VMName       string    `json:"vmName,omitempty"`
	Type         string    `json:"type"` // "full", "incremental", "differential"
	SizeBytes    int64     `json:"sizeBytes"`
	Created      time.Time `json:"created"`
	Status       string    `json:"status"`
	ProviderType ProviderType `json:"providerType"`
	ProviderID   string    `json:"providerId"`
}

// ConsoleTicket contains the information needed to establish a console session.
type ConsoleTicket struct {
	Type     ConsoleType `json:"type"`
	URL      string      `json:"url,omitempty"`      // WebSocket or direct URL
	Token    string      `json:"token,omitempty"`     // Authentication token/ticket
	Password string      `json:"password,omitempty"`  // VNC password if applicable
	Host     string      `json:"host,omitempty"`      // Target host for the connection
	Port     int         `json:"port,omitempty"`      // Target port
	VMID     string      `json:"vmId"`
	NodeID   string      `json:"nodeId,omitempty"`
}

// ProviderHealth captures the connection status of a provider.
type ProviderHealth struct {
	ProviderID   string       `json:"providerId"`
	ProviderName string       `json:"providerName"`
	ProviderType ProviderType `json:"providerType"`
	Connected    bool         `json:"connected"`
	LastCheck    time.Time    `json:"lastCheck"`
	Error        string       `json:"error,omitempty"`
	NodeCount    int          `json:"nodeCount"`
	VMCount      int          `json:"vmCount"`
}
