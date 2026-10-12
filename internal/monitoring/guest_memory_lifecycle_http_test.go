package monitoring

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

func TestGuestMemoryLifecycleHTTPCancellationPreservesSharedFence(t *testing.T) {
	for _, kind := range []string{"client", "cluster-client"} {
		for _, disposition := range []string{"guest", "connection", "unavailable", "stopped"} {
			t.Run(kind+"/"+disposition, func(t *testing.T) {
				entered, release := make(chan struct{}, 1), make(chan struct{})
				var once sync.Once
				unblock := func() { once.Do(func() { close(release) }) }
				var mu sync.Mutex
				var commands []string
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					switch {
					case strings.HasSuffix(r.URL.Path, "/nodes"):
						fmt.Fprint(w, `{"data":[]}`)
					case strings.HasSuffix(r.URL.Path, "/config"):
						fmt.Fprint(w, `{"data":{}}`)
					default:
						mu.Lock()
						commands = append(commands, r.URL.Path)
						mu.Unlock()
						if strings.Contains(r.URL.Path, "/qemu/105/agent/") {
							// Consumed headers do not establish body/command completion.
							w.WriteHeader(http.StatusOK)
							w.(http.Flusher).Flush()
							entered <- struct{}{}
							<-release
						} else {
							fmt.Fprint(w, `{"data":{"content":"MemTotal: 8192 kB\nMemAvailable: 6144 kB\nMemFree: 1024 kB\n","truncated":false}}`)
						}
					}
				}))
				defer server.Close()
				defer unblock()
				client := windowsBootstrapClient(t, server.URL, kind)
				m := &Monitor{}
				done := make(chan guestMemoryReply, 1)
				go func() {
					info, err := m.getVMAgentMemoryAvailability(context.Background(), client, "http-memory", "node", 105)
					done <- guestMemoryReply{info, err}
				}()
				select {
				case <-entered:
				case <-time.After(5 * time.Second):
					t.Fatal("synthetic file-read body did not start")
				}
				invalidateMemoryLifecycle(m, "http-memory", disposition)
				if got := readGuestMemoryReply(t, done); !errors.Is(got.err, proxmox.ErrGuestAgentDeferred) || got.info != (proxmox.LinuxMemoryAvailability{}) {
					t.Fatal("cancelled HTTP read published successful/failed guest memory")
				}
				// A new client and the replacement monitor lifecycle must both
				// respect the original consumed command's transport uncertainty.
				diagnostic := windowsBootstrapClient(t, server.URL, "client")
				if _, err := diagnostic.GetVMAgentInfo(context.Background(), "node", 105); proxmox.GuestAgentDeferredReason(err) != "agent-cooldown" {
					t.Errorf("invalidation erased shared diagnostic uncertainty: %v", err)
				}
				if _, err := m.getVMAgentMemoryAvailability(context.Background(), client, "http-memory", "node", 105); !errors.Is(err, proxmox.ErrGuestAgentDeferred) {
					t.Error("replacement memory lifecycle bypassed shared uncertainty")
				}
				if len(m.vmAgentMemCache) != 0 || len(m.vmAgentMemInFlight) != 0 {
					t.Error("cancelled/deferred memory retained a negative sample or leaked ownership")
				}
				mu.Lock()
				before := append([]string(nil), commands...)
				mu.Unlock()
				if len(before) != 1 || !strings.HasSuffix(before[0], "/file-read") {
					t.Fatal("invalidation/cooldown issued another command")
				}
				if info, err := m.getVMAgentMemoryAvailability(context.Background(), client, "http-memory", "node", 106); err != nil || info.EffectiveAvailable != 6144*1024 {
					t.Errorf("cancelled read blocked independent VM: %+v / %v", info, err)
				}
				mu.Lock()
				if len(commands) != 2 || !strings.Contains(commands[1], "/qemu/106/agent/file-read") {
					t.Error("independent VM added traffic to the cancelled VM")
				}
				mu.Unlock()
			})
		}
	}
}

