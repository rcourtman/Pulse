package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/license"
	"github.com/rcourtman/pulse-go-rewrite/internal/license/entitlements"
	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/webhook"
)

func TestStripeWebhook_SubscriptionUpdated_BackfillsIndexAndAppliesState(t *testing.T) {
	t.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_test_sub_updated")

	tmp := t.TempDir()
	persistence := config.NewMultiTenantPersistence(tmp)
	rbacProvider := NewTenantRBACProvider(tmp)
	billingStore := config.NewFileBillingStore(tmp)
	h := NewStripeWebhookHandlers(billingStore, persistence, rbacProvider, nil, nil, true, tmp)

	orgID := "org_sub_updated"
	createTestOrg(t, persistence, orgID, "owner@example.com")

	err := billingStore.SaveBillingState(orgID, &entitlements.BillingState{
		Capabilities:         []string{},
		Limits:               map[string]int64{},
		MetersEnabled:        []string{},
		PlanVersion:          string(entitlements.SubStateTrial),
		SubscriptionState:    entitlements.SubStateTrial,
		StripeCustomerID:     "cus_scan_only",
		StripeSubscriptionID: "sub_initial",
	})
	if err != nil {
		t.Fatalf("SaveBillingState: %v", err)
	}

	event := map[string]any{
		"id":   "evt_sub_updated_backfill",
		"type": "customer.subscription.updated",
		"data": map[string]any{
			"object": map[string]any{
				"id":       "sub_new",
				"customer": "cus_scan_only",
				"status":   "past_due",
				"items": map[string]any{
					"data": []map[string]any{
						{"price": map[string]any{"id": "   "}},
						{"price": map[string]any{"id": " price_gold "}},
					},
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
		Secret:    "whsec_test_sub_updated",
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

	state, err := billingStore.GetBillingState(orgID)
	if err != nil {
		t.Fatalf("GetBillingState: %v", err)
	}
	if state == nil {
		t.Fatalf("expected billing state")
	}
	if state.SubscriptionState != entitlements.SubStateGrace {
		t.Fatalf("subscription_state=%q, want %q", state.SubscriptionState, entitlements.SubStateGrace)
	}
	if state.StripePriceID != "price_gold" {
		t.Fatalf("stripe_price_id=%q, want %q", state.StripePriceID, "price_gold")
	}
	if state.PlanVersion != "stripe_price:price_gold" {
		t.Fatalf("plan_version=%q, want %q", state.PlanVersion, "stripe_price:price_gold")
	}
	if state.StripeSubscriptionID != "sub_new" {
		t.Fatalf("stripe_subscription_id=%q, want %q", state.StripeSubscriptionID, "sub_new")
	}
	if len(state.Capabilities) == 0 {
		t.Fatalf("expected paid capabilities to be granted")
	}
	hasAutoFix := false
	for _, capability := range state.Capabilities {
		if capability == license.FeatureAIAutoFix {
			hasAutoFix = true
			break
		}
	}
	if !hasAutoFix {
		t.Fatalf("expected capabilities to include %q, got %v", license.FeatureAIAutoFix, state.Capabilities)
	}

	mappedOrgID, ok, err := h.index.LookupOrgID("cus_scan_only")
	if err != nil {
		t.Fatalf("LookupOrgID: %v", err)
	}
	if !ok || mappedOrgID != orgID {
		t.Fatalf("index mapping mismatch: org=%q ok=%v, want org=%q ok=true", mappedOrgID, ok, orgID)
	}
}

func TestStripeWebhook_SubscriptionStatusMappingAndPaidCapabilities(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		status   string
		want     entitlements.SubscriptionState
		wantPaid bool
	}{
		{name: "active", status: "active", want: entitlements.SubStateActive, wantPaid: true},
		{name: "trialing", status: "trialing", want: entitlements.SubStateTrial, wantPaid: true},
		{name: "past due", status: "past_due", want: entitlements.SubStateGrace, wantPaid: true},
		{name: "unpaid", status: "unpaid", want: entitlements.SubStateGrace, wantPaid: true},
		{name: "canceled", status: "canceled", want: entitlements.SubStateCanceled, wantPaid: false},
		{name: "paused", status: "paused", want: entitlements.SubStateSuspended, wantPaid: false},
		{name: "incomplete", status: "incomplete", want: entitlements.SubStateExpired, wantPaid: false},
		{name: "incomplete expired", status: "incomplete_expired", want: entitlements.SubStateExpired, wantPaid: false},
		{name: "unknown defaults expired", status: "  UNKNOWN  ", want: entitlements.SubStateExpired, wantPaid: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := mapStripeSubscriptionStatusToState(tt.status)
			if got != tt.want {
				t.Fatalf("state=%q, want %q", got, tt.want)
			}
			if paid := shouldGrantPaidCapabilities(got); paid != tt.wantPaid {
				t.Fatalf("paid=%v, want %v", paid, tt.wantPaid)
			}
		})
	}
}

func TestStripeWebhook_FirstPriceID(t *testing.T) {
	t.Parallel()

	var empty stripeSubscription
	if got := firstPriceID(empty); got != "" {
		t.Fatalf("empty firstPriceID=%q, want empty", got)
	}

	var sub stripeSubscription
	err := json.Unmarshal([]byte(`{"items":{"data":[{"price":{"id":"   "}},{"price":{"id":" price_123 "}}]}}`), &sub)
	if err != nil {
		t.Fatalf("unmarshal stripe subscription: %v", err)
	}
	if got := firstPriceID(sub); got != "price_123" {
		t.Fatalf("firstPriceID=%q, want %q", got, "price_123")
	}
}

func TestStripeWebhook_HandleEvent_ErrorsAndUnhandled(t *testing.T) {
	t.Parallel()

	h := &StripeWebhookHandlers{}

	if err := h.handleEvent(context.Background(), nil, nil); err == nil {
		t.Fatalf("expected nil event error")
	}

	invalidUpdated := &stripe.Event{
		Type: "customer.subscription.updated",
		Data: &stripe.EventData{Raw: json.RawMessage(`{`)},
	}
	if err := h.handleEvent(context.Background(), invalidUpdated, nil); err == nil {
		t.Fatalf("expected decode error for subscription.updated")
	}

	unhandled := &stripe.Event{
		ID:   "evt_unhandled",
		Type: "customer.created",
	}
	if err := h.handleEvent(context.Background(), unhandled, nil); err != nil {
		t.Fatalf("unexpected error for unhandled event: %v", err)
	}
}
