package auth

import (
	"sync"
	"testing"
	"time"
)

func TestStoreConcurrentCloseAndOperations(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	rec := &TokenRecord{
		Email:     "alice@example.com",
		TenantID:  "tenant-1",
		ExpiresAt: time.Now().UTC().Add(10 * time.Minute),
	}

	done := make(chan struct{})
	var workers sync.WaitGroup
	startWorker := func(fn func(i int), n int) {
		for i := 0; i < n; i++ {
			workers.Add(1)
			go func(i int) {
				defer workers.Done()
				for {
					select {
					case <-done:
						return
					default:
						fn(i)
					}
				}
			}(i)
		}
	}

	startWorker(func(i int) {
		_ = store.Put(signHMAC([]byte("put-key"), string(rune('a'+i%26))), rec)
	}, 4)
	startWorker(func(i int) {
		_, _ = store.Consume(signHMAC([]byte("consume-key"), string(rune('a'+i%26))), time.Now().UTC())
	}, 4)
	startWorker(func(_ int) {
		_ = store.DeleteExpired(time.Now().UTC())
	}, 2)

	time.Sleep(50 * time.Millisecond)

	var closers sync.WaitGroup
	for i := 0; i < 8; i++ {
		closers.Add(1)
		go func() {
			defer closers.Done()
			store.Close()
		}()
	}
	closers.Wait()
	close(done)

	waitCh := make(chan struct{})
	go func() {
		workers.Wait()
		close(waitCh)
	}()

	select {
	case <-waitCh:
	case <-time.After(2 * time.Second):
		t.Fatal("workers did not stop after close")
	}
}
