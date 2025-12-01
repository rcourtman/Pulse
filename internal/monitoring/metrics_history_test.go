package monitoring

import (
	"testing"
	"time"
)

func TestNewMetricsHistory(t *testing.T) {
	tests := []struct {
		name          string
		maxDataPoints int
		retentionTime time.Duration
	}{
		{
			name:          "standard values",
			maxDataPoints: 100,
			retentionTime: time.Hour,
		},
		{
			name:          "zero max points",
			maxDataPoints: 0,
			retentionTime: time.Minute,
		},
		{
			name:          "zero retention",
			maxDataPoints: 50,
			retentionTime: 0,
		},
		{
			name:          "large values",
			maxDataPoints: 10000,
			retentionTime: 24 * time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mh := NewMetricsHistory(tt.maxDataPoints, tt.retentionTime)
			if mh == nil {
				t.Fatal("NewMetricsHistory returned nil")
			}
			if mh.maxDataPoints != tt.maxDataPoints {
				t.Errorf("maxDataPoints = %d, want %d", mh.maxDataPoints, tt.maxDataPoints)
			}
			if mh.retentionTime != tt.retentionTime {
				t.Errorf("retentionTime = %v, want %v", mh.retentionTime, tt.retentionTime)
			}
			if mh.guestMetrics == nil {
				t.Error("guestMetrics map not initialized")
			}
			if mh.nodeMetrics == nil {
				t.Error("nodeMetrics map not initialized")
			}
			if mh.storageMetrics == nil {
				t.Error("storageMetrics map not initialized")
			}
		})
	}
}

func TestAppendMetric(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name          string
		maxDataPoints int
		retentionTime time.Duration
		existing      []MetricPoint
		newPoint      MetricPoint
		wantLen       int
		wantFirst     float64 // value of first point after append
		wantLast      float64 // value of last point after append
	}{
		{
			name:          "append to empty slice",
			maxDataPoints: 10,
			retentionTime: time.Hour,
			existing:      []MetricPoint{},
			newPoint:      MetricPoint{Value: 50.0, Timestamp: now},
			wantLen:       1,
			wantFirst:     50.0,
			wantLast:      50.0,
		},
		{
			name:          "append within limits",
			maxDataPoints: 10,
			retentionTime: time.Hour,
			existing: []MetricPoint{
				{Value: 10.0, Timestamp: now.Add(-30 * time.Minute)},
				{Value: 20.0, Timestamp: now.Add(-20 * time.Minute)},
			},
			newPoint:  MetricPoint{Value: 30.0, Timestamp: now},
			wantLen:   3,
			wantFirst: 10.0,
			wantLast:  30.0,
		},
		{
			name:          "exceed max data points",
			maxDataPoints: 3,
			retentionTime: time.Hour,
			existing: []MetricPoint{
				{Value: 10.0, Timestamp: now.Add(-30 * time.Minute)},
				{Value: 20.0, Timestamp: now.Add(-20 * time.Minute)},
				{Value: 30.0, Timestamp: now.Add(-10 * time.Minute)},
			},
			newPoint:  MetricPoint{Value: 40.0, Timestamp: now},
			wantLen:   3,
			wantFirst: 20.0, // oldest point dropped
			wantLast:  40.0,
		},
		{
			name:          "old points beyond retention removed",
			maxDataPoints: 100,
			retentionTime: 30 * time.Minute,
			existing: []MetricPoint{
				{Value: 10.0, Timestamp: now.Add(-2 * time.Hour)},  // beyond retention
				{Value: 20.0, Timestamp: now.Add(-90 * time.Minute)}, // beyond retention
				{Value: 30.0, Timestamp: now.Add(-20 * time.Minute)}, // within retention
			},
			newPoint:  MetricPoint{Value: 40.0, Timestamp: now},
			wantLen:   2,
			wantFirst: 30.0, // old points removed
			wantLast:  40.0,
		},
		{
			name:          "all old points removed except new",
			maxDataPoints: 100,
			retentionTime: 10 * time.Minute,
			existing: []MetricPoint{
				{Value: 10.0, Timestamp: now.Add(-2 * time.Hour)},
				{Value: 20.0, Timestamp: now.Add(-1 * time.Hour)},
			},
			newPoint:  MetricPoint{Value: 30.0, Timestamp: now},
			wantLen:   1,
			wantFirst: 30.0,
			wantLast:  30.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mh := NewMetricsHistory(tt.maxDataPoints, tt.retentionTime)
			result := mh.appendMetric(tt.existing, tt.newPoint)

			if len(result) != tt.wantLen {
				t.Errorf("len(result) = %d, want %d", len(result), tt.wantLen)
			}
			if len(result) > 0 {
				if result[0].Value != tt.wantFirst {
					t.Errorf("first value = %v, want %v", result[0].Value, tt.wantFirst)
				}
				if result[len(result)-1].Value != tt.wantLast {
					t.Errorf("last value = %v, want %v", result[len(result)-1].Value, tt.wantLast)
				}
			}
		})
	}
}

