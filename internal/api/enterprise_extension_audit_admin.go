package api

import (
	"sync"

	"github.com/rcourtman/pulse-go-rewrite/pkg/extensions"
)

var (
	auditAdminBindMu          sync.RWMutex
	auditAdminEndpointsBinder extensions.BindAuditAdminEndpointsFunc
)

// SetAuditAdminEndpointsBinder registers a binder that can replace or decorate
// default audit admin endpoint handlers.
func SetAuditAdminEndpointsBinder(binder extensions.BindAuditAdminEndpointsFunc) {
	auditAdminBindMu.Lock()
	defer auditAdminBindMu.Unlock()
	auditAdminEndpointsBinder = binder
}

func resolveAuditAdminEndpoints(defaults extensions.AuditAdminEndpoints, runtime extensions.AuditAdminRuntime) extensions.AuditAdminEndpoints {
	auditAdminBindMu.RLock()
	binder := auditAdminEndpointsBinder
	auditAdminBindMu.RUnlock()

	if binder == nil || defaults == nil {
		return defaults
	}

	resolved := binder(defaults, runtime)
	if resolved == nil {
		return defaults
	}

	return resolved
}
