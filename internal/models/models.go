package models

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/proxmoxidentity"
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
	RemovedHostAgents            []RemovedHostAgent         `json:"removedHostAgents"`
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
	PVETagColors                 map[string]string          `json:"pveTagColors,omitempty"`
	PVETagStyles                 map[string]PVETagStyle     `json:"pveTagStyles,omitempty"`
}

// PVETagStyle is the frontend-facing Proxmox tag style for one PVE instance.
type PVETagStyle struct {
	Colors        map[string]string `json:"colors"`
	CaseSensitive bool              `json:"caseSensitive"`
}

var (
	// ErrHostAgentNotFound indicates a requested host agent ID does not exist in state.
	ErrHostAgentNotFound = errors.New("host agent not found")
	// ErrNodeNotFound indicates a requested node ID does not exist in state.
	ErrNodeNotFound = errors.New("node not found")
)

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
	// Metadata carries alert-engine annotations (notably resourceType) so the
	// frontend can classify an alert without re-deriving resource identity.
	Metadata map[string]interface{} `json:"metadata,omitempty"`
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
	LinkedAgentID string `json:"linkedAgentId,omitempty"` // ID of the host agent running on this node
}

func (n Node) NormalizeCollections() Node {
	if n.LoadAverage == nil {
		n.LoadAverage = []float64{}
	}
	if n.Temperature != nil {
		temp := n.Temperature.NormalizeCollections()
		n.Temperature = &temp
	}
	return n
}

// VM represents a virtual machine
type VM struct {
	ID                string                  `json:"id"`
	VMID              int                     `json:"vmid"`
	Name              string                  `json:"name"`
	Node              string                  `json:"node"`
	Pool              string                  `json:"pool,omitempty"`
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
	OnBoot            *bool                   `json:"onBoot,omitempty"`
	LastBackup        time.Time               `json:"lastBackup,omitempty"`
	Tags              []string                `json:"tags,omitempty"`
	Lock              string                  `json:"lock,omitempty"`
	LastSeen          time.Time               `json:"lastSeen"`
}

func (v VM) NormalizeCollections() VM {
	if v.Disks == nil {
		v.Disks = []Disk{}
	}
	if v.IPAddresses == nil {
		v.IPAddresses = []string{}
	}
	if v.NetworkInterfaces == nil {
		v.NetworkInterfaces = []GuestNetworkInterface{}
	}
	for i := range v.NetworkInterfaces {
		v.NetworkInterfaces[i] = v.NetworkInterfaces[i].NormalizeCollections()
	}
	if v.Tags == nil {
		v.Tags = []string{}
	}
	return v
}

