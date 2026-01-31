package ai

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// SignalType identifies the kind of infrastructure problem signal detected.
type SignalType string

// Default signal detection thresholds for deterministic problem identification.
// These are used as fallbacks when no user-configured thresholds are provided.
const (
	signalStorageWarningPercent  = 75.0           // Pool usage above this triggers a warning signal
	signalStorageCriticalPercent = 95.0           // Pool usage above this triggers a critical signal
	signalHighCPUPercent         = 70.0           // Average CPU above this triggers a high-CPU signal
	signalHighMemoryPercent      = 80.0           // Average memory above this triggers a high-memory signal
	signalBackupStaleThreshold   = 48 * time.Hour // No backup in this window triggers a stale-backup signal
)

// SignalThresholds holds configurable thresholds for deterministic signal detection.
// When populated from user config (PatrolThresholds), signals align with user-defined
// alert thresholds instead of using hardcoded defaults.
type SignalThresholds struct {
	StorageWarningPercent  float64
	StorageCriticalPercent float64
	HighCPUPercent         float64
	HighMemoryPercent      float64
	BackupStaleThreshold   time.Duration
}

// DefaultSignalThresholds returns the hardcoded default thresholds.
func DefaultSignalThresholds() SignalThresholds {
	return SignalThresholds{
		StorageWarningPercent:  signalStorageWarningPercent,
		StorageCriticalPercent: signalStorageCriticalPercent,
		HighCPUPercent:         signalHighCPUPercent,
		HighMemoryPercent:      signalHighMemoryPercent,
		BackupStaleThreshold:   signalBackupStaleThreshold,
	}
}

// SignalThresholdsFromPatrol builds SignalThresholds from user-configured PatrolThresholds.
// Zero-value fields fall back to defaults.
func SignalThresholdsFromPatrol(pt PatrolThresholds) SignalThresholds {
	defaults := DefaultSignalThresholds()
	st := SignalThresholds{
		StorageWarningPercent:  pt.StorageWarning,
		StorageCriticalPercent: pt.StorageCritical,
		HighCPUPercent:         pt.NodeCPUWarning,
		HighMemoryPercent:      pt.NodeMemWarning,
		BackupStaleThreshold:   defaults.BackupStaleThreshold, // No user config for this
	}
	// Fall back to defaults for zero values
	if st.StorageWarningPercent == 0 {
		st.StorageWarningPercent = defaults.StorageWarningPercent
	}
	if st.StorageCriticalPercent == 0 {
		st.StorageCriticalPercent = defaults.StorageCriticalPercent
	}
	if st.HighCPUPercent == 0 {
		st.HighCPUPercent = defaults.HighCPUPercent
	}
	if st.HighMemoryPercent == 0 {
		st.HighMemoryPercent = defaults.HighMemoryPercent
	}
	return st
}

const (
	SignalSMARTFailure SignalType = "smart_failure"
	SignalHighCPU      SignalType = "high_cpu"
	SignalHighMemory   SignalType = "high_memory"
	SignalHighDisk     SignalType = "high_disk"
	SignalBackupFailed SignalType = "backup_failed"
	SignalBackupStale  SignalType = "backup_stale"
	SignalActiveAlert  SignalType = "active_alert"
)

// DetectedSignal represents a problem signal found in tool call results.
type DetectedSignal struct {
	SignalType        SignalType
	ResourceID        string
	ResourceName      string
	ResourceType      string
	SuggestedSeverity string // "critical", "warning", "watch"
	Category          string // matches FindingCategory values
	Summary           string // one-line description
	Evidence          string // truncated raw data (max 500 chars)
	ToolCallID        string
}

