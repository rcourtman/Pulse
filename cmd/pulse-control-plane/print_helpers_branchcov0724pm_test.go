package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp"
	cpDocker "github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/docker"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
)

// This file adds branch-coverage for four stdout formatter helpers in this
// package that the prior 0723pm wave did not reach:
//
//   - printMobileProofTenant
//   - printProviderMSPInstallProofReport
//   - printProviderMSPPreflightReport
//   - printProviderMSPProofReport
//
// Every target is a pure printer over an in-memory struct, so they are
// exercised directly with no network, SSH, daemon or database. Output is
// captured with the same os.Stdout pipe harness the package already uses
// (captureStdoutForProviderMSPRecoverTest); the wrappers below are identical to
// the 0723pm ones save for the suffix, so this file is self-contained.

func capturePrint0724pm(t *testing.T, fn func()) string {
	t.Helper()
	var buf bytes.Buffer
	restore := captureStdoutForProviderMSPRecoverTest(t, &buf)
	fn()
	restore()
	return buf.String()
}

func assertContains0724pm(t *testing.T, out, want string) {
	t.Helper()
	if !strings.Contains(out, want) {
		t.Fatalf("output missing %q:\n%s", want, out)
	}
}

func assertNotContains0724pm(t *testing.T, out, want string) {
	t.Helper()
	if strings.Contains(out, want) {
		t.Fatalf("output unexpectedly contains %q:\n%s", want, out)
	}
}

func assertEmpty0724pm(t *testing.T, out string) {
	t.Helper()
	if out != "" {
		t.Fatalf("expected no output, got %q", out)
	}
}

func mustIndexBefore0724pm(t *testing.T, out, first, second string) {
	t.Helper()
	i := strings.Index(out, first)
	j := strings.Index(out, second)
	if i < 0 || j < 0 {
		t.Fatalf("missing substring for ordering check (%q=%d, %q=%d):\n%s", first, i, second, j, out)
	}
	if i >= j {
		t.Fatalf("%q (idx %d) not before %q (idx %d):\n%s", first, i, second, j, out)
	}
}

// TestBranchcov0724pmPrintMobileProofTenant covers every branch of
// printMobileProofTenant: the nil-tenant early return, the cfg-nil vs cfg-non-nil
// arms, the baseDomain-empty vs baseDomain-populated arms (which also drive the
// publicURL-empty vs publicURL-populated arms), and the URL lowercasing + scheme
// /port/path stripping performed by mobileProofBaseDomainFromURL.
func TestBranchcov0724pmPrintMobileProofTenant(t *testing.T) {
	tenant := &registry.Tenant{
		ID:          "T-Alpha",
		AccountID:   "acc-1",
		DisplayName: "Alpha Tenant",
		State:       registry.TenantStateActive,
		PlanVersion: "msp_growth",
		ContainerID: "cid-alpha",
	}

	t.Run("nil tenant prints nothing", func(t *testing.T) {
		out := capturePrint0724pm(t, func() { printMobileProofTenant(nil, nil) })
		assertEmpty0724pm(t, out)
	})

	t.Run("non-nil tenant with nil cfg omits public_url/onboarding_url", func(t *testing.T) {
		out := capturePrint0724pm(t, func() { printMobileProofTenant(nil, tenant) })
		assertContains0724pm(t, out, "tenant_id=T-Alpha\n")
		assertContains0724pm(t, out, "account_id=acc-1\n")
		assertContains0724pm(t, out, "display_name=Alpha Tenant\n")
		assertContains0724pm(t, out, "state=active\n")
		assertContains0724pm(t, out, "plan_version=msp_growth\n")
		assertContains0724pm(t, out, "container_id=cid-alpha\n")
		assertNotContains0724pm(t, out, "public_url=")
		assertNotContains0724pm(t, out, "onboarding_url=")
	})

	t.Run("non-nil cfg with empty BaseURL still omits public_url", func(t *testing.T) {
		cfg := &cloudcp.CPConfig{}
		out := capturePrint0724pm(t, func() { printMobileProofTenant(cfg, tenant) })
		assertContains0724pm(t, out, "tenant_id=T-Alpha\n")
		assertNotContains0724pm(t, out, "public_url=")
		assertNotContains0724pm(t, out, "onboarding_url=")
	})

	t.Run("cfg with BaseURL prints lowercased public_url and onboarding_url", func(t *testing.T) {
		// BaseURL carries a scheme, port and path so the stripping logic in
		// mobileProofBaseDomainFromURL is exercised; the tenant id is mixed-case
		// so the ToLower in the public URL is exercised too.
		cfg := &cloudcp.CPConfig{BaseURL: "https://MSP.Pulse.Example.com:8443/settings"}
		out := capturePrint0724pm(t, func() { printMobileProofTenant(cfg, tenant) })
		assertContains0724pm(t, out, "tenant_id=T-Alpha\n")
		// "https://" + ToLower("T-Alpha") + "." + "MSP.Pulse.Example.com" (port/path stripped).
		assertContains0724pm(t, out, "public_url=https://t-alpha.MSP.Pulse.Example.com\n")
		assertContains0724pm(t, out, "onboarding_url=https://t-alpha.MSP.Pulse.Example.com/api/onboarding/qr\n")
	})
}

