package conversion

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestConversionStoreRecordAndQueryRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	store, err := NewConversionStore(filepath.Join(tmp, "conversion.db"))
	if err != nil {
		t.Fatalf("NewConversionStore() error = %v", err)
	}
	defer store.Close()

	now := time.Now().UTC().Truncate(time.Second)
	ev := StoredConversionEvent{
		OrgID:          "org-a",
		EventType:      EventPaywallViewed,
		Surface:        "history_chart",
		Capability:     "long_term_metrics",
		IdempotencyKey: "paywall_viewed:history_chart:long_term_metrics:1",
		CreatedAt:      now,
	}
	if err := store.Record(ev); err != nil {
		t.Fatalf("Record() error = %v", err)
	}

	got, err := store.Query("org-a", now.Add(-time.Minute), now.Add(time.Minute), "")
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(Query()) = %d, want 1", len(got))
	}
	if got[0].OrgID != "org-a" {
		t.Fatalf("OrgID = %q, want org-a", got[0].OrgID)
	}
	if got[0].EventType != EventPaywallViewed {
		t.Fatalf("EventType = %q, want %q", got[0].EventType, EventPaywallViewed)
	}
	if got[0].Surface != "history_chart" {
		t.Fatalf("Surface = %q, want history_chart", got[0].Surface)
	}
	if got[0].Capability != "long_term_metrics" {
		t.Fatalf("Capability = %q, want long_term_metrics", got[0].Capability)
	}
	if got[0].IdempotencyKey != ev.IdempotencyKey {
		t.Fatalf("IdempotencyKey = %q, want %q", got[0].IdempotencyKey, ev.IdempotencyKey)
	}
}

func TestConversionStoreIdempotency(t *testing.T) {
	tmp := t.TempDir()
	store, err := NewConversionStore(filepath.Join(tmp, "conversion.db"))
	if err != nil {
		t.Fatalf("NewConversionStore() error = %v", err)
	}
	defer store.Close()

	now := time.Now().UTC().Truncate(time.Second)
	ev := StoredConversionEvent{
		OrgID:          "org-a",
		EventType:      EventTrialStarted,
		Surface:        "license_panel",
		Capability:     "",
		IdempotencyKey: "trial_started:license_panel::1",
		CreatedAt:      now,
	}

	if err := store.Record(ev); err != nil {
		t.Fatalf("first Record() error = %v", err)
	}
	if err := store.Record(ev); err != nil {
		t.Fatalf("second Record() error = %v", err)
	}

	got, err := store.Query("org-a", now.Add(-time.Minute), now.Add(time.Minute), "")
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(Query()) = %d, want 1", len(got))
	}
}

func TestConversionStoreSchemaHasCreatedAtIndex(t *testing.T) {
	tmp := t.TempDir()
	store, err := NewConversionStore(filepath.Join(tmp, "conversion.db"))
	if err != nil {
		t.Fatalf("NewConversionStore() error = %v", err)
	}
	defer store.Close()

	var count int
	err = store.db.QueryRow(
		`SELECT COUNT(1)
		 FROM sqlite_master
		 WHERE type = 'index' AND name = 'idx_conversion_events_time'`,
	).Scan(&count)
	if err != nil {
		t.Fatalf("failed to query sqlite_master: %v", err)
	}
	if count != 1 {
		t.Fatalf("idx_conversion_events_time missing, count = %d", count)
	}
}

