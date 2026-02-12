package ai

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/baseline"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/knowledge"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

type precomputeMetricsHistoryProvider struct {
	metrics map[string][]models.MetricPoint
	storage map[string][]models.MetricPoint
}

func (p *precomputeMetricsHistoryProvider) GetNodeMetrics(nodeID string, metricType string, duration time.Duration) []models.MetricPoint {
	return p.metrics[nodeID+":"+metricType]
}

func (p *precomputeMetricsHistoryProvider) GetGuestMetrics(guestID string, metricType string, duration time.Duration) []models.MetricPoint {
	return p.metrics[guestID+":"+metricType]
}

func (p *precomputeMetricsHistoryProvider) GetAllGuestMetrics(guestID string, duration time.Duration) map[string][]models.MetricPoint {
	return nil
}

func (p *precomputeMetricsHistoryProvider) GetAllStorageMetrics(storageID string, duration time.Duration) map[string][]models.MetricPoint {
	if p.storage == nil {
		return nil
	}
	points := p.storage[storageID+":usage"]
	if len(points) == 0 {
		return nil
	}
	return map[string][]models.MetricPoint{"usage": points}
}

func TestSeedPrecomputeIntelligence_PopulatesSignals(t *testing.T) {
	now := time.Now()
	ps := NewPatrolService(nil, nil)

	bs := baseline.NewStore(baseline.StoreConfig{MinSamples: 1})
	baselinePoints := []baseline.MetricPoint{
		{Value: 10, Timestamp: now.Add(-5 * time.Hour)},
		{Value: 10, Timestamp: now.Add(-4 * time.Hour)},
		{Value: 10, Timestamp: now.Add(-3 * time.Hour)},
		{Value: 10, Timestamp: now.Add(-2 * time.Hour)},
		{Value: 10, Timestamp: now.Add(-1 * time.Hour)},
	}
	_ = bs.Learn("node-1", "node", "cpu", baselinePoints)
	_ = bs.Learn("node-1", "node", "memory", baselinePoints)
	_ = bs.Learn("vm-1", "vm", "memory", baselinePoints)
	_ = bs.Learn("vm-1", "vm", "disk", baselinePoints)
	_ = bs.Learn("ct-1", "container", "memory", baselinePoints)
	_ = bs.Learn("ct-1", "container", "disk", baselinePoints)
	_ = bs.Learn("storage-1", "storage", "usage", baselinePoints)
	ps.SetBaselineStore(bs)

	series := func(start float64) []models.MetricPoint {
		return []models.MetricPoint{
			{Value: start, Timestamp: now.Add(-5 * time.Hour)},
			{Value: start + 2, Timestamp: now.Add(-4 * time.Hour)},
			{Value: start + 4, Timestamp: now.Add(-3 * time.Hour)},
			{Value: start + 6, Timestamp: now.Add(-2 * time.Hour)},
			{Value: start + 8, Timestamp: now.Add(-1 * time.Hour)},
		}
	}
	mh := &precomputeMetricsHistoryProvider{
		metrics: map[string][]models.MetricPoint{
			"node-1:memory": series(70),
			"vm-1:memory":   series(60),
			"vm-1:disk":     series(50),
			"ct-1:memory":   series(55),
			"ct-1:disk":     series(45),
		},
		storage: map[string][]models.MetricPoint{
			"storage-1:usage": series(65),
		},
	}
	ps.SetMetricsHistoryProvider(mh)

	patternCfg := DefaultPatternConfig()
	patternCfg.MinOccurrences = 2
	patternCfg.PredictionLimit = 72 * time.Hour
	pd := NewPatternDetector(patternCfg)
	pd.RecordEvent(HistoricalEvent{ResourceID: "vm-1", EventType: EventHighCPU, Timestamp: now.Add(-36 * time.Hour)})
	pd.RecordEvent(HistoricalEvent{ResourceID: "vm-1", EventType: EventHighCPU, Timestamp: now.Add(-12 * time.Hour)})
	ps.SetPatternDetector(pd)

	cd := NewChangeDetector(ChangeDetectorConfig{MaxChanges: 10})
	cd.DetectChanges([]ResourceSnapshot{{ID: "vm-1", Name: "vm-1", Type: "vm", Status: "running", SnapshotTime: now.Add(-2 * time.Hour)}})
	cd.DetectChanges([]ResourceSnapshot{{ID: "vm-1", Name: "vm-1", Type: "vm", Status: "stopped", SnapshotTime: now.Add(-1 * time.Hour)}})
	ps.SetChangeDetector(cd)

	corrCfg := DefaultCorrelationConfig()
	corrCfg.MinOccurrences = 1
	corrCfg.CorrelationWindow = time.Hour
	corr := NewCorrelationDetector(corrCfg)
	for i := 0; i < 3; i++ {
		base := now.Add(time.Duration(-30+i*5) * time.Minute)
		corr.RecordEvent(CorrelationEvent{ResourceID: "node-1", ResourceName: "node-1", ResourceType: "node", EventType: CorrelationEventHighCPU, Timestamp: base})
		corr.RecordEvent(CorrelationEvent{ResourceID: "vm-1", ResourceName: "vm-1", ResourceType: "vm", EventType: CorrelationEventRestart, Timestamp: base.Add(2 * time.Minute)})
	}
	ps.SetCorrelationDetector(corr)

	state := models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "node-1", Name: "node-1", CPU: 80, Memory: models.Memory{Usage: 80}},
		},
		VMs: []models.VM{
			{ID: "vm-1", Name: "vm-1", Status: "running", CPU: 20, Memory: models.Memory{Usage: 70}, Disk: models.Disk{Usage: 60}},
		},
		Containers: []models.Container{
			{ID: "ct-1", Name: "ct-1", Status: "running", CPU: 10, Memory: models.Memory{Usage: 65}, Disk: models.Disk{Usage: 55}},
		},
		Storage: []models.Storage{
			{ID: "storage-1", Name: "local", Usage: 85},
		},
		ActiveAlerts: []models.Alert{{ID: "alert-1"}},
	}

	scoped := map[string]bool{"node-1": true, "vm-1": true, "ct-1": true, "storage-1": true}
	intel := ps.seedPrecomputeIntelligence(state, scoped, now)

	if !intel.hasBaselineStore {
		t.Fatalf("expected baseline store flag to be true")
	}
	if len(intel.anomalies) == 0 {
		t.Fatalf("expected anomalies to be populated")
	}
	nameSet := false
	for _, a := range intel.anomalies {
		if a.ResourceName != "" {
			nameSet = true
			break
		}
	}
	if !nameSet {
		t.Fatalf("expected anomaly resource names to be set")
	}
	if len(intel.forecasts) == 0 {
		t.Fatalf("expected capacity forecasts")
	}
	if len(intel.predictions) == 0 {
		t.Fatalf("expected failure predictions")
	}
	if len(intel.recentChanges) == 0 {
		t.Fatalf("expected recent changes")
	}
	if len(intel.correlations) == 0 {
		t.Fatalf("expected correlations")
	}
	if intel.isQuiet {
		t.Fatalf("expected infrastructure to be non-quiet")
	}
}

