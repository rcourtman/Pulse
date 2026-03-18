package metering

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestNewWindowedAggregator(t *testing.T) {
	agg := NewWindowedAggregator()
	if agg == nil {
		t.Fatal("expected non-nil aggregator")
	}
	if agg.BucketCount() != 0 {
		t.Errorf("new aggregator should have 0 buckets, got %d", agg.BucketCount())
	}
}

func TestRecord_BasicAggregation(t *testing.T) {
	agg := NewWindowedAggregator()

	// Record two events for the same tenant/type/key — they should aggregate.
	for i := 0; i < 5; i++ {
		err := agg.Record(Event{
			Type:     EventAgentSeen,
			TenantID: "tenant-1",
			Key:      "agent-abc",
			Value:    1,
		})
		if err != nil {
			t.Fatalf("unexpected error on record %d: %v", i, err)
		}
	}

	if agg.BucketCount() != 1 {
		t.Errorf("expected 1 bucket, got %d", agg.BucketCount())
	}

	buckets := agg.Flush()
	if len(buckets) != 1 {
		t.Fatalf("expected 1 bucket after flush, got %d", len(buckets))
	}
	b := buckets[0]
	if b.TenantID != "tenant-1" {
		t.Errorf("expected tenant-1, got %s", b.TenantID)
	}
	if b.Type != EventAgentSeen {
		t.Errorf("expected event type %s, got %s", EventAgentSeen, b.Type)
	}
	if b.Key != "agent-abc" {
		t.Errorf("expected key agent-abc, got %s", b.Key)
	}
	if b.Count != 5 {
		t.Errorf("expected count 5, got %d", b.Count)
	}
	if b.TotalValue != 5 {
		t.Errorf("expected total value 5, got %d", b.TotalValue)
	}
}