// DetectSignals scans completed tool calls for known problem signals.
// This is deterministic — it does not depend on LLM judgment.
// Thresholds control when signals fire; use DefaultSignalThresholds() for hardcoded defaults
// or SignalThresholdsFromPatrol() to derive from user-configured alert thresholds.
func DetectSignals(toolCalls []ToolCallRecord, thresholds SignalThresholds) []DetectedSignal {
	var signals []DetectedSignal

	for i := range toolCalls {
		tc := &toolCalls[i]
		if !tc.Success || tc.Output == "" {
			continue
		}

		detected := detectSignalsFromToolCall(tc, thresholds)
		signals = append(signals, detected...)
	}

	return deduplicateSignals(signals)
}

// detectSignalsFromToolCall dispatches to signal-specific detectors based on tool name and input.
func detectSignalsFromToolCall(tc *ToolCallRecord, thresholds SignalThresholds) []DetectedSignal {
	var signals []DetectedSignal

	if shouldSkipSignalParsing(tc.Output) {
		return nil
	}

	switch tc.ToolName {
	case "pulse_storage":
		signals = append(signals, detectStorageSignals(tc, thresholds)...)
	case "pulse_metrics":
		signals = append(signals, detectMetricsSignals(tc, thresholds)...)
	case "pulse_alerts":
		signals = append(signals, detectAlertSignals(tc)...)
	}

	return signals
}

// --- Storage signals ---

// storageDiskHealth is the minimal struct for parsing disk_health tool output.
type storageDiskHealth struct {
	Disks []struct {
		Device string `json:"device"`
		Health string `json:"health"`
		Model  string `json:"model"`
		Serial string `json:"serial"`
		Node   string `json:"node"`
	} `json:"disks"`
	Node string `json:"node"`
}

// storagePool is the minimal struct for parsing storage pool data.
type storagePool struct {
	Pools []struct {
		Name         string  `json:"name"`
		UsagePercent float64 `json:"usage_percent"`
		Node         string  `json:"node"`
		ID           string  `json:"id"`
	} `json:"pools"`
	Storage string `json:"storage"`
	Node    string `json:"node"`
}

// VMIDValue handles VMID fields that may be strings or numbers.
type VMIDValue string

func (v *VMIDValue) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		*v = ""
		return nil
	}

	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*v = VMIDValue(strings.TrimSpace(s))
		return nil
	}

	var n json.Number
	if err := json.Unmarshal(data, &n); err == nil {
		*v = VMIDValue(n.String())
		return nil
	}

	return fmt.Errorf("invalid VMID value: %s", string(data))
}

// backupTask is the minimal struct for parsing backup task data.
type backupTask struct {
	Tasks []struct {
		ID        string    `json:"id"`
		Status    string    `json:"status"`
		Error     string    `json:"error"`
		StartTime string    `json:"start_time"`
		EndTime   string    `json:"end_time"`
		VMID      VMIDValue `json:"vmid"`
		VMName    string    `json:"vm_name"`
		Node      string    `json:"node"`
		Instance  string    `json:"instance"`
		Type      string    `json:"type"`
	} `json:"tasks"`
	Node string `json:"node"`
}

// backupsResponse is the minimal struct for parsing pulse_storage type="backups" output.
type backupsResponse struct {
	PBS []struct {
		VMID       string `json:"vmid"`
		BackupTime string `json:"backup_time"`
		Instance   string `json:"instance"`
	} `json:"pbs"`
	PVE []struct {
		VMID       int    `json:"vmid"`
		BackupTime string `json:"backup_time"`
	} `json:"pve"`
	RecentTasks []struct {
		VMID      int    `json:"vmid"`
		Node      string `json:"node"`
		Status    string `json:"status"`
		StartTime string `json:"start_time"`
	} `json:"recent_tasks"`
}

