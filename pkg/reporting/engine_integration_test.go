package reporting

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
	"github.com/stretchr/testify/require"
)

// TestReportEngineWithMetricsStore verifies the full data pipeline from
// metrics store writes through report generation. This is a regression test
// for the zero-datapoints bug reported in GitHub issue #1186.
func TestReportEngineWithMetricsStore(t *testing.T) {
	dir := t.TempDir()
	store, err := metrics.NewStore(metrics.StoreConfig{
		DBPath:          filepath.Join(dir, "metrics.db"),
		WriteBufferSize: 10,
		FlushInterval:   100 * time.Millisecond,
		RetentionRaw:    24 * time.Hour,
		RetentionMinute: 7 * 24 * time.Hour,
		RetentionHourly: 30 * 24 * time.Hour,
		RetentionDaily:  90 * 24 * time.Hour,
	})
	if err != nil {
		t.Fatalf("failed to create metrics store: %v", err)
	}
	defer store.Close()

	engine := NewReportEngine(EngineConfig{MetricsStore: store})

	// Write metrics data for a node over the last hour
	now := time.Now()
	nodeID := "pve-prod-pve1"
	for i := 0; i < 12; i++ {
		ts := now.Add(time.Duration(-60+i*5) * time.Minute)
		store.Write("node", nodeID, "cpu", float64(40+i*2), ts)
		store.Write("node", nodeID, "memory", float64(55+i), ts)
	}

	// Flush to ensure data is written to SQLite
	store.Flush()

	// Generate a report for the last 2 hours (uses raw tier)
	req := MetricReportRequest{
		ResourceType: "node",
		ResourceID:   nodeID,
		Start:        now.Add(-2 * time.Hour),
		End:          now.Add(time.Minute), // slight buffer
		Format:       FormatCSV,
	}

	var data []byte
	var contentType string
	require.Eventually(t, func() bool {
		var genErr error
		data, contentType, genErr = engine.Generate(req)
		if genErr != nil || contentType != "text/csv" {
			return false
		}
		return countCSVDataRows(string(data)) > 0
	}, 2*time.Second, 25*time.Millisecond)

	if contentType != "text/csv" {
		t.Errorf("expected text/csv, got %s", contentType)
	}

	csv := string(data)

	// Verify report has data points (CSV uses display names)
	if !strings.Contains(csv, "CPU") {
		t.Error("report missing CPU metric data")
	}
	if !strings.Contains(csv, "Memory") {
		t.Error("report missing memory metric data")
	}

	// Verify the report header includes the correct resource ID
	if !strings.Contains(csv, nodeID) {
		t.Errorf("report missing resource ID %s", nodeID)
	}

	// Verify data rows exist (not just headers)
	dataRows := countCSVDataRows(csv)
	if dataRows == 0 {
		t.Fatal("report has zero data rows — metrics store data did not reach report output")
	}
	t.Logf("Report generated with %d data rows", dataRows)
}

// TestReportEngineWithMetricsStore_VM verifies reports work for VM resources
// with composite IDs (instance:node:vmid format).
func TestReportEngineWithMetricsStore_VM(t *testing.T) {
	dir := t.TempDir()
	store, err := metrics.NewStore(metrics.StoreConfig{
		DBPath:          filepath.Join(dir, "metrics.db"),
		WriteBufferSize: 10,
		FlushInterval:   100 * time.Millisecond,
		RetentionRaw:    24 * time.Hour,
		RetentionMinute: 7 * 24 * time.Hour,
		RetentionHourly: 30 * 24 * time.Hour,
		RetentionDaily:  90 * 24 * time.Hour,
	})
	if err != nil {
		t.Fatalf("failed to create metrics store: %v", err)
	}
	defer store.Close()

	engine := NewReportEngine(EngineConfig{MetricsStore: store})

	now := time.Now()
	vmID := "pve-prod:pve1:100"
	for i := 0; i < 6; i++ {
		ts := now.Add(time.Duration(-30+i*5) * time.Minute)
		store.Write("vm", vmID, "cpu", float64(30+i*5), ts)
		store.Write("vm", vmID, "memory", float64(60+i*2), ts)
		store.Write("vm", vmID, "disk", float64(40+i), ts)
	}

	store.Flush()

	req := MetricReportRequest{
		ResourceType: "vm",
		ResourceID:   vmID,
		Start:        now.Add(-1 * time.Hour),
		End:          now.Add(time.Minute),
		Format:       FormatPDF,
	}

	var data []byte
	var contentType string
	require.Eventually(t, func() bool {
		var genErr error
		data, contentType, genErr = engine.Generate(req)
		if genErr != nil || contentType != "application/pdf" {
			return false
		}
		return len(data) >= 1000
	}, 2*time.Second, 25*time.Millisecond)

	if contentType != "application/pdf" {
		t.Errorf("expected application/pdf, got %s", contentType)
	}

	if len(data) < 1000 {
		t.Errorf("PDF seems too small (%d bytes), likely has no data", len(data))
	}
}

