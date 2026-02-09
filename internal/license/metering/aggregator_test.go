package metering

import (
	"errors"
	"fmt"
	"sync"
	"testing"
)

func TestAggregatorRecord(t *testing.T) {
	agg := NewWindowedAggregator()

	for i := 0; i < 3; i++ {
		err := agg.Record(Event{
			Type:     EventAgentSeen,
			TenantID: "tenant-a",
			Key:      "agent-1",
			Value:    1,
		})
		if err != nil {
			t.Fatalf("record failed: %v", err)
		}
	}

	if got := agg.BucketCount(); got != 1 {
		t.Fatalf("BucketCount() = %d, want 1", got)
	}

	buckets := agg.Flush()
	if len(buckets) != 1 {
		t.Fatalf("len(flush) = %d, want 1", len(buckets))
	}

	if buckets[0].Count != 3 {
		t.Fatalf("bucket count = %d, want 3", buckets[0].Count)
	}
}

func TestAggregatorRecordDifferentKeys(t *testing.T) {
	agg := NewWindowedAggregator()

	keys := []string{"agent-1", "agent-2", "agent-3"}
	for _, key := range keys {
		err := agg.Record(Event{
			Type:     EventAgentSeen,
			TenantID: "tenant-a",
			Key:      key,
			Value:    1,
		})
		if err != nil {
			t.Fatalf("record failed for key %q: %v", key, err)
		}
	}

	if got := agg.BucketCount(); got != len(keys) {
		t.Fatalf("BucketCount() = %d, want %d", got, len(keys))
	}
}

func TestAggregatorIdempotency(t *testing.T) {
	agg := NewWindowedAggregator()

	once := Event{
		Type:           EventAgentSeen,
		TenantID:       "tenant-a",
		Key:            "agent-1",
		Value:          1,
		IdempotencyKey: "abc",
	}

	if err := agg.Record(once); err != nil {
		t.Fatalf("first record failed: %v", err)
	}

	if err := agg.Record(once); !errors.Is(err, ErrDuplicateEvent) {
		t.Fatalf("second record error = %v, want %v", err, ErrDuplicateEvent)
	}

	firstFlush := agg.Flush()
	if len(firstFlush) != 1 {
		t.Fatalf("len(firstFlush) = %d, want 1", len(firstFlush))
	}
	if firstFlush[0].Count != 1 {
		t.Fatalf("firstFlush count = %d, want 1", firstFlush[0].Count)
	}

	withoutKey := Event{
		Type:     EventAgentSeen,
		TenantID: "tenant-a",
		Key:      "agent-1",
		Value:    1,
	}
	if err := agg.Record(withoutKey); err != nil {
		t.Fatalf("record without idempotency key failed: %v", err)
	}
	if err := agg.Record(withoutKey); err != nil {
		t.Fatalf("second record without idempotency key failed: %v", err)
	}

	secondFlush := agg.Flush()
	if len(secondFlush) != 1 {
		t.Fatalf("len(secondFlush) = %d, want 1", len(secondFlush))
	}
	if secondFlush[0].Count != 2 {
		t.Fatalf("secondFlush count = %d, want 2", secondFlush[0].Count)
	}
}

func TestAggregatorCardinalityLimit(t *testing.T) {
	agg := NewWindowedAggregator()

	for i := 0; i < MaxCardinalityPerTenant; i++ {
		err := agg.Record(Event{
			Type:     EventAgentSeen,
			TenantID: "tenant-a",
			Key:      fmt.Sprintf("agent-%d", i),
			Value:    1,
		})
		if err != nil {
			t.Fatalf("record at i=%d failed: %v", i, err)
		}
	}

	if err := agg.Record(Event{
		Type:     EventAgentSeen,
		TenantID: "tenant-a",
		Key:      "agent-0",
		Value:    1,
	}); err != nil {
		t.Fatalf("record for existing key failed: %v", err)
	}

	if err := agg.Record(Event{
		Type:     EventAgentSeen,
		TenantID: "tenant-a",
		Key:      "agent-overflow",
		Value:    1,
	}); !errors.Is(err, ErrCardinalityExceeded) {
		t.Fatalf("overflow record error = %v, want %v", err, ErrCardinalityExceeded)
	}

	if err := agg.Record(Event{
		Type:     EventAgentSeen,
		TenantID: "tenant-b",
		Key:      "agent-overflow",
		Value:    1,
	}); err != nil {
		t.Fatalf("different tenant record failed: %v", err)
	}

	if err := agg.Record(Event{
		Type:     EventRelayBytes,
		TenantID: "tenant-a",
		Key:      "stream-1",
		Value:    42,
	}); err != nil {
		t.Fatalf("different event type record failed: %v", err)
	}
}