func detectStorageSignals(tc *ToolCallRecord, thresholds SignalThresholds) []DetectedSignal {
	var signals []DetectedSignal

	inputType := extractInputType(tc.Input)

	switch inputType {
	case "disk_health":
		signals = append(signals, detectSMARTFailures(tc)...)
	case "pools":
		signals = append(signals, detectHighDiskUsage(tc, thresholds)...)
	case "backup_tasks":
		signals = append(signals, detectBackupIssues(tc, thresholds)...)
	case "backups":
		signals = append(signals, detectBackupIssuesFromBackups(tc, thresholds)...)
	case "":
		// Input type missing — attempt to infer from output structure.
		signals = append(signals, detectSMARTFailures(tc)...)
		signals = append(signals, detectHighDiskUsage(tc, thresholds)...)
		signals = append(signals, detectBackupIssues(tc, thresholds)...)
		signals = append(signals, detectBackupIssuesFromBackups(tc, thresholds)...)
	}

	return signals
}

func detectSMARTFailures(tc *ToolCallRecord) []DetectedSignal {
	var signals []DetectedSignal

	var data storageDiskHealth
	if err := json.Unmarshal([]byte(tc.Output), &data); err != nil {
		if !tryParseEmbeddedJSON(tc.Output, &data) {
			log.Debug().Err(err).Str("tool", tc.ToolName).Msg("patrol_signals: failed to parse disk_health output")
			return nil
		}
	}

	node := data.Node
	for _, disk := range data.Disks {
		health := strings.ToUpper(strings.TrimSpace(disk.Health))
		if health == "" || health == "PASSED" || health == "OK" {
			continue
		}

		diskNode := disk.Node
		if diskNode == "" {
			diskNode = node
		}

		resourceID := diskNode
		if resourceID == "" {
			resourceID = disk.Device
		}

		signals = append(signals, DetectedSignal{
			SignalType:        SignalSMARTFailure,
			ResourceID:        resourceID,
			ResourceName:      disk.Device,
			ResourceType:      "node",
			SuggestedSeverity: "critical",
			Category:          string(FindingCategoryReliability),
			Summary:           "SMART health check: " + health + " for " + disk.Device,
			Evidence:          truncateEvidence(tc.Output),
			ToolCallID:        tc.ID,
		})
	}

	return signals
}

func detectHighDiskUsage(tc *ToolCallRecord, thresholds SignalThresholds) []DetectedSignal {
	var signals []DetectedSignal

	var data storagePool
	if err := json.Unmarshal([]byte(tc.Output), &data); err != nil {
		if !tryParseEmbeddedJSON(tc.Output, &data) {
			log.Debug().Err(err).Str("tool", tc.ToolName).Msg("patrol_signals: failed to parse pools output")
			return nil
		}
	}

	for _, pool := range data.Pools {
		if pool.UsagePercent <= thresholds.StorageWarningPercent {
			continue
		}

		severity := "warning"
		if pool.UsagePercent > thresholds.StorageCriticalPercent {
			severity = "critical"
		}

		resourceID := pool.ID
		if resourceID == "" {
			resourceID = pool.Name
		}

		signals = append(signals, DetectedSignal{
			SignalType:        SignalHighDisk,
			ResourceID:        resourceID,
			ResourceName:      pool.Name,
			ResourceType:      "storage",
			SuggestedSeverity: severity,
			Category:          string(FindingCategoryCapacity),
			Summary:           fmt.Sprintf("Storage pool %s at %.1f%% usage", pool.Name, pool.UsagePercent),
			Evidence:          truncateEvidence(tc.Output),
			ToolCallID:        tc.ID,
		})
	}

	return signals
}