// TestBranchcov0724pmPrintProviderMSPInstallProofReport covers every branch of
// printProviderMSPInstallProofReport: the nil-report early return, the zero-value
// populated report (all flags false / counters zero, empty slices -> no workspace
// or failure lines), and a populated report with a mix of pass/fail workspaces and
// multiple failures.
func TestBranchcov0724pmPrintProviderMSPInstallProofReport(t *testing.T) {
	t.Run("nil report prints ok=false and nothing else", func(t *testing.T) {
		out := capturePrint0724pm(t, func() { printProviderMSPInstallProofReport(nil) })
		assertContains0724pm(t, out, "provider_msp_install_proof_ok=false\n")
		assertNotContains0724pm(t, out, "provider_msp_install_proof_ok=true")
		assertNotContains0724pm(t, out, "account_id=")
		assertNotContains0724pm(t, out, "workspace=")
		assertNotContains0724pm(t, out, "failure=")
	})

	t.Run("zero-value report prints ok=false and zero counters, no loops", func(t *testing.T) {
		out := capturePrint0724pm(t, func() { printProviderMSPInstallProofReport(&providerMSPInstallProofReport{}) })
		assertContains0724pm(t, out, "provider_msp_install_proof_ok=false\n")
		assertContains0724pm(t, out, "account_id=\n")
		assertContains0724pm(t, out, "account_name=\n")
		assertContains0724pm(t, out, "workspace_limit=0\n")
		assertContains0724pm(t, out, "bootstrap_ok=false\n")
		assertContains0724pm(t, out, "preflight_ok=false\n")
		assertContains0724pm(t, out, "cleanup_ok=false\n")
		assertContains0724pm(t, out, "final_status_ok=false\n")
		assertContains0724pm(t, out, "backup_bytes=0\n")
		assertContains0724pm(t, out, "recovery_recover_count=0\n")
		assertContains0724pm(t, out, "workspace_count=0\n")
		assertContains0724pm(t, out, "final_status_total_tenants=0\n")
		assertContains0724pm(t, out, "rotated_out_token_rejection_verified=false\n")
		assertNotContains0724pm(t, out, "workspace=")
		assertNotContains0724pm(t, out, "failure=")
	})

	t.Run("populated ok report prints true flags, mix of workspaces and failures", func(t *testing.T) {
		report := &providerMSPInstallProofReport{
			OK:                        true,
			AccountID:                 "acc-install",
			AccountName:               "Acme Install",
			OwnerUserID:               "user-9",
			OwnerEmail:                "ops@install.example.com",
			PlanVersion:               "msp_growth",
			PlanSource:                "license-file",
			LicenseID:                 "lic-install",
			LicenseEmail:              "ops@install.example.com",
			WorkspaceLimit:            12,
			BootstrapOK:               true,
			PreflightOK:               true,
			BackupCreated:             true,
			BackupVerified:            true,
			CleanupOK:                 true,
			FinalStatusOK:             true,
			BackupPath:                "/backups/install.tar.gz",
			BackupBytes:               9999,
			RestoreTargetDataDir:      "/data/install-proof-restore",
			RecoveryRecoverCount:      3,
			RecoverySkippedCount:      1,
			WorkspaceCount:            2,
			InitialStatusTotalTenants: 5,
			FinalStatusTotalTenants:   7,
			Workspaces: []providerMSPProofWorkspace{
				{
					TenantID: "ws-pass", DisplayName: "Pass Workspace", State: "active",
					PlanVersion: "msp_growth", ContainerID: "cid-pass",
					PublicURL: "https://ws-pass.msp.example.com", InstallType: "pve",
					InstallTokenID: "tok-pass", InstallCommandGenerated: true,
					AgentTokenAuthVerified: true, EntitlementLeaseChecked: true,
					EntitlementLeaseVerified: true, ReportScheduleCreated: true,
					ReportScheduleID: "sched-pass", CriticalAlertCount: 0, WarningAlertCount: 2,
				},
				{
					TenantID: "ws-fail", DisplayName: "Fail Workspace", State: "failed",
					InstallType: "docker", OldInstallTokenRejected: false,
					CriticalAlertCount: 9,
				},
			},
			Failures: []string{"preflight storage degraded", "restore dry-run timeout"},
		}
		out := capturePrint0724pm(t, func() { printProviderMSPInstallProofReport(report) })
		assertContains0724pm(t, out, "provider_msp_install_proof_ok=true\n")
		assertContains0724pm(t, out, "account_id=acc-install\n")
		assertContains0724pm(t, out, "account_name=Acme Install\n")
		assertContains0724pm(t, out, "workspace_limit=12\n")
		assertContains0724pm(t, out, "bootstrap_ok=true\n")
		assertContains0724pm(t, out, "backup_path=/backups/install.tar.gz\n")
		assertContains0724pm(t, out, "backup_bytes=9999\n")
		assertContains0724pm(t, out, "recovery_recover_count=3\n")
		assertContains0724pm(t, out, "workspace_count=2\n")
		assertContains0724pm(t, out, "final_status_total_tenants=7\n")
		// Pass workspace (display_name is %q-quoted).
		assertContains0724pm(t, out, `workspace=ws-pass display_name="Pass Workspace"`)
		assertContains0724pm(t, out, "public_url=https://ws-pass.msp.example.com")
		assertContains0724pm(t, out, "install_type=pve")
		assertContains0724pm(t, out, "report_schedule_id=sched-pass")
		// Fail workspace.
		assertContains0724pm(t, out, `workspace=ws-fail display_name="Fail Workspace"`)
		assertContains0724pm(t, out, "state=failed")
		assertContains0724pm(t, out, "install_type=docker")
		assertContains0724pm(t, out, "critical_alert_count=9")
		// Workspaces are emitted in slice order.
		mustIndexBefore0724pm(t, out, "workspace=ws-pass", "workspace=ws-fail")
		// Both failures printed.
		assertContains0724pm(t, out, "failure=preflight storage degraded\n")
		assertContains0724pm(t, out, "failure=restore dry-run timeout\n")
	})
}

