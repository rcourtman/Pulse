package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/license"
	"github.com/rcourtman/pulse-go-rewrite/internal/license/entitlements"
	"github.com/rs/zerolog/log"
	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/webhook"
)

const stripeWebhookBodyLimit = 1024 * 1024 // 1MiB

var errStripeWebhookEventInFlight = errors.New("stripe webhook event is in-flight")

// StripeWebhookHandlers handles Stripe webhooks for hosted Cloud provisioning.
//
// SECURITY: Signature verification (ConstructEvent) is the authentication mechanism for this endpoint.
type StripeWebhookHandlers struct {
	hostedMode bool

	persistence  *config.MultiTenantPersistence
	rbacProvider HostedRBACProvider
	magicLinks   *MagicLinkService
	publicURL    func(*http.Request) string

	billingStore *config.FileBillingStore

	deduper *stripeWebhookDeduper
	index   *stripeCustomerOrgIndex

	now func() time.Time
}

func NewStripeWebhookHandlers(
	billingStore *config.FileBillingStore,
	persistence *config.MultiTenantPersistence,
	rbacProvider HostedRBACProvider,
	magicLinks *MagicLinkService,
	publicURL func(*http.Request) string,
	hostedMode bool,
	dataPath string,
) *StripeWebhookHandlers {
	baseDir := resolvePulseDataDir(dataPath)
	return &StripeWebhookHandlers{
		hostedMode:   hostedMode,
		persistence:  persistence,
		rbacProvider: rbacProvider,
		magicLinks:   magicLinks,
		publicURL:    publicURL,
		billingStore: billingStore,
		deduper:      newStripeWebhookDeduper(filepath.Join(baseDir, "stripe", "webhook-events")),
		index:        newStripeCustomerOrgIndex(filepath.Join(baseDir, "stripe", "customers")),
		now:          time.Now,
	}
}

func (h *StripeWebhookHandlers) HandleStripeWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.hostedMode {
		http.NotFound(w, r)
		return
	}
	if h.billingStore == nil || h.persistence == nil || h.rbacProvider == nil || h.deduper == nil || h.index == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "stripe_unavailable", "Stripe webhook handler is not configured", nil)
		return
	}

	secret := strings.TrimSpace(os.Getenv("STRIPE_WEBHOOK_SECRET"))
	if secret == "" {
		writeErrorResponse(w, http.StatusServiceUnavailable, "stripe_unavailable", "Stripe webhook secret is not configured", nil)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, stripeWebhookBodyLimit)
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_request", "Failed to read request body", nil)
		return
	}

	sigHeader := r.Header.Get("Stripe-Signature")
	if strings.TrimSpace(sigHeader) == "" {
		// Intentionally vague; missing signature is treated as invalid auth.
		writeErrorResponse(w, http.StatusBadRequest, "invalid_signature", "Invalid Stripe signature", nil)
		return
	}

	event, err := webhook.ConstructEventWithOptions(payload, sigHeader, secret, webhook.ConstructEventOptions{
		IgnoreAPIVersionMismatch: true,
	})
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_signature", "Invalid Stripe signature", nil)
		return
	}

	already, err := h.deduper.Do(event.ID, func() error {
		return h.handleEvent(r.Context(), &event, r)
	})
	if err != nil {
		if errors.Is(err, errStripeWebhookEventInFlight) {
			log.Warn().
				Str("event_id", event.ID).
				Str("type", string(event.Type)).
				Msg("Stripe webhook event is already in-flight; returning non-2xx so Stripe retries")
			writeErrorResponse(w, http.StatusConflict, "stripe_in_flight", "Stripe webhook is being processed; retry later", nil)
			return
		}
		log.Error().Err(err).Str("event_id", event.ID).Str("type", string(event.Type)).Msg("Stripe webhook processing failed")
		writeErrorResponse(w, http.StatusInternalServerError, "stripe_processing_failed", "Failed to process Stripe webhook", nil)
		return
	}

	if already {
		// Stripe treats any 2xx as success; returning JSON helps local debugging.
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"received": true,
			"status":   "duplicate",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"received": true,
		"status":   "processed",
	})
}

