package revocation

import (
	"sync"
	"time"
)

// DefaultStaleTTL is the duration after which a CRL cache is considered stale.
// Per B6: bounded staleness of 72 hours.
const DefaultStaleTTL = 72 * time.Hour

// CRLCache maintains a local cache of revoked license IDs.
// When the cache is stale (older than StaleTTL), it fails open (allows access)
// to prevent blocking critical monitoring operations.
type CRLCache struct {
	mu          sync.RWMutex
	revokedIDs  map[string]bool
	lastUpdated time.Time
	staleTTL    time.Duration
}

// NewCRLCache creates a new CRL cache with the given staleness TTL.
// If staleTTL is 0, uses DefaultStaleTTL.
func NewCRLCache(staleTTL time.Duration) *CRLCache {
	if staleTTL == 0 {
		staleTTL = DefaultStaleTTL
	}

	return &CRLCache{
		revokedIDs: make(map[string]bool),
		staleTTL:   staleTTL,
	}
}

// IsRevoked checks if a license ID is in the revocation list.
// Returns (revoked, stale):
//   - revoked=true if the ID is revoked AND cache is fresh
//   - stale=true if the cache is older than staleTTL
//   - If cache is stale, always returns revoked=false (fail-open)
//   - If cache has never been updated, returns revoked=false, stale=true
func (c *CRLCache) IsRevoked(licenseID string) (revoked bool, stale bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.isStaleLocked(time.Now()) {
		return false, true
	}

	return c.revokedIDs[licenseID], false
}

// Update replaces the cache contents with the given revoked IDs and resets the timestamp.
func (c *CRLCache) Update(revokedIDs []string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	next := make(map[string]bool, len(revokedIDs))
	for _, revokedID := range revokedIDs {
		next[revokedID] = true
	}

	c.revokedIDs = next
	c.lastUpdated = time.Now()
}

// LastUpdated returns when the cache was last updated.
func (c *CRLCache) LastUpdated() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastUpdated
}

// IsStale returns true if the cache is older than the staleness TTL.
func (c *CRLCache) IsStale() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.isStaleLocked(time.Now())
}

func (c *CRLCache) isStaleLocked(now time.Time) bool {
	if c.lastUpdated.IsZero() {
		return true
	}
	return now.Sub(c.lastUpdated) > c.staleTTL
}

// Size returns the number of entries in the cache.
func (c *CRLCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.revokedIDs)
}
