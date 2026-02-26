package api

import (
	"sync"

	"github.com/rcourtman/pulse-go-rewrite/pkg/extensions"
)

var (
	aiAlertAnalysisBindMu          sync.RWMutex
	aiAlertAnalysisEndpointsBinder extensions.BindAIAlertAnalysisEndpointsFunc
)

// SetAIAlertAnalysisEndpointsBinder registers a binder that can replace or decorate
// default AI alert analysis endpoint handlers.
func SetAIAlertAnalysisEndpointsBinder(binder extensions.BindAIAlertAnalysisEndpointsFunc) {
	aiAlertAnalysisBindMu.Lock()
	defer aiAlertAnalysisBindMu.Unlock()
	aiAlertAnalysisEndpointsBinder = binder
}

func resolveAIAlertAnalysisEndpoints(defaults extensions.AIAlertAnalysisEndpoints, runtime extensions.AIAlertAnalysisRuntime) extensions.AIAlertAnalysisEndpoints {
	aiAlertAnalysisBindMu.RLock()
	binder := aiAlertAnalysisEndpointsBinder
	aiAlertAnalysisBindMu.RUnlock()

	if binder == nil || defaults == nil {
		return defaults
	}

	resolved := binder(defaults, runtime)
	if resolved == nil {
		return defaults
	}

	return resolved
}
