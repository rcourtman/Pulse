package alerts

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	alertconfig "github.com/rcourtman/pulse-go-rewrite/internal/alerts/config"
	alertspecs "github.com/rcourtman/pulse-go-rewrite/internal/alerts/specs"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rs/zerolog/log"
)

// Type aliases re-exported from alerts/config for backward compatibility.
// These guarantee compile-time type identity: alerts.AlertLevel = alertconfig.AlertLevel.
type AlertLevel = alertconfig.AlertLevel
type ActivationState = alertconfig.ActivationState
type HysteresisThreshold = alertconfig.HysteresisThreshold
type ThresholdConfig = alertconfig.ThresholdConfig
type QuietHours = alertconfig.QuietHours
type QuietHoursSuppression = alertconfig.QuietHoursSuppression
type EscalationLevel = alertconfig.EscalationLevel
type EscalationConfig = alertconfig.EscalationConfig
type GroupingConfig = alertconfig.GroupingConfig
type ScheduleConfig = alertconfig.ScheduleConfig
type FilterCondition = alertconfig.FilterCondition
type FilterStack = alertconfig.FilterStack
type CustomAlertRule = alertconfig.CustomAlertRule
type DockerThresholdConfig = alertconfig.DockerThresholdConfig
type PMGThresholdConfig = alertconfig.PMGThresholdConfig
type SnapshotAlertConfig = alertconfig.SnapshotAlertConfig
type BackupAlertConfig = alertconfig.BackupAlertConfig
type GuestLookup = alertconfig.GuestLookup
type AlertConfig = alertconfig.AlertConfig

const (
	AlertLevelWarning  = alertconfig.AlertLevelWarning
	AlertLevelCritical = alertconfig.AlertLevelCritical
	ActivationPending  = alertconfig.ActivationPending
	ActivationActive   = alertconfig.ActivationActive
	ActivationSnoozed  = alertconfig.ActivationSnoozed
)

var ErrAlertNotFound = errors.New("alert not found")

func NormalizeAlertConfigAliases(config *AlertConfig) {
	alertconfig.NormalizeAlertConfigAliases(config)
}
func NormalizeMetricTimeThresholds(input map[string]map[string]int) map[string]map[string]int {
	return alertconfig.NormalizeMetricTimeThresholds(input)
}
func NormalizeDockerIgnoredPrefixes(prefixes []string) []string {
	return alertconfig.NormalizeDockerIgnoredPrefixes(prefixes)
}
func CanonicalResourceTypeKeys(resourceType string) []string {
	return alertconfig.CanonicalResourceTypeKeys(resourceType)
}
func NormalizePoweredOffSeverity(level AlertLevel) AlertLevel {
	return alertconfig.NormalizePoweredOffSeverity(level)
}

func normalizePoweredOffSeverity(level AlertLevel) AlertLevel {
	return alertconfig.NormalizePoweredOffSeverity(level)
}
func ensureValidHysteresis(threshold *HysteresisThreshold, metricName string) {
	alertconfig.EnsureValidHysteresis(threshold, metricName)
}
func normalizeStorageDefaults(config *AlertConfig) { alertconfig.NormalizeStorageDefaults(config) }
func normalizeDockerThreshold(th HysteresisThreshold, defaultTrigger float64, metricName string) HysteresisThreshold {
	return alertconfig.NormalizeDockerThreshold(th, defaultTrigger, metricName)
}
func normalizeDockerDefaults(config *AlertConfig)   { alertconfig.NormalizeDockerDefaults(config) }
func normalizePMGDefaults(config *AlertConfig)      { alertconfig.NormalizePMGDefaults(config) }
func normalizeSnapshotDefaults(config *AlertConfig) { alertconfig.NormalizeSnapshotDefaults(config) }
func normalizeBackupDefaults(config *AlertConfig)   { alertconfig.NormalizeBackupDefaults(config) }
func normalizeNodeDefaults(config *AlertConfig)     { alertconfig.NormalizeNodeDefaults(config) }
func normalizeAgentDefaults(config *AlertConfig)    { alertconfig.NormalizeAgentDefaults(config) }
func normalizeGeneralSettings(config *AlertConfig)  { alertconfig.NormalizeGeneralSettings(config) }
func normalizeTimeThresholds(config *AlertConfig)   { alertconfig.NormalizeTimeThresholds(config) }
func validateHysteresisThresholds(config *AlertConfig) {
	alertconfig.ValidateHysteresisThresholds(config)
}
func validateQuietHoursTimezone(config *AlertConfig) { alertconfig.ValidateQuietHoursTimezone(config) }

