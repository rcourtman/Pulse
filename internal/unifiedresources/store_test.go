package unifiedresources

import (
	"path/filepath"
	"strings"
	"testing"
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

func TestNewSQLiteResourceStore_UsesSanitizedFilename(t *testing.T) {
	dataDir := t.TempDir()
	store, err := NewSQLiteResourceStore(dataDir, "../../tenant?mode=memory&cache=shared#frag")
	if err != nil {
		t.Fatalf("NewSQLiteResourceStore returned error: %v", err)
	}
	defer store.Close()

	base := filepath.Base(store.dbPath)
	if !strings.HasPrefix(base, "unified_resources_") || !strings.HasSuffix(base, ".db") {
		t.Fatalf("unexpected database filename: %q", base)
	}
	if strings.ContainsAny(base, "/\\?&=# ") {
		t.Fatalf("database filename contains unsafe characters: %q", base)
	}

	rel, err := filepath.Rel(dataDir, store.dbPath)
	if err != nil {
		t.Fatalf("filepath.Rel failed: %v", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		t.Fatalf("db path escaped data dir: dataDir=%q dbPath=%q", dataDir, store.dbPath)
	}
}
