package reporting

import (
	"time"
)

// ReportFormat represents the output format of a report
type ReportFormat string

const (
	FormatCSV ReportFormat = "csv"
	FormatPDF ReportFormat = "pdf"
)

// MetricReportRequest defines the parameters for generating a report
type MetricReportRequest struct {
	ResourceType string
	ResourceID   string
	MetricType   string // Optional, if empty all metrics for the resource are included
	Start        time.Time
	End          time.Time
	Format       ReportFormat
	Title        string

	// Optional enrichment data (populated by handler from monitor state)
	Resource *ResourceInfo // Details about the resource being reported on
	Alerts   []AlertInfo   // Active and recently resolved alerts for this resource
	Backups  []BackupInfo  // Backup information for VMs/containers
	Storage  []StorageInfo // Storage pools (for nodes)
	Disks    []DiskInfo    // Physical disk health (for nodes)
}

// ResourceInfo contains details about the resource being reported on
type ResourceInfo struct {
	Name          string
	DisplayName   string
	Status        string
	Host          string // URL for nodes
	Node          string // Parent node for VMs/containers
	Instance      string // Proxmox instance name
	Uptime        int64
	KernelVersion string
	PVEVersion    string
	OSName        string
	OSVersion     string
	IPAddresses   []string
	CPUModel      string
	CPUCores      int
	CPUSockets    int
	MemoryTotal   int64
	DiskTotal     int64
	LoadAverage   []float64
	Temperature   *float64 // CPU temp if available
	Tags          []string
	ClusterName   string
	IsCluster     bool
}

// AlertInfo contains alert information for the report
type AlertInfo struct {
	Type         string
	Level        string // warning, critical
	Message      string
	Value        float64
	Threshold    float64
	StartTime    time.Time
	ResolvedTime *time.Time // nil if still active
	Acknowledged bool
}

// BackupInfo contains backup information for VMs/containers
type BackupInfo struct {
	Type       string // vzdump, pbs
	Storage    string
	Timestamp  time.Time
	Size       int64
	Verified   bool
	Protected  bool
	VolID      string
	NextBackup *time.Time
}

// StorageInfo contains storage pool information
type StorageInfo struct {
	Name      string
	Type      string // lvm, zfs, dir, nfs, etc.
	Status    string
	Total     int64
	Used      int64
	Available int64
	UsagePerc float64
	Content   string // images, rootdir, backup, etc.
	ZFSHealth string // For ZFS pools
	ZFSErrors int    // Checksum/read/write errors
}

// DiskInfo contains physical disk health information
type DiskInfo struct {
	Device      string
	Model       string
	Serial      string
	Type        string // nvme, ssd, hdd
	Size        int64
	Health      string // PASSED, FAILED, UNKNOWN
	Temperature int    // Celsius
	WearLevel   int    // 0-100, percentage of life REMAINING (100 = healthy, 0 = end of life, -1 = unknown)
}

// Engine defines the interface for report generation.
// This allows the enterprise version to provide PDF/CSV generation.
type Engine interface {
	Generate(req MetricReportRequest) (data []byte, contentType string, err error)
}

var (
	globalEngine Engine
)

// SetEngine sets the global report engine.
func SetEngine(e Engine) {
	globalEngine = e
}

// GetEngine returns the current global report engine.
func GetEngine() Engine {
	return globalEngine
}