func detectBackupIssues(tc *ToolCallRecord, thresholds SignalThresholds) []DetectedSignal {
	var signals []DetectedSignal

	var data backupTask
	if err := json.Unmarshal([]byte(tc.Output), &data); err != nil {
		if !tryParseEmbeddedJSON(tc.Output, &data) {
			log.Debug().Err(err).Str("tool", tc.ToolName).Msg("patrol_signals: failed to parse backup_tasks output")
			return nil
		}
	}

	now := time.Now()
	type latestBackup struct {
		time          time.Time
		isFailure     bool
		statusSummary string
		resourceID    string
		resourceName  string
	}
	latestByResource := make(map[string]latestBackup)

	for _, task := range data.Tasks {
		statusLower := strings.ToLower(strings.TrimSpace(task.Status))
		errorLower := strings.ToLower(strings.TrimSpace(task.Error))
		vmid := strings.TrimSpace(string(task.VMID))
		if vmid == "0" {
			vmid = ""
		}
		var taskEnd time.Time
		if task.EndTime != "" {
			if parsed, err := tryParseTime(task.EndTime); err == nil {
				taskEnd = parsed
			}
		}
		if taskEnd.IsZero() && task.StartTime != "" {
			if parsed, err := tryParseTime(task.StartTime); err == nil {
				taskEnd = parsed
			}
		}
		if taskEnd.IsZero() {
			taskEnd = now
		}
		if now.Sub(taskEnd) > thresholds.BackupStaleThreshold {
			continue
		}

		// Check for failed backups. Treat any non-OK terminal status as failure.
		isSuccess := statusLower == "ok" || strings.Contains(statusLower, "success")
		isFailure := statusLower != "" && !isSuccess
		if strings.Contains(statusLower, "error") || strings.Contains(statusLower, "fail") {
			isFailure = true
		}
		if strings.Contains(statusLower, "running") || strings.Contains(statusLower, "active") {
			isFailure = false
		}
		if errorLower != "" && errorLower != "null" && errorLower != "0" {
			isFailure = true
		}

		node := task.Node
		if node == "" {
			node = data.Node
		}
		resourceID := node
		if vmid != "" {
			resourceID = vmid
		}
		resourceName := vmid
		if task.VMName != "" {
			resourceName = task.VMName
		}
		if resourceName == "" {
			resourceName = resourceID
		}
		statusSummary := task.Status
		if strings.TrimSpace(statusSummary) == "" {
			statusSummary = task.Error
		}

		if prev, ok := latestByResource[resourceID]; !ok || taskEnd.After(prev.time) {
			latestByResource[resourceID] = latestBackup{
				time:          taskEnd,
				isFailure:     isFailure,
				statusSummary: statusSummary,
				resourceID:    resourceID,
				resourceName:  resourceName,
			}
		}
	}

	for _, entry := range latestByResource {
		if !entry.isFailure {
			continue
		}
		signals = append(signals, DetectedSignal{
			SignalType:        SignalBackupFailed,
			ResourceID:        entry.resourceID,
			ResourceName:      entry.resourceName,
			ResourceType:      "backup",
			SuggestedSeverity: "warning",
			Category:          string(FindingCategoryBackup),
			Summary:           "Backup task failed: " + entry.statusSummary,
			Evidence:          truncateEvidence(tc.Output),
			ToolCallID:        tc.ID,
		})
	}

	return signals
}