// TestReportEngineStaleStoreAfterClose verifies that querying a closed
// metrics store returns an error rather than silently returning zero results.
// This documents the behavior that causes blank reports after a monitor reload.
func TestReportEngineStaleStoreAfterClose(t *testing.T) {
	dir := t.TempDir()
	store, err := metrics.NewStore(metrics.StoreConfig{
		DBPath:          filepath.Join(dir, "metrics.db"),
		WriteBufferSize: 10,
		FlushInterval:   100 * time.Millisecond,
		RetentionRaw:    24 * time.Hour,
		RetentionMinute: 7 * 24 * time.Hour,
		RetentionHourly: 30 * 24 * time.Hour,
		RetentionDaily:  90 * 24 * time.Hour,
	})
	if err != nil {
		t.Fatalf("failed to create metrics store: %v", err)
	}

	engine := NewReportEngine(EngineConfig{MetricsStore: store})

	// Write some data
	now := time.Now()
	store.Write("node", "test-node", "cpu", 50.0, now)
	store.Flush()

	// Close the store (simulates what happens during monitor reload)
	store.Close()

	// Attempt to generate a report with the stale store reference
	req := MetricReportRequest{
		ResourceType: "node",
		ResourceID:   "test-node",
		Start:        now.Add(-1 * time.Hour),
		End:          now.Add(time.Minute),
		Format:       FormatCSV,
	}

	_, _, err = engine.Generate(req)
	// After a store is closed, the engine should return an error (not silent empty results).
	if err == nil {
		t.Fatal("expected error when generating a report with a closed metrics store")
	}
}

// TestReportEngineReplacedStore verifies that replacing the global engine
// with a new one pointing to a fresh store produces reports with data.
func TestReportEngineReplacedStore(t *testing.T) {
	dir1 := t.TempDir()
	store1, err := metrics.NewStore(metrics.StoreConfig{
		DBPath:          filepath.Join(dir1, "metrics.db"),
		WriteBufferSize: 10,
		FlushInterval:   100 * time.Millisecond,
		RetentionRaw:    24 * time.Hour,
		RetentionMinute: 7 * 24 * time.Hour,
		RetentionHourly: 30 * 24 * time.Hour,
		RetentionDaily:  90 * 24 * time.Hour,
	})
	if err != nil {
		t.Fatalf("failed to create store1: %v", err)
	}

	engine1 := NewReportEngine(EngineConfig{MetricsStore: store1})
	SetEngine(engine1)
	t.Cleanup(func() { SetEngine(nil) })

	now := time.Now()
	store1.Write("node", "node-1", "cpu", 50.0, now)
	store1.Flush()

	// Simulate reload: close old store, create new one (same DB file)
	store1.Close()

	store2, err := metrics.NewStore(metrics.StoreConfig{
		DBPath:          filepath.Join(dir1, "metrics.db"), // Same file
		WriteBufferSize: 10,
		FlushInterval:   100 * time.Millisecond,
		RetentionRaw:    24 * time.Hour,
		RetentionMinute: 7 * 24 * time.Hour,
		RetentionHourly: 30 * 24 * time.Hour,
		RetentionDaily:  90 * 24 * time.Hour,
	})
	if err != nil {
		t.Fatalf("failed to create store2: %v", err)
	}
	defer store2.Close()

	// Write additional data via new store
	store2.Write("node", "node-1", "cpu", 55.0, now.Add(5*time.Minute))
	store2.Flush()

	// Replace global engine with new store
	engine2 := NewReportEngine(EngineConfig{MetricsStore: store2})
	SetEngine(engine2)

	req := MetricReportRequest{
		ResourceType: "node",
		ResourceID:   "node-1",
		Start:        now.Add(-1 * time.Hour),
		End:          now.Add(1 * time.Hour),
		Format:       FormatCSV,
	}

	var data []byte
	require.Eventually(t, func() bool {
		var genErr error
		data, _, genErr = engine2.Generate(req)
		return genErr == nil && countCSVDataRows(string(data)) > 0
	}, 2*time.Second, 25*time.Millisecond)

	csv := string(data)
	dataRows := countCSVDataRows(csv)

	if dataRows == 0 {
		t.Fatal("report has zero data rows after store replacement — fix did not work")
	}
	t.Logf("After store replacement: report generated with %d data rows", dataRows)
}