// Alert represents an active alert
type Alert struct {
	ID              string                 `json:"id"`
	Type            string                 `json:"type"` // cpu, memory, disk, etc.
	Level           AlertLevel             `json:"level"`
	ResourceID      string                 `json:"resourceId"` // guest or node ID
	CanonicalSpecID string                 `json:"canonicalSpecId,omitempty"`
	CanonicalKind   string                 `json:"canonicalKind,omitempty"`
	CanonicalState  string                 `json:"canonicalState,omitempty"`
	ResourceName    string                 `json:"resourceName"`
	Node            string                 `json:"node"`
	NodeDisplayName string                 `json:"nodeDisplayName,omitempty"`
	Instance        string                 `json:"instance"`
	Message         string                 `json:"message"`
	Value           float64                `json:"value"`
	Threshold       float64                `json:"threshold"`
	StartTime       time.Time              `json:"startTime"`
	LastSeen        time.Time              `json:"lastSeen"`
	Acknowledged    bool                   `json:"acknowledged"`
	AckTime         *time.Time             `json:"ackTime,omitempty"`
	AckUser         string                 `json:"ackUser,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
	LastNotified    *time.Time             `json:"lastNotified,omitempty"`
	LastEscalation  int                    `json:"lastEscalation,omitempty"`
	EscalationTimes []time.Time            `json:"escalationTimes,omitempty"`
}

// Clone returns a deep copy of the alert so it can be safely shared across goroutines.
func (a *Alert) Clone() *Alert {
	if a == nil {
		return nil
	}

	clone := *a

	if a.AckTime != nil {
		t := *a.AckTime
		clone.AckTime = &t
	}

	if a.LastNotified != nil {
		t := *a.LastNotified
		clone.LastNotified = &t
	}

	if len(a.EscalationTimes) > 0 {
		clone.EscalationTimes = append([]time.Time(nil), a.EscalationTimes...)
	}

	if a.Metadata != nil {
		clone.Metadata = cloneMetadata(a.Metadata)
	}

	return &clone
}

func cloneMetadata(src map[string]interface{}) map[string]interface{} {
	if src == nil {
		return nil
	}

	dst := make(map[string]interface{}, len(src))
	for k, v := range src {
		dst[k] = cloneMetadataValue(v)
	}
	return dst
}

func cloneMetadataValue(val interface{}) interface{} {
	switch v := val.(type) {
	case map[string]interface{}:
		return cloneMetadata(v)
	case map[string]string:
		m := make(map[string]interface{}, len(v))
		for key, value := range v {
			m[key] = value
		}
		return m
	case []interface{}:
		arr := make([]interface{}, len(v))
		for i, elem := range v {
			arr[i] = cloneMetadataValue(elem)
		}
		return arr
	case []string:
		arr := make([]string, len(v))
		copy(arr, v)
		return arr
	case []int:
		arr := make([]int, len(v))
		copy(arr, v)
		return arr
	case []float64:
		arr := make([]float64, len(v))
		copy(arr, v)
		return arr
	default:
		return v
	}
}

// ResolvedAlert represents a recently resolved alert
type ResolvedAlert struct {
	*Alert
	ResolvedTime time.Time `json:"resolvedTime"`
}

// Cleanup intervals
const (
	StaleTrackingThreshold              = 24 * time.Hour
	RateLimitCleanupWindow              = 1 * time.Hour
	alertsDirPerm                       = 0o700
	alertsFilePerm                      = 0o600
	offlineRecoveryConfirmationsDefault = 3
	offlineRecoveryConfirmationsStorage = 2
)

// Manager handles alert monitoring and state
//
// Lock Ordering Documentation:
// The Manager uses two mutexes to prevent deadlocks:
//  1. m.mu (primary lock) - protects most manager state
//  2. m.resolvedMutex - protects only recentlyResolved map
//
// Lock Ordering Rules:
//   - NEVER hold m.mu when acquiring resolvedMutex
//   - ALWAYS release m.mu before acquiring resolvedMutex
//   - resolvedMutex can be held independently without m.mu
//   - When both locks are needed, acquire m.mu first, then release it before acquiring resolvedMutex
//
// This ordering prevents deadlock scenarios where different goroutines acquire locks in different orders.

// Metric hooks for integrating with Prometheus
var (
	recordAlertFired        func(*Alert)
	recordAlertResolved     func(*Alert)
	recordAlertSuppressed   func(string)
	recordAlertAcknowledged func()
)

// SetMetricHooks registers callbacks for recording alert metrics.
// - fired: called when an alert is dispatched (in dispatchAlert)
// - resolved: called when an alert is cleared (in clearAlertNoLock)
// - suppressed: called when an alert is suppressed due to flapping
// - acknowledged: called when an alert is acknowledged
func SetMetricHooks(fired func(*Alert), resolved func(*Alert), suppressed func(string), acknowledged func()) {
	recordAlertFired = fired
	recordAlertResolved = resolved
	recordAlertSuppressed = suppressed
	recordAlertAcknowledged = acknowledged
}

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
	// Offline confirmation tracking
	nodeOfflineCount             map[string]int                  // Track consecutive offline counts for nodes (legacy)
	offlineConfirmations         map[string]int                  // Track consecutive offline counts for all resources
	offlineRecoveryConfirmations map[string]int                  // Track consecutive healthy confirmations before clearing poll-driven offline alerts
	dockerOfflineCount           map[string]int                  // Track consecutive offline counts for Docker hosts
	dockerStateConfirm           map[string]int                  // Track consecutive state confirmations for Docker containers
	dockerRestartTracking        map[string]*dockerRestartRecord // Track restart counts and times for restart loop detection
	dockerLastExitCode           map[string]int                  // Track last exit code for OOM detection
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
	alertOrphaned := true
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
		nodeOfflineCount:                make(map[string]int),
		offlineConfirmations:            make(map[string]int),
		offlineRecoveryConfirmations:    make(map[string]int),
		dockerOfflineCount:              make(map[string]int),
		dockerStateConfirm:              make(map[string]int),
		dockerRestartTracking:           make(map[string]*dockerRestartRecord),
		dockerLastExitCode:              make(map[string]int),
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
		config: AlertConfig{
			Enabled:                true,
			ActivationState:        ActivationPending,
			ObservationWindowHours: 24,
			GuestDefaults: ThresholdConfig{
				PoweredOffSeverity: AlertLevelWarning,
				CPU:                &HysteresisThreshold{Trigger: 80, Clear: 75},
				Memory:             &HysteresisThreshold{Trigger: 85, Clear: 80},
				Disk:               &HysteresisThreshold{Trigger: 90, Clear: 85},
				DiskRead:           &HysteresisThreshold{Trigger: 0, Clear: 0}, // Off by default
				DiskWrite:          &HysteresisThreshold{Trigger: 0, Clear: 0}, // Off by default
				NetworkIn:          &HysteresisThreshold{Trigger: 0, Clear: 0}, // Off by default
				NetworkOut:         &HysteresisThreshold{Trigger: 0, Clear: 0}, // Off by default
			},
			NodeDefaults: ThresholdConfig{
				CPU:         &HysteresisThreshold{Trigger: 80, Clear: 75},
				Memory:      &HysteresisThreshold{Trigger: 85, Clear: 80},
				Disk:        &HysteresisThreshold{Trigger: 90, Clear: 85},
				Temperature: &HysteresisThreshold{Trigger: 80, Clear: 75}, // Warning at 80°C, clear at 75°C
			},
			AgentDefaults: ThresholdConfig{
				CPU:             &HysteresisThreshold{Trigger: 80, Clear: 75},
				Memory:          &HysteresisThreshold{Trigger: 85, Clear: 80},
				Disk:            &HysteresisThreshold{Trigger: 90, Clear: 85},
				DiskTemperature: &HysteresisThreshold{Trigger: 55, Clear: 50},
			},
			DockerDefaults: DockerThresholdConfig{
				CPU:                     HysteresisThreshold{Trigger: 80, Clear: 75},
				Memory:                  HysteresisThreshold{Trigger: 85, Clear: 80},
				Disk:                    HysteresisThreshold{Trigger: 85, Clear: 80},
				RestartCount:            3,
				RestartWindow:           300, // 5 minutes
				MemoryWarnPct:           90,
				MemoryCriticalPct:       95,
				StatePoweredOffSeverity: AlertLevelWarning,
			},
			PMGDefaults: PMGThresholdConfig{
				QueueTotalWarning:       500,  // Warning at 500 total queued messages
				QueueTotalCritical:      1000, // Critical at 1000 total queued messages
				OldestMessageWarnMins:   30,   // Warning if oldest message is 30+ minutes old
				OldestMessageCritMins:   60,   // Critical if oldest message is 60+ minutes old
				DeferredQueueWarn:       200,  // Warning at 200 deferred messages
				DeferredQueueCritical:   500,  // Critical at 500 deferred messages
				HoldQueueWarn:           100,  // Warning at 100 held messages
				HoldQueueCritical:       300,  // Critical at 300 held messages
				QuarantineSpamWarn:      2000, // Warning at 2000 spam quarantined
				QuarantineSpamCritical:  5000, // Critical at 5000 spam quarantined
				QuarantineVirusWarn:     2000, // Warning at 2000 virus quarantined
				QuarantineVirusCritical: 5000, // Critical at 5000 virus quarantined
				QuarantineGrowthWarnPct: 25,   // Warning if growth ≥25%
				QuarantineGrowthWarnMin: 250,  // AND ≥250 messages
				QuarantineGrowthCritPct: 50,   // Critical if growth ≥50%
				QuarantineGrowthCritMin: 500,  // AND ≥500 messages
			},
			SnapshotDefaults: SnapshotAlertConfig{
				Enabled:         false,
				WarningDays:     30,
				CriticalDays:    45,
				WarningSizeGiB:  0,
				CriticalSizeGiB: 0,
			},
			BackupDefaults: BackupAlertConfig{
				Enabled:       false,
				WarningDays:   7,
				CriticalDays:  14,
				FreshHours:    24,
				StaleHours:    72,
				AlertOrphaned: &alertOrphaned,
				IgnoreVMIDs:   []string{},
			},
			PBSDefaults: ThresholdConfig{
				CPU:    &HysteresisThreshold{Trigger: 80, Clear: 75},
				Memory: &HysteresisThreshold{Trigger: 85, Clear: 80},
			},
			StorageDefault:    HysteresisThreshold{Trigger: 85, Clear: 80},
			MinimumDelta:      2.0, // 2% minimum change
			SuppressionWindow: 5,   // 5 minutes
			HysteresisMargin:  5.0, // 5% default margin
			TimeThresholds: map[string]int{
				"guest":   5,
				"node":    5,
				"agent":   5,
				"storage": 5,
				"pbs":     5,
			},
			Overrides: make(map[string]ThresholdConfig),
			Schedule: ScheduleConfig{
				QuietHours: QuietHours{
					Enabled:  false, // OFF - users should opt-in to quiet hours
					Start:    "22:00",
					End:      "08:00",
					Timezone: "America/New_York",
					Days: map[string]bool{
						"monday":    true,
						"tuesday":   true,
						"wednesday": true,
						"thursday":  true,
						"friday":    true,
						"saturday":  false,
						"sunday":    false,
					},
					Suppress: QuietHoursSuppression{},
				},
				Cooldown:        5,  // ON - 5 minutes prevents spam
				MaxAlertsHour:   10, // ON - 10 alerts/hour prevents flooding
				NotifyOnResolve: true,
				Escalation: EscalationConfig{
					Enabled: false, // OFF - requires user configuration
					Levels: []EscalationLevel{
						{After: 15, Notify: "email"},
						{After: 30, Notify: "webhook"},
						{After: 60, Notify: "all"},
					},
				},
				Grouping: GroupingConfig{
					Enabled: true,  // ON - reduces notification noise
					Window:  30,    // 30 second window for grouping
					ByNode:  true,  // Group by node for mass node issues
					ByGuest: false, // Don't group by guest by default
				},
			},
			// Alert TTL defaults
			MaxAlertAgeDays:           7,  // Auto-cleanup alerts older than 7 days
			MaxAcknowledgedAgeDays:    1,  // Auto-cleanup acknowledged alerts older than 1 day
			AutoAcknowledgeAfterHours: 24, // Auto-acknowledge alerts after 24 hours
			// Flapping detection defaults
			FlappingEnabled:         true, // Enable flapping detection
			FlappingWindowSeconds:   300,  // 5 minute window
			FlappingThreshold:       5,    // 5 state changes triggers flapping
			FlappingCooldownMinutes: 15,   // 15 minute cooldown
		},
	}

	// Load saved active alerts
	if err := m.LoadActiveAlerts(); err != nil {
		log.Error().Err(err).Msg("failed to load active alerts")
	}

	// Start escalation checker
	go m.escalationChecker()

	// Start periodic save of active alerts
	go m.periodicSaveAlerts()

	// Start periodic cleanup of stale tracking map entries
	go m.trackingMapCleanup()

	return m
}

// SetLicenseChecker sets the function used to check Pro license features.
// This enables gating Pro-only alert features like update alerts.
func (m *Manager) SetLicenseChecker(checker func(feature string) bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hasProFeature = checker
}

// addRecentlyResolvedUnlocked records a resolved alert assuming the caller does not hold m.mu.
func (m *Manager) addRecentlyResolvedUnlocked(resolved *ResolvedAlert) {
	m.resolvedMutex.Lock()
	if resolved == nil || resolved.Alert == nil {
		m.resolvedMutex.Unlock()
		return
	}
	storageKey := activeAlertStorageKey(resolved.Alert, resolved.Alert.ID)
	m.recentlyResolved[storageKey] = resolved
	m.registerResolvedAliasUnlocked(storageKey, resolved)
	m.resolvedMutex.Unlock()
}

// addRecentlyResolvedWithPrimaryLock records a resolved alert while preserving the caller's
// ownership of m.mu. Callers must hold m.mu before invoking this helper.
func (m *Manager) addRecentlyResolvedWithPrimaryLock(resolved *ResolvedAlert) {
	m.mu.Unlock()
	m.addRecentlyResolvedUnlocked(resolved)
	m.mu.Lock()
}

// clearAlert removes an alert if it exists
func (m *Manager) clearAlert(alertID string) {
	m.mu.Lock()
	alert, exists := m.getActiveAlertNoLock(alertID)
	if exists {
		m.removeActiveAlertNoLock(alertID)
	}
	m.mu.Unlock()

	if !exists {
		return
	}

	publicID := effectiveAlertID(alert, alertID)
	resolvedAlert := &ResolvedAlert{
		Alert:        alert,
		ResolvedTime: time.Now(),
	}

	m.addRecentlyResolvedUnlocked(resolvedAlert)

	m.safeCallResolvedAlertCallback(alert, publicID, false)

	log.Info().
		Str("alertID", publicID).
		Msg("Alert cleared")
}

// AcknowledgeAlert acknowledges an alert
func (m *Manager) AcknowledgeAlert(alertID, user string) error {
	m.mu.Lock()

	key, exists := m.resolveActiveAlertKeyNoLock(alertID)
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("%w: %s", ErrAlertNotFound, alertID)
	}
	alert, ok := m.getActiveAlertNoLock(key)
	if !ok || alert == nil {
		m.mu.Unlock()
		return fmt.Errorf("%w: %s", ErrAlertNotFound, alertID)
	}

	alert.Acknowledged = true
	now := time.Now()
	alert.AckTime = &now
	alert.AckUser = user

	m.setActiveAlertNoLock(key, alert)
	m.setAckRecordNoLock(alert, alertID, ackRecord{
		acknowledged: true,
		user:         user,
		time:         now,
	})

	alertCopy := alert.Clone()
	m.mu.Unlock()

	log.Debug().
		Str("alertID", alertID).
		Str("user", user).
		Time("ackTime", now).
		Msg("Alert acknowledgment recorded")

	m.safeCallAcknowledgedCallback(alertCopy, user)
	return nil
}

// UnacknowledgeAlert removes the acknowledged status from an alert
func (m *Manager) UnacknowledgeAlert(alertID string) error {
	m.mu.Lock()

	key, exists := m.resolveActiveAlertKeyNoLock(alertID)
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("%w: %s", ErrAlertNotFound, alertID)
	}
	alert, ok := m.getActiveAlertNoLock(key)
	if !ok || alert == nil {
		m.mu.Unlock()
		return fmt.Errorf("%w: %s", ErrAlertNotFound, alertID)
	}

	alert.Acknowledged = false
	alert.AckTime = nil
	alert.AckUser = ""

	m.setActiveAlertNoLock(key, alert)
	m.deleteAckRecordNoLock(alert, alertID)

	alertCopy := alert.Clone()
	m.mu.Unlock()

	log.Info().
		Str("alertID", alertID).
		Msg("Alert unacknowledged")

	m.safeCallUnacknowledgedCallback(alertCopy, "")
	return nil
}

// preserveAlertState copies acknowledgement and escalation metadata from an existing alert
// into a freshly constructed alert before it replaces the existing entry in the map. This
// prevents UI state from regressing when alerts are rebuilt during polling.
func (m *Manager) preserveAlertState(alertID string, updated *Alert) {
	if updated == nil {
		return
	}
	backfillCanonicalIdentity(updated)

	// Auto-resolve node display name if not already set.
	if updated.NodeDisplayName == "" && updated.Node != "" {
		updated.NodeDisplayName = m.resolveNodeDisplayName(updated.Instance, updated.Node)
	}

	existing, exists := m.getActiveAlertNoLock(alertID)
	if exists && existing != nil {
		// Preserve the original start time so duration calculations are correct
		updated.StartTime = existing.StartTime
		if existing.LastNotified != nil {
			t := *existing.LastNotified
			updated.LastNotified = &t
		} else {
			updated.LastNotified = nil
		}
		updated.Acknowledged = existing.Acknowledged
		updated.AckUser = existing.AckUser
		if existing.AckTime != nil {
			t := *existing.AckTime
			updated.AckTime = &t
		} else {
			updated.AckTime = nil
		}
		updated.LastEscalation = existing.LastEscalation
		if len(existing.EscalationTimes) > 0 {
			updated.EscalationTimes = append([]time.Time(nil), existing.EscalationTimes...)
		} else {
			updated.EscalationTimes = nil
		}

		log.Debug().
			Str("alertID", alertID).
			Time("originalStartTime", existing.StartTime).
			Dur("currentDuration", time.Since(existing.StartTime)).
			Msg("Preserving alert state including StartTime")
		return
	}

	if record, ok := m.getAckRecordNoLock(updated, alertID); ok && record.acknowledged {
		updated.Acknowledged = true
		updated.AckUser = record.user
		t := record.time
		updated.AckTime = &t
	}
}

func (m *Manager) removeActiveAlertNoLock(alertID string) {
	// Before deleting, update the history entry with the alert's final LastSeen
	// timestamp so the stored duration reflects how long the alert was actually active.
	publicID := alertID
	var currentAlert *Alert
	key, exists := m.resolveActiveAlertKeyNoLock(alertID)
	if !exists {
		key, exists = m.resolveActiveAlertKeyByCanonicalStateNoLock(alertID)
	}
	if alert, ok := m.getActiveAlertNoLock(alertID); exists && ok && alert != nil {
		currentAlert = alert
		backfillCanonicalIdentity(alert)
		publicID = effectiveAlertID(alert, alertID)
		m.historyManager.UpdateAlertLastSeenForAlert(alert, alert.LastSeen)
		m.unregisterActiveAlertAliasNoLock(key, alert)
	}
	if exists {
		delete(m.offlineRecoveryConfirmations, key)
		delete(m.activeAlerts, key)
	}
	delete(m.offlineRecoveryConfirmations, alertID)
	// NOTE: Don't delete ackState here - preserve it so if the same alert
	// reappears (e.g., powered-off VM during backup), the acknowledgement
	// is restored via preserveAlertState. ackState is cleaned up in Cleanup().
	// Update inactiveAt so the cleanup TTL is measured from removal time, not ack time.
	if exists {
		m.markAckInactiveNoLock(currentAlert, publicID, time.Now())
	}
}

func (m *Manager) confirmOfflineRecoveryNoLock(alertID string, required int) (int, bool) {
	alertID = strings.TrimSpace(alertID)
	if alertID == "" {
		return 0, false
	}

	if required <= 1 {
		delete(m.offlineRecoveryConfirmations, alertID)
		return required, true
	}

	m.offlineRecoveryConfirmations[alertID]++
	confirmations := m.offlineRecoveryConfirmations[alertID]
	if confirmations < required {
		return confirmations, false
	}

	delete(m.offlineRecoveryConfirmations, alertID)
	return confirmations, true
}

// clearResourceOfflineAlert removes an offline alert when a poll-driven resource
// stays healthy for enough consecutive polls to confirm recovery.
func (m *Manager) clearResourceOfflineAlert(resourceID, resourceName, host, resourceKind string, requiredRecoveryCount int) {
	alertID := canonicalConnectivityStateID(resourceID)

	m.mu.Lock()
	defer m.mu.Unlock()

	// Reset offline confirmation count
	if count, exists := m.offlineConfirmations[resourceID]; exists && count > 0 {
		log.Debug().
			Str(strings.ToLower(resourceKind), resourceName).
			Int("previousCount", count).
			Msg(resourceKind + " is online, resetting offline confirmation count")
		delete(m.offlineConfirmations, resourceID)
	}

	// Check if offline alert exists
	alert, exists := m.getActiveAlertNoLock(alertID)
	if !exists {
		delete(m.offlineRecoveryConfirmations, alertID)
		return
	}

	recoveryCount, confirmed := m.confirmOfflineRecoveryNoLock(alertID, requiredRecoveryCount)
	if !confirmed {
		log.Debug().
			Str(strings.ToLower(resourceKind), resourceName).
			Int("confirmations", recoveryCount).
			Int("required", requiredRecoveryCount).
			Msg(resourceKind + " appears back online, waiting for recovery confirmation")
		return
	}

	// Remove from active alerts
	m.removeActiveAlertNoLock(alertID)

	resolvedAlert := &ResolvedAlert{
		Alert:        alert,
		ResolvedTime: time.Now(),
	}
	m.addRecentlyResolvedWithPrimaryLock(resolvedAlert)

	// Send recovery notification (async to avoid deadlock — callback acquires m.mu.RLock
	// via ShouldSuppressResolvedNotification, and we currently hold m.mu.Lock)
	m.safeCallResolvedAlertCallback(alert, alertID, true)

	// Log recovery
	log.Info().
		Str(strings.ToLower(resourceKind), resourceName).
		Str("host", host).
		Dur("downtime", time.Since(alert.StartTime)).
		Msg(resourceKind + " instance is back online")
}

// ClearAlert removes an alert from active alerts (but keeps in history)
func (m *Manager) ClearAlert(alertID string) bool {
	m.mu.Lock()
	alert, exists := m.getActiveAlertNoLock(alertID)
	if !exists || alert == nil {
		m.mu.Unlock()
		return false
	}
	trackingKey := canonicalTrackingKeyForAlert(alert)

	m.clearAlertNoLock(alertID)
	delete(m.recentAlerts, alertID)
	delete(m.pendingAlerts, alertID)
	delete(m.suppressedUntil, alertID)
	delete(m.alertRateLimit, alertID)
	if trackingKey != "" && trackingKey != alertID {
		delete(m.recentAlerts, trackingKey)
		delete(m.pendingAlerts, trackingKey)
		delete(m.suppressedUntil, trackingKey)
		delete(m.alertRateLimit, trackingKey)
	}
	m.mu.Unlock()

	m.saveActiveAlertsAsync("manual-clear")
	return true
}

// Cleanup removes old acknowledged alerts and cleans up tracking maps
func (m *Manager) Cleanup(maxAge time.Duration) {
	m.mu.Lock()
	now := time.Now()
	var autoAcked []*Alert

	lastSeenTooOld := func(alert *Alert, cutoff time.Duration) bool {
		if alert == nil {
			return true
		}
		lastSeen := alert.LastSeen
		if lastSeen.IsZero() {
			lastSeen = alert.StartTime
		}
		return now.Sub(lastSeen) > cutoff
	}

	// Auto-acknowledge old alerts if configured
	if m.config.AutoAcknowledgeAfterHours > 0 {
		autoAckThreshold := time.Duration(m.config.AutoAcknowledgeAfterHours) * time.Hour
		for id, alert := range m.activeAlerts {
			if !alert.Acknowledged && now.Sub(alert.StartTime) > autoAckThreshold {
				log.Info().
					Str("alertID", id).
					Dur("age", now.Sub(alert.StartTime)).
					Msg("Auto-acknowledging old alert")
				alert.Acknowledged = true
				ackTime := now
				alert.AckTime = &ackTime
				alert.AckUser = "system-auto"
				autoAcked = append(autoAcked, alert.Clone())

				if recordAlertAcknowledged != nil {
					recordAlertAcknowledged()
				}
			}
		}
	}

	// Clean up acknowledged alerts based on TTL
	if m.config.MaxAcknowledgedAgeDays > 0 {
		acknowledgedTTL := time.Duration(m.config.MaxAcknowledgedAgeDays) * 24 * time.Hour
		for id, alert := range m.activeAlerts {
			if alert.Acknowledged && alert.AckTime != nil &&
				now.Sub(*alert.AckTime) > acknowledgedTTL &&
				lastSeenTooOld(alert, acknowledgedTTL) {
				log.Info().
					Str("alertID", id).
					Dur("age", now.Sub(*alert.AckTime)).
					Msg("Cleaning up old acknowledged alert (TTL)")
				m.removeActiveAlertNoLock(id)
			}
		}
	}

	// Clean up old unacknowledged alerts based on TTL
	if m.config.MaxAlertAgeDays > 0 {
		alertTTL := time.Duration(m.config.MaxAlertAgeDays) * 24 * time.Hour
		for id, alert := range m.activeAlerts {
			if !alert.Acknowledged && now.Sub(alert.StartTime) > alertTTL {
				log.Info().
					Str("alertID", id).
					Dur("age", now.Sub(alert.StartTime)).
					Msg("Cleaning up old unacknowledged alert (TTL)")
				m.removeActiveAlertNoLock(id)
			}
		}
	}

	// Original cleanup for acknowledged alerts (fallback if TTL not configured)
	for id, alert := range m.activeAlerts {
		if alert.Acknowledged && alert.AckTime != nil &&
			now.Sub(*alert.AckTime) > maxAge &&
			lastSeenTooOld(alert, maxAge) {
			m.removeActiveAlertNoLock(id)
		}
	}

	// Clean up stale ackState entries for alerts that no longer exist
	// Keep ackState for 1 hour after the alert was removed (not from ack time)
	// to handle transient alert clears (e.g., backups of powered-off VMs)
	ackStateTTL := 1 * time.Hour
	for id, record := range m.ackState {
		if !m.hasActiveAlertNoLock(id) {
			// Use inactiveAt (when alert was removed) for TTL, not ack time
			checkTime := record.inactiveAt
			if checkTime.IsZero() {
				// Fallback for legacy entries without inactiveAt
				checkTime = record.time
			}
			if now.Sub(checkTime) > ackStateTTL {
				delete(m.ackState, id)
			}
		}
	}
	for canonicalID, record := range m.ackStateByCanonical {
		if m.hasActiveAlertTrackingKeyNoLock(canonicalID) {
			continue
		}
		checkTime := record.inactiveAt
		if checkTime.IsZero() {
			checkTime = record.time
		}
		if now.Sub(checkTime) > ackStateTTL {
			delete(m.ackStateByCanonical, canonicalID)
		}
	}

	// Clean up recent alerts older than suppression window
	suppressionWindow := time.Duration(m.config.SuppressionWindow) * time.Minute
	if suppressionWindow == 0 {
		suppressionWindow = 5 * time.Minute // Default
	}

	for id, alert := range m.recentAlerts {
		if now.Sub(alert.StartTime) > suppressionWindow {
			delete(m.recentAlerts, id)
		}
	}

	// Clean up expired suppressions
	for id, suppressUntil := range m.suppressedUntil {
		if now.After(suppressUntil) {
			delete(m.suppressedUntil, id)
		}
	}

	// Clean up old rate limit entries (older than 1 hour)
	cutoff := now.Add(-1 * time.Hour)
	for alertID, times := range m.alertRateLimit {
		var recentTimes []time.Time
		for _, t := range times {
			if t.After(cutoff) {
				recentTimes = append(recentTimes, t)
			}
		}
		if len(recentTimes) == 0 {
			// No recent alerts, remove the entry entirely
			delete(m.alertRateLimit, alertID)
		} else {
			// Update with only recent times
			m.alertRateLimit[alertID] = recentTimes
		}
	}

	// Clean up old recently resolved alerts (older than 5 minutes)
	fiveMinutesAgo := now.Add(-5 * time.Minute)
	m.resolvedMutex.Lock()
	for alertID, resolved := range m.recentlyResolved {
		if resolved.ResolvedTime.Before(fiveMinutesAgo) {
			m.removeResolvedAlertUnlocked(alertID)
		}
	}
	m.resolvedMutex.Unlock()

	// Clean up stale pending alerts (older than max time threshold window)
	// This prevents memory leak from deleted resources that never triggered alerts
	maxPendingAge := 10 * time.Minute // Longest time threshold + safety buffer
	for id, pendingTime := range m.pendingAlerts {
		if now.Sub(pendingTime) > maxPendingAge {
			delete(m.pendingAlerts, id)
			log.Debug().
				Str("resourceID", id).
				Dur("age", now.Sub(pendingTime)).
				Msg("Cleaned up stale pending alert entry")
		}
	}

	// Clean up flapping history for resolved/inactive alerts
	flappingCleanupAge := 1 * time.Hour
	for alertID := range m.flappingHistory {
		// If alert is no longer active and flapping cooldown has expired
		if !m.hasActiveAlertTrackingKeyNoLock(alertID) {
			if suppressUntil, suppressed := m.suppressedUntil[alertID]; !suppressed || now.After(suppressUntil.Add(flappingCleanupAge)) {
				delete(m.flappingHistory, alertID)
				delete(m.flappingActive, alertID)
				log.Debug().
					Str("alertID", alertID).
					Msg("Cleaned up flapping history for inactive alert")
			}
		}
	}

	// Clean up old Docker restart tracking (containers not seen in 24h)
	// Prevents memory leak from ephemeral containers in CI/CD environments
	for resourceID, record := range m.dockerRestartTracking {
		if now.Sub(record.lastChecked) > 24*time.Hour {
			delete(m.dockerRestartTracking, resourceID)
			log.Debug().
				Str("resourceID", resourceID).
				Msg("Cleaned up stale Docker restart tracking entry")
		}
	}

	// Clean up stale PMG anomaly trackers (no samples in 24h)
	// Prevents memory leak from decommissioned or transient PMG instances
	staleTrackerAge := 24 * time.Hour
	for pmgID, tracker := range m.pmgAnomalyTrackers {
		if tracker != nil && !tracker.LastSampleTime.IsZero() {
			if now.Sub(tracker.LastSampleTime) > staleTrackerAge {
				delete(m.pmgAnomalyTrackers, pmgID)
				log.Debug().
					Str("pmgID", pmgID).
					Time("lastSampleTime", tracker.LastSampleTime).
					Msg("Cleaned up stale PMG anomaly tracker")
			}
		}
	}

	// Clean up stale PMG quarantine history (no recent snapshots in 7 days)
	// Prevents memory leak from deleted PMG instances
	staleHistoryAge := 7 * 24 * time.Hour
	for pmgID, snapshots := range m.pmgQuarantineHistory {
		// If no snapshots remain or last snapshot is very old
		if len(snapshots) == 0 {
			delete(m.pmgQuarantineHistory, pmgID)
			log.Debug().
				Str("pmgID", pmgID).
				Msg("Cleaned up empty PMG quarantine history")
			continue
		}

		lastSnapshot := snapshots[len(snapshots)-1]
		if now.Sub(lastSnapshot.Timestamp) > staleHistoryAge {
			delete(m.pmgQuarantineHistory, pmgID)
			log.Debug().
				Str("pmgID", pmgID).
				Time("lastSnapshot", lastSnapshot.Timestamp).
				Msg("Cleaned up stale PMG quarantine history")
		}
	}

	m.mu.Unlock()

	for _, alert := range autoAcked {
		m.safeCallAcknowledgedCallback(alert, "system-auto")
	}
}

// escalationChecker runs periodically to check for alerts that need escalation and cleanup
func (m *Manager) escalationChecker() {
	ticker := time.NewTicker(1 * time.Minute)
	cleanupTicker := time.NewTicker(10 * time.Minute) // Run cleanup every 10 minutes
	defer ticker.Stop()
	defer cleanupTicker.Stop()

	for {
		select {
		case <-ticker.C:
			m.checkEscalations()
		case <-cleanupTicker.C:
			m.Cleanup(24 * time.Hour) // Clean up acknowledged alerts older than 24 hours
		case <-m.escalationStop:
			return
		}
	}
}

// checkEscalations checks all active alerts for escalation
func (m *Manager) checkEscalations() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Respect global alert and activation controls before escalating.
	// Escalations should never bypass a user disabling alerts.
	if !m.config.Enabled || m.config.ActivationState != ActivationActive {
		return
	}

	if !m.config.Schedule.Escalation.Enabled {
		return
	}

	now := time.Now()
	for _, alert := range m.activeAlerts {
		// Skip acknowledged alerts
		if alert.Acknowledged {
			continue
		}

		// Check each escalation level
		for i, level := range m.config.Schedule.Escalation.Levels {
			// Skip if we've already escalated to this level
			if alert.LastEscalation >= i+1 {
				continue
			}

			// Check if it's time to escalate
			escalateTime := alert.StartTime.Add(time.Duration(level.After) * time.Minute)
			if now.After(escalateTime) {
				// Update alert escalation state
				alert.LastEscalation = i + 1
				alert.EscalationTimes = append(alert.EscalationTimes, now)

				log.Info().
					Str("alertID", alert.ID).
					Str("trackingKey", canonicalTrackingKeyForAlert(alert)).
					Int("level", i+1).
					Str("notify", level.Notify).
					Msg("Alert escalated")

				// Trigger escalation callback
				m.safeCallEscalateCallback(alert, i+1)
			}
		}
	}
}

// Stop stops the alert manager and saves history
func (m *Manager) Stop() {
	m.stopOnce.Do(func() {
		closeSignalChannel(m.escalationStop)
		closeSignalChannel(m.cleanupStop)
		if m.historyManager != nil {
			m.historyManager.Stop()
		}

		// Give background goroutines time to exit cleanly
		time.Sleep(100 * time.Millisecond)

		// Save active alerts before stopping
		if err := m.SaveActiveAlerts(); err != nil {
			log.Error().Err(err).Msg("Failed to save active alerts on stop")
		}
	})
}

func closeSignalChannel(ch chan struct{}) {
	if ch == nil {
		return
	}
	defer func() {
		if recover() != nil {
			// Channel was already closed by another shutdown path.
		}
	}()
	close(ch)
}

// CleanupAlertsForNodes removes alerts for nodes that no longer exist
func (m *Manager) CleanupAlertsForNodes(existingNodes map[string]bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	log.Debug().
		Int("totalAlerts", len(m.activeAlerts)).
		Int("existingNodes", len(existingNodes)).
		Interface("nodes", existingNodes).
		Msg("Starting alert cleanup for non-existent nodes")

	removedCount := 0
	for storageKey, alert := range m.activeAlerts {
		alertID := effectiveAlertID(alert, storageKey)
		if alert == nil {
			continue
		}

		// Skip alerts that are not tied to Proxmox nodes. Docker and PBS resources use
		// synthetic node identifiers that won't appear in the Proxmox node list, so we
		// must preserve their alerts here.
		if strings.HasPrefix(alertID, "docker-") || strings.HasPrefix(alert.ResourceID, "docker:") {
			continue
		}
		if strings.HasPrefix(alertID, "pbs-") || alert.Type == "pbs-offline" {
			continue
		}
		if alert.Metadata != nil {
			if resourceType, _ := alert.Metadata["resourceType"].(string); resourceType == "pbs" {
				continue
			}
		}
		if alert.CanonicalKind == string(alertspecs.AlertSpecKindConnectivity) && strings.HasPrefix(alert.ResourceID, "pbs") {
			continue
		}
		// Use the Node field from the alert itself, which is more reliable
		node := alert.Node

		// If we couldn't get a node or the node doesn't exist, remove the alert
		if node == "" || !existingNodes[node] {
			m.removeActiveAlertNoLock(alertID)
			removedCount++
			log.Debug().Str("alertID", alertID).Str("node", node).Msg("removed alert for non-existent node")
		}
	}

	if removedCount > 0 {
		log.Debug().Int("removed", removedCount).Int("remaining", len(m.activeAlerts)).Msg("cleaned up alerts for non-existent nodes")
		// Save the cleaned up state
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Error().Interface("panic", r).Msg("panic in SaveActiveAlerts goroutine (cleanup)")
				}
			}()
			if err := m.SaveActiveAlerts(); err != nil {
				log.Error().Err(err).Msg("failed to save alerts after cleanup")
			}
		}()
	} else {
		log.Info().Msg("no alerts needed cleanup")
	}
}

// ClearActiveAlerts removes all active and pending alerts, resetting the manager state.
func (m *Manager) ClearActiveAlerts() {
	m.mu.Lock()
	if len(m.activeAlerts) == 0 && len(m.pendingAlerts) == 0 {
		m.mu.Unlock()
		return
	}
	m.activeAlerts = make(map[string]*Alert)
	m.activeAlertAlias = make(map[string]string)
	m.pendingAlerts = make(map[string]time.Time)
	m.recentAlerts = make(map[string]*Alert)
	m.suppressedUntil = make(map[string]time.Time)
	m.alertRateLimit = make(map[string][]time.Time)
	m.nodeOfflineCount = make(map[string]int)
	m.offlineConfirmations = make(map[string]int)
	m.offlineRecoveryConfirmations = make(map[string]int)
	m.dockerOfflineCount = make(map[string]int)
	m.dockerStateConfirm = make(map[string]int)
	m.dockerRestartTracking = make(map[string]*dockerRestartRecord)
	m.dockerLastExitCode = make(map[string]int)
	m.dockerUpdateFirstSeen = make(map[string]time.Time)
	m.dockerUpdateFirstSeenByIdentity = make(map[string]time.Time)
	m.ackState = make(map[string]ackRecord)
	m.ackStateByCanonical = make(map[string]ackRecord)
	m.mu.Unlock()

	m.resolvedMutex.Lock()
	m.recentlyResolved = make(map[string]*ResolvedAlert)
	m.resolvedAlias = make(map[string]string)
	m.resolvedMutex.Unlock()

	log.Info().Msg("cleared all active and pending alerts")

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error().Interface("panic", r).Msg("panic in SaveActiveAlerts goroutine (clear)")
			}
		}()
		if err := m.SaveActiveAlerts(); err != nil {
			log.Error().Err(err).Msg("failed to persist cleared alerts")
		}
	}()
}

// clearAlertNoLock clears an alert without locking (must be called with lock held)
func (m *Manager) clearAlertNoLock(alertID string) {
	alert, exists := m.getActiveAlertNoLock(alertID)
	if !exists {
		return
	}
	publicID := effectiveAlertID(alert, alertID)

	// Record metric for resolved alert
	if recordAlertResolved != nil {
		recordAlertResolved(alert)
	}

	m.removeActiveAlertNoLock(alertID)
	resolvedAlert := &ResolvedAlert{
		Alert:        alert,
		ResolvedTime: time.Now(),
	}

	m.addRecentlyResolvedWithPrimaryLock(resolvedAlert)

	m.safeCallResolvedAlertCallback(alert, publicID, true) // Make async to prevent deadlock

	log.Info().
		Str("alertID", publicID).
		Msg("Alert cleared")
}

func (m *Manager) clearActiveAlertIfPresentNoLock(alertID string) bool {
	if _, exists := m.getActiveAlertNoLock(alertID); !exists {
		return false
	}
	m.clearAlertNoLock(alertID)
	return true
}
