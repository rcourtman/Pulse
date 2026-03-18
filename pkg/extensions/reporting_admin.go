package extensions

import (
	"context"
	"net/http"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/reporting"
)

// ReportingAdminEndpoints defines the enterprise reporting admin endpoint surface.
type ReportingAdminEndpoints interface {
	HandleGenerateReport(http.ResponseWriter, *http.Request)
	HandleGenerateMultiReport(http.ResponseWriter, *http.Request)
}

// WriteReportingErrorFunc writes a structured reporting error response.
type WriteReportingErrorFunc func(http.ResponseWriter, int, string, string, map[string]string)

// ReportingNodeSnapshot captures node fields needed for report enrichment.
type ReportingNodeSnapshot struct {
	ID            string
	Name          string
	DisplayName   string
	Status        string
	Host          string
	Instance      string
	Uptime        int64
	KernelVersion string
	PVEVersion    string
	CPUModel      string
	CPUCores      int
	CPUSockets    int
	MemoryTotal   int64
	DiskTotal     int64
	LoadAverage   []float64
	ClusterName   string
	IsCluster     bool
	Temperature   *float64
}

// ReportingVMSnapshot captures VM fields needed for report enrichment.
type ReportingVMSnapshot struct {
	ID          string
	VMID        int
	Name        string
	Status      string
	Node        string
	Instance    string
	Uptime      int64
	OSName      string
	OSVersion   string
	IPAddresses []string
	CPUCores    int
	MemoryTotal int64
	DiskTotal   int64
	Tags        []string
}

// ReportingContainerSnapshot captures container fields needed for report enrichment.
type ReportingContainerSnapshot struct {
	ID          string
	VMID        int
	Name        string
	Status      string
	Node        string
	Instance    string
	Uptime      int64
	OSName      string
	IPAddresses []string
	CPUCores    int
	MemoryTotal int64
	DiskTotal   int64
	Tags        []string
}

// ReportingAlertSnapshot captures alert fields needed for report enrichment.
type ReportingAlertSnapshot struct {
	ResourceID   string
	Node         string
	Type         string
	Level        string
	Message      string
	Value        float64
	Threshold    float64
	StartTime    time.Time
	ResolvedTime *time.Time
}

// ReportingStorageSnapshot captures storage pool fields needed for node reports.
type ReportingStorageSnapshot struct {
	Name      string
	Node      string
	Type      string
	Status    string
	Total     int64
	Used      int64
	Available int64
	UsagePerc float64
	Content   string
}

// ReportingDiskSnapshot captures physical disk fields needed for node reports.
type ReportingDiskSnapshot struct {
	Node        string
	Device      string
	Model       string
	Serial      string
	Type        string
	Size        int64
	Health      string
	Temperature int
	WearLevel   int
}

// ReportingLegacyBackupSnapshot captures legacy backup fields used as fallback.
type ReportingLegacyBackupSnapshot struct {
	VMID      int
	Node      string
	Storage   string
	Timestamp time.Time
	Size      int64
	Protected bool
	VolID     string
}

// ReportingStateSnapshot captures runtime state needed for report enrichment.
type ReportingStateSnapshot struct {
	Nodes          []ReportingNodeSnapshot
	VMs            []ReportingVMSnapshot
	Containers     []ReportingContainerSnapshot
	ActiveAlerts   []ReportingAlertSnapshot
	ResolvedAlerts []ReportingAlertSnapshot
	Storage        []ReportingStorageSnapshot
	Disks          []ReportingDiskSnapshot
	LegacyBackups  []ReportingLegacyBackupSnapshot
}

// EmptyReportingStateSnapshot returns a normalized zero-value reporting
// snapshot with stable empty-slice semantics for all collection fields.
func EmptyReportingStateSnapshot() ReportingStateSnapshot {
	snapshot := ReportingStateSnapshot{}
	snapshot.NormalizeCollections()
	return snapshot
}

// NormalizeCollections ensures reporting snapshots preserve present-but-empty
// collection semantics instead of leaking nil slices to downstream consumers.
func (s *ReportingStateSnapshot) NormalizeCollections() {
	if s.Nodes == nil {
		s.Nodes = []ReportingNodeSnapshot{}
	}
	if s.VMs == nil {
		s.VMs = []ReportingVMSnapshot{}
	}
	if s.Containers == nil {
		s.Containers = []ReportingContainerSnapshot{}
	}
	if s.ActiveAlerts == nil {
		s.ActiveAlerts = []ReportingAlertSnapshot{}
	}
	if s.ResolvedAlerts == nil {
		s.ResolvedAlerts = []ReportingAlertSnapshot{}
	}
	if s.Storage == nil {
		s.Storage = []ReportingStorageSnapshot{}
	}
	if s.Disks == nil {
		s.Disks = []ReportingDiskSnapshot{}
	}
	if s.LegacyBackups == nil {
		s.LegacyBackups = []ReportingLegacyBackupSnapshot{}
	}
}

// ReportingAdminRuntime exposes runtime capabilities needed by reporting admin endpoints.
type ReportingAdminRuntime struct {
	GetEngine              func() reporting.Engine
	GetRequestOrgID        func(context.Context) string
	GetStateSnapshot       func(context.Context, string) (ReportingStateSnapshot, bool)
	ListBackupsForResource func(context.Context, string, string, time.Time, time.Time) []reporting.BackupInfo
	SanitizeFilename       func(string) string
	WriteError             WriteReportingErrorFunc
}

// BindReportingAdminEndpointsFunc allows enterprise modules to bind replacement
// reporting admin endpoints while retaining access to default handlers.
type BindReportingAdminEndpointsFunc func(defaults ReportingAdminEndpoints, runtime ReportingAdminRuntime) ReportingAdminEndpoints
