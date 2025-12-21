package baseline

import (
	"math"
	"testing"
	"time"
)

func TestLearn_Basic(t *testing.T) {
	store := NewStore(StoreConfig{MinSamples: 10})
	
	// Create 50 data points with mean ~50 and some variance
	points := make([]MetricPoint, 50)
	now := time.Now()
	for i := 0; i < 50; i++ {
		points[i] = MetricPoint{
			Value:     50 + float64(i%10) - 5, // Values from 45-54
			Timestamp: now.Add(-time.Duration(50-i) * time.Minute),
		}
	}
	
	err := store.Learn("test-vm", "vm", "cpu", points)
	if err != nil {
		t.Fatalf("Learn failed: %v", err)
	}
	
	baseline, ok := store.GetBaseline("test-vm", "cpu")
	if !ok {
		t.Fatal("Baseline not found after learning")
	}
	
	// Check mean is around 50
	if math.Abs(baseline.Mean-50) > 1 {
		t.Errorf("Expected mean ~50, got %f", baseline.Mean)
	}
	
	// Check stddev is reasonable (should be ~3 for our data)
	if baseline.StdDev < 1 || baseline.StdDev > 5 {
		t.Errorf("Expected stddev ~3, got %f", baseline.StdDev)
	}
	
	if baseline.SampleCount != 50 {
		t.Errorf("Expected 50 samples, got %d", baseline.SampleCount)
	}
}

func TestLearn_InsufficientData(t *testing.T) {
	store := NewStore(StoreConfig{MinSamples: 50})
	
	// Only 10 points, not enough
	points := make([]MetricPoint, 10)
	for i := 0; i < 10; i++ {
		points[i] = MetricPoint{Value: float64(i)}
	}
	
	err := store.Learn("test-vm", "vm", "cpu", points)
	if err != nil {
		t.Fatalf("Learn should not error on insufficient data: %v", err)
	}
	
	_, ok := store.GetBaseline("test-vm", "cpu")
	if ok {
		t.Error("Should not have baseline with insufficient data")
	}
}

func TestIsAnomaly(t *testing.T) {
	store := NewStore(StoreConfig{MinSamples: 10})
	
	// Create stable data around 50 with low variance
	points := make([]MetricPoint, 100)
	for i := 0; i < 100; i++ {
		points[i] = MetricPoint{
			Value: 50 + float64(i%3) - 1, // Values 49, 50, 51
		}
	}
	
	store.Learn("test-vm", "vm", "cpu", points)
	
	// Test normal value
	isAnomaly, zScore := store.IsAnomaly("test-vm", "cpu", 50)
	if isAnomaly {
		t.Errorf("50 should not be anomaly, zScore=%f", zScore)
	}
	
	// Test slightly high - with stddev ~0.82, 51 is within 2 std devs
	isAnomaly, zScore = store.IsAnomaly("test-vm", "cpu", 51)
	if isAnomaly {
		t.Errorf("51 should not be anomaly with this variance, zScore=%f", zScore)
	}
	
	// Test very high (should be anomaly)
	isAnomaly, zScore = store.IsAnomaly("test-vm", "cpu", 60)
	if !isAnomaly {
		t.Errorf("60 should be anomaly, zScore=%f", zScore)
	}
	
	// Test very low (should be anomaly)
	isAnomaly, zScore = store.IsAnomaly("test-vm", "cpu", 40)
	if !isAnomaly {
		t.Errorf("40 should be anomaly, zScore=%f", zScore)
	}
}