func detectBackupIssuesFromBackups(tc *ToolCallRecord, thresholds SignalThresholds) []DetectedSignal {
	var signals []DetectedSignal

	var data backupsResponse
	if err := json.Unmarshal([]byte(tc.Output), &data); err != nil {
		if !tryParseEmbeddedJSON(tc.Output, &data) {
			log.Debug().Err(err).Str("tool", tc.ToolName).Msg("patrol_signals: failed to parse backups output")
			return nil
		}
	}

	now := time.Now()
	knownVMIDs := make(map[string]bool)
	for _, task := range data.RecentTasks {
		if task.VMID == 0 {
			continue
		}
		knownVMIDs[fmt.Sprintf("%d", task.VMID)] = true
	}

	// Recent task failures (from backups response) - only the latest per resource
	type latestBackup struct {
		time          time.Time
		isFailure     bool
		statusSummary string
		resourceID    string
	}
	latestByResource := make(map[string]latestBackup)
	for _, task := range data.RecentTasks {
		statusLower := strings.ToLower(strings.TrimSpace(task.Status))
		if statusLower == "" {
			continue
		}
		isSuccess := statusLower == "ok" || strings.Contains(statusLower, "success")
		isFailure := statusLower != "" && !isSuccess
		if strings.Contains(statusLower, "error") || strings.Contains(statusLower, "fail") {
			isFailure = true
		}
		if strings.Contains(statusLower, "running") || strings.Contains(statusLower, "active") {
			isFailure = false
		}

		var taskTime time.Time
		if task.StartTime != "" {
			if parsed, err := tryParseTime(task.StartTime); err == nil {
				taskTime = parsed
			}
		}
		if taskTime.IsZero() {
			taskTime = now
		}
		if now.Sub(taskTime) > thresholds.BackupStaleThreshold {
			continue
		}

		vmid := strings.TrimSpace(fmt.Sprintf("%d", task.VMID))
		if vmid == "0" {
			vmid = ""
		}
		resourceID := vmid
		if resourceID == "" {
			resourceID = task.Node
		}
		if resourceID == "" {
			resourceID = "backup"
		}

		if prev, ok := latestByResource[resourceID]; !ok || taskTime.After(prev.time) {
			latestByResource[resourceID] = latestBackup{
				time:          taskTime,
				isFailure:     isFailure,
				statusSummary: task.Status,
				resourceID:    resourceID,
			}
		}
	}

	for _, entry := range latestByResource {
		if !entry.isFailure {
			continue
		}
		signals = append(signals, DetectedSignal{
			SignalType:        SignalBackupFailed,
			ResourceID:        entry.resourceID,
			ResourceName:      entry.resourceID,
			ResourceType:      "backup",
			SuggestedSeverity: "warning",
			Category:          string(FindingCategoryBackup),
			Summary:           "Backup task failed: " + entry.statusSummary,
			Evidence:          truncateEvidence(tc.Output),
			ToolCallID:        tc.ID,
		})
	}

	// Stale backup detection from backup summaries
	if len(knownVMIDs) == 0 {
		return signals
	}
	latestByVM := make(map[string]time.Time)
	for _, backup := range data.PBS {
		vmid := strings.TrimSpace(backup.VMID)
		if vmid == "" || vmid == "0" {
			continue
		}
		if !knownVMIDs[vmid] {
			continue
		}
		if t, err := tryParseTime(backup.BackupTime); err == nil {
			if prev, ok := latestByVM[vmid]; !ok || t.After(prev) {
				latestByVM[vmid] = t
			}
		}
	}
	for _, backup := range data.PVE {
		if backup.VMID == 0 {
			continue
		}
		vmid := fmt.Sprintf("%d", backup.VMID)
		if !knownVMIDs[vmid] {
			continue
		}
		if t, err := tryParseTime(backup.BackupTime); err == nil {
			if prev, ok := latestByVM[vmid]; !ok || t.After(prev) {
				latestByVM[vmid] = t
			}
		}
	}

	for vmid, newestEnd := range latestByVM {
		if now.Sub(newestEnd) > thresholds.BackupStaleThreshold {
			signals = append(signals, DetectedSignal{
				SignalType:        SignalBackupStale,
				ResourceID:        vmid,
				ResourceName:      vmid,
				ResourceType:      "backup",
				SuggestedSeverity: "warning",
				Category:          string(FindingCategoryBackup),
				Summary:           "No backup completed in 48+ hours for VM/CT " + vmid,
				Evidence:          truncateEvidence(tc.Output),
				ToolCallID:        tc.ID,
			})
		}
	}

	return signals
}

// --- Metrics signals ---

