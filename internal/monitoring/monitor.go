package monitoring

import (
	"context"
	stderrors "errors"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/memory"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/discovery"
	"github.com/rcourtman/pulse-go-rewrite/internal/errors"
	"github.com/rcourtman/pulse-go-rewrite/internal/logging"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	"github.com/rcourtman/pulse-go-rewrite/internal/system"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pmg"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	defaultTaskTimeout = 90 * time.Second
	minTaskTimeout     = 30 * time.Second
	maxTaskTimeout     = 3 * time.Minute
)

const mockKeepRealPollingEnv = "PULSE_MOCK_KEEP_REAL_POLLING"

func keepRealPollingInMockMode() bool {
	raw := strings.TrimSpace(strings.ToLower(os.Getenv(mockKeepRealPollingEnv)))
	switch raw {
	case "", "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return true
	}
}

// newProxmoxClientFunc is a variable that holds the function to create a new Proxmox client.
// It is used to allow mocking the client creation in tests.
var newProxmoxClientFunc = func(cfg proxmox.ClientConfig) (PVEClientInterface, error) {
	return proxmox.NewClient(cfg)
}

// PVEClientInterface defines the interface for PVE clients (both regular and cluster)
type PVEClientInterface interface {
	GetNodes(ctx context.Context) ([]proxmox.Node, error)
	GetNodeStatus(ctx context.Context, node string) (*proxmox.NodeStatus, error)
	GetNodeRRDData(ctx context.Context, node string, timeframe string, cf string, ds []string) ([]proxmox.NodeRRDPoint, error)
	GetLXCRRDData(ctx context.Context, node string, vmid int, timeframe string, cf string, ds []string) ([]proxmox.GuestRRDPoint, error)
	GetVMs(ctx context.Context, node string) ([]proxmox.VM, error)
	GetContainers(ctx context.Context, node string) ([]proxmox.Container, error)
	GetStorage(ctx context.Context, node string) ([]proxmox.Storage, error)
	GetAllStorage(ctx context.Context) ([]proxmox.Storage, error)
	GetBackupTasks(ctx context.Context) ([]proxmox.Task, error)
	GetReplicationStatus(ctx context.Context) ([]proxmox.ReplicationJob, error)
	GetStorageContent(ctx context.Context, node, storage string) ([]proxmox.StorageContent, error)
	GetVMSnapshots(ctx context.Context, node string, vmid int) ([]proxmox.Snapshot, error)
	GetContainerSnapshots(ctx context.Context, node string, vmid int) ([]proxmox.Snapshot, error)
	GetVMStatus(ctx context.Context, node string, vmid int) (*proxmox.VMStatus, error)
	GetContainerStatus(ctx context.Context, node string, vmid int) (*proxmox.Container, error)
	GetContainerConfig(ctx context.Context, node string, vmid int) (map[string]interface{}, error)
	GetContainerInterfaces(ctx context.Context, node string, vmid int) ([]proxmox.ContainerInterface, error)
	GetClusterResources(ctx context.Context, resourceType string) ([]proxmox.ClusterResource, error)
	IsClusterMember(ctx context.Context) (bool, error)
	GetVMFSInfo(ctx context.Context, node string, vmid int) ([]proxmox.VMFileSystem, error)
	GetVMNetworkInterfaces(ctx context.Context, node string, vmid int) ([]proxmox.VMNetworkInterface, error)
	GetVMAgentInfo(ctx context.Context, node string, vmid int) (map[string]interface{}, error)
	GetVMAgentVersion(ctx context.Context, node string, vmid int) (string, error)
	GetZFSPoolStatus(ctx context.Context, node string) ([]proxmox.ZFSPoolStatus, error)
	GetZFSPoolsWithDetails(ctx context.Context, node string) ([]proxmox.ZFSPoolInfo, error)
	GetDisks(ctx context.Context, node string) ([]proxmox.Disk, error)
	GetNodePendingUpdates(ctx context.Context, node string) ([]proxmox.AptPackage, error)
	GetCephStatus(ctx context.Context) (*proxmox.CephStatus, error)
	GetCephDF(ctx context.Context) (*proxmox.CephDF, error)
}

// ResourceStoreInterface provides methods for polling optimization and resource access.
// When an agent is monitoring a node, we can reduce API polling for that node.
type ResourceStoreInterface interface {
	// ShouldSkipAPIPolling returns true if API polling should be skipped for the hostname
	// because an agent is providing richer data.
	ShouldSkipAPIPolling(hostname string) bool
	// GetPollingRecommendations returns a map of hostname -> polling multiplier.
	// 0 = skip entirely, 0.5 = half frequency, 1 = normal
	GetPollingRecommendations() map[string]float64
	// GetAll returns all resources in the store (for WebSocket broadcasts)
	GetAll() []unifiedresources.LegacyResource
	// PopulateFromSnapshot updates the store with data from a StateSnapshot
	PopulateFromSnapshot(snapshot models.StateSnapshot)
}

func getNodeDisplayName(instance *config.PVEInstance, nodeName string) string {
	baseName := strings.TrimSpace(nodeName)
	if baseName == "" {
		baseName = "unknown-node"
	}

	if instance == nil {
		return baseName
	}

	friendly := strings.TrimSpace(instance.Name)

	if instance.IsCluster {
		if endpointLabel := lookupClusterEndpointLabel(instance, nodeName); endpointLabel != "" {
			return endpointLabel
		}

		if baseName != "" && baseName != "unknown-node" {
			return baseName
		}

		if friendly != "" {
			return friendly
		}

		return baseName
	}

	if friendly != "" {
		return friendly
	}

	if baseName != "" && baseName != "unknown-node" {
		return baseName
	}

	if label := normalizeEndpointHost(instance.Host); label != "" && !isLikelyIPAddress(label) {
		return label
	}

	return baseName
}

func (m *Monitor) getInstanceConfig(instanceName string) *config.PVEInstance {
	if m == nil || m.config == nil {
		return nil
	}
	for i := range m.config.PVEInstances {
		if strings.EqualFold(m.config.PVEInstances[i].Name, instanceName) {
			return &m.config.PVEInstances[i]
		}
	}
	return nil
}

func mergeNVMeTempsIntoDisks(disks []models.PhysicalDisk, nodes []models.Node) []models.PhysicalDisk {
	if len(disks) == 0 || len(nodes) == 0 {
		return disks
	}

	// Build temperature maps by node for both SMART and legacy NVMe data
	smartTempsByNode := make(map[string][]models.DiskTemp)
	nvmeTempsByNode := make(map[string][]models.NVMeTemp)

	for _, node := range nodes {
		log.Debug().
			Str("nodeName", node.Name).
			Bool("hasTemp", node.Temperature != nil).
			Bool("tempAvailable", node.Temperature != nil && node.Temperature.Available).
			Int("smartCount", func() int {
				if node.Temperature != nil {
					return len(node.Temperature.SMART)
				}
				return 0
			}()).
			Msg("mergeNVMeTempsIntoDisks: checking node temperature")

		if node.Temperature == nil || !node.Temperature.Available {
			continue
		}

		// Collect SMART temps (preferred source)
		if len(node.Temperature.SMART) > 0 {
			temps := make([]models.DiskTemp, len(node.Temperature.SMART))
			copy(temps, node.Temperature.SMART)
			smartTempsByNode[node.Name] = temps
			log.Debug().
				Str("nodeName", node.Name).
				Int("smartTempCount", len(temps)).
				Msg("mergeNVMeTempsIntoDisks: collected SMART temps for node")
		}

		// Collect legacy NVMe temps as fallback
		if len(node.Temperature.NVMe) > 0 {
			temps := make([]models.NVMeTemp, len(node.Temperature.NVMe))
			copy(temps, node.Temperature.NVMe)
			sort.Slice(temps, func(i, j int) bool {
				return temps[i].Device < temps[j].Device
			})
			nvmeTempsByNode[node.Name] = temps
		}
	}

	if len(smartTempsByNode) == 0 && len(nvmeTempsByNode) == 0 {
		log.Debug().
			Int("diskCount", len(disks)).
			Msg("mergeNVMeTempsIntoDisks: no SMART or NVMe temperature data available")
		return disks
	}

	log.Debug().
		Int("smartNodeCount", len(smartTempsByNode)).
		Int("nvmeNodeCount", len(nvmeTempsByNode)).
		Int("diskCount", len(disks)).
		Msg("mergeNVMeTempsIntoDisks: starting disk temperature merge")

	updated := make([]models.PhysicalDisk, len(disks))
	copy(updated, disks)

	// Process SMART temperatures first (preferred method)
	for i := range updated {
		smartTemps, ok := smartTempsByNode[updated[i].Node]
		log.Debug().
			Str("diskDevPath", updated[i].DevPath).
			Str("diskNode", updated[i].Node).
			Bool("hasSMARTData", ok).
			Int("smartTempCount", len(smartTemps)).
			Msg("mergeNVMeTempsIntoDisks: checking disk for SMART temp match")
		if !ok || len(smartTemps) == 0 {
			continue
		}

		// Try to match by WWN (most reliable)
		if updated[i].WWN != "" {
			for _, temp := range smartTemps {
				if temp.WWN != "" && strings.EqualFold(temp.WWN, updated[i].WWN) {
					if temp.Temperature > 0 && !temp.StandbySkipped {
						updated[i].Temperature = temp.Temperature
						log.Debug().
							Str("disk", updated[i].DevPath).
							Str("wwn", updated[i].WWN).
							Int("temp", temp.Temperature).
							Msg("Matched SMART temperature by WWN")
					}
					continue
				}
			}
		}

		// Fall back to serial number match (case-insensitive)
		if updated[i].Serial != "" && updated[i].Temperature == 0 {
			for _, temp := range smartTemps {
				if temp.Serial != "" && strings.EqualFold(temp.Serial, updated[i].Serial) {
					if temp.Temperature > 0 && !temp.StandbySkipped {
						updated[i].Temperature = temp.Temperature
						log.Debug().
							Str("disk", updated[i].DevPath).
							Str("serial", updated[i].Serial).
							Int("temp", temp.Temperature).
							Msg("Matched SMART temperature by serial")
					}
					continue
				}
			}
		}

		// Last resort: match by device path (normalized)
		if updated[i].Temperature == 0 {
			normalizedDevPath := strings.TrimPrefix(updated[i].DevPath, "/dev/")
			for _, temp := range smartTemps {
				normalizedTempDev := strings.TrimPrefix(temp.Device, "/dev/")
				if normalizedTempDev == normalizedDevPath {
					if temp.Temperature > 0 && !temp.StandbySkipped {
						updated[i].Temperature = temp.Temperature
						log.Debug().
							Str("disk", updated[i].DevPath).
							Int("temp", temp.Temperature).
							Msg("Matched SMART temperature by device path")
					}
					break
				}
			}
		}
	}

	// Process legacy NVMe temperatures for disks that didn't get SMART data
	disksByNode := make(map[string][]int)
	for i := range updated {
		if strings.EqualFold(updated[i].Type, "nvme") && updated[i].Temperature == 0 {
			disksByNode[updated[i].Node] = append(disksByNode[updated[i].Node], i)
		}
	}

	for nodeName, diskIndexes := range disksByNode {
		temps, ok := nvmeTempsByNode[nodeName]
		if !ok || len(temps) == 0 {
			continue
		}

		sort.Slice(diskIndexes, func(i, j int) bool {
			return updated[diskIndexes[i]].DevPath < updated[diskIndexes[j]].DevPath
		})

		for idx, diskIdx := range diskIndexes {
			if idx >= len(temps) {
				break
			}

			tempVal := temps[idx].Temp
			if tempVal <= 0 || math.IsNaN(tempVal) {
				continue
			}

			updated[diskIdx].Temperature = int(math.Round(tempVal))
			log.Debug().
				Str("disk", updated[diskIdx].DevPath).
				Int("temp", updated[diskIdx].Temperature).
				Msg("Matched legacy NVMe temperature by index")
		}
	}

	return updated
}

