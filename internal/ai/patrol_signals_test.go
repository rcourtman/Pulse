package ai

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDetectSignals_SMARTFailure(t *testing.T) {
	output, _ := json.Marshal(map[string]interface{}{
		"node": "pve1",
		"disks": []map[string]interface{}{
			{"device": "/dev/sda", "health": "PASSED", "model": "Samsung SSD"},
			{"device": "/dev/sdb", "health": "FAILED", "model": "WDC HDD"},
		},
	})

	toolCalls := []ToolCallRecord{
		{
			ID:       "tc1",
			ToolName: "pulse_storage",
			Input:    `{"type":"disk_health","node":"pve1"}`,
			Output:   string(output),
			Success:  true,
		},
	}

	signals := DetectSignals(toolCalls, DefaultSignalThresholds())

	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(signals))
	}
	s := signals[0]
	if s.SignalType != SignalSMARTFailure {
		t.Errorf("expected SignalSMARTFailure, got %s", s.SignalType)
	}
	if s.SuggestedSeverity != "critical" {
		t.Errorf("expected critical severity, got %s", s.SuggestedSeverity)
	}
	if s.ResourceID != "pve1" {
		t.Errorf("expected resource ID pve1, got %s", s.ResourceID)
	}
	if s.Category != string(FindingCategoryReliability) {
		t.Errorf("expected reliability category, got %s", s.Category)
	}
}

func TestDetectSignals_SMARTPassedNoSignal(t *testing.T) {
	output, _ := json.Marshal(map[string]interface{}{
		"node": "pve1",
		"disks": []map[string]interface{}{
			{"device": "/dev/sda", "health": "PASSED"},
			{"device": "/dev/sdb", "health": "OK"},
		},
	})

	toolCalls := []ToolCallRecord{
		{
			ID:       "tc1",
			ToolName: "pulse_storage",
			Input:    `{"type":"disk_health"}`,
			Output:   string(output),
			Success:  true,
		},
	}

	signals := DetectSignals(toolCalls, DefaultSignalThresholds())
	if len(signals) != 0 {
		t.Fatalf("expected 0 signals for healthy disks, got %d", len(signals))
	}
}

func TestDetectSignals_HighCPU(t *testing.T) {
	output, _ := json.Marshal(map[string]interface{}{
		"resources": []map[string]interface{}{
			{"id": "node1", "name": "pve1", "type": "node", "avg_cpu": 85.5, "avg_memory": 40.0},
			{"id": "node2", "name": "pve2", "type": "node", "avg_cpu": 30.0, "avg_memory": 50.0},
		},
	})

	toolCalls := []ToolCallRecord{
		{
			ID:       "tc1",
			ToolName: "pulse_metrics",
			Input:    `{"type":"performance"}`,
			Output:   string(output),
			Success:  true,
		},
	}

	signals := DetectSignals(toolCalls, DefaultSignalThresholds())
	if len(signals) != 1 {
		t.Fatalf("expected 1 signal (high CPU), got %d", len(signals))
	}
	if signals[0].SignalType != SignalHighCPU {
		t.Errorf("expected SignalHighCPU, got %s", signals[0].SignalType)
	}
	if signals[0].ResourceID != "node1" {
		t.Errorf("expected resource ID node1, got %s", signals[0].ResourceID)
	}
}

func TestDetectSignals_HighMemory(t *testing.T) {
	output, _ := json.Marshal(map[string]interface{}{
		"id":         "vm100",
		"name":       "webserver",
		"type":       "vm",
		"avg_cpu":    20.0,
		"avg_memory": 92.3,
	})

	toolCalls := []ToolCallRecord{
		{
			ID:       "tc1",
			ToolName: "pulse_metrics",
			Input:    `{"type":"performance"}`,
			Output:   string(output),
			Success:  true,
		},
	}

	signals := DetectSignals(toolCalls, DefaultSignalThresholds())
	if len(signals) != 1 {
		t.Fatalf("expected 1 signal (high memory), got %d", len(signals))
	}
	if signals[0].SignalType != SignalHighMemory {
		t.Errorf("expected SignalHighMemory, got %s", signals[0].SignalType)
	}
}