func (h *StripeWebhookHandlers) handleEvent(ctx context.Context, event *stripe.Event, r *http.Request) error {
	if event == nil {
		return errors.New("stripe event is nil")
	}

	switch event.Type {
	case "checkout.session.completed":
		var session stripeCheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &session); err != nil {
			return fmt.Errorf("decode checkout.session: %w", err)
		}
		return h.handleCheckoutSessionCompleted(ctx, session, r)
	case "customer.subscription.updated":
		var sub stripeSubscription
		if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
			return fmt.Errorf("decode subscription: %w", err)
		}
		return h.handleSubscriptionUpdated(ctx, sub)
	case "customer.subscription.deleted":
		var sub stripeSubscription
		if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
			return fmt.Errorf("decode subscription: %w", err)
		}
		return h.handleSubscriptionDeleted(ctx, sub)
	default:
		log.Info().Str("type", string(event.Type)).Str("event_id", event.ID).Msg("Stripe webhook ignored (unhandled type)")
		return nil
	}
}

type stripeCheckoutSession struct {
	ID               string            `json:"id"`
	Mode             string            `json:"mode"`
	Customer         string            `json:"customer"`
	Subscription     string            `json:"subscription"`
	CustomerEmail    string            `json:"customer_email"`
	CustomerDetails  stripeCustDetails `json:"customer_details"`
	ClientReference  string            `json:"client_reference_id"`
	Metadata         map[string]string `json:"metadata"`
	SubscriptionData map[string]any    `json:"subscription_data"`
}

type stripeCustDetails struct {
	Email string `json:"email"`
}

type stripeSubscription struct {
	ID                 string `json:"id"`
	Customer           string `json:"customer"`
	Status             string `json:"status"`
	CancelAtPeriodEnd  bool   `json:"cancel_at_period_end"`
	CurrentPeriodEnd   int64  `json:"current_period_end"`
	EndedAt            int64  `json:"ended_at"`
	CancellationReason string `json:"cancellation_reason"`
	Items              struct {
		Data []struct {
			Price struct {
				ID       string            `json:"id"`
				Product  string            `json:"product"`
				Metadata map[string]string `json:"metadata"`
			} `json:"price"`
		} `json:"data"`
	} `json:"items"`
	Metadata map[string]string `json:"metadata"`
}

