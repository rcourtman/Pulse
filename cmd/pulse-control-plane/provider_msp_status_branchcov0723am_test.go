package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp"
	cpDocker "github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/docker"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
)

// TestBranchcov0723AmContainsProviderMSPStatusFailure exercises every branch
// of containsProviderMSPStatusFailure. The source compares with == (exact,
// case-sensitive equality), NOT strings.Contains; the cases below pin that.
func TestBranchcov0723AmContainsProviderMSPStatusFailure(t *testing.T) {
	cases := []struct {
		name     string
		failures []string
		want     string
		expected bool
	}{
		{"nil slice", nil, "anything", false},
		{"empty slice", []string{}, "anything", false},
		{"exact match first element", []string{"preflight: bad", "other"}, "preflight: bad", true},
		{"exact match last element", []string{"other", "preflight: bad"}, "preflight: bad", true},
		{"no match", []string{"a", "b", "c"}, "d", false},
		{"case sensitive equality no match", []string{"Failure"}, "failure", false},
		{"substring of an entry does not match", []string{"preflight: bad"}, "bad", false},
		{"entry is substring of want does not match", []string{"bad"}, "preflight: bad", false},
		{"whole entry exact match", []string{"preflight: bad"}, "preflight: bad", true},
		{"empty want empty entry matches", []string{"a", ""}, "", true},
		{"empty want no empty entry", []string{"a", "b"}, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := containsProviderMSPStatusFailure(tc.failures, tc.want)
			if got != tc.expected {
				t.Fatalf("containsProviderMSPStatusFailure(%#v, %q) = %v, want %v", tc.failures, tc.want, got, tc.expected)
			}
		})
	}
}

// TestBranchcov0723AmSortedProviderMSPStatusStates pins the real semantics:
// states come from map keys, sorted ascending by state NAME (string value),
// not by count. Zero-count entries appear because they are keys.
func TestBranchcov0723AmSortedProviderMSPStatusStates(t *testing.T) {
	t.Run("nil map returns empty slice", func(t *testing.T) {
		got := sortedProviderMSPStatusStates(nil)
		if len(got) != 0 {
			t.Fatalf("nil map -> %v, want empty", got)
		}
	})
	t.Run("empty map returns empty slice", func(t *testing.T) {
		got := sortedProviderMSPStatusStates(map[registry.TenantState]int{})
		if len(got) != 0 {
			t.Fatalf("empty map -> %v, want empty", got)
		}
	})
	t.Run("single entry", func(t *testing.T) {
		got := sortedProviderMSPStatusStates(map[registry.TenantState]int{registry.TenantStateActive: 7})
		if len(got) != 1 || got[0] != registry.TenantStateActive {
			t.Fatalf("single entry -> %v, want [active]", got)
		}
	})
	t.Run("ordering is by state name not count", func(t *testing.T) {
		// Counts are deliberately out of alphabetical order to prove the
		// sort key is the state name, not the count.
		counts := map[registry.TenantState]int{
			registry.TenantStateFailed:       9,
			registry.TenantStateActive:       5,
			registry.TenantStateProvisioning: 1,
			registry.TenantStateDeleted:      3,
		}
		got := sortedProviderMSPStatusStates(counts)
		want := []registry.TenantState{
			registry.TenantStateActive,
			registry.TenantStateDeleted,
			registry.TenantStateFailed,
			registry.TenantStateProvisioning,
		}
		if len(got) != len(want) {
			t.Fatalf("got %v, want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("index %d got %v, want ordering %v", i, got, want)
			}
		}
	})
	t.Run("zero count entry is still listed", func(t *testing.T) {
		got := sortedProviderMSPStatusStates(map[registry.TenantState]int{registry.TenantStateActive: 0})
		if len(got) != 1 || got[0] != registry.TenantStateActive {
			t.Fatalf("zero-count entry -> %v, want [active]", got)
		}
	})
	t.Run("output is deterministic across random map iteration", func(t *testing.T) {
		// Go map iteration order is randomised; the sort must make output
		// stable. Run many times and assert identical result every time.
		counts := map[registry.TenantState]int{
			registry.TenantStateActive:       2,
			registry.TenantStateFailed:       2,
			registry.TenantStateProvisioning: 2,
			registry.TenantStateSuspended:    2,
			registry.TenantStateCanceled:     2,
			registry.TenantStateDeleting:     2,
			registry.TenantStateDeleted:      2,
		}
		first := sortedProviderMSPStatusStates(counts)
		firstKey := statesKey(first)
		for i := 0; i < 100; i++ {
			got := sortedProviderMSPStatusStates(counts)
			if k := statesKey(got); k != firstKey {
				t.Fatalf("iteration %d produced %v, want deterministic %v", i, got, first)
			}
		}
	})
}

func statesKey(states []registry.TenantState) string {
	parts := make([]string, len(states))
	for i, s := range states {
		parts[i] = string(s)
	}
	return strings.Join(parts, ",")
}

