package models

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// State represents the current state of all monitored resources
type State struct {
	mu                           sync.RWMutex
	Nodes                        []Node                     `json:"nodes"`
	VMs                          []VM                       `json:"vms"`
	Containers                   []Container                `json:"containers"`
	DockerHosts                  []DockerHost               `json:"dockerHosts"`
	RemovedDockerHosts           []RemovedDockerHost        `json:"removedDockerHosts"`
	KubernetesClusters           []KubernetesCluster        `json:"kubernetesClusters"`
	RemovedKubernetesClusters    []RemovedKubernetesCluster `json:"removedKubernetesClusters"`
	Hosts                        []Host                     `json:"hosts"`
	Storage                      []Storage                  `json:"storage"`
	CephClusters                 []CephCluster              `json:"cephClusters"`
	PhysicalDisks                []PhysicalDisk             `json:"physicalDisks"`
	PBSInstances                 []PBSInstance              `json:"pbs"`
	PMGInstances                 []PMGInstance              `json:"pmg"`
	PBSBackups                   []PBSBackup                `json:"pbsBackups"`
	PMGBackups                   []PMGBackup                `json:"pmgBackups"`
	Backups                      Backups                    `json:"backups"`
	ReplicationJobs              []ReplicationJob           `json:"replicationJobs"`
	Metrics                      []Metric                   `json:"metrics"`
	PVEBackups                   PVEBackups                 `json:"pveBackups"`
	Performance                  Performance                `json:"performance"`
	ConnectionHealth             map[string]bool            `json:"connectionHealth"`
	Stats                        Stats                      `json:"stats"`
	ActiveAlerts                 []Alert                    `json:"activeAlerts"`
	RecentlyResolved             []ResolvedAlert            `json:"recentlyResolved"`
	LastUpdate                   time.Time                  `json:"lastUpdate"`
	TemperatureMonitoringEnabled bool                       `json:"temperatureMonitoringEnabled"`
}

// Alert represents an active alert (simplified for State)
type Alert struct {
	ID              string     `json:"id"`
	Type            string     `json:"type"`
	Level           string     `json:"level"`
	ResourceID      string     `json:"resourceId"`
	ResourceName    string     `json:"resourceName"`
	Node            string     `json:"node"`
	NodeDisplayName string     `json:"nodeDisplayName,omitempty"`
	Instance        string     `json:"instance"`
	Message         string     `json:"message"`
	Value           float64    `json:"value"`
	Threshold       float64    `json:"threshold"`
	StartTime       time.Time  `json:"startTime"`
	Acknowledged    bool       `json:"acknowledged"`
	AckTime         *time.Time `json:"ackTime,omitempty"`
	AckUser         string     `json:"ackUser,omitempty"`
}

// ResolvedAlert represents a recently resolved alert
type ResolvedAlert struct {
	Alert
	ResolvedTime time.Time `json:"resolvedTime"`
}

// Node represents a Proxmox VE node
type Node struct {
	ID                           string       `json:"id"`
	Name                         string       `json:"name"`
	DisplayName                  string       `json:"displayName,omitempty"`
	Instance                     string       `json:"instance"`
	Host                         string       `json:"host"`     // Full host URL from config
	GuestURL                     string       `json:"guestURL"` // Optional guest-accessible URL (for navigation)
	Status                       string       `json:"status"`
	Type                         string       `json:"type"`
	CPU                          float64      `json:"cpu"`
	Memory                       Memory       `json:"memory"`
	Disk                         Disk         `json:"disk"`
	Uptime                       int64        `json:"uptime"`
	LoadAverage                  []float64    `json:"loadAverage"`
	KernelVersion                string       `json:"kernelVersion"`
	PVEVersion                   string       `json:"pveVersion"`
	CPUInfo                      CPUInfo      `json:"cpuInfo"`
	Temperature                  *Temperature `json:"temperature,omitempty"`                  // CPU/NVMe temperatures
	TemperatureMonitoringEnabled *bool        `json:"temperatureMonitoringEnabled,omitempty"` // Per-node temperature monitoring override
	LastSeen                     time.Time    `json:"lastSeen"`
	ConnectionHealth             string       `json:"connectionHealth"`
	IsClusterMember              bool         `json:"isClusterMember"` // True if part of a cluster
	ClusterName                  string       `json:"clusterName"`     // Name of cluster (empty if standalone)

	// Package updates - polled less frequently (every 30 mins)
	PendingUpdates          int       `json:"pendingUpdates"`                    // Number of pending apt updates
	PendingUpdatesCheckedAt time.Time `json:"pendingUpdatesCheckedAt,omitempty"` // When updates were last checked

	// Linking: When a host agent is running on this PVE node, link them together
	LinkedHostAgentID string `json:"linkedHostAgentId,omitempty"` // ID of the host agent running on this node
}

// VM represents a virtual machine
type VM struct {
	ID                string                  `json:"id"`
	VMID              int                     `json:"vmid"`
	Name              string                  `json:"name"`
	Node              string                  `json:"node"`
	Instance          string                  `json:"instance"`
	Status            string                  `json:"status"`
	Type              string                  `json:"type"`
	CPU               float64                 `json:"cpu"`
	CPUs              int                     `json:"cpus"`
	Memory            Memory                  `json:"memory"`
	Disk              Disk                    `json:"disk"`
	Disks             []Disk                  `json:"disks,omitempty"`
	DiskStatusReason  string                  `json:"diskStatusReason,omitempty"` // Why disk stats are unavailable
	IPAddresses       []string                `json:"ipAddresses,omitempty"`
	OSName            string                  `json:"osName,omitempty"`
	OSVersion         string                  `json:"osVersion,omitempty"`
	AgentVersion      string                  `json:"agentVersion,omitempty"`
	NetworkInterfaces []GuestNetworkInterface `json:"networkInterfaces,omitempty"`
	NetworkIn         int64                   `json:"networkIn"`
	NetworkOut        int64                   `json:"networkOut"`
	DiskRead          int64                   `json:"diskRead"`
	DiskWrite         int64                   `json:"diskWrite"`
	Uptime            int64                   `json:"uptime"`
	Template          bool                    `json:"template"`
	LastBackup        time.Time               `json:"lastBackup,omitempty"`
	Tags              []string                `json:"tags,omitempty"`
	Lock              string                  `json:"lock,omitempty"`
	LastSeen          time.Time               `json:"lastSeen"`
}

// Container represents an LXC container
type Container struct {
	ID                string                  `json:"id"`
	VMID              int                     `json:"vmid"`
	Name              string                  `json:"name"`
	Node              string                  `json:"node"`
	Instance          string                  `json:"instance"`
	Status            string                  `json:"status"`
	Type              string                  `json:"type"`
	CPU               float64                 `json:"cpu"`
	CPUs              int                     `json:"cpus"`
	Memory            Memory                  `json:"memory"`
	Disk              Disk                    `json:"disk"`
	Disks             []Disk                  `json:"disks,omitempty"`
	NetworkIn         int64                   `json:"networkIn"`
	NetworkOut        int64                   `json:"networkOut"`
	DiskRead          int64                   `json:"diskRead"`
	DiskWrite         int64                   `json:"diskWrite"`
	Uptime            int64                   `json:"uptime"`
	Template          bool                    `json:"template"`
	LastBackup        time.Time               `json:"lastBackup,omitempty"`
	Tags              []string                `json:"tags,omitempty"`
	Lock              string                  `json:"lock,omitempty"`
	LastSeen          time.Time               `json:"lastSeen"`
	IPAddresses       []string                `json:"ipAddresses,omitempty"`
	NetworkInterfaces []GuestNetworkInterface `json:"networkInterfaces,omitempty"`
	OSName            string                  `json:"osName,omitempty"`
	// OCI container support (Proxmox VE 9.1+)
	IsOCI      bool   `json:"isOci,omitempty"`      // True if this is an OCI container
	OSTemplate string `json:"osTemplate,omitempty"` // Template or OCI image used (e.g., "docker:alpine:latest")

	// Docker detection - automatically detected by checking for Docker socket inside the container
	HasDocker       bool      `json:"hasDocker,omitempty"`       // True if Docker is installed inside this LXC
	DockerCheckedAt time.Time `json:"dockerCheckedAt,omitempty"` // When Docker presence was last checked
}

// Host represents a generic infrastructure host reporting via external agents.
type Host struct {
	ID                string                 `json:"id"`
	Hostname          string                 `json:"hostname"`
	DisplayName       string                 `json:"displayName,omitempty"`
	Platform          string                 `json:"platform,omitempty"`
	OSName            string                 `json:"osName,omitempty"`
	OSVersion         string                 `json:"osVersion,omitempty"`
	KernelVersion     string                 `json:"kernelVersion,omitempty"`
	Architecture      string                 `json:"architecture,omitempty"`
	CPUCount          int                    `json:"cpuCount,omitempty"`
	CPUUsage          float64                `json:"cpuUsage,omitempty"`
	Memory            Memory                 `json:"memory"`
	LoadAverage       []float64              `json:"loadAverage,omitempty"`
	Disks             []Disk                 `json:"disks,omitempty"`
	DiskIO            []DiskIO               `json:"diskIO,omitempty"`
	NetworkInterfaces []HostNetworkInterface `json:"networkInterfaces,omitempty"`
	Sensors           HostSensorSummary      `json:"sensors,omitempty"`
	RAID              []HostRAIDArray        `json:"raid,omitempty"`
	Ceph              *HostCephCluster       `json:"ceph,omitempty"`
	Status            string                 `json:"status"`
	UptimeSeconds     int64                  `json:"uptimeSeconds,omitempty"`
	IntervalSeconds   int                    `json:"intervalSeconds,omitempty"`
	LastSeen          time.Time              `json:"lastSeen"`
	AgentVersion      string                 `json:"agentVersion,omitempty"`
	MachineID         string                 `json:"machineId,omitempty"`
	CommandsEnabled   bool                   `json:"commandsEnabled,omitempty"` // Whether AI command execution is enabled
	ReportIP          string                 `json:"reportIp,omitempty"`        // User-specified IP for multi-NIC systems
	TokenID           string                 `json:"tokenId,omitempty"`
	TokenName         string                 `json:"tokenName,omitempty"`
	TokenHint         string                 `json:"tokenHint,omitempty"`
	TokenLastUsedAt   *time.Time             `json:"tokenLastUsedAt,omitempty"`
	Tags              []string               `json:"tags,omitempty"`
	IsLegacy          bool                   `json:"isLegacy,omitempty"`

	// Computed I/O rates (bytes/sec), populated from cumulative counters by rate tracker
	NetInRate     float64 `json:"netInRate,omitempty"`
	NetOutRate    float64 `json:"netOutRate,omitempty"`
	DiskReadRate  float64 `json:"diskReadRate,omitempty"`
	DiskWriteRate float64 `json:"diskWriteRate,omitempty"`

	// Linking: When this host agent is running on a known PVE node/VM/container
	LinkedNodeID      string `json:"linkedNodeId,omitempty"`      // ID of the PVE node this agent is running on
	LinkedVMID        string `json:"linkedVmId,omitempty"`        // ID of the VM this agent is running inside
	LinkedContainerID string `json:"linkedContainerId,omitempty"` // ID of the container this agent is running inside
}

// HostNetworkInterface describes a host network adapter summary.
type HostNetworkInterface struct {
	Name      string   `json:"name"`
	MAC       string   `json:"mac,omitempty"`
	Addresses []string `json:"addresses,omitempty"`
	RXBytes   uint64   `json:"rxBytes,omitempty"`
	TXBytes   uint64   `json:"txBytes,omitempty"`
	SpeedMbps *int64   `json:"speedMbps,omitempty"`
}

// HostSensorSummary captures optional per-host sensor readings.
type HostSensorSummary struct {
	TemperatureCelsius map[string]float64 `json:"temperatureCelsius,omitempty"`
	FanRPM             map[string]float64 `json:"fanRpm,omitempty"`
	Additional         map[string]float64 `json:"additional,omitempty"`
	SMART              []HostDiskSMART    `json:"smart,omitempty"` // S.M.A.R.T. disk data
}

// HostDiskSMART represents S.M.A.R.T. data for a disk from a host agent.
type HostDiskSMART struct {
	Device      string           `json:"device"`            // Device name (e.g., sda)
	Model       string           `json:"model,omitempty"`   // Disk model
	Serial      string           `json:"serial,omitempty"`  // Serial number
	WWN         string           `json:"wwn,omitempty"`     // World Wide Name
	Type        string           `json:"type,omitempty"`    // Transport type: sata, sas, nvme
	Temperature int              `json:"temperature"`       // Temperature in Celsius
	Health      string           `json:"health,omitempty"`  // PASSED, FAILED, UNKNOWN
	Standby     bool             `json:"standby,omitempty"` // True if disk was in standby
	Attributes  *SMARTAttributes `json:"attributes,omitempty"`
}

// SMARTAttributes holds normalized SMART attributes for both SATA and NVMe disks.
type SMARTAttributes struct {
	PowerOnHours         *int64 `json:"powerOnHours,omitempty"`
	PowerCycles          *int64 `json:"powerCycles,omitempty"`
	ReallocatedSectors   *int64 `json:"reallocatedSectors,omitempty"`
	PendingSectors       *int64 `json:"pendingSectors,omitempty"`
	OfflineUncorrectable *int64 `json:"offlineUncorrectable,omitempty"`
	UDMACRCErrors        *int64 `json:"udmaCrcErrors,omitempty"`
	PercentageUsed       *int   `json:"percentageUsed,omitempty"`
	AvailableSpare       *int   `json:"availableSpare,omitempty"`
	MediaErrors          *int64 `json:"mediaErrors,omitempty"`
	UnsafeShutdowns      *int64 `json:"unsafeShutdowns,omitempty"`
}

