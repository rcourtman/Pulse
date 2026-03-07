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
		InstanceToken:         "tsi_test",
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
		InstanceToken:         "tsi_test",
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
		InstanceToken:         "tsi_test",
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
		InstanceToken:         "tsi_test",
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
		InstanceToken:         "tsi_test",
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
		InstanceToken:         "tsi_test",
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

func TestTrialSignupStoreStoreOrRotateActivationToken(t *testing.T) {
	store, err := NewTrialSignupStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewTrialSignupStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	now := time.Unix(1710000000, 0).UTC()
	rawToken, err := store.CreateVerification(&TrialSignupRecord{
		OrgID:                 "default",
		ReturnURL:             "https://pulse.example.com/auth/trial-activate",
		InstanceToken:         "tsi_test",
		Name:                  "Owner",
		Email:                 "owner@example.com",
		Company:               "Pulse Labs",
		CreatedAt:             now,
		VerificationExpiresAt: now.Add(trialSignupVerificationTTL),
	})
	if err != nil {
		t.Fatalf("CreateVerification: %v", err)
	}
	record, err := store.ConsumeVerification(rawToken, now)
	if err != nil {
		t.Fatalf("ConsumeVerification: %v", err)
	}

	storedToken, firstIssue, err := store.StoreOrRotateActivationToken(record.ID, "token-one", now, trialSignupActivationTokenTTL)
	if err != nil {
		t.Fatalf("StoreOrRotateActivationToken(first): %v", err)
	}
	if !firstIssue {
		t.Fatal("expected first activation token issuance to report firstIssue=true")
	}
	if storedToken != "token-one" {
		t.Fatalf("storedToken=%q, want %q", storedToken, "token-one")
	}

	storedToken, firstIssue, err = store.StoreOrRotateActivationToken(record.ID, "token-two", now.Add(time.Minute), trialSignupActivationTokenTTL)
	if err != nil {
		t.Fatalf("StoreOrRotateActivationToken(second): %v", err)
	}
	if firstIssue {
		t.Fatal("expected repeat activation token load to report firstIssue=false")
	}
	if storedToken != "token-one" {
		t.Fatalf("storedToken=%q, want existing token %q", storedToken, "token-one")
	}

	storedToken, firstIssue, err = store.StoreOrRotateActivationToken(record.ID, "token-three", now.Add(trialSignupActivationTokenTTL+time.Minute), trialSignupActivationTokenTTL)
	if err != nil {
		t.Fatalf("StoreOrRotateActivationToken(third): %v", err)
	}
	if !firstIssue {
		t.Fatal("expected expired activation token to be rotated")
	}
	if storedToken != "token-three" {
		t.Fatalf("storedToken=%q, want rotated token %q", storedToken, "token-three")
	}
}

func TestTrialSignupStoreMarkRedemptionRecorded(t *testing.T) {
	store, err := NewTrialSignupStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewTrialSignupStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	now := time.Unix(1710000000, 0).UTC()
	rawToken, err := store.CreateVerification(&TrialSignupRecord{
		OrgID:                 "default",
		ReturnURL:             "https://pulse.example.com/auth/trial-activate",
		InstanceToken:         "tsi_test",
		Name:                  "Owner",
		Email:                 "owner@example.com",
		Company:               "Pulse Labs",
		CreatedAt:             now,
		VerificationExpiresAt: now.Add(trialSignupVerificationTTL),
	})
	if err != nil {
		t.Fatalf("CreateVerification: %v", err)
	}
	record, err := store.ConsumeVerification(rawToken, now)
	if err != nil {
		t.Fatalf("ConsumeVerification: %v", err)
	}
	if err := store.MarkCheckoutCompleted(record.ID, "cs_test_redeem", now); err != nil {
		t.Fatalf("MarkCheckoutCompleted: %v", err)
	}
	if _, _, err := store.StoreOrRotateActivationToken(record.ID, "activation-token", now, trialSignupActivationTokenTTL); err != nil {
		t.Fatalf("StoreOrRotateActivationToken: %v", err)
	}

	if err := store.MarkRedemptionRecorded("default", "https://pulse.example.com/auth/trial-activate", "tsi_test", "pulse.example.com", "activation-token", now); err != nil {
		t.Fatalf("MarkRedemptionRecorded(first): %v", err)
	}
	if err := store.MarkRedemptionRecorded("default", "https://pulse.example.com/auth/trial-activate", "tsi_test", "pulse.example.com", "activation-token", now.Add(time.Minute)); err != nil {
		t.Fatalf("MarkRedemptionRecorded(repeat): %v", err)
	}

	updated, err := store.GetRecord(record.ID)
	if err != nil {
		t.Fatalf("GetRecord(updated): %v", err)
	}
	if updated.RedemptionRecordedAt.IsZero() {
		t.Fatalf("expected redemption_recorded_at to be populated")
	}
	if !updated.RedemptionRecordedAt.Equal(now) {
		t.Fatalf("redemption_recorded_at=%v, want %v", updated.RedemptionRecordedAt, now)
	}
}