func TestRecord_MultipleBuckets(t *testing.T) {
	agg := NewWindowedAggregator()

	// Different tenants, types, and keys should create distinct buckets.
	events := []Event{
		{Type: EventAgentSeen, TenantID: "t1", Key: "a1", Value: 1},
		{Type: EventAgentSeen, TenantID: "t1", Key: "a2", Value: 1},
		{Type: EventRelayBytes, TenantID: "t1", Key: "a1", Value: 1024},
		{Type: EventAgentSeen, TenantID: "t2", Key: "a1", Value: 1},
	}
	for _, e := range events {
		if err := agg.Record(e); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	if agg.BucketCount() != 4 {
		t.Errorf("expected 4 buckets, got %d", agg.BucketCount())
	}
}

func TestRecord_IdempotencyDedup(t *testing.T) {
	agg := NewWindowedAggregator()

	first := Event{
		Type:           EventAgentSeen,
		TenantID:       "t1",
		Key:            "a1",
		Value:          1,
		IdempotencyKey: "idem-1",
	}
	if err := agg.Record(first); err != nil {
		t.Fatalf("first record failed: %v", err)
	}

	// Duplicate idempotency key should be rejected.
	err := agg.Record(first)
	if !errors.Is(err, ErrDuplicateEvent) {
		t.Errorf("expected ErrDuplicateEvent, got %v", err)
	}

	// Different idempotency key should succeed.
	second := first
	second.IdempotencyKey = "idem-2"
	if err := agg.Record(second); err != nil {
		t.Fatalf("second record failed: %v", err)
	}

	buckets := agg.Flush()
	if len(buckets) != 1 {
		t.Fatalf("expected 1 bucket, got %d", len(buckets))
	}
	if buckets[0].Count != 2 {
		t.Errorf("expected count 2 (deduped), got %d", buckets[0].Count)
	}
}

func TestRecord_EmptyIdempotencyKey_NeverDeduped(t *testing.T) {
	agg := NewWindowedAggregator()

	// Events without idempotency keys should never be deduped.
	for i := 0; i < 3; i++ {
		if err := agg.Record(Event{
			Type:     EventAgentSeen,
			TenantID: "t1",
			Key:      "a1",
			Value:    1,
		}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	buckets := agg.Flush()
	if len(buckets) != 1 {
		t.Fatalf("expected 1 bucket, got %d", len(buckets))
	}
	if buckets[0].Count != 3 {
		t.Errorf("expected count 3, got %d", buckets[0].Count)
	}
}

func TestRecord_CardinalityLimit(t *testing.T) {
	agg := NewWindowedAggregator()

	// Fill up to the cardinality limit for one tenant/event-type.
	for i := 0; i < MaxCardinalityPerTenant; i++ {
		err := agg.Record(Event{
			Type:     EventAgentSeen,
			TenantID: "t1",
			Key:      fmt.Sprintf("key-%d", i),
			Value:    1,
		})
		if err != nil {
			t.Fatalf("unexpected error at key %d: %v", i, err)
		}
	}

	// Next unique key should be rejected.
	err := agg.Record(Event{
		Type:     EventAgentSeen,
		TenantID: "t1",
		Key:      "one-too-many",
		Value:    1,
	})
	if !errors.Is(err, ErrCardinalityExceeded) {
		t.Errorf("expected ErrCardinalityExceeded, got %v", err)
	}

	// Existing key should still work (not a new bucket).
	err = agg.Record(Event{
		Type:     EventAgentSeen,
		TenantID: "t1",
		Key:      "key-0",
		Value:    1,
	})
	if err != nil {
		t.Errorf("existing key should succeed, got %v", err)
	}

	// Different tenant should not be affected.
	err = agg.Record(Event{
		Type:     EventAgentSeen,
		TenantID: "t2",
		Key:      "new-key",
		Value:    1,
	})
	if err != nil {
		t.Errorf("different tenant should succeed, got %v", err)
	}

	// Different event type for same tenant should not be affected.
	err = agg.Record(Event{
		Type:     EventRelayBytes,
		TenantID: "t1",
		Key:      "relay-key",
		Value:    1024,
	})
	if err != nil {
		t.Errorf("different event type should succeed, got %v", err)
	}
}

func TestRecord_IdempotencyKeyLimit(t *testing.T) {
	agg := NewWindowedAggregator()

	// Fill up idempotency key space. Spread across tenants to avoid hitting
	// MaxCardinalityPerTenant (1000) before MaxIdempotencyKeysPerWindow (10000).
	for i := 0; i < MaxIdempotencyKeysPerWindow; i++ {
		tenantID := fmt.Sprintf("t-%d", i/MaxCardinalityPerTenant)
		err := agg.Record(Event{
			Type:           EventAgentSeen,
			TenantID:       tenantID,
			Key:            fmt.Sprintf("key-%d", i%MaxCardinalityPerTenant),
			Value:          1,
			IdempotencyKey: fmt.Sprintf("idem-%d", i),
		})
		if err != nil {
			t.Fatalf("unexpected error at key %d (tenant %s): %v", i, tenantID, err)
		}
	}

	// Next event with an idempotency key should be rejected.
	err := agg.Record(Event{
		Type:           EventAgentSeen,
		TenantID:       "t1",
		Key:            "overflow",
		Value:          1,
		IdempotencyKey: "idem-overflow",
	})
	if !errors.Is(err, ErrIdempotencyKeyLimitExceeded) {
		t.Errorf("expected ErrIdempotencyKeyLimitExceeded, got %v", err)
	}

	// Events without idempotency keys should still work.
	err = agg.Record(Event{
		Type:     EventAgentSeen,
		TenantID: "t1",
		Key:      "no-idem",
		Value:    1,
	})
	if err != nil {
		t.Errorf("event without idempotency key should succeed, got %v", err)
	}
}

func TestFlush_ResetsState(t *testing.T) {
	agg := NewWindowedAggregator()

	if err := agg.Record(Event{
		Type:           EventAgentSeen,
		TenantID:       "t1",
		Key:            "a1",
		Value:          1,
		IdempotencyKey: "idem-1",
	}); err != nil {
		t.Fatal(err)
	}

	buckets := agg.Flush()
	if len(buckets) != 1 {
		t.Fatalf("expected 1 bucket, got %d", len(buckets))
	}

	// After flush:
	// - counters should be empty
	if agg.BucketCount() != 0 {
		t.Errorf("expected 0 buckets after flush, got %d", agg.BucketCount())
	}
	// - second flush should return empty
	buckets2 := agg.Flush()
	if len(buckets2) != 0 {
		t.Errorf("expected 0 buckets on second flush, got %d", len(buckets2))
	}

	// - idempotency keys should be reset (same key should work again)
	err := agg.Record(Event{
		Type:           EventAgentSeen,
		TenantID:       "t1",
		Key:            "a1",
		Value:          1,
		IdempotencyKey: "idem-1",
	})
	if err != nil {
		t.Errorf("idempotency key should be valid after flush, got %v", err)
	}
}

func TestFlush_WindowTimes(t *testing.T) {
	before := time.Now()
	agg := NewWindowedAggregator()

	if err := agg.Record(Event{Type: EventAgentSeen, TenantID: "t1", Key: "a1", Value: 1}); err != nil {
		t.Fatal(err)
	}

	buckets := agg.Flush()
	after := time.Now()

	if len(buckets) != 1 {
		t.Fatalf("expected 1 bucket, got %d", len(buckets))
	}

	b := buckets[0]
	if b.WindowStart.Before(before) {
		t.Errorf("window start %v should not be before test start %v", b.WindowStart, before)
	}
	if b.WindowEnd.After(after) {
		t.Errorf("window end %v should not be after test end %v", b.WindowEnd, after)
	}
	if b.WindowEnd.Before(b.WindowStart) {
		t.Errorf("window end %v should not be before window start %v", b.WindowEnd, b.WindowStart)
	}
}

func TestSnapshot_DoesNotResetState(t *testing.T) {
	agg := NewWindowedAggregator()

	if err := agg.Record(Event{Type: EventAgentSeen, TenantID: "t1", Key: "a1", Value: 1}); err != nil {
		t.Fatal(err)
	}
	if err := agg.Record(Event{Type: EventAgentSeen, TenantID: "t1", Key: "a1", Value: 1}); err != nil {
		t.Fatal(err)
	}

	snap1 := agg.Snapshot()
	if len(snap1) != 1 {
		t.Fatalf("expected 1 bucket in snapshot, got %d", len(snap1))
	}
	if snap1[0].Count != 2 {
		t.Errorf("expected count 2, got %d", snap1[0].Count)
	}

	// Snapshot should not reset; second snapshot should return same data.
	snap2 := agg.Snapshot()
	if len(snap2) != 1 {
		t.Fatalf("expected 1 bucket in second snapshot, got %d", len(snap2))
	}
	if snap2[0].Count != 2 {
		t.Errorf("expected count 2 in second snapshot, got %d", snap2[0].Count)
	}

	// BucketCount should still be 1.
	if agg.BucketCount() != 1 {
		t.Errorf("expected bucket count 1 after snapshot, got %d", agg.BucketCount())
	}
}

func TestSnapshot_NilAggregator(t *testing.T) {
	var agg *WindowedAggregator
	snap := agg.Snapshot()
	if snap == nil {
		t.Error("nil aggregator snapshot should return empty slice, not nil")
	}
	if len(snap) != 0 {
		t.Errorf("nil aggregator snapshot should be empty, got %d", len(snap))
	}
}

func TestRecord_ValueAggregation(t *testing.T) {
	agg := NewWindowedAggregator()

	values := []int64{100, 200, 300, 400}
	for _, v := range values {
		if err := agg.Record(Event{
			Type:     EventRelayBytes,
			TenantID: "t1",
			Key:      "org-1",
			Value:    v,
		}); err != nil {
			t.Fatal(err)
		}
	}

	buckets := agg.Flush()
	if len(buckets) != 1 {
		t.Fatalf("expected 1 bucket, got %d", len(buckets))
	}
	if buckets[0].TotalValue != 1000 {
		t.Errorf("expected total value 1000, got %d", buckets[0].TotalValue)
	}
	if buckets[0].Count != 4 {
		t.Errorf("expected count 4, got %d", buckets[0].Count)
	}
}

func TestConcurrentRecord(t *testing.T) {
	agg := NewWindowedAggregator()
	const goroutines = 10
	const eventsPerGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(gID int) {
			defer wg.Done()
			for i := 0; i < eventsPerGoroutine; i++ {
				_ = agg.Record(Event{
					Type:     EventAgentSeen,
					TenantID: fmt.Sprintf("t-%d", gID),
					Key:      fmt.Sprintf("key-%d", i%10), // 10 unique keys per goroutine
					Value:    1,
				})
			}
		}(g)
	}
	wg.Wait()

	buckets := agg.Flush()
	totalCount := int64(0)
	for _, b := range buckets {
		totalCount += b.Count
	}
	if totalCount != goroutines*eventsPerGoroutine {
		t.Errorf("expected total count %d, got %d", goroutines*eventsPerGoroutine, totalCount)
	}
}

func TestConcurrentRecordAndSnapshot(t *testing.T) {
	agg := NewWindowedAggregator()
	const goroutines = 5
	const iterations = 50

	var wg sync.WaitGroup
	wg.Add(goroutines * 2) // half writers, half readers

	for g := 0; g < goroutines; g++ {
		go func(gID int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				_ = agg.Record(Event{
					Type:     EventAgentSeen,
					TenantID: fmt.Sprintf("t-%d", gID),
					Key:      "k",
					Value:    1,
				})
			}
		}(g)
	}

	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				_ = agg.Snapshot()
			}
		}()
	}

	wg.Wait()

	// No panics = success. Verify we can still flush.
	buckets := agg.Flush()
	if len(buckets) == 0 {
		t.Error("expected at least some buckets after concurrent operations")
	}
}