func TestCleanupMetrics(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name       string
		metrics    []MetricPoint
		cutoffTime time.Time
		wantLen    int
		wantFirst  float64
	}{
		{
			name:       "empty slice",
			metrics:    []MetricPoint{},
			cutoffTime: now.Add(-time.Hour),
			wantLen:    0,
		},
		{
			name: "no points to remove",
			metrics: []MetricPoint{
				{Value: 10.0, Timestamp: now.Add(-30 * time.Minute)},
				{Value: 20.0, Timestamp: now.Add(-20 * time.Minute)},
				{Value: 30.0, Timestamp: now.Add(-10 * time.Minute)},
			},
			cutoffTime: now.Add(-time.Hour),
			wantLen:    3,
			wantFirst:  10.0,
		},
		{
			name: "remove some old points",
			metrics: []MetricPoint{
				{Value: 10.0, Timestamp: now.Add(-2 * time.Hour)},
				{Value: 20.0, Timestamp: now.Add(-90 * time.Minute)},
				{Value: 30.0, Timestamp: now.Add(-30 * time.Minute)},
				{Value: 40.0, Timestamp: now.Add(-10 * time.Minute)},
			},
			cutoffTime: now.Add(-time.Hour),
			wantLen:    2,
			wantFirst:  30.0,
		},
		{
			name: "remove all points",
			metrics: []MetricPoint{
				{Value: 10.0, Timestamp: now.Add(-3 * time.Hour)},
				{Value: 20.0, Timestamp: now.Add(-2 * time.Hour)},
			},
			cutoffTime: now.Add(-time.Hour),
			wantLen:    2, // cleanupMetrics doesn't remove all - keeps slice if nothing after cutoff
		},
		{
			name: "boundary - point exactly at cutoff",
			metrics: []MetricPoint{
				{Value: 10.0, Timestamp: now.Add(-time.Hour)}, // exactly at cutoff (not after)
				{Value: 20.0, Timestamp: now.Add(-30 * time.Minute)},
			},
			cutoffTime: now.Add(-time.Hour),
			wantLen:    1,
			wantFirst:  20.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mh := NewMetricsHistory(100, time.Hour)
			result := mh.cleanupMetrics(tt.metrics, tt.cutoffTime)

			if len(result) != tt.wantLen {
				t.Errorf("len(result) = %d, want %d", len(result), tt.wantLen)
			}
			if len(result) > 0 && tt.wantFirst != 0 {
				if result[0].Value != tt.wantFirst {
					t.Errorf("first value = %v, want %v", result[0].Value, tt.wantFirst)
				}
			}
		})
	}
}