func TestDetectSignals_HighDisk(t *testing.T) {
	output, _ := json.Marshal(map[string]interface{}{
		"pools": []map[string]interface{}{
			{"name": "local-lvm", "id": "local-lvm", "usage_percent": 80.5},
			{"name": "ceph-pool", "id": "ceph-pool", "usage_percent": 50.0},
			{"name": "backup", "id": "backup", "usage_percent": 97.0},
		},
	})

	toolCalls := []ToolCallRecord{
		{
			ID:       "tc1",
			ToolName: "pulse_storage",
			Input:    `{"type":"pools"}`,
			Output:   string(output),
			Success:  true,
		},
	}

	signals := DetectSignals(toolCalls, DefaultSignalThresholds())
	if len(signals) != 2 {
		t.Fatalf("expected 2 signals (2 pools over threshold), got %d", len(signals))
	}

	// Check that the critical one is reported correctly
	var foundCritical bool
	for _, s := range signals {
		if s.ResourceID == "backup" && s.SuggestedSeverity == "critical" {
			foundCritical = true
		}
	}
	if !foundCritical {
		t.Error("expected critical severity for backup pool at 97%")
	}
}

func TestDetectSignals_BackupFailed(t *testing.T) {
	output, _ := json.Marshal(map[string]interface{}{
		"node": "pve1",
		"tasks": []map[string]interface{}{
			{"id": "task1", "status": "OK", "vmid": "100", "end_time": time.Now().Format(time.RFC3339)},
			{"id": "task2", "status": "ERROR: backup failed", "vmid": "101", "end_time": time.Now().Format(time.RFC3339)},
		},
	})

	toolCalls := []ToolCallRecord{
		{
			ID:       "tc1",
			ToolName: "pulse_storage",
			Input:    `{"type":"backup_tasks"}`,
			Output:   string(output),
			Success:  true,
		},
	}

	signals := DetectSignals(toolCalls, DefaultSignalThresholds())
	if len(signals) != 1 {
		t.Fatalf("expected 1 signal (backup failed), got %d", len(signals))
	}
	if signals[0].SignalType != SignalBackupFailed {
		t.Errorf("expected SignalBackupFailed, got %s", signals[0].SignalType)
	}
	if signals[0].ResourceID != "101" {
		t.Errorf("expected resource ID 101 (VMID), got %s", signals[0].ResourceID)
	}
}

func TestDetectSignals_BackupStale(t *testing.T) {
	staleTime := time.Now().Add(-72 * time.Hour).Format(time.RFC3339)
	output, _ := json.Marshal(map[string]interface{}{
		"node": "pve1",
		"tasks": []map[string]interface{}{
			{"id": "task1", "status": "OK", "vmid": "100", "node": "pve1", "end_time": staleTime},
		},
	})

	toolCalls := []ToolCallRecord{
		{
			ID:       "tc1",
			ToolName: "pulse_storage",
			Input:    `{"type":"backup_tasks"}`,
			Output:   string(output),
			Success:  true,
		},
	}

	signals := DetectSignals(toolCalls, DefaultSignalThresholds())
	if len(signals) != 1 {
		t.Fatalf("expected 1 signal (stale backup), got %d", len(signals))
	}
	if signals[0].SignalType != SignalBackupStale {
		t.Errorf("expected SignalBackupStale, got %s", signals[0].SignalType)
	}
}

func TestDetectSignals_ActiveAlert(t *testing.T) {
	output, _ := json.Marshal(map[string]interface{}{
		"alerts": []map[string]interface{}{
			{"id": "a1", "resource_id": "node1", "resource_name": "pve1", "severity": "critical", "message": "Node offline"},
			{"id": "a2", "resource_id": "vm100", "resource_name": "web", "severity": "info", "message": "Minor issue"},
		},
	})

	toolCalls := []ToolCallRecord{
		{
			ID:       "tc1",
			ToolName: "pulse_alerts",
			Input:    `{"action":"list"}`,
			Output:   string(output),
			Success:  true,
		},
	}

	signals := DetectSignals(toolCalls, DefaultSignalThresholds())
	if len(signals) != 1 {
		t.Fatalf("expected 1 signal (only critical/warning alerts), got %d", len(signals))
	}
	if signals[0].SignalType != SignalActiveAlert {
		t.Errorf("expected SignalActiveAlert, got %s", signals[0].SignalType)
	}
	if signals[0].SuggestedSeverity != "critical" {
		t.Errorf("expected critical severity pass-through, got %s", signals[0].SuggestedSeverity)
	}
}

func TestDetectSignals_FailedToolCallSkipped(t *testing.T) {
	toolCalls := []ToolCallRecord{
		{
			ID:       "tc1",
			ToolName: "pulse_storage",
			Input:    `{"type":"disk_health"}`,
			Output:   `{"disks":[{"device":"/dev/sda","health":"FAILED"}],"node":"pve1"}`,
			Success:  false, // Failed tool call
		},
	}

	signals := DetectSignals(toolCalls, DefaultSignalThresholds())
	if len(signals) != 0 {
		t.Fatalf("expected 0 signals for failed tool call, got %d", len(signals))
	}
}

