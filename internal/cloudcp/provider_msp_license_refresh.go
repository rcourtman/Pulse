package cloudcp

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/portal"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
	"github.com/rs/zerolog/log"
)

// A provider-hosted control plane buys and renews its own licence. The
// licence server knows it only by the Ed25519 lease signing key setup.sh
// generated on the host, so every call that must come from this platform is
// signed with that key, and the licence the server returns binds the same
// key. See pulse-pro license-server/provider_msp_purchase.go for the server
// half.

const (
	providerMSPChallengePrefix            = "pulse-provider-msp-v1"
	providerMSPLicenseRefreshInterval     = 6 * time.Hour
	providerMSPLicenseRefreshInitialDelay = 2 * time.Minute
	providerMSPLicenseRestartDelay        = 2 * time.Second
	providerMSPPlansCacheTTL              = 5 * time.Minute
	providerMSPLicenseHTTPTimeout         = 20 * time.Second
	maxProviderMSPLicenseResponseBytes    = 256 * 1024
)

// ErrProviderMSPAlreadySubscribed is returned when checkout is refused because
// this platform already pays; the billing portal is the way to change plan.
var ErrProviderMSPAlreadySubscribed = errors.New("provider MSP platform already has a paid subscription")

// ErrProviderMSPNoSubscription is returned when the licence server has no paid
// subscription bound to this platform's key.
var ErrProviderMSPNoSubscription = errors.New("no paid provider MSP subscription")

// ProviderMSPPurchasablePlan is one plan the licence server sells now.
type ProviderMSPPurchasablePlan struct {
	PlanVersion    string `json:"plan_version"`
	BillingCycle   string `json:"billing_cycle"`
	UnitAmount     int64  `json:"unit_amount"`
	Currency       string `json:"currency"`
	WorkspaceLimit int    `json:"workspace_limit"`
}

// ProviderMSPLicenseRefreshResult reports one refresh.
type ProviderMSPLicenseRefreshResult struct {
	Status           string `json:"status"` // "active" or "no_paid_subscription"
	Changed          bool   `json:"changed"`
	RestartScheduled bool   `json:"restart_scheduled"`
	LicenseID        string `json:"license_id,omitempty"`
	PlanVersion      string `json:"plan_version,omitempty"`
	ExpiresAt        string `json:"expires_at,omitempty"`
	PaidThrough      string `json:"paid_through,omitempty"`
}

// ProviderMSPLicenseRefresher fetches, validates and installs the licence for
// this platform's paid subscription, and relays checkout and billing-portal
// requests to the licence server.
type ProviderMSPLicenseRefresher struct {
	cfg    *CPConfig
	client *http.Client
	now    func() time.Time

	mu      sync.Mutex // serialises refreshes
	restart func()

	plansMu       sync.Mutex
	plans         []ProviderMSPPurchasablePlan
	plansCachedAt time.Time
}

// NewProviderMSPLicenseRefresher returns nil unless this control plane is a
// provider-hosted MSP platform that can sign requests to a licence server.
func NewProviderMSPLicenseRefresher(cfg *CPConfig) *ProviderMSPLicenseRefresher {
	if cfg == nil || !cfg.IsProviderHostedMSP() {
		return nil
	}
	if strings.TrimSpace(cfg.TrialActivationPrivateKey) == "" || strings.TrimSpace(cfg.LicenseServerURL) == "" {
		return nil
	}
	return &ProviderMSPLicenseRefresher{
		cfg:    cfg,
		client: &http.Client{Timeout: providerMSPLicenseHTTPTimeout},
		now:    time.Now,
	}
}

// SetRestart installs the hook that restarts the control plane so a changed
// licence takes effect. The plan version is read at load by every workspace
// limit, backup and status path, so a restart is the one way to apply it
// everywhere at once; client workspaces keep running meanwhile.
func (r *ProviderMSPLicenseRefresher) SetRestart(restart func()) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.restart = restart
	r.mu.Unlock()
}