// mergeHostAgentSMARTIntoDisks merges SMART temperature data from linked host agents
// into physical disks for Proxmox nodes. This allows disk temps collected by the
// pulse-agent running on a PVE node to populate the Physical Disks view.
func mergeHostAgentSMARTIntoDisks(disks []models.PhysicalDisk, nodes []models.Node, hosts []models.Host) []models.PhysicalDisk {
	if len(disks) == 0 || len(nodes) == 0 || len(hosts) == 0 {
		return disks
	}

	// Build a map of host ID to host for quick lookup
	hostByID := make(map[string]*models.Host, len(hosts))
	for i := range hosts {
		hostByID[hosts[i].ID] = &hosts[i]
	}

	// Build a map of node name to linked host's SMART data
	smartByNodeName := make(map[string][]models.HostDiskSMART)
	for _, node := range nodes {
		if node.LinkedHostAgentID == "" {
			continue
		}
		host, ok := hostByID[node.LinkedHostAgentID]
		if !ok || len(host.Sensors.SMART) == 0 {
			continue
		}
		smartByNodeName[node.Name] = host.Sensors.SMART
		log.Debug().
			Str("nodeName", node.Name).
			Str("hostAgentID", node.LinkedHostAgentID).
			Int("smartDiskCount", len(host.Sensors.SMART)).
			Msg("mergeHostAgentSMARTIntoDisks: found linked host agent with SMART data")
	}

	if len(smartByNodeName) == 0 {
		return disks
	}

	updated := make([]models.PhysicalDisk, len(disks))
	copy(updated, disks)

	for i := range updated {
		smartData, ok := smartByNodeName[updated[i].Node]
		if !ok || len(smartData) == 0 {
			continue
		}

		// Find matching SMART entry by WWN, serial, or device path
		var matched *models.HostDiskSMART

		// Try to match by WWN (most reliable)
		if updated[i].WWN != "" {
			for j := range smartData {
				if smartData[j].WWN != "" && strings.EqualFold(smartData[j].WWN, updated[i].WWN) {
					matched = &smartData[j]
					break
				}
			}
		}

		// Fall back to serial number match
		if matched == nil && updated[i].Serial != "" {
			for j := range smartData {
				if smartData[j].Serial != "" && strings.EqualFold(smartData[j].Serial, updated[i].Serial) {
					matched = &smartData[j]
					break
				}
			}
		}

		// Last resort: match by device path
		if matched == nil {
			normalizedDevPath := strings.TrimPrefix(updated[i].DevPath, "/dev/")
			for j := range smartData {
				normalizedDiskDev := strings.TrimPrefix(smartData[j].Device, "/dev/")
				if normalizedDiskDev == normalizedDevPath {
					matched = &smartData[j]
					break
				}
			}
		}

		if matched == nil || matched.Standby {
			continue
		}

		// Merge temperature if not already set
		if updated[i].Temperature == 0 && matched.Temperature > 0 {
			updated[i].Temperature = matched.Temperature
			log.Debug().
				Str("device", updated[i].DevPath).
				Int("temp", matched.Temperature).
				Msg("Matched host agent SMART temperature")
		}

		// Always merge SMART attributes from host agent
		if matched.Attributes != nil {
			updated[i].SmartAttributes = matched.Attributes
		}
	}

	return updated
}

// writeSMARTMetrics writes SMART attribute metrics to the persistent metrics store for a single disk.
func (m *Monitor) writeSMARTMetrics(disk models.PhysicalDisk, now time.Time) {
	// Determine resource ID: serial (preferred) → WWN → composite fallback
	resourceID := disk.Serial
	if resourceID == "" {
		resourceID = disk.WWN
	}
	if resourceID == "" {
		resourceID = fmt.Sprintf("%s-%s-%s", disk.Instance, disk.Node, strings.ReplaceAll(disk.DevPath, "/", "-"))
	}

	// Temperature (always write if > 0)
	if disk.Temperature > 0 {
		m.metricsStore.Write("disk", resourceID, "smart_temp", float64(disk.Temperature), now)
	}

	attrs := disk.SmartAttributes
	if attrs == nil {
		return
	}

	// Common
	if attrs.PowerOnHours != nil {
		m.metricsStore.Write("disk", resourceID, "smart_power_on_hours", float64(*attrs.PowerOnHours), now)
	}
	if attrs.PowerCycles != nil {
		m.metricsStore.Write("disk", resourceID, "smart_power_cycles", float64(*attrs.PowerCycles), now)
	}

	// SATA-specific
	if attrs.ReallocatedSectors != nil {
		m.metricsStore.Write("disk", resourceID, "smart_reallocated_sectors", float64(*attrs.ReallocatedSectors), now)
	}
	if attrs.PendingSectors != nil {
		m.metricsStore.Write("disk", resourceID, "smart_pending_sectors", float64(*attrs.PendingSectors), now)
	}
	if attrs.OfflineUncorrectable != nil {
		m.metricsStore.Write("disk", resourceID, "smart_offline_uncorrectable", float64(*attrs.OfflineUncorrectable), now)
	}
	if attrs.UDMACRCErrors != nil {
		m.metricsStore.Write("disk", resourceID, "smart_crc_errors", float64(*attrs.UDMACRCErrors), now)
	}

	// NVMe-specific
	if attrs.PercentageUsed != nil {
		m.metricsStore.Write("disk", resourceID, "smart_percentage_used", float64(*attrs.PercentageUsed), now)
	}
	if attrs.AvailableSpare != nil {
		m.metricsStore.Write("disk", resourceID, "smart_available_spare", float64(*attrs.AvailableSpare), now)
	}
	if attrs.MediaErrors != nil {
		m.metricsStore.Write("disk", resourceID, "smart_media_errors", float64(*attrs.MediaErrors), now)
	}
	if attrs.UnsafeShutdowns != nil {
		m.metricsStore.Write("disk", resourceID, "smart_unsafe_shutdowns", float64(*attrs.UnsafeShutdowns), now)
	}
}

// PollExecutor defines the contract for executing polling tasks.
type PollExecutor interface {
	Execute(ctx context.Context, task PollTask)
}

type realExecutor struct {
	monitor *Monitor
}

func newRealExecutor(m *Monitor) PollExecutor {
	return &realExecutor{monitor: m}
}

func (r *realExecutor) Execute(ctx context.Context, task PollTask) {
	if r == nil || r.monitor == nil {
		return
	}

	switch strings.ToLower(task.InstanceType) {
	case "pve":
		if task.PVEClient == nil {
			log.Warn().
				Str("instance", task.InstanceName).
				Msg("PollExecutor received nil PVE client")
			return
		}
		r.monitor.pollPVEInstance(ctx, task.InstanceName, task.PVEClient)
	case "pbs":
		if task.PBSClient == nil {
			log.Warn().
				Str("instance", task.InstanceName).
				Msg("PollExecutor received nil PBS client")
			return
		}
		r.monitor.pollPBSInstance(ctx, task.InstanceName, task.PBSClient)
	case "pmg":
		if task.PMGClient == nil {
			log.Warn().
				Str("instance", task.InstanceName).
				Msg("PollExecutor received nil PMG client")
			return
		}
		r.monitor.pollPMGInstance(ctx, task.InstanceName, task.PMGClient)
	default:
		if logging.IsLevelEnabled(zerolog.DebugLevel) {
			log.Debug().
				Str("instance", task.InstanceName).
				Str("type", task.InstanceType).
				Msg("PollExecutor received unsupported task type")
		}
	}
}

type instanceInfo struct {
	Key         string
	Type        InstanceType
	DisplayName string
	Connection  string
	Metadata    map[string]string
}

type pollStatus struct {
	LastSuccess         time.Time
	LastErrorAt         time.Time
	LastErrorMessage    string
	LastErrorCategory   string
	ConsecutiveFailures int
	FirstFailureAt      time.Time
}

type dlqInsight struct {
	Reason       string
	FirstAttempt time.Time
	LastAttempt  time.Time
	RetryCount   int
	NextRetry    time.Time
}

type ErrorDetail struct {
	At       time.Time `json:"at"`
	Message  string    `json:"message"`
	Category string    `json:"category"`
}

type InstancePollStatus struct {
	LastSuccess         *time.Time   `json:"lastSuccess,omitempty"`
	LastError           *ErrorDetail `json:"lastError,omitempty"`
	ConsecutiveFailures int          `json:"consecutiveFailures"`
	FirstFailureAt      *time.Time   `json:"firstFailureAt,omitempty"`
}

type InstanceBreaker struct {
	State          string     `json:"state"`
	Since          *time.Time `json:"since,omitempty"`
	LastTransition *time.Time `json:"lastTransition,omitempty"`
	RetryAt        *time.Time `json:"retryAt,omitempty"`
	FailureCount   int        `json:"failureCount"`
}

type InstanceDLQ struct {
	Present      bool       `json:"present"`
	Reason       string     `json:"reason,omitempty"`
	FirstAttempt *time.Time `json:"firstAttempt,omitempty"`
	LastAttempt  *time.Time `json:"lastAttempt,omitempty"`
	RetryCount   int        `json:"retryCount,omitempty"`
	NextRetry    *time.Time `json:"nextRetry,omitempty"`
}

type InstanceHealth struct {
	Key         string             `json:"key"`
	Type        string             `json:"type"`
	DisplayName string             `json:"displayName"`
	Instance    string             `json:"instance"`
	Connection  string             `json:"connection"`
	PollStatus  InstancePollStatus `json:"pollStatus"`
	Breaker     InstanceBreaker    `json:"breaker"`
	DeadLetter  InstanceDLQ        `json:"deadLetter"`
	Warnings    []string           `json:"warnings,omitempty"`
}

// Monitor handles all monitoring operations
type Monitor struct {
	config                     *config.Config
	state                      *models.State
	orgID                      string // Organization ID for tenant isolation (empty = default/legacy)
	pveClients                 map[string]PVEClientInterface
	pbsClients                 map[string]*pbs.Client
	pmgClients                 map[string]*pmg.Client
	pollMetrics                *PollMetrics
	scheduler                  *AdaptiveScheduler
	stalenessTracker           *StalenessTracker
	taskQueue                  *TaskQueue
	pollTimeout                time.Duration
	circuitBreakers            map[string]*circuitBreaker
	deadLetterQueue            *TaskQueue
	failureCounts              map[string]int
	lastOutcome                map[string]taskOutcome
	backoffCfg                 backoffConfig
	rng                        *rand.Rand
	maxRetryAttempts           int
	tempCollector              *TemperatureCollector // SSH-based temperature collector
	guestMetadataStore         *config.GuestMetadataStore
	dockerMetadataStore        *config.DockerMetadataStore
	hostMetadataStore          *config.HostMetadataStore
	mu                         sync.RWMutex
	startTime                  time.Time
	rateTracker                *RateTracker
	metricsHistory             *MetricsHistory
	metricsStore               *metrics.Store // Persistent SQLite metrics storage
	alertManager               *alerts.Manager
	alertResolvedAICallback    func(*alerts.Alert)
	alertTriggeredAICallback   func(*alerts.Alert)
	incidentStore              *memory.IncidentStore
	notificationMgr            *notifications.NotificationManager
	configPersist              *config.ConfigPersistence
	discoveryService           *discovery.Service                         // Background discovery service
	activePollCount            int32                                      // Number of active polling operations
	pollCounter                int64                                      // Counter for polling cycles
	authFailures               map[string]int                             // Track consecutive auth failures per node
	lastAuthAttempt            map[string]time.Time                       // Track last auth attempt time
	lastClusterCheck           map[string]time.Time                       // Track last cluster check for standalone nodes
	lastPhysicalDiskPoll       map[string]time.Time                       // Track last physical disk poll time per instance
	lastPVEBackupPoll          map[string]time.Time                       // Track last PVE backup poll per instance
	lastPBSBackupPoll          map[string]time.Time                       // Track last PBS backup poll per instance
	backupPermissionWarnings   map[string]string                          // Track backup permission issues per instance (instance -> warning message)
	persistence                *config.ConfigPersistence                  // Add persistence for saving updated configs
	pbsBackupPollers           map[string]bool                            // Track PBS backup polling goroutines per instance
	pbsBackupCacheTime         map[string]map[pbsBackupGroupKey]time.Time // Track when each PBS backup group was last fetched
	runtimeCtx                 context.Context                            // Context used while monitor is running
	wsHub                      *websocket.Hub                             // Hub used for broadcasting state
	diagMu                     sync.RWMutex                               // Protects diagnostic snapshot maps
	nodeSnapshots              map[string]NodeMemorySnapshot
	guestSnapshots             map[string]GuestMemorySnapshot
	rrdCacheMu                 sync.RWMutex // Protects RRD memavailable cache
	nodeRRDMemCache            map[string]rrdMemCacheEntry
	removedDockerHosts         map[string]time.Time // Track deliberately removed Docker hosts (ID -> removal time)
	dockerTokenBindings        map[string]string    // Track token ID -> agent ID bindings to enforce uniqueness
	removedKubernetesClusters  map[string]time.Time // Track deliberately removed Kubernetes clusters (ID -> removal time)
	kubernetesTokenBindings    map[string]string    // Track token ID -> agent ID bindings to enforce uniqueness
	hostTokenBindings          map[string]string    // Track tokenID:hostname -> host identity bindings
	dockerCommands             map[string]*dockerHostCommand
	dockerCommandIndex         map[string]string
	guestMetadataMu            sync.RWMutex
	guestMetadataCache         map[string]guestMetadataCacheEntry
	guestMetadataLimiterMu     sync.Mutex
	guestMetadataLimiter       map[string]time.Time
	guestMetadataSlots         chan struct{}
	guestMetadataMinRefresh    time.Duration
	guestMetadataRefreshJitter time.Duration
	guestMetadataRetryBackoff  time.Duration
	guestMetadataHoldDuration  time.Duration
	// Configurable guest agent timeouts (refs #592)
	guestAgentFSInfoTimeout  time.Duration
	guestAgentNetworkTimeout time.Duration
	guestAgentOSInfoTimeout  time.Duration
	guestAgentVersionTimeout time.Duration
	guestAgentRetries        int
	executor                 PollExecutor
	breakerBaseRetry         time.Duration
	breakerMaxDelay          time.Duration
	breakerHalfOpenWindow    time.Duration
	instanceInfoCache        map[string]*instanceInfo
	pollStatusMap            map[string]*pollStatus
	dlqInsightMap            map[string]*dlqInsight
	nodeLastOnline           map[string]time.Time           // Track last time each node was seen online (for grace period)
	nodePendingUpdatesCache  map[string]pendingUpdatesCache // Cache pending updates per node (checked every 30 min)
	resourceStore            ResourceStoreInterface         // Optional unified resource store for polling optimization
	mockMetricsCancel        context.CancelFunc
	mockMetricsWg            sync.WaitGroup
	dockerChecker            DockerChecker // Optional Docker checker for LXC containers
	// Agent profile cache to avoid disk I/O on every report (refs #1094)
	agentProfileCacheMu sync.RWMutex
	agentProfileCache   *agentProfileCacheEntry
}

