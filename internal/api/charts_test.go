package api

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// TestChartResponseTypes verifies the ChartResponse struct fields
func TestChartResponseTypes(t *testing.T) {
	t.Parallel()

	now := time.Now().Unix() * 1000

	response := ChartResponse{
		ChartData: map[string]VMChartData{
			"pve1/qemu/100": {
				"cpu":    []MetricPoint{{Timestamp: now, Value: 45.5}},
				"memory": []MetricPoint{{Timestamp: now, Value: 60.0}},
				"disk":   []MetricPoint{{Timestamp: now, Value: 30.0}},
			},
		},
		NodeData: map[string]NodeChartData{
			"pve1": {
				"cpu":    []MetricPoint{{Timestamp: now, Value: 35.0}},
				"memory": []MetricPoint{{Timestamp: now, Value: 50.0}},
			},
		},
		StorageData: map[string]StorageChartData{
			"local-zfs": {
				"disk": []MetricPoint{{Timestamp: now, Value: 25.0}},
			},
		},
		DockerData: map[string]VMChartData{
			"abc123def456": {
				"cpu":    []MetricPoint{{Timestamp: now, Value: 5.0}},
				"memory": []MetricPoint{{Timestamp: now, Value: 15.0}},
			},
		},
		DockerHostData: map[string]VMChartData{
			"docker-host-1": {
				"cpu":    []MetricPoint{{Timestamp: now, Value: 20.0}},
				"memory": []MetricPoint{{Timestamp: now, Value: 40.0}},
			},
		},
		GuestTypes: map[string]string{
			"pve1/qemu/100": "vm",
			"pve1/lxc/200":  "system-container",
		},
		Timestamp: now,
		Stats: ChartStats{
			OldestDataTimestamp: now - 3600000,
		},
	}

	// Verify all fields are properly set
	if len(response.ChartData) != 1 {
		t.Errorf("Expected 1 chart data entry, got %d", len(response.ChartData))
	}
	if len(response.NodeData) != 1 {
		t.Errorf("Expected 1 node data entry, got %d", len(response.NodeData))
	}
	if len(response.StorageData) != 1 {
		t.Errorf("Expected 1 storage data entry, got %d", len(response.StorageData))
	}
	if len(response.DockerData) != 1 {
		t.Errorf("Expected 1 docker data entry, got %d", len(response.DockerData))
	}
	if len(response.DockerHostData) != 1 {
		t.Errorf("Expected 1 docker host data entry, got %d", len(response.DockerHostData))
	}
	if len(response.GuestTypes) != 2 {
		t.Errorf("Expected 2 guest type entries, got %d", len(response.GuestTypes))
	}

	// Verify guest types mapping
	if response.GuestTypes["pve1/qemu/100"] != "vm" {
		t.Errorf("Expected guest type 'vm', got '%s'", response.GuestTypes["pve1/qemu/100"])
	}
	if response.GuestTypes["pve1/lxc/200"] != "system-container" {
		t.Errorf("Expected guest type 'system-container', got '%s'", response.GuestTypes["pve1/lxc/200"])
	}

	// Verify docker data metric points
	cpuPoints := response.DockerData["abc123def456"]["cpu"]
	if len(cpuPoints) != 1 || cpuPoints[0].Value != 5.0 {
		t.Errorf("Docker container CPU metric incorrect")
	}
}

