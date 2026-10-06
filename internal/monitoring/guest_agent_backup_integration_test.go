package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
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

func testGuestAgentBackupMonitoringLifecycle(t *testing.T) {
	testGuestAgentBackupMonitoringLifecycleWithUnverifiedConfig(t, "")
}

func testGuestAgentBackupMonitoringLifecycleWithUnverifiedConfig(t *testing.T, unverifiedConfig string) {
	const mib = uint64(1024 * 1024)
	var phase, calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := phase.Load()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/config"):
			if p == 4 {
				switch unverifiedConfig {
				case "redirect":
					http.Redirect(w, r, "/unrelated-unlocked-config", http.StatusTemporaryRedirect)
				case "":
					w.Header().Set("Content-Length", "1000")
					fmt.Fprint(w, `{"data":{}}`)
				default:
					fmt.Fprint(w, unverifiedConfig)
				}
			} else if p == 1 || p == 2 {
				fmt.Fprint(w, `{"data":{"lock":"backup"}}`)
			} else {
				fmt.Fprint(w, `{"data":{}}`)
			}
		case r.URL.Path == "/unrelated-unlocked-config":
			// Never count this unrelated unlocked response as the guest's lock.
			fmt.Fprint(w, `{"data":{}}`)
		case strings.HasSuffix(r.URL.Path, "/status/current"):
			if p == 2 {
				http.Error(w, "unavailable", 503)
				return
			}
			lock := ""
			if p == 1 {
				lock = `,"lock":"backup"`
			}
			fmt.Fprintf(w, `{"data":{"status":"running","agent":1,"maxmem":%d,"mem":%d%s}}`, 8*mib, 8*mib, lock)
		case strings.Contains(r.URL.Path, "/agent/"):
			calls.Add(1)
			switch {
			case strings.HasSuffix(r.URL.Path, "file-read"):
				available := 5 * 1024
				if p == 3 {
					available = 4 * 1024
				}
				json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"content": fmt.Sprintf("MemTotal: 8192 kB\nMemFree: 1024 kB\nMemAvailable: %d kB\n", available)}})
			case strings.HasSuffix(r.URL.Path, "get-fsinfo"):
				used := 300 * mib
				if p == 3 {
					used = 400 * mib
				}
				fmt.Fprintf(w, `{"data":{"result":[{"mountpoint":"/","type":"ext4","total-bytes":%d,"used-bytes":%d}]}}`, 1000*mib, used)
			case strings.HasSuffix(r.URL.Path, "network-get-interfaces"):
				fmt.Fprint(w, `{"data":{"result":[{"name":"eth0","hardware-address":"02:00:00:00:00:01","ip-addresses":[{"ip-address":"192.0.2.10","ip-address-type":"ipv4","prefix":24}]}]}}`)
			case strings.HasSuffix(r.URL.Path, "get-osinfo"):
				fmt.Fprint(w, `{"data":{"name":"Linux","version":"fixture"}}`)
			default:
				fmt.Fprint(w, `{"data":{"result":{"version":"1.0"}}}`)
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
	m := &Monitor{config: &config.Config{}, rateTracker: NewRateTracker(), metricsHistory: NewMetricsHistory(32, time.Hour), guestMetadataLimiter: make(map[string]time.Time)}
	res := proxmox.ClusterResource{Type: "qemu", Node: "node", Name: "guest", VMID: 105, Status: "running", MaxMem: 8 * mib, Mem: 8 * mib, MaxDisk: 1000 * mib, CPU: 0.1}
	var previous *models.VM
	identity := "fixture:node:105"
	registry := unifiedresources.NewRegistry(nil)
	build := func() (models.VM, string) {
		vm, raw, source, notes, at, ok := m.buildVMFromClusterResource(context.Background(), "fixture", res, client, identity, nil, previous)
		if !ok {
			t.Fatal("VM disappeared")
		}
		m.recordGuestSnapshot("fixture", "qemu", "node", 105, GuestMemorySnapshot{Name: vm.Name, Status: vm.Status, RetrievedAt: at, MemorySource: source, Memory: vm.Memory, Raw: raw, Notes: notes})
		m.recordGuestMetrics([]models.VM{vm}, nil, time.Now().Add(-time.Second))
		// Production obtains previous guest context from the unified read view,
		// not the builder's raw model (the context need not contain memory).
		registry.IngestSnapshot(models.StateSnapshot{VMs: []models.VM{vm}})
		views := registry.VMs()
		if len(views) != 1 || views[0].MemoryUsed() != vm.Memory.Used {
			t.Fatal("memory did not reach the unified read view")
		}
		next := previousVMFromView(views[0])
		previous = &next
		return vm, source
	}
	initial, source := build()
	if source != "guest-agent-meminfo" || initial.Memory.Used != int64(3*mib) || initial.Disk.Used != int64(300*mib) {
		t.Fatalf("healthy readings absent: %s %#v %#v", source, initial.Memory, initial.Disk)
	}
	before := calls.Load()
	if before != 5 {
		t.Fatalf("healthy agent command count = %d, want five", before)
	}
	metadataKey := guestMetadataCacheKey("fixture", "node", 105)
	memKey := guestMemoryCacheKey("fixture", "node", 105)
	originalMetadata := m.guestMetadataCache[metadataKey]
	originalMemory := m.vmAgentMemCache[memKey]
	// An incomplete, redirected or ambiguous config is not lock clearance.
	// Warm guest caches must stay labelled last-known rather than becoming new
	// disk/memory History samples when the lock response cannot be trusted.
	phase.Store(4)
	unverified, _ := build()
	if unverified.ID != initial.ID || unverified.GuestAgentStatus != "deferred" || unverified.DiskStatusReason != "prev-lock-unverified" || unverified.Disk.Used != initial.Disk.Used || unverified.Memory.Used != initial.Memory.Used {
		t.Fatalf("unverified lock response lost truthful continuity: %#v", unverified)
	}
	if calls.Load() != before || !reflect.DeepEqual(m.guestMetadataCache[metadataKey], originalMetadata) || !reflect.DeepEqual(m.vmAgentMemCache[memKey], originalMemory) {
		t.Fatal("unverified lock response sent a guest command or renewed caches")
	}
	for metric, want := range map[string]int{"cpu": 2, "memory": 1, "memoryused": 1, "disk": 1} {
		if got := len(m.metricsHistory.GetGuestMetrics(identity, metric, time.Hour)); got != want {
			t.Errorf("unverified lock %s history points = %d, want %d", metric, got, want)
		}
	}
	phase.Store(1)
	res.CPU = 0.2
	// A normal backup outlives the ordinary memory-read TTL. Deferrals must
	// keep using the original observation, never the previous poll's fallback
	// trust label or timestamp. Include truly-free/cache evidence in the check.
	originalMemory.fetchedAt = time.Now().Add(-2 * vmAgentMemCacheTTL)
	m.vmAgentMemCache[memKey] = originalMemory
	const lockedPolls = 8
	var locked models.VM
	for poll := 0; poll < lockedPolls; poll++ {
		locked, source = build()
		if locked.ID != initial.ID || locked.GuestAgentStatus != "deferred" || locked.DiskStatusReason != "prev-vm-locked" {
			t.Fatalf("locked poll %d lost continuity/truth: %#v", poll, locked)
		}
		if guestMemoryValuesOnly(locked.Memory) != guestMemoryValuesOnly(initial.Memory) || source != "previous-snapshot" || locked.Disk.Used != initial.Disk.Used || !reflect.DeepEqual(locked.NetworkInterfaces, initial.NetworkInterfaces) {
			t.Fatalf("locked poll %d lost last-known evidence: source=%s memory=%#v want=%#v", poll, source, locked.Memory, initial.Memory)
		}
	}
	if calls.Load() != before {
		t.Fatalf("status-lock sent agent commands: %d -> %d", before, calls.Load())
	}
	if !reflect.DeepEqual(m.guestMetadataCache[metadataKey], originalMetadata) || !reflect.DeepEqual(m.vmAgentMemCache[memKey], originalMemory) {
		t.Fatal("lock renewed/replaced cached observations")
	}
	for metric, want := range map[string]int{"cpu": lockedPolls + 2, "memory": 1, "memoryused": 1, "disk": 1} {
		if got := len(m.metricsHistory.GetGuestMetrics(identity, metric, time.Hour)); got != want {
			t.Errorf("%s history points = %d, want %d", metric, got, want)
		}
	}
	// Cluster lock must survive a failed status call and the recent-evidence fallback.
	phase.Store(2)
	if err := json.Unmarshal([]byte(`{"type":"qemu","node":"node","name":"guest","vmid":105,"status":"running","lock":"backup","maxmem":8388608,"mem":8388608,"maxdisk":1048576000}`), &res); err != nil {
		t.Fatal(err)
	}
	locked, _ = build()
	if calls.Load() != before || locked.GuestAgentStatus != "deferred" || locked.AgentVersion != initial.AgentVersion || guestMemoryValuesOnly(locked.Memory) != guestMemoryValuesOnly(initial.Memory) {
		t.Fatalf("resource-lock fallback queried/lost identity: calls=%d %#v", calls.Load(), locked)
	}
	// The per-node inventory fallback must propagate its lock to the same builder.
	manager := alerts.NewManagerWithDataDir(t.TempDir())
	defer manager.Stop()
	m.alertManager = manager
	nodeVMs, _ := m.pollNodeVMsWithClusterResourceBuilder(context.Background(), "fixture", "node", []proxmox.VM{{VMID: 105, Name: "guest", Status: "running", Lock: "backup", MaxMem: 8 * mib, Mem: 8 * mib, MaxDisk: 1000 * mib}}, client, map[string]models.VM{makeGuestID("fixture", "node", 105): locked}, nil)
	if len(nodeVMs) != 1 || nodeVMs[0].GuestAgentStatus != "deferred" || calls.Load() != before {
		t.Fatalf("node fallback lost lock or sent guest commands: %#v calls=%d", nodeVMs, calls.Load())
	}
	// Resume from expired, unrenewed caches; new data must replace the retained view.
	phase.Store(3)
	res.Lock = ""
	cached := m.vmAgentMemCache[memKey]
	cached.fetchedAt = time.Now().Add(-2 * vmAgentMemCacheTTL)
	m.vmAgentMemCache[memKey] = cached
	metadata := m.guestMetadataCache[metadataKey]
	metadata.fetchedAt = time.Now().Add(-2 * guestMetadataCacheTTL)
	m.guestMetadataCache[metadataKey] = metadata
	m.guestMetadataLimiter = make(map[string]time.Time)
	resumed, source := build()
	if resumed.ID != initial.ID || resumed.GuestAgentStatus != "available" || resumed.DiskStatusReason != "" || resumed.Disk.Used != int64(400*mib) || resumed.Memory.Used != int64(4*mib) || source != "guest-agent-meminfo" {
		t.Fatalf("healthy resumption failed: %s %#v", source, resumed)
	}
	if calls.Load() != before+5 {
		t.Errorf("fresh resume commands = %d, want %d", calls.Load(), before+5)
	}
	if got := len(m.metricsHistory.GetGuestMetrics(identity, "disk", time.Hour)); got != 2 {
		t.Errorf("cached disk was written as new telemetry: %d points", got)
	}
}

