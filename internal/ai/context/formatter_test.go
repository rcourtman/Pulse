package context

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestFormatResourceContext_Basic(t *testing.T) {
	ctx := ResourceContext{
		ResourceID:    "vm-100",
		ResourceType:  "vm",
		ResourceName:  "web-server",
		Node:          "pve-1",
		Status:        "running",
		CurrentCPU:    45.5,
		CurrentMemory: 65.2,
		CurrentDisk:   30.0,
		Uptime:        24 * time.Hour,
	}

	result := FormatResourceContext(ctx)

	if result == "" {
		t.Error("Expected non-empty result")
	}

	// Should contain resource name
	if !containsStr(result, "web-server") {
		t.Error("Expected result to contain resource name")
	}

	// Should contain node
	if !containsStr(result, "pve-1") {
		t.Error("Expected result to contain node name")
	}

	// Should contain status
	if !containsStr(result, "running") {
		t.Error("Expected result to contain status")
	}

	// Should contain metrics
	if !containsStr(result, "CPU:") {
		t.Error("Expected result to contain CPU metric")
	}

	// Should contain uptime
	if !containsStr(result, "Uptime") {
		t.Error("Expected result to contain uptime")
	}
}

func TestFormatResourceContext_WithAnomalies(t *testing.T) {
	ctx := ResourceContext{
		ResourceID:   "vm-100",
		ResourceType: "vm",
		ResourceName: "web-server",
		Status:       "running",
		Anomalies: []Anomaly{
			{
				Metric:      "cpu",
				Description: "CPU is critically above normal",
			},
		},
	}

	result := FormatResourceContext(ctx)

	if !containsStr(result, "ANOMALIES") {
		t.Error("Expected result to contain ANOMALIES section")
	}

	if !containsStr(result, "critically above normal") {
		t.Error("Expected result to contain anomaly description")
	}
}

func TestFormatResourceContext_WithPredictions(t *testing.T) {
	ctx := ResourceContext{
		ResourceID:   "storage-1",
		ResourceType: "storage",
		ResourceName: "local-zfs",
		Status:       "available",
		Predictions: []Prediction{
			{
				Event:     "storage_full",
				DaysUntil: 14.5,
			},
		},
	}

	result := FormatResourceContext(ctx)

	if !containsStr(result, "Predictions") {
		t.Error("Expected result to contain Predictions section")
	}

	if !containsStr(result, "storage_full") {
		t.Error("Expected result to contain prediction event")
	}
}

func TestFormatResourceContext_WithUserNotes(t *testing.T) {
	ctx := ResourceContext{
		ResourceID:   "vm-100",
		ResourceType: "vm",
		ResourceName: "web-server",
		Status:       "running",
		UserNotes:    []string{"Web frontend server", "Managed by team-alpha"},
	}

	result := FormatResourceContext(ctx)

	if !containsStr(result, "User Notes") {
		t.Error("Expected result to contain User Notes section")
	}

	if !containsStr(result, "Web frontend server") {
		t.Error("Expected result to contain user note content")
	}
}

func TestFormatResourceContext_WithHistory(t *testing.T) {
	ctx := ResourceContext{
		ResourceID:      "vm-100",
		ResourceType:    "vm",
		ResourceName:    "web-server",
		Status:          "running",
		LastRemediation: "Restarted service 2 days ago",
		PastIssues:      []string{"High CPU on 2024-01-01", "OOM on 2024-01-15"},
	}

	result := FormatResourceContext(ctx)

	if !containsStr(result, "History") {
		t.Error("Expected result to contain History section")
	}

	if !containsStr(result, "Restarted service") {
		t.Error("Expected result to contain remediation info")
	}
}

func TestFormatResourceContext_NodeWithoutNode(t *testing.T) {
	ctx := ResourceContext{
		ResourceID:   "node/pve-1",
		ResourceType: "node",
		ResourceName: "pve-1",
		Status:       "online",
	}

	result := FormatResourceContext(ctx)

	// Node type should NOT show "(on node)" since it IS the node
	if containsStr(result, "(on ") {
		t.Error("Node resource should not show itself as parent node")
	}
}

