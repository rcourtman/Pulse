package monitoring

import (
	"math/rand"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func TestMetricsHistoryConcurrentAccess(t *testing.T) {
	mh := NewMetricsHistory(256, time.Minute)

	const iterations = 1000

	var wg sync.WaitGroup
	wg.Add(5)

	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			mh.AddGuestMetric("guest-1", "cpu", rand.Float64()*100, time.Now())
			mh.AddGuestMetric("guest-1", "memory", rand.Float64()*100, time.Now())
			mh.AddGuestMetric("guest-1", "memoryused", 4096, time.Now())
			mh.AddGuestMetric("guest-2", "disk", rand.Float64()*100, time.Now())
			time.Sleep(time.Microsecond)
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			mh.AddNodeMetric("node-1", "cpu", rand.Float64()*100, time.Now())
			mh.AddNodeMetric("node-1", "memory", rand.Float64()*100, time.Now())
			mh.AddNodeMetric("node-1", "memoryused", 8192, time.Now())
			mh.AddStorageMetric("storage-1", "usage", rand.Float64()*100, time.Now())
			time.Sleep(time.Microsecond)
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			mh.GetGuestMetrics("guest-1", "cpu", time.Minute)
			mh.GetGuestMetrics("guest-1", "memory", time.Minute)
			mh.GetGuestMetrics("guest-1", "memoryused", time.Minute)
			mh.GetGuestMetrics("guest-2", "disk", time.Minute)
			time.Sleep(time.Microsecond)
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			mh.GetNodeMetrics("node-1", "cpu", time.Minute)
			mh.GetNodeMetrics("node-1", "memory", time.Minute)
			mh.GetNodeMetrics("node-1", "memoryused", time.Minute)
			// Storage metrics are exposed through Monitor, but simulate reads via guest metrics map
			mh.GetGuestMetrics("storage-1", "usage", time.Minute)
			time.Sleep(time.Microsecond)
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			mh.Cleanup()
			time.Sleep(time.Microsecond)
		}
	}()

	wg.Wait()
	if len(mh.GetGuestMetrics("guest-1", "memoryused", time.Minute)) == 0 ||
		len(mh.GetNodeMetrics("node-1", "memoryused", time.Minute)) == 0 {
		t.Fatal("concurrent cleanup lost live used-memory samples")
	}
}

func TestMetricsHistoryCleanupLoggingDoesNotBlockHistory(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		now := time.Now()
		history := NewMetricsHistory(1000, time.Hour)
		// Insert via the real writer while the point is fresh, then age it
		// out. An unrelated node still has recent CPU readings.
		history.AddGuestMetric("expired", "cpu", 1, now)
		time.Sleep(2 * time.Hour)
		history.AddNodeMetric("live", "cpu", 7, time.Now())
		time.Sleep(time.Minute)

		// The package logger is shared. Do not run in parallel, and wait
		// for all work to exit before restoring it.
		saved, level := log.Logger, zerolog.GlobalLevel()
		sink := blockedCacheExpiryLog{
			message: "Cleaned up stale metrics history entries",
			entered: make(chan struct{}),
			resume:  make(chan struct{}),
		}
		log.Logger = zerolog.New(sink).Level(zerolog.DebugLevel)
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
		done := make(chan struct{})
		go func() {
			defer close(done)
			history.Cleanup()
		}()
		defer func() {
			close(sink.resume)
			<-done
			synctest.Wait()
			log.Logger = saved
			zerolog.SetGlobalLevel(level)
		}()
		synctest.Wait()
		select {
		case <-sink.entered:
		default:
			t.Fatal("cleanup did not emit the expiry diagnostic")
		}

		var before, after []MetricPoint
		var written bool
		m := &Monitor{metricsHistory: history}
		go func() {
			before = m.GetNodeMetricsForChart("live", "cpu", time.Hour)
		}()
		synctest.Wait()
		if len(before) != 1 || before[0].Value != 7 {
			t.Error("chart could not read the retained point while cleanup was logging")
		}
		go func() {
			history.AddNodeMetric("live", "cpu", 42, time.Now())
			written = true
			after = m.GetNodeMetricsForChart("live", "cpu", time.Hour)
		}()
		synctest.Wait()
		if !written || len(after) != 2 || after[0].Value != 7 || after[1].Value != 42 {
			t.Error("node sample write and chart read did not finish while cleanup was logging")
		}
		if !history.mu.TryRLock() {
			t.Fatal("history remained locked after the sweep")
		}
		_, retained := history.guestMetrics["expired"]
		history.mu.RUnlock()
		if retained {
			t.Error("expired entry was still present when its removal was logged")
		}
	})
}

// rand is automatically seeded in Go 1.20+
