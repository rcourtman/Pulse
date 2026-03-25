package ai

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

func TestFileQuickstartCreditManager_OrgResolverUsesEffectiveBillingOrg(t *testing.T) {
	baseDir := t.TempDir()
	billingStore := config.NewFileBillingStore(baseDir)

	state := pkglicensing.DefaultBillingState()
	state.GrantQuickstartCredits()
	if err := billingStore.SaveBillingState("default", state); err != nil {
		t.Fatalf("SaveBillingState(default): %v", err)
	}

	mgr := NewFileQuickstartCreditManagerWithOrgResolver(
		billingStore,
		"t-tenant",
		func() string { return "default" },
		func() *config.AIConfig { return &config.AIConfig{Enabled: true} },
		"t-tenant",
	)

	if !mgr.HasCredits() {
		t.Fatal("expected quickstart credits through resolved billing org")
	}
	if got := mgr.CreditsRemaining(); got != pkglicensing.QuickstartCreditsTotal {
		t.Fatalf("CreditsRemaining() = %d, want %d", got, pkglicensing.QuickstartCreditsTotal)
	}
	if err := mgr.ConsumeCredit(); err != nil {
		t.Fatalf("ConsumeCredit(): %v", err)
	}

	defaultState, err := billingStore.GetBillingState("default")
	if err != nil {
		t.Fatalf("GetBillingState(default): %v", err)
	}
	if defaultState == nil || defaultState.QuickstartCreditsUsed != 1 {
		t.Fatalf("expected resolved default billing state to consume one credit, got %#v", defaultState)
	}
	tenantState, err := billingStore.GetBillingState("t-tenant")
	if err != nil {
		t.Fatalf("GetBillingState(t-tenant): %v", err)
	}
	if tenantState != nil {
		t.Fatalf("expected no tenant shadow billing state, got %#v", tenantState)
	}
}
