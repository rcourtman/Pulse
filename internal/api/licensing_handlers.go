package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rs/zerolog/log"
)

// LicenseHandlers handles license management API endpoints.
// LicenseHandlers handles license management API endpoints.
type LicenseHandlers struct {
	mtPersistence *config.MultiTenantPersistence
	hostedMode    bool
	cfg           *config.Config
	services      sync.Map // map[string]*licenseService
	trialLimiter  *RateLimiter
	trialReplay   *jtiReplayStore
	monitor       *monitoring.Monitor
	mtMonitor     *monitoring.MultiTenantMonitor
}

// NewLicenseHandlers creates a new license handlers instance.

func NewLicenseHandlers(mtp *config.MultiTenantPersistence, hostedMode bool, cfgs ...*config.Config) *LicenseHandlers {
	var cfg *config.Config
	if len(cfgs) > 0 {
		cfg = cfgs[0]
	}

	var trialReplay *jtiReplayStore
	if mtp != nil {
		trialReplay = &jtiReplayStore{configDir: mtp.BaseDataDir()}
	}

	return &LicenseHandlers{
		mtPersistence: mtp,
		hostedMode:    hostedMode,
		cfg:           cfg,
		trialLimiter:  NewRateLimiter(1, 24*time.Hour), // 1 trial start attempt per org per 24h
		trialReplay:   trialReplay,
	}
}

// SetMonitors wires the monitors used for agent counting in entitlement usage.
func (h *LicenseHandlers) SetMonitors(monitor *monitoring.Monitor, mtMonitor *monitoring.MultiTenantMonitor) {
	if h == nil {
		return
	}
	h.monitor = monitor
	h.mtMonitor = mtMonitor
}

func (h *LicenseHandlers) SetConfig(cfg *config.Config) {
	if h == nil || cfg == nil {
		return
	}
	h.cfg = cfg
}

func trialCallbackURLForRequest(r *http.Request, cfg *config.Config) string {
	baseURL := ""
	if cfg != nil {
		baseURL = strings.TrimSpace(cfg.PublicURL)
	}
	if baseURL == "" && r != nil {
		scheme := "http"
		if xfProto := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); xfProto != "" {
			scheme = strings.ToLower(strings.TrimSpace(strings.Split(xfProto, ",")[0]))
		} else if r.TLS != nil {
			scheme = "https"
		}

		host := strings.TrimSpace(r.Host)
		if xfHost := strings.TrimSpace(r.Header.Get("X-Forwarded-Host")); xfHost != "" {
			host = strings.TrimSpace(strings.Split(xfHost, ",")[0])
		}
		if host == "" {
			return ""
		}
		baseURL = scheme + "://" + host
	}
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return ""
	}
	return baseURL + "/auth/trial-activate"
}

func normalizeHostForTrial(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.Contains(raw, "://") {
		if parsed, err := url.Parse(raw); err == nil {
			raw = parsed.Host
		}
	}
	if host, _, err := net.SplitHostPort(raw); err == nil && host != "" {
		raw = host
	}
	raw = strings.Trim(raw, "[]")
	return strings.ToLower(strings.TrimSpace(raw))
}

