package hosted

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

type stubOrgPersistence struct {
	listOrganizationsFn func() ([]*models.Organization, error)
	getPersistenceFn    func(orgID string) (*config.ConfigPersistence, error)
	saveOrganizationFn  func(org *models.Organization) error
	loadOrganizationFn  func(orgID string) (*models.Organization, error)
}

func (s *stubOrgPersistence) ListOrganizations() ([]*models.Organization, error) {
	if s.listOrganizationsFn == nil {
		return nil, nil
	}
	return s.listOrganizationsFn()
}

func (s *stubOrgPersistence) GetPersistence(orgID string) (*config.ConfigPersistence, error) {
	if s.getPersistenceFn == nil {
		return nil, nil
	}
	return s.getPersistenceFn(orgID)
}

func (s *stubOrgPersistence) SaveOrganization(org *models.Organization) error {
	if s.saveOrganizationFn == nil {
		return nil
	}
	return s.saveOrganizationFn(org)
}

func (s *stubOrgPersistence) LoadOrganization(orgID string) (*models.Organization, error) {
	if s.loadOrganizationFn == nil {
		return nil, nil
	}
	return s.loadOrganizationFn(orgID)
}

type stubTenantRBACProvider struct {
	manager auth.ExtendedManager
	err     error
	calls   int
	lastOrg string
}

func (s *stubTenantRBACProvider) GetManager(orgID string) (auth.ExtendedManager, error) {
	s.calls++
	s.lastOrg = orgID
	if s.err != nil {
		return nil, s.err
	}
	return s.manager, nil
}

func TestProvisionerErrorHelpers(t *testing.T) {
	baseErr := errors.New("boom")
	validationErr := &ValidationError{Field: "email", Message: "invalid"}
	systemErrNoOp := &SystemError{Err: baseErr}
	systemErrWithOp := &SystemError{Op: "save_organization", Err: baseErr}

	if got := validationErr.Error(); got != "validation failed for email: invalid" {
		t.Fatalf("unexpected validation error string: %q", got)
	}
	if got := systemErrNoOp.Error(); got != "system error: boom" {
		t.Fatalf("unexpected system error string without op: %q", got)
	}
	if got := systemErrWithOp.Error(); got != "system error in save_organization: boom" {
		t.Fatalf("unexpected system error string with op: %q", got)
	}
	if !errors.Is(systemErrWithOp, baseErr) {
		t.Fatalf("expected SystemError to unwrap to base error")
	}

	wrappedValidation := fmt.Errorf("wrapped: %w", validationErr)
	if !IsValidationError(wrappedValidation) {
		t.Fatalf("expected IsValidationError to return true for wrapped ValidationError")
	}
	if IsSystemError(wrappedValidation) {
		t.Fatalf("expected IsSystemError to return false for wrapped ValidationError")
	}

	wrappedSystem := fmt.Errorf("wrapped: %w", systemErrWithOp)
	if !IsSystemError(wrappedSystem) {
		t.Fatalf("expected IsSystemError to return true for wrapped SystemError")
	}
	if IsValidationError(wrappedSystem) {
		t.Fatalf("expected IsValidationError to return false for wrapped SystemError")
	}
}

func TestContextErrBranches(t *testing.T) {
	if err := contextErr(nil); err != nil {
		t.Fatalf("expected nil for nil context, got %v", err)
	}

	activeCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := contextErr(activeCtx); err != nil {
		t.Fatalf("expected nil for active context, got %v", err)
	}

	doneCtx, doneCancel := context.WithCancel(context.Background())
	doneCancel()
	err := contextErr(doneCtx)
	if err == nil {
		t.Fatal("expected error for canceled context, got nil")
	}
	if !IsSystemError(err) {
		t.Fatalf("expected system error for canceled context, got %T", err)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled context error, got %v", err)
	}
}

func TestProvisionTenantReturnsSystemErrorsForInvalidProvisionerState(t *testing.T) {
	validReq := ProvisionRequest{
		Email:    "owner@example.com",
		Password: "securepass123",
		OrgName:  "Valid Org",
	}

	testCases := []struct {
		name      string
		call      func() error
		expectOp  string
		expectMsg string
	}{
		{
			name: "nil receiver",
			call: func() error {
				var p *Provisioner
				_, err := p.ProvisionTenant(context.Background(), validReq)
				return err
			},
			expectOp:  "initialize_provisioner",
			expectMsg: "provisioner is nil",
		},
		{
			name: "nil persistence",
			call: func() error {
				p := &Provisioner{authProvider: &mockAuthProvider{manager: &mockAuthManager{}}}
				_, err := p.ProvisionTenant(context.Background(), validReq)
				return err
			},
			expectOp:  "initialize_provisioner",
			expectMsg: "org persistence is nil",
		},
		{
			name: "nil auth provider",
			call: func() error {
				p := &Provisioner{persistence: &stubOrgPersistence{}}
				_, err := p.ProvisionTenant(context.Background(), validReq)
				return err
			},
			expectOp:  "initialize_provisioner",
			expectMsg: "auth provider is nil",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call()
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			var systemErr *SystemError
			if !errors.As(err, &systemErr) {
				t.Fatalf("expected SystemError, got %T (%v)", err, err)
			}
			if systemErr.Op != tc.expectOp {
				t.Fatalf("expected op %q, got %q", tc.expectOp, systemErr.Op)
			}
			if systemErr.Err == nil || systemErr.Err.Error() != tc.expectMsg {
				t.Fatalf("expected underlying message %q, got %v", tc.expectMsg, systemErr.Err)
			}
		})
	}
}

