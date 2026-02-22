package api

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
	"github.com/rs/zerolog/log"
)

const (
	reconcileInterval  = 6 * time.Hour
	staleBillingWindow = 48 * time.Hour
)

// StripeReconciler periodically scans billing states and logs warnings for
// drift between expected and actual subscription states. Log-only for beta.
type StripeReconciler struct {
	dataDir string
}

// NewStripeReconciler creates a reconciler.
func NewStripeReconciler(dataDir string) *StripeReconciler {
	return &StripeReconciler{dataDir: dataDir}
}

// Run starts the reconciliation loop. It blocks until ctx is cancelled.
func (sr *StripeReconciler) Run(ctx context.Context) {
	log.Info().Msg("Stripe reconciler started (log-only mode)")

	ticker := time.NewTicker(reconcileInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Stripe reconciler stopped")
			return
		case <-ticker.C:
			sr.reconcile(ctx)
		}
	}
}

func (sr *StripeReconciler) reconcile(ctx context.Context) {
	_ = ctx

	store := config.NewFileBillingStore(sr.dataDir)
	state, err := store.GetBillingState("default")
	if err != nil {
		log.Debug().Err(err).Msg("Stripe reconciler: no billing state found (self-hosted instance)")
		return
	}
	if state == nil {
		return
	}

	// Staleness signal: billing.json mtime (best-effort).
	if sr.dataDir != "" && state.StripeSubscriptionID != "" {
		billingPath := filepath.Join(sr.dataDir, "billing.json")
		if fi, statErr := os.Stat(billingPath); statErr == nil {
			if time.Since(fi.ModTime()) > staleBillingWindow {
				log.Warn().
					Str("stripe_subscription_id", state.StripeSubscriptionID).
					Str("stripe_customer_id", state.StripeCustomerID).
					Dur("stale_window", staleBillingWindow).
					Time("billing_file_mtime", fi.ModTime()).
					Msg("Stripe reconciler: billing state appears stale; ensure webhook processing is healthy")
			}
		}
	}

	// Check for drift between stored subscription state and expected capabilities.
	if state.StripeSubscriptionID != "" && state.SubscriptionState == pkglicensing.SubStateActive {
		// Active subscription: expected to have capabilities; nothing to warn about.
		return
	}

	if state.StripeSubscriptionID != "" && state.SubscriptionState == pkglicensing.SubStateGrace {
		log.Warn().
			Str("stripe_subscription_id", state.StripeSubscriptionID).
			Str("stripe_customer_id", state.StripeCustomerID).
			Str("subscription_state", string(state.SubscriptionState)).
			Msg("Stripe reconciler: tenant in grace period; verify Stripe dashboard for payment status")
	}

	if state.StripeSubscriptionID != "" && state.SubscriptionState == pkglicensing.SubStateCanceled {
		if len(state.Capabilities) > 0 {
			log.Warn().
				Str("stripe_subscription_id", state.StripeSubscriptionID).
				Str("subscription_state", string(state.SubscriptionState)).
				Int("capability_count", len(state.Capabilities)).
				Msg("Stripe reconciler: DRIFT: canceled subscription still has capabilities")
		}
	}
}
