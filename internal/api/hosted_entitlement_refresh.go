package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rs/zerolog/log"
)

const (
	hostedEntitlementRefreshDefaultInterval = 2 * time.Hour
	hostedEntitlementRefreshMinInterval     = 15 * time.Minute
	hostedEntitlementRefreshMaxInterval     = 6 * time.Hour
	hostedEntitlementRefreshImmediateWindow = 30 * time.Minute
	hostedEntitlementRefreshBackoffMin      = 30 * time.Second
	hostedEntitlementRefreshBackoffMax      = 30 * time.Minute
	hostedEntitlementRefreshJitter          = 0.2
)

type hostedEntitlementRefreshLoop struct {
	mu      sync.Mutex
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	running bool
}

type hostedEntitlementRefreshError struct {
	statusCode int
	message    string
	permanent  bool
}

func (e *hostedEntitlementRefreshError) Error() string {
	if e == nil {
		return ""
	}
	if strings.TrimSpace(e.message) != "" {
		return e.message
	}
	return fmt.Sprintf("hosted entitlement refresh failed with status %d", e.statusCode)
}

func (h *LicenseHandlers) ensureHostedEntitlementRefreshForOrg(orgID string, service *licenseService) {
	if h == nil || h.mtPersistence == nil {
		return
	}
	orgID = strings.TrimSpace(orgID)
	if orgID == "" {
		orgID = "default"
	}
	if service != nil && service.Current() != nil {
		h.stopHostedEntitlementRefreshLoop(orgID)
		return
	}

	billingStore := config.NewFileBillingStore(h.mtPersistence.BaseDataDir())
	state, err := billingStore.GetBillingState(orgID)
	if err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("Failed to load billing state for hosted entitlement refresh")
		return
	}
	if !hasHostedEntitlementRefreshToken(state) {
		h.stopHostedEntitlementRefreshLoop(orgID)
		return
	}

	loop := h.hostedEntitlementRefreshLoop(orgID)
	if !loop.isRunning() && hostedEntitlementNeedsImmediateRefresh(state) {
		if _, permanent, err := h.refreshHostedEntitlementLeaseOnce(orgID, service); err != nil {
			if permanent {
				log.Warn().Err(err).Str("org_id", orgID).Msg("Permanent hosted entitlement refresh failure during initialization")
				h.stopHostedEntitlementRefreshLoop(orgID)
				return
			}
			log.Warn().Err(err).Str("org_id", orgID).Msg("Hosted entitlement refresh initialization failed")
		}
	}

	h.startHostedEntitlementRefreshLoop(orgID)
}

func hasHostedEntitlementRefreshToken(state *billingState) bool {
	return state != nil && strings.TrimSpace(state.EntitlementRefreshToken) != ""
}

func hostedEntitlementNeedsImmediateRefresh(state *billingState) bool {
	if state == nil || strings.TrimSpace(state.EntitlementRefreshToken) == "" {
		return false
	}
	leaseClaims, err := hostedEntitlementLeaseClaimsFromState(state)
	if err != nil || leaseClaims == nil || leaseClaims.ExpiresAt == nil {
		return true
	}
	return time.Until(leaseClaims.ExpiresAt.Time) <= hostedEntitlementRefreshImmediateWindow
}

func hostedEntitlementLeaseClaimsFromState(state *billingState) (*entitlementLeaseClaimsModel, error) {
	if state == nil || strings.TrimSpace(state.EntitlementJWT) == "" {
		return nil, fmt.Errorf("entitlement lease token is required")
	}
	publicKey, err := trialActivationPublicKeyFromLicensing()
	if err != nil {
		return nil, err
	}
	return parseEntitlementLeaseTokenFromLicensing(state.EntitlementJWT, publicKey, "")
}

func (h *LicenseHandlers) hostedEntitlementInstanceHost(state *billingState) string {
	if host := entitlementExpectedInstanceHost(h.cfg); host != "" {
		return host
	}
	claims, err := hostedEntitlementLeaseClaimsFromState(state)
	if err != nil || claims == nil {
		return ""
	}
	return normalizeHostForTrial(claims.InstanceHost)
}

func (h *LicenseHandlers) hostedEntitlementRefreshLoop(orgID string) *hostedEntitlementRefreshLoop {
	if h == nil {
		return nil
	}
	if loop, ok := h.hostedLeaseRefresh.Load(orgID); ok {
		if typed, ok := loop.(*hostedEntitlementRefreshLoop); ok {
			return typed
		}
	}
	loop := &hostedEntitlementRefreshLoop{}
	actual, _ := h.hostedLeaseRefresh.LoadOrStore(orgID, loop)
	if typed, ok := actual.(*hostedEntitlementRefreshLoop); ok {
		return typed
	}
	return loop
}

