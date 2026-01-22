package tools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/stretchr/testify/mock"
)

type stubBaselineProvider struct {
	baselines map[string]map[string]*MetricBaseline
}

func (s *stubBaselineProvider) GetBaseline(resourceID, metric string) *MetricBaseline {
	if s.baselines == nil {
		return nil
	}
	if metrics, ok := s.baselines[resourceID]; ok {
		return metrics[metric]
	}
	return nil
}

func (s *stubBaselineProvider) GetAllBaselines() map[string]map[string]*MetricBaseline {
	return s.baselines
}

type stubPatternProvider struct {
	patterns    []Pattern
	predictions []Prediction
}

func (s *stubPatternProvider) GetPatterns() []Pattern {
	return s.patterns
}

func (s *stubPatternProvider) GetPredictions() []Prediction {
	return s.predictions
}

type stubFindingsManager struct {
	resolveErr error
	dismissErr error
}

func (s *stubFindingsManager) ResolveFinding(string, string) error {
	return s.resolveErr
}

func (s *stubFindingsManager) DismissFinding(string, string, string) error {
	return s.dismissErr
}

func TestExecuteGetMetrics(t *testing.T) {
	executor := NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{}})
	result, _ := executor.executeGetMetrics(context.Background(), map[string]interface{}{"period": "24h"})
	if result.IsError || result.Content[0].Text == "" {
		t.Fatal("expected metrics not available message")
	}

	metricsProv := &mockMetricsHistoryProvider{}
	metricsProv.On("GetAllMetricsSummary", mock.Anything).Return(map[string]ResourceMetricsSummary{
		"res1": {ResourceID: "res1"},
	}, nil)
	metricsProv.On("GetResourceMetrics", "res1", mock.Anything).Return([]MetricPoint{
		{CPU: 1, Memory: 2},
	}, nil)
	executor.metricsHistory = metricsProv
	result, _ = executor.executeGetMetrics(context.Background(), map[string]interface{}{
		"period":      "bad",
		"resource_id": "res1",
	})
	var resp MetricsResponse
	if err := json.Unmarshal([]byte(result.Content[0].Text), &resp); err != nil {
		t.Fatalf("decode metrics response: %v", err)
	}
	if resp.ResourceID != "res1" || len(resp.Points) != 1 {
		t.Fatalf("unexpected metrics response: %+v", resp)
	}

	result, _ = executor.executeGetMetrics(context.Background(), map[string]interface{}{
		"period": "7d",
	})
	if err := json.Unmarshal([]byte(result.Content[0].Text), &resp); err != nil {
		t.Fatalf("decode metrics response: %v", err)
	}
	if resp.Summary == nil || resp.Period != "7d" {
		t.Fatalf("unexpected metrics summary: %+v", resp)
	}
}

func TestExecuteGetBaselinesAndPatterns(t *testing.T) {
	executor := NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{}})
	result, _ := executor.executeGetBaselines(context.Background(), map[string]interface{}{})
	if result.IsError {
		t.Fatal("expected baselines not available message")
	}

	executor.baselineProvider = &stubBaselineProvider{
		baselines: map[string]map[string]*MetricBaseline{
			"res1": {"cpu": {Mean: 1}},
		},
	}
	result, _ = executor.executeGetBaselines(context.Background(), map[string]interface{}{
		"resource_id": "res1",
	})
	var baselines BaselinesResponse
	if err := json.Unmarshal([]byte(result.Content[0].Text), &baselines); err != nil {
		t.Fatalf("decode baselines: %v", err)
	}
	if baselines.ResourceID != "res1" || baselines.Baselines["res1"]["cpu"].Mean != 1 {
		t.Fatalf("unexpected baselines: %+v", baselines)
	}

	result, _ = executor.executeGetPatterns(context.Background(), map[string]interface{}{})
	if result.IsError {
		t.Fatal("expected patterns not available message")
	}

	executor.patternProvider = &stubPatternProvider{
		patterns:    []Pattern{{ResourceID: "r1"}},
		predictions: []Prediction{{ResourceID: "r2"}},
	}
	result, _ = executor.executeGetPatterns(context.Background(), map[string]interface{}{})
	var patterns PatternsResponse
	if err := json.Unmarshal([]byte(result.Content[0].Text), &patterns); err != nil {
		t.Fatalf("decode patterns: %v", err)
	}
	if len(patterns.Patterns) != 1 || len(patterns.Predictions) != 1 {
		t.Fatalf("unexpected patterns: %+v", patterns)
	}
}

