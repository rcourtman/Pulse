package stripe

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	"github.com/rs/zerolog/log"
	stripelib "github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/subscription"
)

const reconcileInterval = 6 * time.Hour

// Reconciler periodically refreshes subscription state from Stripe and applies
// any drift to tenant billing state.
type Reconciler struct {
	registry     *registry.TenantRegistry
	provisioner  *Provisioner
	stripeAPIKey string
}

// NewReconciler creates a billing reconciler.
func NewReconciler(reg *registry.TenantRegistry, provisioner *Provisioner, stripeAPIKey string) *Reconciler {
	return &Reconciler{
		registry:     reg,
		provisioner:  provisioner,
		stripeAPIKey: strings.TrimSpace(stripeAPIKey),
	}
}

// Run starts the reconciliation loop and blocks until ctx is cancelled.
func (r *Reconciler) Run(ctx context.Context) {
	if r == nil || r.registry == nil || r.provisioner == nil {
		log.Warn().Msg("Stripe reconciler disabled: missing dependencies")
		return
	}
	if r.stripeAPIKey == "" {
		log.Warn().Msg("Stripe reconciler disabled: STRIPE_API_KEY not configured")
		return
	}

	log.Info().Dur("interval", reconcileInterval).Msg("Stripe billing reconciler started")
	r.reconcile(ctx)

	ticker := time.NewTicker(reconcileInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Stripe billing reconciler stopped")
			return
		case <-ticker.C:
			r.reconcile(ctx)
		}
	}
}

func (r *Reconciler) reconcile(ctx context.Context) {
	stripelib.Key = r.stripeAPIKey

	accounts, err := r.registry.ListStripeAccounts()
	if err != nil {
		log.Error().Err(err).Msg("Stripe reconciler: list stripe accounts")
		return
	}

	for _, sa := range accounts {
		if ctx.Err() != nil {
			return
		}
		if sa == nil {
			continue
		}

		subID := strings.TrimSpace(sa.StripeSubscriptionID)
		if subID == "" {
			continue
		}

		params := &stripelib.SubscriptionParams{}
		params.Context = ctx
		params.AddExpand("items.data.price")
		stripeSub, getErr := subscription.Get(subID, params)
		if getErr != nil {
			var stripeErr *stripelib.Error
			if errors.As(getErr, &stripeErr) {
				log.Warn().
					Err(getErr).
					Str("account_id", sa.AccountID).
					Str("customer_id", sa.StripeCustomerID).
					Str("subscription_id", subID).
					Str("stripe_code", string(stripeErr.Code)).
					Msg("Stripe reconciler: fetch subscription failed")
			} else {
				log.Warn().
					Err(getErr).
					Str("account_id", sa.AccountID).
					Str("customer_id", sa.StripeCustomerID).
					Str("subscription_id", subID).
					Msg("Stripe reconciler: fetch subscription failed")
			}
			continue
		}
		if stripeSub == nil {
			continue
		}

		sub := mapStripeSubscription(stripeSub, sa.StripeCustomerID)
		acct, acctErr := r.registry.GetAccount(sa.AccountID)
		if acctErr != nil {
			log.Warn().
				Err(acctErr).
				Str("account_id", sa.AccountID).
				Msg("Stripe reconciler: account lookup failed")
			continue
		}
		if acct != nil && acct.Kind == registry.AccountKindMSP {
			if err := r.provisioner.HandleMSPSubscriptionUpdated(ctx, sub); err != nil {
				log.Warn().
					Err(err).
					Str("account_id", sa.AccountID).
					Str("customer_id", sa.StripeCustomerID).
					Str("subscription_id", sub.ID).
					Msg("Stripe reconciler: MSP subscription update failed")
			}
			continue
		}
		if err := r.provisioner.HandleSubscriptionUpdated(ctx, sub); err != nil {
			log.Warn().
				Err(err).
				Str("account_id", sa.AccountID).
				Str("customer_id", sa.StripeCustomerID).
				Str("subscription_id", sub.ID).
				Msg("Stripe reconciler: subscription update failed")
		}
	}
}

func mapStripeSubscription(src *stripelib.Subscription, fallbackCustomerID string) Subscription {
	out := Subscription{
		ID:       strings.TrimSpace(src.ID),
		Status:   string(src.Status),
		Metadata: src.Metadata,
	}

	if src.Customer != nil {
		out.Customer = strings.TrimSpace(src.Customer.ID)
	}
	if out.Customer == "" {
		out.Customer = strings.TrimSpace(fallbackCustomerID)
	}

	if src.Items != nil {
		for _, item := range src.Items.Data {
			if item == nil || item.Price == nil {
				continue
			}
			dstItem := struct {
				Price struct {
					ID       string            `json:"id"`
					Metadata map[string]string `json:"metadata"`
				} `json:"price"`
			}{}
			dstItem.Price.ID = strings.TrimSpace(item.Price.ID)
			dstItem.Price.Metadata = item.Price.Metadata
			out.Items.Data = append(out.Items.Data, dstItem)
		}
	}

	return out
}