// TestBranchcov0724pmPrintProviderMSPPreflightReport covers every branch of
// printProviderMSPPreflightReport: the nil-report early return; the report with
// neither Docker nor Storage and no failures (both optional sections omitted);
// the Docker section; the Storage section with a mix of ok/fail filesystems
// (status + optional error line), ok/fail build cache (status + optional error
// line); and the failures loop.
func TestBranchcov0724pmPrintProviderMSPPreflightReport(t *testing.T) {
	t.Run("nil report prints ok=false and nothing else", func(t *testing.T) {
		out := capturePrint0724pm(t, func() { printProviderMSPPreflightReport(nil) })
		assertContains0724pm(t, out, "provider_msp_preflight_ok=false\n")
		assertNotContains0724pm(t, out, "provider_msp_preflight_ok=true")
		assertNotContains0724pm(t, out, "environment=")
		assertNotContains0724pm(t, out, "docker_reachable=")
		assertNotContains0724pm(t, out, "storage_guardrails_enabled=")
		assertNotContains0724pm(t, out, "failure=")
	})

	t.Run("report with nil Docker/Storage and no failures omits optional sections", func(t *testing.T) {
		report := &providerMSPPreflightReport{
			OK:             true,
			Environment:    "production",
			ControlMode:    string(cloudcp.ControlPlaneModeProviderHostedMSP),
			BaseURL:        "https://msp.example.com",
			PlanVersion:    "msp_growth",
			PlanSource:     "license-file",
			LicenseID:      "lic-pre",
			LicenseEmail:   "ops@pre.example.com",
			WorkspaceLimit: 10,
			RegistryReady:  true,
		}
		out := capturePrint0724pm(t, func() { printProviderMSPPreflightReport(report) })
		assertContains0724pm(t, out, "provider_msp_preflight_ok=true\n")
		assertContains0724pm(t, out, "environment=production\n")
		assertContains0724pm(t, out, "control_plane_mode="+string(cloudcp.ControlPlaneModeProviderHostedMSP)+"\n")
		assertContains0724pm(t, out, "base_url=https://msp.example.com\n")
		assertContains0724pm(t, out, "plan_version=msp_growth\n")
		assertContains0724pm(t, out, "plan_source=license-file\n")
		assertContains0724pm(t, out, "license_id=lic-pre\n")
		assertContains0724pm(t, out, "license_email=ops@pre.example.com\n")
		assertContains0724pm(t, out, "workspace_limit=10\n")
		assertContains0724pm(t, out, "registry_ready=true\n")
		assertNotContains0724pm(t, out, "docker_reachable=")
		assertNotContains0724pm(t, out, "storage_guardrails_enabled=")
		assertNotContains0724pm(t, out, "docker_build_cache_status=")
		assertNotContains0724pm(t, out, "failure=")
	})

	t.Run("Docker section is emitted when Docker is non-nil", func(t *testing.T) {
		report := &providerMSPPreflightReport{
			Docker: &cpDocker.RuntimePrerequisiteReport{
				DockerReachable:   true,
				NetworkName:       "pulse-net",
				NetworkID:         "netid-123",
				NetworkOK:         true,
				ImageRef:          "pulse:latest",
				ImageID:           "sha256:abc",
				ImageAvailable:    true,
				ImagePulled:       true,
				ImagePullRequired: false,
			},
		}
		out := capturePrint0724pm(t, func() { printProviderMSPPreflightReport(report) })
		assertContains0724pm(t, out, "docker_reachable=true\n")
		assertContains0724pm(t, out, "docker_network=pulse-net\n")
		assertContains0724pm(t, out, "docker_network_ok=true\n")
		assertContains0724pm(t, out, "docker_network_id=netid-123\n")
		assertContains0724pm(t, out, "tenant_runtime_image=pulse:latest\n")
		assertContains0724pm(t, out, "tenant_runtime_image_available=true\n")
		assertContains0724pm(t, out, "tenant_runtime_image_pulled=true\n")
		assertContains0724pm(t, out, "tenant_runtime_image_pull_required=false\n")
		assertContains0724pm(t, out, "tenant_runtime_image_id=sha256:abc\n")
	})

	t.Run("Storage section with ok filesystem and ok build cache emits status=ok and no error lines", func(t *testing.T) {
		report := &providerMSPPreflightReport{
			Storage: &cloudcp.StorageGuardrailReport{
				Enabled: true,
				OK:      true,
				Filesystems: []cloudcp.StorageFilesystemReport{
					{Name: "data", Path: "/data", AvailableBytes: 1000, MinAvailableBytes: 500, OK: true},
				},
				BuildCache: cloudcp.StorageBuildCacheReport{
					TotalBytes: 1000, MaxBytes: 5000, OK: true,
				},
			},
		}
		out := capturePrint0724pm(t, func() { printProviderMSPPreflightReport(report) })
		assertContains0724pm(t, out, "storage_guardrails_enabled=true\n")
		assertContains0724pm(t, out, "storage_guardrails_ok=true\n")
		assertContains0724pm(t, out, "storage_filesystem=data path=/data status=ok available_bytes=1000 min_available_bytes=500\n")
		assertNotContains0724pm(t, out, "storage_filesystem_error=")
		assertContains0724pm(t, out, "docker_build_cache_status=ok\n")
		assertContains0724pm(t, out, "docker_build_cache_total_bytes=1000\n")
		assertContains0724pm(t, out, "docker_build_cache_max_bytes=5000\n")
		assertNotContains0724pm(t, out, "docker_build_cache_error=")
	})

	t.Run("Storage section with fail filesystem+error and fail build cache+error emits status=fail and error lines", func(t *testing.T) {
		report := &providerMSPPreflightReport{
			Storage: &cloudcp.StorageGuardrailReport{
				Enabled: true,
				OK:      false,
				Filesystems: []cloudcp.StorageFilesystemReport{
					{Name: "docker", Path: "/var/lib/docker", AvailableBytes: 100, MinAvailableBytes: 500, OK: false, Error: "stat failed"},
				},
				BuildCache: cloudcp.StorageBuildCacheReport{
					TotalBytes: 6000, MaxBytes: 5000, OK: false, Error: "build cache over limit",
				},
			},
		}
		out := capturePrint0724pm(t, func() { printProviderMSPPreflightReport(report) })
		assertContains0724pm(t, out, "storage_guardrails_ok=false\n")
		assertContains0724pm(t, out, "storage_filesystem=docker path=/var/lib/docker status=fail available_bytes=100 min_available_bytes=500\n")
		assertContains0724pm(t, out, "storage_filesystem_error=docker path=/var/lib/docker error=stat failed\n")
		assertContains0724pm(t, out, "docker_build_cache_status=fail\n")
		assertContains0724pm(t, out, "docker_build_cache_error=build cache over limit\n")
	})

	t.Run("failures loop prints each failure", func(t *testing.T) {
		report := &providerMSPPreflightReport{
			OK:       false,
			Failures: []string{"registry unreachable", "docker daemon down"},
		}
		out := capturePrint0724pm(t, func() { printProviderMSPPreflightReport(report) })
		assertContains0724pm(t, out, "provider_msp_preflight_ok=false\n")
		assertContains0724pm(t, out, "failure=registry unreachable\n")
		assertContains0724pm(t, out, "failure=docker daemon down\n")
	})
}

