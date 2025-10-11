package monitoring

import (
	"math/rand"
	"sync"
	"testing"
	"time"
)

func TestMetricsHistoryConcurrentAccess(t *testing.T) {
	mh := NewMetricsHistory(256, time.Minute)

	const iterations = 1000

	var wg sync.WaitGroup
	wg.Add(4)

	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			mh.AddGuestMetric("guest-1", "cpu", rand.Float64()*100, time.Now())
			mh.AddGuestMetric("guest-1", "memory", rand.Float64()*100, time.Now())
			mh.AddGuestMetric("guest-2", "disk", rand.Float64()*100, time.Now())
			time.Sleep(time.Microsecond)
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			mh.AddNodeMetric("node-1", "cpu", rand.Float64()*100, time.Now())
			mh.AddNodeMetric("node-1", "memory", rand.Float64()*100, time.Now())
			mh.AddStorageMetric("storage-1", "usage", rand.Float64()*100, time.Now())
			time.Sleep(time.Microsecond)
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			mh.GetGuestMetrics("guest-1", "cpu", time.Minute)
			mh.GetGuestMetrics("guest-1", "memory", time.Minute)
			mh.GetGuestMetrics("guest-2", "disk", time.Minute)
			time.Sleep(time.Microsecond)
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			mh.GetNodeMetrics("node-1", "cpu", time.Minute)
			mh.GetNodeMetrics("node-1", "memory", time.Minute)
			// Storage metrics are exposed through Monitor, but simulate reads via guest metrics map
			mh.GetGuestMetrics("storage-1", "usage", time.Minute)
			time.Sleep(time.Microsecond)
		}
	}()

	wg.Wait()
}

func init() {
	rand.Seed(time.Now().UnixNano())
}
