package monitoring

import (
	"bytes"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type blockedCacheExpiryLog struct {
	message string
	entered chan struct{}
	resume  chan struct{}
}

func (w blockedCacheExpiryLog) Write(p []byte) (int, error) {
	if bytes.Contains(p, []byte(w.message)) {
		close(w.entered)
		<-w.resume
	}
	return len(p), nil
}

func TestCacheExpiryLoggingDoesNotHoldPollCacheLocks(t *testing.T) {
	now := time.Now()
	stale := now.Add(-2 * time.Hour)
	const deadKey = "expired"
	const instance, node, vmid = "live", "node", 105
	guestKey := guestMetadataCacheKey(instance, node, vmid)

	for _, kind := range []string{"metadata", "limiter", "node-snapshot", "guest-snapshot", "rrd"} {
		t.Run(kind, func(t *testing.T) {
			var message string
			var sweep func(time.Time)
			var checkLive func()
			m := &Monitor{}
			switch kind {
			case "metadata", "limiter":
				sweep = m.cleanupGuestMetadataCache
				m.guestMetadataCache = map[string]guestMetadataCacheEntry{guestKey: metadataObservationFixture(now)}
				if kind == "metadata" {
					m.guestMetadataCache[deadKey] = metadataObservationFixture(stale)
					message = "Cleaned up stale guest metadata cache entry"
				} else {
					m.guestMetadataLimiter = map[string]time.Time{deadKey: stale}
					message = "Cleaned up stale guest metadata limiter entry"
				}
				checkLive = func() {
					if !m.guestMetadataMu.TryRLock() {
						t.Error("expiry logging held the guest metadata cache lock")
					} else {
						_, retained := m.guestMetadataCache[deadKey]
						m.guestMetadataMu.RUnlock()
						if retained {
							t.Error("expired metadata was still present when its removal was logged")
						}
						if !m.hasRecentGuestMetadataEvidence(instance, node, vmid, now) {
							t.Error("live guest lost its metadata observation")
						}
					}
					if !m.guestMetadataLimiterMu.TryLock() {
						t.Fatal("expiry logging held the guest metadata limiter lock")
					}
					_, retained := m.guestMetadataLimiter[deadKey]
					m.guestMetadataLimiterMu.Unlock()
					if retained {
						t.Error("expired limiter was still present when its removal was logged")
					}
					if !m.tryReserveGuestMetadataFetch(guestKey, now) {
						t.Fatal("live guest could not reserve its next read")
					}
					m.releaseGuestMetadataFetch(guestKey)
				}
			case "node-snapshot", "guest-snapshot":
				sweep = m.cleanupDiagnosticSnapshots
				m.guestSnapshots = map[string]GuestMemorySnapshot{guestKey: {Instance: instance, RetrievedAt: now}}
				if kind == "node-snapshot" {
					m.nodeSnapshots = map[string]NodeMemorySnapshot{deadKey: {RetrievedAt: stale}}
					message = "Cleaned up stale node snapshot"
				} else {
					m.guestSnapshots[deadKey] = GuestMemorySnapshot{RetrievedAt: stale}
					message = "Cleaned up stale guest snapshot"
				}
				checkLive = func() {
					if !m.diagMu.TryRLock() {
						t.Fatal("expiry logging held the diagnostic snapshots lock")
					}
					m.diagMu.RUnlock()
					got := m.GetDiagnosticSnapshots()
					if len(got.Nodes) != 0 || len(got.Guests) != 1 || got.Guests[0].Instance != instance || !got.Guests[0].RetrievedAt.Equal(now) {
						t.Fatalf("diagnostic read did not preserve the live guest alone: %+v", got)
					}
				}
			case "rrd":
				sweep = m.cleanupRRDCache
				message = "Cleaned up stale RRD cache entry"
				m.nodeRRDMemCache = map[string]rrdMemCacheEntry{deadKey: {fetchedAt: stale}}
				m.guestAgentFSInfoCacheTTL = vmFSInfoCacheTTL
				m.storeVMFSInfo(instance, node, vmid, 100, nil, now)
				checkLive = func() {
					if !m.rrdCacheMu.TryLock() {
						t.Fatal("expiry logging held the guest memory and filesystem cache lock")
					}
					_, retained := m.nodeRRDMemCache[deadKey]
					m.rrdCacheMu.Unlock()
					if retained {
						t.Error("expired RRD sample was still present when its removal was logged")
					}
					if got, ok := m.cachedVMFSInfo(instance, node, vmid, 101, now); !ok || !got.fetchedAt.Equal(now) {
						t.Fatal("poll lost its independent filesystem observation or changed its age")
					}
				}
			}

			// No parallel execution: the monitor writes through the package logger.
			saved, level := log.Logger, zerolog.GlobalLevel()
			sink := blockedCacheExpiryLog{message, make(chan struct{}), make(chan struct{})}
			log.Logger = zerolog.New(sink).Level(zerolog.DebugLevel)
			zerolog.SetGlobalLevel(zerolog.DebugLevel)
			done := make(chan struct{})
			go func() {
				defer close(done)
				sweep(now)
			}()
			defer func() {
				close(sink.resume)
				<-done
				log.Logger = saved
				zerolog.SetGlobalLevel(level)
			}()
			select {
			case <-sink.entered:
				checkLive()
			case <-done:
				t.Fatal("sweep did not emit its expiry diagnostic")
			case <-time.After(5 * time.Second):
				t.Fatal("sweep did not reach the gated logger")
			}
		})
	}
}
