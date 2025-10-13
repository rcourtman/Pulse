package main

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// peerID identifies a connecting process by UID+PID
type peerID struct {
	uid uint32
	pid uint32
}

// limiterEntry holds rate limiting and concurrency controls for a peer
type limiterEntry struct {
	limiter   *rate.Limiter // throughput: 20/min with burst 10
	semaphore chan struct{} // concurrency: cap 10
	lastSeen  time.Time
}

// rateLimiter manages per-peer rate limits and concurrency
type rateLimiter struct {
	mu       sync.Mutex
	entries  map[peerID]*limiterEntry
	quitChan chan struct{}
}

// newRateLimiter creates a new rate limiter with cleanup loop
func newRateLimiter() *rateLimiter {
	rl := &rateLimiter{
		entries:  make(map[peerID]*limiterEntry),
		quitChan: make(chan struct{}),
	}
	go rl.cleanupLoop()
	return rl
}

// allow checks if a peer is allowed to make a request and reserves a concurrency slot
// Returns a release function and whether the request is allowed
func (rl *rateLimiter) allow(id peerID) (release func(), allowed bool) {
	rl.mu.Lock()
	entry := rl.entries[id]
	if entry == nil {
		entry = &limiterEntry{
			limiter:   rate.NewLimiter(rate.Every(time.Minute/20), 10), // 20/min, burst 10
			semaphore: make(chan struct{}, 10),                          // max 10 concurrent
		}
		rl.entries[id] = entry
	}
	entry.lastSeen = time.Now()
	rl.mu.Unlock()

	// Check rate limit
	if !entry.limiter.Allow() {
		return nil, false
	}

	// Try to acquire concurrency slot
	select {
	case entry.semaphore <- struct{}{}:
		return func() { <-entry.semaphore }, true
	default:
		return nil, false // max concurrent in-flight reached
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