// HostRAIDArray represents an mdadm RAID array on a host.
type HostRAIDArray struct {
	Device         string           `json:"device"`
	Name           string           `json:"name,omitempty"`
	Level          string           `json:"level"`
	State          string           `json:"state"`
	TotalDevices   int              `json:"totalDevices"`
	ActiveDevices  int              `json:"activeDevices"`
	WorkingDevices int              `json:"workingDevices"`
	FailedDevices  int              `json:"failedDevices"`
	SpareDevices   int              `json:"spareDevices"`
	UUID           string           `json:"uuid,omitempty"`
	Devices        []HostRAIDDevice `json:"devices"`
	RebuildPercent float64          `json:"rebuildPercent"`
	RebuildSpeed   string           `json:"rebuildSpeed,omitempty"`
}

// HostRAIDDevice represents a device in a RAID array.
type HostRAIDDevice struct {
	Device string `json:"device"`
	State  string `json:"state"`
	Slot   int    `json:"slot"`
}

// HostCephCluster represents Ceph cluster status collected directly by the host agent.
// This is separate from CephCluster which comes from the Proxmox API.
type HostCephCluster struct {
	FSID        string             `json:"fsid"`
	Health      HostCephHealth     `json:"health"`
	MonMap      HostCephMonitorMap `json:"monMap,omitempty"`
	MgrMap      HostCephManagerMap `json:"mgrMap,omitempty"`
	OSDMap      HostCephOSDMap     `json:"osdMap"`
	PGMap       HostCephPGMap      `json:"pgMap"`
	Pools       []HostCephPool     `json:"pools,omitempty"`
	Services    []HostCephService  `json:"services,omitempty"`
	CollectedAt time.Time          `json:"collectedAt"`
}

// HostCephHealth represents Ceph cluster health status.
type HostCephHealth struct {
	Status  string                   `json:"status"` // HEALTH_OK, HEALTH_WARN, HEALTH_ERR
	Checks  map[string]HostCephCheck `json:"checks,omitempty"`
	Summary []HostCephHealthSummary  `json:"summary,omitempty"`
}

// HostCephCheck represents a health check detail.
type HostCephCheck struct {
	Severity string   `json:"severity"`
	Message  string   `json:"message,omitempty"`
	Detail   []string `json:"detail,omitempty"`
}

// HostCephHealthSummary represents a health summary message.
type HostCephHealthSummary struct {
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

// HostCephMonitorMap represents Ceph monitor information.
type HostCephMonitorMap struct {
	Epoch    int               `json:"epoch"`
	NumMons  int               `json:"numMons"`
	Monitors []HostCephMonitor `json:"monitors,omitempty"`
}

// HostCephMonitor represents a single Ceph monitor.
type HostCephMonitor struct {
	Name   string `json:"name"`
	Rank   int    `json:"rank"`
	Addr   string `json:"addr,omitempty"`
	Status string `json:"status,omitempty"`
}

// HostCephManagerMap represents Ceph manager information.
type HostCephManagerMap struct {
	Available bool   `json:"available"`
	NumMgrs   int    `json:"numMgrs"`
	ActiveMgr string `json:"activeMgr,omitempty"`
	Standbys  int    `json:"standbys"`
}

// HostCephOSDMap represents OSD status summary.
type HostCephOSDMap struct {
	Epoch   int `json:"epoch"`
	NumOSDs int `json:"numOsds"`
	NumUp   int `json:"numUp"`
	NumIn   int `json:"numIn"`
	NumDown int `json:"numDown,omitempty"`
	NumOut  int `json:"numOut,omitempty"`
}

// HostCephPGMap represents placement group statistics.
type HostCephPGMap struct {
	NumPGs           int     `json:"numPgs"`
	BytesTotal       uint64  `json:"bytesTotal"`
	BytesUsed        uint64  `json:"bytesUsed"`
	BytesAvailable   uint64  `json:"bytesAvailable"`
	DataBytes        uint64  `json:"dataBytes,omitempty"`
	UsagePercent     float64 `json:"usagePercent"`
	DegradedRatio    float64 `json:"degradedRatio,omitempty"`
	MisplacedRatio   float64 `json:"misplacedRatio,omitempty"`
	ReadBytesPerSec  uint64  `json:"readBytesPerSec,omitempty"`
	WriteBytesPerSec uint64  `json:"writeBytesPerSec,omitempty"`
	ReadOpsPerSec    uint64  `json:"readOpsPerSec,omitempty"`
	WriteOpsPerSec   uint64  `json:"writeOpsPerSec,omitempty"`
}

// HostCephPool represents a Ceph pool.
type HostCephPool struct {
	ID             int     `json:"id"`
	Name           string  `json:"name"`
	BytesUsed      uint64  `json:"bytesUsed"`
	BytesAvailable uint64  `json:"bytesAvailable"`
	Objects        uint64  `json:"objects"`
	PercentUsed    float64 `json:"percentUsed"`
}

// HostCephService represents a Ceph service summary.
type HostCephService struct {
	Type    string   `json:"type"` // mon, mgr, osd, mds, rgw
	Running int      `json:"running"`
	Total   int      `json:"total"`
	Daemons []string `json:"daemons,omitempty"`
}

// DiskIO represents I/O statistics for a block device.
// Counters are cumulative since boot.
type DiskIO struct {
	Device     string `json:"device"`
	ReadBytes  uint64 `json:"readBytes,omitempty"`
	WriteBytes uint64 `json:"writeBytes,omitempty"`
	ReadOps    uint64 `json:"readOps,omitempty"`
	WriteOps   uint64 `json:"writeOps,omitempty"`
	ReadTime   uint64 `json:"readTimeMs,omitempty"`
	WriteTime  uint64 `json:"writeTimeMs,omitempty"`
	IOTime     uint64 `json:"ioTimeMs,omitempty"`
}

// DockerHost represents a Docker host reporting metrics via the external agent.
type DockerHost struct {
	ID                string                   `json:"id"`
	AgentID           string                   `json:"agentId"`
	Hostname          string                   `json:"hostname"`
	DisplayName       string                   `json:"displayName"`
	CustomDisplayName string                   `json:"customDisplayName,omitempty"` // User-defined custom name
	MachineID         string                   `json:"machineId,omitempty"`
	OS                string                   `json:"os,omitempty"`
	KernelVersion     string                   `json:"kernelVersion,omitempty"`
	Architecture      string                   `json:"architecture,omitempty"`
	Runtime           string                   `json:"runtime,omitempty"`
	RuntimeVersion    string                   `json:"runtimeVersion,omitempty"`
	DockerVersion     string                   `json:"dockerVersion,omitempty"`
	CPUs              int                      `json:"cpus"`
	TotalMemoryBytes  int64                    `json:"totalMemoryBytes"`
	UptimeSeconds     int64                    `json:"uptimeSeconds"`
	CPUUsage          float64                  `json:"cpuUsagePercent"`
	LoadAverage       []float64                `json:"loadAverage,omitempty"`
	Memory            Memory                   `json:"memory"`
	Disks             []Disk                   `json:"disks,omitempty"`
	NetworkInterfaces []HostNetworkInterface   `json:"networkInterfaces,omitempty"`
	Status            string                   `json:"status"`
	LastSeen          time.Time                `json:"lastSeen"`
	IntervalSeconds   int                      `json:"intervalSeconds"`
	AgentVersion      string                   `json:"agentVersion,omitempty"`
	Containers        []DockerContainer        `json:"containers"`
	Services          []DockerService          `json:"services,omitempty"`
	Tasks             []DockerTask             `json:"tasks,omitempty"`
	Swarm             *DockerSwarmInfo         `json:"swarm,omitempty"`
	TokenID           string                   `json:"tokenId,omitempty"`
	TokenName         string                   `json:"tokenName,omitempty"`
	TokenHint         string                   `json:"tokenHint,omitempty"`
	TokenLastUsedAt   *time.Time               `json:"tokenLastUsedAt,omitempty"`
	Hidden            bool                     `json:"hidden"`
	PendingUninstall  bool                     `json:"pendingUninstall"`
	Command           *DockerHostCommandStatus `json:"command,omitempty"`
	IsLegacy          bool                     `json:"isLegacy,omitempty"`
}

// RemovedDockerHost tracks a docker host that was deliberately removed and blocked from reporting.
type RemovedDockerHost struct {
	ID          string    `json:"id"`
	Hostname    string    `json:"hostname,omitempty"`
	DisplayName string    `json:"displayName,omitempty"`
	RemovedAt   time.Time `json:"removedAt"`
}

// DockerContainer represents the state of a Docker container on a monitored host.
type DockerContainer struct {
	ID                  string                       `json:"id"`
	Name                string                       `json:"name"`
	Image               string                       `json:"image"`
	ImageDigest         string                       `json:"imageDigest,omitempty"` // Current image digest (sha256:...)
	State               string                       `json:"state"`
	Status              string                       `json:"status"`
	Health              string                       `json:"health,omitempty"`
	CPUPercent          float64                      `json:"cpuPercent"`
	MemoryUsage         int64                        `json:"memoryUsageBytes"`
	MemoryLimit         int64                        `json:"memoryLimitBytes"`
	MemoryPercent       float64                      `json:"memoryPercent"`
	UptimeSeconds       int64                        `json:"uptimeSeconds"`
	RestartCount        int                          `json:"restartCount"`
	ExitCode            int                          `json:"exitCode"`
	CreatedAt           time.Time                    `json:"createdAt"`
	StartedAt           *time.Time                   `json:"startedAt,omitempty"`
	FinishedAt          *time.Time                   `json:"finishedAt,omitempty"`
	Ports               []DockerContainerPort        `json:"ports,omitempty"`
	Labels              map[string]string            `json:"labels,omitempty"`
	Networks            []DockerContainerNetworkLink `json:"networks,omitempty"`
	WritableLayerBytes  int64                        `json:"writableLayerBytes,omitempty"`
	RootFilesystemBytes int64                        `json:"rootFilesystemBytes,omitempty"`
	BlockIO             *DockerContainerBlockIO      `json:"blockIo,omitempty"`
	Mounts              []DockerContainerMount       `json:"mounts,omitempty"`
	Podman              *DockerPodmanContainer       `json:"podman,omitempty"`
	UpdateStatus        *DockerContainerUpdateStatus `json:"updateStatus,omitempty"` // Image update detection status
}

// DockerContainerUpdateStatus tracks the image update status for a container.
type DockerContainerUpdateStatus struct {
	UpdateAvailable bool      `json:"updateAvailable"`
	CurrentDigest   string    `json:"currentDigest,omitempty"`
	LatestDigest    string    `json:"latestDigest,omitempty"`
	LastChecked     time.Time `json:"lastChecked"`
	Error           string    `json:"error,omitempty"` // e.g., "rate limited", "auth required"
}

// DockerPodmanContainer captures Podman-specific annotations for a container.
type DockerPodmanContainer struct {
	PodName           string `json:"podName,omitempty"`
	PodID             string `json:"podId,omitempty"`
	Infra             bool   `json:"infra,omitempty"`
	ComposeProject    string `json:"composeProject,omitempty"`
	ComposeService    string `json:"composeService,omitempty"`
	ComposeWorkdir    string `json:"composeWorkdir,omitempty"`
	ComposeConfigHash string `json:"composeConfigHash,omitempty"`
	AutoUpdatePolicy  string `json:"autoUpdatePolicy,omitempty"`
	AutoUpdateRestart string `json:"autoUpdateRestart,omitempty"`
	UserNamespace     string `json:"userNamespace,omitempty"`
}

// DockerContainerPort describes an exposed container port mapping.
type DockerContainerPort struct {
	PrivatePort int    `json:"privatePort"`
	PublicPort  int    `json:"publicPort,omitempty"`
	Protocol    string `json:"protocol"`
	IP          string `json:"ip,omitempty"`
}

// DockerContainerNetworkLink summarises container network addresses per network.
type DockerContainerNetworkLink struct {
	Name string `json:"name"`
	IPv4 string `json:"ipv4,omitempty"`
	IPv6 string `json:"ipv6,omitempty"`
}

// DockerContainerBlockIO captures aggregate block IO usage for a container.
type DockerContainerBlockIO struct {
	ReadBytes               uint64   `json:"readBytes,omitempty"`
	WriteBytes              uint64   `json:"writeBytes,omitempty"`
	ReadRateBytesPerSecond  *float64 `json:"readRateBytesPerSecond,omitempty"`
	WriteRateBytesPerSecond *float64 `json:"writeRateBytesPerSecond,omitempty"`
}

// DockerContainerMount describes a mount exposed to a container.
type DockerContainerMount struct {
	Type        string `json:"type,omitempty"`
	Source      string `json:"source,omitempty"`
	Destination string `json:"destination,omitempty"`
	Mode        string `json:"mode,omitempty"`
	RW          bool   `json:"rw"`
	Propagation string `json:"propagation,omitempty"`
	Name        string `json:"name,omitempty"`
	Driver      string `json:"driver,omitempty"`
}

// KubernetesCluster represents a Kubernetes cluster reporting telemetry via the agent.
type KubernetesCluster struct {
	ID                string    `json:"id"`
	AgentID           string    `json:"agentId"`
	Name              string    `json:"name,omitempty"`
	DisplayName       string    `json:"displayName,omitempty"`
	CustomDisplayName string    `json:"customDisplayName,omitempty"`
	Server            string    `json:"server,omitempty"`
	Context           string    `json:"context,omitempty"`
	Version           string    `json:"version,omitempty"`
	Status            string    `json:"status"`
	LastSeen          time.Time `json:"lastSeen"`
	IntervalSeconds   int       `json:"intervalSeconds"`
	AgentVersion      string    `json:"agentVersion,omitempty"`

	Nodes       []KubernetesNode       `json:"nodes,omitempty"`
	Pods        []KubernetesPod        `json:"pods,omitempty"`
	Deployments []KubernetesDeployment `json:"deployments,omitempty"`

	// Token information
	TokenID         string     `json:"tokenId,omitempty"`
	TokenName       string     `json:"tokenName,omitempty"`
	TokenHint       string     `json:"tokenHint,omitempty"`
	TokenLastUsedAt *time.Time `json:"tokenLastUsedAt,omitempty"`

	Hidden           bool `json:"hidden"`
	PendingUninstall bool `json:"pendingUninstall"`
}

// RemovedKubernetesCluster tracks a Kubernetes cluster that was deliberately removed and blocked from reporting.
type RemovedKubernetesCluster struct {
	ID          string    `json:"id"`
	Name        string    `json:"name,omitempty"`
	DisplayName string    `json:"displayName,omitempty"`
	RemovedAt   time.Time `json:"removedAt"`
}

type KubernetesNode struct {
	UID                     string   `json:"uid"`
	Name                    string   `json:"name"`
	Ready                   bool     `json:"ready"`
	Unschedulable           bool     `json:"unschedulable,omitempty"`
	KubeletVersion          string   `json:"kubeletVersion,omitempty"`
	ContainerRuntimeVersion string   `json:"containerRuntimeVersion,omitempty"`
	OSImage                 string   `json:"osImage,omitempty"`
	KernelVersion           string   `json:"kernelVersion,omitempty"`
	Architecture            string   `json:"architecture,omitempty"`
	CapacityCPU             int64    `json:"capacityCpuCores,omitempty"`
	CapacityMemoryBytes     int64    `json:"capacityMemoryBytes,omitempty"`
	CapacityPods            int64    `json:"capacityPods,omitempty"`
	AllocCPU                int64    `json:"allocatableCpuCores,omitempty"`
	AllocMemoryBytes        int64    `json:"allocatableMemoryBytes,omitempty"`
	AllocPods               int64    `json:"allocatablePods,omitempty"`
	Roles                   []string `json:"roles,omitempty"`
}

type KubernetesPod struct {
	UID        string                   `json:"uid"`
	Name       string                   `json:"name"`
	Namespace  string                   `json:"namespace"`
	NodeName   string                   `json:"nodeName,omitempty"`
	Phase      string                   `json:"phase,omitempty"`
	Reason     string                   `json:"reason,omitempty"`
	Message    string                   `json:"message,omitempty"`
	QoSClass   string                   `json:"qosClass,omitempty"`
	CreatedAt  time.Time                `json:"createdAt,omitempty"`
	StartTime  *time.Time               `json:"startTime,omitempty"`
	Restarts   int                      `json:"restarts,omitempty"`
	Labels     map[string]string        `json:"labels,omitempty"`
	OwnerKind  string                   `json:"ownerKind,omitempty"`
	OwnerName  string                   `json:"ownerName,omitempty"`
	Containers []KubernetesPodContainer `json:"containers,omitempty"`
}

type KubernetesPodContainer struct {
	Name         string `json:"name"`
	Image        string `json:"image,omitempty"`
	Ready        bool   `json:"ready"`
	RestartCount int32  `json:"restartCount,omitempty"`
	State        string `json:"state,omitempty"`
	Reason       string `json:"reason,omitempty"`
	Message      string `json:"message,omitempty"`
}

type KubernetesDeployment struct {
	UID               string            `json:"uid"`
	Name              string            `json:"name"`
	Namespace         string            `json:"namespace"`
	DesiredReplicas   int32             `json:"desiredReplicas,omitempty"`
	UpdatedReplicas   int32             `json:"updatedReplicas,omitempty"`
	ReadyReplicas     int32             `json:"readyReplicas,omitempty"`
	AvailableReplicas int32             `json:"availableReplicas,omitempty"`
	Labels            map[string]string `json:"labels,omitempty"`
}

// DockerService summarises a Docker Swarm service.
type DockerService struct {
	ID             string               `json:"id"`
	Name           string               `json:"name"`
	Stack          string               `json:"stack,omitempty"`
	Image          string               `json:"image,omitempty"`
	Mode           string               `json:"mode,omitempty"`
	DesiredTasks   int                  `json:"desiredTasks,omitempty"`
	RunningTasks   int                  `json:"runningTasks,omitempty"`
	CompletedTasks int                  `json:"completedTasks,omitempty"`
	UpdateStatus   *DockerServiceUpdate `json:"updateStatus,omitempty"`
	Labels         map[string]string    `json:"labels,omitempty"`
	EndpointPorts  []DockerServicePort  `json:"endpointPorts,omitempty"`
	CreatedAt      *time.Time           `json:"createdAt,omitempty"`
	UpdatedAt      *time.Time           `json:"updatedAt,omitempty"`
}

// DockerServicePort describes a published service port.
type DockerServicePort struct {
	Name          string `json:"name,omitempty"`
	Protocol      string `json:"protocol,omitempty"`
	TargetPort    uint32 `json:"targetPort,omitempty"`
	PublishedPort uint32 `json:"publishedPort,omitempty"`
	PublishMode   string `json:"publishMode,omitempty"`
}

// DockerServiceUpdate captures service update progress.
type DockerServiceUpdate struct {
	State       string     `json:"state,omitempty"`
	Message     string     `json:"message,omitempty"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
}

// DockerTask summarises a Swarm task.
type DockerTask struct {
	ID            string     `json:"id"`
	ServiceID     string     `json:"serviceId,omitempty"`
	ServiceName   string     `json:"serviceName,omitempty"`
	Slot          int        `json:"slot,omitempty"`
	NodeID        string     `json:"nodeId,omitempty"`
	NodeName      string     `json:"nodeName,omitempty"`
	DesiredState  string     `json:"desiredState,omitempty"`
	CurrentState  string     `json:"currentState,omitempty"`
	Error         string     `json:"error,omitempty"`
	Message       string     `json:"message,omitempty"`
	ContainerID   string     `json:"containerId,omitempty"`
	ContainerName string     `json:"containerName,omitempty"`
	CreatedAt     time.Time  `json:"createdAt,omitempty"`
	UpdatedAt     *time.Time `json:"updatedAt,omitempty"`
	StartedAt     *time.Time `json:"startedAt,omitempty"`
	CompletedAt   *time.Time `json:"completedAt,omitempty"`
}

// DockerSwarmInfo captures node-level swarm metadata.
type DockerSwarmInfo struct {
	NodeID           string `json:"nodeId,omitempty"`
	NodeRole         string `json:"nodeRole,omitempty"`
	LocalState       string `json:"localState,omitempty"`
	ControlAvailable bool   `json:"controlAvailable,omitempty"`
	ClusterID        string `json:"clusterId,omitempty"`
	ClusterName      string `json:"clusterName,omitempty"`
	Scope            string `json:"scope,omitempty"`
	Error            string `json:"error,omitempty"`
}

// DockerHostCommandStatus tracks the lifecycle of a control command issued to a Docker host.
type DockerHostCommandStatus struct {
	ID             string     `json:"id"`
	Type           string     `json:"type"`
	Status         string     `json:"status"`
	Message        string     `json:"message,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
	DispatchedAt   *time.Time `json:"dispatchedAt,omitempty"`
	AcknowledgedAt *time.Time `json:"acknowledgedAt,omitempty"`
	CompletedAt    *time.Time `json:"completedAt,omitempty"`
	FailedAt       *time.Time `json:"failedAt,omitempty"`
	FailureReason  string     `json:"failureReason,omitempty"`
	ExpiresAt      *time.Time `json:"expiresAt,omitempty"`
}

// Storage represents a storage resource
type Storage struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Node      string   `json:"node"`
	Instance  string   `json:"instance"`
	Nodes     []string `json:"nodes,omitempty"`
	NodeIDs   []string `json:"nodeIds,omitempty"`
	NodeCount int      `json:"nodeCount,omitempty"`
	Type      string   `json:"type"`
	Status    string   `json:"status"`
	Path      string   `json:"path,omitempty"`
	Total     int64    `json:"total"`
	Used      int64    `json:"used"`
	Free      int64    `json:"free"`
	Usage     float64  `json:"usage"`
	Content   string   `json:"content"`
	Shared    bool     `json:"shared"`
	Enabled   bool     `json:"enabled"`
	Active    bool     `json:"active"`
	ZFSPool   *ZFSPool `json:"zfsPool,omitempty"` // ZFS pool details if this is ZFS storage
}

