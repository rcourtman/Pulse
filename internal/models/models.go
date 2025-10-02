package models

import (
	"sort"
	"strings"
	"sync"
	"time"
)

// State represents the current state of all monitored resources
type State struct {
	mu               sync.RWMutex
	Nodes            []Node          `json:"nodes"`
	VMs              []VM            `json:"vms"`
	Containers       []Container     `json:"containers"`
	Storage          []Storage       `json:"storage"`
	PhysicalDisks    []PhysicalDisk  `json:"physicalDisks"`
	PBSInstances     []PBSInstance   `json:"pbs"`
	PBSBackups       []PBSBackup     `json:"pbsBackups"`
	Metrics          []Metric        `json:"metrics"`
	PVEBackups       PVEBackups      `json:"pveBackups"`
	Performance      Performance     `json:"performance"`
	ConnectionHealth map[string]bool `json:"connectionHealth"`
	Stats            Stats           `json:"stats"`
	ActiveAlerts     []Alert         `json:"activeAlerts"`
	RecentlyResolved []ResolvedAlert `json:"recentlyResolved"`
	LastUpdate       time.Time       `json:"lastUpdate"`
}

// Alert represents an active alert (simplified for State)
type Alert struct {
	ID           string    `json:"id"`
	Type         string    `json:"type"`
	Level        string    `json:"level"`
	ResourceID   string    `json:"resourceId"`
	ResourceName string    `json:"resourceName"`
	Node         string    `json:"node"`
	Instance     string    `json:"instance"`
	Message      string    `json:"message"`
	Value        float64   `json:"value"`
	Threshold    float64   `json:"threshold"`
	StartTime    time.Time `json:"startTime"`
	Acknowledged bool      `json:"acknowledged"`
}

// ResolvedAlert represents a recently resolved alert
type ResolvedAlert struct {
	Alert
	ResolvedTime time.Time `json:"resolvedTime"`
}

// Node represents a Proxmox VE node
type Node struct {
	ID               string       `json:"id"`
	Name             string       `json:"name"`
	Instance         string       `json:"instance"`
	Host             string       `json:"host"` // Full host URL from config
	Status           string       `json:"status"`
	Type             string       `json:"type"`
	CPU              float64      `json:"cpu"`
	Memory           Memory       `json:"memory"`
	Disk             Disk         `json:"disk"`
	Uptime           int64        `json:"uptime"`
	LoadAverage      []float64    `json:"loadAverage"`
	KernelVersion    string       `json:"kernelVersion"`
	PVEVersion       string       `json:"pveVersion"`
	CPUInfo          CPUInfo      `json:"cpuInfo"`
	Temperature      *Temperature `json:"temperature,omitempty"` // CPU/NVMe temperatures
	LastSeen         time.Time    `json:"lastSeen"`
	ConnectionHealth string       `json:"connectionHealth"`
	IsClusterMember  bool         `json:"isClusterMember"` // True if part of a cluster
	ClusterName      string       `json:"clusterName"`     // Name of cluster (empty if standalone)
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
	ID         string    `json:"id"`
	VMID       int       `json:"vmid"`
	Name       string    `json:"name"`
	Node       string    `json:"node"`
	Instance   string    `json:"instance"`
	Status     string    `json:"status"`
	Type       string    `json:"type"`
	CPU        float64   `json:"cpu"`
	CPUs       int       `json:"cpus"`
	Memory     Memory    `json:"memory"`
	Disk       Disk      `json:"disk"`
	Disks      []Disk    `json:"disks,omitempty"`
	NetworkIn  int64     `json:"networkIn"`
	NetworkOut int64     `json:"networkOut"`
	DiskRead   int64     `json:"diskRead"`
	DiskWrite  int64     `json:"diskWrite"`
	Uptime     int64     `json:"uptime"`
	Template   bool      `json:"template"`
	LastBackup time.Time `json:"lastBackup,omitempty"`
	Tags       []string  `json:"tags,omitempty"`
	Lock       string    `json:"lock,omitempty"`
	LastSeen   time.Time `json:"lastSeen"`
}

// Storage represents a storage resource
type Storage struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Node     string   `json:"node"`
	Instance string   `json:"instance"`
	Type     string   `json:"type"`
	Status   string   `json:"status"`
	Total    int64    `json:"total"`
	Used     int64    `json:"used"`
	Free     int64    `json:"free"`
	Usage    float64  `json:"usage"`
	Content  string   `json:"content"`
	Shared   bool     `json:"shared"`
	Enabled  bool     `json:"enabled"`
	Active   bool     `json:"active"`
	ZFSPool  *ZFSPool `json:"zfsPool,omitempty"` // ZFS pool details if this is ZFS storage
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
}

// PhysicalDisk represents a physical disk on a node
type PhysicalDisk struct {
	ID          string    `json:"id"` // "{instance}-{node}-{devpath}"
	Node        string    `json:"node"`
	Instance    string    `json:"instance"`
	DevPath     string    `json:"devPath"` // /dev/nvme0n1, /dev/sda
	Model       string    `json:"model"`
	Serial      string    `json:"serial"`
	Type        string    `json:"type"`                  // nvme, sata, sas
	Size        int64     `json:"size"`                  // bytes
	Health      string    `json:"health"`                // PASSED, FAILED, UNKNOWN
	Wearout     int       `json:"wearout"`               // SSD life remaining percentage (0-100, 100 is best)
	WearoutUsed int       `json:"wearoutUsed,omitempty"` // Percentage of wear consumed (0-100, 0 is new)
	Temperature int       `json:"temperature"`           // Celsius (if available)
	RPM         int       `json:"rpm"`                   // 0 for SSDs
	Used        string    `json:"used"`                  // Filesystem or partition usage
	LastChecked time.Time `json:"lastChecked"`
}

