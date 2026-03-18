package hosted

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"github.com/rs/zerolog/log"
)

const (
	ProvisionStatusCreated  = "created"
	ProvisionStatusExisting = "existing"

	maxHostedOrganizationIDLength = 64
	maxHostedSignupEmailLength    = 254
	maxHostedSignupPasswordLength = 1024
)

type ProvisionStatus string

type OrgPersistence interface {
	GetPersistence(orgID string) (*config.ConfigPersistence, error)
	SaveOrganization(org *models.Organization) error
	LoadOrganization(orgID string) (*models.Organization, error)
	ListOrganizations() ([]*models.Organization, error)
}

type AuthProvider interface {
	GetManager(orgID string) (AuthManager, error)
}

type AuthManager interface {
	UpdateUserRoles(userID string, roles []string) error
}

type orgRollbackDeleter interface {
	DeleteOrganization(orgID string) error
}

type authRollbackDeleter interface {
	RemoveTenant(orgID string) error
}

type Provisioner struct {
	persistence  OrgPersistence
	authProvider AuthProvider
	newOrgID     func() string
	now          func() time.Time
}

type ProvisionRequest struct {
	Email    string
	Password string
	OrgName  string
}

type HostedSignupRequest struct {
	Email   string
	OrgName string
}

type ProvisionResult struct {
	OrgID  string
	UserID string
	Status ProvisionStatus
}

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation failed for %s: %s", e.Field, e.Message)
}

type SystemError struct {
	Op  string
	Err error
}

func (e *SystemError) Error() string {
	if e.Op == "" {
		return fmt.Sprintf("system error: %v", e.Err)
	}
	return fmt.Sprintf("system error in %s: %v", e.Op, e.Err)
}

func (e *SystemError) Unwrap() error {
	return e.Err
}

func IsValidationError(err error) bool {
	var target *ValidationError
	return errors.As(err, &target)
}

func IsSystemError(err error) bool {
	var target *SystemError
	return errors.As(err, &target)
}

func NewProvisioner(persistence OrgPersistence, authProvider AuthProvider) *Provisioner {
	return &Provisioner{
		persistence:  persistence,
		authProvider: authProvider,
		newOrgID:     uuid.NewString,
		now:          time.Now,
	}
}

func (p *Provisioner) ProvisionTenant(ctx context.Context, req ProvisionRequest) (*ProvisionResult, error) {
	if err := p.ensureReady(); err != nil {
		return nil, err
	}

	req = normalizeProvisionRequest(req)
	if err := validateProvisionRequest(req); err != nil {
		return nil, err
	}
	if err := contextErr(ctx); err != nil {
		return nil, err
	}

	orgs, err := p.persistence.ListOrganizations()
	if err != nil {
		return nil, &SystemError{Op: "list_organizations", Err: err}
	}
	for _, org := range orgs {
		if org == nil {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(org.OwnerUserID), req.Email) {
			return &ProvisionResult{
				OrgID:  org.ID,
				UserID: req.Email,
				Status: ProvisionStatusExisting,
			}, nil
		}
	}

	orgID := p.newOrgID()
	return p.createOrganization(ctx, orgID, req.Email, req.OrgName)
}

func (p *Provisioner) ProvisionHostedSignup(ctx context.Context, req HostedSignupRequest) (*ProvisionResult, error) {
	if err := p.ensureReady(); err != nil {
		return nil, err
	}

	req = normalizeHostedSignupRequest(req)
	if err := validateHostedSignupRequest(req); err != nil {
		return nil, err
	}
	if err := contextErr(ctx); err != nil {
		return nil, err
	}

	return p.createOrganization(ctx, p.newOrgID(), req.Email, req.OrgName)
}

func (p *Provisioner) RollbackProvisioning(orgID string) {
	if p == nil {
		return
	}

	if remover, ok := p.authProvider.(authRollbackDeleter); ok && remover != nil {
		if err := remover.RemoveTenant(orgID); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Warn().
				Err(err).
				Str("org_id", orgID).
				Msg("Hosted tenant rollback: failed to remove auth tenant")
		}
	}

	if deleter, ok := p.persistence.(orgRollbackDeleter); ok && deleter != nil {
		p.cleanupOrgDirectory(orgID, "")
		return
	}

	tenantPersistence, err := p.persistence.GetPersistence(orgID)
	if err != nil || tenantPersistence == nil {
		log.Warn().
			Err(err).
			Str("org_id", orgID).
			Msg("Hosted tenant rollback: failed to resolve tenant persistence")
		return
	}

	p.cleanupOrgDirectory(orgID, tenantPersistence.DataDir())
}