type providerMSPSignedRequest struct {
	EntitlementSigningPublicKey string `json:"entitlement_signing_public_key"`
	Timestamp                   int64  `json:"timestamp"`
	Signature                   string `json:"signature"`
	ReturnURL                   string `json:"return_url,omitempty"`
}

func (r *ProviderMSPLicenseRefresher) signedRequest(action, returnURL string) (providerMSPSignedRequest, error) {
	privateKey, err := pkglicensing.DecodeEd25519PrivateKey(strings.TrimSpace(r.cfg.TrialActivationPrivateKey))
	if err != nil {
		return providerMSPSignedRequest{}, fmt.Errorf("decode lease signing key: %w", err)
	}
	publicKey, ok := privateKey.Public().(ed25519.PublicKey)
	if !ok {
		return providerMSPSignedRequest{}, fmt.Errorf("derive lease signing public key")
	}
	encoded := base64.StdEncoding.EncodeToString(publicKey)
	timestamp := r.now().Unix()
	message := providerMSPChallengePrefix + "\n" + action + "\n" + encoded + "\n" + strconv.FormatInt(timestamp, 10)
	return providerMSPSignedRequest{
		EntitlementSigningPublicKey: encoded,
		Timestamp:                   timestamp,
		Signature:                   base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, []byte(message))),
		ReturnURL:                   returnURL,
	}, nil
}

func (r *ProviderMSPLicenseRefresher) call(ctx context.Context, method, path string, body any, out any) (int, error) {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return 0, err
		}
		reader = bytes.NewReader(payload)
	}
	endpoint := strings.TrimRight(strings.TrimSpace(r.cfg.LicenseServerURL), "/") + path
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return 0, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("licence server unreachable: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxProviderMSPLicenseResponseBytes))
	if err != nil {
		return resp.StatusCode, err
	}
	if out != nil && len(raw) > 0 && resp.StatusCode < 500 {
		if err := json.Unmarshal(raw, out); err != nil && resp.StatusCode == http.StatusOK {
			return resp.StatusCode, fmt.Errorf("decode licence server response: %w", err)
		}
	}
	return resp.StatusCode, nil
}