func TestDetectSignals_EmptyOutputSkipped(t *testing.T) {
	toolCalls := []ToolCallRecord{
		{
			ID:       "tc1",
			ToolName: "pulse_storage",
			Input:    `{"type":"disk_health"}`,
			Output:   "",
			Success:  true,
		},
	}

	signals := DetectSignals(toolCalls, DefaultSignalThresholds())
	if len(signals) != 0 {
		t.Fatalf("expected 0 signals for empty output, got %d", len(signals))
	}
}

func TestDetectSignals_MalformedJSON(t *testing.T) {
	toolCalls := []ToolCallRecord{
		{
			ID:       "tc1",
			ToolName: "pulse_storage",
			Input:    `{"type":"disk_health"}`,
			Output:   "not valid json at all",
			Success:  true,
		},
	}

	signals := DetectSignals(toolCalls, DefaultSignalThresholds())
	if len(signals) != 0 {
		t.Fatalf("expected 0 signals for malformed JSON, got %d", len(signals))
	}
}

func TestDeduplicateSignals(t *testing.T) {
	signals := []DetectedSignal{
		{SignalType: SignalSMARTFailure, ResourceID: "pve1"},
		{SignalType: SignalSMARTFailure, ResourceID: "pve1"}, // duplicate
		{SignalType: SignalSMARTFailure, ResourceID: "pve2"}, // different resource
		{SignalType: SignalHighCPU, ResourceID: "pve1"},      // different signal type, same resource
	}

	result := deduplicateSignals(signals)
	if len(result) != 3 {
		t.Fatalf("expected 3 unique signals, got %d", len(result))
	}
}

func TestUnmatchedSignals(t *testing.T) {
	signals := []DetectedSignal{
		{SignalType: SignalSMARTFailure, ResourceID: "pve1", Category: "reliability"},
		{SignalType: SignalHighCPU, ResourceID: "node1", Category: "performance"},
		{SignalType: SignalHighDisk, ResourceID: "local-lvm", Category: "capacity"},
	}

	findings := []*Finding{
		{ResourceID: "pve1", Category: FindingCategoryReliability},
		// node1 performance is NOT covered
		// local-lvm capacity is NOT covered
	}

	unmatched := UnmatchedSignals(signals, findings)
	if len(unmatched) != 2 {
		t.Fatalf("expected 2 unmatched signals, got %d", len(unmatched))
	}

	for _, s := range unmatched {
		if s.ResourceID == "pve1" && s.Category == "reliability" {
			t.Error("pve1/reliability should have been matched")
		}
	}
}

func TestUnmatchedSignals_AllMatched(t *testing.T) {
	signals := []DetectedSignal{
		{SignalType: SignalSMARTFailure, ResourceID: "pve1", Category: "reliability"},
	}

	findings := []*Finding{
		{ResourceID: "pve1", Category: FindingCategoryReliability},
	}

	unmatched := UnmatchedSignals(signals, findings)
	if len(unmatched) != 0 {
		t.Fatalf("expected 0 unmatched signals, got %d", len(unmatched))
	}
}

func TestUnmatchedSignals_EmptySignals(t *testing.T) {
	unmatched := UnmatchedSignals(nil, []*Finding{{ResourceID: "pve1"}})
	if unmatched != nil {
		t.Fatalf("expected nil for empty signals, got %v", unmatched)
	}
}

func TestDetectSignals_NonPerformanceMetricsIgnored(t *testing.T) {
	output, _ := json.Marshal(map[string]interface{}{
		"id":      "node1",
		"name":    "pve1",
		"avg_cpu": 90.0,
	})

	toolCalls := []ToolCallRecord{
		{
			ID:       "tc1",
			ToolName: "pulse_metrics",
			Input:    `{"type":"temperature"}`,
			Output:   string(output),
			Success:  true,
		},
	}

	signals := DetectSignals(toolCalls, DefaultSignalThresholds())
	if len(signals) != 0 {
		t.Fatalf("expected 0 signals for non-performance metrics type, got %d", len(signals))
	}
}