func (p *Provisioner) ensureReady() error {
	if p == nil {
		return &SystemError{Op: "initialize_provisioner", Err: errors.New("provisioner is nil")}
	}
	if p.persistence == nil {
		return &SystemError{Op: "initialize_provisioner", Err: errors.New("org persistence is nil")}
	}
	if p.authProvider == nil {
		return &SystemError{Op: "initialize_provisioner", Err: errors.New("auth provider is nil")}
	}
	return nil
}

func (p *Provisioner) createOrganization(ctx context.Context, orgID, userID, orgName string) (*ProvisionResult, error) {
	tenantPersistence, err := p.persistence.GetPersistence(orgID)
	if err != nil {
		return nil, &SystemError{Op: "initialize_tenant_directory", Err: err}
	}
	if tenantPersistence == nil {
		return nil, &SystemError{Op: "initialize_tenant_directory", Err: errors.New("tenant persistence is nil")}
	}
	if err := contextErr(ctx); err != nil {
		p.RollbackProvisioning(orgID)
		return nil, err
	}

	now := p.now().UTC()
	org := &models.Organization{
		ID:          orgID,
		DisplayName: orgName,
		CreatedAt:   now,
		OwnerUserID: userID,
		Members: []models.OrganizationMember{
			{
				UserID:  userID,
				Role:    models.OrgRoleOwner,
				AddedAt: now,
				AddedBy: userID,
			},
		},
	}
	if err := p.persistence.SaveOrganization(org); err != nil {
		p.RollbackProvisioning(orgID)
		return nil, &SystemError{Op: "save_organization", Err: err}
	}
	if err := contextErr(ctx); err != nil {
		p.RollbackProvisioning(orgID)
		return nil, err
	}

	authManager, err := p.authProvider.GetManager(orgID)
	if err != nil {
		p.RollbackProvisioning(orgID)
		return nil, &SystemError{Op: "get_auth_manager", Err: err}
	}
	if authManager == nil {
		p.RollbackProvisioning(orgID)
		return nil, &SystemError{Op: "get_auth_manager", Err: errors.New("auth manager is nil")}
	}
	if err := authManager.UpdateUserRoles(userID, []string{auth.RoleAdmin}); err != nil {
		p.RollbackProvisioning(orgID)
		return nil, &SystemError{Op: "create_admin_user", Err: err}
	}

	return &ProvisionResult{
		OrgID:  orgID,
		UserID: userID,
		Status: ProvisionStatusCreated,
	}, nil
}

func (p *Provisioner) cleanupOrgDirectory(orgID, dataDir string) {
	log.Warn().
		Str("org_id", orgID).
		Str("data_dir", dataDir).
		Msg("Hosted tenant provisioning failed; attempting rollback cleanup")

	if !isValidOrganizationID(orgID) || orgID == "default" {
		log.Warn().
			Str("org_id", orgID).
			Msg("Skipping rollback cleanup because organization ID is invalid for deletion")
		return
	}

	if deleter, ok := p.persistence.(orgRollbackDeleter); ok && deleter != nil {
		if err := deleter.DeleteOrganization(orgID); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Error().
				Err(err).
				Str("org_id", orgID).
				Msg("Rollback cleanup failed via organization deleter")
			return
		}
		log.Info().
			Str("org_id", orgID).
			Msg("Rollback cleanup completed via organization deleter")
		return
	}

	if !isSafeTenantDataDir(dataDir, orgID) {
		log.Warn().
			Str("org_id", orgID).
			Str("data_dir", dataDir).
			Msg("Skipping rollback cleanup because data directory does not match expected tenant path")
		return
	}

	cleanDataDir := filepath.Clean(dataDir)
	if err := os.RemoveAll(cleanDataDir); err != nil {
		log.Error().
			Err(err).
			Str("org_id", orgID).
			Str("data_dir", cleanDataDir).
			Msg("Rollback cleanup failed")
		return
	}

	log.Info().
		Str("org_id", orgID).
		Str("data_dir", cleanDataDir).
		Msg("Rollback cleanup completed")
}

