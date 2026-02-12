package ai

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/baseline"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/memory"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestPatrolService_BroadcastFullChannel(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	// Subscribe with a small buffer
	ch := make(chan PatrolStreamEvent, 1)
	ps.streamMu.Lock()
	ps.streamSubscribers[ch] = &streamSubscriber{ch: ch}
	ps.streamMu.Unlock()

	// Fill the channel
	ch <- PatrolStreamEvent{Type: "full"}

	// Broadcast enough times to trigger slow-subscriber eviction (fullCount >= 25).
	for i := 0; i < 25; i++ {
		ps.broadcast(PatrolStreamEvent{Type: "overflow"})
	}

	// Verify the channel was removed from subscribers
	ps.streamMu.RLock()
	_, exists := ps.streamSubscribers[ch]
	ps.streamMu.RUnlock()

	if exists {
		t.Error("Expected channel to be removed from subscribers after full broadcast")
	}
}

type baselineResult struct {
	severity baseline.AnomalySeverity
	zScore   float64
	bl       *baseline.MetricBaseline
}

type mockBaselineStore struct {
	anomalies map[string]baselineResult
}

func (m *mockBaselineStore) CheckAnomaly(resourceID, metric string, value float64) (baseline.AnomalySeverity, float64, *baseline.MetricBaseline) {
	res, ok := m.anomalies[resourceID+":"+metric]
	if !ok {
		return baseline.AnomalyNone, 0, &baseline.MetricBaseline{}
	}
	return res.severity, res.zScore, res.bl
}

func (m *mockBaselineStore) GetBaseline(resourceID, metric string) (*baseline.MetricBaseline, bool) {
	res, ok := m.anomalies[resourceID+":"+metric]
	if !ok {
		return nil, false
	}
	return res.bl, true
}

func (m *mockBaselineStore) Update(resourceID, metric string, value float64) {}
func (m *mockBaselineStore) Save() error                                     { return nil }
func (m *mockBaselineStore) Load() error                                     { return nil }

func TestGenerateRemediationSummary(t *testing.T) {
	tests := []struct {
		command  string
		context  map[string]interface{}
		expected string
	}{
		{"docker restart my-container", nil, "Restarted my-container container"},
		{"docker start my-container", nil, "Restarted my-container container"},
		{"docker restart", nil, "Restarted container"},
		{"docker stop my-container", nil, "Stopped my-container container"},
		{"docker stop", nil, "Stopped container"},
		{"docker ps --filter name=web", nil, "Verified web container is running"},
		{"docker ps", nil, "Checked container status"},
		{"docker logs web", nil, "Retrieved web logs"},
		{"docker logs", nil, "Retrieved container logs"},
		{"systemctl restart nginx", nil, "Restarted nginx service"},
		{"systemctl restart", nil, "Restarted system service"},
		{"systemctl status nginx", nil, "Checked nginx service status"},
		{"systemctl status", nil, "Checked service status"},
		{"df -h /var/lib/frigate", nil, "Analyzed Frigate storage usage"},
		{"du -sh /var/lib/plex", nil, "Analyzed Plex storage usage"},
		{"df -h /mnt/recordings", nil, "Analyzed recordings storage"},
		{"df -h /data/mysql", nil, "Analyzed /data/mysql storage"},
		{"df -h", nil, "Analyzed disk usage"},
		{"grep -r \"config\" /etc/frigate", nil, "Inspected Frigate configuration"},
		{"grep \"server\" /etc/nginx/nginx.conf", nil, "Inspected /nginx/nginx.conf configuration"},
		{"grep \"test\" /sys/config/test.conf", nil, "Inspected /config/test.conf configuration"},
		{"grep \"test\" config", nil, "Inspected configuration"},
		{"tail -f /var/log/syslog", map[string]interface{}{"name": "host1"}, "Reviewed host1 logs"},
		{"journalctl -u nginx", nil, "Reviewed system logs"},
		{"pct resize 100 rootfs +10G", nil, "Resized container 100 disk"},
		{"pct resize", nil, "Resized container disk"},
		{"qm resize 200 virtio0 +20G", nil, "Resized VM 200 disk"},
		{"qm resize", nil, "Resized VM disk"},
		{"ping -c 4 8.8.8.8", nil, "Tested network connectivity"},
		{"curl -I google.com", nil, "Tested network connectivity"},
		{"free -m", nil, "Checked memory usage"},
		{"top -n 1", nil, "Analyzed running processes"},
		{"rm -rf /tmp/test", nil, "Cleaned up files"},
		{"chmod 644 /etc/passwd", nil, "Fixed file permissions"},
		{"ls -la", map[string]interface{}{"name": "host1"}, "Ran diagnostics on host1"},
		{"ls -la", nil, "Ran system diagnostics"},
	}

	for _, tt := range tests {
		result := generateRemediationSummary(tt.command, "", tt.context)
		if result != tt.expected {
			t.Errorf("generateRemediationSummary(%s) = %s, want %s", tt.command, result, tt.expected)
		}
	}
}

