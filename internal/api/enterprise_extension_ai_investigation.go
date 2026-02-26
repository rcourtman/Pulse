package api

import (
	"sync"

	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
)

var (
	aiInvestigationEnabledMu   sync.RWMutex
	aiInvestigationEnabledFunc func() bool

	// Factory hooks for premium component creation.
	aiFactoryMu                         sync.RWMutex
	createRemediationEngineFunc         func(cfg aicontracts.EngineConfig) aicontracts.RemediationEngine
	createInvestigationStoreFunc        func(dataDir string) aicontracts.InvestigationStore
	createInvestigationOrchestratorFunc func(deps aicontracts.OrchestratorDeps) aicontracts.InvestigationOrchestrator
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

// SetCreateRemediationEngine registers the enterprise factory for creating
// remediation engines. When set, router.go uses this factory.
func SetCreateRemediationEngine(fn func(cfg aicontracts.EngineConfig) aicontracts.RemediationEngine) {
	aiFactoryMu.Lock()
	defer aiFactoryMu.Unlock()
	createRemediationEngineFunc = fn
}

// getCreateRemediationEngine returns the registered factory, or nil.
func getCreateRemediationEngine() func(cfg aicontracts.EngineConfig) aicontracts.RemediationEngine {
	aiFactoryMu.RLock()
	defer aiFactoryMu.RUnlock()
	return createRemediationEngineFunc
}

// SetCreateInvestigationStore registers the enterprise factory for creating
// investigation stores. When set, ai_handlers.go uses this instead of
// directly constructing investigation.NewStore.
func SetCreateInvestigationStore(fn func(dataDir string) aicontracts.InvestigationStore) {
	aiFactoryMu.Lock()
	defer aiFactoryMu.Unlock()
	createInvestigationStoreFunc = fn
}

// getCreateInvestigationStore returns the registered factory, or nil.
func getCreateInvestigationStore() func(dataDir string) aicontracts.InvestigationStore {
	aiFactoryMu.RLock()
	defer aiFactoryMu.RUnlock()
	return createInvestigationStoreFunc
}

// SetCreateInvestigationOrchestrator registers the enterprise factory for creating
// investigation orchestrators. When set, ai_handlers.go uses this instead of
// directly constructing investigation.NewOrchestrator.
func SetCreateInvestigationOrchestrator(fn func(deps aicontracts.OrchestratorDeps) aicontracts.InvestigationOrchestrator) {
	aiFactoryMu.Lock()
	defer aiFactoryMu.Unlock()
	createInvestigationOrchestratorFunc = fn
}

// getCreateInvestigationOrchestrator returns the registered factory, or nil.
func getCreateInvestigationOrchestrator() func(deps aicontracts.OrchestratorDeps) aicontracts.InvestigationOrchestrator {
	aiFactoryMu.RLock()
	defer aiFactoryMu.RUnlock()
	return createInvestigationOrchestratorFunc
}