func TestAddGuestMetric(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name       string
		guestID    string
		metricType string
		value      float64
	}{
		{name: "cpu metric", guestID: "vm-100", metricType: "cpu", value: 75.5},
		{name: "memory metric", guestID: "vm-100", metricType: "memory", value: 80.0},
		{name: "disk metric", guestID: "ct-101", metricType: "disk", value: 50.0},
		{name: "diskread metric", guestID: "vm-102", metricType: "diskread", value: 100.5},
		{name: "diskwrite metric", guestID: "vm-102", metricType: "diskwrite", value: 50.25},
		{name: "netin metric", guestID: "vm-103", metricType: "netin", value: 1000.0},
		{name: "netout metric", guestID: "vm-103", metricType: "netout", value: 500.0},
		{name: "unknown metric type", guestID: "vm-104", metricType: "unknown", value: 99.9},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mh := NewMetricsHistory(100, time.Hour)
			mh.AddGuestMetric(tt.guestID, tt.metricType, tt.value, now)

			// Verify guest was created
			mh.mu.RLock()
			defer mh.mu.RUnlock()

			metrics, exists := mh.guestMetrics[tt.guestID]
			if !exists {
				t.Fatal("guest metrics not created")
			}

			// Verify correct metric type was populated
			var slice []MetricPoint
			switch tt.metricType {
			case "cpu":
				slice = metrics.CPU
			case "memory":
				slice = metrics.Memory
			case "disk":
				slice = metrics.Disk
			case "diskread":
				slice = metrics.DiskRead
			case "diskwrite":
				slice = metrics.DiskWrite
			case "netin":
				slice = metrics.NetworkIn
			case "netout":
				slice = metrics.NetworkOut
			default:
				// Unknown types should not populate any slice
				if len(metrics.CPU)+len(metrics.Memory)+len(metrics.Disk)+
					len(metrics.DiskRead)+len(metrics.DiskWrite)+
					len(metrics.NetworkIn)+len(metrics.NetworkOut) != 0 {
					t.Error("unknown metric type populated a slice")
				}
				return
			}

			if len(slice) != 1 {
				t.Errorf("slice length = %d, want 1", len(slice))
			} else if slice[0].Value != tt.value {
				t.Errorf("value = %v, want %v", slice[0].Value, tt.value)
			}
		})
	}
}

func TestAddNodeMetric(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name       string
		nodeID     string
		metricType string
		value      float64
		shouldAdd  bool
	}{
		{name: "cpu metric", nodeID: "node1", metricType: "cpu", value: 45.5, shouldAdd: true},
		{name: "memory metric", nodeID: "node1", metricType: "memory", value: 60.0, shouldAdd: true},
		{name: "disk metric", nodeID: "node2", metricType: "disk", value: 70.0, shouldAdd: true},
		{name: "unknown metric type", nodeID: "node3", metricType: "netin", value: 100.0, shouldAdd: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mh := NewMetricsHistory(100, time.Hour)
			mh.AddNodeMetric(tt.nodeID, tt.metricType, tt.value, now)

			mh.mu.RLock()
			defer mh.mu.RUnlock()

			metrics, exists := mh.nodeMetrics[tt.nodeID]
			if !exists {
				t.Fatal("node metrics not created")
			}

			var slice []MetricPoint
			switch tt.metricType {
			case "cpu":
				slice = metrics.CPU
			case "memory":
				slice = metrics.Memory
			case "disk":
				slice = metrics.Disk
			default:
				if tt.shouldAdd {
					t.Error("expected metric to be added")
				}
				return
			}

			if tt.shouldAdd {
				if len(slice) != 1 {
					t.Errorf("slice length = %d, want 1", len(slice))
				} else if slice[0].Value != tt.value {
					t.Errorf("value = %v, want %v", slice[0].Value, tt.value)
				}
			}
		})
	}
}

