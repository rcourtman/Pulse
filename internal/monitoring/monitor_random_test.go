package monitoring

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// Detect an unprotected source call deterministically, even when a particular
// race-detector run happens to serialize the goroutines.
type monitorRandomOwnershipSource struct {
	monitor     *Monitor
	source      rand.Source
	unprotected atomic.Int32
}

func (s *monitorRandomOwnershipSource) Seed(seed int64) { s.source.Seed(seed) }

func (s *monitorRandomOwnershipSource) Int63() int64 {
	if s.monitor.rngMu.TryLock() {
		s.unprotected.Add(1)
		s.monitor.rngMu.Unlock()
	}
	return s.source.Int63()
}

func TestMonitorRandomSourceOwnership(t *testing.T) {
	for _, caller := range []string{"poll-backoff", "guest-metadata"} {
		t.Run(caller, func(t *testing.T) {
			m := &Monitor{guestMetadataLimiter: make(map[string]time.Time), guestMetadataMinRefresh: time.Minute, guestMetadataRefreshJitter: 30 * time.Second}
			source := &monitorRandomOwnershipSource{monitor: m, source: rand.NewSource(42)}
			m.rng = rand.New(source)
			expected := rand.New(rand.NewSource(42))
			if caller == "poll-backoff" {
				if got, want := m.randomFloat(), expected.Float64(); got != want {
					t.Fatalf("backoff changed the seeded distribution: %v != %v", got, want)
				}
			} else {
				now := time.Now()
				m.scheduleNextGuestMetadataFetch("guest", now)
				want := now.Add(time.Minute + time.Duration(expected.Int63n(int64(30*time.Second))))
				if !m.guestMetadataLimiter["guest"].Equal(want) {
					t.Fatal("guest scheduling changed the seeded jitter or refresh interval")
				}
			}
			if source.unprotected.Load() != 0 {
				t.Fatal("shared random source was used without its ownership lock")
			}
		})
	}
}

func TestMonitorRandomConcurrentBackoffAndMetadata(t *testing.T) {
	for _, seeded := range []bool{false, true} {
		t.Run(fmt.Sprintf("seeded-%t", seeded), func(t *testing.T) {
			m := &Monitor{guestMetadataLimiter: make(map[string]time.Time), guestMetadataMinRefresh: time.Minute, guestMetadataRefreshJitter: 30 * time.Second}
			if seeded {
				m.rng = rand.New(rand.NewSource(42))
			}
			const workers, iterations = 24, 64
			now := time.Now()
			start := make(chan struct{})
			var wg sync.WaitGroup
			for worker := 0; worker < workers; worker++ {
				wg.Add(1)
				go func(worker int) {
					defer wg.Done()
					<-start
					for i := 0; i < iterations; i++ {
						if worker%2 == 0 {
							if value := m.randomFloat(); value < 0 || value >= 1 {
								t.Errorf("backoff random value outside [0,1): %v", value)
							}
						} else {
							m.scheduleNextGuestMetadataFetch(fmt.Sprintf("guest-%d-%d", worker, i), now)
						}
					}
				}(worker)
			}
			close(start)
			wg.Wait()
			if len(m.guestMetadataLimiter) != workers/2*iterations {
				t.Fatal("concurrent scheduling lost a guest key")
			}
			for key, next := range m.guestMetadataLimiter {
				// A nil source deliberately means no jitter until the ordinary
				// backoff caller initializes it; neither path changes that policy.
				if next.Before(now.Add(time.Minute)) || !next.Before(now.Add(90*time.Second)) {
					t.Errorf("%s outside configured refresh/jitter bounds: %v", key, next.Sub(now))
				}
			}
		})
	}
}

func TestGuestMetadataConcurrentRefreshJitter(t *testing.T) {
	const guests = 8
	m := &Monitor{
		guestMetadataLimiter: make(map[string]time.Time), guestMetadataSlots: make(chan struct{}, guests),
		guestMetadataMinRefresh: time.Minute, guestMetadataRefreshJitter: 30 * time.Second,
		rng: rand.New(rand.NewSource(42)),
	}
	var commands atomic.Int32
	client := metadataReservationClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/agent/") {
			commands.Add(1)
		}
		metadataReservationPayload(w, r)
	}))
	status := &proxmox.VMStatus{Agent: proxmox.VMAgentField{Value: 1}}
	start := make(chan struct{})
	var wg sync.WaitGroup
	before := time.Now()
	for vmid := 105; vmid < 105+guests; vmid++ {
		wg.Add(1)
		go func(vmid int) {
			defer wg.Done()
			<-start
			_, _, os, _, version, deferred := m.fetchGuestAgentMetadata(context.Background(), client, "jitter", "node", "guest", vmid, status, false)
			if deferred || os != "FixtureOS" || version != "2.0" {
				t.Errorf("VM %d did not finish ordinary HTTP enrichment", vmid)
			}
		}(vmid)
	}
	close(start)
	wg.Wait()
	after := time.Now()
	assertMetadataReservationReleased(t, m)
	if commands.Load() != 3*guests || len(m.guestMetadataSlots) != 0 {
		t.Fatal("enrichment repeated a command or retained a worker slot")
	}
	for vmid := 105; vmid < 105+guests; vmid++ {
		key := guestMetadataCacheKey("jitter", "node", vmid)
		next := m.guestMetadataLimiter[key]
		if !m.hasRecentGuestMetadataEvidence("jitter", "node", vmid, after) || next.Before(before.Add(time.Minute)) || !next.Before(after.Add(90*time.Second)) {
			t.Errorf("VM %d lost useful metadata or its bounded refresh", vmid)
		}
	}
}