func TestProvisionTenantListOrganizationsAndPersistenceErrors(t *testing.T) {
	listErr := errors.New("list failed")

	p := &Provisioner{
		persistence: &stubOrgPersistence{
			listOrganizationsFn: func() ([]*models.Organization, error) { return nil, listErr },
		},
		authProvider: &mockAuthProvider{manager: &mockAuthManager{}},
		newOrgID:     func() string { return "ignored" },
	}

	_, err := p.ProvisionTenant(context.Background(), ProvisionRequest{
		Email:    "owner@example.com",
		Password: "securepass123",
		OrgName:  "Valid Org",
	})
	if err == nil {
		t.Fatal("expected list error, got nil")
	}
	var systemErr *SystemError
	if !errors.As(err, &systemErr) {
		t.Fatalf("expected SystemError, got %T (%v)", err, err)
	}
	if systemErr.Op != "list_organizations" {
		t.Fatalf("expected op list_organizations, got %q", systemErr.Op)
	}
	if !errors.Is(err, listErr) {
		t.Fatalf("expected wrapped list error, got %v", err)
	}

	p.persistence = &stubOrgPersistence{
		listOrganizationsFn: func() ([]*models.Organization, error) { return nil, nil },
		getPersistenceFn:    func(orgID string) (*config.ConfigPersistence, error) { return nil, nil },
	}

	_, err = p.ProvisionTenant(context.Background(), ProvisionRequest{
		Email:    "owner@example.com",
		Password: "securepass123",
		OrgName:  "Valid Org",
	})
	if err == nil {
		t.Fatal("expected tenant persistence nil error, got nil")
	}
	if !errors.As(err, &systemErr) {
		t.Fatalf("expected SystemError, got %T (%v)", err, err)
	}
	if systemErr.Op != "initialize_tenant_directory" {
		t.Fatalf("expected op initialize_tenant_directory, got %q", systemErr.Op)
	}
	if systemErr.Err == nil || systemErr.Err.Error() != "tenant persistence is nil" {
		t.Fatalf("unexpected underlying error: %v", systemErr.Err)
	}
}

func TestProvisionerCleanupOrgDirectoryNoopForEmptyPath(t *testing.T) {
	p := &Provisioner{}
	p.cleanupOrgDirectory("org-empty", "")
}