// ZFSPool represents a ZFS pool with health and error information
type ZFSPool struct {
	Name           string      `json:"name"`
	State          string      `json:"state"`  // ONLINE, DEGRADED, FAULTED, OFFLINE, REMOVED, UNAVAIL
	Status         string      `json:"status"` // Healthy, Degraded, Faulted, etc.
	Scan           string      `json:"scan"`   // Current scan status (scrub, resilver, none)
	ReadErrors     int64       `json:"readErrors"`
	WriteErrors    int64       `json:"writeErrors"`
	ChecksumErrors int64       `json:"checksumErrors"`
	Devices        []ZFSDevice `json:"devices"`
}

// ZFSDevice represents a device in a ZFS pool
type ZFSDevice struct {
	Name           string `json:"name"`
	Type           string `json:"type"`  // disk, mirror, raidz, raidz2, raidz3, spare, log, cache
	State          string `json:"state"` // ONLINE, DEGRADED, FAULTED, OFFLINE, REMOVED, UNAVAIL
	ReadErrors     int64  `json:"readErrors"`
	WriteErrors    int64  `json:"writeErrors"`
	ChecksumErrors int64  `json:"checksumErrors"`
	Message        string `json:"message,omitempty"` // Additional message provided by Proxmox (if any)
}

// CephCluster represents the health and capacity information for a Ceph cluster
type CephCluster struct {
	ID             string              `json:"id"`
	Instance       string              `json:"instance"`
	Name           string              `json:"name"`
	FSID           string              `json:"fsid,omitempty"`
	Health         string              `json:"health"`
	HealthMessage  string              `json:"healthMessage,omitempty"`
	TotalBytes     int64               `json:"totalBytes"`
	UsedBytes      int64               `json:"usedBytes"`
	AvailableBytes int64               `json:"availableBytes"`
	UsagePercent   float64             `json:"usagePercent"`
	NumMons        int                 `json:"numMons"`
	NumMgrs        int                 `json:"numMgrs"`
	NumOSDs        int                 `json:"numOsds"`
	NumOSDsUp      int                 `json:"numOsdsUp"`
	NumOSDsIn      int                 `json:"numOsdsIn"`
	NumPGs         int                 `json:"numPGs"`
	Pools          []CephPool          `json:"pools,omitempty"`
	Services       []CephServiceStatus `json:"services,omitempty"`
	LastUpdated    time.Time           `json:"lastUpdated"`
}

// CephPool represents usage statistics for a Ceph pool
type CephPool struct {
	ID             int     `json:"id"`
	Name           string  `json:"name"`
	StoredBytes    int64   `json:"storedBytes"`
	AvailableBytes int64   `json:"availableBytes"`
	Objects        int64   `json:"objects"`
	PercentUsed    float64 `json:"percentUsed"`
}

// CephServiceStatus summarises daemon health for a Ceph service type (e.g. mon, mgr)
type CephServiceStatus struct {
	Type    string `json:"type"`
	Running int    `json:"running"`
	Total   int    `json:"total"`
	Message string `json:"message,omitempty"`
}

// PhysicalDisk represents a physical disk on a node
type PhysicalDisk struct {
	ID              string           `json:"id"` // "{instance}-{node}-{devpath}"
	Node            string           `json:"node"`
	Instance        string           `json:"instance"`
	DevPath         string           `json:"devPath"` // /dev/nvme0n1, /dev/sda
	Model           string           `json:"model"`
	Serial          string           `json:"serial"`
	WWN             string           `json:"wwn"`         // World Wide Name
	Type            string           `json:"type"`        // nvme, sata, sas
	Size            int64            `json:"size"`        // bytes
	Health          string           `json:"health"`      // PASSED, FAILED, UNKNOWN
	Wearout         int              `json:"wearout"`     // SSD wear metric from Proxmox (0-100, -1 when unavailable)
	Temperature     int              `json:"temperature"` // Celsius (if available)
	RPM             int              `json:"rpm"`         // 0 for SSDs
	Used            string           `json:"used"`        // Filesystem or partition usage
	SmartAttributes *SMARTAttributes `json:"smartAttributes,omitempty"`
	LastChecked     time.Time        `json:"lastChecked"`
}

