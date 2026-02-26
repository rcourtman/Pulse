package licensing

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	// defaultRefreshInterval is the default interval between grant refreshes.
	defaultRefreshInterval = 6 * time.Hour

	// defaultRefreshJitter is the default jitter applied to the refresh interval.
	defaultRefreshJitter = 0.2

	// minRefreshBackoff is the minimum backoff after a failed refresh.
	minRefreshBackoff = 30 * time.Second

	// maxRefreshBackoff is the maximum backoff after consecutive failed refreshes.
	maxRefreshBackoff = 30 * time.Minute
)

// grantRefreshLoop holds the state for the background grant refresh goroutine.
type grantRefreshLoop struct {
	mu              sync.Mutex
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	running         bool
	refreshInterval time.Duration
	jitterPercent   float64
}

// StartGrantRefresh starts the background grant refresh loop.
// It is safe to call multiple times — only the first call starts the loop.
// The loop runs until StopGrantRefresh is called or the service is cleared.
func (s *Service) StartGrantRefresh(ctx context.Context) {
	s.mu.Lock()
	if s.grantRefresh == nil {
		s.grantRefresh = &grantRefreshLoop{
			refreshInterval: defaultRefreshInterval,
			jitterPercent:   defaultRefreshJitter,
		}
	}
	loop := s.grantRefresh
	s.mu.Unlock()

	loop.mu.Lock()
	defer loop.mu.Unlock()

	if loop.running {
		return
	}

	refreshCtx, cancel := context.WithCancel(ctx)
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
		s.runGrantRefreshLoop(refreshCtx)
	}()

	log.Info().Msg("Grant refresh loop started")
}

// StopGrantRefresh stops the background grant refresh loop and waits for it to exit.
func (s *Service) StopGrantRefresh() {
	s.mu.RLock()
	loop := s.grantRefresh
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
	loop.running = false
	loop.mu.Unlock()

	loop.wg.Wait()
	log.Info().Msg("Grant refresh loop stopped")
}

// SetRefreshHints updates the refresh interval and jitter from server-provided hints.
func (s *Service) SetRefreshHints(hints RefreshHints) {
	s.mu.Lock()
	if s.grantRefresh == nil {
		s.grantRefresh = &grantRefreshLoop{
			refreshInterval: defaultRefreshInterval,
			jitterPercent:   defaultRefreshJitter,
		}
	}
	loop := s.grantRefresh
	s.mu.Unlock()

	loop.mu.Lock()
	defer loop.mu.Unlock()

	if hints.IntervalSeconds > 0 {
		interval := time.Duration(hints.IntervalSeconds) * time.Second
		// Clamp to [1m, 24h] to prevent tight loops or excessively long intervals.
		if interval < time.Minute {
			interval = time.Minute
		} else if interval > 24*time.Hour {
			interval = 24 * time.Hour
		}
		loop.refreshInterval = interval
	}
	// Accept jitter=0 to disable jitter, but only when the hint is from a real
	// server response (IntervalSeconds > 0 guards against zero-value structs).
	if hints.JitterPercent > 0 || (hints.JitterPercent == 0 && hints.IntervalSeconds > 0) {
		jitter := hints.JitterPercent
		// The server sends jitter as a whole number (e.g. 20 for 20%).
		// Convert to fraction if >= 1 (no valid use case for >=100% jitter).
		if jitter >= 1 {
			jitter = jitter / 100
		}
		if jitter <= 0.5 {
			loop.jitterPercent = jitter
		}
	}
}

// runGrantRefreshLoop is the main refresh goroutine.
func (s *Service) runGrantRefreshLoop(ctx context.Context) {
	consecutiveFailures := 0

	for {
		interval := s.nextRefreshInterval(consecutiveFailures)

		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}

		if err := s.refreshGrantOnce(ctx); err != nil {
			consecutiveFailures++

			var apiErr *LicenseServerError
			if errors.As(err, &apiErr) && apiErr.StatusCode == 401 {
				// 401 means the installation token is revoked or expired.
				// Clear activation state and revert to free tier.
				log.Warn().Msg("Grant refresh received 401 (revoked/expired), clearing activation")
				s.clearActivationState()
				return
			}

			log.Warn().
				Err(err).
				Int("consecutive_failures", consecutiveFailures).
				Dur("next_retry", s.nextRefreshInterval(consecutiveFailures)).
				Msg("Grant refresh failed")
		} else {
			consecutiveFailures = 0
		}
	}
}