type rrdMemCacheEntry struct {
	available uint64
	used      uint64
	total     uint64
	netIn     float64
	netOut    float64
	hasNetIn  bool
	hasNetOut bool
	fetchedAt time.Time
}

// pendingUpdatesCache caches apt pending updates count per node
type pendingUpdatesCache struct {
	count     int
	checkedAt time.Time
}

// TTL for pending updates cache (30 minutes - balance between freshness and API load)
const pendingUpdatesCacheTTL = 30 * time.Minute

// agentProfileCacheEntry caches agent profiles and assignments to avoid disk I/O on every agent report.
// TTL is 60 seconds to balance freshness with performance.
type agentProfileCacheEntry struct {
	profiles    []models.AgentProfile
	assignments []models.AgentProfileAssignment
	loadedAt    time.Time
}

const agentProfileCacheTTL = 60 * time.Second

// shouldRunBackupPoll determines whether a backup polling cycle should execute.
// Returns whether polling should run, a human-readable skip reason, and the timestamp to record.
func (m *Monitor) shouldRunBackupPoll(last time.Time, now time.Time) (bool, string, time.Time) {
	if m == nil || m.config == nil {
		return false, "configuration unavailable", last
	}

	if !m.config.EnableBackupPolling {
		return false, "backup polling globally disabled", last
	}

	interval := m.config.BackupPollingInterval
	if interval > 0 {
		if !last.IsZero() && now.Sub(last) < interval {
			next := last.Add(interval)
			return false, fmt.Sprintf("next run scheduled for %s", next.Format(time.RFC3339)), last
		}
		return true, "", now
	}

	backupCycles := m.config.BackupPollingCycles
	if backupCycles <= 0 {
		backupCycles = 10
	}

	if m.pollCounter%int64(backupCycles) == 0 || m.pollCounter == 1 {
		return true, "", now
	}

	remaining := int64(backupCycles) - (m.pollCounter % int64(backupCycles))
	return false, fmt.Sprintf("next run in %d polling cycles", remaining), last
}

const (
	dockerConnectionPrefix           = "docker-"
	kubernetesConnectionPrefix       = "kubernetes-"
	hostConnectionPrefix             = "host-"
	dockerOfflineGraceMultiplier     = 4
	dockerMinimumHealthWindow        = 30 * time.Second
	dockerMaximumHealthWindow        = 10 * time.Minute
	kubernetesOfflineGraceMultiplier = 4
	kubernetesMinimumHealthWindow    = 30 * time.Second
	kubernetesMaximumHealthWindow    = 10 * time.Minute
	hostOfflineGraceMultiplier       = 6
	hostMinimumHealthWindow          = 60 * time.Second
	hostMaximumHealthWindow          = 10 * time.Minute
	nodeOfflineGracePeriod           = 60 * time.Second // Grace period before marking Proxmox nodes offline
	nodeRRDCacheTTL                  = 30 * time.Second
	nodeRRDRequestTimeout            = 2 * time.Second
)

type taskOutcome struct {
	success    bool
	transient  bool
	err        error
	recordedAt time.Time
}

func (m *Monitor) getNodeRRDMetrics(ctx context.Context, client PVEClientInterface, nodeName string) (rrdMemCacheEntry, error) {
	if client == nil || nodeName == "" {
		return rrdMemCacheEntry{}, fmt.Errorf("invalid arguments for RRD lookup")
	}

	now := time.Now()

	m.rrdCacheMu.RLock()
	if entry, ok := m.nodeRRDMemCache[nodeName]; ok && now.Sub(entry.fetchedAt) < nodeRRDCacheTTL {
		m.rrdCacheMu.RUnlock()
		return entry, nil
	}
	m.rrdCacheMu.RUnlock()

	requestCtx, cancel := context.WithTimeout(ctx, nodeRRDRequestTimeout)
	defer cancel()

	points, err := client.GetNodeRRDData(requestCtx, nodeName, "hour", "AVERAGE", []string{"memavailable", "memused", "memtotal", "netin", "netout"})
	if err != nil {
		return rrdMemCacheEntry{}, err
	}

	var memAvailable uint64
	var memUsed uint64
	var memTotal uint64
	var netIn float64
	var netOut float64
	var hasNetIn bool
	var hasNetOut bool

	for i := len(points) - 1; i >= 0; i-- {
		point := points[i]

		if memTotal == 0 && point.MemTotal != nil && !math.IsNaN(*point.MemTotal) && *point.MemTotal > 0 {
			memTotal = uint64(math.Round(*point.MemTotal))
		}

		if memAvailable == 0 && point.MemAvailable != nil && !math.IsNaN(*point.MemAvailable) && *point.MemAvailable > 0 {
			memAvailable = uint64(math.Round(*point.MemAvailable))
		}

		if memUsed == 0 && point.MemUsed != nil && !math.IsNaN(*point.MemUsed) && *point.MemUsed > 0 {
			memUsed = uint64(math.Round(*point.MemUsed))
		}

		if !hasNetIn && point.NetIn != nil && !math.IsNaN(*point.NetIn) {
			netIn = *point.NetIn
			hasNetIn = true
		}
		if !hasNetOut && point.NetOut != nil && !math.IsNaN(*point.NetOut) {
			netOut = *point.NetOut
			hasNetOut = true
		}
	}

	if memTotal > 0 {
		if memAvailable > memTotal {
			memAvailable = memTotal
		}
		if memUsed > memTotal {
			memUsed = memTotal
		}
	}

	if memAvailable == 0 && memUsed == 0 && !hasNetIn && !hasNetOut {
		return rrdMemCacheEntry{}, fmt.Errorf("rrd node metrics not present")
	}

	entry := rrdMemCacheEntry{
		available: memAvailable,
		used:      memUsed,
		total:     memTotal,
		netIn:     netIn,
		netOut:    netOut,
		hasNetIn:  hasNetIn,
		hasNetOut: hasNetOut,
		fetchedAt: now,
	}

	m.rrdCacheMu.Lock()
	m.nodeRRDMemCache[nodeName] = entry
	m.rrdCacheMu.Unlock()

	return entry, nil
}

// RemoveDockerHost removes a docker host from the shared state and clears related alerts.
func (m *Monitor) GetConnectionStatuses() map[string]bool {
	if mock.IsMockEnabled() {
		statuses := make(map[string]bool)
		state := mock.GetMockState()
		for _, node := range state.Nodes {
			key := "pve-" + node.Name
			statuses[key] = strings.ToLower(node.Status) == "online"
			if node.Host != "" {
				statuses[node.Host] = strings.ToLower(node.Status) == "online"
			}
		}
		for _, pbsInst := range state.PBSInstances {
			key := "pbs-" + pbsInst.Name
			statuses[key] = strings.ToLower(pbsInst.Status) != "offline"
			if pbsInst.Host != "" {
				statuses[pbsInst.Host] = strings.ToLower(pbsInst.Status) != "offline"
			}
		}

		for _, dockerHost := range state.DockerHosts {
			key := dockerConnectionPrefix + dockerHost.ID
			statuses[key] = strings.ToLower(dockerHost.Status) == "online"
		}
		return statuses
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	statuses := make(map[string]bool)

	// Check all configured PVE nodes (not just ones with clients)
	for _, pve := range m.config.PVEInstances {
		key := "pve-" + pve.Name
		// Check if we have a client for this node
		if client, exists := m.pveClients[pve.Name]; exists && client != nil {
			// We have a client, check actual connection health from state
			if m.state != nil && m.state.ConnectionHealth != nil {
				statuses[key] = m.state.ConnectionHealth[pve.Name]
			} else {
				statuses[key] = true // Assume connected if we have a client
			}
		} else {
			// No client means disconnected
			statuses[key] = false
		}
	}

	// Check all configured PBS nodes (not just ones with clients)
	for _, pbs := range m.config.PBSInstances {
		key := "pbs-" + pbs.Name
		// Check if we have a client for this node
		if client, exists := m.pbsClients[pbs.Name]; exists && client != nil {
			// We have a client, check actual connection health from state
			if m.state != nil && m.state.ConnectionHealth != nil {
				statuses[key] = m.state.ConnectionHealth["pbs-"+pbs.Name]
			} else {
				statuses[key] = true // Assume connected if we have a client
			}
		} else {
			// No client means disconnected
			statuses[key] = false
		}
	}

	return statuses
}

// checkContainerizedTempMonitoring logs a security warning if Pulse is running
// in a container with SSH-based temperature monitoring enabled
func checkContainerizedTempMonitoring() {
	// Check if running in container
	isContainer := os.Getenv("PULSE_DOCKER") == "true" || system.InContainer()
	if !isContainer {
		return
	}

	// Check if SSH keys exist (indicates temperature monitoring is configured)
	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		homeDir = "/home/pulse"
	}
	sshKeyPath := homeDir + "/.ssh/id_ed25519"
	if _, err := os.Stat(sshKeyPath); err != nil {
		// No SSH key found, temperature monitoring not configured
		return
	}

	// Log warning
	log.Warn().
		Msg("SECURITY NOTICE: Pulse is running in a container with SSH-based temperature monitoring enabled. " +
			"SSH private keys are stored inside the container, which could be a security risk if the container is compromised. " +
			"Future versions will use agent-based architecture for better security. " +
			"See documentation for hardening recommendations.")
}