func TestDetectSignals_AlertsNonListIgnored(t *testing.T) {
	output, _ := json.Marshal(map[string]interface{}{
		"alerts": []map[string]interface{}{
			{"id": "a1", "severity": "critical", "message": "test"},
		},
	})

	toolCalls := []ToolCallRecord{
		{
			ID:       "tc1",
			ToolName: "pulse_alerts",
			Input:    `{"action":"get","id":"a1"}`,
			Output:   string(output),
			Success:  true,
		},
	}

	signals := DetectSignals(toolCalls, DefaultSignalThresholds())
	if len(signals) != 0 {
		t.Fatalf("expected 0 signals for non-list alert action, got %d", len(signals))
	}
}

func TestDetectSignals_TruncatedJSON(t *testing.T) {
	// Truncated JSON should be handled gracefully (no panic, no signals)
	toolCalls := []ToolCallRecord{
		{
			ID:       "tc1",
			ToolName: "pulse_storage",
			Input:    `{"type":"disk_health"}`,
			Output:   `{"disks":[{"device":"/dev/sda","health":"FAIL`,
			Success:  true,
		},
	}

	signals := DetectSignals(toolCalls, DefaultSignalThresholds())
	// Should not panic and should return 0 signals for unparseable JSON
	if len(signals) != 0 {
		t.Fatalf("expected 0 signals for truncated JSON, got %d", len(signals))
	}
}

func TestDetectSignals_MultipleJSONObjects(t *testing.T) {
	// Output contains embedded JSON within a text wrapper
	// This tests the tryParseEmbeddedJSON fallback
	output := `Some text before {"node":"pve1","disks":[{"device":"/dev/sda","health":"FAILED"}]} some text after`

	toolCalls := []ToolCallRecord{
		{
			ID:       "tc1",
			ToolName: "pulse_storage",
			Input:    `{"type":"disk_health"}`,
			Output:   output,
			Success:  true,
		},
	}

	signals := DetectSignals(toolCalls, DefaultSignalThresholds())
	if len(signals) != 1 {
		t.Fatalf("expected 1 signal from embedded JSON, got %d", len(signals))
	}
	if signals[0].SignalType != SignalSMARTFailure {
		t.Errorf("expected SignalSMARTFailure, got %s", signals[0].SignalType)
	}
}

func TestDetectSignals_MetricsCPUPercentFallback(t *testing.T) {
	// Test that cpu_percent is used when avg_cpu is 0
	output, _ := json.Marshal(map[string]interface{}{
		"id":          "node1",
		"name":        "pve1",
		"type":        "node",
		"avg_cpu":     0.0,
		"cpu_percent": 75.0,
		"avg_memory":  0.0,
		"mem_percent": 85.0,
	})

	toolCalls := []ToolCallRecord{
		{
			ID:       "tc1",
			ToolName: "pulse_metrics",
			Input:    `{"type":"performance"}`,
			Output:   string(output),
			Success:  true,
		},
	}

	signals := DetectSignals(toolCalls, DefaultSignalThresholds())
	if len(signals) != 2 {
		t.Fatalf("expected 2 signals (CPU + memory via fallback), got %d", len(signals))
	}
}

func TestExtractInputField(t *testing.T) {
	tests := []struct {
		input    string
		field    string
		expected string
	}{
		{`{"type":"disk_health","node":"pve1"}`, "type", "disk_health"},
		{`{"action":"list"}`, "action", "list"},
		{`{"type":"performance"}`, "type", "performance"},
		{`{}`, "type", ""},
		{``, "type", ""},
		{`not json`, "type", ""},
	}

	for _, tt := range tests {
		result := extractInputField(tt.input, tt.field)
		if result != tt.expected {
			t.Errorf("extractInputField(%q, %q) = %q, want %q", tt.input, tt.field, result, tt.expected)
		}
	}
}

func TestDetectSignals_CustomThresholdsCPU(t *testing.T) {
	// With custom CPU threshold at 50%, a resource at 55% should trigger
	output, _ := json.Marshal(map[string]interface{}{
		"resources": []map[string]interface{}{
			{"id": "node1", "name": "pve1", "type": "node", "avg_cpu": 55.0, "avg_memory": 40.0},
		},
	})

	toolCalls := []ToolCallRecord{
		{
			ID:       "tc1",
			ToolName: "pulse_metrics",
			Input:    `{"type":"performance"}`,
			Output:   string(output),
			Success:  true,
		},
	}

	// Default threshold (70%) should NOT trigger at 55%
	signals := DetectSignals(toolCalls, DefaultSignalThresholds())
	if len(signals) != 0 {
		t.Fatalf("expected 0 signals with default thresholds (70%% CPU), got %d", len(signals))
	}

	// Custom threshold at 50% SHOULD trigger at 55%
	custom := SignalThresholds{
		StorageWarningPercent:  75,
		StorageCriticalPercent: 95,
		HighCPUPercent:         50.0,
		HighMemoryPercent:      80.0,
		BackupStaleThreshold:   48 * time.Hour,
	}
	signals = DetectSignals(toolCalls, custom)
	if len(signals) != 1 {
		t.Fatalf("expected 1 signal with custom CPU threshold (50%%), got %d", len(signals))
	}
	if signals[0].SignalType != SignalHighCPU {
		t.Errorf("expected SignalHighCPU, got %s", signals[0].SignalType)
	}
}

