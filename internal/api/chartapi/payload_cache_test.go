package chartapi

import (
	"bytes"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestBoundedChartPayloadCacheLimitsCardinalityAndBytes(t *testing.T) {
	now := time.Date(2026, time.August, 30, 10, 0, 0, 0, time.UTC)
	cache := newBoundedChartPayloadCache(2, 10)

	cache.put("first", bytes.Repeat([]byte{'a'}, 6), now.Add(time.Minute), now)
	cache.put("second", bytes.Repeat([]byte{'b'}, 4), now.Add(time.Minute), now)
	cache.put("third", bytes.Repeat([]byte{'c'}, 6), now.Add(time.Minute), now)

	if _, ok := cache.get("first", now); ok {
		t.Fatal("oldest entry remained after the shared byte budget was exhausted")
	}
	if _, ok := cache.get("second", now); !ok {
		t.Fatal("newer entry was unexpectedly evicted")
	}
	if _, ok := cache.get("third", now); !ok {
		t.Fatal("new entry was not retained")
	}
	if got := len(cache.entries); got != 2 {
		t.Fatalf("retained entries = %d, want 2", got)
	}
	if cache.bytes > cache.maxBytes {
		t.Fatalf("retained bytes = %d, exceeds budget %d", cache.bytes, cache.maxBytes)
	}
}

func TestBoundedChartPayloadCachePurgesExpiredVariantsOnInsert(t *testing.T) {
	now := time.Date(2026, time.August, 30, 10, 0, 0, 0, time.UTC)
	cache := newBoundedChartPayloadCache(4, 32)

	cache.put("expired-a", []byte("aaaa"), now.Add(time.Second), now)
	cache.put("expired-b", []byte("bbbb"), now.Add(time.Second), now)
	later := now.Add(2 * time.Second)
	cache.put("live", []byte("cccc"), later.Add(time.Second), later)

	if got := len(cache.entries); got != 1 {
		t.Fatalf("retained entries after expiry sweep = %d, want 1", got)
	}
	if cache.bytes != len("cccc") {
		t.Fatalf("retained bytes after expiry sweep = %d, want %d", cache.bytes, len("cccc"))
	}
}

func TestBoundedChartPayloadCacheDoesNotRetainOversizedResponse(t *testing.T) {
	now := time.Date(2026, time.August, 30, 10, 0, 0, 0, time.UTC)
	cache := newBoundedChartPayloadCache(4, 8)
	cache.put("oversized", bytes.Repeat([]byte{'x'}, 9), now.Add(time.Minute), now)

	if _, ok := cache.get("oversized", now); ok {
		t.Fatal("response larger than the full cache budget was retained")
	}
	if len(cache.entries) != 0 || cache.bytes != 0 {
		t.Fatalf("oversized response changed cache state: entries=%d bytes=%d", len(cache.entries), cache.bytes)
	}
}

func TestChartRoutesShareOnePayloadRetentionBudget(t *testing.T) {
	now := time.Date(2026, time.August, 30, 10, 0, 0, 0, time.UTC)
	service := NewService(nil)
	service.chartPayloads = newBoundedChartPayloadCache(2, 10)

	service.cacheInfrastructureChartsPayload("infra", []byte("aaaa"), now)
	service.cacheWorkloadsSummaryChartsPayload("summary", []byte("bbbb"), now)
	service.chartPayloads.put(
		workloadChartsCachePrefix+"workloads",
		[]byte("cccc"),
		now.Add(workloadChartsCacheTTL),
		now,
	)

	if got := len(service.chartPayloads.entries); got != 2 {
		t.Fatalf("retained entries across chart routes = %d, want shared cap 2", got)
	}
	if service.chartPayloads.bytes > service.chartPayloads.maxBytes {
		t.Fatalf("retained bytes across chart routes = %d, exceeds shared budget %d", service.chartPayloads.bytes, service.chartPayloads.maxBytes)
	}
	if _, ok := service.cachedInfrastructureChartsPayload("infra", now); ok {
		t.Fatal("oldest route entry was not evicted from the shared cache")
	}
}

func TestBoundedChartPayloadCacheStaysWithinBudgetUnderConcurrency(t *testing.T) {
	now := time.Date(2026, time.August, 30, 10, 0, 0, 0, time.UTC)
	cache := newBoundedChartPayloadCache(8, 4<<10)
	payload := bytes.Repeat([]byte{'x'}, 1024)

	var workers sync.WaitGroup
	for index := range 128 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			key := fmt.Sprintf("variant-%d", index)
			cache.put(key, payload, now.Add(time.Minute), now)
			_, _ = cache.get(key, now)
		}()
	}
	workers.Wait()

	if got := len(cache.entries); got > cache.maxEntries {
		t.Fatalf("retained entries after concurrent writes = %d, exceeds cap %d", got, cache.maxEntries)
	}
	if cache.bytes > cache.maxBytes {
		t.Fatalf("retained bytes after concurrent writes = %d, exceeds budget %d", cache.bytes, cache.maxBytes)
	}
}
