package api

import (
	"testing"
	"time"
)

func TestPurchaseCheckoutHandoffStore_IssueAndResolve(t *testing.T) {
	store := &purchaseCheckoutHandoffStore{configDir: t.TempDir()}
	expiresAt := time.Now().UTC().Add(time.Hour)

	handoffID, err := store.issue(
		"relay",
		"https://pulse.example.com/auth/license-purchase-activate?purchase_return_token=prt_signed&session_id={CHECKOUT_SESSION_ID}",
		expiresAt,
	)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if handoffID == "" {
		t.Fatal("expected opaque handoff id")
	}

	record, found, err := store.resolve(handoffID, time.Now().UTC())
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !found || record == nil {
		t.Fatal("expected stored handoff record")
	}
	if record.Feature != "relay" {
		t.Fatalf("record.Feature = %q, want relay", record.Feature)
	}
}

func TestPurchaseCheckoutHandoffStore_RejectsExpiredRecords(t *testing.T) {
	store := &purchaseCheckoutHandoffStore{configDir: t.TempDir()}
	expiresAt := time.Now().UTC().Add(5 * time.Minute)

	handoffID, err := store.issue(
		"relay",
		"https://pulse.example.com/auth/license-purchase-activate?purchase_return_token=prt_signed&session_id={CHECKOUT_SESSION_ID}",
		expiresAt,
	)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	record, found, err := store.resolve(handoffID, expiresAt.Add(time.Second))
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if found || record != nil {
		t.Fatal("expected expired handoff record to be unavailable")
	}
}
