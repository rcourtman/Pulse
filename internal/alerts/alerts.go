package alerts

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/rs/zerolog/log"
)

// AlertLevel represents the severity of an alert
type AlertLevel string

const (
	AlertLevelWarning  AlertLevel = "warning"
	AlertLevelCritical AlertLevel = "critical"
)

// Alert represents an active alert
type Alert struct {
	ID           string                 `json:"id"`
	Type         string                 `json:"type"` // cpu, memory, disk, etc.
	Level        AlertLevel             `json:"level"`
	ResourceID   string                 `json:"resourceId"` // guest or node ID
	ResourceName string                 `json:"resourceName"`
	Node         string                 `json:"node"`
	Instance     string                 `json:"instance"`
	Message      string                 `json:"message"`
	Value        float64                `json:"value"`
	Threshold    float64                `json:"threshold"`
	StartTime    time.Time              `json:"startTime"`
	LastSeen     time.Time              `json:"lastSeen"`
	Acknowledged bool                   `json:"acknowledged"`
	AckTime      *time.Time             `json:"ackTime,omitempty"`
	AckUser      string                 `json:"ackUser,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	// Escalation tracking
	LastEscalation  int         `json:"lastEscalation,omitempty"`  // Last escalation level notified
	EscalationTimes []time.Time `json:"escalationTimes,omitempty"` // Times when escalations were sent
}

// ResolvedAlert represents a recently resolved alert
type ResolvedAlert struct {
	*Alert
	ResolvedTime time.Time `json:"resolvedTime"`
}

// HysteresisThreshold represents a threshold with hysteresis
type HysteresisThreshold struct {
	Trigger float64 `json:"trigger"` // Threshold to trigger alert
	Clear   float64 `json:"clear"`   // Threshold to clear alert
}

// ThresholdConfig represents threshold configuration
type ThresholdConfig struct {
	Disabled            bool                 `json:"disabled,omitempty"`            // Completely disable alerts for this guest
	DisableConnectivity bool                 `json:"disableConnectivity,omitempty"` // Disable node offline/connectivity/powered-off alerts
	CPU                 *HysteresisThreshold `json:"cpu,omitempty"`
	Memory              *HysteresisThreshold `json:"memory,omitempty"`
	Disk                *HysteresisThreshold `json:"disk,omitempty"`
	DiskRead            *HysteresisThreshold `json:"diskRead,omitempty"`
	DiskWrite           *HysteresisThreshold `json:"diskWrite,omitempty"`
	NetworkIn           *HysteresisThreshold `json:"networkIn,omitempty"`
	NetworkOut          *HysteresisThreshold `json:"networkOut,omitempty"`
	Usage               *HysteresisThreshold `json:"usage,omitempty"`       // For storage devices
	Temperature         *HysteresisThreshold `json:"temperature,omitempty"` // For node CPU temperature
	// Legacy fields for backward compatibility
	CPULegacy        *float64 `json:"cpuLegacy,omitempty"`
	MemoryLegacy     *float64 `json:"memoryLegacy,omitempty"`
	DiskLegacy       *float64 `json:"diskLegacy,omitempty"`
	DiskReadLegacy   *float64 `json:"diskReadLegacy,omitempty"`
	DiskWriteLegacy  *float64 `json:"diskWriteLegacy,omitempty"`
	NetworkInLegacy  *float64 `json:"networkInLegacy,omitempty"`
	NetworkOutLegacy *float64 `json:"networkOutLegacy,omitempty"`
}

// QuietHours represents quiet hours configuration
type QuietHours struct {
	Enabled  bool            `json:"enabled"`
	Start    string          `json:"start"` // 24-hour format "HH:MM"
	End      string          `json:"end"`   // 24-hour format "HH:MM"
	Timezone string          `json:"timezone"`
	Days     map[string]bool `json:"days"` // monday, tuesday, etc.
}

// EscalationLevel represents an escalation rule
type EscalationLevel struct {
	After  int    `json:"after"`  // minutes after initial alert
	Notify string `json:"notify"` // "email", "webhook", or "all"
}

// EscalationConfig represents alert escalation configuration
type EscalationConfig struct {
	Enabled bool              `json:"enabled"`
	Levels  []EscalationLevel `json:"levels"`
}

// GroupingConfig represents alert grouping configuration
type GroupingConfig struct {
	Enabled bool `json:"enabled"`
	Window  int  `json:"window"`  // seconds
	ByNode  bool `json:"byNode"`  // Group alerts by node
	ByGuest bool `json:"byGuest"` // Group alerts by guest type
}

// ScheduleConfig represents alerting schedule configuration
type ScheduleConfig struct {
	QuietHours     QuietHours       `json:"quietHours"`
	Cooldown       int              `json:"cooldown"`       // minutes
	GroupingWindow int              `json:"groupingWindow"` // seconds (deprecated, use Grouping.Window)
	MaxAlertsHour  int              `json:"maxAlertsHour"`  // max alerts per hour per resource
	Escalation     EscalationConfig `json:"escalation"`
	Grouping       GroupingConfig   `json:"grouping"`
}

// FilterCondition represents a single filter condition
type FilterCondition struct {
	Type     string      `json:"type"` // "metric", "text", or "raw"
	Field    string      `json:"field,omitempty"`
	Operator string      `json:"operator,omitempty"`
	Value    interface{} `json:"value,omitempty"`
	RawText  string      `json:"rawText,omitempty"`
}

// FilterStack represents a collection of filters with logical operator
type FilterStack struct {
	Filters         []FilterCondition `json:"filters"`
	LogicalOperator string            `json:"logicalOperator"` // "AND" or "OR"
}

// CustomAlertRule represents a custom alert rule with filter conditions
type CustomAlertRule struct {
	ID               string          `json:"id"`
	Name             string          `json:"name"`
	Description      string          `json:"description,omitempty"`
	FilterConditions FilterStack     `json:"filterConditions"`
	Thresholds       ThresholdConfig `json:"thresholds"`
	Priority         int             `json:"priority"`
	Enabled          bool            `json:"enabled"`
	Notifications    struct {
		Email *struct {
			Enabled    bool     `json:"enabled"`
			Recipients []string `json:"recipients"`
		} `json:"email,omitempty"`
		Webhook *struct {
			Enabled bool   `json:"enabled"`
			URL     string `json:"url"`
		} `json:"webhook,omitempty"`
	} `json:"notifications"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// AlertConfig represents the complete alert configuration
type AlertConfig struct {
	Enabled        bool                       `json:"enabled"`
	GuestDefaults  ThresholdConfig            `json:"guestDefaults"`
	NodeDefaults   ThresholdConfig            `json:"nodeDefaults"`
	StorageDefault HysteresisThreshold        `json:"storageDefault"`
	Overrides      map[string]ThresholdConfig `json:"overrides"` // keyed by resource ID
	CustomRules    []CustomAlertRule          `json:"customRules,omitempty"`
	Schedule       ScheduleConfig             `json:"schedule"`
	// New configuration options
	MinimumDelta      float64        `json:"minimumDelta"`      // Minimum % change to trigger new alert
	SuppressionWindow int            `json:"suppressionWindow"` // Minutes to suppress duplicate alerts
	HysteresisMargin  float64        `json:"hysteresisMargin"`  // Default margin for legacy thresholds
	TimeThreshold     int            `json:"timeThreshold"`     // Legacy: Seconds that threshold must be exceeded before triggering
	TimeThresholds    map[string]int `json:"timeThresholds"`    // Per-type delays: guest, node, storage, pbs
}

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
type Manager struct {
	mu             sync.RWMutex
	config         AlertConfig
	activeAlerts   map[string]*Alert
	historyManager *HistoryManager
	onAlert        func(alert *Alert)
	onResolved     func(alertID string)
	onEscalate     func(alert *Alert, level int)
	escalationStop chan struct{}
	alertRateLimit map[string][]time.Time // Track alert times for rate limiting
	// New fields for deduplication and suppression
	recentAlerts    map[string]*Alert    // Track recent alerts for deduplication
	suppressedUntil map[string]time.Time // Track suppression windows
	// Recently resolved alerts (kept for 5 minutes)
	recentlyResolved map[string]*ResolvedAlert
	resolvedMutex    sync.RWMutex // Secondary lock - see Lock Ordering Documentation above
	// Time threshold tracking
	pendingAlerts map[string]time.Time // Track when thresholds were first exceeded
	// Offline confirmation tracking
	nodeOfflineCount     map[string]int // Track consecutive offline counts for nodes (legacy)
	offlineConfirmations map[string]int // Track consecutive offline counts for all resources
}

// NewManager creates a new alert manager
func NewManager() *Manager {
	alertsDir := filepath.Join(utils.GetDataDir(), "alerts")
	m := &Manager{
		activeAlerts:         make(map[string]*Alert),
		historyManager:       NewHistoryManager(alertsDir),
		escalationStop:       make(chan struct{}),
		alertRateLimit:       make(map[string][]time.Time),
		recentAlerts:         make(map[string]*Alert),
		suppressedUntil:      make(map[string]time.Time),
		recentlyResolved:     make(map[string]*ResolvedAlert),
		pendingAlerts:        make(map[string]time.Time),
		nodeOfflineCount:     make(map[string]int),
		offlineConfirmations: make(map[string]int),
		config: AlertConfig{
			Enabled: true,
			GuestDefaults: ThresholdConfig{
				CPU:        &HysteresisThreshold{Trigger: 80, Clear: 75},
				Memory:     &HysteresisThreshold{Trigger: 85, Clear: 80},
				Disk:       &HysteresisThreshold{Trigger: 90, Clear: 85},
				DiskRead:   &HysteresisThreshold{Trigger: 0, Clear: 0}, // Off by default
				DiskWrite:  &HysteresisThreshold{Trigger: 0, Clear: 0}, // Off by default
				NetworkIn:  &HysteresisThreshold{Trigger: 0, Clear: 0}, // Off by default
				NetworkOut: &HysteresisThreshold{Trigger: 0, Clear: 0}, // Off by default
			},
			NodeDefaults: ThresholdConfig{
				CPU:         &HysteresisThreshold{Trigger: 80, Clear: 75},
				Memory:      &HysteresisThreshold{Trigger: 85, Clear: 80},
				Disk:        &HysteresisThreshold{Trigger: 90, Clear: 85},
				Temperature: &HysteresisThreshold{Trigger: 80, Clear: 75}, // Warning at 80°C, clear at 75°C
			},
			StorageDefault:    HysteresisThreshold{Trigger: 85, Clear: 80},
			MinimumDelta:      2.0, // 2% minimum change
			SuppressionWindow: 5,   // 5 minutes
			HysteresisMargin:  5.0, // 5% default margin
			TimeThresholds: map[string]int{
				"guest":   10, // 10 second delay for guest CPU alerts
				"node":    15, // 15 second delay for node alerts
				"storage": 30, // 30 second delay for storage alerts
				"pbs":     30, // 30 second delay for PBS alerts
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
				},
				Cooldown:       5,  // ON - 5 minutes prevents spam
				GroupingWindow: 30, // ON - 30 seconds groups related alerts
				MaxAlertsHour:  10, // ON - 10 alerts/hour prevents flooding
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
		},
	}

	// Load saved active alerts
	if err := m.LoadActiveAlerts(); err != nil {
		log.Error().Err(err).Msg("Failed to load active alerts")
	}

	// Start escalation checker
	go m.escalationChecker()

	// Start periodic save of active alerts
	go m.periodicSaveAlerts()

	return m
}

// SetAlertCallback sets the callback for new alerts
func (m *Manager) SetAlertCallback(cb func(alert *Alert)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onAlert = cb
}

// SetResolvedCallback sets the callback for resolved alerts
func (m *Manager) SetResolvedCallback(cb func(alertID string)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onResolved = cb
}

// SetEscalateCallback sets the callback for escalated alerts
func (m *Manager) SetEscalateCallback(cb func(alert *Alert, level int)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onEscalate = cb
}

// UpdateConfig updates the alert configuration
func (m *Manager) UpdateConfig(config AlertConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Preserve defaults for zero values
	if config.StorageDefault.Trigger <= 0 {
		config.StorageDefault.Trigger = 85
		config.StorageDefault.Clear = 80
	}

	// Ensure minimums for other important fields
	if config.MinimumDelta <= 0 {
		config.MinimumDelta = 2.0
	}
	if config.SuppressionWindow <= 0 {
		config.SuppressionWindow = 5
	}
	if config.HysteresisMargin <= 0 {
		config.HysteresisMargin = 5.0
	}

	m.config = config
	for id, override := range m.config.Overrides {
		if override.Usage != nil {
			override.Usage = ensureHysteresisThreshold(override.Usage)
			m.config.Overrides[id] = override
		}
	}
	log.Info().
		Bool("enabled", config.Enabled).
		Interface("guestDefaults", config.GuestDefaults).
		Msg("Alert configuration updated")

	// Re-evaluate active alerts against new thresholds
	m.reevaluateActiveAlertsLocked()
}

// reevaluateActiveAlertsLocked re-evaluates all active alerts against the current configuration
// This should only be called with m.mu already locked
func (m *Manager) reevaluateActiveAlertsLocked() {
	if len(m.activeAlerts) == 0 {
		return
	}

	// Track alerts that should be resolved
	alertsToResolve := make([]string, 0)

	for alertID, alert := range m.activeAlerts {
		// Parse the alert ID to extract resource ID and metric type
		// Alert ID format: {resourceID}-{metricType}
		parts := strings.Split(alertID, "-")
		if len(parts) < 2 {
			continue
		}

		metricType := parts[len(parts)-1]
		resourceID := strings.Join(parts[:len(parts)-1], "-")

		// Get the appropriate threshold based on resource type and ID
		var threshold *HysteresisThreshold

		// Determine the resource type from the alert's metadata or instance
		// We need to check what kind of resource this is
		if alert.Instance == "Node" || alert.Instance == alert.Node {
			// This is a node alert
			thresholds := m.config.NodeDefaults
			threshold = getThresholdForMetric(thresholds, metricType)
		} else if alert.Instance == "Storage" || strings.Contains(alert.ResourceID, ":storage/") {
			// This is a storage alert
			if override, exists := m.config.Overrides[resourceID]; exists && override.Usage != nil {
				threshold = override.Usage
			} else {
				threshold = &m.config.StorageDefault
			}
		} else if alert.Instance == "PBS" {
			// This is a PBS alert
			thresholds := m.config.NodeDefaults
			if override, exists := m.config.Overrides[resourceID]; exists {
				if override.CPU != nil && metricType == "cpu" {
					threshold = ensureHysteresisThreshold(override.CPU)
				} else if override.Memory != nil && metricType == "memory" {
					threshold = ensureHysteresisThreshold(override.Memory)
				}
			}
			if threshold == nil {
				threshold = getThresholdForMetric(thresholds, metricType)
			}
		} else {
			// This is a guest (qemu/lxc) alert
			// We need to evaluate custom rules, but we don't have the guest object here.
			// For now, we'll mark these alerts for re-evaluation by the monitor.
			// The next poll cycle will properly evaluate them with custom rules.

			// Check if there's an override for this specific guest
			if override, exists := m.config.Overrides[resourceID]; exists {
				if override.Disabled {
					// Alert is now disabled for this resource, resolve it
					alertsToResolve = append(alertsToResolve, alertID)
					continue
				}
				threshold = getThresholdForMetricFromConfig(override, metricType)
			}

			// If no override or override doesn't have this metric, use defaults
			// Note: This doesn't consider custom rules - those will be evaluated
			// on the next poll cycle when we have the full guest object
			if threshold == nil {
				threshold = getThresholdForMetric(m.config.GuestDefaults, metricType)
			}
		}

		// If no threshold found or threshold is disabled (trigger <= 0), resolve the alert
		if threshold == nil || threshold.Trigger <= 0 {
			alertsToResolve = append(alertsToResolve, alertID)
			continue
		}

		// Check if current value is now below the clear threshold
		clearThreshold := threshold.Clear
		if clearThreshold <= 0 {
			clearThreshold = threshold.Trigger
		}

		if alert.Value <= clearThreshold {
			// Alert should be resolved due to new threshold
			alertsToResolve = append(alertsToResolve, alertID)
			log.Info().
				Str("alertID", alertID).
				Float64("value", alert.Value).
				Float64("oldThreshold", alert.Threshold).
				Float64("newClearThreshold", clearThreshold).
				Msg("Resolving alert due to threshold change")
		} else if alert.Value < threshold.Trigger {
			// Value is between clear and trigger thresholds after config change
			// Resolve it to prevent confusion
			alertsToResolve = append(alertsToResolve, alertID)
			log.Info().
				Str("alertID", alertID).
				Float64("value", alert.Value).
				Float64("newTrigger", threshold.Trigger).
				Float64("newClear", clearThreshold).
				Msg("Resolving alert - value now below trigger threshold after config change")
		}
	}

	// Resolve all alerts that should be cleared
	for _, alertID := range alertsToResolve {
		if alert, exists := m.activeAlerts[alertID]; exists {
			resolvedAlert := &ResolvedAlert{
				Alert:        alert,
				ResolvedTime: time.Now(),
			}

			// Remove from active alerts
			delete(m.activeAlerts, alertID)

			// Add to recently resolved
			m.resolvedMutex.Lock()
			m.recentlyResolved[alertID] = resolvedAlert
			m.resolvedMutex.Unlock()

			log.Info().
				Str("alertID", alertID).
				Msg("Alert auto-resolved after configuration change")
		}
	}

	// Save updated active alerts if any were resolved
	if len(alertsToResolve) > 0 {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Error().Interface("panic", r).Msg("Panic in SaveActiveAlerts goroutine (config update)")
				}
			}()
			if err := m.SaveActiveAlerts(); err != nil {
				log.Error().Err(err).Msg("Failed to save active alerts after config update")
			}
		}()
	}
}

