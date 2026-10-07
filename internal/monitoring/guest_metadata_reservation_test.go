package monitoring

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

func metadataReservationClient(t *testing.T, handler http.Handler) *proxmox.Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client, err := proxmox.NewClient(proxmox.ClientConfig{Host: server.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture", Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func metadataReservationPayload(w http.ResponseWriter, r *http.Request) {
	switch {
	case strings.HasSuffix(r.URL.Path, "/config"):
		fmt.Fprint(w, `{"data":{}}`)
	case strings.HasSuffix(r.URL.Path, "/network-get-interfaces"):
		fmt.Fprint(w, `{"data":{"result":[{"name":"eth0","hardware-address":"02:00:00:00:00:01","ip-addresses":[{"ip-address":"192.0.2.20","prefix":24,"ip-address-type":"ipv4"}]}]}}`)
	case strings.HasSuffix(r.URL.Path, "/get-osinfo"):
		fmt.Fprint(w, `{"data":{"result":{"name":"FixtureOS","version":"2"}}}`)
	case strings.HasSuffix(r.URL.Path, "/info"):
		fmt.Fprint(w, `{"data":{"result":{"version":"2.0"}}}`)
	default:
		http.NotFound(w, r)
	}
}

func assertMetadataReservationReleased(t *testing.T, m *Monitor) {
	t.Helper()
	m.guestMetadataLimiterMu.Lock()
	defer m.guestMetadataLimiterMu.Unlock()
	if len(m.guestMetadataInFlight) != 0 {
		t.Fatal("completed fetch retained in-flight ownership")
	}
}

func TestGuestMetadataReservationOwnsWholeHTTPFetch(t *testing.T) {
	for _, seeded := range []bool{false, true} {
		for _, expiredHold := range []bool{false, true} {
			t.Run(fmt.Sprintf("cached-%t/hold-expired-%t", seeded, expiredHold), func(t *testing.T) {
				key := guestMetadataCacheKey("reservation", "node", 105)
				original := guestMetadataCacheEntry{}
				m := &Monitor{guestMetadataLimiter: make(map[string]time.Time), guestMetadataSlots: make(chan struct{}, 2)}
				if seeded {
					original = metadataObservationFixture(time.Now().Add(-2 * guestMetadataCacheTTL))
					original.osInfoSkip = true
					original.osInfoFailureCount = guestAgentOSInfoFailureThreshold
					m.guestMetadataCache = map[string]guestMetadataCacheEntry{key: original}
				}
				entered, release := make(chan struct{}), make(chan struct{})
				var commands atomic.Int32
				client := metadataReservationClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if strings.Contains(r.URL.Path, "/agent/") {
						commands.Add(1)
					}
					if strings.Contains(r.URL.Path, "/qemu/105/") && strings.HasSuffix(r.URL.Path, "/network-get-interfaces") {
						select {
						case <-entered:
						default:
							close(entered)
						}
						<-release
					}
					metadataReservationPayload(w, r)
				}))
				status := &proxmox.VMStatus{Agent: proxmox.VMAgentField{Value: 1}}
				done := make(chan bool, 1)
				go func() {
					_, _, _, _, _, deferred := m.fetchGuestAgentMetadata(context.Background(), client, "reservation", "node", "guest", 105, status, false)
					done <- deferred
				}()
				t.Cleanup(func() {
					select {
					case <-release:
					default:
						close(release)
					}
					select {
					case <-done:
					case <-time.After(6 * time.Second):
						t.Error("fetch did not finish during cleanup")
					}
				})
				select {
				case <-entered:
				case <-time.After(5 * time.Second):
					t.Fatal("network command did not enter")
				}
				m.guestMetadataLimiterMu.Lock()
				if expiredHold {
					// Advance only test-owned scheduling state; do not shorten the
					// Proxmox uncertainty fence or wait for a real guest timeout.
					m.guestMetadataLimiter[key] = time.Now().Add(-time.Second)
				}
				before := m.guestMetadataLimiter[key]
				m.guestMetadataLimiterMu.Unlock()
				ips, ifaces, name, version, agent, deferred := m.fetchGuestAgentMetadata(context.Background(), client, "reservation", "node", "guest", 105, status, false)
				if deferred || !reflect.DeepEqual(ips, original.ipAddresses) || !reflect.DeepEqual(ifaces, original.networkInterfaces) || name != original.osName || version != original.osVersion || agent != original.agentVersion {
					t.Error("duplicate fetch invented a transport pause or changed retained identity")
				}
				m.guestMetadataLimiterMu.Lock()
				unchanged := m.guestMetadataLimiter[key].Equal(before)
				m.guestMetadataLimiterMu.Unlock()
				if !unchanged || commands.Load() != 1 || len(m.guestMetadataSlots) != 1 {
					t.Error("duplicate fetch changed retry timing, sent a command or consumed another slot")
				}
				// A different VM is not blocked by this metadata reservation.
				_, _, _, _, otherAgent, otherDeferred := m.fetchGuestAgentMetadata(context.Background(), client, "reservation", "node", "other", 106, status, false)
				if otherDeferred || otherAgent != "2.0" {
					t.Error("reservation blocked another VM")
				}
				close(release)
				select {
				case deferred := <-done:
					if deferred {
						t.Error("owner did not complete normally")
					}
					// Leave a completed marker for the unconditional cleanup drain.
					done <- false
				case <-time.After(5 * time.Second):
					t.Fatal("owner did not finish")
				}
				assertMetadataReservationReleased(t, m)
				m.guestMetadataMu.Lock()
				entry := m.guestMetadataCache[key]
				entry.fetchedAt = time.Now().Add(-2 * guestMetadataCacheTTL)
				m.guestMetadataCache[key] = entry
				m.guestMetadataMu.Unlock()
				m.guestMetadataLimiterMu.Lock()
				m.guestMetadataLimiter[key] = time.Now().Add(-time.Second)
				m.guestMetadataLimiterMu.Unlock()
				_, _, _, _, agent, deferred = m.fetchGuestAgentMetadata(context.Background(), client, "reservation", "node", "guest", 105, status, false)
				if deferred || agent != "2.0" || !m.hasRecentGuestMetadataEvidence("reservation", "node", 105, time.Now()) || len(m.guestMetadataSlots) != 0 {
					t.Error("completed owner did not allow ordinary refresh")
				}
				assertMetadataReservationReleased(t, m)
			})
		}
	}
}

