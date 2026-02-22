package api

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rs/zerolog/log"
)

const (
	reconcileInterval      = 6 * time.Hour
	staleSubscriptionState = 48 * time.Hour
)

// SubscriptionStateReconciler periodically scans subscription state and logs warnings for
// drift between expected and actual subscription states. Log-only for beta.
type SubscriptionStateReconciler struct {
	dataDir string
}

// NewSubscriptionStateReconciler creates a reconciler.
func NewSubscriptionStateReconciler(dataDir string) *SubscriptionStateReconciler {
	return &SubscriptionStateReconciler{dataDir: dataDir}
}

// Run starts the reconciliation loop. It blocks until ctx is cancelled.
func (sr *SubscriptionStateReconciler) Run(ctx context.Context) {
	log.Info().Msg("Subscription reconciler started (log-only mode)")

	ticker := time.NewTicker(reconcileInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Subscription reconciler stopped")
			return
		case <-ticker.C:
			sr.reconcile(ctx)
		}
	}
}

func (sr *SubscriptionStateReconciler) reconcile(ctx context.Context) {
	_ = ctx

	store := config.NewFileBillingStore(sr.dataDir)
	state, err := store.GetBillingState("default")
	if err != nil {
		log.Debug().Err(err).Msg("Subscription reconciler: no state found (self-hosted instance)")
		return
	}
	if state == nil {
		return
	}

	// Staleness signal: billing.json mtime (best-effort).
	if sr.dataDir != "" && state.StripeSubscriptionID != "" {
		billingPath := filepath.Join(sr.dataDir, "billing.json")
		if fi, statErr := os.Stat(billingPath); statErr == nil {
			if time.Since(fi.ModTime()) > staleSubscriptionState {
				log.Warn().
					Str("stripe_subscription_id", state.StripeSubscriptionID).
					Str("stripe_customer_id", state.StripeCustomerID).
					Dur("stale_window", staleSubscriptionState).
					Time("billing_file_mtime", fi.ModTime()).
					Msg("Subscription reconciler: state appears stale; ensure webhook processing is healthy")
			}
		}
	}

	// Check for drift between stored subscription state and expected capabilities.
	if state.StripeSubscriptionID != "" && state.SubscriptionState == subscriptionStateActiveValue {
		// Active subscription: expected to have capabilities; nothing to warn about.
		return
	}

	if state.StripeSubscriptionID != "" && state.SubscriptionState == subscriptionStateGraceValue {
		log.Warn().
			Str("stripe_subscription_id", state.StripeSubscriptionID).
			Str("stripe_customer_id", state.StripeCustomerID).
			Str("subscription_state", string(state.SubscriptionState)).
			Msg("Subscription reconciler: tenant in grace period; verify payment-provider dashboard")
	}

	if state.StripeSubscriptionID != "" && state.SubscriptionState == subscriptionStateCanceledValue {
		if len(state.Capabilities) > 0 {
			log.Warn().
				Str("stripe_subscription_id", state.StripeSubscriptionID).
				Str("subscription_state", string(state.SubscriptionState)).
				Int("capability_count", len(state.Capabilities)).
				Msg("Subscription reconciler: DRIFT: canceled subscription still has capabilities")
		}
	}
}
