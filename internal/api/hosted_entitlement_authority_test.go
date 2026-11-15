package api

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

type hostedAuthorityFixture struct {
	reg      *registry.TenantRegistry
	handlers *LicenseHandlers
	store    *config.FileBillingStore
	baseDir  string
	original *pkglicensing.BillingState
}

func newHostedAuthorityFixture(t *testing.T) *hostedAuthorityFixture {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(pkglicensing.TrialActivationPublicKeyEnvVar, base64.StdEncoding.EncodeToString(pub))
	reg, err := registry.NewTenantRegistry(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reg.Close() })
	if err := reg.CreateAccount(&registry.Account{ID: "a_AUTHORITY", Kind: registry.AccountKindIndividual}); err != nil {
		t.Fatal(err)
	}
	tenant := &registry.Tenant{ID: "t-AUTHORITY", AccountID: "a_AUTHORITY", Email: "owner@example.com", State: registry.TenantStateActive, PlanVersion: "cloud_starter"}
	if err := reg.Create(tenant); err != nil {
		t.Fatal(err)
	}
	service := cloudcp.NewHostedEntitlementsService(&cloudcp.CPConfig{BaseURL: "https://cloud.example.com", TrialActivationPrivateKey: base64.StdEncoding.EncodeToString(priv)}, reg)
	// An expired, legitimately issued lease may retain its refresh identity.
	// The missing current billing row is not evidence authorizing its renewal.
	now := time.Now().UTC()
	service.SetNow(func() time.Time { return now.Add(-48 * time.Hour) })
	original, err := service.IssueTenantBillingState(tenant, pkglicensing.SubStateActive, "cloud_starter", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	service.SetNow(func() time.Time { return now })
	cpHandler := cloudcp.NewHostedEntitlementHandlers(service)
	server := httptest.NewServer(http.HandlerFunc(cpHandler.HandleRefresh))
	t.Cleanup(server.Close)
	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	if _, err := mtp.GetPersistence("default"); err != nil {
		t.Fatal(err)
	}
	store := config.NewFileBillingStore(baseDir)
	if err := store.SaveBillingState("default", original); err != nil {
		t.Fatal(err)
	}
	h := NewLicenseHandlers(mtp, true, &config.Config{PublicURL: "https://t-authority.cloud.example.com", ProTrialSignupURL: server.URL})
	t.Cleanup(func() { h.stopHostedEntitlementRefreshLoop("default") })
	return &hostedAuthorityFixture{reg: reg, handlers: h, store: store, baseDir: baseDir, original: original}
}

func (f *hostedAuthorityFixture) restoreBilling(t *testing.T) {
	t.Helper()
	if err := f.reg.CreateStripeAccount(&registry.StripeAccount{AccountID: "a_AUTHORITY", SubscriptionState: "active", PlanVersion: "cloud_starter"}); err != nil {
		t.Fatal(err)
	}
}

func TestHostedEntitlementMissingBillingPreservesClientRecovery(t *testing.T) {
	f := newHostedAuthorityFixture(t)
	file := filepath.Join(f.baseDir, "billing.json")
	before, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	refreshed, permanent, err := f.handlers.refreshHostedEntitlementLeaseOnce("default", nil)
	var refreshErr *hostedEntitlementRefreshError
	if refreshed || permanent || !errors.As(err, &refreshErr) || refreshErr.statusCode != http.StatusServiceUnavailable {
		t.Fatalf("missing authority result: refreshed=%v permanent=%v err=%v", refreshed, permanent, err)
	}
	after, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("retryable authority failure rewrote saved billing state")
	}
	saved, err := f.store.GetBillingState("default")
	if err != nil {
		t.Fatal(err)
	}
	if saved.EntitlementRefreshToken != f.original.EntitlementRefreshToken || saved.EntitlementJWT != f.original.EntitlementJWT {
		t.Fatal("retryable failure replaced the original identity or extended the old lease")
	}
	authority, err := f.reg.GetHostedEntitlementByRefreshToken(f.original.EntitlementRefreshToken)
	if err != nil {
		t.Fatal(err)
	}
	if authority == nil || authority.RevokedAt != nil || authority.LastRefreshedAt != nil {
		t.Fatal("failed refresh mutated server-side identity or successful-refresh time")
	}

	f.restoreBilling(t)
	refreshed, permanent, err = f.handlers.refreshHostedEntitlementLeaseOnce("default", nil)
	if err != nil || !refreshed || permanent {
		t.Fatalf("restored authority result: refreshed=%v permanent=%v err=%v", refreshed, permanent, err)
	}
	saved, err = f.store.GetBillingState("default")
	if err != nil {
		t.Fatal(err)
	}
	if saved.EntitlementRefreshToken != f.original.EntitlementRefreshToken || saved.EntitlementJWT == f.original.EntitlementJWT {
		t.Fatal("recovery failed to renew using the same saved identity")
	}
	// Exercise the consuming runtime evaluator, not just the refresh endpoint.
	evaluator := newLicenseEvaluatorForBillingStoreFromLicensing(f.store, "default", 0, "t-authority.cloud.example.com")
	if evaluator.SubscriptionState() != pkglicensing.SubStateActive || !evaluator.HasCapability(pkglicensing.FeatureAIAutoFix) {
		t.Fatal("restored valid lease did not restore its paid capabilities")
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var stored pkglicensing.BillingState
	if err := json.Unmarshal(raw, &stored); err != nil {
		t.Fatal(err)
	}
	if stored.EntitlementRefreshToken == f.original.EntitlementRefreshToken || stored.EntitlementJWT == saved.EntitlementJWT {
		t.Fatal("saved recovery identity or lease is not encrypted at rest")
	}
	// A real stored cancellation is still permanent; no new retry policy may
	// recreate withdrawn authority or hide it as a transient read failure.
	sa, err := f.reg.GetStripeAccount("a_AUTHORITY")
	if err != nil {
		t.Fatal(err)
	}
	sa.SubscriptionState = "canceled"
	if err := f.reg.UpdateStripeAccount(sa); err != nil {
		t.Fatal(err)
	}
	refreshed, permanent, err = f.handlers.refreshHostedEntitlementLeaseOnce("default", nil)
	if refreshed || !permanent || err == nil {
		t.Fatalf("cancellation result: refreshed=%v permanent=%v err=%v", refreshed, permanent, err)
	}
	saved, err = f.store.GetBillingState("default")
	if err != nil {
		t.Fatal(err)
	}
	if saved.EntitlementRefreshToken != "" || saved.EntitlementJWT != "" {
		t.Fatal("confirmed cancellation did not clear renewable authority")
	}
}