// PBSInstance represents a Proxmox Backup Server instance
type PBSInstance struct {
	ID               string          `json:"id"`
	Name             string          `json:"name"`
	Host             string          `json:"host"`
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
	CPUPackage float64    `json:"cpuPackage,omitempty"` // CPU package temperature (primary metric)
	CPUMax     float64    `json:"cpuMax,omitempty"`     // Highest core temperature
	Cores      []CoreTemp `json:"cores,omitempty"`      // Individual core temperatures
	NVMe       []NVMeTemp `json:"nvme,omitempty"`       // NVMe drive temperatures
	Available  bool       `json:"available"`            // Whether temperature data is available
	LastUpdate time.Time  `json:"lastUpdate"`           // When this data was collected
}

// CoreTemp represents a CPU core temperature
type CoreTemp struct {
	Core int     `json:"core"`
	Temp float64 `json:"temp"`
}

// NVMeTemp represents an NVMe drive temperature
type NVMeTemp struct {
	Device string  `json:"device"`
	Temp   float64 `json:"temp"`
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
	return &State{
		Nodes:         make([]Node, 0),
		VMs:           make([]VM, 0),
		Containers:    make([]Container, 0),
		Storage:       make([]Storage, 0),
		PhysicalDisks: make([]PhysicalDisk, 0),
		PBSInstances:  make([]PBSInstance, 0),
		PBSBackups:    make([]PBSBackup, 0),
		Metrics:       make([]Metric, 0),
		PVEBackups: PVEBackups{
			BackupTasks:    make([]BackupTask, 0),
			StorageBackups: make([]StorageBackup, 0),
			GuestSnapshots: make([]GuestSnapshot, 0),
		},
		ConnectionHealth: make(map[string]bool),
		ActiveAlerts:     make([]Alert, 0),
		RecentlyResolved: make([]ResolvedAlert, 0),
		LastUpdate:       time.Now(),
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

	// Create a map of existing nodes, excluding those from this instance
	nodeMap := make(map[string]Node)
	for _, node := range s.Nodes {
		if node.Instance != instanceName {
			nodeMap[node.ID] = node
		}
	}

	// Add or update nodes from this instance
	for _, node := range nodes {
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

	// Create a map of existing VMs, excluding those from this instance
	vmMap := make(map[string]VM)
	for _, vm := range s.VMs {
		if vm.Instance != instanceName {
			vmMap[vm.ID] = vm
		}
	}

	// Add or update VMs from this instance
	for _, vm := range vms {
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

// UpdateContainersForInstance updates containers for a specific instance, merging with existing containers
func (s *State) UpdateContainersForInstance(instanceName string, containers []Container) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create a map of existing containers, excluding those from this instance
	containerMap := make(map[string]Container)
	for _, container := range s.Containers {
		if container.Instance != instanceName {
			containerMap[container.ID] = container
		}
	}

	// Add or update containers from this instance
	for _, container := range containers {
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

// UpdateBackupTasksForInstance updates backup tasks for a specific instance, merging with existing tasks
func (s *State) UpdateBackupTasksForInstance(instanceName string, tasks []BackupTask) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create a map of existing tasks, excluding those from this instance
	taskMap := make(map[string]BackupTask)
	for _, task := range s.PVEBackups.BackupTasks {
		// Check if task ID contains the instance name
		if !strings.HasPrefix(task.ID, instanceName+"-") {
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
	s.LastUpdate = time.Now()
}

// UpdateStorageBackupsForInstance updates storage backups for a specific instance, merging with existing backups
func (s *State) UpdateStorageBackupsForInstance(instanceName string, backups []StorageBackup) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create a map of existing backups, excluding those from this instance
	backupMap := make(map[string]StorageBackup)
	for _, backup := range s.PVEBackups.StorageBackups {
		// Check if backup ID contains the instance name
		if !strings.HasPrefix(backup.ID, instanceName+"-") {
			backupMap[backup.ID] = backup
		}
	}

	// Add or update backups from this instance
	for _, backup := range backups {
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
	s.LastUpdate = time.Now()
}

// UpdateGuestSnapshotsForInstance updates guest snapshots for a specific instance, merging with existing snapshots
func (s *State) UpdateGuestSnapshotsForInstance(instanceName string, snapshots []GuestSnapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create a map of existing snapshots, excluding those from this instance
	snapshotMap := make(map[string]GuestSnapshot)
	for _, snapshot := range s.PVEBackups.GuestSnapshots {
		// Check if snapshot ID contains the instance name
		if !strings.HasPrefix(snapshot.ID, instanceName+"-") {
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
	s.LastUpdate = time.Now()
}

// SetConnectionHealth updates the connection health for an instance
func (s *State) SetConnectionHealth(instanceID string, healthy bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ConnectionHealth[instanceID] = healthy
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
	s.LastUpdate = time.Now()
}