func TestChartResponseJSONSerialization(t *testing.T) {
	t.Parallel()

	now := time.Now().Unix() * 1000

	response := ChartResponse{
		ChartData: map[string]VMChartData{
			"vm-1": {"cpu": []MetricPoint{{Timestamp: now, Value: 50.0}}},
		},
		NodeData:    map[string]NodeChartData{},
		StorageData: map[string]StorageChartData{},
		DockerData: map[string]VMChartData{
			"container-abc": {"cpu": []MetricPoint{{Timestamp: now, Value: 10.0}}},
		},
		DockerHostData: map[string]VMChartData{
			"host-1": {"cpu": []MetricPoint{{Timestamp: now, Value: 25.0}}},
		},
		GuestTypes: map[string]string{"vm-1": "vm"},
		Timestamp:  now,
		Stats:      ChartStats{OldestDataTimestamp: now - 3600000},
	}

	// Serialize to JSON
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("Failed to marshal ChartResponse: %v", err)
	}

	// Deserialize back
	var decoded ChartResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal ChartResponse: %v", err)
	}

	// Verify dockerData field name in JSON
	var rawMap map[string]interface{}
	if err := json.Unmarshal(data, &rawMap); err != nil {
		t.Fatalf("Failed to unmarshal to raw map: %v", err)
	}

	// Check that the JSON keys match expected names
	expectedKeys := []string{"data", "nodeData", "storageData", "dockerData", "dockerHostData", "guestTypes", "timestamp", "stats"}
	for _, key := range expectedKeys {
		if _, ok := rawMap[key]; !ok {
			t.Errorf("Expected JSON key '%s' not found", key)
		}
	}

	// Verify dockerData content
	if dockerData, ok := rawMap["dockerData"].(map[string]interface{}); ok {
		if _, ok := dockerData["container-abc"]; !ok {
			t.Errorf("Expected 'container-abc' in dockerData")
		}
	} else {
		t.Errorf("dockerData not found or wrong type")
	}

	// Verify dockerHostData content
	if dockerHostData, ok := rawMap["dockerHostData"].(map[string]interface{}); ok {
		if _, ok := dockerHostData["host-1"]; !ok {
			t.Errorf("Expected 'host-1' in dockerHostData")
		}
	} else {
		t.Errorf("dockerHostData not found or wrong type")
	}
}

func TestMetricPointStructure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		point    MetricPoint
		wantJSON string
	}{
		{
			name:     "positive values",
			point:    MetricPoint{Timestamp: 1733700000000, Value: 45.5},
			wantJSON: `{"timestamp":1733700000000,"value":45.5}`,
		},
		{
			name:     "zero values",
			point:    MetricPoint{Timestamp: 0, Value: 0},
			wantJSON: `{"timestamp":0,"value":0}`,
		},
		{
			name:     "high precision value",
			point:    MetricPoint{Timestamp: 1733700000000, Value: 99.99999},
			wantJSON: `{"timestamp":1733700000000,"value":99.99999}`,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.point)
			if err != nil {
				t.Fatalf("Failed to marshal MetricPoint: %v", err)
			}
			if string(data) != tt.wantJSON {
				t.Errorf("JSON mismatch\ngot:  %s\nwant: %s", string(data), tt.wantJSON)
			}
		})
	}
}

func TestTimeRangeConversion(t *testing.T) {
	t.Parallel()

	// Test the time range conversion logic used in handleCharts
	tests := []struct {
		rangeStr    string
		expectedDur time.Duration
	}{
		{"5m", 5 * time.Minute},
		{"15m", 15 * time.Minute},
		{"30m", 30 * time.Minute},
		{"1h", time.Hour},
		{"4h", 4 * time.Hour},
		{"8h", 8 * time.Hour},
		{"12h", 12 * time.Hour},
		{"24h", 24 * time.Hour},
		{"7d", 7 * 24 * time.Hour},
		{"30d", 30 * 24 * time.Hour},
		{"", time.Hour},        // default
		{"invalid", time.Hour}, // fallback
	}

	for _, tt := range tests {
		tt := tt
		t.Run("range_"+tt.rangeStr, func(t *testing.T) {
			duration := parseChartsRangeDuration(tt.rangeStr)

			if duration != tt.expectedDur {
				t.Errorf("Time range %q: got %v, want %v", tt.rangeStr, duration, tt.expectedDur)
			}
		})
	}
}

func TestTargetInfrastructureSummarySeriesPoints(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		duration time.Duration
		want     int
	}{
		{name: "zero_defaults_to_minimum", duration: 0, want: infrastructureSummaryMinSeriesPoints},
		{name: "one_hour_keeps_minute_resolution", duration: time.Hour, want: 60},
		{name: "four_hours_caps_at_maximum", duration: 4 * time.Hour, want: infrastructureSummaryMaxSeriesPoints},
		{name: "seven_days_caps_at_maximum", duration: 7 * 24 * time.Hour, want: infrastructureSummaryMaxSeriesPoints},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			if got := targetInfrastructureSummarySeriesPoints(tt.duration); got != tt.want {
				t.Fatalf("targetInfrastructureSummarySeriesPoints(%v) = %d, want %d", tt.duration, got, tt.want)
			}
		})
	}
}