// metricsPerformance is the minimal struct for parsing performance metrics.
type metricsPerformance struct {
	Resources []struct {
		ID         string  `json:"id"`
		Name       string  `json:"name"`
		Type       string  `json:"type"`
		Node       string  `json:"node"`
		AvgCPU     float64 `json:"avg_cpu"`
		AvgMemory  float64 `json:"avg_memory"`
		MaxCPU     float64 `json:"max_cpu"`
		MaxMemory  float64 `json:"max_memory"`
		CPUPercent float64 `json:"cpu_percent"`
		MemPercent float64 `json:"mem_percent"`
	} `json:"resources"`
	// Also handle flat response (single resource)
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Type       string  `json:"type"`
	Node       string  `json:"node"`
	AvgCPU     float64 `json:"avg_cpu"`
	AvgMemory  float64 `json:"avg_memory"`
	MaxCPU     float64 `json:"max_cpu"`
	MaxMemory  float64 `json:"max_memory"`
	CPUPercent float64 `json:"cpu_percent"`
	MemPercent float64 `json:"mem_percent"`
}

func detectMetricsSignals(tc *ToolCallRecord, thresholds SignalThresholds) []DetectedSignal {
	inputType := extractInputType(tc.Input)
	if inputType != "performance" && inputType != "" {
		return nil
	}

	var signals []DetectedSignal
	var data metricsPerformance
	if err := json.Unmarshal([]byte(tc.Output), &data); err != nil {
		if !tryParseEmbeddedJSON(tc.Output, &data) {
			log.Debug().Err(err).Str("tool", tc.ToolName).Msg("patrol_signals: failed to parse performance output")
			return nil
		}
	}

	// Collect resources to check (handle both list and single)
	type resource struct {
		id, name, rtype, node string
		avgCPU, avgMemory     float64
	}
	var resources []resource

	if len(data.Resources) > 0 {
		for _, r := range data.Resources {
			resources = append(resources, resource{
				id: r.ID, name: r.Name, rtype: r.Type, node: r.Node,
				avgCPU:    selectNonZero(r.AvgCPU, r.CPUPercent),
				avgMemory: selectNonZero(r.AvgMemory, r.MemPercent),
			})
		}
	} else if data.ID != "" || data.Name != "" {
		resources = append(resources, resource{
			id: data.ID, name: data.Name, rtype: data.Type, node: data.Node,
			avgCPU:    selectNonZero(data.AvgCPU, data.CPUPercent),
			avgMemory: selectNonZero(data.AvgMemory, data.MemPercent),
		})
	}

	for _, r := range resources {
		if r.avgCPU > thresholds.HighCPUPercent {
			signals = append(signals, DetectedSignal{
				SignalType:        SignalHighCPU,
				ResourceID:        r.id,
				ResourceName:      r.name,
				ResourceType:      r.rtype,
				SuggestedSeverity: "warning",
				Category:          string(FindingCategoryPerformance),
				Summary:           fmt.Sprintf("High CPU usage on %s: %.1f%%", r.name, r.avgCPU),
				Evidence:          truncateEvidence(tc.Output),
				ToolCallID:        tc.ID,
			})
		}
		if r.avgMemory > thresholds.HighMemoryPercent {
			signals = append(signals, DetectedSignal{
				SignalType:        SignalHighMemory,
				ResourceID:        r.id,
				ResourceName:      r.name,
				ResourceType:      r.rtype,
				SuggestedSeverity: "warning",
				Category:          string(FindingCategoryPerformance),
				Summary:           fmt.Sprintf("High memory usage on %s: %.1f%%", r.name, r.avgMemory),
				Evidence:          truncateEvidence(tc.Output),
				ToolCallID:        tc.ID,
			})
		}
	}

	return signals
}

// --- Alert signals ---

// alertList is the minimal struct for parsing alert list output.
type alertList struct {
	Alerts []struct {
		ID           string `json:"id"`
		ResourceID   string `json:"resource_id"`
		ResourceName string `json:"resource_name"`
		Type         string `json:"type"`
		Severity     string `json:"severity"`
		Message      string `json:"message"`
	} `json:"alerts"`
}

