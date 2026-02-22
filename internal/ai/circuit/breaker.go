// Package circuit provides circuit breaker functionality for AI patrol operations.
// It prevents cascade failures by temporarily disabling operations after repeated failures.
package circuit

import (
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// State represents the circuit breaker state
type State int

const (
	// StateClosed means the circuit is operating normally
	StateClosed State = iota
	// StateOpen means the circuit is tripped and operations are blocked
	StateOpen
	// StateHalfOpen means the circuit is testing if the service has recovered
	StateHalfOpen
)

// String returns the state as a string
func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// ErrorCategory categorizes different error types for appropriate handling
type ErrorCategory int

const (
	// ErrorCategoryTransient indicates a temporary error that should trigger backoff
	ErrorCategoryTransient ErrorCategory = iota
	// ErrorCategoryRateLimit indicates rate limiting - respect Retry-After header
	ErrorCategoryRateLimit
	// ErrorCategoryInvalid indicates an invalid request that won't succeed on retry
	ErrorCategoryInvalid
	// ErrorCategoryFatal indicates a fatal error that requires user intervention
	ErrorCategoryFatal
)

// Config configures the circuit breaker behavior
type Config struct {
	// FailureThreshold is the number of consecutive failures before opening
	FailureThreshold int
	// SuccessThreshold is the number of successes needed to close from half-open
	SuccessThreshold int
	// InitialBackoff is the initial backoff duration after opening
	InitialBackoff time.Duration
	// MaxBackoff is the maximum backoff duration
	MaxBackoff time.Duration
	// BackoffMultiplier is the factor to multiply backoff by after each failure
	BackoffMultiplier float64
	// HalfOpenTimeout is how long to wait before testing in half-open state
	HalfOpenTimeout time.Duration
}

// DefaultConfig returns sensible default configuration
func DefaultConfig() Config {
	return Config{
		FailureThreshold:  3,
		SuccessThreshold:  2,
		InitialBackoff:    time.Second,
		MaxBackoff:        5 * time.Minute,
		BackoffMultiplier: 2.0,
		HalfOpenTimeout:   30 * time.Second,
	}
}

// Breaker implements the circuit breaker pattern
type Breaker struct {
	mu sync.RWMutex

	config Config
	state  State
	name   string

	// Failure tracking
	consecutiveFailures  int
	consecutiveSuccesses int
	lastFailure          time.Time
	lastSuccess          time.Time
	lastError            error

	// Backoff tracking
	currentBackoff        time.Duration
	openedAt              time.Time
	halfOpenProbeInFlight bool

	// Statistics
	totalFailures  int64
	totalSuccesses int64
	totalTrips     int64

	// Callbacks
	onStateChange func(from, to State)
	onTrip        func(err error)
}

// NewBreaker creates a new circuit breaker with the given configuration
func NewBreaker(name string, config Config) *Breaker {
	if config.FailureThreshold <= 0 {
		config.FailureThreshold = 3
	}
	if config.SuccessThreshold <= 0 {
		config.SuccessThreshold = 2
	}
	if config.InitialBackoff <= 0 {
		config.InitialBackoff = time.Second
	}
	if config.MaxBackoff <= 0 {
		config.MaxBackoff = 5 * time.Minute
	}
	if config.BackoffMultiplier <= 0 {
		config.BackoffMultiplier = 2.0
	}
	if config.HalfOpenTimeout <= 0 {
		config.HalfOpenTimeout = 30 * time.Second
	}

	return &Breaker{
		config:         config,
		state:          StateClosed,
		name:           name,
		currentBackoff: config.InitialBackoff,
	}
}

// SetOnStateChange sets a callback for state changes
func (b *Breaker) SetOnStateChange(fn func(from, to State)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.onStateChange = fn
}

// SetOnTrip sets a callback for when the circuit trips
func (b *Breaker) SetOnTrip(fn func(err error)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.onTrip = fn
}

// CanAllow checks if an operation would be allowed without causing state transitions.
// Use this for read-only status checks. For actual operations, use Allow().
func (b *Breaker) CanAllow() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()

	switch b.state {
	case StateClosed:
		return true
	case StateOpen:
		// Check if backoff period has elapsed (but don't transition)
		return time.Since(b.openedAt) >= b.currentBackoff
	case StateHalfOpen:
		return !b.halfOpenProbeInFlight
	default:
		return true
	}
}

