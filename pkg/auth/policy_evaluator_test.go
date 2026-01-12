package auth

import (
	"context"
	"os"
	"testing"
)

func TestPolicyEvaluator(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "policy-eval-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	m, err := NewSQLiteManager(SQLiteManagerConfig{DataDir: tmpDir})
	if err != nil {
		t.Fatalf("Failed to create SQLiteManager: %v", err)
	}
	defer m.Close()

	evaluator := NewPolicyEvaluator(m)

	// Setup test data
	setupTestRoles(t, m)

	t.Run("Allow permission works", func(t *testing.T) {
		ctx := WithUser(context.Background(), "allow-user")
		m.UpdateUserRoles("allow-user", []string{"allow-role"})

		allowed, err := evaluator.Authorize(ctx, "read", "nodes")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !allowed {
			t.Error("Expected access to be allowed")
		}
	})

	t.Run("No matching permission denies", func(t *testing.T) {
		ctx := WithUser(context.Background(), "allow-user")

		allowed, err := evaluator.Authorize(ctx, "delete", "nodes")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if allowed {
			t.Error("Expected access to be denied (no matching permission)")
		}
	})

	t.Run("Deny takes precedence over allow", func(t *testing.T) {
		ctx := WithUser(context.Background(), "deny-user")
		m.UpdateUserRoles("deny-user", []string{"deny-role"})

		// deny-role has allow on nodes:* but deny on nodes:production
		allowed, err := evaluator.Authorize(ctx, "write", "nodes:test")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !allowed {
			t.Error("Expected write to nodes:test to be allowed")
		}

		allowed, err = evaluator.Authorize(ctx, "write", "nodes:production")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if allowed {
			t.Error("Expected write to nodes:production to be denied")
		}
	})

	t.Run("Multiple roles combined", func(t *testing.T) {
		ctx := WithUser(context.Background(), "multi-user")
		m.UpdateUserRoles("multi-user", []string{"allow-role", "extra-role"})

		// allow-role grants read:nodes, extra-role grants write:alerts
		allowed, err := evaluator.Authorize(ctx, "read", "nodes")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !allowed {
			t.Error("Expected read nodes allowed from allow-role")
		}

		allowed, err = evaluator.Authorize(ctx, "write", "alerts")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !allowed {
			t.Error("Expected write alerts allowed from extra-role")
		}
	})

	t.Run("No user in context denies", func(t *testing.T) {
		ctx := context.Background() // No user

		allowed, err := evaluator.Authorize(ctx, "read", "nodes")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if allowed {
			t.Error("Expected access denied with no user in context")
		}
	})

	t.Run("User with no roles denies", func(t *testing.T) {
		ctx := WithUser(context.Background(), "no-role-user")
		// Don't assign any roles

		allowed, err := evaluator.Authorize(ctx, "read", "nodes")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if allowed {
			t.Error("Expected access denied with no roles")
		}
	})
}

func TestPolicyEvaluatorWithAttributes(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "policy-abac-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	m, err := NewSQLiteManager(SQLiteManagerConfig{DataDir: tmpDir})
	if err != nil {
		t.Fatalf("Failed to create SQLiteManager: %v", err)
	}
	defer m.Close()

	evaluator := NewPolicyEvaluator(m)

	// Create role with conditions
	condRole := Role{
		ID:   "cond-role",
		Name: "Conditional Role",
		Permissions: []Permission{
			{
				Action:     "read",
				Resource:   "nodes:*",
				Effect:     EffectAllow,
				Conditions: map[string]string{"env": "test"},
			},
			{
				Action:     "write",
				Resource:   "nodes:*",
				Effect:     EffectAllow,
				Conditions: map[string]string{"owner": "${user}"},
			},
		},
	}
	m.SaveRole(condRole)
	m.UpdateUserRoles("cond-user", []string{"cond-role"})

	t.Run("Condition matches", func(t *testing.T) {
		ctx := WithUser(context.Background(), "cond-user")

		allowed, err := evaluator.AuthorizeWithAttributes(ctx, "read", "nodes:dev", map[string]string{"env": "test"})
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !allowed {
			t.Error("Expected access allowed when condition matches")
		}
	})

	t.Run("Condition does not match", func(t *testing.T) {
		ctx := WithUser(context.Background(), "cond-user")

		allowed, err := evaluator.AuthorizeWithAttributes(ctx, "read", "nodes:dev", map[string]string{"env": "prod"})
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if allowed {
			t.Error("Expected access denied when condition doesn't match")
		}
	})

	t.Run("Variable substitution works", func(t *testing.T) {
		ctx := WithUser(context.Background(), "cond-user")

		// ${user} should be replaced with "cond-user"
		allowed, err := evaluator.AuthorizeWithAttributes(ctx, "write", "nodes:dev", map[string]string{"owner": "cond-user"})
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !allowed {
			t.Error("Expected access allowed when ${user} matches current user")
		}

		// Different owner should fail
		allowed, err = evaluator.AuthorizeWithAttributes(ctx, "write", "nodes:dev", map[string]string{"owner": "other-user"})
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if allowed {
			t.Error("Expected access denied when owner doesn't match current user")
		}
	})

	t.Run("Missing attribute denies", func(t *testing.T) {
		ctx := WithUser(context.Background(), "cond-user")

		// No env attribute provided
		allowed, err := evaluator.AuthorizeWithAttributes(ctx, "read", "nodes:dev", nil)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if allowed {
			t.Error("Expected access denied when required attribute is missing")
		}
	})
}

