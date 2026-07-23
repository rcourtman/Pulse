package cloudcp

import (
	"context"
	"errors"
	"strings"
	"testing"

	cpDocker "github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/docker"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
)

// TestBranchcov0723AmAddFailure exercises every assertable branch of
// (*CloudAuditReport).addFailure. The nil-receiver arm is intentionally not
// covered: it is only reachable through a typed nil pointer and its sole
// observable effect is "did not panic", which this suite forbids as a
// tautology. Every other branch — empty/whitespace rejection, OK side-effect,
// trimming, order preservation, and the absence of deduplication — is driven
// with concrete inputs and asserted against the source's actual semantics.
func TestBranchcov0723AmAddFailure(t *testing.T) {
	t.Parallel()

	t.Run("empty failure string is rejected and leaves OK untouched", func(t *testing.T) {
		t.Parallel()
		r := &CloudAuditReport{OK: true}
		r.addFailure("")
		if r.OK != true {
			t.Fatalf("OK = %v, want true (empty failure must not flip OK)", r.OK)
		}
		if len(r.Failures) != 0 {
			t.Fatalf("Failures = %v, want empty (empty failure must be rejected)", r.Failures)
		}
	})

	t.Run("whitespace-only failure string is rejected after trim", func(t *testing.T) {
		t.Parallel()
		r := &CloudAuditReport{OK: true}
		r.addFailure("   \t\n  ")
		if r.OK != true {
			t.Fatalf("OK = %v, want true (whitespace-only failure must not flip OK)", r.OK)
		}
		if len(r.Failures) != 0 {
			t.Fatalf("Failures = %v, want empty (whitespace-only failure must be rejected)", r.Failures)
		}
	})

	t.Run("real failure on zero-value report creates slice and flips OK", func(t *testing.T) {
		t.Parallel()
		// Start from a literal zero-value report (Failures nil, OK false) to
		// prove addFailure both allocates the slice and unconditionally sets
		// OK=false for an accepted failure.
		var r CloudAuditReport
		r.addFailure("docker manager unavailable: boom")
		if r.OK != false {
			t.Fatalf("OK = %v, want false after a real failure", r.OK)
		}
		want := []string{"docker manager unavailable: boom"}
		if len(r.Failures) != 1 || r.Failures[0] != want[0] {
			t.Fatalf("Failures = %v, want %v", r.Failures, want)
		}
	})

	t.Run("surrounding whitespace is trimmed before appending", func(t *testing.T) {
		t.Parallel()
		r := &CloudAuditReport{OK: true}
		r.addFailure("  storage guardrails failed: low disk  ")
		if r.OK != false {
			t.Fatalf("OK = %v, want false", r.OK)
		}
		if len(r.Failures) != 1 || r.Failures[0] != "storage guardrails failed: low disk" {
			t.Fatalf("Failures = %v, want single trimmed entry", r.Failures)
		}
	})

	t.Run("multiple failures preserve insertion order", func(t *testing.T) {
		t.Parallel()
		r := &CloudAuditReport{OK: true}
		r.addFailure("first failure")
		r.addFailure("second failure")
		r.addFailure("third failure")
		want := []string{"first failure", "second failure", "third failure"}
		if len(r.Failures) != len(want) {
			t.Fatalf("Failures = %v, want %v", r.Failures, want)
		}
		for i := range want {
			if r.Failures[i] != want[i] {
				t.Fatalf("Failures[%d] = %q, want %q (order not preserved)", i, r.Failures[i], want[i])
			}
		}
		if r.OK != false {
			t.Fatalf("OK = %v, want false after failures", r.OK)
		}
	})

	t.Run("duplicate failure is NOT deduplicated", func(t *testing.T) {
		t.Parallel()
		// The source performs no deduplication; assert that exact semantics so
		// a future change to dedupe is caught as a behaviour change.
		r := &CloudAuditReport{OK: true}
		r.addFailure("same failure")
		r.addFailure("same failure")
		if len(r.Failures) != 2 {
			t.Fatalf("Failures = %v (len=%d), want len=2 (source does not dedupe)",
				r.Failures, len(r.Failures))
		}
		if r.Failures[0] != "same failure" || r.Failures[1] != "same failure" {
			t.Fatalf("Failures = %v, want both entries equal to %q", r.Failures, "same failure")
		}
	})

	t.Run("OK stays false after a real failure even if a later failure is empty", func(t *testing.T) {
		t.Parallel()
		// Asserts the OK side-effect is monotonic: once flipped false it is not
		// restored, and a subsequent rejected (empty) failure does not append.
		r := &CloudAuditReport{OK: true}
		r.addFailure("real failure")
		r.addFailure("")
		if r.OK != false {
			t.Fatalf("OK = %v, want false (must remain false after first real failure)", r.OK)
		}
		if len(r.Failures) != 1 || r.Failures[0] != "real failure" {
			t.Fatalf("Failures = %v, want exactly [\"real failure\"]", r.Failures)
		}
	})
}

