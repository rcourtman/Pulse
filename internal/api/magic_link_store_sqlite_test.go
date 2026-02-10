package api

import (
	"errors"
	"testing"
	"time"
)

func TestSQLiteMagicLinkStore_PersistsAcrossReopen(t *testing.T) {
	dir := t.TempDir()

	store1, err := NewSQLiteMagicLinkStore(dir)
	if err != nil {
		t.Fatalf("NewSQLiteMagicLinkStore: %v", err)
	}
	key := []byte("0123456789abcdef0123456789abcdef")
	svc1 := NewMagicLinkServiceWithKey(key, store1, nil, nil)
	base := time.Date(2026, 2, 10, 12, 0, 0, 0, time.UTC)
	svc1.now = func() time.Time { return base }

	token, err := svc1.GenerateToken("user@example.com", "org-123")
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	svc1.Stop()

	store2, err := NewSQLiteMagicLinkStore(dir)
	if err != nil {
		t.Fatalf("NewSQLiteMagicLinkStore reopen: %v", err)
	}
	svc2 := NewMagicLinkServiceWithKey(key, store2, nil, nil)
	svc2.now = func() time.Time { return base.Add(1 * time.Minute) }
	t.Cleanup(svc2.Stop)

	got, err := svc2.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken after reopen: %v", err)
	}
	if got.Email != "user@example.com" || got.OrgID != "org-123" {
		t.Fatalf("unexpected token record: %+v", got)
	}
}

func TestSQLiteMagicLinkStore_WrongKeyCannotValidate(t *testing.T) {
	dir := t.TempDir()

	store, err := NewSQLiteMagicLinkStore(dir)
	if err != nil {
		t.Fatalf("NewSQLiteMagicLinkStore: %v", err)
	}
	rightKey := []byte("0123456789abcdef0123456789abcdef")
	svc := NewMagicLinkServiceWithKey(rightKey, store, nil, nil)
	t.Cleanup(svc.Stop)

	token, err := svc.GenerateToken("user@example.com", "org-123")
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	otherStore, err := NewSQLiteMagicLinkStore(dir)
	if err != nil {
		t.Fatalf("NewSQLiteMagicLinkStore reopen: %v", err)
	}
	wrongKey := []byte("abcdef0123456789abcdef0123456789")
	svcWrong := NewMagicLinkServiceWithKey(wrongKey, otherStore, nil, nil)
	t.Cleanup(svcWrong.Stop)

	_, err = svcWrong.ValidateToken(token)
	if !errors.Is(err, ErrMagicLinkInvalidToken) {
		t.Fatalf("ValidateToken err=%v, want %v", err, ErrMagicLinkInvalidToken)
	}
}
