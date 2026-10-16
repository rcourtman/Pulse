package config_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestExtendedHistoryReadActivityPersistsOnlyBoundedDailyCounts(t *testing.T) {
	dir := t.TempDir()
	persistence := config.NewConfigPersistence(dir)
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	for _, at := range []time.Time{now.AddDate(0, 0, -40), now.AddDate(0, 0, -30), now, now} {
		if err := persistence.RecordSuccessfulExtendedHistoryRead(at); err != nil {
			t.Fatal(err)
		}
	}
	// Reopening models a restart. No request-level data is needed for continuity.
	reopened := config.NewConfigPersistence(dir)
	if got := reopened.CountSuccessfulExtendedHistoryReadsSince(now.AddDate(0, 0, -30), now); got != 3 {
		t.Fatalf("retained reads = %d, want 3", got)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "history_read_activity.json"))
	if err != nil {
		t.Fatal(err)
	}
	var stored map[string]map[string]int
	if err := json.Unmarshal(raw, &stored); err != nil {
		t.Fatal(err)
	}
	if len(stored) != 1 || len(stored["daily_successful_reads_over_7d"]) != 2 {
		t.Fatalf("unexpected retained fields or days: %s", raw)
	}
	info, err := os.Stat(filepath.Join(dir, "history_read_activity.json"))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("tally mode must be 0600: %v %v", info, err)
	}
}

func TestExtendedHistoryReadActivitySerializesConcurrentRequests(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	now := time.Now().UTC()
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := persistence.RecordSuccessfulExtendedHistoryRead(now); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if got := persistence.CountSuccessfulExtendedHistoryReadsSince(now.AddDate(0, 0, -30), now); got != 16 {
		t.Fatalf("reads = %d, want 16", got)
	}
}
