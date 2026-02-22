package api

import (
	"sync"

	"github.com/rcourtman/pulse-go-rewrite/pkg/extensions"
)

var (
	reportingAdminBindMu          sync.RWMutex
	reportingAdminEndpointsBinder extensions.BindReportingAdminEndpointsFunc
)

// SetReportingAdminEndpointsBinder registers a binder that can replace or decorate
// default reporting admin endpoint handlers.
func SetReportingAdminEndpointsBinder(binder extensions.BindReportingAdminEndpointsFunc) {
	reportingAdminBindMu.Lock()
	defer reportingAdminBindMu.Unlock()
	reportingAdminEndpointsBinder = binder
}

func resolveReportingAdminEndpoints(defaults extensions.ReportingAdminEndpoints, runtime extensions.ReportingAdminRuntime) extensions.ReportingAdminEndpoints {
	reportingAdminBindMu.RLock()
	binder := reportingAdminEndpointsBinder
	reportingAdminBindMu.RUnlock()

	if binder == nil || defaults == nil {
		return defaults
	}

	resolved := binder(defaults, runtime)
	if resolved == nil {
		return defaults
	}

	return resolved
}
