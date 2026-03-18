package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

func TestRBACIntegrityCheck_HealthyOrg(t *testing.T) {
	baseDir := t.TempDir()
	orgDir := filepath.Join(baseDir, "orgs", "test-org")
	if err := os.MkdirAll(orgDir, 0700); err != nil {
		t.Fatalf("failed to create org dir: %v", err)
	}

	provider := NewTenantRBACProvider(baseDir)
	t.Cleanup(func() { _ = provider.Close() })

	handlers := NewRBACHandlers(nil, provider)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/rbac/integrity?org_id=test-org", nil)
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "test-org"))
	rec := httptest.NewRecorder()
	handlers.HandleRBACIntegrityCheck(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var result RBACIntegrityResult
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !result.Healthy {
		t.Fatalf("expected healthy=true, got false (error: %s)", result.Error)
	}
	if result.OrgID != "test-org" {
		t.Fatalf("expected org_id=test-org, got %s", result.OrgID)
	}
	if result.BuiltInRoleCount < 4 {
		t.Fatalf("expected >= 4 built-in roles, got %d", result.BuiltInRoleCount)
	}
}

func TestRBACIntegrityCheck_DefaultOrg(t *testing.T) {
	baseDir := t.TempDir()
	provider := NewTenantRBACProvider(baseDir)
	t.Cleanup(func() { _ = provider.Close() })

	handlers := NewRBACHandlers(nil, provider)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/rbac/integrity", nil)
	rec := httptest.NewRecorder()
	handlers.HandleRBACIntegrityCheck(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var result RBACIntegrityResult
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result.OrgID != "default" {
		t.Fatalf("expected org_id=default, got %s", result.OrgID)
	}
	if !result.DBAccessible {
		t.Fatalf("expected default org DB to be accessible, got false (error: %s)", result.Error)
	}
}

func TestResetAdminRoleEndpoint_MissingToken(t *testing.T) {
	baseDir := t.TempDir()
	provider := NewTenantRBACProvider(baseDir)
	t.Cleanup(func() { _ = provider.Close() })

	handlers := NewRBACHandlers(nil, provider)

	body := `{"org_id":"default","username":"admin"}`
	req := httptest.NewRequest(http.MethodPost, "/api/admin/rbac/reset-admin", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	handlers.HandleRBACAdminReset(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing token, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestResetAdminRoleEndpoint_InvalidToken(t *testing.T) {
	baseDir := t.TempDir()
	provider := NewTenantRBACProvider(baseDir)
	t.Cleanup(func() { _ = provider.Close() })

	resetRecoveryStore()
	t.Cleanup(resetRecoveryStore)
	InitRecoveryTokenStore(baseDir)

	handlers := NewRBACHandlers(nil, provider)

	body := `{"org_id":"default","username":"admin","recovery_token":"invalid-token-value"}`
	req := httptest.NewRequest(http.MethodPost, "/api/admin/rbac/reset-admin", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	handlers.HandleRBACAdminReset(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for invalid token, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestResetAdminRoleEndpoint_ValidToken(t *testing.T) {
	baseDir := t.TempDir()
	orgDir := filepath.Join(baseDir, "orgs", "test-org")
	if err := os.MkdirAll(orgDir, 0700); err != nil {
		t.Fatalf("failed to create org dir: %v", err)
	}

	provider := NewTenantRBACProvider(baseDir)
	t.Cleanup(func() { _ = provider.Close() })

	resetRecoveryStore()
	t.Cleanup(resetRecoveryStore)
	InitRecoveryTokenStore(baseDir)

	store := GetRecoveryTokenStore()
	token, err := store.GenerateRecoveryToken(5 * time.Minute)
	if err != nil {
		t.Fatalf("failed to generate recovery token: %v", err)
	}

	handlers := NewRBACHandlers(nil, provider)

	body, _ := json.Marshal(map[string]string{
		"org_id":         "test-org",
		"username":       "admin",
		"recovery_token": token,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/admin/rbac/reset-admin", bytes.NewBuffer(body))
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "test-org"))
	rec := httptest.NewRecorder()
	handlers.HandleRBACAdminReset(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["status"] != "ok" {
		t.Fatalf("expected status=ok, got %v", resp["status"])
	}
	if resp["org_id"] != "test-org" {
		t.Fatalf("expected org_id=test-org, got %v", resp["org_id"])
	}
	if resp["username"] != "admin" {
		t.Fatalf("expected username=admin, got %v", resp["username"])
	}

	manager, err := provider.GetManager("test-org")
	if err != nil {
		t.Fatalf("failed to get manager for verification: %v", err)
	}
	assignment, ok := manager.GetUserAssignment("admin")
	if !ok {
		t.Fatalf("expected admin assignment to exist")
	}
	if !containsRoleIDForAdminReset(assignment.RoleIDs, auth.RoleAdmin) {
		t.Fatalf("expected admin role assignment, got %v", assignment.RoleIDs)
	}
}

func TestRBACIntegrityCheck_RejectsCrossTenantQueryOrgID(t *testing.T) {
	baseDir := t.TempDir()
	provider := NewTenantRBACProvider(baseDir)
	t.Cleanup(func() { _ = provider.Close() })

	handlers := NewRBACHandlers(nil, provider)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/rbac/integrity?org_id=org-b", nil)
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "org-a"))
	rec := httptest.NewRecorder()
	handlers.HandleRBACIntegrityCheck(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for cross-tenant org_id, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestResetAdminRoleEndpoint_RejectsCrossTenantBodyOrgID(t *testing.T) {
	baseDir := t.TempDir()
	provider := NewTenantRBACProvider(baseDir)
	t.Cleanup(func() { _ = provider.Close() })

	handlers := NewRBACHandlers(nil, provider)

	body := `{"org_id":"org-b","username":"admin","recovery_token":"dummy"}`
	req := httptest.NewRequest(http.MethodPost, "/api/admin/rbac/reset-admin", bytes.NewBufferString(body))
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "org-a"))
	rec := httptest.NewRecorder()
	handlers.HandleRBACAdminReset(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for cross-tenant org_id, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestResetAdminRoleEndpoint_MissingUsername(t *testing.T) {
	baseDir := t.TempDir()
	provider := NewTenantRBACProvider(baseDir)
	t.Cleanup(func() { _ = provider.Close() })

	handlers := NewRBACHandlers(nil, provider)

	body := `{"org_id":"default","recovery_token":"some-token"}`
	req := httptest.NewRequest(http.MethodPost, "/api/admin/rbac/reset-admin", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	handlers.HandleRBACAdminReset(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing username, got %d: %s", rec.Code, rec.Body.String())
	}
}

func containsRoleIDForAdminReset(roleIDs []string, roleID string) bool {
	for _, id := range roleIDs {
		if id == roleID {
			return true
		}
	}
	return false
}
