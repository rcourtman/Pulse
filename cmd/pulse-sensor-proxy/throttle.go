package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// peerID identifies a connecting principal (grouped by UID or ID range)
type peerID struct {
	uid      uint32
	uidRange *idRange
}

func (p peerID) String() string {
	if p.uidRange != nil {
		end := p.uidRange.start + p.uidRange.length - 1
		return fmt.Sprintf("range:%d-%d", p.uidRange.start, end)
	}
	return fmt.Sprintf("uid:%d", p.uid)
}

// limiterEntry holds rate limiting and concurrency controls for a peer
type limiterEntry struct {
	limiter   *rate.Limiter
	semaphore chan struct{}
	lastSeen  time.Time
}

type limiterPolicy struct {
	perPeerLimit       rate.Limit
	perPeerBurst       int
	perPeerConcurrency int
	globalConcurrency  int
	penaltyDuration    time.Duration
}

// rateLimiter manages per-peer rate limits and concurrency
type rateLimiter struct {
	mu        sync.Mutex
	entries   map[string]*limiterEntry
	quitChan  chan struct{}
	globalSem chan struct{}
	policy    limiterPolicy
	metrics   *ProxyMetrics
	uidRanges []idRange
	gidRanges []idRange
}

const (
	defaultPerPeerBurst       = 10 // Allow burst of 10 requests for multi-node polling with retries
	defaultPerPeerConcurrency = 2
	defaultGlobalConcurrency  = 8
)

var (
	defaultPerPeerRateInterval = 500 * time.Millisecond // 2 qps (120/min) - supports larger deployments
	defaultPenaltyDuration     = 2 * time.Second
	defaultPerPeerLimit        = rate.Every(defaultPerPeerRateInterval)
)

// newRateLimiter creates a new rate limiter with cleanup loop
// If rateLimitCfg is provided, it overrides the default rate limit settings
func newRateLimiter(metrics *ProxyMetrics, rateLimitCfg *RateLimitConfig, uidRanges, gidRanges []idRange) *rateLimiter {
	// Use defaults
	perPeerLimit := defaultPerPeerLimit
	perPeerBurst := defaultPerPeerBurst

	// Override with config if provided
	if rateLimitCfg != nil {
		if rateLimitCfg.PerPeerIntervalMs > 0 {
			interval := time.Duration(rateLimitCfg.PerPeerIntervalMs) * time.Millisecond
			perPeerLimit = rate.Every(interval)
		}
		if rateLimitCfg.PerPeerBurst > 0 {
			perPeerBurst = rateLimitCfg.PerPeerBurst
		}
	}

	rl := &rateLimiter{
		entries:   make(map[string]*limiterEntry),
		quitChan:  make(chan struct{}),
		globalSem: make(chan struct{}, defaultGlobalConcurrency),
		policy: limiterPolicy{
			perPeerLimit:       perPeerLimit,
			perPeerBurst:       perPeerBurst,
			perPeerConcurrency: defaultPerPeerConcurrency,
			globalConcurrency:  defaultGlobalConcurrency,
			penaltyDuration:    defaultPenaltyDuration,
		},
		metrics:   metrics,
		uidRanges: append([]idRange(nil), uidRanges...),
		gidRanges: append([]idRange(nil), gidRanges...),
	}
	if rl.metrics != nil {
		rl.metrics.setLimiterPeers(0)
	}
	go rl.cleanupLoop()
	return rl
}

// allow checks if a peer is allowed to make a request and reserves concurrency.
// Returns a release function, rejection reason (if any), and whether the request is allowed.
func (rl *rateLimiter) allow(id peerID) (release func(), reason string, allowed bool) {
	key := id.String()
	rl.mu.Lock()
	entry := rl.entries[key]
	if entry == nil {
		entry = &limiterEntry{
			limiter:   rate.NewLimiter(rl.policy.perPeerLimit, rl.policy.perPeerBurst),
			semaphore: make(chan struct{}, rl.policy.perPeerConcurrency),
		}
		rl.entries[key] = entry
		if rl.metrics != nil {
			rl.metrics.setLimiterPeers(len(rl.entries))
		}
	}
	entry.lastSeen = time.Now()
	rl.mu.Unlock()

	// Check rate limit
	if !entry.limiter.Allow() {
		rl.recordRejection("rate", key)
		return nil, "rate", false
	}

	// Acquire global concurrency
	select {
	case rl.globalSem <- struct{}{}:
		if rl.metrics != nil {
			rl.metrics.incGlobalConcurrency()
		}
	default:
		rl.recordRejection("global_concurrency", key)
		return nil, "global_concurrency", false
	}

	// Try to acquire per-peer concurrency slot
	select {
	case entry.semaphore <- struct{}{}:
		return func() {
			<-entry.semaphore
			<-rl.globalSem
			if rl.metrics != nil {
				rl.metrics.decGlobalConcurrency()
			}
		}, "", true
	default:
		<-rl.globalSem
		if rl.metrics != nil {
			rl.metrics.decGlobalConcurrency()
		}
		rl.recordRejection("peer_concurrency", key)
		return nil, "peer_concurrency", false
	}
}

