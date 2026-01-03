package context

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

type mockMetricsHistory struct {
	nodeMetrics       map[string]map[string][]MetricPoint
	guestMetrics      map[string]map[string][]MetricPoint
	allGuestMetrics   map[string]map[string][]MetricPoint
	allStorageMetrics map[string]map[string][]MetricPoint
}

func (m *mockMetricsHistory) GetNodeMetrics(nodeID string, metricType string, duration time.Duration) []MetricPoint {
	if m.nodeMetrics == nil {
		return nil
	}
	if node, ok := m.nodeMetrics[nodeID]; ok {
		return node[metricType]
	}
	return nil
}

func (m *mockMetricsHistory) GetGuestMetrics(guestID string, metricType string, duration time.Duration) []MetricPoint {
	if m.guestMetrics == nil {
		return nil
	}
	if guest, ok := m.guestMetrics[guestID]; ok {
		return guest[metricType]
	}
	return nil
}

func (m *mockMetricsHistory) GetAllGuestMetrics(guestID string, duration time.Duration) map[string][]MetricPoint {
	if m.allGuestMetrics == nil {
		return nil
	}
	return m.allGuestMetrics[guestID]
}

func (m *mockMetricsHistory) GetAllStorageMetrics(storageID string, duration time.Duration) map[string][]MetricPoint {
	if m.allStorageMetrics == nil {
		return nil
	}
	return m.allStorageMetrics[storageID]
}

type mockKnowledge struct {
	notes map[string][]string
}

func (m *mockKnowledge) GetNotes(guestID string) []string {
	return m.notes[guestID]
}

func (m *mockKnowledge) FormatAllForContext() string {
	return "mock formatted knowledge"
}

type mockFindings struct {
	findings map[string][]string
}

func (m *mockFindings) GetDismissedForContext() string {
	return "mock dismissed findings"
}

func (m *mockFindings) GetPastFindingsForResource(resourceID string) []string {
	return m.findings[resourceID]
}

type mockBaseline struct {
	anomalies map[string]map[string]struct {
		severity string
		zScore   float64
		mean     float64
		stddev   float64
		ok       bool
	}
}

func (m *mockBaseline) CheckAnomaly(resourceID, metric string, value float64) (string, float64, float64, float64, bool) {
	if m.anomalies == nil {
		return "", 0, 0, 0, false
	}
	if res, ok := m.anomalies[resourceID]; ok {
		if val, ok := res[metric]; ok {
			return val.severity, val.zScore, val.mean, val.stddev, val.ok
		}
	}
	return "", 0, 0, 0, false
}

func (m *mockBaseline) GetBaseline(resourceID, metric string) (float64, float64, int, bool) {
	return 50.0, 10.0, 100, true
}

func TestNewBuilder(t *testing.T) {
	builder := NewBuilder()
	if builder == nil {
		t.Fatal("Expected non-nil builder")
	}
}

func TestBuilder_WithMethods(t *testing.T) {
	builder := NewBuilder()

	// Test chaining methods return the builder
	result := builder.
		WithMetricsHistory(nil).
		WithKnowledge(nil).
		WithFindings(nil).
		WithBaseline(nil)

	if result != builder {
		t.Error("Expected method chaining to return same builder instance")
	}
}

func TestBuilder_BuildForInfrastructure_Empty(t *testing.T) {
	builder := NewBuilder()

	// Empty state
	state := models.StateSnapshot{}

	ctx := builder.BuildForInfrastructure(state)

	if ctx == nil {
		t.Fatal("Expected non-nil context")
	}

	// Empty state should have zero totals
	if ctx.TotalResources != 0 {
		t.Errorf("Expected 0 total resources for empty state, got %d", ctx.TotalResources)
	}

	// GeneratedAt should be set
	if ctx.GeneratedAt.IsZero() {
		t.Error("Expected GeneratedAt to be set")
	}
}

func TestBuilder_BuildForInfrastructure_WithNodes(t *testing.T) {
	builder := NewBuilder()

	state := models.StateSnapshot{
		Nodes: []models.Node{
			{
				ID:     "node-1",
				Name:   "pve-primary",
				Status: "online",
				CPU:    0.45,
			},
			{
				ID:     "node-2",
				Name:   "pve-secondary",
				Status: "online",
				CPU:    0.25,
			},
		},
	}

	ctx := builder.BuildForInfrastructure(state)

	if len(ctx.Nodes) != 2 {
		t.Errorf("Expected 2 nodes, got %d", len(ctx.Nodes))
	}
}

