package main

import (
	"sort"
	"sync"
	"sync/atomic"
	"time"

	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

const (
	moduleStateDisabled = "disabled"
	moduleStateStarting = "starting"
	moduleStateRetrying = "retrying"
	moduleStateRunning  = "running"
)

type moduleHealth struct {
	Name      string    `json:"name"`
	Enabled   bool      `json:"enabled"`
	State     string    `json:"state"`
	LastError string    `json:"lastError,omitempty"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type runtimeHealthSnapshot struct {
	Ready   bool           `json:"ready"`
	Modules []moduleHealth `json:"modules"`
}

type runtimeHealth struct {
	mu      sync.RWMutex
	ready   *atomic.Bool
	modules map[string]moduleHealth
	now     func() time.Time
}

func newRuntimeHealth(ready *atomic.Bool, enabled map[string]bool) *runtimeHealth {
	r := &runtimeHealth{
		ready:   ready,
		modules: make(map[string]moduleHealth, len(enabled)),
		now:     func() time.Time { return time.Now().UTC() },
	}
	for name, isEnabled := range enabled {
		state := moduleStateDisabled
		if isEnabled {
			state = moduleStateStarting
		}
		r.modules[name] = moduleHealth{
			Name:      name,
			Enabled:   isEnabled,
			State:     state,
			UpdatedAt: r.now(),
		}
		agentModuleEnabled.WithLabelValues(name).Set(boolGauge(isEnabled))
	}
	r.reconcileReadyLocked()
	return r
}

func (r *runtimeHealth) setEnabled(name string, enabled bool) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	state := moduleStateDisabled
	if enabled {
		state = moduleStateStarting
	}
	r.modules[name] = moduleHealth{Name: name, Enabled: enabled, State: state, UpdatedAt: r.now()}
	agentModuleEnabled.WithLabelValues(name).Set(boolGauge(enabled))
	r.reconcileReadyLocked()
}

func (r *runtimeHealth) setState(name, state string, err error) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	module := r.modules[name]
	module.Name = name
	module.Enabled = state != moduleStateDisabled
	module.State = state
	module.LastError = ""
	if err != nil {
		module.LastError = err.Error()
	}
	module.UpdatedAt = r.now()
	r.modules[name] = module
	r.reconcileReadyLocked()
}

func (r *runtimeHealth) reconcileReadyLocked() {
	ready := true
	for _, module := range r.modules {
		moduleReady := module.Enabled && module.State == moduleStateRunning
		agentModuleReady.WithLabelValues(module.Name).Set(boolGauge(moduleReady))
		if module.Enabled && module.State != moduleStateRunning {
			ready = false
		}
	}
	if r.ready != nil {
		r.ready.Store(ready)
	}
}

func boolGauge(value bool) float64 {
	if value {
		return 1
	}
	return 0
}

func (r *runtimeHealth) snapshot() runtimeHealthSnapshot {
	if r == nil {
		return runtimeHealthSnapshot{}
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	modules := make([]moduleHealth, 0, len(r.modules))
	ready := true
	for _, module := range r.modules {
		modules = append(modules, module)
		if module.Enabled && module.State != moduleStateRunning {
			ready = false
		}
	}
	sort.Slice(modules, func(i, j int) bool { return modules[i].Name < modules[j].Name })
	return runtimeHealthSnapshot{Ready: ready, Modules: modules}
}

func (r *runtimeHealth) moduleStatuses() []agentshost.ModuleStatus {
	snapshot := r.snapshot()
	statuses := make([]agentshost.ModuleStatus, 0, len(snapshot.Modules))
	for _, module := range snapshot.Modules {
		statuses = append(statuses, agentshost.ModuleStatus{
			Name:      module.Name,
			Enabled:   module.Enabled,
			State:     module.State,
			LastError: module.LastError,
			UpdatedAt: module.UpdatedAt,
		})
	}
	return statuses
}