// New creates a new Monitor instance
func New(cfg *config.Config) (*Monitor, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	// Initialize temperature collector with sensors SSH key
	// Will use root user for now - can be made configurable later
	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		homeDir = "/home/pulse"
	}
	sshKeyPath := filepath.Join(homeDir, ".ssh/id_ed25519_sensors")
	tempCollector := NewTemperatureCollectorWithPort("root", sshKeyPath, cfg.SSHPort)

	// Security warning if running in container with SSH temperature monitoring
	checkContainerizedTempMonitoring()

	stalenessTracker := NewStalenessTracker(getPollMetrics())
	stalenessTracker.SetBounds(cfg.AdaptivePollingBaseInterval, cfg.AdaptivePollingMaxInterval)
	taskQueue := NewTaskQueue()
	deadLetterQueue := NewTaskQueue()
	breakers := make(map[string]*circuitBreaker)
	failureCounts := make(map[string]int)
	lastOutcome := make(map[string]taskOutcome)
	backoff := backoffConfig{
		Initial:    5 * time.Second,
		Multiplier: 2,
		Jitter:     0.2,
		Max:        5 * time.Minute,
	}

	if cfg.AdaptivePollingEnabled && cfg.AdaptivePollingMaxInterval > 0 && cfg.AdaptivePollingMaxInterval <= 15*time.Second {
		backoff.Initial = 750 * time.Millisecond
		backoff.Max = 6 * time.Second
	}

	var scheduler *AdaptiveScheduler
	if cfg.AdaptivePollingEnabled {
		scheduler = NewAdaptiveScheduler(SchedulerConfig{
			BaseInterval: cfg.AdaptivePollingBaseInterval,
			MinInterval:  cfg.AdaptivePollingMinInterval,
			MaxInterval:  cfg.AdaptivePollingMaxInterval,
		}, stalenessTracker, nil, nil)
	}

	minRefresh := cfg.GuestMetadataMinRefreshInterval
	if minRefresh <= 0 {
		minRefresh = config.DefaultGuestMetadataMinRefresh
	}
	jitter := cfg.GuestMetadataRefreshJitter
	if jitter < 0 {
		jitter = 0
	}
	retryBackoff := cfg.GuestMetadataRetryBackoff
	if retryBackoff <= 0 {
		retryBackoff = config.DefaultGuestMetadataRetryBackoff
	}
	concurrency := cfg.GuestMetadataMaxConcurrent
	if concurrency <= 0 {
		concurrency = config.DefaultGuestMetadataMaxConcurrent
	}
	holdDuration := defaultGuestMetadataHold

	// Load guest agent timeout configuration from environment variables (refs #592)
	guestAgentFSInfoTimeout := parseDurationEnv("GUEST_AGENT_FSINFO_TIMEOUT", defaultGuestAgentFSInfoTimeout)
	guestAgentNetworkTimeout := parseDurationEnv("GUEST_AGENT_NETWORK_TIMEOUT", defaultGuestAgentNetworkTimeout)
	guestAgentOSInfoTimeout := parseDurationEnv("GUEST_AGENT_OSINFO_TIMEOUT", defaultGuestAgentOSInfoTimeout)
	guestAgentVersionTimeout := parseDurationEnv("GUEST_AGENT_VERSION_TIMEOUT", defaultGuestAgentVersionTimeout)
	guestAgentRetries := parseIntEnv("GUEST_AGENT_RETRIES", defaultGuestAgentRetries)

	// Initialize persistent metrics store (SQLite) with configurable retention
	var metricsStore *metrics.Store
	metricsStoreConfig := metrics.DefaultConfig(cfg.DataPath)
	// Override retention settings from config (allows tier-based pricing in future)
	if cfg.MetricsRetentionRawHours > 0 {
		metricsStoreConfig.RetentionRaw = time.Duration(cfg.MetricsRetentionRawHours) * time.Hour
	}
	if cfg.MetricsRetentionMinuteHours > 0 {
		metricsStoreConfig.RetentionMinute = time.Duration(cfg.MetricsRetentionMinuteHours) * time.Hour
	}
	if cfg.MetricsRetentionHourlyDays > 0 {
		metricsStoreConfig.RetentionHourly = time.Duration(cfg.MetricsRetentionHourlyDays) * 24 * time.Hour
	}
	if cfg.MetricsRetentionDailyDays > 0 {
		metricsStoreConfig.RetentionDaily = time.Duration(cfg.MetricsRetentionDailyDays) * 24 * time.Hour
	}

	// In mock mode, extend ALL tier retentions to 90 days to match the seeded
	// data range. Different query ranges use different tiers, so all need coverage.
	// Also increase buffer size to handle heavy initial seeding.
	if mock.IsMockEnabled() {
		metricsStoreConfig.WriteBufferSize = 2000
		metricsStoreConfig.RetentionRaw = 90 * 24 * time.Hour
		metricsStoreConfig.RetentionMinute = 90 * 24 * time.Hour
		metricsStoreConfig.RetentionHourly = 90 * 24 * time.Hour
		metricsStoreConfig.RetentionDaily = 90 * 24 * time.Hour
	}
	ms, err := metrics.NewStore(metricsStoreConfig)
	if err != nil {
		// Do not automatically delete the DB on error, as it causes data loss on transient errors (e.g. locks).
		// If the DB is truly corrupted, the user should manually remove it.
		log.Error().Err(err).Msg("Failed to initialize persistent metrics store - continuing without metrics persistence")
	} else {
		if mock.IsMockEnabled() {
			ms.SetMaxOpenConns(10)
		}
		metricsStore = ms
		log.Info().
			Str("path", metricsStoreConfig.DBPath).
			Dur("retentionRaw", metricsStoreConfig.RetentionRaw).
			Dur("retentionMinute", metricsStoreConfig.RetentionMinute).
			Dur("retentionHourly", metricsStoreConfig.RetentionHourly).
			Dur("retentionDaily", metricsStoreConfig.RetentionDaily).
			Msg("Persistent metrics store initialized with configurable retention")
	}

	incidentStore := memory.NewIncidentStore(memory.IncidentStoreConfig{
		DataDir: cfg.DataPath,
	})

	m := &Monitor{
		config:                     cfg,
		state:                      models.NewState(),
		pveClients:                 make(map[string]PVEClientInterface),
		pbsClients:                 make(map[string]*pbs.Client),
		pmgClients:                 make(map[string]*pmg.Client),
		pollMetrics:                getPollMetrics(),
		scheduler:                  scheduler,
		stalenessTracker:           stalenessTracker,
		taskQueue:                  taskQueue,
		pollTimeout:                derivePollTimeout(cfg),
		deadLetterQueue:            deadLetterQueue,
		circuitBreakers:            breakers,
		failureCounts:              failureCounts,
		lastOutcome:                lastOutcome,
		backoffCfg:                 backoff,
		rng:                        rand.New(rand.NewSource(time.Now().UnixNano())),
		maxRetryAttempts:           5,
		tempCollector:              tempCollector,
		guestMetadataStore:         config.NewGuestMetadataStore(cfg.DataPath, nil),
		dockerMetadataStore:        config.NewDockerMetadataStore(cfg.DataPath, nil),
		hostMetadataStore:          config.NewHostMetadataStore(cfg.DataPath, nil),
		startTime:                  time.Now(),
		rateTracker:                NewRateTracker(),
		metricsHistory:             NewMetricsHistory(1000, 24*time.Hour), // Keep up to 1000 points (~8h @ 30s)
		metricsStore:               metricsStore,                          // Persistent SQLite storage
		alertManager:               alerts.NewManagerWithDataDir(cfg.DataPath),
		incidentStore:              incidentStore,
		notificationMgr:            notifications.NewNotificationManagerWithDataDir(cfg.PublicURL, cfg.DataPath),
		configPersist:              config.NewConfigPersistence(cfg.DataPath),
		discoveryService:           nil, // Will be initialized in Start()
		authFailures:               make(map[string]int),
		lastAuthAttempt:            make(map[string]time.Time),
		lastClusterCheck:           make(map[string]time.Time),
		lastPhysicalDiskPoll:       make(map[string]time.Time),
		lastPVEBackupPoll:          make(map[string]time.Time),
		lastPBSBackupPoll:          make(map[string]time.Time),
		backupPermissionWarnings:   make(map[string]string),
		persistence:                config.NewConfigPersistence(cfg.DataPath),
		pbsBackupPollers:           make(map[string]bool),
		pbsBackupCacheTime:         make(map[string]map[pbsBackupGroupKey]time.Time),
		nodeSnapshots:              make(map[string]NodeMemorySnapshot),
		guestSnapshots:             make(map[string]GuestMemorySnapshot),
		nodeRRDMemCache:            make(map[string]rrdMemCacheEntry),
		removedDockerHosts:         make(map[string]time.Time),
		dockerTokenBindings:        make(map[string]string),
		removedKubernetesClusters:  make(map[string]time.Time),
		kubernetesTokenBindings:    make(map[string]string),
		hostTokenBindings:          make(map[string]string),
		dockerCommands:             make(map[string]*dockerHostCommand),
		dockerCommandIndex:         make(map[string]string),
		guestMetadataCache:         make(map[string]guestMetadataCacheEntry),
		guestMetadataLimiter:       make(map[string]time.Time),
		guestMetadataMinRefresh:    minRefresh,
		guestMetadataRefreshJitter: jitter,
		guestMetadataRetryBackoff:  retryBackoff,
		guestMetadataHoldDuration:  holdDuration,
		guestAgentFSInfoTimeout:    guestAgentFSInfoTimeout,
		guestAgentNetworkTimeout:   guestAgentNetworkTimeout,
		guestAgentOSInfoTimeout:    guestAgentOSInfoTimeout,
		guestAgentVersionTimeout:   guestAgentVersionTimeout,
		guestAgentRetries:          guestAgentRetries,
		instanceInfoCache:          make(map[string]*instanceInfo),
		pollStatusMap:              make(map[string]*pollStatus),
		dlqInsightMap:              make(map[string]*dlqInsight),
		nodeLastOnline:             make(map[string]time.Time),
		nodePendingUpdatesCache:    make(map[string]pendingUpdatesCache),
	}

	m.breakerBaseRetry = 5 * time.Second
	m.breakerMaxDelay = 5 * time.Minute
	m.breakerHalfOpenWindow = 30 * time.Second

	if cfg.AdaptivePollingEnabled && cfg.AdaptivePollingMaxInterval > 0 && cfg.AdaptivePollingMaxInterval <= 15*time.Second {
		m.breakerBaseRetry = 2 * time.Second
		m.breakerMaxDelay = 10 * time.Second
		m.breakerHalfOpenWindow = 2 * time.Second
	}

	m.executor = newRealExecutor(m)
	m.buildInstanceInfoCache(cfg)

	// Initialize state with config values
	m.state.TemperatureMonitoringEnabled = cfg.TemperatureMonitoringEnabled

	if m.pollMetrics != nil {
		m.pollMetrics.ResetQueueDepth(0)
	}

	// Load saved configurations
	if alertConfig, err := m.configPersist.LoadAlertConfig(); err == nil {
		m.alertManager.UpdateConfig(*alertConfig)
		// Apply schedule settings to notification manager
		m.notificationMgr.SetCooldown(alertConfig.Schedule.Cooldown)
		groupWindow := alertConfig.Schedule.Grouping.Window
		if groupWindow == 0 && alertConfig.Schedule.GroupingWindow != 0 {
			groupWindow = alertConfig.Schedule.GroupingWindow
		}
		m.notificationMgr.SetGroupingWindow(groupWindow)
		m.notificationMgr.SetGroupingOptions(
			alertConfig.Schedule.Grouping.ByNode,
			alertConfig.Schedule.Grouping.ByGuest,
		)
		m.notificationMgr.SetNotifyOnResolve(alertConfig.Schedule.NotifyOnResolve)
	} else {
		log.Warn().Err(err).Msg("Failed to load alert configuration")
	}

	if emailConfig, err := m.configPersist.LoadEmailConfig(); err == nil {
		m.notificationMgr.SetEmailConfig(*emailConfig)
	} else {
		log.Warn().Err(err).Msg("Failed to load email configuration")
	}

	if concurrency > 0 {
		m.guestMetadataSlots = make(chan struct{}, concurrency)
	}

	if appriseConfig, err := m.configPersist.LoadAppriseConfig(); err == nil {
		m.notificationMgr.SetAppriseConfig(*appriseConfig)
	} else {
		log.Warn().Err(err).Msg("Failed to load Apprise configuration")
	}

	// Migrate webhooks if needed (from unencrypted to encrypted)
	if err := m.configPersist.MigrateWebhooksIfNeeded(); err != nil {
		log.Warn().Err(err).Msg("Failed to migrate webhooks")
	}

	if webhooks, err := m.configPersist.LoadWebhooks(); err == nil {
		for _, webhook := range webhooks {
			m.notificationMgr.AddWebhook(webhook)
		}
	} else {
		log.Warn().Err(err).Msg("Failed to load webhook configuration")
	}

	// In mock mode we keep real polling enabled by default so production metrics
	// continue to accumulate while the UI renders mock data.
	mockEnabled := mock.IsMockEnabled()
	if mockEnabled && !keepRealPollingInMockMode() {
		log.Info().Msg("Mock mode enabled - real client initialization disabled by env override")
	} else {
		m.initPVEClients(cfg)
		m.initPBSClients(cfg)
		m.initPMGClients(cfg)
	}

	// Initialize state stats
	m.state.Stats = models.Stats{
		StartTime: m.startTime,
		Version:   "2.0.0-go",
	}

	return m, nil
}

