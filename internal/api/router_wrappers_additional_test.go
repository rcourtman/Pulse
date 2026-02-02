package api

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/baseline"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/patterns"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/proxmox"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/metrics"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

func TestForecastStateProviderWrapper_GetState(t *testing.T) {
	state := models.NewState()
	state.VMs = []models.VM{{ID: "vm-1", Name: "vm-one"}}
	state.Containers = []models.Container{{ID: "ct-1", Name: "ct-one"}}
	state.Nodes = []models.Node{{ID: "node-1", Name: "node-one"}}
	state.Storage = []models.Storage{{ID: "store-1", Name: "store-one"}}

	monitor := &monitoring.Monitor{}
	setUnexportedField(t, monitor, "state", state)

	wrapper := &forecastStateProviderWrapper{monitor: monitor}
	snapshot := wrapper.GetState()

	if len(snapshot.VMs) != 1 || snapshot.VMs[0].ID != "vm-1" || snapshot.VMs[0].Name != "vm-one" {
		t.Fatalf("unexpected VM snapshot: %#v", snapshot.VMs)
	}
	if len(snapshot.Containers) != 1 || snapshot.Containers[0].ID != "ct-1" || snapshot.Containers[0].Name != "ct-one" {
		t.Fatalf("unexpected container snapshot: %#v", snapshot.Containers)
	}
	if len(snapshot.Nodes) != 1 || snapshot.Nodes[0].ID != "node-1" || snapshot.Nodes[0].Name != "node-one" {
		t.Fatalf("unexpected node snapshot: %#v", snapshot.Nodes)
	}
	if len(snapshot.Storage) != 1 || snapshot.Storage[0].ID != "store-1" || snapshot.Storage[0].Name != "store-one" {
		t.Fatalf("unexpected storage snapshot: %#v", snapshot.Storage)
	}
}

func TestForecastStateProviderWrapper_NilMonitor(t *testing.T) {
	wrapper := &forecastStateProviderWrapper{}
	snapshot := wrapper.GetState()

	if len(snapshot.VMs) != 0 || len(snapshot.Containers) != 0 || len(snapshot.Nodes) != 0 || len(snapshot.Storage) != 0 {
		t.Fatalf("expected empty snapshot, got %#v", snapshot)
	}
}

func TestIncidentRecorderProviderWrapper(t *testing.T) {
	now := time.Now().UTC()
	end := now.Add(5 * time.Minute)

	activeWindow := &metrics.IncidentWindow{
		ID:           "win-active",
		ResourceID:   "res-1",
		ResourceName: "Resource",
		ResourceType: "vm",
		TriggerType:  "alert",
		TriggerID:    "alert-1",
		StartTime:    now,
		EndTime:      &end,
		Status:       metrics.IncidentWindowStatusRecording,
		DataPoints: []metrics.IncidentDataPoint{
			{Timestamp: now, Metrics: map[string]float64{"cpu": 10}},
		},
		Summary: &metrics.IncidentSummary{
			Duration:   5 * time.Minute,
			DataPoints: 1,
			Peaks:      map[string]float64{"cpu": 10},
			Lows:       map[string]float64{"cpu": 5},
			Averages:   map[string]float64{"cpu": 7},
			Changes:    map[string]float64{"cpu": 2},
		},
	}

	completedWindow := &metrics.IncidentWindow{
		ID:           "win-complete",
		ResourceID:   "res-1",
		ResourceName: "Resource",
		ResourceType: "vm",
		TriggerType:  "alert",
		TriggerID:    "alert-2",
		StartTime:    now.Add(-time.Hour),
		Status:       metrics.IncidentWindowStatusComplete,
		DataPoints: []metrics.IncidentDataPoint{
			{Timestamp: now.Add(-time.Hour), Metrics: map[string]float64{"cpu": 20}},
		},
	}

	recorder := metrics.NewIncidentRecorder(metrics.DefaultIncidentRecorderConfig())
	setUnexportedField(t, recorder, "activeWindows", map[string]*metrics.IncidentWindow{"win-active": activeWindow})
	setUnexportedField(t, recorder, "completedWindows", []*metrics.IncidentWindow{completedWindow})

	wrapper := &incidentRecorderProviderWrapper{recorder: recorder}
	windows := wrapper.GetWindowsForResource("res-1", 10)
	if len(windows) != 2 {
		t.Fatalf("expected 2 windows, got %d", len(windows))
	}

	ids := []string{windows[0].ID, windows[1].ID}
	if !containsStringSlice(ids, "win-active") || !containsStringSlice(ids, "win-complete") {
		t.Fatalf("unexpected window ids %v", ids)
	}

	window := wrapper.GetWindow("win-active")
	if window == nil || window.ResourceID != "res-1" || window.Status == "" {
		t.Fatalf("unexpected window: %#v", window)
	}
}

