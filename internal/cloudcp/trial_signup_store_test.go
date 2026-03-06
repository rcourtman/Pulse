package cloudcp

import (
	"errors"
	"testing"
	"time"
)

func TestTrialSignupStoreMarkTrialIssuedEnforcesEmailUniqueness(t *testing.T) {
	store, err := NewTrialSignupStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewTrialSignupStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	now := time.Unix(1710000000, 0).UTC()
	firstToken, err := store.CreateVerification(&TrialSignupRecord{
		OrgID:                 "default",
		ReturnURL:             "https://pulse.example.com/auth/trial-activate",
		Name:                  "Test User",
		Email:                 "owner@example.com",
		Company:               "Pulse Labs",
		CreatedAt:             now,
		VerificationExpiresAt: now.Add(trialSignupVerificationTTL),
	})
	if err != nil {
		t.Fatalf("CreateVerification(first): %v", err)
	}
	firstRecord, err := store.ConsumeVerification(firstToken, now)
	if err != nil {
		t.Fatalf("ConsumeVerification(first): %v", err)
	}

	secondToken, err := store.CreateVerification(&TrialSignupRecord{
		OrgID:                 "default",
		ReturnURL:             "https://pulse.example.com/auth/trial-activate",
		Name:                  "Test User",
		Email:                 "owner@example.com",
		Company:               "Pulse Labs",
		CreatedAt:             now,
		VerificationExpiresAt: now.Add(trialSignupVerificationTTL),
	})
	if err != nil {
		t.Fatalf("CreateVerification(second): %v", err)
	}
	secondRecord, err := store.ConsumeVerification(secondToken, now)
	if err != nil {
		t.Fatalf("ConsumeVerification(second): %v", err)
	}

	if err := store.MarkTrialIssued(firstRecord.ID, now); err != nil {
		t.Fatalf("MarkTrialIssued(first): %v", err)
	}
	if err := store.MarkTrialIssued(firstRecord.ID, now); err != nil {
		t.Fatalf("MarkTrialIssued(first repeat): %v", err)
	}

	used, err := store.HasIssuedTrialForEmail("owner@example.com")
	if err != nil {
		t.Fatalf("HasIssuedTrialForEmail: %v", err)
	}
	if !used {
		t.Fatalf("expected issued email lookup to return true")
	}

	err = store.MarkTrialIssued(secondRecord.ID, now)
	if !errors.Is(err, ErrTrialSignupEmailAlreadyUsed) {
		t.Fatalf("MarkTrialIssued(second) error=%v, want %v", err, ErrTrialSignupEmailAlreadyUsed)
	}
}
