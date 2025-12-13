package monitoring

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"math"
	"math/rand"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/discovery"
	"github.com/rcourtman/pulse-go-rewrite/internal/errors"
	"github.com/rcourtman/pulse-go-rewrite/internal/logging"
	"github.com/rcourtman/pulse-go-rewrite/internal/metrics"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	"github.com/rcourtman/pulse-go-rewrite/internal/resources"
	"github.com/rcourtman/pulse-go-rewrite/internal/system"
	"github.com/rcourtman/pulse-go-rewrite/internal/tempproxy"
	"github.com/rcourtman/pulse-go-rewrite/internal/types"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	"github.com/rcourtman/pulse-go-rewrite/pkg/fsfilters"
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
	GetAll() []resources.Resource
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

func lookupClusterEndpointLabel(instance *config.PVEInstance, nodeName string) string {
	if instance == nil {
		return ""
	}

	for _, endpoint := range instance.ClusterEndpoints {
		if !strings.EqualFold(endpoint.NodeName, nodeName) {
			continue
		}

		if host := strings.TrimSpace(endpoint.Host); host != "" {
			if label := normalizeEndpointHost(host); label != "" && !isLikelyIPAddress(label) {
				return label
			}
		}

		if nodeNameLabel := strings.TrimSpace(endpoint.NodeName); nodeNameLabel != "" {
			return nodeNameLabel
		}

		if ip := strings.TrimSpace(endpoint.IP); ip != "" {
			return ip
		}
	}

	return ""
}

func normalizeEndpointHost(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}

	if parsed, err := url.Parse(value); err == nil && parsed.Host != "" {
		host := parsed.Hostname()
		if host != "" {
			return host
		}
		return parsed.Host
	}

	value = strings.TrimPrefix(value, "https://")
	value = strings.TrimPrefix(value, "http://")
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	if idx := strings.Index(value, "/"); idx >= 0 {
		value = strings.TrimSpace(value[:idx])
	}

	if idx := strings.Index(value, ":"); idx >= 0 {
		value = strings.TrimSpace(value[:idx])
	}

	return value
}

func isLikelyIPAddress(value string) bool {
	if value == "" {
		return false
	}

	if ip := net.ParseIP(value); ip != nil {
		return true
	}

	// Handle IPv6 with zone identifier (fe80::1%eth0)
	if i := strings.Index(value, "%"); i > 0 {
		if ip := net.ParseIP(value[:i]); ip != nil {
			return true
		}
	}

	return false
}

func ensureClusterEndpointURL(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}

	lower := strings.ToLower(value)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return value
	}

	if _, _, err := net.SplitHostPort(value); err == nil {
		return "https://" + value
	}

	return "https://" + net.JoinHostPort(value, "8006")
}

func clusterEndpointEffectiveURL(endpoint config.ClusterEndpoint, verifySSL bool, hasFingerprint bool) string {
	// When TLS hostname verification is required (VerifySSL=true and no fingerprint),
	// prefer hostname over IP to ensure certificate CN/SAN validation works correctly.
	// When TLS is not verified (VerifySSL=false) or a fingerprint is provided (which
	// bypasses hostname checks), prefer IP to reduce DNS lookups (refs #620).
	requiresHostnameForTLS := verifySSL && !hasFingerprint

	if requiresHostnameForTLS {
		// Prefer hostname for proper TLS certificate validation
		if endpoint.Host != "" {
			return ensureClusterEndpointURL(endpoint.Host)
		}
		if endpoint.IP != "" {
			return ensureClusterEndpointURL(endpoint.IP)
		}
	} else {
		// Prefer IP address to avoid excessive DNS lookups
		if endpoint.IP != "" {
			return ensureClusterEndpointURL(endpoint.IP)
		}
		if endpoint.Host != "" {
			return ensureClusterEndpointURL(endpoint.Host)
		}
	}
	return ""
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
}

func schedulerKey(instanceType InstanceType, name string) string {
	return string(instanceType) + "::" + name
}

func timePtr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	copy := t
	return &copy
}

// Monitor handles all monitoring operations
type Monitor struct {
	config                     *config.Config
	state                      *models.State
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
	mu                         sync.RWMutex
	startTime                  time.Time
	rateTracker                *RateTracker
	metricsHistory             *MetricsHistory
	metricsStore               *metrics.Store // Persistent SQLite metrics storage
	alertManager               *alerts.Manager
	notificationMgr            *notifications.NotificationManager
	configPersist              *config.ConfigPersistence
	discoveryService           *discovery.Service        // Background discovery service
	activePollCount            int32                     // Number of active polling operations
	pollCounter                int64                     // Counter for polling cycles
	authFailures               map[string]int            // Track consecutive auth failures per node
	lastAuthAttempt            map[string]time.Time      // Track last auth attempt time
	lastClusterCheck           map[string]time.Time      // Track last cluster check for standalone nodes
	lastPhysicalDiskPoll       map[string]time.Time      // Track last physical disk poll time per instance
	lastPVEBackupPoll          map[string]time.Time      // Track last PVE backup poll per instance
	lastPBSBackupPoll          map[string]time.Time      // Track last PBS backup poll per instance
	persistence                *config.ConfigPersistence // Add persistence for saving updated configs
	pbsBackupPollers           map[string]bool           // Track PBS backup polling goroutines per instance
	runtimeCtx                 context.Context           // Context used while monitor is running
	wsHub                      *websocket.Hub            // Hub used for broadcasting state
	diagMu                     sync.RWMutex              // Protects diagnostic snapshot maps
	nodeSnapshots              map[string]NodeMemorySnapshot
	guestSnapshots             map[string]GuestMemorySnapshot
	rrdCacheMu                 sync.RWMutex // Protects RRD memavailable cache
	nodeRRDMemCache            map[string]rrdMemCacheEntry
	removedDockerHosts         map[string]time.Time // Track deliberately removed Docker hosts (ID -> removal time)
	dockerTokenBindings        map[string]string    // Track token ID -> agent ID bindings to enforce uniqueness
	removedKubernetesClusters  map[string]time.Time // Track deliberately removed Kubernetes clusters (ID -> removal time)
	kubernetesTokenBindings    map[string]string    // Track token ID -> agent ID bindings to enforce uniqueness
	hostTokenBindings          map[string]string    // Track token ID -> agent ID bindings to enforce uniqueness
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
	nodeLastOnline           map[string]time.Time   // Track last time each node was seen online (for grace period)
	resourceStore            ResourceStoreInterface // Optional unified resource store for polling optimization
	mockMetricsCancel         context.CancelFunc
	mockMetricsWg             sync.WaitGroup
}

type rrdMemCacheEntry struct {
	available uint64
	used      uint64
	total     uint64
	fetchedAt time.Time
}

// safePercentage calculates percentage safely, returning 0 if divisor is 0
func safePercentage(used, total float64) float64 {
	if total == 0 {
		return 0
	}
	result := used / total * 100
	if math.IsNaN(result) || math.IsInf(result, 0) {
		return 0
	}
	return result
}

// safeFloat ensures a float value is not NaN or Inf
func safeFloat(val float64) float64 {
	if math.IsNaN(val) || math.IsInf(val, 0) {
		return 0
	}
	return val
}

// parseDurationEnv parses a duration from an environment variable, returning defaultVal if not set or invalid
func parseDurationEnv(key string, defaultVal time.Duration) time.Duration {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	parsed, err := time.ParseDuration(val)
	if err != nil {
		log.Warn().
			Str("key", key).
			Str("value", val).
			Err(err).
			Dur("default", defaultVal).
			Msg("Failed to parse duration from environment variable, using default")
		return defaultVal
	}
	return parsed
}

// parseIntEnv parses an integer from an environment variable, returning defaultVal if not set or invalid
func parseIntEnv(key string, defaultVal int) int {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	parsed, err := strconv.Atoi(val)
	if err != nil {
		log.Warn().
			Str("key", key).
			Str("value", val).
			Err(err).
			Int("default", defaultVal).
			Msg("Failed to parse integer from environment variable, using default")
		return defaultVal
	}
	return parsed
}

func clampUint64ToInt64(val uint64) int64 {
	if val > math.MaxInt64 {
		return math.MaxInt64
	}
	return int64(val)
}

func cloneStringFloatMap(src map[string]float64) map[string]float64 {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]float64, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]string, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func convertDockerServices(services []agentsdocker.Service) []models.DockerService {
	if len(services) == 0 {
		return nil
	}

	result := make([]models.DockerService, 0, len(services))
	for _, svc := range services {
		service := models.DockerService{
			ID:             svc.ID,
			Name:           svc.Name,
			Stack:          svc.Stack,
			Image:          svc.Image,
			Mode:           svc.Mode,
			DesiredTasks:   svc.DesiredTasks,
			RunningTasks:   svc.RunningTasks,
			CompletedTasks: svc.CompletedTasks,
		}

		if len(svc.Labels) > 0 {
			service.Labels = cloneStringMap(svc.Labels)
		}

		if len(svc.EndpointPorts) > 0 {
			ports := make([]models.DockerServicePort, len(svc.EndpointPorts))
			for i, port := range svc.EndpointPorts {
				ports[i] = models.DockerServicePort{
					Name:          port.Name,
					Protocol:      port.Protocol,
					TargetPort:    port.TargetPort,
					PublishedPort: port.PublishedPort,
					PublishMode:   port.PublishMode,
				}
			}
			service.EndpointPorts = ports
		}

		if svc.UpdateStatus != nil {
			update := &models.DockerServiceUpdate{
				State:   svc.UpdateStatus.State,
				Message: svc.UpdateStatus.Message,
			}
			if svc.UpdateStatus.CompletedAt != nil && !svc.UpdateStatus.CompletedAt.IsZero() {
				completed := *svc.UpdateStatus.CompletedAt
				update.CompletedAt = &completed
			}
			service.UpdateStatus = update
		}

		if svc.CreatedAt != nil && !svc.CreatedAt.IsZero() {
			created := *svc.CreatedAt
			service.CreatedAt = &created
		}
		if svc.UpdatedAt != nil && !svc.UpdatedAt.IsZero() {
			updated := *svc.UpdatedAt
			service.UpdatedAt = &updated
		}

		result = append(result, service)
	}

	return result
}

func convertDockerTasks(tasks []agentsdocker.Task) []models.DockerTask {
	if len(tasks) == 0 {
		return nil
	}

	result := make([]models.DockerTask, 0, len(tasks))
	for _, task := range tasks {
		modelTask := models.DockerTask{
			ID:            task.ID,
			ServiceID:     task.ServiceID,
			ServiceName:   task.ServiceName,
			Slot:          task.Slot,
			NodeID:        task.NodeID,
			NodeName:      task.NodeName,
			DesiredState:  task.DesiredState,
			CurrentState:  task.CurrentState,
			Error:         task.Error,
			Message:       task.Message,
			ContainerID:   task.ContainerID,
			ContainerName: task.ContainerName,
			CreatedAt:     task.CreatedAt,
		}

		if task.UpdatedAt != nil && !task.UpdatedAt.IsZero() {
			updated := *task.UpdatedAt
			modelTask.UpdatedAt = &updated
		}
		if task.StartedAt != nil && !task.StartedAt.IsZero() {
			started := *task.StartedAt
			modelTask.StartedAt = &started
		}
		if task.CompletedAt != nil && !task.CompletedAt.IsZero() {
			completed := *task.CompletedAt
			modelTask.CompletedAt = &completed
		}

		result = append(result, modelTask)
	}

	return result
}

func normalizeAgentVersion(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return ""
	}
	version = strings.TrimLeft(version, "vV")
	if version == "" {
		return ""
	}
	return "v" + version
}

func convertDockerSwarmInfo(info *agentsdocker.SwarmInfo) *models.DockerSwarmInfo {
	if info == nil {
		return nil
	}

	return &models.DockerSwarmInfo{
		NodeID:           info.NodeID,
		NodeRole:         info.NodeRole,
		LocalState:       info.LocalState,
		ControlAvailable: info.ControlAvailable,
		ClusterID:        info.ClusterID,
		ClusterName:      info.ClusterName,
		Scope:            info.Scope,
		Error:            info.Error,
	}
}

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

	points, err := client.GetNodeRRDData(requestCtx, nodeName, "hour", "AVERAGE", []string{"memavailable", "memused", "memtotal"})
	if err != nil {
		return rrdMemCacheEntry{}, err
	}

	var memAvailable uint64
	var memUsed uint64
	var memTotal uint64

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

		if memTotal > 0 && (memAvailable > 0 || memUsed > 0) {
			break
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

	if memAvailable == 0 && memUsed == 0 {
		return rrdMemCacheEntry{}, fmt.Errorf("rrd mem metrics not present")
	}

	entry := rrdMemCacheEntry{
		available: memAvailable,
		used:      memUsed,
		total:     memTotal,
		fetchedAt: now,
	}

	m.rrdCacheMu.Lock()
	m.nodeRRDMemCache[nodeName] = entry
	m.rrdCacheMu.Unlock()

	return entry, nil
}

// RemoveDockerHost removes a docker host from the shared state and clears related alerts.
func (m *Monitor) RemoveDockerHost(hostID string) (models.DockerHost, error) {
	hostID = strings.TrimSpace(hostID)
	if hostID == "" {
		return models.DockerHost{}, fmt.Errorf("docker host id is required")
	}

	host, removed := m.state.RemoveDockerHost(hostID)
	if !removed {
		if logging.IsLevelEnabled(zerolog.DebugLevel) {
			log.Debug().Str("dockerHostID", hostID).Msg("Docker host not present in state during removal; proceeding to clear alerts")
		}
		host = models.DockerHost{
			ID:          hostID,
			Hostname:    hostID,
			DisplayName: hostID,
		}
	}

	// Revoke the API token associated with this Docker host
	if host.TokenID != "" {
		tokenRemoved := m.config.RemoveAPIToken(host.TokenID)
		if tokenRemoved {
			m.config.SortAPITokens()
			m.config.APITokenEnabled = m.config.HasAPITokens()

			if m.persistence != nil {
				if err := m.persistence.SaveAPITokens(m.config.APITokens); err != nil {
					log.Warn().Err(err).Str("tokenID", host.TokenID).Msg("Failed to persist API token revocation after Docker host removal")
				} else {
					log.Info().Str("tokenID", host.TokenID).Str("tokenName", host.TokenName).Msg("API token revoked for removed Docker host")
				}
			}
		}
	}

	// Track removal to prevent resurrection from cached reports
	removedAt := time.Now()

	m.mu.Lock()
	m.removedDockerHosts[hostID] = removedAt
	// Unbind the token so it can be reused with a different agent if needed
	if host.TokenID != "" {
		delete(m.dockerTokenBindings, host.TokenID)
		log.Debug().
			Str("tokenID", host.TokenID).
			Str("dockerHostID", hostID).
			Msg("Unbound Docker agent token from removed host")
	}
	if cmd, ok := m.dockerCommands[hostID]; ok {
		delete(m.dockerCommandIndex, cmd.status.ID)
	}
	delete(m.dockerCommands, hostID)
	m.mu.Unlock()

	m.state.AddRemovedDockerHost(models.RemovedDockerHost{
		ID:          hostID,
		Hostname:    host.Hostname,
		DisplayName: host.DisplayName,
		RemovedAt:   removedAt,
	})

	m.state.RemoveConnectionHealth(dockerConnectionPrefix + hostID)
	if m.alertManager != nil {
		m.alertManager.HandleDockerHostRemoved(host)
		m.SyncAlertState()
	}

	log.Info().
		Str("dockerHost", host.Hostname).
		Str("dockerHostID", hostID).
		Bool("removed", removed).
		Msg("Docker host removed and alerts cleared")

	return host, nil
}

// RemoveHostAgent removes a host agent from monitoring state and clears related data.
func (m *Monitor) RemoveHostAgent(hostID string) (models.Host, error) {
	hostID = strings.TrimSpace(hostID)
	if hostID == "" {
		return models.Host{}, fmt.Errorf("host id is required")
	}

	host, removed := m.state.RemoveHost(hostID)
	if !removed {
		if logging.IsLevelEnabled(zerolog.DebugLevel) {
			log.Debug().Str("hostID", hostID).Msg("Host not present in state during removal")
		}
		host = models.Host{
			ID:       hostID,
			Hostname: hostID,
		}
	}

	// Revoke the API token associated with this host agent
	if host.TokenID != "" {
		tokenRemoved := m.config.RemoveAPIToken(host.TokenID)
		if tokenRemoved {
			m.config.SortAPITokens()
			m.config.APITokenEnabled = m.config.HasAPITokens()

			if m.persistence != nil {
				if err := m.persistence.SaveAPITokens(m.config.APITokens); err != nil {
					log.Warn().Err(err).Str("tokenID", host.TokenID).Msg("Failed to persist API token revocation after host agent removal")
				} else {
					log.Info().Str("tokenID", host.TokenID).Str("tokenName", host.TokenName).Msg("API token revoked for removed host agent")
				}
			}
		}
	}

	if host.TokenID != "" {
		m.mu.Lock()
		if _, exists := m.hostTokenBindings[host.TokenID]; exists {
			delete(m.hostTokenBindings, host.TokenID)
			log.Debug().
				Str("tokenID", host.TokenID).
				Str("hostID", hostID).
				Msg("Unbound host agent token from removed host")
		}
		m.mu.Unlock()
	}

	m.state.RemoveConnectionHealth(hostConnectionPrefix + hostID)

	log.Info().
		Str("host", host.Hostname).
		Str("hostID", hostID).
		Bool("removed", removed).
		Msg("Host agent removed from monitoring")

	if m.alertManager != nil {
		m.alertManager.HandleHostRemoved(host)
	}

	return host, nil
}

// HideDockerHost marks a docker host as hidden without removing it from state.
// Hidden hosts will not be shown in the frontend but will continue to accept updates.
func (m *Monitor) HideDockerHost(hostID string) (models.DockerHost, error) {
	hostID = strings.TrimSpace(hostID)
	if hostID == "" {
		return models.DockerHost{}, fmt.Errorf("docker host id is required")
	}

	host, ok := m.state.SetDockerHostHidden(hostID, true)
	if !ok {
		return models.DockerHost{}, fmt.Errorf("docker host %q not found", hostID)
	}

	log.Info().
		Str("dockerHost", host.Hostname).
		Str("dockerHostID", hostID).
		Msg("Docker host hidden from view")

	return host, nil
}

// UnhideDockerHost marks a docker host as visible again.
func (m *Monitor) UnhideDockerHost(hostID string) (models.DockerHost, error) {
	hostID = strings.TrimSpace(hostID)
	if hostID == "" {
		return models.DockerHost{}, fmt.Errorf("docker host id is required")
	}

	host, ok := m.state.SetDockerHostHidden(hostID, false)
	if !ok {
		return models.DockerHost{}, fmt.Errorf("docker host %q not found", hostID)
	}

	// Clear removal tracking if it was marked as removed
	m.mu.Lock()
	delete(m.removedDockerHosts, hostID)
	m.mu.Unlock()

	log.Info().
		Str("dockerHost", host.Hostname).
		Str("dockerHostID", hostID).
		Msg("Docker host unhidden")

	return host, nil
}

// MarkDockerHostPendingUninstall marks a docker host as pending uninstall.
// This is used when the user has run the uninstall command and is waiting for the host to go offline.
func (m *Monitor) MarkDockerHostPendingUninstall(hostID string) (models.DockerHost, error) {
	hostID = strings.TrimSpace(hostID)
	if hostID == "" {
		return models.DockerHost{}, fmt.Errorf("docker host id is required")
	}

	host, ok := m.state.SetDockerHostPendingUninstall(hostID, true)
	if !ok {
		return models.DockerHost{}, fmt.Errorf("docker host %q not found", hostID)
	}

	log.Info().
		Str("dockerHost", host.Hostname).
		Str("dockerHostID", hostID).
		Msg("Docker host marked as pending uninstall")

	return host, nil
}

// SetDockerHostCustomDisplayName updates the custom display name for a docker host.
func (m *Monitor) SetDockerHostCustomDisplayName(hostID string, customName string) (models.DockerHost, error) {
	hostID = strings.TrimSpace(hostID)
	if hostID == "" {
		return models.DockerHost{}, fmt.Errorf("docker host id is required")
	}

	customName = strings.TrimSpace(customName)

	// Persist to Docker metadata store first
	var hostMeta *config.DockerHostMetadata
	if customName != "" {
		hostMeta = &config.DockerHostMetadata{
			CustomDisplayName: customName,
		}
	}
	if err := m.dockerMetadataStore.SetHostMetadata(hostID, hostMeta); err != nil {
		log.Error().Err(err).Str("hostID", hostID).Msg("Failed to persist Docker host metadata")
		return models.DockerHost{}, fmt.Errorf("failed to persist custom display name: %w", err)
	}

	// Update in-memory state
	host, ok := m.state.SetDockerHostCustomDisplayName(hostID, customName)
	if !ok {
		return models.DockerHost{}, fmt.Errorf("docker host %q not found", hostID)
	}

	log.Info().
		Str("dockerHost", host.Hostname).
		Str("dockerHostID", hostID).
		Str("customDisplayName", customName).
		Msg("Docker host custom display name updated")

	return host, nil
}