func TestConversionStoreFunnelSummaryAggregation(t *testing.T) {
	tmp := t.TempDir()
	store, err := NewConversionStore(filepath.Join(tmp, "conversion.db"))
	if err != nil {
		t.Fatalf("NewConversionStore() error = %v", err)
	}
	defer store.Close()

	from := time.Now().UTC().Add(-1 * time.Hour).Truncate(time.Second)
	to := from.Add(2 * time.Hour)

	events := []StoredConversionEvent{
		{OrgID: "org-a", EventType: EventPaywallViewed, Surface: "s1", Capability: "c1", IdempotencyKey: "k1", CreatedAt: from.Add(1 * time.Minute)},
		{OrgID: "org-a", EventType: EventPaywallViewed, Surface: "s1", Capability: "c1", IdempotencyKey: "k2", CreatedAt: from.Add(2 * time.Minute)},
		{OrgID: "org-a", EventType: EventTrialStarted, Surface: "s2", Capability: "", IdempotencyKey: "k3", CreatedAt: from.Add(3 * time.Minute)},
		{OrgID: "org-a", EventType: EventUpgradeClicked, Surface: "s3", Capability: "relay", IdempotencyKey: "k4", CreatedAt: from.Add(4 * time.Minute)},
		{OrgID: "org-a", EventType: EventCheckoutCompleted, Surface: "s4", Capability: "", IdempotencyKey: "k5", CreatedAt: from.Add(5 * time.Minute)},
	}
	for _, ev := range events {
		if err := store.Record(ev); err != nil {
			t.Fatalf("Record(%s) error = %v", ev.IdempotencyKey, err)
		}
	}

	summary, err := store.FunnelSummary("org-a", from, to)
	if err != nil {
		t.Fatalf("FunnelSummary() error = %v", err)
	}
	if summary.PaywallViewed != 2 {
		t.Fatalf("PaywallViewed = %d, want 2", summary.PaywallViewed)
	}
	if summary.TrialStarted != 1 {
		t.Fatalf("TrialStarted = %d, want 1", summary.TrialStarted)
	}
	if summary.UpgradeClicked != 1 {
		t.Fatalf("UpgradeClicked = %d, want 1", summary.UpgradeClicked)
	}
	if summary.CheckoutCompleted != 1 {
		t.Fatalf("CheckoutCompleted = %d, want 1", summary.CheckoutCompleted)
	}
	if !summary.Period.From.Equal(from.UTC()) || !summary.Period.To.Equal(to.UTC()) {
		t.Fatalf("Period = %v..%v, want %v..%v", summary.Period.From, summary.Period.To, from.UTC(), to.UTC())
	}
}

func TestConversionStoreOrgIsolation(t *testing.T) {
	tmp := t.TempDir()
	store, err := NewConversionStore(filepath.Join(tmp, "conversion.db"))
	if err != nil {
		t.Fatalf("NewConversionStore() error = %v", err)
	}
	defer store.Close()

	now := time.Now().UTC().Truncate(time.Second)
	if err := store.Record(StoredConversionEvent{
		OrgID:          "org-a",
		EventType:      EventPaywallViewed,
		Surface:        "s",
		Capability:     "c",
		IdempotencyKey: "org-a-k1",
		CreatedAt:      now,
	}); err != nil {
		t.Fatalf("Record() error = %v", err)
	}

	got, err := store.Query("org-b", now.Add(-time.Minute), now.Add(time.Minute), "")
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("len(Query(org-b)) = %d, want 0", len(got))
	}
}

func TestConversionStoreTimeRangeFiltering(t *testing.T) {
	tmp := t.TempDir()
	store, err := NewConversionStore(filepath.Join(tmp, "conversion.db"))
	if err != nil {
		t.Fatalf("NewConversionStore() error = %v", err)
	}
	defer store.Close()

	base := time.Now().UTC().Truncate(time.Second)
	if err := store.Record(StoredConversionEvent{
		OrgID:          "org-a",
		EventType:      EventPaywallViewed,
		Surface:        "s",
		Capability:     "c",
		IdempotencyKey: "old",
		CreatedAt:      base.Add(-2 * time.Hour),
	}); err != nil {
		t.Fatalf("Record(old) error = %v", err)
	}
	if err := store.Record(StoredConversionEvent{
		OrgID:          "org-a",
		EventType:      EventPaywallViewed,
		Surface:        "s",
		Capability:     "c",
		IdempotencyKey: "new",
		CreatedAt:      base.Add(-10 * time.Minute),
	}); err != nil {
		t.Fatalf("Record(new) error = %v", err)
	}

	from := base.Add(-30 * time.Minute)
	to := base.Add(30 * time.Minute)
	got, err := store.Query("org-a", from, to, "")
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(Query()) = %d, want 1", len(got))
	}
	if got[0].IdempotencyKey != "new" {
		t.Fatalf("IdempotencyKey = %q, want new", got[0].IdempotencyKey)
	}
}