func TestAggregatorFlush(t *testing.T) {
	agg := NewWindowedAggregator()

	initialWindowStart := agg.windowStart
	if initialWindowStart.IsZero() {
		t.Fatal("initial window start is zero")
	}

	events := []Event{
		{Type: EventAgentSeen, TenantID: "tenant-a", Key: "agent-1", Value: 1},
		{Type: EventAgentSeen, TenantID: "tenant-a", Key: "agent-1", Value: 1},
		{Type: EventRelayBytes, TenantID: "tenant-a", Key: "relay-1", Value: 100},
		{Type: EventRelayBytes, TenantID: "tenant-a", Key: "relay-1", Value: 50},
	}
	for _, event := range events {
		if err := agg.Record(event); err != nil {
			t.Fatalf("record failed: %v", err)
		}
	}

	buckets := agg.Flush()
	if len(buckets) != 2 {
		t.Fatalf("len(flush) = %d, want 2", len(buckets))
	}

	byBucket := make(map[string]AggregatedBucket, len(buckets))
	for _, b := range buckets {
		byBucket[bucketID(b)] = b
		if !b.WindowStart.Equal(initialWindowStart) {
			t.Fatalf("WindowStart = %v, want %v", b.WindowStart, initialWindowStart)
		}
		if b.WindowEnd.Before(b.WindowStart) {
			t.Fatalf("WindowEnd (%v) is before WindowStart (%v)", b.WindowEnd, b.WindowStart)
		}
	}

	agentBucket, ok := byBucket["tenant-a|agent_seen|agent-1"]
	if !ok {
		t.Fatal("agent bucket not found")
	}
	if agentBucket.Count != 2 {
		t.Fatalf("agent bucket count = %d, want 2", agentBucket.Count)
	}
	if agentBucket.TotalValue != 2 {
		t.Fatalf("agent bucket total = %d, want 2", agentBucket.TotalValue)
	}

	relayBucket, ok := byBucket["tenant-a|relay_bytes|relay-1"]
	if !ok {
		t.Fatal("relay bucket not found")
	}
	if relayBucket.Count != 2 {
		t.Fatalf("relay bucket count = %d, want 2", relayBucket.Count)
	}
	if relayBucket.TotalValue != 150 {
		t.Fatalf("relay bucket total = %d, want 150", relayBucket.TotalValue)
	}

	if got := agg.BucketCount(); got != 0 {
		t.Fatalf("BucketCount() after flush = %d, want 0", got)
	}

	if err := agg.Record(Event{
		Type:     EventAgentSeen,
		TenantID: "tenant-a",
		Key:      "agent-1",
		Value:    1,
	}); err != nil {
		t.Fatalf("record after flush failed: %v", err)
	}

	afterFlush := agg.Flush()
	if len(afterFlush) != 1 {
		t.Fatalf("len(afterFlush) = %d, want 1", len(afterFlush))
	}
	if !afterFlush[0].WindowStart.Equal(agentBucket.WindowEnd) {
		t.Fatalf("new window start = %v, want %v", afterFlush[0].WindowStart, agentBucket.WindowEnd)
	}
}

func TestAggregatorFlushEmpty(t *testing.T) {
	agg := NewWindowedAggregator()

	buckets := agg.Flush()
	if buckets == nil {
		t.Fatal("Flush() returned nil, want empty slice")
	}
	if len(buckets) != 0 {
		t.Fatalf("len(flush) = %d, want 0", len(buckets))
	}
}

func TestAggregatorSnapshotEmpty(t *testing.T) {
	agg := NewWindowedAggregator()

	buckets := agg.Snapshot()
	if buckets == nil {
		t.Fatal("Snapshot() returned nil, want empty slice")
	}
	if len(buckets) != 0 {
		t.Fatalf("len(snapshot) = %d, want 0", len(buckets))
	}
	if got := agg.BucketCount(); got != 0 {
		t.Fatalf("BucketCount() = %d, want 0", got)
	}
}

