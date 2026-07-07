package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

func TestProviderMSPCommandExposesProof(t *testing.T) {
	cmd := newProviderMSPCmd()
	for _, child := range cmd.Commands() {
		if child.Name() == "proof" {
			return
		}
	}
	t.Fatal("provider-msp proof command is not registered")
}

func TestNormalizeProviderMSPProofOptionsRequiresIsolationWorkspaceCount(t *testing.T) {
	_, err := normalizeProviderMSPProofOptions(providerMSPProofOptions{
		AccountName:     "Acme MSP",
		OwnerEmail:      "owner@example.com",
		WorkspacePrefix: "Provider MSP Proof",
		WorkspaceCount:  1,
		InstallType:     "pve",
		TargetPath:      "/settings/infrastructure?add=linux-host",
	})
	if err == nil || !strings.Contains(err.Error(), "at least 2") {
		t.Fatalf("expected isolation workspace-count error, got %v", err)
	}
}

func TestProviderMSPProofRequiresLicenseBackedPlanSourceByDefault(t *testing.T) {
	cfg := testProviderMSPProofConfig(t)
	cfg.ProviderMSPPlanSource = cloudcp.ProviderMSPPlanSourceEnvFallback

	rt, err := newProviderMSPProofRuntimeFromConfig(cfg)
	if err != nil {
		t.Fatalf("newProviderMSPProofRuntimeFromConfig: %v", err)
	}
	defer rt.close()

	_, err = rt.runProviderMSPProof(context.Background(), providerMSPProofOptions{
		AccountName:     "Acme Provider",
		OwnerEmail:      "owner@example.com",
		WorkspacePrefix: "Provider MSP Proof",
		WorkspaceCount:  2,
		InstallType:     "pve",
		TargetPath:      "/settings/infrastructure?add=linux-host",
	})
	if err == nil {
		t.Fatal("expected provider MSP proof to reject environment-fallback plan source")
	}
	if !strings.Contains(err.Error(), cloudcp.ProviderMSPPlanSourceLicenseFile) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProviderMSPProofRuntimeAcceptsPulseHostedMSPMode(t *testing.T) {
	t.Setenv("DOCKER_HOST", "unix:///tmp/pulse-provider-msp-proof-missing-docker.sock")
	t.Setenv("DOCKER_TLS_VERIFY", "")
	t.Setenv("DOCKER_CERT_PATH", "")

	cfg := testProviderMSPProofConfig(t)
	cfg.ControlPlaneMode = cloudcp.ControlPlaneModePulseHostedMSP
	cfg.BaseURL = "https://acme.msp.pulserelay.pro"

	rt, err := newProviderMSPProofRuntimeFromConfig(cfg)
	if err != nil {
		t.Fatalf("newProviderMSPProofRuntimeFromConfig: %v", err)
	}
	defer rt.close()
}

func TestProviderMSPProofExercisesWorkspaceInstallHandoffAndIsolation(t *testing.T) {
	t.Setenv("DOCKER_HOST", "unix:///tmp/pulse-provider-msp-proof-missing-docker.sock")
	t.Setenv("DOCKER_TLS_VERIFY", "")
	t.Setenv("DOCKER_CERT_PATH", "")

	cfg := testProviderMSPProofConfig(t)
	rt, err := newProviderMSPProofRuntimeFromConfig(cfg)
	if err != nil {
		t.Fatalf("newProviderMSPProofRuntimeFromConfig: %v", err)
	}
	defer rt.close()

	report, err := rt.runProviderMSPProof(context.Background(), providerMSPProofOptions{
		AccountName:     "Acme Provider",
		OwnerEmail:      "Owner@Example.com",
		WorkspacePrefix: "Provider MSP Proof",
		WorkspaceCount:  2,
		InstallType:     "pbs",
		TargetPath:      "/settings/infrastructure?add=linux-host",
	})
	if err != nil {
		t.Fatalf("runProviderMSPProof: %v", err)
	}

	if report.AccountID == "" || report.OwnerUserID == "" {
		t.Fatalf("bootstrap identity incomplete: %#v", report)
	}
	if report.OwnerEmail != "owner@example.com" {
		t.Fatalf("OwnerEmail = %q, want owner@example.com", report.OwnerEmail)
	}
	if report.PlanVersion != "msp_growth" || report.WorkspaceLimit != 15 {
		t.Fatalf("plan proof = %q limit=%d, want msp_growth limit 15", report.PlanVersion, report.WorkspaceLimit)
	}
	if report.PlanSource != cloudcp.ProviderMSPPlanSourceLicenseFile {
		t.Fatalf("PlanSource = %q, want %q", report.PlanSource, cloudcp.ProviderMSPPlanSourceLicenseFile)
	}
	if report.WorkspaceCount != 2 || len(report.Workspaces) != 2 {
		t.Fatalf("WorkspaceCount = %d len=%d, want 2", report.WorkspaceCount, len(report.Workspaces))
	}
	if !report.DockerlessProvisioning {
		t.Fatal("expected dockerless provisioning in missing-docker test mode")
	}
	if report.RuntimeContainerVerified {
		t.Fatal("runtime container should not be verified in dockerless test mode")
	}
	if !report.HandoffExchangeVerified {
		t.Fatal("handoff exchange was not verified")
	}
	if !report.InstallTokenBoundaryOK {
		t.Fatal("install token boundary was not verified")
	}
	if !report.SetupFactsTokenUseVisible {
		t.Fatal("setup facts did not see hosted root token use")
	}
	if !report.AgentReportIngestVerified {
		t.Fatal("agent report ingest was not verified")
	}
	if !report.TokenRotationVerified {
		t.Fatal("token rotation was not verified")
	}
	if !report.ReportScheduleVisible {
		t.Fatal("report schedule portal fact was not verified")
	}
	if !report.ActiveAlertRollupVisible {
		t.Fatal("active alert portal rollup was not verified")
	}

	seenTenants := map[string]struct{}{}
	for _, workspace := range report.Workspaces {
		if workspace.TenantID == "" {
			t.Fatalf("workspace missing tenant id: %#v", workspace)
		}
		if _, exists := seenTenants[workspace.TenantID]; exists {
			t.Fatalf("duplicate tenant id %s", workspace.TenantID)
		}
		seenTenants[workspace.TenantID] = struct{}{}
		if workspace.InstallType != "pbs" {
			t.Fatalf("InstallType = %q, want pbs", workspace.InstallType)
		}
		if !workspace.InstallCommandGenerated || workspace.InstallTokenID == "" || workspace.InstallToken == "" {
			t.Fatalf("workspace install proof incomplete: %#v", workspace)
		}
		if !strings.Contains(workspace.PublicURL, workspace.TenantID+".msp.example.com") {
			t.Fatalf("PublicURL = %q, want tenant subdomain", workspace.PublicURL)
		}
		if !workspace.AgentTokenAuthVerified {
			t.Fatalf("agent token auth not verified for %s", workspace.TenantID)
		}
		if !workspace.SetupFactsTokenUseVisible {
			t.Fatalf("setup facts token use not visible for %s", workspace.TenantID)
		}
		if !workspace.AgentReportIngestVerified {
			t.Fatalf("agent report ingest not verified for %s", workspace.TenantID)
		}
		if workspace.AgentReportAgentID == "" {
			t.Fatalf("agent report id missing for %s", workspace.TenantID)
		}
		if workspace.AgentReportHostname != "pve1" {
			t.Fatalf("AgentReportHostname = %q, want pve1", workspace.AgentReportHostname)
		}
		if !workspace.TokenRotationVerified {
			t.Fatalf("token rotation not verified for %s", workspace.TenantID)
		}
		if workspace.RotatedInstallTokenID == "" || workspace.RotatedInstallTokenID == workspace.InstallTokenID {
			t.Fatalf("rotated token id = %q, original = %q", workspace.RotatedInstallTokenID, workspace.InstallTokenID)
		}
		if workspace.RotatedInstallToken == "" || workspace.RotatedInstallToken == workspace.InstallToken {
			t.Fatalf("rotated raw token was not replaced for %s", workspace.TenantID)
		}
		if !workspace.OldInstallTokenRejected {
			t.Fatalf("old install token was not rejected for %s", workspace.TenantID)
		}
		if !workspace.RotatedAgentReportVerified {
			t.Fatalf("rotated install token did not report for %s", workspace.TenantID)
		}
		if !workspace.HandoffExchangeVerified || workspace.HandoffTargetPath != "/settings/infrastructure?add=linux-host" {
			t.Fatalf("handoff exchange proof mismatch: %#v", workspace)
		}
		if !workspace.EntitlementLeaseChecked {
			t.Fatalf("entitlement lease was not checked for %s despite a provider MSP license being configured", workspace.TenantID)
		}
		if !workspace.EntitlementLeaseVerified {
			t.Fatalf("entitlement lease did not chain-verify for %s", workspace.TenantID)
		}
		if !workspace.EntitlementWhiteLabel {
			t.Fatalf("entitlement lease for %s is missing white_label", workspace.TenantID)
		}
		if !workspace.ReportScheduleCreated || workspace.ReportScheduleID == "" || !workspace.ReportScheduleVisible {
			t.Fatalf("report schedule portal proof incomplete for %s: %#v", workspace.TenantID, workspace)
		}
		if workspace.ReportScheduleCount != 1 || workspace.DisabledReportScheduleCount != 0 {
			t.Fatalf("report schedule facts for %s = enabled:%d disabled:%d, want enabled:1 disabled:0", workspace.TenantID, workspace.ReportScheduleCount, workspace.DisabledReportScheduleCount)
		}
		if !workspace.ActiveAlertPersisted || !workspace.ActiveAlertRollupVisible {
			t.Fatalf("active alert portal rollup proof incomplete for %s: %#v", workspace.TenantID, workspace)
		}
		if workspace.CriticalAlertCount != 1 || workspace.WarningAlertCount != 1 {
			t.Fatalf("active alert facts for %s = critical:%d warning:%d, want critical:1 warning:1", workspace.TenantID, workspace.CriticalAlertCount, workspace.WarningAlertCount)
		}
	}
}

func TestProviderMSPProofLoadsSignedLicenseFilePlan(t *testing.T) {
	t.Setenv("DOCKER_HOST", "unix:///tmp/pulse-provider-msp-proof-missing-docker.sock")
	t.Setenv("DOCKER_TLS_VERIFY", "")
	t.Setenv("DOCKER_CERT_PATH", "")

	dataDir := t.TempDir()
	licenseFile := writeProviderMSPProofLicenseForTest(t, "lic_provider_msp_proof", "provider@example.com", "msp_growth")
	setProviderMSPProofLoadConfigEnv(t, dataDir, licenseFile)

	cfg, err := cloudcp.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.ProviderMSPPlanSource != cloudcp.ProviderMSPPlanSourceLicenseFile {
		t.Fatalf("ProviderMSPPlanSource = %q, want %q", cfg.ProviderMSPPlanSource, cloudcp.ProviderMSPPlanSourceLicenseFile)
	}
	if cfg.ProviderMSPPlanVersion != "msp_growth" {
		t.Fatalf("ProviderMSPPlanVersion = %q, want msp_growth", cfg.ProviderMSPPlanVersion)
	}

	rt, err := newProviderMSPProofRuntimeFromConfig(cfg)
	if err != nil {
		t.Fatalf("newProviderMSPProofRuntimeFromConfig: %v", err)
	}
	defer rt.close()

	report, err := rt.runProviderMSPProof(context.Background(), providerMSPProofOptions{
		AccountName:     "Acme Provider",
		OwnerEmail:      "owner@example.com",
		WorkspacePrefix: "Provider MSP Proof",
		WorkspaceCount:  2,
		InstallType:     "pve",
		TargetPath:      "/settings/infrastructure?add=linux-host",
	})
	if err != nil {
		t.Fatalf("runProviderMSPProof: %v", err)
	}
	if report.PlanVersion != "msp_growth" || report.WorkspaceLimit != 15 {
		t.Fatalf("plan proof = %q limit=%d, want msp_growth limit 15", report.PlanVersion, report.WorkspaceLimit)
	}
	if report.PlanSource != cloudcp.ProviderMSPPlanSourceLicenseFile {
		t.Fatalf("PlanSource = %q, want %q", report.PlanSource, cloudcp.ProviderMSPPlanSourceLicenseFile)
	}
	if report.LicenseID != "lic_provider_msp_proof" {
		t.Fatalf("LicenseID = %q, want lic_provider_msp_proof", report.LicenseID)
	}
	if report.LicenseEmail != "provider@example.com" {
		t.Fatalf("LicenseEmail = %q, want provider@example.com", report.LicenseEmail)
	}
	if !report.AgentReportIngestVerified || !report.InstallTokenBoundaryOK || !report.TokenRotationVerified || !report.HandoffExchangeVerified || !report.ReportScheduleVisible || !report.ActiveAlertRollupVisible {
		t.Fatalf("provider MSP proof did not complete core runtime checks: %#v", report)
	}
}

// mintProviderMSPProofLicense installs a fresh license trust root via env
// (dev builds only) and returns a root-signed MSP license binding the given
// lease signing public key, mirroring what LoadConfig resolves from
// CP_PROVIDER_MSP_LICENSE_FILE in a real deployment.
func mintProviderMSPProofLicense(t *testing.T, planVersion string, leaseSigningPublicKey ed25519.PublicKey) string {
	t.Helper()
	rootPub, rootPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey root: %v", err)
	}
	t.Setenv("PULSE_LICENSE_PUBLIC_KEY", base64.StdEncoding.EncodeToString(rootPub))
	t.Setenv("PULSE_TRIAL_ACTIVATION_PUBLIC_KEY", base64.StdEncoding.EncodeToString(rootPub))
	t.Cleanup(func() { pkglicensing.SetPublicKey(nil) })
	pkglicensing.InitEmbeddedPublicKey()

	claims := pkglicensing.Claims{
		LicenseID:                   "lic_provider_msp_test",
		Email:                       "provider@example.com",
		Tier:                        pkglicensing.TierMSP,
		IssuedAt:                    time.Now().Add(-time.Minute).Unix(),
		ExpiresAt:                   time.Now().Add(24 * time.Hour).Unix(),
		PlanVersion:                 planVersion,
		EntitlementSigningPublicKey: base64.StdEncoding.EncodeToString(leaseSigningPublicKey),
	}
	payloadBytes, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("Marshal claims: %v", err)
	}
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"EdDSA","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	signature := base64.RawURLEncoding.EncodeToString(ed25519.Sign(rootPriv, []byte(header+"."+payload)))
	return header + "." + payload + "." + signature
}