// mockMetricsHistoryProvider implements MetricsHistoryProvider
type mockMetricsHistoryProvider struct {
	metrics map[string][]models.MetricPoint // resourceID:metrics -> points
}

func (m *mockMetricsHistoryProvider) GetNodeMetrics(nodeID string, metricType string, duration time.Duration) []models.MetricPoint {
	return m.metrics[nodeID+":"+metricType]
}

func (m *mockMetricsHistoryProvider) GetGuestMetrics(guestID string, metricType string, duration time.Duration) []models.MetricPoint {
	return m.metrics[guestID+":"+metricType]
}

func (m *mockMetricsHistoryProvider) GetAllGuestMetrics(guestID string, duration time.Duration) map[string][]models.MetricPoint {
	result := make(map[string][]models.MetricPoint)
	for k, v := range m.metrics {
		if strings.HasPrefix(k, guestID+":") {
			parts := strings.Split(k, ":")
			if len(parts) == 2 {
				result[parts[1]] = v
			}
		}
	}
	return result
}

func (m *mockMetricsHistoryProvider) GetAllStorageMetrics(storageID string, duration time.Duration) map[string][]models.MetricPoint {
	return nil
}

// mockAlertProvider implements AlertProvider
type mockAlertProvider struct {
	active  []AlertInfo
	history []ResolvedAlertInfo
}

func (m *mockAlertProvider) GetActiveAlerts() []AlertInfo                        { return m.active }
func (m *mockAlertProvider) GetRecentlyResolved(minutes int) []ResolvedAlertInfo { return m.history }
func (m *mockAlertProvider) GetAlertsByResource(resourceID string) []AlertInfo {
	var res []AlertInfo
	for _, a := range m.active {
		if a.ResourceID == resourceID {
			res = append(res, a)
		}
	}
	return res
}
func (m *mockAlertProvider) GetAlertHistory(resourceID string, limit int) []ResolvedAlertInfo {
	var res []ResolvedAlertInfo
	for _, a := range m.history {
		if a.ResourceID == resourceID {
			res = append(res, a)
		}
	}
	return res
}

