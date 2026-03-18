package unifiedresources

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSanitizeOrgID_AllowsSafeChars(t *testing.T) {
	in := "Acme_Org-123"
	if got := sanitizeOrgID(in); got != in {
		t.Fatalf("sanitizeOrgID(%q) = %q, want %q", in, got, in)
	}
}

func TestSanitizeOrgID_StripsUnsafeCharsAndBoundsLength(t *testing.T) {
	in := "../../../../tenant?mode=memory&_pragma=trusted_schema(OFF)#frag"
	got := sanitizeOrgID(in)

	if got == "" {
		t.Fatal("expected non-empty sanitized org ID")
	}
	if len(got) > maxOrgIDLength {
		t.Fatalf("sanitizeOrgID length = %d, want <= %d", len(got), maxOrgIDLength)
	}
	if strings.ContainsAny(got, "/\\?&=#. \t\r\n") {
		t.Fatalf("sanitizeOrgID produced unsafe characters: %q", got)
	}
}

func TestSanitizeOrgID_AllUnsafeInputReturnsEmpty(t *testing.T) {
	if got := sanitizeOrgID("../??//..  "); got != "" {
		t.Fatalf("sanitizeOrgID returned %q, want empty string", got)
	}
}

func TestNewSQLiteResourceStore_DefaultOrgUsesSharedResourcesPath(t *testing.T) {
	dataDir := t.TempDir()
	store, err := NewSQLiteResourceStore(dataDir, "default")
	if err != nil {
		t.Fatalf("NewSQLiteResourceStore returned error: %v", err)
	}
	defer store.Close()

	wantPath := filepath.Join(dataDir, "resources", "unified_resources.db")
	if store.dbPath != wantPath {
		t.Fatalf("db path = %q, want %q", store.dbPath, wantPath)
	}
}

func TestNewSQLiteResourceStore_NonDefaultOrgUsesTenantScopedPath(t *testing.T) {
	dataDir := t.TempDir()
	store, err := NewSQLiteResourceStore(dataDir, "org-a")
	if err != nil {
		t.Fatalf("NewSQLiteResourceStore returned error: %v", err)
	}
	defer store.Close()

	wantPath := filepath.Join(dataDir, "orgs", "org-a", "resources", "unified_resources.db")
	if store.dbPath != wantPath {
		t.Fatalf("db path = %q, want %q", store.dbPath, wantPath)
	}
}

func TestNewSQLiteResourceStore_OrgDotAndUnderscoreDoNotCollide(t *testing.T) {
	dataDir := t.TempDir()
	dotStore, err := NewSQLiteResourceStore(dataDir, "org.a")
	if err != nil {
		t.Fatalf("NewSQLiteResourceStore(org.a) returned error: %v", err)
	}
	defer dotStore.Close()

	underscoreStore, err := NewSQLiteResourceStore(dataDir, "org_a")
	if err != nil {
		t.Fatalf("NewSQLiteResourceStore(org_a) returned error: %v", err)
	}
	defer underscoreStore.Close()

	if dotStore.dbPath == underscoreStore.dbPath {
		t.Fatalf("db path collision: org.a and org_a both mapped to %q", dotStore.dbPath)
	}
}

func TestNewSQLiteResourceStore_RejectsInvalidOrgID(t *testing.T) {
	dataDir := t.TempDir()
	if _, err := NewSQLiteResourceStore(dataDir, "../bad-org"); err == nil {
		t.Fatal("expected invalid org ID error, got nil")
	}
}

func TestNewSQLiteResourceStore_MigratesLegacyStore(t *testing.T) {
	dataDir := t.TempDir()
	orgID := "org.a"
	legacyPath := filepath.Join(dataDir, "resources", legacyResourceStoreFileName(orgID))
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0o700); err != nil {
		t.Fatalf("MkdirAll(%q) failed: %v", filepath.Dir(legacyPath), err)
	}
	seedLegacyLinksTable(t, legacyPath)

	store, err := NewSQLiteResourceStore(dataDir, orgID)
	if err != nil {
		t.Fatalf("NewSQLiteResourceStore returned error: %v", err)
	}
	defer store.Close()

	links, err := store.GetLinks()
	if err != nil {
		t.Fatalf("GetLinks returned error: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("GetLinks length = %d, want 1", len(links))
	}
	if links[0].ResourceA != "legacy-a" || links[0].ResourceB != "legacy-b" {
		t.Fatalf("unexpected migrated link: %+v", links[0])
	}

	if _, err := os.Stat(legacyPath); err != nil {
		t.Fatalf("legacy db should remain for compatibility, stat(%q) failed: %v", legacyPath, err)
	}
	if store.dbPath == legacyPath {
		t.Fatalf("expected migrated store path to differ from legacy path: %q", store.dbPath)
	}
}