func testProviderMSPProofConfig(t *testing.T) *cloudcp.CPConfig {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	licenseKey := mintProviderMSPProofLicense(t, "msp_growth", publicKey)
	return &cloudcp.CPConfig{
		DataDir:                     t.TempDir(),
		Environment:                 "development",
		ControlPlaneMode:            cloudcp.ControlPlaneModeProviderHostedMSP,
		BaseURL:                     "https://msp.example.com",
		PulseImage:                  "pulse:test",
		DockerNetwork:               "bridge",
		TenantMemoryLimit:           512 * 1024 * 1024,
		TenantCPUShares:             256,
		TenantLogMaxSize:            "10m",
		TenantLogMaxFile:            3,
		AllowDockerlessProvisioning: true,
		StorageGuardrailsEnabled:    false,
		ProviderMSPPlanVersion:      "msp_growth",
		ProviderMSPPlanSource:       cloudcp.ProviderMSPPlanSourceLicenseFile,
		ProviderMSPLicenseID:        "lic_provider_msp_test",
		ProviderMSPLicenseEmail:     "provider@example.com",
		ProviderMSPLicenseKey:       licenseKey,
		TrialActivationPrivateKey:   base64.StdEncoding.EncodeToString(privateKey),
		TrialActivationPublicKey:    base64.StdEncoding.EncodeToString(publicKey),
		EmailFrom:                   "noreply@example.com",
	}
}

