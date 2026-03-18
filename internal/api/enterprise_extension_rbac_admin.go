package api

import (
	"sync"

	"github.com/rcourtman/pulse-go-rewrite/pkg/extensions"
)

var (
	rbacAdminBindMu          sync.RWMutex
	rbacAdminEndpointsBinder extensions.BindRBACAdminEndpointsFunc
)

// SetRBACAdminEndpointsBinder registers a binder that can replace or decorate
// default RBAC admin endpoint handlers.
func SetRBACAdminEndpointsBinder(binder extensions.BindRBACAdminEndpointsFunc) {
	rbacAdminBindMu.Lock()
	defer rbacAdminBindMu.Unlock()
	rbacAdminEndpointsBinder = binder
}

func resolveRBACAdminEndpoints(defaults extensions.RBACAdminEndpoints, runtime extensions.RBACAdminRuntime) extensions.RBACAdminEndpoints {
	rbacAdminBindMu.RLock()
	binder := rbacAdminEndpointsBinder
	rbacAdminBindMu.RUnlock()

	if binder == nil || defaults == nil {
		return defaults
	}

	resolved := binder(defaults, runtime)
	if resolved == nil {
		return defaults
	}

	return resolved
}