// cleanupLoop periodically removes idle peer entries
func (rl *rateLimiter) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			rl.mu.Lock()
			for key, entry := range rl.entries {
				if time.Since(entry.lastSeen) > 10*time.Minute {
					delete(rl.entries, key)
				}
			}
			if rl.metrics != nil {
				rl.metrics.setLimiterPeers(len(rl.entries))
			}
			rl.mu.Unlock()
		case <-rl.quitChan:
			return
		}
	}
}

// shutdown stops the cleanup loop
func (rl *rateLimiter) shutdown() {
	close(rl.quitChan)
}

func (rl *rateLimiter) penalize(peerLabel, reason string) {
	if rl.policy.penaltyDuration <= 0 {
		return
	}
	time.Sleep(rl.policy.penaltyDuration)
	if rl.metrics != nil {
		rl.metrics.recordPenalty(reason, peerLabel)
	}
}

func (rl *rateLimiter) recordRejection(reason, peerLabel string) {
	if rl.metrics != nil {
		rl.metrics.recordLimiterReject(reason, peerLabel)
	}
}

func (rl *rateLimiter) identifyPeer(cred *peerCredentials) peerID {
	if cred == nil {
		return peerID{}
	}
	if rl == nil {
		return peerID{uid: cred.uid}
	}

	if len(rl.uidRanges) == 0 || len(rl.gidRanges) == 0 {
		return peerID{uid: cred.uid}
	}

	uidRange := findRange(rl.uidRanges, cred.uid)
	gidRange := findRange(rl.gidRanges, cred.gid)

	if uidRange != nil && gidRange != nil {
		return peerID{uid: cred.uid, uidRange: uidRange}
	}

	return peerID{uid: cred.uid}
}

func findRange(ranges []idRange, value uint32) *idRange {
	for i := range ranges {
		if ranges[i].contains(value) {
			return &ranges[i]
		}
	}
	return nil
}

// nodeGate controls per-node concurrency for temperature requests
type nodeGate struct {
	mu       sync.Mutex
	inFlight map[string]*nodeLock
}

// nodeLock tracks in-flight requests for a specific node
type nodeLock struct {
	refCount int
	guard    chan struct{}
}

// newNodeGate creates a new node concurrency gate
func newNodeGate() *nodeGate {
	return &nodeGate{
		inFlight: make(map[string]*nodeLock),
	}
}

// acquire gets exclusive access to make requests to a node
// Returns a release function that must be called when done
// Deprecated: Use acquireContext for context-aware acquisition
func (g *nodeGate) acquire(node string) func() {
	g.mu.Lock()
	lock := g.inFlight[node]
	if lock == nil {
		lock = &nodeLock{
			guard: make(chan struct{}, 1), // single slot = only one SSH fetch per node
		}
		g.inFlight[node] = lock
	}
	lock.refCount++
	g.mu.Unlock()

	// Wait for exclusive access
	lock.guard <- struct{}{}

	// Return release function
	return func() {
		<-lock.guard
		g.mu.Lock()
		lock.refCount--
		if lock.refCount == 0 {
			delete(g.inFlight, node)
		}
		g.mu.Unlock()
	}
}

// acquireContext gets exclusive access to make requests to a node with context cancellation support.
// Returns a release function and nil error on success.
// Returns nil and context error if context is cancelled while waiting.
func (g *nodeGate) acquireContext(ctx context.Context, node string) (func(), error) {
	g.mu.Lock()
	lock := g.inFlight[node]
	if lock == nil {
		lock = &nodeLock{
			guard: make(chan struct{}, 1),
		}
		g.inFlight[node] = lock
	}
	lock.refCount++
	g.mu.Unlock()

	// Wait for exclusive access OR context cancellation
	select {
	case lock.guard <- struct{}{}:
		return func() {
			<-lock.guard
			g.mu.Lock()
			lock.refCount--
			if lock.refCount == 0 {
				delete(g.inFlight, node)
			}
			g.mu.Unlock()
		}, nil
	case <-ctx.Done():
		// Clean up refCount since we're not proceeding
		g.mu.Lock()
		lock.refCount--
		if lock.refCount == 0 {
			delete(g.inFlight, node)
		}
		g.mu.Unlock()
		return nil, ctx.Err()
	}
}
