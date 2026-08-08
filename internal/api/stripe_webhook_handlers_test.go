package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/license"
	"github.com/rcourtman/pulse-go-rewrite/internal/license/entitlements"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/stripe/stripe-go/v82/webhook"
)

type captureEmailer struct {
	mu    sync.Mutex
	calls []struct {
		to  string
		url string
	}
}

func (e *captureEmailer) SendMagicLink(to, magicLinkURL string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.calls = append(e.calls, struct {
		to  string
		url string
	}{to: to, url: magicLinkURL})
	return nil
}

func (e *captureEmailer) Count() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.calls)
}

func (e *captureEmailer) LastTo() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.calls) == 0 {
		return ""
	}
	return e.calls[len(e.calls)-1].to
}

func createTestOrg(t *testing.T, persistence *config.MultiTenantPersistence, orgID, ownerEmail string) {
	t.Helper()

	if _, err := persistence.GetPersistence(orgID); err != nil {
		t.Fatalf("GetPersistence(%s): %v", orgID, err)
	}

	now := time.Now().UTC()
	ownerUserID := "u_" + orgID + "_owner"
	org := &models.Organization{
		ID:          orgID,
		DisplayName: orgID,
		CreatedAt:   now,
		OwnerUserID: ownerUserID,
		OwnerEmail:  ownerEmail,
		Members: []models.OrganizationMember{
			{
				UserID:  ownerUserID,
				Email:   ownerEmail,
				Role:    models.OrgRoleOwner,
				AddedAt: now,
				AddedBy: ownerUserID,
			},
		},
	}
	if err := persistence.SaveOrganization(org); err != nil {
		t.Fatalf("SaveOrganization(%s): %v", orgID, err)
	}
}

func TestStripeWebhook_SignatureVerification(t *testing.T) {
	t.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_test_123")

	tmp := t.TempDir()
	persistence := config.NewMultiTenantPersistence(tmp)
	rbacProvider := NewTenantRBACProvider(tmp)
	billingStore := config.NewFileBillingStore(tmp)

	emailer := &captureEmailer{}
	magicLinks := NewMagicLinkServiceWithKey([]byte("01234567890123456789012345678901"), nil, emailer, nil)
	t.Cleanup(magicLinks.Stop)

	publicURL := func(_ *http.Request) string { return "https://pulse.example.test" }
	h := NewStripeWebhookHandlers(billingStore, persistence, rbacProvider, magicLinks, publicURL, true, tmp)

	event := map[string]any{
		"id":   "evt_1",
		"type": "checkout.session.completed",
		"data": map[string]any{
			"object": map[string]any{
				"id":             "cs_1",
				"mode":           "subscription",
				"customer":       "cus_123",
				"customer_email": "user@example.com",
				"subscription":   "sub_123",
				"metadata": map[string]any{
					"org_name":     "Acme",
					"plan_version": "cloud-v1",
				},
			},
		},
	}
	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}

	t.Run("missing signature rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/webhooks/stripe", bytes.NewReader(payload))
		rr := httptest.NewRecorder()
		h.HandleStripeWebhook(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status=%d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("invalid signature rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/webhooks/stripe", bytes.NewReader(payload))
		signed := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{
			Payload:   payload,
			Secret:    "whsec_wrong",
			Timestamp: time.Now(),
			Scheme:    "v1",
		})
		req.Header.Set("Stripe-Signature", signed.Header)
		rr := httptest.NewRecorder()
		h.HandleStripeWebhook(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status=%d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("valid signature accepted", func(t *testing.T) {
		createTestOrg(t, persistence, "org_signature", "user@example.com")

		event := map[string]any{
			"id":   "evt_signature_valid",
			"type": "checkout.session.completed",
			"data": map[string]any{
				"object": map[string]any{
					"id":             "cs_signature_valid",
					"mode":           "subscription",
					"customer":       "cus_signature_valid",
					"customer_email": "user@example.com",
					"subscription":   "sub_signature_valid",
					"metadata": map[string]any{
						"org_id":       "org_signature",
						"org_name":     "Acme",
						"plan_version": "cloud-v1",
					},
				},
			},
		}
		payload, err := json.Marshal(event)
		if err != nil {
			t.Fatalf("marshal valid event: %v", err)
		}

		req := httptest.NewRequest(http.MethodPost, "/api/webhooks/stripe", bytes.NewReader(payload))
		req.Host = "app.example.test"
		signed := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{
			Payload:   payload,
			Secret:    "whsec_test_123",
			Timestamp: time.Now(),
			Scheme:    "v1",
		})
		req.Header.Set("Stripe-Signature", signed.Header)

		// Sanity-check the stripe-go verifier with the exact payload/header pair.
		if _, err := webhook.ConstructEventWithOptions(payload, signed.Header, "whsec_test_123", webhook.ConstructEventOptions{
			IgnoreAPIVersionMismatch: true,
		}); err != nil {
			t.Fatalf("ConstructEvent sanity-check failed: %v (header=%q)", err, signed.Header)
		}

		rr := httptest.NewRecorder()
		h.HandleStripeWebhook(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status=%d, want %d", rr.Code, http.StatusOK)
		}
	})
}

