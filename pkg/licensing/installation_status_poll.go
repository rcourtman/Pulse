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
	defaultInstallationStatusInterval = 5 * time.Minute
	defaultInstallationStatusJitter   = 0.2
	minInstallationStatusInterval     = time.Minute
	maxInstallationStatusInterval     = 10 * time.Minute
	defaultInstallationStatusBackoff  = 30 * time.Second
	maxInstallationStatusBackoff      = 10 * time.Minute
	installationStatusStaleAfter      = 15 * time.Minute
)

var errNoActivationState = errors.New("no activation state")

// installationStatusPollLoop owns customer-safe commercial invalidation.
// It carries no operator credential and records only non-secret health state.
type installationStatusPollLoop struct {
	mu                  sync.Mutex
	cancel              context.CancelFunc
	wg                  sync.WaitGroup
	running             bool
	interval            time.Duration
	jitterPercent       float64
	retryBase           time.Duration
	retryMax            time.Duration
	lastAttemptAt       time.Time
	lastSuccessAt       time.Time
	nextCheckAt         time.Time
	consecutiveFailures int
}

func newInstallationStatusPollLoop() *installationStatusPollLoop {
	return &installationStatusPollLoop{
		interval:      defaultInstallationStatusInterval,
		jitterPercent: defaultInstallationStatusJitter,
		retryBase:     defaultInstallationStatusBackoff,
		retryMax:      maxInstallationStatusBackoff,
	}
}

// StartInstallationStatusPoll starts the installation-authenticated status
// loop. The first network check happens immediately in the goroutine before
// any timer wait. Repeated starts are idempotent.
func (s *Service) StartInstallationStatusPoll(ctx context.Context) {
	s.mu.Lock()
	if s.installationStatus == nil {
		s.installationStatus = newInstallationStatusPollLoop()
	}
	loop := s.installationStatus
	s.mu.Unlock()

	loop.mu.Lock()
	if loop.running {
		loop.mu.Unlock()
		return
	}
	pollCtx, cancel := context.WithCancel(ctx)
	loop.cancel = cancel
	loop.running = true
	loop.nextCheckAt = time.Now()
	loop.wg.Add(1)
	loop.mu.Unlock()

	go func() {
		defer func() {
			loop.mu.Lock()
			loop.running = false
			loop.nextCheckAt = time.Time{}
			loop.mu.Unlock()
			loop.wg.Done()
		}()
		s.runInstallationStatusLoop(pollCtx, loop)
	}()
	log.Info().Msg("Installation status poll loop started")
}

// StopInstallationStatusPoll stops the status loop and waits for exit.
func (s *Service) StopInstallationStatusPoll() {
	s.mu.RLock()
	loop := s.installationStatus
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
	log.Info().Msg("Installation status poll loop stopped")
}

func (s *Service) runInstallationStatusLoop(ctx context.Context, loop *installationStatusPollLoop) {
	for {
		loop.recordAttempt(time.Now())
		err := s.checkInstallationStatusOnce(ctx)
		if errors.Is(err, errNoActivationState) || errors.Is(err, context.Canceled) {
			return
		}
		if err != nil {
			if isRevokedActivationError(err) {
				loop.recordFailure()
				log.Warn().Msg("Installation status rejected as revoked, clearing activation")
				s.clearActivationState()
				return
			}
			if isSuspendedActivationError(err) {
				loop.recordSuccess(time.Now())
				log.Warn().Msg("Installation status reports suspended paid access, preserving activation for recovery")
				s.suspendPaidEntitlements()
			} else {
				loop.recordFailure()
				log.Warn().Err(err).Int("consecutive_failures", loop.failureCount()).Msg("Installation status check failed")
			}
		} else {
			loop.recordSuccess(time.Now())
		}

		delay := loop.nextDelay()
		loop.recordNextCheck(time.Now().Add(delay))
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return
		case <-timer.C:
		}
	}
}

func (s *Service) checkInstallationStatusOnce(ctx context.Context) error {
	s.mu.RLock()
	client := s.serverClient
	state := cloneActivationState(s.activationState)
	clientVersion := s.clientVersion
	runtimeIdentity := NormalizeRuntimeIdentity(s.runtimeIdentity)
	s.mu.RUnlock()
	if state == nil {
		return errNoActivationState
	}
	if client == nil {
		return fmt.Errorf("license server client not configured")
	}

	resp, err := client.CheckInstallationStatus(ctx, state.InstallationToken, InstallationStatusRequest{
		InstallationID:        state.InstallationID,
		InstanceFingerprint:   state.InstanceFingerprint,
		CurrentGrantJTI:       state.GrantJTI,
		CurrentLicenseVersion: state.LicenseVersion,
		ClientVersion:         clientVersion,
		Runtime:               CloneRuntimeIdentity(runtimeIdentity),
	})
	if err != nil {
		return fmt.Errorf("check installation status: %w", err)
	}
	s.SetInstallationStatusHints(resp.StatusPolicy)
	if !resp.RefreshRequired && resp.LicenseVersion <= state.LicenseVersion {
		return nil
	}
	if resp.LicenseVersion <= state.LicenseVersion {
		return fmt.Errorf("status requested refresh without a newer license version")
	}
	if err := s.refreshGrantOnce(ctx); err != nil {
		return fmt.Errorf("refresh changed grant: %w", err)
	}
	return nil
}