// PBSInstance represents a Proxmox Backup Server instance
type PBSInstance struct {
	ID               string          `json:"id"`
	Name             string          `json:"name"`
	Host             string          `json:"host"`
	GuestURL         string          `json:"guestURL,omitempty"` // Optional guest-accessible URL (for navigation)
	Status           string          `json:"status"`
	Version          string          `json:"version"`
	CPU              float64         `json:"cpu"`         // CPU usage percentage
	Memory           float64         `json:"memory"`      // Memory usage percentage
	MemoryUsed       int64           `json:"memoryUsed"`  // Memory used in bytes
	MemoryTotal      int64           `json:"memoryTotal"` // Total memory in bytes
	Uptime           int64           `json:"uptime"`      // Uptime in seconds
	Datastores       []PBSDatastore  `json:"datastores"`
	BackupJobs       []PBSBackupJob  `json:"backupJobs"`
	SyncJobs         []PBSSyncJob    `json:"syncJobs"`
	VerifyJobs       []PBSVerifyJob  `json:"verifyJobs"`
	PruneJobs        []PBSPruneJob   `json:"pruneJobs"`
	GarbageJobs      []PBSGarbageJob `json:"garbageJobs"`
	ConnectionHealth string          `json:"connectionHealth"`
	LastSeen         time.Time       `json:"lastSeen"`
}

// PBSDatastore represents a PBS datastore
type PBSDatastore struct {
	Name                string         `json:"name"`
	Total               int64          `json:"total"`
	Used                int64          `json:"used"`
	Free                int64          `json:"free"`
	Usage               float64        `json:"usage"`
	Status              string         `json:"status"`
	Error               string         `json:"error,omitempty"`
	Namespaces          []PBSNamespace `json:"namespaces,omitempty"`
	DeduplicationFactor float64        `json:"deduplicationFactor,omitempty"`
}

// PBSNamespace represents a PBS namespace
type PBSNamespace struct {
	Path   string `json:"path"`
	Parent string `json:"parent,omitempty"`
	Depth  int    `json:"depth"`
}

// PBSBackup represents a backup stored on PBS
type PBSBackup struct {
	ID         string    `json:"id"`       // Unique ID combining PBS instance, namespace, type, vmid, and time
	Instance   string    `json:"instance"` // PBS instance name
	Datastore  string    `json:"datastore"`
	Namespace  string    `json:"namespace"`
	BackupType string    `json:"backupType"` // "vm" or "ct"
	VMID       string    `json:"vmid"`
	BackupTime time.Time `json:"backupTime"`
	Size       int64     `json:"size"`
	Protected  bool      `json:"protected"`
	Verified   bool      `json:"verified"`
	Comment    string    `json:"comment,omitempty"`
	Files      []string  `json:"files,omitempty"`
	Owner      string    `json:"owner,omitempty"` // User who created the backup
}

// PBSBackupJob represents a PBS backup job
type PBSBackupJob struct {
	ID         string    `json:"id"`
	Store      string    `json:"store"`
	Type       string    `json:"type"`
	VMID       string    `json:"vmid,omitempty"`
	LastBackup time.Time `json:"lastBackup"`
	NextRun    time.Time `json:"nextRun,omitempty"`
	Status     string    `json:"status"`
	Error      string    `json:"error,omitempty"`
}

// PBSSyncJob represents a PBS sync job
type PBSSyncJob struct {
	ID       string    `json:"id"`
	Store    string    `json:"store"`
	Remote   string    `json:"remote"`
	Status   string    `json:"status"`
	LastSync time.Time `json:"lastSync"`
	NextRun  time.Time `json:"nextRun,omitempty"`
	Error    string    `json:"error,omitempty"`
}

// PBSVerifyJob represents a PBS verification job
type PBSVerifyJob struct {
	ID         string    `json:"id"`
	Store      string    `json:"store"`
	Status     string    `json:"status"`
	LastVerify time.Time `json:"lastVerify"`
	NextRun    time.Time `json:"nextRun,omitempty"`
	Error      string    `json:"error,omitempty"`
}

// PBSPruneJob represents a PBS prune job
type PBSPruneJob struct {
	ID        string    `json:"id"`
	Store     string    `json:"store"`
	Status    string    `json:"status"`
	LastPrune time.Time `json:"lastPrune"`
	NextRun   time.Time `json:"nextRun,omitempty"`
	Error     string    `json:"error,omitempty"`
}

// PBSGarbageJob represents a PBS garbage collection job
type PBSGarbageJob struct {
	ID           string    `json:"id"`
	Store        string    `json:"store"`
	Status       string    `json:"status"`
	LastGarbage  time.Time `json:"lastGarbage"`
	NextRun      time.Time `json:"nextRun,omitempty"`
	RemovedBytes int64     `json:"removedBytes,omitempty"`
	Error        string    `json:"error,omitempty"`
}

// PMGInstance represents a Proxmox Mail Gateway connection
type PMGInstance struct {
	ID               string               `json:"id"`
	Name             string               `json:"name"`
	Host             string               `json:"host"`
	GuestURL         string               `json:"guestURL,omitempty"` // Optional guest-accessible URL (for navigation)
	Status           string               `json:"status"`
	Version          string               `json:"version"`
	Nodes            []PMGNodeStatus      `json:"nodes,omitempty"`
	MailStats        *PMGMailStats        `json:"mailStats,omitempty"`
	MailCount        []PMGMailCountPoint  `json:"mailCount,omitempty"`
	SpamDistribution []PMGSpamBucket      `json:"spamDistribution,omitempty"`
	Quarantine       *PMGQuarantineTotals `json:"quarantine,omitempty"`
	ConnectionHealth string               `json:"connectionHealth"`
	LastSeen         time.Time            `json:"lastSeen"`
	LastUpdated      time.Time            `json:"lastUpdated"`
}

// PMGNodeStatus represents the status of a PMG cluster node
type PMGNodeStatus struct {
	Name        string          `json:"name"`
	Status      string          `json:"status"`
	Role        string          `json:"role,omitempty"`
	Uptime      int64           `json:"uptime,omitempty"`
	LoadAvg     string          `json:"loadAvg,omitempty"`
	QueueStatus *PMGQueueStatus `json:"queueStatus,omitempty"` // Postfix queue status for this node
}

// PMGBackup represents a configuration backup generated by a PMG node.
type PMGBackup struct {
	ID         string    `json:"id"`
	Instance   string    `json:"instance"`
	Node       string    `json:"node"`
	Filename   string    `json:"filename"`
	BackupTime time.Time `json:"backupTime"`
	Size       int64     `json:"size"`
}

// Backups aggregates backup collections by source type.
type Backups struct {
	PVE PVEBackups  `json:"pve"`
	PBS []PBSBackup `json:"pbs"`
	PMG []PMGBackup `json:"pmg"`
}

// PMGMailStats summarizes aggregated mail statistics for a timeframe
type PMGMailStats struct {
	Timeframe            string    `json:"timeframe"`
	CountTotal           float64   `json:"countTotal"`
	CountIn              float64   `json:"countIn"`
	CountOut             float64   `json:"countOut"`
	SpamIn               float64   `json:"spamIn"`
	SpamOut              float64   `json:"spamOut"`
	VirusIn              float64   `json:"virusIn"`
	VirusOut             float64   `json:"virusOut"`
	BouncesIn            float64   `json:"bouncesIn"`
	BouncesOut           float64   `json:"bouncesOut"`
	BytesIn              float64   `json:"bytesIn"`
	BytesOut             float64   `json:"bytesOut"`
	GreylistCount        float64   `json:"greylistCount"`
	JunkIn               float64   `json:"junkIn"`
	AverageProcessTimeMs float64   `json:"averageProcessTimeMs"`
	RBLRejects           float64   `json:"rblRejects"`
	PregreetRejects      float64   `json:"pregreetRejects"`
	UpdatedAt            time.Time `json:"updatedAt"`
}

// PMGMailCountPoint represents a point-in-time mail counter snapshot
type PMGMailCountPoint struct {
	Timestamp   time.Time `json:"timestamp"`
	Count       float64   `json:"count"`
	CountIn     float64   `json:"countIn"`
	CountOut    float64   `json:"countOut"`
	SpamIn      float64   `json:"spamIn"`
	SpamOut     float64   `json:"spamOut"`
	VirusIn     float64   `json:"virusIn"`
	VirusOut    float64   `json:"virusOut"`
	RBLRejects  float64   `json:"rblRejects"`
	Pregreet    float64   `json:"pregreet"`
	BouncesIn   float64   `json:"bouncesIn"`
	BouncesOut  float64   `json:"bouncesOut"`
	Greylist    float64   `json:"greylist"`
	Index       int       `json:"index"`
	Timeframe   string    `json:"timeframe"`
	WindowStart time.Time `json:"windowStart,omitempty"`
	WindowEnd   time.Time `json:"windowEnd,omitempty"`
}

// PMGSpamBucket represents spam distribution counts by score
type PMGSpamBucket struct {
	Score string  `json:"score"`
	Count float64 `json:"count"`
}

// PMGQuarantineTotals summarizes quarantine counts per category
type PMGQuarantineTotals struct {
	Spam        int `json:"spam"`
	Virus       int `json:"virus"`
	Attachment  int `json:"attachment"`
	Blacklisted int `json:"blacklisted"`
}

// PMGQueueStatus represents the Postfix mail queue status for a PMG instance
type PMGQueueStatus struct {
	Active    int       `json:"active"`    // Messages currently being delivered
	Deferred  int       `json:"deferred"`  // Messages waiting for retry
	Hold      int       `json:"hold"`      // Messages on hold
	Incoming  int       `json:"incoming"`  // Messages in incoming queue
	Total     int       `json:"total"`     // Total messages in all queues
	OldestAge int64     `json:"oldestAge"` // Age of oldest message in seconds (0 if queue empty)
	UpdatedAt time.Time `json:"updatedAt"` // When this queue data was collected
}

// Memory represents memory usage
type Memory struct {
	Total     int64   `json:"total"`
	Used      int64   `json:"used"`
	Free      int64   `json:"free"`
	Usage     float64 `json:"usage"`
	Balloon   int64   `json:"balloon,omitempty"`
	SwapUsed  int64   `json:"swapUsed,omitempty"`
	SwapTotal int64   `json:"swapTotal,omitempty"`
}

type GuestNetworkInterface struct {
	Name      string   `json:"name"`
	MAC       string   `json:"mac,omitempty"`
	Addresses []string `json:"addresses,omitempty"`
	RXBytes   int64    `json:"rxBytes,omitempty"`
	TXBytes   int64    `json:"txBytes,omitempty"`
}

// Disk represents disk usage
type Disk struct {
	Total      int64   `json:"total"`
	Used       int64   `json:"used"`
	Free       int64   `json:"free"`
	Usage      float64 `json:"usage"`
	Mountpoint string  `json:"mountpoint,omitempty"`
	Type       string  `json:"type,omitempty"`
	Device     string  `json:"device,omitempty"`
}

// CPUInfo represents CPU information
type CPUInfo struct {
	Model   string `json:"model"`
	Cores   int    `json:"cores"`
	Sockets int    `json:"sockets"`
	MHz     string `json:"mhz"`
}

// Temperature represents temperature sensors data
type Temperature struct {
	CPUPackage   float64    `json:"cpuPackage,omitempty"`   // CPU package temperature (primary metric)
	CPUMax       float64    `json:"cpuMax,omitempty"`       // Highest core temperature
	CPUMin       float64    `json:"cpuMin,omitempty"`       // Minimum recorded CPU temperature (since monitoring started)
	CPUMaxRecord float64    `json:"cpuMaxRecord,omitempty"` // Maximum recorded CPU temperature (since monitoring started)
	MinRecorded  time.Time  `json:"minRecorded,omitempty"`  // When minimum temperature was recorded
	MaxRecorded  time.Time  `json:"maxRecorded,omitempty"`  // When maximum temperature was recorded
	Cores        []CoreTemp `json:"cores,omitempty"`        // Individual core temperatures
	GPU          []GPUTemp  `json:"gpu,omitempty"`          // GPU temperatures
	NVMe         []NVMeTemp `json:"nvme,omitempty"`         // NVMe drive temperatures
	SMART        []DiskTemp `json:"smart,omitempty"`        // Physical disk temperatures from SMART data
	Available    bool       `json:"available"`              // Whether any temperature data is available
	HasCPU       bool       `json:"hasCPU"`                 // Whether CPU temperature data is available
	HasGPU       bool       `json:"hasGPU"`                 // Whether GPU temperature data is available
	HasNVMe      bool       `json:"hasNVMe"`                // Whether NVMe temperature data is available
	HasSMART     bool       `json:"hasSMART"`               // Whether SMART disk temperature data is available
	LastUpdate   time.Time  `json:"lastUpdate"`             // When this data was collected
}

// CoreTemp represents a CPU core temperature
type CoreTemp struct {
	Core int     `json:"core"`
	Temp float64 `json:"temp"`
}

// GPUTemp represents a GPU temperature sensor
type GPUTemp struct {
	Device   string  `json:"device"`             // GPU device identifier (e.g., "amdgpu-pci-0400")
	Edge     float64 `json:"edge,omitempty"`     // Edge temperature
	Junction float64 `json:"junction,omitempty"` // Junction/hotspot temperature
	Mem      float64 `json:"mem,omitempty"`      // Memory temperature
}

// NVMeTemp represents an NVMe drive temperature
type NVMeTemp struct {
	Device string  `json:"device"`
	Temp   float64 `json:"temp"`
}

