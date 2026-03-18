package metrics

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func TestStoreWriteBatchSync(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(DefaultConfig(dir))
	if err != nil {
		t.Fatalf("NewStore returned error: %v", err)
	}
	defer store.Close()

	ts := time.Unix(1000, 0)
	metrics := []WriteMetric{
		{ResourceType: "vm", ResourceID: "v1", MetricType: "cpu", Value: 10.0, Timestamp: ts, Tier: TierRaw},
		{ResourceType: "vm", ResourceID: "v1", MetricType: "mem", Value: 50.0, Timestamp: ts, Tier: TierRaw},
	}

	store.WriteBatchSync(metrics)

	// Verify data was written
	points, err := store.Query("vm", "v1", "cpu", ts.Add(-time.Second), ts.Add(time.Second), 0)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(points) != 1 || points[0].Value != 10.0 {
		t.Fatalf("expected 1 point with value 10.0, got %v", points)
	}
}

func TestStoreClear(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(DefaultConfig(dir))
	if err != nil {
		t.Fatalf("NewStore returned error: %v", err)
	}
	defer store.Close()

	// Write some data
	store.Write("vm", "v1", "cpu", 10.0, time.Now())
	store.Flush()

	// Write some data
	store.Write("vm", "v1", "cpu", 10.0, time.Now())
	store.Flush()

	// Wait for data to be written (Flush is async via channel)
	deadline := time.Now().Add(2 * time.Second)
	var stats Stats
	for time.Now().Before(deadline) {
		stats = store.GetStats()
		if stats.RawCount > 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if stats.RawCount == 0 {
		t.Fatal("expected data to be written before clearing")
	}

	// clear
	if err := store.Clear(); err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	// Verify empty
	stats = store.GetStats()
	if stats.RawCount != 0 {
		t.Fatalf("expected empty store, got %d raw records", stats.RawCount)
	}
}

func TestStoreRunRollupManually(t *testing.T) {
	// Tests the runRollup function wrapper which was showing 0% coverage
	dir := t.TempDir()
	cfg := DefaultConfig(dir)
	// Create separate DB for this test
	cfg.DBPath = filepath.Join(dir, "metrics-rollup-manual.db")

	store, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("NewStore returned error: %v", err)
	}
	defer store.Close()

	// Trigger manual rollup - should not panic
	store.runRollup()
}

func TestStoreGetMetaIntInvalid(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(DefaultConfig(dir))
	if err != nil {
		t.Fatalf("NewStore returned error: %v", err)
	}
	defer store.Close()

	// Insert invalid int value
	_, err = store.db.Exec("INSERT INTO metrics_meta (key, value) VALUES (?, ?)", "bad_key", "not_an_int")
	if err != nil {
		t.Fatalf("failed to insert invalid meta: %v", err)
	}

	val, ok := store.getMetaInt("bad_key")
	if ok {
		t.Fatalf("expected getMetaInt to fail for invalid int, got %d", val)
	}
}

func TestStoreFlushLockedLogsStructuredContextWhenWriteChannelFull(t *testing.T) {
	var buf bytes.Buffer
	origLogger := log.Logger
	log.Logger = zerolog.New(&buf).Level(zerolog.DebugLevel)
	t.Cleanup(func() {
		log.Logger = origLogger
	})

	store := &Store{
		config: StoreConfig{WriteBufferSize: 4},
		buffer: []bufferedMetric{
			{
				resourceType: "vm",
				resourceID:   "vm-101",
				metricType:   "cpu",
				value:        42,
				timestamp:    time.Unix(1000, 0),
				tier:         TierRaw,
			},
		},
		writeCh: make(chan []bufferedMetric, 1),
	}

	store.writeCh <- []bufferedMetric{{
		resourceType: "vm",
		resourceID:   "vm-102",
		metricType:   "memory",
		value:        12,
		timestamp:    time.Unix(1000, 0),
		tier:         TierRaw,
	}}

	store.flushLocked()

	if len(store.buffer) != 0 {
		t.Fatalf("expected flushLocked to clear in-memory buffer, got %d items", len(store.buffer))
	}

	logOutput := buf.String()
	required := []string{
		`"component":"metrics_store"`,
		`"action":"drop_write_batch"`,
		`"batch_size":1`,
		`"write_queue_depth":1`,
		`"write_queue_capacity":1`,
	}
	for _, token := range required {
		if !strings.Contains(logOutput, token) {
			t.Fatalf("expected log output to contain %s, got %s", token, logOutput)
		}
	}
}

