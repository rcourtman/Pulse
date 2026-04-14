package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/crypto"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rs/zerolog/log"
)

const (
	trialStartRateLimitBurst                = 6
	trialStartRateLimitWindow               = 15 * time.Minute
	licensePurchaseStartPath                = "/auth/license-purchase-start"
	licensePurchaseActivationPath           = "/auth/license-purchase-activate"
	licensePurchaseSessionIDField           = "session_id"
	licensePurchaseReturnTokenField         = "purchase_return_token"
	pulseAccountUpgradeService              = "upgrade"
	pulseAccountPortalFeatureQueryParam     = "feature"
	pulseAccountPortalHandoffIDField        = "portal_handoff_id"
	pulseAccountPortalHandoffURLQueryParam  = "purchase_handoff_url"
	pulseAccountPortalServiceQueryParam     = "service"
	pulseAccountPortalReturnTokenQueryParam = "purchase_return_token"
	selfHostedBillingPurchaseQueryParam     = "purchase"
	selfHostedBillingPurchaseActivated      = "activated"
	selfHostedBillingPurchaseCancelled      = "cancelled"
	selfHostedBillingPurchaseExpired        = "expired"
	selfHostedBillingPurchaseFailed         = "failed"
	purchaseReturnKeyPurpose                = "pulse-license-purchase-return"
)

// revocationFeedToken returns the relay feed token for revocation polling.
// Empty string means revocation polling is disabled.
func revocationFeedToken() string {
	return os.Getenv("PULSE_REVOCATION_FEED_TOKEN")
}

func wantsMockFixturesFromEnv() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("PULSE_MOCK_MODE")), "true")
}

// LicenseHandlers handles license management API endpoints.
type LicenseHandlers struct {
	mtPersistence              *config.MultiTenantPersistence
	hostedMode                 bool
	cfg                        *config.Config
	services                   sync.Map // map[string]*licenseService
	trialLimiter               *RateLimiter
	trialReplay                *jtiReplayStore
	purchaseReturnRedemptions  *purchaseReturnRedemptionStore
	trialInitiations           *trialSignupInitiationStore
	trialRedeemer              func(token string) (*hostedTrialRedemptionResponse, error)
	monitor                    *monitoring.Monitor
	mtMonitor                  *monitoring.MultiTenantMonitor
	conversionRecorder         *conversionRecorder
	conversionHealth           *conversionPipelineHealth
	hostedLeaseRefresh         sync.Map // map[string]*hostedEntitlementRefreshLoop
	legacyGrandfatherReconcile sync.Map // map[string]*legacyGrandfatherReconcileLoop
	runtimeVersion             string
}

// NewLicenseHandlers creates a new license handlers instance.

func NewLicenseHandlers(mtp *config.MultiTenantPersistence, hostedMode bool, cfgs ...*config.Config) *LicenseHandlers {
	var cfg *config.Config
	if len(cfgs) > 0 {
		cfg = cfgs[0]
	}

	var trialReplay *jtiReplayStore
	var purchaseReturnRedemptions *purchaseReturnRedemptionStore
	var trialInitiations *trialSignupInitiationStore
	if mtp != nil {
		trialReplay = &jtiReplayStore{configDir: mtp.BaseDataDir()}
		purchaseReturnRedemptions = &purchaseReturnRedemptionStore{configDir: mtp.BaseDataDir()}
		trialInitiations = &trialSignupInitiationStore{configDir: mtp.BaseDataDir()}
	}

	return &LicenseHandlers{
		mtPersistence:             mtp,
		hostedMode:                hostedMode,
		cfg:                       cfg,
		trialLimiter:              NewRateLimiter(trialStartRateLimitBurst, trialStartRateLimitWindow),
		trialReplay:               trialReplay,
		purchaseReturnRedemptions: purchaseReturnRedemptions,
		trialInitiations:          trialInitiations,
	}
}

// SetMonitors wires the monitors used for monitored-system counting in entitlement usage.
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

func (h *LicenseHandlers) SetRuntimeVersion(version string) {
	if h == nil {
		return
	}
	h.runtimeVersion = strings.TrimSpace(version)
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
	h.hostedLeaseRefresh.Range(func(key, value any) bool {
		if orgID, ok := key.(string); ok {
			h.stopHostedEntitlementRefreshLoop(orgID)
		}
		return true
	})
	h.legacyGrandfatherReconcile.Range(func(key, value any) bool {
		if orgID, ok := key.(string); ok {
			h.stopLegacyGrandfatherReconcileLoop(orgID)
		}
		return true
	})
	h.services.Range(func(_, value any) bool {
		if svc, ok := value.(*licenseService); ok {
			svc.StopGrantRefresh()
			svc.StopRevocationPoll()
		}
		return true
	})
}

func publicBaseURLForRequest(r *http.Request, cfg *config.Config) string {
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
	return baseURL
}

func trialCallbackURLForRequest(r *http.Request, cfg *config.Config) string {
	baseURL := publicBaseURLForRequest(r, cfg)
	if baseURL == "" {
		return ""
	}
	return baseURL + "/auth/trial-activate"
}

func licensePurchaseCallbackURLForRequest(r *http.Request, cfg *config.Config) string {
	baseURL := publicBaseURLForRequest(r, cfg)
	if baseURL == "" {
		return ""
	}
	return baseURL + licensePurchaseActivationPath
}

func purchaseReturnExpectedHost(r *http.Request, cfg *config.Config) string {
	if cfg != nil && strings.TrimSpace(cfg.PublicURL) != "" {
		return normalizeHostForTrial(cfg.PublicURL)
	}
	return normalizeHostForTrial(publicBaseURLForRequest(r, nil))
}

func trialSignupActionURLForRequest(cfg *config.Config, orgID, returnURL, instanceToken string) (string, error) {
	signupBaseURL := ""
	if cfg != nil {
		signupBaseURL = strings.TrimSpace(cfg.ProTrialSignupURL)
	}
	signupURL := strings.TrimSpace(proTrialSignupURLFromLicensing(signupBaseURL))
	if signupURL == "" {
		return "", fmt.Errorf("trial signup URL is unavailable")
	}

	parsed, err := url.Parse(signupURL)
	if err != nil {
		return "", fmt.Errorf("parse trial signup URL: %w", err)
	}

	query := parsed.Query()
	query.Set("org_id", strings.TrimSpace(orgID))
	query.Set("return_url", strings.TrimSpace(returnURL))
	query.Set("instance_token", strings.TrimSpace(instanceToken))
	parsed.RawQuery = query.Encode()

	return parsed.String(), nil
}

func (h *LicenseHandlers) purchaseReturnDataDir() string {
	if h == nil {
		return ""
	}
	if h.mtPersistence != nil {
		return h.mtPersistence.BaseDataDir()
	}
	if h.cfg != nil {
		return strings.TrimSpace(h.cfg.DataPath)
	}
	return ""
}

func (h *LicenseHandlers) purchaseReturnSigningKey() ([]byte, error) {
	dataDir := h.purchaseReturnDataDir()
	if strings.TrimSpace(dataDir) == "" {
		return nil, fmt.Errorf("purchase return data path is unavailable")
	}
	cryptoManager, err := crypto.NewCryptoManagerAt(dataDir)
	if err != nil {
		return nil, fmt.Errorf("load purchase return crypto manager: %w", err)
	}
	signingKey, err := cryptoManager.DeriveKey(purchaseReturnKeyPurpose, 32)
	if err != nil {
		return nil, fmt.Errorf("derive purchase return signing key: %w", err)
	}
	return signingKey, nil
}

func (h *LicenseHandlers) purchaseReturnRedemptionStore() *purchaseReturnRedemptionStore {
	if h == nil {
		return nil
	}
	if h.purchaseReturnRedemptions != nil {
		return h.purchaseReturnRedemptions
	}
	dataDir := h.purchaseReturnDataDir()
	if strings.TrimSpace(dataDir) == "" {
		return nil
	}
	return &purchaseReturnRedemptionStore{configDir: dataDir}
}