// DiskTemp represents a physical disk temperature from SMART data
type DiskTemp struct {
	Device         string    `json:"device"`                   // Device path (e.g., /dev/sda)
	Serial         string    `json:"serial,omitempty"`         // Disk serial number
	WWN            string    `json:"wwn,omitempty"`            // World Wide Name
	Model          string    `json:"model,omitempty"`          // Disk model
	Type           string    `json:"type,omitempty"`           // Transport type (sata, sas, nvme)
	Temperature    int       `json:"temperature"`              // Temperature in Celsius
	LastUpdated    time.Time `json:"lastUpdated"`              // When this reading was taken
	StandbySkipped bool      `json:"standbySkipped,omitempty"` // True if disk was in standby and not queried
}

// Metric represents a time-series metric
type Metric struct {
	Timestamp time.Time              `json:"timestamp"`
	Type      string                 `json:"type"`
	ID        string                 `json:"id"`
	Values    map[string]interface{} `json:"values"`
}

// PVEBackups represents PVE backup information
type PVEBackups struct {
	BackupTasks    []BackupTask    `json:"backupTasks"`
	StorageBackups []StorageBackup `json:"storageBackups"`
	GuestSnapshots []GuestSnapshot `json:"guestSnapshots"`
}

// BackupTask represents a PVE backup task
type BackupTask struct {
	ID        string    `json:"id"`
	Node      string    `json:"node"`
	Instance  string    `json:"instance"` // Unique instance identifier
	Type      string    `json:"type"`
	VMID      int       `json:"vmid"`
	Status    string    `json:"status"`
	StartTime time.Time `json:"startTime"`
	EndTime   time.Time `json:"endTime,omitempty"`
	Size      int64     `json:"size,omitempty"`
	Error     string    `json:"error,omitempty"`
}

// StorageBackup represents a backup file in storage
type StorageBackup struct {
	ID           string    `json:"id"`
	Storage      string    `json:"storage"`
	Node         string    `json:"node"`
	Instance     string    `json:"instance"` // Unique instance identifier (for nodes with duplicate names)
	Type         string    `json:"type"`
	VMID         int       `json:"vmid"`
	Time         time.Time `json:"time"`
	CTime        int64     `json:"ctime"` // Unix timestamp for compatibility
	Size         int64     `json:"size"`
	Format       string    `json:"format"`
	Notes        string    `json:"notes,omitempty"`
	Protected    bool      `json:"protected"`
	Volid        string    `json:"volid"`                  // Volume ID for compatibility
	IsPBS        bool      `json:"isPBS"`                  // Indicates if backup is on PBS storage
	Verified     bool      `json:"verified"`               // PBS verification status
	Verification string    `json:"verification,omitempty"` // Verification details
}

// GuestSnapshot represents a VM/CT snapshot
type GuestSnapshot struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Node        string    `json:"node"`
	Instance    string    `json:"instance"` // Unique instance identifier (for nodes with duplicate names)
	Type        string    `json:"type"`
	VMID        int       `json:"vmid"`
	Time        time.Time `json:"time"`
	Description string    `json:"description,omitempty"`
	Parent      string    `json:"parent,omitempty"`
	VMState     bool      `json:"vmstate"`
	SizeBytes   int64     `json:"sizeBytes,omitempty"`
}

// ReplicationJob represents the status of a Proxmox storage replication job.
type ReplicationJob struct {
	ID                      string     `json:"id"`
	Instance                string     `json:"instance"`
	JobID                   string     `json:"jobId"`
	JobNumber               int        `json:"jobNumber,omitempty"`
	Guest                   string     `json:"guest,omitempty"`
	GuestID                 int        `json:"guestId,omitempty"`
	GuestName               string     `json:"guestName,omitempty"`
	GuestType               string     `json:"guestType,omitempty"`
	GuestNode               string     `json:"guestNode,omitempty"`
	SourceNode              string     `json:"sourceNode,omitempty"`
	SourceStorage           string     `json:"sourceStorage,omitempty"`
	TargetNode              string     `json:"targetNode,omitempty"`
	TargetStorage           string     `json:"targetStorage,omitempty"`
	Schedule                string     `json:"schedule,omitempty"`
	Type                    string     `json:"type,omitempty"`
	Enabled                 bool       `json:"enabled"`
	State                   string     `json:"state,omitempty"`
	Status                  string     `json:"status,omitempty"`
	LastSyncStatus          string     `json:"lastSyncStatus,omitempty"`
	LastSyncTime            *time.Time `json:"lastSyncTime,omitempty"`
	LastSyncUnix            int64      `json:"lastSyncUnix,omitempty"`
	LastSyncDurationSeconds int        `json:"lastSyncDurationSeconds,omitempty"`
	LastSyncDurationHuman   string     `json:"lastSyncDurationHuman,omitempty"`
	NextSyncTime            *time.Time `json:"nextSyncTime,omitempty"`
	NextSyncUnix            int64      `json:"nextSyncUnix,omitempty"`
	DurationSeconds         int        `json:"durationSeconds,omitempty"`
	DurationHuman           string     `json:"durationHuman,omitempty"`
	FailCount               int        `json:"failCount,omitempty"`
	Error                   string     `json:"error,omitempty"`
	Comment                 string     `json:"comment,omitempty"`
	RemoveJob               string     `json:"removeJob,omitempty"`
	RateLimitMbps           *float64   `json:"rateLimitMbps,omitempty"`
	LastPolled              time.Time  `json:"lastPolled"`
}

// Performance represents performance metrics
type Performance struct {
	APICallDuration  map[string]float64 `json:"apiCallDuration"`
	LastPollDuration float64            `json:"lastPollDuration"`
	PollingStartTime time.Time          `json:"pollingStartTime"`
	TotalAPICalls    int                `json:"totalApiCalls"`
	FailedAPICalls   int                `json:"failedApiCalls"`
}

// Stats represents runtime statistics
type Stats struct {
	StartTime        time.Time `json:"startTime"`
	Uptime           int64     `json:"uptime"`
	PollingCycles    int       `json:"pollingCycles"`
	WebSocketClients int       `json:"webSocketClients"`
	Version          string    `json:"version"`
}

// NewState creates a new State instance
func NewState() *State {
	pveBackups := PVEBackups{
		BackupTasks:    make([]BackupTask, 0),
		StorageBackups: make([]StorageBackup, 0),
		GuestSnapshots: make([]GuestSnapshot, 0),
	}

	state := &State{
		Nodes:         make([]Node, 0),
		VMs:           make([]VM, 0),
		Containers:    make([]Container, 0),
		DockerHosts:   make([]DockerHost, 0),
		Storage:       make([]Storage, 0),
		PhysicalDisks: make([]PhysicalDisk, 0),
		PBSInstances:  make([]PBSInstance, 0),
		PMGInstances:  make([]PMGInstance, 0),
		PBSBackups:    make([]PBSBackup, 0),
		PMGBackups:    make([]PMGBackup, 0),
		Backups: Backups{
			PVE: pveBackups,
			PBS: make([]PBSBackup, 0),
			PMG: make([]PMGBackup, 0),
		},
		ReplicationJobs:  make([]ReplicationJob, 0),
		Metrics:          make([]Metric, 0),
		PVEBackups:       pveBackups,
		ConnectionHealth: make(map[string]bool),
		ActiveAlerts:     make([]Alert, 0),
		RecentlyResolved: make([]ResolvedAlert, 0),
		LastUpdate:       time.Now(),
	}

	state.syncBackupsLocked()
	return state
}

// syncBackupsLocked updates the aggregated backups structure.
func (s *State) syncBackupsLocked() {
	s.Backups = Backups{
		PVE: s.PVEBackups,
		PBS: append([]PBSBackup(nil), s.PBSBackups...),
		PMG: append([]PMGBackup(nil), s.PMGBackups...),
	}
}

// UpdateActiveAlerts updates the active alerts in the state
func (s *State) UpdateActiveAlerts(alerts []Alert) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ActiveAlerts = alerts
}

// UpdateRecentlyResolved updates the recently resolved alerts in the state
func (s *State) UpdateRecentlyResolved(resolved []ResolvedAlert) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.RecentlyResolved = resolved
}

// UpdateNodes updates the nodes in the state
func (s *State) UpdateNodes(nodes []Node) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Sort nodes by name to ensure consistent ordering
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Name < nodes[j].Name
	})

	s.Nodes = nodes
	s.LastUpdate = time.Now()
}

// UpdateNodesForInstance updates nodes for a specific instance, merging with existing nodes
func (s *State) UpdateNodesForInstance(instanceName string, nodes []Node) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Build a map of ALL existing nodes by ID (not filtered by instance)
	// This handles cluster-based IDs where the same node ID comes from multiple instances
	// Also preserve LinkedHostAgentID for nodes that are being updated
	existingNodeLinks := make(map[string]string) // nodeID -> linkedHostAgentID
	nodeMap := make(map[string]Node)
	for _, node := range s.Nodes {
		nodeMap[node.ID] = node
		if node.LinkedHostAgentID != "" {
			existingNodeLinks[node.ID] = node.LinkedHostAgentID
		}
	}

	// Build hostname-to-hostAgentID map for linking new nodes to existing host agents.
	// Keep all candidates so we can avoid creating links when hostname matching is ambiguous.
	// Also build a set of valid host agent IDs to validate existing links
	hostAgentByHostname := make(map[string]map[string]struct{}) // lowercase hostname -> hostAgentIDs
	validHostAgentIDs := make(map[string]bool)                  // set of existing host agent IDs
	addHostAlias := func(name, hostID string) {
		name = strings.TrimSpace(strings.ToLower(name))
		if name == "" || hostID == "" {
			return
		}
		bucket := hostAgentByHostname[name]
		if bucket == nil {
			bucket = make(map[string]struct{})
			hostAgentByHostname[name] = bucket
		}
		bucket[hostID] = struct{}{}
	}
	for _, host := range s.Hosts {
		if host.ID != "" {
			validHostAgentIDs[host.ID] = true
			addHostAlias(host.Hostname, host.ID)
			// Also index by short hostname
			if idx := strings.Index(host.Hostname, "."); idx > 0 {
				addHostAlias(host.Hostname[:idx], host.ID)
			}
		}
	}

	// Add or update nodes from this instance
	for _, node := range nodes {
		// Preserve existing link if we had one, but only if the host agent still exists
		if existingLink, ok := existingNodeLinks[node.ID]; ok {
			if validHostAgentIDs[existingLink] {
				node.LinkedHostAgentID = existingLink
			}
			// If host agent no longer exists, leave LinkedHostAgentID empty (stale reference cleared)
		}
		// If no existing link, try to match by hostname
		if node.LinkedHostAgentID == "" {
			nodeName := strings.TrimSpace(strings.ToLower(node.Name))
			candidates := make(map[string]struct{})
			if nodeName != "" {
				if ids, ok := hostAgentByHostname[nodeName]; ok {
					for id := range ids {
						candidates[id] = struct{}{}
					}
				}
				if idx := strings.Index(nodeName, "."); idx > 0 {
					if ids, ok := hostAgentByHostname[nodeName[:idx]]; ok {
						for id := range ids {
							candidates[id] = struct{}{}
						}
					}
				}
			}
			if len(candidates) == 1 {
				for id := range candidates {
					node.LinkedHostAgentID = id
				}
			}
		}
		nodeMap[node.ID] = node
	}

	// Convert map back to slice
	newNodes := make([]Node, 0, len(nodeMap))
	for _, node := range nodeMap {
		newNodes = append(newNodes, node)
	}

	// Sort nodes by name to ensure consistent ordering
	sort.Slice(newNodes, func(i, j int) bool {
		return newNodes[i].Name < newNodes[j].Name
	})

	s.Nodes = newNodes
	s.LastUpdate = time.Now()
}

// UpdateVMs updates the VMs in the state
func (s *State) UpdateVMs(vms []VM) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.VMs = vms
	s.LastUpdate = time.Now()
}

// UpdateVMsForInstance updates VMs for a specific instance, merging with existing VMs
func (s *State) UpdateVMsForInstance(instanceName string, vms []VM) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Build a lookup of existing VMs for this instance to preserve LastBackup
	existingByVMID := make(map[int]VM)
	for _, vm := range s.VMs {
		if vm.Instance == instanceName {
			existingByVMID[vm.VMID] = vm
		}
	}

	// Create a map of existing VMs, excluding those from this instance
	vmMap := make(map[string]VM)
	for _, vm := range s.VMs {
		if vm.Instance != instanceName {
			vmMap[vm.ID] = vm
		}
	}

	// Add or update VMs from this instance, preserving LastBackup from existing data
	for _, vm := range vms {
		if existing, ok := existingByVMID[vm.VMID]; ok && vm.LastBackup.IsZero() {
			vm.LastBackup = existing.LastBackup
		}
		vmMap[vm.ID] = vm
	}

	// Convert map back to slice
	newVMs := make([]VM, 0, len(vmMap))
	for _, vm := range vmMap {
		newVMs = append(newVMs, vm)
	}

	// Sort VMs by VMID to ensure consistent ordering
	sort.Slice(newVMs, func(i, j int) bool {
		return newVMs[i].VMID < newVMs[j].VMID
	})

	s.VMs = newVMs
	s.LastUpdate = time.Now()
}

// UpdateContainers updates the containers in the state
func (s *State) UpdateContainers(containers []Container) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Containers = containers
	s.LastUpdate = time.Now()
}