// SetExecutor allows tests to override the poll executor; passing nil restores the default executor.
func (m *Monitor) SetExecutor(exec PollExecutor) {
	if m == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if exec == nil {
		m.executor = newRealExecutor(m)
		return
	}

	m.executor = exec
}

func (m *Monitor) buildInstanceInfoCache(cfg *config.Config) {
	if m == nil || cfg == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.instanceInfoCache == nil {
		m.instanceInfoCache = make(map[string]*instanceInfo)
	}

	add := func(instType InstanceType, name string, displayName string, connection string, metadata map[string]string) {
		key := schedulerKey(instType, name)
		m.instanceInfoCache[key] = &instanceInfo{
			Key:         key,
			Type:        instType,
			DisplayName: displayName,
			Connection:  connection,
			Metadata:    metadata,
		}
	}

	// PVE instances
	for _, inst := range cfg.PVEInstances {
		name := strings.TrimSpace(inst.Name)
		if name == "" {
			name = strings.TrimSpace(inst.Host)
		}
		if name == "" {
			name = "pve-instance"
		}
		display := name
		if display == "" {
			display = strings.TrimSpace(inst.Host)
		}
		connection := strings.TrimSpace(inst.Host)
		add(InstanceTypePVE, name, display, connection, nil)
	}

	// PBS instances
	for _, inst := range cfg.PBSInstances {
		name := strings.TrimSpace(inst.Name)
		if name == "" {
			name = strings.TrimSpace(inst.Host)
		}
		if name == "" {
			name = "pbs-instance"
		}
		display := name
		if display == "" {
			display = strings.TrimSpace(inst.Host)
		}
		connection := strings.TrimSpace(inst.Host)
		add(InstanceTypePBS, name, display, connection, nil)
	}

	// PMG instances
	for _, inst := range cfg.PMGInstances {
		name := strings.TrimSpace(inst.Name)
		if name == "" {
			name = strings.TrimSpace(inst.Host)
		}
		if name == "" {
			name = "pmg-instance"
		}
		display := name
		if display == "" {
			display = strings.TrimSpace(inst.Host)
		}
		connection := strings.TrimSpace(inst.Host)
		add(InstanceTypePMG, name, display, connection, nil)
	}
}

func (m *Monitor) getExecutor() PollExecutor {
	m.mu.RLock()
	exec := m.executor
	m.mu.RUnlock()
	return exec
}

func clampInterval(value, min, max time.Duration) time.Duration {
	if value <= 0 {
		return min
	}
	if min > 0 && value < min {
		return min
	}
	if max > 0 && value > max {
		return max
	}
	return value
}

func (m *Monitor) effectivePVEPollingInterval() time.Duration {
	const minInterval = 10 * time.Second
	const maxInterval = time.Hour

	interval := minInterval
	if m != nil && m.config != nil && m.config.PVEPollingInterval > 0 {
		interval = m.config.PVEPollingInterval
	}
	if interval < minInterval {
		interval = minInterval
	}
	if interval > maxInterval {
		interval = maxInterval
	}
	return interval
}

func (m *Monitor) baseIntervalForInstanceType(instanceType InstanceType) time.Duration {
	if m == nil || m.config == nil {
		return DefaultSchedulerConfig().BaseInterval
	}

	switch instanceType {
	case InstanceTypePVE:
		return m.effectivePVEPollingInterval()
	case InstanceTypePBS:
		return clampInterval(m.config.PBSPollingInterval, 10*time.Second, time.Hour)
	case InstanceTypePMG:
		return clampInterval(m.config.PMGPollingInterval, 10*time.Second, time.Hour)
	default:
		base := m.config.AdaptivePollingBaseInterval
		if base <= 0 {
			base = DefaultSchedulerConfig().BaseInterval
		}
		return clampInterval(base, time.Second, 0)
	}
}

// Start begins the monitoring loop
func (m *Monitor) Start(ctx context.Context, wsHub *websocket.Hub) {
	// Consolidate any duplicate cluster instances before starting
	// This fixes the case where multiple agents registered from the same cluster
	m.consolidateDuplicateClusters()

	pollingInterval := m.effectivePVEPollingInterval()
	log.Info().
		Dur("pollingInterval", pollingInterval).
		Msg("Starting monitoring loop")

	m.mu.Lock()
	m.runtimeCtx = ctx
	m.wsHub = wsHub
	m.mu.Unlock()
	defer m.stopMockMetricsSampler()

	if mock.IsMockEnabled() {
		m.startMockMetricsSampler(ctx)
	}

	// Initialize and start discovery service if enabled
	if mock.IsMockEnabled() {
		log.Info().Msg("Mock mode enabled - skipping discovery service")
		m.discoveryService = nil
	} else if m.config.DiscoveryEnabled {
		discoverySubnet := m.config.DiscoverySubnet
		if discoverySubnet == "" {
			discoverySubnet = "auto"
		}
		cfgProvider := func() config.DiscoveryConfig {
			m.mu.RLock()
			defer m.mu.RUnlock()
			if m.config == nil {
				return config.DefaultDiscoveryConfig()
			}
			cfg := config.CloneDiscoveryConfig(m.config.Discovery)
			// Auto-populate IPBlocklist with configured Proxmox host IPs to avoid
			// probing hosts we already know about (reduces PBS auth failure log spam)
			cfg.IPBlocklist = m.getConfiguredHostIPs()
			return cfg
		}
		m.discoveryService = discovery.NewService(wsHub, 5*time.Minute, discoverySubnet, cfgProvider)
		if m.discoveryService != nil {
			m.discoveryService.Start(ctx)
			log.Info().Msg("Discovery service initialized and started")
		} else {
			log.Error().Msg("Failed to initialize discovery service")
		}
	} else {
		log.Info().Msg("Discovery service disabled by configuration")
		m.discoveryService = nil
	}

	// Set up alert callbacks
	m.alertManager.SetAlertCallback(func(alert *alerts.Alert) {
		m.handleAlertFired(alert)
	})
	// Set up AI analysis callback - this bypasses activation state and other notification suppression
	// so AI can analyze alerts even during pending_review setup phase
	m.alertManager.SetAlertForAICallback(func(alert *alerts.Alert) {
		log.Debug().Str("alertID", alert.ID).Msg("AI alert callback invoked (bypassing notification suppression)")
		if m.alertTriggeredAICallback != nil {
			m.alertTriggeredAICallback(alert)
		}
	})
	m.alertManager.SetResolvedCallback(func(alertID string) {
		m.handleAlertResolved(alertID)
		// Don't broadcast full state here - it causes a cascade with many guests.
		// The frontend will get the updated alerts through the regular broadcast ticker.
	})
	m.alertManager.SetAcknowledgedCallback(func(alert *alerts.Alert, user string) {
		m.handleAlertAcknowledged(alert, user)
	})
	m.alertManager.SetUnacknowledgedCallback(func(alert *alerts.Alert, user string) {
		m.handleAlertUnacknowledged(alert, user)
	})
	m.alertManager.SetEscalateCallback(func(alert *alerts.Alert, level int) {
		log.Info().
			Str("alertID", alert.ID).
			Int("level", level).
			Msg("Alert escalated - sending notifications")

		// Get escalation config
		config := m.alertManager.GetConfig()
		if level <= 0 || level > len(config.Schedule.Escalation.Levels) {
			return
		}

		escalationLevel := config.Schedule.Escalation.Levels[level-1]

		// Send notifications based on escalation level
		switch escalationLevel.Notify {
		case "email":
			// Only send email
			if emailConfig := m.notificationMgr.GetEmailConfig(); emailConfig.Enabled {
				m.notificationMgr.SendAlert(alert)
			}
		case "webhook":
			// Only send webhooks
			for _, webhook := range m.notificationMgr.GetWebhooks() {
				if webhook.Enabled {
					m.notificationMgr.SendAlert(alert)
					break
				}
			}
		case "all":
			// Send all notifications
			m.notificationMgr.SendAlert(alert)
		}

		// Update WebSocket with escalation
		wsHub.BroadcastAlert(alert)
	})

	// Create separate tickers for polling and broadcasting using the configured cadence

	workerCount := len(m.pveClients) + len(m.pbsClients) + len(m.pmgClients)
	m.startTaskWorkers(ctx, workerCount)

	pollTicker := time.NewTicker(pollingInterval)
	defer pollTicker.Stop()

	broadcastTicker := time.NewTicker(pollingInterval)
	defer broadcastTicker.Stop()

	keepRealPolling := keepRealPollingInMockMode()

	// Start connection retry mechanism for failed clients
	// This handles cases where network/Proxmox isn't ready on initial startup
	if !mock.IsMockEnabled() || keepRealPolling {
		go m.retryFailedConnections(ctx)
	}

	// Do an immediate poll on start.
	if mock.IsMockEnabled() {
		if keepRealPolling {
			log.Info().Msg("Mock mode enabled - running mock alerts and real metric polling")
			go m.checkMockAlerts()
			go m.poll(ctx, wsHub)
		} else {
			log.Info().Msg("Mock mode enabled - skipping real node polling")
			go m.checkMockAlerts()
		}
	} else {
		go m.poll(ctx, wsHub)
	}

	for {
		select {
		case <-pollTicker.C:
			now := time.Now()
			m.evaluateDockerAgents(now)
			m.evaluateKubernetesAgents(now)
			m.evaluateHostAgents(now)
			m.cleanupRemovedDockerHosts(now)
			m.cleanupRemovedKubernetesClusters(now)
			m.cleanupGuestMetadataCache(now)
			m.cleanupDiagnosticSnapshots(now)
			m.cleanupRRDCache(now)
			m.cleanupTrackingMaps(now)
			m.cleanupMetricsHistory()
			m.cleanupRateTracker(now)
			if mock.IsMockEnabled() {
				// In mock mode, keep synthetic alerts fresh
				go m.checkMockAlerts()
				if keepRealPolling {
					// Keep real metrics flowing while mock UI mode is active.
					go m.poll(ctx, wsHub)
				}
			} else {
				// Poll real infrastructure
				go m.poll(ctx, wsHub)
			}

		case <-broadcastTicker.C:
			// Broadcast current state regardless of polling status
			// Use GetState() instead of m.state.GetSnapshot() to respect mock mode
			state := m.GetState()
			log.Info().
				Int("nodes", len(state.Nodes)).
				Int("vms", len(state.VMs)).
				Int("containers", len(state.Containers)).
				Int("hosts", len(state.Hosts)).
				Int("pbs", len(state.PBSInstances)).
				Int("pbsBackups", len(state.Backups.PBS)).
				Int("physicalDisks", len(state.PhysicalDisks)).
				Msg("Broadcasting state update (ticker)")
			// Convert to frontend format before broadcasting (converts time.Time to int64, etc.)
			frontendState := state.ToFrontend()
			// Update and inject unified resources if resource store is available
			m.updateResourceStore(state)
			frontendState.Resources = m.getResourcesForBroadcast()
			// Use tenant-aware broadcast method
			m.broadcastState(wsHub, frontendState)

		case <-ctx.Done():
			log.Info().Msg("Monitoring loop stopped")
			return
		}
	}
}