func TestGuestMetadataColdCacheHonoursEarlyBackoff(t *testing.T) {
	for _, mode := range []string{"unverified-lock", "cancelled-slot"} {
		t.Run(mode, func(t *testing.T) {
			key := guestMetadataCacheKey("backoff", "node", 105)
			m := &Monitor{guestMetadataLimiter: make(map[string]time.Time), guestMetadataRetryBackoff: time.Minute}
			var requests atomic.Int32
			var healthy atomic.Bool
			client := metadataReservationClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if !healthy.Load() && strings.HasSuffix(r.URL.Path, "/config") {
					fmt.Fprint(w, `{"data":null}`)
					return
				}
				metadataReservationPayload(w, r)
			}))
			status := &proxmox.VMStatus{Agent: proxmox.VMAgentField{Value: 1}}
			ctx := context.Background()
			if mode == "cancelled-slot" {
				m.guestMetadataSlots = make(chan struct{}, 1)
				m.guestMetadataSlots <- struct{}{}
				cancelled, cancel := context.WithCancel(ctx)
				cancel()
				ctx = cancelled
			}
			_, _, _, _, _, deferred := m.fetchGuestAgentMetadata(ctx, client, "backoff", "node", "guest", 105, status, false)
			if deferred != (mode == "unverified-lock") || len(m.guestMetadataCache) != 0 {
				t.Fatal("early exit fabricated metadata or lost observed deferral")
			}
			assertMetadataReservationReleased(t, m)
			if mode == "cancelled-slot" {
				<-m.guestMetadataSlots
			}
			before, next := requests.Load(), m.guestMetadataLimiter[key]
			ips, ifaces, name, version, agent, deferred := m.fetchGuestAgentMetadata(context.Background(), client, "backoff", "node", "guest", 105, status, false)
			if requests.Load() != before || !m.guestMetadataLimiter[key].Equal(next) || len(m.guestMetadataCache) != 0 || len(ips)+len(ifaces) != 0 || name+version+agent != "" || deferred {
				t.Error("cold cache bypassed retry backoff or invented guest evidence")
			}
			// No metadata from another installation can satisfy this key.
			m.guestMetadataCache = map[string]guestMetadataCacheEntry{guestMetadataCacheKey("independent", "node", 105): {osName: "independent"}}
			healthy.Store(true)
			m.guestMetadataLimiter[key] = time.Now().Add(-time.Second)
			_, _, _, _, agent, deferred = m.fetchGuestAgentMetadata(context.Background(), client, "backoff", "node", "guest", 105, status, false)
			if deferred || agent != "2.0" || !m.hasRecentGuestMetadataEvidence("backoff", "node", 105, time.Now()) || !reflect.DeepEqual(m.guestMetadataCache[guestMetadataCacheKey("independent", "node", 105)], guestMetadataCacheEntry{osName: "independent"}) {
				t.Error("healthy cold cache did not resume independently after backoff")
			}
			assertMetadataReservationReleased(t, m)
		})
	}
}