// Allow checks if an operation should be allowed
// Returns true if the operation can proceed, false if it should be blocked
// Note: This may cause state transitions (open → half-open), so use CanAllow() for read-only checks
func (b *Breaker) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case StateClosed:
		return true

	case StateOpen:
		// Check if backoff period has elapsed
		if time.Since(b.openedAt) >= b.currentBackoff {
			b.transitionTo(StateHalfOpen)
			b.halfOpenProbeInFlight = true
			log.Info().
				Str("breaker", b.name).
				Str("state", "half-open").
				Msg("Circuit breaker transitioning to half-open for test")
			return true
		}
		return false

	case StateHalfOpen:
		// Allow one test operation
		if b.halfOpenProbeInFlight {
			return false
		}
		b.halfOpenProbeInFlight = true
		return true

	default:
		return true
	}
}

// RecordSuccess records a successful operation
func (b *Breaker) RecordSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.lastSuccess = time.Now()
	b.consecutiveFailures = 0
	b.consecutiveSuccesses++
	b.totalSuccesses++

	switch b.state {
	case StateHalfOpen:
		b.halfOpenProbeInFlight = false
		if b.consecutiveSuccesses >= b.config.SuccessThreshold {
			b.transitionTo(StateClosed)
			b.currentBackoff = b.config.InitialBackoff // Reset backoff
			log.Info().
				Str("breaker", b.name).
				Str("state", "closed").
				Msg("Circuit breaker recovered and closed")
		}

	case StateClosed:
		// Already closed, nothing special to do
	}
}

// RecordFailure records a failed operation
func (b *Breaker) RecordFailure(err error) {
	b.RecordFailureWithCategory(err, ErrorCategoryTransient)
}

// RecordFailureWithCategory records a failed operation with error categorization
func (b *Breaker) RecordFailureWithCategory(err error, category ErrorCategory) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.lastFailure = time.Now()
	b.lastError = err
	b.consecutiveSuccesses = 0
	b.totalFailures++

	// Handle different error categories
	switch category {
	case ErrorCategoryInvalid, ErrorCategoryFatal:
		// Don't trip on invalid/fatal errors - these won't be fixed by waiting.
		// Don't increment consecutiveFailures so a subsequent transient error
		// isn't closer to tripping than it should be.
		if b.state == StateHalfOpen {
			b.halfOpenProbeInFlight = false
		}
		log.Warn().
			Str("breaker", b.name).
			Err(err).
			Str("category", "non-transient").
			Msg("Circuit breaker ignoring non-transient error")
		return

	case ErrorCategoryRateLimit:
		// Rate limit errors should trip immediately with appropriate backoff
		b.consecutiveFailures = b.config.FailureThreshold
		// Fall through to trip logic below

	default:
		b.consecutiveFailures++
	}

	switch b.state {
	case StateClosed:
		if b.consecutiveFailures >= b.config.FailureThreshold {
			b.tripCircuit(err)
		}

	case StateHalfOpen:
		b.halfOpenProbeInFlight = false
		// Single failure in half-open returns to open with increased backoff
		b.currentBackoff = time.Duration(float64(b.currentBackoff) * b.config.BackoffMultiplier)
		if b.currentBackoff > b.config.MaxBackoff {
			b.currentBackoff = b.config.MaxBackoff
		}
		b.tripCircuit(err)
	}
}

// tripCircuit opens the circuit breaker
func (b *Breaker) tripCircuit(err error) {
	b.transitionTo(StateOpen)
	b.openedAt = time.Now()
	b.halfOpenProbeInFlight = false
	b.totalTrips++

	log.Warn().
		Str("breaker", b.name).
		Dur("backoff", b.currentBackoff).
		Int("failures", b.consecutiveFailures).
		Err(err).
		Msg("Circuit breaker tripped")

	// Call trip callback
	if b.onTrip != nil {
		go b.onTrip(err)
	}
}

// transitionTo changes the circuit breaker state
func (b *Breaker) transitionTo(newState State) {
	if b.state == newState {
		return
	}

	oldState := b.state
	b.state = newState

	if b.onStateChange != nil {
		go b.onStateChange(oldState, newState)
	}
}

// Reset resets the circuit breaker to closed state
func (b *Breaker) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.transitionTo(StateClosed)
	b.consecutiveFailures = 0
	b.consecutiveSuccesses = 0
	b.currentBackoff = b.config.InitialBackoff
	b.lastError = nil
	b.halfOpenProbeInFlight = false

	log.Info().
		Str("breaker", b.name).
		Msg("Circuit breaker reset")
}

// State returns the current state of the circuit breaker
func (b *Breaker) State() State {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.state
}

