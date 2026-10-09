package monitoring

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

func testGuestAgentEvidenceOriginalAge(t *testing.T) {
	now := time.Date(2026, time.October, 9, 15, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name     string
		evidence models.GuestAgentEvidence
		seen     time.Time
		want     bool
	}{
		{"recent", models.GuestAgentEvidence{Explicit: true, ObservedAt: now.Add(-time.Minute)}, now, true},
		{"inclusive-boundary", models.GuestAgentEvidence{Explicit: true, ObservedAt: now.Add(-recentGuestAgentEvidenceMaxAge)}, now, true},
		{"expired-with-fresh-poll", models.GuestAgentEvidence{Explicit: true, ObservedAt: now.Add(-recentGuestAgentEvidenceMaxAge - time.Nanosecond)}, now, false},
		{"explicitly-missing", models.GuestAgentEvidence{Explicit: true}, now, false},
		{"future-origin", models.GuestAgentEvidence{Explicit: true, ObservedAt: now.Add(time.Second)}, now, false},
		{"legacy-recent", models.GuestAgentEvidence{}, now.Add(-time.Minute), true},
		{"legacy-expired", models.GuestAgentEvidence{}, now.Add(-20 * time.Minute), false},
		{"legacy-future", models.GuestAgentEvidence{}, now.Add(time.Second), false},
		{"legacy-missing", models.GuestAgentEvidence{}, time.Time{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prev := models.VM{Type: "qemu", LastSeen: tc.seen, AgentVersion: "cached", IPAddresses: []string{"192.0.2.10"}, GuestAgentStatus: "available", GuestAgentEvidence: tc.evidence}
			if got := hasRecentGuestAgentEvidence(&prev, now); got != tc.want {
				t.Errorf("eligibility=%t want=%t", got, tc.want)
			}
			retained := retainGuestAgentEvidence(&prev, now)
			prev.LastSeen = now
			prev.GuestAgentEvidence = retained
			if got := hasRecentGuestAgentEvidence(&prev, now); got != tc.want {
				t.Errorf("new poll changed eligibility=%t want=%t", got, tc.want)
			}
		})
	}
	key := guestMetadataCacheKey("pve", "node", 105)
	m := &Monitor{guestMetadataCache: map[string]guestMetadataCacheEntry{key: metadataObservationFixture(now.Add(time.Second))}}
	if m.hasRecentGuestMetadataEvidence("pve", "node", 105, now) {
		t.Fatal("future metadata admitted a guest query")
	}
	for _, tc := range []struct {
		name string
		disk models.GuestDiskObservation
		want bool
	}{
		{"legacy-disk-only-original", models.GuestDiskObservation{Source: "guest-agent", ObservedAt: now.Add(-time.Minute)}, true},
		{"legacy-disk-only-expired", models.GuestDiskObservation{Source: "guest-agent", ObservedAt: now.Add(-20 * time.Minute)}, false},
		{"legacy-disk-only-unknown", models.GuestDiskObservation{Source: "guest-agent"}, false},
		{"legacy-linked-agent-is-not-QGA", models.GuestDiskObservation{Source: "agent", ObservedAt: now}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prev := models.VM{Type: "qemu", LastSeen: now, Disks: []models.Disk{{Total: 1000, Used: 200}}, DiskObservation: tc.disk}
			if got := hasRecentGuestAgentEvidence(&prev, now); got != tc.want {
				t.Errorf("disk-only legacy eligibility=%t want=%t", got, tc.want)
			}
		})
	}
}