// poll fetches data from all configured instances
func (m *Monitor) poll(_ context.Context, wsHub *websocket.Hub) {
	defer recoverFromPanic("poll")

	// Limit concurrent polls to 2 to prevent resource exhaustion
	currentCount := atomic.AddInt32(&m.activePollCount, 1)
	if currentCount > 2 {
		atomic.AddInt32(&m.activePollCount, -1)
		if logging.IsLevelEnabled(zerolog.DebugLevel) {
			log.Debug().Int32("activePolls", currentCount-1).Msg("Too many concurrent polls, skipping")
		}
		return
	}
	defer atomic.AddInt32(&m.activePollCount, -1)

	if logging.IsLevelEnabled(zerolog.DebugLevel) {
		log.Debug().Msg("Starting polling cycle")
	}
	startTime := time.Now()
	now := startTime

	plannedTasks := m.buildScheduledTasks(now)
	for _, task := range plannedTasks {
		m.taskQueue.Upsert(task)
	}
	m.updateQueueDepthMetric()

	// Update performance metrics
	m.state.Performance.LastPollDuration = time.Since(startTime).Seconds()
	m.state.Stats.PollingCycles++
	m.state.Stats.Uptime = int64(time.Since(m.startTime).Seconds())
	if wsHub != nil {
		m.state.Stats.WebSocketClients = wsHub.GetClientCount()
	} else {
		m.state.Stats.WebSocketClients = 0
	}

	// Sync alert state so broadcasts include the latest acknowledgement data
	m.syncAlertsToState()

	// Increment poll counter
	m.mu.Lock()
	m.pollCounter++
	m.mu.Unlock()

	if logging.IsLevelEnabled(zerolog.DebugLevel) {
		log.Debug().Dur("duration", time.Since(startTime)).Msg("Polling cycle completed")
	}

	// Broadcasting is now handled by the timer in Start()
}

func (m *Monitor) startTaskWorkers(ctx context.Context, workers int) {
	if m.taskQueue == nil {
		return
	}
	if workers < 1 {
		workers = 1
	}
	if workers > 10 {
		workers = 10
	}
	for i := 0; i < workers; i++ {
		go m.taskWorker(ctx, i)
	}
}

func (m *Monitor) taskWorker(ctx context.Context, id int) {
	defer recoverFromPanic(fmt.Sprintf("taskWorker-%d", id))

	if logging.IsLevelEnabled(zerolog.DebugLevel) {
		log.Debug().Int("worker", id).Msg("Task worker started")
	}
	for {
		task, ok := m.taskQueue.WaitNext(ctx)
		if !ok {
			if logging.IsLevelEnabled(zerolog.DebugLevel) {
				log.Debug().Int("worker", id).Msg("Task worker stopping")
			}
			return
		}

		m.executeScheduledTask(ctx, task)

		m.rescheduleTask(task)
		m.updateQueueDepthMetric()
	}
}

func derivePollTimeout(cfg *config.Config) time.Duration {
	timeout := defaultTaskTimeout
	if cfg != nil && cfg.ConnectionTimeout > 0 {
		timeout = cfg.ConnectionTimeout * 2
	}
	if timeout < minTaskTimeout {
		timeout = minTaskTimeout
	}
	// Use configurable max timeout from config (set via MAX_POLL_TIMEOUT env var)
	// Falls back to hardcoded maxTaskTimeout if config is nil or MaxPollTimeout not set
	maxTimeout := maxTaskTimeout
	if cfg != nil && cfg.MaxPollTimeout > 0 {
		maxTimeout = cfg.MaxPollTimeout
	}
	if timeout > maxTimeout {
		timeout = maxTimeout
	}
	return timeout
}

func (m *Monitor) taskExecutionTimeout(_ InstanceType) time.Duration {
	if m == nil {
		return defaultTaskTimeout
	}
	timeout := m.pollTimeout
	if timeout <= 0 {
		timeout = defaultTaskTimeout
	}
	return timeout
}

