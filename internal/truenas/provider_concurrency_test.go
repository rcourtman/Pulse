package truenas

import (
	"sync"
	"testing"
)

func TestFeatureFlagConcurrentAccess(t *testing.T) {
	previous := IsFeatureEnabled()
	t.Cleanup(func() {
		SetFeatureEnabled(previous)
	})

	const readers = 8
	const writers = 4
	const iterations = 2000

	var wg sync.WaitGroup
	start := make(chan struct{})

	for i := 0; i < writers; i++ {
		writerID := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for j := 0; j < iterations; j++ {
				SetFeatureEnabled((writerID+j)%2 == 0)
			}
		}()
	}

	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for j := 0; j < iterations; j++ {
				_ = IsFeatureEnabled()
			}
		}()
	}

	close(start)
	wg.Wait()
}