func testGuestAgentEvidenceCanonicalContinuity(t *testing.T) {
	origin := time.Date(2026, time.October, 9, 15, 0, 0, 0, time.UTC)
	vm := models.VM{ID: "pve:node:105", Instance: "pve", Node: "node", VMID: 105, Type: "qemu", Status: "running", LastSeen: origin, AgentVersion: "cached", GuestAgentEvidence: models.GuestAgentEvidence{Explicit: true, ObservedAt: origin}}
	registry := unifiedresources.NewRegistry(nil)
	for minute := 1; minute <= 12; minute++ {
		now := origin.Add(time.Duration(minute) * time.Minute)
		vm.LastSeen = now
		registry.IngestSnapshot(models.StateSnapshot{VMs: []models.VM{vm}})
		vm = previousVMFromView(registry.VMs()[0])
		if vm.GuestAgentEvidence.ObservedAt != origin || hasRecentGuestAgentEvidence(&vm, now) != (minute <= 10) {
			t.Fatalf("minute %d renewed evidence or lost inclusive expiry: %+v", minute, vm.GuestAgentEvidence)
		}
		vm.GuestAgentEvidence = retainGuestAgentEvidence(&vm, now)
	}
	vm.GuestAgentEvidence = models.GuestAgentEvidence{Explicit: true}
	registry.IngestSnapshot(models.StateSnapshot{VMs: []models.VM{vm}})
	vm = previousVMFromView(registry.VMs()[0])
	if hasRecentGuestAgentEvidence(&vm, vm.LastSeen) {
		t.Fatal("canonical replacement resurrected unknown evidence")
	}
	// Fresh QGA/PVE evidence restores eligibility; linked Pulse-agent disks
	// alone cannot turn this evidence into an observation of QGA availability.
	now := origin.Add(13 * time.Minute)
	renewGuestAgentEvidence(&vm.GuestAgentEvidence, now, now)
	registry.IngestSnapshot(models.StateSnapshot{VMs: []models.VM{vm}})
	vm = previousVMFromView(registry.VMs()[0])
	if !hasRecentGuestAgentEvidence(&vm, now) {
		t.Fatal("new ordinary availability did not restore eligibility")
	}
	client := &linkedDiskPollClient{}
	m := &Monitor{rateTracker: NewRateTracker()}
	id := makeGuestID("pve", "node", 106)
	linked, _, _, _, _, ok := m.buildVMFromClusterResource(context.Background(), "pve", proxmox.ClusterResource{Type: "qemu", Node: "node", VMID: 106, Status: "running", MaxMem: 8000, MaxDisk: 1000}, client, id, map[string]models.Host{id: {Memory: models.Memory{Total: 8000, Used: 2000}, Disks: []models.Disk{{Total: 1000, Used: 250, Free: 750, Usage: 25}}, LastSeen: time.Now()}}, nil)
	if !ok || linked.DiskStatusReason != "" || linked.Disk.Usage != 25 || hasRecentGuestAgentEvidence(&linked, time.Now()) || client.fsCalls != 0 {
		t.Fatalf("linked agent changed QGA admission or lost independent disks: %+v", linked)
	}
}

type guestEvidencePollClient struct {
	metadataObservationClient
	filesystems []proxmox.VMFileSystem
}

func (c *guestEvidencePollClient) GetVMStatus(context.Context, string, int) (*proxmox.VMStatus, error) {
	return nil, nil
}

func (c *guestEvidencePollClient) GetVMFSInfo(context.Context, string, int) ([]proxmox.VMFileSystem, error) {
	return c.filesystems, nil
}