// TestBranchcov0723AmRuntimeContainerUnhealthy covers both arms of every
// conditional in runtimeContainerUnhealthy: the State=="running" guard (which
// is case-SENSITIVE on State per the source), and the lowercased/trimmed
// HealthStatus switch covering the healthy cases ("", "none", "healthy"), the
// default/unhealthy arm, case-insensitivity, and whitespace handling.
func TestBranchcov0723AmRuntimeContainerUnhealthy(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		state      string
		health     string
		wantUnwell bool
	}{
		// --- State arm (case-sensitive): anything not exactly "running" is unhealthy ---
		{name: "zero value state and health", state: "", health: "", wantUnwell: true},
		{name: "paused state", state: "paused", health: "", wantUnwell: true},
		{name: "exited state", state: "exited", health: "", wantUnwell: true},
		{name: "created state", state: "created", health: "", wantUnwell: true},
		{name: "restarting state", state: "restarting", health: "", wantUnwell: true},
		{name: "state is case sensitive uppercase RUNNING", state: "RUNNING", health: "", wantUnwell: true},
		{name: "state is case mixed Running", state: "Running", health: "", wantUnwell: true},

		// --- Healthy arm: running + (empty|none|healthy), after lowercasing & trimming ---
		{name: "running with empty health", state: "running", health: "", wantUnwell: false},
		{name: "running with none", state: "running", health: "none", wantUnwell: false},
		{name: "running with healthy", state: "running", health: "healthy", wantUnwell: false},
		{name: "running with NONE uppercase", state: "running", health: "NONE", wantUnwell: false},
		{name: "running with HEALTHY uppercase", state: "running", health: "HEALTHY", wantUnwell: false},
		{name: "running with Mixed Case Healthy", state: "running", health: "HeAlThY", wantUnwell: false},
		{name: "running with health surrounded by whitespace", state: "running", health: "  healthy  ", wantUnwell: false},

		// --- Default/unhealthy arm: running + any other health status ---
		{name: "running with unhealthy", state: "running", health: "unhealthy", wantUnwell: true},
		{name: "running with starting", state: "running", health: "starting", wantUnwell: true},
		{name: "running with unknown health no-health-status", state: "running", health: "no-such-state", wantUnwell: true},
		{name: "running with UNHEALTHY uppercase", state: "running", health: "UNHEALTHY", wantUnwell: true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := cpDocker.RuntimeContainerSummary{
				ID:           "c-id",
				Name:         "c-name",
				State:        tc.state,
				HealthStatus: tc.health,
			}
			got := runtimeContainerUnhealthy(c)
			if got != tc.wantUnwell {
				t.Fatalf("runtimeContainerUnhealthy(state=%q, health=%q) = %v, want %v",
					tc.state, tc.health, got, tc.wantUnwell)
			}
		})
	}
}

// TestBranchcov0723AmTenantDisplayNameResolver covers both branches of the
// closure returned by tenantDisplayNameResolver: the lookup-failure arm
// (unknown tenant id -> "") and the success arm (existing tenant -> trimmed
// display name), plus the whitespace-name edge case. It reuses the existing
// newEmptyRegistryForTest helper from cloudcp_pure_branchcov0718_test.go,
// which builds a real SQLite-backed registry rooted at a per-test tempdir.
func TestBranchcov0723AmTenantDisplayNameResolver(t *testing.T) {
	t.Parallel()

	t.Run("unknown tenant id resolves to empty string", func(t *testing.T) {
		t.Parallel()
		reg := newEmptyRegistryForTest(t)
		defer reg.Close()
		resolver := tenantDisplayNameResolver(reg)
		if got := resolver("does-not-exist"); got != "" {
			t.Fatalf("resolver(unknown) = %q, want empty string", got)
		}
	})

	t.Run("existing tenant display name is returned trimmed", func(t *testing.T) {
		t.Parallel()
		reg := newEmptyRegistryForTest(t)
		defer reg.Close()
		tenant := &registry.Tenant{
			ID:          "t-display",
			AccountID:   "acc-1",
			Email:       "owner@example.com",
			DisplayName: "  Acme Workspace  ",
		}
		if err := reg.Create(tenant); err != nil {
			t.Fatalf("Create tenant: %v", err)
		}
		resolver := tenantDisplayNameResolver(reg)
		if got := resolver("t-display"); got != "Acme Workspace" {
			t.Fatalf("resolver(existing) = %q, want %q", got, "Acme Workspace")
		}
	})

	t.Run("whitespace-only display name resolves to empty string", func(t *testing.T) {
		t.Parallel()
		reg := newEmptyRegistryForTest(t)
		defer reg.Close()
		tenant := &registry.Tenant{
			ID:          "t-blank",
			AccountID:   "acc-2",
			Email:       "owner2@example.com",
			DisplayName: "   ",
		}
		if err := reg.Create(tenant); err != nil {
			t.Fatalf("Create tenant: %v", err)
		}
		resolver := tenantDisplayNameResolver(reg)
		if got := resolver("t-blank"); got != "" {
			t.Fatalf("resolver(whitespace-name) = %q, want empty string after TrimSpace", got)
		}
	})
}

