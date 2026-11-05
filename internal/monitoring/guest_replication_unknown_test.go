package monitoring

import (
	"context"
	"crypto/sha256"
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

type replicationSafetyFrame struct {
	Phase             string          `json:"phase"`
	Resources         json.RawMessage `json:"resources"`
	GuestCommands     int32           `json:"guestCommands"`
	DiskHistoryPoints int             `json:"diskHistoryPoints"`
	ActiveAlerts      int             `json:"activeAlerts"`
	Alerts            []models.Alert  `json:"alerts"`
}

// Exercise the real HTTP admission, both enumeration routes, canonical saved
// context, alert evaluation, History and served resource projection. The fixture
// never freezes a guest or executes a backup; it models only the status wire.
func replicationSafetyMonitoringJourney(t *testing.T, clientKind, collector string) []replicationSafetyFrame {
	t.Helper()
	const mib = uint64(1024 * 1024)
	const instance = "replication-safety"
	var phase, commands atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "PVEAPIToken=fixture@pve!pulse=fixture" {
			t.Error("lost configured authentication")
			w.WriteHeader(403)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		p := phase.Load()
		switch {
		case strings.HasSuffix(r.URL.Path, "/nodes"):
			fmt.Fprint(w, `{"data":[{"node":"node","status":"online"}]}`)
		case strings.HasSuffix(r.URL.Path, "/cluster/replication"):
			fmt.Fprint(w, `{"data":[{"id":"105-0","guest":105,"type":"local","target":"peer"}]}`)
		case strings.HasSuffix(r.URL.Path, "/nodes/node/replication"):
			if r.URL.Query().Get("guest") != "105" {
				t.Error("lost guest status scope")
			}
			switch p {
			case 1:
				fmt.Fprintf(w, `{"data":[{"pid":4242,"last_try":%d}]}`, time.Now().Unix())
			case 2, 3:
				http.Error(w, `{"data":null,"message":"synthetic status failure"}`, http.StatusInternalServerError)
			default:
				fmt.Fprint(w, `{"data":[]}`)
			}
		case strings.HasSuffix(r.URL.Path, "/config"):
			fmt.Fprint(w, `{"data":{}}`)
		case strings.HasSuffix(r.URL.Path, "/status/current"):
			fmt.Fprintf(w, `{"data":{"status":"running","agent":1,"uptime":3600,"maxmem":%d,"mem":%d}}`, 8*mib, 8*mib)
		case strings.Contains(r.URL.Path, "/agent/"):
			commands.Add(1)
			switch {
			case strings.HasSuffix(r.URL.Path, "/get-fsinfo"):
				used := uint64(900)
				if p == 2 || p == 3 {
					used = 100
				}
				if p == 4 {
					used = 400
				}
				fmt.Fprintf(w, `{"data":{"result":[{"name":"sda1","mountpoint":"/data","type":"ext4","total-bytes":%d,"used-bytes":%d}]}}`, 1000*mib, used*mib)
			case strings.HasSuffix(r.URL.Path, "/file-read"):
				available := 5 * 1024
				if p > 0 {
					available = 4 * 1024
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"content": fmt.Sprintf("MemTotal: 8192 kB\nMemFree: 1024 kB\nMemAvailable: %d kB\n", available)}})
			case strings.HasSuffix(r.URL.Path, "/network-get-interfaces"):
				fmt.Fprint(w, `{"data":{"result":[{"name":"eth0","hardware-address":"02:00:00:00:00:01","ip-addresses":[{"ip-address":"192.0.2.10","ip-address-type":"ipv4","prefix":24}]}]}}`)
			case strings.HasSuffix(r.URL.Path, "/get-osinfo"):
				fmt.Fprint(w, `{"data":{"name":"Linux","version":"fixture"}}`)
			case strings.HasSuffix(r.URL.Path, "/info"):
				fmt.Fprint(w, `{"data":{"result":{"version":"fixture"}}}`)
			default:
				t.Error("unexpected guest command", r.URL.Path)
				http.NotFound(w, r)
			}
		default:
			t.Error("unexpected fixture route", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	host := fmt.Sprintf("%s/replication-safety-%x", server.URL, sha256.Sum256([]byte(t.Name())))
	cfg := proxmox.ClientConfig{Host: host, TokenName: "fixture@pve!pulse", TokenValue: "fixture", Timeout: guestPollFixtureHangGuard}
	var client PVEClientInterface
	if clientKind == "direct" {
		var err error
		client, err = proxmox.NewClient(cfg)
		if err != nil {
			t.Fatal(err)
		}
	} else {
		client = proxmox.NewClusterClient(instance, cfg, []string{host}, nil)
	}
	if _, err := client.GetReplicationStatus(context.Background()); err != nil {
		t.Fatal(err)
	}
	am := alerts.NewManagerWithDataDir(t.TempDir(), alerts.WithoutPersistedAlertRestore())
	t.Cleanup(am.Stop)
	acfg := am.GetConfig()
	acfg.Enabled = true
	acfg.GuestDefaults.Disk = &alerts.HysteresisThreshold{Trigger: 80, Clear: 75}
	acfg.MetricTimeThresholds = map[string]map[string]int{"guest": {"disk": 0}}
	am.UpdateConfig(acfg)
	m := &Monitor{config: &config.Config{}, alertManager: am, state: models.NewState(), rateTracker: NewRateTracker(), metricsHistory: NewMetricsHistory(32, time.Hour), guestMetadataLimiter: make(map[string]time.Time), guestAgentFSInfoCacheTTL: vmFSInfoCacheTTL}
	registry := unifiedresources.NewRegistry(nil)
	m.resourceStore = unifiedresources.NewMonitorAdapter(registry)
	id := makeGuestID(instance, "node", 105)
	previous := map[string]models.VM{}
	var frames []replicationSafetyFrame
	var originalMemory agentMemCacheEntry
	var originalMetadata guestMetadataCacheEntry
	var originalDisk models.GuestDiskObservation
	var originalUsed int64
	for step, name := range []string{"healthy", "replication-active", "status-unknown", "repeated-unknown", "recovered"} {
		phase.Store(int32(step))
		key := guestMemoryCacheKey(instance, "node", 105)
		metaKey := guestMetadataCacheKey(instance, "node", 105)
		if step == 1 || step == 4 {
			// Expire ordinary read TTLs, not their original retention authority. The
			// running job and unknown replies must never refresh those original times.
			entry := m.vmFSInfoCache[key]
			entry.fetchedAt = time.Now().Add(-2 * vmFSInfoCacheTTL)
			m.vmFSInfoCache[key] = entry
			mem := m.vmAgentMemCache[key]
			mem.fetchedAt = time.Now().Add(-2 * vmAgentMemCacheTTL)
			m.vmAgentMemCache[key] = mem
			metadata := m.guestMetadataCache[metaKey]
			metadata.fetchedAt = time.Now().Add(-2 * guestMetadataCacheTTL)
			m.guestMetadataCache[metaKey] = metadata
			m.guestMetadataLimiter = make(map[string]time.Time)
			if step == 1 {
				originalMemory = mem
				originalMetadata = metadata
			}
		}
		res := proxmox.ClusterResource{Type: "qemu", Node: "node", VMID: 105, Name: "replication-guest", Status: "running", CPU: .2, MaxCPU: 2, MaxMem: 8 * mib, Mem: 8 * mib, MaxDisk: 1000 * mib, Uptime: 3600}
		cycleStart := time.Now()
		var vms []models.VM
		if collector == "cluster" {
			vms = m.collectClusterVMResources(context.Background(), instance, []indexedClusterResource{{resource: res, guestID: id}}, client, previous, nil)
		} else {
			vms, _ = m.pollNodeVMsWithClusterResourceBuilder(context.Background(), instance, "node", []proxmox.VM{{VMID: 105, Name: res.Name, Status: res.Status, CPU: res.CPU, CPUs: res.MaxCPU, MaxMem: res.MaxMem, Mem: res.Mem, MaxDisk: res.MaxDisk, Uptime: 3600}}, client, previous, nil)
		}
		if len(vms) != 1 {
			t.Fatal("collection lost guest")
		}
		vm := vms[0]
		m.recordGuestMetrics(vms, nil, cycleStart)
		m.state.UpdateVMs(vms)
		m.updateResourceStore(m.currentStateWithScope())
		views := m.currentModeReadState().VMs()
		if len(views) != 1 || views[0].DiskStatusReason() != vm.DiskStatusReason {
			t.Fatal("canonical projection changed guest safety")
		}
		switch step {
		case 0:
			if vm.Disk.Usage != 90 || vm.DiskStatusReason != "" || vm.GuestAgentStatus != "available" || commands.Load() != 5 {
				t.Fatalf("initial observation failed: disk=%+v reason=%q agent=%q commands=%d", vm.Disk, vm.DiskStatusReason, vm.GuestAgentStatus, commands.Load())
			}
			originalDisk = vm.DiskObservation
			originalUsed = vm.Disk.Used
		case 1, 2, 3:
			reason := "prev-replication-active"
			if step > 1 {
				reason = "prev-lock-unverified"
			}
			if vm.DiskStatusReason != reason || vm.GuestAgentStatus != "deferred" || vm.Disk.Used != originalUsed || !reflect.DeepEqual(vm.DiskObservation, originalDisk) {
				t.Errorf("%s invented fresh guest readings: reason=%q agent=%q used=%d observation=%+v", name, vm.DiskStatusReason, vm.GuestAgentStatus, vm.Disk.Used, vm.DiskObservation)
			}
			if commands.Load() != 5 || !reflect.DeepEqual(m.vmAgentMemCache[key], originalMemory) || !reflect.DeepEqual(m.guestMetadataCache[metaKey], originalMetadata) {
				t.Errorf("%s dispatched guest commands or renewed original caches: %d", name, commands.Load())
			}
			if got := len(m.metricsHistory.GetGuestMetrics(id, "disk", time.Hour)); got != 1 {
				t.Errorf("%s fabricated current disk History: %d", name, got)
			}
		case 4:
			if vm.Disk.Usage != 40 || vm.DiskStatusReason != "" || vm.GuestAgentStatus != "available" || commands.Load() != 10 {
				t.Errorf("verified idle did not recover: disk=%+v reason=%q agent=%q commands=%d", vm.Disk, vm.DiskStatusReason, vm.GuestAgentStatus, commands.Load())
			}
		}
		wantAlerts := 1
		if step == 4 {
			wantAlerts = 0
		}
		if got := len(am.GetActiveAlerts()); got != wantAlerts {
			t.Errorf("%s changed established disk alert: got=%d want=%d", name, got, wantAlerts)
		}
		front := m.buildBroadcastFrontendStateFromSnapshot(models.StateSnapshot{VMs: vms, ActiveAlerts: m.activeAlertsSnapshot()}, m.mockModeFence.begin())
		wire, err := json.Marshal(front.Resources)
		if len(front.ActiveAlerts) != wantAlerts {
			t.Errorf("served %s alert state lost evaluated result: %d", name, len(front.ActiveAlerts))
		}
		if err != nil || len(front.Resources) != 1 {
			t.Fatal("served projection lost guest", err)
		}
		var served []struct{ Proxmox unifiedresources.ProxmoxData }
		if err := json.Unmarshal(wire, &served); err != nil || served[0].Proxmox.DiskStatusReason != vm.DiskStatusReason || served[0].Proxmox.GuestAgentStatus != vm.GuestAgentStatus {
			t.Fatal("served projection changed guest safety", err)
		}
		frames = append(frames, replicationSafetyFrame{Phase: name, Resources: wire, Alerts: front.ActiveAlerts, GuestCommands: commands.Load(), DiskHistoryPoints: len(m.metricsHistory.GetGuestMetrics(id, "disk", time.Hour)), ActiveAlerts: len(am.GetActiveAlerts())})
		previous[id] = previousVMFromView(views[0])
	}
	if got := len(m.metricsHistory.GetGuestMetrics(id, "disk", time.Hour)); got != 2 {
		t.Errorf("current History does not isolate unknown interval: %d", got)
	}
	return frames
}

func TestGuestAgentUnknownReplicationMonitoringContract(t *testing.T) {
	for _, client := range []string{"direct", "cluster"} {
		for _, collector := range []string{"cluster", "node"} {
			t.Run(client+"/"+collector, func(t *testing.T) { replicationSafetyMonitoringJourney(t, client, collector) })
		}
	}
}