func pulseAccountUpgradeURLForRequest(portalHandoffID string, query url.Values) (string, error) {
	portalURL := strings.TrimSpace(pulseAccountPortalURLFromLicensing(""))
	if portalURL == "" {
		return "", fmt.Errorf("pulse account portal url is unavailable")
	}

	parsed, err := url.Parse(portalURL)
	if err != nil {
		return "", fmt.Errorf("parse pulse account portal url: %w", err)
	}

	params := parsed.Query()
	for key, values := range query {
		switch key {
		case pulseAccountPortalFeatureQueryParam,
			pulseAccountPortalHandoffIDField,
			pulseAccountPortalHandoffURLQueryParam,
			pulseAccountPortalServiceQueryParam,
			pulseAccountPortalReturnTokenQueryParam,
			"return_url",
			"checkout",
			"session_id":
			continue
		}
		for _, value := range values {
			normalizedValue := strings.TrimSpace(value)
			if normalizedValue == "" {
				continue
			}
			params.Set(key, normalizedValue)
		}
	}

	params.Set(pulseAccountPortalHandoffIDField, strings.TrimSpace(portalHandoffID))
	parsed.RawQuery = params.Encode()
	return parsed.String(), nil
}

func publicAbsoluteURLForPath(r *http.Request, cfg *config.Config, path string) (string, error) {
	baseURL := publicBaseURLForRequest(r, cfg)
	if baseURL == "" {
		return "", fmt.Errorf("public base url is unavailable")
	}
	base, err := url.Parse(baseURL)
	if err != nil || base == nil {
		return "", fmt.Errorf("parse public base url: %w", err)
	}
	relative, err := url.Parse(strings.TrimSpace(path))
	if err != nil || relative == nil {
		return "", fmt.Errorf("parse relative public path: %w", err)
	}
	return base.ResolveReference(relative).String(), nil
}

func licensePurchaseActivationTemplateURL(returnURL, returnToken string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(returnURL))
	if err != nil || parsed == nil {
		return "", fmt.Errorf("parse purchase activation return url: %w", err)
	}
	query := parsed.Query()
	query.Set(licensePurchaseReturnTokenField, strings.TrimSpace(returnToken))
	query.Set(licensePurchaseSessionIDField, "{CHECKOUT_SESSION_ID}")
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func licensePurchaseActivationRedirectPath(feature, purchaseResult string) string {
	normalizedFeature := strings.TrimSpace(feature)
	query := url.Values{}
	switch normalizedFeature {
	case "max_monitored_systems":
		query.Set("intent", "max_monitored_systems")
	}
	switch strings.TrimSpace(purchaseResult) {
	case selfHostedBillingPurchaseActivated,
		selfHostedBillingPurchaseCancelled,
		selfHostedBillingPurchaseExpired,
		selfHostedBillingPurchaseFailed:
		query.Set(selfHostedBillingPurchaseQueryParam, strings.TrimSpace(purchaseResult))
	}
	encoded := query.Encode()
	if encoded == "" {
		return "/settings/system/billing/plan"
	}
	return "/settings/system/billing/plan?" + encoded
}

func writeLicensePurchaseActivationFailurePage(
	w http.ResponseWriter,
	statusCode int,
	feature string,
	purchaseResult string,
	message string,
) {
	if statusCode < http.StatusBadRequest {
		statusCode = http.StatusBadRequest
	}
	redirectPath := licensePurchaseActivationRedirectPath(feature, purchaseResult)
	escapedMessage := html.EscapeString(strings.TrimSpace(message))
	if escapedMessage == "" {
		escapedMessage = "Purchase activation could not be completed."
	}
	escapedLink := html.EscapeString(redirectPath)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(statusCode)
	_, _ = fmt.Fprintf(
		w,
		"<!DOCTYPE html><html lang=\"en\"><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width, initial-scale=1\"><title>Pulse Pro activation</title></head><body><main style=\"max-width:32rem;margin:3rem auto;padding:0 1rem;font:16px/1.5 -apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif\"><h1 style=\"margin-bottom:0.5rem\">Pulse Pro activation needs attention</h1><p id=\"purchase-return-status\" style=\"margin-bottom:1rem\">%s</p><p><a href=\"%s\">Open Pulse Pro billing</a></p></main><script>(function(){var redirectPath=%q;var statusEl=document.getElementById('purchase-return-status');var redirectedOriginalTab=false;try{if(window.opener&&!window.opener.closed){redirectedOriginalTab=true;window.opener.postMessage({type:'pulse-license-purchase-return',redirectPath:redirectPath,result:%q},window.location.origin);window.opener.location.assign(redirectPath);}}catch(_){redirectedOriginalTab=false;}if(redirectedOriginalTab){setTimeout(function(){try{window.close();}catch(_){ }if(statusEl){statusEl.textContent='Pulse Pro billing has been reopened in the original tab. You can close this window if it stays open.';}},75);return;}if(statusEl){statusEl.textContent='Returning to Pulse Pro billing.';}setTimeout(function(){window.location.replace(redirectPath);},150);}());</script></body></html>",
		escapedMessage,
		escapedLink,
		redirectPath,
		strings.TrimSpace(purchaseResult),
	)
}

func writeLicensePurchaseActivationSuccessPage(w http.ResponseWriter, feature string) {
	redirectPath := licensePurchaseActivationRedirectPath(feature, selfHostedBillingPurchaseActivated)
	escapedLink := html.EscapeString(redirectPath)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(
		w,
		"<!DOCTYPE html><html lang=\"en\"><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width, initial-scale=1\"><title>Pulse Pro activated</title></head><body><main style=\"max-width:32rem;margin:3rem auto;padding:0 1rem;font:16px/1.5 -apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif\"><h1 style=\"margin-bottom:0.5rem\">Pulse Pro activated</h1><p id=\"purchase-return-status\" style=\"margin-bottom:1rem\">Pulse is refreshing billing in the original tab.</p><p><a href=\"%s\">Return to Pulse Pro billing</a></p></main><script>(function(){var redirectPath=%q;var statusEl=document.getElementById('purchase-return-status');var redirectedOriginalTab=false;try{if(window.opener&&!window.opener.closed){redirectedOriginalTab=true;window.opener.postMessage({type:'pulse-license-purchase-activated',redirectPath:redirectPath},window.location.origin);window.opener.location.assign(redirectPath);}}catch(_){redirectedOriginalTab=false;}if(redirectedOriginalTab){setTimeout(function(){try{window.close();}catch(_){ }if(statusEl){statusEl.textContent='Pulse Pro is ready. You can close this tab if it stays open.';}},75);return;}if(statusEl){statusEl.textContent='Pulse Pro is ready. Returning to billing.';}setTimeout(function(){window.location.replace(redirectPath);},75);}());</script></body></html>",
		escapedLink,
		redirectPath,
	)
}