// getTenantComponents resolves the license service and persistence for the current tenant.
// It initializes them if they haven't been loaded yet.
func (h *LicenseHandlers) getTenantComponents(ctx context.Context) (*licenseService, *licensePersistence, error) {
	orgID := GetOrgID(ctx)

	// Check if service already exists
	if v, ok := h.services.Load(orgID); ok {
		svc := v.(*licenseService)
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

	service := newLicenseService()

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

func (h *LicenseHandlers) ensureEvaluatorForOrg(orgID string, service *licenseService) error {
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
		evaluator := newLicenseEvaluatorForBillingStoreFromLicensing(billingStore, orgID, time.Hour)
		service.SetEvaluator(evaluator)
		return nil
	}

	// Self-hosted mode:
	// - Default org uses its own billing state.
	// - Non-default orgs inherit default-org billing state when they do not yet have
	//   org-local billing state. This keeps instance-wide licenses/trials consistent
	//   across tenant contexts in self-hosted deployments.
	evaluatorOrgID := orgID

	state, err := billingStore.GetBillingState(orgID)
	if err != nil {
		return fmt.Errorf("load billing state for org %q: %w", orgID, err)
	}
	if (state == nil || state.SubscriptionState == "") && orgID != "" && orgID != "default" {
		defaultState, defaultErr := billingStore.GetBillingState("default")
		if defaultErr != nil {
			return fmt.Errorf("load default billing state fallback: %w", defaultErr)
		}
		if defaultState != nil && defaultState.SubscriptionState != "" {
			state = defaultState
			evaluatorOrgID = "default"
		}
	}

	if state == nil || state.SubscriptionState == "" {
		service.SetEvaluator(nil)
		return nil
	}

	// Billing state exists: wire evaluator without caching so UI updates immediately after writes.
	evaluator := newLicenseEvaluatorForBillingStoreFromLicensing(billingStore, evaluatorOrgID, 0)
	service.SetEvaluator(evaluator)
	return nil
}

func (h *LicenseHandlers) getPersistenceForOrg(orgID string) (*licensePersistence, error) {
	configPersistence, err := h.mtPersistence.GetPersistence(orgID)
	if err != nil {
		return nil, err
	}
	return newLicensePersistenceFromLicensing(configPersistence.GetConfigDir())
}

// Service returns the license service for use by other handlers.
// NOTE: This now requires context to identify the tenant.
// Handlers using this will need to be updated.
func (h *LicenseHandlers) Service(ctx context.Context) *licenseService {
	svc, _, _ := h.getTenantComponents(ctx)
	return svc
}

// FeatureService resolves a request-scoped feature checker.
// This satisfies pkg/licensing.FeatureServiceResolver for reusable middleware.
func (h *LicenseHandlers) FeatureService(ctx context.Context) licenseFeatureChecker {
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
	decision := evaluateTrialStartEligibilityFromLicensing(svc.Current() != nil && svc.IsValid(), existing)
	if !decision.Allowed {
		code, message, includeOrgID := trialStartErrorFromLicensing(decision.Reason)
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

	state := buildProTrialBillingStateFromLicensing(time.Now())

	if err := billingStore.SaveBillingState(orgID, state); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "billing_state_save_failed", "Failed to save billing state", nil)
		return
	}

	// Ensure the in-memory service reflects the trial immediately.
	// Note: JWT licenses (if present) still take precedence over evaluator.
	if svc.Current() == nil {
		eval := newLicenseEvaluatorForBillingStoreFromLicensing(billingStore, orgID, 0)
		svc.SetEvaluator(eval)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(state)
}

// HandleTrialActivation handles GET /auth/trial-activate.
// It verifies a hosted signup trial activation token, blocks replay, starts a
// local 14-day Pro trial, then redirects to Settings.
func (h *LicenseHandlers) HandleTrialActivation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h == nil || h.mtPersistence == nil {
		http.Redirect(w, r, "/settings?trial=unavailable", http.StatusTemporaryRedirect)
		return
	}

	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if token == "" {
		http.Redirect(w, r, "/settings?trial=invalid", http.StatusTemporaryRedirect)
		return
	}

	publicKey, err := trialActivationPublicKeyFromLicensing()
	if err != nil {
		log.Error().Err(err).Msg("Trial activation public key not configured")
		http.Redirect(w, r, "/settings?trial=unavailable", http.StatusTemporaryRedirect)
		return
	}

	expectedHost := normalizeHostForTrial(r.Host)
	if expectedHost == "" {
		expectedHost = normalizeHostForTrial(trialCallbackURLForRequest(r, h.cfg))
	}
	claims, err := verifyTrialActivationTokenFromLicensing(token, publicKey, expectedHost, time.Now().UTC())
	if err != nil {
		log.Warn().Err(err).Msg("Trial activation token verification failed")
		http.Redirect(w, r, "/settings?trial=invalid", http.StatusTemporaryRedirect)
		return
	}

	replayStore := h.trialReplay
	if replayStore == nil {
		replayStore = &jtiReplayStore{configDir: h.mtPersistence.BaseDataDir()}
	}
	tokenHash := sha256.Sum256([]byte(token))
	replayID := "trial_activate:" + hex.EncodeToString(tokenHash[:])
	expiresAt := time.Now().UTC().Add(15 * time.Minute)
	if claims.ExpiresAt != nil {
		expiresAt = claims.ExpiresAt.Time
	}
	stored, err := replayStore.checkAndStore(replayID, expiresAt)
	if err != nil {
		log.Error().Err(err).Msg("Trial activation replay-store failure")
		http.Redirect(w, r, "/settings?trial=unavailable", http.StatusTemporaryRedirect)
		return
	}
	if !stored {
		log.Warn().Str("replay_id_prefix", replayID[:24]).Msg("Trial activation token replay blocked")
		http.Redirect(w, r, "/settings?trial=replayed", http.StatusTemporaryRedirect)
		return
	}

	orgID := strings.TrimSpace(claims.OrgID)
	if orgID == "" {
		orgID = "default"
	}

	ctx := context.WithValue(r.Context(), OrgIDContextKey, orgID)
	svc, _, err := h.getTenantComponents(ctx)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("Trial activation tenant resolution failed")
		http.Redirect(w, r, "/settings?trial=unavailable", http.StatusTemporaryRedirect)
		return
	}

	billingStore := config.NewFileBillingStore(h.mtPersistence.BaseDataDir())
	existing, err := billingStore.GetBillingState(orgID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("Trial activation billing state load failed")
		http.Redirect(w, r, "/settings?trial=unavailable", http.StatusTemporaryRedirect)
		return
	}
	decision := evaluateTrialStartEligibilityFromLicensing(svc.Current() != nil && svc.IsValid(), existing)
	if !decision.Allowed {
		log.Info().
			Str("org_id", orgID).
			Str("reason", string(decision.Reason)).
			Msg("Trial activation denied due to ineligible state")
		http.Redirect(w, r, "/settings?trial=ineligible", http.StatusTemporaryRedirect)
		return
	}

	state := buildProTrialBillingStateFromLicensing(time.Now())
	if err := billingStore.SaveBillingState(orgID, state); err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("Trial activation billing state save failed")
		http.Redirect(w, r, "/settings?trial=unavailable", http.StatusTemporaryRedirect)
		return
	}
	if svc.Current() == nil {
		eval := newLicenseEvaluatorForBillingStoreFromLicensing(billingStore, orgID, 0)
		svc.SetEvaluator(eval)
	}

	isSecure, sameSite := getCookieSettings(r)
	http.SetCookie(w, &http.Cookie{
		Name:     CookieNameOrgID,
		Value:    orgID,
		Path:     "/",
		Secure:   isSecure,
		SameSite: sameSite,
		MaxAge:   int((24 * time.Hour).Seconds()),
	})

	log.Info().
		Str("org_id", orgID).
		Str("email", strings.TrimSpace(claims.Email)).
		Msg("Trial activation succeeded")

	http.Redirect(w, r, "/settings?trial=activated", http.StatusTemporaryRedirect)
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
type LicenseFeaturesResponse = licenseFeaturesResponse

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
		Features:      buildFeatureMapFromLicensing(service),
		UpgradeURL:    upgradeURLForFeatureFromLicensing(""),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ActivateLicenseRequest is the request body for activating a license.
type ActivateLicenseRequest = activateLicenseRequestModel

// ActivateLicenseResponse is the response for license activation.
type ActivateLicenseResponse = activateLicenseResponseModel

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
	writePaymentRequiredFromLicensing(w, payload)
}

func WriteLicenseRequired(w http.ResponseWriter, feature, message string) {
	writeLicenseRequiredFromLicensing(w, feature, message)
}

// RequireLicenseFeature is a middleware that checks if a license feature is available.
// Returns HTTP 402 Payment Required if the feature is not licensed.
// Note: Changed to take *LicenseHandlers to access service at runtime.
func RequireLicenseFeature(resolver licenseFeatureServiceResolver, feature string, next http.HandlerFunc) http.HandlerFunc {
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
func LicenseGatedEmptyResponse(resolver licenseFeatureServiceResolver, feature string, next http.HandlerFunc) http.HandlerFunc {
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
