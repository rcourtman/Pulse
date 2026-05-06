package alerts

import (
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"time"

	alertconfig "github.com/rcourtman/pulse-go-rewrite/internal/alerts/config"
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