func TestSeedBackupAnalysis_StaleAndRecent(t *testing.T) {
	now := time.Now()
	ps := NewPatrolService(nil, nil)

	state := models.StateSnapshot{
		VMs: []models.VM{
			{ID: "vm-1", VMID: 101, Name: "vm-1", Template: false, LastBackup: now.Add(-24 * time.Hour)},
			{ID: "vm-2", VMID: 102, Name: "vm-2", Template: false},
		},
		Containers: []models.Container{{ID: "ct-1", VMID: 201, Name: "ct-1", Template: false}},
		PVEBackups: models.PVEBackups{
			BackupTasks:    []models.BackupTask{{VMID: 102, Status: "OK", EndTime: now.Add(-72 * time.Hour)}},
			StorageBackups: []models.StorageBackup{{VMID: 101, Time: now.Add(-36 * time.Hour)}},
		},
		PBSBackups: []models.PBSBackup{{VMID: "102", BackupTime: now.Add(-72 * time.Hour)}},
	}

	output := ps.seedBackupAnalysis(state, now)
	if output == "" {
		t.Fatalf("expected backup analysis output")
	}
	if !strings.Contains(output, "Guests with no backup in >48h") {
		t.Fatalf("expected stale backup section, got: %s", output)
	}
	if !strings.Contains(output, "ct-1 (never)") {
		t.Fatalf("expected never-backed guest, got: %s", output)
	}
	if !strings.Contains(output, "vm-2 (last:") {
		t.Fatalf("expected stale vm entry, got: %s", output)
	}
	if !strings.Contains(output, "Guests with recent backups: 1/3") {
		t.Fatalf("expected recent backup count, got: %s", output)
	}
}