func TestBuilder_BuildForInfrastructure_WithGuests(t *testing.T) {
	builder := NewBuilder()

	state := models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "node-1", Name: "pve-primary", Status: "online"},
		},
		VMs: []models.VM{
			{ID: "vm-100", Name: "web-server", Node: "pve-primary", Status: "running", CPU: 0.30},
			{ID: "vm-101", Name: "database", Node: "pve-primary", Status: "running", CPU: 0.50},
		},
		Containers: []models.Container{
			{ID: "ct-200", Name: "nginx-proxy", Node: "pve-primary", Status: "running", CPU: 0.10},
		},
	}

	ctx := builder.BuildForInfrastructure(state)

	if len(ctx.VMs) != 2 {
		t.Errorf("Expected 2 VMs, got %d", len(ctx.VMs))
	}

	if len(ctx.Containers) != 1 {
		t.Errorf("Expected 1 container, got %d", len(ctx.Containers))
	}
}

func TestBuilder_BuildForInfrastructure_SkipsTemplates(t *testing.T) {
	builder := NewBuilder()

	state := models.StateSnapshot{
		VMs: []models.VM{
			{ID: "vm-100", Name: "web-server", Status: "running", Template: false},
			{ID: "vm-101", Name: "template", Status: "stopped", Template: true},
		},
		Containers: []models.Container{
			{ID: "ct-200", Name: "nginx", Status: "running", Template: false},
			{ID: "ct-201", Name: "template", Status: "stopped", Template: true},
		},
	}

	ctx := builder.BuildForInfrastructure(state)

	// Should skip templates
	if len(ctx.VMs) != 1 {
		t.Errorf("Expected 1 VM (template skipped), got %d", len(ctx.VMs))
	}

	if len(ctx.Containers) != 1 {
		t.Errorf("Expected 1 container (template skipped), got %d", len(ctx.Containers))
	}
}

func TestBuilder_BuildForInfrastructure_WithStorage(t *testing.T) {
	builder := NewBuilder()

	state := models.StateSnapshot{
		Storage: []models.Storage{
			{
				ID:     "storage-1",
				Name:   "local-zfs",
				Type:   "zfspool",
				Status: "available",
			},
			{
				ID:     "storage-2",
				Name:   "nfs-share",
				Type:   "nfs",
				Status: "available",
			},
		},
	}

	ctx := builder.BuildForInfrastructure(state)

	if len(ctx.Storage) != 2 {
		t.Errorf("Expected 2 storage, got %d", len(ctx.Storage))
	}
}

func TestBuilder_BuildForInfrastructure_WithDocker(t *testing.T) {
	builder := NewBuilder()

	state := models.StateSnapshot{
		DockerHosts: []models.DockerHost{
			{
				ID:       "docker-1",
				Hostname: "docker-host-1",
				Containers: []models.DockerContainer{
					{ID: "container-1", Name: "nginx", State: "running"},
					{ID: "container-2", Name: "redis", State: "running"},
				},
			},
		},
	}

	ctx := builder.BuildForInfrastructure(state)

	if len(ctx.DockerHosts) != 1 {
		t.Errorf("Expected 1 docker host, got %d", len(ctx.DockerHosts))
	}
}

func TestBuilder_BuildForInfrastructure_WithHosts(t *testing.T) {
	builder := NewBuilder()

	state := models.StateSnapshot{
		Hosts: []models.Host{
			{
				ID:       "host-1",
				Hostname: "server-1",
				Status:   "online",
				CPUCount: 8,
			},
		},
	}

	ctx := builder.BuildForInfrastructure(state)

	if len(ctx.Hosts) != 1 {
		t.Errorf("Expected 1 host, got %d", len(ctx.Hosts))
	}
}