func detectAlertSignals(tc *ToolCallRecord) []DetectedSignal {
	inputAction := extractInputField(tc.Input, "action")
	if inputAction != "list" {
		return nil
	}

	var signals []DetectedSignal
	var data alertList
	if err := json.Unmarshal([]byte(tc.Output), &data); err != nil {
		if !tryParseEmbeddedJSON(tc.Output, &data) {
			log.Debug().Err(err).Str("tool", tc.ToolName).Msg("patrol_signals: failed to parse alerts output")
			return nil
		}
	}

	for _, alert := range data.Alerts {
		sevLower := strings.ToLower(alert.Severity)
		if sevLower != "critical" && sevLower != "warning" {
			continue
		}

		resourceType := inferFindingResourceType(alert.ResourceID, alert.ResourceName)

		signals = append(signals, DetectedSignal{
			SignalType:        SignalActiveAlert,
			ResourceID:        alert.ResourceID,
			ResourceName:      alert.ResourceName,
			ResourceType:      resourceType,
			SuggestedSeverity: sevLower,
			Category:          string(FindingCategoryGeneral),
			Summary:           "Active " + sevLower + " alert: " + alert.Message,
			Evidence:          truncateEvidence(tc.Output),
			ToolCallID:        tc.ID,
		})
	}

	return signals
}

// --- Deduplication and matching ---

// deduplicateSignals removes duplicate signals with the same SignalType:ResourceID.
func deduplicateSignals(signals []DetectedSignal) []DetectedSignal {
	if len(signals) == 0 {
		return signals
	}

	seen := make(map[string]bool)
	var result []DetectedSignal

	for _, s := range signals {
		key := string(s.SignalType) + ":" + s.ResourceID
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, s)
	}

	return result
}

// UnmatchedSignals returns signals that don't have a corresponding finding already reported.
// A signal is considered matched if any finding shares the same ResourceID AND Category.
func UnmatchedSignals(signals []DetectedSignal, findings []*Finding) []DetectedSignal {
	if len(signals) == 0 {
		return nil
	}

	// Build a set of resourceID:category from existing findings
	matched := make(map[string]bool)
	for _, f := range findings {
		key := f.ResourceID + ":" + string(f.Category)
		matched[key] = true
	}

	var unmatched []DetectedSignal
	for _, s := range signals {
		key := s.ResourceID + ":" + s.Category
		if !matched[key] {
			unmatched = append(unmatched, s)
		}
	}

	return unmatched
}

// --- Helpers ---

// extractInputType extracts the "type" field from a tool call's input JSON string.
func extractInputType(input string) string {
	return extractInputField(input, "type")
}

// extractInputField extracts a named field from a tool call's input JSON string.
func extractInputField(input string, field string) string {
	if input == "" {
		return ""
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(input), &m); err != nil {
		return ""
	}
	if v, ok := m[field].(string); ok {
		return v
	}
	return ""
}

func shouldSkipSignalParsing(output string) bool {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return true
	}
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "no ") {
		return true
	}
	if strings.HasPrefix(lower, "state provider not available") {
		return true
	}
	return false
}

// tryParseEmbeddedJSON attempts to find and parse a JSON object embedded in a larger string.
func tryParseEmbeddedJSON(s string, v interface{}) bool {
	start := strings.Index(s, "{")
	if start == -1 {
		return false
	}
	// Find the matching closing brace by trying progressively from the end
	for end := len(s); end > start; end-- {
		if s[end-1] == '}' {
			if json.Unmarshal([]byte(s[start:end]), v) == nil {
				return true
			}
		}
	}
	return false
}

// truncateEvidence truncates evidence to a maximum of 500 characters.
func truncateEvidence(s string) string {
	if len(s) <= 500 {
		return s
	}
	return s[:500]
}

// selectNonZero returns the first non-zero value, preferring the first argument.
func selectNonZero(a, b float64) float64 {
	if a > 0 {
		return a
	}
	return b
}

// tryParseTime tries common time formats.
func tryParseTime(s string) (time.Time, error) {
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		time.RFC3339Nano,
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unable to parse time: %s", s)
}