func (h *StripeWebhookHandlers) handleCheckoutSessionCompleted(ctx context.Context, session stripeCheckoutSession, r *http.Request) error {
	// Expect subscription-mode sessions for Cloud.
	if strings.TrimSpace(session.Customer) == "" {
		return fmt.Errorf("checkout session missing customer")
	}

	// SECURITY: customer email is not a safe org identifier.
	// If present, it's used only for best-effort post-checkout UX (magic link) and audit logs.
	email := strings.ToLower(strings.TrimSpace(session.CustomerEmail))
	if email == "" {
		email = strings.ToLower(strings.TrimSpace(session.CustomerDetails.Email))
	}

	orgName := ""
	if session.Metadata != nil {
		orgName = strings.TrimSpace(session.Metadata["org_name"])
		if orgName == "" {
			orgName = strings.TrimSpace(session.Metadata["org"])
		}
	}
	// Prefer existing mapping by customer ID; otherwise require server-owned linkage (metadata/client_reference_id).
	orgID, ok, err := h.index.LookupOrgID(session.Customer)
	if err != nil {
		return fmt.Errorf("lookup org by customer id: %w", err)
	}
	orgResolvedBy := "customer_index"
	if !ok {
		orgID = ""
		if session.Metadata != nil {
			orgID = strings.TrimSpace(session.Metadata["org_id"])
		}
		if orgID == "" {
			orgID = strings.TrimSpace(session.ClientReference)
		}
		orgResolvedBy = "session_linkage"
	}
	orgID = strings.TrimSpace(orgID)
	if orgID == "" {
		log.Warn().
			Str("session_id", strings.TrimSpace(session.ID)).
			Str("customer_id", strings.TrimSpace(session.Customer)).
			Str("subscription_id", strings.TrimSpace(session.Subscription)).
			Msg("Stripe checkout.session.completed: missing org linkage (refusing to provision)")
		return nil
	}
	if !isValidOrganizationID(orgID) {
		log.Warn().
			Str("session_id", strings.TrimSpace(session.ID)).
			Str("customer_id", strings.TrimSpace(session.Customer)).
			Str("org_id", orgID).
			Str("resolved_by", orgResolvedBy).
			Msg("Stripe checkout.session.completed: invalid org id linkage (refusing to provision)")
		return nil
	}

	// SECURITY: only provision into an org that already exists. Do not create tenants from webhook payloads.
	org, err := h.persistence.LoadOrganizationStrict(orgID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Warn().
				Str("session_id", strings.TrimSpace(session.ID)).
				Str("customer_id", strings.TrimSpace(session.Customer)).
				Str("org_id", orgID).
				Str("resolved_by", orgResolvedBy).
				Msg("Stripe checkout.session.completed: org not found for linkage (refusing to provision)")
			return nil
		}
		return fmt.Errorf("load org: %w", err)
	}
	if org == nil {
		return fmt.Errorf("load org: empty org")
	}

	// Persist customer->org mapping once the linkage has been validated.
	if !ok {
		if err := h.index.Save(session.Customer, orgID); err != nil {
			return fmt.Errorf("save customer index: %w", err)
		}
	}

	planVersion := derivePlanVersion(session.Metadata, "")

	state := &entitlements.BillingState{
		Capabilities:         license.DeriveCapabilitiesFromTier(license.TierCloud, nil),
		Limits:               map[string]int64{},
		MetersEnabled:        []string{},
		PlanVersion:          planVersion,
		SubscriptionState:    entitlements.SubStateActive,
		StripeCustomerID:     session.Customer,
		StripeSubscriptionID: strings.TrimSpace(session.Subscription),
	}

	if err := h.billingStore.SaveBillingState(orgID, state); err != nil {
		return fmt.Errorf("save billing state: %w", err)
	}

	// Best-effort: issue a magic link so the user can sign in quickly after checkout.
	// (In dev/staging this is log-only; production should swap in a real emailer.)
	if h.magicLinks != nil && email != "" {
		// Only send a link to an existing org member/owner. Stripe customer email is user-controlled.
		sendTo := ""
		if strings.EqualFold(org.OwnerUserID, email) {
			sendTo = org.OwnerUserID
		} else {
			for _, m := range org.Members {
				if strings.EqualFold(m.UserID, email) {
					sendTo = m.UserID
					break
				}
			}
		}
		if sendTo != "" && h.magicLinks.AllowRequest(sendTo) {
			token, genErr := h.magicLinks.GenerateToken(sendTo, orgID)
			if genErr == nil {
				baseURL := ""
				if h.publicURL != nil && r != nil {
					baseURL = h.publicURL(r)
				}
				if baseURL != "" {
					if sendErr := h.magicLinks.SendMagicLink(sendTo, orgID, token, baseURL); sendErr != nil {
						log.Warn().Err(sendErr).Str("email", sendTo).Str("org_id", orgID).Msg("Stripe checkout: failed to send magic link")
					}
				}
			}
		}
	}

	log.Info().
		Str("org_id", orgID).
		Str("email", email).
		Str("org_name", orgName).
		Str("customer_id", session.Customer).
		Str("resolved_by", orgResolvedBy).
		Msg("Stripe checkout.session.completed processed")

	return nil
}