func TestPolicyEvaluatorWithInheritance(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "policy-inherit-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	m, err := NewSQLiteManager(SQLiteManagerConfig{DataDir: tmpDir})
	if err != nil {
		t.Fatalf("Failed to create SQLiteManager: %v", err)
	}
	defer m.Close()

	evaluator := NewPolicyEvaluator(m)

	// Create parent role
	parentRole := Role{
		ID:          "parent",
		Name:        "Parent",
		Permissions: []Permission{{Action: "read", Resource: "base"}},
	}
	m.SaveRole(parentRole)

	// Create child role
	childRole := Role{
		ID:          "child",
		Name:        "Child",
		ParentID:    "parent",
		Permissions: []Permission{{Action: "write", Resource: "child"}},
	}
	m.SaveRole(childRole)

	m.UpdateUserRoles("inherit-user", []string{"child"})

	t.Run("Inherited permission works", func(t *testing.T) {
		ctx := WithUser(context.Background(), "inherit-user")

		// Should have access via inheritance
		allowed, err := evaluator.Authorize(ctx, "read", "base")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !allowed {
			t.Error("Expected inherited permission to grant access")
		}
	})

	t.Run("Own permission works", func(t *testing.T) {
		ctx := WithUser(context.Background(), "inherit-user")

		allowed, err := evaluator.Authorize(ctx, "write", "child")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !allowed {
			t.Error("Expected own permission to grant access")
		}
	})
}

func TestRBACAuthorizer(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "rbac-auth-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	m, err := NewSQLiteManager(SQLiteManagerConfig{DataDir: tmpDir})
	if err != nil {
		t.Fatalf("Failed to create SQLiteManager: %v", err)
	}
	defer m.Close()

	authorizer := NewRBACAuthorizer(m)

	// Setup
	setupTestRoles(t, m)
	m.UpdateUserRoles("normal-user", []string{"allow-role"})

	t.Run("Normal user authorization", func(t *testing.T) {
		ctx := WithUser(context.Background(), "normal-user")

		allowed, err := authorizer.Authorize(ctx, "read", "nodes")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !allowed {
			t.Error("Expected normal user to have read access")
		}
	})

	t.Run("Admin bypass", func(t *testing.T) {
		authorizer.SetAdminUser("superadmin")
		ctx := WithUser(context.Background(), "superadmin")

		// Admin should have access even without roles
		allowed, err := authorizer.Authorize(ctx, "delete", "everything")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !allowed {
			t.Error("Expected admin user to have full access")
		}
	})

	t.Run("Non-admin still checks permissions", func(t *testing.T) {
		ctx := WithUser(context.Background(), "normal-user")

		// normal-user doesn't have delete permission
		allowed, err := authorizer.Authorize(ctx, "delete", "nodes")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if allowed {
			t.Error("Expected non-admin to be denied")
		}
	})
}

func TestMatchesResource(t *testing.T) {
	tests := []struct {
		pattern  string
		resource string
		expected bool
	}{
		{"*", "anything", true},
		{"*", "nodes", true},
		{"nodes", "nodes", true},
		{"nodes", "alerts", false},
		{"nodes:pve1", "nodes:pve1", true},
		{"nodes:pve1", "nodes:pve2", false},
		{"nodes:*", "nodes:pve1", true},
		{"nodes:*", "nodes:pve2", true},
		{"nodes:*", "nodes", true},
		{"nodes:*", "alerts", false},
		{"settings:*", "settings:admin", true},
		{"settings:*", "settings", true},
		{"admin:*", "admin:users", true},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.resource, func(t *testing.T) {
			result := MatchesResource(tt.pattern, tt.resource)
			if result != tt.expected {
				t.Errorf("MatchesResource(%q, %q) = %v, expected %v", tt.pattern, tt.resource, result, tt.expected)
			}
		})
	}
}

func TestMatchesAction(t *testing.T) {
	tests := []struct {
		permAction      string
		requestedAction string
		expected        bool
	}{
		{"admin", "read", true},
		{"admin", "write", true},
		{"admin", "delete", true},
		{"admin", "admin", true},
		{"read", "read", true},
		{"read", "write", false},
		{"write", "write", true},
		{"write", "read", false},
		{"delete", "delete", true},
		{"delete", "read", false},
	}

	for _, tt := range tests {
		t.Run(tt.permAction+"_"+tt.requestedAction, func(t *testing.T) {
			result := MatchesAction(tt.permAction, tt.requestedAction)
			if result != tt.expected {
				t.Errorf("MatchesAction(%q, %q) = %v, expected %v", tt.permAction, tt.requestedAction, result, tt.expected)
			}
		})
	}
}

// Helper function to setup test roles
func setupTestRoles(t *testing.T, m *SQLiteManager) {
	// Allow role - basic read access
	allowRole := Role{
		ID:   "allow-role",
		Name: "Allow Role",
		Permissions: []Permission{
			{Action: "read", Resource: "nodes", Effect: EffectAllow},
		},
	}
	if err := m.SaveRole(allowRole); err != nil {
		t.Fatalf("Failed to create allow-role: %v", err)
	}

	// Deny role - allow with specific deny
	denyRole := Role{
		ID:   "deny-role",
		Name: "Deny Role",
		Permissions: []Permission{
			{Action: "write", Resource: "nodes:*", Effect: EffectAllow},
			{Action: "write", Resource: "nodes:production", Effect: EffectDeny},
		},
	}
	if err := m.SaveRole(denyRole); err != nil {
		t.Fatalf("Failed to create deny-role: %v", err)
	}

	// Extra role for multi-role tests
	extraRole := Role{
		ID:   "extra-role",
		Name: "Extra Role",
		Permissions: []Permission{
			{Action: "write", Resource: "alerts", Effect: EffectAllow},
		},
	}
	if err := m.SaveRole(extraRole); err != nil {
		t.Fatalf("Failed to create extra-role: %v", err)
	}
}
