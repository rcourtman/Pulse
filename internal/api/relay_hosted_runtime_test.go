package api

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/relay"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

func TestLoadRelayConfigForRuntime_HostedEntitlementAutoBootstrapsRelay(t *testing.T) {
	router, entitlementJWT, instanceHost := newHostedRelayRuntimeTestRouter(t)

	cfg, err := router.loadRelayConfigForRuntime(context.Background())
	if err != nil {
		t.Fatalf("loadRelayConfigForRuntime() error = %v", err)
	}
	if cfg == nil {
		t.Fatal("loadRelayConfigForRuntime() returned nil config")
	}
	if !cfg.Enabled {
		t.Fatal("expected hosted relay bootstrap to enable relay")
	}
	if cfg.ServerURL != relay.DefaultServerURL {
		t.Fatalf("server_url = %q, want %q", cfg.ServerURL, relay.DefaultServerURL)
	}
	if cfg.InstanceSecret != instanceHost {
		t.Fatalf("instance_secret = %q, want %q", cfg.InstanceSecret, instanceHost)
	}
	if cfg.IdentityPrivateKey == "" || cfg.IdentityPublicKey == "" || cfg.IdentityFingerprint == "" {
		t.Fatalf("expected hosted relay bootstrap to generate full identity, got %+v", cfg)
	}

	persisted, err := router.persistence.LoadRelayConfig()
	if err != nil {
		t.Fatalf("LoadRelayConfig() error = %v", err)
	}
	if persisted == nil || !persisted.Enabled {
		t.Fatal("expected persisted hosted relay config to be enabled")
	}
	if persisted.InstanceSecret != instanceHost {
		t.Fatalf("persisted instance_secret = %q, want %q", persisted.InstanceSecret, instanceHost)
	}

	gotToken := router.relayRegistrationToken(context.Background())
	if gotToken != entitlementJWT {
		t.Fatalf("relayRegistrationToken() = %q, want hosted entitlement JWT", gotToken)
	}
}

func TestLoadRelayConfigForRuntime_DoesNotOverrideExplicitDisabledRelayConfig(t *testing.T) {
	router, _, instanceHost := newHostedRelayRuntimeTestRouter(t)

	explicit := relay.Config{
		Enabled:             false,
		ServerURL:           "wss://relay.example.test/ws/instance",
		InstanceSecret:      "user-managed-secret",
		IdentityPrivateKey:  "user-private",
		IdentityPublicKey:   "user-public",
		IdentityFingerprint: "USER:FP",
	}
	if err := router.persistence.SaveRelayConfig(explicit); err != nil {
		t.Fatalf("SaveRelayConfig() error = %v", err)
	}

	cfg, err := router.loadRelayConfigForRuntime(context.Background())
	if err != nil {
		t.Fatalf("loadRelayConfigForRuntime() error = %v", err)
	}
	if cfg.Enabled {
		t.Fatal("expected explicit disabled relay config to remain disabled")
	}
	if cfg.InstanceSecret != explicit.InstanceSecret {
		t.Fatalf("instance_secret = %q, want %q", cfg.InstanceSecret, explicit.InstanceSecret)
	}
	if cfg.InstanceSecret == instanceHost {
		t.Fatalf("expected explicit config to avoid hosted auto-bootstrap secret %q", instanceHost)
	}
}

func newHostedRelayRuntimeTestRouter(t *testing.T) (*Router, string, string) {
	t.Helper()

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate entitlement keypair: %v", err)
	}
	t.Setenv(pkglicensing.TrialActivationPublicKeyEnvVar, base64.StdEncoding.EncodeToString(pub))

	baseDir := t.TempDir()
	cfg := &config.Config{
		DataPath:     baseDir,
		FrontendPort: 7655,
		PublicURL:    "https://t-HOSTEDRELAY01.cloud.pulserelay.pro",
	}
	mtp := config.NewMultiTenantPersistence(baseDir)
	persistence, err := mtp.GetPersistence(hostedRelayBootstrapOrgID)
	if err != nil {
		t.Fatalf("GetPersistence(default) error = %v", err)
	}

	instanceHost := "t-hostedrelay01.cloud.pulserelay.pro"
	entitlementJWT, err := pkglicensing.SignEntitlementLeaseToken(priv, pkglicensing.EntitlementLeaseClaims{
		OrgID:             hostedRelayBootstrapOrgID,
		InstanceHost:      instanceHost,
		PlanVersion:       "cloud",
		SubscriptionState: pkglicensing.SubStateActive,
		Capabilities:      []string{pkglicensing.FeatureRelay},
	})
	if err != nil {
		t.Fatalf("SignEntitlementLeaseToken() error = %v", err)
	}

	billingStore := config.NewFileBillingStore(mtp.BaseDataDir())
	if err := billingStore.SaveBillingState(hostedRelayBootstrapOrgID, &billingState{
		PlanVersion:       "cloud",
		SubscriptionState: subscriptionStateActiveValue,
		Capabilities:      []string{pkglicensing.FeatureRelay},
		EntitlementJWT:    entitlementJWT,
	}); err != nil {
		t.Fatalf("SaveBillingState() error = %v", err)
	}

	router := &Router{
		config:          cfg,
		persistence:     persistence,
		licenseHandlers: NewLicenseHandlers(mtp, true, cfg),
	}
	return router, entitlementJWT, instanceHost
}
