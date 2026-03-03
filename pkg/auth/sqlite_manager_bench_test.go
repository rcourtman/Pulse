package auth

import (
	"context"
	"fmt"
	"testing"
)

// newBenchManager creates an ephemeral SQLiteManager pre-populated with
// built-in roles. Caller must close via b.Cleanup.
func newBenchManager(b *testing.B) *SQLiteManager {
	b.Helper()
	dir := b.TempDir()
	m, err := NewSQLiteManager(SQLiteManagerConfig{DataDir: dir})
	if err != nil {
		b.Fatalf("NewSQLiteManager: %v", err)
	}
	b.Cleanup(func() { m.Close() })
	return m
}

// seedBenchUsers creates numUsers users, each assigned to one of the
// built-in roles in a round-robin pattern (admin, operator, viewer, auditor).
func seedBenchUsers(b *testing.B, m *SQLiteManager, numUsers int) []string {
	b.Helper()
	roles := []string{RoleAdmin, RoleOperator, RoleViewer, RoleAuditor}
	users := make([]string, numUsers)
	for i := 0; i < numUsers; i++ {
		username := fmt.Sprintf("bench-user-%04d", i)
		users[i] = username
		roleID := roles[i%len(roles)]
		if err := m.AssignRole(username, roleID); err != nil {
			b.Fatalf("AssignRole(%s, %s): %v", username, roleID, err)
		}
	}
	return users
}

// seedBenchCustomRoles creates numRoles custom roles, each with numPerms
// permissions. Returns the role IDs.
func seedBenchCustomRoles(b *testing.B, m *SQLiteManager, numRoles, numPerms int) []string {
	b.Helper()
	actions := []string{"read", "write", "delete"}
	resources := []string{"nodes", "vms", "containers", "alerts", "settings", "ai", "discovery", "audit_logs"}
	ids := make([]string, numRoles)
	for i := 0; i < numRoles; i++ {
		id := fmt.Sprintf("custom-role-%04d", i)
		ids[i] = id
		perms := make([]Permission, numPerms)
		for j := 0; j < numPerms; j++ {
			perms[j] = Permission{
				Action:   actions[j%len(actions)],
				Resource: resources[j%len(resources)],
				Effect:   EffectAllow,
			}
		}
		if err := m.SaveRole(Role{
			ID:          id,
			Name:        fmt.Sprintf("Custom Role %d", i),
			Description: "Benchmark custom role",
			Permissions: perms,
		}); err != nil {
			b.Fatalf("SaveRole(%s): %v", id, err)
		}
	}
	return ids
}

// BenchmarkGetUserPermissions measures the hot path of resolving a user's
// effective permissions from SQLite. This is called on every permissioned
// API request via the PolicyEvaluator → SQLiteManager chain.
func BenchmarkGetUserPermissions(b *testing.B) {
	for _, numUsers := range []int{10, 50, 200} {
		b.Run(fmt.Sprintf("users=%d", numUsers), func(b *testing.B) {
			m := newBenchManager(b)
			users := seedBenchUsers(b, m, numUsers)

			// Verify permissions are returned (guard against silent regressions).
			perms := m.GetUserPermissions(users[0])
			if len(perms) == 0 {
				b.Fatalf("expected non-empty permissions for %s", users[0])
			}

			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = m.GetUserPermissions(users[i%len(users)])
			}
		})
	}
}

// BenchmarkGetRole measures single role lookup by ID — called during
// permission resolution and role inheritance walks.
func BenchmarkGetRole(b *testing.B) {
	m := newBenchManager(b)
	// Create 10 custom roles to add variety beyond built-ins.
	customIDs := seedBenchCustomRoles(b, m, 10, 6)

	allRoleIDs := append([]string{RoleAdmin, RoleOperator, RoleViewer, RoleAuditor}, customIDs...)

	// Verify every role is found (guard against silent regressions).
	for _, id := range allRoleIDs {
		if _, ok := m.GetRole(id); !ok {
			b.Fatalf("role %q not found", id)
		}
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, ok := m.GetRole(allRoleIDs[i%len(allRoleIDs)]); !ok {
			b.Fatal("unexpected miss in timed loop")
		}
	}
}