func writeLicensePurchaseActivationContinuePage(
	w http.ResponseWriter,
	sessionID string,
	portalHandoffID string,
	feature string,
	returnToken string,
) {
	escapedFeature := html.EscapeString(feature)
	escapedReturnToken := html.EscapeString(returnToken)
	escapedSessionID := html.EscapeString(sessionID)
	escapedPortalHandoffID := html.EscapeString(strings.TrimSpace(portalHandoffID))
	featureInput := ""
	if strings.TrimSpace(feature) != "" {
		featureInput = fmt.Sprintf("<input type=\"hidden\" name=\"feature\" value=\"%s\">", escapedFeature)
	}
	portalHandoffInput := ""
	if strings.TrimSpace(portalHandoffID) != "" {
		portalHandoffInput = fmt.Sprintf("<input type=\"hidden\" name=\"%s\" value=\"%s\">", pulseAccountPortalHandoffIDField, escapedPortalHandoffID)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(
		w,
		"<!DOCTYPE html><html lang=\"en\"><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width, initial-scale=1\"><title>Finalizing Pulse Pro upgrade</title></head><body><main style=\"max-width:32rem;margin:3rem auto;padding:0 1rem;font:16px/1.5 -apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif\"><h1 style=\"margin-bottom:0.5rem\">Finalizing Pulse Pro upgrade</h1><p id=\"purchase-activation-continue-status\" style=\"margin-bottom:1rem\">Pulse is securely finalizing the completed checkout.</p><form id=\"purchase-activation-continue-form\" method=\"POST\" action=\"%s\"><input type=\"hidden\" name=\"%s\" value=\"%s\"><input type=\"hidden\" name=\"%s\" value=\"%s\">%s%s<button type=\"submit\">Continue to Pulse Pro</button></form></main><script>(function(){var form=document.getElementById('purchase-activation-continue-form');var statusEl=document.getElementById('purchase-activation-continue-status');if(statusEl){statusEl.textContent='Pulse is finishing the upgrade and returning you to billing.';}if(form&&typeof form.submit==='function'){setTimeout(function(){form.submit();},50);}}());</script></body></html>",
		licensePurchaseActivationPath,
		licensePurchaseSessionIDField,
		escapedSessionID,
		licensePurchaseReturnTokenField,
		escapedReturnToken,
		featureInput,
		portalHandoffInput,
	)
}

func (h *LicenseHandlers) verifiedPurchaseReturnClaims(
	r *http.Request,
	returnToken string,
) (*purchaseReturnClaimsModel, error) {
	expectedHost := purchaseReturnExpectedHost(r, h.cfg)
	if expectedHost == "" {
		return nil, fmt.Errorf("purchase return expected host is unavailable")
	}
	signingKey, err := h.purchaseReturnSigningKey()
	if err != nil {
		return nil, err
	}
	return verifyPurchaseReturnTokenFromLicensing(returnToken, signingKey, expectedHost, time.Now().UTC())
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
		svc.SetClientVersion(h.runtimeVersion)
		if err := h.ensureEvaluatorForOrg(orgID, svc); err != nil {
			log.Warn().Str("org_id", orgID).Err(err).Msg("Failed to refresh license evaluator for org")
		}
		h.ensureHostedEntitlementRefreshForOrg(orgID, svc)
		// We need persistence too, reconstruct it or cache it?
		// Reconstructing persistence is cheap (just a struct with path).
		// But let's recreate it to be safe and stateless here.
		// Actually, we need the EXACT persistence object if it holds state, but license.Persistence seems stateless (file I/O).
		p, err := h.getPersistenceForOrg(orgID)
		h.syncReleaseDemoFixtureRuntime(orgID, svc)
		return svc, p, err
	}

	// Initialize for this tenant
	persistence, err := h.getPersistenceForOrg(orgID)
	if err != nil {
		return nil, nil, err
	}

	service := newLicenseService()
	service.SetClientVersion(h.runtimeVersion)
	h.bindLegacyGrandfatherReconcileOwnership(orgID, service)

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
				h.reconcileLegacyMigrationGrandfatherFloor(ctx, orgID, service)
				if clearErr := h.setCommercialMigrationState(orgID, nil); clearErr != nil {
					log.Warn().Str("org_id", orgID).Err(clearErr).Msg("Failed to clear commercial migration state after activation restore")
				}
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
				if persistErr := h.setCommercialMigrationState(orgID, classifyLegacyExchangeErrorFromLicensing(err)); persistErr != nil {
					log.Warn().Str("org_id", orgID).Err(persistErr).Msg("Failed to persist commercial migration state")
				}
				log.Warn().Str("org_id", orgID).Err(err).Msg("Failed to auto-exchange persisted legacy license")
			} else if service.IsActivated() {
				h.reconcileLegacyMigrationGrandfatherFloor(ctx, orgID, service)
				if clearErr := h.setCommercialMigrationState(orgID, nil); clearErr != nil {
					log.Warn().Str("org_id", orgID).Err(clearErr).Msg("Failed to clear commercial migration state after successful auto-exchange")
				}
				service.StartGrantRefresh(context.Background())
				if feedToken := revocationFeedToken(); feedToken != "" {
					service.StartRevocationPoll(context.Background(), feedToken)
				}
				if current := service.Current(); current != nil {
					log.Info().
						Str("org_id", orgID).
						Str("license_id", current.Claims.LicenseID).
						Msg("Auto-exchanged persisted legacy license while preserving downgrade fallback")
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
		// Re-home legacy continuity ownership onto the canonical service when
		// concurrent first-request initialization races create an orphan.
		h.stopLegacyGrandfatherReconcileLoop(orgID)
		h.syncLegacyGrandfatherReconcileOwnership(orgID, svc)
		h.ensureHostedEntitlementRefreshForOrg(orgID, svc)
		p, pErr := h.getPersistenceForOrg(orgID)
		h.syncReleaseDemoFixtureRuntime(orgID, svc)
		return svc, p, pErr
	}

	h.syncLegacyGrandfatherReconcileOwnership(orgID, service)
	h.ensureHostedEntitlementRefreshForOrg(orgID, service)
	h.syncReleaseDemoFixtureRuntime(orgID, service)

	return service, persistence, nil
}

func (h *LicenseHandlers) demoFixturesAuthorized(service *licenseService) bool {
	return h != nil &&
		h.cfg != nil &&
		h.cfg.DemoMode &&
		service != nil &&
		service.HasFeature(featureDemoFixturesValue)
}

func (h *LicenseHandlers) syncReleaseDemoFixtureRuntime(orgID string, service *licenseService) {
	if !shouldEnforceReleaseDemoFixtureRuntime() {
		return
	}

	if strings.TrimSpace(orgID) == "" {
		orgID = "default"
	}
	if orgID != "default" {
		return
	}

	authorized := h.demoFixturesAuthorized(service)
	mock.SetReleaseFixturesAuthorized(authorized)

	if !authorized {
		if h.monitor != nil {
			if err := h.monitor.SetMockMode(false); err != nil {
				log.Warn().Err(err).Str("org_id", orgID).Msg("Failed to disable unauthorized demo fixtures")
			}
		} else if err := mock.SetEnabled(false); err != nil {
			log.Warn().Err(err).Str("org_id", orgID).Msg("Failed to disable unauthorized demo fixtures")
		}
		return
	}

	if !wantsMockFixturesFromEnv() {
		return
	}

	if h.monitor != nil {
		if err := h.monitor.SetMockMode(true); err != nil {
			log.Warn().Err(err).Str("org_id", orgID).Msg("Failed to enable entitled demo fixtures")
		}
		return
	}

	if err := mock.SetEnabled(true); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("Failed to enable entitled demo fixtures")
	}
}