func TestAddStorageMetric(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name       string
		storageID  string
		metricType string
		value      float64
		shouldAdd  bool
	}{
		{name: "usage metric", storageID: "local-lvm", metricType: "usage", value: 45.5, shouldAdd: true},
		{name: "used metric", storageID: "local-lvm", metricType: "used", value: 100e9, shouldAdd: true},
		{name: "total metric", storageID: "ceph-pool", metricType: "total", value: 500e9, shouldAdd: true},
		{name: "avail metric", storageID: "ceph-pool", metricType: "avail", value: 400e9, shouldAdd: true},
		{name: "unknown metric type", storageID: "nfs-share", metricType: "iops", value: 1000.0, shouldAdd: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mh := NewMetricsHistory(100, time.Hour)
			mh.AddStorageMetric(tt.storageID, tt.metricType, tt.value, now)

			mh.mu.RLock()
			defer mh.mu.RUnlock()

			metrics, exists := mh.storageMetrics[tt.storageID]
			if !exists {
				t.Fatal("storage metrics not created")
			}

			var slice []MetricPoint
			switch tt.metricType {
			case "usage":
				slice = metrics.Usage
			case "used":
				slice = metrics.Used
			case "total":
				slice = metrics.Total
			case "avail":
				slice = metrics.Avail
			default:
				if tt.shouldAdd {
					t.Error("expected metric to be added")
				}
				return
			}

			if tt.shouldAdd {
				if len(slice) != 1 {
					t.Errorf("slice length = %d, want 1", len(slice))
				} else if slice[0].Value != tt.value {
					t.Errorf("value = %v, want %v", slice[0].Value, tt.value)
				}
			}
		})
	}
}

func TestGetGuestMetrics(t *testing.T) {
	now := time.Now()
	mh := NewMetricsHistory(100, time.Hour)

	// Add test data
	mh.AddGuestMetric("vm-100", "cpu", 10.0, now.Add(-50*time.Minute))
	mh.AddGuestMetric("vm-100", "cpu", 20.0, now.Add(-40*time.Minute))
	mh.AddGuestMetric("vm-100", "cpu", 30.0, now.Add(-30*time.Minute))
	mh.AddGuestMetric("vm-100", "cpu", 40.0, now.Add(-20*time.Minute))
	mh.AddGuestMetric("vm-100", "cpu", 50.0, now.Add(-10*time.Minute))
	mh.AddGuestMetric("vm-100", "memory", 80.0, now.Add(-5*time.Minute))

	tests := []struct {
		name       string
		guestID    string
		metricType string
		duration   time.Duration
		wantLen    int
		wantFirst  float64
	}{
		{
			name:       "get all cpu within hour",
			guestID:    "vm-100",
			metricType: "cpu",
			duration:   time.Hour,
			wantLen:    5,
			wantFirst:  10.0,
		},
		{
			name:       "get recent cpu only",
			guestID:    "vm-100",
			metricType: "cpu",
			duration:   25 * time.Minute,
			wantLen:    2,
			wantFirst:  40.0,
		},
		{
			name:       "get memory",
			guestID:    "vm-100",
			metricType: "memory",
			duration:   time.Hour,
			wantLen:    1,
			wantFirst:  80.0,
		},
		{
			name:       "nonexistent guest",
			guestID:    "vm-999",
			metricType: "cpu",
			duration:   time.Hour,
			wantLen:    0,
		},
		{
			name:       "nonexistent metric type",
			guestID:    "vm-100",
			metricType: "invalid",
			duration:   time.Hour,
			wantLen:    0,
		},
		{
			name:       "zero duration returns nothing",
			guestID:    "vm-100",
			metricType: "cpu",
			duration:   0,
			wantLen:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mh.GetGuestMetrics(tt.guestID, tt.metricType, tt.duration)

			if len(result) != tt.wantLen {
				t.Errorf("len(result) = %d, want %d", len(result), tt.wantLen)
			}
			if len(result) > 0 && tt.wantFirst != 0 {
				if result[0].Value != tt.wantFirst {
					t.Errorf("first value = %v, want %v", result[0].Value, tt.wantFirst)
				}
			}
		})
	}
}

