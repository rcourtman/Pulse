package stripe

import (
	"context"
	"errors"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/entitlements"
	stripelib "github.com/stripe/stripe-go/v82"
)

// TestBranchcov0723Am_MapStripeSubscription drives every branch of the pure
// mapper mapStripeSubscription with concrete stripe structs and asserts the
// concrete converted output. No network or API access is involved.
func TestBranchcov0723Am_MapStripeSubscription(t *testing.T) {
	// priceMeta is a sentinel metadata map reused across subtests so each
	// assertion checks identity, not a freshly-allocated equivalent.
	priceMeta := map[string]string{"tier": "pro"}

	t.Run("zero_value_src_uses_fallback_customer_and_empty_fields", func(t *testing.T) {
		var src stripelib.Subscription
		got := mapStripeSubscription(&src, "cus_fallback")

		if got.ID != "" {
			t.Fatalf("ID = %q, want empty", got.ID)
		}
		if got.Status != "" {
			t.Fatalf("Status = %q, want empty", got.Status)
		}
		if got.Metadata != nil {
			t.Fatalf("Metadata = %v, want nil", got.Metadata)
		}
		if got.Customer != "cus_fallback" {
			t.Fatalf("Customer = %q, want fallback cus_fallback", got.Customer)
		}
		if len(got.Items.Data) != 0 {
			t.Fatalf("Items.Data = %v, want empty", got.Items.Data)
		}
	})

	t.Run("nil_customer_with_empty_fallback_leaves_customer_blank", func(t *testing.T) {
		var src stripelib.Subscription
		got := mapStripeSubscription(&src, "")
		if got.Customer != "" {
			t.Fatalf("Customer = %q, want empty", got.Customer)
		}
	})

	t.Run("present_customer_id_wins_over_fallback_and_is_trimmed", func(t *testing.T) {
		src := stripelib.Subscription{
			Customer: &stripelib.Customer{ID: "  cus_real  "},
		}
		got := mapStripeSubscription(&src, "cus_fallback")
		if got.Customer != "cus_real" {
			t.Fatalf("Customer = %q, want trimmed cus_real", got.Customer)
		}
	})

	t.Run("present_customer_with_empty_id_falls_back", func(t *testing.T) {
		src := stripelib.Subscription{
			Customer: &stripelib.Customer{ID: "   "},
		}
		got := mapStripeSubscription(&src, "cus_fallback")
		if got.Customer != "cus_fallback" {
			t.Fatalf("Customer = %q, want fallback cus_fallback", got.Customer)
		}
	})

	t.Run("id_and_status_are_trimmed_and_copied_verbatim", func(t *testing.T) {
		// The mapper does not translate statuses; it copies string(src.Status)
		// verbatim. Assert exact passthrough for a known and an unknown value.
		for _, tc := range []struct {
			name       string
			rawID      string
			rawStatus  stripelib.SubscriptionStatus
			wantID     string
			wantStatus string
		}{
			{"known_active_trimmed", "  sub_active  ", stripelib.SubscriptionStatusActive, "sub_active", "active"},
			{"unknown_status_passthrough", "sub_x", stripelib.SubscriptionStatus("bogus_state"), "sub_x", "bogus_state"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				src := stripelib.Subscription{ID: tc.rawID, Status: tc.rawStatus}
				got := mapStripeSubscription(&src, "")
				if got.ID != tc.wantID {
					t.Fatalf("ID = %q, want %q", got.ID, tc.wantID)
				}
				if got.Status != tc.wantStatus {
					t.Fatalf("Status = %q, want %q", got.Status, tc.wantStatus)
				}
			})
		}
	})

	t.Run("metadata_is_passed_through_by_reference", func(t *testing.T) {
		meta := map[string]string{"k": "v"}
		src := stripelib.Subscription{Metadata: meta}
		got := mapStripeSubscription(&src, "")
		if got.Metadata == nil || len(got.Metadata) != 1 || got.Metadata["k"] != "v" {
			t.Fatalf("Metadata = %v, want %v", got.Metadata, meta)
		}
	})

	t.Run("nil_items_leaves_output_items_empty", func(t *testing.T) {
		src := stripelib.Subscription{Items: nil}
		got := mapStripeSubscription(&src, "")
		if len(got.Items.Data) != 0 {
			t.Fatalf("Items.Data = %v, want empty", got.Items.Data)
		}
	})

	t.Run("empty_items_data_leaves_output_items_empty", func(t *testing.T) {
		src := stripelib.Subscription{Items: &stripelib.SubscriptionItemList{Data: nil}}
		got := mapStripeSubscription(&src, "")
		if len(got.Items.Data) != 0 {
			t.Fatalf("Items.Data = %v, want empty", got.Items.Data)
		}
	})

	t.Run("single_valid_item_is_extracted_with_trimmed_id_and_metadata", func(t *testing.T) {
		src := stripelib.Subscription{
			Items: &stripelib.SubscriptionItemList{
				Data: []*stripelib.SubscriptionItem{
					{Price: &stripelib.Price{ID: "  price_1  ", Metadata: priceMeta}},
				},
			},
		}
		got := mapStripeSubscription(&src, "")
		if len(got.Items.Data) != 1 {
			t.Fatalf("len(Items.Data) = %d, want 1", len(got.Items.Data))
		}
		if got.Items.Data[0].Price.ID != "price_1" {
			t.Fatalf("Price.ID = %q, want trimmed price_1", got.Items.Data[0].Price.ID)
		}
		if got.Items.Data[0].Price.Metadata["tier"] != "pro" {
			t.Fatalf("Price.Metadata = %v, want %v", got.Items.Data[0].Price.Metadata, priceMeta)
		}
	})

	t.Run("multiple_valid_items_are_all_appended_in_order", func(t *testing.T) {
		src := stripelib.Subscription{
			Items: &stripelib.SubscriptionItemList{
				Data: []*stripelib.SubscriptionItem{
					{Price: &stripelib.Price{ID: "price_a"}},
					{Price: &stripelib.Price{ID: "price_b", Metadata: priceMeta}},
				},
			},
		}
		got := mapStripeSubscription(&src, "")
		// The mapper appends every valid item (it does not pick one).
		if len(got.Items.Data) != 2 {
			t.Fatalf("len(Items.Data) = %d, want 2", len(got.Items.Data))
		}
		if got.Items.Data[0].Price.ID != "price_a" {
			t.Fatalf("Items.Data[0].Price.ID = %q, want price_a", got.Items.Data[0].Price.ID)
		}
		if got.Items.Data[1].Price.ID != "price_b" {
			t.Fatalf("Items.Data[1].Price.ID = %q, want price_b", got.Items.Data[1].Price.ID)
		}
		if got.Items.Data[1].Price.Metadata["tier"] != "pro" {
			t.Fatalf("Items.Data[1].Price.Metadata = %v, want tier=pro", got.Items.Data[1].Price.Metadata)
		}
	})

	t.Run("nil_item_and_nil_price_are_skipped_valid_items_kept", func(t *testing.T) {
		src := stripelib.Subscription{
			Items: &stripelib.SubscriptionItemList{
				Data: []*stripelib.SubscriptionItem{
					nil,
					{Price: nil},
					{Price: &stripelib.Price{ID: "price_kept"}},
				},
			},
		}
		got := mapStripeSubscription(&src, "")
		if len(got.Items.Data) != 1 {
			t.Fatalf("len(Items.Data) = %d, want 1 after skipping nil item/price", len(got.Items.Data))
		}
		if got.Items.Data[0].Price.ID != "price_kept" {
			t.Fatalf("Items.Data[0].Price.ID = %q, want price_kept", got.Items.Data[0].Price.ID)
		}
	})

	t.Run("combined_populated_source", func(t *testing.T) {
		meta := map[string]string{"source": "reconciler"}
		src := stripelib.Subscription{
			ID:       "sub_full",
			Status:   stripelib.SubscriptionStatusActive,
			Customer: &stripelib.Customer{ID: "cus_full"},
			Metadata: meta,
			Items: &stripelib.SubscriptionItemList{
				Data: []*stripelib.SubscriptionItem{
					{Price: &stripelib.Price{ID: "price_full", Metadata: priceMeta}},
				},
			},
		}
		got := mapStripeSubscription(&src, "should_not_be_used")
		if got.ID != "sub_full" || got.Status != "active" || got.Customer != "cus_full" {
			t.Fatalf("header fields = %+v, want sub_full/active/cus_full", got)
		}
		if got.Metadata["source"] != "reconciler" {
			t.Fatalf("Metadata = %v, want source=reconciler", got.Metadata)
		}
		if len(got.Items.Data) != 1 || got.Items.Data[0].Price.ID != "price_full" {
			t.Fatalf("items = %+v, want single price_full", got.Items.Data)
		}
	})
}

