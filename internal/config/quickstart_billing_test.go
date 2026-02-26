package config

import (
	"crypto/sha256"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

func setupTestBillingDir(t *testing.T) (string, []byte) {
	t.Helper()
	dir := t.TempDir()

	// Create a 32-byte encryption key
	rawKey := make([]byte, 32)
	for i := range rawKey {
		rawKey[i] = byte(i)
	}
	encoded := base64.StdEncoding.EncodeToString(rawKey)
	if err := os.WriteFile(filepath.Join(dir, ".encryption.key"), []byte(encoded), 0o600); err != nil {
		t.Fatal(err)
	}

	// Derive HMAC key (same as loadHMACKey)
	h := sha256.New()
	h.Write([]byte("pulse-billing-integrity-"))
	h.Write(rawKey)
	return dir, h.Sum(nil)
}

func TestQuickstartFieldsInHMAC(t *testing.T) {
	_, hmacKey := setupTestBillingDir(t)

	// Two states: identical except quickstart credits
	state1 := &pkglicensing.BillingState{
		Capabilities:      []string{},
		Limits:            map[string]int64{},
		PlanVersion:       "trial",
		SubscriptionState: pkglicensing.SubStateTrial,
	}
	state2 := &pkglicensing.BillingState{
		Capabilities:             []string{},
		Limits:                   map[string]int64{},
		PlanVersion:              "trial",
		SubscriptionState:        pkglicensing.SubStateTrial,
		QuickstartCreditsGranted: true,
		QuickstartCreditsUsed:    5,
	}

	sig1 := billingIntegrity(state1, hmacKey)
	sig2 := billingIntegrity(state2, hmacKey)

	if sig1 == sig2 {
		t.Fatal("HMAC should differ when quickstart fields change")
	}
}

func TestQuickstartFieldsPreservedByHMAC(t *testing.T) {
	_, hmacKey := setupTestBillingDir(t)

	now := int64(1709000000)
	state := &pkglicensing.BillingState{
		Capabilities:               []string{},
		Limits:                     map[string]int64{},
		PlanVersion:                "trial",
		SubscriptionState:          pkglicensing.SubStateTrial,
		QuickstartCreditsGranted:   true,
		QuickstartCreditsUsed:      10,
		QuickstartCreditsGrantedAt: &now,
	}

	sig := billingIntegrity(state, hmacKey)
	state.Integrity = sig

	// Verify succeeds
	if !verifyBillingIntegrity(state, hmacKey) {
		t.Fatal("HMAC should verify for state with quickstart fields")
	}

	// Tamper with credits used
	state.QuickstartCreditsUsed = 0
	if verifyBillingIntegrity(state, hmacKey) {
		t.Fatal("HMAC should fail when credits_used is tampered")
	}
}

func TestV6PreQuickstartMigration(t *testing.T) {
	_, hmacKey := setupTestBillingDir(t)

	// Simulate a state signed with v6-pre-quickstart format (no quickstart fields)
	state := &pkglicensing.BillingState{
		Capabilities:      []string{"ai_patrol"},
		Limits:            map[string]int64{"host_limit": 10},
		PlanVersion:       "active",
		SubscriptionState: pkglicensing.SubStateActive,
	}

	// Sign with the v6-pre-quickstart format
	state.Integrity = billingIntegrityV6PreQuickstart(state, hmacKey)

	// Current format should NOT verify (different payload format)
	if verifyBillingIntegrity(state, hmacKey) {
		t.Fatal("current format should not match pre-quickstart signature")
	}

	// V6 pre-quickstart format SHOULD verify
	if !verifyBillingIntegrityV6PreQuickstart(state, hmacKey) {
		t.Fatal("v6-pre-quickstart format should verify")
	}

	// After migration: re-sign with current format
	state.Integrity = billingIntegrity(state, hmacKey)
	if !verifyBillingIntegrity(state, hmacKey) {
		t.Fatal("migrated state should verify with current format")
	}
}

func TestNormalizationPreservesQuickstartFields(t *testing.T) {
	now := int64(1709000000)
	original := &pkglicensing.BillingState{
		Capabilities:               []string{},
		Limits:                     map[string]int64{},
		PlanVersion:                "trial",
		SubscriptionState:          pkglicensing.SubStateTrial,
		QuickstartCreditsGranted:   true,
		QuickstartCreditsUsed:      7,
		QuickstartCreditsGrantedAt: &now,
	}

	normalized := pkglicensing.NormalizeBillingState(original)

	if !normalized.QuickstartCreditsGranted {
		t.Fatal("QuickstartCreditsGranted should be preserved")
	}
	if normalized.QuickstartCreditsUsed != 7 {
		t.Errorf("QuickstartCreditsUsed should be 7, got %d", normalized.QuickstartCreditsUsed)
	}
	if normalized.QuickstartCreditsGrantedAt == nil {
		t.Fatal("QuickstartCreditsGrantedAt should be preserved")
	}
	if *normalized.QuickstartCreditsGrantedAt != now {
		t.Errorf("QuickstartCreditsGrantedAt should be %d, got %d", now, *normalized.QuickstartCreditsGrantedAt)
	}

	// Ensure deep clone (pointer independence)
	*original.QuickstartCreditsGrantedAt = 999
	if *normalized.QuickstartCreditsGrantedAt == 999 {
		t.Fatal("QuickstartCreditsGrantedAt should be deep-cloned (independent pointer)")
	}
}

func TestQuickstartCreditsSaveAndLoad(t *testing.T) {
	dir, _ := setupTestBillingDir(t)
	store := NewFileBillingStore(dir)

	now := int64(1709000000)
	state := &pkglicensing.BillingState{
		Capabilities:               []string{},
		Limits:                     map[string]int64{},
		PlanVersion:                "trial",
		SubscriptionState:          pkglicensing.SubStateTrial,
		QuickstartCreditsGranted:   true,
		QuickstartCreditsUsed:      3,
		QuickstartCreditsGrantedAt: &now,
	}

	if err := store.SaveBillingState("default", state); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded, err := store.GetBillingState("default")
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if loaded == nil {
		t.Fatal("loaded state is nil")
	}

	if !loaded.QuickstartCreditsGranted {
		t.Fatal("QuickstartCreditsGranted should be true")
	}
	if loaded.QuickstartCreditsUsed != 3 {
		t.Errorf("expected 3, got %d", loaded.QuickstartCreditsUsed)
	}
	if loaded.QuickstartCreditsGrantedAt == nil || *loaded.QuickstartCreditsGrantedAt != now {
		t.Fatal("QuickstartCreditsGrantedAt should be preserved")
	}
	if loaded.QuickstartCreditsRemaining() != 22 {
		t.Errorf("expected 22 remaining, got %d", loaded.QuickstartCreditsRemaining())
	}
}
