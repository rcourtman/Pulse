package monitoring

import (
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestRateTrackerConcurrentAccess(t *testing.T) {
	rt := NewRateTracker()

	const iterations = 1000

	var wg sync.WaitGroup
	wg.Add(3)

	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			metrics := models.IOMetrics{
				DiskRead:   int64(i * 100),
				DiskWrite:  int64(i * 80),
				NetworkIn:  int64(i * 60),
				NetworkOut: int64(i * 40),
				Timestamp:  time.Now().Add(time.Duration(i) * time.Millisecond),
			}
			rt.CalculateRates("guest-1", metrics)
			time.Sleep(time.Microsecond)
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			metrics := models.IOMetrics{
				DiskRead:   int64(i * 50),
				DiskWrite:  int64(i * 70),
				NetworkIn:  int64(i * 30),
				NetworkOut: int64(i * 20),
				Timestamp:  time.Now().Add(time.Duration(i) * time.Millisecond),
			}
			rt.CalculateRates("guest-2", metrics)
			time.Sleep(time.Microsecond)
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < iterations/10; i++ {
			rt.Clear()
			time.Sleep(5 * time.Microsecond)
		}
	}()

	wg.Wait()
}