// TestBranchcov0723AmProviderMSPRestoreDefaultDataDir covers the env-driven
// default data dir resolver (a pure, filesystem-free helper).
func TestBranchcov0723AmProviderMSPRestoreDefaultDataDir(t *testing.T) {
	t.Run("env unset returns default", func(t *testing.T) {
		t.Setenv("CP_DATA_DIR", "")
		if got := providerMSPRestoreDefaultDataDir(); got != "/data" {
			t.Fatalf("env unset -> %q, want /data", got)
		}
	})
	t.Run("env whitespace only returns default", func(t *testing.T) {
		t.Setenv("CP_DATA_DIR", "   \t\n")
		if got := providerMSPRestoreDefaultDataDir(); got != "/data" {
			t.Fatalf("whitespace env -> %q, want /data", got)
		}
	})
	t.Run("env value is trimmed", func(t *testing.T) {
		t.Setenv("CP_DATA_DIR", "  /var/pulse  ")
		if got := providerMSPRestoreDefaultDataDir(); got != "/var/pulse" {
			t.Fatalf("trimmed env -> %q, want /var/pulse", got)
		}
	})
}

// TestBranchcov0723AmDefaultProviderMSPInstallProofRestoreTargetDataDir covers
// the cfg/env precedence: cfg.DataDir wins when set, else env-derived base,
// joined with the install-proof restore drill suffix.
func TestBranchcov0723AmDefaultProviderMSPInstallProofRestoreTargetDataDir(t *testing.T) {
	t.Run("nil cfg uses env base", func(t *testing.T) {
		t.Setenv("CP_DATA_DIR", "")
		got := defaultProviderMSPInstallProofRestoreTargetDataDir(nil)
		if want := filepath.Join("/data", "install-proof-restore-drill"); got != want {
			t.Fatalf("nil cfg -> %q, want %q", got, want)
		}
	})
	t.Run("nil cfg honours env override", func(t *testing.T) {
		t.Setenv("CP_DATA_DIR", "/custom")
		got := defaultProviderMSPInstallProofRestoreTargetDataDir(nil)
		if want := filepath.Join("/custom", "install-proof-restore-drill"); got != want {
			t.Fatalf("nil cfg env -> %q, want %q", got, want)
		}
	})
	t.Run("cfg DataDir overrides env", func(t *testing.T) {
		t.Setenv("CP_DATA_DIR", "/from-env")
		cfg := &cloudcp.CPConfig{DataDir: "/from-cfg"}
		got := defaultProviderMSPInstallProofRestoreTargetDataDir(cfg)
		if want := filepath.Join("/from-cfg", "install-proof-restore-drill"); got != want {
			t.Fatalf("cfg override -> %q, want %q", got, want)
		}
	})
	t.Run("cfg DataDir whitespace only falls back to env", func(t *testing.T) {
		t.Setenv("CP_DATA_DIR", "/from-env")
		cfg := &cloudcp.CPConfig{DataDir: "   "}
		got := defaultProviderMSPInstallProofRestoreTargetDataDir(cfg)
		if want := filepath.Join("/from-env", "install-proof-restore-drill"); got != want {
			t.Fatalf("whitespace cfg -> %q, want %q", got, want)
		}
	})
	t.Run("cfg DataDir is trimmed", func(t *testing.T) {
		t.Setenv("CP_DATA_DIR", "")
		cfg := &cloudcp.CPConfig{DataDir: "  /trimmed  "}
		got := defaultProviderMSPInstallProofRestoreTargetDataDir(cfg)
		if want := filepath.Join("/trimmed", "install-proof-restore-drill"); got != want {
			t.Fatalf("trimmed cfg -> %q, want %q", got, want)
		}
	})
}