func TestCheckAnomaly_Severity(t *testing.T) {
	store := NewStore(StoreConfig{MinSamples: 10})
	
	// Create very stable data with known statistics
	// Mean = 50, StdDev = 1
	points := make([]MetricPoint, 100)
	for i := 0; i < 100; i++ {
		// Alternate between 49, 50, 51 for stddev ~1
		points[i] = MetricPoint{Value: 50 + float64(i%3) - 1}
	}
	
	store.Learn("test-vm", "vm", "cpu", points)
	baseline, _ := store.GetBaseline("test-vm", "cpu")
	
	testCases := []struct {
		value            float64
		expectedSeverity AnomalySeverity
	}{
		{50, AnomalyNone},                     // Mean
		{50 + baseline.StdDev*1.5, AnomalyNone}, // 1.5 std devs - normal
		{50 + baseline.StdDev*2.2, AnomalyLow},  // 2.2 std devs
		{50 + baseline.StdDev*2.7, AnomalyMedium}, // 2.7 std devs
		{50 + baseline.StdDev*3.5, AnomalyHigh},   // 3.5 std devs
		{50 + baseline.StdDev*4.5, AnomalyCritical}, // 4.5 std devs
	}
	
	for _, tc := range testCases {
		severity, _, _ := store.CheckAnomaly("test-vm", "cpu", tc.value)
		if severity != tc.expectedSeverity {
			t.Errorf("Value %f: expected severity %s, got %s", tc.value, tc.expectedSeverity, severity)
		}
	}
}

func TestGetResourceBaseline(t *testing.T) {
	store := NewStore(StoreConfig{MinSamples: 10})
	
	// Learn multiple metrics
	cpuPoints := make([]MetricPoint, 50)
	memPoints := make([]MetricPoint, 50)
	for i := 0; i < 50; i++ {
		cpuPoints[i] = MetricPoint{Value: 30}
		memPoints[i] = MetricPoint{Value: 70}
	}
	
	store.Learn("test-vm", "vm", "cpu", cpuPoints)
	store.Learn("test-vm", "vm", "memory", memPoints)
	
	rb, ok := store.GetResourceBaseline("test-vm")
	if !ok {
		t.Fatal("Resource baseline not found")
	}
	
	if rb.ResourceType != "vm" {
		t.Errorf("Expected resource type 'vm', got '%s'", rb.ResourceType)
	}
	
	if len(rb.Metrics) != 2 {
		t.Errorf("Expected 2 metrics, got %d", len(rb.Metrics))
	}
	
	if rb.Metrics["cpu"] == nil {
		t.Error("CPU metric baseline missing")
	}
	
	if rb.Metrics["memory"] == nil {
		t.Error("Memory metric baseline missing")
	}
}

func TestPercentiles(t *testing.T) {
	values := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	percentiles := computePercentiles(values)
	
	// P50 should be ~5.5 for 1-10
	if percentiles[50] < 5 || percentiles[50] > 6 {
		t.Errorf("P50 should be ~5.5, got %f", percentiles[50])
	}
	
	// P5 should be close to 1
	if percentiles[5] < 1 || percentiles[5] > 2 {
		t.Errorf("P5 should be ~1, got %f", percentiles[5])
	}
	
	// P95 should be close to 10
	if percentiles[95] < 9 || percentiles[95] > 10 {
		t.Errorf("P95 should be ~10, got %f", percentiles[95])
	}
}

func TestComputeStats(t *testing.T) {
	// Test mean and stddev with known values
	values := []float64{2, 4, 4, 4, 5, 5, 7, 9} // Mean = 5, Stddev = 2 (sample)
	
	mean := computeMean(values)
	if mean != 5 {
		t.Errorf("Expected mean 5, got %f", mean)
	}
	
	stddev := computeStdDev(values)
	// Sample stddev of [2,4,4,4,5,5,7,9] is approximately 2.14, not exactly 2
	if math.Abs(stddev-2.14) > 0.1 {
		t.Errorf("Expected stddev ~2.14, got %f", stddev)
	}
}

func TestCalculateTrend_InsufficientData(t *testing.T) {
	// Less than 5 samples should return nil
	samples := []float64{10, 20, 30}
	result := CalculateTrend(samples, 30)
	if result != nil {
		t.Error("Expected nil for insufficient data")
	}
}

func TestCalculateTrend_IncreasingTrend(t *testing.T) {
	// Simulate hourly samples increasing by 1% per hour
	// 24 samples = 1 day, so 24% increase per day
	samples := make([]float64, 48) // 2 days of data
	for i := 0; i < 48; i++ {
		samples[i] = 50 + float64(i) // 50, 51, 52, ...
	}
	
	result := CalculateTrend(samples, 97) // Currently at 97%
	if result == nil {
		t.Fatal("Expected non-nil result for increasing trend")
	}
	
	// Should be trending toward full
	if result.DaysToFull <= 0 {
		t.Errorf("Expected positive DaysToFull for increasing trend, got %d", result.DaysToFull)
	}
	
	// With 24% increase per day and 3% remaining, should be full very soon
	if result.Severity != "critical" && result.Severity != "warning" {
		t.Errorf("Expected critical or warning severity, got %s", result.Severity)
	}
}