func TestBuilder_BuildForInfrastructure_TotalResources(t *testing.T) {
	builder := NewBuilder()

	state := models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "node-1", Name: "pve-1", Status: "online"},
		},
		VMs: []models.VM{
			{ID: "vm-100", Name: "web", Status: "running"},
			{ID: "vm-101", Name: "db", Status: "running"},
		},
		Containers: []models.Container{
			{ID: "ct-200", Name: "nginx", Status: "running"},
		},
		Storage: []models.Storage{
			{ID: "local", Name: "local", Status: "available"},
		},
	}

	ctx := builder.BuildForInfrastructure(state)

	// 1 node + 2 VMs + 1 container + 1 storage = 5
	expected := 5
	if ctx.TotalResources != expected {
		t.Errorf("Expected %d total resources, got %d", expected, ctx.TotalResources)
	}
}

func TestBuilder_BuildForInfrastructure_OCI(t *testing.T) {
	builder := NewBuilder()

	state := models.StateSnapshot{
		Containers: []models.Container{
			{
				ID:         "ct-200",
				Name:       "nginx-oci",
				Status:     "running",
				IsOCI:      true,
				OSTemplate: "docker.io/library/nginx:latest",
			},
		},
	}

	ctx := builder.BuildForInfrastructure(state)

	if len(ctx.Containers) != 1 {
		t.Fatalf("Expected 1 container, got %d", len(ctx.Containers))
	}

	// OCI container should have type oci_container
	if ctx.Containers[0].ResourceType != "oci_container" {
		t.Errorf("Expected resource type 'oci_container', got '%s'", ctx.Containers[0].ResourceType)
	}

	// Should have metadata with image
	if ctx.Containers[0].Metadata == nil {
		t.Error("Expected metadata to be set for OCI container")
	} else if ctx.Containers[0].Metadata["oci_image"] != "docker.io/library/nginx:latest" {
		t.Errorf("Expected oci_image metadata, got %v", ctx.Containers[0].Metadata)
	}
}

func TestBuilder_MergeContexts(t *testing.T) {
	builder := NewBuilder()

	target := &ResourceContext{
		ResourceID:   "vm-100",
		ResourceType: "vm",
		ResourceName: "web-server",
		Status:       "running",
		Node:         "pve-1",
	}

	infra := &InfrastructureContext{
		TotalResources: 10,
		GeneratedAt:    time.Now(),
		VMs: []ResourceContext{
			{ResourceID: "vm-100", ResourceName: "web-server", Node: "pve-1"},
			{ResourceID: "vm-101", ResourceName: "database", Node: "pve-1"},
		},
	}

	result := builder.MergeContexts(target, infra)

	if result == "" {
		t.Error("Expected non-empty merged context")
	}

	// Should contain target resource section
	if !containsSubstring(result, "Target Resource") {
		t.Error("Expected merged context to contain 'Target Resource' section")
	}
}

func TestBuilder_MergeContexts_IncludesRelated(t *testing.T) {
	builder := NewBuilder()

	target := &ResourceContext{
		ResourceID:   "vm-100",
		ResourceType: "vm",
		ResourceName: "web-server",
		Node:         "pve-1",
	}

	infra := &InfrastructureContext{
		VMs: []ResourceContext{
			{ResourceID: "vm-100", ResourceName: "web-server", Node: "pve-1"},
			{ResourceID: "vm-101", ResourceName: "database", Node: "pve-1"},
			{ResourceID: "vm-102", ResourceName: "other", Node: "pve-2"}, // Different node
		},
	}

	result := builder.MergeContexts(target, infra)

	// Should include related resources section
	if !containsSubstring(result, "Related Resources") {
		t.Error("Expected merged context to contain 'Related Resources' section when target has a node")
	}
}

func TestResourceContext_Fields(t *testing.T) {
	ctx := ResourceContext{
		ResourceID:    "vm-100",
		ResourceType:  "vm",
		ResourceName:  "test-vm",
		Node:          "node-1",
		CurrentCPU:    0.45,
		CurrentMemory: 0.65,
		CurrentDisk:   0.30,
		Status:        "running",
	}

	if ctx.ResourceID != "vm-100" {
		t.Errorf("Expected ResourceID 'vm-100', got '%s'", ctx.ResourceID)
	}

	if ctx.CurrentCPU != 0.45 {
		t.Errorf("Expected CurrentCPU 0.45, got %f", ctx.CurrentCPU)
	}

	if ctx.Node != "node-1" {
		t.Errorf("Expected Node 'node-1', got '%s'", ctx.Node)
	}
}

