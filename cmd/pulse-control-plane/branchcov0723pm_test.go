package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp"
	cpDocker "github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/docker"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
)

// This file adds branch-coverage for the stdout formatter helpers in this
// package. Every target is a pure printer over an in-memory struct, so they
// can be exercised directly with no network, SSH, daemon or database. Output is
// captured by reusing the package's existing os.Stdout redirect helper
// (captureStdoutForProviderMSPRecoverTest) verbatim; the thin wrappers below
// only tidy the call site and route every returned error through t.Helper.

func capturePrint0723pm(t *testing.T, fn func()) string {
	t.Helper()
	var buf bytes.Buffer
	restore := captureStdoutForProviderMSPRecoverTest(t, &buf)
	fn()
	restore()
	return buf.String()
}

func assertContains0723pm(t *testing.T, out, want string) {
	t.Helper()
	if !strings.Contains(out, want) {
		t.Fatalf("output missing %q:\n%s", want, out)
	}
}

func assertNotContains0723pm(t *testing.T, out, want string) {
	t.Helper()
	if strings.Contains(out, want) {
		t.Fatalf("output unexpectedly contains %q:\n%s", want, out)
	}
}

func mustIndexBefore0723pm(t *testing.T, out, first, second string) {
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

// TestBranchcov0723pmPrintTenantRuntimeReconcilePlan covers every branch of
// printTenantRuntimeReconcilePlan: the nil-plan early return, the empty-tenants
// path, each arm of the action switch (rollout/noop/default), nil item skipping,
// and every optional-field present/absent gate.
func TestBranchcov0723pmPrintTenantRuntimeReconcilePlan(t *testing.T) {
	t.Run("nil plan prints total=0 and no rollup counters", func(t *testing.T) {
		out := capturePrint0723pm(t, func() { printTenantRuntimeReconcilePlan(nil) })
		assertContains0723pm(t, out, "summary_total=0\n")
		assertNotContains0723pm(t, out, "summary_rollout=")
		assertNotContains0723pm(t, out, "tenant_id=")
	})

	t.Run("non-nil empty plan prints zero counters", func(t *testing.T) {
		out := capturePrint0723pm(t, func() {
			printTenantRuntimeReconcilePlan(&cloudcp.TenantRuntimeContractReconcilePlan{})
		})
		assertContains0723pm(t, out, "summary_rollout=0\nsummary_noop=0\nsummary_skip=0\nsummary_total=0\n")
		assertNotContains0723pm(t, out, "tenant_id=")
	})

	t.Run("item with all optional fields populated prints them", func(t *testing.T) {
		plan := &cloudcp.TenantRuntimeContractReconcilePlan{
			Tenants: []*cloudcp.TenantRuntimeContractReconcilePlanItem{
				{
					TenantID: "t-ROLL", Action: "rollout", Reason: "image drift",
					LiveContainerID:  "cid-roll",
					ImageRef:         "img:v2",
					LiveRouteHost:    "live.example.com",
					DesiredRouteHost: "desired.example.com",
					LivePublicURL:    "https://live.example.com",
					DesiredPublicURL: "https://desired.example.com",
				},
			},
		}
		out := capturePrint0723pm(t, func() { printTenantRuntimeReconcilePlan(plan) })
		for _, w := range []string{
			"tenant_id=t-ROLL",
			"action=rollout",
			"live_container_id=cid-roll",
			"image_ref=img:v2",
			"live_route_host=live.example.com",
			"desired_route_host=desired.example.com",
			"live_public_url=https://live.example.com",
			"desired_public_url=https://desired.example.com",
			"summary_rollout=1\n",
		} {
			assertContains0723pm(t, out, w)
		}
		assertNotContains0723pm(t, out, "summary_noop=1")
		assertNotContains0723pm(t, out, "summary_skip=1")
	})

	t.Run("item with all optional fields empty omits them", func(t *testing.T) {
		plan := &cloudcp.TenantRuntimeContractReconcilePlan{
			Tenants: []*cloudcp.TenantRuntimeContractReconcilePlanItem{
				{TenantID: "t-NOOPT", Action: "noop", Reason: "no drift"},
			},
		}
		out := capturePrint0723pm(t, func() { printTenantRuntimeReconcilePlan(plan) })
		assertContains0723pm(t, out, "tenant_id=t-NOOPT")
		assertContains0723pm(t, out, "action=noop")
		assertContains0723pm(t, out, "summary_noop=1\n")
		for _, absent := range []string{
			"live_container_id=",
			"image_ref=",
			"live_route_host=",
			"desired_route_host=",
			"live_public_url=",
			"desired_public_url=",
		} {
			assertNotContains0723pm(t, out, absent)
		}
	})

	t.Run("action switch default arm + nil item counted in total but skipped", func(t *testing.T) {
		// 4 entries: one nil (skipped before the switch and before printing),
		// one rollout, one noop, one unknown action (falls through to default ->
		// skip). summary_total uses len(plan.Tenants) which is 4, while only 3
		// reach the switch.
		plan := &cloudcp.TenantRuntimeContractReconcilePlan{
			Tenants: []*cloudcp.TenantRuntimeContractReconcilePlanItem{
				nil,
				{TenantID: "t-R", Action: "rollout", Reason: "r"},
				{TenantID: "t-N", Action: "noop", Reason: "n"},
				{TenantID: "t-S", Action: "skip", Reason: "s"},
			},
		}
		out := capturePrint0723pm(t, func() { printTenantRuntimeReconcilePlan(plan) })
		assertContains0723pm(t, out, "summary_rollout=1\nsummary_noop=1\nsummary_skip=1\nsummary_total=4\n")
		// All three non-nil tenants are printed; the nil entry never emits a block.
		assertContains0723pm(t, out, "tenant_id=t-R\n")
		assertContains0723pm(t, out, "tenant_id=t-N\n")
		assertContains0723pm(t, out, "tenant_id=t-S\n")
		assertContains0723pm(t, out, "action=skip")
	})
}

// TestBranchcov0723pmPrintTenantRuntimeImageRolloutPlan covers every branch of
// printTenantRuntimeImageRolloutPlan. It is structurally a sibling of the
// reconcile printer but carries a different optional-field set (State,
// LiveImageRef, TargetImageRef), so each gate is pinned independently.
func TestBranchcov0723pmPrintTenantRuntimeImageRolloutPlan(t *testing.T) {
	t.Run("nil plan prints total=0 and no rollup counters", func(t *testing.T) {
		out := capturePrint0723pm(t, func() { printTenantRuntimeImageRolloutPlan(nil) })
		assertContains0723pm(t, out, "summary_total=0\n")
		assertNotContains0723pm(t, out, "summary_rollout=")
		assertNotContains0723pm(t, out, "tenant_id=")
	})

	t.Run("non-nil empty plan prints zero counters", func(t *testing.T) {
		out := capturePrint0723pm(t, func() {
			printTenantRuntimeImageRolloutPlan(&cloudcp.TenantRuntimeImageRolloutPlan{})
		})
		assertContains0723pm(t, out, "summary_rollout=0\nsummary_noop=0\nsummary_skip=0\nsummary_total=0\n")
	})

	t.Run("item with all optional fields populated prints them", func(t *testing.T) {
		plan := &cloudcp.TenantRuntimeImageRolloutPlan{
			Tenants: []*cloudcp.TenantRuntimeImageRolloutPlanItem{
				{
					TenantID: "t-ROLL", Action: "rollout", Reason: "target image differs",
					State:            "active",
					LiveContainerID:  "cid-roll",
					LiveImageRef:     "img:v1",
					TargetImageRef:   "img:v2",
					LiveRouteHost:    "live.example.com",
					DesiredRouteHost: "desired.example.com",
					LivePublicURL:    "https://live.example.com",
					DesiredPublicURL: "https://desired.example.com",
				},
			},
		}
		out := capturePrint0723pm(t, func() { printTenantRuntimeImageRolloutPlan(plan) })
		for _, w := range []string{
			"tenant_id=t-ROLL",
			"tenant_state=active",
			"live_container_id=cid-roll",
			"live_image_ref=img:v1",
			"target_image_ref=img:v2",
			"live_route_host=live.example.com",
			"desired_route_host=desired.example.com",
			"live_public_url=https://live.example.com",
			"desired_public_url=https://desired.example.com",
			"summary_rollout=1\n",
		} {
			assertContains0723pm(t, out, w)
		}
	})

	t.Run("item with all optional fields empty omits them", func(t *testing.T) {
		plan := &cloudcp.TenantRuntimeImageRolloutPlan{
			Tenants: []*cloudcp.TenantRuntimeImageRolloutPlanItem{
				{TenantID: "t-NOOPT", Action: "noop", Reason: "same image"},
			},
		}
		out := capturePrint0723pm(t, func() { printTenantRuntimeImageRolloutPlan(plan) })
		assertContains0723pm(t, out, "tenant_id=t-NOOPT")
		assertContains0723pm(t, out, "summary_noop=1\n")
		for _, absent := range []string{
			"tenant_state=",
			"live_container_id=",
			"live_image_ref=",
			"target_image_ref=",
			"live_route_host=",
			"desired_route_host=",
			"live_public_url=",
			"desired_public_url=",
		} {
			assertNotContains0723pm(t, out, absent)
		}
	})

	t.Run("action switch default arm + nil item counted in total but skipped", func(t *testing.T) {
		plan := &cloudcp.TenantRuntimeImageRolloutPlan{
			Tenants: []*cloudcp.TenantRuntimeImageRolloutPlanItem{
				nil,
				{TenantID: "t-R", Action: "rollout", Reason: "r"},
				{TenantID: "t-N", Action: "noop", Reason: "n"},
				{TenantID: "t-S", Action: "pause", Reason: "s"},
			},
		}
		out := capturePrint0723pm(t, func() { printTenantRuntimeImageRolloutPlan(plan) })
		assertContains0723pm(t, out, "summary_rollout=1\nsummary_noop=1\nsummary_skip=1\nsummary_total=4\n")
		assertContains0723pm(t, out, "action=pause")
	})
}

// TestBranchcov0723pmPrintCloudAuditReport covers the audit report printer,
// which has the richest branch set: nil report, the fixed per-state count loop
// (with a nil map), the conditional docker_unavailable / storage section, the
// storage filesystem status (ok/fail + error gate), the build-cache status
// (ok/fail + error gate), the stale-proof loops, the orphan-entitlement loop,
// the unhealthy-container filter, and the failures loop.
func TestBranchcov0723pmPrintCloudAuditReport(t *testing.T) {
	t.Run("nil report prints audit_ok=false and nothing else", func(t *testing.T) {
		out := capturePrint0723pm(t, func() { printCloudAuditReport(nil) })
		assertContains0723pm(t, out, "audit_ok=false\n")
		assertNotContains0723pm(t, out, "audit_ok=true")
		assertNotContains0723pm(t, out, "tenant_total=")
		assertNotContains0723pm(t, out, "docker_managed_total=")
	})

	t.Run("minimal report prints zero counts and omits optional sections", func(t *testing.T) {
		report := &cloudcp.CloudAuditReport{OK: true}
		out := capturePrint0723pm(t, func() { printCloudAuditReport(report) })
		assertContains0723pm(t, out, "audit_ok=true\n")
		assertContains0723pm(t, out, "tenant_total=0\n")
		// The per-state loop always emits, even against a nil map (zero value).
		assertContains0723pm(t, out, "tenant_provisioning=0\n")
		assertContains0723pm(t, out, "tenant_active=0\n")
		assertContains0723pm(t, out, "tenant_failed=0\n")
		assertContains0723pm(t, out, "docker_managed_total=0\n")
		assertContains0723pm(t, out, "proof_tenant_stale_count=0\n")
		assertContains0723pm(t, out, "proof_account_stale_count=0\n")
		assertContains0723pm(t, out, "hosted_paid_orphan_entitlement_count=0\n")
		// Optional sections absent.
		assertNotContains0723pm(t, out, "docker_unavailable=")
		assertNotContains0723pm(t, out, "storage_guardrails_enabled=")
		assertNotContains0723pm(t, out, "docker_build_cache_status=")
		assertNotContains0723pm(t, out, "docker_unhealthy_container=")
		assertNotContains0723pm(t, out, "failure=")
	})

	t.Run("non-zero state counts and docker_unavailable are emitted", func(t *testing.T) {
		report := &cloudcp.CloudAuditReport{
			OK:                false,
			TenantTotal:       5,
			DockerUnavailable: "daemon unreachable",
			TenantCounts: map[registry.TenantState]int{
				registry.TenantStateActive:       3,
				registry.TenantStateProvisioning: 2,
			},
			RegistryUnhealthyActive: 1,
			DockerManagedTotal:      4,
			DockerManagedRunning:    3,
			DockerManagedUnhealthy:  1,
		}
		out := capturePrint0723pm(t, func() { printCloudAuditReport(report) })
		assertContains0723pm(t, out, "audit_ok=false\n")
		assertContains0723pm(t, out, "tenant_total=5\n")
		assertContains0723pm(t, out, "tenant_active=3\n")
		assertContains0723pm(t, out, "tenant_provisioning=2\n")
		// States absent from the map still print as zero.
		assertContains0723pm(t, out, "tenant_suspended=0\n")
		assertContains0723pm(t, out, "tenant_canceled=0\n")
		assertContains0723pm(t, out, "tenant_deleting=0\n")
		assertContains0723pm(t, out, "tenant_deleted=0\n")
		assertContains0723pm(t, out, "tenant_failed=0\n")
		assertContains0723pm(t, out, "tenant_registry_unhealthy_active=1\n")
		assertContains0723pm(t, out, "docker_managed_total=4\n")
		assertContains0723pm(t, out, "docker_managed_running=3\n")
		assertContains0723pm(t, out, "docker_managed_unhealthy=1\n")
		assertContains0723pm(t, out, "docker_unavailable=daemon unreachable\n")
	})

	t.Run("storage ok filesystem emits status=ok and no error line", func(t *testing.T) {
		report := &cloudcp.CloudAuditReport{
			Storage: &cloudcp.StorageGuardrailReport{
				Enabled: true,
				OK:      true,
				Filesystems: []cloudcp.StorageFilesystemReport{
					{
						Name: "data", Path: "/data",
						AvailableBytes: 1000, MinAvailableBytes: 500, TotalBytes: 2000,
						OK: true,
					},
				},
				BuildCache: cloudcp.StorageBuildCacheReport{
					TotalBytes: 1000, MaxBytes: 5000, ReclaimableBytes: 800, OK: true,
				},
			},
		}
		out := capturePrint0723pm(t, func() { printCloudAuditReport(report) })
		assertContains0723pm(t, out, "storage_guardrails_enabled=true\n")
		assertContains0723pm(t, out, "storage_ok=true\n")
		assertContains0723pm(t, out, "storage_data_status=ok\n")
		assertContains0723pm(t, out, "storage_data_path=/data\n")
		assertContains0723pm(t, out, "storage_data_available_bytes=1000\n")
		assertContains0723pm(t, out, "storage_data_min_available_bytes=500\n")
		assertContains0723pm(t, out, "storage_data_total_bytes=2000\n")
		assertNotContains0723pm(t, out, "storage_data_error=")
		assertContains0723pm(t, out, "docker_build_cache_status=ok\n")
		assertContains0723pm(t, out, "docker_build_cache_total_bytes=1000\n")
		assertContains0723pm(t, out, "docker_build_cache_max_bytes=5000\n")
		assertContains0723pm(t, out, "docker_build_cache_reclaimable_bytes=800\n")
		assertNotContains0723pm(t, out, "docker_build_cache_error=")
	})

	t.Run("storage fail filesystem and build-cache emit status=fail plus error lines", func(t *testing.T) {
		report := &cloudcp.CloudAuditReport{
			Storage: &cloudcp.StorageGuardrailReport{
				Enabled: true,
				OK:      false,
				Filesystems: []cloudcp.StorageFilesystemReport{
					{
						Name: "docker", Path: "/var/lib/docker",
						AvailableBytes: 100, MinAvailableBytes: 500, TotalBytes: 2000,
						OK: false, Error: "stat failed",
					},
				},
				BuildCache: cloudcp.StorageBuildCacheReport{
					TotalBytes: 6000, MaxBytes: 5000, ReclaimableBytes: 800,
					OK: false, Error: "build cache over limit",
				},
			},
		}
		out := capturePrint0723pm(t, func() { printCloudAuditReport(report) })
		assertContains0723pm(t, out, "storage_ok=false\n")
		assertContains0723pm(t, out, "storage_docker_status=fail\n")
		assertContains0723pm(t, out, "storage_docker_error=stat failed\n")
		assertContains0723pm(t, out, "docker_build_cache_status=fail\n")
		assertContains0723pm(t, out, "docker_build_cache_error=build cache over limit\n")
	})

	t.Run("stale proof tenants, accounts, orphans and failures are printed", func(t *testing.T) {
		report := &cloudcp.CloudAuditReport{
			OK: false,
			StaleProofTenants: []cloudcp.ProofTenantAuditItem{
				{
					TenantID: "t-STALE", State: registry.TenantStateProvisioning,
					AccountID: "acc-1", Email: "canary@example.com",
					Age: 90 * time.Second,
				},
			},
			StaleProofAccounts: []cloudcp.ProofAccountAuditItem{
				{
					AccountID: "acc-STALE", Kind: registry.AccountKindMSP,
					Age: 120 * time.Second,
				},
			},
			OrphanPaidHostedEntitlements: []cloudcp.HostedEntitlementAuditItem{
				{EntitlementID: "ent-1", TenantID: "t-MISSING", Kind: registry.HostedEntitlementKindPaid},
			},
			Failures: []string{"audit failed"},
		}
		out := capturePrint0723pm(t, func() { printCloudAuditReport(report) })
		assertContains0723pm(t, out, "proof_tenant_stale_count=1\n")
		assertContains0723pm(t, out, "proof_tenant_stale=t-STALE state=provisioning account_id=acc-1 email=canary@example.com age=1m30s\n")
		assertContains0723pm(t, out, "proof_account_stale_count=1\n")
		assertContains0723pm(t, out, "proof_account_stale=acc-STALE kind="+string(registry.AccountKindMSP)+" age=2m0s\n")
		assertContains0723pm(t, out, "hosted_paid_orphan_entitlement_count=1\n")
		assertContains0723pm(t, out, "hosted_paid_orphan_entitlement=ent-1 tenant_id=t-MISSING kind="+string(registry.HostedEntitlementKindPaid)+"\n")
		assertContains0723pm(t, out, "failure=audit failed\n")
	})

	t.Run("unhealthy container filter skips healthy/none/empty and prints the rest", func(t *testing.T) {
		// running+healthy, running+none and running+empty-health are all skipped;
		// running+unhealthy and any non-running state are printed.
		report := &cloudcp.CloudAuditReport{
			ManagedRuntimeContainers: []cpDocker.RuntimeContainerSummary{
				{ID: "c-HEALTHY", Name: "n1", State: "running", HealthStatus: "healthy", Status: "Up"},
				{ID: "c-NONE", Name: "n2", State: "running", HealthStatus: "none", Status: "Up"},
				{ID: "c-EMPTY", Name: "n3", State: "running", HealthStatus: "", Status: "Up"},
				{ID: "c-UNHEALTHY", Name: "n4", State: "running", HealthStatus: "unhealthy", Status: "Up"},
				{ID: "c-EXITED", Name: "n5", State: "exited", HealthStatus: "healthy", Status: "Exited"},
			},
		}
		out := capturePrint0723pm(t, func() { printCloudAuditReport(report) })
		assertContains0723pm(t, out, "docker_unhealthy_container=c-UNHEALTHY name=n4 state=running health=unhealthy status=Up\n")
		assertContains0723pm(t, out, "docker_unhealthy_container=c-EXITED name=n5 state=exited health=healthy status=Exited\n")
		assertNotContains0723pm(t, out, "docker_unhealthy_container=c-HEALTHY")
		assertNotContains0723pm(t, out, "docker_unhealthy_container=c-NONE")
		assertNotContains0723pm(t, out, "docker_unhealthy_container=c-EMPTY")
	})
}

// TestBranchcov0723pmPrintProviderMSPPortalLinkResult covers the portal-link
// printer. Its only branch is the nil guard; the populated arm is unconditional.
func TestBranchcov0723pmPrintProviderMSPPortalLinkResult(t *testing.T) {
	t.Run("nil result prints ok=false and no fields", func(t *testing.T) {
		out := capturePrint0723pm(t, func() { printProviderMSPPortalLinkResult(nil) })
		assertContains0723pm(t, out, "provider_msp_portal_link_ok=false\n")
		assertNotContains0723pm(t, out, "provider_msp_portal_link_ok=true")
		assertNotContains0723pm(t, out, "email=")
	})

	t.Run("populated result prints ok=true and all fields", func(t *testing.T) {
		result := &cloudcp.ProviderMSPPortalLinkResult{
			Email:        "ops@example.com",
			AccessState:  "active",
			Role:         "owner",
			MagicLinkURL: "https://msp.example.com/magic/abc",
		}
		out := capturePrint0723pm(t, func() { printProviderMSPPortalLinkResult(result) })
		assertContains0723pm(t, out, "provider_msp_portal_link_ok=true\n")
		assertContains0723pm(t, out, "email=ops@example.com\n")
		assertContains0723pm(t, out, "access_state=active\n")
		assertContains0723pm(t, out, "role=owner\n")
		assertContains0723pm(t, out, "portal_magic_link=https://msp.example.com/magic/abc\n")
	})
}

// TestBranchcov0723pmPrintProviderMSPBootstrapResult covers the bootstrap
// printer: nil guard plus the conditional MagicLinkURL gate.
func TestBranchcov0723pmPrintProviderMSPBootstrapResult(t *testing.T) {
	t.Run("nil result prints ok=false and no fields", func(t *testing.T) {
		out := capturePrint0723pm(t, func() { printProviderMSPBootstrapResult(nil) })
		assertContains0723pm(t, out, "provider_msp_bootstrap_ok=false\n")
		assertNotContains0723pm(t, out, "provider_msp_bootstrap_ok=true")
		assertNotContains0723pm(t, out, "account_id=")
	})

	t.Run("populated result without magic link omits portal_magic_link", func(t *testing.T) {
		result := &cloudcp.ProviderMSPBootstrapResult{
			AccountID: "acc-1", AccountName: "Acme MSP",
			OwnerUserID: "user-1", OwnerEmail: "ops@example.com",
			PlanVersion: "msp_growth", PlanSource: "license-file",
			LicenseID: "lic-1", LicenseEmail: "ops@example.com",
			WorkspaceLimit: 15,
		}
		out := capturePrint0723pm(t, func() { printProviderMSPBootstrapResult(result) })
		assertContains0723pm(t, out, "provider_msp_bootstrap_ok=true\n")
		assertContains0723pm(t, out, "account_id=acc-1\n")
		assertContains0723pm(t, out, "account_name=Acme MSP\n")
		assertContains0723pm(t, out, "owner_user_id=user-1\n")
		assertContains0723pm(t, out, "owner_email=ops@example.com\n")
		assertContains0723pm(t, out, "plan_version=msp_growth\n")
		assertContains0723pm(t, out, "plan_source=license-file\n")
		assertContains0723pm(t, out, "license_id=lic-1\n")
		assertContains0723pm(t, out, "license_email=ops@example.com\n")
		assertContains0723pm(t, out, "workspace_limit=15\n")
		assertNotContains0723pm(t, out, "portal_magic_link=")
	})

	t.Run("populated result with magic link prints it", func(t *testing.T) {
		result := &cloudcp.ProviderMSPBootstrapResult{
			AccountID: "acc-1", MagicLinkURL: "https://msp.example.com/magic/xyz",
		}
		out := capturePrint0723pm(t, func() { printProviderMSPBootstrapResult(result) })
		assertContains0723pm(t, out, "portal_magic_link=https://msp.example.com/magic/xyz\n")
	})
}

// TestBranchcov0723pmPrintProviderMSPBackupCreateResult covers the create
// printer: nil guard plus the populated arm that delegates to the manifest
// printer (verified by a single manifest line) and emits entry counts.
func TestBranchcov0723pmPrintProviderMSPBackupCreateResult(t *testing.T) {
	t.Run("nil result prints created=false", func(t *testing.T) {
		out := capturePrint0723pm(t, func() { printProviderMSPBackupCreateResult(nil) })
		assertContains0723pm(t, out, "provider_msp_backup_created=false\n")
		assertNotContains0723pm(t, out, "provider_msp_backup_created=true")
		assertNotContains0723pm(t, out, "archive_path=")
	})

	t.Run("populated result prints created=true, manifest and counts", func(t *testing.T) {
		result := &cloudcp.ProviderMSPBackupCreateResult{
			ArchivePath:         "/data/backups/provider-msp/latest.tar.gz",
			BytesWritten:        4096,
			ControlPlaneEntries: 3,
			TenantEntries:       5,
			LicenseEntries:      1,
			Manifest: cloudcp.ProviderMSPBackupManifest{
				Version:          cloudcp.ProviderMSPBackupManifestVersion,
				ControlPlaneMode: string(cloudcp.ControlPlaneModeProviderHostedMSP),
			},
		}
		out := capturePrint0723pm(t, func() { printProviderMSPBackupCreateResult(result) })
		assertContains0723pm(t, out, "provider_msp_backup_created=true\n")
		assertContains0723pm(t, out, "archive_path=/data/backups/provider-msp/latest.tar.gz\n")
		assertContains0723pm(t, out, "archive_bytes=4096\n")
		assertContains0723pm(t, out, "control_plane_entries=3\n")
		assertContains0723pm(t, out, "tenant_entries=5\n")
		assertContains0723pm(t, out, "license_entries=1\n")
		// Delegation to the manifest printer happened.
		assertContains0723pm(t, out, "manifest_version="+cloudcp.ProviderMSPBackupManifestVersion+"\n")
	})
}

// TestBranchcov0723pmPrintProviderMSPBackupRestoreResult covers the restore
// printer. Its notable branch is provider_msp_backup_restored being the negation
// of DryRun, plus the nil guard and the RestoredRuntimeTenantIDs loop.
func TestBranchcov0723pmPrintProviderMSPBackupRestoreResult(t *testing.T) {
	t.Run("nil result prints restored=false", func(t *testing.T) {
		out := capturePrint0723pm(t, func() { printProviderMSPBackupRestoreResult(nil) })
		assertContains0723pm(t, out, "provider_msp_backup_restored=false\n")
		assertNotContains0723pm(t, out, "archive_path=")
	})

	t.Run("dry run prints restored=false and dry_run=true", func(t *testing.T) {
		result := &cloudcp.ProviderMSPBackupRestoreResult{
			DryRun:               true,
			ArchivePath:          "/b/a.tar.gz",
			TargetDataDir:        "/data",
			ControlPlaneDir:      "/data/control-plane",
			TenantsDir:           "/data/tenants",
			LicenseOutputPath:    "/data/provider-msp-license.jwt",
			ReplaceExisting:      true,
			VerifiedArchiveBytes: 2048,
		}
		out := capturePrint0723pm(t, func() { printProviderMSPBackupRestoreResult(result) })
		// restored is !DryRun -> false even though result is non-nil.
		assertContains0723pm(t, out, "provider_msp_backup_restored=false\n")
		assertNotContains0723pm(t, out, "provider_msp_backup_restored=true")
		assertContains0723pm(t, out, "provider_msp_backup_restore_dry_run=true\n")
		assertContains0723pm(t, out, "replace_existing=true\n")
		assertContains0723pm(t, out, "archive_bytes=2048\n")
	})

	t.Run("real restore prints restored=true and runtime tenant ids", func(t *testing.T) {
		result := &cloudcp.ProviderMSPBackupRestoreResult{
			DryRun:                      false,
			ArchivePath:                 "/b/a.tar.gz",
			TargetDataDir:               "/data",
			ControlPlaneDir:             "/data/control-plane",
			TenantsDir:                  "/data/tenants",
			LicenseOutputPath:           "/data/provider-msp-license.jwt",
			ReplaceExisting:             false,
			VerifiedArchiveBytes:        4096,
			ControlPlaneEntriesRestored: 3,
			TenantEntriesRestored:       5,
			LicenseEntriesRestored:      1,
			RestoredRegistryTenantCount: 2,
			RestoredRuntimeTenantIDs:    []string{"t-1", "t-2"},
		}
		out := capturePrint0723pm(t, func() { printProviderMSPBackupRestoreResult(result) })
		assertContains0723pm(t, out, "provider_msp_backup_restored=true\n")
		assertContains0723pm(t, out, "provider_msp_backup_restore_dry_run=false\n")
		assertContains0723pm(t, out, "control_plane_entries_restored=3\n")
		assertContains0723pm(t, out, "tenant_entries_restored=5\n")
		assertContains0723pm(t, out, "license_entries_restored=1\n")
		assertContains0723pm(t, out, "restored_registry_tenant_count=2\n")
		// Both runtime ids printed, in order.
		assertContains0723pm(t, out, "restored_runtime_tenant_id=t-1\n")
		assertContains0723pm(t, out, "restored_runtime_tenant_id=t-2\n")
		mustIndexBefore0723pm(t, out, "restored_runtime_tenant_id=t-1", "restored_runtime_tenant_id=t-2")
	})

	t.Run("empty restored runtime tenant list omits the loop line", func(t *testing.T) {
		result := &cloudcp.ProviderMSPBackupRestoreResult{DryRun: false}
		out := capturePrint0723pm(t, func() { printProviderMSPBackupRestoreResult(result) })
		assertNotContains0723pm(t, out, "restored_runtime_tenant_id=")
	})
}

// TestBranchcov0723pmPrintProviderMSPBackupVerifyResult covers the verify
// printer: nil guard plus the ControlPlaneDBFiles and RuntimeTenantDirs loops.
func TestBranchcov0723pmPrintProviderMSPBackupVerifyResult(t *testing.T) {
	t.Run("nil result prints verified=false", func(t *testing.T) {
		out := capturePrint0723pm(t, func() { printProviderMSPBackupVerifyResult(nil) })
		assertContains0723pm(t, out, "provider_msp_backup_verified=false\n")
		assertNotContains0723pm(t, out, "provider_msp_backup_verified=true")
		assertNotContains0723pm(t, out, "archive_path=")
	})

	t.Run("empty slices omit the loop lines", func(t *testing.T) {
		result := &cloudcp.ProviderMSPBackupVerifyResult{
			ArchivePath:          "/b/a.tar.gz",
			VerifiedArchiveBytes: 4096,
			ControlPlaneEntries:  3,
			TenantEntries:        5,
			LicenseEntries:       1,
			HasTenantRegistryDB:  true,
			HasLicenseFile:       false,
		}
		out := capturePrint0723pm(t, func() { printProviderMSPBackupVerifyResult(result) })
		assertContains0723pm(t, out, "provider_msp_backup_verified=true\n")
		assertContains0723pm(t, out, "archive_bytes=4096\n")
		assertContains0723pm(t, out, "control_plane_entries=3\n")
		assertContains0723pm(t, out, "tenant_entries=5\n")
		assertContains0723pm(t, out, "license_entries=1\n")
		assertContains0723pm(t, out, "tenant_registry_db_present=true\n")
		assertContains0723pm(t, out, "license_file_present=false\n")
		assertNotContains0723pm(t, out, "control_plane_db_backup=")
		assertNotContains0723pm(t, out, "runtime_tenant_dir=")
	})

	t.Run("populated slices emit both loops", func(t *testing.T) {
		result := &cloudcp.ProviderMSPBackupVerifyResult{
			ControlPlaneDBFiles: []string{"/data/registry.db.bak", "/data/audit.db.bak"},
			RuntimeTenantDirs:   []string{"t-1", "t-2"},
		}
		out := capturePrint0723pm(t, func() { printProviderMSPBackupVerifyResult(result) })
		assertContains0723pm(t, out, "control_plane_db_backup=/data/registry.db.bak\n")
		assertContains0723pm(t, out, "control_plane_db_backup=/data/audit.db.bak\n")
		assertContains0723pm(t, out, "runtime_tenant_dir=t-1\n")
		assertContains0723pm(t, out, "runtime_tenant_dir=t-2\n")
	})
}

// TestBranchcov0723pmPrintProviderMSPBackupManifest covers the manifest printer.
// It takes the manifest BY VALUE so there is no nil branch; coverage targets the
// zero-value form (empty slices -> no loop lines) and the populated form, plus
// the fixed CreatedAt timestamp formatting.
func TestBranchcov0723pmPrintProviderMSPBackupManifest(t *testing.T) {
	t.Run("zero-value manifest prints zero counts and no loop lines", func(t *testing.T) {
		out := capturePrint0723pm(t, func() { printProviderMSPBackupManifest(cloudcp.ProviderMSPBackupManifest{}) })
		assertContains0723pm(t, out, "manifest_version=\n")
		// Zero time.Time formats deterministically in this layout.
		assertContains0723pm(t, out, "created_at=0001-01-01T00:00:00Z\n")
		assertContains0723pm(t, out, "control_plane_mode=\n")
		assertContains0723pm(t, out, "workspace_limit=0\n")
		assertContains0723pm(t, out, "registry_account_count=0\n")
		assertContains0723pm(t, out, "registry_tenant_count=0\n")
		assertContains0723pm(t, out, "runtime_tenant_count=0\n")
		assertContains0723pm(t, out, "license_included=false\n")
		assertNotContains0723pm(t, out, "runtime_tenant_id=")
		assertNotContains0723pm(t, out, "manifest_db_backup=")
	})

	t.Run("populated manifest prints all fields, formatted timestamp and both loops", func(t *testing.T) {
		created := time.Date(2026, 7, 23, 12, 34, 56, 0, time.UTC)
		manifest := cloudcp.ProviderMSPBackupManifest{
			Version:              cloudcp.ProviderMSPBackupManifestVersion,
			CreatedAt:            created,
			ControlPlaneMode:     string(cloudcp.ControlPlaneModeProviderHostedMSP),
			Environment:          "production",
			BaseURL:              "https://msp.example.com",
			PlanVersion:          "msp_growth",
			PlanSource:           "license-file",
			LicenseID:            "lic-1",
			LicenseEmail:         "ops@example.com",
			LicenseIncluded:      true,
			WorkspaceLimit:       15,
			RegistryAccountCount: 2,
			RegistryTenantCount:  5,
			RuntimeTenantCount:   5,
			RuntimeTenantIDs:     []string{"t-1", "t-2"},
			ControlPlaneDBBackups: []string{
				"/data/backups/provider-msp/control-plane/registry.db.bak",
			},
		}
		out := capturePrint0723pm(t, func() { printProviderMSPBackupManifest(manifest) })
		assertContains0723pm(t, out, "manifest_version="+cloudcp.ProviderMSPBackupManifestVersion+"\n")
		assertContains0723pm(t, out, "created_at=2026-07-23T12:34:56Z\n")
		assertContains0723pm(t, out, "control_plane_mode="+string(cloudcp.ControlPlaneModeProviderHostedMSP)+"\n")
		assertContains0723pm(t, out, "environment=production\n")
		assertContains0723pm(t, out, "base_url=https://msp.example.com\n")
		assertContains0723pm(t, out, "plan_version=msp_growth\n")
		assertContains0723pm(t, out, "plan_source=license-file\n")
		assertContains0723pm(t, out, "license_id=lic-1\n")
		assertContains0723pm(t, out, "license_email=ops@example.com\n")
		assertContains0723pm(t, out, "license_included=true\n")
		assertContains0723pm(t, out, "workspace_limit=15\n")
		assertContains0723pm(t, out, "registry_account_count=2\n")
		assertContains0723pm(t, out, "registry_tenant_count=5\n")
		assertContains0723pm(t, out, "runtime_tenant_count=5\n")
		assertContains0723pm(t, out, "runtime_tenant_id=t-1\n")
		assertContains0723pm(t, out, "runtime_tenant_id=t-2\n")
		assertContains0723pm(t, out, "manifest_db_backup=/data/backups/provider-msp/control-plane/registry.db.bak\n")
	})
}