func TestStoreMigratesLegacyHostResourceTypeToAgent(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultConfig(dir)

	store1, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("NewStore returned error: %v", err)
	}

	ts := time.Now().UTC().Truncate(time.Second)
	if _, err := store1.db.Exec(`
		INSERT INTO metrics (resource_type, resource_id, metric_type, value, timestamp, tier)
		VALUES (?, ?, ?, ?, ?, ?)
	`, "host", "host-1", "cpu", 77.0, ts.Unix(), string(TierRaw)); err != nil {
		t.Fatalf("failed to seed legacy host metric row: %v", err)
	}

	if err := store1.Close(); err != nil {
		t.Fatalf("failed to close first store: %v", err)
	}

	store2, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("failed to reopen metrics store: %v", err)
	}
	defer store2.Close()

	agentPoints, err := store2.Query("agent", "host-1", "cpu", ts.Add(-time.Second), ts.Add(time.Second), 0)
	if err != nil {
		t.Fatalf("query migrated agent metrics failed: %v", err)
	}
	if len(agentPoints) != 1 || agentPoints[0].Value != 77 {
		t.Fatalf("expected migrated agent metric value 77, got %+v", agentPoints)
	}

	legacyPoints, err := store2.Query("host", "host-1", "cpu", ts.Add(-time.Second), ts.Add(time.Second), 0)
	if err != nil {
		t.Fatalf("query legacy host metrics failed: %v", err)
	}
	if len(legacyPoints) != 0 {
		t.Fatalf("expected no legacy host metrics after migration, got %+v", legacyPoints)
	}
}

func TestStoreRejectsLegacyHostWrites(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(DefaultConfig(dir))
	if err != nil {
		t.Fatalf("NewStore returned error: %v", err)
	}
	defer store.Close()

	ts := time.Now().UTC().Truncate(time.Second)
	store.WriteBatchSync([]WriteMetric{
		{
			ResourceType: "host",
			ResourceID:   "host-1",
			MetricType:   "cpu",
			Value:        61,
			Timestamp:    ts,
			Tier:         TierRaw,
		},
	})

	agentPoints, err := store.Query("agent", "host-1", "cpu", ts.Add(-time.Second), ts.Add(time.Second), 0)
	if err != nil {
		t.Fatalf("query agent metrics failed: %v", err)
	}
	if len(agentPoints) != 0 {
		t.Fatalf("expected no agent rows from rejected legacy host write, got %+v", agentPoints)
	}

	legacyPoints, err := store.Query("host", "host-1", "cpu", ts.Add(-time.Second), ts.Add(time.Second), 0)
	if err != nil {
		t.Fatalf("query legacy host metrics failed: %v", err)
	}
	if len(legacyPoints) != 0 {
		t.Fatalf("expected no host rows for rejected writes, got %+v", legacyPoints)
	}
}

func TestStoreRejectsMalformedWrites(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(DefaultConfig(dir))
	if err != nil {
		t.Fatalf("NewStore returned error: %v", err)
	}
	defer store.Close()

	ts := time.Now().UTC().Truncate(time.Second)
	store.WriteWithTier("", "agent-1", "cpu", 12, ts, TierRaw)
	store.WriteWithTier("agent", "", "cpu", 12, ts, TierRaw)
	store.WriteWithTier("agent", "agent-1", "", 12, ts, TierRaw)
	store.WriteWithTier("agent", "agent-1", "cpu", 12, ts, Tier("bogus"))
	store.Flush()

	store.WriteBatchSync([]WriteMetric{
		{ResourceType: "", ResourceID: "agent-1", MetricType: "cpu", Value: 1, Timestamp: ts, Tier: TierRaw},
		{ResourceType: "agent", ResourceID: "", MetricType: "cpu", Value: 1, Timestamp: ts, Tier: TierRaw},
		{ResourceType: "agent", ResourceID: "agent-1", MetricType: "", Value: 1, Timestamp: ts, Tier: TierRaw},
		{ResourceType: "agent", ResourceID: "agent-1", MetricType: "cpu", Value: 1, Timestamp: ts, Tier: Tier("bogus")},
	})

	stats := store.GetStats()
	if stats.RawCount != 0 || stats.MinuteCount != 0 || stats.HourlyCount != 0 || stats.DailyCount != 0 {
		t.Fatalf("expected malformed writes to be dropped, got stats %+v", stats)
	}

	points, err := store.Query("agent", "agent-1", "cpu", ts.Add(-time.Second), ts.Add(time.Second), 0)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if len(points) != 0 {
		t.Fatalf("expected no stored points from malformed writes, got %+v", points)
	}
}

func TestStoreCanonicalizesResourceTypeCaseOnWriteAndQuery(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(DefaultConfig(dir))
	if err != nil {
		t.Fatalf("NewStore returned error: %v", err)
	}
	defer store.Close()

	ts := time.Now().UTC().Truncate(time.Second)
	store.WriteBatchSync([]WriteMetric{
		{
			ResourceType: " AGENT ",
			ResourceID:   "host-1",
			MetricType:   "cpu",
			Value:        61,
			Timestamp:    ts,
			Tier:         TierRaw,
		},
	})

	lowerPoints, err := store.Query("agent", "host-1", "cpu", ts.Add(-time.Second), ts.Add(time.Second), 0)
	if err != nil {
		t.Fatalf("lowercase query failed: %v", err)
	}
	if len(lowerPoints) != 1 || lowerPoints[0].Value != 61 {
		t.Fatalf("expected lowercase query to return canonicalized point, got %+v", lowerPoints)
	}

	upperPoints, err := store.Query("AGENT", "host-1", "cpu", ts.Add(-time.Second), ts.Add(time.Second), 0)
	if err != nil {
		t.Fatalf("uppercase query failed: %v", err)
	}
	if len(upperPoints) != 1 || upperPoints[0].Value != 61 {
		t.Fatalf("expected uppercase query to normalize to canonical resource type, got %+v", upperPoints)
	}
}

