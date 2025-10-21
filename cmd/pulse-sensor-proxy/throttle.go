package main

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// peerID identifies a connecting principal (grouped by UID)
type peerID struct {
	uid uint32
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
	entries   map[peerID]*limiterEntry
	quitChan  chan struct{}
	globalSem chan struct{}
	policy    limiterPolicy
	metrics   *ProxyMetrics
}

const (
	defaultPerPeerBurst       = 5  // Allow burst of 5 requests for multi-node polling
	defaultPerPeerConcurrency = 2
	defaultGlobalConcurrency  = 8
)

var (
	defaultPerPeerRateInterval = 1 * time.Second // 1 qps (60/min) - supports 5-10 node deployments
	defaultPenaltyDuration     = 2 * time.Second
	defaultPerPeerLimit        = rate.Every(defaultPerPeerRateInterval)
)

// newRateLimiter creates a new rate limiter with cleanup loop
func newRateLimiter(metrics *ProxyMetrics) *rateLimiter {
	rl := &rateLimiter{
		entries:   make(map[peerID]*limiterEntry),
		quitChan:  make(chan struct{}),
		globalSem: make(chan struct{}, defaultGlobalConcurrency),
		policy: limiterPolicy{
			perPeerLimit:       defaultPerPeerLimit,
			perPeerBurst:       defaultPerPeerBurst,
			perPeerConcurrency: defaultPerPeerConcurrency,
			globalConcurrency:  defaultGlobalConcurrency,
			penaltyDuration:    defaultPenaltyDuration,
		},
		metrics: metrics,
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
	rl.mu.Lock()
	entry := rl.entries[id]
	if entry == nil {
		entry = &limiterEntry{
			limiter:   rate.NewLimiter(rl.policy.perPeerLimit, rl.policy.perPeerBurst),
			semaphore: make(chan struct{}, rl.policy.perPeerConcurrency),
		}
		rl.entries[id] = entry
		if rl.metrics != nil {
			rl.metrics.setLimiterPeers(len(rl.entries))
		}
	}
	entry.lastSeen = time.Now()
	rl.mu.Unlock()

	// Check rate limit
	if !entry.limiter.Allow() {
		rl.recordRejection("rate")
		return nil, "rate", false
	}

	// Acquire global concurrency
	select {
	case rl.globalSem <- struct{}{}:
		if rl.metrics != nil {
			rl.metrics.incGlobalConcurrency()
		}
	default:
		rl.recordRejection("global_concurrency")
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
		rl.recordRejection("peer_concurrency")
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
			for id, entry := range rl.entries {
				if time.Since(entry.lastSeen) > 10*time.Minute {
					delete(rl.entries, id)
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

func (rl *rateLimiter) penalize(id peerID, reason string) {
	if rl.policy.penaltyDuration <= 0 {
		return
	}
	time.Sleep(rl.policy.penaltyDuration)
	if rl.metrics != nil {
		rl.metrics.recordPenalty(reason)
	}
}

func (rl *rateLimiter) recordRejection(reason string) {
	if rl.metrics != nil {
		rl.metrics.recordLimiterReject(reason)
	}
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
