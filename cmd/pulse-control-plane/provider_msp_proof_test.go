package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp"
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
	if report.AgentReportIngestVerified {
		t.Fatal("agent report ingest should remain explicitly unverified in this control-plane proof")
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
		if !workspace.HandoffExchangeVerified || workspace.HandoffTargetPath != "/settings/infrastructure?add=linux-host" {
			t.Fatalf("handoff exchange proof mismatch: %#v", workspace)
		}
	}
}

func testProviderMSPProofConfig(t *testing.T) *cloudcp.CPConfig {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
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
		TrialActivationPrivateKey:   base64.StdEncoding.EncodeToString(privateKey),
		TrialActivationPublicKey:    base64.StdEncoding.EncodeToString(publicKey),
		EmailFrom:                   "noreply@example.com",
	}
}