func TestStripeWebhook_CheckoutCompleted_IdempotentProvisioning(t *testing.T) {
	t.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_test_456")

	tmp := t.TempDir()
	persistence := config.NewMultiTenantPersistence(tmp)
	rbacProvider := NewTenantRBACProvider(tmp)
	billingStore := config.NewFileBillingStore(tmp)

	emailer := &captureEmailer{}
	magicLinks := NewMagicLinkServiceWithKey([]byte("01234567890123456789012345678901"), nil, emailer, nil)
	t.Cleanup(magicLinks.Stop)

	publicURL := func(_ *http.Request) string { return "https://pulse.example.test" }
	h := NewStripeWebhookHandlers(billingStore, persistence, rbacProvider, magicLinks, publicURL, true, tmp)

	orgID := "org_beta"
	createTestOrg(t, persistence, orgID, "user2@example.com")

	event := map[string]any{
		"id":   "evt_checkout_1",
		"type": "checkout.session.completed",
		"data": map[string]any{
			"object": map[string]any{
				"id":             "cs_1",
				"mode":           "subscription",
				"customer":       "cus_abc",
				"customer_email": "user2@example.com",
				"subscription":   "sub_abc",
				"metadata": map[string]any{
					"org_id":       orgID,
					"org_name":     "Beta Org",
					"plan_version": "cloud-v1",
				},
			},
		},
	}
	payload, _ := json.Marshal(event)
	signed := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{
		Payload:   payload,
		Secret:    "whsec_test_456",
		Timestamp: time.Now(),
		Scheme:    "v1",
	})
	sig := signed.Header

	post := func() int {
		req := httptest.NewRequest(http.MethodPost, "/api/webhooks/stripe", bytes.NewReader(payload))
		req.Host = "app.example.test"
		req.Header.Set("Stripe-Signature", sig)
		rr := httptest.NewRecorder()
		h.HandleStripeWebhook(rr, req)
		return rr.Code
	}

	if code := post(); code != http.StatusOK {
		t.Fatalf("first post status=%d, want %d", code, http.StatusOK)
	}
	if code := post(); code != http.StatusOK {
		t.Fatalf("second post status=%d, want %d", code, http.StatusOK)
	}

	state, err := billingStore.GetBillingState(orgID)
	if err != nil {
		t.Fatalf("GetBillingState: %v", err)
	}
	if state == nil {
		t.Fatalf("expected billing state")
	}
	if state.SubscriptionState != entitlements.SubStateActive {
		t.Fatalf("subscription_state=%q, want %q", state.SubscriptionState, entitlements.SubStateActive)
	}
	if state.PlanVersion != "cloud_starter" {
		t.Fatalf("plan_version=%q, want %q", state.PlanVersion, "cloud_starter")
	}
	if _, ok := state.Limits["max_monitored_systems"]; ok {
		t.Fatalf("expected retired max_monitored_systems limit to be omitted, got %+v", state.Limits)
	}
	if state.StripeCustomerID != "cus_abc" {
		t.Fatalf("stripe_customer_id=%q, want %q", state.StripeCustomerID, "cus_abc")
	}
	if !license.TierHasFeature(license.TierCloud, license.FeatureAIAutoFix) {
		t.Fatalf("sanity: cloud tier must include ai_autofix")
	}
	hasAutoFix := false
	for _, cap := range state.Capabilities {
		if cap == license.FeatureAIAutoFix {
			hasAutoFix = true
		}
	}
	if !hasAutoFix {
		t.Fatalf("expected cloud capabilities to include %q, got %v", license.FeatureAIAutoFix, state.Capabilities)
	}

	if emailer.Count() != 1 {
		t.Fatalf("magic link send count=%d, want %d (idempotency)", emailer.Count(), 1)
	}
}

