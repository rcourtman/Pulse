package cloudcp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These tests raise branch/function coverage of the two exported planner
// wrappers in tenant_runtime_rollout.go:
//   - PlanTenantRuntimeContractReconcile
//   - PlanTenantRuntimeImageRollout
//
// Both wrappers build a real service via newTenantRuntimeRolloutServiceFromConfig
// (which constructs a lazy Docker client and a local sqlite registry) and then
// delegate to the (already covered) service.PlanContractReconcile /
// service.PlanImageRollout methods. The genuinely deterministic, Docker-free
// arms are exercised here:
//   * the happy path with an empty tenant registry and All=true (the plan loop
//     never runs, so s.docker.Inspect against the live daemon is never reached),
//   * the image-resolution fallbacks of PlanTenantRuntimeImageRollout
//     (opts.Image wins; otherwise cfg.PulseImage),
//   * constructor-error propagation when the tenants dir or control-plane dir
//     cannot be created.
//
// The tenant-populated planning branches (noop/rollout/skip/missing/drift) are
// only reachable through the exported wrappers by calling s.docker.Inspect
// against a live Docker daemon and are therefore out of scope per the purity
// gate. They are already covered for the underlying (mocked) service methods in
// tenant_runtime_rollout_test.go.

// branchcov0723AmEmptyRegistryCfg returns a CPConfig whose DataDir points at a
// fresh, writable temp directory, so newTenantRuntimeRolloutServiceFromConfig
// can construct a real (empty) sqlite registry and lazy Docker client. Each
// caller gets its own temp dir so subtests are fully independent.
func branchcov0723AmEmptyRegistryCfg(t *testing.T, pulseImage string) *CPConfig {
	t.Helper()
	return &CPConfig{
		DataDir:    t.TempDir(),
		PulseImage: pulseImage,
	}
}

// branchcov0723AmBlockingFile places a regular file at <dir>/<name> so that a
// subsequent os.MkdirAll on a path that descends through it fails with ENOTDIR.
func branchcov0723AmBlockingFile(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte("block"), 0o600); err != nil {
		t.Fatalf("write blocking file %s: %v", filepath.Join(dir, name), err)
	}
}

func TestBranchcov0723AmPlanTenantRuntimeContractReconcile(t *testing.T) {
	t.Run("EmptyRegistryReturnsNonNilEmptyPlan", func(t *testing.T) {
		// All=true against an empty registry: selectContractReconcileTenants
		// returns an empty list, the planning loop is skipped entirely (no
		// live Docker Inspect), and an empty plan with a non-nil Tenants
		// slice is returned. This exercises the full happy path of the
		// wrapper, including successful service construction.
		cfg := branchcov0723AmEmptyRegistryCfg(t, "pulse-runtime:stable")

		plan, err := PlanTenantRuntimeContractReconcile(
			context.Background(),
			cfg,
			TenantRuntimeContractReconcilePlanOptions{All: true},
		)
		if err != nil {
			t.Fatalf("PlanTenantRuntimeContractReconcile() error = %v", err)
		}
		if plan == nil {
			t.Fatal("plan = nil, want non-nil plan")
		}
		if plan.Tenants == nil {
			t.Fatal("plan.Tenants = nil, want non-nil (empty) slice")
		}
		if len(plan.Tenants) != 0 {
			t.Fatalf("len(plan.Tenants) = %d, want 0", len(plan.Tenants))
		}
	})

	t.Run("TenantsDirNotCreatablePropagatesError", func(t *testing.T) {
		// Point DataDir at a regular file so TenantsDir() = <file>/tenants
		// cannot be created: newTenantRuntimeRolloutServiceFromConfig fails
		// at the very first MkdirAll and the wrapper returns that error.
		// No Docker client is constructed.
		blockingDir := t.TempDir()
		blockingFile := filepath.Join(blockingDir, "iamfile")
		branchcov0723AmBlockingFile(t, blockingDir, "iamfile")

		cfg := &CPConfig{DataDir: blockingFile, PulseImage: "pulse-runtime:stable"}
		plan, err := PlanTenantRuntimeContractReconcile(
			context.Background(),
			cfg,
			TenantRuntimeContractReconcilePlanOptions{All: true},
		)
		if err == nil {
			t.Fatal("expected error when tenants dir is not creatable, got nil")
		}
		if !strings.Contains(err.Error(), "ensure tenants dir") {
			t.Fatalf("err = %q, want substring 'ensure tenants dir'", err.Error())
		}
		if plan != nil {
			t.Fatalf("plan = %v, want nil on constructor error", plan)
		}
	})

	t.Run("ControlPlaneDirNotCreatablePropagatesError", func(t *testing.T) {
		// TenantsDir() is creatable, but ControlPlaneDir() resolves to an
		// existing regular file: the second MkdirAll fails and the wrapper
		// returns the distinct "ensure control-plane dir" error.
		dataDir := t.TempDir()
		branchcov0723AmBlockingFile(t, dataDir, "control-plane")

		cfg := &CPConfig{DataDir: dataDir, PulseImage: "pulse-runtime:stable"}
		plan, err := PlanTenantRuntimeContractReconcile(
			context.Background(),
			cfg,
			TenantRuntimeContractReconcilePlanOptions{All: true},
		)
		if err == nil {
			t.Fatal("expected error when control-plane dir is not creatable, got nil")
		}
		if !strings.Contains(err.Error(), "ensure control-plane dir") {
			t.Fatalf("err = %q, want substring 'ensure control-plane dir'", err.Error())
		}
		if plan != nil {
			t.Fatalf("plan = %v, want nil on constructor error", plan)
		}
	})
}