// TestBranchcov0723AmEnforceStorageAdmission covers the two pure early-return
// branches of EnforceStorageAdmission: cfg=nil propagates the underlying
// CheckStorageGuardrails error, and a cfg with guardrails disabled returns nil
// without ever touching the Docker usage provider or the filesystem.
func TestBranchcov0723AmEnforceStorageAdmission(t *testing.T) {
	t.Parallel()

	t.Run("nil cfg returns control plane config required error", func(t *testing.T) {
		t.Parallel()
		err := EnforceStorageAdmission(context.Background(), nil, nil)
		if err == nil {
			t.Fatal("expected error for nil cfg, got nil")
		}
		if !strings.Contains(err.Error(), "control plane config is required") {
			t.Fatalf("err = %q, want substring 'control plane config is required'", err.Error())
		}
	})

	t.Run("guardrails disabled returns nil without invoking docker usage provider", func(t *testing.T) {
		t.Parallel()
		// Provider that fails the test if invoked, proving the disabled
		// guardrail path short-circuits before any Docker call.
		var providerInvoked bool
		provider := failIfInvokedUsageProvider{t: t, invoked: &providerInvoked}
		cfg := &CPConfig{StorageGuardrailsEnabled: false}
		if err := EnforceStorageAdmission(context.Background(), cfg, provider); err != nil {
			t.Fatalf("EnforceStorageAdmission(disabled) = %v, want nil", err)
		}
		if providerInvoked {
			t.Fatal("docker usage provider was invoked; disabled guardrail path should short-circuit")
		}
	})
}

// failIfInvokedUsageProvider is a StorageDockerUsageProvider that fails the
// associated test if DiskUsage is called. It exists to prove that the
// guardrails-disabled path in EnforceStorageAdmission never reaches the
// Docker usage provider.
type failIfInvokedUsageProvider struct {
	t       *testing.T
	invoked *bool
}

func (f failIfInvokedUsageProvider) DiskUsage(context.Context) (*cpDocker.DiskUsageSnapshot, error) {
	*f.invoked = true
	f.t.Fatal("DiskUsage must not be called on the disabled-guardrail path")
	return nil, nil
}

// TestBranchcov0723AmRecoverProviderMSPWorkspaces covers the pure
// early-return branches of RecoverProviderMSPWorkspaces that are reachable
// before any Docker daemon, live registry, or running server is required:
// already-cancelled context, nil cfg, and both options-validation error
// arms. The deeper execution branches (which need a Docker manager and a
// provider-MSP-configured control plane) are intentionally out of scope.
func TestBranchcov0723AmRecoverProviderMSPWorkspaces(t *testing.T) {
	t.Parallel()

	t.Run("cancelled context returns ctx error", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := RecoverProviderMSPWorkspaces(ctx, &CPConfig{},
			ProviderMSPRecoveryOptions{AllDegraded: true})
		if err == nil {
			t.Fatal("expected error for cancelled context, got nil")
		}
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want errors.Is(err, context.Canceled)", err)
		}
	})

	t.Run("nil cfg returns control plane config required", func(t *testing.T) {
		t.Parallel()
		_, err := RecoverProviderMSPWorkspaces(context.Background(), nil,
			ProviderMSPRecoveryOptions{AllDegraded: true})
		if err == nil {
			t.Fatal("expected error for nil cfg, got nil")
		}
		if !strings.Contains(err.Error(), "control plane config is required") {
			t.Fatalf("err = %q, want substring 'control plane config is required'", err.Error())
		}
	})

	t.Run("opts with both AllDegraded and TenantIDs is rejected", func(t *testing.T) {
		t.Parallel()
		_, err := RecoverProviderMSPWorkspaces(context.Background(), &CPConfig{},
			ProviderMSPRecoveryOptions{AllDegraded: true, TenantIDs: []string{"t-1"}})
		if err == nil {
			t.Fatal("expected error for mutually exclusive options, got nil")
		}
		if !strings.Contains(err.Error(), "choose either --all-degraded or one or more --tenant-id values") {
			t.Fatalf("err = %q, want substring 'choose either --all-degraded or one or more --tenant-id values'", err.Error())
		}
	})

	t.Run("opts with neither AllDegraded nor TenantIDs is rejected", func(t *testing.T) {
		t.Parallel()
		_, err := RecoverProviderMSPWorkspaces(context.Background(), &CPConfig{},
			ProviderMSPRecoveryOptions{})
		if err == nil {
			t.Fatal("expected error for empty options, got nil")
		}
		if !strings.Contains(err.Error(), "choose --all-degraded or at least one --tenant-id") {
			t.Fatalf("err = %q, want substring 'choose --all-degraded or at least one --tenant-id'", err.Error())
		}
	})
}