func setProviderMSPProofLoadConfigEnv(t *testing.T, dataDir, licenseFile string) {
	t.Helper()
	t.Setenv("CP_ADMIN_KEY", "test-key")
	t.Setenv("CP_BASE_URL", "https://msp.example.com")
	t.Setenv("CP_DATA_DIR", dataDir)
	t.Setenv("CP_ENV", "development")
	t.Setenv("CP_CONTROL_PLANE_MODE", string(cloudcp.ControlPlaneModeProviderHostedMSP))
	t.Setenv("CP_REQUIRE_EMAIL_PROVIDER", "false")
	t.Setenv("STRIPE_WEBHOOK_SECRET", "")
	t.Setenv("STRIPE_API_KEY", "")
	t.Setenv("CP_PUBLIC_CLOUD_SIGNUP_ENABLED", "false")
	t.Setenv("CP_MSP_STARTER_PRICE_ID", "")
	t.Setenv("CP_MSP_GROWTH_PRICE_ID", "")
	t.Setenv("CP_MSP_SCALE_PRICE_ID", "")
	t.Setenv("CP_PROVIDER_MSP_PLAN_VERSION", "msp_starter")
	t.Setenv("CP_PROVIDER_MSP_LICENSE_FILE", licenseFile)
	t.Setenv("CP_PULSE_IMAGE", "pulse:test")
	t.Setenv("CP_DOCKER_NETWORK", "bridge")
	t.Setenv("CP_ALLOW_DOCKERLESS_PROVISIONING", "true")
	t.Setenv("CP_STORAGE_GUARDRAILS_ENABLED", "false")
	t.Setenv("CP_TRIAL_ACTIVATION_PRIVATE_KEY", "A8medgdNdm12GXfTXWo6+TMZ2BeHPCLg2kd0znn6ZUk=")
}

