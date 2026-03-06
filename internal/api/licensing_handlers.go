package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rs/zerolog/log"
)

// revocationFeedToken returns the relay feed token for revocation polling.
// Empty string means revocation polling is disabled.
func revocationFeedToken() string {
	return os.Getenv("PULSE_REVOCATION_FEED_TOKEN")
}

// LicenseHandlers handles license management API endpoints.
type LicenseHandlers struct {
	mtPersistence      *config.MultiTenantPersistence
	hostedMode         bool
	cfg                *config.Config
	services           sync.Map // map[string]*licenseService
	trialLimiter       *RateLimiter
	trialReplay        *jtiReplayStore
	monitor            *monitoring.Monitor
	mtMonitor          *monitoring.MultiTenantMonitor
	conversionRecorder *conversionRecorder
	conversionHealth   *conversionPipelineHealth
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

// SetConversionRecorder wires the conversion event recorder for backend-emitted
// conversion events (trial_started, license_activated, license_activation_failed).
func (h *LicenseHandlers) SetConversionRecorder(rec *conversionRecorder, health *conversionPipelineHealth) {
	if h == nil {
		return
	}
	h.conversionRecorder = rec
	h.conversionHealth = health
}

// emitConversionEvent is a fire-and-forget helper that records a backend-emitted
// conversion event. Respects the DisableLocalUpgradeMetrics config flag.
// Errors are logged but never propagated to callers.
func (h *LicenseHandlers) emitConversionEvent(orgID string, event conversionEvent) {
	if h == nil || h.conversionRecorder == nil {
		return
	}
	if h.cfg != nil && h.cfg.DisableLocalUpgradeMetrics {
		return
	}
	if orgID == "" {
		orgID = "default"
	}
	event.OrgID = orgID
	if event.Timestamp <= 0 {
		event.Timestamp = time.Now().UnixMilli()
	}
	if event.IdempotencyKey == "" {
		event.IdempotencyKey = fmt.Sprintf("backend:%s:%s:%s:%d", orgID, event.Type, event.Surface, event.Timestamp)
	}
	if err := h.conversionRecorder.Record(event); err != nil {
		log.Warn().Err(err).Str("event_type", event.Type).Str("org_id", orgID).Msg("Failed to record backend conversion event")
	} else {
		recordConversionEventMetric(event.Type, event.Surface)
		if h.conversionHealth != nil {
			h.conversionHealth.RecordEvent(event.Type)
		}
	}
}

// StopAllBackgroundLoops stops grant refresh and revocation poll loops for all tenant services.
// Called during server shutdown to ensure clean goroutine termination.
func (h *LicenseHandlers) StopAllBackgroundLoops() {
	if h == nil {
		return
	}
	h.services.Range(func(_, value any) bool {
		if svc, ok := value.(*licenseService); ok {
			svc.StopGrantRefresh()
			svc.StopRevocationPoll()
		}
		return true
	})
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

	// Wire license server client and persistence so activation / refresh can use them.
	lsClient := newLicenseServerClientFromLicensing("")
	service.SetLicenseServerClient(lsClient)
	if persistence != nil {
		service.SetPersistence(persistence)
	}

	if err := h.ensureEvaluatorForOrg(orgID, service); err != nil {
		log.Warn().Str("org_id", orgID).Err(err).Msg("Failed to initialize license evaluator for org")
	}

	// Restore activation state from v6 grant persistence when present.
	if persistence != nil {
		activationState, loadErr := persistence.LoadActivationState()
		if loadErr != nil {
			log.Warn().Str("org_id", orgID).Err(loadErr).Msg("Failed to load activation state")
		} else if activationState != nil {
			if err := service.RestoreActivation(activationState); err != nil {
				log.Warn().Str("org_id", orgID).Err(err).Msg("Failed to restore activation")
			} else {
				// Start the background refresh loop for the restored grant.
				service.StartGrantRefresh(context.Background())
				// Start revocation polling if a feed token is configured.
				if feedToken := revocationFeedToken(); feedToken != "" {
					service.StartRevocationPoll(context.Background(), feedToken)
				}
				log.Info().Str("org_id", orgID).Str("license_id", activationState.LicenseID).Msg("Restored license activation")
			}
		}
	}

	// Strict v6 automatically exchanges persisted legacy JWT licenses into the
	// activation/grant model on startup when no activation state exists yet.
	if persistence != nil && !isLicenseValidationDevModeFromLicensing() && !service.IsActivated() {
		legacyJWT, loadErr := persistence.Load()
		if loadErr != nil {
			if !os.IsNotExist(loadErr) {
				log.Warn().Str("org_id", orgID).Err(loadErr).Msg("Failed to load persisted legacy license")
			}
		} else if strings.TrimSpace(legacyJWT) != "" {
			if _, err := service.Activate(legacyJWT); err != nil {
				log.Warn().Str("org_id", orgID).Err(err).Msg("Failed to auto-exchange persisted legacy license")
			} else if service.IsActivated() {
				service.StartGrantRefresh(context.Background())
				if feedToken := revocationFeedToken(); feedToken != "" {
					service.StartRevocationPoll(context.Background(), feedToken)
				}
				if err := persistence.Delete(); err != nil {
					log.Warn().Str("org_id", orgID).Err(err).Msg("Failed to delete migrated legacy license persistence")
				}
				if current := service.Current(); current != nil {
					log.Info().
						Str("org_id", orgID).
						Str("license_id", current.Claims.LicenseID).
						Msg("Auto-exchanged persisted legacy license")
				}
			}
		}
	}

	// Use LoadOrStore to avoid racing with concurrent first requests for the same org.
	// If another goroutine stored first, use its service and let ours be GC'd.
	actual, loaded := h.services.LoadOrStore(orgID, service)
	if loaded {
		service.StopGrantRefresh()   // stop our orphaned refresh loop if started
		service.StopRevocationPoll() // stop our orphaned revocation poller if started
		svc := actual.(*licenseService)
		p, pErr := h.getPersistenceForOrg(orgID)
		return svc, p, pErr
	}

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

	h.emitConversionEvent(orgID, conversionEvent{
		Type:    conversionEventTrialStarted,
		Surface: "license_api",
	})

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

	// Prefer configured PublicURL for host binding validation.
	// Fall back to request Host only if no PublicURL is configured, to avoid
	// an attacker-controlled Host header weakening instance binding.
	// If PublicURL is set but normalizes to empty (malformed), fail closed.
	expectedHost := ""
	if h.cfg != nil && strings.TrimSpace(h.cfg.PublicURL) != "" {
		expectedHost = normalizeHostForTrial(h.cfg.PublicURL)
		if expectedHost == "" {
			log.Error().Msg("PublicURL configured but could not extract host for trial activation binding")
			http.Redirect(w, r, "/settings?trial=unavailable", http.StatusTemporaryRedirect)
			return
		}
	} else {
		expectedHost = normalizeHostForTrial(r.Host)
	}
	if expectedHost == "" {
		log.Error().Msg("Could not determine host for trial activation binding")
		http.Redirect(w, r, "/settings?trial=unavailable", http.StatusTemporaryRedirect)
		return
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

	h.emitConversionEvent(orgID, conversionEvent{
		Type:    conversionEventTrialStarted,
		Surface: "hosted_signup",
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

// userFriendlyActivationError maps internal activation errors to user-facing messages.
// The raw error should already be logged before calling this function.
func userFriendlyActivationError(err error) string {
	switch {
	case errors.Is(err, errMalformedLicenseSentinel):
		return "The license key format is not valid. Please check for typos and try again."
	case errors.Is(err, errInvalidLicenseSentinel):
		return "The license key is not valid. Please check for typos and try again."
	case errors.Is(err, errSignatureInvalidSentinel):
		return "The license key could not be verified. Please ensure you are using the correct key."
	case errors.Is(err, errExpiredLicenseSentinel):
		return "This license has expired. Contact support for renewal options."
	case errors.Is(err, errNoPublicKeySentinel):
		return "License verification is temporarily unavailable. Please try again later."
	}

	// License server errors from the activation key flow.
	var serverErr *licenseServerErrorModel
	if errors.As(err, &serverErr) {
		if serverErr.Retryable {
			return "The license server is temporarily unavailable. Please try again in a few minutes."
		}
		if serverErr.Message != "" {
			return serverErr.Message
		}
	}

	// Known non-sentinel patterns.
	msg := err.Error()
	if strings.Contains(msg, "activation key") {
		return "This looks like a legacy Pulse v5 license key. Pulse v6 requires an activation key from your Pulse account. If you purchased Pro or Lifetime on v5 and do not see a migrated activation key, contact support."
	}
	if strings.Contains(msg, "license server client not configured") {
		return "License activation is temporarily unavailable. Please try again later or contact support."
	}

	return "License activation failed. Please try again or contact support if the problem persists."
}

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

	orgID := GetOrgID(r.Context())

	lic, err := service.Activate(req.LicenseKey)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to activate license")

		h.emitConversionEvent(orgID, conversionEvent{
			Type:    conversionEventLicenseActivationFailed,
			Surface: "license_api",
		})

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ActivateLicenseResponse{
			Success: false,
			Message: userFriendlyActivationError(err),
		})
		return
	}

	// Persist based on activation type.
	if service.IsActivated() {
		// Activation state is already persisted by ActivateWithKey, but start the refresh loop.
		service.StartGrantRefresh(context.Background())
		if feedToken := revocationFeedToken(); feedToken != "" {
			service.StartRevocationPoll(context.Background(), feedToken)
		}
		// Remove stale legacy JWT persistence to prevent fallback to old license on restart.
		if persistence != nil {
			_ = persistence.Delete()
		}
		log.Info().
			Str("tier", string(lic.Claims.Tier)).
			Msg("Pulse license activated via activation key")
	} else {
		// Strict v6: legacy JWT activation is only possible in explicit dev mode.
		log.Info().
			Str("email", lic.Claims.Email).
			Str("tier", string(lic.Claims.Tier)).
			Bool("lifetime", lic.IsLifetime()).
			Msg("Pulse license activated in development JWT mode")
	}

	h.emitConversionEvent(orgID, conversionEvent{
		Type:    conversionEventLicenseActivated,
		Surface: "license_api",
	})

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

	// Clear from persistence (both legacy JWT and activation state).
	if persistence != nil {
		if err := persistence.Delete(); err != nil {
			log.Warn().Err(err).Msg("Failed to delete persisted license")
		}
		if err := persistence.ClearActivationState(); err != nil {
			log.Warn().Err(err).Msg("Failed to delete persisted activation state")
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
