package api

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/telemetry"
	"github.com/rcourtman/pulse-go-rewrite/pkg/audit"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

// The RBAC adoption count must mean "this operator configured RBAC", so the
// built-in roles every install ships with are excluded.
func TestApplyLicensedFeatureTelemetrySnapshot_CountsOnlyOperatorAuthoredRBAC(t *testing.T) {
	provider := NewTenantRBACProvider(t.TempDir())
	t.Cleanup(func() {
		if err := provider.Close(); err != nil {
			t.Errorf("provider close failed: %v", err)
		}
	})

	manager, err := provider.GetManager("default")
	if err != nil {
		t.Fatalf("GetManager(default) failed: %v", err)
	}
	builtInRoles := 0
	for _, role := range manager.GetRoles() {
		if role.IsBuiltIn {
			builtInRoles++
		}
	}
	if builtInRoles == 0 {
		t.Fatal("expected the RBAC store to seed built-in roles")
	}

	if err := manager.SaveRole(auth.Role{ID: "ops-oncall", Name: "Ops On-Call"}); err != nil {
		t.Fatalf("SaveRole failed: %v", err)
	}
	if err := manager.AssignRole("alice", "ops-oncall"); err != nil {
		t.Fatalf("AssignRole failed: %v", err)
	}

	router := &Router{rbacProvider: provider}
	var snap telemetry.Snapshot
	router.ApplyLicensedFeatureTelemetrySnapshot(&snap, time.Now().UTC())

	if snap.RBACCustomRoles != 1 {
		t.Fatalf("custom roles = %d, want 1 (built-ins must not count)", snap.RBACCustomRoles)
	}
	if snap.RBACUserAssignments != 1 {
		t.Fatalf("user assignments = %d, want 1", snap.RBACUserAssignments)
	}
}

// The OSS console audit logger is not a persistent store; reporting it as one
// would overstate audit-logging adoption across the fleet.
func TestApplyLicensedFeatureTelemetrySnapshot_ConsoleAuditLoggerIsNotAdoption(t *testing.T) {
	provider := NewTenantRBACProvider(t.TempDir())
	t.Cleanup(func() {
		if err := provider.Close(); err != nil {
			t.Errorf("provider close failed: %v", err)
		}
	})

	// The audit logger is process-global, so pin it rather than inheriting
	// whatever a sibling test in this package installed.
	previous := audit.GetLogger()
	audit.SetLogger(audit.NewConsoleLogger())
	t.Cleanup(func() { audit.SetLogger(previous) })

	router := &Router{rbacProvider: provider}
	var snap telemetry.Snapshot
	router.ApplyLicensedFeatureTelemetrySnapshot(&snap, time.Now().UTC())

	if snap.AuditLoggingPersistent {
		t.Fatal("console audit logger must not report persistent audit logging")
	}
	if snap.AuditEvents30d != 0 {
		t.Fatalf("audit events = %d, want 0 without a persistent store", snap.AuditEvents30d)
	}
}

func TestApplyLicensedFeatureTelemetrySnapshot_ToleratesNilInputs(t *testing.T) {
	var router *Router
	var snap telemetry.Snapshot
	router.ApplyLicensedFeatureTelemetrySnapshot(&snap, time.Now().UTC())
	(&Router{}).ApplyLicensedFeatureTelemetrySnapshot(nil, time.Now().UTC())
}