func TestGetGuestMetrics_AllMetricTypes(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name       string
		metricType string
		value      float64
	}{
		{name: "cpu metric", metricType: "cpu", value: 50.5},
		{name: "memory metric", metricType: "memory", value: 70.0},
		{name: "disk metric", metricType: "disk", value: 45.0},
		{name: "diskread metric", metricType: "diskread", value: 1024.0},
		{name: "diskwrite metric", metricType: "diskwrite", value: 512.0},
		{name: "netin metric", metricType: "netin", value: 2048.0},
		{name: "netout metric", metricType: "netout", value: 1536.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mh := NewMetricsHistory(100, time.Hour)
			mh.AddGuestMetric("vm-test", tt.metricType, tt.value, now.Add(-5*time.Minute))

			result := mh.GetGuestMetrics("vm-test", tt.metricType, time.Hour)

			if len(result) != 1 {
				t.Fatalf("len(result) = %d, want 1", len(result))
			}
			if result[0].Value != tt.value {
				t.Errorf("value = %v, want %v", result[0].Value, tt.value)
			}
		})
	}
}

func TestGetGuestMetrics_GuestNotFound(t *testing.T) {
	mh := NewMetricsHistory(100, time.Hour)

	// Add data for one guest
	mh.AddGuestMetric("vm-100", "cpu", 50.0, time.Now())

	// Query non-existent guest
	result := mh.GetGuestMetrics("vm-nonexistent", "cpu", time.Hour)

	if len(result) != 0 {
		t.Errorf("expected empty slice for non-existent guest, got %d elements", len(result))
	}
	if result == nil {
		t.Error("expected empty slice, got nil")
	}
}

func TestGetGuestMetrics_UnknownMetricType(t *testing.T) {
	now := time.Now()
	mh := NewMetricsHistory(100, time.Hour)

	// Add data for the guest
	mh.AddGuestMetric("vm-100", "cpu", 50.0, now.Add(-5*time.Minute))
	mh.AddGuestMetric("vm-100", "memory", 70.0, now.Add(-5*time.Minute))

	unknownTypes := []string{"invalid", "unknown", "foo", "bar", "CPU", "Memory", ""}

	for _, metricType := range unknownTypes {
		t.Run("type_"+metricType, func(t *testing.T) {
			result := mh.GetGuestMetrics("vm-100", metricType, time.Hour)

			if len(result) != 0 {
				t.Errorf("expected empty slice for unknown metric type %q, got %d elements", metricType, len(result))
			}
			if result == nil {
				t.Errorf("expected empty slice for unknown metric type %q, got nil", metricType)
			}
		})
	}
}

func TestGetGuestMetrics_DurationFiltering(t *testing.T) {
	now := time.Now()
	mh := NewMetricsHistory(100, 2*time.Hour)

	// Add points at different times
	mh.AddGuestMetric("vm-100", "cpu", 10.0, now.Add(-90*time.Minute)) // old
	mh.AddGuestMetric("vm-100", "cpu", 20.0, now.Add(-60*time.Minute)) // old
	mh.AddGuestMetric("vm-100", "cpu", 30.0, now.Add(-30*time.Minute)) // recent
	mh.AddGuestMetric("vm-100", "cpu", 40.0, now.Add(-15*time.Minute)) // recent
	mh.AddGuestMetric("vm-100", "cpu", 50.0, now.Add(-5*time.Minute))  // recent

	tests := []struct {
		name      string
		duration  time.Duration
		wantLen   int
		wantFirst float64
		wantLast  float64
	}{
		{
			name:      "all points within 2 hours",
			duration:  2 * time.Hour,
			wantLen:   5,
			wantFirst: 10.0,
			wantLast:  50.0,
		},
		{
			name:      "points within 45 minutes",
			duration:  45 * time.Minute,
			wantLen:   3,
			wantFirst: 30.0,
			wantLast:  50.0,
		},
		{
			name:      "points within 20 minutes",
			duration:  20 * time.Minute,
			wantLen:   2,
			wantFirst: 40.0,
			wantLast:  50.0,
		},
		{
			name:      "points within 10 minutes",
			duration:  10 * time.Minute,
			wantLen:   1,
			wantFirst: 50.0,
			wantLast:  50.0,
		},
		{
			name:     "zero duration excludes all",
			duration: 0,
			wantLen:  0,
		},
		{
			name:     "very short duration excludes all",
			duration: 1 * time.Minute,
			wantLen:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mh.GetGuestMetrics("vm-100", "cpu", tt.duration)

			if len(result) != tt.wantLen {
				t.Errorf("len(result) = %d, want %d", len(result), tt.wantLen)
			}
			if len(result) > 0 {
				if result[0].Value != tt.wantFirst {
					t.Errorf("first value = %v, want %v", result[0].Value, tt.wantFirst)
				}
				if result[len(result)-1].Value != tt.wantLast {
					t.Errorf("last value = %v, want %v", result[len(result)-1].Value, tt.wantLast)
				}
			}
		})
	}
}