func (l *hostedEntitlementRefreshLoop) isRunning() bool {
	if l == nil {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.running
}

func (h *LicenseHandlers) startHostedEntitlementRefreshLoop(orgID string) {
	loop := h.hostedEntitlementRefreshLoop(orgID)
	if loop == nil {
		return
	}

	loop.mu.Lock()
	defer loop.mu.Unlock()
	if loop.running {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	loop.cancel = cancel
	loop.running = true
	loop.wg.Add(1)
	go func() {
		defer func() {
			loop.mu.Lock()
			loop.running = false
			loop.mu.Unlock()
			loop.wg.Done()
		}()
		h.runHostedEntitlementRefreshLoop(ctx, orgID)
	}()
}

func (h *LicenseHandlers) stopHostedEntitlementRefreshLoop(orgID string) {
	if h == nil {
		return
	}
	value, ok := h.hostedLeaseRefresh.Load(orgID)
	if !ok {
		return
	}
	loop, ok := value.(*hostedEntitlementRefreshLoop)
	if !ok || loop == nil {
		h.hostedLeaseRefresh.Delete(orgID)
		return
	}

	loop.mu.Lock()
	if !loop.running {
		loop.mu.Unlock()
		h.hostedLeaseRefresh.Delete(orgID)
		return
	}
	loop.cancel()
	loop.running = false
	loop.mu.Unlock()
	loop.wg.Wait()
	h.hostedLeaseRefresh.Delete(orgID)
}

func (h *LicenseHandlers) runHostedEntitlementRefreshLoop(ctx context.Context, orgID string) {
	consecutiveFailures := 0
	for {
		interval, ok := h.nextHostedEntitlementRefreshInterval(orgID, consecutiveFailures)
		if !ok {
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}

		refreshed, permanent, err := h.refreshHostedEntitlementLeaseOnce(orgID, nil)
		if err != nil {
			if permanent {
				log.Warn().Err(err).Str("org_id", orgID).Msg("Stopping hosted entitlement refresh loop after permanent failure")
				return
			}
			consecutiveFailures++
			log.Warn().
				Err(err).
				Str("org_id", orgID).
				Int("consecutive_failures", consecutiveFailures).
				Dur("next_retry", hostedEntitlementRefreshBackoff(consecutiveFailures)).
				Msg("Hosted entitlement refresh failed")
			continue
		}
		if !refreshed {
			return
		}
		consecutiveFailures = 0
	}
}

func (h *LicenseHandlers) nextHostedEntitlementRefreshInterval(orgID string, consecutiveFailures int) (time.Duration, bool) {
	if consecutiveFailures > 0 {
		return hostedEntitlementRefreshBackoff(consecutiveFailures), true
	}
	if h == nil || h.mtPersistence == nil {
		return 0, false
	}
	billingStore := config.NewFileBillingStore(h.mtPersistence.BaseDataDir())
	state, err := billingStore.GetBillingState(orgID)
	if err != nil || !hasHostedEntitlementRefreshToken(state) {
		return 0, false
	}

	claims, err := hostedEntitlementLeaseClaimsFromState(state)
	if err != nil || claims == nil || claims.ExpiresAt == nil {
		return time.Minute, true
	}

	remaining := time.Until(claims.ExpiresAt.Time)
	if remaining <= hostedEntitlementRefreshImmediateWindow {
		return time.Minute, true
	}
	interval := remaining / 2
	if interval < hostedEntitlementRefreshMinInterval {
		interval = hostedEntitlementRefreshMinInterval
	}
	if interval > hostedEntitlementRefreshMaxInterval {
		interval = hostedEntitlementRefreshMaxInterval
	}
	return withHostedEntitlementRefreshJitter(interval), true
}

func hostedEntitlementRefreshBackoff(consecutiveFailures int) time.Duration {
	if consecutiveFailures <= 0 {
		return hostedEntitlementRefreshDefaultInterval
	}
	backoff := hostedEntitlementRefreshBackoffMin * (1 << min(consecutiveFailures-1, 10))
	if backoff > hostedEntitlementRefreshBackoffMax {
		backoff = hostedEntitlementRefreshBackoffMax
	}
	return backoff
}

func withHostedEntitlementRefreshJitter(interval time.Duration) time.Duration {
	jitterRange := float64(interval) * hostedEntitlementRefreshJitter
	offset := (rand.Float64()*2 - 1) * jitterRange
	return interval + time.Duration(offset)
}

func (h *LicenseHandlers) refreshHostedEntitlementLeaseOnce(orgID string, service *licenseService) (bool, bool, error) {
	if h == nil || h.mtPersistence == nil {
		return false, true, nil
	}
	if service == nil {
		if value, ok := h.services.Load(orgID); ok {
			if typed, ok := value.(*licenseService); ok {
				service = typed
			}
		}
	}
	if service != nil && service.Current() != nil {
		return false, true, nil
	}

	billingStore := config.NewFileBillingStore(h.mtPersistence.BaseDataDir())
	state, err := billingStore.GetBillingState(orgID)
	if err != nil {
		return false, false, fmt.Errorf("load billing state: %w", err)
	}
	if !hasHostedEntitlementRefreshToken(state) {
		return false, true, nil
	}

	instanceHost := h.hostedEntitlementInstanceHost(state)
	if instanceHost == "" {
		return false, true, fmt.Errorf("hosted entitlement instance host is unavailable")
	}

	response, err := h.requestHostedEntitlementLeaseRefresh(orgID, instanceHost, state.EntitlementRefreshToken)
	if err != nil {
		var refreshErr *hostedEntitlementRefreshError
		if errors.As(err, &refreshErr) && refreshErr != nil && refreshErr.permanent {
			if clearErr := h.clearHostedEntitlementState(orgID, billingStore); clearErr != nil {
				log.Warn().Err(clearErr).Str("org_id", orgID).Msg("Failed to clear hosted entitlement state after permanent refresh failure")
			}
			if service != nil && service.Current() == nil {
				service.SetEvaluator(nil)
				_ = h.ensureEvaluatorForOrg(orgID, service)
			}
			return false, true, err
		}
		return false, false, err
	}

	publicKey, err := trialActivationPublicKeyFromLicensing()
	if err != nil {
		return false, false, fmt.Errorf("load entitlement verification key: %w", err)
	}
	leaseClaims, err := verifyEntitlementLeaseTokenFromLicensing(response.EntitlementJWT, publicKey, instanceHost, time.Now().UTC())
	if err != nil {
		return false, false, fmt.Errorf("verify refreshed entitlement lease: %w", err)
	}
	if normalizeHostedEntitlementOrgID(leaseClaims.OrgID) != normalizeHostedEntitlementOrgID(orgID) {
		return false, false, fmt.Errorf("refreshed entitlement lease org mismatch")
	}

	updated := normalizeBillingStateFromLicensing(state)
	updated.EntitlementJWT = strings.TrimSpace(response.EntitlementJWT)
	updated.Capabilities = []string{}
	updated.Limits = map[string]int64{}
	updated.MetersEnabled = []string{}
	updated.PlanVersion = ""
	updated.SubscriptionState = ""
	updated.TrialStartedAt = leaseClaims.TrialStartedAt
	updated.TrialEndsAt = nil
	updated.TrialExtendedAt = nil
	if err := billingStore.SaveBillingState(orgID, updated); err != nil {
		return false, false, fmt.Errorf("save refreshed entitlement lease: %w", err)
	}
	if service != nil && service.Current() == nil {
		service.SetEvaluator(newLicenseEvaluatorForBillingStoreFromLicensing(billingStore, orgID, 0, instanceHost))
	}
	return true, false, nil
}

func (h *LicenseHandlers) clearHostedEntitlementState(orgID string, billingStore *config.FileBillingStore) error {
	if billingStore == nil {
		return nil
	}
	existing, err := billingStore.GetBillingState(orgID)
	if err != nil {
		return err
	}
	if existing == nil {
		return nil
	}
	existing.Capabilities = []string{}
	existing.Limits = map[string]int64{}
	existing.MetersEnabled = []string{}
	existing.EntitlementJWT = ""
	existing.EntitlementRefreshToken = ""
	existing.PlanVersion = string(subscriptionStateExpiredValue)
	existing.SubscriptionState = subscriptionStateExpiredValue
	existing.TrialEndsAt = nil
	existing.TrialExtendedAt = nil
	return billingStore.SaveBillingState(orgID, existing)
}

func (h *LicenseHandlers) requestHostedEntitlementLeaseRefresh(orgID, instanceHost, refreshToken string) (*hostedTrialLeaseRefreshResponse, error) {
	if h == nil {
		return nil, &hostedEntitlementRefreshError{permanent: true, message: "license handlers unavailable"}
	}
	refreshURL := hostedEntitlementRefreshURLFromConfig(h.cfg)
	if refreshURL == "" {
		return nil, &hostedEntitlementRefreshError{permanent: true, message: "hosted entitlement refresh URL unavailable"}
	}

	payload, err := json.Marshal(hostedTrialLeaseRefreshRequest{
		OrgID:                   normalizeHostedEntitlementOrgID(orgID),
		InstanceHost:            strings.TrimSpace(instanceHost),
		EntitlementRefreshToken: strings.TrimSpace(refreshToken),
	})
	if err != nil {
		return nil, fmt.Errorf("marshal hosted entitlement refresh request: %w", err)
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, refreshURL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build hosted entitlement refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("post hosted entitlement refresh request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		permanent := resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusGone || resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusNotFound
		return nil, &hostedEntitlementRefreshError{
			statusCode: resp.StatusCode,
			message:    fmt.Sprintf("hosted entitlement refresh returned status %d", resp.StatusCode),
			permanent:  permanent,
		}
	}

	var response hostedTrialLeaseRefreshResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("decode hosted entitlement refresh response: %w", err)
	}
	if strings.TrimSpace(response.EntitlementJWT) == "" {
		return nil, fmt.Errorf("hosted entitlement refresh response missing entitlement_jwt")
	}
	return &response, nil
}

func normalizeHostedEntitlementOrgID(raw string) string {
	orgID := strings.TrimSpace(raw)
	if orgID == "" {
		return "default"
	}
	return orgID
}