func TestIncidentRecorderProviderWrapper_NilRecorder(t *testing.T) {
	wrapper := &incidentRecorderProviderWrapper{}
	if got := wrapper.GetWindowsForResource("res-1", 5); got != nil {
		t.Fatalf("expected nil windows, got %#v", got)
	}
	if got := wrapper.GetWindow("win-1"); got != nil {
		t.Fatalf("expected nil window, got %#v", got)
	}
}

func TestConvertIncidentWindowNil(t *testing.T) {
	if got := convertIncidentWindow(nil); got != nil {
		t.Fatalf("expected nil window, got %#v", got)
	}
}

func TestEventCorrelatorProviderWrapper(t *testing.T) {
	now := time.Now().UTC()
	corr := proxmox.EventCorrelation{
		ID: "corr-1",
		Event: proxmox.ProxmoxEvent{
			ID:           "evt-1",
			Type:         proxmox.EventBackupStart,
			Timestamp:    now,
			ResourceID:   "res-1",
			ResourceName: "Resource",
		},
		Anomalies:   []proxmox.MetricAnomaly{{ResourceID: "res-1", Metric: "cpu", Value: 10}},
		Explanation: "Backup caused spike",
		Confidence:  0.75,
		ImpactedResources: []string{
			"res-1",
		},
	}

	correlator := proxmox.NewEventCorrelator(proxmox.EventCorrelatorConfig{})
	setUnexportedField(t, correlator, "correlations", []proxmox.EventCorrelation{corr})

	wrapper := &eventCorrelatorProviderWrapper{correlator: correlator}
	results := wrapper.GetCorrelationsForResource("res-1", 30*time.Minute)
	if len(results) != 1 {
		t.Fatalf("expected 1 correlation, got %d", len(results))
	}

	result := results[0]
	if result.EventType != string(corr.Event.Type) || result.ResourceID != "res-1" || result.Description == "" {
		t.Fatalf("unexpected correlation: %#v", result)
	}
	if result.Metadata["event_id"] != "evt-1" {
		t.Fatalf("expected metadata event_id, got %#v", result.Metadata)
	}
}

func TestEventCorrelatorProviderWrapper_NilCorrelator(t *testing.T) {
	wrapper := &eventCorrelatorProviderWrapper{}
	if got := wrapper.GetCorrelationsForResource("res-1", time.Minute); got != nil {
		t.Fatalf("expected nil correlations, got %#v", got)
	}
}

func TestMetricsSourceWrapper(t *testing.T) {
	history := monitoring.NewMetricsHistory(10, time.Hour)
	now := time.Now()
	history.AddGuestMetric("guest-1", "cpu", 0.5, now)
	history.AddGuestMetric("guest-1", "memory", 0.7, now)
	history.AddNodeMetric("node-1", "cpu", 0.2, now)

	wrapper := &metricsSourceWrapper{history: history}
	guestCPU := wrapper.GetGuestMetrics("guest-1", "cpu", time.Hour)
	if len(guestCPU) != 1 || guestCPU[0].Value != 0.5 {
		t.Fatalf("unexpected guest metrics: %#v", guestCPU)
	}

	nodeCPU := wrapper.GetNodeMetrics("node-1", "cpu", time.Hour)
	if len(nodeCPU) != 1 || nodeCPU[0].Value != 0.2 {
		t.Fatalf("unexpected node metrics: %#v", nodeCPU)
	}

	allGuest := wrapper.GetAllGuestMetrics("guest-1", time.Hour)
	if len(allGuest) == 0 || len(allGuest["cpu"]) != 1 {
		t.Fatalf("unexpected all guest metrics: %#v", allGuest)
	}
}

