package api

import (
	"context"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// defaultLegacyExchangeRetrySchedule backs off quickly at first (a DNS blip
// at boot usually clears in seconds) and settles at the final interval for
// the lifetime of the process.
var defaultLegacyExchangeRetrySchedule = []time.Duration{
	30 * time.Second,
	time.Minute,
	2 * time.Minute,
	5 * time.Minute,
	10 * time.Minute,
	30 * time.Minute,
}

// scheduleLegacyExchangeRetry retries a failed startup v5→v6 license exchange
// in the background. The startup exchange runs once per process; without a
// retry loop, a transient license-server failure at first boot left a paying
// v5 customer on Community until they manually restarted or retried from the
// license panel. Only retryable (pending) failures schedule a loop; terminal
// classifications keep their persisted migration state for the UI.
func (h *LicenseHandlers) scheduleLegacyExchangeRetry(orgID, legacyJWT string) {
	if h == nil || strings.TrimSpace(legacyJWT) == "" {
		return
	}
	stop := make(chan struct{})
	if _, alreadyRunning := h.legacyExchangeRetries.LoadOrStore(orgID, stop); alreadyRunning {
		return
	}

	schedule := h.legacyExchangeRetrySchedule
	if len(schedule) == 0 {
		schedule = defaultLegacyExchangeRetrySchedule
	}

	go func() {
		defer h.legacyExchangeRetries.CompareAndDelete(orgID, stop)

		for attempt := 0; ; attempt++ {
			timer := time.NewTimer(schedule[min(attempt, len(schedule)-1)])
			select {
			case <-stop:
				timer.Stop()
				return
			case <-timer.C:
			}

			v, ok := h.services.Load(orgID)
			if !ok {
				// Tenant evicted; the next getTenantComponents call re-runs
				// the startup exchange itself.
				return
			}
			service := v.(*licenseService)
			if service.IsActivated() || service.Current() != nil {
				// Resolved through another path (manual activation).
				return
			}

			if _, err := service.Activate(legacyJWT); err != nil {
				migrationStatus := classifyLegacyExchangeErrorFromLicensing(err)
				if persistErr := h.setCommercialMigrationState(orgID, migrationStatus); persistErr != nil {
					log.Warn().Str("org_id", orgID).Err(persistErr).Msg("Failed to persist commercial migration state during background legacy exchange retry")
				}
				if migrationStatus == nil || migrationStatus.State != commercialMigrationStatePendingValue {
					log.Warn().Str("org_id", orgID).Err(err).Msg("Legacy license migration failed terminally; stopping background retries")
					return
				}
				log.Info().Str("org_id", orgID).Int("attempt", attempt+1).Err(err).Msg("Legacy license migration still pending; will retry in background")
				continue
			}

			if !service.IsActivated() {
				// Exchange "succeeded" without producing an activation; a
				// retry cannot improve on that, so leave state for the UI.
				return
			}

			if clearErr := h.setCommercialMigrationState(orgID, nil); clearErr != nil {
				log.Warn().Str("org_id", orgID).Err(clearErr).Msg("Failed to clear commercial migration state after background legacy exchange")
			}
			service.StartGrantRefresh(context.Background())
			if feedToken := revocationFeedToken(); feedToken != "" {
				service.StartRevocationPoll(context.Background(), feedToken)
			}
			h.syncReleaseDemoFixtureRuntime(orgID, service)
			if current := service.Current(); current != nil {
				log.Info().
					Str("org_id", orgID).
					Str("license_id", current.Claims.LicenseID).
					Int("attempts", attempt+1).
					Msg("Background retry completed v5 legacy license migration")
			}
			return
		}
	}()
}

// stopLegacyExchangeRetry cancels a pending background exchange retry loop.
func (h *LicenseHandlers) stopLegacyExchangeRetry(orgID string) {
	if h == nil {
		return
	}
	if v, ok := h.legacyExchangeRetries.LoadAndDelete(orgID); ok {
		if stop, ok := v.(chan struct{}); ok {
			close(stop)
		}
	}
}
