package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
	"github.com/rs/zerolog/log"
)

// LicenseHandlers handles license management API endpoints.
// LicenseHandlers handles license management API endpoints.
type LicenseHandlers struct {
	mtPersistence *config.MultiTenantPersistence
	hostedMode    bool
	services      sync.Map // map[string]*pkglicensing.Service
	trialLimiter  *RateLimiter
}

// NewLicenseHandlers creates a new license handlers instance.

func NewLicenseHandlers(mtp *config.MultiTenantPersistence, hostedMode bool) *LicenseHandlers {
	return &LicenseHandlers{
		mtPersistence: mtp,
		hostedMode:    hostedMode,
		trialLimiter:  NewRateLimiter(1, 24*time.Hour), // 1 trial start attempt per org per 24h
	}
}

// getTenantComponents resolves the license service and persistence for the current tenant.
// It initializes them if they haven't been loaded yet.
func (h *LicenseHandlers) getTenantComponents(ctx context.Context) (*pkglicensing.Service, *pkglicensing.Persistence, error) {
	orgID := GetOrgID(ctx)

	// Check if service already exists
	if v, ok := h.services.Load(orgID); ok {
		svc := v.(*pkglicensing.Service)
		if err := h.ensureEvaluatorForOrg(orgID, svc); err != nil {
			log.Warn().Str("org_id", orgID).Err(err).Msg("Failed to refresh license evaluator for org")
		}
		// We need persistence too, reconstruct it or cache it?
		// Reconstructing persistence is cheap (just a struct with path).
		// But let's recreate it to be safe and stateless here.
		// Actually, we need the EXACT persistence object if it holds state, but license.Persistence seems stateless (file I/O).
		p, err := h.getPersistenceForOrg(orgID)
		return svc, p, err
	}

	// Initialize for this tenant
	persistence, err := h.getPersistenceForOrg(orgID)
	if err != nil {
		return nil, nil, err
	}

	service := pkglicensing.NewService()

	if err := h.ensureEvaluatorForOrg(orgID, service); err != nil {
		log.Warn().Str("org_id", orgID).Err(err).Msg("Failed to initialize license evaluator for org")
	}

	// Try to load existing license
	if persistence != nil {
		persisted, err := persistence.LoadWithMetadata()
		if err == nil && persisted.LicenseKey != "" {
			lic, err := service.Activate(persisted.LicenseKey)
			if err != nil {
				log.Warn().Str("org_id", orgID).Err(err).Msg("Failed to load saved license")
			} else {
				if persisted.GracePeriodEnd != nil && lic != nil {
					gracePeriodEnd := time.Unix(*persisted.GracePeriodEnd, 0)
					lic.GracePeriodEnd = &gracePeriodEnd
				}
				log.Info().Str("org_id", orgID).Msg("Loaded saved Pulse Pro license")
			}
		}
	}

	h.services.Store(orgID, service)
	return service, persistence, nil
}

func (h *LicenseHandlers) ensureEvaluatorForOrg(orgID string, service *pkglicensing.Service) error {
	if h == nil || service == nil || h.mtPersistence == nil {
		return nil
	}
	// Never override token-backed evaluator when a JWT license is present.
	if service.Current() != nil {
		return nil
	}

	billingStore := config.NewFileBillingStore(h.mtPersistence.BaseDataDir())

	// Hosted non-default tenants always consult billing state (fail-open).
	if h.hostedMode && orgID != "default" && orgID != "" {
		dbSource := pkglicensing.NewDatabaseSource(billingStore, orgID, time.Hour)
		evaluator := pkglicensing.NewEvaluator(dbSource)
		service.SetEvaluator(evaluator)
		return nil
	}

	// Self-hosted default org: only wire trial evaluator when an explicit, active trial exists.
	state, err := billingStore.GetBillingState(orgID)
	if err != nil {
		return fmt.Errorf("load billing state for org %q: %w", orgID, err)
	}
	if state == nil || state.SubscriptionState == "" {
		service.SetEvaluator(nil)
		return nil
	}

	// Billing state exists: wire evaluator without caching so UI updates immediately after writes.
	dbSource := pkglicensing.NewDatabaseSource(billingStore, orgID, 0)
	evaluator := pkglicensing.NewEvaluator(dbSource)
	service.SetEvaluator(evaluator)
	return nil
}

func (h *LicenseHandlers) getPersistenceForOrg(orgID string) (*pkglicensing.Persistence, error) {
	configPersistence, err := h.mtPersistence.GetPersistence(orgID)
	if err != nil {
		return nil, err
	}
	return pkglicensing.NewPersistence(configPersistence.GetConfigDir())
}