// backupKey creates a composite key for backup matching using instance and VMID.
// This ensures backups are correctly matched to guests even when VMIDs are reused across instances.
func backupKey(instance string, vmid int) string {
	return instance + "-" + strconv.Itoa(vmid)
}

// namespaceMatchesInstance checks if a PBS namespace likely corresponds to a PVE instance.
// This helps disambiguate backups when multiple PVE instances have VMs with the same VMID.
// Examples: namespace "pve1" matches instance "pve1", namespace "nat" matches instance "pve-nat"
func namespaceMatchesInstance(namespace, instance string) bool {
	if namespace == "" || instance == "" {
		return false
	}

	// Normalize both strings: lowercase and keep only alphanumeric
	normalize := func(s string) string {
		var b strings.Builder
		for _, r := range strings.ToLower(s) {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
				b.WriteRune(r)
			}
		}
		return b.String()
	}

	ns := normalize(namespace)
	inst := normalize(instance)

	if ns == "" || inst == "" {
		return false
	}

	// Exact match after normalization
	if ns == inst {
		return true
	}

	// Check if namespace is a suffix of instance
	// e.g., namespace "nat" matches instance "pvenat" (normalized from "pve-nat")
	// This is more precise than substring matching because:
	// - "nat" should match "pve-nat" but not "natpve"
	// - "pve" should match "pve" but not "pve-nat" (handled by exact match above)
	if strings.HasSuffix(inst, ns) {
		return true
	}

	// Check if instance is a suffix of namespace (reverse case)
	// e.g., namespace "pvebackups" could match instance "pve"
	if strings.HasSuffix(ns, inst) {
		return true
	}

	return false
}

// SyncGuestBackupTimes updates LastBackup on VMs and Containers from storage backups and PBS backups.
// Call this after updating storage backups or PBS backups to ensure guest backup indicators are accurate.
// Matching is done by instance+VMID to prevent cross-instance VMID collisions.
// For PBS backups with namespaces, namespace matching is used to disambiguate.
func (s *State) SyncGuestBackupTimes() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Build a map of instance+VMID -> latest backup time from all backup sources
	// Using composite key prevents cross-instance VMID collision issues
	latestBackup := make(map[string]time.Time)

	// Process PVE storage backups
	for _, backup := range s.PVEBackups.StorageBackups {
		if backup.VMID <= 0 || backup.Instance == "" {
			continue
		}
		key := backupKey(backup.Instance, backup.VMID)
		if existing, ok := latestBackup[key]; !ok || backup.Time.After(existing) {
			latestBackup[key] = backup.Time
		}
	}

	// Process PBS backups (VMID is string, BackupTime is the timestamp)
	// PBS backups can have a Namespace field that often corresponds to the PVE instance.
	// We use namespace matching to associate PBS backups with the correct PVE instance.
	// Structure: map[vmid][]PBSBackup to handle multiple backups per VMID
	pbsBackupsByVMID := make(map[int][]PBSBackup)
	for _, backup := range s.PBSBackups {
		vmid, err := strconv.Atoi(backup.VMID)
		if err != nil || vmid <= 0 {
			continue
		}
		pbsBackupsByVMID[vmid] = append(pbsBackupsByVMID[vmid], backup)
	}

	// Build a set of VMIDs that appear on more than one PVE instance.
	// When a VMID is ambiguous, we must not fall back to VMID-only matching
	// because we can't tell which guest the backup belongs to.
	vmidInstances := make(map[int]map[string]struct{})
	for i := range s.VMs {
		m, ok := vmidInstances[s.VMs[i].VMID]
		if !ok {
			m = make(map[string]struct{})
			vmidInstances[s.VMs[i].VMID] = m
		}
		m[s.VMs[i].Instance] = struct{}{}
	}
	for i := range s.Containers {
		m, ok := vmidInstances[s.Containers[i].VMID]
		if !ok {
			m = make(map[string]struct{})
			vmidInstances[s.Containers[i].VMID] = m
		}
		m[s.Containers[i].Instance] = struct{}{}
	}
	vmidIsAmbiguous := make(map[int]bool)
	for vmid, instances := range vmidInstances {
		if len(instances) > 1 {
			vmidIsAmbiguous[vmid] = true
		}
	}

	// findBestPBSBackup finds the most recent PBS backup for a given VMID and instance.
	// If the backup has a namespace that matches the instance, it's preferred.
	// Returns zero time if no suitable backup found.
	findBestPBSBackup := func(vmid int, instance string) time.Time {
		backups, ok := pbsBackupsByVMID[vmid]
		if !ok || len(backups) == 0 {
			return time.Time{}
		}

		var bestTime time.Time
		var bestMatchTime time.Time // Best time among namespace-matched backups

		for _, backup := range backups {
			// If namespace matches this instance, track it separately
			if backup.Namespace != "" && namespaceMatchesInstance(backup.Namespace, instance) {
				if backup.BackupTime.After(bestMatchTime) {
					bestMatchTime = backup.BackupTime
				}
			}
			// Track overall best time as fallback
			if backup.BackupTime.After(bestTime) {
				bestTime = backup.BackupTime
			}
		}

		// Prefer namespace-matched backup if available
		if !bestMatchTime.IsZero() {
			return bestMatchTime
		}

		// Fall back to any backup with this VMID, but only when the VMID is
		// unique across PVE instances. If the VMID exists on multiple instances,
		// the match is ambiguous and we must not guess.
		if vmidIsAmbiguous[vmid] {
			return time.Time{}
		}
		return bestTime
	}

	// Update VMs - prefer instance-specific PVE backup, fall back to PBS
	for i := range s.VMs {
		key := backupKey(s.VMs[i].Instance, s.VMs[i].VMID)
		if backupTime, ok := latestBackup[key]; ok {
			s.VMs[i].LastBackup = backupTime
		}
		// Check if PBS has a more recent backup
		pbsTime := findBestPBSBackup(s.VMs[i].VMID, s.VMs[i].Instance)
		if !pbsTime.IsZero() {
			if s.VMs[i].LastBackup.IsZero() || pbsTime.After(s.VMs[i].LastBackup) {
				s.VMs[i].LastBackup = pbsTime
			}
		}
	}

	// Update Containers - prefer instance-specific PVE backup, fall back to PBS
	for i := range s.Containers {
		key := backupKey(s.Containers[i].Instance, s.Containers[i].VMID)
		if backupTime, ok := latestBackup[key]; ok {
			s.Containers[i].LastBackup = backupTime
		}
		// Check if PBS has a more recent backup
		pbsTime := findBestPBSBackup(s.Containers[i].VMID, s.Containers[i].Instance)
		if !pbsTime.IsZero() {
			if s.Containers[i].LastBackup.IsZero() || pbsTime.After(s.Containers[i].LastBackup) {
				s.Containers[i].LastBackup = pbsTime
			}
		}
	}

	s.LastUpdate = time.Now()
}

// UpdateContainersForInstance updates containers for a specific instance, merging with existing containers
func (s *State) UpdateContainersForInstance(instanceName string, containers []Container) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Build a lookup of existing containers for this instance to preserve LastBackup
	existingByVMID := make(map[int]Container)
	for _, ct := range s.Containers {
		if ct.Instance == instanceName {
			existingByVMID[ct.VMID] = ct
		}
	}

	// Create a map of existing containers, excluding those from this instance
	containerMap := make(map[string]Container)
	for _, container := range s.Containers {
		if container.Instance != instanceName {
			containerMap[container.ID] = container
		}
	}

	// Add or update containers from this instance, preserving LastBackup from existing data
	for _, container := range containers {
		if existing, ok := existingByVMID[container.VMID]; ok && container.LastBackup.IsZero() {
			container.LastBackup = existing.LastBackup
		}
		containerMap[container.ID] = container
	}

	// Convert map back to slice
	newContainers := make([]Container, 0, len(containerMap))
	for _, container := range containerMap {
		newContainers = append(newContainers, container)
	}

	// Sort containers by VMID to ensure consistent ordering
	sort.Slice(newContainers, func(i, j int) bool {
		return newContainers[i].VMID < newContainers[j].VMID
	})

	s.Containers = newContainers
	s.LastUpdate = time.Now()
}

// UpsertDockerHost inserts or updates a Docker host in state.
func (s *State) UpsertDockerHost(host DockerHost) {
	s.mu.Lock()
	defer s.mu.Unlock()

	updated := false
	for i, existing := range s.DockerHosts {
		if existing.ID == host.ID {
			// Preserve custom display name if it was set
			if existing.CustomDisplayName != "" {
				host.CustomDisplayName = existing.CustomDisplayName
			}
			// Preserve Hidden and PendingUninstall flags
			host.Hidden = existing.Hidden
			host.PendingUninstall = existing.PendingUninstall
			// Preserve Command if it exists
			if existing.Command != nil {
				host.Command = existing.Command
			}
			s.DockerHosts[i] = host
			updated = true
			break
		}
	}

	if !updated {
		s.DockerHosts = append(s.DockerHosts, host)
	}

	sort.Slice(s.DockerHosts, func(i, j int) bool {
		return s.DockerHosts[i].Hostname < s.DockerHosts[j].Hostname
	})

	s.LastUpdate = time.Now()
}

// RemoveDockerHost removes a docker host by ID and returns the removed host.
func (s *State) RemoveDockerHost(hostID string) (DockerHost, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, host := range s.DockerHosts {
		if host.ID == hostID {
			// Remove the host while preserving slice order
			s.DockerHosts = append(s.DockerHosts[:i], s.DockerHosts[i+1:]...)
			s.LastUpdate = time.Now()
			return host, true
		}
	}

	return DockerHost{}, false
}

// ClearAllDockerHosts removes all docker hosts from the state.
// This is used during initial security setup to clear any docker hosts that may have
// connected during the brief unauthenticated window before credentials were configured.
func (s *State) ClearAllDockerHosts() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := len(s.DockerHosts)
	s.DockerHosts = nil
	s.LastUpdate = time.Now()
	return count
}

// SetDockerHostStatus updates the status of a docker host if present.
func (s *State) SetDockerHostStatus(hostID, status string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	changed := false
	for i, host := range s.DockerHosts {
		if host.ID == hostID {
			if host.Status != status {
				host.Status = status
				s.DockerHosts[i] = host
				s.LastUpdate = time.Now()
			}
			changed = true
			break
		}
	}

	return changed
}

// SetDockerHostHidden updates the hidden status of a docker host and returns the updated host.
func (s *State) SetDockerHostHidden(hostID string, hidden bool) (DockerHost, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, host := range s.DockerHosts {
		if host.ID == hostID {
			host.Hidden = hidden
			s.DockerHosts[i] = host
			s.LastUpdate = time.Now()
			return host, true
		}
	}

	return DockerHost{}, false
}

// SetDockerHostPendingUninstall updates the pending uninstall status of a docker host and returns the updated host.
func (s *State) SetDockerHostPendingUninstall(hostID string, pending bool) (DockerHost, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, host := range s.DockerHosts {
		if host.ID == hostID {
			host.PendingUninstall = pending
			s.DockerHosts[i] = host
			s.LastUpdate = time.Now()
			return host, true
		}
	}

	return DockerHost{}, false
}

// SetDockerHostCommand updates the active command status for a docker host.
func (s *State) SetDockerHostCommand(hostID string, command *DockerHostCommandStatus) (DockerHost, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, host := range s.DockerHosts {
		if host.ID == hostID {
			host.Command = command
			s.DockerHosts[i] = host
			s.LastUpdate = time.Now()
			return host, true
		}
	}

	return DockerHost{}, false
}

// SetDockerHostCustomDisplayName updates the custom display name for a docker host.
func (s *State) SetDockerHostCustomDisplayName(hostID string, customName string) (DockerHost, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, host := range s.DockerHosts {
		if host.ID == hostID {
			host.CustomDisplayName = customName
			s.DockerHosts[i] = host
			s.LastUpdate = time.Now()
			return host, true
		}
	}

	return DockerHost{}, false
}

// TouchDockerHost updates the last seen timestamp for a docker host.
func (s *State) TouchDockerHost(hostID string, ts time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, host := range s.DockerHosts {
		if host.ID == hostID {
			host.LastSeen = ts
			s.DockerHosts[i] = host
			s.LastUpdate = time.Now()
			return true
		}
	}

	return false
}

// RemoveStaleDockerHosts removes docker hosts that haven't been seen since cutoff.
func (s *State) RemoveStaleDockerHosts(cutoff time.Time) []DockerHost {
	s.mu.Lock()
	defer s.mu.Unlock()

	removed := make([]DockerHost, 0)
	fresh := make([]DockerHost, 0, len(s.DockerHosts))
	for _, host := range s.DockerHosts {
		if host.LastSeen.Before(cutoff) && cutoff.After(host.LastSeen) {
			removed = append(removed, host)
			continue
		}
		fresh = append(fresh, host)
	}

	if len(removed) > 0 {
		s.DockerHosts = fresh
		s.LastUpdate = time.Now()
	}

	return removed
}

// GetDockerHosts returns a copy of docker hosts.
func (s *State) GetDockerHosts() []DockerHost {
	s.mu.RLock()
	defer s.mu.RUnlock()

	hosts := make([]DockerHost, len(s.DockerHosts))
	copy(hosts, s.DockerHosts)
	return hosts
}