func TestCalculateTrend_DecreasingTrend(t *testing.T) {
	// Simulate hourly samples decreasing
	samples := make([]float64, 48)
	for i := 0; i < 48; i++ {
		samples[i] = 80 - float64(i)*0.5 // 80, 79.5, 79, ...
	}
	
	result := CalculateTrend(samples, 56)
	if result == nil {
		t.Fatal("Expected non-nil result for decreasing trend")
	}
	
	// Should indicate decreasing (DaysToFull = -1)
	if result.DaysToFull != -1 {
		t.Errorf("Expected DaysToFull=-1 for decreasing trend, got %d", result.DaysToFull)
	}
	
	if result.Severity != "info" {
		t.Errorf("Expected info severity for decreasing trend, got %s", result.Severity)
	}
}

func TestCalculateTrend_StableTrend(t *testing.T) {
	// Simulate stable usage around 50%
	samples := make([]float64, 48)
	for i := 0; i < 48; i++ {
		samples[i] = 50 + float64(i%3-1)*0.01 // Tiny fluctuations
	}
	
	result := CalculateTrend(samples, 50)
	if result == nil {
		t.Fatal("Expected non-nil result for stable trend")
	}
	
	// Should indicate stable (DaysToFull = -1)
	if result.DaysToFull != -1 {
		t.Errorf("Expected DaysToFull=-1 for stable trend, got %d", result.DaysToFull)
	}
}

func TestFormatDays(t *testing.T) {
	testCases := []struct {
		days     int
		expected string
	}{
		{0, "now"},
		{1, "1 day"},
		{5, "5 days"},
		{7, "~1 week"},
		{14, "~2 weeks"},
		{30, "~1 month"},
		{60, "~2 months"},
		{400, ">1 year"},
	}
	
	for _, tc := range testCases {
		result := formatDays(tc.days)
		if result != tc.expected {
			t.Errorf("formatDays(%d): expected %q, got %q", tc.days, tc.expected, result)
		}
	}
}

func TestCheckResourceAnomalies_Disk(t *testing.T) {
	store := NewStore(StoreConfig{MinSamples: 10})
	
	// Create stable data with mean ~60% disk usage
	points := make([]MetricPoint, 100)
	for i := 0; i < 100; i++ {
		points[i] = MetricPoint{Value: 60 + float64(i%5) - 2} // 58-62
	}
	store.Learn("test-vm", "vm", "disk", points)
	
	// Test: disk above 85% should be reported
	metrics := map[string]float64{"disk": 90}
	anomalies := store.CheckResourceAnomalies("test-vm", metrics)
	if len(anomalies) == 0 {
		t.Error("Expected disk anomaly to be reported for 90% usage")
	}
	
	// Test: disk increase >15 points from baseline should be reported
	metrics = map[string]float64{"disk": 80}
	anomalies = store.CheckResourceAnomalies("test-vm", metrics)
	if len(anomalies) == 0 {
		t.Error("Expected disk anomaly to be reported for 20 point increase from baseline")
	}
	
	// Test: disk at baseline should not be reported (no significant deviation)
	metrics = map[string]float64{"disk": 60}
	anomalies = store.CheckResourceAnomalies("test-vm", metrics)
	if len(anomalies) != 0 {
		t.Errorf("Expected no anomaly for disk at baseline, got %d", len(anomalies))
	}
}

