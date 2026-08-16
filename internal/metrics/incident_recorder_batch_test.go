package metrics

import (
	"errors"
	"testing"
	"time"
)

// gappyBatchProvider implements both MetricsProvider and BatchMetricsProvider
// but omits one monitored ID from the batch. The recorder must fall through
// to the per-ID method for that ID so provider semantics are preserved.
type gappyBatchProvider struct {
	batchCalls   int
	perIDCalls   map[string]int
	perIDResults map[string]map[string]float64
	perIDErr     map[string]error
}

func (p *gappyBatchProvider) GetMonitoredResourceIDs() []string {
	return []string{"in-batch", "not-in-batch", "erroring"}
}

func (p *gappyBatchProvider) GetCurrentMetricsBatch() map[string]map[string]float64 {
	p.batchCalls++
	return map[string]map[string]float64{
		"in-batch": {"cpu": 42},
	}
}

func (p *gappyBatchProvider) GetCurrentMetrics(resourceID string) (map[string]float64, error) {
	if p.perIDCalls == nil {
		p.perIDCalls = map[string]int{}
	}
	p.perIDCalls[resourceID]++
	if err := p.perIDErr[resourceID]; err != nil {
		return nil, err
	}
	return p.perIDResults[resourceID], nil
}

func TestRecordSampleBatchMissFallsThroughToPerID(t *testing.T) {
	recorder := NewIncidentRecorder(IncidentRecorderConfig{
		SampleInterval:         time.Hour, // ticks driven manually
		PreIncidentWindow:      time.Minute,
		PostIncidentWindow:     time.Minute,
		MaxDataPointsPerWindow: 5,
	})
	provider := &gappyBatchProvider{
		perIDResults: map[string]map[string]float64{
			"not-in-batch": {"cpu": 7},
		},
		perIDErr: map[string]error{
			"erroring": errors.New("unknown resource"),
		},
	}
	recorder.SetMetricsProvider(provider)

	recorder.recordSample()

	if provider.batchCalls != 1 {
		t.Fatalf("batch calls = %d, want 1", provider.batchCalls)
	}
	if provider.perIDCalls["in-batch"] != 0 {
		t.Fatalf("batch hit %q still took the per-ID path", "in-batch")
	}
	if provider.perIDCalls["not-in-batch"] != 1 || provider.perIDCalls["erroring"] != 1 {
		t.Fatalf("batch misses did not fall through per-ID: %+v", provider.perIDCalls)
	}

	recorder.mu.RLock()
	defer recorder.mu.RUnlock()
	if got := len(recorder.preIncidentBuffer["not-in-batch"]); got != 1 {
		t.Fatalf("fallthrough metrics not buffered: %d points", got)
	}
	if got := recorder.preIncidentBuffer["not-in-batch"][0].Metrics["cpu"]; got != 7 {
		t.Fatalf("fallthrough buffered cpu = %v, want 7 (per-ID value)", got)
	}
	if got := len(recorder.preIncidentBuffer["erroring"]); got != 0 {
		t.Fatalf("erroring ID gained %d buffered points, want 0 (per-ID error must skip)", got)
	}
	if got := len(recorder.preIncidentBuffer["in-batch"]); got != 1 {
		t.Fatalf("batch-served ID not buffered: %d points", got)
	}
}