// TestBranchcov0723AmPrintProviderMSPStatusReport covers the package's stdout
// print helper for the status report. The package already has a convention for
// capturing os.Stdout (captureStdoutForProviderMSPRecoverTest), which we reuse.
func TestBranchcov0723AmPrintProviderMSPStatusReport(t *testing.T) {
	t.Run("nil report prints ok=false", func(t *testing.T) {
		var buf bytes.Buffer
		restore := captureStdoutForProviderMSPRecoverTest(t, &buf)
		printProviderMSPStatusReport(nil)
		restore()
		out := buf.String()
		if !strings.Contains(out, "provider_msp_status_ok=false") {
			t.Fatalf("nil report output missing ok=false:\n%s", out)
		}
		if strings.Contains(out, "provider_msp_status_ok=true") {
			t.Fatalf("nil report should not print ok=true:\n%s", out)
		}
	})
	t.Run("populated report prints all sections in expected order", func(t *testing.T) {
		ts := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)
		report := &providerMSPStatusReport{
			OK:                       true,
			Environment:              "production",
			ControlMode:              "provider_hosted_msp",
			BaseURL:                  "https://msp.example.com",
			PlanVersion:              "msp_growth",
			PlanSource:               "license-file",
			LicenseID:                "lic_test",
			LicenseEmail:             "ops@example.com",
			WorkspaceLimit:           15,
			RegistryReady:            true,
			TotalTenants:             4,
			HealthyTenants:           2,
			UnhealthyTenants:         1,
			FailedTenants:            1,
			ProvisioningTenants:      1,
			StuckProvisioningTimeout: 30 * time.Minute,
			StuckProvisioningTenants: []string{"t-STUCK-1", "t-STUCK-2"},
			CountsByState: map[registry.TenantState]int{
				registry.TenantStateActive:       2,
				registry.TenantStateFailed:       1,
				registry.TenantStateProvisioning: 1,
			},
			Preflight: &providerMSPPreflightReport{
				Docker: &cpDocker.RuntimePrerequisiteReport{
					DockerReachable: true,
					NetworkOK:       true,
					ImageRef:        "pulse:test",
					ImageAvailable:  true,
					ImagePulled:     true,
				},
				Storage: &cloudcp.StorageGuardrailReport{
					Enabled: true,
					OK:      true,
				},
			},
			Backup: &providerMSPBackupStatus{
				Directory:           "/data/backups/provider-msp",
				LatestPath:          "/data/backups/provider-msp/latest.tar.gz",
				LatestCreatedAt:     ts,
				LatestModifiedAt:    ts,
				LatestBytes:         4096,
				Verified:            true,
				LicenseIncluded:     true,
				RegistryTenantCount: 2,
				RuntimeTenantCount:  2,
			},
			BackupReadyForUpgrade: true,
			Warnings:              []string{"disk usage high"},
			Failures:              []string{"failed workspaces: 1"},
		}
		var buf bytes.Buffer
		restore := captureStdoutForProviderMSPRecoverTest(t, &buf)
		printProviderMSPStatusReport(report)
		restore()
		out := buf.String()
		wants := []string{
			"provider_msp_status_ok=true",
			"environment=production",
			"control_plane_mode=provider_hosted_msp",
			"base_url=https://msp.example.com",
			"plan_version=msp_growth",
			"plan_source=license-file",
			"license_id=lic_test",
			"license_email=ops@example.com",
			"workspace_limit=15",
			"registry_ready=true",
			"total_tenants=4",
			"healthy_tenants=2",
			"unhealthy_tenants=1",
			"failed_tenants=1",
			"provisioning_tenants=1",
			"stuck_provisioning_count=2",
			"stuck_provisioning_tenant=t-STUCK-1",
			"stuck_provisioning_tenant=t-STUCK-2",
			// Sorted states (alphabetical by name): active, failed, provisioning.
			"tenant_state=active count=2",
			"tenant_state=failed count=1",
			"tenant_state=provisioning count=1",
			"docker_reachable=true",
			"docker_network_ok=true",
			"tenant_runtime_image=pulse:test",
			"tenant_runtime_image_available=true",
			"tenant_runtime_image_pulled=true",
			"storage_guardrails_enabled=true",
			"storage_guardrails_ok=true",
			"backup_ready_for_upgrade=true",
			"backup_directory=/data/backups/provider-msp",
			"latest_backup_path=/data/backups/provider-msp/latest.tar.gz",
			"latest_backup_created_at=",
			"latest_backup_modified_at=",
			"latest_backup_bytes=4096",
			"latest_backup_verified=true",
			"latest_backup_license_included=true",
			"latest_backup_registry_tenants=2",
			"latest_backup_runtime_tenants=2",
			"warning=disk usage high",
			"failure=failed workspaces: 1",
		}
		for _, w := range wants {
			if !strings.Contains(out, w) {
				t.Fatalf("output missing %q:\n%s", w, out)
			}
		}
		// Assert the alphabetical ordering of states in the printed output.
		activeIdx := strings.Index(out, "tenant_state=active")
		failedIdx := strings.Index(out, "tenant_state=failed")
		provisioningIdx := strings.Index(out, "tenant_state=provisioning")
		if !(activeIdx < failedIdx && failedIdx < provisioningIdx) {
			t.Fatalf("states not in alphabetical order: active=%d failed=%d provisioning=%d\n%s", activeIdx, failedIdx, provisioningIdx, out)
		}
	})
	t.Run("report without preflight or backup omits those sections", func(t *testing.T) {
		report := &providerMSPStatusReport{OK: false}
		var buf bytes.Buffer
		restore := captureStdoutForProviderMSPRecoverTest(t, &buf)
		printProviderMSPStatusReport(report)
		restore()
		out := buf.String()
		if !strings.Contains(out, "provider_msp_status_ok=false") {
			t.Fatalf("missing ok=false:\n%s", out)
		}
		for _, absent := range []string{
			"docker_reachable=",
			"storage_guardrails_enabled=",
			"backup_directory=",
			"latest_backup_path=",
		} {
			if strings.Contains(out, absent) {
				t.Fatalf("optional section %q should be absent:\n%s", absent, out)
			}
		}
	})
}
