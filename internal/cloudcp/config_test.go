package cloudcp

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

func setRequiredCPEnv(t *testing.T) {
	t.Helper()
	t.Setenv("CP_ADMIN_KEY", "test-key")
	t.Setenv("CP_BASE_URL", "https://cloud.example.com")
	t.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_test")
	t.Setenv("CP_ENV", "development")
	t.Setenv("CP_REQUIRE_EMAIL_PROVIDER", "false")
}

func setTrialSigningEnv(t *testing.T) {
	t.Helper()
	t.Setenv("CP_TRIAL_ACTIVATION_PRIVATE_KEY", "A8medgdNdm12GXfTXWo6+TMZ2BeHPCLg2kd0znn6ZUk=")
}

func TestLoadConfig_MissingRequired(t *testing.T) {
	// Clear relevant env vars
	for _, key := range []string{
		"CP_ADMIN_KEY", "CP_BASE_URL", "STRIPE_WEBHOOK_SECRET",
		"CP_DATA_DIR", "CP_BIND_ADDRESS", "CP_PORT",
	} {
		t.Setenv(key, "")
	}

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error for missing required vars")
	}
}

func TestLoadConfig_AllRequired(t *testing.T) {
	setRequiredCPEnv(t)

	// Clear optional vars to use defaults
	for _, key := range []string{
		"CP_DATA_DIR", "CP_BIND_ADDRESS", "CP_PORT",
		"CP_PULSE_IMAGE", "CP_DOCKER_NETWORK",
		"CP_TENANT_MEMORY_LIMIT", "CP_TENANT_CPU_SHARES",
		"STRIPE_API_KEY",
	} {
		t.Setenv(key, "")
	}

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if cfg.AdminKey != "test-key" {
		t.Errorf("AdminKey = %q, want %q", cfg.AdminKey, "test-key")
	}
	if cfg.BaseURL != "https://cloud.example.com" {
		t.Errorf("BaseURL = %q", cfg.BaseURL)
	}
	if cfg.Port != 8443 {
		t.Errorf("Port = %d, want 8443", cfg.Port)
	}
	if cfg.DataDir != "/data" {
		t.Errorf("DataDir = %q, want /data", cfg.DataDir)
	}
	if cfg.BindAddress != "0.0.0.0" {
		t.Errorf("BindAddress = %q, want 0.0.0.0", cfg.BindAddress)
	}
	if cfg.TenantLogMaxSize != "10m" {
		t.Errorf("TenantLogMaxSize = %q, want 10m", cfg.TenantLogMaxSize)
	}
	if cfg.TenantLogMaxFile != 3 {
		t.Errorf("TenantLogMaxFile = %d, want 3", cfg.TenantLogMaxFile)
	}
	if cfg.EmailReplyTo != "support@pulserelay.pro" {
		t.Errorf("EmailReplyTo = %q, want support@pulserelay.pro", cfg.EmailReplyTo)
	}
	if cfg.StorageGuardrailsEnabled {
		t.Errorf("StorageGuardrailsEnabled = true in development, want false")
	}
}