// TestBranchcov0723AmAuditCloudNilCfg covers the single pure branch of
// AuditCloud reachable without a Docker daemon or live registry: the nil-cfg
// guard that returns an error before any I/O is attempted. The remaining
// branches require a control-plane directory, a Docker manager, and storage
// guardrail checks, and are therefore out of scope per the purity gate.
func TestBranchcov0723AmAuditCloudNilCfg(t *testing.T) {
	t.Parallel()
	report, err := AuditCloud(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil cfg, got nil")
	}
	if !strings.Contains(err.Error(), "control plane config is required") {
		t.Fatalf("err = %q, want substring 'control plane config is required'", err.Error())
	}
	if report != nil {
		t.Fatalf("report = %v, want nil on cfg error", report)
	}
}

// TestBranchcov0723AmRolloutTenantRuntimePureBranches covers the two pure
// early-return branches of RolloutTenantRuntime: nil cfg and missing image
// (both opts.Image and cfg.PulseImage empty). The remaining branches construct
// a Docker manager and registry and call into service.Rollout, which requires
// a Docker daemon and is therefore out of scope.
func TestBranchcov0723AmRolloutTenantRuntimePureBranches(t *testing.T) {
	t.Parallel()

	t.Run("nil cfg returns control plane config required", func(t *testing.T) {
		t.Parallel()
		_, err := RolloutTenantRuntime(context.Background(), nil,
			TenantRuntimeRolloutOptions{TenantID: "t-1", Image: "pulse-runtime:latest"})
		if err == nil {
			t.Fatal("expected error for nil cfg, got nil")
		}
		if !strings.Contains(err.Error(), "control plane config is required") {
			t.Fatalf("err = %q, want substring 'control plane config is required'", err.Error())
		}
	})

	t.Run("missing image falls back to cfg then errors", func(t *testing.T) {
		t.Parallel()
		// opts.Image empty AND cfg.PulseImage empty -> the function must
		// reject before attempting any Docker/registry construction.
		cfg := &CPConfig{}
		_, err := RolloutTenantRuntime(context.Background(), cfg,
			TenantRuntimeRolloutOptions{TenantID: "t-1"})
		if err == nil {
			t.Fatal("expected error for missing image, got nil")
		}
		if !strings.Contains(err.Error(), "missing tenant runtime image") {
			t.Fatalf("err = %q, want substring 'missing tenant runtime image'", err.Error())
		}
	})
}

// TestBranchcov0723AmPlanTenantRuntimeImageRolloutPureBranches covers the two
// pure early-return branches of PlanTenantRuntimeImageRollout, mirroring
// RolloutTenantRuntime. The deeper branches require a Docker daemon.
func TestBranchcov0723AmPlanTenantRuntimeImageRolloutPureBranches(t *testing.T) {
	t.Parallel()

	t.Run("nil cfg returns control plane config required", func(t *testing.T) {
		t.Parallel()
		_, err := PlanTenantRuntimeImageRollout(context.Background(), nil,
			TenantRuntimeImageRolloutPlanOptions{Image: "pulse-runtime:latest"})
		if err == nil {
			t.Fatal("expected error for nil cfg, got nil")
		}
		if !strings.Contains(err.Error(), "control plane config is required") {
			t.Fatalf("err = %q, want substring 'control plane config is required'", err.Error())
		}
	})

	t.Run("missing image falls back to cfg then errors", func(t *testing.T) {
		t.Parallel()
		cfg := &CPConfig{}
		_, err := PlanTenantRuntimeImageRollout(context.Background(), cfg,
			TenantRuntimeImageRolloutPlanOptions{All: true})
		if err == nil {
			t.Fatal("expected error for missing image, got nil")
		}
		if !strings.Contains(err.Error(), "missing tenant runtime image") {
			t.Fatalf("err = %q, want substring 'missing tenant runtime image'", err.Error())
		}
	})
}
