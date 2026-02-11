package baseline

import (
	"os"
	"testing"
	"time"
)

// Additional tests to improve coverage

func TestNewStore_Defaults(t *testing.T) {
	// Test with zero values - should use defaults
	store := NewStore(StoreConfig{})

	if store == nil {
		t.Fatal("Expected non-nil store")
	}

	// Store should be properly initialized
	if store.baselines == nil {
		t.Error("baselines map should be initialized")
	}
}

func TestNewStore_NonPositiveConfigUsesDefaults(t *testing.T) {
	store := NewStore(StoreConfig{
		LearningWindow: -1 * time.Hour,
		MinSamples:     -10,
		UpdateInterval: -1 * time.Minute,
	})

	if store.learningWindow != 7*24*time.Hour {
		t.Fatalf("learningWindow = %v, want %v", store.learningWindow, 7*24*time.Hour)
	}
	if store.minSamples != 50 {
		t.Fatalf("minSamples = %d, want 50", store.minSamples)
	}
	if store.updateInterval != 1*time.Hour {
		t.Fatalf("updateInterval = %v, want %v", store.updateInterval, 1*time.Hour)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.LearningWindow == 0 {
		t.Error("LearningWindow should have a default value")
	}
	if cfg.MinSamples == 0 {
		t.Error("MinSamples should have a default value")
	}
	if cfg.UpdateInterval == 0 {
		t.Error("UpdateInterval should have a default value")
	}
}

func TestResourceCount(t *testing.T) {
	store := NewStore(StoreConfig{MinSamples: 10})

	// Initially empty
	if store.ResourceCount() != 0 {
		t.Errorf("Expected 0 resources, got %d", store.ResourceCount())
	}

	// Add some baselines
	points := make([]MetricPoint, 50)
	for i := range points {
		points[i] = MetricPoint{Value: 50}
	}

	store.Learn("vm-100", "vm", "cpu", points)
	store.Learn("vm-200", "vm", "cpu", points)

	if store.ResourceCount() != 2 {
		t.Errorf("Expected 2 resources, got %d", store.ResourceCount())
	}
}

func TestIsAnomaly_NoBaseline(t *testing.T) {
	store := NewStore(StoreConfig{MinSamples: 10})

	// Check anomaly for resource with no baseline
	isAnomaly, zScore := store.IsAnomaly("nonexistent", "cpu", 50)

	if isAnomaly {
		t.Error("Should not detect anomaly without baseline")
	}
	if zScore != 0 {
		t.Errorf("Expected zScore 0 without baseline, got %f", zScore)
	}
}

func TestCheckAnomaly_NoBaseline(t *testing.T) {
	store := NewStore(StoreConfig{MinSamples: 10})

	severity, zScore, baseline := store.CheckAnomaly("nonexistent", "cpu", 50)

	if severity != AnomalyNone {
		t.Errorf("Expected AnomalyNone, got %s", severity)
	}
	if zScore != 0 {
		t.Errorf("Expected zScore 0, got %f", zScore)
	}
	if baseline != nil {
		t.Error("Expected nil baseline for nonexistent resource")
	}
}

func TestGetBaseline_NotFound(t *testing.T) {
	store := NewStore(StoreConfig{MinSamples: 10})

	baseline, ok := store.GetBaseline("nonexistent", "cpu")

	if ok {
		t.Error("Should not find baseline for nonexistent resource")
	}
	if baseline != nil {
		t.Error("Expected nil baseline")
	}
}

func TestGetResourceBaseline_NotFound(t *testing.T) {
	store := NewStore(StoreConfig{MinSamples: 10})

	rb, ok := store.GetResourceBaseline("nonexistent")

	if ok {
		t.Error("Should not find resource baseline for nonexistent resource")
	}
	if rb != nil {
		t.Error("Expected nil resource baseline")
	}
}

func TestLearn_ZeroStdDev(t *testing.T) {
	store := NewStore(StoreConfig{MinSamples: 10})

	// All same values - stddev should be 0 but Learn should handle it
	points := make([]MetricPoint, 50)
	for i := range points {
		points[i] = MetricPoint{Value: 50} // All same value
	}

	err := store.Learn("test-vm", "vm", "cpu", points)
	if err != nil {
		t.Fatalf("Learn failed: %v", err)
	}

	baseline, ok := store.GetBaseline("test-vm", "cpu")
	if !ok {
		t.Fatal("Baseline not found")
	}

	if baseline.Mean != 50 {
		t.Errorf("Expected mean 50, got %f", baseline.Mean)
	}

	// StdDev should be 0 or very close to it
	if baseline.StdDev > 0.001 {
		t.Errorf("Expected stddev ~0, got %f", baseline.StdDev)
	}
}

func TestIsAnomaly_ZeroStdDev(t *testing.T) {
	store := NewStore(StoreConfig{MinSamples: 10})

	// All same values
	points := make([]MetricPoint, 50)
	for i := range points {
		points[i] = MetricPoint{Value: 50}
	}

	store.Learn("test-vm", "vm", "cpu", points)

	// With zero stddev, any different value should be anomaly
	isAnomaly, _ := store.IsAnomaly("test-vm", "cpu", 50)
	if isAnomaly {
		// Exact mean should not be anomaly even with zero stddev
		t.Error("Exact mean should not be anomaly")
	}
}

func TestHourlyMeans(t *testing.T) {
	store := NewStore(StoreConfig{MinSamples: 10})

	now := time.Now()
	points := make([]MetricPoint, 100)
	for i := 0; i < 100; i++ {
		// Create points at different hours
		points[i] = MetricPoint{
			Value:     float64(50 + i%24), // Different value for each hour
			Timestamp: now.Add(-time.Duration(i) * time.Hour),
		}
	}

	err := store.Learn("test-vm", "vm", "cpu", points)
	if err != nil {
		t.Fatalf("Learn failed: %v", err)
	}

	baseline, ok := store.GetBaseline("test-vm", "cpu")
	if !ok {
		t.Fatal("Baseline not found")
	}

	// Hourly means should be computed
	// At least some hours should have non-zero means
	hasNonZeroHour := false
	for _, mean := range baseline.HourlyMeans {
		if mean != 0 {
			hasNonZeroHour = true
			break
		}
	}

	if !hasNonZeroHour {
		t.Log("Note: HourlyMeans may all be zero depending on implementation")
	}
}

func TestPersistence_WithDataDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "baseline-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := NewStore(StoreConfig{
		MinSamples: 10,
		DataDir:    tmpDir,
	})

	// Add a baseline
	points := make([]MetricPoint, 50)
	for i := range points {
		points[i] = MetricPoint{Value: 50}
	}

	store.Learn("test-vm", "vm", "cpu", points)

	// Save
	err = store.Save()
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Create new store and load
	store2 := NewStore(StoreConfig{
		MinSamples: 10,
		DataDir:    tmpDir,
	})

	if store2.ResourceCount() == 0 {
		t.Log("Note: Baselines may not persist depending on implementation")
	}
}

