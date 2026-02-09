package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

func TestRBACLifecycle_OrgDeletionCleansUpRBACCache(t *testing.T) {
	baseDir := t.TempDir()
	orgID := "acme"
	mustCreateLifecycleOrgDir(t, baseDir, orgID)

	provider := NewTenantRBACProvider(baseDir)
	t.Cleanup(func() {
		if err := provider.Close(); err != nil {
			t.Errorf("provider close failed: %v", err)
		}
	})

	if got := provider.ManagerCount(); got != 0 {
		t.Fatalf("expected empty manager cache, got %d", got)
	}

	if _, err := provider.GetManager(orgID); err != nil {
		t.Fatalf("GetManager(%s) failed: %v", orgID, err)
	}
	if got := provider.ManagerCount(); got != 1 {
		t.Fatalf("expected 1 cached manager after load, got %d", got)
	}

	if err := provider.RemoveTenant(orgID); err != nil {
		t.Fatalf("RemoveTenant(%s) failed: %v", orgID, err)
	}
	if got := provider.ManagerCount(); got != 0 {
		t.Fatalf("expected empty manager cache after remove, got %d", got)
	}
}

func TestRBACLifecycle_HandleDeleteOrgRemovesTenantManager(t *testing.T) {
	t.Setenv("PULSE_DEV", "true")
	defer SetMultiTenantEnabled(false)
	SetMultiTenantEnabled(true)

	baseDir := t.TempDir()
	persistence := config.NewMultiTenantPersistence(baseDir)
	provider := NewTenantRBACProvider(baseDir)
	t.Cleanup(func() {
		if err := provider.Close(); err != nil {
			t.Errorf("provider close failed: %v", err)
		}
	})

	handlers := NewOrgHandlers(persistence, nil, provider)

	createReq := withUser(
		httptest.NewRequest(http.MethodPost, "/api/orgs", bytes.NewBufferString(`{"id":"acme","displayName":"Acme"}`)),
		"alice",
	)
	createRec := httptest.NewRecorder()
	handlers.HandleCreateOrg(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create failed: %d %s", createRec.Code, createRec.Body.String())
	}

	if _, err := provider.GetManager("acme"); err != nil {
		t.Fatalf("GetManager(acme) failed: %v", err)
	}
	if got := provider.ManagerCount(); got != 1 {
		t.Fatalf("expected 1 cached manager before delete, got %d", got)
	}

	deleteReq := withUser(httptest.NewRequest(http.MethodDelete, "/api/orgs/acme", nil), "alice")
	deleteReq.SetPathValue("id", "acme")
	deleteRec := httptest.NewRecorder()
	handlers.HandleDeleteOrg(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusNoContent {
		t.Fatalf("delete failed: %d %s", deleteRec.Code, deleteRec.Body.String())
	}

	if got := provider.ManagerCount(); got != 0 {
		t.Fatalf("expected empty RBAC manager cache after org delete, got %d", got)
	}
}

func TestRBACLifecycle_OrgDeletionFreshManagerAfterRecreation(t *testing.T) {
	baseDir := t.TempDir()
	orgID := "acme"
	orgDir := filepath.Join(baseDir, "orgs", orgID)
	mustCreateLifecycleOrgDir(t, baseDir, orgID)

	provider := NewTenantRBACProvider(baseDir)
	t.Cleanup(func() {
		if err := provider.Close(); err != nil {
			t.Errorf("provider close failed: %v", err)
		}
	})

	manager1, err := provider.GetManager(orgID)
	if err != nil {
		t.Fatalf("GetManager(%s) failed: %v", orgID, err)
	}
	firstPtr := lifecycleSQLiteManagerPtr(t, manager1)

	if err := provider.RemoveTenant(orgID); err != nil {
		t.Fatalf("RemoveTenant(%s) failed: %v", orgID, err)
	}

	if err := os.RemoveAll(orgDir); err != nil {
		t.Fatalf("failed to remove org dir %s: %v", orgDir, err)
	}
	if err := os.MkdirAll(orgDir, 0700); err != nil {
		t.Fatalf("failed to recreate org dir %s: %v", orgDir, err)
	}

	manager2, err := provider.GetManager(orgID)
	if err != nil {
		t.Fatalf("GetManager(%s) after recreation failed: %v", orgID, err)
	}
	secondPtr := lifecycleSQLiteManagerPtr(t, manager2)

	if firstPtr == secondPtr {
		t.Fatalf("expected new manager instance for %s after remove/recreate", orgID)
	}
}

func TestRBACLifecycle_RemoveTenantNonExistentOrg(t *testing.T) {
	baseDir := t.TempDir()
	provider := NewTenantRBACProvider(baseDir)
	t.Cleanup(func() {
		if err := provider.Close(); err != nil {
			t.Errorf("provider close failed: %v", err)
		}
	})

	if err := provider.RemoveTenant("never-loaded"); err != nil {
		t.Fatalf("RemoveTenant(non-existent) returned error: %v", err)
	}
	if got := provider.ManagerCount(); got != 0 {
		t.Fatalf("expected empty manager cache, got %d", got)
	}
}

func TestRBACLifecycle_ManagerCountTracksCache(t *testing.T) {
	baseDir := t.TempDir()
	mustCreateLifecycleOrgDir(t, baseDir, "org-a")
	mustCreateLifecycleOrgDir(t, baseDir, "org-b")

	provider := NewTenantRBACProvider(baseDir)
	t.Cleanup(func() {
		if err := provider.Close(); err != nil {
			t.Errorf("provider close failed: %v", err)
		}
	})

	if _, err := provider.GetManager("org-a"); err != nil {
		t.Fatalf("GetManager(org-a) failed: %v", err)
	}
	if got := provider.ManagerCount(); got != 1 {
		t.Fatalf("expected 1 cached manager, got %d", got)
	}

	if _, err := provider.GetManager("org-b"); err != nil {
		t.Fatalf("GetManager(org-b) failed: %v", err)
	}
	if got := provider.ManagerCount(); got != 2 {
		t.Fatalf("expected 2 cached managers, got %d", got)
	}

	if _, err := provider.GetManager("org-a"); err != nil {
		t.Fatalf("second GetManager(org-a) failed: %v", err)
	}
	if got := provider.ManagerCount(); got != 2 {
		t.Fatalf("expected manager count to remain 2 after cache hit, got %d", got)
	}

	if err := provider.RemoveTenant("org-b"); err != nil {
		t.Fatalf("RemoveTenant(org-b) failed: %v", err)
	}
	if got := provider.ManagerCount(); got != 1 {
		t.Fatalf("expected 1 cached manager after remove, got %d", got)
	}
}

func mustCreateLifecycleOrgDir(t *testing.T, baseDir, orgID string) {
	t.Helper()
	orgDir := filepath.Join(baseDir, "orgs", orgID)
	if err := os.MkdirAll(orgDir, 0700); err != nil {
		t.Fatalf("failed to create org dir %s: %v", orgDir, err)
	}
}

func lifecycleSQLiteManagerPtr(t *testing.T, manager auth.ExtendedManager) *auth.SQLiteManager {
	t.Helper()
	sqliteManager, ok := manager.(*auth.SQLiteManager)
	if !ok {
		t.Fatalf("expected *auth.SQLiteManager, got %T", manager)
	}
	return sqliteManager
}