// ReevaluateGuestAlert reevaluates a specific guest's alerts with full threshold resolution including custom rules
// This should be called by the monitor with the current guest state
func (m *Manager) ReevaluateGuestAlert(guest interface{}, guestID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Get the correct thresholds for this guest (includes custom rules evaluation)
	thresholds := m.getGuestThresholds(guest, guestID)

	// Check all metric types for this guest
	metricTypes := []string{"cpu", "memory", "disk", "diskRead", "diskWrite", "networkIn", "networkOut"}

	for _, metricType := range metricTypes {
		alertID := fmt.Sprintf("%s-%s", guestID, metricType)
		alert, exists := m.activeAlerts[alertID]
		if !exists {
			continue
		}

		// Get the threshold for this metric
		var threshold *HysteresisThreshold
		switch metricType {
		case "cpu":
			threshold = thresholds.CPU
		case "memory":
			threshold = thresholds.Memory
		case "disk":
			threshold = thresholds.Disk
		case "diskRead":
			threshold = thresholds.DiskRead
		case "diskWrite":
			threshold = thresholds.DiskWrite
		case "networkIn":
			threshold = thresholds.NetworkIn
		case "networkOut":
			threshold = thresholds.NetworkOut
		}

		// If threshold is disabled or doesn't exist, clear the alert
		if threshold == nil || threshold.Trigger <= 0 {
			m.clearAlertNoLock(alertID)
			log.Info().
				Str("alertID", alertID).
				Str("metric", metricType).
				Msg("Cleared alert - threshold disabled")
			continue
		}

		// Check if alert should be cleared based on new threshold
		clearThreshold := threshold.Clear
		if clearThreshold <= 0 {
			clearThreshold = threshold.Trigger
		}

		if alert.Value <= clearThreshold || alert.Value < threshold.Trigger {
			m.clearAlertNoLock(alertID)
			log.Info().
				Str("alertID", alertID).
				Str("metric", metricType).
				Float64("value", alert.Value).
				Float64("trigger", threshold.Trigger).
				Float64("clear", clearThreshold).
				Msg("Cleared alert - value now below threshold after config change")
		}
	}
}

// getThresholdForMetric returns the threshold for a specific metric type from a ThresholdConfig
func getThresholdForMetric(config ThresholdConfig, metricType string) *HysteresisThreshold {
	switch metricType {
	case "cpu":
		return config.CPU
	case "memory":
		return config.Memory
	case "disk":
		return config.Disk
	case "diskRead":
		return config.DiskRead
	case "diskWrite":
		return config.DiskWrite
	case "networkIn":
		return config.NetworkIn
	case "networkOut":
		return config.NetworkOut
	case "temperature":
		return config.Temperature
	case "usage":
		return config.Usage
	default:
		return nil
	}
}

// getThresholdForMetricFromConfig returns the threshold for a specific metric type from a ThresholdConfig
// ensuring hysteresis is properly set
func getThresholdForMetricFromConfig(config ThresholdConfig, metricType string) *HysteresisThreshold {
	var threshold *HysteresisThreshold
	switch metricType {
	case "cpu":
		if config.CPU != nil {
			threshold = ensureHysteresisThreshold(config.CPU)
		}
	case "memory":
		if config.Memory != nil {
			threshold = ensureHysteresisThreshold(config.Memory)
		}
	case "disk":
		if config.Disk != nil {
			threshold = ensureHysteresisThreshold(config.Disk)
		}
	case "diskRead":
		if config.DiskRead != nil {
			threshold = ensureHysteresisThreshold(config.DiskRead)
		}
	case "diskWrite":
		if config.DiskWrite != nil {
			threshold = ensureHysteresisThreshold(config.DiskWrite)
		}
	case "networkIn":
		if config.NetworkIn != nil {
			threshold = ensureHysteresisThreshold(config.NetworkIn)
		}
	case "networkOut":
		if config.NetworkOut != nil {
			threshold = ensureHysteresisThreshold(config.NetworkOut)
		}
	case "temperature":
		if config.Temperature != nil {
			threshold = ensureHysteresisThreshold(config.Temperature)
		}
	case "usage":
		if config.Usage != nil {
			threshold = ensureHysteresisThreshold(config.Usage)
		}
	}
	return threshold
}

// isInQuietHours checks if the current time is within quiet hours
func (m *Manager) isInQuietHours() bool {
	if !m.config.Schedule.QuietHours.Enabled {
		return false
	}

	// Load timezone
	loc, err := time.LoadLocation(m.config.Schedule.QuietHours.Timezone)
	if err != nil {
		log.Warn().Err(err).Str("timezone", m.config.Schedule.QuietHours.Timezone).Msg("Failed to load timezone, using local time")
		loc = time.Local
	}

	now := time.Now().In(loc)
	dayName := strings.ToLower(now.Format("Monday"))

	// Check if today is enabled for quiet hours
	if enabled, ok := m.config.Schedule.QuietHours.Days[dayName]; !ok || !enabled {
		return false
	}

	// Parse start and end times
	startTime, err := time.ParseInLocation("15:04", m.config.Schedule.QuietHours.Start, loc)
	if err != nil {
		log.Warn().Err(err).Str("start", m.config.Schedule.QuietHours.Start).Msg("Failed to parse quiet hours start time")
		return false
	}

	endTime, err := time.ParseInLocation("15:04", m.config.Schedule.QuietHours.End, loc)
	if err != nil {
		log.Warn().Err(err).Str("end", m.config.Schedule.QuietHours.End).Msg("Failed to parse quiet hours end time")
		return false
	}

	// Set to today's date
	startTime = time.Date(now.Year(), now.Month(), now.Day(), startTime.Hour(), startTime.Minute(), 0, 0, loc)
	endTime = time.Date(now.Year(), now.Month(), now.Day(), endTime.Hour(), endTime.Minute(), 0, 0, loc)

	// Handle overnight quiet hours (e.g., 22:00 to 08:00)
	if endTime.Before(startTime) {
		// If we're past the start time or before the end time
		if now.After(startTime) || now.Before(endTime) {
			return true
		}
	} else {
		// Normal case (e.g., 08:00 to 17:00)
		if now.After(startTime) && now.Before(endTime) {
			return true
		}
	}

	return false
}