func TestLoadConfig_CustomValues(t *testing.T) {
	t.Setenv("CP_ADMIN_KEY", "key")
	t.Setenv("CP_BASE_URL", "https://test.example.com")
	t.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_x")
	t.Setenv("CP_ENV", "development")
	t.Setenv("CP_REQUIRE_EMAIL_PROVIDER", "false")
	t.Setenv("CP_PORT", "9000")
	t.Setenv("CP_DATA_DIR", "/custom/data")
	t.Setenv("CP_BIND_ADDRESS", "127.0.0.1")
	t.Setenv("CP_TENANT_LOG_MAX_SIZE", "25m")
	t.Setenv("CP_TENANT_LOG_MAX_FILE", "4")
	t.Setenv("CP_STORAGE_GUARDRAILS_ENABLED", "true")
	t.Setenv("CP_STORAGE_ROOT_PATH", "/host-root")
	t.Setenv("CP_STORAGE_DATA_PATH", "/tenant-data")
	t.Setenv("CP_STORAGE_DOCKER_PATH", "/host-var-lib-docker")
	t.Setenv("CP_STORAGE_MIN_ROOT_AVAILABLE", "12GiB")
	t.Setenv("CP_STORAGE_MIN_DATA_AVAILABLE", "6GiB")
	t.Setenv("CP_STORAGE_MIN_DOCKER_AVAILABLE", "8GiB")
	t.Setenv("CP_STORAGE_MAX_DOCKER_BUILD_CACHE", "1500MiB")
	t.Setenv("CP_PROOF_TENANT_MAX_AGE", "6h")
	t.Setenv("CP_PROOF_TENANT_MATCHERS", "proof,canary,proof")
	t.Setenv("PULSE_EMAIL_REPLY_TO", "help@example.com")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Port != 9000 {
		t.Errorf("Port = %d, want 9000", cfg.Port)
	}
	if cfg.DataDir != "/custom/data" {
		t.Errorf("DataDir = %q", cfg.DataDir)
	}
	if cfg.BindAddress != "127.0.0.1" {
		t.Errorf("BindAddress = %q", cfg.BindAddress)
	}
	if cfg.TenantLogMaxSize != "25m" {
		t.Errorf("TenantLogMaxSize = %q", cfg.TenantLogMaxSize)
	}
	if cfg.TenantLogMaxFile != 4 {
		t.Errorf("TenantLogMaxFile = %d, want 4", cfg.TenantLogMaxFile)
	}
	if !cfg.StorageGuardrailsEnabled {
		t.Errorf("StorageGuardrailsEnabled = false, want true")
	}
	if cfg.StorageRootPath != "/host-root" || cfg.StorageDataPath != "/tenant-data" || cfg.StorageDockerPath != "/host-var-lib-docker" {
		t.Fatalf("storage paths = %q %q %q", cfg.StorageRootPath, cfg.StorageDataPath, cfg.StorageDockerPath)
	}
	if cfg.StorageMinRootAvailableBytes != 12*1024*1024*1024 {
		t.Fatalf("StorageMinRootAvailableBytes = %d", cfg.StorageMinRootAvailableBytes)
	}
	if cfg.StorageMinDataAvailableBytes != 6*1024*1024*1024 {
		t.Fatalf("StorageMinDataAvailableBytes = %d", cfg.StorageMinDataAvailableBytes)
	}
	if cfg.StorageMinDockerAvailableBytes != 8*1024*1024*1024 {
		t.Fatalf("StorageMinDockerAvailableBytes = %d", cfg.StorageMinDockerAvailableBytes)
	}
	if cfg.StorageMaxDockerBuildCacheBytes != 1500*1024*1024 {
		t.Fatalf("StorageMaxDockerBuildCacheBytes = %d", cfg.StorageMaxDockerBuildCacheBytes)
	}
	if cfg.ProofTenantMaxAge.String() != "6h0m0s" {
		t.Fatalf("ProofTenantMaxAge = %s, want 6h", cfg.ProofTenantMaxAge)
	}
	if got := strings.Join(cfg.ProofTenantMatchers, ","); got != "proof,canary" {
		t.Fatalf("ProofTenantMatchers = %q, want proof,canary", got)
	}
	if cfg.EmailReplyTo != "help@example.com" {
		t.Fatalf("EmailReplyTo = %q, want help@example.com", cfg.EmailReplyTo)
	}
}

func TestLoadConfig_EnablesStorageGuardrailsByDefaultInProduction(t *testing.T) {
	setRequiredCPEnv(t)
	t.Setenv("CP_ENV", "production")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if !cfg.StorageGuardrailsEnabled {
		t.Fatal("StorageGuardrailsEnabled = false, want true")
	}
	if cfg.StorageRootPath != "/" {
		t.Fatalf("StorageRootPath = %q, want /", cfg.StorageRootPath)
	}
	if cfg.StorageDataPath != "/data" {
		t.Fatalf("StorageDataPath = %q, want /data", cfg.StorageDataPath)
	}
	if cfg.StorageDockerPath != "/var/lib/docker" {
		t.Fatalf("StorageDockerPath = %q, want /var/lib/docker", cfg.StorageDockerPath)
	}
	if got := strings.Join(cfg.ProofTenantMatchers, ","); got != "proof,canary,rehearsal,msp_prod,ownerseed,owner_seed" {
		t.Fatalf("ProofTenantMatchers = %q", got)
	}
}

