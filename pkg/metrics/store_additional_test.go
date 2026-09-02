package metrics

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func TestDefaultConfigUsesLargerWriteBuffer(t *testing.T) {
	cfg := DefaultConfig("/tmp/pulse")
	if cfg.WriteBufferSize != 500 {
		t.Fatalf("WriteBufferSize = %d, want 500", cfg.WriteBufferSize)
	}
	if cfg.RollupInterval != defaultRollupInterval {
		t.Fatalf("RollupInterval = %v, want %v", cfg.RollupInterval, defaultRollupInterval)
	}
}

func TestNormalizeRollupInterval(t *testing.T) {
	tests := []struct {
		name         string
		interval     time.Duration
		rawRetention time.Duration
		want         time.Duration
	}{
		{
			name:         "default",
			interval:     0,
			rawRetention: 2 * time.Hour,
			want:         defaultRollupInterval,
		},
		{
			name:         "minimum",
			interval:     time.Minute,
			rawRetention: 2 * time.Hour,
			want:         minRollupInterval,
		},
		{
			name:         "configured",
			interval:     30 * time.Minute,
			rawRetention: 2 * time.Hour,
			want:         30 * time.Minute,
		},
		{
			name:         "bounded by raw retention",
			interval:     3 * time.Hour,
			rawRetention: 2 * time.Hour,
			want:         time.Hour,
		},
		{
			name:         "no raw retention bound",
			interval:     2 * time.Hour,
			rawRetention: 0,
			want:         2 * time.Hour,
		},
		{
			name:         "short raw retention still honors minimum",
			interval:     20 * time.Minute,
			rawRetention: 6 * time.Minute,
			want:         minRollupInterval,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeRollupInterval(tt.interval, tt.rawRetention); got != tt.want {
				t.Fatalf("normalizeRollupInterval(%v, %v) = %v, want %v", tt.interval, tt.rawRetention, got, tt.want)
			}
		})
	}
}

func TestStoreCoalesceQueuedBatches(t *testing.T) {
	store := &Store{
		writeCh: make(chan writeRequest, 4),
	}

	initial := writeRequest{
		metrics: []bufferedMetric{
			{resourceType: "vm", resourceID: "vm-1", metricType: "cpu", value: 10},
		},
	}
	store.writeCh <- writeRequest{metrics: []bufferedMetric{
		{resourceType: "vm", resourceID: "vm-1", metricType: "memory", value: 20},
	}}
	store.writeCh <- writeRequest{metrics: []bufferedMetric{
		{resourceType: "vm", resourceID: "vm-2", metricType: "cpu", value: 30},
	}}

	combined := store.coalesceQueuedRequests(initial)
	if len(combined) != 3 {
		t.Fatalf("expected 3 combined requests, got %d", len(combined))
	}
	totalMetrics := 0
	for _, req := range combined {
		totalMetrics += len(req.metrics)
	}
	if totalMetrics != 3 {
		t.Fatalf("expected 3 combined metrics, got %d", totalMetrics)
	}
	if len(store.writeCh) != 0 {
		t.Fatalf("expected queued batches to be drained, got %d remaining", len(store.writeCh))
	}
}