func TestStripeWebhook_CheckoutCompleted_MissingOrgLinkageFailsClosed(t *testing.T) {
	t.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_test_missing_org")

	tmp := t.TempDir()
	persistence := config.NewMultiTenantPersistence(tmp)
	rbacProvider := NewTenantRBACProvider(tmp)
	billingStore := config.NewFileBillingStore(tmp)

	h := NewStripeWebhookHandlers(billingStore, persistence, rbacProvider, nil, nil, true, tmp)

	event := map[string]any{
		"id":   "evt_checkout_missing_org",
		"type": "checkout.session.completed",
		"data": map[string]any{
			"object": map[string]any{
				"id":             "cs_missing_org",
				"mode":           "subscription",
				"customer":       "cus_missing_org",
				"customer_email": "owner@example.com",
				"subscription":   "sub_missing_org",
				"metadata": map[string]any{
					"org_name":     "Missing Linkage Org",
					"plan_version": "cloud-v1",
				},
			},
		},
	}
	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	signed := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{
		Payload:   payload,
		Secret:    "whsec_test_missing_org",
		Timestamp: time.Now(),
		Scheme:    "v1",
	})

	post := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/webhooks/stripe", bytes.NewReader(payload))
		req.Header.Set("Stripe-Signature", signed.Header)
		rr := httptest.NewRecorder()
		h.HandleStripeWebhook(rr, req)
		return rr
	}

	first := post()
	if first.Code != http.StatusInternalServerError {
		t.Fatalf("first status=%d, want %d: %s", first.Code, http.StatusInternalServerError, first.Body.String())
	}

	second := post()
	if second.Code != http.StatusInternalServerError {
		t.Fatalf("second status=%d, want %d: %s", second.Code, http.StatusInternalServerError, second.Body.String())
	}

	orgID, ok, err := h.index.LookupOrgID("cus_missing_org")
	if err != nil {
		t.Fatalf("LookupOrgID: %v", err)
	}
	if ok || orgID != "" {
		t.Fatalf("unexpected customer index mapping org=%q ok=%v", orgID, ok)
	}
}

func TestStripeWebhook_CheckoutCompleted_RetriesUntilLinkedOrgExists(t *testing.T) {
	t.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_test_retry_org")

	tmp := t.TempDir()
	persistence := config.NewMultiTenantPersistence(tmp)
	rbacProvider := NewTenantRBACProvider(tmp)
	billingStore := config.NewFileBillingStore(tmp)

	emailer := &captureEmailer{}
	magicLinks := NewMagicLinkServiceWithKey([]byte("01234567890123456789012345678901"), nil, emailer, nil)
	t.Cleanup(magicLinks.Stop)

	publicURL := func(_ *http.Request) string { return "https://pulse.example.test" }
	h := NewStripeWebhookHandlers(billingStore, persistence, rbacProvider, magicLinks, publicURL, true, tmp)

	event := map[string]any{
		"id":   "evt_checkout_retry_org",
		"type": "checkout.session.completed",
		"data": map[string]any{
			"object": map[string]any{
				"id":             "cs_retry_org",
				"mode":           "subscription",
				"customer":       "cus_retry_org",
				"customer_email": "owner@example.com",
				"subscription":   "sub_retry_org",
				"metadata": map[string]any{
					"org_id":       "org_retry_org",
					"org_name":     "Retry Org",
					"plan_version": "cloud-v1",
				},
			},
		},
	}
	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	signed := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{
		Payload:   payload,
		Secret:    "whsec_test_retry_org",
		Timestamp: time.Now(),
		Scheme:    "v1",
	})

	post := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/webhooks/stripe", bytes.NewReader(payload))
		req.Header.Set("Stripe-Signature", signed.Header)
		rr := httptest.NewRecorder()
		h.HandleStripeWebhook(rr, req)
		return rr
	}

	first := post()
	if first.Code != http.StatusInternalServerError {
		t.Fatalf("first status=%d, want %d: %s", first.Code, http.StatusInternalServerError, first.Body.String())
	}

	createTestOrg(t, persistence, "org_retry_org", "owner@example.com")

	second := post()
	if second.Code != http.StatusOK {
		t.Fatalf("second status=%d, want %d: %s", second.Code, http.StatusOK, second.Body.String())
	}

	state, err := billingStore.GetBillingState("org_retry_org")
	if err != nil {
		t.Fatalf("GetBillingState: %v", err)
	}
	if state == nil {
		t.Fatalf("expected billing state")
	}
	if state.SubscriptionState != entitlements.SubStateActive {
		t.Fatalf("subscription_state=%q, want %q", state.SubscriptionState, entitlements.SubStateActive)
	}
	if state.StripeCustomerID != "cus_retry_org" {
		t.Fatalf("stripe_customer_id=%q, want %q", state.StripeCustomerID, "cus_retry_org")
	}

	mappedOrgID, ok, err := h.index.LookupOrgID("cus_retry_org")
	if err != nil {
		t.Fatalf("LookupOrgID: %v", err)
	}
	if !ok || mappedOrgID != "org_retry_org" {
		t.Fatalf("index mapping mismatch: org=%q ok=%v, want org=%q ok=true", mappedOrgID, ok, "org_retry_org")
	}

	if emailer.Count() != 1 {
		t.Fatalf("magic link send count=%d, want %d", emailer.Count(), 1)
	}
}