// SetInstallationStatusHints applies server hints within client-owned hard
// bounds so server drift cannot disable timely invalidation or create a tight
// request loop.
func (s *Service) SetInstallationStatusHints(hints StatusHints) {
	s.mu.Lock()
	if s.installationStatus == nil {
		s.installationStatus = newInstallationStatusPollLoop()
	}
	loop := s.installationStatus
	s.mu.Unlock()

	loop.mu.Lock()
	defer loop.mu.Unlock()
	if hints.IntervalSeconds > 0 {
		loop.interval = clampStatusDuration(time.Duration(hints.IntervalSeconds)*time.Second, minInstallationStatusInterval, maxInstallationStatusInterval)
	}
	if hints.JitterPercent > 0 || (hints.JitterPercent == 0 && hints.IntervalSeconds > 0) {
		jitter := hints.JitterPercent
		if jitter >= 1 {
			jitter /= 100
		}
		if jitter >= 0 && jitter <= 0.5 {
			loop.jitterPercent = jitter
		}
	}
	if hints.RetryBaseSeconds > 0 {
		loop.retryBase = clampStatusDuration(time.Duration(hints.RetryBaseSeconds)*time.Second, 5*time.Second, time.Minute)
	}
	if hints.RetryMaxSeconds > 0 {
		loop.retryMax = clampStatusDuration(time.Duration(hints.RetryMaxSeconds)*time.Second, loop.retryBase, maxInstallationStatusBackoff)
	}
}

func clampStatusDuration(value, minValue, maxValue time.Duration) time.Duration {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func (loop *installationStatusPollLoop) nextDelay() time.Duration {
	loop.mu.Lock()
	defer loop.mu.Unlock()
	if loop.consecutiveFailures > 0 {
		backoff := loop.retryBase * (1 << min(loop.consecutiveFailures-1, 10))
		if backoff > loop.retryMax {
			backoff = loop.retryMax
		}
		return backoff
	}
	jitterRange := float64(loop.interval) * loop.jitterPercent
	offset := (rand.Float64()*2 - 1) * jitterRange
	return loop.interval + time.Duration(offset)
}

func (loop *installationStatusPollLoop) recordAttempt(at time.Time) {
	loop.mu.Lock()
	loop.lastAttemptAt = at
	loop.nextCheckAt = time.Time{}
	loop.mu.Unlock()
}

func (loop *installationStatusPollLoop) recordFailure() {
	loop.mu.Lock()
	loop.consecutiveFailures++
	loop.mu.Unlock()
}

func (loop *installationStatusPollLoop) recordSuccess(at time.Time) {
	loop.mu.Lock()
	loop.lastSuccessAt = at
	loop.consecutiveFailures = 0
	loop.mu.Unlock()
}

func (loop *installationStatusPollLoop) recordNextCheck(at time.Time) {
	loop.mu.Lock()
	loop.nextCheckAt = at
	loop.mu.Unlock()
}

func (loop *installationStatusPollLoop) failureCount() int {
	loop.mu.Lock()
	defer loop.mu.Unlock()
	return loop.consecutiveFailures
}

func (s *Service) installationStatusSnapshotLocked(now time.Time) *LicenseSynchronizationStatus {
	if s.activationState == nil || s.installationStatus == nil {
		return nil
	}
	loop := s.installationStatus
	loop.mu.Lock()
	defer loop.mu.Unlock()
	stale := !loop.lastSuccessAt.IsZero() && now.Sub(loop.lastSuccessAt) > installationStatusStaleAfter
	status := &LicenseSynchronizationStatus{
		Running:             loop.running,
		Healthy:             !loop.lastSuccessAt.IsZero() && loop.consecutiveFailures == 0 && !stale,
		Stale:               stale,
		ConsecutiveFailures: loop.consecutiveFailures,
	}
	status.LastAttemptAt = formatStatusTime(loop.lastAttemptAt)
	status.LastSuccessAt = formatStatusTime(loop.lastSuccessAt)
	status.NextCheckAt = formatStatusTime(loop.nextCheckAt)
	return status
}

func formatStatusTime(value time.Time) *string {
	if value.IsZero() {
		return nil
	}
	formatted := value.UTC().Format(time.RFC3339)
	return &formatted
}