// GetConfig returns the current alert configuration
func (m *Manager) GetConfig() AlertConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// CheckGuest checks a guest (VM or container) against thresholds
func (m *Manager) CheckGuest(guest interface{}, instanceName string) {
	m.mu.RLock()
	if !m.config.Enabled {
		m.mu.RUnlock()
		log.Debug().Msg("CheckGuest: alerts disabled globally")
		return
	}
	m.mu.RUnlock()

	var guestID, name, node, guestType, status string
	var cpu, memUsage, diskUsage float64
	var diskRead, diskWrite, netIn, netOut int64

	// Extract data based on guest type
	switch g := guest.(type) {
	case models.VM:
		guestID = g.ID
		name = g.Name
		node = g.Node
		status = g.Status
		guestType = "VM"
		cpu = g.CPU * 100 // Convert to percentage
		memUsage = g.Memory.Usage
		diskUsage = g.Disk.Usage
		diskRead = g.DiskRead
		diskWrite = g.DiskWrite
		netIn = g.NetworkIn
		netOut = g.NetworkOut

		// Debug logging for high memory VMs
		if memUsage > 85 {
			log.Info().
				Str("vm", name).
				Float64("memUsage", memUsage).
				Str("status", status).
				Msg("VM with high memory detected in CheckGuest")
		}
	case models.Container:
		guestID = g.ID
		name = g.Name
		node = g.Node
		status = g.Status
		guestType = "Container"
		cpu = g.CPU * 100 // Convert to percentage
		memUsage = g.Memory.Usage
		diskUsage = g.Disk.Usage
		diskRead = g.DiskRead
		diskWrite = g.DiskWrite
		netIn = g.NetworkIn
		netOut = g.NetworkOut
	default:
		log.Debug().
			Str("type", fmt.Sprintf("%T", guest)).
			Msg("CheckGuest: unsupported guest type")
		return
	}

	// Handle non-running guests
	// Proxmox VM states: running, stopped, paused, suspended
	if status != "running" {
		// Check for powered-off state and generate alert if configured
		if status == "stopped" {
			m.checkGuestPoweredOff(guestID, name, node, instanceName, guestType)
		} else {
			// For paused/suspended, clear powered-off alert
			m.clearGuestPoweredOffAlert(guestID, name)
		}

		// Clear all resource metric alerts (cpu, memory, disk, etc.) for non-running guests
		m.mu.Lock()
		alertsCleared := 0
		for alertID, alert := range m.activeAlerts {
			// Only clear resource metric alerts, not powered-off alerts
			if alert.ResourceID == guestID && alert.Type != "powered-off" {
				delete(m.activeAlerts, alertID)
				alertsCleared++
				log.Debug().
					Str("alertID", alertID).
					Str("guest", name).
					Str("status", status).
					Msg("Cleared metric alert for non-running guest")
			}
		}
		m.mu.Unlock()

		if alertsCleared > 0 {
			log.Debug().
				Str("guest", name).
				Str("status", status).
				Int("alertsCleared", alertsCleared).
				Msg("Cleared metric alerts for non-running guest")
		}
		return
	}

	// If guest is running, clear any powered-off alert
	m.clearGuestPoweredOffAlert(guestID, name)

	// Get thresholds (check custom rules, then overrides, then defaults)
	m.mu.RLock()
	thresholds := m.getGuestThresholds(guest, guestID)
	m.mu.RUnlock()

	// If alerts are disabled for this guest, clear any existing alerts and return
	if thresholds.Disabled {
		m.mu.Lock()
		for alertID, alert := range m.activeAlerts {
			if alert.ResourceID == guestID {
				delete(m.activeAlerts, alertID)
				log.Info().
					Str("alertID", alertID).
					Str("guest", name).
					Msg("Cleared alert - guest has alerts disabled")
			}
		}
		m.mu.Unlock()
		return
	}

	// Check each metric
	log.Info().
		Str("guest", name).
		Float64("cpu", cpu).
		Float64("memory", memUsage).
		Float64("disk", diskUsage).
		Interface("thresholds", thresholds).
		Msg("Checking guest thresholds")

	// Check thresholds
	m.checkMetric(guestID, name, node, instanceName, guestType, "cpu", cpu, thresholds.CPU)
	m.checkMetric(guestID, name, node, instanceName, guestType, "memory", memUsage, thresholds.Memory)
	m.checkMetric(guestID, name, node, instanceName, guestType, "disk", diskUsage, thresholds.Disk)

	// Check I/O metrics (convert bytes/s to MB/s)
	if thresholds.DiskRead != nil && thresholds.DiskRead.Trigger > 0 {
		m.checkMetric(guestID, name, node, instanceName, guestType, "diskRead", float64(diskRead)/1024/1024, thresholds.DiskRead)
	}
	if thresholds.DiskWrite != nil && thresholds.DiskWrite.Trigger > 0 {
		m.checkMetric(guestID, name, node, instanceName, guestType, "diskWrite", float64(diskWrite)/1024/1024, thresholds.DiskWrite)
	}
	if thresholds.NetworkIn != nil && thresholds.NetworkIn.Trigger > 0 {
		m.checkMetric(guestID, name, node, instanceName, guestType, "networkIn", float64(netIn)/1024/1024, thresholds.NetworkIn)
	}
	if thresholds.NetworkOut != nil && thresholds.NetworkOut.Trigger > 0 {
		m.checkMetric(guestID, name, node, instanceName, guestType, "networkOut", float64(netOut)/1024/1024, thresholds.NetworkOut)
	}
}

// CheckNode checks a node against thresholds
func (m *Manager) CheckNode(node models.Node) {
	m.mu.RLock()
	if !m.config.Enabled {
		m.mu.RUnlock()
		return
	}
	thresholds := m.config.NodeDefaults
	m.mu.RUnlock()

	// CRITICAL: Check if node is offline first
	if node.Status == "offline" || node.ConnectionHealth == "error" || node.ConnectionHealth == "failed" {
		m.checkNodeOffline(node)
	} else {
		// Clear any existing offline alert if node is back online
		m.clearNodeOfflineAlert(node)
	}

	// Check each metric (only if node is online)
	if node.Status != "offline" {
		m.checkMetric(node.ID, node.Name, node.Name, node.Instance, "Node", "cpu", node.CPU*100, thresholds.CPU)
		m.checkMetric(node.ID, node.Name, node.Name, node.Instance, "Node", "memory", node.Memory.Usage, thresholds.Memory)
		m.checkMetric(node.ID, node.Name, node.Name, node.Instance, "Node", "disk", node.Disk.Usage, thresholds.Disk)

		// Check temperature if available
		if node.Temperature != nil && node.Temperature.Available && thresholds.Temperature != nil {
			// Use CPU package temp if available, otherwise use max core temp
			temp := node.Temperature.CPUPackage
			if temp == 0 {
				temp = node.Temperature.CPUMax
			}
			m.checkMetric(node.ID, node.Name, node.Name, node.Instance, "Node", "temperature", temp, thresholds.Temperature)
		}
	}
}

// CheckPBS checks PBS instance metrics against thresholds
func (m *Manager) CheckPBS(pbs models.PBSInstance) {
	m.mu.RLock()
	if !m.config.Enabled {
		m.mu.RUnlock()
		return
	}

	// Check if there's an override for this PBS instance
	override, hasOverride := m.config.Overrides[pbs.ID]

	// Use node defaults for PBS (same as nodes: CPU, Memory)
	cpuThreshold := m.config.NodeDefaults.CPU
	memoryThreshold := m.config.NodeDefaults.Memory
	m.mu.RUnlock()

	// Check if PBS is offline first (similar to nodes)
	if pbs.Status == "offline" || pbs.ConnectionHealth == "error" || pbs.ConnectionHealth == "unhealthy" {
		m.checkPBSOffline(pbs)
	} else {
		// Clear any existing offline alert if PBS is back online
		m.clearPBSOfflineAlert(pbs)
	}

	// If alerts are disabled for this PBS instance, clear any existing alerts and return
	if hasOverride && override.Disabled {
		m.mu.Lock()
		// Clear CPU alert
		cpuAlertID := fmt.Sprintf("%s-cpu", pbs.ID)
		if _, exists := m.activeAlerts[cpuAlertID]; exists {
			delete(m.activeAlerts, cpuAlertID)
			log.Info().
				Str("alertID", cpuAlertID).
				Str("pbs", pbs.Name).
				Msg("Cleared CPU alert - PBS has alerts disabled")
		}
		// Clear Memory alert
		memAlertID := fmt.Sprintf("%s-memory", pbs.ID)
		if _, exists := m.activeAlerts[memAlertID]; exists {
			delete(m.activeAlerts, memAlertID)
			log.Info().
				Str("alertID", memAlertID).
				Str("pbs", pbs.Name).
				Msg("Cleared Memory alert - PBS has alerts disabled")
		}
		// Clear offline alert
		offlineAlertID := fmt.Sprintf("pbs-offline-%s", pbs.ID)
		if _, exists := m.activeAlerts[offlineAlertID]; exists {
			delete(m.activeAlerts, offlineAlertID)
			log.Info().
				Str("alertID", offlineAlertID).
				Str("pbs", pbs.Name).
				Msg("Cleared offline alert - PBS has alerts disabled")
		}
		m.mu.Unlock()
		return
	}

	// Check if there are custom thresholds for this PBS instance
	if hasOverride {
		if override.CPU != nil {
			cpuThreshold = override.CPU
		}
		if override.Memory != nil {
			memoryThreshold = override.Memory
		}
	}

	// Check metrics only if PBS is online
	if pbs.Status != "offline" {
		// PBS CPU is already a percentage
		m.checkMetric(pbs.ID, pbs.Name, pbs.Host, pbs.Name, "PBS", "cpu", pbs.CPU, cpuThreshold)
		// PBS Memory is already a percentage
		m.checkMetric(pbs.ID, pbs.Name, pbs.Host, pbs.Name, "PBS", "memory", pbs.Memory, memoryThreshold)
	}
}

// CheckStorage checks storage against thresholds
func (m *Manager) CheckStorage(storage models.Storage) {
	m.mu.RLock()
	if !m.config.Enabled {
		m.mu.RUnlock()
		return
	}

	// Check if there's an override for this storage device
	override, hasOverride := m.config.Overrides[storage.ID]
	threshold := m.config.StorageDefault

	// Apply override if it exists for usage threshold
	if hasOverride && override.Usage != nil {
		threshold = *override.Usage
	}
	m.mu.RUnlock()

	// Check if storage is truly offline/unavailable (not just inactive from other nodes)
	// Note: In a cluster, local storage from other nodes shows as inactive which is normal
	if storage.Status == "offline" || storage.Status == "unavailable" {
		m.checkStorageOffline(storage)
	} else {
		// Clear any existing offline alert if storage is back online
		m.clearStorageOfflineAlert(storage)
	}

	// If alerts are disabled for this storage device, clear any existing alerts and return
	if hasOverride && override.Disabled {
		m.mu.Lock()
		// Clear usage alert
		usageAlertID := fmt.Sprintf("%s-usage", storage.ID)
		if _, exists := m.activeAlerts[usageAlertID]; exists {
			delete(m.activeAlerts, usageAlertID)
			log.Info().
				Str("alertID", usageAlertID).
				Str("storage", storage.Name).
				Msg("Cleared usage alert - storage has alerts disabled")
		}
		// Clear offline alert
		offlineAlertID := fmt.Sprintf("storage-offline-%s", storage.ID)
		if _, exists := m.activeAlerts[offlineAlertID]; exists {
			delete(m.activeAlerts, offlineAlertID)
			log.Info().
				Str("alertID", offlineAlertID).
				Str("storage", storage.Name).
				Msg("Cleared offline alert - storage has alerts disabled")
		}
		m.mu.Unlock()
		return
	}

	// Check usage if storage has valid data (even if not currently active on this node)
	// In clusters, storage may show as inactive on nodes where it's not currently mounted
	// but we still want to alert on high usage
	log.Info().
		Str("storage", storage.Name).
		Str("id", storage.ID).
		Float64("usage", storage.Usage).
		Str("status", storage.Status).
		Float64("trigger", threshold.Trigger).
		Float64("clear", threshold.Clear).
		Bool("hasOverride", hasOverride).
		Msg("Checking storage thresholds")

	if storage.Status != "offline" && storage.Status != "unavailable" && storage.Usage > 0 {
		m.checkMetric(storage.ID, storage.Name, storage.Node, storage.Instance, "Storage", "usage", storage.Usage, &threshold)
	}

	// Check ZFS pool status if this is ZFS storage
	if storage.ZFSPool != nil {
		m.checkZFSPoolHealth(storage)
	}
}

