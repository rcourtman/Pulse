package hypervisor

import (
	"context"
	"fmt"
	"sync"

	"github.com/rs/zerolog/log"
)

// Registry manages all registered hypervisor/cloud providers.
// It is safe for concurrent use.
type Registry struct {
	mu        sync.RWMutex
	providers map[string]Provider
}

// NewRegistry creates a new empty provider registry.
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]Provider),
	}
}

// Register adds a provider to the registry. If a provider with the same ID
// already exists, the old one is closed and replaced.
func (r *Registry) Register(p Provider) error {
	if p == nil {
		return fmt.Errorf("cannot register nil provider")
	}
	id := p.ID()
	if id == "" {
		return fmt.Errorf("provider ID must not be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Close existing provider with the same ID if any.
	if old, ok := r.providers[id]; ok {
		log.Info().Str("id", id).Str("type", string(old.Type())).Msg("replacing existing provider")
		_ = old.Close()
	}

	r.providers[id] = p
	log.Info().Str("id", id).Str("type", string(p.Type())).Str("name", p.Name()).Msg("provider registered")
	return nil
}

// Get returns a provider by ID.
func (r *Registry) Get(id string) (Provider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[id]
	return p, ok
}

// All returns a snapshot of all registered providers.
func (r *Registry) All() map[string]Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]Provider, len(r.providers))
	for k, v := range r.providers {
		out[k] = v
	}
	return out
}

// ByType returns all providers of a given type.
func (r *Registry) ByType(t ProviderType) []Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []Provider
	for _, p := range r.providers {
		if p.Type() == t {
			out = append(out, p)
		}
	}
	return out
}

// Remove removes and closes a provider by ID.
func (r *Registry) Remove(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p, ok := r.providers[id]; ok {
		log.Info().Str("id", id).Str("type", string(p.Type())).Msg("removing provider")
		_ = p.Close()
		delete(r.providers, id)
	}
}

// Close shuts down all providers.
func (r *Registry) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, p := range r.providers {
		log.Info().Str("id", id).Str("type", string(p.Type())).Msg("closing provider")
		_ = p.Close()
	}
	r.providers = make(map[string]Provider)
}

// HealthCheck runs a health check on all providers and returns their status.
func (r *Registry) HealthCheck(ctx context.Context) []ProviderHealth {
	r.mu.RLock()
	providers := make(map[string]Provider, len(r.providers))
	for k, v := range r.providers {
		providers[k] = v
	}
	r.mu.RUnlock()

	var results []ProviderHealth
	for _, p := range providers {
		healthy := p.Healthy(ctx)
		h := ProviderHealth{
			ProviderID:   p.ID(),
			ProviderName: p.Name(),
			ProviderType: p.Type(),
			Connected:    healthy,
		}
		if healthy {
			// Best effort: count resources.
			if nodes, err := p.GetNodes(ctx); err == nil {
				h.NodeCount = len(nodes)
			}
			if vms, err := p.GetVMs(ctx, ""); err == nil {
				h.VMCount = len(vms)
			}
		}
		results = append(results, h)
	}
	return results
}

// GetConsoleProvider returns the ConsoleProvider interface for a provider, if supported.
func (r *Registry) GetConsoleProvider(id string) (ConsoleProvider, bool) {
	p, ok := r.Get(id)
	if !ok {
		return nil, false
	}
	cp, ok := p.(ConsoleProvider)
	return cp, ok
}

// GetPowerProvider returns the PowerProvider interface for a provider, if supported.
func (r *Registry) GetPowerProvider(id string) (PowerProvider, bool) {
	p, ok := r.Get(id)
	if !ok {
		return nil, false
	}
	pp, ok := p.(PowerProvider)
	return pp, ok
}