func TestLoadConfig_InvalidStorageByteSize(t *testing.T) {
	setRequiredCPEnv(t)
	t.Setenv("CP_STORAGE_MIN_ROOT_AVAILABLE", "not-a-size")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error for invalid storage byte size")
	}
	if !strings.Contains(err.Error(), "CP_STORAGE_MIN_ROOT_AVAILABLE must be a valid byte size") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadConfig_DerivesTrialActivationPublicKey(t *testing.T) {
	setRequiredCPEnv(t)
	setTrialSigningEnv(t)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if strings.TrimSpace(cfg.TrialActivationPublicKey) == "" {
		t.Fatal("TrialActivationPublicKey = empty, want derived public key")
	}
}

func TestLoadConfig_TrustedProxyCIDRs(t *testing.T) {
	setRequiredCPEnv(t)
	t.Setenv("CP_TRUSTED_PROXY_CIDRS", "172.18.0.0/16, 127.0.0.1/32")
	t.Setenv("PULSE_TRUSTED_PROXY_CIDRS", "192.168.0.0/16")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	want := []string{"172.18.0.0/16", "127.0.0.1/32", "192.168.0.0/16"}
	if len(cfg.TrustedProxyCIDRs) != len(want) {
		t.Fatalf("len(TrustedProxyCIDRs) = %d, want %d (%v)", len(cfg.TrustedProxyCIDRs), len(want), cfg.TrustedProxyCIDRs)
	}
	for i, expected := range want {
		if cfg.TrustedProxyCIDRs[i] != expected {
			t.Fatalf("TrustedProxyCIDRs[%d] = %q, want %q", i, cfg.TrustedProxyCIDRs[i], expected)
		}
	}
}