func TestNormalizeInfrastructureSummaryMetricPointSeriesSmoothsMixedCadenceTail(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.March, 31, 12, 0, 0, 0, time.UTC)
	duration := 7 * 24 * time.Hour
	windowStart := now.Add(-duration)

	var points []MetricPoint
	for ts := windowStart; ts.Before(now.Add(-24 * time.Hour)); ts = ts.Add(65 * time.Minute) {
		points = append(points, MetricPoint{
			Timestamp: ts.UnixMilli(),
			Value:     20,
		})
	}
	for ts := now.Add(-24 * time.Hour); ts.Before(now.Add(-2 * time.Hour)); ts = ts.Add(2 * time.Minute) {
		points = append(points, MetricPoint{
			Timestamp: ts.UnixMilli(),
			Value:     40,
		})
	}
	for ts := now.Add(-2 * time.Hour); ts.Before(now); ts = ts.Add(time.Minute) {
		points = append(points, MetricPoint{
			Timestamp: ts.UnixMilli(),
			Value:     60,
		})
	}
	points = append(points, MetricPoint{Timestamp: now.UnixMilli(), Value: 75})

	normalized := normalizeInfrastructureSummaryMetricPointSeries(points, "cpu", duration, now.UnixMilli())
	if len(normalized) > infrastructureSummaryMaxSeriesPoints {
		t.Fatalf("expected normalized point count <= %d, got %d", infrastructureSummaryMaxSeriesPoints, len(normalized))
	}
	if len(normalized) < 60 {
		t.Fatalf("expected a range-wide summary series, got only %d points", len(normalized))
	}
	if normalized[len(normalized)-1].Timestamp != points[len(points)-1].Timestamp {
		t.Fatalf("expected latest timestamp %d, got %d", points[len(points)-1].Timestamp, normalized[len(normalized)-1].Timestamp)
	}
	if normalized[len(normalized)-1].Value != points[len(points)-1].Value {
		t.Fatalf("expected latest value %.1f, got %.1f", points[len(points)-1].Value, normalized[len(normalized)-1].Value)
	}

	recentWindowStart := now.Add(-24 * time.Hour).UnixMilli()
	recentCount := 0
	for _, point := range normalized {
		if point.Timestamp >= recentWindowStart {
			recentCount++
		}
	}
	if recentCount > 20 {
		t.Fatalf("expected roughly day-proportional recent buckets after normalization, got %d recent points", recentCount)
	}
}

func TestNormalizeInfrastructureSummaryMetricPointSeriesAveragesPlateauMetrics(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.March, 31, 12, 0, 0, 0, time.UTC)
	duration := 4 * time.Hour
	bucketCount := targetInfrastructureSummarySeriesPoints(duration)
	windowStart := now.Add(-duration)
	durationMillis := duration.Milliseconds()
	firstBucketEnd := windowStart.UnixMilli() + durationMillis/int64(bucketCount)

	points := []MetricPoint{
		{Timestamp: windowStart.UnixMilli() + 1_000, Value: 20},
		{Timestamp: windowStart.UnixMilli() + 30_000, Value: 40},
		{Timestamp: firstBucketEnd - 1_000, Value: 80},
		{Timestamp: now.UnixMilli(), Value: 55},
	}

	normalized := normalizeInfrastructureSummaryMetricPointSeries(points, "memory", duration, now.UnixMilli())
	if len(normalized) < 2 {
		t.Fatalf("expected at least two normalized points, got %d", len(normalized))
	}

	firstValue := normalized[0].Value
	if firstValue != (20+40+80)/3.0 {
		t.Fatalf("expected first bucket average %.2f, got %.2f", (20+40+80)/3.0, firstValue)
	}
	if normalized[len(normalized)-1].Value != 55 {
		t.Fatalf("expected latest point to preserve final value 55, got %.2f", normalized[len(normalized)-1].Value)
	}
}