// AddRemovedDockerHost records a removed docker host entry.
func (s *State) AddRemovedDockerHost(entry RemovedDockerHost) {
	s.mu.Lock()
	defer s.mu.Unlock()

	replaced := false
	for i, existing := range s.RemovedDockerHosts {
		if existing.ID == entry.ID {
			s.RemovedDockerHosts[i] = entry
			replaced = true
			break
		}
	}
	if !replaced {
		s.RemovedDockerHosts = append(s.RemovedDockerHosts, entry)
	}
	sort.Slice(s.RemovedDockerHosts, func(i, j int) bool {
		return s.RemovedDockerHosts[i].RemovedAt.After(s.RemovedDockerHosts[j].RemovedAt)
	})
	s.LastUpdate = time.Now()
}

// RemoveRemovedDockerHost deletes a removed docker host entry by ID.
func (s *State) RemoveRemovedDockerHost(hostID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, entry := range s.RemovedDockerHosts {
		if entry.ID == hostID {
			s.RemovedDockerHosts = append(s.RemovedDockerHosts[:i], s.RemovedDockerHosts[i+1:]...)
			s.LastUpdate = time.Now()
			break
		}
	}
}

// GetRemovedDockerHosts returns a copy of removed docker host entries.
func (s *State) GetRemovedDockerHosts() []RemovedDockerHost {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries := make([]RemovedDockerHost, len(s.RemovedDockerHosts))
	copy(entries, s.RemovedDockerHosts)
	return entries
}

// UpsertKubernetesCluster inserts or updates a Kubernetes cluster in state.
func (s *State) UpsertKubernetesCluster(cluster KubernetesCluster) {
	s.mu.Lock()
	defer s.mu.Unlock()

	updated := false
	for i, existing := range s.KubernetesClusters {
		if existing.ID == cluster.ID {
			if existing.CustomDisplayName != "" {
				cluster.CustomDisplayName = existing.CustomDisplayName
			}
			cluster.Hidden = existing.Hidden
			cluster.PendingUninstall = existing.PendingUninstall
			s.KubernetesClusters[i] = cluster
			updated = true
			break
		}
	}

	if !updated {
		s.KubernetesClusters = append(s.KubernetesClusters, cluster)
	}

	sort.Slice(s.KubernetesClusters, func(i, j int) bool {
		left := s.KubernetesClusters[i].Name
		right := s.KubernetesClusters[j].Name
		if left == "" {
			left = s.KubernetesClusters[i].ID
		}
		if right == "" {
			right = s.KubernetesClusters[j].ID
		}
		return left < right
	})

	s.LastUpdate = time.Now()
}

// RemoveKubernetesCluster removes a Kubernetes cluster by ID and returns the removed cluster.
func (s *State) RemoveKubernetesCluster(clusterID string) (KubernetesCluster, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, cluster := range s.KubernetesClusters {
		if cluster.ID == clusterID {
			s.KubernetesClusters = append(s.KubernetesClusters[:i], s.KubernetesClusters[i+1:]...)
			s.LastUpdate = time.Now()
			return cluster, true
		}
	}

	return KubernetesCluster{}, false
}

// SetKubernetesClusterStatus updates the status of a kubernetes cluster if present.
func (s *State) SetKubernetesClusterStatus(clusterID, status string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	changed := false
	for i, cluster := range s.KubernetesClusters {
		if cluster.ID == clusterID {
			if cluster.Status != status {
				cluster.Status = status
				s.KubernetesClusters[i] = cluster
				s.LastUpdate = time.Now()
			}
			changed = true
			break
		}
	}
	return changed
}

// SetKubernetesClusterHidden updates the hidden status of a kubernetes cluster and returns the updated cluster.
func (s *State) SetKubernetesClusterHidden(clusterID string, hidden bool) (KubernetesCluster, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, cluster := range s.KubernetesClusters {
		if cluster.ID == clusterID {
			cluster.Hidden = hidden
			s.KubernetesClusters[i] = cluster
			s.LastUpdate = time.Now()
			return cluster, true
		}
	}
	return KubernetesCluster{}, false
}

// SetKubernetesClusterPendingUninstall updates the pending uninstall flag and returns the updated cluster.
func (s *State) SetKubernetesClusterPendingUninstall(clusterID string, pending bool) (KubernetesCluster, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, cluster := range s.KubernetesClusters {
		if cluster.ID == clusterID {
			cluster.PendingUninstall = pending
			s.KubernetesClusters[i] = cluster
			s.LastUpdate = time.Now()
			return cluster, true
		}
	}
	return KubernetesCluster{}, false
}

// SetKubernetesClusterCustomDisplayName updates the custom display name for a kubernetes cluster.
func (s *State) SetKubernetesClusterCustomDisplayName(clusterID string, customName string) (KubernetesCluster, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, cluster := range s.KubernetesClusters {
		if cluster.ID == clusterID {
			cluster.CustomDisplayName = customName
			s.KubernetesClusters[i] = cluster
			s.LastUpdate = time.Now()
			return cluster, true
		}
	}

	return KubernetesCluster{}, false
}

// GetKubernetesClusters returns a copy of kubernetes clusters.
func (s *State) GetKubernetesClusters() []KubernetesCluster {
	s.mu.RLock()
	defer s.mu.RUnlock()

	clusters := make([]KubernetesCluster, len(s.KubernetesClusters))
	copy(clusters, s.KubernetesClusters)
	return clusters
}

// AddRemovedKubernetesCluster records a removed kubernetes cluster entry.
func (s *State) AddRemovedKubernetesCluster(entry RemovedKubernetesCluster) {
	s.mu.Lock()
	defer s.mu.Unlock()

	replaced := false
	for i, existing := range s.RemovedKubernetesClusters {
		if existing.ID == entry.ID {
			s.RemovedKubernetesClusters[i] = entry
			replaced = true
			break
		}
	}
	if !replaced {
		s.RemovedKubernetesClusters = append(s.RemovedKubernetesClusters, entry)
	}
	sort.Slice(s.RemovedKubernetesClusters, func(i, j int) bool {
		return s.RemovedKubernetesClusters[i].RemovedAt.After(s.RemovedKubernetesClusters[j].RemovedAt)
	})
	s.LastUpdate = time.Now()
}

// RemoveRemovedKubernetesCluster deletes a removed kubernetes cluster entry by ID.
func (s *State) RemoveRemovedKubernetesCluster(clusterID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, entry := range s.RemovedKubernetesClusters {
		if entry.ID == clusterID {
			s.RemovedKubernetesClusters = append(s.RemovedKubernetesClusters[:i], s.RemovedKubernetesClusters[i+1:]...)
			s.LastUpdate = time.Now()
			break
		}
	}
}

// GetRemovedKubernetesClusters returns a copy of removed kubernetes cluster entries.
func (s *State) GetRemovedKubernetesClusters() []RemovedKubernetesCluster {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries := make([]RemovedKubernetesCluster, len(s.RemovedKubernetesClusters))
	copy(entries, s.RemovedKubernetesClusters)
	return entries
}

// UpsertHost inserts or updates a generic host in state.
func (s *State) UpsertHost(host Host) {
	s.mu.Lock()
	defer s.mu.Unlock()

	updated := false
	for i, existing := range s.Hosts {
		if existing.ID == host.ID {
			s.Hosts[i] = host
			updated = true
			break
		}
	}

	if !updated {
		s.Hosts = append(s.Hosts, host)
	}

	sort.Slice(s.Hosts, func(i, j int) bool {
		return s.Hosts[i].Hostname < s.Hosts[j].Hostname
	})

	s.LastUpdate = time.Now()
}

// GetHosts returns a copy of all generic hosts.
func (s *State) GetHosts() []Host {
	s.mu.RLock()
	defer s.mu.RUnlock()

	hosts := make([]Host, len(s.Hosts))
	copy(hosts, s.Hosts)
	return hosts
}

// RemoveHost removes a host by ID and returns the removed entry.
func (s *State) RemoveHost(hostID string) (Host, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, host := range s.Hosts {
		if host.ID == hostID {
			s.Hosts = append(s.Hosts[:i], s.Hosts[i+1:]...)
			s.LastUpdate = time.Now()
			return host, true
		}
	}

	return Host{}, false
}

// ClearAllHosts removes all host agents from the state.
// This is used during initial security setup to clear any hosts that may have
// connected during the brief unauthenticated window before credentials were configured.
func (s *State) ClearAllHosts() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := len(s.Hosts)
	s.Hosts = nil
	s.LastUpdate = time.Now()
	return count
}

// LinkNodeToHostAgent updates a PVE node to link to its host agent.
// This is called when a host agent registers and matches a known PVE node by hostname.
func (s *State) LinkNodeToHostAgent(nodeID, hostAgentID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, node := range s.Nodes {
		if node.ID == nodeID {
			s.Nodes[i].LinkedHostAgentID = hostAgentID
			s.LastUpdate = time.Now()
			return true
		}
	}
	return false
}

// UnlinkNodesFromHostAgent clears LinkedHostAgentID from all nodes linked to the given host agent.
// This is called when a host agent is removed to clean up stale references.
func (s *State) UnlinkNodesFromHostAgent(hostAgentID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := 0
	for i, node := range s.Nodes {
		if node.LinkedHostAgentID == hostAgentID {
			s.Nodes[i].LinkedHostAgentID = ""
			count++
		}
	}
	if count > 0 {
		s.LastUpdate = time.Now()
	}
	return count
}

// LinkHostAgentToNode creates a bidirectional link between a host agent and a PVE node.
// This is used for manual linking when auto-linking can't disambiguate (e.g., multiple nodes
// with the same hostname). Sets LinkedNodeID on the host and LinkedHostAgentID on the node.
// Returns an error if either the host or node is not found.
func (s *State) LinkHostAgentToNode(hostID, nodeID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Find the host
	var hostIdx int = -1
	for i, host := range s.Hosts {
		if host.ID == hostID {
			hostIdx = i
			break
		}
	}
	if hostIdx < 0 {
		return fmt.Errorf("host agent not found: %s", hostID)
	}

	// Find the node
	var nodeIdx int = -1
	for i, node := range s.Nodes {
		if node.ID == nodeID {
			nodeIdx = i
			break
		}
	}
	if nodeIdx < 0 {
		return fmt.Errorf("node not found: %s", nodeID)
	}

	// Clear any existing links from this host
	oldNodeID := s.Hosts[hostIdx].LinkedNodeID
	if oldNodeID != "" {
		for i, node := range s.Nodes {
			if node.ID == oldNodeID {
				s.Nodes[i].LinkedHostAgentID = ""
				break
			}
		}
	}

	// Clear any existing links to the target node (from other hosts)
	for i, host := range s.Hosts {
		if host.LinkedNodeID == nodeID && i != hostIdx {
			s.Hosts[i].LinkedNodeID = ""
		}
	}

	// Create the bidirectional link
	s.Hosts[hostIdx].LinkedNodeID = nodeID
	s.Hosts[hostIdx].LinkedVMID = "" // Clear VM/container links if setting node link
	s.Hosts[hostIdx].LinkedContainerID = ""
	s.Nodes[nodeIdx].LinkedHostAgentID = hostID

	s.LastUpdate = time.Now()
	return nil
}

// UnlinkHostAgent removes the bidirectional link between a host agent and its PVE node.
// Clears LinkedNodeID on the host and LinkedHostAgentID on the node.
// Returns true if the host was found and unlinked.
func (s *State) UnlinkHostAgent(hostID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Find the host
	var hostIdx int = -1
	var linkedNodeID string
	for i, host := range s.Hosts {
		if host.ID == hostID {
			hostIdx = i
			linkedNodeID = host.LinkedNodeID
			break
		}
	}

	if hostIdx < 0 || linkedNodeID == "" {
		return false
	}

	// Clear the link on the host
	s.Hosts[hostIdx].LinkedNodeID = ""
	s.Hosts[hostIdx].LinkedVMID = ""
	s.Hosts[hostIdx].LinkedContainerID = ""

	// Clear the link on the node
	for i, node := range s.Nodes {
		if node.ID == linkedNodeID || node.LinkedHostAgentID == hostID {
			s.Nodes[i].LinkedHostAgentID = ""
		}
	}

	s.LastUpdate = time.Now()
	return true
}

// UpsertCephCluster inserts or updates a Ceph cluster in the state.
// Uses ID (typically the FSID) for matching.
func (s *State) UpsertCephCluster(cluster CephCluster) {
	s.mu.Lock()
	defer s.mu.Unlock()

	updated := false
	for i, existing := range s.CephClusters {
		if existing.ID == cluster.ID {
			s.CephClusters[i] = cluster
			updated = true
			break
		}
	}

	if !updated {
		s.CephClusters = append(s.CephClusters, cluster)
	}

	sort.Slice(s.CephClusters, func(i, j int) bool {
		return s.CephClusters[i].Name < s.CephClusters[j].Name
	})

	s.LastUpdate = time.Now()
}

// SetHostStatus updates the status of a host if present.
func (s *State) SetHostStatus(hostID, status string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, host := range s.Hosts {
		if host.ID == hostID {
			if host.Status != status {
				host.Status = status
				s.Hosts[i] = host
				s.LastUpdate = time.Now()
			}
			return true
		}
	}
	return false
}

// TouchHost updates the last seen timestamp for a host.
func (s *State) TouchHost(hostID string, ts time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, host := range s.Hosts {
		if host.ID == hostID {
			host.LastSeen = ts
			s.Hosts[i] = host
			s.LastUpdate = time.Now()
			return true
		}
	}
	return false
}