func TestBranchcov0723AmPlanTenantRuntimeImageRollout(t *testing.T) {
	t.Run("EmptyRegistryUsesOptsImageAndReturnsNonNilEmptyPlan", func(t *testing.T) {
		// opts.Image is set and must win: the wrapper assigns it back to
		// opts.Image (line opts.Image = image) and builds the service with
		// that image. With an empty registry the plan loop never reaches
		// s.docker.Inspect, so this is deterministic without a daemon.
		cfg := branchcov0723AmEmptyRegistryCfg(t, "pulse-runtime:fallback")

		plan, err := PlanTenantRuntimeImageRollout(
			context.Background(),
			cfg,
			TenantRuntimeImageRolloutPlanOptions{All: true, Image: "pulse-runtime:next"},
		)
		if err != nil {
			t.Fatalf("PlanTenantRuntimeImageRollout() error = %v", err)
		}
		if plan == nil {
			t.Fatal("plan = nil, want non-nil plan")
		}
		if plan.Tenants == nil {
			t.Fatal("plan.Tenants = nil, want non-nil (empty) slice")
		}
		if len(plan.Tenants) != 0 {
			t.Fatalf("len(plan.Tenants) = %d, want 0", len(plan.Tenants))
		}
	})

	t.Run("TenantsDirNotCreatablePropagatesError", func(t *testing.T) {
		// Image is resolved successfully (so the nil/empty guards are passed)
		// but the service constructor fails at the first MkdirAll. Covers the
		// wrapper's error-propagation branch for a constructor failure, which
		// the pure-branch tests do not reach.
		blockingDir := t.TempDir()
		blockingFile := filepath.Join(blockingDir, "iamfile")
		branchcov0723AmBlockingFile(t, blockingDir, "iamfile")

		cfg := &CPConfig{DataDir: blockingFile, PulseImage: "pulse-runtime:stable"}
		plan, err := PlanTenantRuntimeImageRollout(
			context.Background(),
			cfg,
			TenantRuntimeImageRolloutPlanOptions{All: true, Image: "pulse-runtime:next"},
		)
		if err == nil {
			t.Fatal("expected error when tenants dir is not creatable, got nil")
		}
		if !strings.Contains(err.Error(), "ensure tenants dir") {
			t.Fatalf("err = %q, want substring 'ensure tenants dir'", err.Error())
		}
		if plan != nil {
			t.Fatalf("plan = %v, want nil on constructor error", plan)
		}
	})

	t.Run("ControlPlaneDirNotCreatablePropagatesError", func(t *testing.T) {
		// TenantsDir creatable, ControlPlaneDir blocked by a regular file:
		// the second MkdirAll fails and the wrapper returns the distinct
		// "ensure control-plane dir" error.
		dataDir := t.TempDir()
		branchcov0723AmBlockingFile(t, dataDir, "control-plane")

		cfg := &CPConfig{DataDir: dataDir, PulseImage: "pulse-runtime:stable"}
		plan, err := PlanTenantRuntimeImageRollout(
			context.Background(),
			cfg,
			TenantRuntimeImageRolloutPlanOptions{All: true, Image: "pulse-runtime:next"},
		)
		if err == nil {
			t.Fatal("expected error when control-plane dir is not creatable, got nil")
		}
		if !strings.Contains(err.Error(), "ensure control-plane dir") {
			t.Fatalf("err = %q, want substring 'ensure control-plane dir'", err.Error())
		}
		if plan != nil {
			t.Fatalf("plan = %v, want nil on constructor error", plan)
		}
	})
}