// TestBranchcov0724pmPrintProviderMSPProofReport covers every branch of
// printProviderMSPProofReport: the nil-report early return, the zero-value
// populated report (which still prints ok=true because the printer hardcodes
// true for any non-nil report), and a populated report with a mix of pass/fail
// workspaces.
func TestBranchcov0724pmPrintProviderMSPProofReport(t *testing.T) {
	t.Run("nil report prints ok=false and nothing else", func(t *testing.T) {
		out := capturePrint0724pm(t, func() { printProviderMSPProofReport(nil) })
		assertContains0724pm(t, out, "provider_msp_control_plane_proof_ok=false\n")
		assertNotContains0724pm(t, out, "provider_msp_control_plane_proof_ok=true")
		assertNotContains0724pm(t, out, "account_id=")
		assertNotContains0724pm(t, out, "workspace=")
	})

	t.Run("zero-value report prints ok=true and empty fields, no workspace lines", func(t *testing.T) {
		out := capturePrint0724pm(t, func() { printProviderMSPProofReport(&providerMSPProofReport{}) })
		assertContains0724pm(t, out, "provider_msp_control_plane_proof_ok=true\n")
		assertContains0724pm(t, out, "account_id=\n")
		assertContains0724pm(t, out, "account_name=\n")
		assertContains0724pm(t, out, "workspace_limit=0\n")
		assertContains0724pm(t, out, "workspace_count=0\n")
		assertContains0724pm(t, out, "dockerless_provisioning=false\n")
		assertContains0724pm(t, out, "cleanup=false\n")
		assertNotContains0724pm(t, out, "workspace=")
	})

	t.Run("populated report prints fields and a mix of pass/fail workspaces", func(t *testing.T) {
		report := &providerMSPProofReport{
			AccountID:                "acc-proof",
			AccountName:              "Acme Proof",
			OwnerUserID:              "user-7",
			OwnerEmail:               "ops@proof.example.com",
			PlanVersion:              "msp_growth",
			PlanSource:               "license-file",
			LicenseID:                "lic-proof",
			LicenseEmail:             "ops@proof.example.com",
			WorkspaceLimit:           8,
			WorkspaceCount:           2,
			DockerlessProvisioning:   true,
			RuntimeContainerVerified: true,
			HandoffExchangeVerified:  true,
			InstallTokenBoundaryOK:   true,
			TokenRotationVerified:    true,
			ReportScheduleVisible:    true,
			ActiveAlertRollupVisible: true,
			Cleanup:                  true,
			Workspaces: []providerMSPProofWorkspace{
				{
					TenantID: "ws-ok", DisplayName: "OK Workspace", State: "active",
					PlanVersion: "msp_growth", ContainerID: "cid-ok",
					PublicURL: "https://ws-ok.msp.example.com", InstallType: "pve",
					InstallTokenID: "tok-ok", InstallCommandGenerated: true,
					AgentTokenAuthVerified: true, TokenRotationVerified: true,
					HandoffExchangeVerified: true, HandoffTargetPath: "/settings/infrastructure",
					EntitlementLeaseChecked: true, EntitlementLeaseVerified: true,
					ReportScheduleCreated: true, ReportScheduleID: "sched-ok",
					ReportScheduleVisible: true, ReportScheduleCount: 1,
					ActiveAlertPersisted: true, CriticalAlertCount: 0, WarningAlertCount: 1,
				},
				{
					TenantID: "ws-bad", DisplayName: "Bad Workspace", State: "failed",
					InstallType: "docker", EntitlementSkippedReason: "no lease",
					CriticalAlertCount: 4,
				},
			},
		}
		out := capturePrint0724pm(t, func() { printProviderMSPProofReport(report) })
		assertContains0724pm(t, out, "provider_msp_control_plane_proof_ok=true\n")
		assertContains0724pm(t, out, "account_id=acc-proof\n")
		assertContains0724pm(t, out, "account_name=Acme Proof\n")
		assertContains0724pm(t, out, "workspace_limit=8\n")
		assertContains0724pm(t, out, "workspace_count=2\n")
		assertContains0724pm(t, out, "dockerless_provisioning=true\n")
		assertContains0724pm(t, out, "cleanup=true\n")
		// Pass workspace.
		assertContains0724pm(t, out, `workspace=ws-ok display_name="OK Workspace"`)
		assertContains0724pm(t, out, "public_url=https://ws-ok.msp.example.com")
		assertContains0724pm(t, out, "install_type=pve")
		assertContains0724pm(t, out, "report_schedule_id=sched-ok")
		// Fail workspace.
		assertContains0724pm(t, out, `workspace=ws-bad display_name="Bad Workspace"`)
		assertContains0724pm(t, out, "state=failed")
		assertContains0724pm(t, out, "entitlement_skipped_reason=no lease")
		assertContains0724pm(t, out, "critical_alert_count=4")
		// Workspaces are emitted in slice order.
		mustIndexBefore0724pm(t, out, "workspace=ws-ok", "workspace=ws-bad")
	})
}
