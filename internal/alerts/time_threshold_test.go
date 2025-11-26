package alerts

import (
	"testing"
	"time"
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
	manager.config.TimeThreshold = 15
	manager.mu.Unlock()

	testCases := []struct {
		resourceType string
		expected     int
	}{
		{"VM", 300},
		{"Container", 300},
		{"ct", 300},
		{"guest", 300},
		{"qemu", 300},
		{"lxc", 300},
		{"Node", 120},
		{"storage", 45},
		{"PBS", 90},
		{"UNKNOWN", 15},
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
	manager.config.TimeThreshold = 15
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

	if _, ok := manager.pendingAlerts["guest-123-cpu"]; !ok {
		manager.mu.Unlock()
		t.Fatalf("expected pending alert tracking to be started")
	}

	forcedStart := time.Now().Add(-3 * time.Second)
	manager.pendingAlerts["guest-123-cpu"] = forcedStart
	manager.mu.Unlock()

	// Second exceedance should trigger the alert using the pending start time.
	manager.checkMetric("guest-123", "test-vm", "node1", "qemu/123", "VM", "cpu", 90, threshold, nil)

	manager.mu.Lock()
	alert, exists := manager.activeAlerts["guest-123-cpu"]
	manager.mu.Unlock()

	if !exists {
		t.Fatalf("expected alert to be active after exceeding delay")
	}

	if !alert.StartTime.Equal(forcedStart) {
		t.Fatalf("expected alert start time %v, got %v", forcedStart, alert.StartTime)
	}
}
