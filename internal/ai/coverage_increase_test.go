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
	ps.streamSubscribers[ch] = struct{}{}
	ps.streamMu.Unlock()

	// Fill the channel
	ch <- PatrolStreamEvent{Type: "full"}

	// Broadcast another event - this should hit the default case and mark for removal
	ps.broadcast(PatrolStreamEvent{Type: "overflow"})

	// Verify the channel was removed from subscribers
	ps.streamMu.RLock()
	_, exists := ps.streamSubscribers[ch]
	ps.streamMu.RUnlock()

	if exists {
		t.Error("Expected channel to be removed from subscribers after full broadcast")
	}
}

func TestPatrolService_CheckAnomalies(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	// Case 1: No baseline store
	findings := ps.checkAnomalies("res1", "name1", "node", map[string]float64{"cpu": 50})
	if findings != nil {
		t.Error("Expected nil findings when baseline store is nil")
	}

	// Case 2: With baseline store
	bs := baseline.NewStore(baseline.StoreConfig{MinSamples: 1})

	// Set some baselines
	pts := []baseline.MetricPoint{
		{Value: 10, Timestamp: time.Now().Add(-1 * time.Hour)},
		{Value: 10, Timestamp: time.Now()},
	}
	// We need enough samples to satisfy minSamples. Default for NewStore is 50.
	// Let's use 1 to make it easy.
	bs.Learn("res1", "node", "cpu", pts)
	bs.Learn("res1", "node", "memory", pts)
	bs.Learn("res1", "node", "disk", pts)

	ps.mu.Lock()
	ps.baselineStore = bs
	ps.mu.Unlock()

	// Metric values that should trigger High/Critical anomalies
	// Need to check baseline.go to see what z-scores correspond to High (3-4) and Critical (>4)
	// zScore = (value - mean) / stddev
	// Since pts are all 10, mean=10, stddev=0.
	// When stddev=0, CheckAnomaly returns AnomalyMedium if absDiff > 5.
	// Wait, I want to test High and Critical.

	// Let's set some variance
	ptsVar := []baseline.MetricPoint{
		{Value: 10, Timestamp: time.Now().Add(-10 * time.Hour)},
		{Value: 20, Timestamp: time.Now().Add(-9 * time.Hour)},
		{Value: 10, Timestamp: time.Now().Add(-8 * time.Hour)},
		{Value: 20, Timestamp: time.Now().Add(-7 * time.Hour)},
		{Value: 15, Timestamp: time.Now().Add(-6 * time.Hour)},
	}
	bs.Learn("res1", "node", "cpu", ptsVar) // Mean ~15, StdDev ~5

	metrics := map[string]float64{
		"cpu":    100, // (100-15)/5 = 17 -> Critical
		"normal": 15,
	}

	findings = ps.checkAnomalies("res1", "name1", "node", metrics)

	if len(findings) == 0 {
		t.Error("Expected findings for anomalous CPU")
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

func TestPatrolService_ValidateAIFindings(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	state := models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "node1", Name: "Node 1", CPU: 0.1, Memory: models.Memory{Total: 1000, Used: 100}}, // CPU 10%, Mem 10%
		},
		VMs: []models.VM{
			{ID: "vm1", Name: "VM 1", CPU: 0.95, Memory: models.Memory{Usage: 95}, Disk: models.Disk{Usage: 95}}, // 95% across board
		},
		Containers: []models.Container{
			{ID: "ct1", Name: "CT 1", CPU: 0.2, Memory: models.Memory{Usage: 30}, Disk: models.Disk{Usage: 40}}, // Low usage
		},
		Storage: []models.Storage{
			{ID: "st1", Name: "ST 1", Total: 1000, Used: 900}, // 90% usage
		},
	}

	findings := []*Finding{
		nil, // Should be ignored
		{
			ID:           "f1",
			Key:          "cpu-high",
			Title:        "High CPU on Node 1",
			ResourceID:   "node1",
			ResourceName: "Node 1",
			Severity:     FindingSeverityWarning,
			Category:     FindingCategoryPerformance,
		}, // Should be filtered (actual 10% < 50%)
		{
			ID:           "f2",
			Key:          "cpu-high",
			Title:        "High CPU on VM 1",
			ResourceID:   "vm1",
			ResourceName: "VM 1",
			Severity:     FindingSeverityWarning,
			Category:     FindingCategoryPerformance,
		}, // Should be kept (actual 95% > 50%)
		{
			ID:           "f3",
			Key:          "unknown",
			Title:        "Generic Issue",
			ResourceID:   "unknown-res",
			ResourceName: "Unknown",
			Severity:     FindingSeverityWarning,
			Category:     FindingCategoryPerformance,
		}, // Should be kept (benefit of doubt)
		{
			ID:           "f4",
			Key:          "cpu-high",
			Title:        "Critical CPU",
			ResourceID:   "node1",
			ResourceName: "Node 1",
			Severity:     FindingSeverityCritical,
			Category:     FindingCategoryPerformance,
		}, // Should be kept (Critical severity)
	}

	validated := ps.validateAIFindings(findings, state)

	// Expected: f2, f3, f4
	if len(validated) != 3 {
		t.Errorf("Expected 3 validated findings, got %d", len(validated))
	}

	// Verify specific ones
	foundF2 := false
	foundF3 := false
	foundF4 := false
	for _, v := range validated {
		if v.ID == "f2" {
			foundF2 = true
		}
		if v.ID == "f3" {
			foundF3 = true
		}
		if v.ID == "f4" {
			foundF4 = true
		}
	}

	if !foundF2 || !foundF3 || !foundF4 {
		t.Error("Missing expected findings in validated output")
	}
}

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
