package chartapi

import (
	"sync"
	"time"
)

type cachedChartPayload struct {
	payload   []byte
	expiresAt time.Time
	sequence  uint64
}

// boundedChartPayloadCache keeps the short-lived chart response caches from
// retaining an unbounded set of expired query variants. Chart payloads can be
// several MiB on large estates, so both cardinality and bytes are bounded.
type boundedChartPayloadCache struct {
	mu         sync.Mutex
	entries    map[string]cachedChartPayload
	maxEntries int
	maxBytes   int
	bytes      int
	sequence   uint64
}

func newBoundedChartPayloadCache(maxEntries, maxBytes int) boundedChartPayloadCache {
	if maxEntries < 0 {
		maxEntries = 0
	}
	if maxBytes < 0 {
		maxBytes = 0
	}
	return boundedChartPayloadCache{
		entries:    make(map[string]cachedChartPayload, maxEntries),
		maxEntries: maxEntries,
		maxBytes:   maxBytes,
	}
}

func (c *boundedChartPayloadCache) get(key string, now time.Time) ([]byte, bool) {
	if c == nil || key == "" {
		return nil, false
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	if !now.Before(entry.expiresAt) {
		c.removeLocked(key, entry)
		return nil, false
	}
	return entry.payload, true
}

func (c *boundedChartPayloadCache) put(key string, payload []byte, expiresAt, now time.Time) {
	if c == nil || key == "" || len(payload) == 0 {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil {
		c.entries = make(map[string]cachedChartPayload, c.maxEntries)
	}
	c.pruneExpiredLocked(now)
	if previous, ok := c.entries[key]; ok {
		c.removeLocked(key, previous)
	}
	if c.maxEntries == 0 || c.maxBytes == 0 || len(payload) > c.maxBytes {
		return
	}

	for len(c.entries) >= c.maxEntries || c.bytes+len(payload) > c.maxBytes {
		if !c.removeOldestLocked() {
			return
		}
	}
	c.sequence++
	c.entries[key] = cachedChartPayload{
		payload:   payload,
		expiresAt: expiresAt,
		sequence:  c.sequence,
	}
	c.bytes += len(payload)
}

func (c *boundedChartPayloadCache) pruneExpiredLocked(now time.Time) {
	for key, entry := range c.entries {
		if !now.Before(entry.expiresAt) {
			c.removeLocked(key, entry)
		}
	}
}

func (c *boundedChartPayloadCache) removeOldestLocked() bool {
	var (
		oldestKey   string
		oldestEntry cachedChartPayload
		found       bool
	)
	for key, entry := range c.entries {
		if !found || entry.sequence < oldestEntry.sequence {
			oldestKey = key
			oldestEntry = entry
			found = true
		}
	}
	if !found {
		return false
	}
	c.removeLocked(oldestKey, oldestEntry)
	return true
}

func (c *boundedChartPayloadCache) removeLocked(key string, entry cachedChartPayload) {
	delete(c.entries, key)
	c.bytes -= len(entry.payload)
	if c.bytes < 0 {
		c.bytes = 0
	}
}