func TestCheckResourceAnomalies_CPU(t *testing.T) {
	store := NewStore(StoreConfig{MinSamples: 10})
	
	// Create stable data with mean ~20% CPU usage
	points := make([]MetricPoint, 100)
	for i := 0; i < 100; i++ {
		points[i] = MetricPoint{Value: 20 + float64(i%3) - 1} // 19-21
	}
	store.Learn("test-vm", "vm", "cpu", points)
	
	// Test: CPU at 80% (above 70% and >2x baseline) should be reported
	metrics := map[string]float64{"cpu": 80}
	anomalies := store.CheckResourceAnomalies("test-vm", metrics)
	if len(anomalies) == 0 {
		t.Error("Expected CPU anomaly to be reported for 80% (>70% and 4x baseline)")
	}
	
	// Test: CPU at 50% should NOT be reported (below 70% threshold)
	metrics = map[string]float64{"cpu": 50}
	anomalies = store.CheckResourceAnomalies("test-vm", metrics)
	if len(anomalies) != 0 {
		t.Errorf("Expected no anomaly for CPU at 50%%, got %d", len(anomalies))
	}
	
	// Test: CPU at 20% (baseline) should not be reported
	metrics = map[string]float64{"cpu": 20}
	anomalies = store.CheckResourceAnomalies("test-vm", metrics)
	if len(anomalies) != 0 {
		t.Errorf("Expected no anomaly for CPU at baseline, got %d", len(anomalies))
	}
}

func TestCheckResourceAnomalies_Memory(t *testing.T) {
	store := NewStore(StoreConfig{MinSamples: 10})
	
	// Create stable data with mean ~40% memory usage
	points := make([]MetricPoint, 100)
	for i := 0; i < 100; i++ {
		points[i] = MetricPoint{Value: 40 + float64(i%3) - 1} // 39-41
	}
	store.Learn("test-vm", "vm", "memory", points)
	
	// Test: Memory at 85% should be reported (above 80% threshold)
	metrics := map[string]float64{"memory": 85}
	anomalies := store.CheckResourceAnomalies("test-vm", metrics)
	if len(anomalies) == 0 {
		t.Error("Expected memory anomaly to be reported for 85% (above 80%)")
	}
	
	// Test: Memory at 70% with 1.75x baseline should be reported (>1.5x and >60%)
	metrics = map[string]float64{"memory": 70}
	anomalies = store.CheckResourceAnomalies("test-vm", metrics)
	if len(anomalies) == 0 {
		t.Error("Expected memory anomaly to be reported for 70% (1.75x baseline, >60%)")
	}
	
	// Test: Memory at 50% should NOT be reported (not >1.5x enough or >80%)
	metrics = map[string]float64{"memory": 50}
	anomalies = store.CheckResourceAnomalies("test-vm", metrics)
	if len(anomalies) != 0 {
		t.Errorf("Expected no anomaly for memory at 50%%, got %d", len(anomalies))
	}
}

func TestCheckResourceAnomalies_OtherMetrics(t *testing.T) {
	store := NewStore(StoreConfig{MinSamples: 10})
	
	// Create stable network data with mean ~100
	points := make([]MetricPoint, 100)
	for i := 0; i < 100; i++ {
		points[i] = MetricPoint{Value: 100 + float64(i%5) - 2} // 98-102
	}
	store.Learn("test-vm", "vm", "network_in", points)
	
	// Test: network_in at 2x baseline should be reported
	metrics := map[string]float64{"network_in": 250}
	anomalies := store.CheckResourceAnomalies("test-vm", metrics)
	if len(anomalies) == 0 {
		t.Error("Expected network anomaly to be reported for 2.5x baseline")
	}
	
	// Test: network_in at 0.3x baseline should be reported (below 0.5x)
	metrics = map[string]float64{"network_in": 30}
	anomalies = store.CheckResourceAnomalies("test-vm", metrics)
	if len(anomalies) == 0 {
		t.Error("Expected network anomaly to be reported for 0.3x baseline")
	}
}

func TestCheckResourceAnomalies_NoBaseline(t *testing.T) {
	store := NewStore(StoreConfig{MinSamples: 10})
	
	// No baselines learned - should return empty
	metrics := map[string]float64{"cpu": 90, "memory": 85}
	anomalies := store.CheckResourceAnomalies("unknown-vm", metrics)
	if len(anomalies) != 0 {
		t.Errorf("Expected no anomalies for unknown resource, got %d", len(anomalies))
	}
}