// AllowDockerHostReenroll removes a host ID from the removal blocklist so it can report again.
func (m *Monitor) AllowDockerHostReenroll(hostID string) error {
	hostID = strings.TrimSpace(hostID)
	if hostID == "" {
		return fmt.Errorf("docker host id is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.removedDockerHosts[hostID]; !exists {
		host, found := m.GetDockerHost(hostID)
		event := log.Info().
			Str("dockerHostID", hostID)
		if found {
			event = event.Str("dockerHost", host.Hostname)
		}
		event.Msg("Allow re-enroll requested but host was not blocked; ignoring")
		return nil
	}

	delete(m.removedDockerHosts, hostID)
	if cmd, exists := m.dockerCommands[hostID]; exists {
		delete(m.dockerCommandIndex, cmd.status.ID)
		delete(m.dockerCommands, hostID)
	}
	m.state.SetDockerHostCommand(hostID, nil)
	m.state.RemoveRemovedDockerHost(hostID)

	log.Info().
		Str("dockerHostID", hostID).
		Msg("Docker host removal block cleared; host may report again")

	return nil
}

// GetDockerHost retrieves a docker host by identifier if present in state.
func (m *Monitor) GetDockerHost(hostID string) (models.DockerHost, bool) {
	hostID = strings.TrimSpace(hostID)
	if hostID == "" {
		return models.DockerHost{}, false
	}

	hosts := m.state.GetDockerHosts()
	for _, host := range hosts {
		if host.ID == hostID {
			return host, true
		}
	}
	return models.DockerHost{}, false
}

// GetDockerHosts returns a point-in-time snapshot of all Docker hosts Pulse knows about.
func (m *Monitor) GetDockerHosts() []models.DockerHost {
	if m == nil || m.state == nil {
		return nil
	}
	return m.state.GetDockerHosts()
}

// RebuildTokenBindings reconstructs agent-to-token binding maps from the current
// state of Docker hosts and host agents. This should be called after API tokens
// are reloaded from disk to ensure bindings remain consistent with the new token set.
// It preserves bindings for tokens that still exist and removes orphaned entries.
func (m *Monitor) RebuildTokenBindings() {
	if m == nil || m.state == nil || m.config == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Build a set of valid token IDs from the current config
	validTokens := make(map[string]struct{})
	for _, token := range m.config.APITokens {
		if token.ID != "" {
			validTokens[token.ID] = struct{}{}
		}
	}

	// Rebuild Docker token bindings
	newDockerBindings := make(map[string]string)
	dockerHosts := m.state.GetDockerHosts()
	for _, host := range dockerHosts {
		tokenID := strings.TrimSpace(host.TokenID)
		if tokenID == "" {
			continue
		}
		// Only keep bindings for tokens that still exist in config
		if _, valid := validTokens[tokenID]; !valid {
			continue
		}
		// Use AgentID if available, otherwise fall back to host ID
		agentID := strings.TrimSpace(host.AgentID)
		if agentID == "" {
			agentID = host.ID
		}
		if agentID != "" {
			newDockerBindings[tokenID] = agentID
		}
	}

	// Rebuild Host agent token bindings
	newHostBindings := make(map[string]string)
	hosts := m.state.GetHosts()
	for _, host := range hosts {
		tokenID := strings.TrimSpace(host.TokenID)
		if tokenID == "" {
			continue
		}
		// Only keep bindings for tokens that still exist in config
		if _, valid := validTokens[tokenID]; !valid {
			continue
		}
		// Use host ID as the binding identifier
		if host.ID != "" {
			newHostBindings[tokenID] = host.ID
		}
	}

	// Log what changed
	oldDockerCount := len(m.dockerTokenBindings)
	oldHostCount := len(m.hostTokenBindings)
	m.dockerTokenBindings = newDockerBindings
	m.hostTokenBindings = newHostBindings

	log.Info().
		Int("dockerBindings", len(newDockerBindings)).
		Int("hostBindings", len(newHostBindings)).
		Int("previousDockerBindings", oldDockerCount).
		Int("previousHostBindings", oldHostCount).
		Int("validTokens", len(validTokens)).
		Msg("Rebuilt agent token bindings after API token reload")
}

// QueueDockerHostStop queues a stop command for the specified docker host.
func (m *Monitor) QueueDockerHostStop(hostID string) (models.DockerHostCommandStatus, error) {
	return m.queueDockerStopCommand(hostID)
}

// FetchDockerCommandForHost retrieves the next command payload (if any) for the host.
func (m *Monitor) FetchDockerCommandForHost(hostID string) (map[string]any, *models.DockerHostCommandStatus) {
	return m.getDockerCommandPayload(hostID)
}

// AcknowledgeDockerHostCommand updates the lifecycle status for a docker host command.
func (m *Monitor) AcknowledgeDockerHostCommand(commandID, hostID, status, message string) (models.DockerHostCommandStatus, string, bool, error) {
	return m.acknowledgeDockerCommand(commandID, hostID, status, message)
}

// ApplyDockerReport ingests a docker agent report into the shared state.
func (m *Monitor) ApplyDockerReport(report agentsdocker.Report, tokenRecord *config.APITokenRecord) (models.DockerHost, error) {
	hostsSnapshot := m.state.GetDockerHosts()
	identifier, legacyIDs, previous, hasPrevious := resolveDockerHostIdentifier(report, tokenRecord, hostsSnapshot)
	if strings.TrimSpace(identifier) == "" {
		return models.DockerHost{}, fmt.Errorf("docker report missing agent identifier")
	}

	// Check if this host was deliberately removed - reject report to prevent resurrection
	m.mu.RLock()
	removedAt, wasRemoved := m.removedDockerHosts[identifier]
	if !wasRemoved {
		for _, legacyID := range legacyIDs {
			if legacyID == "" || legacyID == identifier {
				continue
			}
			if ts, ok := m.removedDockerHosts[legacyID]; ok {
				removedAt = ts
				wasRemoved = true
				break
			}
		}
	}
	m.mu.RUnlock()

	if wasRemoved {
		log.Info().
			Str("dockerHostID", identifier).
			Time("removedAt", removedAt).
			Msg("Rejecting report from deliberately removed Docker host")
		return models.DockerHost{}, fmt.Errorf("docker host %q was removed at %v and cannot report again. Use Allow re-enroll in Settings -> Agents -> Removed Docker Hosts or rerun the installer with a docker:manage token to clear this block", identifier, removedAt.Format(time.RFC3339))
	}

	// Enforce token uniqueness: each token can only be bound to one agent
	if tokenRecord != nil && tokenRecord.ID != "" {
		tokenID := strings.TrimSpace(tokenRecord.ID)
		agentID := strings.TrimSpace(report.Agent.ID)
		if agentID == "" {
			agentID = identifier
		}

		m.mu.Lock()
		if boundAgentID, exists := m.dockerTokenBindings[tokenID]; exists {
			if boundAgentID != agentID {
				m.mu.Unlock()
				// Find the conflicting host to provide helpful error message
				conflictingHostname := "unknown"
				for _, host := range hostsSnapshot {
					if host.AgentID == boundAgentID || host.ID == boundAgentID {
						conflictingHostname = host.Hostname
						if host.CustomDisplayName != "" {
							conflictingHostname = host.CustomDisplayName
						} else if host.DisplayName != "" {
							conflictingHostname = host.DisplayName
						}
						break
					}
				}
				tokenHint := tokenHintFromRecord(tokenRecord)
				if tokenHint != "" {
					tokenHint = " (" + tokenHint + ")"
				}
				log.Warn().
					Str("tokenID", tokenID).
					Str("tokenHint", tokenHint).
					Str("reportingAgentID", agentID).
					Str("boundAgentID", boundAgentID).
					Str("conflictingHost", conflictingHostname).
					Msg("Rejecting Docker report: token already bound to different agent")
				return models.DockerHost{}, fmt.Errorf("API token%s is already in use by agent %q (host: %s). Each Docker agent must use a unique API token. Generate a new token for this agent", tokenHint, boundAgentID, conflictingHostname)
			}
		} else {
			// First time seeing this token - bind it to this agent
			m.dockerTokenBindings[tokenID] = agentID
			log.Debug().
				Str("tokenID", tokenID).
				Str("agentID", agentID).
				Str("hostname", report.Host.Hostname).
				Msg("Bound Docker agent token to agent identity")
		}
		m.mu.Unlock()
	}

	hostname := strings.TrimSpace(report.Host.Hostname)
	if hostname == "" {
		return models.DockerHost{}, fmt.Errorf("docker report missing hostname")
	}

	timestamp := report.Timestamp
	if timestamp.IsZero() {
		timestamp = time.Now()
	}

	agentID := strings.TrimSpace(report.Agent.ID)
	if agentID == "" {
		agentID = identifier
	}

	displayName := strings.TrimSpace(report.Host.Name)
	if displayName == "" {
		displayName = hostname
	}

	runtime := strings.ToLower(strings.TrimSpace(report.Host.Runtime))
	switch runtime {
	case "", "auto", "default":
		runtime = "docker"
	case "docker", "podman":
		// supported runtimes
	default:
		runtime = "docker"
	}

	runtimeVersion := strings.TrimSpace(report.Host.RuntimeVersion)
	dockerVersion := strings.TrimSpace(report.Host.DockerVersion)
	if runtimeVersion == "" {
		runtimeVersion = dockerVersion
	}
	if dockerVersion == "" {
		dockerVersion = runtimeVersion
	}

	containers := make([]models.DockerContainer, 0, len(report.Containers))
	for _, payload := range report.Containers {
		container := models.DockerContainer{
			ID:            payload.ID,
			Name:          payload.Name,
			Image:         payload.Image,
			State:         payload.State,
			Status:        payload.Status,
			Health:        payload.Health,
			CPUPercent:    safeFloat(payload.CPUPercent),
			MemoryUsage:   payload.MemoryUsageBytes,
			MemoryLimit:   payload.MemoryLimitBytes,
			MemoryPercent: safeFloat(payload.MemoryPercent),
			UptimeSeconds: payload.UptimeSeconds,
			RestartCount:  payload.RestartCount,
			ExitCode:      payload.ExitCode,
			CreatedAt:     payload.CreatedAt,
			StartedAt:     payload.StartedAt,
			FinishedAt:    payload.FinishedAt,
		}

		if len(payload.Ports) > 0 {
			ports := make([]models.DockerContainerPort, len(payload.Ports))
			for i, port := range payload.Ports {
				ports[i] = models.DockerContainerPort{
					PrivatePort: port.PrivatePort,
					PublicPort:  port.PublicPort,
					Protocol:    port.Protocol,
					IP:          port.IP,
				}
			}
			container.Ports = ports
		}

		if len(payload.Labels) > 0 {
			labels := make(map[string]string, len(payload.Labels))
			for k, v := range payload.Labels {
				labels[k] = v
			}
			container.Labels = labels
		}

		if len(payload.Networks) > 0 {
			networks := make([]models.DockerContainerNetworkLink, len(payload.Networks))
			for i, net := range payload.Networks {
				networks[i] = models.DockerContainerNetworkLink{
					Name: net.Name,
					IPv4: net.IPv4,
					IPv6: net.IPv6,
				}
			}
			container.Networks = networks
		}

		container.WritableLayerBytes = payload.WritableLayerBytes
		container.RootFilesystemBytes = payload.RootFilesystemBytes

		if payload.BlockIO != nil {
			container.BlockIO = &models.DockerContainerBlockIO{
				ReadBytes:  payload.BlockIO.ReadBytes,
				WriteBytes: payload.BlockIO.WriteBytes,
			}

			containerIdentifier := payload.ID
			if strings.TrimSpace(containerIdentifier) == "" {
				containerIdentifier = payload.Name
			}
			if strings.TrimSpace(containerIdentifier) != "" {
				metrics := types.IOMetrics{
					DiskRead:  clampUint64ToInt64(payload.BlockIO.ReadBytes),
					DiskWrite: clampUint64ToInt64(payload.BlockIO.WriteBytes),
					Timestamp: timestamp,
				}
				readRate, writeRate, _, _ := m.rateTracker.CalculateRates(fmt.Sprintf("docker:%s:%s", identifier, containerIdentifier), metrics)
				if readRate >= 0 {
					value := readRate
					container.BlockIO.ReadRateBytesPerSecond = &value
				}
				if writeRate >= 0 {
					value := writeRate
					container.BlockIO.WriteRateBytesPerSecond = &value
				}
			}
		}

		if len(payload.Mounts) > 0 {
			mounts := make([]models.DockerContainerMount, len(payload.Mounts))
			for i, mount := range payload.Mounts {
				mounts[i] = models.DockerContainerMount{
					Type:        mount.Type,
					Source:      mount.Source,
					Destination: mount.Destination,
					Mode:        mount.Mode,
					RW:          mount.RW,
					Propagation: mount.Propagation,
					Name:        mount.Name,
					Driver:      mount.Driver,
				}
			}
			container.Mounts = mounts
		}

		containers = append(containers, container)
	}

	services := convertDockerServices(report.Services)
	tasks := convertDockerTasks(report.Tasks)
	swarmInfo := convertDockerSwarmInfo(report.Host.Swarm)

	loadAverage := make([]float64, 0, len(report.Host.LoadAverage))
	if len(report.Host.LoadAverage) > 0 {
		loadAverage = append(loadAverage, report.Host.LoadAverage...)
	}

	var memory models.Memory
	if report.Host.Memory.TotalBytes > 0 || report.Host.Memory.UsedBytes > 0 {
		memory = models.Memory{
			Total:     report.Host.Memory.TotalBytes,
			Used:      report.Host.Memory.UsedBytes,
			Free:      report.Host.Memory.FreeBytes,
			Usage:     safeFloat(report.Host.Memory.Usage),
			SwapTotal: report.Host.Memory.SwapTotal,
			SwapUsed:  report.Host.Memory.SwapUsed,
		}
	}

	disks := make([]models.Disk, 0, len(report.Host.Disks))
	for _, disk := range report.Host.Disks {
		disks = append(disks, models.Disk{
			Total:      disk.TotalBytes,
			Used:       disk.UsedBytes,
			Free:       disk.FreeBytes,
			Usage:      safeFloat(disk.Usage),
			Mountpoint: disk.Mountpoint,
			Type:       disk.Type,
			Device:     disk.Device,
		})
	}

	networkIfaces := make([]models.HostNetworkInterface, 0, len(report.Host.Network))
	for _, iface := range report.Host.Network {
		addresses := append([]string(nil), iface.Addresses...)
		networkIfaces = append(networkIfaces, models.HostNetworkInterface{
			Name:      iface.Name,
			MAC:       iface.MAC,
			Addresses: addresses,
			RXBytes:   iface.RXBytes,
			TXBytes:   iface.TXBytes,
			SpeedMbps: iface.SpeedMbps,
		})
	}

	agentVersion := normalizeAgentVersion(report.Agent.Version)
	if agentVersion == "" && hasPrevious {
		agentVersion = normalizeAgentVersion(previous.AgentVersion)
	}

	host := models.DockerHost{
		ID:                identifier,
		AgentID:           agentID,
		Hostname:          hostname,
		DisplayName:       displayName,
		MachineID:         strings.TrimSpace(report.Host.MachineID),
		OS:                report.Host.OS,
		KernelVersion:     report.Host.KernelVersion,
		Architecture:      report.Host.Architecture,
		Runtime:           runtime,
		RuntimeVersion:    runtimeVersion,
		DockerVersion:     dockerVersion,
		CPUs:              report.Host.TotalCPU,
		TotalMemoryBytes:  report.Host.TotalMemoryBytes,
		UptimeSeconds:     report.Host.UptimeSeconds,
		CPUUsage:          safeFloat(report.Host.CPUUsagePercent),
		LoadAverage:       loadAverage,
		Memory:            memory,
		Disks:             disks,
		NetworkInterfaces: networkIfaces,
		Status:            "online",
		LastSeen:          timestamp,
		IntervalSeconds:   report.Agent.IntervalSeconds,
		AgentVersion:      agentVersion,
		Containers:        containers,
		Services:          services,
		Tasks:             tasks,
		Swarm:             swarmInfo,
		IsLegacy:          isLegacyDockerAgent(report.Agent.Type),
	}

	if tokenRecord != nil {
		host.TokenID = tokenRecord.ID
		host.TokenName = tokenRecord.Name
		host.TokenHint = tokenHintFromRecord(tokenRecord)
		if tokenRecord.LastUsedAt != nil {
			t := tokenRecord.LastUsedAt.UTC()
			host.TokenLastUsedAt = &t
		} else {
			t := time.Now().UTC()
			host.TokenLastUsedAt = &t
		}
	} else if hasPrevious {
		host.TokenID = previous.TokenID
		host.TokenName = previous.TokenName
		host.TokenHint = previous.TokenHint
		host.TokenLastUsedAt = previous.TokenLastUsedAt
	}

	// Load custom display name from metadata store if not already set
	if host.CustomDisplayName == "" {
		if hostMeta := m.dockerMetadataStore.GetHostMetadata(identifier); hostMeta != nil {
			host.CustomDisplayName = hostMeta.CustomDisplayName
		}
	}

	m.state.UpsertDockerHost(host)
	m.state.SetConnectionHealth(dockerConnectionPrefix+host.ID, true)

	// Check if the host was previously hidden and is now visible again
	if hasPrevious && previous.Hidden && !host.Hidden {
		log.Info().
			Str("dockerHost", host.Hostname).
			Str("dockerHostID", host.ID).
			Msg("Docker host auto-unhidden after receiving report")
	}

	// Check if the host was pending uninstall - if so, log a warning that uninstall failed and clear the flag
	if hasPrevious && previous.PendingUninstall {
		log.Warn().
			Str("dockerHost", host.Hostname).
			Str("dockerHostID", host.ID).
			Msg("Docker host reporting again after pending uninstall - uninstall may have failed")

		// Clear the pending uninstall flag since the host is clearly still active
		m.state.SetDockerHostPendingUninstall(host.ID, false)
	}

	if m.alertManager != nil {
		m.alertManager.CheckDockerHost(host)
	}

	// Record Docker HOST metrics for sparkline charts
	now := time.Now()
	hostMetricKey := fmt.Sprintf("dockerHost:%s", host.ID)

	// Record host CPU usage
	m.metricsHistory.AddGuestMetric(hostMetricKey, "cpu", host.CPUUsage, now)

	// Record host Memory usage
	m.metricsHistory.AddGuestMetric(hostMetricKey, "memory", host.Memory.Usage, now)

	// Record host Disk usage (use first disk or calculate total)
	var hostDiskPercent float64
	if len(host.Disks) > 0 {
		hostDiskPercent = host.Disks[0].Usage
	}
	m.metricsHistory.AddGuestMetric(hostMetricKey, "disk", hostDiskPercent, now)

	// Also write to persistent SQLite store
	if m.metricsStore != nil {
		m.metricsStore.Write("dockerHost", host.ID, "cpu", host.CPUUsage, now)
		m.metricsStore.Write("dockerHost", host.ID, "memory", host.Memory.Usage, now)
		m.metricsStore.Write("dockerHost", host.ID, "disk", hostDiskPercent, now)
	}

	// Record Docker CONTAINER metrics for sparkline charts
	// Use a prefixed key (docker:containerID) to distinguish from Proxmox containers
	for _, container := range containers {
		if container.ID == "" {
			continue
		}
		// Build a unique metric key for Docker containers
		metricKey := fmt.Sprintf("docker:%s", container.ID)

		// Record CPU (already a percentage 0-100)
		m.metricsHistory.AddGuestMetric(metricKey, "cpu", container.CPUPercent, now)

		// Record Memory (already a percentage 0-100)
		m.metricsHistory.AddGuestMetric(metricKey, "memory", container.MemoryPercent, now)

		// Record Disk usage as percentage of writable layer vs root filesystem
		var diskPercent float64
		if container.RootFilesystemBytes > 0 && container.WritableLayerBytes > 0 {
			diskPercent = float64(container.WritableLayerBytes) / float64(container.RootFilesystemBytes) * 100
			if diskPercent > 100 {
				diskPercent = 100
			}
		}
		m.metricsHistory.AddGuestMetric(metricKey, "disk", diskPercent, now)

		// Also write to persistent SQLite store for long-term storage
		if m.metricsStore != nil {
			m.metricsStore.Write("dockerContainer", container.ID, "cpu", container.CPUPercent, now)
			m.metricsStore.Write("dockerContainer", container.ID, "memory", container.MemoryPercent, now)
			m.metricsStore.Write("dockerContainer", container.ID, "disk", diskPercent, now)
		}
	}

	log.Debug().
		Str("dockerHost", host.Hostname).
		Int("containers", len(containers)).
		Msg("Docker host report processed")

	return host, nil
}

// ApplyHostReport ingests a host agent report into the shared state.
func (m *Monitor) ApplyHostReport(report agentshost.Report, tokenRecord *config.APITokenRecord) (models.Host, error) {
	hostname := strings.TrimSpace(report.Host.Hostname)
	if hostname == "" {
		return models.Host{}, fmt.Errorf("host report missing hostname")
	}

	identifier := strings.TrimSpace(report.Host.ID)
	if identifier != "" {
		identifier = sanitizeDockerHostSuffix(identifier)
	}
	if identifier == "" {
		if machine := sanitizeDockerHostSuffix(report.Host.MachineID); machine != "" {
			identifier = machine
		}
	}
	if identifier == "" {
		if agentID := sanitizeDockerHostSuffix(report.Agent.ID); agentID != "" {
			identifier = agentID
		}
	}
	if identifier == "" {
		if hostName := sanitizeDockerHostSuffix(hostname); hostName != "" {
			identifier = hostName
		}
	}
	if identifier == "" {
		seedParts := uniqueNonEmptyStrings(
			report.Host.MachineID,
			report.Agent.ID,
			report.Host.Hostname,
		)
		if len(seedParts) == 0 {
			seedParts = []string{hostname}
		}
		seed := strings.Join(seedParts, "|")
		sum := sha1.Sum([]byte(seed))
		identifier = fmt.Sprintf("host-%s", hex.EncodeToString(sum[:6]))
	}

	existingHosts := m.state.GetHosts()

	agentID := strings.TrimSpace(report.Agent.ID)
	if agentID == "" {
		agentID = identifier
	}

	if tokenRecord != nil && tokenRecord.ID != "" {
		tokenID := strings.TrimSpace(tokenRecord.ID)
		bindingID := agentID
		if bindingID == "" {
			bindingID = identifier
		}

		m.mu.Lock()
		if m.hostTokenBindings == nil {
			m.hostTokenBindings = make(map[string]string)
		}
		if boundID, exists := m.hostTokenBindings[tokenID]; exists && boundID != bindingID {
			m.mu.Unlock()

			conflictingHost := "unknown"
			for _, candidate := range existingHosts {
				if candidate.TokenID == tokenID || candidate.ID == boundID {
					conflictingHost = candidate.Hostname
					if candidate.DisplayName != "" {
						conflictingHost = candidate.DisplayName
					}
					break
				}
			}

			tokenHint := tokenHintFromRecord(tokenRecord)
			if tokenHint != "" {
				tokenHint = " (" + tokenHint + ")"
			}

			log.Warn().
				Str("tokenID", tokenID).
				Str("tokenHint", tokenHint).
				Str("reportingAgentID", bindingID).
				Str("boundAgentID", boundID).
				Str("conflictingHost", conflictingHost).
				Msg("Rejecting host report: token already bound to different agent")

			return models.Host{}, fmt.Errorf("API token%s is already in use by host %q (agent: %s). Generate a new token or set --agent-id before reusing it", tokenHint, conflictingHost, boundID)
		}

		if _, exists := m.hostTokenBindings[tokenID]; !exists {
			m.hostTokenBindings[tokenID] = bindingID
			log.Debug().
				Str("tokenID", tokenID).
				Str("agentID", bindingID).
				Str("hostname", hostname).
				Msg("Bound host agent token to agent identity")
		}
		m.mu.Unlock()
	}

	var previous models.Host
	var hasPrevious bool
	for _, candidate := range existingHosts {
		if candidate.ID == identifier {
			previous = candidate
			hasPrevious = true
			break
		}
	}

	displayName := strings.TrimSpace(report.Host.DisplayName)
	if displayName == "" {
		displayName = hostname
	}

	timestamp := report.Timestamp
	if timestamp.IsZero() {
		timestamp = time.Now().UTC()
	}

	memory := models.Memory{
		Total:     report.Metrics.Memory.TotalBytes,
		Used:      report.Metrics.Memory.UsedBytes,
		Free:      report.Metrics.Memory.FreeBytes,
		Usage:     safeFloat(report.Metrics.Memory.Usage),
		SwapTotal: report.Metrics.Memory.SwapTotal,
		SwapUsed:  report.Metrics.Memory.SwapUsed,
	}
	if memory.Usage <= 0 && memory.Total > 0 {
		memory.Usage = safePercentage(float64(memory.Used), float64(memory.Total))
	}

	disks := make([]models.Disk, 0, len(report.Disks))
	for _, disk := range report.Disks {
		// Filter virtual/system filesystems and read-only filesystems to avoid cluttering
		// the UI with tmpfs, devtmpfs, /dev, /run, /sys, docker overlay mounts, snap mounts,
		// immutable OS images, etc. (issues #505, #690, #790).
		if shouldSkip, _ := fsfilters.ShouldSkipFilesystem(disk.Type, disk.Mountpoint, uint64(disk.TotalBytes), uint64(disk.UsedBytes)); shouldSkip {
			continue
		}

		usage := safeFloat(disk.Usage)
		if usage <= 0 && disk.TotalBytes > 0 {
			usage = safePercentage(float64(disk.UsedBytes), float64(disk.TotalBytes))
		}
		disks = append(disks, models.Disk{
			Total:      disk.TotalBytes,
			Used:       disk.UsedBytes,
			Free:       disk.FreeBytes,
			Usage:      usage,
			Mountpoint: disk.Mountpoint,
			Type:       disk.Type,
			Device:     disk.Device,
		})
	}

	diskIO := make([]models.DiskIO, 0, len(report.DiskIO))
	for _, io := range report.DiskIO {
		diskIO = append(diskIO, models.DiskIO{
			Device:     io.Device,
			ReadBytes:  io.ReadBytes,
			WriteBytes: io.WriteBytes,
			ReadOps:    io.ReadOps,
			WriteOps:   io.WriteOps,
			ReadTime:   io.ReadTime,
			WriteTime:  io.WriteTime,
			IOTime:     io.IOTime,
		})
	}

	network := make([]models.HostNetworkInterface, 0, len(report.Network))
	for _, nic := range report.Network {
		network = append(network, models.HostNetworkInterface{
			Name:      nic.Name,
			MAC:       nic.MAC,
			Addresses: append([]string(nil), nic.Addresses...),
			RXBytes:   nic.RXBytes,
			TXBytes:   nic.TXBytes,
			SpeedMbps: nic.SpeedMbps,
		})
	}

	raid := make([]models.HostRAIDArray, 0, len(report.RAID))
	for _, array := range report.RAID {
		devices := make([]models.HostRAIDDevice, 0, len(array.Devices))
		for _, dev := range array.Devices {
			devices = append(devices, models.HostRAIDDevice{
				Device: dev.Device,
				State:  dev.State,
				Slot:   dev.Slot,
			})
		}
		raid = append(raid, models.HostRAIDArray{
			Device:         array.Device,
			Name:           array.Name,
			Level:          array.Level,
			State:          array.State,
			TotalDevices:   array.TotalDevices,
			ActiveDevices:  array.ActiveDevices,
			WorkingDevices: array.WorkingDevices,
			FailedDevices:  array.FailedDevices,
			SpareDevices:   array.SpareDevices,
			UUID:           array.UUID,
			Devices:        devices,
			RebuildPercent: array.RebuildPercent,
			RebuildSpeed:   array.RebuildSpeed,
		})
	}

	// Convert Ceph data from agent report
	var cephData *models.HostCephCluster
	if report.Ceph != nil {
		cephData = convertAgentCephToModels(report.Ceph)
	}

	host := models.Host{
		ID:                identifier,
		Hostname:          hostname,
		DisplayName:       displayName,
		Platform:          strings.TrimSpace(strings.ToLower(report.Host.Platform)),
		OSName:            strings.TrimSpace(report.Host.OSName),
		OSVersion:         strings.TrimSpace(report.Host.OSVersion),
		KernelVersion:     strings.TrimSpace(report.Host.KernelVersion),
		Architecture:      strings.TrimSpace(report.Host.Architecture),
		CPUCount:          report.Host.CPUCount,
		CPUUsage:          safeFloat(report.Metrics.CPUUsagePercent),
		LoadAverage:       append([]float64(nil), report.Host.LoadAverage...),
		Memory:            memory,
		Disks:             disks,
		DiskIO:            diskIO,
		NetworkInterfaces: network,
		Sensors: models.HostSensorSummary{
			TemperatureCelsius: cloneStringFloatMap(report.Sensors.TemperatureCelsius),
			FanRPM:             cloneStringFloatMap(report.Sensors.FanRPM),
			Additional:         cloneStringFloatMap(report.Sensors.Additional),
		},
		RAID:            raid,
		Ceph:            cephData,
		Status:          "online",
		UptimeSeconds:   report.Host.UptimeSeconds,
		IntervalSeconds: report.Agent.IntervalSeconds,
		LastSeen:        timestamp,
		AgentVersion:    strings.TrimSpace(report.Agent.Version),
		Tags:            append([]string(nil), report.Tags...),
		IsLegacy:        isLegacyHostAgent(report.Agent.Type),
	}

	if len(host.LoadAverage) == 0 {
		host.LoadAverage = nil
	}
	if len(host.Disks) == 0 {
		host.Disks = nil
	}
	if len(host.DiskIO) == 0 {
		host.DiskIO = nil
	}
	if len(host.NetworkInterfaces) == 0 {
		host.NetworkInterfaces = nil
	}
	if len(host.RAID) == 0 {
		host.RAID = nil
	}

	if tokenRecord != nil {
		host.TokenID = tokenRecord.ID
		host.TokenName = tokenRecord.Name
		host.TokenHint = tokenHintFromRecord(tokenRecord)
		if tokenRecord.LastUsedAt != nil {
			t := tokenRecord.LastUsedAt.UTC()
			host.TokenLastUsedAt = &t
		} else {
			now := time.Now().UTC()
			host.TokenLastUsedAt = &now
		}
	} else if hasPrevious {
		host.TokenID = previous.TokenID
		host.TokenName = previous.TokenName
		host.TokenHint = previous.TokenHint
		host.TokenLastUsedAt = previous.TokenLastUsedAt
	}

	m.state.UpsertHost(host)
	m.state.SetConnectionHealth(hostConnectionPrefix+host.ID, true)

	// If host reports Ceph data, also update the global CephClusters state
	if report.Ceph != nil {
		cephCluster := convertAgentCephToGlobalCluster(report.Ceph, hostname, identifier, timestamp)
		m.state.UpsertCephCluster(cephCluster)
		log.Debug().
			Str("hostId", identifier).
			Str("hostname", hostname).
			Str("fsid", cephCluster.FSID).
			Str("health", cephCluster.Health).
			Int("osds", cephCluster.NumOSDs).
			Msg("Updated Ceph cluster from host agent")
	}

	if m.alertManager != nil {
		m.alertManager.CheckHost(host)
	}

	return host, nil
}

const (
	removedDockerHostsTTL = 24 * time.Hour // Clean up removed hosts tracking after 24 hours
)

// recoverFromPanic recovers from panics in monitoring goroutines and logs them.
// This prevents a panic in one component from crashing the entire monitoring system.
func recoverFromPanic(goroutineName string) {
	if r := recover(); r != nil {
		log.Error().
			Str("goroutine", goroutineName).
			Interface("panic", r).
			Stack().
			Msg("Recovered from panic in monitoring goroutine")
	}
}

// cleanupRemovedDockerHosts removes entries from the removed hosts map that are older than 24 hours.
func (m *Monitor) cleanupRemovedDockerHosts(now time.Time) {
	// Collect IDs to remove first to avoid holding lock during state update
	var toRemove []string

	m.mu.Lock()
	for hostID, removedAt := range m.removedDockerHosts {
		if now.Sub(removedAt) > removedDockerHostsTTL {
			toRemove = append(toRemove, hostID)
		}
	}
	m.mu.Unlock()

	// Remove from state and map without holding both locks
	for _, hostID := range toRemove {
		m.state.RemoveRemovedDockerHost(hostID)

		m.mu.Lock()
		removedAt := m.removedDockerHosts[hostID]
		delete(m.removedDockerHosts, hostID)
		m.mu.Unlock()

		log.Debug().
			Str("dockerHostID", hostID).
			Time("removedAt", removedAt).
			Msg("Cleaned up old removed Docker host entry")
	}
}

// cleanupGuestMetadataCache removes stale guest metadata entries.
// Entries older than 2x the cache TTL (10 minutes) are removed to prevent unbounded growth
// when VMs are deleted or moved.
func (m *Monitor) cleanupGuestMetadataCache(now time.Time) {
	const maxAge = 2 * guestMetadataCacheTTL // 10 minutes

	m.guestMetadataMu.Lock()
	defer m.guestMetadataMu.Unlock()

	for key, entry := range m.guestMetadataCache {
		if now.Sub(entry.fetchedAt) > maxAge {
			delete(m.guestMetadataCache, key)
			log.Debug().
				Str("key", key).
				Time("fetchedAt", entry.fetchedAt).
				Msg("Cleaned up stale guest metadata cache entry")
		}
	}
}

// cleanupDiagnosticSnapshots removes stale diagnostic snapshots.
// Snapshots older than 1 hour are removed to prevent unbounded growth
// when nodes/VMs are deleted or reconfigured.
func (m *Monitor) cleanupDiagnosticSnapshots(now time.Time) {
	const maxAge = 1 * time.Hour

	m.mu.Lock()
	defer m.mu.Unlock()

	for key, snapshot := range m.nodeSnapshots {
		if now.Sub(snapshot.RetrievedAt) > maxAge {
			delete(m.nodeSnapshots, key)
			log.Debug().
				Str("key", key).
				Time("retrievedAt", snapshot.RetrievedAt).
				Msg("Cleaned up stale node snapshot")
		}
	}

	for key, snapshot := range m.guestSnapshots {
		if now.Sub(snapshot.RetrievedAt) > maxAge {
			delete(m.guestSnapshots, key)
			log.Debug().
				Str("key", key).
				Time("retrievedAt", snapshot.RetrievedAt).
				Msg("Cleaned up stale guest snapshot")
		}
	}
}

// cleanupRRDCache removes stale RRD memory cache entries.
// Entries older than 2x the cache TTL (1 minute) are removed to prevent unbounded growth
// when nodes are removed from the cluster.
func (m *Monitor) cleanupRRDCache(now time.Time) {
	const maxAge = 2 * nodeRRDCacheTTL // 1 minute

	m.rrdCacheMu.Lock()
	defer m.rrdCacheMu.Unlock()

	for key, entry := range m.nodeRRDMemCache {
		if now.Sub(entry.fetchedAt) > maxAge {
			delete(m.nodeRRDMemCache, key)
			log.Debug().
				Str("node", key).
				Time("fetchedAt", entry.fetchedAt).
				Msg("Cleaned up stale RRD cache entry")
		}
	}
}

// evaluateDockerAgents updates health for Docker hosts based on last report time.
func (m *Monitor) evaluateDockerAgents(now time.Time) {
	hosts := m.state.GetDockerHosts()
	for _, host := range hosts {
		interval := host.IntervalSeconds
		if interval <= 0 {
			interval = int(dockerMinimumHealthWindow / time.Second)
		}

		window := time.Duration(interval) * time.Second * dockerOfflineGraceMultiplier
		if window < dockerMinimumHealthWindow {
			window = dockerMinimumHealthWindow
		} else if window > dockerMaximumHealthWindow {
			window = dockerMaximumHealthWindow
		}

		healthy := !host.LastSeen.IsZero() && now.Sub(host.LastSeen) <= window
		key := dockerConnectionPrefix + host.ID
		m.state.SetConnectionHealth(key, healthy)
		hostCopy := host
		if healthy {
			hostCopy.Status = "online"
			m.state.SetDockerHostStatus(host.ID, "online")
			if m.alertManager != nil {
				m.alertManager.HandleDockerHostOnline(hostCopy)
			}
		} else {
			hostCopy.Status = "offline"
			m.state.SetDockerHostStatus(host.ID, "offline")
			if m.alertManager != nil {
				m.alertManager.HandleDockerHostOffline(hostCopy)
			}
		}
	}
}

// evaluateHostAgents updates health for host agents based on last report time.
func (m *Monitor) evaluateHostAgents(now time.Time) {
	hosts := m.state.GetHosts()
	for _, host := range hosts {
		interval := host.IntervalSeconds
		if interval <= 0 {
			interval = int(hostMinimumHealthWindow / time.Second)
		}

		window := time.Duration(interval) * time.Second * hostOfflineGraceMultiplier
		if window < hostMinimumHealthWindow {
			window = hostMinimumHealthWindow
		} else if window > hostMaximumHealthWindow {
			window = hostMaximumHealthWindow
		}

		age := now.Sub(host.LastSeen)
		healthy := !host.LastSeen.IsZero() && age <= window
		key := hostConnectionPrefix + host.ID
		m.state.SetConnectionHealth(key, healthy)

		hostCopy := host
		if healthy {
			hostCopy.Status = "online"
			// Log status transition from offline to online
			if host.Status == "offline" {
				log.Debug().
					Str("hostID", host.ID).
					Str("hostname", host.Hostname).
					Dur("age", age).
					Dur("window", window).
					Msg("Host agent back online")
			}
			m.state.SetHostStatus(host.ID, "online")
			if m.alertManager != nil {
				m.alertManager.HandleHostOnline(hostCopy)
			}
		} else {
			hostCopy.Status = "offline"
			// Log status transition from online to offline with diagnostic info
			if host.Status == "online" || host.Status == "" {
				log.Debug().
					Str("hostID", host.ID).
					Str("hostname", host.Hostname).
					Time("lastSeen", host.LastSeen).
					Dur("age", age).
					Dur("window", window).
					Int("intervalSeconds", host.IntervalSeconds).
					Bool("lastSeenZero", host.LastSeen.IsZero()).
					Msg("Host agent appears offline")
			}
			m.state.SetHostStatus(host.ID, "offline")
			if m.alertManager != nil {
				m.alertManager.HandleHostOffline(hostCopy)
			}
		}
	}
}

// sortContent sorts comma-separated content values for consistent display
func sortContent(content string) string {
	if content == "" {
		return ""
	}
	parts := strings.Split(content, ",")
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

func (m *Monitor) enrichContainerMetadata(ctx context.Context, client PVEClientInterface, instanceName, nodeName string, container *models.Container) {
	if container == nil {
		return
	}

	ensureContainerRootDiskEntry(container)

	if client == nil {
		return
	}

	isRunning := container.Status == "running"

	var status *proxmox.Container
	if isRunning {
		statusCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		statusResp, err := client.GetContainerStatus(statusCtx, nodeName, container.VMID)
		cancel()
		if err != nil {
			log.Debug().
				Err(err).
				Str("instance", instanceName).
				Str("node", nodeName).
				Str("container", container.Name).
				Int("vmid", container.VMID).
				Msg("Container status metadata unavailable")
		} else {
			status = statusResp
		}
	}

	rootDeviceHint := ""
	var mountMetadata map[string]containerMountMetadata
	addressSet := make(map[string]struct{})
	addressOrder := make([]string, 0, 4)

	addAddress := func(addr string) {
		addr = strings.TrimSpace(addr)
		if addr == "" {
			return
		}
		if _, exists := addressSet[addr]; exists {
			return
		}
		addressSet[addr] = struct{}{}
		addressOrder = append(addressOrder, addr)
	}

	if status != nil {
		for _, addr := range sanitizeGuestAddressStrings(status.IP) {
			addAddress(addr)
		}
		for _, addr := range sanitizeGuestAddressStrings(status.IP6) {
			addAddress(addr)
		}
		for _, addr := range parseContainerRawIPs(status.IPv4) {
			addAddress(addr)
		}
		for _, addr := range parseContainerRawIPs(status.IPv6) {
			addAddress(addr)
		}
	}

	networkIfaces := make([]models.GuestNetworkInterface, 0, 4)
	if status != nil {
		networkIfaces = make([]models.GuestNetworkInterface, 0, len(status.Network))
		for rawName, cfg := range status.Network {
			if cfg == (proxmox.ContainerNetworkConfig{}) {
				continue
			}

			iface := models.GuestNetworkInterface{}
			name := strings.TrimSpace(cfg.Name)
			if name == "" {
				name = strings.TrimSpace(rawName)
			}
			if name != "" {
				iface.Name = name
			}
			if mac := strings.TrimSpace(cfg.HWAddr); mac != "" {
				iface.MAC = mac
			}

			addrCandidates := make([]string, 0, 4)
			addrCandidates = append(addrCandidates, collectIPsFromInterface(cfg.IP)...)
			addrCandidates = append(addrCandidates, collectIPsFromInterface(cfg.IP6)...)
			addrCandidates = append(addrCandidates, collectIPsFromInterface(cfg.IPv4)...)
			addrCandidates = append(addrCandidates, collectIPsFromInterface(cfg.IPv6)...)

			if len(addrCandidates) > 0 {
				deduped := dedupeStringsPreserveOrder(addrCandidates)
				if len(deduped) > 0 {
					iface.Addresses = deduped
					for _, addr := range deduped {
						addAddress(addr)
					}
				}
			}

			if iface.Name != "" || iface.MAC != "" || len(iface.Addresses) > 0 {
				networkIfaces = append(networkIfaces, iface)
			}
		}
	}

	configCtx, cancelConfig := context.WithTimeout(ctx, 5*time.Second)
	configData, configErr := client.GetContainerConfig(configCtx, nodeName, container.VMID)
	cancelConfig()
	if configErr != nil {
		log.Debug().
			Err(configErr).
			Str("instance", instanceName).
			Str("node", nodeName).
			Str("container", container.Name).
			Int("vmid", container.VMID).
			Msg("Container config metadata unavailable")
	} else if len(configData) > 0 {
		mountMetadata = parseContainerMountMetadata(configData)
		if rootDeviceHint == "" {
			if meta, ok := mountMetadata["rootfs"]; ok && meta.Source != "" {
				rootDeviceHint = meta.Source
			}
		}
		if rootDeviceHint == "" {
			if hint := extractContainerRootDeviceFromConfig(configData); hint != "" {
				rootDeviceHint = hint
			}
		}
		for _, detail := range parseContainerConfigNetworks(configData) {
			if len(detail.Addresses) > 0 {
				for _, addr := range detail.Addresses {
					addAddress(addr)
				}
			}
			mergeContainerNetworkInterface(&networkIfaces, detail)
		}
		// Extract OS type from container config
		if osName := extractContainerOSType(configData); osName != "" {
			container.OSName = osName
		}
		// Detect OCI containers (Proxmox VE 9.1+)
		// Method 1: Check ostemplate for OCI registry patterns
		if osTemplate := extractContainerOSTemplate(configData); osTemplate != "" {
			container.OSTemplate = osTemplate
			if isOCITemplate(osTemplate) {
				container.IsOCI = true
				container.Type = "oci"
				log.Debug().
					Str("container", container.Name).
					Int("vmid", container.VMID).
					Str("osTemplate", osTemplate).
					Msg("Detected OCI container by template")
			}
		}
		// Method 2: Check config fields (entrypoint, ostype, cmode)
		// This is needed because Proxmox doesn't persist ostemplate after creation
		if !container.IsOCI && isOCIContainerByConfig(configData) {
			container.IsOCI = true
			container.Type = "oci"
			log.Debug().
				Str("container", container.Name).
				Int("vmid", container.VMID).
				Msg("Detected OCI container by config (entrypoint/ostype)")
		}
	}

	if len(addressOrder) == 0 {
		if isRunning {
			interfacesCtx, cancelInterfaces := context.WithTimeout(ctx, 5*time.Second)
			ifaceDetails, ifaceErr := client.GetContainerInterfaces(interfacesCtx, nodeName, container.VMID)
			cancelInterfaces()
			if ifaceErr != nil {
				log.Debug().
					Err(ifaceErr).
					Str("instance", instanceName).
					Str("node", nodeName).
					Str("container", container.Name).
					Int("vmid", container.VMID).
					Msg("Container interface metadata unavailable")
			} else if len(ifaceDetails) > 0 {
				for _, detail := range ifaceDetails {
					parsed := containerNetworkDetails{}
					parsed.Name = strings.TrimSpace(detail.Name)
					parsed.MAC = strings.ToUpper(strings.TrimSpace(detail.HWAddr))

					for _, addr := range detail.IPAddresses {
						stripped := strings.TrimSpace(addr.Address)
						if stripped == "" {
							continue
						}
						if slash := strings.Index(stripped, "/"); slash > 0 {
							stripped = stripped[:slash]
						}
						parsed.Addresses = append(parsed.Addresses, sanitizeGuestAddressStrings(stripped)...)
					}

					if len(parsed.Addresses) == 0 && strings.TrimSpace(detail.Inet) != "" {
						parts := strings.Fields(detail.Inet)
						for _, part := range parts {
							stripped := strings.TrimSpace(part)
							if stripped == "" {
								continue
							}
							if slash := strings.Index(stripped, "/"); slash > 0 {
								stripped = stripped[:slash]
							}
							parsed.Addresses = append(parsed.Addresses, sanitizeGuestAddressStrings(stripped)...)
						}
					}

					parsed.Addresses = dedupeStringsPreserveOrder(parsed.Addresses)

					if len(parsed.Addresses) > 0 {
						for _, addr := range parsed.Addresses {
							addAddress(addr)
						}
					}

					if parsed.Name != "" || parsed.MAC != "" || len(parsed.Addresses) > 0 {
						mergeContainerNetworkInterface(&networkIfaces, parsed)
					}
				}
			}
		}
	}

	if len(networkIfaces) > 1 {
		sort.SliceStable(networkIfaces, func(i, j int) bool {
			left := strings.TrimSpace(networkIfaces[i].Name)
			right := strings.TrimSpace(networkIfaces[j].Name)
			return left < right
		})
	}

	if len(addressOrder) > 1 {
		sort.Strings(addressOrder)
	}

	if len(addressOrder) > 0 {
		container.IPAddresses = addressOrder
	}

	if len(networkIfaces) > 0 {
		container.NetworkInterfaces = networkIfaces
	}

	if disks := convertContainerDiskInfo(status, mountMetadata); len(disks) > 0 {
		container.Disks = disks
	}

	ensureContainerRootDiskEntry(container)

	if rootDeviceHint != "" && len(container.Disks) > 0 {
		for i := range container.Disks {
			if container.Disks[i].Mountpoint == "/" && container.Disks[i].Device == "" {
				container.Disks[i].Device = rootDeviceHint
			}
		}
	}
}

// GetConnectionStatuses returns the current connection status for all nodes
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

// HasSocketTemperatureProxy reports whether the local unix socket proxy is available.
func (m *Monitor) HasSocketTemperatureProxy() bool {
	// Always check the real socket path first so we reflect the actual runtime state
	// even if the temperature collector hasn't latched onto the proxy yet.
	if tempproxy.NewClient().IsAvailable() {
		return true
	}

	if m == nil {
		return false
	}

	m.mu.RLock()
	collector := m.tempCollector
	m.mu.RUnlock()

	if collector == nil {
		return false
	}
	return collector.SocketProxyDetected()
}

// SocketProxyHostDiagnostics exposes per-host proxy cooldown state for diagnostics.
func (m *Monitor) SocketProxyHostDiagnostics() []ProxyHostDiagnostics {
	m.mu.RLock()
	collector := m.tempCollector
	m.mu.RUnlock()

	if collector == nil {
		return nil
	}

	return collector.ProxyHostDiagnostics()
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

	if cfg.TemperatureMonitoringEnabled {
		isContainer := os.Getenv("PULSE_DOCKER") == "true" || system.InContainer()
		if isContainer && tempCollector != nil && !tempCollector.SocketProxyAvailable() {
			log.Warn().Msg("Temperature monitoring is enabled but the container does not have access to pulse-sensor-proxy. Install the proxy on the host or disable temperatures until it is available.")
		}
	}

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
	ms, err := metrics.NewStore(metricsStoreConfig)
	if err != nil {
		log.Error().Err(err).Msg("Failed to initialize persistent metrics store - continuing with in-memory only")
	} else {
		metricsStore = ms
		log.Info().
			Str("path", metricsStoreConfig.DBPath).
			Dur("retentionRaw", metricsStoreConfig.RetentionRaw).
			Dur("retentionMinute", metricsStoreConfig.RetentionMinute).
			Dur("retentionHourly", metricsStoreConfig.RetentionHourly).
			Dur("retentionDaily", metricsStoreConfig.RetentionDaily).
			Msg("Persistent metrics store initialized with configurable retention")
	}

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
		guestMetadataStore:         config.NewGuestMetadataStore(cfg.DataPath),
		dockerMetadataStore:        config.NewDockerMetadataStore(cfg.DataPath),
		startTime:                  time.Now(),
		rateTracker:                NewRateTracker(),
		metricsHistory:             NewMetricsHistory(1000, 24*time.Hour), // Keep up to 1000 points or 24 hours
		metricsStore:               metricsStore,                          // Persistent SQLite storage
		alertManager:               alerts.NewManager(),
		notificationMgr:            notifications.NewNotificationManager(cfg.PublicURL),
		configPersist:              config.NewConfigPersistence(cfg.DataPath),
		discoveryService:           nil, // Will be initialized in Start()
		authFailures:               make(map[string]int),
		lastAuthAttempt:            make(map[string]time.Time),
		lastClusterCheck:           make(map[string]time.Time),
		lastPhysicalDiskPoll:       make(map[string]time.Time),
		lastPVEBackupPoll:          make(map[string]time.Time),
		lastPBSBackupPoll:          make(map[string]time.Time),
		persistence:                config.NewConfigPersistence(cfg.DataPath),
		pbsBackupPollers:           make(map[string]bool),
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

	// Check if mock mode is enabled before initializing clients
	mockEnabled := mock.IsMockEnabled()

	if mockEnabled {
		log.Info().Msg("Mock mode enabled - skipping PVE/PBS client initialization")
	} else {
		// Initialize PVE clients
		log.Info().Int("count", len(cfg.PVEInstances)).Msg("Initializing PVE clients")
		for _, pve := range cfg.PVEInstances {
			log.Info().
				Str("name", pve.Name).
				Str("host", pve.Host).
				Str("user", pve.User).
				Bool("hasToken", pve.TokenName != "").
				Msg("Configuring PVE instance")

				// Check if this is a cluster
			if pve.IsCluster && len(pve.ClusterEndpoints) > 0 {
				// For clusters, check if endpoints have IPs/resolvable hosts
				// If not, use the main host for all connections (Proxmox will route cluster API calls)
				hasValidEndpoints := false
				endpoints := make([]string, 0, len(pve.ClusterEndpoints))

				for _, ep := range pve.ClusterEndpoints {
					hasFingerprint := pve.Fingerprint != ""
					effectiveURL := clusterEndpointEffectiveURL(ep, pve.VerifySSL, hasFingerprint)
					if effectiveURL == "" {
						log.Warn().
							Str("node", ep.NodeName).
							Msg("Skipping cluster endpoint with no host/IP")
						continue
					}

					if parsed, err := url.Parse(effectiveURL); err == nil {
						hostname := parsed.Hostname()
						if hostname != "" && (strings.Contains(hostname, ".") || net.ParseIP(hostname) != nil) {
							hasValidEndpoints = true
						}
					} else {
						hostname := normalizeEndpointHost(effectiveURL)
						if hostname != "" && (strings.Contains(hostname, ".") || net.ParseIP(hostname) != nil) {
							hasValidEndpoints = true
						}
					}

					endpoints = append(endpoints, effectiveURL)
				}

				// If endpoints are just node names (not FQDNs or IPs), use main host only
				// This is common when cluster nodes are discovered but not directly reachable
				if !hasValidEndpoints || len(endpoints) == 0 {
					log.Info().
						Str("instance", pve.Name).
						Str("mainHost", pve.Host).
						Msg("Cluster endpoints are not resolvable, using main host for all cluster operations")
					fallback := ensureClusterEndpointURL(pve.Host)
					if fallback == "" {
						fallback = ensureClusterEndpointURL(pve.Host)
					}
					endpoints = []string{fallback}
				}

				log.Info().
					Str("cluster", pve.ClusterName).
					Strs("endpoints", endpoints).
					Msg("Creating cluster-aware client")

				clientConfig := config.CreateProxmoxConfig(&pve)
				clientConfig.Timeout = cfg.ConnectionTimeout
				clusterClient := proxmox.NewClusterClient(
					pve.Name,
					clientConfig,
					endpoints,
				)
				m.pveClients[pve.Name] = clusterClient
				log.Info().
					Str("instance", pve.Name).
					Str("cluster", pve.ClusterName).
					Int("endpoints", len(endpoints)).
					Msg("Cluster client created successfully")
				// Set initial connection health to true for cluster
				m.state.SetConnectionHealth(pve.Name, true)
			} else {
				// Create regular client
				clientConfig := config.CreateProxmoxConfig(&pve)
				clientConfig.Timeout = cfg.ConnectionTimeout
				client, err := proxmox.NewClient(clientConfig)
				if err != nil {
					monErr := errors.WrapConnectionError("create_pve_client", pve.Name, err)
					log.Error().
						Err(monErr).
						Str("instance", pve.Name).
						Str("host", pve.Host).
						Str("user", pve.User).
						Bool("hasPassword", pve.Password != "").
						Bool("hasToken", pve.TokenValue != "").
						Msg("Failed to create PVE client - node will show as disconnected")
					// Set initial connection health to false for this node
					m.state.SetConnectionHealth(pve.Name, false)
					continue
				}
				m.pveClients[pve.Name] = client
				log.Info().Str("instance", pve.Name).Msg("PVE client created successfully")
				// Set initial connection health to true
				m.state.SetConnectionHealth(pve.Name, true)
			}
		}

		// Initialize PBS clients
		log.Info().Int("count", len(cfg.PBSInstances)).Msg("Initializing PBS clients")
		for _, pbsInst := range cfg.PBSInstances {
			log.Info().
				Str("name", pbsInst.Name).
				Str("host", pbsInst.Host).
				Str("user", pbsInst.User).
				Bool("hasToken", pbsInst.TokenName != "").
				Msg("Configuring PBS instance")

			clientConfig := config.CreatePBSConfig(&pbsInst)
			clientConfig.Timeout = 60 * time.Second // Very generous timeout for slow PBS servers
			client, err := pbs.NewClient(clientConfig)
			if err != nil {
				monErr := errors.WrapConnectionError("create_pbs_client", pbsInst.Name, err)
				log.Error().
					Err(monErr).
					Str("instance", pbsInst.Name).
					Str("host", pbsInst.Host).
					Str("user", pbsInst.User).
					Bool("hasPassword", pbsInst.Password != "").
					Bool("hasToken", pbsInst.TokenValue != "").
					Msg("Failed to create PBS client - node will show as disconnected")
				// Set initial connection health to false for this node
				m.state.SetConnectionHealth("pbs-"+pbsInst.Name, false)
				continue
			}
			m.pbsClients[pbsInst.Name] = client
			log.Info().Str("instance", pbsInst.Name).Msg("PBS client created successfully")
			// Set initial connection health to true
			m.state.SetConnectionHealth("pbs-"+pbsInst.Name, true)
		}

		// Initialize PMG clients
		log.Info().Int("count", len(cfg.PMGInstances)).Msg("Initializing PMG clients")
		for _, pmgInst := range cfg.PMGInstances {
			log.Info().
				Str("name", pmgInst.Name).
				Str("host", pmgInst.Host).
				Str("user", pmgInst.User).
				Bool("hasToken", pmgInst.TokenName != "").
				Msg("Configuring PMG instance")

			clientConfig := config.CreatePMGConfig(&pmgInst)
			if clientConfig.Timeout <= 0 {
				clientConfig.Timeout = 45 * time.Second
			}

			client, err := pmg.NewClient(clientConfig)
			if err != nil {
				monErr := errors.WrapConnectionError("create_pmg_client", pmgInst.Name, err)
				log.Error().
					Err(monErr).
					Str("instance", pmgInst.Name).
					Str("host", pmgInst.Host).
					Str("user", pmgInst.User).
					Bool("hasPassword", pmgInst.Password != "").
					Bool("hasToken", pmgInst.TokenValue != "").
					Msg("Failed to create PMG client - gateway will show as disconnected")
				m.state.SetConnectionHealth("pmg-"+pmgInst.Name, false)
				continue
			}

			m.pmgClients[pmgInst.Name] = client
			log.Info().Str("instance", pmgInst.Name).Msg("PMG client created successfully")
			m.state.SetConnectionHealth("pmg-"+pmgInst.Name, true)
		}
	} // End of else block for mock mode check

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

// getConfiguredHostIPs returns a list of IP addresses from all configured Proxmox hosts.
// This is used to prevent discovery from probing hosts we already know about.
// Caller must hold m.mu.RLock or m.mu.Lock.
func (m *Monitor) getConfiguredHostIPs() []string {
	if m.config == nil {
		return nil
	}

	seen := make(map[string]struct{})
	var ips []string

	addHost := func(host string) {
		// Parse the host to extract IP/hostname
		host = strings.TrimSpace(host)
		if host == "" {
			return
		}
		// Remove scheme if present
		if strings.HasPrefix(host, "https://") {
			host = strings.TrimPrefix(host, "https://")
		} else if strings.HasPrefix(host, "http://") {
			host = strings.TrimPrefix(host, "http://")
		}
		// Remove port if present
		if colonIdx := strings.LastIndex(host, ":"); colonIdx != -1 {
			// Check if it's an IPv6 address
			if !strings.Contains(host[colonIdx:], "]") {
				host = host[:colonIdx]
			}
		}
		// Remove trailing path
		if slashIdx := strings.Index(host, "/"); slashIdx != -1 {
			host = host[:slashIdx]
		}
		host = strings.TrimSpace(host)
		if host == "" {
			return
		}
		// Check if it's already an IP
		if ip := net.ParseIP(host); ip != nil {
			if _, exists := seen[host]; !exists {
				seen[host] = struct{}{}
				ips = append(ips, host)
			}
			return
		}
		// Try to resolve hostname to IP
		if addrs, err := net.LookupIP(host); err == nil && len(addrs) > 0 {
			for _, addr := range addrs {
				// Prefer IPv4
				if v4 := addr.To4(); v4 != nil {
					ipStr := v4.String()
					if _, exists := seen[ipStr]; !exists {
						seen[ipStr] = struct{}{}
						ips = append(ips, ipStr)
					}
					break
				}
			}
		}
	}

	// Add PVE hosts
	for _, pve := range m.config.PVEInstances {
		addHost(pve.Host)
		// Also add cluster endpoints
		for _, ep := range pve.ClusterEndpoints {
			addHost(ep.Host)
			addHost(ep.IP)
		}
	}

	// Add PBS hosts
	for _, pbs := range m.config.PBSInstances {
		addHost(pbs.Host)
	}

	// Add PMG hosts
	for _, pmg := range m.config.PMGInstances {
		addHost(pmg.Host)
	}

	return ips
}

// Start begins the monitoring loop
func (m *Monitor) Start(ctx context.Context, wsHub *websocket.Hub) {
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
		wsHub.BroadcastAlert(alert)
		// Send notifications
		log.Debug().
			Str("alertID", alert.ID).
			Str("level", string(alert.Level)).
			Msg("Alert raised, sending to notification manager")
		go m.notificationMgr.SendAlert(alert)
	})
	m.alertManager.SetResolvedCallback(func(alertID string) {
		wsHub.BroadcastAlertResolved(alertID)
		m.notificationMgr.CancelAlert(alertID)
		if m.notificationMgr.GetNotifyOnResolve() {
			if resolved := m.alertManager.GetResolvedAlert(alertID); resolved != nil {
				go m.notificationMgr.SendResolvedAlert(resolved)
			}
		}
		// Don't broadcast full state here - it causes a cascade with many guests.
		// The frontend will get the updated alerts through the regular broadcast ticker.
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

	// Start connection retry mechanism for failed clients
	// This handles cases where network/Proxmox isn't ready on initial startup
	if !mock.IsMockEnabled() {
		go m.retryFailedConnections(ctx)
	}

	// Do an immediate poll on start (only if not in mock mode)
	if mock.IsMockEnabled() {
		log.Info().Msg("Mock mode enabled - skipping real node polling")
		go m.checkMockAlerts()
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
			if mock.IsMockEnabled() {
				// In mock mode, keep synthetic alerts fresh
				go m.checkMockAlerts()
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
			wsHub.BroadcastState(frontendState)

		case <-ctx.Done():
			log.Info().Msg("Monitoring loop stopped")
			return
		}
	}
}

// retryFailedConnections attempts to recreate clients that failed during initialization
// This handles cases where Proxmox/network isn't ready when Pulse starts
func (m *Monitor) retryFailedConnections(ctx context.Context) {
	defer recoverFromPanic("retryFailedConnections")

	// Retry schedule: 5s, 10s, 20s, 40s, 60s, then every 60s for up to 5 minutes total
	retryDelays := []time.Duration{
		5 * time.Second,
		10 * time.Second,
		20 * time.Second,
		40 * time.Second,
		60 * time.Second,
	}

	maxRetryDuration := 5 * time.Minute
	startTime := time.Now()
	retryIndex := 0

	for {
		// Stop retrying after max duration or if context is cancelled
		select {
		case <-ctx.Done():
			return
		default:
		}

		if time.Since(startTime) > maxRetryDuration {
			log.Info().Msg("Connection retry period expired")
			return
		}

		// Calculate next retry delay
		var delay time.Duration
		if retryIndex < len(retryDelays) {
			delay = retryDelays[retryIndex]
			retryIndex++
		} else {
			delay = 60 * time.Second // Continue retrying every 60s
		}

		// Wait before retry
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return
		}

		// Check for missing clients and try to recreate them
		m.mu.Lock()
		missingPVE := []config.PVEInstance{}
		missingPBS := []config.PBSInstance{}

		// Find PVE instances without clients
		for _, pve := range m.config.PVEInstances {
			if _, exists := m.pveClients[pve.Name]; !exists {
				missingPVE = append(missingPVE, pve)
			}
		}

		// Find PBS instances without clients
		for _, pbs := range m.config.PBSInstances {
			if _, exists := m.pbsClients[pbs.Name]; !exists {
				missingPBS = append(missingPBS, pbs)
			}
		}
		m.mu.Unlock()

		// If no missing clients, we're done
		if len(missingPVE) == 0 && len(missingPBS) == 0 {
			log.Info().Msg("All client connections established successfully")
			return
		}

		log.Info().
			Int("missingPVE", len(missingPVE)).
			Int("missingPBS", len(missingPBS)).
			Dur("nextRetry", delay).
			Msg("Attempting to reconnect failed clients")

		// Try to recreate PVE clients
		for _, pve := range missingPVE {
			if pve.IsCluster && len(pve.ClusterEndpoints) > 0 {
				// Create cluster client
				hasValidEndpoints := false
				endpoints := make([]string, 0, len(pve.ClusterEndpoints))

				for _, ep := range pve.ClusterEndpoints {
					host := ep.IP
					if host == "" {
						host = ep.Host
					}
					if host == "" {
						continue
					}
					if strings.Contains(host, ".") || net.ParseIP(host) != nil {
						hasValidEndpoints = true
					}
					if !strings.HasPrefix(host, "http") {
						host = fmt.Sprintf("https://%s:8006", host)
					}
					endpoints = append(endpoints, host)
				}

				if !hasValidEndpoints || len(endpoints) == 0 {
					endpoints = []string{pve.Host}
					if !strings.HasPrefix(endpoints[0], "http") {
						endpoints[0] = fmt.Sprintf("https://%s:8006", endpoints[0])
					}
				}

				clientConfig := config.CreateProxmoxConfig(&pve)
				clientConfig.Timeout = m.config.ConnectionTimeout
				clusterClient := proxmox.NewClusterClient(pve.Name, clientConfig, endpoints)

				m.mu.Lock()
				m.pveClients[pve.Name] = clusterClient
				m.state.SetConnectionHealth(pve.Name, true)
				m.mu.Unlock()

				log.Info().
					Str("instance", pve.Name).
					Str("cluster", pve.ClusterName).
					Msg("Successfully reconnected cluster client")
			} else {
				// Create regular client
				clientConfig := config.CreateProxmoxConfig(&pve)
				clientConfig.Timeout = m.config.ConnectionTimeout
				client, err := proxmox.NewClient(clientConfig)
				if err != nil {
					log.Warn().
						Err(err).
						Str("instance", pve.Name).
						Msg("Failed to reconnect PVE client, will retry")
					continue
				}

				m.mu.Lock()
				m.pveClients[pve.Name] = client
				m.state.SetConnectionHealth(pve.Name, true)
				m.mu.Unlock()

				log.Info().
					Str("instance", pve.Name).
					Msg("Successfully reconnected PVE client")
			}
		}

		// Try to recreate PBS clients
		for _, pbsInst := range missingPBS {
			clientConfig := config.CreatePBSConfig(&pbsInst)
			clientConfig.Timeout = 60 * time.Second
			client, err := pbs.NewClient(clientConfig)
			if err != nil {
				log.Warn().
					Err(err).
					Str("instance", pbsInst.Name).
					Msg("Failed to reconnect PBS client, will retry")
				continue
			}

			m.mu.Lock()
			m.pbsClients[pbsInst.Name] = client
			m.state.SetConnectionHealth("pbs-"+pbsInst.Name, true)
			m.mu.Unlock()

			log.Info().
				Str("instance", pbsInst.Name).
				Msg("Successfully reconnected PBS client")
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
	m.state.Stats.WebSocketClients = wsHub.GetClientCount()

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

// syncAlertsToState copies the latest alert manager data into the shared state snapshot.
// This keeps WebSocket broadcasts aligned with in-memory acknowledgement updates.
func (m *Monitor) syncAlertsToState() {
	if m.pruneStaleDockerAlerts() {
		if logging.IsLevelEnabled(zerolog.DebugLevel) {
			log.Debug().Msg("Pruned stale docker alerts during sync")
		}
	}

	activeAlerts := m.alertManager.GetActiveAlerts()
	modelAlerts := make([]models.Alert, 0, len(activeAlerts))
	for _, alert := range activeAlerts {
		modelAlerts = append(modelAlerts, models.Alert{
			ID:           alert.ID,
			Type:         alert.Type,
			Level:        string(alert.Level),
			ResourceID:   alert.ResourceID,
			ResourceName: alert.ResourceName,
			Node:         alert.Node,
			Instance:     alert.Instance,
			Message:      alert.Message,
			Value:        alert.Value,
			Threshold:    alert.Threshold,
			StartTime:    alert.StartTime,
			Acknowledged: alert.Acknowledged,
			AckTime:      alert.AckTime,
			AckUser:      alert.AckUser,
		})
		if alert.Acknowledged && logging.IsLevelEnabled(zerolog.DebugLevel) {
			log.Debug().Str("alertID", alert.ID).Interface("ackTime", alert.AckTime).Msg("Syncing acknowledged alert")
		}
	}
	m.state.UpdateActiveAlerts(modelAlerts)

	recentlyResolved := m.alertManager.GetRecentlyResolved()
	if len(recentlyResolved) > 0 {
		log.Info().Int("count", len(recentlyResolved)).Msg("Syncing recently resolved alerts")
	}
	m.state.UpdateRecentlyResolved(recentlyResolved)
}

// SyncAlertState is the exported wrapper used by APIs that mutate alerts outside the poll loop.
func (m *Monitor) SyncAlertState() {
	m.syncAlertsToState()
}

// pruneStaleDockerAlerts removes docker alerts that reference hosts no longer present in state.
func (m *Monitor) pruneStaleDockerAlerts() bool {
	if m.alertManager == nil {
		return false
	}

	hosts := m.state.GetDockerHosts()
	knownHosts := make(map[string]struct{}, len(hosts))
	for _, host := range hosts {
		id := strings.TrimSpace(host.ID)
		if id != "" {
			knownHosts[id] = struct{}{}
		}
	}

	if len(knownHosts) == 0 {
		// Still allow stale entries to be cleared if no hosts remain.
	}

	active := m.alertManager.GetActiveAlerts()
	processed := make(map[string]struct{})
	cleared := false

	for _, alert := range active {
		var hostID string

		switch {
		case alert.Type == "docker-host-offline":
			hostID = strings.TrimPrefix(alert.ID, "docker-host-offline-")
		case strings.HasPrefix(alert.ResourceID, "docker:"):
			resource := strings.TrimPrefix(alert.ResourceID, "docker:")
			if idx := strings.Index(resource, "/"); idx >= 0 {
				hostID = resource[:idx]
			} else {
				hostID = resource
			}
		default:
			continue
		}

		hostID = strings.TrimSpace(hostID)
		if hostID == "" {
			continue
		}

		if _, known := knownHosts[hostID]; known {
			continue
		}
		if _, alreadyCleared := processed[hostID]; alreadyCleared {
			continue
		}

		host := models.DockerHost{
			ID:          hostID,
			DisplayName: alert.ResourceName,
			Hostname:    alert.Node,
		}
		if host.DisplayName == "" {
			host.DisplayName = hostID
		}
		if host.Hostname == "" {
			host.Hostname = hostID
		}

		m.alertManager.HandleDockerHostRemoved(host)
		processed[hostID] = struct{}{}
		cleared = true
	}

	return cleared
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
	if timeout > maxTaskTimeout {
		timeout = maxTaskTimeout
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

			instances = append(instances, InstanceHealth{
				Key:         key,
				Type:        instType,
				DisplayName: display,
				Instance:    instName,
				Connection:  connection,
				PollStatus:  instanceStatus,
				Breaker:     breakerInfo,
				DeadLetter:  dlqInfo,
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

func shouldTryPortlessFallback(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "no such host") ||
		strings.Contains(msg, "client.timeout exceeded") ||
		strings.Contains(msg, "i/o timeout") ||
		strings.Contains(msg, "context deadline exceeded") {
		return true
	}
	return false
}

// retryPVEPortFallback handles the case where a normalized :8006 host is unreachable
// because the actual endpoint is fronted by a reverse proxy on 443. If the initial
// GetNodes call fails with a connection error and the host has the default PVE port,
// retry without the default port to hit the proxy. On success, swap the client so
// subsequent polls reuse the working endpoint.
func (m *Monitor) retryPVEPortFallback(ctx context.Context, instanceName string, instanceCfg *config.PVEInstance, currentClient PVEClientInterface, cause error) ([]proxmox.Node, PVEClientInterface, error) {
	if instanceCfg == nil || !shouldTryPortlessFallback(cause) {
		return nil, currentClient, cause
	}

	fallbackHost := config.StripDefaultPort(instanceCfg.Host, config.DefaultPVEPort)
	if fallbackHost == "" || fallbackHost == instanceCfg.Host {
		return nil, currentClient, cause
	}

	clientCfg := config.CreateProxmoxConfigWithHost(instanceCfg, fallbackHost, false)
	if clientCfg.Timeout <= 0 {
		clientCfg.Timeout = m.config.ConnectionTimeout
	}

	fallbackClient, err := proxmox.NewClient(clientCfg)
	if err != nil {
		return nil, currentClient, cause
	}

	fallbackNodes, err := fallbackClient.GetNodes(ctx)
	if err != nil {
		return nil, currentClient, cause
	}

	// Switch to the working host for the remainder of the poll (and future polls)
	primaryHost := instanceCfg.Host

	// Persist with an explicit port to avoid re-normalization back to :8006 on reloads.
	persistHost := fallbackHost
	if parsed, err := url.Parse(fallbackHost); err == nil && parsed.Host != "" && parsed.Port() == "" {
		port := "443"
		if strings.EqualFold(parsed.Scheme, "http") {
			port = "80"
		}
		parsed.Host = net.JoinHostPort(parsed.Hostname(), port)
		persistHost = parsed.Scheme + "://" + parsed.Host
	}

	instanceCfg.Host = persistHost
	m.pveClients[instanceName] = fallbackClient

	// Update in-memory config so subsequent polls build clients against the working port.
	for i := range m.config.PVEInstances {
		if m.config.PVEInstances[i].Name == instanceName {
			m.config.PVEInstances[i].Host = persistHost
			break
		}
	}

	// Persist to disk so restarts keep the working endpoint.
	if m.persistence != nil {
		if err := m.persistence.SaveNodesConfig(m.config.PVEInstances, m.config.PBSInstances, m.config.PMGInstances); err != nil {
			log.Warn().Err(err).Str("instance", instanceName).Msg("Failed to persist fallback PVE host")
		}
	}

	log.Warn().
		Str("instance", instanceName).
		Str("primary", primaryHost).
		Str("fallback", persistHost).
		Msg("Primary PVE host failed; using fallback without default port")

	return fallbackNodes, fallbackClient, nil
}

// pollPVEInstance polls a single PVE instance
func (m *Monitor) pollPVEInstance(ctx context.Context, instanceName string, client PVEClientInterface) {
	defer recoverFromPanic(fmt.Sprintf("pollPVEInstance-%s", instanceName))

	start := time.Now()
	debugEnabled := logging.IsLevelEnabled(zerolog.DebugLevel)
	var pollErr error
	if m.pollMetrics != nil {
		m.pollMetrics.IncInFlight("pve")
		defer m.pollMetrics.DecInFlight("pve")
		defer func() {
			m.pollMetrics.RecordResult(PollResult{
				InstanceName: instanceName,
				InstanceType: "pve",
				Success:      pollErr == nil,
				Error:        pollErr,
				StartTime:    start,
				EndTime:      time.Now(),
			})
		}()
	}
	if m.stalenessTracker != nil {
		defer func() {
			if pollErr == nil {
				m.stalenessTracker.UpdateSuccess(InstanceTypePVE, instanceName, nil)
			} else {
				m.stalenessTracker.UpdateError(InstanceTypePVE, instanceName)
			}
		}()
	}
	defer m.recordTaskResult(InstanceTypePVE, instanceName, pollErr)

	// Check if context is cancelled
	select {
	case <-ctx.Done():
		pollErr = ctx.Err()
		if debugEnabled {
			log.Debug().Str("instance", instanceName).Msg("Polling cancelled")
		}
		return
	default:
	}

	if debugEnabled {
		log.Debug().Str("instance", instanceName).Msg("Polling PVE instance")
	}

	// Get instance config
	var instanceCfg *config.PVEInstance
	for _, cfg := range m.config.PVEInstances {
		if cfg.Name == instanceName {
			instanceCfg = &cfg
			break
		}
	}
	if instanceCfg == nil {
		pollErr = fmt.Errorf("pve instance config not found for %s", instanceName)
		return
	}

	// Poll nodes
	nodes, err := client.GetNodes(ctx)
	if err != nil {
		if fallbackNodes, fallbackClient, fallbackErr := m.retryPVEPortFallback(ctx, instanceName, instanceCfg, client, err); fallbackErr == nil {
			client = fallbackClient
			nodes = fallbackNodes
		} else {
			monErr := errors.WrapConnectionError("poll_nodes", instanceName, err)
			pollErr = monErr
			log.Error().Err(monErr).Str("instance", instanceName).Msg("Failed to get nodes")
			m.state.SetConnectionHealth(instanceName, false)

			// Track auth failure if it's an authentication error
			if errors.IsAuthError(err) {
				m.recordAuthFailure(instanceName, "pve")
			}
			return
		}
	}

	// Reset auth failures on successful connection
	m.resetAuthFailures(instanceName, "pve")

	// Check if client is a ClusterClient to determine health status
	connectionHealthStr := "healthy"
	if clusterClient, ok := client.(*proxmox.ClusterClient); ok {
		// For cluster clients, check if all endpoints are healthy
		healthStatus := clusterClient.GetHealthStatus()
		healthyCount := 0
		totalCount := len(healthStatus)

		for _, isHealthy := range healthStatus {
			if isHealthy {
				healthyCount++
			}
		}

		if healthyCount == 0 {
			// All endpoints are down
			connectionHealthStr = "error"
			m.state.SetConnectionHealth(instanceName, false)
		} else if healthyCount < totalCount {
			// Some endpoints are down - degraded state
			connectionHealthStr = "degraded"
			m.state.SetConnectionHealth(instanceName, true) // Still functional but degraded
			log.Warn().
				Str("instance", instanceName).
				Int("healthy", healthyCount).
				Int("total", totalCount).
				Msg("Cluster is in degraded state - some nodes are unreachable")
		} else {
			// All endpoints are healthy
			connectionHealthStr = "healthy"
			m.state.SetConnectionHealth(instanceName, true)
		}
	} else {
		// Regular client - simple healthy/unhealthy
		m.state.SetConnectionHealth(instanceName, true)
	}

	// Capture previous memory metrics so we can preserve them if detailed status fails
	prevState := m.GetState()
	prevNodeMemory := make(map[string]models.Memory)
	prevInstanceNodes := make([]models.Node, 0)
	for _, existingNode := range prevState.Nodes {
		if existingNode.Instance != instanceName {
			continue
		}
		prevNodeMemory[existingNode.ID] = existingNode.Memory
		prevInstanceNodes = append(prevInstanceNodes, existingNode)
	}

	// Convert to models
	var modelNodes []models.Node
	nodeEffectiveStatus := make(map[string]string) // Track effective status (with grace period) for each node
	// Parallel node polling
	type nodePollResult struct {
		node            models.Node
		effectiveStatus string
	}

	resultChan := make(chan nodePollResult, len(nodes))
	var wg sync.WaitGroup

	if debugEnabled {
		log.Debug().
			Str("instance", instanceName).
			Int("nodes", len(nodes)).
			Msg("Starting parallel node polling")
	}

	for _, node := range nodes {
		wg.Add(1)
		go func(node proxmox.Node) {
			defer wg.Done()

			modelNode, effectiveStatus, _ := m.pollPVENode(ctx, instanceName, instanceCfg, client, node, connectionHealthStr, prevNodeMemory, prevInstanceNodes)

			resultChan <- nodePollResult{
				node:            modelNode,
				effectiveStatus: effectiveStatus,
			}
		}(node)
	}

	wg.Wait()
	close(resultChan)

	for res := range resultChan {
		modelNodes = append(modelNodes, res.node)
		nodeEffectiveStatus[res.node.Name] = res.effectiveStatus
	}

	if len(modelNodes) == 0 && len(prevInstanceNodes) > 0 {
		log.Warn().
			Str("instance", instanceName).
			Int("previousCount", len(prevInstanceNodes)).
			Msg("No Proxmox nodes returned this cycle - preserving previous state")

		// Mark connection health as degraded to reflect polling failure
		m.state.SetConnectionHealth(instanceName, false)

		preserved := make([]models.Node, 0, len(prevInstanceNodes))
		for _, prevNode := range prevInstanceNodes {
			nodeCopy := prevNode
			nodeCopy.Status = "offline"
			nodeCopy.ConnectionHealth = "error"
			nodeCopy.Uptime = 0
			nodeCopy.CPU = 0
			preserved = append(preserved, nodeCopy)
		}
		modelNodes = preserved
	}

	// Update state first so we have nodes available
	m.state.UpdateNodesForInstance(instanceName, modelNodes)

	// Storage fallback is used to provide disk metrics when rootfs is not available.
	// We run this asynchronously with a short timeout so it doesn't block VM/container polling.
	// This addresses the issue where slow storage APIs (e.g., NFS mounts) can cause the entire
	// polling task to timeout before reaching VM/container polling.
	storageByNode := make(map[string]models.Disk)
	var storageByNodeMu sync.Mutex
	storageFallbackDone := make(chan struct{})

	if instanceCfg.MonitorStorage {
		go func() {
			defer close(storageFallbackDone)

			// Use a short timeout for storage fallback - it's an optimization, not critical
			storageFallbackTimeout := 10 * time.Second
			storageCtx, storageCancel := context.WithTimeout(context.Background(), storageFallbackTimeout)
			defer storageCancel()

			_, err := client.GetAllStorage(storageCtx)
			if err != nil {
				if storageCtx.Err() != nil {
					log.Debug().
						Str("instance", instanceName).
						Dur("timeout", storageFallbackTimeout).
						Msg("Storage fallback timed out - continuing without disk fallback data")
				}
				return
			}

			for _, node := range nodes {
				// Check if context was cancelled
				select {
				case <-storageCtx.Done():
					log.Debug().
						Str("instance", instanceName).
						Msg("Storage fallback cancelled - partial data collected")
					return
				default:
				}

				// Skip offline nodes to avoid 595 errors
				if nodeEffectiveStatus[node.Node] != "online" {
					continue
				}

				nodeStorages, err := client.GetStorage(storageCtx, node.Node)
				if err != nil {
					continue
				}

				// Look for local or local-lvm storage as most stable disk metric
				for _, storage := range nodeStorages {
					if reason, skip := readOnlyFilesystemReason(storage.Type, storage.Total, storage.Used); skip {
						log.Debug().
							Str("node", node.Node).
							Str("storage", storage.Storage).
							Str("type", storage.Type).
							Str("skipReason", reason).
							Uint64("total", storage.Total).
							Uint64("used", storage.Used).
							Msg("Skipping read-only storage while building disk fallback")
						continue
					}
					if storage.Storage == "local" || storage.Storage == "local-lvm" {
						disk := models.Disk{
							Total: int64(storage.Total),
							Used:  int64(storage.Used),
							Free:  int64(storage.Available),
							Usage: safePercentage(float64(storage.Used), float64(storage.Total)),
						}
						// Prefer "local" over "local-lvm"
						storageByNodeMu.Lock()
						if _, exists := storageByNode[node.Node]; !exists || storage.Storage == "local" {
							storageByNode[node.Node] = disk
							log.Debug().
								Str("node", node.Node).
								Str("storage", storage.Storage).
								Float64("usage", disk.Usage).
								Msg("Using storage for disk metrics fallback")
						}
						storageByNodeMu.Unlock()
					}
				}
			}
		}()
	} else {
		// No storage monitoring, close channel immediately
		close(storageFallbackDone)
	}

	// Poll VMs and containers FIRST - this is the most critical data.
	// This happens immediately after starting the storage fallback goroutine,
	// so VM/container polling runs in parallel with (and is not blocked by) storage operations.
	if instanceCfg.MonitorVMs || instanceCfg.MonitorContainers {
		select {
		case <-ctx.Done():
			pollErr = ctx.Err()
			return
		default:
			// Always try the efficient cluster/resources endpoint first
			// This endpoint works on both clustered and standalone nodes
			// Testing confirmed it works on standalone nodes like pimox
			useClusterEndpoint := m.pollVMsAndContainersEfficient(ctx, instanceName, client, nodeEffectiveStatus)

			if !useClusterEndpoint {
				// Fall back to traditional polling only if cluster/resources not available
				// This should be rare - only for very old Proxmox versions
				log.Debug().
					Str("instance", instanceName).
					Msg("cluster/resources endpoint not available, using traditional polling")

				// Check if configuration needs updating
				if instanceCfg.IsCluster {
					isActuallyCluster, checkErr := client.IsClusterMember(ctx)
					if checkErr == nil && !isActuallyCluster {
						log.Warn().
							Str("instance", instanceName).
							Msg("Instance marked as cluster but is actually standalone - consider updating configuration")
						instanceCfg.IsCluster = false
					}
				}

				// Use optimized parallel polling for better performance
				if instanceCfg.MonitorVMs {
					m.pollVMsWithNodes(ctx, instanceName, client, nodes, nodeEffectiveStatus)
				}
				if instanceCfg.MonitorContainers {
					m.pollContainersWithNodes(ctx, instanceName, client, nodes, nodeEffectiveStatus)
				}
			}
		}
	}

	// Poll physical disks for health monitoring (enabled by default unless explicitly disabled)
	// Skip if MonitorPhysicalDisks is explicitly set to false
	// Physical disk polling runs in a background goroutine since GetDisks can be slow
	// and we don't want it to cause task timeouts. It has its own 5-minute interval anyway.
	if instanceCfg.MonitorPhysicalDisks != nil && !*instanceCfg.MonitorPhysicalDisks {
		log.Debug().Str("instance", instanceName).Msg("Physical disk monitoring explicitly disabled")
		// Keep any existing disk data visible (don't clear it)
	} else {
		// Enabled by default (when nil or true)
		// Determine polling interval (default 5 minutes to avoid spinning up HDDs too frequently)
		pollingInterval := 5 * time.Minute
		if instanceCfg.PhysicalDiskPollingMinutes > 0 {
			pollingInterval = time.Duration(instanceCfg.PhysicalDiskPollingMinutes) * time.Minute
		}

		// Check if enough time has elapsed since last poll
		m.mu.Lock()
		lastPoll, exists := m.lastPhysicalDiskPoll[instanceName]
		shouldPoll := !exists || time.Since(lastPoll) >= pollingInterval
		if shouldPoll {
			m.lastPhysicalDiskPoll[instanceName] = time.Now()
		}
		m.mu.Unlock()

		if !shouldPoll {
			log.Debug().
				Str("instance", instanceName).
				Dur("sinceLastPoll", time.Since(lastPoll)).
				Dur("interval", pollingInterval).
				Msg("Skipping physical disk poll - interval not elapsed")
			// Refresh NVMe temperatures using the latest sensor data even when we skip the disk poll
			currentState := m.state.GetSnapshot()
			existing := make([]models.PhysicalDisk, 0)
			for _, disk := range currentState.PhysicalDisks {
				if disk.Instance == instanceName {
					existing = append(existing, disk)
				}
			}
			if len(existing) > 0 {
				updated := mergeNVMeTempsIntoDisks(existing, modelNodes)
				m.state.UpdatePhysicalDisks(instanceName, updated)
			}
		} else {
			// Run physical disk polling in background to avoid blocking the main task
			go func(inst string, pveClient PVEClientInterface, nodeList []proxmox.Node, nodeStatus map[string]string, modelNodesCopy []models.Node) {
				defer recoverFromPanic(fmt.Sprintf("pollPhysicalDisks-%s", inst))

				// Use a generous timeout for disk polling
				diskTimeout := 60 * time.Second
				diskCtx, diskCancel := context.WithTimeout(context.Background(), diskTimeout)
				defer diskCancel()

				log.Debug().
					Int("nodeCount", len(nodeList)).
					Dur("interval", pollingInterval).
					Msg("Starting disk health polling")

				// Get existing disks from state to preserve data for offline nodes
				currentState := m.state.GetSnapshot()
				existingDisksMap := make(map[string]models.PhysicalDisk)
				for _, disk := range currentState.PhysicalDisks {
					if disk.Instance == inst {
						existingDisksMap[disk.ID] = disk
					}
				}

				var allDisks []models.PhysicalDisk
				polledNodes := make(map[string]bool) // Track which nodes we successfully polled

				for _, node := range nodeList {
					// Check if context timed out
					select {
					case <-diskCtx.Done():
						log.Debug().
							Str("instance", inst).
							Msg("Physical disk polling timed out - preserving existing data")
						return
					default:
					}

					// Skip offline nodes but preserve their existing disk data
					if nodeStatus[node.Node] != "online" {
						log.Debug().Str("node", node.Node).Msg("Skipping disk poll for offline node - preserving existing data")
						continue
					}

					// Get disk list for this node
					log.Debug().Str("node", node.Node).Msg("Getting disk list for node")
					disks, err := pveClient.GetDisks(diskCtx, node.Node)
					if err != nil {
						// Check if it's a permission error or if the endpoint doesn't exist
						errStr := err.Error()
						if strings.Contains(errStr, "401") || strings.Contains(errStr, "403") {
							log.Warn().
								Str("node", node.Node).
								Err(err).
								Msg("Insufficient permissions to access disk information - check API token permissions")
						} else if strings.Contains(errStr, "404") || strings.Contains(errStr, "501") {
							log.Info().
								Str("node", node.Node).
								Msg("Disk monitoring not available on this node (may be using non-standard storage)")
						} else {
							log.Warn().
								Str("node", node.Node).
								Err(err).
								Msg("Failed to get disk list")
						}
						continue
					}

					log.Debug().
						Str("node", node.Node).
						Int("diskCount", len(disks)).
						Msg("Got disk list for node")

					// Mark this node as successfully polled
					polledNodes[node.Node] = true

					// Check each disk for health issues and add to state
					for _, disk := range disks {
						// Create PhysicalDisk model
						diskID := fmt.Sprintf("%s-%s-%s", inst, node.Node, strings.ReplaceAll(disk.DevPath, "/", "-"))
						physicalDisk := models.PhysicalDisk{
							ID:          diskID,
							Node:        node.Node,
							Instance:    inst,
							DevPath:     disk.DevPath,
							Model:       disk.Model,
							Serial:      disk.Serial,
							WWN:         disk.WWN,
							Type:        disk.Type,
							Size:        disk.Size,
							Health:      disk.Health,
							Wearout:     disk.Wearout,
							RPM:         disk.RPM,
							Used:        disk.Used,
							LastChecked: time.Now(),
						}

						allDisks = append(allDisks, physicalDisk)

						log.Debug().
							Str("node", node.Node).
							Str("disk", disk.DevPath).
							Str("model", disk.Model).
							Str("health", disk.Health).
							Int("wearout", disk.Wearout).
							Msg("Checking disk health")

						normalizedHealth := strings.ToUpper(strings.TrimSpace(disk.Health))
						if normalizedHealth != "" && normalizedHealth != "UNKNOWN" && normalizedHealth != "PASSED" && normalizedHealth != "OK" {
							// Disk has failed or is failing - alert manager will handle this
							log.Warn().
								Str("node", node.Node).
								Str("disk", disk.DevPath).
								Str("model", disk.Model).
								Str("health", disk.Health).
								Int("wearout", disk.Wearout).
								Msg("Disk health issue detected")

							// Pass disk info to alert manager
							m.alertManager.CheckDiskHealth(inst, node.Node, disk)
						} else if disk.Wearout > 0 && disk.Wearout < 10 {
							// Low wearout warning (less than 10% life remaining)
							log.Warn().
								Str("node", node.Node).
								Str("disk", disk.DevPath).
								Str("model", disk.Model).
								Int("wearout", disk.Wearout).
								Msg("SSD wearout critical - less than 10% life remaining")

							// Pass to alert manager for wearout alert
							m.alertManager.CheckDiskHealth(inst, node.Node, disk)
						}
					}
				}

				// Preserve existing disk data for nodes that weren't polled (offline or error)
				for _, existingDisk := range existingDisksMap {
					// Only preserve if we didn't poll this node
					if !polledNodes[existingDisk.Node] {
						// Keep the existing disk data but update the LastChecked to indicate it's stale
						allDisks = append(allDisks, existingDisk)
						log.Debug().
							Str("node", existingDisk.Node).
							Str("disk", existingDisk.DevPath).
							Msg("Preserving existing disk data for unpolled node")
					}
				}

				allDisks = mergeNVMeTempsIntoDisks(allDisks, modelNodesCopy)

				// Update physical disks in state
				log.Debug().
					Str("instance", inst).
					Int("diskCount", len(allDisks)).
					Int("preservedCount", len(existingDisksMap)-len(polledNodes)).
					Msg("Updating physical disks in state")
				m.state.UpdatePhysicalDisks(inst, allDisks)
			}(instanceName, client, nodes, nodeEffectiveStatus, modelNodes)
		}
	}
	// Note: Physical disk monitoring is now enabled by default with a 5-minute polling interval.
	// Users can explicitly disable it in node settings. Disk data is preserved between polls.

	// Wait for storage fallback to complete (with a short timeout) before using the data.
	// This is non-blocking in the sense that VM/container polling has already completed by now.
	// We give the storage fallback goroutine up to 2 additional seconds to finish if it's still running.
	select {
	case <-storageFallbackDone:
		// Storage fallback completed normally
	case <-time.After(2 * time.Second):
		log.Debug().
			Str("instance", instanceName).
			Msg("Storage fallback still running - proceeding without waiting (disk fallback may be unavailable)")
	}

	// Update nodes with storage fallback if rootfs was not available
	// Copy storageByNode under lock, then release to avoid holding during metric updates
	storageByNodeMu.Lock()
	localStorageByNode := make(map[string]models.Disk, len(storageByNode))
	for k, v := range storageByNode {
		localStorageByNode[k] = v
	}
	storageByNodeMu.Unlock()

	for i := range modelNodes {
		if modelNodes[i].Disk.Total == 0 {
			if disk, exists := localStorageByNode[modelNodes[i].Name]; exists {
				modelNodes[i].Disk = disk
				log.Debug().
					Str("node", modelNodes[i].Name).
					Float64("usage", disk.Usage).
					Msg("Applied storage fallback for disk metrics")
			}
		}

		if modelNodes[i].Status == "online" {
			// Record node metrics history only for online nodes
			now := time.Now()
			m.metricsHistory.AddNodeMetric(modelNodes[i].ID, "cpu", modelNodes[i].CPU*100, now)
			m.metricsHistory.AddNodeMetric(modelNodes[i].ID, "memory", modelNodes[i].Memory.Usage, now)
			m.metricsHistory.AddNodeMetric(modelNodes[i].ID, "disk", modelNodes[i].Disk.Usage, now)
			// Also write to persistent store
			if m.metricsStore != nil {
				m.metricsStore.Write("node", modelNodes[i].ID, "cpu", modelNodes[i].CPU*100, now)
				m.metricsStore.Write("node", modelNodes[i].ID, "memory", modelNodes[i].Memory.Usage, now)
				m.metricsStore.Write("node", modelNodes[i].ID, "disk", modelNodes[i].Disk.Usage, now)
			}
		}

		// Check thresholds for alerts
		m.alertManager.CheckNode(modelNodes[i])
	}

	// Update state again with corrected disk metrics
	m.state.UpdateNodesForInstance(instanceName, modelNodes)

	// Clean up alerts for nodes that no longer exist
	// Get all nodes from the global state (includes all instances)
	existingNodes := make(map[string]bool)
	allState := m.state.GetSnapshot()
	for _, node := range allState.Nodes {
		existingNodes[node.Name] = true
	}
	m.alertManager.CleanupAlertsForNodes(existingNodes)

	// Periodically re-check cluster status for nodes marked as standalone
	// This addresses issue #437 where clusters aren't detected on first attempt
	if !instanceCfg.IsCluster {
		// Check every 5 minutes if this is actually a cluster
		if time.Since(m.lastClusterCheck[instanceName]) > 5*time.Minute {
			m.lastClusterCheck[instanceName] = time.Now()

			// Try to detect if this is actually a cluster
			isActuallyCluster, checkErr := client.IsClusterMember(ctx)
			if checkErr == nil && isActuallyCluster {
				// This node is actually part of a cluster!
				log.Info().
					Str("instance", instanceName).
					Msg("Detected that standalone node is actually part of a cluster - updating configuration")

				// Update the configuration
				for i := range m.config.PVEInstances {
					if m.config.PVEInstances[i].Name == instanceName {
						m.config.PVEInstances[i].IsCluster = true
						// Note: We can't get the cluster name here without direct client access
						// It will be detected on the next configuration update
						log.Info().
							Str("instance", instanceName).
							Msg("Marked node as cluster member - cluster name will be detected on next update")

							// Save the updated configuration
						if m.persistence != nil {
							if err := m.persistence.SaveNodesConfig(m.config.PVEInstances, m.config.PBSInstances, m.config.PMGInstances); err != nil {
								log.Warn().Err(err).Msg("Failed to persist updated node configuration")
							}
						}
						break
					}
				}
			}
		}
	}

	// Update cluster endpoint online status if this is a cluster
	if instanceCfg.IsCluster && len(instanceCfg.ClusterEndpoints) > 0 {
		// Create a map of online nodes from our polling results
		onlineNodes := make(map[string]bool)
		for _, node := range modelNodes {
			// Node is online if we successfully got its data
			onlineNodes[node.Name] = node.Status == "online"
		}

		// Get Pulse connectivity status from ClusterClient if available
		var pulseHealth map[string]proxmox.EndpointHealth
		if clusterClient, ok := client.(*proxmox.ClusterClient); ok {
			pulseHealth = clusterClient.GetHealthStatusWithErrors()
		}

		// Update the online status for each cluster endpoint
		hasFingerprint := instanceCfg.Fingerprint != ""
		for i := range instanceCfg.ClusterEndpoints {
			if online, exists := onlineNodes[instanceCfg.ClusterEndpoints[i].NodeName]; exists {
				instanceCfg.ClusterEndpoints[i].Online = online
				if online {
					instanceCfg.ClusterEndpoints[i].LastSeen = time.Now()
				}
			}

			// Update Pulse connectivity status
			if pulseHealth != nil {
				// Try to find the endpoint in the health map by matching the effective URL
				endpointURL := clusterEndpointEffectiveURL(instanceCfg.ClusterEndpoints[i], instanceCfg.VerifySSL, hasFingerprint)
				if health, exists := pulseHealth[endpointURL]; exists {
					reachable := health.Healthy
					instanceCfg.ClusterEndpoints[i].PulseReachable = &reachable
					if !health.LastCheck.IsZero() {
						instanceCfg.ClusterEndpoints[i].LastPulseCheck = &health.LastCheck
					}
					instanceCfg.ClusterEndpoints[i].PulseError = health.LastError
				}
			}
		}

		// Update the config with the new online status
		// This is needed so the UI can reflect the current status
		for idx, cfg := range m.config.PVEInstances {
			if cfg.Name == instanceName {
				m.config.PVEInstances[idx].ClusterEndpoints = instanceCfg.ClusterEndpoints
				break
			}
		}
	}

	// Poll storage in background if enabled - storage APIs can be slow (NFS mounts, etc.)
	// so we run this asynchronously to prevent it from causing task timeouts.
	// This is similar to how backup polling runs in the background.
	if instanceCfg.MonitorStorage {
		select {
		case <-ctx.Done():
			pollErr = ctx.Err()
			return
		default:
			go func(inst string, pveClient PVEClientInterface, nodeList []proxmox.Node) {
				defer recoverFromPanic(fmt.Sprintf("pollStorageWithNodes-%s", inst))

				// Use a generous timeout for storage polling - it's not blocking the main task
				storageTimeout := 60 * time.Second
				storageCtx, storageCancel := context.WithTimeout(context.Background(), storageTimeout)
				defer storageCancel()

				m.pollStorageWithNodes(storageCtx, inst, pveClient, nodeList)
			}(instanceName, client, nodes)
		}
	}

	// Poll backups if enabled - respect configured interval or cycle gating
	if instanceCfg.MonitorBackups {
		if !m.config.EnableBackupPolling {
			log.Debug().
				Str("instance", instanceName).
				Msg("Skipping backup polling - globally disabled")
		} else {
			now := time.Now()

			m.mu.RLock()
			lastPoll := m.lastPVEBackupPoll[instanceName]
			m.mu.RUnlock()

			shouldPoll, reason, newLast := m.shouldRunBackupPoll(lastPoll, now)
			if !shouldPoll {
				if reason != "" {
					log.Debug().
						Str("instance", instanceName).
						Str("reason", reason).
						Msg("Skipping PVE backup polling this cycle")
				}
			} else {
				select {
				case <-ctx.Done():
					pollErr = ctx.Err()
					return
				default:
					// Set initial timestamp before starting goroutine (prevents concurrent starts)
					m.mu.Lock()
					m.lastPVEBackupPoll[instanceName] = newLast
					m.mu.Unlock()

					// Run backup polling in a separate goroutine to avoid blocking real-time stats
					go func(startTime time.Time, inst string, pveClient PVEClientInterface) {
						timeout := m.calculateBackupOperationTimeout(inst)
						log.Info().
							Str("instance", inst).
							Dur("timeout", timeout).
							Msg("Starting background backup/snapshot polling")

						// The per-cycle ctx is canceled as soon as the main polling loop finishes,
						// so derive the backup poll context from the long-lived runtime context instead.
						parentCtx := m.runtimeCtx
						if parentCtx == nil {
							parentCtx = context.Background()
						}

						backupCtx, cancel := context.WithTimeout(parentCtx, timeout)
						defer cancel()

						// Poll backup tasks
						m.pollBackupTasks(backupCtx, inst, pveClient)

						// Poll storage backups - pass nodes to avoid duplicate API calls
						m.pollStorageBackupsWithNodes(backupCtx, inst, pveClient, nodes, nodeEffectiveStatus)

						// Poll guest snapshots
						m.pollGuestSnapshots(backupCtx, inst, pveClient)

						duration := time.Since(startTime)
						log.Info().
							Str("instance", inst).
							Dur("duration", duration).
							Msg("Completed background backup/snapshot polling")

						// Update timestamp after completion for accurate interval scheduling
						m.mu.Lock()
						m.lastPVEBackupPoll[inst] = time.Now()
						m.mu.Unlock()
					}(now, instanceName, client)
				}
			}
		}
	}
}

// pollVMsAndContainersEfficient uses the cluster/resources endpoint to get all VMs and containers in one call
// This works on both clustered and standalone nodes for efficient polling
func (m *Monitor) pollVMsAndContainersEfficient(ctx context.Context, instanceName string, client PVEClientInterface, nodeEffectiveStatus map[string]string) bool {
	log.Info().Str("instance", instanceName).Msg("Polling VMs and containers using efficient cluster/resources endpoint")

	// Get all resources in a single API call
	resources, err := client.GetClusterResources(ctx, "vm")
	if err != nil {
		log.Debug().Err(err).Str("instance", instanceName).Msg("cluster/resources not available, falling back to traditional polling")
		return false
	}

	// Seed OCI classification from previous state so we never "downgrade" to LXC
	// if container config fetching intermittently fails (permissions or transient API errors).
	prevState := m.GetState()
	prevContainerIsOCI := make(map[int]bool)
	for _, ct := range prevState.Containers {
		if ct.Instance != instanceName {
			continue
		}
		if ct.VMID <= 0 {
			continue
		}
		if ct.Type == "oci" || ct.IsOCI {
			prevContainerIsOCI[ct.VMID] = true
		}
	}

	var allVMs []models.VM
	var allContainers []models.Container

	for _, res := range resources {
		// Avoid duplicating node name in ID when instance name equals node name
		var guestID string
		if instanceName == res.Node {
			guestID = fmt.Sprintf("%s-%d", res.Node, res.VMID)
		} else {
			guestID = fmt.Sprintf("%s-%s-%d", instanceName, res.Node, res.VMID)
		}

		// Debug log the resource type
		log.Debug().
			Str("instance", instanceName).
			Str("name", res.Name).
			Int("vmid", res.VMID).
			Str("type", res.Type).
			Msg("Processing cluster resource")

		// Initialize I/O metrics from cluster resources (may be 0 for VMs)
		diskReadBytes := int64(res.DiskRead)
		diskWriteBytes := int64(res.DiskWrite)
		networkInBytes := int64(res.NetIn)
		networkOutBytes := int64(res.NetOut)
		var individualDisks []models.Disk // Store individual filesystems for multi-disk monitoring
		var ipAddresses []string
		var networkInterfaces []models.GuestNetworkInterface
		var osName, osVersion, agentVersion string

		if res.Type == "qemu" {
			// Skip templates if configured
			if res.Template == 1 {
				continue
			}

			memTotal := res.MaxMem
			memUsed := res.Mem
			memorySource := "cluster-resources"
			guestRaw := VMMemoryRaw{
				ListingMem:    res.Mem,
				ListingMaxMem: res.MaxMem,
			}
			var detailedStatus *proxmox.VMStatus

			// Try to get actual disk usage from guest agent if VM is running
			diskUsed := res.Disk
			diskTotal := res.MaxDisk
			diskFree := diskTotal - diskUsed
			diskUsage := safePercentage(float64(diskUsed), float64(diskTotal))

			// If VM shows 0 disk usage but has allocated disk, it's likely guest agent issue
			// Set to -1 to indicate "unknown" rather than showing misleading 0%
			if res.Type == "qemu" && diskUsed == 0 && diskTotal > 0 && res.Status == "running" {
				diskUsage = -1
			}

			// For running VMs, always try to get filesystem info from guest agent
			// The cluster/resources endpoint often returns 0 or incorrect values for disk usage
			// We should prefer guest agent data when available for accurate metrics
			if res.Status == "running" && res.Type == "qemu" {
				// First check if agent is enabled by getting VM status
				status, err := client.GetVMStatus(ctx, res.Node, res.VMID)
				if err != nil {
					log.Debug().
						Err(err).
						Str("instance", instanceName).
						Str("vm", res.Name).
						Int("vmid", res.VMID).
						Msg("Could not get VM status to check guest agent availability")
				} else if status != nil {
					detailedStatus = status
					guestRaw.StatusMaxMem = detailedStatus.MaxMem
					guestRaw.StatusMem = detailedStatus.Mem
					guestRaw.StatusFreeMem = detailedStatus.FreeMem
					guestRaw.Balloon = detailedStatus.Balloon
					guestRaw.BalloonMin = detailedStatus.BalloonMin
					guestRaw.Agent = detailedStatus.Agent.Value
					memAvailable := uint64(0)
					if detailedStatus.MemInfo != nil {
						guestRaw.MemInfoUsed = detailedStatus.MemInfo.Used
						guestRaw.MemInfoFree = detailedStatus.MemInfo.Free
						guestRaw.MemInfoTotal = detailedStatus.MemInfo.Total
						guestRaw.MemInfoAvailable = detailedStatus.MemInfo.Available
						guestRaw.MemInfoBuffers = detailedStatus.MemInfo.Buffers
						guestRaw.MemInfoCached = detailedStatus.MemInfo.Cached
						guestRaw.MemInfoShared = detailedStatus.MemInfo.Shared

						switch {
						case detailedStatus.MemInfo.Available > 0:
							memAvailable = detailedStatus.MemInfo.Available
							memorySource = "meminfo-available"
						case detailedStatus.MemInfo.Free > 0 ||
							detailedStatus.MemInfo.Buffers > 0 ||
							detailedStatus.MemInfo.Cached > 0:
							memAvailable = detailedStatus.MemInfo.Free +
								detailedStatus.MemInfo.Buffers +
								detailedStatus.MemInfo.Cached
							memorySource = "meminfo-derived"
						}
					}

					// Use actual disk I/O values from detailed status
					diskReadBytes = int64(detailedStatus.DiskRead)
					diskWriteBytes = int64(detailedStatus.DiskWrite)
					networkInBytes = int64(detailedStatus.NetIn)
					networkOutBytes = int64(detailedStatus.NetOut)

					if detailedStatus.Balloon > 0 && detailedStatus.Balloon < detailedStatus.MaxMem {
						memTotal = detailedStatus.Balloon
						guestRaw.DerivedFromBall = true
					} else if detailedStatus.MaxMem > 0 {
						memTotal = detailedStatus.MaxMem
						guestRaw.DerivedFromBall = false
					}

					switch {
					case memAvailable > 0:
						if memAvailable > memTotal {
							memAvailable = memTotal
						}
						memUsed = memTotal - memAvailable
					case detailedStatus.FreeMem > 0 && memTotal >= detailedStatus.FreeMem:
						memUsed = memTotal - detailedStatus.FreeMem
						memorySource = "status-freemem"
					case detailedStatus.Mem > 0:
						memUsed = detailedStatus.Mem
						memorySource = "status-mem"
					}
					if memUsed > memTotal {
						memUsed = memTotal
					}

					// Gather guest metadata from the agent when available
					guestIPs, guestIfaces, guestOSName, guestOSVersion, guestAgentVersion := m.fetchGuestAgentMetadata(ctx, client, instanceName, res.Node, res.Name, res.VMID, detailedStatus)
					if len(guestIPs) > 0 {
						ipAddresses = guestIPs
					}
					if len(guestIfaces) > 0 {
						networkInterfaces = guestIfaces
					}
					if guestOSName != "" {
						osName = guestOSName
					}
					if guestOSVersion != "" {
						osVersion = guestOSVersion
					}
					if guestAgentVersion != "" {
						agentVersion = guestAgentVersion
					}

					// Always try to get filesystem info if agent is enabled
					// Prefer guest agent data over cluster/resources data for accuracy
					if detailedStatus.Agent.Value > 0 {
						log.Debug().
							Str("instance", instanceName).
							Str("vm", res.Name).
							Int("vmid", res.VMID).
							Int("agent", detailedStatus.Agent.Value).
							Uint64("current_disk", diskUsed).
							Uint64("current_maxdisk", diskTotal).
							Msg("Guest agent enabled, querying filesystem info for accurate disk usage")

						// Use retry logic for guest agent calls to handle transient timeouts (refs #630)
						fsInfoRaw, err := m.retryGuestAgentCall(ctx, m.guestAgentFSInfoTimeout, m.guestAgentRetries, func(ctx context.Context) (interface{}, error) {
							return client.GetVMFSInfo(ctx, res.Node, res.VMID)
						})
						var fsInfo []proxmox.VMFileSystem
						if err == nil {
							if fs, ok := fsInfoRaw.([]proxmox.VMFileSystem); ok {
								fsInfo = fs
							}
						}
						if err != nil {
							// Log more helpful error messages based on the error type
							errMsg := err.Error()
							if strings.Contains(errMsg, "500") || strings.Contains(errMsg, "QEMU guest agent is not running") {
								log.Info().
									Str("instance", instanceName).
									Str("vm", res.Name).
									Int("vmid", res.VMID).
									Msg("Guest agent enabled in VM config but not running inside guest OS. Install and start qemu-guest-agent in the VM")
								log.Info().
									Str("instance", instanceName).
									Str("vm", res.Name).
									Msg("To verify: ssh into VM and run 'systemctl status qemu-guest-agent' or 'ps aux | grep qemu-ga'")
							} else if strings.Contains(errMsg, "timeout") {
								log.Info().
									Str("instance", instanceName).
									Str("vm", res.Name).
									Int("vmid", res.VMID).
									Msg("Guest agent timeout - agent may be installed but not responding")
							} else if strings.Contains(errMsg, "403") || strings.Contains(errMsg, "401") || strings.Contains(errMsg, "authentication error") {
								// Permission error - user/token lacks required permissions
								log.Info().
									Str("instance", instanceName).
									Str("vm", res.Name).
									Int("vmid", res.VMID).
									Msg("VM disk monitoring permission denied. Check permissions:")
								log.Info().
									Str("instance", instanceName).
									Str("vm", res.Name).
									Msg("• Proxmox 9: Ensure token/user has VM.GuestAgent.Audit privilege (Pulse setup adds this via PulseMonitor role)")
								log.Info().
									Str("instance", instanceName).
									Str("vm", res.Name).
									Msg("• Proxmox 8: Ensure token/user has VM.Monitor privilege (Pulse setup adds this via PulseMonitor role)")
								log.Info().
									Str("instance", instanceName).
									Str("vm", res.Name).
									Msg("• All versions: Sys.Audit is recommended for Ceph metrics and applied when available")
								log.Info().
									Str("instance", instanceName).
									Str("vm", res.Name).
									Msg("• Re-run Pulse setup script if node was added before v4.7")
								log.Info().
									Str("instance", instanceName).
									Str("vm", res.Name).
									Msg("• Verify guest agent is installed and running inside the VM")
							} else {
								log.Debug().
									Err(err).
									Str("instance", instanceName).
									Str("vm", res.Name).
									Int("vmid", res.VMID).
									Msg("Failed to get filesystem info from guest agent")
							}
						} else if len(fsInfo) == 0 {
							log.Info().
								Str("instance", instanceName).
								Str("vm", res.Name).
								Int("vmid", res.VMID).
								Msg("Guest agent returned no filesystem info - agent may need restart or VM may have no mounted filesystems")
						} else {
							log.Debug().
								Str("instance", instanceName).
								Str("vm", res.Name).
								Int("filesystems", len(fsInfo)).
								Msg("Got filesystem info from guest agent")

							// Aggregate disk usage from all filesystems AND preserve individual disk data
							var totalBytes, usedBytes uint64
							var skippedFS []string
							var includedFS []string

							// Log all filesystems received for debugging
							log.Debug().
								Str("instance", instanceName).
								Str("vm", res.Name).
								Int("vmid", res.VMID).
								Int("filesystem_count", len(fsInfo)).
								Msg("Processing filesystems from guest agent")

							for _, fs := range fsInfo {
								// Skip special filesystems and mounts
								shouldSkip, reasons := fsfilters.ShouldSkipFilesystem(fs.Type, fs.Mountpoint, fs.TotalBytes, fs.UsedBytes)
								if shouldSkip {
									// Check if any reason is read-only for detailed logging
									for _, r := range reasons {
										if strings.HasPrefix(r, "read-only-") {
											log.Debug().
												Str("instance", instanceName).
												Str("vm", res.Name).
												Int("vmid", res.VMID).
												Str("mountpoint", fs.Mountpoint).
												Str("type", fs.Type).
												Float64("total_gb", float64(fs.TotalBytes)/1073741824).
												Float64("used_gb", float64(fs.UsedBytes)/1073741824).
												Msg("Skipping read-only filesystem from disk aggregation")
											break
										}
									}
									skippedFS = append(skippedFS, fmt.Sprintf("%s(%s,%s)",
										fs.Mountpoint, fs.Type, strings.Join(reasons, ",")))
									continue
								}

								// Only count real filesystems with valid data
								// Some filesystems report 0 bytes (like unformatted or system partitions)
								if fs.TotalBytes > 0 {
									totalBytes += fs.TotalBytes
									usedBytes += fs.UsedBytes
									includedFS = append(includedFS, fmt.Sprintf("%s(%s,%.1fGB)",
										fs.Mountpoint, fs.Type, float64(fs.TotalBytes)/1073741824))

									// Add to individual disks array
									individualDisks = append(individualDisks, models.Disk{
										Total:      int64(fs.TotalBytes),
										Used:       int64(fs.UsedBytes),
										Free:       int64(fs.TotalBytes - fs.UsedBytes),
										Usage:      safePercentage(float64(fs.UsedBytes), float64(fs.TotalBytes)),
										Mountpoint: fs.Mountpoint,
										Type:       fs.Type,
										Device:     fs.Disk,
									})

									log.Debug().
										Str("instance", instanceName).
										Str("vm", res.Name).
										Int("vmid", res.VMID).
										Str("mountpoint", fs.Mountpoint).
										Str("type", fs.Type).
										Uint64("total", fs.TotalBytes).
										Uint64("used", fs.UsedBytes).
										Float64("total_gb", float64(fs.TotalBytes)/1073741824).
										Float64("used_gb", float64(fs.UsedBytes)/1073741824).
										Msg("Including filesystem in disk usage calculation")
								} else if fs.TotalBytes == 0 && len(fs.Mountpoint) > 0 {
									skippedFS = append(skippedFS, fmt.Sprintf("%s(%s,0GB)", fs.Mountpoint, fs.Type))
									log.Debug().
										Str("instance", instanceName).
										Str("vm", res.Name).
										Int("vmid", res.VMID).
										Str("mountpoint", fs.Mountpoint).
										Str("type", fs.Type).
										Msg("Skipping filesystem with zero total bytes")
								}
							}

							if len(skippedFS) > 0 {
								log.Debug().
									Str("instance", instanceName).
									Str("vm", res.Name).
									Strs("skipped", skippedFS).
									Msg("Skipped special filesystems")
							}

							if len(includedFS) > 0 {
								log.Info().
									Str("instance", instanceName).
									Str("vm", res.Name).
									Int("vmid", res.VMID).
									Strs("included", includedFS).
									Msg("Filesystems included in disk calculation")
							}

							// If we got valid data from guest agent, use it
							if totalBytes > 0 {
								// Sanity check: if the reported disk is way larger than allocated disk,
								// we might be getting host disk info somehow
								allocatedDiskGB := float64(res.MaxDisk) / 1073741824
								reportedDiskGB := float64(totalBytes) / 1073741824

								// If reported disk is more than 2x the allocated disk, log a warning
								// This could indicate we're getting host disk or network shares
								if allocatedDiskGB > 0 && reportedDiskGB > allocatedDiskGB*2 {
									log.Warn().
										Str("instance", instanceName).
										Str("vm", res.Name).
										Int("vmid", res.VMID).
										Float64("allocated_gb", allocatedDiskGB).
										Float64("reported_gb", reportedDiskGB).
										Float64("ratio", reportedDiskGB/allocatedDiskGB).
										Strs("filesystems", includedFS).
										Msg("VM reports disk usage significantly larger than allocated disk - possible issue with filesystem detection")
								}

								diskTotal = totalBytes
								diskUsed = usedBytes
								diskFree = totalBytes - usedBytes
								diskUsage = safePercentage(float64(usedBytes), float64(totalBytes))

								log.Info().
									Str("instance", instanceName).
									Str("vm", res.Name).
									Int("vmid", res.VMID).
									Uint64("totalBytes", totalBytes).
									Uint64("usedBytes", usedBytes).
									Float64("total_gb", float64(totalBytes)/1073741824).
									Float64("used_gb", float64(usedBytes)/1073741824).
									Float64("allocated_gb", allocatedDiskGB).
									Float64("usage", diskUsage).
									Uint64("old_disk", res.Disk).
									Uint64("old_maxdisk", res.MaxDisk).
									Msg("Using guest agent data for accurate disk usage (replacing cluster/resources data)")
							} else {
								// Only special filesystems found - show allocated disk size instead
								if diskTotal > 0 {
									diskUsage = -1 // Show as allocated size
								}
								log.Info().
									Str("instance", instanceName).
									Str("vm", res.Name).
									Int("filesystems_found", len(fsInfo)).
									Msg("Guest agent provided filesystem info but no usable filesystems found (all were special mounts)")
							}
						}
					} else {
						// Agent disabled - show allocated disk size
						if diskTotal > 0 {
							diskUsage = -1 // Show as allocated size
						}
						log.Debug().
							Str("instance", instanceName).
							Str("vm", res.Name).
							Int("vmid", res.VMID).
							Int("agent", detailedStatus.Agent.Value).
							Msg("VM does not have guest agent enabled in config")
					}
				} else {
					// No vmStatus available - keep cluster/resources data
					log.Debug().
						Str("instance", instanceName).
						Str("vm", res.Name).
						Int("vmid", res.VMID).
						Msg("Could not get VM status, using cluster/resources disk data")
				}
			}

			if res.Status != "running" {
				memorySource = "powered-off"
				memUsed = 0
			}

			memFree := uint64(0)
			if memTotal >= memUsed {
				memFree = memTotal - memUsed
			}

			sampleTime := time.Now()
			currentMetrics := IOMetrics{
				DiskRead:   diskReadBytes,
				DiskWrite:  diskWriteBytes,
				NetworkIn:  networkInBytes,
				NetworkOut: networkOutBytes,
				Timestamp:  sampleTime,
			}
			diskReadRate, diskWriteRate, netInRate, netOutRate := m.rateTracker.CalculateRates(guestID, currentMetrics)

			memoryUsage := safePercentage(float64(memUsed), float64(memTotal))
			memory := models.Memory{
				Total: int64(memTotal),
				Used:  int64(memUsed),
				Free:  int64(memFree),
				Usage: memoryUsage,
			}
			if memory.Free < 0 {
				memory.Free = 0
			}
			if memory.Used > memory.Total {
				memory.Used = memory.Total
			}
			if detailedStatus != nil && detailedStatus.Balloon > 0 {
				memory.Balloon = int64(detailedStatus.Balloon)
			}

			vm := models.VM{
				ID:       guestID,
				VMID:     res.VMID,
				Name:     res.Name,
				Node:     res.Node,
				Instance: instanceName,
				Status:   res.Status,
				Type:     "qemu",
				CPU:      safeFloat(res.CPU),
				CPUs:     res.MaxCPU,
				Memory:   memory,
				Disk: models.Disk{
					Total: int64(diskTotal),
					Used:  int64(diskUsed),
					Free:  int64(diskFree),
					Usage: diskUsage,
				},
				Disks:             individualDisks, // Individual filesystem data
				IPAddresses:       ipAddresses,
				OSName:            osName,
				OSVersion:         osVersion,
				AgentVersion:      agentVersion,
				NetworkInterfaces: networkInterfaces,
				NetworkIn:         max(0, int64(netInRate)),
				NetworkOut:        max(0, int64(netOutRate)),
				DiskRead:          max(0, int64(diskReadRate)),
				DiskWrite:         max(0, int64(diskWriteRate)),
				Uptime:            int64(res.Uptime),
				Template:          res.Template == 1,
				LastSeen:          sampleTime,
			}

			// Parse tags
			if res.Tags != "" {
				vm.Tags = strings.Split(res.Tags, ";")

				// Log if Pulse-specific tags are detected
				for _, tag := range vm.Tags {
					switch tag {
					case "pulse-no-alerts", "pulse-monitor-only", "pulse-relaxed":
						log.Info().
							Str("vm", vm.Name).
							Str("node", vm.Node).
							Str("tag", tag).
							Msg("Pulse control tag detected on VM")
					}
				}
			}

			allVMs = append(allVMs, vm)

			m.recordGuestSnapshot(instanceName, vm.Type, res.Node, res.VMID, GuestMemorySnapshot{
				Name:         vm.Name,
				Status:       vm.Status,
				RetrievedAt:  sampleTime,
				MemorySource: memorySource,
				Memory:       vm.Memory,
				Raw:          guestRaw,
			})

			// For non-running VMs, zero out resource usage metrics to prevent false alerts
			// Proxmox may report stale or residual metrics for stopped VMs
			if vm.Status != "running" {
				log.Debug().
					Str("vm", vm.Name).
					Str("status", vm.Status).
					Float64("originalCpu", vm.CPU).
					Float64("originalMemUsage", vm.Memory.Usage).
					Msg("Non-running VM detected - zeroing metrics")

				// Zero out all usage metrics for stopped/paused/suspended VMs
				vm.CPU = 0
				vm.Memory.Usage = 0
				vm.Disk.Usage = 0
				vm.NetworkIn = 0
				vm.NetworkOut = 0
				vm.DiskRead = 0
				vm.DiskWrite = 0
			}

			// Check thresholds for alerts
			m.alertManager.CheckGuest(vm, instanceName)

		} else if res.Type == "lxc" {
			// Skip templates if configured
			if res.Template == 1 {
				continue
			}

			// Calculate I/O rates for container
			sampleTime := time.Now()
			currentMetrics := IOMetrics{
				DiskRead:   int64(res.DiskRead),
				DiskWrite:  int64(res.DiskWrite),
				NetworkIn:  int64(res.NetIn),
				NetworkOut: int64(res.NetOut),
				Timestamp:  sampleTime,
			}
			diskReadRate, diskWriteRate, netInRate, netOutRate := m.rateTracker.CalculateRates(guestID, currentMetrics)

			// Calculate cache-aware memory for LXC containers
			// The cluster resources API returns mem from cgroup which includes cache/buffers (inflated).
			// Try to get more accurate memory metrics from RRD data.
			memTotal := res.MaxMem
			memUsed := res.Mem
			memorySource := "cluster-resources"
			guestRaw := VMMemoryRaw{
				ListingMem:    res.Mem,
				ListingMaxMem: res.MaxMem,
			}

			// For running containers, try to get RRD data for cache-aware memory calculation
			if res.Status == "running" {
				rrdCtx, rrdCancel := context.WithTimeout(ctx, 5*time.Second)
				rrdPoints, err := client.GetLXCRRDData(rrdCtx, res.Node, res.VMID, "hour", "AVERAGE", []string{"memavailable", "memused", "maxmem"})
				rrdCancel()

				if err == nil && len(rrdPoints) > 0 {
					// Use the most recent RRD point
					point := rrdPoints[len(rrdPoints)-1]

					if point.MaxMem != nil && *point.MaxMem > 0 {
						guestRaw.StatusMaxMem = uint64(*point.MaxMem)
					}

					// Prefer memavailable-based calculation (excludes cache/buffers)
					if point.MemAvailable != nil && *point.MemAvailable > 0 {
						memAvailable := uint64(*point.MemAvailable)
						if memAvailable <= memTotal {
							memUsed = memTotal - memAvailable
							memorySource = "rrd-memavailable"
							guestRaw.MemInfoAvailable = memAvailable
							log.Debug().
								Str("container", res.Name).
								Str("node", res.Node).
								Uint64("total", memTotal).
								Uint64("available", memAvailable).
								Uint64("used", memUsed).
								Float64("usage", safePercentage(float64(memUsed), float64(memTotal))).
								Msg("LXC memory: using RRD memavailable (excludes reclaimable cache)")
						}
					} else if point.MemUsed != nil && *point.MemUsed > 0 {
						// Fall back to memused from RRD if available
						memUsed = uint64(*point.MemUsed)
						if memUsed <= memTotal {
							memorySource = "rrd-memused"
							guestRaw.MemInfoUsed = memUsed
							log.Debug().
								Str("container", res.Name).
								Str("node", res.Node).
								Uint64("total", memTotal).
								Uint64("used", memUsed).
								Float64("usage", safePercentage(float64(memUsed), float64(memTotal))).
								Msg("LXC memory: using RRD memused (excludes reclaimable cache)")
						}
					}
				} else if err != nil {
					log.Debug().
						Err(err).
						Str("instance", instanceName).
						Str("container", res.Name).
						Int("vmid", res.VMID).
						Msg("RRD memory data unavailable for LXC, using cluster resources value")
				}
			}

			container := models.Container{
				ID:       guestID,
				VMID:     res.VMID,
				Name:     res.Name,
				Node:     res.Node,
				Instance: instanceName,
				Status:   res.Status,
				Type:     "lxc",
				CPU:      safeFloat(res.CPU),
				CPUs:     res.MaxCPU,
				Memory: models.Memory{
					Total: int64(memTotal),
					Used:  int64(memUsed),
					Free:  int64(memTotal - memUsed),
					Usage: safePercentage(float64(memUsed), float64(memTotal)),
				},
				Disk: models.Disk{
					Total: int64(res.MaxDisk),
					Used:  int64(res.Disk),
					Free:  int64(res.MaxDisk - res.Disk),
					Usage: safePercentage(float64(res.Disk), float64(res.MaxDisk)),
				},
				NetworkIn:  max(0, int64(netInRate)),
				NetworkOut: max(0, int64(netOutRate)),
				DiskRead:   max(0, int64(diskReadRate)),
				DiskWrite:  max(0, int64(diskWriteRate)),
				Uptime:     int64(res.Uptime),
				Template:   res.Template == 1,
				LastSeen:   time.Now(),
			}

			if prevContainerIsOCI[container.VMID] {
				container.IsOCI = true
				container.Type = "oci"
			}

			// Parse tags
			if res.Tags != "" {
				container.Tags = strings.Split(res.Tags, ";")

				// Log if Pulse-specific tags are detected
				for _, tag := range container.Tags {
					switch tag {
					case "pulse-no-alerts", "pulse-monitor-only", "pulse-relaxed":
						log.Info().
							Str("container", container.Name).
							Str("node", container.Node).
							Str("tag", tag).
							Msg("Pulse control tag detected on container")
					}
				}
			}

			m.enrichContainerMetadata(ctx, client, instanceName, res.Node, &container)

			// For non-running containers, zero out resource usage metrics to prevent false alerts.
			// Proxmox may report stale or residual metrics for stopped containers.
			if container.Status != "running" {
				log.Debug().
					Str("container", container.Name).
					Str("status", container.Status).
					Float64("originalCpu", container.CPU).
					Float64("originalMemUsage", container.Memory.Usage).
					Msg("Non-running container detected - zeroing metrics")

				container.CPU = 0
				container.Memory.Usage = 0
				container.Disk.Usage = 0
				container.NetworkIn = 0
				container.NetworkOut = 0
				container.DiskRead = 0
				container.DiskWrite = 0
			}

			allContainers = append(allContainers, container)

			m.recordGuestSnapshot(instanceName, container.Type, res.Node, res.VMID, GuestMemorySnapshot{
				Name:         container.Name,
				Status:       container.Status,
				RetrievedAt:  sampleTime,
				MemorySource: memorySource,
				Memory:       container.Memory,
				Raw:          guestRaw,
			})

			// Check thresholds for alerts
			m.alertManager.CheckGuest(container, instanceName)
		}
	}

	// Preserve VMs and containers from nodes within grace period
	// The cluster/resources endpoint doesn't return VMs/containers from nodes Proxmox considers offline,
	// but we want to keep showing them if the node is within grace period
	// Count previous resources for this instance
	prevVMCount := 0
	prevContainerCount := 0
	for _, vm := range prevState.VMs {
		if vm.Instance == instanceName {
			prevVMCount++
		}
	}
	for _, container := range prevState.Containers {
		if container.Instance == instanceName {
			prevContainerCount++
		}
	}

	// Build map of which nodes are covered by current resources
	nodesWithResources := make(map[string]bool)
	for _, res := range resources {
		nodesWithResources[res.Node] = true
	}

	log.Info().
		Str("instance", instanceName).
		Int("nodesInResources", len(nodesWithResources)).
		Int("totalVMsFromResources", len(allVMs)).
		Int("totalContainersFromResources", len(allContainers)).
		Int("prevVMs", prevVMCount).
		Int("prevContainers", prevContainerCount).
		Msg("Cluster resources received, checking for grace period preservation")

	// If we got ZERO resources but had resources before, and we have no node data,
	// this likely means the cluster health check failed. Preserve everything.
	if len(allVMs) == 0 && len(allContainers) == 0 &&
		(prevVMCount > 0 || prevContainerCount > 0) &&
		len(nodeEffectiveStatus) == 0 {
		log.Warn().
			Str("instance", instanceName).
			Int("prevVMs", prevVMCount).
			Int("prevContainers", prevContainerCount).
			Msg("Cluster returned zero resources but had resources before - likely cluster health issue, preserving all previous resources")

		// Preserve all previous VMs and containers for this instance
		for _, vm := range prevState.VMs {
			if vm.Instance == instanceName {
				allVMs = append(allVMs, vm)
			}
		}
		for _, container := range prevState.Containers {
			if container.Instance == instanceName {
				allContainers = append(allContainers, container)
			}
		}
	}

	// Check for nodes that are within grace period but not in cluster/resources response
	preservedVMCount := 0
	preservedContainerCount := 0
	for nodeName, effectiveStatus := range nodeEffectiveStatus {
		if effectiveStatus == "online" && !nodesWithResources[nodeName] {
			// This node is within grace period but Proxmox didn't return its resources
			// Preserve previous VMs and containers from this node
			vmsBefore := len(allVMs)
			containersBefore := len(allContainers)

			// Preserve VMs from this node
			for _, vm := range prevState.VMs {
				if vm.Instance == instanceName && vm.Node == nodeName {
					allVMs = append(allVMs, vm)
				}
			}

			// Preserve containers from this node
			for _, container := range prevState.Containers {
				if container.Instance == instanceName && container.Node == nodeName {
					allContainers = append(allContainers, container)
				}
			}

			vmsPreserved := len(allVMs) - vmsBefore
			containersPreserved := len(allContainers) - containersBefore
			preservedVMCount += vmsPreserved
			preservedContainerCount += containersPreserved

			log.Info().
				Str("instance", instanceName).
				Str("node", nodeName).
				Int("vmsPreserved", vmsPreserved).
				Int("containersPreserved", containersPreserved).
				Msg("Preserved VMs/containers from node in grace period")
		}
	}

	if preservedVMCount > 0 || preservedContainerCount > 0 {
		log.Info().
			Str("instance", instanceName).
			Int("totalPreservedVMs", preservedVMCount).
			Int("totalPreservedContainers", preservedContainerCount).
			Msg("Grace period preservation complete")
	}

	// Always update state when using efficient polling path
	// Even if arrays are empty, we need to update to clear out VMs from genuinely offline nodes
	m.state.UpdateVMsForInstance(instanceName, allVMs)
	m.state.UpdateContainersForInstance(instanceName, allContainers)

	// Record guest metrics history for running guests (enables sparkline/trends view)
	now := time.Now()
	for _, vm := range allVMs {
		if vm.Status == "running" {
			m.metricsHistory.AddGuestMetric(vm.ID, "cpu", vm.CPU*100, now)
			m.metricsHistory.AddGuestMetric(vm.ID, "memory", vm.Memory.Usage, now)
			if vm.Disk.Usage >= 0 {
				m.metricsHistory.AddGuestMetric(vm.ID, "disk", vm.Disk.Usage, now)
			}
			// Also write to persistent store
			if m.metricsStore != nil {
				m.metricsStore.Write("vm", vm.ID, "cpu", vm.CPU*100, now)
				m.metricsStore.Write("vm", vm.ID, "memory", vm.Memory.Usage, now)
				if vm.Disk.Usage >= 0 {
					m.metricsStore.Write("vm", vm.ID, "disk", vm.Disk.Usage, now)
				}
			}
		}
	}
	for _, ct := range allContainers {
		if ct.Status == "running" {
			m.metricsHistory.AddGuestMetric(ct.ID, "cpu", ct.CPU*100, now)
			m.metricsHistory.AddGuestMetric(ct.ID, "memory", ct.Memory.Usage, now)
			if ct.Disk.Usage >= 0 {
				m.metricsHistory.AddGuestMetric(ct.ID, "disk", ct.Disk.Usage, now)
			}
			// Also write to persistent store
			if m.metricsStore != nil {
				m.metricsStore.Write("container", ct.ID, "cpu", ct.CPU*100, now)
				m.metricsStore.Write("container", ct.ID, "memory", ct.Memory.Usage, now)
				if ct.Disk.Usage >= 0 {
					m.metricsStore.Write("container", ct.ID, "disk", ct.Disk.Usage, now)
				}
			}
		}
	}

	m.pollReplicationStatus(ctx, instanceName, client, allVMs)

	log.Info().
		Str("instance", instanceName).
		Int("vms", len(allVMs)).
		Int("containers", len(allContainers)).
		Msg("VMs and containers polled efficiently with cluster/resources")

	return true
}

// pollBackupTasks polls backup tasks from a PVE instance
func (m *Monitor) pollBackupTasks(ctx context.Context, instanceName string, client PVEClientInterface) {
	log.Debug().Str("instance", instanceName).Msg("Polling backup tasks")

	tasks, err := client.GetBackupTasks(ctx)
	if err != nil {
		monErr := errors.WrapAPIError("get_backup_tasks", instanceName, err, 0)
		log.Error().Err(monErr).Str("instance", instanceName).Msg("Failed to get backup tasks")
		return
	}

	var backupTasks []models.BackupTask
	for _, task := range tasks {
		// Extract VMID from task ID (format: "UPID:node:pid:starttime:type:vmid:user@realm:")
		vmid := 0
		if task.ID != "" {
			if vmidInt, err := strconv.Atoi(task.ID); err == nil {
				vmid = vmidInt
			}
		}

		taskID := fmt.Sprintf("%s-%s", instanceName, task.UPID)

		backupTask := models.BackupTask{
			ID:        taskID,
			Node:      task.Node,
			Type:      task.Type,
			VMID:      vmid,
			Status:    task.Status,
			StartTime: time.Unix(task.StartTime, 0),
		}

		if task.EndTime > 0 {
			backupTask.EndTime = time.Unix(task.EndTime, 0)
		}

		backupTasks = append(backupTasks, backupTask)
	}

	// Update state with new backup tasks for this instance
	m.state.UpdateBackupTasksForInstance(instanceName, backupTasks)
}

// pollReplicationStatus polls storage replication jobs for a PVE instance.
func (m *Monitor) pollReplicationStatus(ctx context.Context, instanceName string, client PVEClientInterface, vms []models.VM) {
	log.Debug().Str("instance", instanceName).Msg("Polling replication status")

	jobs, err := client.GetReplicationStatus(ctx)
	if err != nil {
		errMsg := err.Error()
		lowerMsg := strings.ToLower(errMsg)
		if strings.Contains(errMsg, "501") || strings.Contains(errMsg, "404") || strings.Contains(lowerMsg, "not implemented") || strings.Contains(lowerMsg, "not supported") {
			log.Debug().
				Str("instance", instanceName).
				Msg("Replication API not available on this Proxmox instance")
			m.state.UpdateReplicationJobsForInstance(instanceName, []models.ReplicationJob{})
			return
		}

		monErr := errors.WrapAPIError("get_replication_status", instanceName, err, 0)
		log.Warn().
			Err(monErr).
			Str("instance", instanceName).
			Msg("Failed to get replication status")
		return
	}

	if len(jobs) == 0 {
		m.state.UpdateReplicationJobsForInstance(instanceName, []models.ReplicationJob{})
		return
	}

	vmByID := make(map[int]models.VM, len(vms))
	for _, vm := range vms {
		vmByID[vm.VMID] = vm
	}

	converted := make([]models.ReplicationJob, 0, len(jobs))
	now := time.Now()

	for idx, job := range jobs {
		guestID := job.GuestID
		if guestID == 0 {
			if parsed, err := strconv.Atoi(strings.TrimSpace(job.Guest)); err == nil {
				guestID = parsed
			}
		}

		guestName := ""
		guestType := ""
		guestNode := ""
		if guestID > 0 {
			if vm, ok := vmByID[guestID]; ok {
				guestName = vm.Name
				guestType = vm.Type
				guestNode = vm.Node
			}
		}
		if guestNode == "" {
			guestNode = strings.TrimSpace(job.Source)
		}

		sourceNode := strings.TrimSpace(job.Source)
		if sourceNode == "" {
			sourceNode = guestNode
		}

		targetNode := strings.TrimSpace(job.Target)

		var lastSyncTime *time.Time
		if job.LastSyncTime != nil && !job.LastSyncTime.IsZero() {
			t := job.LastSyncTime.UTC()
			lastSyncTime = &t
		}

		var nextSyncTime *time.Time
		if job.NextSyncTime != nil && !job.NextSyncTime.IsZero() {
			t := job.NextSyncTime.UTC()
			nextSyncTime = &t
		}

		lastSyncDurationHuman := job.LastSyncDurationHuman
		if lastSyncDurationHuman == "" && job.LastSyncDurationSeconds > 0 {
			lastSyncDurationHuman = formatSeconds(job.LastSyncDurationSeconds)
		}
		durationHuman := job.DurationHuman
		if durationHuman == "" && job.DurationSeconds > 0 {
			durationHuman = formatSeconds(job.DurationSeconds)
		}

		rateLimit := copyFloatPointer(job.RateLimitMbps)

		status := job.Status
		if status == "" {
			status = job.State
		}

		jobID := strings.TrimSpace(job.ID)
		if jobID == "" {
			if job.JobNumber > 0 && guestID > 0 {
				jobID = fmt.Sprintf("%d-%d", guestID, job.JobNumber)
			} else {
				jobID = fmt.Sprintf("job-%s-%d", instanceName, idx)
			}
		}

		uniqueID := fmt.Sprintf("%s-%s", instanceName, jobID)

		converted = append(converted, models.ReplicationJob{
			ID:                      uniqueID,
			Instance:                instanceName,
			JobID:                   jobID,
			JobNumber:               job.JobNumber,
			Guest:                   job.Guest,
			GuestID:                 guestID,
			GuestName:               guestName,
			GuestType:               guestType,
			GuestNode:               guestNode,
			SourceNode:              sourceNode,
			SourceStorage:           job.SourceStorage,
			TargetNode:              targetNode,
			TargetStorage:           job.TargetStorage,
			Schedule:                job.Schedule,
			Type:                    job.Type,
			Enabled:                 job.Enabled,
			State:                   job.State,
			Status:                  status,
			LastSyncStatus:          job.LastSyncStatus,
			LastSyncTime:            lastSyncTime,
			LastSyncUnix:            job.LastSyncUnix,
			LastSyncDurationSeconds: job.LastSyncDurationSeconds,
			LastSyncDurationHuman:   lastSyncDurationHuman,
			NextSyncTime:            nextSyncTime,
			NextSyncUnix:            job.NextSyncUnix,
			DurationSeconds:         job.DurationSeconds,
			DurationHuman:           durationHuman,
			FailCount:               job.FailCount,
			Error:                   job.Error,
			Comment:                 job.Comment,
			RemoveJob:               job.RemoveJob,
			RateLimitMbps:           rateLimit,
			LastPolled:              now,
		})
	}

	m.state.UpdateReplicationJobsForInstance(instanceName, converted)
}

func formatSeconds(total int) string {
	if total <= 0 {
		return ""
	}
	hours := total / 3600
	minutes := (total % 3600) / 60
	seconds := total % 60
	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}

func copyFloatPointer(src *float64) *float64 {
	if src == nil {
		return nil
	}
	val := *src
	return &val
}

// pollPBSInstance polls a single PBS instance
func (m *Monitor) pollPBSInstance(ctx context.Context, instanceName string, client *pbs.Client) {
	defer recoverFromPanic(fmt.Sprintf("pollPBSInstance-%s", instanceName))

	start := time.Now()
	debugEnabled := logging.IsLevelEnabled(zerolog.DebugLevel)
	var pollErr error
	if m.pollMetrics != nil {
		m.pollMetrics.IncInFlight("pbs")
		defer m.pollMetrics.DecInFlight("pbs")
		defer func() {
			m.pollMetrics.RecordResult(PollResult{
				InstanceName: instanceName,
				InstanceType: "pbs",
				Success:      pollErr == nil,
				Error:        pollErr,
				StartTime:    start,
				EndTime:      time.Now(),
			})
		}()
	}
	if m.stalenessTracker != nil {
		defer func() {
			if pollErr == nil {
				m.stalenessTracker.UpdateSuccess(InstanceTypePBS, instanceName, nil)
			} else {
				m.stalenessTracker.UpdateError(InstanceTypePBS, instanceName)
			}
		}()
	}
	defer m.recordTaskResult(InstanceTypePBS, instanceName, pollErr)

	// Check if context is cancelled
	select {
	case <-ctx.Done():
		pollErr = ctx.Err()
		if debugEnabled {
			log.Debug().Str("instance", instanceName).Msg("Polling cancelled")
		}
		return
	default:
	}

	if debugEnabled {
		log.Debug().Str("instance", instanceName).Msg("Polling PBS instance")
	}

	// Get instance config
	var instanceCfg *config.PBSInstance
	for _, cfg := range m.config.PBSInstances {
		if cfg.Name == instanceName {
			instanceCfg = &cfg
			if debugEnabled {
				log.Debug().
					Str("instance", instanceName).
					Bool("monitorDatastores", cfg.MonitorDatastores).
					Msg("Found PBS instance config")
			}
			break
		}
	}
	if instanceCfg == nil {
		log.Error().Str("instance", instanceName).Msg("PBS instance config not found")
		return
	}

	// Initialize PBS instance with default values
	pbsInst := models.PBSInstance{
		ID:               "pbs-" + instanceName,
		Name:             instanceName,
		Host:             instanceCfg.Host,
		Status:           "offline",
		Version:          "unknown",
		ConnectionHealth: "unhealthy",
		LastSeen:         time.Now(),
	}

	// Try to get version first
	version, versionErr := client.GetVersion(ctx)
	if versionErr == nil {
		pbsInst.Status = "online"
		pbsInst.Version = version.Version
		pbsInst.ConnectionHealth = "healthy"
		m.resetAuthFailures(instanceName, "pbs")
		m.state.SetConnectionHealth("pbs-"+instanceName, true)

		if debugEnabled {
			log.Debug().
				Str("instance", instanceName).
				Str("version", version.Version).
				Bool("monitorDatastores", instanceCfg.MonitorDatastores).
				Msg("PBS version retrieved successfully")
		}
	} else {
		if debugEnabled {
			log.Debug().Err(versionErr).Str("instance", instanceName).Msg("Failed to get PBS version, trying fallback")
		}

		// Use parent context for proper cancellation chain
		ctx2, cancel2 := context.WithTimeout(ctx, 10*time.Second)
		defer cancel2()
		_, datastoreErr := client.GetDatastores(ctx2)
		if datastoreErr == nil {
			pbsInst.Status = "online"
			pbsInst.Version = "connected"
			pbsInst.ConnectionHealth = "healthy"
			m.resetAuthFailures(instanceName, "pbs")
			m.state.SetConnectionHealth("pbs-"+instanceName, true)

			log.Info().
				Str("instance", instanceName).
				Msg("PBS connected (version unavailable but datastores accessible)")
		} else {
			pbsInst.Status = "offline"
			pbsInst.ConnectionHealth = "error"
			monErr := errors.WrapConnectionError("get_pbs_version", instanceName, versionErr)
			log.Error().Err(monErr).Str("instance", instanceName).Msg("Failed to connect to PBS")
			m.state.SetConnectionHealth("pbs-"+instanceName, false)

			if errors.IsAuthError(versionErr) || errors.IsAuthError(datastoreErr) {
				m.recordAuthFailure(instanceName, "pbs")
				return
			}
		}
	}

	// Get node status (CPU, memory, etc.)
	nodeStatus, err := client.GetNodeStatus(ctx)
	if err != nil {
		if debugEnabled {
			log.Debug().Err(err).Str("instance", instanceName).Msg("Could not get PBS node status (may need Sys.Audit permission)")
		}
	} else if nodeStatus != nil {
		pbsInst.CPU = nodeStatus.CPU
		if nodeStatus.Memory.Total > 0 {
			pbsInst.Memory = float64(nodeStatus.Memory.Used) / float64(nodeStatus.Memory.Total) * 100
			pbsInst.MemoryUsed = nodeStatus.Memory.Used
			pbsInst.MemoryTotal = nodeStatus.Memory.Total
		}
		pbsInst.Uptime = nodeStatus.Uptime

		log.Debug().
			Str("instance", instanceName).
			Float64("cpu", pbsInst.CPU).
			Float64("memory", pbsInst.Memory).
			Int64("uptime", pbsInst.Uptime).
			Msg("PBS node status retrieved")
	}

	// Poll datastores if enabled
	if instanceCfg.MonitorDatastores {
		datastores, err := client.GetDatastores(ctx)
		if err != nil {
			monErr := errors.WrapAPIError("get_datastores", instanceName, err, 0)
			log.Error().Err(monErr).Str("instance", instanceName).Msg("Failed to get datastores")
		} else {
			log.Info().
				Str("instance", instanceName).
				Int("count", len(datastores)).
				Msg("Got PBS datastores")

			for _, ds := range datastores {
				total := ds.Total
				if total == 0 && ds.TotalSpace > 0 {
					total = ds.TotalSpace
				}
				used := ds.Used
				if used == 0 && ds.UsedSpace > 0 {
					used = ds.UsedSpace
				}
				avail := ds.Avail
				if avail == 0 && ds.AvailSpace > 0 {
					avail = ds.AvailSpace
				}
				if total == 0 && used > 0 && avail > 0 {
					total = used + avail
				}

				log.Debug().
					Str("store", ds.Store).
					Int64("total", total).
					Int64("used", used).
					Int64("avail", avail).
					Int64("orig_total", ds.Total).
					Int64("orig_total_space", ds.TotalSpace).
					Msg("PBS datastore details")

				modelDS := models.PBSDatastore{
					Name:                ds.Store,
					Total:               total,
					Used:                used,
					Free:                avail,
					Usage:               safePercentage(float64(used), float64(total)),
					Status:              "available",
					DeduplicationFactor: ds.DeduplicationFactor,
				}

				namespaces, err := client.ListNamespaces(ctx, ds.Store, "", 0)
				if err != nil {
					log.Warn().Err(err).
						Str("instance", instanceName).
						Str("datastore", ds.Store).
						Msg("Failed to list namespaces")
				} else {
					for _, ns := range namespaces {
						nsPath := ns.NS
						if nsPath == "" {
							nsPath = ns.Path
						}
						if nsPath == "" {
							nsPath = ns.Name
						}

						modelNS := models.PBSNamespace{
							Path:   nsPath,
							Parent: ns.Parent,
							Depth:  strings.Count(nsPath, "/"),
						}
						modelDS.Namespaces = append(modelDS.Namespaces, modelNS)
					}

					hasRoot := false
					for _, ns := range modelDS.Namespaces {
						if ns.Path == "" {
							hasRoot = true
							break
						}
					}
					if !hasRoot {
						modelDS.Namespaces = append([]models.PBSNamespace{{Path: "", Depth: 0}}, modelDS.Namespaces...)
					}
				}

				pbsInst.Datastores = append(pbsInst.Datastores, modelDS)
			}
		}
	}

	// Update state and run alerts
	m.state.UpdatePBSInstance(pbsInst)
	log.Info().
		Str("instance", instanceName).
		Str("id", pbsInst.ID).
		Int("datastores", len(pbsInst.Datastores)).
		Msg("PBS instance updated in state")

	if m.alertManager != nil {
		m.alertManager.CheckPBS(pbsInst)
	}

	// Poll backups if enabled
	if instanceCfg.MonitorBackups {
		if len(pbsInst.Datastores) == 0 {
			log.Debug().
				Str("instance", instanceName).
				Msg("No PBS datastores available for backup polling")
		} else if !m.config.EnableBackupPolling {
			log.Debug().
				Str("instance", instanceName).
				Msg("Skipping PBS backup polling - globally disabled")
		} else {
			now := time.Now()

			m.mu.Lock()
			lastPoll := m.lastPBSBackupPoll[instanceName]
			if m.pbsBackupPollers == nil {
				m.pbsBackupPollers = make(map[string]bool)
			}
			inProgress := m.pbsBackupPollers[instanceName]
			m.mu.Unlock()

			shouldPoll, reason, newLast := m.shouldRunBackupPoll(lastPoll, now)
			if !shouldPoll {
				if reason != "" {
					log.Debug().
						Str("instance", instanceName).
						Str("reason", reason).
						Msg("Skipping PBS backup polling this cycle")
				}
			} else if inProgress {
				log.Debug().
					Str("instance", instanceName).
					Msg("PBS backup polling already in progress")
			} else {
				datastoreSnapshot := make([]models.PBSDatastore, len(pbsInst.Datastores))
				copy(datastoreSnapshot, pbsInst.Datastores)

				// Atomically check and set poller flag
				m.mu.Lock()
				if m.pbsBackupPollers[instanceName] {
					// Race: another goroutine started between our check and lock
					m.mu.Unlock()
					log.Debug().
						Str("instance", instanceName).
						Msg("PBS backup polling started by another goroutine")
				} else {
					m.pbsBackupPollers[instanceName] = true
					m.lastPBSBackupPoll[instanceName] = newLast
					m.mu.Unlock()

					go func(ds []models.PBSDatastore, inst string, start time.Time, pbsClient *pbs.Client) {
						defer func() {
							m.mu.Lock()
							delete(m.pbsBackupPollers, inst)
							m.lastPBSBackupPoll[inst] = time.Now()
							m.mu.Unlock()
						}()

						log.Info().
							Str("instance", inst).
							Int("datastores", len(ds)).
							Msg("Starting background PBS backup polling")

						// Detached background poll: parent ctx may be cancelled when the main
						// poll cycle finishes, so use a fresh context to let PBS polling
						// complete unless the explicit timeout is reached.
						backupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
						defer cancel()

						m.pollPBSBackups(backupCtx, inst, pbsClient, ds)

						log.Info().
							Str("instance", inst).
							Dur("duration", time.Since(start)).
							Msg("Completed background PBS backup polling")
					}(datastoreSnapshot, instanceName, now, client)
				}
			}
		}
	} else {
		log.Debug().
			Str("instance", instanceName).
			Msg("PBS backup monitoring disabled")
	}
}

// pollPMGInstance polls a single Proxmox Mail Gateway instance
func (m *Monitor) pollPMGInstance(ctx context.Context, instanceName string, client *pmg.Client) {
	defer recoverFromPanic(fmt.Sprintf("pollPMGInstance-%s", instanceName))

	start := time.Now()
	debugEnabled := logging.IsLevelEnabled(zerolog.DebugLevel)
	var pollErr error
	if m.pollMetrics != nil {
		m.pollMetrics.IncInFlight("pmg")
		defer m.pollMetrics.DecInFlight("pmg")
		defer func() {
			m.pollMetrics.RecordResult(PollResult{
				InstanceName: instanceName,
				InstanceType: "pmg",
				Success:      pollErr == nil,
				Error:        pollErr,
				StartTime:    start,
				EndTime:      time.Now(),
			})
		}()
	}
	if m.stalenessTracker != nil {
		defer func() {
			if pollErr == nil {
				m.stalenessTracker.UpdateSuccess(InstanceTypePMG, instanceName, nil)
			} else {
				m.stalenessTracker.UpdateError(InstanceTypePMG, instanceName)
			}
		}()
	}
	defer m.recordTaskResult(InstanceTypePMG, instanceName, pollErr)

	select {
	case <-ctx.Done():
		pollErr = ctx.Err()
		if debugEnabled {
			log.Debug().Str("instance", instanceName).Msg("PMG polling cancelled by context")
		}
		return
	default:
	}

	if debugEnabled {
		log.Debug().Str("instance", instanceName).Msg("Polling PMG instance")
	}

	var instanceCfg *config.PMGInstance
	for idx := range m.config.PMGInstances {
		if m.config.PMGInstances[idx].Name == instanceName {
			instanceCfg = &m.config.PMGInstances[idx]
			break
		}
	}

	if instanceCfg == nil {
		log.Error().Str("instance", instanceName).Msg("PMG instance config not found")
		pollErr = fmt.Errorf("pmg instance config not found for %s", instanceName)
		return
	}

	now := time.Now()
	pmgInst := models.PMGInstance{
		ID:               "pmg-" + instanceName,
		Name:             instanceName,
		Host:             instanceCfg.Host,
		Status:           "offline",
		ConnectionHealth: "unhealthy",
		LastSeen:         now,
		LastUpdated:      now,
	}

	version, err := client.GetVersion(ctx)
	if err != nil {
		monErr := errors.WrapConnectionError("pmg_get_version", instanceName, err)
		pollErr = monErr
		log.Error().Err(monErr).Str("instance", instanceName).Msg("Failed to connect to PMG instance")
		m.state.SetConnectionHealth("pmg-"+instanceName, false)
		m.state.UpdatePMGInstance(pmgInst)

		// Check PMG offline status against alert thresholds
		if m.alertManager != nil {
			m.alertManager.CheckPMG(pmgInst)
		}

		if errors.IsAuthError(err) {
			m.recordAuthFailure(instanceName, "pmg")
		}
		return
	}

	pmgInst.Status = "online"
	pmgInst.ConnectionHealth = "healthy"
	if version != nil {
		pmgInst.Version = strings.TrimSpace(version.Version)
	}
	m.state.SetConnectionHealth("pmg-"+instanceName, true)
	m.resetAuthFailures(instanceName, "pmg")

	cluster, err := client.GetClusterStatus(ctx, true)
	if err != nil {
		if debugEnabled {
			log.Debug().Err(err).Str("instance", instanceName).Msg("Failed to retrieve PMG cluster status")
		}
	}

	backupNodes := make(map[string]struct{})

	if len(cluster) > 0 {
		nodes := make([]models.PMGNodeStatus, 0, len(cluster))
		for _, entry := range cluster {
			status := strings.ToLower(strings.TrimSpace(entry.Type))
			if status == "" {
				status = "online"
			}
			node := models.PMGNodeStatus{
				Name:   entry.Name,
				Status: status,
				Role:   entry.Type,
			}

			backupNodes[entry.Name] = struct{}{}

			// Fetch queue status for this node
			if queueData, qErr := client.GetQueueStatus(ctx, entry.Name); qErr != nil {
				if debugEnabled {
					log.Debug().Err(qErr).
						Str("instance", instanceName).
						Str("node", entry.Name).
						Msg("Failed to fetch PMG queue status")
				}
			} else if queueData != nil {
				total := queueData.Active.Int64() + queueData.Deferred.Int64() + queueData.Hold.Int64() + queueData.Incoming.Int64()
				node.QueueStatus = &models.PMGQueueStatus{
					Active:    queueData.Active.Int(),
					Deferred:  queueData.Deferred.Int(),
					Hold:      queueData.Hold.Int(),
					Incoming:  queueData.Incoming.Int(),
					Total:     int(total),
					OldestAge: queueData.OldestAge.Int64(),
					UpdatedAt: time.Now(),
				}
			}

			nodes = append(nodes, node)
		}
		pmgInst.Nodes = nodes
	}

	if len(backupNodes) == 0 {
		trimmed := strings.TrimSpace(instanceName)
		if trimmed != "" {
			backupNodes[trimmed] = struct{}{}
		}
	}

	pmgBackups := make([]models.PMGBackup, 0)
	seenBackupIDs := make(map[string]struct{})

	for nodeName := range backupNodes {
		if ctx.Err() != nil {
			break
		}

		backups, backupErr := client.ListBackups(ctx, nodeName)
		if backupErr != nil {
			if debugEnabled {
				log.Debug().Err(backupErr).
					Str("instance", instanceName).
					Str("node", nodeName).
					Msg("Failed to list PMG configuration backups")
			}
			continue
		}

		for _, b := range backups {
			timestamp := b.Timestamp.Int64()
			backupTime := time.Unix(timestamp, 0)
			id := fmt.Sprintf("pmg-%s-%s-%d", instanceName, nodeName, timestamp)
			if _, exists := seenBackupIDs[id]; exists {
				continue
			}
			seenBackupIDs[id] = struct{}{}
			pmgBackups = append(pmgBackups, models.PMGBackup{
				ID:         id,
				Instance:   instanceName,
				Node:       nodeName,
				Filename:   b.Filename,
				BackupTime: backupTime,
				Size:       b.Size.Int64(),
			})
		}
	}

	if debugEnabled {
		log.Debug().
			Str("instance", instanceName).
			Int("backupCount", len(pmgBackups)).
			Msg("PMG backups polled")
	}

	if stats, err := client.GetMailStatistics(ctx, ""); err != nil {
		log.Warn().Err(err).Str("instance", instanceName).Msg("Failed to fetch PMG mail statistics")
	} else if stats != nil {
		pmgInst.MailStats = &models.PMGMailStats{
			Timeframe:            "day",
			CountTotal:           stats.Count.Float64(),
			CountIn:              stats.CountIn.Float64(),
			CountOut:             stats.CountOut.Float64(),
			SpamIn:               stats.SpamIn.Float64(),
			SpamOut:              stats.SpamOut.Float64(),
			VirusIn:              stats.VirusIn.Float64(),
			VirusOut:             stats.VirusOut.Float64(),
			BouncesIn:            stats.BouncesIn.Float64(),
			BouncesOut:           stats.BouncesOut.Float64(),
			BytesIn:              stats.BytesIn.Float64(),
			BytesOut:             stats.BytesOut.Float64(),
			GreylistCount:        stats.GreylistCount.Float64(),
			JunkIn:               stats.JunkIn.Float64(),
			AverageProcessTimeMs: stats.AvgProcessSec.Float64() * 1000,
			RBLRejects:           stats.RBLRejects.Float64(),
			PregreetRejects:      stats.Pregreet.Float64(),
			UpdatedAt:            time.Now(),
		}
	}

	if counts, err := client.GetMailCount(ctx, 86400); err != nil {
		if debugEnabled {
			log.Debug().Err(err).Str("instance", instanceName).Msg("Failed to fetch PMG mail count data")
		}
	} else if len(counts) > 0 {
		points := make([]models.PMGMailCountPoint, 0, len(counts))
		for _, entry := range counts {
			ts := time.Unix(entry.Time.Int64(), 0)
			points = append(points, models.PMGMailCountPoint{
				Timestamp:   ts,
				Count:       entry.Count.Float64(),
				CountIn:     entry.CountIn.Float64(),
				CountOut:    entry.CountOut.Float64(),
				SpamIn:      entry.SpamIn.Float64(),
				SpamOut:     entry.SpamOut.Float64(),
				VirusIn:     entry.VirusIn.Float64(),
				VirusOut:    entry.VirusOut.Float64(),
				RBLRejects:  entry.RBLRejects.Float64(),
				Pregreet:    entry.PregreetReject.Float64(),
				BouncesIn:   entry.BouncesIn.Float64(),
				BouncesOut:  entry.BouncesOut.Float64(),
				Greylist:    entry.GreylistCount.Float64(),
				Index:       entry.Index.Int(),
				Timeframe:   "hour",
				WindowStart: ts,
			})
		}
		pmgInst.MailCount = points
	}

	if scores, err := client.GetSpamScores(ctx); err != nil {
		if debugEnabled {
			log.Debug().Err(err).Str("instance", instanceName).Msg("Failed to fetch PMG spam score distribution")
		}
	} else if len(scores) > 0 {
		buckets := make([]models.PMGSpamBucket, 0, len(scores))
		for _, bucket := range scores {
			buckets = append(buckets, models.PMGSpamBucket{
				Score: bucket.Level,
				Count: float64(bucket.Count.Int()),
			})
		}
		pmgInst.SpamDistribution = buckets
	}

	quarantine := models.PMGQuarantineTotals{}
	if spamStatus, err := client.GetQuarantineStatus(ctx, "spam"); err == nil && spamStatus != nil {
		quarantine.Spam = int(spamStatus.Count.Int64())
	}
	if virusStatus, err := client.GetQuarantineStatus(ctx, "virus"); err == nil && virusStatus != nil {
		quarantine.Virus = int(virusStatus.Count.Int64())
	}
	pmgInst.Quarantine = &quarantine

	m.state.UpdatePMGBackups(instanceName, pmgBackups)
	m.state.UpdatePMGInstance(pmgInst)
	log.Info().
		Str("instance", instanceName).
		Str("status", pmgInst.Status).
		Int("nodes", len(pmgInst.Nodes)).
		Msg("PMG instance updated in state")

	// Check PMG metrics against alert thresholds
	if m.alertManager != nil {
		m.alertManager.CheckPMG(pmgInst)
	}
}

// GetState returns the current state
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
		hub.BroadcastState(frontendState)
	}

	if !enable && ctx != nil && hub != nil {
		// Kick off an immediate poll to repopulate state with live data
		go m.poll(ctx, hub)
		if m.config.DiscoveryEnabled {
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

// GetGuestMetrics returns historical metrics for a guest
func (m *Monitor) GetGuestMetrics(guestID string, duration time.Duration) map[string][]MetricPoint {
	return m.metricsHistory.GetAllGuestMetrics(guestID, duration)
}

// GetNodeMetrics returns historical metrics for a node
func (m *Monitor) GetNodeMetrics(nodeID string, metricType string, duration time.Duration) []MetricPoint {
	return m.metricsHistory.GetNodeMetrics(nodeID, metricType, duration)
}

// GetStorageMetrics returns historical metrics for storage
func (m *Monitor) GetStorageMetrics(storageID string, duration time.Duration) map[string][]MetricPoint {
	return m.metricsHistory.GetAllStorageMetrics(storageID, duration)
}

// GetAlertManager returns the alert manager
func (m *Monitor) GetAlertManager() *alerts.Manager {
	return m.alertManager
}

// SetAlertTriggeredAICallback sets an additional callback for AI analysis when alerts fire
// This enables token-efficient, real-time AI insights on specific resources
func (m *Monitor) SetAlertTriggeredAICallback(callback func(*alerts.Alert)) {
	if m.alertManager == nil || callback == nil {
		return
	}

	// Get the current callback
	originalCallback := m.alertManager

	// Wrap the existing callback to also call the AI callback
	m.alertManager.SetAlertCallback(func(alert *alerts.Alert) {
		// Broadcast to WebSocket (this happens via the callback set in Start())
		if m.wsHub != nil {
			m.wsHub.BroadcastAlert(alert)
		}

		// Send notifications
		log.Debug().
			Str("alertID", alert.ID).
			Str("level", string(alert.Level)).
			Msg("Alert raised, sending to notification manager")
		go m.notificationMgr.SendAlert(alert)

		// Trigger AI analysis
		go callback(alert)
	})

	// Avoid unused variable warning
	_ = originalCallback

	log.Info().Msg("Alert-triggered AI callback registered")
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

		// Convert platform data from json.RawMessage to map
		if len(r.PlatformData) > 0 {
			var platformMap map[string]any
			if err := json.Unmarshal(r.PlatformData, &platformMap); err == nil {
				input.PlatformData = platformMap
			}
		}

		result[i] = models.ConvertResourceToFrontend(input)
	}

	return result
}

// pollStorageBackupsWithNodes polls backups using a provided nodes list to avoid duplicate GetNodes calls
func (m *Monitor) pollStorageBackupsWithNodes(ctx context.Context, instanceName string, client PVEClientInterface, nodes []proxmox.Node, nodeEffectiveStatus map[string]string) {

	var allBackups []models.StorageBackup
	seenVolids := make(map[string]bool) // Track seen volume IDs to avoid duplicates
	hadSuccessfulNode := false          // Track if at least one node responded successfully
	storagesWithBackup := 0             // Number of storages that should contain backups
	contentSuccess := 0                 // Number of successful storage content fetches
	contentFailures := 0                // Number of failed storage content fetches
	storageQueryErrors := 0             // Number of nodes where storage list could not be queried
	storagePreserveNeeded := map[string]struct{}{}
	storageSuccess := map[string]struct{}{}

	// Build guest lookup map to find actual node for each VMID
	snapshot := m.state.GetSnapshot()
	guestNodeMap := make(map[int]string) // VMID -> actual node name
	for _, vm := range snapshot.VMs {
		if vm.Instance == instanceName {
			guestNodeMap[vm.VMID] = vm.Node
		}
	}
	for _, ct := range snapshot.Containers {
		if ct.Instance == instanceName {
			guestNodeMap[ct.VMID] = ct.Node
		}
	}

	// For each node, get storage and check content
	for _, node := range nodes {
		if nodeEffectiveStatus[node.Node] != "online" {
			for _, storageName := range storageNamesForNode(instanceName, node.Node, snapshot) {
				storagePreserveNeeded[storageName] = struct{}{}
			}
			continue
		}

		// Get storage for this node - retry once on timeout
		var storages []proxmox.Storage
		var err error

		for attempt := 1; attempt <= 2; attempt++ {
			storages, err = client.GetStorage(ctx, node.Node)
			if err == nil {
				break // Success
			}

			// Check if it's a timeout error
			errStr := err.Error()
			if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded") {
				if attempt == 1 {
					log.Warn().
						Str("node", node.Node).
						Str("instance", instanceName).
						Msg("Storage query timed out, retrying with extended timeout...")
					// Give it a bit more time on retry
					time.Sleep(2 * time.Second)
					continue
				}
			}
			// Non-timeout error or second attempt failed
			break
		}

		if err != nil {
			monErr := errors.NewMonitorError(errors.ErrorTypeAPI, "get_storage_for_backups", instanceName, err).WithNode(node.Node)
			log.Warn().Err(monErr).Str("node", node.Node).Msg("Failed to get storage for backups - skipping node")
			for _, storageName := range storageNamesForNode(instanceName, node.Node, snapshot) {
				storagePreserveNeeded[storageName] = struct{}{}
			}
			storageQueryErrors++
			continue
		}

		hadSuccessfulNode = true

		// For each storage that can contain backups or templates
		for _, storage := range storages {
			// Check if storage supports backup content
			if !strings.Contains(storage.Content, "backup") {
				continue
			}
			if !storageContentQueryable(storage) {
				continue
			}

			storagesWithBackup++

			// Get storage content
			contents, err := client.GetStorageContent(ctx, node.Node, storage.Storage)
			if err != nil {
				monErr := errors.NewMonitorError(errors.ErrorTypeAPI, "get_storage_content", instanceName, err).WithNode(node.Node)
				log.Debug().Err(monErr).
					Str("node", node.Node).
					Str("storage", storage.Storage).
					Msg("Failed to get storage content")
				if _, ok := storageSuccess[storage.Storage]; !ok {
					storagePreserveNeeded[storage.Storage] = struct{}{}
				}
				contentFailures++
				continue
			}

			contentSuccess++
			storageSuccess[storage.Storage] = struct{}{}
			delete(storagePreserveNeeded, storage.Storage)

			// Convert to models
			for _, content := range contents {
				// Skip if we've already seen this item (shared storage duplicate)
				if seenVolids[content.Volid] {
					continue
				}
				seenVolids[content.Volid] = true

				// Skip templates and ISOs - they're not backups
				if content.Content == "vztmpl" || content.Content == "iso" {
					continue
				}

				// Determine type from content type and VMID
				backupType := "unknown"
				if content.VMID == 0 {
					backupType = "host"
				} else if strings.Contains(content.Volid, "/vm/") || strings.Contains(content.Volid, "qemu") {
					backupType = "qemu"
				} else if strings.Contains(content.Volid, "/ct/") || strings.Contains(content.Volid, "lxc") {
					backupType = "lxc"
				} else if strings.Contains(content.Format, "pbs-ct") {
					// PBS format check as fallback
					backupType = "lxc"
				} else if strings.Contains(content.Format, "pbs-vm") {
					// PBS format check as fallback
					backupType = "qemu"
				}

				// Determine the correct node: for guest backups (VMID > 0), use the actual guest's node
				// For host backups (VMID == 0), use the node where the backup was found
				backupNode := node.Node
				if content.VMID > 0 {
					if actualNode, found := guestNodeMap[content.VMID]; found {
						backupNode = actualNode
					}
					// If not found in map, fall back to queried node (shouldn't happen normally)
				}
				isPBSStorage := strings.HasPrefix(storage.Storage, "pbs-") || storage.Type == "pbs"

				// Check verification status for PBS backups
				verified := false
				verificationInfo := ""
				if isPBSStorage {
					// Check if verified flag is set
					if content.Verified > 0 {
						verified = true
					}
					// Also check verification map if available
					if content.Verification != nil {
						if state, ok := content.Verification["state"].(string); ok {
							verified = (state == "ok")
							verificationInfo = state
						}
					}
				}

				backup := models.StorageBackup{
					ID:           fmt.Sprintf("%s-%s", instanceName, content.Volid),
					Storage:      storage.Storage,
					Node:         backupNode,
					Instance:     instanceName,
					Type:         backupType,
					VMID:         content.VMID,
					Time:         time.Unix(content.CTime, 0),
					CTime:        content.CTime,
					Size:         int64(content.Size),
					Format:       content.Format,
					Notes:        content.Notes,
					Protected:    content.Protected > 0,
					Volid:        content.Volid,
					IsPBS:        isPBSStorage,
					Verified:     verified,
					Verification: verificationInfo,
				}

				allBackups = append(allBackups, backup)
			}
		}
	}

	allBackups, preservedStorages := preserveFailedStorageBackups(instanceName, snapshot, storagePreserveNeeded, allBackups)
	if len(preservedStorages) > 0 {
		log.Warn().
			Str("instance", instanceName).
			Strs("storages", preservedStorages).
			Msg("Preserving previous storage backup data due to partial failures")
	}

	// Decide whether to keep existing backups when every query failed
	if shouldPreserveBackups(len(nodes), hadSuccessfulNode, storagesWithBackup, contentSuccess) {
		if len(nodes) > 0 && !hadSuccessfulNode {
			log.Warn().
				Str("instance", instanceName).
				Int("nodes", len(nodes)).
				Int("errors", storageQueryErrors).
				Msg("Failed to query storage on all nodes; keeping previous backup list")
		} else if storagesWithBackup > 0 && contentSuccess == 0 {
			log.Warn().
				Str("instance", instanceName).
				Int("storages", storagesWithBackup).
				Int("failures", contentFailures).
				Msg("All storage content queries failed; keeping previous backup list")
		}
		return
	}

	// Update state with storage backups for this instance
	m.state.UpdateStorageBackupsForInstance(instanceName, allBackups)

	// Sync backup times to VMs/Containers for backup status indicators
	m.state.SyncGuestBackupTimes()

	if m.alertManager != nil {
		snapshot := m.state.GetSnapshot()
		guestsByKey, guestsByVMID := buildGuestLookups(snapshot, m.guestMetadataStore)
		pveStorage := snapshot.Backups.PVE.StorageBackups
		if len(pveStorage) == 0 && len(snapshot.PVEBackups.StorageBackups) > 0 {
			pveStorage = snapshot.PVEBackups.StorageBackups
		}
		pbsBackups := snapshot.Backups.PBS
		if len(pbsBackups) == 0 && len(snapshot.PBSBackups) > 0 {
			pbsBackups = snapshot.PBSBackups
		}
		pmgBackups := snapshot.Backups.PMG
		if len(pmgBackups) == 0 && len(snapshot.PMGBackups) > 0 {
			pmgBackups = snapshot.PMGBackups
		}
		m.alertManager.CheckBackups(pveStorage, pbsBackups, pmgBackups, guestsByKey, guestsByVMID)
	}

	log.Debug().
		Str("instance", instanceName).
		Int("count", len(allBackups)).
		Msg("Storage backups polled")
}

func shouldPreserveBackups(nodeCount int, hadSuccessfulNode bool, storagesWithBackup, contentSuccess int) bool {
	if nodeCount > 0 && !hadSuccessfulNode {
		return true
	}
	if storagesWithBackup > 0 && contentSuccess == 0 {
		return true
	}
	return false
}

func shouldPreservePBSBackups(datastoreCount, datastoreFetches int) bool {
	// If there are datastores but all fetches failed, preserve existing backups
	if datastoreCount > 0 && datastoreFetches == 0 {
		return true
	}
	return false
}

func storageNamesForNode(instanceName, nodeName string, snapshot models.StateSnapshot) []string {
	if nodeName == "" {
		return nil
	}

	var storages []string
	for _, storage := range snapshot.Storage {
		if storage.Instance != instanceName {
			continue
		}
		if storage.Name == "" {
			continue
		}
		if !strings.Contains(storage.Content, "backup") {
			continue
		}
		if storage.Node == nodeName {
			storages = append(storages, storage.Name)
			continue
		}
		for _, node := range storage.Nodes {
			if node == nodeName {
				storages = append(storages, storage.Name)
				break
			}
		}
	}

	return storages
}

func preserveFailedStorageBackups(instanceName string, snapshot models.StateSnapshot, storagesToPreserve map[string]struct{}, current []models.StorageBackup) ([]models.StorageBackup, []string) {
	if len(storagesToPreserve) == 0 {
		return current, nil
	}

	existing := make(map[string]struct{}, len(current))
	for _, backup := range current {
		existing[backup.ID] = struct{}{}
	}

	preserved := make(map[string]struct{})
	for _, backup := range snapshot.PVEBackups.StorageBackups {
		if backup.Instance != instanceName {
			continue
		}
		if _, ok := storagesToPreserve[backup.Storage]; !ok {
			continue
		}
		if _, duplicate := existing[backup.ID]; duplicate {
			continue
		}
		current = append(current, backup)
		existing[backup.ID] = struct{}{}
		preserved[backup.Storage] = struct{}{}
	}

	if len(preserved) == 0 {
		return current, nil
	}

	storages := make([]string, 0, len(preserved))
	for storage := range preserved {
		storages = append(storages, storage)
	}
	sort.Strings(storages)
	return current, storages
}

func buildGuestLookups(snapshot models.StateSnapshot, metadataStore *config.GuestMetadataStore) (map[string]alerts.GuestLookup, map[string][]alerts.GuestLookup) {
	byKey := make(map[string]alerts.GuestLookup)
	byVMID := make(map[string][]alerts.GuestLookup)

	for _, vm := range snapshot.VMs {
		info := alerts.GuestLookup{
			Name:     vm.Name,
			Instance: vm.Instance,
			Node:     vm.Node,
			Type:     vm.Type,
			VMID:     vm.VMID,
		}
		key := alerts.BuildGuestKey(vm.Instance, vm.Node, vm.VMID)
		byKey[key] = info

		vmidKey := strconv.Itoa(vm.VMID)
		byVMID[vmidKey] = append(byVMID[vmidKey], info)

		// Persist last-known name and type for this guest
		if metadataStore != nil && vm.Name != "" {
			persistGuestIdentity(metadataStore, key, vm.Name, vm.Type)
		}
	}

	for _, ct := range snapshot.Containers {
		info := alerts.GuestLookup{
			Name:     ct.Name,
			Instance: ct.Instance,
			Node:     ct.Node,
			Type:     ct.Type,
			VMID:     ct.VMID,
		}
		key := alerts.BuildGuestKey(ct.Instance, ct.Node, ct.VMID)
		if _, exists := byKey[key]; !exists {
			byKey[key] = info
		}

		vmidKey := strconv.Itoa(ct.VMID)
		byVMID[vmidKey] = append(byVMID[vmidKey], info)

		// Persist last-known name and type for this guest
		if metadataStore != nil && ct.Name != "" {
			persistGuestIdentity(metadataStore, key, ct.Name, ct.Type)
		}
	}

	// Augment byVMID with persisted metadata for deleted guests
	if metadataStore != nil {
		enrichWithPersistedMetadata(metadataStore, byVMID)
	}

	return byKey, byVMID
}

// enrichWithPersistedMetadata adds entries from the metadata store for guests
// that no longer exist in the live inventory but have persisted identity data
func enrichWithPersistedMetadata(metadataStore *config.GuestMetadataStore, byVMID map[string][]alerts.GuestLookup) {
	allMetadata := metadataStore.GetAll()
	for guestKey, meta := range allMetadata {
		if meta.LastKnownName == "" {
			continue // No name persisted, skip
		}

		// Parse the guest key (format: instance:node:vmid)
		// We need to extract instance, node, and vmid
		var instance, node string
		var vmid int
		if _, err := fmt.Sscanf(guestKey, "%[^:]:%[^:]:%d", &instance, &node, &vmid); err != nil {
			continue // Invalid key format
		}

		vmidKey := strconv.Itoa(vmid)

		// Check if we already have a live entry for this exact guest
		hasLiveEntry := false
		for _, existing := range byVMID[vmidKey] {
			if existing.Instance == instance && existing.Node == node && existing.VMID == vmid {
				hasLiveEntry = true
				break
			}
		}

		// Only add persisted metadata if no live entry exists
		if !hasLiveEntry {
			byVMID[vmidKey] = append(byVMID[vmidKey], alerts.GuestLookup{
				Name:     meta.LastKnownName,
				Instance: instance,
				Node:     node,
				Type:     meta.LastKnownType,
				VMID:     vmid,
			})
		}
	}
}

// persistGuestIdentity updates the metadata store with the last-known name and type for a guest
func persistGuestIdentity(metadataStore *config.GuestMetadataStore, guestKey, name, guestType string) {
	existing := metadataStore.Get(guestKey)
	if existing == nil {
		existing = &config.GuestMetadata{
			ID:   guestKey,
			Tags: []string{},
		}
	}

	guestType = strings.TrimSpace(guestType)
	if guestType == "" {
		return
	}

	// Never "downgrade" OCI containers back to LXC. OCI classification can be transiently
	// unavailable if Proxmox config reads fail due to permissions or transient API errors.
	if existing.LastKnownType == "oci" && guestType != "oci" {
		guestType = existing.LastKnownType
	}

	// Only update if the name or type has changed
	if existing.LastKnownName != name || existing.LastKnownType != guestType {
		existing.LastKnownName = name
		existing.LastKnownType = guestType
		// Save asynchronously to avoid blocking the monitor
		go func() {
			if err := metadataStore.Set(guestKey, existing); err != nil {
				log.Error().Err(err).Str("guestKey", guestKey).Msg("Failed to persist guest identity")
			}
		}()
	}
}

func (m *Monitor) calculateBackupOperationTimeout(instanceName string) time.Duration {
	const (
		minTimeout      = 2 * time.Minute
		maxTimeout      = 5 * time.Minute
		timeoutPerGuest = 2 * time.Second
	)

	timeout := minTimeout
	snapshot := m.state.GetSnapshot()

	guestCount := 0
	for _, vm := range snapshot.VMs {
		if vm.Instance == instanceName && !vm.Template {
			guestCount++
		}
	}
	for _, ct := range snapshot.Containers {
		if ct.Instance == instanceName && !ct.Template {
			guestCount++
		}
	}

	if guestCount > 0 {
		dynamic := time.Duration(guestCount) * timeoutPerGuest
		if dynamic > timeout {
			timeout = dynamic
		}
	}

	if timeout > maxTimeout {
		return maxTimeout
	}

	return timeout
}

// pollGuestSnapshots polls snapshots for all VMs and containers
func (m *Monitor) pollGuestSnapshots(ctx context.Context, instanceName string, client PVEClientInterface) {
	log.Debug().Str("instance", instanceName).Msg("Polling guest snapshots")

	// Get current VMs and containers from state for this instance
	m.mu.RLock()
	var vms []models.VM
	for _, vm := range m.state.VMs {
		if vm.Instance == instanceName {
			vms = append(vms, vm)
		}
	}
	var containers []models.Container
	for _, ct := range m.state.Containers {
		if ct.Instance == instanceName {
			containers = append(containers, ct)
		}
	}
	m.mu.RUnlock()

	guestKey := func(instance, node string, vmid int) string {
		if instance == node {
			return fmt.Sprintf("%s-%d", node, vmid)
		}
		return fmt.Sprintf("%s-%s-%d", instance, node, vmid)
	}

	guestNames := make(map[string]string, len(vms)+len(containers))
	for _, vm := range vms {
		guestNames[guestKey(instanceName, vm.Node, vm.VMID)] = vm.Name
	}
	for _, ct := range containers {
		guestNames[guestKey(instanceName, ct.Node, ct.VMID)] = ct.Name
	}

	activeGuests := 0
	for _, vm := range vms {
		if !vm.Template {
			activeGuests++
		}
	}
	for _, ct := range containers {
		if !ct.Template {
			activeGuests++
		}
	}

	const (
		minSnapshotTimeout      = 60 * time.Second
		maxSnapshotTimeout      = 4 * time.Minute
		snapshotTimeoutPerGuest = 2 * time.Second
	)

	timeout := minSnapshotTimeout
	if activeGuests > 0 {
		dynamic := time.Duration(activeGuests) * snapshotTimeoutPerGuest
		if dynamic > timeout {
			timeout = dynamic
		}
	}
	if timeout > maxSnapshotTimeout {
		timeout = maxSnapshotTimeout
	}

	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			log.Warn().
				Str("instance", instanceName).
				Msg("Skipping guest snapshot polling; backup context deadline exceeded")
			return
		}
		if timeout > remaining {
			timeout = remaining
		}
	}

	snapshotCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	log.Debug().
		Str("instance", instanceName).
		Int("guestCount", activeGuests).
		Dur("timeout", timeout).
		Msg("Guest snapshot polling budget established")

	var allSnapshots []models.GuestSnapshot
	deadlineExceeded := false

	// Poll VM snapshots
	for _, vm := range vms {
		// Skip templates
		if vm.Template {
			continue
		}

		snapshots, err := client.GetVMSnapshots(snapshotCtx, vm.Node, vm.VMID)
		if err != nil {
			if snapshotCtx.Err() != nil {
				log.Warn().
					Str("instance", instanceName).
					Str("node", vm.Node).
					Int("vmid", vm.VMID).
					Err(snapshotCtx.Err()).
					Msg("Aborting guest snapshot polling due to context cancellation while fetching VM snapshots")
				deadlineExceeded = true
				break
			}
			// This is common for VMs without snapshots, so use debug level
			monErr := errors.NewMonitorError(errors.ErrorTypeAPI, "get_vm_snapshots", instanceName, err).WithNode(vm.Node)
			log.Debug().
				Err(monErr).
				Str("node", vm.Node).
				Int("vmid", vm.VMID).
				Msg("Failed to get VM snapshots")
			continue
		}

		for _, snap := range snapshots {
			snapshot := models.GuestSnapshot{
				ID:          fmt.Sprintf("%s-%s-%d-%s", instanceName, vm.Node, vm.VMID, snap.Name),
				Name:        snap.Name,
				Node:        vm.Node,
				Instance:    instanceName,
				Type:        "qemu",
				VMID:        vm.VMID,
				Time:        time.Unix(snap.SnapTime, 0),
				Description: snap.Description,
				Parent:      snap.Parent,
				VMState:     true, // VM state support enabled
			}

			allSnapshots = append(allSnapshots, snapshot)
		}
	}

	if deadlineExceeded {
		log.Warn().
			Str("instance", instanceName).
			Msg("Guest snapshot polling timed out before completing VM collection; retaining previous snapshots")
		return
	}

	// Poll container snapshots
	for _, ct := range containers {
		// Skip templates
		if ct.Template {
			continue
		}

		snapshots, err := client.GetContainerSnapshots(snapshotCtx, ct.Node, ct.VMID)
		if err != nil {
			if snapshotCtx.Err() != nil {
				log.Warn().
					Str("instance", instanceName).
					Str("node", ct.Node).
					Int("vmid", ct.VMID).
					Err(snapshotCtx.Err()).
					Msg("Aborting guest snapshot polling due to context cancellation while fetching container snapshots")
				deadlineExceeded = true
				break
			}
			// API error 596 means snapshots not supported/available - this is expected for many containers
			errStr := err.Error()
			if strings.Contains(errStr, "596") || strings.Contains(errStr, "not available") {
				// Silently skip containers without snapshot support
				continue
			}
			// Log other errors at debug level
			monErr := errors.NewMonitorError(errors.ErrorTypeAPI, "get_container_snapshots", instanceName, err).WithNode(ct.Node)
			log.Debug().
				Err(monErr).
				Str("node", ct.Node).
				Int("vmid", ct.VMID).
				Msg("Failed to get container snapshots")
			continue
		}

		for _, snap := range snapshots {
			snapshot := models.GuestSnapshot{
				ID:          fmt.Sprintf("%s-%s-%d-%s", instanceName, ct.Node, ct.VMID, snap.Name),
				Name:        snap.Name,
				Node:        ct.Node,
				Instance:    instanceName,
				Type:        "lxc",
				VMID:        ct.VMID,
				Time:        time.Unix(snap.SnapTime, 0),
				Description: snap.Description,
				Parent:      snap.Parent,
				VMState:     false,
			}

			allSnapshots = append(allSnapshots, snapshot)
		}
	}

	if deadlineExceeded || snapshotCtx.Err() != nil {
		log.Warn().
			Str("instance", instanceName).
			Msg("Guest snapshot polling timed out before completion; retaining previous snapshots")
		return
	}

	if len(allSnapshots) > 0 {
		sizeMap := m.collectSnapshotSizes(snapshotCtx, instanceName, client, allSnapshots)
		if len(sizeMap) > 0 {
			for i := range allSnapshots {
				if size, ok := sizeMap[allSnapshots[i].ID]; ok && size > 0 {
					allSnapshots[i].SizeBytes = size
				}
			}
		}
	}

	// Update state with guest snapshots for this instance
	m.state.UpdateGuestSnapshotsForInstance(instanceName, allSnapshots)

	if m.alertManager != nil {
		m.alertManager.CheckSnapshotsForInstance(instanceName, allSnapshots, guestNames)
	}

	log.Debug().
		Str("instance", instanceName).
		Int("count", len(allSnapshots)).
		Msg("Guest snapshots polled")
}

func (m *Monitor) collectSnapshotSizes(ctx context.Context, instanceName string, client PVEClientInterface, snapshots []models.GuestSnapshot) map[string]int64 {
	sizes := make(map[string]int64, len(snapshots))
	if len(snapshots) == 0 {
		return sizes
	}

	validSnapshots := make(map[string]struct{}, len(snapshots))
	nodes := make(map[string]struct{})

	for _, snap := range snapshots {
		validSnapshots[snap.ID] = struct{}{}
		if snap.Node != "" {
			nodes[snap.Node] = struct{}{}
		}
	}

	if len(nodes) == 0 {
		return sizes
	}

	seenVolids := make(map[string]struct{})

	for nodeName := range nodes {
		if ctx.Err() != nil {
			break
		}

		storages, err := client.GetStorage(ctx, nodeName)
		if err != nil {
			log.Debug().
				Err(err).
				Str("node", nodeName).
				Str("instance", instanceName).
				Msg("Failed to get storage list for snapshot sizing")
			continue
		}

		for _, storage := range storages {
			if ctx.Err() != nil {
				break
			}

			contentTypes := strings.ToLower(storage.Content)
			if !strings.Contains(contentTypes, "images") && !strings.Contains(contentTypes, "rootdir") {
				continue
			}
			if !storageContentQueryable(storage) {
				continue
			}

			contents, err := client.GetStorageContent(ctx, nodeName, storage.Storage)
			if err != nil {
				log.Debug().
					Err(err).
					Str("node", nodeName).
					Str("storage", storage.Storage).
					Str("instance", instanceName).
					Msg("Failed to get storage content for snapshot sizing")
				continue
			}

			for _, item := range contents {
				if item.VMID <= 0 {
					continue
				}

				if _, seen := seenVolids[item.Volid]; seen {
					continue
				}

				snapName := extractSnapshotName(item.Volid)
				if snapName == "" {
					continue
				}

				key := fmt.Sprintf("%s-%s-%d-%s", instanceName, nodeName, item.VMID, snapName)
				if _, ok := validSnapshots[key]; !ok {
					continue
				}

				seenVolids[item.Volid] = struct{}{}

				size := int64(item.Size)
				if size < 0 {
					size = 0
				}

				sizes[key] += size
			}
		}
	}

	return sizes
}

func extractSnapshotName(volid string) string {
	if volid == "" {
		return ""
	}

	parts := strings.SplitN(volid, ":", 2)
	remainder := volid
	if len(parts) == 2 {
		remainder = parts[1]
	}

	if idx := strings.Index(remainder, "@"); idx >= 0 && idx+1 < len(remainder) {
		return strings.TrimSpace(remainder[idx+1:])
	}

	return ""
}

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
func (m *Monitor) recordAuthFailure(instanceName string, nodeType string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	nodeID := instanceName
	if nodeType != "" {
		nodeID = nodeType + "-" + instanceName
	}

	// Increment failure count
	m.authFailures[nodeID]++
	m.lastAuthAttempt[nodeID] = time.Now()

	log.Warn().
		Str("node", nodeID).
		Int("failures", m.authFailures[nodeID]).
		Msg("Authentication failure recorded")

	// If we've exceeded the threshold, remove the node
	const maxAuthFailures = 5
	if m.authFailures[nodeID] >= maxAuthFailures {
		log.Error().
			Str("node", nodeID).
			Int("failures", m.authFailures[nodeID]).
			Msg("Maximum authentication failures reached, removing node from state")

		// Remove from state based on type
		if nodeType == "pve" {
			m.removeFailedPVENode(instanceName)
		} else if nodeType == "pbs" {
			m.removeFailedPBSNode(instanceName)
		} else if nodeType == "pmg" {
			m.removeFailedPMGInstance(instanceName)
		}

		// Reset the counter since we've removed the node
		delete(m.authFailures, nodeID)
		delete(m.lastAuthAttempt, nodeID)
	}
}

// resetAuthFailures resets the failure count for a node after successful auth
func (m *Monitor) resetAuthFailures(instanceName string, nodeType string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	nodeID := instanceName
	if nodeType != "" {
		nodeID = nodeType + "-" + instanceName
	}

	if count, exists := m.authFailures[nodeID]; exists && count > 0 {
		log.Info().
			Str("node", nodeID).
			Int("previousFailures", count).
			Msg("Authentication succeeded, resetting failure count")

		delete(m.authFailures, nodeID)
		delete(m.lastAuthAttempt, nodeID)
	}
}

// removeFailedPVENode updates a PVE node to show failed authentication status
func (m *Monitor) removeFailedPVENode(instanceName string) {
	// Get instance config to get host URL
	var hostURL string
	for _, cfg := range m.config.PVEInstances {
		if cfg.Name == instanceName {
			hostURL = cfg.Host
			break
		}
	}

	// Create a failed node entry to show in UI with error status
	failedNode := models.Node{
		ID:               instanceName + "-failed",
		Name:             instanceName,
		DisplayName:      instanceName,
		Instance:         instanceName,
		Host:             hostURL, // Include host URL even for failed nodes
		Status:           "offline",
		Type:             "node",
		ConnectionHealth: "error",
		LastSeen:         time.Now(),
		// Set other fields to zero values to indicate no data
		CPU:    0,
		Memory: models.Memory{},
		Disk:   models.Disk{},
	}

	// Update with just the failed node
	m.state.UpdateNodesForInstance(instanceName, []models.Node{failedNode})

	// Remove all other resources associated with this instance
	m.state.UpdateVMsForInstance(instanceName, []models.VM{})
	m.state.UpdateContainersForInstance(instanceName, []models.Container{})
	m.state.UpdateStorageForInstance(instanceName, []models.Storage{})
	m.state.UpdateCephClustersForInstance(instanceName, []models.CephCluster{})
	m.state.UpdateBackupTasksForInstance(instanceName, []models.BackupTask{})
	m.state.UpdateStorageBackupsForInstance(instanceName, []models.StorageBackup{})
	m.state.UpdateGuestSnapshotsForInstance(instanceName, []models.GuestSnapshot{})

	// Set connection health to false
	m.state.SetConnectionHealth(instanceName, false)
}

// removeFailedPBSNode removes a PBS node and all its resources from state
func (m *Monitor) removeFailedPBSNode(instanceName string) {
	// Remove PBS instance by passing empty array
	currentInstances := m.state.PBSInstances
	var updatedInstances []models.PBSInstance
	for _, inst := range currentInstances {
		if inst.Name != instanceName {
			updatedInstances = append(updatedInstances, inst)
		}
	}
	m.state.UpdatePBSInstances(updatedInstances)

	// Remove PBS backups
	m.state.UpdatePBSBackups(instanceName, []models.PBSBackup{})

	// Set connection health to false
	m.state.SetConnectionHealth("pbs-"+instanceName, false)
}

// removeFailedPMGInstance removes PMG data from state when authentication fails repeatedly
func (m *Monitor) removeFailedPMGInstance(instanceName string) {
	currentInstances := m.state.PMGInstances
	updated := make([]models.PMGInstance, 0, len(currentInstances))
	for _, inst := range currentInstances {
		if inst.Name != instanceName {
			updated = append(updated, inst)
		}
	}

	m.state.UpdatePMGInstances(updated)
	m.state.UpdatePMGBackups(instanceName, nil)
	m.state.SetConnectionHealth("pmg-"+instanceName, false)
}

type pbsBackupGroupKey struct {
	datastore  string
	namespace  string
	backupType string
	backupID   string
}

type cachedPBSGroup struct {
	snapshots []models.PBSBackup
	latest    time.Time
}

type pbsBackupFetchRequest struct {
	datastore string
	namespace string
	group     pbs.BackupGroup
	cached    cachedPBSGroup
}

// pollPBSBackups fetches all backups from PBS datastores
func (m *Monitor) pollPBSBackups(ctx context.Context, instanceName string, client *pbs.Client, datastores []models.PBSDatastore) {
	log.Debug().Str("instance", instanceName).Msg("Polling PBS backups")

	// Cache existing PBS backups so we can avoid redundant API calls when no changes occurred.
	existingGroups := m.buildPBSBackupCache(instanceName)

	var allBackups []models.PBSBackup
	datastoreCount := len(datastores) // Number of datastores to query
	datastoreFetches := 0             // Number of successful datastore fetches
	datastoreErrors := 0              // Number of failed datastore fetches

	// Process each datastore
	for _, ds := range datastores {
		if ctx.Err() != nil {
			log.Warn().
				Str("instance", instanceName).
				Msg("PBS backup polling cancelled before completion")
			return
		}

		namespacePaths := namespacePathsForDatastore(ds)

		log.Info().
			Str("instance", instanceName).
			Str("datastore", ds.Name).
			Int("namespaces", len(namespacePaths)).
			Strs("namespace_paths", namespacePaths).
			Msg("Processing datastore namespaces")

		datastoreHadSuccess := false
		groupsReused := 0
		groupsRequested := 0

		for _, namespace := range namespacePaths {
			if ctx.Err() != nil {
				log.Warn().
					Str("instance", instanceName).
					Msg("PBS backup polling cancelled mid-datastore")
				return
			}

			groups, err := client.ListBackupGroups(ctx, ds.Name, namespace)
			if err != nil {
				log.Error().
					Err(err).
					Str("instance", instanceName).
					Str("datastore", ds.Name).
					Str("namespace", namespace).
					Msg("Failed to list PBS backup groups")
				continue
			}

			datastoreHadSuccess = true
			requests := make([]pbsBackupFetchRequest, 0, len(groups))

			for _, group := range groups {
				key := pbsBackupGroupKey{
					datastore:  ds.Name,
					namespace:  namespace,
					backupType: group.BackupType,
					backupID:   group.BackupID,
				}
				cached := existingGroups[key]

				// Group deleted (no backups left) - ensure cached data is dropped.
				if group.BackupCount == 0 {
					continue
				}

				lastBackupTime := time.Unix(group.LastBackup, 0)
				hasCachedData := len(cached.snapshots) > 0
				// Only re-fetch when the backup count changes or the most recent backup is newer.
				if hasCachedData &&
					len(cached.snapshots) == group.BackupCount &&
					!lastBackupTime.After(cached.latest) {

					allBackups = append(allBackups, cached.snapshots...)
					groupsReused++
					continue
				}

				requests = append(requests, pbsBackupFetchRequest{
					datastore: ds.Name,
					namespace: namespace,
					group:     group,
					cached:    cached,
				})
			}

			if len(requests) == 0 {
				continue
			}

			groupsRequested += len(requests)
			fetched := m.fetchPBSBackupSnapshots(ctx, client, instanceName, requests)
			if len(fetched) > 0 {
				allBackups = append(allBackups, fetched...)
			}
		}

		if datastoreHadSuccess {
			datastoreFetches++
			log.Info().
				Str("instance", instanceName).
				Str("datastore", ds.Name).
				Int("namespaces", len(namespacePaths)).
				Int("groups_reused", groupsReused).
				Int("groups_refreshed", groupsRequested).
				Msg("PBS datastore processed")
		} else {
			// Preserve cached data for this datastore if we couldn't fetch anything new.
			log.Warn().
				Str("instance", instanceName).
				Str("datastore", ds.Name).
				Msg("No namespaces succeeded for PBS datastore; using cached backups")
			for key, entry := range existingGroups {
				if key.datastore != ds.Name || len(entry.snapshots) == 0 {
					continue
				}
				allBackups = append(allBackups, entry.snapshots...)
			}
			datastoreErrors++
		}
	}

	log.Info().
		Str("instance", instanceName).
		Int("count", len(allBackups)).
		Msg("PBS backups fetched")

	// Decide whether to keep existing backups when all queries failed
	if shouldPreservePBSBackups(datastoreCount, datastoreFetches) {
		log.Warn().
			Str("instance", instanceName).
			Int("datastores", datastoreCount).
			Int("errors", datastoreErrors).
			Msg("All PBS datastore queries failed; keeping previous backup list")
		return
	}

	// Update state
	m.state.UpdatePBSBackups(instanceName, allBackups)

	// Sync backup times to VMs/Containers for backup status indicators
	m.state.SyncGuestBackupTimes()

	if m.alertManager != nil {
		snapshot := m.state.GetSnapshot()
		guestsByKey, guestsByVMID := buildGuestLookups(snapshot, m.guestMetadataStore)
		pveStorage := snapshot.Backups.PVE.StorageBackups
		if len(pveStorage) == 0 && len(snapshot.PVEBackups.StorageBackups) > 0 {
			pveStorage = snapshot.PVEBackups.StorageBackups
		}
		pbsBackups := snapshot.Backups.PBS
		if len(pbsBackups) == 0 && len(snapshot.PBSBackups) > 0 {
			pbsBackups = snapshot.PBSBackups
		}
		pmgBackups := snapshot.Backups.PMG
		if len(pmgBackups) == 0 && len(snapshot.PMGBackups) > 0 {
			pmgBackups = snapshot.PMGBackups
		}
		m.alertManager.CheckBackups(pveStorage, pbsBackups, pmgBackups, guestsByKey, guestsByVMID)
	}
}

func (m *Monitor) buildPBSBackupCache(instanceName string) map[pbsBackupGroupKey]cachedPBSGroup {
	snapshot := m.state.GetSnapshot()
	cache := make(map[pbsBackupGroupKey]cachedPBSGroup)
	for _, backup := range snapshot.PBSBackups {
		if backup.Instance != instanceName {
			continue
		}
		key := pbsBackupGroupKey{
			datastore:  backup.Datastore,
			namespace:  normalizePBSNamespacePath(backup.Namespace),
			backupType: backup.BackupType,
			backupID:   backup.VMID,
		}
		entry := cache[key]
		entry.snapshots = append(entry.snapshots, backup)
		if backup.BackupTime.After(entry.latest) {
			entry.latest = backup.BackupTime
		}
		cache[key] = entry
	}
	return cache
}

func normalizePBSNamespacePath(ns string) string {
	if ns == "/" {
		return ""
	}
	return ns
}

func namespacePathsForDatastore(ds models.PBSDatastore) []string {
	if len(ds.Namespaces) == 0 {
		return []string{""}
	}

	seen := make(map[string]struct{}, len(ds.Namespaces))
	var paths []string
	for _, ns := range ds.Namespaces {
		path := normalizePBSNamespacePath(ns.Path)
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		paths = append(paths, path)
	}
	return paths
}

func (m *Monitor) fetchPBSBackupSnapshots(ctx context.Context, client *pbs.Client, instanceName string, requests []pbsBackupFetchRequest) []models.PBSBackup {
	if len(requests) == 0 {
		return nil
	}

	results := make(chan []models.PBSBackup, len(requests))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 5)

	for _, req := range requests {
		req := req
		wg.Add(1)
		go func() {
			defer wg.Done()

			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()

			log.Debug().
				Str("instance", instanceName).
				Str("datastore", req.datastore).
				Str("namespace", req.namespace).
				Str("type", req.group.BackupType).
				Str("id", req.group.BackupID).
				Msg("Refreshing PBS backup group")

			snapshots, err := client.ListBackupSnapshots(ctx, req.datastore, req.namespace, req.group.BackupType, req.group.BackupID)
			if err != nil {
				log.Error().
					Err(err).
					Str("instance", instanceName).
					Str("datastore", req.datastore).
					Str("namespace", req.namespace).
					Str("type", req.group.BackupType).
					Str("id", req.group.BackupID).
					Msg("Failed to list PBS backup snapshots")

				if len(req.cached.snapshots) > 0 {
					results <- req.cached.snapshots
				}
				return
			}

			results <- convertPBSSnapshots(instanceName, req.datastore, req.namespace, snapshots)
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var combined []models.PBSBackup
	for backups := range results {
		if len(backups) == 0 {
			continue
		}
		combined = append(combined, backups...)
	}

	return combined
}

func convertPBSSnapshots(instanceName, datastore, namespace string, snapshots []pbs.BackupSnapshot) []models.PBSBackup {
	backups := make([]models.PBSBackup, 0, len(snapshots))
	for _, snapshot := range snapshots {
		backupTime := time.Unix(snapshot.BackupTime, 0)
		id := fmt.Sprintf("pbs-%s-%s-%s-%s-%s-%d",
			instanceName, datastore, namespace,
			snapshot.BackupType, snapshot.BackupID,
			snapshot.BackupTime)

		var fileNames []string
		for _, file := range snapshot.Files {
			switch f := file.(type) {
			case string:
				fileNames = append(fileNames, f)
			case map[string]interface{}:
				if filename, ok := f["filename"].(string); ok {
					fileNames = append(fileNames, filename)
				}
			}
		}

		verified := false
		if snapshot.Verification != nil {
			switch v := snapshot.Verification.(type) {
			case string:
				verified = v == "ok"
			case map[string]interface{}:
				if state, ok := v["state"].(string); ok {
					verified = state == "ok"
				}
			}

			log.Debug().
				Str("vmid", snapshot.BackupID).
				Int64("time", snapshot.BackupTime).
				Interface("verification", snapshot.Verification).
				Bool("verified", verified).
				Msg("PBS backup verification status")
		}

		backups = append(backups, models.PBSBackup{
			ID:         id,
			Instance:   instanceName,
			Datastore:  datastore,
			Namespace:  namespace,
			BackupType: snapshot.BackupType,
			VMID:       snapshot.BackupID,
			BackupTime: backupTime,
			Size:       snapshot.Size,
			Protected:  snapshot.Protected,
			Verified:   verified,
			Comment:    snapshot.Comment,
			Files:      fileNames,
			Owner:      snapshot.Owner,
		})
	}

	return backups
}

// checkMockAlerts checks alerts for mock data
func (m *Monitor) checkMockAlerts() {
	defer recoverFromPanic("checkMockAlerts")

	log.Info().Bool("mockEnabled", mock.IsMockEnabled()).Msg("checkMockAlerts called")
	if !mock.IsMockEnabled() {
		log.Info().Msg("Mock mode not enabled, skipping mock alert check")
		return
	}

	// Get mock state
	state := mock.GetMockState()

	log.Info().
		Int("vms", len(state.VMs)).
		Int("containers", len(state.Containers)).
		Int("nodes", len(state.Nodes)).
		Msg("Checking alerts for mock data")

	// Clean up alerts for nodes that no longer exist
	existingNodes := make(map[string]bool)
	for _, node := range state.Nodes {
		existingNodes[node.Name] = true
		if node.Host != "" {
			existingNodes[node.Host] = true
		}
	}
	for _, pbsInst := range state.PBSInstances {
		existingNodes[pbsInst.Name] = true
		existingNodes["pbs-"+pbsInst.Name] = true
		if pbsInst.Host != "" {
			existingNodes[pbsInst.Host] = true
		}
	}
	log.Info().
		Int("trackedNodes", len(existingNodes)).
		Msg("Collecting resources for alert cleanup in mock mode")
	m.alertManager.CleanupAlertsForNodes(existingNodes)

	guestsByKey, guestsByVMID := buildGuestLookups(state, m.guestMetadataStore)
	pveStorage := state.Backups.PVE.StorageBackups
	if len(pveStorage) == 0 && len(state.PVEBackups.StorageBackups) > 0 {
		pveStorage = state.PVEBackups.StorageBackups
	}
	pbsBackups := state.Backups.PBS
	if len(pbsBackups) == 0 && len(state.PBSBackups) > 0 {
		pbsBackups = state.PBSBackups
	}
	pmgBackups := state.Backups.PMG
	if len(pmgBackups) == 0 && len(state.PMGBackups) > 0 {
		pmgBackups = state.PMGBackups
	}
	m.alertManager.CheckBackups(pveStorage, pbsBackups, pmgBackups, guestsByKey, guestsByVMID)

	// Limit how many guests we check per cycle to prevent blocking with large datasets
	const maxGuestsPerCycle = 50
	guestsChecked := 0

	// Check alerts for VMs (up to limit)
	for _, vm := range state.VMs {
		if guestsChecked >= maxGuestsPerCycle {
			log.Debug().
				Int("checked", guestsChecked).
				Int("total", len(state.VMs)+len(state.Containers)).
				Msg("Reached guest check limit for this cycle")
			break
		}
		m.alertManager.CheckGuest(vm, "mock")
		guestsChecked++
	}

	// Check alerts for containers (if we haven't hit the limit)
	for _, container := range state.Containers {
		if guestsChecked >= maxGuestsPerCycle {
			break
		}
		m.alertManager.CheckGuest(container, "mock")
		guestsChecked++
	}

	// Check alerts for each node
	for _, node := range state.Nodes {
		m.alertManager.CheckNode(node)
	}

	// Check alerts for storage
	log.Info().Int("storageCount", len(state.Storage)).Msg("Checking storage alerts")
	for _, storage := range state.Storage {
		log.Debug().
			Str("name", storage.Name).
			Float64("usage", storage.Usage).
			Msg("Checking storage for alerts")
		m.alertManager.CheckStorage(storage)
	}

	// Check alerts for PBS instances
	log.Info().Int("pbsCount", len(state.PBSInstances)).Msg("Checking PBS alerts")
	for _, pbsInst := range state.PBSInstances {
		m.alertManager.CheckPBS(pbsInst)
	}

	// Check alerts for PMG instances
	log.Info().Int("pmgCount", len(state.PMGInstances)).Msg("Checking PMG alerts")
	for _, pmgInst := range state.PMGInstances {
		m.alertManager.CheckPMG(pmgInst)
	}

	// Cache the latest alert snapshots directly in the mock data so the API can serve
	// mock state without needing to grab the alert manager lock again.
	mock.UpdateAlertSnapshots(m.alertManager.GetActiveAlerts(), m.alertManager.GetRecentlyResolved())
}
func isLegacyHostAgent(agentType string) bool {
	// Unified agent reports type="unified"
	// Legacy standalone agents have empty type
	return agentType != "unified"
}

func isLegacyDockerAgent(agentType string) bool {
	// Unified agent reports type="unified"
	// Legacy standalone agents have empty type
	return agentType != "unified"
}

// convertAgentCephToModels converts agent report Ceph data to the models.HostCephCluster format.
func convertAgentCephToModels(ceph *agentshost.CephCluster) *models.HostCephCluster {
	if ceph == nil {
		return nil
	}

	collectedAt, _ := time.Parse(time.RFC3339, ceph.CollectedAt)

	result := &models.HostCephCluster{
		FSID: ceph.FSID,
		Health: models.HostCephHealth{
			Status: ceph.Health.Status,
			Checks: make(map[string]models.HostCephCheck),
		},
		MonMap: models.HostCephMonitorMap{
			Epoch:   ceph.MonMap.Epoch,
			NumMons: ceph.MonMap.NumMons,
		},
		MgrMap: models.HostCephManagerMap{
			Available: ceph.MgrMap.Available,
			NumMgrs:   ceph.MgrMap.NumMgrs,
			ActiveMgr: ceph.MgrMap.ActiveMgr,
			Standbys:  ceph.MgrMap.Standbys,
		},
		OSDMap: models.HostCephOSDMap{
			Epoch:   ceph.OSDMap.Epoch,
			NumOSDs: ceph.OSDMap.NumOSDs,
			NumUp:   ceph.OSDMap.NumUp,
			NumIn:   ceph.OSDMap.NumIn,
			NumDown: ceph.OSDMap.NumDown,
			NumOut:  ceph.OSDMap.NumOut,
		},
		PGMap: models.HostCephPGMap{
			NumPGs:           ceph.PGMap.NumPGs,
			BytesTotal:       ceph.PGMap.BytesTotal,
			BytesUsed:        ceph.PGMap.BytesUsed,
			BytesAvailable:   ceph.PGMap.BytesAvailable,
			DataBytes:        ceph.PGMap.DataBytes,
			UsagePercent:     ceph.PGMap.UsagePercent,
			DegradedRatio:    ceph.PGMap.DegradedRatio,
			MisplacedRatio:   ceph.PGMap.MisplacedRatio,
			ReadBytesPerSec:  ceph.PGMap.ReadBytesPerSec,
			WriteBytesPerSec: ceph.PGMap.WriteBytesPerSec,
			ReadOpsPerSec:    ceph.PGMap.ReadOpsPerSec,
			WriteOpsPerSec:   ceph.PGMap.WriteOpsPerSec,
		},
		CollectedAt: collectedAt,
	}

	// Convert monitors
	for _, mon := range ceph.MonMap.Monitors {
		result.MonMap.Monitors = append(result.MonMap.Monitors, models.HostCephMonitor{
			Name:   mon.Name,
			Rank:   mon.Rank,
			Addr:   mon.Addr,
			Status: mon.Status,
		})
	}

	// Convert health checks
	for name, check := range ceph.Health.Checks {
		result.Health.Checks[name] = models.HostCephCheck{
			Severity: check.Severity,
			Message:  check.Message,
			Detail:   check.Detail,
		}
	}

	// Convert health summary
	for _, s := range ceph.Health.Summary {
		result.Health.Summary = append(result.Health.Summary, models.HostCephHealthSummary{
			Severity: s.Severity,
			Message:  s.Message,
		})
	}

	// Convert pools
	for _, pool := range ceph.Pools {
		result.Pools = append(result.Pools, models.HostCephPool{
			ID:             pool.ID,
			Name:           pool.Name,
			BytesUsed:      pool.BytesUsed,
			BytesAvailable: pool.BytesAvailable,
			Objects:        pool.Objects,
			PercentUsed:    pool.PercentUsed,
		})
	}

	// Convert services
	for _, svc := range ceph.Services {
		result.Services = append(result.Services, models.HostCephService{
			Type:    svc.Type,
			Running: svc.Running,
			Total:   svc.Total,
			Daemons: svc.Daemons,
		})
	}

	return result
}

// convertAgentCephToGlobalCluster converts agent Ceph data to the global CephCluster format
// used by the State.CephClusters list.
func convertAgentCephToGlobalCluster(ceph *agentshost.CephCluster, hostname, hostID string, timestamp time.Time) models.CephCluster {
	// Use FSID as the primary ID since it's unique per Ceph cluster
	id := ceph.FSID
	if id == "" {
		id = "agent-ceph-" + hostID
	}

	cluster := models.CephCluster{
		ID:             id,
		Instance:       "agent:" + hostname,
		Name:           hostname + " Ceph",
		FSID:           ceph.FSID,
		Health:         strings.TrimPrefix(ceph.Health.Status, "HEALTH_"),
		TotalBytes:     int64(ceph.PGMap.BytesTotal),
		UsedBytes:      int64(ceph.PGMap.BytesUsed),
		AvailableBytes: int64(ceph.PGMap.BytesAvailable),
		UsagePercent:   ceph.PGMap.UsagePercent,
		NumMons:        ceph.MonMap.NumMons,
		NumMgrs:        ceph.MgrMap.NumMgrs,
		NumOSDs:        ceph.OSDMap.NumOSDs,
		NumOSDsUp:      ceph.OSDMap.NumUp,
		NumOSDsIn:      ceph.OSDMap.NumIn,
		NumPGs:         ceph.PGMap.NumPGs,
		LastUpdated:    timestamp,
	}

	// Build health message from checks
	var healthMessages []string
	for _, check := range ceph.Health.Checks {
		if check.Message != "" {
			healthMessages = append(healthMessages, check.Message)
		}
	}
	if len(healthMessages) > 0 {
		cluster.HealthMessage = strings.Join(healthMessages, "; ")
	}

	// Convert pools
	for _, pool := range ceph.Pools {
		cluster.Pools = append(cluster.Pools, models.CephPool{
			ID:             pool.ID,
			Name:           pool.Name,
			StoredBytes:    int64(pool.BytesUsed),
			AvailableBytes: int64(pool.BytesAvailable),
			Objects:        int64(pool.Objects),
			PercentUsed:    pool.PercentUsed,
		})
	}

	// Convert services
	for _, svc := range ceph.Services {
		cluster.Services = append(cluster.Services, models.CephServiceStatus{
			Type:    svc.Type,
			Running: svc.Running,
			Total:   svc.Total,
		})
	}

	return cluster
}
