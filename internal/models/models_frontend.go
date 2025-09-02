package models

// Frontend-friendly type aliases with proper JSON tags
// These extend the base types with additional computed fields

// NodeFrontend represents a Node with frontend-friendly field names
type NodeFrontend struct {
	ID               string    `json:"id"`
	Node             string    `json:"node"`     // Maps to Name
	Name             string    `json:"name"`     
	Instance         string    `json:"instance"`
	Status           string    `json:"status"`
	Type             string    `json:"type"`
	CPU              float64   `json:"cpu"`
	Mem              int64     `json:"mem"`      // Maps to Memory.Used
	MaxMem           int64     `json:"maxmem"`   // Maps to Memory.Total
	Disk             int64     `json:"disk"`     // Maps to Disk.Used
	MaxDisk          int64     `json:"maxdisk"`  // Maps to Disk.Total
	Uptime           int64     `json:"uptime"`
	LoadAverage      []float64 `json:"loadAverage"`
	KernelVersion    string    `json:"kernelVersion"`
	PVEVersion       string    `json:"pveVersion"`
	CPUInfo          CPUInfo   `json:"cpuInfo"`
	LastSeen         int64     `json:"lastSeen"` // Unix timestamp
	ConnectionHealth string    `json:"connectionHealth"`
}

// VMFrontend represents a VM with frontend-friendly field names
type VMFrontend struct {
	ID         string   `json:"id"`
	VMID       int      `json:"vmid"`
	Name       string   `json:"name"`
	Node       string   `json:"node"`
	Instance   string   `json:"instance"`
	Status     string   `json:"status"`
	Type       string   `json:"type"`
	CPU        float64  `json:"cpu"`
	CPUs       int      `json:"cpus"`
	Mem        int64    `json:"mem"`       // Maps to Memory.Used
	MaxMem     int64    `json:"maxmem"`    // Maps to Memory.Total
	Disk       int64    `json:"disk"`      // Maps to Disk.Used  
	MaxDisk    int64    `json:"maxdisk"`   // Maps to Disk.Total
	NetIn      int64    `json:"netin"`     // Maps to NetworkIn
	NetOut     int64    `json:"netout"`    // Maps to NetworkOut
	DiskRead   int64    `json:"diskread"`  // Maps to DiskRead
	DiskWrite  int64    `json:"diskwrite"` // Maps to DiskWrite
	Uptime     int64    `json:"uptime"`
	Template   bool     `json:"template"`
	LastBackup int64    `json:"lastBackup,omitempty"` // Unix timestamp
	Tags       string   `json:"tags,omitempty"`        // Joined string
	Lock       string   `json:"lock,omitempty"`
	LastSeen   int64    `json:"lastSeen"` // Unix timestamp
}

// ContainerFrontend represents a Container with frontend-friendly field names
type ContainerFrontend struct {
	ID         string   `json:"id"`
	VMID       int      `json:"vmid"`
	Name       string   `json:"name"`
	Node       string   `json:"node"`
	Instance   string   `json:"instance"`
	Status     string   `json:"status"`
	Type       string   `json:"type"`
	CPU        float64  `json:"cpu"`
	CPUs       int      `json:"cpus"`
	Mem        int64    `json:"mem"`       // Maps to Memory.Used
	MaxMem     int64    `json:"maxmem"`    // Maps to Memory.Total
	Disk       int64    `json:"disk"`      // Maps to Disk.Used
	MaxDisk    int64    `json:"maxdisk"`   // Maps to Disk.Total
	NetIn      int64    `json:"netin"`     // Maps to NetworkIn
	NetOut     int64    `json:"netout"`    // Maps to NetworkOut
	DiskRead   int64    `json:"diskread"`  // Maps to DiskRead
	DiskWrite  int64    `json:"diskwrite"` // Maps to DiskWrite
	Uptime     int64    `json:"uptime"`
	Template   bool     `json:"template"`
	LastBackup int64    `json:"lastBackup,omitempty"` // Unix timestamp
	Tags       string   `json:"tags,omitempty"`        // Joined string
	Lock       string   `json:"lock,omitempty"`
	LastSeen   int64    `json:"lastSeen"` // Unix timestamp
}

// StorageFrontend represents Storage with frontend-friendly field names
type StorageFrontend struct {
	ID       string  `json:"id"`
	Storage  string  `json:"storage"`  // Maps to Name
	Name     string  `json:"name"`
	Node     string  `json:"node"`
	Instance string  `json:"instance"`
	Type     string  `json:"type"`
	Status   string  `json:"status"`
	Total    int64   `json:"total"`
	Used     int64   `json:"used"`
	Avail    int64   `json:"avail"`    // Maps to Free
	Free     int64   `json:"free"`
	Usage    float64 `json:"usage"`
	Content  string  `json:"content"`
	Shared   bool    `json:"shared"`
	Enabled  bool    `json:"enabled"`
	Active   bool    `json:"active"`
}

// StateFrontend represents the state with frontend-friendly field names
type StateFrontend struct {
	Nodes            []NodeFrontend      `json:"nodes"`
	VMs              []VMFrontend        `json:"vms"`
	Containers       []ContainerFrontend `json:"containers"`
	Storage          []StorageFrontend   `json:"storage"`
	PBS              []PBSInstance       `json:"pbs"`              // Keep as is
	ActiveAlerts     []Alert             `json:"activeAlerts"`     // Active alerts
	Metrics          map[string]any      `json:"metrics"`          // Empty object for now
	PVEBackups       PVEBackups          `json:"pveBackups"`       // Keep as is
	Performance      map[string]any      `json:"performance"`      // Empty object for now
	ConnectionHealth map[string]bool     `json:"connectionHealth"` // Keep as is
	Stats            map[string]any      `json:"stats"`            // Empty object for now
	LastUpdate       int64               `json:"lastUpdate"`       // Unix timestamp
}