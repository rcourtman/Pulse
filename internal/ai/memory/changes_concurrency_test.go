package memory

import (
	"sync"
	"testing"
)

func TestGenerateChangeID_ConcurrentUnique(t *testing.T) {
	changeCounter.Store(0)

	const workers = 500
	start := make(chan struct{})
	ids := make(chan string, workers)

	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			<-start
			ids <- generateChangeID()
		}()
	}

	close(start)
	wg.Wait()
	close(ids)

	seen := make(map[string]struct{}, workers)
	for id := range ids {
		if _, exists := seen[id]; exists {
			t.Fatalf("duplicate change ID generated under concurrency: %s", id)
		}
		seen[id] = struct{}{}
	}

	if len(seen) != workers {
		t.Fatalf("expected %d IDs, got %d", workers, len(seen))
	}
}
