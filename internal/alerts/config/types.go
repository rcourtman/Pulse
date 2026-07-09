package config

import (
	"encoding/json"
	"strings"
	"time"
)

// AlertLevel represents the severity of an alert
type AlertLevel string

const (
	AlertLevelWarning  AlertLevel = "warning"
	AlertLevelCritical AlertLevel = "critical"
)

// ActivationState represents the alert notification activation state
type ActivationState string

const (
	ActivationPending ActivationState = "pending_review"
	ActivationActive  ActivationState = "active"
	ActivationSnoozed ActivationState = "snoozed"
)

func NormalizePoweredOffSeverity(level AlertLevel) AlertLevel {
	switch strings.ToLower(string(level)) {
	case string(AlertLevelCritical):
		return AlertLevelCritical
	default:
		return AlertLevelWarning
	}
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
	PoweredOffSeverity  AlertLevel           `json:"poweredOffSeverity,omitempty"`  // Severity for powered-off alerts
	CPU                 *HysteresisThreshold `json:"cpu,omitempty"`
	Memory              *HysteresisThreshold `json:"memory,omitempty"`
	Disk                *HysteresisThreshold `json:"disk,omitempty"`
	DiskRead            *HysteresisThreshold `json:"diskRead,omitempty"`
	DiskWrite           *HysteresisThreshold `json:"diskWrite,omitempty"`
	NetworkIn           *HysteresisThreshold `json:"networkIn,omitempty"`
	NetworkOut          *HysteresisThreshold `json:"networkOut,omitempty"`
	Usage               *HysteresisThreshold `json:"usage,omitempty"`           // For storage devices
	Temperature         *HysteresisThreshold `json:"temperature,omitempty"`     // For node CPU temperature
	DiskTemperature     *HysteresisThreshold `json:"diskTemperature,omitempty"` // For host SMART temperatures
	Backup              *BackupAlertConfig   `json:"backup,omitempty"`
	Snapshot            *SnapshotAlertConfig `json:"snapshot,omitempty"`
	Note                *string              `json:"note,omitempty"`
}

// QuietHours represents quiet hours configuration
type QuietHours struct {
	Enabled  bool                  `json:"enabled"`
	Start    string                `json:"start"` // 24-hour format "HH:MM"
	End      string                `json:"end"`   // 24-hour format "HH:MM"
	Timezone string                `json:"timezone"`
	Days     map[string]bool       `json:"days"` // monday, tuesday, etc.
	Suppress QuietHoursSuppression `json:"suppress"`
}