func TestGuestMemoryLifecycleRestartPublishesNewSample(t *testing.T) {
	for _, kind := range []string{"client", "cluster-client"} {
		for _, collector := range []string{"cluster", "node"} {
			for _, disposition := range []string{"guest", "connection", "unavailable", "stopped"} {
				t.Run(kind+"/"+collector+"/"+disposition, func(t *testing.T) {
					f, server := newWindowsBootstrapFixture(t)
					f.osName = "Debian GNU/Linux"
					client := windowsBootstrapClient(t, server.URL, kind)
					m := windowsBootstrapMonitor(t)
					initial := windowsBootstrapPoll(t, m, client, collector, "bootstrap", "node", 105, 1)
					key := guestMemoryCacheKey("bootstrap", "node", 105)
					original := m.vmAgentMemCache[key]
					if initial.Memory.Used != int64(2*bootstrapMiB) || initial.Memory.Observation.Source != "guest-agent-meminfo" || original.fetchedAt.IsZero() {
						t.Fatal("initial real-client Linux memory sample missing")
					}
					invalidateMemoryLifecycle(m, "bootstrap", disposition)
					f.mu.Lock()
					f.linuxAvailable = 4 * bootstrapMiB
					f.mu.Unlock()
					f.poll.Store(2)
					resumed := windowsBootstrapPoll(t, m, client, collector, "bootstrap", "node", 105, 2)
					if resumed.ID != initial.ID || resumed.Memory.Used != int64(4*bootstrapMiB) || resumed.Memory.Usage != 50 || resumed.Memory.Observation.State != "current" || resumed.Memory.Observation.Source != "guest-agent-meminfo" || !resumed.Memory.Observation.ObservedAt.After(original.fetchedAt) {
						t.Errorf("restart reused pre-transition memory: %+v", resumed.Memory)
					}
					var fileReads int
					for _, command := range f.trace() {
						if command == "file-read" {
							fileReads++
						}
					}
					if fileReads != 2 {
						t.Errorf("restart file reads = %d, want two separate lifecycle samples", fileReads)
					}
					front := m.buildBroadcastFrontendStateFromSnapshot(models.StateSnapshot{VMs: []models.VM{resumed}}, m.mockModeFence.begin())
					data, err := json.Marshal(front.Resources)
					if err != nil {
						t.Fatal(err)
					}
					var served []struct{ Proxmox unifiedresources.ProxmoxData }
					if err := json.Unmarshal(data, &served); err != nil || len(served) != 1 || served[0].Proxmox.Memory.Used != resumed.Memory.Used || served[0].Proxmox.Memory.Observation != resumed.Memory.Observation {
						t.Errorf("served memory lost the new sample: %s / %v", data, err)
					}
					points := m.metricsHistory.GetGuestMetrics(initial.ID, "memory", time.Hour)
					if len(points) != 2 || points[0].Value != 25 || points[1].Value != 50 {
						t.Errorf("restart History retained old numeric memory: %+v", points)
					}
				})
			}
		}
	}
}

func TestGuestMemoryLifecycleCancelledRefreshPreservesOriginalSample(t *testing.T) {
	for _, outcome := range []string{"success", "failure", "empty"} {
		t.Run(outcome, func(t *testing.T) {
			key := guestMemoryCacheKey("cancel-refresh", "node", 105)
			original := agentMemCacheEntry{info: memoryLifecycleSample(4096), fetchedAt: time.Now().Add(-2 * vmAgentMemCacheTTL)}
			m := &Monitor{vmAgentMemCache: map[string]agentMemCacheEntry{key: original}}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			old := &blockedGuestMemoryClient{stubPVEClient: &stubPVEClient{}, entered: make(chan context.Context, 1), release: make(chan struct{}), info: memoryLifecycleSample(6144)}
			if outcome != "success" {
				old.info = proxmox.LinuxMemoryAvailability{}
			}
			if outcome == "failure" {
				old.err = context.Canceled
			}
			var once sync.Once
			unblock := func() { once.Do(func() { close(old.release) }) }
			defer unblock()
			done := make(chan guestMemoryReply, 1)
			go func() {
				info, err := m.getVMAgentMemoryAvailability(ctx, old, "cancel-refresh", "node", 105)
				done <- guestMemoryReply{info, err}
			}()
			select {
			case <-old.entered:
			case <-time.After(5 * time.Second):
				t.Fatal("cancelled refresh did not start")
			}
			cancel()
			nextInfo := memoryLifecycleSample(5120)
			next := &guestMemoryAgentTestClient{stubPVEClient: &stubPVEClient{}, memInfo: &nextInfo}
			if _, err := m.getVMAgentMemoryAvailability(context.Background(), next, "cancel-refresh", "node", 105); !errors.Is(err, proxmox.ErrGuestAgentDeferred) || next.memCalls != 0 {
				t.Error("parent cancellation released an unfinished memory read")
			}
			unblock()
			if got := readGuestMemoryReply(t, done); !errors.Is(got.err, proxmox.ErrGuestAgentDeferred) || got.info != (proxmox.LinuxMemoryAvailability{}) {
				t.Error("cancelled refresh returned an unsupported new observation")
			}
			if m.vmAgentMemCache[key] != original || len(m.vmAgentMemInFlight) != 0 {
				t.Error("caller cancellation renewed/erased the original sample or created backoff")
			}
			if got, err := m.getVMAgentMemoryAvailability(context.Background(), next, "cancel-refresh", "node", 105); err != nil || got != nextInfo || next.memCalls != 1 {
				t.Error("ordinary post-cancellation refresh did not obtain new evidence")
			}
		})
	}
}
