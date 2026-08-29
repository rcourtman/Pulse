package alerts

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/reducer"
)

func TestGetTimeThresholdMappings(t *testing.T) {
	manager := NewManager()

	manager.mu.Lock()
	manager.config.TimeThresholds = map[string]int{
		"guest":   300,
		"node":    120,
		"storage": 45,
		"pbs":     90,
	}
	manager.mu.Unlock()

	testCases := []struct {
		resourceType string
		expected     int
	}{
		{"VM", 300},
		{"Container", 0},
		{"system-container", 300},
		{"guest", 300},
		{"qemu", 0},
		{"Node", 120},
		{"storage", 45},
		{"PBS", 90},
		{"UNKNOWN", 0},
	}

	for _, tc := range testCases {
		if got := manager.getTimeThreshold("", tc.resourceType, "cpu"); got != tc.expected {
			t.Errorf("getTimeThreshold(%q, \"cpu\") = %d, want %d", tc.resourceType, got, tc.expected)
		}
	}
}

func TestGetTimeThresholdMetricOverrides(t *testing.T) {
	manager := NewManager()

	manager.mu.Lock()
	manager.config.TimeThresholds = map[string]int{
		"guest":   30,
		"node":    60,
		"storage": 90,
	}
	manager.config.MetricTimeThresholds = map[string]map[string]int{
		"guest": {
			"cpu": 5,
		},
		"node": {
			"temperature": 120,
		},
		"all": {
			"default": 20,
		},
	}
	manager.mu.Unlock()

	cases := []struct {
		resourceID   string
		resourceType string
		metricType   string
		expected     int
	}{
		{"vm-resource", "VM", "cpu", 5},        // guest metric override
		{"vm-resource", "VM", "memory", 30},    // falls back to guest type delay
		{"node-1", "Node", "temperature", 120}, // node metric override
		{"node-1", "Node", "cpu", 60},          // node type delay
		{"storage-1", "storage", "usage", 90},  // storage type delay
		{"unknown", "unknown", "cpu", 20},      // global default metric override
		{"unknown", "unknown", "disk", 20},
	}

	for _, tc := range cases {
		if got := manager.getTimeThreshold(tc.resourceID, tc.resourceType, tc.metricType); got != tc.expected {
			t.Errorf("getTimeThreshold(%q, %q, %q) = %d, want %d", tc.resourceID, tc.resourceType, tc.metricType, got, tc.expected)
		}
	}
}

func TestGetTimeThresholdUsesTrustStableNoisyGaugeDefaults(t *testing.T) {
	manager := NewManagerWithDataDir(t.TempDir(), WithoutPersistedAlertRestore())

	t.Cleanup(manager.Stop)

	if got := manager.getTimeThreshold("vm-resource", "VM", "memory"); got != defaultNoisyGaugeStabilitySeconds {
		t.Fatalf("default memory stability = %d, want %d", got, defaultNoisyGaugeStabilitySeconds)
	}
	if got := manager.getTimeThreshold("node-1", "Node", "temperature"); got != defaultNoisyGaugeStabilitySeconds {
		t.Fatalf("default temperature stability = %d, want %d", got, defaultNoisyGaugeStabilitySeconds)
	}
	if got := manager.getTimeThreshold("vm-resource", "VM", "cpu"); got != defaultLegacyMetricDelaySeconds {
		t.Fatalf("default CPU delay = %d, want %d", got, defaultLegacyMetricDelaySeconds)
	}

	manager.mu.Lock()
	manager.config.MetricTimeThresholds = map[string]map[string]int{
		"all": {"memory": 0},
	}
	manager.mu.Unlock()

	if got := manager.getTimeThreshold("vm-resource", "VM", "memory"); got != 0 {
		t.Fatalf("explicit global memory override = %d, want 0", got)
	}
}

func TestCheckMetricNoisyWarningWaitsButCriticalFiresImmediately(t *testing.T) {
	manager := NewManagerWithDataDir(t.TempDir(), WithoutPersistedAlertRestore())
	t.Cleanup(manager.Stop)

	threshold := &HysteresisThreshold{Trigger: 85, Clear: 80}
	canonicalState := buildCanonicalStateID("vm-resource", "metric-threshold:memory")

	manager.checkMetric("vm-resource", "database", "node-1", "pve-1", "VM", "memory", 90, threshold, nil)
	manager.mu.RLock()
	_, warningActive := manager.getActiveAlertNoLock(canonicalState)
	manager.mu.RUnlock()
	if warningActive {
		t.Fatal("brief warning-level memory evidence opened an alert")
	}

	manager.checkMetric("vm-resource", "database", "node-1", "pve-1", "VM", "memory", 96, threshold, nil)
	manager.mu.RLock()
	alert, criticalActive := manager.getActiveAlertNoLock(canonicalState)
	manager.mu.RUnlock()
	if !criticalActive {
		t.Fatal("critical memory evidence did not bypass the factory stability delay")
	}
	if alert.Level != AlertLevelCritical {
		t.Fatalf("critical memory alert level = %q, want %q", alert.Level, AlertLevelCritical)
	}
}

func TestCheckMetricUsesPendingStartTime(t *testing.T) {
	manager := NewManager()

	manager.mu.Lock()
	manager.config.TimeThresholds["guest"] = 2
	manager.mu.Unlock()

	threshold := &HysteresisThreshold{Trigger: 80, Clear: 75}

	// First exceedance should start tracking and not create an alert immediately.
	manager.checkMetric("guest-123", "test-vm", "node1", "qemu/123", "VM", "cpu", 90, threshold, nil)

	manager.mu.Lock()
	if len(manager.activeAlerts) != 0 {
		manager.mu.Unlock()
		t.Fatalf("expected no active alerts after initial exceedance")
	}

	incident, pending := manager.core.Incident("guest-123", "metric-threshold:cpu")
	if !pending || incident.State != reducer.StatePending {
		manager.mu.Unlock()
		t.Fatalf("expected core pending run to be started")
	}

	// Age the core's pending run past the delay.
	manager.core.ShiftPending(-3 * time.Second)
	forcedIncident, _ := manager.core.Incident("guest-123", "metric-threshold:cpu")
	forcedStart := forcedIncident.PendingSince
	manager.mu.Unlock()

	// Second exceedance should trigger the alert using the pending start time.
	manager.checkMetric("guest-123", "test-vm", "node1", "qemu/123", "VM", "cpu", 90, threshold, nil)

	manager.mu.Lock()
	canonicalState := buildCanonicalStateID("guest-123", "metric-threshold:cpu")
	alert, exists := manager.activeAlerts[canonicalState]
	manager.mu.Unlock()

	if !exists {
		t.Fatalf("expected alert to be active after exceeding delay")
	}

	if !alert.StartTime.Equal(forcedStart) {
		t.Fatalf("expected alert start time %v, got %v", forcedStart, alert.StartTime)
	}
}
