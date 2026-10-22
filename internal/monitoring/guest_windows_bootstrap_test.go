package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

const bootstrapMiB = uint64(1024 * 1024)
const bootstrapWindowsOS = "Microsoft Windows Server 2025"

// Deliberately make an inappropriate Linux read uncertain. The repair must
// avoid sending it, not reinterpret a 500 as safe completion or clear its fence.
type windowsBootstrapFixture struct {
	t              *testing.T
	poll           atomic.Int32
	mu             sync.Mutex
	commands       []string
	osName         string
	uncertain      string
	locked         bool
	linuxAvailable uint64
	fsEntered      chan struct{}
	fsRelease      chan struct{}
	fsOnce         sync.Once
}

func newWindowsBootstrapFixture(t *testing.T) (*windowsBootstrapFixture, *httptest.Server) {
	t.Helper()
	f := &windowsBootstrapFixture{t: t, osName: bootstrapWindowsOS, linuxAvailable: 6 * bootstrapMiB}
	f.poll.Store(1)
	s := httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(s.Close)
	return f, s
}

func (f *windowsBootstrapFixture) trace() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.commands...)
}

func (f *windowsBootstrapFixture) resetTrace() {
	f.mu.Lock()
	f.commands = nil
	f.mu.Unlock()
}

func (f *windowsBootstrapFixture) serve(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") != "PVEAPIToken=fixture@pve!pulse=fixture" {
		f.t.Error("bootstrap lost configured fixture authentication")
		w.WriteHeader(http.StatusForbidden)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	f.mu.Lock()
	osName, uncertain, locked, available := f.osName, f.uncertain, f.locked, f.linuxAvailable
	fsEntered, fsRelease := f.fsEntered, f.fsRelease
	f.mu.Unlock()
	switch {
	case strings.HasSuffix(r.URL.Path, "/nodes"):
		fmt.Fprint(w, `{"data":[]}`)
		return
	case strings.HasSuffix(r.URL.Path, "/config"):
		if locked {
			fmt.Fprint(w, `{"data":{"lock":"backup","ostype":"win11"}}`)
		} else {
			// A configured OS hint is not the classification used by the poller.
			fmt.Fprint(w, `{"data":{"ostype":"win11"}}`)
		}
		return
	case strings.HasSuffix(r.URL.Path, "/status/current"):
		fmt.Fprintf(w, `{"data":{"status":"running","agent":1,"maxmem":%d,"mem":%d}}`, 8*bootstrapMiB, uint64(2+f.poll.Load())*bootstrapMiB)
		return
	}
	parts := strings.Split(r.URL.Path, "/")
	cmd := parts[len(parts)-1]
	f.mu.Lock()
	f.commands = append(f.commands, cmd)
	f.mu.Unlock()
	if cmd == "get-fsinfo" && fsEntered != nil {
		f.fsOnce.Do(func() { close(fsEntered) })
		<-fsRelease
	}
	if cmd == uncertain || cmd == "file-read" && osName == bootstrapWindowsOS {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "fixture command completion is unknown")
		return
	}
	switch cmd {
	case "get-fsinfo":
		cUsed, dUsed := uint64(400)*bootstrapMiB, uint64(100)*bootstrapMiB
		if f.poll.Load() == 2 {
			cUsed, dUsed = 0, 0
		}
		fmt.Fprintf(w, `{"data":{"result":[{"mountpoint":"C:\\","type":"ntfs","total-bytes":%d,"used-bytes":%d},{"mountpoint":"D:\\","type":"ntfs","total-bytes":%d,"used-bytes":%d}]}}`, 1500*bootstrapMiB, cUsed, 500*bootstrapMiB, dUsed)
	case "network-get-interfaces":
		fmt.Fprint(w, `{"data":{"result":[{"name":"Ethernet","ip-addresses":[{"ip-address":"192.0.2.10","ip-address-type":"ipv4","prefix":24}]}]}}`)
	case "get-osinfo":
		if osName == "" {
			fmt.Fprint(w, `{"data":{}}`)
		} else {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"data": map[string]string{"name": osName}})
		}
	case "info":
		fmt.Fprint(w, `{"data":{"result":{"version":"fixture"}}}`)
	case "file-read":
		if r.URL.Query().Get("file") != "/proc/meminfo" {
			f.t.Error("bootstrap requested an unexpected guest file")
		}
		content := fmt.Sprintf("MemTotal: 8192 kB\nMemAvailable: %d kB\nMemFree: 2048 kB\n", available/1024)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"data": map[string]interface{}{"content": content, "truncated": false}})
	default:
		f.t.Errorf("unexpected bootstrap request %s", r.URL.Path)
		http.NotFound(w, r)
	}
}

