package extensions

import "net/http"

// RBACAdminEndpoints defines the enterprise RBAC admin endpoint surface.
// Implementations can replace or decorate the default handlers.
type RBACAdminEndpoints interface {
	HandleIntegrityCheck(http.ResponseWriter, *http.Request)
	HandleAdminReset(http.ResponseWriter, *http.Request)
}

// BindRBACAdminEndpointsFunc allows enterprise modules to bind replacement
// RBAC admin endpoints while retaining access to default handlers.
type BindRBACAdminEndpointsFunc func(defaults RBACAdminEndpoints) RBACAdminEndpoints