func newTestStore(t *testing.T) *SQLiteResourceStore {
	t.Helper()
	dir := t.TempDir()
	store, err := NewSQLiteResourceStore(dir, "testorg")
	if err != nil {
		t.Fatalf("NewSQLiteResourceStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestRecordChange_RoundTrip(t *testing.T) {
	store := newTestStore(t)
	now := time.Now().UTC().Truncate(time.Second)

	change := ResourceChange{
		ID:         "chg-1",
		ResourceID: "vm:100",
		ObservedAt: now,
		Kind:       ChangeStateTransition,
		From:       "offline",
		To:         "online",
		SourceType: "proxmox",
		Confidence: ConfidenceHigh,
	}

	if err := store.RecordChange(change); err != nil {
		t.Fatalf("RecordChange: %v", err)
	}

	results, err := store.GetRecentChanges("vm:100", now.Add(-time.Minute), 10)
	if err != nil {
		t.Fatalf("GetRecentChanges: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 change, got %d", len(results))
	}
	got := results[0]
	if got.ID != change.ID {
		t.Errorf("ID: got %q, want %q", got.ID, change.ID)
	}
	if got.Kind != change.Kind {
		t.Errorf("Kind: got %q, want %q", got.Kind, change.Kind)
	}
	if got.From != change.From || got.To != change.To {
		t.Errorf("From/To: got %q/%q, want %q/%q", got.From, got.To, change.From, change.To)
	}
	if got.Confidence != change.Confidence {
		t.Errorf("Confidence: got %q, want %q", got.Confidence, change.Confidence)
	}
}

func TestGetRecentChanges_RespectsTimeFilter(t *testing.T) {
	store := newTestStore(t)
	base := time.Now().UTC().Truncate(time.Second)

	old := ResourceChange{ID: "chg-old", ResourceID: "vm:1", ObservedAt: base.Add(-2 * time.Hour), Kind: ChangeStateTransition, SourceType: "proxmox", Confidence: ConfidenceHigh}
	recent := ResourceChange{ID: "chg-new", ResourceID: "vm:1", ObservedAt: base, Kind: ChangeStateTransition, SourceType: "proxmox", Confidence: ConfidenceHigh}

	for _, c := range []ResourceChange{old, recent} {
		if err := store.RecordChange(c); err != nil {
			t.Fatalf("RecordChange: %v", err)
		}
	}

	results, err := store.GetRecentChanges("vm:1", base.Add(-time.Hour), 10)
	if err != nil {
		t.Fatalf("GetRecentChanges: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result (recent only), got %d", len(results))
	}
	if results[0].ID != "chg-new" {
		t.Errorf("expected chg-new, got %q", results[0].ID)
	}
}

func TestGetRecentChanges_RespectsLimit(t *testing.T) {
	store := newTestStore(t)
	base := time.Now().UTC().Truncate(time.Second)

	for i := 0; i < 5; i++ {
		c := ResourceChange{
			ID:         strings.Repeat("x", 3) + string(rune('0'+i)),
			ResourceID: "vm:2",
			ObservedAt: base.Add(time.Duration(i) * time.Second),
			Kind:       ChangeStateTransition,
			SourceType: "proxmox",
			Confidence: ConfidenceHigh,
		}
		if err := store.RecordChange(c); err != nil {
			t.Fatalf("RecordChange: %v", err)
		}
	}

	results, err := store.GetRecentChanges("vm:2", base.Add(-time.Minute), 3)
	if err != nil {
		t.Fatalf("GetRecentChanges: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results (limit), got %d", len(results))
	}
}

func seedLegacyLinksTable(t *testing.T, legacyPath string) {
	t.Helper()

	db, err := sql.Open("sqlite", legacyPath)
	if err != nil {
		t.Fatalf("sql.Open(%q) failed: %v", legacyPath, err)
	}
	defer db.Close()

	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS resource_links (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			resource_a TEXT NOT NULL,
			resource_b TEXT NOT NULL,
			primary_id TEXT NOT NULL,
			reason TEXT,
			created_by TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(resource_a, resource_b)
		);
	`); err != nil {
		t.Fatalf("failed to create legacy schema: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO resource_links (resource_a, resource_b, primary_id, reason, created_by, created_at)
		VALUES ('legacy-a', 'legacy-b', 'legacy-a', 'legacy migration', 'tester', CURRENT_TIMESTAMP);
	`); err != nil {
		t.Fatalf("failed to seed legacy link row: %v", err)
	}
}
