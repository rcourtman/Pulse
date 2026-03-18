package api

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
)

// Multi-tenant feature flag (default: disabled)
// Set PULSE_MULTI_TENANT_ENABLED=true to enable multi-tenant functionality.
// This is separate from licensing - the feature must be explicitly enabled
// AND properly licensed for non-default organizations to work.
var multiTenantEnabled = strings.EqualFold(os.Getenv("PULSE_MULTI_TENANT_ENABLED"), "true")

// IsMultiTenantEnabled returns whether multi-tenant functionality is enabled.
func IsMultiTenantEnabled() bool {
	return multiTenantEnabled
}

// DefaultMultiTenantChecker implements websocket.MultiTenantChecker for use with the WebSocket hub.
type DefaultMultiTenantChecker struct {
	hostedMode bool
}

// CheckMultiTenant checks if multi-tenant is enabled (feature flag) and licensed for the org.
// In hosted mode, tenant routing uses subscription lifecycle checks instead of FeatureMultiTenant.
// Uses the LicenseServiceProvider for proper per-tenant license lookup.
func (c *DefaultMultiTenantChecker) CheckMultiTenant(ctx context.Context, orgID string) websocket.MultiTenantCheckResult {
	// Default org is always allowed
	if orgID == "" || orgID == "default" {
		return websocket.MultiTenantCheckResult{
			Allowed:        true,
			FeatureEnabled: true,
			Licensed:       true,
		}
	}

	if c.hostedMode {
		// Hosted mode: tenant routing is infrastructure. Check subscription lifecycle
		// instead of FeatureMultiTenant so Cloud customers can connect.
		checkCtx := context.WithValue(ctx, OrgIDContextKey, orgID)
		if isHostedSubscriptionValid(checkCtx) {
			return websocket.MultiTenantCheckResult{
				Allowed:        true,
				FeatureEnabled: true,
				Licensed:       true,
			}
		}
		return websocket.MultiTenantCheckResult{
			Allowed:        false,
			FeatureEnabled: true,
			Licensed:       false,
			Reason:         "Cloud subscription is not active",
		}
	}

	// Self-hosted mode: check feature flag first
	if !multiTenantEnabled {
		return websocket.MultiTenantCheckResult{
			Allowed:        false,
			FeatureEnabled: false,
			Licensed:       false,
			Reason:         "Multi-tenant functionality is not enabled",
		}
	}

	// Feature is enabled, check license using the provider
	service := getLicenseServiceForContext(ctx)
	if !hasMultiTenantLicense(service) {
		return websocket.MultiTenantCheckResult{
			Allowed:        false,
			FeatureEnabled: true,
			Licensed:       false,
			Reason:         "Multi-tenant access requires an Enterprise license",
		}
	}

	return websocket.MultiTenantCheckResult{
		Allowed:        true,
		FeatureEnabled: true,
		Licensed:       true,
	}
}

// NewMultiTenantChecker creates a new DefaultMultiTenantChecker.
func NewMultiTenantChecker(hostedMode bool) *DefaultMultiTenantChecker {
	return &DefaultMultiTenantChecker{hostedMode: hostedMode}
}

// SetMultiTenantEnabled allows programmatic control of the feature flag (for testing).
func SetMultiTenantEnabled(enabled bool) {
	multiTenantEnabled = enabled
}

// LicenseServiceProvider provides license service for a given context.
// This allows the middleware to use the properly initialized per-tenant services.
type LicenseServiceProvider interface {
	Service(ctx context.Context) *licenseService
}

var (
	licenseServiceProvider LicenseServiceProvider
	licenseServiceMu       sync.RWMutex
)

// SetLicenseServiceProvider sets the provider for license services.
// This should be called during router initialization with LicenseHandlers.
func SetLicenseServiceProvider(provider LicenseServiceProvider) {
	licenseServiceMu.Lock()
	defer licenseServiceMu.Unlock()
	licenseServiceProvider = provider
}

// getLicenseServiceForContext returns the license service for the given context.
// Falls back to a new service if no provider is set (shouldn't happen in production).
func getLicenseServiceForContext(ctx context.Context) *licenseService {
	licenseServiceMu.RLock()
	provider := licenseServiceProvider
	licenseServiceMu.RUnlock()

	if provider != nil {
		return provider.Service(ctx)
	}
	// Fallback: create a new service (won't have persisted license)
	return newLicenseService()
}

// hasMultiTenantFeatureForContext checks if the multi-tenant feature is licensed for the context.
func hasMultiTenantFeatureForContext(ctx context.Context) bool {
	service := getLicenseServiceForContext(ctx)
	return hasMultiTenantLicense(service)
}

// RequireMultiTenant returns a middleware that checks if the multi-tenant feature is licensed.
// It allows access to the "default" organization without a license, but requires
// an Enterprise license for non-default organizations.
func RequireMultiTenant(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID := GetOrgID(r.Context())

		// Default org is always allowed (backward compatibility)
		if orgID == "" || orgID == "default" {
			next(w, r)
			return
		}

		// Feature flag check - multi-tenant must be explicitly enabled
		if !multiTenantEnabled {
			writeMultiTenantDisabledError(w)
			return
		}

		// Non-default orgs require multi-tenant license
		if !hasMultiTenantFeatureForContext(r.Context()) {
			writeMultiTenantRequiredError(w)
			return
		}

		next(w, r)
	}
}

// RequireMultiTenantHandler returns middleware for http.Handler.
func RequireMultiTenantHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		orgID := GetOrgID(r.Context())

		// Default org is always allowed (backward compatibility)
		if orgID == "" || orgID == "default" {
			next.ServeHTTP(w, r)
			return
		}

		// Feature flag check - multi-tenant must be explicitly enabled
		if !multiTenantEnabled {
			writeMultiTenantDisabledError(w)
			return
		}

		// Non-default orgs require multi-tenant license
		if !hasMultiTenantFeatureForContext(r.Context()) {
			writeMultiTenantRequiredError(w)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// writeMultiTenantRequiredError writes a 402 Payment Required response
// indicating that multi-tenant requires an Enterprise license.
func writeMultiTenantRequiredError(w http.ResponseWriter) {
	writePaymentRequired(w, map[string]interface{}{
		"error":   "license_required",
		"message": "Multi-tenant access requires an Enterprise license",
		"feature": featureMultiTenantKey,
		"tier":    "enterprise",
	})
}

// writeMultiTenantDisabledError writes a 501 Not Implemented response
// indicating that multi-tenant functionality is not enabled.
func writeMultiTenantDisabledError(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error":   "feature_disabled",
		"message": "Multi-tenant functionality is not enabled. Set PULSE_MULTI_TENANT_ENABLED=true to enable.",
	})
}

// CheckMultiTenantLicense checks if multi-tenant is enabled and licensed for the given org ID.
// Returns true if:
// - The org ID is "default" or empty (always allowed)
// - The feature flag is enabled AND the multi-tenant feature is licensed
func CheckMultiTenantLicense(ctx context.Context, orgID string) bool {
	if orgID == "" || orgID == "default" {
		return true
	}
	// Feature flag must be enabled
	if !multiTenantEnabled {
		return false
	}
	return hasMultiTenantFeatureForContext(ctx)
}