func (h *LicenseHandlers) canonicalMonitoredSystemGrandfatherFloor(ctx context.Context) (int, bool) {
	usage := h.entitlementUsageSnapshot(ctx)
	if !usage.MonitoredSystemsAvailable {
		return 0, false
	}
	if usage.MonitoredSystems < 0 {
		return 0, false
	}
	return int(usage.MonitoredSystems), true
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

	// Hosted tenant runtimes still carry instance-scoped billing state in the
	// root billing.json. If a tenant-scoped org has no org-local billing state,
	// inherit the default-org lease so hosted auth handoff can preserve tenant
	// org context without dropping entitlements on first runtime entry.
	if h.hostedMode && orgID != "default" && orgID != "" {
		evaluatorOrgID := orgID
		state, err := billingStore.GetBillingState(orgID)
		if err != nil {
			return fmt.Errorf("load hosted billing state for org %q: %w", orgID, err)
		}
		if state == nil || state.SubscriptionState == "" {
			defaultState, defaultErr := billingStore.GetBillingState("default")
			if defaultErr != nil {
				return fmt.Errorf("load hosted default billing state fallback: %w", defaultErr)
			}
			if defaultState == nil || defaultState.SubscriptionState == "" {
				service.SetEvaluator(nil)
				return nil
			}
			evaluatorOrgID = "default"
		}
		evaluator := newLicenseEvaluatorForBillingStoreFromLicensing(billingStore, evaluatorOrgID, time.Hour, entitlementExpectedInstanceHost(h.cfg))
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
	evaluator := newLicenseEvaluatorForBillingStoreFromLicensing(billingStore, evaluatorOrgID, 0, entitlementExpectedInstanceHost(h.cfg))
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

func (h *LicenseHandlers) setCommercialMigrationState(orgID string, status *commercialMigrationStatusModel) error {
	if h == nil || h.mtPersistence == nil {
		return nil
	}

	billingStore := config.NewFileBillingStore(h.mtPersistence.BaseDataDir())
	existing, err := billingStore.GetBillingState(orgID)
	if err != nil {
		return err
	}
	if existing == nil {
		if status == nil {
			return nil
		}
		existing = &billingState{
			Capabilities:      []string{},
			Limits:            map[string]int64{},
			MetersEnabled:     []string{},
			PlanVersion:       string(subscriptionStateExpiredValue),
			SubscriptionState: subscriptionStateExpiredValue,
		}
	} else {
		existing = normalizeBillingStateFromLicensing(existing)
		if existing.SubscriptionState == "" {
			existing.PlanVersion = string(subscriptionStateExpiredValue)
			existing.SubscriptionState = subscriptionStateExpiredValue
		}
	}

	existing.CommercialMigration = cloneCommercialMigrationStatusFromLicensing(status)
	return billingStore.SaveBillingState(orgID, existing)
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
// SaaS trials are initiated through hosted signup; the local instance only redeems
// a signed activation token via /auth/trial-activate.
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

	if h.trialLimiter != nil {
		allowed, retryDelay := h.trialLimiter.allowAt(orgID, time.Now().UTC())
		if !allowed {
			retryAfterSeconds := int(retryDelay.Round(time.Second).Seconds())
			if retryAfterSeconds < 1 {
				retryAfterSeconds = 1
			}
			w.Header().Set("Retry-After", strconv.Itoa(retryAfterSeconds))
			writeErrorResponse(w, http.StatusTooManyRequests, "trial_rate_limited", "Trial start rate limit exceeded", map[string]string{
				"org_id":              orgID,
				"retry_after_seconds": strconv.Itoa(retryAfterSeconds),
			})
			return
		}
	}

	if h.trialInitiations == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "trial_signup_unavailable", "Hosted trial signup is unavailable", nil)
		return
	}
	returnURL := trialCallbackURLForRequest(r, h.cfg)
	if returnURL == "" {
		log.Error().Str("org_id", orgID).Msg("Trial callback URL unavailable for initiation")
		writeErrorResponse(w, http.StatusServiceUnavailable, "trial_signup_unavailable", "Hosted trial signup is unavailable", nil)
		return
	}
	instanceToken, err := h.trialInitiations.issue(orgID, returnURL, time.Now().UTC().Add(trialSignupInitiationTTL))
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("Trial signup initiation token unavailable")
		writeErrorResponse(w, http.StatusServiceUnavailable, "trial_signup_unavailable", "Hosted trial signup is unavailable", nil)
		return
	}

	actionURL, err := trialSignupActionURLForRequest(h.cfg, orgID, returnURL, instanceToken)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("Trial signup redirect unavailable")
		writeErrorResponse(w, http.StatusServiceUnavailable, "trial_signup_unavailable", "Hosted trial signup is unavailable", nil)
		return
	}

	h.emitConversionEvent(orgID, conversionEvent{
		Type:    conversionEventCheckoutStarted,
		Surface: "license_api",
	})

	writeErrorResponse(w, http.StatusConflict, "trial_signup_required", "Complete hosted signup to start your trial", map[string]string{
		"org_id":     orgID,
		"action_url": actionURL,
	})
}