func TestStoreQueryCanonicalizesIdentifiers(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(DefaultConfig(dir))
	if err != nil {
		t.Fatalf("NewStore returned error: %v", err)
	}
	defer store.Close()
	ts := time.Unix(1710000000, 0)

	store.WriteBatchSync([]WriteMetric{{
		ResourceType: "agent",
		ResourceID:   " agent-1 ",
		MetricType:   " cpu ",
		Value:        42.0,
		Timestamp:    ts,
		Tier:         TierRaw,
	}})

	points, err := store.Query(" agent ", "agent-1", "cpu", ts.Add(-time.Second), ts.Add(time.Second), 0)
	if err != nil {
		t.Fatalf("query canonical identifiers failed: %v", err)
	}
	if len(points) != 1 {
		t.Fatalf("expected 1 point after identifier normalization, got %+v", points)
	}

	all, err := store.QueryAll(" agent ", " agent-1 ", ts.Add(-time.Second), ts.Add(time.Second), 0)
	if err != nil {
		t.Fatalf("query all canonical identifiers failed: %v", err)
	}
	if got := len(all["cpu"]); got != 1 {
		t.Fatalf("expected cpu metric after identifier normalization, got %+v", all)
	}
}

func TestStoreCanonicalizesMetricTypeCaseOnWriteAndQuery(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(DefaultConfig(dir))
	if err != nil {
		t.Fatalf("NewStore returned error: %v", err)
	}
	defer store.Close()

	ts := time.Unix(1710000200, 0)
	store.WriteBatchSync([]WriteMetric{{
		ResourceType: "agent",
		ResourceID:   "agent-1",
		MetricType:   " CPU ",
		Value:        27.0,
		Timestamp:    ts,
		Tier:         TierRaw,
	}})

	lowerPoints, err := store.Query("agent", "agent-1", "cpu", ts.Add(-time.Second), ts.Add(time.Second), 0)
	if err != nil {
		t.Fatalf("lowercase metric query failed: %v", err)
	}
	if len(lowerPoints) != 1 || lowerPoints[0].Value != 27 {
		t.Fatalf("expected lowercase metric query to return canonicalized point, got %+v", lowerPoints)
	}

	upperPoints, err := store.Query("agent", "agent-1", "CPU", ts.Add(-time.Second), ts.Add(time.Second), 0)
	if err != nil {
		t.Fatalf("uppercase metric query failed: %v", err)
	}
	if len(upperPoints) != 1 || upperPoints[0].Value != 27 {
		t.Fatalf("expected uppercase metric query to normalize to canonical metric type, got %+v", upperPoints)
	}

	all, err := store.QueryAll("agent", "agent-1", ts.Add(-time.Second), ts.Add(time.Second), 0)
	if err != nil {
		t.Fatalf("query all metric case canonicalization failed: %v", err)
	}
	if got := len(all["cpu"]); got != 1 {
		t.Fatalf("expected canonical lowercase metric key after normalization, got %+v", all)
	}
}

func TestStoreQueryAllBatchCanonicalizesAndDeduplicatesIdentifiers(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(DefaultConfig(dir))
	if err != nil {
		t.Fatalf("NewStore returned error: %v", err)
	}
	defer store.Close()
	ts := time.Unix(1710000100, 0)

	store.WriteBatchSync([]WriteMetric{
		{ResourceType: "agent", ResourceID: "agent-1", MetricType: "cpu", Value: 11.0, Timestamp: ts, Tier: TierRaw},
		{ResourceType: "agent", ResourceID: "agent-2", MetricType: "cpu", Value: 22.0, Timestamp: ts, Tier: TierRaw},
	})

	result, err := store.QueryAllBatch(" agent ", []string{" agent-1 ", "agent-1", "", " agent-2 "}, ts.Add(-time.Second), ts.Add(time.Second), 0)
	if err != nil {
		t.Fatalf("query all batch canonical identifiers failed: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 unique resources after normalization, got %+v", result)
	}
	if got := len(result["agent-1"]["cpu"]); got != 1 {
		t.Fatalf("expected agent-1 cpu metric after normalization, got %+v", result["agent-1"])
	}
	if got := len(result["agent-2"]["cpu"]); got != 1 {
		t.Fatalf("expected agent-2 cpu metric after normalization, got %+v", result["agent-2"])
	}
}
