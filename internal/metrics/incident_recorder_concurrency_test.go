package metrics

import (
	"sync"
	"testing"
	"time"
)

func TestGenerateWindowIDConcurrentUnique(t *testing.T) {
	t.Parallel()

	const (
		workers    = 16
		idsPerWork = 128
	)

	ids := make(chan string, workers*idsPerWork)
	start := make(chan struct{})

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for j := 0; j < idsPerWork; j++ {
				ids <- generateWindowID("res-1")
			}
		}()
	}

	close(start)
	wg.Wait()
	close(ids)

	seen := make(map[string]struct{}, workers*idsPerWork)
	for id := range ids {
		if _, exists := seen[id]; exists {
			t.Fatalf("duplicate window ID generated: %s", id)
		}
		seen[id] = struct{}{}
	}
}

func TestIncidentRecorderConcurrentStartStopAndFlush(t *testing.T) {
	recorder := NewIncidentRecorder(IncidentRecorderConfig{
		SampleInterval:         time.Millisecond,
		PreIncidentWindow:      10 * time.Millisecond,
		PostIncidentWindow:     10 * time.Millisecond,
		MaxDataPointsPerWindow: 10,
		DataDir:                t.TempDir(),
	})
	provider := &stubMetricsProvider{
		metricsByID: map[string]map[string]float64{
			"res-1": {"cpu": 1},
		},
		ids: []string{"res-1"},
	}
	recorder.SetMetricsProvider(provider)

	const goroutines = 8
	const iterations = 15
	start := make(chan struct{})

	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for j := 0; j < iterations; j++ {
				recorder.Start()
				windowID := recorder.StartRecording("res-1", "db", "host", "alert", "a-1")
				recorder.recordSample()
				recorder.StopRecording(windowID)
				recorder.Stop()
			}
		}()
	}

	close(start)
	wg.Wait()

	// Final stop should remain idempotent after concurrent shutdowns.
	recorder.Stop()
}