func isSafeTenantDataDir(dataDir, orgID string) bool {
	if dataDir == "" || !isValidOrganizationID(orgID) || orgID == "default" {
		return false
	}

	cleanDataDir := filepath.Clean(dataDir)
	if cleanDataDir == "." || cleanDataDir == string(os.PathSeparator) {
		return false
	}
	if filepath.Base(cleanDataDir) != orgID {
		return false
	}
	if filepath.Base(filepath.Dir(cleanDataDir)) != "orgs" {
		return false
	}

	return true
}

func validateProvisionRequest(req ProvisionRequest) error {
	if err := validateHostedSignupRequest(HostedSignupRequest{
		Email:   req.Email,
		OrgName: req.OrgName,
	}); err != nil {
		return err
	}
	if len(req.Password) < 8 {
		return &ValidationError{Field: "password", Message: "password must be at least 8 characters"}
	}
	if len(req.Password) > maxHostedSignupPasswordLength {
		return &ValidationError{Field: "password", Message: "password exceeds maximum length"}
	}
	return nil
}

func validateHostedSignupRequest(req HostedSignupRequest) error {
	if !isValidSignupEmail(req.Email) {
		return &ValidationError{Field: "email", Message: "invalid email format"}
	}
	if !isValidHostedOrgName(req.OrgName) {
		return &ValidationError{Field: "org_name", Message: "invalid organization name"}
	}
	return nil
}

func normalizeProvisionRequest(req ProvisionRequest) ProvisionRequest {
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.OrgName = strings.TrimSpace(req.OrgName)
	return req
}

func normalizeHostedSignupRequest(req HostedSignupRequest) HostedSignupRequest {
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.OrgName = strings.TrimSpace(req.OrgName)
	return req
}

func isValidSignupEmail(email string) bool {
	if email == "" || len(email) > maxHostedSignupEmailLength || strings.TrimSpace(email) != email {
		return false
	}
	for _, r := range email {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	at := strings.Index(email, "@")
	if at <= 0 || at >= len(email)-1 {
		return false
	}
	domain := email[at+1:]
	if strings.Contains(domain, "@") {
		return false
	}
	dot := strings.Index(domain, ".")
	if dot <= 0 || dot >= len(domain)-1 {
		return false
	}
	return true
}

func isValidHostedOrgName(orgName string) bool {
	if len(orgName) < 3 || len(orgName) > maxHostedOrganizationIDLength {
		return false
	}
	return isValidOrganizationID(orgName)
}

func isValidOrganizationID(orgID string) bool {
	if orgID == "" || orgID == "." || orgID == ".." {
		return false
	}
	if len(orgID) > maxHostedOrganizationIDLength {
		return false
	}
	if strings.TrimSpace(orgID) != orgID {
		return false
	}
	if strings.ContainsAny(orgID, `/\`) {
		return false
	}
	if filepath.Base(orgID) != orgID {
		return false
	}
	for _, r := range orgID {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

func contextErr(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return &SystemError{Op: "context", Err: err}
	}
	return nil
}

type tenantRBACProvider interface {
	GetManager(orgID string) (auth.ExtendedManager, error)
}

type tenantRBACAdapter struct {
	provider tenantRBACProvider
}

func NewTenantRBACAuthProvider(provider tenantRBACProvider) AuthProvider {
	return &tenantRBACAdapter{provider: provider}
}

func (a *tenantRBACAdapter) GetManager(orgID string) (AuthManager, error) {
	if a == nil || a.provider == nil {
		return nil, errors.New("tenant RBAC provider is nil")
	}
	return a.provider.GetManager(orgID)
}

func (a *tenantRBACAdapter) RemoveTenant(orgID string) error {
	if a == nil || a.provider == nil {
		return errors.New("tenant RBAC provider is nil")
	}
	remover, ok := a.provider.(authRollbackDeleter)
	if !ok || remover == nil {
		return nil
	}
	return remover.RemoveTenant(orgID)
}