func TestInfrastructureContext_Fields(t *testing.T) {
	now := time.Now()
	ctx := InfrastructureContext{
		GeneratedAt:       now,
		TotalResources:    25,
		ResourcesWithData: 20,
	}

	if ctx.TotalResources != 25 {
		t.Errorf("Expected 25 total resources, got %d", ctx.TotalResources)
	}

	if ctx.GeneratedAt != now {
		t.Error("Expected GeneratedAt to match")
	}

	if ctx.ResourcesWithData != 20 {
		t.Errorf("Expected 20 resources with data, got %d", ctx.ResourcesWithData)
	}
}

// Helper function
func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestBuilder_BuildForInfrastructure_Enriched(t *testing.T) {
	now := time.Now()
	mh := &mockMetricsHistory{
		nodeMetrics: map[string]map[string][]MetricPoint{
			"node-1": {
				"cpu": []MetricPoint{
					{Timestamp: now.Add(-1 * time.Hour), Value: 10.0},
					{Timestamp: now.Add(-2 * time.Hour), Value: 20.0},
					{Timestamp: now.Add(-3 * time.Hour), Value: 30.0},
					{Timestamp: now.Add(-4 * time.Hour), Value: 40.0},
					{Timestamp: now.Add(-24 * time.Hour), Value: 50.0},
					{Timestamp: now.Add(-25 * time.Hour), Value: 60.0},
					{Timestamp: now.Add(-26 * time.Hour), Value: 70.0},
					{Timestamp: now.Add(-27 * time.Hour), Value: 80.0},
					{Timestamp: now.Add(-28 * time.Hour), Value: 90.0},
					{Timestamp: now.Add(-29 * time.Hour), Value: 100.0},
					{Timestamp: now.Add(-30 * time.Hour), Value: 110.0},
				},
			},
		},
		allGuestMetrics: map[string]map[string][]MetricPoint{
			"vm-100": {
				"cpu": []MetricPoint{
					{Timestamp: now.Add(-1 * time.Hour), Value: 10.0},
					{Timestamp: now.Add(-2 * time.Hour), Value: 11.0},
					{Timestamp: now.Add(-3 * time.Hour), Value: 12.0},
				},
			},
		},
		allStorageMetrics: map[string]map[string][]MetricPoint{
			"storage-1": {
				"usage": []MetricPoint{
					{Timestamp: now.Add(-1 * time.Hour), Value: 80.0},
					{Timestamp: now.Add(-2 * time.Hour), Value: 79.0},
					{Timestamp: now.Add(-3 * time.Hour), Value: 78.0},
					{Timestamp: now.Add(-4 * time.Hour), Value: 77.0},
					{Timestamp: now.Add(-5 * time.Hour), Value: 76.0},
					{Timestamp: now.Add(-6 * time.Hour), Value: 75.0},
					{Timestamp: now.Add(-7 * time.Hour), Value: 74.0},
					{Timestamp: now.Add(-8 * time.Hour), Value: 73.0},
					{Timestamp: now.Add(-9 * time.Hour), Value: 72.0},
					{Timestamp: now.Add(-10 * time.Hour), Value: 71.0},
				},
			},
		},
	}

	known := &mockKnowledge{
		notes: map[string][]string{
			"vm-100": {"Important database"},
		},
	}

	bl := &mockBaseline{
		anomalies: map[string]map[string]struct {
			severity string
			zScore   float64
			mean     float64
			stddev   float64
			ok       bool
		}{
			"node-1": {
				"cpu": {severity: "high", zScore: 3.5, mean: 20.0, stddev: 5.0, ok: true},
			},
			"vm-100": {
				"memory": {severity: "low", zScore: -2.1, mean: 80.0, stddev: 10.0, ok: true},
			},
		},
	}

	builder := NewBuilder().
		WithMetricsHistory(mh).
		WithKnowledge(known).
		WithBaseline(bl)

	state := models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "node-1", Name: "pve-1", Status: "online", CPU: 40.0},
		},
		VMs: []models.VM{
			{ID: "vm-100", Name: "db", Node: "pve-1", Status: "running", Memory: models.Memory{Usage: 0.5}},
		},
		Storage: []models.Storage{
			{ID: "storage-1", Name: "local", Status: "available", Usage: 80.0, Total: 1000, Used: 800},
		},
	}

	ctx := builder.BuildForInfrastructure(state)

	if len(ctx.Nodes) == 0 || len(ctx.Nodes[0].Trends) == 0 {
		t.Error("Expected trends for node-1")
	}

	if len(ctx.Nodes[0].Anomalies) == 0 {
		t.Error("Expected anomalies for node-1")
	}

	if len(ctx.VMs) == 0 || len(ctx.VMs[0].UserNotes) == 0 {
		t.Error("Expected notes for vm-100")
	}

	if len(ctx.VMs[0].MetricSamples) == 0 {
		t.Error("Expected metric samples for vm-100")
	}

	if len(ctx.Storage) == 0 || len(ctx.Storage[0].Trends) == 0 {
		t.Error("Expected trends for storage-1")
	}
}