// checkZFSPoolHealth checks ZFS pool for errors and degraded state
func (m *Manager) checkZFSPoolHealth(storage models.Storage) {
	pool := storage.ZFSPool
	if pool == nil {
		return
	}

	// Check pool state (DEGRADED, FAULTED, etc.)
	stateAlertID := fmt.Sprintf("zfs-pool-state-%s", storage.ID)
	if pool.State != "ONLINE" {
		level := AlertLevelWarning
		if pool.State == "FAULTED" || pool.State == "UNAVAIL" {
			level = AlertLevelCritical
		}

		m.mu.Lock()
		if _, exists := m.activeAlerts[stateAlertID]; !exists {
			alert := &Alert{
				ID:           stateAlertID,
				Type:         "zfs-pool-state",
				Level:        level,
				ResourceID:   storage.ID,
				ResourceName: fmt.Sprintf("%s (%s)", storage.Name, pool.Name),
				Node:         storage.Node,
				Instance:     storage.Instance,
				Message:      fmt.Sprintf("ZFS pool '%s' is %s", pool.Name, pool.State),
				Value:        0,
				Threshold:    0,
				StartTime:    time.Now(),
				LastSeen:     time.Now(),
				Metadata: map[string]interface{}{
					"pool_name":  pool.Name,
					"pool_state": pool.State,
				},
			}

			m.activeAlerts[stateAlertID] = alert
			m.recentAlerts[stateAlertID] = alert
			m.historyManager.AddAlert(*alert)

			if m.onAlert != nil {
				m.onAlert(alert)
			}

			log.Warn().
				Str("pool", pool.Name).
				Str("state", pool.State).
				Str("node", storage.Node).
				Msg("ZFS pool is not healthy")
		}
		m.mu.Unlock()
	} else {
		// Clear state alert if pool is back online
		m.clearAlert(stateAlertID)
	}

	// Check for read/write/checksum errors
	totalErrors := pool.ReadErrors + pool.WriteErrors + pool.ChecksumErrors
	errorsAlertID := fmt.Sprintf("zfs-pool-errors-%s", storage.ID)
	if totalErrors > 0 {
		m.mu.Lock()
		existingAlert, exists := m.activeAlerts[errorsAlertID]

		// Only create new alert or update if error count increased
		if !exists || float64(totalErrors) > existingAlert.Value {
			alert := &Alert{
				ID:           errorsAlertID,
				Type:         "zfs-pool-errors",
				Level:        AlertLevelWarning,
				ResourceID:   storage.ID,
				ResourceName: fmt.Sprintf("%s (%s)", storage.Name, pool.Name),
				Node:         storage.Node,
				Instance:     storage.Instance,
				Message: fmt.Sprintf("ZFS pool '%s' has errors: %d read, %d write, %d checksum",
					pool.Name, pool.ReadErrors, pool.WriteErrors, pool.ChecksumErrors),
				Value:     float64(totalErrors),
				Threshold: 0,
				StartTime: time.Now(),
				LastSeen:  time.Now(),
				Metadata: map[string]interface{}{
					"pool_name":       pool.Name,
					"read_errors":     pool.ReadErrors,
					"write_errors":    pool.WriteErrors,
					"checksum_errors": pool.ChecksumErrors,
				},
			}

			if exists {
				// Preserve original start time when updating
				alert.StartTime = existingAlert.StartTime
			}

			m.activeAlerts[errorsAlertID] = alert
			m.recentAlerts[errorsAlertID] = alert
			m.historyManager.AddAlert(*alert)

			if m.onAlert != nil {
				m.onAlert(alert)
			}

			log.Error().
				Str("pool", pool.Name).
				Int64("read_errors", pool.ReadErrors).
				Int64("write_errors", pool.WriteErrors).
				Int64("checksum_errors", pool.ChecksumErrors).
				Str("node", storage.Node).
				Msg("ZFS pool has I/O errors")
		}
		m.mu.Unlock()
	} else {
		m.clearAlert(errorsAlertID)
	}

	// Check individual devices for errors
	m.mu.Lock()
	for _, device := range pool.Devices {
		alertID := fmt.Sprintf("zfs-device-%s-%s", storage.ID, device.Name)

		// Skip SPARE devices unless they have actual errors
		if (device.State != "ONLINE" && device.State != "SPARE") || device.ReadErrors > 0 || device.WriteErrors > 0 || device.ChecksumErrors > 0 {
			if _, exists := m.activeAlerts[alertID]; !exists {
				level := AlertLevelWarning
				if device.State == "FAULTED" || device.State == "UNAVAIL" {
					level = AlertLevelCritical
				}

				message := fmt.Sprintf("ZFS device '%s' in pool '%s'", device.Name, pool.Name)
				if device.State != "ONLINE" {
					message += fmt.Sprintf(" is %s", device.State)
				}
				if device.ReadErrors > 0 || device.WriteErrors > 0 || device.ChecksumErrors > 0 {
					message += fmt.Sprintf(" has errors: %d read, %d write, %d checksum",
						device.ReadErrors, device.WriteErrors, device.ChecksumErrors)
				}

				alert := &Alert{
					ID:           alertID,
					Type:         "zfs-device",
					Level:        level,
					ResourceID:   storage.ID,
					ResourceName: fmt.Sprintf("%s (%s/%s)", storage.Name, pool.Name, device.Name),
					Node:         storage.Node,
					Instance:     storage.Instance,
					Message:      message,
					Value:        float64(device.ReadErrors + device.WriteErrors + device.ChecksumErrors),
					Threshold:    0,
					StartTime:    time.Now(),
					LastSeen:     time.Now(),
					Metadata: map[string]interface{}{
						"pool_name":       pool.Name,
						"device_name":     device.Name,
						"device_state":    device.State,
						"read_errors":     device.ReadErrors,
						"write_errors":    device.WriteErrors,
						"checksum_errors": device.ChecksumErrors,
					},
				}

				m.activeAlerts[alertID] = alert
				m.recentAlerts[alertID] = alert
				m.historyManager.AddAlert(*alert)

				if m.onAlert != nil {
					m.onAlert(alert)
				}

				log.Warn().
					Str("pool", pool.Name).
					Str("device", device.Name).
					Str("state", device.State).
					Int64("errors", device.ReadErrors+device.WriteErrors+device.ChecksumErrors).
					Str("node", storage.Node).
					Msg("ZFS device has issues")
			}
		} else {
			// Clear device alert if it's back to normal
			m.clearAlertNoLock(alertID)
		}
	}
	m.mu.Unlock()
}

// clearAlert removes an alert if it exists
func (m *Manager) clearAlert(alertID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if alert, exists := m.activeAlerts[alertID]; exists {
		delete(m.activeAlerts, alertID)

		// Add to recently resolved
		resolvedAlert := &ResolvedAlert{
			Alert:        alert,
			ResolvedTime: time.Now(),
		}
		m.recentlyResolved[alertID] = resolvedAlert

		// Send recovery notification
		if m.onResolved != nil {
			m.onResolved(alertID)
		}

		log.Info().
			Str("alertID", alertID).
			Msg("Alert cleared")
	}
}

// getTimeThresholdForType returns the appropriate time threshold for the resource type
func (m *Manager) getTimeThresholdForType(resourceType string) int {
	// Use per-type thresholds if available
	if m.config.TimeThresholds != nil {
		switch resourceType {
		case "qemu", "lxc", "guest":
			if delay, ok := m.config.TimeThresholds["guest"]; ok {
				return delay
			}
		case "node":
			if delay, ok := m.config.TimeThresholds["node"]; ok {
				return delay
			}
		case "storage":
			if delay, ok := m.config.TimeThresholds["storage"]; ok {
				return delay
			}
		case "pbs":
			if delay, ok := m.config.TimeThresholds["pbs"]; ok {
				return delay
			}
		}
	}

	// Fall back to legacy single threshold
	return m.config.TimeThreshold
}