func TestGetGuestMetrics_EmptyMetricsData(t *testing.T) {
	mh := NewMetricsHistory(100, time.Hour)

	// Directly populate an empty guest metrics entry
	mh.mu.Lock()
	mh.guestMetrics["vm-empty"] = &GuestMetrics{
		CPU:        []MetricPoint{},
		Memory:     []MetricPoint{},
		Disk:       []MetricPoint{},
		DiskRead:   []MetricPoint{},
		DiskWrite:  []MetricPoint{},
		NetworkIn:  []MetricPoint{},
		NetworkOut: []MetricPoint{},
	}
	mh.mu.Unlock()

	metricTypes := []string{"cpu", "memory", "disk", "diskread", "diskwrite", "netin", "netout"}

	for _, metricType := range metricTypes {
		t.Run(metricType, func(t *testing.T) {
			result := mh.GetGuestMetrics("vm-empty", metricType, time.Hour)

			if len(result) != 0 {
				t.Errorf("expected empty slice for empty %s metrics, got %d elements", metricType, len(result))
			}
			if result == nil {
				t.Errorf("expected empty slice for empty %s metrics, got nil", metricType)
			}
		})
	}
}

func TestGetNodeMetrics(t *testing.T) {
	now := time.Now()
	mh := NewMetricsHistory(100, time.Hour)

	// Add test data
	mh.AddNodeMetric("node1", "cpu", 25.0, now.Add(-45*time.Minute))
	mh.AddNodeMetric("node1", "cpu", 35.0, now.Add(-30*time.Minute))
	mh.AddNodeMetric("node1", "cpu", 45.0, now.Add(-15*time.Minute))
	mh.AddNodeMetric("node1", "memory", 60.0, now.Add(-10*time.Minute))

	tests := []struct {
		name       string
		nodeID     string
		metricType string
		duration   time.Duration
		wantLen    int
	}{
		{
			name:       "get all cpu",
			nodeID:     "node1",
			metricType: "cpu",
			duration:   time.Hour,
			wantLen:    3,
		},
		{
			name:       "get recent cpu",
			nodeID:     "node1",
			metricType: "cpu",
			duration:   20 * time.Minute,
			wantLen:    1,
		},
		{
			name:       "get memory",
			nodeID:     "node1",
			metricType: "memory",
			duration:   time.Hour,
			wantLen:    1,
		},
		{
			name:       "nonexistent node",
			nodeID:     "node999",
			metricType: "cpu",
			duration:   time.Hour,
			wantLen:    0,
		},
		{
			name:       "invalid metric type",
			nodeID:     "node1",
			metricType: "netin",
			duration:   time.Hour,
			wantLen:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mh.GetNodeMetrics(tt.nodeID, tt.metricType, tt.duration)
			if len(result) != tt.wantLen {
				t.Errorf("len(result) = %d, want %d", len(result), tt.wantLen)
			}
		})
	}
}