func TestConvertMetricPoints(t *testing.T) {
	now := time.Now()
	points := []monitoring.MetricPoint{{Value: 1.23, Timestamp: now}}

	got := convertMetricPoints(points)
	if len(got) != 1 || got[0].Value != 1.23 || !got[0].Timestamp.Equal(now) {
		t.Fatalf("unexpected metric points: %#v", got)
	}
}

func TestBaselineSourceWrapper(t *testing.T) {
	store := baseline.NewStore(baseline.StoreConfig{MinSamples: 1})
	now := time.Now()
	points := []baseline.MetricPoint{{Value: 1.0, Timestamp: now}}
	if err := store.Learn("res-1", "vm", "cpu", points); err != nil {
		t.Fatalf("learn baseline: %v", err)
	}

	wrapper := &baselineSourceWrapper{store: store}
	mean, stddev, samples, ok := wrapper.GetBaseline("res-1", "cpu")
	if !ok || samples != 1 || mean != 1.0 || stddev != 0 {
		t.Fatalf("unexpected baseline: mean=%v stddev=%v samples=%d ok=%v", mean, stddev, samples, ok)
	}

	all := wrapper.GetAllBaselines()
	if all == nil {
		t.Fatalf("expected baselines map, got nil")
	}
	resourceBaselines, ok := all["res-1"]
	if !ok {
		t.Fatalf("expected baselines for res-1, got %#v", all)
	}
	cpuBaseline, ok := resourceBaselines["cpu"]
	if !ok || cpuBaseline.SampleCount != 1 {
		t.Fatalf("unexpected cpu baseline: %#v", cpuBaseline)
	}
}

func TestBaselineSourceWrapper_NilStore(t *testing.T) {
	wrapper := &baselineSourceWrapper{}
	if _, _, _, ok := wrapper.GetBaseline("res-1", "cpu"); ok {
		t.Fatalf("expected baseline lookup to fail")
	}
	if got := wrapper.GetAllBaselines(); got != nil {
		t.Fatalf("expected nil baselines, got %#v", got)
	}
}

func TestPatternSourceWrapper(t *testing.T) {
	detector := patterns.NewDetector(patterns.DetectorConfig{MinOccurrences: 1})
	now := time.Now()
	pattern := &patterns.Pattern{
		ResourceID:     "res-1",
		EventType:      patterns.EventType("reboot"),
		Occurrences:    2,
		Confidence:     0.9,
		LastOccurrence: now.Add(-time.Hour),
		NextPredicted:  now.Add(24 * time.Hour),
	}

	patternsMap := map[string]*patterns.Pattern{
		"res-1:reboot": pattern,
		"nil-pattern":  nil,
	}
	setUnexportedField(t, detector, "patterns", patternsMap)

	wrapper := &patternSourceWrapper{detector: detector}
	gotPatterns := wrapper.GetPatterns()
	if len(gotPatterns) != 1 {
		t.Fatalf("expected 1 pattern, got %d", len(gotPatterns))
	}
	if gotPatterns[0].ResourceID != "res-1" || gotPatterns[0].PatternType != "reboot" {
		t.Fatalf("unexpected pattern: %#v", gotPatterns[0])
	}

	predictions := wrapper.GetPredictions()
	if len(predictions) != 1 || predictions[0].ResourceID != "res-1" {
		t.Fatalf("unexpected predictions: %#v", predictions)
	}
}

func TestPatternSourceWrapper_NilDetector(t *testing.T) {
	wrapper := &patternSourceWrapper{}
	if got := wrapper.GetPatterns(); got != nil {
		t.Fatalf("expected nil patterns, got %#v", got)
	}
	if got := wrapper.GetPredictions(); got != nil {
		t.Fatalf("expected nil predictions, got %#v", got)
	}
}

func TestUpdatesConfigWrapper(t *testing.T) {
	wrapper := &updatesConfigWrapper{}
	if !wrapper.IsDockerUpdateActionsEnabled() {
		t.Fatalf("expected docker update actions enabled by default")
	}

	wrapper = &updatesConfigWrapper{cfg: &config.Config{DisableDockerUpdateActions: true}}
	if wrapper.IsDockerUpdateActionsEnabled() {
		t.Fatalf("expected docker update actions disabled")
	}
}

func containsStringSlice(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func TestConvertMetricPoints_Empty(t *testing.T) {
	if got := convertMetricPoints(nil); len(got) != 0 {
		t.Fatalf("expected empty slice, got %#v", got)
	}
}