// Refresh asks the licence server for the licence its paid subscription
// entitles, installs it when it differs from the running one, and schedules
// a restart so it takes effect. A platform without a paid subscription keeps
// the licence it already has.
func (r *ProviderMSPLicenseRefresher) Refresh(ctx context.Context) (ProviderMSPLicenseRefreshResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	request, err := r.signedRequest("license-current", "")
	if err != nil {
		return ProviderMSPLicenseRefreshResult{}, err
	}
	var resp struct {
		Status      string `json:"status"`
		LicenseID   string `json:"license_id"`
		License     string `json:"license"`
		PlanVersion string `json:"plan_version"`
		PaidThrough string `json:"paid_through"`
		ExpiresAt   string `json:"expires_at"`
	}
	status, err := r.call(ctx, http.MethodPost, "/v1/provider-msp/license/current", request, &resp)
	if err != nil {
		return ProviderMSPLicenseRefreshResult{}, err
	}
	switch status {
	case http.StatusOK:
	case http.StatusNotFound:
		return ProviderMSPLicenseRefreshResult{Status: "no_paid_subscription"}, nil
	default:
		return ProviderMSPLicenseRefreshResult{}, fmt.Errorf("licence server refused the refresh (HTTP %d)", status)
	}

	renewedPath := ProviderMSPRenewedLicensePath(r.cfg.DataDir)
	if err := os.MkdirAll(filepath.Dir(renewedPath), 0o700); err != nil {
		return ProviderMSPLicenseRefreshResult{}, err
	}
	staged, err := os.CreateTemp(filepath.Dir(renewedPath), ".provider-msp-license-*.jwt")
	if err != nil {
		return ProviderMSPLicenseRefreshResult{}, err
	}
	stagedPath := staged.Name()
	defer os.Remove(stagedPath)
	if _, err := staged.WriteString(strings.TrimSpace(resp.License) + "\n"); err != nil {
		staged.Close()
		return ProviderMSPLicenseRefreshResult{}, err
	}
	if err := staged.Chmod(0o600); err != nil {
		staged.Close()
		return ProviderMSPLicenseRefreshResult{}, err
	}
	if err := staged.Close(); err != nil {
		return ProviderMSPLicenseRefreshResult{}, err
	}

	// The same validation a startup applies: Pulse signature, MSP tier, a
	// plan with a known workspace limit. Then the key must be this
	// platform's, or validate would refuse to start on it.
	renewed, err := resolveProviderMSPPlanFromLicenseFile(stagedPath)
	if err != nil {
		return ProviderMSPLicenseRefreshResult{}, fmt.Errorf("licence server returned an unusable licence: %w", err)
	}
	if strings.TrimSpace(r.cfg.TrialActivationPublicKey) == "" || renewed.LeaseSigningPublicKey != r.cfg.TrialActivationPublicKey {
		return ProviderMSPLicenseRefreshResult{}, fmt.Errorf("licence server returned a licence bound to another platform's key")
	}

	result := ProviderMSPLicenseRefreshResult{
		Status:      "active",
		LicenseID:   renewed.LicenseID,
		PlanVersion: renewed.PlanVersion,
		PaidThrough: resp.PaidThrough,
	}
	if !renewed.ExpiresAt.IsZero() {
		result.ExpiresAt = renewed.ExpiresAt.Format(time.RFC3339)
	}
	result.Changed = renewed.LicenseID != r.cfg.ProviderMSPLicenseID ||
		renewed.PlanVersion != r.cfg.ProviderMSPPlanVersion ||
		!renewed.ExpiresAt.Equal(r.cfg.ProviderMSPLicenseExpiresAt)
	if !result.Changed {
		return result, nil
	}
	if err := os.Rename(stagedPath, renewedPath); err != nil {
		return ProviderMSPLicenseRefreshResult{}, err
	}
	log.Info().
		Str("license_id", renewed.LicenseID).
		Str("plan_version", renewed.PlanVersion).
		Str("expires_at", result.ExpiresAt).
		Msg("Installed renewed provider MSP license")
	if r.restart != nil {
		restart := r.restart
		result.RestartScheduled = true
		time.AfterFunc(providerMSPLicenseRestartDelay, func() {
			log.Info().Msg("Restarting control plane to apply the renewed provider MSP license")
			restart()
		})
	}
	return result, nil
}

// Run refreshes shortly after start and then periodically. Failures are
// logged and retried on the next tick; the running licence stays in force.
func (r *ProviderMSPLicenseRefresher) Run(ctx context.Context) {
	if r == nil {
		return
	}
	timer := time.NewTimer(providerMSPLicenseRefreshInitialDelay)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			if result, err := r.Refresh(ctx); err != nil {
				log.Warn().Err(err).Msg("Provider MSP license refresh failed; keeping the current license")
			} else if result.Changed {
				log.Info().Str("plan_version", result.PlanVersion).Msg("Provider MSP license refresh installed a new license")
			}
			timer.Reset(providerMSPLicenseRefreshInterval)
		}
	}
}

// Plans returns what the licence server sells now, with each plan's client
// workspace cap, cached briefly.
func (r *ProviderMSPLicenseRefresher) Plans(ctx context.Context) ([]ProviderMSPPurchasablePlan, error) {
	r.plansMu.Lock()
	defer r.plansMu.Unlock()
	if r.plans != nil && r.now().Sub(r.plansCachedAt) < providerMSPPlansCacheTTL {
		return r.plans, nil
	}
	var resp struct {
		Plans []ProviderMSPPurchasablePlan `json:"plans"`
	}
	status, err := r.call(ctx, http.MethodGet, "/v1/provider-msp/plans", nil, &resp)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("licence server plans unavailable (HTTP %d)", status)
	}
	plans := make([]ProviderMSPPurchasablePlan, 0, len(resp.Plans))
	for _, plan := range resp.Plans {
		limit, known := pkglicensing.WorkspaceLimitForPlan(plan.PlanVersion)
		if !known {
			// A plan this control plane cannot enforce is not offered.
			continue
		}
		plan.WorkspaceLimit = limit
		plans = append(plans, plan)
	}
	r.plans = plans
	r.plansCachedAt = r.now()
	return plans, nil
}