func TestStoreWriteBatchSync(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(DefaultConfig(dir))
	if err != nil {
		t.Fatalf("NewStore returned error: %v", err)
	}
	defer store.Close()

	ts := time.Now().UTC().Truncate(time.Second)
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

func TestStoreWriteBatchSyncSerializesConcurrentPollers(t *testing.T) {
	store, err := NewStore(DefaultConfig(t.TempDir()))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()
	if err := store.WaitForMaintenance(10 * time.Second); err != nil {
		t.Fatalf("WaitForMaintenance: %v", err)
	}

	// Hold SQLite's WAL writer lock while several platform pollers submit
	// synchronous batches. Only the store's single ingestion worker should
	// wait on a database connection; callers must queue before the pool instead
	// of consuming every reader connection with competing write transactions.
	blockingTx, err := store.db.Begin()
	if err != nil {
		t.Fatalf("begin blocking transaction: %v", err)
	}
	if _, err := blockingTx.Exec(`
		INSERT INTO metrics (resource_type, resource_id, metric_type, value, timestamp, tier)
		VALUES ('node', 'blocker', 'cpu', 1, ?, 'raw')
	`, time.Now().UTC().Unix()); err != nil {
		_ = blockingTx.Rollback()
		t.Fatalf("lock WAL writer: %v", err)
	}

	const pollers = 8
	start := make(chan struct{})
	done := make(chan struct{}, pollers)
	base := time.Now().UTC().Truncate(time.Second)
	for i := 0; i < pollers; i++ {
		i := i
		go func() {
			<-start
			store.WriteBatchSync([]WriteMetric{{
				ResourceType: "node",
				ResourceID:   "poller-" + strconv.Itoa(i),
				MetricType:   "cpu",
				Value:        float64(i),
				Timestamp:    base,
				Tier:         TierRaw,
			}})
			done <- struct{}{}
		}()
	}
	close(start)

	maxInUse := 0
	observationDeadline := time.Now().Add(750 * time.Millisecond)
	for time.Now().Before(observationDeadline) {
		if inUse := store.db.DB.Stats().InUse; inUse > maxInUse {
			maxInUse = inUse
		}
		time.Sleep(5 * time.Millisecond)
	}

	if err := blockingTx.Rollback(); err != nil {
		t.Fatalf("release WAL writer: %v", err)
	}
	for i := 0; i < pollers; i++ {
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatal("concurrent synchronous metrics writes did not drain")
		}
	}

	if maxInUse > 2 {
		t.Fatalf("concurrent writers consumed %d database connections, want at most blocker plus one ingestion worker", maxInUse)
	}

	var persisted int
	if err := store.db.QueryRow(`
		SELECT COUNT(*)
		FROM metrics
		WHERE resource_type = 'node' AND resource_id LIKE 'poller-%'
	`).Scan(&persisted); err != nil {
		t.Fatalf("count persisted poller metrics: %v", err)
	}
	if persisted != pollers {
		t.Fatalf("persisted poller metrics = %d, want %d", persisted, pollers)
	}
}

func TestStoreWriteBatchUpsertsDuplicateRawMetric(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultConfig(dir)
	cfg.DBPath = filepath.Join(dir, "metrics-duplicates.db")
	cfg.FlushInterval = time.Hour

	store, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("NewStore returned error: %v", err)
	}
	defer store.Close()

	ts := time.Now().UTC().Truncate(time.Second)
	store.writeBatch([]bufferedMetric{
		{resourceType: "agent", resourceID: "host-1", metricType: "memory", value: 42, timestamp: ts, tier: TierRaw},
		{resourceType: "agent", resourceID: "host-1", metricType: "memory", value: 55, timestamp: ts, tier: TierRaw},
	})

	points, err := store.Query("agent", "host-1", "memory", ts.Add(-time.Second), ts.Add(time.Second), 0)
	if err != nil {
		t.Fatalf("Query returned error: %v", err)
	}
	if len(points) != 1 {
		t.Fatalf("expected duplicate timestamp to collapse to one point, got %+v", points)
	}
	if points[0].Value != 55 {
		t.Fatalf("expected latest duplicate value to win, got %+v", points[0])
	}
}