func TestGetAllGuestMetrics(t *testing.T) {
	now := time.Now()
	mh := NewMetricsHistory(100, time.Hour)

	// Add test data for multiple metric types
	mh.AddGuestMetric("vm-100", "cpu", 50.0, now.Add(-30*time.Minute))
	mh.AddGuestMetric("vm-100", "memory", 70.0, now.Add(-25*time.Minute))
	mh.AddGuestMetric("vm-100", "disk", 40.0, now.Add(-20*time.Minute))
	mh.AddGuestMetric("vm-100", "diskread", 100.0, now.Add(-15*time.Minute))
	mh.AddGuestMetric("vm-100", "diskwrite", 50.0, now.Add(-10*time.Minute))
	mh.AddGuestMetric("vm-100", "netin", 1000.0, now.Add(-5*time.Minute))
	mh.AddGuestMetric("vm-100", "netout", 500.0, now.Add(-1*time.Minute))

	tests := []struct {
		name     string
		guestID  string
		duration time.Duration
		wantKeys []string
	}{
		{
			name:     "get all metrics within hour",
			guestID:  "vm-100",
			duration: time.Hour,
			wantKeys: []string{"cpu", "memory", "disk", "diskread", "diskwrite", "netin", "netout"},
		},
		{
			name:     "nonexistent guest",
			guestID:  "vm-999",
			duration: time.Hour,
			wantKeys: []string{},
		},
		{
			name:     "short duration filters out old",
			guestID:  "vm-100",
			duration: 10 * time.Minute,
			wantKeys: []string{"cpu", "memory", "disk", "diskread", "diskwrite", "netin", "netout"}, // keys exist but may be empty
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mh.GetAllGuestMetrics(tt.guestID, tt.duration)

			if tt.guestID == "vm-999" {
				if len(result) != 0 {
					t.Errorf("expected empty result for nonexistent guest")
				}
				return
			}

			// Check that expected keys exist
			for _, key := range tt.wantKeys {
				if _, exists := result[key]; !exists {
					t.Errorf("missing key: %s", key)
				}
			}
		})
	}
}

func TestGetAllStorageMetrics(t *testing.T) {
	now := time.Now()
	mh := NewMetricsHistory(100, time.Hour)

	// Add test data
	mh.AddStorageMetric("local-lvm", "usage", 45.0, now.Add(-30*time.Minute))
	mh.AddStorageMetric("local-lvm", "used", 100e9, now.Add(-25*time.Minute))
	mh.AddStorageMetric("local-lvm", "total", 200e9, now.Add(-20*time.Minute))
	mh.AddStorageMetric("local-lvm", "avail", 100e9, now.Add(-15*time.Minute))

	tests := []struct {
		name      string
		storageID string
		duration  time.Duration
		wantKeys  []string
	}{
		{
			name:      "get all metrics",
			storageID: "local-lvm",
			duration:  time.Hour,
			wantKeys:  []string{"usage", "used", "total", "avail"},
		},
		{
			name:      "nonexistent storage",
			storageID: "nonexistent",
			duration:  time.Hour,
			wantKeys:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mh.GetAllStorageMetrics(tt.storageID, tt.duration)

			if len(tt.wantKeys) == 0 {
				if len(result) != 0 {
					t.Errorf("expected empty result")
				}
				return
			}

			for _, key := range tt.wantKeys {
				if _, exists := result[key]; !exists {
					t.Errorf("missing key: %s", key)
				}
			}
		})
	}
}

func TestCleanup(t *testing.T) {
	now := time.Now()
	retentionTime := 30 * time.Minute
	mh := NewMetricsHistory(100, retentionTime)

	// Add old data (beyond retention)
	mh.AddGuestMetric("vm-100", "cpu", 10.0, now.Add(-2*time.Hour))
	mh.AddGuestMetric("vm-100", "cpu", 20.0, now.Add(-90*time.Minute))

	// Add recent data (within retention)
	mh.AddGuestMetric("vm-100", "cpu", 30.0, now.Add(-20*time.Minute))
	mh.AddGuestMetric("vm-100", "cpu", 40.0, now.Add(-10*time.Minute))

	// Add node metrics
	mh.AddNodeMetric("node1", "cpu", 50.0, now.Add(-2*time.Hour))
	mh.AddNodeMetric("node1", "cpu", 60.0, now.Add(-10*time.Minute))

	// Add storage metrics
	mh.AddStorageMetric("local", "usage", 70.0, now.Add(-2*time.Hour))
	mh.AddStorageMetric("local", "usage", 80.0, now.Add(-10*time.Minute))

	// Run cleanup
	mh.Cleanup()

	// Verify old data was removed
	mh.mu.RLock()
	defer mh.mu.RUnlock()

	// Check guest metrics
	guestMetrics := mh.guestMetrics["vm-100"]
	if len(guestMetrics.CPU) != 2 {
		t.Errorf("guest CPU metrics count = %d, want 2", len(guestMetrics.CPU))
	}

	// Check node metrics
	nodeMetrics := mh.nodeMetrics["node1"]
	if len(nodeMetrics.CPU) != 1 {
		t.Errorf("node CPU metrics count = %d, want 1", len(nodeMetrics.CPU))
	}

	// Check storage metrics
	storageMetrics := mh.storageMetrics["local"]
	if len(storageMetrics.Usage) != 1 {
		t.Errorf("storage usage metrics count = %d, want 1", len(storageMetrics.Usage))
	}
}

