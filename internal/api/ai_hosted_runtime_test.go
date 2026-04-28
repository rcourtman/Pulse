package api

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/chat"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

func TestLoadHostedAwareAIConfig_DoesNotAutoBootstrapHostedQuickstart(t *testing.T) {
	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	persistence, err := mtp.GetPersistence("default")
	if err != nil {
		t.Fatalf("GetPersistence(default): %v", err)
	}

	seedHostedAIBillingState(t, mtp, "default")

	loaded, err := loadHostedAwareAIConfig(true, mtp.BaseDataDir(), "default", persistence)
	if err != nil {
		t.Fatalf("loadHostedAwareAIConfig(): %v", err)
	}
	if loaded == nil {
		t.Fatal("expected default Pulse Assistant config")
	}
	if loaded.Enabled {
		t.Fatal("expected hosted Pulse Assistant config to stay disabled until BYOK/local setup")
	}
	if loaded.Model != "" || loaded.ChatModel != "" || loaded.PatrolModel != "" {
		t.Fatalf("expected no quickstart models, got model=%q chat=%q patrol=%q", loaded.Model, loaded.ChatModel, loaded.PatrolModel)
	}
	if persistence.HasAIConfig() {
		t.Fatal("expected hosted AI loader not to persist an implicit quickstart config")
	}

	billingStore := config.NewFileBillingStore(mtp.BaseDataDir())
	state, err := billingStore.GetBillingState("default")
	if err != nil {
		t.Fatalf("GetBillingState(default): %v", err)
	}
	if state == nil {
		t.Fatal("expected hosted billing state to remain readable")
	}
	if state.QuickstartCreditsGranted {
		t.Fatal("expected hosted AI loader not to grant quickstart credits")
	}
}

func TestLoadHostedAwareAIConfig_DoesNotOverrideExplicitAIConfig(t *testing.T) {
	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	persistence, err := mtp.GetPersistence("default")
	if err != nil {
		t.Fatalf("GetPersistence(default): %v", err)
	}

	seedHostedAIBillingState(t, mtp, "default")

	explicit := config.NewDefaultAIConfig()
	explicit.Enabled = false
	explicit.Model = "anthropic:existing-explicit-model"
	if err := persistence.SaveAIConfig(*explicit); err != nil {
		t.Fatalf("SaveAIConfig(): %v", err)
	}

	loaded, err := loadHostedAwareAIConfig(true, mtp.BaseDataDir(), "default", persistence)
	if err != nil {
		t.Fatalf("loadHostedAwareAIConfig(): %v", err)
	}
	if loaded == nil {
		t.Fatal("expected explicit Pulse Assistant config")
	}
	if loaded.Enabled {
		t.Fatal("expected explicit disabled Pulse Assistant config to remain disabled")
	}
	if loaded.Model != explicit.Model {
		t.Fatalf("model=%q, want %q", loaded.Model, explicit.Model)
	}
}

func TestLoadHostedAwareAIConfig_HostedTenantFallsBackToDefaultBillingState(t *testing.T) {
	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	persistence, err := mtp.GetPersistence("t-tenant")
	if err != nil {
		t.Fatalf("GetPersistence(t-tenant): %v", err)
	}

	seedHostedAIBillingState(t, mtp, "default")

	loaded, err := loadHostedAwareAIConfig(true, mtp.BaseDataDir(), "t-tenant", persistence)
	if err != nil {
		t.Fatalf("loadHostedAwareAIConfig(): %v", err)
	}
	if loaded == nil {
		t.Fatal("expected default tenant AI config")
	}
	if loaded.Enabled {
		t.Fatalf("expected hosted tenant AI config to stay disabled until BYOK/local setup, got %#v", loaded)
	}
	if persistence.HasAIConfig() {
		t.Fatal("expected hosted tenant AI loader not to persist an implicit quickstart config")
	}

	billingStore := config.NewFileBillingStore(mtp.BaseDataDir())
	defaultState, err := billingStore.GetBillingState("default")
	if err != nil {
		t.Fatalf("GetBillingState(default): %v", err)
	}
	if defaultState == nil {
		t.Fatal("expected hosted tenant AI loader to keep default hosted billing state readable")
	}
	if defaultState.QuickstartCreditsGranted {
		t.Fatal("expected hosted tenant AI loader not to grant quickstart credits")
	}
	tenantState, err := billingStore.GetBillingState("t-tenant")
	if err != nil {
		t.Fatalf("GetBillingState(t-tenant): %v", err)
	}
	if tenantState != nil && tenantState.SubscriptionState != "" {
		t.Fatalf("expected tenant org to avoid shadow billing state, got %#v", tenantState)
	}
}

func TestAIHandlerStart_DoesNotAutoBootstrapHostedQuickstartService(t *testing.T) {
	oldNewService := newChatService
	defer func() { newChatService = oldNewService }()

	newChatService = func(chat.Config) AIService {
		t.Fatal("newChatService must not be called without explicit BYOK/local AI config")
		return nil
	}

	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	persistence, err := mtp.GetPersistence("default")
	if err != nil {
		t.Fatalf("GetPersistence(default): %v", err)
	}
	seedHostedAIBillingState(t, mtp, "default")

	handler := NewAIHandler(mtp, nil, nil)
	handler.defaultPersistence = persistence
	handler.hostedMode = true

	if err := handler.Start(context.Background(), nil); err != nil {
		t.Fatalf("Start(): %v", err)
	}
	if handler.defaultService != nil {
		t.Fatal("expected hosted start not to create a quickstart-backed service")
	}
}

func seedHostedAIBillingState(t *testing.T, mtp *config.MultiTenantPersistence, orgID string) {
	t.Helper()

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate entitlement keypair: %v", err)
	}
	t.Setenv(pkglicensing.TrialActivationPublicKeyEnvVar, base64.StdEncoding.EncodeToString(pub))

	entitlementJWT, err := pkglicensing.SignEntitlementLeaseToken(priv, pkglicensing.EntitlementLeaseClaims{
		OrgID:             orgID,
		InstanceHost:      "t-hostedai.cloud.pulserelay.pro",
		PlanVersion:       "msp_starter",
		SubscriptionState: pkglicensing.SubStateActive,
		Capabilities:      []string{pkglicensing.FeatureAIPatrol, pkglicensing.FeatureAIAutoFix},
	})
	if err != nil {
		t.Fatalf("SignEntitlementLeaseToken(): %v", err)
	}

	billingStore := config.NewFileBillingStore(mtp.BaseDataDir())
	if err := billingStore.SaveBillingState(orgID, &billingState{
		EntitlementJWT:          entitlementJWT,
		EntitlementRefreshToken: "etr_hosted_test_bootstrap",
	}); err != nil {
		t.Fatalf("SaveBillingState(): %v", err)
	}
}