func TestConversionStoreConcurrentWrites(t *testing.T) {
	tmp := t.TempDir()
	store, err := NewConversionStore(filepath.Join(tmp, "conversion.db"))
	if err != nil {
		t.Fatalf("NewConversionStore() error = %v", err)
	}
	defer store.Close()

	start := time.Now().UTC().Truncate(time.Second)

	const goroutines = 5
	const perG = 100

	var wg sync.WaitGroup
	errCh := make(chan error, goroutines*perG)
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			for i := 0; i < perG; i++ {
				key := fmt.Sprintf("org-a:%d:%d", id, i)
				if err := store.Record(StoredConversionEvent{
					OrgID:          "org-a",
					EventType:      EventPaywallViewed,
					Surface:        "s",
					Capability:     "c",
					IdempotencyKey: key,
					CreatedAt:      start.Add(time.Duration(i) * time.Second),
				}); err != nil {
					errCh <- err
					return
				}
			}
		}(g)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("concurrent Record() error = %v", err)
		}
	}

	got, err := store.Query("org-a", start.Add(-time.Minute), start.Add(10*time.Minute), EventPaywallViewed)
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	want := goroutines * perG
	if len(got) != want {
		t.Fatalf("len(Query()) = %d, want %d", len(got), want)
	}
}

func TestConversionStoreEnforcesOwnerOnlyPermissions(t *testing.T) {
	tmp := t.TempDir()
	if err := os.Chmod(tmp, 0755); err != nil {
		t.Fatalf("failed to relax temp dir perms: %v", err)
	}

	dbPath := filepath.Join(tmp, "conversion.db")
	store, err := NewConversionStore(dbPath)
	if err != nil {
		t.Fatalf("NewConversionStore() error = %v", err)
	}
	defer store.Close()

	if err := store.Record(StoredConversionEvent{
		OrgID:          "org-a",
		EventType:      EventPaywallViewed,
		Surface:        "s",
		Capability:     "c",
		IdempotencyKey: "perm-check",
		CreatedAt:      time.Now().UTC(),
	}); err != nil {
		t.Fatalf("Record() error = %v", err)
	}

	dirInfo, err := os.Stat(tmp)
	if err != nil {
		t.Fatalf("failed to stat dir: %v", err)
	}
	if got := dirInfo.Mode().Perm(); got != 0700 {
		t.Fatalf("dir perms = %o, want 700", got)
	}

	dbInfo, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("failed to stat db: %v", err)
	}
	if got := dbInfo.Mode().Perm(); got != 0600 {
		t.Fatalf("db perms = %o, want 600", got)
	}

	for _, sidecar := range []string{"-wal", "-shm"} {
		path := dbPath + sidecar
		if info, err := os.Stat(path); err == nil {
			if got := info.Mode().Perm(); got != 0600 {
				t.Fatalf("%s perms = %o, want 600", sidecar, got)
			}
		}
	}
}

func TestConversionStoreRejectsSymlinkDBPath(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "real.db")
	if err := os.WriteFile(target, []byte(""), 0600); err != nil {
		t.Fatalf("failed to create target db file: %v", err)
	}

	linkPath := filepath.Join(tmp, "conversion.db")
	if err := os.Symlink(target, linkPath); err != nil {
		t.Skipf("symlink unsupported on this platform: %v", err)
	}

	_, err := NewConversionStore(linkPath)
	if err == nil {
		t.Fatal("expected NewConversionStore to reject symlink db path")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("expected symlink error, got: %v", err)
	}
}