// checkMetric checks a single metric against its threshold with hysteresis
func (m *Manager) checkMetric(resourceID, resourceName, node, instance, resourceType, metricType string, value float64, threshold *HysteresisThreshold) {
	if threshold == nil || threshold.Trigger <= 0 {
		return
	}

	log.Debug().
		Str("resource", resourceName).
		Str("metric", metricType).
		Float64("value", value).
		Float64("trigger", threshold.Trigger).
		Float64("clear", threshold.Clear).
		Bool("exceeds", value >= threshold.Trigger).
		Msg("Checking metric threshold")

	alertID := fmt.Sprintf("%s-%s", resourceID, metricType)

	m.mu.Lock()
	defer m.mu.Unlock()

	existingAlert, exists := m.activeAlerts[alertID]

	// Check for suppression
	if suppressUntil, suppressed := m.suppressedUntil[alertID]; suppressed && time.Now().Before(suppressUntil) {
		log.Debug().
			Str("alertID", alertID).
			Time("suppressedUntil", suppressUntil).
			Msg("Alert suppressed")
		return
	}

	if value >= threshold.Trigger {
		// Threshold exceeded
		if !exists {
			// Determine the appropriate time threshold based on resource type
			timeThreshold := m.getTimeThresholdForType(resourceType)

			// Check if we have a time threshold configured
			if timeThreshold > 0 {
				// Check if this threshold was already pending
				if pendingTime, isPending := m.pendingAlerts[alertID]; isPending {
					// Check if enough time has passed
					if time.Since(pendingTime) >= time.Duration(timeThreshold)*time.Second {
						// Time threshold met, proceed with alert
						delete(m.pendingAlerts, alertID)
						log.Debug().
							Str("alertID", alertID).
							Int("timeThreshold", timeThreshold).
							Dur("elapsed", time.Since(pendingTime)).
							Msg("Time threshold met, triggering alert")
					} else {
						// Still waiting for time threshold
						log.Debug().
							Str("alertID", alertID).
							Int("timeThreshold", timeThreshold).
							Dur("elapsed", time.Since(pendingTime)).
							Msg("Threshold exceeded but waiting for time threshold")
						return
					}
				} else {
					// First time exceeding threshold, start tracking
					m.pendingAlerts[alertID] = time.Now()
					log.Debug().
						Str("alertID", alertID).
						Int("timeThreshold", timeThreshold).
						Msg("Threshold exceeded, starting time threshold tracking")
					return
				}
			}

			// Check for recent similar alert to prevent spam
			if recent, hasRecent := m.recentAlerts[alertID]; hasRecent {
				// Check minimum delta
				if m.config.MinimumDelta > 0 &&
					time.Since(recent.StartTime) < time.Duration(m.config.SuppressionWindow)*time.Minute &&
					abs(recent.Value-value) < m.config.MinimumDelta {
					log.Debug().
						Str("alertID", alertID).
						Float64("recentValue", recent.Value).
						Float64("currentValue", value).
						Float64("delta", abs(recent.Value-value)).
						Float64("minimumDelta", m.config.MinimumDelta).
						Msg("Alert suppressed due to minimum delta")

					// Set suppression window
					m.suppressedUntil[alertID] = time.Now().Add(time.Duration(m.config.SuppressionWindow) * time.Minute)
					return
				}
			}

			// New alert
			alert := &Alert{
				ID:           alertID,
				Type:         metricType,
				Level:        AlertLevelWarning,
				ResourceID:   resourceID,
				ResourceName: resourceName,
				Node:         node,
				Instance:     instance,
				Message: func() string {
					if metricType == "usage" {
						return fmt.Sprintf("%s at %.1f%%", resourceType, value)
					}
					// For I/O metrics (diskRead, diskWrite, networkIn, networkOut), show MB/s not percentage
					if metricType == "diskRead" || metricType == "diskWrite" ||
						metricType == "networkIn" || metricType == "networkOut" {
						return fmt.Sprintf("%s %s at %.1f MB/s", resourceType, metricType, value)
					}
					// For CPU, memory, disk metrics show percentage
					return fmt.Sprintf("%s %s at %.1f%%", resourceType, metricType, value)
				}(),
				Value:     value,
				Threshold: threshold.Trigger,
				StartTime: time.Now(),
				LastSeen:  time.Now(),
				Metadata: map[string]interface{}{
					"resourceType":   resourceType,
					"clearThreshold": threshold.Clear,
				},
			}

			// Set level based on how much over threshold
			if value >= threshold.Trigger+10 {
				alert.Level = AlertLevelCritical
			}

			m.activeAlerts[alertID] = alert
			m.recentAlerts[alertID] = alert
			m.historyManager.AddAlert(*alert)

			// Save active alerts after adding new one
			go func() {
				defer func() {
					if r := recover(); r != nil {
						log.Error().Interface("panic", r).Msg("Panic in SaveActiveAlerts goroutine")
					}
				}()
				if err := m.SaveActiveAlerts(); err != nil {
					log.Error().Err(err).Msg("Failed to save active alerts after creation")
				}
			}()

			log.Warn().
				Str("alertID", alertID).
				Str("resource", resourceName).
				Str("metric", metricType).
				Float64("value", value).
				Float64("trigger", threshold.Trigger).
				Float64("clear", threshold.Clear).
				Int("activeAlerts", len(m.activeAlerts)).
				Msg("Alert triggered")

			// Check rate limit (but don't remove alert from tracking)
			if !m.checkRateLimit(alertID) {
				log.Debug().
					Str("alertID", alertID).
					Int("maxPerHour", m.config.Schedule.MaxAlertsHour).
					Msg("Alert notification suppressed due to rate limit")
				// Don't delete the alert, just suppress notifications
				return
			}

			// Check if we should suppress notifications due to quiet hours
			if m.isInQuietHours() && alert.Level != AlertLevelCritical {
				log.Debug().
					Str("alertID", alertID).
					Msg("Alert notification suppressed due to quiet hours (non-critical)")
			} else {
				// Notify callback
				if m.onAlert != nil {
					log.Info().Str("alertID", alertID).Msg("Calling onAlert callback")
					go m.onAlert(alert)
				} else {
					log.Warn().Msg("No onAlert callback set!")
				}
			}
		} else {
			// Update existing alert
			existingAlert.LastSeen = time.Now()
			existingAlert.Value = value

			// Update level if needed
			if value >= threshold.Trigger+10 {
				existingAlert.Level = AlertLevelCritical
			} else {
				existingAlert.Level = AlertLevelWarning
			}
		}
	} else {
		// Value is below trigger threshold
		// Clear any pending alert for this metric
		if _, isPending := m.pendingAlerts[alertID]; isPending {
			delete(m.pendingAlerts, alertID)
			log.Debug().
				Str("alertID", alertID).
				Msg("Value dropped below threshold, clearing pending alert")
		}

		if exists {
			// Use hysteresis for resolution - only resolve if below clear threshold
			clearThreshold := threshold.Clear
			if clearThreshold <= 0 {
				clearThreshold = threshold.Trigger // Fallback to trigger if clear not set
			}

			if value <= clearThreshold {
				// Threshold cleared with hysteresis - auto resolve
				resolvedAlert := &ResolvedAlert{
					Alert:        existingAlert,
					ResolvedTime: time.Now(),
				}

				// Remove from active alerts
				delete(m.activeAlerts, alertID)

				// Save active alerts after resolution
				go func() {
					defer func() {
						if r := recover(); r != nil {
							log.Error().Interface("panic", r).Msg("Panic in SaveActiveAlerts goroutine (resolution)")
						}
					}()
					if err := m.SaveActiveAlerts(); err != nil {
						log.Error().Err(err).Msg("Failed to save active alerts after resolution")
					}
				}()

				// Add to recently resolved
				m.resolvedMutex.Lock()
				m.recentlyResolved[alertID] = resolvedAlert
				m.resolvedMutex.Unlock()

				log.Info().
					Str("alertID", alertID).
					Int("totalRecentlyResolved", len(m.recentlyResolved)).
					Msg("Added alert to recently resolved")

				// Schedule cleanup after 5 minutes
				go func() {
					defer func() {
						if r := recover(); r != nil {
							log.Error().Interface("panic", r).Str("alertID", alertID).Msg("Panic in cleanup goroutine")
						}
					}()
					time.Sleep(5 * time.Minute)
					m.resolvedMutex.Lock()
					delete(m.recentlyResolved, alertID)
					m.resolvedMutex.Unlock()
				}()

				log.Info().
					Str("resource", resourceName).
					Str("metric", metricType).
					Float64("value", value).
					Float64("clearThreshold", clearThreshold).
					Bool("wasAcknowledged", existingAlert.Acknowledged).
					Msg("Alert resolved with hysteresis")

				if m.onResolved != nil {
					go m.onResolved(alertID)
				}
			}
		}
	}
}

// abs returns the absolute value of a float64
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// AcknowledgeAlert acknowledges an alert
func (m *Manager) AcknowledgeAlert(alertID, user string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	alert, exists := m.activeAlerts[alertID]
	if !exists {
		return fmt.Errorf("alert not found: %s", alertID)
	}

	alert.Acknowledged = true
	now := time.Now()
	alert.AckTime = &now
	alert.AckUser = user

	// Write the modified alert back to the map
	m.activeAlerts[alertID] = alert

	return nil
}

// UnacknowledgeAlert removes the acknowledged status from an alert
func (m *Manager) UnacknowledgeAlert(alertID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	alert, exists := m.activeAlerts[alertID]
	if !exists {
		return fmt.Errorf("alert not found: %s", alertID)
	}

	alert.Acknowledged = false
	alert.AckTime = nil
	alert.AckUser = ""

	// Write the modified alert back to the map
	m.activeAlerts[alertID] = alert

	log.Info().
		Str("alertID", alertID).
		Msg("Alert unacknowledged")

	return nil
}

// GetActiveAlerts returns all active alerts
func (m *Manager) GetActiveAlerts() []Alert {
	m.mu.RLock()
	// Make a quick copy of the map to avoid holding the lock too long
	alertsCopy := make(map[string]*Alert, len(m.activeAlerts))
	for k, v := range m.activeAlerts {
		alertsCopy[k] = v
	}
	m.mu.RUnlock()

	alerts := make([]Alert, 0, len(alertsCopy))
	for _, alert := range alertsCopy {
		alerts = append(alerts, *alert)
	}
	return alerts
}

// GetRecentlyResolved returns recently resolved alerts
func (m *Manager) GetRecentlyResolved() []models.ResolvedAlert {
	m.resolvedMutex.RLock()
	defer m.resolvedMutex.RUnlock()

	resolved := make([]models.ResolvedAlert, 0, len(m.recentlyResolved))
	for _, alert := range m.recentlyResolved {
		resolved = append(resolved, models.ResolvedAlert{
			Alert: models.Alert{
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
			},
			ResolvedTime: alert.ResolvedTime,
		})
	}
	return resolved
}

// GetAlertHistory returns alert history
func (m *Manager) GetAlertHistory(limit int) []Alert {
	return m.historyManager.GetAllHistory(limit)
}

// ClearAlertHistory clears all alert history
func (m *Manager) ClearAlertHistory() error {
	return m.historyManager.ClearAllHistory()
}

// checkNodeOffline creates an alert for offline nodes after confirmation
func (m *Manager) checkNodeOffline(node models.Node) {
	alertID := fmt.Sprintf("node-offline-%s", node.ID)

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if node connectivity alerts are disabled
	if override, exists := m.config.Overrides[node.ID]; exists && override.DisableConnectivity {
		// Node connectivity alerts are disabled, clear any existing alert and return
		if _, alertExists := m.activeAlerts[alertID]; alertExists {
			delete(m.activeAlerts, alertID)
			log.Debug().
				Str("node", node.Name).
				Msg("Node offline alert cleared (connectivity alerts disabled)")
		}
		delete(m.nodeOfflineCount, node.ID)
		return
	}

	// Check if alert already exists
	if _, exists := m.activeAlerts[alertID]; exists {
		// Alert already exists, just update time
		m.activeAlerts[alertID].StartTime = time.Now()
		return
	}

	// Increment offline count
	m.nodeOfflineCount[node.ID]++
	offlineCount := m.nodeOfflineCount[node.ID]

	log.Debug().
		Str("node", node.Name).
		Str("instance", node.Instance).
		Int("offlineCount", offlineCount).
		Msg("Node offline detection count")

	// Require 3 consecutive offline polls (~15 seconds) before alerting
	// This prevents false positives from transient cluster communication issues
	const requiredOfflineCount = 3
	if offlineCount < requiredOfflineCount {
		log.Info().
			Str("node", node.Name).
			Int("count", offlineCount).
			Int("required", requiredOfflineCount).
			Msg("Node appears offline, waiting for confirmation")
		return
	}

	// Create new offline alert after confirmation
	alert := &Alert{
		ID:           alertID,
		Type:         "connectivity",
		Level:        AlertLevelCritical, // Node offline is always critical
		ResourceID:   node.ID,
		ResourceName: node.Name,
		Node:         node.Name,
		Instance:     node.Instance,
		Message:      fmt.Sprintf("Node '%s' is offline", node.Name),
		Value:        0, // Not applicable for offline status
		Threshold:    0, // Not applicable for offline status
		StartTime:    time.Now(),
		Acknowledged: false,
	}

	m.activeAlerts[alertID] = alert
	m.recentAlerts[alertID] = alert

	// Add to history
	m.historyManager.AddAlert(*alert)

	// Send notification after confirmation
	if m.onAlert != nil {
		m.onAlert(alert)
	}

	// Log the critical event
	log.Error().
		Str("node", node.Name).
		Str("instance", node.Instance).
		Str("status", node.Status).
		Str("connectionHealth", node.ConnectionHealth).
		Int("confirmedAfter", requiredOfflineCount).
		Msg("CRITICAL: Node is offline (confirmed)")
}

// clearNodeOfflineAlert removes offline alert when node comes back online
func (m *Manager) clearNodeOfflineAlert(node models.Node) {
	alertID := fmt.Sprintf("node-offline-%s", node.ID)

	m.mu.Lock()
	defer m.mu.Unlock()

	// Reset offline count when node comes back online
	if m.nodeOfflineCount[node.ID] > 0 {
		log.Debug().
			Str("node", node.Name).
			Int("previousCount", m.nodeOfflineCount[node.ID]).
			Msg("Node back online, resetting offline count")
		delete(m.nodeOfflineCount, node.ID)
	}

	// Check if offline alert exists
	alert, exists := m.activeAlerts[alertID]
	if !exists {
		return
	}

	// Remove from active alerts
	delete(m.activeAlerts, alertID)

	// Add to recently resolved
	resolvedAlert := &ResolvedAlert{
		ResolvedTime: time.Now(),
	}
	resolvedAlert.Alert = alert
	m.recentlyResolved[alertID] = resolvedAlert

	// Send recovery notification
	if m.onResolved != nil {
		m.onResolved(alertID)
	}

	// Log recovery
	log.Info().
		Str("node", node.Name).
		Str("instance", node.Instance).
		Dur("downtime", time.Since(alert.StartTime)).
		Msg("Node is back online")
}

