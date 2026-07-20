package alerts

import (
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rs/zerolog/log"
)

// Manager handles alert monitoring and state
//
// Lock Ordering Documentation:
// The Manager uses two mutexes to prevent deadlocks:
//  1. m.mu (primary lock) - protects most manager state
//  2. m.resolvedMutex - protects only the recentlyResolved and resolvedAlias maps
//
// Lock Ordering Rules:
//   - resolvedMutex is subordinate to m.mu: it MAY be acquired while holding
//     m.mu (the cleanup and canonical-eval paths do), but NEVER acquire m.mu
//     while holding resolvedMutex
//   - keep resolvedMutex critical sections to map access only; never call
//     dispatch, history, or notification code while holding it
//   - every access to recentlyResolved or resolvedAlias must hold
//     resolvedMutex, and writers need the write lock: getResolvedAlertNoLock
//     can backfill resolvedAlias, so even lookups are potential writes
//
// This ordering prevents deadlock scenarios where different goroutines acquire locks in different orders.
type Manager struct {
	mu               sync.RWMutex
	saveMu           sync.Mutex
	callbacks        callbackBus
	alertsDir        string
	config           AlertConfig
	activeAlerts     map[string]*Alert
	activeAlertAlias map[string]string
	historyManager   *HistoryManager
	escalationStop   chan struct{}
	alertRateLimit   map[string][]time.Time // Track alert times for rate limiting
	// New fields for deduplication and suppression
	recentAlerts    map[string]*Alert    // Track recent alerts for deduplication
	suppressedUntil map[string]time.Time // Track suppression windows
	// Recently resolved alerts (kept for 5 minutes)
	recentlyResolved map[string]*ResolvedAlert
	resolvedAlias    map[string]string
	resolvedMutex    sync.RWMutex // Secondary lock - see Lock Ordering Documentation above
	// Time threshold tracking
	pendingAlerts map[string]time.Time // Track when thresholds were first exceeded
	// Intent-policy pending state retains wall-clock and transient-context
	// evidence for policy-enabled candidates. It is keyed by canonical alert
	// tracking key and persisted with the alert manager's transition state.
	intentPending          map[string]IntentPendingState
	intentPolicies         AlertIntentPolicyDocument
	operatorIntentResolver OperatorIntentContextResolver
	backupIntentResolver   BackupIntentContextResolver
	resourceIntentResolver ResourceIntentIdentityResolver
	// Offline confirmation tracking
	nodeOfflineCount             map[string]int                  // Track consecutive offline counts for nodes (legacy)
	connectionDegradedCount      map[string]int                  // Track consecutive degraded counts for platform connections (pve/pbs/pmg/vmware/truenas)
	offlineConfirmations         map[string]int                  // Track consecutive offline counts for all resources
	offlineRecoveryConfirmations map[string]int                  // Track consecutive healthy confirmations before clearing poll-driven offline alerts
	dockerOfflineCount           map[string]int                  // Track consecutive offline counts for Docker hosts
	dockerStateConfirm           map[string]int                  // Track consecutive state confirmations for Docker containers
	dockerRestartTracking        map[string]*dockerRestartRecord // Track restart counts and times for restart loop detection
	dockerUpdateFirstSeen        map[string]time.Time            // Track when image updates were first detected for alert delay
	// Stable identity tracking prevents update-delay resets when host IDs churn.
	dockerUpdateFirstSeenByIdentity map[string]time.Time
	// PMG quarantine growth tracking
	pmgQuarantineHistory map[string][]pmgQuarantineSnapshot // Track quarantine snapshots for growth detection
	// PMG anomaly detection tracking
	pmgAnomalyTrackers map[string]*pmgAnomalyTracker // Track mail metrics for anomaly detection per PMG instance
	// Persistent acknowledgement state so quick alert rebuilds keep user acknowledgements
	ackState map[string]ackRecord
	// Canonical acknowledgement state is keyed by resource_id + spec_id so later
	// alert-ID migration can preserve user state across storage-key changes.
	ackStateByCanonical map[string]ackRecord
	// Flapping detection tracking
	flappingHistory map[string][]time.Time // Track state change times for flapping detection
	flappingActive  map[string]bool        // Track which alerts are currently in flapping state
	// Cleanup control
	cleanupStop chan struct{} // Signal to stop cleanup goroutine
	// Host agent deduplication: track hostnames of active host agents
	// When a host agent is running on a Proxmox node, we prefer the host agent
	// alerts and suppress the node alerts to avoid duplicate monitoring.
	hostAgentHostnames map[string]struct{} // Normalized hostnames (lowercase)
	// Node display name caches. Proxmox nodes can share the same raw node name
	// across multiple configured instances, so keep instance-scoped entries in
	// addition to the legacy raw-name cache used by instance-less resources.
	nodeDisplayNames         map[string]string
	instanceNodeDisplayNames map[string]string
	// License checking for Pro-only alert features
	hasProFeature func(feature string) bool

	// Cached timezone for quiet hours
	quietHoursLoc *time.Location
	now           func() time.Time
	stopOnce      sync.Once
	stopMu        sync.RWMutex
	stopping      bool
	workerWG      sync.WaitGroup
}