// Container represents an LXC container
type Container struct {
	ID                string                  `json:"id"`
	VMID              int                     `json:"vmid"`
	Name              string                  `json:"name"`
	Node              string                  `json:"node"`
	Pool              string                  `json:"pool,omitempty"`
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
	OnBoot            *bool                   `json:"onBoot,omitempty"`
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

func (c Container) NormalizeCollections() Container {
	if c.Disks == nil {
		c.Disks = []Disk{}
	}
	if c.IPAddresses == nil {
		c.IPAddresses = []string{}
	}
	if c.NetworkInterfaces == nil {
		c.NetworkInterfaces = []GuestNetworkInterface{}
	}
	for i := range c.NetworkInterfaces {
		c.NetworkInterfaces[i] = c.NetworkInterfaces[i].NormalizeCollections()
	}
	if c.Tags == nil {
		c.Tags = []string{}
	}
	return c
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
	Unraid            *HostUnraidStorage     `json:"unraid,omitempty"`
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
	DiskExclude       []string               `json:"diskExclude,omitempty"` // Agent's --disk-exclude patterns
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

func (h Host) NormalizeCollections() Host {
	if h.LoadAverage == nil {
		h.LoadAverage = []float64{}
	}
	if h.Disks == nil {
		h.Disks = []Disk{}
	}
	if h.DiskIO == nil {
		h.DiskIO = []DiskIO{}
	}
	if h.NetworkInterfaces == nil {
		h.NetworkInterfaces = []HostNetworkInterface{}
	}
	for i := range h.NetworkInterfaces {
		h.NetworkInterfaces[i] = h.NetworkInterfaces[i].NormalizeCollections()
	}
	h.Sensors = h.Sensors.NormalizeCollections()
	if h.RAID == nil {
		h.RAID = []HostRAIDArray{}
	}
	for i := range h.RAID {
		h.RAID[i] = h.RAID[i].NormalizeCollections()
	}
	if h.Unraid != nil {
		unraid := h.Unraid.NormalizeCollections()
		h.Unraid = &unraid
	}
	if h.Ceph != nil {
		ceph := h.Ceph.NormalizeCollections()
		h.Ceph = &ceph
	}
	if h.Tags == nil {
		h.Tags = []string{}
	}
	if h.DiskExclude == nil {
		h.DiskExclude = []string{}
	}
	return h
}

// HostNetworkInterface describes a host network adapter summary.
type HostNetworkInterface struct {
	Name      string   `json:"name"`
	MAC       string   `json:"mac,omitempty"`
	Addresses []string `json:"addresses"`
	RXBytes   uint64   `json:"rxBytes,omitempty"`
	TXBytes   uint64   `json:"txBytes,omitempty"`
	SpeedMbps *int64   `json:"speedMbps,omitempty"`
}

func (i HostNetworkInterface) NormalizeCollections() HostNetworkInterface {
	if i.Addresses == nil {
		i.Addresses = []string{}
	}
	return i
}

// HostSensorSummary captures optional per-host sensor readings.
type HostSensorSummary struct {
	TemperatureCelsius map[string]float64 `json:"temperatureCelsius,omitempty"`
	FanRPM             map[string]float64 `json:"fanRpm,omitempty"`
	PowerWatts         map[string]float64 `json:"powerWatts,omitempty"`
	Additional         map[string]float64 `json:"additional,omitempty"`
	GPU                []HostGPUSensor    `json:"gpu,omitempty"`
	ThermalState       *HostThermalState  `json:"thermalState,omitempty"`
	SMART              []HostDiskSMART    `json:"smart,omitempty"` // S.M.A.R.T. disk data
}

type HostGPUSensor struct {
	ID                 string   `json:"id,omitempty"`
	Name               string   `json:"name,omitempty"`
	TemperatureCelsius *float64 `json:"temperatureCelsius,omitempty"`
	UtilizationPercent *float64 `json:"utilizationPercent,omitempty"`
	MemoryUsedBytes    *int64   `json:"memoryUsedBytes,omitempty"`
	MemoryTotalBytes   *int64   `json:"memoryTotalBytes,omitempty"`
}

type HostThermalState struct {
	Source                  string         `json:"source,omitempty"`
	Pressure                string         `json:"pressure,omitempty"`
	ThermalWarningLevel     *int           `json:"thermalWarningLevel,omitempty"`
	PerformanceWarningLevel *int           `json:"performanceWarningLevel,omitempty"`
	CPUPowerStatus          *int           `json:"cpuPowerStatus,omitempty"`
	LimitsPercent           map[string]int `json:"limitsPercent,omitempty"`
}

func (s HostSensorSummary) NormalizeCollections() HostSensorSummary {
	if s.TemperatureCelsius == nil {
		s.TemperatureCelsius = map[string]float64{}
	}
	if s.FanRPM == nil {
		s.FanRPM = map[string]float64{}
	}
	if s.PowerWatts == nil {
		s.PowerWatts = map[string]float64{}
	}
	if s.Additional == nil {
		s.Additional = map[string]float64{}
	}
	if s.GPU == nil {
		s.GPU = []HostGPUSensor{}
	}
	if s.SMART == nil {
		s.SMART = []HostDiskSMART{}
	}
	return s
}

// HostDiskSMART represents S.M.A.R.T. data for a disk from a host agent.
type HostDiskSMART struct {
	Device      string           `json:"device"`              // Block device name (e.g., sda, nvme0n1)
	Model       string           `json:"model,omitempty"`     // Disk model
	Serial      string           `json:"serial,omitempty"`    // Serial number
	WWN         string           `json:"wwn,omitempty"`       // World Wide Name
	Type        string           `json:"type,omitempty"`      // Transport type: sata, sas, nvme
	SizeBytes   int64            `json:"sizeBytes,omitempty"` // Capacity in bytes (0 when unknown)
	Temperature int              `json:"temperature"`         // Temperature in Celsius
	Health      string           `json:"health,omitempty"`    // PASSED, FAILED, UNKNOWN
	Standby     bool             `json:"standby,omitempty"`   // True if disk was in standby
	Pool        string           `json:"pool,omitempty"`      // ZFS pool this disk belongs to (empty if not a ZFS member)
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
	// Operation is the in-progress sync action from /proc/mdstat
	// ("recovery", "resync", "check", or "reshape"); empty when idle.
	Operation string `json:"operation,omitempty"`
}

func (a HostRAIDArray) NormalizeCollections() HostRAIDArray {
	if a.Devices == nil {
		a.Devices = []HostRAIDDevice{}
	}
	return a
}

// HostRAIDDevice represents a device in a RAID array.
type HostRAIDDevice struct {
	Device string `json:"device"`
	State  string `json:"state"`
	Slot   int    `json:"slot"`
}

// HostUnraidStorage represents best-effort Unraid array topology from the agent.
type HostUnraidStorage struct {
	ArrayStarted bool             `json:"arrayStarted"`
	ArrayState   string           `json:"arrayState,omitempty"`
	SyncAction   string           `json:"syncAction,omitempty"`
	SyncProgress float64          `json:"syncProgress,omitempty"`
	SyncErrors   int64            `json:"syncErrors,omitempty"`
	NumProtected int              `json:"numProtected,omitempty"`
	NumDisabled  int              `json:"numDisabled,omitempty"`
	NumInvalid   int              `json:"numInvalid,omitempty"`
	NumMissing   int              `json:"numMissing,omitempty"`
	Disks        []HostUnraidDisk `json:"disks,omitempty"`
}

func (s HostUnraidStorage) NormalizeCollections() HostUnraidStorage {
	if s.Disks == nil {
		s.Disks = []HostUnraidDisk{}
	}
	return s
}

// HostUnraidDisk represents a disk's role and state inside an Unraid array.
type HostUnraidDisk struct {
	Name        string `json:"name"`
	Device      string `json:"device,omitempty"`
	Role        string `json:"role,omitempty"`
	Status      string `json:"status,omitempty"`
	RawStatus   string `json:"rawStatus,omitempty"`
	Model       string `json:"model,omitempty"`
	Serial      string `json:"serial,omitempty"`
	Filesystem  string `json:"filesystem,omitempty"`
	Transport   string `json:"transport,omitempty"`
	SizeBytes   int64  `json:"sizeBytes,omitempty"`
	UsedBytes   int64  `json:"usedBytes,omitempty"`
	FreeBytes   int64  `json:"freeBytes,omitempty"`
	Temperature int    `json:"temperature,omitempty"`
	SpunDown    bool   `json:"spunDown,omitempty"`
	ReadCount   int64  `json:"readCount,omitempty"`
	WriteCount  int64  `json:"writeCount,omitempty"`
	ErrorCount  int64  `json:"errorCount,omitempty"`
	Slot        int    `json:"slot,omitempty"`
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

func (c HostCephCluster) NormalizeCollections() HostCephCluster {
	c.Health = c.Health.NormalizeCollections()
	c.MonMap = c.MonMap.NormalizeCollections()
	if c.Pools == nil {
		c.Pools = []HostCephPool{}
	}
	if c.Services == nil {
		c.Services = []HostCephService{}
	}
	for i := range c.Services {
		c.Services[i] = c.Services[i].NormalizeCollections()
	}
	return c
}

// HostCephHealth represents Ceph cluster health status.
type HostCephHealth struct {
	Status  string                   `json:"status"` // HEALTH_OK, HEALTH_WARN, HEALTH_ERR
	Checks  map[string]HostCephCheck `json:"checks,omitempty"`
	Summary []HostCephHealthSummary  `json:"summary,omitempty"`
}

func (h HostCephHealth) NormalizeCollections() HostCephHealth {
	if h.Checks == nil {
		h.Checks = map[string]HostCephCheck{}
	}
	for key, check := range h.Checks {
		h.Checks[key] = check.NormalizeCollections()
	}
	if h.Summary == nil {
		h.Summary = []HostCephHealthSummary{}
	}
	return h
}

// HostCephCheck represents a health check detail.
type HostCephCheck struct {
	Severity string   `json:"severity"`
	Message  string   `json:"message,omitempty"`
	Detail   []string `json:"detail,omitempty"`
}

func (c HostCephCheck) NormalizeCollections() HostCephCheck {
	if c.Detail == nil {
		c.Detail = []string{}
	}
	return c
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

func (m HostCephMonitorMap) NormalizeCollections() HostCephMonitorMap {
	if m.Monitors == nil {
		m.Monitors = []HostCephMonitor{}
	}
	return m
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

func (s HostCephService) NormalizeCollections() HostCephService {
	if s.Daemons == nil {
		s.Daemons = []string{}
	}
	return s
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
	Images            []DockerImage            `json:"images,omitempty"`
	Volumes           []DockerVolume           `json:"volumes,omitempty"`
	Networks          []DockerNetwork          `json:"networks,omitempty"`
	Services          []DockerService          `json:"services,omitempty"`
	Tasks             []DockerTask             `json:"tasks,omitempty"`
	Nodes             []DockerNode             `json:"nodes,omitempty"`
	Secrets           []DockerSecret           `json:"secrets,omitempty"`
	Configs           []DockerConfig           `json:"configs,omitempty"`
	StorageUsage      *DockerStorageUsage      `json:"storageUsage,omitempty"`
	Swarm             *DockerSwarmInfo         `json:"swarm,omitempty"`
	Security          *DockerHostSecurity      `json:"security,omitempty"`
	Temperature       *float64                 `json:"temperature,omitempty"` // Optional host temperature in Celsius
	TokenID           string                   `json:"tokenId,omitempty"`
	TokenName         string                   `json:"tokenName,omitempty"`
	TokenHint         string                   `json:"tokenHint,omitempty"`
	TokenLastUsedAt   *time.Time               `json:"tokenLastUsedAt,omitempty"`
	Hidden            bool                     `json:"hidden"`
	PendingUninstall  bool                     `json:"pendingUninstall"`
	Command           *DockerHostCommandStatus `json:"command,omitempty"`
	IsLegacy          bool                     `json:"isLegacy,omitempty"`

	// Computed I/O rates (bytes/sec), populated by monitoring pipeline when available.
	NetInRate     float64 `json:"netInRate,omitempty"`
	NetOutRate    float64 `json:"netOutRate,omitempty"`
	DiskReadRate  float64 `json:"diskReadRate,omitempty"`
	DiskWriteRate float64 `json:"diskWriteRate,omitempty"`
}

func (h DockerHost) NormalizeCollections() DockerHost {
	if h.LoadAverage == nil {
		h.LoadAverage = []float64{}
	}
	if h.Disks == nil {
		h.Disks = []Disk{}
	}
	if h.NetworkInterfaces == nil {
		h.NetworkInterfaces = []HostNetworkInterface{}
	}
	for i := range h.NetworkInterfaces {
		h.NetworkInterfaces[i] = h.NetworkInterfaces[i].NormalizeCollections()
	}
	if h.Containers == nil {
		h.Containers = []DockerContainer{}
	}
	for i := range h.Containers {
		h.Containers[i] = h.Containers[i].NormalizeCollections()
	}
	if h.Images == nil {
		h.Images = []DockerImage{}
	}
	for i := range h.Images {
		h.Images[i] = h.Images[i].NormalizeCollections()
	}
	if h.Volumes == nil {
		h.Volumes = []DockerVolume{}
	}
	for i := range h.Volumes {
		h.Volumes[i] = h.Volumes[i].NormalizeCollections()
	}
	if h.Networks == nil {
		h.Networks = []DockerNetwork{}
	}
	for i := range h.Networks {
		h.Networks[i] = h.Networks[i].NormalizeCollections()
	}
	if h.Services == nil {
		h.Services = []DockerService{}
	}
	for i := range h.Services {
		h.Services[i] = h.Services[i].NormalizeCollections()
	}
	if h.Tasks == nil {
		h.Tasks = []DockerTask{}
	}
	if h.Nodes == nil {
		h.Nodes = []DockerNode{}
	}
	for i := range h.Nodes {
		h.Nodes[i] = h.Nodes[i].NormalizeCollections()
	}
	if h.Secrets == nil {
		h.Secrets = []DockerSecret{}
	}
	for i := range h.Secrets {
		h.Secrets[i] = h.Secrets[i].NormalizeCollections()
	}
	if h.Configs == nil {
		h.Configs = []DockerConfig{}
	}
	for i := range h.Configs {
		h.Configs[i] = h.Configs[i].NormalizeCollections()
	}
	if h.Security != nil {
		security := h.Security.NormalizeCollections()
		h.Security = &security
	}
	return h
}

// DockerHostSecurity describes security-relevant runtime posture for a Docker host.
type DockerHostSecurity struct {
	AuthorizationPlugins          []string `json:"authorizationPlugins,omitempty"`
	MutatingCommandsBlocked       bool     `json:"mutatingCommandsBlocked,omitempty"`
	MutatingCommandsBlockedReason string   `json:"mutatingCommandsBlockedReason,omitempty"`
}

func (s DockerHostSecurity) NormalizeCollections() DockerHostSecurity {
	if s.AuthorizationPlugins == nil {
		s.AuthorizationPlugins = []string{}
	}
	return s
}

// RemovedDockerHost tracks a docker host that was deliberately removed and blocked from reporting.
type RemovedDockerHost struct {
	ID          string    `json:"id"`
	Hostname    string    `json:"hostname,omitempty"`
	DisplayName string    `json:"displayName,omitempty"`
	RemovedAt   time.Time `json:"removedAt"`
}

// RemovedHostAgent tracks a host agent that was deliberately removed and blocked from reporting.
type RemovedHostAgent struct {
	ID                string    `json:"id"`
	Hostname          string    `json:"hostname,omitempty"`
	DisplayName       string    `json:"displayName,omitempty"`
	LinkedVMID        string    `json:"linkedVmId,omitempty"`
	LinkedContainerID string    `json:"linkedContainerId,omitempty"`
	RemovedAt         time.Time `json:"removedAt"`
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
	NetworkRXBytes      uint64                       `json:"networkRxBytes,omitempty"`
	NetworkTXBytes      uint64                       `json:"networkTxBytes,omitempty"`
	NetInRate           float64                      `json:"netInRate,omitempty"`
	NetOutRate          float64                      `json:"netOutRate,omitempty"`
	WritableLayerBytes  int64                        `json:"writableLayerBytes,omitempty"`
	RootFilesystemBytes int64                        `json:"rootFilesystemBytes,omitempty"`
	BlockIO             *DockerContainerBlockIO      `json:"blockIo,omitempty"`
	Mounts              []DockerContainerMount       `json:"mounts,omitempty"`
	Podman              *DockerPodmanContainer       `json:"podman,omitempty"`
	UpdateStatus        *DockerContainerUpdateStatus `json:"updateStatus,omitempty"` // Image update detection status
}

func (c DockerContainer) NormalizeCollections() DockerContainer {
	if c.Ports == nil {
		c.Ports = []DockerContainerPort{}
	}
	if c.Labels == nil {
		c.Labels = map[string]string{}
	}
	if c.Networks == nil {
		c.Networks = []DockerContainerNetworkLink{}
	}
	if c.Mounts == nil {
		c.Mounts = []DockerContainerMount{}
	}
	return c
}

// DockerImage represents a local image on a monitored Docker-compatible host.
type DockerImage struct {
	ID              string            `json:"id"`
	RepoTags        []string          `json:"repoTags,omitempty"`
	RepoDigests     []string          `json:"repoDigests,omitempty"`
	SizeBytes       int64             `json:"sizeBytes,omitempty"`
	SharedSizeBytes int64             `json:"sharedSizeBytes,omitempty"`
	Containers      int64             `json:"containers,omitempty"`
	CreatedAt       time.Time         `json:"createdAt,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
}

func (i DockerImage) NormalizeCollections() DockerImage {
	if i.RepoTags == nil {
		i.RepoTags = []string{}
	}
	if i.RepoDigests == nil {
		i.RepoDigests = []string{}
	}
	if i.Labels == nil {
		i.Labels = map[string]string{}
	}
	return i
}

// DockerVolume represents a local volume on a monitored Docker-compatible host.
type DockerVolume struct {
	Name       string            `json:"name"`
	Driver     string            `json:"driver,omitempty"`
	Mountpoint string            `json:"mountpoint,omitempty"`
	Scope      string            `json:"scope,omitempty"`
	CreatedAt  string            `json:"createdAt,omitempty"`
	SizeBytes  int64             `json:"sizeBytes,omitempty"`
	RefCount   int64             `json:"refCount,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
	Options    map[string]string `json:"options,omitempty"`
}

func (v DockerVolume) NormalizeCollections() DockerVolume {
	if v.Labels == nil {
		v.Labels = map[string]string{}
	}
	if v.Options == nil {
		v.Options = map[string]string{}
	}
	return v
}

// DockerNetwork represents a local network on a monitored Docker-compatible host.
type DockerNetwork struct {
	ID         string                `json:"id"`
	Name       string                `json:"name"`
	Driver     string                `json:"driver,omitempty"`
	Scope      string                `json:"scope,omitempty"`
	CreatedAt  time.Time             `json:"createdAt,omitempty"`
	EnableIPv4 bool                  `json:"enableIpv4,omitempty"`
	EnableIPv6 bool                  `json:"enableIpv6,omitempty"`
	Internal   bool                  `json:"internal,omitempty"`
	Attachable bool                  `json:"attachable,omitempty"`
	Ingress    bool                  `json:"ingress,omitempty"`
	ConfigOnly bool                  `json:"configOnly,omitempty"`
	Subnets    []DockerNetworkSubnet `json:"subnets,omitempty"`
	Labels     map[string]string     `json:"labels,omitempty"`
	Options    map[string]string     `json:"options,omitempty"`
}

func (n DockerNetwork) NormalizeCollections() DockerNetwork {
	if n.Subnets == nil {
		n.Subnets = []DockerNetworkSubnet{}
	}
	if n.Labels == nil {
		n.Labels = map[string]string{}
	}
	if n.Options == nil {
		n.Options = map[string]string{}
	}
	return n
}

// DockerNetworkSubnet describes one Docker network IPAM subnet.
type DockerNetworkSubnet struct {
	Subnet  string `json:"subnet,omitempty"`
	Gateway string `json:"gateway,omitempty"`
}

// DockerStorageUsage summarises Docker-compatible /system/df usage.
type DockerStorageUsage struct {
	Images     DockerStorageUsageBucket `json:"images,omitempty"`
	Containers DockerStorageUsageBucket `json:"containers,omitempty"`
	Volumes    DockerStorageUsageBucket `json:"volumes,omitempty"`
	BuildCache DockerStorageUsageBucket `json:"buildCache,omitempty"`
}

// DockerStorageUsageBucket captures one /system/df resource bucket.
type DockerStorageUsageBucket struct {
	TotalCount       int64 `json:"totalCount,omitempty"`
	ActiveCount      int64 `json:"activeCount,omitempty"`
	TotalSizeBytes   int64 `json:"totalSizeBytes,omitempty"`
	ReclaimableBytes int64 `json:"reclaimableBytes,omitempty"`
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

	Nodes                    []KubernetesNode                    `json:"nodes,omitempty"`
	Namespaces               []KubernetesNamespace               `json:"namespaces,omitempty"`
	Pods                     []KubernetesPod                     `json:"pods,omitempty"`
	Deployments              []KubernetesDeployment              `json:"deployments,omitempty"`
	ReplicaSets              []KubernetesReplicaSet              `json:"replicaSets,omitempty"`
	StatefulSets             []KubernetesStatefulSet             `json:"statefulSets,omitempty"`
	DaemonSets               []KubernetesDaemonSet               `json:"daemonSets,omitempty"`
	Services                 []KubernetesService                 `json:"services,omitempty"`
	Jobs                     []KubernetesJob                     `json:"jobs,omitempty"`
	CronJobs                 []KubernetesCronJob                 `json:"cronJobs,omitempty"`
	Ingresses                []KubernetesIngress                 `json:"ingresses,omitempty"`
	EndpointSlices           []KubernetesEndpointSlice           `json:"endpointSlices,omitempty"`
	NetworkPolicies          []KubernetesNetworkPolicy           `json:"networkPolicies,omitempty"`
	PersistentVolumes        []KubernetesPersistentVolume        `json:"persistentVolumes,omitempty"`
	PersistentVolumeClaims   []KubernetesPersistentVolumeClaim   `json:"persistentVolumeClaims,omitempty"`
	StorageClasses           []KubernetesStorageClass            `json:"storageClasses,omitempty"`
	ConfigMaps               []KubernetesConfigMap               `json:"configMaps,omitempty"`
	Secrets                  []KubernetesSecret                  `json:"secrets,omitempty"`
	ServiceAccounts          []KubernetesServiceAccount          `json:"serviceAccounts,omitempty"`
	Roles                    []KubernetesRole                    `json:"roles,omitempty"`
	ClusterRoles             []KubernetesClusterRole             `json:"clusterRoles,omitempty"`
	RoleBindings             []KubernetesRoleBinding             `json:"roleBindings,omitempty"`
	ClusterRoleBindings      []KubernetesClusterRoleBinding      `json:"clusterRoleBindings,omitempty"`
	ResourceQuotas           []KubernetesResourceQuota           `json:"resourceQuotas,omitempty"`
	LimitRanges              []KubernetesLimitRange              `json:"limitRanges,omitempty"`
	PodDisruptionBudgets     []KubernetesPodDisruptionBudget     `json:"podDisruptionBudgets,omitempty"`
	HorizontalPodAutoscalers []KubernetesHorizontalPodAutoscaler `json:"horizontalPodAutoscalers,omitempty"`
	Events                   []KubernetesEvent                   `json:"events,omitempty"`

	// Token information
	TokenID         string     `json:"tokenId,omitempty"`
	TokenName       string     `json:"tokenName,omitempty"`
	TokenHint       string     `json:"tokenHint,omitempty"`
	TokenLastUsedAt *time.Time `json:"tokenLastUsedAt,omitempty"`

	Hidden           bool `json:"hidden"`
	PendingUninstall bool `json:"pendingUninstall"`
}

func (c KubernetesCluster) NormalizeCollections() KubernetesCluster {
	if c.Nodes == nil {
		c.Nodes = []KubernetesNode{}
	}
	for i := range c.Nodes {
		c.Nodes[i] = c.Nodes[i].NormalizeCollections()
	}
	if c.Namespaces == nil {
		c.Namespaces = []KubernetesNamespace{}
	}
	for i := range c.Namespaces {
		c.Namespaces[i] = c.Namespaces[i].NormalizeCollections()
	}
	if c.Pods == nil {
		c.Pods = []KubernetesPod{}
	}
	for i := range c.Pods {
		c.Pods[i] = c.Pods[i].NormalizeCollections()
	}
	if c.Deployments == nil {
		c.Deployments = []KubernetesDeployment{}
	}
	for i := range c.Deployments {
		c.Deployments[i] = c.Deployments[i].NormalizeCollections()
	}
	if c.ReplicaSets == nil {
		c.ReplicaSets = []KubernetesReplicaSet{}
	}
	for i := range c.ReplicaSets {
		c.ReplicaSets[i] = c.ReplicaSets[i].NormalizeCollections()
	}
	if c.StatefulSets == nil {
		c.StatefulSets = []KubernetesStatefulSet{}
	}
	for i := range c.StatefulSets {
		c.StatefulSets[i] = c.StatefulSets[i].NormalizeCollections()
	}
	if c.DaemonSets == nil {
		c.DaemonSets = []KubernetesDaemonSet{}
	}
	for i := range c.DaemonSets {
		c.DaemonSets[i] = c.DaemonSets[i].NormalizeCollections()
	}
	if c.Services == nil {
		c.Services = []KubernetesService{}
	}
	for i := range c.Services {
		c.Services[i] = c.Services[i].NormalizeCollections()
	}
	if c.Jobs == nil {
		c.Jobs = []KubernetesJob{}
	}
	for i := range c.Jobs {
		c.Jobs[i] = c.Jobs[i].NormalizeCollections()
	}
	if c.CronJobs == nil {
		c.CronJobs = []KubernetesCronJob{}
	}
	for i := range c.CronJobs {
		c.CronJobs[i] = c.CronJobs[i].NormalizeCollections()
	}
	if c.Ingresses == nil {
		c.Ingresses = []KubernetesIngress{}
	}
	for i := range c.Ingresses {
		c.Ingresses[i] = c.Ingresses[i].NormalizeCollections()
	}
	if c.EndpointSlices == nil {
		c.EndpointSlices = []KubernetesEndpointSlice{}
	}
	for i := range c.EndpointSlices {
		c.EndpointSlices[i] = c.EndpointSlices[i].NormalizeCollections()
	}
	if c.NetworkPolicies == nil {
		c.NetworkPolicies = []KubernetesNetworkPolicy{}
	}
	for i := range c.NetworkPolicies {
		c.NetworkPolicies[i] = c.NetworkPolicies[i].NormalizeCollections()
	}
	if c.PersistentVolumes == nil {
		c.PersistentVolumes = []KubernetesPersistentVolume{}
	}
	for i := range c.PersistentVolumes {
		c.PersistentVolumes[i] = c.PersistentVolumes[i].NormalizeCollections()
	}
	if c.PersistentVolumeClaims == nil {
		c.PersistentVolumeClaims = []KubernetesPersistentVolumeClaim{}
	}
	for i := range c.PersistentVolumeClaims {
		c.PersistentVolumeClaims[i] = c.PersistentVolumeClaims[i].NormalizeCollections()
	}
	if c.StorageClasses == nil {
		c.StorageClasses = []KubernetesStorageClass{}
	}
	for i := range c.StorageClasses {
		c.StorageClasses[i] = c.StorageClasses[i].NormalizeCollections()
	}
	if c.ConfigMaps == nil {
		c.ConfigMaps = []KubernetesConfigMap{}
	}
	for i := range c.ConfigMaps {
		c.ConfigMaps[i] = c.ConfigMaps[i].NormalizeCollections()
	}
	if c.Secrets == nil {
		c.Secrets = []KubernetesSecret{}
	}
	for i := range c.Secrets {
		c.Secrets[i] = c.Secrets[i].NormalizeCollections()
	}
	if c.ServiceAccounts == nil {
		c.ServiceAccounts = []KubernetesServiceAccount{}
	}
	for i := range c.ServiceAccounts {
		c.ServiceAccounts[i] = c.ServiceAccounts[i].NormalizeCollections()
	}
	if c.Roles == nil {
		c.Roles = []KubernetesRole{}
	}
	for i := range c.Roles {
		c.Roles[i] = c.Roles[i].NormalizeCollections()
	}
	if c.ClusterRoles == nil {
		c.ClusterRoles = []KubernetesClusterRole{}
	}
	for i := range c.ClusterRoles {
		c.ClusterRoles[i] = c.ClusterRoles[i].NormalizeCollections()
	}
	if c.RoleBindings == nil {
		c.RoleBindings = []KubernetesRoleBinding{}
	}
	for i := range c.RoleBindings {
		c.RoleBindings[i] = c.RoleBindings[i].NormalizeCollections()
	}
	if c.ClusterRoleBindings == nil {
		c.ClusterRoleBindings = []KubernetesClusterRoleBinding{}
	}
	for i := range c.ClusterRoleBindings {
		c.ClusterRoleBindings[i] = c.ClusterRoleBindings[i].NormalizeCollections()
	}
	if c.ResourceQuotas == nil {
		c.ResourceQuotas = []KubernetesResourceQuota{}
	}
	for i := range c.ResourceQuotas {
		c.ResourceQuotas[i] = c.ResourceQuotas[i].NormalizeCollections()
	}
	if c.LimitRanges == nil {
		c.LimitRanges = []KubernetesLimitRange{}
	}
	for i := range c.LimitRanges {
		c.LimitRanges[i] = c.LimitRanges[i].NormalizeCollections()
	}
	if c.PodDisruptionBudgets == nil {
		c.PodDisruptionBudgets = []KubernetesPodDisruptionBudget{}
	}
	for i := range c.PodDisruptionBudgets {
		c.PodDisruptionBudgets[i] = c.PodDisruptionBudgets[i].NormalizeCollections()
	}
	if c.HorizontalPodAutoscalers == nil {
		c.HorizontalPodAutoscalers = []KubernetesHorizontalPodAutoscaler{}
	}
	for i := range c.HorizontalPodAutoscalers {
		c.HorizontalPodAutoscalers[i] = c.HorizontalPodAutoscalers[i].NormalizeCollections()
	}
	if c.Events == nil {
		c.Events = []KubernetesEvent{}
	}
	return c
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
	UsageCPUMilliCores      int64    `json:"usageCpuMilliCores,omitempty"`
	UsageMemoryBytes        int64    `json:"usageMemoryBytes,omitempty"`
	UsageCPUPercent         float64  `json:"usageCpuPercent,omitempty"`
	UsageMemoryPercent      float64  `json:"usageMemoryPercent,omitempty"`
	Roles                   []string `json:"roles,omitempty"`
}

func (n KubernetesNode) NormalizeCollections() KubernetesNode {
	if n.Roles == nil {
		n.Roles = []string{}
	}
	return n
}

type KubernetesPod struct {
	UID                           string                   `json:"uid"`
	Name                          string                   `json:"name"`
	Namespace                     string                   `json:"namespace"`
	NodeName                      string                   `json:"nodeName,omitempty"`
	Phase                         string                   `json:"phase,omitempty"`
	Reason                        string                   `json:"reason,omitempty"`
	Message                       string                   `json:"message,omitempty"`
	QoSClass                      string                   `json:"qosClass,omitempty"`
	CreatedAt                     time.Time                `json:"createdAt,omitempty"`
	StartTime                     *time.Time               `json:"startTime,omitempty"`
	Restarts                      int                      `json:"restarts,omitempty"`
	UsageCPUMilliCores            int                      `json:"usageCpuMilliCores,omitempty"`
	UsageMemoryBytes              int64                    `json:"usageMemoryBytes,omitempty"`
	UsageCPUPercent               float64                  `json:"usageCpuPercent,omitempty"`
	UsageMemoryPercent            float64                  `json:"usageMemoryPercent,omitempty"`
	NetworkRxBytes                int64                    `json:"networkRxBytes,omitempty"`
	NetworkTxBytes                int64                    `json:"networkTxBytes,omitempty"`
	NetInRate                     float64                  `json:"netInRate,omitempty"`
	NetOutRate                    float64                  `json:"netOutRate,omitempty"`
	EphemeralStorageUsedBytes     int64                    `json:"ephemeralStorageUsedBytes,omitempty"`
	EphemeralStorageCapacityBytes int64                    `json:"ephemeralStorageCapacityBytes,omitempty"`
	DiskUsagePercent              float64                  `json:"diskUsagePercent,omitempty"`
	Labels                        map[string]string        `json:"labels,omitempty"`
	OwnerKind                     string                   `json:"ownerKind,omitempty"`
	OwnerName                     string                   `json:"ownerName,omitempty"`
	Containers                    []KubernetesPodContainer `json:"containers,omitempty"`
}

func (p KubernetesPod) NormalizeCollections() KubernetesPod {
	if p.Labels == nil {
		p.Labels = map[string]string{}
	}
	if p.Containers == nil {
		p.Containers = []KubernetesPodContainer{}
	}
	return p
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
	UID                string            `json:"uid"`
	Name               string            `json:"name"`
	Namespace          string            `json:"namespace"`
	CreatedAt          time.Time         `json:"createdAt,omitempty"`
	DesiredReplicas    int32             `json:"desiredReplicas,omitempty"`
	UpdatedReplicas    int32             `json:"updatedReplicas,omitempty"`
	ReadyReplicas      int32             `json:"readyReplicas,omitempty"`
	AvailableReplicas  int32             `json:"availableReplicas,omitempty"`
	ObservedGeneration int64             `json:"observedGeneration,omitempty"`
	Labels             map[string]string `json:"labels,omitempty"`
}

func (d KubernetesDeployment) NormalizeCollections() KubernetesDeployment {
	if d.Labels == nil {
		d.Labels = map[string]string{}
	}
	return d
}

type KubernetesReplicaSet struct {
	UID                  string            `json:"uid"`
	Name                 string            `json:"name"`
	Namespace            string            `json:"namespace"`
	DesiredReplicas      int32             `json:"desiredReplicas,omitempty"`
	ReadyReplicas        int32             `json:"readyReplicas,omitempty"`
	AvailableReplicas    int32             `json:"availableReplicas,omitempty"`
	FullyLabeledReplicas int32             `json:"fullyLabeledReplicas,omitempty"`
	ObservedGeneration   int64             `json:"observedGeneration,omitempty"`
	OwnerKind            string            `json:"ownerKind,omitempty"`
	OwnerName            string            `json:"ownerName,omitempty"`
	Labels               map[string]string `json:"labels,omitempty"`
}

func (r KubernetesReplicaSet) NormalizeCollections() KubernetesReplicaSet {
	if r.Labels == nil {
		r.Labels = map[string]string{}
	}
	return r
}

type KubernetesNamespace struct {
	UID       string            `json:"uid"`
	Name      string            `json:"name"`
	Phase     string            `json:"phase,omitempty"`
	CreatedAt time.Time         `json:"createdAt,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
}

func (n KubernetesNamespace) NormalizeCollections() KubernetesNamespace {
	if n.Labels == nil {
		n.Labels = map[string]string{}
	}
	return n
}

type KubernetesStatefulSet struct {
	UID               string            `json:"uid"`
	Name              string            `json:"name"`
	Namespace         string            `json:"namespace"`
	DesiredReplicas   int32             `json:"desiredReplicas,omitempty"`
	ReadyReplicas     int32             `json:"readyReplicas,omitempty"`
	CurrentReplicas   int32             `json:"currentReplicas,omitempty"`
	UpdatedReplicas   int32             `json:"updatedReplicas,omitempty"`
	AvailableReplicas int32             `json:"availableReplicas,omitempty"`
	ServiceName       string            `json:"serviceName,omitempty"`
	Labels            map[string]string `json:"labels,omitempty"`
}

func (s KubernetesStatefulSet) NormalizeCollections() KubernetesStatefulSet {
	if s.Labels == nil {
		s.Labels = map[string]string{}
	}
	return s
}

type KubernetesDaemonSet struct {
	UID                    string            `json:"uid"`
	Name                   string            `json:"name"`
	Namespace              string            `json:"namespace"`
	DesiredNumberScheduled int32             `json:"desiredNumberScheduled,omitempty"`
	CurrentNumberScheduled int32             `json:"currentNumberScheduled,omitempty"`
	NumberReady            int32             `json:"numberReady,omitempty"`
	UpdatedNumberScheduled int32             `json:"updatedNumberScheduled,omitempty"`
	NumberAvailable        int32             `json:"numberAvailable,omitempty"`
	NumberUnavailable      int32             `json:"numberUnavailable,omitempty"`
	NumberMisscheduled     int32             `json:"numberMisscheduled,omitempty"`
	Labels                 map[string]string `json:"labels,omitempty"`
}

func (d KubernetesDaemonSet) NormalizeCollections() KubernetesDaemonSet {
	if d.Labels == nil {
		d.Labels = map[string]string{}
	}
	return d
}

type KubernetesService struct {
	UID         string                  `json:"uid"`
	Name        string                  `json:"name"`
	Namespace   string                  `json:"namespace"`
	ServiceType string                  `json:"type,omitempty"`
	ClusterIP   string                  `json:"clusterIp,omitempty"`
	ExternalIPs []string                `json:"externalIps,omitempty"`
	Ports       []KubernetesServicePort `json:"ports,omitempty"`
	Selector    map[string]string       `json:"selector,omitempty"`
	CreatedAt   time.Time               `json:"createdAt,omitempty"`
	Labels      map[string]string       `json:"labels,omitempty"`
}

func (s KubernetesService) NormalizeCollections() KubernetesService {
	if s.ExternalIPs == nil {
		s.ExternalIPs = []string{}
	}
	if s.Ports == nil {
		s.Ports = []KubernetesServicePort{}
	}
	if s.Selector == nil {
		s.Selector = map[string]string{}
	}
	if s.Labels == nil {
		s.Labels = map[string]string{}
	}
	return s
}

type KubernetesServicePort struct {
	Name       string `json:"name,omitempty"`
	Protocol   string `json:"protocol,omitempty"`
	Port       int32  `json:"port,omitempty"`
	TargetPort string `json:"targetPort,omitempty"`
	NodePort   int32  `json:"nodePort,omitempty"`
}

type KubernetesJob struct {
	UID                string            `json:"uid"`
	Name               string            `json:"name"`
	Namespace          string            `json:"namespace"`
	DesiredCompletions int32             `json:"desiredCompletions,omitempty"`
	Succeeded          int32             `json:"succeeded,omitempty"`
	Failed             int32             `json:"failed,omitempty"`
	Active             int32             `json:"active,omitempty"`
	StartTime          *time.Time        `json:"startTime,omitempty"`
	CompletionTime     *time.Time        `json:"completionTime,omitempty"`
	Labels             map[string]string `json:"labels,omitempty"`
}

func (j KubernetesJob) NormalizeCollections() KubernetesJob {
	if j.Labels == nil {
		j.Labels = map[string]string{}
	}
	return j
}

type KubernetesCronJob struct {
	UID                string            `json:"uid"`
	Name               string            `json:"name"`
	Namespace          string            `json:"namespace"`
	Schedule           string            `json:"schedule,omitempty"`
	Suspend            bool              `json:"suspend,omitempty"`
	Active             int               `json:"active,omitempty"`
	LastScheduleTime   *time.Time        `json:"lastScheduleTime,omitempty"`
	LastSuccessfulTime *time.Time        `json:"lastSuccessfulTime,omitempty"`
	Labels             map[string]string `json:"labels,omitempty"`
}

func (j KubernetesCronJob) NormalizeCollections() KubernetesCronJob {
	if j.Labels == nil {
		j.Labels = map[string]string{}
	}
	return j
}

type KubernetesIngress struct {
	UID       string            `json:"uid"`
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	ClassName string            `json:"className,omitempty"`
	Hosts     []string          `json:"hosts,omitempty"`
	Addresses []string          `json:"addresses,omitempty"`
	CreatedAt time.Time         `json:"createdAt,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
}

func (i KubernetesIngress) NormalizeCollections() KubernetesIngress {
	if i.Hosts == nil {
		i.Hosts = []string{}
	}
	if i.Addresses == nil {
		i.Addresses = []string{}
	}
	if i.Labels == nil {
		i.Labels = map[string]string{}
	}
	return i
}

type KubernetesEndpointSlice struct {
	UID                string                   `json:"uid"`
	Name               string                   `json:"name"`
	Namespace          string                   `json:"namespace"`
	AddressType        string                   `json:"addressType,omitempty"`
	ServiceName        string                   `json:"serviceName,omitempty"`
	Ports              []KubernetesEndpointPort `json:"ports,omitempty"`
	EndpointCount      int                      `json:"endpointCount,omitempty"`
	ReadyEndpointCount int                      `json:"readyEndpointCount,omitempty"`
	CreatedAt          time.Time                `json:"createdAt,omitempty"`
	Labels             map[string]string        `json:"labels,omitempty"`
}

func (e KubernetesEndpointSlice) NormalizeCollections() KubernetesEndpointSlice {
	if e.Ports == nil {
		e.Ports = []KubernetesEndpointPort{}
	}
	if e.Labels == nil {
		e.Labels = map[string]string{}
	}
	return e
}

type KubernetesEndpointPort struct {
	Name        string `json:"name,omitempty"`
	Protocol    string `json:"protocol,omitempty"`
	Port        int32  `json:"port,omitempty"`
	AppProtocol string `json:"appProtocol,omitempty"`
}

type KubernetesNetworkPolicy struct {
	UID              string            `json:"uid"`
	Name             string            `json:"name"`
	Namespace        string            `json:"namespace"`
	PolicyTypes      []string          `json:"policyTypes,omitempty"`
	IngressRuleCount int               `json:"ingressRuleCount,omitempty"`
	EgressRuleCount  int               `json:"egressRuleCount,omitempty"`
	CreatedAt        time.Time         `json:"createdAt,omitempty"`
	Labels           map[string]string `json:"labels,omitempty"`
}

func (n KubernetesNetworkPolicy) NormalizeCollections() KubernetesNetworkPolicy {
	if n.PolicyTypes == nil {
		n.PolicyTypes = []string{}
	}
	if n.Labels == nil {
		n.Labels = map[string]string{}
	}
	return n
}

type KubernetesPersistentVolume struct {
	UID            string            `json:"uid"`
	Name           string            `json:"name"`
	Phase          string            `json:"phase,omitempty"`
	StorageClass   string            `json:"storageClass,omitempty"`
	CapacityBytes  int64             `json:"capacityBytes,omitempty"`
	AccessModes    []string          `json:"accessModes,omitempty"`
	ReclaimPolicy  string            `json:"reclaimPolicy,omitempty"`
	ClaimNamespace string            `json:"claimNamespace,omitempty"`
	ClaimName      string            `json:"claimName,omitempty"`
	CreatedAt      time.Time         `json:"createdAt,omitempty"`
	Labels         map[string]string `json:"labels,omitempty"`
}

func (p KubernetesPersistentVolume) NormalizeCollections() KubernetesPersistentVolume {
	if p.AccessModes == nil {
		p.AccessModes = []string{}
	}
	if p.Labels == nil {
		p.Labels = map[string]string{}
	}
	return p
}

type KubernetesPersistentVolumeClaim struct {
	UID            string            `json:"uid"`
	Name           string            `json:"name"`
	Namespace      string            `json:"namespace"`
	Phase          string            `json:"phase,omitempty"`
	StorageClass   string            `json:"storageClass,omitempty"`
	RequestedBytes int64             `json:"requestedBytes,omitempty"`
	CapacityBytes  int64             `json:"capacityBytes,omitempty"`
	AccessModes    []string          `json:"accessModes,omitempty"`
	VolumeName     string            `json:"volumeName,omitempty"`
	CreatedAt      time.Time         `json:"createdAt,omitempty"`
	Labels         map[string]string `json:"labels,omitempty"`
}

func (p KubernetesPersistentVolumeClaim) NormalizeCollections() KubernetesPersistentVolumeClaim {
	if p.AccessModes == nil {
		p.AccessModes = []string{}
	}
	if p.Labels == nil {
		p.Labels = map[string]string{}
	}
	return p
}

type KubernetesStorageClass struct {
	UID                  string            `json:"uid"`
	Name                 string            `json:"name"`
	Provisioner          string            `json:"provisioner,omitempty"`
	ReclaimPolicy        string            `json:"reclaimPolicy,omitempty"`
	VolumeBindingMode    string            `json:"volumeBindingMode,omitempty"`
	AllowVolumeExpansion *bool             `json:"allowVolumeExpansion,omitempty"`
	ParameterKeys        []string          `json:"parameterKeys,omitempty"`
	CreatedAt            time.Time         `json:"createdAt,omitempty"`
	Labels               map[string]string `json:"labels,omitempty"`
}

func (s KubernetesStorageClass) NormalizeCollections() KubernetesStorageClass {
	if s.ParameterKeys == nil {
		s.ParameterKeys = []string{}
	}
	if s.Labels == nil {
		s.Labels = map[string]string{}
	}
	return s
}

type KubernetesConfigMap struct {
	UID            string            `json:"uid"`
	Name           string            `json:"name"`
	Namespace      string            `json:"namespace"`
	DataKeys       []string          `json:"dataKeys,omitempty"`
	BinaryDataKeys []string          `json:"binaryDataKeys,omitempty"`
	Immutable      bool              `json:"immutable,omitempty"`
	MetadataOnly   bool              `json:"metadataOnly,omitempty"`
	CreatedAt      time.Time         `json:"createdAt,omitempty"`
	Labels         map[string]string `json:"labels,omitempty"`
}

func (c KubernetesConfigMap) NormalizeCollections() KubernetesConfigMap {
	if c.DataKeys == nil {
		c.DataKeys = []string{}
	}
	if c.BinaryDataKeys == nil {
		c.BinaryDataKeys = []string{}
	}
	if c.Labels == nil {
		c.Labels = map[string]string{}
	}
	return c
}

type KubernetesSecret struct {
	UID          string            `json:"uid"`
	Name         string            `json:"name"`
	Namespace    string            `json:"namespace"`
	Type         string            `json:"type,omitempty"`
	DataKeys     []string          `json:"dataKeys,omitempty"`
	Immutable    bool              `json:"immutable,omitempty"`
	MetadataOnly bool              `json:"metadataOnly,omitempty"`
	CreatedAt    time.Time         `json:"createdAt,omitempty"`
	Labels       map[string]string `json:"labels,omitempty"`
}

func (s KubernetesSecret) NormalizeCollections() KubernetesSecret {
	if s.DataKeys == nil {
		s.DataKeys = []string{}
	}
	if s.Labels == nil {
		s.Labels = map[string]string{}
	}
	return s
}

type KubernetesServiceAccount struct {
	UID                          string            `json:"uid"`
	Name                         string            `json:"name"`
	Namespace                    string            `json:"namespace"`
	AutomountServiceAccountToken *bool             `json:"automountServiceAccountToken,omitempty"`
	SecretCount                  int               `json:"secretCount,omitempty"`
	ImagePullSecrets             []string          `json:"imagePullSecrets,omitempty"`
	CreatedAt                    time.Time         `json:"createdAt,omitempty"`
	Labels                       map[string]string `json:"labels,omitempty"`
}

func (s KubernetesServiceAccount) NormalizeCollections() KubernetesServiceAccount {
	if s.ImagePullSecrets == nil {
		s.ImagePullSecrets = []string{}
	}
	if s.Labels == nil {
		s.Labels = map[string]string{}
	}
	return s
}

// KubernetesRole mirrors the agent's namespaced RBAC Role contract. Full
// PolicyRule contents are deliberately not propagated so Pulse stays a
// "what permissions exist where" surface, not an RBAC enumeration tool.
type KubernetesRole struct {
	UID       string            `json:"uid"`
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	RuleCount int               `json:"ruleCount,omitempty"`
	CreatedAt time.Time         `json:"createdAt,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
}

func (r KubernetesRole) NormalizeCollections() KubernetesRole {
	if r.Labels == nil {
		r.Labels = map[string]string{}
	}
	return r
}

// KubernetesClusterRole mirrors the agent's cluster-scoped RBAC ClusterRole
// contract. See KubernetesRole for the rule-count rationale.
type KubernetesClusterRole struct {
	UID               string            `json:"uid"`
	Name              string            `json:"name"`
	RuleCount         int               `json:"ruleCount,omitempty"`
	AggregationLabels map[string]string `json:"aggregationLabels,omitempty"`
	CreatedAt         time.Time         `json:"createdAt,omitempty"`
	Labels            map[string]string `json:"labels,omitempty"`
}

func (r KubernetesClusterRole) NormalizeCollections() KubernetesClusterRole {
	if r.AggregationLabels == nil {
		r.AggregationLabels = map[string]string{}
	}
	if r.Labels == nil {
		r.Labels = map[string]string{}
	}
	return r
}

// KubernetesRoleBinding mirrors the agent's namespaced RBAC RoleBinding.
// SubjectKinds is the distinct, sorted set of subject Kinds bound by this
// rule (User / Group / ServiceAccount); individual subject names are not
// reported.
type KubernetesRoleBinding struct {
	UID          string            `json:"uid"`
	Name         string            `json:"name"`
	Namespace    string            `json:"namespace"`
	RoleKind     string            `json:"roleKind"`
	RoleName     string            `json:"roleName"`
	SubjectCount int               `json:"subjectCount,omitempty"`
	SubjectKinds []string          `json:"subjectKinds,omitempty"`
	CreatedAt    time.Time         `json:"createdAt,omitempty"`
	Labels       map[string]string `json:"labels,omitempty"`
}

func (b KubernetesRoleBinding) NormalizeCollections() KubernetesRoleBinding {
	if b.SubjectKinds == nil {
		b.SubjectKinds = []string{}
	}
	if b.Labels == nil {
		b.Labels = map[string]string{}
	}
	return b
}

// KubernetesClusterRoleBinding mirrors the agent's cluster-scoped RBAC
// ClusterRoleBinding. See KubernetesRoleBinding for the subject-kinds rationale.
type KubernetesClusterRoleBinding struct {
	UID          string            `json:"uid"`
	Name         string            `json:"name"`
	RoleKind     string            `json:"roleKind"`
	RoleName     string            `json:"roleName"`
	SubjectCount int               `json:"subjectCount,omitempty"`
	SubjectKinds []string          `json:"subjectKinds,omitempty"`
	CreatedAt    time.Time         `json:"createdAt,omitempty"`
	Labels       map[string]string `json:"labels,omitempty"`
}

func (b KubernetesClusterRoleBinding) NormalizeCollections() KubernetesClusterRoleBinding {
	if b.SubjectKinds == nil {
		b.SubjectKinds = []string{}
	}
	if b.Labels == nil {
		b.Labels = map[string]string{}
	}
	return b
}

type KubernetesResourceQuota struct {
	UID       string            `json:"uid"`
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Hard      map[string]string `json:"hard,omitempty"`
	Used      map[string]string `json:"used,omitempty"`
	CreatedAt time.Time         `json:"createdAt,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
}

func (q KubernetesResourceQuota) NormalizeCollections() KubernetesResourceQuota {
	if q.Hard == nil {
		q.Hard = map[string]string{}
	}
	if q.Used == nil {
		q.Used = map[string]string{}
	}
	if q.Labels == nil {
		q.Labels = map[string]string{}
	}
	return q
}

type KubernetesLimitRange struct {
	UID        string            `json:"uid"`
	Name       string            `json:"name"`
	Namespace  string            `json:"namespace"`
	LimitTypes []string          `json:"limitTypes,omitempty"`
	CreatedAt  time.Time         `json:"createdAt,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
}

func (l KubernetesLimitRange) NormalizeCollections() KubernetesLimitRange {
	if l.LimitTypes == nil {
		l.LimitTypes = []string{}
	}
	if l.Labels == nil {
		l.Labels = map[string]string{}
	}
	return l
}

type KubernetesPodDisruptionBudget struct {
	UID                string            `json:"uid"`
	Name               string            `json:"name"`
	Namespace          string            `json:"namespace"`
	MinAvailable       string            `json:"minAvailable,omitempty"`
	MaxUnavailable     string            `json:"maxUnavailable,omitempty"`
	DesiredHealthy     int32             `json:"desiredHealthy,omitempty"`
	CurrentHealthy     int32             `json:"currentHealthy,omitempty"`
	DisruptionsAllowed int32             `json:"disruptionsAllowed,omitempty"`
	ExpectedPods       int32             `json:"expectedPods,omitempty"`
	CreatedAt          time.Time         `json:"createdAt,omitempty"`
	Labels             map[string]string `json:"labels,omitempty"`
}

func (p KubernetesPodDisruptionBudget) NormalizeCollections() KubernetesPodDisruptionBudget {
	if p.Labels == nil {
		p.Labels = map[string]string{}
	}
	return p
}

type KubernetesHorizontalPodAutoscaler struct {
	UID             string            `json:"uid"`
	Name            string            `json:"name"`
	Namespace       string            `json:"namespace"`
	TargetKind      string            `json:"targetKind,omitempty"`
	TargetName      string            `json:"targetName,omitempty"`
	MinReplicas     int32             `json:"minReplicas,omitempty"`
	MaxReplicas     int32             `json:"maxReplicas,omitempty"`
	CurrentReplicas int32             `json:"currentReplicas,omitempty"`
	DesiredReplicas int32             `json:"desiredReplicas,omitempty"`
	MetricTypes     []string          `json:"metricTypes,omitempty"`
	CreatedAt       time.Time         `json:"createdAt,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
}

func (h KubernetesHorizontalPodAutoscaler) NormalizeCollections() KubernetesHorizontalPodAutoscaler {
	if h.MetricTypes == nil {
		h.MetricTypes = []string{}
	}
	if h.Labels == nil {
		h.Labels = map[string]string{}
	}
	return h
}

type KubernetesEvent struct {
	UID          string     `json:"uid"`
	Name         string     `json:"name"`
	Namespace    string     `json:"namespace,omitempty"`
	EventType    string     `json:"type,omitempty"`
	Reason       string     `json:"reason,omitempty"`
	Message      string     `json:"message,omitempty"`
	InvolvedKind string     `json:"involvedKind,omitempty"`
	InvolvedName string     `json:"involvedName,omitempty"`
	Count        int32      `json:"count,omitempty"`
	FirstSeen    *time.Time `json:"firstSeen,omitempty"`
	LastSeen     *time.Time `json:"lastSeen,omitempty"`
	EventTime    *time.Time `json:"eventTime,omitempty"`
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

func (s DockerService) NormalizeCollections() DockerService {
	if s.Labels == nil {
		s.Labels = map[string]string{}
	}
	if s.EndpointPorts == nil {
		s.EndpointPorts = []DockerServicePort{}
	}
	return s
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

// DockerNode summarises a Docker Swarm node.
type DockerNode struct {
	ID                  string            `json:"id"`
	Hostname            string            `json:"hostname,omitempty"`
	Role                string            `json:"role,omitempty"`
	Availability        string            `json:"availability,omitempty"`
	State               string            `json:"state,omitempty"`
	Message             string            `json:"message,omitempty"`
	Address             string            `json:"address,omitempty"`
	ManagerReachability string            `json:"managerReachability,omitempty"`
	ManagerAddress      string            `json:"managerAddress,omitempty"`
	Leader              bool              `json:"leader,omitempty"`
	EngineVersion       string            `json:"engineVersion,omitempty"`
	OS                  string            `json:"os,omitempty"`
	Architecture        string            `json:"architecture,omitempty"`
	NanoCPUs            int64             `json:"nanoCpus,omitempty"`
	MemoryBytes         int64             `json:"memoryBytes,omitempty"`
	Labels              map[string]string `json:"labels,omitempty"`
	EngineLabels        map[string]string `json:"engineLabels,omitempty"`
	CreatedAt           time.Time         `json:"createdAt,omitempty"`
	UpdatedAt           *time.Time        `json:"updatedAt,omitempty"`
}

func (n DockerNode) NormalizeCollections() DockerNode {
	if n.Labels == nil {
		n.Labels = map[string]string{}
	}
	if n.EngineLabels == nil {
		n.EngineLabels = map[string]string{}
	}
	return n
}

// DockerSecret summarises Docker Swarm secret metadata.
type DockerSecret struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	Labels           map[string]string `json:"labels,omitempty"`
	DriverName       string            `json:"driverName,omitempty"`
	TemplatingDriver string            `json:"templatingDriver,omitempty"`
	CreatedAt        time.Time         `json:"createdAt,omitempty"`
	UpdatedAt        *time.Time        `json:"updatedAt,omitempty"`
}

func (s DockerSecret) NormalizeCollections() DockerSecret {
	if s.Labels == nil {
		s.Labels = map[string]string{}
	}
	return s
}

// DockerConfig summarises Docker Swarm config metadata.
type DockerConfig struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	Labels           map[string]string `json:"labels,omitempty"`
	TemplatingDriver string            `json:"templatingDriver,omitempty"`
	CreatedAt        time.Time         `json:"createdAt,omitempty"`
	UpdatedAt        *time.Time        `json:"updatedAt,omitempty"`
}

func (c DockerConfig) NormalizeCollections() DockerConfig {
	if c.Labels == nil {
		c.Labels = map[string]string{}
	}
	return c
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
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Node      string    `json:"node"`
	Instance  string    `json:"instance"`
	Nodes     []string  `json:"nodes,omitempty"`
	NodeIDs   []string  `json:"nodeIds,omitempty"`
	AliasIDs  []string  `json:"aliasIds,omitempty"`
	NodeCount int       `json:"nodeCount,omitempty"`
	Type      string    `json:"type"`
	Status    string    `json:"status"`
	Pool      string    `json:"pool,omitempty"`
	Path      string    `json:"path,omitempty"`
	Total     int64     `json:"total"`
	Used      int64     `json:"used"`
	Free      int64     `json:"free"`
	Usage     float64   `json:"usage"`
	Content   string    `json:"content"`
	Shared    bool      `json:"shared"`
	Enabled   bool      `json:"enabled"`
	Active    bool      `json:"active"`
	LastSeen  time.Time `json:"lastSeen,omitzero"` // when the owning poller last delivered this entry; zero = never seen
	ZFSPool   *ZFSPool  `json:"zfsPool,omitempty"` // ZFS pool details if this is ZFS storage
}

func CephPoolStorageID(instanceName, poolName string) string {
	instance := strings.TrimSpace(instanceName)
	if instance == "" {
		instance = "ceph"
	}
	name := strings.TrimSpace(poolName)
	if name == "" {
		name = "pool"
	}
	return fmt.Sprintf("%s-ceph-pool-%s", instance, name)
}

func CephPoolStorageAliasIDs(instanceName, poolName string, instanceAliases ...string) []string {
	canonicalID := CephPoolStorageID(instanceName, poolName)
	aliases := make([]string, 0, len(instanceAliases)+2)
	for _, instance := range append([]string{instanceName}, instanceAliases...) {
		instance = strings.TrimSpace(instance)
		if instance == "" {
			continue
		}
		candidates := []string{instance}
		if strings.HasPrefix(instance, "agent:") {
			if unprefixed := strings.TrimSpace(strings.TrimPrefix(instance, "agent:")); unprefixed != "" {
				candidates = append(candidates, unprefixed)
			}
		} else {
			candidates = append(candidates, "agent:"+instance)
		}
		for _, candidate := range candidates {
			aliasID := CephPoolStorageID(candidate, poolName)
			if aliasID == canonicalID || containsString(aliases, aliasID) {
				continue
			}
			aliases = append(aliases, aliasID)
		}
	}
	return aliases
}

func StorageFromCephPool(cluster CephCluster, pool CephPool) Storage {
	name := strings.TrimSpace(pool.Name)
	if name == "" {
		name = fmt.Sprintf("pool-%d", pool.ID)
	}
	total := pool.StoredBytes + pool.AvailableBytes
	free := pool.AvailableBytes
	if free < 0 {
		free = 0
	}
	usage := pool.PercentUsed
	if usage == 0 && total > 0 && pool.StoredBytes > 0 {
		usage = (float64(pool.StoredBytes) / float64(total)) * 100
	}

	status := "available"
	switch strings.ToUpper(strings.TrimSpace(cluster.Health)) {
	case "HEALTH_ERR", "ERR", "ERROR":
		status = "unavailable"
	}

	return Storage{
		ID:       CephPoolStorageID(cluster.Instance, name),
		Name:     name,
		Node:     "cluster",
		Instance: cluster.Instance,
		AliasIDs: CephPoolStorageAliasIDs(cluster.Instance, name, cluster.InstanceAliases...),
		Type:     "ceph",
		Status:   status,
		Pool:     name,
		Total:    total,
		Used:     pool.StoredBytes,
		Free:     free,
		Usage:    usage,
		Content:  "ceph",
		Shared:   true,
		Enabled:  true,
		Active:   status == "available",
		LastSeen: cluster.LastUpdated,
	}
}

func CephPoolStorage(cluster CephCluster) []Storage {
	if len(cluster.Pools) == 0 {
		return nil
	}
	out := make([]Storage, 0, len(cluster.Pools))
	for _, pool := range cluster.Pools {
		out = append(out, StorageFromCephPool(cluster, pool))
	}
	return out
}

func (s Storage) NormalizeCollections() Storage {
	if s.Nodes == nil {
		s.Nodes = []string{}
	}
	if s.NodeIDs == nil {
		s.NodeIDs = []string{}
	}
	if s.ZFSPool != nil {
		pool := s.ZFSPool.NormalizeCollections()
		s.ZFSPool = &pool
	}
	return s
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

func (p ZFSPool) NormalizeCollections() ZFSPool {
	if p.Devices == nil {
		p.Devices = []ZFSDevice{}
	}
	return p
}

// ZFSDevice represents a device in a ZFS pool
type ZFSDevice struct {
	Name           string `json:"name"`
	Type           string `json:"type"`  // disk, mirror, raidz, raidz2, raidz3, spare, log, cache, special, special-group
	State          string `json:"state"` // ONLINE, DEGRADED, FAULTED, OFFLINE, REMOVED, UNAVAIL
	ReadErrors     int64  `json:"readErrors"`
	WriteErrors    int64  `json:"writeErrors"`
	ChecksumErrors int64  `json:"checksumErrors"`
	Message        string `json:"message,omitempty"` // Additional message provided by Proxmox (if any)
}

// CephCluster represents the health and capacity information for a Ceph cluster
type CephCluster struct {
	ID              string              `json:"id"`
	Instance        string              `json:"instance"`
	InstanceAliases []string            `json:"instanceAliases,omitempty"`
	Source          string              `json:"source,omitempty"`
	Name            string              `json:"name"`
	FSID            string              `json:"fsid,omitempty"`
	Health          string              `json:"health"`
	HealthMessage   string              `json:"healthMessage,omitempty"`
	TotalBytes      int64               `json:"totalBytes"`
	UsedBytes       int64               `json:"usedBytes"`
	AvailableBytes  int64               `json:"availableBytes"`
	UsagePercent    float64             `json:"usagePercent"`
	NumMons         int                 `json:"numMons"`
	NumMgrs         int                 `json:"numMgrs"`
	NumOSDs         int                 `json:"numOsds"`
	NumOSDsUp       int                 `json:"numOsdsUp"`
	NumOSDsIn       int                 `json:"numOsdsIn"`
	NumPGs          int                 `json:"numPGs"`
	Pools           []CephPool          `json:"pools,omitempty"`
	Services        []CephServiceStatus `json:"services,omitempty"`
	LastUpdated     time.Time           `json:"lastUpdated"`
}

func (c CephCluster) NormalizeCollections() CephCluster {
	if c.InstanceAliases == nil {
		c.InstanceAliases = []string{}
	}
	if c.Pools == nil {
		c.Pools = []CephPool{}
	}
	if c.Services == nil {
		c.Services = []CephServiceStatus{}
	}
	return c
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
	WWN             string           `json:"wwn"`          // World Wide Name
	Type            string           `json:"type"`         // nvme, sata, sas
	Size            int64            `json:"size"`         // bytes
	Health          string           `json:"health"`       // PASSED, FAILED, UNKNOWN
	Wearout         int              `json:"wearout"`      // SSD wear metric from Proxmox (0-100, -1 when unavailable)
	Temperature     int              `json:"temperature"`  // Celsius (if available)
	RPM             int              `json:"rpm"`          // 0 for SSDs
	Used            string           `json:"used"`         // Filesystem or partition usage
	StorageGroup    string           `json:"storageGroup"` // Pool/VG/array this disk belongs to (e.g. ZFS pool name); empty if not matched
	SmartAttributes *SMARTAttributes `json:"smartAttributes,omitempty"`
	LastChecked     time.Time        `json:"lastChecked"`
}

// PBSInstance represents a Proxmox Backup Server instance
type PBSInstance struct {
	ID                string                 `json:"id"`
	Name              string                 `json:"name"`
	Host              string                 `json:"host"`
	GuestURL          string                 `json:"guestURL,omitempty"` // Optional guest-accessible URL (for navigation)
	Status            string                 `json:"status"`
	Version           string                 `json:"version"`
	CPU               float64                `json:"cpu"`         // CPU usage percentage
	Memory            float64                `json:"memory"`      // Memory usage percentage
	MemoryUsed        int64                  `json:"memoryUsed"`  // Memory used in bytes
	MemoryTotal       int64                  `json:"memoryTotal"` // Total memory in bytes
	Uptime            int64                  `json:"uptime"`      // Uptime in seconds
	Datastores        []PBSDatastore         `json:"datastores"`
	BackupJobs        []PBSBackupJob         `json:"backupJobs"`
	SyncJobs          []PBSSyncJob           `json:"syncJobs"`
	VerifyJobs        []PBSVerifyJob         `json:"verifyJobs"`
	PruneJobs         []PBSPruneJob          `json:"pruneJobs"`
	GarbageJobs       []PBSGarbageJob        `json:"garbageJobs"`
	JobHealthEvidence []PBSJobHealthEvidence `json:"jobHealthEvidence"`
	ConnectionHealth  string                 `json:"connectionHealth"`
	LastSeen          time.Time              `json:"lastSeen"`
}

func (i PBSInstance) NormalizeCollections() PBSInstance {
	if i.Datastores == nil {
		i.Datastores = []PBSDatastore{}
	}
	for idx := range i.Datastores {
		i.Datastores[idx] = i.Datastores[idx].NormalizeCollections()
	}
	if i.BackupJobs == nil {
		i.BackupJobs = []PBSBackupJob{}
	}
	if i.SyncJobs == nil {
		i.SyncJobs = []PBSSyncJob{}
	}
	if i.VerifyJobs == nil {
		i.VerifyJobs = []PBSVerifyJob{}
	}
	if i.PruneJobs == nil {
		i.PruneJobs = []PBSPruneJob{}
	}
	if i.GarbageJobs == nil {
		i.GarbageJobs = []PBSGarbageJob{}
	}
	if i.JobHealthEvidence == nil {
		i.JobHealthEvidence = []PBSJobHealthEvidence{}
	}
	return i
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
	Namespaces          []PBSNamespace `json:"namespaces"`
	DeduplicationFactor float64        `json:"deduplicationFactor,omitempty"`
}

func (d PBSDatastore) NormalizeCollections() PBSDatastore {
	if d.Namespaces == nil {
		d.Namespaces = []PBSNamespace{}
	}
	return d
}

// PBSNamespace represents a PBS namespace
type PBSNamespace struct {
	Path   string `json:"path"`
	Parent string `json:"parent,omitempty"`
	Depth  int    `json:"depth"`
}

// PBSBackup represents a backup stored on PBS
type PBSBackup struct {
	ID              string    `json:"id"`       // Unique ID combining PBS instance, namespace, type, vmid, and time
	Instance        string    `json:"instance"` // PBS instance name
	Datastore       string    `json:"datastore"`
	Namespace       string    `json:"namespace"`
	BackupType      string    `json:"backupType"` // "vm" or "ct"
	VMID            string    `json:"vmid"`
	BackupTime      time.Time `json:"backupTime"`
	Size            int64     `json:"size"`
	Protected       bool      `json:"protected"`
	Verified        bool      `json:"verified"`
	VerificationRaw any       `json:"verificationRaw,omitempty"`
	Comment         string    `json:"comment,omitempty"`
	Files           []string  `json:"files"`
	Owner           string    `json:"owner,omitempty"` // User who created the backup
}

func (b PBSBackup) NormalizeCollections() PBSBackup {
	if b.Files == nil {
		b.Files = []string{}
	}
	return b
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

// PBSJobHealthEvidence is the shared PBS job-health ledger shape for backup,
// sync, verify, prune, and garbage-collection job families. PBS backup-family
// evidence is observed task-history evidence only; configured scheduled backup
// compliance belongs to a future PVE /cluster/backup source.
type PBSJobHealthEvidence struct {
	ID             string                `json:"id"`
	Family         string                `json:"family"`
	Store          string                `json:"store,omitempty"`
	Remote         string                `json:"remote,omitempty"`
	Namespace      string                `json:"namespace,omitempty"`
	Schedule       string                `json:"schedule,omitempty"`
	Comment        string                `json:"comment,omitempty"`
	Enabled        bool                  `json:"enabled"`
	LastRunState   string                `json:"last-run-state,omitempty"`
	LastRunUPID    string                `json:"last-run-upid,omitempty"`
	LastRunEndtime int64                 `json:"last-run-endtime,omitempty"`
	NextRun        int64                 `json:"next-run,omitempty"`
	UPID           string                `json:"upid,omitempty"`
	WorkerType     string                `json:"worker-type,omitempty"`
	WorkerID       string                `json:"worker-id,omitempty"`
	TaskStatus     string                `json:"task-status,omitempty"`
	TaskStartTime  int64                 `json:"task-starttime,omitempty"`
	TaskEndTime    int64                 `json:"task-endtime,omitempty"`
	Confidence     string                `json:"confidence"`
	EvidenceSource string                `json:"evidenceSource,omitempty"`
	EvidenceScope  string                `json:"evidenceScope,omitempty"`
	Freshness      PBSJobHealthFreshness `json:"freshness"`
	Posture        string                `json:"posture,omitempty"`
	PostureReason  string                `json:"postureReason,omitempty"`
	Error          string                `json:"error,omitempty"`
}

// PBSJobHealthFreshness captures timing separately from evidence confidence.
type PBSJobHealthFreshness struct {
	ObservedAt     time.Time `json:"observedAt"`
	LastRunEndTime time.Time `json:"lastRunEndTime,omitempty"`
	NextRun        time.Time `json:"nextRun,omitempty"`
	AgeSeconds     int64     `json:"ageSeconds,omitempty"`
	State          string    `json:"state,omitempty"`
}

// PMGInstance represents a Proxmox Mail Gateway connection
type PMGInstance struct {
	ID               string               `json:"id"`
	Name             string               `json:"name"`
	Host             string               `json:"host"`
	GuestURL         string               `json:"guestURL,omitempty"` // Optional guest-accessible URL (for navigation)
	Status           string               `json:"status"`
	Version          string               `json:"version"`
	Nodes            []PMGNodeStatus      `json:"nodes"`
	MailStats        *PMGMailStats        `json:"mailStats,omitempty"`
	MailCount        []PMGMailCountPoint  `json:"mailCount"`
	SpamDistribution []PMGSpamBucket      `json:"spamDistribution"`
	Quarantine       *PMGQuarantineTotals `json:"quarantine,omitempty"`
	RelayDomains     []PMGRelayDomain     `json:"relayDomains"`
	DomainStats      []PMGDomainStat      `json:"domainStats"`
	DomainStatsAsOf  time.Time            `json:"domainStatsAsOf,omitempty"`
	ConnectionHealth string               `json:"connectionHealth"`
	LastSeen         time.Time            `json:"lastSeen"`
	LastUpdated      time.Time            `json:"lastUpdated"`
}

func (i PMGInstance) NormalizeCollections() PMGInstance {
	if i.Nodes == nil {
		i.Nodes = []PMGNodeStatus{}
	}
	if i.MailCount == nil {
		i.MailCount = []PMGMailCountPoint{}
	}
	if i.SpamDistribution == nil {
		i.SpamDistribution = []PMGSpamBucket{}
	}
	if i.RelayDomains == nil {
		i.RelayDomains = []PMGRelayDomain{}
	}
	if i.DomainStats == nil {
		i.DomainStats = []PMGDomainStat{}
	}
	return i
}

// PMGDomainStat describes mail statistics for a domain over a fixed time window
// (currently: the last 24 hours at poll time).
type PMGDomainStat struct {
	Domain     string  `json:"domain"`
	MailCount  float64 `json:"mailCount"`
	SpamCount  float64 `json:"spamCount"`
	VirusCount float64 `json:"virusCount"`
	Bytes      float64 `json:"bytes,omitempty"`
}

// PMGRelayDomain represents a relay domain configured in Proxmox Mail Gateway.
type PMGRelayDomain struct {
	Domain  string `json:"domain"`
	Comment string `json:"comment,omitempty"`
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

func (b Backups) NormalizeCollections() Backups {
	b.PVE = b.PVE.NormalizeCollections()
	if b.PBS == nil {
		b.PBS = []PBSBackup{}
	}
	for i := range b.PBS {
		b.PBS[i] = b.PBS[i].NormalizeCollections()
	}
	if b.PMG == nil {
		b.PMG = []PMGBackup{}
	}
	return b
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
	Total int64 `json:"total"`
	Used  int64 `json:"used"`
	Free  int64 `json:"free"`
	// Reclaimable buff/cache (available - truly free); used + cache + free ≈ total.
	Cache     int64   `json:"cache,omitempty"`
	Usage     float64 `json:"usage"`
	Balloon   int64   `json:"balloon,omitempty"`
	SwapUsed  int64   `json:"swapUsed,omitempty"`
	SwapTotal int64   `json:"swapTotal,omitempty"`
}

type GuestNetworkInterface struct {
	Name      string   `json:"name"`
	MAC       string   `json:"mac,omitempty"`
	Addresses []string `json:"addresses"`
	RXBytes   int64    `json:"rxBytes,omitempty"`
	TXBytes   int64    `json:"txBytes,omitempty"`
}

func (i GuestNetworkInterface) NormalizeCollections() GuestNetworkInterface {
	if i.Addresses == nil {
		i.Addresses = []string{}
	}
	return i
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
	Cores        []CoreTemp `json:"cores"`                  // Individual core temperatures
	GPU          []GPUTemp  `json:"gpu"`                    // GPU temperatures
	NVMe         []NVMeTemp `json:"nvme"`                   // NVMe drive temperatures
	SMART        []DiskTemp `json:"smart"`                  // Physical disk temperatures from SMART data
	Available    bool       `json:"available"`              // Whether any temperature data is available
	HasCPU       bool       `json:"hasCPU"`                 // Whether CPU temperature data is available
	HasGPU       bool       `json:"hasGPU"`                 // Whether GPU temperature data is available
	HasNVMe      bool       `json:"hasNVMe"`                // Whether NVMe temperature data is available
	HasSMART     bool       `json:"hasSMART"`               // Whether SMART disk temperature data is available
	// True when the SSH payload was raw `sensors -j` output instead of the
	// pulse-sensors wrapper payload. Pre-v6.0.0-rc.6 setup scripts lock the
	// authorized_keys entry to `sensors -j`, so SMART (SATA/SAS) disk temps
	// can never arrive until the node setup script is re-run.
	LegacySensorsFormat bool      `json:"legacySensorsFormat,omitempty"`
	LastUpdate          time.Time `json:"lastUpdate"` // When this data was collected
}

func (t Temperature) NormalizeCollections() Temperature {
	if t.Cores == nil {
		t.Cores = []CoreTemp{}
	}
	if t.GPU == nil {
		t.GPU = []GPUTemp{}
	}
	if t.NVMe == nil {
		t.NVMe = []NVMeTemp{}
	}
	if t.SMART == nil {
		t.SMART = []DiskTemp{}
	}
	return t
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

func (m Metric) NormalizeCollections() Metric {
	if m.Values == nil {
		m.Values = map[string]interface{}{}
	}
	return m
}

// PVEBackups represents PVE backup information
type PVEBackups struct {
	BackupTasks    []BackupTask    `json:"backupTasks"`
	StorageBackups []StorageBackup `json:"storageBackups"`
	GuestSnapshots []GuestSnapshot `json:"guestSnapshots"`
}

func (b PVEBackups) NormalizeCollections() PVEBackups {
	if b.BackupTasks == nil {
		b.BackupTasks = []BackupTask{}
	}
	if b.StorageBackups == nil {
		b.StorageBackups = []StorageBackup{}
	}
	if b.GuestSnapshots == nil {
		b.GuestSnapshots = []GuestSnapshot{}
	}
	return b
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

func (p Performance) NormalizeCollections() Performance {
	if p.APICallDuration == nil {
		p.APICallDuration = map[string]float64{}
	}
	return p
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
		Nodes:                     make([]Node, 0),
		VMs:                       make([]VM, 0),
		Containers:                make([]Container, 0),
		DockerHosts:               make([]DockerHost, 0),
		RemovedDockerHosts:        make([]RemovedDockerHost, 0),
		KubernetesClusters:        make([]KubernetesCluster, 0),
		RemovedKubernetesClusters: make([]RemovedKubernetesCluster, 0),
		Hosts:                     make([]Host, 0),
		RemovedHostAgents:         make([]RemovedHostAgent, 0),
		Storage:                   make([]Storage, 0),
		CephClusters:              make([]CephCluster, 0),
		PhysicalDisks:             make([]PhysicalDisk, 0),
		PBSInstances:              make([]PBSInstance, 0),
		PMGInstances:              make([]PMGInstance, 0),
		PBSBackups:                make([]PBSBackup, 0),
		PMGBackups:                make([]PMGBackup, 0),
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
		PVETagColors:     make(map[string]string),
		PVETagStyles:     make(map[string]PVETagStyle),
		Performance:      Performance{}.NormalizeCollections(),
		LastUpdate:       time.Now(),
	}

	state.syncBackupsLocked()
	return state
}

// MergeTagColors merges Proxmox tag color entries into the shared state.
func (s *State) MergeTagColors(colors map[string]string) {
	if len(colors) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.PVETagColors == nil {
		s.PVETagColors = make(map[string]string, len(colors))
	}
	for tag, color := range colors {
		s.PVETagColors[strings.ToLower(strings.TrimSpace(tag))] = strings.TrimSpace(color)
	}
	s.LastUpdate = time.Now()
}

// MergePVETagStyle merges the Proxmox tag style for one PVE instance into the
// shared state while preserving the legacy aggregate color map.
func (s *State) MergePVETagStyle(instance string, style PVETagStyle) {
	instance = strings.TrimSpace(instance)
	if instance == "" && len(style.Colors) == 0 && !style.CaseSensitive {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.PVETagColors == nil {
		s.PVETagColors = make(map[string]string, len(style.Colors))
	}
	if s.PVETagStyles == nil {
		s.PVETagStyles = make(map[string]PVETagStyle)
	}

	normalizedColors := make(map[string]string, len(style.Colors))
	for tag, color := range style.Colors {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if !style.CaseSensitive {
			tag = strings.ToLower(tag)
		}
		color = strings.TrimSpace(color)
		normalizedColors[tag] = color
	}

	if instance != "" {
		s.PVETagStyles[instance] = PVETagStyle{
			Colors:        normalizedColors,
			CaseSensitive: style.CaseSensitive,
		}
		s.PVETagColors = make(map[string]string)
		for _, instanceStyle := range s.PVETagStyles {
			for tag, color := range instanceStyle.Colors {
				s.PVETagColors[tag] = color
			}
		}
	} else {
		for tag, color := range normalizedColors {
			s.PVETagColors[tag] = color
		}
	}
	s.LastUpdate = time.Now()
}

// syncBackupsLocked updates the aggregated backups structure.
func (s *State) syncBackupsLocked() {
	s.Backups = Backups{
		PVE: clonePVEBackups(s.PVEBackups),
		PBS: clonePBSBackups(s.PBSBackups),
		PMG: clonePMGBackups(s.PMGBackups),
	}
}

// UpdateActiveAlerts updates the active alerts in the state.
// Always sets a non-nil slice so callers can distinguish "synced with zero alerts" from "never synced".
func (s *State) UpdateActiveAlerts(alerts []Alert) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cloned := cloneAlerts(alerts)
	if cloned == nil {
		cloned = []Alert{}
	}
	s.ActiveAlerts = cloned
}

// UpdateRecentlyResolved updates the recently resolved alerts in the state
func (s *State) UpdateRecentlyResolved(resolved []ResolvedAlert) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cloned := cloneResolvedAlerts(resolved)
	if cloned == nil {
		cloned = []ResolvedAlert{}
	}
	s.RecentlyResolved = cloned
}

// UpdateNodes updates the nodes in the state
func (s *State) UpdateNodes(nodes []Node) {
	s.mu.Lock()
	defer s.mu.Unlock()

	stateNodes := cloneNodes(nodes)
	if stateNodes == nil {
		stateNodes = []Node{}
	}

	// Sort nodes by name to ensure consistent ordering
	sort.Slice(stateNodes, func(i, j int) bool {
		return stateNodes[i].Name < stateNodes[j].Name
	})

	s.Nodes = stateNodes
	s.LastUpdate = time.Now()
}

func normalizeNodeIdentityPart(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizeIPAddress(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.Contains(value, "/") {
		if ip, _, err := net.ParseCIDR(value); err == nil && ip != nil {
			return ip.String()
		}
		value = strings.Split(value, "/")[0]
	}
	ip := net.ParseIP(value)
	if ip == nil {
		return ""
	}
	return ip.String()
}

func extractHostEndpoint(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err == nil && parsed.Host != "" {
		return strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	}
	if strings.Contains(raw, "/") {
		raw = strings.Split(raw, "/")[0]
	}
	if host, port, err := net.SplitHostPort(raw); err == nil {
		_ = port
		return strings.ToLower(strings.TrimSpace(host))
	}
	return strings.ToLower(strings.Trim(strings.TrimSpace(raw), "[]"))
}

func shortHostname(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}
	if idx := strings.IndexRune(value, '.'); idx > 0 {
		return value[:idx]
	}
	return value
}

func nodeEndpointMergeAliases(node Node) []string {
	name := normalizeNodeIdentityPart(node.Name)
	if name == "" {
		return nil
	}
	endpoint := extractHostEndpoint(node.Host)
	if endpoint == "" {
		return nil
	}
	if ip := normalizeIPAddress(endpoint); ip != "" {
		return []string{"endpoint-ip:" + ip + ":" + name}
	}

	aliases := []string{"endpoint-host:" + endpoint + ":" + name}
	if short := shortHostname(endpoint); short != "" && short != endpoint {
		aliases = append(aliases, "endpoint-host:"+short+":"+name)
	}
	return aliases
}

func nodeLogicalKey(node Node) string {
	name := normalizeNodeIdentityPart(node.Name)
	if name == "" {
		return ""
	}

	if cluster := normalizeNodeIdentityPart(node.ClusterName); cluster != "" {
		return "cluster:" + cluster + ":" + name
	}
	if instance := normalizeNodeIdentityPart(node.Instance); instance != "" {
		return "instance:" + instance + ":" + name
	}
	if host := normalizeNodeIdentityPart(node.Host); host != "" {
		return "endpoint:" + host + ":" + name
	}

	return "name:" + name
}

func registerNodeAliases(
	aliasToKey map[string]string,
	ambiguousAliases map[string]struct{},
	aliases []string,
	key string,
) {
	for _, alias := range aliases {
		if alias == "" || key == "" {
			continue
		}
		if _, ambiguous := ambiguousAliases[alias]; ambiguous {
			continue
		}
		if existing, ok := aliasToKey[alias]; ok && existing != key {
			delete(aliasToKey, alias)
			ambiguousAliases[alias] = struct{}{}
			continue
		}
		aliasToKey[alias] = key
	}
}

func resolveNodeMergeKey(
	primaryKey string,
	node Node,
	nodeMap map[string]Node,
	aliasToKey map[string]string,
	ambiguousAliases map[string]struct{},
) string {
	if primaryKey == "" {
		return ""
	}
	if _, ok := nodeMap[primaryKey]; ok {
		return primaryKey
	}

	resolved := ""
	for _, alias := range nodeEndpointMergeAliases(node) {
		if _, ambiguous := ambiguousAliases[alias]; ambiguous {
			continue
		}
		key, ok := aliasToKey[alias]
		if !ok || key == "" {
			continue
		}
		if resolved != "" && resolved != key {
			return primaryKey
		}
		resolved = key
	}
	if resolved != "" {
		return resolved
	}
	return primaryKey
}

func nodeStatusRank(status string) int {
	switch normalizeNodeIdentityPart(status) {
	case "online":
		return 3
	case "degraded":
		return 2
	case "unknown":
		return 1
	default:
		return 0
	}
}

func nodeConnectionHealthRank(health string) int {
	switch normalizeNodeIdentityPart(health) {
	case "healthy":
		return 3
	case "degraded":
		return 2
	case "unknown":
		return 1
	default:
		return 0
	}
}

func preferNodeForMerge(existing Node, candidate Node) Node {
	existingScore := nodeStatusRank(existing.Status)*100 + nodeConnectionHealthRank(existing.ConnectionHealth)*10
	if existing.LinkedAgentID != "" {
		existingScore += 2
	}
	if existing.IsClusterMember {
		existingScore++
	}

	candidateScore := nodeStatusRank(candidate.Status)*100 + nodeConnectionHealthRank(candidate.ConnectionHealth)*10
	if candidate.LinkedAgentID != "" {
		candidateScore += 2
	}
	if candidate.IsClusterMember {
		candidateScore++
	}

	if candidateScore > existingScore {
		return candidate
	}
	if candidateScore < existingScore {
		return existing
	}

	if candidate.LastSeen.After(existing.LastSeen) {
		return candidate
	}
	if existing.LastSeen.After(candidate.LastSeen) {
		return existing
	}

	return existing
}

func reconcileHostNodeLinksLocked(hosts []Host, nodes []Node) {
	linkedNodeByHostID := make(map[string]string)
	multipleNodeLinksByHostID := make(map[string]struct{})
	for _, node := range nodes {
		hostID := strings.TrimSpace(node.LinkedAgentID)
		nodeID := strings.TrimSpace(node.ID)
		if hostID == "" || nodeID == "" {
			continue
		}
		if existingNodeID, ok := linkedNodeByHostID[hostID]; ok && existingNodeID != nodeID {
			multipleNodeLinksByHostID[hostID] = struct{}{}
			continue
		}
		linkedNodeByHostID[hostID] = nodeID
	}

	for i := range hosts {
		hostID := strings.TrimSpace(hosts[i].ID)
		if hostID == "" {
			continue
		}

		nodeID, hasLinkedNode := linkedNodeByHostID[hostID]
		_, ambiguous := multipleNodeLinksByHostID[hostID]
		switch {
		case ambiguous:
			hosts[i].LinkedNodeID = ""
		case hasLinkedNode:
			hosts[i].LinkedNodeID = nodeID
			hosts[i].LinkedVMID = ""
			hosts[i].LinkedContainerID = ""
		case hosts[i].LinkedNodeID != "":
			hosts[i].LinkedNodeID = ""
		}
	}
}

// UpdateNodesForInstance updates nodes for a specific instance, merging with existing nodes
func (s *State) UpdateNodesForInstance(instanceName string, nodes []Node) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Preserve LinkedAgentID for nodes that are being updated, including when IDs churn.
	existingNodeLinks := make(map[string]string)             // nodeID -> linkedAgentID
	existingNodeLinksByLogicalKey := make(map[string]string) // logical node key -> linkedAgentID
	for _, node := range s.Nodes {
		if node.LinkedAgentID == "" {
			continue
		}
		existingNodeLinks[node.ID] = node.LinkedAgentID
		if key := nodeLogicalKey(node); key != "" {
			existingNodeLinksByLogicalKey[key] = node.LinkedAgentID
		}
	}

	// Build map excluding nodes from this instance (they'll be replaced by the new set).
	// Key by logical identity so nodes with different IDs but the same cluster+name merge.
	nodeMap := make(map[string]Node)
	nodeAliasToKey := make(map[string]string)
	ambiguousNodeAliases := make(map[string]struct{})
	linkedHostToKey := make(map[string]string)
	ambiguousLinkedHosts := make(map[string]struct{})
	registerLinkedHostKey := func(hostID, key string) {
		hostID = strings.TrimSpace(hostID)
		key = strings.TrimSpace(key)
		if hostID == "" || key == "" {
			return
		}
		if _, ambiguous := ambiguousLinkedHosts[hostID]; ambiguous {
			return
		}
		if existing, ok := linkedHostToKey[hostID]; ok && existing != key {
			delete(linkedHostToKey, hostID)
			ambiguousLinkedHosts[hostID] = struct{}{}
			return
		}
		linkedHostToKey[hostID] = key
	}
	for _, node := range s.Nodes {
		if node.Instance == instanceName {
			continue
		}

		key := nodeLogicalKey(node)
		if key == "" {
			key = "id:" + normalizeNodeIdentityPart(node.ID)
		}
		if key == "id:" {
			key = "instance:" + normalizeNodeIdentityPart(node.Instance) + ":" + normalizeNodeIdentityPart(node.Name)
		}

		if existing, ok := nodeMap[key]; ok {
			nodeMap[key] = preferNodeForMerge(existing, node)
		} else {
			nodeMap[key] = node
		}
		registerNodeAliases(nodeAliasToKey, ambiguousNodeAliases, nodeEndpointMergeAliases(node), key)
		registerLinkedHostKey(node.LinkedAgentID, key)
	}

	// Deduplicate incoming nodes by logical identity so a single node cannot appear
	// multiple times in the same poll cycle with different IDs.
	dedupedNodes := make(map[string]Node)
	for _, node := range nodes {
		node = cloneNode(node)
		key := nodeLogicalKey(node)
		if key == "" {
			key = "id:" + normalizeNodeIdentityPart(node.ID)
		}
		if key == "id:" {
			key = "instance:" + normalizeNodeIdentityPart(instanceName) + ":" + normalizeNodeIdentityPart(node.Name)
		}

		if existing, ok := dedupedNodes[key]; ok {
			dedupedNodes[key] = preferNodeForMerge(existing, node)
			continue
		}
		dedupedNodes[key] = node
	}

	// Build hostname-to-hostAgentID map for linking new nodes to existing host agents.
	// Keep all candidates so we can avoid creating links when hostname matching is ambiguous.
	// Also build a set of valid host agent IDs to validate existing links.
	hostAgentByHostname := make(map[string]map[string]struct{}) // lowercase hostname -> hostAgentIDs
	hostAgentByIP := make(map[string]map[string]struct{})       // normalized ip -> hostAgentIDs
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
	addHostIP := func(address, hostID string) {
		ip := normalizeIPAddress(address)
		if ip == "" || hostID == "" {
			return
		}
		bucket := hostAgentByIP[ip]
		if bucket == nil {
			bucket = make(map[string]struct{})
			hostAgentByIP[ip] = bucket
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
			addHostIP(host.ReportIP, host.ID)
			for _, iface := range host.NetworkInterfaces {
				for _, address := range iface.Addresses {
					addHostIP(address, host.ID)
				}
			}
		}
	}

	// Add or update nodes from this instance
	for _, node := range dedupedNodes {
		// Preserve existing link if we had one, but only if the host agent still exists
		if existingLink, ok := existingNodeLinks[node.ID]; ok {
			if validHostAgentIDs[existingLink] {
				node.LinkedAgentID = existingLink
			}
			// If host agent no longer exists, leave LinkedAgentID empty (stale reference cleared)
		}
		// Fallback: preserve link by logical identity when node IDs changed across polls.
		if node.LinkedAgentID == "" {
			if existingLink, ok := existingNodeLinksByLogicalKey[nodeLogicalKey(node)]; ok {
				if validHostAgentIDs[existingLink] {
					node.LinkedAgentID = existingLink
				}
			}
		}
		endpointCandidateCount := 0
		endpointCandidates := make(map[string]struct{})
		if node.LinkedAgentID == "" {
			endpoint := extractHostEndpoint(node.Host)
			if ip := normalizeIPAddress(endpoint); ip != "" {
				if ids, ok := hostAgentByIP[ip]; ok {
					for hostID := range ids {
						endpointCandidates[hostID] = struct{}{}
					}
				}
			} else if endpoint != "" {
				if ids, ok := hostAgentByHostname[strings.ToLower(endpoint)]; ok {
					for hostID := range ids {
						endpointCandidates[hostID] = struct{}{}
					}
				}
				if short := shortHostname(endpoint); short != "" && short != strings.ToLower(endpoint) {
					if ids, ok := hostAgentByHostname[short]; ok {
						for hostID := range ids {
							endpointCandidates[hostID] = struct{}{}
						}
					}
				}
			}
			endpointCandidateCount = len(endpointCandidates)
			if endpointCandidateCount == 1 {
				for hostID := range endpointCandidates {
					node.LinkedAgentID = hostID
				}
			}
		}
		// If no existing link and endpoint evidence was absent, try to match by hostname.
		if node.LinkedAgentID == "" && endpointCandidateCount == 0 {
			nodeName := normalizeNodeIdentityPart(node.Name)
			candidates := make(map[string]struct{})
			if nodeName != "" {
				if ids, ok := hostAgentByHostname[nodeName]; ok {
					for hostID := range ids {
						candidates[hostID] = struct{}{}
					}
				}
				if idx := strings.Index(nodeName, "."); idx > 0 {
					if ids, ok := hostAgentByHostname[nodeName[:idx]]; ok {
						for hostID := range ids {
							candidates[hostID] = struct{}{}
						}
					}
				}
			}
			if len(candidates) == 1 {
				for hostID := range candidates {
					node.LinkedAgentID = hostID
				}
			}
		}

		primaryKey := nodeLogicalKey(node)
		targetKey := resolveNodeMergeKey(primaryKey, node, nodeMap, nodeAliasToKey, ambiguousNodeAliases)
		if node.LinkedAgentID != "" {
			if _, ambiguous := ambiguousLinkedHosts[node.LinkedAgentID]; !ambiguous {
				if linkedKey := linkedHostToKey[node.LinkedAgentID]; linkedKey != "" {
					targetKey = linkedKey
				}
			}
		}
		if targetKey == "" {
			targetKey = "id:" + normalizeNodeIdentityPart(node.ID)
		}
		if targetKey == "id:" {
			targetKey = "instance:" + normalizeNodeIdentityPart(instanceName) + ":" + normalizeNodeIdentityPart(node.Name)
		}
		if node.LinkedAgentID == "" {
			if existing, ok := nodeMap[targetKey]; ok {
				if validHostAgentIDs[existing.LinkedAgentID] {
					node.LinkedAgentID = existing.LinkedAgentID
				}
			}
		}

		if existing, ok := nodeMap[targetKey]; ok {
			nodeMap[targetKey] = preferNodeForMerge(existing, node)
		} else {
			nodeMap[targetKey] = node
		}
		registerNodeAliases(nodeAliasToKey, ambiguousNodeAliases, nodeEndpointMergeAliases(node), targetKey)
		registerLinkedHostKey(node.LinkedAgentID, targetKey)
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

	reconcileHostNodeLinksLocked(s.Hosts, newNodes)

	s.Nodes = newNodes
	s.LastUpdate = time.Now()
}

// UpdateVMs updates the VMs in the state
func (s *State) UpdateVMs(vms []VM) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cloned := cloneVMs(vms)
	if cloned == nil {
		cloned = []VM{}
	}
	s.VMs = cloned
	s.LastUpdate = time.Now()
}

func updateSliceByInstanceWithBackup[T any, K int | int64](
	existing []T,
	newItems []T,
	instanceName string,
	getID func(T) string,
	getInstance func(T) string,
	getVMID func(T) K,
	getLastBackup func(T) time.Time,
	setLastBackup func(T, time.Time) T,
	clone func(T) T,
	less func([]T, int, int) bool,
) []T {
	existingByVMID := make(map[K]T)
	for _, item := range existing {
		if getInstance(item) == instanceName {
			existingByVMID[getVMID(item)] = item
		}
	}

	itemMap := make(map[string]T)
	for _, item := range existing {
		if getInstance(item) != instanceName {
			itemMap[getID(item)] = item
		}
	}
	for _, item := range newItems {
		item = clone(item)
		if existing, ok := existingByVMID[getVMID(item)]; ok && getLastBackup(item).IsZero() {
			item = setLastBackup(item, getLastBackup(existing))
		}
		itemMap[getID(item)] = item
	}
	result := make([]T, 0, len(itemMap))
	for _, item := range itemMap {
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool { return less(result, i, j) })
	return result
}

func updateSliceForInstance[T any, K int | int64](
	s *State,
	slicePtr *[]T,
	newItems []T,
	instanceName string,
	getID func(T) string,
	getInstance func(T) string,
	getVMID func(T) K,
	getLastBackup func(T) time.Time,
	setLastBackup func(T, time.Time) T,
	clone func(T) T,
	less func([]T, int, int) bool,
) {
	s.mu.Lock()
	defer s.mu.Unlock()

	*slicePtr = updateSliceByInstanceWithBackup(
		*slicePtr, newItems, instanceName,
		getID, getInstance, getVMID, getLastBackup, setLastBackup, clone, less,
	)
	s.LastUpdate = time.Now()
}

// UpdateVMsForInstance updates VMs for a specific instance, merging with existing VMs
func (s *State) UpdateVMsForInstance(instanceName string, vms []VM) {
	updateSliceForInstance(
		s, &s.VMs, vms, instanceName,
		func(vm VM) string { return vm.ID },
		func(vm VM) string { return vm.Instance },
		func(vm VM) int { return vm.VMID },
		func(vm VM) time.Time { return vm.LastBackup },
		func(vm VM, t time.Time) VM { vm.LastBackup = t; return vm },
		cloneVM,
		func(items []VM, i, j int) bool { return items[i].VMID < items[j].VMID },
	)
}

// UpdateContainers updates the containers in the state
func (s *State) UpdateContainers(containers []Container) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cloned := cloneContainers(containers)
	if cloned == nil {
		cloned = []Container{}
	}
	s.Containers = cloned
	s.LastUpdate = time.Now()
}

// backupKey creates a composite key for backup matching using instance and VMID.
// This ensures backups are correctly matched to guests even when VMIDs are reused across instances.
func backupKey(instance string, vmid int) string {
	return instance + "-" + strconv.Itoa(vmid)
}

// SyncGuestBackupTimes updates LastBackup on VMs and Containers from storage backups and PBS backups.
// Call this after updating storage backups or PBS backups to ensure guest backup indicators are accurate.
// Matching is done by instance+VMID to prevent cross-instance VMID collisions.
// For PBS backups with namespaces, namespace matching is ranked against the guest's node and
// connection instance so clustered API entrypoints do not shadow the guest's actual placement.
func (s *State) SyncGuestBackupTimes() {
	s.mu.Lock()
	defer s.mu.Unlock()

	type pbsSubjectKey struct {
		backupType string
		vmid       int
	}

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

	// Process PBS backups (VMID is string, BackupTime is the timestamp).
	// Group by PBS subject type and VMID so VM and CT IDs do not cross-match.
	pbsBackupsBySubject := make(map[pbsSubjectKey][]PBSBackup)
	for _, backup := range s.PBSBackups {
		vmid, err := strconv.Atoi(backup.VMID)
		if err != nil || vmid <= 0 {
			continue
		}
		backupType := strings.ToLower(strings.TrimSpace(backup.BackupType))
		if backupType != "vm" && backupType != "ct" {
			continue
		}
		key := pbsSubjectKey{backupType: backupType, vmid: vmid}
		pbsBackupsBySubject[key] = append(pbsBackupsBySubject[key], backup)
	}

	// Build a set of typed VMIDs that appear on more than one PVE location.
	// When a typed VMID is ambiguous, we must not fall back to VMID-only matching
	// because we can't tell which guest the backup belongs to.
	subjectLocations := make(map[pbsSubjectKey]map[string]struct{})
	for i := range s.VMs {
		key := pbsSubjectKey{backupType: "vm", vmid: s.VMs[i].VMID}
		m, ok := subjectLocations[key]
		if !ok {
			m = make(map[string]struct{})
			subjectLocations[key] = m
		}
		m[backupKey(s.VMs[i].Instance, s.VMs[i].VMID)+"@"+s.VMs[i].Node] = struct{}{}
	}
	for i := range s.Containers {
		key := pbsSubjectKey{backupType: "ct", vmid: s.Containers[i].VMID}
		m, ok := subjectLocations[key]
		if !ok {
			m = make(map[string]struct{})
			subjectLocations[key] = m
		}
		m[backupKey(s.Containers[i].Instance, s.Containers[i].VMID)+"@"+s.Containers[i].Node] = struct{}{}
	}
	subjectIsAmbiguous := make(map[pbsSubjectKey]bool)
	for key, locations := range subjectLocations {
		if len(locations) > 1 {
			subjectIsAmbiguous[key] = true
		}
	}

	// findBestPBSBackup finds the best PBS backup for a given typed VMID and guest location.
	// Placement and guest-name matches are preferred over VMID-only fallback.
	// Returns zero time if no suitable backup found.
	findBestPBSBackup := func(vmid int, backupType string, instance string, node string, name string) time.Time {
		subjectKey := pbsSubjectKey{backupType: backupType, vmid: vmid}
		backups, ok := pbsBackupsBySubject[subjectKey]
		if !ok || len(backups) == 0 {
			return time.Time{}
		}

		var bestTime time.Time
		bestScore := -1

		for _, backup := range backups {
			score := proxmoxidentity.BackupGuestMatchScore(
				backup.Namespace,
				backup.Comment,
				backup.VMID,
				name,
				instance,
				node,
			)
			if score == 0 && !subjectIsAmbiguous[subjectKey] {
				score = 1
			}
			if score <= 0 {
				continue
			}
			if score > bestScore || (score == bestScore && backup.BackupTime.After(bestTime)) {
				bestScore = score
				bestTime = backup.BackupTime
			}
		}

		return bestTime
	}

	// Update VMs - recompute from current backup evidence instead of preserving stale values
	for i := range s.VMs {
		var lastBackup time.Time
		key := backupKey(s.VMs[i].Instance, s.VMs[i].VMID)
		if backupTime, ok := latestBackup[key]; ok {
			lastBackup = backupTime
		}
		// Check if PBS has a more recent backup
		pbsTime := findBestPBSBackup(s.VMs[i].VMID, "vm", s.VMs[i].Instance, s.VMs[i].Node, s.VMs[i].Name)
		if !pbsTime.IsZero() {
			if lastBackup.IsZero() || pbsTime.After(lastBackup) {
				lastBackup = pbsTime
			}
		}
		s.VMs[i].LastBackup = lastBackup
	}

	// Update Containers - recompute from current backup evidence instead of preserving stale values
	for i := range s.Containers {
		var lastBackup time.Time
		key := backupKey(s.Containers[i].Instance, s.Containers[i].VMID)
		if backupTime, ok := latestBackup[key]; ok {
			lastBackup = backupTime
		}
		// Check if PBS has a more recent backup
		pbsTime := findBestPBSBackup(s.Containers[i].VMID, "ct", s.Containers[i].Instance, s.Containers[i].Node, s.Containers[i].Name)
		if !pbsTime.IsZero() {
			if lastBackup.IsZero() || pbsTime.After(lastBackup) {
				lastBackup = pbsTime
			}
		}
		s.Containers[i].LastBackup = lastBackup
	}

	s.LastUpdate = time.Now()
}

// UpdateContainersForInstance updates containers for a specific instance, merging with existing containers
func (s *State) UpdateContainersForInstance(instanceName string, containers []Container) {
	updateSliceForInstance(
		s, &s.Containers, containers, instanceName,
		func(ct Container) string { return ct.ID },
		func(ct Container) string { return ct.Instance },
		func(ct Container) int { return ct.VMID },
		func(ct Container) time.Time { return ct.LastBackup },
		func(ct Container, t time.Time) Container { ct.LastBackup = t; return ct },
		cloneContainer,
		func(items []Container, i, j int) bool { return items[i].VMID < items[j].VMID },
	)
}

// UpsertDockerHost inserts or updates a Docker host in state.
func (s *State) UpsertDockerHost(host DockerHost) {
	s.mu.Lock()
	defer s.mu.Unlock()

	host = cloneDockerHost(host)

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
				host.Command = cloneDockerHostCommandStatus(existing.Command)
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
			return cloneDockerHost(host), true
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
	s.DockerHosts = []DockerHost{}
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
			return cloneDockerHost(host), true
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
			return cloneDockerHost(host), true
		}
	}

	return DockerHost{}, false
}

// SetDockerHostCommand updates the active command status for a docker host.
func (s *State) SetDockerHostCommand(hostID string, command *DockerHostCommandStatus) (DockerHost, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	command = cloneDockerHostCommandStatus(command)

	for i, host := range s.DockerHosts {
		if host.ID == hostID {
			host.Command = command
			s.DockerHosts[i] = host
			s.LastUpdate = time.Now()
			return cloneDockerHost(host), true
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
			return cloneDockerHost(host), true
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

	return cloneDockerHosts(removed)
}

// GetDockerHosts returns a copy of docker hosts.
func (s *State) GetDockerHosts() []DockerHost {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return cloneDockerHosts(s.DockerHosts)
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

// AddRemovedHostAgent records a removed host agent entry.
func (s *State) AddRemovedHostAgent(entry RemovedHostAgent) {
	s.mu.Lock()
	defer s.mu.Unlock()

	replaced := false
	for i, existing := range s.RemovedHostAgents {
		if existing.ID == entry.ID {
			s.RemovedHostAgents[i] = entry
			replaced = true
			break
		}
	}
	if !replaced {
		s.RemovedHostAgents = append(s.RemovedHostAgents, entry)
	}
	sort.Slice(s.RemovedHostAgents, func(i, j int) bool {
		return s.RemovedHostAgents[i].RemovedAt.After(s.RemovedHostAgents[j].RemovedAt)
	})
	s.LastUpdate = time.Now()
}

// RemoveRemovedHostAgent deletes a removed host agent entry by ID.
func (s *State) RemoveRemovedHostAgent(hostID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, entry := range s.RemovedHostAgents {
		if entry.ID == hostID {
			s.RemovedHostAgents = append(s.RemovedHostAgents[:i], s.RemovedHostAgents[i+1:]...)
			s.LastUpdate = time.Now()
			break
		}
	}
}

// GetRemovedHostAgents returns a copy of removed host agent entries.
func (s *State) GetRemovedHostAgents() []RemovedHostAgent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries := make([]RemovedHostAgent, len(s.RemovedHostAgents))
	copy(entries, s.RemovedHostAgents)
	return entries
}

// UpsertKubernetesCluster inserts or updates a Kubernetes cluster in state.
func (s *State) UpsertKubernetesCluster(cluster KubernetesCluster) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cluster = cloneKubernetesCluster(cluster)

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
			return cloneKubernetesCluster(cluster), true
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
			return cloneKubernetesCluster(cluster), true
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
			return cloneKubernetesCluster(cluster), true
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
			return cloneKubernetesCluster(cluster), true
		}
	}

	return KubernetesCluster{}, false
}

// GetKubernetesClusters returns a copy of kubernetes clusters.
func (s *State) GetKubernetesClusters() []KubernetesCluster {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return cloneKubernetesClusters(s.KubernetesClusters)
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

	host = cloneHost(host)

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

	return cloneHosts(s.Hosts)
}

// RemoveHost removes a host by ID and returns the removed entry.
func (s *State) RemoveHost(hostID string) (Host, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, host := range s.Hosts {
		if host.ID == hostID {
			s.Hosts = append(s.Hosts[:i], s.Hosts[i+1:]...)
			s.LastUpdate = time.Now()
			return cloneHost(host), true
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
	s.Hosts = []Host{}
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
			s.Nodes[i].LinkedAgentID = hostAgentID
			s.LastUpdate = time.Now()
			return true
		}
	}
	return false
}

// UnlinkNodesFromHostAgent clears LinkedAgentID from all nodes linked to the given host agent.
// This is called when a host agent is removed to clean up stale references.
func (s *State) UnlinkNodesFromHostAgent(hostAgentID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := 0
	for i, node := range s.Nodes {
		if node.LinkedAgentID == hostAgentID {
			s.Nodes[i].LinkedAgentID = ""
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
// with the same hostname). Sets LinkedNodeID on the host and LinkedAgentID on the node.
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
		return fmt.Errorf("%w: %s", ErrHostAgentNotFound, hostID)
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
		return fmt.Errorf("%w: %s", ErrNodeNotFound, nodeID)
	}

	// Clear any existing links from this host
	oldNodeID := s.Hosts[hostIdx].LinkedNodeID
	if oldNodeID != "" {
		for i, node := range s.Nodes {
			if node.ID == oldNodeID {
				s.Nodes[i].LinkedAgentID = ""
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
	s.Nodes[nodeIdx].LinkedAgentID = hostID

	s.LastUpdate = time.Now()
	return nil
}

// UnlinkHostAgent removes the bidirectional link between a host agent and its PVE node.
// Clears LinkedNodeID on the host and LinkedAgentID on the node.
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
		if node.ID == linkedNodeID || node.LinkedAgentID == hostID {
			s.Nodes[i].LinkedAgentID = ""
		}
	}

	s.LastUpdate = time.Now()
	return true
}

// UpsertCephCluster inserts or updates a Ceph cluster in the state.
// FSID identifies the physical Ceph cluster across API and host-agent reports.
func (s *State) UpsertCephCluster(cluster CephCluster) CephCluster {
	s.mu.Lock()
	defer s.mu.Unlock()

	cluster = normalizeCephClusterForState(cloneCephCluster(cluster), "")
	var stored CephCluster
	s.CephClusters, stored = upsertCephClusterInSlice(s.CephClusters, cluster)
	sortCephClustersForState(s.CephClusters)

	s.LastUpdate = time.Now()
	return cloneCephCluster(stored)
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
	cloned := cloneStorages(storage)
	if cloned == nil {
		cloned = []Storage{}
	}
	s.Storage = cloned
	s.LastUpdate = time.Now()
}

func updateSliceByInstance[T any](
	existing []T,
	newItems []T,
	instanceName string,
	getID func(T) string,
	getInstance func(T) string,
	clone func(T) T,
	less func([]T, int, int) bool,
) []T {
	itemMap := make(map[string]T)
	for _, item := range existing {
		if getInstance(item) != instanceName {
			itemMap[getID(item)] = item
		}
	}
	for _, item := range newItems {
		itemMap[getID(item)] = clone(item)
	}
	result := make([]T, 0, len(itemMap))
	for _, item := range itemMap {
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool { return less(result, i, j) })
	return result
}

// UpdatePhysicalDisks updates physical disks for a specific instance
func (s *State) UpdatePhysicalDisks(instanceName string, disks []PhysicalDisk) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.PhysicalDisks = updateSliceByInstance(
		s.PhysicalDisks, disks, instanceName,
		func(d PhysicalDisk) string { return d.ID },
		func(d PhysicalDisk) string { return d.Instance },
		clonePhysicalDisk,
		func(items []PhysicalDisk, i, j int) bool {
			if items[i].Node != items[j].Node {
				return items[i].Node < items[j].Node
			}
			return items[i].DevPath < items[j].DevPath
		},
	)
	s.LastUpdate = time.Now()
}

// UpdateStorageForInstance updates storage for a specific instance, merging with existing storage
func (s *State) UpdateStorageForInstance(instanceName string, storage []Storage) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Storage = updateSliceByInstance(
		s.Storage, storage, instanceName,
		func(st Storage) string { return st.ID },
		func(st Storage) string { return st.Instance },
		cloneStorage,
		func(items []Storage, i, j int) bool {
			if items[i].Instance == items[j].Instance {
				return items[i].Name < items[j].Name
			}
			return items[i].Instance < items[j].Instance
		},
	)
	s.LastUpdate = time.Now()
}

// UpdateCephClustersForInstance updates Proxmox-API Ceph cluster information for a specific instance.
func (s *State) UpdateCephClustersForInstance(instanceName string, clusters []CephCluster) []CephCluster {
	s.mu.Lock()
	defer s.mu.Unlock()

	filtered := make([]CephCluster, 0, len(s.CephClusters))
	for _, cluster := range s.CephClusters {
		cluster = normalizeCephClusterForState(cluster, "")
		if cluster.Instance == instanceName && normalizeCephClusterSource(cluster.Source, cluster.Instance, "") == CephClusterSourceProxmoxAPI {
			continue
		}
		filtered = append(filtered, cluster)
	}

	stored := make([]CephCluster, 0, len(clusters))
	for _, cluster := range clusters {
		cluster = cloneCephCluster(cluster)
		if strings.TrimSpace(cluster.Instance) == "" {
			cluster.Instance = instanceName
		}
		cluster = normalizeCephClusterForState(cluster, CephClusterSourceProxmoxAPI)
		var reconciled CephCluster
		filtered, reconciled = upsertCephClusterInSlice(filtered, cluster)
		stored = append(stored, reconciled)
	}

	sortCephClustersForState(filtered)
	s.CephClusters = filtered
	s.LastUpdate = time.Now()
	return cloneCephClusters(stored)
}

// UpdatePBSInstances updates the PBS instances in the state
func (s *State) UpdatePBSInstances(instances []PBSInstance) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cloned := clonePBSInstances(instances)
	if cloned == nil {
		cloned = []PBSInstance{}
	}
	s.PBSInstances = cloned
	s.LastUpdate = time.Now()
}

// UpdatePBSInstance updates a single PBS instance in the state, merging with existing instances
func (s *State) UpdatePBSInstance(instance PBSInstance) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Find and update existing instance or append new one
	found := false
	instance = clonePBSInstance(instance)

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

	cloned := clonePMGInstances(instances)
	if cloned == nil {
		cloned = []PMGInstance{}
	}
	s.PMGInstances = cloned
	s.LastUpdate = time.Now()
}

// UpdatePMGInstance updates or inserts a PMG instance record
func (s *State) UpdatePMGInstance(instance PMGInstance) {
	s.mu.Lock()
	defer s.mu.Unlock()

	instance = clonePMGInstance(instance)

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
		job = cloneReplicationJob(job)
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
		backupMap[backup.ID] = clonePBSBackup(backup)
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
		combined = append(combined, clonePMGBackups(backups)...)
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

	return cloneContainers(s.Containers)
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

// UpdatePollStats atomically updates performance and stats fields that are
// written at the end of each polling cycle. This prevents data races when
// multiple poll() goroutines run concurrently (e.g. during mock mode transitions).
func (s *State) UpdatePollStats(pollDuration float64, uptime int64, wsClients int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Performance.LastPollDuration = pollDuration
	s.Stats.PollingCycles++
	s.Stats.Uptime = uptime
	s.Stats.WebSocketClients = wsClients
}
