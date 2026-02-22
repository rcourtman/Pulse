package api

import (
	"sync"

	"github.com/rcourtman/pulse-go-rewrite/pkg/extensions"
)

var (
	ssoAdminBindMu          sync.RWMutex
	ssoAdminEndpointsBinder extensions.BindSSOAdminEndpointsFunc
)

// SetSSOAdminEndpointsBinder registers a binder that can replace or decorate
// default SSO admin endpoint handlers.
func SetSSOAdminEndpointsBinder(binder extensions.BindSSOAdminEndpointsFunc) {
	ssoAdminBindMu.Lock()
	defer ssoAdminBindMu.Unlock()
	ssoAdminEndpointsBinder = binder
}

func resolveSSOAdminEndpoints(defaults extensions.SSOAdminEndpoints, runtime extensions.SSOAdminRuntime) extensions.SSOAdminEndpoints {
	ssoAdminBindMu.RLock()
	binder := ssoAdminEndpointsBinder
	ssoAdminBindMu.RUnlock()

	if binder == nil || defaults == nil {
		return defaults
	}

	resolved := binder(defaults, runtime)
	if resolved == nil {
		return defaults
	}

	return resolved
}