// checkPBSOffline creates an alert for offline PBS instances
func (m *Manager) checkPBSOffline(pbs models.PBSInstance) {
	alertID := fmt.Sprintf("pbs-offline-%s", pbs.ID)

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if PBS offline alerts are disabled via disableConnectivity flag
	if override, exists := m.config.Overrides[pbs.ID]; exists && (override.Disabled || override.DisableConnectivity) {
		// PBS connectivity alerts are disabled, clear any existing alert and return
		if _, alertExists := m.activeAlerts[alertID]; alertExists {
			delete(m.activeAlerts, alertID)
			log.Debug().
				Str("pbs", pbs.Name).
				Msg("PBS offline alert cleared (connectivity alerts disabled)")
		}
		return
	}

	// Track confirmation count for this PBS
	m.offlineConfirmations[pbs.ID]++

	// Require 3 consecutive offline polls (~15 seconds) before alerting
	if m.offlineConfirmations[pbs.ID] < 3 {
		log.Debug().
			Str("pbs", pbs.Name).
			Int("confirmations", m.offlineConfirmations[pbs.ID]).
			Msg("PBS offline detected, waiting for confirmation")
		return
	}

	// Check if alert already exists
	if _, exists := m.activeAlerts[alertID]; exists {
		// Update last seen time
		m.activeAlerts[alertID].LastSeen = time.Now()
		return
	}

	// Create new offline alert after confirmation
	alert := &Alert{
		ID:           alertID,
		Type:         "offline",
		Level:        AlertLevelCritical,
		ResourceID:   pbs.ID,
		ResourceName: pbs.Name,
		Node:         pbs.Host,
		Instance:     pbs.Name,
		Message:      fmt.Sprintf("PBS instance %s is offline", pbs.Name),
		Value:        0,
		Threshold:    0,
		StartTime:    time.Now(),
		LastSeen:     time.Now(),
	}

	m.activeAlerts[alertID] = alert

	// Log and notify
	log.Error().
		Str("pbs", pbs.Name).
		Str("host", pbs.Host).
		Int("confirmations", m.offlineConfirmations[pbs.ID]).
		Msg("PBS instance is offline")

	if m.onAlert != nil {
		go m.onAlert(alert)
	}
}

// clearPBSOfflineAlert removes offline alert when PBS comes back online
func (m *Manager) clearPBSOfflineAlert(pbs models.PBSInstance) {
	alertID := fmt.Sprintf("pbs-offline-%s", pbs.ID)

	m.mu.Lock()
	defer m.mu.Unlock()

	// Reset offline confirmation count
	if count, exists := m.offlineConfirmations[pbs.ID]; exists && count > 0 {
		log.Debug().
			Str("pbs", pbs.Name).
			Int("previousCount", count).
			Msg("PBS is online, resetting offline confirmation count")
		delete(m.offlineConfirmations, pbs.ID)
	}

	// Check if offline alert exists
	alert, exists := m.activeAlerts[alertID]
	if !exists {
		return
	}

	// Remove from active alerts
	delete(m.activeAlerts, alertID)

	// Add to recently resolved
	resolvedAlert := &ResolvedAlert{
		ResolvedTime: time.Now(),
	}
	resolvedAlert.Alert = alert
	m.recentlyResolved[alertID] = resolvedAlert

	// Send recovery notification
	if m.onResolved != nil {
		m.onResolved(alertID)
	}

	// Log recovery
	log.Info().
		Str("pbs", pbs.Name).
		Str("host", pbs.Host).
		Dur("downtime", time.Since(alert.StartTime)).
		Msg("PBS instance is back online")
}

// checkStorageOffline creates an alert for offline/unavailable storage
func (m *Manager) checkStorageOffline(storage models.Storage) {
	alertID := fmt.Sprintf("storage-offline-%s", storage.ID)

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if storage offline alerts are disabled
	if override, exists := m.config.Overrides[storage.ID]; exists && override.Disabled {
		// Storage alerts are disabled, clear any existing alert and return
		if _, alertExists := m.activeAlerts[alertID]; alertExists {
			delete(m.activeAlerts, alertID)
			log.Debug().
				Str("storage", storage.Name).
				Msg("Storage offline alert cleared (alerts disabled)")
		}
		return
	}

	// Track confirmation count for this storage
	m.offlineConfirmations[storage.ID]++

	// Require 2 consecutive offline polls (~10 seconds) before alerting for storage
	// (less than nodes since storage status can be more transient)
	if m.offlineConfirmations[storage.ID] < 2 {
		log.Debug().
			Str("storage", storage.Name).
			Int("confirmations", m.offlineConfirmations[storage.ID]).
			Msg("Storage offline detected, waiting for confirmation")
		return
	}

	// Check if alert already exists
	if _, exists := m.activeAlerts[alertID]; exists {
		// Update last seen time
		m.activeAlerts[alertID].LastSeen = time.Now()
		return
	}

	// Create new offline alert after confirmation
	alert := &Alert{
		ID:           alertID,
		Type:         "offline",
		Level:        AlertLevelWarning, // Storage offline is Warning, not Critical
		ResourceID:   storage.ID,
		ResourceName: storage.Name,
		Node:         storage.Node,
		Instance:     storage.Instance,
		Message:      fmt.Sprintf("Storage %s on node %s is unavailable", storage.Name, storage.Node),
		Value:        0,
		Threshold:    0,
		StartTime:    time.Now(),
		LastSeen:     time.Now(),
	}

	m.activeAlerts[alertID] = alert

	// Log and notify
	log.Warn().
		Str("storage", storage.Name).
		Str("node", storage.Node).
		Int("confirmations", m.offlineConfirmations[storage.ID]).
		Msg("Storage is offline/unavailable")

	if m.onAlert != nil {
		go m.onAlert(alert)
	}
}

// clearStorageOfflineAlert removes offline alert when storage comes back online
func (m *Manager) clearStorageOfflineAlert(storage models.Storage) {
	alertID := fmt.Sprintf("storage-offline-%s", storage.ID)

	m.mu.Lock()
	defer m.mu.Unlock()

	// Reset offline confirmation count
	if count, exists := m.offlineConfirmations[storage.ID]; exists && count > 0 {
		log.Debug().
			Str("storage", storage.Name).
			Int("previousCount", count).
			Msg("Storage is online, resetting offline confirmation count")
		delete(m.offlineConfirmations, storage.ID)
	}

	// Check if offline alert exists
	alert, exists := m.activeAlerts[alertID]
	if !exists {
		return
	}

	// Remove from active alerts
	delete(m.activeAlerts, alertID)

	// Add to recently resolved
	resolvedAlert := &ResolvedAlert{
		ResolvedTime: time.Now(),
	}
	resolvedAlert.Alert = alert
	m.recentlyResolved[alertID] = resolvedAlert

	// Send recovery notification
	if m.onResolved != nil {
		m.onResolved(alertID)
	}

	// Log recovery
	log.Info().
		Str("storage", storage.Name).
		Str("node", storage.Node).
		Dur("downtime", time.Since(alert.StartTime)).
		Msg("Storage is back online")
}

// checkGuestPoweredOff creates an alert for powered-off guests
func (m *Manager) checkGuestPoweredOff(guestID, name, node, instanceName, guestType string) {
	alertID := fmt.Sprintf("guest-powered-off-%s", guestID)

	m.mu.Lock()
	defer m.mu.Unlock()

	// Get thresholds to check if powered-off alerts are disabled
	var thresholds ThresholdConfig
	if override, exists := m.config.Overrides[guestID]; exists {
		thresholds = override
	} else {
		thresholds = m.config.GuestDefaults
	}

	// Check if powered-off alerts are disabled for this guest
	if thresholds.Disabled || thresholds.DisableConnectivity {
		// Powered-off alerts are disabled, clear any existing alert and return
		if _, alertExists := m.activeAlerts[alertID]; alertExists {
			delete(m.activeAlerts, alertID)
			log.Debug().
				Str("guest", name).
				Msg("Guest powered-off alert cleared (alerts disabled)")
		}
		delete(m.offlineConfirmations, guestID)
		return
	}

	// Check if alert already exists
	if _, exists := m.activeAlerts[alertID]; exists {
		// Alert already exists, just update LastSeen
		m.activeAlerts[alertID].LastSeen = time.Now()
		return
	}

	// Increment confirmation count
	m.offlineConfirmations[guestID]++
	confirmCount := m.offlineConfirmations[guestID]

	log.Debug().
		Str("guest", name).
		Str("type", guestType).
		Int("confirmations", confirmCount).
		Msg("Guest powered-off detected")

	// Require 2 consecutive powered-off polls (~10 seconds) before alerting
	// This prevents false positives from transient states
	const requiredConfirmations = 2
	if confirmCount < requiredConfirmations {
		log.Debug().
			Str("guest", name).
			Int("count", confirmCount).
			Int("required", requiredConfirmations).
			Msg("Guest appears powered-off, waiting for confirmation")
		return
	}

	// Create new powered-off alert after confirmation
	alert := &Alert{
		ID:           alertID,
		Type:         "powered-off",
		Level:        AlertLevelWarning, // Powered-off is a warning, not critical
		ResourceID:   guestID,
		ResourceName: name,
		Node:         node,
		Instance:     instanceName,
		Message:      fmt.Sprintf("%s '%s' is powered off", guestType, name),
		Value:        0, // Not applicable for powered-off status
		Threshold:    0, // Not applicable for powered-off status
		StartTime:    time.Now(),
		LastSeen:     time.Now(),
		Acknowledged: false,
	}

	m.activeAlerts[alertID] = alert
	m.recentAlerts[alertID] = alert

	// Add to history
	m.historyManager.AddAlert(*alert)

	// Send notification after confirmation
	if m.onAlert != nil {
		m.onAlert(alert)
	}

	// Log the event
	log.Warn().
		Str("guest", name).
		Str("type", guestType).
		Str("node", node).
		Str("instance", instanceName).
		Int("confirmedAfter", requiredConfirmations).
		Msg("Guest is powered off (confirmed)")
}

// clearGuestPoweredOffAlert removes powered-off alert when guest starts running
func (m *Manager) clearGuestPoweredOffAlert(guestID, name string) {
	alertID := fmt.Sprintf("guest-powered-off-%s", guestID)

	m.mu.Lock()
	defer m.mu.Unlock()

	// Reset confirmation count when guest comes back online
	if count, exists := m.offlineConfirmations[guestID]; exists && count > 0 {
		log.Debug().
			Str("guest", name).
			Int("previousCount", count).
			Msg("Guest is running, resetting powered-off confirmation count")
		delete(m.offlineConfirmations, guestID)
	}

	// Check if powered-off alert exists
	alert, exists := m.activeAlerts[alertID]
	if !exists {
		return
	}

	// Remove from active alerts
	delete(m.activeAlerts, alertID)

	// Add to recently resolved (lock ordering: must not hold m.mu when acquiring resolvedMutex)
	downtime := time.Since(alert.StartTime)

	// Release m.mu before acquiring resolvedMutex
	m.mu.Unlock()
	m.resolvedMutex.Lock()
	resolvedAlert := &ResolvedAlert{
		Alert:        alert,
		ResolvedTime: time.Now(),
	}
	m.recentlyResolved[alertID] = resolvedAlert
	m.resolvedMutex.Unlock()
	m.mu.Lock() // Re-acquire for deferred unlock

	// Send recovery notification
	if m.onResolved != nil {
		m.onResolved(alertID)
	}

	// Log recovery
	log.Info().
		Str("guest", name).
		Dur("downtime", downtime).
		Msg("Guest is now running")
}

// ClearAlert removes an alert from active alerts (but keeps in history)
func (m *Manager) ClearAlert(alertID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Remove from active alerts only
	delete(m.activeAlerts, alertID)

	if m.onResolved != nil {
		go m.onResolved(alertID)
	}
}

