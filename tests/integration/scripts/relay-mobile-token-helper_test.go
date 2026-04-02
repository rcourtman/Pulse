package main

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestFindTokenRecordResolvesRawToken(t *testing.T) {
	rawToken, err := config.NewAPITokenRecord("relay-mobile-raw-token-123.12345678", "Pulse Mobile", []string{config.ScopeRelayMobileAccess})
	if err != nil {
		t.Fatalf("NewAPITokenRecord: %v", err)
	}
	otherToken, err := config.NewAPITokenRecord("other-raw-token-123.12345678", "Other", []string{config.ScopeRelayMobileAccess})
	if err != nil {
		t.Fatalf("NewAPITokenRecord other: %v", err)
	}

	record, err := findTokenRecord([]config.APITokenRecord{*rawToken, *otherToken}, "", "relay-mobile-raw-token-123.12345678")
	if err != nil {
		t.Fatalf("findTokenRecord: %v", err)
	}
	if record == nil {
		t.Fatal("expected record, got nil")
	}
	if record.ID != rawToken.ID {
		t.Fatalf("record ID = %q, want %q", record.ID, rawToken.ID)
	}
}

func TestDeleteTokenRecordRemovesOnlyMatchingToken(t *testing.T) {
	first, err := config.NewAPITokenRecord("relay-mobile-delete-1.12345678", "First", []string{config.ScopeRelayMobileAccess})
	if err != nil {
		t.Fatalf("NewAPITokenRecord first: %v", err)
	}
	second, err := config.NewAPITokenRecord("relay-mobile-delete-2.12345678", "Second", []string{config.ScopeRelayMobileAccess})
	if err != nil {
		t.Fatalf("NewAPITokenRecord second: %v", err)
	}

	filtered, removed := deleteTokenRecord([]config.APITokenRecord{*first, *second}, second.ID)
	if removed == nil {
		t.Fatal("expected removed token, got nil")
	}
	if removed.ID != second.ID {
		t.Fatalf("removed ID = %q, want %q", removed.ID, second.ID)
	}
	if len(filtered) != 1 {
		t.Fatalf("filtered len = %d, want 1", len(filtered))
	}
	if filtered[0].ID != first.ID {
		t.Fatalf("remaining ID = %q, want %q", filtered[0].ID, first.ID)
	}
}