func TestProvisionTenantGetPersistenceErrorWrapped(t *testing.T) {
	getPersistenceErr := errors.New("persistence unavailable")
	p := &Provisioner{
		persistence: &stubOrgPersistence{
			listOrganizationsFn: func() ([]*models.Organization, error) { return nil, nil },
			getPersistenceFn:    func(orgID string) (*config.ConfigPersistence, error) { return nil, getPersistenceErr },
		},
		authProvider: &mockAuthProvider{manager: &mockAuthManager{}},
		newOrgID:     func() string { return "org-get-persistence-error" },
	}

	_, err := p.ProvisionTenant(context.Background(), ProvisionRequest{
		Email:    "owner@example.com",
		Password: "securepass123",
		OrgName:  "Valid Org",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var systemErr *SystemError
	if !errors.As(err, &systemErr) {
		t.Fatalf("expected SystemError, got %T (%v)", err, err)
	}
	if systemErr.Op != "initialize_tenant_directory" {
		t.Fatalf("expected op initialize_tenant_directory, got %q", systemErr.Op)
	}
	if !errors.Is(err, getPersistenceErr) {
		t.Fatalf("expected wrapped get persistence error %v, got %v", getPersistenceErr, err)
	}
}

func TestProvisionTenantContextCanceledAfterTenantInitCleansUp(t *testing.T) {
	baseDir := t.TempDir()
	tenantPersistence := config.NewMultiTenantPersistence(baseDir)
	authProvider := &mockAuthProvider{manager: &mockAuthManager{}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	saveCalls := 0
	p := &Provisioner{
		persistence: &stubOrgPersistence{
			listOrganizationsFn: func() ([]*models.Organization, error) {
				// Include a nil org to exercise the skip path while scanning existing tenants.
				return []*models.Organization{nil}, nil
			},
			getPersistenceFn: func(orgID string) (*config.ConfigPersistence, error) {
				cp, err := tenantPersistence.GetPersistence(orgID)
				if err != nil {
					return nil, err
				}
				cancel()
				return cp, nil
			},
			saveOrganizationFn: func(org *models.Organization) error {
				saveCalls++
				return nil
			},
		},
		authProvider: authProvider,
		newOrgID:     func() string { return "org-context-cancel" },
	}

	_, err := p.ProvisionTenant(ctx, ProvisionRequest{
		Email:    " owner@example.com ",
		Password: "securepass123",
		OrgName:  " Valid Org ",
	})
	if err == nil {
		t.Fatal("expected context error, got nil")
	}

	var systemErr *SystemError
	if !errors.As(err, &systemErr) {
		t.Fatalf("expected SystemError, got %T (%v)", err, err)
	}
	if systemErr.Op != "context" {
		t.Fatalf("expected op context, got %q", systemErr.Op)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled error, got %v", err)
	}
	if saveCalls != 0 {
		t.Fatalf("expected no SaveOrganization calls after cancellation, got %d", saveCalls)
	}
	if authProvider.calls != 0 {
		t.Fatalf("expected auth provider not called after cancellation, got %d", authProvider.calls)
	}

	orgDir := filepath.Join(baseDir, "orgs", "org-context-cancel")
	if _, statErr := os.Stat(orgDir); !os.IsNotExist(statErr) {
		t.Fatalf("expected org dir to be removed on rollback, stat error: %v", statErr)
	}
}

func TestProvisionTenantAuthManagerFailuresCleanup(t *testing.T) {
	authManagerErr := errors.New("manager unavailable")
	testCases := []struct {
		name        string
		authProvider *mockAuthProvider
		expectError error
		expectMsg   string
	}{
		{
			name:         "provider error",
			authProvider: &mockAuthProvider{err: authManagerErr},
			expectError:  authManagerErr,
		},
		{
			name:         "nil manager",
			authProvider: &mockAuthProvider{},
			expectMsg:    "auth manager is nil",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			baseDir := t.TempDir()
			p := NewProvisioner(config.NewMultiTenantPersistence(baseDir), tc.authProvider)
			p.newOrgID = func() string { return "org-auth-failure" }

			_, err := p.ProvisionTenant(context.Background(), ProvisionRequest{
				Email:    "owner@example.com",
				Password: "securepass123",
				OrgName:  "Valid Org",
			})
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			var systemErr *SystemError
			if !errors.As(err, &systemErr) {
				t.Fatalf("expected SystemError, got %T (%v)", err, err)
			}
			if systemErr.Op != "get_auth_manager" {
				t.Fatalf("expected op get_auth_manager, got %q", systemErr.Op)
			}
			if tc.expectError != nil && !errors.Is(err, tc.expectError) {
				t.Fatalf("expected wrapped error %v, got %v", tc.expectError, err)
			}
			if tc.expectMsg != "" && (systemErr.Err == nil || systemErr.Err.Error() != tc.expectMsg) {
				t.Fatalf("expected message %q, got %v", tc.expectMsg, systemErr.Err)
			}

			orgDir := filepath.Join(baseDir, "orgs", "org-auth-failure")
			if _, statErr := os.Stat(orgDir); !os.IsNotExist(statErr) {
				t.Fatalf("expected org dir to be removed on rollback, stat error: %v", statErr)
			}
		})
	}
}

func TestValidationHelpersAdditionalBranches(t *testing.T) {
	if isValidSignupEmail("user@bad@domain.com") {
		t.Fatal("expected second @ in domain to be invalid")
	}
	if isValidOrganizationID(".") {
		t.Fatal("expected dot org ID to be invalid")
	}
	if isValidOrganizationID("..") {
		t.Fatal("expected dot-dot org ID to be invalid")
	}
}

func TestTenantRBACAdapterGetManager(t *testing.T) {
	testErr := errors.New("provider failed")

	provider := &stubTenantRBACProvider{}
	authProvider := NewTenantRBACAuthProvider(provider)

	manager, err := authProvider.GetManager("org-1")
	if err != nil {
		t.Fatalf("expected nil error from provider, got %v", err)
	}
	if manager != nil {
		t.Fatalf("expected nil manager passthrough, got %T", manager)
	}
	if provider.calls != 1 || provider.lastOrg != "org-1" {
		t.Fatalf("expected provider called once for org-1, got calls=%d org=%q", provider.calls, provider.lastOrg)
	}

	provider.err = testErr
	_, err = authProvider.GetManager("org-2")
	if !errors.Is(err, testErr) {
		t.Fatalf("expected provider error %v, got %v", testErr, err)
	}

	var nilAdapter *tenantRBACAdapter
	if _, err := nilAdapter.GetManager("org-3"); err == nil {
		t.Fatal("expected nil adapter error, got nil")
	}

	adapterWithoutProvider := &tenantRBACAdapter{}
	if _, err := adapterWithoutProvider.GetManager("org-4"); err == nil {
		t.Fatal("expected nil provider error, got nil")
	}
}
