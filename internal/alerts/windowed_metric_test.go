package alerts

import (
	"math"
	"testing"
	"time"
)

func TestCalculateMetricWindowUsesTimeWeightedAverage(t *testing.T) {
	end := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	start := end.Add(-5 * time.Minute)
	points := []MetricWindowPoint{
		{Timestamp: start, Value: 40},
		{Timestamp: start.Add(2 * time.Minute), Value: 40},
		{Timestamp: start.Add(4 * time.Minute), Value: 40},
		{Timestamp: end, Value: 100},
	}

	got := calculateMetricWindow(points, start, end, 100, 300)
	if !got.Ready {
		t.Fatal("expected complete window to be ready")
	}
	if math.Abs(got.Value-46) > 0.001 {
		t.Fatalf("expected a time-weighted average of 46, got %.3f", got.Value)
	}
	if got.SampleCount != 4 || got.CoverageSeconds != 300 {
		t.Fatalf("unexpected evidence: %+v", got)
	}
}

func TestCalculateMetricWindowRejectsIncompleteAndGappedEvidence(t *testing.T) {
	end := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	start := end.Add(-5 * time.Minute)

	for name, points := range map[string][]MetricWindowPoint{
		"too few samples": {
			{Timestamp: start, Value: 90},
			{Timestamp: end, Value: 90},
		},
		"insufficient coverage": {
			{Timestamp: end.Add(-2 * time.Minute), Value: 90},
			{Timestamp: end.Add(-time.Minute), Value: 90},
			{Timestamp: end, Value: 90},
		},
		"large internal gap": {
			{Timestamp: start, Value: 90},
			{Timestamp: start.Add(30 * time.Second), Value: 90},
			{Timestamp: end, Value: 90},
		},
	} {
		t.Run(name, func(t *testing.T) {
			if got := calculateMetricWindow(points, start, end, 90, 300); got.Ready {
				t.Fatalf("expected weak evidence to remain unknown, got %+v", got)
			}
		})
	}
}

func TestWindowedMetricSpikeDoesNotFireAndMetadataExplainsAverage(t *testing.T) {
	m := NewManagerWithDataDir(t.TempDir())
	defer m.Stop()
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	m.now = func() time.Time { return now }
	cfg := m.GetConfig()
	cfg.ActivationState = ActivationActive
	cfg.MetricEvaluationWindows = map[string]map[string]int{"all": {"cpu": 300}}
	m.UpdateConfig(cfg)
	m.SetMetricWindowProvider(func(request MetricWindowRequest) ([]MetricWindowPoint, error) {
		return []MetricWindowPoint{
			{Timestamp: request.Start, Value: 40},
			{Timestamp: request.Start.Add(2 * time.Minute), Value: 40},
			{Timestamp: request.Start.Add(4 * time.Minute), Value: 40},
		}, nil
	})

	threshold := &HysteresisThreshold{Trigger: 80, Clear: 75}
	m.checkMetric("vm-1", "web-1", "node-1", "pve-1", "vm", "cpu", 100, threshold, nil)
	if alerts := m.GetActiveAlerts(); len(alerts) != 0 {
		t.Fatalf("expected a short CPU spike to be filtered, got %+v", alerts)
	}

	m.SetMetricWindowProvider(func(request MetricWindowRequest) ([]MetricWindowPoint, error) {
		return []MetricWindowPoint{
			{Timestamp: request.Start, Value: 90},
			{Timestamp: request.Start.Add(2 * time.Minute), Value: 90},
			{Timestamp: request.Start.Add(4 * time.Minute), Value: 90},
		}, nil
	})
	m.checkMetric("vm-1", "web-1", "node-1", "pve-1", "vm", "cpu", 90, threshold, nil)
	now = now.Add(6 * time.Second)
	m.checkMetric("vm-1", "web-1", "node-1", "pve-1", "vm", "cpu", 90, threshold, nil)
	active := m.GetActiveAlerts()
	if len(active) != 1 {
		t.Fatalf("expected sustained CPU to fire, got %+v", active)
	}
	alert := active[0]
	if alert.Metadata["evaluationMode"] != "rolling_average" || alert.Metadata["evaluationWindowSeconds"] != 300 {
		t.Fatalf("missing rolling-window evidence: %+v", alert.Metadata)
	}
	if alert.Message == "" || alert.Value != 90 {
		t.Fatalf("unexpected rolling alert presentation: %+v", alert)
	}
}

func TestWindowedMetricUnknownHistoryPreservesActiveIncident(t *testing.T) {
	m := NewManagerWithDataDir(t.TempDir())
	defer m.Stop()
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	m.now = func() time.Time { return now }
	cfg := m.GetConfig()
	cfg.ActivationState = ActivationActive
	cfg.MetricEvaluationWindows = map[string]map[string]int{"all": {"cpu": 300}}
	m.UpdateConfig(cfg)
	complete := true
	m.SetMetricWindowProvider(func(request MetricWindowRequest) ([]MetricWindowPoint, error) {
		if !complete {
			return nil, nil
		}
		return []MetricWindowPoint{
			{Timestamp: request.Start, Value: 90},
			{Timestamp: request.Start.Add(2 * time.Minute), Value: 90},
			{Timestamp: request.Start.Add(4 * time.Minute), Value: 90},
		}, nil
	})
	threshold := &HysteresisThreshold{Trigger: 80, Clear: 75}
	m.checkMetric("vm-1", "web-1", "node-1", "pve-1", "vm", "cpu", 90, threshold, nil)
	now = now.Add(6 * time.Second)
	m.checkMetric("vm-1", "web-1", "node-1", "pve-1", "vm", "cpu", 90, threshold, nil)
	complete = false
	now = now.Add(time.Minute)
	m.checkMetric("vm-1", "web-1", "node-1", "pve-1", "vm", "cpu", 10, threshold, nil)
	if active := m.GetActiveAlerts(); len(active) != 1 {
		t.Fatalf("expected unknown history to preserve the incident, got %+v", active)
	}
}