func TestExecuteListResolvedAlerts(t *testing.T) {
	executor := NewPulseToolExecutor(ExecutorConfig{})
	result, _ := executor.executeListResolvedAlerts(context.Background(), map[string]interface{}{})
	if result.Content[0].Text != "State provider not available." {
		t.Fatalf("unexpected response: %s", result.Content[0].Text)
	}

	executor.stateProvider = &mockStateProvider{state: models.StateSnapshot{}}
	result, _ = executor.executeListResolvedAlerts(context.Background(), map[string]interface{}{})
	if result.Content[0].Text != "No recently resolved alerts." {
		t.Fatalf("unexpected response: %s", result.Content[0].Text)
	}

	now := time.Now()
	executor.stateProvider = &mockStateProvider{state: models.StateSnapshot{
		RecentlyResolved: []models.ResolvedAlert{
			{
				Alert: models.Alert{
					ID:           "a1",
					Type:         "cpu",
					Level:        "warning",
					ResourceID:   "r1",
					ResourceName: "node1",
					Node:         "node1",
					Instance:     "i1",
					Message:      "msg",
					Value:        1,
					Threshold:    2,
					StartTime:    now,
				},
				ResolvedTime: now,
			},
			{
				Alert: models.Alert{
					ID:           "a2",
					Type:         "disk",
					Level:        "critical",
					ResourceID:   "r2",
					ResourceName: "node2",
					Node:         "node2",
					Instance:     "i2",
					Message:      "msg2",
					Value:        3,
					Threshold:    4,
					StartTime:    now,
				},
				ResolvedTime: now,
			},
		},
	}}

	result, _ = executor.executeListResolvedAlerts(context.Background(), map[string]interface{}{
		"type":  "cpu",
		"level": "warning",
		"limit": 1,
	})

	var resp ResolvedAlertsResponse
	if err := json.Unmarshal([]byte(result.Content[0].Text), &resp); err != nil {
		t.Fatalf("decode resolved alerts: %v", err)
	}
	if len(resp.Alerts) != 1 || resp.Alerts[0].ID != "a1" {
		t.Fatalf("unexpected resolved alerts: %+v", resp)
	}
}

func TestExecuteListAlertsAndFindings(t *testing.T) {
	executor := NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{}})
	result, _ := executor.executeListAlerts(context.Background(), map[string]interface{}{})
	if result.IsError {
		t.Fatal("expected alerts not available message")
	}

	alertProv := &mockAlertProvider{}
	alertProv.On("GetActiveAlerts").Return([]ActiveAlert{
		{ID: "a1", Severity: "warning"},
		{ID: "a2", Severity: "critical"},
	})
	executor.alertProvider = alertProv
	result, _ = executor.executeListAlerts(context.Background(), map[string]interface{}{
		"severity": "critical",
		"limit":    float64(1),
	})
	var alerts AlertsResponse
	if err := json.Unmarshal([]byte(result.Content[0].Text), &alerts); err != nil {
		t.Fatalf("decode alerts: %v", err)
	}
	if alerts.Count != 1 || alerts.Alerts[0].ID != "a2" {
		t.Fatalf("unexpected alerts: %+v", alerts)
	}

	findingsProv := &mockFindingsProvider{}
	findingsProv.On("GetActiveFindings").Return([]Finding{{ID: "f1", Severity: "warning"}})
	findingsProv.On("GetDismissedFindings").Return([]Finding{{ID: "f2", Severity: "info"}})
	executor.findingsProvider = findingsProv
	result, _ = executor.executeListFindings(context.Background(), map[string]interface{}{
		"include_dismissed": true,
		"severity":          "warning",
	})
	var findings FindingsResponse
	if err := json.Unmarshal([]byte(result.Content[0].Text), &findings); err != nil {
		t.Fatalf("decode findings: %v", err)
	}
	if findings.Counts.Active != 1 || findings.Counts.Dismissed != 0 {
		t.Fatalf("unexpected counts: %+v", findings.Counts)
	}
	if len(findings.Active) != 1 || len(findings.Dismissed) != 0 {
		t.Fatalf("unexpected findings: %+v", findings)
	}
}

func TestExecuteResolveAndDismissFinding(t *testing.T) {
	executor := NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{}})
	result, _ := executor.executeResolveFinding(context.Background(), map[string]interface{}{})
	if !result.IsError {
		t.Fatal("expected error without findings manager")
	}

	executor.findingsManager = &stubFindingsManager{resolveErr: errors.New("resolve")}
	result, _ = executor.executeResolveFinding(context.Background(), map[string]interface{}{
		"finding_id":      "f1",
		"resolution_note": "note",
	})
	if !result.IsError {
		t.Fatal("expected resolve error")
	}

	executor.findingsManager = &stubFindingsManager{dismissErr: errors.New("dismiss")}
	result, _ = executor.executeDismissFinding(context.Background(), map[string]interface{}{
		"finding_id": "f1",
		"reason":     "not_an_issue",
		"note":       "note",
	})
	if !result.IsError {
		t.Fatal("expected dismiss error")
	}

	executor.findingsManager = &stubFindingsManager{}
	result, _ = executor.executeDismissFinding(context.Background(), map[string]interface{}{
		"finding_id": "f1",
		"reason":     "not_an_issue",
		"note":       "note",
	})
	var okResp map[string]interface{}
	if err := json.Unmarshal([]byte(result.Content[0].Text), &okResp); err != nil {
		t.Fatalf("decode dismiss response: %v", err)
	}
	if okResp["success"] != true {
		t.Fatalf("unexpected dismiss response: %+v", okResp)
	}
}
