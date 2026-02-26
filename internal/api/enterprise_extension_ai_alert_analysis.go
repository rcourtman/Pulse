package api

import (
	"sync"

	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
	"github.com/rcourtman/pulse-go-rewrite/pkg/extensions"
)

var (
	aiAlertAnalysisBindMu          sync.RWMutex
	aiAlertAnalysisEndpointsBinder extensions.BindAIAlertAnalysisEndpointsFunc

	createAlertAnalyzerFunc func(deps aicontracts.AlertAnalyzerDeps) aicontracts.AlertAnalyzer
)

// SetCreateAlertAnalyzer registers a factory that creates the enterprise
// AlertTriggeredAnalyzer. When nil (OSS binary), no alert analyzer is created.
func SetCreateAlertAnalyzer(fn func(deps aicontracts.AlertAnalyzerDeps) aicontracts.AlertAnalyzer) {
	aiAlertAnalysisBindMu.Lock()
	defer aiAlertAnalysisBindMu.Unlock()
	createAlertAnalyzerFunc = fn
}

func getCreateAlertAnalyzer() func(deps aicontracts.AlertAnalyzerDeps) aicontracts.AlertAnalyzer {
	aiAlertAnalysisBindMu.RLock()
	defer aiAlertAnalysisBindMu.RUnlock()
	return createAlertAnalyzerFunc
}

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
