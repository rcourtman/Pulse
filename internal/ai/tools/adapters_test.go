package tools

import (
	"errors"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	ur "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

type fakeAlertManager struct {
	alerts   []alerts.Alert
	resolved []models.ResolvedAlert
}

func (f fakeAlertManager) GetActiveAlerts() []alerts.Alert {
	return f.alerts
}

func (f fakeAlertManager) GetRecentlyResolved() []models.ResolvedAlert {
	return f.resolved
}

type fakeMetricsSource struct {
	allGuest map[string]map[string][]RawMetricPoint
	guest    map[string]map[string][]RawMetricPoint
	node     map[string]map[string][]RawMetricPoint
}

func (f *fakeMetricsSource) GetGuestMetrics(guestID string, metricType string, _ time.Duration) []RawMetricPoint {
	if f.guest == nil {
		return nil
	}
	if byMetric, ok := f.guest[guestID]; ok {
		return byMetric[metricType]
	}
	return nil
}

func (f *fakeMetricsSource) GetNodeMetrics(nodeID string, metricType string, _ time.Duration) []RawMetricPoint {
	if f.node == nil {
		return nil
	}
	if byMetric, ok := f.node[nodeID]; ok {
		return byMetric[metricType]
	}
	return nil
}

func (f *fakeMetricsSource) GetAllGuestMetrics(guestID string, _ time.Duration) map[string][]RawMetricPoint {
	if f.allGuest == nil {
		return nil
	}
	return f.allGuest[guestID]
}

type fakeBaselineSource struct {
	mean   float64
	stddev float64
	ok     bool
	all    map[string]map[string]BaselineData
}

func (f *fakeBaselineSource) GetBaseline(_ string, _ string) (float64, float64, int, bool) {
	return f.mean, f.stddev, 10, f.ok
}

func (f *fakeBaselineSource) GetAllBaselines() map[string]map[string]BaselineData {
	return f.all
}

type fakePatternSource struct {
	patterns    []PatternData
	predictions []PredictionData
}

func (f *fakePatternSource) GetPatterns() []PatternData {
	return f.patterns
}

func (f *fakePatternSource) GetPredictions() []PredictionData {
	return f.predictions
}

type fakeFindingsManager struct {
	resolveArgs []string
	dismissArgs []string
	resolveErr  error
	dismissErr  error
}

func (f *fakeFindingsManager) ResolveFinding(findingID, note string) error {
	f.resolveArgs = []string{findingID, note}
	return f.resolveErr
}

func (f *fakeFindingsManager) DismissFinding(findingID, reason, note string) error {
	f.dismissArgs = []string{findingID, reason, note}
	return f.dismissErr
}

type fakeMetadataUpdater struct {
	resourceArgs []string
	err          error
}

func (f *fakeMetadataUpdater) SetResourceURL(resourceType, resourceID, url string) error {
	f.resourceArgs = []string{resourceType, resourceID, url}
	return f.err
}

type fakeUpdatesCommandRunner struct {
	checkStatus  models.DockerHostCommandStatus
	updateStatus models.DockerHostCommandStatus
	checkErr     error
	updateErr    error
}

func (f *fakeUpdatesCommandRunner) QueueDockerCheckUpdatesCommand(_ string) (models.DockerHostCommandStatus, error) {
	return f.checkStatus, f.checkErr
}

func (f *fakeUpdatesCommandRunner) QueueDockerContainerUpdateCommand(_ string, _ string, _ string) (models.DockerHostCommandStatus, error) {
	return f.updateStatus, f.updateErr
}

type fakeUpdatesConfig struct {
	enabled bool
}

func (f *fakeUpdatesConfig) IsDockerUpdateActionsEnabled() bool {
	return f.enabled
}

func TestAlertManagerMCPAdapter(t *testing.T) {
	if NewAlertManagerMCPAdapter(nil) != nil {
		t.Fatal("expected nil adapter for nil manager")
	}

	ts := time.Now()
	manager := fakeAlertManager{
		alerts: []alerts.Alert{
			{
				ID:           "a1",
				ResourceID:   "vm-1",
				ResourceName: "vm1",
				Type:         "cpu",
				Level:        alerts.AlertLevelWarning,
				Value:        80,
				Threshold:    70,
				StartTime:    ts,
				Message:      "high cpu",
			},
		},
	}

	adapter := NewAlertManagerMCPAdapter(manager)
	got := adapter.GetActiveAlerts()
	if len(got) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(got))
	}
	if got[0].Severity != "warning" || got[0].ResourceName != "vm1" || got[0].Message != "high cpu" {
		t.Fatalf("unexpected alert mapping: %+v", got[0])
	}
}

