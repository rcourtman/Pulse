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

// backupTask is the minimal struct for parsing backup task data.
type backupTask struct {
	Tasks []struct {
		ID      string `json:"id"`
		Status  string `json:"status"`
		EndTime string `json:"end_time"`
		VMID    string `json:"vmid"`
		Node    string `json:"node"`
		Type    string `json:"type"`
	} `json:"tasks"`
	Node string `json:"node"`
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

	// Track the most recent task per node for staleness check
	type nodeInfo struct {
		newestEnd time.Time
		node      string
	}
	newestPerNode := make(map[string]*nodeInfo)

	for _, task := range data.Tasks {
		statusLower := strings.ToLower(task.Status)

		// Check for failed backups
		if strings.Contains(statusLower, "error") || strings.Contains(statusLower, "fail") {
			node := task.Node
			if node == "" {
				node = data.Node
			}
			resourceID := node
			if task.VMID != "" {
				resourceID = task.VMID
			}

			signals = append(signals, DetectedSignal{
				SignalType:        SignalBackupFailed,
				ResourceID:        resourceID,
				ResourceName:      task.VMID,
				ResourceType:      "backup",
				SuggestedSeverity: "warning",
				Category:          string(FindingCategoryBackup),
				Summary:           "Backup task failed: " + task.Status,
				Evidence:          truncateEvidence(tc.Output),
				ToolCallID:        tc.ID,
			})
		}

		// Track newest end time per node
		if task.EndTime != "" {
			endTime, err := tryParseTime(task.EndTime)
			if err == nil {
				node := task.Node
				if node == "" {
					node = data.Node
				}
				if node == "" {
					node = "_default"
				}
				if ni, ok := newestPerNode[node]; !ok || endTime.After(ni.newestEnd) {
					newestPerNode[node] = &nodeInfo{newestEnd: endTime, node: node}
				}
			}
		}
	}

	// Check for stale backups (newest task older than threshold)
	now := time.Now()
	for _, ni := range newestPerNode {
		if now.Sub(ni.newestEnd) > thresholds.BackupStaleThreshold {
			signals = append(signals, DetectedSignal{
				SignalType:        SignalBackupStale,
				ResourceID:        ni.node,
				ResourceName:      ni.node,
				ResourceType:      "backup",
				SuggestedSeverity: "warning",
				Category:          string(FindingCategoryBackup),
				Summary:           "No backup completed in 48+ hours for " + ni.node,
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
	if inputType != "performance" {
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

		signals = append(signals, DetectedSignal{
			SignalType:        SignalActiveAlert,
			ResourceID:        alert.ResourceID,
			ResourceName:      alert.ResourceName,
			ResourceType:      "",
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
