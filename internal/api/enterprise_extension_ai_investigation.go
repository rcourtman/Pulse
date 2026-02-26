package api

import "sync"

var (
	aiInvestigationEnabledMu   sync.RWMutex
	aiInvestigationEnabledFunc func() bool
)

// SetAIInvestigationEnabled registers the enterprise hook that controls
// whether premium AI investigation/remediation components are created.
func SetAIInvestigationEnabled(fn func() bool) {
	aiInvestigationEnabledMu.Lock()
	defer aiInvestigationEnabledMu.Unlock()
	aiInvestigationEnabledFunc = fn
}

// isAIInvestigationEnabled returns true if the enterprise hook is registered
// and returns true, meaning premium components should be created.
func isAIInvestigationEnabled() bool {
	aiInvestigationEnabledMu.RLock()
	defer aiInvestigationEnabledMu.RUnlock()
	return aiInvestigationEnabledFunc != nil && aiInvestigationEnabledFunc()
}
