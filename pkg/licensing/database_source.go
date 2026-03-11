package licensing

import (
	"strings"
	"sync"
	"time"
)

const defaultDatabaseSourceCacheTTL = time.Hour

// DatabaseSource implements EntitlementSource from hosted billing state.
type DatabaseSource struct {
	store                BillingStore
	orgID                string
	cache                *BillingState
	cacheTime            time.Time
	cacheTTL             time.Duration
	mu                   sync.RWMutex
	defaults             BillingState // trial-equivalent defaults for fail-open
	expectedInstanceHost string
}

// NewDatabaseSource creates a DatabaseSource for a hosted org.
func NewDatabaseSource(store BillingStore, orgID string, cacheTTL time.Duration) *DatabaseSource {
	// cacheTTL semantics:
	// - cacheTTL > 0: cache for that duration
	// - cacheTTL == 0: no caching (always refresh)
	// - cacheTTL < 0: defaults only (never consult store)

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

// WithExpectedInstanceHost binds hosted entitlement lease validation to the
// current instance host when a canonical public URL is configured.
func (d *DatabaseSource) WithExpectedInstanceHost(host string) *DatabaseSource {
	if d == nil {
		return nil
	}
	d.expectedInstanceHost = normalizeHost(host)
	return d
}

// Capabilities returns the current capability keys.
func (d *DatabaseSource) Capabilities() []string {
	return d.currentState().Capabilities
}

// Limits returns the current normalized plan limits.
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

// TrialStartedAt returns the stored trial start timestamp (Unix seconds) when present.
func (d *DatabaseSource) TrialStartedAt() *int64 {
	return cloneInt64Ptr(d.currentState().TrialStartedAt)
}

// TrialEndsAt returns the stored trial end timestamp (Unix seconds) when present.
func (d *DatabaseSource) TrialEndsAt() *int64 {
	return cloneInt64Ptr(d.currentState().TrialEndsAt)
}

// OverflowGrantedAt returns the stored overflow grant timestamp (Unix seconds) when present.
func (d *DatabaseSource) OverflowGrantedAt() *int64 {
	return cloneInt64Ptr(d.currentState().OverflowGrantedAt)
}

func (d *DatabaseSource) currentState() BillingState {
	defaults := d.defaultState()
	if d == nil {
		return normalizeDatabaseSourceState(normalizeTrialExpiry(defaults, time.Now()))
	}

	cacheTTL := d.cacheTTL
	now := time.Now()

	// cacheTTL < 0 means "defaults only" (e.g., fail-open / offline mode).
	if cacheTTL < 0 {
		return d.resolveState(defaults, now)
	}

	// cacheTTL == 0 means "no caching" (always refresh).
	noCache := cacheTTL == 0
	if cacheTTL == 0 {
		// Placeholder value so TTL comparisons compile; guarded by noCache.
		cacheTTL = defaultDatabaseSourceCacheTTL
	}

	d.mu.RLock()
	if !noCache && d.cache != nil && now.Sub(d.cacheTime) <= cacheTTL {
		cached := cloneBillingState(*d.cache)
		d.mu.RUnlock()
		return d.resolveState(cached, now)
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
			return d.resolveState(stale, now)
		}
		return d.resolveState(defaults, now)
	}

	fresh, err := d.store.GetBillingState(d.orgID)
	if err == nil && fresh != nil {
		cached := cloneBillingState(*fresh)
		cached = d.resolveState(cached, now)
		d.mu.Lock()
		d.cache = &cached
		d.cacheTime = time.Now()
		d.mu.Unlock()
		return cloneBillingState(cached)
	}

	if hasStale {
		return d.resolveState(stale, now)
	}

	return d.resolveState(defaults, now)
}

func (d *DatabaseSource) defaultState() BillingState {
	if d == nil {
		return BillingState{
			PlanVersion:       string(SubStateTrial),
			SubscriptionState: SubStateTrial,
		}
	}

	// Clone the full defaults struct so new fields are never silently dropped.
	defaults := cloneBillingState(d.defaults)

	// Apply fallback values for required fields.
	if defaults.PlanVersion == "" {
		defaults.PlanVersion = string(SubStateTrial)
	}
	if defaults.SubscriptionState == "" {
		defaults.SubscriptionState = SubStateTrial
	}

	return normalizeDatabaseSourceState(defaults)
}

func cloneBillingState(state BillingState) BillingState {
	// Start with a full value copy so new fields are never silently dropped.
	cp := state

	// Deep-clone reference types to break aliasing.
	cp.Capabilities = cloneStringSlice(state.Capabilities)
	cp.Limits = cloneInt64Map(state.Limits)
	cp.MetersEnabled = cloneStringSlice(state.MetersEnabled)
	cp.TrialStartedAt = cloneInt64Ptr(state.TrialStartedAt)
	cp.TrialEndsAt = cloneInt64Ptr(state.TrialEndsAt)
	cp.TrialExtendedAt = cloneInt64Ptr(state.TrialExtendedAt)
	cp.OverflowGrantedAt = cloneInt64Ptr(state.OverflowGrantedAt)
	cp.CommercialMigration = CloneCommercialMigrationStatus(state.CommercialMigration)

	return cp
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

func normalizeTrialExpiry(state BillingState, now time.Time) BillingState {
	if state.SubscriptionState != SubStateTrial || state.TrialEndsAt == nil {
		return state
	}
	if now.Unix() < *state.TrialEndsAt {
		return state
	}

	// Trial has expired: mark state as expired and strip capabilities.
	// Free-tier capabilities are granted via tier fallback in license.Service.
	state.SubscriptionState = SubStateExpired
	state.Capabilities = nil
	state.Limits = nil
	state.MetersEnabled = nil
	return state
}

func (d *DatabaseSource) resolveState(state BillingState, now time.Time) BillingState {
	return normalizeDatabaseSourceState(ResolveEntitlementLeaseBillingState(state, d.expectedInstanceHost, now))
}

func normalizeDatabaseSourceState(state BillingState) BillingState {
	normalized := cloneBillingState(state)
	normalized.PlanVersion = CanonicalizePlanVersion(strings.TrimSpace(normalized.PlanVersion))
	normalized.SubscriptionState = SubscriptionState(strings.ToLower(strings.TrimSpace(string(normalized.SubscriptionState))))
	normalized.EntitlementJWT = strings.TrimSpace(normalized.EntitlementJWT)
	normalized.EntitlementRefreshToken = strings.TrimSpace(normalized.EntitlementRefreshToken)
	normalized.StripeCustomerID = strings.TrimSpace(normalized.StripeCustomerID)
	normalized.StripeSubscriptionID = strings.TrimSpace(normalized.StripeSubscriptionID)
	normalized.StripePriceID = strings.TrimSpace(normalized.StripePriceID)
	normalized.CommercialMigration = NormalizeCommercialMigrationStatus(normalized.CommercialMigration)

	if normalized.Limits != nil {
		if v, hasOld := normalized.Limits[MaxNodesLicenseGateKey]; hasOld {
			if _, hasNew := normalized.Limits[MaxAgentsLicenseGateKey]; !hasNew {
				normalized.Limits[MaxAgentsLicenseGateKey] = v
			}
			delete(normalized.Limits, MaxNodesLicenseGateKey)
		}
	}

	if limit, known := CloudPlanAgentLimits[normalized.PlanVersion]; known {
		if normalized.Limits == nil {
			normalized.Limits = map[string]int64{}
		}
		normalized.Limits[MaxAgentsLicenseGateKey] = int64(limit)
	}

	return normalized
}
