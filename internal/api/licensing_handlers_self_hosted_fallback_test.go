package api

import (
	"context"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

func TestLicenseHandlersService_SelfHostedNonDefaultFallbackToDefaultBillingState(t *testing.T) {
	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)

	// Ensure persistence roots exist so billing store can read/write cleanly.
	if _, err := mtp.GetPersistence("default"); err != nil {
		t.Fatalf("init default persistence: %v", err)
	}
	if _, err := mtp.GetPersistence("acme"); err != nil {
		t.Fatalf("init acme persistence: %v", err)
	}

	store := config.NewFileBillingStore(baseDir)
	err := store.SaveBillingState("default", &pkglicensing.BillingState{
		Capabilities:      []string{pkglicensing.FeatureMultiTenant},
		PlanVersion:       string(pkglicensing.TierEnterprise),
		SubscriptionState: pkglicensing.SubStateActive,
	})
	if err != nil {
		t.Fatalf("save default billing state: %v", err)
	}

	handlers := NewLicenseHandlers(mtp, false)
	ctx := context.WithValue(context.Background(), OrgIDContextKey, "acme")
	service := handlers.Service(ctx)
	if service == nil {
		t.Fatalf("service is nil")
	}

	if !service.HasFeature(pkglicensing.FeatureMultiTenant) {
		t.Fatalf("expected non-default org to inherit default org multi_tenant entitlement in self-hosted mode")
	}
}

func TestLicenseHandlersService_SelfHostedNonDefaultBillingStateOverridesDefaultFallback(t *testing.T) {
	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)

	if _, err := mtp.GetPersistence("default"); err != nil {
		t.Fatalf("init default persistence: %v", err)
	}
	if _, err := mtp.GetPersistence("acme"); err != nil {
		t.Fatalf("init acme persistence: %v", err)
	}

	store := config.NewFileBillingStore(baseDir)
	if err := store.SaveBillingState("default", &pkglicensing.BillingState{
		Capabilities:      []string{pkglicensing.FeatureMultiTenant},
		PlanVersion:       string(pkglicensing.TierEnterprise),
		SubscriptionState: pkglicensing.SubStateActive,
	}); err != nil {
		t.Fatalf("save default billing state: %v", err)
	}
	if err := store.SaveBillingState("acme", &pkglicensing.BillingState{
		Capabilities:      []string{},
		PlanVersion:       string(pkglicensing.TierFree),
		SubscriptionState: pkglicensing.SubStateExpired,
	}); err != nil {
		t.Fatalf("save acme billing state: %v", err)
	}

	handlers := NewLicenseHandlers(mtp, false)
	ctx := context.WithValue(context.Background(), OrgIDContextKey, "acme")
	service := handlers.Service(ctx)
	if service == nil {
		t.Fatalf("service is nil")
	}

	if service.HasFeature(pkglicensing.FeatureMultiTenant) {
		t.Fatalf("expected explicit non-default billing state to take precedence over default fallback")
	}
}