// Checkout starts a Stripe checkout for this platform and returns its URL.
func (r *ProviderMSPLicenseRefresher) Checkout(ctx context.Context, planVersion, billingCycle string) (string, error) {
	var resp struct {
		URL     string `json:"url"`
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	status, err := r.call(ctx, http.MethodPost, "/v1/provider-msp/checkout", map[string]string{
		"license_id":    r.cfg.ProviderMSPLicenseID,
		"plan_version":  planVersion,
		"billing_cycle": billingCycle,
		"return_url":    buildCPURL(r.cfg.BaseURL, portal.PortalPagePath, nil),
	}, &resp)
	if err != nil {
		return "", err
	}
	switch {
	case status == http.StatusOK && strings.HasPrefix(resp.URL, "https://"):
		return resp.URL, nil
	case status == http.StatusConflict:
		return "", ErrProviderMSPAlreadySubscribed
	default:
		return "", fmt.Errorf("licence server refused checkout (HTTP %d)", status)
	}
}

// BillingPortal opens the Stripe billing portal for this platform's
// subscription and returns its URL.
func (r *ProviderMSPLicenseRefresher) BillingPortal(ctx context.Context) (string, error) {
	// Come back flagged, so the portal applies a plan changed there instead
	// of showing the old one until the next scheduled refresh.
	returnURL := buildCPURL(r.cfg.BaseURL, portal.PortalPagePath, url.Values{"provider_msp_checkout": {"billing"}})
	request, err := r.signedRequest("billing-portal", returnURL)
	if err != nil {
		return "", err
	}
	var resp struct {
		URL string `json:"url"`
	}
	status, err := r.call(ctx, http.MethodPost, "/v1/provider-msp/billing-portal", request, &resp)
	if err != nil {
		return "", err
	}
	switch {
	case status == http.StatusOK && strings.HasPrefix(resp.URL, "https://"):
		return resp.URL, nil
	case status == http.StatusNotFound:
		return "", ErrProviderMSPNoSubscription
	default:
		return "", fmt.Errorf("licence server refused the billing portal (HTTP %d)", status)
	}
}

// ProviderMSPPlanState is what the portal shows about this platform's plan.
type ProviderMSPPlanState struct {
	PlanVersion       string                       `json:"plan_version"`
	PlanSource        string                       `json:"plan_source"`
	Evaluation        bool                         `json:"evaluation"`
	LicenseID         string                       `json:"license_id,omitempty"`
	ExpiresAt         string                       `json:"expires_at,omitempty"`
	WorkspaceLimit    int                          `json:"workspace_limit"`
	PurchaseAvailable bool                         `json:"purchase_available"`
	Plans             []ProviderMSPPurchasablePlan `json:"plans"`
	PlansError        string                       `json:"plans_error,omitempty"`
}

func providerMSPPlanState(ctx context.Context, cfg *CPConfig, refresher *ProviderMSPLicenseRefresher) ProviderMSPPlanState {
	plan := providerMSPPlanVersion(cfg)
	limit, _ := pkglicensing.WorkspaceLimitForPlan(plan)
	state := ProviderMSPPlanState{
		PlanVersion:    plan,
		PlanSource:     providerMSPPlanSourceOrDefault(cfg.ProviderMSPPlanSource),
		Evaluation:     plan == pkglicensing.PlanVersionMSPEval,
		LicenseID:      strings.TrimSpace(cfg.ProviderMSPLicenseID),
		WorkspaceLimit: limit,
		Plans:          []ProviderMSPPurchasablePlan{},
	}
	if !cfg.ProviderMSPLicenseExpiresAt.IsZero() {
		state.ExpiresAt = cfg.ProviderMSPLicenseExpiresAt.Format(time.RFC3339)
	}
	if refresher == nil || state.LicenseID == "" {
		return state
	}
	plans, err := refresher.Plans(ctx)
	if err != nil {
		state.PlansError = "Plans are unavailable right now. Try again in a few minutes."
		return state
	}
	state.Plans = plans
	state.PurchaseAvailable = len(plans) > 0
	return state
}

func writeProviderMSPJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// HandleProviderMSPPlan serves GET .../provider-msp/plan.
func HandleProviderMSPPlan(cfg *CPConfig, refresher *ProviderMSPLicenseRefresher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeProviderMSPJSON(w, http.StatusOK, providerMSPPlanState(r.Context(), cfg, refresher))
	}
}

