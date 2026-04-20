package conversion

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	_ "modernc.org/sqlite"
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
	dbPath := filepath.Join(tmp, "conversion.db")
	store, err := NewConversionStore(dbPath)
	if err != nil {
		t.Fatalf("NewConversionStore() error = %v", err)
	}
	defer store.Close()

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open sqlite db directly: %v", err)
	}
	defer db.Close()

	var count int
	err = db.QueryRow(
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
		{OrgID: "org-a", EventType: EventPricingViewed, Surface: "s0", Capability: "self_hosted_plan", IdempotencyKey: "k0", CreatedAt: from.Add(30 * time.Second)},
		{OrgID: "org-a", EventType: EventPaywallViewed, Surface: "s1", Capability: "c1", IdempotencyKey: "k1", CreatedAt: from.Add(1 * time.Minute)},
		{OrgID: "org-a", EventType: EventPaywallViewed, Surface: "s1", Capability: "c1", IdempotencyKey: "k2", CreatedAt: from.Add(2 * time.Minute)},
		{OrgID: "org-a", EventType: EventTrialStarted, Surface: "s2", Capability: "", IdempotencyKey: "k3", CreatedAt: from.Add(3 * time.Minute)},
		{OrgID: "org-a", EventType: EventUpgradeClicked, Surface: "s3", Capability: "relay", IdempotencyKey: "k4", CreatedAt: from.Add(4 * time.Minute)},
		{OrgID: "org-a", EventType: EventCheckoutClicked, Surface: "s4", Capability: "self_hosted_plan", IdempotencyKey: "k5", CreatedAt: from.Add(5 * time.Minute)},
		{OrgID: "org-a", EventType: EventCheckoutStarted, Surface: "s5", Capability: "", IdempotencyKey: "k6", CreatedAt: from.Add(6 * time.Minute)},
		{OrgID: "org-a", EventType: EventCheckoutCompleted, Surface: "s6", Capability: "", IdempotencyKey: "k7", CreatedAt: from.Add(7 * time.Minute)},
		{OrgID: "org-a", EventType: EventLicenseActivated, Surface: "s7", Capability: "", IdempotencyKey: "k8", CreatedAt: from.Add(8 * time.Minute)},
		{OrgID: "org-a", EventType: EventLicenseActivationFailed, Surface: "s8", Capability: "", IdempotencyKey: "k9", CreatedAt: from.Add(9 * time.Minute)},
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
	if summary.PricingViewed != 1 {
		t.Fatalf("PricingViewed = %d, want 1", summary.PricingViewed)
	}
	if summary.TrialStarted != 1 {
		t.Fatalf("TrialStarted = %d, want 1", summary.TrialStarted)
	}
	if summary.UpgradeClicked != 1 {
		t.Fatalf("UpgradeClicked = %d, want 1", summary.UpgradeClicked)
	}
	if summary.CheckoutClicked != 1 {
		t.Fatalf("CheckoutClicked = %d, want 1", summary.CheckoutClicked)
	}
	if summary.CheckoutStarted != 1 {
		t.Fatalf("CheckoutStarted = %d, want 1", summary.CheckoutStarted)
	}
	if summary.CheckoutCompleted != 1 {
		t.Fatalf("CheckoutCompleted = %d, want 1", summary.CheckoutCompleted)
	}
	if summary.LicenseActivated != 1 {
		t.Fatalf("LicenseActivated = %d, want 1", summary.LicenseActivated)
	}
	if summary.LicenseActivationFailed != 1 {
		t.Fatalf("LicenseActivationFailed = %d, want 1", summary.LicenseActivationFailed)
	}
	if !summary.Period.From.Equal(from.UTC()) || !summary.Period.To.Equal(to.UTC()) {
		t.Fatalf("Period = %v..%v, want %v..%v", summary.Period.From, summary.Period.To, from.UTC(), to.UTC())
	}
}