// Service returns the license service for use by other handlers.
// NOTE: This now requires context to identify the tenant.
// Handlers using this will need to be updated.
func (h *LicenseHandlers) Service(ctx context.Context) *pkglicensing.Service {
	svc, _, _ := h.getTenantComponents(ctx)
	return svc
}

// FeatureService resolves a request-scoped feature checker.
// This satisfies pkg/licensing.FeatureServiceResolver for reusable middleware.
func (h *LicenseHandlers) FeatureService(ctx context.Context) pkglicensing.FeatureChecker {
	if h == nil {
		return nil
	}
	return h.Service(ctx)
}

// HandleStartTrial handles POST /api/license/trial/start.
// This is a one-time, 14-day Pro trial stored locally in billing.json (no phone-home).
func (h *LicenseHandlers) HandleStartTrial(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h == nil || h.mtPersistence == nil {
		writeErrorResponse(w, http.StatusInternalServerError, "trial_start_unavailable", "Trial start is unavailable", nil)
		return
	}

	orgID := GetOrgID(r.Context())
	if orgID == "" {
		orgID = "default"
	}

	svc, _, err := h.getTenantComponents(r.Context())
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "tenant_error", "Failed to resolve tenant", nil)
		return
	}

	billingStore := config.NewFileBillingStore(h.mtPersistence.BaseDataDir())
	existing, err := billingStore.GetBillingState(orgID)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "billing_state_load_failed", "Failed to load billing state", nil)
		return
	}
	decision := pkglicensing.EvaluateTrialStartEligibility(svc.Current() != nil && svc.IsValid(), existing)
	if !decision.Allowed {
		code, message, includeOrgID := pkglicensing.TrialStartError(decision.Reason)
		details := map[string]string(nil)
		if includeOrgID {
			details = map[string]string{"org_id": orgID}
		}
		writeErrorResponse(w, http.StatusConflict, code, message, details)
		return
	}

	if h.trialLimiter != nil && !h.trialLimiter.Allow(orgID) {
		w.Header().Set("Retry-After", "86400")
		writeErrorResponse(w, http.StatusTooManyRequests, "trial_rate_limited", "Trial start rate limit exceeded", map[string]string{
			"org_id": orgID,
		})
		return
	}

	state := pkglicensing.BuildTrialBillingState(time.Now(), pkglicensing.TierFeatures[pkglicensing.TierPro])

	if err := billingStore.SaveBillingState(orgID, state); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "billing_state_save_failed", "Failed to save billing state", nil)
		return
	}

	// Ensure the in-memory service reflects the trial immediately.
	// Note: JWT licenses (if present) still take precedence over evaluator.
	if svc.Current() == nil {
		eval := pkglicensing.NewEvaluator(pkglicensing.NewDatabaseSource(billingStore, orgID, 0))
		svc.SetEvaluator(eval)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(state)
}

// HandleLicenseStatus handles GET /api/license/status
// Returns the current license status.
func (h *LicenseHandlers) HandleLicenseStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	service, _, err := h.getTenantComponents(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("Failed to get license components")
		http.Error(w, "Tenant error", http.StatusInternalServerError)
		return
	}

	status := service.Status()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// LicenseFeaturesResponse provides a minimal, non-admin license view for feature gating.
type LicenseFeaturesResponse = pkglicensing.LicenseFeaturesResponse

