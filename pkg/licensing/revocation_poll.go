package licensing

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	// defaultRevocationPollInterval is the default interval between revocation feed polls.
	defaultRevocationPollInterval = 5 * time.Minute

	// revocationPollJitter adds randomness to prevent thundering herd.
	revocationPollJitter = 0.2

	// minRevocationPollBackoff is the minimum backoff after a failed poll.
	minRevocationPollBackoff = 30 * time.Second

	// maxRevocationPollBackoff is the maximum backoff after consecutive failures.
	maxRevocationPollBackoff = 10 * time.Minute
)

// revocationPollLoop holds state for the background revocation feed poller.
type revocationPollLoop struct {
	mu        sync.Mutex
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	running   bool
	feedToken string
	lastSeq   int64
}

// StartRevocationPoll starts the background revocation feed poller.
// The feedToken authenticates access to the revocation feed endpoint.
// It is safe to call multiple times — only the first call starts the loop.
func (s *Service) StartRevocationPoll(ctx context.Context, feedToken string) {
	if feedToken == "" {
		return // No token → revocation polling disabled.
	}

	s.mu.Lock()
	if s.revocationPoll == nil {
		s.revocationPoll = &revocationPollLoop{}
	}
	loop := s.revocationPoll
	s.mu.Unlock()

	loop.mu.Lock()
	defer loop.mu.Unlock()

	if loop.running {
		return
	}

	loop.feedToken = feedToken
	pollCtx, cancel := context.WithCancel(ctx)
	loop.cancel = cancel
	loop.running = true
	loop.wg.Add(1)

	go func() {
		defer func() {
			loop.mu.Lock()
			loop.running = false
			loop.mu.Unlock()
			loop.wg.Done()
		}()
		s.runRevocationPollLoop(pollCtx)
	}()

	log.Info().Msg("Revocation poll loop started")
}

// StopRevocationPoll stops the background revocation feed poller and waits for it to exit.
func (s *Service) StopRevocationPoll() {
	s.mu.RLock()
	loop := s.revocationPoll
	s.mu.RUnlock()

	if loop == nil {
		return
	}

	loop.mu.Lock()
	if !loop.running {
		loop.mu.Unlock()
		return
	}
	loop.cancel()
	loop.mu.Unlock()

	loop.wg.Wait()
	log.Info().Msg("Revocation poll loop stopped")
}

// runRevocationPollLoop is the main poll goroutine.
func (s *Service) runRevocationPollLoop(ctx context.Context) {
	consecutiveFailures := 0

	for {
		interval := nextRevocationPollInterval(consecutiveFailures)

		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}

		if err := s.pollRevocationsOnce(ctx); err != nil {
			consecutiveFailures++
			log.Warn().
				Err(err).
				Int("consecutive_failures", consecutiveFailures).
				Msg("Revocation poll failed")
		} else {
			consecutiveFailures = 0
		}
	}
}

// pollRevocationsOnce performs a single revocation feed poll.
// maxPages limits pagination depth to prevent infinite loops.
func (s *Service) pollRevocationsOnce(ctx context.Context) error {
	return s.pollRevocationsPage(ctx, 20) // Max 20 pages per poll cycle (10k events).
}

func (s *Service) pollRevocationsPage(ctx context.Context, remainingPages int) error {
	if remainingPages <= 0 {
		log.Warn().Msg("Revocation poll hit pagination limit, will continue next cycle")
		return nil
	}
	s.mu.RLock()
	client := s.serverClient
	state := s.activationState
	s.mu.RUnlock()

	if client == nil {
		return fmt.Errorf("license server client not configured")
	}
	if state == nil {
		return nil // No activation — nothing to check revocations for.
	}

	s.mu.RLock()
	loop := s.revocationPoll
	s.mu.RUnlock()

	if loop == nil {
		return fmt.Errorf("revocation poll not initialized")
	}

	loop.mu.Lock()
	feedToken := loop.feedToken
	sinceSeq := loop.lastSeq
	loop.mu.Unlock()

	resp, err := client.FetchRevocations(ctx, feedToken, sinceSeq, 500)
	if err != nil {
		return fmt.Errorf("fetch revocations: %w", err)
	}

	// Process events.
	for _, event := range resp.Events {
		s.handleRevocationEvent(event)
	}

	// Update checkpoint — but only if sequence advanced.
	loop.mu.Lock()
	advanced := resp.NextSeq > loop.lastSeq
	if advanced {
		loop.lastSeq = resp.NextSeq
	}
	loop.mu.Unlock()

	// If there are more events and sequence advanced, fetch next page.
	// Guard against server returning HasMore=true without advancing NextSeq.
	if resp.HasMore && advanced {
		return s.pollRevocationsPage(ctx, remainingPages-1)
	}

	return nil
}

// handleRevocationEvent processes a single revocation event.
func (s *Service) handleRevocationEvent(event RevocationEvent) {
	s.mu.RLock()
	state := s.activationState
	s.mu.RUnlock()

	if state == nil {
		return
	}

	switch event.Action {
	case "revoke_license":
		if event.LicenseID == state.LicenseID {
			log.Warn().
				Str("license_id", event.LicenseID).
				Str("reason", event.ReasonCode).
				Msg("License revoked via revocation feed — clearing activation")
			s.clearActivationState()
		}

	case "revoke_installation":
		if event.InstallationID == state.InstallationID {
			log.Warn().
				Str("installation_id", event.InstallationID).
				Str("reason", event.ReasonCode).
				Msg("Installation revoked via revocation feed — clearing activation")
			s.clearActivationState()
		}

	case "bump_license_version":
		if event.LicenseID == state.LicenseID {
			// The current grant may be stale. Force an immediate refresh
			// to pick up the new license version.
			log.Info().
				Str("license_id", event.LicenseID).
				Int64("min_version", event.MinLicenseVersion).
				Msg("License version bumped — triggering immediate grant refresh")
			go func() {
				if err := s.refreshGrantOnce(context.Background()); err != nil {
					var apiErr *LicenseServerError
					if errors.As(err, &apiErr) && apiErr.StatusCode == 401 {
						s.clearActivationState()
						return
					}
					log.Warn().Err(err).Msg("Immediate grant refresh after version bump failed")
				}
			}()
		}
	}
}

// nextRevocationPollInterval calculates the next poll delay with jitter and backoff.
func nextRevocationPollInterval(consecutiveFailures int) time.Duration {
	if consecutiveFailures > 0 {
		backoff := minRevocationPollBackoff * (1 << min(consecutiveFailures-1, 10))
		if backoff > maxRevocationPollBackoff {
			backoff = maxRevocationPollBackoff
		}
		return backoff
	}

	jitterRange := float64(defaultRevocationPollInterval) * revocationPollJitter
	offset := (rand.Float64()*2 - 1) * jitterRange
	return defaultRevocationPollInterval + time.Duration(offset)
}