func writeProviderMSPProofLicenseForTest(t *testing.T, licenseID, email, planVersion string) string {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	t.Setenv("PULSE_LICENSE_PUBLIC_KEY", base64.StdEncoding.EncodeToString(publicKey))
	// Release binaries embed one key as both the license root and the hosted
	// entitlement verification root; mirror that so the proof's lease
	// verification can resolve a root in dev test builds.
	t.Setenv("PULSE_TRIAL_ACTIVATION_PUBLIC_KEY", base64.StdEncoding.EncodeToString(publicKey))
	t.Setenv("PULSE_LICENSE_DEV_MODE", "false")
	t.Cleanup(func() { pkglicensing.SetPublicKey(nil) })

	// Bind the lease signing key for the fixed CP_TRIAL_ACTIVATION_PRIVATE_KEY
	// seed installed by setProviderMSPProofLoadConfigEnv; provider-hosted MSP
	// config loading requires the license to bind the control plane's key.
	leaseSigningSeed, err := base64.StdEncoding.DecodeString("A8medgdNdm12GXfTXWo6+TMZ2BeHPCLg2kd0znn6ZUk=")
	if err != nil {
		t.Fatalf("decode lease signing seed: %v", err)
	}
	leaseSigningPublicKey := ed25519.NewKeyFromSeed(leaseSigningSeed).Public().(ed25519.PublicKey)

	claims := pkglicensing.Claims{
		LicenseID:                   licenseID,
		Email:                       email,
		Tier:                        pkglicensing.TierMSP,
		IssuedAt:                    time.Now().Add(-time.Minute).Unix(),
		ExpiresAt:                   time.Now().Add(24 * time.Hour).Unix(),
		PlanVersion:                 planVersion,
		EntitlementSigningPublicKey: base64.StdEncoding.EncodeToString(leaseSigningPublicKey),
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