func TestBuilder_StoragePredictions(t *testing.T) {
	// Mock growing trend: 1% per day
	trends := map[string]Trend{
		"usage_7d": {
			Direction:  TrendGrowing,
			RatePerDay: 1.0,
			Confidence: 0.9,
			DataPoints: 12,
			Period:     7 * 24 * time.Hour,
		},
	}

	storage := models.Storage{
		ID:    "storage-1",
		Usage: 85.0,
		Total: 1000,
		Used:  850,
	}

	builder := NewBuilder()
	predictions := builder.computeStoragePredictions(storage, trends)

	// Current 85%, growing 1%/day.
	// 90% in 5 days.
	// 100% in 15 days.
	if len(predictions) != 2 {
		t.Fatalf("Expected 2 predictions, got %d", len(predictions))
	}

	if predictions[0].Event != "storage_warning_90pct" {
		t.Errorf("Expected first prediction to be 90%% warning, got %s", predictions[0].Event)
	}
	if predictions[0].DaysUntil != 5.0 {
		t.Errorf("Expected 5 days until 90%%, got %f", predictions[0].DaysUntil)
	}

	if predictions[1].Event != "storage_full" {
		t.Errorf("Expected second prediction to be storage_full, got %s", predictions[1].Event)
	}
	if predictions[1].DaysUntil != 15.0 {
		t.Errorf("Expected 15 days until 100%%, got %f", predictions[1].DaysUntil)
	}

	// Test already past threshold
	storage.Usage = 95.0
	predictions = builder.computeStoragePredictions(storage, trends)
	if len(predictions) != 1 {
		t.Errorf("Expected 1 prediction (only 100%%), got %d", len(predictions))
	}

	// Test not growing
	trends["usage_7d"] = Trend{Direction: TrendStable, RatePerDay: 0.05}
	predictions = builder.computeStoragePredictions(storage, trends)
	if len(predictions) != 0 {
		t.Errorf("Expected 0 predictions for stable trend, got %d", len(predictions))
	}
}

func TestBuilder_BuildHostContext(t *testing.T) {
	builder := NewBuilder()
	host := models.Host{
		ID:            "host-1",
		Hostname:      "server-1",
		DisplayName:   "Primary Server",
		Status:        "online",
		UptimeSeconds: 3600,
		LoadAverage:   []float64{4.0, 3.5, 3.0},
		CPUCount:      8,
		Memory: models.Memory{
			Total: 16000,
			Used:  8000,
		},
	}

	ctx := builder.buildHostContext(host)
	if ctx.ResourceName != "Primary Server" {
		t.Errorf("Expected display name, got %s", ctx.ResourceName)
	}
	if ctx.CurrentCPU != 50.0 { // 4.0 / 8 * 100
		t.Errorf("Expected 50%% CPU, got %f", ctx.CurrentCPU)
	}
	if ctx.CurrentMemory != 50.0 { // 8000 / 16000 * 100
		t.Errorf("Expected 50%% memory, got %f", ctx.CurrentMemory)
	}
}

func TestBuilder_BuildDockerHostContext(t *testing.T) {
	builder := NewBuilder()
	host := models.DockerHost{
		ID:            "docker-1",
		Hostname:      "docker-host",
		DisplayName:   "Docker Box",
		Status:        "running",
		UptimeSeconds: 7200,
	}

	ctx := builder.buildDockerHostContext(host)
	if ctx.ResourceName != "Docker Box" {
		t.Errorf("Expected display name, got %s", ctx.ResourceName)
	}
	if ctx.ResourceType != "docker_host" {
		t.Errorf("Expected type docker_host, got %s", ctx.ResourceType)
	}
}