// HandleProviderMSPCheckout serves POST .../provider-msp/checkout.
func HandleProviderMSPCheckout(refresher *ProviderMSPLicenseRefresher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if refresher == nil {
			writeProviderMSPJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "purchase_unavailable"})
			return
		}
		var req struct {
			PlanVersion  string `json:"plan_version"`
			BillingCycle string `json:"billing_cycle"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&req); err != nil {
			writeProviderMSPJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request"})
			return
		}
		checkoutURL, err := refresher.Checkout(r.Context(), strings.TrimSpace(req.PlanVersion), strings.TrimSpace(req.BillingCycle))
		switch {
		case errors.Is(err, ErrProviderMSPAlreadySubscribed):
			writeProviderMSPJSON(w, http.StatusConflict, map[string]string{
				"error":   "already_subscribed",
				"message": "This platform already has a paid plan. Use Manage billing to change it.",
			})
		case err != nil:
			log.Warn().Err(err).Msg("Provider MSP checkout failed")
			writeProviderMSPJSON(w, http.StatusBadGateway, map[string]string{
				"error":   "checkout_unavailable",
				"message": "Checkout is unavailable right now. Try again in a few minutes.",
			})
		default:
			writeProviderMSPJSON(w, http.StatusOK, map[string]string{"url": checkoutURL})
		}
	}
}

// HandleProviderMSPBillingPortal serves POST .../provider-msp/billing-portal.
func HandleProviderMSPBillingPortal(refresher *ProviderMSPLicenseRefresher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if refresher == nil {
			writeProviderMSPJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "billing_unavailable"})
			return
		}
		portalURL, err := refresher.BillingPortal(r.Context())
		switch {
		case errors.Is(err, ErrProviderMSPNoSubscription):
			writeProviderMSPJSON(w, http.StatusNotFound, map[string]string{
				"error":   "no_paid_subscription",
				"message": "This platform has no paid plan yet.",
			})
		case err != nil:
			log.Warn().Err(err).Msg("Provider MSP billing portal failed")
			writeProviderMSPJSON(w, http.StatusBadGateway, map[string]string{
				"error":   "billing_unavailable",
				"message": "Billing is unavailable right now. Try again in a few minutes.",
			})
		default:
			writeProviderMSPJSON(w, http.StatusOK, map[string]string{"url": portalURL})
		}
	}
}

// HandleProviderMSPLicenseRefresh serves POST .../provider-msp/license/refresh,
// the "I've just paid" path that applies a purchase without waiting for the
// periodic refresh.
func HandleProviderMSPLicenseRefresh(refresher *ProviderMSPLicenseRefresher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if refresher == nil {
			writeProviderMSPJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "refresh_unavailable"})
			return
		}
		result, err := refresher.Refresh(r.Context())
		if err != nil {
			log.Warn().Err(err).Msg("Provider MSP license refresh failed")
			writeProviderMSPJSON(w, http.StatusBadGateway, map[string]string{
				"error":   "refresh_unavailable",
				"message": "The licence could not be refreshed right now. Try again in a few minutes.",
			})
			return
		}
		writeProviderMSPJSON(w, http.StatusOK, result)
	}
}