func TestTargetWorkloadsSummarySeriesPoints(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		duration time.Duration
		want     int
	}{
		{name: "zero_defaults_to_minimum", duration: 0, want: workloadsSummaryMinSeriesPoints},
		{name: "one_hour_keeps_minute_resolution", duration: time.Hour, want: 60},
		{name: "four_hours_caps_at_maximum", duration: 4 * time.Hour, want: workloadsSummaryMaxSeriesPoints},
		{name: "seven_days_caps_at_maximum", duration: 7 * 24 * time.Hour, want: workloadsSummaryMaxSeriesPoints},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			if got := targetWorkloadsSummarySeriesPoints(tt.duration); got != tt.want {
				t.Fatalf("targetWorkloadsSummarySeriesPoints(%v) = %d, want %d", tt.duration, got, tt.want)
			}
		})
	}
}

func TestCapMetricPointSeriesSmoothsMixedCadenceTail(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.March, 31, 12, 0, 0, 0, time.UTC)
	duration := 7 * 24 * time.Hour
	windowStart := now.Add(-duration)

	var points []MetricPoint
	for ts := windowStart; ts.Before(now.Add(-24 * time.Hour)); ts = ts.Add(65 * time.Minute) {
		points = append(points, MetricPoint{Timestamp: ts.UnixMilli(), Value: 20})
	}
	for ts := now.Add(-24 * time.Hour); ts.Before(now.Add(-2 * time.Hour)); ts = ts.Add(2 * time.Minute) {
		points = append(points, MetricPoint{Timestamp: ts.UnixMilli(), Value: 40})
	}
	for ts := now.Add(-2 * time.Hour); ts.Before(now); ts = ts.Add(time.Minute) {
		points = append(points, MetricPoint{Timestamp: ts.UnixMilli(), Value: 60})
	}
	points = append(points, MetricPoint{Timestamp: now.UnixMilli(), Value: 75})

	capped := capMetricPointSeries(points, 24)
	if len(capped) > 24 {
		t.Fatalf("expected capped point count <= 24, got %d", len(capped))
	}
	if capped[0].Timestamp != points[0].Timestamp {
		t.Fatalf("expected first timestamp %d, got %d", points[0].Timestamp, capped[0].Timestamp)
	}
	if capped[len(capped)-1].Timestamp != points[len(points)-1].Timestamp {
		t.Fatalf("expected latest timestamp %d, got %d", points[len(points)-1].Timestamp, capped[len(capped)-1].Timestamp)
	}

	recentWindowStart := now.Add(-24 * time.Hour).UnixMilli()
	recentCount := 0
	for _, point := range capped {
		if point.Timestamp >= recentWindowStart {
			recentCount++
		}
	}
	if recentCount > 6 {
		t.Fatalf("expected day-proportional capped points, got %d points in the final day", recentCount)
	}
}

func TestNormalizeWorkloadsSummaryMetricPointSeriesSmoothsMixedCadenceTail(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.March, 31, 12, 0, 0, 0, time.UTC)
	duration := 7 * 24 * time.Hour
	windowStart := now.Add(-duration)

	var points []MetricPoint
	for ts := windowStart; ts.Before(now.Add(-24 * time.Hour)); ts = ts.Add(65 * time.Minute) {
		points = append(points, MetricPoint{Timestamp: ts.UnixMilli(), Value: 20})
	}
	for ts := now.Add(-24 * time.Hour); ts.Before(now.Add(-2 * time.Hour)); ts = ts.Add(2 * time.Minute) {
		points = append(points, MetricPoint{Timestamp: ts.UnixMilli(), Value: 40})
	}
	for ts := now.Add(-2 * time.Hour); ts.Before(now); ts = ts.Add(time.Minute) {
		points = append(points, MetricPoint{Timestamp: ts.UnixMilli(), Value: 60})
	}
	points = append(points, MetricPoint{Timestamp: now.UnixMilli(), Value: 75})

	normalized := normalizeWorkloadsSummaryMetricPointSeries(WorkloadsSummaryMetricData{
		P50: append([]MetricPoint(nil), points...),
		P95: append([]MetricPoint(nil), points...),
	}, duration)
	if len(normalized.P50) > workloadsSummaryMaxSeriesPoints {
		t.Fatalf("expected p50 point count <= %d, got %d", workloadsSummaryMaxSeriesPoints, len(normalized.P50))
	}
	if len(normalized.P95) > workloadsSummaryMaxSeriesPoints {
		t.Fatalf("expected p95 point count <= %d, got %d", workloadsSummaryMaxSeriesPoints, len(normalized.P95))
	}
	if normalized.P50[len(normalized.P50)-1].Timestamp != now.UnixMilli() {
		t.Fatalf("expected latest p50 timestamp %d, got %d", now.UnixMilli(), normalized.P50[len(normalized.P50)-1].Timestamp)
	}
	if normalized.P95[len(normalized.P95)-1].Timestamp != now.UnixMilli() {
		t.Fatalf("expected latest p95 timestamp %d, got %d", now.UnixMilli(), normalized.P95[len(normalized.P95)-1].Timestamp)
	}

	recentWindowStart := now.Add(-24 * time.Hour).UnixMilli()
	recentCount := 0
	for _, point := range normalized.P50 {
		if point.Timestamp >= recentWindowStart {
			recentCount++
		}
	}
	if recentCount > 20 {
		t.Fatalf("expected day-proportional workload summary buckets, got %d recent p50 points", recentCount)
	}
}