func TestSeedFindingsAndContext_ResolvesMissingAndAddsNotes(t *testing.T) {
	now := time.Now()
	ps := NewPatrolService(nil, nil)

	missing := &Finding{
		ID:           "find-missing",
		Severity:     FindingSeverityInfo,
		Category:     FindingCategoryPerformance,
		ResourceID:   "vm-missing",
		ResourceName: "vm-missing",
		Title:        "Missing VM",
		Description:  "no longer exists",
		DetectedAt:   now.Add(-2 * time.Hour),
		LastSeenAt:   now.Add(-2 * time.Hour),
	}
	active := &Finding{
		ID:           "find-active",
		Severity:     FindingSeverityInfo,
		Category:     FindingCategoryPerformance,
		ResourceID:   "node-1",
		ResourceName: "node-1",
		Title:        "High CPU",
		Description:  "cpu high",
		UserNote:     "keep an eye",
		DetectedAt:   now.Add(-1 * time.Hour),
		LastSeenAt:   now.Add(-1 * time.Hour),
	}
	dismissed := &Finding{
		ID:           "find-dismissed",
		Severity:     FindingSeverityInfo,
		Category:     FindingCategoryPerformance,
		ResourceID:   "node-1",
		ResourceName: "node-1",
		Title:        "Noisy alerts",
		Description:  "expected",
		DetectedAt:   now.Add(-30 * time.Minute),
		LastSeenAt:   now.Add(-30 * time.Minute),
	}

	ps.findings.Add(missing)
	ps.findings.Add(active)
	ps.findings.Add(dismissed)
	ps.findings.Dismiss(dismissed.ID, "expected_behavior", "known workload")

	resolvedID := ""
	ps.unifiedFindingResolver = func(findingID string) {
		resolvedID = findingID
	}

	knowledgeStore, err := knowledge.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create knowledge store: %v", err)
	}
	if err := knowledgeStore.SaveNote("node-1", "node-1", "node", "config", "Pinned", "keep settings"); err != nil {
		t.Fatalf("failed to save knowledge note: %v", err)
	}
	ps.knowledgeStore = knowledgeStore

	state := models.StateSnapshot{Nodes: []models.Node{{ID: "node-1", Name: "node-1"}}}
	output, seeded := ps.seedFindingsAndContext(&PatrolScope{ResourceIDs: []string{"node-1"}}, state)

	if resolvedID != missing.ID {
		t.Fatalf("expected unified resolver to be called for missing finding")
	}
	if missing.ResolvedAt == nil {
		t.Fatalf("expected missing finding to be resolved")
	}
	if len(seeded) != 1 || seeded[0] != active.ID {
		t.Fatalf("expected active finding to be seeded")
	}
	if !strings.Contains(output, "Active Findings to Re-check") {
		t.Fatalf("expected active findings section, got: %s", output)
	}
	if !strings.Contains(output, "User note: \"keep an eye\"") {
		t.Fatalf("expected user note in output, got: %s", output)
	}
	if !strings.Contains(output, "User Feedback on Previous Findings") {
		t.Fatalf("expected dismissed findings context, got: %s", output)
	}
	if !strings.Contains(output, "# User Notes") || !strings.Contains(output, "Saved Knowledge") {
		t.Fatalf("expected knowledge context, got: %s", output)
	}
}