func TestBackupAndDiskAdapters(t *testing.T) {
	backups := models.Backups{PVE: models.PVEBackups{
		BackupTasks: []models.BackupTask{{ID: "task1"}},
	}}
	pbsInstances := []models.PBSInstance{{ID: "pbs1"}}
	repJobs := []models.ReplicationJob{{ID: "rep1"}}

	if NewBackupMCPAdapter(nil, nil) != nil {
		t.Fatal("expected nil backup adapter for nil getters")
	}
	// Partial-nil: one getter nil should also return nil
	if NewBackupMCPAdapter(func() models.Backups { return models.Backups{} }, nil) != nil {
		t.Fatal("expected nil backup adapter when getPBSInstances is nil")
	}
	if NewBackupMCPAdapter(nil, func() []models.PBSInstance { return nil }) != nil {
		t.Fatal("expected nil backup adapter when getBackups is nil")
	}
	backupAdapter := NewBackupMCPAdapter(
		func() models.Backups { return backups },
		func() []models.PBSInstance { return pbsInstances },
	)
	if len(backupAdapter.GetBackups().PVE.BackupTasks) != 1 {
		t.Fatal("expected backup tasks")
	}
	if len(backupAdapter.GetPBSInstances()) != 1 {
		t.Fatal("expected pbs instances")
	}

	if NewDiskHealthMCPAdapter(nil) != nil {
		t.Fatal("expected nil disk health adapter for nil read state")
	}
	rs := &fakeReadState{
		hosts: []*ur.HostView{
			newHostView("host-resource-1", "Host 1", "host1", "host-1", nil, nil, nil),
		},
	}
	diskAdapter := NewDiskHealthMCPAdapter(
		rs,
	)
	if len(diskAdapter.GetHosts()) != 1 {
		t.Fatal("expected hosts")
	}

	if NewReplicationMCPAdapter(nil) != nil {
		t.Fatal("expected nil replication adapter for nil getter")
	}
	replicationAdapter := NewReplicationMCPAdapter(
		func() []models.ReplicationJob { return repJobs },
	)
	if len(replicationAdapter.GetReplicationJobs()) != 1 {
		t.Fatal("expected replication jobs")
	}

	if NewConnectionHealthMCPAdapter(nil) != nil {
		t.Fatal("expected nil connection health adapter for nil getter")
	}
	connectionHealthAdapter := NewConnectionHealthMCPAdapter(
		func() map[string]bool { return nil },
	)
	if connectionHealthAdapter.GetConnectionHealth() != nil {
		t.Fatal("expected nil connection health map")
	}
}