func TestDockerMetricKeyFormat(t *testing.T) {
	t.Parallel()

	// Test that Docker container metric keys follow the expected format
	// This mirrors the logic in handleCharts: fmt.Sprintf("docker:%s", container.ID)
	tests := []struct {
		containerID string
		expectedKey string
	}{
		{"abc123", "docker:abc123"},
		{"abc123def456789", "docker:abc123def456789"},
		{"sha256:abc123", "docker:sha256:abc123"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.containerID, func(t *testing.T) {
			key := "docker:" + tt.containerID
			if key != tt.expectedKey {
				t.Errorf("Docker metric key: got %q, want %q", key, tt.expectedKey)
			}
		})
	}
}

func TestDockerHostMetricKeyFormat(t *testing.T) {
	t.Parallel()

	// Test that Docker host metric keys follow the expected format
	// This mirrors the logic in handleCharts: fmt.Sprintf("dockerHost:%s", host.ID)
	tests := []struct {
		hostID      string
		expectedKey string
	}{
		{"docker-host-1", "dockerHost:docker-host-1"},
		{"my-server", "dockerHost:my-server"},
		{"192.168.1.10", "dockerHost:192.168.1.10"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.hostID, func(t *testing.T) {
			key := "dockerHost:" + tt.hostID
			if key != tt.expectedKey {
				t.Errorf("Docker host metric key: got %q, want %q", key, tt.expectedKey)
			}
		})
	}
}

func TestGuestTypesMapping(t *testing.T) {
	t.Parallel()

	// Simulate the guest types map building from state
	vms := []models.VM{
		{ID: "pve1/qemu/100"},
		{ID: "pve1/qemu/200"},
	}
	containers := []models.Container{
		{ID: "pve1/lxc/300"},
		{ID: "pve1/lxc/400"},
	}

	guestTypes := make(map[string]string)
	for _, vm := range vms {
		guestTypes[vm.ID] = "vm"
	}
	for _, ct := range containers {
		guestTypes[ct.ID] = "system-container"
	}

	if len(guestTypes) != 4 {
		t.Errorf("Expected 4 guest types, got %d", len(guestTypes))
	}

	// Verify VM types
	for _, vm := range vms {
		if guestTypes[vm.ID] != "vm" {
			t.Errorf("Expected 'vm' for %s, got '%s'", vm.ID, guestTypes[vm.ID])
		}
	}

	// Verify container types
	for _, ct := range containers {
		if guestTypes[ct.ID] != "system-container" {
			t.Errorf("Expected 'system-container' for %s, got '%s'", ct.ID, guestTypes[ct.ID])
		}
	}
}