func TestCoalesceMetricBatchReducesPreSQLCardinalityAndPreservesLatestValue(t *testing.T) {
	ts := time.Unix(1700000000, 123)
	batch := []bufferedMetric{
		{resourceType: "agent", resourceID: "host-1", metricType: "cpu", value: 10, timestamp: ts, tier: TierRaw},
		{resourceType: "agent", resourceID: "host-1", metricType: "cpu", value: 20, timestamp: ts.Add(500 * time.Millisecond), tier: TierRaw},
		{resourceType: "agent", resourceID: "host-1", metricType: "memory", value: 30, timestamp: ts, tier: TierRaw},
		{resourceType: "agent", resourceID: "host-1", metricType: "cpu", value: 40, timestamp: ts, tier: TierMinute},
		{resourceType: "agent", resourceID: "host-1", metricType: "cpu", value: 50, timestamp: ts.Add(time.Second), tier: TierRaw},
	}

	coalesced := coalesceMetricBatch(batch)
	if len(coalesced) != 4 {
		t.Fatalf("expected pre-SQL batch cardinality to drop from %d to 4, got %d: %+v", len(batch), len(coalesced), coalesced)
	}

	foundRawCPU := false
	for _, metric := range coalesced {
		if metric.resourceType == "agent" &&
			metric.resourceID == "host-1" &&
			metric.metricType == "cpu" &&
			metric.timestamp.Unix() == ts.Unix() &&
			metric.tier == TierRaw {
			foundRawCPU = true
			if metric.value != 20 {
				t.Fatalf("expected latest duplicate raw CPU value to win before SQL execution, got %+v", metric)
			}
		}
	}
	if !foundRawCPU {
		t.Fatalf("expected coalesced batch to retain raw CPU sample: %+v", coalesced)
	}
}

func TestResolveStoreDBPathCanonicalizesOwnedPath(t *testing.T) {
	root := t.TempDir()
	rawPath := filepath.Join(root, "metrics", "..", "metrics", "metrics.db")

	resolved, err := resolveStoreDBPath("  " + rawPath + "  ")
	if err != nil {
		t.Fatalf("resolveStoreDBPath() error = %v", err)
	}

	want := filepath.Join(filepath.Clean(filepath.Join(root, "metrics")), "metrics.db")
	if resolved != want {
		t.Fatalf("resolveStoreDBPath() = %q, want %q", resolved, want)
	}
}

func TestResolveStoreDBPathRejectsBlank(t *testing.T) {
	if _, err := resolveStoreDBPath(" \t "); err == nil {
		t.Fatal("expected blank DB path to be rejected")
	}
}

func TestNewStoreCanonicalizesDBPath(t *testing.T) {
	root := t.TempDir()
	cfg := DefaultConfig(root)
	cfg.DBPath = filepath.Join(root, "metrics", "..", "metrics", "tenant-metrics.db")

	store, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	defer store.Close()

	want := filepath.Join(filepath.Clean(filepath.Join(root, "metrics")), "tenant-metrics.db")
	if store.config.DBPath != want {
		t.Fatalf("store.config.DBPath = %q, want %q", store.config.DBPath, want)
	}
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("expected canonical metrics DB at %q: %v", want, err)
	}
}

func TestEnsureOwnerOnlyDirHardensOwnedDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "metrics")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	if err := ensureOwnerOnlyDir(dir); err != nil {
		t.Fatalf("ensureOwnerOnlyDir: %v", err)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if got := info.Mode().Perm(); got != privateDirPerm {
		t.Fatalf("directory permissions = %#o, want %#o", got, privateDirPerm)
	}
}

func TestEnsureOwnerOnlyDirRejectsFilesystemRoot(t *testing.T) {
	root := string(filepath.Separator)
	before, err := os.Stat(root)
	if err != nil {
		t.Fatalf("Stat root: %v", err)
	}

	err = ensureOwnerOnlyDir(root)
	if err == nil || !strings.Contains(err.Error(), "dedicated subdirectory") {
		t.Fatalf("ensureOwnerOnlyDir(root) error = %v, want dedicated-subdirectory guidance", err)
	}

	after, err := os.Stat(root)
	if err != nil {
		t.Fatalf("Stat root after rejection: %v", err)
	}
	if after.Mode().Perm() != before.Mode().Perm() {
		t.Fatalf("root permissions changed from %#o to %#o", before.Mode().Perm(), after.Mode().Perm())
	}
}

