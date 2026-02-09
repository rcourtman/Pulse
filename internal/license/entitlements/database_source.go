package entitlements

import (
	"sync"
	"time"
)

const defaultDatabaseSourceCacheTTL = time.Hour

// DatabaseSource implements EntitlementSource from hosted billing state.
type DatabaseSource struct {
	store     BillingStore
	orgID     string
	cache     *BillingState
	cacheTime time.Time
	cacheTTL  time.Duration
	mu        sync.RWMutex
	defaults  BillingState // trial-equivalent defaults for fail-open
}

// NewDatabaseSource creates a DatabaseSource for a hosted org.
func NewDatabaseSource(store BillingStore, orgID string, cacheTTL time.Duration) *DatabaseSource {
	if cacheTTL <= 0 {
		cacheTTL = defaultDatabaseSourceCacheTTL
	}

	return &DatabaseSource{
		store:    store,
		orgID:    orgID,
		cacheTTL: cacheTTL,
		defaults: BillingState{
			PlanVersion:       string(SubStateTrial),
			SubscriptionState: SubStateTrial,
		},
	}
}

// Capabilities returns the current capability keys.
func (d *DatabaseSource) Capabilities() []string {
	return d.currentState().Capabilities
}

// Limits returns the current plan limits.
func (d *DatabaseSource) Limits() map[string]int64 {
	return d.currentState().Limits
}

// MetersEnabled returns the enabled metering dimensions.
func (d *DatabaseSource) MetersEnabled() []string {
	return d.currentState().MetersEnabled
}

// PlanVersion returns the current plan version label.
func (d *DatabaseSource) PlanVersion() string {
	return d.currentState().PlanVersion
}

// SubscriptionState returns the current subscription lifecycle state.
func (d *DatabaseSource) SubscriptionState() SubscriptionState {
	return d.currentState().SubscriptionState
}

func (d *DatabaseSource) currentState() BillingState {
	defaults := d.defaultState()
	if d == nil {
		return defaults
	}

	cacheTTL := d.cacheTTL
	if cacheTTL <= 0 {
		cacheTTL = defaultDatabaseSourceCacheTTL
	}

	now := time.Now()

	d.mu.RLock()
	if d.cache != nil && now.Sub(d.cacheTime) <= cacheTTL {
		cached := cloneBillingState(*d.cache)
		d.mu.RUnlock()
		return cached
	}

	var stale BillingState
	hasStale := false
	if d.cache != nil {
		stale = cloneBillingState(*d.cache)
		hasStale = true
	}
	d.mu.RUnlock()

	if d.store == nil {
		if hasStale {
			return stale
		}
		return defaults
	}

	fresh, err := d.store.GetBillingState(d.orgID)
	if err == nil && fresh != nil {
		cached := cloneBillingState(*fresh)
		d.mu.Lock()
		d.cache = &cached
		d.cacheTime = time.Now()
		d.mu.Unlock()
		return cloneBillingState(cached)
	}

	if hasStale {
		return stale
	}

	return defaults
}

func (d *DatabaseSource) defaultState() BillingState {
	defaults := BillingState{
		PlanVersion:       string(SubStateTrial),
		SubscriptionState: SubStateTrial,
	}

	if d == nil {
		return defaults
	}

	if d.defaults.PlanVersion != "" {
		defaults.PlanVersion = d.defaults.PlanVersion
	}
	if d.defaults.SubscriptionState != "" {
		defaults.SubscriptionState = d.defaults.SubscriptionState
	}

	defaults.Capabilities = cloneStringSlice(d.defaults.Capabilities)
	defaults.Limits = cloneInt64Map(d.defaults.Limits)
	defaults.MetersEnabled = cloneStringSlice(d.defaults.MetersEnabled)

	return defaults
}

func cloneBillingState(state BillingState) BillingState {
	return BillingState{
		Capabilities:      cloneStringSlice(state.Capabilities),
		Limits:            cloneInt64Map(state.Limits),
		MetersEnabled:     cloneStringSlice(state.MetersEnabled),
		PlanVersion:       state.PlanVersion,
		SubscriptionState: state.SubscriptionState,
	}
}

func cloneStringSlice(values []string) []string {
	if values == nil {
		return nil
	}

	cloned := make([]string, len(values))
	copy(cloned, values)
	return cloned
}

func cloneInt64Map(values map[string]int64) map[string]int64 {
	if values == nil {
		return nil
	}

	cloned := make(map[string]int64, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}