func (m *Monitor) executeScheduledTask(ctx context.Context, task ScheduledTask) {
	if !m.allowExecution(task) {
		if logging.IsLevelEnabled(zerolog.DebugLevel) {
			log.Debug().
				Str("instance", task.InstanceName).
				Str("type", string(task.InstanceType)).
				Msg("Task blocked by circuit breaker")
		}
		return
	}

	if m.pollMetrics != nil {
		wait := time.Duration(0)
		if !task.NextRun.IsZero() {
			wait = time.Since(task.NextRun)
			if wait < 0 {
				wait = 0
			}
		}
		instanceType := string(task.InstanceType)
		if strings.TrimSpace(instanceType) == "" {
			instanceType = "unknown"
		}
		m.pollMetrics.RecordQueueWait(instanceType, wait)
	}

	executor := m.getExecutor()
	if executor == nil {
		log.Error().
			Str("instance", task.InstanceName).
			Str("type", string(task.InstanceType)).
			Msg("No poll executor configured; skipping task")
		return
	}

	pollTask := PollTask{
		InstanceName: task.InstanceName,
		InstanceType: string(task.InstanceType),
	}

	switch task.InstanceType {
	case InstanceTypePVE:
		client, ok := m.pveClients[task.InstanceName]
		if !ok || client == nil {
			log.Warn().Str("instance", task.InstanceName).Msg("PVE client missing for scheduled task")
			return
		}
		pollTask.PVEClient = client
	case InstanceTypePBS:
		client, ok := m.pbsClients[task.InstanceName]
		if !ok || client == nil {
			log.Warn().Str("instance", task.InstanceName).Msg("PBS client missing for scheduled task")
			return
		}
		pollTask.PBSClient = client
	case InstanceTypePMG:
		client, ok := m.pmgClients[task.InstanceName]
		if !ok || client == nil {
			log.Warn().Str("instance", task.InstanceName).Msg("PMG client missing for scheduled task")
			return
		}
		pollTask.PMGClient = client
	default:
		log.Debug().
			Str("instance", task.InstanceName).
			Str("type", string(task.InstanceType)).
			Msg("Skipping unsupported task type")
		return
	}

	taskCtx := ctx
	var cancel context.CancelFunc
	timeout := m.taskExecutionTimeout(task.InstanceType)
	if timeout > 0 {
		taskCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	executor.Execute(taskCtx, pollTask)

	if timeout > 0 && stderrors.Is(taskCtx.Err(), context.DeadlineExceeded) {
		log.Warn().
			Str("instance", task.InstanceName).
			Str("type", string(task.InstanceType)).
			Dur("timeout", timeout).
			Msg("Polling task timed out; rescheduling with fresh worker")
	}
}

func (m *Monitor) rescheduleTask(task ScheduledTask) {
	if m.taskQueue == nil {
		return
	}

	key := schedulerKey(task.InstanceType, task.InstanceName)
	m.mu.Lock()
	outcome, hasOutcome := m.lastOutcome[key]
	failureCount := m.failureCounts[key]
	m.mu.Unlock()

	if hasOutcome && !outcome.success {
		if !outcome.transient || failureCount >= m.maxRetryAttempts {
			m.sendToDeadLetter(task, outcome.err)
			return
		}
		delay := m.backoffCfg.nextDelay(failureCount-1, m.randomFloat())
		if delay <= 0 {
			delay = 5 * time.Second
		}
		if m.config != nil && m.config.AdaptivePollingEnabled && m.config.AdaptivePollingMaxInterval > 0 && m.config.AdaptivePollingMaxInterval <= 15*time.Second {
			maxDelay := 4 * time.Second
			if delay > maxDelay {
				delay = maxDelay
			}
		}
		next := task
		next.Interval = delay
		next.NextRun = time.Now().Add(delay)
		m.taskQueue.Upsert(next)
		return
	}

	if m.scheduler == nil {
		baseInterval := m.baseIntervalForInstanceType(task.InstanceType)
		nextInterval := task.Interval
		if nextInterval <= 0 {
			nextInterval = baseInterval
		}
		if nextInterval <= 0 {
			nextInterval = DefaultSchedulerConfig().BaseInterval
		}
		next := task
		next.NextRun = time.Now().Add(nextInterval)
		next.Interval = nextInterval
		m.taskQueue.Upsert(next)
		return
	}

	desc := InstanceDescriptor{
		Name:          task.InstanceName,
		Type:          task.InstanceType,
		LastInterval:  task.Interval,
		LastScheduled: task.NextRun,
	}
	if m.stalenessTracker != nil {
		if snap, ok := m.stalenessTracker.snapshot(task.InstanceType, task.InstanceName); ok {
			desc.LastSuccess = snap.LastSuccess
			desc.LastFailure = snap.LastError
			if snap.ChangeHash != "" {
				desc.Metadata = map[string]any{"changeHash": snap.ChangeHash}
			}
		}
	}

	tasks := m.scheduler.BuildPlan(time.Now(), []InstanceDescriptor{desc}, m.taskQueue.Size())
	if len(tasks) == 0 {
		next := task
		nextInterval := task.Interval
		if nextInterval <= 0 && m.config != nil {
			nextInterval = m.config.AdaptivePollingBaseInterval
		}
		if nextInterval <= 0 {
			nextInterval = DefaultSchedulerConfig().BaseInterval
		}
		next.Interval = nextInterval
		next.NextRun = time.Now().Add(nextInterval)
		m.taskQueue.Upsert(next)
		return
	}
	for _, next := range tasks {
		m.taskQueue.Upsert(next)
	}
}

func (m *Monitor) sendToDeadLetter(task ScheduledTask, err error) {
	if m.deadLetterQueue == nil {
		log.Error().
			Str("instance", task.InstanceName).
			Str("type", string(task.InstanceType)).
			Err(err).
			Msg("Dead-letter queue unavailable; dropping task")
		return
	}

	log.Error().
		Str("instance", task.InstanceName).
		Str("type", string(task.InstanceType)).
		Err(err).
		Msg("Routing task to dead-letter queue after repeated failures")

	next := task
	next.Interval = 30 * time.Minute
	next.NextRun = time.Now().Add(next.Interval)
	m.deadLetterQueue.Upsert(next)
	m.updateDeadLetterMetrics()

	key := schedulerKey(task.InstanceType, task.InstanceName)
	now := time.Now()

	m.mu.Lock()
	if m.dlqInsightMap == nil {
		m.dlqInsightMap = make(map[string]*dlqInsight)
	}
	info, ok := m.dlqInsightMap[key]
	if !ok {
		info = &dlqInsight{}
		m.dlqInsightMap[key] = info
	}
	if info.FirstAttempt.IsZero() {
		info.FirstAttempt = now
	}
	info.LastAttempt = now
	info.RetryCount++
	info.NextRetry = next.NextRun
	if err != nil {
		info.Reason = classifyDLQReason(err)
	}
	m.mu.Unlock()
}

func classifyDLQReason(err error) string {
	if err == nil {
		return ""
	}
	if errors.IsRetryableError(err) {
		return "max_retry_attempts"
	}
	return "permanent_failure"
}

func (m *Monitor) updateDeadLetterMetrics() {
	if m.pollMetrics == nil || m.deadLetterQueue == nil {
		return
	}

	size := m.deadLetterQueue.Size()
	if size <= 0 {
		m.pollMetrics.UpdateDeadLetterCounts(nil)
		return
	}

	tasks := m.deadLetterQueue.PeekAll(size)
	m.pollMetrics.UpdateDeadLetterCounts(tasks)
}

func (m *Monitor) updateBreakerMetric(instanceType InstanceType, instance string, breaker *circuitBreaker) {
	if m.pollMetrics == nil || breaker == nil {
		return
	}

	state, failures, retryAt, _, _ := breaker.stateDetails()
	m.pollMetrics.SetBreakerState(string(instanceType), instance, state, failures, retryAt)
}

func (m *Monitor) randomFloat() float64 {
	if m.rng == nil {
		m.rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	return m.rng.Float64()
}

func (m *Monitor) updateQueueDepthMetric() {
	if m.pollMetrics == nil || m.taskQueue == nil {
		return
	}
	snapshot := m.taskQueue.Snapshot()
	m.pollMetrics.SetQueueDepth(snapshot.Depth)
	m.pollMetrics.UpdateQueueSnapshot(snapshot)
}

func (m *Monitor) allowExecution(task ScheduledTask) bool {
	if m.circuitBreakers == nil {
		return true
	}
	key := schedulerKey(task.InstanceType, task.InstanceName)
	breaker := m.ensureBreaker(key)
	allowed := breaker.allow(time.Now())
	m.updateBreakerMetric(task.InstanceType, task.InstanceName, breaker)
	return allowed
}

func (m *Monitor) ensureBreaker(key string) *circuitBreaker {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.circuitBreakers == nil {
		m.circuitBreakers = make(map[string]*circuitBreaker)
	}
	if breaker, ok := m.circuitBreakers[key]; ok {
		return breaker
	}
	baseRetry := m.breakerBaseRetry
	if baseRetry <= 0 {
		baseRetry = 5 * time.Second
	}
	maxDelay := m.breakerMaxDelay
	if maxDelay <= 0 {
		maxDelay = 5 * time.Minute
	}
	halfOpen := m.breakerHalfOpenWindow
	if halfOpen <= 0 {
		halfOpen = 30 * time.Second
	}
	breaker := newCircuitBreaker(3, baseRetry, maxDelay, halfOpen)
	m.circuitBreakers[key] = breaker
	return breaker
}

func (m *Monitor) recordTaskResult(instanceType InstanceType, instance string, pollErr error) {
	if m == nil {
		return
	}

	key := schedulerKey(instanceType, instance)
	now := time.Now()

	breaker := m.ensureBreaker(key)

	m.mu.Lock()
	status, ok := m.pollStatusMap[key]
	if !ok {
		status = &pollStatus{}
		m.pollStatusMap[key] = status
	}

	if pollErr == nil {
		if m.failureCounts != nil {
			m.failureCounts[key] = 0
		}
		if m.lastOutcome != nil {
			m.lastOutcome[key] = taskOutcome{
				success:    true,
				transient:  true,
				err:        nil,
				recordedAt: now,
			}
		}
		status.LastSuccess = now
		status.ConsecutiveFailures = 0
		status.FirstFailureAt = time.Time{}
		m.mu.Unlock()
		if breaker != nil {
			breaker.recordSuccess()
			m.updateBreakerMetric(instanceType, instance, breaker)
		}
		return
	}

	transient := isTransientError(pollErr)
	category := "permanent"
	if transient {
		category = "transient"
	}
	if m.failureCounts != nil {
		m.failureCounts[key] = m.failureCounts[key] + 1
	}
	if m.lastOutcome != nil {
		m.lastOutcome[key] = taskOutcome{
			success:    false,
			transient:  transient,
			err:        pollErr,
			recordedAt: now,
		}
	}
	status.LastErrorAt = now
	status.LastErrorMessage = pollErr.Error()
	status.LastErrorCategory = category
	status.ConsecutiveFailures++
	if status.ConsecutiveFailures == 1 {
		status.FirstFailureAt = now
	}
	m.mu.Unlock()
	if breaker != nil {
		breaker.recordFailure(now)
		m.updateBreakerMetric(instanceType, instance, breaker)
	}
}

// SchedulerHealthResponse contains complete scheduler health data for API exposure.
type SchedulerHealthResponse struct {
	UpdatedAt  time.Time           `json:"updatedAt"`
	Enabled    bool                `json:"enabled"`
	Queue      QueueSnapshot       `json:"queue"`
	DeadLetter DeadLetterSnapshot  `json:"deadLetter"`
	Breakers   []BreakerSnapshot   `json:"breakers,omitempty"`
	Staleness  []StalenessSnapshot `json:"staleness,omitempty"`
	Instances  []InstanceHealth    `json:"instances"`
}

// DeadLetterSnapshot contains dead-letter queue data.
type DeadLetterSnapshot struct {
	Count int              `json:"count"`
	Tasks []DeadLetterTask `json:"tasks"`
}

// SchedulerHealth returns a complete snapshot of scheduler health for API exposure.
func (m *Monitor) SchedulerHealth() SchedulerHealthResponse {
	response := SchedulerHealthResponse{
		UpdatedAt: time.Now(),
		Enabled:   m.config != nil && m.config.AdaptivePollingEnabled,
	}

	// Queue snapshot
	if m.taskQueue != nil {
		response.Queue = m.taskQueue.Snapshot()
		if m.pollMetrics != nil {
			m.pollMetrics.UpdateQueueSnapshot(response.Queue)
		}
	}

	// Dead-letter queue snapshot
	if m.deadLetterQueue != nil {
		deadLetterTasks := m.deadLetterQueue.PeekAll(25) // limit to top 25
		m.mu.RLock()
		for i := range deadLetterTasks {
			key := schedulerKey(InstanceType(deadLetterTasks[i].Type), deadLetterTasks[i].Instance)
			if outcome, ok := m.lastOutcome[key]; ok && outcome.err != nil {
				deadLetterTasks[i].LastError = outcome.err.Error()
			}
			if count, ok := m.failureCounts[key]; ok {
				deadLetterTasks[i].Failures = count
			}
		}
		m.mu.RUnlock()
		response.DeadLetter = DeadLetterSnapshot{
			Count: m.deadLetterQueue.Size(),
			Tasks: deadLetterTasks,
		}
		m.updateDeadLetterMetrics()
	}

	// Circuit breaker snapshots
	m.mu.RLock()
	breakerSnapshots := make([]BreakerSnapshot, 0, len(m.circuitBreakers))
	for key, breaker := range m.circuitBreakers {
		state, failures, retryAt := breaker.State()
		// Only include breakers that are not in default closed state with 0 failures
		if state != "closed" || failures > 0 {
			// Parse instance type and name from key
			parts := strings.SplitN(key, "::", 2)
			instanceType, instanceName := "unknown", key
			if len(parts) == 2 {
				instanceType, instanceName = parts[0], parts[1]
			}
			breakerSnapshots = append(breakerSnapshots, BreakerSnapshot{
				Instance: instanceName,
				Type:     instanceType,
				State:    state,
				Failures: failures,
				RetryAt:  retryAt,
			})
		}
	}
	m.mu.RUnlock()
	response.Breakers = breakerSnapshots

	// Staleness snapshots
	if m.stalenessTracker != nil {
		response.Staleness = m.stalenessTracker.Snapshot()
	}

	instanceInfos := make(map[string]*instanceInfo)
	pollStatuses := make(map[string]pollStatus)
	dlqInsights := make(map[string]dlqInsight)
	breakerRefs := make(map[string]*circuitBreaker)

	m.mu.RLock()
	for k, v := range m.instanceInfoCache {
		if v == nil {
			continue
		}
		copyVal := *v
		instanceInfos[k] = &copyVal
	}
	for k, v := range m.pollStatusMap {
		if v == nil {
			continue
		}
		pollStatuses[k] = *v
	}
	for k, v := range m.dlqInsightMap {
		if v == nil {
			continue
		}
		dlqInsights[k] = *v
	}
	for k, v := range m.circuitBreakers {
		if v != nil {
			breakerRefs[k] = v
		}
	}
	m.mu.RUnlock()
	for key, breaker := range breakerRefs {
		instanceType := InstanceType("unknown")
		instanceName := key
		if parts := strings.SplitN(key, "::", 2); len(parts) == 2 {
			if parts[0] != "" {
				instanceType = InstanceType(parts[0])
			}
			if parts[1] != "" {
				instanceName = parts[1]
			}
		}
		m.updateBreakerMetric(instanceType, instanceName, breaker)
	}

	keySet := make(map[string]struct{})
	for k := range instanceInfos {
		if k != "" {
			keySet[k] = struct{}{}
		}
	}
	for k := range pollStatuses {
		if k != "" {
			keySet[k] = struct{}{}
		}
	}
	for k := range dlqInsights {
		if k != "" {
			keySet[k] = struct{}{}
		}
	}
	for k := range breakerRefs {
		if k != "" {
			keySet[k] = struct{}{}
		}
	}
	for _, task := range response.DeadLetter.Tasks {
		if task.Instance == "" {
			continue
		}
		keySet[schedulerKey(InstanceType(task.Type), task.Instance)] = struct{}{}
	}
	for _, snap := range response.Staleness {
		if snap.Instance == "" {
			continue
		}
		keySet[schedulerKey(InstanceType(snap.Type), snap.Instance)] = struct{}{}
	}

	if len(keySet) > 0 {
		keys := make([]string, 0, len(keySet))
		for k := range keySet {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		instances := make([]InstanceHealth, 0, len(keys))
		for _, key := range keys {
			instType := "unknown"
			instName := key
			if parts := strings.SplitN(key, "::", 2); len(parts) == 2 {
				if parts[0] != "" {
					instType = parts[0]
				}
				if parts[1] != "" {
					instName = parts[1]
				}
			}
			instType = strings.TrimSpace(instType)
			instName = strings.TrimSpace(instName)

			info := instanceInfos[key]
			display := instName
			connection := ""
			if info != nil {
				if instType == "unknown" || instType == "" {
					if info.Type != "" {
						instType = string(info.Type)
					}
				}
				if strings.Contains(info.Key, "::") {
					if parts := strings.SplitN(info.Key, "::", 2); len(parts) == 2 {
						if instName == key {
							instName = parts[1]
						}
						if (instType == "" || instType == "unknown") && parts[0] != "" {
							instType = parts[0]
						}
					}
				}
				if info.DisplayName != "" {
					display = info.DisplayName
				}
				if info.Connection != "" {
					connection = info.Connection
				}
			}
			display = strings.TrimSpace(display)
			connection = strings.TrimSpace(connection)
			if display == "" {
				display = instName
			}
			if display == "" {
				display = connection
			}
			if instType == "" {
				instType = "unknown"
			}
			if instName == "" {
				instName = key
			}

			status, hasStatus := pollStatuses[key]
			instanceStatus := InstancePollStatus{}
			if hasStatus {
				instanceStatus.ConsecutiveFailures = status.ConsecutiveFailures
				instanceStatus.LastSuccess = timePtr(status.LastSuccess)
				if !status.FirstFailureAt.IsZero() {
					instanceStatus.FirstFailureAt = timePtr(status.FirstFailureAt)
				}
				if !status.LastErrorAt.IsZero() && status.LastErrorMessage != "" {
					instanceStatus.LastError = &ErrorDetail{
						At:       status.LastErrorAt,
						Message:  status.LastErrorMessage,
						Category: status.LastErrorCategory,
					}
				}
			}

			breakerInfo := InstanceBreaker{
				State:        "closed",
				FailureCount: 0,
			}
			if br, ok := breakerRefs[key]; ok && br != nil {
				state, failures, retryAt, since, lastTransition := br.stateDetails()
				if state != "" {
					breakerInfo.State = state
				}
				breakerInfo.FailureCount = failures
				breakerInfo.RetryAt = timePtr(retryAt)
				breakerInfo.Since = timePtr(since)
				breakerInfo.LastTransition = timePtr(lastTransition)
			}

			dlqInfo := InstanceDLQ{Present: false}
			if dlq, ok := dlqInsights[key]; ok {
				dlqInfo.Present = true
				dlqInfo.Reason = dlq.Reason
				dlqInfo.FirstAttempt = timePtr(dlq.FirstAttempt)
				dlqInfo.LastAttempt = timePtr(dlq.LastAttempt)
				dlqInfo.RetryCount = dlq.RetryCount
				dlqInfo.NextRetry = timePtr(dlq.NextRetry)
			}

			// Collect any warnings for this instance
			var warnings []string
			if instType == "pve" {
				if warning, ok := m.backupPermissionWarnings[instName]; ok {
					warnings = append(warnings, warning)
				}
			}

			instances = append(instances, InstanceHealth{
				Key:         key,
				Type:        instType,
				DisplayName: display,
				Instance:    instName,
				Connection:  connection,
				PollStatus:  instanceStatus,
				Breaker:     breakerInfo,
				DeadLetter:  dlqInfo,
				Warnings:    warnings,
			})
		}

		response.Instances = instances
	} else {
		response.Instances = []InstanceHealth{}
	}

	return response
}

func isTransientError(err error) bool {
	if err == nil {
		return true
	}
	if errors.IsRetryableError(err) {
		return true
	}
	if stderrors.Is(err, context.Canceled) || stderrors.Is(err, context.DeadlineExceeded) {
		return true
	}
	return false
}

func (m *Monitor) GetState() models.StateSnapshot {
	// Check if mock mode is enabled
	if mock.IsMockEnabled() {
		state := mock.GetMockState()
		if state.ActiveAlerts == nil {
			// Populate snapshot lazily if the cache hasn't been filled yet.
			mock.UpdateAlertSnapshots(m.alertManager.GetActiveAlerts(), m.alertManager.GetRecentlyResolved())
			state = mock.GetMockState()
		}
		return state
	}
	return m.state.GetSnapshot()
}

// GetLiveStateSnapshot returns the underlying monitor state snapshot without
// applying global mock mode overrides.
//
// This is useful for agent management endpoints that need to reflect actual
// registrations even when mock mode is enabled for the UI/demo experience.
func (m *Monitor) GetLiveStateSnapshot() models.StateSnapshot {
	if m == nil || m.state == nil {
		return models.StateSnapshot{}
	}
	return m.state.GetSnapshot()
}

// SetOrgID sets the organization ID for this monitor instance.
// This is used for tenant isolation in multi-tenant deployments.
func (m *Monitor) SetOrgID(orgID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.orgID = orgID
}

// GetOrgID returns the organization ID for this monitor instance.
// Returns empty string for default/legacy monitors.
func (m *Monitor) GetOrgID() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.orgID
}

// broadcastState broadcasts state to WebSocket clients.
// For tenant monitors, it broadcasts only to clients of that tenant.
// For default monitors, it broadcasts to all clients.
func (m *Monitor) broadcastState(hub *websocket.Hub, frontendState interface{}) {
	if hub == nil {
		return
	}

	orgID := m.GetOrgID()
	if orgID != "" && orgID != "default" {
		// Tenant-specific broadcast
		hub.BroadcastStateToTenant(orgID, frontendState)
	} else {
		// Legacy broadcast to all clients
		hub.BroadcastState(frontendState)
	}
}

// SetMockMode switches between mock data and real infrastructure data at runtime.
func (m *Monitor) SetMockMode(enable bool) {
	current := mock.IsMockEnabled()
	if current == enable {
		log.Info().Bool("mockMode", enable).Msg("Mock mode already in desired state")
		return
	}

	if enable {
		m.stopMockMetricsSampler()
		mock.SetEnabled(true)
		m.alertManager.ClearActiveAlerts()
		m.mu.Lock()
		m.resetStateLocked()
		m.metricsHistory.Reset()
		m.mu.Unlock()
		m.StopDiscoveryService()
		m.mu.RLock()
		ctx := m.runtimeCtx
		m.mu.RUnlock()
		if ctx != nil {
			m.startMockMetricsSampler(ctx)
		}
		log.Info().Msg("Switched monitor to mock mode")
	} else {
		m.stopMockMetricsSampler()
		mock.SetEnabled(false)
		m.alertManager.ClearActiveAlerts()
		m.mu.Lock()
		m.resetStateLocked()
		m.metricsHistory.Reset()
		m.mu.Unlock()
		log.Info().Msg("Switched monitor to real data mode")
	}

	m.mu.RLock()
	ctx := m.runtimeCtx
	hub := m.wsHub
	m.mu.RUnlock()

	if hub != nil {
		state := m.GetState()
		frontendState := state.ToFrontend()
		m.updateResourceStore(state)
		frontendState.Resources = m.getResourcesForBroadcast()
		// Use tenant-aware broadcast method
		m.broadcastState(hub, frontendState)
	}

	if enable && ctx != nil && keepRealPollingInMockMode() {
		// Keep real metrics flowing while mock mode is enabled.
		go m.poll(ctx, hub)
	}

	if !enable && ctx != nil {
		// Kick off an immediate poll to repopulate state with live data.
		go m.poll(ctx, hub)
		if hub != nil && m.config.DiscoveryEnabled {
			go m.StartDiscoveryService(ctx, hub, m.config.DiscoverySubnet)
		}
	}
}

func (m *Monitor) resetStateLocked() {
	m.state = models.NewState()
	m.state.Stats = models.Stats{
		StartTime: m.startTime,
		Version:   "2.0.0-go",
	}
}

// GetStartTime returns the monitor start time
func (m *Monitor) GetStartTime() time.Time {
	return m.startTime
}

// GetDiscoveryService returns the discovery service
func (m *Monitor) GetDiscoveryService() *discovery.Service {
	return m.discoveryService
}

// StartDiscoveryService starts the discovery service if not already running
func (m *Monitor) StartDiscoveryService(ctx context.Context, wsHub *websocket.Hub, subnet string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.discoveryService != nil {
		log.Debug().Msg("Discovery service already running")
		return
	}

	if subnet == "" {
		subnet = "auto"
	}

	cfgProvider := func() config.DiscoveryConfig {
		m.mu.RLock()
		defer m.mu.RUnlock()
		if m.config == nil {
			return config.DefaultDiscoveryConfig()
		}
		return config.CloneDiscoveryConfig(m.config.Discovery)
	}

	m.discoveryService = discovery.NewService(wsHub, 5*time.Minute, subnet, cfgProvider)
	if m.discoveryService != nil {
		m.discoveryService.Start(ctx)
		log.Info().Str("subnet", subnet).Msg("Discovery service started")
	} else {
		log.Error().Msg("Failed to create discovery service")
	}
}

// StopDiscoveryService stops the discovery service if running
func (m *Monitor) StopDiscoveryService() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.discoveryService != nil {
		m.discoveryService.Stop()
		m.discoveryService = nil
		log.Info().Msg("Discovery service stopped")
	}
}

// EnableTemperatureMonitoring enables temperature data collection
func (m *Monitor) EnableTemperatureMonitoring() {
	// Temperature collection is always enabled when tempCollector is initialized
	// This method exists for interface compatibility
	log.Info().Msg("Temperature monitoring enabled")
}

// DisableTemperatureMonitoring disables temperature data collection
func (m *Monitor) DisableTemperatureMonitoring() {
	// Temperature collection is always enabled when tempCollector is initialized
	// This method exists for interface compatibility
	log.Info().Msg("Temperature monitoring disabled")
}

// SetResourceStore sets the resource store for polling optimization.
// When set, the monitor will check if it should reduce polling frequency
// for nodes that have host agents providing data.
func (m *Monitor) SetResourceStore(store ResourceStoreInterface) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.resourceStore = store
	log.Info().Msg("Resource store set for polling optimization")
}