func windowsBootstrapClient(t *testing.T, host, kind string) PVEClientInterface {
	t.Helper()
	cfg := proxmox.ClientConfig{Host: host, TokenName: "fixture@pve!pulse", TokenValue: "fixture", Timeout: windowsPollingHangGuard}
	if kind == "cluster-client" {
		return proxmox.NewClusterClient("bootstrap", cfg, []string{host}, nil)
	}
	c, err := proxmox.NewClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func windowsBootstrapMonitor(t *testing.T) *Monitor {
	t.Helper()
	m := guestHistoryObservationMonitor(t)
	m.guestAgentFSInfoTimeout, m.guestAgentNetworkTimeout = windowsPollingHangGuard, windowsPollingHangGuard
	m.guestAgentOSInfoTimeout, m.guestAgentVersionTimeout = windowsPollingHangGuard, windowsPollingHangGuard
	m.alertManager = alerts.NewManagerWithDataDir(t.TempDir(), alerts.WithoutPersistedAlertRestore())
	t.Cleanup(m.alertManager.Stop)
	m.config, m.state, m.rateTracker = &config.Config{}, models.NewState(), NewRateTracker()
	m.resourceStore = unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
	return m
}

func windowsBootstrapPoll(t *testing.T, m *Monitor, client PVEClientInterface, collector, instance, node string, vmid, poll int) models.VM {
	t.Helper()
	id := makeGuestID(instance, node, vmid)
	cycleStart := time.Now()
	previous := m.previousGuestContextForInstance(instance).vmsByID[id]
	res := proxmox.ClusterResource{Type: "qemu", Node: node, Name: "guest", VMID: vmid, Status: "running", MaxMem: 8 * bootstrapMiB, MaxDisk: 2000 * bootstrapMiB, CPU: float64(poll) / 10}
	var vm models.VM
	if collector == "node" {
		vms, _ := m.pollNodeVMsWithClusterResourceBuilder(context.Background(), instance, node, []proxmox.VM{{VMID: vmid, Name: res.Name, Status: res.Status, MaxMem: res.MaxMem, MaxDisk: res.MaxDisk, CPU: res.CPU}}, client, map[string]models.VM{id: previous}, nil)
		if len(vms) != 1 {
			t.Errorf("node bootstrap lost guest: %d records", len(vms))
			return models.VM{}
		}
		vm = vms[0]
	} else {
		var ok bool
		vm, _, _, _, _, ok = m.buildVMFromClusterResource(context.Background(), instance, res, client, id, nil, &previous)
		if !ok {
			t.Error("cluster bootstrap lost guest")
			return models.VM{}
		}
	}
	m.state.UpdateVMs([]models.VM{vm})
	m.updateResourceStore(m.currentStateWithScope())
	m.recordGuestMetrics([]models.VM{vm}, nil, cycleStart)
	view := m.currentModeReadState().VMs()[0]
	canonicalID, resolved := m.resourceStore.(*unifiedresources.MonitorAdapter).ResolveCanonicalResourceID(id)
	knownDisk := vm.DiskStatusReason == "" || strings.HasPrefix(vm.DiskStatusReason, "prev-")
	if !resolved || view.ID() != canonicalID || knownDisk && view.DiskUsed() != vm.Disk.Used || view.DiskStatusReason() != vm.DiskStatusReason || view.OSName() != vm.OSName {
		t.Error("bootstrap changed at the canonical read boundary")
	}
	front := m.buildBroadcastFrontendStateFromSnapshot(models.StateSnapshot{VMs: []models.VM{vm}}, m.mockModeFence.begin())
	wire, err := json.Marshal(front.Resources)
	if err != nil || strings.Contains(string(wire), "windowsGuest") || strings.Contains(string(wire), "osInfoObservedAt") {
		t.Error("private OS admission evidence leaked into bootstrap JSON")
	}
	var served []struct{ Proxmox unifiedresources.ProxmoxData }
	if err := json.Unmarshal(wire, &served); err != nil || len(served) != 1 || served[0].Proxmox.OSName != vm.OSName || served[0].Proxmox.GuestAgentStatus != vm.GuestAgentStatus || served[0].Proxmox.DiskStatusReason != vm.DiskStatusReason {
		t.Error("served bootstrap reading lost OS or status")
	}
	return vm
}

func checkWindowsBootstrapReading(t *testing.T, vm models.VM, poll int) {
	t.Helper()
	wantDisk := int64(500 * bootstrapMiB)
	if poll == 2 {
		wantDisk = 0
	}
	if vm.OSName != bootstrapWindowsOS || vm.Disk.Total != int64(2000*bootstrapMiB) || vm.Disk.Used != wantDisk || len(vm.Disks) != 2 || vm.GuestAgentStatus != "available" || vm.DiskStatusReason != "" || vm.Memory.Used != int64(2+poll)*int64(bootstrapMiB) || vm.Memory.Observation.Source != "status-mem" || vm.Memory.Observation.State != "current" || vm.CPU != float64(poll)/10 {
		t.Errorf("bootstrap poll %d: OS=%q disk=%d/%d rows=%d guest=%q reason=%q memory=%d/%s/%s CPU=%v", poll, vm.OSName, vm.Disk.Used, vm.Disk.Total, len(vm.Disks), vm.GuestAgentStatus, vm.DiskStatusReason, vm.Memory.Used, vm.Memory.Observation.Source, vm.Memory.Observation.State, vm.CPU)
	}
	wantVolumes := map[string][2]int64{`C:\`: {int64(1500 * bootstrapMiB), int64(400 * bootstrapMiB)}, `D:\`: {int64(500 * bootstrapMiB), int64(100 * bootstrapMiB)}}
	if poll == 2 {
		for mount, volume := range wantVolumes {
			volume[1] = 0
			wantVolumes[mount] = volume
		}
	}
	volumes := make(map[string][2]int64)
	for _, disk := range vm.Disks {
		volumes[disk.Mountpoint] = [2]int64{disk.Total, disk.Used}
	}
	if !reflect.DeepEqual(volumes, wantVolumes) {
		t.Errorf("bootstrap poll %d lost individual NTFS volumes: %v, want %v", poll, volumes, wantVolumes)
	}
}

func checkBootstrapHistory(t *testing.T, m *Monitor, id, metric string, want []float64) {
	t.Helper()
	var stored, memory []float64
	for _, p := range guestHistoryStoredPoints(t, m, "vm", id, metric) {
		stored = append(stored, p.Value)
	}
	for _, p := range m.metricsHistory.GetGuestMetrics(id, metric, time.Hour) {
		memory = append(memory, p.Value)
	}
	if !reflect.DeepEqual(stored, want) || !reflect.DeepEqual(memory, want) {
		t.Errorf("bootstrap %s History stored=%v memory=%v, want %v", metric, stored, memory, want)
	}
}

func testGuestWindowsBootstrapPolling(t *testing.T) {
	for _, kind := range []string{"client", "cluster-client"} {
		for _, collector := range []string{"cluster", "node"} {
			for _, state := range []string{"cold", "expired"} {
				t.Run(kind+"/"+collector+"/"+state, func(t *testing.T) {
					f, s := newWindowsBootstrapFixture(t)
					client := windowsBootstrapClient(t, s.URL, kind)
					m := windowsBootstrapMonitor(t)
					key := guestMetadataCacheKey("bootstrap", "node", 105)
					if len(m.guestMetadataCache) != 0 || len(m.vmAgentMemCache) != 0 {
						t.Fatal("fresh monitor was not cold")
					}
					if state == "expired" {
						// Only this expiry control learns OS ahead of the poll. Its
						// actual receipt is then aged; no Windows strings are seeded,
						// and no command uncertainty fence is shortened.
						_, _, name, _, _, deferred := m.fetchGuestAgentMetadata(context.Background(), client, "bootstrap", "node", "guest", 105, &proxmox.VMStatus{Agent: proxmox.VMAgentField{Value: 1}}, false)
						if name != bootstrapWindowsOS || deferred {
							t.Fatal("expiry control did not obtain its real OS receipt")
						}
						aged := time.Now().Add(-2 * vmAgentMemCleanupMaxAge)
						entry := m.guestMetadataCache[key]
						entry.fetchedAt, entry.osInfoObservedAt = aged, aged
						m.guestMetadataCache[key] = entry
						m.guestMetadataLimiter[key] = aged
						f.resetTrace()
					}
					before := time.Now()
					first := windowsBootstrapPoll(t, m, client, collector, "bootstrap", "node", 105, 1)
					checkWindowsBootstrapReading(t, first, 1)
					osReceipt := m.guestMetadataCache[key]
					if !osReceipt.windowsGuest || osReceipt.osInfoObservedAt.Before(before) || osReceipt.osInfoObservedAt.After(time.Now()) {
						t.Error("ordinary first poll did not establish fresh actual OS evidence")
					}
					// Separate second-resolution History keys. This is not a QGA
					// cooldown wait; the successful path creates no such cooldown.
					time.Sleep(1100 * time.Millisecond)
					f.poll.Store(2)
					second := windowsBootstrapPoll(t, m, client, collector, "bootstrap", "node", 105, 2)
					checkWindowsBootstrapReading(t, second, 2)
					wantTrace := []string{"get-fsinfo", "network-get-interfaces", "get-osinfo", "info", "get-fsinfo"}
					if got := f.trace(); !reflect.DeepEqual(got, wantTrace) {
						t.Errorf("ordinary %s bootstrap wire trace=%v, want %v", state, got, wantTrace)
					}
					if !reflect.DeepEqual(m.guestMetadataCache[key], osReceipt) || len(m.vmAgentMemCache) != 0 {
						t.Error("second poll renewed OS evidence or invented a Linux memory observation")
					}
					id := makeGuestID("bootstrap", "node", 105)
					checkBootstrapHistory(t, m, id, "disk", []float64{25, 0})
					checkBootstrapHistory(t, m, id, "memory", []float64{37.5, 50})
					checkBootstrapHistory(t, m, id, "cpu", []float64{10, 20})
				})
			}
		}
	}
}

const bootstrapChildEnv = "PULSE_TEST_WINDOWS_BOOTSTRAP_CHILD"

type windowsBootstrapChild struct {
	Host, Kind, Collector, Output string
	Poll                          int
}

// Executed only by the parent contract using the same compiled test binary.
// Each invocation starts a new OS process, client, monitor and empty caches.
// This is source-level restart acceptance against loopback, not native PVE/Pro.
func TestGuestWindowsBootstrapProcessWorker(t *testing.T) {
	raw := os.Getenv(bootstrapChildEnv)
	if raw == "" {
		return
	}
	var child windowsBootstrapChild
	if err := json.Unmarshal([]byte(raw), &child); err != nil {
		t.Fatal(err)
	}
	m := windowsBootstrapMonitor(t)
	if len(m.guestMetadataCache) != 0 || len(m.vmAgentMemCache) != 0 {
		t.Fatal("new process inherited guest caches")
	}
	before := time.Now()
	vm := windowsBootstrapPoll(t, m, windowsBootstrapClient(t, child.Host, child.Kind), child.Collector, "bootstrap", "node", 105, child.Poll)
	checkWindowsBootstrapReading(t, vm, child.Poll)
	entry := m.guestMetadataCache[guestMetadataCacheKey("bootstrap", "node", 105)]
	if !entry.windowsGuest || entry.osInfoObservedAt.Before(before) || len(m.vmAgentMemCache) != 0 {
		t.Error("new process did not learn OS before optional Linux memory")
	}
	id := makeGuestID("bootstrap", "node", 105)
	wantDisk := float64(25)
	if child.Poll == 2 {
		wantDisk = 0
	}
	checkBootstrapHistory(t, m, id, "disk", []float64{wantDisk})
	checkBootstrapHistory(t, m, id, "memory", []float64{float64(2+child.Poll) / 8 * 100})
	checkBootstrapHistory(t, m, id, "cpu", []float64{float64(child.Poll) * 10})
	data, err := json.Marshal(struct {
		PID int
		VM  models.VM
	}{os.Getpid(), vm})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(child.Output, data, 0600); err != nil {
		t.Fatal(err)
	}
}

func testGuestWindowsBootstrapProcessRestart(t *testing.T) {
	for _, kind := range []string{"client", "cluster-client"} {
		for _, collector := range []string{"cluster", "node"} {
			t.Run(kind+"/"+collector, func(t *testing.T) {
				f, s := newWindowsBootstrapFixture(t)
				var firstPID int
				for poll := 1; poll <= 2; poll++ {
					f.poll.Store(int32(poll))
					f.resetTrace()
					out := filepath.Join(t.TempDir(), "child.json")
					cfg, err := json.Marshal(windowsBootstrapChild{s.URL, kind, collector, out, poll})
					if err != nil {
						t.Fatal(err)
					}
					ctx, cancel := context.WithTimeout(context.Background(), windowsPollingHangGuard)
					cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestGuestWindowsBootstrapProcessWorker$", "-test.count=1")
					cmd.Env = append(os.Environ(), bootstrapChildEnv+"="+string(cfg))
					output, err := cmd.CombinedOutput()
					cancel()
					if err != nil {
						t.Errorf("fresh process %d failed: %v\n%s", poll, err, output)
					}
					var result struct {
						PID int
						VM  models.VM
					}
					data, err := os.ReadFile(out)
					if err != nil || json.Unmarshal(data, &result) != nil {
						t.Fatal("fresh-process result missing or invalid")
					}
					checkWindowsBootstrapReading(t, result.VM, poll)
					if result.PID == os.Getpid() || poll == 2 && result.PID == firstPID {
						t.Fatal("restart reused the old process")
					}
					firstPID = result.PID
					wantTrace := []string{"get-fsinfo", "network-get-interfaces", "get-osinfo", "info"}
					if got := f.trace(); !reflect.DeepEqual(got, wantTrace) {
						t.Errorf("fresh process %d wire trace=%v, want %v", poll, got, wantTrace)
					}
					t.Logf("process %d PID=%d OS=%q disk=%d memory-source=%s", poll, result.PID, result.VM.OSName, result.VM.Disk.Used, result.VM.Memory.Observation.Source)
				}
			})
		}
	}
}
func testGuestWindowsBootstrapSafety(t *testing.T) {
	for _, kind := range []string{"client", "cluster-client"} {
		for _, collector := range []string{"cluster", "node"} {
			t.Run(kind+"/"+collector, func(t *testing.T) {
				t.Run("authoritative-lock", func(t *testing.T) {
					f, s := newWindowsBootstrapFixture(t)
					f.mu.Lock()
					f.locked = true
					f.mu.Unlock()
					m := windowsBootstrapMonitor(t)
					// Only metadata scheduling is shortened in this fixture. Lock
					// checks, client deadlines and command fencing are unchanged.
					m.guestMetadataRetryBackoff = time.Nanosecond
					client := windowsBootstrapClient(t, s.URL, kind)
					vm := windowsBootstrapPoll(t, m, client, collector, "bootstrap", "node", 105, 1)
					if vm.GuestAgentStatus != "deferred" || vm.DiskStatusReason != "vm-locked" || len(f.trace()) != 0 || len(m.guestMetadataCache) != 0 || len(m.vmAgentMemCache) != 0 {
						t.Error("cold Windows bootstrap bypassed the backup lock or invented evidence")
					}
					f.mu.Lock()
					f.locked = false
					f.mu.Unlock()
					checkWindowsBootstrapReading(t, windowsBootstrapPoll(t, m, client, collector, "bootstrap", "node", 105, 1), 1)
					if got := f.trace(); !reflect.DeepEqual(got, []string{"get-fsinfo", "network-get-interfaces", "get-osinfo", "info"}) {
						t.Errorf("unlocked ordinary bootstrap trace=%v", got)
					}
					checkBootstrapHistory(t, m, makeGuestID("bootstrap", "node", 105), "disk", []float64{25})
				})
				t.Run("in-flight-channel", func(t *testing.T) {
					f, s := newWindowsBootstrapFixture(t)
					f.mu.Lock()
					f.fsEntered, f.fsRelease = make(chan struct{}), make(chan struct{})
					f.mu.Unlock()
					m := windowsBootstrapMonitor(t)
					client := windowsBootstrapClient(t, s.URL, kind)
					done := make(chan models.VM, 1)
					go func() { done <- windowsBootstrapPoll(t, m, client, collector, "bootstrap", "node", 105, 1) }()
					select {
					case <-f.fsEntered:
					case <-time.After(windowsPollingHangGuard):
						close(f.fsRelease)
						t.Fatal("filesystem request never entered")
					}
					_, err := windowsBootstrapClient(t, s.URL, kind).GetVMAgentInfo(context.Background(), "node", 105)
					if proxmox.GuestAgentDeferredReason(err) != "agent-busy" || !reflect.DeepEqual(f.trace(), []string{"get-fsinfo"}) {
						t.Errorf("new diagnostic bypassed in-flight bootstrap: %v trace=%v", err, f.trace())
					}
					close(f.fsRelease)
					select {
					case vm := <-done:
						checkWindowsBootstrapReading(t, vm, 1)
					case <-time.After(windowsPollingHangGuard):
						t.Fatal("bootstrap did not finish after its original request completed")
					}
					if got := f.trace(); !reflect.DeepEqual(got, []string{"get-fsinfo", "network-get-interfaces", "get-osinfo", "info"}) {
						t.Errorf("in-flight owner lost the ordinary bootstrap: %v", got)
					}
				})
				for _, uncertain := range []string{"get-fsinfo", "network-get-interfaces", "get-osinfo", "info"} {
					t.Run("uncertain-"+uncertain, func(t *testing.T) {
						f, s := newWindowsBootstrapFixture(t)
						f.mu.Lock()
						f.uncertain = uncertain
						f.mu.Unlock()
						m := windowsBootstrapMonitor(t)
						m.guestAgentRetries = 2 // No uncertain command may be retried.
						client := windowsBootstrapClient(t, s.URL, kind)
						first := windowsBootstrapPoll(t, m, client, collector, "bootstrap", "node", 105, 1)
						trace := f.trace()
						if first.GuestAgentStatus != "deferred" || len(trace) == 0 || trace[len(trace)-1] != uncertain || len(m.vmAgentMemCache) != 0 {
							t.Errorf("unknown completion was exempted, replayed or lost: %v guest=%q", trace, first.GuestAgentStatus)
						}
						for _, cmd := range trace {
							if cmd == "file-read" {
								t.Fatal("optional Linux read followed an uncertain bootstrap command")
							}
						}
						_, err := windowsBootstrapClient(t, s.URL, kind).GetVMAgentInfo(context.Background(), "node", 105)
						if proxmox.GuestAgentDeferredReason(err) != "agent-cooldown" || !reflect.DeepEqual(f.trace(), trace) {
							t.Errorf("new client escaped bootstrap uncertainty: %v trace=%v", err, f.trace())
						}
						originalOS := m.guestMetadataCache[guestMetadataCacheKey("bootstrap", "node", 105)]
						id := makeGuestID("bootstrap", "node", 105)
						if uncertain == "get-fsinfo" {
							checkBootstrapHistory(t, m, id, "disk", nil)
						} else {
							checkBootstrapHistory(t, m, id, "disk", []float64{25})
						}
						// Distinct History keys expose false renewal. The live one-
						// minute uncertainty fence is not advanced or overwritten.
						time.Sleep(1100 * time.Millisecond)
						f.poll.Store(2)
						second := windowsBootstrapPoll(t, m, windowsBootstrapClient(t, s.URL, kind), collector, "bootstrap", "node", 105, 2)
						if second.GuestAgentStatus != "deferred" || !reflect.DeepEqual(f.trace(), trace) || !reflect.DeepEqual(m.guestMetadataCache[guestMetadataCacheKey("bootstrap", "node", 105)], originalOS) || len(m.vmAgentMemCache) != 0 {
							t.Error("paused poll dispatched or renewed original OS/memory evidence")
						}
						if uncertain != "get-fsinfo" {
							if second.Disk.Used != first.Disk.Used || second.DiskObservation != first.DiskObservation || second.DiskStatusReason != "prev-agent-cooldown" {
								t.Error("paused poll lost or renewed the accepted filesystem receipt")
							}
							checkBootstrapHistory(t, m, id, "disk", []float64{25})
						} else {
							checkBootstrapHistory(t, m, id, "disk", nil)
						}
						// A new monitor is still in the same process: cold caches do
						// not permit it to bypass the original client's admission.
						cold := windowsBootstrapMonitor(t)
						blocked := windowsBootstrapPoll(t, cold, windowsBootstrapClient(t, s.URL, kind), collector, "bootstrap", "node", 105, 2)
						if blocked.GuestAgentStatus != "deferred" || !reflect.DeepEqual(f.trace(), trace) || len(cold.guestMetadataCache) != 0 || len(cold.vmAgentMemCache) != 0 {
							t.Error("cold monitor bypassed live process uncertainty")
						}
						checkBootstrapHistory(t, m, id, "cpu", []float64{10, 20})
						checkBootstrapHistory(t, m, id, "memory", []float64{37.5, 50})
					})
				}
				t.Run("linux-unknown-and-replacement", func(t *testing.T) {
					f, s := newWindowsBootstrapFixture(t)
					m := windowsBootstrapMonitor(t)
					client := windowsBootstrapClient(t, s.URL, kind)
					checkWindowsBootstrapReading(t, windowsBootstrapPoll(t, m, client, collector, "bootstrap", "node", 105, 1), 1)
					original := m.guestMetadataCache[guestMetadataCacheKey("bootstrap", "node", 105)]
					for _, identity := range []struct {
						instance, node, os string
						id                 int
					}{{"other", "node", "Linux", 105}, {"bootstrap", "other", "", 107}, {"bootstrap", "node", "Linux", 106}} {
						f.mu.Lock()
						f.osName = identity.os
						f.mu.Unlock()
						f.resetTrace()
						vm := windowsBootstrapPoll(t, m, client, collector, identity.instance, identity.node, identity.id, 1)
						if !reflect.DeepEqual(f.trace(), []string{"get-fsinfo", "network-get-interfaces", "get-osinfo", "info", "file-read"}) || vm.Memory.Used != int64(2*bootstrapMiB) || vm.Memory.Observation.Source != "guest-agent-meminfo" || vm.GuestAgentStatus != "available" {
							t.Errorf("Windows evidence/config hint suppressed another identity's Linux fallback: trace=%v memory=%+v", f.trace(), vm.Memory)
						}
					}
					if !reflect.DeepEqual(m.guestMetadataCache[guestMetadataCacheKey("bootstrap", "node", 105)], original) {
						t.Error("replacement identity renewed the original Windows evidence")
					}
					// VMIDs are cluster-wide: a node move is the same Windows
					// guest, not another identity that should send a Linux read.
					// Its metadata/safety age stays unchanged; node-bound memory
					// observations are still read independently at the new node.
					f.mu.Lock()
					f.osName = bootstrapWindowsOS
					f.mu.Unlock()
					f.resetTrace()
					moved := windowsBootstrapPoll(t, m, client, collector, "bootstrap", "migrated", 105, 1)
					if moved.OSName != bootstrapWindowsOS || !reflect.DeepEqual(f.trace(), []string{"get-fsinfo"}) || moved.Memory.Used != int64(3*bootstrapMiB) || moved.Memory.Observation.Source != "status-mem" || !reflect.DeepEqual(m.guestMetadataCache[guestMetadataCacheKey("bootstrap", "node", 105)], original) {
						t.Errorf("Windows migration reset metadata or sent Linux work: trace=%v OS=%q memory=%+v", f.trace(), moved.OSName, moved.Memory)
					}
					if _, exists := m.vmAgentMemCache[guestMemoryCacheKey("bootstrap", "migrated", 105)]; exists {
						t.Error("Windows migration copied a node-bound memory cache")
					}
					// A reinstalled guest at the same tuple must be reclassified
					// from its new OS receipt, including measured-zero availability.
					key := guestMetadataCacheKey("bootstrap", "node", 105)
					aged := time.Now().Add(-2 * vmAgentMemCleanupMaxAge)
					original.fetchedAt, original.osInfoObservedAt = aged, aged
					m.guestMetadataCache[key], m.guestMetadataLimiter[key] = original, aged
					f.mu.Lock()
					f.osName, f.linuxAvailable = "Linux", 0
					f.mu.Unlock()
					f.resetTrace()
					vm := windowsBootstrapPoll(t, m, client, collector, "bootstrap", "node", 105, 1)
					if vm.Memory.Used != int64(8*bootstrapMiB) || vm.Memory.Observation.Source != "guest-agent-meminfo" || m.guestMetadataCache[key].windowsGuest || vm.OSName != "Linux" || !reflect.DeepEqual(f.trace(), []string{"get-fsinfo", "network-get-interfaces", "get-osinfo", "info", "file-read"}) {
						t.Errorf("new actual Linux OS failed to invalidate expired Windows evidence: trace=%v OS=%q memory=%+v", f.trace(), vm.OSName, vm.Memory)
					}
				})
			})
		}
	}
}