type ackRecord struct {
	acknowledged bool
	user         string
	time         time.Time // When the alert was acknowledged
	inactiveAt   time.Time // When the alert was removed (zero if still active)
}

// NewManager creates a new alert manager using the global data directory.
// For multi-tenant deployments, use NewManagerWithDataDir instead.
func NewManager() *Manager {
	return NewManagerWithDataDir(utils.GetDataDir())
}

// NewManagerWithDataDir creates a new alert manager with a custom data directory.
// This enables tenant-scoped alert persistence in multi-tenant deployments.
func NewManagerWithDataDir(dataDir string) *Manager {
	if strings.TrimSpace(dataDir) == "" {
		dataDir = utils.GetDataDir()
	}

	alertsDir := filepath.Join(dataDir, "alerts")
	m := &Manager{
		alertsDir:                       alertsDir,
		activeAlerts:                    make(map[string]*Alert),
		activeAlertAlias:                make(map[string]string),
		historyManager:                  NewHistoryManager(alertsDir),
		callbacks:                       newCallbackBus(),
		escalationStop:                  make(chan struct{}),
		alertRateLimit:                  make(map[string][]time.Time),
		recentAlerts:                    make(map[string]*Alert),
		suppressedUntil:                 make(map[string]time.Time),
		recentlyResolved:                make(map[string]*ResolvedAlert),
		resolvedAlias:                   make(map[string]string),
		pendingAlerts:                   make(map[string]time.Time),
		intentPending:                   make(map[string]IntentPendingState),
		intentPolicies:                  NewAlertIntentPolicyDocument(),
		nodeOfflineCount:                make(map[string]int),
		connectionDegradedCount:         make(map[string]int),
		offlineConfirmations:            make(map[string]int),
		offlineRecoveryConfirmations:    make(map[string]int),
		dockerOfflineCount:              make(map[string]int),
		dockerStateConfirm:              make(map[string]int),
		dockerRestartTracking:           make(map[string]*dockerRestartRecord),
		dockerUpdateFirstSeen:           make(map[string]time.Time),
		dockerUpdateFirstSeenByIdentity: make(map[string]time.Time),
		pmgQuarantineHistory:            make(map[string][]pmgQuarantineSnapshot),
		pmgAnomalyTrackers:              make(map[string]*pmgAnomalyTracker),
		ackState:                        make(map[string]ackRecord),
		ackStateByCanonical:             make(map[string]ackRecord),
		flappingHistory:                 make(map[string][]time.Time),
		flappingActive:                  make(map[string]bool),
		cleanupStop:                     make(chan struct{}),
		hostAgentHostnames:              make(map[string]struct{}),
		nodeDisplayNames:                make(map[string]string),
		instanceNodeDisplayNames:        make(map[string]string),
		now:                             time.Now,
		config:                          defaultAlertConfig(),
	}

	// Load saved active alerts
	if err := m.LoadActiveAlerts(); err != nil {
		log.Error().Err(err).Msg("failed to load active alerts")
	}

	// Start background workers.
	m.workerWG.Add(3)
	go func() {
		defer m.workerWG.Done()
		m.escalationChecker()
	}()
	go func() {
		defer m.workerWG.Done()
		m.periodicSaveAlerts()
	}()
	go func() {
		defer m.workerWG.Done()
		m.trackingMapCleanup()
	}()

	return m
}

// SetLicenseChecker sets the function used to check Pro license features.
// This enables gating Pro-only alert features like update alerts.
func (m *Manager) SetLicenseChecker(checker func(feature string) bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hasProFeature = checker
}