// HandleTrialActivation handles GET /auth/trial-activate.
// It verifies a hosted signup trial activation token, blocks replay, persists a
// signed hosted entitlement lease, then redirects to Settings.
func (h *LicenseHandlers) HandleTrialActivation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h == nil || h.mtPersistence == nil {
		http.Redirect(w, r, trialActivationResultURL("unavailable"), http.StatusTemporaryRedirect)
		return
	}

	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if token == "" {
		http.Redirect(w, r, trialActivationResultURL("invalid"), http.StatusTemporaryRedirect)
		return
	}

	publicKey, err := trialActivationPublicKeyFromLicensing()
	if err != nil {
		log.Error().Err(err).Msg("Trial activation public key not configured")
		http.Redirect(w, r, trialActivationResultURL("unavailable"), http.StatusTemporaryRedirect)
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
			http.Redirect(w, r, trialActivationResultURL("unavailable"), http.StatusTemporaryRedirect)
			return
		}
	} else {
		expectedHost = normalizeHostForTrial(r.Host)
	}
	if expectedHost == "" {
		log.Error().Msg("Could not determine host for trial activation binding")
		http.Redirect(w, r, trialActivationResultURL("unavailable"), http.StatusTemporaryRedirect)
		return
	}
	claims, err := verifyTrialActivationTokenFromLicensing(token, publicKey, expectedHost, time.Now().UTC())
	if err != nil {
		log.Warn().Err(err).Msg("Trial activation token verification failed")
		http.Redirect(w, r, trialActivationResultURL("invalid"), http.StatusTemporaryRedirect)
		return
	}

	replayStore := h.trialReplay
	if replayStore == nil {
		replayStore = &jtiReplayStore{configDir: h.mtPersistence.BaseDataDir()}
	}
	replaySubject := strings.TrimSpace(claims.Subject)
	if replaySubject == "" {
		replaySubject = strings.TrimSpace(claims.ID)
	}
	if replaySubject == "" {
		log.Warn().Msg("Trial activation token missing subject and jti")
		http.Redirect(w, r, trialActivationResultURL("invalid"), http.StatusTemporaryRedirect)
		return
	}
	replayID := "trial_activate:" + replaySubject
	expiresAt := time.Now().UTC().Add(15 * time.Minute)
	if claims.ExpiresAt != nil {
		expiresAt = claims.ExpiresAt.Time
	}
	stored, err := replayStore.checkAndStore(replayID, expiresAt)
	if err != nil {
		log.Error().Err(err).Msg("Trial activation replay-store failure")
		http.Redirect(w, r, trialActivationResultURL("unavailable"), http.StatusTemporaryRedirect)
		return
	}
	if !stored {
		log.Warn().Str("replay_id_prefix", replayID[:24]).Msg("Trial activation token replay blocked")
		http.Redirect(w, r, trialActivationResultURL("replayed"), http.StatusTemporaryRedirect)
		return
	}
	clearReplay := func(reason string, err error) {
		if replayStore == nil {
			return
		}
		if deleteErr := replayStore.delete(replayID); deleteErr != nil {
			log.Warn().Err(deleteErr).Str("replay_id_prefix", replayID[:24]).Str("reason", reason).Msg("Trial activation replay-store cleanup failed")
			return
		}
		if err != nil {
			log.Warn().Err(err).Str("replay_id_prefix", replayID[:24]).Str("reason", reason).Msg("Cleared trial activation replay marker after transient failure")
		}
	}

	orgID := strings.TrimSpace(claims.OrgID)
	if orgID == "" {
		orgID = "default"
	}
	if h.trialInitiations == nil {
		clearReplay("initiation_store_unavailable", nil)
		http.Redirect(w, r, trialActivationResultURL("unavailable"), http.StatusTemporaryRedirect)
		return
	}
	returnURL := strings.TrimSpace(claims.ReturnURL)
	ok, err := h.trialInitiations.validate(orgID, returnURL, claims.InstanceToken, time.Now().UTC())
	if err != nil {
		clearReplay("initiation_validation_failed", err)
		log.Error().Err(err).Str("org_id", orgID).Msg("Trial activation initiation token validation failed")
		http.Redirect(w, r, trialActivationResultURL("unavailable"), http.StatusTemporaryRedirect)
		return
	}
	if !ok {
		log.Warn().Str("org_id", orgID).Msg("Trial activation missing or invalid initiation token")
		http.Redirect(w, r, trialActivationResultURL("invalid"), http.StatusTemporaryRedirect)
		return
	}

	ctx := context.WithValue(r.Context(), OrgIDContextKey, orgID)
	svc, _, err := h.getTenantComponents(ctx)
	if err != nil {
		clearReplay("tenant_resolution_failed", err)
		log.Error().Err(err).Str("org_id", orgID).Msg("Trial activation tenant resolution failed")
		http.Redirect(w, r, trialActivationResultURL("unavailable"), http.StatusTemporaryRedirect)
		return
	}

	billingStore := config.NewFileBillingStore(h.mtPersistence.BaseDataDir())
	existing, err := billingStore.GetBillingState(orgID)
	if err != nil {
		clearReplay("billing_state_load_failed", err)
		log.Error().Err(err).Str("org_id", orgID).Msg("Trial activation billing state load failed")
		http.Redirect(w, r, trialActivationResultURL("unavailable"), http.StatusTemporaryRedirect)
		return
	}
	decision := evaluateTrialStartEligibilityFromLicensing(svc.Current() != nil && svc.IsValid(), existing)
	if !decision.Allowed {
		log.Info().
			Str("org_id", orgID).
			Str("reason", string(decision.Reason)).
			Msg("Trial activation denied due to ineligible state")
		http.Redirect(w, r, trialActivationResultURL("ineligible"), http.StatusTemporaryRedirect)
		return
	}

	redeemer := h.trialRedeemer
	if redeemer == nil {
		redeemer = h.acknowledgeHostedTrialRedemption
	}
	var redemption *hostedTrialRedemptionResponse
	if redeemer != nil {
		redemption, err = redeemer(token)
		if err != nil {
			clearReplay("redemption_ack_failed", err)
			log.Warn().Err(err).Str("org_id", orgID).Msg("Hosted trial redemption acknowledgement failed")
			http.Redirect(w, r, trialActivationResultURL("unavailable"), http.StatusTemporaryRedirect)
			return
		}
	}
	entitlementJWT := ""
	entitlementRefreshToken := ""
	if redemption != nil {
		entitlementJWT = strings.TrimSpace(redemption.EntitlementJWT)
		entitlementRefreshToken = strings.TrimSpace(redemption.EntitlementRefreshToken)
	}
	if strings.TrimSpace(entitlementJWT) == "" {
		clearReplay("entitlement_lease_missing", nil)
		log.Warn().Str("org_id", orgID).Msg("Hosted trial redemption returned no entitlement lease")
		http.Redirect(w, r, trialActivationResultURL("unavailable"), http.StatusTemporaryRedirect)
		return
	}
	if entitlementRefreshToken == "" {
		clearReplay("entitlement_refresh_token_missing", nil)
		log.Warn().Str("org_id", orgID).Msg("Hosted trial redemption returned no entitlement refresh token")
		http.Redirect(w, r, trialActivationResultURL("unavailable"), http.StatusTemporaryRedirect)
		return
	}
	leaseClaims, err := verifyEntitlementLeaseTokenFromLicensing(entitlementJWT, publicKey, expectedHost, time.Now().UTC())
	if err != nil {
		clearReplay("entitlement_lease_invalid", err)
		log.Warn().Err(err).Str("org_id", orgID).Msg("Hosted trial entitlement lease verification failed")
		http.Redirect(w, r, trialActivationResultURL("unavailable"), http.StatusTemporaryRedirect)
		return
	}
	if strings.TrimSpace(leaseClaims.OrgID) != orgID {
		clearReplay("entitlement_lease_org_mismatch", nil)
		log.Warn().
			Str("org_id", orgID).
			Str("lease_org_id", strings.TrimSpace(leaseClaims.OrgID)).
			Msg("Hosted trial entitlement lease org mismatch")
		http.Redirect(w, r, trialActivationResultURL("invalid"), http.StatusTemporaryRedirect)
		return
	}

	state := &billingState{}
	if existing != nil {
		state = normalizeBillingStateFromLicensing(existing)
	}
	// Keep only the signed hosted lease plus trial-used bookkeeping locally.
	// Effective Pro capabilities and limits must resolve from the lease on read.
	state.EntitlementJWT = entitlementJWT
	state.EntitlementRefreshToken = entitlementRefreshToken
	state.Capabilities = []string{}
	state.Limits = map[string]int64{}
	state.MetersEnabled = []string{}
	state.PlanVersion = ""
	state.SubscriptionState = ""
	state.TrialStartedAt = leaseClaims.TrialStartedAt
	state.TrialEndsAt = nil
	state.TrialExtendedAt = nil
	if err := billingStore.SaveBillingState(orgID, state); err != nil {
		clearReplay("billing_state_save_failed", err)
		log.Error().Err(err).Str("org_id", orgID).Msg("Trial activation billing state save failed")
		http.Redirect(w, r, trialActivationResultURL("unavailable"), http.StatusTemporaryRedirect)
		return
	}
	if svc.Current() == nil {
		eval := newLicenseEvaluatorForBillingStoreFromLicensing(billingStore, orgID, 0, expectedHost)
		svc.SetEvaluator(eval)
	}
	h.ensureHostedEntitlementRefreshForOrg(orgID, svc)

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

	if consumed, consumeErr := h.trialInitiations.consume(orgID, returnURL, claims.InstanceToken, time.Now().UTC()); consumeErr != nil {
		log.Warn().Err(consumeErr).Str("org_id", orgID).Msg("Trial initiation token consume failed after activation")
	} else if !consumed {
		log.Warn().Str("org_id", orgID).Msg("Trial initiation token was not consumed after activation")
	}

	log.Info().
		Str("org_id", orgID).
		Str("email", strings.TrimSpace(claims.Email)).
		Msg("Trial activation succeeded")

	http.Redirect(w, r, trialActivationResultURL("activated"), http.StatusTemporaryRedirect)
}

func trialActivationResultURL(result string) string {
	return "/settings/system-pro?trial=" + url.QueryEscape(strings.TrimSpace(result))
}

func normalizeTrialOrgID(raw string) string {
	orgID := strings.TrimSpace(raw)
	if orgID == "" {
		return "default"
	}
	return orgID
}

type hostedTrialRedemptionResponse struct {
	EntitlementJWT          string `json:"entitlement_jwt"`
	EntitlementRefreshToken string `json:"entitlement_refresh_token"`
}

type hostedTrialLeaseRefreshRequest struct {
	OrgID                   string `json:"org_id"`
	InstanceHost            string `json:"instance_host"`
	EntitlementRefreshToken string `json:"entitlement_refresh_token"`
}

type hostedTrialLeaseRefreshResponse struct {
	EntitlementJWT string `json:"entitlement_jwt"`
}

