package alerts

import (
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/reducer"
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
	// Intent-policy pending state retains server timestamps, accumulated
	// monotonic elapsed time, and transient-context evidence for policy-enabled
	// candidates. It is keyed by canonical alert tracking key and persisted
	// with the alert manager's transition state.
	intentPending          map[string]IntentPendingState
	intentRuntimeTicks     map[string]time.Duration
	intentClock            func() time.Duration
	intentPolicies         AlertIntentPolicyDocument
	operatorIntentResolver OperatorIntentContextResolver
	backupIntentResolver   BackupIntentContextResolver
	resourceIntentResolver ResourceIntentIdentityResolver
	// Offline confirmation tracking
	// core is the authoritative transition state for the canonical
	// lifecycle (match-spec) family: the deterministic reducer owns
	// confirmations, pending runs, first-matched anchoring, recovery
	// gates, re-fire retention, and ack restoration for alerts that flow
	// through evaluateCanonicalLifecycleAlert and the poll-driven recovery
	// paths (docs/ALERT_ENGINE_EVOLUTION.md, Phase 2). Access under m.mu.
	core                         *reducer.State
	unifiedIncidentConfirmations map[string]int                  // Track consecutive provider-incident observations before activation
	unifiedIncidentFirstSeen     map[string]time.Time            // Preserve the first confirmed observation as lifecycle start
	unifiedIncidentRecoveries    map[string]int                  // Track consecutive healthy observations before provider-incident recovery
	dockerRestartTracking        map[string]*dockerRestartRecord // Track restart counts and times for restart loop detection
	dockerUpdateFirstSeen        map[string]time.Time            // Track when image updates were first detected for alert delay
	// Stable identity tracking prevents update-delay resets when host IDs churn.
	dockerUpdateFirstSeenByIdentity map[string]time.Time
	// PMG quarantine growth tracking
	pmgQuarantineHistory map[string][]pmgQuarantineSnapshot // Track quarantine snapshots for growth detection
	// SMART counter snapshots let alert evaluation distinguish historical
	// counters from new disk errors. They are keyed by the canonical disk
	// resource ID and intentionally retain only the latest observation.
	smartCounterSnapshots map[string]smartCounterSnapshot
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

	// Append-only alert event log (transitions + notification decisions).
	// Nil until EnableEventLog/SetEventLog; recording is then a no-op.
	eventLog atomic.Pointer[eventlog.Store]
	// eventHistoryAuthoritative becomes true only after legacy JSON history is
	// absent or has been durably imported. Reads keep using JSON while migration
	// is incomplete or the event store reports a write failure.
	eventHistoryAuthoritative atomic.Bool

	// Shadow-mode reducer feed (Phase 1 capstone). Nil until
	// EnableShadowFeed; all access is under m.mu.
	shadow *shadowFeed

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

// ManagerOption adjusts how a Manager is constructed.
type ManagerOption func(*managerOptions)

type managerOptions struct {
	skipPersistedAlertRestore bool
}

// WithoutPersistedAlertRestore starts the manager with an empty active-alert
// set instead of restoring active-alerts.json from disk. Mock mode uses this so
// a demo session never resurfaces alerts raised against real infrastructure:
// SetMockMode already clears active alerts when the toggle flips, but a process
// that starts with mock mode already enabled never runs that path and would
// otherwise reload the persisted real alerts.
func WithoutPersistedAlertRestore() ManagerOption {
	return func(opts *managerOptions) {
		opts.skipPersistedAlertRestore = true
	}
}

// NewManagerWithDataDir creates a new alert manager with a custom data directory.
// This enables tenant-scoped alert persistence in multi-tenant deployments.
func NewManagerWithDataDir(dataDir string, options ...ManagerOption) *Manager {
	opts := managerOptions{}
	for _, option := range options {
		if option != nil {
			option(&opts)
		}
	}

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
		intentPending:                   make(map[string]IntentPendingState),
		intentRuntimeTicks:              make(map[string]time.Duration),
		intentPolicies:                  NewAlertIntentPolicyDocument(),
		core:                            reducer.NewState(),
		unifiedIncidentConfirmations:    make(map[string]int),
		unifiedIncidentFirstSeen:        make(map[string]time.Time),
		unifiedIncidentRecoveries:       make(map[string]int),
		dockerRestartTracking:           make(map[string]*dockerRestartRecord),
		dockerUpdateFirstSeen:           make(map[string]time.Time),
		dockerUpdateFirstSeenByIdentity: make(map[string]time.Time),
		pmgQuarantineHistory:            make(map[string][]pmgQuarantineSnapshot),
		smartCounterSnapshots:           make(map[string]smartCounterSnapshot),
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
	intentClockEpoch := time.Now()
	m.intentClock = func() time.Duration {
		return time.Since(intentClockEpoch)
	}

	// Load saved active alerts
	if opts.skipPersistedAlertRestore {
		log.Info().Msg("skipping persisted active alert restore")
	} else if err := m.LoadActiveAlerts(); err != nil {
		log.Error().Err(err).Msg("failed to load active alerts")
	}

	// Seed the authoritative reducer core from restored alerts so a restart
	// resumes firing incidents instead of re-running their confirmations.
	m.seedReducerCoreNoLock()

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