func TestMultipleGuestsIsolation(t *testing.T) {
	now := time.Now()
	mh := NewMetricsHistory(100, time.Hour)

	// Add metrics for different guests
	mh.AddGuestMetric("vm-100", "cpu", 50.0, now)
	mh.AddGuestMetric("vm-101", "cpu", 60.0, now)
	mh.AddGuestMetric("vm-102", "cpu", 70.0, now)

	// Verify isolation
	result100 := mh.GetGuestMetrics("vm-100", "cpu", time.Hour)
	result101 := mh.GetGuestMetrics("vm-101", "cpu", time.Hour)
	result102 := mh.GetGuestMetrics("vm-102", "cpu", time.Hour)

	if len(result100) != 1 || result100[0].Value != 50.0 {
		t.Errorf("vm-100 metrics incorrect")
	}
	if len(result101) != 1 || result101[0].Value != 60.0 {
		t.Errorf("vm-101 metrics incorrect")
	}
	if len(result102) != 1 || result102[0].Value != 70.0 {
		t.Errorf("vm-102 metrics incorrect")
	}
}

func TestMaxDataPointsEnforced(t *testing.T) {
	now := time.Now()
	maxPoints := 5
	mh := NewMetricsHistory(maxPoints, time.Hour)

	// Add more points than maxDataPoints
	for i := 0; i < 10; i++ {
		mh.AddGuestMetric("vm-100", "cpu", float64(i*10), now.Add(-time.Duration(10-i)*time.Minute))
	}

	result := mh.GetGuestMetrics("vm-100", "cpu", time.Hour)
	if len(result) != maxPoints {
		t.Errorf("got %d points, want %d", len(result), maxPoints)
	}

	// Verify we have the most recent points (values 50-90)
	if result[0].Value != 50.0 {
		t.Errorf("first value = %v, want 50.0 (most recent kept)", result[0].Value)
	}
	if result[len(result)-1].Value != 90.0 {
		t.Errorf("last value = %v, want 90.0", result[len(result)-1].Value)
	}
}

func TestRetentionTimeEnforced(t *testing.T) {
	now := time.Now()
	retentionTime := 30 * time.Minute
	mh := NewMetricsHistory(100, retentionTime)

	// Add points at various times
	mh.AddGuestMetric("vm-100", "cpu", 10.0, now.Add(-2*time.Hour))   // beyond retention
	mh.AddGuestMetric("vm-100", "cpu", 20.0, now.Add(-1*time.Hour))   // beyond retention
	mh.AddGuestMetric("vm-100", "cpu", 30.0, now.Add(-45*time.Minute)) // beyond retention
	mh.AddGuestMetric("vm-100", "cpu", 40.0, now.Add(-20*time.Minute)) // within retention
	mh.AddGuestMetric("vm-100", "cpu", 50.0, now.Add(-10*time.Minute)) // within retention
	mh.AddGuestMetric("vm-100", "cpu", 60.0, now)                       // within retention

	// appendMetric removes old points on each add
	result := mh.GetGuestMetrics("vm-100", "cpu", time.Hour)

	// Due to appendMetric behavior, only recent points should remain
	if len(result) > 3 {
		t.Errorf("got %d points, expected <= 3 recent points after retention enforcement", len(result))
	}
}