// TestReportEngineGetterSurvivesReload verifies that an engine configured with
// MetricsStoreGetter automatically picks up a new store after the old one is
// closed, without needing to recreate the engine. This is the key behavior
// that prevents stale store references after monitor reloads.
func TestReportEngineGetterSurvivesReload(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "metrics.db")

	newStore := func() *metrics.Store {
		s, err := metrics.NewStore(metrics.StoreConfig{
			DBPath:          dbPath,
			WriteBufferSize: 10,
			FlushInterval:   100 * time.Millisecond,
			RetentionRaw:    24 * time.Hour,
			RetentionMinute: 7 * 24 * time.Hour,
			RetentionHourly: 30 * 24 * time.Hour,
			RetentionDaily:  90 * 24 * time.Hour,
		})
		if err != nil {
			t.Fatalf("failed to create store: %v", err)
		}
		return s
	}

	// Simulate what reloadableMonitor.GetMonitor().GetMetricsStore() does:
	// returns whatever the current store is.
	var currentStore *metrics.Store
	currentStore = newStore()

	engine := NewReportEngine(EngineConfig{
		MetricsStoreGetter: func() *metrics.Store {
			return currentStore
		},
	})

	now := time.Now()
	req := MetricReportRequest{
		ResourceType: "node",
		ResourceID:   "node-1",
		Start:        now.Add(-1 * time.Hour),
		End:          now.Add(1 * time.Hour),
		Format:       FormatCSV,
	}

	// Phase 1: Write data and generate a report — should work
	currentStore.Write("node", "node-1", "cpu", 50.0, now)
	currentStore.Flush()

	var data []byte
	require.Eventually(t, func() bool {
		var genErr error
		data, _, genErr = engine.Generate(req)
		return genErr == nil && countCSVDataRows(string(data)) > 0
	}, 2*time.Second, 25*time.Millisecond)
	rows1 := countCSVDataRows(string(data))
	if rows1 == 0 {
		t.Fatal("phase 1: expected data rows, got zero")
	}
	t.Logf("Phase 1 (before reload): %d data rows", rows1)

	// Phase 2: Simulate reload — close old store, create new one
	currentStore.Close()
	currentStore = newStore()

	// Write new data through the new store
	currentStore.Write("node", "node-1", "cpu", 60.0, now.Add(5*time.Minute))
	currentStore.Write("node", "node-1", "cpu", 70.0, now.Add(10*time.Minute))
	currentStore.Flush()

	// Phase 3: Same engine instance, should use the new store via getter
	require.Eventually(t, func() bool {
		var genErr error
		data, _, genErr = engine.Generate(req)
		return genErr == nil && countCSVDataRows(string(data)) > 0
	}, 2*time.Second, 25*time.Millisecond)
	rows3 := countCSVDataRows(string(data))
	if rows3 == 0 {
		t.Fatal("phase 3: report has zero data rows after reload — getter did not resolve new store")
	}
	t.Logf("Phase 3 (after reload): %d data rows", rows3)

	// The new store should see ALL data (original + new) since it opens the same SQLite file
	if rows3 < rows1 {
		t.Errorf("phase 3: expected at least %d rows (pre-reload count), got %d", rows1, rows3)
	}

	currentStore.Close()
}

func countCSVDataRows(csv string) int {
	lines := strings.Split(csv, "\n")
	dataStarted := false
	count := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "# DATA") {
			dataStarted = true
			continue
		}
		if dataStarted && !strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "Timestamp") && line != "" {
			count++
		}
	}
	return count
}