func (h *LicenseHandlers) acknowledgeHostedTrialRedemption(token string) (*hostedTrialRedemptionResponse, error) {
	if h == nil {
		return nil, nil
	}
	redemptionURL := trialSignupRedemptionURLFromConfig(h.cfg)
	if redemptionURL == "" {
		return nil, nil
	}
	payload, err := json.Marshal(map[string]string{"token": strings.TrimSpace(token)})
	if err != nil {
		return nil, fmt.Errorf("marshal trial redemption payload: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, redemptionURL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build trial redemption request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("post trial redemption acknowledgement: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("trial redemption acknowledgement returned status %d", resp.StatusCode)
	}
	var response hostedTrialRedemptionResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("decode trial redemption acknowledgement: %w", err)
	}
	if strings.TrimSpace(response.EntitlementJWT) == "" {
		return nil, fmt.Errorf("trial redemption acknowledgement missing entitlement_jwt")
	}
	if strings.TrimSpace(response.EntitlementRefreshToken) == "" {
		return nil, fmt.Errorf("trial redemption acknowledgement missing entitlement_refresh_token")
	}
	return &response, nil
}

func trialSignupRedemptionURLFromConfig(cfg *config.Config) string {
	signupBaseURL := ""
	if cfg != nil {
		signupBaseURL = strings.TrimSpace(cfg.ProTrialSignupURL)
	}
	signupURL := strings.TrimSpace(proTrialSignupURLFromLicensing(signupBaseURL))
	if signupURL == "" {
		return ""
	}
	parsed, err := url.Parse(signupURL)
	if err != nil || parsed == nil {
		return ""
	}
	parsed.Path = "/api/trial-signup/redeem"
	parsed.RawQuery = ""
	return parsed.String()
}

func hostedEntitlementRefreshURLFromConfig(cfg *config.Config) string {
	signupBaseURL := ""
	if cfg != nil {
		signupBaseURL = strings.TrimSpace(cfg.ProTrialSignupURL)
	}
	signupURL := strings.TrimSpace(proTrialSignupURLFromLicensing(signupBaseURL))
	if signupURL == "" {
		return ""
	}
	parsed, err := url.Parse(signupURL)
	if err != nil || parsed == nil {
		return ""
	}
	parsed.Path = "/api/entitlements/refresh"
	parsed.RawQuery = ""
	return parsed.String()
}

func entitlementExpectedInstanceHost(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	return normalizeHostForTrial(cfg.PublicURL)
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
	if strings.Contains(msg, "supported v6 activation key") || strings.Contains(msg, "migratable v5 license") {
		return "This key is not a valid Pulse v6 activation key or a supported Pulse v5 license for migration. Paste a v6 activation key, or a valid v5 Pro/Lifetime license and Pulse will exchange it automatically."
	}
	if strings.Contains(msg, "license server client not configured") {
		return "License activation is temporarily unavailable. Please try again later or contact support."
	}

	return "License activation failed. Please try again or contact support if the problem persists."
}

// HandleActivateLicense handles POST /api/license/activate
// Validates and activates a license key.
func (h *LicenseHandlers) activateLicenseKey(ctx context.Context, licenseKey string) (ActivateLicenseResponse, error) {
	service, persistence, err := h.getTenantComponents(ctx)
	if err != nil {
		return ActivateLicenseResponse{}, fmt.Errorf("resolve tenant licensing components: %w", err)
	}

	orgID := GetOrgID(ctx)
	trimmedKey := strings.TrimSpace(licenseKey)
	migratedLegacyKey := !strings.HasPrefix(trimmedKey, activationKeyPrefixValue)

	lic, err := service.Activate(trimmedKey)
	if err != nil {
		h.emitConversionEvent(orgID, conversionEvent{
			Type:    conversionEventLicenseActivationFailed,
			Surface: "license_api",
		})
		return ActivateLicenseResponse{}, err
	}

	if service.IsActivated() {
		if migratedLegacyKey {
			h.reconcileLegacyMigrationGrandfatherFloor(ctx, orgID, service)
		}
		h.stopHostedEntitlementRefreshLoop(orgID)
		if clearErr := h.setCommercialMigrationState(orgID, nil); clearErr != nil {
			log.Warn().Err(clearErr).Str("org_id", orgID).Msg("Failed to clear commercial migration state after activation")
		}
		// Activation state is already persisted by ActivateWithKey, but start the refresh loop.
		service.StartGrantRefresh(context.Background())
		if feedToken := revocationFeedToken(); feedToken != "" {
			service.StartRevocationPoll(context.Background(), feedToken)
		}
		if persistence != nil {
			if strings.HasPrefix(trimmedKey, activationKeyPrefixValue) {
				_ = persistence.Delete()
			} else if err := persistence.Save(trimmedKey); err != nil {
				log.Warn().Err(err).Msg("Failed to persist migrated legacy license for downgrade fallback")
			}
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

	h.syncReleaseDemoFixtureRuntime(orgID, service)

	h.emitConversionEvent(orgID, conversionEvent{
		Type:    conversionEventLicenseActivated,
		Surface: "license_api",
	})

	successMessage := "License activated successfully"
	if migratedLegacyKey && service.IsActivated() {
		successMessage = "Pulse v5 license migrated and activated successfully"
	}

	return ActivateLicenseResponse{
		Success: true,
		Message: successMessage,
		Status:  service.Status(),
	}, nil
}

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

	if strings.TrimSpace(req.LicenseKey) == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ActivateLicenseResponse{
			Success: false,
			Message: "License key is required",
		})
		return
	}

	response, err := h.activateLicenseKey(r.Context(), req.LicenseKey)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to activate license")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ActivateLicenseResponse{
			Success: false,
			Message: userFriendlyActivationError(err),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleCheckoutStart handles GET /auth/license-purchase-start.
// It mints a short-lived signed return token and redirects the operator into
// Pulse Account so checkout can safely return to the local instance.
func (h *LicenseHandlers) HandleCheckoutStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	expectedHost := purchaseReturnExpectedHost(r, h.cfg)
	returnURL := licensePurchaseCallbackURLForRequest(r, h.cfg)
	if expectedHost == "" || returnURL == "" {
		http.Error(w, "Pulse Account handoff unavailable", http.StatusServiceUnavailable)
		return
	}

	signingKey, err := h.purchaseReturnSigningKey()
	if err != nil {
		log.Error().Err(err).Msg("Purchase return signing key unavailable")
		http.Error(w, "Pulse Account handoff unavailable", http.StatusServiceUnavailable)
		return
	}

	orgID := strings.TrimSpace(GetOrgID(r.Context()))
	if orgID == "" {
		orgID = "default"
	}
	feature := strings.TrimSpace(r.URL.Query().Get(pulseAccountPortalFeatureQueryParam))
	returnToken, err := signPurchaseReturnTokenFromLicensing(signingKey, purchaseReturnClaimsModel{
		OrgID:        orgID,
		Feature:      feature,
		InstanceHost: expectedHost,
		ReturnURL:    returnURL,
	})
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Str("feature", feature).Msg("Failed to sign purchase return token")
		http.Error(w, "Pulse Account handoff unavailable", http.StatusServiceUnavailable)
		return
	}

	activationURLTemplate, err := licensePurchaseActivationTemplateURL(returnURL, returnToken)
	if err != nil {
		log.Error().Err(err).Str("feature", feature).Msg("Failed to build Pulse Account activation template")
		http.Error(w, "Pulse Account handoff unavailable", http.StatusServiceUnavailable)
		return
	}

	cancelURL, err := publicAbsoluteURLForPath(
		r,
		h.cfg,
		licensePurchaseActivationRedirectPath(feature, selfHostedBillingPurchaseCancelled),
	)
	if err != nil {
		log.Error().Err(err).Str("feature", feature).Msg("Failed to build Pulse Account cancellation return url")
		http.Error(w, "Pulse Account handoff unavailable", http.StatusServiceUnavailable)
		return
	}

	lsClient := newLicenseServerClientFromLicensing("")
	if lsClient == nil {
		http.Error(w, "Pulse Account handoff unavailable", http.StatusServiceUnavailable)
		return
	}

	purchaseReturnJTI := ""
	if claims, verifyErr := h.verifiedPurchaseReturnClaims(r, returnToken); verifyErr == nil && claims != nil {
		purchaseReturnJTI = strings.TrimSpace(claims.ID)
	}
	if purchaseReturnJTI == "" {
		log.Error().Str("feature", feature).Msg("Pulse Account purchase return token omitted jti")
		http.Error(w, "Pulse Account handoff unavailable", http.StatusServiceUnavailable)
		return
	}

	portalHandoff, err := lsClient.CreateCheckoutPortalHandoff(r.Context(), checkoutPortalHandoffRequestModel{
		Feature:           feature,
		SuccessURL:        activationURLTemplate,
		CancelURL:         cancelURL,
		PurchaseReturnJTI: purchaseReturnJTI,
	})
	if err != nil {
		log.Error().Err(err).Str("feature", feature).Msg("Failed to create Pulse Account checkout portal handoff")
		http.Error(w, "Pulse Account handoff unavailable", http.StatusServiceUnavailable)
		return
	}
	portalHandoffID := strings.TrimSpace(portalHandoff.PortalHandoffID)
	if portalHandoffID == "" {
		log.Error().Str("feature", feature).Msg("Pulse Account checkout portal handoff response omitted id")
		http.Error(w, "Pulse Account handoff unavailable", http.StatusServiceUnavailable)
		return
	}

	destination, err := pulseAccountUpgradeURLForRequest(portalHandoffID, r.URL.Query())
	if err != nil {
		log.Error().Err(err).Str("feature", feature).Msg("Failed to build Pulse Account upgrade destination")
		http.Error(w, "Pulse Account handoff unavailable", http.StatusServiceUnavailable)
		return
	}

	http.Redirect(w, r, destination, http.StatusSeeOther)
}