func TestStripeWebhook_CheckoutCompleted_UsesContactEmailForStableOwnerIdentity(t *testing.T) {
	t.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_test_stable_owner_contact")

	tmp := t.TempDir()
	persistence := config.NewMultiTenantPersistence(tmp)
	rbacProvider := NewTenantRBACProvider(tmp)
	billingStore := config.NewFileBillingStore(tmp)

	emailer := &captureEmailer{}
	magicLinks := NewMagicLinkServiceWithKey([]byte("01234567890123456789012345678901"), nil, emailer, nil)
	t.Cleanup(magicLinks.Stop)

	publicURL := func(_ *http.Request) string { return "https://pulse.example.test" }
	h := NewStripeWebhookHandlers(billingStore, persistence, rbacProvider, magicLinks, publicURL, true, tmp)

	orgID := "org_stable_owner_contact"
	if _, err := persistence.GetPersistence(orgID); err != nil {
		t.Fatalf("GetPersistence(%s): %v", orgID, err)
	}
	now := time.Now().UTC()
	if err := persistence.SaveOrganization(&models.Organization{
		ID:          orgID,
		DisplayName: "Stable Owner Contact",
		CreatedAt:   now,
		OwnerUserID: "u_owner_stable",
		OwnerEmail:  "owner@example.com",
		Members: []models.OrganizationMember{
			{
				UserID:  "u_owner_stable",
				Email:   "owner@example.com",
				Role:    models.OrgRoleOwner,
				AddedAt: now,
				AddedBy: "u_owner_stable",
			},
		},
	}); err != nil {
		t.Fatalf("SaveOrganization(%s): %v", orgID, err)
	}

	event := map[string]any{
		"id":   "evt_checkout_stable_owner_contact",
		"type": "checkout.session.completed",
		"data": map[string]any{
			"object": map[string]any{
				"id":             "cs_stable_owner_contact",
				"mode":           "subscription",
				"customer":       "cus_stable_owner_contact",
				"customer_email": "OWNER@example.com",
				"subscription":   "sub_stable_owner_contact",
				"metadata": map[string]any{
					"org_id":       orgID,
					"org_name":     "Stable Owner Contact",
					"plan_version": "cloud-v1",
				},
			},
		},
	}
	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	signed := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{
		Payload:   payload,
		Secret:    "whsec_test_stable_owner_contact",
		Timestamp: time.Now(),
		Scheme:    "v1",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/stripe", bytes.NewReader(payload))
	req.Header.Set("Stripe-Signature", signed.Header)
	rr := httptest.NewRecorder()
	h.HandleStripeWebhook(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if got := emailer.LastTo(); got != "owner@example.com" {
		t.Fatalf("magic link recipient=%q, want owner contact email", got)
	}
}

func TestStripeWebhook_CheckoutCompleted_DoesNotSendMagicLinkForBlankOrganizationPrincipal(t *testing.T) {
	t.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_test_blank_owner_contact")

	tmp := t.TempDir()
	persistence := config.NewMultiTenantPersistence(tmp)
	rbacProvider := NewTenantRBACProvider(tmp)
	billingStore := config.NewFileBillingStore(tmp)

	emailer := &captureEmailer{}
	magicLinks := NewMagicLinkServiceWithKey([]byte("01234567890123456789012345678901"), nil, emailer, nil)
	t.Cleanup(magicLinks.Stop)

	publicURL := func(_ *http.Request) string { return "https://pulse.example.test" }
	h := NewStripeWebhookHandlers(billingStore, persistence, rbacProvider, magicLinks, publicURL, true, tmp)

	orgID := "org_blank_owner_contact"
	if _, err := persistence.GetPersistence(orgID); err != nil {
		t.Fatalf("GetPersistence(%s): %v", orgID, err)
	}
	if err := persistence.SaveOrganization(&models.Organization{
		ID:          orgID,
		DisplayName: "Blank Owner Contact",
		CreatedAt:   time.Now().UTC(),
		OwnerEmail:  "owner@example.com",
	}); err != nil {
		t.Fatalf("SaveOrganization(%s): %v", orgID, err)
	}

	event := map[string]any{
		"id":   "evt_checkout_blank_owner_contact",
		"type": "checkout.session.completed",
		"data": map[string]any{
			"object": map[string]any{
				"id":             "cs_blank_owner_contact",
				"mode":           "subscription",
				"customer":       "cus_blank_owner_contact",
				"customer_email": "owner@example.com",
				"subscription":   "sub_blank_owner_contact",
				"metadata": map[string]any{
					"org_id":       orgID,
					"org_name":     "Blank Owner Contact",
					"plan_version": "cloud-v1",
				},
			},
		},
	}
	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	signed := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{
		Payload:   payload,
		Secret:    "whsec_test_blank_owner_contact",
		Timestamp: time.Now(),
		Scheme:    "v1",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/stripe", bytes.NewReader(payload))
	req.Header.Set("Stripe-Signature", signed.Header)
	rr := httptest.NewRecorder()
	h.HandleStripeWebhook(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if emailer.Count() != 0 {
		t.Fatalf("magic link send count=%d, want none for blank stored principal", emailer.Count())
	}

	state, err := billingStore.GetBillingState(orgID)
	if err != nil {
		t.Fatalf("GetBillingState: %v", err)
	}
	if state == nil || state.SubscriptionState != entitlements.SubStateActive {
		t.Fatalf("billing state=%v, want active checkout billing despite skipped magic link", state)
	}
}

func TestStripeWebhook_DoesNotSendMagicLinkWithoutPublicURL(t *testing.T) {
	t.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_test_no_url")

	tmp := t.TempDir()
	persistence := config.NewMultiTenantPersistence(tmp)
	rbacProvider := NewTenantRBACProvider(tmp)
	billingStore := config.NewFileBillingStore(tmp)

	emailer := &captureEmailer{}
	magicLinks := NewMagicLinkServiceWithKey([]byte("01234567890123456789012345678901"), nil, emailer, nil)
	t.Cleanup(magicLinks.Stop)

	// publicURL callback intentionally omitted to simulate missing canonical URL.
	h := NewStripeWebhookHandlers(billingStore, persistence, rbacProvider, magicLinks, nil, true, tmp)

	orgID := "org_no_url"
	createTestOrg(t, persistence, orgID, "no-url@example.com")

	event := map[string]any{
		"id":   "evt_checkout_no_url",
		"type": "checkout.session.completed",
		"data": map[string]any{
			"object": map[string]any{
				"id":             "cs_no_url",
				"mode":           "subscription",
				"customer":       "cus_no_url",
				"customer_email": "no-url@example.com",
				"subscription":   "sub_no_url",
				"metadata": map[string]any{
					"org_id":       orgID,
					"org_name":     "No URL Org",
					"plan_version": "cloud-v1",
				},
			},
		},
	}
	payload, _ := json.Marshal(event)
	signed := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{
		Payload:   payload,
		Secret:    "whsec_test_no_url",
		Timestamp: time.Now(),
		Scheme:    "v1",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/stripe", bytes.NewReader(payload))
	// Even if Host is set, hosted mode must not use it for magic links.
	req.Host = "attacker.example.test"
	req.Header.Set("Stripe-Signature", signed.Header)
	rr := httptest.NewRecorder()
	h.HandleStripeWebhook(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d", rr.Code, http.StatusOK)
	}
	if emailer.Count() != 0 {
		t.Fatalf("magic link send count=%d, want %d (public url missing must fail closed)", emailer.Count(), 0)
	}
}

func TestStripeWebhook_CheckoutMagicLinkValidatesOriginBeforeMutation(t *testing.T) {
	type originHeaders struct {
		host             string
		remoteAddr       string
		trustedProxyCIDR string
		forwardedHost    string
		forwardedProto   string
	}
	type storeFactory struct {
		name string
		new  func(*testing.T) (MagicLinkStore, func(*testing.T) int)
	}
	type testCase struct {
		name            string
		publicURL       string
		autoDetected    bool
		agentConnectURL string
		wantBaseURL     string
		headers         originHeaders
	}

	storeFactories := []storeFactory{
		{
			name: "in-memory",
			new: func(t *testing.T) (MagicLinkStore, func(*testing.T) int) {
				t.Helper()
				store := NewInMemoryMagicLinkStore()
				return store, func(t *testing.T) int {
					t.Helper()
					store.mu.RLock()
					defer store.mu.RUnlock()
					return len(store.tokens)
				}
			},
		},
		{
			name: "sqlite",
			new: func(t *testing.T) (MagicLinkStore, func(*testing.T) int) {
				t.Helper()
				store, err := NewSQLiteMagicLinkStore(t.TempDir())
				if err != nil {
					t.Fatalf("NewSQLiteMagicLinkStore: %v", err)
				}
				return store, func(t *testing.T) int {
					t.Helper()
					var count int
					if err := store.db.QueryRow(`SELECT COUNT(*) FROM magic_link_tokens`).Scan(&count); err != nil {
						t.Fatalf("count SQLite magic-link tokens: %v", err)
					}
					return count
				}
			},
		},
	}

	cases := []testCase{
		{
			name: "missing configuration ignores hostile direct Host and untrusted forwarding",
			headers: originHeaders{
				host:           "user@direct-attacker.example/path",
				remoteAddr:     "198.51.100.20:41000",
				forwardedHost:  "forwarded-attacker.example",
				forwardedProto: "https",
			},
		},
		{
			name:         "auto-detected URL is not hosted authority",
			publicURL:    "https://auto-detected.example",
			autoDetected: true,
			headers: originHeaders{
				host:           "direct-attacker.example",
				remoteAddr:     "198.51.100.20:41000",
				forwardedHost:  "forwarded-attacker.example",
				forwardedProto: "https",
			},
		},
		{
			name:      "invalid PublicURL ignores trusted forwarding",
			publicURL: "https://user@configured.example",
			headers: originHeaders{
				host:             "direct-attacker.example",
				remoteAddr:       "192.0.2.44:41000",
				trustedProxyCIDR: "192.0.2.0/24",
				forwardedHost:    "trusted-forwarded-attacker.example",
				forwardedProto:   "https",
			},
		},
		{
			name:            "invalid higher-precedence AgentConnectURL does not fall through",
			publicURL:       "https://configured.example",
			agentConnectURL: "https://user@agents.configured.example",
			headers: originHeaders{
				host:           "direct-attacker.example",
				remoteAddr:     "198.51.100.20:41000",
				forwardedHost:  "forwarded-attacker.example",
				forwardedProto: "https",
			},
		},
		{
			name:        "configured PublicURL defeats hostile direct Host and untrusted forwarding",
			publicURL:   "https://configured.example/base/",
			wantBaseURL: "https://configured.example/base",
			headers: originHeaders{
				host:           "direct-attacker.example",
				remoteAddr:     "198.51.100.20:41000",
				forwardedHost:  "forwarded-attacker.example",
				forwardedProto: "http",
			},
		},
		{
			name:            "configured AgentConnectURL outranks PublicURL and trusted forwarding",
			publicURL:       "https://configured.example",
			agentConnectURL: "https://agents.configured.example/",
			wantBaseURL:     "https://agents.configured.example",
			headers: originHeaders{
				host:             "direct-attacker.example",
				remoteAddr:       "192.0.2.44:41000",
				trustedProxyCIDR: "192.0.2.0/24",
				forwardedHost:    "trusted-forwarded-attacker.example",
				forwardedProto:   "http",
			},
		},
	}

	for _, factory := range storeFactories {
		factory := factory
		for _, tc := range cases {
			tc := tc
			t.Run(factory.name+" "+tc.name, func(t *testing.T) {
				t.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_magic_link_origin_order")
				t.Setenv("PULSE_TRUSTED_PROXY_CIDRS", tc.headers.trustedProxyCIDR)
				resetTrustedProxyCIDRsForTests()

				dataPath := t.TempDir()
				persistence := config.NewMultiTenantPersistence(dataPath)
				rbacProvider := NewTenantRBACProvider(dataPath)
				t.Cleanup(func() { _ = rbacProvider.Close() })
				billingStore := config.NewFileBillingStore(dataPath)
				createTestOrg(t, persistence, "org_magic_checkout", "owner@example.com")

				store, countTokens := factory.new(t)
				emailer := &captureEmailer{}
				magicLinks := NewMagicLinkServiceWithKey(
					[]byte("0123456789abcdef0123456789abcdef"),
					store,
					emailer,
					NewRateLimiter(100, time.Hour),
				)
				t.Cleanup(magicLinks.Stop)

				urlRouter := &Router{
					hostedMode: true,
					config: &config.Config{
						PublicURL:             tc.publicURL,
						PublicURLAutoDetected: tc.autoDetected,
						AgentConnectURL:       tc.agentConnectURL,
					},
				}
				handler := NewStripeWebhookHandlers(
					billingStore,
					persistence,
					rbacProvider,
					magicLinks,
					urlRouter.resolvePublicURL,
					true,
					dataPath,
				)

				event := map[string]any{
					"id":   "evt_checkout_magic_link_origin_order",
					"type": "checkout.session.completed",
					"data": map[string]any{
						"object": map[string]any{
							"id":             "cs_magic_link_origin_order",
							"mode":           "subscription",
							"customer":       "cus_magic_link_origin_order",
							"customer_email": "owner@example.com",
							"subscription":   "sub_magic_link_origin_order",
							"metadata": map[string]any{
								"org_id":       "org_magic_checkout",
								"org_name":     "Magic Checkout",
								"plan_version": "cloud-v1",
							},
						},
					},
				}
				payload, err := json.Marshal(event)
				if err != nil {
					t.Fatalf("marshal Stripe event: %v", err)
				}
				signed := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{
					Payload:   payload,
					Secret:    "whsec_magic_link_origin_order",
					Timestamp: time.Now(),
					Scheme:    "v1",
				})

				post := func(t *testing.T) *httptest.ResponseRecorder {
					t.Helper()
					req := httptest.NewRequest(http.MethodPost, "/api/webhooks/stripe", bytes.NewReader(payload))
					req.Host = tc.headers.host
					req.RemoteAddr = tc.headers.remoteAddr
					if tc.headers.forwardedHost != "" {
						req.Header.Set("X-Forwarded-Host", tc.headers.forwardedHost)
					}
					if tc.headers.forwardedProto != "" {
						req.Header.Set("X-Forwarded-Proto", tc.headers.forwardedProto)
					}
					req.Header.Set("Stripe-Signature", signed.Header)
					rec := httptest.NewRecorder()
					handler.HandleStripeWebhook(rec, req)
					return rec
				}

				first := post(t)
				if first.Code != http.StatusOK || !strings.Contains(first.Body.String(), `"status":"processed"`) {
					t.Fatalf("first checkout status = %d body=%s, want processed 200", first.Code, first.Body.String())
				}
				wantTokens := 0
				if tc.wantBaseURL != "" {
					wantTokens = 1
				}
				if got := countTokens(t); got != wantTokens {
					t.Fatalf("stored magic-link token count after processed event = %d, want exactly %d", got, wantTokens)
				}

				second := post(t)
				if second.Code != http.StatusOK || !strings.Contains(second.Body.String(), `"status":"duplicate"`) {
					t.Fatalf("duplicate checkout status = %d body=%s, want duplicate 200", second.Code, second.Body.String())
				}
				if got := countTokens(t); got != wantTokens {
					t.Fatalf("stored magic-link token count after duplicate event = %d, want exactly %d", got, wantTokens)
				}

				state, err := billingStore.GetBillingState("org_magic_checkout")
				if err != nil {
					t.Fatalf("GetBillingState: %v", err)
				}
				if state == nil || state.SubscriptionState != entitlements.SubStateActive || state.StripeCustomerID != "cus_magic_link_origin_order" {
					t.Fatalf("billing state = %#v, want active checkout projection", state)
				}
				mappedOrgID, ok, err := handler.index.LookupOrgID("cus_magic_link_origin_order")
				if err != nil || !ok || mappedOrgID != "org_magic_checkout" {
					t.Fatalf("customer index = org:%q ok:%t err:%v", mappedOrgID, ok, err)
				}

				if tc.wantBaseURL == "" {
					if emailer.Count() != 0 {
						t.Fatalf("magic-link sends = %d, want 0 when canonical URL is unavailable", emailer.Count())
					}
					return
				}
				if emailer.Count() != 1 {
					t.Fatalf("magic-link sends = %d, want exactly 1 across duplicate delivery", emailer.Count())
				}
				emailer.mu.Lock()
				magicLinkURL := emailer.calls[0].url
				emailer.mu.Unlock()
				if !strings.HasPrefix(magicLinkURL, tc.wantBaseURL+"/api/public/magic-link/verify?") || strings.Contains(magicLinkURL, "attacker") {
					t.Fatalf("magic-link URL was not bound to configured base %q: %q", tc.wantBaseURL, magicLinkURL)
				}
			})
		}
	}
}

func TestStripeWebhook_SubscriptionDeleted_RevokesCapabilities(t *testing.T) {
	tests := []struct {
		name        string
		customerID  string
		email       string
		orgID       string
		planVersion string
	}{
		{
			name:        "monthly grandfathered recurring plan",
			customerID:  "cus_del_monthly",
			email:       "user3@example.com",
			orgID:       "org_gamma_monthly",
			planVersion: "v5_pro_monthly_grandfathered",
		},
		{
			name:        "annual grandfathered recurring plan",
			customerID:  "cus_del_annual",
			email:       "user4@example.com",
			orgID:       "org_gamma_annual",
			planVersion: "v5_pro_annual_grandfathered",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_test_789")

			tmp := t.TempDir()
			persistence := config.NewMultiTenantPersistence(tmp)
			rbacProvider := NewTenantRBACProvider(tmp)
			billingStore := config.NewFileBillingStore(tmp)

			h := NewStripeWebhookHandlers(billingStore, persistence, rbacProvider, nil, nil, true, tmp)

			createTestOrg(t, persistence, tc.orgID, tc.email)

			// Provision via checkout first so cancellation is evaluated against an existing plan state.
			checkout := map[string]any{
				"id":   "evt_checkout_2",
				"type": "checkout.session.completed",
				"data": map[string]any{
					"object": map[string]any{
						"id":             "cs_2",
						"mode":           "subscription",
						"customer":       tc.customerID,
						"customer_email": tc.email,
						"subscription":   "sub_del",
						"metadata": map[string]any{
							"org_id":       tc.orgID,
							"org_name":     "Gamma Org",
							"plan_version": tc.planVersion,
						},
					},
				},
			}
			checkoutPayload, _ := json.Marshal(checkout)
			req := httptest.NewRequest(http.MethodPost, "/api/webhooks/stripe", bytes.NewReader(checkoutPayload))
			req.Header.Set("Stripe-Signature", webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{
				Payload:   checkoutPayload,
				Secret:    "whsec_test_789",
				Timestamp: time.Now(),
				Scheme:    "v1",
			}).Header)
			rr := httptest.NewRecorder()
			h.HandleStripeWebhook(rr, req)
			if rr.Code != http.StatusOK {
				t.Fatalf("checkout status=%d, want %d", rr.Code, http.StatusOK)
			}

			del := map[string]any{
				"id":   "evt_sub_deleted_1",
				"type": "customer.subscription.deleted",
				"data": map[string]any{
					"object": map[string]any{
						"id":       "sub_del",
						"customer": tc.customerID,
						"status":   "canceled",
					},
				},
			}
			delPayload, _ := json.Marshal(del)
			req2 := httptest.NewRequest(http.MethodPost, "/api/webhooks/stripe", bytes.NewReader(delPayload))
			req2.Header.Set("Stripe-Signature", webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{
				Payload:   delPayload,
				Secret:    "whsec_test_789",
				Timestamp: time.Now(),
				Scheme:    "v1",
			}).Header)
			rr2 := httptest.NewRecorder()
			h.HandleStripeWebhook(rr2, req2)
			if rr2.Code != http.StatusOK {
				t.Fatalf("delete status=%d, want %d", rr2.Code, http.StatusOK)
			}

			state, err := billingStore.GetBillingState(tc.orgID)
			if err != nil {
				t.Fatalf("GetBillingState: %v", err)
			}
			if state.SubscriptionState != entitlements.SubStateCanceled {
				t.Fatalf("subscription_state=%q, want %q", state.SubscriptionState, entitlements.SubStateCanceled)
			}
			if state.PlanVersion != tc.planVersion {
				t.Fatalf("plan_version=%q, want %q", state.PlanVersion, tc.planVersion)
			}
			if len(state.Capabilities) != 0 {
				t.Fatalf("capabilities=%v, want empty", state.Capabilities)
			}
			if len(state.Limits) != 0 {
				t.Fatalf("limits=%v, want empty", state.Limits)
			}
		})
	}
}

func TestStripeWebhook_CheckoutCompleted_EmailCollisionDoesNotCrossProvision(t *testing.T) {
	t.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_test_collision")

	tmp := t.TempDir()
	persistence := config.NewMultiTenantPersistence(tmp)
	rbacProvider := NewTenantRBACProvider(tmp)
	billingStore := config.NewFileBillingStore(tmp)

	h := NewStripeWebhookHandlers(billingStore, persistence, rbacProvider, nil, nil, true, tmp)

	victimOrgID := "org_victim"
	attackerOrgID := "org_attacker"
	createTestOrg(t, persistence, victimOrgID, "victim@example.com")
	createTestOrg(t, persistence, attackerOrgID, "attacker@example.com")

	// Attacker pays with victim's email, but the checkout session is linked to the attacker's org.
	event := map[string]any{
		"id":   "evt_checkout_collision",
		"type": "checkout.session.completed",
		"data": map[string]any{
			"object": map[string]any{
				"id":             "cs_collision",
				"mode":           "subscription",
				"customer":       "cus_collision",
				"customer_email": "victim@example.com",
				"subscription":   "sub_collision",
				"metadata": map[string]any{
					"org_id":       attackerOrgID,
					"org_name":     "Attacker Org",
					"plan_version": "cloud-v1",
				},
			},
		},
	}
	payload, _ := json.Marshal(event)
	signed := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{
		Payload:   payload,
		Secret:    "whsec_test_collision",
		Timestamp: time.Now(),
		Scheme:    "v1",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/stripe", bytes.NewReader(payload))
	req.Header.Set("Stripe-Signature", signed.Header)
	rr := httptest.NewRecorder()
	h.HandleStripeWebhook(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d", rr.Code, http.StatusOK)
	}

	attackerState, err := billingStore.GetBillingState(attackerOrgID)
	if err != nil {
		t.Fatalf("GetBillingState(attacker): %v", err)
	}
	if attackerState == nil || attackerState.SubscriptionState != entitlements.SubStateActive {
		t.Fatalf("attacker billing state=%v, want subscription_state=%q", attackerState, entitlements.SubStateActive)
	}

	victimState, err := billingStore.GetBillingState(victimOrgID)
	if err != nil {
		t.Fatalf("GetBillingState(victim): %v", err)
	}
	if victimState != nil {
		t.Fatalf("victim billing state should be untouched (nil), got %+v", victimState)
	}
}