func TestConversionStoreFunnelReportIncludesDailySurfaceAndCapabilityBreakdowns(t *testing.T) {
	tmp := t.TempDir()
	store, err := NewConversionStore(filepath.Join(tmp, "conversion.db"))
	if err != nil {
		t.Fatalf("NewConversionStore() error = %v", err)
	}
	defer store.Close()

	from := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	to := from.Add(3 * 24 * time.Hour)

	events := []StoredConversionEvent{
		{
			OrgID:          "org-a",
			EventType:      EventPricingViewed,
			Surface:        "settings_self_hosted_billing_plan",
			Capability:     "self_hosted_plan",
			IdempotencyKey: "funnel:1",
			CreatedAt:      from.Add(2 * time.Hour),
		},
		{
			OrgID:          "org-a",
			EventType:      EventCheckoutClicked,
			Surface:        "settings_self_hosted_billing_compare_prompt",
			Capability:     "self_hosted_plan",
			IdempotencyKey: "funnel:2",
			CreatedAt:      from.Add(3 * time.Hour),
		},
		{
			OrgID:          "org-a",
			EventType:      EventCheckoutStarted,
			Surface:        "license_api",
			Capability:     "self_hosted_plan",
			IdempotencyKey: "funnel:3",
			CreatedAt:      from.Add(4 * time.Hour),
		},
		{
			OrgID:          "org-a",
			EventType:      EventLicenseActivated,
			Surface:        "license_api",
			Capability:     "self_hosted_plan",
			IdempotencyKey: "funnel:4",
			CreatedAt:      from.Add(27 * time.Hour),
		},
		{
			OrgID:          "org-a",
			EventType:      EventTrialStarted,
			Surface:        "license_panel",
			Capability:     "relay",
			IdempotencyKey: "funnel:5",
			CreatedAt:      from.Add(28 * time.Hour),
		},
		{
			OrgID:          "org-a",
			EventType:      EventPricingViewed,
			Surface:        "paywall_modal",
			Capability:     "relay",
			IdempotencyKey: "funnel:6",
			CreatedAt:      from.Add(50 * time.Hour),
		},
	}
	for _, ev := range events {
		if err := store.Record(ev); err != nil {
			t.Fatalf("Record(%s) error = %v", ev.IdempotencyKey, err)
		}
	}

	report, err := store.FunnelReport("org-a", from, to)
	if err != nil {
		t.Fatalf("FunnelReport() error = %v", err)
	}

	if report.Summary.PricingViewed != 2 {
		t.Fatalf("Summary.PricingViewed = %d, want 2", report.Summary.PricingViewed)
	}
	if report.Summary.CheckoutClicked != 1 {
		t.Fatalf("Summary.CheckoutClicked = %d, want 1", report.Summary.CheckoutClicked)
	}
	if report.Summary.LicenseActivated != 1 {
		t.Fatalf("Summary.LicenseActivated = %d, want 1", report.Summary.LicenseActivated)
	}
	if len(report.Daily) != 3 {
		t.Fatalf("len(Daily) = %d, want 3", len(report.Daily))
	}
	if report.Daily[0].Day != "2026-04-01" || report.Daily[0].PricingViewed != 1 || report.Daily[0].CheckoutClicked != 1 {
		t.Fatalf("unexpected first daily bucket: %+v", report.Daily[0])
	}
	if report.Daily[1].Day != "2026-04-02" || report.Daily[1].LicenseActivated != 1 || report.Daily[1].TrialStarted != 1 {
		t.Fatalf("unexpected second daily bucket: %+v", report.Daily[1])
	}
	if report.Daily[2].Day != "2026-04-03" || report.Daily[2].PricingViewed != 1 {
		t.Fatalf("unexpected third daily bucket: %+v", report.Daily[2])
	}

	if len(report.Surfaces) < 3 {
		t.Fatalf("len(Surfaces) = %d, want >= 3", len(report.Surfaces))
	}
	if report.Surfaces[0].Key != "settings_self_hosted_billing_compare_prompt" {
		t.Fatalf("Surfaces[0].Key = %q, want settings_self_hosted_billing_compare_prompt", report.Surfaces[0].Key)
	}
	if report.Surfaces[0].CheckoutClicked != 1 {
		t.Fatalf("Surfaces[0].CheckoutClicked = %d, want 1", report.Surfaces[0].CheckoutClicked)
	}

	if len(report.Capabilities) != 2 {
		t.Fatalf("len(Capabilities) = %d, want 2", len(report.Capabilities))
	}
	if report.Capabilities[0].Key != "self_hosted_plan" {
		t.Fatalf("Capabilities[0].Key = %q, want self_hosted_plan", report.Capabilities[0].Key)
	}
	if report.Capabilities[0].PricingViewed != 1 || report.Capabilities[0].LicenseActivated != 1 {
		t.Fatalf("unexpected self_hosted_plan capability bucket: %+v", report.Capabilities[0])
	}
	if report.Capabilities[1].Key != "relay" {
		t.Fatalf("Capabilities[1].Key = %q, want relay", report.Capabilities[1].Key)
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

func TestConversionStoreRequiresOrgScopeForReads(t *testing.T) {
	tmp := t.TempDir()
	store, err := NewConversionStore(filepath.Join(tmp, "conversion.db"))
	if err != nil {
		t.Fatalf("NewConversionStore() error = %v", err)
	}
	defer store.Close()

	now := time.Now().UTC().Truncate(time.Second)
	if _, err := store.Query("", now.Add(-time.Minute), now.Add(time.Minute), ""); err == nil {
		t.Fatal("Query() with empty org_id error = nil, want error")
	}

	if _, err := store.FunnelSummary("", now.Add(-time.Minute), now.Add(time.Minute)); err == nil {
		t.Fatal("FunnelSummary() with empty org_id error = nil, want error")
	}
	if _, err := store.FunnelDailyBreakdown("", now.Add(-time.Minute), now.Add(time.Minute)); err == nil {
		t.Fatal("FunnelDailyBreakdown() with empty org_id error = nil, want error")
	}
	if _, err := store.FunnelDimensionBreakdown("", now.Add(-time.Minute), now.Add(time.Minute), "surface"); err == nil {
		t.Fatal("FunnelDimensionBreakdown() with empty org_id error = nil, want error")
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
