package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// A diagnostic refresh is not a new memory reading. These cases use the
// existing selectors, so the identical test also runs against the exact parent.
func testGuestMemoryCarryForwardOriginalAge(t *testing.T) {
	now := time.Date(2026, time.October, 9, 13, 0, 0, 0, time.UTC)
	for _, path := range []struct {
		name   string
		window time.Duration
		read   func(*GuestMemorySnapshot) bool
	}{
		{"trusted-reconnect", guestMemoryCarryForwardMaxAge, func(prev *GuestMemorySnapshot) bool {
			return shouldCarryForwardPreviousGuestMemory(prev, "running", "unavailable", 100, 0, now)
		}},
		{"healthy-guest", guestMemoryHealthyGuestMaxAge, func(prev *GuestMemorySnapshot) bool {
			return shouldCarryForwardHealthyGuestLowTrustMemory(prev, "running", "status-mem", 100, 100, now, true)
		}},
	} {
		t.Run(path.name, func(t *testing.T) {
			for _, tc := range []struct {
				name        string
				observation models.MemoryObservation
				retrievedAt time.Time
				source      string
				want        bool
			}{
				{"recent-current", models.MemoryObservation{State: "current", Source: "guest-agent-meminfo", ObservedAt: now.Add(-path.window / 2)}, now, "guest-agent-meminfo", true},
				{"recent-last-known", models.MemoryObservation{State: "last-known", Source: "guest-agent-meminfo", ObservedAt: now.Add(-path.window / 2)}, now, "guest-agent-meminfo", true},
				{"inclusive-boundary", models.MemoryObservation{State: "last-known", Source: "guest-agent-meminfo", ObservedAt: now.Add(-path.window)}, now, "guest-agent-meminfo", true},
				{"expired-original-with-fresh-poll", models.MemoryObservation{State: "last-known", Source: "guest-agent-meminfo", ObservedAt: now.Add(-path.window - time.Nanosecond)}, now, "guest-agent-meminfo", false},
				{"future-original", models.MemoryObservation{State: "current", Source: "guest-agent-meminfo", ObservedAt: now.Add(time.Second)}, now, "guest-agent-meminfo", false},
				{"missing-original", models.MemoryObservation{State: "last-known", Source: "guest-agent-meminfo"}, now, "guest-agent-meminfo", false},
				{"explicit-unavailable", models.MemoryObservation{State: "unavailable", Source: "guest-agent-meminfo", ObservedAt: now}, now, "guest-agent-meminfo", false},
				{"unknown-state", models.MemoryObservation{State: "unknown", Source: "guest-agent-meminfo", ObservedAt: now}, now, "guest-agent-meminfo", false},
				{"partial-annotation", models.MemoryObservation{Source: "guest-agent-meminfo", ObservedAt: now}, now, "guest-agent-meminfo", false},
				{"legacy-direct-recent", models.MemoryObservation{}, now.Add(-path.window / 2), "guest-agent-meminfo", true},
				{"legacy-direct-expired", models.MemoryObservation{}, now.Add(-path.window - time.Second), "guest-agent-meminfo", false},
				{"legacy-direct-future", models.MemoryObservation{}, now.Add(time.Second), "guest-agent-meminfo", false},
				{"legacy-direct-missing", models.MemoryObservation{}, time.Time{}, "guest-agent-meminfo", false},
				{"legacy-retained-has-no-original-age", models.MemoryObservation{}, now, "previous-snapshot", false},
			} {
				t.Run(tc.name, func(t *testing.T) {
					prev := &GuestMemorySnapshot{Status: "running", RetrievedAt: tc.retrievedAt, MemorySource: tc.source,
						Memory: models.Memory{Total: 100, Used: 25, Free: 75, Usage: 25, Observation: tc.observation}}
					if got := path.read(prev); got != tc.want {
						t.Errorf("carry-forward = %t, want %t; original=%v poll=%v", got, tc.want, tc.observation, tc.retrievedAt)
					}
				})
			}
		})
	}
}

