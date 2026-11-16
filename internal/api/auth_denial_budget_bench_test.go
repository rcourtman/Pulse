package api

import (
	"fmt"
	"testing"
	"time"
)

// Each operation is one fixed-size burst of previously unseen callers. Keys
// and clock input are prepared before timing so the benchmark measures the
// shared tracker, including its normal map/counter allocations and housekeeping.
func BenchmarkAuthDenialBudget(b *testing.B) {
	for _, n := range []int{512, authDenialMaxTracked} {
		for _, full := range []bool{false, true} {
			b.Run(fmt.Sprintf("callers=%d/full=%t", n, full), func(b *testing.B) {
				keys := make([]string, n)
				for i := range keys {
					keys[i] = fmt.Sprintf("new-%d", i)
				}
				seedKeys := make([]string, authDenialMaxTracked)
				for i := range seedKeys {
					seedKeys[i] = fmt.Sprintf("seed-%d", i)
				}
				now := time.Unix(1_700_000_000, 0)
				oldNow := authDenialNow
				authDenialNow = func() time.Time { return now }
				b.Cleanup(func() {
					authDenialNow = oldNow
					authDenialMu.Lock()
					authDenials = make(map[string]*authDenialCounter)
					authDenialMu.Unlock()
				})
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					// Identical fresh state per burst; saturated cases include
					// the cost of removing the oldest live half at capacity.
					b.StopTimer()
					authDenialMu.Lock()
					authDenials = make(map[string]*authDenialCounter)
					if full {
						for j, key := range seedKeys {
							authDenials[key] = &authDenialCounter{windowStart: now.Add(-time.Second + time.Duration(j))}
						}
					}
					authDenialMu.Unlock()
					b.StartTimer()
					for _, key := range keys {
						recordAuthDenial(key)
					}
				}
				b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*n), "ns/denial")
			})
		}
	}
}