func TestFormatResourceType(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"node", "Node"},
		{"vm", "VM"},
		{"container", "Container"},
		{"oci_container", "OCI Container"},
		{"storage", "Storage"},
		{"docker_host", "Docker Host"},
		{"docker_container", "Docker Container"},
		{"host", "Host"},
		{"unknown", "Unknown"},
	}

	for _, tt := range tests {
		result := formatResourceType(tt.input)
		if result != tt.expected {
			t.Errorf("formatResourceType(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		input    time.Duration
		expected string
	}{
		{30 * time.Second, "30s"},
		{5 * time.Minute, "5m"},
		{2 * time.Hour, "2h"},
		{2*time.Hour + 30*time.Minute, "2h30m"},
		{24 * time.Hour, "1d"},
		{25 * time.Hour, "1d1h"},
		{48 * time.Hour, "2d"},
		{72*time.Hour + 5*time.Hour, "3d5h"},
	}

	for _, tt := range tests {
		result := formatDuration(tt.input)
		if result != tt.expected {
			t.Errorf("formatDuration(%v) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestFormatRate(t *testing.T) {
	tests := []struct {
		input    float64
		expected string
	}{
		{5.5, "5.5/day"},
		{1.0, "1.0/day"},
		{0.5, "slow"},
		{0.0, "slow"},
		{-2.5, "2.5/day"}, // Absolute value
	}

	for _, tt := range tests {
		result := formatRate(tt.input)
		if result != tt.expected {
			t.Errorf("formatRate(%f) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestFormatTrendLine(t *testing.T) {
	tests := []struct {
		name     string
		metric   string
		trend    Trend
		expected string
	}{
		{
			name:   "insufficient data",
			metric: "cpu",
			trend:  Trend{DataPoints: 2},
			expected: "",
		},
		{
			name:   "growing trend",
			metric: "cpu",
			trend: Trend{
				Direction:  TrendGrowing,
				RatePerDay: 5.0,
				DataPoints: 10,
			},
			expected: "Cpu: (rising 5.0/day)",
		},
		{
			name:   "declining trend",
			metric: "memory",
			trend: Trend{
				Direction:  TrendDeclining,
				RatePerDay: 2.0,
				DataPoints: 10,
			},
			expected: "Memory: (falling 2.0/day)",
		},
		{
			name:   "stable trend",
			metric: "disk",
			trend: Trend{
				Direction:  TrendStable,
				DataPoints: 10,
			},
			expected: "Disk: (stable)",
		},
		{
			name:   "volatile trend",
			metric: "cpu",
			trend: Trend{
				Direction:  TrendVolatile,
				DataPoints: 10,
			},
			expected: "Cpu: (volatile)",
		},
		{
			name:   "with significant range",
			metric: "cpu",
			trend: Trend{
				Direction:  TrendStable,
				DataPoints: 10,
				Min:        20,
				Max:        80,
			},
			expected: "Cpu: (stable) (20-80%)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatTrendLine(tt.metric, tt.trend)
			if result != tt.expected {
				t.Errorf("formatTrendLine(%q, %+v) = %q, want %q", tt.metric, tt.trend, result, tt.expected)
			}
		})
	}
}

func TestFormatBackupStatus(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		contains string
	}{
		{
			name:     "never backed up",
			input:    time.Time{},
			contains: "never",
		},
		{
			name:     "recent backup",
			input:    time.Now().Add(-2 * time.Hour),
			contains: "h ago",
		},
		{
			name:     "old backup",
			input:    time.Now().Add(-72 * time.Hour),
			contains: "d ago",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatBackupStatus(tt.input)
			if !containsStr(result, tt.contains) {
				t.Errorf("FormatBackupStatus() = %q, want to contain %q", result, tt.contains)
			}
		})
	}
}

func TestFormatNodeForContext(t *testing.T) {
	node := models.Node{
		ID:     "node/pve-1",
		Name:   "pve-1",
		Status: "online",
		CPU:    0.45, // 45%
		Memory: models.Memory{
			Used:  32 * 1024 * 1024 * 1024, // 32GB
			Total: 64 * 1024 * 1024 * 1024, // 64GB
		},
		Uptime: 86400, // 1 day
	}

	trends := map[string]Trend{
		"cpu_24h": {Direction: TrendStable, DataPoints: 24},
	}

	ctx := FormatNodeForContext(node, trends)

	if ctx.ResourceID != "node/pve-1" {
		t.Errorf("Expected ResourceID 'node/pve-1', got '%s'", ctx.ResourceID)
	}

	if ctx.ResourceType != "node" {
		t.Errorf("Expected ResourceType 'node', got '%s'", ctx.ResourceType)
	}

	// CPU should be converted from 0-1 to percentage
	if ctx.CurrentCPU != 45.0 {
		t.Errorf("Expected CurrentCPU 45.0, got %f", ctx.CurrentCPU)
	}

	// Memory should be 50%
	if ctx.CurrentMemory != 50.0 {
		t.Errorf("Expected CurrentMemory 50.0, got %f", ctx.CurrentMemory)
	}

	if len(ctx.Trends) != 1 {
		t.Errorf("Expected 1 trend, got %d", len(ctx.Trends))
	}
}

func TestFormatGuestForContext(t *testing.T) {
	trends := map[string]Trend{}
	lastBackup := time.Now().Add(-24 * time.Hour)

	ctx := FormatGuestForContext(
		"vm-100",
		"web-server",
		"pve-1",
		"vm",
		"running",
		0.35,  // CPU (0-1)
		65.0,  // Memory (0-100)
		45.0,  // Disk (0-100)
		3600,  // 1 hour uptime
		lastBackup,
		trends,
	)

	if ctx.ResourceID != "vm-100" {
		t.Errorf("Expected ResourceID 'vm-100', got '%s'", ctx.ResourceID)
	}

	if ctx.ResourceType != "vm" {
		t.Errorf("Expected ResourceType 'vm', got '%s'", ctx.ResourceType)
	}

	if ctx.Node != "pve-1" {
		t.Errorf("Expected Node 'pve-1', got '%s'", ctx.Node)
	}

	// CPU should be converted from 0-1 to percentage
	if ctx.CurrentCPU != 35.0 {
		t.Errorf("Expected CurrentCPU 35.0, got %f", ctx.CurrentCPU)
	}

	// Memory and disk should pass through as-is
	if ctx.CurrentMemory != 65.0 {
		t.Errorf("Expected CurrentMemory 65.0, got %f", ctx.CurrentMemory)
	}

	if ctx.CurrentDisk != 45.0 {
		t.Errorf("Expected CurrentDisk 45.0, got %f", ctx.CurrentDisk)
	}
}

func TestFormatStorageForContext(t *testing.T) {
	storage := models.Storage{
		ID:     "local-zfs",
		Name:   "local-zfs",
		Node:   "pve-1",
		Status: "available",
		Used:   500 * 1024 * 1024 * 1024, // 500GB
		Total:  1000 * 1024 * 1024 * 1024, // 1TB
		Usage:  0, // Will be calculated
	}

	trends := map[string]Trend{
		"usage_7d": {Direction: TrendGrowing, RatePerDay: 1.5, DataPoints: 168},
	}

	ctx := FormatStorageForContext(storage, trends)

	if ctx.ResourceID != "local-zfs" {
		t.Errorf("Expected ResourceID 'local-zfs', got '%s'", ctx.ResourceID)
	}

	if ctx.ResourceType != "storage" {
		t.Errorf("Expected ResourceType 'storage', got '%s'", ctx.ResourceType)
	}

	// Usage should be calculated as 50%
	if ctx.CurrentDisk != 50.0 {
		t.Errorf("Expected CurrentDisk 50.0, got %f", ctx.CurrentDisk)
	}
}

func TestFormatInfrastructureContext_Empty(t *testing.T) {
	ctx := &InfrastructureContext{
		GeneratedAt:    time.Now(),
		TotalResources: 0,
	}

	result := FormatInfrastructureContext(ctx)

	if result == "" {
		t.Error("Expected non-empty result")
	}

	if !containsStr(result, "Infrastructure State") {
		t.Error("Expected result to contain header")
	}

	if !containsStr(result, "0 resources") {
		t.Error("Expected result to contain resource count")
	}
}

func TestFormatInfrastructureContext_Full(t *testing.T) {
	ctx := &InfrastructureContext{
		GeneratedAt:    time.Now(),
		TotalResources: 5,
		Nodes: []ResourceContext{
			{ResourceID: "node-1", ResourceName: "pve-1", ResourceType: "node", Status: "online"},
		},
		VMs: []ResourceContext{
			{ResourceID: "vm-100", ResourceName: "web", ResourceType: "vm", Status: "running"},
		},
		Containers: []ResourceContext{
			{ResourceID: "ct-200", ResourceName: "nginx", ResourceType: "container", Status: "running"},
		},
		Storage: []ResourceContext{
			{ResourceID: "local", ResourceName: "local-zfs", ResourceType: "storage", Status: "available"},
		},
		DockerHosts: []ResourceContext{
			{ResourceID: "docker-1", ResourceName: "docker-host", ResourceType: "docker_host", Status: "online"},
		},
	}

	result := FormatInfrastructureContext(ctx)

	// Check for all sections
	if !containsStr(result, "Proxmox Nodes") {
		t.Error("Expected result to contain Proxmox Nodes section")
	}
	if !containsStr(result, "Virtual Machines") {
		t.Error("Expected result to contain Virtual Machines section")
	}
	if !containsStr(result, "LXC/OCI Containers") {
		t.Error("Expected result to contain Containers section")
	}
	if !containsStr(result, "Storage") {
		t.Error("Expected result to contain Storage section")
	}
	if !containsStr(result, "Docker Hosts") {
		t.Error("Expected result to contain Docker Hosts section")
	}
}

func TestFormatInfrastructureContext_WithAnomalies(t *testing.T) {
	ctx := &InfrastructureContext{
		GeneratedAt:    time.Now(),
		TotalResources: 1,
		Anomalies: []Anomaly{
			{Metric: "cpu", Description: "High CPU cluster-wide"},
		},
	}

	result := FormatInfrastructureContext(ctx)

	if !containsStr(result, "Current Anomalies") {
		t.Error("Expected result to contain Anomalies section")
	}
}

func TestFormatInfrastructureContext_WithPredictions(t *testing.T) {
	ctx := &InfrastructureContext{
		GeneratedAt:    time.Now(),
		TotalResources: 1,
		Predictions: []Prediction{
			{
				Event:      "storage_full",
				ResourceID: "local-zfs",
				DaysUntil:  7,
				Confidence: 0.85,
				Basis:      "Based on 7-day growth trend",
			},
		},
	}

	result := FormatInfrastructureContext(ctx)

	if !containsStr(result, "Predictions") {
		t.Error("Expected result to contain Predictions section")
	}
	if !containsStr(result, "85%") {
		t.Error("Expected result to contain confidence percentage")
	}
}

func TestFormatCompactSummary(t *testing.T) {
	ctx := &InfrastructureContext{
		GeneratedAt:    time.Now(),
		TotalResources: 10,
		Nodes: []ResourceContext{
			{ResourceID: "node-1", Status: "online"},
		},
		VMs: []ResourceContext{
			{ResourceID: "vm-100", Status: "running"},
			{ResourceID: "vm-101", Status: "running", Anomalies: []Anomaly{{Metric: "cpu"}}},
		},
	}

	result := FormatCompactSummary(ctx)

	if result == "" {
		t.Error("Expected non-empty result")
	}

	if !containsStr(result, "10 resources") {
		t.Error("Expected result to contain resource count")
	}

	if !containsStr(result, "Health:") {
		t.Error("Expected result to contain health summary")
	}
}

func TestFormatCompactSummary_WithPredictions(t *testing.T) {
	ctx := &InfrastructureContext{
		GeneratedAt:    time.Now(),
		TotalResources: 5,
		Predictions: []Prediction{
			{Event: "storage_full", DaysUntil: 14},
			{Event: "memory_exhaustion", DaysUntil: 7}, // Nearest
			{Event: "disk_warning", DaysUntil: 21},
		},
	}

	result := FormatCompactSummary(ctx)

	// Should show the nearest prediction (7 days)
	if !containsStr(result, "Nearest prediction") {
		t.Error("Expected result to contain nearest prediction")
	}
	if !containsStr(result, "memory_exhaustion") {
		t.Error("Expected result to show the nearest prediction (memory_exhaustion)")
	}
}

func TestHasGrowingTrend(t *testing.T) {
	tests := []struct {
		name     string
		resource ResourceContext
		expected bool
	}{
		{
			name:     "no trends",
			resource: ResourceContext{},
			expected: false,
		},
		{
			name: "stable trend",
			resource: ResourceContext{
				Trends: map[string]Trend{
					"cpu": {Direction: TrendStable, RatePerDay: 0.5},
				},
			},
			expected: false,
		},
		{
			name: "slow growing trend",
			resource: ResourceContext{
				Trends: map[string]Trend{
					"cpu": {Direction: TrendGrowing, RatePerDay: 0.5}, // < 1
				},
			},
			expected: false,
		},
		{
			name: "fast growing trend",
			resource: ResourceContext{
				Trends: map[string]Trend{
					"cpu": {Direction: TrendGrowing, RatePerDay: 2.0}, // > 1
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasGrowingTrend(tt.resource)
			if result != tt.expected {
				t.Errorf("hasGrowingTrend() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// Helper function
func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestFormatMetricSamples_StepChange(t *testing.T) {
	// Simulate a step change: stable at 26%, then jump to 31%, then stable at 31%
	now := time.Now()
	points := []MetricPoint{
		{Value: 26.2, Timestamp: now.Add(-6 * time.Hour)},
		{Value: 26.1, Timestamp: now.Add(-5 * time.Hour)},
		{Value: 26.3, Timestamp: now.Add(-4 * time.Hour)},
		{Value: 30.7, Timestamp: now.Add(-2 * time.Hour)}, // Jump
		{Value: 30.8, Timestamp: now.Add(-1 * time.Hour)},
		{Value: 30.7, Timestamp: now},
	}

	result := formatMetricSamples("disk", points)

	// Should show the step change: 26→31 (deduped)
	if !containsStr(result, "Disk:") {
		t.Error("Expected result to contain 'Disk:'")
	}
	// Should show the progression, not just the rate
	if !containsStr(result, "26") || !containsStr(result, "31") {
		t.Errorf("Expected result to show both values (26 and 31), got: %s", result)
	}
}

func TestFormatMetricSamples_Stable(t *testing.T) {
	// All values the same
	now := time.Now()
	points := []MetricPoint{
		{Value: 50.0, Timestamp: now.Add(-3 * time.Hour)},
		{Value: 50.1, Timestamp: now.Add(-2 * time.Hour)},
		{Value: 49.9, Timestamp: now.Add(-1 * time.Hour)},
		{Value: 50.0, Timestamp: now},
	}

	result := formatMetricSamples("memory", points)

	// All values round to 50, should show "stable at 50%"
	if !containsStr(result, "stable at 50%") {
		t.Errorf("Expected 'stable at 50%%' for consistent values, got: %s", result)
	}
}

func TestFormatMetricSamples_InsufficientData(t *testing.T) {
	points := []MetricPoint{
		{Value: 50.0, Timestamp: time.Now()},
	}

	result := formatMetricSamples("cpu", points)

	if result != "" {
		t.Errorf("Expected empty string for insufficient data, got: %s", result)
	}
}

func TestDownsampleMetrics(t *testing.T) {
	now := time.Now()
	
	// Create 100 points
	points := make([]MetricPoint, 100)
	for i := 0; i < 100; i++ {
		points[i] = MetricPoint{
			Value:     float64(i),
			Timestamp: now.Add(time.Duration(-100+i) * time.Minute),
		}
	}

	// Downsample to 10
	sampled := DownsampleMetrics(points, 10)

	// Should have roughly 10-11 points (plus potentially the last one)
	if len(sampled) < 10 || len(sampled) > 15 {
		t.Errorf("Expected ~10-15 samples, got %d", len(sampled))
	}

	// Last point should be included
	if sampled[len(sampled)-1].Timestamp != points[99].Timestamp {
		t.Error("Expected last point to be included")
	}

	// First point should be included
	if sampled[0].Timestamp != points[0].Timestamp {
		t.Error("Expected first point to be included")
	}
}

func TestDownsampleMetrics_SmallInput(t *testing.T) {
	now := time.Now()
	
	// Create 5 points - less than target
	points := []MetricPoint{
		{Value: 10, Timestamp: now.Add(-4 * time.Minute)},
		{Value: 20, Timestamp: now.Add(-3 * time.Minute)},
		{Value: 30, Timestamp: now.Add(-2 * time.Minute)},
		{Value: 40, Timestamp: now.Add(-1 * time.Minute)},
		{Value: 50, Timestamp: now},
	}

	// Downsample to 10 should return all 5
	sampled := DownsampleMetrics(points, 10)

	if len(sampled) != 5 {
		t.Errorf("Expected all 5 points when target > input, got %d", len(sampled))
	}
}

func TestFormatResourceContext_WithMetricSamples(t *testing.T) {
	now := time.Now()
	ctx := ResourceContext{
		ResourceID:    "ct-105",
		ResourceType:  "container",
		ResourceName:  "frigate",
		Status:        "running",
		CurrentDisk:   30.7,
		MetricSamples: map[string][]MetricPoint{
			"disk": {
				{Value: 26.2, Timestamp: now.Add(-3 * time.Hour)},
				{Value: 30.7, Timestamp: now.Add(-1 * time.Hour)},
				{Value: 30.7, Timestamp: now},
			},
		},
	}

	result := FormatResourceContext(ctx)

	// Should contain the History section with sampled data
	if !containsStr(result, "History") {
		t.Error("Expected result to contain History section with metric samples")
	}
}