func TestLoadConfig_InvalidPortParse(t *testing.T) {
	setRequiredCPEnv(t)
	t.Setenv("CP_PORT", "not-a-number")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error for invalid CP_PORT")
	}
	if !strings.Contains(err.Error(), "CP_PORT must be a valid integer") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadConfig_InvalidPortRange(t *testing.T) {
	setRequiredCPEnv(t)
	t.Setenv("CP_PORT", "0")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error for invalid CP_PORT range")
	}
	if !strings.Contains(err.Error(), "CP_PORT must be between 1 and 65535") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadConfig_InvalidTenantLimits(t *testing.T) {
	t.Run("memory limit", func(t *testing.T) {
		setRequiredCPEnv(t)
		t.Setenv("CP_TENANT_MEMORY_LIMIT", "0")

		_, err := LoadConfig()
		if err == nil {
			t.Fatal("expected error for invalid CP_TENANT_MEMORY_LIMIT")
		}
		if !strings.Contains(err.Error(), "CP_TENANT_MEMORY_LIMIT must be greater than 0") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("cpu shares", func(t *testing.T) {
		setRequiredCPEnv(t)
		t.Setenv("CP_TENANT_CPU_SHARES", "-10")

		_, err := LoadConfig()
		if err == nil {
			t.Fatal("expected error for invalid CP_TENANT_CPU_SHARES")
		}
		if !strings.Contains(err.Error(), "CP_TENANT_CPU_SHARES must be greater than 0") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestLoadConfig_InvalidRateLimits(t *testing.T) {
	setRequiredCPEnv(t)
	t.Setenv("CP_RL_ADMIN_PER_MINUTE", "0")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error for invalid CP_RL_ADMIN_PER_MINUTE")
	}
	if !strings.Contains(err.Error(), "CP_RL_ADMIN_PER_MINUTE must be greater than 0") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadConfig_InvalidBaseURL(t *testing.T) {
	setRequiredCPEnv(t)
	t.Setenv("CP_BASE_URL", "cloud.example.com")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error for invalid CP_BASE_URL")
	}
	if !strings.Contains(err.Error(), "CP_BASE_URL must use http or https scheme") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadConfig_EmailProviderRequired(t *testing.T) {
	setRequiredCPEnv(t)
	t.Setenv("CP_REQUIRE_EMAIL_PROVIDER", "true")
	t.Setenv("RESEND_API_KEY", "")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error when email provider is required but RESEND_API_KEY is missing")
	}
	if !strings.Contains(err.Error(), "RESEND_API_KEY is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadConfig_RejectsNonCloudTrialSignupPriceIDInProductionCatalog(t *testing.T) {
	setRequiredCPEnv(t)
	setTrialSigningEnv(t)
	t.Setenv("CP_ENV", "production")
	t.Setenv("STRIPE_API_KEY", "sk_live_123")
	t.Setenv("CP_TRIAL_SIGNUP_PRICE_ID", "price_1T47OVBrHBocJIGHg4sMHMV7")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error for non-cloud CP_TRIAL_SIGNUP_PRICE_ID")
	}
	if !strings.Contains(err.Error(), "CP_TRIAL_SIGNUP_PRICE_ID must map to the canonical cloud_starter Stripe price") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadConfig_AllowsUnmappedTestPriceIDsInStaging(t *testing.T) {
	setRequiredCPEnv(t)
	setTrialSigningEnv(t)
	t.Setenv("CP_ENV", "staging")
	t.Setenv("STRIPE_API_KEY", "sk_test_123")
	t.Setenv("CP_PUBLIC_CLOUD_SIGNUP_ENABLED", "true")
	t.Setenv("CP_TRIAL_SIGNUP_PRICE_ID", "price_1SandboxTrial123")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.TrialSignupPriceID != "price_1SandboxTrial123" {
		t.Fatalf("TrialSignupPriceID=%q want staging sandbox price", cfg.TrialSignupPriceID)
	}
}

func TestLoadConfig_RejectsInvalidTestPriceIDInStaging(t *testing.T) {
	setRequiredCPEnv(t)
	setTrialSigningEnv(t)
	t.Setenv("CP_ENV", "staging")
	t.Setenv("STRIPE_API_KEY", "sk_test_123")
	t.Setenv("CP_PUBLIC_CLOUD_SIGNUP_ENABLED", "true")
	t.Setenv("CP_TRIAL_SIGNUP_PRICE_ID", "not_a_price")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error for invalid staging CP_TRIAL_SIGNUP_PRICE_ID")
	}
	if !strings.Contains(err.Error(), "must be a Stripe price id") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadConfig_AcceptsCanonicalCloudSignupPriceIDs(t *testing.T) {
	setRequiredCPEnv(t)
	setTrialSigningEnv(t)
	t.Setenv("STRIPE_API_KEY", "sk_test_123")
	t.Setenv("CP_PUBLIC_CLOUD_SIGNUP_ENABLED", "true")
	t.Setenv("CP_TRIAL_SIGNUP_PRICE_ID", "price_1T5kflBrHBocJIGHUqPv1dzV")
	t.Setenv("CP_CLOUD_POWER_PRICE_ID", "price_1T5kg2BrHBocJIGHmkoF0zXY")
	t.Setenv("CP_CLOUD_MAX_PRICE_ID", "price_1T5kg4BrHBocJIGHHa8Ecqho")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.TrialSignupPriceID != "price_1T5kflBrHBocJIGHUqPv1dzV" {
		t.Fatalf("TrialSignupPriceID=%q want canonical cloud starter price", cfg.TrialSignupPriceID)
	}
}

func TestLoadConfig_AcceptsCanonicalMSPSignupPriceIDs(t *testing.T) {
	setRequiredCPEnv(t)
	setTrialSigningEnv(t)
	t.Setenv("CP_ENV", "production")
	t.Setenv("STRIPE_API_KEY", "sk_live_123")
	t.Setenv("CP_MSP_STARTER_PRICE_ID", "price_1T5kgTBrHBocJIGHjOs15LI2")
	t.Setenv("CP_MSP_GROWTH_PRICE_ID", "price_1T5kgVBrHBocJIGHulNsCTb1")
	t.Setenv("CP_MSP_SCALE_PRICE_ID", "price_1T5kgWBrHBocJIGHo40iFeRd")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.CloudMSPStarterPriceID != "price_1T5kgTBrHBocJIGHjOs15LI2" {
		t.Fatalf("CloudMSPStarterPriceID=%q want canonical msp_starter price", cfg.CloudMSPStarterPriceID)
	}
	if cfg.CloudMSPGrowthPriceID != "price_1T5kgVBrHBocJIGHulNsCTb1" {
		t.Fatalf("CloudMSPGrowthPriceID=%q want canonical msp_growth price", cfg.CloudMSPGrowthPriceID)
	}
	if cfg.CloudMSPScalePriceID != "price_1T5kgWBrHBocJIGHo40iFeRd" {
		t.Fatalf("CloudMSPScalePriceID=%q want canonical msp_scale price", cfg.CloudMSPScalePriceID)
	}
}

func TestLoadConfig_RejectsNonMSPStarterPriceIDInProductionCatalog(t *testing.T) {
	setRequiredCPEnv(t)
	setTrialSigningEnv(t)
	t.Setenv("CP_ENV", "production")
	t.Setenv("STRIPE_API_KEY", "sk_live_123")
	// A canonical cloud_power price is a valid Stripe price but maps to the
	// wrong plan version for the MSP Starter slot, so config must fail closed.
	t.Setenv("CP_MSP_STARTER_PRICE_ID", "price_1T5kg2BrHBocJIGHmkoF0zXY")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error for non-msp_starter CP_MSP_STARTER_PRICE_ID")
	}
	if !strings.Contains(err.Error(), "CP_MSP_STARTER_PRICE_ID must map to the canonical msp_starter Stripe price") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadConfig_AllowsMissingTrialSignupPriceWhenPublicCloudSignupDisabled(t *testing.T) {
	setRequiredCPEnv(t)
	setTrialSigningEnv(t)
	t.Setenv("STRIPE_API_KEY", "sk_test_123")
	t.Setenv("CP_TRIAL_SIGNUP_PRICE_ID", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.PublicCloudSignupEnabled {
		t.Fatal("PublicCloudSignupEnabled = true, want false by default")
	}
}

func setProviderHostedMSPEnv(t *testing.T) {
	t.Helper()
	t.Setenv("CP_ADMIN_KEY", "test-key")
	t.Setenv("CP_BASE_URL", "https://msp.example.com")
	t.Setenv("CP_ENV", "development")
	t.Setenv("CP_CONTROL_PLANE_MODE", "provider_hosted_msp")
	t.Setenv("CP_REQUIRE_EMAIL_PROVIDER", "false")
	t.Setenv("STRIPE_WEBHOOK_SECRET", "")
	t.Setenv("STRIPE_API_KEY", "")
	t.Setenv("CP_PUBLIC_CLOUD_SIGNUP_ENABLED", "false")
	t.Setenv("CP_MSP_STARTER_PRICE_ID", "")
	t.Setenv("CP_MSP_GROWTH_PRICE_ID", "")
	t.Setenv("CP_MSP_SCALE_PRICE_ID", "")
	setTrialSigningEnv(t)
}

func setPulseHostedMSPEnv(t *testing.T) {
	t.Helper()
	setProviderHostedMSPEnv(t)
	t.Setenv("CP_CONTROL_PLANE_MODE", "pulse_hosted_msp")
}

func writeProviderMSPLicenseForTest(t *testing.T, tier pkglicensing.Tier, planVersion string) string {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	t.Setenv("PULSE_LICENSE_PUBLIC_KEY", base64.StdEncoding.EncodeToString(publicKey))
	t.Setenv("PULSE_LICENSE_DEV_MODE", "false")
	t.Cleanup(func() { pkglicensing.SetPublicKey(nil) })

	claims := pkglicensing.Claims{
		LicenseID:   "lic_provider_msp_test",
		Email:       "provider@example.com",
		Tier:        tier,
		IssuedAt:    time.Now().Add(-time.Minute).Unix(),
		ExpiresAt:   time.Now().Add(24 * time.Hour).Unix(),
		PlanVersion: planVersion,
	}
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"EdDSA","typ":"JWT"}`))
	payloadBytes, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("Marshal claims: %v", err)
	}
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	signedData := []byte(header + "." + payload)
	signature := base64.RawURLEncoding.EncodeToString(ed25519.Sign(privateKey, signedData))
	licenseKey := header + "." + payload + "." + signature

	path := t.TempDir() + "/provider-msp-license.jwt"
	if err := os.WriteFile(path, []byte(licenseKey+"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

func TestLoadConfig_ProviderHostedMSPDoesNotRequireStripe(t *testing.T) {
	setProviderHostedMSPEnv(t)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.ControlPlaneMode != ControlPlaneModeProviderHostedMSP {
		t.Fatalf("ControlPlaneMode = %q, want %q", cfg.ControlPlaneMode, ControlPlaneModeProviderHostedMSP)
	}
	if cfg.UsesStripeBilling() {
		t.Fatal("UsesStripeBilling = true, want false")
	}
	if cfg.ProviderMSPPlanVersion != "msp_starter" {
		t.Fatalf("ProviderMSPPlanVersion = %q, want msp_starter", cfg.ProviderMSPPlanVersion)
	}
	if cfg.ProviderMSPPlanSource != ProviderMSPPlanSourceEnvFallback {
		t.Fatalf("ProviderMSPPlanSource = %q, want %q", cfg.ProviderMSPPlanSource, ProviderMSPPlanSourceEnvFallback)
	}
}

func TestLoadConfig_PulseHostedMSPUsesStripeFreeMSPStack(t *testing.T) {
	setPulseHostedMSPEnv(t)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.ControlPlaneMode != ControlPlaneModePulseHostedMSP {
		t.Fatalf("ControlPlaneMode = %q, want %q", cfg.ControlPlaneMode, ControlPlaneModePulseHostedMSP)
	}
	if !cfg.IsMSPControlPlane() {
		t.Fatal("IsMSPControlPlane = false, want true")
	}
	if cfg.IsProviderHostedMSP() {
		t.Fatal("IsProviderHostedMSP = true, want false for Pulse-hosted MSP")
	}
	if cfg.UsesStripeBilling() {
		t.Fatal("UsesStripeBilling = true, want false")
	}
	if cfg.ProviderMSPPlanVersion != "msp_starter" {
		t.Fatalf("ProviderMSPPlanVersion = %q, want msp_starter", cfg.ProviderMSPPlanVersion)
	}
}

func TestLoadConfig_PulseHostedMSPUsesSignedLicenseFilePlan(t *testing.T) {
	setPulseHostedMSPEnv(t)
	t.Setenv("CP_ENV", "production")
	t.Setenv("CP_PROVIDER_MSP_PLAN_VERSION", "msp_starter")
	t.Setenv("CP_PROVIDER_MSP_LICENSE_FILE", writeProviderMSPLicenseForTest(t, pkglicensing.TierMSP, "msp_scale"))

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.ProviderMSPPlanVersion != "msp_scale" {
		t.Fatalf("ProviderMSPPlanVersion = %q, want msp_scale", cfg.ProviderMSPPlanVersion)
	}
	if cfg.ProviderMSPPlanSource != ProviderMSPPlanSourceLicenseFile {
		t.Fatalf("ProviderMSPPlanSource = %q, want %q", cfg.ProviderMSPPlanSource, ProviderMSPPlanSourceLicenseFile)
	}
}

func TestNormalizeControlPlaneMode_PulseHostedMSPAliases(t *testing.T) {
	for _, raw := range []string{"pulse_hosted_msp", "pulse-hosted-msp", "pulse_msp", "pulse-msp", "hosted_msp", "hosted-msp"} {
		t.Run(raw, func(t *testing.T) {
			if got := normalizeControlPlaneMode(raw); got != ControlPlaneModePulseHostedMSP {
				t.Fatalf("normalizeControlPlaneMode(%q) = %q, want %q", raw, got, ControlPlaneModePulseHostedMSP)
			}
		})
	}
}

func TestLoadConfig_ProviderHostedMSPReportBrandDefaults(t *testing.T) {
	setProviderHostedMSPEnv(t)
	t.Setenv("CP_REPORT_BRAND_DISPLAY_NAME", "Provider Default")
	t.Setenv("CP_REPORT_BRAND_LOGO_PATH", "/etc/pulse/brand.png")
	t.Setenv("CP_REPORT_BRAND_LOGO_BASE64", "iVBORw0KGgo=")
	t.Setenv("CP_REPORT_BRAND_LOGO_FORMAT", "png")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.ReportBrandDisplayName != "Provider Default" {
		t.Fatalf("ReportBrandDisplayName = %q, want Provider Default", cfg.ReportBrandDisplayName)
	}
	if cfg.ReportBrandLogoPath != "/etc/pulse/brand.png" {
		t.Fatalf("ReportBrandLogoPath = %q", cfg.ReportBrandLogoPath)
	}
	if cfg.ReportBrandLogoBase64 != "iVBORw0KGgo=" {
		t.Fatalf("ReportBrandLogoBase64 = %q", cfg.ReportBrandLogoBase64)
	}
	if cfg.ReportBrandLogoFormat != "png" {
		t.Fatalf("ReportBrandLogoFormat = %q, want png", cfg.ReportBrandLogoFormat)
	}
}

func TestLoadConfig_ReportBrandDefaultsRejectInvalidLogoFormat(t *testing.T) {
	setProviderHostedMSPEnv(t)
	t.Setenv("CP_REPORT_BRAND_LOGO_FORMAT", "svg")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected invalid report brand logo format to fail config validation")
	}
	if !strings.Contains(err.Error(), "CP_REPORT_BRAND_LOGO_FORMAT must be png, jpg, jpeg, gif, or empty") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadConfig_ProviderHostedMSPRejectsStripeConfig(t *testing.T) {
	for _, tc := range []struct {
		name string
		key  string
		val  string
	}{
		{name: "webhook_secret", key: "STRIPE_WEBHOOK_SECRET", val: "whsec_test"},
		{name: "api_key", key: "STRIPE_API_KEY", val: "sk_test_123"},
		{name: "msp_price", key: "CP_MSP_STARTER_PRICE_ID", val: "price_1T5kgTBrHBocJIGHjOs15LI2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setProviderHostedMSPEnv(t)
			t.Setenv(tc.key, tc.val)

			_, err := LoadConfig()
			if err == nil {
				t.Fatalf("expected error when %s is configured", tc.key)
			}
			if !strings.Contains(err.Error(), "must not be configured") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestLoadConfig_PulseHostedMSPRejectsStripeConfig(t *testing.T) {
	setPulseHostedMSPEnv(t)
	t.Setenv("STRIPE_API_KEY", "sk_test_123")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error when STRIPE_API_KEY is configured")
	}
	if !strings.Contains(err.Error(), "must not be configured") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadConfig_ProviderHostedMSPRejectsPublicSignup(t *testing.T) {
	setProviderHostedMSPEnv(t)
	t.Setenv("CP_PUBLIC_CLOUD_SIGNUP_ENABLED", "true")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error when public signup is enabled in provider-hosted MSP mode")
	}
	if !strings.Contains(err.Error(), "CP_PUBLIC_CLOUD_SIGNUP_ENABLED must be false") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadConfig_ProviderHostedMSPRejectsInvalidPlan(t *testing.T) {
	for _, tc := range []struct {
		name string
		plan string
		want string
	}{
		{name: "non_msp", plan: "cloud_starter", want: "canonical MSP plan"},
		{name: "unknown_msp", plan: "msp_unknown", want: "known workspace limit"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setProviderHostedMSPEnv(t)
			t.Setenv("CP_PROVIDER_MSP_PLAN_VERSION", tc.plan)

			_, err := LoadConfig()
			if err == nil {
				t.Fatalf("expected error for CP_PROVIDER_MSP_PLAN_VERSION=%q", tc.plan)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestLoadConfig_ProviderHostedMSPUsesSignedLicenseFilePlan(t *testing.T) {
	setProviderHostedMSPEnv(t)
	t.Setenv("CP_ENV", "production")
	t.Setenv("CP_PROVIDER_MSP_PLAN_VERSION", "msp_starter")
	t.Setenv("CP_PROVIDER_MSP_LICENSE_FILE", writeProviderMSPLicenseForTest(t, pkglicensing.TierMSP, "msp_growth"))

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.ProviderMSPPlanVersion != "msp_growth" {
		t.Fatalf("ProviderMSPPlanVersion = %q, want msp_growth", cfg.ProviderMSPPlanVersion)
	}
	if cfg.ProviderMSPPlanSource != ProviderMSPPlanSourceLicenseFile {
		t.Fatalf("ProviderMSPPlanSource = %q, want %q", cfg.ProviderMSPPlanSource, ProviderMSPPlanSourceLicenseFile)
	}
	if cfg.ProviderMSPLicenseID != "lic_provider_msp_test" {
		t.Fatalf("ProviderMSPLicenseID = %q", cfg.ProviderMSPLicenseID)
	}
	if cfg.ProviderMSPLicenseEmail != "provider@example.com" {
		t.Fatalf("ProviderMSPLicenseEmail = %q", cfg.ProviderMSPLicenseEmail)
	}
}

func TestLoadConfig_ProviderHostedMSPRequiresLicenseFileInProduction(t *testing.T) {
	setProviderHostedMSPEnv(t)
	t.Setenv("CP_ENV", "production")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error without CP_PROVIDER_MSP_LICENSE_FILE in production")
	}
	if !strings.Contains(err.Error(), "CP_PROVIDER_MSP_LICENSE_FILE is required in production") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadConfig_ProviderHostedMSPRejectsNonMSPLicenseFile(t *testing.T) {
	setProviderHostedMSPEnv(t)
	t.Setenv("CP_PROVIDER_MSP_LICENSE_FILE", writeProviderMSPLicenseForTest(t, pkglicensing.TierCloud, "cloud_starter"))

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error for non-MSP provider license")
	}
	if !strings.Contains(err.Error(), "must contain an MSP license tier") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadConfig_RequiresTrialSignupPriceWhenPublicCloudSignupEnabled(t *testing.T) {
	setRequiredCPEnv(t)
	setTrialSigningEnv(t)
	t.Setenv("STRIPE_API_KEY", "sk_test_123")
	t.Setenv("CP_PUBLIC_CLOUD_SIGNUP_ENABLED", "true")
	t.Setenv("CP_TRIAL_SIGNUP_PRICE_ID", "")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error when public cloud signup is enabled but CP_TRIAL_SIGNUP_PRICE_ID is missing")
	}
	if !strings.Contains(err.Error(), "CP_TRIAL_SIGNUP_PRICE_ID is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadConfig_RequiresLiveStripeKeyInProduction(t *testing.T) {
	setRequiredCPEnv(t)
	t.Setenv("CP_ENV", "production")
	t.Setenv("CP_REQUIRE_EMAIL_PROVIDER", "false")
	t.Setenv("STRIPE_API_KEY", "sk_test_123")
	t.Setenv("CP_TRIAL_SIGNUP_PRICE_ID", "price_1T5kflBrHBocJIGHUqPv1dzV")
	setTrialSigningEnv(t)

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error when CP_ENV=production and STRIPE_API_KEY is test mode")
	}
	if !strings.Contains(err.Error(), "must be a live key") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadConfig_RequiresTestStripeKeyInStaging(t *testing.T) {
	setRequiredCPEnv(t)
	t.Setenv("CP_ENV", "staging")
	t.Setenv("CP_REQUIRE_EMAIL_PROVIDER", "false")
	t.Setenv("STRIPE_API_KEY", "sk_live_123")
	t.Setenv("CP_TRIAL_SIGNUP_PRICE_ID", "price_1T5kflBrHBocJIGHUqPv1dzV")
	setTrialSigningEnv(t)

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error when CP_ENV=staging and STRIPE_API_KEY is live mode")
	}
	if !strings.Contains(err.Error(), "must be a test key") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadConfig_RejectsDockerlessProvisioningInProduction(t *testing.T) {
	setRequiredCPEnv(t)
	t.Setenv("CP_ENV", "production")
	t.Setenv("CP_ALLOW_DOCKERLESS_PROVISIONING", "true")
	t.Setenv("CP_REQUIRE_EMAIL_PROVIDER", "true")
	t.Setenv("RESEND_API_KEY", "re_test_key")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error when dockerless provisioning is enabled in production")
	}
	if !strings.Contains(err.Error(), "CP_ALLOW_DOCKERLESS_PROVISIONING must be false in production") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTenantsDir(t *testing.T) {
	cfg := &CPConfig{DataDir: "/data"}
	if got := cfg.TenantsDir(); got != "/data/tenants" {
		t.Errorf("TenantsDir = %q, want /data/tenants", got)
	}
}

func TestControlPlaneDir(t *testing.T) {
	cfg := &CPConfig{DataDir: "/data"}
	if got := cfg.ControlPlaneDir(); got != "/data/control-plane" {
		t.Errorf("ControlPlaneDir = %q, want /data/control-plane", got)
	}
}

func TestEnvOrDefaultInt64(t *testing.T) {
	const key = "TEST_CP_ENV_OR_DEFAULT_INT64"
	t.Setenv(key, "")
	if got, _ := envOrDefaultInt64(key, 99); got != 99 {
		t.Fatalf("envOrDefaultInt64 unset = %d, want 99", got)
	}

	t.Setenv(key, " 1234 ")
	if got, _ := envOrDefaultInt64(key, 99); got != 1234 {
		t.Fatalf("envOrDefaultInt64 valid = %d, want 1234", got)
	}

	t.Setenv(key, "not-an-int")
	if got, _ := envOrDefaultInt64(key, 99); got != 99 {
		t.Fatalf("envOrDefaultInt64 invalid = %d, want 99", got)
	}
}

func TestStripeSecretKeyMode(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want string
	}{
		{name: "test", key: "sk_test_123", want: "test"},
		{name: "live", key: "sk_live_123", want: "live"},
		{name: "unknown", key: "abc", want: "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stripeSecretKeyMode(tt.key); got != tt.want {
				t.Fatalf("stripeSecretKeyMode(%q) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}