func TestBuilder_GuestTrends_Insufficient(t *testing.T) {
	now := time.Now()
	mh := &mockMetricsHistory{
		allGuestMetrics: map[string]map[string][]MetricPoint{
			"vm-100": {
				"cpu": []MetricPoint{
					{Timestamp: now.Add(-1 * time.Hour), Value: 10.0},
					{Timestamp: now.Add(-2 * time.Hour), Value: 11.0},
					{Timestamp: now.Add(-3 * time.Hour), Value: 12.0},
					// Only 3 points, not enough for 7d trend (needs 10)
				},
			},
		},
	}
	builder := NewBuilder().WithMetricsHistory(mh)
	trends := builder.computeGuestTrends("vm-100")
	if _, ok := trends["cpu_7d"]; ok {
		t.Error("Did not expect 7d trend for only 3 points")
	}
}

func TestBuilder_MergeContexts_WithContainers(t *testing.T) {
	builder := NewBuilder()
	target := &ResourceContext{ResourceID: "vm-100", Node: "pve-1"}
	infra := &InfrastructureContext{
		Containers: []ResourceContext{
			{ResourceID: "ct-200", Node: "pve-1", ResourceName: "ct1"},
		},
	}
	result := builder.MergeContexts(target, infra)
	if !containsSubstring(result, "ct1") {
		t.Error("Expected related container to be included in merged context")
	}
}

func TestBuilder_Options(t *testing.T) {
	b := NewBuilder()
	b.includeTrends = false
	if len(b.computeGuestTrends("vm-1")) != 0 {
		t.Error("Expected no trends when includeTrends is false")
	}

	b.includeBaseline = false
	ctx := &ResourceContext{ResourceID: "vm-1"}
	b.enrichWithAnomalies(ctx)
	if len(ctx.Anomalies) != 0 {
		t.Error("Expected no anomalies when includeBaseline is false")
	}

	b.metricsHistory = nil
	if len(b.computeGuestMetricSamples("vm-1")) != 0 {
		t.Error("Expected no samples when metricsHistory is nil")
	}
}

func TestBuilder_StoragePredictions_Far(t *testing.T) {
	b := NewBuilder()
	storage := models.Storage{Usage: 10.0}
	trends := map[string]Trend{
		"usage_7d": {
			Direction:  TrendGrowing,
			RatePerDay: 0.1, // Will take 800 days to reach 90%
			DataPoints: 12,
		},
	}
	preds := b.computeStoragePredictions(storage, trends)
	if len(preds) != 0 {
		t.Errorf("Expected no predictions for far-out ETA, got %d", len(preds))
	}
}

func TestBuilder_GuestTrends_Empty(t *testing.T) {
	mh := &mockMetricsHistory{
		allGuestMetrics: map[string]map[string][]MetricPoint{
			"vm-1": {"cpu": {{}}}, // only 1 point
		},
	}
	b := NewBuilder().WithMetricsHistory(mh)
	if len(b.computeGuestTrends("vm-1")) != 0 {
		t.Error("Expected no trends for single point")
	}
}

func TestBuilder_BuildForInfrastructure_EdgeCases(t *testing.T) {
	builder := NewBuilder().WithBaseline(&mockBaseline{
		anomalies: map[string]map[string]struct {
			severity string
			zScore   float64
			mean     float64
			stddev   float64
			ok       bool
		}{
			"node-1": {"cpu": {severity: "high", ok: true}},
		},
	})

	state := models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "node-1", CPU: 0.0}, // zero value should skip anomaly check
		},
		Storage: []models.Storage{
			{ID: "s1", Usage: 95.0, Total: 100, Used: 95}, // past 90% threshold
		},
	}

	// Mock growing trend for storage
	trends := map[string]Trend{
		"usage_7d": {Direction: TrendGrowing, RatePerDay: 1.0, DataPoints: 10},
	}

	ctx := builder.BuildForInfrastructure(state)
	if len(ctx.Nodes[0].Anomalies) != 0 {
		t.Error("Expected no anomalies for zero CPU")
	}

	// computeStoragePredictions for s1 should only have 100% prediction, not 90%
	preds := builder.computeStoragePredictions(state.Storage[0], trends)
	if len(preds) != 1 || preds[0].Event != "storage_full" {
		t.Errorf("Expected only storage_full prediction, got %v", preds)
	}
}