func (h *StripeWebhookHandlers) handleSubscriptionUpdated(ctx context.Context, sub stripeSubscription) error {
	customerID := strings.TrimSpace(sub.Customer)
	if customerID == "" {
		return fmt.Errorf("subscription missing customer")
	}

	orgID, ok, err := h.index.LookupOrgID(customerID)
	if err != nil {
		return fmt.Errorf("lookup org by customer id: %w", err)
	}
	if !ok {
		// Backstop for older data: scan org billing files.
		orgID, ok, err = h.scanOrgByStripeCustomerID(customerID)
		if err != nil {
			return fmt.Errorf("scan org by customer id: %w", err)
		}
		if ok {
			_ = h.index.Save(customerID, orgID)
		}
	}
	if !ok {
		log.Warn().Str("customer_id", customerID).Str("subscription_id", sub.ID).Msg("Stripe subscription.updated: org not found for customer")
		return nil
	}

	before, err := h.billingStore.GetBillingState(orgID)
	if err != nil {
		return fmt.Errorf("load billing state: %w", err)
	}
	state := normalizeBillingState(before)

	subState := mapStripeSubscriptionStatusToState(sub.Status)
	state.SubscriptionState = subState

	priceID := firstPriceID(sub)
	state.StripePriceID = priceID
	state.StripeCustomerID = customerID
	state.StripeSubscriptionID = strings.TrimSpace(sub.ID)
	state.PlanVersion = derivePlanVersion(sub.Metadata, priceID)

	if shouldGrantPaidCapabilities(subState) {
		state.Capabilities = license.DeriveCapabilitiesFromTier(license.TierCloud, nil)
	} else {
		state.Capabilities = []string{}
	}

	if err := h.billingStore.SaveBillingState(orgID, state); err != nil {
		return fmt.Errorf("save billing state: %w", err)
	}

	log.Info().
		Str("org_id", orgID).
		Str("customer_id", customerID).
		Str("subscription_id", sub.ID).
		Str("subscription_state", string(subState)).
		Msg("Stripe customer.subscription.updated processed")

	return nil
}

func (h *StripeWebhookHandlers) handleSubscriptionDeleted(ctx context.Context, sub stripeSubscription) error {
	customerID := strings.TrimSpace(sub.Customer)
	if customerID == "" {
		return fmt.Errorf("subscription missing customer")
	}

	orgID, ok, err := h.index.LookupOrgID(customerID)
	if err != nil {
		return fmt.Errorf("lookup org by customer id: %w", err)
	}
	if !ok {
		orgID, ok, err = h.scanOrgByStripeCustomerID(customerID)
		if err != nil {
			return fmt.Errorf("scan org by customer id: %w", err)
		}
		if ok {
			_ = h.index.Save(customerID, orgID)
		}
	}
	if !ok {
		log.Warn().Str("customer_id", customerID).Str("subscription_id", sub.ID).Msg("Stripe subscription.deleted: org not found for customer")
		return nil
	}

	before, err := h.billingStore.GetBillingState(orgID)
	if err != nil {
		return fmt.Errorf("load billing state: %w", err)
	}
	state := normalizeBillingState(before)

	// CRITICAL: revoke paid capabilities immediately on cancellation.
	state.SubscriptionState = entitlements.SubStateCanceled
	state.Capabilities = []string{}
	state.StripeCustomerID = customerID
	state.StripeSubscriptionID = strings.TrimSpace(sub.ID)

	if err := h.billingStore.SaveBillingState(orgID, state); err != nil {
		return fmt.Errorf("save billing state: %w", err)
	}

	log.Info().
		Str("org_id", orgID).
		Str("customer_id", customerID).
		Str("subscription_id", sub.ID).
		Msg("Stripe customer.subscription.deleted processed (capabilities revoked)")

	return nil
}

func (h *StripeWebhookHandlers) scanOrgByStripeCustomerID(customerID string) (string, bool, error) {
	orgs, err := h.persistence.ListOrganizations()
	if err != nil {
		return "", false, err
	}
	customerID = strings.TrimSpace(customerID)
	if customerID == "" {
		return "", false, nil
	}

	for _, org := range orgs {
		if org == nil || strings.TrimSpace(org.ID) == "" {
			continue
		}
		state, loadErr := h.billingStore.GetBillingState(org.ID)
		if loadErr != nil || state == nil {
			continue
		}
		if strings.TrimSpace(state.StripeCustomerID) == customerID {
			return org.ID, true, nil
		}
	}

	return "", false, nil
}

func firstPriceID(sub stripeSubscription) string {
	for _, item := range sub.Items.Data {
		if strings.TrimSpace(item.Price.ID) != "" {
			return strings.TrimSpace(item.Price.ID)
		}
	}
	return ""
}