// QuietHoursSuppression controls which alert categories are silenced during quiet hours.
type QuietHoursSuppression struct {
	Performance bool `json:"performance"`
	Storage     bool `json:"storage"`
	Offline     bool `json:"offline"`
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
	QuietHours      QuietHours       `json:"quietHours"`
	Cooldown        int              `json:"cooldown"`        // minutes
	MaxAlertsHour   int              `json:"maxAlertsHour"`   // max alerts per hour per resource
	NotifyOnResolve bool             `json:"notifyOnResolve"` // Send notification when alert clears
	Escalation      EscalationConfig `json:"escalation"`
	Grouping        GroupingConfig   `json:"grouping"`
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

// DockerThresholdConfig represents Docker-specific alert thresholds
type DockerThresholdConfig struct {
	CPU                      HysteresisThreshold `json:"cpu"`                                // CPU usage % threshold (default: 80%)
	Memory                   HysteresisThreshold `json:"memory"`                             // Memory usage % threshold (default: 85%)
	Disk                     HysteresisThreshold `json:"disk"`                               // Writable layer usage % threshold (default: 85%)
	RestartCount             int                 `json:"restartCount"`                       // Number of restarts to trigger alert (default: 3)
	RestartWindow            int                 `json:"restartWindow"`                      // Time window in seconds for restart loop detection (default: 300 = 5min)
	MemoryWarnPct            int                 `json:"memoryWarnPct"`                      // Memory limit % to trigger warning (default: 90)
	MemoryCriticalPct        int                 `json:"memoryCriticalPct"`                  // Memory limit % to trigger critical (default: 95)
	ServiceWarnGapPct        int                 `json:"serviceWarnGapPercent"`              // % of desired tasks missing to trigger warning (default: 10)
	ServiceCritGapPct        int                 `json:"serviceCriticalGapPercent"`          // % of desired tasks missing to trigger critical (default: 50)
	StateDisableConnectivity bool                `json:"stateDisableConnectivity,omitempty"` // Disable container offline/state alerts globally
	StatePoweredOffSeverity  AlertLevel          `json:"statePoweredOffSeverity,omitempty"`  // Default severity for container state/offline alerts
	UpdateAlertDelayHours    int                 `json:"updateAlertDelayHours,omitempty"`    // Hours to wait before alerting on available image updates (default: 24, -1 = disabled)
}

// PMGThresholdConfig represents Proxmox Mail Gateway-specific alert thresholds
type PMGThresholdConfig struct {
	QueueTotalWarning       int `json:"queueTotalWarning"`       // Total queue depth warning threshold (default: 500)
	QueueTotalCritical      int `json:"queueTotalCritical"`      // Total queue depth critical threshold (default: 1000)
	OldestMessageWarnMins   int `json:"oldestMessageWarnMins"`   // Oldest queued message age warning in minutes (default: 30)
	OldestMessageCritMins   int `json:"oldestMessageCritMins"`   // Oldest queued message age critical in minutes (default: 60)
	DeferredQueueWarn       int `json:"deferredQueueWarn"`       // Deferred queue depth warning (default: 200)
	DeferredQueueCritical   int `json:"deferredQueueCritical"`   // Deferred queue depth critical (default: 500)
	HoldQueueWarn           int `json:"holdQueueWarn"`           // Hold queue depth warning (default: 100)
	HoldQueueCritical       int `json:"holdQueueCritical"`       // Hold queue depth critical (default: 300)
	QuarantineSpamWarn      int `json:"quarantineSpamWarn"`      // Spam quarantine absolute warning (default: 2000)
	QuarantineSpamCritical  int `json:"quarantineSpamCritical"`  // Spam quarantine absolute critical (default: 5000)
	QuarantineVirusWarn     int `json:"quarantineVirusWarn"`     // Virus quarantine absolute warning (default: 2000)
	QuarantineVirusCritical int `json:"quarantineVirusCritical"` // Virus quarantine absolute critical (default: 5000)
	QuarantineGrowthWarnPct int `json:"quarantineGrowthWarnPct"` // Growth % to trigger warning (default: 25)
	QuarantineGrowthWarnMin int `json:"quarantineGrowthWarnMin"` // Minimum message growth for warning (default: 250)
	QuarantineGrowthCritPct int `json:"quarantineGrowthCritPct"` // Growth % to trigger critical (default: 50)
	QuarantineGrowthCritMin int `json:"quarantineGrowthCritMin"` // Minimum message growth for critical (default: 500)
}

// SnapshotAlertConfig represents snapshot age alert configuration
type SnapshotAlertConfig struct {
	Enabled         bool    `json:"enabled"`
	WarningDays     int     `json:"warningDays"`
	CriticalDays    int     `json:"criticalDays"`
	WarningSizeGiB  float64 `json:"warningSizeGiB,omitempty"`
	CriticalSizeGiB float64 `json:"criticalSizeGiB,omitempty"`
}

// BackupAlertConfig represents backup age alert configuration
type BackupAlertConfig struct {
	Enabled      bool `json:"enabled"`
	WarningDays  int  `json:"warningDays"`
	CriticalDays int  `json:"criticalDays"`
	// Indicator thresholds for the dashboard (separate from alert thresholds)
	FreshHours int `json:"freshHours"` // Backups newer than this show as green (default: 24)
	StaleHours int `json:"staleHours"` // Backups older than FreshHours but newer than this show as amber (default: 72)
	// Global backup alert filters
	AlertOrphaned *bool    `json:"alertOrphaned,omitempty"` // Alert on backups that do not match a known guest (default: true)
	IgnoreVMIDs   []string `json:"ignoreVMIDs,omitempty"`   // Skip alerts for matching VMIDs (supports prefix*)
}

// GuestLookup describes a guest identity used for snapshot/backup evaluations.
type GuestLookup struct {
	ResourceID string
	Name       string
	Instance   string
	Node       string
	Type       string
	VMID       int
	Tags       []string
}

// AlertConfig represents the complete alert configuration
type AlertConfig struct {
	Enabled                        bool                           `json:"enabled"`
	ActivationState                ActivationState                `json:"activationState,omitempty"`
	ObservationWindowHours         int                            `json:"observationWindowHours,omitempty"`
	ActivationTime                 *time.Time                     `json:"activationTime,omitempty"`
	GuestDefaults                  ThresholdConfig                `json:"guestDefaults"`
	NodeDefaults                   ThresholdConfig                `json:"nodeDefaults"`
	AgentDefaults                  ThresholdConfig                `json:"agentDefaults"`
	StorageDefault                 HysteresisThreshold            `json:"storageDefault"`
	DiskFillByType                 map[string]HysteresisThreshold `json:"diskFillByType,omitempty"`
	DiskTempByType                 map[string]HysteresisThreshold `json:"diskTempByType,omitempty"`
	DockerDefaults                 DockerThresholdConfig          `json:"dockerDefaults"`
	DockerIgnoredContainerPrefixes []string                       `json:"dockerIgnoredContainerPrefixes,omitempty"`
	IgnoredGuestPrefixes           []string                       `json:"ignoredGuestPrefixes,omitempty"`
	GuestTagWhitelist              []string                       `json:"guestTagWhitelist,omitempty"`
	GuestTagBlacklist              []string                       `json:"guestTagBlacklist,omitempty"`
	PMGDefaults                    PMGThresholdConfig             `json:"pmgDefaults"`
	PBSDefaults                    ThresholdConfig                `json:"pbsDefaults"`
	KubernetesDefaults             ThresholdConfig                `json:"kubernetesDefaults"`
	TrueNASDefaults                ThresholdConfig                `json:"truenasDefaults"`
	TrueNASDiskDefaults            ThresholdConfig                `json:"truenasDiskDefaults"`
	VMwareDefaults                 ThresholdConfig                `json:"vmwareDefaults"`
	SnapshotDefaults               SnapshotAlertConfig            `json:"snapshotDefaults"`
	BackupDefaults                 BackupAlertConfig              `json:"backupDefaults"`
	Overrides                      map[string]ThresholdConfig     `json:"overrides"` // keyed by resource ID
	CustomRules                    []CustomAlertRule              `json:"customRules,omitempty"`
	Schedule                       ScheduleConfig                 `json:"schedule"`
	DisableAllNodes                bool                           `json:"disableAllNodes"`              // Disable all alerts for Proxmox nodes
	DisableAllGuests               bool                           `json:"disableAllGuests"`             // Disable all alerts for VMs/containers
	DisableAllAgents               bool                           `json:"disableAllAgents"`             // Disable all alerts for Pulse agents
	DisableAllStorage              bool                           `json:"disableAllStorage"`            // Disable all alerts for storage
	DisableAllPBS                  bool                           `json:"disableAllPBS"`                // Disable all alerts for PBS servers
	DisableAllPMG                  bool                           `json:"disableAllPMG"`                // Disable all alerts for PMG instances
	DisableAllDockerHosts          bool                           `json:"disableAllDockerHosts"`        // Disable all alerts for Docker hosts
	DisableAllDockerContainers     bool                           `json:"disableAllDockerContainers"`   // Disable all alerts for Docker containers
	DisableAllDockerServices       bool                           `json:"disableAllDockerServices"`     // Disable all alerts for Docker services
	DisableAllKubernetes           bool                           `json:"disableAllKubernetes"`         // Disable all alerts for Kubernetes resources
	DisableAllTrueNAS              bool                           `json:"disableAllTrueNAS"`            // Disable all alerts for TrueNAS resources
	DisableAllVMware               bool                           `json:"disableAllVMware"`             // Disable all alerts for VMware vSphere resources
	DisableAllNodesOffline         bool                           `json:"disableAllNodesOffline"`       // Disable node offline/connectivity alerts globally
	DisableAllGuestsOffline        bool                           `json:"disableAllGuestsOffline"`      // Disable guest powered-off alerts globally
	DisableAllAgentsOffline        bool                           `json:"disableAllAgentsOffline"`      // Disable agent offline alerts globally
	DisableAllPBSOffline           bool                           `json:"disableAllPBSOffline"`         // Disable PBS offline alerts globally
	DisableAllPMGOffline           bool                           `json:"disableAllPMGOffline"`         // Disable PMG offline alerts globally
	DisableAllDockerHostsOffline   bool                           `json:"disableAllDockerHostsOffline"` // Disable Docker host offline alerts globally
	MinimumDelta                   float64                        `json:"minimumDelta"`                 // Minimum % change to trigger new alert
	SuppressionWindow              int                            `json:"suppressionWindow"`            // Minutes to suppress duplicate alerts
	HysteresisMargin               float64                        `json:"hysteresisMargin"`             // Default margin for legacy thresholds
	TimeThresholds                 map[string]int                 `json:"timeThresholds"`               // Per-type delays: guest, node, agent, storage, pbs
	MetricTimeThresholds           map[string]map[string]int      `json:"metricTimeThresholds"`         // Optional per-metric delays keyed by resource type
	MaxAlertAgeDays                int                            `json:"maxAlertAgeDays"`              // Maximum age for alerts before auto-cleanup (0 = disabled)
	MaxAcknowledgedAgeDays         int                            `json:"maxAcknowledgedAgeDays"`       // Maximum age for acknowledged alerts (0 = disabled)
	AutoAcknowledgeAfterHours      int                            `json:"autoAcknowledgeAfterHours"`    // Auto-acknowledge alerts after X hours (0 = disabled)
	FlappingEnabled                bool                           `json:"flappingEnabled"`              // Enable flapping detection
	FlappingWindowSeconds          int                            `json:"flappingWindowSeconds"`        // Time window for counting state changes
	FlappingThreshold              int                            `json:"flappingThreshold"`            // Number of state changes to trigger flapping
	FlappingCooldownMinutes        int                            `json:"flappingCooldownMinutes"`      // Cooldown period after flapping detected
}

// UnmarshalJSON accepts canonical v6 alert config keys.
func (c *AlertConfig) UnmarshalJSON(data []byte) error {
	type alias AlertConfig
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*c = AlertConfig(decoded)

	raw := make(map[string]json.RawMessage)
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	NormalizeAlertConfigAliases(c)
	return nil
}

// NormalizeAlertConfigAliases strips deprecated legacy alias keys.
func NormalizeAlertConfigAliases(config *AlertConfig) {
	if config == nil {
		return
	}

	if config.TimeThresholds != nil {
		for key := range config.TimeThresholds {
			typeKey := CanonicalAlertResourceType(key)
			if typeKey == "" || typeKey == "all" {
				continue
			}
			if isUnsupportedLegacyAlertResourceType(typeKey) {
				delete(config.TimeThresholds, key)
			}
		}
	}

	if len(config.MetricTimeThresholds) == 0 {
		return
	}

	for key := range config.MetricTimeThresholds {
		typeKey := CanonicalAlertResourceType(key)
		if typeKey == "" || typeKey == "all" {
			continue
		}
		if isUnsupportedLegacyAlertResourceType(typeKey) {
			delete(config.MetricTimeThresholds, key)
		}
	}
}
