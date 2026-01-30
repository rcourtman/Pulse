package ai

import (
	"context"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestAlertTriggeredAnalyzer_ResourceKeyFromAlert(t *testing.T) {
	analyzer := NewAlertTriggeredAnalyzer(nil, nil)

	tests := []struct {
		name     string
		alert    *alerts.Alert
		expected string
	}{
		{
			name: "with resource ID",
			alert: &alerts.Alert{
				ResourceID:   "vm-100",
				ResourceName: "test-vm",
				Instance:     "cluster-1",
			},
			expected: "vm-100",
		},
		{
			name: "with resource name and instance",
			alert: &alerts.Alert{
				ResourceName: "test-vm",
				Instance:     "cluster-1",
			},
			expected: "cluster-1/test-vm",
		},
		{
			name: "with resource name only",
			alert: &alerts.Alert{
				ResourceName: "test-vm",
			},
			expected: "test-vm",
		},
		{
			name:     "empty alert",
			alert:    &alerts.Alert{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := analyzer.resourceKeyFromAlert(tt.alert)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestAlertTriggeredAnalyzer_CleanupOldCooldowns(t *testing.T) {
	analyzer := NewAlertTriggeredAnalyzer(nil, nil)

	// Add some cooldown entries - one old, one recent
	analyzer.mu.Lock()
	analyzer.lastAnalyzed["old-resource"] = time.Now().Add(-2 * time.Hour) // > 1 hour old
	analyzer.lastAnalyzed["recent-resource"] = time.Now()                  // Recent
	analyzer.mu.Unlock()

	// Cleanup
	analyzer.CleanupOldCooldowns()

	analyzer.mu.RLock()
	_, oldExists := analyzer.lastAnalyzed["old-resource"]
	_, recentExists := analyzer.lastAnalyzed["recent-resource"]
	analyzer.mu.RUnlock()

	if oldExists {
		t.Error("Expected old cooldown entry to be removed")
	}
	if !recentExists {
		t.Error("Expected recent cooldown entry to be kept")
	}
}

func TestAlertTriggeredAnalyzer_OnAlertFired_Enabled(t *testing.T) {
	stateProvider := &mockStateProvider{
		state: models.StateSnapshot{
			Nodes: []models.Node{
				{
					ID:     "node/pve1",
					Name:   "pve1",
					Status: "online",
					CPU:    0.95,
					Memory: models.Memory{
						Total: 32000000000,
						Used:  30000000000,
					},
				},
			},
		},
	}

	patrolService := &PatrolService{
		thresholds: DefaultPatrolThresholds(),
	}

	analyzer := NewAlertTriggeredAnalyzer(patrolService, stateProvider)
	analyzer.SetEnabled(true)

	alert := &alerts.Alert{
		ID:           "test-alert-1",
		Type:         "node_cpu",
		ResourceID:   "node/pve1",
		ResourceName: "pve1",
		Value:        95.0,
		Threshold:    90.0,
	}

	// Fire the alert
	analyzer.OnAlertFired(alert)

	// Give time for the goroutine to start and set pending
	time.Sleep(50 * time.Millisecond)

	// Wait for analysis to complete
	time.Sleep(100 * time.Millisecond)

	// After analysis, lastAnalyzed should be updated
	analyzer.mu.RLock()
	_, exists := analyzer.lastAnalyzed["node/pve1"]
	analyzer.mu.RUnlock()

	if !exists {
		t.Error("Expected lastAnalyzed to be updated after alert was fired")
	}
}

func TestAlertTriggeredAnalyzer_OnAlertFired_Disabled(t *testing.T) {
	analyzer := NewAlertTriggeredAnalyzer(nil, nil)
	// Analyzer is disabled by default

	alert := &alerts.Alert{
		ID:           "test-alert-1",
		Type:         "cpu",
		ResourceID:   "node-1",
		ResourceName: "test-node",
		Value:        95.0,
		Threshold:    90.0,
	}

	// When disabled, OnAlertFired should do nothing (no panic)
	analyzer.OnAlertFired(alert)

	// Verify no pending analyses were started
	analyzer.mu.RLock()
	pending := len(analyzer.pending)
	analyzer.mu.RUnlock()

	if pending != 0 {
		t.Errorf("Expected no pending analyses when disabled, got %d", pending)
	}
}

func TestAlertTriggeredAnalyzer_OnAlertFired_NilAlert(t *testing.T) {
	analyzer := NewAlertTriggeredAnalyzer(nil, nil)
	analyzer.SetEnabled(true)

	// Should handle nil alert gracefully (no panic)
	analyzer.OnAlertFired(nil)

	// Verify no pending analyses
	analyzer.mu.RLock()
	pending := len(analyzer.pending)
	analyzer.mu.RUnlock()

	if pending != 0 {
		t.Errorf("Expected no pending analyses for nil alert, got %d", pending)
	}
}

func TestAlertTriggeredAnalyzer_OnAlertFired_EmptyResourceKey(t *testing.T) {
	analyzer := NewAlertTriggeredAnalyzer(nil, nil)
	analyzer.SetEnabled(true)

	// Alert with no resource identifiers
	alert := &alerts.Alert{
		ID:   "test-alert",
		Type: "cpu",
	}

	// Should skip analysis due to empty resource key
	analyzer.OnAlertFired(alert)

	// Wait briefly
	time.Sleep(10 * time.Millisecond)

	// No pending should exist
	analyzer.mu.RLock()
	pending := len(analyzer.pending)
	analyzer.mu.RUnlock()

	if pending != 0 {
		t.Errorf("Expected no pending analyses for empty resource key, got %d", pending)
	}
}

func TestAlertTriggeredAnalyzer_OnAlertFired_Cooldown(t *testing.T) {
	analyzer := NewAlertTriggeredAnalyzer(nil, nil)
	analyzer.SetEnabled(true)
	// Set a short cooldown for testing
	analyzer.cooldown = 100 * time.Millisecond

	alert := &alerts.Alert{
		ID:           "test-alert-1",
		Type:         "cpu",
		ResourceID:   "node-1",
		ResourceName: "test-node",
	}

	// Manually set the resource as recently analyzed
	analyzer.mu.Lock()
	analyzer.lastAnalyzed["node-1"] = time.Now()
	analyzer.mu.Unlock()

	// OnAlertFired should skip due to cooldown
	analyzer.OnAlertFired(alert)

	// Give a moment for any async operations
	time.Sleep(10 * time.Millisecond)

	// Should not have a pending analysis since cooldown is active
	analyzer.mu.RLock()
	pending := len(analyzer.pending)
	analyzer.mu.RUnlock()

	if pending != 0 {
		t.Errorf("Expected no pending analyses during cooldown, got %d", pending)
	}
}

func TestAlertTriggeredAnalyzer_OnAlertFired_Deduplication(t *testing.T) {
	stateProvider := &mockStateProvider{
		state: models.StateSnapshot{
			Nodes: []models.Node{
				{ID: "node-1", Name: "test-node", Status: "online"},
			},
		},
	}

	analyzer := NewAlertTriggeredAnalyzer(nil, stateProvider)
	analyzer.SetEnabled(true)

	alert := &alerts.Alert{
		ID:           "test-alert-1",
		Type:         "node",
		ResourceID:   "node-1",
		ResourceName: "test-node",
	}

	// Manually mark as pending
	analyzer.mu.Lock()
	analyzer.pending["node-1"] = true
	analyzer.mu.Unlock()

	// Second call should be deduplicated
	analyzer.OnAlertFired(alert)

	// Check that we still only have one pending
	analyzer.mu.RLock()
	pendingCount := 0
	for _, isPending := range analyzer.pending {
		if isPending {
			pendingCount++
		}
	}
	analyzer.mu.RUnlock()

	if pendingCount != 1 {
		t.Errorf("Expected 1 pending analysis (deduplication), got %d", pendingCount)
	}
}

func TestAlertTriggeredAnalyzer_AnalyzeResourceByAlert(t *testing.T) {
	patrolService := &PatrolService{thresholds: DefaultPatrolThresholds()}
	analyzer := NewAlertTriggeredAnalyzer(patrolService, &mockStateProvider{})

	// Non-update alert types return nil (handled by LLM-based patrol)
	alertNode := &alerts.Alert{
		ID:           "node-alert",
		Type:         "node_cpu",
		ResourceID:   "node/pve1",
		ResourceName: "pve1",
	}
	if findings := analyzer.analyzeResourceByAlert(context.Background(), alertNode); findings != nil {
		t.Errorf("Expected nil findings for non-update alert, got %v", findings)
	}

	// Unknown alert type returns nil
	alertUnknown := &alerts.Alert{
		ID:   "unknown-alert",
		Type: "unknown_type_xyz",
	}
	if findings := analyzer.analyzeResourceByAlert(context.Background(), alertUnknown); findings != nil {
		t.Errorf("Expected nil findings for unknown alert type, got %v", findings)
	}

	// Test with nil patrol service
	analyzerNoPatrol := NewAlertTriggeredAnalyzer(nil, &mockStateProvider{})
	if findings := analyzerNoPatrol.analyzeResourceByAlert(context.Background(), alertNode); findings != nil {
		t.Error("Expected nil findings when patrol service is nil")
	}
}

func TestAlertTriggeredAnalyzer_AnalyzeResource(t *testing.T) {
	// Heuristic analysis was removed; analyzeResourceByAlert returns nil for node alerts,
	// so analyzeResource should clear pending, update lastAnalyzed, but produce no findings.
	stateProvider := &mockStateProvider{
		state: models.StateSnapshot{
			Nodes: []models.Node{
				{ID: "node/pve1", Name: "pve1", Status: "online", CPU: 0.95},
			},
		},
	}
	patrolService := NewPatrolService(nil, nil)
	patrolService.aiService = &Service{}

	analyzer := NewAlertTriggeredAnalyzer(patrolService, stateProvider)

	alert := &alerts.Alert{
		ID:           "alert-1",
		Type:         "node_cpu",
		ResourceID:   "node/pve1",
		ResourceName: "pve1",
	}
	analyzer.analyzeResource(alert, "node/pve1")

	if analyzer.pending["node/pve1"] {
		t.Error("Expected pending to be cleared after analysis")
	}
	if _, exists := analyzer.lastAnalyzed["node/pve1"]; !exists {
		t.Error("Expected lastAnalyzed to be updated after analysis")
	}
	if len(patrolService.findings.GetAll(nil)) != 0 {
		t.Errorf("Expected no findings (heuristic analysis removed), got %d", len(patrolService.findings.GetAll(nil)))
	}
}

func TestAlertTriggeredAnalyzer_AnalyzeResource_NoFindings(t *testing.T) {
	stateProvider := &mockStateProvider{
		state: models.StateSnapshot{
			Nodes: []models.Node{
				{ID: "node/pve1", Name: "pve1", Status: "online", CPU: 0.05},
			},
		},
	}
	patrolService := NewPatrolService(nil, nil)
	patrolService.aiService = &Service{}

	analyzer := NewAlertTriggeredAnalyzer(patrolService, stateProvider)

	alert := &alerts.Alert{
		ID:           "alert-1",
		Type:         "node_cpu",
		ResourceID:   "node/pve1",
		ResourceName: "pve1",
	}
	analyzer.analyzeResource(alert, "node/pve1")

	if len(patrolService.findings.GetAll(nil)) != 0 {
		t.Error("Expected no findings for healthy resource")
	}
}

type mockStateProvider struct {
	state models.StateSnapshot
}

func (m *mockStateProvider) GetState() models.StateSnapshot {
	return m.state
}

func TestAlertTriggeredAnalyzer_StartStop(t *testing.T) {
	analyzer := NewAlertTriggeredAnalyzer(nil, nil)

	// Verify not started initially
	analyzer.mu.RLock()
	tickerBefore := analyzer.cleanupTicker
	analyzer.mu.RUnlock()

	if tickerBefore != nil {
		t.Error("Expected cleanupTicker to be nil before Start")
	}

	// Start the cleanup goroutine
	analyzer.Start()

	analyzer.mu.RLock()
	tickerAfter := analyzer.cleanupTicker
	analyzer.mu.RUnlock()

	if tickerAfter == nil {
		t.Error("Expected cleanupTicker to be set after Start")
	}

	// Calling Start again should be a no-op
	analyzer.Start()

	analyzer.mu.RLock()
	tickerAfterSecondStart := analyzer.cleanupTicker
	analyzer.mu.RUnlock()

	if tickerAfterSecondStart != tickerAfter {
		t.Error("Expected cleanupTicker to remain the same after second Start")
	}

	// Stop the cleanup goroutine
	analyzer.Stop()

	analyzer.mu.RLock()
	tickerAfterStop := analyzer.cleanupTicker
	analyzer.mu.RUnlock()

	if tickerAfterStop != nil {
		t.Error("Expected cleanupTicker to be nil after Stop")
	}
}

func TestAlertTriggeredAnalyzer_CleanupTickerRuns(t *testing.T) {
	analyzer := NewAlertTriggeredAnalyzer(nil, nil)

	// Add an old entry
	analyzer.mu.Lock()
	analyzer.lastAnalyzed["old-entry"] = time.Now().Add(-2 * time.Hour)
	analyzer.mu.Unlock()

	// Start with a very short ticker for testing (we'll manually trigger cleanup)
	analyzer.Start()
	defer analyzer.Stop()

	// Verify the entry exists
	analyzer.mu.RLock()
	_, existsBefore := analyzer.lastAnalyzed["old-entry"]
	analyzer.mu.RUnlock()

	if !existsBefore {
		t.Fatal("Expected 'old-entry' to exist before cleanup")
	}

	// Manually trigger cleanup
	analyzer.CleanupOldCooldowns()

	// Verify the entry was removed
	analyzer.mu.RLock()
	_, existsAfter := analyzer.lastAnalyzed["old-entry"]
	analyzer.mu.RUnlock()

	if existsAfter {
		t.Error("Expected 'old-entry' to be removed after cleanup")
	}
}

// ===== Update Alert Analysis Tests =====

func TestAlertTriggeredAnalyzer_AnalyzeUpdateAlertFromAlert_NilAlert(t *testing.T) {
	analyzer := NewAlertTriggeredAnalyzer(nil, nil)
	findings := analyzer.analyzeUpdateAlertFromAlert(context.Background(), nil)
	if findings != nil {
		t.Error("Expected nil findings for nil alert")
	}
}

func TestAlertTriggeredAnalyzer_AnalyzeUpdateAlertFromAlert_NilMetadata(t *testing.T) {
	stateProvider := &mockStateProvider{state: models.StateSnapshot{}}
	analyzer := NewAlertTriggeredAnalyzer(nil, stateProvider)

	alert := &alerts.Alert{
		ID:           "update-alert-1",
		Type:         "docker-container-update",
		ResourceID:   "docker:host1/container1",
		ResourceName: "my-container",
		Message:      "Update available",
		// Metadata is nil
	}

	findings := analyzer.analyzeUpdateAlertFromAlert(context.Background(), alert)

	if len(findings) != 1 {
		t.Fatalf("Expected 1 finding, got %d", len(findings))
	}

	f := findings[0]
	if f.ResourceName != "my-container" {
		t.Errorf("Expected ResourceName 'my-container', got '%s'", f.ResourceName)
	}
	// Should use default severity (watch) for unknown container type
	if f.Severity != FindingSeverityWatch {
		t.Errorf("Expected severity 'watch' for unknown type, got '%s'", f.Severity)
	}
}

func TestAlertTriggeredAnalyzer_AnalyzeUpdateAlertFromAlert_EmptyMetadata(t *testing.T) {
	stateProvider := &mockStateProvider{state: models.StateSnapshot{}}
	analyzer := NewAlertTriggeredAnalyzer(nil, stateProvider)

	alert := &alerts.Alert{
		ID:         "update-alert-2",
		Type:       "docker-container-update",
		ResourceID: "docker:host1/container2",
		Metadata:   map[string]interface{}{}, // Empty metadata
	}

	findings := analyzer.analyzeUpdateAlertFromAlert(context.Background(), alert)

	if len(findings) != 1 {
		t.Fatalf("Expected 1 finding, got %d", len(findings))
	}

	f := findings[0]
	// Should use fallback container name
	if f.ResourceName != "unknown container" {
		t.Errorf("Expected fallback 'unknown container', got '%s'", f.ResourceName)
	}
}

func TestAlertTriggeredAnalyzer_AnalyzeUpdateAlertFromAlert_WebServer(t *testing.T) {
	stateProvider := &mockStateProvider{state: models.StateSnapshot{}}
	analyzer := NewAlertTriggeredAnalyzer(nil, stateProvider)

	testCases := []struct {
		image    string
		category FindingCategory
	}{
		{"nginx:latest", FindingCategorySecurity},
		{"traefik:v2.10", FindingCategorySecurity},
		{"haproxy:2.8", FindingCategorySecurity},
		{"apache/httpd:2.4", FindingCategorySecurity},
		{"caddy:2.7", FindingCategorySecurity},
		{"envoyproxy/envoy:v1.28", FindingCategorySecurity},
	}

	for _, tc := range testCases {
		t.Run(tc.image, func(t *testing.T) {
			alert := &alerts.Alert{
				ID:         "update-" + tc.image,
				Type:       "docker-container-update",
				ResourceID: "container-" + tc.image,
				Metadata: map[string]interface{}{
					"containerName": "web-proxy",
					"image":         tc.image,
					"pendingHours":  24,
				},
			}

			findings := analyzer.analyzeUpdateAlertFromAlert(context.Background(), alert)

			if len(findings) != 1 {
				t.Fatalf("Expected 1 finding, got %d", len(findings))
			}

			f := findings[0]
			if f.Category != tc.category {
				t.Errorf("Expected category %s for %s, got %s", tc.category, tc.image, f.Category)
			}
			if f.Severity != FindingSeverityWarning {
				t.Errorf("Expected severity 'warning' for web server, got '%s'", f.Severity)
			}
		})
	}
}

func TestAlertTriggeredAnalyzer_AnalyzeUpdateAlertFromAlert_Database(t *testing.T) {
	stateProvider := &mockStateProvider{state: models.StateSnapshot{}}
	analyzer := NewAlertTriggeredAnalyzer(nil, stateProvider)

	databases := []string{
		"postgres:15",
		"mysql:8.0",
		"mariadb:11",
		"mongo:7",
		"redis:7",
		"influxdb:2.7",
		"clickhouse/clickhouse-server:23",
	}

	for _, db := range databases {
		t.Run(db, func(t *testing.T) {
			alert := &alerts.Alert{
				ID:         "update-db-" + db,
				Type:       "docker-container-update",
				ResourceID: "container-" + db,
				Metadata: map[string]interface{}{
					"containerName": "database",
					"image":         db,
					"pendingHours":  48,
				},
			}

			findings := analyzer.analyzeUpdateAlertFromAlert(context.Background(), alert)

			if len(findings) != 1 {
				t.Fatalf("Expected 1 finding for %s, got %d", db, len(findings))
			}

			f := findings[0]
			if f.Category != FindingCategoryReliability {
				t.Errorf("Expected category 'reliability' for %s, got '%s'", db, f.Category)
			}
			if f.Severity != FindingSeverityWatch {
				t.Errorf("Expected severity 'watch' for database, got '%s'", f.Severity)
			}
		})
	}
}

func TestAlertTriggeredAnalyzer_AnalyzeUpdateAlertFromAlert_AuthService(t *testing.T) {
	stateProvider := &mockStateProvider{state: models.StateSnapshot{}}
	analyzer := NewAlertTriggeredAnalyzer(nil, stateProvider)

	authServices := []string{
		"quay.io/keycloak/keycloak:23",
		"authelia/authelia:4.37",
		"ghcr.io/goauthentik/server:2023",
		"hashicorp/vault:1.15",
	}

	for _, svc := range authServices {
		t.Run(svc, func(t *testing.T) {
			alert := &alerts.Alert{
				ID:         "update-auth-" + svc,
				Type:       "docker-container-update",
				ResourceID: "container-" + svc,
				Metadata: map[string]interface{}{
					"containerName": "auth-service",
					"image":         svc,
					"pendingHours":  12,
				},
			}

			findings := analyzer.analyzeUpdateAlertFromAlert(context.Background(), alert)

			if len(findings) != 1 {
				t.Fatalf("Expected 1 finding for %s, got %d", svc, len(findings))
			}

			f := findings[0]
			if f.Category != FindingCategorySecurity {
				t.Errorf("Expected category 'security' for auth service %s, got '%s'", svc, f.Category)
			}
			if f.Severity != FindingSeverityWarning {
				t.Errorf("Expected severity 'warning' for auth service, got '%s'", f.Severity)
			}
		})
	}
}

func TestAlertTriggeredAnalyzer_AnalyzeUpdateAlertFromAlert_MonitoringLowPriority(t *testing.T) {
	stateProvider := &mockStateProvider{state: models.StateSnapshot{}}
	analyzer := NewAlertTriggeredAnalyzer(nil, stateProvider)

	monitoringTools := []string{
		"prom/prometheus:v2.48",
		"grafana/grafana:10.2",
		"grafana/loki:2.9",
		"jaegertracing/all-in-one:1.52",
		"prom/alertmanager:v0.26",
	}

	for _, tool := range monitoringTools {
		t.Run(tool, func(t *testing.T) {
			alert := &alerts.Alert{
				ID:         "update-mon-" + tool,
				Type:       "docker-container-update",
				ResourceID: "container-" + tool,
				Metadata: map[string]interface{}{
					"containerName": "monitoring",
					"image":         tool,
					"pendingHours":  72,
				},
			}

			findings := analyzer.analyzeUpdateAlertFromAlert(context.Background(), alert)

			if len(findings) != 1 {
				t.Fatalf("Expected 1 finding for %s, got %d", tool, len(findings))
			}

			f := findings[0]
			// Monitoring should be low priority (info)
			if f.Severity != FindingSeverityInfo {
				t.Errorf("Expected severity 'info' for monitoring %s, got '%s'", tool, f.Severity)
			}
		})
	}
}

func TestAlertTriggeredAnalyzer_AnalyzeUpdateAlertFromAlert_TimeBasedEscalation(t *testing.T) {
	stateProvider := &mockStateProvider{state: models.StateSnapshot{}}
	analyzer := NewAlertTriggeredAnalyzer(nil, stateProvider)

	testCases := []struct {
		name             string
		pendingHours     int
		expectedSeverity FindingSeverity
	}{
		{"fresh update (24h)", 24, FindingSeverityWatch},     // < 7 days
		{"week old (168h)", 168, FindingSeverityWatch},       // Exactly 7 days
		{"9 days old (216h)", 216, FindingSeverityWarning},   // > 7 days
		{"2 weeks old (336h)", 336, FindingSeverityWarning},  // Exactly 14 days
		{"3 weeks old (504h)", 504, FindingSeverityCritical}, // > 14 days
		{"month old (720h)", 720, FindingSeverityCritical},   // Way overdue
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			alert := &alerts.Alert{
				ID:         "update-time-test",
				Type:       "docker-container-update",
				ResourceID: "container-generic",
				Metadata: map[string]interface{}{
					"containerName": "generic-app",
					"image":         "myapp:latest", // Unknown type, will get base "watch" severity
					"pendingHours":  tc.pendingHours,
				},
			}

			findings := analyzer.analyzeUpdateAlertFromAlert(context.Background(), alert)

			if len(findings) != 1 {
				t.Fatalf("Expected 1 finding, got %d", len(findings))
			}

			f := findings[0]
			if f.Severity != tc.expectedSeverity {
				t.Errorf("For %s (%d hours), expected severity '%s', got '%s'",
					tc.name, tc.pendingHours, tc.expectedSeverity, f.Severity)
			}
		})
	}
}

func TestAlertTriggeredAnalyzer_AnalyzeUpdateAlertFromAlert_MetadataFloat64(t *testing.T) {
	// Test that pendingHours works when JSON unmarshaled as float64
	stateProvider := &mockStateProvider{state: models.StateSnapshot{}}
	analyzer := NewAlertTriggeredAnalyzer(nil, stateProvider)

	alert := &alerts.Alert{
		ID:         "update-float",
		Type:       "docker-container-update",
		ResourceID: "container-float",
		Metadata: map[string]interface{}{
			"containerName": "test-container",
			"image":         "nginx:latest",
			"pendingHours":  float64(240), // 10 days as float64
		},
	}

	findings := analyzer.analyzeUpdateAlertFromAlert(context.Background(), alert)

	if len(findings) != 1 {
		t.Fatalf("Expected 1 finding, got %d", len(findings))
	}

	f := findings[0]
	// 240 hours (10 days) should escalate from warning (nginx) but still be warning (> 7d but < 14d)
	if f.Severity != FindingSeverityWarning {
		t.Errorf("Expected severity 'warning' for 10 days pending, got '%s'", f.Severity)
	}
}

func TestAlertTriggeredAnalyzer_ClassifyContainerUpdate(t *testing.T) {
	analyzer := NewAlertTriggeredAnalyzer(nil, nil)

	testCases := []struct {
		containerName    string
		imageName        string
		pendingHours     int
		expectedSeverity FindingSeverity
		expectedCategory FindingCategory
		expectedUrgency  string
	}{
		// Web servers
		{"web", "nginx:latest", 24, FindingSeverityWarning, FindingCategorySecurity, "reverse proxy/web server"},
		{"proxy", "traefik:v2", 24, FindingSeverityWarning, FindingCategorySecurity, "reverse proxy/web server"},

		// Auth services
		{"auth", "keycloak:23", 12, FindingSeverityWarning, FindingCategorySecurity, "auth/identity service"},
		{"sso", "authelia:4", 12, FindingSeverityWarning, FindingCategorySecurity, "auth/identity service"},

		// Databases
		{"db", "postgres:15", 48, FindingSeverityWatch, FindingCategoryReliability, "database"},
		{"cache", "redis:7", 48, FindingSeverityWatch, FindingCategoryReliability, "database"},

		// Message queues
		{"queue", "rabbitmq:3.12", 48, FindingSeverityWatch, FindingCategoryReliability, "message queue"},
		{"broker", "confluentinc/kafka:7", 48, FindingSeverityWatch, FindingCategoryReliability, "message queue"},

		// CI/CD
		{"ci", "jenkins/jenkins:lts", 72, FindingSeverityWatch, FindingCategoryReliability, "CI/CD system"},
		{"git", "gitea/gitea:1.21", 72, FindingSeverityWatch, FindingCategoryReliability, "CI/CD system"},

		// Storage/Backup
		{"storage", "minio/minio:latest", 48, FindingSeverityWatch, FindingCategoryBackup, "storage/backup service"},
		{"files", "nextcloud:27", 48, FindingSeverityWatch, FindingCategoryBackup, "storage/backup service"},

		// Monitoring (low priority)
		{"metrics", "prom/prometheus:v2.48", 72, FindingSeverityInfo, FindingCategoryReliability, "monitoring/observability"},
		{"dashboard", "grafana/grafana:10", 72, FindingSeverityInfo, FindingCategoryReliability, "monitoring/observability"},

		// Home automation
		{"home", "homeassistant/home-assistant:2023", 48, FindingSeverityWatch, FindingCategoryReliability, "home automation"},

		// Media
		{"media", "jellyfin/jellyfin:10.8", 72, FindingSeverityInfo, FindingCategoryReliability, "media service"},
		{"movies", "linuxserver/radarr:4.7", 72, FindingSeverityInfo, FindingCategoryReliability, "media service"},

		// Unknown
		{"app", "mycompany/custom-app:v1", 48, FindingSeverityWatch, FindingCategoryReliability, ""},
	}

	for _, tc := range testCases {
		t.Run(tc.imageName, func(t *testing.T) {
			severity, category, urgency, recommendation := analyzer.classifyContainerUpdate(tc.containerName, tc.imageName, tc.pendingHours)

			if severity != tc.expectedSeverity {
				t.Errorf("Expected severity %s for %s, got %s", tc.expectedSeverity, tc.imageName, severity)
			}
			if category != tc.expectedCategory {
				t.Errorf("Expected category %s for %s, got %s", tc.expectedCategory, tc.imageName, category)
			}
			if urgency != tc.expectedUrgency {
				t.Errorf("Expected urgency '%s' for %s, got '%s'", tc.expectedUrgency, tc.imageName, urgency)
			}
			if recommendation == "" {
				t.Errorf("Expected non-empty recommendation for %s", tc.imageName)
			}
		})
	}
}

func TestAlertTriggeredAnalyzer_AnalyzeResourceByAlert_DockerUpdate(t *testing.T) {
	stateProvider := &mockStateProvider{state: models.StateSnapshot{}}
	patrolService := &PatrolService{thresholds: DefaultPatrolThresholds()}
	analyzer := NewAlertTriggeredAnalyzer(patrolService, stateProvider)

	// Docker container update alerts should route to analyzeUpdateAlertFromAlert
	alert := &alerts.Alert{
		ID:         "docker-update-test",
		Type:       "docker-container-update",
		ResourceID: "docker:host/container1",
		Metadata: map[string]interface{}{
			"containerName": "nginx-proxy",
			"image":         "nginx:1.24",
			"pendingHours":  48,
		},
	}

	findings := analyzer.analyzeResourceByAlert(context.Background(), alert)

	if len(findings) != 1 {
		t.Fatalf("Expected 1 finding from update alert routing, got %d", len(findings))
	}

	f := findings[0]
	if f.ResourceType != "Docker Container" {
		t.Errorf("Expected ResourceType 'Docker Container', got '%s'", f.ResourceType)
	}
	if f.Category != FindingCategorySecurity {
		t.Errorf("Expected category 'security' for nginx, got '%s'", f.Category)
	}
}