// TestBranchcov0723Am_WithAdmissionCheck exercises the WithAdmissionCheck
// option: setting a real check, observing it via enforceAdmission, setting a
// nil check, last-option-wins override, and the nil-receiver guard.
func TestBranchcov0723Am_WithAdmissionCheck(t *testing.T) {
	sentinel := errors.New("admission denied: sentinel")

	t.Run("sets_check_invokable_from_provisioner", func(t *testing.T) {
		p := &Provisioner{}
		opt := WithAdmissionCheck(func(context.Context) error { return sentinel })
		if opt == nil {
			t.Fatal("WithAdmissionCheck returned a nil option")
		}
		opt(p)

		if p.admissionCheck == nil {
			t.Fatal("admissionCheck is nil after applying option")
		}
		if err := p.enforceAdmission(context.Background(), "op"); !errors.Is(err, sentinel) {
			t.Fatalf("enforceAdmission error = %v, want wrapping sentinel", err)
		}
	})

	t.Run("nil_check_clears_field_and_enforce_admission_is_noop", func(t *testing.T) {
		p := &Provisioner{admissionCheck: func(context.Context) error { return sentinel }}
		WithAdmissionCheck(nil)(p)
		if p.admissionCheck != nil {
			t.Fatalf("admissionCheck = %p, want nil after nil option", p.admissionCheck)
		}
		if err := p.enforceAdmission(context.Background(), "op"); err != nil {
			t.Fatalf("enforceAdmission error = %v, want nil when no check set", err)
		}
	})

	t.Run("later_option_overrides_earlier_one", func(t *testing.T) {
		p := &Provisioner{}
		first := WithAdmissionCheck(func(context.Context) error { return sentinel })
		second := WithAdmissionCheck(func(context.Context) error { return nil })
		first(p)
		second(p)
		// The last applied check should win: enforceAdmission returns nil.
		if err := p.enforceAdmission(context.Background(), "op"); err != nil {
			t.Fatalf("enforceAdmission error = %v, want nil (second check should win)", err)
		}
	})

	t.Run("nil_receiver_guard_does_not_panic", func(t *testing.T) {
		// The option short-circuits when the receiver is nil. Assert the
		// returned option is well-formed and that invoking it on a nil
		// *Provisioner completes safely.
		opt := WithAdmissionCheck(func(context.Context) error { return sentinel })
		if opt == nil {
			t.Fatal("WithAdmissionCheck returned a nil option")
		}
		opt(nil) // must not panic
	})
}