func TestDockerContainerDiskPercentCalculation(t *testing.T) {
	t.Parallel()

	// Test disk percentage calculation for Docker containers
	// Mirrors: float64(container.WritableLayerBytes) / float64(container.RootFilesystemBytes) * 100
	tests := []struct {
		name                string
		writableLayerBytes  uint64
		rootFilesystemBytes uint64
		expectedDiskPercent float64
	}{
		{"50% usage", 500, 1000, 50.0},
		{"100% usage", 1000, 1000, 100.0},
		{"0% usage", 0, 1000, 0.0},
		{"zero root filesystem", 100, 0, 0.0}, // Should avoid division by zero
		{"both zero", 0, 0, 0.0},
		{"small values", 10, 100, 10.0},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			var diskPercent float64
			if tt.rootFilesystemBytes > 0 && tt.writableLayerBytes > 0 {
				diskPercent = float64(tt.writableLayerBytes) / float64(tt.rootFilesystemBytes) * 100
				if diskPercent > 100 {
					diskPercent = 100
				}
			}

			if diskPercent != tt.expectedDiskPercent {
				t.Errorf("Disk percent: got %f, want %f", diskPercent, tt.expectedDiskPercent)
			}
		})
	}
}

func TestChartStatsOldestTimestamp(t *testing.T) {
	t.Parallel()

	now := time.Now().Unix() * 1000
	oneHourAgo := now - 3600000    // 1 hour in ms
	fourHoursAgo := now - 14400000 // 4 hours in ms

	// Simulate finding oldest timestamp
	timestamps := []int64{now, oneHourAgo, fourHoursAgo, now - 1800000}

	oldestTimestamp := now
	for _, ts := range timestamps {
		if ts < oldestTimestamp {
			oldestTimestamp = ts
		}
	}

	if oldestTimestamp != fourHoursAgo {
		t.Errorf("Oldest timestamp: got %d, want %d", oldestTimestamp, fourHoursAgo)
	}

	stats := ChartStats{OldestDataTimestamp: oldestTimestamp}
	if stats.OldestDataTimestamp != fourHoursAgo {
		t.Errorf("ChartStats.OldestDataTimestamp: got %d, want %d",
			stats.OldestDataTimestamp, fourHoursAgo)
	}
}

func TestChartStatsDebugMetadata(t *testing.T) {
	t.Parallel()

	stats := ChartStats{
		OldestDataTimestamp:   1_730_000_000_000,
		Range:                 "24h",
		RangeSeconds:          86_400,
		MetricsStoreEnabled:   true,
		PrimarySourceHint:     "store_or_memory_fallback",
		InMemoryThresholdSecs: 7_200,
		PointCounts: ChartPointCounts{
			Total:            1200,
			Guests:           300,
			Nodes:            200,
			Storage:          100,
			DockerContainers: 250,
			DockerHosts:      150,
			Agents:           200,
		},
	}

	raw, err := json.Marshal(stats)
	if err != nil {
		t.Fatalf("Failed to marshal ChartStats: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal ChartStats JSON: %v", err)
	}

	if decoded["range"] != "24h" {
		t.Errorf("Expected range=24h, got %v", decoded["range"])
	}
	if decoded["primarySourceHint"] != "store_or_memory_fallback" {
		t.Errorf("Expected primarySourceHint, got %v", decoded["primarySourceHint"])
	}
	pointCounts, ok := decoded["pointCounts"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected pointCounts object in debug stats")
	}
	if pointCounts["total"] != float64(1200) {
		t.Errorf("Expected pointCounts.total=1200, got %v", pointCounts["total"])
	}
}

func TestEmptyDockerData(t *testing.T) {
	t.Parallel()

	// Verify that empty DockerData is properly handled in JSON
	response := ChartResponse{
		ChartData:      map[string]VMChartData{},
		NodeData:       map[string]NodeChartData{},
		StorageData:    map[string]StorageChartData{},
		DockerData:     map[string]VMChartData{},
		DockerHostData: map[string]VMChartData{},
		GuestTypes:     map[string]string{},
		Timestamp:      time.Now().Unix() * 1000,
		Stats:          ChartStats{OldestDataTimestamp: time.Now().Unix() * 1000},
	}

	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("Failed to marshal empty ChartResponse: %v", err)
	}

	var rawMap map[string]interface{}
	if err := json.Unmarshal(data, &rawMap); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Check dockerData is present but empty
	if dockerData, ok := rawMap["dockerData"].(map[string]interface{}); ok {
		if len(dockerData) != 0 {
			t.Errorf("Expected empty dockerData, got %d entries", len(dockerData))
		}
	} else {
		t.Errorf("dockerData should be an empty object, not missing")
	}
}