func TestAggregatorSnapshotNonDestructive(t *testing.T) {
	agg := NewWindowedAggregator()

	events := []Event{
		{Type: EventAgentSeen, TenantID: "tenant-a", Key: "agent-1", Value: 1},
		{Type: EventAgentSeen, TenantID: "tenant-a", Key: "agent-1", Value: 1},
		{Type: EventRelayBytes, TenantID: "tenant-a", Key: "relay-1", Value: 100},
	}
	for _, event := range events {
		if err := agg.Record(event); err != nil {
			t.Fatalf("record failed: %v", err)
		}
	}

	snapshot := agg.Snapshot()
	if len(snapshot) != 2 {
		t.Fatalf("len(snapshot) = %d, want 2", len(snapshot))
	}

	if got := agg.BucketCount(); got != 2 {
		t.Fatalf("BucketCount() after snapshot = %d, want 2", got)
	}

	flushed := agg.Flush()
	if len(flushed) != 2 {
		t.Fatalf("len(flush) = %d, want 2", len(flushed))
	}

	snapshotBuckets := make(map[string]AggregatedBucket, len(snapshot))
	for _, bucket := range snapshot {
		snapshotBuckets[bucketID(bucket)] = bucket
	}
	for _, bucket := range flushed {
		snap, ok := snapshotBuckets[bucketID(bucket)]
		if !ok {
			t.Fatalf("snapshot missing bucket %q", bucketID(bucket))
		}
		if snap.Count != bucket.Count {
			t.Fatalf("bucket %q count = %d in snapshot, want %d", bucketID(bucket), snap.Count, bucket.Count)
		}
		if snap.TotalValue != bucket.TotalValue {
			t.Fatalf("bucket %q total = %d in snapshot, want %d", bucketID(bucket), snap.TotalValue, bucket.TotalValue)
		}
	}
}

func TestAggregatorSnapshotReturnsCopy(t *testing.T) {
	agg := NewWindowedAggregator()

	if err := agg.Record(Event{
		Type:     EventAgentSeen,
		TenantID: "tenant-a",
		Key:      "agent-1",
		Value:    1,
	}); err != nil {
		t.Fatalf("record failed: %v", err)
	}

	first := agg.Snapshot()
	if len(first) != 1 {
		t.Fatalf("len(first snapshot) = %d, want 1", len(first))
	}
	first[0].Key = "mutated"
	first[0].Count = 999
	first[0].TotalValue = 999

	second := agg.Snapshot()
	if len(second) != 1 {
		t.Fatalf("len(second snapshot) = %d, want 1", len(second))
	}
	if second[0].Key != "agent-1" {
		t.Fatalf("second snapshot key = %q, want agent-1", second[0].Key)
	}
	if second[0].Count != 1 {
		t.Fatalf("second snapshot count = %d, want 1", second[0].Count)
	}
	if second[0].TotalValue != 1 {
		t.Fatalf("second snapshot total = %d, want 1", second[0].TotalValue)
	}
}

func TestAggregatorSnapshotPreservesIdempotencyState(t *testing.T) {
	agg := NewWindowedAggregator()

	event := Event{
		Type:           EventAgentSeen,
		TenantID:       "tenant-a",
		Key:            "agent-1",
		Value:          1,
		IdempotencyKey: "idem-key",
	}
	if err := agg.Record(event); err != nil {
		t.Fatalf("first record failed: %v", err)
	}

	_ = agg.Snapshot()

	if err := agg.Record(event); !errors.Is(err, ErrDuplicateEvent) {
		t.Fatalf("second record error = %v, want %v", err, ErrDuplicateEvent)
	}
}

func TestAggregatorConcurrency(t *testing.T) {
	agg := NewWindowedAggregator()

	const (
		goroutines   = 10
		eventsPerGor = 100
	)

	var wg sync.WaitGroup
	errCh := make(chan error, goroutines)

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < eventsPerGor; i++ {
				err := agg.Record(Event{
					Type:     EventAgentSeen,
					TenantID: "tenant-a",
					Key:      "agent-1",
					Value:    1,
				})
				if err != nil {
					errCh <- fmt.Errorf("worker=%d i=%d: %w", worker, i, err)
					return
				}
			}
		}(g)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Fatalf("concurrent record error: %v", err)
	}

	buckets := agg.Flush()
	if len(buckets) != 1 {
		t.Fatalf("len(flush) = %d, want 1", len(buckets))
	}

	wantTotal := int64(goroutines * eventsPerGor)
	if buckets[0].Count != wantTotal {
		t.Fatalf("count = %d, want %d", buckets[0].Count, wantTotal)
	}
	if buckets[0].TotalValue != wantTotal {
		t.Fatalf("total value = %d, want %d", buckets[0].TotalValue, wantTotal)
	}
}

func bucketID(b AggregatedBucket) string {
	return fmt.Sprintf("%s|%s|%s", b.TenantID, b.Type, b.Key)
}