// refreshGrantOnce performs a single grant refresh attempt.
func (s *Service) refreshGrantOnce(ctx context.Context) error {
	s.mu.RLock()
	client := s.serverClient
	state := s.activationState
	persistence := s.persistence
	s.mu.RUnlock()

	if client == nil {
		return fmt.Errorf("license server client not configured")
	}
	if state == nil {
		return fmt.Errorf("no activation state")
	}

	req := RefreshGrantRequest{
		InstallationID:      state.InstallationID,
		InstanceFingerprint: state.InstanceFingerprint,
		CurrentGrantJTI:     state.GrantJTI,
	}

	resp, err := client.RefreshGrant(ctx, state.InstallationID, state.InstallationToken, req)
	if err != nil {
		return fmt.Errorf("refresh grant: %w", err)
	}

	// Parse the new grant JWT.
	gc, err := parseGrantJWT(resp.Grant.JWT)
	if err != nil {
		return fmt.Errorf("parse refreshed grant: %w", err)
	}

	// Update the license from the new grant.
	lic := grantClaimsToLicense(gc, resp.Grant.JWT)

	s.mu.Lock()

	// If activation state was cleared concurrently (e.g. by a revocation event),
	// do not re-install a license. This prevents a race where a bump_license_version
	// refresh overwrites a revocation clear.
	if s.activationState == nil {
		s.mu.Unlock()
		return nil
	}

	s.license = cloneLicense(lic)
	source := NewTokenSource(&s.license.Claims)
	s.evaluator = NewEvaluator(source)

	// Update persisted activation state.
	if s.activationState != nil {
		s.activationState.GrantJWT = resp.Grant.JWT
		s.activationState.GrantJTI = resp.Grant.JTI
		s.activationState.GrantExpiresAt = resp.Grant.ParseExpiresAt()
		s.activationState.LastRefreshedAt = time.Now().Unix()
	}

	cb := s.onLicenseChange
	snapshot := cloneLicense(s.license)
	stateCopy := s.activationState
	s.mu.Unlock()

	// Apply updated refresh hints from the server (policy may change between refreshes).
	s.SetRefreshHints(resp.RefreshPolicy)

	// Persist the updated activation state.
	if persistence != nil && stateCopy != nil {
		if err := persistence.SaveActivationState(stateCopy); err != nil {
			log.Warn().Err(err).Msg("Failed to persist activation state after grant refresh")
		}
	}

	if cb != nil {
		cb(snapshot)
	}

	log.Info().
		Str("grant_jti", resp.Grant.JTI).
		Str("expires_at", resp.Grant.ExpiresAt).
		Msg("Grant refreshed successfully")

	return nil
}

// clearActivationState clears the activation state and reverts to free tier.
// Called when the license server indicates the installation is revoked.
func (s *Service) clearActivationState() {
	s.mu.Lock()
	persistence := s.persistence
	s.activationState = nil
	s.license = nil
	s.evaluator = nil
	cb := s.onLicenseChange
	s.mu.Unlock()

	if persistence != nil {
		if err := persistence.ClearActivationState(); err != nil {
			log.Warn().Err(err).Msg("Failed to clear activation state file after revocation")
		}
	}

	if cb != nil {
		cb(nil)
	}
}

// nextRefreshInterval calculates the next refresh delay with jitter and backoff.
func (s *Service) nextRefreshInterval(consecutiveFailures int) time.Duration {
	s.mu.RLock()
	loop := s.grantRefresh
	s.mu.RUnlock()

	if loop == nil {
		return defaultRefreshInterval
	}

	loop.mu.Lock()
	interval := loop.refreshInterval
	jitter := loop.jitterPercent
	loop.mu.Unlock()

	if consecutiveFailures > 0 {
		// Exponential backoff: 30s, 60s, 120s, ..., capped at 30m.
		backoff := minRefreshBackoff * (1 << min(consecutiveFailures-1, 10))
		if backoff > maxRefreshBackoff {
			backoff = maxRefreshBackoff
		}
		return backoff
	}

	// Apply jitter: interval ± jitter%.
	jitterRange := float64(interval) * jitter
	offset := (rand.Float64()*2 - 1) * jitterRange
	return interval + time.Duration(offset)
}

// parseGrantJWT extracts GrantClaims from a grant JWT without signature verification.
// Grant JWTs are validated by the license server; the client trusts the TLS connection.
func parseGrantJWT(jwt string) (*GrantClaims, error) {
	parts := splitJWT(jwt)
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid grant JWT: expected 3 parts, got %d", len(parts))
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode grant payload: %w", err)
	}

	var gc GrantClaims
	if err := json.Unmarshal(payload, &gc); err != nil {
		return nil, fmt.Errorf("unmarshal grant claims: %w", err)
	}

	if gc.LicenseID == "" {
		return nil, fmt.Errorf("grant missing license ID")
	}
	if gc.Tier == "" {
		return nil, fmt.Errorf("grant missing tier")
	}

	return &gc, nil
}

// splitJWT splits a JWT string into its three parts.
func splitJWT(token string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(token); i++ {
		if token[i] == '.' {
			parts = append(parts, token[start:i])
			start = i + 1
		}
	}
	parts = append(parts, token[start:])
	return parts
}