func TestMetricsHistoryMCPAdapter(t *testing.T) {
	now := time.Now()
	points := map[string][]RawMetricPoint{
		"cpu":    {{Value: 10, Timestamp: now}},
		"memory": {{Value: 20, Timestamp: now}},
	}
	source := &fakeMetricsSource{
		allGuest: map[string]map[string][]RawMetricPoint{
			"100": points,
		},
	}

	rs := &fakeReadState{}
	adapter := NewMetricsHistoryMCPAdapter(source, rs)
	got, err := adapter.GetResourceMetrics("100", time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].CPU != 10 || got[0].Memory != 20 {
		t.Fatalf("unexpected merged metrics: %+v", got)
	}

	// Node fallback when guest metrics empty
	source = &fakeMetricsSource{
		allGuest: map[string]map[string][]RawMetricPoint{},
		node: map[string]map[string][]RawMetricPoint{
			"node1": {
				"cpu":    {{Value: 5, Timestamp: now}},
				"memory": {{Value: 15, Timestamp: now}},
			},
		},
	}
	adapter = NewMetricsHistoryMCPAdapter(source, rs)
	got, err = adapter.GetResourceMetrics("node1", time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].CPU != 5 || got[0].Memory != 15 {
		t.Fatalf("unexpected node metrics: %+v", got)
	}

	adapter = &MetricsHistoryMCPAdapter{}
	empty, err := adapter.GetResourceMetrics("missing", time.Hour)
	if err != nil || empty != nil {
		t.Fatal("expected nil metrics when source missing")
	}
}