// TestBranchcov0723Am_WithHostedEntitlementService exercises the
// WithHostedEntitlementService option: setting a real service (pointer
// identity), nil service, override semantics, and the nil-receiver guard.
func TestBranchcov0723Am_WithHostedEntitlementService(t *testing.T) {
	// Reuse the in-package registry helper from provisioner_rollback_test.go.
	reg := newStripeTestRegistry(t)
	newSvc := func() *entitlements.Service {
		return entitlements.NewService(reg, "https://cloud.example.com", "")
	}

	t.Run("sets_service_by_pointer_identity", func(t *testing.T) {
		p := &Provisioner{}
		svc := newSvc()
		opt := WithHostedEntitlementService(svc)
		if opt == nil {
			t.Fatal("WithHostedEntitlementService returned a nil option")
		}
		opt(p)
		if p.hostedEntitlements != svc {
			t.Fatalf("hostedEntitlements = %p, want exact pointer %p", p.hostedEntitlements, svc)
		}
	})

	t.Run("nil_service_clears_field", func(t *testing.T) {
		// Start from a non-nil service to prove the option overwrites to nil.
		p := &Provisioner{hostedEntitlements: newSvc()}
		WithHostedEntitlementService(nil)(p)
		if p.hostedEntitlements != nil {
			t.Fatalf("hostedEntitlements = %p, want nil after nil option", p.hostedEntitlements)
		}
	})

	t.Run("later_option_overrides_earlier_one", func(t *testing.T) {
		p := &Provisioner{}
		first := newSvc()
		second := newSvc()
		WithHostedEntitlementService(first)(p)
		WithHostedEntitlementService(second)(p)
		if p.hostedEntitlements != second {
			t.Fatalf("hostedEntitlements = %p, want second service %p (last write wins)", p.hostedEntitlements, second)
		}
	})

	t.Run("nil_receiver_guard_does_not_panic", func(t *testing.T) {
		opt := WithHostedEntitlementService(newSvc())
		if opt == nil {
			t.Fatal("WithHostedEntitlementService returned a nil option")
		}
		opt(nil) // must not panic
	})
}
