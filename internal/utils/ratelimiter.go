package utils

import (
	"context"
	"sync"
	"time"
)

// RateLimiter is a generic rate limiter using channels
type RateLimiter[T any] struct {
	rate     int           // requests per interval
	interval time.Duration // time interval
	tokens   chan struct{} // token bucket
	stopCh   chan struct{}
	once     sync.Once
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter[T any](rate int, interval time.Duration) *RateLimiter[T] {
	rl := &RateLimiter[T]{
		rate:     rate,
		interval: interval,
		tokens:   make(chan struct{}, rate),
		stopCh:   make(chan struct{}),
	}

	// Fill initial tokens
	for i := 0; i < rate; i++ {
		rl.tokens <- struct{}{}
	}

	// Start token refill goroutine
	go rl.refillTokens()

	return rl
}

// refillTokens periodically adds tokens to the bucket
func (rl *RateLimiter[T]) refillTokens() {
	ticker := time.NewTicker(rl.interval / time.Duration(rl.rate))
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			select {
			case rl.tokens <- struct{}{}:
				// Token added successfully
			default:
				// Bucket is full
			}
		case <-rl.stopCh:
			return
		}
	}
}

// Wait blocks until a token is available
func (rl *RateLimiter[T]) Wait(ctx context.Context) error {
	select {
	case <-rl.tokens:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// TryAcquire attempts to acquire a token without blocking
func (rl *RateLimiter[T]) TryAcquire() bool {
	select {
	case <-rl.tokens:
		return true
	default:
		return false
	}
}

// Close stops the rate limiter
func (rl *RateLimiter[T]) Close() {
	rl.once.Do(func() {
		close(rl.stopCh)
	})
}

// RateLimitedProcessor processes items with rate limiting
type RateLimitedProcessor[T any, R any] struct {
	limiter   *RateLimiter[T]
	processor func(context.Context, T) (R, error)
}

// NewRateLimitedProcessor creates a new rate limited processor
func NewRateLimitedProcessor[T any, R any](rate int, interval time.Duration, processor func(context.Context, T) (R, error)) *RateLimitedProcessor[T, R] {
	return &RateLimitedProcessor[T, R]{
		limiter:   NewRateLimiter[T](rate, interval),
		processor: processor,
	}
}

// Process processes an item with rate limiting
func (rlp *RateLimitedProcessor[T, R]) Process(ctx context.Context, item T) (R, error) {
	if err := rlp.limiter.Wait(ctx); err != nil {
		var zero R
		return zero, err
	}
	return rlp.processor(ctx, item)
}

// ProcessBatch processes multiple items with rate limiting
func (rlp *RateLimitedProcessor[T, R]) ProcessBatch(ctx context.Context, items []T) []Result[R] {
	results := make([]Result[R], len(items))

	for i, item := range items {
		r, err := rlp.Process(ctx, item)
		if err != nil {
			results[i] = Err[R](err)
		} else {
			results[i] = Ok(r)
		}
	}

	return results
}

// Close closes the processor
func (rlp *RateLimitedProcessor[T, R]) Close() {
	rlp.limiter.Close()
}

// Pipeline represents a generic processing pipeline
type Pipeline[T any, R any] struct {
	stages []func(T) Result[T]
	final  func(T) Result[R]
}

// NewPipeline creates a new processing pipeline
func NewPipeline[T any, R any]() *Pipeline[T, R] {
	return &Pipeline[T, R]{
		stages: make([]func(T) Result[T], 0),
	}
}

// AddStage adds a processing stage to the pipeline
func (p *Pipeline[T, R]) AddStage(stage func(T) Result[T]) *Pipeline[T, R] {
	p.stages = append(p.stages, stage)
	return p
}

// SetFinal sets the final transformation stage
func (p *Pipeline[T, R]) SetFinal(final func(T) Result[R]) *Pipeline[T, R] {
	p.final = final
	return p
}

// Process runs an item through the pipeline
func (p *Pipeline[T, R]) Process(input T) Result[R] {
	current := Ok(input)

	// Process through each stage
	for _, stage := range p.stages {
		if current.IsErr() {
			break
		}
		current = stage(current.Unwrap())
	}

	// If any stage failed, return error
	if current.IsErr() {
		var zero R
		return Result[R]{value: zero, err: current.err}
	}

	// Apply final transformation
	if p.final != nil {
		return p.final(current.Unwrap())
	}

	// If no final transformation, return error
	// In a real implementation, we'd need type constraints to handle T->R conversion
	return Err[R](nil)
}

// ProcessAll processes multiple items through the pipeline
func (p *Pipeline[T, R]) ProcessAll(inputs []T) []Result[R] {
	return SliceMap(inputs, p.Process)
}

// Throttle is a generic throttler using channels
type Throttle[T any] struct {
	input  chan T
	output chan T
	delay  time.Duration
}

// NewThrottle creates a new throttler
func NewThrottle[T any](bufferSize int, delay time.Duration) *Throttle[T] {
	t := &Throttle[T]{
		input:  make(chan T, bufferSize),
		output: make(chan T, bufferSize),
		delay:  delay,
	}
	go t.run()
	return t
}

// run processes items with throttling
func (t *Throttle[T]) run() {
	ticker := time.NewTicker(t.delay)
	defer ticker.Stop()

	for {
		select {
		case item, ok := <-t.input:
			if !ok {
				close(t.output)
				return
			}
			<-ticker.C
			t.output <- item
		}
	}
}

// Send sends an item to be throttled
func (t *Throttle[T]) Send(item T) {
	t.input <- item
}

// Receive receives a throttled item
func (t *Throttle[T]) Receive() <-chan T {
	return t.output
}

// Close closes the throttle
func (t *Throttle[T]) Close() {
	close(t.input)
}