// Status returns a summary of the circuit breaker's current status
type Status struct {
	Name                 string        `json:"name"`
	State                string        `json:"state"`
	ConsecutiveFailures  int           `json:"consecutive_failures"`
	ConsecutiveSuccesses int           `json:"consecutive_successes"`
	LastFailure          *time.Time    `json:"last_failure,omitempty"`
	LastSuccess          *time.Time    `json:"last_success,omitempty"`
	LastError            string        `json:"last_error,omitempty"`
	CurrentBackoff       time.Duration `json:"current_backoff_ms"`
	TotalFailures        int64         `json:"total_failures"`
	TotalSuccesses       int64         `json:"total_successes"`
	TotalTrips           int64         `json:"total_trips"`
	TimeUntilRetry       time.Duration `json:"time_until_retry_ms,omitempty"`
}

// GetStatus returns the current status of the circuit breaker
func (b *Breaker) GetStatus() Status {
	b.mu.RLock()
	defer b.mu.RUnlock()

	status := Status{
		Name:                 b.name,
		State:                b.state.String(),
		ConsecutiveFailures:  b.consecutiveFailures,
		ConsecutiveSuccesses: b.consecutiveSuccesses,
		CurrentBackoff:       b.currentBackoff,
		TotalFailures:        b.totalFailures,
		TotalSuccesses:       b.totalSuccesses,
		TotalTrips:           b.totalTrips,
	}

	if !b.lastFailure.IsZero() {
		status.LastFailure = &b.lastFailure
	}
	if !b.lastSuccess.IsZero() {
		status.LastSuccess = &b.lastSuccess
	}
	if b.lastError != nil {
		status.LastError = b.lastError.Error()
	}

	// Calculate time until retry if circuit is open
	if b.state == StateOpen {
		retryIn := b.currentBackoff - time.Since(b.openedAt)
		if retryIn > 0 {
			status.TimeUntilRetry = retryIn
		}
	}

	return status
}

// IsOpen returns true if the circuit is open (blocking operations)
func (b *Breaker) IsOpen() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.state == StateOpen
}

// IsClosed returns true if the circuit is closed (allowing operations)
func (b *Breaker) IsClosed() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.state == StateClosed
}

// Execute wraps an operation with circuit breaker logic
// Returns error if the circuit is open or if the operation fails
func (b *Breaker) Execute(operation func() error) error {
	if !b.Allow() {
		return ErrCircuitOpen
	}

	err := operation()
	if err != nil {
		b.RecordFailure(err)
		return err
	}

	b.RecordSuccess()
	return nil
}

// ExecuteWithCategory wraps an operation with circuit breaker logic and error categorization
func (b *Breaker) ExecuteWithCategory(operation func() (error, ErrorCategory)) error {
	if !b.Allow() {
		return ErrCircuitOpen
	}

	err, category := operation()
	if err != nil {
		b.RecordFailureWithCategory(err, category)
		return err
	}

	b.RecordSuccess()
	return nil
}

// circuitOpenError is the error type returned when an operation is blocked by an open circuit
type circuitOpenError struct{}

func (e circuitOpenError) Error() string {
	return "circuit breaker is open"
}

// ErrCircuitOpen is returned when an operation is blocked by an open circuit
var ErrCircuitOpen error = circuitOpenError{}

// IsCircuitOpen checks if an error is a circuit open error
func IsCircuitOpen(err error) bool {
	_, ok := err.(circuitOpenError)
	return ok
}

// CategorizeError categorizes an error for circuit breaker handling
func CategorizeError(err error) ErrorCategory {
	if err == nil {
		return ErrorCategoryTransient
	}

	errStr := err.Error()

	// Rate limit errors
	if contains(errStr, "rate limit", "429", "too many requests", "quota exceeded") {
		return ErrorCategoryRateLimit
	}

	// Invalid request errors (won't succeed on retry)
	if contains(errStr, "400", "bad request", "invalid", "malformed") {
		return ErrorCategoryInvalid
	}

	// Authentication/authorization errors (require user intervention)
	if contains(errStr, "401", "403", "unauthorized", "forbidden", "api key") {
		return ErrorCategoryFatal
	}

	// Insufficient credits (require user intervention)
	if contains(errStr, "402", "insufficient balance", "payment required", "credit") {
		return ErrorCategoryFatal
	}

	// Default to transient (retryable)
	return ErrorCategoryTransient
}

// contains checks if the error string contains any of the substrings (case-insensitive)
func contains(errStr string, substrings ...string) bool {
	lowerErr := toLower(errStr)
	for _, sub := range substrings {
		if containsSubstring(lowerErr, toLower(sub)) {
			return true
		}
	}
	return false
}

func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c = c + 32
		}
		result[i] = c
	}
	return string(result)
}

func containsSubstring(s, sub string) bool {
	if len(sub) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