func testGuestMemoryCarryForwardRepeatedPollsDoNotExtendAge(t *testing.T) {
	original := time.Date(2026, time.October, 9, 13, 0, 0, 0, time.UTC)
	prev := &GuestMemorySnapshot{Status: "running", MemorySource: "previous-snapshot",
		Memory: models.Memory{Total: 100, Used: 25, Free: 75, Usage: 25,
			Observation: models.MemoryObservation{State: "last-known", Source: "available-field", ObservedAt: original}}}
	for minute := 1; minute <= 12; minute++ {
		now := original.Add(time.Duration(minute) * time.Minute)
		prev.RetrievedAt = now // Ordinary polling refreshes diagnostics every time.
		used, source, _ := stabilizeGuestLowTrustMemory(prev, "running", "status-mem", 100, 100, now, true)
		wantUsed, wantSource := uint64(25), "previous-snapshot"
		if minute > 10 {
			wantUsed, wantSource = 100, "status-mem"
		}
		if used != wantUsed || source != wantSource {
			t.Errorf("minute %d selected %d/%s, want %d/%s", minute, used, source, wantUsed, wantSource)
		}
		if prev.Memory.Observation.ObservedAt != original {
			t.Fatal("selector mutated the original observation")
		}
	}
}

// The real PVE client sees an unverified configuration and dispatches no guest
// commands. This exercises selection, source-owned cache age, served canonical
// projections, History and normal recovery without freezing/probing a real VM.
func testGuestMemoryCarryForwardOrdinaryDeferralAndRecovery(t *testing.T) {
	const gib = uint64(1024 * 1024 * 1024)
	for _, tc := range []struct {
		name          string
		age           time.Duration
		priorSource   string
		missingOrigin bool
		wantUsed      uint64
		wantSource    string
	}{
		{"recent-retained", 3 * time.Minute, "previous-snapshot", false, 3 * gib, "previous-snapshot"},
		{"expired-retained", 11 * time.Minute, "previous-snapshot", false, 8 * gib, "status-mem"},
		{"expired-direct", 11 * time.Minute, "guest-agent-meminfo", false, 8 * gib, "status-mem"},
		{"retained-without-origin", 11 * time.Minute, "previous-snapshot", true, 8 * gib, "status-mem"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var phase, commands atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.HasSuffix(r.URL.Path, "/status/current"):
					memInfo := ""
					if phase.Load() == 2 {
						memInfo = fmt.Sprintf(`,"meminfo":{"total":%d,"available":%d}`, 8*gib, 6*gib)
					}
					fmt.Fprintf(w, `{"data":{"status":"running","agent":1,"maxmem":%d,"mem":%d%s}}`, 8*gib, 8*gib, memInfo)
				case strings.HasSuffix(r.URL.Path, "/config"):
					if phase.Load() == 1 {
						fmt.Fprint(w, `{"data":{}}`)
					} else {
						fmt.Fprint(w, `{"data":null}`)
					}
				case strings.Contains(r.URL.Path, "/agent/"):
					commands.Add(1)
					if strings.HasSuffix(r.URL.Path, "/file-read") {
						fmt.Fprint(w, `{"data":{"content":"MemTotal: 8388608 kB\nMemAvailable: 4194304 kB\n"}}`)
					} else {
						fmt.Fprint(w, `{"data":{"result":[]}}`)
					}
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			client, err := proxmox.NewClient(proxmox.ClientConfig{Host: server.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture", Timeout: time.Second})
			if err != nil {
				t.Fatal(err)
			}
			identity := makeGuestID("expiry", "node", 105)
			registry := unifiedresources.NewRegistry(nil)
			m := &Monitor{config: &config.Config{}, rateTracker: NewRateTracker(), metricsHistory: NewMetricsHistory(32, time.Hour),
				resourceStore: unifiedresources.NewMonitorAdapter(registry), guestMetadataLimiter: make(map[string]time.Time)}
			original := time.Now().Add(-tc.age)
			origin := original
			if tc.missingOrigin {
				origin = time.Time{}
			}
			m.recordGuestSnapshot("expiry", "qemu", "node", 105, GuestMemorySnapshot{Status: "running", RetrievedAt: time.Now(), MemorySource: tc.priorSource,
				Memory: models.Memory{Total: 8 * int64(gib), Used: 3 * int64(gib), Free: 5 * int64(gib), Usage: 37.5,
					Observation: models.MemoryObservation{State: "last-known", Source: "guest-agent-meminfo", ObservedAt: origin}}})
			key := guestMemoryCacheKey("expiry", "node", 105)
			m.vmAgentMemCache = map[string]agentMemCacheEntry{key: {fetchedAt: original,
				info: proxmox.LinuxMemoryAvailability{Source: "meminfo-available", EffectiveAvailable: 5 * gib}}}
			m.guestMetadataCache = map[string]guestMetadataCacheEntry{guestMetadataCacheKey("expiry", "node", 105): {fetchedAt: time.Now(), agentVersion: "fixture"}}
			res := proxmox.ClusterResource{Type: "qemu", Node: "node", Name: "guest", VMID: 105, Status: "running", MaxMem: 8 * gib, Mem: 8 * gib, MaxDisk: gib, CPU: 0.2}
			build := func() (models.VM, string) {
				vm, raw, source, notes, at, ok := m.buildVMFromClusterResource(context.Background(), "expiry", res, client, identity, nil, nil)
				if !ok {
					t.Fatal("guest disappeared")
				}
				m.recordGuestSnapshot("expiry", "qemu", "node", 105, GuestMemorySnapshot{Status: vm.Status, RetrievedAt: at, MemorySource: source, Memory: vm.Memory, Raw: raw, Notes: notes})
				m.recordGuestMetrics([]models.VM{vm}, nil, time.Now().Add(-time.Second))
				return vm, source
			}
			vm, source := build()
			if uint64(vm.Memory.Used) != tc.wantUsed || source != tc.wantSource || vm.GuestAgentStatus != "deferred" {
				t.Errorf("ordinary deferred poll selected %d/%s/%s, want %d/%s/deferred", vm.Memory.Used, source, vm.GuestAgentStatus, tc.wantUsed, tc.wantSource)
			}
			if commands.Load() != 0 || m.vmAgentMemCache[key].fetchedAt != original {
				t.Fatal("deferral sent a guest command or renewed the cache")
			}
			state, observedSource, at, history := "current", "status-mem", vm.Memory.Observation.ObservedAt, 1
			if tc.wantSource == "previous-snapshot" {
				state, observedSource, at, history = "last-known", "guest-agent-meminfo", original, 0
			}
			assertGuestMemoryObservation(t, vm.Memory, state, observedSource, at)
			front := m.buildBroadcastFrontendStateFromSnapshot(models.StateSnapshot{VMs: []models.VM{vm}}, m.mockModeFence.begin())
			wire, err := json.Marshal(front.Resources)
			if err != nil {
				t.Fatal(err)
			}
			var served []struct {
				Memory  *models.ResourceMetricFrontend `json:"memory"`
				Proxmox struct {
					Memory models.Memory `json:"memory"`
				} `json:"proxmox"`
			}
			if err := json.Unmarshal(wire, &served); err != nil || len(served) != 1 || served[0].Memory == nil {
				t.Fatalf("served memory missing: %s / %v", wire, err)
			}
			assertGuestMemoryObservation(t, served[0].Proxmox.Memory, state, observedSource, at)
			assertGuestMemoryObservation(t, models.Memory{Observation: served[0].Memory.Observation}, state, observedSource, at)
			if served[0].Proxmox.Memory.Used != int64(tc.wantUsed) || served[0].Memory.Current != float64(tc.wantUsed)*100/float64(8*gib) {
				t.Error("served values do not match the selected source")
			}
			for metric, want := range map[string]int{"cpu": 1, "memory": history, "memoryused": history, "disk": 0} {
				if got := len(m.metricsHistory.GetGuestMetrics(identity, metric, time.Hour)); got != want {
					t.Errorf("%s History points = %d, want %d", metric, got, want)
				}
			}
			// Independent current PVE memory is not displaced by any retained
			// value or disk/QGA deferral, even before the fallback window expires.
			phase.Store(2)
			live, liveSource := build()
			if live.Memory.Used != int64(2*gib) || liveSource != "available-field" || live.Memory.Observation.State != "current" || commands.Load() != 0 {
				t.Error("independent PVE memory lost to guest deferral")
			}
			// An ordinary, newly admitted successful QGA read replaces the old
			// observation; no restart, manual probe or forced thaw is involved.
			phase.Store(1)
			resumed, resumedSource := build()
			if resumed.Memory.Used != int64(4*gib) || resumedSource != "guest-agent-meminfo" || resumed.Memory.Observation.State != "current" || !m.vmAgentMemCache[key].fetchedAt.After(original) || commands.Load() != 2 {
				t.Errorf("normal resumption failed: %d/%s/%s commands=%d", resumed.Memory.Used, resumedSource, resumed.Memory.Observation.State, commands.Load())
			}
		})
	}
}
