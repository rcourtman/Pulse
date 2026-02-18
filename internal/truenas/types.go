package truenas

import "time"

// FixtureSnapshot represents a deterministic TrueNAS API snapshot used for contract testing.
type FixtureSnapshot struct {
	CollectedAt      time.Time
	System           SystemInfo
	Pools            []Pool
	Datasets         []Dataset
	Disks            []Disk
	Alerts           []Alert
	ZFSSnapshots     []ZFSSnapshot
	ReplicationTasks []ReplicationTask
}

// SystemInfo mirrors high-level TrueNAS system identity/status data.
type SystemInfo struct {
	Hostname      string
	Version       string
	Build         string
	UptimeSeconds int64
	Healthy       bool
	MachineID     string
}

// Pool mirrors the subset of TrueNAS pool fields needed for unified mapping.
type Pool struct {
	ID         string
	Name       string
	Status     string
	TotalBytes int64
	UsedBytes  int64
	FreeBytes  int64
}

// Dataset mirrors the subset of TrueNAS dataset fields needed for unified mapping.
type Dataset struct {
	ID         string
	Name       string
	Pool       string
	UsedBytes  int64
	AvailBytes int64
	Mounted    bool
	ReadOnly   bool
}

// Disk mirrors a TrueNAS disk listing entry.
type Disk struct {
	ID         string
	Name       string
	Pool       string
	Status     string
	Model      string
	Serial     string
	SizeBytes  int64
	Transport  string
	Rotational bool
}

// Alert mirrors a TrueNAS alert listing entry.
type Alert struct {
	ID        string
	Level     string
	Message   string
	Source    string
	Dismissed bool
	Datetime  time.Time
}

// ZFSSnapshot mirrors the subset of snapshot fields needed for recovery-point mapping.
type ZFSSnapshot struct {
	ID         string
	Dataset    string
	Name       string // snapshot name (without dataset prefix), best-effort
	FullName   string // dataset@snapshot, best-effort
	CreatedAt  *time.Time
	UsedBytes  *int64
	Referenced *int64
}

// ReplicationTask mirrors the subset of replication task fields needed for recovery-point mapping.
type ReplicationTask struct {
	ID             string
	Name           string
	SourceDatasets []string
	TargetDataset  string
	Direction      string

	LastRun   *time.Time
	LastState string // SUCCESS / FAILED / RUNNING / etc, best-effort
	LastError string

	// Best-effort last snapshot name/identifier if exposed by API.
	LastSnapshot string
}
