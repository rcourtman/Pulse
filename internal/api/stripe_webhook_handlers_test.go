package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

func createTestOrg(t *testing.T, persistence *config.MultiTenantPersistence, orgID, ownerEmail string) {
	t.Helper()

	if _, err := persistence.GetPersistence(orgID); err != nil {
		t.Fatalf("GetPersistence(%s): %v", orgID, err)
	}

	now := time.Now().UTC()
	org := &models.Organization{
		ID:          orgID,
		DisplayName: orgID,
		CreatedAt:   now,
		OwnerUserID: ownerEmail,
		Members: []models.OrganizationMember{
			{
				UserID:  ownerEmail,
				Role:    models.OrgRoleOwner,
				AddedAt: now,
				AddedBy: ownerEmail,
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

func TestStripeWebhook_SubscriptionDeleted_RevokesCapabilities(t *testing.T) {
	t.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_test_789")

	tmp := t.TempDir()
	persistence := config.NewMultiTenantPersistence(tmp)
	rbacProvider := NewTenantRBACProvider(tmp)
	billingStore := config.NewFileBillingStore(tmp)

	h := NewStripeWebhookHandlers(billingStore, persistence, rbacProvider, nil, nil, true, tmp)

	orgID := "org_gamma"
	createTestOrg(t, persistence, orgID, "user3@example.com")

	// First: provision via checkout to establish customer->org mapping.
	checkout := map[string]any{
		"id":   "evt_checkout_2",
		"type": "checkout.session.completed",
		"data": map[string]any{
			"object": map[string]any{
				"id":             "cs_2",
				"mode":           "subscription",
				"customer":       "cus_del",
				"customer_email": "user3@example.com",
				"subscription":   "sub_del",
				"metadata": map[string]any{
					"org_id":       orgID,
					"org_name":     "Gamma Org",
					"plan_version": "cloud-v1",
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

	// Then: delete subscription should cancel + strip capabilities.
	del := map[string]any{
		"id":   "evt_sub_deleted_1",
		"type": "customer.subscription.deleted",
		"data": map[string]any{
			"object": map[string]any{
				"id":       "sub_del",
				"customer": "cus_del",
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

	state, err := billingStore.GetBillingState(orgID)
	if err != nil {
		t.Fatalf("GetBillingState: %v", err)
	}
	if state.SubscriptionState != entitlements.SubStateCanceled {
		t.Fatalf("subscription_state=%q, want %q", state.SubscriptionState, entitlements.SubStateCanceled)
	}
	if len(state.Capabilities) != 0 {
		t.Fatalf("capabilities=%v, want empty", state.Capabilities)
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