func mapStripeSubscriptionStatusToState(status string) entitlements.SubscriptionState {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "active":
		return entitlements.SubStateActive
	case "trialing":
		return entitlements.SubStateTrial
	case "past_due", "unpaid":
		return entitlements.SubStateGrace
	case "canceled":
		return entitlements.SubStateCanceled
	case "paused":
		return entitlements.SubStateSuspended
	case "incomplete", "incomplete_expired":
		return entitlements.SubStateExpired
	default:
		// Fail closed: unknown status should not grant paid capabilities.
		return entitlements.SubStateExpired
	}
}

func shouldGrantPaidCapabilities(state entitlements.SubscriptionState) bool {
	switch state {
	case entitlements.SubStateActive, entitlements.SubStateTrial, entitlements.SubStateGrace:
		return true
	default:
		return false
	}
}

func derivePlanVersion(metadata map[string]string, priceID string) string {
	if metadata != nil {
		if v := strings.TrimSpace(metadata["plan_version"]); v != "" {
			return v
		}
		if v := strings.TrimSpace(metadata["plan"]); v != "" {
			return v
		}
	}
	if strings.TrimSpace(priceID) != "" {
		return "stripe_price:" + strings.TrimSpace(priceID)
	}
	return "stripe"
}

func resolvePulseDataDir(dataPath string) string {
	if dir := strings.TrimSpace(dataPath); dir != "" {
		return dir
	}
	if dir := strings.TrimSpace(os.Getenv("PULSE_DATA_DIR")); dir != "" {
		return dir
	}
	return "/etc/pulse"
}

// stripeWebhookDeduper provides durable idempotency for Stripe webhook event IDs.
// Stripe retries webhooks; without a persistent dedupe store, retries can provision duplicate tenants.
type stripeWebhookDeduper struct {
	dir      string
	lockTTL  time.Duration
	now      func() time.Time
	hashSalt []byte
}

func newStripeWebhookDeduper(dir string) *stripeWebhookDeduper {
	return &stripeWebhookDeduper{
		dir:     dir,
		lockTTL: 10 * time.Minute,
		now:     time.Now,
		// Salt prevents event IDs from being used directly as filenames if they contain odd characters.
		// (Event IDs are normally safe, but this keeps the filesystem contract tight.)
		hashSalt: []byte("pulse-stripe-webhook-v1"),
	}
}

func (d *stripeWebhookDeduper) Do(eventID string, fn func() error) (already bool, err error) {
	if d == nil {
		return false, errors.New("deduper is nil")
	}
	if strings.TrimSpace(eventID) == "" {
		return false, errors.New("event id is required")
	}
	if fn == nil {
		return false, errors.New("handler is required")
	}

	donePath := d.donePath(eventID)
	if fileExists(donePath) {
		return true, nil
	}

	lockPath := d.lockPath(eventID)
	acquired, lockErr := d.acquireLock(lockPath)
	if lockErr != nil {
		return false, lockErr
	}
	if !acquired {
		// Another in-flight processor exists. If processing has already completed, treat as a duplicate.
		// If the lock exists but the done file does not, we must return a non-2xx so Stripe retries later.
		// (Otherwise a concurrent Stripe retry could stop retrying while the original attempt still fails.)
		if fileExists(donePath) {
			return true, nil
		}
		return false, errStripeWebhookEventInFlight
	}

	defer func() {
		_ = os.Remove(lockPath)
	}()

	if err := fn(); err != nil {
		return false, err
	}

	if err := os.MkdirAll(filepath.Dir(donePath), 0o700); err != nil {
		return false, fmt.Errorf("create dedupe dir: %w", err)
	}

	meta := map[string]any{
		"handled_at": d.now().UTC().UnixMilli(),
	}
	data, _ := json.Marshal(meta)
	tmp := donePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return false, fmt.Errorf("write dedupe tmp: %w", err)
	}
	if err := os.Rename(tmp, donePath); err != nil {
		_ = os.Remove(tmp)
		return false, fmt.Errorf("commit dedupe: %w", err)
	}

	return false, nil
}

