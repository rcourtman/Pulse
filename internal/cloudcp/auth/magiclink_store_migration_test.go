package auth

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestNewStoreMigratesLegacySchemaWithNoTargetColumn(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "cp_magic_links.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(`
		CREATE TABLE magic_link_tokens (
			token_hash TEXT PRIMARY KEY,
			email TEXT NOT NULL,
			tenant_id TEXT NOT NULL,
			expires_at INTEGER NOT NULL,
			used INTEGER NOT NULL DEFAULT 0,
			created_at INTEGER NOT NULL,
			used_at INTEGER
		);
	`); err != nil {
		t.Fatalf("create legacy schema: %v", err)
	}

	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(store.Close)

	tokenHash := signHMAC([]byte("legacy-key"), "portal")
	if err := store.Put(tokenHash, &TokenRecord{
		Email:     "buyer@example.com",
		TenantID:  "",
		Target:    string(MagicLinkTargetPortal),
		ExpiresAt: time.Now().UTC().Add(10 * time.Minute),
	}); err != nil {
		t.Fatalf("Put: %v", err)
	}

	rec, err := store.Consume(tokenHash, time.Now().UTC())
	if err != nil {
		t.Fatalf("Consume: %v", err)
	}
	if rec.Target != string(MagicLinkTargetPortal) {
		t.Fatalf("target=%q, want %q", rec.Target, MagicLinkTargetPortal)
	}
	if rec.TenantID != "" {
		t.Fatalf("tenantID=%q, want empty", rec.TenantID)
	}
}
