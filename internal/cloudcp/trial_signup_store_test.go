package cloudcp

import (
	"errors"
	"strings"
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

func TestTrialSignupStoreMarkTrialIssuedEnforcesBusinessDomainUniqueness(t *testing.T) {
	store, err := NewTrialSignupStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewTrialSignupStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	now := time.Unix(1710000000, 0).UTC()
	firstToken, err := store.CreateVerification(&TrialSignupRecord{
		OrgID:                 "default",
		ReturnURL:             "https://pulse.example.com/auth/trial-activate",
		Name:                  "Alice",
		Email:                 "alice@acme.com",
		Company:               "Acme Inc.",
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
	if err := store.MarkTrialIssued(firstRecord.ID, now); err != nil {
		t.Fatalf("MarkTrialIssued(first): %v", err)
	}

	conflict, err := store.FindIssuedTrialConflict("bob@acme.com", "Acme Holdings")
	if err != nil {
		t.Fatalf("FindIssuedTrialConflict: %v", err)
	}
	if conflict == nil || conflict.Kind != trialSignupConflictDomain {
		t.Fatalf("conflict=%#v, want business-domain conflict", conflict)
	}

	secondToken, err := store.CreateVerification(&TrialSignupRecord{
		OrgID:                 "default",
		ReturnURL:             "https://pulse.example.com/auth/trial-activate",
		Name:                  "Bob",
		Email:                 "bob@acme.com",
		Company:               "Acme Holdings",
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

	err = store.MarkTrialIssued(secondRecord.ID, now)
	if !errors.Is(err, ErrTrialSignupOrganizationUsed) {
		t.Fatalf("MarkTrialIssued(second) error=%v, want %v", err, ErrTrialSignupOrganizationUsed)
	}
}

func TestTrialSignupStoreFindIssuedTrialConflictUsesCompanyForPublicEmailDomains(t *testing.T) {
	store, err := NewTrialSignupStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewTrialSignupStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	now := time.Unix(1710000000, 0).UTC()
	firstToken, err := store.CreateVerification(&TrialSignupRecord{
		OrgID:                 "default",
		ReturnURL:             "https://pulse.example.com/auth/trial-activate",
		Name:                  "Alice",
		Email:                 "alice@gmail.com",
		Company:               "Pulse Labs Ltd.",
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
	if err := store.MarkTrialIssued(firstRecord.ID, now); err != nil {
		t.Fatalf("MarkTrialIssued(first): %v", err)
	}

	conflict, err := store.FindIssuedTrialConflict("teammate@yahoo.com", "Pulse Labs")
	if err != nil {
		t.Fatalf("FindIssuedTrialConflict: %v", err)
	}
	if conflict == nil || conflict.Kind != trialSignupConflictPublicCompany {
		t.Fatalf("conflict=%#v, want public-company conflict", conflict)
	}

	noConflict, err := store.FindIssuedTrialConflict("other@yahoo.com", "Different Company")
	if err != nil {
		t.Fatalf("FindIssuedTrialConflict(no conflict): %v", err)
	}
	if noConflict != nil {
		t.Fatalf("noConflict=%#v, want nil", noConflict)
	}
}

func TestTrialSignupStoreFindPendingVerificationByEmail(t *testing.T) {
	store, err := NewTrialSignupStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewTrialSignupStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	now := time.Unix(1710000000, 0).UTC()
	token, err := store.CreateVerification(&TrialSignupRecord{
		OrgID:                 "default",
		ReturnURL:             "https://pulse.example.com/auth/trial-activate",
		Name:                  "Owner",
		Email:                 "owner@example.com",
		Company:               "Pulse Labs",
		CreatedAt:             now,
		VerificationExpiresAt: now.Add(trialSignupVerificationTTL),
	})
	if err != nil {
		t.Fatalf("CreateVerification: %v", err)
	}
	if strings.TrimSpace(token) == "" {
		t.Fatalf("expected raw token")
	}

	pending, err := store.FindPendingVerificationByEmail("owner@example.com", now)
	if err != nil {
		t.Fatalf("FindPendingVerificationByEmail: %v", err)
	}
	if pending == nil {
		t.Fatal("expected pending verification record")
	}

	if _, err := store.ConsumeVerification(token, now); err != nil {
		t.Fatalf("ConsumeVerification: %v", err)
	}
	pending, err = store.FindPendingVerificationByEmail("owner@example.com", now)
	if err != nil {
		t.Fatalf("FindPendingVerificationByEmail after consume: %v", err)
	}
	if pending != nil {
		t.Fatalf("pending=%#v, want nil after consume", pending)
	}
}