func TestService_BuildEnrichedResourceContext(t *testing.T) {
	s := NewService(nil, nil)

	// Case 1: Patrol service is nil
	ctx := s.buildEnrichedResourceContext("res1", "", nil)
	if ctx != "" {
		t.Error("Expected empty context when patrol service is nil")
	}

	// Case 2: Patrol service exists but no baseline store
	ps := NewPatrolService(nil, nil)
	s.mu.Lock()
	s.patrolService = ps
	s.mu.Unlock()

	ctx = s.buildEnrichedResourceContext("res1", "", nil)
	// Should at least return empty or minimal if no baseline store
	t.Logf("Ctx (no baseline store): %q", ctx)

	// Case 3: With baseline store and data
	bs := baseline.NewStore(baseline.StoreConfig{MinSamples: 1})
	ps.mu.Lock()
	ps.baselineStore = bs
	ps.mu.Unlock()

	// Add baselines
	now := time.Now()
	// We need 10 samples for the "meaningful" baseline message in buildEnrichedResourceContext
	var cpuPoints, memPoints []baseline.MetricPoint
	for i := 0; i < 11; i++ {
		cpuPoints = append(cpuPoints, baseline.MetricPoint{Value: 10, Timestamp: now.Add(time.Duration(i) * time.Minute)})
		memPoints = append(memPoints, baseline.MetricPoint{Value: 20, Timestamp: now.Add(time.Duration(i) * time.Minute)})
	}
	bs.Learn("res1", "node", "cpu", cpuPoints)
	bs.Learn("res1", "node", "memory", memPoints)

	metrics := map[string]interface{}{
		"cpu_usage":    float64(50), // 5x baseline (anomaly)
		"memory_usage": float64(22), // normal-ish
	}

	ctx = s.buildEnrichedResourceContext("res1", "node", metrics)
	if ctx == "" {
		t.Error("Expected non-empty context")
	}
	t.Logf("Enriched context: %s", ctx)

	if !strings.Contains(ctx, "ANOMALY") {
		t.Error("Expected ANOMALY in context for high CPU")
	}
	if !strings.Contains(ctx, "normal") {
		t.Error("Expected normal in context for memory")
	}

	// Case 4: With Metrics History (Trends)
	mockMH := &mockMetricsHistoryProvider{
		metrics: make(map[string][]models.MetricPoint),
	}
	// Add data showing increasing trend for CPU (>5% trend)
	// Last 24h: 10% -> 20% = +10% over 1 day (+10%/day > 5%)
	tsOld := now.Add(-24 * time.Hour)
	mockMH.metrics["res1:cpu"] = []models.MetricPoint{
		{Value: 10, Timestamp: tsOld},
		{Value: 15, Timestamp: tsOld.Add(12 * time.Hour)},
		{Value: 20, Timestamp: now},
	}
	// Add data showing flat trend for memory
	mockMH.metrics["res1:memory"] = []models.MetricPoint{
		{Value: 50, Timestamp: tsOld},
		{Value: 50, Timestamp: now},
	}

	ps.SetMetricsHistoryProvider(mockMH)

	ctx = s.buildEnrichedResourceContext("res1", "node", metrics)
	t.Logf("Enriched context (trends): %s", ctx)

	if !strings.Contains(ctx, "CPU") || !strings.Contains(ctx, "increasing") {
		t.Error("Expected 'CPU' and 'increasing' trend in context")
	}

	// Case 5: With Pattern Detector (Predictions)
	pd := NewPatternDetector(DefaultPatternConfig())
	// Record 3 events to form a pattern (T-4h, T-2h, T-0h -> Interval 2h -> Next T+2h)
	pd.RecordEvent(HistoricalEvent{ResourceID: "res1", EventType: EventHighCPU, Timestamp: now.Add(-4 * time.Hour)})
	pd.RecordEvent(HistoricalEvent{ResourceID: "res1", EventType: EventHighCPU, Timestamp: now.Add(-2 * time.Hour)})
	pd.RecordEvent(HistoricalEvent{ResourceID: "res1", EventType: EventHighCPU, Timestamp: now})

	ps.SetPatternDetector(pd)

	ctx = s.buildEnrichedResourceContext("res1", "node", metrics)
	t.Logf("Enriched context (patterns): %s", ctx)

	if !strings.Contains(ctx, "Predictions") {
		t.Error("Expected 'Predictions' section in context")
	}
	// Note: formatPatternBasis is showing just the event type currently
	if !strings.Contains(ctx, "high_cpu") {
		t.Error("Expected 'high_cpu' prediction in context")
	}

	// Case 6: With Active/Historical Alerts
	mockAP := &mockAlertProvider{
		active: []AlertInfo{
			{ResourceID: "res1", Type: "cpu", Level: "warning", Message: "High CPU detected", Value: 85, Duration: "5m"},
		},
		history: []ResolvedAlertInfo{
			{AlertInfo: AlertInfo{ResourceID: "res1", Type: "cpu", Level: "warning"}, ResolvedTime: now.Add(-1 * time.Hour)},
		},
	}
	s.SetAlertProvider(mockAP)

	ctx = s.buildEnrichedResourceContext("res1", "node", metrics)
	t.Logf("Enriched context (alerts): %s", ctx)

	if !strings.Contains(ctx, "active alert") {
		t.Error("Expected 'active alert' count in context")
	}
	if !strings.Contains(ctx, "Past 30 days") {
		t.Error("Expected 'Past 30 days' history in context")
	}

	// Case 7: With Change Logic
	cd := NewChangeDetector(memory.ChangeDetectorConfig{})
	// Simulate a "created" change
	cd.DetectChanges([]memory.ResourceSnapshot{
		{
			ID:           "res1",
			Name:         "res1",
			Type:         "vm",
			Status:       "running",
			SnapshotTime: now,
		},
	})
	// Force persistence flush/processing if needed (DetectChanges does it async but returns changes immediately)

	ps.SetChangeDetector(cd)

	ctx = s.buildEnrichedResourceContext("res1", "node", metrics)
	t.Logf("Enriched context (changes): %s", ctx)

	if !strings.Contains(ctx, "Changes") {
		t.Error("Expected 'Changes' section")
	}
	if !strings.Contains(ctx, "created") {
		t.Error("Expected creation event in context")
	}

	// Case 8: With Correlation Logic
	// We need correlation package or just use the alias
	cord := NewCorrelationDetector(DefaultCorrelationConfig())
	// Record an event that should contribute to correlation
	cord.RecordEvent(CorrelationEvent{
		ResourceID:   "res1",
		ResourceType: "vm",
		EventType:    CorrelationEventHighCPU,
		Timestamp:    now,
	})

	ps.SetCorrelationDetector(cord)

	ctx = s.buildEnrichedResourceContext("res1", "node", metrics)
	t.Logf("Enriched context (correlation): %s", ctx)

	// Correlation text might appear depending on implementation.
	// If no correlations found (single event might not be enough), it might be empty section.
	// But let's verify it doesn't crash and we exercised the path.
}

