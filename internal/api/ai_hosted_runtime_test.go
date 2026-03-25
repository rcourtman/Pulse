package api

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/chat"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
	"github.com/stretchr/testify/mock"
)

func TestLoadHostedAwareAIConfig_AutoBootstrapsHostedQuickstart(t *testing.T) {
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
		t.Fatal("expected hosted Pulse Assistant config")
	}
	if !loaded.Enabled {
		t.Fatal("expected hosted Pulse Assistant config to be enabled")
	}
	quickstartModel := config.DefaultModelForProvider(config.AIProviderQuickstart)
	if loaded.Model != quickstartModel || loaded.ChatModel != quickstartModel || loaded.PatrolModel != quickstartModel {
		t.Fatalf("expected quickstart models, got model=%q chat=%q patrol=%q", loaded.Model, loaded.ChatModel, loaded.PatrolModel)
	}
	if !persistence.HasAIConfig() {
		t.Fatal("expected hosted AI bootstrap to persist ai config")
	}

	billingStore := config.NewFileBillingStore(mtp.BaseDataDir())
	state, err := billingStore.GetBillingState("default")
	if err != nil {
		t.Fatalf("GetBillingState(default): %v", err)
	}
	if state == nil || !state.QuickstartCreditsGranted {
		t.Fatal("expected hosted AI bootstrap to backfill quickstart credits")
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
	explicit.Model = config.DefaultModelForProvider(config.AIProviderAnthropic)
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

func TestAIHandlerStart_HostedAutoBootstrapStartsService(t *testing.T) {
	oldNewService := newChatService
	defer func() { newChatService = oldNewService }()

	mockSvc := new(MockAIService)
	newChatService = func(cfg chat.Config) AIService {
		return mockSvc
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

	mockSvc.On("Start", mock.Anything).Return(nil).Once()

	if err := handler.Start(context.Background(), nil); err != nil {
		t.Fatalf("Start(): %v", err)
	}
	if handler.defaultService != mockSvc {
		t.Fatal("expected hosted auto-bootstrap to start AI service")
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