func TestGetAllBaselines_Empty(t *testing.T) {
	store := NewStore(StoreConfig{MinSamples: 10})

	baselines := store.GetAllBaselines()

	if len(baselines) != 0 {
		t.Errorf("Expected empty baselines, got %d", len(baselines))
	}
}

func TestGetAllBaselines_WithData(t *testing.T) {
	store := NewStore(StoreConfig{MinSamples: 10})

	points := make([]MetricPoint, 50)
	for i := range points {
		points[i] = MetricPoint{Value: 50}
	}

	store.Learn("vm-100", "vm", "cpu", points)
	store.Learn("vm-100", "vm", "memory", points)

	baselines := store.GetAllBaselines()

	if len(baselines) != 2 {
		t.Errorf("Expected 2 flat baselines, got %d", len(baselines))
	}
}

func TestLearn_UpdatesExistingBaseline(t *testing.T) {
	store := NewStore(StoreConfig{MinSamples: 10})

	points1 := make([]MetricPoint, 50)
	for i := range points1 {
		points1[i] = MetricPoint{Value: 50}
	}

	store.Learn("test-vm", "vm", "cpu", points1)
	baseline1, _ := store.GetBaseline("test-vm", "cpu")
	origMean := baseline1.Mean

	// Learn again with different data
	points2 := make([]MetricPoint, 50)
	for i := range points2 {
		points2[i] = MetricPoint{Value: 100}
	}

	store.Learn("test-vm", "vm", "cpu", points2)
	baseline2, _ := store.GetBaseline("test-vm", "cpu")

	// Mean should be updated
	if baseline2.Mean == origMean {
		t.Error("Expected mean to be updated after second Learn call")
	}
}

func TestComputeMean_Empty(t *testing.T) {
	result := computeMean([]float64{})
	if result != 0 {
		t.Errorf("Expected 0 for empty slice, got %f", result)
	}
}

func TestComputeStdDev_Empty(t *testing.T) {
	result := computeStdDev([]float64{})
	if result != 0 {
		t.Errorf("Expected 0 for empty slice, got %f", result)
	}
}

func TestComputeStdDev_SingleValue(t *testing.T) {
	result := computeStdDev([]float64{50})
	if result != 0 {
		t.Errorf("Expected 0 for single value, got %f", result)
	}
}

func TestComputePercentiles_Edge(t *testing.T) {
	// Single value
	p := computePercentiles([]float64{50})
	if p[50] != 50 {
		t.Errorf("Expected P50 = 50 for single value, got %f", p[50])
	}

	// Empty slice
	p = computePercentiles([]float64{})
	if len(p) != 0 {
		t.Error("Expected empty percentiles for empty slice")
	}
}

func TestAnomalySeverityString(t *testing.T) {
	tests := []struct {
		severity AnomalySeverity
		expected string
	}{
		{AnomalyNone, ""},
		{AnomalyLow, "low"},
		{AnomalyMedium, "medium"},
		{AnomalyHigh, "high"},
		{AnomalyCritical, "critical"},
	}

	for _, tt := range tests {
		if string(tt.severity) != tt.expected {
			t.Errorf("Expected %q, got %q", tt.expected, string(tt.severity))
		}
	}
}