func TestService_BuildIncidentContext(t *testing.T) {
	s := NewService(nil, nil)

	// Case 1: store is nil
	ctx := s.buildIncidentContext("res1", "alert1")
	if ctx != "" {
		t.Logf("Note: ctx is %v", ctx)
	}

	// Mock store
	store := memory.NewIncidentStore(memory.IncidentStoreConfig{})
	s.mu.Lock()
	s.incidentStore = store
	s.mu.Unlock()

	// Case 2: alertID set
	ctx = s.buildIncidentContext("res1", "alert1")
	// Since alert1 doesn't exist, it returns empty
	if ctx != "" {
		t.Logf("Alert context: %s", ctx)
	}

	// Case 3: resourceID set
	ctx = s.buildIncidentContext("res1", "")
	if ctx != "" {
		t.Logf("Resource context: %s", ctx)
	}

	// Case 4: both empty
	ctx = s.buildIncidentContext("", "")
	if ctx != "" {
		t.Error("Expected empty context when both IDs are empty")
	}
}

type mockIncidentStore struct {
}

func (m *mockIncidentStore) FormatForAlert(alertID string, limit int) string {
	return "alert:" + alertID
}

func (m *mockIncidentStore) FormatForResource(resourceID string, limit int) string {
	return "res:" + resourceID
}

func (m *mockIncidentStore) FormatForPatrol(limit int) string {
	return "patrol"
}

func (m *mockIncidentStore) Record(resourceID, resourceType, alertID, analysis, remediation string) error {
	return nil
}