func TestFormatRatio(t *testing.T) {
	testCases := []struct {
		ratio    float64
		expected string
	}{
		{0.005, "near zero"},
		{0.5, "significantly below"},
		{0.8, "significantly below"},
		{1.2, "slightly above"},
		{1.4, "slightly above"},
		{1.7, "1.5x"},
		{2.5, "2x"},
		{4.0, "3x"},
		{6.0, "~6x"},
	}
	
	for _, tc := range testCases {
		result := formatRatio(tc.ratio)
		if result != tc.expected {
			t.Errorf("formatRatio(%f): expected %q, got %q", tc.ratio, tc.expected, result)
		}
	}
}

func TestFormatAnomalyDescription(t *testing.T) {
	testCases := []struct {
		metric    string
		ratio     float64
		direction string
		severity  AnomalySeverity
		contains  string
	}{
		{"cpu", 2.0, "above", AnomalyCritical, "Critical anomaly: CPU usage"},
		{"memory", 1.5, "above", AnomalyHigh, "High anomaly: Memory usage"},
		{"disk", 1.8, "above", AnomalyMedium, "Moderate anomaly: Disk usage"},
		{"network_in", 2.0, "below", AnomalyLow, "Minor anomaly: Network inbound"},
		{"network_out", 1.5, "above", AnomalyNone, "Network outbound"},
	}
	
	for _, tc := range testCases {
		result := formatAnomalyDescription(tc.metric, tc.ratio, tc.direction, tc.severity)
		if !contains(result, tc.contains) {
			t.Errorf("formatAnomalyDescription(%s, %f, %s, %s): expected to contain %q, got %q",
				tc.metric, tc.ratio, tc.direction, tc.severity, tc.contains, result)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestGetAllAnomalies(t *testing.T) {
	store := NewStore(StoreConfig{MinSamples: 10})
	
	// Learn baselines for multiple resources
	points := make([]MetricPoint, 100)
	for i := 0; i < 100; i++ {
		points[i] = MetricPoint{Value: 20 + float64(i%3) - 1}
	}
	store.Learn("vm-1", "vm", "cpu", points)
	store.Learn("vm-2", "vm", "cpu", points)
	
	diskPoints := make([]MetricPoint, 100)
	for i := 0; i < 100; i++ {
		diskPoints[i] = MetricPoint{Value: 50 + float64(i%3) - 1}
	}
	store.Learn("vm-1", "vm", "disk", diskPoints)
	
	// Create a metrics provider that returns anomalous values
	metricsProvider := func(resourceID string) map[string]float64 {
		switch resourceID {
		case "vm-1":
			return map[string]float64{"cpu": 80, "disk": 90} // CPU 4x baseline, disk high
		case "vm-2":
			return map[string]float64{"cpu": 25} // Normal
		default:
			return nil
		}
	}
	
	anomalies := store.GetAllAnomalies(metricsProvider)
	
	// Should have anomalies for vm-1 (cpu 4x baseline + disk at 90%)
	if len(anomalies) < 1 {
		t.Errorf("Expected at least 1 anomaly, got %d", len(anomalies))
	}
	
	// vm-2 should not have anomalies
	for _, a := range anomalies {
		if a.ResourceID == "vm-2" {
			t.Errorf("Did not expect anomaly for vm-2 with normal metrics")
		}
	}
}

func TestGetAllAnomalies_EmptyStore(t *testing.T) {
	store := NewStore(StoreConfig{MinSamples: 10})
	
	metricsProvider := func(resourceID string) map[string]float64 {
		return map[string]float64{"cpu": 90}
	}
	
	anomalies := store.GetAllAnomalies(metricsProvider)
	if len(anomalies) != 0 {
		t.Errorf("Expected no anomalies from empty store, got %d", len(anomalies))
	}
}

func TestFloatToStr(t *testing.T) {
	testCases := []struct {
		value     float64
		precision int
		expected  string
	}{
		{1.5, 1, "1.5"},
		{2.0, 1, "2"},
		{1.05, 2, "1.05"},
		{3.0, 2, "3"},
		{0.5, 1, "0.5"},
		{0.05, 2, "0.05"},
	}
	
	for _, tc := range testCases {
		result := floatToStr(tc.value, tc.precision)
		if result != tc.expected {
			t.Errorf("floatToStr(%f, %d): expected %q, got %q", tc.value, tc.precision, tc.expected, result)
		}
	}
}