// GetNotificationManager returns the notification manager
func (m *Monitor) GetNotificationManager() *notifications.NotificationManager {
	return m.notificationMgr
}

// GetConfigPersistence returns the config persistence manager
func (m *Monitor) GetConfigPersistence() *config.ConfigPersistence {
	return m.configPersist
}

// GetMetricsStore returns the persistent metrics store
func (m *Monitor) GetMetricsStore() *metrics.Store {
	return m.metricsStore
}

// GetMetricsHistory returns the in-memory metrics history for trend analysis
// This is used by the AI context builder to compute trends and predictions
func (m *Monitor) GetMetricsHistory() *MetricsHistory {
	return m.metricsHistory
}

// shouldSkipNodeMetrics returns true if we should skip detailed metric polling
// for the given node because a host agent is providing richer data.
// This helps reduce API load when agents are active.
func (m *Monitor) shouldSkipNodeMetrics(nodeName string) bool {
	m.mu.RLock()
	store := m.resourceStore
	m.mu.RUnlock()

	if store == nil {
		return false
	}

	should := store.ShouldSkipAPIPolling(nodeName)
	if should {
		log.Debug().
			Str("node", nodeName).
			Msg("Skipping detailed node metrics - host agent provides data")
	}
	return should
}

// updateResourceStore populates the resource store with data from the current state.
// This should be called before broadcasting to ensure fresh data.
func (m *Monitor) updateResourceStore(state models.StateSnapshot) {
	m.mu.RLock()
	store := m.resourceStore
	m.mu.RUnlock()

	if store == nil {
		log.Debug().Msg("[Resources] No resource store configured, skipping population")
		return
	}

	log.Debug().
		Int("nodes", len(state.Nodes)).
		Int("vms", len(state.VMs)).
		Int("containers", len(state.Containers)).
		Int("hosts", len(state.Hosts)).
		Int("dockerHosts", len(state.DockerHosts)).
		Msg("[Resources] Populating resource store from state snapshot")

	store.PopulateFromSnapshot(state)
}

// getResourcesForBroadcast retrieves all resources from the store and converts them to frontend format.
// Returns nil if no resource store is configured.
func (m *Monitor) getResourcesForBroadcast() []models.ResourceFrontend {
	m.mu.RLock()
	store := m.resourceStore
	m.mu.RUnlock()

	if store == nil {
		log.Debug().Msg("[Resources] No store for broadcast")
		return nil
	}

	allResources := store.GetAll()
	log.Debug().Int("count", len(allResources)).Msg("[Resources] Got resources for broadcast")
	if len(allResources) == 0 {
		return nil
	}

	result := make([]models.ResourceFrontend, len(allResources))
	for i, r := range allResources {
		input := models.ResourceConvertInput{
			ID:           r.ID,
			Type:         string(r.Type),
			Name:         r.Name,
			DisplayName:  r.DisplayName,
			PlatformID:   r.PlatformID,
			PlatformType: string(r.PlatformType),
			SourceType:   string(r.SourceType),
			ParentID:     r.ParentID,
			ClusterID:    r.ClusterID,
			Status:       string(r.Status),
			Temperature:  r.Temperature,
			Uptime:       r.Uptime,
			Tags:         r.Tags,
			Labels:       r.Labels,
			LastSeenUnix: r.LastSeen.UnixMilli(),
		}

		// Convert metrics
		if r.CPU != nil {
			input.CPU = &models.ResourceMetricInput{
				Current: r.CPU.Current,
				Total:   r.CPU.Total,
				Used:    r.CPU.Used,
				Free:    r.CPU.Free,
			}
		}
		if r.Memory != nil {
			input.Memory = &models.ResourceMetricInput{
				Current: r.Memory.Current,
				Total:   r.Memory.Total,
				Used:    r.Memory.Used,
				Free:    r.Memory.Free,
			}
		}
		if r.Disk != nil {
			input.Disk = &models.ResourceMetricInput{
				Current: r.Disk.Current,
				Total:   r.Disk.Total,
				Used:    r.Disk.Used,
				Free:    r.Disk.Free,
			}
		}
		if r.Network != nil {
			input.HasNetwork = true
			input.NetworkRX = r.Network.RXBytes
			input.NetworkTX = r.Network.TXBytes
		}

		// Convert alerts
		if len(r.Alerts) > 0 {
			input.Alerts = make([]models.ResourceAlertInput, len(r.Alerts))
			for j, a := range r.Alerts {
				input.Alerts[j] = models.ResourceAlertInput{
					ID:            a.ID,
					Type:          a.Type,
					Level:         a.Level,
					Message:       a.Message,
					Value:         a.Value,
					Threshold:     a.Threshold,
					StartTimeUnix: a.StartTime.UnixMilli(),
				}
			}
		}

		// Convert identity
		if r.Identity != nil {
			input.Identity = &models.ResourceIdentityInput{
				Hostname:  r.Identity.Hostname,
				MachineID: r.Identity.MachineID,
				IPs:       r.Identity.IPs,
			}
		}

		// Pass platform data directly as json.RawMessage
		input.PlatformData = r.PlatformData

		result[i] = models.ConvertResourceToFrontend(input)
	}

	return result
}

// pollStorageBackupsWithNodes polls backups using a provided nodes list to avoid duplicate GetNodes calls
// Stop gracefully stops the monitor
func (m *Monitor) Stop() {
	log.Info().Msg("Stopping monitor")

	// Stop the alert manager to save history
	if m.alertManager != nil {
		m.alertManager.Stop()
	}

	// Stop notification manager
	if m.notificationMgr != nil {
		m.notificationMgr.Stop()
	}

	// Close persistent metrics store (flushes buffered data)
	if m.metricsStore != nil {
		if err := m.metricsStore.Close(); err != nil {
			log.Error().Err(err).Msg("Failed to close metrics store")
		} else {
			log.Info().Msg("Metrics store closed successfully")
		}
	}

	log.Info().Msg("Monitor stopped")
}

// recordAuthFailure records an authentication failure for a node