func TestEnsureOwnerOnlyDirRejectsSystemTemporaryDirectory(t *testing.T) {
	dir := os.TempDir()
	before, err := os.Stat(dir)
	if err != nil {
		t.Skipf("temporary directory is unavailable: %v", err)
	}
	if directoryOwnedByCurrentUser(before) {
		t.Skip("temporary directory is owned by the test user on this platform")
	}

	err = ensureOwnerOnlyDir(dir)
	if err == nil || (!strings.Contains(err.Error(), "is shared") && !strings.Contains(err.Error(), "not owned by the current user")) {
		t.Fatalf("ensureOwnerOnlyDir(%q) error = %v, want shared-directory or ownership guidance", dir, err)
	}

	after, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Stat shared directory after rejection: %v", err)
	}
	if after.Mode().Perm() != before.Mode().Perm() {
		t.Fatalf("shared directory permissions changed from %#o to %#o", before.Mode().Perm(), after.Mode().Perm())
	}
}

func TestEnsureOwnerOnlyDirRejectsOwnedStickySharedDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "shared")
	if err := os.Mkdir(dir, 0o777); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	if err := os.Chmod(dir, os.ModeSticky|0o777); err != nil {
		t.Skipf("sticky directories are unavailable: %v", err)
	}

	err := ensureOwnerOnlyDir(dir)
	if err == nil || !strings.Contains(err.Error(), "is shared") {
		t.Fatalf("ensureOwnerOnlyDir(sticky shared directory) error = %v, want shared-directory guidance", err)
	}

	after, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Stat shared directory after rejection: %v", err)
	}
	if after.Mode().Perm() != 0o777 || after.Mode()&os.ModeSticky == 0 {
		t.Fatalf("shared directory mode changed to %v", after.Mode())
	}
}

func TestEnsureOwnerOnlyDirRejectsSymlink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	link := filepath.Join(root, "metrics")
	if err := os.Mkdir(target, privateDirPerm); err != nil {
		t.Fatalf("Mkdir target: %v", err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}

	err := ensureOwnerOnlyDir(link)
	if err == nil || !strings.Contains(err.Error(), "symlink is not allowed") {
		t.Fatalf("ensureOwnerOnlyDir(symlink) error = %v, want symlink rejection", err)
	}
}

func TestStoreFilesOwnerOnlyUnderPermissiveUmask(t *testing.T) {
	oldUmask := syscall.Umask(0o022)
	defer syscall.Umask(oldUmask)

	dir := t.TempDir()
	cfg := DefaultConfig(dir)
	cfg.DBPath = filepath.Join(dir, "metrics-perm-test.db")
	cfg.FlushInterval = time.Hour

	store, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	ts := time.Now().UTC().Truncate(time.Second)
	store.writeBatch([]bufferedMetric{
		{resourceType: "vm", resourceID: "vm-1", metricType: "cpu", value: 1.0, timestamp: ts, tier: TierRaw},
	})

	artifactPaths := []string{cfg.DBPath, cfg.DBPath + "-wal", cfg.DBPath + "-shm"}
	for _, path := range artifactPaths {
		fi, err := os.Stat(path)
		if err != nil {
			t.Errorf("%s: os.Stat() failed: %v", path, err)
			continue
		}
		gotPerm := fi.Mode().Perm()
		if gotPerm != 0o600 {
			t.Errorf("%s: expected 0600, got %#o", path, gotPerm)
		}
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

	stats := store.GetStats()
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
		writeCh: make(chan writeRequest, 1),
	}

	store.writeCh <- writeRequest{metrics: []bufferedMetric{{
		resourceType: "vm",
		resourceID:   "vm-102",
		metricType:   "memory",
		value:        12,
		timestamp:    time.Unix(1000, 0),
		tier:         TierRaw,
	}}}

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

func TestStoreFlushMakesQueuedWritesVisible(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultConfig(dir)
	cfg.WriteBufferSize = 1
	cfg.FlushInterval = time.Hour

	store, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("NewStore returned error: %v", err)
	}
	defer store.Close()

	ts := time.Now().UTC().Truncate(time.Second)
	store.Write("vm", "vm-1", "cpu", 42, ts)

	// With a buffer size of 1, the write above is already queued
	// asynchronously. Flush must still wait for that queued batch to commit.
	store.Flush()

	points, err := store.Query("vm", "vm-1", "cpu", ts.Add(-time.Second), ts.Add(time.Second), 0)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(points) != 1 || points[0].Value != 42 {
		t.Fatalf("expected flushed metric to be immediately visible, got %v", points)
	}
}