func TestTrialSignupStoreMarkRedemptionRecordedRejectsMismatchedReturnTarget(t *testing.T) {
	store, err := NewTrialSignupStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewTrialSignupStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	now := time.Unix(1710000000, 0).UTC()
	rawToken, err := store.CreateVerification(&TrialSignupRecord{
		OrgID:                 "default",
		ReturnURL:             "https://pulse.example.com/auth/trial-activate",
		InstanceToken:         "tsi_test",
		Name:                  "Owner",
		Email:                 "owner@example.com",
		Company:               "Pulse Labs",
		CreatedAt:             now,
		VerificationExpiresAt: now.Add(trialSignupVerificationTTL),
	})
	if err != nil {
		t.Fatalf("CreateVerification: %v", err)
	}
	record, err := store.ConsumeVerification(rawToken, now)
	if err != nil {
		t.Fatalf("ConsumeVerification: %v", err)
	}
	if err := store.MarkCheckoutCompleted(record.ID, "cs_test_redeem", now); err != nil {
		t.Fatalf("MarkCheckoutCompleted: %v", err)
	}
	if _, _, err := store.StoreOrRotateActivationToken(record.ID, "activation-token", now, trialSignupActivationTokenTTL); err != nil {
		t.Fatalf("StoreOrRotateActivationToken: %v", err)
	}

	err = store.MarkRedemptionRecorded("default", "https://other.example.com/auth/trial-activate", "tsi_test", "other.example.com", "activation-token", now)
	if !errors.Is(err, ErrTrialSignupRecordNotFound) {
		t.Fatalf("MarkRedemptionRecorded error=%v, want %v", err, ErrTrialSignupRecordNotFound)
	}
}

func TestTrialSignupStoreIssueCheckoutTokenAndLookup(t *testing.T) {
	store, err := NewTrialSignupStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewTrialSignupStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	now := time.Unix(1710000000, 0).UTC()
	rawToken, err := store.CreateVerification(&TrialSignupRecord{
		OrgID:                 "default",
		ReturnURL:             "https://pulse.example.com/auth/trial-activate",
		InstanceToken:         "tsi_test",
		Name:                  "Owner",
		Email:                 "owner@example.com",
		Company:               "Pulse Labs",
		CreatedAt:             now,
		VerificationExpiresAt: now.Add(trialSignupVerificationTTL),
	})
	if err != nil {
		t.Fatalf("CreateVerification: %v", err)
	}
	record, err := store.ConsumeVerification(rawToken, now)
	if err != nil {
		t.Fatalf("ConsumeVerification: %v", err)
	}

	checkoutToken, err := store.IssueCheckoutToken(record.ID, now, trialSignupVerificationTTL)
	if err != nil {
		t.Fatalf("IssueCheckoutToken: %v", err)
	}
	if checkoutToken == "" {
		t.Fatal("expected checkout token")
	}

	loaded, err := store.GetRecordByCheckoutToken(checkoutToken, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("GetRecordByCheckoutToken: %v", err)
	}
	if loaded.ID != record.ID {
		t.Fatalf("loaded.ID=%q, want %q", loaded.ID, record.ID)
	}
	if loaded.CheckoutTokenHash == "" {
		t.Fatal("expected checkout token hash to be stored")
	}
	if loaded.CheckoutTokenExpiresAt.IsZero() {
		t.Fatal("expected checkout token expiry to be stored")
	}
}
