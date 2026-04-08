package hypervisor

import "context"

// Provider is the core interface that every hypervisor/cloud backend must implement.
// It provides read-only access to infrastructure resources (nodes, VMs, containers, storage).
type Provider interface {
	// Type returns the provider type (e.g., "proxmox", "vmware", "libvirt").
	Type() ProviderType

	// ID returns the unique identifier for this provider instance.
	ID() string

	// Name returns a human-friendly name for this provider instance.
	Name() string

	// Connect establishes or verifies the connection to the backend.
	Connect(ctx context.Context) error

	// Close cleanly shuts down the connection.
	Close() error

	// Healthy returns true if the provider is connected and responding.
	Healthy(ctx context.Context) bool

	// GetNodes returns all compute nodes/hosts managed by this provider.
	GetNodes(ctx context.Context) ([]Node, error)

	// GetVMs returns virtual machines, optionally filtered by node.
	// Pass empty nodeID to get VMs across all nodes.
	GetVMs(ctx context.Context, nodeID string) ([]VM, error)

	// GetContainers returns containers, optionally filtered by node.
	// Providers that don't support containers return nil, nil.
	GetContainers(ctx context.Context, nodeID string) ([]Container, error)

	// GetStorage returns storage resources, optionally filtered by node.
	GetStorage(ctx context.Context, nodeID string) ([]Storage, error)
}

// ConsoleProvider is optionally implemented by providers that support direct console access.
type ConsoleProvider interface {
	// GetConsoleTicket acquires a console ticket/token for connecting to a VM's console.
	GetConsoleTicket(ctx context.Context, nodeID, vmID string, consoleType ConsoleType) (*ConsoleTicket, error)

	// SupportedConsoleTypes returns the console protocols this provider supports.
	SupportedConsoleTypes() []ConsoleType
}

// PowerProvider is optionally implemented by providers that support VM power operations.
type PowerProvider interface {
	StartVM(ctx context.Context, nodeID, vmID string) error
	StopVM(ctx context.Context, nodeID, vmID string) error
	RebootVM(ctx context.Context, nodeID, vmID string) error
	SuspendVM(ctx context.Context, nodeID, vmID string) error
	ResumeVM(ctx context.Context, nodeID, vmID string) error
}

// SnapshotProvider is optionally implemented by providers that support VM snapshots.
type SnapshotProvider interface {
	GetSnapshots(ctx context.Context, nodeID, vmID string) ([]Snapshot, error)
	CreateSnapshot(ctx context.Context, nodeID, vmID, name, description string) error
	DeleteSnapshot(ctx context.Context, nodeID, vmID, snapshotID string) error
	RevertSnapshot(ctx context.Context, nodeID, vmID, snapshotID string) error
}

// BackupProvider is optionally implemented by providers that support backup monitoring.
type BackupProvider interface {
	GetBackupJobs(ctx context.Context) ([]BackupJob, error)
	GetBackupHistory(ctx context.Context, vmID string) ([]Backup, error)
}

// MetricsProvider is optionally implemented by providers that support historical metrics.
type MetricsProvider interface {
	// GetNodeMetrics returns time-series metrics for a node.
	GetNodeMetrics(ctx context.Context, nodeID string, timeframe string) ([]MetricPoint, error)
	// GetVMMetrics returns time-series metrics for a VM.
	GetVMMetrics(ctx context.Context, nodeID, vmID string, timeframe string) ([]MetricPoint, error)
}

// MetricPoint represents a single data point in a time series.
type MetricPoint struct {
	Timestamp int64   `json:"timestamp"`
	CPU       float64 `json:"cpu,omitempty"`
	MemUsed   uint64  `json:"memUsed,omitempty"`
	MemTotal  uint64  `json:"memTotal,omitempty"`
	DiskRead  int64   `json:"diskRead,omitempty"`
	DiskWrite int64   `json:"diskWrite,omitempty"`
	NetIn     int64   `json:"netIn,omitempty"`
	NetOut    int64   `json:"netOut,omitempty"`
}