// HandleLicenseFeatures handles GET /api/license/features
// Returns license state and feature availability for authenticated users.
func (h *LicenseHandlers) HandleLicenseFeatures(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	service, _, err := h.getTenantComponents(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("Failed to get license components")
		http.Error(w, "Tenant error", http.StatusInternalServerError)
		return
	}

	state, _ := service.GetLicenseState()
	response := LicenseFeaturesResponse{
		LicenseStatus: string(state),
		Features:      pkglicensing.BuildFeatureMap(service, nil),
		UpgradeURL:    pkglicensing.UpgradeURLForFeature(""),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ActivateLicenseRequest is the request body for activating a license.
type ActivateLicenseRequest = pkglicensing.ActivateLicenseRequest

// ActivateLicenseResponse is the response for license activation.
type ActivateLicenseResponse = pkglicensing.ActivateLicenseResponse

// HandleActivateLicense handles POST /api/license/activate
// Validates and activates a license key.
func (h *LicenseHandlers) HandleActivateLicense(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ActivateLicenseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ActivateLicenseResponse{
			Success: false,
			Message: "Invalid request body",
		})
		return
	}

	if req.LicenseKey == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ActivateLicenseResponse{
			Success: false,
			Message: "License key is required",
		})
		return
	}

	// Activate the license
	service, persistence, err := h.getTenantComponents(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("Failed to get license components")
		http.Error(w, "Tenant error", http.StatusInternalServerError)
		return
	}

	lic, err := service.Activate(req.LicenseKey)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to activate license")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ActivateLicenseResponse{
			Success: false,
			Message: err.Error(),
		})
		return
	}

	// Persist the license with grace period if applicable
	if persistence != nil {
		var gracePeriodEnd *int64
		if lic.GracePeriodEnd != nil {
			ts := lic.GracePeriodEnd.Unix()
			gracePeriodEnd = &ts
		}
		if err := persistence.SaveWithGracePeriod(req.LicenseKey, gracePeriodEnd); err != nil {
			log.Warn().Err(err).Msg("Failed to persist license, it won't survive restarts")
		}
	}

	log.Info().
		Str("email", lic.Claims.Email).
		Str("tier", string(lic.Claims.Tier)).
		Bool("lifetime", lic.IsLifetime()).
		Msg("Pulse Pro license activated")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ActivateLicenseResponse{
		Success: true,
		Message: "License activated successfully",
		Status:  service.Status(),
	})
}

// HandleClearLicense handles POST /api/license/clear
// Removes the current license.
func (h *LicenseHandlers) HandleClearLicense(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Clear from service
	service, persistence, err := h.getTenantComponents(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("Failed to get license components")
		http.Error(w, "Tenant error", http.StatusInternalServerError)
		return
	}

	service.Clear()

	// Clear from persistence
	if persistence != nil {
		if err := persistence.Delete(); err != nil {
			log.Warn().Err(err).Msg("Failed to delete persisted license")
		}
	}

	log.Info().Msg("Pulse Pro license cleared")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "License cleared",
	})
}

// RequireLicenseFeature is a middleware that checks if a license feature is available.
// Returns HTTP 402 Payment Required if the feature is not licensed.
// WriteLicenseRequired writes a 402 Payment Required response for a missing license feature.
// ALL license gate responses in handlers MUST use this function to ensure consistent response format.
//
// NOTE: Direct 402 responses are intentionally centralized in this file to keep API behavior consistent.
func writePaymentRequired(w http.ResponseWriter, payload map[string]interface{}) {
	pkglicensing.WritePaymentRequired(w, payload)
}

func WriteLicenseRequired(w http.ResponseWriter, feature, message string) {
	pkglicensing.WriteLicenseRequired(w, feature, message, pkglicensing.UpgradeURLForFeature)
}

// RequireLicenseFeature is a middleware that checks if a license feature is available.
// Returns HTTP 402 Payment Required if the feature is not licensed.
// Note: Changed to take *LicenseHandlers to access service at runtime.
func RequireLicenseFeature(resolver pkglicensing.FeatureServiceResolver, feature string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if resolver == nil {
			WriteLicenseRequired(w, feature, "license service unavailable")
			return
		}
		service := resolver.FeatureService(r.Context())
		if service == nil {
			WriteLicenseRequired(w, feature, "license service unavailable")
			return
		}
		if err := service.RequireFeature(feature); err != nil {
			WriteLicenseRequired(w, feature, err.Error())
			return
		}
		next(w, r)
	}
}

// LicenseGatedEmptyResponse returns an empty array with license metadata header for unlicensed users.
// Use this instead of RequireLicenseFeature when the endpoint should return empty data
// rather than a 402 error (to avoid breaking Promise.all in the frontend).
// The X-License-Required header indicates upgrade is needed.
// LicenseGatedEmptyResponse returns an empty array with license metadata header for unlicensed users.
// Use this instead of RequireLicenseFeature when the endpoint should return empty data
// rather than a 402 error (to avoid breaking Promise.all in the frontend).
// The X-License-Required header indicates upgrade is needed.
func LicenseGatedEmptyResponse(resolver pkglicensing.FeatureServiceResolver, feature string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if resolver == nil {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-License-Required", "true")
			w.Header().Set("X-License-Feature", feature)
			w.Write([]byte("[]"))
			return
		}
		service := resolver.FeatureService(r.Context())
		if service == nil || service.RequireFeature(feature) != nil {
			w.Header().Set("Content-Type", "application/json")
			// Set header to indicate license is required (frontend can check this)
			w.Header().Set("X-License-Required", "true")
			w.Header().Set("X-License-Feature", feature)
			// Return 200 with empty array (compatible with frontend array expectations)
			w.Write([]byte("[]"))
			return
		}
		next(w, r)
	}
}