// Cleanup removes old acknowledged alerts and cleans up tracking maps
func (m *Manager) Cleanup(maxAge time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	// Clean up acknowledged alerts
	for id, alert := range m.activeAlerts {
		if alert.Acknowledged && alert.AckTime != nil && now.Sub(*alert.AckTime) > maxAge {
			delete(m.activeAlerts, id)
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
}

// convertLegacyThreshold converts a legacy float64 threshold to HysteresisThreshold
func (m *Manager) convertLegacyThreshold(legacy *float64) *HysteresisThreshold {
	if legacy == nil || *legacy <= 0 {
		return nil
	}
	margin := m.config.HysteresisMargin
	if margin <= 0 {
		margin = 5.0 // Default 5% margin
	}
	return &HysteresisThreshold{
		Trigger: *legacy,
		Clear:   *legacy - margin,
	}
}

// ensureHysteresisThreshold ensures a threshold has hysteresis configured
func ensureHysteresisThreshold(threshold *HysteresisThreshold) *HysteresisThreshold {
	if threshold == nil {
		return nil
	}
	if threshold.Clear <= 0 {
		threshold.Clear = threshold.Trigger - 5.0 // Default 5% margin
	}
	return threshold
}

// evaluateFilterCondition evaluates a single filter condition against a guest
func (m *Manager) evaluateFilterCondition(guest interface{}, condition FilterCondition) bool {
	switch g := guest.(type) {
	case models.VM:
		return m.evaluateVMCondition(g, condition)
	case models.Container:
		return m.evaluateContainerCondition(g, condition)
	default:
		return false
	}
}

// evaluateVMCondition evaluates a filter condition against a VM
func (m *Manager) evaluateVMCondition(vm models.VM, condition FilterCondition) bool {
	switch condition.Type {
	case "metric":
		value := 0.0
		switch strings.ToLower(condition.Field) {
		case "cpu":
			value = vm.CPU * 100
		case "memory":
			value = vm.Memory.Usage
		case "disk":
			value = vm.Disk.Usage
		case "diskread":
			value = float64(vm.DiskRead) / 1024 / 1024 // Convert to MB/s
		case "diskwrite":
			value = float64(vm.DiskWrite) / 1024 / 1024
		case "networkin":
			value = float64(vm.NetworkIn) / 1024 / 1024
		case "networkout":
			value = float64(vm.NetworkOut) / 1024 / 1024
		default:
			return false
		}

		condValue, ok := condition.Value.(float64)
		if !ok {
			// Try to convert from int
			if intVal, ok := condition.Value.(int); ok {
				condValue = float64(intVal)
			} else {
				return false
			}
		}

		switch condition.Operator {
		case ">":
			return value > condValue
		case "<":
			return value < condValue
		case ">=":
			return value >= condValue
		case "<=":
			return value <= condValue
		case "=", "==":
			return value >= condValue-0.5 && value <= condValue+0.5
		}

	case "text":
		searchValue := strings.ToLower(fmt.Sprintf("%v", condition.Value))
		switch strings.ToLower(condition.Field) {
		case "name":
			return strings.Contains(strings.ToLower(vm.Name), searchValue)
		case "node":
			return strings.Contains(strings.ToLower(vm.Node), searchValue)
		case "vmid":
			return strings.Contains(vm.ID, searchValue)
		}

	case "raw":
		if condition.RawText != "" {
			term := strings.ToLower(condition.RawText)
			return strings.Contains(strings.ToLower(vm.Name), term) ||
				strings.Contains(vm.ID, term) ||
				strings.Contains(strings.ToLower(vm.Node), term) ||
				strings.Contains(strings.ToLower(vm.Status), term)
		}
	}

	return false
}

// evaluateContainerCondition evaluates a filter condition against a Container
func (m *Manager) evaluateContainerCondition(ct models.Container, condition FilterCondition) bool {
	// Similar logic to evaluateVMCondition but for Container type
	switch condition.Type {
	case "metric":
		value := 0.0
		switch strings.ToLower(condition.Field) {
		case "cpu":
			value = ct.CPU * 100
		case "memory":
			value = ct.Memory.Usage
		case "disk":
			value = ct.Disk.Usage
		case "diskread":
			value = float64(ct.DiskRead) / 1024 / 1024
		case "diskwrite":
			value = float64(ct.DiskWrite) / 1024 / 1024
		case "networkin":
			value = float64(ct.NetworkIn) / 1024 / 1024
		case "networkout":
			value = float64(ct.NetworkOut) / 1024 / 1024
		default:
			return false
		}

		condValue, ok := condition.Value.(float64)
		if !ok {
			if intVal, ok := condition.Value.(int); ok {
				condValue = float64(intVal)
			} else {
				return false
			}
		}

		switch condition.Operator {
		case ">":
			return value > condValue
		case "<":
			return value < condValue
		case ">=":
			return value >= condValue
		case "<=":
			return value <= condValue
		case "=", "==":
			return value >= condValue-0.5 && value <= condValue+0.5
		}

	case "text":
		searchValue := strings.ToLower(fmt.Sprintf("%v", condition.Value))
		switch strings.ToLower(condition.Field) {
		case "name":
			return strings.Contains(strings.ToLower(ct.Name), searchValue)
		case "node":
			return strings.Contains(strings.ToLower(ct.Node), searchValue)
		case "vmid":
			return strings.Contains(ct.ID, searchValue)
		}

	case "raw":
		if condition.RawText != "" {
			term := strings.ToLower(condition.RawText)
			return strings.Contains(strings.ToLower(ct.Name), term) ||
				strings.Contains(ct.ID, term) ||
				strings.Contains(strings.ToLower(ct.Node), term) ||
				strings.Contains(strings.ToLower(ct.Status), term)
		}
	}

	return false
}

// evaluateFilterStack evaluates a filter stack against a guest
func (m *Manager) evaluateFilterStack(guest interface{}, stack FilterStack) bool {
	if len(stack.Filters) == 0 {
		return true
	}

	results := make([]bool, len(stack.Filters))
	for i, filter := range stack.Filters {
		results[i] = m.evaluateFilterCondition(guest, filter)
	}

	// Apply logical operator
	if stack.LogicalOperator == "AND" {
		for _, result := range results {
			if !result {
				return false
			}
		}
		return true
	} else { // OR
		for _, result := range results {
			if result {
				return true
			}
		}
		return false
	}
}

// getGuestThresholds returns the appropriate thresholds for a guest
// Priority: Guest-specific overrides > Custom rules (by priority) > Global defaults
func (m *Manager) getGuestThresholds(guest interface{}, guestID string) ThresholdConfig {
	// Start with defaults
	thresholds := m.config.GuestDefaults

	// Check custom rules (sorted by priority, highest first)
	var applicableRule *CustomAlertRule
	highestPriority := -1

	for i := range m.config.CustomRules {
		rule := &m.config.CustomRules[i]
		if !rule.Enabled {
			continue
		}

		// Check if this rule applies to the guest
		if m.evaluateFilterStack(guest, rule.FilterConditions) {
			if rule.Priority > highestPriority {
				applicableRule = rule
				highestPriority = rule.Priority
			}
		}
	}

	// Apply custom rule thresholds if found
	if applicableRule != nil {
		if applicableRule.Thresholds.CPU != nil {
			thresholds.CPU = ensureHysteresisThreshold(applicableRule.Thresholds.CPU)
		} else if applicableRule.Thresholds.CPULegacy != nil {
			thresholds.CPU = m.convertLegacyThreshold(applicableRule.Thresholds.CPULegacy)
		}
		if applicableRule.Thresholds.Memory != nil {
			thresholds.Memory = ensureHysteresisThreshold(applicableRule.Thresholds.Memory)
		} else if applicableRule.Thresholds.MemoryLegacy != nil {
			thresholds.Memory = m.convertLegacyThreshold(applicableRule.Thresholds.MemoryLegacy)
		}
		if applicableRule.Thresholds.Disk != nil {
			thresholds.Disk = ensureHysteresisThreshold(applicableRule.Thresholds.Disk)
		} else if applicableRule.Thresholds.DiskLegacy != nil {
			thresholds.Disk = m.convertLegacyThreshold(applicableRule.Thresholds.DiskLegacy)
		}
		if applicableRule.Thresholds.DiskRead != nil {
			thresholds.DiskRead = ensureHysteresisThreshold(applicableRule.Thresholds.DiskRead)
		} else if applicableRule.Thresholds.DiskReadLegacy != nil {
			thresholds.DiskRead = m.convertLegacyThreshold(applicableRule.Thresholds.DiskReadLegacy)
		}
		if applicableRule.Thresholds.DiskWrite != nil {
			thresholds.DiskWrite = ensureHysteresisThreshold(applicableRule.Thresholds.DiskWrite)
		} else if applicableRule.Thresholds.DiskWriteLegacy != nil {
			thresholds.DiskWrite = m.convertLegacyThreshold(applicableRule.Thresholds.DiskWriteLegacy)
		}
		if applicableRule.Thresholds.NetworkIn != nil {
			thresholds.NetworkIn = ensureHysteresisThreshold(applicableRule.Thresholds.NetworkIn)
		} else if applicableRule.Thresholds.NetworkInLegacy != nil {
			thresholds.NetworkIn = m.convertLegacyThreshold(applicableRule.Thresholds.NetworkInLegacy)
		}
		if applicableRule.Thresholds.NetworkOut != nil {
			thresholds.NetworkOut = ensureHysteresisThreshold(applicableRule.Thresholds.NetworkOut)
		} else if applicableRule.Thresholds.NetworkOutLegacy != nil {
			thresholds.NetworkOut = m.convertLegacyThreshold(applicableRule.Thresholds.NetworkOutLegacy)
		}

		log.Debug().
			Str("guest", guestID).
			Str("rule", applicableRule.Name).
			Int("priority", applicableRule.Priority).
			Msg("Applied custom alert rule")
	}

	// Finally check guest-specific overrides (highest priority)
	if override, exists := m.config.Overrides[guestID]; exists {
		// Apply the disabled flag if set
		if override.Disabled {
			thresholds.Disabled = true
		}

		if override.CPU != nil {
			thresholds.CPU = ensureHysteresisThreshold(override.CPU)
		} else if override.CPULegacy != nil {
			thresholds.CPU = m.convertLegacyThreshold(override.CPULegacy)
		}
		if override.Memory != nil {
			thresholds.Memory = ensureHysteresisThreshold(override.Memory)
		} else if override.MemoryLegacy != nil {
			thresholds.Memory = m.convertLegacyThreshold(override.MemoryLegacy)
		}
		if override.Disk != nil {
			thresholds.Disk = ensureHysteresisThreshold(override.Disk)
		} else if override.DiskLegacy != nil {
			thresholds.Disk = m.convertLegacyThreshold(override.DiskLegacy)
		}
		if override.DiskRead != nil {
			thresholds.DiskRead = ensureHysteresisThreshold(override.DiskRead)
		} else if override.DiskReadLegacy != nil {
			thresholds.DiskRead = m.convertLegacyThreshold(override.DiskReadLegacy)
		}
		if override.DiskWrite != nil {
			thresholds.DiskWrite = ensureHysteresisThreshold(override.DiskWrite)
		} else if override.DiskWriteLegacy != nil {
			thresholds.DiskWrite = m.convertLegacyThreshold(override.DiskWriteLegacy)
		}
		if override.NetworkIn != nil {
			thresholds.NetworkIn = ensureHysteresisThreshold(override.NetworkIn)
		} else if override.NetworkInLegacy != nil {
			thresholds.NetworkIn = m.convertLegacyThreshold(override.NetworkInLegacy)
		}
		if override.NetworkOut != nil {
			thresholds.NetworkOut = ensureHysteresisThreshold(override.NetworkOut)
		} else if override.NetworkOutLegacy != nil {
			thresholds.NetworkOut = m.convertLegacyThreshold(override.NetworkOutLegacy)
		}
	}

	return thresholds
}

// checkRateLimit checks if an alert has exceeded rate limit
func (m *Manager) checkRateLimit(alertID string) bool {
	if m.config.Schedule.MaxAlertsHour <= 0 {
		return true // No rate limit
	}

	now := time.Now()
	cutoff := now.Add(-1 * time.Hour)

	// Clean old entries and count recent alerts
	var recentAlerts []time.Time
	if times, exists := m.alertRateLimit[alertID]; exists {
		for _, t := range times {
			if t.After(cutoff) {
				recentAlerts = append(recentAlerts, t)
			}
		}
	}

	// Check if we've hit the limit
	if len(recentAlerts) >= m.config.Schedule.MaxAlertsHour {
		return false
	}

	// Add current time
	recentAlerts = append(recentAlerts, now)
	m.alertRateLimit[alertID] = recentAlerts

	return true
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
					Int("level", i+1).
					Str("notify", level.Notify).
					Msg("Alert escalated")

				// Trigger escalation callback
				if m.onEscalate != nil {
					go m.onEscalate(alert, i+1)
				}
			}
		}
	}
}