func testGuestAgentEvidenceAcceptedReadOrigins(t *testing.T) {
	for _, kind := range []string{"cached-metadata-empty-filesystem", "new-useful-metadata", "new-zero-filesystem"} {
		t.Run(kind, func(t *testing.T) {
			origin := time.Now().Add(-time.Minute)
			cached := metadataObservationFixture(origin)
			client := &guestEvidencePollClient{}
			if kind == "new-useful-metadata" {
				cached.fetchedAt = origin.Add(-20 * time.Minute)
				client.version = "2.0"
			}
			if kind == "new-zero-filesystem" {
				client.filesystems = []proxmox.VMFileSystem{{Mountpoint: "/", Type: "ext4", TotalBytes: 1000}}
			}
			m := &Monitor{rateTracker: NewRateTracker(), guestMetadataCache: map[string]guestMetadataCacheEntry{guestMetadataCacheKey("pve", "node", 105): cached}}
			prev := models.VM{Type: "qemu", LastSeen: time.Now(), AgentVersion: "cached", GuestAgentEvidence: models.GuestAgentEvidence{Explicit: true, ObservedAt: origin}}
			before := time.Now()
			vm, _, _, _, _, ok := m.buildVMFromClusterResource(context.Background(), "pve", proxmox.ClusterResource{Type: "qemu", Node: "node", VMID: 105, Status: "running", MaxMem: 8000, MaxDisk: 1000}, client, "pve:node:105", nil, &prev)
			if !ok {
				t.Fatal("guest disappeared")
			}
			if kind == "cached-metadata-empty-filesystem" {
				if vm.GuestAgentStatus == "available" || vm.GuestAgentEvidence.ObservedAt != origin || vm.AgentVersion != "1.0" || client.calls != 0 {
					t.Fatalf("cached metadata renewed eligibility or availability: %+v / calls=%d", vm, client.calls)
				}
			} else if vm.GuestAgentStatus != "available" || vm.GuestAgentEvidence.ObservedAt.Before(before) || !hasRecentGuestAgentEvidence(&vm, time.Now()) {
				t.Fatalf("current useful read did not restore its original eligibility time: %+v", vm)
			}
		})
	}
}