func (d *stripeWebhookDeduper) acquireLock(lockPath string) (bool, error) {
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o700); err != nil {
		return false, fmt.Errorf("create lock dir: %w", err)
	}

	f, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err == nil {
		_ = f.Close()
		return true, nil
	}

	if !errors.Is(err, os.ErrExist) {
		return false, fmt.Errorf("create lock: %w", err)
	}

	// Break stale locks (e.g., process crash) so Stripe retries can succeed.
	info, statErr := os.Stat(lockPath)
	if statErr == nil && d.now().Sub(info.ModTime()) > d.lockTTL {
		if rmErr := os.Remove(lockPath); rmErr == nil {
			f, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
			if err == nil {
				_ = f.Close()
				return true, nil
			}
			if errors.Is(err, os.ErrExist) {
				return false, nil
			}
			return false, fmt.Errorf("recreate lock: %w", err)
		}
	}

	return false, nil
}

func (d *stripeWebhookDeduper) donePath(eventID string) string {
	return filepath.Join(d.dir, d.filenameForID(eventID)+".done")
}

func (d *stripeWebhookDeduper) lockPath(eventID string) string {
	return filepath.Join(d.dir, d.filenameForID(eventID)+".lock")
}

func (d *stripeWebhookDeduper) filenameForID(id string) string {
	// Use a deterministic HMAC so we never trust arbitrary IDs as filesystem paths.
	mac := hmac.New(sha256.New, d.hashSalt)
	_, _ = mac.Write([]byte(id))
	return hex.EncodeToString(mac.Sum(nil))
}

type stripeCustomerOrgIndex struct {
	dir string
}

func newStripeCustomerOrgIndex(dir string) *stripeCustomerOrgIndex {
	return &stripeCustomerOrgIndex{dir: dir}
}

func (i *stripeCustomerOrgIndex) LookupOrgID(customerID string) (string, bool, error) {
	if i == nil {
		return "", false, errors.New("index is nil")
	}
	customerID = strings.TrimSpace(customerID)
	if customerID == "" {
		return "", false, nil
	}
	if !isSafeStripeID(customerID) {
		return "", false, fmt.Errorf("invalid stripe customer id")
	}

	path := filepath.Join(i.dir, customerID+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", false, nil
		}
		return "", false, err
	}
	var rec struct {
		OrgID string `json:"org_id"`
	}
	if err := json.Unmarshal(data, &rec); err != nil {
		return "", false, err
	}
	orgID := strings.TrimSpace(rec.OrgID)
	if orgID == "" {
		return "", false, nil
	}
	return orgID, true, nil
}

func (i *stripeCustomerOrgIndex) Save(customerID, orgID string) error {
	if i == nil {
		return errors.New("index is nil")
	}
	customerID = strings.TrimSpace(customerID)
	orgID = strings.TrimSpace(orgID)
	if customerID == "" || orgID == "" {
		return fmt.Errorf("customerID and orgID are required")
	}
	if !isSafeStripeID(customerID) {
		return fmt.Errorf("invalid stripe customer id")
	}
	if !isValidOrganizationID(orgID) {
		return fmt.Errorf("invalid org id")
	}

	if err := os.MkdirAll(i.dir, 0o700); err != nil {
		return err
	}

	path := filepath.Join(i.dir, customerID+".json")
	data, _ := json.Marshal(map[string]any{
		"org_id":      orgID,
		"updated_at":  time.Now().UTC().UnixMilli(),
		"customer_id": customerID,
	})
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func isSafeStripeID(id string) bool {
	// Stripe IDs are typically like "cus_...", "sub_...", "evt_...".
	// Keep this strict to avoid filesystem surprises.
	if len(id) < 5 || len(id) > 128 {
		return false
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-' {
			continue
		}
		return false
	}
	if filepath.Base(id) != id {
		return false
	}
	return true
}

// fileExists is defined in router.go (same package). Keep a single implementation
// to avoid duplicate symbol errors across the internal/api package.