// Stop stops the alert manager and saves history
func (m *Manager) Stop() {
	close(m.escalationStop)
	m.historyManager.Stop()
	// Save active alerts before stopping
	if err := m.SaveActiveAlerts(); err != nil {
		log.Error().Err(err).Msg("Failed to save active alerts on stop")
	}
}

// SaveActiveAlerts persists active alerts to disk
func (m *Manager) SaveActiveAlerts() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Create directory if it doesn't exist
	alertsDir := filepath.Join(utils.GetDataDir(), "alerts")
	if err := os.MkdirAll(alertsDir, 0755); err != nil {
		return fmt.Errorf("failed to create alerts directory: %w", err)
	}

	// Convert map to slice for JSON encoding
	alerts := make([]*Alert, 0, len(m.activeAlerts))
	for _, alert := range m.activeAlerts {
		alerts = append(alerts, alert)
	}

	data, err := json.MarshalIndent(alerts, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal active alerts: %w", err)
	}

	// Write to temporary file first, then rename (atomic operation)
	tmpFile := filepath.Join(alertsDir, "active-alerts.json.tmp")
	finalFile := filepath.Join(alertsDir, "active-alerts.json")

	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write active alerts: %w", err)
	}

	if err := os.Rename(tmpFile, finalFile); err != nil {
		return fmt.Errorf("failed to rename active alerts file: %w", err)
	}

	log.Info().Int("count", len(alerts)).Msg("Saved active alerts to disk")
	return nil
}

// LoadActiveAlerts restores active alerts from disk
func (m *Manager) LoadActiveAlerts() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	alertsFile := filepath.Join(utils.GetDataDir(), "alerts", "active-alerts.json")
	data, err := os.ReadFile(alertsFile)
	if err != nil {
		if os.IsNotExist(err) {
			log.Info().Msg("No active alerts file found, starting fresh")
			return nil
		}
		return fmt.Errorf("failed to read active alerts: %w", err)
	}

	var alerts []*Alert
	if err := json.Unmarshal(data, &alerts); err != nil {
		return fmt.Errorf("failed to unmarshal active alerts: %w", err)
	}

	// Restore alerts to the map with deduplication
	now := time.Now()
	restoredCount := 0
	duplicateCount := 0
	seen := make(map[string]bool)

	for _, alert := range alerts {
		// Skip duplicates
		if seen[alert.ID] {
			duplicateCount++
			log.Warn().Str("alertID", alert.ID).Msg("Skipping duplicate alert during restore")
			continue
		}
		seen[alert.ID] = true

		// Skip very old alerts (older than 24 hours)
		if now.Sub(alert.StartTime) > 24*time.Hour {
			log.Debug().Str("alertID", alert.ID).Msg("Skipping old alert during restore")
			continue
		}

		// Skip acknowledged alerts older than 1 hour
		if alert.Acknowledged && alert.AckTime != nil && now.Sub(*alert.AckTime) > time.Hour {
			log.Debug().Str("alertID", alert.ID).Msg("Skipping old acknowledged alert")
			continue
		}

		m.activeAlerts[alert.ID] = alert
		restoredCount++

		// For critical alerts that are still active after restart, send notifications
		// This ensures users are notified about ongoing critical issues even after service restarts
		// Only notify for alerts that started recently (within last 2 hours) to avoid spam
		if alert.Level == AlertLevelCritical && now.Sub(alert.StartTime) < 2*time.Hour {
			// Use a goroutine and add a small delay to avoid notification spam on startup
			if m.onAlert != nil {
				go func(a *Alert) {
					time.Sleep(10 * time.Second) // Wait for system to stabilize after restart
					log.Info().
						Str("alertID", a.ID).
						Str("resource", a.ResourceName).
						Msg("Sending notification for restored critical alert")
					m.onAlert(a)
				}(alert)
			}
		}
	}

	log.Info().
		Int("restored", restoredCount).
		Int("total", len(alerts)).
		Int("duplicates", duplicateCount).
		Msg("Restored active alerts from disk")
	return nil
}

// CleanupAlertsForNodes removes alerts for nodes that no longer exist
func (m *Manager) CleanupAlertsForNodes(existingNodes map[string]bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	log.Info().
		Int("totalAlerts", len(m.activeAlerts)).
		Int("existingNodes", len(existingNodes)).
		Interface("nodes", existingNodes).
		Msg("Starting alert cleanup for non-existent nodes")

	removedCount := 0
	for alertID, alert := range m.activeAlerts {
		// Use the Node field from the alert itself, which is more reliable
		node := alert.Node

		// If we couldn't get a node or the node doesn't exist, remove the alert
		if node == "" || !existingNodes[node] {
			delete(m.activeAlerts, alertID)
			removedCount++
			log.Debug().Str("alertID", alertID).Str("node", node).Msg("Removed alert for non-existent node")
		}
	}

	if removedCount > 0 {
		log.Info().Int("removed", removedCount).Int("remaining", len(m.activeAlerts)).Msg("Cleaned up alerts for non-existent nodes")
		// Save the cleaned up state
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Error().Interface("panic", r).Msg("Panic in SaveActiveAlerts goroutine (cleanup)")
				}
			}()
			if err := m.SaveActiveAlerts(); err != nil {
				log.Error().Err(err).Msg("Failed to save alerts after cleanup")
			}
		}()
	} else {
		log.Info().Msg("No alerts needed cleanup")
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
	m.pendingAlerts = make(map[string]time.Time)
	m.recentAlerts = make(map[string]*Alert)
	m.suppressedUntil = make(map[string]time.Time)
	m.alertRateLimit = make(map[string][]time.Time)
	m.nodeOfflineCount = make(map[string]int)
	m.offlineConfirmations = make(map[string]int)
	m.mu.Unlock()

	m.resolvedMutex.Lock()
	m.recentlyResolved = make(map[string]*ResolvedAlert)
	m.resolvedMutex.Unlock()

	log.Info().Msg("Cleared all active and pending alerts")

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error().Interface("panic", r).Msg("Panic in SaveActiveAlerts goroutine (clear)")
			}
		}()
		if err := m.SaveActiveAlerts(); err != nil {
			log.Error().Err(err).Msg("Failed to persist cleared alerts")
		}
	}()
}

// periodicSaveAlerts saves active alerts to disk periodically
func (m *Manager) periodicSaveAlerts() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := m.SaveActiveAlerts(); err != nil {
				log.Error().Err(err).Msg("Failed to save active alerts during periodic save")
			}
		case <-m.escalationStop:
			return
		}
	}
}

// CheckDiskHealth checks disk health and creates alerts if needed
func (m *Manager) CheckDiskHealth(instance, node string, disk proxmox.Disk) {
	// Create unique alert ID for this disk
	alertID := fmt.Sprintf("disk-health-%s-%s-%s", instance, node, disk.DevPath)

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if disk health is not PASSED
	if disk.Health != "PASSED" && disk.Health != "" {
		// Check if alert already exists
		if _, exists := m.activeAlerts[alertID]; !exists {
			// Create new health alert
			alert := &Alert{
				ID:           alertID,
				Type:         "disk-health",
				Level:        AlertLevelCritical,
				ResourceID:   fmt.Sprintf("%s-%s", node, disk.DevPath),
				ResourceName: fmt.Sprintf("%s (%s)", disk.Model, disk.DevPath),
				Node:         node,
				Instance:     instance,
				Message:      fmt.Sprintf("Disk health check failed: %s", disk.Health),
				Value:        0, // Not applicable for health status
				Threshold:    0,
				StartTime:    time.Now(),
				LastSeen:     time.Now(),
				Metadata: map[string]interface{}{
					"disk_path":   disk.DevPath,
					"disk_model":  disk.Model,
					"disk_serial": disk.Serial,
					"disk_type":   disk.Type,
					"disk_health": disk.Health,
					"disk_size":   disk.Size,
				},
			}

			m.activeAlerts[alertID] = alert
			m.recentAlerts[alertID] = alert
			m.historyManager.AddAlert(*alert)

			if m.onAlert != nil {
				m.onAlert(alert)
			}

			log.Error().
				Str("node", node).
				Str("disk", disk.DevPath).
				Str("model", disk.Model).
				Str("health", disk.Health).
				Msg("Disk health alert created")
		}
	} else {
		// Disk is healthy, clear alert if it exists
		m.clearAlertNoLock(alertID)
	}

	// Check for low wearout (SSD life remaining)
	if disk.Wearout > 0 && disk.Wearout < 10 {
		wearoutAlertID := fmt.Sprintf("disk-wearout-%s-%s-%s", instance, node, disk.DevPath)

		if _, exists := m.activeAlerts[wearoutAlertID]; !exists {
			// Create wearout alert
			alert := &Alert{
				ID:           wearoutAlertID,
				Type:         "disk-wearout",
				Level:        AlertLevelWarning,
				ResourceID:   fmt.Sprintf("%s-%s", node, disk.DevPath),
				ResourceName: fmt.Sprintf("%s (%s)", disk.Model, disk.DevPath),
				Node:         node,
				Instance:     instance,
				Message:      fmt.Sprintf("SSD has less than 10%% life remaining (%d%% wearout)", disk.Wearout),
				Value:        float64(disk.Wearout),
				Threshold:    10.0,
				StartTime:    time.Now(),
				LastSeen:     time.Now(),
				Metadata: map[string]interface{}{
					"disk_path":    disk.DevPath,
					"disk_model":   disk.Model,
					"disk_serial":  disk.Serial,
					"disk_type":    disk.Type,
					"disk_wearout": disk.Wearout,
				},
			}

			m.activeAlerts[wearoutAlertID] = alert
			m.recentAlerts[wearoutAlertID] = alert
			m.historyManager.AddAlert(*alert)

			if m.onAlert != nil {
				m.onAlert(alert)
			}

			log.Warn().
				Str("node", node).
				Str("disk", disk.DevPath).
				Str("model", disk.Model).
				Int("wearout", disk.Wearout).
				Msg("Disk wearout alert created")
		}
	} else if disk.Wearout >= 10 {
		// Wearout is acceptable, clear alert if it exists
		wearoutAlertID := fmt.Sprintf("disk-wearout-%s-%s-%s", instance, node, disk.DevPath)
		m.clearAlertNoLock(wearoutAlertID)
	}
}

// clearAlertNoLock clears an alert without locking (must be called with lock held)
func (m *Manager) clearAlertNoLock(alertID string) {
	if alert, exists := m.activeAlerts[alertID]; exists {
		delete(m.activeAlerts, alertID)

		// Add to recently resolved
		resolvedAlert := &ResolvedAlert{
			Alert:        alert,
			ResolvedTime: time.Now(),
		}
		m.recentlyResolved[alertID] = resolvedAlert

		// Send recovery notification
		if m.onResolved != nil {
			m.onResolved(alertID)
		}

		log.Info().
			Str("alertID", alertID).
			Msg("Alert cleared")
	}
}