func TestMetricsSummaryAndHelpers(t *testing.T) {
	now := time.Now()
	source := &fakeMetricsSource{
		guest: map[string]map[string][]RawMetricPoint{
			"100": {
				"cpu":    {{Value: 10, Timestamp: now}, {Value: 20, Timestamp: now.Add(time.Minute)}},
				"memory": {{Value: 30, Timestamp: now}},
			},
			"200": {
				"cpu":    {{Value: 5, Timestamp: now}},
				"memory": {{Value: 15, Timestamp: now}},
			},
		},
		node: map[string]map[string][]RawMetricPoint{
			"node1": {
				"cpu":    {{Value: 0, Timestamp: now}, {Value: 10, Timestamp: now.Add(time.Minute)}},
				"memory": {{Value: 5, Timestamp: now}},
			},
		},
	}

	rs := &fakeReadState{
		vms:        []*ur.VMView{newVMView("vm-100", "vm1", 100)},
		containers: []*ur.ContainerView{newContainerView("ct-200", "ct1", 200)},
		nodes:      []*ur.NodeView{newNodeView("reg-node-hash", "node-1", "node1")},
	}

	adapter := NewMetricsHistoryMCPAdapter(source, rs)
	summary, err := adapter.GetAllMetricsSummary(time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(summary) != 3 {
		t.Fatalf("expected summaries for vm, system-container, and node, got %d", len(summary))
	}
	if summary["100"].ResourceName != "vm1" || summary["node1"].ResourceName != "node-1" {
		t.Fatalf("unexpected summary names: %+v", summary)
	}
	if summary["200"].ResourceType != "system-container" {
		t.Fatalf("expected canonical system-container type for container summary, got %q", summary["200"].ResourceType)
	}

	merged := mergeMetricsByTimestamp(map[string][]RawMetricPoint{
		"cpu":    {{Value: 1, Timestamp: now}},
		"memory": {{Value: 2, Timestamp: now}},
		"disk":   {{Value: 3, Timestamp: now.Add(time.Minute)}},
	})
	if len(merged) != 2 {
		t.Fatalf("expected 2 merged points, got %d", len(merged))
	}

	avg, max := computeStats([]RawMetricPoint{{Value: 1}, {Value: 3}})
	if avg != 2 || max != 3 {
		t.Fatalf("unexpected stats avg=%v max=%v", avg, max)
	}

	if computeTrend([]RawMetricPoint{{Value: 1}}) != "stable" {
		t.Fatal("expected stable trend for short series")
	}
	if computeTrend([]RawMetricPoint{{Value: 0}, {Value: 0}, {Value: 0}, {Value: 10}}) != "growing" {
		t.Fatal("expected growing trend")
	}
	if computeTrend([]RawMetricPoint{{Value: 10}, {Value: 10}, {Value: 10}, {Value: 0}}) != "declining" {
		t.Fatal("expected declining trend")
	}
	if computeTrend([]RawMetricPoint{{Value: 1}, {Value: 2}, {Value: 2}, {Value: 2}}) != "stable" {
		t.Fatal("expected stable trend within threshold")
	}
}

func TestBaselineMCPAdapter(t *testing.T) {
	if NewBaselineMCPAdapter(nil) != nil {
		t.Fatal("expected nil baseline adapter for nil source")
	}
	adapter := NewBaselineMCPAdapter(&fakeBaselineSource{mean: 10, stddev: 2, ok: true})
	baseline := adapter.GetBaseline("vm1", "cpu")
	if baseline == nil || baseline.Min != 6 || baseline.Max != 14 {
		t.Fatalf("unexpected baseline: %+v", baseline)
	}

	adapter = &BaselineMCPAdapter{}
	if adapter.GetBaseline("vm1", "cpu") != nil {
		t.Fatal("expected nil baseline when source missing")
	}

	adapter = NewBaselineMCPAdapter(&fakeBaselineSource{ok: false})
	if adapter.GetBaseline("vm1", "cpu") != nil {
		t.Fatal("expected nil baseline when not found")
	}

	adapter = NewBaselineMCPAdapter(&fakeBaselineSource{all: nil})
	if adapter.GetAllBaselines() != nil {
		t.Fatal("expected nil baselines when source returns nil")
	}

	all := map[string]map[string]BaselineData{
		"100": {"cpu": {Mean: 5, StdDev: 1}},
	}
	adapter = NewBaselineMCPAdapter(&fakeBaselineSource{all: all})
	allBaselines := adapter.GetAllBaselines()
	if allBaselines["100"]["cpu"].Min != 3 || allBaselines["100"]["cpu"].Max != 7 {
		t.Fatalf("unexpected all baselines: %+v", allBaselines)
	}
}

func TestPatternMCPAdapter(t *testing.T) {
	rs := &fakeReadState{
		vms:        []*ur.VMView{newVMView("vm-100", "vm1", 100)},
		containers: []*ur.ContainerView{newContainerView("ct-200", "ct1", 200)},
		nodes:      []*ur.NodeView{newNodeView("reg-node-hash", "node-1", "node1")},
	}
	source := &fakePatternSource{
		patterns: []PatternData{
			{ResourceID: "100", PatternType: "cpu", Description: "spike"},
			{ResourceID: "node1", PatternType: "disk", Description: "trend"},
		},
		predictions: []PredictionData{
			{ResourceID: "200", IssueType: "memory", Recommendation: "scale"},
		},
	}

	adapter := NewPatternMCPAdapter(source, rs)
	patterns := adapter.GetPatterns()
	if len(patterns) != 2 || patterns[0].ResourceName != "vm1" || patterns[1].ResourceName != "node-1" {
		t.Fatalf("unexpected patterns: %+v", patterns)
	}
	predictions := adapter.GetPredictions()
	if len(predictions) != 1 || predictions[0].ResourceName != "ct1" {
		t.Fatalf("unexpected predictions: %+v", predictions)
	}

	adapter = NewPatternMCPAdapter(source, nil)
	patterns = adapter.GetPatterns()
	if patterns[0].ResourceName != "100" {
		t.Fatal("expected resource ID when readState missing")
	}
}

func TestFindingsAndMetadataAdapters(t *testing.T) {
	manager := &fakeFindingsManager{resolveErr: errors.New("resolve"), dismissErr: errors.New("dismiss")}
	adapter := NewFindingsManagerMCPAdapter(manager)
	if err := adapter.ResolveFinding("f1", "note"); err == nil {
		t.Fatal("expected resolve error")
	}
	if err := adapter.DismissFinding("f1", "reason", "note"); err == nil {
		t.Fatal("expected dismiss error")
	}
	if len(manager.resolveArgs) != 2 || len(manager.dismissArgs) != 3 {
		t.Fatal("expected args to be captured")
	}

	adapter = &FindingsManagerMCPAdapter{}
	if err := adapter.ResolveFinding("f1", "note"); err == nil {
		t.Fatal("expected error when manager missing")
	}

	updater := &fakeMetadataUpdater{err: errors.New("update")}
	meta := NewMetadataUpdaterMCPAdapter(updater)
	if err := meta.SetResourceURL("vm", "1", "http://x"); err == nil {
		t.Fatal("expected update error")
	}
	if len(updater.resourceArgs) != 3 {
		t.Fatal("expected resource args captured")
	}
	meta = &MetadataUpdaterMCPAdapter{}
	if err := meta.SetResourceURL("vm", "1", "http://x"); err == nil {
		t.Fatal("expected error when metadata updater missing")
	}
}

func TestUpdatesMCPAdapter(t *testing.T) {
	if NewUpdatesMCPAdapter(nil, nil, nil) != nil {
		t.Fatal("expected nil updates adapter for nil getters")
	}
	// Partial-nil: either read state or commands nil should return nil.
	if NewUpdatesMCPAdapter(&fakeReadState{}, nil, nil) != nil {
		t.Fatal("expected nil updates adapter when commands is nil")
	}
	if NewUpdatesMCPAdapter(nil, &fakeUpdatesCommandRunner{}, nil) != nil {
		t.Fatal("expected nil updates adapter when readState is nil")
	}

	now := time.Now()
	rs := &fakeReadState{
		dockerHosts: []*ur.DockerHostView{
			newDockerHostView("docker-resource-1", "host1", "Host 1", "h1"),
			newDockerHostView("docker-resource-2", "host2", "Host 2", "h2"),
		},
		dockerContainers: []*ur.DockerContainerView{
			newDockerContainerView(
				"docker-container-resource-1",
				"docker-resource-1",
				"host1",
				"/nginx",
				"c1",
				"nginx:latest",
				&ur.DockerUpdateStatusMeta{
					UpdateAvailable: true,
					CurrentDigest:   "old",
					LatestDigest:    "new",
					LastChecked:     now,
				},
			),
			newDockerContainerView(
				"docker-container-resource-2",
				"docker-resource-2",
				"host2",
				"redis",
				"c2",
				"redis:latest",
				&ur.DockerUpdateStatusMeta{
					Error: "rate limited",
				},
			),
		},
	}

	runner := &fakeUpdatesCommandRunner{}
	adapter := NewUpdatesMCPAdapter(
		rs,
		runner,
		&fakeUpdatesConfig{enabled: false},
	)

	updates := adapter.GetPendingUpdates("host1")
	if len(updates) != 1 || updates[0].ContainerName != "nginx" {
		t.Fatalf("unexpected updates: %+v", updates)
	}

	if adapter.IsUpdateActionsEnabled() {
		t.Fatal("expected updates disabled")
	}
	if (&UpdatesMCPAdapter{}).IsUpdateActionsEnabled() != true {
		t.Fatal("expected updates enabled by default")
	}

	runner.checkErr = errors.New("check")
	if _, err := adapter.TriggerUpdateCheck("host1"); err == nil {
		t.Fatal("expected check error")
	}

	runner.checkErr = nil
	runner.checkStatus = models.DockerHostCommandStatus{ID: "cmd1", Type: "check", Status: "queued"}
	status, err := adapter.TriggerUpdateCheck("host1")
	if err != nil || status.ID != "cmd1" {
		t.Fatalf("unexpected status: %+v err=%v", status, err)
	}

	runner.updateErr = errors.New("update")
	if _, err := adapter.UpdateContainer("host1", "c1", "nginx"); err == nil {
		t.Fatal("expected update error")
	}

	runner.updateErr = nil
	runner.updateStatus = models.DockerHostCommandStatus{ID: "cmd2", Type: "update", Status: "queued"}
	status, err = adapter.UpdateContainer("host1", "c1", "nginx")
	if err != nil || status.ID != "cmd2" {
		t.Fatalf("unexpected update status: %+v err=%v", status, err)
	}

	if trimContainerName("/redis") != "redis" || trimContainerName("plain") != "plain" {
		t.Fatal("unexpected trim result")
	}
}
