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

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// Exercise actual HTTP client, builders, saved snapshots, unified views, History
// and the production alert call. No native QGA or backup is performed.
func TestGuestBackupDeferralAlertObservationLifecycle(t *testing.T) {
	const mib = uint64(1024 * 1024)
	var phase, guestCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/config"):
			if phase.Load() == 1 {
				fmt.Fprint(w, `{"data":{"lock":"backup"}}`)
			} else {
				fmt.Fprint(w, `{"data":{}}`)
			}
		case strings.HasSuffix(r.URL.Path, "/status/current"):
			fmt.Fprintf(w, `{"data":{"status":"running","agent":1,"maxmem":%d,"mem":%d}}`, 1000*mib, 1000*mib)
		case strings.Contains(r.URL.Path, "/agent/"):
			guestCalls.Add(1)
			switch {
			case strings.HasSuffix(r.URL.Path, "file-read"):
				available := 40 * 1024
				if phase.Load() == 2 {
					available = 800 * 1024
				}
				json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"content": fmt.Sprintf("MemTotal: 1024000 kB\nMemFree: 10240 kB\nMemAvailable: %d kB\n", available)}})
			case strings.HasSuffix(r.URL.Path, "get-fsinfo"):
				used := 980 * mib
				if phase.Load() == 2 {
					used = 200 * mib
				}
				fmt.Fprintf(w, `{"data":{"result":[{"mountpoint":"/","type":"ext4","total-bytes":%d,"used-bytes":%d}]}}`, 1000*mib, used)
			case strings.HasSuffix(r.URL.Path, "network-get-interfaces"):
				fmt.Fprint(w, `{"data":{"result":[]}}`)
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
	am := alerts.NewManagerWithDataDir(t.TempDir(), alerts.WithoutPersistedAlertRestore())
	defer am.Stop()
	cfg := am.GetConfig()
	cfg.Enabled = true
	cfg.GuestDefaults.Memory = &alerts.HysteresisThreshold{Trigger: 85, Clear: 80}
	cfg.GuestDefaults.Disk = &alerts.HysteresisThreshold{Trigger: 90, Clear: 85}
	cfg.GuestDefaults.CPU = &alerts.HysteresisThreshold{Trigger: 80, Clear: 75}
	cfg.MetricTimeThresholds = map[string]map[string]int{"guest": {"memory": 0, "disk": 0, "cpu": 0}}
	am.UpdateConfig(cfg)
	registry := unifiedresources.NewRegistry(nil)
	m := &Monitor{config: &config.Config{}, alertManager: am, rateTracker: NewRateTracker(), metricsHistory: NewMetricsHistory(32, time.Hour), guestMetadataLimiter: make(map[string]time.Time)}
	res := proxmox.ClusterResource{Type: "qemu", Node: "node", Name: "guest", VMID: 105, Status: "running", MaxMem: 1000 * mib, Mem: 1000 * mib, MaxDisk: 1000 * mib}
	identity := makeGuestID("alert-observation", "node", 105)
	var previous *models.VM
	build := func() models.VM {
		t.Helper()
		vm, raw, source, notes, at, ok := m.buildVMFromClusterResource(context.Background(), "alert-observation", res, client, identity, nil, previous)
		if !ok {
			t.Fatal("VM disappeared")
		}
		m.recordGuestSnapshot("alert-observation", "qemu", "node", 105, GuestMemorySnapshot{Status: vm.Status, RetrievedAt: at, MemorySource: source, Memory: vm.Memory, Raw: raw, Notes: notes})
		m.recordGuestMetrics([]models.VM{vm}, nil, time.Now().Add(-time.Second))
		registry.IngestSnapshot(models.StateSnapshot{VMs: []models.VM{vm}})
		next := previousVMFromView(registry.VMs()[0])
		previous = &next
		return vm
	}
	initial := build()
	// The observation was made before alert policy became enabled for this VM.
	// A retained high value must not create an incident upon the next poll.
	phase.Store(1)
	before := guestCalls.Load()
	locked := build()
	if locked.Memory.Observation.State != "last-known" || !strings.HasPrefix(locked.DiskStatusReason, "prev-") {
		t.Fatalf("fixture did not produce real last-known evidence: %+v", locked)
	}
	m.checkGuestAlertsForVM("alert-observation", locked)
	if len(am.GetActiveAlerts()) != 0 {
		t.Fatal("production alert call treated retained backup-lock evidence as current")
	}
	// The original current evidence can establish an alert. Subsequent lock
	// polls must neither refresh its observation nor resolve it on expiry.
	m.checkGuestAlertsForVM("alert-observation", initial)
	if len(am.GetActiveAlerts()) != 2 {
		t.Fatal("current memory/filesystem evidence did not establish two alerts")
	}
	for poll := 0; poll < 4; poll++ {
		m.checkGuestAlertsForVM("alert-observation", build())
	}
	if guestCalls.Load() != before || len(am.GetActiveAlerts()) != 2 {
		t.Fatal("backup deferral queried QGA or lost the original alerts")
	}
	for _, a := range am.GetActiveAlerts() {
		if a.Value != 96 && a.Value != 98 {
			t.Fatal("retained poll changed the last trusted alert value")
		}
	}
	if got := len(m.metricsHistory.GetGuestMetrics(identity, "memory", time.Hour)); got != 1 {
		t.Fatalf("retained memory refreshed History: %d points", got)
	}
	if got := len(m.metricsHistory.GetGuestMetrics(identity, "disk", time.Hour)); got != 1 {
		t.Fatalf("retained filesystem refreshed History: %d points", got)
	}
	phase.Store(2)
	entry := m.vmAgentMemCache[guestMemoryCacheKey("alert-observation", "node", 105)]
	entry.fetchedAt = time.Now().Add(-2 * vmAgentMemCacheTTL)
	m.vmAgentMemCache[guestMemoryCacheKey("alert-observation", "node", 105)] = entry
	resumed := build()
	m.checkGuestAlertsForVM("alert-observation", resumed)
	if guestCalls.Load() <= before || resumed.Memory.Observation.State != "current" || resumed.DiskStatusReason != "" || len(am.GetActiveAlerts()) != 0 {
		t.Fatal("fresh unlocked memory/filesystem evidence did not recover the original incidents")
	}
}