// This body also runs on the exact parent: optional identity preservation is
// not a new guest-agent observation. All HTTP endpoints here are synthetic.
func testGuestAgentRetainedIdentityCannotRenewAdmission(t *testing.T) {
	for _, collector := range []string{"cluster", "node"} {
		t.Run(collector, func(t *testing.T) {
			var phase, guestCalls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch {
				case strings.HasSuffix(r.URL.Path, "/config"):
					fmt.Fprint(w, `{"data":{}}`)
				case strings.HasSuffix(r.URL.Path, "/status/current"):
					switch phase.Load() {
					case 0:
						fmt.Fprint(w, `{"data":{"status":"running","agent":{"enabled":1,"available":0},"maxmem":8000,"meminfo":{"available":6000}}}`)
					case 1:
						http.Error(w, "permission denied", http.StatusForbidden)
					default:
						fmt.Fprint(w, `{"data":{"status":"running","agent":1,"maxmem":8000,"meminfo":{"available":6000}}}`)
					}
				case strings.Contains(r.URL.Path, "/agent/"):
					guestCalls.Add(1)
					if phase.Load() != 2 {
						http.Error(w, "permission denied", http.StatusForbidden)
						return
					}
					switch {
					case strings.HasSuffix(r.URL.Path, "get-fsinfo"):
						fmt.Fprint(w, `{"data":{"result":[{"mountpoint":"C:\\","type":"ntfs","total-bytes":1000,"used-bytes":0}]}}`)
					case strings.HasSuffix(r.URL.Path, "network-get-interfaces"):
						fmt.Fprint(w, `{"data":{"result":[{"name":"Ethernet","ip-addresses":[{"ip-address":"192.0.2.11","ip-address-type":"ipv4","prefix":24}]}]}}`)
					case strings.HasSuffix(r.URL.Path, "get-osinfo"):
						fmt.Fprint(w, `{"data":{"name":"Windows","version":"fixture"}}`)
					case strings.HasSuffix(r.URL.Path, "info"):
						fmt.Fprint(w, `{"data":{"result":{"version":"2.0"}}}`)
					default:
						http.NotFound(w, r)
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
			m := &Monitor{config: &config.Config{}, state: models.NewState(), rateTracker: NewRateTracker(), metricsHistory: NewMetricsHistory(32, time.Hour), guestMetadataLimiter: make(map[string]time.Time)}
			m.alertManager = alerts.NewManagerWithDataDir(t.TempDir(), alerts.WithoutPersistedAlertRestore())
			t.Cleanup(m.alertManager.Stop)
			m.guestMetadataRetryBackoff = time.Nanosecond
			id := makeGuestID("evidence", "node", 105)
			prev := models.VM{ID: id, Instance: "evidence", Node: "node", VMID: 105, Type: "qemu", Status: "running", LastSeen: time.Now().Add(-20 * time.Minute), IPAddresses: []string{"192.0.2.10"}, OSName: "Windows", AgentVersion: "1.0"}
			res := proxmox.ClusterResource{Type: "qemu", Node: "node", VMID: 105, Name: "windows", Status: "running", MaxMem: 8000, Mem: 2000, MaxDisk: 1000, CPU: .2}
			registry := unifiedresources.NewRegistry(nil)
			build := func() models.VM {
				t.Helper()
				ctx := context.Background()
				if phase.Load() == -1 {
					canceled, cancel := context.WithCancel(ctx)
					cancel()
					ctx = canceled
				}
				var vms []models.VM
				if collector == "cluster" {
					vms = m.collectClusterVMResources(ctx, "evidence", []indexedClusterResource{{resource: res, guestID: id}}, client, map[string]models.VM{id: prev}, nil)
				} else {
					vms, _ = m.pollNodeVMsWithClusterResourceBuilder(ctx, "evidence", "node", []proxmox.VM{{VMID: 105, Name: "windows", Status: "running", MaxMem: 8000, Mem: 2000, MaxDisk: 1000, CPU: .2}}, client, map[string]models.VM{id: prev}, nil)
				}
				if len(vms) != 1 {
					t.Fatal("guest disappeared")
				}
				vm := vms[0]
				m.recordGuestMetrics([]models.VM{vm}, nil, time.Now().Add(-time.Second))
				registry.IngestSnapshot(models.StateSnapshot{VMs: []models.VM{vm}})
				prev = previousVMFromView(registry.VMs()[0])
				if vm.ID != id || vm.CPU != .2 {
					t.Error("independent reading or identity lost")
				}
				return vm
			}
			phase.Store(-1)
			skipped := build()
			if len(skipped.IPAddresses) != 1 || skipped.OSName != "Windows" || skipped.AgentVersion != "1.0" {
				t.Fatal("cancelled detail did not preserve optional identity")
			}
			phase.Store(0)
			for poll := 0; poll < 3; poll++ {
				vm := build()
				if vm.GuestAgentStatus != "not-running" || vm.GuestAgentExpected {
					t.Errorf("poll %d: retained identity became current availability: %s/%t", poll, vm.GuestAgentStatus, vm.GuestAgentExpected)
				}
			}
			phase.Store(1)
			for poll := 0; poll < 3; poll++ {
				build()
			}
			if guestCalls.Load() != 0 {
				t.Errorf("expired retained identity admitted %d guest commands after a missing status", guestCalls.Load())
			}
			if got := m.metricsHistory.GetGuestMetrics(id, "cpu", time.Hour); len(got) != 7 {
				t.Errorf("independent CPU History lost: %d", len(got))
			}
			phase.Store(2)
			recovered := build()
			if recovered.GuestAgentStatus != "available" || guestCalls.Load() != 4 || recovered.AgentVersion != "2.0" || recovered.DiskStatusReason != "" || recovered.Disk.Usage != 0 {
				t.Errorf("ordinary fresh availability/disk-zero recovery failed: status=%s calls=%d version=%s disk=%+v reason=%s", recovered.GuestAgentStatus, guestCalls.Load(), recovered.AgentVersion, recovered.Disk, recovered.DiskStatusReason)
			}
			if !hasRecentGuestAgentEvidence(&prev, time.Now()) {
				t.Error("fresh metadata-only canonical evidence lost")
			}
		})
	}
}
