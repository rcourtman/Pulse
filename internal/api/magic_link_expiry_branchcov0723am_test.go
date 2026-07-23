package api

import (
	"database/sql"
	"encoding/hex"
	"testing"
	"time"
)

func countMagicLinkRowsBranchcov0723Am(t *testing.T, db *sql.DB) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM magic_link_tokens`).Scan(&n); err != nil {
		t.Fatalf("count magic link rows: %v", err)
	}
	return n
}

func emailForHashBranchcov0723Am(t *testing.T, db *sql.DB, tokenHash []byte) string {
	t.Helper()
	var email string
	key := hex.EncodeToString(tokenHash)
	if err := db.QueryRow(`SELECT email FROM magic_link_tokens WHERE token_hash = ?`, key).Scan(&email); err != nil {
		t.Fatalf("lookup email for %x: %v", tokenHash, err)
	}
	return email
}

func TestBranchcov0723Am_InMemoryMagicLinkStore_DeleteExpired(t *testing.T) {
	base := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
	expiredAt := base.Add(-1 * time.Hour)
	futureAt := base.Add(1 * time.Hour)

	t.Run("empty_store_is_noop", func(t *testing.T) {
		s := NewInMemoryMagicLinkStore()
		t.Cleanup(s.Stop)

		s.DeleteExpired(base)

		if got := len(s.tokens); got != 0 {
			t.Fatalf("empty store token count = %d, want 0", got)
		}
	})

	t.Run("all_valid_keeps_everything", func(t *testing.T) {
		s := NewInMemoryMagicLinkStore()
		t.Cleanup(s.Stop)

		h1 := []byte("hash-all-valid-1_______________")
		h2 := []byte("hash-all-valid-2_______________")
		if err := s.Put(h1, &MagicLinkToken{Email: "a@x.com", OrgID: "org-1", ExpiresAt: futureAt}); err != nil {
			t.Fatalf("Put h1: %v", err)
		}
		if err := s.Put(h2, &MagicLinkToken{Email: "b@x.com", OrgID: "org-1", ExpiresAt: base.Add(2 * time.Hour)}); err != nil {
			t.Fatalf("Put h2: %v", err)
		}

		s.DeleteExpired(base)

		if got := len(s.tokens); got != 2 {
			t.Fatalf("token count = %d, want 2 (all valid)", got)
		}
		if _, ok := s.tokens[hex.EncodeToString(h1)]; !ok {
			t.Fatalf("valid token h1 was wrongly removed")
		}
		if _, ok := s.tokens[hex.EncodeToString(h2)]; !ok {
			t.Fatalf("valid token h2 was wrongly removed")
		}
	})

	t.Run("all_expired_removes_everything", func(t *testing.T) {
		s := NewInMemoryMagicLinkStore()
		t.Cleanup(s.Stop)

		h1 := []byte("hash-all-expired-1_____________")
		h2 := []byte("hash-all-expired-2_____________")
		if err := s.Put(h1, &MagicLinkToken{Email: "a@x.com", OrgID: "org-1", ExpiresAt: expiredAt}); err != nil {
			t.Fatalf("Put h1: %v", err)
		}
		if err := s.Put(h2, &MagicLinkToken{Email: "b@x.com", OrgID: "org-1", ExpiresAt: base.Add(-2 * time.Hour)}); err != nil {
			t.Fatalf("Put h2: %v", err)
		}

		s.DeleteExpired(base)

		if got := len(s.tokens); got != 0 {
			t.Fatalf("token count = %d, want 0 (all expired)", got)
		}
	})

	t.Run("mix_removes_only_expired", func(t *testing.T) {
		s := NewInMemoryMagicLinkStore()
		t.Cleanup(s.Stop)

		hExp := []byte("hash-mix-expired_______________")
		hKeep1 := []byte("hash-mix-valid-1_______________")
		hKeep2 := []byte("hash-mix-valid-2_______________")
		if err := s.Put(hExp, &MagicLinkToken{Email: "exp@x.com", OrgID: "org-1", ExpiresAt: expiredAt}); err != nil {
			t.Fatalf("Put hExp: %v", err)
		}
		if err := s.Put(hKeep1, &MagicLinkToken{Email: "keep1@x.com", OrgID: "org-1", ExpiresAt: futureAt}); err != nil {
			t.Fatalf("Put hKeep1: %v", err)
		}
		if err := s.Put(hKeep2, &MagicLinkToken{Email: "keep2@x.com", OrgID: "org-1", ExpiresAt: base.Add(2 * time.Hour)}); err != nil {
			t.Fatalf("Put hKeep2: %v", err)
		}

		s.DeleteExpired(base)

		if got := len(s.tokens); got != 2 {
			t.Fatalf("token count = %d, want 2", got)
		}
		if _, ok := s.tokens[hex.EncodeToString(hExp)]; ok {
			t.Fatalf("expired token hExp was wrongly kept")
		}
		kept1, ok := s.tokens[hex.EncodeToString(hKeep1)]
		if !ok {
			t.Fatalf("valid token hKeep1 was wrongly removed")
		}
		if kept1.Email != "keep1@x.com" {
			t.Fatalf("hKeep1 email = %q, want keep1@x.com", kept1.Email)
		}
		if _, ok := s.tokens[hex.EncodeToString(hKeep2)]; !ok {
			t.Fatalf("valid token hKeep2 was wrongly removed")
		}
	})

	t.Run("boundary_exactly_at_now_is_kept", func(t *testing.T) {
		s := NewInMemoryMagicLinkStore()
		t.Cleanup(s.Stop)

		hBoundary := []byte("hash-boundary-now______________")
		if err := s.Put(hBoundary, &MagicLinkToken{Email: "edge@x.com", OrgID: "org-1", ExpiresAt: base}); err != nil {
			t.Fatalf("Put hBoundary: %v", err)
		}

		s.DeleteExpired(base)

		if got := len(s.tokens); got != 1 {
			t.Fatalf("token count = %d, want 1 (now.After(expires) is strict, equal is kept)", got)
		}
		tok, ok := s.tokens[hex.EncodeToString(hBoundary)]
		if !ok {
			t.Fatalf("boundary token (expires==now) was removed; operator is strictly After, so it must be kept")
		}
		if !tok.ExpiresAt.Equal(base) {
			t.Fatalf("ExpiresAt = %v, want %v", tok.ExpiresAt, base)
		}
	})

	t.Run("nil_entry_removed", func(t *testing.T) {
		s := NewInMemoryMagicLinkStore()
		t.Cleanup(s.Stop)

		hValid := []byte("hash-nil-valid_________________")
		if err := s.Put(hValid, &MagicLinkToken{Email: "v@x.com", OrgID: "org-1", ExpiresAt: futureAt}); err != nil {
			t.Fatalf("Put hValid: %v", err)
		}
		s.tokens[hex.EncodeToString([]byte("nil-key-entry_________________"))] = nil

		s.DeleteExpired(base)

		if got := len(s.tokens); got != 1 {
			t.Fatalf("token count = %d, want 1 (nil entry purged, valid kept)", got)
		}
		if _, ok := s.tokens[hex.EncodeToString(hValid)]; !ok {
			t.Fatalf("valid token should remain after nil-entry purge")
		}
	})

	t.Run("idempotent_on_repeat", func(t *testing.T) {
		s := NewInMemoryMagicLinkStore()
		t.Cleanup(s.Stop)

		hExp := []byte("hash-idempotent-expired________")
		hKeep := []byte("hash-idempotent-valid__________")
		if err := s.Put(hExp, &MagicLinkToken{Email: "exp@x.com", OrgID: "org-1", ExpiresAt: expiredAt}); err != nil {
			t.Fatalf("Put hExp: %v", err)
		}
		if err := s.Put(hKeep, &MagicLinkToken{Email: "keep@x.com", OrgID: "org-1", ExpiresAt: futureAt}); err != nil {
			t.Fatalf("Put hKeep: %v", err)
		}

		s.DeleteExpired(base)
		if got := len(s.tokens); got != 1 {
			t.Fatalf("after first delete: count = %d, want 1", got)
		}

		s.DeleteExpired(base)
		if got := len(s.tokens); got != 1 {
			t.Fatalf("after second delete: count = %d, want 1 (idempotent)", got)
		}
		if _, ok := s.tokens[hex.EncodeToString(hKeep)]; !ok {
			t.Fatalf("valid token must still be present after second delete")
		}
	})
}

func TestBranchcov0723Am_SQLiteMagicLinkStore_DeleteExpired(t *testing.T) {
	base := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
	expiredAt := base.Add(-1 * time.Hour)
	futureAt := base.Add(1 * time.Hour)

	newStore := func(t *testing.T) *SQLiteMagicLinkStore {
		t.Helper()
		store, err := NewSQLiteMagicLinkStore(t.TempDir())
		if err != nil {
			t.Fatalf("NewSQLiteMagicLinkStore: %v", err)
		}
		t.Cleanup(func() { store.Stop() })
		return store
	}

	t.Run("nil_and_zero_value_receivers_are_noop", func(t *testing.T) {
		var nilPtr *SQLiteMagicLinkStore
		nilPtr.DeleteExpired(base)

		zero := &SQLiteMagicLinkStore{}
		zero.DeleteExpired(base)
	})

	t.Run("empty_store_is_noop", func(t *testing.T) {
		s := newStore(t)

		s.DeleteExpired(base)

		if got := countMagicLinkRowsBranchcov0723Am(t, s.db); got != 0 {
			t.Fatalf("empty store row count = %d, want 0", got)
		}
	})

	t.Run("all_valid_keeps_everything", func(t *testing.T) {
		s := newStore(t)
		h1 := []byte("sql-all-valid-1________________")
		h2 := []byte("sql-all-valid-2________________")
		if err := s.Put(h1, &MagicLinkToken{Email: "a@x.com", OrgID: "org-1", ExpiresAt: futureAt}); err != nil {
			t.Fatalf("Put h1: %v", err)
		}
		if err := s.Put(h2, &MagicLinkToken{Email: "b@x.com", OrgID: "org-1", ExpiresAt: base.Add(2 * time.Hour)}); err != nil {
			t.Fatalf("Put h2: %v", err)
		}

		s.DeleteExpired(base)

		if got := countMagicLinkRowsBranchcov0723Am(t, s.db); got != 2 {
			t.Fatalf("row count = %d, want 2 (all valid)", got)
		}
	})

	t.Run("all_expired_removes_everything", func(t *testing.T) {
		s := newStore(t)
		h1 := []byte("sql-all-expired-1______________")
		h2 := []byte("sql-all-expired-2______________")
		if err := s.Put(h1, &MagicLinkToken{Email: "a@x.com", OrgID: "org-1", ExpiresAt: expiredAt}); err != nil {
			t.Fatalf("Put h1: %v", err)
		}
		if err := s.Put(h2, &MagicLinkToken{Email: "b@x.com", OrgID: "org-1", ExpiresAt: base.Add(-2 * time.Hour)}); err != nil {
			t.Fatalf("Put h2: %v", err)
		}

		s.DeleteExpired(base)

		if got := countMagicLinkRowsBranchcov0723Am(t, s.db); got != 0 {
			t.Fatalf("row count = %d, want 0 (all expired)", got)
		}
	})

	t.Run("mix_removes_only_expired", func(t *testing.T) {
		s := newStore(t)
		hExp := []byte("sql-mix-expired________________")
		hKeep := []byte("sql-mix-valid__________________")
		if err := s.Put(hExp, &MagicLinkToken{Email: "exp@x.com", OrgID: "org-1", ExpiresAt: expiredAt}); err != nil {
			t.Fatalf("Put hExp: %v", err)
		}
		if err := s.Put(hKeep, &MagicLinkToken{Email: "keep@x.com", OrgID: "org-1", ExpiresAt: futureAt}); err != nil {
			t.Fatalf("Put hKeep: %v", err)
		}

		s.DeleteExpired(base)

		if got := countMagicLinkRowsBranchcov0723Am(t, s.db); got != 1 {
			t.Fatalf("row count = %d, want 1", got)
		}
		if got := emailForHashBranchcov0723Am(t, s.db, hKeep); got != "keep@x.com" {
			t.Fatalf("remaining row email = %q, want keep@x.com (valid token must survive)", got)
		}
	})

	t.Run("boundary_exactly_at_now_is_kept", func(t *testing.T) {
		s := newStore(t)
		hBoundary := []byte("sql-boundary-now_______________")
		if err := s.Put(hBoundary, &MagicLinkToken{Email: "edge@x.com", OrgID: "org-1", ExpiresAt: base}); err != nil {
			t.Fatalf("Put hBoundary: %v", err)
		}

		s.DeleteExpired(base)

		if got := countMagicLinkRowsBranchcov0723Am(t, s.db); got != 1 {
			t.Fatalf("row count = %d, want 1 (SQL uses strict expires_at < now, equal is kept)", got)
		}
	})

	t.Run("idempotent_on_repeat", func(t *testing.T) {
		s := newStore(t)
		hExp := []byte("sql-idempotent-expired_________")
		hKeep := []byte("sql-idempotent-valid___________")
		if err := s.Put(hExp, &MagicLinkToken{Email: "exp@x.com", OrgID: "org-1", ExpiresAt: expiredAt}); err != nil {
			t.Fatalf("Put hExp: %v", err)
		}
		if err := s.Put(hKeep, &MagicLinkToken{Email: "keep@x.com", OrgID: "org-1", ExpiresAt: futureAt}); err != nil {
			t.Fatalf("Put hKeep: %v", err)
		}

		s.DeleteExpired(base)
		if got := countMagicLinkRowsBranchcov0723Am(t, s.db); got != 1 {
			t.Fatalf("after first delete: count = %d, want 1", got)
		}
		s.DeleteExpired(base)
		if got := countMagicLinkRowsBranchcov0723Am(t, s.db); got != 1 {
			t.Fatalf("after second delete: count = %d, want 1 (idempotent)", got)
		}
		if got := emailForHashBranchcov0723Am(t, s.db, hKeep); got != "keep@x.com" {
			t.Fatalf("valid token email = %q, want keep@x.com after second delete", got)
		}
	})
}
