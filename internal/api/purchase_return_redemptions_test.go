package api

import (
	"testing"
	"time"
)

func TestPurchaseReturnRedemptionStore_RetriesFailedRedemptionAndBlocksActivatedReplay(t *testing.T) {
	store := &purchaseReturnRedemptionStore{configDir: t.TempDir()}
	attempt := purchaseReturnRedemptionAttempt{
		PortalHandoffID:     "cph_test_upgrade",
		PurchaseReturnJTI:   "prt_test_jti",
		CheckoutSessionID:   "cs_test_upgrade",
		LicenseID:           "lic_test_upgrade",
		ActivationKeyPrefix: "ppk_live",
		ExpiresAt:           time.Now().UTC().Add(2 * time.Hour),
	}

	decision, record, err := store.begin(attempt)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if decision != purchaseReturnRedemptionDecisionStarted {
		t.Fatalf("decision = %q, want %q", decision, purchaseReturnRedemptionDecisionStarted)
	}
	if record == nil || record.Status != purchaseReturnRedemptionStateStarted {
		t.Fatalf("record status = %#v, want started", record)
	}

	if err := store.markFailed(attempt.PortalHandoffID, attempt.PurchaseReturnJTI, "license_activation_failed", "boom"); err != nil {
		t.Fatalf("markFailed: %v", err)
	}
	record, err = store.get(attempt.PortalHandoffID, attempt.PurchaseReturnJTI)
	if err != nil {
		t.Fatalf("get after failure: %v", err)
	}
	if record == nil || record.Status != purchaseReturnRedemptionStateFailed {
		t.Fatalf("record status after failure = %#v, want failed", record)
	}
	if record.FailureReason != "license_activation_failed" {
		t.Fatalf("failure_reason = %q, want license_activation_failed", record.FailureReason)
	}

	decision, record, err = store.begin(attempt)
	if err != nil {
		t.Fatalf("begin retry: %v", err)
	}
	if decision != purchaseReturnRedemptionDecisionStarted {
		t.Fatalf("retry decision = %q, want %q", decision, purchaseReturnRedemptionDecisionStarted)
	}
	if record == nil || record.Status != purchaseReturnRedemptionStateStarted {
		t.Fatalf("retry record status = %#v, want started", record)
	}
	if record.FailureReason != "" || record.FailureMessage != "" {
		t.Fatalf("retry record failure fields = %#v, want cleared", record)
	}

	if err := store.markActivated(attempt.PortalHandoffID, attempt.PurchaseReturnJTI, attempt.LicenseID, attempt.ActivationKeyPrefix); err != nil {
		t.Fatalf("markActivated: %v", err)
	}
	record, err = store.get(attempt.PortalHandoffID, attempt.PurchaseReturnJTI)
	if err != nil {
		t.Fatalf("get after activation: %v", err)
	}
	if record == nil || record.Status != purchaseReturnRedemptionStateActivated {
		t.Fatalf("record status after activation = %#v, want activated", record)
	}
	if record.RedeemedAt == nil {
		t.Fatal("redeemed_at was not recorded")
	}

	decision, _, err = store.begin(attempt)
	if err != nil {
		t.Fatalf("begin after activation: %v", err)
	}
	if decision != purchaseReturnRedemptionDecisionAlreadyActivated {
		t.Fatalf("decision after activation = %q, want %q", decision, purchaseReturnRedemptionDecisionAlreadyActivated)
	}
}

func TestPurchaseReturnRedemptionStore_RejectsConflictingBindings(t *testing.T) {
	store := &purchaseReturnRedemptionStore{configDir: t.TempDir()}
	attempt := purchaseReturnRedemptionAttempt{
		PortalHandoffID:     "cph_test_upgrade",
		PurchaseReturnJTI:   "prt_test_jti",
		CheckoutSessionID:   "cs_test_upgrade",
		LicenseID:           "lic_test_upgrade",
		ActivationKeyPrefix: "ppk_live",
		ExpiresAt:           time.Now().UTC().Add(2 * time.Hour),
	}

	if decision, _, err := store.begin(attempt); err != nil {
		t.Fatalf("begin: %v", err)
	} else if decision != purchaseReturnRedemptionDecisionStarted {
		t.Fatalf("decision = %q, want %q", decision, purchaseReturnRedemptionDecisionStarted)
	}

	conflictingSession := attempt
	conflictingSession.CheckoutSessionID = "cs_conflicting_upgrade"
	if decision, _, err := store.begin(conflictingSession); err != nil {
		t.Fatalf("begin conflicting session: %v", err)
	} else if decision != purchaseReturnRedemptionDecisionConflict {
		t.Fatalf("decision = %q, want %q", decision, purchaseReturnRedemptionDecisionConflict)
	}

	conflictingHandoff := attempt
	conflictingHandoff.PortalHandoffID = "cph_other_upgrade"
	conflictingHandoff.CheckoutSessionID = attempt.CheckoutSessionID
	conflictingHandoff.PurchaseReturnJTI = "prt_other_jti"
	if decision, _, err := store.begin(conflictingHandoff); err != nil {
		t.Fatalf("begin conflicting handoff: %v", err)
	} else if decision != purchaseReturnRedemptionDecisionConflict {
		t.Fatalf("decision = %q, want %q", decision, purchaseReturnRedemptionDecisionConflict)
	}
}
