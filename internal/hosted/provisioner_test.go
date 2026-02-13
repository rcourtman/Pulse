package hosted

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

type mockAuthProvider struct {
	manager   AuthManager
	err       error
	calls     int
	lastOrgID string
}

func (m *mockAuthProvider) GetManager(orgID string) (AuthManager, error) {
	m.calls++
	m.lastOrgID = orgID
	if m.err != nil {
		return nil, m.err
	}
	return m.manager, nil
}

type mockAuthManager struct {
	updateErr error
	calls     int
	lastUser  string
	lastRoles []string
}

func (m *mockAuthManager) UpdateUserRoles(userID string, roles []string) error {
	m.calls++
	m.lastUser = userID
	m.lastRoles = append([]string(nil), roles...)
	return m.updateErr
}

func TestProvisionTenantSuccess(t *testing.T) {
	baseDir := t.TempDir()
	persistence := config.NewMultiTenantPersistence(baseDir)
	authManager := &mockAuthManager{}
	authProvider := &mockAuthProvider{manager: authManager}

	provisioner := NewProvisioner(persistence, authProvider)
	provisioner.newOrgID = func() string { return "org-success" }
	provisioner.now = func() time.Time { return time.Unix(1700000000, 0).UTC() }

	result, err := provisioner.ProvisionTenant(context.Background(), ProvisionRequest{
		Email:    "owner@example.com",
		Password: "securepass123",
		OrgName:  "My Organization",
	})
	if err != nil {
		t.Fatalf("ProvisionTenant returned error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.Status != ProvisionStatusCreated {
		t.Fatalf("expected status %q, got %q", ProvisionStatusCreated, result.Status)
	}
	if result.OrgID != "org-success" {
		t.Fatalf("expected org ID org-success, got %q", result.OrgID)
	}
	if result.UserID != "owner@example.com" {
		t.Fatalf("expected user ID owner@example.com, got %q", result.UserID)
	}

	org, err := persistence.LoadOrganization("org-success")
	if err != nil {
		t.Fatalf("LoadOrganization returned error: %v", err)
	}
	if org.DisplayName != "My Organization" {
		t.Fatalf("expected org display name My Organization, got %q", org.DisplayName)
	}
	if org.OwnerUserID != "owner@example.com" {
		t.Fatalf("expected owner owner@example.com, got %q", org.OwnerUserID)
	}
	if len(org.Members) != 1 {
		t.Fatalf("expected 1 member, got %d", len(org.Members))
	}
	if org.Members[0].UserID != "owner@example.com" {
		t.Fatalf("expected member user ID owner@example.com, got %q", org.Members[0].UserID)
	}
	if org.Members[0].Role != models.OrgRoleOwner {
		t.Fatalf("expected owner role, got %q", org.Members[0].Role)
	}

	if authProvider.calls != 1 {
		t.Fatalf("expected GetManager to be called once, got %d", authProvider.calls)
	}
	if authProvider.lastOrgID != "org-success" {
		t.Fatalf("expected GetManager org ID org-success, got %q", authProvider.lastOrgID)
	}
	if authManager.calls != 1 {
		t.Fatalf("expected UpdateUserRoles to be called once, got %d", authManager.calls)
	}
	if authManager.lastUser != "owner@example.com" {
		t.Fatalf("expected UpdateUserRoles for owner@example.com, got %q", authManager.lastUser)
	}
	if len(authManager.lastRoles) != 1 || authManager.lastRoles[0] != auth.RoleAdmin {
		t.Fatalf("expected roles [%s], got %v", auth.RoleAdmin, authManager.lastRoles)
	}
}

func TestProvisionTenantIdempotentDuplicateEmail(t *testing.T) {
	baseDir := t.TempDir()
	persistence := config.NewMultiTenantPersistence(baseDir)
	authManager := &mockAuthManager{}
	authProvider := &mockAuthProvider{manager: authManager}
	provisioner := NewProvisioner(persistence, authProvider)

	existingOrg := &models.Organization{
		ID:          "existing-org",
		DisplayName: "Existing Org",
		CreatedAt:   time.Now().UTC(),
		OwnerUserID: "owner@example.com",
		Members: []models.OrganizationMember{
			{
				UserID:  "owner@example.com",
				Role:    models.OrgRoleOwner,
				AddedAt: time.Now().UTC(),
				AddedBy: "owner@example.com",
			},
		},
	}
	if err := persistence.SaveOrganization(existingOrg); err != nil {
		t.Fatalf("SaveOrganization returned error: %v", err)
	}

	result, err := provisioner.ProvisionTenant(context.Background(), ProvisionRequest{
		Email:    "owner@example.com",
		Password: "securepass123",
		OrgName:  "New Org Name",
	})
	if err != nil {
		t.Fatalf("ProvisionTenant returned error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.Status != ProvisionStatusExisting {
		t.Fatalf("expected status %q, got %q", ProvisionStatusExisting, result.Status)
	}
	if result.OrgID != "existing-org" {
		t.Fatalf("expected org ID existing-org, got %q", result.OrgID)
	}
	if result.UserID != "owner@example.com" {
		t.Fatalf("expected user ID owner@example.com, got %q", result.UserID)
	}
	if authProvider.calls != 0 {
		t.Fatalf("expected GetManager to not be called for idempotent path, got %d", authProvider.calls)
	}
	if authManager.calls != 0 {
		t.Fatalf("expected UpdateUserRoles to not be called for idempotent path, got %d", authManager.calls)
	}
}

func TestProvisionTenantIdempotentDuplicateEmailCaseInsensitive(t *testing.T) {
	baseDir := t.TempDir()
	persistence := config.NewMultiTenantPersistence(baseDir)
	authManager := &mockAuthManager{}
	authProvider := &mockAuthProvider{manager: authManager}
	provisioner := NewProvisioner(persistence, authProvider)

	existingOrg := &models.Organization{
		ID:          "existing-org",
		DisplayName: "Existing Org",
		CreatedAt:   time.Now().UTC(),
		OwnerUserID: "owner@example.com",
		Members: []models.OrganizationMember{
			{
				UserID:  "owner@example.com",
				Role:    models.OrgRoleOwner,
				AddedAt: time.Now().UTC(),
				AddedBy: "owner@example.com",
			},
		},
	}
	if err := persistence.SaveOrganization(existingOrg); err != nil {
		t.Fatalf("SaveOrganization returned error: %v", err)
	}

	result, err := provisioner.ProvisionTenant(context.Background(), ProvisionRequest{
		Email:    "Owner@Example.com",
		Password: "securepass123",
		OrgName:  "New Org Name",
	})
	if err != nil {
		t.Fatalf("ProvisionTenant returned error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.Status != ProvisionStatusExisting {
		t.Fatalf("expected status %q, got %q", ProvisionStatusExisting, result.Status)
	}
	if result.OrgID != "existing-org" {
		t.Fatalf("expected org ID existing-org, got %q", result.OrgID)
	}
	if result.UserID != "owner@example.com" {
		t.Fatalf("expected normalized user ID owner@example.com, got %q", result.UserID)
	}
	if authProvider.calls != 0 {
		t.Fatalf("expected GetManager to not be called for idempotent path, got %d", authProvider.calls)
	}
	if authManager.calls != 0 {
		t.Fatalf("expected UpdateUserRoles to not be called for idempotent path, got %d", authManager.calls)
	}
}

func TestProvisionTenantValidationFailures(t *testing.T) {
	baseDir := t.TempDir()
	persistence := config.NewMultiTenantPersistence(baseDir)
	authProvider := &mockAuthProvider{manager: &mockAuthManager{}}
	provisioner := NewProvisioner(persistence, authProvider)

	testCases := []struct {
		name          string
		request       ProvisionRequest
		expectedField string
	}{
		{
			name: "invalid email",
			request: ProvisionRequest{
				Email:    "invalid-email",
				Password: "securepass123",
				OrgName:  "Valid Org",
			},
			expectedField: "email",
		},
		{
			name: "email with control characters",
			request: ProvisionRequest{
				Email:    "owner@\nexample.com",
				Password: "securepass123",
				OrgName:  "Valid Org",
			},
			expectedField: "email",
		},
		{
			name: "short password",
			request: ProvisionRequest{
				Email:    "owner@example.com",
				Password: "short",
				OrgName:  "Valid Org",
			},
			expectedField: "password",
		},
		{
			name: "password exceeds maximum length",
			request: ProvisionRequest{
				Email:    "owner@example.com",
				Password: strings.Repeat("a", maxHostedSignupPasswordLength+1),
				OrgName:  "Valid Org",
			},
			expectedField: "password",
		},
		{
			name: "bad org name",
			request: ProvisionRequest{
				Email:    "owner@example.com",
				Password: "securepass123",
				OrgName:  "../evil",
			},
			expectedField: "org_name",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := provisioner.ProvisionTenant(context.Background(), tc.request)
			if err == nil {
				t.Fatal("expected validation error, got nil")
			}

			var validationErr *ValidationError
			if !errors.As(err, &validationErr) {
				t.Fatalf("expected ValidationError, got %T (%v)", err, err)
			}
			if validationErr.Field != tc.expectedField {
				t.Fatalf("expected validation field %q, got %q", tc.expectedField, validationErr.Field)
			}
		})
	}
}

func TestProvisionTenantPartialFailureRollback(t *testing.T) {
	baseDir := t.TempDir()
	persistence := config.NewMultiTenantPersistence(baseDir)
	authManager := &mockAuthManager{updateErr: errors.New("rbac update failed")}
	authProvider := &mockAuthProvider{manager: authManager}
	provisioner := NewProvisioner(persistence, authProvider)
	provisioner.newOrgID = func() string { return "rollback-org" }

	_, err := provisioner.ProvisionTenant(context.Background(), ProvisionRequest{
		Email:    "owner@example.com",
		Password: "securepass123",
		OrgName:  "Rollback Org",
	})
	if err == nil {
		t.Fatal("expected provisioning error, got nil")
	}

	var systemErr *SystemError
	if !errors.As(err, &systemErr) {
		t.Fatalf("expected SystemError, got %T (%v)", err, err)
	}

	orgDir := filepath.Join(baseDir, "orgs", "rollback-org")
	_, statErr := os.Stat(orgDir)
	if !os.IsNotExist(statErr) {
		t.Fatalf("expected org dir to be removed, stat error: %v", statErr)
	}
}

func TestCleanupOrgDirectorySkipsUnsafePath(t *testing.T) {
	baseDir := t.TempDir()
	p := &Provisioner{}

	unsafeDir := filepath.Join(baseDir, "not-orgs", "unsafe-org")
	if err := os.MkdirAll(unsafeDir, 0700); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}

	p.cleanupOrgDirectory("unsafe-org", unsafeDir)

	if _, err := os.Stat(unsafeDir); err != nil {
		t.Fatalf("expected unsafe directory to be preserved, stat error: %v", err)
	}
}

func TestCleanupOrgDirectoryUsesOrganizationDeleter(t *testing.T) {
	baseDir := t.TempDir()
	persistence := config.NewMultiTenantPersistence(baseDir)
	orgID := "rollback-delete-org"

	if _, err := persistence.GetPersistence(orgID); err != nil {
		t.Fatalf("GetPersistence returned error: %v", err)
	}

	orgDir := filepath.Join(baseDir, "orgs", orgID)
	if _, err := os.Stat(orgDir); err != nil {
		t.Fatalf("expected org directory to exist before cleanup, stat error: %v", err)
	}

	p := &Provisioner{persistence: persistence}
	p.cleanupOrgDirectory(orgID, filepath.Join(baseDir, "wrong", "path"))

	if _, err := os.Stat(orgDir); !os.IsNotExist(err) {
		t.Fatalf("expected org directory to be removed by organization deleter, stat error: %v", err)
	}
}