func TestNewStoreDefersStartupMaintenance(t *testing.T) {
	previousHook := startupMaintenanceHook

	// The hook parks the maintenance worker before any startup work runs, so
	// startup maintenance cannot complete until the test releases it.
	started := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	startupMaintenanceHook = func() {
		close(started)
		<-release
	}

	dir := t.TempDir()
	cfg := DefaultConfig(dir)
	cfg.FlushInterval = time.Hour

	done := make(chan struct{})
	var (
		store *Store
		err   error
	)
	go func() {
		store, err = NewStore(cfg)
		close(done)
	}()

	// Release the worker and wait for NewStore before restoring the hook, so a
	// failed run never leaks a parked store into the next test.
	t.Cleanup(func() {
		releaseOnce.Do(func() { close(release) })
		<-done
		if store != nil {
			store.Close()
		}
		startupMaintenanceHook = previousHook
	})

	// Deferral is proven by ordering, not by a wall-clock bound: NewStore must
	// return while the worker is still parked in the hook. If startup
	// maintenance ever ran inline again, this receive would block until the
	// package test timeout instead of flaking on a slow CI disk.
	<-started
	<-done
	if err != nil {
		t.Fatalf("NewStore returned error: %v", err)
	}

	releaseOnce.Do(func() { close(release) })
}