// BenchmarkGetUserAssignment measures user-to-role assignment lookup — a
// prerequisite for every permission resolution.
func BenchmarkGetUserAssignment(b *testing.B) {
	for _, numUsers := range []int{10, 100} {
		b.Run(fmt.Sprintf("users=%d", numUsers), func(b *testing.B) {
			m := newBenchManager(b)
			users := seedBenchUsers(b, m, numUsers)

			// Verify assignments exist (guard against silent regressions).
			if _, ok := m.GetUserAssignment(users[0]); !ok {
				b.Fatalf("expected assignment for %s", users[0])
			}

			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, ok := m.GetUserAssignment(users[i%len(users)]); !ok {
					b.Fatal("unexpected miss in timed loop")
				}
			}
		})
	}
}

// BenchmarkAuthorize measures the full RBAC authorization chain:
// RBACAuthorizer → PolicyEvaluator → SQLiteManager. This is the complete
// per-request hot path when RBAC is active. Contexts are prebuilt outside
// the timed loop to isolate auth-chain cost from context allocation.
func BenchmarkAuthorize(b *testing.B) {
	m := newBenchManager(b)
	users := seedBenchUsers(b, m, 50)
	authorizer := NewRBACAuthorizer(m)

	// Prebuild contexts and request descriptors.
	type authReq struct {
		ctx      context.Context
		action   string
		resource string
	}
	requests := make([]authReq, len(users))
	for i, u := range users {
		requests[i] = authReq{
			ctx:      WithUser(context.Background(), u),
			action:   ActionRead,
			resource: ResourceNodes,
		}
	}

	// Verify the authorization path works.
	allowed, err := authorizer.Authorize(requests[0].ctx, ActionRead, ResourceNodes)
	if err != nil {
		b.Fatalf("Authorize error: %v", err)
	}
	// users[0] is assigned to admin (round-robin), admin has action=admin resource=*
	if !allowed {
		b.Fatal("expected user-0 (admin) to be allowed")
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req := requests[i%len(requests)]
		authorizer.Authorize(req.ctx, req.action, req.resource)
	}
}

// BenchmarkGetRolesWithInheritance measures the inheritance-aware role
// resolution path. This exercises the recursive parent-chain walk that
// occurs when the ExtendedManager interface is detected by PolicyEvaluator.
func BenchmarkGetRolesWithInheritance(b *testing.B) {
	m := newBenchManager(b)

	// Create a 3-level role hierarchy: base → mid → leaf.
	if err := m.SaveRole(Role{
		ID:          "bench-base",
		Name:        "Base",
		Description: "Base role",
		Permissions: []Permission{
			{Action: "read", Resource: "nodes"},
			{Action: "read", Resource: "vms"},
		},
	}); err != nil {
		b.Fatalf("SaveRole(base): %v", err)
	}
	if err := m.SaveRole(Role{
		ID:          "bench-mid",
		Name:        "Mid",
		Description: "Mid role",
		ParentID:    "bench-base",
		Permissions: []Permission{
			{Action: "write", Resource: "nodes"},
		},
	}); err != nil {
		b.Fatalf("SaveRole(mid): %v", err)
	}
	if err := m.SaveRole(Role{
		ID:          "bench-leaf",
		Name:        "Leaf",
		Description: "Leaf role",
		ParentID:    "bench-mid",
		Permissions: []Permission{
			{Action: "write", Resource: "alerts"},
		},
	}); err != nil {
		b.Fatalf("SaveRole(leaf): %v", err)
	}

	// Assign users to the leaf role (triggers 3-level inheritance walk).
	const numUsers = 20
	users := make([]string, numUsers)
	for i := 0; i < numUsers; i++ {
		username := fmt.Sprintf("inherit-user-%04d", i)
		users[i] = username
		if err := m.AssignRole(username, "bench-leaf"); err != nil {
			b.Fatalf("AssignRole: %v", err)
		}
	}

	// Verify inheritance resolves 3 roles.
	roles := m.GetRolesWithInheritance(users[0])
	if len(roles) != 3 {
		b.Fatalf("expected 3 inherited roles, got %d", len(roles))
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		m.GetRolesWithInheritance(users[i%len(users)])
	}
}