func TestGuestAgentDeferralPreservesMetadataAndMemoryCache(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/config") {
			fmt.Fprint(w, `{"data":{}}`)
			return
		}
		calls.Add(1)
		http.Error(w, "QEMU command timed out", 500)
	}))
	defer server.Close()
	client, err := proxmox.NewClient(proxmox.ClientConfig{Host: server.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture", Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().Add(-10 * time.Minute)
	metadata := guestMetadataCacheEntry{ipAddresses: []string{"192.0.2.10"}, osName: "Linux", agentVersion: "1.0", fetchedAt: now, osInfoFailureCount: 2}
	memory := agentMemCacheEntry{available: 1024, fetchedAt: now, info: proxmox.LinuxMemoryAvailability{Source: "meminfo-available", EffectiveAvailable: 1024}}
	mk, gk := guestMemoryCacheKey("fixture", "node", 105), guestMetadataCacheKey("fixture", "node", 105)
	m := &Monitor{guestMetadataCache: map[string]guestMetadataCacheEntry{gk: metadata}, vmAgentMemCache: map[string]agentMemCacheEntry{mk: memory}, guestMetadataLimiter: make(map[string]time.Time)}
	ips, _, os, _, version, _ := m.fetchGuestAgentMetadata(context.Background(), client, "fixture", "node", "guest", 105, &proxmox.VMStatus{Agent: proxmox.VMAgentField{Value: 1}}, false)
	if len(ips) != 1 || os != "Linux" || version != "1.0" {
		t.Fatal("deferral lost last known metadata")
	}
	if _, err := m.getVMAgentMemoryAvailability(context.Background(), client, "fixture", "node", 105); proxmox.GuestAgentDeferredReason(err) != "agent-cooldown" {
		t.Fatalf("memory retried deferred VM: %v", err)
	}
	if calls.Load() != 1 || !reflect.DeepEqual(m.guestMetadataCache[gk], metadata) || !reflect.DeepEqual(m.vmAgentMemCache[mk], memory) {
		t.Fatal("deferral queued work, renewed cache or poisoned supported-OS evidence")
	}
}

// Provenance is asserted separately; numeric continuity includes every old field.
func guestMemoryValuesOnly(memory models.Memory) models.Memory {
	memory.Observation = models.MemoryObservation{}
	return memory
}
