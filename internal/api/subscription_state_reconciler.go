package api

import (
	"time"
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