// HandleCheckoutActivation handles the local checkout return at
// /auth/license-purchase-activate. GET renders the auto-submitting bridge for
// a completed Stripe redirect, while POST redeems the completed session into a
// local activation and returns the operator to the canonical billing route.
func (h *LicenseHandlers) HandleCheckoutActivation(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		sessionID := strings.TrimSpace(r.URL.Query().Get(licensePurchaseSessionIDField))
		portalHandoffID := strings.TrimSpace(r.URL.Query().Get(pulseAccountPortalHandoffIDField))
		feature := strings.TrimSpace(r.URL.Query().Get("feature"))
		returnToken := strings.TrimSpace(r.URL.Query().Get(licensePurchaseReturnTokenField))
		if returnToken == "" || sessionID == "" {
			writeLicensePurchaseActivationFailurePage(w, http.StatusBadRequest, feature, selfHostedBillingPurchaseExpired, "Purchase activation link expired or is missing required state. Reopen the upgrade flow from Pulse Pro billing.")
			return
		}
		claims, err := h.verifiedPurchaseReturnClaims(r, returnToken)
		if err != nil {
			log.Warn().Err(err).Msg("Purchase return token verification failed during GET bridge")
			writeLicensePurchaseActivationFailurePage(w, http.StatusBadRequest, feature, selfHostedBillingPurchaseExpired, "Purchase activation link expired or is invalid. Reopen the upgrade flow from Pulse Pro billing.")
			return
		}
		if claims != nil && strings.TrimSpace(claims.Feature) != "" {
			feature = strings.TrimSpace(claims.Feature)
		}
		writeLicensePurchaseActivationContinuePage(w, sessionID, portalHandoffID, feature, returnToken)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		writeLicensePurchaseActivationFailurePage(w, http.StatusBadRequest, "", selfHostedBillingPurchaseFailed, "Purchase activation request was invalid.")
		return
	}

	sessionID := strings.TrimSpace(r.FormValue(licensePurchaseSessionIDField))
	portalHandoffID := strings.TrimSpace(r.FormValue(pulseAccountPortalHandoffIDField))
	feature := strings.TrimSpace(r.FormValue("feature"))
	returnToken := strings.TrimSpace(r.FormValue(licensePurchaseReturnTokenField))
	if returnToken == "" {
		writeLicensePurchaseActivationFailurePage(w, http.StatusBadRequest, feature, selfHostedBillingPurchaseExpired, "Purchase activation link expired or is missing required state. Reopen the upgrade flow from Pulse Pro billing.")
		return
	}

	claims, err := h.verifiedPurchaseReturnClaims(r, returnToken)
	if err != nil {
		log.Warn().Err(err).Msg("Purchase return token verification failed")
		writeLicensePurchaseActivationFailurePage(w, http.StatusBadRequest, feature, selfHostedBillingPurchaseExpired, "Purchase activation link expired or is invalid. Reopen the upgrade flow from Pulse Pro billing.")
		return
	}
	if claims != nil && strings.TrimSpace(claims.Feature) != "" {
		if feature != "" && feature != strings.TrimSpace(claims.Feature) {
			writeLicensePurchaseActivationFailurePage(w, http.StatusBadRequest, strings.TrimSpace(claims.Feature), selfHostedBillingPurchaseExpired, "Purchase activation state did not match the completed upgrade flow.")
			return
		}
		feature = strings.TrimSpace(claims.Feature)
	}
	if sessionID == "" {
		writeLicensePurchaseActivationFailurePage(w, http.StatusBadRequest, feature, selfHostedBillingPurchaseFailed, "Purchase activation requires a completed checkout session.")
		return
	}

	lsClient := newLicenseServerClientFromLicensing("")
	if lsClient == nil {
		writeLicensePurchaseActivationFailurePage(w, http.StatusServiceUnavailable, feature, selfHostedBillingPurchaseFailed, "Pulse could not contact the commercial activation service.")
		return
	}

	ctx := r.Context()
	if claims != nil && strings.TrimSpace(claims.OrgID) != "" {
		ctx = context.WithValue(ctx, OrgIDContextKey, strings.TrimSpace(claims.OrgID))
	}

	checkoutResult, err := lsClient.GetCheckoutSessionResult(ctx, sessionID)
	if err != nil {
		log.Warn().Err(err).Str("checkout_session_id", sessionID).Msg("Failed to resolve checkout session during purchase activation")
		writeLicensePurchaseActivationFailurePage(w, http.StatusBadGateway, feature, selfHostedBillingPurchaseFailed, "Pulse could not confirm the completed checkout yet. Please try again in a moment.")
		return
	}

	if checkoutResult == nil || strings.TrimSpace(checkoutResult.Status) != "fulfilled" {
		message := "Checkout has not completed yet."
		if checkoutResult != nil && strings.TrimSpace(checkoutResult.Message) != "" {
			message = strings.TrimSpace(checkoutResult.Message)
		}
		writeLicensePurchaseActivationFailurePage(w, http.StatusConflict, feature, selfHostedBillingPurchaseFailed, message)
		return
	}
	expectedPurchaseReturnJTI := ""
	if claims != nil {
		expectedPurchaseReturnJTI = strings.TrimSpace(claims.ID)
	}
	resolvedPurchaseReturnJTI := ""
	if checkoutResult != nil {
		resolvedPurchaseReturnJTI = strings.TrimSpace(checkoutResult.PurchaseReturnJTI)
	}
	resolvedPortalHandoffID := ""
	if checkoutResult != nil {
		resolvedPortalHandoffID = strings.TrimSpace(checkoutResult.PortalHandoffID)
	}
	if portalHandoffID != "" {
		if resolvedPortalHandoffID == "" || resolvedPortalHandoffID != portalHandoffID {
			log.Warn().
				Str("checkout_session_id", sessionID).
				Str("expected_portal_handoff_id", portalHandoffID).
				Str("resolved_portal_handoff_id", resolvedPortalHandoffID).
				Msg("Rejected checkout activation due to portal handoff binding mismatch")
			writeLicensePurchaseActivationFailurePage(w, http.StatusConflict, feature, selfHostedBillingPurchaseExpired, "Purchase activation state did not match the completed upgrade flow. Reopen the upgrade flow from Pulse Pro billing.")
			return
		}
	} else {
		portalHandoffID = resolvedPortalHandoffID
	}
	if portalHandoffID == "" {
		log.Warn().
			Str("checkout_session_id", sessionID).
			Msg("Rejected checkout activation without canonical portal handoff binding")
		writeLicensePurchaseActivationFailurePage(w, http.StatusConflict, feature, selfHostedBillingPurchaseExpired, "Purchase activation state did not match the completed upgrade flow. Reopen the upgrade flow from Pulse Pro billing.")
		return
	}
	if expectedPurchaseReturnJTI == "" || resolvedPurchaseReturnJTI == "" || expectedPurchaseReturnJTI != resolvedPurchaseReturnJTI {
		log.Warn().
			Str("checkout_session_id", sessionID).
			Str("expected_purchase_return_jti", expectedPurchaseReturnJTI).
			Str("resolved_purchase_return_jti", resolvedPurchaseReturnJTI).
			Msg("Rejected checkout activation due to purchase return binding mismatch")
		writeLicensePurchaseActivationFailurePage(w, http.StatusConflict, feature, selfHostedBillingPurchaseExpired, "Purchase activation state did not match the completed upgrade flow. Reopen the upgrade flow from Pulse Pro billing.")
		return
	}

	activationKey := strings.TrimSpace(checkoutResult.ActivationKey)
	if activationKey == "" {
		writeLicensePurchaseActivationFailurePage(w, http.StatusBadGateway, feature, selfHostedBillingPurchaseFailed, "The completed checkout did not return an activation key.")
		return
	}

	redemptionStore := h.purchaseReturnRedemptionStore()
	if redemptionStore == nil {
		writeLicensePurchaseActivationFailurePage(w, http.StatusServiceUnavailable, feature, selfHostedBillingPurchaseFailed, "Pulse could not verify the purchase activation state for this checkout.")
		return
	}
	expiresAt := time.Now().UTC().Add(2 * time.Hour)
	if claims != nil && claims.ExpiresAt != nil {
		expiresAt = claims.ExpiresAt.Time
	}
	redemptionDecision, _, err := redemptionStore.begin(purchaseReturnRedemptionAttempt{
		PortalHandoffID:     portalHandoffID,
		PurchaseReturnJTI:   expectedPurchaseReturnJTI,
		CheckoutSessionID:   sessionID,
		LicenseID:           strings.TrimSpace(checkoutResult.LicenseID),
		ActivationKeyPrefix: strings.TrimSpace(checkoutResult.ActivationKeyPrefix),
		ExpiresAt:           expiresAt,
	})
	if err != nil {
		log.Error().Err(err).Str("portal_handoff_id", portalHandoffID).Msg("Purchase activation redemption-store failure")
		writeLicensePurchaseActivationFailurePage(w, http.StatusServiceUnavailable, feature, selfHostedBillingPurchaseFailed, "Pulse could not verify the purchase activation state for this checkout.")
		return
	}
	switch redemptionDecision {
	case purchaseReturnRedemptionDecisionAlreadyActivated:
		writeLicensePurchaseActivationFailurePage(w, http.StatusConflict, feature, selfHostedBillingPurchaseActivated, "This completed purchase was already returned to Pulse Pro. Reopen billing if you need to confirm the current entitlement.")
		return
	case purchaseReturnRedemptionDecisionInProgress:
		writeLicensePurchaseActivationFailurePage(w, http.StatusConflict, feature, selfHostedBillingPurchaseFailed, "Pulse is already finalizing this completed purchase. Reopen billing in a moment if this tab does not close automatically.")
		return
	case purchaseReturnRedemptionDecisionConflict:
		log.Warn().
			Str("checkout_session_id", sessionID).
			Str("portal_handoff_id", portalHandoffID).
			Msg("Rejected checkout activation due to local redemption binding conflict")
		writeLicensePurchaseActivationFailurePage(w, http.StatusConflict, feature, selfHostedBillingPurchaseExpired, "Purchase activation state did not match the completed upgrade flow. Reopen the upgrade flow from Pulse Pro billing.")
		return
	}
	recordFailure := func(reason string, activationErr error) {
		message := ""
		if activationErr != nil {
			message = activationErr.Error()
		}
		if markErr := redemptionStore.markFailed(portalHandoffID, expectedPurchaseReturnJTI, reason, message); markErr != nil {
			log.Warn().
				Err(markErr).
				Str("portal_handoff_id", portalHandoffID).
				Str("purchase_return_jti", expectedPurchaseReturnJTI).
				Str("reason", reason).
				Msg("Failed to update purchase activation redemption record")
			return
		}
		if activationErr != nil {
			log.Warn().
				Err(activationErr).
				Str("portal_handoff_id", portalHandoffID).
				Str("purchase_return_jti", expectedPurchaseReturnJTI).
				Str("reason", reason).
				Msg("Marked purchase activation redemption as failed")
		}
	}

	if _, err := h.activateLicenseKey(ctx, activationKey); err != nil {
		recordFailure("license_activation_failed", err)
		log.Warn().Err(err).Str("checkout_session_id", sessionID).Msg("Failed to activate completed checkout locally")
		writeLicensePurchaseActivationFailurePage(w, http.StatusBadRequest, feature, selfHostedBillingPurchaseFailed, userFriendlyActivationError(err))
		return
	}
	if err := redemptionStore.markActivated(portalHandoffID, expectedPurchaseReturnJTI, strings.TrimSpace(checkoutResult.LicenseID), strings.TrimSpace(checkoutResult.ActivationKeyPrefix)); err != nil {
		log.Error().Err(err).Str("portal_handoff_id", portalHandoffID).Str("checkout_session_id", sessionID).Msg("Failed to finalize purchase activation redemption record")
		writeLicensePurchaseActivationFailurePage(w, http.StatusServiceUnavailable, feature, selfHostedBillingPurchaseFailed, "Pulse activated the license but could not finalize the local purchase record. Reopen billing to confirm the current entitlement.")
		return
	}

	writeLicensePurchaseActivationSuccessPage(w, feature)
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
	h.syncReleaseDemoFixtureRuntime(GetOrgID(r.Context()), service)

	orgID := GetOrgID(r.Context())
	if orgID == "" {
		orgID = "default"
	}

	// Clear from persistence (both legacy JWT and activation state).
	if persistence != nil {
		if err := persistence.Delete(); err != nil {
			log.Warn().Err(err).Msg("Failed to delete persisted license")
		}
		if err := persistence.ClearActivationState(); err != nil {
			log.Warn().Err(err).Msg("Failed to delete persisted activation state")
		}
	}

	// Clear any locally cached billing-backed entitlement grant as well.
	// Preserve trial_started_at and free-tier bookkeeping so the effective trial
	// ends immediately but trial reuse remains blocked.
	if h != nil && h.mtPersistence != nil {
		h.stopLegacyGrandfatherReconcileLoop(orgID)
		h.stopHostedEntitlementRefreshLoop(orgID)
		billingStore := config.NewFileBillingStore(h.mtPersistence.BaseDataDir())
		existing, err := billingStore.GetBillingState(orgID)
		if err != nil {
			log.Warn().Err(err).Str("org_id", orgID).Msg("Failed to load billing state while clearing license")
		} else if existing != nil {
			existing.Capabilities = []string{}
			existing.Limits = map[string]int64{}
			existing.MetersEnabled = []string{}
			existing.EntitlementJWT = ""
			existing.EntitlementRefreshToken = ""
			existing.CommercialMigration = nil
			existing.PlanVersion = string(subscriptionStateExpiredValue)
			existing.SubscriptionState = subscriptionStateExpiredValue
			existing.TrialEndsAt = nil
			existing.TrialExtendedAt = nil

			if err := billingStore.SaveBillingState(orgID, existing); err != nil {
				log.Warn().Err(err).Str("org_id", orgID).Msg("Failed to clear billing-backed entitlement state")
			}
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
