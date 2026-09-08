package resourceapi

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// These tests exercise API coordination only; no durable store is opened.
type coordinationStore struct {
	unified.ResourceStore
	closed atomic.Int32
}

func (s *coordinationStore) Close() error { s.closed.Add(1); return nil }

func TestStoreConstructionDoesNotBlockOtherTenants(t *testing.T) {
	h := NewQueryService(nil)
	warm := &coordinationStore{}
	h.stores[cacheKey("warm")] = warm
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	h.openStore = func(_, key string) (unified.ResourceStore, error) {
		if key == "cold" {
			close(entered)
			<-release
		}
		return &coordinationStore{}, nil
	}
	coldDone := make(chan struct{})
	go func() { defer close(coldDone); _, _ = h.getStore("cold") }()
	<-entered
	for _, key := range []string{"warm", "independent"} {
		done := make(chan error, 1)
		go func(key string) { _, err := h.getStore(key); done <- err }(key)
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(time.Second):
			unblock()
			<-coldDone
			t.Fatalf("tenant %s waited for unrelated cold construction", key)
		}
	}
	unblock()
	<-coldDone
	if err := h.CloseStores(); err != nil {
		t.Fatal(err)
	}
}

func TestStoreConstructionDeduplicatesAndCoordinatesClose(t *testing.T) {
	for _, all := range []bool{false, true} {
		name := "tenant"
		if all {
			name = "all"
		}
		t.Run(name, func(t *testing.T) {
			h := NewQueryService(nil)
			entered, release := make(chan struct{}), make(chan struct{})
			var calls atomic.Int32
			store := &coordinationStore{}
			h.openStore = func(_, _ string) (unified.ResourceStore, error) {
				if calls.Add(1) == 1 {
					close(entered)
				}
				<-release
				return store, nil
			}
			var wg sync.WaitGroup
			for i := 0; i < 12; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					_, err := h.getStore("same")
					if err != nil {
						t.Error(err)
					}
				}()
			}
			<-entered
			close(release)
			wg.Wait()
			if calls.Load() != 1 {
				t.Fatalf("constructors=%d", calls.Load())
			}
			// A second blocked construction must finish before either eviction method returns.
			entered, release = make(chan struct{}), make(chan struct{})
			pendingStore := &coordinationStore{}
			h.openStore = func(_, _ string) (unified.ResourceStore, error) {
				close(entered)
				<-release
				return pendingStore, nil
			}
			done := make(chan struct{})
			go func() { defer close(done); _, _ = h.getStore("pending") }()
			<-entered
			closed := make(chan error, 1)
			go func() {
				if all {
					closed <- h.CloseStores()
				} else {
					closed <- h.CloseTenantStore("pending")
				}
			}()
			select {
			case err := <-closed:
				close(release)
				<-done
				t.Fatalf("close returned before pending constructor: %v", err)
			case <-time.After(50 * time.Millisecond):
			}
			close(release)
			<-done
			if err := <-closed; err != nil {
				t.Fatal(err)
			}
			if _, ok := h.stores[cacheKey("pending")]; ok {
				t.Fatal("pending store survived eviction")
			}
			if pendingStore.closed.Load() != 1 {
				t.Fatalf("pending store closed %d times", pendingStore.closed.Load())
			}
			if err := h.CloseStores(); err != nil {
				t.Fatal(err)
			}
			if store.closed.Load() != 1 {
				t.Fatalf("original store closed %d times", store.closed.Load())
			}
		})
	}
}

func TestStoreConstructionFailureCanRetry(t *testing.T) {
	h := NewQueryService(nil)
	failure := errors.New("synthetic constructor failure")
	calls := 0
	h.openStore = func(_, _ string) (unified.ResourceStore, error) {
		calls++
		if calls == 1 {
			return nil, failure
		}
		return &coordinationStore{}, nil
	}
	if _, err := h.getStore("retry"); !errors.Is(err, failure) {
		t.Fatalf("got %v", err)
	}
	if _, err := h.getStore("retry"); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("calls=%d", calls)
	}
	if err := h.CloseStores(); err != nil {
		t.Fatal(err)
	}
}