func TestDetectSignals_CustomThresholdsStorage(t *testing.T) {
	// With custom storage threshold at 90%, a pool at 80% should NOT trigger
	output, _ := json.Marshal(map[string]interface{}{
		"pools": []map[string]interface{}{
			{"name": "local-lvm", "id": "local-lvm", "usage_percent": 80.5},
		},
	})

	toolCalls := []ToolCallRecord{
		{
			ID:       "tc1",
			ToolName: "pulse_storage",
			Input:    `{"type":"pools"}`,
			Output:   string(output),
			Success:  true,
		},
	}

	// Default threshold (75%) triggers at 80.5%
	signals := DetectSignals(toolCalls, DefaultSignalThresholds())
	if len(signals) != 1 {
		t.Fatalf("expected 1 signal with default thresholds, got %d", len(signals))
	}

	// Custom threshold at 90% should NOT trigger at 80.5%
	custom := SignalThresholds{
		StorageWarningPercent:  90.0,
		StorageCriticalPercent: 95.0,
		HighCPUPercent:         70.0,
		HighMemoryPercent:      80.0,
		BackupStaleThreshold:   48 * time.Hour,
	}
	signals = DetectSignals(toolCalls, custom)
	if len(signals) != 0 {
		t.Fatalf("expected 0 signals with custom storage threshold (90%%), got %d", len(signals))
	}
}

func TestSignalThresholdsFromPatrol_ZeroFallback(t *testing.T) {
	// Zero-value PatrolThresholds should fall back to defaults
	st := SignalThresholdsFromPatrol(PatrolThresholds{})
	defaults := DefaultSignalThresholds()

	if st.StorageWarningPercent != defaults.StorageWarningPercent {
		t.Errorf("expected StorageWarningPercent=%v, got %v", defaults.StorageWarningPercent, st.StorageWarningPercent)
	}
	if st.StorageCriticalPercent != defaults.StorageCriticalPercent {
		t.Errorf("expected StorageCriticalPercent=%v, got %v", defaults.StorageCriticalPercent, st.StorageCriticalPercent)
	}
	if st.HighCPUPercent != defaults.HighCPUPercent {
		t.Errorf("expected HighCPUPercent=%v, got %v", defaults.HighCPUPercent, st.HighCPUPercent)
	}
	if st.HighMemoryPercent != defaults.HighMemoryPercent {
		t.Errorf("expected HighMemoryPercent=%v, got %v", defaults.HighMemoryPercent, st.HighMemoryPercent)
	}
}

func TestSignalThresholdsFromPatrol_UserConfig(t *testing.T) {
	pt := PatrolThresholds{
		StorageWarning:  85.0,
		StorageCritical: 92.0,
		NodeCPUWarning:  60.0,
		NodeMemWarning:  70.0,
	}
	st := SignalThresholdsFromPatrol(pt)

	if st.StorageWarningPercent != 85.0 {
		t.Errorf("expected StorageWarningPercent=85, got %v", st.StorageWarningPercent)
	}
	if st.StorageCriticalPercent != 92.0 {
		t.Errorf("expected StorageCriticalPercent=92, got %v", st.StorageCriticalPercent)
	}
	if st.HighCPUPercent != 60.0 {
		t.Errorf("expected HighCPUPercent=60, got %v", st.HighCPUPercent)
	}
	if st.HighMemoryPercent != 70.0 {
		t.Errorf("expected HighMemoryPercent=70, got %v", st.HighMemoryPercent)
	}
}

func TestTruncateEvidence(t *testing.T) {
	short := "short string"
	if truncateEvidence(short) != short {
		t.Error("short string should not be truncated")
	}

	long := make([]byte, 600)
	for i := range long {
		long[i] = 'x'
	}
	result := truncateEvidence(string(long))
	if len(result) != 500 {
		t.Errorf("expected truncated to 500 chars, got %d", len(result))
	}
}