func TestStoreWaitForMaintenanceWaitsForQueuedStartupWork(t *testing.T) {
	previousHook := startupMaintenanceHook
	defer func() {
		startupMaintenanceHook = previousHook
	}()

	started := make(chan struct{})
	release := make(chan struct{})
	startupMaintenanceHook = func() {
		close(started)
		<-release
	}

	dir := t.TempDir()
	cfg := DefaultConfig(dir)
	cfg.FlushInterval = time.Hour

	store, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("NewStore returned error: %v", err)
	}
	defer store.Close()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("startup maintenance did not start")
	}

	waitDone := make(chan error, 1)
	go func() {
		waitDone <- store.WaitForMaintenance(time.Second)
	}()

	select {
	case err := <-waitDone:
		t.Fatalf("WaitForMaintenance returned before startup maintenance completed: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	close(release)

	select {
	case err := <-waitDone:
		if err != nil {
			t.Fatalf("WaitForMaintenance returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("WaitForMaintenance did not return after startup maintenance completed")
	}
}

// A connection pool of one queued every UI history read behind metric-flush
// commits, freezing charts whenever a commit picked up a WAL checkpoint
// (#1601). Reads must proceed on their own connections while a write
// transaction holds the WAL write lock.
func TestQueriesDoNotQueueBehindWriteTransaction(t *testing.T) {
	store, err := NewStore(DefaultConfig(t.TempDir()))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	if err := store.WaitForMaintenance(10 * time.Second); err != nil {
		t.Fatalf("WaitForMaintenance: %v", err)
	}

	if got := store.db.DB.Stats().MaxOpenConnections; got < 2 {
		t.Fatalf("MaxOpenConnections = %d, want at least 2 so reads do not serialize behind writes", got)
	}

	tx, err := store.db.Begin()
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(
		`INSERT INTO metrics (resource_type, resource_id, metric_type, value, timestamp, tier)
		 VALUES ('node', 'n1', 'cpu', 1.0, ?, 'raw')`,
		time.Now().Unix(),
	); err != nil {
		t.Fatalf("write inside transaction: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := store.Query("node", "n1", "cpu", time.Now().Add(-time.Hour), time.Now(), 60)
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Query during open write transaction: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Query blocked behind the open write transaction")
	}
}

// Startup maintenance used to execute on the same worker that drained
// writeCh. A large retention/VACUUM pass could therefore fill the bounded
// write queue and drop live samples. Keep maintenance blocked here and prove
// that ingestion and its Flush barrier remain operational.
func TestStartupMaintenanceDoesNotBlockIngestionWorker(t *testing.T) {
	maintenanceEntered := make(chan struct{})
	releaseMaintenance := make(chan struct{})
	var releaseOnce sync.Once
	release := func() {
		releaseOnce.Do(func() { close(releaseMaintenance) })
	}
	t.Cleanup(release)

	previousHook := startupMaintenanceHook
	startupMaintenanceHook = func() {
		close(maintenanceEntered)
		<-releaseMaintenance
	}
	t.Cleanup(func() { startupMaintenanceHook = previousHook })

	config := DefaultConfig(t.TempDir())
	config.WriteBufferSize = 1
	config.FlushInterval = time.Hour

	store, err := NewStore(config)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	select {
	case <-maintenanceEntered:
	case <-time.After(5 * time.Second):
		t.Fatal("startup maintenance did not begin")
	}

	timestamp := time.Now().UTC().Truncate(time.Second)
	store.Write("node", "n1", "cpu", 42, timestamp)

	flushed := make(chan struct{})
	go func() {
		store.Flush()
		close(flushed)
	}()

	select {
	case <-flushed:
	case <-time.After(5 * time.Second):
		t.Fatal("metric ingestion blocked behind startup maintenance")
	}

	release()
	if err := store.WaitForMaintenance(10 * time.Second); err != nil {
		t.Fatalf("WaitForMaintenance: %v", err)
	}

	points, err := store.Query("node", "n1", "cpu", timestamp.Add(-time.Second), timestamp.Add(time.Second), 0)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(points) != 1 || points[0].Value != 42 {
		t.Fatalf("persisted points = %+v, want one value 42", points)
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
	ts := time.Now().UTC().Truncate(time.Second)

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

	ts := time.Now().UTC().Truncate(time.Second)
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
	ts := time.Now().UTC().Truncate(time.Second)

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

func TestStoreQueryMetricTypesBatchFiltersMetricTypes(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(DefaultConfig(dir))
	if err != nil {
		t.Fatalf("NewStore returned error: %v", err)
	}
	defer store.Close()
	ts := time.Now().UTC().Truncate(time.Second)

	store.WriteBatchSync([]WriteMetric{
		{ResourceType: "agent", ResourceID: "agent-1", MetricType: "cpu", Value: 11.0, Timestamp: ts, Tier: TierRaw},
		{ResourceType: "agent", ResourceID: "agent-1", MetricType: "memory", Value: 21.0, Timestamp: ts, Tier: TierRaw},
		{ResourceType: "agent", ResourceID: "agent-2", MetricType: "cpu", Value: 12.0, Timestamp: ts, Tier: TierRaw},
		{ResourceType: "agent", ResourceID: "agent-2", MetricType: "memory", Value: 22.0, Timestamp: ts, Tier: TierRaw},
	})

	result, err := store.QueryMetricTypesBatch(
		" agent ",
		[]string{" agent-1 ", "agent-2", "agent-2"},
		[]string{" CPU ", "cpu"},
		ts.Add(-time.Second),
		ts.Add(time.Second),
		0,
	)
	if err != nil {
		t.Fatalf("query metric types batch failed: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 resources after normalization, got %+v", result)
	}
	if got := len(result["agent-1"]["cpu"]); got != 1 {
		t.Fatalf("expected agent-1 cpu metric, got %+v", result["agent-1"])
	}
	if got := len(result["agent-2"]["cpu"]); got != 1 {
		t.Fatalf("expected agent-2 cpu metric, got %+v", result["agent-2"])
	}
	if _, ok := result["agent-1"]["memory"]; ok {
		t.Fatalf("expected metric filter to exclude memory for agent-1, got %+v", result["agent-1"])
	}
	if _, ok := result["agent-2"]["memory"]; ok {
		t.Fatalf("expected metric filter to exclude memory for agent-2, got %+v", result["agent-2"])
	}
}

// TestStoreRetentionReclaimsFreePages guards the fix for unbounded metrics.db
// growth. A pre-existing freelist backlog must drain on a normal retention pass
// even in an hour where nothing new was deleted. The previous code reclaimed
// only when that cycle deleted rows, and only 5000 pages at a time, so on busy
// instances the freelist outpaced reclaim and the file bloated to GBs over
// ~60MB of live data. Reclaiming every cycle, proportional to the freelist,
// returns the freed pages to the OS so the file shrinks.
func TestStoreRetentionReclaimsFreePages(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultConfig(dir)
	cfg.DBPath = filepath.Join(dir, "metrics-reclaim.db")
	cfg.RetentionRaw = time.Minute
	cfg.FlushInterval = time.Hour

	store, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("NewStore returned error: %v", err)
	}
	defer store.Close()
	if err := store.WaitForMaintenance(10 * time.Second); err != nil {
		t.Fatalf("wait for startup maintenance: %v", err)
	}

	// Build a freelist backlog: insert many rows (kept unique by timestamp) then
	// delete them directly, so the rows are already gone before runRetention.
	tx, err := store.db.Begin()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	stmt, err := tx.Prepare(`INSERT INTO metrics (resource_type, resource_id, metric_type, value, timestamp, tier) VALUES ('vm', 'vm-101', 'cpu', 1.0, ?, 'raw')`)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	base := time.Now().Add(-72 * time.Hour).Unix()
	for i := 0; i < 20000; i++ {
		if _, err := stmt.Exec(base + int64(i)); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	stmt.Close()
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if _, err := store.db.Exec(`DELETE FROM metrics`); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := store.db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		t.Fatalf("checkpoint: %v", err)
	}

	var freelistBefore, pagesBefore int64
	if err := store.db.QueryRow(`PRAGMA freelist_count`).Scan(&freelistBefore); err != nil {
		t.Fatalf("freelist before: %v", err)
	}
	if err := store.db.QueryRow(`PRAGMA page_count`).Scan(&pagesBefore); err != nil {
		t.Fatalf("page_count before: %v", err)
	}
	if freelistBefore == 0 {
		t.Fatalf("test precondition failed: expected a freelist backlog, got 0 free pages")
	}

	// Nothing to prune this cycle (table is already empty), but the backlog must
	// still be reclaimed. The old code skipped reclaim entirely when nothing was
	// deleted, so the file would not shrink and this would fail.
	store.runRetention()

	var freelistAfter, pagesAfter int64
	if err := store.db.QueryRow(`PRAGMA freelist_count`).Scan(&freelistAfter); err != nil {
		t.Fatalf("freelist after: %v", err)
	}
	if err := store.db.QueryRow(`PRAGMA page_count`).Scan(&pagesAfter); err != nil {
		t.Fatalf("page_count after: %v", err)
	}
	if pagesAfter >= pagesBefore {
		t.Fatalf("expected database file to shrink after reclaim: pages before=%d after=%d (freelist %d -> %d)", pagesBefore, pagesAfter, freelistBefore, freelistAfter)
	}
	if freelistAfter >= freelistBefore {
		t.Fatalf("expected freelist to shrink after reclaim: %d -> %d", freelistBefore, freelistAfter)
	}
}

func TestCommercialHistoryRetentionNeverExpandsOperatorPolicy(t *testing.T) {
	store := &Store{}
	now := time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC)
	store.SetCommercialHistoryRetention(14, now)
	if got := store.effectiveRetention(7*24*time.Hour, now); got != 7*24*time.Hour {
		t.Fatalf("commercial ceiling expanded shorter operator retention: %v", got)
	}
}
