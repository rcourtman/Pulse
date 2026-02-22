package extensions

import (
	"context"
	"errors"
	"net/http"
)

var (
	// ErrRBACUnavailable indicates RBAC infrastructure is not available.
	ErrRBACUnavailable = errors.New("rbac_unavailable")
)

// RBACIntegrityResult contains RBAC integrity status for an organization.
type RBACIntegrityResult struct {
	OrgID            string `json:"org_id"`
	DBAccessible     bool   `json:"db_accessible"`
	TablesPresent    bool   `json:"tables_present"`
	BuiltInRoleCount int    `json:"built_in_role_count"`
	TotalRoles       int    `json:"total_roles"`
	TotalAssignments int    `json:"total_assignments"`
	Healthy          bool   `json:"healthy"`
	Error            string `json:"error,omitempty"`
}

// RBACAdminEndpoints defines the enterprise RBAC admin endpoint surface.
// Implementations can replace or decorate the default handlers.
type RBACAdminEndpoints interface {
	HandleIntegrityCheck(http.ResponseWriter, *http.Request)
	HandleAdminReset(http.ResponseWriter, *http.Request)
}

// WriteRBACErrorFunc writes a structured RBAC error response.
type WriteRBACErrorFunc func(http.ResponseWriter, int, string, string, map[string]string)

// RBACAdminRuntime exposes API/runtime capabilities needed by RBAC admin endpoints.
// Implementations are provided by the public server and consumed by enterprise binders.
type RBACAdminRuntime struct {
	GetRequestOrgID       func(context.Context) string
	IsValidOrganizationID func(string) bool
	GetClientIP           func(*http.Request) string
	ValidateRecoveryToken func(token, clientIP string) bool
	VerifyIntegrity       func(orgID string) (RBACIntegrityResult, error)
	ResetAdminRole        func(orgID, username string) error
	WriteError            WriteRBACErrorFunc
}

// BindRBACAdminEndpointsFunc allows enterprise modules to bind replacement
// RBAC admin endpoints while retaining access to default handlers.
type BindRBACAdminEndpointsFunc func(defaults RBACAdminEndpoints, runtime RBACAdminRuntime) RBACAdminEndpoints