// SetHostCommandsEnabled updates the CommandsEnabled flag for a host.
// This allows the UI to immediately reflect config changes without waiting for agent confirmation.
func (s *State) SetHostCommandsEnabled(hostID string, enabled bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, host := range s.Hosts {
		if host.ID == hostID {
			if host.CommandsEnabled != enabled {
				host.CommandsEnabled = enabled
				s.Hosts[i] = host
				s.LastUpdate = time.Now()
			}
			return true
		}
	}
	return false
}

// UpdateStorage updates the storage in the state
func (s *State) UpdateStorage(storage []Storage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Storage = storage
	s.LastUpdate = time.Now()
}

// UpdatePhysicalDisks updates physical disks for a specific instance
func (s *State) UpdatePhysicalDisks(instanceName string, disks []PhysicalDisk) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create a map of existing disks, excluding those from this instance
	diskMap := make(map[string]PhysicalDisk)
	for _, disk := range s.PhysicalDisks {
		if disk.Instance != instanceName {
			diskMap[disk.ID] = disk
		}
	}

	// Add or update disks from this instance
	for _, disk := range disks {
		diskMap[disk.ID] = disk
	}

	// Convert map back to slice
	newDisks := make([]PhysicalDisk, 0, len(diskMap))
	for _, disk := range diskMap {
		newDisks = append(newDisks, disk)
	}

	// Sort by node and dev path for consistent ordering
	sort.Slice(newDisks, func(i, j int) bool {
		if newDisks[i].Node != newDisks[j].Node {
			return newDisks[i].Node < newDisks[j].Node
		}
		return newDisks[i].DevPath < newDisks[j].DevPath
	})

	s.PhysicalDisks = newDisks
	s.LastUpdate = time.Now()
}

// UpdateStorageForInstance updates storage for a specific instance, merging with existing storage
func (s *State) UpdateStorageForInstance(instanceName string, storage []Storage) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create a map of existing storage, excluding those from this instance
	storageMap := make(map[string]Storage)
	for _, st := range s.Storage {
		if st.Instance != instanceName {
			storageMap[st.ID] = st
		}
	}

	// Add or update storage from this instance
	for _, st := range storage {
		storageMap[st.ID] = st
	}

	// Convert map back to slice
	newStorage := make([]Storage, 0, len(storageMap))
	for _, st := range storageMap {
		newStorage = append(newStorage, st)
	}

	// Sort storage by name to ensure consistent ordering
	sort.Slice(newStorage, func(i, j int) bool {
		if newStorage[i].Instance == newStorage[j].Instance {
			return newStorage[i].Name < newStorage[j].Name
		}
		return newStorage[i].Instance < newStorage[j].Instance
	})

	s.Storage = newStorage
	s.LastUpdate = time.Now()
}

// UpdateCephClustersForInstance updates Ceph cluster information for a specific instance
func (s *State) UpdateCephClustersForInstance(instanceName string, clusters []CephCluster) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Preserve clusters from other instances
	filtered := make([]CephCluster, 0, len(s.CephClusters))
	for _, cluster := range s.CephClusters {
		if cluster.Instance != instanceName {
			filtered = append(filtered, cluster)
		}
	}

	// Add updated clusters (if any) for this instance
	if len(clusters) > 0 {
		filtered = append(filtered, clusters...)
	}

	// Sort for stable ordering in UI
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].Instance == filtered[j].Instance {
			if filtered[i].Name == filtered[j].Name {
				return filtered[i].ID < filtered[j].ID
			}
			return filtered[i].Name < filtered[j].Name
		}
		return filtered[i].Instance < filtered[j].Instance
	})

	s.CephClusters = filtered
	s.LastUpdate = time.Now()
}

// UpdatePBSInstances updates the PBS instances in the state
func (s *State) UpdatePBSInstances(instances []PBSInstance) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.PBSInstances = instances
	s.LastUpdate = time.Now()
}

// UpdatePBSInstance updates a single PBS instance in the state, merging with existing instances
func (s *State) UpdatePBSInstance(instance PBSInstance) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Find and update existing instance or append new one
	found := false
	for i, existing := range s.PBSInstances {
		if existing.ID == instance.ID {
			s.PBSInstances[i] = instance
			found = true
			break
		}
	}

	if !found {
		s.PBSInstances = append(s.PBSInstances, instance)
	}

	s.LastUpdate = time.Now()
}

// UpdatePMGInstances replaces the entire PMG instance list
func (s *State) UpdatePMGInstances(instances []PMGInstance) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.PMGInstances = instances
	s.LastUpdate = time.Now()
}

// UpdatePMGInstance updates or inserts a PMG instance record
func (s *State) UpdatePMGInstance(instance PMGInstance) {
	s.mu.Lock()
	defer s.mu.Unlock()

	updated := false
	for i := range s.PMGInstances {
		if s.PMGInstances[i].ID == instance.ID || strings.EqualFold(s.PMGInstances[i].Name, instance.Name) {
			s.PMGInstances[i] = instance
			updated = true
			break
		}
	}

	if !updated {
		s.PMGInstances = append(s.PMGInstances, instance)
	}

	s.LastUpdate = time.Now()
}

// UpdateBackupTasksForInstance updates backup tasks for a specific instance, merging with existing tasks
func (s *State) UpdateBackupTasksForInstance(instanceName string, tasks []BackupTask) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create a map of existing tasks, excluding those from this instance
	taskMap := make(map[string]BackupTask)
	for _, task := range s.PVEBackups.BackupTasks {
		if task.Instance != instanceName {
			taskMap[task.ID] = task
		}
	}

	// Add or update tasks from this instance
	for _, task := range tasks {
		taskMap[task.ID] = task
	}

	// Convert map back to slice
	newTasks := make([]BackupTask, 0, len(taskMap))
	for _, task := range taskMap {
		newTasks = append(newTasks, task)
	}

	// Sort by start time descending
	sort.Slice(newTasks, func(i, j int) bool {
		return newTasks[i].StartTime.After(newTasks[j].StartTime)
	})

	s.PVEBackups.BackupTasks = newTasks
	s.syncBackupsLocked()
	s.LastUpdate = time.Now()
}

// UpdateStorageBackupsForInstance updates storage backups for a specific instance, merging with existing backups
func (s *State) UpdateStorageBackupsForInstance(instanceName string, backups []StorageBackup) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// When storage is shared across nodes, backups can appear under whichever node reported the content.
	// Align each backup with the guest's current node so the frontend column matches the VM/CT placement.
	guestNodeByVMID := make(map[int]string)
	for _, vm := range s.VMs {
		if vm.Instance == instanceName && vm.Node != "" {
			guestNodeByVMID[vm.VMID] = vm.Node
		}
	}
	for _, ct := range s.Containers {
		if ct.Instance == instanceName && ct.Node != "" {
			guestNodeByVMID[ct.VMID] = ct.Node
		}
	}

	normalizedBackups := make([]StorageBackup, 0, len(backups))
	for _, backup := range backups {
		if backup.VMID > 0 {
			if node, ok := guestNodeByVMID[backup.VMID]; ok {
				backup.Node = node
			}
		}
		normalizedBackups = append(normalizedBackups, backup)
	}

	// Create a map of existing backups, excluding those from this instance
	backupMap := make(map[string]StorageBackup)
	for _, backup := range s.PVEBackups.StorageBackups {
		if backup.Instance != instanceName {
			backupMap[backup.ID] = backup
		}
	}

	// Add or update backups from this instance
	for _, backup := range normalizedBackups {
		backupMap[backup.ID] = backup
	}

	// Convert map back to slice
	newBackups := make([]StorageBackup, 0, len(backupMap))
	for _, backup := range backupMap {
		newBackups = append(newBackups, backup)
	}

	// Sort by time descending
	sort.Slice(newBackups, func(i, j int) bool {
		return newBackups[i].Time.After(newBackups[j].Time)
	})

	s.PVEBackups.StorageBackups = newBackups
	s.syncBackupsLocked()
	s.LastUpdate = time.Now()
}

// UpdateReplicationJobsForInstance updates replication jobs for a specific instance.
func (s *State) UpdateReplicationJobsForInstance(instanceName string, jobs []ReplicationJob) {
	s.mu.Lock()
	defer s.mu.Unlock()

	filtered := make([]ReplicationJob, 0, len(s.ReplicationJobs))
	for _, job := range s.ReplicationJobs {
		if job.Instance != instanceName {
			filtered = append(filtered, job)
		}
	}

	now := time.Now()
	for _, job := range jobs {
		if job.Instance == "" {
			job.Instance = instanceName
		}
		if job.JobID == "" {
			job.JobID = job.ID
		}
		if job.LastPolled.IsZero() {
			job.LastPolled = now
		}
		filtered = append(filtered, job)
	}

	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].Instance == filtered[j].Instance {
			if filtered[i].GuestID == filtered[j].GuestID {
				if filtered[i].JobNumber == filtered[j].JobNumber {
					if filtered[i].JobID == filtered[j].JobID {
						return filtered[i].ID < filtered[j].ID
					}
					return filtered[i].JobID < filtered[j].JobID
				}
				return filtered[i].JobNumber < filtered[j].JobNumber
			}
			if filtered[i].GuestID == 0 || filtered[j].GuestID == 0 {
				return filtered[i].Guest < filtered[j].Guest
			}
			return filtered[i].GuestID < filtered[j].GuestID
		}
		return filtered[i].Instance < filtered[j].Instance
	})

	s.ReplicationJobs = filtered
	s.LastUpdate = now
}

// UpdateGuestSnapshotsForInstance updates guest snapshots for a specific instance, merging with existing snapshots
func (s *State) UpdateGuestSnapshotsForInstance(instanceName string, snapshots []GuestSnapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create a map of existing snapshots, excluding those from this instance
	snapshotMap := make(map[string]GuestSnapshot)
	for _, snapshot := range s.PVEBackups.GuestSnapshots {
		if snapshot.Instance != instanceName {
			snapshotMap[snapshot.ID] = snapshot
		}
	}

	// Add or update snapshots from this instance
	for _, snapshot := range snapshots {
		snapshotMap[snapshot.ID] = snapshot
	}

	// Convert map back to slice
	newSnapshots := make([]GuestSnapshot, 0, len(snapshotMap))
	for _, snapshot := range snapshotMap {
		newSnapshots = append(newSnapshots, snapshot)
	}

	// Sort by time descending
	sort.Slice(newSnapshots, func(i, j int) bool {
		return newSnapshots[i].Time.After(newSnapshots[j].Time)
	})

	s.PVEBackups.GuestSnapshots = newSnapshots
	s.syncBackupsLocked()
	s.LastUpdate = time.Now()
}

// SetConnectionHealth updates the connection health for an instance
func (s *State) SetConnectionHealth(instanceID string, healthy bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ConnectionHealth[instanceID] = healthy
}

// RemoveConnectionHealth removes a connection health entry if it exists.
func (s *State) RemoveConnectionHealth(instanceID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.ConnectionHealth, instanceID)
}

// UpdatePBSBackups updates PBS backups for a specific instance
func (s *State) UpdatePBSBackups(instanceName string, backups []PBSBackup) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create a map of existing backups excluding ones from this instance
	backupMap := make(map[string]PBSBackup)
	for _, backup := range s.PBSBackups {
		if backup.Instance != instanceName {
			backupMap[backup.ID] = backup
		}
	}

	// Add new backups from this instance
	for _, backup := range backups {
		backupMap[backup.ID] = backup
	}

	// Convert map back to slice
	newBackups := make([]PBSBackup, 0, len(backupMap))
	for _, backup := range backupMap {
		newBackups = append(newBackups, backup)
	}

	// Sort by backup time (newest first)
	sort.Slice(newBackups, func(i, j int) bool {
		return newBackups[i].BackupTime.After(newBackups[j].BackupTime)
	})

	s.PBSBackups = newBackups
	s.syncBackupsLocked()
	s.LastUpdate = time.Now()
}

// UpdatePMGBackups updates PMG backups for a specific instance.
func (s *State) UpdatePMGBackups(instanceName string, backups []PMGBackup) {
	s.mu.Lock()
	defer s.mu.Unlock()

	combined := make([]PMGBackup, 0, len(s.PMGBackups)+len(backups))
	for _, backup := range s.PMGBackups {
		if backup.Instance != instanceName {
			combined = append(combined, backup)
		}
	}
	if len(backups) > 0 {
		combined = append(combined, backups...)
	}

	if len(combined) > 1 {
		sort.Slice(combined, func(i, j int) bool {
			return combined[i].BackupTime.After(combined[j].BackupTime)
		})
	}

	s.PMGBackups = combined
	s.syncBackupsLocked()
	s.LastUpdate = time.Now()
}

// GetContainers returns a copy of all LXC containers.
func (s *State) GetContainers() []Container {
	s.mu.RLock()
	defer s.mu.RUnlock()

	containers := make([]Container, len(s.Containers))
	copy(containers, s.Containers)
	return containers
}

// UpdateContainerDockerStatus updates the Docker detection status for a specific container.
func (s *State) UpdateContainerDockerStatus(containerID string, hasDocker bool, checkedAt time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.Containers {
		if s.Containers[i].ID == containerID {
			s.Containers[i].HasDocker = hasDocker
			s.Containers[i].DockerCheckedAt = checkedAt
			return true
		}
	}
	return false
}
